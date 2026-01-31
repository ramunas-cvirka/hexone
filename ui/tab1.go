package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func (ui *UI) layoutTab1(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if ui.wantFocusTable {
		ui.wantFocusTable = false
		ui.tbl.Focus(gtx) // expose a Focus method that calls FocusCmd
	}
	return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.tbl.Layout(th, gtx, ui.model)
	})
}
