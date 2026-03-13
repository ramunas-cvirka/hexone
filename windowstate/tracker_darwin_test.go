// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && !ios

package windowstate

import (
	"testing"

	"hexone/fm"

	"gioui.org/unit"
)

func TestApplyDarwinWindowStateUsesContentSize(t *testing.T) {
	s := fm.DefaultSession()
	s.Window.Width = 900
	s.Window.Height = 700

	applyDarwinWindowState(s, 20, 30, 720, 840, 2)

	if !s.Window.HasPosition {
		t.Fatal("window position should be marked present")
	}
	if got, want := s.Window.X, 40; got != want {
		t.Fatalf("window x=%d want %d", got, want)
	}
	if got, want := s.Window.Y, 60; got != want {
		t.Fatalf("window y=%d want %d", got, want)
	}
	if got, want := s.Window.Width, 1440; got != want {
		t.Fatalf("window width=%d want %d", got, want)
	}
	if got, want := s.Window.Height, 1680; got != want {
		t.Fatalf("window height=%d want %d", got, want)
	}
	if got, want := s.Window.PxPerDp, float32(2); got != want {
		t.Fatalf("window px_per_dp=%v want %v", got, want)
	}
}

func TestApplyDarwinWindowStateLeavesExistingSizeWhenMissing(t *testing.T) {
	s := fm.DefaultSession()
	s.Window.Width = 900
	s.Window.Height = 700

	applyDarwinWindowState(s, 10, 15, 0, 0, 1)

	if got, want := s.Window.Width, 900; got != want {
		t.Fatalf("window width=%d want %d", got, want)
	}
	if got, want := s.Window.Height, 700; got != want {
		t.Fatalf("window height=%d want %d", got, want)
	}
}

func TestApplyDarwinMetricFallbackPreservesNativeScale(t *testing.T) {
	s := fm.DefaultSession()
	s.Window.PxPerDp = 2

	applyDarwinMetricFallback(s, unit.Metric{PxPerDp: 1}, false)

	if got, want := s.Window.PxPerDp, float32(2); got != want {
		t.Fatalf("window px_per_dp=%v want %v", got, want)
	}
}

func TestApplyDarwinMetricFallbackAppliesWhenAllowed(t *testing.T) {
	s := fm.DefaultSession()
	s.Window.PxPerDp = 1

	applyDarwinMetricFallback(s, unit.Metric{PxPerDp: 2}, true)

	if got, want := s.Window.PxPerDp, float32(2); got != want {
		t.Fatalf("window px_per_dp=%v want %v", got, want)
	}
}
