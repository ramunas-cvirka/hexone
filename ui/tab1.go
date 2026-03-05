package ui

import (
	"hexone/ui/widget/table"
	"image"
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const (
	filePaneWheelRange            = 1 << 30
	filePanePathDoubleClickWindow = 450 * time.Millisecond
	filePanePathClickDelay        = filePanePathDoubleClickWindow
	filePaneFavoriteTooltipDelay  = 2 * time.Second
	filePaneCornerDp              = 8
	filePaneControlCornerDp       = 6
	filePaneOverlayCornerDp       = 6
)

type visiblePane struct {
	idx  int
	pane *filePaneState
}

func (ui *UI) layoutTab1(th *material.Theme, gtx layout.Context) layout.Dimensions {
	ui.pumpFileViewerState(gtx)
	ui.pumpFileCopyState(gtx)
	ui.pumpFileDeleteState(gtx)

	dims := layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFilePanes(th, gtx)
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileCopyDialog(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileDeleteDialog(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileViewer(th, gtx)
		}),
	)

	ui.handleFileManagerKeys(gtx)
	if ui.flushPendingFileOpen() {
		gtx.Execute(op.InvalidateCmd{})
	}

	return dims
}

func (ui *UI) handleFileManagerKeys(gtx layout.Context) {
	if ui.settingsModal != nil || ui.sshModal != nil {
		return
	}
	if ui.fileViewer != nil {
		ui.handleFileViewerKeys(gtx)
		return
	}
	if ui.fileCopy != nil || ui.fileDelete != nil {
		return
	}
	ui.handleFileManagerEscape(gtx)
	if ui.pathEditActive() {
		return
	}

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
			case fileActionView:
				ui.startFileViewer(ui.activeFilePane, gtx.Now)
				ui.rep.active = false
				continue
			case fileActionCopy:
				ui.startFileCopyDialog(ui.activeFilePane, gtx.Now)
				ui.rep.active = false
				continue
			case fileActionDelete:
				ui.startFileDeleteDialog(ui.activeFilePane, gtx.Now)
				ui.rep.active = false
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

func (ui *UI) handleFileManagerEscape(gtx layout.Context) {
	closedAny := false
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		for _, pane := range ui.filePanes {
			if pane == nil {
				continue
			}
			closed := false
			if pane.favoriteMenuOpen {
				pane.closeFavoriteMenu()
				closed = true
			}
			if pane.sortMenuOpen {
				pane.sortMenuOpen = false
				closed = true
			}
			if pane.ctxMenuOpen {
				pane.closeContextMenu()
				closed = true
			}
			if closed {
				closedAny = true
			}
		}
	}
	if closedAny {
		ui.rep.active = false
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) layoutFilePanes(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if len(ui.filePanes) == 0 {
		lbl := material.Body1(th, "No panes.")
		lbl.Font.Typeface = ui.mainTypeface()
		lbl.Color = hintColor
		return lbl.Layout(gtx)
	}

	visible := make([]visiblePane, 0, len(ui.filePanes))
	for i, pane := range ui.filePanes {
		if pane == nil {
			continue
		}
		visible = append(visible, visiblePane{idx: i, pane: pane})
	}
	if len(visible) == 0 {
		return layout.Dimensions{}
	}

	children := make([]layout.FlexChild, 0, len(visible))
	for pos, item := range visible {
		roundLeft := pos == 0
		roundRight := pos == len(visible)-1
		drawLeftBorder := pos == 0
		drawRightBorder := pos == len(visible)-1
		idx := item.idx
		cur := item.pane
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePane(th, gtx, idx, cur, roundLeft, roundRight, drawLeftBorder, drawRightBorder)
		}))
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			ui.layoutFilePaneSeams(gtx, visible)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
	)
}

func paneColumnWidths(total, count int) []int {
	if count <= 0 {
		return nil
	}
	if total < 0 {
		total = 0
	}
	base := total / count
	rem := total % count
	out := make([]int, count)
	for i := 0; i < count; i++ {
		w := base
		if i < rem {
			w++
		}
		out[i] = w
	}
	return out
}

func drawVLine(gtx layout.Context, x, h int, c color.NRGBA) {
	if x < 0 || x >= gtx.Constraints.Max.X || h <= 0 || c.A == 0 {
		return
	}
	paint.FillShape(gtx.Ops, c, clip.Rect(image.Rect(x, 0, x+1, h)).Op())
}

func (ui *UI) layoutFilePaneSeams(gtx layout.Context, visible []visiblePane) {
	if len(visible) <= 1 {
		return
	}
	widths := paneColumnWidths(gtx.Constraints.Max.X, len(visible))
	x := 0
	height := gtx.Constraints.Max.Y
	base := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
	highlight := color.NRGBA{R: 150, G: 175, B: 240, A: 150}
	for i := 0; i < len(visible)-1; i++ {
		x += widths[i]
		if x <= 0 || x >= gtx.Constraints.Max.X {
			continue
		}
		col := base
		if visible[i].idx == ui.activeFilePane || visible[i+1].idx == ui.activeFilePane {
			col = highlight
		}
		drawVLine(gtx, x, height, col)
	}
}

func (ui *UI) layoutFilePane(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState, roundLeft, roundRight, drawLeftBorder, drawRightBorder bool) layout.Dimensions {
	active := idx == ui.activeFilePane

	radius := gtx.Dp(unit.Dp(filePaneCornerDp))
	border := color.NRGBA{R: 255, G: 255, B: 255, A: 18}
	if active {
		border = color.NRGBA{R: 150, G: 175, B: 240, A: 150}
	}

	return layoutFilePaneChrome(gtx, active, radius, roundLeft, roundRight, drawLeftBorder, drawRightBorder, func(gtx layout.Context) layout.Dimensions {
		return fillFilePaneBox(gtx, radius, roundLeft, roundRight, drawLeftBorder, drawRightBorder,
			color.NRGBA{R: 18, G: 22, B: 30, A: 255},
			border,
			func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{}.Layout(gtx,
						layout.Expanded(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									dims := ui.layoutFilePaneHeader(th, gtx, idx, pane, active)
									pane.headerHeight = dims.Size.Y
									return dims
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if pane.err == "" {
										return layout.Dimensions{}
									}
									lbl := material.Body2(th, pane.err)
									lbl.Font.Typeface = ui.mainTypeface()
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
									return ui.layoutFilePaneBody(th, gtx, idx, pane)
								}),
							)
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutFilePaneFavoriteMenu(th, gtx, idx, pane)
						}),
					)
				})
			},
		)
	})
}

func (ui *UI) layoutFilePaneBody(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePaneTable(th, gtx, idx, pane)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePaneNotice(th, gtx, pane)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePaneContextMenu(th, gtx, pane)
		}),
	)
}

func (ui *UI) layoutFilePaneNotice(th *material.Theme, gtx layout.Context, pane *filePaneState) layout.Dimensions {
	if pane == nil || pane.noticeText == "" {
		return layout.Dimensions{}
	}
	if !gtx.Now.Before(pane.noticeUntil) {
		pane.noticeText = ""
		pane.noticeUntil = time.Time{}
		return layout.Dimensions{}
	}

	gtx.Execute(op.InvalidateCmd{At: pane.noticeUntil})
	return layout.Inset{Top: unit.Dp(4), Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.NW.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return fillRoundedBox(
				gtx,
				gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
				color.NRGBA{R: 56, G: 20, B: 20, A: 242},
				color.NRGBA{R: 170, G: 70, B: 70, A: 180},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, pane.noticeText)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleThemeFontSize(th, 12)
						lbl.Color = color.NRGBA{R: 255, G: 180, B: 180, A: 255}
						lbl.MaxLines = 2
						lbl.Truncator = "…"
						return lbl.Layout(gtx)
					})
				},
			)
		})
	})
}

func (ui *UI) layoutFilePaneContextMenu(th *material.Theme, gtx layout.Context, pane *filePaneState) layout.Dimensions {
	if pane == nil || !pane.ctxMenuOpen {
		return layout.Dimensions{}
	}

	spec := pane.contextMenuSpec()
	pane.ensureContextMenuClicks(len(spec.items))
	for i, label := range spec.items {
		if pane.ctxMenuClicks[i].Clicked(gtx) {
			pane.closeContextMenu()
			pane.setNotice(label+" is not implemented yet", gtx.Now)
		}
	}

	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &pane.ctxPointerTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok || pe.Kind != pointer.Press {
			continue
		}
		if !pe.Buttons.Contain(pointer.ButtonPrimary) {
			continue
		}
		pos := pe.Position.Round()
		if pane.ctxMenuRect.Dx() <= 0 || pane.ctxMenuRect.Dy() <= 0 ||
			pos.X < pane.ctxMenuRect.Min.X || pos.X >= pane.ctxMenuRect.Max.X ||
			pos.Y < pane.ctxMenuRect.Min.Y || pos.Y >= pane.ctxMenuRect.Max.Y {
			pane.closeContextMenu()
		}
	}

	if !pane.ctxMenuOpen {
		return layout.Dimensions{}
	}

	m := op.Record(gtx.Ops)
	menuDims := ui.layoutFilePaneContextMenuCard(th, gtx, pane, spec)
	call := m.Stop()

	anchor := clampFilePaneMenuPoint(pane.ctxMenuPos, menuDims.Size, gtx.Constraints.Max)
	pane.ctxMenuRect = image.Rectangle{Min: anchor, Max: anchor.Add(menuDims.Size)}

	bodyClip := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	offset := op.Offset(anchor).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	bodyClip.Pop()

	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &pane.ctxPointerTag)
	pass.Pop()

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *UI) layoutFilePaneContextMenuCard(th *material.Theme, gtx layout.Context, pane *filePaneState, spec fileContextMenuSpec) layout.Dimensions {
	const menuWidth = 172
	width := gtx.Dp(unit.Dp(menuWidth))
	if width > gtx.Constraints.Max.X {
		width = gtx.Constraints.Max.X
	}
	if width < 1 {
		width = 1
	}

	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			color.NRGBA{R: 20, G: 24, B: 34, A: 250},
			color.NRGBA{R: 255, G: 255, B: 255, A: 22},
			func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(spec.items)+3)
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(5), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, spec.title)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleThemeFontSize(th, 10)
						lbl.Color = color.NRGBA{R: 170, G: 180, B: 205, A: 255}
						lbl.MaxLines = 1
						lbl.Truncator = "…"
						lbl.Font.Weight = font.Medium
						return lbl.Layout(gtx)
					})
				}))
				children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout))
				for i, label := range spec.items {
					i := i
					itemLabel := label
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layoutFilePaneContextMenuItem(th, gtx, ui.mainTypeface(), &pane.ctxMenuClicks[i], itemLabel)
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			},
		)
	})
}

func layoutFilePaneContextMenuItem(th *material.Theme, gtx layout.Context, typeface font.Typeface, click *widget.Clickable, label string) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{}
		if click.Hovered() {
			bg = color.NRGBA{R: 68, G: 92, B: 180, A: 54}
		}
		return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = typeface
				lbl.TextSize = scaleThemeFontSize(th, 11)
				lbl.Font.Weight = font.Medium
				lbl.Color = txtColor
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			})
		})
	})
}

func clampFilePaneMenuPoint(anchor, size, bounds image.Point) image.Point {
	if anchor.X < 0 {
		anchor.X = 0
	}
	if anchor.Y < 0 {
		anchor.Y = 0
	}
	if size.X >= bounds.X {
		anchor.X = 0
	} else if anchor.X+size.X > bounds.X {
		anchor.X = bounds.X - size.X
	}
	if size.Y >= bounds.Y {
		anchor.Y = 0
	} else if anchor.Y+size.Y > bounds.Y {
		anchor.Y = bounds.Y - size.Y
	}
	if anchor.X < 0 {
		anchor.X = 0
	}
	if anchor.Y < 0 {
		anchor.Y = 0
	}
	return anchor
}

func (ui *UI) layoutFilePaneTable(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	if pane == nil || pane.table == nil || pane.model == nil {
		return layout.Dimensions{}
	}

	total := pane.model.Len()
	dims := pane.table.Layout(th, gtx, pane.model)

	selectionChanged := false
	pathEditClosed := false
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &pane.tablePointerTag,
			Kinds:  pointer.Scroll | pointer.Press,
			ScrollY: pointer.ScrollRange{
				Min: -filePaneWheelRange,
				Max: filePaneWheelRange,
			},
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		if idx != ui.activeFilePane {
			ui.setActiveFilePane(idx)
		}
		pane.clearPendingPathNavigate()
		switch pe.Kind {
		case pointer.Scroll:
			if pe.Scroll.Y == 0 {
				continue
			}
			pane.clearPathClickState()
			if pane.pathEditing {
				pane.stopPathEdit()
				pathEditClosed = true
			}
			if pane.table.HandleScrollSelection(pe.Scroll.Y, total) {
				selectionChanged = true
			}
		case pointer.Press:
			pane.clearPathClickState()
			if pane.pathEditing {
				pane.stopPathEdit()
				pathEditClosed = true
			}
			if !pe.Buttons.Contain(pointer.ButtonSecondary) {
				continue
			}
			row := pane.table.HitRow(pe.Position.Round(), total)
			if row >= 0 && row != pane.table.Selected {
				prev := pane.table.Selected
				pane.table.SetSelected(row, total, false)
				if pane.table.OnSelect != nil && prev != pane.table.Selected {
					pane.table.OnSelect(pane.table.Selected)
				}
				selectionChanged = true
			}
			ui.openFilePaneContextMenu(idx, row, pe.Position.Round())
		}
	}

	if selectionChanged {
		gtx.Execute(op.InvalidateCmd{})
	}
	if pathEditClosed {
		gtx.Execute(op.InvalidateCmd{})
	}
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}

	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &pane.tablePointerTag)
	pass.Pop()
	return dims
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

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if pane.pathEditing {
				return ui.layoutFilePanePathEditor(th, gtx, idx, pane, active)
			}
			if !pane.sortMenuOpen {
				return ui.layoutFilePanePathArea(th, gtx, idx, pane, active)
			}
			m := op.Record(gtx.Ops)
			sortDims := ui.layoutFilePaneSortOptionsStrip(th, gtx, pane, sortOptions)
			sortCall := m.Stop()
			fillH := sortDims.Size.Y
			if fillH < 1 {
				fillH = gtx.Dp(unit.Dp(22))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					sortCall.Add(gtx.Ops)
					return sortDims
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutFilePanePathFill(gtx, fillH)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePaneControlStrip(th, gtx, idx, pane)
		}),
	)
}

func (ui *UI) layoutFilePanePathArea(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState, active bool) layout.Dimensions {
	if pane == nil {
		return layout.Dimensions{}
	}
	if pane.pathEditing {
		return ui.layoutFilePanePathEditor(th, gtx, idx, pane, active)
	}
	ui.handleFilePanePathRowClicks(gtx, idx, pane)
	if pane.pathEditing {
		return ui.layoutFilePanePathEditor(th, gtx, idx, pane, active)
	}

	return pane.pathRowClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		pathDims := ui.layoutFilePanePath(th, gtx, idx, pane, active)
		pathCall := m.Stop()
		fillH := pathDims.Size.Y
		if fillH < 1 {
			fillH = gtx.Dp(unit.Dp(18))
		}

		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				pathCall.Add(gtx.Ops)
				return pathDims
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFilePanePathFill(gtx, fillH)
			}),
		)
	})
}

func (ui *UI) handleFilePanePathRowClicks(gtx layout.Context, idx int, pane *filePaneState) {
	if pane == nil {
		return
	}
	for {
		_, ok := pane.pathRowClick.Update(gtx)
		if !ok {
			break
		}
		ui.setActiveFilePane(idx)
		pane.sortMenuOpen = false
		pane.closeFavoriteMenu()
		pane.closeContextMenu()
		if pane.registerPathClick("row:"+pane.dir, gtx.Now, filePanePathDoubleClickWindow) {
			pane.clearPendingPathNavigate()
			pane.beginPathEdit()
		}
	}
}

func (ui *UI) layoutFilePanePathFill(gtx layout.Context, fillH int) layout.Dimensions {
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, fillH)}
}

func (ui *UI) layoutFilePanePath(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState, active bool) layout.Dimensions {
	if pane == nil {
		return layout.Dimensions{}
	}
	segments := splitFilePathSegments(pane.dir)
	pane.ensurePathClicks(len(segments))
	if pane.pendingPathNav != "" {
		if gtx.Now.Before(pane.pendingPathAt) {
			gtx.Execute(op.InvalidateCmd{At: pane.pendingPathAt})
		} else {
			target := pane.pendingPathNav
			pane.clearPendingPathNavigate()
			if ui.loadPaneDir(idx, target) {
				return ui.layoutFilePanePath(th, gtx, idx, pane, active)
			}
		}
	}

	for i := range segments {
		click := &pane.pathSegClicks[i]
		for {
			_, ok := click.Update(gtx)
			if !ok {
				break
			}
			ui.setActiveFilePane(idx)
			pane.sortMenuOpen = false
			pane.closeFavoriteMenu()
			pane.closeContextMenu()
			pane.clearPendingPathNavigate()
			if i != len(segments)-1 {
				pane.queuePathNavigate(segments[i].path, gtx.Now.Add(filePanePathClickDelay))
				gtx.Execute(op.InvalidateCmd{At: pane.pendingPathAt})
			}
		}
		if pane.pathEditing {
			return ui.layoutFilePanePathEditor(th, gtx, idx, pane, active)
		}
	}

	children := make([]layout.FlexChild, 0, len(segments))
	baseColor := txtColor
	hoverColor := color.NRGBA{R: 230, G: 236, B: 255, A: 255}
	if active {
		baseColor = color.NRGBA{R: 220, G: 230, B: 255, A: 255}
	}
	for i := range segments {
		i := i
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			click := &pane.pathSegClicks[i]
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				bg := color.NRGBA{}
				lblColor := baseColor
				if click.Hovered() {
					bg = color.NRGBA{R: 44, G: 52, B: 74, A: 255}
					lblColor = hoverColor
				}
				if i == len(segments)-1 {
					lblColor = color.NRGBA{R: 205, G: 220, B: 255, A: 255}
					if click.Hovered() {
						lblColor = color.NRGBA{R: 240, G: 244, B: 255, A: 255}
					}
				}
				return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, segments[i].label)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.Font.Weight = font.Normal
					if i == len(segments)-1 {
						lbl.Font.Weight = font.Medium
					}
					lbl.TextSize = scaleThemeFontSize(th, 11)
					lbl.Color = lblColor
					lbl.MaxLines = 1
					return lbl.Layout(gtx)
				})
			})
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (ui *UI) layoutFilePanePathEditor(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState, active bool) layout.Dimensions {
	if pane == nil {
		return layout.Dimensions{}
	}
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &pane.pathEdit, Name: key.NameEscape})
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		pane.stopPathEdit()
		gtx.Execute(key.FocusCmd{})
		return ui.layoutFilePanePath(th, gtx, idx, pane, active)
	}
	for {
		ev, ok := pane.pathEdit.Update(gtx)
		if !ok {
			break
		}
		if submit, ok := ev.(widget.SubmitEvent); ok {
			if ui.submitPanePathEdit(idx, submit.Text) {
				break
			}
		}
	}
	if !pane.pathEditing {
		return ui.layoutFilePanePath(th, gtx, idx, pane, active)
	}
	if pane.pathEditFocus {
		pane.pathEditFocus = false
		gtx.Execute(key.FocusCmd{Tag: &pane.pathEdit})
	} else if !gtx.Focused(&pane.pathEdit) {
		pane.stopPathEdit()
		return ui.layoutFilePanePath(th, gtx, idx, pane, active)
	}

	ed := material.Editor(th, &pane.pathEdit, "")
	ed.Font.Typeface = ui.mainTypeface()
	ed.TextSize = scaleThemeFontSize(th, 12)
	ed.Color = txtColor
	if active {
		ed.Color = color.NRGBA{R: 220, G: 230, B: 255, A: 255}
	}
	ed.HintColor = hintColor

	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 22, G: 28, B: 40, A: 255},
		color.NRGBA{R: 110, G: 132, B: 190, A: 120},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, ed.Layout)
		},
	)
}

func (ui *UI) layoutFilePaneSortOptionsStrip(th *material.Theme, gtx layout.Context, pane *filePaneState, sortOptions []struct {
	key   fileSortKey
	label string
}) layout.Dimensions {
	if pane == nil || len(sortOptions) == 0 {
		return layout.Dimensions{}
	}
	gtx.Constraints.Min.X = 0
	stripH := gtx.Dp(unit.Dp(22))
	if stripH < 1 {
		stripH = 1
	}
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 18, G: 22, B: 30, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 22},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(1), Right: unit.Dp(1), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0, len(sortOptions)*2-1)
					for i, opt := range sortOptions {
						if i > 0 {
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutFilePaneControlDivider(gtx, stripH)
							}))
						}
						i := i
						activeOpt := pane.sortKey == opt.key
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return pane.sortOptionBtns[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								pointer.CursorPointer.Add(gtx.Ops)
								bg := color.NRGBA{}
								fg := txtColor
								if activeOpt {
									bg = color.NRGBA{R: 68, G: 92, B: 180, A: 255}
									fg = color.NRGBA{R: 240, G: 246, B: 255, A: 255}
								} else if pane.sortOptionBtns[i].Hovered() {
									bg = color.NRGBA{R: 28, G: 34, B: 48, A: 255}
									fg = color.NRGBA{R: 230, G: 236, B: 255, A: 255}
								}
								return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										lbl := material.Body2(th, opt.label)
										lbl.Font.Typeface = ui.mainTypeface()
										lbl.Font.Weight = font.Medium
										lbl.TextSize = scaleThemeFontSize(th, 11)
										lbl.Color = fg
										lbl.MaxLines = 1
										return lbl.Layout(gtx)
									})
								})
							})
						}))
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
				})
			})
		},
	)
}

func layoutFilePaneControlDivider(gtx layout.Context, h int) layout.Dimensions {
	w := gtx.Dp(unit.Dp(1))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 22}, clip.Rect(image.Rect(0, 0, w, h)).Op())
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}

func (ui *UI) processFileModeBadgeInput(gtx layout.Context, idx int, pane *filePaneState) {
	if pane == nil {
		return
	}
	if pane.modeClick.Clicked(gtx) {
		pane.clearPendingPathNavigate()
		pane.stopPathEdit()
		ui.togglePaneMode(idx)
	}
}

func (ui *UI) processFilePaneSortBadgeInput(gtx layout.Context, idx int, pane *filePaneState) {
	if pane == nil {
		return
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &pane.sortClick,
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
			pane.clearPendingPathNavigate()
			pane.stopPathEdit()
			next := !pane.sortMenuOpen
			ui.closeSortMenusExcept(idx)
			ui.closeFavoriteMenusExcept(idx)
			pane.closeContextMenu()
			pane.sortMenuOpen = next
			if next {
				pane.closeFavoriteMenu()
			}
		}
	}

	if pane.sortClick.Clicked(gtx) {
		pane.clearPendingPathNavigate()
		pane.stopPathEdit()
		pane.closeFavoriteMenu()
		ui.togglePaneSortDirection(idx)
	}
}

func (ui *UI) processFilePaneFavoriteBadgeInput(gtx layout.Context, idx int, pane *filePaneState) {
	if pane == nil {
		return
	}
	if pane.favoriteClick.Clicked(gtx) {
		ui.setActiveFilePane(idx)
		pane.clearPendingPathNavigate()
		pane.stopPathEdit()
		next := !pane.favoriteMenuOpen
		ui.closeFavoriteMenusExcept(idx)
		ui.closeSortMenusExcept(idx)
		pane.closeContextMenu()
		pane.sortMenuOpen = false
		pane.favoriteMenuOpen = next
		if !next {
			pane.favoriteMenuRect = image.Rectangle{}
		}
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) layoutFilePaneControlStrip(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	if pane == nil {
		return layout.Dimensions{}
	}
	ui.processFileModeBadgeInput(gtx, idx, pane)
	ui.processFilePaneSortBadgeInput(gtx, idx, pane)
	ui.processFilePaneFavoriteBadgeInput(gtx, idx, pane)

	stripH := gtx.Dp(unit.Dp(22))
	if stripH < 1 {
		stripH = 1
	}
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 18, G: 22, B: 30, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 22},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(1), Right: unit.Dp(1), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return pane.modeClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								pointer.CursorPointer.Add(gtx.Ops)
								bg := color.NRGBA{}
								iconColor := color.NRGBA{R: 210, G: 210, B: 210, A: 255}
								if pane.modeClick.Hovered() {
									bg = color.NRGBA{R: 28, G: 34, B: 48, A: 255}
									iconColor = color.NRGBA{R: 230, G: 236, B: 255, A: 255}
								}
								return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										iconGtx := gtx
										iconGtx.Constraints = layout.Exact(image.Pt(gtx.Dp(unit.Dp(16)), gtx.Dp(unit.Dp(12))))
										return layoutModeGlyph(iconGtx, pane.table.Mode, iconColor)
									})
								})
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutFilePaneControlDivider(gtx, stripH)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return pane.sortClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								pointer.CursorPointer.Add(gtx.Ops)
								bg := color.NRGBA{}
								fg := txtColor
								if pane.sortMenuOpen {
									bg = color.NRGBA{R: 68, G: 92, B: 180, A: 255}
									fg = color.NRGBA{R: 240, G: 246, B: 255, A: 255}
								} else if pane.sortClick.Hovered() {
									bg = color.NRGBA{R: 28, G: 34, B: 48, A: 255}
									fg = color.NRGBA{R: 230, G: 236, B: 255, A: 255}
								}
								return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										lbl := material.Body2(th, pane.sortBadgeText())
										lbl.Font.Typeface = ui.mainTypeface()
										lbl.Font.Weight = font.Medium
										lbl.TextSize = scaleThemeFontSize(th, 11)
										lbl.Color = fg
										lbl.MaxLines = 1
										return lbl.Layout(gtx)
									})
								})
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutFilePaneControlDivider(gtx, stripH)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return pane.favoriteClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								pointer.CursorPointer.Add(gtx.Ops)
								bg := color.NRGBA{}
								fg := txtColor
								if pane.favoriteMenuOpen {
									bg = color.NRGBA{R: 68, G: 92, B: 180, A: 255}
									fg = color.NRGBA{R: 240, G: 246, B: 255, A: 255}
								} else if pane.favoriteClick.Hovered() {
									bg = color.NRGBA{R: 28, G: 34, B: 48, A: 255}
									fg = color.NRGBA{R: 230, G: 236, B: 255, A: 255}
								}
								return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										lbl := material.Body2(th, "*")
										lbl.Font.Typeface = ui.mainTypeface()
										lbl.Font.Weight = font.Medium
										lbl.TextSize = scaleThemeFontSize(th, 10)
										lbl.Color = fg
										lbl.MaxLines = 1
										return lbl.Layout(gtx)
									})
								})
							})
						}),
					)
				})
			})
		},
	)
}

func (ui *UI) layoutFileModeBadge(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	ui.processFileModeBadgeInput(gtx, idx, pane)
	if pane == nil {
		return layout.Dimensions{}
	}
	dims := pane.modeClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{R: 18, G: 22, B: 30, A: 255}
		border := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
		iconColor := color.NRGBA{R: 210, G: 210, B: 210, A: 255}
		if pane.modeClick.Hovered() {
			bg = color.NRGBA{R: 28, G: 34, B: 48, A: 255}
			border = color.NRGBA{R: 120, G: 150, B: 255, A: 70}
			iconColor = color.NRGBA{R: 230, G: 236, B: 255, A: 255}
		}

		width := unit.Dp(30)
		height := unit.Dp(22)
		return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), bg, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				iconGtx := gtx
				iconGtx.Constraints.Min = image.Pt(gtx.Dp(width)-gtx.Dp(unit.Dp(12)), gtx.Dp(height)-gtx.Dp(unit.Dp(8)))
				iconGtx.Constraints.Max = iconGtx.Constraints.Min
				return layoutModeGlyph(iconGtx, pane.table.Mode, iconColor)
			})
		})
	})
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}

	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func layoutModeGlyph(gtx layout.Context, mode table.Mode, barColor color.NRGBA) layout.Dimensions {
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
	ui.processFilePaneSortBadgeInput(gtx, idx, pane)

	dims := layoutModeButton(th, gtx, ui.mainTypeface(), &pane.sortClick, pane.sortBadgeText(), pane.sortMenuOpen)
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}

	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func (ui *UI) layoutFilePaneFavoriteBadge(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	if pane == nil {
		return layout.Dimensions{}
	}
	ui.processFilePaneFavoriteBadgeInput(gtx, idx, pane)

	dims := layoutTinyModeButton(th, gtx, ui.mainTypeface(), &pane.favoriteClick, "*", pane.favoriteMenuOpen)
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}

	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func (ui *UI) layoutFilePaneFavoriteMenu(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	if pane == nil || !pane.favoriteMenuOpen {
		return layout.Dimensions{}
	}

	items := ui.paneFavoriteItems(pane)
	pane.ensureFavoriteOptionClicks(len(items))
	pane.ensureFavoriteRemoveClicks(len(items))
	skipActivate := make(map[int]struct{}, len(items))
	for i, item := range items {
		if !item.removable {
			continue
		}
		if pane.favoriteRemoveClicks[i].Clicked(gtx) {
			_, err := ui.removeFavoriteLocation(item.targetDir)
			if err != nil {
				pane.setNotice("failed to save favorites: "+err.Error(), gtx.Now)
			}
			skipActivate[i] = struct{}{}
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	for i, item := range items {
		if item.disabled {
			continue
		}
		if _, skip := skipActivate[i]; skip {
			continue
		}
		if pane.favoriteOptionClicks[i].Clicked(gtx) {
			if item.addCurrent {
				ui.addPaneCurrentDirFavorite(idx, gtx.Now)
			} else {
				ui.navigatePaneFavorite(idx, item.targetDir)
			}
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	ui.updateFilePaneFavoriteHover(gtx, pane, items)

	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &pane.favoritePointerTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok || pe.Kind != pointer.Press {
			continue
		}
		if !pe.Buttons.Contain(pointer.ButtonPrimary) {
			continue
		}
		pos := pe.Position.Round()
		inMenu := pane.favoriteMenuRect.Dx() > 0 && pane.favoriteMenuRect.Dy() > 0 &&
			pos.X >= pane.favoriteMenuRect.Min.X && pos.X < pane.favoriteMenuRect.Max.X &&
			pos.Y >= pane.favoriteMenuRect.Min.Y && pos.Y < pane.favoriteMenuRect.Max.Y
		overFavoriteToggle := pane.favoriteClick.Pressed() || pane.favoriteClick.Hovered()
		if !inMenu && !overFavoriteToggle {
			pane.closeFavoriteMenu()
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	if !pane.favoriteMenuOpen {
		return layout.Dimensions{}
	}

	m := op.Record(gtx.Ops)
	menuDims := ui.layoutFilePaneFavoriteMenuCard(th, gtx, pane, items)
	call := m.Stop()

	anchor := image.Point{
		X: gtx.Constraints.Max.X - menuDims.Size.X,
		Y: pane.headerHeight + gtx.Dp(unit.Dp(4)),
	}
	anchor = clampFilePaneMenuPoint(anchor, menuDims.Size, gtx.Constraints.Max)
	pane.favoriteMenuRect = image.Rectangle{Min: anchor, Max: anchor.Add(menuDims.Size)}

	bodyClip := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	offset := op.Offset(anchor).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	ui.layoutFilePaneFavoriteTooltip(th, gtx, pane, anchor, menuDims.Size)
	bodyClip.Pop()

	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &pane.favoritePointerTag)
	pass.Pop()

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *UI) updateFilePaneFavoriteHover(gtx layout.Context, pane *filePaneState, items []fileFavoriteItem) {
	if pane == nil {
		return
	}
	hoveredKey := ""
	hoveredLabel := ""
	for i, item := range items {
		if item.disabled || item.addCurrent {
			continue
		}
		hovered := i < len(pane.favoriteOptionClicks) && pane.favoriteOptionClicks[i].Hovered()
		if !hovered && item.removable && i < len(pane.favoriteRemoveClicks) {
			hovered = pane.favoriteRemoveClicks[i].Hovered()
		}
		if !hovered {
			continue
		}
		hoveredKey = item.targetDir
		hoveredLabel = item.targetDir
		break
	}

	if hoveredKey == "" {
		pane.favoriteHoverKey = ""
		pane.favoriteHoverLabel = ""
		pane.favoriteHoverAt = time.Time{}
		return
	}
	if pane.favoriteHoverKey != hoveredKey {
		pane.favoriteHoverKey = hoveredKey
		pane.favoriteHoverLabel = hoveredLabel
		pane.favoriteHoverAt = gtx.Now
	}
	if pane.favoriteHoverAt.IsZero() {
		pane.favoriteHoverAt = gtx.Now
	}
	showAt := pane.favoriteHoverAt.Add(filePaneFavoriteTooltipDelay)
	if gtx.Now.Before(showAt) {
		gtx.Execute(op.InvalidateCmd{At: showAt})
	}
}

func (ui *UI) layoutFilePaneFavoriteTooltip(th *material.Theme, gtx layout.Context, pane *filePaneState, menuAnchor image.Point, menuSize image.Point) {
	if pane == nil || pane.favoriteHoverLabel == "" || pane.favoriteHoverAt.IsZero() {
		return
	}
	if gtx.Now.Before(pane.favoriteHoverAt.Add(filePaneFavoriteTooltipDelay)) {
		return
	}

	maxW := gtx.Dp(unit.Dp(360))
	limitW := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(8))
	if maxW > limitW {
		maxW = limitW
	}
	if maxW < gtx.Dp(unit.Dp(120)) {
		maxW = gtx.Dp(unit.Dp(120))
	}
	if maxW < 1 {
		return
	}

	m := op.Record(gtx.Ops)
	tipDims := fixedWidth(gtx, maxW, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			color.NRGBA{R: 12, G: 16, B: 24, A: 248},
			color.NRGBA{R: 120, G: 150, B: 255, A: 90},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, pane.favoriteHoverLabel)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleThemeFontSize(th, 10)
					lbl.Color = color.NRGBA{R: 230, G: 236, B: 255, A: 255}
					lbl.WrapPolicy = text.WrapGraphemes
					return lbl.Layout(gtx)
				})
			},
		)
	})
	tipCall := m.Stop()

	gap := gtx.Dp(unit.Dp(6))
	pos := image.Point{X: menuAnchor.X + menuSize.X + gap, Y: menuAnchor.Y}
	if pos.X+tipDims.Size.X > gtx.Constraints.Max.X {
		pos.X = menuAnchor.X - tipDims.Size.X - gap
	}
	if pos.X < 0 {
		pos.X = 0
	}
	if pos.Y+tipDims.Size.Y > gtx.Constraints.Max.Y {
		pos.Y = gtx.Constraints.Max.Y - tipDims.Size.Y
	}
	if pos.Y < 0 {
		pos.Y = 0
	}

	offset := op.Offset(pos).Push(gtx.Ops)
	tipCall.Add(gtx.Ops)
	offset.Pop()
}

func (ui *UI) layoutFilePaneFavoriteMenuCard(th *material.Theme, gtx layout.Context, pane *filePaneState, items []fileFavoriteItem) layout.Dimensions {
	const menuWidthDp = 134
	width := gtx.Dp(unit.Dp(menuWidthDp))
	if width > gtx.Constraints.Max.X {
		width = gtx.Constraints.Max.X
	}
	if width < 1 {
		width = 1
	}

	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		if pane == nil {
			return layout.Dimensions{}
		}
		return pane.favoriteMenuClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return fillRoundedBox(
				gtx,
				gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
				color.NRGBA{R: 20, G: 24, B: 34, A: 250},
				color.NRGBA{R: 255, G: 255, B: 255, A: 22},
				func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0, len(items)+3)
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(3), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Caption(th, "Favorites")
							lbl.Font.Typeface = ui.mainTypeface()
							lbl.TextSize = scaleThemeFontSize(th, 9)
							lbl.Color = color.NRGBA{R: 170, G: 180, B: 205, A: 255}
							lbl.MaxLines = 1
							lbl.Font.Weight = font.Medium
							return lbl.Layout(gtx)
						})
					}))
					children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout))
					for i, item := range items {
						i := i
						item := item
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutFilePaneFavoriteMenuItem(th, gtx, &pane.favoriteOptionClicks[i], &pane.favoriteRemoveClicks[i], item)
						}))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				},
			)
		})
	})
}

func (ui *UI) layoutFilePaneFavoriteMenuItem(th *material.Theme, gtx layout.Context, click *widget.Clickable, removeClick *widget.Clickable, item fileFavoriteItem) layout.Dimensions {
	label := item.label

	renderLabel := func(gtx layout.Context, fg color.NRGBA, weight font.Weight) layout.Dimensions {
		out := label
		if !item.addCurrent {
			out = trimLeftToFit(gtx, out, scaleThemeFontSize(th, 10))
		}
		lbl := material.Body2(th, out)
		lbl.Font.Typeface = ui.mainTypeface()
		lbl.TextSize = scaleThemeFontSize(th, 10)
		lbl.Font.Weight = weight
		lbl.Color = fg
		lbl.MaxLines = 1
		// Suppress right-side ellipsis; we want left-side-only trimming for paths.
		lbl.Truncator = "\u200b"
		return lbl.Layout(gtx)
	}

	bg := color.NRGBA{}
	fg := txtColor
	weight := font.Normal
	if item.disabled {
		fg = color.NRGBA{R: 130, G: 136, B: 150, A: 255}
	}
	if item.addCurrent {
		fg = color.NRGBA{R: 185, G: 218, B: 255, A: 255}
		weight = font.Medium
	}
	hovered := (click != nil && click.Hovered()) || (item.removable && removeClick != nil && removeClick.Hovered())
	if item.active {
		bg = color.NRGBA{R: 68, G: 92, B: 180, A: 54}
		weight = font.Medium
	}
	if hovered && !item.disabled {
		bg = color.NRGBA{R: 68, G: 92, B: 180, A: 54}
		fg = color.NRGBA{R: 230, G: 236, B: 255, A: 255}
	}

	return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(5), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 3)
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if item.disabled || click == nil {
					return renderLabel(gtx, fg, weight)
				}
				return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return renderLabel(gtx, fg, weight)
				})
			}))
			if item.removable && !item.disabled && removeClick != nil {
				children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout))
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutFilePaneFavoriteRemoveButton(th, gtx, removeClick)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

func trimLeftRunes(text string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	if max <= 2 {
		return string(runes[len(runes)-max:])
	}
	return ".." + string(runes[len(runes)-(max-2):])
}

func trimLeftToFit(gtx layout.Context, text string, size unit.Sp) string {
	if text == "" {
		return text
	}
	maxPx := gtx.Constraints.Max.X
	if maxPx <= 0 {
		return text
	}
	glyphPx := gtx.Sp(size)
	if glyphPx < 1 {
		glyphPx = 1
	}
	avgCharPx := (glyphPx*56 + 99) / 100
	if avgCharPx < 1 {
		avgCharPx = 1
	}
	capacity := maxPx / avgCharPx
	if capacity < 1 {
		capacity = 1
	}
	return trimLeftRunes(text, capacity)
}

func layoutFilePaneFavoriteRemoveButton(th *material.Theme, gtx layout.Context, c *widget.Clickable) layout.Dimensions {
	return layoutTinyIconModeButton(th, gtx, c, uiCloseIcon(), false)
}

func paneRRect(size image.Point, radius int, roundLeft, roundRight bool) clip.RRect {
	if size.X < 1 {
		size.X = 1
	}
	if size.Y < 1 {
		size.Y = 1
	}
	rr := clip.RRect{Rect: image.Rect(0, 0, size.X, size.Y)}
	if roundLeft {
		rr.NW = radius
		rr.SW = radius
	}
	if roundRight {
		rr.NE = radius
		rr.SE = radius
	}
	return rr
}

func maskPaneBorderEdges(gtx layout.Context, size image.Point, bg color.NRGBA, drawLeft, drawRight bool, maskW int) {
	if size.X < 1 || size.Y < 1 {
		return
	}
	if maskW < 1 {
		maskW = 1
	}
	if maskW > size.X {
		maskW = size.X
	}
	y0 := 0
	y1 := size.Y
	// Preserve the first/last pixel rows so seam joins look continuous at corners.
	if size.Y > 2 {
		y0 = 1
		y1 = size.Y - 1
	}
	if y1 <= y0 {
		return
	}
	if !drawLeft {
		paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rect(0, y0, maskW, y1)).Op())
	}
	if !drawRight {
		paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rect(size.X-maskW, y0, size.X, y1)).Op())
	}
}

func fillFilePaneBox(gtx layout.Context, radius int, roundLeft, roundRight, drawLeftBorder, drawRightBorder bool, bg, border color.NRGBA, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		call.Add(gtx.Ops)
		return dims
	}

	rr := paneRRect(dims.Size, radius, roundLeft, roundRight)
	defer clip.Rect(image.Rect(0, 0, dims.Size.X, dims.Size.Y)).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())
	maskPaneBorderEdges(gtx, dims.Size, bg, drawLeftBorder, drawRightBorder, 1)

	call.Add(gtx.Ops)
	return dims
}

func layoutFilePaneChrome(gtx layout.Context, active bool, radius int, roundLeft, roundRight, drawLeftBorder, drawRightBorder bool, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	call.Add(gtx.Ops)
	if !active || dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}

	rr := paneRRect(dims.Size, radius, roundLeft, roundRight)
	hl := color.NRGBA{R: 185, G: 205, B: 255, A: 120}
	defer clip.Rect(image.Rect(0, 0, dims.Size.X, dims.Size.Y)).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, hl, clip.Stroke{
		Path:  rr.Path(gtx.Ops),
		Width: 2,
	}.Op())
	maskPaneBorderEdges(gtx, dims.Size, color.NRGBA{R: 18, G: 22, B: 30, A: 255}, drawLeftBorder, drawRightBorder, 2)
	return dims
}

func layoutTinyModeButton(th *material.Theme, gtx layout.Context, typeface font.Typeface, c *widget.Clickable, label string, active bool) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{R: 18, G: 22, B: 30, A: 255}
		border := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
		labelColor := txtColor
		if active {
			bg = color.NRGBA{R: 68, G: 92, B: 180, A: 255}
			border = color.NRGBA{R: 120, G: 150, B: 255, A: 90}
			labelColor = color.NRGBA{R: 240, G: 246, B: 255, A: 255}
		} else if c.Hovered() {
			bg = color.NRGBA{R: 28, G: 34, B: 48, A: 255}
			border = color.NRGBA{R: 120, G: 150, B: 255, A: 70}
			labelColor = color.NRGBA{R: 230, G: 236, B: 255, A: 255}
		}

		return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), bg, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = typeface
				lbl.Font.Weight = font.Medium
				lbl.TextSize = scaleThemeFontSize(th, 10)
				lbl.Color = labelColor
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			})
		})
	})
}

func layoutTinyIconModeButton(th *material.Theme, gtx layout.Context, c *widget.Clickable, icon *widget.Icon, active bool) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{R: 18, G: 22, B: 30, A: 255}
		border := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
		iconColor := txtColor
		if active {
			bg = color.NRGBA{R: 68, G: 92, B: 180, A: 255}
			border = color.NRGBA{R: 120, G: 150, B: 255, A: 90}
			iconColor = color.NRGBA{R: 240, G: 246, B: 255, A: 255}
		} else if c.Hovered() {
			bg = color.NRGBA{R: 40, G: 54, B: 82, A: 255}
			border = color.NRGBA{R: 120, G: 150, B: 255, A: 130}
			iconColor = color.NRGBA{R: 235, G: 242, B: 255, A: 255}
		}

		return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), bg, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := gtx.Dp(unit.Dp(12))
				if size < 1 {
					size = 1
				}
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					if icon != nil {
						iconGtx := gtx
						iconGtx.Constraints = layout.Exact(image.Pt(size, size))
						icon.Layout(iconGtx, iconColor)
					}
					return layout.Dimensions{Size: image.Pt(size, size)}
				})
			})
		})
	})
}

func layoutModeButton(th *material.Theme, gtx layout.Context, typeface font.Typeface, c *widget.Clickable, label string, active bool) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{R: 18, G: 22, B: 30, A: 255}
		border := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
		labelColor := txtColor
		if active {
			bg = color.NRGBA{R: 68, G: 92, B: 180, A: 255}
			border = color.NRGBA{R: 120, G: 150, B: 255, A: 90}
			labelColor = color.NRGBA{R: 240, G: 246, B: 255, A: 255}
		} else if c.Hovered() {
			bg = color.NRGBA{R: 28, G: 34, B: 48, A: 255}
			border = color.NRGBA{R: 120, G: 150, B: 255, A: 70}
			labelColor = color.NRGBA{R: 230, G: 236, B: 255, A: 255}
		}

		return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), bg, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = typeface
				lbl.Font.Weight = font.Medium
				lbl.TextSize = scaleThemeFontSize(th, 11)
				lbl.Color = labelColor
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			})
		})
	})
}
