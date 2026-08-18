// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"image"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget/material"
)

const fileViewerTextUndoLimit = 256

type fileViewerTextUndoRecord struct {
	start           int
	before          string
	after           string
	selectionBefore streamSelectionState
	selectionAfter  streamSelectionState
	revisionBefore  int64
	revisionAfter   int64
}

func (st *fileViewerState) virtualEditText() string {
	if st == nil {
		return ""
	}
	if !st.editVirtualReady {
		return st.contentEditor.Text()
	}
	return strings.Join(st.stream.lines, "\n")
}

func (st *fileViewerState) initializeVirtualEditText(content string) {
	if st == nil {
		return
	}
	// The editor buffer keeps the file's original bytes, so the view has to lay
	// its tabs out on tab stops to reach the columns the sanitized read-only
	// text already occupies.
	st.stream.tabCols = viewerTabColumns
	st.stream.SetContent(content)
	st.editVirtualReady = true
	st.editWidgetMirrorText = content
	st.editLineRunes = viewerEditLineRuneOffsetsFromLines(st.stream.lines)
	st.editDesiredColSet = false
	if !st.stream.selActive {
		st.stream.beginSelection(0)
	}
}

func viewerEditLineRuneOffsetsFromLines(lines []string) []int {
	if len(lines) == 0 {
		return []int{0}
	}
	offsets := make([]int, len(lines))
	runeOffset := 0
	for i, line := range lines {
		offsets[i] = runeOffset
		runeOffset += utf8.RuneCountInString(line)
		if i+1 < len(lines) {
			runeOffset++
		}
	}
	return offsets
}

func viewerEditLineRuneOffsetsFromView(v *streamOutputView) []int {
	if v == nil || len(v.lines) == 0 {
		return []int{0}
	}
	if len(v.lineRunes) != len(v.lines) {
		return viewerEditLineRuneOffsetsFromLines(v.lines)
	}
	offsets := make([]int, len(v.lines))
	runeOffset := 0
	for i, runes := range v.lineRunes {
		offsets[i] = runeOffset
		runeOffset += runes
		if i+1 < len(v.lines) {
			runeOffset++
		}
	}
	return offsets
}

func (st *fileViewerState) virtualEditTotalRunes() int {
	if st == nil || len(st.stream.lines) == 0 {
		return 0
	}
	if len(st.editLineRunes) != len(st.stream.lines) {
		st.editLineRunes = viewerEditLineRuneOffsetsFromLines(st.stream.lines)
	}
	last := len(st.stream.lines) - 1
	return st.editLineRunes[last] + utf8.RuneCountInString(st.stream.lines[last])
}

func (st *fileViewerState) virtualEditByteAtRune(runeOffset int) int {
	if st == nil || len(st.stream.lines) == 0 {
		return 0
	}
	total := st.virtualEditTotalRunes()
	if runeOffset < 0 {
		runeOffset = 0
	}
	if runeOffset > total {
		runeOffset = total
	}
	line := viewerEditLineAtRune(st.editLineRunes, runeOffset)
	localRune := runeOffset - st.editLineRunes[line]
	if localRune < 0 {
		localRune = 0
	}
	lineRunes := utf8.RuneCountInString(st.stream.lines[line])
	if localRune > lineRunes {
		localRune = lineRunes
	}
	return st.stream.lineByteStart(line) + byteIndexAtRune(st.stream.lines[line], localRune)
}

func (st *fileViewerState) virtualEditRuneAtByte(byteOffset int) int {
	if st == nil || len(st.stream.lines) == 0 {
		return 0
	}
	if len(st.editLineRunes) != len(st.stream.lines) {
		st.editLineRunes = viewerEditLineRuneOffsetsFromLines(st.stream.lines)
	}
	line, local, ok := st.stream.lineForOffset(byteOffset)
	if !ok {
		return 0
	}
	return st.editLineRunes[line] + runeIndexAtByte(st.stream.lines[line], local)
}

func virtualEditRangeText(v *streamOutputView, start, end int) string {
	if v == nil || len(v.lines) == 0 {
		return ""
	}
	start = v.clampOffset(start)
	end = v.clampOffset(end)
	if end < start {
		start, end = end, start
	}
	if end <= start {
		return ""
	}
	startLine, startLocal, _ := v.lineForOffset(start)
	endLine, endLocal, _ := v.lineForOffset(end)
	if startLine == endLine {
		return v.lines[startLine][startLocal:endLocal]
	}
	var out strings.Builder
	out.Grow(end - start)
	out.WriteString(v.lines[startLine][startLocal:])
	for line := startLine + 1; line < endLine; line++ {
		out.WriteByte('\n')
		out.WriteString(v.lines[line])
	}
	out.WriteByte('\n')
	out.WriteString(v.lines[endLine][:endLocal])
	return out.String()
}

func (st *fileViewerState) applyVirtualEditReplacement(start, end int, replacement string) {
	if st == nil {
		return
	}
	v := &st.stream
	if len(v.lines) == 0 {
		v.SetContent("")
	}
	start = v.clampOffset(start)
	end = v.clampOffset(end)
	if end < start {
		start, end = end, start
	}
	replacement = normalizeViewerLineEndings(replacement)
	startLine, startLocal, _ := v.lineForOffset(start)
	endLine, endLocal, _ := v.lineForOffset(end)
	oldSyntax := v.syntax
	oldChangedLines := append([]string(nil), v.lines[startLine:endLine+1]...)
	prefix := v.lines[startLine][:startLocal]
	suffix := v.lines[endLine][endLocal:]
	replacementLines := strings.Split(prefix+replacement+suffix, "\n")
	oldLines := v.lines
	oldOffsets := v.lineOffsets
	oldRunes := v.lineRunes
	oldWidths := v.lineWidths
	oldTop := v.topLine
	oldWrapRows := v.wrapRows
	hadWrapCache := v.wrapEnabled && v.wrapCols > 0 && len(oldWrapRows) > 0
	oldWrapStart, oldWrapEnd := 0, 0
	if hadWrapCache {
		oldWrapStart = sort.Search(len(oldWrapRows), func(i int) bool {
			return oldWrapRows[i].line >= startLine
		})
		oldWrapEnd = sort.Search(len(oldWrapRows), func(i int) bool {
			return oldWrapRows[i].line > endLine
		})
	}

	removedLineCount := endLine - startLine + 1
	lineDelta := len(replacementLines) - removedLineCount
	fastSingleLine := startLine == endLine && len(replacementLines) == 1
	if fastSingleLine {
		// Normal typing stays entirely in the existing document arrays. Only
		// byte/rune offsets after the edited line need an integer adjustment.
		oldLineRunes := oldRunes[startLine]
		oldLineCols := oldWidths[startLine]
		newLine := replacementLines[0]
		newLineRunes := utf8.RuneCountInString(newLine)
		newLineCols := v.lineWidth(newLine)
		byteDelta := len(newLine) - len(oldLines[startLine])
		runeDelta := newLineRunes - oldLineRunes
		v.lines[startLine] = newLine
		v.lineRunes[startLine] = newLineRunes
		v.lineWidths[startLine] = newLineCols
		for i := startLine + 1; i < len(v.lineOffsets); i++ {
			v.lineOffsets[i] += byteDelta
		}
		v.totalBytes += byteDelta
		if newLineCols >= v.maxCols {
			v.maxCols = newLineCols
		} else if oldLineCols == v.maxCols && newLineCols < oldLineCols {
			v.maxCols = 0
			for _, cols := range v.lineWidths {
				if cols > v.maxCols {
					v.maxCols = cols
				}
			}
		}
		if len(st.editLineRunes) == len(v.lines) {
			for i := startLine + 1; i < len(st.editLineRunes); i++ {
				st.editLineRunes[i] += runeDelta
			}
		} else {
			st.editLineRunes = viewerEditLineRuneOffsetsFromView(v)
		}
		if hadWrapCache {
			changedRows := v.wrapRowsForLine(startLine, newLine, v.wrapCols)
			oldRowCount := oldWrapEnd - oldWrapStart
			if len(changedRows) == oldRowCount {
				copy(v.wrapRows[oldWrapStart:oldWrapEnd], changedRows)
			} else {
				newWrapRows := make([]streamWrapRow, 0, len(oldWrapRows)-oldRowCount+len(changedRows))
				newWrapRows = append(newWrapRows, oldWrapRows[:oldWrapStart]...)
				newWrapRows = append(newWrapRows, changedRows...)
				newWrapRows = append(newWrapRows, oldWrapRows[oldWrapEnd:]...)
				v.wrapRows = newWrapRows
			}
			rowDelta := len(changedRows) - oldRowCount
			switch {
			case oldTop >= oldWrapEnd:
				v.topLine = oldTop + rowDelta
			case oldTop >= oldWrapStart:
				v.topLine = oldWrapStart
			}
		} else {
			v.wrapRows = nil
		}
	} else {
		newCount := len(oldLines) - removedLineCount + len(replacementLines)
		newLines := make([]string, 0, newCount)
		newLines = append(newLines, oldLines[:startLine]...)
		newLines = append(newLines, replacementLines...)
		newLines = append(newLines, oldLines[endLine+1:]...)
		v.lines = newLines

		// Preserve unchanged rune counts and scan only replacement text. Suffix
		// offsets still require cheap integer work, never decoding file contents.
		v.lineOffsets = make([]int, newCount)
		v.lineRunes = make([]int, newCount)
		v.lineWidths = make([]int, newCount)
		copy(v.lineOffsets[:startLine], oldOffsets[:startLine])
		copy(v.lineRunes[:startLine], oldRunes[:startLine])
		copy(v.lineWidths[:startLine], oldWidths[:startLine])
		for i, line := range replacementLines {
			v.lineRunes[startLine+i] = utf8.RuneCountInString(line)
			v.lineWidths[startLine+i] = v.lineWidth(line)
		}
		newSuffixStart := startLine + len(replacementLines)
		copy(v.lineRunes[newSuffixStart:], oldRunes[endLine+1:])
		copy(v.lineWidths[newSuffixStart:], oldWidths[endLine+1:])
		offset := 0
		if startLine > 0 {
			offset = oldOffsets[startLine]
		}
		for i := startLine; i < newCount; i++ {
			v.lineOffsets[i] = offset
			offset += len(v.lines[i])
			if i+1 < newCount {
				offset++
			}
		}
		v.totalBytes = offset
		v.maxCols = 0
		for _, cols := range v.lineWidths {
			if cols > v.maxCols {
				v.maxCols = cols
			}
		}

		if hadWrapCache {
			newWrapRows := make([]streamWrapRow, 0, len(oldWrapRows)-(oldWrapEnd-oldWrapStart)+len(replacementLines))
			newWrapRows = append(newWrapRows, oldWrapRows[:oldWrapStart]...)
			for i, line := range replacementLines {
				newWrapRows = append(newWrapRows, v.wrapRowsForLine(startLine+i, line, v.wrapCols)...)
			}
			newWrapEnd := len(newWrapRows)
			for _, row := range oldWrapRows[oldWrapEnd:] {
				row.line += lineDelta
				newWrapRows = append(newWrapRows, row)
			}
			v.wrapRows = newWrapRows
			rowDelta := (newWrapEnd - oldWrapStart) - (oldWrapEnd - oldWrapStart)
			switch {
			case oldTop >= oldWrapEnd:
				v.topLine = oldTop + rowDelta
			case oldTop >= oldWrapStart:
				v.topLine = oldWrapStart
			}
		} else {
			v.wrapRows = nil
			if !v.wrapEnabled {
				switch {
				case oldTop > endLine:
					v.topLine = oldTop + lineDelta
				case oldTop >= startLine:
					v.topLine = startLine
				}
			}
		}
	}
	v.clampTop()
	v.syncVisualTop()
	caret := start + len(replacement)
	v.selActive = true
	v.selAnchor = caret
	v.selHead = caret
	v.updateSelectionRange()

	if !fastSingleLine {
		st.editLineRunes = viewerEditLineRuneOffsetsFromView(v)
	}
	st.editDesiredColSet = false
	st.editSyntax = viewerPreserveSyntaxAfterEdit(oldSyntax, oldChangedLines, startLine, endLine, startLocal, endLocal, replacement, v.lines)
	v.setSyntax(st.editSyntax)
	st.editSyntaxSeq++
	st.editSyntaxDue = time.Now().Add(fileViewerEditSyntaxDelay)
}

func (ui *UI) replaceFileViewerVirtualText(st *fileViewerState, start, end int, replacement string, now time.Time) bool {
	if st == nil || !st.editVirtualReady {
		return false
	}
	v := &st.stream
	start = v.clampOffset(start)
	end = v.clampOffset(end)
	if end < start {
		start, end = end, start
	}
	replacement = normalizeViewerLineEndings(replacement)
	before := virtualEditRangeText(v, start, end)
	if before == replacement {
		return false
	}
	record := fileViewerTextUndoRecord{
		start:           start,
		before:          before,
		after:           replacement,
		selectionBefore: v.selectionState(),
		revisionBefore:  st.editRevision,
	}
	st.applyVirtualEditReplacement(start, end, replacement)
	st.editNextRevision++
	if st.editNextRevision <= 0 {
		st.editNextRevision = 1
	}
	st.editRevision = st.editNextRevision
	record.revisionAfter = st.editRevision
	record.selectionAfter = v.selectionState()
	if st.editUndoIndex < len(st.editUndo) {
		st.editUndo = st.editUndo[:st.editUndoIndex]
	}
	st.editUndo = append(st.editUndo, record)
	if len(st.editUndo) > fileViewerTextUndoLimit {
		copy(st.editUndo, st.editUndo[len(st.editUndo)-fileViewerTextUndoLimit:])
		st.editUndo = st.editUndo[:fileViewerTextUndoLimit]
	}
	st.editUndoIndex = len(st.editUndo)
	st.editDirty = st.editRevision != st.editSavedRevision
	if st.editDirty && !st.saving {
		st.status = "modified"
	}
	st.editCaretBlinkAt = now
	return true
}

func (ui *UI) undoFileViewerVirtualText(st *fileViewerState, now time.Time) bool {
	if st == nil || st.editUndoIndex <= 0 || st.editUndoIndex > len(st.editUndo) {
		return false
	}
	record := st.editUndo[st.editUndoIndex-1]
	st.applyVirtualEditReplacement(record.start, record.start+len(record.after), record.before)
	st.stream.restoreSelectionState(record.selectionBefore)
	st.editUndoIndex--
	st.editRevision = record.revisionBefore
	st.editDirty = st.editRevision != st.editSavedRevision
	st.status = "editing"
	if st.editDirty {
		st.status = "modified"
	}
	st.editCaretBlinkAt = now
	return true
}

func (ui *UI) redoFileViewerVirtualText(st *fileViewerState, now time.Time) bool {
	if st == nil || st.editUndoIndex < 0 || st.editUndoIndex >= len(st.editUndo) {
		return false
	}
	record := st.editUndo[st.editUndoIndex]
	st.applyVirtualEditReplacement(record.start, record.start+len(record.before), record.after)
	st.stream.restoreSelectionState(record.selectionAfter)
	st.editUndoIndex++
	st.editRevision = record.revisionAfter
	st.editDirty = st.editRevision != st.editSavedRevision
	st.status = "editing"
	if st.editDirty {
		st.status = "modified"
	}
	st.editCaretBlinkAt = now
	return true
}

func virtualEditSelection(v *streamOutputView) (start, end int) {
	if v == nil {
		return 0, 0
	}
	start, end = v.selAnchor, v.selHead
	if end < start {
		start, end = end, start
	}
	return v.clampOffset(start), v.clampOffset(end)
}

func virtualEditSetCaret(v *streamOutputView, offset int, extend bool) {
	if v == nil {
		return
	}
	offset = v.clampOffset(offset)
	if extend {
		if !v.selActive {
			v.selActive = true
			v.selAnchor = v.selHead
		}
		v.selHead = offset
	} else {
		v.selActive = true
		v.selAnchor = offset
		v.selHead = offset
	}
	v.updateSelectionRange()
}

func virtualEditMoveRune(v *streamOutputView, offset, direction int) int {
	if v == nil || direction == 0 {
		return offset
	}
	offset = v.clampOffset(offset)
	line, local, ok := v.lineForOffset(offset)
	if !ok {
		return 0
	}
	if direction < 0 {
		if local == 0 {
			if line == 0 {
				return 0
			}
			return offset - 1
		}
		_, size := utf8.DecodeLastRuneInString(v.lines[line][:local])
		if size < 1 {
			size = 1
		}
		return offset - size
	}
	if local < len(v.lines[line]) {
		_, size := utf8.DecodeRuneInString(v.lines[line][local:])
		if size < 1 {
			size = 1
		}
		return offset + size
	}
	if line+1 < len(v.lines) {
		return offset + 1
	}
	return offset
}

func virtualEditRuneNear(v *streamOutputView, offset, direction int) (rune, bool) {
	next := virtualEditMoveRune(v, offset, direction)
	if next == offset {
		return 0, false
	}
	if direction < 0 {
		offset = next
	}
	line, local, ok := v.lineForOffset(offset)
	if !ok {
		return 0, false
	}
	if local == len(v.lines[line]) {
		return '\n', true
	}
	r, _ := utf8.DecodeRuneInString(v.lines[line][local:])
	return r, true
}

func virtualEditWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func virtualEditMoveWord(v *streamOutputView, offset, direction int) int {
	if direction < 0 {
		for {
			r, ok := virtualEditRuneNear(v, offset, -1)
			if !ok || virtualEditWordRune(r) {
				break
			}
			offset = virtualEditMoveRune(v, offset, -1)
		}
		for {
			r, ok := virtualEditRuneNear(v, offset, -1)
			if !ok || !virtualEditWordRune(r) {
				break
			}
			offset = virtualEditMoveRune(v, offset, -1)
		}
		return offset
	}
	for {
		r, ok := virtualEditRuneNear(v, offset, 1)
		if !ok || virtualEditWordRune(r) {
			break
		}
		offset = virtualEditMoveRune(v, offset, 1)
	}
	for {
		r, ok := virtualEditRuneNear(v, offset, 1)
		if !ok || !virtualEditWordRune(r) {
			break
		}
		offset = virtualEditMoveRune(v, offset, 1)
	}
	return offset
}

func (st *fileViewerState) moveVirtualEditCaretVertical(rows int, extend bool) {
	if st == nil || rows == 0 {
		return
	}
	v := &st.stream
	line, local, ok := v.lineForOffset(v.selHead)
	if !ok {
		return
	}
	col := v.colAtByte(v.lines[line], local)
	if v.wrapEnabled && len(v.wrapRows) > 0 {
		rowIndex := v.rowForLineCol(line, col)
		row := v.rowAt(rowIndex)
		if !st.editDesiredColSet {
			st.editDesiredCol = col - row.from
			st.editDesiredColSet = true
		}
		targetIndex := rowIndex + rows
		if targetIndex < 0 {
			targetIndex = 0
		}
		if targetIndex >= len(v.wrapRows) {
			targetIndex = len(v.wrapRows) - 1
		}
		target := v.rowAt(targetIndex)
		targetCol := target.from + st.editDesiredCol
		if targetCol > target.to {
			targetCol = target.to
		}
		offset := v.lineByteStart(target.line) + v.byteAtCol(v.lines[target.line], targetCol)
		virtualEditSetCaret(v, offset, extend)
		return
	}
	if !st.editDesiredColSet {
		st.editDesiredCol = col
		st.editDesiredColSet = true
	}
	targetLine := line + rows
	if targetLine < 0 {
		targetLine = 0
	}
	if targetLine >= len(v.lines) {
		targetLine = len(v.lines) - 1
	}
	targetCol := st.editDesiredCol
	if maxCol := v.lineCols(targetLine); targetCol > maxCol {
		targetCol = maxCol
	}
	offset := v.lineByteStart(targetLine) + v.byteAtCol(v.lines[targetLine], targetCol)
	virtualEditSetCaret(v, offset, extend)
}

func (st *fileViewerState) revealVirtualEditCaret() {
	if st == nil {
		return
	}
	v := &st.stream
	line, local, ok := v.lineForOffset(v.selHead)
	if !ok {
		return
	}
	col := v.colAtByte(v.lines[line], local)
	row := line
	if v.wrapEnabled && len(v.wrapRows) > 0 {
		row = v.rowForLineCol(line, col)
	}
	if row < v.topLine {
		v.topLine = row
	} else if visible := max(1, v.visibleLines); row >= v.topLine+visible {
		v.topLine = row - visible + 1
	}
	v.clampTop()
	v.syncVisualTop()
	if !v.wrapEnabled {
		visibleCols := v.visibleCols(v.textRect.Dx())
		if col < v.hCol {
			v.hCol = col
		} else if visibleCols > 0 && col >= v.hCol+visibleCols {
			v.hCol = col - visibleCols + 1
		}
		v.clampHCol(v.textRect.Dx())
	}
}

func viewerVirtualEditColumn(text string, tabSize int) int {
	if tabSize < 1 {
		tabSize = 4
	}
	col := 0
	for _, r := range text {
		if r == '\t' {
			col += tabSize - col%tabSize
		} else {
			col++
		}
	}
	return col
}

func (ui *UI) indentFileViewerVirtualSelection(st *fileViewerState, start, end int, outdent bool, now time.Time) bool {
	if st == nil || start == end {
		return false
	}
	v := &st.stream
	startLine, _, ok := v.lineForOffset(start)
	if !ok {
		return false
	}
	endLine, endLocal, ok := v.lineForOffset(end)
	if !ok {
		return false
	}
	if endLocal == 0 && endLine > startLine {
		endLine--
	}
	blockStart := v.lineByteStart(startLine)
	blockEnd := v.lineByteEnd(endLine)
	unit := "\t"
	if st.editIndentStyle != fm.ViewerEditorIndentTabs {
		unit = strings.Repeat(" ", max(1, st.editTabSize))
	}
	lines := append([]string(nil), v.lines[startLine:endLine+1]...)
	changed := false
	for i, line := range lines {
		if !outdent {
			lines[i] = unit + line
			changed = true
			continue
		}
		switch {
		case strings.HasPrefix(line, "\t"):
			lines[i] = line[1:]
			changed = true
		default:
			remove := 0
			for remove < len(line) && remove < max(1, st.editTabSize) && line[remove] == ' ' {
				remove++
			}
			if remove > 0 {
				lines[i] = line[remove:]
				changed = true
			}
		}
	}
	if !changed {
		return false
	}
	replacement := strings.Join(lines, "\n")
	anchorBefore, headBefore := v.selAnchor, v.selHead
	if !ui.replaceFileViewerVirtualText(st, blockStart, blockEnd, replacement, now) {
		return false
	}
	selectionStart := blockStart
	selectionEnd := blockStart + len(replacement)
	if anchorBefore > headBefore {
		v.selAnchor, v.selHead = selectionEnd, selectionStart
	} else {
		v.selAnchor, v.selHead = selectionStart, selectionEnd
	}
	v.selActive = true
	v.updateSelectionRange()
	if st.editUndoIndex > 0 && st.editUndoIndex <= len(st.editUndo) {
		st.editUndo[st.editUndoIndex-1].selectionAfter = v.selectionState()
	}
	return true
}

func (ui *UI) insertFileViewerVirtualTab(st *fileViewerState, start, end int, outdent bool, now time.Time) bool {
	if st == nil {
		return false
	}
	if start != end {
		return ui.indentFileViewerVirtualSelection(st, start, end, outdent, now)
	}
	v := &st.stream
	line, local, ok := v.lineForOffset(start)
	if !ok {
		return false
	}
	if outdent {
		lineText := v.lines[line]
		if local > 0 && lineText[local-1] == '\t' {
			return ui.replaceFileViewerVirtualText(st, start-1, start, "", now)
		}
		spaces := 0
		for pos := local - 1; pos >= 0 && lineText[pos] == ' ' && spaces < max(1, st.editTabSize); pos-- {
			spaces++
		}
		if spaces == 0 {
			return false
		}
		col := viewerVirtualEditColumn(lineText[:local], st.editTabSize)
		remove := col % max(1, st.editTabSize)
		if remove == 0 {
			remove = max(1, st.editTabSize)
		}
		remove = min(remove, spaces)
		return ui.replaceFileViewerVirtualText(st, start-remove, start, "", now)
	}
	if st.editIndentStyle == fm.ViewerEditorIndentTabs {
		return ui.replaceFileViewerVirtualText(st, start, end, "\t", now)
	}
	col := viewerVirtualEditColumn(v.lines[line][:local], st.editTabSize)
	spaces := max(1, st.editTabSize) - col%max(1, st.editTabSize)
	return ui.replaceFileViewerVirtualText(st, start, end, strings.Repeat(" ", spaces), now)
}

func (ui *UI) handleFileViewerVirtualEditKey(st *fileViewerState, gtx layout.Context, ke key.Event) bool {
	if st == nil || ke.State != key.Press {
		return false
	}
	v := &st.stream
	extend := ke.Modifiers.Contain(key.ModShift)
	shortcut := ke.Modifiers.Contain(key.ModShortcut)
	wordMove := ke.Modifiers.Contain(key.ModShortcutAlt)
	start, end := virtualEditSelection(v)

	if shortcut {
		switch ke.Name {
		case "A":
			v.selActive = true
			v.selAnchor = 0
			v.selHead = v.totalBytes
			v.updateSelectionRange()
			return true
		case "C", "X":
			if end > start {
				text := virtualEditRangeText(v, start, end)
				gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
				if ke.Name == "X" {
					ui.replaceFileViewerVirtualText(st, start, end, "", gtx.Now)
				}
			}
			return true
		case "V":
			gtx.Execute(clipboard.ReadCmd{Tag: &st.editClipboardTag})
			return true
		case "Z":
			if extend {
				return ui.redoFileViewerVirtualText(st, gtx.Now)
			}
			return ui.undoFileViewerVirtualText(st, gtx.Now)
		case key.NameHome:
			virtualEditSetCaret(v, 0, extend)
			return true
		case key.NameEnd:
			virtualEditSetCaret(v, v.totalBytes, extend)
			return true
		case key.NameLeftArrow:
			line, _, _ := v.lineForOffset(v.selHead)
			virtualEditSetCaret(v, v.lineByteStart(line), extend)
			return true
		case key.NameRightArrow:
			line, _, _ := v.lineForOffset(v.selHead)
			virtualEditSetCaret(v, v.lineByteEnd(line), extend)
			return true
		}
	}

	switch ke.Name {
	case key.NameLeftArrow, key.NameRightArrow:
		direction := -1
		if ke.Name == key.NameRightArrow {
			direction = 1
		}
		offset := v.selHead
		if !extend && end > start {
			offset = start
			if direction > 0 {
				offset = end
			}
		} else if wordMove {
			offset = virtualEditMoveWord(v, offset, direction)
		} else {
			offset = virtualEditMoveRune(v, offset, direction)
		}
		virtualEditSetCaret(v, offset, extend)
		st.editDesiredColSet = false
	case key.NameUpArrow:
		st.moveVirtualEditCaretVertical(-1, extend)
	case key.NameDownArrow:
		st.moveVirtualEditCaretVertical(1, extend)
	case key.NamePageUp:
		st.moveVirtualEditCaretVertical(-max(1, v.visibleLines), extend)
	case key.NamePageDown:
		st.moveVirtualEditCaretVertical(max(1, v.visibleLines), extend)
	case key.NameHome, key.NameEnd:
		line, local, _ := v.lineForOffset(v.selHead)
		col := v.colAtByte(v.lines[line], local)
		target := v.lineByteStart(line)
		if v.wrapEnabled && len(v.wrapRows) > 0 {
			row := v.rowAt(v.rowForLineCol(line, col))
			target = v.lineByteStart(line) + v.byteAtCol(v.lines[line], row.from)
			if ke.Name == key.NameEnd {
				target = v.lineByteStart(line) + v.byteAtCol(v.lines[line], row.to)
			}
		} else if ke.Name == key.NameEnd {
			target = v.lineByteEnd(line)
		}
		virtualEditSetCaret(v, target, extend)
		st.editDesiredColSet = false
	case key.NameDeleteBackward, key.NameDeleteForward:
		if start == end {
			if wordMove {
				if ke.Name == key.NameDeleteBackward {
					start = virtualEditMoveWord(v, start, -1)
				} else {
					end = virtualEditMoveWord(v, end, 1)
				}
			} else if ke.Name == key.NameDeleteBackward {
				start = virtualEditMoveRune(v, start, -1)
			} else {
				end = virtualEditMoveRune(v, end, 1)
			}
		}
		return ui.replaceFileViewerVirtualText(st, start, end, "", gtx.Now)
	case key.NameEnter, key.NameReturn:
		return ui.replaceFileViewerVirtualText(st, start, end, "\n", gtx.Now)
	case key.NameTab:
		return ui.insertFileViewerVirtualTab(st, start, end, extend, gtx.Now)
	default:
		return false
	}
	st.revealVirtualEditCaret()
	st.editCaretBlinkAt = gtx.Now
	return true
}

func (ui *UI) handleFileViewerVirtualEditEvents(gtx layout.Context, st *fileViewerState) {
	if st == nil || !st.editVirtualReady {
		return
	}
	anyMods := ^key.Modifiers(0)
	filters := []event.Filter{
		key.FocusFilter{Target: &st.editKeyTag},
		transfer.TargetFilter{Target: &st.editClipboardTag, Type: "application/text"},
	}
	for _, name := range []key.Name{
		key.NameLeftArrow, key.NameRightArrow, key.NameUpArrow, key.NameDownArrow,
		key.NamePageUp, key.NamePageDown, key.NameHome, key.NameEnd,
		key.NameDeleteBackward, key.NameDeleteForward, key.NameEnter, key.NameReturn, key.NameTab,
		"A", "C", "V", "X", "Z",
	} {
		filters = append(filters, key.Filter{Focus: &st.editKeyTag, Name: name, Optional: anyMods})
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		switch ev := ev.(type) {
		case key.EditEvent:
			start := st.virtualEditByteAtRune(ev.Range.Start)
			end := st.virtualEditByteAtRune(ev.Range.End)
			if ui.replaceFileViewerVirtualText(st, start, end, ev.Text, gtx.Now) {
				st.revealVirtualEditCaret()
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.SelectionEvent:
			v := &st.stream
			v.selActive = true
			v.selAnchor = st.virtualEditByteAtRune(ev.Start)
			v.selHead = st.virtualEditByteAtRune(ev.End)
			v.updateSelectionRange()
			st.revealVirtualEditCaret()
		case key.Event:
			if ui.handleFileViewerVirtualEditKey(st, gtx, ev) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case transfer.DataEvent:
			data := ev.Open()
			if data == nil {
				continue
			}
			content, err := io.ReadAll(data)
			_ = data.Close()
			if err != nil {
				continue
			}
			start, end := virtualEditSelection(&st.stream)
			if ui.replaceFileViewerVirtualText(st, start, end, string(content), gtx.Now) {
				st.revealVirtualEditCaret()
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
}

func (ui *UI) drawFileViewerVirtualCaret(gtx layout.Context, st *fileViewerState) {
	if st == nil || !gtx.Focused(&st.editKeyTag) {
		return
	}
	v := &st.stream
	if v.lineH <= 0 || v.textRect.Empty() {
		return
	}
	if st.editCaretBlinkAt.IsZero() {
		st.editCaretBlinkAt = gtx.Now
	}
	const blinkHalf = 500 * time.Millisecond
	elapsed := gtx.Now.Sub(st.editCaretBlinkAt)
	if elapsed < 0 {
		elapsed = 0
	}
	next := gtx.Now.Add(blinkHalf - elapsed%blinkHalf)
	gtx.Execute(op.InvalidateCmd{At: next})
	if (elapsed/blinkHalf)%2 != 0 {
		return
	}
	line, local, ok := v.lineForOffset(v.selHead)
	if !ok {
		return
	}
	col := v.colAtByte(v.lines[line], local)
	rowIndex := line
	row := streamWrapRow{line: line, from: 0, to: v.lineCols(line)}
	if v.wrapEnabled && len(v.wrapRows) > 0 {
		rowIndex = v.rowForLineCol(line, col)
		row = v.rowAt(rowIndex)
	}
	if rowIndex < v.displayTop || rowIndex >= v.displayTop+v.renderedLineCount() {
		return
	}
	visibleCol := col - row.from
	if !v.wrapEnabled {
		visibleCol = col - v.hCol
	}
	x := v.textRect.Min.X + v.textPad + v.colOffsetPx(visibleCol)
	y := v.textRect.Min.Y + v.displayY + (rowIndex-v.displayTop)*v.lineH
	caretW := max(1, gtx.Dp(1))
	caret := image.Rect(x, y, x+caretW, y+v.lineH).Intersect(v.textRect)
	if caret.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, ui.fileViewerTheme().Text, clip.Rect(caret).Op())
}

func (ui *UI) updateFileViewerVirtualIME(gtx layout.Context, st *fileViewerState) {
	if st == nil || !st.editVirtualReady {
		return
	}
	v := &st.stream
	anchorRune := st.virtualEditRuneAtByte(v.selAnchor)
	headRune := st.virtualEditRuneAtByte(v.selHead)
	line, local, ok := v.lineForOffset(v.selHead)
	if !ok {
		return
	}
	col := v.colAtByte(v.lines[line], local)
	rowIndex := line
	rowFrom := 0
	if v.wrapEnabled && len(v.wrapRows) > 0 {
		rowIndex = v.rowForLineCol(line, col)
		rowFrom = v.rowAt(rowIndex).from
	}
	visibleCol := col - rowFrom
	if !v.wrapEnabled {
		visibleCol = col - v.hCol
	}
	caretPos := image.Pt(
		v.textRect.Min.X+v.textPad+v.colOffsetPx(visibleCol),
		v.textRect.Min.Y+v.displayY+(rowIndex-v.displayTop+1)*v.lineH-v.lineH/4,
	)
	gtx.Execute(key.SelectionCmd{
		Tag:   &st.editKeyTag,
		Range: key.Range{Start: anchorRune, End: headRune},
		Caret: key.Caret{
			Pos:     layout.FPt(caretPos),
			Ascent:  float32(v.lineH * 3 / 4),
			Descent: float32(v.lineH / 4),
		},
	})
	lineStartRune := st.editLineRunes[line]
	gtx.Execute(key.SnippetCmd{
		Tag: &st.editKeyTag,
		Snippet: key.Snippet{
			Range: key.Range{Start: lineStartRune, End: lineStartRune + utf8.RuneCountInString(v.lines[line])},
			Text:  v.lines[line],
		},
	})
}

func (ui *UI) layoutFileViewerVirtualTextEditor(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	if !st.editVirtualReady {
		st.initializeVirtualEditText(st.contentEditor.Text())
	}
	ui.handleFileViewerVirtualEditEvents(gtx, st)
	ui.pumpFileViewerEditSyntax(st)
	ui.startFileViewerEditSyntaxIfDue(st, gtx)
	beforeHead := st.stream.selHead
	dims := ui.layoutStreamOutputView(th, gtx, st)
	if st.stream.selHead != beforeHead {
		st.editDesiredColSet = false
		st.editCaretBlinkAt = gtx.Now
	}
	if st.editFocus {
		st.editFocus = false
		gtx.Execute(key.FocusCmd{Tag: &st.editKeyTag})
	}
	ui.drawFileViewerVirtualCaret(gtx, st)
	ui.updateFileViewerVirtualIME(gtx, st)
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &st.editKeyTag)
	event.Op(gtx.Ops, &st.editClipboardTag)
	pass.Pop()
	key.InputHintOp{Tag: &st.editKeyTag, Hint: key.HintText}.Add(gtx.Ops)
	return dims
}

func (ui *UI) copyFileViewerVirtualEditText(gtx layout.Context, st *fileViewerState) bool {
	if st == nil || !st.editVirtualReady {
		return false
	}
	start, end := virtualEditSelection(&st.stream)
	text := virtualEditRangeText(&st.stream, start, end)
	if text == "" {
		text = st.virtualEditText()
	}
	if text == "" {
		return false
	}
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
	return true
}

func (ui *UI) pasteFileViewerVirtualEditText(gtx layout.Context, st *fileViewerState) bool {
	if st == nil || !st.editVirtualReady {
		return false
	}
	if text, err := readEditorContextClipboardText(); err == nil {
		start, end := virtualEditSelection(&st.stream)
		if ui.replaceFileViewerVirtualText(st, start, end, text, gtx.Now) {
			st.revealVirtualEditCaret()
			gtx.Execute(key.FocusCmd{Tag: &st.editKeyTag})
			gtx.Execute(op.InvalidateCmd{})
		}
		return true
	}
	gtx.Execute(key.FocusCmd{Tag: &st.editKeyTag})
	gtx.Execute(clipboard.ReadCmd{Tag: &st.editClipboardTag})
	return true
}
