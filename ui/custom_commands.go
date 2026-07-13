// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"hexone/fm"
	"image"
	"image/color"
	pathpkg "path"
	"path/filepath"
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

type customCommandMenuSpec struct {
	key      string
	label    string
	shortcut string
	editor   bool
	index    int
	command  fm.CustomCommand
}

type customCommandEditorState struct {
	draft    []fm.CustomCommand
	saved    []fm.CustomCommand
	selected int

	nameEdit    widget.Editor
	commandEdit widget.Editor

	backdropClick widget.Clickable
	closeClick    widget.Clickable
	cancelClick   widget.Clickable
	saveClick     widget.Clickable
	runClick      widget.Clickable

	commandClicks []widget.Clickable

	actionsAnim  segmentedAnimState
	lastErr      string
	keyFocus     dialogKeyboardFocusState
	focus        customCommandEditorFocus
	focusPending customCommandEditorFocus
	actionFocus  customCommandEditorAction
}

type customCommandEditorFocus uint8

const (
	customCommandEditorFocusNone customCommandEditorFocus = iota
	customCommandEditorFocusSlots
	customCommandEditorFocusName
	customCommandEditorFocusCommand
	customCommandEditorFocusActions
)

type customCommandEditorAction uint8

const (
	customCommandEditorActionCancel customCommandEditorAction = iota
	customCommandEditorActionSave
	customCommandEditorActionRun
)

func (ui *UI) ensureCustomCommandMenuClicks(n int) {
	if ui == nil {
		return
	}
	if n <= cap(ui.customCommandMenuClicks) {
		ui.customCommandMenuClicks = ui.customCommandMenuClicks[:n]
		return
	}
	old := ui.customCommandMenuClicks
	ui.customCommandMenuClicks = make([]widget.Clickable, n)
	copy(ui.customCommandMenuClicks, old)
}

func (ui *UI) customCommandMenuSpecs() []customCommandMenuSpec {
	items := []customCommandMenuSpec{{
		key:    "editor",
		label:  "Custom command editor",
		editor: true,
		index:  -1,
	}}
	if ui == nil || ui.fmCfg == nil {
		return items
	}
	commands := customCommandSlots(ui.fmCfg.CustomCommands)
	for i, cmd := range commands {
		if strings.TrimSpace(cmd.Command) == "" {
			continue
		}
		items = append(items, customCommandMenuSpec{
			key:      fmt.Sprintf("cmd:%d:%s", i, cmd.Name),
			label:    cmd.Name,
			shortcut: customCommandMenuShortcut(i),
			index:    i,
			command:  cmd,
		})
	}
	return items
}

func customCommandMenuShortcut(index int) string {
	if index < 0 || index >= 10 {
		return ""
	}
	keyText := fmt.Sprintf("%d", index+1)
	if index == 9 {
		keyText = "0"
	}
	return "Ctrl+" + keyText
}

func customCommandSlots(raw []fm.CustomCommand) []fm.CustomCommand {
	slots := make([]fm.CustomCommand, 10)
	for i := range slots {
		slots[i].Slot = i + 1
	}
	for _, cmd := range fm.NormalizeCustomCommands(raw) {
		if cmd.Slot < 1 || cmd.Slot > 10 {
			continue
		}
		slots[cmd.Slot-1] = cmd
	}
	return slots
}

func customCommandShortcutKeyFilters(optional key.Modifiers) []event.Filter {
	filters := make([]event.Filter, 0, 30)
	for _, name := range []key.Name{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"} {
		filters = append(filters,
			key.Filter{Name: name, Required: key.ModCtrl, Optional: optional},
			key.Filter{Name: name, Required: key.ModShortcut, Optional: optional},
			key.Filter{Name: name, Required: key.ModCommand, Optional: optional},
		)
	}
	return filters
}

func customCommandMenuDefaultIndex(items []customCommandMenuSpec) int {
	if len(items) == 0 {
		return -1
	}
	return 0
}

func clampCustomCommandMenuIndex(index, n int) int {
	if n <= 0 {
		return -1
	}
	if index < 0 {
		return 0
	}
	if index >= n {
		return n - 1
	}
	return index
}

func (ui *UI) setCustomCommandMenuSelection(index int, items []customCommandMenuSpec) bool {
	if ui == nil {
		return false
	}
	next := clampCustomCommandMenuIndex(index, len(items))
	if next < 0 {
		next = customCommandMenuDefaultIndex(items)
	}
	if ui.customCommandMenuSelected == next {
		return false
	}
	ui.customCommandMenuSelected = next
	return true
}

func (ui *UI) currentCustomCommandMenuSelection(items []customCommandMenuSpec) int {
	if ui == nil {
		return -1
	}
	if idx := clampCustomCommandMenuIndex(ui.customCommandMenuSelected, len(items)); idx >= 0 {
		return idx
	}
	return customCommandMenuDefaultIndex(items)
}

func (ui *UI) moveCustomCommandMenuSelection(delta int) bool {
	if ui == nil || !ui.customCommandMenuOpen || delta == 0 {
		return false
	}
	items := ui.customCommandMenuSpecs()
	if len(items) == 0 {
		return false
	}
	index := ui.currentCustomCommandMenuSelection(items)
	if index < 0 {
		return false
	}
	index += delta
	if index < 0 {
		index = len(items) - 1
	} else if index >= len(items) {
		index = 0
	}
	return ui.setCustomCommandMenuSelection(index, items)
}

func (ui *UI) activateSelectedCustomCommandMenuItem(now time.Time) bool {
	if ui == nil || !ui.customCommandMenuOpen {
		return false
	}
	items := ui.customCommandMenuSpecs()
	index := ui.currentCustomCommandMenuSelection(items)
	if index < 0 || index >= len(items) {
		return false
	}
	ui.activateCustomCommandMenuItem(items[index], now)
	return true
}

func (ui *UI) activateCustomCommandMenuItem(item customCommandMenuSpec, now time.Time) {
	if ui == nil {
		return
	}
	ui.closeCustomCommandMenu()
	if item.editor {
		ui.openCustomCommandEditor(-1)
		return
	}
	ui.startCustomCommandViewer(item.command, now)
}

func (ui *UI) activateCustomCommandMenuShortcut(name key.Name, mods key.Modifiers, now time.Time) bool {
	if ui == nil || !ui.customCommandMenuOpen {
		return false
	}
	idx, ok := customCommandShortcutSlotFromName(name)
	if !ok {
		return false
	}
	if mods != 0 && !customCommandShortcutModifier(mods) {
		return false
	}
	if !ui.activateCustomCommandSlot(idx, now) {
		return false
	}
	ui.closeCustomCommandMenu()
	return true
}

func customCommandShortcutModifier(mods key.Modifiers) bool {
	shortcutMods := key.ModCtrl | key.ModShortcut | key.ModCommand
	return mods&shortcutMods != 0 && mods&^shortcutMods == 0
}

func customCommandShortcutSlotFromName(name key.Name) (int, bool) {
	switch name {
	case "1":
		return 0, true
	case "2":
		return 1, true
	case "3":
		return 2, true
	case "4":
		return 3, true
	case "5":
		return 4, true
	case "6":
		return 5, true
	case "7":
		return 6, true
	case "8":
		return 7, true
	case "9":
		return 8, true
	case "0":
		return 9, true
	default:
		return -1, false
	}
}

func customCommandShortcutSlot(ke key.Event) (int, bool) {
	if !customCommandShortcutModifier(ke.Modifiers) {
		return -1, false
	}
	return customCommandShortcutSlotFromName(ke.Name)
}

func (ui *UI) activateCustomCommandSlot(slot int, now time.Time) bool {
	if ui == nil || ui.fmCfg == nil || slot < 0 || slot >= 10 {
		return false
	}
	commands := customCommandSlots(ui.fmCfg.CustomCommands)
	cmd := commands[slot]
	if strings.TrimSpace(cmd.Command) == "" {
		return false
	}
	return ui.startCustomCommandViewer(cmd, now)
}

func (ui *UI) activateCustomCommandFixedShortcut(ke key.Event, now time.Time) bool {
	if ui == nil || ui.fmCfg == nil || ke.State != key.Press {
		return false
	}
	slot, ok := customCommandShortcutSlot(ke)
	if !ok {
		return false
	}
	if !ui.activateCustomCommandSlot(slot, now) {
		return false
	}
	ui.closeCustomCommandMenu()
	return true
}

func (ui *UI) activateCustomCommandGlobalShortcut(ke key.Event, now time.Time) bool {
	if ui == nil || ui.Tabs.Value != "tab0" || ui.helpModal != nil || ui.settingsModal != nil || ui.sshModal != nil || ui.hasBlockingFileDialog() || ui.pathEditActive() || ui.fileViewer != nil {
		return false
	}
	return ui.activateCustomCommandFixedShortcut(ke, now)
}

func (ui *UI) customCommandMenuAnchorRect(gtx layout.Context) image.Rectangle {
	if ui != nil && ui.customCommandMenuButtonRect.Dx() > 0 && ui.customCommandMenuButtonRect.Dy() > 0 {
		return ui.customCommandMenuButtonRect
	}
	x := gtx.Dp(unit.Dp(functionBarOuterInsetDp)) + 1
	y := gtx.Dp(unit.Dp(functionBarTopInsetDp)) + 1
	h := gtx.Dp(unit.Dp(functionBarStripDp))
	if h < 1 {
		h = 1
	}
	return image.Rect(x, y, x, y+h)
}

func (ui *UI) handleCustomCommandMenuOutsideClick(gtx layout.Context) {
	if ui == nil || !ui.customCommandMenuOpen {
		return
	}
	pressedPopup := popupPressed(gtx, &ui.customCommandMenuBodyTag)
	closed := false
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &ui.customCommandMenuGlobalTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok || pe.Kind != pointer.Press || !pe.Buttons.Contain(pointer.ButtonPrimary) {
			continue
		}
		if ui.functionBarClicks[1].Hovered() || pressedPopup {
			continue
		}
		ui.closeCustomCommandMenu()
		closed = true
	}
	if closed {
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) customCommandMenuCardWidth(th *material.Theme, gtx layout.Context, items []customCommandMenuSpec) int {
	tools := make([]functionBarToolSpec, 0, len(items))
	for _, item := range items {
		tools = append(tools, functionBarToolSpec{key: item.key, label: item.label, shortcut: item.shortcut})
	}
	width := ui.functionBarToolCardWidth(th, gtx, tools)
	minWidth := gtx.Dp(unit.Dp(210))
	if width < minWidth {
		width = minWidth
	}
	return width
}

func (ui *UI) customCommandMenuHoveredIndex(items []customCommandMenuSpec) int {
	if ui == nil {
		return -1
	}
	for i := range items {
		if i < len(ui.customCommandMenuClicks) && ui.customCommandMenuClicks[i].Hovered() {
			return i
		}
	}
	return -1
}

func (ui *UI) layoutCustomCommandMenuCard(th *material.Theme, gtx layout.Context, items []customCommandMenuSpec, alpha float32) layout.Dimensions {
	width := ui.customCommandMenuCardWidth(th, gtx, items)
	theme := ui.filePanePopupTheme()
	hoverIndex := ui.customCommandMenuHoveredIndex(items)
	if hoverIndex >= 0 && hoverIndex != ui.customCommandMenuSelected {
		ui.customCommandMenuSelected = hoverIndex
	}
	selectedIndex := ui.currentCustomCommandMenuSelection(items)
	hoverID := ""
	if hoverIndex >= 0 && hoverIndex < len(items) {
		hoverID = items[hoverIndex].key
	} else if selectedIndex >= 0 && selectedIndex < len(items) {
		hoverID = items[selectedIndex].key
	}
	if hoverID != ui.customCommandMenuHoverID {
		ui.customCommandMenuHoverID = hoverID
		ui.customCommandMenuHoverAnim.setHover(hoverID, gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		dims := fillRoundedClipBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			scaleColorAlpha(theme.Bg, alpha),
			scaleColorAlpha(theme.Border, alpha),
			func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(items)+1)
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, ui.fileContextMenuTitleHeight(gtx), func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(4), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Caption(th, "F2")
							lbl.Font.Typeface = ui.interfaceTypeface()
							lbl.Font.Weight = font.Medium
							lbl.TextSize = scaleConfigFontSize(ui.fmCfg, 9)
							lbl.Color = scaleColorAlpha(theme.Title, alpha)
							lbl.MaxLines = 1
							return layoutVCenteredLabel(gtx, lbl)
						})
					})
				}))
				for i, item := range items {
					i := i
					item := item
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						hoverFill, animating := ui.customCommandMenuHoverAnim.hoverFill(gtx.Now, item.key)
						if animating {
							gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
						}
						tool := functionBarToolSpec{key: item.key, label: item.label, shortcut: item.shortcut}
						return ui.layoutFunctionBarToolOption(th, gtx, theme, &ui.customCommandMenuClicks[i], tool, i == selectedIndex, hoverFill, alpha)
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			},
		)
		registerPopupArea(gtx, &ui.customCommandMenuBodyTag, dims.Size)
		return dims
	})
}

func (ui *UI) layoutCustomCommandMenuPopup(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if ui == nil || !ui.customCommandMenuOpen {
		return layout.Dimensions{}
	}

	items := ui.customCommandMenuSpecs()
	ui.ensureCustomCommandMenuClicks(len(items))
	for i, item := range items {
		if i >= len(ui.customCommandMenuClicks) {
			break
		}
		for ui.customCommandMenuClicks[i].Clicked(gtx) {
			ui.activateCustomCommandMenuItem(item, gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
			return layout.Dimensions{}
		}
	}

	alpha, slideY, animating := popupOpenProgress(gtx.Now, ui.customCommandMenuOpenedAt)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	blockClip := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, &ui.customCommandMenuGlobalTag)
	blockClip.Pop()
	m := op.Record(gtx.Ops)
	card := ui.layoutCustomCommandMenuCard(th, gtx, items, alpha)
	call := m.Stop()

	anchorRect := ui.customCommandMenuAnchorRect(gtx)
	anchor := image.Point{
		X: anchorRect.Min.X,
		Y: anchorRect.Max.Y + gtx.Dp(unit.Dp(functionBarPopupGapDp)) + slideY,
	}
	anchor = clampFilePaneMenuPoint(anchor, card.Size, gtx.Constraints.Max)
	ui.customCommandMenuRect = image.Rectangle{Min: anchor, Max: anchor.Add(card.Size)}

	bodyClip := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	offset := op.Offset(anchor).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	bodyClip.Pop()

	ui.handleCustomCommandMenuOutsideClick(gtx)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *UI) handleCustomCommandMenuKeys(gtx layout.Context) {
	if ui == nil || !ui.customCommandMenuOpen {
		return
	}
	anyMods := ^key.Modifiers(0)
	for {
		filters := []event.Filter{
			key.Filter{Name: key.NameUpArrow},
			key.Filter{Name: key.NameDownArrow},
			key.Filter{Name: key.NameEnter},
			key.Filter{Name: key.NameReturn},
		}
		for _, name := range []key.Name{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"} {
			filters = append(filters,
				key.Filter{Name: name},
				key.Filter{Name: name, Required: key.ModCtrl, Optional: anyMods},
				key.Filter{Name: name, Required: key.ModShortcut, Optional: anyMods},
			)
		}
		filters = append(filters, customCommandShortcutKeyFilters(anyMods)...)
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		handled := false
		switch ke.Name {
		case key.NameUpArrow:
			if ke.Modifiers == 0 {
				handled = ui.moveCustomCommandMenuSelection(-1)
			}
		case key.NameDownArrow:
			if ke.Modifiers == 0 {
				handled = ui.moveCustomCommandMenuSelection(1)
			}
		case key.NameEnter, key.NameReturn:
			if ke.Modifiers == 0 {
				handled = ui.activateSelectedCustomCommandMenuItem(gtx.Now)
			}
		default:
			handled = ui.activateCustomCommandMenuShortcut(ke.Name, ke.Modifiers, gtx.Now)
			if !handled {
				handled = ui.activateCustomCommandFixedShortcut(ke, gtx.Now)
			}
		}
		if handled {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (ui *UI) openCustomCommandEditor(index int) {
	if ui == nil {
		return
	}
	if err := ui.ensureFMConfigLoaded(); err != nil {
		if pane := ui.activePane(); pane != nil {
			pane.setNotice(err.Error(), time.Now())
		}
		return
	}
	ui.resetKeys()
	slots := customCommandSlots(ui.fmCfg.CustomCommands)
	st := &customCommandEditorState{
		draft:        cloneCustomCommands(slots),
		saved:        cloneCustomCommands(slots),
		selected:     -1,
		focus:        customCommandEditorFocusCommand,
		focusPending: customCommandEditorFocusCommand,
		actionFocus:  customCommandEditorActionSave,
	}
	st.nameEdit.SingleLine = true
	st.nameEdit.Submit = false
	st.commandEdit.SingleLine = false
	st.commandEdit.Submit = false
	ui.customCommandEditor = st
	if index < 0 || index >= 10 {
		index = 0
	}
	st.selectSlot(index)
}

func (ui *UI) closeCustomCommandEditor() {
	if ui == nil {
		return
	}
	ui.customCommandEditor = nil
	ui.closeEditorContextMenu()
}

func (st *customCommandEditorState) ensureCommandClicks(n int) {
	if st == nil {
		return
	}
	if n <= cap(st.commandClicks) {
		st.commandClicks = st.commandClicks[:n]
		return
	}
	old := st.commandClicks
	st.commandClicks = make([]widget.Clickable, n)
	copy(st.commandClicks, old)
}

func (st *customCommandEditorState) focusOrder() []customCommandEditorFocus {
	if st == nil {
		return nil
	}
	return []customCommandEditorFocus{
		customCommandEditorFocusSlots,
		customCommandEditorFocusName,
		customCommandEditorFocusCommand,
		customCommandEditorFocusActions,
	}
}

func (st *customCommandEditorState) syncEditorFocus(gtx layout.Context) {
	if st == nil {
		return
	}
	switch st.focusPending {
	case customCommandEditorFocusName:
		if gtx.Focused(&st.nameEdit) {
			st.focus = customCommandEditorFocusName
			st.focusPending = customCommandEditorFocusNone
		}
		return
	case customCommandEditorFocusCommand:
		if gtx.Focused(&st.commandEdit) {
			st.focus = customCommandEditorFocusCommand
			st.focusPending = customCommandEditorFocusNone
		}
		return
	case customCommandEditorFocusSlots, customCommandEditorFocusActions:
		if gtx.Focused(&st.keyFocus.tag) {
			st.focus = st.focusPending
			st.focusPending = customCommandEditorFocusNone
		}
		return
	}
	switch {
	case gtx.Focused(&st.nameEdit):
		st.focus = customCommandEditorFocusName
	case gtx.Focused(&st.commandEdit):
		st.focus = customCommandEditorFocusCommand
	}
}

func (st *customCommandEditorState) setFocus(target customCommandEditorFocus) bool {
	if st == nil || target == customCommandEditorFocusNone {
		return false
	}
	prev := st.focus
	changed := st.focus != target || st.focusPending != target
	st.focus = target
	st.focusPending = target
	if target == customCommandEditorFocusActions {
		if prev != customCommandEditorFocusActions {
			st.actionFocus = customCommandEditorActionSave
		}
		st.keyFocus.focusKeyboard()
	}
	if target == customCommandEditorFocusSlots {
		st.keyFocus.focusKeyboard()
	}
	return changed
}

func (st *customCommandEditorState) applyPendingEditorFocus(gtx layout.Context, target customCommandEditorFocus, tag any) {
	if st == nil || tag == nil || st.focusPending != target {
		return
	}
	gtx.Execute(key.FocusCmd{Tag: tag})
	st.focusPending = customCommandEditorFocusNone
}

func (st *customCommandEditorState) stepFocus(step int) bool {
	order := st.focusOrder()
	if len(order) == 0 {
		return false
	}
	current := -1
	for i, target := range order {
		if target == st.focus {
			current = i
			break
		}
	}
	return st.setFocus(order[dialogWrappedIndex(current, len(order), step)])
}

func (st *customCommandEditorState) stepAction(step int) bool {
	if st == nil {
		return false
	}
	order := []customCommandEditorAction{
		customCommandEditorActionCancel,
		customCommandEditorActionSave,
		customCommandEditorActionRun,
	}
	current := 0
	for i, action := range order {
		if action == st.actionFocus {
			current = i
			break
		}
	}
	next := order[dialogWrappedIndex(current, len(order), step)]
	if next == st.actionFocus {
		return false
	}
	st.actionFocus = next
	return true
}

func (st *customCommandEditorState) stepSlot(step int) bool {
	if st == nil || step == 0 {
		return false
	}
	st.ensureDraftSlots()
	current := st.selected
	if current < 0 || current >= 10 {
		current = 0
	}
	next := dialogWrappedIndex(current, 10, step)
	if next == st.selected {
		return false
	}
	st.selectSlot(next)
	return true
}

func (st *customCommandEditorState) actionVisualState(target customCommandEditorAction, _ bool) dialogActionVisualState {
	if st == nil {
		return dialogActionVisualState{}
	}
	if st.focus == customCommandEditorFocusActions {
		active := st.actionFocus == target
		return dialogActionVisualState{Focused: active, Default: active}
	}
	return dialogActionVisualState{Default: target == customCommandEditorActionSave}
}

func customCommandEditorEquivalent(a, b fm.CustomCommand) bool {
	return strings.TrimSpace(a.Name) == strings.TrimSpace(b.Name) &&
		strings.TrimSpace(a.Command) == strings.TrimSpace(b.Command)
}

func (st *customCommandEditorState) syncCurrentFieldsToDraft() {
	if st == nil || st.selected < 0 || st.selected >= 10 {
		return
	}
	st.ensureDraftSlots()
	st.draft[st.selected] = fm.CustomCommand{
		Slot:    st.selected + 1,
		Name:    strings.TrimSpace(st.nameEdit.Text()),
		Command: st.commandEdit.Text(),
	}
}

func (st *customCommandEditorState) slotDirty(index int) bool {
	if st == nil || index < 0 || index >= 10 {
		return false
	}
	st.ensureDraftSlots()
	if len(st.saved) != 10 {
		st.saved = customCommandSlots(st.saved)
	}
	return !customCommandEditorEquivalent(st.draft[index], st.saved[index])
}

func (st *customCommandEditorState) loadSlotFields(index int) {
	if st == nil || index < 0 || index >= 10 {
		return
	}
	st.ensureDraftSlots()
	cmd := st.draft[index]
	st.selected = index
	st.nameEdit.SetText(cmd.Name)
	st.commandEdit.SetText(cmd.Command)
	st.commandEdit.SetCaret(st.commandEdit.Len(), st.commandEdit.Len())
	st.lastErr = ""
}

func (st *customCommandEditorState) selectSlot(index int) {
	if st == nil || index < 0 || index >= 10 {
		return
	}
	st.syncCurrentFieldsToDraft()
	st.loadSlotFields(index)
}

func (st *customCommandEditorState) selectCommand(index int) {
	if st == nil || index < 0 || index >= 10 {
		return
	}
	st.selectSlot(index)
	st.setFocus(customCommandEditorFocusCommand)
}

func (st *customCommandEditorState) ensureDraftSlots() {
	if st == nil {
		return
	}
	if len(st.draft) != 10 {
		st.draft = customCommandSlots(st.draft)
	}
	if len(st.saved) != 10 {
		st.saved = customCommandSlots(st.saved)
	}
	for i := range st.draft {
		st.draft[i].Slot = i + 1
	}
	for i := range st.saved {
		st.saved[i].Slot = i + 1
	}
}

func (st *customCommandEditorState) refreshSavedSlotsFromConfig(cfg *fm.Config) {
	if st == nil || cfg == nil {
		return
	}
	slots := customCommandSlots(cfg.CustomCommands)
	st.draft = cloneCustomCommands(slots)
	st.saved = cloneCustomCommands(slots)
	index := st.selected
	if index < 0 || index >= 10 {
		index = 0
	}
	st.loadSlotFields(index)
}

func (st *customCommandEditorState) selectedDraftCommand() fm.CustomCommand {
	if st == nil || st.selected < 0 || st.selected >= 10 {
		return fm.CustomCommand{}
	}
	st.ensureDraftSlots()
	return st.draft[st.selected]
}

func (st *customCommandEditorState) currentCommandFields() (fm.CustomCommand, error) {
	if st == nil {
		return fm.CustomCommand{}, fmt.Errorf("custom command editor is not open")
	}
	st.ensureDraftSlots()
	if st.selected < 0 || st.selected >= 10 {
		return fm.CustomCommand{}, fmt.Errorf("custom command slot is not selected")
	}
	cmd, ok := fm.NormalizeCustomCommand(fm.CustomCommand{
		Slot:    st.selected + 1,
		Name:    st.nameEdit.Text(),
		Command: st.commandEdit.Text(),
	})
	if !ok {
		return fm.CustomCommand{}, fmt.Errorf("command is empty")
	}
	return cmd, nil
}

func (st *customCommandEditorState) upsertCurrentCommand() (fm.CustomCommand, error) {
	cmd, err := st.currentCommandFields()
	if err != nil {
		return fm.CustomCommand{}, err
	}
	st.ensureDraftSlots()
	cmd.Slot = st.selected + 1
	st.draft[st.selected] = cmd
	return cmd, nil
}

func (st *customCommandEditorState) canUpsertCurrentCommand() bool {
	_, err := st.currentCommandFields()
	return err == nil
}

func (ui *UI) saveCustomCommandEditorDraft() error {
	if ui == nil || ui.customCommandEditor == nil {
		return nil
	}
	if err := ui.ensureFMConfigLoaded(); err != nil {
		return err
	}
	ui.customCommandEditor.ensureDraftSlots()
	ui.fmCfg.CustomCommands = fm.NormalizeCustomCommands(ui.customCommandEditor.draft)
	return ui.saveFMConfigWithOptions("custom-commands", false)
}

func (ui *UI) saveCurrentCustomCommand() bool {
	st := ui.customCommandEditor
	if st == nil {
		return false
	}
	st.syncCurrentFieldsToDraft()
	st.ensureDraftSlots()
	if st.selected < 0 || st.selected >= 10 {
		st.lastErr = "custom command slot is not selected"
		return false
	}
	if strings.TrimSpace(st.commandEdit.Text()) == "" {
		st.draft[st.selected] = fm.CustomCommand{Slot: st.selected + 1}
		if err := ui.saveCustomCommandEditorDraft(); err != nil {
			st.lastErr = err.Error()
			return false
		}
		st.refreshSavedSlotsFromConfig(ui.fmCfg)
		st.lastErr = ""
		return true
	}
	if _, err := st.upsertCurrentCommand(); err != nil {
		st.lastErr = err.Error()
		return false
	}
	if err := ui.saveCustomCommandEditorDraft(); err != nil {
		st.lastErr = err.Error()
		return false
	}
	st.refreshSavedSlotsFromConfig(ui.fmCfg)
	st.lastErr = ""
	return true
}

func (ui *UI) runCurrentCustomCommand(now time.Time) bool {
	st := ui.customCommandEditor
	if st == nil {
		return false
	}
	st.syncCurrentFieldsToDraft()
	cmd, err := st.upsertCurrentCommand()
	if err != nil {
		st.lastErr = err.Error()
		return false
	}
	if err := ui.saveCustomCommandEditorDraft(); err != nil {
		st.lastErr = err.Error()
		return false
	}
	ui.closeCustomCommandEditor()
	return ui.startCustomCommandViewer(cmd, now)
}

func (ui *UI) handleCustomCommandEditorKeys(gtx layout.Context, st *customCommandEditorState) bool {
	if ui == nil || st == nil {
		return false
	}
	anyMods := ^key.Modifiers(0)
	for {
		editorFocused := gtx.Focused(&st.nameEdit) || gtx.Focused(&st.commandEdit)
		filters := []event.Filter{
			key.Filter{Name: key.NameEscape, Optional: anyMods},
			key.Filter{Name: key.NameTab, Optional: anyMods},
			key.Filter{Name: key.NameEnter, Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: key.NameReturn, Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: key.NameEnter, Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: key.NameReturn, Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "s", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "S", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "s", Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "S", Required: key.ModShortcut, Optional: anyMods},
		}
		if st.focus == customCommandEditorFocusSlots || st.focus == customCommandEditorFocusActions || !editorFocused {
			filters = append(filters,
				key.Filter{Name: key.NameEnter, Optional: anyMods},
				key.Filter{Name: key.NameReturn, Optional: anyMods},
				key.Filter{Name: key.NameLeftArrow, Optional: anyMods},
				key.Filter{Name: key.NameRightArrow, Optional: anyMods},
				key.Filter{Name: key.NameUpArrow, Optional: anyMods},
				key.Filter{Name: key.NameDownArrow, Optional: anyMods},
			)
		}
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case key.NameEscape:
			ui.closeCustomCommandEditor()
			return true
		case key.NameTab:
			step, ok := dialogTabStep(ke.Modifiers)
			if !ok {
				continue
			}
			if st.stepFocus(step) {
				gtx.Execute(op.InvalidateCmd{})
			}
			return true
		case key.NameLeftArrow:
			if ke.Modifiers != 0 || st.focus != customCommandEditorFocusActions {
				continue
			}
			if st.stepAction(-1) {
				gtx.Execute(op.InvalidateCmd{})
			}
			return true
		case key.NameRightArrow:
			if ke.Modifiers != 0 || st.focus != customCommandEditorFocusActions {
				continue
			}
			if st.stepAction(1) {
				gtx.Execute(op.InvalidateCmd{})
			}
			return true
		case key.NameUpArrow:
			if ke.Modifiers != 0 {
				continue
			}
			switch st.focus {
			case customCommandEditorFocusSlots:
				if st.stepSlot(-1) {
					gtx.Execute(op.InvalidateCmd{})
				}
			default:
			}
			return true
		case key.NameDownArrow:
			if ke.Modifiers != 0 {
				continue
			}
			switch st.focus {
			case customCommandEditorFocusSlots:
				if st.stepSlot(1) {
					gtx.Execute(op.InvalidateCmd{})
				}
			default:
			}
			return true
		case key.NameEnter, key.NameReturn:
			if ke.Modifiers.Contain(key.ModCtrl) || ke.Modifiers.Contain(key.ModShortcut) {
				if ui.runCurrentCustomCommand(gtx.Now) {
					gtx.Execute(op.InvalidateCmd{})
					return true
				}
				gtx.Execute(op.InvalidateCmd{})
				return true
			}
			if ke.Modifiers != 0 || st.focus != customCommandEditorFocusActions {
				if ke.Modifiers == 0 && st.focus == customCommandEditorFocusSlots {
					st.setFocus(customCommandEditorFocusCommand)
					gtx.Execute(op.InvalidateCmd{})
					return true
				}
				if ke.Modifiers == 0 {
					return true
				}
				continue
			}
			switch st.actionFocus {
			case customCommandEditorActionCancel:
				st.actionsAnim.setPulse("cancel", gtx.Now)
				ui.closeCustomCommandEditor()
				return true
			case customCommandEditorActionRun:
				st.actionsAnim.setPulse("run", gtx.Now)
				if ui.runCurrentCustomCommand(gtx.Now) {
					gtx.Execute(op.InvalidateCmd{})
					return true
				}
			default:
				st.actionsAnim.setPulse("save", gtx.Now)
				ui.saveCurrentCustomCommand()
			}
			gtx.Execute(op.InvalidateCmd{})
			return true
		case "s", "S":
			ui.saveCurrentCustomCommand()
			gtx.Execute(op.InvalidateCmd{})
			return true
		}
	}
	return false
}

func (ui *UI) handleCustomCommandEditorPreLayoutInput(gtx layout.Context) {
	st := ui.customCommandEditor
	if ui == nil || st == nil {
		return
	}
	st.keyFocus.attach(gtx)
	st.syncEditorFocus(gtx)
	ui.handleCustomCommandEditorKeys(gtx, st)
	st = ui.customCommandEditor
	if st == nil {
		return
	}
	for _, ed := range []*widget.Editor{&st.nameEdit, &st.commandEdit} {
		for {
			ev, ok := ed.Update(gtx)
			if !ok {
				break
			}
			if _, ok := ev.(widget.ChangeEvent); ok {
				st.syncCurrentFieldsToDraft()
				st.lastErr = ""
			}
		}
	}
}

func (ui *UI) layoutCustomCommandEditor(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.customCommandEditor
	if st == nil {
		return layout.Dimensions{}
	}
	st.keyFocus.attach(gtx)
	st.syncEditorFocus(gtx)

	st.ensureDraftSlots()
	st.ensureCommandClicks(10)
	for i := 0; i < 10; i++ {
		i := i
		for st.commandClicks[i].Clicked(gtx) {
			st.selectSlot(i)
			st.setFocus(customCommandEditorFocusSlots)
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	if st.cancelClick.Clicked(gtx) || st.closeClick.Clicked(gtx) {
		st.actionFocus = customCommandEditorActionCancel
		ui.closeCustomCommandEditor()
		return layout.Dimensions{}
	}
	if st.saveClick.Clicked(gtx) {
		st.actionFocus = customCommandEditorActionSave
		st.actionsAnim.setPulse("save", gtx.Now)
		ui.saveCurrentCustomCommand()
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.runClick.Clicked(gtx) {
		st.actionFocus = customCommandEditorActionRun
		st.actionsAnim.setPulse("run", gtx.Now)
		ui.runCurrentCustomCommand(gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
		return layout.Dimensions{}
	}
	for st.backdropClick.Clicked(gtx) {
	}

	return st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{A: 130}, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

		width := gtx.Dp(unit.Dp(760))
		if maxWidth := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(28)); width > maxWidth {
			width = maxWidth
		}
		if width < gtx.Dp(unit.Dp(340)) {
			width = gtx.Dp(unit.Dp(340))
		}

		m := op.Record(gtx.Ops)
		dialog := fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
			return fillRoundedBox(
				gtx,
				gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
				color.NRGBA{R: 20, G: 20, B: 20, A: 252},
				color.NRGBA{R: 255, G: 255, B: 255, A: 18},
				func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutCustomCommandEditorBody(th, gtx, st)
					})
				},
			)
		})
		call := m.Stop()

		x := (gtx.Constraints.Max.X - dialog.Size.X) / 2
		y := (gtx.Constraints.Max.Y - dialog.Size.Y) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: gtx.Constraints.Max, Baseline: dialog.Baseline}
	})
}

func (ui *UI) layoutCustomCommandEditorBody(th *material.Theme, gtx layout.Context, st *customCommandEditorState) layout.Dimensions {
	hoverActionKey := ""
	switch {
	case st.cancelClick.Hovered():
		hoverActionKey = "cancel"
	case st.saveClick.Hovered():
		hoverActionKey = "save"
	case st.runClick.Hovered():
		hoverActionKey = "run"
	}
	st.actionsAnim.setHover(hoverActionKey, gtx.Now)
	hoverCancel, hoverAnimCancel := st.actionsAnim.hoverFill(gtx.Now, "cancel")
	hoverSave, hoverAnimSave := st.actionsAnim.hoverFill(gtx.Now, "save")
	hoverRun, hoverAnimRun := st.actionsAnim.hoverFill(gtx.Now, "run")
	pulseCancel, pulseAnimCancel := st.actionsAnim.pulseFill(gtx.Now, "cancel")
	pulseSave, pulseAnimSave := st.actionsAnim.pulseFill(gtx.Now, "save")
	pulseRun, pulseAnimRun := st.actionsAnim.pulseFill(gtx.Now, "run")
	if hoverAnimCancel || hoverAnimSave || hoverAnimRun || pulseAnimCancel || pulseAnimSave || pulseAnimRun {
		gtx.Execute(op.InvalidateCmd{})
	}

	canRun := st.canUpsertCurrentCommand()

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					title := material.Body1(th, "Custom Command")
					title.Font.Typeface = ui.interfaceTypeface()
					title.Font.Weight = font.Bold
					title.TextSize = ui.scaleDialogFontSize(12)
					title.Color = txtColor
					return title.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutFlatCloseButton(gtx, &st.closeClick, false)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(layoutDialogHorizontalDivider),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			leftW := gtx.Dp(unit.Dp(190))
			if gtx.Constraints.Max.X < gtx.Dp(unit.Dp(520)) {
				leftW = 0
			}
			if leftW <= 0 {
				return ui.layoutCustomCommandEditorFields(th, gtx, st)
			}
			_, _, totalH := customCommandEditorListMetrics(gtx)
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, leftW, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutCustomCommandEditorList(th, gtx, st)
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, totalH, layoutDialogVerticalDivider)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutCustomCommandEditorFields(th, gtx, st)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.lastErr == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, st.lastErr)
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.TextSize = ui.scaleDialogFontSize(9)
				lbl.Color = color.NRGBA{R: 220, G: 140, B: 140, A: 255}
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(layoutDialogHorizontalDivider),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutDialogActionTriple(
					th, gtx,
					&st.cancelClick, "Cancel", hoverCancel, pulseCancel, false,
					&st.saveClick, "Save", hoverSave, pulseSave, false,
					&st.runClick, "Run", hoverRun, pulseRun, !canRun,
					st.actionVisualState(customCommandEditorActionCancel, canRun),
					st.actionVisualState(customCommandEditorActionSave, canRun),
					st.actionVisualState(customCommandEditorActionRun, canRun),
				)
			})
		}),
	)
}

func customCommandEditorSlotLabel(st *customCommandEditorState, index int) string {
	if st == nil || index < 0 || index >= 10 {
		return ""
	}
	st.ensureDraftSlots()
	name := strings.TrimSpace(st.draft[index].Name)
	if name == "" {
		name = fmt.Sprintf("Slot %d", index+1)
	}
	if st.slotDirty(index) {
		name += " *"
	}
	return fmt.Sprintf("%s  %s", customCommandMenuShortcut(index), name)
}

func (ui *UI) layoutCustomCommandEditorList(th *material.Theme, gtx layout.Context, st *customCommandEditorState) layout.Dimensions {
	st.ensureDraftSlots()
	stripH, sepH, totalH := customCommandEditorListMetrics(gtx)
	selected := st.selected
	if selected < 0 || selected >= 10 {
		selected = 0
	}
	return fillBgExact(gtx, color.NRGBA{R: 24, G: 24, B: 24, A: 255}, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, totalH, func(gtx layout.Context) layout.Dimensions {
			w := gtx.Constraints.Max.X
			if w < 1 {
				w = 1
			}
			step := stripH + sepH
			sliderY := selected * step
			maxSliderY := totalH - stripH
			if sliderY > maxSliderY {
				sliderY = maxSliderY
			}
			if sliderY < 0 {
				sliderY = 0
			}

			innerClip := clip.Rect(image.Rect(0, 0, w, totalH)).Push(gtx.Ops)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 54, G: 54, B: 54, A: 255}, clip.Rect(image.Rect(0, sliderY, w, sliderY+stripH)).Op())

			children := make([]layout.FlexChild, 0, 19)
			for i := 0; i < 10; i++ {
				i := i
				label := customCommandEditorSlotLabel(st, i)
				activeFill := float32(0)
				if i == selected {
					activeFill = 1
				}
				hoverFill := float32(0)
				if st.commandClicks[i].Hovered() {
					hoverFill = 1
				}
				focusFill := float32(0)
				if st.focus == customCommandEditorFocusSlots && i == selected {
					focusFill = 1
				}
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.commandClicks[i], label, activeFill, hoverFill, 0, focusFill, stripH)
				}))
				if i < 9 {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layoutSettingsNavSeparator(gtx)
					}))
				}
			}
			dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			innerClip.Pop()
			return dims
		})
	})
}

func customCommandEditorListMetrics(gtx layout.Context) (stripH, sepH, totalH int) {
	stripH = gtx.Dp(unit.Dp(30))
	if stripH < 1 {
		stripH = 1
	}
	sepH = gtx.Dp(unit.Dp(1))
	if sepH < 1 {
		sepH = 1
	}
	totalH = stripH*10 + sepH*9
	return stripH, sepH, totalH
}

func (ui *UI) layoutCustomCommandEditorFields(th *material.Theme, gtx layout.Context, st *customCommandEditorState) layout.Dimensions {
	rowLabel := func(label string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, label)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleDialogFontSize(9)
			lbl.Color = hintColor
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		}
	}
	editor := func(id, hint string, ed *widget.Editor, height unit.Dp, target customCommandEditorFocus) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			st.applyPendingEditorFocus(gtx, target, ed)
			materialEd := material.Editor(th, ed, hint)
			materialEd.Font.Typeface = ui.viewerTypeface()
			materialEd.TextSize = ui.scaleDialogFontSize(10)
			materialEd.Color = txtColor
			materialEd.HintColor = hintColor
			host := func(gtx layout.Context) layout.Dimensions {
				focused := gtx.Focused(ed) || st.focus == target
				return layoutNeutralEditorBox(gtx, focused, true, func(gtx layout.Context) layout.Dimensions {
					if height > 0 {
						return fixedHeight(gtx, gtx.Dp(height), materialEd.Layout)
					}
					return materialEd.Layout(gtx)
				})
			}
			return ui.layoutEditorWithContextMenu(th, gtx, id, ed, true, host)
		}
	}
	_, _, totalH := customCommandEditorListMetrics(gtx)
	return fixedHeight(gtx, totalH, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(rowLabel("Slot")),
			layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				text := fmt.Sprintf("%d / %s", st.selected+1, customCommandMenuShortcut(st.selected))
				lbl := material.Body2(th, text)
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.TextSize = ui.scaleDialogFontSize(10)
				lbl.Color = txtColor
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(rowLabel("Short name")),
			layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
			layout.Rigid(editor("custom-command-name", "gpstrack summary", &st.nameEdit, 0, customCommandEditorFocusName)),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(rowLabel("Command")),
			layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
				return editor("custom-command-body", "python - <<'PY'\nprint('hello')\nPY", &st.commandEdit, 0, customCommandEditorFocusCommand)(gtx)
			}),
		)
	})
}

func (ui *UI) startCustomCommandViewer(raw fm.CustomCommand, now time.Time) bool {
	cmd, ok := fm.NormalizeCustomCommand(raw)
	if !ok {
		if pane := ui.activePane(); pane != nil {
			pane.setNotice("custom command is empty", now)
		}
		return false
	}
	idx := ui.activeFilePane
	pane := ui.activePane()
	var remote *paneSSHSession
	if pane != nil && pane.remoteConnected() {
		remote = pane.remote.clone()
		if remote == nil {
			pane.setNotice("remote session is not connected", now)
			return false
		}
	}
	targetPath := ui.customCommandViewerTarget(pane, remote)
	st := &fileViewerState{
		pane:         idx,
		path:         targetPath,
		name:         cmd.Name,
		remote:       remote,
		status:       "loading...",
		fileEncoding: fm.ViewerFileEncodingAuto,
		wrapEnabled:  viewerWordWrap(ui.fmCfg),
		commandOnly:  true,
		resultCh:     make(chan fileViewerResult, 4),
		pdfDocCh:     make(chan pdfDocResult, 16),
	}
	st.mode = "command"
	st.command = cmd.Command
	st.autoRefresh = false
	st.commandInfinite = viewerCommandLooksInfinite(st.command)
	st.contentEditor.SingleLine = false
	st.contentEditor.ReadOnly = true
	st.contentEditor.Submit = false
	st.contentEditor.SetText("")
	st.stream.SetContent("")
	st.commandEditor.SingleLine = false
	st.commandEditor.Submit = false
	st.commandEditor.SetText(st.command)
	st.find.editor.SingleLine = true
	st.find.editor.Submit = false
	st.find.resultCh = make(chan fileViewerFindResult, 1)
	st.find.pdfResultCh = make(chan viewerPDFFindResult, 16)
	st.find.pdfList.Axis = layout.Vertical
	st.find.textList.Axis = layout.Vertical
	st.find.hexList.Axis = layout.Vertical
	st.find.index = -1
	st.wordSelectRE, st.wordSelectExpr = viewerWordSelectRegexp(ui.fmCfg)
	st.hex = newHexViewerState()

	ui.fileViewer = st
	ui.closeFunctionBarPopups()
	ui.rep.active = false
	ui.rep.pane = -1
	if pane != nil {
		ui.setActiveFilePane(idx)
		pane.stopPathEdit()
		pane.sortMenuOpen = false
		pane.closeFavoriteMenu()
		pane.closeContextMenu()
	}
	ui.closeSortMenusExcept(idx)
	ui.closeFavoriteMenusExcept(idx)
	ui.closeContextMenusExcept(idx)
	ui.startFileViewerLoad(now)
	return true
}

func (ui *UI) customCommandViewerTarget(pane *filePaneState, remote *paneSSHSession) string {
	if pane != nil {
		if entry := pane.selectedEntry(); entry != nil && strings.TrimSpace(entry.Path) != "" {
			return entry.Path
		}
		if dir := strings.TrimSpace(pane.dir); dir != "" {
			if remote != nil {
				return pathpkg.Clean(dir)
			}
			return filepath.Clean(dir)
		}
	}
	if remote != nil {
		return "."
	}
	if cwd, err := filepath.Abs("."); err == nil {
		return cwd
	}
	return "."
}
