// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func newMultiRenameKeyboardTestLayout() (*UI, *multiRenameState, *input.Router, func()) {
	ui := NewUI(fm.DefaultConfig())
	st := &multiRenameState{
		targets: []multiRenameTarget{{oldName: "one.txt", newName: "one.txt", kind: filesys.EntryFile}},
		focus:   multiRenameFocusFind, actionFocus: multiRenameActionRename, focusWant: true,
	}
	for _, ed := range []*widget.Editor{&st.searchEdit, &st.replaceEdit, &st.prefixEdit, &st.suffixEdit, &st.startEdit, &st.stepEdit, &st.digitsEdit} {
		ed.SingleLine = true
		ed.Submit = true
	}
	st.startEdit.SetText("1")
	st.stepEdit.SetText("1")
	st.digitsEdit.SetText("2")
	ui.multiRename = st
	th := material.NewTheme()
	router := new(input.Router)
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(1024, 720)},
	}
	frame := func() {
		gtx.Ops.Reset()
		ui.handleMultiRenamePreLayoutInput(gtx)
		ui.layoutMultiRename(th, gtx)
		router.Frame(gtx.Ops)
	}
	return ui, st, router, frame
}

func TestMultiRenameTabOrderWrapsAndSkipsDisabledCounterFields(t *testing.T) {
	_, st, router, frame := newMultiRenameKeyboardTestLayout()
	frame()
	frame()
	if st.focus != multiRenameFocusFind {
		t.Fatalf("initial focus=%v want Find", st.focus)
	}
	want := []multiRenameFocus{
		multiRenameFocusReplace,
		multiRenameFocusPrefix,
		multiRenameFocusSuffix,
		multiRenameFocusScope,
		multiRenameFocusCase,
		multiRenameFocusCaseSensitive,
		multiRenameFocusCounter,
		multiRenameFocusActions,
		multiRenameFocusFind,
	}
	for i, target := range want {
		router.Queue(key.Event{Name: key.NameTab, State: key.Press})
		frame()
		if st.focus != target {
			t.Fatalf("focus after Tab %d=%v want %v", i+1, st.focus, target)
		}
	}
	router.Queue(key.Event{Name: key.NameTab, Modifiers: key.ModShift, State: key.Press})
	frame()
	if st.focus != multiRenameFocusActions {
		t.Fatalf("focus after wrapped Shift+Tab=%v want Actions", st.focus)
	}
}

func TestMultiRenameTabOrderIncludesEnabledCounterFields(t *testing.T) {
	_, st, router, frame := newMultiRenameKeyboardTestLayout()
	st.sequence.Value = true
	frame()
	frame()
	for i := 0; i < 7; i++ {
		router.Queue(key.Event{Name: key.NameTab, State: key.Press})
		frame()
	}
	if st.focus != multiRenameFocusCounter {
		t.Fatalf("focus=%v want Counter before its settings", st.focus)
	}
	want := []multiRenameFocus{
		multiRenameFocusCounterStart,
		multiRenameFocusCounterStep,
		multiRenameFocusCounterDigits,
		multiRenameFocusCounterPosition,
		multiRenameFocusActions,
	}
	for i, target := range want {
		router.Queue(key.Event{Name: key.NameTab, State: key.Press})
		frame()
		if st.focus != target {
			t.Fatalf("counter focus after Tab %d=%v want %v", i+1, st.focus, target)
		}
	}
}

func TestMultiRenameArrowKeysChangeFocusedTabGroup(t *testing.T) {
	_, st, router, frame := newMultiRenameKeyboardTestLayout()
	frame()
	frame()
	for i := 0; i < 4; i++ {
		router.Queue(key.Event{Name: key.NameTab, State: key.Press})
		frame()
	}
	if st.focus != multiRenameFocusScope || st.scope != multiRenameScopeName {
		t.Fatalf("focus=%v scope=%v want Name scope focused", st.focus, st.scope)
	}
	router.Queue(key.Event{Name: key.NameRightArrow, State: key.Press})
	frame()
	if st.scope != multiRenameScopeExtension {
		t.Fatalf("scope after Right=%v want Extension", st.scope)
	}
	router.Queue(key.Event{Name: key.NameLeftArrow, State: key.Press})
	frame()
	if st.scope != multiRenameScopeName {
		t.Fatalf("scope after Left=%v want Name", st.scope)
	}
}

func TestMultiRenameKeyboardCanSelectAndActivateCancel(t *testing.T) {
	ui, st, router, frame := newMultiRenameKeyboardTestLayout()
	frame()
	frame()
	router.Queue(key.Event{Name: key.NameTab, Modifiers: key.ModShift, State: key.Press})
	frame()
	if st.focus != multiRenameFocusActions || st.actionFocus != multiRenameActionRename {
		t.Fatalf("footer focus=%v action=%v want Rename", st.focus, st.actionFocus)
	}
	router.Queue(key.Event{Name: key.NameLeftArrow, State: key.Press})
	frame()
	if st.actionFocus != multiRenameActionCancel {
		t.Fatalf("action after Left=%v want Cancel", st.actionFocus)
	}
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame()
	if ui.multiRename != nil {
		t.Fatal("Enter on focused Cancel should close Multi-Rename")
	}
}

func TestStartMultiRenameRequiresAFilePaneSelection(t *testing.T) {
	pane := newFilePaneState(t.TempDir(), fm.DefaultConfig())
	pane.applyListing(filesys.Listing{Dir: pane.dir}, "", "", 0)
	ui := &UI{filePanes: []*filePaneState{pane}, activeFilePane: 0}

	if ui.startMultiRename(0, time.Now()) {
		t.Fatal("multi-rename should not open without a selected file")
	}
	if ui.multiRename != nil {
		t.Fatal("multi-rename state should remain closed")
	}
	if pane.noticeText == "" {
		t.Fatal("missing-selection attempt should explain why it did not open")
	}
}

func TestMultiRenameApplyKeepsExtensionByDefault(t *testing.T) {
	got := multiRenameApply(
		"Quarterly REPORT.xlsx", filesys.EntryFile,
		"report", "summary", "2026-", "-final",
		false, multiRenameScopeName, false, false, 1, 2, multiRenameCaseLower,
	)
	if want := "2026-quarterly summary-final.xlsx"; got != want {
		t.Fatalf("renamed filename=%q want %q", got, want)
	}
}

func TestMultiRenameApplyCanIncludeExtensionAndSequence(t *testing.T) {
	got := multiRenameApply(
		"Photo.JPG", filesys.EntryFile,
		"jpg", "jpeg", "", "",
		false, multiRenameScopeBoth, true, false, 7, 3, multiRenameCaseUpper,
	)
	if want := "007PHOTO.JPEG"; got != want {
		t.Fatalf("renamed filename=%q want %q", got, want)
	}
}

func TestMultiRenameCounterCanFollowNameBeforePreservedExtension(t *testing.T) {
	got := multiRenameApply(
		"photo.jpg", filesys.EntryFile,
		"", "", "", "",
		false, multiRenameScopeName, true, true, 5, 2, multiRenameCaseKeep,
	)
	if want := "photo05.jpg"; got != want {
		t.Fatalf("renamed filename=%q want %q", got, want)
	}
}

func TestMultiRenameCanApplyActionsOnlyToExtension(t *testing.T) {
	got := multiRenameApply(
		"report.TXT", filesys.EntryFile,
		"txt", "md", "archived-", "", false,
		multiRenameScopeExtension, false, false, 1, 2, multiRenameCaseLower,
	)
	if want := "report.archived-md"; got != want {
		t.Fatalf("renamed filename=%q want %q", got, want)
	}
}

func TestMultiRenameBodyStaysCompactAtDialogWidth(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &multiRenameState{
		targets: []multiRenameTarget{
			{oldName: "one.txt", newName: "one.txt", kind: filesys.EntryFile},
			{oldName: "two.txt", newName: "two.txt", kind: filesys.EntryFile},
		},
	}
	for _, ed := range []*widget.Editor{&st.searchEdit, &st.replaceEdit, &st.prefixEdit, &st.suffixEdit, &st.startEdit, &st.stepEdit, &st.digitsEdit} {
		ed.SingleLine = true
	}
	st.startEdit.SetText("1")
	st.stepEdit.SetText("1")
	st.digitsEdit.SetText("2")
	var router input.Router
	for _, width := range []int{740, 320} {
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(width, 620)},
		}
		dims := ui.layoutMultiRenameBody(material.NewTheme(), gtx, st)
		if dims.Size.X > width {
			t.Fatalf("body width=%d exceeds content width %d", dims.Size.X, width)
		}
		if width == 740 && dims.Size.Y > 470 {
			t.Fatalf("body height=%d is not compact", dims.Size.Y)
		}
	}
}

func TestMultiRenamePreviewRejectsDuplicateResults(t *testing.T) {
	st := &multiRenameState{
		targets: []multiRenameTarget{
			{oldName: "one.txt", kind: filesys.EntryFile},
			{oldName: "two.txt", kind: filesys.EntryFile},
		},
	}
	st.searchEdit.SetText("one")
	st.replaceEdit.SetText("two")
	st.refreshPreview()
	if st.lastErr == "" {
		t.Fatal("duplicate preview should prevent rename")
	}
	if got := st.changedCount(); got != 0 {
		t.Fatalf("changed count=%d want 0 while preview is invalid", got)
	}
}

func TestExecuteMultiRenameSupportsNameSwap(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("from-a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("from-b"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := executeMultiRename(copyEndpoint{dir: dir}, []multiRenamePlanItem{
		{src: a, dst: b},
		{src: b, dst: a},
	})
	if result.err != nil {
		t.Fatalf("swap rename failed: %v", result.err)
	}
	if result.renamed != 2 {
		t.Fatalf("renamed=%d want 2", result.renamed)
	}
	gotA, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	gotB, err := os.ReadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotA) != "from-b" || string(gotB) != "from-a" {
		t.Fatalf("swap contents a=%q b=%q", gotA, gotB)
	}
}

func TestMultiRenameBuildPlanRejectsExistingDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "old.txt")
	dst := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(src, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	st := &multiRenameState{
		endpoint: copyEndpoint{dir: dir},
		targets:  []multiRenameTarget{{path: src, oldName: "old.txt", kind: filesys.EntryFile}},
	}
	st.searchEdit.SetText("old")
	st.replaceEdit.SetText("new")
	if _, err := st.buildPlan(); err == nil {
		t.Fatal("existing destination should prevent rename")
	}
}
