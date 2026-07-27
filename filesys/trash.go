// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrTrashUnsupported = errors.New("system trash is unavailable")

// MovePathsToTrash moves local files or directories to the operating system's
// Trash or Recycle Bin. It never falls back to permanent deletion.
func MovePathsToTrash(paths []string) error {
	cleaned := make([]string, 0, len(paths))
	for _, value := range paths {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		abs, err := filepath.Abs(value)
		if err != nil {
			return err
		}
		abs = filepath.Clean(abs)
		if _, err := os.Lstat(abs); err != nil {
			return err
		}
		cleaned = append(cleaned, abs)
	}
	if len(cleaned) == 0 {
		return fmt.Errorf("trash: no paths")
	}
	return movePathsToTrash(cleaned)
}
