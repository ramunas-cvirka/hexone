//go:build windows

package ui

import (
	"os/exec"
	"syscall"
)

func configureViewerCommandProcess(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
