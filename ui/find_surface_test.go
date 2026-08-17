// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"testing"
	"time"
)

func TestCompactFindCursorAnimMovesQuicklyTowardNewRow(t *testing.T) {
	base := time.Now()
	var anim compactFindCursorAnim
	anim.setTarget(base, 1)
	if _, alpha, progress, moving := anim.frame(base, 1); alpha != 255 || progress != 1 || moving {
		t.Fatalf("initial frame alpha=%d progress=%v moving=%v", alpha, progress, moving)
	}
	anim.setTarget(base.Add(time.Millisecond), 3)
	offset, alpha, progress, moving := anim.frame(base.Add(time.Millisecond), 3)
	if offset >= 0 || alpha >= 255 || progress != 0 || !moving {
		t.Fatalf("motion start offset=%d alpha=%d progress=%v moving=%v", offset, alpha, progress, moving)
	}
	offset, alpha, progress, moving = anim.frame(base.Add(time.Millisecond+compactFindCursorAnimDuration), 3)
	if offset != 0 || alpha != 255 || progress != 1 || moving {
		t.Fatalf("motion end offset=%d alpha=%d progress=%v moving=%v", offset, alpha, progress, moving)
	}
}
