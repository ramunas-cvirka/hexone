// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func (ui *UI) fileOpDialogTextSize() unit.Sp {
	// File-operation content should follow the same scaled interface metric as
	// the Save/Cancel action labels used throughout dialogs and Settings.
	return ui.scaleDialogFontSize(10)
}

func (ui *UI) layoutFileOpRow(th *material.Theme, gtx layout.Context, label string, content layout.Widget) layout.Dimensions {
	// "Discovered:" is the widest regular status label. Seventy-two dp keeps
	// it readable at the dialog scale while leaving only a compact value gap.
	labelWidth := gtx.Dp(ui.scaleInterfaceDp(unit.Dp(72)))
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, labelWidth, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label+":")
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.TextSize = ui.fileOpDialogTextSize()
				lbl.Color = color.NRGBA{R: 198, G: 198, B: 198, A: 255}
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			})
		}),
		layout.Flexed(1, content),
	)
}

func (ui *UI) layoutFileOpTextRow(th *material.Theme, gtx layout.Context, label, value string, valueColor color.NRGBA) layout.Dimensions {
	return ui.layoutFileOpRow(th, gtx, label, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Body2(th, value)
		lbl.Font.Typeface = ui.interfaceTypeface()
		lbl.TextSize = ui.fileOpDialogTextSize()
		lbl.Color = valueColor
		lbl.MaxLines = 1
		lbl.Truncator = "…"
		return lbl.Layout(gtx)
	})
}

func fileOpSourceCountText(count int) string {
	if count == 1 {
		return "1 source item"
	}
	return fmt.Sprintf("%d source items", count)
}

func fileOpCountText(count int, singular, plural string) string {
	word := plural
	if count == 1 {
		word = singular
	}
	return formatFileOpCount(count) + " " + word
}

func formatFileOpCount(value int) string {
	text := fmt.Sprintf("%d", value)
	start := 0
	if strings.HasPrefix(text, "-") {
		start = 1
	}
	for i := len(text) - 3; i > start; i -= 3 {
		text = text[:i] + "," + text[i:]
	}
	return text
}

func textCellProgressBar(frac float32, width int) string {
	if width < 1 {
		width = 1
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	done := int(frac*float32(width) + 0.5)
	if done < 0 {
		done = 0
	}
	if done > width {
		done = width
	}
	return strings.Repeat("█", done) + strings.Repeat("░", width-done)
}

func textCellActivityBar(now time.Time, width int) string {
	if width < 1 {
		width = 1
	}
	segment := width / 4
	if segment < 3 {
		segment = 3
	}
	if segment > width {
		segment = width
	}
	const cycle = 1100 * time.Millisecond
	phase := float64(now.UnixNano()%int64(cycle)) / float64(cycle)
	start := int(phase*float64(width+segment)) - segment

	var bar strings.Builder
	for i := 0; i < width; i++ {
		if i >= start && i < start+segment {
			bar.WriteRune('█')
		} else {
			bar.WriteRune('░')
		}
	}
	return bar.String()
}
