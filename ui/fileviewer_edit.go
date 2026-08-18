// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hexone/filesys"
	"hexone/fm"
	"io"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget/material"
	"golang.org/x/text/encoding/charmap"
)

const fileViewerEditSyntaxDelay = 160 * time.Millisecond

type fileViewerSaveResult struct {
	mode       string
	text       string
	revision   int64
	hexChanges map[int64]byte
	err        error
}

type fileViewerEditSyntaxResult struct {
	seq  int
	text string
	doc  viewerSyntaxDocument
}

func (ui *UI) fileViewerCanEdit(st *fileViewerState) (bool, string) {
	if st == nil {
		return false, "viewer is not open"
	}
	if st.commandOnly || st.mode == "command" {
		return false, "command output is read-only"
	}
	if filesys.ArchiveMemberPath(st.path) {
		return false, "files inside archives are read-only"
	}
	if st.loading {
		return false, "wait for the file to finish loading"
	}
	switch st.mode {
	case "file":
		if st.detectedImagePreview {
			return false, "image and PDF previews are read-only"
		}
		if st.detectedBinaryPreview {
			return false, "binary preview is read-only; use Hex mode"
		}
		return true, ""
	case "hex":
		if st.hex == nil || st.hex.fileSize <= 0 {
			return false, "empty files have no bytes to edit in Hex mode"
		}
		return true, ""
	default:
		return false, "this viewer mode is read-only"
	}
}

func (ui *UI) startFileViewerEdit(now time.Time) bool {
	st := ui.fileViewer
	ok, reason := ui.fileViewerCanEdit(st)
	if !ok {
		if st != nil {
			st.status = reason
		}
		return false
	}
	if st.editMode {
		return true
	}
	st.closeEncodingMenu()
	if st.find.open {
		ui.closeFileViewerFind()
	}
	st.editMode = true
	st.editFocus = true
	st.status = "editing"
	st.err = ""
	st.nextWatchCheck = time.Time{}
	if st.mode == "file" {
		initialLine := 0
		if len(st.stream.lines) > 0 {
			topRow := st.stream.topLine
			if topRow < 0 {
				topRow = 0
			}
			if totalRows := st.stream.totalRows(); totalRows > 0 {
				if topRow >= totalRows {
					topRow = totalRows - 1
				}
				initialLine = st.stream.rowAt(topRow).line
			}
		}
		st.contentEditor.ReadOnly = false
		if !st.editDirty {
			st.contentEditor.SetText(st.editBaselineText)
		}
		syntax := st.stream.syntax
		current := st.contentEditor.Text()
		// The read-only viewer renders sanitized text (notably, tabs become
		// spaces), while the editor must preserve the original bytes. Syntax
		// spans from the viewer therefore cannot be reused when those texts
		// differ: their byte offsets may point beyond the editor's lines.
		syntaxNeedsRefresh := st.content != current
		if syntaxNeedsRefresh {
			syntax = viewerSyntaxDocument{}
		}
		st.initializeVirtualEditText(current)
		st.editSyntax = syntax
		st.stream.setSyntax(st.editSyntax)
		if syntaxNeedsRefresh {
			st.editSyntaxDue = now
		} else {
			st.editSyntaxDue = time.Time{}
		}
		st.editIndentStyle, st.editTabSize = viewerEditorIndentation(ui.fmCfg, current)
		if st.editSyntaxCh == nil {
			st.editSyntaxCh = make(chan fileViewerEditSyntaxResult, 1)
		}
		st.editUndo = nil
		st.editUndoIndex = 0
		st.editRevision = 0
		st.editSavedRevision = 0
		st.editNextRevision = 0
		if current != st.editBaselineText {
			st.editRevision = 1
			st.editNextRevision = 1
		}
		caret := st.stream.lineByteStart(initialLine)
		if caret < 0 {
			caret = 0
		}
		st.stream.beginSelection(caret)
		st.editCaretBlinkAt = now
	} else if st.hex != nil {
		v := st.hex
		if v.edits == nil {
			v.edits = make(map[int64]byte)
		}
		selectionLen := v.selectionLen
		caret := v.selectionStart
		if !v.hasSelection() {
			caret = v.topLine * int64(max(1, v.bytesPerLine))
		}
		v.editCaret = v.clampByteOffset(caret)
		v.editNibble = 0
		v.editASCII = false
		if selectionLen <= 0 {
			v.setSelectionRange(v.editCaret, 1)
		}
	}
	return true
}

func viewerEditorIndentation(cfg *fm.Config, content string) (string, int) {
	style := fm.ViewerEditorIndentAuto
	tabSize := 4
	if cfg != nil {
		style = fm.NormalizeViewerEditorIndentStyle(cfg.Viewer.EditorIndentStyle)
		tabSize = fm.NormalizeViewerEditorTabSize(cfg.Viewer.EditorTabSize)
	}
	if style != fm.ViewerEditorIndentAuto {
		return style, tabSize
	}

	spaceLines := 0
	tabLines := 0
	detectedSize := 0
	spaceIndents := make([]int, 0, 32)
	for index, line := range strings.Split(content, "\n") {
		if index >= 4000 {
			break
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		spaces := 0
		for spaces < len(line) && line[spaces] == ' ' {
			spaces++
		}
		switch {
		case spaces > 0:
			spaceLines++
			if spaces <= 16 {
				detectedSize = viewerIndentGCD(detectedSize, spaces)
				spaceIndents = append(spaceIndents, spaces)
			}
		case line[0] == '\t':
			tabLines++
		}
	}
	if tabLines > spaceLines {
		return fm.ViewerEditorIndentTabs, tabSize
	}
	if detectedSize == 1 && len(spaceIndents) > 1 {
		bestSize, bestScore := 1, 0
		for candidate := 2; candidate <= 8; candidate++ {
			score := 0
			for _, indent := range spaceIndents {
				if indent%candidate == 0 {
					score++
				}
			}
			if score > bestScore || score == bestScore && candidate > bestSize {
				bestSize, bestScore = candidate, score
			}
		}
		if bestScore >= 2 && bestScore*3 >= len(spaceIndents)*2 {
			detectedSize = bestSize
		}
	}
	if detectedSize >= 1 && detectedSize <= 16 {
		tabSize = detectedSize
	}
	return fm.ViewerEditorIndentSpaces, tabSize
}

func viewerIndentGCD(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func (ui *UI) stopFileViewerEdit() bool {
	st := ui.fileViewer
	if st == nil || !st.editMode {
		return false
	}
	if st.mode == "file" {
		ui.syncFileViewerTextEdit(st)
		st.contentEditor.ReadOnly = true
		st.editableContent = st.virtualEditText()
		st.contentEditor.SetText(st.editableContent)
		st.editWidgetMirrorText = st.editableContent
		st.content = sanitizeViewerContent(st.editableContent)
		st.stream.tabCols = 0
		st.stream.SetContent(st.content)
		ui.applyFileViewerViewSyntax(st)
		st.markdown.setSource(st.path, st.editableContent)
	}
	st.editMode = false
	st.editFocus = false
	if st.editDirty {
		st.status = "modified"
	} else {
		st.status = "ready"
	}
	return true
}

// applyFileViewerViewSyntax installs a syntax document that matches the text the
// read-only viewer is about to show. The editor keeps the file's raw bytes while
// the viewer shows a sanitized copy, so a document built for one text addresses
// the wrong offsets in the other: the painter draws only span-covered ranges, so
// stale spans paint shifted slices and silently drop the tail of every line.
func (ui *UI) applyFileViewerViewSyntax(st *fileViewerState) {
	if st == nil {
		return
	}
	// The editor's document is reusable only when it was built from exactly the
	// text the viewer now shows and no rebuild is still outstanding; a pending or
	// in-flight build means it predates the last keystrokes. Bumping the sequence
	// makes the pump discard whatever that build eventually returns.
	reusable := st.editSyntax.ready() && st.content == st.editableContent &&
		st.editSyntaxDue.IsZero() && !st.editSyntaxRunning
	if !reusable {
		st.editSyntaxSeq++
		st.editSyntaxDue = time.Time{}
		st.editSyntax = viewerBuildSyntaxDocument(context.Background(), st.path, st.content)
	}
	st.stream.setSyntax(st.editSyntax)
}

func (ui *UI) toggleFileViewerEdit(now time.Time) bool {
	st := ui.fileViewer
	if st == nil {
		return false
	}
	if !st.editMode {
		return ui.startFileViewerEdit(now)
	}
	if st.markdown.detected {
		// Returning to Markdown preview must retain the current edit buffer so
		// F4 -> edit -> F3 can be used as a live preview workflow.
		return ui.stopFileViewerEdit()
	}
	ui.discardFileViewerChanges(st)
	return ui.stopFileViewerEdit()
}

func (ui *UI) discardFileViewerChanges(st *fileViewerState) bool {
	if st == nil {
		return false
	}
	hadChanges := st.editDirty
	switch st.mode {
	case "file":
		if st.virtualEditText() != st.editBaselineText {
			hadChanges = true
		}
		st.contentEditor.SetText(st.editBaselineText)
		st.initializeVirtualEditText(st.editBaselineText)
		st.editableContent = st.editBaselineText
		st.content = sanitizeViewerContent(st.editBaselineText)
		st.markdown.setSource(st.path, st.editableContent)
		// The stream now holds the raw baseline text, so any document built for
		// the previous buffer indexes different bytes. Drop it and let the editor
		// rebuild off the UI thread; leaving edit mode resolves it synchronously
		// against the viewer's sanitized text instead.
		st.editSyntax = viewerSyntaxDocument{}
		st.stream.clearSyntax()
		st.editLineRunes = viewerEditLineRuneOffsetsFromLines(st.stream.lines)
		st.editSyntaxSeq++
		st.editSyntaxDue = time.Time{}
		if st.editMode && st.editSyntaxCh != nil {
			st.editSyntaxDue = time.Now()
		}
		st.editUndo = nil
		st.editUndoIndex = 0
		st.editRevision = 0
		st.editSavedRevision = 0
		st.editNextRevision = 0
	case "hex":
		if st.hex != nil {
			if len(st.hex.edits) > 0 {
				hadChanges = true
			}
			st.hex.edits = nil
			st.hex.editNibble = 0
		}
	}
	st.editDirty = false
	st.err = ""
	if st.editMode {
		st.status = "editing"
	} else if hadChanges {
		st.status = "changes discarded"
	} else {
		st.status = "ready"
	}
	return hadChanges
}

func (ui *UI) syncFileViewerTextEdit(st *fileViewerState) {
	if st == nil || st.mode != "file" {
		return
	}
	widgetText := st.contentEditor.Text()
	if !st.editVirtualReady || widgetText != st.editWidgetMirrorText {
		st.initializeVirtualEditText(widgetText)
		st.editRevision++
		if st.editRevision <= 0 {
			st.editRevision = 1
		}
		st.editNextRevision = max(st.editNextRevision, st.editRevision)
	}
	ui.syncFileViewerTextEditContent(st, st.virtualEditText())
}

func (ui *UI) syncFileViewerTextEditContent(st *fileViewerState, current string) {
	if st == nil || st.mode != "file" {
		return
	}
	st.editDirty = current != st.editBaselineText
	if st.editDirty && !st.saving {
		st.status = "modified"
	}
}

func (ui *UI) layoutFileViewerTextEditor(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	return ui.layoutEditorWithContextMenu(th, gtx, "viewer-file-edit", &st.contentEditor, true, func(gtx layout.Context) layout.Dimensions {
		return ui.layoutFileViewerVirtualTextEditor(th, gtx, st)
	})
}

func (ui *UI) startFileViewerEditSyntaxIfDue(st *fileViewerState, gtx layout.Context) {
	if st == nil || st.editSyntaxDue.IsZero() {
		return
	}
	if st.editSyntaxRunning {
		return
	}
	if gtx.Now.Before(st.editSyntaxDue) {
		gtx.Execute(op.InvalidateCmd{At: st.editSyntaxDue})
		return
	}
	st.editSyntaxDue = time.Time{}
	st.editSyntaxRunning = true
	seq := st.editSyntaxSeq
	path := st.path
	content := st.virtualEditText()
	ch := st.editSyntaxCh
	go func() {
		res := fileViewerEditSyntaxResult{
			seq:  seq,
			text: content,
			doc:  viewerBuildSyntaxDocument(context.Background(), path, content),
		}
		select {
		case ch <- res:
		default:
			select {
			case <-ch:
			default:
			}
			ch <- res
		}
		ui.invalidateFromWorker()
	}()
}

func (ui *UI) pumpFileViewerEditSyntax(st *fileViewerState) {
	if st == nil || st.editSyntaxCh == nil {
		return
	}
	for {
		select {
		case res := <-st.editSyntaxCh:
			st.editSyntaxRunning = false
			if res.seq == st.editSyntaxSeq {
				st.editSyntax = res.doc
				st.stream.setSyntax(res.doc)
			} else if st.editSyntaxDue.IsZero() {
				st.editSyntaxDue = time.Now().Add(fileViewerEditSyntaxDelay)
			}
		default:
			return
		}
	}
}

func viewerEditLineAtRune(lineRunes []int, runeOffset int) int {
	if len(lineRunes) == 0 {
		return 0
	}
	line := sort.Search(len(lineRunes), func(i int) bool {
		return lineRunes[i] > runeOffset
	}) - 1
	if line < 0 {
		return 0
	}
	if line >= len(lineRunes) {
		return len(lineRunes) - 1
	}
	return line
}

func (ui *UI) startFileViewerSave(now time.Time) bool {
	st := ui.fileViewer
	if st == nil {
		return false
	}
	if st.editMode && st.mode == "file" {
		ui.syncFileViewerTextEdit(st)
	}
	if st.saving {
		st.status = "saving"
		return false
	}
	if !st.editDirty {
		st.status = "no changes"
		return false
	}
	if filesys.ArchiveMemberPath(st.path) {
		st.err = "files inside archives are read-only"
		return false
	}
	if st.saveCh == nil {
		st.saveCh = make(chan fileViewerSaveResult, 1)
	}
	st.saving = true
	st.status = "saving"
	st.err = ""

	path := st.path
	remote := st.remote
	ch := st.saveCh
	mode := st.mode
	if mode == "file" {
		textSnapshot := st.virtualEditText()
		revision := st.editRevision
		encoding := st.detectedEncoding
		if encoding == "" {
			encoding = fm.NormalizeViewerFileEncoding(st.fileEncoding)
		}
		withBOM := st.detectedEncodingBOM
		lineEnding := st.detectedLineEnding
		go func() {
			data, err := encodeViewerText(textSnapshot, encoding, withBOM, lineEnding)
			if err == nil {
				err = writeViewerFile(path, remote, data)
			}
			ch <- fileViewerSaveResult{mode: "file", text: textSnapshot, revision: revision, err: err}
			ui.invalidateFromWorker()
		}()
	} else if mode == "hex" && st.hex != nil {
		changes := cloneViewerHexChanges(st.hex.edits)
		go func() {
			err := writeViewerHexChanges(path, remote, changes)
			ch <- fileViewerSaveResult{mode: "hex", hexChanges: changes, err: err}
			ui.invalidateFromWorker()
		}()
	} else {
		st.saving = false
		st.err = "this viewer mode is read-only"
		return false
	}
	_ = now
	return true
}

func (ui *UI) pumpFileViewerSaveState(gtx layout.Context, st *fileViewerState) {
	if st == nil || st.saveCh == nil {
		return
	}
	for {
		select {
		case res := <-st.saveCh:
			st.saving = false
			if res.err != nil {
				st.err = "save failed: " + res.err.Error()
				switch res.mode {
				case "file":
					st.editDirty = st.editRevision != st.editSavedRevision
				case "hex":
					st.editDirty = st.hex != nil && len(st.hex.edits) > 0
				}
				if st.editDirty {
					st.status = "modified"
				} else if st.editMode {
					st.status = "editing"
				} else {
					st.status = "ready"
				}
				ui.finishFileViewerModeSwitchSave(st, false, gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			st.err = ""
			switch res.mode {
			case "file":
				st.editBaselineText = res.text
				st.editableContent = res.text
				st.editSavedRevision = res.revision
				st.editDirty = st.editRevision != st.editSavedRevision
				if !st.editMode {
					st.content = sanitizeViewerContent(st.editableContent)
					st.stream.SetContent(st.content)
					ui.applyFileViewerViewSyntax(st)
				}
			case "hex":
				applySavedViewerHexChanges(st.hex, res.hexChanges)
				st.editDirty = st.hex != nil && len(st.hex.edits) > 0
			}
			if st.editDirty {
				st.status = "modified"
			} else if st.editMode {
				st.status = "editing"
			} else {
				st.status = "saved"
			}
			if st.remote == nil {
				ui.refreshLocalFilePanesForPath(st.path)
			}
			st.captureWatchState()
			ui.finishFileViewerModeSwitchSave(st, true, gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		default:
			return
		}
	}
}

func encodeViewerText(raw, encoding string, withBOM bool, lineEnding string) ([]byte, error) {
	raw = normalizeViewerLineEndings(raw)
	if lineEnding == viewerLineEndingCRLF {
		raw = strings.ReplaceAll(raw, "\n", "\r\n")
	}
	switch fm.NormalizeViewerFileEncoding(encoding) {
	case fm.ViewerFileEncodingUTF16LE:
		return encodeViewerUTF16(raw, binary.LittleEndian, withBOM), nil
	case fm.ViewerFileEncodingUTF16BE:
		return encodeViewerUTF16(raw, binary.BigEndian, withBOM), nil
	case fm.ViewerFileEncodingCP437:
		return charmap.CodePage437.NewEncoder().Bytes([]byte(raw))
	default:
		data := []byte(raw)
		if withBOM {
			data = append([]byte{0xEF, 0xBB, 0xBF}, data...)
		}
		return data, nil
	}
}

func encodeViewerUTF16(raw string, order binary.ByteOrder, withBOM bool) []byte {
	units := utf16.Encode([]rune(raw))
	extra := 0
	if withBOM {
		extra = 2
	}
	out := make([]byte, extra+len(units)*2)
	if withBOM {
		if order == binary.LittleEndian {
			copy(out, []byte{0xFF, 0xFE})
		} else {
			copy(out, []byte{0xFE, 0xFF})
		}
	}
	for i, value := range units {
		order.PutUint16(out[extra+i*2:], value)
	}
	return out
}

func writeViewerFile(path string, remote *paneSSHSession, data []byte) error {
	if remote != nil {
		client := remote.sftpClient()
		if client == nil {
			return errors.New("sftp session is not connected")
		}
		file, err := client.OpenFile(path, os.O_WRONLY|os.O_TRUNC)
		if err != nil {
			return err
		}
		if _, err = io.Copy(file, bytes.NewReader(data)); err != nil {
			_ = file.Close()
			return err
		}
		return file.Close()
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	if _, err = io.Copy(file, bytes.NewReader(data)); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func cloneViewerHexChanges(changes map[int64]byte) map[int64]byte {
	out := make(map[int64]byte, len(changes))
	for off, value := range changes {
		out[off] = value
	}
	return out
}

type viewerHexPatchRange struct {
	offset int64
	data   []byte
}

func buildViewerHexPatch(changes map[int64]byte) []viewerHexPatchRange {
	if len(changes) == 0 {
		return nil
	}
	offsets := make([]int64, 0, len(changes))
	for off := range changes {
		offsets = append(offsets, off)
	}
	sort.Slice(offsets, func(i, j int) bool { return offsets[i] < offsets[j] })

	patch := make([]viewerHexPatchRange, 0, len(offsets))
	for i := 0; i < len(offsets); {
		start := offsets[i]
		data := []byte{changes[start]}
		i++
		for i < len(offsets) && offsets[i] == start+int64(len(data)) {
			data = append(data, changes[offsets[i]])
			i++
		}
		patch = append(patch, viewerHexPatchRange{offset: start, data: data})
	}
	return patch
}

func writeViewerHexChanges(path string, remote *paneSSHSession, changes map[int64]byte) error {
	patch := buildViewerHexPatch(changes)
	if len(patch) == 0 {
		return nil
	}
	var file interface {
		WriteAt([]byte, int64) (int, error)
		Close() error
	}
	if remote != nil {
		client := remote.sftpClient()
		if client == nil {
			return errors.New("sftp session is not connected")
		}
		f, err := client.OpenFile(path, os.O_WRONLY)
		if err != nil {
			return err
		}
		file = f
	} else {
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		file = f
	}
	for _, run := range patch {
		n, err := file.WriteAt(run.data, run.offset)
		if err != nil {
			_ = file.Close()
			return err
		}
		if n != len(run.data) {
			_ = file.Close()
			return io.ErrShortWrite
		}
	}
	if syncer, ok := file.(interface{ Sync() error }); ok {
		if err := syncer.Sync(); err != nil {
			_ = file.Close()
			return err
		}
	}
	return file.Close()
}

func applySavedViewerHexChanges(v *hexViewerState, saved map[int64]byte) {
	if v == nil {
		return
	}
	for off, value := range saved {
		if off >= v.bufferStart && off < v.bufferStart+int64(len(v.buffer)) {
			v.buffer[off-v.bufferStart] = value
		}
		if current, ok := v.edits[off]; ok && current == value {
			delete(v.edits, off)
		}
	}
	if len(v.edits) == 0 {
		v.edits = nil
	}
}

func (ui *UI) handleFileViewerHexEditKey(st *fileViewerState, ke key.Event) bool {
	if st == nil || !st.editMode || st.mode != "hex" || st.hex == nil || ke.State != key.Press {
		return false
	}
	v := st.hex
	if ke.Modifiers&^key.ModShift != 0 {
		return false
	}
	extend := ke.Modifiers.Contain(key.ModShift)
	selectionStart, selectionEnd := v.selectionStart, v.selectionEnd()
	selectionAnchor := v.editCaret
	if extend && v.selectionLen > 1 {
		switch v.editCaret {
		case selectionStart:
			selectionAnchor = selectionEnd - 1
		case selectionEnd - 1:
			selectionAnchor = selectionStart
		default:
			selectionAnchor = selectionStart
		}
	}

	target := v.editCaret
	step := int64(0)
	switch ke.Name {
	case key.NameLeftArrow:
		if !extend && v.selectionLen > 1 {
			target = selectionStart
		} else {
			step = -1
		}
	case key.NameRightArrow:
		if !extend && v.selectionLen > 1 {
			target = selectionEnd - 1
		} else {
			step = 1
		}
	case key.NameUpArrow:
		step = -int64(max(1, v.bytesPerLine))
	case key.NameDownArrow:
		step = int64(max(1, v.bytesPerLine))
	case key.NamePageUp:
		step = -int64(max(1, v.bytesPerLine) * max(1, v.visibleLines))
	case key.NamePageDown:
		step = int64(max(1, v.bytesPerLine) * max(1, v.visibleLines))
	case key.NameHome:
		target -= target % int64(max(1, v.bytesPerLine))
	case key.NameEnd:
		target = target - target%int64(max(1, v.bytesPerLine)) + int64(max(1, v.bytesPerLine)-1)
	default:
		return false
	}
	v.editCaret = v.clampByteOffset(target + step)
	v.editNibble = 0
	if extend {
		v.setSelectionFromAnchor(selectionAnchor, v.editCaret)
	} else {
		v.setSelectionRange(v.editCaret, 1)
	}
	v.revealByte(v.editCaret)
	return true
}

func (ui *UI) handleFileViewerHexEditText(st *fileViewerState, input string) bool {
	if st == nil || !st.editMode || st.mode != "hex" || st.hex == nil || input == "" {
		return false
	}
	v := st.hex
	handled := false
	for _, r := range input {
		if v.editASCII {
			if r < ' ' || r > unicode.MaxASCII || !unicode.IsPrint(r) {
				continue
			}
			if v.selectionLen > 1 {
				if !applyFileViewerHexASCIISelection(v, byte(r)) {
					st.status = "selection is not loaded"
					continue
				}
				handled = true
				continue
			}
			if v.edits == nil {
				v.edits = make(map[int64]byte)
			}
			v.setEditedByte(v.editCaret, byte(r))
			ui.advanceFileViewerHexCaret(v)
			handled = true
			continue
		}
		digit, ok := viewerHexDigitRune(r)
		if !ok {
			continue
		}
		if v.selectionLen > 1 {
			if !applyFileViewerHexNibbleSelection(v, digit) {
				st.status = "selection is not loaded"
				continue
			}
			handled = true
			continue
		}
		if v.edits == nil {
			v.edits = make(map[int64]byte)
		}
		current, ok := v.byteAt(v.editCaret)
		if !ok {
			continue
		}
		if v.editNibble == 0 {
			current = (current & 0x0F) | byte(digit<<4)
			v.editNibble = 1
		} else {
			current = (current & 0xF0) | byte(digit)
			v.editNibble = 0
		}
		v.setEditedByte(v.editCaret, current)
		if v.editNibble == 0 {
			ui.advanceFileViewerHexCaret(v)
		} else {
			v.setSelectionRange(v.editCaret, 1)
		}
		handled = true
	}
	if !handled {
		return false
	}
	v.revealByte(v.editCaret)
	ui.updateFileViewerHexDirtyState(st)
	return true
}

func applyFileViewerHexASCIISelection(v *hexViewerState, value byte) bool {
	if v == nil || v.selectionLen <= 1 || v.selectionStart < 0 || v.selectionEnd() > v.fileSize {
		return false
	}
	for off := v.selectionStart; off < v.selectionEnd(); off++ {
		if _, ok := v.byteAt(off); !ok {
			return false
		}
	}
	for off := v.selectionStart; off < v.selectionEnd(); off++ {
		v.setEditedByte(off, value)
	}
	v.editNibble = 0
	return true
}

func applyFileViewerHexNibbleSelection(v *hexViewerState, digit int) bool {
	if v == nil || digit < 0 || digit > 0x0F || v.selectionLen <= 1 || v.selectionStart < 0 || v.selectionEnd() > v.fileSize {
		return false
	}
	values := make([]byte, v.selectionLen)
	for i := int64(0); i < v.selectionLen; i++ {
		value, ok := v.byteAt(v.selectionStart + i)
		if !ok {
			return false
		}
		values[i] = value
	}
	for i, value := range values {
		if v.editNibble == 0 {
			value = (value & 0x0F) | byte(digit<<4)
		} else {
			value = (value & 0xF0) | byte(digit)
		}
		v.setEditedByte(v.selectionStart+int64(i), value)
	}
	if v.editNibble == 0 {
		v.editNibble = 1
	} else {
		v.editNibble = 0
	}
	return true
}

func (ui *UI) advanceFileViewerHexCaret(v *hexViewerState) {
	if v == nil {
		return
	}
	if v.editCaret+1 < v.fileSize {
		v.editCaret++
	}
	v.editNibble = 0
	v.setSelectionRange(v.editCaret, 1)
}

func (ui *UI) updateFileViewerHexDirtyState(st *fileViewerState) {
	if st == nil || st.hex == nil {
		return
	}
	st.editDirty = len(st.hex.edits) > 0
	if st.editDirty {
		st.status = "modified"
	} else {
		st.status = "editing"
	}
}

func viewerHexDigitRune(r rune) (int, bool) {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0'), true
	case r >= 'a' && r <= 'f':
		return int(r-'a') + 10, true
	case r >= 'A' && r <= 'F':
		return int(r-'A') + 10, true
	default:
		return 0, false
	}
}

func (v *hexViewerState) byteAt(off int64) (byte, bool) {
	if v == nil || off < 0 || off >= v.fileSize {
		return 0, false
	}
	if value, ok := v.edits[off]; ok {
		return value, true
	}
	if off < v.bufferStart || off >= v.bufferStart+int64(len(v.buffer)) {
		return 0, false
	}
	return v.buffer[off-v.bufferStart], true
}

func (v *hexViewerState) setEditedByte(off int64, value byte) {
	if v == nil {
		return
	}
	if off >= v.bufferStart && off < v.bufferStart+int64(len(v.buffer)) && v.buffer[off-v.bufferStart] == value {
		delete(v.edits, off)
		return
	}
	if v.edits == nil {
		v.edits = make(map[int64]byte)
	}
	v.edits[off] = value
}

func (v *hexViewerState) revealByte(off int64) {
	if v == nil || v.bytesPerLine <= 0 {
		return
	}
	line := off / int64(v.bytesPerLine)
	if line < v.topLine {
		v.topLine = line
	} else if line >= v.topLine+int64(max(1, v.visibleLines)) {
		v.topLine = line - int64(max(1, v.visibleLines)) + 1
	}
	v.clampTop()
	v.syncVisualTop()
}
