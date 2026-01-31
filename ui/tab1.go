package ui

import (
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func (ui *UI) layoutTab1(th *material.Theme, gtx layout.Context) layout.Dimensions {
	dims := layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.tbl.Layout(th, gtx, ui.model)
	})

	toGlyph := func(k key.Name) string {
		switch k {
		case key.NameUpArrow:
			return "↑"
		case key.NameDownArrow:
			return "↓"
		case key.NamePageUp:
			return "⇞"
		case key.NamePageDown:
			return "⇟"
		case key.NameHome:
			return "⇱"
		case key.NameEnd:
			return "⇲"
		case key.NameEnter, key.NameReturn:
			return "⏎"
		default:
			return ""
		}
	}
	repeatable := func(g string) bool {
		return g == "↑" || g == "↓" || g == "⇞" || g == "⇟"
	}

	n := ui.model.Len()

	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameUpArrow},
			key.Filter{Name: key.NameDownArrow},
			key.Filter{Name: key.NamePageUp},
			key.Filter{Name: key.NamePageDown},
			key.Filter{Name: key.NameHome},
			key.Filter{Name: key.NameEnd},
			key.Filter{Name: key.NameEnter},
			key.Filter{Name: key.NameReturn},
		)
		if !ok {
			break
		}

		ke, ok := ev.(key.Event)
		if !ok {
			continue
		}

		g := toGlyph(ke.Name)
		if g == "" {
			continue
		}

		switch ke.State {
		case key.Press:
			// Debounce OS repeats.
			if ui.held[g] {
				continue
			}
			ui.held[g] = true

			// One immediate step.
			ui.tbl.HandleKey(g, n)

			// Start repeat immediately (slow), then accelerate (fast).
			if repeatable(g) {
				ui.rep.active = true
				ui.rep.name = g
				ui.rep.started = gtx.Now
				ui.rep.slow = repeatSlow
				ui.rep.fast = repeatFast
				ui.rep.accelAfter = repeatAccelAfter
				ui.rep.period = ui.rep.slow
				ui.rep.next = gtx.Now.Add(ui.rep.period)
				gtx.Execute(op.InvalidateCmd{At: ui.rep.next})
			} else {
				ui.rep.active = false
			}

		case key.Release:
			ui.held[g] = false
			if ui.rep.active && ui.rep.name == g {
				ui.rep.active = false
			}
		}
	}

	if ui.rep.active {
		// accelerate after a short time
		if gtx.Now.Sub(ui.rep.started) >= ui.rep.accelAfter && ui.rep.period != ui.rep.fast {
			ui.rep.period = ui.rep.fast
			if ui.rep.next.Before(gtx.Now) {
				ui.rep.next = gtx.Now.Add(ui.rep.period)
			}
		}

		if !gtx.Now.Before(ui.rep.next) {
			ui.tbl.HandleKey(ui.rep.name, n)
			ui.rep.next = gtx.Now.Add(ui.rep.period)
		}
		gtx.Execute(op.InvalidateCmd{At: ui.rep.next})
	}

	return dims
}
