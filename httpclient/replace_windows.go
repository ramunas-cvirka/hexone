// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package httpclient

import (
	"errors"
	"os"
)

func replaceFile(oldPath, newPath string) error {
	if err := os.Rename(oldPath, newPath); err == nil {
		return nil
	}
	if err := os.Remove(newPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(oldPath, newPath)
}
