// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"slices"
	"testing"
)

func TestTerminalFindMatchesAreCaseInsensitiveWithHoverContext(t *testing.T) {
	lines := []string{
		"building package",
		"ERROR first failure",
		"retrying request",
		"second error returned",
		"done",
	}
	matches := terminalFindMatches(lines, "error", 20)
	if got, want := len(matches), 2; got != want {
		t.Fatalf("matches=%d want %d", got, want)
	}
	if got, want := matches[0].Row, 1; got != want {
		t.Fatalf("first row=%d want %d", got, want)
	}
	if got, want := len(matches[0].Preview), terminalFindPreviewLines; got != want {
		t.Fatalf("preview lines=%d want %d", got, want)
	}
	if matches[0].Preview[0] != lines[1] || matches[0].Preview[2] != lines[3] {
		t.Fatalf("preview=%q want current and following lines", matches[0].Preview)
	}
}

func TestTerminalFindMatchesUnicodeColumns(t *testing.T) {
	matches := terminalFindMatches([]string{"λx ERROR"}, "error", 10)
	if len(matches) != 1 {
		t.Fatalf("matches=%d want 1", len(matches))
	}
	if got, want := matches[0].StartCol, 3; got != want {
		t.Fatalf("start column=%d want %d", got, want)
	}
	if got, want := matches[0].EndCol, 7; got != want {
		t.Fatalf("end column=%d want %d", got, want)
	}
}

func TestTerminalFindResultLimit(t *testing.T) {
	matches := terminalFindMatches([]string{"x x x x"}, "x", 2)
	if got, want := len(matches), 2; got != want {
		t.Fatalf("matches=%d want capped %d", got, want)
	}
}

func TestTerminalFindPreviewPreservesIndentation(t *testing.T) {
	lines := []string{"service:", "\tport: 7031", "  enabled: true"}
	matches := terminalFindMatches(lines, "7031", 10)
	if len(matches) != 1 {
		t.Fatalf("matches=%d want 1", len(matches))
	}
	if got, want := matches[0].Preview[0], "\tport: 7031"; got != want {
		t.Fatalf("stored preview=%q want %q", got, want)
	}
	if got, want := terminalFindPreviewText(matches[0].Preview[0]), "    port: 7031"; got != want {
		t.Fatalf("rendered preview=%q want %q", got, want)
	}
	if got, want := matches[0].PreviewFocus, 0; got != want {
		t.Fatalf("preview focus=%d want %d", got, want)
	}
}

func TestTerminalFindDockedPreviewIncludesCurrentMatch(t *testing.T) {
	find := terminalFindState{
		matches: []terminalFindMatch{{
			Preview:      []string{"service:", "  port: 7031", "enabled: true"},
			PreviewFocus: 1,
		}},
	}
	preview := terminalFindPreviewForIndex(&find, 0)
	if got, want := len(preview), 3; got != want {
		t.Fatalf("preview lines=%d want %d", got, want)
	}
	if preview[1] != "  port: 7031" {
		t.Fatalf("preview=%q should include the current matching line", preview)
	}
}

func TestTerminalFindCustomPreviewOffsets(t *testing.T) {
	lines := []string{"zero", "one", "two match", "three", "four", "five"}
	matches := terminalFindMatchesWithPreview(lines, "match", 10, -1, 3)
	if len(matches) != 1 {
		t.Fatalf("matches=%d want 1", len(matches))
	}
	want := []string{"one", "two match", "three", "four", "five"}
	if !slices.Equal(matches[0].Preview, want) {
		t.Fatalf("preview=%q want %q", matches[0].Preview, want)
	}
	if got, want := matches[0].PreviewFocus, 1; got != want {
		t.Fatalf("preview focus=%d want %d", got, want)
	}
}
