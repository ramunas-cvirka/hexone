// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"os"
	"path/filepath"
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

func TestFileCreateDialogFocusedNameEnterSubmits(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)
	baseDir := t.TempDir()

	st := &fileCreateState{
		baseDir:     baseDir,
		kind:        fileCreateKindFolder,
		kindPrev:    fileCreateKindFolder,
		kindFocus:   fileCreateKindFolder,
		focus:       fileCreateDialogFocusName,
		actionFocus: fileCreateDialogActionConfirm,
		endpoint:    copyEndpoint{dir: baseDir},
	}
	st.nameEdit.SingleLine = true
	st.nameEdit.Submit = true
	st.nameEdit.SetText("created-by-enter")
	st.nameEditWant = true
	ui.fileCreate = st

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutFileCreateDialog(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	frame(now.Add(time.Millisecond))

	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(2 * time.Millisecond))

	if !st.running || st.doneCh == nil {
		t.Fatal("Enter on the focused name editor should start folder creation")
	}
	if got, want := st.targetPath, filepath.Join(baseDir, "created-by-enter"); got != want {
		t.Fatalf("targetPath=%q want %q", got, want)
	}
}

func TestFileCopyDialogFocusedDestinationEnterSubmits(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)
	baseDir := t.TempDir()
	srcPath := filepath.Join(baseDir, "source.txt")
	dstPath := filepath.Join(baseDir, "copied.txt")
	if err := os.WriteFile(srcPath, []byte("copy me"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", srcPath, err)
	}

	st := &fileCopyState{
		srcPath:     srcPath,
		srcEndpoint: copyEndpoint{dir: baseDir},
		dstEndpoint: copyEndpoint{dir: baseDir},
		focus:       fileCopyDialogFocusDestination,
		actionFocus: fileCopyDialogActionConfirm,
	}
	st.dstEdit.SingleLine = true
	st.dstEdit.Submit = true
	st.dstEdit.SetText(dstPath)
	st.dstEditWant = true
	ui.fileCopy = st

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutFileCopyDialog(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	frame(now.Add(time.Millisecond))

	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(2 * time.Millisecond))

	if !st.running || st.doneCh == nil {
		t.Fatal("Enter on the focused destination editor should start copying")
	}
	if got := st.dstPath; got != dstPath {
		t.Fatalf("dstPath=%q want %q", got, dstPath)
	}
}

func TestFileCopyDialogRunningCancelRequiresConfirmation(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	cancelCalls := 0
	st := &fileCopyState{
		srcPath:     "source.txt",
		dstPath:     "copy.txt",
		running:     true,
		startedAt:   now,
		cancelFunc:  func() { cancelCalls++ },
		focus:       fileCopyDialogFocusActions,
		actionFocus: fileCopyDialogActionCancel,
		keyFocus:    dialogKeyboardFocusState{wantFocus: true},
	}
	ui.fileCopy = st

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutFileCopyDialog(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)

	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(time.Millisecond))
	if cancelCalls != 0 {
		t.Fatalf("first Enter canceled immediately, calls=%d", cancelCalls)
	}
	if !st.cancelConfirmActive(now.Add(time.Millisecond)) {
		t.Fatal("first Enter should arm cancel confirmation")
	}
	if got := st.cancelButtonLabel(now.Add(time.Millisecond)); got != "Confirm 5s" {
		t.Fatalf("cancel label after first Enter = %q, want Confirm 5s", got)
	}

	frame(now.Add(6 * time.Second))
	if st.cancelConfirmActive(now.Add(6 * time.Second)) {
		t.Fatal("cancel confirmation should expire after the countdown")
	}
	if got := st.cancelButtonLabel(now.Add(6 * time.Second)); got != "Cancel" {
		t.Fatalf("cancel label after expiry = %q, want Cancel", got)
	}
	if cancelCalls != 0 {
		t.Fatalf("expiry canceled unexpectedly, calls=%d", cancelCalls)
	}

	router.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	frame(now.Add(6*time.Second + time.Millisecond))
	if cancelCalls != 0 {
		t.Fatalf("first Esc canceled immediately, calls=%d", cancelCalls)
	}
	if !st.cancelConfirmActive(now.Add(6*time.Second + time.Millisecond)) {
		t.Fatal("first Esc should arm cancel confirmation")
	}

	router.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	frame(now.Add(6*time.Second + 2*time.Millisecond))
	if cancelCalls != 1 {
		t.Fatalf("second Esc cancel calls=%d, want 1", cancelCalls)
	}
	if !st.canceling {
		t.Fatal("confirmed cancel should mark the copy as canceling")
	}
}

func TestFileMoveDialogFocusedDestinationEnterSubmits(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)
	baseDir := t.TempDir()
	srcPath := filepath.Join(baseDir, "source.txt")
	dstPath := filepath.Join(baseDir, "moved.txt")
	if err := os.WriteFile(srcPath, []byte("move me"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", srcPath, err)
	}

	st := &fileMoveState{
		srcPath:     srcPath,
		srcName:     "source.txt",
		endpoint:    copyEndpoint{dir: baseDir},
		focus:       fileMoveDialogFocusDestination,
		actionFocus: fileMoveDialogActionConfirm,
	}
	st.dstEdit.SingleLine = true
	st.dstEdit.Submit = true
	st.dstEdit.SetText(dstPath)
	st.dstEditWant = true
	ui.fileMove = st

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutFileMoveDialog(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	frame(now.Add(time.Millisecond))

	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(2 * time.Millisecond))

	if !st.running || st.doneCh == nil {
		t.Fatal("Enter on the focused destination editor should start moving")
	}
	if got := st.dstPath; got != dstPath {
		t.Fatalf("dstPath=%q want %q", got, dstPath)
	}
}

func TestFileCreateDialogFocusedNameArrowKeysMoveCaret(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := &fileCreateState{
		kind:        fileCreateKindFolder,
		kindPrev:    fileCreateKindFolder,
		kindFocus:   fileCreateKindFolder,
		focus:       fileCreateDialogFocusName,
		actionFocus: fileCreateDialogActionConfirm,
	}
	st.nameEdit.SingleLine = true
	st.nameEdit.Submit = true
	st.nameEdit.SetText("demo")
	st.nameEdit.SetCaret(2, 2)
	st.nameEditWant = true
	ui.fileCreate = st

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutFileCreateDialog(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	frame(now.Add(time.Millisecond))

	router.Queue(key.Event{Name: key.NameLeftArrow, State: key.Press})
	frame(now.Add(2 * time.Millisecond))
	if start, end := st.nameEdit.Selection(); start != 1 || end != 1 {
		t.Fatalf("caret after LeftArrow = (%d, %d), want (1, 1)", start, end)
	}
	if st.kind != fileCreateKindFolder || st.kindFocus != fileCreateKindFolder {
		t.Fatalf("kind changed unexpectedly: kind=%v focus=%v", st.kind, st.kindFocus)
	}

	router.Queue(key.Event{Name: key.NameRightArrow, State: key.Press})
	frame(now.Add(3 * time.Millisecond))
	if start, end := st.nameEdit.Selection(); start != 2 || end != 2 {
		t.Fatalf("caret after RightArrow = (%d, %d), want (2, 2)", start, end)
	}
	if st.kind != fileCreateKindFolder || st.kindFocus != fileCreateKindFolder {
		t.Fatalf("kind changed unexpectedly after RightArrow: kind=%v focus=%v", st.kind, st.kindFocus)
	}
}

func TestFileCopyDialogFocusedDestinationArrowKeysMoveCaret(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := &fileCopyState{
		srcPath:     "source.txt",
		srcEndpoint: copyEndpoint{dir: "."},
		dstEndpoint: copyEndpoint{dir: "."},
		focus:       fileCopyDialogFocusDestination,
		actionFocus: fileCopyDialogActionConfirm,
	}
	st.dstEdit.SingleLine = true
	st.dstEdit.Submit = true
	st.dstEdit.SetText("copy.txt")
	st.dstEdit.SetCaret(2, 2)
	st.dstEditWant = true
	ui.fileCopy = st

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutFileCopyDialog(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	frame(now.Add(time.Millisecond))

	router.Queue(key.Event{Name: key.NameLeftArrow, State: key.Press})
	frame(now.Add(2 * time.Millisecond))
	if start, end := st.dstEdit.Selection(); start != 1 || end != 1 {
		t.Fatalf("caret after LeftArrow = (%d, %d), want (1, 1)", start, end)
	}
	if st.actionFocus != fileCopyDialogActionConfirm {
		t.Fatalf("action focus changed unexpectedly: %v", st.actionFocus)
	}

	router.Queue(key.Event{Name: key.NameRightArrow, State: key.Press})
	frame(now.Add(3 * time.Millisecond))
	if start, end := st.dstEdit.Selection(); start != 2 || end != 2 {
		t.Fatalf("caret after RightArrow = (%d, %d), want (2, 2)", start, end)
	}
	if st.actionFocus != fileCopyDialogActionConfirm {
		t.Fatalf("action focus changed unexpectedly after RightArrow: %v", st.actionFocus)
	}
}

func TestFileMoveDialogFocusedDestinationArrowKeysMoveCaret(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := &fileMoveState{
		srcPath:     "source.txt",
		srcName:     "source.txt",
		endpoint:    copyEndpoint{dir: "."},
		focus:       fileMoveDialogFocusDestination,
		actionFocus: fileMoveDialogActionConfirm,
	}
	st.dstEdit.SingleLine = true
	st.dstEdit.Submit = true
	st.dstEdit.SetText("move.txt")
	st.dstEdit.SetCaret(2, 2)
	st.dstEditWant = true
	ui.fileMove = st

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutFileMoveDialog(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	frame(now.Add(time.Millisecond))

	router.Queue(key.Event{Name: key.NameLeftArrow, State: key.Press})
	frame(now.Add(2 * time.Millisecond))
	if start, end := st.dstEdit.Selection(); start != 1 || end != 1 {
		t.Fatalf("caret after LeftArrow = (%d, %d), want (1, 1)", start, end)
	}
	if st.actionFocus != fileMoveDialogActionConfirm {
		t.Fatalf("action focus changed unexpectedly: %v", st.actionFocus)
	}

	router.Queue(key.Event{Name: key.NameRightArrow, State: key.Press})
	frame(now.Add(3 * time.Millisecond))
	if start, end := st.dstEdit.Selection(); start != 2 || end != 2 {
		t.Fatalf("caret after RightArrow = (%d, %d), want (2, 2)", start, end)
	}
	if st.actionFocus != fileMoveDialogActionConfirm {
		t.Fatalf("action focus changed unexpectedly after RightArrow: %v", st.actionFocus)
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
