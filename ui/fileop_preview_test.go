// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"path/filepath"
	"testing"

	"hexone/filesys"
)

func TestSingleFileCopyAndMoveSourcesShowTheItem(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "source")
	path := filepath.Join(dir, "report.txt")

	copyState := &fileCopyState{
		srcPath: path,
		sources: []fileCopySource{{Path: path, Name: "report.txt"}},
	}
	if got, want := copyState.sourceLocation(), "report.txt"; got != want {
		t.Fatalf("copy source location=%q want %q", got, want)
	}

	moveState := &fileMoveState{
		srcPath: path,
		sources: []fileMoveSource{{Path: path, Name: "report.txt"}},
	}
	if got, want := moveState.sourceLocation(), "report.txt"; got != want {
		t.Fatalf("move source location=%q want %q", got, want)
	}
}

func TestCopyProgressCurrentShowsSingleSourceName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.txt")
	st := &fileCopyState{
		srcPath: path,
		sources: []fileCopySource{{Path: path, Name: "report.txt"}},
		progress: filesys.CopyProgress{
			CurrentPath: path,
		},
	}
	if got, want := st.progressCurrentLabel(), "report.txt"; got != want {
		t.Fatalf("current progress filename=%q want %q", got, want)
	}

	st.progress.CurrentPath = filepath.Join(filepath.Dir(path), "nested.txt")
	if got, want := st.progressCurrentLabel(), "nested.txt"; got != want {
		t.Fatalf("nested progress filename=%q want %q", got, want)
	}
}

func TestCopyProgressCurrentShowsSelectedDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vendor")
	current := filepath.Join(root, "pkg", "source.go")
	st := &fileCopyState{
		sources: []fileCopySource{{Path: root, Name: "vendor"}, {Path: "/tmp/readme", Name: "readme"}},
		progress: filesys.CopyProgress{
			CurrentRootPath: root,
			CurrentPath:     current,
		},
	}
	if got, want := st.progressCurrentLabel(), "vendor  ›  source.go"; got != want {
		t.Fatalf("progress source label=%q want %q", got, want)
	}
	if got, want := st.sourceLocation(), "2 items selected"; got != want {
		t.Fatalf("multi-source summary=%q want %q", got, want)
	}
}
