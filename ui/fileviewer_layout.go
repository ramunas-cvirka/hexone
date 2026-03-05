package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func (ui *UI) layoutFileViewer(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.fileViewer
	if st == nil {
		return layout.Dimensions{}
	}
	if st.commandEditOn {
		for {
			ev, ok := st.commandEditor.Update(gtx)
			if !ok {
				break
			}
			if submit, ok := ev.(widget.SubmitEvent); ok {
				st.commandEditor.SetText(submit.Text)
				ui.applyViewerCommandEdit(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
		}
	}
	if st.commandClick.Clicked(gtx) {
		ui.startViewerCommandEdit()
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.commandFocus {
		st.commandFocus = false
		gtx.Execute(key.FocusCmd{Tag: &st.commandEditor})
	}

	for st.backdropClick.Clicked(gtx) {
	}
	if st.closeClick.Clicked(gtx) {
		ui.closeFileViewer()
		return layout.Dimensions{}
	}
	if st.refreshClick.Clicked(gtx) {
		ui.startFileViewerLoad(gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.loading {
		// Keep frames ticking while background load is running, otherwise
		// results can remain pending until an external event (e.g. resize).
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(33 * time.Millisecond)})
	}

	ui.scheduleFileViewerWatch(gtx)

	return st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{R: 14, G: 18, B: 24, A: 252}, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFileViewerHeader(th, gtx, st)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if st.err == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, st.err)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleThemeFontSize(th, 10)
					lbl.Color = color.NRGBA{R: 255, G: 170, B: 170, A: 255}
					lbl.MaxLines = 2
					lbl.Truncator = "..."
					return lbl.Layout(gtx)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
					ed := material.Editor(th, &st.contentEditor, "")
					ed.Font.Typeface = ui.mainTypeface()
					ed.TextSize = ui.viewerTextSize()
					ed.Color = color.NRGBA{R: 220, G: 226, B: 240, A: 255}
					ed.HintColor = hintColor
					return fillRoundedBox(
						gtx,
						gtx.Dp(unit.Dp(filePaneControlCornerDp)),
						color.NRGBA{R: 18, G: 24, B: 34, A: 255},
						color.NRGBA{R: 255, G: 255, B: 255, A: 20},
						func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
							return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
								if st.loading && st.content == "" {
									wait := material.Body2(th, "Loading...")
									wait.Font.Typeface = ui.mainTypeface()
									wait.TextSize = ui.viewerTextSize()
									wait.Color = hintColor
									return wait.Layout(gtx)
								}
								gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
								return ed.Layout(gtx)
							})
						},
					)
				})
			}),
		)
	})
}

func (ui *UI) layoutFileViewerHeader(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	title := st.name
	if title == "" {
		title = st.path
	}
	if title == "" {
		title = "viewer"
	}
	status := st.status
	if status == "" {
		status = "ready"
	}
	if st.loading {
		status = "loading..."
	}
	summary := title + " | " + status

	return fillRoundedBox(
		gtx,
		0,
		color.NRGBA{R: 20, G: 26, B: 38, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 24},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, summary)
								lbl.Font.Typeface = ui.mainTypeface()
								lbl.Font.Weight = font.Medium
								lbl.TextSize = scaleThemeFontSize(th, 10)
								lbl.Color = color.NRGBA{R: 210, G: 224, B: 250, A: 255}
								lbl.MaxLines = 1
								lbl.Truncator = "..."
								return lbl.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutFileViewerInlineCommand(th, gtx, st)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layoutTinyIconModeButton(th, gtx, &st.refreshClick, uiRefreshGlyphIcon(), false)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layoutTinyIconModeButton(th, gtx, &st.closeClick, uiCloseIcon(), false)
					}),
				)
			})
		},
	)
}

func (ui *UI) layoutFileViewerInlineCommand(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	if st == nil || st.mode != "command" {
		return layout.Dimensions{}
	}
	fg := color.NRGBA{R: 245, G: 231, B: 180, A: 255}
	bg := color.NRGBA{R: 200, G: 170, B: 92, A: 16}
	if st.commandClick.Hovered() || st.commandEditOn {
		bg = color.NRGBA{R: 200, G: 170, B: 92, A: 34}
	}
	if st.commandEditOn {
		w := gtx.Dp(unit.Dp(240))
		if w < 96 {
			w = 96
		}
		if gtx.Constraints.Max.X > 0 && w > gtx.Constraints.Max.X {
			w = gtx.Constraints.Max.X
		}
		return fixedWidth(gtx, w, func(gtx layout.Context) layout.Dimensions {
			ed := material.Editor(th, &st.commandEditor, "cat {fullpath}")
			ed.Font.Typeface = ui.mainTypeface()
			ed.TextSize = scaleThemeFontSize(th, 9)
			ed.Color = fg
			ed.HintColor = color.NRGBA{R: 160, G: 150, B: 120, A: 255}
			return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, ed.Layout)
			})
		})
	}

	label := " | " + st.command + " |"
	return st.commandClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(3), Right: unit.Dp(3), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleThemeFontSize(th, 9)
				lbl.Font.Weight = font.Medium
				lbl.Color = fg
				lbl.MaxLines = 1
				lbl.Truncator = "..."
				return lbl.Layout(gtx)
			})
		})
	})
}
