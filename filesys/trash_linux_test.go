// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package filesys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFreeDesktopTrashEscapeUsesURLByteEncoding(t *testing.T) {
	if got, want := freeDesktopTrashEscape("/tmp/a b/%é"), "/tmp/a%20b/%25%C3%A9"; got != want {
		t.Fatalf("escaped path=%q want %q", got, want)
	}
}

func TestFreeDesktopFallbackMovesWithoutWalkingDirectory(t *testing.T) {
	root := t.TempDir()
	dataHome := filepath.Join(root, "data")
	t.Setenv("XDG_DATA_HOME", dataHome)
	source := filepath.Join(root, "folder")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "child.txt"), []byte("child"), 0o644); err != nil {
		t.Fatalf("write child: %v", err)
	}

	if err := movePathsToFreeDesktopHomeTrash([]string{source}); err != nil {
		t.Fatalf("move to fallback Trash: %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source should be moved, stat error=%v", err)
	}
	files, err := os.ReadDir(filepath.Join(dataHome, "Trash", "files"))
	if err != nil || len(files) != 1 {
		t.Fatalf("Trash files=%v err=%v", files, err)
	}
	info, err := os.ReadDir(filepath.Join(dataHome, "Trash", "info"))
	if err != nil || len(info) != 1 || !strings.HasSuffix(info[0].Name(), ".trashinfo") {
		t.Fatalf("Trash info=%v err=%v", info, err)
	}
}
