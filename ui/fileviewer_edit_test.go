// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"context"
	"encoding/binary"
	"hexone/fm"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func TestEncodeViewerTextPreservesUTF16BOMAndCRLF(t *testing.T) {
	got, err := encodeViewerText("alpha\r\nŽ", fm.ViewerFileEncodingUTF16LE, true, viewerLineEndingCRLF)
	if err != nil {
		t.Fatalf("encodeViewerText: %v", err)
	}
	if !bytes.HasPrefix(got, []byte{0xFF, 0xFE}) {
		t.Fatalf("encoded data does not start with UTF-16LE BOM: % X", got)
	}
	if decoded := decodeViewerUTF16(got, binary.LittleEndian, []byte{0xFF, 0xFE}, true); decoded != "alpha\r\nŽ" {
		t.Fatalf("decoded=%q want CRLF text", decoded)
	}
}

func TestReadViewerFileKeepsUnsanitizedEditableText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tabs.txt")
	if err := os.WriteFile(path, []byte("a\tb\r\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	content, _, errText, info := readViewerFile(path, fm.ViewerFileEncodingUTF8, 1<<20, time.Time{}, nil)
	if errText != "" {
		t.Fatalf("readViewerFile: %s", errText)
	}
	if content != "a    b\n" {
		t.Fatalf("sanitized viewer content=%q", content)
	}
	if info.editableText != "a\tb\n" {
		t.Fatalf("editable text=%q want tabs preserved", info.editableText)
	}
}

func TestFileViewerTextEditTracksDirtyStateAndTitle(t *testing.T) {
	st := &fileViewerState{
		mode:             "file",
		name:             "notes.txt",
		editBaselineText: "alpha",
	}
	st.contentEditor.SetText("alpha")
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st

	if !ui.startFileViewerEdit(time.Now()) {
		t.Fatalf("startFileViewerEdit failed: %s", st.status)
	}
	st.contentEditor.SetText("beta")
	ui.syncFileViewerTextEdit(st)

	if !st.editDirty {
		t.Fatal("changed editor text should mark the viewer dirty")
	}
	if got := viewerFilenameRailTitle(st); got != "notes.txt *" {
		t.Fatalf("dirty filename rail title=%q", got)
	}
	if !ui.stopFileViewerEdit() {
		t.Fatal("F3-style edit stop should succeed")
	}
	if st.editMode || !st.editDirty || st.content != "beta" {
		t.Fatalf("stopped state edit=%v dirty=%v content=%q", st.editMode, st.editDirty, st.content)
	}
}

func TestFileViewerTextEditOpensEditorContextMenuWithPaste(t *testing.T) {
	st := &fileViewerState{
		mode:             "file",
		editMode:         true,
		editBaselineText: "alpha",
		content:          "alpha",
	}
	st.contentEditor.SetText("alpha")
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	th := material.NewTheme()
	router := new(input.Router)

	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(480, 240)),
			Now:         time.Now(),
		}
		ui.handleEditorContextMenuGlobalPresses(gtx)
		ui.layoutFileViewerTextEditor(th, gtx, st)
		ui.handleFileViewerRootPointerEvents(gtx, st)
		pass := pointer.PassOp{}.Push(&ops)
		event.Op(&ops, &st.rootPointerTag)
		pass.Pop()
		ui.registerEditorContextMenuGlobalPointer(gtx)
		router.Frame(&ops)
	}

	frame()
	router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Buttons:  pointer.ButtonSecondary,
		Position: f32.Pt(80, 60),
	})
	frame()

	if ui.editorMenuOpenID != "viewer-file-edit" || ui.editorMenuTarget != &st.contentEditor {
		t.Fatalf("text edit context menu target=(%q,%p) want viewer editor %p", ui.editorMenuOpenID, ui.editorMenuTarget, &st.contentEditor)
	}
	if !ui.editorMenuCanPaste {
		t.Fatal("text edit context menu should enable Paste")
	}
	if st.menuOpen {
		t.Fatal("text edit should use the editor context menu, not the read-only viewer menu")
	}
	hitGTX := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	rowH := ui.editorContextMenuRowHeight(hitGTX)
	dividerH := ui.fileContextMenuSeparatorHeight(hitGTX)
	ui.editorMenuRect = image.Rect(0, 0, 160, rowH*3+dividerH*2)
	wrapY := rowH*2 + dividerH*2 + rowH/2
	if action := ui.editorContextMenuActionAt(hitGTX, image.Pt(20, wrapY)); action != "word-wrap" {
		t.Fatalf("File edit context menu third action=%q want word-wrap", action)
	}
}

func TestFileViewerEditToggleEntersEditAndDiscardsWhenLeaving(t *testing.T) {
	st := &fileViewerState{
		mode:             "file",
		name:             "notes.txt",
		editBaselineText: "alpha",
		content:          "alpha",
	}
	st.contentEditor.SetText("alpha")
	st.stream.SetContent("alpha")
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st

	if !ui.toggleFileViewerEdit(time.Now()) || !st.editMode {
		t.Fatalf("read-only toggle did not enter edit mode: edit=%v status=%q", st.editMode, st.status)
	}
	st.contentEditor.SetText("beta")
	ui.syncFileViewerTextEdit(st)
	if !st.editDirty {
		t.Fatal("edited text should be dirty before toggling back to read-only")
	}
	if !ui.toggleFileViewerEdit(time.Now()) {
		t.Fatal("editing toggle did not return to read-only mode")
	}
	if st.editMode || st.editDirty {
		t.Fatalf("read-only state edit=%v dirty=%v", st.editMode, st.editDirty)
	}
	if got := st.contentEditor.Text(); got != "alpha" {
		t.Fatalf("discarded editor text=%q want baseline", got)
	}
	if got := st.content; got != "alpha" {
		t.Fatalf("discarded viewer content=%q want baseline", got)
	}
}

func TestFileViewerTextSaveWritesAndClearsDirtyState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("alpha\r\n"), 0o640); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	st := &fileViewerState{
		mode:               "file",
		path:               path,
		name:               "notes.txt",
		detectedEncoding:   fm.ViewerFileEncodingUTF8,
		detectedLineEnding: viewerLineEndingCRLF,
		editBaselineText:   "alpha\n",
		editDirty:          true,
		saveCh:             make(chan fileViewerSaveResult, 1),
	}
	st.contentEditor.SetText("beta\n")
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st

	if !ui.startFileViewerSave(time.Now()) {
		t.Fatal("startFileViewerSave should start a dirty save")
	}
	deadline := time.Now().Add(2 * time.Second)
	for st.saving && time.Now().Before(deadline) {
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(320, 200)),
			Now:         time.Now(),
		}
		ui.pumpFileViewerSaveState(gtx, st)
		time.Sleep(time.Millisecond)
	}
	if st.saving {
		t.Fatal("save did not complete")
	}
	if st.editDirty || st.err != "" {
		t.Fatalf("save state dirty=%v err=%q", st.editDirty, st.err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "beta\r\n" {
		t.Fatalf("saved bytes=%q want CRLF", got)
	}
}

func TestFileViewerHexEditReplacesNibblesAndSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.bin")
	if err := os.WriteFile(path, []byte{0x12, 0x34}, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	v := newHexViewerState()
	v.fileSize = 2
	v.buffer = []byte{0x12, 0x34}
	v.bytesPerLine = 16
	st := &fileViewerState{
		mode:     "hex",
		path:     path,
		name:     "sample.bin",
		editMode: true,
		hex:      v,
		saveCh:   make(chan fileViewerSaveResult, 1),
	}
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st

	if !ui.handleFileViewerHexEditText(st, "F") {
		t.Fatal("high hex nibble was not handled")
	}
	if !ui.handleFileViewerHexEditText(st, "0") {
		t.Fatal("low hex nibble was not handled")
	}
	if got := v.edits[0]; got != 0xF0 {
		t.Fatalf("edited byte=%02X want F0", got)
	}
	if line, _ := v.lineBytes(0); !bytes.Equal(line, []byte{0xF0, 0x34}) {
		t.Fatalf("rendered edited line=% X", line)
	}
	if !st.editDirty {
		t.Fatal("hex edit should mark viewer dirty")
	}
	if !ui.startFileViewerSave(time.Now()) {
		t.Fatal("startFileViewerSave should start a dirty HEX save")
	}
	deadline := time.Now().Add(2 * time.Second)
	for st.saving && time.Now().Before(deadline) {
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(320, 200)),
			Now:         time.Now(),
		}
		ui.pumpFileViewerSaveState(gtx, st)
		time.Sleep(time.Millisecond)
	}
	if st.saving {
		t.Fatal("HEX save did not complete")
	}
	if st.editDirty || v.edits != nil {
		t.Fatalf("saved HEX state dirty=%v edits=%v; edit memory should be released", st.editDirty, v.edits)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, []byte{0xF0, 0x34}) {
		t.Fatalf("saved bytes=% X", got)
	}
}

func TestBuildViewerHexPatchWritesOnlyChangedRunsAtHugeOffsets(t *testing.T) {
	const hugeOffset = int64(100) << 30
	patch := buildViewerHexPatch(map[int64]byte{
		3:              0x11,
		4:              0x22,
		5:              0x33,
		hugeOffset - 1: 0xFE,
		hugeOffset:     0xFF,
	})
	if len(patch) != 2 {
		t.Fatalf("patch runs=%d want 2: %#v", len(patch), patch)
	}
	if patch[0].offset != 3 || !bytes.Equal(patch[0].data, []byte{0x11, 0x22, 0x33}) {
		t.Fatalf("first patch run=%#v", patch[0])
	}
	if patch[1].offset != hugeOffset-1 || !bytes.Equal(patch[1].data, []byte{0xFE, 0xFF}) {
		t.Fatalf("huge-offset patch run=%#v", patch[1])
	}
	written := 0
	for _, run := range patch {
		written += len(run.data)
	}
	if written != 5 {
		t.Fatalf("patch contains %d bytes, want exactly the 5 changed bytes", written)
	}
}

func TestFileViewerDirtyModeSwitchDefaultsToSave(t *testing.T) {
	st := &fileViewerState{
		mode:             "file",
		path:             filepath.Join(t.TempDir(), "notes.txt"),
		editMode:         true,
		editDirty:        true,
		editBaselineText: "alpha",
	}
	st.contentEditor.SetText("beta")
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st

	ui.setFileViewerMode("hex", time.Now())

	if !st.modeSwitchPrompt.open {
		t.Fatal("dirty mode switch did not open confirmation")
	}
	if st.mode != "file" || st.modeSwitchPrompt.targetMode != "hex" {
		t.Fatalf("mode=%q target=%q; switch should remain pending", st.mode, st.modeSwitchPrompt.targetMode)
	}
	if st.modeSwitchPrompt.actionFocus != fileViewerModeSwitchSave {
		t.Fatalf("default action=%v want Save", st.modeSwitchPrompt.actionFocus)
	}
}

func TestFileViewerFindWhileEditingUsesLastViewBuffer(t *testing.T) {
	st := &fileViewerState{
		mode:             "file",
		path:             "notes.txt",
		content:          "committed needle",
		editMode:         true,
		editDirty:        true,
		editBaselineText: "committed needle",
		status:           "modified",
	}
	st.contentEditor.SetText("unsaved replacement")
	st.stream.SetContent(st.content)
	st.find.editor.SetText("needle")
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st

	ui.openFileViewerFind(time.Now())

	if !st.find.open {
		t.Fatal("Find did not open while editing")
	}
	if len(st.find.matches) != 1 {
		t.Fatalf("stale view-buffer matches=%d want 1", len(st.find.matches))
	}
	if st.find.matches[0].Snippet != "committed needle" {
		t.Fatalf("Find searched unexpected buffer: %#v", st.find.matches[0])
	}
	if st.status != "modified" {
		t.Fatalf("opening Find replaced viewer status with %q", st.status)
	}
}

func TestFileViewerDirtyModeSwitchDiscard(t *testing.T) {
	st := &fileViewerState{
		mode:             "file",
		path:             filepath.Join(t.TempDir(), "notes.txt"),
		editMode:         true,
		editDirty:        true,
		editBaselineText: "alpha",
		content:          "alpha",
	}
	st.contentEditor.SetText("beta")
	st.stream.SetContent("alpha")
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st

	now := time.Now()
	ui.setFileViewerMode("hex", now)
	ui.confirmFileViewerModeSwitchDiscard(st, now)

	if st.modeSwitchPrompt.open {
		t.Fatal("Discard left mode-switch confirmation open")
	}
	if st.mode != "hex" || st.editMode || st.editDirty {
		t.Fatalf("after Discard: mode=%q edit=%v dirty=%v", st.mode, st.editMode, st.editDirty)
	}
	if got := st.contentEditor.Text(); got != "alpha" {
		t.Fatalf("discarded editor=%q want baseline", got)
	}
}

func TestFileViewerDirtyModeSwitchSaveThenSwitches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("alpha"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	st := &fileViewerState{
		mode:             "file",
		path:             path,
		name:             "notes.txt",
		editMode:         true,
		editDirty:        true,
		editBaselineText: "alpha",
		content:          "alpha",
		detectedEncoding: fm.ViewerFileEncodingUTF8,
		saveCh:           make(chan fileViewerSaveResult, 1),
	}
	st.contentEditor.SetText("beta")
	st.stream.SetContent("alpha")
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st

	now := time.Now()
	ui.setFileViewerMode("hex", now)
	ui.confirmFileViewerModeSwitchSave(st, now)
	if !st.saving || !st.modeSwitchPrompt.awaitingSave {
		t.Fatalf("save did not start: saving=%v awaiting=%v", st.saving, st.modeSwitchPrompt.awaitingSave)
	}

	deadline := time.Now().Add(2 * time.Second)
	for st.saving && time.Now().Before(deadline) {
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(320, 200)),
			Now:         time.Now(),
		}
		ui.pumpFileViewerSaveState(gtx, st)
		time.Sleep(time.Millisecond)
	}
	if st.saving {
		t.Fatal("save did not complete")
	}
	if st.modeSwitchPrompt.open || st.mode != "hex" || st.editDirty {
		t.Fatalf("after Save: prompt=%v mode=%q dirty=%v", st.modeSwitchPrompt.open, st.mode, st.editDirty)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "beta" {
		t.Fatalf("saved text=%q want beta", got)
	}
}

func TestFileViewerHexEditRepeatsAcrossBytes(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 4
	v.buffer = []byte{0, 0, 0, 0}
	v.bytesPerLine = 4
	st := &fileViewerState{mode: "hex", editMode: true, hex: v}
	ui := NewUI(fm.DefaultConfig())

	if !ui.handleFileViewerHexEditText(st, "11111111") {
		t.Fatal("repeated hex input was not handled")
	}
	if line, _ := v.lineBytes(0); !bytes.Equal(line, []byte{0x11, 0x11, 0x11, 0x11}) {
		t.Fatalf("repeated hex input=% X", line)
	}
}

func TestFileViewerHexEditAppliesEachNibbleAcrossSelection(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 3
	v.buffer = []byte{0x12, 0x34, 0x56}
	v.bytesPerLine = 3
	v.editCaret = 0
	v.setSelectionRange(0, 3)
	st := &fileViewerState{mode: "hex", editMode: true, hex: v}
	ui := NewUI(fm.DefaultConfig())

	if !ui.handleFileViewerHexEditText(st, "1") {
		t.Fatal("selected high-nibble edit was not handled")
	}
	if line, _ := v.lineBytes(0); !bytes.Equal(line, []byte{0x12, 0x14, 0x16}) {
		t.Fatalf("selected high nibble produced % X", line)
	}
	if v.editNibble != 1 || v.selectionLen != 3 {
		t.Fatalf("after high nibble cursor=%d selection=%d; want low nibble and preserved range", v.editNibble, v.selectionLen)
	}

	if !ui.handleFileViewerHexEditText(st, "A") {
		t.Fatal("selected low-nibble edit was not handled")
	}
	if line, _ := v.lineBytes(0); !bytes.Equal(line, []byte{0x1A, 0x1A, 0x1A}) {
		t.Fatalf("selected low nibble produced % X", line)
	}
	if v.editNibble != 0 || v.selectionLen != 3 {
		t.Fatalf("after low nibble cursor=%d selection=%d; want high nibble and preserved range", v.editNibble, v.selectionLen)
	}
}

func TestHexEditNibbleHighlightCoversSelectedBytes(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 8
	v.editCaret = 4
	v.setSelectionRange(2, 3)

	for off := int64(0); off < v.fileSize; off++ {
		want := off >= 2 && off < 5
		if got := hexEditNibbleActiveAt(v, off); got != want {
			t.Fatalf("selected nibble highlight at byte %d=%v want %v", off, got, want)
		}
	}

	v.editASCII = true
	if hexEditNibbleActiveAt(v, 3) {
		t.Fatal("Hex nibble highlight should be disabled in the ASCII lane")
	}

	v.editASCII = false
	v.setSelectionRange(4, 1)
	if !hexEditNibbleActiveAt(v, 4) || hexEditNibbleActiveAt(v, 3) {
		t.Fatal("single-byte nibble highlight should follow only the edit caret")
	}
}

func TestFileViewerHexEditPreservesExistingSelectionOnEntry(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 4
	v.buffer = []byte{1, 2, 3, 4}
	v.bytesPerLine = 4
	v.setSelectionRange(1, 3)
	st := &fileViewerState{mode: "hex", hex: v}
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st

	if !ui.startFileViewerEdit(time.Now()) {
		t.Fatal("Hex edit mode did not start")
	}
	if v.selectionStart != 1 || v.selectionLen != 3 || v.editCaret != 1 {
		t.Fatalf("edit entry selection=(%d,%d) caret=%d want (1,3), caret 1", v.selectionStart, v.selectionLen, v.editCaret)
	}
}

func TestFileViewerHexEditMouseDragCreatesSelection(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 4
	v.buffer = []byte{0x10, 0x20, 0x30, 0x40}
	v.bytesPerLine = 4
	v.visibleLines = 1
	v.charW = 10
	v.lineH = 16
	v.hexByteX = []int{0, 30, 60, 90}
	v.hexRect = image.Rect(20, 0, 130, 16)
	v.textRect = image.Rect(150, 0, 190, 16)
	st := &fileViewerState{mode: "hex", editMode: true, hex: v}
	ui := NewUI(fm.DefaultConfig())
	router := new(input.Router)

	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Constraints: layout.Exact(image.Pt(240, 80)),
			Now:         time.Now(),
		}
		ui.handleHexViewerEvents(gtx, st)
		pass := pointer.PassOp{}.Push(&ops)
		event.Op(&ops, &v.pointerTag)
		pass.Pop()
		router.Frame(&ops)
	}

	frame()
	router.Queue(pointer.Event{
		Kind:      pointer.Press,
		Source:    pointer.Mouse,
		PointerID: 1,
		Buttons:   pointer.ButtonPrimary,
		Position:  f32.Pt(21, 8),
	})
	frame()
	router.Queue(pointer.Event{
		Kind:      pointer.Move,
		Source:    pointer.Mouse,
		PointerID: 1,
		Buttons:   pointer.ButtonPrimary,
		Position:  f32.Pt(111, 8),
	})
	frame()

	if v.selectionStart != 0 || v.selectionLen != 4 {
		t.Fatalf("drag selection=(%d,%d) want (0,4)", v.selectionStart, v.selectionLen)
	}
	if v.editCaret != 3 || v.editASCII {
		t.Fatalf("drag edit caret=%d ascii=%v want Hex byte 3", v.editCaret, v.editASCII)
	}
}

func TestFileViewerASCIIEditAppliesCharacterAcrossSelection(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 3
	v.buffer = []byte("abc")
	v.bytesPerLine = 3
	v.editASCII = true
	v.setSelectionRange(0, 3)
	st := &fileViewerState{mode: "hex", editMode: true, hex: v}
	ui := NewUI(fm.DefaultConfig())

	if !ui.handleFileViewerHexEditText(st, "Z") {
		t.Fatal("selected ASCII edit was not handled")
	}
	if line, _ := v.lineBytes(0); !bytes.Equal(line, []byte("ZZZ")) {
		t.Fatalf("selected ASCII edit produced %q", line)
	}
	if v.selectionLen != 3 {
		t.Fatalf("ASCII edit collapsed selection to %d", v.selectionLen)
	}
}

func TestFileViewerHexEditConsumesFocusedEditEvents(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 2
	v.buffer = []byte{0, 0}
	v.bytesPerLine = 2
	st := &fileViewerState{mode: "hex", editMode: true, hex: v}
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	router := new(input.Router)
	frame := func(focus bool) {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(320, 120)),
			Now:         time.Now(),
		}
		event.Op(&ops, &v.editKeyTag)
		if focus {
			gtx.Execute(key.FocusCmd{Tag: &v.editKeyTag})
		}
		ui.handleFileViewerKeys(gtx)
		router.Frame(&ops)
	}

	frame(true)
	router.Queue(key.EditEvent{Text: "1111"})
	frame(false)

	if line, _ := v.lineBytes(0); !bytes.Equal(line, []byte{0x11, 0x11}) {
		t.Fatalf("focused edit events produced % X", line)
	}
}

func TestFileViewerHexNavigationCommitsPartialByteAndMoves(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 6
	v.buffer = []byte{0xAA, 0xBC, 0xCC, 0xDD, 0xEE, 0xFF}
	v.bytesPerLine = 2
	v.visibleLines = 3
	setHexViewerEditCaret(v, 1, false)
	st := &fileViewerState{mode: "hex", editMode: true, hex: v}
	ui := NewUI(fm.DefaultConfig())

	if !ui.handleFileViewerHexEditText(st, "1") {
		t.Fatal("high nibble was not handled")
	}
	if got := v.edits[1]; got != 0x1C {
		t.Fatalf("partial byte=%02X want 1C", got)
	}
	if v.editNibble != 1 {
		t.Fatalf("edit nibble=%d want low nibble cursor", v.editNibble)
	}
	if !ui.handleFileViewerHexEditKey(st, key.Event{Name: key.NameLeftArrow, State: key.Press}) {
		t.Fatal("left arrow was not handled")
	}
	if v.editCaret != 0 || v.editNibble != 0 {
		t.Fatalf("left arrow caret=%d nibble=%d want byte 0 high nibble", v.editCaret, v.editNibble)
	}
	if got := v.edits[1]; got != 0x1C {
		t.Fatalf("moving away changed partial byte to %02X", got)
	}
	if !ui.handleFileViewerHexEditKey(st, key.Event{Name: key.NameDownArrow, State: key.Press}) || v.editCaret != 2 {
		t.Fatalf("down arrow caret=%d want 2", v.editCaret)
	}
	if !ui.handleFileViewerHexEditKey(st, key.Event{Name: key.NameRightArrow, State: key.Press}) || v.editCaret != 3 {
		t.Fatalf("right arrow caret=%d want 3", v.editCaret)
	}
	if !ui.handleFileViewerHexEditKey(st, key.Event{Name: key.NameUpArrow, State: key.Press}) || v.editCaret != 1 {
		t.Fatalf("up arrow caret=%d want 1", v.editCaret)
	}
}

func TestHexViewerFullBufferDoesNotPrefetchAtFileEdges(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 4
	v.buffer = []byte{0xAA, 0xBB, 0xCC, 0xDD}
	v.bytesPerLine = 4
	v.visibleLines = 10
	if v.needsPrefetch() {
		t.Fatal("a buffer containing the complete file should not trigger another load")
	}
}

func TestViewerEditKeysOverrideStaleTerminalFocus(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 4
	v.buffer = []byte{0xAA, 0xBC, 0xCC, 0xDD}
	v.bytesPerLine = 4
	v.selectionStart = 1
	v.selectionLen = 1
	st := &fileViewerState{mode: "hex", hex: v}
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab0"
	ui.fileViewer = st
	ui.terminal = newTerminalSession(nil)
	ui.terminal.setActive(true)
	th := material.NewTheme()
	router := new(input.Router)

	registerFocus := func(tag event.Tag) {
		var ops op.Ops
		event.Op(&ops, tag)
		router.Frame(&ops)
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Source: router.Source()}
		event.Op(&ops, tag)
		gtx.Execute(key.FocusCmd{Tag: tag})
		router.Frame(&ops)
	}
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(640, 240)),
			Now:         time.Now(),
		}
		event.Op(&ops, &ui.terminal.keyTag)
		ui.handleGlobalFunctionKeys(gtx)
		ui.layoutHexOutputView(th, gtx, st)
		ui.handleFileManagerKeys(gtx)
		router.Frame(&ops)
	}

	registerFocus(&ui.terminal.keyTag)
	router.Event(key.Filter{Name: key.NameF4, Optional: ^key.Modifiers(0)})
	router.Queue(key.Event{Name: key.NameF4, State: key.Press})
	frame()
	if !st.editMode {
		t.Fatal("F4 should enter viewer edit mode even when terminal focus is stale")
	}

	router.Event(key.FocusFilter{Target: &v.editKeyTag})
	router.Queue(key.EditEvent{Text: "1"})
	frame()
	router.Event(key.Filter{Name: key.NameLeftArrow})
	router.Queue(key.Event{Name: key.NameLeftArrow, State: key.Press})
	frame()
	if got := v.edits[1]; got != 0x1C {
		t.Fatalf("routed partial byte=%02X want 1C", got)
	}
	if v.editCaret != 0 {
		t.Fatalf("routed left arrow caret=%d want 0", v.editCaret)
	}

	router.Event(key.Filter{Name: key.NameF3, Optional: ^key.Modifiers(0)})
	router.Queue(key.Event{Name: key.NameF3, State: key.Press})
	frame()
	if st.editMode {
		t.Fatal("F3 should leave viewer edit mode")
	}

	registerFocus(&ui.terminal.keyTag)
	router.Event(key.Filter{Name: key.NameF4, Optional: ^key.Modifiers(0)})
	router.Queue(key.Event{Name: key.NameF4, State: key.Press})
	frame()
	if !st.editMode {
		t.Fatalf("F4 should reliably re-enter viewer edit mode (status=%q err=%q loading=%v dirty=%v terminalFocused=%v)", st.status, st.err, st.loading, st.editDirty, ui.terminalFocused(layout.Context{Source: router.Source()}))
	}
}

func TestFileViewerASCIIEditWritesTextAndAdvances(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 4
	v.buffer = []byte{0, 0, 0, 0}
	v.bytesPerLine = 4
	setHexViewerEditCaret(v, 0, true)
	st := &fileViewerState{mode: "hex", editMode: true, hex: v}
	ui := NewUI(fm.DefaultConfig())

	if !v.editASCII {
		t.Fatal("ASCII lane selection should activate text entry")
	}
	if !ui.handleFileViewerHexEditText(st, "Ab! ") {
		t.Fatal("ASCII input was not handled")
	}
	if line, _ := v.lineBytes(0); !bytes.Equal(line, []byte("Ab! ")) {
		t.Fatalf("ASCII edited bytes=%q", line)
	}
}

func TestFileViewerTextEditKeepsSyntaxAndExposesScrollbar(t *testing.T) {
	content := strings.Repeat("package main\n\nfunc main() { println(\"cyan\") }\n\n", 40)
	doc := viewerBuildSyntaxDocument(context.Background(), "main.go", content)
	if !doc.ready() {
		t.Fatal("Go source should produce syntax spans")
	}
	st := &fileViewerState{
		mode:             "file",
		path:             "main.go",
		editBaselineText: content,
		content:          content,
	}
	st.stream.SetContent(content)
	st.stream.setSyntax(doc)
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	if !ui.startFileViewerEdit(time.Now()) {
		t.Fatalf("startFileViewerEdit failed: %s", st.status)
	}

	gtx := layout.Context{
		Ops:         new(op.Ops),
		Source:      new(input.Router).Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(480, 120)),
		Now:         time.Now(),
	}
	ui.layoutFileViewerTextEditor(material.NewTheme(), gtx, st)

	metrics, scrollable := editorVerticalScrollMetrics(&st.contentEditor)
	if !scrollable || metrics.Content <= metrics.Viewport {
		t.Fatalf("edit scrollbar metrics content=%d viewport=%d scrollable=%v", metrics.Content, metrics.Viewport, scrollable)
	}
	editorScrollToVerticalOffset(&st.contentEditor, metrics.MaxOffset)
	_, visibleEnd, ok := editorVisibleRuneRange(&st.contentEditor)
	if !ok {
		t.Fatal("visible editor rune range unavailable")
	}
	if lastVisibleLine := viewerEditLineAtRune(st.editLineRunes, visibleEnd); lastVisibleLine < len(st.editLineRunes)-3 {
		t.Fatalf("bottom visible syntax line=%d want near %d", lastVisibleLine, len(st.editLineRunes)-1)
	}
	if !st.editSyntax.ready() {
		t.Fatal("edit mode should retain the viewer syntax document")
	}
}

func TestFileViewerTextEditHonorsWordWrapSetting(t *testing.T) {
	content := strings.Repeat("alpha beta gamma delta epsilon ", 24)
	st := &fileViewerState{
		mode:             "file",
		path:             "long.txt",
		editBaselineText: content,
		content:          content,
		wrapEnabled:      false,
	}
	st.stream.SetContent(content)
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	if !ui.startFileViewerEdit(time.Now()) {
		t.Fatalf("startFileViewerEdit failed: %s", st.status)
	}
	router := new(input.Router)
	layoutEditor := func() {
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(240, 140)),
			Now:         time.Now(),
		}
		ui.layoutFileViewerTextEditor(material.NewTheme(), gtx, st)
	}

	layoutEditor()
	textView, ok := editorTextView(&st.contentEditor)
	if !ok {
		t.Fatal("editor text view unavailable")
	}
	noWrapHeight := textView.FullDimensions().Size.Y
	if noWrapHeight > measureEditorEquivalentLineHeightAt(layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}, ui.viewerTextSize())*2 {
		t.Fatalf("word wrap off produced content height=%d for one logical line", noWrapHeight)
	}
	st.contentEditor.SetCaret(st.contentEditor.Len(), st.contentEditor.Len())
	layoutEditor()
	layoutEditor()
	if st.editHOffset <= 0 {
		t.Fatal("word wrap off should reveal the end of a long line horizontally")
	}

	st.wrapEnabled = true
	layoutEditor()
	wrapHeight := textView.FullDimensions().Size.Y
	if wrapHeight <= noWrapHeight*2 {
		t.Fatalf("word wrap on content height=%d want substantially above no-wrap height=%d", wrapHeight, noWrapHeight)
	}
	if st.editHOffset != 0 || st.contentEditor.WrapPolicy != text.WrapHeuristically {
		t.Fatalf("word wrap on horizontal offset=%d policy=%v", st.editHOffset, st.contentEditor.WrapPolicy)
	}
}

func TestViewerFunctionBarUsesEditAndSaveLabels(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs = widget.Enum{Value: "tab0"}
	ui.fileViewer = &fileViewerState{mode: "file", editMode: true}

	specs := ui.functionBarButtonSpecs()
	if specs[1].label != "Save" || specs[2].label != "Discard" || specs[3].label != "Edit" {
		t.Fatalf("viewer labels F2=%q F3=%q F4=%q", specs[1].label, specs[2].label, specs[3].label)
	}
}

func TestViewerFunctionActionsUseF4ForEditAndF3ForDiscard(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs = widget.Enum{Value: "tab0"}
	st := &fileViewerState{mode: "file", editBaselineText: "alpha"}
	st.contentEditor.SetText("alpha")
	ui.fileViewer = st

	if !ui.performFunctionBarAction(functionBarActionOpen, time.Now()) || !st.editMode {
		t.Fatal("F4/Open action should enter viewer edit mode")
	}
	st.contentEditor.SetText("changed")
	ui.syncFileViewerTextEdit(st)
	if !ui.performFunctionBarAction(functionBarActionView, time.Now()) || st.editMode {
		t.Fatal("F3/View action should discard and leave viewer edit mode")
	}
	if st.editDirty || st.contentEditor.Text() != "alpha" || st.content != "alpha" {
		t.Fatalf("F3 discard dirty=%v editor=%q content=%q", st.editDirty, st.contentEditor.Text(), st.content)
	}
}

func TestViewerF3DiscardsHexChangesAndKeepsViewerOpen(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 2
	v.buffer = []byte{0x12, 0x34}
	v.bytesPerLine = 2
	st := &fileViewerState{mode: "hex", editMode: true, hex: v}
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st

	if !ui.handleFileViewerHexEditText(st, "F0") {
		t.Fatal("HEX edit was not handled")
	}
	if !ui.performFunctionBarAction(functionBarActionView, time.Now()) {
		t.Fatal("F3 action was not handled")
	}
	if ui.fileViewer != st {
		t.Fatal("F3 should keep the viewer open")
	}
	if st.editMode || st.editDirty || v.edits != nil {
		t.Fatalf("F3 HEX discard editMode=%v dirty=%v edits=%v", st.editMode, st.editDirty, v.edits)
	}
	if line, _ := v.lineBytes(0); !bytes.Equal(line, []byte{0x12, 0x34}) {
		t.Fatalf("F3 restored HEX line=% X", line)
	}
}

func TestViewerEscapeDiscardsAndClosesFileAndHexEdits(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*UI) *fileViewerState
		check func(*testing.T, *fileViewerState)
	}{
		{
			name: "file",
			setup: func(ui *UI) *fileViewerState {
				st := &fileViewerState{mode: "file", editMode: true, editBaselineText: "alpha"}
				st.contentEditor.SetText("changed")
				st.editDirty = true
				ui.fileViewer = st
				return st
			},
			check: func(t *testing.T, st *fileViewerState) {
				if st.editDirty || st.contentEditor.Text() != "alpha" || st.content != "alpha" {
					t.Fatalf("Escape file discard dirty=%v editor=%q content=%q", st.editDirty, st.contentEditor.Text(), st.content)
				}
			},
		},
		{
			name: "hex",
			setup: func(ui *UI) *fileViewerState {
				v := newHexViewerState()
				v.fileSize = 2
				v.buffer = []byte{0x12, 0x34}
				v.bytesPerLine = 2
				v.edits = map[int64]byte{0: 0xF0}
				st := &fileViewerState{mode: "hex", editMode: true, editDirty: true, hex: v}
				ui.fileViewer = st
				return st
			},
			check: func(t *testing.T, st *fileViewerState) {
				if st.editDirty || st.hex.edits != nil {
					t.Fatalf("Escape HEX discard dirty=%v edits=%v", st.editDirty, st.hex.edits)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ui := NewUI(fm.DefaultConfig())
			st := tc.setup(ui)
			router := new(input.Router)
			gtx := layout.Context{
				Ops:         new(op.Ops),
				Source:      router.Source(),
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Constraints: layout.Exact(image.Pt(320, 120)),
				Now:         time.Now(),
			}
			router.Event(key.Filter{Name: key.NameEscape})
			router.Queue(key.Event{Name: key.NameEscape, State: key.Press})
			ui.handleFileViewerKeys(gtx)
			if ui.fileViewer != nil {
				t.Fatal("Escape should close the viewer")
			}
			tc.check(t, st)
		})
	}
}
