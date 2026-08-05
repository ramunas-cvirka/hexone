// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

var shellExecuteFile = shellExecuteFileWindows

func OpenFileWithConfiguredApp(appPath, filePath string) error {
	appPath = strings.TrimSpace(appPath)
	filePath = strings.TrimSpace(filePath)
	if appPath == "" {
		return errors.New("app path is empty")
	}
	if filePath == "" {
		return errors.New("file path is empty")
	}
	return openFileWithConfiguredAppCommand(appPath, filePath).Start()
}

func OpenFileWithSystemAssociation(filePath string) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return errors.New("file path is empty")
	}
	absolutePath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("resolve file path: %w", err)
	}
	absolutePath = filepath.Clean(absolutePath)
	return shellExecuteFile(absolutePath, filepath.Dir(absolutePath))
}

func shellExecuteFileWindows(filePath, workingDirectory string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(filePath)
	if err != nil {
		return err
	}
	directory, err := windows.UTF16PtrFromString(workingDirectory)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, target, nil, directory, windows.SW_SHOWNORMAL)
}

func configureViewerCommandProcess(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

func openFileWithConfiguredAppCommand(appPath, filePath string) *exec.Cmd {
	return exec.Command(appPath, filePath)
}
