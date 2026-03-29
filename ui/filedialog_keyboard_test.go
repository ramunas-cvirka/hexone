// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"testing"
	"time"

	"hexone/fm"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func testDialogLayoutContext(router *input.Router, now time.Time) layout.Context {
	return layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(960, 720),
		},
		Now: now,
	}
}

func TestFileCreateDialogKeyboardFocusActivatesKindTabsWithArrows(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := &fileCreateState{
		kind:      fileCreateKindFolder,
		kindPrev:  fileCreateKindFolder,
		kindFocus: fileCreateKindFolder,
		focus:     fileCreateDialogFocusName,
	}
	st.nameEdit.SingleLine = true
	st.nameEdit.Submit = true
	ui.fileCreate = st

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutFileCreateDialog(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)

	router.Queue(key.Event{Name: key.NameTab, Modifiers: key.ModShift, State: key.Press})
	frame(now.Add(time.Millisecond))
	if st.focus != fileCreateDialogFocusKindTabs {
		t.Fatalf("focus after Shift+Tab = %v, want kind tabs", st.focus)
	}

	router.Queue(key.Event{Name: key.NameRightArrow, State: key.Press})
	frame(now.Add(2 * time.Millisecond))
	if st.kindFocus != fileCreateKindFile {
		t.Fatalf("kind focus after RightArrow = %v, want file", st.kindFocus)
	}
	if st.kind != fileCreateKindFile {
		t.Fatalf("active kind after RightArrow = %v, want file", st.kind)
	}
}

func TestFileCreateDialogKeyboardFocusIgnoresUpDownForHorizontalGroups(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := &fileCreateState{
		kind:      fileCreateKindFolder,
		kindPrev:  fileCreateKindFolder,
		kindFocus: fileCreateKindFolder,
		focus:     fileCreateDialogFocusKindTabs,
		keyFocus:  dialogKeyboardFocusState{wantFocus: true},
	}
	st.nameEdit.SingleLine = true
	st.nameEdit.Submit = true
	ui.fileCreate = st

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutFileCreateDialog(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)

	router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	frame(now.Add(time.Millisecond))
	if st.kindFocus != fileCreateKindFolder {
		t.Fatalf("kind focus after DownArrow = %v, want folder", st.kindFocus)
	}
	if st.kind != fileCreateKindFolder {
		t.Fatalf("active kind after DownArrow = %v, want folder", st.kind)
	}
}

func TestFileCopyDialogKeyboardFocusCyclesToActionGroup(t *testing.T) {
	st := &fileCopyState{
		focus:       fileCopyDialogFocusDestination,
		actionFocus: fileCopyDialogActionConfirm,
	}

	if !st.stepFocus(1) || st.focus != fileCopyDialogFocusActions {
		t.Fatalf("first Tab focus = %v, want actions", st.focus)
	}
	if !st.stepAction(-1) || st.actionFocus != fileCopyDialogActionCancel {
		t.Fatalf("LeftArrow action = %v, want cancel", st.actionFocus)
	}
	if !st.stepAction(1) || st.actionFocus != fileCopyDialogActionConfirm {
		t.Fatalf("RightArrow action = %v, want confirm", st.actionFocus)
	}
	if !st.stepFocus(-1) || st.focus != fileCopyDialogFocusDestination {
		t.Fatalf("Shift+Tab focus = %v, want destination", st.focus)
	}
}

func TestFileMoveDialogKeyboardFocusCyclesToActionGroup(t *testing.T) {
	st := &fileMoveState{
		focus:       fileMoveDialogFocusDestination,
		actionFocus: fileMoveDialogActionConfirm,
	}

	if !st.stepFocus(1) || st.focus != fileMoveDialogFocusActions {
		t.Fatalf("first Tab focus = %v, want actions", st.focus)
	}
	if !st.stepAction(-1) || st.actionFocus != fileMoveDialogActionCancel {
		t.Fatalf("LeftArrow action = %v, want cancel", st.actionFocus)
	}
	if !st.stepAction(1) || st.actionFocus != fileMoveDialogActionConfirm {
		t.Fatalf("RightArrow action = %v, want confirm", st.actionFocus)
	}
	if !st.stepFocus(-1) || st.focus != fileMoveDialogFocusDestination {
		t.Fatalf("Shift+Tab focus = %v, want destination", st.focus)
	}
}

func TestFileDeleteDialogKeyboardEnterActivatesArrowSelectedAction(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	ui.fileDelete = &fileDeleteState{
		targetName:  "sample.txt",
		targetPath:  "sample.txt",
		focus:       fileDeleteDialogFocusActions,
		actionFocus: fileDeleteDialogActionConfirm,
		keyFocus:    dialogKeyboardFocusState{wantFocus: true},
	}

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutFileDeleteDialog(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)

	router.Queue(key.Event{Name: key.NameLeftArrow, State: key.Press})
	frame(now.Add(time.Millisecond))
	if ui.fileDelete == nil {
		t.Fatal("delete dialog closed unexpectedly after LeftArrow")
	}
	if ui.fileDelete.actionFocus != fileDeleteDialogActionCancel {
		t.Fatalf("action after LeftArrow = %v, want cancel", ui.fileDelete.actionFocus)
	}

	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(2 * time.Millisecond))
	if ui.fileDelete != nil {
		t.Fatal("Enter on focused cancel should close the delete dialog")
	}
}
