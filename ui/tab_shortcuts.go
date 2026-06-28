// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
)

type tabShortcutTarget uint8

const (
	tabShortcutTargetNone tabShortcutTarget = iota
	tabShortcutTargetFilePane
	tabShortcutTargetTerminal
)

type tabShortcutState struct {
	armed  bool
	used   bool
	target tabShortcutTarget
	pane   int
}

func (ui *UI) handleTabShortcuts(gtx layout.Context) bool {
	if ui == nil {
		return false
	}
	if ui.tabShortcutsBlocked() {
		ui.tabShortcut = tabShortcutState{}
		return false
	}
	target, pane := ui.currentTabShortcutTarget(gtx)
	if target == tabShortcutTargetNone && !ui.tabShortcut.armed {
		return false
	}

	anyMods := ^key.Modifiers(0)
	filters := []event.Filter{
		key.Filter{Name: "N", Required: key.ModCtrl, Optional: anyMods},
		key.Filter{Name: "n", Required: key.ModCtrl, Optional: anyMods},
		key.Filter{Name: "N", Required: key.ModShortcut, Optional: anyMods},
		key.Filter{Name: "n", Required: key.ModShortcut, Optional: anyMods},
		key.Filter{Name: "N", Required: key.ModCommand, Optional: anyMods},
		key.Filter{Name: "n", Required: key.ModCommand, Optional: anyMods},
		key.Filter{Name: "X", Required: key.ModCtrl, Optional: anyMods},
		key.Filter{Name: "x", Required: key.ModCtrl, Optional: anyMods},
		key.Filter{Name: "X", Required: key.ModShortcut, Optional: anyMods},
		key.Filter{Name: "x", Required: key.ModShortcut, Optional: anyMods},
		key.Filter{Name: "X", Required: key.ModCommand, Optional: anyMods},
		key.Filter{Name: "x", Required: key.ModCommand, Optional: anyMods},
		key.Filter{Name: key.NameTab, Required: key.ModCtrl, Optional: anyMods},
		key.Filter{Name: key.NameTab, Required: key.ModShortcut, Optional: anyMods},
		key.Filter{Name: key.NameTab, Required: key.ModCommand, Optional: anyMods},
	}

	handled := ui.handleArmedTabShortcutArrows(gtx)
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return handled
		}
		ke, ok := ev.(key.Event)
		if !ok {
			continue
		}

		switch ke.Name {
		case "N", "n":
			if ke.State != key.Press || !tabShortcutModifiers(ke.Modifiers, false) {
				continue
			}
			handled = true
			if ui.addTabForShortcutTarget(target, pane) {
				gtx.Execute(op.InvalidateCmd{})
			}

		case "X", "x":
			if ke.State != key.Press || !tabShortcutModifiers(ke.Modifiers, false) {
				continue
			}
			handled = true
			if ui.closeTabForShortcutTarget(target, pane) {
				gtx.Execute(op.InvalidateCmd{})
			}

		case key.NameTab:
			if !tabShortcutModifiers(ke.Modifiers, true) {
				continue
			}
			handled = true
			switch ke.State {
			case key.Press:
				if ke.Modifiers.Contain(key.ModShift) {
					ui.tabShortcut = tabShortcutState{}
					if ui.stepTabForShortcutTarget(target, pane, -1) {
						gtx.Execute(op.InvalidateCmd{})
					}
					continue
				}
				ui.tabShortcut = tabShortcutState{
					armed:  true,
					target: target,
					pane:   pane,
				}
				if ui.handleArmedTabShortcutArrows(gtx) {
					handled = true
				}
			case key.Release:
				state := ui.tabShortcut
				ui.tabShortcut = tabShortcutState{}
				if state.armed && !state.used && ui.stepTabForShortcutTarget(state.target, state.pane, 1) {
					gtx.Execute(op.InvalidateCmd{})
				}
			}
		}
	}
}

func (ui *UI) handleArmedTabShortcutArrows(gtx layout.Context) bool {
	if ui == nil || !ui.tabShortcut.armed {
		return false
	}
	anyMods := ^key.Modifiers(0)
	handled := false
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameLeftArrow, Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: key.NameLeftArrow, Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: key.NameLeftArrow, Required: key.ModCommand, Optional: anyMods},
			key.Filter{Name: key.NameRightArrow, Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: key.NameRightArrow, Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: key.NameRightArrow, Required: key.ModCommand, Optional: anyMods},
		)
		if !ok {
			return handled
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press || !tabShortcutModifiers(ke.Modifiers, false) {
			continue
		}
		handled = true
		step := -1
		if ke.Name == key.NameRightArrow {
			step = 1
		}
		ui.tabShortcut.used = true
		if ui.stepTabForShortcutTarget(ui.tabShortcut.target, ui.tabShortcut.pane, step) {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (ui *UI) tabShortcutsBlocked() bool {
	return ui.helpModal != nil ||
		ui.settingsModal != nil ||
		ui.sshModal != nil ||
		ui.hasBlockingFileDialog()
}

func (ui *UI) currentTabShortcutTarget(gtx layout.Context) (tabShortcutTarget, int) {
	if ui.terminalFocused(gtx) {
		return tabShortcutTargetTerminal, -1
	}
	if ui.Tabs.Value != "tab0" || ui.fileViewer != nil || ui.pathEditActive() {
		return tabShortcutTargetNone, -1
	}
	if ui.activeFilePane < 0 || ui.activeFilePane >= len(ui.filePanes) {
		return tabShortcutTargetNone, -1
	}
	return tabShortcutTargetFilePane, ui.activeFilePane
}

func (ui *UI) addTabForShortcutTarget(target tabShortcutTarget, pane int) bool {
	switch target {
	case tabShortcutTargetFilePane:
		return ui.addFilePaneTab(pane)
	case tabShortcutTargetTerminal:
		return ui.addTerminalTab()
	default:
		return false
	}
}

func (ui *UI) closeTabForShortcutTarget(target tabShortcutTarget, pane int) bool {
	switch target {
	case tabShortcutTargetFilePane:
		ui.ensureFilePaneTabs()
		if pane < 0 || pane >= len(ui.filePaneTabs) {
			return false
		}
		return ui.closeFilePaneTab(pane, ui.filePaneTabs[pane].active)
	case tabShortcutTargetTerminal:
		ui.ensureTerminalTabs()
		return ui.closeTerminalTab(ui.terminalTabs.active)
	default:
		return false
	}
}

func (ui *UI) stepTabForShortcutTarget(target tabShortcutTarget, pane, step int) bool {
	switch target {
	case tabShortcutTargetFilePane:
		return ui.stepFilePaneTab(pane, step)
	case tabShortcutTargetTerminal:
		return ui.stepTerminalTab(step)
	default:
		return false
	}
}

func tabShortcutModifiers(mods key.Modifiers, allowShift bool) bool {
	shift := mods.Contain(key.ModShift)
	if shift && !allowShift {
		return false
	}
	mods &^= key.ModShift
	shortcutMods := key.ModCtrl | key.ModShortcut | key.ModCommand
	return mods&shortcutMods != 0 && mods&^shortcutMods == 0
}
