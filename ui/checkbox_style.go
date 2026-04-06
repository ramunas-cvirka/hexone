// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const (
	themeCheckboxSizeDp   = unit.Dp(14)
	themeCheckboxCornerDp = unit.Dp(4)
	themeCheckboxGapDp    = unit.Dp(6)
)

func (ui *UI) layoutThemeCheckbox(th *material.Theme, gtx layout.Context, state *widget.Bool, label string, textSize unit.Sp) layout.Dimensions {
	if state == nil {
		return layout.Dimensions{}
	}
	checked := state.Value
	hovered := state.Hovered()
	focused := gtx.Focused(state)
	boxBg, boxBorder, checkColor, labelColor, frameBg, frameBorder := ui.themeCheckboxColors(checked, hovered, focused, gtx.Enabled())

	return state.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.CheckBox.Add(gtx.Ops)
		if gtx.Enabled() {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutThemeCheckboxBox(gtx, checked, boxBg, boxBorder, checkColor)
			}),
		}
		if label != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Width: themeCheckboxGapDp}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, label)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.Font.Weight = font.Medium
					lbl.TextSize = textSize
					lbl.Color = labelColor
					return lbl.Layout(gtx)
				}),
			)
		}
		return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			dims := layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
			call := macro.Stop()
			if focused && dims.Size.X > 0 && dims.Size.Y > 0 {
				radius := gtx.Dp(unit.Dp(filePaneControlCornerDp - 1))
				if radius < 1 {
					radius = 1
				}
				rr := clip.UniformRRect(image.Rect(0, 0, dims.Size.X, dims.Size.Y), radius)
				if frameBg.A != 0 {
					paint.FillShape(gtx.Ops, frameBg, rr.Op(gtx.Ops))
				}
				if frameBorder.A != 0 {
					paint.FillShape(gtx.Ops, frameBorder, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())
				}
			}
			call.Add(gtx.Ops)
			return dims
		})
	})
}

func (ui *UI) themeCheckboxColors(checked, hovered, focused, enabled bool) (boxBg, boxBorder, checkColor, labelColor, frameBg, frameBorder color.NRGBA) {
	theme := ui.filePanePopupTheme()
	boxBg = mixNRGBA(theme.Bg, theme.ButtonBg, 0.7)
	boxBg.A = 242
	boxBorder = mixNRGBA(theme.Border, theme.Text, 0.14)
	boxBorder.A = 146
	checkColor = theme.ActiveText
	labelColor = theme.Text

	if hovered {
		boxBg = mixNRGBA(boxBg, theme.HoverBg, 0.34)
		boxBg.A = 246
		boxBorder = mixNRGBA(boxBorder, theme.HoverText, 0.2)
		boxBorder.A = 164
		labelColor = mixNRGBA(theme.Text, theme.HoverText, 0.22)
	}
	if checked {
		boxBg = mixNRGBA(theme.ButtonBg, theme.ActiveBg, 0.86)
		boxBg.A = 250
		boxBorder = mixNRGBA(theme.Border, theme.ActiveText, 0.24)
		boxBorder.A = 176
		labelColor = mixNRGBA(labelColor, theme.ActiveText, 0.16)
	}
	if focused {
		frameBg = mixNRGBA(theme.Bg, theme.ActiveBg, 0.62)
		frameBg.A = 28
		frameBorder = mixNRGBA(theme.Border, theme.ActiveText, 0.48)
		frameBorder.A = 110
		boxBg = mixNRGBA(boxBg, theme.ActiveBg, 0.26)
		boxBg.A = 248
		boxBorder = mixNRGBA(boxBorder, theme.ActiveText, 0.42)
		boxBorder.A = 212
		labelColor = mixNRGBA(labelColor, theme.ActiveText, 0.34)
	}
	if !enabled {
		boxBg = mixNRGBA(boxBg, theme.Bg, 0.46)
		boxBg.A = 220
		boxBorder = mixNRGBA(boxBorder, theme.Bg, 0.4)
		boxBorder.A = 118
		checkColor = theme.DisabledText
		labelColor = theme.DisabledText
		frameBg = mixNRGBA(frameBg, theme.Bg, 0.5)
		frameBorder = mixNRGBA(frameBorder, theme.Bg, 0.5)
	}
	return boxBg, boxBorder, checkColor, labelColor, frameBg, frameBorder
}

func (ui *UI) layoutThemeCheckboxBox(gtx layout.Context, checked bool, bg, border, check color.NRGBA) layout.Dimensions {
	size := gtx.Dp(themeCheckboxSizeDp)
	if size < 1 {
		size = 1
	}
	radius := gtx.Dp(themeCheckboxCornerDp)
	rect := image.Rect(0, 0, size, size)
	rr := clip.UniformRRect(rect, radius)
	paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())
	if checked {
		paintThemeCheckboxCheckmark(gtx, size, check)
	}
	return layout.Dimensions{Size: rect.Max}
}

func paintThemeCheckboxCheckmark(gtx layout.Context, size int, col color.NRGBA) {
	if size < 6 || col.A == 0 {
		return
	}
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(float32(size)*0.24, float32(size)*0.55))
	path.LineTo(f32.Pt(float32(size)*0.43, float32(size)*0.74))
	path.LineTo(f32.Pt(float32(size)*0.78, float32(size)*0.31))
	width := float32(size) * 0.12
	if width < 1.6 {
		width = 1.6
	}
	paint.FillShape(gtx.Ops, col, clip.Stroke{
		Path:  path.End(),
		Width: width,
	}.Op())
}
