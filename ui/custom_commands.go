// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"hexone/fm"
	uitheme "hexone/ui/theme"
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
	selected int

	nameEdit     widget.Editor
	shortcutEdit widget.Editor
	commandEdit  widget.Editor

	backdropClick widget.Clickable
	closeClick    widget.Clickable
	newClick      widget.Clickable
	deleteClick   widget.Clickable
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

type customCommandShortcutKey struct {
	name key.Name
	mods key.Modifiers
}

type customCommandEditorFocus uint8

const (
	customCommandEditorFocusNone customCommandEditorFocus = iota
	customCommandEditorFocusName
	customCommandEditorFocusShortcut
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
	commands := fm.NormalizeCustomCommands(ui.fmCfg.CustomCommands)
	for i, cmd := range commands {
		items = append(items, customCommandMenuSpec{
			key:      fmt.Sprintf("cmd:%d:%s", i, cmd.Name),
			label:    cmd.Name,
			shortcut: customCommandMenuShortcut(i, cmd.Shortcut),
			index:    i,
			command:  cmd,
		})
	}
	return items
}

func customCommandMenuShortcut(index int, configured string) string {
	if shortcut := strings.TrimSpace(configured); shortcut != "" {
		return shortcut
	}
	if index < 0 || index >= 10 {
		return ""
	}
	keyText := fmt.Sprintf("%d", index+1)
	if index == 9 {
		keyText = "0"
	}
	return "Ctrl+" + keyText
}

func parseCustomCommandShortcut(raw string) (customCommandShortcutKey, bool) {
	parts := strings.Split(strings.TrimSpace(raw), "+")
	if len(parts) == 0 {
		return customCommandShortcutKey{}, false
	}
	keyPart := strings.TrimSpace(parts[len(parts)-1])
	if keyPart == "" {
		return customCommandShortcutKey{}, false
	}
	var mods key.Modifiers
	for _, rawPart := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(rawPart)) {
		case "ctrl", "control":
			mods |= key.ModCtrl
		case "cmd", "command", "shortcut":
			mods |= key.ModShortcut
		case "alt", "option":
			mods |= key.ModAlt
		case "shift":
			mods |= key.ModShift
		case "":
		default:
			return customCommandShortcutKey{}, false
		}
	}
	return customCommandShortcutKey{name: key.Name(keyPart), mods: mods}, true
}

func customCommandShortcutNames(name key.Name) []key.Name {
	raw := string(name)
	if len(raw) == 1 {
		lower := key.Name(strings.ToLower(raw))
		upper := key.Name(strings.ToUpper(raw))
		if lower != upper {
			return []key.Name{lower, upper}
		}
	}
	return []key.Name{name}
}

func customCommandShortcutMatches(ke key.Event, raw string) bool {
	shortcut, ok := parseCustomCommandShortcut(raw)
	if !ok || ke.Modifiers != shortcut.mods {
		return false
	}
	eventName := strings.ToLower(string(ke.Name))
	wantName := strings.ToLower(string(shortcut.name))
	return eventName == wantName
}

func (ui *UI) customCommandShortcutKeyFilters(optional key.Modifiers) []event.Filter {
	if ui == nil || ui.fmCfg == nil {
		return nil
	}
	var filters []event.Filter
	for _, cmd := range fm.NormalizeCustomCommands(ui.fmCfg.CustomCommands) {
		shortcut, ok := parseCustomCommandShortcut(cmd.Shortcut)
		if !ok {
			continue
		}
		for _, name := range customCommandShortcutNames(shortcut.name) {
			filters = append(filters, key.Filter{Name: name, Required: shortcut.mods, Optional: optional})
		}
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
	if mods != 0 && mods != key.ModCtrl && mods != key.ModShortcut {
		return false
	}
	idx := -1
	switch name {
	case "1":
		idx = 0
	case "2":
		idx = 1
	case "3":
		idx = 2
	case "4":
		idx = 3
	case "5":
		idx = 4
	case "6":
		idx = 5
	case "7":
		idx = 6
	case "8":
		idx = 7
	case "9":
		idx = 8
	case "0":
		idx = 9
	}
	if idx < 0 {
		return false
	}
	items := ui.customCommandMenuSpecs()
	itemIndex := idx + 1
	if itemIndex >= len(items) {
		return false
	}
	ui.activateCustomCommandMenuItem(items[itemIndex], now)
	return true
}

func (ui *UI) activateCustomCommandConfiguredShortcut(ke key.Event, now time.Time) bool {
	if ui == nil || ui.fmCfg == nil || ke.State != key.Press {
		return false
	}
	for _, cmd := range fm.NormalizeCustomCommands(ui.fmCfg.CustomCommands) {
		if strings.TrimSpace(cmd.Shortcut) == "" || !customCommandShortcutMatches(ke, cmd.Shortcut) {
			continue
		}
		ui.closeCustomCommandMenu()
		return ui.startCustomCommandViewer(cmd, now)
	}
	return false
}

func (ui *UI) activateCustomCommandGlobalShortcut(ke key.Event, now time.Time) bool {
	if ui == nil || ui.Tabs.Value != "tab0" || ui.helpModal != nil || ui.settingsModal != nil || ui.sshModal != nil || ui.hasBlockingFileDialog() || ui.pathEditActive() || ui.fileViewer != nil {
		return false
	}
	return ui.activateCustomCommandConfiguredShortcut(ke, now)
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
							lbl.Font.Typeface = ui.mainTypeface()
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
		filters = append(filters, ui.customCommandShortcutKeyFilters(anyMods)...)
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
				handled = ui.activateCustomCommandConfiguredShortcut(ke, gtx.Now)
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
	st := &customCommandEditorState{
		draft:        cloneCustomCommands(ui.fmCfg.CustomCommands),
		selected:     -1,
		focus:        customCommandEditorFocusCommand,
		focusPending: customCommandEditorFocusCommand,
		actionFocus:  customCommandEditorActionSave,
	}
	st.nameEdit.SingleLine = true
	st.nameEdit.Submit = false
	st.shortcutEdit.SingleLine = true
	st.shortcutEdit.Submit = false
	st.commandEdit.SingleLine = false
	st.commandEdit.Submit = false
	st.commandEdit.SetText("")
	st.nameEdit.SetText("")
	st.shortcutEdit.SetText("")
	ui.customCommandEditor = st
	if index >= 0 && index < len(st.draft) {
		st.selectCommand(index)
	}
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
		customCommandEditorFocusName,
		customCommandEditorFocusShortcut,
		customCommandEditorFocusCommand,
		customCommandEditorFocusActions,
	}
}

func (st *customCommandEditorState) syncEditorFocus(gtx layout.Context) {
	if st == nil {
		return
	}
	switch {
	case gtx.Focused(&st.nameEdit):
		st.focus = customCommandEditorFocusName
	case gtx.Focused(&st.shortcutEdit):
		st.focus = customCommandEditorFocusShortcut
	case gtx.Focused(&st.commandEdit):
		st.focus = customCommandEditorFocusCommand
	case gtx.Focused(&st.keyFocus.tag):
		st.focus = customCommandEditorFocusActions
		if st.focusPending == customCommandEditorFocusActions {
			st.focusPending = customCommandEditorFocusNone
		}
	}
}

func (st *customCommandEditorState) setFocus(target customCommandEditorFocus) bool {
	if st == nil || target == customCommandEditorFocusNone {
		return false
	}
	changed := st.focus != target || st.focusPending != target
	st.focus = target
	st.focusPending = target
	if target == customCommandEditorFocusActions {
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

func (st *customCommandEditorState) actionVisualState(target customCommandEditorAction, canSaveRun bool) dialogActionVisualState {
	if st == nil {
		return dialogActionVisualState{}
	}
	if st.focus == customCommandEditorFocusActions {
		active := st.actionFocus == target
		return dialogActionVisualState{Focused: active, Default: active}
	}
	return dialogActionVisualState{Default: canSaveRun && target == customCommandEditorActionSave}
}

func (st *customCommandEditorState) clearFields() {
	if st == nil {
		return
	}
	st.selected = -1
	st.nameEdit.SetText("")
	st.shortcutEdit.SetText("")
	st.commandEdit.SetText("")
	st.setFocus(customCommandEditorFocusCommand)
	st.lastErr = ""
}

func (st *customCommandEditorState) selectCommand(index int) {
	if st == nil || index < 0 || index >= len(st.draft) {
		return
	}
	cmd := st.draft[index]
	st.selected = index
	st.nameEdit.SetText(cmd.Name)
	st.shortcutEdit.SetText(cmd.Shortcut)
	st.commandEdit.SetText(cmd.Command)
	st.commandEdit.SetCaret(st.commandEdit.Len(), st.commandEdit.Len())
	st.setFocus(customCommandEditorFocusCommand)
	st.lastErr = ""
}

func (st *customCommandEditorState) currentCommandFields() (fm.CustomCommand, error) {
	if st == nil {
		return fm.CustomCommand{}, fmt.Errorf("custom command editor is not open")
	}
	cmd, ok := fm.NormalizeCustomCommand(fm.CustomCommand{
		Name:     st.nameEdit.Text(),
		Shortcut: st.shortcutEdit.Text(),
		Command:  st.commandEdit.Text(),
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
	commands := fm.NormalizeCustomCommands(st.draft)
	idx := -1
	if st.selected >= 0 && st.selected < len(commands) {
		idx = st.selected
	}
	for i, existing := range commands {
		if strings.EqualFold(existing.Name, cmd.Name) {
			idx = i
			break
		}
	}
	if idx >= 0 {
		commands[idx] = cmd
	} else {
		if len(commands) >= 10 {
			return fm.CustomCommand{}, fmt.Errorf("custom command limit is 10")
		}
		commands = append(commands, cmd)
		idx = len(commands) - 1
	}
	st.draft = fm.NormalizeCustomCommands(commands)
	for i, existing := range st.draft {
		if strings.EqualFold(existing.Name, cmd.Name) {
			st.selected = i
			break
		}
	}
	return cmd, nil
}

func (st *customCommandEditorState) canUpsertCurrentCommand() bool {
	cmd, err := st.currentCommandFields()
	if err != nil {
		return false
	}
	commands := fm.NormalizeCustomCommands(st.draft)
	if st.selected >= 0 && st.selected < len(commands) {
		return true
	}
	for _, existing := range commands {
		if strings.EqualFold(existing.Name, cmd.Name) {
			return true
		}
	}
	return len(commands) < 10
}

func (ui *UI) saveCustomCommandEditorDraft() error {
	if ui == nil || ui.customCommandEditor == nil {
		return nil
	}
	if err := ui.ensureFMConfigLoaded(); err != nil {
		return err
	}
	ui.fmCfg.CustomCommands = fm.NormalizeCustomCommands(ui.customCommandEditor.draft)
	return ui.saveFMConfigWithOptions("custom-commands", false)
}

func (ui *UI) saveCurrentCustomCommand() bool {
	st := ui.customCommandEditor
	if st == nil {
		return false
	}
	if _, err := st.upsertCurrentCommand(); err != nil {
		st.lastErr = err.Error()
		return false
	}
	if err := ui.saveCustomCommandEditorDraft(); err != nil {
		st.lastErr = err.Error()
		return false
	}
	st.lastErr = "saved"
	return true
}

func (ui *UI) runCurrentCustomCommand(now time.Time) bool {
	st := ui.customCommandEditor
	if st == nil {
		return false
	}
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

func (ui *UI) deleteSelectedCustomCommand() bool {
	st := ui.customCommandEditor
	if st == nil || st.selected < 0 || st.selected >= len(st.draft) {
		return false
	}
	st.draft = append(st.draft[:st.selected], st.draft[st.selected+1:]...)
	st.draft = fm.NormalizeCustomCommands(st.draft)
	st.clearFields()
	if err := ui.saveCustomCommandEditorDraft(); err != nil {
		st.lastErr = err.Error()
		return false
	}
	st.lastErr = "deleted"
	return true
}

func (ui *UI) handleCustomCommandEditorKeys(gtx layout.Context, st *customCommandEditorState) bool {
	if ui == nil || st == nil {
		return false
	}
	anyMods := ^key.Modifiers(0)
	for {
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
		if st.focus == customCommandEditorFocusActions {
			filters = append(filters,
				key.Filter{Name: key.NameEnter, Optional: anyMods},
				key.Filter{Name: key.NameReturn, Optional: anyMods},
				key.Filter{Name: key.NameLeftArrow, Optional: anyMods},
				key.Filter{Name: key.NameRightArrow, Optional: anyMods},
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

func (ui *UI) layoutCustomCommandEditor(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.customCommandEditor
	if st == nil {
		return layout.Dimensions{}
	}
	st.keyFocus.attach(gtx)
	st.syncEditorFocus(gtx)
	if ui.handleCustomCommandEditorKeys(gtx, st) {
		return layout.Dimensions{}
	}
	for _, ed := range []*widget.Editor{&st.nameEdit, &st.shortcutEdit, &st.commandEdit} {
		for {
			ev, ok := ed.Update(gtx)
			if !ok {
				break
			}
			if _, ok := ev.(widget.ChangeEvent); ok {
				st.lastErr = ""
			}
		}
	}

	st.ensureCommandClicks(len(st.draft))
	for i := range st.draft {
		i := i
		for st.commandClicks[i].Clicked(gtx) {
			st.selectCommand(i)
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	if st.newClick.Clicked(gtx) {
		st.clearFields()
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.deleteClick.Clicked(gtx) {
		st.actionsAnim.setPulse("delete", gtx.Now)
		ui.deleteSelectedCustomCommand()
		gtx.Execute(op.InvalidateCmd{})
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
	case st.deleteClick.Hovered():
		hoverActionKey = "delete"
	case st.cancelClick.Hovered():
		hoverActionKey = "cancel"
	case st.saveClick.Hovered():
		hoverActionKey = "save"
	case st.runClick.Hovered():
		hoverActionKey = "run"
	}
	st.actionsAnim.setHover(hoverActionKey, gtx.Now)
	hoverDelete, hoverAnimDelete := st.actionsAnim.hoverFill(gtx.Now, "delete")
	hoverCancel, hoverAnimCancel := st.actionsAnim.hoverFill(gtx.Now, "cancel")
	hoverSave, hoverAnimSave := st.actionsAnim.hoverFill(gtx.Now, "save")
	hoverRun, hoverAnimRun := st.actionsAnim.hoverFill(gtx.Now, "run")
	pulseDelete, pulseAnimDelete := st.actionsAnim.pulseFill(gtx.Now, "delete")
	pulseCancel, pulseAnimCancel := st.actionsAnim.pulseFill(gtx.Now, "cancel")
	pulseSave, pulseAnimSave := st.actionsAnim.pulseFill(gtx.Now, "save")
	pulseRun, pulseAnimRun := st.actionsAnim.pulseFill(gtx.Now, "run")
	if hoverAnimDelete || hoverAnimCancel || hoverAnimSave || hoverAnimRun || pulseAnimDelete || pulseAnimCancel || pulseAnimSave || pulseAnimRun {
		gtx.Execute(op.InvalidateCmd{})
	}

	canDelete := st.selected >= 0 && st.selected < len(st.draft)
	canSaveRun := st.canUpsertCurrentCommand()

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					title := material.Body1(th, "Custom Command")
					title.Font.Typeface = ui.mainTypeface()
					title.Font.Weight = font.Bold
					title.TextSize = scaleDialogThemeFontSize(th, 12)
					title.Color = txtColor
					return title.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutTinyIconModeButton(th, gtx, &st.closeClick, uitheme.CloseIcon(), false)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			leftW := gtx.Dp(unit.Dp(190))
			if gtx.Constraints.Max.X < gtx.Dp(unit.Dp(520)) {
				leftW = 0
			}
			if leftW <= 0 {
				return ui.layoutCustomCommandEditorFields(th, gtx, st)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, leftW, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutCustomCommandEditorList(th, gtx, st, canDelete, hoverDelete, pulseDelete)
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutCustomCommandEditorFields(th, gtx, st)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.lastErr == "" {
				return layout.Dimensions{}
			}
			col := hintColor
			if st.lastErr != "saved" && st.lastErr != "deleted" {
				col = color.NRGBA{R: 220, G: 140, B: 140, A: 255}
			}
			lbl := material.Caption(th, st.lastErr)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleDialogThemeFontSize(th, 9)
			lbl.Color = col
			lbl.MaxLines = 2
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutDialogActionTriple(
					th, gtx,
					&st.cancelClick, "Cancel", hoverCancel, pulseCancel, false,
					&st.saveClick, "Save", hoverSave, pulseSave, !canSaveRun,
					&st.runClick, "Run", hoverRun, pulseRun, !canSaveRun,
					st.actionVisualState(customCommandEditorActionCancel, canSaveRun),
					st.actionVisualState(customCommandEditorActionSave, canSaveRun),
					st.actionVisualState(customCommandEditorActionRun, canSaveRun),
				)
			})
		}),
	)
}

func (ui *UI) layoutCustomCommandEditorList(th *material.Theme, gtx layout.Context, st *customCommandEditorState, canDelete bool, hoverDelete, pulseDelete float32) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, fmt.Sprintf("%d/10 saved", len(st.draft)))
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleDialogThemeFontSize(th, 9)
					lbl.Color = hintColor
					lbl.MaxLines = 1
					return lbl.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutDialogActionSingle(th, gtx, &st.deleteClick, "Delete", hoverDelete, pulseDelete, !canDelete, dialogActionVisualState{})
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return st.newClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutCustomCommandEditorListRow(th, gtx, "New command", "", st.selected < 0, st.newClick.Hovered())
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(st.draft)*2)
			for i, cmd := range st.draft {
				i := i
				cmd := cmd
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					click := &st.commandClicks[i]
					return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutCustomCommandEditorListRow(th, gtx, cmd.Name, cmd.Shortcut, st.selected == i, click.Hovered())
					})
				}))
				if i < len(st.draft)-1 {
					children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout))
				}
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}),
	)
}

func (ui *UI) layoutCustomCommandEditorListRow(th *material.Theme, gtx layout.Context, name, shortcut string, selected, hovered bool) layout.Dimensions {
	theme := ui.filePanePopupTheme()
	bg := theme.Bg
	border := theme.Border
	fg := theme.Text
	detail := theme.Muted
	if hovered {
		bg = theme.HoverBg
		fg = theme.HoverText
	}
	if selected {
		bg = theme.ActiveBg
		fg = theme.ActiveText
		detail = mixNRGBA(theme.ActiveText, theme.ActiveBg, 0.48)
	}
	return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), bg, border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, name)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleDialogThemeFontSize(th, 10)
					lbl.Font.Weight = font.Medium
					lbl.Color = fg
					lbl.MaxLines = 1
					lbl.Truncator = "..."
					return lbl.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if strings.TrimSpace(shortcut) == "" {
						return layout.Dimensions{}
					}
					lbl := material.Caption(th, shortcut)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleDialogThemeFontSize(th, 8)
					lbl.Color = detail
					lbl.MaxLines = 1
					return lbl.Layout(gtx)
				}),
			)
		})
	})
}

func (ui *UI) layoutCustomCommandEditorFields(th *material.Theme, gtx layout.Context, st *customCommandEditorState) layout.Dimensions {
	rowLabel := func(label string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, label)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleDialogThemeFontSize(th, 9)
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
			materialEd.TextSize = scaleDialogThemeFontSize(th, 10)
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
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(rowLabel("Short name")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(editor("custom-command-name", "gpstrack summary", &st.nameEdit, 0, customCommandEditorFocusName)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(rowLabel("Shortcut")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(150)), editor("custom-command-shortcut", "Ctrl+1", &st.shortcutEdit, 0, customCommandEditorFocusShortcut))
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(rowLabel("Command")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(editor("custom-command-body", "python - <<'PY'\nprint('hello')\nPY", &st.commandEdit, 230, customCommandEditorFocusCommand)),
	)
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
		pane:            idx,
		path:            targetPath,
		name:            cmd.Name,
		remote:          remote,
		status:          "loading...",
		fileEncoding:    fm.ViewerFileEncodingAuto,
		wrapEnabled:     viewerWordWrap(ui.fmCfg),
		commandOnly:     true,
		resultCh:        make(chan fileViewerResult, 4),
		previewRenderCh: make(chan fileViewerPreviewRenderResult, 2),
		previewCacheCh:  make(chan fileViewerPDFCacheResult, 4),
		pdfPageCache:    make(map[int]viewerPDFRenderResult, 3),
		pdfPreloadPages: make(map[int]struct{}, 2),
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
