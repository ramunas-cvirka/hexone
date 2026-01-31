package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func (ui *UI) layoutTab0(th *material.Theme, gtx layout.Context) layout.Dimensions {
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
							ed.Color = txtColor
							ed.HintColor = hintColor
							ed.TextSize = unit.Sp(15)
							gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
							return ed.Layout(gtx)
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
									return vRule(gtx, unit.Dp(1))
								})
							}),
						)
					}),

					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						// left padding so text isn't glued to the divider
						pad := layout.Inset{Left: unit.Dp(6)}
						return pad.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(th, &ui.RightEd, "Right text...")
							ed.Color = txtColor
							ed.HintColor = hintColor
							ed.TextSize = unit.Sp(15)
							gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
							return ed.Layout(gtx)
						})
					}),
				)
			})
		}),

		// Bottom has fixed/min height, padding, and a Body2 label
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Force a minimum height for this bottom bar
			gtx.Constraints.Min.Y = gtx.Dp(32)

			pad := layout.Inset{
				Left:   unit.Dp(16),
				Right:  unit.Dp(16),
				Top:    unit.Dp(10),
				Bottom: unit.Dp(10),
			}
			return pad.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, ui.LeftInfo)
				lbl.Color = hintColor
				// lbl.Color = ... if you set colors
				return layout.W.Layout(gtx, lbl.Layout)
			})
		}),
	)
}
