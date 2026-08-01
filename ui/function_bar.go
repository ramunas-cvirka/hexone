// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"image/color"
	"runtime"
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
	functionBarTopInsetDp   = 0
	functionBarOuterInsetDp = 0
	functionBarStripDp      = 22
	functionBarPopupGapDp   = 4
)

type functionBarAction uint8

const (
	functionBarActionNone functionBarAction = iota
	functionBarActionHelp
	functionBarActionCustom
	functionBarActionView
	functionBarActionOpen
	functionBarActionCopy
	functionBarActionMove
	functionBarActionCreate
	functionBarActionDelete
	functionBarActionTools
	functionBarActionExit
	functionBarActionViewerSave
	functionBarActionViewerFind
	functionBarActionViewerMode
	functionBarActionViewerWrap
	functionBarActionViewerLineNumbers
)

type functionBarButtonSpec struct {
	action     functionBarAction
	keyLabel   string
	label      string
	click      *widget.Clickable
	activeFill float32
	enabled    bool
}

type functionBarToolSpec struct {
	key      string
	label    string
	shortcut string
	active   bool
}

type functionBarHintSpec struct {
	shortcut string
	label    string
}

func registerPopupArea(gtx layout.Context, tag event.Tag, size image.Point) {
	if tag == nil || size.X <= 0 || size.Y <= 0 {
		return
	}
	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, tag)
	pass.Pop()
}

func popupPressed(gtx layout.Context, tag event.Tag) bool {
	if tag == nil {
		return false
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: tag,
			Kinds:  pointer.Press,
		})
		if !ok {
			return false
		}
		pe, ok := ev.(pointer.Event)
		if ok && pe.Kind == pointer.Press && pe.Buttons.Contain(pointer.ButtonPrimary) {
			return true
		}
	}
}

func (ui *UI) hasBlockingFileDialog() bool {
	return ui != nil && (ui.fileCopyBlocksUI() || ui.fileDelete != nil || ui.fileMove != nil || ui.fileCreate != nil || ui.filePerm != nil || ui.multiRename != nil || ui.customCommandEditor != nil || ui.terminalSnippetEditor != nil || ui.archiveExtractConflictOpen())
}

func (ui *UI) fileCopyBlocksUI() bool {
	return ui != nil && ui.fileCopy != nil && !ui.fileCopy.directPaste
}

func (ui *UI) closeFunctionBarToolsMenu() {
	if ui == nil {
		return
	}
	ui.functionBarToolsOpen = false
	ui.functionBarToolsRect = image.Rectangle{}
	ui.functionBarToolsOpenedAt = time.Time{}
	ui.functionBarToolsHoverID = ""
	ui.functionBarToolsHoverAnim = segmentedAnimState{}
	ui.functionBarToolsSelected = -1
}

func (ui *UI) closeCustomCommandMenu() {
	if ui == nil {
		return
	}
	ui.customCommandMenuOpen = false
	ui.customCommandMenuRect = image.Rectangle{}
	ui.customCommandMenuOpenedAt = time.Time{}
	ui.customCommandMenuHoverID = ""
	ui.customCommandMenuHoverAnim = segmentedAnimState{}
	ui.customCommandMenuSelected = -1
}

func (ui *UI) closeFunctionBarPopups() {
	if ui == nil {
		return
	}
	ui.closeCustomCommandMenu()
	ui.closeFunctionBarToolsMenu()
}

func (ui *UI) requestWindowClose() {
	if ui == nil {
		return
	}
	ui.requestedWindowClose = true
}

func (ui *UI) ConsumeWindowCloseRequest() bool {
	if ui == nil || !ui.requestedWindowClose {
		return false
	}
	ui.requestedWindowClose = false
	return true
}

func (ui *UI) functionBarAutoHiddenForViewer() bool {
	return ui != nil &&
		ui.fileViewer != nil &&
		ui.fmCfg != nil &&
		ui.fmCfg.Viewer.HideFunctionBarWhenOpen
}

func (ui *UI) functionBarAutoHiddenForTerminal() bool {
	return ui != nil && ui.terminalMaximized()
}

func (ui *UI) functionBarVisible() bool {
	if ui == nil {
		return false
	}
	if ui.functionBarAutoHiddenForViewer() {
		return ui.functionBarViewerShown
	}
	if ui.functionBarAutoHiddenForTerminal() {
		return ui.functionBarTerminalShown
	}
	return !ui.functionBarHidden
}

func (ui *UI) toggleFunctionBarVisibility(now time.Time) bool {
	if ui == nil {
		return false
	}
	if ui.functionBarAutoHiddenForViewer() {
		ui.functionBarViewerShown = !ui.functionBarViewerShown
	} else if ui.functionBarAutoHiddenForTerminal() {
		ui.functionBarTerminalShown = !ui.functionBarTerminalShown
	} else {
		ui.functionBarHidden = !ui.functionBarHidden
	}
	ui.closeFunctionBarPopups()
	ui.setToolbarHover("", now)
	if !ui.functionBarVisible() && ui.Tabs.Value == "tab0" {
		if pane := ui.activePane(); pane != nil {
			pane.setNotice("function bar hidden; press F11 to show it again", now)
		}
	}
	return true
}

func (ui *UI) functionBarActionEnabled(action functionBarAction) bool {
	if ui == nil {
		return false
	}
	switch action {
	case functionBarActionExit:
		return ui.helpModal == nil && ui.settingsModal == nil && ui.sshModal == nil && !ui.hasBlockingFileDialog()
	case functionBarActionHelp:
		return ui.helpModal == nil && ui.settingsModal == nil && ui.sshModal == nil && !ui.hasBlockingFileDialog()
	case functionBarActionTools:
		return ui.helpModal == nil && ui.settingsModal == nil && ui.sshModal == nil && !ui.hasBlockingFileDialog()
	case functionBarActionViewerSave,
		functionBarActionViewerFind,
		functionBarActionViewerMode,
		functionBarActionViewerWrap,
		functionBarActionViewerLineNumbers:
		return ui.viewerFunctionBarActionEnabled(action)
	}

	if ui.Tabs.Value != "tab0" || ui.helpModal != nil || ui.settingsModal != nil || ui.sshModal != nil || ui.pathEditActive() {
		return false
	}

	switch action {
	case functionBarActionCustom:
		return !ui.hasBlockingFileDialog()
	case functionBarActionView:
		if ui.fileViewer != nil {
			return true
		}
		return !ui.hasBlockingFileDialog()
	case functionBarActionOpen:
		if ui.fileViewer != nil {
			ok, _ := ui.fileViewerCanEdit(ui.fileViewer)
			return ok
		}
		return !ui.hasBlockingFileDialog()
	case functionBarActionCopy, functionBarActionMove, functionBarActionCreate, functionBarActionDelete:
		return ui.fileViewer == nil && ui.fileCopy == nil && !ui.hasBlockingFileDialog()
	default:
		return false
	}
}

func (ui *UI) viewerFunctionBarActionEnabled(action functionBarAction) bool {
	if ui == nil || ui.fileViewer == nil || ui.Tabs.Value != "tab0" ||
		ui.helpModal != nil || ui.settingsModal != nil || ui.sshModal != nil ||
		ui.hasBlockingFileDialog() || ui.fileViewer.modeSwitchPrompt.open {
		return false
	}
	st := ui.fileViewer
	switch action {
	case functionBarActionViewerSave:
		return st.editMode && st.editDirty && !st.saving
	case functionBarActionViewerFind:
		return !st.commandEditOn && viewerSupportsFind(st)
	case functionBarActionViewerMode:
		return !st.commandOnly
	case functionBarActionViewerWrap:
		return !st.detectedImagePreview && st.mode != "hex" && !st.commandEditOn
	case functionBarActionViewerLineNumbers:
		return !st.detectedImagePreview && st.mode != "hex" && !st.commandEditOn
	default:
		return false
	}
}

func (ui *UI) performFunctionBarAction(action functionBarAction, now time.Time) bool {
	return ui.performFunctionBarActionContext(layout.Context{Now: now}, action)
}

func (ui *UI) performFunctionBarActionContext(gtx layout.Context, action functionBarAction) bool {
	if ui == nil {
		return false
	}
	now := gtx.Now
	switch action {
	case functionBarActionHelp:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		ui.closeFunctionBarPopups()
		ui.openHelpModal()
		return true
	case functionBarActionCustom:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		if ui.fileViewer != nil {
			ui.closeFunctionBarPopups()
			ui.startFileViewerSave(now)
			return true
		}
		if ui.customCommandMenuOpen {
			ui.closeCustomCommandMenu()
		} else {
			ui.closeFunctionBarToolsMenu()
			items := ui.customCommandMenuSpecs()
			ui.customCommandMenuOpen = true
			ui.customCommandMenuOpenedAt = now
			ui.customCommandMenuHoverID = ""
			ui.customCommandMenuHoverAnim = segmentedAnimState{}
			ui.customCommandMenuSelected = customCommandMenuDefaultIndex(items)
		}
		return true
	case functionBarActionView:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		ui.closeFunctionBarPopups()
		if ui.fileViewer != nil {
			if ui.fileViewer.editMode {
				ui.discardFileViewerChanges(ui.fileViewer)
				ui.stopFileViewerEdit()
				return true
			}
			ui.closeFileViewer()
			return true
		}
		ui.startFileViewer(ui.activeFilePane, now)
		return true
	case functionBarActionOpen:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		ui.closeFunctionBarPopups()
		if ui.fileViewer != nil {
			ui.startFileViewerEdit(now)
			return true
		}
		ui.startFileExternalOpenAction(ui.activeFilePane, now)
		return true
	case functionBarActionCopy:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		ui.closeFunctionBarPopups()
		ui.startFileCopyDialog(ui.activeFilePane, now)
		return true
	case functionBarActionMove:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		ui.closeFunctionBarPopups()
		ui.startFileMoveDialog(ui.activeFilePane, now)
		return true
	case functionBarActionCreate:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		ui.closeFunctionBarPopups()
		ui.startFileCreateDialog(ui.activeFilePane, now)
		return true
	case functionBarActionDelete:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		ui.closeFunctionBarPopups()
		ui.startFileDeleteDialog(ui.activeFilePane, now)
		return true
	case functionBarActionTools:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		if ui.functionBarToolsOpen {
			ui.closeFunctionBarToolsMenu()
		} else {
			ui.closeCustomCommandMenu()
			items := ui.functionBarToolSpecs()
			ui.functionBarToolsOpen = true
			ui.functionBarToolsOpenedAt = now
			ui.functionBarToolsHoverID = ""
			ui.functionBarToolsHoverAnim = segmentedAnimState{}
			ui.functionBarToolsSelected = functionBarDefaultToolIndex(items)
		}
		return true
	case functionBarActionExit:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		ui.closeFunctionBarPopups()
		if st := ui.fileViewer; st != nil {
			ui.discardFileViewerChanges(st)
			ui.stopFileViewerEdit()
		}
		ui.requestWindowClose()
		return true
	case functionBarActionViewerSave:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		return ui.startFileViewerSave(now)
	case functionBarActionViewerFind:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		ui.openFileViewerFind(now)
		return true
	case functionBarActionViewerMode:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		next := "hex"
		if ui.fileViewer.mode == "hex" {
			next = "file"
		}
		ui.setFileViewerMode(next, now)
		return true
	case functionBarActionViewerWrap:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		ui.toggleViewerWordWrap()
		return true
	case functionBarActionViewerLineNumbers:
		if !ui.functionBarActionEnabled(action) {
			return false
		}
		ui.toggleViewerLineNumbers()
		return true
	}
	return false
}

func (ui *UI) functionBarButtonSpecs() []functionBarButtonSpec {
	if ui != nil && ui.fileViewer != nil {
		return ui.viewerFunctionBarButtonSpecs()
	}
	customLabel := "Custom"
	viewLabel := "View"
	openLabel := "Open"
	customFill := ui.customCommandMenuFill()
	openFill := float32(0)
	return []functionBarButtonSpec{
		{action: functionBarActionHelp, keyLabel: "F1", label: "Help", click: &ui.functionBarClicks[0], activeFill: 0, enabled: ui.functionBarActionEnabled(functionBarActionHelp)},
		{action: functionBarActionCustom, keyLabel: "F2", label: customLabel, click: &ui.functionBarClicks[1], activeFill: customFill, enabled: ui.functionBarActionEnabled(functionBarActionCustom)},
		{action: functionBarActionView, keyLabel: "F3", label: viewLabel, click: &ui.functionBarClicks[2], activeFill: boolFill(ui.fileViewer != nil), enabled: ui.functionBarActionEnabled(functionBarActionView)},
		{action: functionBarActionOpen, keyLabel: "F4", label: openLabel, click: &ui.functionBarClicks[3], activeFill: openFill, enabled: ui.functionBarActionEnabled(functionBarActionOpen)},
		{action: functionBarActionCopy, keyLabel: "F5", label: "Copy", click: &ui.functionBarClicks[4], activeFill: boolFill(ui.fileCopy != nil), enabled: ui.functionBarActionEnabled(functionBarActionCopy)},
		{action: functionBarActionMove, keyLabel: "F6", label: "Move", click: &ui.functionBarClicks[5], activeFill: boolFill(ui.fileMove != nil), enabled: ui.functionBarActionEnabled(functionBarActionMove)},
		{action: functionBarActionCreate, keyLabel: "F7", label: "New", click: &ui.functionBarClicks[6], activeFill: boolFill(ui.fileCreate != nil), enabled: ui.functionBarActionEnabled(functionBarActionCreate)},
		{action: functionBarActionDelete, keyLabel: "F8", label: "Delete", click: &ui.functionBarClicks[7], activeFill: boolFill(ui.fileDelete != nil), enabled: ui.functionBarActionEnabled(functionBarActionDelete)},
		{action: functionBarActionTools, keyLabel: "F9", label: "Tools", click: &ui.functionBarClicks[8], activeFill: ui.functionBarToolsFill(), enabled: ui.functionBarActionEnabled(functionBarActionTools)},
		{action: functionBarActionExit, keyLabel: "F10", label: "Exit", click: &ui.functionBarClicks[9], activeFill: 0, enabled: ui.functionBarActionEnabled(functionBarActionExit)},
	}
}

func (ui *UI) viewerFunctionBarButtonSpecs() []functionBarButtonSpec {
	st := ui.fileViewer
	viewLabel := "Close"
	if st.editMode {
		viewLabel = "View"
	}
	modeLabel := "Hex"
	if st.mode == "hex" {
		modeLabel = "Text"
	}
	wrapLabel := "Wrap"
	if st.wrapEnabled {
		wrapLabel = "Unwrap"
	}
	return []functionBarButtonSpec{
		{action: functionBarActionHelp, keyLabel: "F1", label: "Help", click: &ui.functionBarClicks[0], enabled: ui.functionBarActionEnabled(functionBarActionHelp)},
		{action: functionBarActionViewerSave, keyLabel: "F2", label: "Save", click: &ui.functionBarClicks[1], enabled: ui.functionBarActionEnabled(functionBarActionViewerSave)},
		{action: functionBarActionView, keyLabel: "F3", label: viewLabel, click: &ui.functionBarClicks[2], enabled: ui.functionBarActionEnabled(functionBarActionView)},
		{action: functionBarActionOpen, keyLabel: "F4", label: "Edit", click: &ui.functionBarClicks[3], enabled: ui.functionBarActionEnabled(functionBarActionOpen)},
		{action: functionBarActionViewerLineNumbers, keyLabel: "F5", label: "Lines", click: &ui.functionBarClicks[4], enabled: ui.functionBarActionEnabled(functionBarActionViewerLineNumbers)},
		{action: functionBarActionNone, keyLabel: "F6", label: "", click: &ui.functionBarClicks[5], enabled: false},
		{action: functionBarActionViewerFind, keyLabel: "F7", label: "Find", click: &ui.functionBarClicks[6], enabled: ui.functionBarActionEnabled(functionBarActionViewerFind)},
		{action: functionBarActionViewerMode, keyLabel: "F8", label: modeLabel, click: &ui.functionBarClicks[7], enabled: ui.functionBarActionEnabled(functionBarActionViewerMode)},
		{action: functionBarActionViewerWrap, keyLabel: "F9", label: wrapLabel, click: &ui.functionBarClicks[8], enabled: ui.functionBarActionEnabled(functionBarActionViewerWrap)},
		{action: functionBarActionExit, keyLabel: "F10", label: "Exit", click: &ui.functionBarClicks[9], enabled: ui.functionBarActionEnabled(functionBarActionExit)},
	}
}

func boolFill(v bool) float32 {
	if v {
		return 1
	}
	return 0
}

func (ui *UI) functionBarToolsFill() float32 {
	if ui == nil {
		return 0
	}
	if ui.functionBarToolsOpen {
		return 1
	}
	if ui.settingsModal != nil || ui.sshModal != nil || ui.Tabs.Value == "tab1" || ui.Tabs.Value == "tab2" || ui.Tabs.Value == "tab3" {
		return 0.7
	}
	return 0
}

func (ui *UI) customCommandMenuFill() float32 {
	if ui == nil {
		return 0
	}
	if ui.customCommandMenuOpen || ui.customCommandEditor != nil {
		return 1
	}
	return 0
}

func (ui *UI) functionBarTextSize() unit.Sp {
	if ui == nil {
		return 11
	}
	return ui.scaleInterfaceFontSize(11)
}

func functionBarKeyTextColor(labelColor color.NRGBA) color.NRGBA {
	return color.NRGBA{R: 92, G: 214, B: 255, A: labelColor.A}
}

func (ui *UI) functionBarLabelStyle(th *material.Theme, label string, fg color.NRGBA, weight font.Weight) material.LabelStyle {
	lbl := material.Body2(th, label)
	lbl.Font.Typeface = ui.interfaceTypeface()
	lbl.Font.Weight = weight
	lbl.TextSize = ui.functionBarTextSize()
	lbl.Color = fg
	lbl.MaxLines = 1
	lbl.Truncator = "…"
	return lbl
}

func (ui *UI) functionBarShortcutLabelStyle(th *material.Theme, label string, labelColor color.NRGBA) material.LabelStyle {
	return ui.functionBarLabelStyle(th, label, functionBarKeyTextColor(labelColor), font.Bold)
}

func (ui *UI) functionBarActionLabelStyle(th *material.Theme, label string, labelColor color.NRGBA) material.LabelStyle {
	return ui.functionBarLabelStyle(th, label, labelColor, font.Normal)
}

func (ui *UI) functionBarSplitLabelStyles(th *material.Theme, shortcut, action string, labelColor color.NRGBA) (material.LabelStyle, material.LabelStyle, material.LabelStyle) {
	shortcutLabel := ui.functionBarShortcutLabelStyle(th, strings.TrimSpace(shortcut), labelColor)
	actionLabel := ui.functionBarActionLabelStyle(th, strings.TrimSpace(action), labelColor)
	spaceLabel := ui.functionBarActionLabelStyle(th, " ", labelColor)
	return shortcutLabel, spaceLabel, actionLabel
}

func (ui *UI) layoutFunctionBarSplitLabel(th *material.Theme, gtx layout.Context, shortcut, action string, labelColor color.NRGBA) layout.Dimensions {
	shortcutLabel, spaceLabel, actionLabel := ui.functionBarSplitLabelStyles(th, shortcut, action, labelColor)

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(
			gtx,
			layout.Rigid(shortcutLabel.Layout),
			layout.Rigid(spaceLabel.Layout),
			layout.Rigid(actionLabel.Layout),
		)
	})
}

func (ui *UI) setFunctionBarHeldModifier(mod key.Modifiers, down bool) bool {
	if ui == nil || mod == 0 {
		return false
	}
	next := ui.functionBarHeldMods
	if down {
		next |= mod
	} else {
		next &^= mod
	}
	if next == ui.functionBarHeldMods {
		return false
	}
	ui.functionBarHeldMods = next
	return true
}

func (ui *UI) SyncPlatformAltHeld(down bool) bool {
	if ui == nil {
		return false
	}
	return ui.setFunctionBarHeldModifier(key.ModAlt, down)
}

func (ui *UI) handleFunctionBarModifierKeys(gtx layout.Context) {
	if ui == nil {
		return
	}
	anyMods := ^key.Modifiers(0)
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameCtrl, Optional: anyMods},
			key.Filter{Name: key.NameCommand, Optional: anyMods},
			key.Filter{Name: key.NameAlt, Optional: anyMods},
			key.Filter{Name: key.NameShift, Optional: anyMods},
		)
		if !ok {
			return
		}
		ke, ok := ev.(key.Event)
		if !ok {
			continue
		}
		var mod key.Modifiers
		switch ke.Name {
		case key.NameCtrl:
			mod = key.ModCtrl
		case key.NameCommand:
			mod = key.ModCommand
		case key.NameAlt:
			mod = key.ModAlt
		case key.NameShift:
			mod = key.ModShift
		default:
			continue
		}
		var changed bool
		switch ke.State {
		case key.Press:
			changed = ui.setFunctionBarHeldModifier(mod, true)
		case key.Release:
			changed = ui.setFunctionBarHeldModifier(mod, false)
		}
		if changed {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (ui *UI) functionBarHeldHintModifier() key.Modifiers {
	if ui == nil {
		return 0
	}
	switch {
	case ui.functionBarHeldMods.Contain(key.ModCommand):
		return key.ModCommand
	case ui.functionBarHeldMods.Contain(key.ModCtrl):
		return key.ModCtrl
	case ui.functionBarHeldMods.Contain(key.ModAlt):
		return key.ModAlt
	default:
		return 0
	}
}

func (ui *UI) functionBarShortcutName() string {
	switch ui.functionBarHeldHintModifier() {
	case key.ModCommand:
		return "Cmd"
	case key.ModCtrl:
		return "Ctrl"
	case key.ModAlt:
		return "Alt"
	default:
		return ""
	}
}

func (ui *UI) functionBarShortcutLabel(keys string) string {
	name := ui.functionBarShortcutName()
	keys = strings.TrimSpace(keys)
	if name == "" || keys == "" {
		return ""
	}
	sep := "+"
	if strings.HasPrefix(keys, "+") || strings.HasPrefix(keys, "-") {
		sep = ""
	}
	return name + sep + keys
}

func (ui *UI) functionBarModifierHintSpecs() []functionBarHintSpec {
	return ui.functionBarModifierHintSpecsForContext(false, runtime.GOOS)
}

func (ui *UI) functionBarModifierHintSpecsForContext(terminalFocused bool, goos string) []functionBarHintSpec {
	mod := ui.functionBarHeldHintModifier()
	if ui == nil || mod == 0 || ui.customCommandMenuOpen || ui.functionBarToolsOpen {
		return nil
	}
	if ui.helpModal != nil || ui.settingsModal != nil || ui.sshModal != nil || ui.hasBlockingFileDialog() {
		return nil
	}

	hints := make([]functionBarHintSpec, 0, 4)
	add := func(keys, label string) {
		shortcut := ui.functionBarShortcutLabel(keys)
		label = strings.TrimSpace(label)
		if shortcut == "" || label == "" {
			return
		}
		hints = append(hints, functionBarHintSpec{
			shortcut: shortcut,
			label:    label,
		})
	}

	if terminalFocused {
		if mod != key.ModCtrl && mod != key.ModCommand {
			return nil
		}
		add("A", "Select All")
		switch {
		case goos == "darwin" && mod == key.ModCommand:
			add("K", "Clear")
		case goos != "darwin" && mod == key.ModCtrl:
			add("Shift+K", "Clear")
		}
		add("N", "New Tab")
		add("X", "Close Tab")
		add("S", "Settings")
		return hints
	}

	if mod == key.ModAlt {
		if ui.fileViewer != nil {
			return nil
		}
		if ui.Tabs.Value == "tab0" && !ui.pathEditActive() {
			add("1", "Left Drive")
			add("2", "Right Drive")
		}
		return hints
	}

	if ui.fileViewer != nil {
		st := ui.fileViewer
		if st != nil && !st.commandEditOn && !st.historyOpen {
			if st.detectedImagePreview {
				add("+/-", "Zoom")
			} else {
				if viewerSupportsFind(st) {
					add("F", "Find")
				}
				add("C", "Copy")
				add("A", "Select All")
			}
		}
		if !ui.pathEditActive() {
			add("S", "Save")
		}
		return hints
	}

	if ui.Tabs.Value == "tab0" {
		if !ui.pathEditActive() {
			add("A", "Select All")
			add("E", "Same Ext")
			add("N", "New Tab")
			add("X", "Close Tab")
			add("F", "SSH")
			add("M", "Multi-Rename")
			add("S", "Settings")
		}
		return hints
	}

	if !ui.pathEditActive() {
		add("S", "Settings")
	}
	return hints
}

func (ui *UI) functionBarModifierHintText() (string, bool) {
	return ui.functionBarModifierHintTextForContext(false, runtime.GOOS)
}

func (ui *UI) functionBarModifierHintTextForContext(terminalFocused bool, goos string) (string, bool) {
	hints := ui.functionBarModifierHintSpecsForContext(terminalFocused, goos)
	if len(hints) == 0 {
		return "", false
	}
	parts := make([]string, 0, len(hints))
	for _, hint := range hints {
		text := strings.TrimSpace(strings.TrimSpace(hint.shortcut) + " " + strings.TrimSpace(hint.label))
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, " | "), true
}

func (ui *UI) functionBarHintLabel(hint functionBarHintSpec) string {
	switch {
	case strings.TrimSpace(hint.shortcut) == "":
		return strings.TrimSpace(hint.label)
	case strings.TrimSpace(hint.label) == "":
		return strings.TrimSpace(hint.shortcut)
	default:
		return strings.TrimSpace(hint.shortcut) + " " + strings.TrimSpace(hint.label)
	}
}

func (ui *UI) functionBarHintSlotLabels(slotCount int) []string {
	return ui.functionBarHintSlotLabelsForSpecs(ui.functionBarModifierHintSpecs(), slotCount)
}

func (ui *UI) functionBarHintSlotLabelsForSpecs(hints []functionBarHintSpec, slotCount int) []string {
	if slotCount < 0 {
		slotCount = 0
	}
	labels := make([]string, slotCount)
	if len(hints) > slotCount {
		hints = hints[:slotCount]
	}
	for i, hint := range hints {
		labels[i] = ui.functionBarHintLabel(hint)
	}
	return labels
}

func functionBarHintSlots(hints []functionBarHintSpec, slotCount int) []functionBarHintSpec {
	if slotCount < 0 {
		slotCount = 0
	}
	slots := make([]functionBarHintSpec, slotCount)
	copy(slots, hints)
	return slots
}

func (ui *UI) layoutFunctionBarHintStrip(th *material.Theme, gtx layout.Context, hints []functionBarHintSpec) layout.Dimensions {
	stripH := gtx.Dp(unit.Dp(functionBarStripDp))
	if stripH < 1 {
		stripH = 1
	}
	ui.functionBarToolsButtonRect = image.Rectangle{}
	ui.customCommandMenuButtonRect = image.Rectangle{}

	return layout.Inset{
		Top:   unit.Dp(functionBarTopInsetDp),
		Left:  unit.Dp(functionBarOuterInsetDp),
		Right: unit.Dp(functionBarOuterInsetDp),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		barRadius := 0
		outerW := gtx.Constraints.Max.X
		if outerW < 1 {
			outerW = 1
		}
		specs := ui.functionBarButtonSpecs()
		widths := ui.functionBarWidths(th, gtx, specs)
		slots := functionBarHintSlots(hints, len(widths))
		labelColor := color.NRGBA{R: 228, G: 232, B: 240, A: 255}
		return fixedWidth(gtx, outerW, func(gtx layout.Context) layout.Dimensions {
			return fillRoundedBox(
				gtx,
				barRadius,
				color.NRGBA{R: 24, G: 24, B: 24, A: 255},
				color.NRGBA{R: 255, G: 255, B: 255, A: 22},
				func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
							return fillBgExact(gtx, color.NRGBA{R: 30, G: 34, B: 40, A: 255}, func(gtx layout.Context) layout.Dimensions {
								children := make([]layout.FlexChild, 0, len(widths))
								for i := range widths {
									i := i
									children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return fixedWidth(gtx, widths[i], func(gtx layout.Context) layout.Dimensions {
											return fillBgExact(gtx, color.NRGBA{}, func(gtx layout.Context) layout.Dimensions {
												return ui.layoutFunctionBarSplitLabel(th, gtx, slots[i].shortcut, slots[i].label, labelColor)
											})
										})
									}))
								}
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
							})
						})
					})
				},
			)
		})
	})
}

func dimColor(c color.NRGBA, alpha uint8) color.NRGBA {
	c.A = alpha
	return c
}

func (ui *UI) functionBarWidths(_ *material.Theme, gtx layout.Context, specs []functionBarButtonSpec) []int {
	widths := make([]int, len(specs))
	if len(specs) == 0 {
		return widths
	}
	avail := gtx.Constraints.Max.X
	if avail < len(widths) {
		avail = len(widths)
	}
	base := avail / len(widths)
	rem := avail % len(widths)
	for i := range widths {
		widths[i] = base
		if i < rem {
			widths[i]++
		}
	}
	return widths
}

func (ui *UI) functionBarIndexForAction(specs []functionBarButtonSpec, action functionBarAction) int {
	for i, spec := range specs {
		if spec.action == action {
			return i
		}
	}
	return -1
}

func (ui *UI) functionBarActiveIndex(specs []functionBarButtonSpec) int {
	if ui == nil {
		return -1
	}
	if ui.fileViewer != nil {
		return -1
	}
	switch {
	case ui.customCommandMenuOpen, ui.customCommandEditor != nil:
		return ui.functionBarIndexForAction(specs, functionBarActionCustom)
	case ui.functionBarToolsOpen, ui.settingsModal != nil, ui.Tabs.Value == "tab1", ui.Tabs.Value == "tab2", ui.Tabs.Value == "tab3":
		return ui.functionBarIndexForAction(specs, functionBarActionTools)
	case ui.fileDelete != nil:
		return ui.functionBarIndexForAction(specs, functionBarActionDelete)
	case ui.fileCreate != nil:
		return ui.functionBarIndexForAction(specs, functionBarActionCreate)
	case ui.fileMove != nil:
		return ui.functionBarIndexForAction(specs, functionBarActionMove)
	case ui.fileCopy != nil:
		return ui.functionBarIndexForAction(specs, functionBarActionCopy)
	default:
		return -1
	}
}

func (ui *UI) setFunctionBarSlider(index int, shown bool, now time.Time) {
	if ui == nil {
		return
	}
	if index < 0 {
		switch {
		case ui.functionBarSliderIndex >= 0:
			index = ui.functionBarSliderIndex
		case ui.functionBarSliderPrevIndex >= 0:
			index = ui.functionBarSliderPrevIndex
		default:
			index = 0
		}
	}
	if ui.functionBarSliderIndex == index && ui.functionBarSliderShown == shown {
		return
	}

	prevIndex := ui.functionBarSliderIndex
	if prevIndex < 0 || !ui.functionBarSliderShown {
		prevIndex = index
	}
	ui.functionBarSliderPrevIndex = prevIndex
	ui.functionBarSliderPrevShown = ui.functionBarSliderShown
	ui.functionBarSliderIndex = index
	ui.functionBarSliderShown = shown
	ui.functionBarSliderAnimAt = now
}

func (ui *UI) functionBarSliderState(now time.Time) (float32, float32, bool) {
	if ui == nil {
		return 0, 0, false
	}

	currentIndex := ui.functionBarSliderIndex
	if currentIndex < 0 {
		currentIndex = 0
	}
	prevIndex := ui.functionBarSliderPrevIndex
	if prevIndex < 0 {
		prevIndex = currentIndex
	}

	currentAlpha := float32(0)
	if ui.functionBarSliderShown {
		currentAlpha = 1
	}
	prevAlpha := float32(0)
	if ui.functionBarSliderPrevShown {
		prevAlpha = 1
	}

	if ui.functionBarSliderAnimAt.IsZero() || (prevIndex == currentIndex && prevAlpha == currentAlpha) {
		return float32(currentIndex), currentAlpha, false
	}

	elapsed := now.Sub(ui.functionBarSliderAnimAt)
	if elapsed >= toolbarAnimDur {
		ui.functionBarSliderPrevIndex = currentIndex
		ui.functionBarSliderPrevShown = ui.functionBarSliderShown
		ui.functionBarSliderAnimAt = time.Time{}
		return float32(currentIndex), currentAlpha, false
	}

	t := smoothstep01(clamp01(float32(elapsed) / float32(toolbarAnimDur)))
	pos := float32(prevIndex) + (float32(currentIndex)-float32(prevIndex))*t
	alpha := prevAlpha + (currentAlpha-prevAlpha)*t
	return pos, alpha, true
}

func (ui *UI) layoutFunctionBar(th *material.Theme, gtx layout.Context) layout.Dimensions {
	return layoutClippedToDimensions(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.layoutFunctionBarContent(th, gtx)
	})
}

func (ui *UI) layoutFunctionBarContent(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if ui == nil {
		return layout.Dimensions{}
	}
	if hints := ui.functionBarModifierHintSpecsForContext(ui.terminalFocused(gtx), runtime.GOOS); len(hints) > 0 {
		return ui.layoutFunctionBarHintStrip(th, gtx, hints)
	}

	specs := ui.functionBarButtonSpecs()
	for _, spec := range specs {
		if spec.click == nil {
			continue
		}
		for spec.click.Clicked(gtx) {
			ui.setToolbarPulse(spec.keyLabel, gtx.Now)
			if !ui.performFunctionBarActionContext(gtx, spec.action) {
				continue
			}
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	specs = ui.functionBarButtonSpecs()
	hoverKey := ""
	hoverIndex := -1
	for i, spec := range specs {
		if spec.click != nil && spec.click.Hovered() {
			hoverKey = spec.keyLabel
			hoverIndex = i
		}
	}
	ui.setToolbarHover(hoverKey, gtx.Now)

	activeIndex := ui.functionBarActiveIndex(specs)
	targetIndex := activeIndex
	targetShown := activeIndex >= 0
	if hoverIndex >= 0 {
		targetIndex = hoverIndex
		targetShown = true
	}
	ui.setFunctionBarSlider(targetIndex, targetShown, gtx.Now)
	sliderPos, sliderAlpha, sliderAnim := ui.functionBarSliderState(gtx.Now)

	leftPx := gtx.Dp(unit.Dp(functionBarOuterInsetDp))
	topPx := gtx.Dp(unit.Dp(functionBarTopInsetDp))
	stripH := gtx.Dp(unit.Dp(functionBarStripDp))
	if stripH < 1 {
		stripH = 1
	}
	ui.functionBarToolsButtonRect = image.Rectangle{}
	ui.customCommandMenuButtonRect = image.Rectangle{}

	animating := sliderAnim
	dims := layout.Inset{
		Top:   unit.Dp(functionBarTopInsetDp),
		Left:  unit.Dp(functionBarOuterInsetDp),
		Right: unit.Dp(functionBarOuterInsetDp),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		barRadius := 0
		outerW := gtx.Constraints.Max.X
		if outerW < 1 {
			outerW = 1
		}
		widths := ui.functionBarWidths(th, gtx, specs)
		starts := make([]int, len(widths))
		totalW := 0
		for i, w := range widths {
			starts[i] = totalW
			totalW += w
		}
		if totalW < outerW {
			totalW = outerW
		}
		return fixedWidth(gtx, outerW, func(gtx layout.Context) layout.Dimensions {
			return fillRoundedBox(
				gtx,
				barRadius,
				color.NRGBA{R: 24, G: 24, B: 24, A: 255},
				color.NRGBA{R: 255, G: 255, B: 255, A: 22},
				func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
							innerR := barRadius

							if sliderAlpha > 0 && len(widths) > 0 {
								baseIdx := int(sliderPos)
								if baseIdx < 0 {
									baseIdx = 0
								}
								if baseIdx > len(widths)-1 {
									baseIdx = len(widths) - 1
								}
								nextIdx := baseIdx + 1
								if nextIdx > len(widths)-1 {
									nextIdx = len(widths) - 1
								}
								frac := sliderPos - float32(baseIdx)
								sliderX := int(float32(starts[baseIdx]) + float32(starts[nextIdx]-starts[baseIdx])*frac)
								sliderW := int(float32(widths[baseIdx]) + float32(widths[nextIdx]-widths[baseIdx])*frac)
								if sliderW < 1 {
									sliderW = 1
								}
								sliderRect := image.Rect(sliderX, 0, sliderX+sliderW, stripH)
								innerClip := clip.UniformRRect(image.Rect(0, 0, totalW, stripH), innerR).Push(gtx.Ops)
								paint.FillShape(gtx.Ops, dimColor(color.NRGBA{R: 56, G: 56, B: 56, A: 255}, uint8(255*clamp01(sliderAlpha))), clip.UniformRRect(sliderRect, innerR).Op(gtx.Ops))
								innerClip.Pop()
							}

							cursorX := leftPx + 1
							y0 := topPx + 1
							children := make([]layout.FlexChild, 0, len(specs))
							for i, spec := range specs {
								i := i
								spec := spec
								segW := widths[i]
								if spec.action == functionBarActionCustom {
									ui.customCommandMenuButtonRect = image.Rect(cursorX, y0, cursorX+segW, y0+stripH)
								}
								if spec.action == functionBarActionTools {
									ui.functionBarToolsButtonRect = image.Rect(cursorX, y0, cursorX+segW, y0+stripH)
								}
								cursorX += segW

								hoverFill, hoverAnim := ui.toolbarHoverLevel(gtx.Now, spec.keyLabel)
								pulseFill, pulseAnim := ui.toolbarPulseLevel(gtx.Now, spec.keyLabel)
								animating = animating || hoverAnim || pulseAnim

								children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return fixedWidth(gtx, segW, func(gtx layout.Context) layout.Dimensions {
										return spec.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											hoverFill = clamp01(hoverFill)
											pulseFill = clamp01(pulseFill)
											if spec.click.Pressed() && pulseFill < 0.55 {
												pulseFill = 0.55
											}

											proximity := clamp01(1-float32Abs(float32(i)-sliderPos)) * clamp01(sliderAlpha)
											bg := color.NRGBA{}
											bg = mixNRGBA(bg, color.NRGBA{R: 255, G: 255, B: 255, A: 8}, hoverFill*(1-proximity)*0.45)
											bg = mixNRGBA(bg, color.NRGBA{R: 255, G: 255, B: 255, A: 16}, pulseFill*0.18)

											fg := slidingStripTextColor(proximity, hoverFill, pulseFill)
											if !spec.enabled {
												bg = dimColor(bg, uint8(float32(bg.A)*0.6))
												fg = mixNRGBA(dimColor(fg, 112), fg, hoverFill*0.55+proximity*0.35)
											}

											dims := fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
												return ui.layoutFunctionBarSplitLabel(th, gtx, spec.keyLabel, spec.label, fg)
											})
											if spec.enabled {
												defer clip.Rect(image.Rectangle{Max: image.Pt(segW, stripH)}).Push(gtx.Ops).Pop()
												pointer.CursorPointer.Add(gtx.Ops)
											}
											return dims
										})
									})
								}))
							}
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
						})
					})
				},
			)
		})
	})
	if animating {
		gtx.Execute(op.InvalidateCmd{})
	}
	return dims
}

func (ui *UI) ensureFunctionBarToolClicks(n int) {
	if ui == nil {
		return
	}
	if n <= cap(ui.functionBarToolClicks) {
		ui.functionBarToolClicks = ui.functionBarToolClicks[:n]
		return
	}
	old := ui.functionBarToolClicks
	ui.functionBarToolClicks = make([]widget.Clickable, n)
	copy(ui.functionBarToolClicks, old)
}

func (ui *UI) functionBarToolSpecs() []functionBarToolSpec {
	active := "files"
	switch {
	case ui == nil:
		active = "files"
	case ui.settingsModal != nil:
		active = "settings"
	case ui.sshModal != nil:
		active = "ssh"
	case ui.Tabs.Value == "tab1":
		active = "hex"
	case ui.Tabs.Value == "tab2":
		active = "protocol"
	case ui.Tabs.Value == "tab3":
		active = "http"
	}
	return []functionBarToolSpec{
		{key: "multi-rename", label: "Multi-Rename", shortcut: "Ctrl+M"},
		{key: "ssh", label: "SSH Setup", shortcut: "Ctrl+F", active: active == "ssh"},
		{key: "hex", label: "Hex to ASCII", active: active == "hex"},
		{key: "protocol", label: "Protocol Analyzer", active: active == "protocol"},
		{key: "http", label: "HTTP Client", active: active == "http"},
		{key: "settings", label: "Settings", shortcut: "Ctrl+S", active: active == "settings"},
	}
}

func functionBarDefaultToolIndex(items []functionBarToolSpec) int {
	if len(items) == 0 {
		return -1
	}
	for i, item := range items {
		if item.active {
			return i
		}
	}
	return 0
}

func clampFunctionBarToolIndex(index, n int) int {
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

func (ui *UI) setFunctionBarToolSelection(index int, items []functionBarToolSpec) bool {
	if ui == nil {
		return false
	}
	next := clampFunctionBarToolIndex(index, len(items))
	if next < 0 {
		next = functionBarDefaultToolIndex(items)
	}
	if ui.functionBarToolsSelected == next {
		return false
	}
	ui.functionBarToolsSelected = next
	return true
}

func (ui *UI) currentFunctionBarToolSelection(items []functionBarToolSpec) int {
	if ui == nil {
		return -1
	}
	if idx := clampFunctionBarToolIndex(ui.functionBarToolsSelected, len(items)); idx >= 0 {
		return idx
	}
	return functionBarDefaultToolIndex(items)
}

func (ui *UI) moveFunctionBarToolSelection(delta int) bool {
	if ui == nil || !ui.functionBarToolsOpen || delta == 0 {
		return false
	}
	items := ui.functionBarToolSpecs()
	if len(items) == 0 {
		return false
	}
	index := ui.currentFunctionBarToolSelection(items)
	if index < 0 {
		return false
	}
	index += delta
	if index < 0 {
		index = len(items) - 1
	} else if index >= len(items) {
		index = 0
	}
	return ui.setFunctionBarToolSelection(index, items)
}

func (ui *UI) activateSelectedFunctionBarTool(now time.Time) bool {
	if ui == nil || !ui.functionBarToolsOpen {
		return false
	}
	items := ui.functionBarToolSpecs()
	index := ui.currentFunctionBarToolSelection(items)
	if index < 0 || index >= len(items) {
		return false
	}
	ui.activateFunctionBarTool(items[index].key, now)
	return true
}

func (ui *UI) activateFunctionBarTool(key string, now time.Time) {
	if ui == nil {
		return
	}
	ui.closeFunctionBarPopups()
	switch key {
	case "multi-rename":
		ui.startMultiRename(ui.activeFilePane, now)
	case "ssh":
		ui.openSSHModal()
	case "files":
		ui.setActiveTab("tab0", now)
	case "hex":
		ui.setActiveTab("tab1", now)
	case "protocol":
		ui.setActiveTab("tab2", now)
	case "http":
		ui.setActiveTab("tab3", now)
	case "settings":
		ui.openSettingsModal()
	}
}

func (ui *UI) functionBarToolsAnchorRect(gtx layout.Context) image.Rectangle {
	if ui != nil && ui.functionBarToolsButtonRect.Dx() > 0 && ui.functionBarToolsButtonRect.Dy() > 0 {
		return ui.functionBarToolsButtonRect
	}
	x := gtx.Dp(unit.Dp(functionBarOuterInsetDp)) + 1
	y := gtx.Dp(unit.Dp(functionBarTopInsetDp)) + 1
	h := gtx.Dp(unit.Dp(functionBarStripDp))
	if h < 1 {
		h = 1
	}
	return image.Rect(x, y, x, y+h)
}

func (ui *UI) handleFunctionBarPopupOutsideClick(gtx layout.Context) {
	if ui == nil || !ui.functionBarToolsOpen {
		return
	}
	pressedPopup := popupPressed(gtx, &ui.functionBarPopupBodyTag)
	closed := false
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &ui.functionBarPopupGlobalTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok || pe.Kind != pointer.Press || !pe.Buttons.Contain(pointer.ButtonPrimary) {
			continue
		}
		if ui.functionBarClicks[8].Hovered() || pressedPopup {
			continue
		}
		ui.closeFunctionBarToolsMenu()
		closed = true
	}
	if closed {
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) functionBarToolCardWidth(th *material.Theme, gtx layout.Context, items []functionBarToolSpec) int {
	maxTextW := 0
	for _, item := range items {
		lbl := material.Body2(th, item.label)
		lbl.Font.Typeface = ui.interfaceTypeface()
		lbl.Font.Weight = font.Medium
		lbl.TextSize = ui.functionBarTextSize()
		lbl.MaxLines = 1
		w := measureLabelUnconstrained(gtx, lbl).Size.X
		if item.shortcut != "" {
			shortcut := material.Caption(th, item.shortcut)
			shortcut.Font.Typeface = ui.interfaceTypeface()
			shortcut.Font.Weight = font.Medium
			shortcut.TextSize = scaleConfigFontSize(ui.fmCfg, 9)
			shortcut.MaxLines = 1
			w += gtx.Dp(unit.Dp(18)) + measureLabelUnconstrained(gtx, shortcut).Size.X
		}
		if w > maxTextW {
			maxTextW = w
		}
	}
	if maxTextW == 0 {
		maxTextW = gtx.Dp(unit.Dp(96))
	}
	width := maxTextW + gtx.Dp(unit.Dp(26))
	if width < gtx.Dp(unit.Dp(132)) {
		width = gtx.Dp(unit.Dp(132))
	}
	if width > gtx.Constraints.Max.X {
		width = gtx.Constraints.Max.X
	}
	if width < 1 {
		width = 1
	}
	return width
}

func (ui *UI) functionBarHoveredToolID(items []functionBarToolSpec) string {
	if ui == nil {
		return ""
	}
	hoverID := ""
	for i, item := range items {
		if i < len(ui.functionBarToolClicks) && ui.functionBarToolClicks[i].Hovered() {
			hoverID = item.key
		}
	}
	return hoverID
}

func (ui *UI) functionBarHoveredToolIndex(items []functionBarToolSpec) int {
	if ui == nil {
		return -1
	}
	for i := range items {
		if i < len(ui.functionBarToolClicks) && ui.functionBarToolClicks[i].Hovered() {
			return i
		}
	}
	return -1
}

func (ui *UI) layoutFunctionBarToolOption(th *material.Theme, gtx layout.Context, theme filePanePopupTheme, click *widget.Clickable, item functionBarToolSpec, selected bool, selectedFill, alpha float32) layout.Dimensions {
	if click == nil {
		return layout.Dimensions{}
	}
	menuItem := fileContextMenuItem{ID: item.key, Label: item.label}
	rowH := ui.fileContextMenuRowHeight(gtx, menuItem)
	selectedT := smoothstep01(clamp01(selectedFill))
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)

		bg := color.NRGBA{}
		fg := scaleColorAlpha(theme.Text, alpha)
		detailColor := scaleColorAlpha(theme.Muted, alpha)
		weight := font.Medium

		if item.active {
			bg = scaleColorAlpha(mixNRGBA(theme.Bg, theme.ActiveBg, 0.58), alpha)
			fg = scaleColorAlpha(theme.ActiveText, alpha)
			detailColor = scaleColorAlpha(mixNRGBA(theme.ActiveText, theme.ActiveBg, 0.42), alpha)
		}

		accent := color.NRGBA{}
		if selected && selectedT > 0 {
			selectedBg := mixNRGBA(theme.HoverBg, theme.ActiveBg, 0.42)
			selectedText := bestContrastColor(selectedBg, theme.ActiveText, theme.HoverText, theme.Text)
			selectedDetail := mixNRGBA(selectedText, selectedBg, 0.46)
			if item.active {
				bg = scaleColorAlpha(mixNRGBA(bg, selectedBg, 0.82*selectedT), alpha)
				fg = scaleColorAlpha(mixNRGBA(theme.ActiveText, selectedText, 0.44*selectedT), alpha)
				detailColor = scaleColorAlpha(mixNRGBA(detailColor, selectedDetail, 0.6*selectedT), alpha)
			} else {
				bg = scaleColorAlpha(mixNRGBA(theme.Bg, selectedBg, 0.9*selectedT), alpha)
				fg = scaleColorAlpha(mixNRGBA(theme.Text, selectedText, 0.88*selectedT), alpha)
				detailColor = scaleColorAlpha(mixNRGBA(theme.Muted, selectedDetail, 0.76*selectedT), alpha)
			}
			accent = scaleColorAlpha(mixNRGBA(selectedText, theme.ActiveBg, 0.16), alpha*selectedT)
		}

		return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
			m := op.Record(gtx.Ops)
			dims := fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(6), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body2(th, item.label)
							lbl.Font.Typeface = ui.interfaceTypeface()
							lbl.TextSize = ui.functionBarTextSize()
							lbl.Font.Weight = weight
							lbl.Color = fg
							lbl.MaxLines = 1
							lbl.Truncator = "…"
							return layoutVCenteredLabel(gtx, lbl)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if item.shortcut == "" {
								return layout.Dimensions{}
							}
							return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Caption(th, item.shortcut)
								lbl.Font.Typeface = ui.interfaceTypeface()
								lbl.TextSize = scaleConfigFontSize(ui.fmCfg, 9)
								lbl.Font.Weight = font.Medium
								lbl.Color = detailColor
								lbl.MaxLines = 1
								return layoutVCenteredLabel(gtx, lbl)
							})
						}),
					)
				})
			})
			call := m.Stop()
			call.Add(gtx.Ops)
			if accent.A != 0 && dims.Size.X > 0 && dims.Size.Y > 0 {
				yPad := gtx.Dp(unit.Dp(3))
				if yPad*2 >= dims.Size.Y {
					yPad = 0
				}
				w := gtx.Dp(unit.Dp(3))
				if w < 1 {
					w = 1
				}
				x := gtx.Dp(unit.Dp(2))
				if x+w > dims.Size.X {
					x = 0
				}
				rect := image.Rect(x, yPad, x+w, dims.Size.Y-yPad)
				if rect.Dx() > 0 && rect.Dy() > 0 {
					paint.FillShape(gtx.Ops, accent, clip.UniformRRect(rect, w).Op(gtx.Ops))
				}
			}
			return dims
		})
	})
}

func (ui *UI) layoutFunctionBarToolsCard(th *material.Theme, gtx layout.Context, items []functionBarToolSpec, alpha float32) layout.Dimensions {
	width := ui.functionBarToolCardWidth(th, gtx, items)
	theme := ui.filePanePopupTheme()
	hoverIndex := ui.functionBarHoveredToolIndex(items)
	if hoverIndex >= 0 && hoverIndex != ui.functionBarToolsSelected {
		ui.functionBarToolsSelected = hoverIndex
	}
	selectedIndex := ui.currentFunctionBarToolSelection(items)
	hoverID := ""
	if hoverIndex >= 0 && hoverIndex < len(items) {
		hoverID = items[hoverIndex].key
	} else if selectedIndex >= 0 && selectedIndex < len(items) {
		hoverID = items[selectedIndex].key
	}
	if hoverID != ui.functionBarToolsHoverID {
		ui.functionBarToolsHoverID = hoverID
		ui.functionBarToolsHoverAnim.setHover(hoverID, gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		dims := fillRoundedClipBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			scaleColorAlpha(theme.Bg, alpha),
			scaleColorAlpha(theme.Border, alpha),
			func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(items)+2)
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, ui.fileContextMenuTitleHeight(gtx), func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(4), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Caption(th, "Tools")
							lbl.Font.Typeface = ui.interfaceTypeface()
							lbl.Font.Weight = font.Medium
							lbl.TextSize = scaleConfigFontSize(ui.fmCfg, 9)
							lbl.Color = scaleColorAlpha(theme.Title, alpha)
							lbl.MaxLines = 1
							lbl.Truncator = "…"
							return layoutVCenteredLabel(gtx, lbl)
						})
					})
				}))
				for i, item := range items {
					i := i
					item := item
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						hoverFill, animating := ui.functionBarToolsHoverAnim.hoverFill(gtx.Now, item.key)
						if animating {
							gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
						}
						return ui.layoutFunctionBarToolOption(th, gtx, theme, &ui.functionBarToolClicks[i], item, i == selectedIndex, hoverFill, alpha)
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			},
		)
		registerPopupArea(gtx, &ui.functionBarPopupBodyTag, dims.Size)
		return dims
	})
}

func (ui *UI) layoutFunctionBarPopup(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if ui == nil || !ui.functionBarToolsOpen {
		return layout.Dimensions{}
	}

	items := ui.functionBarToolSpecs()
	ui.ensureFunctionBarToolClicks(len(items))
	for i, item := range items {
		if i >= len(ui.functionBarToolClicks) {
			break
		}
		for ui.functionBarToolClicks[i].Clicked(gtx) {
			ui.activateFunctionBarTool(item.key, gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
			return layout.Dimensions{}
		}
	}

	alpha, slideY, animating := popupOpenProgress(gtx.Now, ui.functionBarToolsOpenedAt)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	blockClip := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, &ui.functionBarPopupGlobalTag)
	blockClip.Pop()
	m := op.Record(gtx.Ops)
	card := ui.layoutFunctionBarToolsCard(th, gtx, items, alpha)
	call := m.Stop()

	anchorRect := ui.functionBarToolsAnchorRect(gtx)
	anchor := image.Point{
		X: anchorRect.Min.X,
		Y: anchorRect.Max.Y + gtx.Dp(unit.Dp(functionBarPopupGapDp)) + slideY,
	}
	anchor = clampFilePaneMenuPoint(anchor, card.Size, gtx.Constraints.Max)
	ui.functionBarToolsRect = image.Rectangle{Min: anchor, Max: anchor.Add(card.Size)}

	bodyClip := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	offset := op.Offset(anchor).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	bodyClip.Pop()

	ui.handleFunctionBarPopupOutsideClick(gtx)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *UI) handleFunctionBarPopupKeys(gtx layout.Context) {
	if ui == nil || !ui.functionBarToolsOpen {
		return
	}
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameUpArrow},
			key.Filter{Name: key.NameDownArrow},
			key.Filter{Name: key.NameEnter},
			key.Filter{Name: key.NameReturn},
		)
		if !ok {
			return
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press || ke.Modifiers != 0 {
			continue
		}
		handled := false
		switch ke.Name {
		case key.NameUpArrow:
			handled = ui.moveFunctionBarToolSelection(-1)
		case key.NameDownArrow:
			handled = ui.moveFunctionBarToolSelection(1)
		case key.NameEnter, key.NameReturn:
			handled = ui.activateSelectedFunctionBarTool(gtx.Now)
		}
		if handled {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}
