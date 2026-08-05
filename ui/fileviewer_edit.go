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
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/text/encoding/charmap"
)

const (
	fileViewerEditSyntaxDelay       = 160 * time.Millisecond
	fileViewerEditResizeSettleDelay = 140 * time.Millisecond
)

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
		st.editScrollRatio = 0
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
				maxTop := totalRows - max(1, st.stream.visibleLines)
				if maxTop > 0 {
					st.editScrollRatio = clamp01(float32(topRow) / float32(maxTop))
				}
			}
		}
		st.contentEditor.ReadOnly = false
		if !st.editDirty {
			st.contentEditor.SetText(st.editBaselineText)
		}
		syntax := st.stream.syntax
		current := st.contentEditor.Text()
		st.initializeVirtualEditText(current)
		st.editSyntax = syntax
		st.stream.setSyntax(st.editSyntax)
		st.editSyntaxDue = time.Time{}
		st.editRenderText = current
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
		st.editCaretStart = -1
		st.editScrollPending = false
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
		st.stream.SetContent(st.content)
		st.stream.setSyntax(st.editSyntax)
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

func (ui *UI) toggleFileViewerEdit(now time.Time) bool {
	st := ui.fileViewer
	if st == nil {
		return false
	}
	if !st.editMode {
		return ui.startFileViewerEdit(now)
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
		st.editSyntax = viewerBuildSyntaxDocument(context.Background(), st.path, st.editBaselineText)
		st.stream.setSyntax(st.editSyntax)
		st.editRenderText = st.editBaselineText
		st.editLineRunes = viewerEditLineRuneOffsetsFromLines(st.stream.lines)
		st.editSyntaxSeq++
		st.editSyntaxDue = time.Time{}
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

func (ui *UI) layoutFileViewerTextEditorBody(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	contentChanged := false
	for {
		ev, ok := st.contentEditor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			contentChanged = true
		}
	}
	if contentChanged {
		current := st.contentEditor.Text()
		ui.syncFileViewerTextEditContent(st, current)
		ui.updateFileViewerEditRenderContent(st, current, gtx.Now)
	}
	ui.pumpFileViewerEditSyntax(st)
	ui.startFileViewerEditSyntaxIfDue(st, gtx)
	if st.editFocus {
		st.editFocus = false
		gtx.Execute(key.FocusCmd{Tag: &st.contentEditor})
	}
	st.contentEditor.ReadOnly = false
	if st.wrapEnabled {
		st.contentEditor.WrapPolicy = text.WrapHeuristically
	} else {
		st.contentEditor.WrapPolicy = text.WrapWords
	}
	if !st.editWrapInitialized || st.editWrapValue != st.wrapEnabled {
		st.editWrapInitialized = true
		st.editWrapValue = st.wrapEnabled
		st.editHOffset = 0
		st.editCaretStart = -1
		st.editLayoutViewport = image.Point{}
		st.editLayoutSize = image.Point{}
		st.editLayoutDue = time.Time{}
	}
	theme := ui.fileViewerTheme()
	ed := material.Editor(th, &st.contentEditor, "")
	ed.Font.Typeface = ui.viewerTypeface()
	ed.Font.Weight = font.Normal
	ed.TextSize = ui.viewerTextSize()
	ed.Color = theme.Text
	ed.HintColor = theme.Hint
	ed.SelectionColor = theme.Selection
	return layout.Inset{Left: unit.Dp(2), Right: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		st.stream.ensureTextMetrics(ui, th, gtx, true)
		layoutWidth := size.X
		hbarHeight := 0
		if !st.wrapEnabled {
			contentWidth := int(math.Ceil(float64(max(st.editMaxCols+2, 1)) * float64(st.stream.charAdvance)))
			if contentWidth > layoutWidth {
				layoutWidth = contentWidth
				hbarHeight = viewerScrollbarThickness(gtx, size.Y)
				if hbarHeight > size.Y/2 {
					hbarHeight = size.Y / 2
				}
			}
		}
		textHeight := size.Y - hbarHeight
		if textHeight < 1 {
			textHeight = 1
			hbarHeight = 0
		}
		editorSize := st.fileViewerEditLayoutSize(
			gtx,
			image.Pt(size.X, textHeight),
			image.Pt(layoutWidth, textHeight),
		)
		editorGTX := gtx
		editorGTX.Constraints = layout.Exact(editorSize)
		editorClip := clip.Rect(image.Rect(0, 0, size.X, textHeight)).Push(gtx.Ops)
		editorOffset := op.Offset(image.Pt(-st.editHOffset, 0)).Push(gtx.Ops)
		ed.Layout(editorGTX)
		editorOffset.Pop()
		editorClip.Pop()

		metrics, scrollable := editorVerticalScrollMetrics(&st.contentEditor)
		if st.editScrollPending {
			st.editScrollPending = false
			if scrollable {
				editorScrollToVerticalOffset(&st.contentEditor, int(st.editScrollRatio*float32(metrics.MaxOffset)))
				metrics, scrollable = editorVerticalScrollMetrics(&st.contentEditor)
				gtx.Execute(op.InvalidateCmd{})
			}
		}
		textWidth := size.X
		barWidth := 0
		if scrollable {
			style := material.Scrollbar(th, &st.editScrollbar)
			style.Track.Color = theme.ScrollTrack
			style.Indicator.Color = theme.ScrollThumb
			style.Indicator.HoverColor = theme.ScrollThumbHover
			style.Indicator.MajorMinLen = unit.Dp(fileViewerScrollbarMinThumbPx)
			barWidth = gtx.Dp(style.Width())
			if barWidth > 0 && barWidth < size.X {
				textWidth -= barWidth
				start := clamp01(float32(metrics.Offset) / float32(metrics.Content))
				end := clamp01(float32(metrics.Offset+metrics.Viewport) / float32(metrics.Content))
				barGTX := gtx
				barGTX.Constraints = layout.Exact(image.Pt(barWidth, textHeight))
				offset := op.Offset(image.Pt(size.X-barWidth, 0)).Push(gtx.Ops)
				style.Layout(barGTX, layout.Vertical, start, end)
				offset.Pop()
				if delta := st.editScrollbar.ScrollDistance(); delta != 0 {
					editorScrollToVerticalOffset(&st.contentEditor, metrics.Offset+int(delta*float32(metrics.Content)))
					gtx.Execute(op.InvalidateCmd{})
				}
			}
		}
		horizontalViewport := textWidth
		if horizontalViewport < 1 {
			horizontalViewport = 1
		}
		if st.wrapEnabled || layoutWidth <= horizontalViewport {
			st.editHOffset = 0
			hbarHeight = 0
		} else {
			maxOffset := layoutWidth - horizontalViewport
			caretStart, _ := st.contentEditor.Selection()
			if caretStart != st.editCaretStart {
				st.editCaretStart = caretStart
				caretX := int(st.contentEditor.CaretCoords().X)
				margin := max(1, st.stream.charW)
				switch {
				case caretX < st.editHOffset:
					st.editHOffset = caretX
				case caretX+margin > st.editHOffset+horizontalViewport:
					st.editHOffset = caretX + margin - horizontalViewport
				}
			}
			if st.editHOffset < 0 {
				st.editHOffset = 0
			}
			if st.editHOffset > maxOffset {
				st.editHOffset = maxOffset
			}
		}

		syntaxClip := clip.Rect(image.Rect(0, 0, horizontalViewport, textHeight)).Push(gtx.Ops)
		syntaxOffset := op.Offset(image.Pt(-st.editHOffset, 0)).Push(gtx.Ops)
		syntaxGTX := gtx
		syntaxGTX.Constraints = layout.Exact(editorSize)
		ui.drawFileViewerEditSyntax(th, syntaxGTX, st, metrics, editorSize.X)
		syntaxOffset.Pop()
		syntaxClip.Pop()

		if hbarHeight > 0 {
			style := material.Scrollbar(th, &st.editHScrollbar)
			style.Track.Color = theme.ScrollTrack
			style.Indicator.Color = theme.ScrollThumb
			style.Indicator.HoverColor = theme.ScrollThumbHover
			style.Indicator.MajorMinLen = unit.Dp(fileViewerScrollbarMinThumbPx)
			trackWidth := horizontalViewport
			start := clamp01(float32(st.editHOffset) / float32(layoutWidth))
			end := clamp01(float32(st.editHOffset+horizontalViewport) / float32(layoutWidth))
			barGTX := gtx
			barGTX.Constraints = layout.Exact(image.Pt(trackWidth, hbarHeight))
			offset := op.Offset(image.Pt(0, textHeight)).Push(gtx.Ops)
			style.Layout(barGTX, layout.Horizontal, start, end)
			offset.Pop()
			if delta := st.editHScrollbar.ScrollDistance(); delta != 0 {
				st.editHOffset += int(delta * float32(layoutWidth))
				maxOffset := layoutWidth - horizontalViewport
				if st.editHOffset < 0 {
					st.editHOffset = 0
				}
				if st.editHOffset > maxOffset {
					st.editHOffset = maxOffset
				}
				gtx.Execute(op.InvalidateCmd{})
			}
		}
		return layout.Dimensions{Size: size}
	})
}

// fileViewerEditLayoutSize keeps Gio's editor viewport stable while native
// window-resize events are arriving. Gio v0.10 reshapes and re-indexes the
// entire document whenever either viewport dimension changes; for wrapped
// files that can allocate hundreds of megabytes for every resize frame. The
// outer clip still follows the window immediately, then one exact reflow is
// performed after the resize stream settles.
func (st *fileViewerState) fileViewerEditLayoutSize(gtx layout.Context, viewport, desired image.Point) image.Point {
	if viewport.X < 1 {
		viewport.X = 1
	}
	if viewport.Y < 1 {
		viewport.Y = 1
	}
	if desired.X < 1 {
		desired.X = 1
	}
	if desired.Y < 1 {
		desired.Y = 1
	}
	if st.editLayoutSize.X <= 0 || st.editLayoutSize.Y <= 0 {
		st.editLayoutViewport = viewport
		st.editLayoutSize = desired
		st.editLayoutDue = time.Time{}
		return desired
	}
	if viewport != st.editLayoutViewport {
		st.editLayoutViewport = viewport
		st.editLayoutDue = gtx.Now.Add(fileViewerEditResizeSettleDelay)
		gtx.Execute(op.InvalidateCmd{At: st.editLayoutDue})
		return st.editLayoutSize
	}
	if !st.editLayoutDue.IsZero() {
		if gtx.Now.Before(st.editLayoutDue) {
			gtx.Execute(op.InvalidateCmd{At: st.editLayoutDue})
			return st.editLayoutSize
		}
		st.editLayoutDue = time.Time{}
	}
	st.editLayoutSize = desired
	return desired
}

func (ui *UI) updateFileViewerEditRender(st *fileViewerState, now time.Time) {
	if st == nil || st.mode != "file" {
		return
	}
	ui.updateFileViewerEditRenderContent(st, st.contentEditor.Text(), now)
}

func (ui *UI) updateFileViewerEditRenderContent(st *fileViewerState, current string, now time.Time) {
	if st == nil || st.mode != "file" {
		return
	}
	if current == st.editRenderText {
		return
	}
	st.editRenderText = current
	if st.editSyntaxCh == nil {
		st.editSyntaxCh = make(chan fileViewerEditSyntaxResult, 1)
	}
	syntax := st.editSyntax
	st.initializeVirtualEditText(current)
	st.stream.setSyntax(syntax)
	st.editMaxCols = viewerEditMaxColumns(current)
	st.editSyntaxSeq++
	st.editSyntaxDue = now.Add(fileViewerEditSyntaxDelay)
}

func viewerEditMaxColumns(content string) int {
	maxCols := 0
	for _, line := range splitStreamLines(sanitizeViewerContent(content)) {
		if cols := utf8.RuneCountInString(line); cols > maxCols {
			maxCols = cols
		}
	}
	return maxCols
}

func viewerEditLineRuneOffsets(content string) []int {
	lines := splitStreamLines(content)
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

func (ui *UI) drawFileViewerEditSyntax(th *material.Theme, gtx layout.Context, st *fileViewerState, _ editorScrollMetrics, textWidth int) {
	if st == nil || !st.editSyntax.ready() || textWidth <= 0 || len(st.editLineRunes) == 0 {
		return
	}
	v := &st.stream
	firstLine := 0
	lastLine := len(st.editSyntax.lines) - 1
	if visibleStart, visibleEnd, ok := editorVisibleRuneRange(&st.contentEditor); ok {
		firstLine = viewerEditLineAtRune(st.editLineRunes, visibleStart) - 2
		lastLine = viewerEditLineAtRune(st.editLineRunes, visibleEnd) + 2
	}
	if firstLine < 0 {
		firstLine = 0
	}
	if lastLine >= len(st.editSyntax.lines) {
		lastLine = len(st.editSyntax.lines) - 1
	}
	if lastLine >= len(v.lines) {
		lastLine = len(v.lines) - 1
	}
	if lastLine < firstLine {
		return
	}

	defer clip.Rect(image.Rect(0, 0, textWidth, gtx.Constraints.Max.Y)).Push(gtx.Ops).Pop()
	theme := ui.fileViewerTheme()
	for lineIndex := firstLine; lineIndex <= lastLine; lineIndex++ {
		line := v.lines[lineIndex]
		globalStart := st.editLineRunes[lineIndex]
		for _, span := range st.editSyntax.lines[lineIndex].spans {
			if span.role == viewerSyntaxText || span.byteStart < 0 || span.byteEnd > len(line) || span.byteEnd <= span.byteStart {
				continue
			}
			start := globalStart + span.colStart
			end := globalStart + span.colEnd
			regions := st.contentEditor.Regions(start, end, nil)
			if len(regions) == 0 {
				continue
			}
			segment := line[span.byteStart:span.byteEnd]
			fg := viewerSyntaxColor(theme, span.role)
			if len(regions) == 1 {
				ui.drawFileViewerEditSyntaxSegment(th, gtx, segment, regions[0], fg, textWidth)
				continue
			}
			runes := []rune(segment)
			for i, r := range runes {
				runeRegions := st.contentEditor.Regions(start+i, start+i+1, nil)
				for _, region := range runeRegions {
					ui.drawFileViewerEditSyntaxSegment(th, gtx, string(r), region, fg, textWidth)
				}
			}
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

func (ui *UI) drawFileViewerEditSyntaxSegment(th *material.Theme, gtx layout.Context, segment string, region widget.Region, fg color.NRGBA, textWidth int) {
	if segment == "" || region.Bounds.Empty() || region.Bounds.Min.X >= textWidth {
		return
	}
	width := textWidth - region.Bounds.Min.X
	if width < 1 {
		return
	}
	label := material.Body2(th, segment)
	label.Font.Typeface = ui.viewerTypeface()
	label.Font.Weight = font.Normal
	label.TextSize = ui.viewerTextSize()
	label.Color = fg
	label.MaxLines = 1
	label.Truncator = ""
	labelGTX := gtx
	labelGTX.Constraints.Min = image.Point{}
	labelGTX.Constraints.Max.X = width
	if labelGTX.Constraints.Max.Y < region.Bounds.Dy()*2 {
		labelGTX.Constraints.Max.Y = region.Bounds.Dy() * 2
	}
	record := op.Record(gtx.Ops)
	dims := label.Layout(labelGTX)
	call := record.Stop()
	regionBaseline := region.Bounds.Max.Y - region.Baseline
	labelBaseline := dims.Size.Y - dims.Baseline
	offset := op.Offset(image.Pt(region.Bounds.Min.X, regionBaseline-labelBaseline)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
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
					st.content = sanitizeViewerContent(st.virtualEditText())
					st.stream.SetContent(st.content)
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
