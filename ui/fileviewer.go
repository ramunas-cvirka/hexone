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
	"strings"
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
		return readViewerCommand(path, cfg.Command, maxBytes, start)
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

func readViewerCommand(path, template string, maxBytes int, started time.Time) (string, string, string) {
	cmdline := strings.TrimSpace(template)
	if cmdline == "" {
		return "", "", "viewer command is empty"
	}
	cmdline = expandViewerCommandTemplate(cmdline, path)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-lc", cmdline)
	cmd.Dir = filepath.Clean(filepath.Dir(path))
	out, err := cmd.CombinedOutput()
	truncated := false
	if len(out) > maxBytes {
		truncated = true
		out = out[:maxBytes]
	}
	content := string(bytes.ToValidUTF8(out, []byte("\xef\xbf\xbd")))
	content = sanitizeViewerContent(content)
	if truncated {
		content += "\n\n[truncated]"
	}
	status := "command"
	if truncated {
		status += " (truncated)"
	}
	status += fmt.Sprintf(" | %s", time.Since(started).Round(time.Millisecond))
	if err != nil {
		return content, status, err.Error()
	}
	return content, status, ""
}

func expandViewerCommandTemplate(template, fullpath string) string {
	filename := filepath.Base(fullpath)
	cmdline := strings.TrimSpace(template)
	cmdline = strings.ReplaceAll(cmdline, "{fullpath}", shellQuote(fullpath))
	cmdline = strings.ReplaceAll(cmdline, "{path}", shellQuote(fullpath))
	cmdline = strings.ReplaceAll(cmdline, "{filename}", shellQuote(filename))
	return cmdline
}

func shellQuote(raw string) string {
	if raw == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(raw, "'", "'\"'\"'") + "'"
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
