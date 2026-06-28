// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"hexone/fm"
)

func registerTabShortcutKey(router *input.Router, name key.Name, required key.Modifiers) {
	router.Event(key.Filter{Name: name, Required: required, Optional: ^key.Modifiers(0)})
}

func TestCtrlNAddsTabOnlyToActiveFilePane(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.activeFilePane = 1

	gtx, router := testKeyContext()
	registerTabShortcutKey(router, "N", key.ModCtrl)
	router.Queue(key.Event{Name: "N", Modifiers: key.ModCtrl, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)

	if got, want := len(ui.filePaneTabs[0].tabs), 1; got != want {
		t.Fatalf("left pane tab count=%d want %d", got, want)
	}
	if got, want := len(ui.filePaneTabs[1].tabs), 2; got != want {
		t.Fatalf("active right pane tab count=%d want %d", got, want)
	}
	if got, want := ui.filePaneTabs[1].active, 1; got != want {
		t.Fatalf("active right pane tab=%d want %d", got, want)
	}
}

func TestCtrlXClosesOnlyActiveFilePaneTab(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	if !ui.addFilePaneTab(0) || !ui.addFilePaneTab(1) {
		t.Fatal("failed to prepare file pane tabs")
	}
	ui.activeFilePane = 1

	gtx, router := testKeyContext()
	registerTabShortcutKey(router, "x", key.ModCtrl)
	router.Queue(key.Event{Name: "x", Modifiers: key.ModCtrl, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)

	if got, want := len(ui.filePaneTabs[0].tabs), 2; got != want {
		t.Fatalf("inactive left pane tab count=%d want unchanged %d", got, want)
	}
	if got, want := len(ui.filePaneTabs[1].tabs), 1; got != want {
		t.Fatalf("active right pane tab count=%d want %d", got, want)
	}
}

func TestCtrlXKeepsLastFilePaneTabOpen(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())

	gtx, router := testKeyContext()
	registerTabShortcutKey(router, "X", key.ModCtrl)
	router.Queue(key.Event{Name: "X", Modifiers: key.ModCtrl, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)

	if got, want := len(ui.filePaneTabs[0].tabs), 1; got != want {
		t.Fatalf("file pane tab count=%d want last tab preserved %d", got, want)
	}
}

func TestCtrlTabNavigationStaysInActiveFilePaneAndWraps(t *testing.T) {
	cfg := fm.DefaultConfig()
	left := []*filePaneState{newFilePaneState(t.TempDir(), cfg), newFilePaneState(t.TempDir(), cfg)}
	right := []*filePaneState{newFilePaneState(t.TempDir(), cfg), newFilePaneState(t.TempDir(), cfg)}
	ui := &UI{
		fmCfg:          cfg,
		filePanes:      []*filePaneState{left[1], right[1]},
		filePaneTabs:   []filePaneTabSet{{tabs: left, active: 1}, {tabs: right, active: 1}},
		activeFilePane: 1,
	}
	ui.Tabs.Value = "tab0"

	gtx, router := testKeyContext()
	registerTabShortcutKey(router, key.NameTab, key.ModCtrl)
	router.Queue(
		key.Event{Name: key.NameTab, Modifiers: key.ModCtrl, State: key.Press},
		key.Event{Name: key.NameTab, Modifiers: key.ModCtrl, State: key.Release},
	)

	ui.handleGlobalFunctionKeys(gtx)

	if got, want := ui.filePaneTabs[0].active, 1; got != want {
		t.Fatalf("inactive left pane tab=%d want unchanged %d", got, want)
	}
	if got, want := ui.filePaneTabs[1].active, 0; got != want {
		t.Fatalf("active right pane tab=%d want wrapped %d", got, want)
	}
}

func TestCtrlShiftTabSelectsPreviousFilePaneTab(t *testing.T) {
	cfg := fm.DefaultConfig()
	tabs := []*filePaneState{
		newFilePaneState(t.TempDir(), cfg),
		newFilePaneState(t.TempDir(), cfg),
		newFilePaneState(t.TempDir(), cfg),
	}
	ui := &UI{
		fmCfg:          cfg,
		filePanes:      []*filePaneState{tabs[0]},
		filePaneTabs:   []filePaneTabSet{{tabs: tabs, active: 0}},
		activeFilePane: 0,
	}
	ui.Tabs.Value = "tab0"

	gtx, router := testKeyContext()
	registerTabShortcutKey(router, key.NameTab, key.ModCtrl)
	router.Queue(key.Event{Name: key.NameTab, Modifiers: key.ModCtrl | key.ModShift, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)

	if got, want := ui.filePaneTabs[0].active, 2; got != want {
		t.Fatalf("active pane tab=%d want wrapped previous %d", got, want)
	}
}

func TestTerminalTabShortcutsDoNotChangeFilePaneTabs(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.ensureTerminalTabs()
	ui.terminal.setActive(true)
	if !ui.addTerminalTab() {
		t.Fatal("failed to prepare second terminal tab")
	}
	ui.activateTerminalTab(1)

	gtx, router := testKeyContext()
	gtx.Execute(key.FocusCmd{Tag: &ui.terminal.keyTag})
	if !ui.terminalFocused(gtx) {
		t.Fatal("terminal should be focused")
	}

	registerTabShortcutKey(router, key.NameTab, key.ModCtrl)
	registerTabShortcutKey(router, key.NameLeftArrow, key.ModCtrl)
	router.Queue(
		key.Event{Name: key.NameTab, Modifiers: key.ModCtrl, State: key.Press},
		key.Event{Name: key.NameLeftArrow, Modifiers: key.ModCtrl, State: key.Press},
		key.Event{Name: key.NameTab, Modifiers: key.ModCtrl, State: key.Release},
	)

	ui.handleGlobalFunctionKeys(gtx)

	if got, want := ui.terminalTabs.active, 0; got != want {
		t.Fatalf("active terminal tab=%d want %d", got, want)
	}
	for i := range ui.filePaneTabs {
		if got, want := ui.filePaneTabs[i].active, 0; got != want {
			t.Fatalf("file pane %d active tab=%d want unchanged %d", i, got, want)
		}
		if got, want := len(ui.filePaneTabs[i].tabs), 1; got != want {
			t.Fatalf("file pane %d tab count=%d want unchanged %d", i, got, want)
		}
	}
}

func TestCtrlNAddsTerminalTabWhenTerminalFocused(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.ensureTerminalTabs()
	ui.terminal.setActive(true)

	gtx, router := testKeyContext()
	gtx.Execute(key.FocusCmd{Tag: &ui.terminal.keyTag})
	registerTabShortcutKey(router, "n", key.ModCtrl)
	router.Queue(key.Event{Name: "n", Modifiers: key.ModCtrl, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)

	if got, want := len(ui.terminalTabs.sessions), 2; got != want {
		t.Fatalf("terminal tab count=%d want %d", got, want)
	}
	if got, want := ui.terminalTabs.active, 1; got != want {
		t.Fatalf("active terminal tab=%d want %d", got, want)
	}
	for i := range ui.filePaneTabs {
		if got, want := len(ui.filePaneTabs[i].tabs), 1; got != want {
			t.Fatalf("file pane %d tab count=%d want unchanged %d", i, got, want)
		}
	}
}

func TestCtrlXClosesTerminalTabWithoutChangingFilePaneTabs(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.ensureTerminalTabs()
	ui.terminal.setActive(true)
	if !ui.addTerminalTab() {
		t.Fatal("failed to prepare second terminal tab")
	}

	gtx, router := testKeyContext()
	gtx.Execute(key.FocusCmd{Tag: &ui.terminal.keyTag})
	registerTabShortcutKey(router, "x", key.ModCtrl)
	router.Queue(key.Event{Name: "x", Modifiers: key.ModCtrl, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)

	if got, want := len(ui.terminalTabs.sessions), 1; got != want {
		t.Fatalf("terminal tab count=%d want %d", got, want)
	}
	for i := range ui.filePaneTabs {
		if got, want := len(ui.filePaneTabs[i].tabs), 1; got != want {
			t.Fatalf("file pane %d tab count=%d want unchanged %d", i, got, want)
		}
	}
}

func TestTabShortcutsRemainAvailableOnlyInFileManager(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab1"

	gtx, router := testKeyContext()
	registerTabShortcutKey(router, "N", key.ModCtrl)
	router.Queue(key.Event{Name: "N", Modifiers: key.ModCtrl, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)

	if got, want := len(ui.filePaneTabs[0].tabs), 1; got != want {
		t.Fatalf("file pane tab count=%d want unchanged %d outside file manager", got, want)
	}
	if _, ok := gtx.Event(key.Filter{Name: "N", Required: key.ModCtrl, Optional: ^key.Modifiers(0)}); !ok {
		t.Fatal("Ctrl+N should remain available outside the file manager")
	}
}

func TestTerminalCtrlArrowIsNotStolenWithoutTabChord(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.ensureTerminalTabs()
	ui.terminal.setActive(true)

	gtx, router := testKeyContext()
	gtx.Execute(key.FocusCmd{Tag: &ui.terminal.keyTag})
	registerTabShortcutKey(router, key.NameLeftArrow, key.ModCtrl)
	router.Queue(key.Event{Name: key.NameLeftArrow, Modifiers: key.ModCtrl, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)

	if _, ok := gtx.Event(key.Filter{Name: key.NameLeftArrow, Required: key.ModCtrl, Optional: ^key.Modifiers(0)}); !ok {
		t.Fatal("Ctrl+Left should remain available to the terminal without an armed Ctrl+Tab chord")
	}
}
