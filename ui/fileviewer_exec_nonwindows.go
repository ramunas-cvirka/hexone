//go:build !windows

package ui

import "os/exec"

func configureViewerCommandProcess(cmd *exec.Cmd) {
	_ = cmd
}
