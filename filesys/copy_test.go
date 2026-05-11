// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyPathContextReturnsCanceledBeforeCopy(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "source.txt")
	dstPath := filepath.Join(dstDir, "copy.txt")
	if err := os.WriteFile(srcPath, []byte("copy me"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q): %v", srcPath, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := CopyPathContext(ctx, srcPath, dstPath, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CopyPathContext error = %v, want context.Canceled", err)
	}
	if _, statErr := os.Stat(dstPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination stat error = %v, want not exist", statErr)
	}
}
