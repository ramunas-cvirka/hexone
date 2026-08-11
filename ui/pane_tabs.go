// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	resources "hexone"
	"hexone/fm"
	uitheme "hexone/ui/theme"
	"image"
	"image/color"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
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
	tabStripHeightDp       = 22
	tabStripButtonWidthDp  = 22
	tabStripTitlePadDp     = 7
	tabStripCloseWidthDp   = 16
	tabStripMaxTabsPerPane = 64
	remoteIndicatorWidthDp = 11
)

func (ui *UI) tabStripHeight(gtx layout.Context) int {
	height := gtx.Dp(unit.Dp(tabStripHeightDp))
	textHeight := gtx.Sp(ui.tabStripTextSize()) + gtx.Dp(unit.Dp(8))
	if textHeight > height {
		height = textHeight
	}
	return height
}

type appTabStripGeometry struct {
	activeMinX    int
	activeMaxX    int
	activeVisible bool
}

type appTabStripStyle struct {
	open             bool
	activeBackground color.NRGBA
	keyboardFocus    color.NRGBA
}

type filePaneTabSet struct {
	tabs        []*filePaneState
	active      int
	scroll      int
	tabClicks   []widget.Clickable
	closeClicks []widget.Clickable
	remoteHover []*remoteIndicatorHover
	prevClick   widget.Clickable
	nextClick   widget.Clickable
	addClick    widget.Clickable
	geometry    appTabStripGeometry
}

type terminalTabSet struct {
	sessions     []*terminalSession
	active       int
	scroll       int
	tabClicks    []widget.Clickable
	closeClicks  []widget.Clickable
	remoteHover  []*remoteIndicatorHover
	prevClick    widget.Clickable
	nextClick    widget.Clickable
	addClick     widget.Clickable
	snippetClick widget.Clickable
	maxClick     widget.Clickable
	geometry     appTabStripGeometry
}

type appTabItem struct {
	title           string
	active          bool
	keyboardFocused bool
	remoteKey       string
	remoteTip       string
	remote          *remoteIndicatorHover
}

type remoteIndicatorHover struct {
	identity string
	hovered  bool
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
	set.scroll = clampTabScrollAnchor(set.scroll, len(set.tabs))
	ui.filePanes[paneIdx] = set.tabs[tabIdx]
	ui.setActiveFilePane(paneIdx)
	return true
}

func (ui *UI) stepFilePaneTab(paneIdx, step int) bool {
	if ui == nil {
		return false
	}
	ui.ensureFilePaneTabs()
	if paneIdx < 0 || paneIdx >= len(ui.filePaneTabs) {
		return false
	}
	set := &ui.filePaneTabs[paneIdx]
	if len(set.tabs) < 2 {
		return false
	}
	next := wrappedTabIndex(set.active, step, len(set.tabs))
	if !ui.activateFilePaneTab(paneIdx, next) {
		return false
	}
	set.scroll = tabScrollToActive(set.scroll, set.active)
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
	set.scroll = tabScrollAfterClose(set.scroll, tabIdx, len(set.tabs))
	ui.filePanes[paneIdx] = set.tabs[set.active]
	ui.setActiveFilePane(paneIdx)
	return true
}

func (ui *UI) disconnectCurrentFilePaneTab(paneIdx int, now time.Time) bool {
	if ui == nil {
		return false
	}
	ui.ensureFilePaneTabs()
	if paneIdx < 0 || paneIdx >= len(ui.filePaneTabs) || paneIdx >= len(ui.filePanes) {
		return false
	}
	set := &ui.filePaneTabs[paneIdx]
	if len(set.tabs) == 0 {
		return false
	}
	tabIdx := clampTabIndex(set.active, len(set.tabs))
	pane := set.tabs[tabIdx]
	if pane == nil || !pane.remoteConnected() {
		return false
	}
	if len(set.tabs) > 1 {
		return ui.closeFilePaneTab(paneIdx, tabIdx)
	}
	ui.disconnectPaneSSH(paneIdx, now)
	return !pane.remoteConnected()
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
	ensureRemoteIndicatorHovers(&set.remoteHover, len(set.tabs))
	items := make([]appTabItem, 0, len(set.tabs))
	for i, pane := range set.tabs {
		item := filePaneTabItem(pane)
		item.active = i == set.active
		item.remote = prepareRemoteIndicatorHover(set.remoteHover[i], item.remoteKey)
		items = append(items, item)
	}
	style := appTabStripStyle{
		open:             true,
		activeBackground: filePanePaletteFromConfig(ui.fmCfg).CurrentDirBg,
	}
	actions, dims, geometry := ui.layoutAppTabStrip(th, gtx, items, &set.scroll, &set.tabClicks, &set.closeClicks, &set.prevClick, &set.nextClick, &set.addClick, style)
	set.geometry = geometry
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
	ui.terminalTabs.scroll = clampTabScrollAnchor(ui.terminalTabs.scroll, len(ui.terminalTabs.sessions))
	ui.terminal = ui.terminalTabs.sessions[tabIdx]
	ui.terminal.setActive(wasActive)
	if wasActive {
		ui.terminal.focusKeyboard()
	}
	return true
}

func (ui *UI) stepTerminalTab(step int) bool {
	if ui == nil {
		return false
	}
	ui.ensureTerminalTabs()
	if len(ui.terminalTabs.sessions) < 2 {
		return false
	}
	next := wrappedTabIndex(ui.terminalTabs.active, step, len(ui.terminalTabs.sessions))
	if !ui.activateTerminalTab(next) {
		return false
	}
	ui.terminalTabs.scroll = tabScrollToActive(ui.terminalTabs.scroll, ui.terminalTabs.active)
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
	set.scroll = tabScrollAfterClose(set.scroll, tabIdx, len(set.sessions))
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
	ensureRemoteIndicatorHovers(&ui.terminalTabs.remoteHover, len(ui.terminalTabs.sessions))
	items := make([]appTabItem, 0, len(ui.terminalTabs.sessions))
	for i, st := range ui.terminalTabs.sessions {
		item := terminalTabItem(st, ui.fmCfg)
		item.active = i == ui.terminalTabs.active
		item.remote = prepareRemoteIndicatorHover(ui.terminalTabs.remoteHover[i], item.remoteKey)
		items = append(items, item)
	}
	for ui.terminalTabs.maxClick.Clicked(gtx) {
		if ui.toggleTerminalMaximized() {
			gtx.Execute(opInvalidate())
		}
	}
	for ui.terminalTabs.snippetClick.Clicked(gtx) {
		ui.toggleTerminalSnippetMenu(gtx.Now)
		gtx.Execute(opInvalidate())
	}
	icon := uitheme.FullscreenIcon()
	if ui.terminalMaximized() {
		icon = uitheme.FullscreenExitIcon()
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			style := appTabStripStyle{open: true, activeBackground: terminalBG}
			actions, dims, geometry := ui.layoutAppTabStrip(th, gtx, items, &ui.terminalTabs.scroll, &ui.terminalTabs.tabClicks, &ui.terminalTabs.closeClicks, &ui.terminalTabs.prevClick, &ui.terminalTabs.nextClick, &ui.terminalTabs.addClick, style)
			ui.terminalTabs.geometry = geometry
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
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutTabStripSeparatorStyle(gtx, true)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutTabStripButton(th, gtx, &ui.terminalTabs.snippetClick, uitheme.FavoriteIcon(ui.terminalSnippetMenuOpen), true)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutTabStripSeparatorStyle(gtx, true)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutTabStripButton(th, gtx, &ui.terminalTabs.maxClick, icon, true)
		}),
	)
}

func (ui *UI) layoutAppTabStrip(
	th *material.Theme,
	gtx layout.Context,
	items []appTabItem,
	scroll *int,
	tabClicks *[]widget.Clickable,
	closeClicks *[]widget.Clickable,
	prevClick, nextClick, addClick *widget.Clickable,
	style appTabStripStyle,
) (appTabStripActions, layout.Dimensions, appTabStripGeometry) {
	actions := appTabStripActions{selectIdx: -1, closeIdx: -1}
	geometry := appTabStripGeometry{}
	if len(items) == 0 {
		return actions, layout.Dimensions{}, geometry
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
	widths := ui.tabStripWidths(th, gtx, ui.fmCfg, items)
	minWidths := tabStripMinWidths(gtx, ui.fmCfg, len(items))
	available := gtx.Constraints.Max.X
	if available < 1 {
		available = 1
	}
	controlW := tabStripControlWidth(gtx)
	plan := tabStripPlanWithMin(widths, minWidths, available, controlW, *scroll)
	*scroll = clampTabScrollAnchor(*scroll, len(items))
	if !plan.overflow {
		*scroll = 0
	}
	if prevClick.Clicked(gtx) && plan.overflow && plan.start > 0 {
		*scroll = tabStripPrevScrollAnchor(plan)
		plan = tabStripPlanWithMin(widths, minWidths, available, controlW, *scroll)
	}
	if nextClick.Clicked(gtx) && plan.overflow && plan.end < len(items) {
		*scroll = tabStripNextScrollAnchor(plan, len(items))
		plan = tabStripPlanWithMin(widths, minWidths, available, controlW, *scroll)
	}

	dims := fixedHeight(gtx, ui.tabStripHeight(gtx), func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(items)+4)
		x := 0
		if plan.overflow {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutTabStripButton(th, gtx, prevClick, uitheme.ChevronLeftIcon(), plan.start > 0)
			}))
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutTabStripSeparatorStyle(gtx, style.open)
			}))
			x += controlW + tabStripSeparatorWidth(gtx)
		}
		for i := plan.start; i < plan.end; i++ {
			idx := i
			tabW := widths[idx]
			if planIdx := i - plan.start; planIdx >= 0 && planIdx < len(plan.widths) {
				tabW = plan.widths[planIdx]
			}
			if items[idx].active {
				geometry.activeMinX = x
				geometry.activeMaxX = x + tabW
				geometry.activeVisible = true
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, tabW, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutTabStripTab(th, gtx, items[idx], &(*tabClicks)[idx], &(*closeClicks)[idx], idx, len(items) > 1, style)
				})
			}))
			x += tabW
			if i < plan.end-1 || plan.overflow {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutTabStripSeparatorStyle(gtx, style.open)
				}))
				x += tabStripSeparatorWidth(gtx)
			}
		}
		if plan.overflow {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutTabStripButton(th, gtx, nextClick, uitheme.ChevronRightIcon(), plan.end < len(items))
			}))
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutTabStripSeparatorStyle(gtx, style.open)
			}))
			x += controlW + tabStripSeparatorWidth(gtx)
		} else if len(items) > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutTabStripSeparatorStyle(gtx, style.open)
			}))
			x += tabStripSeparatorWidth(gtx)
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutTabStripButton(th, gtx, addClick, uitheme.AddIcon(), true)
		}))
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
	return actions, dims, geometry
}

func (ui *UI) layoutTabStripTab(th *material.Theme, gtx layout.Context, item appTabItem, click, closeClick *widget.Clickable, idx int, closable bool, style appTabStripStyle) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		bg, fg := ui.tabStripColors(item.active, click.Hovered(), idx)
		if style.open && item.active {
			bg = style.activeBackground
		}
		focusColor := style.keyboardFocus
		if focusColor.A == 0 {
			focusColor = ui.tabStripAccentColor(idx)
		}
		if item.keyboardFocused {
			fg = focusColor
		}
		if bg.A != 0 {
			paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())
		}
		if item.active && !style.open {
			h := gtx.Constraints.Max.Y
			if h < 1 {
				h = gtx.Dp(unit.Dp(tabStripHeightDp))
			}
			paint.FillShape(gtx.Ops, ui.tabStripAccentColor(idx), clip.Rect(image.Rect(0, h-1, gtx.Constraints.Max.X, h)).Op())
		}
		rightPad := unit.Dp(2)
		if !closable {
			rightPad = unit.Dp(tabStripTitlePadDp)
		}
		dimensions := layout.Inset{Left: unit.Dp(tabStripTitlePadDp), Right: rightPad, Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if item.remoteKey == "" {
						return layout.Dimensions{}
					}
					return ui.layoutRemoteTabIndicator(th, gtx, item)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, item.title)
					lbl.Font.Typeface = ui.tabStripTypeface()
					lbl.Font.Weight = font.Medium
					lbl.TextSize = ui.tabStripTextSize()
					lbl.Color = fg
					lbl.MaxLines = 1
					lbl.Truncator = ".."
					lbl.Alignment = text.Middle
					return layoutVCenteredLabel(gtx, lbl)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !closable {
						return layout.Dimensions{}
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
		if item.keyboardFocused && dimensions.Size.X > 0 && dimensions.Size.Y > 0 {
			indicatorHeight := max(1, gtx.Dp(unit.Dp(2)))
			indicatorPad := gtx.Dp(unit.Dp(5))
			if indicatorPad*2 >= dimensions.Size.X {
				indicatorPad = 0
			}
			paint.FillShape(gtx.Ops, focusColor, clip.Rect(image.Rect(indicatorPad, dimensions.Size.Y-indicatorHeight, dimensions.Size.X-indicatorPad, dimensions.Size.Y)).Op())
		}
		return dimensions
	})
}

func (ui *UI) drawTabStripActionSpot(gtx layout.Context, highlighted bool) {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	palette := filePanePaletteFromConfig(ui.fmCfg)
	bg := mixNRGBA(palette.PaneBg, color.NRGBA{R: 82, G: 88, B: 104, A: 255}, 0.52)
	bg.A = 54
	if highlighted {
		bg = mixNRGBA(palette.PaneBg, color.NRGBA{R: 43, G: 129, B: 157, A: 255}, 0.76)
		bg.A = 220
	}
	paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rectangle{Max: size}).Op())
	separator := ui.tabStripSeparatorColor()
	separator.A = 96
	paint.FillShape(gtx.Ops, separator, clip.Rect(image.Rect(0, size.Y/4, 1, size.Y-size.Y/4)).Op())
	if highlighted {
		accent := color.NRGBA{R: 92, G: 214, B: 255, A: 232}
		paint.FillShape(gtx.Ops, accent, clip.Rect(image.Rect(0, size.Y-1, size.X, size.Y)).Op())
	}
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

func (ui *UI) layoutFlatCloseButton(gtx layout.Context, click *widget.Clickable, disabled bool) layout.Dimensions {
	return ui.layoutFlatCloseButtonState(gtx, click, disabled, false)
}

func (ui *UI) layoutFlatCloseButtonState(gtx layout.Context, click *widget.Clickable, disabled, focused bool) layout.Dimensions {
	if click == nil {
		return layout.Dimensions{}
	}
	buttonW := gtx.Dp(unit.Dp(20))
	buttonH := gtx.Dp(unit.Dp(18))
	if buttonW < 16 {
		buttonW = 16
	}
	if buttonH < 16 {
		buttonH = 16
	}
	return fixedWidth(gtx, buttonW, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, buttonH, func(gtx layout.Context) layout.Dimensions {
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if !disabled {
					pointer.CursorPointer.Add(gtx.Ops)
				}
				highlighted := !disabled && (click.Hovered() || click.Pressed())
				ui.drawTabStripCloseSpot(gtx, highlighted)
				if focused && !disabled && !highlighted {
					h := gtx.Dp(unit.Dp(2))
					if h < 1 {
						h = 1
					}
					focusColor := ui.tabStripAccentColor(0)
					focusColor.A = 190
					paint.FillShape(gtx.Ops, focusColor, clip.Rect(image.Rect(0, buttonH-h, buttonW, buttonH)).Op())
				}
				iconColor := ui.tabStripCloseColor(tabStripInactiveColor(), highlighted)
				if disabled {
					iconColor.A = 72
				}
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
	})
}

func (ui *UI) layoutTabStripButton(_ *material.Theme, gtx layout.Context, click *widget.Clickable, icon *widget.Icon, enabled bool) layout.Dimensions {
	return fixedWidth(gtx, tabStripControlWidth(gtx), func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if enabled {
				pointer.CursorPointer.Add(gtx.Ops)
			}
			bg, fg := ui.tabStripButtonColors(enabled, click.Hovered())
			if bg.A != 0 {
				paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())
			}
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := gtx.Dp(unit.Dp(15))
				if size < 10 {
					size = 10
				}
				if icon != nil {
					iconGtx := gtx
					iconGtx.Constraints = layout.Exact(image.Pt(size, size))
					icon.Layout(iconGtx, fg)
				}
				return layout.Dimensions{Size: image.Pt(size, size)}
			})
		})
	})
}

func (ui *UI) layoutTabStripSeparator(gtx layout.Context) layout.Dimensions {
	return ui.layoutTabStripSeparatorStyle(gtx, false)
}

func (ui *UI) layoutTabStripSeparatorStyle(gtx layout.Context, fullHeight bool) layout.Dimensions {
	w := tabStripSeparatorWidth(gtx)
	h := gtx.Constraints.Max.Y
	if h < 1 {
		h = gtx.Dp(unit.Dp(tabStripHeightDp))
	}
	lineH := h
	y0 := 0
	if !fullHeight {
		lineH = h - gtx.Dp(unit.Dp(7))
		if lineH < h/2 {
			lineH = h / 2
		}
		y0 = (h - lineH) / 2
	}
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
	if !enabled {
		fg = mixNRGBA(palette.PaneFg, palette.PaneBg, 0.72)
		fg.A = 72
		return bg, fg
	}
	if enabled {
		fg = mixNRGBA(fg, ui.tabStripAccentColor(0), 0.25)
		fg.A = 214
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

func (ui *UI) remoteHostAccent(identity string) color.NRGBA {
	return remoteHostAccentForBackground(identity, filePanePaletteFromConfig(ui.fmCfg).PaneBg)
}

func remoteHostAccentForBackground(identity string, bg color.NRGBA) color.NRGBA {
	colors := [...]color.NRGBA{
		{R: 84, G: 184, B: 255, A: 255},
		{R: 86, G: 211, B: 146, A: 255},
		{R: 255, G: 181, B: 71, A: 255},
		{R: 194, G: 139, B: 255, A: 255},
		{R: 255, G: 113, B: 133, A: 255},
		{R: 75, G: 216, B: 211, A: 255},
		{R: 123, G: 156, B: 255, A: 255},
		{R: 164, G: 207, B: 83, A: 255},
		{R: 255, G: 144, B: 82, A: 255},
		{R: 224, G: 112, B: 218, A: 255},
		{R: 235, G: 203, B: 78, A: 255},
		{R: 82, G: 197, B: 236, A: 255},
	}
	hash := uint32(2166136261)
	for _, b := range []byte(strings.ToLower(strings.TrimSpace(identity))) {
		hash ^= uint32(b)
		hash *= 16777619
	}
	accent := colors[int(hash%uint32(len(colors)))]
	if contrastScore(bg, accent) < 2.1 {
		if relativeLuminance(bg) >= 0.42 {
			accent = mixNRGBA(accent, color.NRGBA{A: 255}, 0.38)
		} else {
			accent = mixNRGBA(accent, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, 0.42)
		}
	}
	accent.A = 255
	return accent
}

func (ui *UI) layoutRemoteIndicator(gtx layout.Context, identity string, alpha float32) layout.Dimensions {
	return ui.layoutRemoteIndicatorOn(gtx, identity, alpha, filePanePaletteFromConfig(ui.fmCfg).PaneBg)
}

func (ui *UI) layoutRemoteTabIndicator(th *material.Theme, gtx layout.Context, item appTabItem) layout.Dimensions {
	state := item.remote
	if state != nil {
		for {
			ev, ok := gtx.Event(pointer.Filter{Target: state, Kinds: pointer.Enter | pointer.Move | pointer.Leave})
			if !ok {
				break
			}
			pe, ok := ev.(pointer.Event)
			if !ok {
				continue
			}
			switch pe.Kind {
			case pointer.Enter, pointer.Move:
				state.hovered = true
			case pointer.Leave:
				state.hovered = false
			}
		}
	}
	dims := ui.layoutRemoteIndicator(gtx, item.remoteKey, 1)
	if state != nil {
		area := clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops)
		pass := pointer.PassOp{}.Push(gtx.Ops)
		event.Op(gtx.Ops, state)
		pass.Pop()
		area.Pop()
	}
	if state != nil && state.hovered && strings.TrimSpace(item.remoteTip) != "" {
		ui.deferRemoteTabTooltip(th, gtx, dims.Size, item.remoteTip)
	}
	return dims
}

func (ui *UI) deferRemoteTabTooltip(th *material.Theme, gtx layout.Context, indicatorSize image.Point, tip string) {
	tip = strings.TrimSpace(tip)
	if tip == "" {
		return
	}
	m := op.Record(gtx.Ops)
	offset := op.Offset(image.Pt(-gtx.Dp(unit.Dp(4)), ui.tabStripHeight(gtx)+gtx.Dp(unit.Dp(3))))
	offset.Add(gtx.Ops)
	tipGtx := gtx
	tipGtx.Constraints.Min = image.Point{}
	tipGtx.Constraints.Max = image.Pt(gtx.Dp(unit.Dp(320)), gtx.Dp(unit.Dp(44)))
	ui.layoutRemoteTabTooltip(th, tipGtx, tip)
	op.Defer(gtx.Ops, m.Stop())
}

func (ui *UI) layoutRemoteTabTooltip(th *material.Theme, gtx layout.Context, tip string) layout.Dimensions {
	theme := ui.filePanePopupTheme()
	return fillRoundedBox(gtx, gtx.Dp(unit.Dp(4)), theme.Bg, theme.Border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, tip)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.functionBarTextSize()
			lbl.Color = theme.Text
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		})
	})
}

func (ui *UI) layoutRemoteIndicatorOn(gtx layout.Context, identity string, alpha float32, bg color.NRGBA) layout.Dimensions {
	w := gtx.Dp(unit.Dp(remoteIndicatorWidthDp))
	size := gtx.Dp(unit.Dp(8))
	if w < 8 {
		w = 8
	}
	if size < 6 {
		size = 6
	}
	return fixedWidth(gtx, w, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			c := scaleColorAlpha(remoteHostAccentForBackground(identity, bg), alpha)
			stroke := float32(gtx.Dp(unit.Dp(1)))
			if stroke < 1 {
				stroke = 1
			}
			pad := float32(size) * 0.18
			end := float32(size) - pad
			arm := float32(size) * 0.36
			var p clip.Path
			p.Begin(gtx.Ops)
			p.MoveTo(f32.Pt(pad, end))
			p.LineTo(f32.Pt(end, pad))
			p.MoveTo(f32.Pt(end-arm, pad))
			p.LineTo(f32.Pt(end, pad))
			p.LineTo(f32.Pt(end, pad+arm))
			paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: stroke}.Op())
			return layout.Dimensions{Size: image.Pt(size, size)}
		})
	})
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
	widths   []int
}

func tabStripPlan(widths []int, available, controlW, scroll int) tabPlan {
	return tabStripPlanWithMin(widths, widths, available, controlW, scroll)
}

func tabStripPlanWithMin(widths, minWidths []int, available, controlW, scroll int) tabPlan {
	if len(widths) == 0 {
		return tabPlan{}
	}
	minWidths = tabStripNormalizeMinWidths(widths, minWidths)
	compactWidths := tabStripCompactWidths(widths, minWidths)
	gap := 1
	total := tabStripTotalWidth(widths, gap)
	addW := controlW + gap
	if total+addW <= available {
		return tabPlan{start: 0, end: len(widths), widths: tabStripCopyWidths(widths, 0, len(widths))}
	}
	totalCompact := tabStripTotalWidth(compactWidths, gap)
	if totalCompact+addW <= available {
		budget := available - controlW - len(widths)*gap
		return tabPlan{start: 0, end: len(widths), widths: tabStripFitWidths(widths, compactWidths, 0, len(widths), budget)}
	}
	scroll = clampTabScrollAnchor(scroll, len(widths))
	tabSpace := available - 3*controlW - 3*gap
	if tabSpace < compactWidths[scroll] {
		tabSpace = compactWidths[scroll]
	}
	start := scroll
	end := scroll + 1
	used := compactWidths[scroll]
	for end < len(widths) {
		next := gap + compactWidths[end]
		if used+next > tabSpace {
			break
		}
		used += next
		end++
	}
	for start > 0 {
		next := gap + compactWidths[start-1]
		if used+next > tabSpace {
			break
		}
		start--
		used += next
	}
	visible := end - start
	budget := available - 3*controlW - (visible+2)*gap
	return tabPlan{
		start:    start,
		end:      end,
		overflow: true,
		widths:   tabStripFitWidths(widths, compactWidths, start, end, budget),
	}
}

func tabStripNormalizeMinWidths(widths, minWidths []int) []int {
	out := make([]int, len(widths))
	for i, w := range widths {
		minW := w
		if i < len(minWidths) {
			minW = minWidths[i]
		}
		if minW < 1 {
			minW = 1
		}
		if w > 0 && minW > w {
			minW = w
		}
		out[i] = minW
	}
	return out
}

func tabStripTotalWidth(widths []int, gap int) int {
	total := 0
	for i, w := range widths {
		total += w
		if i > 0 {
			total += gap
		}
	}
	return total
}

func tabStripCompactWidths(widths, minWidths []int) []int {
	minWidths = tabStripNormalizeMinWidths(widths, minWidths)
	out := make([]int, len(widths))
	for i, w := range widths {
		if w < 1 {
			w = 1
		}
		compact := (w*4 + 4) / 5
		if compact < minWidths[i] {
			compact = minWidths[i]
		}
		if compact > w {
			compact = w
		}
		out[i] = compact
	}
	return out
}

func tabStripCopyWidths(widths []int, start, end int) []int {
	if start < 0 {
		start = 0
	}
	if end > len(widths) {
		end = len(widths)
	}
	if start >= end {
		return nil
	}
	out := make([]int, end-start)
	copy(out, widths[start:end])
	return out
}

func tabStripFitWidths(widths, minWidths []int, start, end, budget int) []int {
	if start < 0 {
		start = 0
	}
	if end > len(widths) {
		end = len(widths)
	}
	if start >= end {
		return nil
	}
	minWidths = tabStripNormalizeMinWidths(widths, minWidths)
	out := make([]int, end-start)
	mins := make([]int, end-start)
	total := 0
	minTotal := 0
	for i := start; i < end; i++ {
		w := widths[i]
		if w < 1 {
			w = 1
		}
		out[i-start] = w
		mins[i-start] = minWidths[i]
		total += w
		minTotal += minWidths[i]
	}
	if budget < minTotal {
		copy(out, mins)
		return out
	}
	if total > budget {
		deficit := total - budget
		for deficit > 0 {
			adjustable := 0
			for i := range out {
				if out[i] > mins[i] {
					adjustable++
				}
			}
			if adjustable == 0 {
				break
			}
			step := (deficit + adjustable - 1) / adjustable
			for i := range out {
				if deficit <= 0 {
					break
				}
				room := out[i] - mins[i]
				if room <= 0 {
					continue
				}
				cut := step
				if cut > room {
					cut = room
				}
				if cut > deficit {
					cut = deficit
				}
				out[i] -= cut
				deficit -= cut
			}
		}
		return out
	}
	if total < budget && len(out) > 0 {
		extra := budget - total
		add := extra / len(out)
		rem := extra % len(out)
		for i := range out {
			out[i] += add
			if i < rem {
				out[i]++
			}
		}
	}
	return out
}

func (ui *UI) tabStripWidths(th *material.Theme, gtx layout.Context, cfg *fm.Config, items []appTabItem) []int {
	face := font.Typeface("")
	size := unit.Sp(10)
	if ui != nil {
		face = ui.tabStripTypeface()
		size = ui.tabStripTextSize()
	}
	return tabStripWidthsForTheme(th, gtx, cfg, items, face, size)
}

func tabStripWidths(gtx layout.Context, cfg *fm.Config, items []appTabItem) []int {
	return tabStripWidthsForTheme(nil, gtx, cfg, items, "", 10)
}

func tabStripWidthsForTheme(th *material.Theme, gtx layout.Context, cfg *fm.Config, items []appTabItem, face font.Typeface, size unit.Sp) []int {
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
			closeW := tabStripCloseWidthDp
			if len(items) <= 1 {
				closeW = 2
			}
			padding := gtx.Dp(unit.Dp(tabStripTitlePadDp + 2 + closeW + 4))
			if item.remoteKey != "" {
				padding += gtx.Dp(unit.Dp(remoteIndicatorWidthDp))
			}
			w = padding + tabStripTitleTextWidth(th, gtx, face, size, item.title)
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

func tabStripTitleTextWidth(th *material.Theme, gtx layout.Context, face font.Typeface, size unit.Sp, title string) int {
	if th == nil || th.Shaper == nil {
		charW := gtx.Dp(unit.Dp(7))
		return utf8.RuneCountInString(title) * charW
	}
	lbl := material.Body2(th, title)
	if face != "" {
		lbl.Font.Typeface = face
	}
	lbl.Font.Weight = font.Medium
	lbl.TextSize = size
	lbl.MaxLines = 1
	lbl.Truncator = ""
	return measureLabelUnconstrained(gtx, lbl).Size.X
}

func (ui *UI) tabStripTypeface() font.Typeface {
	if ui == nil || ui.fmCfg == nil || ui.fmCfg.Tabs.Typeface == "" {
		return ui.mainTypeface()
	}
	if !resources.IsBundledFontFamily(ui.fmCfg.Tabs.Typeface) {
		return ui.mainTypeface()
	}
	return font.Typeface(ui.fmCfg.Tabs.Typeface)
}

func (ui *UI) tabStripTextSize() unit.Sp {
	if ui == nil || ui.fmCfg == nil || ui.fmCfg.Tabs.FontSizeSp < 6 {
		return 10
	}
	return unit.Sp(ui.fmCfg.Tabs.FontSizeSp)
}

func tabStripMinWidths(gtx layout.Context, cfg *fm.Config, count int) []int {
	minW := gtx.Dp(unit.Dp(72))
	if cfg != nil {
		minW = gtx.Dp(unit.Dp(cfg.Tabs.MinWidthDp))
	}
	if minW < 44 {
		minW = 44
	}
	out := make([]int, count)
	for i := range out {
		out[i] = minW
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

func clampTabScrollAnchor(idx, n int) int {
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

func tabStripPrevScrollAnchor(plan tabPlan) int {
	if plan.start <= 0 {
		return 0
	}
	return plan.start - 1
}

func tabStripNextScrollAnchor(plan tabPlan, n int) int {
	if plan.end >= n {
		return clampTabScrollAnchor(plan.start, n)
	}
	return clampTabScrollAnchor(plan.start+1, n)
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

func ensureRemoteIndicatorHovers(dst *[]*remoteIndicatorHover, n int) {
	if dst == nil {
		return
	}
	for len(*dst) < n {
		*dst = append(*dst, &remoteIndicatorHover{})
	}
}

func prepareRemoteIndicatorHover(state *remoteIndicatorHover, identity string) *remoteIndicatorHover {
	if state == nil || identity == "" {
		return nil
	}
	if state.identity != identity {
		state.identity = identity
		state.hovered = false
	}
	return state
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

func wrappedTabIndex(idx, step, n int) int {
	if n <= 0 {
		return 0
	}
	idx = clampTabIndex(idx, n)
	next := (idx + step) % n
	if next < 0 {
		next += n
	}
	return next
}

func tabScrollToActive(scroll, active int) int {
	if active < 0 {
		return 0
	}
	return active
}

func tabScrollAfterClose(scroll, closed, count int) int {
	if closed >= 0 && scroll > closed {
		scroll--
	}
	return clampTabScrollAnchor(scroll, count)
}

func filePaneTabTitle(pane *filePaneState) string {
	if pane == nil {
		return "tab"
	}
	dir := ""
	if pane.remoteConnected() {
		dir = strings.TrimSpace(pane.dir)
		if dir == "" {
			dir = "/"
		}
	} else {
		dir = strings.TrimSpace(pane.displayDir())
	}
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
		return base
	}
	clean := filepath.Clean(dir)
	base := filepath.Base(clean)
	if base == "." || base == string(filepath.Separator) {
		return clean
	}
	return base
}

func filePaneTabItem(pane *filePaneState) appTabItem {
	item := appTabItem{title: filePaneTabTitle(pane)}
	if pane != nil && pane.remoteConnected() && pane.remote != nil {
		item.remoteKey = sshSetupIdentity(pane.remote.setup)
		item.remoteTip = sshSetupRemoteTooltip(pane.remote.setup)
	}
	return item
}

func sshSetupRemoteTooltip(setup fm.SSHSetup) string {
	identity := sshSetupIdentity(setup)
	name := strings.TrimSpace(setup.Name)
	if name != "" && name != identity {
		return name + " · " + identity
	}
	return identity
}

func terminalTabTitle(st *terminalSession) string {
	if st == nil {
		return "terminal"
	}
	if loc, ok := st.osc7Location(); ok && strings.TrimSpace(loc.Dir) != "" {
		if terminalOSC7HostIsLocal(loc.Host) {
			return terminalDirTitle(terminalOSC7LocalDir(loc.Dir))
		}
		base := path.Base(path.Clean(loc.Dir))
		if base == "." || base == "/" {
			base = path.Clean(loc.Dir)
		}
		return base
	}
	if title, ok := st.reportedDirectoryTitle(); ok {
		return title
	}
	if dir := st.pendingDirectory(); dir != "" {
		return terminalDirTitle(dir)
	}
	if dir, ok := st.currentDir(); ok && strings.TrimSpace(dir) != "" {
		return terminalDirTitle(dir)
	}
	return "terminal"
}

func terminalTabItem(st *terminalSession, cfg *fm.Config) appTabItem {
	item := appTabItem{title: terminalTabTitle(st)}
	loc, ok := st.osc7Location()
	if !ok || terminalOSC7HostIsLocal(loc.Host) {
		return item
	}
	host := terminalOSC7DisplayHost(loc)
	if setup, found, ambiguous := findSSHSetupForTerminalOSC7(cfg, loc); found && !ambiguous {
		item.remoteKey = sshSetupIdentity(setup)
		item.remoteTip = sshSetupRemoteTooltip(setup)
	} else {
		item.remoteKey = host
		item.remoteTip = host
	}
	if strings.TrimSpace(loc.Dir) != "" {
		base := path.Base(path.Clean(loc.Dir))
		if base == "." || base == "/" {
			base = path.Clean(loc.Dir)
		}
		item.title = base
	}
	return item
}

func (st *terminalSession) reportedDirectoryTitle() (string, bool) {
	if st == nil || st.term == nil {
		return "", false
	}
	st.parserMu.Lock()
	reported := strings.TrimSpace(st.term.Title())
	st.parserMu.Unlock()
	if reported == "" {
		return "", false
	}
	candidate := reported
	if idx := strings.LastIndex(candidate, ": "); idx >= 0 {
		candidate = strings.TrimSpace(candidate[idx+2:])
	}
	if candidate == "~" {
		return candidate, true
	}
	if len(candidate) >= 3 && candidate[1] == ':' && (candidate[2] == '\\' || candidate[2] == '/') {
		candidate = strings.ReplaceAll(candidate, `\`, "/")
	} else if !strings.Contains(candidate, "/") && !strings.Contains(candidate, `\`) {
		return "", false
	} else {
		candidate = strings.ReplaceAll(candidate, `\`, "/")
	}
	clean := path.Clean(candidate)
	base := path.Base(clean)
	if base == "." || base == "/" || base == "" {
		return clean, true
	}
	return base, true
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
