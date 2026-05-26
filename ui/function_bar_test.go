// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"testing"
	"time"

	"gioui.org/io/key"
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

func TestFunctionBarCustomMenuSpecsStartWithEditorAndSavedCommands(t *testing.T) {
	ui := &UI{fmCfg: fm.DefaultConfig()}
	ui.fmCfg.CustomCommands = []fm.CustomCommand{
		{Name: "Health", Command: "uptime"},
		{Slot: 3, Name: "Ports", Command: "ss -tanp"},
	}

	items := ui.customCommandMenuSpecs()
	if len(items) != 3 {
		t.Fatalf("custom command item count=%d want 3", len(items))
	}
	if !items[0].editor || items[0].label != "Custom command editor" {
		t.Fatalf("first item=%#v want editor", items[0])
	}
	if got, want := items[1].label, "Health"; got != want {
		t.Fatalf("first command label=%q want %q", got, want)
	}
	if got, want := items[1].shortcut, "Ctrl+1"; got != want {
		t.Fatalf("first command shortcut=%q want %q", got, want)
	}
	if got, want := items[2].shortcut, "Ctrl+3"; got != want {
		t.Fatalf("fixed slot shortcut=%q want %q", got, want)
	}
}

func TestFunctionBarCustomActionOpensMenuAndClosesTools(t *testing.T) {
	now := time.Date(2026, time.March, 11, 11, 0, 0, 0, time.UTC)
	ui := &UI{
		Tabs:                 widget.Enum{Value: "tab0"},
		fmCfg:                fm.DefaultConfig(),
		functionBarToolsOpen: true,
	}

	if !ui.performFunctionBarAction(functionBarActionCustom, now) {
		t.Fatal("custom action should open the popup")
	}
	if !ui.customCommandMenuOpen {
		t.Fatal("custom command popup should be open")
	}
	if ui.functionBarToolsOpen {
		t.Fatal("opening F2 menu should close the tools popup")
	}
	if got := ui.customCommandMenuSelected; got != 0 {
		t.Fatalf("selected custom item=%d want editor at 0", got)
	}
}

func TestCustomCommandShortcutSlotUsesFixedNumberBindings(t *testing.T) {
	slot, ok := customCommandShortcutSlot(key.Event{Name: "3", State: key.Press, Modifiers: key.ModCtrl | key.ModShortcut})
	if !ok || slot != 2 {
		t.Fatalf("Ctrl+3 slot=%d ok=%v want slot 2", slot, ok)
	}
	if _, ok := customCommandShortcutSlot(key.Event{Name: "p", State: key.Press, Modifiers: key.ModCtrl}); ok {
		t.Fatal("custom letter shortcuts should not be accepted")
	}
	if _, ok := customCommandShortcutSlot(key.Event{Name: "3", State: key.Press, Modifiers: key.ModCtrl | key.ModAlt}); ok {
		t.Fatal("custom command shortcuts should not accept extra modifiers")
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

func TestFunctionBarExitDisabledWhileCustomCommandEditorOpen(t *testing.T) {
	ui := &UI{customCommandEditor: &customCommandEditorState{}}
	if ui.functionBarActionEnabled(functionBarActionExit) {
		t.Fatal("exit should be disabled while the custom command editor is open")
	}
	if ui.performFunctionBarAction(functionBarActionExit, time.Now()) {
		t.Fatal("exit action should not run while the custom command editor is open")
	}
	if ui.ConsumeWindowCloseRequest() {
		t.Fatal("disabled exit should not request window close")
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
		Tabs:  widget.Enum{Value: "tab0"},
		fmCfg: fm.DefaultConfig(),
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

func TestFunctionBarModifierHintTextShowsFileManagerShortcuts(t *testing.T) {
	ui := &UI{
		Tabs:                widget.Enum{Value: "tab0"},
		functionBarHeldMods: key.ModCtrl,
	}

	got, ok := ui.functionBarModifierHintText()
	if !ok {
		t.Fatal("expected ctrl hints for the file manager")
	}
	want := "Ctrl+A Select All | Ctrl+E Same Ext | Ctrl+F SSH | Ctrl+S Settings"
	if got != want {
		t.Fatalf("functionBarModifierHintText()=%q want %q", got, want)
	}
}

func TestFunctionBarModifierHintTextShowsViewerTextShortcuts(t *testing.T) {
	ui := &UI{
		Tabs:                widget.Enum{Value: "tab0"},
		functionBarHeldMods: key.ModCtrl,
		fileViewer: &fileViewerState{
			mode: "file",
		},
	}

	got, ok := ui.functionBarModifierHintText()
	if !ok {
		t.Fatal("expected ctrl hints for the viewer")
	}
	want := "Ctrl+F Find | Ctrl+C Copy | Ctrl+A Select All | Ctrl+S Settings"
	if got != want {
		t.Fatalf("functionBarModifierHintText()=%q want %q", got, want)
	}
}

func TestFunctionBarModifierHintTextShowsViewerImageShortcuts(t *testing.T) {
	ui := &UI{
		Tabs:                widget.Enum{Value: "tab0"},
		functionBarHeldMods: key.ModCtrl,
		fileViewer: &fileViewerState{
			mode:                 "file",
			detectedImagePreview: true,
		},
	}

	got, ok := ui.functionBarModifierHintText()
	if !ok {
		t.Fatal("expected ctrl hints for image previews")
	}
	want := "Ctrl+/- Zoom | Ctrl+S Settings"
	if got != want {
		t.Fatalf("functionBarModifierHintText()=%q want %q", got, want)
	}
}

func TestFunctionBarModifierHintTextUsesCmdLabelWhenCommandHeld(t *testing.T) {
	ui := &UI{
		Tabs:                widget.Enum{Value: "tab1"},
		functionBarHeldMods: key.ModCommand,
	}

	got, ok := ui.functionBarModifierHintText()
	if !ok {
		t.Fatal("expected cmd hints when command is held")
	}
	want := "Cmd+S Settings"
	if got != want {
		t.Fatalf("functionBarModifierHintText()=%q want %q", got, want)
	}
}

func TestFunctionBarModifierHintTextShowsFileManagerAltShortcuts(t *testing.T) {
	ui := &UI{
		Tabs:                widget.Enum{Value: "tab0"},
		functionBarHeldMods: key.ModAlt,
	}

	got, ok := ui.functionBarModifierHintText()
	if !ok {
		t.Fatal("expected alt hints for the file manager")
	}
	want := "Alt+1 Left Drive | Alt+2 Right Drive"
	if got != want {
		t.Fatalf("functionBarModifierHintText()=%q want %q", got, want)
	}
}

func TestFunctionBarHintSlotLabelsUseLeadingFunctionBarSlots(t *testing.T) {
	ui := &UI{
		Tabs:                widget.Enum{Value: "tab0"},
		functionBarHeldMods: key.ModCtrl,
	}

	labels := ui.functionBarHintSlotLabels(10)
	if len(labels) != 10 {
		t.Fatalf("slot count=%d want 10", len(labels))
	}
	if labels[0] != "Ctrl+A Select All" {
		t.Fatalf("slot 0=%q want %q", labels[0], "Ctrl+A Select All")
	}
	if labels[1] != "Ctrl+E Same Ext" {
		t.Fatalf("slot 1=%q want %q", labels[1], "Ctrl+E Same Ext")
	}
	if labels[2] != "Ctrl+F SSH" {
		t.Fatalf("slot 2=%q want %q", labels[2], "Ctrl+F SSH")
	}
	if labels[3] != "Ctrl+S Settings" {
		t.Fatalf("slot 3=%q want %q", labels[3], "Ctrl+S Settings")
	}
	for i := 4; i < len(labels); i++ {
		if labels[i] != "" {
			t.Fatalf("slot %d=%q want empty", i, labels[i])
		}
	}
}

func TestFunctionBarHintSlotLabelsUseLeadingSlotsForAltShortcuts(t *testing.T) {
	ui := &UI{
		Tabs:                widget.Enum{Value: "tab0"},
		functionBarHeldMods: key.ModAlt,
	}

	labels := ui.functionBarHintSlotLabels(10)
	if len(labels) != 10 {
		t.Fatalf("slot count=%d want 10", len(labels))
	}
	if labels[0] != "Alt+1 Left Drive" {
		t.Fatalf("slot 0=%q want %q", labels[0], "Alt+1 Left Drive")
	}
	if labels[1] != "Alt+2 Right Drive" {
		t.Fatalf("slot 1=%q want %q", labels[1], "Alt+2 Right Drive")
	}
	for i := 2; i < len(labels); i++ {
		if labels[i] != "" {
			t.Fatalf("slot %d=%q want empty", i, labels[i])
		}
	}
}

func TestHandleFunctionBarModifierKeysTracksHeldCtrl(t *testing.T) {
	ui := &UI{}

	gtx, router := testKeyContext()
	router.Event(key.Filter{Name: key.NameCtrl})
	router.Queue(key.Event{Name: key.NameCtrl, State: key.Press})

	ui.handleFunctionBarModifierKeys(gtx)
	if !ui.functionBarHeldMods.Contain(key.ModCtrl) {
		t.Fatal("ctrl press should mark the function bar modifier as held")
	}

	router.Event(key.Filter{Name: key.NameCtrl})
	router.Queue(key.Event{Name: key.NameCtrl, State: key.Release})

	ui.handleFunctionBarModifierKeys(gtx)
	if ui.functionBarHeldMods.Contain(key.ModCtrl) {
		t.Fatal("ctrl release should clear the function bar modifier state")
	}
}

func TestSyncPlatformAltHeldClearsStuckAltModifier(t *testing.T) {
	ui := &UI{
		functionBarHeldMods: key.ModAlt,
	}

	if !ui.SyncPlatformAltHeld(false) {
		t.Fatal("SyncPlatformAltHeld should report a change when clearing Alt")
	}
	if ui.functionBarHeldMods.Contain(key.ModAlt) {
		t.Fatal("platform Alt sync should clear the held Alt modifier")
	}
	if ui.SyncPlatformAltHeld(false) {
		t.Fatal("SyncPlatformAltHeld should be a no-op when Alt is already cleared")
	}
}
