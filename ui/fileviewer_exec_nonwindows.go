// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package ui

import "os/exec"

func configureViewerCommandProcess(cmd *exec.Cmd) {
	_ = cmd
}
