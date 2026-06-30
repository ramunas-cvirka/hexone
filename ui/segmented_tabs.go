// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type slidingTabSpec struct {
	Label      string
	Typeface   font.Typeface
	Click      *widget.Clickable
	ActiveFill float32
	HoverFill  float32
	PulseFill  float32
	FocusFill  float32
	Disabled   bool
}

func float32Abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func slidingStripTextColor(activeFill, hoverFill, pulseFill float32) color.NRGBA {
	fg := mixNRGBA(txtColor, color.NRGBA{R: 236, G: 236, B: 236, A: 255}, clamp01(activeFill))
	fg = mixNRGBA(fg, color.NRGBA{R: 244, G: 244, B: 244, A: 255}, clamp01(hoverFill*0.7+pulseFill*0.25))
	return fg
}

func (ui *UI) slidingTabWidths(th *material.Theme, gtx layout.Context, textSize unit.Sp, specs []slidingTabSpec) []int {
	widths := make([]int, len(specs))
	total := 0
	padding := gtx.Dp(unit.Dp(10)) * 2
	minWidth := gtx.Dp(unit.Dp(30))
	for i, spec := range specs {
		lbl := material.Body2(th, spec.Label)
		lbl.Font.Typeface = ui.slidingTabTypeface(spec)
		lbl.Font.Weight = font.Medium
		lbl.TextSize = textSize
		lbl.MaxLines = 1
		w := measureLabelUnconstrained(gtx, lbl).Size.X + padding
		if w < minWidth {
			w = minWidth
		}
		widths[i] = w
		total += w
	}
	if max := gtx.Constraints.Max.X; max > 0 && total > max {
		scaled := 0
		for i := range widths {
			w := widths[i] * max / total
			if w < 1 {
				w = 1
			}
			widths[i] = w
			scaled += w
		}
		for i := len(widths) - 1; i >= 0 && scaled < max; i-- {
			widths[i]++
			scaled++
		}
	}
	return widths
}

func (ui *UI) slidingTabTypeface(spec slidingTabSpec) font.Typeface {
	if spec.Typeface != "" {
		return spec.Typeface
	}
	return ui.mainTypeface()
}

func (ui *UI) layoutSlidingTabStrip(th *material.Theme, gtx layout.Context, stripH int, sliderPos float32, textSize unit.Sp, specs []slidingTabSpec) layout.Dimensions {
	if ui == nil || len(specs) == 0 {
		return layout.Dimensions{}
	}
	if stripH < 1 {
		stripH = 1
	}
	if sliderPos < 0 {
		sliderPos = 0
	}
	last := float32(len(specs) - 1)
	if sliderPos > last {
		sliderPos = last
	}
	widths := ui.slidingTabWidths(th, gtx, textSize, specs)
	starts := make([]int, len(widths))
	totalW := 0
	for i, w := range widths {
		starts[i] = totalW
		totalW += w
	}
	if totalW < len(specs) {
		totalW = len(specs)
	}

	return fixedWidth(gtx, totalW, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneControlCornerDp)),
			color.NRGBA{R: 24, G: 24, B: 24, A: 255},
			color.NRGBA{R: 255, G: 255, B: 255, A: 22},
			func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
						w := totalW
						innerR := gtx.Dp(unit.Dp(filePaneControlCornerDp - 1))
						if innerR < 1 {
							innerR = 1
						}

						baseIdx := int(sliderPos)
						if baseIdx < 0 {
							baseIdx = 0
						}
						if baseIdx > len(widths)-1 {
							baseIdx = len(widths) - 1
						}
						nextIdx := baseIdx + 1
						if nextIdx > len(widths)-1 {
							nextIdx = len(widths) - 1
						}
						frac := sliderPos - float32(baseIdx)
						sliderX := int(float32(starts[baseIdx]) + float32(starts[nextIdx]-starts[baseIdx])*frac)
						sliderW := int(float32(widths[baseIdx]) + float32(widths[nextIdx]-widths[baseIdx])*frac)
						if sliderW < 1 {
							sliderW = 1
						}
						sliderRect := image.Rect(sliderX, 0, sliderX+sliderW, stripH)

						innerClip := clip.UniformRRect(image.Rect(0, 0, w, stripH), innerR).Push(gtx.Ops)
						paint.FillShape(gtx.Ops, color.NRGBA{R: 54, G: 54, B: 54, A: 255}, clip.UniformRRect(sliderRect, innerR).Op(gtx.Ops))

						children := make([]layout.FlexChild, 0, len(specs))
						for i, spec := range specs {
							i := i
							spec := spec
							segW := widths[i]
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, segW, func(gtx layout.Context) layout.Dimensions {
									return spec.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										activeFill := clamp01(spec.ActiveFill)
										hoverFill := clamp01(spec.HoverFill)
										pulseFill := clamp01(spec.PulseFill)
										focusFill := clamp01(spec.FocusFill)
										if spec.Click.Pressed() && pulseFill < 0.5 {
											pulseFill = 0.5
										}
										proximity := clamp01(1 - float32Abs(float32(i)-sliderPos))
										labelActive := clamp01(activeFill*0.8 + proximity*0.45)
										bg := color.NRGBA{}
										bg = mixNRGBA(bg, color.NRGBA{R: 255, G: 255, B: 255, A: 8}, hoverFill*(1-labelActive)*0.45)
										bg = mixNRGBA(bg, color.NRGBA{R: 255, G: 255, B: 255, A: 14}, pulseFill*0.18)
										bg = mixNRGBA(bg, color.NRGBA{R: 212, G: 196, B: 164, A: 28}, focusFill*0.34)
										fg := slidingStripTextColor(labelActive, hoverFill, pulseFill)
										fg = mixNRGBA(fg, color.NRGBA{R: 248, G: 242, B: 228, A: 255}, focusFill*0.24)
										if spec.Disabled {
											bg = color.NRGBA{}
											fg = mixNRGBA(fg, color.NRGBA{R: 24, G: 24, B: 24, A: 255}, 0.55)
										}
										dims := fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
											lbl := material.Body2(th, spec.Label)
											lbl.Font.Typeface = ui.slidingTabTypeface(spec)
											lbl.Font.Weight = font.Medium
											lbl.TextSize = textSize
											lbl.Color = fg
											lbl.MaxLines = 1
											lbl.Alignment = text.Middle
											dims := layoutVCenteredLabel(gtx, lbl)
											defer clip.Rect(image.Rectangle{Max: image.Pt(segW, stripH)}).Push(gtx.Ops).Pop()
											if !spec.Disabled {
												pointer.CursorPointer.Add(gtx.Ops)
											}
											return dims
										})
										if focusFill > 0 && dims.Size.X > 0 && dims.Size.Y > 0 {
											pad := gtx.Dp(unit.Dp(5))
											if pad*2 >= dims.Size.X {
												pad = 0
											}
											h := gtx.Dp(unit.Dp(2))
											if h < 1 {
												h = 1
											}
											rect := image.Rect(pad, dims.Size.Y-h, dims.Size.X-pad, dims.Size.Y)
											if rect.Dx() > 0 && rect.Dy() > 0 {
												paint.FillShape(gtx.Ops, color.NRGBA{R: 214, G: 198, B: 166, A: uint8(140 + 64*focusFill)}, clip.UniformRRect(rect, h).Op(gtx.Ops))
											}
										}
										return dims
									})
								})
							}))
						}
						dims := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
						innerClip.Pop()
						return dims
					})
				})
			},
		)
	})
}
