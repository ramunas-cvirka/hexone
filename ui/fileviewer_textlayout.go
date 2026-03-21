// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	uitheme "hexone/ui/theme"
)

func viewerLineContentRect(ui *UI, th *material.Theme, gtx layout.Context, face font.Typeface, size unit.Sp, rowH, x0, x1 int) image.Rectangle {
	if rowH <= 0 || x1 <= x0 {
		return image.Rectangle{}
	}
	textH := rowH
	if th != nil && th.Shaper != nil {
		lbl := material.Body2(th, "Mg")
		lbl.Font.Typeface = face
		lbl.Font.Weight = font.Normal
		lbl.TextSize = size
		lbl.MaxLines = 1
		lbl.Truncator = ""
		if measured := measureLabelUnconstrained(gtx, lbl).Size.Y; measured > 0 && measured < textH {
			textH = measured
		}
	}
	if textH < 1 {
		textH = rowH
	}
	offsetY := 0
	if rowH > textH {
		offsetY = (rowH - textH) / 2
	}
	if nudge := uitheme.OpticalTextYOffsetPx(gtx, face, size); nudge > 0 {
		maxExtra := rowH - textH - offsetY
		if maxExtra < 0 {
			maxExtra = 0
		}
		if nudge > maxExtra {
			nudge = maxExtra
		}
		offsetY += nudge
	}
	y1 := offsetY + textH
	if y1 <= offsetY {
		offsetY = 0
		y1 = rowH
	}
	if y1 > rowH {
		y1 = rowH
	}
	return image.Rect(x0, offsetY, x1, y1)
}

func viewerLineSelectionRect(rowH, x0, x1 int) image.Rectangle {
	if rowH <= 0 || x1 <= x0 {
		return image.Rectangle{}
	}
	return image.Rect(x0, 0, x1, rowH)
}
