//go:build !linux

package server

import "os/exec"

func configureSupervisorChildProcAttr(_ *exec.Cmd) {}
