// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package platform

import "testing"

func TestOpenDirectoryInSystemFileManagerCommandUsesShellStart(t *testing.T) {
	cmd := openDirectoryInSystemFileManagerCommand(`C:/tmp/docs`)
	want := []string{"cmd", "/c", "start", "", `C:\tmp\docs`}
	if len(cmd.Args) != len(want) {
		t.Fatalf("args len=%d want %d: %v", len(cmd.Args), len(want), cmd.Args)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Fatalf("args[%d]=%q want %q (all=%v)", i, cmd.Args[i], want[i], cmd.Args)
		}
	}
}

func TestRevealPathInSystemFileManagerCommandUsesSingleSelectArgument(t *testing.T) {
	cmd := revealPathInSystemFileManagerCommand(`C:/tmp/report.txt`)
	want := []string{"explorer.exe", `/select,"C:\tmp\report.txt"`}
	if len(cmd.Args) != len(want) {
		t.Fatalf("args len=%d want %d: %v", len(cmd.Args), len(want), cmd.Args)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Fatalf("args[%d]=%q want %q (all=%v)", i, cmd.Args[i], want[i], cmd.Args)
		}
	}
}
