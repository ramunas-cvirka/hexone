// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build uiverify

package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"hexone/fm"
)

func TestHeadlessViewerWrappedEditResize(t *testing.T) {
	outDir := os.Getenv("VIEWER_RESIZE_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := strings.Repeat("2026-07-19 12:34:56 INFO request completed component=viewer elapsed=17ms status=ok\n", 5400)
	if path := os.Getenv("VIEWER_RESIZE_FILE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read resize fixture: %v", err)
		}
		content = string(data)
	}

	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	st := &fileViewerState{
		mode:             "file",
		path:             "viewer-resize.log",
		name:             "viewer-resize.log",
		content:          sanitizeViewerContent(content),
		editableContent:  content,
		editBaselineText: content,
		wrapEnabled:      true,
		status:           "ready",
	}
	st.stream.SetContent(st.content)
	ui.fileViewer = st

	start := time.Now()
	if !ui.startFileViewerEdit(start) {
		t.Fatalf("start edit: %s", st.status)
	}
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	router := new(input.Router)

	render := func(name string, size image.Point, now time.Time) time.Duration {
		win, err := headless.NewWindow(size.X, size.Y)
		if err != nil {
			t.Fatalf("headless window: %v", err)
		}
		defer win.Release()

		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(size),
			Now:         now,
			Source:      router.Source(),
		}
		layoutStart := time.Now()
		ui.Layout(th, gtx)
		layoutElapsed := time.Since(layoutStart)
		router.Frame(&ops)
		if err := win.Frame(&ops); err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rectangle{Max: size})
		if err := win.Screenshot(img); err != nil {
			t.Fatal(err)
		}
		file, err := os.Create(filepath.Join(outDir, name))
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
		return layoutElapsed
	}

	initialSize := image.Pt(1100, 700)
	initialElapsed := render("viewer-resize-initial.png", initialSize, start)
	widths := []int{1040, 960, 880, 800, 720}
	activeMax := time.Duration(0)
	lastNow := start
	for i, width := range widths {
		lastNow = start.Add(time.Duration(i+1) * 20 * time.Millisecond)
		elapsed := render("viewer-resize-active.png", image.Pt(width, initialSize.Y), lastNow)
		if elapsed > activeMax {
			activeMax = elapsed
		}
	}
	settledElapsed := render(
		"viewer-resize-settled.png",
		image.Pt(widths[len(widths)-1], initialSize.Y),
		lastNow.Add(fileViewerEditResizeSettleDelay+time.Millisecond),
	)
	t.Logf("layout initial=%s active-max=%s settled=%s fixture-bytes=%d", initialElapsed, activeMax, settledElapsed, len(content))
}
