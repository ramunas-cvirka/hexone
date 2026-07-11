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

func TestFullModeColumnGapsKeepFixedColumnsAtRight(t *testing.T) {
	tbl := New([]Column{
		{Width: unit.Dp(100), MinWidth: unit.Dp(20), Flex: true},
		{Width: unit.Dp(50), MinWidth: unit.Dp(50), GapBefore: unit.Dp(12)},
		{Width: unit.Dp(60), MinWidth: unit.Dp(60), GapBefore: unit.Dp(12)},
	})
	gtx := testTableLayoutContext(image.Pt(300, 100))
	widths := tbl.computeColumnWidths(gtx, 300)
	want := []int{166, 62, 72}
	for i := range want {
		if widths[i] != want[i] {
			t.Fatalf("column widths=%v want %v", widths, want)
		}
	}
}

func TestFullModeHidesPartialRowBelowMinimum(t *testing.T) {
	th := material.NewTheme()
	tbl := New([]Column{{Width: unit.Dp(80), MinWidth: unit.Dp(20), Flex: true}})
	tbl.RowHeight = unit.Dp(20)
	tbl.ScrollbarWidth = unit.Dp(10)
	// The table's 2px outer inset leaves a 101px viewport: five complete
	// rows and one pixel of the sixth row.
	gtx := testTableLayoutContext(image.Pt(120, 105))

	tbl.Layout(th, gtx, tableTestModel{rows: 20})

	if got, want := tbl.hitSize.Y, 100; got != want {
		t.Fatalf("hit height=%d want complete-row height %d", got, want)
	}
	if got, want := tbl.viewRows, 5; got != want {
		t.Fatalf("complete visible rows=%d want %d", got, want)
	}
	if got, want := tbl.pageStep(), 5; got != want {
		t.Fatalf("page step=%d want intersecting-row count %d", got, want)
	}
	if got, want := tbl.List.Position.Count, 5; got != want {
		t.Fatalf("fully visible list count=%d want %d", got, want)
	}
	if got := tbl.HitRow(image.Pt(4, 102), 20); got != -1 {
		t.Fatalf("sub-threshold remainder hit row %d want none", got)
	}

	if _, visible, maxFirst := tbl.scrollbarMetrics(20); visible != 5 || maxFirst != 15 {
		t.Fatalf("scrollbar visible/maxFirst=%d/%d want 5/15", visible, maxFirst)
	}

	// A row below the minimum preview threshold is not visible, so selecting
	// it should scroll normally.
	tbl.Selected = 5
	tbl.pendingEnsure = true
	tbl.Layout(th, gtx, tableTestModel{rows: 20})
	if got, want := tbl.List.Position.First, 1; got != want {
		t.Fatalf("first row after ensuring partial selection=%d want %d", got, want)
	}
	if rect, ok := tbl.RowRect(5, 20); !ok || rect.Dy() != 20 {
		t.Fatalf("ensured row rect=%v ok=%v want full row", rect, ok)
	}
}

func TestPartialRowViewportHeightRequiresContentThreshold(t *testing.T) {
	for _, tc := range []struct {
		height int
		want   int
	}{
		{height: 100, want: 100},
		{height: 101, want: 100},
		{height: 109, want: 100},
		{height: 110, want: 100},
		{height: 117, want: 100},
		{height: 118, want: 118},
		{height: 119, want: 119},
		{height: 120, want: 120},
	} {
		if got := partialRowViewportHeight(tc.height, 20, 18); got != tc.want {
			t.Fatalf("partialRowViewportHeight(%d, 20, 18)=%d want %d", tc.height, got, tc.want)
		}
	}
}

func TestFilePaneStylePartialRowRequiresIntactContent(t *testing.T) {
	tbl := New([]Column{{Width: unit.Dp(80), Flex: true}})
	tbl.RowHeight = unit.Dp(18)
	tbl.RowPadY = 0
	tbl.TextSize = unit.Sp(13)
	gtx := testTableLayoutContext(image.Pt(120, 100))
	model := tableIconTestModel{tableTestModel{rows: 10}}

	minimum := tbl.minimumPartialRowHeight(gtx, 18, model)
	if got, want := minimum, 16; got != want {
		t.Fatalf("file-pane partial-row minimum=%d want %d", got, want)
	}
	if got, want := partialRowViewportHeight(33, 18, minimum), 18; got != want {
		t.Fatalf("15px fragment viewport=%d want hidden at %d", got, want)
	}
	if got, want := partialRowViewportHeight(34, 18, minimum), 34; got != want {
		t.Fatalf("16px fragment viewport=%d want full available %d", got, want)
	}
}

func TestFullModeUsesAllRemainderAbovePartialMinimum(t *testing.T) {
	th := material.NewTheme()
	tbl := New([]Column{{Width: unit.Dp(80), MinWidth: unit.Dp(20), Flex: true}})
	tbl.RowHeight = unit.Dp(20)
	// The 119px inner viewport exposes 19px of the sixth row. Because that is
	// above the minimum threshold, all 19 available pixels should be used.
	gtx := testTableLayoutContext(image.Pt(120, 123))

	tbl.Layout(th, gtx, tableTestModel{rows: 20})

	if got, want := tbl.hitSize.Y, 119; got != want {
		t.Fatalf("partial-row hit height=%d want %d", got, want)
	}
	if rect, ok := tbl.RowRect(5, 20); !ok || rect.Dy() != 19 {
		t.Fatalf("partial row rect=%v ok=%v want 19px", rect, ok)
	}
	if got, want := tbl.HitRow(image.Pt(4, 120), 20), 5; got != want {
		t.Fatalf("available partial-row space hit=%d want row %d", got, want)
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
	if got, want := tbl.briefRowsPerCol, 3; got != want {
		t.Fatalf("rows per brief column=%d want recomputed value %d", got, want)
	}
	if got, want := tbl.hitSize.Y, 86; got != want {
		t.Fatalf("hit height=%d want content height after scrollbar %d", got, want)
	}
	if tbl.HitRow(image.Pt(4, 89), 30) != -1 {
		t.Fatal("bottom scrollbar gutter should not hit a file row")
	}
}

func TestBriefModeUsesRemainderAbovePartialMinimum(t *testing.T) {
	th := material.NewTheme()
	tbl := New([]Column{{Width: unit.Dp(80), MinWidth: unit.Dp(20), Flex: true}})
	tbl.SetMode(ModeBrief)
	tbl.RowHeight = unit.Dp(20)
	tbl.BriefColumnWidth = unit.Dp(200)
	tbl.BriefGap = unit.Dp(0)
	// The 119px inner height fits five complete rows plus 19px, enough to
	// preserve the row's centered content.
	gtx := testTableLayoutContext(image.Pt(120, 123))

	tbl.Layout(th, gtx, tableTestModel{rows: 6})

	if got, want := tbl.briefRowsPerCol, 6; got != want {
		t.Fatalf("brief rows per column=%d want intersecting-row count %d", got, want)
	}
	if tbl.scrollbarVisible {
		t.Fatal("six intersecting rows should fit in one brief column without a scrollbar")
	}
	if got, want := tbl.HitRow(image.Pt(4, 120), 6), 5; got != want {
		t.Fatalf("partially visible brief row hit=%d want row %d", got, want)
	}
	if rect, ok := tbl.RowRect(5, 6); !ok || rect.Dy() != 19 {
		t.Fatalf("partial brief row rect=%v ok=%v want 19px", rect, ok)
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

type tableIconTestModel struct {
	tableTestModel
}

func (tableIconTestModel) LeadingIcon(row, col int) (LeadingIcon, bool) {
	return LeadingIcon{Kind: IconFolder}, row >= 0 && col == 0
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
