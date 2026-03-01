package ui

import (
	"hexone/ui/widget/table"
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func (ui *UI) layoutTab1(th *material.Theme, gtx layout.Context) layout.Dimensions {
	dims := layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.layoutFilePanes(th, gtx)
	})

	ui.handleFileManagerKeys(gtx)
	if ui.flushPendingFileOpen() {
		gtx.Execute(op.InvalidateCmd{})
	}

	return dims
}

func (ui *UI) handleFileManagerKeys(gtx layout.Context) {
	filters := ui.fileKeys.Filters()
	if len(filters) == 0 {
		return
	}

	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}

		ke, ok := ev.(key.Event)
		if !ok {
			continue
		}
		action, ok := ui.fileKeys.Resolve(ke)
		if !ok {
			continue
		}
		holdKey := fileActionKey(action)
		command := fileActionCommand(action)

		switch ke.State {
		case key.Press:
			// Debounce OS repeats.
			if ui.held[holdKey] {
				continue
			}
			ui.held[holdKey] = true

			switch action {
			case fileActionFocusNextPane:
				ui.cycleActiveFilePane(1)
				continue
			case fileActionFocusPrevPane:
				ui.cycleActiveFilePane(-1)
				continue
			}

			pane := ui.activePane()
			if pane == nil || pane.table == nil || pane.model == nil {
				ui.held[holdKey] = false
				continue
			}

			handled := pane.table.HandleKey(command, pane.model.Len())
			if !handled {
				ui.held[holdKey] = false
				continue
			}

			// Start repeat immediately (slow), then accelerate (fast).
			if fileActionRepeatable(action) {
				ui.rep.active = true
				ui.rep.pane = ui.activeFilePane
				ui.rep.name = command
				ui.rep.started = gtx.Now
				ui.rep.slow = repeatSlow
				ui.rep.fast = repeatFast
				ui.rep.accelAfter = repeatAccelAfter
				ui.rep.period = ui.rep.slow
				ui.rep.next = gtx.Now.Add(repeatStartDelay)
				gtx.Execute(op.InvalidateCmd{At: ui.rep.next})
			} else {
				ui.rep.active = false
			}

		case key.Release:
			ui.held[holdKey] = false
			if ui.rep.active && ui.rep.name == command {
				ui.rep.active = false
			}
		}
	}

	if ui.rep.active {
		pane := ui.activePane()
		if ui.rep.pane >= 0 && ui.rep.pane < len(ui.filePanes) {
			pane = ui.filePanes[ui.rep.pane]
		}
		if pane == nil || pane.table == nil || pane.model == nil {
			ui.rep.active = false
			return
		}

		// accelerate after a short time
		if gtx.Now.Sub(ui.rep.started) >= ui.rep.accelAfter && ui.rep.period != ui.rep.fast {
			ui.rep.period = ui.rep.fast
			if ui.rep.next.Before(gtx.Now) {
				ui.rep.next = gtx.Now.Add(ui.rep.period)
			}
		}

		if !gtx.Now.Before(ui.rep.next) {
			pane.table.HandleKey(ui.rep.name, pane.model.Len())
			ui.rep.next = gtx.Now.Add(ui.rep.period)
		}
		gtx.Execute(op.InvalidateCmd{At: ui.rep.next})
	}
}

func (ui *UI) layoutFilePanes(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if len(ui.filePanes) == 0 {
		lbl := material.Body1(th, "No panes.")
		lbl.Font.Typeface = "Fira Code"
		lbl.Color = hintColor
		return lbl.Layout(gtx)
	}

	children := make([]layout.FlexChild, 0, len(ui.filePanes)*2)
	for i, pane := range ui.filePanes {
		if pane == nil {
			continue
		}
		if len(children) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout))
		}
		idx := i
		cur := pane
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePane(th, gtx, idx, cur)
		}))
	}

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func (ui *UI) layoutFilePane(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	active := idx == ui.activeFilePane
	if pane.headerClick.Clicked(gtx) {
		ui.setActiveFilePane(idx)
		pane.sortMenuOpen = false
	}

	radius := gtx.Dp(unit.Dp(12))
	border := color.NRGBA{R: 255, G: 255, B: 255, A: 18}
	if active {
		border = color.NRGBA{R: 150, G: 175, B: 240, A: 150}
	}

	return layoutFilePaneChrome(gtx, active, radius, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedBox(gtx, radius,
			color.NRGBA{R: 18, G: 22, B: 30, A: 255},
			border,
			func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return pane.headerClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutFilePaneHeader(th, gtx, idx, pane, active)
							})
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if pane.err == "" {
								return layout.Dimensions{}
							}
							lbl := material.Body2(th, pane.err)
							lbl.Font.Typeface = "Fira Code"
							lbl.Color = color.NRGBA{R: 240, G: 90, B: 90, A: 255}
							lbl.MaxLines = 2
							return lbl.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if pane.err == "" {
								return layout.Dimensions{}
							}
							return layout.Spacer{Height: unit.Dp(2)}.Layout(gtx)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return pane.table.Layout(th, gtx, pane.model)
						}),
					)
				})
			},
		)
	})
}

func (ui *UI) layoutFilePaneHeader(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState, active bool) layout.Dimensions {
	sortOptions := []struct {
		key   fileSortKey
		label string
	}{
		{key: fileSortName, label: "Name"},
		{key: fileSortDate, label: "Date"},
		{key: fileSortExt, label: "Ext"},
		{key: fileSortSize, label: "Size"},
	}
	for i, opt := range sortOptions {
		opt := opt
		if pane.sortOptionBtns[i].Clicked(gtx) {
			ui.choosePaneSort(idx, opt.key)
		}
	}

	title := material.Body2(th, pane.dir)
	title.Font.Typeface = "Fira Code"
	title.Font.Weight = font.Medium
	title.TextSize = unit.Sp(12)
	title.Color = txtColor
	if active {
		title.Color = color.NRGBA{R: 220, G: 230, B: 255, A: 255}
	}
	title.MaxLines = 1
	title.Truncator = "…"

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if !pane.sortMenuOpen {
				return title.Layout(gtx)
			}

			children := make([]layout.FlexChild, 0, len(sortOptions)*2)
			for i, opt := range sortOptions {
				if i > 0 {
					children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout))
				}
				i := i
				activeOpt := pane.sortKey == opt.key
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutModeButton(th, gtx, &pane.sortOptionBtns[i], opt.label, activeOpt)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileModeBadge(th, gtx, idx, pane)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePaneSortBadge(th, gtx, idx, pane)
		}),
	)
}

func (ui *UI) layoutFileModeBadge(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	if pane != nil && pane.modeClick.Clicked(gtx) {
		ui.togglePaneMode(idx)
	}
	if pane == nil {
		return layout.Dimensions{}
	}
	return pane.modeClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{R: 18, G: 22, B: 30, A: 255}
		border := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
		if pane.modeClick.Hovered() {
			bg = color.NRGBA{R: 26, G: 30, B: 38, A: 255}
		}

		width := unit.Dp(30)
		height := unit.Dp(22)
		return fillRoundedBox(gtx, gtx.Dp(unit.Dp(10)), bg, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				iconGtx := gtx
				iconGtx.Constraints.Min = image.Pt(gtx.Dp(width)-gtx.Dp(unit.Dp(12)), gtx.Dp(height)-gtx.Dp(unit.Dp(8)))
				iconGtx.Constraints.Max = iconGtx.Constraints.Min
				return layoutModeGlyph(iconGtx, pane.table.Mode)
			})
		})
	})
}

func layoutModeGlyph(gtx layout.Context, mode table.Mode) layout.Dimensions {
	size := gtx.Constraints.Min
	if size.X <= 0 {
		size.X = gtx.Constraints.Max.X
	}
	if size.Y <= 0 {
		size.Y = gtx.Constraints.Max.Y
	}
	if size.X < 10 {
		size.X = 10
	}
	if size.Y < 10 {
		size.Y = 10
	}

	barColor := color.NRGBA{R: 210, G: 210, B: 210, A: 255}
	barH := 2
	gapY := 2
	if size.Y >= 14 {
		gapY = 3
	}
	top := 1

	drawColumn := func(x, w int) {
		if w < 2 {
			w = 2
		}
		for i := 0; i < 3; i++ {
			y := top + i*(barH+gapY)
			if y+barH > size.Y {
				break
			}
			paint.FillShape(gtx.Ops, barColor, clip.Rect(image.Rect(x, y, x+w, y+barH)).Op())
		}
	}

	if mode == table.ModeBrief {
		colW := (size.X - 3) / 2
		if colW < 3 {
			colW = 3
		}
		drawColumn(0, colW)
		drawColumn(size.X-colW, colW)
		return layout.Dimensions{Size: size}
	}

	drawColumn(0, size.X)
	return layout.Dimensions{Size: size}
}

func (ui *UI) layoutFilePaneSortBadge(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &pane.sortPointerTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok || pe.Kind != pointer.Press {
			continue
		}
		if pe.Buttons.Contain(pointer.ButtonSecondary) {
			ui.setActiveFilePane(idx)
			next := !pane.sortMenuOpen
			ui.closeSortMenusExcept(idx)
			pane.sortMenuOpen = next
		}
	}

	if pane.sortClick.Clicked(gtx) {
		ui.togglePaneSortDirection(idx)
	}

	dims := layoutModeButton(th, gtx, &pane.sortClick, pane.sortBadgeText(), pane.sortMenuOpen)
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}

	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &pane.sortPointerTag)
	pass.Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func layoutFilePaneChrome(gtx layout.Context, active bool, radius int, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	call.Add(gtx.Ops)
	if active {
		rect := image.Rect(0, 0, dims.Size.X, dims.Size.Y)
		rr := clip.UniformRRect(rect, radius)
		paint.FillShape(gtx.Ops, color.NRGBA{R: 185, G: 205, B: 255, A: 120}, clip.Stroke{
			Path:  rr.Path(gtx.Ops),
			Width: 2,
		}.Op())
	}
	return dims
}

func layoutModeButton(th *material.Theme, gtx layout.Context, c *widget.Clickable, label string, active bool) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{R: 18, G: 22, B: 30, A: 255}
		border := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
		if active {
			bg = color.NRGBA{R: 68, G: 92, B: 180, A: 255}
			border = color.NRGBA{R: 120, G: 150, B: 255, A: 90}
		} else if c.Hovered() {
			bg = color.NRGBA{R: 26, G: 30, B: 38, A: 255}
		}

		return fillRoundedBox(gtx, gtx.Dp(unit.Dp(10)), bg, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(10), Right: unit.Dp(10), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = "Fira Code"
				lbl.Font.Weight = font.Medium
				lbl.TextSize = unit.Sp(12)
				lbl.Color = txtColor
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			})
		})
	})
}
