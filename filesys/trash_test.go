// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMovePathsToTrashRejectsEmptyInput(t *testing.T) {
	if err := MovePathsToTrash(nil); err == nil {
		t.Fatal("empty Trash request should fail")
	}
}

func TestMovePathsToTrashChecksSourceBeforePlatformCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	err := MovePathsToTrash([]string{path})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing source error=%v want os.ErrNotExist", err)
	}
}
