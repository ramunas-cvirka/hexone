// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"path/filepath"
	"testing"
)

func TestParseFileClipboardURIsSupportsGNOMEPayloadAndEscaping(t *testing.T) {
	got := parseFileClipboardURIs("copy\nfile:///tmp/one%20file.txt\nfile:///tmp/two.txt\n")
	want := []string{
		filepath.Clean("/tmp/one file.txt"),
		filepath.Clean("/tmp/two.txt"),
	}
	if len(got) != len(want) {
		t.Fatalf("paths=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("path[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeClipboardFilePathsDeduplicates(t *testing.T) {
	got := normalizeClipboardFilePaths([]string{"/tmp/one.txt", "/tmp/./one.txt", ""})
	if len(got) != 1 || got[0] != filepath.Clean("/tmp/one.txt") {
		t.Fatalf("paths=%v", got)
	}
}
