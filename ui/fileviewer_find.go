// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hexone/filesys"
	"hexone/fm"
	"image"
	"image/color"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	uitheme "hexone/ui/theme"
)

const (
	viewerFindChunkBytes        = 256 << 10
	viewerFindBarInsetDp        = 6
	viewerFindBarRowHeightDp    = 22
	viewerFindStatusMaxDp       = 120
	viewerFindFieldChars        = 42
	viewerFindFieldMinChars     = 18
	viewerFindSearchingDelay    = 220 * time.Millisecond
	viewerRemoteSearchMaxBytes  = 8 << 10
	viewerFindMaxRows           = 7
	viewerFindRowHeightDp       = 24
	viewerHexFindResultLimit    = 200
	viewerHexPreviewBytesPerRow = 12
	viewerHexPreviewMaxRows     = 8
)

type viewerFindMatch struct {
	Start             int
	End               int
	Line              int
	Snippet           string
	SnippetHighlight  compactFindHighlight
	Preview           []string
	PreviewFocus      int
	PreviewHighlights []compactFindHighlight
}

type viewerHexFindMatch struct {
	Start        int64
	Length       int64
	PreviewBytes []byte
	PreviewMatch int
	TextPreview  string
	HexPreview   string
}

// viewerPDFFindMatch keeps character offsets in the extracted page text so
// selecting a result can both scroll to and precisely highlight the hit.
type viewerPDFFindMatch struct {
	Page              int
	Start             int
	End               int
	Snippet           string
	SnippetHighlight  compactFindHighlight
	Preview           []string
	PreviewFocus      int
	PreviewHighlights []compactFindHighlight
}

type viewerPDFFindResult struct {
	requestSeq int
	pageText   *viewerPDFPageText
	matches    []viewerPDFFindMatch
	searched   int
	done       bool
	err        string
}

type fileViewerFindState struct {
	editor widget.Editor

	open  bool
	focus bool

	prevClick         widget.Clickable
	nextClick         widget.Clickable
	closeClick        widget.Clickable
	findByClick       widget.Clickable
	previewClick      widget.Clickable
	sourceMenuClick   widget.Clickable
	sourceLocalClick  widget.Clickable
	sourceRemoteClick widget.Clickable

	remoteSearch      bool
	hexInput          bool
	hexPreview        bool
	modeKey           string
	sourceInit        bool
	sourceMenuOpen    bool
	sourceMenuAt      time.Time
	sourceButtonRect  image.Rectangle
	sourceMenuRect    image.Rectangle
	findByButtonRect  image.Rectangle
	previewButtonRect image.Rectangle
	status            string
	searchStartedAt   time.Time
	previewIndex      int
	previewAt         time.Time
	cursorAnim        compactFindCursorAnim

	matches    []viewerFindMatch
	index      int
	textClicks []widget.Clickable
	textList   widget.List
	hexMatches []viewerHexFindMatch
	hexClicks  []widget.Clickable
	hexList    widget.List

	currentStart int64
	currentLen   int64
	currentValid bool

	searching  bool
	requestSeq int
	resultCh   chan fileViewerFindResult
	cancel     context.CancelFunc

	pdfMatches    []viewerPDFFindMatch
	pdfClicks     []widget.Clickable
	pdfList       widget.List
	pdfSearched   int
	pdfTotalPages int
	pdfAnchorPage int
	pdfResultCh   chan viewerPDFFindResult
}

type fileViewerFindResult struct {
	requestSeq int
	found      bool
	start      int64
	length     int64
	wrapped    bool
	err        string
	matches    []viewerHexFindMatch
	all        bool
	limited    bool
}

type viewerRemoteSearchSpec struct {
	template string
	mode     string
	shell    viewerShellSpec
	hexInput bool
}

type viewerRemoteSearchTemplateArgs struct {
	fullpath     string
	filename     string
	patternText  string
	patternBytes []byte
	rangeStart   int64
	rangeEnd     int64
	direction    string
	matchLimit   string
	resultSelect string
}

type viewerFindChunkSource struct {
	size  int64
	read  func(start, length int64) ([]byte, error)
	close func() error
}

var runViewerRemoteSearchCommandFunc = func(ctx context.Context, remote *paneSSHSession, cmdline string, shell viewerShellSpec) (string, error) {
	content, _, errText := readViewerRemoteCommand(ctx, remote, cmdline, shell, viewerRemoteSearchMaxBytes, time.Now(), viewerCommandExecTimeout, false, nil)
	if errText != "" {
		return content, errors.New(errText)
	}
	return content, nil
}

func (src viewerFindChunkSource) Close() error {
	if src.close == nil {
		return nil
	}
	return src.close()
}

func (st *fileViewerFindState) closeSourceMenu() {
	if st == nil {
		return
	}
	st.sourceMenuOpen = false
	st.sourceMenuAt = time.Time{}
	st.sourceMenuRect = image.Rectangle{}
}

func (ui *UI) handleFileViewerFindInput(gtx layout.Context, st *fileViewerState) {
	if st == nil || !st.find.open {
		return
	}
	if st.find.focus {
		st.find.focus = false
		gtx.Execute(key.FocusCmd{Tag: &st.find.editor})
	}
	// Handle focused Enter before Editor.Update so Windows doesn't feed the
	// single-line find field with whitespace/newline input instead of stepping.
	anyMods := ^key.Modifiers(0)
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &st.find.editor, Name: key.NameEnter, Optional: anyMods},
			key.Filter{Focus: &st.find.editor, Name: key.NameReturn, Optional: anyMods},
		)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		switch ke.Modifiers {
		case 0:
			if ui.stepFileViewerFind(gtx.Now, 1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.ModShift:
			if ui.stepFileViewerFind(gtx.Now, -1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
	for {
		ev, ok := st.find.editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			ui.refreshFileViewerFind(gtx.Now, false)
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	if st.find.closeClick.Clicked(gtx) {
		ui.closeFileViewerFind()
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.find.prevClick.Clicked(gtx) {
		if ui.stepFileViewerFind(gtx.Now, -1) {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	if st.find.nextClick.Clicked(gtx) {
		if ui.stepFileViewerFind(gtx.Now, 1) {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	if st.mode == "hex" && st.find.previewClick.Clicked(gtx) {
		st.find.hexPreview = !st.find.hexPreview
		redecodeViewerHexFindPreviews(st)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.mode == "hex" && st.find.findByClick.Clicked(gtx) {
		st.find.hexInput = !st.find.hexInput
		ui.refreshFileViewerFind(gtx.Now, false)
		gtx.Execute(op.InvalidateCmd{})
	}
	if viewerPDFPreviewActive(st) {
		for i := range st.find.pdfClicks {
			if st.find.pdfClicks[i].Clicked(gtx) && i < len(st.find.pdfMatches) {
				ui.applyFileViewerPDFFindMatch(gtx.Now, i)
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	} else if st.mode == "hex" {
		for i := range st.find.hexClicks {
			if st.find.hexClicks[i].Clicked(gtx) && i < len(st.find.hexMatches) {
				ui.applyFileViewerHexFindMatch(gtx.Now, i)
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	} else {
		for i := range st.find.textClicks {
			if st.find.textClicks[i].Clicked(gtx) && i < len(st.find.matches) {
				ui.applyFileViewerTextFindMatch(gtx.Now, i)
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
	if st.mode == "hex" && ui.fileViewerFindRemoteSearchConfigured(st) {
		if st.find.sourceMenuClick.Clicked(gtx) {
			if st.find.sourceMenuOpen {
				st.find.closeSourceMenu()
			} else {
				st.find.sourceMenuOpen = true
				st.find.sourceMenuAt = gtx.Now
			}
			gtx.Execute(op.InvalidateCmd{})
		}
		if st.find.sourceLocalClick.Clicked(gtx) {
			st.find.closeSourceMenu()
			if ui.setFileViewerFindRemoteSearch(gtx.Now, false) {
				gtx.Execute(op.InvalidateCmd{})
			}
		}
		if st.find.sourceRemoteClick.Clicked(gtx) {
			st.find.closeSourceMenu()
			if ui.setFileViewerFindRemoteSearch(gtx.Now, true) {
				gtx.Execute(op.InvalidateCmd{})
			} else if st.find.hexInput {
				st.find.status = "Remote search needs a text query or hex-aware command"
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
}

func (ui *UI) openFileViewerFind(now time.Time) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	if !viewerSupportsFind(st) {
		return
	}
	if st.commandEditOn {
		ui.cancelViewerCommandEdit()
	}
	st.setHistoryOpen(false, now)
	st.closeEncodingMenu()
	if !st.find.open {
		st.find.previewIndex = -1
		st.find.previewAt = time.Time{}
		st.find.cursorAnim.reset()
	}
	st.find.open = true
	st.find.focus = true
	ui.ensureFileViewerFindSearchSource(now, st)
	st.find.editor.SetCaret(st.find.editor.Len(), st.find.editor.Len())
	ui.refreshFileViewerFind(now, false)
}

func (ui *UI) closeFileViewerFind() {
	st := ui.fileViewer
	if st == nil || !st.find.open {
		return
	}
	st.find.open = false
	st.find.focus = false
	st.find.status = ""
	st.find.matches = nil
	st.find.index = -1
	st.find.currentValid = false
	st.find.currentStart = 0
	st.find.currentLen = 0
	st.find.textClicks = nil
	st.find.hexMatches = nil
	st.find.hexClicks = nil
	st.find.pdfMatches = nil
	st.find.pdfClicks = nil
	st.find.pdfSearched = 0
	st.find.pdfTotalPages = 0
	st.find.pdfAnchorPage = 0
	st.find.previewIndex = -1
	st.find.previewAt = time.Time{}
	st.find.cursorAnim.reset()
	st.find.sourceButtonRect = image.Rectangle{}
	st.find.findByButtonRect = image.Rectangle{}
	st.find.previewButtonRect = image.Rectangle{}
	st.find.closeSourceMenu()
	ui.cancelFileViewerFindSearch(st)
	ui.closeEditorContextMenu()
}

func (ui *UI) cancelFileViewerFindSearch(st *fileViewerState) {
	if st == nil {
		return
	}
	if st.find.cancel != nil {
		st.find.cancel()
		st.find.cancel = nil
		st.find.requestSeq++
	}
	st.find.searching = false
	st.find.searchStartedAt = time.Time{}
}

func (ui *UI) viewerRemoteSearchTemplate(remote *paneSSHSession) string {
	if ui == nil || remote == nil || ui.fmCfg == nil {
		return ""
	}
	return fm.EffectiveViewerRemoteSearchCommand(ui.fmCfg.Viewer.RemoteSearchCommand)
}

func (ui *UI) viewerRemoteSearchSpec(remote *paneSSHSession, hexInput, enabled bool) viewerRemoteSearchSpec {
	if ui == nil || remote == nil || ui.fmCfg == nil {
		return viewerRemoteSearchSpec{}
	}
	template := ui.viewerRemoteSearchTemplate(remote)
	if template == "" {
		return viewerRemoteSearchSpec{}
	}
	mode := fm.ViewerRemoteSearchModeLocal
	if enabled {
		mode = fm.ViewerRemoteSearchModeRemote
	}
	return viewerRemoteSearchSpec{
		template: template,
		mode:     mode,
		shell:    resolveViewerShell(ui.fmCfg.Viewer.Shell, true),
		hexInput: hexInput,
	}
}

func (ui *UI) refreshFileViewerFind(now time.Time, preserve bool) {
	st := ui.fileViewer
	if st == nil || !st.find.open {
		return
	}
	if !viewerSupportsFind(st) {
		ui.closeFileViewerFind()
		return
	}
	ui.prepareFileViewerFindMode(st)
	ui.ensureFileViewerFindSearchSource(now, st)
	ui.syncFileViewerFindRemoteSearch(now, st)
	query := st.find.editor.Text()
	if st.mode == "hex" {
		ui.refreshHexFileViewerFind(now, query, preserve)
		return
	}
	if viewerPDFPreviewActive(st) {
		ui.refreshPDFFileViewerFind(now, query, preserve)
		return
	}
	ui.refreshStreamFileViewerFind(now, query, preserve)
}

func viewerFileFindModeKey(st *fileViewerState) string {
	if st == nil {
		return ""
	}
	if viewerPDFPreviewActive(st) {
		return "pdf"
	}
	return normalizeViewerMode(st.mode)
}

func (ui *UI) prepareFileViewerFindMode(st *fileViewerState) {
	if st == nil {
		return
	}
	modeKey := viewerFileFindModeKey(st)
	if st.find.modeKey == modeKey {
		return
	}
	ui.cancelFileViewerFindSearch(st)
	st.find.modeKey = modeKey
	st.find.matches = nil
	st.find.textClicks = nil
	st.find.hexMatches = nil
	st.find.hexClicks = nil
	st.find.pdfMatches = nil
	st.find.pdfClicks = nil
	st.find.pdfSearched = 0
	st.find.pdfTotalPages = 0
	st.find.index = -1
	st.find.currentValid = false
	st.find.currentStart = 0
	st.find.currentLen = 0
	st.find.status = ""
	st.find.previewIndex = -1
	st.find.previewAt = time.Time{}
	st.find.cursorAnim.reset()
}

func (ui *UI) refreshPDFFileViewerFind(now time.Time, query string, preserve bool) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	anchorPage := st.pdfDoc.currentPage()
	if preserve && st.find.index >= 0 && st.find.index < len(st.find.pdfMatches) {
		anchorPage = st.find.pdfMatches[st.find.index].Page
	}
	ui.cancelFileViewerFindSearch(st)
	st.find.pdfMatches = nil
	st.find.pdfClicks = nil
	st.find.pdfSearched = 0
	st.find.pdfAnchorPage = anchorPage
	st.find.pdfTotalPages = st.pdfDoc.pageCount()
	if st.find.pdfTotalPages <= 0 {
		st.find.pdfTotalPages = st.imagePreviewPageCount
	}
	st.find.index = -1
	st.find.currentValid = false
	st.find.currentStart = 0
	st.find.currentLen = 0
	if query == "" || st.find.pdfTotalPages <= 0 {
		st.find.status = ""
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	st.find.cancel = cancel
	st.find.searching = true
	st.find.searchStartedAt = now
	st.find.status = ""
	st.find.requestSeq++
	requestSeq := st.find.requestSeq
	pageCount := st.find.pdfTotalPages
	previewStart, previewEnd := ui.fileViewerFindPreviewRange()
	localPath, data := pdfDocRenderSource(st)
	backend := viewerPDFPreviewBackend
	ch := st.find.pdfResultCh

	go func() {
		for page := 0; page < pageCount; page++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			text, err := backend.PageText(viewerPDFRenderRequest{Data: data, LocalPath: localPath, Page: page})
			res := viewerPDFFindResult{requestSeq: requestSeq, searched: page + 1}
			if err != nil {
				res.err = err.Error()
				res.done = true
				if sendViewerPDFFindResult(ctx, ch, res) {
					ui.invalidateFromWorker()
				}
				return
			}
			res.pageText = &text
			res.matches = viewerPDFFindPageMatchesWithPreview(text, query, previewStart, previewEnd)
			if !sendViewerPDFFindResult(ctx, ch, res) {
				return
			}
			ui.invalidateFromWorker()
		}
		if sendViewerPDFFindResult(ctx, ch, viewerPDFFindResult{
			requestSeq: requestSeq,
			searched:   pageCount,
			done:       true,
		}) {
			ui.invalidateFromWorker()
		}
	}()

}

func viewerPDFFindPageMatches(text viewerPDFPageText, query string) []viewerPDFFindMatch {
	return viewerPDFFindPageMatchesWithPreview(text, query, 0, 2)
}

func viewerPDFFindPageMatchesWithPreview(text viewerPDFPageText, query string, previewStart, previewEnd int) []viewerPDFFindMatch {
	queryRunes := []rune(query)
	if len(text.Chars) == 0 || len(queryRunes) == 0 || len(queryRunes) > len(text.Chars) {
		return nil
	}
	needle := make([]rune, len(queryRunes))
	for i, r := range queryRunes {
		needle[i] = unicode.ToLower(r)
	}
	pageRunes := make([]rune, len(text.Chars))
	pageRows := make([]int, len(text.Chars)+1)
	for i, ch := range text.Chars {
		pageRunes[i] = ch.Rune
		pageRows[i+1] = pageRows[i]
		if ch.Rune == '\n' {
			pageRows[i+1]++
		}
	}
	pageLines := strings.Split(string(pageRunes), "\n")
	matches := make([]viewerPDFFindMatch, 0, 4)
	for start := 0; start+len(needle) <= len(text.Chars); start++ {
		matched := true
		for i, r := range needle {
			if unicode.ToLower(text.Chars[start+i].Rune) != r {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		end := start + len(needle)
		row := pageRows[start]
		preview, previewFocus := compactFindPreviewWindow(pageLines, row, previewStart, previewEnd)
		snippet, snippetHighlight := viewerPDFFindSnippetWithHighlight(text.Chars, start, end)
		rowStart := start
		for rowStart > 0 && text.Chars[rowStart-1].Rune != '\n' {
			rowStart--
		}
		rowEnd := end
		for rowEnd < len(text.Chars) && text.Chars[rowEnd].Rune != '\n' {
			rowEnd++
		}
		previewHighlights := compactFindPreviewRuneHighlights(preview, previewFocus, start-rowStart, min(end, rowEnd)-rowStart)
		matches = append(matches, viewerPDFFindMatch{
			Page:              text.Page,
			Start:             start,
			End:               end,
			Snippet:           snippet,
			SnippetHighlight:  snippetHighlight,
			Preview:           preview,
			PreviewFocus:      previewFocus,
			PreviewHighlights: previewHighlights,
		})
	}
	return matches
}

func viewerPDFFindSnippet(chars []viewerPDFTextChar, start, end int) string {
	snippet, _ := viewerPDFFindSnippetWithHighlight(chars, start, end)
	return snippet
}

func viewerPDFFindSnippetWithHighlight(chars []viewerPDFTextChar, start, end int) (string, compactFindHighlight) {
	const contextRunes = 34
	from := start - contextRunes
	if from < 0 {
		from = 0
	}
	to := end + contextRunes
	if to > len(chars) {
		to = len(chars)
	}
	runes := make([]rune, 0, to-from)
	for _, ch := range chars[from:to] {
		runes = append(runes, ch.Rune)
	}
	raw := string(runes)
	matchStartByte := compactFindRuneByteOffset(raw, start-from)
	matchEndByte := compactFindRuneByteOffset(raw, end-from)
	snippet, highlight := compactFindCollapsedTextHighlight(raw, matchStartByte, matchEndByte)
	if from > 0 {
		snippet = "…" + snippet
		if highlight.Start >= 0 {
			highlight.Start += len("…")
			highlight.End += len("…")
		}
	}
	if to < len(chars) {
		snippet += "…"
	}
	return snippet, highlight
}

func (ui *UI) refreshStreamFileViewerFind(now time.Time, query string, preserve bool) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	ui.cancelFileViewerFindSearch(st)
	st.find.matches = nil
	st.find.index = -1
	st.find.currentValid = false
	st.find.currentStart = 0
	st.find.currentLen = 0
	if query == "" {
		st.find.status = ""
		return
	}
	previewStart, previewEnd := ui.fileViewerFindPreviewRange()
	matches := viewerFindTextMatchesWithPreview(st.content, query, previewStart, previewEnd)
	st.find.matches = matches
	st.find.textClicks = make([]widget.Clickable, len(matches))
	if len(matches) == 0 {
		st.find.status = "No matches"
		return
	}
	anchor := viewerStreamFindAnchor(st, preserve)
	idx := viewerFindTextMatchIndexAtOrAfter(matches, anchor)
	ui.applyFileViewerTextFindMatch(now, idx)
}

func (ui *UI) applyFileViewerTextFindMatch(now time.Time, idx int) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	if idx < 0 || idx >= len(st.find.matches) {
		st.find.index = -1
		st.find.currentValid = false
		st.find.currentStart = 0
		st.find.currentLen = 0
		st.find.status = "No matches"
		return
	}
	match := st.find.matches[idx]
	st.find.index = idx
	st.find.currentStart = int64(match.Start)
	st.find.currentLen = int64(match.End - match.Start)
	st.find.currentValid = st.find.currentLen > 0
	st.find.status = fmt.Sprintf("%d/%d", idx+1, len(st.find.matches))
	viewerScrollStreamFindMatch(st, match, now)
	st.find.textList.ScrollTo(idx)
}

func (ui *UI) refreshHexFileViewerFind(now time.Time, query string, preserve bool) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	pattern, errText := viewerFindPatternBytes(query, st.find.hexInput)
	useHex := st.find.hexInput
	if errText != "" {
		ui.cancelFileViewerFindSearch(st)
		st.find.hexMatches = nil
		st.find.hexClicks = nil
		st.find.status = errText
		st.find.currentValid = false
		st.find.currentStart = 0
		st.find.currentLen = 0
		return
	}
	if len(pattern) == 0 {
		ui.cancelFileViewerFindSearch(st)
		st.find.hexMatches = nil
		st.find.hexClicks = nil
		st.find.status = ""
		st.find.currentValid = false
		st.find.currentStart = 0
		st.find.currentLen = 0
		return
	}
	anchor := viewerHexFindAnchor(st, preserve)
	ui.startHexFileViewerFindAll(now, pattern, anchor, useHex)
	ui.seedBufferedHexFileViewerFind(now, st, pattern, anchor)
}

func (ui *UI) seedBufferedHexFileViewerFind(now time.Time, st *fileViewerState, pattern []byte, anchor int64) {
	if st == nil || st.hex == nil || len(st.hex.buffer) == 0 || len(pattern) == 0 {
		return
	}
	matches := viewerBufferedHexFindMatches(st.hex.buffer, st.hex.bufferStart, pattern, viewerHexFindResultLimit)
	if len(matches) == 0 {
		return
	}
	st.find.hexMatches = matches
	st.find.hexClicks = make([]widget.Clickable, len(matches))
	idx := sort.Search(len(matches), func(i int) bool { return matches[i].Start >= anchor })
	if idx >= len(matches) {
		idx = 0
	}
	ui.applyFileViewerHexFindMatch(now, idx)
}

func viewerBufferedHexFindMatches(buffer []byte, bufferStart int64, pattern []byte, limit int) []viewerHexFindMatch {
	if len(buffer) == 0 || len(pattern) == 0 || limit < 1 {
		return nil
	}
	matches := make([]viewerHexFindMatch, 0, min(limit, 16))
	for cursor := 0; cursor+len(pattern) <= len(buffer); {
		idx := bytes.Index(buffer[cursor:], pattern)
		if idx < 0 {
			break
		}
		idx += cursor
		from := idx - viewerHexPreviewBytesPerRow
		if from < 0 {
			from = 0
		}
		to := from + viewerHexPreviewBytesPerRow*viewerHexPreviewMaxRows
		if minimum := idx + len(pattern) + viewerHexPreviewBytesPerRow; to < minimum {
			to = minimum
		}
		if to > len(buffer) {
			to = len(buffer)
		}
		preview := append([]byte(nil), buffer[from:to]...)
		previewMatch := idx - from
		compact := viewerHexCompactPreviewBytes(viewerHexFindMatch{Length: int64(len(pattern)), PreviewBytes: preview, PreviewMatch: previewMatch})
		matches = append(matches, viewerHexFindMatch{
			Start:        bufferStart + int64(idx),
			Length:       int64(len(pattern)),
			PreviewBytes: preview,
			PreviewMatch: previewMatch,
			TextPreview:  viewerHexTextSnippet(compact),
			HexPreview:   viewerHexBytesSnippet(compact),
		})
		if len(matches) >= limit {
			break
		}
		cursor = idx + 1
	}
	return matches
}

func (ui *UI) stepFileViewerFind(now time.Time, direction int) bool {
	st := ui.fileViewer
	if st == nil || !st.find.open {
		return false
	}
	if st.mode == "hex" {
		if len(st.find.hexMatches) == 0 {
			return false
		}
		idx := st.find.index
		if idx < 0 {
			idx = 0
		} else if direction < 0 {
			idx = (idx - 1 + len(st.find.hexMatches)) % len(st.find.hexMatches)
		} else {
			idx = (idx + 1) % len(st.find.hexMatches)
		}
		ui.applyFileViewerHexFindMatch(now, idx)
		return true
	}
	if viewerPDFPreviewActive(st) {
		if len(st.find.pdfMatches) == 0 {
			return false
		}
		idx := st.find.index
		if idx < 0 {
			idx = 0
		} else if direction < 0 {
			idx = (idx - 1 + len(st.find.pdfMatches)) % len(st.find.pdfMatches)
		} else {
			idx = (idx + 1) % len(st.find.pdfMatches)
		}
		ui.applyFileViewerPDFFindMatch(now, idx)
		return true
	}
	query := st.find.editor.Text()
	if query == "" {
		st.find.status = ""
		return false
	}
	if len(st.find.matches) == 0 {
		ui.refreshStreamFileViewerFind(now, query, true)
		return st.find.currentValid
	}
	idx := st.find.index
	switch {
	case idx >= 0 && direction > 0:
		idx = (idx + 1) % len(st.find.matches)
	case idx >= 0 && direction < 0:
		idx = (idx - 1 + len(st.find.matches)) % len(st.find.matches)
	case direction < 0:
		idx = viewerFindTextMatchIndexBefore(st.find.matches, viewerStreamFindAnchor(st, false))
	default:
		idx = viewerFindTextMatchIndexAtOrAfter(st.find.matches, viewerStreamFindAnchor(st, false))
	}
	ui.applyFileViewerTextFindMatch(now, idx)
	return true
}

func (ui *UI) applyFileViewerPDFFindMatch(now time.Time, idx int) {
	st := ui.fileViewer
	if st == nil || idx < 0 || idx >= len(st.find.pdfMatches) {
		return
	}
	match := st.find.pdfMatches[idx]
	st.find.index = idx
	st.find.currentStart = int64(match.Start)
	st.find.currentLen = int64(match.End - match.Start)
	st.find.currentValid = match.End > match.Start
	st.find.status = fmt.Sprintf("%d/%d", idx+1, len(st.find.pdfMatches))
	st.pdfDoc.scrollToTextMatch(match)
	st.pdfDoc.clearSelection()
	st.markUserBrowsing(now)
	// Keep the selected result visible in the compact rail.
	st.find.pdfList.ScrollTo(idx)
}

func (ui *UI) applyFileViewerHexFindMatch(now time.Time, idx int) {
	st := ui.fileViewer
	if st == nil || idx < 0 || idx >= len(st.find.hexMatches) {
		return
	}
	match := st.find.hexMatches[idx]
	st.find.index = idx
	st.find.currentStart = match.Start
	st.find.currentLen = match.Length
	st.find.currentValid = match.Length > 0
	st.find.status = fmt.Sprintf("%d/%d", idx+1, len(st.find.hexMatches))
	viewerScrollHexFindMatch(st, match.Start, match.Length, now)
	st.find.hexList.ScrollTo(idx)
	ui.startHexViewerLoad(st, false)
}

func (ui *UI) startHexFileViewerFindAll(now time.Time, pattern []byte, anchor int64, useHex bool) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	ui.cancelFileViewerFindSearch(st)
	st.find.hexMatches = nil
	st.find.hexClicks = nil
	st.find.index = -1
	st.find.currentValid = false
	st.find.currentStart = 0
	st.find.currentLen = 0
	ctx, cancel := context.WithCancel(context.Background())
	st.find.cancel = cancel
	st.find.searching = true
	st.find.searchStartedAt = now
	st.find.status = ""
	st.find.requestSeq++
	requestSeq := st.find.requestSeq
	path := st.path
	remote := st.remote
	remoteSearch := ui.viewerRemoteSearchSpec(remote, useHex, st.find.remoteSearch)
	ch := st.find.resultCh

	go func() {
		matches, limited, err := searchViewerHexAll(ctx, path, remote, pattern, remoteSearch, viewerHexFindResultLimit)
		res := fileViewerFindResult{
			requestSeq: requestSeq,
			matches:    matches,
			all:        true,
			limited:    limited,
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			res.err = err.Error()
		}
		if len(matches) > 0 {
			idx := sort.Search(len(matches), func(i int) bool { return matches[i].Start >= anchor })
			if idx >= len(matches) {
				idx = 0
			}
			res.found = true
			res.start = matches[idx].Start
			res.length = matches[idx].Length
		}
		sendFileViewerFindResult(ch, res)
		ui.invalidateFromWorker()
	}()
}

func (ui *UI) stepHexFileViewerFind(now time.Time, direction int) bool {
	st := ui.fileViewer
	if st == nil {
		return false
	}
	pattern, errText := viewerFindPatternBytes(st.find.editor.Text(), st.find.hexInput)
	useHex := st.find.hexInput
	if errText != "" {
		ui.cancelFileViewerFindSearch(st)
		st.find.status = errText
		st.find.currentValid = false
		return false
	}
	if len(pattern) == 0 {
		st.find.status = ""
		st.find.currentValid = false
		return false
	}
	if direction < 0 {
		anchor := viewerHexFindAnchor(st, false)
		if st.find.currentValid {
			anchor = st.find.currentStart
		}
		ui.startHexFileViewerFindPrev(now, pattern, anchor, useHex)
		return true
	}
	anchor := viewerHexFindAnchor(st, false)
	if st.find.currentValid {
		anchor = st.find.currentStart + 1
	}
	ui.startHexFileViewerFindNext(now, pattern, anchor, useHex)
	return true
}

func (ui *UI) startHexFileViewerFindNext(now time.Time, pattern []byte, anchor int64, useHex bool) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	ui.cancelFileViewerFindSearch(st)
	ctx, cancel := context.WithCancel(context.Background())
	st.find.cancel = cancel
	st.find.searching = true
	st.find.searchStartedAt = now
	if !st.find.currentValid {
		st.find.status = ""
	}
	st.find.requestSeq++
	requestSeq := st.find.requestSeq
	path := st.path
	remote := st.remote
	remoteSearch := ui.viewerRemoteSearchSpec(remote, useHex, st.find.remoteSearch)
	ch := st.find.resultCh

	go func() {
		res := searchViewerHexNext(ctx, path, remote, pattern, anchor, remoteSearch)
		res.requestSeq = requestSeq
		sendFileViewerFindResult(ch, res)
		ui.invalidateFromWorker()
	}()

	if st.hex != nil {
		st.markUserBrowsing(now)
	}
}

func (ui *UI) startHexFileViewerFindPrev(now time.Time, pattern []byte, anchor int64, useHex bool) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	current := int64(-1)
	if st.find.currentValid {
		current = st.find.currentStart
	}
	ui.cancelFileViewerFindSearch(st)
	ctx, cancel := context.WithCancel(context.Background())
	st.find.cancel = cancel
	st.find.searching = true
	st.find.searchStartedAt = now
	if !st.find.currentValid {
		st.find.status = ""
	}
	st.find.requestSeq++
	requestSeq := st.find.requestSeq
	path := st.path
	remote := st.remote
	remoteSearch := ui.viewerRemoteSearchSpec(remote, useHex, st.find.remoteSearch)
	ch := st.find.resultCh

	go func() {
		res := searchViewerHexPrev(ctx, path, remote, pattern, anchor, current, remoteSearch)
		res.requestSeq = requestSeq
		sendFileViewerFindResult(ch, res)
		ui.invalidateFromWorker()
	}()

	if st.hex != nil {
		st.markUserBrowsing(now)
	}
}

func (ui *UI) pumpFileViewerFindState(gtx layout.Context, st *fileViewerState) {
	if st == nil || st.find.resultCh == nil {
		return
	}
	ui.pumpPDFFileViewerFindState(gtx, st)
	for {
		select {
		case res := <-st.find.resultCh:
			if res.requestSeq != st.find.requestSeq {
				continue
			}
			st.find.searching = false
			st.find.cancel = nil
			st.find.searchStartedAt = time.Time{}
			if res.all {
				if res.err != "" {
					st.find.status = res.err
					st.find.currentValid = false
					gtx.Execute(op.InvalidateCmd{})
					continue
				}
				st.find.hexMatches = res.matches
				st.find.hexClicks = make([]widget.Clickable, len(res.matches))
				if len(res.matches) == 0 {
					st.find.status = "No matches"
					st.find.currentValid = false
					gtx.Execute(op.InvalidateCmd{})
					continue
				}
				idx := sort.Search(len(res.matches), func(i int) bool { return res.matches[i].Start >= res.start })
				if idx >= len(res.matches) {
					idx = 0
				}
				ui.applyFileViewerHexFindMatch(gtx.Now, idx)
				if res.limited {
					st.find.status += "+"
				}
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			if res.err != "" {
				st.find.status = res.err
				st.find.currentValid = false
				st.find.currentStart = 0
				st.find.currentLen = 0
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			if !res.found {
				st.find.status = "No matches"
				st.find.currentValid = false
				st.find.currentStart = 0
				st.find.currentLen = 0
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			st.find.currentStart = res.start
			st.find.currentLen = res.length
			st.find.currentValid = res.length > 0
			st.find.status = viewerHexFindStatus(res.start, res.wrapped)
			viewerScrollHexFindMatch(st, res.start, res.length, gtx.Now)
			ui.startHexViewerLoad(st, false)
			gtx.Execute(op.InvalidateCmd{})
		default:
			return
		}
	}
}

func (ui *UI) pumpPDFFileViewerFindState(gtx layout.Context, st *fileViewerState) {
	if st == nil || st.find.pdfResultCh == nil {
		return
	}
	for {
		select {
		case res := <-st.find.pdfResultCh:
			if res.requestSeq != st.find.requestSeq {
				continue
			}
			if res.pageText != nil {
				st.pdfDoc.storeText(*res.pageText)
			}
			st.find.pdfSearched = res.searched
			if len(res.matches) > 0 {
				st.find.pdfMatches = append(st.find.pdfMatches, res.matches...)
				for len(st.find.pdfClicks) < len(st.find.pdfMatches) {
					st.find.pdfClicks = append(st.find.pdfClicks, widget.Clickable{})
				}
				if st.find.index < 0 {
					for i, match := range st.find.pdfMatches {
						if match.Page >= st.find.pdfAnchorPage {
							ui.applyFileViewerPDFFindMatch(gtx.Now, i)
							break
						}
					}
				}
			}
			if res.err != "" {
				st.find.status = res.err
				st.find.searching = false
				st.find.currentValid = false
				if st.find.cancel != nil {
					st.find.cancel()
					st.find.cancel = nil
				}
			} else if res.done {
				st.find.searching = false
				st.find.searchStartedAt = time.Time{}
				if st.find.cancel != nil {
					st.find.cancel()
					st.find.cancel = nil
				}
				if len(st.find.pdfMatches) == 0 {
					st.find.status = "No matches"
				} else {
					if st.find.index < 0 {
						ui.applyFileViewerPDFFindMatch(gtx.Now, 0)
					}
					st.find.status = fmt.Sprintf("%d/%d", st.find.index+1, len(st.find.pdfMatches))
				}
			} else if len(st.find.pdfMatches) > 0 {
				if st.find.index >= 0 {
					st.find.status = fmt.Sprintf("%d/%d · %d/%d pages", st.find.index+1, len(st.find.pdfMatches), st.find.pdfSearched, st.find.pdfTotalPages)
				} else {
					st.find.status = fmt.Sprintf("%d matches · %d/%d pages", len(st.find.pdfMatches), st.find.pdfSearched, st.find.pdfTotalPages)
				}
			}
			gtx.Execute(op.InvalidateCmd{})
		default:
			return
		}
	}
}

func sendViewerPDFFindResult(ctx context.Context, ch chan viewerPDFFindResult, res viewerPDFFindResult) bool {
	if ch == nil {
		return false
	}
	select {
	case ch <- res:
		return true
	case <-ctx.Done():
		return false
	}
}

func sendFileViewerFindResult(ch chan fileViewerFindResult, res fileViewerFindResult) {
	if ch == nil {
		return
	}
	select {
	case ch <- res:
	default:
		// Treat the size-one channel as a latest-request mailbox. A canceled
		// search can finish after a newer one and must never evict its result.
		newest := res
		select {
		case queued := <-ch:
			if queued.requestSeq > newest.requestSeq {
				newest = queued
			}
		default:
		}
		select {
		case ch <- newest:
		default:
		}
	}
}

// Worker completions must wake Gio explicitly. Continuous test frames and
// animations can otherwise hide a queued result until the next user event.
func (ui *UI) invalidateFromWorker() {
	if ui != nil && ui.invalidate != nil {
		ui.invalidate()
	}
}

func viewerFindTextMatches(content, query string) []viewerFindMatch {
	return viewerFindTextMatchesWithPreview(content, query, 0, 2)
}

func viewerFindTextMatchesWithPreview(content, query string, previewStart, previewEnd int) []viewerFindMatch {
	if content == "" || query == "" {
		return nil
	}
	matches := make([]viewerFindMatch, 0, 8)
	lines := strings.Split(content, "\n")
	line := 1
	lineScan := 0
	for off := 0; off <= len(content)-len(query); {
		idx := strings.Index(content[off:], query)
		if idx < 0 {
			break
		}
		start := off + idx
		line += strings.Count(content[lineScan:start], "\n")
		lineScan = start
		end := start + len(query)
		preview, previewFocus := compactFindPreviewWindow(lines, line-1, previewStart, previewEnd)
		snippet, snippetHighlight := viewerFindTextSnippetWithHighlight(content, start, end)
		lineStart := strings.LastIndex(content[:start], "\n") + 1
		lineEnd := len(content)
		if at := strings.IndexByte(content[start:], '\n'); at >= 0 {
			lineEnd = start + at
		}
		previewHighlights := compactFindPreviewByteHighlights(preview, previewFocus, start-lineStart, min(end, lineEnd)-lineStart)
		matches = append(matches, viewerFindMatch{
			Start:             start,
			End:               end,
			Line:              line,
			Snippet:           snippet,
			SnippetHighlight:  snippetHighlight,
			Preview:           preview,
			PreviewFocus:      previewFocus,
			PreviewHighlights: previewHighlights,
		})
		off = start + 1
	}
	return matches
}

func (ui *UI) fileViewerFindPreviewRange() (int, int) {
	if ui == nil || ui.fmCfg == nil {
		return fm.NormalizeViewerPreviewRange(0, 2)
	}
	return fm.NormalizeViewerPreviewRange(ui.fmCfg.Viewer.PreviewStart, ui.fmCfg.Viewer.PreviewEnd)
}

func viewerFindTextSnippet(content string, start, end int) string {
	snippet, _ := viewerFindTextSnippetWithHighlight(content, start, end)
	return snippet
}

func viewerFindTextSnippetWithHighlight(content string, start, end int) (string, compactFindHighlight) {
	if start < 0 || end < start || start > len(content) {
		return "", compactFindHighlight{Start: -1, End: -1}
	}
	if end > len(content) {
		end = len(content)
	}
	lineStart := strings.LastIndex(content[:start], "\n") + 1
	lineEnd := len(content)
	if idx := strings.IndexByte(content[end:], '\n'); idx >= 0 {
		lineEnd = end + idx
	}
	from := lineStart
	if start-from > 56 {
		from = start - 56
	}
	to := lineEnd
	if to-end > 76 {
		to = end + 76
	}
	raw := strings.ToValidUTF8(content[from:to], "")
	snippet, highlight := compactFindCollapsedTextHighlight(raw, start-from, end-from)
	if from > lineStart {
		snippet = "…" + snippet
		if highlight.Start >= 0 {
			highlight.Start += len("…")
			highlight.End += len("…")
		}
	}
	if to < lineEnd {
		snippet += "…"
	}
	return snippet, highlight
}

func compactFindPreviewByteHighlights(lines []string, focus, startByte, endByte int) []compactFindHighlight {
	highlights := make([]compactFindHighlight, len(lines))
	for i := range highlights {
		highlights[i] = compactFindHighlight{Start: -1, End: -1}
	}
	if focus < 0 || focus >= len(lines) {
		return highlights
	}
	_, highlights[focus] = compactFindTabbedTextHighlight(lines[focus], startByte, endByte)
	return highlights
}

func compactFindPreviewRuneHighlights(lines []string, focus, startRune, endRune int) []compactFindHighlight {
	if focus < 0 || focus >= len(lines) {
		return compactFindPreviewByteHighlights(lines, focus, 0, 0)
	}
	startByte := compactFindRuneByteOffset(lines[focus], startRune)
	endByte := compactFindRuneByteOffset(lines[focus], endRune)
	return compactFindPreviewByteHighlights(lines, focus, startByte, endByte)
}

func viewerFindTextMatchIndexAtOrAfter(matches []viewerFindMatch, anchor int) int {
	if len(matches) == 0 {
		return -1
	}
	idx := sort.Search(len(matches), func(i int) bool {
		return matches[i].Start >= anchor
	})
	if idx >= len(matches) {
		return 0
	}
	return idx
}

func viewerFindTextMatchIndexBefore(matches []viewerFindMatch, anchor int) int {
	if len(matches) == 0 {
		return -1
	}
	idx := sort.Search(len(matches), func(i int) bool {
		return matches[i].Start >= anchor
	}) - 1
	if idx < 0 {
		return len(matches) - 1
	}
	return idx
}

func viewerStreamFindAnchor(st *fileViewerState, preserve bool) int {
	if st == nil {
		return 0
	}
	if preserve && st.find.currentValid {
		return int(st.find.currentStart)
	}
	if start, _, ok := st.stream.selectionBounds(); ok {
		return start
	}
	return st.stream.lineByteStart(st.stream.topLine)
}

func viewerHexFindAnchor(st *fileViewerState, preserve bool) int64 {
	if st == nil {
		return 0
	}
	if preserve && st.find.currentValid {
		return st.find.currentStart
	}
	if st.hex != nil && st.hex.hasSelection() {
		return st.hex.selectionStart
	}
	if st.hex != nil && st.hex.bytesPerLine > 0 {
		return st.hex.topLine * int64(st.hex.bytesPerLine)
	}
	return 0
}

func viewerScrollStreamFindMatch(st *fileViewerState, match viewerFindMatch, now time.Time) {
	if st == nil {
		return
	}
	v := &st.stream
	line, local, ok := v.lineForOffset(match.Start)
	if !ok {
		return
	}
	visible := v.visibleLines
	if visible < 1 {
		visible = 1
	}
	row := line
	if v.wrapEnabled {
		row = v.rowForLineCol(line, runeIndexAtByte(v.lines[line], local))
	}
	v.topLine = viewerKeepStreamLineVisible(v.topLine, visible, row)
	v.clampTop()
	v.syncVisualTop()
	if !v.wrapEnabled && line >= 0 && line < len(v.lines) {
		lineText := v.lines[line]
		fromCol := runeIndexAtByte(lineText, local)
		toLocal := local + (match.End - match.Start)
		if toLocal > len(lineText) {
			toLocal = len(lineText)
		}
		toCol := runeIndexAtByte(lineText, toLocal)
		if toCol <= fromCol {
			toCol = fromCol + 1
		}
		if v.textRect.Dx() > 0 {
			visibleCols := v.visibleCols(v.textRect.Dx())
			if visibleCols < 1 {
				visibleCols = 1
			}
			if fromCol < v.hCol || toCol > v.hCol+visibleCols {
				target := fromCol - 4
				if target < 0 {
					target = 0
				}
				if toCol > target+visibleCols {
					target = toCol - visibleCols + 2
				}
				if target < 0 {
					target = 0
				}
				v.hCol = target
				v.clampHCol(v.textRect.Dx())
			}
		} else {
			v.hCol = fromCol
			if v.hCol > 4 {
				v.hCol -= 4
			}
		}
	}
	st.markUserBrowsing(now)
}

func viewerScrollHexFindMatch(st *fileViewerState, start, length int64, now time.Time) {
	if st == nil || st.hex == nil {
		return
	}
	v := st.hex
	if v.bytesPerLine <= 0 {
		v.bytesPerLine = 16
	}
	line := start / int64(v.bytesPerLine)
	visible := v.visibleLines
	if visible < 1 {
		visible = 1
	}
	v.topLine = viewerKeepHexLineVisible(v.topLine, visible, line)
	v.clampTop()
	if length > 0 {
		st.markUserBrowsing(now)
	}
}

func viewerKeepStreamLineVisible(topLine, visibleLines, line int) int {
	if visibleLines < 1 {
		visibleLines = 1
	}
	if line < topLine {
		return line
	}
	lastVisible := topLine + visibleLines - 1
	if line > lastVisible {
		return line - visibleLines + 1
	}
	return topLine
}

func viewerKeepHexLineVisible(topLine int64, visibleLines int, line int64) int64 {
	if visibleLines < 1 {
		visibleLines = 1
	}
	if line < topLine {
		return line
	}
	lastVisible := topLine + int64(visibleLines) - 1
	if line > lastVisible {
		return line - int64(visibleLines) + 1
	}
	return topLine
}

func viewerFindPatternBytes(raw string, hexInput bool) ([]byte, string) {
	if !hexInput {
		if raw == "" {
			return nil, ""
		}
		return []byte(raw), ""
	}
	pat, err := parseViewerFindHexString(raw)
	if err != "" {
		return nil, err
	}
	return pat, ""
}

func (st *fileViewerFindState) setRemoteSearch(enabled bool, now time.Time) bool {
	_ = now
	if st == nil || st.remoteSearch == enabled {
		return false
	}
	st.remoteSearch = enabled
	st.sourceInit = true
	return true
}

func (ui *UI) fileViewerFindRemoteSearchConfigured(st *fileViewerState) bool {
	if st == nil || st.mode != "hex" || st.remote == nil {
		return false
	}
	return strings.TrimSpace(ui.viewerRemoteSearchTemplate(st.remote)) != ""
}

func (ui *UI) fileViewerFindRemoteSearchAvailable(st *fileViewerState) bool {
	if !ui.fileViewerFindRemoteSearchConfigured(st) {
		return false
	}
	if st != nil && st.find.hexInput && !viewerRemoteSearchTemplateSupportsHex(ui.viewerRemoteSearchTemplate(st.remote)) {
		return false
	}
	return true
}

func (ui *UI) defaultFileViewerFindRemoteSearch(st *fileViewerState) bool {
	if ui == nil || ui.fmCfg == nil || !ui.fileViewerFindRemoteSearchConfigured(st) {
		return false
	}
	return fm.NormalizeViewerRemoteSearchMode(ui.fmCfg.Viewer.RemoteSearchMode) != fm.ViewerRemoteSearchModeLocal
}

func (ui *UI) ensureFileViewerFindSearchSource(now time.Time, st *fileViewerState) {
	if st == nil || st.find.sourceInit {
		return
	}
	st.find.remoteSearch = ui.defaultFileViewerFindRemoteSearch(st)
	st.find.sourceInit = true
	ui.syncFileViewerFindRemoteSearch(now, st)
}

func (ui *UI) syncFileViewerFindRemoteSearch(now time.Time, st *fileViewerState) bool {
	if st == nil || !st.find.remoteSearch {
		return false
	}
	if ui.fileViewerFindRemoteSearchAvailable(st) {
		return false
	}
	return st.find.setRemoteSearch(false, now)
}

func (ui *UI) setFileViewerFindRemoteSearch(now time.Time, enabled bool) bool {
	st := ui.fileViewer
	if st == nil {
		return false
	}
	if enabled && !ui.fileViewerFindRemoteSearchAvailable(st) {
		return false
	}
	if !st.find.setRemoteSearch(enabled, now) {
		return false
	}
	ui.refreshFileViewerFind(now, false)
	return true
}

func parseViewerFindHexString(raw string) ([]byte, string) {
	if strings.TrimSpace(raw) == "" {
		return nil, ""
	}
	replacer := strings.NewReplacer(
		"0x", "",
		"0X", "",
		" ", "",
		"\t", "",
		"\r", "",
		"\n", "",
		"_", "",
		"-", "",
		":", "",
		",", "",
	)
	compact := replacer.Replace(raw)
	if compact == "" {
		return nil, ""
	}
	if len(compact)%2 != 0 {
		return nil, "hex query needs full bytes"
	}
	out := make([]byte, len(compact)/2)
	for i := 0; i < len(compact); i += 2 {
		hi, ok := viewerHexNibble(compact[i])
		if !ok {
			return nil, "hex query contains invalid digits"
		}
		lo, ok := viewerHexNibble(compact[i+1])
		if !ok {
			return nil, "hex query contains invalid digits"
		}
		out[i/2] = hi<<4 | lo
	}
	return out, ""
}

func viewerHexNibble(b byte) (byte, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	default:
		return 0, false
	}
}

func viewerHexFindStatus(start int64, wrapped bool) string {
	if wrapped {
		return fmt.Sprintf("0x%X (wrapped)", start)
	}
	return fmt.Sprintf("0x%X", start)
}

func searchViewerHexNext(ctx context.Context, path string, remote *paneSSHSession, pattern []byte, anchor int64, remoteSearch viewerRemoteSearchSpec) fileViewerFindResult {
	src, err := openViewerFindChunkSource(path, remote)
	if err != nil {
		return fileViewerFindResult{err: err.Error()}
	}
	defer src.Close()
	totalStarts := src.size - int64(len(pattern)) + 1
	if len(pattern) == 0 || totalStarts <= 0 {
		return fileViewerFindResult{}
	}
	if anchor < 0 {
		anchor = 0
	}
	if anchor > totalStarts {
		anchor = totalStarts
	}
	if res, used := viewerFindRemoteUtilityForward(ctx, src.size, path, remote, pattern, anchor, totalStarts, remoteSearch); used {
		return res
	}
	if off, found, err := viewerFindBytesForwardRange(ctx, src, pattern, anchor, totalStarts); err != nil {
		return fileViewerFindResult{err: err.Error()}
	} else if found {
		return fileViewerFindResult{found: true, start: off, length: int64(len(pattern))}
	}
	if anchor > 0 {
		if off, found, err := viewerFindBytesForwardRange(ctx, src, pattern, 0, anchor); err != nil {
			return fileViewerFindResult{err: err.Error()}
		} else if found {
			return fileViewerFindResult{found: true, start: off, length: int64(len(pattern)), wrapped: true}
		}
	}
	return fileViewerFindResult{}
}

func searchViewerHexAll(ctx context.Context, path string, remote *paneSSHSession, pattern []byte, remoteSearch viewerRemoteSearchSpec, limit int) ([]viewerHexFindMatch, bool, error) {
	if len(pattern) == 0 {
		return nil, false, nil
	}
	if limit < 1 {
		limit = viewerHexFindResultLimit
	}
	src, err := openViewerFindChunkSource(path, remote)
	if err != nil {
		return nil, false, err
	}
	defer src.Close()
	totalStarts := src.size - int64(len(pattern)) + 1
	if totalStarts <= 0 {
		return nil, false, nil
	}
	if viewerRemoteSearchUsable(remoteSearch, src.size) {
		if offsets, used := viewerRunRemoteSearchAll(ctx, path, remote, pattern, remoteSearch, src.size, totalStarts, limit); used {
			matches := make([]viewerHexFindMatch, 0, len(offsets))
			for _, off := range offsets {
				matches = append(matches, viewerHexFindMatch{Start: off, Length: int64(len(pattern))})
			}
			populateViewerHexFindPreviews(src, matches, 32)
			return matches, len(offsets) >= limit, nil
		}
	}
	offsets, limited, err := viewerFindBytesAll(ctx, src, pattern, totalStarts, limit)
	if err != nil {
		return nil, false, err
	}
	matches := make([]viewerHexFindMatch, 0, len(offsets))
	for _, off := range offsets {
		matches = append(matches, viewerHexFindMatch{Start: off, Length: int64(len(pattern))})
	}
	populateViewerHexFindPreviews(src, matches, 32)
	return matches, limited, nil
}

func populateViewerHexFindPreviews(src viewerFindChunkSource, matches []viewerHexFindMatch, limit int) {
	if src.read == nil || limit <= 0 {
		return
	}
	if limit > len(matches) {
		limit = len(matches)
	}
	for i := 0; i < limit; i++ {
		match := &matches[i]
		start := match.Start - viewerHexPreviewBytesPerRow
		if start < 0 {
			start = 0
		}
		end := start + int64(viewerHexPreviewBytesPerRow*viewerHexPreviewMaxRows)
		if minimum := match.Start + match.Length + viewerHexPreviewBytesPerRow; end < minimum {
			end = minimum
		}
		if end > src.size {
			end = src.size
		}
		if end <= start {
			continue
		}
		data, err := src.read(start, end-start)
		if err != nil || len(data) == 0 {
			continue
		}
		match.PreviewBytes = append(match.PreviewBytes[:0], data...)
		match.PreviewMatch = int(match.Start - start)
		compact := viewerHexCompactPreviewBytes(*match)
		match.TextPreview = viewerHexTextSnippet(compact)
		match.HexPreview = viewerHexBytesSnippet(compact)
	}
}

func redecodeViewerHexFindPreviews(st *fileViewerState) {
	if st == nil {
		return
	}
	for i := range st.find.hexMatches {
		match := &st.find.hexMatches[i]
		if len(match.PreviewBytes) == 0 {
			if data, ok := viewerHexFindContext(st, *match); ok {
				match.PreviewBytes = append([]byte(nil), data...)
			}
		}
		if len(match.PreviewBytes) == 0 {
			continue
		}
		compact := viewerHexCompactPreviewBytes(*match)
		if st.find.hexPreview {
			match.HexPreview = viewerHexBytesSnippet(compact)
		} else {
			match.TextPreview = viewerHexTextSnippet(compact)
		}
	}
}

func viewerFindBytesAll(ctx context.Context, src viewerFindChunkSource, pattern []byte, totalStarts int64, limit int) ([]int64, bool, error) {
	if len(pattern) == 0 || totalStarts <= 0 || limit < 1 {
		return nil, false, nil
	}
	chunkBytes := int64(viewerFindChunkBytes)
	if chunkBytes < int64(len(pattern)) {
		chunkBytes = int64(len(pattern))
	}
	offsets := make([]int64, 0, min(limit, 32))
	for pos := int64(0); pos < totalStarts; pos += chunkBytes {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		rangeEnd := pos + chunkBytes
		if rangeEnd > totalStarts {
			rangeEnd = totalStarts
		}
		readLen := rangeEnd - pos + int64(len(pattern)) - 1
		data, err := src.read(pos, readLen)
		if err != nil {
			return nil, false, err
		}
		for cursor := 0; cursor+len(pattern) <= len(data); {
			idx := bytes.Index(data[cursor:], pattern)
			if idx < 0 {
				break
			}
			idx += cursor
			off := pos + int64(idx)
			if off >= rangeEnd {
				break
			}
			offsets = append(offsets, off)
			if len(offsets) >= limit {
				return offsets, off+1 < totalStarts, nil
			}
			cursor = idx + 1
		}
	}
	return offsets, false, nil
}

func viewerRunRemoteSearchAll(ctx context.Context, path string, remote *paneSSHSession, pattern []byte, spec viewerRemoteSearchSpec, rangeEnd, totalStarts int64, limit int) ([]int64, bool) {
	if remote == nil || strings.TrimSpace(spec.template) == "" || rangeEnd <= 0 || totalStarts <= 0 {
		return nil, false
	}
	cmdline := expandViewerRemoteSearchTemplate(spec.template, viewerRemoteSearchTemplateArgs{
		fullpath:     path,
		filename:     viewerCommandMatchName(path, remote),
		patternText:  string(pattern),
		patternBytes: append([]byte(nil), pattern...),
		rangeStart:   0,
		rangeEnd:     rangeEnd,
		direction:    "next",
		matchLimit:   "",
		resultSelect: "head -n " + strconv.Itoa(limit),
	}, spec.shell.quoteFn)
	content, err := runViewerRemoteSearchCommandFunc(ctx, remote, cmdline, spec.shell)
	if err != nil {
		return nil, false
	}
	offsets, valid := viewerRemoteSearchOffsets(content, totalStarts, limit)
	if !valid {
		return nil, false
	}
	return offsets, true
}

func viewerRemoteSearchOffsets(raw string, end int64, limit int) ([]int64, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	offsets := make([]int64, 0, min(limit, 32))
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		digitEnd := 0
		for digitEnd < len(line) && line[digitEnd] >= '0' && line[digitEnd] <= '9' {
			digitEnd++
		}
		if digitEnd == 0 {
			return nil, false
		}
		off, err := strconv.ParseInt(line[:digitEnd], 10, 64)
		if err != nil || off < 0 || off >= end {
			return nil, false
		}
		if len(offsets) == 0 || offsets[len(offsets)-1] != off {
			offsets = append(offsets, off)
		}
		if len(offsets) >= limit {
			break
		}
	}
	sort.Slice(offsets, func(i, j int) bool { return offsets[i] < offsets[j] })
	unique := offsets[:0]
	for _, off := range offsets {
		if len(unique) == 0 || unique[len(unique)-1] != off {
			unique = append(unique, off)
		}
	}
	return unique, true
}

func searchViewerHexPrev(ctx context.Context, path string, remote *paneSSHSession, pattern []byte, anchor, selfStart int64, remoteSearch viewerRemoteSearchSpec) fileViewerFindResult {
	src, err := openViewerFindChunkSource(path, remote)
	if err != nil {
		return fileViewerFindResult{err: err.Error()}
	}
	defer src.Close()
	totalStarts := src.size - int64(len(pattern)) + 1
	if len(pattern) == 0 || totalStarts <= 0 {
		return fileViewerFindResult{}
	}
	if anchor < 0 {
		anchor = 0
	}
	if anchor > totalStarts {
		anchor = totalStarts
	}
	if res, used := viewerFindRemoteUtilityBackward(ctx, src, path, remote, pattern, anchor, selfStart, totalStarts, remoteSearch); used {
		return res
	}
	if off, found, err := viewerFindBytesBackwardRange(ctx, src, pattern, 0, anchor); err != nil {
		return fileViewerFindResult{err: err.Error()}
	} else if found {
		return fileViewerFindResult{found: true, start: off, length: int64(len(pattern))}
	}
	secondaryStart := anchor
	if selfStart >= 0 && selfStart+1 > secondaryStart {
		secondaryStart = selfStart + 1
	}
	if secondaryStart < totalStarts {
		if off, found, err := viewerFindBytesBackwardRange(ctx, src, pattern, secondaryStart, totalStarts); err != nil {
			return fileViewerFindResult{err: err.Error()}
		} else if found {
			return fileViewerFindResult{found: true, start: off, length: int64(len(pattern)), wrapped: true}
		}
	}
	if selfStart >= 0 && selfStart < totalStarts {
		if same, err := viewerFindExactRange(src, pattern, selfStart); err != nil {
			return fileViewerFindResult{err: err.Error()}
		} else if same {
			return fileViewerFindResult{found: true, start: selfStart, length: int64(len(pattern)), wrapped: true}
		}
	}
	return fileViewerFindResult{}
}

func viewerFindRemoteUtilityForward(ctx context.Context, size int64, path string, remote *paneSSHSession, pattern []byte, anchor, totalStarts int64, spec viewerRemoteSearchSpec) (fileViewerFindResult, bool) {
	if !viewerRemoteSearchUsable(spec, size) {
		return fileViewerFindResult{}, false
	}
	if off, found, ok := viewerRunRemoteSearchRange(ctx, path, remote, pattern, spec, anchor, totalStarts, false); ok {
		if found {
			return fileViewerFindResult{found: true, start: off, length: int64(len(pattern))}, true
		}
	} else {
		return fileViewerFindResult{}, false
	}
	if anchor > 0 {
		if off, found, ok := viewerRunRemoteSearchRange(ctx, path, remote, pattern, spec, 0, anchor, false); ok {
			if found {
				return fileViewerFindResult{found: true, start: off, length: int64(len(pattern)), wrapped: true}, true
			}
		} else {
			return fileViewerFindResult{}, false
		}
	}
	return fileViewerFindResult{}, true
}

func viewerFindRemoteUtilityBackward(ctx context.Context, src viewerFindChunkSource, path string, remote *paneSSHSession, pattern []byte, anchor, selfStart, totalStarts int64, spec viewerRemoteSearchSpec) (fileViewerFindResult, bool) {
	if !viewerRemoteSearchUsable(spec, src.size) {
		return fileViewerFindResult{}, false
	}
	if off, found, ok := viewerRunRemoteSearchRange(ctx, path, remote, pattern, spec, 0, anchor, true); ok {
		if found {
			return fileViewerFindResult{found: true, start: off, length: int64(len(pattern))}, true
		}
	} else {
		return fileViewerFindResult{}, false
	}
	secondaryStart := anchor
	if selfStart >= 0 && selfStart+1 > secondaryStart {
		secondaryStart = selfStart + 1
	}
	if secondaryStart < totalStarts {
		if off, found, ok := viewerRunRemoteSearchRange(ctx, path, remote, pattern, spec, secondaryStart, totalStarts, true); ok {
			if found {
				return fileViewerFindResult{found: true, start: off, length: int64(len(pattern)), wrapped: true}, true
			}
		} else {
			return fileViewerFindResult{}, false
		}
	}
	if selfStart >= 0 && selfStart < totalStarts {
		if same, err := viewerFindExactRange(src, pattern, selfStart); err != nil {
			return fileViewerFindResult{}, false
		} else if same {
			return fileViewerFindResult{found: true, start: selfStart, length: int64(len(pattern)), wrapped: true}, true
		}
	}
	return fileViewerFindResult{}, true
}

func viewerRemoteSearchUsable(spec viewerRemoteSearchSpec, size int64) bool {
	_ = size
	if spec.template == "" {
		return false
	}
	switch fm.NormalizeViewerRemoteSearchMode(spec.mode) {
	case fm.ViewerRemoteSearchModeLocal:
		return false
	case fm.ViewerRemoteSearchModeRemote:
	default:
		return false
	}
	if spec.hexInput && !viewerRemoteSearchTemplateSupportsHex(spec.template) {
		return false
	}
	return true
}

func viewerRemoteSearchTemplateSupportsHex(template string) bool {
	if strings.TrimSpace(template) == "" {
		return false
	}
	return strings.Contains(template, "{pattern_hex}") ||
		strings.Contains(template, "{pattern_hex_raw}") ||
		strings.Contains(template, "{pattern_base64}") ||
		strings.Contains(template, "{pattern_base64_raw}")
}

func viewerRunRemoteSearchRange(ctx context.Context, path string, remote *paneSSHSession, pattern []byte, spec viewerRemoteSearchSpec, start, end int64, pickLast bool) (int64, bool, bool) {
	if remote == nil || strings.TrimSpace(spec.template) == "" {
		return 0, false, false
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if start >= end {
		return 0, false, true
	}
	direction := "next"
	resultSelect := "head -n 1"
	matchLimit := "-m 1"
	if pickLast {
		direction = "prev"
		resultSelect = "tail -n 1"
		matchLimit = ""
	}
	cmdline := expandViewerRemoteSearchTemplate(spec.template, viewerRemoteSearchTemplateArgs{
		fullpath:     path,
		filename:     viewerCommandMatchName(path, remote),
		patternText:  string(pattern),
		patternBytes: append([]byte(nil), pattern...),
		rangeStart:   start,
		rangeEnd:     end,
		direction:    direction,
		matchLimit:   matchLimit,
		resultSelect: resultSelect,
	}, spec.shell.quoteFn)
	content, err := runViewerRemoteSearchCommandFunc(ctx, remote, cmdline, spec.shell)
	if err != nil {
		return 0, false, false
	}
	rel, found := viewerRemoteSearchOffset(content)
	if !found {
		if strings.TrimSpace(content) != "" {
			return 0, false, false
		}
		return 0, false, true
	}
	off := start + rel
	if off < start || off >= end {
		return 0, false, false
	}
	return off, true, true
}

func viewerRemoteSearchOffset(raw string) (int64, bool) {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		end := 0
		for end < len(line) && line[end] >= '0' && line[end] <= '9' {
			end++
		}
		if end == 0 {
			continue
		}
		off, err := strconv.ParseInt(line[:end], 10, 64)
		if err != nil || off < 0 {
			continue
		}
		return off, true
	}
	return 0, false
}

func expandViewerRemoteSearchTemplate(template string, args viewerRemoteSearchTemplateArgs, quoteFn func(string) string) string {
	cmdline := strings.TrimSpace(template)
	if quoteFn == nil {
		quoteFn = shellQuote
	}
	for _, placeholder := range []string{
		"{fullpath}",
		"{path}",
		"{filename}",
		"{pattern}",
		"{pattern_hex}",
		"{pattern_base64}",
	} {
		cmdline = collapseQuotedViewerPlaceholder(cmdline, placeholder)
	}
	rangeLen := args.rangeEnd - args.rangeStart
	if rangeLen < 0 {
		rangeLen = 0
	}
	patternHex := hex.EncodeToString(args.patternBytes)
	patternBase64 := base64.StdEncoding.EncodeToString(args.patternBytes)
	replacements := []string{
		"{fullpath_raw}", args.fullpath,
		"{path_raw}", args.fullpath,
		"{filename_raw}", args.filename,
		"{pattern_raw}", args.patternText,
		"{pattern_hex_raw}", patternHex,
		"{pattern_base64_raw}", patternBase64,
		"{range_start}", strconv.FormatInt(args.rangeStart, 10),
		"{range_start_1based}", strconv.FormatInt(args.rangeStart+1, 10),
		"{range_start1}", strconv.FormatInt(args.rangeStart+1, 10),
		"{range_end}", strconv.FormatInt(args.rangeEnd, 10),
		"{range_len}", strconv.FormatInt(rangeLen, 10),
		"{direction}", args.direction,
		"{match_limit}", args.matchLimit,
		"{result_select}", args.resultSelect,
		"{pattern}", quoteFn(args.patternText),
		"{pattern_hex}", quoteFn(patternHex),
		"{pattern_base64}", quoteFn(patternBase64),
		"{fullpath}", quoteFn(args.fullpath),
		"{path}", quoteFn(args.fullpath),
		"{filename}", quoteFn(args.filename),
	}
	return strings.NewReplacer(replacements...).Replace(cmdline)
}

func openViewerFindChunkSource(path string, remote *paneSSHSession) (viewerFindChunkSource, error) {
	if remote != nil {
		client := remote.sftpClient()
		if client == nil {
			return viewerFindChunkSource{}, fmt.Errorf("sftp session is not connected")
		}
		info, err := client.Stat(path)
		if err != nil {
			return viewerFindChunkSource{}, err
		}
		if info.IsDir() {
			return viewerFindChunkSource{}, fmt.Errorf("viewer supports files only")
		}
		file, err := client.Open(path)
		if err != nil {
			return viewerFindChunkSource{}, err
		}
		ra, ok := any(file).(interface {
			ReadAt([]byte, int64) (int, error)
			Close() error
		})
		if !ok {
			_ = file.Close()
			return viewerFindChunkSource{}, fmt.Errorf("remote viewer find needs random-access reads")
		}
		size := info.Size()
		return viewerFindChunkSource{
			size: size,
			read: func(start, length int64) ([]byte, error) {
				return viewerReadChunkFromRA(ra, size, start, length)
			},
			close: ra.Close,
		}, nil
	}

	reader, info, err := filesys.OpenLocalPath(path)
	if err != nil {
		return viewerFindChunkSource{}, err
	}
	if info.IsDir() {
		_ = reader.Close()
		return viewerFindChunkSource{}, fmt.Errorf("viewer supports files only")
	}
	size := info.Size()
	if ra, ok := reader.(interface {
		ReadAt([]byte, int64) (int, error)
		Close() error
	}); ok {
		return viewerFindChunkSource{
			size: size,
			read: func(start, length int64) ([]byte, error) {
				return viewerReadChunkFromRA(ra, size, start, length)
			},
			close: ra.Close,
		}, nil
	}
	_ = reader.Close()
	return viewerFindChunkSource{
		size: size,
		read: func(start, length int64) ([]byte, error) {
			chunk, _, err := filesys.ReadLocalFileChunk(path, start, length)
			return chunk, err
		},
	}, nil
}

func viewerReadChunkFromRA(ra io.ReaderAt, size, start, length int64) ([]byte, error) {
	if start < 0 {
		start = 0
	}
	if start > size {
		start = size
	}
	if length < 0 {
		length = 0
	}
	if start+length > size {
		length = size - start
	}
	if length <= 0 {
		return nil, nil
	}
	buf := make([]byte, length)
	n, err := ra.ReadAt(buf, start)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf[:n], nil
}

func viewerFindBytesForwardRange(ctx context.Context, src viewerFindChunkSource, pattern []byte, start, end int64) (int64, bool, error) {
	if len(pattern) == 0 || src.size <= 0 || start >= end {
		return 0, false, nil
	}
	if start < 0 {
		start = 0
	}
	totalStarts := src.size - int64(len(pattern)) + 1
	if totalStarts < 1 {
		return 0, false, nil
	}
	if end > totalStarts {
		end = totalStarts
	}
	if start >= end {
		return 0, false, nil
	}
	chunkBytes := int64(viewerFindChunkBytes)
	if chunkBytes < int64(len(pattern)) {
		chunkBytes = int64(len(pattern))
	}
	for pos := start; pos < end; pos += chunkBytes {
		if err := ctx.Err(); err != nil {
			return 0, false, err
		}
		readLen := chunkBytes + int64(len(pattern)) - 1
		data, err := src.read(pos, readLen)
		if err != nil {
			return 0, false, err
		}
		if len(data) == 0 {
			break
		}
		searchLen := int(end - pos + int64(len(pattern)) - 1)
		if searchLen > len(data) {
			searchLen = len(data)
		}
		if searchLen < len(pattern) {
			searchLen = len(data)
		}
		if idx := bytes.Index(data[:searchLen], pattern); idx >= 0 {
			off := pos + int64(idx)
			if off < end {
				return off, true, nil
			}
		}
	}
	return 0, false, nil
}

func viewerFindBytesBackwardRange(ctx context.Context, src viewerFindChunkSource, pattern []byte, start, end int64) (int64, bool, error) {
	if len(pattern) == 0 || src.size <= 0 || start >= end {
		return 0, false, nil
	}
	if start < 0 {
		start = 0
	}
	totalStarts := src.size - int64(len(pattern)) + 1
	if totalStarts < 1 {
		return 0, false, nil
	}
	if end > totalStarts {
		end = totalStarts
	}
	if start >= end {
		return 0, false, nil
	}
	chunkBytes := int64(viewerFindChunkBytes)
	if chunkBytes < int64(len(pattern)) {
		chunkBytes = int64(len(pattern))
	}
	for pos := end; pos > start; {
		if err := ctx.Err(); err != nil {
			return 0, false, err
		}
		chunkStart := pos - chunkBytes
		if chunkStart < start {
			chunkStart = start
		}
		readLen := pos - chunkStart + int64(len(pattern)) - 1
		data, err := src.read(chunkStart, readLen)
		if err != nil {
			return 0, false, err
		}
		if len(data) == 0 {
			break
		}
		searchLen := int(pos - chunkStart + int64(len(pattern)) - 1)
		if searchLen > len(data) {
			searchLen = len(data)
		}
		if idx := bytes.LastIndex(data[:searchLen], pattern); idx >= 0 {
			off := chunkStart + int64(idx)
			if off >= start && off < end {
				return off, true, nil
			}
		}
		pos = chunkStart
	}
	return 0, false, nil
}

func viewerFindExactRange(src viewerFindChunkSource, pattern []byte, off int64) (bool, error) {
	if len(pattern) == 0 || off < 0 || off+int64(len(pattern)) > src.size {
		return false, nil
	}
	data, err := src.read(off, int64(len(pattern)))
	if err != nil {
		return false, err
	}
	return len(data) == len(pattern) && bytes.Equal(data, pattern), nil
}

func fileViewerFindColsForLine(st *fileViewerState, line int) (int, int, bool) {
	if st == nil || !st.find.open || !st.find.currentValid || st.find.currentLen <= 0 {
		return 0, 0, false
	}
	start := int(st.find.currentStart)
	end := start + int(st.find.currentLen)
	return st.stream.rangeColsForLine(line, start, end)
}

func fileViewerFindHighlightColors(theme fileViewerTheme) (color.NRGBA, color.NRGBA) {
	base := mixNRGBA(theme.PanelBg, color.NRGBA{R: 168, G: 128, B: 16, A: 255}, 0.78)
	base.A = 224
	strong := mixNRGBA(theme.PanelBg, color.NRGBA{R: 204, G: 164, B: 28, A: 255}, 0.88)
	strong.A = 240
	return base, strong
}

func tinyIconModeButtonWidth(gtx layout.Context) int {
	return gtx.Dp(unit.Dp(12)) + gtx.Dp(unit.Dp(8))
}

func (ui *UI) fileViewerFindSourceChipWidth(th *material.Theme, gtx layout.Context, st *fileViewerState) int {
	if ui == nil || th == nil || st == nil || !ui.fileViewerFindRemoteSearchConfigured(st) {
		return 0
	}
	lbl := material.Body2(th, ui.fileViewerFindSourceLabel(st)+" ▾")
	lbl.Font.Typeface = ui.viewerTypeface()
	lbl.Font.Weight = font.Medium
	lbl.TextSize = scaleThemeFontSize(th, 10)
	lbl.MaxLines = 1
	return measureLabelUnconstrained(gtx, lbl).Size.X + gtx.Dp(unit.Dp(12))
}

func (ui *UI) fileViewerFindEditorWidths(th *material.Theme, gtx layout.Context) (desired int, minimum int) {
	advance := measureStreamCharAdvance(ui, th, gtx)
	padding := gtx.Dp(unit.Dp(10))
	desired = int(float32(viewerFindFieldChars)*advance + 0.5)
	minimum = int(float32(viewerFindFieldMinChars)*advance + 0.5)
	desired += padding
	minimum += padding
	if minimum < gtx.Dp(unit.Dp(120)) {
		minimum = gtx.Dp(unit.Dp(120))
	}
	if desired < minimum {
		desired = minimum
	}
	return desired, minimum
}

func (ui *UI) fileViewerFindStatusWidth(th *material.Theme, gtx layout.Context) int {
	if ui == nil || th == nil {
		return 0
	}
	samples := []string{
		"9999/9999",
		"Searching...",
		"No matches",
	}
	w := 0
	for _, sample := range samples {
		lbl := material.Body2(th, sample)
		lbl.Font.Typeface = ui.viewerTypeface()
		lbl.TextSize = scaleThemeFontSize(th, 10)
		lbl.MaxLines = 1
		lbl.Truncator = "..."
		if measured := measureLabelUnconstrained(gtx, lbl).Size.X + gtx.Dp(unit.Dp(2)); measured > w {
			w = measured
		}
	}
	maxW := gtx.Dp(unit.Dp(viewerFindStatusMaxDp))
	if w > maxW {
		w = maxW
	}
	return w
}

func (ui *UI) fileViewerFindBarWidths(th *material.Theme, gtx layout.Context, st *fileViewerState, now time.Time) (barW, editorW int) {
	editorDesiredW, editorMinW := ui.fileViewerFindEditorWidths(th, gtx)
	if cap := gtx.Dp(unit.Dp(164)); editorDesiredW > cap {
		editorDesiredW = cap
	}
	if cap := gtx.Dp(unit.Dp(112)); editorMinW > cap {
		editorMinW = cap
	}
	statusW := ui.fileViewerFindStatusWidth(th, gtx)
	reserved := gtx.Dp(unit.Dp(16))
	if sourceW := ui.fileViewerFindSourceChipWidth(th, gtx, st); sourceW > 0 {
		reserved += sourceW + gtx.Dp(unit.Dp(6))
	}
	if st != nil && st.mode == "hex" {
		// One square mode glyph on each side of the editor plus the two tight
		// gaps between glyphs and the field.
		reserved += 2*gtx.Dp(unit.Dp(viewerFindBarRowHeightDp)) + gtx.Dp(unit.Dp(8))
	}
	findLbl := material.Body2(th, "Find")
	findLbl.Font.Typeface = ui.viewerTypeface()
	findLbl.Font.Weight = font.Medium
	findLbl.TextSize = ui.viewerTextSize()
	findLbl.MaxLines = 1
	reserved += measureLabelUnconstrained(gtx, findLbl).Size.X
	reserved += gtx.Dp(unit.Dp(6))
	reserved += gtx.Dp(unit.Dp(6))
	reserved += tinyIconModeButtonWidth(gtx)
	reserved += gtx.Dp(unit.Dp(4))
	reserved += tinyIconModeButtonWidth(gtx)
	reserved += gtx.Dp(unit.Dp(6))
	reserved += statusW
	reserved += gtx.Dp(unit.Dp(6))
	reserved += tinyIconModeButtonWidth(gtx)

	available := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(viewerFindBarInsetDp*2))
	if available < 1 {
		available = 1
	}
	maxEditorW := available - reserved
	if maxEditorW < 1 {
		maxEditorW = 1
	}
	editorW = editorDesiredW
	if editorW > maxEditorW {
		editorW = maxEditorW
	}
	if editorW < editorMinW && maxEditorW >= editorMinW {
		editorW = editorMinW
	}
	if editorW < 1 {
		editorW = 1
	}
	barW = reserved + editorW
	if barW > available {
		barW = available
	}
	if barW < 1 {
		barW = 1
	}
	return barW, editorW
}

func (ui *UI) layoutFileViewerFindBar(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	if st == nil || !st.find.open || !viewerSupportsFind(st) {
		return layout.Dimensions{}
	}
	theme := ui.fileViewerTheme()
	bg := mixNRGBA(theme.PanelBg, theme.HeaderBg, 0.24)
	bg.A = 244
	border := mixNRGBA(theme.PanelBorder, theme.Divider, 0.42)
	border.A = 180
	innerOrigin := image.Pt(gtx.Dp(unit.Dp(8)), gtx.Dp(unit.Dp(4)))
	statusW := ui.fileViewerFindStatusWidth(th, gtx)
	barW, editorW := ui.fileViewerFindBarWidths(th, gtx, st, gtx.Now)
	st.find.sourceButtonRect = image.Rectangle{}
	st.find.findByButtonRect = image.Rectangle{}
	st.find.previewButtonRect = image.Rectangle{}
	if st.mode == "hex" {
		buttonSize := gtx.Dp(unit.Dp(viewerFindBarRowHeightDp))
		x := innerOrigin.X
		if sourceW := ui.fileViewerFindSourceChipWidth(th, gtx, st); sourceW > 0 {
			x += sourceW + gtx.Dp(unit.Dp(6))
		}
		findLabel := material.Body2(th, "Find")
		findLabel.Font.Typeface = ui.viewerTypeface()
		findLabel.Font.Weight = font.Medium
		findLabel.TextSize = scaleThemeFontSize(th, 10)
		findLabel.MaxLines = 1
		x += measureLabelUnconstrained(gtx, findLabel).Size.X + gtx.Dp(unit.Dp(6))
		st.find.findByButtonRect = image.Rect(x, innerOrigin.Y, x+buttonSize, innerOrigin.Y+buttonSize)
		x += buttonSize + gtx.Dp(unit.Dp(4)) + editorW + gtx.Dp(unit.Dp(4))
		st.find.previewButtonRect = image.Rect(x, innerOrigin.Y, x+buttonSize, innerOrigin.Y+buttonSize)
	}
	bar := op.Record(gtx.Ops)
	barDims := fixedWidth(gtx, barW, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedClipBox(
			gtx,
			0,
			bg,
			border,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, gtx.Dp(unit.Dp(viewerFindBarRowHeightDp)), func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if !ui.fileViewerFindRemoteSearchConfigured(st) {
									st.find.sourceButtonRect = image.Rectangle{}
									st.find.closeSourceMenu()
									return layout.Dimensions{}
								}
								dims := ui.layoutFileViewerFindSourceSelect(th, gtx, st)
								st.find.sourceButtonRect = image.Rectangle{Min: innerOrigin, Max: innerOrigin.Add(dims.Size)}
								return dims
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if !ui.fileViewerFindRemoteSearchConfigured(st) {
									return layout.Dimensions{}
								}
								return layout.Spacer{Width: unit.Dp(6)}.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, "Find")
								lbl.Font.Typeface = ui.viewerTypeface()
								lbl.Font.Weight = font.Medium
								lbl.TextSize = scaleThemeFontSize(th, 10)
								lbl.Color = theme.HeaderText
								lbl.MaxLines = 1
								return layoutVCenteredLabel(gtx, lbl)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if st.mode != "hex" {
									return layout.Dimensions{}
								}
								return ui.layoutFileViewerFindModeButton(gtx, &st.find.findByClick, st.find.hexInput)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if st.mode != "hex" {
									return layout.Dimensions{}
								}
								return layout.Spacer{Width: unit.Dp(4)}.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, editorW, func(gtx layout.Context) layout.Dimensions {
									return ui.layoutEditorWithContextMenu(th, gtx, "viewer-find", &st.find.editor, true, func(gtx layout.Context) layout.Dimensions {
										return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											gtx.Constraints.Min.X = gtx.Constraints.Max.X
											ed := material.Editor(th, &st.find.editor, ui.fileViewerFindPlaceholder(st))
											ed.Font.Typeface = ui.viewerTypeface()
											ed.TextSize = ui.viewerTextSize()
											ed.Color = theme.CommandText
											ed.HintColor = theme.CommandHint
											focused := st.find.focus || gtx.Focused(&st.find.editor)
											return layoutNeutralEditorBox(gtx, focused, true, func(gtx layout.Context) layout.Dimensions {
												return layout.Inset{Left: unit.Dp(2), Right: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
													gtx.Constraints.Min.X = gtx.Constraints.Max.X
													return ed.Layout(gtx)
												})
											})
										})
									})
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if st.mode != "hex" {
									return layout.Dimensions{}
								}
								return layout.Spacer{Width: unit.Dp(4)}.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if st.mode != "hex" {
									return layout.Dimensions{}
								}
								return ui.layoutFileViewerFindModeButton(gtx, &st.find.previewClick, st.find.hexPreview)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutTinyIconModeButton(th, gtx, &st.find.prevClick, uitheme.ArrowUpIcon(), false)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutTinyIconModeButton(th, gtx, &st.find.nextClick, uitheme.ArrowDownIcon(), false)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Spacer{Width: unit.Dp(6)}.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, statusW, func(gtx layout.Context) layout.Dimensions {
									return fixedHeight(gtx, gtx.Dp(unit.Dp(viewerFindBarRowHeightDp)), func(gtx layout.Context) layout.Dimensions {
										return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											lbl := material.Body2(th, ui.fileViewerFindStatusText(st, gtx.Now))
											lbl.Font.Typeface = ui.viewerTypeface()
											lbl.TextSize = scaleThemeFontSize(th, 10)
											lbl.Color = ui.fileViewerFindStatusColor(st, gtx.Now)
											lbl.MaxLines = 1
											lbl.Truncator = "..."
											return layoutVCenteredLabel(gtx, lbl)
										})
									})
								})
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutFlatCloseButton(gtx, &st.find.closeClick, false)
							}),
						)
					})
				})
			},
		)
	})
	barCall := bar.Stop()
	if barDims.Size.X <= 0 || barDims.Size.Y <= 0 {
		st.find.sourceButtonRect = image.Rectangle{}
		st.find.findByButtonRect = image.Rectangle{}
		st.find.previewButtonRect = image.Rectangle{}
		st.find.sourceMenuRect = image.Rectangle{}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	barPos := image.Pt(
		gtx.Constraints.Max.X-barDims.Size.X-gtx.Dp(unit.Dp(viewerFindBarInsetDp)),
		0,
	)
	if barPos.X < 0 {
		barPos.X = 0
	}
	if barPos.Y < 0 {
		barPos.Y = 0
	}
	if st.find.sourceButtonRect.Dx() > 0 && st.find.sourceButtonRect.Dy() > 0 {
		st.find.sourceButtonRect = st.find.sourceButtonRect.Add(barPos)
	}
	if !st.find.findByButtonRect.Empty() {
		st.find.findByButtonRect = st.find.findByButtonRect.Add(barPos)
	}
	if !st.find.previewButtonRect.Empty() {
		st.find.previewButtonRect = st.find.previewButtonRect.Add(barPos)
	}
	offset := op.Offset(barPos).Push(gtx.Ops)
	barCall.Add(gtx.Ops)
	offset.Pop()

	if st.find.sourceMenuOpen && ui.fileViewerFindRemoteSearchConfigured(st) {
		alpha, slideY, animating := popupOpenProgress(gtx.Now, st.find.sourceMenuAt)
		if animating {
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
		}
		menu := op.Record(gtx.Ops)
		menuDims := ui.layoutFileViewerFindSourceMenu(th, gtx, st, alpha)
		menuCall := menu.Stop()
		menuPos := image.Pt(st.find.sourceButtonRect.Min.X, st.find.sourceButtonRect.Max.Y+gtx.Dp(unit.Dp(4))+slideY)
		menuPos = clampFilePaneMenuPoint(menuPos, menuDims.Size, gtx.Constraints.Max)
		st.find.sourceMenuRect = image.Rectangle{Min: menuPos, Max: menuPos.Add(menuDims.Size)}
		offset = op.Offset(menuPos).Push(gtx.Ops)
		menuCall.Add(gtx.Ops)
		offset.Pop()
	} else {
		st.find.sourceMenuRect = image.Rectangle{}
	}
	if !st.find.sourceMenuOpen && (fileViewerFindResultCount(st) > 0 || viewerFindPendingPanel(st)) {
		panelW := barDims.Size.X
		panel := op.Record(gtx.Ops)
		ui.layoutFileViewerFindResults(th, gtx, st, panelW)
		panelCall := panel.Stop()
		panelPos := image.Pt(
			barPos.X,
			barPos.Y+barDims.Size.Y,
		)
		if panelPos.X < 0 {
			panelPos.X = 0
		}
		offset = op.Offset(panelPos).Push(gtx.Ops)
		panelCall.Add(gtx.Ops)
		offset.Pop()
	}
	ui.deferFileViewerFindModeHint(th, gtx, st)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *UI) layoutFileViewerFindModeButton(gtx layout.Context, click *widget.Clickable, hexMode bool) layout.Dimensions {
	if click == nil {
		return layout.Dimensions{}
	}
	theme := ui.fileViewerTheme()
	size := gtx.Dp(unit.Dp(viewerFindBarRowHeightDp))
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return fixedWidth(gtx, size, func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, size, func(gtx layout.Context) layout.Dimensions {
				bg := mixNRGBA(theme.CommandBg, theme.TooltipBg, 0.28)
				border := scaleColorAlpha(theme.CommandBorder, 0.68)
				fg := theme.CommandText
				if click.Hovered() {
					bg = mixNRGBA(theme.CommandBgHover, theme.TooltipBg, 0.24)
					border = scaleColorAlpha(theme.CommandBorderHover, 0.84)
					fg = theme.HeaderText
				}
				return fillFlatBox(gtx, bg, border, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						glyphGTX := gtx
						glyphGTX.Constraints = layout.Exact(image.Pt(gtx.Dp(unit.Dp(14)), gtx.Dp(unit.Dp(10))))
						return layoutFileViewerFindModeGlyph(glyphGTX, hexMode, fg)
					})
				})
			})
		})
	})
}

func layoutFileViewerFindModeGlyph(gtx layout.Context, hexMode bool, fg color.NRGBA) layout.Dimensions {
	size := gtx.Constraints.Min
	if size.X < 8 || size.Y < 6 {
		return layout.Dimensions{Size: size}
	}
	rowH := 2
	top := (size.Y - (3*rowH + 2)) / 2
	if top < 0 {
		top = 0
	}
	draw := func(x, y, width int) {
		if width > 0 {
			paint.FillShape(gtx.Ops, fg, clip.Rect(image.Rect(x, y, x+width, y+rowH)).Op())
		}
	}
	for row := 0; row < 3; row++ {
		y := top + row*(rowH+1)
		if hexMode {
			cellW := 2
			gap := 1
			groupGap := 2
			left := (size.X - (4*cellW + 2*gap + groupGap)) / 2
			draw(left, y, cellW)
			draw(left+cellW+gap, y, cellW)
			right := left + 2*cellW + gap + groupGap
			draw(right, y, cellW)
			draw(right+cellW+gap, y, cellW)
			continue
		}
		width := size.X - 2
		if row == 1 {
			width -= 3
		}
		draw(1, y, width)
	}
	return layout.Dimensions{Size: size}
}

func (ui *UI) deferFileViewerFindModeHint(th *material.Theme, gtx layout.Context, st *fileViewerState) {
	if st == nil || st.mode != "hex" {
		return
	}
	tip := ""
	anchor := image.Rectangle{}
	if st.find.findByClick.Hovered() {
		anchor = st.find.findByButtonRect
		if st.find.hexInput {
			tip = "Search as hex"
		} else {
			tip = "Search as text"
		}
	} else if st.find.previewClick.Hovered() {
		anchor = st.find.previewButtonRect
		if st.find.hexPreview {
			tip = "Preview as hex"
		} else {
			tip = "Preview as text"
		}
	}
	if tip == "" || anchor.Empty() {
		return
	}
	tipGTX := gtx
	tipGTX.Constraints.Min = image.Point{}
	tipGTX.Constraints.Max = image.Pt(gtx.Dp(unit.Dp(140)), gtx.Dp(unit.Dp(24)))
	recorded := op.Record(gtx.Ops)
	theme := ui.fileViewerTheme()
	tipDims := fillFlatBox(tipGTX, theme.TooltipBg, theme.TooltipBorder, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, tip)
			lbl.Font.Typeface = ui.viewerTypeface()
			lbl.TextSize = scaleThemeFontSize(th, 9)
			lbl.Color = theme.TooltipText
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		})
	})
	tipCall := recorded.Stop()
	pos := viewerFindHintPoint(anchor, tipDims.Size, gtx.Constraints.Max, gtx.Dp(unit.Dp(2)))
	deferred := op.Record(gtx.Ops)
	offset := op.Offset(pos).Push(gtx.Ops)
	tipCall.Add(gtx.Ops)
	offset.Pop()
	op.Defer(gtx.Ops, deferred.Stop())
}

func viewerFindHintPoint(anchor image.Rectangle, tipSize, viewport image.Point, gap int) image.Point {
	pos := image.Pt(anchor.Min.X+(anchor.Dx()-tipSize.X)/2, anchor.Max.Y+gap)
	return clampFilePaneMenuPoint(pos, tipSize, viewport)
}

func fileViewerFindResultCount(st *fileViewerState) int {
	if st == nil {
		return 0
	}
	if viewerPDFPreviewActive(st) {
		return len(st.find.pdfMatches)
	}
	if st.mode == "hex" {
		return len(st.find.hexMatches)
	}
	return len(st.find.matches)
}

func viewerFindPendingPanel(st *fileViewerState) bool {
	return st != nil && st.mode == "hex" && st.find.open && st.find.searching && strings.TrimSpace(st.find.editor.Text()) != ""
}

func (ui *UI) layoutFileViewerFindResults(th *material.Theme, gtx layout.Context, st *fileViewerState, width int) layout.Dimensions {
	count := fileViewerFindResultCount(st)
	if st == nil || width <= 0 || (count == 0 && !viewerFindPendingPanel(st)) {
		return layout.Dimensions{}
	}
	if hovered := fileViewerFindHoveredIndex(st); hovered >= 0 {
		st.find.previewIndex = hovered
		st.find.previewAt = gtx.Now
	} else if st.find.previewIndex >= 0 && !st.find.previewAt.IsZero() {
		const hoverBridge = 140 * time.Millisecond
		expires := st.find.previewAt.Add(hoverBridge)
		if !gtx.Now.Before(expires) {
			st.find.previewIndex = -1
			st.find.previewAt = time.Time{}
		} else {
			gtx.Execute(op.InvalidateCmd{At: expires})
		}
	}
	cursorIndex := st.find.index
	if st.find.previewIndex >= 0 {
		cursorIndex = st.find.previewIndex
	}
	st.find.cursorAnim.setTarget(gtx.Now, cursorIndex)
	preview := ui.fileViewerFindPreviewForIndex(st, st.find.previewIndex)
	listState := &st.find.textList
	if viewerPDFPreviewActive(st) {
		listState = &st.find.pdfList
	} else if st.mode == "hex" {
		listState = &st.find.hexList
	}
	theme := ui.fileViewerTheme()
	bg := mixNRGBA(theme.PanelBg, theme.HeaderBg, 0.18)
	bg.A = 248
	border := mixNRGBA(theme.Divider, theme.HeaderText, 0.18)
	border.A = 84
	rows := min(count, viewerFindMaxRows)
	if rows == 0 {
		rows = 1
	}
	resultsHeight := rows * gtx.Dp(unit.Dp(viewerFindRowHeightDp))
	previewHeight := compactFindPreviewHeight(gtx, preview)
	height := resultsHeight + previewHeight
	if available := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(48)); height > available {
		height = available
	}
	if height < 1 {
		return layout.Dimensions{}
	}
	gtx.Constraints.Max.Y = height
	gtx.Constraints.Min.Y = height
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedClipBox(gtx, 0, bg, border, func(gtx layout.Context) layout.Dimensions {
			availablePreviewHeight := max(0, height-resultsHeight)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, min(resultsHeight, height), func(gtx layout.Context) layout.Dimensions {
						if count == 0 {
							return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, "Searching…")
								lbl.Font.Typeface = ui.viewerTypeface()
								lbl.TextSize = scaleThemeFontSize(th, 10)
								lbl.Color = theme.Hint
								lbl.MaxLines = 1
								return layoutVCenteredLabel(gtx, lbl)
							})
						}
						list := material.List(th, listState)
						list.AnchorStrategy = material.Occupy
						list.ScrollbarStyle.Track.MajorPadding = 0
						list.ScrollbarStyle.Track.MinorPadding = unit.Dp(1)
						list.ScrollbarStyle.Track.Color = color.NRGBA{}
						list.ScrollbarStyle.Indicator.MajorMinLen = unit.Dp(18)
						list.ScrollbarStyle.Indicator.MinorWidth = unit.Dp(3)
						list.ScrollbarStyle.Indicator.CornerRadius = 0
						list.ScrollbarStyle.Indicator.Color = theme.ScrollThumb
						list.ScrollbarStyle.Indicator.HoverColor = theme.ScrollThumbHover
						return list.Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
							if index < 0 || index >= count {
								return layout.Dimensions{}
							}
							return ui.layoutFileViewerFindResult(th, gtx, st, index)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if availablePreviewHeight <= 0 {
						return layout.Dimensions{}
					}
					return fixedHeight(gtx, availablePreviewHeight, func(gtx layout.Context) layout.Dimensions {
						return layoutCompactFindDockedPreview(th, gtx, theme, ui.viewerTypeface(), scaleThemeFontSize(th, 9), preview)
					})
				}),
			)
		})
	})
}

func fileViewerFindHoveredIndex(st *fileViewerState) int {
	if st == nil {
		return -1
	}
	clicks := st.find.textClicks
	if viewerPDFPreviewActive(st) {
		clicks = st.find.pdfClicks
	} else if st.mode == "hex" {
		clicks = st.find.hexClicks
	}
	for i := range clicks {
		if clicks[i].Hovered() {
			return i
		}
	}
	return -1
}

func (ui *UI) fileViewerFindPreviewForIndex(st *fileViewerState, index int) compactFindPreview {
	if st == nil || index < 0 {
		return compactFindPreview{}
	}
	if viewerPDFPreviewActive(st) {
		if index >= len(st.find.pdfMatches) {
			return compactFindPreview{}
		}
		match := st.find.pdfMatches[index]
		return compactFindPreview{Lines: viewerFindPreviewLines(match.Preview), Focus: match.PreviewFocus, Highlights: match.PreviewHighlights}
	}
	if st.mode == "hex" {
		if index >= len(st.find.hexMatches) {
			return compactFindPreview{}
		}
		rows := fm.NormalizeViewerHexPreviewRows(ui.fmCfg.Viewer.HexPreviewRows)
		return viewerHexFindPreview(st, st.find.hexMatches[index], rows)
	}
	if index >= len(st.find.matches) {
		return compactFindPreview{}
	}
	match := st.find.matches[index]
	return compactFindPreview{Lines: viewerFindPreviewLines(match.Preview), Focus: match.PreviewFocus, Highlights: match.PreviewHighlights}
}

func viewerFindPreviewLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strings.ReplaceAll(line, "\t", "    ")
	}
	return out
}

func viewerHexFindPreviewLines(st *fileViewerState, match viewerHexFindMatch) []string {
	return viewerHexFindPreview(st, match, fm.NormalizeViewerHexPreviewRows(0)).Lines
}

type viewerHexPreviewLine struct {
	data  []byte
	start int
}

func viewerHexFindPreview(st *fileViewerState, match viewerHexFindMatch, rows int) compactFindPreview {
	rows = fm.NormalizeViewerHexPreviewRows(rows)
	data := match.PreviewBytes
	if len(data) == 0 {
		if context, ok := viewerHexFindContext(st, match); ok {
			data = context
		}
	}
	if len(data) == 0 {
		return compactFindPreview{}
	}
	if st != nil && st.find.hexPreview {
		matchOffset := min(max(match.PreviewMatch, 0), len(data))
		dataStart := match.Start - int64(matchOffset)
		rowStart := int((match.Start/int64(viewerHexPreviewBytesPerRow))*int64(viewerHexPreviewBytesPerRow) - dataStart)
		if rowStart < 0 || rowStart >= len(data) {
			rowStart = (matchOffset / viewerHexPreviewBytesPerRow) * viewerHexPreviewBytesPerRow
		}
		preview := compactFindPreview{Focus: 0}
		matchEnd := matchOffset + int(match.Length)
		for from := rowStart; from < len(data) && len(preview.Lines) < rows; from += viewerHexPreviewBytesPerRow {
			to := min(len(data), from+viewerHexPreviewBytesPerRow)
			line := viewerHexBytesSnippet(data[from:to])
			preview.Lines = append(preview.Lines, line)
			hitFrom := max(from, matchOffset)
			hitTo := min(to, matchEnd)
			highlight := compactFindHighlight{Start: -1, End: -1}
			if hitFrom < hitTo {
				highlight.Start = (hitFrom - from) * 3
				highlight.End = (hitTo-from)*3 - 1
			}
			preview.Highlights = append(preview.Highlights, highlight)
		}
		return preview
	}
	lines := viewerHexDetectedPreviewLineData(data)
	if len(lines) < 2 {
		return compactFindPreview{}
	}
	matchOffset := min(max(match.PreviewMatch, 0), len(data))
	focus := 0
	for i, line := range lines {
		if matchOffset >= line.start && matchOffset <= line.start+len(line.data) {
			focus = i
			break
		}
	}
	end := min(len(lines), focus+rows)
	start := focus
	if end-start < rows {
		start = max(0, end-rows)
	}
	preview := compactFindPreview{Focus: focus - start}
	matchEnd := matchOffset + int(match.Length)
	for _, line := range lines[start:end] {
		localStart := max(0, matchOffset-line.start)
		localEnd := max(0, min(len(line.data), matchEnd-line.start))
		before, hit, after := viewerHexTextParts(line.data, localStart, max(0, localEnd-localStart))
		preview.Lines = append(preview.Lines, before+hit+after)
		highlight := compactFindHighlight{Start: -1, End: -1}
		if hit != "" {
			highlight.Start = len(before)
			highlight.End = len(before) + len(hit)
		}
		preview.Highlights = append(preview.Highlights, highlight)
	}
	return preview
}

func viewerHexDetectedPreviewLines(data []byte) [][]byte {
	lineData := viewerHexDetectedPreviewLineData(data)
	lines := make([][]byte, len(lineData))
	for i := range lineData {
		lines[i] = lineData[i].data
	}
	return lines
}

func viewerHexDetectedPreviewLineData(data []byte) []viewerHexPreviewLine {
	if len(data) == 0 || !bytes.Contains(data, []byte{'\n'}) {
		return nil
	}
	raw := make([]viewerHexPreviewLine, 0, bytes.Count(data, []byte{'\n'})+1)
	start := 0
	for start <= len(data) {
		end := bytes.IndexByte(data[start:], '\n')
		if end < 0 {
			end = len(data)
		} else {
			end += start
		}
		line := bytes.TrimSuffix(data[start:end], []byte{'\r'})
		raw = append(raw, viewerHexPreviewLine{data: line, start: start})
		if end == len(data) {
			break
		}
		start = end + 1
	}
	for len(raw) > 0 && len(raw[0].data) == 0 {
		raw = raw[1:]
	}
	for len(raw) > 0 && len(raw[len(raw)-1].data) == 0 {
		raw = raw[:len(raw)-1]
	}
	if len(raw) < 2 {
		return nil
	}
	return raw
}

func (ui *UI) layoutFileViewerFindResult(th *material.Theme, gtx layout.Context, st *fileViewerState, index int) layout.Dimensions {
	marker, snippet, click, markerWidth := ui.fileViewerFindResultPresentation(st, index)
	if click == nil {
		return layout.Dimensions{}
	}
	theme := ui.fileViewerTheme()
	active := index == st.find.index
	rowBg := color.NRGBA{}
	if active {
		rowBg = mixNRGBA(theme.PanelBg, theme.StrongSelection, 0.34)
		rowBg.A = 224
	} else if click.Hovered() {
		progress := compactFindHoverProgress(&st.find.cursorAnim, gtx.Now, index)
		rowBg = mixNRGBA(theme.PanelBg, theme.HeaderText, 0.03+0.05*progress)
		rowBg.A = uint8(205 + 33*progress)
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return fixedHeight(gtx, gtx.Dp(unit.Dp(viewerFindRowHeightDp)), func(gtx layout.Context) layout.Dimensions {
			return fillBgExact(gtx, rowBg, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(6), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutCompactFindCursor(th, gtx, ui.viewerTypeface(), scaleThemeFontSize(th, 10), theme.StatusAccent, &st.find.cursorAnim, index)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							page := material.Body2(th, marker)
							page.Font.Typeface = ui.viewerTypeface()
							page.Font.Weight = font.Bold
							page.TextSize = scaleThemeFontSize(th, 10)
							page.Color = theme.StatusAccent
							page.MaxLines = 1
							return fixedWidth(gtx, gtx.Dp(unit.Dp(markerWidth)), func(gtx layout.Context) layout.Dimensions {
								return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layoutVCenteredLabel(gtx, page)
								})
							})
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(9)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							value, highlight := fileViewerFindResultHighlight(st, index, snippet)
							return layoutCompactFindHighlightedText(th, gtx, theme, ui.viewerTypeface(), scaleThemeFontSize(th, 10), value, highlight, true)
						}),
					)
				})
			})
		})
	})
}

func fileViewerFindResultHighlight(st *fileViewerState, index int, fallback string) (string, compactFindHighlight) {
	if st == nil || index < 0 {
		return fallback, compactFindHighlight{Start: -1, End: -1}
	}
	if viewerPDFPreviewActive(st) {
		if index < len(st.find.pdfMatches) {
			match := st.find.pdfMatches[index]
			return match.Snippet, match.SnippetHighlight
		}
		return fallback, compactFindHighlight{Start: -1, End: -1}
	}
	if st.mode == "hex" {
		if index < len(st.find.hexMatches) {
			return viewerHexFindHighlightedSnippet(st, st.find.hexMatches[index])
		}
		return fallback, compactFindHighlight{Start: -1, End: -1}
	}
	if index < len(st.find.matches) {
		match := st.find.matches[index]
		return match.Snippet, match.SnippetHighlight
	}
	return fallback, compactFindHighlight{Start: -1, End: -1}
}

func (ui *UI) fileViewerFindResultPresentation(st *fileViewerState, index int) (marker, snippet string, click *widget.Clickable, markerWidth int) {
	if st == nil || index < 0 {
		return "", "", nil, 0
	}
	if viewerPDFPreviewActive(st) {
		if index >= len(st.find.pdfMatches) || index >= len(st.find.pdfClicks) {
			return "", "", nil, 0
		}
		match := st.find.pdfMatches[index]
		return strconv.Itoa(match.Page + 1), match.Snippet, &st.find.pdfClicks[index], 34
	}
	if st.mode == "hex" {
		if index >= len(st.find.hexMatches) || index >= len(st.find.hexClicks) {
			return "", "", nil, 0
		}
		match := st.find.hexMatches[index]
		return fmt.Sprintf("0x%X", match.Start), viewerHexFindSnippet(st, match), &st.find.hexClicks[index], 66
	}
	if index >= len(st.find.matches) || index >= len(st.find.textClicks) {
		return "", "", nil, 0
	}
	match := st.find.matches[index]
	return strconv.Itoa(match.Line), match.Snippet, &st.find.textClicks[index], 34
}

func viewerHexFindSnippet(st *fileViewerState, match viewerHexFindMatch) string {
	if len(match.PreviewBytes) > 0 {
		compact := viewerHexCompactPreviewBytes(match)
		if st != nil && st.find.hexPreview {
			return viewerHexBytesSnippet(compact)
		}
		return viewerHexTextSnippet(compact)
	}
	if st != nil && st.find.hexPreview && match.HexPreview != "" {
		return match.HexPreview
	}
	if st != nil && !st.find.hexPreview && match.TextPreview != "" {
		return match.TextPreview
	}
	data, ok := viewerHexFindContext(st, match)
	if !ok {
		return viewerHexFindPatternSnippet(st)
	}
	if st.find.hexPreview {
		return viewerHexBytesSnippet(data)
	}
	return viewerHexTextSnippet(data)
}

func viewerHexCompactPreviewBytes(match viewerHexFindMatch) []byte {
	data, _ := viewerHexCompactPreview(match)
	return data
}

func viewerHexCompactPreview(match viewerHexFindMatch) ([]byte, int) {
	if len(match.PreviewBytes) == 0 {
		return nil, 0
	}
	hit := min(max(match.PreviewMatch, 0), len(match.PreviewBytes))
	from := max(0, hit-8)
	to := min(len(match.PreviewBytes), hit+int(match.Length)+12)
	if to <= from {
		return match.PreviewBytes, hit
	}
	return match.PreviewBytes[from:to], hit - from
}

func viewerHexFindHighlightedSnippet(st *fileViewerState, match viewerHexFindMatch) (string, compactFindHighlight) {
	data, hit := viewerHexCompactPreview(match)
	if len(data) == 0 {
		value := viewerHexFindPatternSnippet(st)
		return value, compactFindHighlight{Start: 0, End: len(value)}
	}
	hitLen := min(int(match.Length), len(data)-hit)
	if st != nil && st.find.hexPreview {
		value := viewerHexBytesSnippet(data)
		return value, compactFindHighlight{Start: hit * 3, End: (hit+hitLen)*3 - 1}
	}
	before, found, after := viewerHexTextParts(data, hit, hitLen)
	return before + found + after, compactFindHighlight{Start: len(before), End: len(before) + len(found)}
}

func viewerHexFindContext(st *fileViewerState, match viewerHexFindMatch) ([]byte, bool) {
	if st == nil || st.hex == nil || len(st.hex.buffer) == 0 {
		return nil, false
	}
	from := match.Start - 8
	to := match.Start + match.Length + 12
	bufferStart := st.hex.bufferStart
	bufferEnd := bufferStart + int64(len(st.hex.buffer))
	if from < bufferStart {
		from = bufferStart
	}
	if to > bufferEnd {
		to = bufferEnd
	}
	if from >= to || from < bufferStart || to > bufferEnd {
		return nil, false
	}
	return st.hex.buffer[from-bufferStart : to-bufferStart], true
}

func viewerHexBytesSnippet(data []byte) string {
	var hexText strings.Builder
	for i, b := range data {
		if i > 0 {
			hexText.WriteByte(' ')
		}
		fmt.Fprintf(&hexText, "%02X", b)
	}
	return hexText.String()
}

func viewerHexTextSnippet(data []byte) string {
	var text strings.Builder
	for _, b := range data {
		switch {
		case b >= 0x20 && b <= 0x7E:
			text.WriteByte(b)
		case b == '\t' || b == '\r' || b == '\n':
			text.WriteByte(' ')
		default:
			text.WriteByte('.')
		}
	}
	return strings.Join(strings.Fields(text.String()), " ")
}

func viewerHexTextParts(data []byte, hitStart, hitLen int) (before, hit, after string) {
	hitStart = min(max(hitStart, 0), len(data))
	hitEnd := min(len(data), hitStart+max(hitLen, 0))
	format := func(part []byte) string {
		var text strings.Builder
		for _, b := range part {
			switch {
			case b >= 0x20 && b <= 0x7E:
				text.WriteByte(b)
			case b == '\t':
				text.WriteRune('⇥')
			case b == '\r' || b == '\n':
				text.WriteByte(' ')
			default:
				text.WriteByte('.')
			}
		}
		return text.String()
	}
	return format(data[:hitStart]), format(data[hitStart:hitEnd]), format(data[hitEnd:])
}

func viewerHexFindPatternSnippet(st *fileViewerState) string {
	if st == nil {
		return "byte match"
	}
	pattern, _ := viewerFindPatternBytes(st.find.editor.Text(), st.find.hexInput)
	if len(pattern) == 0 {
		return "byte match"
	}
	if !st.find.hexPreview {
		return viewerHexTextSnippet(pattern)
	}
	var hexText strings.Builder
	for i, b := range pattern {
		if i >= 20 {
			hexText.WriteString(" …")
			break
		}
		if i > 0 {
			hexText.WriteByte(' ')
		}
		fmt.Fprintf(&hexText, "%02X", b)
	}
	return hexText.String()
}

func (ui *UI) fileViewerFindPlaceholder(st *fileViewerState) string {
	if st != nil && st.mode == "hex" {
		if st.find.hexInput {
			return "DE AD BE EF"
		}
		return "text"
	}
	return "Find text"
}

func (ui *UI) fileViewerFindStatusText(st *fileViewerState, now time.Time) string {
	if st == nil {
		return ""
	}
	if st.find.searching && !st.find.searchStartedAt.IsZero() && now.Sub(st.find.searchStartedAt) >= viewerFindSearchingDelay {
		if st.find.currentValid && st.find.status != "" {
			return st.find.status
		}
		return "Searching..."
	}
	return st.find.status
}

func (ui *UI) fileViewerFindStatusColor(st *fileViewerState, now time.Time) color.NRGBA {
	theme := ui.fileViewerTheme()
	text := ui.fileViewerFindStatusText(st, now)
	switch {
	case st == nil:
		return theme.Hint
	case text == "Searching...":
		return theme.StatusAccent
	case strings.HasPrefix(text, "No match"):
		return theme.StatusWarn
	case strings.Contains(strings.ToLower(text), "query") || strings.Contains(strings.ToLower(text), "invalid"):
		return theme.Error
	case st.find.currentValid:
		return theme.HeaderText
	default:
		return theme.Hint
	}
}

func (ui *UI) fileViewerFindSourceLabel(st *fileViewerState) string {
	if st != nil && st.find.remoteSearch {
		return "Remote"
	}
	return "Local"
}

func (ui *UI) layoutFileViewerFindSourceSelect(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	if ui == nil || st == nil || !ui.fileViewerFindRemoteSearchConfigured(st) {
		return layout.Dimensions{}
	}
	theme := ui.fileViewerTheme()
	return ui.layoutFileViewerOverlayChip(th, gtx, ui.fileViewerFindSourceLabel(st)+" ▾", theme.CommandText, st.find.sourceMenuOpen, &st.find.sourceMenuClick)
}

func (ui *UI) layoutFileViewerFindSourceMenu(th *material.Theme, gtx layout.Context, st *fileViewerState, alpha float32) layout.Dimensions {
	if ui == nil || st == nil {
		return layout.Dimensions{}
	}
	theme := ui.filePanePopupTheme()
	remoteDetail := "SSH utility command"
	if st.find.hexInput && !viewerRemoteSearchTemplateSupportsHex(ui.viewerRemoteSearchTemplate(st.remote)) {
		remoteDetail = "Current template is text-only"
	}
	type menuRow struct {
		click  *widget.Clickable
		item   fileContextMenuItem
		active bool
	}
	rows := []menuRow{
		{
			click: &st.find.sourceLocalClick,
			item: fileContextMenuItem{
				ID:     "viewer-find-source-local",
				Label:  "Local",
				Detail: "Built-in chunked search",
			},
			active: !st.find.remoteSearch,
		},
		{
			click: &st.find.sourceRemoteClick,
			item: fileContextMenuItem{
				ID:     "viewer-find-source-remote",
				Label:  "Remote",
				Detail: remoteDetail,
			},
			active: st.find.remoteSearch,
		},
	}
	width := gtx.Dp(unit.Dp(196))
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedClipBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			scaleColorAlpha(theme.Bg, alpha),
			scaleColorAlpha(theme.Border, alpha),
			func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(rows)*2)
				for i := range rows {
					if i > 0 {
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fillBgExact(gtx, scaleColorAlpha(theme.Divider, alpha), func(gtx layout.Context) layout.Dimensions {
								return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
							})
						}))
					}
					row := rows[i]
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						hoverFill := float32(0)
						if row.click.Hovered() {
							hoverFill = 1
						}
						dims, _, _ := ui.layoutFilePaneContextMenuItem(th, gtx, theme, row.click, row.item, row.active, hoverFill, alpha, ui.fileContextMenuRowHeight(gtx, row.item))
						return dims
					}))
				}
				return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			},
		)
	})
}
