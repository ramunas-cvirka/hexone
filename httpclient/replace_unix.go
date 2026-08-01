// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package httpclient

import "os"

func replaceFile(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
