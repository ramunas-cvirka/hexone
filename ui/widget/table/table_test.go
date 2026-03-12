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
