// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build terminalmiddleclickverify

package ui

import (
	"hexone/fm"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestHeadlessTerminalMiddleClickPaste(t *testing.T) {
	const width, height = 900, 300
	outDir := os.Getenv("TERMINAL_MIDDLE_CLICK_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldRead := readTerminalClipboardText
	readTerminalClipboardText = func() (string, error) {
		return "clipboard fallback", nil
	}
	defer func() {
		readTerminalClipboardText = oldRead
	}()

	cfg := fm.DefaultConfig()
	cfg.Terminal.HeightRows = 10
	cfg.Terminal.Maximized = true
	hexUI := NewUI(cfg)
	st := newTerminalSession(nil, cfg.Terminal.HeightRows)
	st.setActive(true)
	st.startAttempted = true
	st.writeOutput([]byte("\x1b[?1000h\x1b[?1006hclipboard fallback / selected source"))
	proc := &terminalWriteProcess{}
	st.procMu.Lock()
	st.pty = proc
	st.running = true
	st.procMu.Unlock()
	hexUI.terminal = st
	hexUI.terminalTabs.sessions = []*terminalSession{st}
	hexUI.terminalTabs.active = 0

	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatal(err)
	}
	defer win.Release()
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	router := new(input.Router)
	base := time.Now()
	render := func(at time.Time) *image.RGBA {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Now:         at,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)),
		}
		hexUI.Layout(th, gtx)
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

	render(base)
	render(base.Add(16 * time.Millisecond))
	router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonTertiary,
		Position: f32.Pt(120, 120),
	})
	render(base.Add(32 * time.Millisecond))
	if got, want := proc.String(), "clipboard fallback"; got != want {
		t.Fatalf("clipboard middle-click paste=%q want %q", got, want)
	}

	st.viewMu.Lock()
	st.selectionActive = true
	st.selectionStart = terminalPoint{Row: 0, Col: 21}
	st.selectionEnd = terminalPoint{Row: 0, Col: 35}
	st.viewMu.Unlock()
	router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonTertiary,
		Position: f32.Pt(120, 120),
	})
	img := render(base.Add(48 * time.Millisecond))
	if got, want := proc.String(), "clipboard fallbackselected source"; got != want {
		t.Fatalf("selection middle-click paste=%q want %q", got, want)
	}
	if !st.hasActiveSelection() {
		t.Fatal("middle-click paste cleared the rendered terminal selection")
	}

	path := filepath.Join(outDir, "terminal-middle-click-selection.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote middle-click verification frame to %s", path)
}
