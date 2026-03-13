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
		Tabs: widget.Enum{Value: "tab0"},
	}

	gtx, router := testKeyContext()
	router.Event(key.Filter{Name: "S", Required: key.ModCtrl})
	router.Queue(key.Event{Name: "S", Modifiers: key.ModCtrl, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)
	if ui.settingsModal == nil {
		t.Fatal("ctrl+s should open settings")
	}
}
