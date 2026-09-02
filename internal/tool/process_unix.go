//go:build !windows

package tool

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type processGuard interface {
	AfterStart(proc *os.Process)
	WaitChildren(ctx context.Context) error
	Close()
}

type unixProcessGuard struct{}

func (unixProcessGuard) AfterStart(*os.Process)              {}
func (unixProcessGuard) WaitChildren(context.Context) error { return nil }
func (unixProcessGuard) Close()                             {}

// configureCommandCancellation puts the shell and all of its children in a
// separate process group. Canceling the tool context then terminates the whole
// group rather than merely the shell process.
//
// WaitDelay is armed only when Cancel runs. Setting it unconditionally made
// long jobs that keep writing stdout look finished as soon as pipes drained
// after a partial shutdown; normal completion must wait for the real exit.
func configureCommandCancellation(cmd *exec.Cmd) processGuard {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		cmd.WaitDelay = 250 * time.Millisecond
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}
		return nil
	}
	return unixProcessGuard{}
}
