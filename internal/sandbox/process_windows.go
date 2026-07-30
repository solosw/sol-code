//go:build windows

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

func configureCommandCancellation(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		pid := cmd.Process.Pid
		go func() {
			_ = exec.Command("taskkill", "/PID", fmt.Sprint(pid), "/T", "/F").Run()
		}()
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 250 * time.Millisecond
}
