// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"path/filepath"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"hexone/fm"
)

func TestSaveCurrentCustomCommandKeepsEditorOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	if err := fm.SaveConfig(path, fm.DefaultConfig()); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	ui := NewUI(fm.DefaultConfig())
	ui.configPath = path
	ui.openCustomCommandEditor(-1)
	st := ui.customCommandEditor
	if st == nil {
		t.Fatal("custom command editor should open")
	}
	st.nameEdit.SetText("Health")
	st.commandEdit.SetText("uptime")

	if !ui.saveCurrentCustomCommand() {
		t.Fatalf("saveCurrentCustomCommand failed: %s", st.lastErr)
	}
	if ui.customCommandEditor == nil {
		t.Fatal("saving should keep the custom command editor open")
	}
	if ui.ConsumeWindowCloseRequest() {
		t.Fatal("saving should not request application exit")
	}

	saved := fm.LoadConfig(path)
	if got, want := len(saved.CustomCommands), 1; got != want {
		t.Fatalf("custom command count=%d want %d", got, want)
	}
	if got, want := saved.CustomCommands[0].Name, "Health"; got != want {
		t.Fatalf("custom command name=%q want %q", got, want)
	}
	if got, want := saved.CustomCommands[0].Slot, 1; got != want {
		t.Fatalf("custom command slot=%d want %d", got, want)
	}
	if st.lastErr != "" {
		t.Fatalf("successful save status=%q want empty", st.lastErr)
	}
	if st.slotDirty(0) {
		t.Fatal("slot dirty marker should clear after saving")
	}
}

func TestSaveEmptyCustomCommandClearsSelectedSlot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	cfg := fm.DefaultConfig()
	cfg.CustomCommands = []fm.CustomCommand{{Slot: 1, Name: "Health", Command: "uptime"}}
	if err := fm.SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	ui := NewUI(cfg)
	ui.configPath = path
	ui.openCustomCommandEditor(0)
	st := ui.customCommandEditor
	if st == nil {
		t.Fatal("custom command editor should open")
	}
	st.nameEdit.SetText("")
	st.commandEdit.SetText("")

	if !ui.saveCurrentCustomCommand() {
		t.Fatalf("saveCurrentCustomCommand failed: %s", st.lastErr)
	}
	if ui.customCommandEditor == nil {
		t.Fatal("saving an empty slot should keep the custom command editor open")
	}
	if st.lastErr != "" {
		t.Fatalf("successful empty save status=%q want empty", st.lastErr)
	}
	if st.slotDirty(0) {
		t.Fatal("cleared slot should not remain dirty after saving")
	}

	saved := fm.LoadConfig(path)
	if got := len(saved.CustomCommands); got != 0 {
		t.Fatalf("custom command count after clear=%d want 0", got)
	}
}

func TestCustomCommandSlotsPreserveSparseSlotNumbers(t *testing.T) {
	slots := customCommandSlots([]fm.CustomCommand{
		{Slot: 5, Name: "Ports", Command: "ss -tanp"},
	})
	if got, want := len(slots), 10; got != want {
		t.Fatalf("slot count=%d want %d", got, want)
	}
	if got, want := slots[4].Name, "Ports"; got != want {
		t.Fatalf("slot 5 name=%q want %q", got, want)
	}
	if got := slots[0].Command; got != "" {
		t.Fatalf("slot 1 command=%q want empty", got)
	}
}

func TestGlobalFixedCustomCommandShortcutOpensCommandViewer(t *testing.T) {
	ui := &UI{
		Tabs:  widget.Enum{Value: "tab0"},
		fmCfg: fm.DefaultConfig(),
	}
	ui.fmCfg.CustomCommands = []fm.CustomCommand{
		{Slot: 1, Name: "Health", Command: "echo health"},
	}

	gtx, router := testKeyContext()
	router.Event(key.Filter{Name: "1", Required: key.ModCtrl, Optional: ^key.Modifiers(0)})
	router.Queue(key.Event{Name: "1", Modifiers: key.ModCtrl | key.ModShortcut, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)
	if ui.fileViewer == nil {
		t.Fatal("Ctrl+1 should open the custom command viewer")
	}
	if !ui.fileViewer.commandOnly {
		t.Fatal("custom command shortcut should open command-only viewer")
	}
	if got, want := ui.fileViewer.command, "echo health"; got != want {
		t.Fatalf("viewer command=%q want %q", got, want)
	}
	ui.closeFileViewer()
}

func TestCustomCommandEditorFocusStepsThroughFieldsAndActions(t *testing.T) {
	st := &customCommandEditorState{
		focus:       customCommandEditorFocusCommand,
		actionFocus: customCommandEditorActionRun,
	}

	if !st.stepFocus(1) || st.focus != customCommandEditorFocusActions {
		t.Fatalf("Tab from command focus=%v want actions", st.focus)
	}
	if st.actionFocus != customCommandEditorActionSave {
		t.Fatalf("Tab into actions action=%v want Save", st.actionFocus)
	}
	if !st.stepAction(1) || st.actionFocus != customCommandEditorActionRun {
		t.Fatalf("right from Save action=%v want Run", st.actionFocus)
	}
	if !st.stepFocus(1) || st.focus != customCommandEditorFocusSlots {
		t.Fatalf("Tab from actions focus=%v want slots", st.focus)
	}
	if !st.stepFocus(1) || st.focus != customCommandEditorFocusName {
		t.Fatalf("Tab from slots focus=%v want name", st.focus)
	}
	if !st.stepFocus(1) || st.focus != customCommandEditorFocusCommand {
		t.Fatalf("Tab from name focus=%v want command", st.focus)
	}
	if !st.stepFocus(-1) || st.focus != customCommandEditorFocusName {
		t.Fatalf("Shift+Tab from command focus=%v want name", st.focus)
	}
	if !st.stepFocus(-1) || st.focus != customCommandEditorFocusSlots {
		t.Fatalf("Shift+Tab from name focus=%v want slots", st.focus)
	}
	if !st.stepFocus(-1) || st.focus != customCommandEditorFocusActions {
		t.Fatalf("Shift+Tab from slots focus=%v want actions", st.focus)
	}
}

func TestCustomCommandEditorKeyboardTabCycleMatchesVisualGroups(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openCustomCommandEditor(-1)
	st := ui.customCommandEditor
	if st == nil {
		t.Fatal("custom command editor should open")
	}

	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)
	tick := 0
	frame := func() {
		tick++
		gtx.Now = now.Add(time.Duration(tick) * time.Millisecond)
		gtx.Ops.Reset()
		ui.handleCustomCommandEditorPreLayoutInput(gtx)
		ui.layoutCustomCommandEditor(th, gtx)
		router.Frame(gtx.Ops)
	}
	press := func(name key.Name, mods key.Modifiers) {
		router.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
		frame()
		frame()
	}

	frame()
	frame()
	if st.focus != customCommandEditorFocusCommand {
		t.Fatalf("initial focus=%v want command", st.focus)
	}

	press(key.NameTab, 0)
	if st.focus != customCommandEditorFocusActions {
		t.Fatalf("Tab from command focus=%v want actions", st.focus)
	}
	if st.actionFocus != customCommandEditorActionSave {
		t.Fatalf("Tab into actions action=%v want Save", st.actionFocus)
	}

	press(key.NameRightArrow, 0)
	if st.actionFocus != customCommandEditorActionRun {
		t.Fatalf("Right from Save action=%v want Run", st.actionFocus)
	}

	press(key.NameTab, 0)
	if st.focus != customCommandEditorFocusSlots {
		t.Fatalf("Tab from actions focus=%v want slots", st.focus)
	}

	press(key.NameDownArrow, 0)
	if st.selected != 1 {
		t.Fatalf("Down in slots selected=%d want 1", st.selected)
	}

	press(key.NameTab, 0)
	if st.focus != customCommandEditorFocusName {
		t.Fatalf("Tab from slots focus=%v want name", st.focus)
	}

	press(key.NameTab, 0)
	if st.focus != customCommandEditorFocusCommand {
		t.Fatalf("Tab from name focus=%v want command", st.focus)
	}

	press(key.NameTab, key.ModShift)
	if st.focus != customCommandEditorFocusName {
		t.Fatalf("Shift+Tab from command focus=%v want name", st.focus)
	}

	press(key.NameTab, key.ModShift)
	if st.focus != customCommandEditorFocusSlots {
		t.Fatalf("Shift+Tab from name focus=%v want slots", st.focus)
	}

	press(key.NameTab, key.ModShift)
	if st.focus != customCommandEditorFocusActions {
		t.Fatalf("Shift+Tab from slots focus=%v want actions", st.focus)
	}
}

func TestCustomCommandEditorPreLayoutConsumesArrowKeys(t *testing.T) {
	ui := &UI{
		customCommandEditor: &customCommandEditorState{
			draft:    customCommandSlots(nil),
			saved:    customCommandSlots(nil),
			selected: 0,
			focus:    customCommandEditorFocusSlots,
		},
	}

	gtx, router := testKeyContext()
	anyMods := ^key.Modifiers(0)
	router.Event(key.Filter{Name: key.NameDownArrow, Optional: anyMods})
	router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})

	ui.handleCustomCommandEditorPreLayoutInput(gtx)
	if got, want := ui.customCommandEditor.selected, 1; got != want {
		t.Fatalf("selected slot=%d want %d", got, want)
	}
	if _, ok := gtx.Event(key.Filter{Name: key.NameDownArrow, Optional: anyMods}); ok {
		t.Fatal("custom command editor should consume arrow keys before file panes see them")
	}
}
