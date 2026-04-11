// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
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

func TestPerformFileViewerKeyScrollPDFDownArrowScrollsBeforePaging(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	now := time.Date(2026, time.April, 11, 15, 0, 0, 0, time.UTC)
	st := &fileViewerState{
		mode:                  "file",
		detectedImagePreview:  true,
		imagePreview:          image.NewNRGBA(image.Rect(0, 0, 400, 600)),
		imagePreviewFormat:    "pdf",
		imagePreviewPage:      0,
		imagePreviewPageCount: 3,
	}
	st.imageView.zoom = 1
	st.imageView.viewportRect = image.Rect(0, 0, 160, 120)
	ui.fileViewer = st

	if !ui.performFileViewerKeyScroll(now, key.NameDownArrow) {
		t.Fatal("Down should scroll within the current PDF page before paging")
	}
	if st.imageView.scrollY <= 0 {
		t.Fatalf("scrollY=%d want > 0", st.imageView.scrollY)
	}
	if st.status == "rendering page 2/3" {
		t.Fatal("Down should not start a new page render while the page can still scroll")
	}
}

func TestPerformFileViewerKeyScrollPDFDownArrowPagesAtBottom(t *testing.T) {
	prev := viewerPDFPreviewBackend
	fake := &fakeViewerPDFRenderer{
		available: true,
		result: viewerPDFRenderResult{
			Image:     image.NewNRGBA(image.Rect(0, 0, 120, 180)),
			Page:      1,
			PageCount: 3,
			Size:      image.Pt(120, 180),
		},
	}
	viewerPDFPreviewBackend = fake
	t.Cleanup(func() {
		viewerPDFPreviewBackend = prev
	})

	ui := NewUI(fm.DefaultConfig())
	now := time.Date(2026, time.April, 11, 15, 0, 0, 0, time.UTC)
	st := &fileViewerState{
		mode:                  "file",
		detectedImagePreview:  true,
		imagePreview:          image.NewNRGBA(image.Rect(0, 0, 400, 600)),
		imagePreviewData:      []byte("%PDF-1.7"),
		imagePreviewFormat:    "pdf",
		imagePreviewPage:      0,
		imagePreviewPageCount: 3,
		previewRenderCh:       make(chan fileViewerPreviewRenderResult, 1),
		seq:                   5,
	}
	st.imageView.zoom = 1
	st.imageView.viewportRect = image.Rect(0, 0, 160, 120)
	_, maxY := st.imageView.maxScroll(st.imagePreview)
	st.imageView.scrollY = maxY
	ui.fileViewer = st

	if !ui.performFileViewerKeyScroll(now, key.NameDownArrow) {
		t.Fatal("Down should move to the next PDF page at the bottom edge")
	}
	if got := st.status; got != "rendering page 2/3" {
		t.Fatalf("status=%q want %q", got, "rendering page 2/3")
	}
	select {
	case res := <-st.previewRenderCh:
		if res.page != 1 {
			t.Fatalf("rendered page=%d want 1", res.page)
		}
		if res.scrollToEnd {
			t.Fatal("Down edge paging should open the next page at the top")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pdf page render result")
	}
	if len(fake.requests) != 1 {
		t.Fatalf("render requests=%d want 1", len(fake.requests))
	}
	if got := fake.requests[0].Page; got != 1 {
		t.Fatalf("requested page=%d want 1", got)
	}
}

func TestPerformFileViewerKeyScrollPDFUpArrowPagesAtTopToPreviousPageEnd(t *testing.T) {
	prev := viewerPDFPreviewBackend
	fake := &fakeViewerPDFRenderer{
		available: true,
		result: viewerPDFRenderResult{
			Image:     image.NewNRGBA(image.Rect(0, 0, 120, 180)),
			Page:      0,
			PageCount: 3,
			Size:      image.Pt(120, 180),
		},
	}
	viewerPDFPreviewBackend = fake
	t.Cleanup(func() {
		viewerPDFPreviewBackend = prev
	})

	ui := NewUI(fm.DefaultConfig())
	now := time.Date(2026, time.April, 11, 15, 0, 0, 0, time.UTC)
	st := &fileViewerState{
		mode:                  "file",
		detectedImagePreview:  true,
		imagePreview:          image.NewNRGBA(image.Rect(0, 0, 400, 600)),
		imagePreviewData:      []byte("%PDF-1.7"),
		imagePreviewFormat:    "pdf",
		imagePreviewPage:      1,
		imagePreviewPageCount: 3,
		previewRenderCh:       make(chan fileViewerPreviewRenderResult, 1),
		seq:                   8,
	}
	st.imageView.zoom = 1
	st.imageView.viewportRect = image.Rect(0, 0, 160, 120)
	ui.fileViewer = st

	if !ui.performFileViewerKeyScroll(now, key.NameUpArrow) {
		t.Fatal("Up should move to the previous PDF page at the top edge")
	}
	if got := st.status; got != "rendering page 1/3" {
		t.Fatalf("status=%q want %q", got, "rendering page 1/3")
	}
	select {
	case res := <-st.previewRenderCh:
		if res.page != 0 {
			t.Fatalf("rendered page=%d want 0", res.page)
		}
		if !res.scrollToEnd {
			t.Fatal("Up edge paging should open the previous page at the bottom")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pdf page render result")
	}
	if len(fake.requests) != 1 {
		t.Fatalf("render requests=%d want 1", len(fake.requests))
	}
	if got := fake.requests[0].Page; got != 0 {
		t.Fatalf("requested page=%d want 0", got)
	}
}
