// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package ui

import "syscall"

func mkfifoForTest(path string, mode uint32) error {
	return syscall.Mkfifo(path, mode)
}
