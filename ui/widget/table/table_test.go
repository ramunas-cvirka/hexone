// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package table

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/font"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func TestResetPointerStateClearsClickMemory(t *testing.T) {
	tbl := New(nil)
	tbl.rowClicks = []widget.Clickable{{}, {}}
	tbl.lastClickRow = 1
	tbl.lastClickAt = time.Now()

	tbl.ResetPointerState()

	if tbl.lastClickRow != -1 {
		t.Fatalf("lastClickRow = %d, want -1", tbl.lastClickRow)
	}
	if !tbl.lastClickAt.IsZero() {
		t.Fatal("lastClickAt should be cleared")
	}
	if len(tbl.rowClicks) != 2 {
		t.Fatalf("row click count = %d, want 2", len(tbl.rowClicks))
	}
}

func TestLayoutCellLabelCentersBaselineInTallCell(t *testing.T) {
	th := material.NewTheme()
	rawGtx := testTableLayoutContext(image.Pt(160, 24))
	helperGtx := testTableLayoutContext(image.Pt(160, 24))
	colorCall := testTextColorCall(rawGtx)

	lbl := widget.Label{
		Alignment: text.Start,
		MaxLines:  1,
		Truncator: "…",
	}
	rawDims, _ := lbl.LayoutDetailed(rawGtx, th.Shaper, font.Font{}, unit.Sp(13), "decoder_2525.go", colorCall)
	helperDims := layoutCellLabel(helperGtx, th, "", unit.Sp(13), "decoder_2525.go", CellStyle{
		Color:  color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		Weight: font.Normal,
	}, text.Start, false)

	if helperDims.Size != rawDims.Size {
		t.Fatalf("helper size = %v, want raw size %v", helperDims.Size, rawDims.Size)
	}
	if helperDims.Baseline >= rawDims.Baseline {
		t.Fatalf("helper baseline = %d, want < raw baseline %d", helperDims.Baseline, rawDims.Baseline)
	}
}

func TestAdaptiveBriefCellInsetsKeepsFullLeftPadding(t *testing.T) {
	gtx := testTableLayoutContext(image.Pt(160, 24))

	left, right := adaptiveBriefCellInsets(gtx, unit.Dp(4), 160)

	if got, want := gtx.Dp(left), 4; got != want {
		t.Fatalf("left inset = %d, want %d", got, want)
	}
	if got, want := gtx.Dp(right), 2; got != want {
		t.Fatalf("right inset = %d, want %d", got, want)
	}
}

func TestLeadingIconMetricsGivesParentMoreRoom(t *testing.T) {
	fileSize, fileGap := leadingIconMetrics(IconFile, 18)
	parentSize, parentGap := leadingIconMetrics(IconParent, 18)

	if parentSize <= fileSize {
		t.Fatalf("parent icon size = %d, want > file size %d", parentSize, fileSize)
	}
	if parentSize < 14 {
		t.Fatalf("parent icon size = %d, want at least 14", parentSize)
	}
	if parentGap >= fileGap {
		t.Fatalf("parent icon gap = %d, want < file gap %d", parentGap, fileGap)
	}
}

func TestLeadingIconMetricsScaleWithCellHeight(t *testing.T) {
	fileSmall, _ := leadingIconMetrics(IconFile, 18)
	fileLarge, _ := leadingIconMetrics(IconFile, 36)
	parentSmall, _ := leadingIconMetrics(IconParent, 18)
	parentLarge, _ := leadingIconMetrics(IconParent, 36)

	if fileLarge <= fileSmall {
		t.Fatalf("file icon should scale with cell height, got small=%d large=%d", fileSmall, fileLarge)
	}
	if parentLarge <= parentSmall {
		t.Fatalf("parent icon should scale with cell height, got small=%d large=%d", parentSmall, parentLarge)
	}
}

func TestApplyRowForegroundRespectsPreserveColor(t *testing.T) {
	fg := color.NRGBA{R: 0xAA, G: 0xBB, B: 0xCC, A: 0xFF}
	base := CellStyle{
		Color:         color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xFF},
		PreserveColor: true,
	}

	got := applyRowForeground(base, &fg)
	if got.Color != base.Color {
		t.Fatalf("applyRowForeground preserved color=%v want %v", got.Color, base.Color)
	}

	base.PreserveColor = false
	got = applyRowForeground(base, &fg)
	if got.Color != fg {
		t.Fatalf("applyRowForeground color=%v want row fg %v", got.Color, fg)
	}
}

func TestFullModeScrollbarReservesColumnWidth(t *testing.T) {
	th := material.NewTheme()
	tbl := New([]Column{
		{Width: unit.Dp(80), MinWidth: unit.Dp(20), Flex: true},
		{Width: unit.Dp(30), MinWidth: unit.Dp(20)},
	})
	tbl.RowHeight = unit.Dp(20)
	tbl.ScrollbarWidth = unit.Dp(10)
	gtx := testTableLayoutContext(image.Pt(120, 100))

	tbl.Layout(th, gtx, tableTestModel{rows: 20})

	if !tbl.scrollbarVisible {
		t.Fatal("expected full-mode vertical scrollbar")
	}
	if tbl.scrollbarAxis != layout.Vertical {
		t.Fatalf("scrollbar axis=%v want vertical", tbl.scrollbarAxis)
	}
	if got, want := tbl.hitSize.X, 106; got != want {
		t.Fatalf("hit width=%d want content width after scrollbar %d", got, want)
	}
	if got := sumInts(tbl.fullModeWidths); got != tbl.hitSize.X {
		t.Fatalf("column width sum=%d want hit width %d", got, tbl.hitSize.X)
	}
	if !tbl.HitScrollbar(image.Pt(109, 8)) {
		t.Fatal("scrollbar gutter should be hittable")
	}
	if col := tbl.HitColumn(image.Pt(109, 8)); col != -1 {
		t.Fatalf("scrollbar gutter column=%d want -1", col)
	}
}

func TestBriefModeScrollbarReservesBottomSpaceAndRecomputesRows(t *testing.T) {
	th := material.NewTheme()
	tbl := New([]Column{{Width: unit.Dp(60), MinWidth: unit.Dp(20), Flex: true}})
	tbl.SetMode(ModeBrief)
	tbl.RowHeight = unit.Dp(30)
	tbl.BriefColumnWidth = unit.Dp(48)
	tbl.BriefGap = unit.Dp(0)
	tbl.ScrollbarWidth = unit.Dp(10)
	gtx := testTableLayoutContext(image.Pt(120, 100))

	tbl.Layout(th, gtx, tableTestModel{rows: 30})

	if !tbl.scrollbarVisible {
		t.Fatal("expected brief-mode horizontal scrollbar")
	}
	if tbl.scrollbarAxis != layout.Horizontal {
		t.Fatalf("scrollbar axis=%v want horizontal", tbl.scrollbarAxis)
	}
	if got, want := tbl.briefRowsPerCol, 2; got != want {
		t.Fatalf("rows per brief column=%d want recomputed value %d", got, want)
	}
	if got, want := tbl.hitSize.Y, 86; got != want {
		t.Fatalf("hit height=%d want content height after scrollbar %d", got, want)
	}
	if tbl.HitRow(image.Pt(4, 89), 30) != -1 {
		t.Fatal("bottom scrollbar gutter should not hit a file row")
	}
}

func TestScrollbarDragUpdatesListPositionImmediately(t *testing.T) {
	th := material.NewTheme()
	tbl := New([]Column{{Width: unit.Dp(80), MinWidth: unit.Dp(20), Flex: true}})
	tbl.RowHeight = unit.Dp(20)
	tbl.ScrollbarWidth = unit.Dp(10)
	gtx := testTableLayoutContext(image.Pt(120, 100))
	tbl.Layout(th, gtx, tableTestModel{rows: 100})

	tbl.scrollbarDragGrab = tbl.scrollbarThumb.Dy() / 2
	if !tbl.setScrollFromScrollbarPos(image.Pt(tbl.scrollbarTrack.Min.X+1, tbl.scrollbarTrack.Max.Y), 100) {
		t.Fatal("dragging to the bottom should update the list position")
	}
	if got, want := tbl.List.Position.First, 96; got != want {
		t.Fatalf("list first=%d want max first %d", got, want)
	}
}

type tableTestModel struct {
	rows int
}

func (m tableTestModel) Len() int {
	return m.rows
}

func (m tableTestModel) Cell(row, col int) (string, CellStyle) {
	return "cell", CellStyle{Color: color.NRGBA{A: 255}}
}

func sumInts(values []int) int {
	total := 0
	for _, v := range values {
		total += v
	}
	return total
}

func testTableLayoutContext(size image.Point) layout.Context {
	var router input.Router
	return layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Min: size,
			Max: size,
		},
	}
}

func testTextColorCall(gtx layout.Context) op.CallOp {
	m := op.Record(gtx.Ops)
	paint.ColorOp{Color: color.NRGBA{R: 255, G: 255, B: 255, A: 255}}.Add(gtx.Ops)
	return m.Stop()
}
