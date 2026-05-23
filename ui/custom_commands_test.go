// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"path/filepath"
	"testing"

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
	st.shortcutEdit.SetText("Ctrl+1")
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
}

func TestCustomCommandEditorFocusStepsThroughFieldsAndActions(t *testing.T) {
	st := &customCommandEditorState{
		focus:       customCommandEditorFocusCommand,
		actionFocus: customCommandEditorActionSave,
	}

	if !st.stepFocus(1) || st.focus != customCommandEditorFocusActions {
		t.Fatalf("Tab from command focus=%v want actions", st.focus)
	}
	if !st.stepAction(1) || st.actionFocus != customCommandEditorActionRun {
		t.Fatalf("right from Save action=%v want Run", st.actionFocus)
	}
	if !st.stepFocus(1) || st.focus != customCommandEditorFocusName {
		t.Fatalf("Tab from actions focus=%v want name", st.focus)
	}
	if !st.stepFocus(-1) || st.focus != customCommandEditorFocusActions {
		t.Fatalf("Shift+Tab from name focus=%v want actions", st.focus)
	}
}
