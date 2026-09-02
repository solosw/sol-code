//go:build windows

package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32                   = windows.NewLazySystemDLL("kernel32.dll")
	procCreateJobObjectW          = modKernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject   = modKernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject  = modKernel32.NewProc("AssignProcessToJobObject")
	procQueryInformationJobObject = modKernel32.NewProc("QueryInformationJobObject")
)

const (
	jobObjectBasicAccountingInformationClass = 1
	jobObjectExtendedLimitInformationClass   = 9
	jobObjectLimitKillOnJobClose             = 0x00002000
)

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

// JOBOBJECT_BASIC_ACCOUNTING_INFORMATION
type jobObjectBasicAccountingInformation struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

// processGuard owns OS resources that must outlive Start and die with Wait/Cancel.
type processGuard interface {
	AfterStart(proc *os.Process)
	// WaitChildren blocks until every process in the tree has exited (or ctx is done).
	// On Windows this covers children that outlive cmd.exe (e.g. python training).
	WaitChildren(ctx context.Context) error
	Close()
}

type windowsJobGuard struct {
	mu     sync.Mutex
	job    windows.Handle
	closed bool
}

func (g *windowsJobGuard) AfterStart(proc *os.Process) {
	if g == nil || proc == nil {
		return
	}
	g.mu.Lock()
	job := g.job
	closed := g.closed
	g.mu.Unlock()
	if job == 0 || closed {
		return
	}
	h, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_INFORMATION,
		false,
		uint32(proc.Pid),
	)
	if err != nil {
		return
	}
	r1, _, _ := procAssignProcessToJobObject.Call(uintptr(job), uintptr(h))
	_ = windows.CloseHandle(h)
	_ = r1
}

// WaitChildren polls the job until ActiveProcesses hits 0. Shell wrappers like
// cmd.exe can exit while a launched python/go process keeps the job non-empty;
// without this, Bash would report completion on the first batch of training logs
// once the wrapper exited.
func (g *windowsJobGuard) WaitChildren(ctx context.Context) error {
	if g == nil {
		return nil
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		g.mu.Lock()
		job := g.job
		closed := g.closed
		g.mu.Unlock()
		if job == 0 || closed {
			return nil
		}
		active, err := jobActiveProcesses(job)
		if err != nil {
			return nil // job already gone
		}
		if active == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (g *windowsJobGuard) Close() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return
	}
	g.closed = true
	if g.job != 0 {
		_ = windows.CloseHandle(g.job)
		g.job = 0
	}
}

func jobActiveProcesses(job windows.Handle) (uint32, error) {
	var info jobObjectBasicAccountingInformation
	var retLen uint32
	r1, _, err := procQueryInformationJobObject.Call(
		uintptr(job),
		uintptr(jobObjectBasicAccountingInformationClass),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
		uintptr(unsafe.Pointer(&retLen)),
	)
	if r1 == 0 {
		return 0, err
	}
	return info.ActiveProcesses, nil
}

// configureCommandCancellation places children into a kill-on-close job and
// wires Cancel to tear down the whole tree. WaitDelay is armed only on Cancel
// so a long-running process that keeps writing stdout is never treated as done
// merely because pipes went quiet — Wait blocks until the real process exits.
func configureCommandCancellation(cmd *exec.Cmd) processGuard {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	job, err := createKillOnCloseJob()
	var guard *windowsJobGuard
	if err == nil && job != 0 {
		guard = &windowsJobGuard{job: job}
	}

	cmd.Cancel = func() error {
		// After cancel only: do not block forever on orphaned pipe handles.
		cmd.WaitDelay = 250 * time.Millisecond

		if guard != nil {
			// Closing the job with KILL_ON_JOB_CLOSE terminates every member.
			guard.Close()
		}
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		pid := cmd.Process.Pid
		go func() {
			_ = exec.Command("taskkill", "/PID", fmt.Sprint(pid), "/T", "/F").Run()
		}()
		return cmd.Process.Kill()
	}

	if guard == nil {
		return nil
	}
	return guard
}

func createKillOnCloseJob() (windows.Handle, error) {
	r1, _, err := procCreateJobObjectW.Call(0, 0)
	if r1 == 0 {
		return 0, err
	}
	job := windows.Handle(r1)
	var info jobObjectExtendedLimitInformation
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	r1, _, err = procSetInformationJobObject.Call(
		uintptr(job),
		uintptr(jobObjectExtendedLimitInformationClass),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
	if r1 == 0 {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}
