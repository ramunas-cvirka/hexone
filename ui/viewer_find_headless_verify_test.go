// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build pdfverify

package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"hexone/fm"
)

func TestHeadlessViewerFindModes(t *testing.T) {
	outDir := os.Getenv("PDF_FIND_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	textPath := filepath.Join(dir, "find-modes.txt")
	var lines strings.Builder
	for i := 1; i <= 14; i++ {
		fmt.Fprintf(&lines, "line %02d has needle and compact viewer context\n", i)
	}
	if err := os.WriteFile(textPath, []byte(lines.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(dir, "find-modes.bin")
	pattern := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	block := append([]byte{0x10, 0x20, 0x30, 0x40}, pattern...)
	block = append(block, []byte(" hex-context\n")...)
	if err := os.WriteFile(binPath, bytes.Repeat(block, 14), 0o600); err != nil {
		t.Fatal(err)
	}

	const width, height = 1100, 760
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("headless window: %v", err)
	}
	defer win.Release()
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	ui := NewUI(fm.DefaultConfig())
	oldWriteNow := writeFileViewerClipboardNow
	var copiedNow string
	writeFileViewerClipboardNow = func(text string) error {
		copiedNow = text
		return nil
	}
	t.Cleanup(func() { writeFileViewerClipboardNow = oldWriteNow })
	router := new(input.Router)
	frame := func() *image.RGBA {
		var ops op.Ops
		gtx := layout.Context{
			Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)), Now: time.Now(), Source: router.Source(),
		}
		ui.Layout(th, gtx)
		router.Frame(&ops)
		if err := win.Frame(&ops); err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		if err := win.Screenshot(img); err != nil {
			t.Fatal(err)
		}
		return img
	}
	pumpUntil := func(deadline time.Time, ready func() bool) *image.RGBA {
		var img *image.RGBA
		for time.Now().Before(deadline) {
			img = frame()
			if ready() {
				return img
			}
			time.Sleep(12 * time.Millisecond)
		}
		return img
	}
	shoot := func(name string, img *image.RGBA) {
		path := filepath.Join(outDir, name)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
	openPath := func(path string) *fileViewerState {
		pane := ui.filePanes[0]
		ui.requestPaneLoadWithSelection(0, dir, path, "", 0)
		pumpUntil(time.Now().Add(3*time.Second), func() bool {
			entry := pane.selectedEntry()
			return entry != nil && entry.Path == path
		})
		ui.startFileViewer(0, time.Now())
		pumpUntil(time.Now().Add(4*time.Second), func() bool {
			return ui.fileViewer != nil && !ui.fileViewer.loading
		})
		if ui.fileViewer == nil {
			t.Fatalf("viewer did not open %s", path)
		}
		return ui.fileViewer
	}

	st := openPath(textPath)
	ui.openFileViewerFind(time.Now())
	st.find.editor.SetText("needle")
	ui.refreshFileViewerFind(time.Now(), false)
	if len(st.find.matches) != 14 {
		t.Fatalf("file matches=%d want 14", len(st.find.matches))
	}
	shoot("file-find-results.png", frame())
	router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(850, 100)})
	frame()
	time.Sleep(110 * time.Millisecond)
	shoot("file-find-hover-preview.png", frame())
	router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(850, 124)})
	frame()
	time.Sleep(24 * time.Millisecond)
	shoot("file-find-cursor-motion.png", frame())

	st.userBrowseUntil = time.Time{}
	st.stream.clearSelection()
	seqBefore := st.seq
	ui.setFileViewerMode("command", time.Now())
	pumpUntil(time.Now().Add(5*time.Second), func() bool {
		return st.mode == "command" && st.seq > seqBefore && !st.loading && !st.pendingUpdate &&
			!strings.HasPrefix(st.status, "refresh") && strings.Contains(st.content, "needle") && len(st.find.matches) == 14
	})
	if st.mode != "command" || st.loading || st.pendingUpdate || len(st.find.matches) != 14 {
		t.Fatalf("command find state mode=%q loading=%v pending=%v matches=%d status=%q err=%q", st.mode, st.loading, st.pendingUpdate, len(st.find.matches), st.status, st.err)
	}
	shoot("command-find-results.png", frame())

	ui.closeFileViewer()
	frame()
	st = openPath(binPath)
	ui.setFileViewerMode("hex", time.Now())
	pumpUntil(time.Now().Add(5*time.Second), func() bool {
		return st.mode == "hex" && !st.loading && st.hex != nil && st.hex.fileSize > 0
	})
	ui.openFileViewerFind(time.Now())
	frame()
	for i, r := range []rune("hex-context") {
		router.Queue(key.EditEvent{Range: key.Range{Start: i, End: i}, Text: string(r)})
		frame()
	}
	pumpUntil(time.Now().Add(5*time.Second), func() bool {
		return !st.find.searching && len(st.find.hexMatches) == 14
	})
	if st.find.editor.Text() != "hex-context" || len(st.find.hexMatches) != 14 {
		t.Fatalf("typed Hex find query=%q matches=%d status=%q", st.find.editor.Text(), len(st.find.hexMatches), st.find.status)
	}

	st.find.editor.SetText("DEADBEEF")
	st.find.findByClick.Click()
	frame()
	pumpUntil(time.Now().Add(5*time.Second), func() bool {
		return !st.find.searching && len(st.find.hexMatches) == 14
	})
	if !st.find.hexInput || st.find.hexPreview || len(st.find.hexMatches) != 14 {
		t.Fatalf("hex text-preview state input=%v preview=%v matches=%d status=%q", st.find.hexInput, st.find.hexPreview, len(st.find.hexMatches), st.find.status)
	}
	shoot("hex-find-results.png", frame())
	router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(850, 76)})
	frame()
	time.Sleep(110 * time.Millisecond)
	shoot("hex-find-hover-text-preview.png", frame())
	if len(st.find.hexMatches) < 2 || len(viewerHexFindPreviewLines(st, st.find.hexMatches[1])) < 2 {
		t.Fatal("second Hex match should expose detected multiline context")
	}
	router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(850, 100)})
	frame()
	time.Sleep(110 * time.Millisecond)
	shoot("hex-find-hover-preview.png", frame())

	st.find.previewClick.Click()
	frame()
	ui.refreshFileViewerFind(time.Now(), true)
	pumpUntil(time.Now().Add(5*time.Second), func() bool {
		return !st.find.searching && len(st.find.hexMatches) == 14
	})
	shoot("hex-find-hex-preview.png", frame())
	if !st.find.hexInput || !st.find.hexPreview {
		t.Fatalf("preview toggle changed find mode: input=%v preview=%v", st.find.hexInput, st.find.hexPreview)
	}

	st.find.editor.SetText("hex-context")
	st.find.findByClick.Click()
	frame()
	pumpUntil(time.Now().Add(5*time.Second), func() bool {
		return !st.find.searching && len(st.find.hexMatches) == 14
	})
	if st.find.hexInput || !st.find.hexPreview || len(st.find.hexMatches) != 14 {
		t.Fatalf("independent modes input=%v preview=%v matches=%d status=%q", st.find.hexInput, st.find.hexPreview, len(st.find.hexMatches), st.find.status)
	}
	shoot("hex-find-text-preview-hex.png", frame())

	selectionStart := st.hex.bufferStart
	selectionLen := int64(len(st.hex.buffer))
	if selectionLen > 48 {
		selectionLen = 48
	}
	if selectionLen < 2 {
		t.Fatalf("hex viewport has only %d selectable bytes", selectionLen)
	}
	st.hex.setSelectionRange(selectionStart, selectionLen)
	wantCopy := formatHexSelectionTextCopy(st.hex.buffer[:selectionLen])
	st.openContextMenu(image.Pt(420, 280), time.Now().Add(-time.Second))
	shoot("hex-find-copy-selection-menu.png", frame())
	st.copyTextToggle.Click()
	frame()
	_, copied, ok := router.WriteClipboard()
	if !ok || string(copied) != wantCopy || copiedNow != wantCopy {
		t.Fatalf("copy with Find open: Gio=(%t, %q) sync=%q want selection %q", ok, copied, copiedNow, wantCopy)
	}
}
