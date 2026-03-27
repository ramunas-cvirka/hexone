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

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	uitheme "hexone/ui/theme"
)

const (
	viewerFindChunkBytes       = 256 << 10
	viewerFindBarGapDp         = 4
	viewerFindBarInsetDp       = 6
	viewerFindBarRowHeightDp   = 22
	viewerFindStatusMaxDp      = 120
	viewerFindFieldChars       = 42
	viewerFindFieldMinChars    = 18
	viewerFindSearchingDelay   = 220 * time.Millisecond
	viewerRemoteSearchMaxBytes = 8 << 10
)

type viewerFindMatch struct {
	Start int
	End   int
}

type fileViewerFindState struct {
	editor widget.Editor

	open  bool
	focus bool

	prevClick         widget.Clickable
	nextClick         widget.Clickable
	closeClick        widget.Clickable
	sourceMenuClick   widget.Clickable
	sourceLocalClick  widget.Clickable
	sourceRemoteClick widget.Clickable

	remoteSearch     bool
	sourceInit       bool
	sourceMenuOpen   bool
	sourceMenuAt     time.Time
	sourceButtonRect image.Rectangle
	sourceMenuRect   image.Rectangle
	status           string
	searchStartedAt  time.Time

	matches []viewerFindMatch
	index   int

	currentStart int64
	currentLen   int64
	currentValid bool

	searching  bool
	requestSeq int
	resultCh   chan fileViewerFindResult
	cancel     context.CancelFunc
}

type fileViewerFindResult struct {
	requestSeq int
	found      bool
	start      int64
	length     int64
	wrapped    bool
	err        string
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
			} else if viewerFindHexModeFromQuery(st.find.editor.Text()) {
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
	st.find.sourceButtonRect = image.Rectangle{}
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
	ui.ensureFileViewerFindSearchSource(now, st)
	ui.syncFileViewerFindRemoteSearch(now, st)
	query := st.find.editor.Text()
	if st.mode == "hex" {
		ui.refreshHexFileViewerFind(now, query, preserve)
		return
	}
	ui.refreshStreamFileViewerFind(now, query, preserve)
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
	matches := viewerFindTextMatches(st.content, query)
	st.find.matches = matches
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
}

func (ui *UI) refreshHexFileViewerFind(now time.Time, query string, preserve bool) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	pattern, useHex, errText := viewerFindAutoPatternBytes(query)
	if errText != "" {
		ui.cancelFileViewerFindSearch(st)
		st.find.status = errText
		st.find.currentValid = false
		st.find.currentStart = 0
		st.find.currentLen = 0
		return
	}
	if len(pattern) == 0 {
		ui.cancelFileViewerFindSearch(st)
		st.find.status = ""
		st.find.currentValid = false
		st.find.currentStart = 0
		st.find.currentLen = 0
		return
	}
	anchor := viewerHexFindAnchor(st, preserve)
	ui.startHexFileViewerFindNext(now, pattern, anchor, useHex)
}

func (ui *UI) stepFileViewerFind(now time.Time, direction int) bool {
	st := ui.fileViewer
	if st == nil || !st.find.open {
		return false
	}
	if st.mode == "hex" {
		return ui.stepHexFileViewerFind(now, direction)
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

func (ui *UI) stepHexFileViewerFind(now time.Time, direction int) bool {
	st := ui.fileViewer
	if st == nil {
		return false
	}
	pattern, useHex, errText := viewerFindAutoPatternBytes(st.find.editor.Text())
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
	}()

	if st.hex != nil {
		st.markUserBrowsing(now)
	}
}

func (ui *UI) pumpFileViewerFindState(gtx layout.Context, st *fileViewerState) {
	if st == nil || st.find.resultCh == nil {
		return
	}
	for {
		select {
		case res := <-st.find.resultCh:
			if res.requestSeq != st.find.requestSeq {
				continue
			}
			st.find.searching = false
			st.find.cancel = nil
			st.find.searchStartedAt = time.Time{}
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

func sendFileViewerFindResult(ch chan fileViewerFindResult, res fileViewerFindResult) {
	if ch == nil {
		return
	}
	select {
	case ch <- res:
	default:
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- res:
		default:
		}
	}
}

func viewerFindTextMatches(content, query string) []viewerFindMatch {
	if content == "" || query == "" {
		return nil
	}
	matches := make([]viewerFindMatch, 0, 8)
	for off := 0; off <= len(content)-len(query); {
		idx := strings.Index(content[off:], query)
		if idx < 0 {
			break
		}
		start := off + idx
		matches = append(matches, viewerFindMatch{Start: start, End: start + len(query)})
		off = start + 1
	}
	return matches
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
	v.topLine = viewerKeepStreamLineVisible(v.topLine, visible, line)
	v.clampTop()
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

func viewerFindHexModeFromQuery(raw string) bool {
	query := strings.TrimSpace(raw)
	if query == "" || len(query)%2 != 0 {
		return false
	}
	for i := 0; i < len(query); i++ {
		if _, ok := viewerHexNibble(query[i]); !ok {
			return false
		}
	}
	return true
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

func viewerFindAutoPatternBytes(raw string) ([]byte, bool, string) {
	if !viewerFindHexModeFromQuery(raw) {
		pattern, errText := viewerFindPatternBytes(raw, false)
		return pattern, false, errText
	}
	pattern, errText := viewerFindPatternBytes(strings.TrimSpace(raw), true)
	return pattern, true, errText
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
	if st != nil && viewerFindHexModeFromQuery(st.find.editor.Text()) && !viewerRemoteSearchTemplateSupportsHex(ui.viewerRemoteSearchTemplate(st.remote)) {
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
	statusW := ui.fileViewerFindStatusWidth(th, gtx)
	reserved := gtx.Dp(unit.Dp(16))
	if sourceW := ui.fileViewerFindSourceChipWidth(th, gtx, st); sourceW > 0 {
		reserved += sourceW + gtx.Dp(unit.Dp(6))
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
	bar := op.Record(gtx.Ops)
	barDims := fixedWidth(gtx, barW, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedClipBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
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
								return layoutTinyIconModeButton(th, gtx, &st.find.closeClick, uitheme.CloseIcon(), false)
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
		st.find.sourceMenuRect = image.Rectangle{}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	barPos := image.Pt(
		gtx.Constraints.Max.X-barDims.Size.X-gtx.Dp(unit.Dp(viewerFindBarInsetDp)),
		gtx.Dp(unit.Dp(viewerFindBarGapDp)),
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
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *UI) fileViewerFindPlaceholder(st *fileViewerState) string {
	if st != nil && st.mode == "hex" {
		return "Text or DEADBEEF"
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
	if viewerFindHexModeFromQuery(st.find.editor.Text()) && !viewerRemoteSearchTemplateSupportsHex(ui.viewerRemoteSearchTemplate(st.remote)) {
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
