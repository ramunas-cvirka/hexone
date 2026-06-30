// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"testing"
	"time"
)

func TestMaybePlayBackgroundOperationSoundRequiresUnfocusedLongOperation(t *testing.T) {
	oldPlay := playBackgroundOperationSound
	defer func() {
		playBackgroundOperationSound = oldPlay
	}()

	calls := 0
	playBackgroundOperationSound = func() {
		calls++
	}

	now := time.Unix(100, 0)
	ui := &UI{windowFocused: false}
	ui.maybePlayBackgroundOperationSound(now.Add(-backgroundOperationSoundMinDuration), now)
	if calls != 1 {
		t.Fatalf("sound calls = %d, want 1 for long unfocused operation", calls)
	}

	ui.maybePlayBackgroundOperationSound(now.Add(-backgroundOperationSoundMinDuration+time.Nanosecond), now)
	if calls != 1 {
		t.Fatalf("sound calls = %d, want no change for short operation", calls)
	}

	ui.SetWindowFocused(true)
	ui.maybePlayBackgroundOperationSound(now.Add(-backgroundOperationSoundMinDuration), now)
	if calls != 1 {
		t.Fatalf("sound calls = %d, want no change while focused", calls)
	}

	ui.fmCfg = fm.DefaultConfig()
	ui.fmCfg.General.CompletionSound = fm.CompletionSoundAlways
	ui.maybePlayBackgroundOperationSound(now.Add(-backgroundOperationSoundMinDuration), now)
	if calls != 2 {
		t.Fatalf("sound calls = %d, want always mode to play while focused", calls)
	}

	ui.SetWindowFocused(false)
	ui.fmCfg.General.CompletionSound = fm.CompletionSoundNever
	ui.maybePlayBackgroundOperationSound(now.Add(-backgroundOperationSoundMinDuration), now)
	if calls != 2 {
		t.Fatalf("sound calls = %d, want never mode to suppress playback", calls)
	}
}
