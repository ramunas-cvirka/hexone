package ui

import (
	"hexone/ui/widget/table"
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestLayoutFilePaneModeGlyphKeepsSameCanvasAcrossModes(t *testing.T) {
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(16, 11)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}

	full := layoutFilePaneModeGlyph(gtx, table.ModeFull, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	gtx.Ops = new(op.Ops)
	brief := layoutFilePaneModeGlyph(gtx, table.ModeBrief, color.NRGBA{R: 255, G: 255, B: 255, A: 255})

	if full.Size != brief.Size {
		t.Fatalf("mode glyph size should stay stable across modes, got full=%v brief=%v", full.Size, brief.Size)
	}
}
