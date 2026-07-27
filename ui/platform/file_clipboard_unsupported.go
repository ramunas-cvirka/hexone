// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !linux && !windows

package platform

import "errors"

func ReadClipboardFiles() ([]string, error) {
	return nil, errors.New("file clipboard is unsupported on this platform")
}

func WriteClipboardFiles([]string) error {
	return errors.New("file clipboard is unsupported on this platform")
}
