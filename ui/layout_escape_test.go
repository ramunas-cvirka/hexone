// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"hexone/fm"
)

func testKeyContext() (layout.Context, *input.Router) {
	router := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
	}
	return gtx, router
}

func TestGlobalEscapeLeavesViewerEscapeInTab0(t *testing.T) {
	ui := &UI{
		Tabs:       widget.Enum{Value: "tab0"},
		fileViewer: &fileViewerState{},
	}

	gtx, router := testKeyContext()
	router.Event(key.Filter{Name: key.NameEscape})
	router.Queue(key.Event{Name: key.NameEscape, State: key.Press})

	ui.handleGlobalEscapeToFileManager(gtx)
	if ui.fileViewer == nil {
		t.Fatal("global escape handler should not close viewer in tab0")
	}

	ui.handleFileViewerKeys(gtx)
	if ui.fileViewer != nil {
		t.Fatal("viewer should still receive escape after global handler")
	}
}

func TestGlobalEscapeClosesFunctionBarToolsInTab0(t *testing.T) {
	ui := &UI{
		Tabs:                 widget.Enum{Value: "tab0"},
		functionBarToolsOpen: true,
	}

	gtx, router := testKeyContext()
	router.Event(key.Filter{Name: key.NameEscape})
	router.Queue(key.Event{Name: key.NameEscape, State: key.Press})

	ui.handleGlobalEscapeToFileManager(gtx)
	if ui.functionBarToolsOpen {
		t.Fatal("escape should close function bar tools popup in tab0")
	}
}

func TestGlobalShortcutOpensSettings(t *testing.T) {
	ui := &UI{
		Tabs:  widget.Enum{Value: "tab0"},
		fmCfg: fm.DefaultConfig(),
	}

	gtx, router := testKeyContext()
	router.Event(key.Filter{Name: "S", Required: key.ModCtrl})
	router.Queue(key.Event{Name: "S", Modifiers: key.ModCtrl, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)
	if ui.settingsModal == nil {
		t.Fatal("ctrl+s should open settings")
	}
}

func TestGlobalShortcutOpensSettingsWhenTerminalFocused(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab0"
	ui.terminal = newTerminalSession(nil)
	ui.terminal.setActive(true)

	gtx, router := testKeyContext()
	gtx.Execute(key.FocusCmd{Tag: &ui.terminal.keyTag})
	if !ui.terminalFocused(gtx) {
		t.Fatal("terminal should be focused for this shortcut test")
	}
	anyMods := ^key.Modifiers(0)
	router.Event(key.Filter{Name: "S", Required: key.ModCtrl, Optional: anyMods})
	router.Queue(key.Event{Name: "S", Modifiers: key.ModCtrl, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)
	if ui.settingsModal == nil {
		t.Fatal("ctrl+s should open settings when terminal is focused")
	}
}

func TestShiftTabTogglesTerminalFocusWhenDrawerOpen(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab0"
	ui.terminal = newTerminalSession(nil)
	ui.terminal.setActive(true)

	gtx, router := testKeyContext()
	gtx.Execute(key.FocusCmd{Tag: &ui.terminal.keyTag})
	if !ui.terminalFocused(gtx) {
		t.Fatal("terminal should start focused")
	}

	anyMods := ^key.Modifiers(0)
	router.Event(key.Filter{Name: key.NameTab, Required: key.ModShift, Optional: anyMods})
	router.Queue(key.Event{Name: key.NameTab, Modifiers: key.ModShift, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)
	if ui.terminalFocused(gtx) {
		t.Fatal("Shift+Tab should return focus from terminal to file panes")
	}

	router.Event(key.Filter{Name: key.NameTab, Required: key.ModShift, Optional: anyMods})
	router.Queue(key.Event{Name: key.NameTab, Modifiers: key.ModShift, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)
	if !ui.terminalFocused(gtx) {
		t.Fatal("Shift+Tab should focus the open terminal from file panes")
	}
}

func TestF11ShowsFunctionBarWhenMaximizedTerminalFocused(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Terminal.Maximized = true
	ui := NewUI(cfg)
	ui.Tabs.Value = "tab0"
	ui.terminal = newTerminalSession(nil)
	ui.terminal.setActive(true)

	gtx, router := testKeyContext()
	gtx.Execute(key.FocusCmd{Tag: &ui.terminal.keyTag})
	if !ui.terminalFocused(gtx) {
		t.Fatal("terminal should start focused")
	}
	if ui.functionBarVisible() {
		t.Fatal("maximized terminal should initially auto-hide the function bar")
	}

	anyMods := ^key.Modifiers(0)
	router.Event(key.Filter{Name: key.NameF11, Optional: anyMods})
	router.Queue(key.Event{Name: key.NameF11, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)
	if !ui.functionBarVisible() {
		t.Fatal("F11 should show the function bar while maximized terminal is focused")
	}
	if !ui.functionBarTerminalShown {
		t.Fatal("F11 should set the terminal function-bar override")
	}
}

func TestPlainTabRemainsAvailableWhenTerminalFocused(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab0"
	ui.terminal = newTerminalSession(nil)
	ui.terminal.setActive(true)

	gtx, router := testKeyContext()
	gtx.Execute(key.FocusCmd{Tag: &ui.terminal.keyTag})
	if !ui.terminalFocused(gtx) {
		t.Fatal("terminal should start focused")
	}

	anyMods := ^key.Modifiers(0)
	router.Event(key.Filter{Name: key.NameTab, Optional: anyMods})
	router.Queue(key.Event{Name: key.NameTab, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)
	if !ui.terminalFocused(gtx) {
		t.Fatal("plain Tab should not move focus away from the terminal")
	}
	if _, ok := gtx.Event(key.Filter{Name: key.NameTab, Optional: anyMods}); !ok {
		t.Fatal("plain Tab should remain available for terminal input")
	}
}

func TestShiftTabIsNotStolenOutsideFilePanesWhenTerminalOpen(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab1"
	ui.terminal = newTerminalSession(nil)
	ui.terminal.setActive(true)

	gtx, router := testKeyContext()
	anyMods := ^key.Modifiers(0)
	router.Event(key.Filter{Name: key.NameTab, Required: key.ModShift, Optional: anyMods})
	router.Queue(key.Event{Name: key.NameTab, Modifiers: key.ModShift, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)
	if _, ok := gtx.Event(key.Filter{Name: key.NameTab, Required: key.ModShift, Optional: anyMods}); !ok {
		t.Fatal("Shift+Tab should remain available outside the file panes when terminal is not focused")
	}
}

func TestGlobalShortcutLeavesViewerCtrlFForViewer(t *testing.T) {
	ui := &UI{
		Tabs: widget.Enum{Value: "tab0"},
		fileViewer: &fileViewerState{
			resultCh: make(chan fileViewerResult, 1),
		},
	}

	gtx, router := testKeyContext()
	router.Event(key.Filter{Name: "F", Required: key.ModCtrl})
	router.Queue(key.Event{Name: "F", Modifiers: key.ModCtrl, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)
	ui.handleFileViewerKeys(gtx)

	if !ui.fileViewer.find.open {
		t.Fatal("ctrl+f should reach the viewer when it is open")
	}
}
