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

func TestWindowOptionsForSessionIncludesPositionForMaximized(t *testing.T) {
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

	if !cfg.HasPosition {
		t.Fatal("expected restored options to preserve window position")
	}
	if got, want := cfg.Position.X, -500; got != want {
		t.Fatalf("cfg.Position.X=%d want %d", got, want)
	}
	if got, want := cfg.Position.Y, 120; got != want {
		t.Fatalf("cfg.Position.Y=%d want %d", got, want)
	}
	if got, want := cfg.Mode, app.Maximized; got != want {
		t.Fatalf("cfg.Mode=%v want %v", got, want)
	}
}

func TestWindowOptionsForSessionSkipsHiddenPosition(t *testing.T) {
	session := fm.DefaultSession()
	session.Window.Width = 1200
	session.Window.Height = 800
	session.Window.X = -32000
	session.Window.Y = -32000
	session.Window.HasPosition = true
	session.Window.Mode = "windowed"

	var cfg app.Config
	for _, opt := range windowOptionsForSession(session) {
		opt(unit.Metric{PxPerDp: 1, PxPerSp: 1}, &cfg)
	}

	if cfg.HasPosition {
		t.Fatal("hidden restore position should be ignored")
	}
}
