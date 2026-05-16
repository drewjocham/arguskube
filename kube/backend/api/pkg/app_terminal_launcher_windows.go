//go:build windows

package pkg

import (
	"os/exec"
	"syscall"
)

// detachProcess uses CREATE_NEW_PROCESS_GROUP on Windows so the
// spawned terminal isn't bound to Argus's console session. The flag
// is the rough Windows analogue of POSIX setsid for our purposes.
func detachProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= 0x00000200 // CREATE_NEW_PROCESS_GROUP
}
