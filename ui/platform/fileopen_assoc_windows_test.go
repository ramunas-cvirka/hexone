// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package platform

import "testing"

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
