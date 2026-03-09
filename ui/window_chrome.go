package ui

import (
	"image"
	"image/color"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const windowChromeHeightDp = unit.Dp(30)

func (ui *UI) SetWindowChrome(enabled bool, mode app.WindowMode) {
	if ui == nil {
		return
	}
	ui.windowChromeEnabled = enabled
	ui.windowMode = mode
}

func (ui *UI) requestWindowAction(action system.Action) {
	if ui == nil {
		return
	}
	ui.requestedWindowActions |= action
}

func (ui *UI) ConsumeWindowActions() system.Action {
	if ui == nil {
		return 0
	}
	actions := ui.requestedWindowActions
	ui.requestedWindowActions = 0
	return actions
}

func (ui *UI) layoutWindowChrome(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if ui == nil || !ui.windowChromeEnabled || ui.windowMode == app.Fullscreen {
		return layout.Dimensions{}
	}
	for ui.windowMinimizeClick.Clicked(gtx) {
		ui.requestWindowAction(system.ActionMinimize)
	}
	for ui.windowMaximizeClick.Clicked(gtx) {
		if ui.windowMode == app.Maximized {
			ui.requestWindowAction(system.ActionUnmaximize)
		} else {
			ui.requestWindowAction(system.ActionMaximize)
		}
	}
	for ui.windowCloseClick.Clicked(gtx) {
		ui.requestWindowClose()
	}

	barH := gtx.Dp(windowChromeHeightDp)
	if barH < 1 {
		barH = 1
	}
	return fillRoundedBox(
		gtx,
		0,
		color.NRGBA{R: 16, G: 20, B: 26, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 20},
		func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, barH, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						dragRect := image.Rect(0, 0, gtx.Constraints.Max.X, barH)
						defer clip.Rect(dragRect).Push(gtx.Ops).Pop()
						system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
						return layout.Inset{Left: unit.Dp(10), Right: unit.Dp(8), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body2(th, "hexone")
							lbl.Font.Typeface = ui.mainTypeface()
							lbl.Font.Weight = font.Medium
							lbl.TextSize = scaleThemeFontSize(th, 11)
							lbl.Color = color.NRGBA{R: 228, G: 234, B: 245, A: 255}
							lbl.MaxLines = 1
							return layoutVCenteredLabel(gtx, lbl)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: unit.Dp(6), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layoutWindowChromeButton(th, gtx, ui.mainTypeface(), &ui.windowMinimizeClick, "min", false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										label := "max"
										if ui.windowMode == app.Maximized {
											label = "restore"
										}
										return layoutWindowChromeButton(th, gtx, ui.mainTypeface(), &ui.windowMaximizeClick, label, false)
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return layoutWindowChromeButton(th, gtx, ui.mainTypeface(), &ui.windowCloseClick, "x", true)
									})
								}),
							)
						})
					}),
				)
			})
		},
	)
}

func layoutWindowChromeButton(th *material.Theme, gtx layout.Context, typeface font.Typeface, c *widget.Clickable, label string, closeButton bool) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		bg := color.NRGBA{R: 24, G: 29, B: 38, A: 255}
		border := color.NRGBA{R: 255, G: 255, B: 255, A: 18}
		labelColor := color.NRGBA{R: 218, G: 224, B: 236, A: 255}
		if closeButton {
			bg = color.NRGBA{R: 46, G: 24, B: 28, A: 255}
			border = color.NRGBA{R: 255, G: 120, B: 130, A: 36}
			labelColor = color.NRGBA{R: 244, G: 220, B: 224, A: 255}
		}
		if c.Hovered() {
			if closeButton {
				bg = color.NRGBA{R: 118, G: 38, B: 48, A: 255}
				border = color.NRGBA{R: 255, G: 170, B: 176, A: 80}
				labelColor = color.NRGBA{R: 255, G: 240, B: 242, A: 255}
			} else {
				bg = color.NRGBA{R: 44, G: 54, B: 76, A: 255}
				border = color.NRGBA{R: 140, G: 160, B: 255, A: 70}
				labelColor = color.NRGBA{R: 238, G: 244, B: 255, A: 255}
			}
		}
		return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), bg, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = typeface
				lbl.Font.Weight = font.Medium
				lbl.TextSize = scaleThemeFontSize(th, 10)
				lbl.Color = labelColor
				lbl.MaxLines = 1
				return layoutVCenteredLabel(gtx, lbl)
			})
		})
	})
}
