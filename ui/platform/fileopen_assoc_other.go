//go:build !windows

package platform

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

func OpenFileWithConfiguredApp(appPath, filePath string) error {
	appPath = strings.TrimSpace(appPath)
	filePath = strings.TrimSpace(filePath)
	if appPath == "" {
		return errors.New("app path is empty")
	}
	if filePath == "" {
		return errors.New("file path is empty")
	}
	cmd := exec.Command(appPath, filePath)
	return cmd.Start()
}

func OpenFileWithSystemAssociation(filePath string) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return errors.New("file path is empty")
	}
	program := "xdg-open"
	if runtime.GOOS == "darwin" {
		program = "open"
	}
	cmd := exec.Command(program, filePath)
	return cmd.Start()
}

func configureViewerCommandProcess(cmd *exec.Cmd) {
	_ = cmd
}
