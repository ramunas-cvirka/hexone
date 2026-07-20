// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build uiverify

package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"hexone/fm"
)

func TestHeadlessFunctionBar(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	const width, height = 1000, 640
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("create headless window: %v", err)
	}
	defer win.Release()

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	cfg := fm.DefaultConfig()
	cfg.Viewer.HideFunctionBarWhenOpen = false
	ui := NewUI(cfg)
	router := new(input.Router)
	render := func() *image.RGBA {
		var img *image.RGBA
		base := time.Now()
		for frame := 0; frame < 4; frame++ {
			var ops op.Ops
			gtx := layout.Context{
				Ops:         &ops,
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Constraints: layout.Exact(image.Pt(width, height)),
				Now:         base.Add(time.Duration(frame) * 100 * time.Millisecond),
				Source:      router.Source(),
			}
			ui.Layout(th, gtx)
			router.Frame(&ops)
			if err := win.Frame(&ops); err != nil {
				t.Fatalf("render frame: %v", err)
			}
			img = image.NewRGBA(image.Rect(0, 0, width, height))
			if err := win.Screenshot(img); err != nil {
				t.Fatalf("capture frame: %v", err)
			}
		}
		return img
	}
	writePNG := func(name string, img *image.RGBA) {
		path := filepath.Join(outDir, name)
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create screenshot: %v", err)
		}
		if err := png.Encode(file, img); err != nil {
			file.Close()
			t.Fatalf("encode screenshot: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close screenshot: %v", err)
		}
		t.Logf("wrote %s", path)
	}

	writePNG("function-bar.png", render())
	ui.functionBarHeldMods = key.ModCtrl
	writePNG("function-bar-ctrl.png", render())
	ui.functionBarHeldMods = key.ModAlt
	writePNG("function-bar-alt.png", render())

	viewerDir := t.TempDir()
	viewerPath := filepath.Join(viewerDir, "viewer-function-bar.txt")
	if err := os.WriteFile(viewerPath, []byte("HexOne viewer function bar verification\n"), 0o644); err != nil {
		t.Fatalf("write viewer fixture: %v", err)
	}
	if !ui.requestPaneLoadWithSelection(0, viewerDir, viewerPath, "", 0) {
		t.Fatal("request viewer fixture directory")
	}
	waitFor := func(label string, ready func() bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for !ready() && time.Now().Before(deadline) {
			render()
			time.Sleep(5 * time.Millisecond)
		}
		if !ready() {
			t.Fatalf("timed out waiting for %s", label)
		}
	}
	waitFor("fixture selection", func() bool {
		pane := ui.filePanes[0]
		entry := pane.selectedEntry()
		return !pane.loading && entry != nil && entry.Path == viewerPath
	})
	ui.startFileViewer(0, time.Now())
	waitFor("viewer load", func() bool {
		return ui.fileViewer != nil && !ui.fileViewer.loading
	})
	ui.functionBarHeldMods = 0
	writePNG("function-bar-viewer.png", render())
}
