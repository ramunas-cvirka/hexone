package ui

import (
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

	assertActive("files")

	ui.Tabs.Value = "tab1"
	assertActive("hex")

	ui.Tabs.Value = "tab2"
	assertActive("protocol")

	ui.settingsModal = &settingsModalState{}
	assertActive("settings")
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
