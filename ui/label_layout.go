// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	uitheme "hexone/ui/theme"
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget/material"
)

func layoutVCenteredLabel(gtx layout.Context, lbl material.LabelStyle) layout.Dimensions {
	orig := gtx.Constraints
	labelGtx := gtx
	labelGtx.Constraints.Min = image.Point{}

	m := op.Record(gtx.Ops)
	dims := lbl.Layout(labelGtx)
	call := m.Stop()

	out := dims
	if orig.Min.X > out.Size.X {
		out.Size.X = orig.Min.X
	}
	if orig.Min.Y > out.Size.Y {
		out.Size.Y = orig.Min.Y
	}
	out.Size = orig.Constrain(out.Size)

	offsetX := 0
	if out.Size.X > dims.Size.X {
		switch lbl.Alignment {
		case text.Middle:
			offsetX = (out.Size.X - dims.Size.X) / 2
		case text.End:
			offsetX = out.Size.X - dims.Size.X
		}
	}
	offsetY := 0
	if out.Size.Y > dims.Size.Y {
		offsetY = (out.Size.Y - dims.Size.Y) / 2
	}
	if nudge := uitheme.OpticalTextYOffsetPx(gtx, lbl.Font.Typeface, lbl.TextSize); nudge > 0 {
		maxExtra := out.Size.Y - dims.Size.Y - offsetY
		if maxExtra < 0 {
			maxExtra = 0
		}
		if nudge > maxExtra {
			nudge = maxExtra
		}
		offsetY += nudge
	}
	if offsetX != 0 || offsetY != 0 {
		tr := op.Offset(image.Pt(offsetX, offsetY)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		tr.Pop()
	} else {
		call.Add(gtx.Ops)
	}

	if out.Baseline > 0 && offsetY > 0 {
		out.Baseline -= offsetY
		if out.Baseline < 0 {
			out.Baseline = 0
		}
	}
	return out
}
