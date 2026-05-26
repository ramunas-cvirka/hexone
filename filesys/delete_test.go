// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDeletePathRemovesBrokenSymlinkItself(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("local symlink creation requires extra privileges on Windows")
	}

	root := t.TempDir()
	linkPath := filepath.Join(root, "broken-link")
	if err := os.Symlink("missing-target", linkPath); err != nil {
		t.Fatalf("os.Symlink: %v", err)
	}

	if err := DeletePath(linkPath); err != nil {
		t.Fatalf("DeletePath(%q): %v", linkPath, err)
	}
	if _, err := os.Lstat(linkPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Lstat(%q) error = %v, want not exist", linkPath, err)
	}
}
