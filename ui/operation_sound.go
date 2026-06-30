// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"hexone/notify"
	"time"
)

const backgroundOperationSoundMinDuration = 2 * time.Second

var playBackgroundOperationSound = notify.PlayOperationComplete

func (ui *UI) SetWindowFocused(focused bool) {
	if ui == nil {
		return
	}
	ui.windowFocused = focused
}

func (ui *UI) maybePlayBackgroundOperationSound(startedAt, now time.Time) {
	if ui == nil || startedAt.IsZero() || now.Before(startedAt) {
		return
	}
	if now.Sub(startedAt) < backgroundOperationSoundMinDuration {
		return
	}
	mode := fm.CompletionSoundBackground
	if ui.fmCfg != nil {
		mode = fm.NormalizeCompletionSound(ui.fmCfg.General.CompletionSound)
	}
	switch mode {
	case fm.CompletionSoundNever:
		return
	case fm.CompletionSoundBackground:
		if ui.windowFocused {
			return
		}
	}
	playBackgroundOperationSound()
}
