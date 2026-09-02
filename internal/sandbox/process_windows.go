//go:build windows

package sandbox

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

type processGuard interface {
	AfterStart(proc *os.Process)
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
	_, _, _ = procAssignProcessToJobObject.Call(uintptr(job), uintptr(h))
	_ = windows.CloseHandle(h)
}

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
			return nil
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
		cmd.WaitDelay = 250 * time.Millisecond
		if guard != nil {
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
