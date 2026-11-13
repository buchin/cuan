//go:build linux

package taskprocess

import "syscall"

// SysProcAttr returns process attributes appropriate for Linux, binding child lifetime to the parent thread.
func SysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}
}
