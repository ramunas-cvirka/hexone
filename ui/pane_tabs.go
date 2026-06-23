// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"image"
	"image/color"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/font"
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
	tabStripHeightDp       = 22
	tabStripButtonWidthDp  = 22
	tabStripTitlePadDp     = 7
	tabStripCloseWidthDp   = 16
	tabStripMaxTabsPerPane = 64
)

type filePaneTabSet struct {
	tabs        []*filePaneState
	active      int
	scroll      int
	tabClicks   []widget.Clickable
	closeClicks []widget.Clickable
	prevClick   widget.Clickable
	nextClick   widget.Clickable
	addClick    widget.Clickable
}

type terminalTabSet struct {
	sessions    []*terminalSession
	active      int
	scroll      int
	tabClicks   []widget.Clickable
	closeClicks []widget.Clickable
	prevClick   widget.Clickable
	nextClick   widget.Clickable
	addClick    widget.Clickable
}

type appTabItem struct {
	title  string
	active bool
}

type appTabStripActions struct {
	selectIdx int
	closeIdx  int
	add       bool
}

func (ui *UI) installFilePaneHandlers(idx int, pane *filePaneState) {
	if ui == nil || pane == nil {
		return
	}
	pane.table.OnClick = func(row int) {
		_ = row
		ui.setActiveFilePane(idx)
	}
	pane.table.OnDoubleClick = func(row int) {
		ui.queueFilePaneSystemOpen(idx, row)
	}
	pane.table.OnActivate = func(row int) {
		ui.queueFilePaneOpen(idx, row)
	}
}

func (ui *UI) ensureFilePaneTabs() {
	if ui == nil || len(ui.filePanes) == 0 {
		return
	}
	if len(ui.filePaneTabs) != len(ui.filePanes) {
		sets := make([]filePaneTabSet, len(ui.filePanes))
		for i, pane := range ui.filePanes {
			if pane == nil {
				continue
			}
			sets[i].tabs = []*filePaneState{pane}
			sets[i].active = 0
			ui.installFilePaneHandlers(i, pane)
		}
		ui.filePaneTabs = sets
		return
	}
	for i := range ui.filePaneTabs {
		set := &ui.filePaneTabs[i]
		if len(set.tabs) == 0 {
			if ui.filePanes[i] != nil {
				set.tabs = []*filePaneState{ui.filePanes[i]}
				set.active = 0
				ui.installFilePaneHandlers(i, ui.filePanes[i])
			}
			continue
		}
		set.active = clampTabIndex(set.active, len(set.tabs))
		if ui.filePanes[i] != nil && set.tabs[set.active] != ui.filePanes[i] {
			found := -1
			for j, pane := range set.tabs {
				if pane == ui.filePanes[i] {
					found = j
					break
				}
			}
			if found >= 0 {
				set.active = found
			} else {
				set.tabs[set.active] = ui.filePanes[i]
				ui.installFilePaneHandlers(i, ui.filePanes[i])
			}
		}
		ui.filePanes[i] = set.tabs[set.active]
	}
}

func (ui *UI) allFilePaneTabPanes() []*filePaneState {
	if ui == nil {
		return nil
	}
	ui.ensureFilePaneTabs()
	out := make([]*filePaneState, 0, len(ui.filePanes))
	seen := make(map[*filePaneState]struct{})
	for i := range ui.filePaneTabs {
		for _, pane := range ui.filePaneTabs[i].tabs {
			if pane == nil {
				continue
			}
			if _, ok := seen[pane]; ok {
				continue
			}
			seen[pane] = struct{}{}
			out = append(out, pane)
		}
	}
	for _, pane := range ui.filePanes {
		if pane == nil {
			continue
		}
		if _, ok := seen[pane]; ok {
			continue
		}
		out = append(out, pane)
	}
	return out
}

func (ui *UI) activateFilePaneTab(paneIdx, tabIdx int) bool {
	if ui == nil {
		return false
	}
	ui.ensureFilePaneTabs()
	if paneIdx < 0 || paneIdx >= len(ui.filePaneTabs) {
		return false
	}
	set := &ui.filePaneTabs[paneIdx]
	if tabIdx < 0 || tabIdx >= len(set.tabs) || set.tabs[tabIdx] == nil {
		return false
	}
	if paneIdx < len(ui.filePanes) && ui.filePanes[paneIdx] != nil && ui.filePanes[paneIdx] != set.tabs[tabIdx] {
		closeFilePaneTabTransient(ui.filePanes[paneIdx])
	}
	set.active = tabIdx
	set.scroll = tabScrollToActive(set.scroll, set.active)
	ui.filePanes[paneIdx] = set.tabs[tabIdx]
	ui.setActiveFilePane(paneIdx)
	return true
}

func (ui *UI) addFilePaneTab(paneIdx int) bool {
	if ui == nil {
		return false
	}
	ui.ensureFilePaneTabs()
	if paneIdx < 0 || paneIdx >= len(ui.filePaneTabs) || paneIdx >= len(ui.filePanes) {
		return false
	}
	set := &ui.filePaneTabs[paneIdx]
	if len(set.tabs) >= tabStripMaxTabsPerPane {
		if pane := ui.filePanes[paneIdx]; pane != nil {
			pane.setNotice("tab limit reached", time.Now())
		}
		return false
	}
	base := ui.filePanes[paneIdx]
	dir := "."
	if base != nil {
		dir = strings.TrimSpace(base.dir)
		if base.loading && strings.TrimSpace(base.loadingDir) != "" {
			dir = strings.TrimSpace(base.loadingDir)
		}
	}
	if dir == "" {
		dir = "."
	}
	pane := newFilePaneState(dir, ui.fmCfg)
	if base != nil && base.remoteConnected() {
		pane.remote = base.remote.clone()
		pane.localDirBeforeRemote = base.localDirBeforeRemote
	}
	ui.installFilePaneHandlers(paneIdx, pane)
	set.tabs = append(set.tabs, pane)
	set.active = len(set.tabs) - 1
	set.scroll = tabScrollToActive(set.scroll, set.active)
	ui.filePanes[paneIdx] = pane
	ui.requestPaneLoadWithSelection(paneIdx, dir, "", "", 0)
	return true
}

func (ui *UI) closeFilePaneTab(paneIdx, tabIdx int) bool {
	if ui == nil {
		return false
	}
	ui.ensureFilePaneTabs()
	if paneIdx < 0 || paneIdx >= len(ui.filePaneTabs) || paneIdx >= len(ui.filePanes) {
		return false
	}
	set := &ui.filePaneTabs[paneIdx]
	if len(set.tabs) <= 1 || tabIdx < 0 || tabIdx >= len(set.tabs) {
		return false
	}
	closing := set.tabs[tabIdx]
	if closing != nil {
		closeFilePaneTabTransient(closing)
		if closing.remote != nil {
			closing.remote.close()
		}
	}
	set.tabs = append(set.tabs[:tabIdx], set.tabs[tabIdx+1:]...)
	if set.active >= tabIdx {
		set.active--
	}
	set.active = clampTabIndex(set.active, len(set.tabs))
	set.scroll = tabScrollToActive(set.scroll, set.active)
	ui.filePanes[paneIdx] = set.tabs[set.active]
	ui.setActiveFilePane(paneIdx)
	return true
}

func closeFilePaneTabTransient(pane *filePaneState) {
	if pane == nil {
		return
	}
	pane.stopPathEdit()
	pane.stopInlineNameEdit()
	pane.closeSortMenu()
	pane.closeDriveMenu()
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
}

func (ui *UI) layoutFilePaneTabStrip(th *material.Theme, gtx layout.Context, paneIdx int) layout.Dimensions {
	if ui == nil {
		return layout.Dimensions{}
	}
	ui.ensureFilePaneTabs()
	if paneIdx < 0 || paneIdx >= len(ui.filePaneTabs) {
		return layout.Dimensions{}
	}
	set := &ui.filePaneTabs[paneIdx]
	items := make([]appTabItem, 0, len(set.tabs))
	for i, pane := range set.tabs {
		items = append(items, appTabItem{
			title:  filePaneTabTitle(pane),
			active: i == set.active,
		})
	}
	actions, dims := ui.layoutAppTabStrip(th, gtx, items, &set.scroll, &set.tabClicks, &set.closeClicks, &set.prevClick, &set.nextClick, &set.addClick)
	if actions.selectIdx >= 0 {
		ui.activateFilePaneTab(paneIdx, actions.selectIdx)
		gtx.Execute(opInvalidate())
	}
	if actions.closeIdx >= 0 && ui.closeFilePaneTab(paneIdx, actions.closeIdx) {
		gtx.Execute(opInvalidate())
	}
	if actions.add && ui.addFilePaneTab(paneIdx) {
		gtx.Execute(opInvalidate())
	}
	return dims
}

func (ui *UI) ensureTerminalTabs() {
	if ui == nil {
		return
	}
	if len(ui.terminalTabs.sessions) == 0 {
		if ui.terminal == nil {
			ui.terminal = newTerminalSession(ui.invalidate, terminalConfiguredRows(ui.fmCfg))
		}
		ui.terminalTabs.sessions = []*terminalSession{ui.terminal}
		ui.terminalTabs.active = 0
		return
	}
	ui.terminalTabs.active = clampTabIndex(ui.terminalTabs.active, len(ui.terminalTabs.sessions))
	ui.terminal = ui.terminalTabs.sessions[ui.terminalTabs.active]
}

func (ui *UI) activateTerminalTab(tabIdx int) bool {
	if ui == nil {
		return false
	}
	ui.ensureTerminalTabs()
	if tabIdx < 0 || tabIdx >= len(ui.terminalTabs.sessions) {
		return false
	}
	wasActive := ui.terminal != nil && ui.terminal.active()
	if ui.terminal != nil {
		ui.terminal.setActive(false)
	}
	ui.terminalTabs.active = tabIdx
	ui.terminalTabs.scroll = tabScrollToActive(ui.terminalTabs.scroll, tabIdx)
	ui.terminal = ui.terminalTabs.sessions[tabIdx]
	ui.terminal.setActive(wasActive)
	if wasActive {
		ui.terminal.focusKeyboard()
	}
	return true
}

func (ui *UI) addTerminalTab() bool {
	if ui == nil {
		return false
	}
	ui.ensureTerminalTabs()
	if len(ui.terminalTabs.sessions) >= tabStripMaxTabsPerPane {
		if ui.terminal != nil {
			ui.terminal.setError("terminal tab limit reached")
		}
		return false
	}
	wasActive := ui.terminal == nil || ui.terminal.active()
	if ui.terminal != nil {
		ui.terminal.setActive(false)
	}
	st := newTerminalSession(ui.invalidate, terminalConfiguredRows(ui.fmCfg))
	st.setActive(wasActive)
	ui.terminalTabs.sessions = append(ui.terminalTabs.sessions, st)
	ui.terminalTabs.active = len(ui.terminalTabs.sessions) - 1
	ui.terminalTabs.scroll = tabScrollToActive(ui.terminalTabs.scroll, ui.terminalTabs.active)
	ui.terminal = st
	if wasActive {
		st.focusKeyboard()
	}
	return true
}

func (ui *UI) closeTerminalTab(tabIdx int) bool {
	if ui == nil {
		return false
	}
	ui.ensureTerminalTabs()
	set := &ui.terminalTabs
	if len(set.sessions) <= 1 || tabIdx < 0 || tabIdx >= len(set.sessions) {
		return false
	}
	wasActive := ui.terminal != nil && ui.terminal.active()
	if closing := set.sessions[tabIdx]; closing != nil {
		closing.Close()
	}
	set.sessions = append(set.sessions[:tabIdx], set.sessions[tabIdx+1:]...)
	if set.active >= tabIdx {
		set.active--
	}
	set.active = clampTabIndex(set.active, len(set.sessions))
	set.scroll = tabScrollToActive(set.scroll, set.active)
	ui.terminal = set.sessions[set.active]
	ui.terminal.setActive(wasActive)
	if wasActive {
		ui.terminal.focusKeyboard()
	}
	return true
}

func (ui *UI) closeAllTerminalTabs() {
	if ui == nil {
		return
	}
	seen := make(map[*terminalSession]struct{})
	for _, st := range ui.terminalTabs.sessions {
		if st == nil {
			continue
		}
		if _, ok := seen[st]; ok {
			continue
		}
		seen[st] = struct{}{}
		st.Close()
	}
	if ui.terminal != nil {
		if _, ok := seen[ui.terminal]; !ok {
			ui.terminal.Close()
		}
	}
}

func (ui *UI) layoutTerminalTabStrip(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if ui == nil {
		return layout.Dimensions{}
	}
	ui.ensureTerminalTabs()
	items := make([]appTabItem, 0, len(ui.terminalTabs.sessions))
	for i, st := range ui.terminalTabs.sessions {
		items = append(items, appTabItem{
			title:  terminalTabTitle(st),
			active: i == ui.terminalTabs.active,
		})
	}
	actions, dims := ui.layoutAppTabStrip(th, gtx, items, &ui.terminalTabs.scroll, &ui.terminalTabs.tabClicks, &ui.terminalTabs.closeClicks, &ui.terminalTabs.prevClick, &ui.terminalTabs.nextClick, &ui.terminalTabs.addClick)
	if actions.selectIdx >= 0 {
		ui.activateTerminalTab(actions.selectIdx)
		gtx.Execute(opInvalidate())
	}
	if actions.closeIdx >= 0 && ui.closeTerminalTab(actions.closeIdx) {
		gtx.Execute(opInvalidate())
	}
	if actions.add && ui.addTerminalTab() {
		gtx.Execute(opInvalidate())
	}
	return dims
}

func (ui *UI) layoutAppTabStrip(
	th *material.Theme,
	gtx layout.Context,
	items []appTabItem,
	scroll *int,
	tabClicks *[]widget.Clickable,
	closeClicks *[]widget.Clickable,
	prevClick, nextClick, addClick *widget.Clickable,
) (appTabStripActions, layout.Dimensions) {
	actions := appTabStripActions{selectIdx: -1, closeIdx: -1}
	if len(items) == 0 {
		return actions, layout.Dimensions{}
	}
	ensureClickableSlice(tabClicks, len(items))
	ensureClickableSlice(closeClicks, len(items))
	if addClick.Clicked(gtx) {
		actions.add = true
	}
	for i := range *tabClicks {
		if (*tabClicks)[i].Clicked(gtx) {
			actions.selectIdx = i
		}
	}
	for i := range *closeClicks {
		if (*closeClicks)[i].Clicked(gtx) {
			actions.closeIdx = i
		}
	}
	if actions.closeIdx >= 0 {
		actions.selectIdx = -1
	}
	widths := tabStripWidths(gtx, ui.fmCfg, items)
	available := gtx.Constraints.Max.X
	if available < 1 {
		available = 1
	}
	plan := tabStripPlan(widths, available, tabStripControlWidth(gtx), *scroll)
	*scroll = plan.start
	if prevClick.Clicked(gtx) && plan.overflow && *scroll > 0 {
		*scroll--
		plan = tabStripPlan(widths, available, tabStripControlWidth(gtx), *scroll)
	}
	if nextClick.Clicked(gtx) && plan.overflow && plan.end < len(items) {
		*scroll++
		plan = tabStripPlan(widths, available, tabStripControlWidth(gtx), *scroll)
	}

	return actions, fixedHeight(gtx, gtx.Dp(unit.Dp(tabStripHeightDp)), func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(items)+4)
		if plan.overflow {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutTabStripButton(th, gtx, prevClick, "<", *scroll > 0)
			}))
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutTabStripSeparator(gtx)
			}))
		}
		for i := plan.start; i < plan.end; i++ {
			idx := i
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, widths[idx], func(gtx layout.Context) layout.Dimensions {
					return ui.layoutTabStripTab(th, gtx, items[idx], &(*tabClicks)[idx], &(*closeClicks)[idx], idx, len(items) > 1)
				})
			}))
			if i < plan.end-1 || plan.overflow {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutTabStripSeparator(gtx)
				}))
			}
		}
		if plan.overflow {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutTabStripButton(th, gtx, nextClick, ">", plan.end < len(items))
			}))
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutTabStripSeparator(gtx)
			}))
		} else if len(items) > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutTabStripSeparator(gtx)
			}))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutTabStripButton(th, gtx, addClick, "+", true)
		}))
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func (ui *UI) layoutTabStripTab(th *material.Theme, gtx layout.Context, item appTabItem, click, closeClick *widget.Clickable, idx int, closable bool) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		bg, fg := ui.tabStripColors(item.active, click.Hovered(), idx)
		if bg.A != 0 {
			paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())
		}
		if item.active {
			h := gtx.Constraints.Max.Y
			if h < 1 {
				h = gtx.Dp(unit.Dp(tabStripHeightDp))
			}
			paint.FillShape(gtx.Ops, ui.tabStripAccentColor(idx), clip.Rect(image.Rect(0, h-1, gtx.Constraints.Max.X, h)).Op())
		}
		return layout.Inset{Left: unit.Dp(tabStripTitlePadDp), Right: unit.Dp(2), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, item.title)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.Font.Weight = font.Medium
					lbl.TextSize = scaleThemeFontSize(th, 10)
					lbl.Color = fg
					lbl.MaxLines = 1
					lbl.Truncator = ".."
					return layoutVCenteredLabel(gtx, lbl)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !closable {
						return layout.Spacer{Width: unit.Dp(2)}.Layout(gtx)
					}
					return fixedWidth(gtx, gtx.Dp(unit.Dp(tabStripCloseWidthDp)), func(gtx layout.Context) layout.Dimensions {
						return closeClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							pointer.CursorPointer.Add(gtx.Ops)
							ui.drawTabStripCloseSpot(gtx, closeClick.Hovered() || closeClick.Pressed())
							iconColor := ui.tabStripCloseColor(fg, closeClick.Hovered())
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								size := gtx.Dp(unit.Dp(9))
								if size < 7 {
									size = 7
								}
								drawTabCloseIcon(gtx, size, iconColor)
								return layout.Dimensions{Size: image.Pt(size, size)}
							})
						})
					})
				}),
			)
		})
	})
}

func (ui *UI) drawTabStripCloseSpot(gtx layout.Context, highlighted bool) {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	palette := filePanePaletteFromConfig(ui.fmCfg)
	bg := mixNRGBA(palette.PaneBg, color.NRGBA{R: 82, G: 88, B: 104, A: 255}, 0.52)
	bg.A = 54
	if highlighted {
		bg = mixNRGBA(palette.PaneBg, color.NRGBA{R: 136, G: 55, B: 72, A: 255}, 0.76)
		bg.A = 230
	}
	paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rectangle{Max: size}).Op())
	separator := ui.tabStripSeparatorColor()
	separator.A = 96
	paint.FillShape(gtx.Ops, separator, clip.Rect(image.Rect(0, size.Y/4, 1, size.Y-size.Y/4)).Op())
	if !highlighted {
		return
	}
	accent := color.NRGBA{R: 255, G: 126, B: 142, A: 232}
	paint.FillShape(gtx.Ops, accent, clip.Rect(image.Rect(0, size.Y-1, size.X, size.Y)).Op())
}

func (ui *UI) layoutTabStripButton(th *material.Theme, gtx layout.Context, click *widget.Clickable, label string, enabled bool) layout.Dimensions {
	return fixedWidth(gtx, tabStripControlWidth(gtx), func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if enabled {
				pointer.CursorPointer.Add(gtx.Ops)
			}
			bg, fg := ui.tabStripButtonColors(enabled, click.Hovered())
			if bg.A != 0 {
				paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())
			}
			lbl := material.Body2(th, label)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.Font.Weight = font.Bold
			lbl.TextSize = scaleThemeFontSize(th, 10)
			lbl.Color = fg
			lbl.MaxLines = 1
			return layout.Center.Layout(gtx, lbl.Layout)
		})
	})
}

func (ui *UI) layoutTabStripSeparator(gtx layout.Context) layout.Dimensions {
	w := tabStripSeparatorWidth(gtx)
	h := gtx.Constraints.Max.Y
	if h < 1 {
		h = gtx.Dp(unit.Dp(tabStripHeightDp))
	}
	lineH := h - gtx.Dp(unit.Dp(7))
	if lineH < h/2 {
		lineH = h / 2
	}
	y0 := (h - lineH) / 2
	paint.FillShape(gtx.Ops, ui.tabStripSeparatorColor(), clip.Rect(image.Rect(0, y0, w, y0+lineH)).Op())
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func drawTabCloseIcon(gtx layout.Context, size int, c color.NRGBA) {
	if size < 1 || c.A == 0 {
		return
	}
	pad := float32(size) * 0.24
	max := float32(size) - pad
	width := float32(gtx.Dp(unit.Dp(1)))
	if width < 1 {
		width = 1
	}
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(pad, pad))
	p.LineTo(f32.Pt(max, max))
	p.MoveTo(f32.Pt(max, pad))
	p.LineTo(f32.Pt(pad, max))
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: width}.Op())
}

func (ui *UI) tabStripColors(active, hovered bool, idx int) (bg, fg color.NRGBA) {
	palette := filePanePaletteFromConfig(ui.fmCfg)
	bg = color.NRGBA{}
	accent := ui.tabStripAccentColor(idx)
	if active {
		fg = bestContrastColor(palette.PaneBg, accent, palette.CurrentDirFg, palette.HoverFg, color.NRGBA{R: 248, G: 250, B: 255, A: 255})
		fg.A = 255
		return bg, fg
	}
	if hovered {
		bg = mixNRGBA(palette.PaneBg, palette.HoverBg, 0.7)
		bg.A = 150
		fg = bestContrastColor(bg, palette.HoverFg, palette.CurrentDirFg, accent, color.NRGBA{R: 232, G: 236, B: 244, A: 255})
		fg.A = 245
		return bg, fg
	}
	if c, ok := tabColorFromConfig(ui.fmCfg, true, idx); ok {
		base := tabStripInactiveColor()
		c = mixNRGBA(base, c, 0.34)
		c.A = 206
		return bg, c
	}
	return bg, tabStripInactiveColor()
}

func (ui *UI) tabStripButtonColors(enabled, hovered bool) (bg, fg color.NRGBA) {
	palette := filePanePaletteFromConfig(ui.fmCfg)
	bg = color.NRGBA{}
	fg = tabStripInactiveColor()
	if enabled {
		fg = mixNRGBA(fg, ui.tabStripAccentColor(0), 0.25)
		fg.A = 184
	}
	if enabled && hovered {
		bg = mixNRGBA(palette.PaneBg, palette.HoverBg, 0.64)
		bg.A = 116
		fg = ui.tabStripAccentColor(0)
	}
	return bg, fg
}

func (ui *UI) tabStripCloseColor(base color.NRGBA, hovered bool) color.NRGBA {
	if !hovered {
		out := base
		if out.A < 188 {
			out.A = 188
		}
		return out
	}
	palette := filePanePaletteFromConfig(ui.fmCfg)
	return bestContrastColor(palette.PaneBg, color.NRGBA{R: 255, G: 154, B: 154, A: 255}, ui.tabStripAccentColor(0), palette.HoverFg)
}

func (ui *UI) tabStripAccentColor(idx int) color.NRGBA {
	palette := filePanePaletteFromConfig(ui.fmCfg)
	if c, ok := tabActiveColorFromConfig(ui.fmCfg); ok {
		return c
	}
	return bestContrastColor(palette.PaneBg, palette.CurrentDirFg, palette.HoverFg, color.NRGBA{R: 245, G: 247, B: 255, A: 255})
}

func (ui *UI) tabStripSeparatorColor() color.NRGBA {
	palette := filePanePaletteFromConfig(ui.fmCfg)
	c := mixNRGBA(palette.PaneFg, palette.PaneBg, 0.62)
	c.A = 72
	return c
}

func tabStripInactiveColor() color.NRGBA {
	return color.NRGBA{R: 176, G: 182, B: 194, A: 206}
}

func tabColorFromConfig(cfg *fm.Config, alternating bool, idx int) (color.NRGBA, bool) {
	if cfg == nil {
		return color.NRGBA{}, false
	}
	primary := cfg.Tabs.Color
	secondary := cfg.Tabs.AltColor
	if alternating && primary == "" && secondary == "" {
		primary = "#7E344D"
		secondary = "#58408E"
	}
	if alternating && idx%2 == 1 && secondary != "" {
		return parseConfigColorHexFallback(secondary, secondary), true
	}
	if primary != "" {
		return parseConfigColorHexFallback(primary, primary), true
	}
	return color.NRGBA{}, false
}

func tabActiveColorFromConfig(cfg *fm.Config) (color.NRGBA, bool) {
	if cfg == nil || cfg.Tabs.ActiveColor == "" {
		return color.NRGBA{}, false
	}
	return parseConfigColorHexFallback(cfg.Tabs.ActiveColor, cfg.Tabs.ActiveColor), true
}

type tabPlan struct {
	start    int
	end      int
	overflow bool
}

func tabStripPlan(widths []int, available, controlW, scroll int) tabPlan {
	if len(widths) == 0 {
		return tabPlan{}
	}
	total := 0
	gap := 1
	for i, w := range widths {
		total += w
		if i > 0 {
			total += gap
		}
	}
	addW := controlW + gap
	if total+addW <= available {
		return tabPlan{start: 0, end: len(widths)}
	}
	tabSpace := available - addW - 2*controlW - 3*gap
	if tabSpace < widths[0] {
		tabSpace = widths[0]
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll >= len(widths) {
		scroll = len(widths) - 1
	}
	used := 0
	end := scroll
	for end < len(widths) {
		next := widths[end]
		if end > scroll {
			next += gap
		}
		if end > scroll && used+next > tabSpace {
			break
		}
		used += next
		end++
	}
	if end <= scroll {
		end = scroll + 1
	}
	return tabPlan{start: scroll, end: end, overflow: true}
}

func tabStripWidths(gtx layout.Context, cfg *fm.Config, items []appTabItem) []int {
	out := make([]int, len(items))
	minW := gtx.Dp(unit.Dp(72))
	fixedW := gtx.Dp(unit.Dp(118))
	maxW := gtx.Dp(unit.Dp(168))
	mode := "variable"
	if cfg != nil {
		minW = gtx.Dp(unit.Dp(cfg.Tabs.MinWidthDp))
		fixedW = gtx.Dp(unit.Dp(cfg.Tabs.FixedWidthDp))
		maxW = gtx.Dp(unit.Dp(cfg.Tabs.MaxWidthDp))
		mode = cfg.Tabs.WidthMode
	}
	if minW < 44 {
		minW = 44
	}
	if maxW < minW {
		maxW = minW
	}
	if fixedW < minW {
		fixedW = minW
	}
	if fixedW > maxW {
		fixedW = maxW
	}
	for i, item := range items {
		w := fixedW
		if mode != "fixed" {
			charW := gtx.Dp(unit.Dp(7))
			w = gtx.Dp(unit.Dp(tabStripTitlePadDp*2+tabStripCloseWidthDp+8)) + utf8.RuneCountInString(item.title)*charW
			if w < minW {
				w = minW
			}
			if w > maxW {
				w = maxW
			}
		}
		out[i] = w
	}
	return out
}

func tabStripControlWidth(gtx layout.Context) int {
	w := gtx.Dp(unit.Dp(tabStripButtonWidthDp))
	if w < 18 {
		w = 18
	}
	return w
}

func tabStripSeparatorWidth(gtx layout.Context) int {
	w := gtx.Dp(unit.Dp(1))
	if w < 1 {
		w = 1
	}
	return w
}

func ensureClickableSlice(dst *[]widget.Clickable, n int) {
	if dst == nil {
		return
	}
	if len(*dst) >= n {
		return
	}
	*dst = append(*dst, make([]widget.Clickable, n-len(*dst))...)
}

func clampTabIndex(idx, n int) int {
	if n <= 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= n {
		return n - 1
	}
	return idx
}

func tabScrollToActive(scroll, active int) int {
	if active < 0 {
		return 0
	}
	return active
}

func filePaneTabTitle(pane *filePaneState) string {
	if pane == nil {
		return "tab"
	}
	dir := strings.TrimSpace(pane.displayDir())
	if dir == "" {
		dir = strings.TrimSpace(pane.dir)
	}
	if dir == "" {
		return "tab"
	}
	if pane.remoteConnected() {
		clean := path.Clean(dir)
		base := path.Base(clean)
		if base == "." || base == "/" {
			base = clean
		}
		if pane.remote != nil {
			if prefix := strings.TrimSpace(pane.remote.displayPrefix()); prefix != "" {
				return prefix + ":" + base
			}
		}
		return "ssh:" + base
	}
	clean := filepath.Clean(dir)
	base := filepath.Base(clean)
	if base == "." || base == string(filepath.Separator) {
		return clean
	}
	return base
}

func terminalTabTitle(st *terminalSession) string {
	if st == nil {
		return "terminal"
	}
	if dir, ok := st.currentDir(); ok && strings.TrimSpace(dir) != "" {
		return terminalDirTitle(dir)
	}
	if strings.TrimSpace(st.startDir) != "" {
		return terminalDirTitle(st.startDir)
	}
	return "terminal"
}

func terminalDirTitle(dir string) string {
	clean := filepath.Clean(strings.TrimSpace(dir))
	base := filepath.Base(clean)
	if base == "." || base == string(filepath.Separator) {
		return clean
	}
	return base
}

func opInvalidate() op.InvalidateCmd {
	return op.InvalidateCmd{}
}
