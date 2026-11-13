//go:build !linux

package taskprocess

import "syscall"

// SysProcAttr returns process attributes for non-Linux OSes (e.g. Darwin, BSD, Windows) where Pdeathsig is not supported.
func SysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}
