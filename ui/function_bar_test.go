// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"testing"
	"time"

	"gioui.org/widget"
)

func TestFunctionBarToolSpecsFollowActiveWorkspace(t *testing.T) {
	ui := &UI{}

	assertActive := func(want string) {
		t.Helper()
		items := ui.functionBarToolSpecs()
		got := ""
		for _, item := range items {
			if item.active {
				got = item.key
				break
			}
		}
		if got != want {
			t.Fatalf("active tool=%q want %q", got, want)
		}
	}

	assertActive("")

	ui.Tabs.Value = "tab1"
	assertActive("hex")

	ui.Tabs.Value = "tab2"
	assertActive("protocol")

	ui.settingsModal = &settingsModalState{}
	assertActive("settings")
}

func TestFunctionBarToolsExposeCompactShortcutHint(t *testing.T) {
	ui := &UI{}

	items := ui.functionBarToolSpecs()
	if len(items) != 3 {
		t.Fatalf("tool count=%d want 3", len(items))
	}
	if items[0].key == "files" {
		t.Fatal("redundant file-manager entry should not be present")
	}
	if items[2].key != "settings" {
		t.Fatalf("last tool=%q want settings", items[2].key)
	}
	if got := items[2].shortcut; got != "Ctrl+S" {
		t.Fatalf("settings shortcut=%q want %q", got, "Ctrl+S")
	}
}

func TestFunctionBarToolSpecsOmitFileManagerEntryInFilesTab(t *testing.T) {
	ui := &UI{}

	items := ui.functionBarToolSpecs()
	if len(items) == 0 {
		t.Fatal("function bar tool specs should not be empty")
	}
	for _, item := range items {
		if item.key == "files" {
			t.Fatal("file manager should not be listed in tools popup")
		}
	}
}

func TestFunctionBarExitRequestsWindowClose(t *testing.T) {
	ui := &UI{}
	if !ui.performFunctionBarAction(functionBarActionExit, time.Now()) {
		t.Fatal("exit action should be handled")
	}
	if !ui.ConsumeWindowCloseRequest() {
		t.Fatal("close request should be pending after exit action")
	}
	if ui.ConsumeWindowCloseRequest() {
		t.Fatal("close request should only be consumed once")
	}
}

func TestToggleFunctionBarVisibilityClosesToolsPopup(t *testing.T) {
	now := time.Date(2026, time.March, 8, 10, 0, 0, 0, time.UTC)
	ui := &UI{
		Tabs:                 widget.Enum{Value: "tab0"},
		functionBarToolsOpen: true,
		filePanes:            []*filePaneState{{}},
	}

	if !ui.toggleFunctionBarVisibility(now) {
		t.Fatal("toggleFunctionBarVisibility should succeed")
	}
	if !ui.functionBarHidden {
		t.Fatal("function bar should be hidden after toggle")
	}
	if ui.functionBarToolsOpen {
		t.Fatal("tools popup should close when the bar is hidden")
	}
	if got := ui.filePanes[0].noticeText; got == "" {
		t.Fatal("hiding the bar should leave a restore hint in the active pane notice")
	}
}

func TestViewerFunctionBarAutoHideCanBeTemporarilyShown(t *testing.T) {
	now := time.Date(2026, time.March, 8, 10, 5, 0, 0, time.UTC)
	ui := &UI{
		fmCfg:             fm.DefaultConfig(),
		functionBarHidden: true,
		fileViewer:        &fileViewerState{},
	}

	if ui.functionBarVisible() {
		t.Fatal("viewer should hide function bar by default when auto-hide is enabled")
	}
	if ui.functionBarViewerShown {
		t.Fatal("viewer override should start disabled")
	}

	if !ui.toggleFunctionBarVisibility(now) {
		t.Fatal("toggleFunctionBarVisibility should succeed")
	}
	if !ui.functionBarVisible() {
		t.Fatal("F11 should temporarily show the function bar in viewer mode")
	}
	if !ui.functionBarViewerShown {
		t.Fatal("viewer override should be enabled after showing the bar")
	}
	if !ui.functionBarHidden {
		t.Fatal("global manual hidden state should be preserved while viewer override is active")
	}

	ui.closeFileViewer()

	if ui.functionBarViewerShown {
		t.Fatal("viewer override should clear when viewer closes")
	}
	if ui.functionBarVisible() {
		t.Fatal("manual hidden state should resume after viewer closes")
	}
}

func TestFunctionBarToolsOpenSeedsKeyboardSelectionFromActiveTool(t *testing.T) {
	now := time.Date(2026, time.March, 11, 10, 0, 0, 0, time.UTC)
	ui := &UI{
		Tabs: widget.Enum{Value: "tab2"},
	}

	if !ui.performFunctionBarAction(functionBarActionTools, now) {
		t.Fatal("tools action should open the popup")
	}
	if !ui.functionBarToolsOpen {
		t.Fatal("tools popup should be open")
	}
	if got := ui.functionBarToolsSelected; got != 1 {
		t.Fatalf("selected tool=%d want 1 for protocol analyzer", got)
	}
}

func TestFunctionBarToolKeyboardSelectionWrapsAndActivates(t *testing.T) {
	now := time.Date(2026, time.March, 11, 10, 5, 0, 0, time.UTC)
	ui := &UI{
		Tabs: widget.Enum{Value: "tab0"},
	}

	if !ui.performFunctionBarAction(functionBarActionTools, now) {
		t.Fatal("tools action should open the popup")
	}
	if !ui.moveFunctionBarToolSelection(-1) {
		t.Fatal("up should wrap selection to the last tool")
	}
	if got := ui.functionBarToolsSelected; got != 2 {
		t.Fatalf("selected tool=%d want 2", got)
	}
	if !ui.activateSelectedFunctionBarTool(now) {
		t.Fatal("enter should activate the selected tool")
	}
	if ui.functionBarToolsOpen {
		t.Fatal("tools popup should close after activation")
	}
	if ui.settingsModal == nil {
		t.Fatal("activating the Settings tool should open the settings modal")
	}
}
