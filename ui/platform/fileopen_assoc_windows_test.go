// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package platform

import (
	"errors"
	"testing"
)

func TestOpenFileWithSystemAssociationUsesTargetDirectory(t *testing.T) {
	original := shellExecuteFile
	t.Cleanup(func() { shellExecuteFile = original })

	var openedPath string
	var workingDirectory string
	shellExecuteFile = func(filePath, dir string) error {
		openedPath = filePath
		workingDirectory = dir
		return nil
	}

	wantPath := `C:\Program Files (x86)\Grinding Gear Games\Path of Exile 2\PathOfExile.exe`
	wantDir := `C:\Program Files (x86)\Grinding Gear Games\Path of Exile 2`
	if err := OpenFileWithSystemAssociation("  " + wantPath + "  "); err != nil {
		t.Fatalf("open file: %v", err)
	}
	if openedPath != wantPath {
		t.Fatalf("opened path = %q, want %q", openedPath, wantPath)
	}
	if workingDirectory != wantDir {
		t.Fatalf("working directory = %q, want %q", workingDirectory, wantDir)
	}
}

func TestOpenFileWithSystemAssociationPropagatesShellError(t *testing.T) {
	original := shellExecuteFile
	t.Cleanup(func() { shellExecuteFile = original })

	wantErr := errors.New("shell launch failed")
	shellExecuteFile = func(string, string) error { return wantErr }

	err := OpenFileWithSystemAssociation(`C:\Apps\broken.exe`)
	if !errors.Is(err, wantErr) {
		t.Fatalf("open error = %v, want %v", err, wantErr)
	}
}

func TestOpenFileWithConfiguredAppCommandKeepsAppWindowVisible(t *testing.T) {
	cmd := openFileWithConfiguredAppCommand(`C:\Apps\Player\player.exe`, `C:\Media\movie.mkv`)
	want := []string{`C:\Apps\Player\player.exe`, `C:\Media\movie.mkv`}
	if len(cmd.Args) != len(want) {
		t.Fatalf("args len=%d want %d: %v", len(cmd.Args), len(want), cmd.Args)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Fatalf("args[%d]=%q want %q (all=%v)", i, cmd.Args[i], want[i], cmd.Args)
		}
	}
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.HideWindow {
		t.Fatal("configured app should be launched with a visible window")
	}
}
