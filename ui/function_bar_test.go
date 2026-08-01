// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"image/color"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"hexone/fm"
)

func TestFunctionBarKeyTextUsesDistinctHighContrastColor(t *testing.T) {
	barBackground := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
	labelColor := color.NRGBA{R: 210, G: 210, B: 210, A: 255}
	keyColor := functionBarKeyTextColor(labelColor)

	if keyColor == labelColor {
		t.Fatal("function-key and action labels should use different colors")
	}
	if got := contrastScore(barBackground, keyColor); got < 7 {
		t.Fatalf("function-key contrast ratio=%0.2f want at least 7", got)
	}
	if keyColor.A != labelColor.A {
		t.Fatalf("function-key alpha=%d want label alpha %d", keyColor.A, labelColor.A)
	}
}

func TestFunctionBarSplitLabelUsesBoldShortcutAndNormalAction(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	labelColor := color.NRGBA{R: 210, G: 210, B: 210, A: 255}

	shortcut, _, action := ui.functionBarSplitLabelStyles(th, "Ctrl+A", "Select All", labelColor)

	if shortcut.Font.Weight != font.Bold {
		t.Fatalf("shortcut weight=%v want bold", shortcut.Font.Weight)
	}
	if action.Font.Weight != font.Normal {
		t.Fatalf("action weight=%v want normal", action.Font.Weight)
	}
	if shortcut.Color != functionBarKeyTextColor(labelColor) {
		t.Fatalf("shortcut color=%v want accent %v", shortcut.Color, functionBarKeyTextColor(labelColor))
	}
	if action.Color != labelColor {
		t.Fatalf("action color=%v want label color %v", action.Color, labelColor)
	}
	if action.MaxLines != 1 || action.Truncator != "…" {
		t.Fatalf("action truncation MaxLines=%d Truncator=%q want one-line ellipsis", action.MaxLines, action.Truncator)
	}
}

func TestFunctionBarWidthsAreFixedByWindowWidth(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = &fileViewerState{mode: "file"}
	th := material.NewTheme()
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(1003, 24)),
	}
	short := ui.viewerFunctionBarButtonSpecs()
	long := append([]functionBarButtonSpec(nil), short...)
	long[2].label = "very-long-descriptive-text-bla-blabla"

	shortWidths := ui.functionBarWidths(th, gtx, short)
	longWidths := ui.functionBarWidths(th, gtx, long)
	if !reflect.DeepEqual(shortWidths, longWidths) {
		t.Fatalf("label changed slot widths: short=%v long=%v", shortWidths, longWidths)
	}
	total := 0
	for i, width := range shortWidths {
		total += width
		if width < 100 || width > 101 {
			t.Fatalf("slot %d width=%d want equal 100/101px partition", i, width)
		}
	}
	if total != 1003 {
		t.Fatalf("slot widths total=%d want window width 1003", total)
	}
}

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

	ui.Tabs.Value = "tab3"
	assertActive("http")

	ui.settingsModal = &settingsModalState{}
	assertActive("settings")
}

func TestFunctionBarToolsExposeCompactShortcutHint(t *testing.T) {
	ui := &UI{}

	items := ui.functionBarToolSpecs()
	if len(items) != 6 {
		t.Fatalf("tool count=%d want 6", len(items))
	}
	if items[0].key == "files" {
		t.Fatal("redundant file-manager entry should not be present")
	}
	if items[0].key != "multi-rename" || items[0].shortcut != "Ctrl+M" {
		t.Fatalf("first tool=%#v want multi-rename with Ctrl+M", items[0])
	}
	if items[1].key != "ssh" || items[1].shortcut != "Ctrl+F" {
		t.Fatalf("second tool=%#v want SSH Setup with Ctrl+F", items[1])
	}
	if items[4].key != "http" {
		t.Fatalf("fifth tool=%q want HTTP Client", items[4].key)
	}
	if items[5].key != "settings" {
		t.Fatalf("last tool=%q want settings", items[5].key)
	}
	if got := items[5].shortcut; got != "Ctrl+S" {
		t.Fatalf("settings shortcut=%q want %q", got, "Ctrl+S")
	}
}

func TestFunctionBarSSHSetupToolOpensSSHModal(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.activateFunctionBarTool("ssh", time.Now())
	if ui.sshModal == nil {
		t.Fatal("SSH Setup tool should open the SSH modal")
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

func TestViewerFunctionBarExitRequestsApplicationExit(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab0"
	ui.fileViewer = &fileViewerState{mode: "file"}

	if !ui.performFunctionBarAction(functionBarActionExit, time.Now()) {
		t.Fatal("viewer F10 Exit action should be handled")
	}
	if ui.fileViewer == nil {
		t.Fatal("application Exit should leave viewer state intact until the window closes")
	}
	if !ui.ConsumeWindowCloseRequest() {
		t.Fatal("viewer F10 Exit should request application exit")
	}
}

func TestViewerFunctionBarExitDiscardsUnsavedTextChanges(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab0"
	st := &fileViewerState{
		mode:             "file",
		editMode:         true,
		editDirty:        true,
		editBaselineText: "original",
		content:          "changed",
	}
	st.contentEditor.SetText("changed")
	ui.fileViewer = st

	if !ui.performFunctionBarAction(functionBarActionExit, time.Now()) {
		t.Fatal("viewer F10 Exit action should be handled")
	}
	if st.editMode || st.editDirty || st.contentEditor.Text() != "original" || st.content != "original" {
		t.Fatalf("F10 text discard edit=%v dirty=%v editor=%q content=%q", st.editMode, st.editDirty, st.contentEditor.Text(), st.content)
	}
	if !ui.ConsumeWindowCloseRequest() {
		t.Fatal("viewer F10 should request application exit after discarding text changes")
	}
}

func TestViewerFunctionBarExitDiscardsUnsavedHexChanges(t *testing.T) {
	v := newHexViewerState()
	v.edits = map[int64]byte{3: 0xFF}
	st := &fileViewerState{mode: "hex", editMode: true, editDirty: true, hex: v}
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab0"
	ui.fileViewer = st

	if !ui.performFunctionBarAction(functionBarActionExit, time.Now()) {
		t.Fatal("viewer F10 Exit action should be handled")
	}
	if st.editMode || st.editDirty || v.edits != nil {
		t.Fatalf("F10 HEX discard edit=%v dirty=%v edits=%v", st.editMode, st.editDirty, v.edits)
	}
	if !ui.ConsumeWindowCloseRequest() {
		t.Fatal("viewer F10 should request application exit after discarding HEX changes")
	}
}

func TestViewerFunctionBarEnablesEditorActions(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab0"
	st := &fileViewerState{mode: "file"}
	ui.fileViewer = st

	if ui.functionBarActionEnabled(functionBarActionViewerSave) {
		t.Fatal("viewer Save should be disabled outside edit mode")
	}
	st.editMode = true
	st.editDirty = true
	if !ui.functionBarActionEnabled(functionBarActionViewerSave) {
		t.Fatal("viewer Save should be enabled for dirty edits")
	}
	if !ui.functionBarActionEnabled(functionBarActionViewerFind) {
		t.Fatal("viewer Find should remain enabled while editing")
	}
	if !ui.functionBarActionEnabled(functionBarActionViewerMode) {
		t.Fatal("viewer mode switch should remain enabled while editing")
	}
}

func TestViewerEditorFunctionKeysRouteFindAndModeSwitch(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab0"
	st := &fileViewerState{
		mode:             "file",
		path:             "notes.txt",
		content:          "committed needle",
		editMode:         true,
		editDirty:        true,
		editBaselineText: "committed needle",
	}
	st.stream.SetContent(st.content)
	ui.fileViewer = st
	gtx, router := testKeyContext()
	anyMods := ^key.Modifiers(0)

	router.Event(key.Filter{Name: key.NameF7, Optional: anyMods})
	router.Queue(key.Event{Name: key.NameF7, State: key.Press})
	ui.handleGlobalFunctionKeys(gtx)
	if !st.find.open {
		t.Fatal("viewer F7 should open Find while editing")
	}

	router.Event(key.Filter{Name: key.NameF8, Optional: anyMods})
	router.Queue(key.Event{Name: key.NameF8, State: key.Press})
	ui.handleGlobalFunctionKeys(gtx)
	if !st.modeSwitchPrompt.open || st.modeSwitchPrompt.targetMode != "hex" {
		t.Fatalf("viewer F8 prompt=%v target=%q want dirty-edit switch to hex", st.modeSwitchPrompt.open, st.modeSwitchPrompt.targetMode)
	}
}

func TestViewerFunctionKeysRouteToViewerCommands(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab0"
	ui.fileViewer = &fileViewerState{mode: "file"}
	gtx, router := testKeyContext()
	anyMods := ^key.Modifiers(0)

	router.Event(key.Filter{Name: key.NameF7, Optional: anyMods})
	router.Queue(key.Event{Name: key.NameF7, State: key.Press})
	ui.handleGlobalFunctionKeys(gtx)
	if !ui.fileViewer.find.open {
		t.Fatal("viewer F7 should open Find")
	}

	router.Event(key.Filter{Name: key.NameF10, Optional: anyMods})
	router.Queue(key.Event{Name: key.NameF10, State: key.Press})
	ui.handleGlobalFunctionKeys(gtx)
	if ui.fileViewer == nil {
		t.Fatal("viewer F10 should preserve viewer state until application exit")
	}
	if !ui.ConsumeWindowCloseRequest() {
		t.Fatal("viewer F10 should request application exit")
	}
}

func TestViewerF5TogglesLineNumbersAndF6IsUnassigned(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.configPath = filepath.Join(t.TempDir(), "hexone.yaml")
	ui.Tabs.Value = "tab0"
	st := &fileViewerState{mode: "file", editMode: true, editDirty: true}
	ui.fileViewer = st
	gtx, router := testKeyContext()
	anyMods := ^key.Modifiers(0)

	router.Event(key.Filter{Name: key.NameF5, Optional: anyMods})
	router.Queue(key.Event{Name: key.NameF5, State: key.Press})
	ui.handleGlobalFunctionKeys(gtx)
	if ui.fmCfg.Viewer.ShowLineNumbers {
		t.Fatal("viewer F5 should disable line numbers")
	}

	router.Event(key.Filter{Name: key.NameF6, Optional: anyMods})
	router.Queue(key.Event{Name: key.NameF6, State: key.Press})
	ui.handleGlobalFunctionKeys(gtx)
	if st.saving || !st.editMode || !st.editDirty {
		t.Fatalf("F5/F6 changed edit state saving=%v edit=%v dirty=%v", st.saving, st.editMode, st.editDirty)
	}
	if ui.ConsumeWindowCloseRequest() {
		t.Fatal("F5/F6 should not request application exit")
	}
}

func TestTerminalMaximizedFunctionBarAutoHideCanBeTemporarilyShown(t *testing.T) {
	now := time.Date(2026, time.March, 8, 10, 5, 0, 0, time.UTC)
	cfg := fm.DefaultConfig()
	cfg.Terminal.Maximized = true
	st := newTerminalSession(nil)
	st.setActive(true)
	ui := &UI{
		fmCfg:             cfg,
		terminal:          st,
		functionBarHidden: true,
	}

	if ui.functionBarVisible() {
		t.Fatal("maximized terminal should hide function bar by default")
	}
	if ui.functionBarTerminalShown {
		t.Fatal("terminal override should start disabled")
	}

	if !ui.toggleFunctionBarVisibility(now) {
		t.Fatal("toggleFunctionBarVisibility should succeed")
	}
	if !ui.functionBarVisible() {
		t.Fatal("F11 should temporarily show the function bar over maximized terminal")
	}
	if !ui.functionBarTerminalShown {
		t.Fatal("terminal override should be enabled after showing the bar")
	}
	if !ui.functionBarHidden {
		t.Fatal("global manual hidden state should be preserved while terminal override is active")
	}

	cfg.Terminal.Maximized = false
	if ui.functionBarVisible() {
		t.Fatal("manual hidden state should resume after terminal leaves maximized mode")
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
	if got := ui.functionBarToolsSelected; got != 3 {
		t.Fatalf("selected tool=%d want 3 for protocol analyzer", got)
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
	if got := ui.functionBarToolsSelected; got != 5 {
		t.Fatalf("selected tool=%d want 5", got)
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
	want := "Ctrl+A Select All | Ctrl+E Same Ext | Ctrl+N New Tab | Ctrl+X Close Tab | Ctrl+F SSH | Ctrl+M Multi-Rename | Ctrl+S Settings"
	if got != want {
		t.Fatalf("functionBarModifierHintText()=%q want %q", got, want)
	}
}

func TestFunctionBarModifierHintTextShowsFocusedTerminalShortcuts(t *testing.T) {
	ui := &UI{
		Tabs:                widget.Enum{Value: "tab0"},
		functionBarHeldMods: key.ModCtrl,
	}

	got, ok := ui.functionBarModifierHintTextForContext(true, "linux")
	if !ok {
		t.Fatal("expected ctrl hints for the focused terminal")
	}
	want := "Ctrl+A Select All | Ctrl+Shift+K Clear | Ctrl+N New Tab | Ctrl+X Close Tab | Ctrl+S Settings"
	if got != want {
		t.Fatalf("functionBarModifierHintTextForContext()=%q want %q", got, want)
	}
}

func TestFunctionBarModifierHintTextUsesMacTerminalClearShortcut(t *testing.T) {
	ui := &UI{
		Tabs:                widget.Enum{Value: "tab0"},
		functionBarHeldMods: key.ModCommand,
	}

	got, ok := ui.functionBarModifierHintTextForContext(true, "darwin")
	if !ok {
		t.Fatal("expected command hints for the focused terminal")
	}
	want := "Cmd+A Select All | Cmd+K Clear | Cmd+N New Tab | Cmd+X Close Tab | Cmd+S Settings"
	if got != want {
		t.Fatalf("functionBarModifierHintTextForContext()=%q want %q", got, want)
	}
}

func TestFunctionBarModifierHintTextDoesNotAdvertiseMacClearForCtrl(t *testing.T) {
	ui := &UI{
		Tabs:                widget.Enum{Value: "tab0"},
		functionBarHeldMods: key.ModCtrl,
	}

	got, ok := ui.functionBarModifierHintTextForContext(true, "darwin")
	if !ok {
		t.Fatal("expected ctrl hints for the focused terminal")
	}
	want := "Ctrl+A Select All | Ctrl+N New Tab | Ctrl+X Close Tab | Ctrl+S Settings"
	if got != want {
		t.Fatalf("functionBarModifierHintTextForContext()=%q want %q", got, want)
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
	want := "Ctrl+F Find | Ctrl+C Copy | Ctrl+A Select All | Ctrl+S Save"
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
	want := "Ctrl+/- Zoom | Ctrl+S Save"
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
	if labels[2] != "Ctrl+N New Tab" {
		t.Fatalf("slot 2=%q want %q", labels[2], "Ctrl+N New Tab")
	}
	if labels[3] != "Ctrl+X Close Tab" {
		t.Fatalf("slot 3=%q want %q", labels[3], "Ctrl+X Close Tab")
	}
	if labels[4] != "Ctrl+F SSH" {
		t.Fatalf("slot 4=%q want %q", labels[4], "Ctrl+F SSH")
	}
	if labels[5] != "Ctrl+M Multi-Rename" {
		t.Fatalf("slot 5=%q want %q", labels[5], "Ctrl+M Multi-Rename")
	}
	if labels[6] != "Ctrl+S Settings" {
		t.Fatalf("slot 6=%q want %q", labels[6], "Ctrl+S Settings")
	}
	for i := 7; i < len(labels); i++ {
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
