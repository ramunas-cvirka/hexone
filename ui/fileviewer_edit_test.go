// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hexone/filesys"
	"hexone/fm"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
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

func TestFileViewerTextSaveRefreshesPaneFileSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("create empty file: %v", err)
	}
	listing, err := filesys.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}
	pane := newFilePaneState(dir, nil)
	pane.applyListing(listing, path, "", 0)
	row := pane.findEntryPathIndex(path)
	if row < 0 || pane.model.Entry(row).SizeBytes != 0 {
		t.Fatalf("initial empty-file row = %#v", pane.model.Entry(row))
	}

	st := &fileViewerState{
		pane:             0,
		mode:             "file",
		path:             path,
		name:             "empty.txt",
		detectedEncoding: fm.ViewerFileEncodingUTF8,
		editMode:         true,
		editDirty:        true,
		saveCh:           make(chan fileViewerSaveResult, 1),
	}
	st.contentEditor.SetText("updated")
	ui := NewUI(fm.DefaultConfig())
	ui.filePanes = []*filePaneState{pane}
	ui.filePaneTabs = nil
	ui.fileViewer = st

	if !ui.startFileViewerSave(time.Now()) {
		t.Fatal("startFileViewerSave should start a dirty save")
	}
	gtx := layout.Context{Ops: new(op.Ops), Now: time.Now()}
	deadline := time.Now().Add(2 * time.Second)
	for st.saving && time.Now().Before(deadline) {
		gtx.Now = time.Now()
		ui.pumpFileViewerSaveState(gtx, st)
		time.Sleep(time.Millisecond)
	}
	if st.saving {
		t.Fatal("save did not complete")
	}
	if !pane.loading {
		t.Fatal("successful viewer save should schedule a pane refresh")
	}
	for pane.loading && time.Now().Before(deadline) {
		gtx.Now = time.Now()
		ui.pumpFilePaneLoads(gtx)
		time.Sleep(time.Millisecond)
	}
	if pane.loading {
		t.Fatal("pane refresh did not complete")
	}
	row = pane.findEntryPathIndex(path)
	entry := pane.model.Entry(row)
	if entry == nil || entry.SizeBytes != int64(len("updated")) || entry.SizeText == "0 B" {
		t.Fatalf("refreshed entry = %#v, want size %d", entry, len("updated"))
	}
}

func TestViewerF2RoutesToSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("alpha\n"), 0o640); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	st := &fileViewerState{
		mode:             "file",
		path:             path,
		name:             "notes.txt",
		detectedEncoding: fm.ViewerFileEncodingUTF8,
		editBaselineText: "alpha\n",
		editMode:         true,
		editDirty:        true,
		saveCh:           make(chan fileViewerSaveResult, 1),
	}
	st.contentEditor.SetText("beta\n")
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab0"
	ui.fileViewer = st

	gtx, router := testKeyContext()
	router.Event(key.Filter{Name: key.NameF2})
	router.Queue(key.Event{Name: key.NameF2, State: key.Press})
	ui.handleGlobalFunctionKeys(gtx)
	if !st.saving {
		t.Fatal("viewer F2 should start saving a dirty edit")
	}

	deadline := time.Now().Add(2 * time.Second)
	for st.saving && time.Now().Before(deadline) {
		ui.pumpFileViewerSaveState(layout.Context{Ops: new(op.Ops), Now: time.Now()}, st)
		time.Sleep(time.Millisecond)
	}
	if st.saving || st.editDirty || st.err != "" {
		t.Fatalf("F2 save state saving=%v dirty=%v err=%q", st.saving, st.editDirty, st.err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "beta\n" {
		t.Fatalf("F2 saved content=%q err=%v", got, err)
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

func TestHexEditASCIIHighlightCoversSelectedBytes(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 8
	v.editCaret = 4
	v.editASCII = true
	v.setSelectionRange(2, 3)

	for off := int64(0); off < v.fileSize; off++ {
		want := off >= 2 && off < 5
		if got := hexEditASCIIActiveAt(v, off); got != want {
			t.Fatalf("selected ASCII highlight at byte %d=%v want %v", off, got, want)
		}
	}

	v.editASCII = false
	if hexEditASCIIActiveAt(v, 3) {
		t.Fatal("ASCII text highlight should be disabled in the HEX lane")
	}

	v.editASCII = true
	v.setSelectionRange(4, 1)
	if !hexEditASCIIActiveAt(v, 4) || hexEditASCIIActiveAt(v, 3) {
		t.Fatal("single-byte ASCII highlight should follow only the edit caret")
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

func TestFileViewerHexEditMouseDragSwitchesInputLane(t *testing.T) {
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
	if v.editASCII {
		t.Fatal("press in the HEX lane should activate hexadecimal input")
	}

	router.Queue(pointer.Event{
		Kind:      pointer.Move,
		Source:    pointer.Mouse,
		PointerID: 1,
		Buttons:   pointer.ButtonPrimary,
		Position:  f32.Pt(185, 8),
	})
	frame()
	if !v.editASCII {
		t.Fatal("dragging into the text lane should activate ASCII input")
	}
	if v.selectionStart != 0 || v.selectionLen != 4 || v.editCaret != 3 {
		t.Fatalf("text-lane drag selection=(%d,%d) caret=%d want (0,4), caret 3", v.selectionStart, v.selectionLen, v.editCaret)
	}

	router.Queue(pointer.Event{
		Kind:      pointer.Move,
		Source:    pointer.Mouse,
		PointerID: 1,
		Buttons:   pointer.ButtonPrimary,
		Position:  f32.Pt(51, 8),
	})
	frame()
	if v.editASCII {
		t.Fatal("dragging back into the HEX lane should reactivate hexadecimal input")
	}
}

func TestHexSelectionLaneColorsFollowEditInputLane(t *testing.T) {
	regular := color.NRGBA{R: 1, G: 2, B: 3, A: 4}
	strong := color.NRGBA{R: 5, G: 6, B: 7, A: 8}
	edit := color.NRGBA{R: 9, G: 10, B: 11, A: 12}
	theme := fileViewerTheme{
		HexSelection:       regular,
		HexStrongSelection: strong,
		EditCursorBg:       edit,
	}
	v := newHexViewerState()

	hexSelection, textSelection := hexSelectionLaneColors(theme, v, true)
	if hexSelection != edit || textSelection != regular {
		t.Fatalf("HEX input colors=(%v,%v) want (%v,%v)", hexSelection, textSelection, edit, regular)
	}

	v.editASCII = true
	hexSelection, textSelection = hexSelectionLaneColors(theme, v, true)
	if hexSelection != regular || textSelection != edit {
		t.Fatalf("ASCII input colors=(%v,%v) want (%v,%v)", hexSelection, textSelection, regular, edit)
	}

	hexSelection, textSelection = hexSelectionLaneColors(theme, v, false)
	if hexSelection != regular || textSelection != strong {
		t.Fatalf("read-only colors=(%v,%v) want (%v,%v)", hexSelection, textSelection, regular, strong)
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

func TestFileViewerVirtualTextEditConsumesFocusedEvents(t *testing.T) {
	content := "alpha\nbeta"
	st := &fileViewerState{
		mode:             "file",
		path:             "events.txt",
		content:          content,
		editBaselineText: content,
	}
	st.contentEditor.SetText(content)
	st.stream.SetContent(content)
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	if !ui.startFileViewerEdit(time.Now()) {
		t.Fatalf("start edit: %s", st.status)
	}
	th := material.NewTheme()
	router := new(input.Router)
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(320, 120)),
			Now:         time.Now(),
		}
		ui.layoutFileViewerTextEditor(th, gtx, st)
		router.Frame(&ops)
	}

	frame()
	router.Event(key.FocusFilter{Target: &st.editKeyTag})
	router.Queue(key.EditEvent{Range: key.Range{Start: 5, End: 5}, Text: "!"})
	frame()
	if got := st.virtualEditText(); got != "alpha!\nbeta" {
		t.Fatalf("focused edit text=%q", got)
	}
	router.Event(key.Filter{Focus: &st.editKeyTag, Name: key.NameDeleteBackward, Optional: ^key.Modifiers(0)})
	router.Queue(key.Event{Name: key.NameDeleteBackward, State: key.Press})
	frame()
	if got := st.virtualEditText(); got != content {
		t.Fatalf("focused backspace text=%q want %q", got, content)
	}
}

func TestFileViewerVirtualTextEditMouseScrollAndSelection(t *testing.T) {
	content := strings.Repeat("0123456789 selection target\n", 80)
	st := &fileViewerState{
		mode:             "file",
		path:             "pointer.txt",
		content:          content,
		editBaselineText: content,
	}
	st.contentEditor.SetText(content)
	st.stream.SetContent(content)
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	if !ui.startFileViewerEdit(time.Now()) {
		t.Fatalf("start edit: %s", st.status)
	}
	th := material.NewTheme()
	router := new(input.Router)
	now := time.Now()
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(360, 130)),
			Now:         now,
		}
		ui.layoutFileViewerTextEditor(th, gtx, st)
		router.Frame(&ops)
		now = now.Add(16 * time.Millisecond)
	}

	frame()
	if st.stream.lineNumRect.Empty() || st.stream.textRect.Min.X <= 0 {
		t.Fatalf("line-number gutter=%v text rect=%v", st.stream.lineNumRect, st.stream.textRect)
	}
	inside := f32.Pt(float32(st.stream.textRect.Min.X+20), float32(st.stream.lineH/2))
	router.Queue(pointer.Event{
		Kind:     pointer.Scroll,
		Source:   pointer.Mouse,
		Position: inside,
		Scroll:   f32.Pt(0, 3),
	})
	frame()
	if st.stream.topLine <= 0 {
		t.Fatal("mouse wheel did not scroll the virtual editor")
	}
	beforeDrag := st.stream.topLine
	scrollbar := f32.Pt(float32(st.stream.trackRect.Min.X+1), float32(st.stream.trackRect.Min.Y+st.stream.trackRect.Dy()*3/4))
	router.Queue(pointer.Event{
		Kind:      pointer.Press,
		Source:    pointer.Mouse,
		PointerID: 2,
		Buttons:   pointer.ButtonPrimary,
		Position:  scrollbar,
	})
	frame()
	if !st.stream.dragging || st.stream.topLine <= beforeDrag {
		t.Fatalf("vertical scrollbar drag start dragging=%v top=%d before=%d", st.stream.dragging, st.stream.topLine, beforeDrag)
	}
	router.Queue(pointer.Event{
		Kind:      pointer.Release,
		Source:    pointer.Mouse,
		PointerID: 2,
		Position:  scrollbar,
	})
	frame()

	st.stream.topLine = 0
	st.stream.syncVisualTop()
	start := f32.Pt(float32(st.stream.textRect.Min.X+st.stream.textPad+st.stream.colOffsetPx(1)), float32(st.stream.lineH/2))
	end := f32.Pt(float32(st.stream.textRect.Min.X+st.stream.textPad+st.stream.colOffsetPx(8)), float32(st.stream.lineH*2+st.stream.lineH/2))
	router.Queue(pointer.Event{
		Kind:      pointer.Press,
		Source:    pointer.Mouse,
		PointerID: 1,
		Buttons:   pointer.ButtonPrimary,
		Position:  start,
	})
	frame()
	router.Queue(pointer.Event{
		Kind:      pointer.Move,
		Source:    pointer.Mouse,
		PointerID: 1,
		Buttons:   pointer.ButtonPrimary,
		Position:  end,
	})
	frame()
	router.Queue(pointer.Event{
		Kind:      pointer.Release,
		Source:    pointer.Mouse,
		PointerID: 1,
		Position:  end,
	})
	frame()
	if st.stream.selLen <= 0 || st.stream.selAnchor == st.stream.selHead {
		t.Fatalf("mouse drag selection=(anchor=%d head=%d len=%d)", st.stream.selAnchor, st.stream.selHead, st.stream.selLen)
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

func TestFileViewerHexShiftNavigationExtendsAndReversesSelection(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 8
	v.buffer = []byte{0, 1, 2, 3, 4, 5, 6, 7}
	v.bytesPerLine = 4
	v.visibleLines = 2
	setHexViewerEditCaret(v, 2, false)
	st := &fileViewerState{mode: "hex", editMode: true, hex: v}
	ui := NewUI(fm.DefaultConfig())
	shiftPress := func(name key.Name) {
		t.Helper()
		if !ui.handleFileViewerHexEditKey(st, key.Event{Name: name, Modifiers: key.ModShift, State: key.Press}) {
			t.Fatalf("%s was not handled", name)
		}
	}

	shiftPress(key.NameRightArrow)
	shiftPress(key.NameRightArrow)
	if v.selectionStart != 2 || v.selectionLen != 3 || v.editCaret != 4 {
		t.Fatalf("extended selection=(%d,%d) caret=%d want (2,3), caret 4", v.selectionStart, v.selectionLen, v.editCaret)
	}

	shiftPress(key.NameLeftArrow)
	shiftPress(key.NameLeftArrow)
	shiftPress(key.NameLeftArrow)
	if v.selectionStart != 1 || v.selectionLen != 2 || v.editCaret != 1 {
		t.Fatalf("reversed selection=(%d,%d) caret=%d want (1,2), caret 1", v.selectionStart, v.selectionLen, v.editCaret)
	}

	shiftPress(key.NameDownArrow)
	if v.selectionStart != 2 || v.selectionLen != 4 || v.editCaret != 5 {
		t.Fatalf("vertical selection=(%d,%d) caret=%d want (2,4), caret 5", v.selectionStart, v.selectionLen, v.editCaret)
	}
}

func TestFileViewerHexPlainHorizontalNavigationCollapsesSelection(t *testing.T) {
	v := newHexViewerState()
	v.fileSize = 8
	v.buffer = []byte{0, 1, 2, 3, 4, 5, 6, 7}
	v.bytesPerLine = 4
	v.setSelectionRange(2, 4)
	v.editCaret = 5
	st := &fileViewerState{mode: "hex", editMode: true, hex: v}
	ui := NewUI(fm.DefaultConfig())

	if !ui.handleFileViewerHexEditKey(st, key.Event{Name: key.NameLeftArrow, State: key.Press}) {
		t.Fatal("left arrow was not handled")
	}
	if v.selectionStart != 2 || v.selectionLen != 1 || v.editCaret != 2 {
		t.Fatalf("left collapse selection=(%d,%d) caret=%d want (2,1), caret 2", v.selectionStart, v.selectionLen, v.editCaret)
	}

	v.setSelectionRange(2, 4)
	v.editCaret = 2
	if !ui.handleFileViewerHexEditKey(st, key.Event{Name: key.NameRightArrow, State: key.Press}) {
		t.Fatal("right arrow was not handled")
	}
	if v.selectionStart != 5 || v.selectionLen != 1 || v.editCaret != 5 {
		t.Fatalf("right collapse selection=(%d,%d) caret=%d want (5,1), caret 5", v.selectionStart, v.selectionLen, v.editCaret)
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

	router.Event(key.Filter{Name: key.NameRightArrow, Optional: key.ModShift})
	router.Queue(key.Event{Name: key.NameRightArrow, Modifiers: key.ModShift, State: key.Press})
	frame()
	if v.selectionStart != 0 || v.selectionLen != 2 || v.editCaret != 1 {
		t.Fatalf("routed Shift+Right selection=(%d,%d) caret=%d want (0,2), caret 1", v.selectionStart, v.selectionLen, v.editCaret)
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

	if st.stream.totalRows() <= st.stream.visibleLines || st.stream.trackRect.Empty() {
		t.Fatalf("virtual edit scrollbar rows=%d visible=%d track=%v", st.stream.totalRows(), st.stream.visibleLines, st.stream.trackRect)
	}
	st.stream.scrollToBottom()
	ui.layoutFileViewerTextEditor(material.NewTheme(), gtx, st)
	lastVisibleRow := st.stream.displayTop + st.stream.renderedLineCount() - 1
	if lastVisibleRow < st.stream.totalRows()-3 {
		t.Fatalf("bottom visible row=%d want near %d", lastVisibleRow, st.stream.totalRows()-1)
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
	noWrapRows := st.stream.totalRows()
	if noWrapRows != 1 || st.stream.hTrackRect.Empty() {
		t.Fatalf("word wrap off rows=%d horizontal track=%v", noWrapRows, st.stream.hTrackRect)
	}
	virtualEditSetCaret(&st.stream, st.stream.totalBytes, false)
	st.revealVirtualEditCaret()
	layoutEditor()
	if st.stream.hCol <= 0 {
		t.Fatal("word wrap off should reveal the end of a long line horizontally")
	}

	st.wrapEnabled = true
	layoutEditor()
	wrapRows := st.stream.totalRows()
	if wrapRows <= noWrapRows*2 {
		t.Fatalf("word wrap on rows=%d want substantially above no-wrap rows=%d", wrapRows, noWrapRows)
	}
	if st.stream.hCol != 0 || !st.stream.hTrackRect.Empty() {
		t.Fatalf("word wrap on horizontal column=%d track=%v", st.stream.hCol, st.stream.hTrackRect)
	}
}

func TestFileViewerVirtualTextEditReplaceUndoRedo(t *testing.T) {
	content := "alpha\nŽeta\nomega"
	st := &fileViewerState{
		mode:             "file",
		path:             "unicode.txt",
		editBaselineText: content,
		content:          content,
	}
	st.contentEditor.SetText(content)
	st.stream.SetContent(content)
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	if !ui.startFileViewerEdit(time.Now()) {
		t.Fatalf("start edit: %s", st.status)
	}

	start := st.stream.lineByteStart(1)
	end := st.stream.lineByteEnd(1)
	if !ui.replaceFileViewerVirtualText(st, start, end, "delta\nextra", time.Now()) {
		t.Fatal("virtual replacement was not applied")
	}
	if got, want := st.virtualEditText(), "alpha\ndelta\nextra\nomega"; got != want {
		t.Fatalf("replaced text=%q want %q", got, want)
	}
	if !st.editDirty || st.editUndoIndex != 1 {
		t.Fatalf("replacement dirty=%v undo-index=%d", st.editDirty, st.editUndoIndex)
	}
	if !ui.undoFileViewerVirtualText(st, time.Now()) || st.virtualEditText() != content || st.editDirty {
		t.Fatalf("undo text=%q dirty=%v", st.virtualEditText(), st.editDirty)
	}
	if !ui.redoFileViewerVirtualText(st, time.Now()) || st.virtualEditText() != "alpha\ndelta\nextra\nomega" || !st.editDirty {
		t.Fatalf("redo text=%q dirty=%v", st.virtualEditText(), st.editDirty)
	}
}

func TestViewerEditorIndentationDetectsAndHonorsConfig(t *testing.T) {
	cfg := fm.DefaultConfig()
	style, size := viewerEditorIndentation(cfg, "root:\n  child:\n    value: yes\n")
	if style != fm.ViewerEditorIndentSpaces || size != 2 {
		t.Fatalf("space indentation=%q/%d want spaces/2", style, size)
	}
	style, size = viewerEditorIndentation(cfg, "root:\n\tchild:\n\t\tvalue: yes\n")
	if style != fm.ViewerEditorIndentTabs || size != 4 {
		t.Fatalf("tab indentation=%q/%d want tabs/4", style, size)
	}
	cfg.Viewer.EditorIndentStyle = fm.ViewerEditorIndentTabs
	cfg.Viewer.EditorTabSize = 8
	style, size = viewerEditorIndentation(cfg, "root:\n  child: yes\n")
	if style != fm.ViewerEditorIndentTabs || size != 8 {
		t.Fatalf("configured indentation=%q/%d want tabs/8", style, size)
	}
}

func TestFileViewerVirtualTabUsesDetectedIndentation(t *testing.T) {
	content := "root:\n  child:\n    value: yes"
	st := &fileViewerState{mode: "file", path: "config.yaml", editBaselineText: content, content: content}
	st.contentEditor.SetText(content)
	st.stream.SetContent(content)
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	if !ui.startFileViewerEdit(time.Now()) {
		t.Fatalf("start edit: %s", st.status)
	}
	if st.editIndentStyle != fm.ViewerEditorIndentSpaces || st.editTabSize != 2 {
		t.Fatalf("detected indentation=%q/%d", st.editIndentStyle, st.editTabSize)
	}
	caret := st.stream.lineByteEnd(1)
	virtualEditSetCaret(&st.stream, caret, false)
	if !ui.insertFileViewerVirtualTab(st, caret, caret, false, time.Now()) {
		t.Fatal("Tab was not applied")
	}
	if got, want := st.stream.lines[1], "  child:  "; got != want {
		t.Fatalf("line after Tab=%q want %q", got, want)
	}
	caret = st.stream.selHead
	if !ui.insertFileViewerVirtualTab(st, caret, caret, true, time.Now()) {
		t.Fatal("Shift+Tab was not applied")
	}
	if got, want := st.stream.lines[1], "  child:"; got != want {
		t.Fatalf("line after Shift+Tab=%q want %q", got, want)
	}
}

func TestFileViewerVirtualTabIndentsSelectedLines(t *testing.T) {
	content := "one\ntwo\nthree"
	st := &fileViewerState{editVirtualReady: true, editIndentStyle: fm.ViewerEditorIndentSpaces, editTabSize: 2}
	st.stream.SetContent(content)
	st.editVirtualReady = true
	st.editLineRunes = viewerEditLineRuneOffsetsFromLines(st.stream.lines)
	start := st.stream.lineByteStart(0)
	end := st.stream.lineByteEnd(1)
	st.stream.selActive = true
	st.stream.selAnchor, st.stream.selHead = start, end
	st.stream.updateSelectionRange()
	ui := NewUI(fm.DefaultConfig())
	if !ui.insertFileViewerVirtualTab(st, start, end, false, time.Now()) {
		t.Fatal("selection Tab was not applied")
	}
	if got, want := st.virtualEditText(), "  one\n  two\nthree"; got != want {
		t.Fatalf("indented text=%q want %q", got, want)
	}
	start, end = virtualEditSelection(&st.stream)
	if !ui.insertFileViewerVirtualTab(st, start, end, true, time.Now()) {
		t.Fatal("selection Shift+Tab was not applied")
	}
	if got := st.virtualEditText(); got != content {
		t.Fatalf("outdented text=%q want %q", got, content)
	}
}

func TestFileViewerVirtualEditPreservesSyntaxWhileReparseIsPending(t *testing.T) {
	content := "root:\n  child: value\nother: true"
	doc := viewerBuildSyntaxDocument(context.Background(), "config.yaml", content)
	if !doc.ready() {
		t.Fatal("YAML source should produce syntax spans")
	}
	st := &fileViewerState{mode: "file", path: "config.yaml", editBaselineText: content, content: content}
	st.contentEditor.SetText(content)
	st.stream.SetContent(content)
	st.stream.setSyntax(doc)
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	if !ui.startFileViewerEdit(time.Now()) {
		t.Fatalf("start edit: %s", st.status)
	}
	start := st.stream.lineByteStart(1) + strings.Index(st.stream.lines[1], "value")
	if !ui.replaceFileViewerVirtualText(st, start, start+1, "V", time.Now()) {
		t.Fatal("edit was not applied")
	}
	if !st.editSyntax.ready() || !st.stream.syntax.ready() {
		t.Fatal("syntax highlighting was cleared during the debounce window")
	}
	if len(st.editSyntax.lines) != len(st.stream.lines) || len(st.editSyntax.lines[0].spans) == 0 || len(st.editSyntax.lines[2].spans) == 0 {
		t.Fatalf("preserved syntax lines=%d first=%d last=%d", len(st.editSyntax.lines), len(st.editSyntax.lines[0].spans), len(st.editSyntax.lines[2].spans))
	}
}

func TestFileViewerVirtualTextEditRuneByteMapping(t *testing.T) {
	content := "aŽ🙂\nβeta"
	st := &fileViewerState{}
	st.contentEditor.SetText(content)
	st.initializeVirtualEditText(content)
	for runeOffset := 0; runeOffset <= utf8.RuneCountInString(content); runeOffset++ {
		byteOffset := st.virtualEditByteAtRune(runeOffset)
		if got := st.virtualEditRuneAtByte(byteOffset); got != runeOffset {
			t.Fatalf("rune %d -> byte %d -> rune %d", runeOffset, byteOffset, got)
		}
	}
}

func TestFileViewerVirtualTextEditMaintainsIncrementalMetadata(t *testing.T) {
	content := "zero\nalpha beta gamma\nŽeta🙂\nlast"
	st := &fileViewerState{}
	st.initializeVirtualEditText(content)
	st.stream.wrapEnabled = true
	st.stream.prepareWrapRows(6)

	applyAndCompare := func(start, end int, replacement string) {
		t.Helper()
		before := st.virtualEditText()
		replacement = normalizeViewerLineEndings(replacement)
		wantText := before[:start] + replacement + before[end:]
		st.applyVirtualEditReplacement(start, end, replacement)
		if got := st.virtualEditText(); got != wantText {
			t.Fatalf("text=%q want %q", got, wantText)
		}

		var rebuilt streamOutputView
		rebuilt.SetContent(wantText)
		rebuilt.wrapEnabled = true
		rebuilt.prepareWrapRows(6)
		if !reflect.DeepEqual(st.stream.lineOffsets, rebuilt.lineOffsets) {
			t.Fatalf("line offsets=%v want %v", st.stream.lineOffsets, rebuilt.lineOffsets)
		}
		if !reflect.DeepEqual(st.stream.lineRunes, rebuilt.lineRunes) {
			t.Fatalf("line runes=%v want %v", st.stream.lineRunes, rebuilt.lineRunes)
		}
		if st.stream.totalBytes != rebuilt.totalBytes || st.stream.maxCols != rebuilt.maxCols {
			t.Fatalf("totals=(%d,%d) want (%d,%d)", st.stream.totalBytes, st.stream.maxCols, rebuilt.totalBytes, rebuilt.maxCols)
		}
		if !reflect.DeepEqual(st.stream.wrapRows, rebuilt.wrapRows) {
			t.Fatalf("wrap rows=%v want %v", st.stream.wrapRows, rebuilt.wrapRows)
		}
		wantRuneOffsets := viewerEditLineRuneOffsetsFromView(&rebuilt)
		if !reflect.DeepEqual(st.editLineRunes, wantRuneOffsets) {
			t.Fatalf("edit rune offsets=%v want %v", st.editLineRunes, wantRuneOffsets)
		}
	}

	// Exercise the allocation-free, same-line typing path.
	insert := st.stream.lineByteStart(1) + len("alpha")
	applyAndCompare(insert, insert, "-typed-")
	// Exercise line insertion/removal and Unicode offset adjustment.
	current := st.virtualEditText()
	start := strings.Index(current, "beta")
	end := strings.Index(current, "🙂") + len("🙂")
	applyAndCompare(start, end, "new\nwrapped replacement\nrows")
	current = st.virtualEditText()
	newline := strings.Index(current, "\nwrapped")
	applyAndCompare(newline, newline+1, "")
}

func TestFileViewerTextEditCoalescesWindowResizeReflow(t *testing.T) {
	st := &fileViewerState{}
	now := time.Now()
	gtx := layout.Context{
		Ops: new(op.Ops),
		Now: now,
	}
	initialViewport := image.Pt(640, 400)
	initialLayout := image.Pt(640, 400)
	if got := st.fileViewerEditLayoutSize(gtx, initialViewport, initialLayout); got != initialLayout {
		t.Fatalf("initial layout size=%v want %v", got, initialLayout)
	}

	resizedViewport := image.Pt(520, 400)
	resizedLayout := image.Pt(520, 400)
	gtx.Now = now.Add(10 * time.Millisecond)
	if got := st.fileViewerEditLayoutSize(gtx, resizedViewport, resizedLayout); got != initialLayout {
		t.Fatalf("active resize layout size=%v want stable %v", got, initialLayout)
	}
	if st.editLayoutDue.IsZero() {
		t.Fatal("active resize did not schedule a settled reflow")
	}

	gtx.Now = st.editLayoutDue.Add(-time.Millisecond)
	if got := st.fileViewerEditLayoutSize(gtx, resizedViewport, resizedLayout); got != initialLayout {
		t.Fatalf("pre-settle layout size=%v want stable %v", got, initialLayout)
	}
	gtx.Now = st.editLayoutDue
	if got := st.fileViewerEditLayoutSize(gtx, resizedViewport, resizedLayout); got != resizedLayout {
		t.Fatalf("settled layout size=%v want %v", got, resizedLayout)
	}
	if !st.editLayoutDue.IsZero() {
		t.Fatal("settled reflow deadline was not cleared")
	}
}

func TestViewerFunctionBarUsesViewerSpecificCommands(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.configPath = filepath.Join(t.TempDir(), "hexone.yaml")
	ui.Tabs = widget.Enum{Value: "tab0"}
	ui.fileViewer = &fileViewerState{mode: "file"}

	specs := ui.functionBarButtonSpecs()
	want := []string{"Help", "Save", "Close", "Edit", "Lines", "", "Find", "Hex", "Wrap", "Exit"}
	if len(specs) != len(want) {
		t.Fatalf("viewer function count=%d want %d", len(specs), len(want))
	}
	for i, label := range want {
		if specs[i].keyLabel != fmt.Sprintf("F%d", i+1) || specs[i].label != label {
			t.Fatalf("viewer F%d=%q %q want %q", i+1, specs[i].keyLabel, specs[i].label, label)
		}
		if specs[i].activeFill != 0 {
			t.Fatalf("viewer F%d active fill=%v want no highlighted key", i+1, specs[i].activeFill)
		}
	}
	if specs[1].action != functionBarActionViewerSave || specs[2].action != functionBarActionView {
		t.Fatalf("viewer actions F2=%v F3=%v", specs[1].action, specs[2].action)
	}
	if specs[4].action != functionBarActionViewerLineNumbers || !specs[4].enabled ||
		specs[5].action != functionBarActionNone || specs[5].enabled {
		t.Fatalf("viewer F5 should toggle lines and F6 should be empty: F5=%#v F6=%#v", specs[4], specs[5])
	}
	if got := ui.functionBarActiveIndex(specs); got != -1 {
		t.Fatalf("viewer function bar active index=%d want no highlighted key", got)
	}

	ui.fileViewer.mode = "hex"
	ui.fileViewer.wrapEnabled = true
	specs = ui.functionBarButtonSpecs()
	if specs[7].label != "Text" || specs[8].label != "Unwrap" {
		t.Fatalf("viewer toggle labels F8=%q F9=%q", specs[7].label, specs[8].label)
	}

	ui.fileViewer.editMode = true
	specs = ui.functionBarButtonSpecs()
	if specs[2].label != "View" {
		t.Fatalf("viewer edit-mode F3 label=%q want View", specs[2].label)
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

func TestViewerF3ClosesViewerFromViewOnlyMode(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs = widget.Enum{Value: "tab0"}
	ui.fileViewer = &fileViewerState{mode: "file"}

	if !ui.performFunctionBarAction(functionBarActionView, time.Now()) {
		t.Fatal("F3 should be handled in view-only mode")
	}
	if ui.fileViewer != nil {
		t.Fatal("F3 should close the viewer from view-only mode")
	}
	if ui.ConsumeWindowCloseRequest() {
		t.Fatal("F3 should not exit the application")
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
