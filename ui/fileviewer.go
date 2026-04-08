// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hexone/filesys"
	"hexone/fm"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"golang.org/x/crypto/ssh"
	"golang.org/x/text/encoding/charmap"
)

const (
	viewerDefaultMaxLoadBytes  = 1 << 20
	viewerHexCopyMaxBytes      = 1 << 20
	viewerBinaryPreviewBytes   = 64
	viewerBinaryPreviewMaxCols = viewerBinaryPreviewBytes
	viewerCommandExecTimeout   = 15 * time.Second
	viewerCommandStreamTick    = 180 * time.Millisecond
	viewerDefaultRefreshMs     = 1500
	viewerMinimumRefreshMs     = 200
	viewerDefaultAutoRefresh   = true
	viewerDefaultMaxReadMB     = 1
	viewerDefaultWordRegex     = `[a-zA-Z0-9]+`
	viewerCommandHistoryLimit  = 80
)

const (
	viewerLineEndingLF    = "lf"
	viewerLineEndingCRLF  = "crlf"
	viewerLineEndingMixed = "mixed"
	viewerLineEndingNone  = "none"
)

// Pointer event tags must be non-zero-sized; zero-sized fields can share the
// same address and collapse distinct handlers onto one Gio tag.
type fileViewerEventTag struct {
	_ byte
}

type fileViewerState struct {
	pane    int
	path    string
	name    string
	mode    string
	tabPrev string
	remote  *paneSSHSession

	backdropClick        widget.Clickable
	closeClick           widget.Clickable
	autoRefreshClick     widget.Clickable
	modeFileClick        widget.Clickable
	modeHexClick         widget.Clickable
	modeCmdClick         widget.Clickable
	encodingMenuClick    widget.Clickable
	encodingAutoClick    widget.Clickable
	encodingUTF8Click    widget.Clickable
	encodingUTF16LEClick widget.Clickable
	encodingUTF16BEClick widget.Clickable
	encodingCP437Click   widget.Clickable
	historyClick         widget.Clickable
	commandClick         widget.Clickable
	contentEditor        widget.Editor
	commandEditor        widget.Editor
	wrapToggle           widget.Clickable
	copyToggle           widget.Clickable
	commandEditOn        bool
	commandFocus         bool
	fileEncoding         string
	encodingMenuOpen     bool

	content               string
	status                string
	err                   string
	command               string
	detectedEncoding      string
	detectedEncodingBOM   bool
	detectedImagePreview  bool
	imagePreview          image.Image
	imagePreviewData      []byte
	imagePreviewFormat    string
	imagePreviewSize      image.Point
	detectedBinaryPreview bool
	detectedLineEnding    string
	binaryPreviewData     []byte
	binaryPreviewCols     int
	commandInfinite       bool
	autoRefresh           bool
	wordSelectRE          *regexp.Regexp
	wordSelectExpr        string
	updatedAt             time.Time
	tabAnimAt             time.Time
	stream                streamOutputView
	imageView             imagePreviewView
	hex                   *hexViewerState
	find                  fileViewerFindState
	historyOpen           bool

	loading    bool
	seq        int
	loadCancel context.CancelFunc

	contentPointerTag    fileViewerEventTag
	rootPointerTag       fileViewerEventTag
	commandAreaTag       fileViewerEventTag
	commandAreaPress     map[pointer.ID]struct{}
	userBrowseUntil      time.Time
	pendingUpdate        bool
	pendingContent       string
	pendingStatus        string
	pendingErr           string
	pendingEncoding      string
	pendingEncodingBOM   bool
	pendingImagePreview  bool
	pendingImage         image.Image
	pendingImageData     []byte
	pendingImageFormat   string
	pendingImageSize     image.Point
	pendingBinaryPreview bool
	pendingLineEnding    string
	pendingBinaryData    []byte
	wrapEnabled          bool
	menuOpen             bool
	menuPos              image.Point
	menuRect             image.Rectangle
	menuOpenedAt         time.Time
	menuHoverID          string
	menuPointerTag       fileViewerEventTag
	scrollCarry          float32
	scrollbarTrack       image.Rectangle
	scrollbarThumb       image.Rectangle
	scrollbarDragging    bool
	scrollbarDragID      pointer.ID
	scrollbarHover       bool
	scrollbarVisible     bool
	scrollbarLines       int
	scrollbarVisibleN    int

	nextWatchCheck   time.Time
	watchExists      bool
	watchSize        int64
	watchModTime     time.Time
	fileSelection    streamSelectionState
	commandSelection streamSelectionState
	pendingSelection string
	resultCh         chan fileViewerResult
	historyClicks    map[string]*widget.Clickable
	tabAnim          segmentedAnimState
	menuHoverAnim    segmentedAnimState
	activeTabRect    image.Rectangle
	encodingBarRect  image.Rectangle
	encodingMenuRect image.Rectangle
	encodingMenuAt   time.Time
}

type fileViewerResult struct {
	seq           int
	content       string
	status        string
	err           string
	encoding      string
	encodingBOM   bool
	imagePreview  bool
	image         image.Image
	imageData     []byte
	imageFormat   string
	imageSize     image.Point
	binaryPreview bool
	lineEnding    string
	binaryData    []byte
	partial       bool
	final         bool
}

type viewerReadInfo struct {
	encoding      string
	encodingBOM   bool
	imagePreview  bool
	image         image.Image
	imageData     []byte
	imageFormat   string
	imageSize     image.Point
	binaryPreview bool
	lineEnding    string
	binaryData    []byte
}

func (st *fileViewerState) openContextMenu(pos image.Point, now time.Time) {
	if st == nil {
		return
	}
	st.menuOpen = true
	st.menuPos = pos
	st.menuOpenedAt = now
	st.menuHoverID = ""
	st.menuHoverAnim = segmentedAnimState{}
}

func (st *fileViewerState) closeContextMenu() {
	if st == nil {
		return
	}
	st.menuOpen = false
	st.menuRect = image.Rectangle{}
	st.menuOpenedAt = time.Time{}
	st.menuHoverID = ""
	st.menuHoverAnim = segmentedAnimState{}
}

func (st *fileViewerState) closeEncodingMenu() {
	if st == nil {
		return
	}
	st.encodingMenuOpen = false
	st.encodingBarRect = image.Rectangle{}
	st.encodingMenuRect = image.Rectangle{}
	st.encodingMenuAt = time.Time{}
}

func (st *fileViewerState) rememberStreamSelection(mode string) {
	if st == nil {
		return
	}
	switch normalizeViewerMode(mode) {
	case "file":
		st.fileSelection = st.stream.selectionState()
	case "command":
		st.commandSelection = st.stream.selectionState()
	}
}

func (st *fileViewerState) restoreStreamSelection(mode string) {
	if st == nil {
		return
	}
	switch normalizeViewerMode(mode) {
	case "file":
		st.stream.restoreSelectionState(st.fileSelection)
	case "command":
		st.stream.restoreSelectionState(st.commandSelection)
	default:
		st.stream.clearSelection()
	}
}

func (st *fileViewerState) prepareStreamSelectionForMode(mode string) {
	if st == nil {
		return
	}
	st.rememberStreamSelection(st.mode)
	mode = normalizeViewerMode(mode)
	if mode == "file" || mode == "command" {
		st.pendingSelection = mode
	} else {
		st.pendingSelection = ""
	}
	st.stream.clearSelection()
}

func (st *fileViewerState) restorePendingStreamSelection() {
	if st == nil || st.pendingSelection == "" {
		return
	}
	if normalizeViewerMode(st.mode) != st.pendingSelection {
		return
	}
	st.restoreStreamSelection(st.pendingSelection)
	st.pendingSelection = ""
}

func (ui *UI) handleFileViewerKeys(gtx layout.Context) {
	anyMods := ^key.Modifiers(0)
	st := ui.fileViewer
	if st == nil {
		return
	}
	findFocused := st.find.open && gtx.Focused(&st.find.editor)
	editorFocused := st.commandEditOn || findFocused
	filters := []event.Filter{
		key.Filter{Name: key.NameEscape},
		key.Filter{Name: "f", Required: key.ModCtrl, Optional: anyMods},
		key.Filter{Name: "F", Required: key.ModCtrl, Optional: anyMods},
		key.Filter{Name: "f", Required: key.ModShortcut, Optional: anyMods},
		key.Filter{Name: "F", Required: key.ModShortcut, Optional: anyMods},
	}
	if st.find.open {
		filters = append(filters,
			key.Filter{Focus: &st.find.editor, Name: key.NameEnter, Optional: anyMods},
			key.Filter{Focus: &st.find.editor, Name: key.NameReturn, Optional: anyMods},
			key.Filter{Focus: &st.find.editor, Name: key.NameTab, Optional: anyMods},
		)
	}
	if !editorFocused {
		filters = append(filters,
			key.Filter{Name: key.NameF3},
			key.Filter{Name: key.NameTab, Optional: anyMods},
			key.Filter{Name: key.NameUpArrow},
			key.Filter{Name: key.NameDownArrow},
			key.Filter{Name: key.NamePageUp},
			key.Filter{Name: key.NamePageDown},
			key.Filter{Name: key.NameHome},
			key.Filter{Name: key.NameEnd},
			key.Filter{Name: "c", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "C", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "c", Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "C", Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "a", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "A", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "a", Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "A", Required: key.ModShortcut, Optional: anyMods},
		)
		if st.detectedImagePreview {
			filters = append(filters,
				key.Filter{Name: key.NameLeftArrow},
				key.Filter{Name: key.NameRightArrow},
				key.Filter{Name: "+", Required: key.ModCtrl, Optional: anyMods},
				key.Filter{Name: "+", Required: key.ModShortcut, Optional: anyMods},
				key.Filter{Name: "=", Required: key.ModCtrl, Optional: anyMods},
				key.Filter{Name: "=", Required: key.ModShortcut, Optional: anyMods},
				key.Filter{Name: "-", Required: key.ModCtrl, Optional: anyMods},
				key.Filter{Name: "-", Required: key.ModShortcut, Optional: anyMods},
				key.Filter{Name: "_", Required: key.ModCtrl, Optional: anyMods},
				key.Filter{Name: "_", Required: key.ModShortcut, Optional: anyMods},
			)
		}
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok {
			continue
		}
		if ke.Name == key.NameF3 && ke.State == key.Release {
			ui.clearFileViewHotkeyHold()
			continue
		}
		if factor, ok := viewerImageZoomFactorForKey(ke.Name, ke.Modifiers); ok {
			if ke.State != key.Press || !st.detectedImagePreview {
				continue
			}
			if st.imageView.zoomBy(st.imagePreview, factor) {
				st.markUserBrowsing(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
			}
			continue
		}
		if viewerScrollKeySupported(ke.Name) {
			switch ke.State {
			case key.Press:
				st := ui.fileViewer
				if st == nil || st.commandEditOn || st.historyOpen || ke.Modifiers != 0 {
					continue
				}
				if viewerScrollRepeatableKey(ke.Name) {
					if ui.held[string(ke.Name)] {
						continue
					}
					ui.held[string(ke.Name)] = true
				}
				if !ui.performFileViewerKeyScroll(gtx.Now, ke.Name) {
					if viewerScrollRepeatableKey(ke.Name) {
						ui.stopFileViewerScrollRepeat(ke.Name)
					}
					continue
				}
				gtx.Execute(op.InvalidateCmd{})
				if viewerScrollRepeatableKey(ke.Name) {
					ui.startFileViewerScrollRepeat(ke.Name, gtx.Now)
					gtx.Execute(op.InvalidateCmd{At: ui.rep.next})
				}
			case key.Release:
				if viewerScrollRepeatableKey(ke.Name) {
					ui.stopFileViewerScrollRepeat(ke.Name)
				}
			}
			continue
		}
		if ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case "f", "F":
			if ke.Modifiers != key.ModCtrl && ke.Modifiers != key.ModShortcut {
				continue
			}
			ui.openFileViewerFind(gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		case key.NameEscape:
			if st.find.open {
				if st.find.sourceMenuOpen {
					st.find.closeSourceMenu()
					gtx.Execute(op.InvalidateCmd{})
					continue
				}
				ui.closeFileViewerFind()
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			if st.commandEditOn {
				ui.cancelViewerCommandEdit()
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			ui.closeFileViewer()
		case key.NameF3:
			ui.startFileViewerLoad(gtx.Now)
		case key.NameEnter, key.NameReturn:
			if !st.find.open || !findFocused {
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
		case key.NameTab:
			step, ok := viewerModeTabStep(ke.Modifiers)
			if !ok {
				continue
			}
			ui.setFileViewerMode(viewerStepMode(st.mode, step), gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		case "c", "C":
			if st.commandEditOn || findFocused {
				continue
			}
			if ui.copyFileViewerText(gtx, false) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case "a", "A":
			if st.commandEditOn || findFocused {
				continue
			}
			if st.mode == "hex" {
				if st.hex != nil && len(st.hex.buffer) > 0 {
					start := st.hex.bufferStart
					length := int64(len(st.hex.buffer))
					st.hex.setSelectionRange(start, length)
				}
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			st.stream.selectAll()
			st.err = ""
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	ui.pumpFileViewerScrollRepeat(gtx)
}

func (ui *UI) copyFileViewerText(gtx layout.Context, fallbackAll bool) bool {
	st := ui.fileViewer
	if st == nil {
		return false
	}
	if st.mode == "hex" && st.hex != nil {
		var data []byte
		if st.hex.hasSelection() {
			if st.hex.selectionLen > viewerHexCopyMaxBytes {
				st.status = "hex copy is limited to 1 MiB"
				return false
			}
			var ok bool
			data, ok = st.hex.selectedBytes()
			if !ok {
				st.status = "selection is not loaded"
				return false
			}
		} else if fallbackAll && len(st.hex.buffer) > 0 {
			if len(st.hex.buffer) > viewerHexCopyMaxBytes {
				st.status = "hex copy is limited to 1 MiB"
				return false
			}
			data = append([]byte(nil), st.hex.buffer...)
		}
		if len(data) == 0 {
			st.status = "nothing to copy"
			return false
		}
		text := formatHexSelectionCopy(data)
		gtx.Execute(clipboard.WriteCmd{
			Type: "application/text",
			Data: io.NopCloser(strings.NewReader(text)),
		})
		st.err = ""
		return true
	}
	text := st.stream.selectedText()
	if text == "" && fallbackAll {
		text = st.content
	}
	if text == "" {
		st.status = "nothing to copy"
		return false
	}
	text = viewerClipboardContent(st, text)
	gtx.Execute(clipboard.WriteCmd{
		Type: "application/text",
		Data: io.NopCloser(strings.NewReader(text)),
	})
	st.err = ""
	return true
}

func (ui *UI) startFileViewer(idx int, now time.Time) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return
	}
	entry := pane.selectedEntry()
	if entry == nil || entry.Path == "" {
		pane.setNotice("nothing selected to view", now)
		return
	}
	if entry.Kind == filesys.EntryDir || entry.Kind == filesys.EntryParent {
		pane.setNotice("viewer supports files only", now)
		return
	}

	var remote *paneSSHSession
	if pane.remoteConnected() {
		remote = pane.remote.clone()
		if remote == nil {
			pane.setNotice("remote session is not connected", now)
			return
		}
	}

	st := &fileViewerState{
		pane:         idx,
		path:         entry.Path,
		name:         entry.DisplayName,
		remote:       remote,
		status:       "loading...",
		fileEncoding: fm.ViewerFileEncodingAuto,
		wrapEnabled:  false,
		resultCh:     make(chan fileViewerResult, 1),
	}
	st.mode = "file"
	st.command = "cat {path}"
	st.autoRefresh = viewerDefaultAutoRefresh
	if ui != nil && ui.fmCfg != nil {
		cfg := ui.fmCfg.Viewer
		st.autoRefresh = cfg.CommandAutoRefresh
		st.fileEncoding = fm.NormalizeViewerFileEncoding(cfg.FileEncoding)
		st.mode, st.command = ui.viewerConfiguredModeAndCommand(st.path, st.remote, cfg.Mode, cfg.Command)
	} else {
		st.mode, st.command = ui.viewerConfiguredModeAndCommand(st.path, st.remote, st.mode, st.command)
	}
	st.commandInfinite = st.mode == "command" && viewerCommandLooksInfinite(st.command)
	if st.name == "" {
		if st.remote != nil {
			st.name = pathpkg.Base(entry.Path)
		} else {
			st.name = filepath.Base(entry.Path)
		}
	}
	st.contentEditor.SingleLine = false
	st.contentEditor.ReadOnly = true
	st.contentEditor.Submit = false
	st.contentEditor.SetText("")
	st.stream.SetContent("")
	st.commandEditor.SingleLine = true
	st.commandEditor.Submit = true
	st.commandEditor.SetText(st.command)
	st.find.editor.SingleLine = true
	st.find.editor.Submit = false
	st.find.resultCh = make(chan fileViewerFindResult, 1)
	st.find.index = -1
	st.wordSelectRE, st.wordSelectExpr = viewerWordSelectRegexp(ui.fmCfg)
	st.captureWatchState()
	st.hex = newHexViewerState()
	st.hex.offsetDigits = viewerHexOffsetDigits(st.watchSize)

	ui.fileViewer = st
	ui.setActiveFilePane(idx)
	pane.stopPathEdit()
	pane.sortMenuOpen = false
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	ui.closeSortMenusExcept(idx)
	ui.closeFavoriteMenusExcept(idx)
	ui.closeContextMenusExcept(idx)
	ui.rep.active = false
	ui.rep.pane = -1
	ui.startFileViewerLoad(now)
}

func (ui *UI) closeFileViewer() {
	if st := ui.fileViewer; st != nil {
		if st.loadCancel != nil {
			st.loadCancel()
			st.loadCancel = nil
		}
		ui.cancelFileViewerFindSearch(st)
		st.closeEncodingMenu()
		if st.remote != nil {
			st.remote.close()
			st.remote = nil
		}
	}
	ui.clearFileViewHotkeyHold()
	ui.clearFileViewerScrollHold()
	ui.fileViewer = nil
	ui.functionBarViewerShown = false
}

func (ui *UI) clearFileViewHotkeyHold() {
	if ui == nil || ui.held == nil {
		return
	}
	ui.held[fileActionKey(fileActionView)] = false
}

func (ui *UI) clearFileViewerScrollHold() {
	ui.stopFileViewerScrollRepeat(key.NameLeftArrow)
	ui.stopFileViewerScrollRepeat(key.NameRightArrow)
	ui.stopFileViewerScrollRepeat(key.NameUpArrow)
	ui.stopFileViewerScrollRepeat(key.NameDownArrow)
	ui.stopFileViewerScrollRepeat(key.NamePageUp)
	ui.stopFileViewerScrollRepeat(key.NamePageDown)
}

func viewerScrollKeySupported(name key.Name) bool {
	switch name {
	case key.NameLeftArrow, key.NameRightArrow, key.NameUpArrow, key.NameDownArrow, key.NamePageUp, key.NamePageDown, key.NameHome, key.NameEnd:
		return true
	default:
		return false
	}
}

func viewerScrollRepeatableKey(name key.Name) bool {
	switch name {
	case key.NameLeftArrow, key.NameRightArrow, key.NameUpArrow, key.NameDownArrow, key.NamePageUp, key.NamePageDown:
		return true
	default:
		return false
	}
}

func (ui *UI) performFileViewerKeyScroll(now time.Time, name key.Name) bool {
	st := ui.fileViewer
	if st == nil || st.commandEditOn || st.historyOpen {
		return false
	}
	st.markUserBrowsing(now)
	changed := false
	if st.detectedImagePreview {
		switch name {
		case key.NameLeftArrow:
			changed = st.imageView.scrollByKeyStep(st.imagePreview, -1, 0)
		case key.NameRightArrow:
			changed = st.imageView.scrollByKeyStep(st.imagePreview, 1, 0)
		case key.NameUpArrow:
			changed = st.imageView.scrollByKeyStep(st.imagePreview, 0, -1)
		case key.NameDownArrow:
			changed = st.imageView.scrollByKeyStep(st.imagePreview, 0, 1)
		case key.NamePageUp:
			changed = st.imageView.scrollByPage(st.imagePreview, -1)
		case key.NamePageDown:
			changed = st.imageView.scrollByPage(st.imagePreview, 1)
		case key.NameHome:
			changed = st.imageView.scrollToOrigin()
		case key.NameEnd:
			changed = st.imageView.scrollToEnd(st.imagePreview)
		}
		return changed
	}
	switch name {
	case key.NameUpArrow:
		changed = viewerScrollByLines(st, -1)
	case key.NameDownArrow:
		changed = viewerScrollByLines(st, 1)
	case key.NamePageUp:
		changed = viewerScrollByPage(st, -1)
	case key.NamePageDown:
		changed = viewerScrollByPage(st, 1)
	case key.NameHome:
		changed = viewerScrollToStart(st)
	case key.NameEnd:
		changed = viewerScrollToEnd(st)
	}
	if changed && st.mode == "hex" && st.hex != nil {
		ui.startHexViewerLoad(st, false)
	}
	return changed
}

func (ui *UI) startFileViewerScrollRepeat(name key.Name, now time.Time) {
	ui.rep.active = true
	ui.rep.pane = -1
	ui.rep.name = string(name)
	ui.rep.started = now
	ui.rep.slow = repeatSlow
	ui.rep.fast = repeatFast
	ui.rep.accelAfter = repeatAccelAfter
	ui.rep.period = ui.rep.slow
	ui.rep.next = now.Add(repeatStartDelay)
}

func (ui *UI) stopFileViewerScrollRepeat(name key.Name) {
	if ui == nil {
		return
	}
	if ui.held != nil {
		ui.held[string(name)] = false
	}
	if ui.rep.active && ui.rep.name == string(name) {
		ui.rep.active = false
		ui.rep.pane = -1
	}
}

func (ui *UI) pumpFileViewerScrollRepeat(gtx layout.Context) {
	if ui == nil || !ui.rep.active {
		return
	}
	name := key.Name(ui.rep.name)
	if !viewerScrollRepeatableKey(name) {
		return
	}
	st := ui.fileViewer
	if st == nil || st.commandEditOn || st.historyOpen {
		ui.rep.active = false
		ui.rep.pane = -1
		return
	}
	if gtx.Now.Sub(ui.rep.started) >= ui.rep.accelAfter && ui.rep.period != ui.rep.fast {
		ui.rep.period = ui.rep.fast
		if ui.rep.next.Before(gtx.Now) {
			ui.rep.next = gtx.Now.Add(ui.rep.period)
		}
	}
	if !gtx.Now.Before(ui.rep.next) {
		if !ui.performFileViewerKeyScroll(gtx.Now, name) {
			ui.rep.active = false
			ui.rep.pane = -1
			return
		}
		ui.rep.next = gtx.Now.Add(ui.rep.period)
	}
	gtx.Execute(op.InvalidateCmd{At: ui.rep.next})
}

func (ui *UI) startFileViewerLoad(now time.Time) {
	ui.startFileViewerLoadWithOptions(now, false)
}

func (ui *UI) restartFileViewerLoad(now time.Time) {
	ui.startFileViewerLoadWithOptions(now, true)
}

func (ui *UI) startFileViewerLoadWithOptions(now time.Time, force bool) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	if force && st.loadCancel != nil {
		st.loadCancel()
		st.loadCancel = nil
		st.loading = false
	}
	if st.loading {
		return
	}

	cfg := fm.ViewerConfig{
		Mode:    "file",
		Shell:   "auto",
		Command: "cat {path}",
	}
	if ui != nil && ui.fmCfg != nil {
		cfg = ui.fmCfg.Viewer
	}
	st.mode = normalizeViewerMode(st.mode)
	st.command = strings.TrimSpace(st.command)
	st.fileEncoding = fm.NormalizeViewerFileEncoding(st.fileEncoding)
	if st.command == "" {
		st.command = "cat {path}"
	}
	cfg.Mode = st.mode
	cfg.Command = st.command
	cfg.CommandAutoRefresh = st.autoRefresh
	cfg.FileEncoding = st.fileEncoding
	st.wordSelectRE, st.wordSelectExpr = viewerWordSelectRegexp(ui.fmCfg)
	st.commandInfinite = st.mode == "command" && viewerCommandLooksInfinite(st.command)
	if !st.commandEditOn {
		st.commandEditor.SetText(st.command)
	}
	if st.mode == "hex" {
		ui.startHexViewerLoad(st, force)
		return
	}

	maxBytes := viewerMaxLoadBytes(ui.fmCfg)

	st.seq++
	seq := st.seq
	st.loading = true
	if st.updatedAt.IsZero() && st.content == "" {
		st.err = ""
		st.status = "loading..."
	}
	st.nextWatchCheck = time.Time{}
	if st.loadCancel != nil {
		st.loadCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	st.loadCancel = cancel
	path := st.path
	ch := st.resultCh
	remote := st.remote

	go func() {
		progress := func(content, status string) {
			sendViewerResult(ch, fileViewerResult{
				seq:     seq,
				content: content,
				status:  status,
				partial: true,
			})
		}
		content, status, err, info := readViewerContent(ctx, path, cfg, maxBytes, remote, progress)
		res := fileViewerResult{
			seq:           seq,
			content:       content,
			status:        status,
			err:           err,
			encoding:      info.encoding,
			encodingBOM:   info.encodingBOM,
			imagePreview:  info.imagePreview,
			image:         info.image,
			imageData:     info.imageData,
			imageFormat:   info.imageFormat,
			imageSize:     info.imageSize,
			binaryPreview: info.binaryPreview,
			lineEnding:    info.lineEnding,
			binaryData:    info.binaryData,
			final:         true,
		}
		sendViewerResult(ch, res)
	}()

	_ = now
}

func (st *fileViewerState) markUpdated(now time.Time) {
	if st == nil || now.IsZero() {
		return
	}
	st.updatedAt = now
}

func sendViewerResult(ch chan fileViewerResult, res fileViewerResult) {
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

func (ui *UI) pumpFileViewerState(gtx layout.Context) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	ui.pumpHexViewerState(gtx, st)
	ui.pumpFileViewerFindState(gtx, st)
	if st.resultCh == nil {
		return
	}
	if st.pendingUpdate && !st.userIsBrowsing(gtx.Now) {
		st.pendingUpdate = false
		st.err = st.pendingErr
		st.status = st.pendingStatus
		st.detectedEncoding = st.pendingEncoding
		st.detectedEncodingBOM = st.pendingEncodingBOM
		st.detectedImagePreview = st.pendingImagePreview
		st.imagePreview = st.pendingImage
		st.imagePreviewData = st.pendingImageData
		st.imagePreviewFormat = st.pendingImageFormat
		st.imagePreviewSize = st.pendingImageSize
		st.detectedBinaryPreview = st.pendingBinaryPreview
		st.detectedLineEnding = st.pendingLineEnding
		st.binaryPreviewData = st.pendingBinaryData
		if st.detectedImagePreview {
			st.imageView.reset()
			st.closeEncodingMenu()
			st.binaryPreviewCols = 0
		} else if st.detectedBinaryPreview {
			st.binaryPreviewCols = viewerBinaryPreviewBytes
		} else {
			st.binaryPreviewCols = 0
		}
		st.pendingEncoding = ""
		st.pendingEncodingBOM = false
		st.pendingImagePreview = false
		st.pendingImage = nil
		st.pendingImageData = nil
		st.pendingImageFormat = ""
		st.pendingImageSize = image.Point{}
		st.pendingBinaryPreview = false
		st.pendingLineEnding = ""
		st.pendingBinaryData = nil
		if st.status == "" {
			st.status = "ready"
		}
		applyFileViewerContentResult(st, st.pendingContent)
		st.restorePendingStreamSelection()
		if !viewerSupportsFind(st) && st.find.open {
			ui.closeFileViewerFind()
		}
		ui.refreshFileViewerFind(gtx.Now, true)
		st.markUpdated(gtx.Now)
		st.captureWatchState()
		gtx.Execute(op.InvalidateCmd{})
	}

	for {
		select {
		case res := <-st.resultCh:
			if res.seq != st.seq {
				continue
			}
			if res.partial && !res.final {
				if res.status != "" {
					st.status = res.status
				}
				// Keep finite command outputs stable during refresh.
				// Applying partial chunks causes transient content shrink/expand,
				// which resets vertical position and makes the scrollbar flicker.
				if st.mode == "command" && !st.commandInfinite {
					gtx.Execute(op.InvalidateCmd{})
					continue
				}
				if viewerUpdateAction(st, res.content, false, nil, false, nil) != viewerUpdateSame {
					applyFileViewerContentResult(st, res.content)
					ui.refreshFileViewerFind(gtx.Now, true)
					st.markUpdated(gtx.Now)
				}
				st.restorePendingStreamSelection()
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			st.loading = false
			st.loadCancel = nil
			st.err = res.err
			st.status = res.status
			if st.status == "" {
				st.status = "ready"
			}
			updateAction := viewerUpdateAction(st, res.content, res.imagePreview, res.imageData, res.binaryPreview, res.binaryData)
			contentToApply := res.content
			if updateAction == viewerUpdateSame && res.binaryPreview && st.detectedBinaryPreview {
				contentToApply = st.content
			}
			if updateAction == viewerUpdateReplace && (st.userIsBrowsing(gtx.Now) || st.stream.hasSelection()) {
				st.pendingUpdate = true
				st.pendingContent = contentToApply
				st.pendingStatus = st.status
				st.pendingErr = st.err
				st.pendingEncoding = res.encoding
				st.pendingEncodingBOM = res.encodingBOM
				st.pendingImagePreview = res.imagePreview
				st.pendingImage = res.image
				st.pendingImageData = append([]byte(nil), res.imageData...)
				st.pendingImageFormat = res.imageFormat
				st.pendingImageSize = res.imageSize
				st.pendingBinaryPreview = res.binaryPreview
				st.pendingLineEnding = res.lineEnding
				st.pendingBinaryData = append([]byte(nil), res.binaryData...)
				st.status = "update pending"
				ui.scheduleFileViewerWatch(gtx)
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			st.pendingUpdate = false
			st.pendingEncoding = ""
			st.pendingEncodingBOM = false
			st.pendingImagePreview = false
			st.pendingImage = nil
			st.pendingImageData = nil
			st.pendingImageFormat = ""
			st.pendingImageSize = image.Point{}
			st.pendingBinaryPreview = false
			st.pendingLineEnding = ""
			st.pendingBinaryData = nil
			st.detectedEncoding = res.encoding
			st.detectedEncodingBOM = res.encodingBOM
			st.detectedImagePreview = res.imagePreview
			st.imagePreview = res.image
			st.imagePreviewData = append([]byte(nil), res.imageData...)
			st.imagePreviewFormat = res.imageFormat
			st.imagePreviewSize = res.imageSize
			st.detectedBinaryPreview = res.binaryPreview
			st.detectedLineEnding = res.lineEnding
			st.binaryPreviewData = append([]byte(nil), res.binaryData...)
			if st.detectedImagePreview {
				if updateAction != viewerUpdateSame {
					st.imageView.reset()
				}
				st.closeEncodingMenu()
				st.binaryPreviewCols = 0
			} else if st.detectedBinaryPreview {
				if updateAction != viewerUpdateSame {
					st.binaryPreviewCols = viewerBinaryPreviewBytes
				}
			} else {
				st.binaryPreviewCols = 0
			}
			applyFileViewerContentResult(st, contentToApply)
			st.restorePendingStreamSelection()
			if !viewerSupportsFind(st) && st.find.open {
				ui.closeFileViewerFind()
			}
			ui.refreshFileViewerFind(gtx.Now, true)
			st.markUpdated(gtx.Now)
			st.captureWatchState()
			ui.scheduleFileViewerWatch(gtx)
			gtx.Execute(op.InvalidateCmd{})
		default:
			return
		}
	}
}

func (ui *UI) scheduleFileViewerWatch(gtx layout.Context) {
	st := ui.fileViewer
	if st == nil || st.loading {
		return
	}

	interval := 500 * time.Millisecond
	if st.mode == "command" {
		if st.commandInfinite {
			return
		}
		if !st.autoRefresh || st.commandEditOn {
			return
		}
		interval = viewerCommandRefreshInterval(ui.fmCfg)
		if st.userIsBrowsing(gtx.Now) {
			st.nextWatchCheck = gtx.Now.Add(interval)
			gtx.Execute(op.InvalidateCmd{At: st.nextWatchCheck})
			return
		}
	}
	if st.nextWatchCheck.IsZero() {
		st.nextWatchCheck = gtx.Now.Add(interval)
	}
	if !gtx.Now.Before(st.nextWatchCheck) {
		st.nextWatchCheck = gtx.Now.Add(interval)
		if st.mode == "command" {
			st.nextWatchCheck = time.Time{}
			ui.startFileViewerLoad(gtx.Now)
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(33 * time.Millisecond)})
			return
		}
		if st.watchChanged() {
			st.nextWatchCheck = time.Time{}
			ui.startFileViewerLoad(gtx.Now)
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(33 * time.Millisecond)})
			return
		}
	}
	gtx.Execute(op.InvalidateCmd{At: st.nextWatchCheck})
}

func applyFileViewerContentResult(st *fileViewerState, next string) {
	if st == nil {
		return
	}
	prev := st.content
	if prev == next {
		return
	}
	commandMode := st.mode == "command"

	if commandMode && prev != "" && strings.HasPrefix(next, prev) {
		tail := next[len(prev):]
		followBottom := st.stream.nearBottom() && !st.stream.hasSelection()
		st.content = next
		if tail == "" {
			return
		}
		st.stream.Append(tail)
		if followBottom {
			st.stream.scrollToBottom()
		}
		return
	}

	st.content = next
	st.stream.SetContent(next)
}

const (
	viewerUpdateSame = iota
	viewerUpdateAppend
	viewerUpdateReplace
)

func viewerUpdateAction(st *fileViewerState, next string, nextImagePreview bool, nextImageData []byte, nextBinaryPreview bool, nextBinaryData []byte) int {
	if st == nil {
		return viewerUpdateReplace
	}
	if st.detectedImagePreview || nextImagePreview {
		if st.detectedImagePreview && nextImagePreview && bytes.Equal(st.imagePreviewData, nextImageData) {
			return viewerUpdateSame
		}
		return viewerUpdateReplace
	}
	if st.detectedBinaryPreview || nextBinaryPreview {
		if st.detectedBinaryPreview && nextBinaryPreview && bytes.Equal(st.binaryPreviewData, nextBinaryData) {
			return viewerUpdateSame
		}
		return viewerUpdateReplace
	}
	prev := st.content
	if next == prev {
		return viewerUpdateSame
	}
	if st.mode == "command" && prev != "" && strings.HasPrefix(next, prev) {
		return viewerUpdateAppend
	}
	return viewerUpdateReplace
}

func viewerSelectionNearEnd(start, end, total int) bool {
	if total <= 0 {
		return true
	}
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	if end > total {
		end = total
	}
	const tailThreshold = 64
	return total-end <= tailThreshold
}

func clampViewerCaret(pos, total int) int {
	if pos < 0 {
		return 0
	}
	if pos > total {
		return total
	}
	return pos
}

func viewerPointInRect(pos image.Point, rect image.Rectangle) bool {
	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		return false
	}
	return pos.X >= rect.Min.X && pos.X < rect.Max.X && pos.Y >= rect.Min.Y && pos.Y < rect.Max.Y
}

func viewerTotalLines(content string) int {
	if content == "" {
		return 1
	}
	return strings.Count(content, "\n") + 1
}

func viewerLineStartRune(content string, line int) int {
	if line <= 0 || content == "" {
		return 0
	}
	curLine := 0
	runePos := 0
	for _, r := range content {
		if curLine >= line {
			break
		}
		runePos++
		if r == '\n' {
			curLine++
		}
	}
	return runePos
}

func viewerScrollToLine(st *fileViewerState, line int) {
	if st == nil {
		return
	}
	total := viewerTotalLines(st.content)
	if line < 0 {
		line = 0
	}
	if line > total-1 {
		line = total - 1
	}
	pos := viewerLineStartRune(st.content, line)
	st.contentEditor.SetCaret(pos, pos)
}

func viewerScrollByLines(st *fileViewerState, lines int) bool {
	if st == nil || lines == 0 {
		return false
	}
	if st.mode == "hex" {
		if st.hex == nil {
			return false
		}
		before := st.hex.topLine
		st.hex.topLine += int64(lines)
		st.hex.clampTop()
		return st.hex.topLine != before
	}
	before := st.stream.topLine
	st.stream.scrollByLines(lines)
	return st.stream.topLine != before
}

func viewerScrollByPage(st *fileViewerState, pages int) bool {
	if st == nil || pages == 0 {
		return false
	}
	lines := viewerPageScrollLines(st)
	if lines < 1 {
		lines = 1
	}
	return viewerScrollByLines(st, pages*lines)
}

func viewerPageScrollLines(st *fileViewerState) int {
	if st == nil {
		return 1
	}
	visible := 1
	if st.mode == "hex" {
		if st.hex != nil && st.hex.visibleLines > 0 {
			visible = st.hex.visibleLines
		}
	} else if st.stream.visibleLines > 0 {
		visible = st.stream.visibleLines
	}
	if visible <= 1 {
		return 1
	}
	return visible - 1
}

func viewerScrollToStart(st *fileViewerState) bool {
	if st == nil {
		return false
	}
	if st.mode == "hex" {
		if st.hex == nil {
			return false
		}
		if st.hex.topLine == 0 {
			return false
		}
		st.hex.topLine = 0
		st.hex.clampTop()
		return true
	}
	if st.stream.topLine == 0 {
		return false
	}
	st.stream.topLine = 0
	st.stream.clampTop()
	st.stream.syncVisualTop()
	return true
}

func viewerScrollToEnd(st *fileViewerState) bool {
	if st == nil {
		return false
	}
	if st.mode == "hex" {
		if st.hex == nil {
			return false
		}
		visible := st.hex.visibleLines
		if visible < 1 {
			visible = 1
		}
		maxTop := st.hex.totalLines() - int64(visible)
		if maxTop < 0 {
			maxTop = 0
		}
		if st.hex.topLine == maxTop {
			return false
		}
		st.hex.topLine = maxTop
		st.hex.clampTop()
		return true
	}
	totalLines := len(st.stream.lines)
	if totalLines < 1 {
		totalLines = 1
	}
	visible := st.stream.visibleLines
	if visible < 1 {
		visible = 1
	}
	maxTop := totalLines - visible
	if maxTop < 0 {
		maxTop = 0
	}
	if st.stream.topLine == maxTop {
		return false
	}
	st.stream.topLine = maxTop
	st.stream.clampTop()
	st.stream.syncVisualTop()
	return true
}

func viewerScrollByDelta(st *fileViewerState, delta float32) {
	if st == nil || delta == 0 {
		return
	}
	if st.mode == "hex" {
		if st.hex != nil {
			st.hex.scrollByDelta(delta)
		}
		return
	}
	st.stream.scrollByDelta(delta)
}

func viewerScrollFromScrollbarPos(st *fileViewerState, y int) {
	if st == nil || !st.scrollbarVisible {
		return
	}
	track := st.scrollbarTrack
	thumb := st.scrollbarThumb
	if track.Dy() <= 0 || thumb.Dy() <= 0 {
		return
	}
	maxTop := st.scrollbarLines - st.scrollbarVisibleN
	if maxTop <= 0 {
		return
	}
	maxTravel := track.Dy() - thumb.Dy()
	if maxTravel <= 0 {
		return
	}
	dragY := y - track.Min.Y - thumb.Dy()/2
	if dragY < 0 {
		dragY = 0
	}
	if dragY > maxTravel {
		dragY = maxTravel
	}
	ratio := float32(dragY) / float32(maxTravel)
	top := int(ratio*float32(maxTop) + 0.5)
	targetLine := top + st.scrollbarVisibleN/2
	viewerScrollToLine(st, targetLine)
}

func (st *fileViewerState) markUserBrowsing(now time.Time) {
	if st == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	st.userBrowseUntil = now.Add(2500 * time.Millisecond)
}

func (st *fileViewerState) userIsBrowsing(now time.Time) bool {
	if st == nil || st.userBrowseUntil.IsZero() {
		return false
	}
	return now.Before(st.userBrowseUntil)
}

func (st *fileViewerState) updateScrollbarHover(pos image.Point) {
	if st == nil || !st.scrollbarVisible {
		st.scrollbarHover = false
		return
	}
	st.scrollbarHover = viewerPointInRect(pos, st.scrollbarTrack)
}

func (st *fileViewerState) captureWatchState() {
	if st == nil {
		return
	}
	info, err := st.statPath()
	if err != nil {
		st.watchExists = false
		st.watchSize = 0
		st.watchModTime = time.Time{}
		return
	}
	st.watchExists = true
	st.watchSize = info.Size()
	st.watchModTime = info.ModTime()
}

func (st *fileViewerState) watchChanged() bool {
	if st == nil {
		return false
	}
	info, err := st.statPath()
	if err != nil {
		changed := st.watchExists
		st.watchExists = false
		st.watchSize = 0
		st.watchModTime = time.Time{}
		return changed
	}
	changed := !st.watchExists || st.watchSize != info.Size() || !st.watchModTime.Equal(info.ModTime())
	st.watchExists = true
	st.watchSize = info.Size()
	st.watchModTime = info.ModTime()
	return changed
}

func (st *fileViewerState) statPath() (os.FileInfo, error) {
	if st == nil {
		return nil, errors.New("viewer state is nil")
	}
	if st.remote == nil {
		if filesys.ArchiveMemberPath(st.path) {
			return filesys.StatLocalPath(st.path)
		}
		return os.Stat(st.path)
	}
	client := st.remote.sftpClient()
	if client == nil {
		return nil, errors.New("sftp session is not connected")
	}
	return client.Stat(st.path)
}

func (ui *UI) viewerTextSize() unit.Sp {
	if ui == nil || ui.fmCfg == nil {
		return normalizeUIFontSize(13)
	}
	if ui.fmCfg.Viewer.FontSizeSp < 6 {
		return scaleConfigFontSize(ui.fmCfg, 13)
	}
	return normalizeUIFontSize(unit.Sp(ui.fmCfg.Viewer.FontSizeSp))
}

func viewerMaxLoadBytes(cfg *fm.Config) int {
	mb := float64(viewerDefaultMaxReadMB)
	if cfg != nil && cfg.Viewer.MaxReadMB > 0 {
		mb = float64(cfg.Viewer.MaxReadMB)
	}
	if mb <= 0 {
		mb = viewerDefaultMaxReadMB
	}
	bytes := int(mb * 1024 * 1024)
	if bytes < 1 {
		return viewerDefaultMaxLoadBytes
	}
	return bytes
}

func viewerWordSelectRegexp(cfg *fm.Config) (*regexp.Regexp, string) {
	pattern := viewerDefaultWordRegex
	if cfg != nil {
		if raw := strings.TrimSpace(cfg.Viewer.WordSelectRegex); raw != "" {
			pattern = raw
		}
	}
	re, err := regexp.Compile(pattern)
	if err == nil {
		return re, pattern
	}
	return regexp.MustCompile(viewerDefaultWordRegex), viewerDefaultWordRegex
}

func viewerCommandAutoRefresh(cfg *fm.Config) bool {
	if cfg == nil {
		return viewerDefaultAutoRefresh
	}
	return cfg.Viewer.CommandAutoRefresh
}

func viewerWordWrap(cfg *fm.Config) bool {
	if cfg == nil {
		return false
	}
	return cfg.Viewer.WordWrap
}

func viewerSmoothScrolling(cfg *fm.Config) bool {
	if cfg == nil {
		return true
	}
	return cfg.Viewer.SmoothScrolling
}

func viewerCommandRefreshInterval(cfg *fm.Config) time.Duration {
	ms := viewerDefaultRefreshMs
	if cfg != nil && cfg.Viewer.CommandRefreshMs > 0 {
		ms = cfg.Viewer.CommandRefreshMs
	}
	if ms < viewerMinimumRefreshMs {
		ms = viewerMinimumRefreshMs
	}
	return time.Duration(ms) * time.Millisecond
}

func viewerCommandLooksInfinite(cmdline string) bool {
	s := strings.ToLower(cmdline)
	return strings.Contains(s, "tail -f") ||
		strings.Contains(s, "tail --follow") ||
		strings.Contains(s, "tailf ") ||
		strings.Contains(s, "tailf\t") ||
		strings.Contains(s, "journalctl -f") ||
		strings.Contains(s, "watch ")
}

func (ui *UI) refreshFileViewerNow(now time.Time) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	if ui != nil && ui.fmCfg != nil {
		st.mode, st.command = ui.viewerConfiguredModeAndCommand(st.path, st.remote, ui.fmCfg.Viewer.Mode, ui.fmCfg.Viewer.Command)
		st.autoRefresh = ui.fmCfg.Viewer.CommandAutoRefresh
		st.fileEncoding = fm.NormalizeViewerFileEncoding(ui.fmCfg.Viewer.FileEncoding)
	}
	st.commandEditOn = false
	st.commandFocus = false
	st.historyOpen = false
	st.closeEncodingMenu()
	st.commandInfinite = st.mode == "command" && viewerCommandLooksInfinite(st.command)
	st.commandEditor.SetText(st.command)
	st.nextWatchCheck = time.Time{}
	ui.restartFileViewerLoad(now)
}

func normalizeViewerMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "command", "hex", "file":
		return mode
	default:
		return "file"
	}
}

func viewerTabIndex(key string) int {
	switch key {
	case "hex":
		return 1
	case "command":
		return 2
	case "history":
		return 3
	default:
		return 0
	}
}

func viewerStepMode(mode string, step int) string {
	order := [...]string{"file", "hex", "command"}
	current := normalizeViewerMode(mode)
	idx := 0
	for i, candidate := range order {
		if candidate == current {
			idx = i
			break
		}
	}
	if step == 0 {
		return order[idx]
	}
	n := len(order)
	next := (idx + step) % n
	if next < 0 {
		next += n
	}
	return order[next]
}

func viewerModeTabStep(mods key.Modifiers) (int, bool) {
	switch mods {
	case 0:
		return 1, true
	case key.ModShift:
		return -1, true
	default:
		return 0, false
	}
}

func (st *fileViewerState) activeTabKey() string {
	if st == nil {
		return "file"
	}
	if st.historyOpen {
		return "history"
	}
	return normalizeViewerMode(st.mode)
}

func (st *fileViewerState) setHistoryOpen(open bool, now time.Time) {
	if st == nil || st.historyOpen == open {
		return
	}
	if open {
		st.closeEncodingMenu()
	}
	prev := st.activeTabKey()
	st.historyOpen = open
	if next := st.activeTabKey(); next != prev {
		st.tabPrev = prev
		st.tabAnimAt = now
	}
}

func (st *fileViewerState) tabFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" {
		return 0, false
	}
	current := st.activeTabKey()
	if st.tabPrev == "" || st.tabAnimAt.IsZero() || st.tabPrev == current {
		if key == current {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(st.tabAnimAt)
	if elapsed >= toolbarAnimDur {
		st.tabPrev = ""
		st.tabAnimAt = time.Time{}
		if key == current {
			return 1, false
		}
		return 0, false
	}
	t := smoothstep01(clamp01(float32(elapsed) / float32(toolbarAnimDur)))
	if key == current {
		return t, true
	}
	if key == st.tabPrev {
		return 1 - t, true
	}
	return 0, true
}

func (st *fileViewerState) tabPosition(now time.Time) (float32, bool) {
	if st == nil {
		return 0, false
	}
	current := float32(viewerTabIndex(st.activeTabKey()))
	if st.tabPrev == "" || st.tabAnimAt.IsZero() || st.tabPrev == st.activeTabKey() {
		return current, false
	}
	prev := float32(viewerTabIndex(st.tabPrev))
	elapsed := now.Sub(st.tabAnimAt)
	if elapsed >= toolbarAnimDur {
		st.tabPrev = ""
		st.tabAnimAt = time.Time{}
		return current, false
	}
	t := smoothstep01(clamp01(float32(elapsed) / float32(toolbarAnimDur)))
	return prev + (current-prev)*t, true
}

func (ui *UI) setFileViewerMode(mode string, now time.Time) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	mode = normalizeViewerMode(mode)
	if mode == st.mode && !st.historyOpen {
		return
	}
	prevTab := st.activeTabKey()
	st.prepareStreamSelectionForMode(mode)
	st.mode = mode
	st.commandEditOn = false
	st.commandFocus = false
	st.historyOpen = false
	st.closeEncodingMenu()
	if nextTab := st.activeTabKey(); nextTab != prevTab {
		st.tabPrev = prevTab
		st.tabAnimAt = now
	}
	if st.mode == "command" {
		st.command = ui.viewerCommandForTarget(st.path, st.remote, st.command)
		if st.command == "" {
			st.command = "cat {path}"
		}
		st.commandEditor.SetText(st.command)
		st.commandInfinite = viewerCommandLooksInfinite(st.command)
	} else {
		st.commandInfinite = false
	}
	if ui.fmCfg != nil {
		ui.fmCfg.Viewer.Mode = st.mode
		if st.mode == "command" {
			ui.fmCfg.Viewer.Command = st.command
		}
		if err := ui.saveFMConfig(); err != nil {
			st.err = err.Error()
			return
		}
	}
	st.nextWatchCheck = time.Time{}
	ui.refreshFileViewerFind(now, false)
	ui.restartFileViewerLoad(now)
}

func (ui *UI) toggleFileViewerAutoRefresh(now time.Time) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	st.autoRefresh = !st.autoRefresh
	if ui.fmCfg != nil {
		ui.fmCfg.Viewer.CommandAutoRefresh = st.autoRefresh
		if err := ui.saveFMConfig(); err != nil {
			st.err = err.Error()
		}
	}
	st.nextWatchCheck = time.Time{}
	if st.autoRefresh && st.mode == "command" && !st.commandInfinite {
		ui.startFileViewerLoad(now)
	}
}

func (ui *UI) startViewerCommandEdit(now time.Time) {
	st := ui.fileViewer
	if st == nil || st.mode != "command" {
		return
	}
	st.setHistoryOpen(false, now)
	st.closeEncodingMenu()
	st.commandEditOn = true
	st.commandFocus = true
	st.commandEditor.SetText(st.command)
	st.commandEditor.SetCaret(st.commandEditor.Len(), st.commandEditor.Len())
}

func (ui *UI) cancelViewerCommandEdit() {
	st := ui.fileViewer
	if st == nil || !st.commandEditOn {
		return
	}
	st.commandEditOn = false
	st.commandFocus = false
	st.commandEditor.SetText(st.command)
	ui.closeEditorContextMenu()
}

func (ui *UI) applyViewerCommandEdit(now time.Time) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	cmd := strings.TrimSpace(st.commandEditor.Text())
	if cmd == "" {
		st.err = "viewer command is empty"
		return
	}
	st.command = cmd
	prevTab := st.activeTabKey()
	st.prepareStreamSelectionForMode("command")
	st.mode = "command"
	st.commandInfinite = viewerCommandLooksInfinite(st.command)
	st.commandEditOn = false
	st.commandFocus = false
	st.setHistoryOpen(false, now)
	if nextTab := st.activeTabKey(); nextTab != prevTab {
		st.tabPrev = prevTab
		st.tabAnimAt = now
	}
	if err := ui.rememberViewerCommand(st, cmd); err != nil {
		st.err = err.Error()
		return
	}
	ui.restartFileViewerLoad(now)
}

func (ui *UI) applyViewerHistoryCommand(cmd string, now time.Time) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	prevTab := st.activeTabKey()
	st.prepareStreamSelectionForMode("command")
	st.mode = "command"
	st.command = cmd
	st.commandInfinite = viewerCommandLooksInfinite(cmd)
	st.commandEditOn = false
	st.commandFocus = false
	st.setHistoryOpen(false, now)
	if nextTab := st.activeTabKey(); nextTab != prevTab {
		st.tabPrev = prevTab
		st.tabAnimAt = now
	}
	st.commandEditor.SetText(cmd)
	if err := ui.rememberViewerCommand(st, cmd); err != nil {
		st.err = err.Error()
		return
	}
	ui.restartFileViewerLoad(now)
}

func (ui *UI) rememberViewerCommand(st *fileViewerState, cmd string) error {
	if st == nil {
		return nil
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	if ui.fmCfg == nil {
		return nil
	}
	ui.fmCfg.Viewer.Mode = "command"
	ui.fmCfg.Viewer.Command = cmd

	key := viewerCommandTargetKey(st.path, st.remote)
	if key != "" {
		if ui.fmCfg.Viewer.CommandByTarget == nil {
			ui.fmCfg.Viewer.CommandByTarget = make(map[string]string, 8)
		}
		ui.fmCfg.Viewer.CommandByTarget[key] = cmd
	}

	history := make([]string, 0, len(ui.fmCfg.Viewer.CommandHistory)+1)
	history = append(history, cmd)
	for _, item := range ui.fmCfg.Viewer.CommandHistory {
		item = strings.TrimSpace(item)
		if item == "" || item == cmd {
			continue
		}
		history = append(history, item)
		if len(history) >= viewerCommandHistoryLimit {
			break
		}
	}
	ui.fmCfg.Viewer.CommandHistory = history
	return ui.saveFMConfig()
}

func (ui *UI) viewerConfiguredModeAndCommand(path string, remote *paneSSHSession, fallbackMode, fallbackCommand string) (string, string) {
	mode := normalizeViewerMode(fallbackMode)
	if remote == nil && filesys.ArchiveMemberPath(path) && mode == "command" {
		mode = "file"
	}
	cmd, matchedRule := ui.viewerDefaultCommand(path, remote, fallbackCommand)
	if remote == nil && filesys.ArchiveMemberPath(path) {
		return mode, cmd
	}
	if matchedRule {
		mode = "command"
	}
	return mode, cmd
}

func (ui *UI) viewerDefaultCommand(path string, remote *paneSSHSession, fallback string) (string, bool) {
	cmd := strings.TrimSpace(fallback)
	if cmd == "" {
		cmd = "cat {path}"
	}
	matchedRule := false
	if ui != nil && ui.fmCfg != nil {
		if byRule, ok := ui.viewerRuleCommand(path, remote); ok {
			cmd = byRule
			matchedRule = true
		}
	}
	return ui.viewerCommandForTarget(path, remote, cmd), matchedRule
}

func (ui *UI) viewerRuleCommand(path string, remote *paneSSHSession) (string, bool) {
	if ui == nil || ui.fmCfg == nil || len(ui.fmCfg.Viewer.CommandRules) == 0 {
		return "", false
	}
	name := viewerCommandMatchName(path, remote)
	if name == "" {
		return "", false
	}
	cmd, ok := fm.MatchViewerCommandRules(ui.fmCfg.Viewer.CommandRules, name)
	if !ok {
		return "", false
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "", false
	}
	return cmd, true
}

func (ui *UI) viewerCommandForTarget(path string, remote *paneSSHSession, fallback string) string {
	cmd := strings.TrimSpace(fallback)
	if cmd == "" {
		cmd = "cat {path}"
	}
	if ui == nil || ui.fmCfg == nil {
		return cmd
	}
	key := viewerCommandTargetKey(path, remote)
	if key == "" || len(ui.fmCfg.Viewer.CommandByTarget) == 0 {
		return cmd
	}
	if byTarget := strings.TrimSpace(ui.fmCfg.Viewer.CommandByTarget[key]); byTarget != "" {
		return byTarget
	}
	return cmd
}

func viewerCommandMatchName(path string, remote *paneSSHSession) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if remote != nil {
		return pathpkg.Base(path)
	}
	return filepath.Base(path)
}

func (ui *UI) viewerHistoryCommands(current string) []string {
	if ui == nil || ui.fmCfg == nil || len(ui.fmCfg.Viewer.CommandHistory) == 0 {
		return nil
	}
	current = strings.TrimSpace(current)
	out := make([]string, 0, len(ui.fmCfg.Viewer.CommandHistory))
	for _, raw := range ui.fmCfg.Viewer.CommandHistory {
		cmd := strings.TrimSpace(raw)
		if cmd == "" || cmd == current {
			continue
		}
		out = append(out, cmd)
	}
	return out
}

func viewerCommandTargetKey(path string, remote *paneSSHSession) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if remote == nil {
		clean := filepath.Clean(path)
		if runtime.GOOS == "windows" {
			clean = strings.ToLower(clean)
		}
		return "local:" + clean
	}
	clean := pathpkg.Clean(path)
	host := strings.TrimSpace(remote.setup.Host)
	user := strings.TrimSpace(remote.setup.User)
	port := remote.setup.Port
	if port <= 0 {
		port = 22
	}
	return fmt.Sprintf("ssh:%s@%s:%d:%s", user, host, port, clean)
}

func (st *fileViewerState) historyClickable(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.historyClicks == nil {
		st.historyClicks = make(map[string]*widget.Clickable, 12)
	}
	if c := st.historyClicks[key]; c != nil {
		return c
	}
	c := new(widget.Clickable)
	st.historyClicks[key] = c
	return c
}

func (ui *UI) toggleViewerWordWrap() {
	st := ui.fileViewer
	if st == nil {
		return
	}
	st.wrapEnabled = !st.wrapEnabled
	if ui.fmCfg != nil {
		ui.fmCfg.Viewer.WordWrap = st.wrapEnabled
		_ = ui.saveFMConfig()
	}
}

func (ui *UI) setFileViewerEncoding(encoding string, now time.Time) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	encoding = fm.NormalizeViewerFileEncoding(encoding)
	if st.fileEncoding == encoding {
		st.closeEncodingMenu()
		return
	}
	st.fileEncoding = encoding
	st.closeEncodingMenu()
	if ui.fmCfg != nil {
		ui.fmCfg.Viewer.FileEncoding = encoding
		if err := ui.saveFMConfig(); err != nil {
			st.err = err.Error()
			return
		}
	}
	st.nextWatchCheck = time.Time{}
	ui.restartFileViewerLoad(now)
}

func readViewerContent(ctx context.Context, path string, cfg fm.ViewerConfig, maxBytes int, remote *paneSSHSession, onProgress func(string, string)) (string, string, string, viewerReadInfo) {
	start := time.Now()
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	switch mode {
	case "command":
		content, status, err := readViewerCommand(ctx, path, cfg, maxBytes, start, remote, onProgress)
		return content, status, err, viewerReadInfo{}
	default:
		return readViewerFile(path, cfg.FileEncoding, maxBytes, start, remote)
	}
}

func readViewerFile(path, encoding string, maxBytes int, _ time.Time, remote *paneSSHSession) (string, string, string, viewerReadInfo) {
	if maxBytes < 1 {
		maxBytes = viewerDefaultMaxLoadBytes
	}

	var (
		size    int64 = -1
		reader  io.ReadCloser
		openErr error
		prefix  = "file"
	)

	if remote == nil {
		if filesys.ArchiveMemberPath(path) {
			prefix = "archive entry"
			info, err := filesys.StatLocalPath(path)
			if err != nil {
				return "", "", err.Error(), viewerReadInfo{}
			}
			if info.IsDir() {
				return "", "", "viewer supports files only", viewerReadInfo{}
			}
			size = info.Size()
			if size > int64(maxBytes) {
				return "", fmt.Sprintf("%s: %d bytes", prefix, size),
					fmt.Sprintf("file too large: %s > %s limit", formatCopySize(size), formatCopySize(int64(maxBytes))), viewerReadInfo{}
			}
			reader, _, openErr = filesys.OpenLocalPath(path)
		} else {
			info, err := os.Stat(path)
			if err != nil {
				return "", "", err.Error(), viewerReadInfo{}
			}
			if info.IsDir() {
				return "", "", "viewer supports files only", viewerReadInfo{}
			}
			size = info.Size()
			if size > int64(maxBytes) {
				return "", fmt.Sprintf("file: %d bytes", size),
					fmt.Sprintf("file too large: %s > %s limit", formatCopySize(size), formatCopySize(int64(maxBytes))), viewerReadInfo{}
			}
			reader, openErr = os.Open(path)
		}
	} else {
		client := remote.sftpClient()
		if client == nil {
			return "", "", "sftp session is not connected", viewerReadInfo{}
		}
		info, err := client.Stat(path)
		if err != nil {
			return "", "", err.Error(), viewerReadInfo{}
		}
		if info.IsDir() {
			return "", "", "viewer supports files only", viewerReadInfo{}
		}
		size = info.Size()
		if size > int64(maxBytes) {
			return "", fmt.Sprintf("remote file: %d bytes", size),
				fmt.Sprintf("file too large: %s > %s limit", formatCopySize(size), formatCopySize(int64(maxBytes))), viewerReadInfo{}
		}
		reader, openErr = client.Open(path)
		prefix = "remote file"
	}
	if openErr != nil {
		return "", "", openErr.Error(), viewerReadInfo{}
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, int64(maxBytes)))
	if err != nil {
		return "", "", err.Error(), viewerReadInfo{}
	}
	if img, ok := decodeViewerImagePreview(path, data); ok {
		if size >= 0 {
			return "", fmt.Sprintf("%s: %d bytes", prefix, size), "", img
		}
		return "", fmt.Sprintf("%s: %d bytes", prefix, len(data)), "", img
	}
	content, info := decodeViewerText(path, data, encoding)
	if !info.binaryPreview {
		info.lineEnding = detectViewerLineEnding(content)
		content = normalizeViewerLineEndings(content)
		content = sanitizeViewerContent(content)
	} else {
		info.binaryData = append([]byte(nil), data...)
	}
	if size >= 0 {
		return content, fmt.Sprintf("%s: %d bytes", prefix, size), "", info
	}
	return content, fmt.Sprintf("%s: %d bytes", prefix, len(data)), "", info
}

func decodeViewerImagePreview(path string, data []byte) (viewerReadInfo, bool) {
	if !viewerLooksPreviewableImage(path, data) {
		return viewerReadInfo{}, false
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return viewerReadInfo{}, false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return viewerReadInfo{}, false
	}
	return viewerReadInfo{
		imagePreview: true,
		image:        img,
		imageData:    append([]byte(nil), data...),
		imageFormat:  normalizeViewerImageFormat(format),
		imageSize:    image.Pt(cfg.Width, cfg.Height),
	}, true
}

func readViewerCommand(ctx context.Context, path string, cfg fm.ViewerConfig, maxBytes int, started time.Time, remote *paneSSHSession, onProgress func(string, string)) (string, string, string) {
	cmdline := strings.TrimSpace(cfg.Command)
	if cmdline == "" {
		return "", "", "viewer command is empty"
	}
	shell := resolveViewerShell(cfg.Shell, remote != nil)
	filename := filepath.Base(path)
	if remote != nil {
		filename = pathpkg.Base(path)
	}
	cmdline = expandViewerCommandTemplate(cmdline, path, filename, shell.quoteFn)
	infinite := viewerCommandLooksInfinite(cmdline)
	timeout := time.Duration(0)
	if !infinite {
		timeout = viewerCommandExecTimeout
	}

	if remote != nil {
		return readViewerRemoteCommand(ctx, remote, cmdline, shell, maxBytes, started, timeout, infinite, onProgress)
	}
	return readViewerLocalCommand(ctx, path, cmdline, shell, maxBytes, started, timeout, infinite, onProgress)
}

func readViewerLocalCommand(ctx context.Context, path, cmdline string, shell viewerShellSpec, maxBytes int, started time.Time, timeout time.Duration, infinite bool, onProgress func(string, string)) (string, string, string) {
	runCtx, cancel := context.WithCancel(ctx)
	if timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	args := append(append([]string{}, shell.args...), cmdline)
	cmd := exec.CommandContext(runCtx, shell.program, args...)
	configureViewerCommandProcess(cmd)
	cmd.Dir = filepath.Clean(filepath.Dir(path))
	buf := newViewerCommandBuffer(maxBytes, cancel)
	cmd.Stdout = buf
	cmd.Stderr = buf
	if err := cmd.Start(); err != nil {
		return "", "", err.Error()
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	lastSent := -1
	lastTruncated := false
	ticker := time.NewTicker(viewerCommandStreamTick)
	defer ticker.Stop()

	var err error
loop:
	for {
		select {
		case err = <-waitCh:
			break loop
		case <-ticker.C:
			lastSent, lastTruncated = emitViewerCommandProgress(onProgress, buf, "command", shell, started, infinite, true, lastSent, lastTruncated)
		case <-runCtx.Done():
			err = <-waitCh
			break loop
		}
	}

	out := buf.Bytes()
	truncated := buf.Truncated()
	_, _ = emitViewerCommandProgress(onProgress, buf, "command", shell, started, infinite, false, lastSent, lastTruncated)
	content := string(bytes.ToValidUTF8(out, []byte("\xef\xbf\xbd")))
	content = sanitizeViewerContent(content)
	if truncated {
		content += "\n\n[truncated]"
	}
	status := viewerCommandStatus("command", shell, started, truncated, infinite, false)
	if err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return content, status, "viewer command timed out"
		}
		if runCtx.Err() == context.Canceled && truncated {
			return content, status, ""
		}
		if runCtx.Err() == context.Canceled && errors.Is(ctx.Err(), context.Canceled) {
			return "", "", "viewer command canceled"
		}
		if viewerCommandTreatsExitAsEmpty(cmdline, content, err) {
			return "", "no output", ""
		}
		return content, status, viewerCommandErrorMessage(err)
	}
	if content == "" {
		return "", "no output", ""
	}
	return content, status, ""
}

func readViewerRemoteCommand(ctx context.Context, remote *paneSSHSession, cmdline string, shell viewerShellSpec, maxBytes int, started time.Time, timeout time.Duration, infinite bool, onProgress func(string, string)) (string, string, string) {
	if remote == nil {
		return "", "", "remote session is not connected"
	}
	client := remote.commandClient()
	if client == nil {
		return "", "", "remote ssh session is not connected"
	}

	runCtx, cancel := context.WithCancel(ctx)
	if timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	session, err := client.NewSession()
	if err != nil {
		return "", "", err.Error()
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", "", err.Error()
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return "", "", err.Error()
	}

	fullCmd := cmdline
	if shell.program != "" {
		args := append(append([]string{}, shell.args...), cmdline)
		fullCmd = shell.program
		for _, arg := range args {
			fullCmd += " " + shellQuote(arg)
		}
	}

	buf := newViewerCommandBuffer(maxBytes, cancel)
	if err := session.Start(fullCmd); err != nil {
		return "", "", err.Error()
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(buf, stdout)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(buf, stderr)
	}()

	waitCh := make(chan error, 1)
	go func() {
		wg.Wait()
		waitCh <- session.Wait()
	}()

	var waitErr error
	lastSent := -1
	lastTruncated := false
	ticker := time.NewTicker(viewerCommandStreamTick)
	defer ticker.Stop()

loop:
	for {
		select {
		case waitErr = <-waitCh:
			break loop
		case <-ticker.C:
			lastSent, lastTruncated = emitViewerCommandProgress(onProgress, buf, "remote command", shell, started, infinite, true, lastSent, lastTruncated)
		case <-runCtx.Done():
			_ = session.Close()
			waitErr = <-waitCh
			break loop
		}
	}

	out := buf.Bytes()
	truncated := buf.Truncated()
	_, _ = emitViewerCommandProgress(onProgress, buf, "remote command", shell, started, infinite, false, lastSent, lastTruncated)
	content := string(bytes.ToValidUTF8(out, []byte("\xef\xbf\xbd")))
	content = sanitizeViewerContent(content)
	if truncated {
		content += "\n\n[truncated]"
	}

	status := viewerCommandStatus("remote command", shell, started, truncated, infinite, false)

	if waitErr != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return content, status, "viewer command timed out"
		}
		if runCtx.Err() == context.Canceled && truncated {
			return content, status, ""
		}
		if runCtx.Err() == context.Canceled && errors.Is(ctx.Err(), context.Canceled) {
			return "", "", "viewer command canceled"
		}
		if viewerCommandTreatsExitAsEmpty(cmdline, content, waitErr) {
			return "", "no output", ""
		}
		return content, status, viewerCommandErrorMessage(waitErr)
	}
	if content == "" {
		return "", "no output", ""
	}
	return content, status, ""
}

func viewerCommandErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if code, ok := viewerCommandExitStatus(err); ok {
		return fmt.Sprintf("command exited with status %d", code)
	}
	return err.Error()
}

func viewerCommandExitStatus(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code := exitErr.ExitCode(); code >= 0 {
			return code, true
		}
	}
	var sshExitErr *ssh.ExitError
	if errors.As(err, &sshExitErr) {
		return sshExitErr.ExitStatus(), true
	}
	return 0, false
}

func viewerCommandTreatsExitAsEmpty(cmdline, content string, err error) bool {
	if strings.TrimSpace(content) != "" {
		return false
	}
	code, ok := viewerCommandExitStatus(err)
	if !ok || code != 1 {
		return false
	}
	return viewerCommandUsesNoMatchExit(cmdline)
}

func viewerCommandUsesNoMatchExit(cmdline string) bool {
	tokens := strings.FieldsFunc(cmdline, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("|&;()", r)
	})
	for i := 0; i < len(tokens); i++ {
		name := viewerCommandTokenName(tokens[i])
		switch name {
		case "grep", "egrep", "fgrep", "rg", "ripgrep", "findstr":
			return true
		case "git":
			if i+1 < len(tokens) && viewerCommandTokenName(tokens[i+1]) == "grep" {
				return true
			}
		}
	}
	return false
}

func viewerCommandTokenName(token string) string {
	token = strings.TrimSpace(strings.Trim(token, `"'`))
	token = strings.TrimSuffix(token, ".exe")
	token = strings.ReplaceAll(token, `\`, "/")
	token = pathpkg.Base(token)
	return strings.ToLower(token)
}

func emitViewerCommandProgress(onProgress func(string, string), buf *viewerCommandBuffer, kind string, shell viewerShellSpec, started time.Time, infinite, running bool, lastLen int, lastTruncated bool) (int, bool) {
	if onProgress == nil || buf == nil {
		return lastLen, lastTruncated
	}
	out := buf.Bytes()
	truncated := buf.Truncated()
	if len(out) == lastLen && truncated == lastTruncated {
		return lastLen, lastTruncated
	}
	content := string(bytes.ToValidUTF8(out, []byte("\xef\xbf\xbd")))
	content = sanitizeViewerContent(content)
	if truncated {
		content += "\n\n[truncated]"
	}
	status := viewerCommandStatus(kind, shell, started, truncated, infinite, running)
	onProgress(content, status)
	return len(out), truncated
}

func viewerCommandStatus(kind string, shell viewerShellSpec, started time.Time, truncated, infinite, running bool) string {
	parts := make([]string, 0, 2)
	if infinite {
		parts = append(parts, "streaming")
	}
	if truncated {
		parts = append(parts, "truncated")
	}
	_ = started
	_ = running
	_ = kind
	_ = shell
	return strings.Join(parts, ", ")
}

type viewerCommandBuffer struct {
	mu         sync.Mutex
	data       []byte
	max        int
	cancel     context.CancelFunc
	cancelOnce sync.Once
	truncated  bool
}

func newViewerCommandBuffer(maxBytes int, cancel context.CancelFunc) *viewerCommandBuffer {
	if maxBytes < 1 {
		maxBytes = 1
	}
	return &viewerCommandBuffer{
		data:   make([]byte, 0, maxBytes),
		max:    maxBytes,
		cancel: cancel,
	}
}

func (b *viewerCommandBuffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	triggerCancel := false
	b.mu.Lock()
	remain := b.max - len(b.data)
	if remain > 0 {
		if len(p) <= remain {
			b.data = append(b.data, p...)
		} else {
			b.data = append(b.data, p[:remain]...)
			b.truncated = true
			triggerCancel = true
		}
	} else {
		b.truncated = true
		triggerCancel = true
	}
	b.mu.Unlock()

	if triggerCancel && b.cancel != nil {
		b.cancelOnce.Do(b.cancel)
	}
	return len(p), nil
}

func (b *viewerCommandBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, len(b.data))
	copy(out, b.data)
	return out
}

func (b *viewerCommandBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}

type viewerShellSpec struct {
	name    string
	program string
	args    []string
	quoteFn func(string) string
}

func resolveViewerShell(raw string, remote bool) viewerShellSpec {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" || mode == "auto" {
		if remote {
			mode = "sh"
		} else if runtime.GOOS == "windows" {
			mode = "powershell"
		} else {
			mode = "sh"
		}
	}
	switch mode {
	case "sh":
		return viewerShellSpec{
			name:    "sh",
			program: "/bin/sh",
			args:    []string{"-lc"},
			quoteFn: shellQuote,
		}
	case "pwsh", "powershell":
		program := "pwsh"
		if runtime.GOOS == "windows" {
			program = "powershell"
		}
		return viewerShellSpec{
			name:    "powershell",
			program: program,
			args:    []string{"-NoProfile", "-NonInteractive", "-Command"},
			quoteFn: powerShellQuote,
		}
	default:
		return resolveViewerShell("auto", remote)
	}
}

func expandViewerCommandTemplate(template, fullpath, filename string, quoteFn func(string) string) string {
	cmdline := strings.TrimSpace(template)
	if quoteFn == nil {
		quoteFn = shellQuote
	}
	cmdline = collapseQuotedViewerPlaceholder(cmdline, "{fullpath}")
	cmdline = collapseQuotedViewerPlaceholder(cmdline, "{path}")
	cmdline = collapseQuotedViewerPlaceholder(cmdline, "{filename}")

	cmdline = strings.ReplaceAll(cmdline, "{fullpath_raw}", fullpath)
	cmdline = strings.ReplaceAll(cmdline, "{path_raw}", fullpath)
	cmdline = strings.ReplaceAll(cmdline, "{filename_raw}", filename)

	cmdline = strings.ReplaceAll(cmdline, "{fullpath}", quoteFn(fullpath))
	cmdline = strings.ReplaceAll(cmdline, "{path}", quoteFn(fullpath))
	cmdline = strings.ReplaceAll(cmdline, "{filename}", quoteFn(filename))
	return cmdline
}

func collapseQuotedViewerPlaceholder(cmdline, placeholder string) string {
	if placeholder == "" {
		return cmdline
	}
	cmdline = strings.ReplaceAll(cmdline, "'"+placeholder+"'", placeholder)
	cmdline = strings.ReplaceAll(cmdline, "\""+placeholder+"\"", placeholder)
	return cmdline
}

type viewerEncodingDecision struct {
	encoding string
	withBOM  bool
}

func decodeViewerText(path string, data []byte, encoding string) (string, viewerReadInfo) {
	decision := chooseViewerEncoding(path, data, encoding)
	info := viewerReadInfo{
		encoding:    decision.encoding,
		encodingBOM: decision.withBOM,
	}
	switch decision.encoding {
	case fm.ViewerFileEncodingUTF16LE:
		return decodeViewerUTF16(data, binary.LittleEndian, []byte{0xFF, 0xFE}, decision.withBOM), info
	case fm.ViewerFileEncodingUTF16BE:
		return decodeViewerUTF16(data, binary.BigEndian, []byte{0xFE, 0xFF}, decision.withBOM), info
	case fm.ViewerFileEncodingCP437:
		return decodeViewerCP437(data), info
	default:
		if decision.encoding == fm.ViewerFileEncodingUTF8 && viewerLooksBinary(path, data) {
			info.binaryPreview = true
			return formatViewerBinaryPreview(data), info
		}
		if decision.withBOM {
			data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
		}
		return string(bytes.ToValidUTF8(data, []byte("\xef\xbf\xbd"))), info
	}
}

func viewerLooksBinary(path string, data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	if bytes.IndexByte(sample, 0) >= 0 {
		return true
	}
	if mediaType := viewerBinaryMediaType(path, sample); mediaType != "" {
		if viewerMediaTypeLooksText(mediaType) {
			return false
		}
		return true
	}
	controls := 0
	for _, b := range sample {
		switch {
		case b == '\n' || b == '\r' || b == '\t' || b == '\f':
		case b >= 0x20 && b <= 0x7E:
		case b < 0x20 || b == 0x7F:
			controls++
		}
	}
	if controls >= 4 && controls*10 >= len(sample) {
		return true
	}
	if !utf8.Valid(sample) {
		decoded := string(bytes.ToValidUTF8(sample, []byte("\xef\xbf\xbd")))
		runes := len([]rune(decoded))
		if runes < 1 {
			runes = 1
		}
		if suspicious := viewerSuspiciousTextRunes(decoded); suspicious > max(1, runes/24) {
			return true
		}
	}
	return false
}

func viewerBinaryMediaType(path string, sample []byte) string {
	if len(sample) > 0 {
		if mediaType := viewerTrimMediaType(http.DetectContentType(sample)); mediaType != "" && mediaType != "application/octet-stream" {
			return mediaType
		}
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		ext = strings.ToLower(pathpkg.Ext(path))
	}
	if ext != "" {
		if mediaType := viewerTrimMediaType(mime.TypeByExtension(ext)); mediaType != "" {
			return mediaType
		}
	}
	return ""
}

func viewerTrimMediaType(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if semi := strings.IndexByte(raw, ';'); semi >= 0 {
		raw = raw[:semi]
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

func viewerMediaTypeLooksText(mediaType string) bool {
	switch {
	case strings.HasPrefix(mediaType, "text/"):
		return true
	}
	switch mediaType {
	case "application/json",
		"application/ld+json",
		"application/xml",
		"application/xhtml+xml",
		"application/javascript",
		"application/x-javascript",
		"application/ecmascript",
		"application/x-sh",
		"application/x-yaml",
		"application/yaml",
		"image/svg+xml":
		return true
	default:
		return false
	}
}

func viewerLooksPreviewableImage(path string, data []byte) bool {
	if len(data) == 0 {
		return false
	}
	return viewerMediaTypeLooksPreviewableImage(viewerBinaryMediaType(path, data))
}

func viewerMediaTypeLooksPreviewableImage(mediaType string) bool {
	switch viewerTrimMediaType(mediaType) {
	case "image/png", "image/jpeg", "image/gif":
		return true
	default:
		return false
	}
}

func normalizeViewerImageFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jpg", "jpeg":
		return "jpeg"
	case "png":
		return "png"
	case "gif":
		return "gif"
	default:
		return strings.ToLower(strings.TrimSpace(format))
	}
}

func viewerSupportsFind(st *fileViewerState) bool {
	if st == nil {
		return false
	}
	if st.mode == "hex" {
		return true
	}
	if st.mode == "command" {
		return true
	}
	return !st.detectedImagePreview
}

func viewerImageZoomFactorForKey(name key.Name, mods key.Modifiers) (float32, bool) {
	if !mods.Contain(key.ModCtrl) && !mods.Contain(key.ModShortcut) {
		return 0, false
	}
	switch name {
	case "+", "=":
		return fileViewerImageZoomFactor, true
	case "-", "_":
		return 1 / fileViewerImageZoomFactor, true
	default:
		return 0, false
	}
}

func formatViewerBinaryPreview(data []byte) string {
	return formatViewerBinaryPreviewWithCols(data, viewerBinaryPreviewBytes)
}

func formatViewerBinaryPreviewWithCols(data []byte, cols int) string {
	if len(data) == 0 {
		return ""
	}
	cols = viewerBinaryPreviewWrapCols(cols)
	var b strings.Builder
	lines := (len(data) + cols - 1) / cols
	b.Grow(len(data) + max(0, lines-1))
	for i, raw := range data {
		if i > 0 && i%cols == 0 {
			b.WriteByte('\n')
		}
		b.WriteByte(viewerBinaryPreviewByte(raw))
	}
	return b.String()
}

func viewerBinaryPreviewWrapCols(cols int) int {
	if cols <= 0 {
		return viewerBinaryPreviewBytes
	}
	if cols > viewerBinaryPreviewMaxCols {
		return viewerBinaryPreviewMaxCols
	}
	return cols
}

func reflowFileViewerBinaryPreview(st *fileViewerState, cols int) bool {
	if st == nil || !st.detectedBinaryPreview || len(st.binaryPreviewData) == 0 {
		return false
	}
	cols = viewerBinaryPreviewWrapCols(cols)
	if cols == st.binaryPreviewCols {
		return false
	}
	next := formatViewerBinaryPreviewWithCols(st.binaryPreviewData, cols)
	st.binaryPreviewCols = cols
	if st.content == next {
		return false
	}
	st.content = next
	st.stream.SetContent(next)
	st.stream.clearSelection()
	return true
}

func viewerBinaryPreviewByte(b byte) byte {
	if b >= 0x20 && b <= 0x7E {
		return b
	}
	return '.'
}

func chooseViewerEncoding(_ string, data []byte, requested string) viewerEncodingDecision {
	requested = fm.NormalizeViewerFileEncoding(requested)
	bomEncoding, hasBOM := viewerEncodingFromBOM(data)
	if hasBOM {
		return viewerEncodingDecision{
			encoding: bomEncoding,
			withBOM:  true,
		}
	}
	if requested != fm.ViewerFileEncodingAuto {
		return viewerEncodingDecision{
			encoding: requested,
		}
	}
	if heuristic := detectViewerUTF16Encoding(data); heuristic != "" {
		return viewerEncodingDecision{encoding: heuristic}
	}
	if legacy := detectViewerLegacyEncoding(data); legacy != "" {
		return viewerEncodingDecision{encoding: legacy}
	}
	return viewerEncodingDecision{encoding: fm.ViewerFileEncodingUTF8}
}

func viewerEncodingFromBOM(data []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return fm.ViewerFileEncodingUTF8, true
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return fm.ViewerFileEncodingUTF16LE, true
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return fm.ViewerFileEncodingUTF16BE, true
	default:
		return "", false
	}
}

func detectViewerUTF16Encoding(data []byte) string {
	if prefix := detectViewerUTF16Prefix(data); prefix != "" {
		return prefix
	}
	sample := len(data)
	if sample > 1024 {
		sample = 1024
	}
	sample -= sample % 2
	if sample < 4 {
		return ""
	}
	data = data[:sample]
	leScore := scoreViewerUTF16Candidate(data, binary.LittleEndian)
	beScore := scoreViewerUTF16Candidate(data, binary.BigEndian)
	switch {
	case leScore >= 20 && leScore >= beScore+4:
		return fm.ViewerFileEncodingUTF16LE
	case beScore >= 20 && beScore >= leScore+4:
		return fm.ViewerFileEncodingUTF16BE
	default:
		return ""
	}
}

func detectViewerUTF16Prefix(data []byte) string {
	sample := len(data)
	if sample > 8 {
		sample = 8
	}
	sample -= sample % 2
	if sample < 4 {
		return ""
	}
	data = data[:sample]
	if viewerUTF16PrefixMatches(data, binary.LittleEndian) {
		return fm.ViewerFileEncodingUTF16LE
	}
	if viewerUTF16PrefixMatches(data, binary.BigEndian) {
		return fm.ViewerFileEncodingUTF16BE
	}
	return ""
}

func viewerUTF16PrefixMatches(data []byte, order binary.ByteOrder) bool {
	pairs := len(data) / 2
	expectedZero := 0
	otherZero := 0
	textBytes := 0
	for i := 0; i < len(data); i += 2 {
		first := data[i]
		second := data[i+1]
		text := first
		zero := second
		other := first
		if order == binary.BigEndian {
			text = second
			zero = first
			other = second
		}
		if zero == 0 {
			expectedZero++
		}
		if other == 0 {
			otherZero++
		}
		if viewerLikelyUTF16TextByte(text) {
			textBytes++
		}
	}
	return expectedZero >= max(2, pairs/2) && otherZero == 0 && textBytes >= max(2, pairs/2)
}

func scoreViewerUTF16Candidate(data []byte, order binary.ByteOrder) int {
	if len(data) < 8 || len(data)%2 != 0 {
		return 0
	}
	pairs := len(data) / 2
	expectedZero := 0
	otherZero := 0
	textBytes := 0
	for i := 0; i < len(data); i += 2 {
		lo := data[i]
		hi := data[i+1]
		text := lo
		zero := hi
		other := lo
		if order == binary.BigEndian {
			text = hi
			zero = lo
			other = hi
		}
		if zero == 0 {
			expectedZero++
		}
		if other == 0 {
			otherZero++
		}
		if viewerLikelyUTF16TextByte(text) {
			textBytes++
		}
	}
	textRunes, suspicious, decodeErrors := viewerUTF16DecodeStats(data, order)
	score := expectedZero*10 + textBytes*3 + textRunes*5 - otherZero*14 - suspicious*10 - decodeErrors*16
	minExpectedZero := pairs / 4
	if minExpectedZero < 2 {
		minExpectedZero = 2
	}
	if expectedZero < minExpectedZero {
		return 0
	}
	if expectedZero < otherZero+2 {
		return 0
	}
	maxOtherZero := pairs / 5
	if maxOtherZero < 1 {
		maxOtherZero = 1
	}
	if otherZero > maxOtherZero {
		return 0
	}
	minTextRunes := pairs / 2
	if minTextRunes < 2 {
		minTextRunes = 2
	}
	if textRunes < minTextRunes {
		return 0
	}
	minTextBytes := pairs / 4
	if minTextBytes < 2 {
		minTextBytes = 2
	}
	if textBytes < minTextBytes {
		return 0
	}
	if decodeErrors > max(1, pairs/8) {
		return 0
	}
	if suspicious > pairs/3+1 {
		return 0
	}
	return score
}

func viewerUTF16DecodeStats(data []byte, order binary.ByteOrder) (int, int, int) {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		units = append(units, order.Uint16(data[i:i+2]))
	}
	textRunes := 0
	suspicious := 0
	decodeErrors := 0
	for i := 0; i < len(units); i++ {
		unit := units[i]
		switch {
		case unit >= 0xD800 && unit <= 0xDBFF:
			if i+1 >= len(units) {
				decodeErrors++
				suspicious++
				continue
			}
			next := units[i+1]
			if next < 0xDC00 || next > 0xDFFF {
				decodeErrors++
				suspicious++
				continue
			}
			r := utf16.DecodeRune(rune(unit), rune(next))
			if viewerLikelyUTF16TextRune(r) {
				textRunes++
			} else {
				suspicious++
			}
			i++
		case unit >= 0xDC00 && unit <= 0xDFFF:
			decodeErrors++
			suspicious++
		default:
			r := rune(unit)
			switch {
			case viewerLikelyUTF16TextRune(r):
				textRunes++
			case r == 0:
				suspicious += 2
			default:
				suspicious++
			}
		}
	}
	return textRunes, suspicious, decodeErrors
}

func viewerLikelyUTF16TextByte(b byte) bool {
	switch b {
	case 0, 0x7F:
		return false
	case '\t', '\n', '\r':
		return true
	default:
		return b >= 0x20
	}
}

func viewerLikelyUTF16TextRune(r rune) bool {
	switch {
	case r == '\t' || r == '\n' || r == '\r':
		return true
	case r == 0 || r == utf8.RuneError:
		return false
	case unicode.IsLetter(r), unicode.IsNumber(r), unicode.IsPunct(r), unicode.IsSpace(r), unicode.IsMark(r):
		return true
	case unicode.IsSymbol(r):
		switch r {
		case '$', '+', '<', '=', '>', '^', '`', '|', '~', '£', '¥', '§', '©', '®', '°', '±', 'µ', '¶', '×', '÷', '€', '™':
			return true
		}
		return false
	default:
		return false
	}
}

func detectViewerLegacyEncoding(data []byte) string {
	sample := data
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	if len(sample) == 0 {
		return ""
	}
	if bytes.IndexByte(sample, 0) >= 0 || utf8.Valid(sample) {
		return ""
	}
	highBytes := 0
	cp437Hints := 0
	for _, b := range sample {
		if b < 0x80 {
			continue
		}
		highBytes++
		if viewerLikelyCP437Byte(b) {
			cp437Hints++
		}
	}
	if highBytes < 4 || cp437Hints < 4 {
		return ""
	}
	if cp437Hints*4 < highBytes*3 {
		return ""
	}
	decoded := decodeViewerCP437(sample)
	artScore := viewerCP437ArtScore(decoded)
	if artScore < 4 {
		return ""
	}
	if viewerSuspiciousTextRunes(decoded) > max(1, len([]rune(decoded))/12) {
		return ""
	}
	if artScore*2 >= cp437Hints {
		return fm.ViewerFileEncodingCP437
	}
	return ""
}

func viewerLikelyCP437Byte(b byte) bool {
	switch {
	case b >= 0xB0 && b <= 0xDF:
		return true
	case b >= 0xF0:
		return true
	}
	switch b {
	case 0x87, 0x8E, 0x91, 0x92, 0x93, 0x9A, 0xA0, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5, 0xA6, 0xA7, 0xAD:
		return true
	default:
		return false
	}
}

func viewerCP437ArtScore(text string) int {
	score := 0
	for _, r := range text {
		switch {
		case r >= 0x2500 && r <= 0x259F:
			score++
		case r == '²' || r == '°' || r == 'ç' || r == 'Ç' || r == '¡' || r == '¬':
			score++
		}
	}
	return score
}

func viewerSuspiciousTextRunes(text string) int {
	suspicious := 0
	for _, r := range text {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			continue
		case r == utf8.RuneError || r == 0:
			suspicious++
		case unicode.IsControl(r):
			suspicious++
		}
	}
	return suspicious
}

func decodeViewerCP437(data []byte) string {
	out, err := charmap.CodePage437.NewDecoder().Bytes(data)
	if err != nil {
		return string(bytes.ToValidUTF8(data, []byte("\xef\xbf\xbd")))
	}
	return string(out)
}

func decodeViewerUTF16(data []byte, order binary.ByteOrder, bom []byte, trimBOM bool) string {
	if trimBOM {
		data = bytes.TrimPrefix(data, bom)
	}
	if len(data) == 0 {
		return ""
	}
	units := make([]uint16, 0, (len(data)+1)/2)
	for i := 0; i+1 < len(data); i += 2 {
		units = append(units, order.Uint16(data[i:i+2]))
	}
	runes := utf16.Decode(units)
	if len(data)%2 != 0 {
		runes = append(runes, unicode.ReplacementChar)
	}
	return string(runes)
}

func detectViewerLineEnding(raw string) string {
	if raw == "" {
		return viewerLineEndingNone
	}
	hasCRLF := strings.Contains(raw, "\r\n")
	raw = strings.ReplaceAll(raw, "\r\n", "")
	hasLF := strings.Contains(raw, "\n")
	hasCR := strings.Contains(raw, "\r")
	switch {
	case hasCRLF && !hasLF && !hasCR:
		return viewerLineEndingCRLF
	case !hasCRLF && hasLF && !hasCR:
		return viewerLineEndingLF
	case !hasCRLF && !hasLF && !hasCR:
		return viewerLineEndingNone
	default:
		return viewerLineEndingMixed
	}
}

func normalizeViewerLineEndings(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	return strings.ReplaceAll(raw, "\r", "\n")
}

func shellQuote(raw string) string {
	if raw == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(raw, "'", "'\"'\"'") + "'"
}

func powerShellQuote(raw string) string {
	if raw == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(raw, "'", "''") + "'"
}

func sanitizeViewerContent(raw string) string {
	if raw == "" {
		return raw
	}
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		switch r {
		case '\n':
			b.WriteRune('\n')
		case '\r':
			// Skip CR in CRLF sequences to avoid odd editor artifacts.
		case '\t':
			b.WriteString("    ")
		case unicode.ReplacementChar:
			b.WriteByte('.')
		default:
			if !unicode.IsPrint(r) {
				b.WriteByte('.')
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

func viewerClipboardContent(st *fileViewerState, text string) string {
	if st == nil || text == "" || st.mode != "file" {
		return text
	}
	if st.detectedLineEnding == viewerLineEndingCRLF {
		return strings.ReplaceAll(text, "\n", "\r\n")
	}
	return text
}
