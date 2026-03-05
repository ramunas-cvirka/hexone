package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

func layoutNeutralEditorBox(gtx layout.Context, focused, enabled bool, inner layout.Widget) layout.Dimensions {
	bg := color.NRGBA{R: 34, G: 34, B: 34, A: 255}
	border := color.NRGBA{R: 255, G: 255, B: 255, A: 18}
	underline := color.NRGBA{R: 122, G: 114, B: 98, A: 185}
	underlineH := 1
	if focused && enabled {
		bg = color.NRGBA{R: 48, G: 48, B: 48, A: 255}
		border = color.NRGBA{R: 255, G: 255, B: 255, A: 46}
		underline = color.NRGBA{R: 160, G: 148, B: 122, A: 230}
		underlineH = 2
	}
	if !enabled {
		bg = color.NRGBA{R: 24, G: 24, B: 24, A: 255}
		border = color.NRGBA{R: 255, G: 255, B: 255, A: 10}
		underline = color.NRGBA{R: 104, G: 98, B: 88, A: 132}
		underlineH = 1
	}

	m := op.Record(gtx.Ops)
	dims := layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, inner)
	call := m.Stop()

	rect := image.Rect(0, 0, dims.Size.X, dims.Size.Y)
	rr := clip.UniformRRect(rect, 0)
	paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())
	if dims.Size.Y >= underlineH {
		paint.FillShape(gtx.Ops, underline, clip.Rect(image.Rect(0, dims.Size.Y-underlineH, dims.Size.X, dims.Size.Y)).Op())
	}

	call.Add(gtx.Ops)
	return dims
}
