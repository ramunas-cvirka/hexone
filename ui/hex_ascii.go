package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func (ui *UI) layoutTab0(th *material.Theme, gtx layout.Context) layout.Dimensions {
	theme := ui.hexASCIITabTheme()
	return fillFilePaneBox(gtx, theme.Bg, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				outer := layout.UniformInset(unit.Dp(12))
				return outer.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					// two editors, with a gutter between them that includes a vertical rule
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							// right padding so text isn't glued to the divider
							pad := layout.Inset{Right: unit.Dp(6)}
							return pad.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(th, &ui.LeftEd, "Left text...")
								ed.Font.Typeface = theme.Typeface
								ed.Color = theme.Text
								ed.HintColor = theme.Hint
								ed.TextSize = theme.TextSize
								gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
								return ui.layoutEditorWithContextMenu(th, gtx, "tab0-left", &ui.LeftEd, true, func(gtx layout.Context) layout.Dimensions {
									return layoutHexASCIIEditorBox(gtx, theme, gtx.Focused(&ui.LeftEd), ed.Layout)
								})
							})
						}),

						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							// gutter width between columns
							return layout.Stack{}.Layout(gtx,
								layout.Expanded(func(gtx layout.Context) layout.Dimensions {
									return layout.Spacer{Width: unit.Dp(14)}.Layout(gtx)
								}),
								layout.Stacked(func(gtx layout.Context) layout.Dimensions {
									// vertical line centered within gutter
									return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return vRuleColor(gtx, unit.Dp(1), theme.Divider)
									})
								}),
							)
						}),

						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							// left padding so text isn't glued to the divider
							pad := layout.Inset{Left: unit.Dp(6)}
							return pad.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(th, &ui.RightEd, "Right text...")
								ed.Font.Typeface = theme.Typeface
								ed.Color = theme.Text
								ed.HintColor = theme.Hint
								ed.TextSize = theme.TextSize
								gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
								return ui.layoutEditorWithContextMenu(th, gtx, "tab0-right", &ui.RightEd, true, func(gtx layout.Context) layout.Dimensions {
									return layoutHexASCIIEditorBox(gtx, theme, gtx.Focused(&ui.RightEd), ed.Layout)
								})
							})
						}),
					)
				})
			}),

			// Bottom has fixed/min height, padding, and a Body2 label
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = gtx.Dp(32)
				pad := layout.Inset{
					Left:   unit.Dp(16),
					Right:  unit.Dp(16),
					Top:    unit.Dp(10),
					Bottom: unit.Dp(10),
				}
				return pad.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, ui.LeftInfo)
					lbl.Font.Typeface = theme.Typeface
					lbl.TextSize = scaleFontSize(theme.TextSize, 12)
					lbl.Color = theme.Hint
					return layout.W.Layout(gtx, lbl.Layout)
				})
			}),
		)
	})
}

type hexASCIITabTheme struct {
	Typeface font.Typeface
	TextSize unit.Sp
	Bg       color.NRGBA
	Text     color.NRGBA
	Hint     color.NRGBA
	Divider  color.NRGBA
	EditorBg color.NRGBA
	Border   color.NRGBA
	Accent   color.NRGBA
}

func (ui *UI) hexASCIITabTheme() hexASCIITabTheme {
	palette := filePanePaletteFromConfig(ui.fmCfg)
	divider := filePaneActiveBorderColor(palette.PaneBg)
	divider.A = 52
	border := mixNRGBA(palette.PaneBg, palette.CurrentDirFg, 0.22)
	border.A = 38
	accent := mixNRGBA(palette.CurrentDirFg, palette.PaneFg, 0.26)
	accent.A = 214
	editorBg := mixNRGBA(palette.PaneBg, palette.CurrentDirBg, 0.14)
	editorBg.A = 255
	return hexASCIITabTheme{
		Typeface: ui.mainTypeface(),
		TextSize: scaleConfigFontSize(ui.fmCfg, 13),
		Bg:       palette.PaneBg,
		Text:     palette.PaneFg,
		Hint:     filePanePathMutedColor(palette),
		Divider:  divider,
		EditorBg: editorBg,
		Border:   border,
		Accent:   accent,
	}
}

func vRuleColor(gtx layout.Context, w unit.Dp, c color.NRGBA) layout.Dimensions {
	width := gtx.Dp(w)
	h := gtx.Constraints.Max.Y
	if h < 1 {
		h = 1
	}
	r := image.Rect(0, 0, width, h)
	paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
	return layout.Dimensions{Size: image.Pt(width, h)}
}

func layoutHexASCIIEditorBox(gtx layout.Context, theme hexASCIITabTheme, focused bool, inner layout.Widget) layout.Dimensions {
	bg := theme.EditorBg
	border := theme.Border
	underline := theme.Border
	underlineH := 1
	if focused {
		border = theme.Accent
		underline = theme.Accent
		underlineH = 2
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
