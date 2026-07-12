// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package windowstate

import (
	"testing"

	"hexone/fm"

	"gioui.org/app"
	"gioui.org/unit"
)

func TestSessionWindowPositionLooksHidden(t *testing.T) {
	tests := []struct {
		x, y int
		want bool
	}{
		{x: -32000, y: -32000, want: true},
		{x: -32000, y: 120, want: true},
		{x: -1910, y: 80, want: false},
		{x: 40, y: 40, want: false},
	}

	for _, tc := range tests {
		if got := sessionWindowPositionLooksHidden(tc.x, tc.y); got != tc.want {
			t.Fatalf("sessionWindowPositionLooksHidden(%d, %d) = %v, want %v", tc.x, tc.y, got, tc.want)
		}
	}
}

func TestWindowOptionsForSessionRestoresSizeAndMode(t *testing.T) {
	session := fm.DefaultSession()
	session.Window.Width = 1200
	session.Window.Height = 800
	session.Window.X = -500
	session.Window.Y = 120
	session.Window.HasPosition = true
	session.Window.Mode = "maximized"
	session.Window.PxPerDp = 1

	var cfg app.Config
	for _, opt := range windowOptionsForSession(session) {
		opt(unit.Metric{PxPerDp: 1, PxPerSp: 1}, &cfg)
	}

	if got, want := cfg.Size.X, 1200; got != want {
		t.Fatalf("cfg.Size.X=%d want %d", got, want)
	}
	if got, want := cfg.Size.Y, 800; got != want {
		t.Fatalf("cfg.Size.Y=%d want %d", got, want)
	}
	if got, want := cfg.Mode, app.Maximized; got != want {
		t.Fatalf("cfg.Mode=%v want %v", got, want)
	}
}

func TestWindowOptionsForSessionIgnoresSavedPosition(t *testing.T) {
	session := fm.DefaultSession()
	session.Window.Width = 1200
	session.Window.Height = 800
	session.Window.X = 320
	session.Window.Y = 240
	session.Window.HasPosition = true
	session.Window.Mode = "windowed"

	var cfg app.Config
	for _, opt := range windowOptionsForSession(session) {
		opt(unit.Metric{PxPerDp: 1, PxPerSp: 1}, &cfg)
	}

	if got, want := cfg.Size.X, 1200; got != want {
		t.Fatalf("cfg.Size.X=%d want %d", got, want)
	}
	if got, want := cfg.Size.Y, 800; got != want {
		t.Fatalf("cfg.Size.Y=%d want %d", got, want)
	}
}

func TestCleanStartupWindowSizeUsesSixtyPercentOfDisplay(t *testing.T) {
	width, height := cleanStartupWindowSize(1920, 1080)

	if got, want := width, unit.Dp(1152); got != want {
		t.Fatalf("width=%v want %v", got, want)
	}
	if got, want := height, unit.Dp(648); got != want {
		t.Fatalf("height=%v want %v", got, want)
	}
}

func TestStartupWindowOptionsSizeAndCenterCleanSession(t *testing.T) {
	session := fm.DefaultSession()
	opts, center := startupWindowOptions(session, 1920, 1080, true)
	if !center {
		t.Fatal("clean windowed startup should be centered")
	}

	var cfg app.Config
	for _, opt := range opts {
		opt(unit.Metric{PxPerDp: 1, PxPerSp: 1}, &cfg)
	}
	if got, want := cfg.Size.X, 1152; got != want {
		t.Fatalf("cfg.Size.X=%d want %d", got, want)
	}
	if got, want := cfg.Size.Y, 648; got != want {
		t.Fatalf("cfg.Size.Y=%d want %d", got, want)
	}
}

func TestStartupWindowOptionsPreserveSavedSize(t *testing.T) {
	session := fm.DefaultSession()
	session.Window.Width = 1200
	session.Window.Height = 800
	session.Window.PxPerDp = 1

	opts, center := startupWindowOptions(session, 1920, 1080, true)
	if center {
		t.Fatal("saved startup should preserve its restored position")
	}

	var cfg app.Config
	for _, opt := range opts {
		opt(unit.Metric{PxPerDp: 1, PxPerSp: 1}, &cfg)
	}
	if got, want := cfg.Size.X, 1200; got != want {
		t.Fatalf("cfg.Size.X=%d want %d", got, want)
	}
	if got, want := cfg.Size.Y, 800; got != want {
		t.Fatalf("cfg.Size.Y=%d want %d", got, want)
	}
}

func TestSessionHasWindowSizeRequiresBothDimensions(t *testing.T) {
	tests := []struct {
		name    string
		session *fm.SessionState
		want    bool
	}{
		{name: "nil", session: nil, want: false},
		{name: "clean", session: fm.DefaultSession(), want: false},
		{name: "width only", session: &fm.SessionState{Window: fm.SessionWindow{Width: 1200}}, want: false},
		{name: "height only", session: &fm.SessionState{Window: fm.SessionWindow{Height: 800}}, want: false},
		{name: "saved", session: &fm.SessionState{Window: fm.SessionWindow{Width: 1200, Height: 800}}, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sessionHasWindowSize(tc.session); got != tc.want {
				t.Fatalf("sessionHasWindowSize()=%v want %v", got, tc.want)
			}
		})
	}
}

func TestSessionWindowRestorePosition(t *testing.T) {
	session := fm.DefaultSession()
	session.Window.X = 320
	session.Window.Y = 240
	session.Window.HasPosition = true

	x, y, ok := sessionWindowRestorePosition(session)
	if !ok {
		t.Fatal("expected valid restore position")
	}
	if x != 320 || y != 240 {
		t.Fatalf("restore position = (%d, %d), want (320, 240)", x, y)
	}
}

func TestSessionWindowRestorePositionRejectsHiddenWindow(t *testing.T) {
	session := fm.DefaultSession()
	session.Window.X = -32000
	session.Window.Y = 40
	session.Window.HasPosition = true

	if _, _, ok := sessionWindowRestorePosition(session); ok {
		t.Fatal("hidden position should not be restored")
	}
}

func TestMoveWindowRectOriginPreservesSize(t *testing.T) {
	left, top, right, bottom := moveWindowRectOrigin(10, 20, 210, 120, -500, 75)
	if left != -500 || top != 75 {
		t.Fatalf("origin = (%d, %d), want (-500, 75)", left, top)
	}
	if right != -300 || bottom != 175 {
		t.Fatalf("corner = (%d, %d), want (-300, 175)", right, bottom)
	}
}
