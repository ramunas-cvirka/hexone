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
	"golang.org/x/image/math/fixed"
)

func layoutVCenteredLabel(gtx layout.Context, lbl material.LabelStyle) layout.Dimensions {
	return layoutVCenteredLabelMode(gtx, lbl, false)
}

// layoutInkVCenteredLabel centers the visible glyph outline instead of the
// font's logical line box. It is useful for isolated icon-like letters, whose
// cap position can vary noticeably between typefaces even when their line
// metrics are centered correctly.
func layoutInkVCenteredLabel(gtx layout.Context, lbl material.LabelStyle) layout.Dimensions {
	return layoutVCenteredLabelMode(gtx, lbl, true)
}

func layoutVCenteredLabelMode(gtx layout.Context, lbl material.LabelStyle, centerInk bool) layout.Dimensions {
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
	if centerInk {
		if inkTop, inkBottom, ok := labelInkVerticalBounds(labelGtx, lbl); ok {
			inkH := inkBottom - inkTop
			desiredTop := (out.Size.Y - inkH) / 2
			offsetY += desiredTop - (offsetY + inkTop)
			minOffset := -inkTop
			maxOffset := out.Size.Y - inkBottom
			if offsetY < minOffset {
				offsetY = minOffset
			}
			if offsetY > maxOffset {
				offsetY = maxOffset
			}
		}
	} else if nudge := uitheme.OpticalTextYOffsetPx(gtx, lbl.Font.Typeface, lbl.TextSize); nudge > 0 {
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

func labelInkVerticalBounds(gtx layout.Context, lbl material.LabelStyle) (top, bottom int, ok bool) {
	if lbl.Shaper == nil || lbl.Text == "" {
		return 0, 0, false
	}
	lbl.Shaper.LayoutString(text.Parameters{
		Font:            lbl.Font,
		Alignment:       lbl.Alignment,
		PxPerEm:         fixed.I(gtx.Sp(lbl.TextSize)),
		MaxLines:        lbl.MaxLines,
		Truncator:       lbl.Truncator,
		WrapPolicy:      lbl.WrapPolicy,
		MinWidth:        0,
		MaxWidth:        gtx.Constraints.Max.X,
		Locale:          gtx.Locale,
		LineHeight:      fixed.I(gtx.Sp(lbl.LineHeight)),
		LineHeightScale: lbl.LineHeightScale,
	}, lbl.Text)
	for glyph, more := lbl.Shaper.NextGlyph(); more; glyph, more = lbl.Shaper.NextGlyph() {
		glyphTop := int(glyph.Y) + glyph.Bounds.Min.Y.Floor()
		glyphBottom := int(glyph.Y) + glyph.Bounds.Max.Y.Ceil()
		if glyphBottom <= glyphTop {
			continue
		}
		if !ok || glyphTop < top {
			top = glyphTop
		}
		if !ok || glyphBottom > bottom {
			bottom = glyphBottom
		}
		ok = true
	}
	return top, bottom, ok
}
