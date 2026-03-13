// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package fm

import "testing"

func TestSessionNormalizePaneDefaults(t *testing.T) {
	s := &SessionState{
		Panes: []SessionPane{
			{
				Dir:            " C:\\Temp ",
				SelectedPath:   " C:\\Temp\\file.txt ",
				SortKey:        "invalid",
				SortDescending: true,
				Mode:           "invalid",
			},
		},
	}

	s.normalize()

	if len(s.Panes) != 1 {
		t.Fatalf("len(Panes)=%d, want 1", len(s.Panes))
	}
	if got, want := s.Panes[0].SortKey, "name"; got != want {
		t.Fatalf("SortKey=%q, want %q", got, want)
	}
	if got, want := s.Panes[0].Mode, "full"; got != want {
		t.Fatalf("Mode=%q, want %q", got, want)
	}
	if !s.Panes[0].SortDescending {
		t.Fatal("SortDescending should preserve true")
	}
}

func TestSessionNormalizePaneAliases(t *testing.T) {
	s := &SessionState{
		Panes: []SessionPane{
			{SortKey: "type", Mode: "2c"},
			{SortKey: "datetime", Mode: "brief"},
			{SortKey: "size", Mode: "full"},
		},
	}

	s.normalize()

	if got, want := s.Panes[0].SortKey, "ext"; got != want {
		t.Fatalf("pane0 SortKey=%q, want %q", got, want)
	}
	if got, want := s.Panes[0].Mode, "brief"; got != want {
		t.Fatalf("pane0 Mode=%q, want %q", got, want)
	}
	if got, want := s.Panes[1].SortKey, "date"; got != want {
		t.Fatalf("pane1 SortKey=%q, want %q", got, want)
	}
	if got, want := s.Panes[2].Mode, "full"; got != want {
		t.Fatalf("pane2 Mode=%q, want %q", got, want)
	}
}
