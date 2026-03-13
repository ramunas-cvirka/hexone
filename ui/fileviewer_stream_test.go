// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"hexone/fm"
)

func TestStreamLinePaintSpecUsesOffsetInsteadOfSlicing(t *testing.T) {
	v := &streamOutputView{
		textPad:     2,
		charAdvance: 8.5,
		charW:       9,
		hCol:        5,
		wrapEnabled: false,
	}
	line := `alpha\x01beta\x02gamma`

	text, offsetX := v.linePaintSpec(line)
	if text != `\x01beta\x02gamma` {
		t.Fatalf("line paint text = %q, want visible suffix %q", text, `\x01beta\x02gamma`)
	}
	if want := 2; offsetX != want {
		t.Fatalf("line paint offset = %d, want %d", offsetX, want)
	}
}

func TestStreamLinePaintSpecLeavesWrappedLinesAtPad(t *testing.T) {
	v := &streamOutputView{
		textPad:     3,
		charAdvance: 8.0,
		charW:       8,
		hCol:        7,
		wrapEnabled: true,
	}
	line := "wrapped text stays anchored"

	text, offsetX := v.linePaintSpec(line)
	if text != line {
		t.Fatalf("wrapped line text = %q, want %q", text, line)
	}
	if offsetX != 3 {
		t.Fatalf("wrapped line offset = %d, want 3", offsetX)
	}
}

func TestTextOffsetFromPointRespectsFractionalHorizontalAdvance(t *testing.T) {
	v := &streamOutputView{
		lines:        []string{"abcdefghij"},
		lineOffsets:  []int{0},
		lineRunes:    []int{10},
		totalBytes:   10,
		topLine:      0,
		visibleLines: 1,
		lineH:        16,
		textRect:     imageRect(0, 0, 160, 16),
		textPad:      2,
		charAdvance:  8.5,
		charW:        9,
		hCol:         5,
	}

	pos := image.Pt(2+v.colOffsetPx(3)+1, 8)
	if got := v.textOffsetFromPoint(pos); got != 8 {
		t.Fatalf("text offset = %d, want 8", got)
	}
}

func TestPointOverSelectableTextRejectsEmptyAreaAfterScrolledShortLine(t *testing.T) {
	v := &streamOutputView{
		lines:        []string{"abcdefghij", "ab"},
		lineOffsets:  []int{0, 11},
		lineRunes:    []int{10, 2},
		totalBytes:   13,
		topLine:      0,
		visibleLines: 2,
		lineH:        16,
		textRect:     imageRect(0, 0, 160, 32),
		textPad:      2,
		charAdvance:  8.5,
		charW:        9,
		hCol:         5,
	}

	pos := image.Pt(40, 20)
	if v.pointOverSelectableText(pos) {
		t.Fatal("expected empty area on short scrolled line to be non-selectable")
	}
}

func TestEstimatedHColFromDragXPreservesThumbGrabOffset(t *testing.T) {
	v := &streamOutputView{
		hTrackRect:  imageRect(0, 0, 100, 10),
		hThumbRect:  imageRect(30, 0, 50, 10),
		textRect:    imageRect(0, 0, 100, 40),
		charAdvance: 8.5,
		charW:       8,
		textPad:     2,
		maxCols:     20,
	}

	got := v.estimatedHColFromDragX(35, 5)
	want := v.estimatedHColFromDragX(40, 10)
	if got != want {
		t.Fatalf("drag col with preserved grab = %d, want %d", got, want)
	}
}

func TestStopTextSelectionDragPreservesExistingSelectionRange(t *testing.T) {
	v := &streamOutputView{
		selActive:        true,
		selAnchor:        2,
		selHead:          9,
		selStart:         2,
		selLen:           7,
		selectingText:    true,
		selectID:         4,
		selectDirty:      true,
		cancelPending:    true,
		autoScrollActive: true,
	}

	v.stopTextSelectionDrag()

	if !v.selActive || v.selStart != 2 || v.selLen != 7 {
		t.Fatalf("selection changed unexpectedly: active=%v start=%d len=%d", v.selActive, v.selStart, v.selLen)
	}
	if v.selectingText || v.selectID != 0 || v.selectDirty || v.cancelPending || v.autoScrollActive {
		t.Fatalf("selection drag not fully stopped: selecting=%v id=%d dirty=%v cancel=%v auto=%v",
			v.selectingText, v.selectID, v.selectDirty, v.cancelPending, v.autoScrollActive)
	}
}

func TestMeasureStreamOutputTooltipBoxTracksContentWidth(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(640, 120),
		},
	}

	short := ui.measureStreamOutputTooltipBox(th, gtx, "~ line 1/9 (0.0%)", 400)
	long := ui.measureStreamOutputTooltipBox(th, gtx, "~ line 12345/67890 (100.0%)", 400)

	if short.X >= 160 {
		t.Fatalf("short tooltip width=%d want content-sized width below previous fixed width", short.X)
	}
	if long.X <= short.X {
		t.Fatalf("long tooltip width=%d want > short width %d", long.X, short.X)
	}
}

func TestMeasureStreamOutputTooltipBoxRespectsAvailableWidth(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(640, 120),
		},
	}

	box := ui.measureStreamOutputTooltipBox(th, gtx, "~ line 12345/67890 (100.0%)", 90)

	if box.X != 90 {
		t.Fatalf("tooltip width=%d want capped width 90", box.X)
	}
	if box.Y < 18 {
		t.Fatalf("tooltip height=%d want at least 18", box.Y)
	}
}

func imageRect(x0, y0, x1, y1 int) image.Rectangle {
	return image.Rect(x0, y0, x1, y1)
}
