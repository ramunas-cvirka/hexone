// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package ui

import "errors"

func mkfifoForTest(path string, mode uint32) error {
	return errors.New("mkfifo is unsupported on Windows")
}
