// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"hexone/fm"
)

func TestScrollActiveFileViewerByLinesUsesStreamMode(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	st := &fileViewerState{mode: "file"}
	st.stream.lines = []string{"alpha", "beta", "gamma"}
	st.stream.visibleLines = 1
	ui.fileViewer = st

	if !ui.performFileViewerKeyScroll(now, key.NameDownArrow) {
		t.Fatal("performFileViewerKeyScroll should scroll stream mode")
	}
	if got := st.stream.topLine; got != 1 {
		t.Fatalf("stream topLine=%d want 1", got)
	}
	if !st.userIsBrowsing(now.Add(time.Second)) {
		t.Fatal("stream key scroll should mark user browsing")
	}
}

func TestScrollActiveFileViewerByLinesUsesHexMode(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	st := &fileViewerState{
		mode: "hex",
		hex: &hexViewerState{
			bytesPerLine: 16,
			fileSize:     16000,
			visibleLines: 4,
			topLine:      100,
			bufferStart:  0,
			buffer:       make([]byte, 6000),
		},
	}
	ui.fileViewer = st

	if !ui.performFileViewerKeyScroll(now, key.NameDownArrow) {
		t.Fatal("performFileViewerKeyScroll should scroll hex mode")
	}
	if got := st.hex.topLine; got != 101 {
		t.Fatalf("hex topLine=%d want 101", got)
	}
	if !st.userIsBrowsing(now.Add(time.Second)) {
		t.Fatal("hex key scroll should mark user browsing")
	}
}

func TestPerformFileViewerKeyScrollPageAndBoundsStream(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	st := &fileViewerState{mode: "file"}
	st.stream.lines = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	st.stream.visibleLines = 4
	ui.fileViewer = st

	if !ui.performFileViewerKeyScroll(now, key.NamePageDown) {
		t.Fatal("PageDown should scroll stream mode")
	}
	if got := st.stream.topLine; got != 3 {
		t.Fatalf("topLine after PageDown=%d want 3", got)
	}
	if !ui.performFileViewerKeyScroll(now, key.NameEnd) {
		t.Fatal("End should jump stream mode to bottom")
	}
	if got := st.stream.topLine; got != 6 {
		t.Fatalf("topLine after End=%d want 6", got)
	}
	if !ui.performFileViewerKeyScroll(now, key.NameHome) {
		t.Fatal("Home should jump stream mode to top")
	}
	if got := st.stream.topLine; got != 0 {
		t.Fatalf("topLine after Home=%d want 0", got)
	}
}

func TestPerformFileViewerKeyScrollPageAndBoundsHex(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	st := &fileViewerState{
		mode: "hex",
		hex: &hexViewerState{
			bytesPerLine: 16,
			fileSize:     16000,
			visibleLines: 5,
			topLine:      100,
			bufferStart:  0,
			buffer:       make([]byte, 12000),
		},
	}
	ui.fileViewer = st

	if !ui.performFileViewerKeyScroll(now, key.NamePageDown) {
		t.Fatal("PageDown should scroll hex mode")
	}
	if got := st.hex.topLine; got != 104 {
		t.Fatalf("hex topLine after PageDown=%d want 104", got)
	}
	if !ui.performFileViewerKeyScroll(now, key.NameEnd) {
		t.Fatal("End should jump hex mode to bottom")
	}
	wantBottom := st.hex.totalLines() - int64(st.hex.visibleLines)
	if got := st.hex.topLine; got != wantBottom {
		t.Fatalf("hex topLine after End=%d want %d", got, wantBottom)
	}
	if !ui.performFileViewerKeyScroll(now, key.NameHome) {
		t.Fatal("Home should jump hex mode to top")
	}
	if got := st.hex.topLine; got != 0 {
		t.Fatalf("hex topLine after Home=%d want 0", got)
	}
}

func TestPumpFileViewerScrollRepeatUsesPaneTiming(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{mode: "file"}
	st.stream.lines = []string{"0", "1", "2", "3", "4"}
	st.stream.visibleLines = 1
	ui.fileViewer = st

	start := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	if !ui.performFileViewerKeyScroll(start, key.NameDownArrow) {
		t.Fatal("initial key press should scroll once")
	}
	ui.startFileViewerScrollRepeat(key.NameDownArrow, start)

	gtx := layout.Context{Ops: new(op.Ops), Now: start.Add(repeatStartDelay - time.Millisecond)}
	ui.pumpFileViewerScrollRepeat(gtx)
	if got := st.stream.topLine; got != 1 {
		t.Fatalf("topLine before repeat=%d want 1", got)
	}

	gtx.Now = start.Add(repeatStartDelay)
	ui.pumpFileViewerScrollRepeat(gtx)
	if got := st.stream.topLine; got != 2 {
		t.Fatalf("topLine at first repeat=%d want 2", got)
	}

	gtx.Now = gtx.Now.Add(repeatFast)
	ui.pumpFileViewerScrollRepeat(gtx)
	if got := st.stream.topLine; got != 3 {
		t.Fatalf("topLine at accelerated repeat=%d want 3", got)
	}

	ui.stopFileViewerScrollRepeat(key.NameDownArrow)
	if ui.rep.active {
		t.Fatal("repeat should stop on release")
	}
}

func TestViewerStepModeWrapsAcrossViewerModes(t *testing.T) {
	tests := []struct {
		mode string
		step int
		want string
	}{
		{mode: "file", step: 1, want: "hex"},
		{mode: "hex", step: 1, want: "command"},
		{mode: "command", step: 1, want: "file"},
		{mode: "file", step: -1, want: "command"},
		{mode: "command", step: -1, want: "hex"},
		{mode: "weird", step: 1, want: "hex"},
	}

	for _, tc := range tests {
		if got := viewerStepMode(tc.mode, tc.step); got != tc.want {
			t.Fatalf("viewerStepMode(%q, %d)=%q want %q", tc.mode, tc.step, got, tc.want)
		}
	}
}

func TestViewerModeTabStepAcceptsOnlyPlainTabAndShiftTab(t *testing.T) {
	tests := []struct {
		mods key.Modifiers
		want int
		ok   bool
	}{
		{mods: 0, want: 1, ok: true},
		{mods: key.ModShift, want: -1, ok: true},
		{mods: key.ModCtrl, want: 0, ok: false},
		{mods: key.ModShift | key.ModCtrl, want: 0, ok: false},
	}

	for _, tc := range tests {
		got, ok := viewerModeTabStep(tc.mods)
		if ok != tc.ok {
			t.Fatalf("viewerModeTabStep(%v) ok=%v want %v", tc.mods, ok, tc.ok)
		}
		if got != tc.want {
			t.Fatalf("viewerModeTabStep(%v) step=%d want %d", tc.mods, got, tc.want)
		}
	}
}

func TestPumpFileViewerScrollRepeatSupportsPageDown(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{mode: "file"}
	st.stream.lines = []string{
		"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
		"10", "11", "12", "13", "14", "15", "16", "17", "18", "19",
	}
	st.stream.visibleLines = 5
	ui.fileViewer = st

	start := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	if !ui.performFileViewerKeyScroll(start, key.NamePageDown) {
		t.Fatal("initial PageDown should scroll once")
	}
	ui.startFileViewerScrollRepeat(key.NamePageDown, start)

	gtx := layout.Context{Ops: new(op.Ops), Now: start.Add(repeatStartDelay)}
	ui.pumpFileViewerScrollRepeat(gtx)
	if got := st.stream.topLine; got != 8 {
		t.Fatalf("topLine after repeated PageDown=%d want 8", got)
	}

	ui.stopFileViewerScrollRepeat(key.NamePageDown)
	if ui.rep.active {
		t.Fatal("page repeat should stop on release")
	}
}
