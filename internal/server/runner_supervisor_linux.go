//go:build linux

package server

import (
	"os/exec"
	"syscall"
)

// configureSupervisorChildProcAttr ensures a supervised child exits if the parent server dies unexpectedly.
func configureSupervisorChildProcAttr(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGTERM
}
