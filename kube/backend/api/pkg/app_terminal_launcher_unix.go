//go:build !windows

package pkg

import (
	"os/exec"
	"syscall"
)

// detachProcess starts the child in its own session so signals sent
// to the Argus process group (Ctrl-C in a dev terminal, e.g.) do not
// also kill the spawned lufis-terminal. Setsid is the canonical way:
// it makes the child a session leader, detaching it from the
// controlling tty.
func detachProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}
