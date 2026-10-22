package server

import (
	"github.com/buchin/cuan/internal/taskprocess"
	"os"
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == taskprocess.Command {
		os.Exit(taskprocess.Run(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
	}
}
