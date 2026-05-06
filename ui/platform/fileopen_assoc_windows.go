// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package platform

import (
	"errors"
	"os/exec"
	"strings"
	"syscall"
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
	return openFileWithConfiguredAppCommand(appPath, filePath).Start()
}

func OpenFileWithSystemAssociation(filePath string) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return errors.New("file path is empty")
	}
	cmd := exec.Command("cmd", "/c", "start", "", filePath)
	configureViewerCommandProcess(cmd)
	return cmd.Start()
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
