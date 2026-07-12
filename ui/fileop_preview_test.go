// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"path/filepath"
	"reflect"
	"testing"

	"hexone/filesys"
)

func TestFileOpPreviewLines(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "three or fewer",
			in:   []string{"alpha", "beta", "gamma"},
			want: []string{"alpha", "beta", "gamma"},
		},
		{
			name: "four items",
			in:   []string{"alpha", "beta", "gamma", "omega"},
			want: []string{"alpha", "beta", "...", "omega"},
		},
		{
			name: "filters blanks",
			in:   []string{"alpha", "", "beta", "gamma", "omega"},
			want: []string{"alpha", "beta", "...", "omega"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileOpPreviewLines(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("fileOpPreviewLines(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSingleFileOperationSourceLocationShowsContainingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "source")
	path := filepath.Join(dir, "report.txt")

	copyState := &fileCopyState{
		srcPath: path,
		sources: []fileCopySource{{Path: path, Name: "report.txt"}},
	}
	if got, want := copyState.sourceLocation(), dir; got != want {
		t.Fatalf("copy source location=%q want %q", got, want)
	}

	moveState := &fileMoveState{
		srcPath: path,
		sources: []fileMoveSource{{Path: path, Name: "report.txt"}},
	}
	if got, want := moveState.sourceLocation(), dir; got != want {
		t.Fatalf("move source location=%q want %q", got, want)
	}
}

func TestCopyProgressCurrentOmitsRepeatedSingleSourceName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.txt")
	st := &fileCopyState{
		srcPath: path,
		sources: []fileCopySource{{Path: path, Name: "report.txt"}},
		progress: filesys.CopyProgress{
			CurrentPath: path,
		},
	}
	if got := st.progressCurrentLabel(); got != "" {
		t.Fatalf("repeated progress filename=%q want hidden", got)
	}

	st.progress.CurrentPath = filepath.Join(filepath.Dir(path), "nested.txt")
	if got, want := st.progressCurrentLabel(), "nested.txt"; got != want {
		t.Fatalf("nested progress filename=%q want %q", got, want)
	}
}
