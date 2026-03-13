// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowstate

import (
	"testing"

	"hexone/fm"

	"gioui.org/app"
	"gioui.org/unit"
)

func TestTrackerDelaysStartupPositionUntilFirstFrame(t *testing.T) {
	prevPlacement := winApplyStartupPlacement
	prevPos := winApplyStartupPosNoSize
	defer func() {
		winApplyStartupPlacement = prevPlacement
		winApplyStartupPosNoSize = prevPos
	}()

	posCalls := 0
	winApplyStartupPlacement = func(hwnd uintptr, x, y int) bool {
		t.Fatalf("unexpected placement restore for windowed startup: hwnd=%d x=%d y=%d", hwnd, x, y)
		return false
	}
	winApplyStartupPosNoSize = func(hwnd uintptr, x, y int) bool {
		posCalls++
		if hwnd != 1 || x != 320 || y != 240 {
			t.Fatalf("startup position = hwnd:%d x:%d y:%d, want hwnd:1 x:320 y:240", hwnd, x, y)
		}
		return true
	}

	tkr := NewTracker(&fm.SessionState{
		Window: fm.SessionWindow{
			Mode:        "windowed",
			X:           320,
			Y:           240,
			HasPosition: true,
		},
	}, func(f func()) { f() })
	tkr.hwnd = 1
	tkr.haveHWND = true
	tkr.cfg = app.Config{Mode: app.Windowed}
	tkr.haveCfg = true

	tkr.applyStartupPosition()
	if posCalls != 1 {
		t.Fatalf("startup position calls=%d want 1", posCalls)
	}
	if !tkr.startupPositionApplied {
		t.Fatal("startup position should be marked applied")
	}

	tkr.ObserveFrame(unit.Metric{PxPerDp: 1, PxPerSp: 1})
	if posCalls != 1 {
		t.Fatalf("startup position should only apply once, got %d calls", posCalls)
	}
}

func TestTrackerUsesPlacementRestoreForMaximizedStartup(t *testing.T) {
	prevPlacement := winApplyStartupPlacement
	prevPos := winApplyStartupPosNoSize
	defer func() {
		winApplyStartupPlacement = prevPlacement
		winApplyStartupPosNoSize = prevPos
	}()

	placementCalls := 0
	winApplyStartupPlacement = func(hwnd uintptr, x, y int) bool {
		placementCalls++
		if hwnd != 7 || x != 500 || y != 300 {
			t.Fatalf("startup placement = hwnd:%d x:%d y:%d, want hwnd:7 x:500 y:300", hwnd, x, y)
		}
		return true
	}
	winApplyStartupPosNoSize = func(hwnd uintptr, x, y int) bool {
		t.Fatalf("unexpected no-size move for maximized startup: hwnd=%d x=%d y=%d", hwnd, x, y)
		return false
	}

	tkr := NewTracker(&fm.SessionState{
		Window: fm.SessionWindow{
			Mode:        "maximized",
			X:           500,
			Y:           300,
			HasPosition: true,
		},
	}, func(f func()) { f() })
	tkr.hwnd = 7
	tkr.haveHWND = true
	tkr.cfg = app.Config{Mode: app.Maximized}
	tkr.haveCfg = true

	tkr.applyStartupPosition()
	if placementCalls != 1 {
		t.Fatalf("startup placement calls=%d want 1", placementCalls)
	}
	if !tkr.startupPositionApplied {
		t.Fatal("startup placement should be marked applied")
	}
}
