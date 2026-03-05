package ui

import (
	"bytes"
	"context"
	"fmt"
	"hexone/filesys"
	"hexone/fm"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

const viewerMaxLoadBytes = 1 << 20

type fileViewerState struct {
	pane int
	path string
	name string
	mode string

	backdropClick widget.Clickable
	closeClick    widget.Clickable
	refreshClick  widget.Clickable
	commandClick  widget.Clickable
	contentEditor widget.Editor
	commandEditor widget.Editor
	commandEditOn bool
	commandFocus  bool

	content string
	status  string
	err     string
	command string

	loading bool
	seq     int

	nextWatchCheck time.Time
	watchExists    bool
	watchSize      int64
	watchModTime   time.Time
	resultCh       chan fileViewerResult
}

type fileViewerResult struct {
	seq     int
	content string
	status  string
	err     string
}

func (ui *UI) handleFileViewerKeys(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape},
			key.Filter{Name: key.NameF3},
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
		if ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case key.NameEscape:
			if st := ui.fileViewer; st != nil && st.commandEditOn {
				st.commandEditOn = false
				st.commandFocus = false
				st.commandEditor.SetText(st.command)
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			ui.closeFileViewer()
		case key.NameF3:
			ui.startFileViewerLoad(gtx.Now)
		}
	}
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

	st := &fileViewerState{
		pane:     idx,
		path:     entry.Path,
		name:     entry.DisplayName,
		status:   "loading...",
		resultCh: make(chan fileViewerResult, 1),
	}
	if st.name == "" {
		st.name = filepath.Base(entry.Path)
	}
	st.contentEditor.SingleLine = false
	st.contentEditor.ReadOnly = true
	st.contentEditor.Submit = false
	st.contentEditor.SetText("")
	st.commandEditor.SingleLine = true
	st.commandEditor.Submit = true
	st.captureWatchState()

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
	ui.clearFileViewHotkeyHold()
	ui.fileViewer = nil
}

func (ui *UI) clearFileViewHotkeyHold() {
	if ui == nil || ui.held == nil {
		return
	}
	ui.held[fileActionKey(fileActionView)] = false
}

func (ui *UI) startFileViewerLoad(now time.Time) {
	st := ui.fileViewer
	if st == nil || st.loading {
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
	st.mode = normalizeViewerMode(cfg.Mode)
	st.command = strings.TrimSpace(cfg.Command)
	if st.command == "" {
		st.command = "cat {path}"
	}
	if !st.commandEditOn {
		st.commandEditor.SetText(st.command)
	}

	st.seq++
	seq := st.seq
	st.loading = true
	st.err = ""
	st.status = "loading..."
	st.nextWatchCheck = time.Time{}
	path := st.path
	ch := st.resultCh

	go func() {
		content, status, err := readViewerContent(path, cfg, viewerMaxLoadBytes)
		res := fileViewerResult{
			seq:     seq,
			content: content,
			status:  status,
			err:     err,
		}
		select {
		case ch <- res:
		default:
		}
	}()

	_ = now
}

func (ui *UI) pumpFileViewerState(gtx layout.Context) {
	st := ui.fileViewer
	if st == nil || st.resultCh == nil {
		return
	}

	for {
		select {
		case res := <-st.resultCh:
			if res.seq != st.seq {
				continue
			}
			st.loading = false
			st.err = res.err
			st.status = res.status
			if st.status == "" {
				st.status = "ready"
			}
			st.content = res.content
			st.contentEditor.SetText(res.content)
			st.contentEditor.SetCaret(0, 0)
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
	if st.nextWatchCheck.IsZero() {
		st.nextWatchCheck = gtx.Now.Add(interval)
	}
	if !gtx.Now.Before(st.nextWatchCheck) {
		st.nextWatchCheck = gtx.Now.Add(interval)
		if st.watchChanged() {
			st.status = "changed on disk"
			st.nextWatchCheck = time.Time{}
			ui.startFileViewerLoad(gtx.Now)
			return
		}
	}
	gtx.Execute(op.InvalidateCmd{At: st.nextWatchCheck})
}

func (st *fileViewerState) captureWatchState() {
	if st == nil {
		return
	}
	info, err := os.Stat(st.path)
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
	info, err := os.Stat(st.path)
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

func (ui *UI) viewerTextSize() unit.Sp {
	if ui == nil || ui.fmCfg == nil {
		return normalizeUIFontSize(13)
	}
	if ui.fmCfg.Viewer.FontSizeSp < 6 {
		return scaleConfigFontSize(ui.fmCfg, 13)
	}
	return normalizeUIFontSize(unit.Sp(ui.fmCfg.Viewer.FontSizeSp))
}

func (ui *UI) refreshFileViewerNow(now time.Time) {
	st := ui.fileViewer
	if st == nil {
		return
	}
	st.nextWatchCheck = time.Time{}
	ui.startFileViewerLoad(now)
}

func normalizeViewerMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "command" {
		return "file"
	}
	return "command"
}

func (ui *UI) startViewerCommandEdit() {
	st := ui.fileViewer
	if st == nil || st.mode != "command" {
		return
	}
	st.commandEditOn = true
	st.commandFocus = true
	st.commandEditor.SetText(st.command)
	st.commandEditor.SetCaret(st.commandEditor.Len(), st.commandEditor.Len())
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
	st.commandEditOn = false
	st.commandFocus = false
	if ui.fmCfg != nil {
		ui.fmCfg.Viewer.Mode = "command"
		ui.fmCfg.Viewer.Command = cmd
		if err := fm.SaveConfig("fm.yaml", ui.fmCfg); err != nil {
			st.err = err.Error()
			return
		}
	}
	ui.startFileViewerLoad(now)
}

func readViewerContent(path string, cfg fm.ViewerConfig, maxBytes int) (string, string, string) {
	start := time.Now()
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	switch mode {
	case "command":
		return readViewerCommand(path, cfg.Command, cfg.Shell, maxBytes, start)
	default:
		return readViewerFile(path, maxBytes, start)
	}
}

func readViewerFile(path string, maxBytes int, started time.Time) (string, string, string) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err.Error()
	}
	defer f.Close()

	limited := io.LimitReader(f, int64(maxBytes)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", "", err.Error()
	}
	truncated := false
	if len(data) > maxBytes {
		truncated = true
		data = data[:maxBytes]
	}
	content := string(bytes.ToValidUTF8(data, []byte("\xef\xbf\xbd")))
	content = sanitizeViewerContent(content)
	if truncated {
		content += "\n\n[truncated]"
	}
	status := fmt.Sprintf("file: %d bytes", len(data))
	if truncated {
		status += " (truncated)"
	}
	status += fmt.Sprintf(" | %s", time.Since(started).Round(time.Millisecond))
	return content, status, ""
}

func readViewerCommand(path, template, shellMode string, maxBytes int, started time.Time) (string, string, string) {
	cmdline := strings.TrimSpace(template)
	if cmdline == "" {
		return "", "", "viewer command is empty"
	}
	shell := resolveViewerShell(shellMode)
	cmdline = expandViewerCommandTemplate(cmdline, path, shell.quoteFn)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	args := append(append([]string{}, shell.args...), cmdline)
	cmd := exec.CommandContext(ctx, shell.program, args...)
	configureViewerCommandProcess(cmd)
	cmd.Dir = filepath.Clean(filepath.Dir(path))
	buf := newViewerCommandBuffer(maxBytes, cancel)
	cmd.Stdout = buf
	cmd.Stderr = buf
	err := cmd.Run()
	out := buf.Bytes()
	truncated := buf.Truncated()
	content := string(bytes.ToValidUTF8(out, []byte("\xef\xbf\xbd")))
	content = sanitizeViewerContent(content)
	if truncated {
		content += "\n\n[truncated]"
	}
	status := fmt.Sprintf("command (%s)", shell.name)
	if truncated {
		status += " (truncated)"
	}
	status += fmt.Sprintf(" | %s", time.Since(started).Round(time.Millisecond))
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return content, status, "viewer command timed out"
		}
		if ctx.Err() == context.Canceled && truncated {
			return content, status, ""
		}
		return content, status, err.Error()
	}
	return content, status, ""
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

func resolveViewerShell(raw string) viewerShellSpec {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" || mode == "auto" {
		if runtime.GOOS == "windows" {
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
		return resolveViewerShell("auto")
	}
}

func expandViewerCommandTemplate(template, fullpath string, quoteFn func(string) string) string {
	filename := filepath.Base(fullpath)
	cmdline := strings.TrimSpace(template)
	if quoteFn == nil {
		quoteFn = shellQuote
	}
	cmdline = strings.ReplaceAll(cmdline, "{fullpath}", quoteFn(fullpath))
	cmdline = strings.ReplaceAll(cmdline, "{path}", quoteFn(fullpath))
	cmdline = strings.ReplaceAll(cmdline, "{filename}", quoteFn(filename))
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
