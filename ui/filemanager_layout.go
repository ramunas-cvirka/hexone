// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/ui/platform"
	uitheme "hexone/ui/theme"
	"hexone/ui/widget/table"
	"image"
	"image/color"
	"strconv"
	"strings"
	"time"

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

const (
	filePaneWheelRange                 = 1 << 30
	filePanePathDoubleClickWindow      = 450 * time.Millisecond
	filePaneFavoriteRevealDelay        = 350 * time.Millisecond
	filePaneFavoriteRevealFadeDur      = 220 * time.Millisecond
	filePaneFavoriteRevealHotspotPadDp = 8
	filePaneFavoriteMenuWidthDp        = 148
	filePaneCornerDp                   = 8
	filePaneControlCornerDp            = 6
	filePaneOverlayCornerDp            = 6
	filePaneNoticeVisibleDur           = 3 * time.Second
	filePaneNoticeFadeInDur            = 180 * time.Millisecond
	filePaneNoticeFadeOutDur           = 220 * time.Millisecond
	filePaneNoticeSlideDp              = unit.Dp(6)
	filePaneTableDoubleClickWindow     = 400 * time.Millisecond
	filePaneLoadingHintDelay           = 350 * time.Millisecond
)

type visiblePane struct {
	idx  int
	pane *filePaneState
}

func (ui *UI) layoutTab1(th *material.Theme, gtx layout.Context) layout.Dimensions {
	ui.pumpFilePaneLoads(gtx)
	ui.pumpFileViewerState(gtx)
	ui.pumpFileCopyState(gtx)
	ui.pumpArchiveExtractState(gtx)
	ui.pumpFileDeleteState(gtx)
	ui.pumpFileMoveState(gtx)
	ui.pumpFileCreateState(gtx)
	ui.pumpFilePermState(gtx)

	dims := layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePanes(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileCopyDialog(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutArchiveExtractConflictDialog(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileDeleteDialog(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileMoveDialog(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileCreateDialog(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePermDialog(th, gtx)
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
	if ui.fileCopy != nil || ui.fileDelete != nil || ui.fileMove != nil || ui.fileCreate != nil || ui.filePerm != nil || ui.archiveExtractConflictOpen() {
		return
	}
	ui.handleFileManagerEscape(gtx)
	if ui.pathEditActive() {
		return
	}
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameF1, Required: key.ModAlt},
			key.Filter{Name: key.NameF2, Required: key.ModAlt},
		)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case key.NameF1:
			if ui.openPaneDriveMenu(0) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameF2:
			if ui.openPaneDriveMenu(1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}

	anyMods := ^key.Modifiers(0)
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: "A", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "a", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "A", Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "a", Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "E", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "e", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "E", Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "e", Required: key.ModShortcut, Optional: anyMods},
		)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case "A", "a":
			if ke.Modifiers != key.ModCtrl && ke.Modifiers != key.ModShortcut {
				continue
			}
			if ui.handleFileManagerSelectAll(gtx.Now) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case "E", "e":
			if ke.Modifiers != key.ModCtrl && ke.Modifiers != key.ModShortcut {
				continue
			}
			if ui.handleFileManagerSelectMatching(gtx.Now) {
				gtx.Execute(op.InvalidateCmd{})
			}
		}
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
			case fileActionRenameMove:
				ui.startFileMoveDialog(ui.activeFilePane, gtx.Now)
				ui.rep.active = false
				continue
			case fileActionCreate:
				ui.startFileCreateDialog(ui.activeFilePane, gtx.Now)
				ui.rep.active = false
				continue
			case fileActionDelete:
				ui.startFileDeleteDialog(ui.activeFilePane, gtx.Now)
				ui.rep.active = false
				continue
			case fileActionMarkSelectNext:
				if ui.handleFileManagerInsert() {
					gtx.Execute(op.InvalidateCmd{})
				}
				continue
			}

			if active := ui.activePane(); active != nil && active.loading {
				ui.held[holdKey] = false
				continue
			}

			pane := ui.activePane()
			if pane == nil || pane.table == nil || pane.model == nil {
				ui.held[holdKey] = false
				continue
			}
			if pane.loading {
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

func (ui *UI) HandlePlatformInsertKey(_ time.Time) bool {
	return ui.handleFileManagerInsert()
}

func (ui *UI) handleFileManagerInsert() bool {
	if ui == nil || ui.settingsModal != nil || ui.sshModal != nil {
		return false
	}
	if ui.fileViewer != nil {
		return false
	}
	if ui.fileCopy != nil || ui.fileDelete != nil || ui.fileMove != nil || ui.fileCreate != nil || ui.filePerm != nil || ui.archiveExtractConflictOpen() {
		return false
	}
	if ui.pathEditActive() {
		return false
	}
	pane := ui.activePane()
	if pane == nil {
		return false
	}
	ui.rep.active = false
	return pane.markCurrentAndAdvance()
}

func (ui *UI) handleFileManagerSelectAll(_ time.Time) bool {
	if ui == nil || ui.settingsModal != nil || ui.sshModal != nil {
		return false
	}
	if ui.fileViewer != nil {
		return false
	}
	if ui.fileCopy != nil || ui.fileDelete != nil || ui.fileMove != nil || ui.fileCreate != nil || ui.filePerm != nil || ui.archiveExtractConflictOpen() {
		return false
	}
	if ui.pathEditActive() {
		return false
	}
	pane := ui.activePane()
	if pane == nil {
		return false
	}
	ui.rep.active = false
	return pane.toggleMarkAllSelectable()
}

func (ui *UI) handleFileManagerSelectMatching(_ time.Time) bool {
	if ui == nil || ui.settingsModal != nil || ui.sshModal != nil {
		return false
	}
	if ui.fileViewer != nil {
		return false
	}
	if ui.fileCopy != nil || ui.fileDelete != nil || ui.fileMove != nil || ui.fileCreate != nil || ui.filePerm != nil || ui.archiveExtractConflictOpen() {
		return false
	}
	if ui.pathEditActive() {
		return false
	}
	pane := ui.activePane()
	if pane == nil {
		return false
	}
	ui.rep.active = false
	return pane.toggleMarkRowsMatchingCurrentSelection()
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
			if pane.driveMenuOpen {
				pane.closeDriveMenu()
				closed = true
			}
			if pane.sortMenuOpen {
				pane.closeSortMenu()
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
	for _, item := range visible {
		idx := item.idx
		cur := item.pane
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePane(th, gtx, idx, cur)
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
	palette := filePanePaletteFromConfig(ui.fmCfg)
	widths := paneColumnWidths(gtx.Constraints.Max.X, len(visible))
	x := 0
	height := gtx.Constraints.Max.Y
	base := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
	highlight := filePaneActiveBorderColor(palette.PaneBg)
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

func (ui *UI) layoutFilePane(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	active := idx == ui.activeFilePane
	palette := filePanePaletteFromConfig(ui.fmCfg)
	accent := filePaneActiveBorderColor(palette.PaneBg)
	shade := filePaneInactiveShadeColor(ui.fmCfg, palette.PaneBg)

	return layoutFilePaneChrome(gtx, active, accent, shade, func(gtx layout.Context) layout.Dimensions {
		return fillFilePaneBox(gtx, palette.PaneBg, func(gtx layout.Context) layout.Dimensions {
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
						return ui.layoutFilePaneDriveMenu(th, gtx, idx, pane)
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return ui.layoutFilePaneFavoriteMenu(th, gtx, idx, pane)
					}),
				)
			})
		})
	})
}

func (ui *UI) layoutFilePaneBody(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePaneTable(th, gtx, idx, pane)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePaneInlineNameEditor(th, gtx, idx, pane)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePaneNotice(th, gtx, pane)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePaneContextMenu(th, gtx, idx, pane)
		}),
	)
}

func (ui *UI) layoutFilePaneNotice(th *material.Theme, gtx layout.Context, pane *filePaneState) layout.Dimensions {
	if pane == nil || pane.noticeText == "" {
		return layout.Dimensions{}
	}
	showAt := pane.noticeShownAt
	if showAt.IsZero() {
		showAt = pane.noticeUntil.Add(-filePaneNoticeVisibleDur)
	}
	hideAt := pane.noticeUntil.Add(filePaneNoticeFadeOutDur)
	if pane.noticeUntil.IsZero() || !gtx.Now.Before(hideAt) {
		pane.noticeText = ""
		pane.noticeShownAt = time.Time{}
		pane.noticeUntil = time.Time{}
		return layout.Dimensions{}
	}

	alpha := float32(1)
	animating := false

	if gtx.Now.Before(showAt.Add(filePaneNoticeFadeInDur)) {
		t := clamp01(float32(gtx.Now.Sub(showAt)) / float32(filePaneNoticeFadeInDur))
		alpha = smoothstep01(t)
		animating = true
	}
	if !gtx.Now.Before(pane.noticeUntil) {
		t := clamp01(float32(gtx.Now.Sub(pane.noticeUntil)) / float32(filePaneNoticeFadeOutDur))
		alpha = 1 - smoothstep01(t)
		animating = true
	}
	alpha = clamp01(alpha)
	if alpha <= 0 {
		return layout.Dimensions{}
	}

	gtx.Execute(op.InvalidateCmd{At: hideAt})
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}

	offsetY := gtx.Dp(filePaneNoticeSlideDp) - int(float32(gtx.Dp(filePaneNoticeSlideDp))*alpha)
	defer op.Offset(image.Pt(0, offsetY)).Push(gtx.Ops).Pop()
	return layout.Inset{Top: unit.Dp(4), Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.NW.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return fillRoundedBox(
				gtx,
				gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
				scaleNoticeAlpha(color.NRGBA{R: 22, G: 30, B: 38, A: 234}, alpha),
				scaleNoticeAlpha(color.NRGBA{R: 136, G: 168, B: 196, A: 132}, alpha),
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, pane.noticeText)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleThemeFontSize(th, 12)
						lbl.Color = scaleNoticeAlpha(color.NRGBA{R: 220, G: 228, B: 236, A: 255}, alpha)
						lbl.MaxLines = 2
						lbl.Truncator = "…"
						return lbl.Layout(gtx)
					})
				},
			)
		})
	})
}

func scaleNoticeAlpha(c color.NRGBA, a float32) color.NRGBA {
	c.A = uint8(float32(c.A) * clamp01(a))
	return c
}

func (ui *UI) layoutFilePaneContextMenu(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	if pane == nil || !pane.ctxMenuOpen {
		return layout.Dimensions{}
	}

	spec := ui.filePaneContextMenuSpec(idx, pane)
	if len(spec.Items) == 0 {
		pane.closeContextMenu()
		return layout.Dimensions{}
	}
	pane.ctxMenuPath = normalizeFileContextMenuPath(spec, pane.ctxMenuPath)

	visiblePanels := fileContextMenuVisiblePanels(spec, pane.ctxMenuPath)
	actionTriggered := false
	for level, panelSpec := range visiblePanels {
		for _, item := range panelSpec.Items {
			if item.Separator {
				continue
			}
			click := pane.contextMenuClick(item.ID)
			if click == nil || !click.Clicked(gtx) {
				continue
			}
			if item.Disabled {
				continue
			}
			if item.hasSubmenu() {
				nextPath := replaceFileContextMenuPathLevel(pane.ctxMenuPath, level, item.ID)
				if !equalStringSlices(nextPath, pane.ctxMenuPath) {
					pane.ctxMenuPath = nextPath
					gtx.Execute(op.InvalidateCmd{})
				}
				continue
			}
			row := pane.ctxMenuRow
			pane.closeContextMenu()
			result := ui.handleFilePaneContextMenuAction(idx, pane, row, item.Action, gtx.Now)
			if result.ClipboardText != "" {
				ui.writeClipboardText(gtx, result.ClipboardText)
			}
			gtx.Execute(op.InvalidateCmd{})
			actionTriggered = true
			break
		}
		if actionTriggered {
			break
		}
	}
	if actionTriggered || !pane.ctxMenuOpen {
		return layout.Dimensions{}
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
		pos := pe.Position.Round()
		inMenu := false
		for _, rect := range pane.ctxMenuRects {
			if rect.Dx() <= 0 || rect.Dy() <= 0 {
				continue
			}
			if pos.X >= rect.Min.X && pos.X < rect.Max.X && pos.Y >= rect.Min.Y && pos.Y < rect.Max.Y {
				inMenu = true
				break
			}
		}
		if inMenu {
			continue
		}
		if pe.Buttons.Contain(pointer.ButtonSecondary) {
			pane.clearPendingInlineNameEdit()
			if ui.openFilePaneContextMenuAtPointer(idx, pane, pos, gtx.Now) {
				gtx.Execute(op.InvalidateCmd{})
			}
			continue
		}
		if pe.Buttons.Contain(pointer.ButtonPrimary) {
			pane.closeContextMenu()
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	if !pane.ctxMenuOpen {
		return layout.Dimensions{}
	}

	spec = ui.filePaneContextMenuSpec(idx, pane)
	if len(spec.Items) == 0 {
		pane.closeContextMenu()
		return layout.Dimensions{}
	}
	pane.ctxMenuPath = normalizeFileContextMenuPath(spec, pane.ctxMenuPath)
	visiblePanels = fileContextMenuVisiblePanels(spec, pane.ctxMenuPath)

	if pane.ctxMenuItemRects == nil {
		pane.ctxMenuItemRects = make(map[string]image.Rectangle)
	} else {
		clear(pane.ctxMenuItemRects)
	}
	if pane.ctxMenuRects != nil {
		pane.ctxMenuRects = pane.ctxMenuRects[:0]
	}
	blockClip := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, &pane.ctxPointerTag)
	blockClip.Pop()
	hoverID := fileContextMenuHoveredItemID(pane, visiblePanels)
	if hoverID != pane.ctxMenuHoverID {
		pane.ctxMenuHoverID = hoverID
		pane.ctxMenuHoverAnim.setHover(hoverID, gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}

	alpha, slideY, animating := popupOpenProgress(gtx.Now, pane.ctxMenuOpenedAt)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	nextPath := append([]string(nil), pane.ctxMenuPath...)
	for level, panelSpec := range visiblePanels {
		panelSize := ui.fileContextMenuPanelSize(gtx, panelSpec)
		anchor := pane.ctxMenuPos
		if level == 0 {
			anchor.Y += slideY
		}
		if level > 0 {
			if parentRect, ok := fileContextMenuParentRect(pane, level); ok {
				anchor = fileContextMenuSubmenuPoint(parentRect, panelSize, gtx.Constraints.Max)
				anchor.Y += slideY
			}
		}
		anchor = clampFilePaneMenuPoint(anchor, panelSize, gtx.Constraints.Max)
		state := ui.layoutFilePaneContextMenuPanel(th, gtx, pane, panelSpec, anchor, alpha, level)
		if state.hoveredSubmenuID != "" {
			nextPath = replaceFileContextMenuPathLevel(nextPath, level, state.hoveredSubmenuID)
		} else if state.hoveredAny && len(nextPath) > level {
			nextPath = nextPath[:level]
		}
	}
	nextPath = normalizeFileContextMenuPath(spec, nextPath)
	if !equalStringSlices(nextPath, pane.ctxMenuPath) {
		pane.ctxMenuPath = nextPath
		gtx.Execute(op.InvalidateCmd{})
	}

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func fileContextMenuParentRect(pane *filePaneState, level int) (image.Rectangle, bool) {
	if pane == nil || level <= 0 || level-1 >= len(pane.ctxMenuPath) {
		return image.Rectangle{}, false
	}
	parentID := pane.ctxMenuPath[level-1]
	if strings.TrimSpace(parentID) == "" {
		return image.Rectangle{}, false
	}
	rect, ok := pane.ctxMenuItemRects[parentID]
	if !ok || rect.Dx() <= 0 || rect.Dy() <= 0 {
		return image.Rectangle{}, false
	}
	return rect, true
}

type fileContextMenuPanelState struct {
	hoveredAny       bool
	hoveredSubmenuID string
}

func (ui *UI) fileContextMenuPanelSize(gtx layout.Context, spec fileContextMenuSpec) image.Point {
	widthDp := spec.WidthDp
	if widthDp <= 0 {
		widthDp = filePaneContextMenuRootWidthDp
	}
	width := gtx.Dp(unit.Dp(widthDp))
	if width > gtx.Constraints.Max.X {
		width = gtx.Constraints.Max.X
	}
	if width < 1 {
		width = 1
	}
	height := 0
	if strings.TrimSpace(spec.Title) != "" {
		height += ui.fileContextMenuTitleHeight(gtx)
	}
	for _, item := range spec.Items {
		if item.Separator {
			height += ui.fileContextMenuSeparatorHeight(gtx)
			continue
		}
		height += ui.fileContextMenuRowHeight(gtx, item)
	}
	if height < 1 {
		height = 1
	}
	return image.Pt(width, height)
}

func (ui *UI) fileContextMenuTitleHeight(gtx layout.Context) int {
	h := gtx.Dp(unit.Dp(20))
	if h < 1 {
		h = 1
	}
	return h
}

func (ui *UI) fileContextMenuRowHeight(gtx layout.Context, item fileContextMenuItem) int {
	h := gtx.Sp(ui.functionBarTextSize()) + gtx.Dp(unit.Dp(11))
	if item.Detail != "" {
		h = gtx.Sp(ui.functionBarTextSize()) + gtx.Sp(scaleConfigFontSize(ui.fmCfg, unit.Sp(filePaneContextMenuItemDetailTextSp))) + gtx.Dp(unit.Dp(13))
		if minH := gtx.Dp(unit.Dp(34)); h < minH {
			h = minH
		}
	} else if minH := gtx.Dp(unit.Dp(22)); h < minH {
		h = minH
	}
	if h < 1 {
		h = 1
	}
	return h
}

func (ui *UI) fileContextMenuSeparatorHeight(gtx layout.Context) int {
	h := gtx.Dp(unit.Dp(7))
	if h < 3 {
		h = 3
	}
	return h
}

func fileContextMenuSubmenuPoint(parentRect image.Rectangle, panelSize image.Point, bounds image.Point) image.Point {
	anchor := image.Point{X: parentRect.Max.X - 1, Y: parentRect.Min.Y}
	if anchor.X+panelSize.X > bounds.X {
		anchor.X = parentRect.Min.X - panelSize.X + 1
	}
	return clampFilePaneMenuPoint(anchor, panelSize, bounds)
}

func fileContextMenuHoveredItemID(pane *filePaneState, panels []fileContextMenuSpec) string {
	if pane == nil {
		return ""
	}
	hoverID := ""
	for _, panel := range panels {
		for _, item := range panel.Items {
			if item.Separator || item.Disabled {
				continue
			}
			click := pane.contextMenuClick(item.ID)
			if click != nil && click.Hovered() {
				hoverID = item.ID
			}
		}
	}
	return hoverID
}

func (ui *UI) layoutFilePaneContextMenuPanel(th *material.Theme, gtx layout.Context, pane *filePaneState, spec fileContextMenuSpec, anchor image.Point, alpha float32, level int) fileContextMenuPanelState {
	panelSize := ui.fileContextMenuPanelSize(gtx, spec)
	panelRect := image.Rectangle{Min: anchor, Max: anchor.Add(panelSize)}
	pane.ctxMenuRects = append(pane.ctxMenuRects, panelRect)

	theme := ui.filePanePopupTheme()
	activeID := ""
	if level < len(pane.ctxMenuPath) {
		activeID = pane.ctxMenuPath[level]
	}
	state := fileContextMenuPanelState{}

	childGTX := gtx
	childGTX.Constraints = layout.Exact(panelSize)
	offset := op.Offset(anchor).Push(gtx.Ops)
	_ = fixedWidth(childGTX, panelSize.X, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedClipBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			scaleColorAlpha(theme.Bg, alpha),
			scaleColorAlpha(theme.Border, alpha),
			func(gtx layout.Context) layout.Dimensions {
				y := 0
				children := make([]layout.FlexChild, 0, len(spec.Items)+1)
				if strings.TrimSpace(spec.Title) != "" {
					titleH := ui.fileContextMenuTitleHeight(gtx)
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, titleH, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(4), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Caption(th, spec.Title)
								lbl.Font.Typeface = ui.mainTypeface()
								lbl.TextSize = scaleConfigFontSize(ui.fmCfg, 9)
								lbl.Font.Weight = font.Medium
								lbl.Color = scaleColorAlpha(theme.Title, alpha)
								lbl.MaxLines = 1
								lbl.Truncator = "…"
								return layoutVCenteredLabel(gtx, lbl)
							})
						})
					}))
					y += titleH
				}
				for _, item := range spec.Items {
					item := item
					if item.Separator {
						sepH := ui.fileContextMenuSeparatorHeight(gtx)
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, sepH, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									h := gtx.Dp(unit.Dp(1))
									if h < 1 {
										h = 1
									}
									return fillBgExact(gtx, scaleColorAlpha(theme.Divider, alpha), func(gtx layout.Context) layout.Dimensions {
										return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
									})
								})
							})
						}))
						y += sepH
						continue
					}
					rowH := ui.fileContextMenuRowHeight(gtx, item)
					rowRect := image.Rect(anchor.X, anchor.Y+y, anchor.X+panelSize.X, anchor.Y+y+rowH)
					pane.ctxMenuItemRects[item.ID] = rowRect
					y += rowH
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						active := activeID == item.ID && item.hasSubmenu()
						click := pane.contextMenuClick(item.ID)
						hoverFill, hoverAnim := pane.ctxMenuHoverAnim.hoverFill(gtx.Now, item.ID)
						dims, hovered, animating := ui.layoutFilePaneContextMenuItem(th, gtx, theme, click, item, active, hoverFill, alpha, rowH)
						if hovered {
							state.hoveredAny = true
							if item.hasSubmenu() {
								state.hoveredSubmenuID = item.ID
							}
						}
						if hoverAnim || animating {
							gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
						}
						return dims
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			},
		)
	})
	offset.Pop()
	return state
}

func (ui *UI) layoutFilePaneContextMenuItem(th *material.Theme, gtx layout.Context, theme filePanePopupTheme, click *widget.Clickable, item fileContextMenuItem, active bool, hoverFill, alpha float32, rowH int) (layout.Dimensions, bool, bool) {
	if click == nil {
		return layout.Dimensions{}, false, false
	}
	hovered := false
	hoverT := smoothstep01(clamp01(hoverFill))
	dims := click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hovered = click.Hovered()
		if !item.Disabled {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		bg := color.NRGBA{}
		fg := scaleColorAlpha(theme.Text, alpha)
		detailColor := scaleColorAlpha(theme.Muted, alpha)
		if item.Disabled {
			fg = scaleColorAlpha(theme.DisabledText, alpha)
			detailColor = fg
		}
		if active {
			bg = scaleColorAlpha(theme.ActiveBg, alpha)
			fg = scaleColorAlpha(theme.ActiveText, alpha)
			detailColor = scaleColorAlpha(mixNRGBA(theme.ActiveText, theme.ActiveBg, 0.48), alpha)
		} else if !item.Disabled && hoverT > 0 {
			bg = scaleColorAlpha(theme.HoverBg, alpha*hoverT)
			fg = scaleColorAlpha(mixNRGBA(theme.Text, theme.HoverText, hoverT), alpha)
			detailColor = scaleColorAlpha(mixNRGBA(theme.Muted, mixNRGBA(theme.HoverText, theme.HoverBg, 0.48), hoverT), alpha)
		}
		return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
			return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(6), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							if item.Detail == "" {
								lbl := material.Body2(th, item.Label)
								lbl.Font.Typeface = ui.mainTypeface()
								lbl.TextSize = ui.functionBarTextSize()
								lbl.Font.Weight = font.Medium
								lbl.Color = fg
								lbl.MaxLines = 1
								lbl.Truncator = "…"
								return layoutVCenteredLabel(gtx, lbl)
							}
							return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceStart}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									lbl := material.Body2(th, item.Label)
									lbl.Font.Typeface = ui.mainTypeface()
									lbl.TextSize = ui.functionBarTextSize()
									lbl.Font.Weight = font.Medium
									lbl.Color = fg
									lbl.MaxLines = 1
									lbl.Truncator = "…"
									return lbl.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									lbl := material.Caption(th, item.Detail)
									lbl.Font.Typeface = ui.mainTypeface()
									lbl.TextSize = scaleConfigFontSize(ui.fmCfg, unit.Sp(filePaneContextMenuItemDetailTextSp))
									lbl.Color = detailColor
									lbl.MaxLines = 1
									lbl.Truncator = "…"
									return lbl.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if !item.hasSubmenu() {
								return layout.Dimensions{}
							}
							return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, ">")
								lbl.Font.Typeface = ui.mainTypeface()
								lbl.TextSize = ui.functionBarTextSize()
								lbl.Font.Weight = font.Medium
								lbl.Color = fg
								lbl.MaxLines = 1
								return layoutVCenteredLabel(gtx, lbl)
							})
						}),
					)
				})
			})
		})
	})
	return dims, hovered, hoverT > 0 && hoverT < 1
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
	if th != nil && th.Shaper != nil {
		pane.model.setTextMeasurer(func(text string) int {
			lbl := material.Body2(th, text)
			lbl.Font.Typeface = pane.table.Typeface
			lbl.Font.Weight = font.Medium
			lbl.TextSize = pane.table.TextSize
			lbl.MaxLines = 1
			lbl.Truncator = ""
			return measureLabelUnconstrained(gtx, lbl).Size.X
		})
		defer pane.model.setTextMeasurer(nil)
	}

	total := pane.model.Len()
	selectedBefore := pane.table.Selected
	dims := pane.table.Layout(th, gtx, pane.model)
	if pane.loading {
		return dims
	}

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
			pane.clearPendingInlineNameEdit()
			if pane.inlineNameEditing {
				ui.finishInlineFileNameEdit(idx, gtx.Now, true, true)
			}
			if pane.pathEditing {
				pane.stopPathEdit()
				pathEditClosed = true
			}
			if pane.table.HandleScrollSelection(pe.Scroll.Y, total) {
				selectionChanged = true
			}
		case pointer.Press:
			pane.clearPathClickState()
			pos := pe.Position.Round()
			if pane.inlineNameEditing {
				if pane.inlineNameRect.Dx() > 0 && pane.inlineNameRect.Dy() > 0 &&
					pos.X >= pane.inlineNameRect.Min.X && pos.X < pane.inlineNameRect.Max.X &&
					pos.Y >= pane.inlineNameRect.Min.Y && pos.Y < pane.inlineNameRect.Max.Y {
					continue
				}
				ui.finishInlineFileNameEdit(idx, gtx.Now, true, true)
			}
			if pane.pathEditing {
				pane.stopPathEdit()
				pathEditClosed = true
			}
			row := pane.table.HitRow(pos, total)
			col := pane.table.HitColumn(pos)
			if pe.Buttons.Contain(pointer.ButtonPrimary) && row >= 0 && col >= 0 {
				if pe.Modifiers.Contain(key.ModShift) {
					pane.clearPendingInlineNameEdit()
					prev := pane.table.Selected
					pane.table.SetSelected(row, total, false)
					if pane.replaceMarkedRange(prev, row) || prev != pane.table.Selected {
						selectionChanged = true
					}
					continue
				}
				if row == selectedBefore && col == 0 && !pane.hasMarkedRows() {
					if pane.registerTablePrimaryClick(row, col, gtx.Now, filePaneTableDoubleClickWindow) {
						pane.clearPendingInlineNameEdit()
						if col == pane.permissionColumnIndex() {
							if ui.startFilePermDialog(idx, row, gtx.Now) {
								selectionChanged = true
							}
						} else {
							ui.queueFilePaneSystemOpen(idx, row)
						}
						gtx.Execute(op.InvalidateCmd{})
					} else {
						pane.queueInlineNameEdit(row, gtx.Now.Add(filePaneTableDoubleClickWindow))
						gtx.Execute(op.InvalidateCmd{At: pane.inlineNamePendingAt})
					}
					continue
				}
				pane.clearPendingInlineNameEdit()
				if pane.clearMarkedRows() {
					selectionChanged = true
				}
				if pane.registerTablePrimaryClick(row, col, gtx.Now, filePaneTableDoubleClickWindow) {
					if col == pane.permissionColumnIndex() {
						if ui.startFilePermDialog(idx, row, gtx.Now) {
							selectionChanged = true
						}
					} else {
						ui.queueFilePaneSystemOpen(idx, row)
					}
					gtx.Execute(op.InvalidateCmd{})
				}
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) && (row < 0 || col < 0) {
				pane.clearPendingInlineNameEdit()
				if pane.clearMarkedRows() {
					selectionChanged = true
				}
			}
			if !pe.Buttons.Contain(pointer.ButtonSecondary) {
				continue
			}
			pane.clearPendingInlineNameEdit()
			if ui.openFilePaneContextMenuAtPointer(idx, pane, pos, gtx.Now) {
				selectionChanged = true
			}
		}
	}
	if pane.inlineNamePendingRow >= 0 {
		if gtx.Now.Before(pane.inlineNamePendingAt) {
			gtx.Execute(op.InvalidateCmd{At: pane.inlineNamePendingAt})
		} else if ui.activatePendingInlineNameEdit(idx, gtx.Now) {
			selectionChanged = true
			gtx.Execute(op.InvalidateCmd{})
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

func (ui *UI) layoutFilePaneInlineNameEditor(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	if pane == nil || !pane.inlineNameEditing {
		if pane != nil {
			pane.inlineNameRect = image.Rectangle{}
		}
		return layout.Dimensions{}
	}

	for {
		ev, ok := gtx.Event(key.Filter{Focus: &pane.inlineNameEdit, Name: key.NameEscape})
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		ui.finishInlineFileNameEdit(idx, gtx.Now, false, true)
		return layout.Dimensions{}
	}
	for {
		ev, ok := pane.inlineNameEdit.Update(gtx)
		if !ok {
			break
		}
		switch submit := ev.(type) {
		case widget.SubmitEvent:
			pane.inlineNameEdit.SetText(submit.Text)
			if ui.finishInlineFileNameEdit(idx, gtx.Now, true, false) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case widget.ChangeEvent:
			pane.err = ""
		}
	}
	if !pane.inlineNameEditing {
		pane.inlineNameRect = image.Rectangle{}
		return layout.Dimensions{}
	}
	if pane.inlineNameFocus {
		pane.inlineNameFocus = false
		gtx.Execute(key.FocusCmd{Tag: &pane.inlineNameEdit})
	} else if !gtx.Focused(&pane.inlineNameEdit) {
		ui.finishInlineFileNameEdit(idx, gtx.Now, true, true)
		if !pane.inlineNameEditing {
			pane.inlineNameRect = image.Rectangle{}
			return layout.Dimensions{}
		}
	}

	rect, ok := ui.inlineFileNameEditRect(gtx, pane)
	if !ok {
		pane.inlineNameRect = image.Rectangle{}
		return layout.Dimensions{}
	}
	pane.inlineNameRect = rect

	ed := material.Editor(th, &pane.inlineNameEdit, "")
	ed.Font.Typeface = ui.mainTypeface()
	ed.TextSize = pane.table.TextSize
	ed.Color = pane.model.paneTextColor()
	ed.HintColor = hintColor

	m := op.Record(gtx.Ops)
	childGTX := gtx
	childGTX.Constraints = layout.Exact(rect.Size())
	_ = ui.layoutEditorWithContextMenu(th, childGTX, "pane-name-"+strconv.Itoa(idx), &pane.inlineNameEdit, true, func(gtx layout.Context) layout.Dimensions {
		return layoutCompactNeutralEditorBox(gtx, gtx.Focused(&pane.inlineNameEdit), true, ed.Layout)
	})
	call := m.Stop()

	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	offset := op.Offset(rect.Min).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: gtx.Constraints.Max}
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
		palette := filePanePaletteFromConfig(ui.fmCfg)
		rowBg, rowBorder := filePanePathRowColors(palette)
		stripH := gtx.Dp(unit.Dp(22))
		if stripH < 1 {
			stripH = 1
		}
		return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), rowBg, rowBorder, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(1), Right: unit.Dp(1), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								m := op.Record(gtx.Ops)
								pathDims := ui.layoutFilePanePath(th, gtx, idx, pane, active)
								pathCall := m.Stop()
								fillH := pathDims.Size.Y
								if fillH < 1 {
									fillH = stripH
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
						})
					})
				})
			})
		})
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
		pane.closeSortMenu()
		pane.closeDriveMenu()
		pane.closeFavoriteMenu()
		pane.closeContextMenu()
		if pane.registerPathClick("row:"+pane.dir, gtx.Now, filePanePathDoubleClickWindow) {
			pane.clearPendingPathNavigate()
			pane.beginPathEdit()
		}
	}
}

func (ui *UI) processFilePaneDriveSegmentInput(gtx layout.Context, idx int, pane *filePaneState) {
	if pane == nil || pane.remoteConnected() || localDriveRoot(pane.displayDir()) == "" {
		if pane != nil {
			pane.driveSegmentRect = image.Rectangle{}
		}
		return
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &pane.drivePointerTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok || pe.Kind != pointer.Press || !pe.Buttons.Contain(pointer.ButtonSecondary) {
			continue
		}
		if len(platform.AvailableLocalDrives()) == 0 {
			continue
		}
		ui.setActiveFilePane(idx)
		ui.closeSortMenusExcept(idx)
		ui.closeDriveMenusExcept(idx)
		ui.closeFavoriteMenusExcept(idx)
		ui.closeContextMenusExcept(idx)
		pane.closeSortMenu()
		pane.closeFavoriteMenu()
		pane.closeContextMenu()
		pane.stopPathEdit()
		pane.openDriveMenu(image.Point{
			X: pe.Position.Round().X,
			Y: pane.headerHeight + gtx.Dp(unit.Dp(4)),
		}, gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) layoutFilePanePathFill(gtx layout.Context, fillH int) layout.Dimensions {
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, fillH)}
}

func filePanePathBaseColor(palette filePanePalette) color.NRGBA {
	return palette.CurrentDirFg
}

func filePanePathMutedColor(palette filePanePalette) color.NRGBA {
	return mixNRGBA(palette.CurrentDirFg, hintColor, 0.38)
}

func filePanePathRowColors(palette filePanePalette) (bg, border color.NRGBA) {
	bg = palette.CurrentDirBg
	border = mixNRGBA(palette.CurrentDirFg, palette.CurrentDirBg, 0.78)
	border.A = 34
	return bg, border
}

func filePanePathHoverColors(palette filePanePalette) (bg, fg, border color.NRGBA) {
	bg = mixNRGBA(palette.CurrentDirBg, palette.HoverBg, 0.78)
	bg.A = 255
	fg = bestContrastColor(bg, palette.HoverFg, palette.CurrentDirFg)
	border = mixNRGBA(fg, bg, 0.52)
	border.A = 88
	return bg, fg, border
}

func (ui *UI) layoutFilePanePathSegmentLabel(th *material.Theme, gtx layout.Context, label string, bg, fg, border color.NRGBA, weight font.Weight) layout.Dimensions {
	segH := gtx.Constraints.Min.Y
	if gtx.Constraints.Max.Y > segH {
		segH = gtx.Constraints.Max.Y
	}
	if segH < 1 {
		segH = gtx.Dp(unit.Dp(18))
	}
	content := func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, segH, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.Font.Weight = weight
				lbl.TextSize = scaleThemeFontSize(th, 11)
				lbl.Color = fg
				lbl.MaxLines = 1
				return layoutVCenteredLabel(gtx, lbl)
			})
		})
	}
	if bg.A == 0 && border.A == 0 {
		return content(gtx)
	}
	return fillRoundedBox(gtx, gtx.Dp(unit.Dp(4)), bg, border, content)
}

func (ui *UI) layoutFilePanePath(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState, active bool) layout.Dimensions {
	if pane == nil {
		return layout.Dimensions{}
	}
	palette := filePanePaletteFromConfig(ui.fmCfg)
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

	if pane.remoteConnected() {
		address := "ssh"
		if pane.remote != nil {
			if prefix := strings.TrimSpace(pane.remote.displayPrefix()); prefix != "" {
				address = prefix
			}
		}
		segments := remotePathDisplaySegments(address, pane.dir)
		pane.ensurePathClicks(len(segments))
		for i := range segments {
			click := &pane.pathSegClicks[i]
			for {
				_, ok := click.Update(gtx)
				if !ok {
					break
				}
				ui.setActiveFilePane(idx)
				pane.closeSortMenu()
				pane.closeDriveMenu()
				pane.closeFavoriteMenu()
				pane.closeContextMenu()
				if i != len(segments)-1 {
					if ui.activateFilePanePathSegment(idx, pane, segments[i].path) {
						gtx.Execute(op.InvalidateCmd{})
						return ui.layoutFilePanePath(th, gtx, idx, pane, active)
					}
				}
			}
			if pane.pathEditing {
				return ui.layoutFilePanePathEditor(th, gtx, idx, pane, active)
			}
		}

		pathBaseColor := filePanePathBaseColor(palette)
		pathHoverBg, pathHoverColor, pathHoverBorder := filePanePathHoverColors(palette)
		children := make([]layout.FlexChild, 0, len(segments))
		for i := range segments {
			i := i
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				click := &pane.pathSegClicks[i]
				return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					bg := color.NRGBA{}
					lblColor := pathBaseColor
					border := color.NRGBA{}
					weight := font.Normal
					if click.Hovered() {
						bg = pathHoverBg
						lblColor = pathHoverColor
						border = pathHoverBorder
					}
					if i == len(segments)-1 {
						weight = font.Medium
					}
					return ui.layoutFilePanePathSegmentLabel(th, gtx, segments[i].label, bg, lblColor, border, weight)
				})
			}))
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	}

	segments := splitFilePathSegments(pane.displayDir())
	ui.processFilePaneDriveSegmentInput(gtx, idx, pane)
	showLoadingHint := pane.loadingHintVisible(gtx.Now)
	if pane.loading && !showLoadingHint && !pane.loadingStartedAt.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: pane.loadingStartedAt.Add(filePaneLoadingHintDelay)})
	}
	pane.ensurePathClicks(len(segments))
	for i := range segments {
		click := &pane.pathSegClicks[i]
		for {
			_, ok := click.Update(gtx)
			if !ok {
				break
			}
			ui.setActiveFilePane(idx)
			pane.closeSortMenu()
			pane.closeDriveMenu()
			pane.closeFavoriteMenu()
			pane.closeContextMenu()
			if i != len(segments)-1 {
				if ui.activateFilePanePathSegment(idx, pane, segments[i].path) {
					gtx.Execute(op.InvalidateCmd{})
					return ui.layoutFilePanePath(th, gtx, idx, pane, active)
				}
			}
		}
		if pane.pathEditing {
			return ui.layoutFilePanePathEditor(th, gtx, idx, pane, active)
		}
	}

	children := make([]layout.FlexChild, 0, len(segments))
	baseColor := filePanePathBaseColor(palette)
	hoverBg, hoverColor, hoverBorder := filePanePathHoverColors(palette)
	for i := range segments {
		i := i
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			click := &pane.pathSegClicks[i]
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				bg := color.NRGBA{}
				lblColor := baseColor
				border := color.NRGBA{}
				weight := font.Normal
				if click.Hovered() {
					bg = hoverBg
					lblColor = hoverColor
					border = hoverBorder
				}
				if i == len(segments)-1 {
					weight = font.Medium
				}
				dims := ui.layoutFilePanePathSegmentLabel(th, gtx, segments[i].label, bg, lblColor, border, weight)
				if i == 0 && localDriveRoot(pane.displayDir()) != "" {
					pane.driveSegmentRect = image.Rectangle{Max: dims.Size}
					defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
					pass := pointer.PassOp{}.Push(gtx.Ops)
					event.Op(gtx.Ops, &pane.drivePointerTag)
					pass.Pop()
				}
				return dims
			})
		}))
	}
	if showLoadingHint {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, " [loading...]")
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.Font.Weight = font.Medium
				lbl.TextSize = scaleThemeFontSize(th, 10)
				lbl.Color = filePanePathMutedColor(palette)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
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
		return ui.layoutFilePanePathArea(th, gtx, idx, pane, active)
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
		return ui.layoutFilePanePathArea(th, gtx, idx, pane, active)
	}
	if pane.pathEditFocus {
		pane.pathEditFocus = false
		gtx.Execute(key.FocusCmd{Tag: &pane.pathEdit})
	} else if !gtx.Focused(&pane.pathEdit) {
		pane.stopPathEdit()
		return ui.layoutFilePanePathArea(th, gtx, idx, pane, active)
	}

	ed := material.Editor(th, &pane.pathEdit, "")
	ed.Font.Typeface = ui.mainTypeface()
	ed.TextSize = scaleThemeFontSize(th, 12)
	palette := filePanePaletteFromConfig(ui.fmCfg)
	ed.Color = palette.PaneFg
	ed.HintColor = filePanePathMutedColor(palette)

	return ui.layoutEditorWithContextMenu(th, gtx, "pane-path-"+strconv.Itoa(idx), &pane.pathEdit, true, func(gtx layout.Context) layout.Dimensions {
		return layoutNeutralEditorBox(gtx, gtx.Focused(&pane.pathEdit), true, ed.Layout)
	})
}

func (ui *UI) layoutFilePaneSortOptionsStrip(th *material.Theme, gtx layout.Context, pane *filePaneState, sortOptions []struct {
	key   fileSortKey
	label string
}) layout.Dimensions {
	if pane == nil || len(sortOptions) == 0 {
		return layout.Dimensions{}
	}
	gtx.Constraints.Min.X = 0
	theme := ui.filePanePopupTheme()
	alpha, _, animating := popupOpenProgress(gtx.Now, pane.sortMenuOpenedAt)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	hoverID := filePaneSortOptionHoveredID(pane, sortOptions)
	if hoverID != pane.sortMenuHoverID {
		pane.sortMenuHoverID = hoverID
		pane.sortMenuHoverAnim.setHover(hoverID, gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	stripH := gtx.Dp(unit.Dp(22))
	if stripH < 1 {
		stripH = 1
	}
	return fillRoundedClipBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
		scaleColorAlpha(theme.Bg, alpha),
		scaleColorAlpha(theme.Border, alpha),
		func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(sortOptions)*2-1)
				for i, opt := range sortOptions {
					if i > 0 {
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutFilePaneControlDividerColor(gtx, stripH, scaleColorAlpha(theme.Divider, alpha))
						}))
					}
					i := i
					opt := opt
					activeOpt := pane.sortKey == opt.key
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						hoverFill, hoverAnim := pane.sortMenuHoverAnim.hoverFill(gtx.Now, filePaneSortOptionID(opt.key))
						if hoverAnim {
							gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
						}
						return pane.sortOptionBtns[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							pointer.CursorPointer.Add(gtx.Ops)
							hoverT := smoothstep01(clamp01(hoverFill))
							bg := color.NRGBA{}
							fg := scaleColorAlpha(theme.Text, alpha)
							if activeOpt {
								bg = scaleColorAlpha(theme.ActiveBg, alpha)
								fg = scaleColorAlpha(theme.ActiveText, alpha)
							} else if hoverT > 0 {
								bg = scaleColorAlpha(theme.HoverBg, alpha*hoverT)
								fg = scaleColorAlpha(mixNRGBA(theme.Text, theme.HoverText, hoverT), alpha)
							}
							return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									lbl := material.Body2(th, opt.label)
									lbl.Font.Typeface = ui.mainTypeface()
									lbl.Font.Weight = font.Medium
									lbl.TextSize = ui.functionBarTextSize()
									lbl.Color = fg
									lbl.MaxLines = 1
									return layoutVCenteredLabel(gtx, lbl)
								})
							})
						})
					}))
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
			})
		},
	)
}

func layoutFilePaneControlDivider(gtx layout.Context, h int) layout.Dimensions {
	return layoutFilePaneControlDividerColor(gtx, h, color.NRGBA{R: 255, G: 255, B: 255, A: 22})
}

func layoutFilePaneControlDividerColor(gtx layout.Context, h int, fill color.NRGBA) layout.Dimensions {
	w := gtx.Dp(unit.Dp(1))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, fill, clip.Rect(image.Rect(0, 0, w, h)).Op())
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
		pane.closeDriveMenu()
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
			pane.closeDriveMenu()
			next := !pane.sortMenuOpen
			ui.closeSortMenusExcept(idx)
			ui.closeDriveMenusExcept(idx)
			ui.closeFavoriteMenusExcept(idx)
			pane.closeContextMenu()
			if next {
				pane.openSortMenu(gtx.Now)
				pane.closeFavoriteMenu()
			} else {
				pane.closeSortMenu()
			}
		}
	}

	if pane.sortClick.Clicked(gtx) {
		pane.clearPendingPathNavigate()
		pane.stopPathEdit()
		pane.closeDriveMenu()
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
		pane.closeDriveMenu()
		next := !pane.favoriteMenuOpen
		ui.closeFavoriteMenusExcept(idx)
		ui.closeDriveMenusExcept(idx)
		ui.closeSortMenusExcept(idx)
		pane.closeContextMenu()
		pane.closeSortMenu()
		if next {
			pane.openFavoriteMenu(gtx.Now)
		} else {
			pane.closeFavoriteMenu()
		}
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) processFilePaneDisconnectInput(gtx layout.Context, idx int, pane *filePaneState) {
	if pane == nil || !pane.remoteConnected() {
		return
	}
	if pane.disconnectClick.Clicked(gtx) {
		ui.setActiveFilePane(idx)
		pane.closeDriveMenu()
		ui.disconnectPaneSSH(idx, gtx.Now)
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
	ui.processFilePaneDisconnectInput(gtx, idx, pane)

	stripH := gtx.Dp(unit.Dp(22))
	if stripH < 1 {
		stripH = 1
	}
	return fillRoundedClipBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 18, G: 22, B: 30, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 22},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(1), Right: unit.Dp(1), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0, 10)
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
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
									return ui.layoutFilePaneModeIcon(gtx, pane.table.Mode, iconColor)
								})
							})
						})
					}))
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layoutFilePaneControlDivider(gtx, stripH)
					}))
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
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
									return layoutVCenteredLabel(gtx, lbl)
								})
							})
						})
					}))
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layoutFilePaneControlDivider(gtx, stripH)
					}))
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
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
									size := gtx.Dp(unit.Dp(14))
									if size < 1 {
										size = 1
									}
									return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										if ic := uitheme.FavoriteIcon(pane.favoriteMenuOpen); ic != nil {
											iconGtx := gtx
											iconGtx.Constraints = layout.Exact(image.Pt(size, size))
											ic.Layout(iconGtx, fg)
										}
										return layout.Dimensions{Size: image.Pt(size, size)}
									})
								})
							})
						})
					}))
					if pane.remoteConnected() {
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutFilePaneControlDivider(gtx, stripH)
						}))
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return pane.disconnectClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								pointer.CursorPointer.Add(gtx.Ops)
								bg := color.NRGBA{}
								iconColor := txtColor
								if pane.disconnectClick.Hovered() {
									bg = color.NRGBA{R: 56, G: 38, B: 38, A: 255}
									iconColor = color.NRGBA{R: 255, G: 208, B: 208, A: 255}
								}
								return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										size := gtx.Dp(unit.Dp(13))
										if size < 1 {
											size = 1
										}
										if ic := uitheme.DisconnectIcon(); ic != nil {
											iconGtx := gtx
											iconGtx.Constraints = layout.Exact(image.Pt(size, size))
											ic.Layout(iconGtx, iconColor)
										}
										return layout.Dimensions{Size: image.Pt(size, size)}
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

		return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), bg, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFilePaneModeIcon(gtx, pane.table.Mode, iconColor)
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

func (ui *UI) layoutFilePaneModeIcon(gtx layout.Context, mode table.Mode, iconColor color.NRGBA) layout.Dimensions {
	size := image.Pt(gtx.Dp(unit.Dp(16)), gtx.Dp(unit.Dp(11)))
	if max := gtx.Constraints.Max; max.X > 0 && size.X > max.X {
		size.X = max.X
	}
	if max := gtx.Constraints.Max; max.Y > 0 && size.Y > max.Y {
		size.Y = max.Y
	}
	if size.X < 10 {
		size.X = 10
	}
	if size.Y < 8 {
		size.Y = 8
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		iconGtx := gtx
		iconGtx.Constraints = layout.Exact(size)
		return layoutFilePaneModeGlyph(iconGtx, mode, iconColor)
	})
}

func layoutFilePaneModeGlyph(gtx layout.Context, mode table.Mode, barColor color.NRGBA) layout.Dimensions {
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
	if size.Y < 8 {
		size.Y = 8
	}

	barH := size.Y / 5
	if barH < 2 {
		barH = 2
	}
	if barH > 3 {
		barH = 3
	}
	gapY := (size.Y - 3*barH) / 2
	if gapY < 1 {
		gapY = 1
	}
	usedH := 3*barH + 2*gapY
	top := (size.Y - usedH) / 2
	if top < 0 {
		top = 0
	}

	padX := size.X / 10
	if padX < 1 {
		padX = 1
	}
	left := padX
	totalW := size.X - 2*padX
	if totalW < 6 {
		totalW = size.X
		left = 0
	}
	splitGap := totalW / 5
	if splitGap < 2 {
		splitGap = 2
	}
	if splitGap > 4 {
		splitGap = 4
	}
	splitW := (totalW - splitGap) / 2
	if splitW < 2 {
		splitW = 2
	}

	drawBar := func(x, y, w int) {
		if w < 1 {
			return
		}
		rect := image.Rect(x, y, x+w, y+barH)
		radius := barH / 2
		if radius < 1 {
			radius = 1
		}
		paint.FillShape(gtx.Ops, barColor, clip.UniformRRect(rect, radius).Op(gtx.Ops))
	}

	for row := 0; row < 3; row++ {
		y := top + row*(barH+gapY)
		if mode == table.ModeBrief {
			drawBar(left, y, splitW)
			drawBar(left+totalW-splitW, y, splitW)
			continue
		}
		drawBar(left, y, totalW)
	}

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

	dims := layoutTinyIconModeButton(th, gtx, &pane.favoriteClick, uitheme.FavoriteIcon(pane.favoriteMenuOpen), pane.favoriteMenuOpen)
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}

	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func (ui *UI) layoutFilePaneDriveMenu(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	if pane == nil || !pane.driveMenuOpen || pane.remoteConnected() {
		return layout.Dimensions{}
	}

	drives := platform.AvailableLocalDrives()
	if len(drives) == 0 {
		pane.closeDriveMenu()
		return layout.Dimensions{}
	}

	pane.ensureDriveMenuClicks(len(drives))
	for i, drive := range drives {
		if !pane.driveMenuClicks[i].Clicked(gtx) {
			continue
		}
		pane.closeDriveMenu()
		ui.requestPaneLoadWithSelection(idx, drive, "", "", 0)
		gtx.Execute(op.InvalidateCmd{})
	}

	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &pane.driveMenuPointerTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok || pe.Kind != pointer.Press {
			continue
		}
		pos := pe.Position.Round()
		inMenu := pane.driveMenuRect.Dx() > 0 && pane.driveMenuRect.Dy() > 0 &&
			pos.X >= pane.driveMenuRect.Min.X && pos.X < pane.driveMenuRect.Max.X &&
			pos.Y >= pane.driveMenuRect.Min.Y && pos.Y < pane.driveMenuRect.Max.Y
		inSegment := pane.driveSegmentRect.Dx() > 0 && pane.driveSegmentRect.Dy() > 0 &&
			pos.X >= pane.driveSegmentRect.Min.X && pos.X < pane.driveSegmentRect.Max.X &&
			pos.Y >= pane.driveSegmentRect.Min.Y && pos.Y < pane.driveSegmentRect.Max.Y
		if !inMenu && !inSegment {
			pane.closeDriveMenu()
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	if !pane.driveMenuOpen {
		return layout.Dimensions{}
	}

	alpha, slideY, animating := popupOpenProgress(gtx.Now, pane.driveMenuOpenedAt)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	m := op.Record(gtx.Ops)
	menuDims := ui.layoutFilePaneDriveMenuCard(th, gtx, pane, drives, alpha)
	call := m.Stop()

	anchor := clampFilePaneMenuPoint(pane.driveMenuPos, menuDims.Size, gtx.Constraints.Max)
	anchor.Y += slideY
	anchor = clampFilePaneMenuPoint(anchor, menuDims.Size, gtx.Constraints.Max)
	pane.driveMenuRect = image.Rectangle{Min: anchor, Max: anchor.Add(menuDims.Size)}

	bodyClip := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	offset := op.Offset(anchor).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	bodyClip.Pop()

	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &pane.driveMenuPointerTag)
	pass.Pop()

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *UI) driveMenuHoveredID(pane *filePaneState, drives []string) string {
	if pane == nil {
		return ""
	}
	for i, drive := range drives {
		if i < len(pane.driveMenuClicks) && pane.driveMenuClicks[i].Hovered() {
			return drive
		}
	}
	return ""
}

func (ui *UI) driveMenuCardWidth(th *material.Theme, gtx layout.Context, drives []string) int {
	maxTextW := 0
	for _, drive := range drives {
		lbl := material.Body2(th, drive)
		lbl.Font.Typeface = ui.mainTypeface()
		lbl.Font.Weight = font.Medium
		lbl.TextSize = ui.functionBarTextSize()
		lbl.MaxLines = 1
		if w := measureLabelUnconstrained(gtx, lbl).Size.X; w > maxTextW {
			maxTextW = w
		}
	}
	if maxTextW == 0 {
		maxTextW = gtx.Dp(unit.Dp(64))
	}
	width := maxTextW + gtx.Dp(unit.Dp(26))
	if width < gtx.Dp(unit.Dp(118)) {
		width = gtx.Dp(unit.Dp(118))
	}
	if width > gtx.Constraints.Max.X {
		width = gtx.Constraints.Max.X
	}
	if width < 1 {
		width = 1
	}
	return width
}

func (ui *UI) layoutFilePaneDriveMenuCard(th *material.Theme, gtx layout.Context, pane *filePaneState, drives []string, alpha float32) layout.Dimensions {
	width := ui.driveMenuCardWidth(th, gtx, drives)
	currentDrive := localDriveRoot(pane.displayDir())
	theme := ui.filePanePopupTheme()
	hoverID := ui.driveMenuHoveredID(pane, drives)
	if hoverID != pane.driveMenuHoverID {
		pane.driveMenuHoverID = hoverID
		pane.driveMenuHoverAnim.setHover(hoverID, gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}

	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedClipBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			scaleColorAlpha(theme.Bg, alpha),
			scaleColorAlpha(theme.Border, alpha),
			func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(drives)+1)
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, ui.fileContextMenuTitleHeight(gtx), func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(4), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Caption(th, "Drives")
							lbl.Font.Typeface = ui.mainTypeface()
							lbl.Font.Weight = font.Medium
							lbl.TextSize = scaleConfigFontSize(ui.fmCfg, 9)
							lbl.Color = scaleColorAlpha(theme.Title, alpha)
							lbl.MaxLines = 1
							lbl.Truncator = "…"
							return layoutVCenteredLabel(gtx, lbl)
						})
					})
				}))
				for i, drive := range drives {
					i := i
					drive := drive
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						active := strings.EqualFold(currentDrive, drive)
						item := fileContextMenuItem{ID: "drive:" + drive, Label: drive}
						hoverFill, hoverAnim := pane.driveMenuHoverAnim.hoverFill(gtx.Now, drive)
						if hoverAnim {
							gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
						}
						dims, _, animating := ui.layoutFilePaneContextMenuItem(
							th,
							gtx,
							theme,
							&pane.driveMenuClicks[i],
							item,
							active,
							hoverFill,
							alpha,
							ui.fileContextMenuRowHeight(gtx, item),
						)
						if animating {
							gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
						}
						return dims
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			},
		)
	})
}

func (ui *UI) layoutFilePaneFavoriteMenu(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState) layout.Dimensions {
	if pane == nil || !pane.favoriteMenuOpen {
		return layout.Dimensions{}
	}

	items := ui.paneFavoriteItems(pane)
	pane.ensureFavoriteOptionClicks(len(items))
	pane.ensureFavoriteRemoveClicks(len(items))
	alpha, slideY, animating := popupOpenProgress(gtx.Now, pane.favoriteMenuOpenedAt)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	menuRect := ui.filePaneFavoriteMenuBaseRect(gtx, pane, items, slideY)

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
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &pane.favoritePointerTag,
			Kinds:  pointer.Move | pointer.Enter | pointer.Leave,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		pos := pe.Position.Round()
		switch pe.Kind {
		case pointer.Enter:
			pane.favoritePointerPos = pos
			pane.favoritePointerPosSet = true
		case pointer.Move:
			pane.favoritePointerPos = pos
			pane.favoritePointerPosSet = true
		case pointer.Leave:
			pane.favoritePointerPos = image.Point{}
			pane.favoritePointerPosSet = false
		}
	}
	ui.updateFilePaneFavoriteHover(th, gtx, pane, menuRect, items)
	pane.favoriteMenuRect = menuRect
	revealRect := ui.favoriteMenuRevealRect(th, gtx, pane, menuRect, items)
	if revealRect.Dx() > 0 && revealRect.Dy() > 0 {
		if revealRect.Min.X < pane.favoriteMenuRect.Min.X {
			pane.favoriteMenuRect.Min.X = revealRect.Min.X
		}
		if revealRect.Min.Y < pane.favoriteMenuRect.Min.Y {
			pane.favoriteMenuRect.Min.Y = revealRect.Min.Y
		}
		if revealRect.Max.X > pane.favoriteMenuRect.Max.X {
			pane.favoriteMenuRect.Max.X = revealRect.Max.X
		}
		if revealRect.Max.Y > pane.favoriteMenuRect.Max.Y {
			pane.favoriteMenuRect.Max.Y = revealRect.Max.Y
		}
	}

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
	ui.layoutFilePaneFavoriteMenuCard(th, gtx, pane, items, alpha)
	call := m.Stop()

	bodyClip := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	offset := op.Offset(menuRect.Min).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	ui.layoutFilePaneFavoriteReveal(th, gtx, pane, menuRect, items, alpha)
	bodyClip.Pop()

	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &pane.favoritePointerTag)
	pass.Pop()

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func filePaneSortOptionID(key fileSortKey) string {
	switch key {
	case fileSortName:
		return "sort-name"
	case fileSortDate:
		return "sort-date"
	case fileSortExt:
		return "sort-ext"
	case fileSortSize:
		return "sort-size"
	default:
		return "sort-unknown"
	}
}

func filePaneSortOptionHoveredID(pane *filePaneState, sortOptions []struct {
	key   fileSortKey
	label string
}) string {
	if pane == nil {
		return ""
	}
	for i, opt := range sortOptions {
		if i < len(pane.sortOptionBtns) && pane.sortOptionBtns[i].Hovered() {
			return filePaneSortOptionID(opt.key)
		}
	}
	return ""
}

func filePaneFavoriteMenuItemID(item fileFavoriteItem) string {
	switch {
	case item.addCurrent:
		return "favorite-add-current"
	case item.targetDir != "":
		return "favorite-" + item.targetDir
	case item.label != "":
		return "favorite-" + item.label
	default:
		return "favorite-item"
	}
}

func filePaneFavoriteMenuHoveredID(pane *filePaneState, items []fileFavoriteItem) string {
	if pane == nil {
		return ""
	}
	for i, item := range items {
		if item.disabled {
			continue
		}
		if i < len(pane.favoriteOptionClicks) && pane.favoriteOptionClicks[i].Hovered() {
			return filePaneFavoriteMenuItemID(item)
		}
	}
	return ""
}

func filePaneFavoriteRevealedItem(pane *filePaneState, now time.Time, items []fileFavoriteItem) (int, fileFavoriteItem, bool) {
	if pane == nil {
		return -1, fileFavoriteItem{}, false
	}
	for i, item := range items {
		if filePaneFavoriteLabelRevealed(pane, now, item) {
			return i, item, true
		}
	}
	return -1, fileFavoriteItem{}, false
}

func (ui *UI) favoriteMenuRevealHotspotRect(th *material.Theme, gtx layout.Context, menuRect image.Rectangle, items []fileFavoriteItem, index int, item fileFavoriteItem) image.Rectangle {
	if menuRect.Dx() <= 0 || menuRect.Dy() <= 0 || index < 0 || index >= len(items) || item.disabled || item.addCurrent {
		return image.Rectangle{}
	}
	_, _, hiddenPrefixWidth, ellipsisWidth := ui.favoriteMenuLabelMetrics(th, gtx, item)
	if hiddenPrefixWidth <= 0 || ellipsisWidth <= 0 {
		return image.Rectangle{}
	}
	hotspotWidth := ellipsisWidth + gtx.Dp(unit.Dp(filePaneFavoriteRevealHotspotPadDp))
	maxWidth := filePaneFavoriteMenuTextWidth(gtx, item)
	if hotspotWidth > maxWidth {
		hotspotWidth = maxWidth
	}
	if hotspotWidth < ellipsisWidth {
		hotspotWidth = ellipsisWidth
	}
	rowTop := menuRect.Min.Y + filePaneFavoriteMenuItemOffsetY(gtx, items, index)
	rowH := filePaneFavoriteMenuRowHeight(gtx)
	left := menuRect.Min.X + gtx.Dp(unit.Dp(7))
	return image.Rect(left, rowTop, left+hotspotWidth, rowTop+rowH)
}

func (ui *UI) filePaneFavoriteRevealHotspotKey(th *material.Theme, gtx layout.Context, pane *filePaneState, menuRect image.Rectangle, items []fileFavoriteItem) string {
	if pane == nil || !pane.favoritePointerPosSet {
		return ""
	}
	pos := pane.favoritePointerPos
	for i, item := range items {
		if item.targetDir == "" || item.disabled || item.addCurrent {
			continue
		}
		if filePaneFavoriteRectContains(ui.favoriteMenuRevealHotspotRect(th, gtx, menuRect, items, i, item), pos) {
			return item.targetDir
		}
	}
	return ""
}

func (ui *UI) favoriteMenuRevealRect(th *material.Theme, gtx layout.Context, pane *filePaneState, menuRect image.Rectangle, items []fileFavoriteItem) image.Rectangle {
	revealIndex, revealItem, ok := filePaneFavoriteRevealedItem(pane, gtx.Now, items)
	if !ok {
		return image.Rectangle{}
	}
	revealWidth := ui.favoriteMenuRevealWidth(th, gtx, revealItem)
	maxWidth := menuRect.Max.X
	if revealWidth > maxWidth {
		revealWidth = maxWidth
	}
	if revealWidth <= menuRect.Dx() {
		return image.Rectangle{}
	}
	rowTop := menuRect.Min.Y + filePaneFavoriteMenuItemOffsetY(gtx, items, revealIndex)
	rowH := filePaneFavoriteMenuRowHeight(gtx)
	return image.Rect(menuRect.Max.X-revealWidth, rowTop, menuRect.Max.X, rowTop+rowH)
}

func (ui *UI) updateFilePaneFavoriteHover(th *material.Theme, gtx layout.Context, pane *filePaneState, menuRect image.Rectangle, items []fileFavoriteItem) {
	if pane == nil {
		return
	}
	now := gtx.Now
	if pane.favoriteRevealKey != "" && !pane.favoriteRevealHideAt.IsZero() {
		fadeEnd := pane.favoriteRevealHideAt.Add(filePaneFavoriteRevealFadeDur)
		if now.Before(pane.favoriteRevealHideAt) {
			gtx.Execute(op.InvalidateCmd{At: pane.favoriteRevealHideAt})
		} else if now.Before(fadeEnd) {
			gtx.Execute(op.InvalidateCmd{At: now.Add(16 * time.Millisecond)})
		} else {
			pane.favoriteRevealKey = ""
			pane.favoriteRevealHideAt = time.Time{}
		}
	}

	hoveredKey := ui.filePaneFavoriteRevealHotspotKey(th, gtx, pane, menuRect, items)

	if hoveredKey == "" {
		pane.favoriteHoverKey = ""
		pane.favoriteHoverAt = time.Time{}
		if pane.favoriteRevealKey != "" && pane.favoriteRevealHideAt.IsZero() {
			pane.favoriteRevealHideAt = now
			gtx.Execute(op.InvalidateCmd{At: now.Add(16 * time.Millisecond)})
		}
		return
	}
	if pane.favoriteRevealKey == hoveredKey {
		if !pane.favoriteRevealHideAt.IsZero() {
			pane.favoriteRevealHideAt = time.Time{}
			gtx.Execute(op.InvalidateCmd{})
		}
		return
	}
	if pane.favoriteHoverKey != hoveredKey {
		pane.favoriteHoverKey = hoveredKey
		pane.favoriteHoverAt = now
		if pane.favoriteRevealKey != "" && pane.favoriteRevealHideAt.IsZero() {
			pane.favoriteRevealHideAt = now
			gtx.Execute(op.InvalidateCmd{At: now.Add(16 * time.Millisecond)})
		}
	}
	if pane.favoriteHoverAt.IsZero() {
		pane.favoriteHoverAt = now
	}
	showAt := pane.favoriteHoverAt.Add(filePaneFavoriteRevealDelay)
	if gtx.Now.Before(showAt) {
		gtx.Execute(op.InvalidateCmd{At: showAt})
		return
	}
	pane.favoriteRevealKey = hoveredKey
	pane.favoriteRevealHideAt = time.Time{}
}

func filePaneFavoriteRevealAlpha(pane *filePaneState, now time.Time, item fileFavoriteItem) float32 {
	if pane == nil || item.disabled || item.addCurrent || item.targetDir == "" || pane.favoriteRevealKey == "" || pane.favoriteRevealKey != item.targetDir {
		return 0
	}
	if pane.favoriteRevealHideAt.IsZero() || now.Before(pane.favoriteRevealHideAt) {
		return 1
	}
	fadeEnd := pane.favoriteRevealHideAt.Add(filePaneFavoriteRevealFadeDur)
	if !now.Before(fadeEnd) {
		return 0
	}
	return 1 - clamp01(float32(now.Sub(pane.favoriteRevealHideAt))/float32(filePaneFavoriteRevealFadeDur))
}

func filePaneFavoriteLabelRevealed(pane *filePaneState, now time.Time, item fileFavoriteItem) bool {
	return filePaneFavoriteRevealAlpha(pane, now, item) > 0
}

func filePaneFavoriteMenuWidth(gtx layout.Context) int {
	width := gtx.Dp(unit.Dp(filePaneFavoriteMenuWidthDp))
	if width > gtx.Constraints.Max.X {
		width = gtx.Constraints.Max.X
	}
	if width < 1 {
		width = 1
	}
	return width
}

func filePaneFavoriteRectContains(rect image.Rectangle, pos image.Point) bool {
	return rect.Dx() > 0 && rect.Dy() > 0 &&
		pos.X >= rect.Min.X && pos.X < rect.Max.X &&
		pos.Y >= rect.Min.Y && pos.Y < rect.Max.Y
}

func filePaneFavoriteMenuTitleHeight(gtx layout.Context) int {
	titleH := gtx.Dp(unit.Dp(17))
	if titleH < 1 {
		titleH = 1
	}
	return titleH
}

func filePaneFavoriteMenuRowHeight(gtx layout.Context) int {
	rowH := gtx.Dp(unit.Dp(18))
	if rowH < 1 {
		rowH = 1
	}
	return rowH
}

func filePaneFavoriteMenuSeparatorHeight(gtx layout.Context) int {
	sepH := gtx.Dp(unit.Dp(5))
	if sepH < 3 {
		sepH = 3
	}
	return sepH
}

func filePaneFavoriteMenuCardSize(gtx layout.Context, items []fileFavoriteItem) image.Point {
	height := filePaneFavoriteMenuTitleHeight(gtx)
	for i, item := range items {
		if item.addCurrent && i > 0 {
			height += filePaneFavoriteMenuSeparatorHeight(gtx)
		}
		height += filePaneFavoriteMenuRowHeight(gtx)
	}
	return image.Pt(filePaneFavoriteMenuWidth(gtx), height)
}

func filePaneFavoriteRemoveButtonWidth(gtx layout.Context) int {
	size := gtx.Dp(unit.Dp(9))
	if size < 1 {
		size = 1
	}
	return size + gtx.Dp(unit.Dp(4))
}

func (ui *UI) filePaneFavoriteMenuBaseRect(gtx layout.Context, pane *filePaneState, items []fileFavoriteItem, slideY int) image.Rectangle {
	size := filePaneFavoriteMenuCardSize(gtx, items)
	anchor := image.Point{
		X: gtx.Constraints.Max.X - size.X,
		Y: pane.headerHeight + gtx.Dp(unit.Dp(4)) + slideY,
	}
	anchor = clampFilePaneMenuPoint(anchor, size, gtx.Constraints.Max)
	return image.Rectangle{Min: anchor, Max: anchor.Add(size)}
}

func filePaneFavoriteMenuTextWidth(gtx layout.Context, item fileFavoriteItem) int {
	width := filePaneFavoriteMenuWidth(gtx) - gtx.Dp(unit.Dp(7)) - gtx.Dp(unit.Dp(6))
	if item.removable && !item.disabled {
		width -= gtx.Dp(unit.Dp(2)) + filePaneFavoriteRemoveButtonWidth(gtx)
	}
	if width < 0 {
		width = 0
	}
	return width
}

func (ui *UI) favoriteMenuLabelMetrics(th *material.Theme, gtx layout.Context, item fileFavoriteItem) (fullWidth, trimmedWidth, hiddenPrefixWidth, ellipsisWidth int) {
	lbl := material.Body2(th, item.label)
	lbl.Font.Typeface = ui.mainTypeface()
	lbl.TextSize = ui.functionBarTextSize()
	if item.active || item.addCurrent {
		lbl.Font.Weight = font.Medium
	}
	lbl.MaxLines = 1
	fullWidth = measureLabelUnconstrained(gtx, lbl).Size.X

	textWidth := filePaneFavoriteMenuTextWidth(gtx, item)
	if textWidth <= 0 {
		return fullWidth, fullWidth, 0, 0
	}

	trimGtx := gtx
	trimGtx.Constraints.Min = image.Point{}
	trimGtx.Constraints.Max = image.Pt(textWidth, gtx.Constraints.Max.Y)
	trimLbl := lbl
	trimLbl.Truncator = "\u200b"
	trimLbl.Text = trimLeftLabelToFit(trimGtx, trimLbl, item.label)
	trimmedWidth = measureLabelUnconstrained(gtx, trimLbl).Size.X
	if trimmedWidth <= 0 {
		trimmedWidth = fullWidth
	}
	trimmedRunes := []rune(trimLbl.Text)
	suffixStart := 0
	if len(trimmedRunes) >= 2 && trimmedRunes[0] == '.' && trimmedRunes[1] == '.' {
		ellipsisLbl := lbl
		ellipsisLbl.Text = ".."
		ellipsisWidth = measureLabelUnconstrained(gtx, ellipsisLbl).Size.X
		suffixLen := len(trimmedRunes) - 2
		fullRunes := []rune(item.label)
		if suffixLen > 0 && suffixLen <= len(fullRunes) {
			suffixStart = len(fullRunes) - suffixLen
		}
	}
	if suffixStart > 0 {
		prefixLbl := lbl
		prefixLbl.Text = string([]rune(item.label)[:suffixStart])
		hiddenPrefixWidth = measureLabelUnconstrained(gtx, prefixLbl).Size.X
	}
	return fullWidth, trimmedWidth, hiddenPrefixWidth, ellipsisWidth
}

func filePaneFavoriteMenuItemOffsetY(gtx layout.Context, items []fileFavoriteItem, target int) int {
	y := filePaneFavoriteMenuTitleHeight(gtx)
	for i, item := range items {
		if i >= target {
			break
		}
		if item.addCurrent && i > 0 {
			y += filePaneFavoriteMenuSeparatorHeight(gtx)
		}
		y += filePaneFavoriteMenuRowHeight(gtx)
	}
	if target >= 0 && target < len(items) && items[target].addCurrent && target > 0 {
		y += filePaneFavoriteMenuSeparatorHeight(gtx)
	}
	return y
}

func filePaneFavoriteMenuItemStyle(theme filePanePopupTheme, item fileFavoriteItem, hoverFill, alpha float32) (bg, fg color.NRGBA, weight font.Weight, hoverT float32) {
	bg = color.NRGBA{}
	baseFg := theme.Text
	weight = font.Normal
	if item.disabled {
		baseFg = theme.DisabledText
	}
	if item.addCurrent {
		baseFg = mixNRGBA(theme.Text, theme.HoverText, 0.28)
		weight = font.Medium
	}
	fg = scaleColorAlpha(baseFg, alpha)
	hoverT = smoothstep01(clamp01(hoverFill))
	if item.active {
		bg = scaleColorAlpha(mixNRGBA(theme.Bg, theme.ActiveBg, 0.68), alpha)
		fg = scaleColorAlpha(theme.ActiveText, alpha)
		weight = font.Medium
	}
	if hoverT > 0 && !item.disabled {
		if item.active {
			hoverBg := mixNRGBA(theme.ActiveBg, theme.HoverBg, 0.22)
			hoverBg = mixNRGBA(hoverBg, theme.HoverText, 0.05*hoverT)
			bg = scaleColorAlpha(hoverBg, alpha)
			fg = scaleColorAlpha(bestContrastColor(hoverBg, theme.ActiveText, theme.HoverText, theme.Text), alpha)
		} else {
			hoverBg := mixNRGBA(theme.ActiveBg, theme.HoverBg, 0.18)
			hoverBg = mixNRGBA(hoverBg, theme.HoverText, 0.06*hoverT)
			bg = scaleColorAlpha(hoverBg, alpha)
			fg = scaleColorAlpha(bestContrastColor(hoverBg, theme.HoverText, theme.ActiveText, baseFg), alpha)
			weight = font.Medium
		}
	}
	return bg, fg, weight, hoverT
}

func (ui *UI) favoriteMenuRevealWidth(th *material.Theme, gtx layout.Context, item fileFavoriteItem) int {
	_, _, hiddenPrefixWidth, ellipsisWidth := ui.favoriteMenuLabelMetrics(th, gtx, item)
	width := filePaneFavoriteMenuWidth(gtx)
	if extraWidth := hiddenPrefixWidth - ellipsisWidth; extraWidth > 0 {
		width += extraWidth
	}
	if item.removable {
		// The base menu width already includes the remove button area.
	}
	return width
}

func (ui *UI) layoutFilePaneFavoriteMenuItemContent(th *material.Theme, gtx layout.Context, theme filePanePopupTheme, click *widget.Clickable, removeClick *widget.Clickable, item fileFavoriteItem, fg color.NRGBA, weight font.Weight, alpha float32, fullLabel, interactive bool) layout.Dimensions {
	label := item.label
	renderLabel := func(gtx layout.Context) layout.Dimensions {
		lbl := material.Body2(th, label)
		lbl.Font.Typeface = ui.mainTypeface()
		lbl.TextSize = ui.functionBarTextSize()
		lbl.Font.Weight = weight
		lbl.Color = fg
		lbl.MaxLines = 1
		if !item.addCurrent && !fullLabel {
			// Suppress right-side ellipsis; we want left-side-only trimming for paths.
			lbl.Truncator = "\u200b"
			lbl.Text = trimLeftLabelToFit(gtx, lbl, label)
		}
		return layoutVCenteredLabel(gtx, lbl)
	}

	return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(6), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 3)
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if item.disabled || click == nil {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return renderLabel(gtx)
			}
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if interactive {
					pointer.CursorPointer.Add(gtx.Ops)
				}
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return renderLabel(gtx)
			})
		}))
		if item.removable && !item.disabled {
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout))
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if interactive && removeClick != nil {
					return layoutFilePaneFavoriteRemoveButton(th, gtx, theme, removeClick, alpha)
				}
				return layoutFilePaneFavoriteRemoveButtonVisual(gtx, theme, alpha, false)
			}))
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func (ui *UI) layoutFilePaneFavoriteReveal(th *material.Theme, gtx layout.Context, pane *filePaneState, menuRect image.Rectangle, items []fileFavoriteItem, alpha float32) image.Rectangle {
	if pane == nil {
		return image.Rectangle{}
	}
	revealIndex, revealItem, ok := filePaneFavoriteRevealedItem(pane, gtx.Now, items)
	if !ok {
		return image.Rectangle{}
	}
	rect := ui.favoriteMenuRevealRect(th, gtx, pane, menuRect, items)
	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		return image.Rectangle{}
	}
	theme := ui.filePanePopupTheme()
	hoverFill, _ := pane.favoriteMenuHoverAnim.hoverFill(gtx.Now, filePaneFavoriteMenuItemID(revealItem))
	revealAlpha := filePaneFavoriteRevealAlpha(pane, gtx.Now, revealItem)
	if revealAlpha <= 0 {
		return image.Rectangle{}
	}
	bg, fg, weight, hoverT := filePaneFavoriteMenuItemStyle(theme, revealItem, hoverFill, alpha*revealAlpha)
	bg = scaleColorAlpha(bg, 0.92)
	border := scaleColorAlpha(mixNRGBA(theme.Border, fg, 0.26+hoverT*0.18), alpha*revealAlpha*0.88)
	radius := gtx.Dp(unit.Dp(filePaneOverlayCornerDp))
	var removeClick *widget.Clickable
	if revealItem.removable && revealIndex >= 0 && revealIndex < len(pane.favoriteRemoveClicks) {
		removeClick = &pane.favoriteRemoveClicks[revealIndex]
	}

	offset := op.Offset(rect.Min).Push(gtx.Ops)
	size := rect.Size()
	rr := paneRRect(size, radius, true, false)
	paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())
	maskPaneBorderEdges(gtx, size, bg, true, false, 1)

	contentGtx := gtx
	contentGtx.Constraints = layout.Exact(size)
	clipStack := rr.Push(gtx.Ops)
	ui.layoutFilePaneFavoriteMenuItemContent(th, contentGtx, theme, nil, removeClick, revealItem, fg, weight, alpha*revealAlpha, true, false)
	clipStack.Pop()
	offset.Pop()

	return rect
}

func (ui *UI) layoutFilePaneFavoriteMenuCard(th *material.Theme, gtx layout.Context, pane *filePaneState, items []fileFavoriteItem, alpha float32) layout.Dimensions {
	if pane == nil {
		return layout.Dimensions{}
	}
	width := filePaneFavoriteMenuWidth(gtx)
	theme := ui.filePanePopupTheme()
	hoverID := filePaneFavoriteMenuHoveredID(pane, items)
	if hoverID != pane.favoriteMenuHoverID {
		pane.favoriteMenuHoverID = hoverID
		pane.favoriteMenuHoverAnim.setHover(hoverID, gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}

	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return pane.favoriteMenuClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return fillRoundedClipBox(
				gtx,
				gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
				scaleColorAlpha(theme.Bg, alpha),
				scaleColorAlpha(theme.Border, alpha),
				func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0, len(items)+3)
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						titleH := filePaneFavoriteMenuTitleHeight(gtx)
						return fixedHeight(gtx, titleH, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(3), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Caption(th, "Favorites")
								lbl.Font.Typeface = ui.mainTypeface()
								lbl.TextSize = scaleConfigFontSize(ui.fmCfg, 9)
								lbl.Color = scaleColorAlpha(theme.Title, alpha)
								lbl.MaxLines = 1
								lbl.Font.Weight = font.Medium
								lbl.Truncator = "…"
								return layoutVCenteredLabel(gtx, lbl)
							})
						})
					}))
					for i, item := range items {
						i := i
						item := item
						if item.addCurrent && i > 0 {
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								sepH := filePaneFavoriteMenuSeparatorHeight(gtx)
								return fixedHeight(gtx, sepH, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										h := gtx.Dp(unit.Dp(1))
										if h < 1 {
											h = 1
										}
										return fillBgExact(gtx, scaleColorAlpha(theme.Divider, alpha), func(gtx layout.Context) layout.Dimensions {
											return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
										})
									})
								})
							}))
						}
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							hoverFill, animating := pane.favoriteMenuHoverAnim.hoverFill(gtx.Now, filePaneFavoriteMenuItemID(item))
							if animating {
								gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
							}
							revealAlpha := filePaneFavoriteRevealAlpha(pane, gtx.Now, item)
							return ui.layoutFilePaneFavoriteMenuItem(th, gtx, theme, &pane.favoriteOptionClicks[i], &pane.favoriteRemoveClicks[i], item, hoverFill, alpha, revealAlpha)
						}))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				},
			)
		})
	})
}

func (ui *UI) layoutFilePaneFavoriteMenuItem(th *material.Theme, gtx layout.Context, theme filePanePopupTheme, click *widget.Clickable, removeClick *widget.Clickable, item fileFavoriteItem, hoverFill, alpha, _ float32) layout.Dimensions {
	rowH := filePaneFavoriteMenuRowHeight(gtx)
	bg, fg, weight, _ := filePaneFavoriteMenuItemStyle(theme, item, hoverFill, alpha)
	return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
		return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePaneFavoriteMenuItemContent(th, gtx, theme, click, removeClick, item, fg, weight, alpha, false, true)
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

func trimLeftLabelToFit(gtx layout.Context, lbl material.LabelStyle, text string) string {
	if text == "" {
		return text
	}
	maxPx := gtx.Constraints.Max.X
	if maxPx <= 0 {
		return text
	}
	lbl.Text = text
	if measureLabelUnconstrained(gtx, lbl).Size.X <= maxPx {
		return text
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}
	best := trimLeftRunes(text, 1)
	lo, hi := 1, len(runes)
	for lo <= hi {
		mid := (lo + hi) / 2
		candidate := trimLeftRunes(text, mid)
		lbl.Text = candidate
		if measureLabelUnconstrained(gtx, lbl).Size.X <= maxPx {
			best = candidate
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}

func layoutFilePaneFavoriteRemoveButtonVisual(gtx layout.Context, theme filePanePopupTheme, alpha float32, hovered bool) layout.Dimensions {
	bg := scaleColorAlpha(theme.ButtonBg, alpha)
	border := scaleColorAlpha(theme.ButtonBorder, alpha)
	iconBase := bestContrastColor(theme.ButtonBg, theme.Text, theme.HoverText, theme.ActiveText)
	iconColor := scaleColorAlpha(mixNRGBA(iconBase, theme.ButtonBg, 0.18), alpha)
	if hovered {
		bg = scaleColorAlpha(mixNRGBA(theme.ButtonBg, theme.HoverBg, 0.7), alpha)
		border = scaleColorAlpha(mixNRGBA(theme.ButtonBorder, theme.HoverText, 0.3), alpha)
		iconColor = scaleColorAlpha(theme.HoverText, alpha)
	}
	return fillRoundedBox(gtx, gtx.Dp(unit.Dp(3)), bg, border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(2), Right: unit.Dp(2), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := gtx.Dp(unit.Dp(9))
			if size < 1 {
				size = 1
			}
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if ic := uitheme.CloseIcon(); ic != nil {
					iconGtx := gtx
					iconGtx.Constraints = layout.Exact(image.Pt(size, size))
					ic.Layout(iconGtx, iconColor)
				}
				return layout.Dimensions{Size: image.Pt(size, size)}
			})
		})
	})
}

func layoutFilePaneFavoriteRemoveButton(_ *material.Theme, gtx layout.Context, theme filePanePopupTheme, c *widget.Clickable, alpha float32) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layoutFilePaneFavoriteRemoveButtonVisual(gtx, theme, alpha, c.Hovered())
	})
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

func fillFilePaneBox(gtx layout.Context, bg color.NRGBA, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		call.Add(gtx.Ops)
		return dims
	}

	defer clip.Rect(image.Rect(0, 0, dims.Size.X, dims.Size.Y)).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rect(0, 0, dims.Size.X, dims.Size.Y)).Op())

	call.Add(gtx.Ops)
	return dims
}

func layoutFilePaneChrome(gtx layout.Context, active bool, accent, shade color.NRGBA, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	call.Add(gtx.Ops)
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}

	defer clip.Rect(image.Rect(0, 0, dims.Size.X, dims.Size.Y)).Push(gtx.Ops).Pop()
	if !active {
		if shade.A != 0 {
			paint.FillShape(gtx.Ops, shade, clip.Rect(image.Rect(0, 0, dims.Size.X, dims.Size.Y)).Op())
		}
		return dims
	}

	lineH := 1
	paint.FillShape(gtx.Ops, accent, clip.Rect(image.Rect(0, 0, dims.Size.X, lineH)).Op())
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
				return layoutVCenteredLabel(gtx, lbl)
			})
		})
	})
}

func layoutTinyIconModeButton(_ *material.Theme, gtx layout.Context, c *widget.Clickable, icon *widget.Icon, active bool) layout.Dimensions {
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
				return layoutVCenteredLabel(gtx, lbl)
			})
		})
	})
}
