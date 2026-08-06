// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build terminalfindverify

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

func TestHeadlessTerminalFind(t *testing.T) {
	const width, height = 920, 340
	outDir := os.Getenv("TERMINAL_FIND_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatal(err)
	}
	defer win.Release()

	cfg := fm.DefaultConfig()
	cfg.Terminal.HeightRows = 16
	ui := NewUI(cfg)
	st := newTerminalSession(nil, cfg.Terminal.HeightRows)
	st.setActive(true)
	st.startAttempted = true
	ui.terminal = st
	ui.terminalTabs.sessions = []*terminalSession{st}
	ui.terminalTabs.active = 0
	st.writeOutput([]byte("starting build\r\nservice alpha:\r\n\tERROR unable to resolve package alpha\r\n\tretrying dependency scan\r\nservice beta:\r\n\tERROR package beta timed out\r\n\tcontinuing with cached modules\r\nservice linker:\r\n\tERROR final linker failure\r\nbuild stopped\r\n$ "))
	st.find.results = make(chan terminalFindResult, 16)
	st.find.editor.SingleLine = true
	st.find.editor.Submit = true
	st.find.list.Axis = layout.Vertical
	st.find.editor.SetText("error")
	st.find.open = true
	st.find.focus = true
	st.startFindSearch("error")

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	router := new(input.Router)
	base := time.Now()
	frame := func(at time.Time) *image.RGBA {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Now:         at,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)),
		}
		ui.layoutTerminalPane(th, gtx)
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
	var img *image.RGBA
	for i := 0; i < 30; i++ {
		img = frame(base.Add(time.Duration(i) * 16 * time.Millisecond))
		if len(st.find.matches) == 3 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if got, want := len(st.find.matches), 3; got != want {
		t.Fatalf("async matches=%d want %d", got, want)
	}
	writeTerminalFindPNG(t, filepath.Join(outDir, "terminal-find-results.png"), img)

	// The find overlay reuses the standard flat close button, including its
	// red hover surface and close icon treatment.
	router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(899, 43)})
	frame(base.Add(560 * time.Millisecond))
	img = frame(base.Add(576 * time.Millisecond))
	writeTerminalFindPNG(t, filepath.Join(outDir, "terminal-find-close-hover.png"), img)

	// The compact panel begins at the right edge. Move across its second row
	// and render two frames so Gio updates Clickable.Hovered and the deferred
	// three-line preview.
	router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(730, 102)})
	frame(base.Add(600 * time.Millisecond))
	img = frame(base.Add(616 * time.Millisecond))
	writeTerminalFindPNG(t, filepath.Join(outDir, "terminal-find-hover-preview.png"), img)

	// Result rows stay fixed while the preview changes, so moving directly to
	// the next row must not collapse or shift the list under the pointer.
	router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(730, 118)})
	frame(base.Add(632 * time.Millisecond))
	img = frame(base.Add(648 * time.Millisecond))
	if got, want := st.find.previewIndex, 2; got != want {
		t.Fatalf("preview transition index=%d want %d", got, want)
	}
	writeTerminalFindPNG(t, filepath.Join(outDir, "terminal-find-hover-transition.png"), img)

	ui.fmCfg.Terminal.PreviewStart = -1
	ui.fmCfg.Terminal.PreviewEnd = 3
	st.setFindPreviewRange(-1, 3)
	for i := 0; i < 30 && (st.find.searching || len(st.find.matches) != 3); i++ {
		frame(base.Add(700*time.Millisecond + time.Duration(i)*16*time.Millisecond))
		time.Sleep(time.Millisecond)
	}
	if got, want := len(st.find.matches), 3; got != want {
		t.Fatalf("custom range matches=%d want %d", got, want)
	}
	if got, want := len(st.find.matches[1].Preview), 5; got != want {
		t.Fatalf("custom -1..3 preview lines=%d want %d", got, want)
	}
	router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(730, 94)})
	frame(base.Add(1220 * time.Millisecond))
	img = frame(base.Add(1236 * time.Millisecond))
	writeTerminalFindPNG(t, filepath.Join(outDir, "terminal-find-custom-preview-range.png"), img)
	t.Logf("wrote terminal find frames to %s", outDir)
}

func writeTerminalFindPNG(t *testing.T, path string, img image.Image) {
	t.Helper()
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
}
