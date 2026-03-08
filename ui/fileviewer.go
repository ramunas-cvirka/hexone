package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hexone/filesys"
	"hexone/fm"
	"image"
	"io"
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

	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

const (
	viewerDefaultMaxLoadBytes = 1 << 20
	viewerCommandExecTimeout  = 15 * time.Second
	viewerCommandStreamTick   = 180 * time.Millisecond
	viewerDefaultRefreshMs    = 1500
	viewerMinimumRefreshMs    = 200
	viewerDefaultAutoRefresh  = true
	viewerDefaultMaxReadMB    = 1
	viewerDefaultWordRegex    = `[a-zA-Z0-9]+`
	viewerCommandHistoryLimit = 80
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

	backdropClick    widget.Clickable
	closeClick       widget.Clickable
	autoRefreshClick widget.Clickable
	modeFileClick    widget.Clickable
	modeHexClick     widget.Clickable
	modeCmdClick     widget.Clickable
	historyClick     widget.Clickable
	commandClick     widget.Clickable
	contentEditor    widget.Editor
	commandEditor    widget.Editor
	wrapToggle       widget.Clickable
	copyToggle       widget.Clickable
	commandEditOn    bool
	commandFocus     bool

	content         string
	status          string
	err             string
	command         string
	commandInfinite bool
	autoRefresh     bool
	wordSelectRE    *regexp.Regexp
	wordSelectExpr  string
	updatedAt       time.Time
	tabAnimAt       time.Time
	stream          streamOutputView
	hex             *hexViewerState
	historyOpen     bool

	loading    bool
	seq        int
	loadCancel context.CancelFunc

	contentPointerTag fileViewerEventTag
	rootPointerTag    fileViewerEventTag
	commandAreaTag    fileViewerEventTag
	commandAreaPress  map[pointer.ID]struct{}
	userBrowseUntil   time.Time
	pendingUpdate     bool
	pendingContent    string
	pendingStatus     string
	pendingErr        string
	wrapEnabled       bool
	menuOpen          bool
	menuPos           image.Point
	menuRect          image.Rectangle
	menuPointerTag    fileViewerEventTag
	scrollCarry       float32
	scrollbarTrack    image.Rectangle
	scrollbarThumb    image.Rectangle
	scrollbarDragging bool
	scrollbarDragID   pointer.ID
	scrollbarHover    bool
	scrollbarVisible  bool
	scrollbarLines    int
	scrollbarVisibleN int

	nextWatchCheck time.Time
	watchExists    bool
	watchSize      int64
	watchModTime   time.Time
	resultCh       chan fileViewerResult
	historyClicks  map[string]*widget.Clickable
	tabAnim        segmentedAnimState
}

type fileViewerResult struct {
	seq     int
	content string
	status  string
	err     string
	partial bool
	final   bool
}

func (ui *UI) handleFileViewerKeys(gtx layout.Context) {
	anyMods := ^key.Modifiers(0)
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape},
			key.Filter{Name: key.NameF3},
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
		case key.NameEscape:
			if st := ui.fileViewer; st != nil && st.commandEditOn {
				ui.cancelViewerCommandEdit()
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			ui.closeFileViewer()
		case key.NameF3:
			ui.startFileViewerLoad(gtx.Now)
		case "c", "C":
			st := ui.fileViewer
			if st == nil || st.commandEditOn {
				continue
			}
			if ui.copyFileViewerText(gtx, false) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case "a", "A":
			st := ui.fileViewer
			if st == nil || st.commandEditOn {
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
		data, ok := st.hex.selectedBytes()
		if !ok && fallbackAll && len(st.hex.buffer) > 0 {
			data = append([]byte(nil), st.hex.buffer...)
			ok = true
		}
		if !ok || len(data) == 0 {
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
		pane:        idx,
		path:        entry.Path,
		name:        entry.DisplayName,
		remote:      remote,
		status:      "loading...",
		wrapEnabled: false,
		resultCh:    make(chan fileViewerResult, 1),
	}
	st.mode = "file"
	st.command = "cat {path}"
	st.autoRefresh = viewerDefaultAutoRefresh
	if ui != nil && ui.fmCfg != nil {
		cfg := ui.fmCfg.Viewer
		st.mode = normalizeViewerMode(cfg.Mode)
		st.autoRefresh = cfg.CommandAutoRefresh
		if cmd := strings.TrimSpace(cfg.Command); cmd != "" {
			st.command = cmd
		}
	}
	st.command = ui.viewerCommandForTarget(st.path, st.remote, st.command)
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
	ui.stopFileViewerScrollRepeat(key.NameUpArrow)
	ui.stopFileViewerScrollRepeat(key.NameDownArrow)
	ui.stopFileViewerScrollRepeat(key.NamePageUp)
	ui.stopFileViewerScrollRepeat(key.NamePageDown)
}

func viewerScrollKeySupported(name key.Name) bool {
	switch name {
	case key.NameUpArrow, key.NameDownArrow, key.NamePageUp, key.NamePageDown, key.NameHome, key.NameEnd:
		return true
	default:
		return false
	}
}

func viewerScrollRepeatableKey(name key.Name) bool {
	switch name {
	case key.NameUpArrow, key.NameDownArrow, key.NamePageUp, key.NamePageDown:
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
	if st.command == "" {
		st.command = "cat {path}"
	}
	cfg.Mode = st.mode
	cfg.Command = st.command
	cfg.CommandAutoRefresh = st.autoRefresh
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
	st.err = ""
	if st.content == "" {
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
		content, status, err := readViewerContent(ctx, path, cfg, maxBytes, remote, progress)
		res := fileViewerResult{
			seq:     seq,
			content: content,
			status:  status,
			err:     err,
			final:   true,
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
	if st.resultCh == nil {
		return
	}
	if st.pendingUpdate && !st.userIsBrowsing(gtx.Now) {
		st.pendingUpdate = false
		st.err = st.pendingErr
		st.status = st.pendingStatus
		if st.status == "" {
			st.status = "ready"
		}
		applyFileViewerContentResult(st, st.pendingContent)
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
				if viewerUpdateAction(st, res.content) != viewerUpdateSame {
					applyFileViewerContentResult(st, res.content)
					st.markUpdated(gtx.Now)
				}
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
			updateAction := viewerUpdateAction(st, res.content)
			if updateAction == viewerUpdateReplace && (st.userIsBrowsing(gtx.Now) || st.stream.hasSelection()) {
				st.pendingUpdate = true
				st.pendingContent = res.content
				st.pendingStatus = st.status
				st.pendingErr = st.err
				st.status = "update pending"
				ui.scheduleFileViewerWatch(gtx)
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			st.pendingUpdate = false
			applyFileViewerContentResult(st, res.content)
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

func viewerUpdateAction(st *fileViewerState, next string) int {
	if st == nil {
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
		st.mode = normalizeViewerMode(ui.fmCfg.Viewer.Mode)
		st.autoRefresh = ui.fmCfg.Viewer.CommandAutoRefresh
		if st.mode == "command" {
			st.command = ui.viewerCommandForTarget(st.path, st.remote, ui.fmCfg.Viewer.Command)
			if st.command == "" {
				st.command = "cat {path}"
			}
		}
	}
	st.commandEditOn = false
	st.commandFocus = false
	st.historyOpen = false
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
	st.mode = mode
	st.commandEditOn = false
	st.commandFocus = false
	st.historyOpen = false
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

func readViewerContent(ctx context.Context, path string, cfg fm.ViewerConfig, maxBytes int, remote *paneSSHSession, onProgress func(string, string)) (string, string, string) {
	start := time.Now()
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	switch mode {
	case "command":
		return readViewerCommand(ctx, path, cfg, maxBytes, start, remote, onProgress)
	default:
		return readViewerFile(path, maxBytes, start, remote)
	}
}

func readViewerFile(path string, maxBytes int, started time.Time, remote *paneSSHSession) (string, string, string) {
	if maxBytes < 1 {
		maxBytes = viewerDefaultMaxLoadBytes
	}

	var (
		size    int64 = -1
		reader  io.ReadCloser
		openErr error
	)

	if remote == nil {
		info, err := os.Stat(path)
		if err != nil {
			return "", "", err.Error()
		}
		if info.IsDir() {
			return "", "", "viewer supports files only"
		}
		size = info.Size()
		if size > int64(maxBytes) {
			return "", fmt.Sprintf("file: %d bytes", size),
				fmt.Sprintf("file too large: %s > %s limit", formatCopySize(size), formatCopySize(int64(maxBytes)))
		}
		reader, openErr = os.Open(path)
	} else {
		client := remote.sftpClient()
		if client == nil {
			return "", "", "sftp session is not connected"
		}
		info, err := client.Stat(path)
		if err != nil {
			return "", "", err.Error()
		}
		if info.IsDir() {
			return "", "", "viewer supports files only"
		}
		size = info.Size()
		if size > int64(maxBytes) {
			return "", fmt.Sprintf("remote file: %d bytes", size),
				fmt.Sprintf("file too large: %s > %s limit", formatCopySize(size), formatCopySize(int64(maxBytes)))
		}
		reader, openErr = client.Open(path)
	}
	if openErr != nil {
		return "", "", openErr.Error()
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, int64(maxBytes)))
	if err != nil {
		return "", "", err.Error()
	}
	content := string(bytes.ToValidUTF8(data, []byte("\xef\xbf\xbd")))
	content = sanitizeViewerContent(content)
	prefix := "file"
	if remote != nil {
		prefix = "remote file"
	}
	if size >= 0 {
		return content, fmt.Sprintf("%s: %d bytes", prefix, size), ""
	}
	return content, fmt.Sprintf("%s: %d bytes", prefix, len(data)), ""
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
		return content, status, err.Error()
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
		return content, status, waitErr.Error()
	}
	return content, status, ""
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
			b.WriteByte('?')
		default:
			if !unicode.IsPrint(r) {
				appendEscapedRune(&b, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

func appendEscapedRune(b *strings.Builder, r rune) {
	const hex = "0123456789ABCDEF"
	switch {
	case r <= 0xFF:
		b.WriteString(`\x`)
		b.WriteByte(hex[(r>>4)&0xF])
		b.WriteByte(hex[r&0xF])
	case r <= 0xFFFF:
		b.WriteString(`\u`)
		b.WriteByte(hex[(r>>12)&0xF])
		b.WriteByte(hex[(r>>8)&0xF])
		b.WriteByte(hex[(r>>4)&0xF])
		b.WriteByte(hex[r&0xF])
	default:
		b.WriteString(`\U`)
		b.WriteByte(hex[(r>>28)&0xF])
		b.WriteByte(hex[(r>>24)&0xF])
		b.WriteByte(hex[(r>>20)&0xF])
		b.WriteByte(hex[(r>>16)&0xF])
		b.WriteByte(hex[(r>>12)&0xF])
		b.WriteByte(hex[(r>>8)&0xF])
		b.WriteByte(hex[(r>>4)&0xF])
		b.WriteByte(hex[r&0xF])
	}
}
