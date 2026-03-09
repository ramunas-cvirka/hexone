package ui

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestLayoutVCenteredLabelLowersBaselineInTallCell(t *testing.T) {
	th := material.NewTheme()
	rawGtx := testLabelLayoutContext(image.Pt(180, 24))
	centeredGtx := testLabelLayoutContext(image.Pt(180, 24))

	lbl := material.Body2(th, "decoder_2525.go")
	lbl.TextSize = unit.Sp(13)
	lbl.MaxLines = 1
	lbl.Alignment = text.Start

	rawDims := lbl.Layout(rawGtx)
	centeredDims := layoutVCenteredLabel(centeredGtx, lbl)

	if centeredDims.Size != rawDims.Size {
		t.Fatalf("centered size = %v, want raw size %v", centeredDims.Size, rawDims.Size)
	}
	if centeredDims.Baseline >= rawDims.Baseline {
		t.Fatalf("centered baseline = %d, want < raw baseline %d", centeredDims.Baseline, rawDims.Baseline)
	}
}

func testLabelLayoutContext(size image.Point) layout.Context {
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
