// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build terminalverify

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
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"hexone/fm"
)

func TestHeadlessTerminalSeventyFivePercentResize(t *testing.T) {
	const width, height = 1100, 1000
	outDir := os.Getenv("TERMINAL_RESIZE_OUT")
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
	cfg.Terminal.HeightRows = 512
	ui := NewUI(cfg)
	ui.terminal = newTerminalSession(nil, cfg.Terminal.HeightRows)
	ui.terminal.setActive(true)
	ui.terminal.startAttempted = true // Render without launching a shell in verification.
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	router := new(input.Router)
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Now:         time.Now(),
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(width, height)),
	}
	ui.Layout(th, gtx)
	router.Frame(&ops)
	if err := win.Frame(&ops); err != nil {
		t.Fatal(err)
	}
	paneH, _, ok := ui.terminal.paneMetrics()
	if !ok {
		t.Fatal("terminal pane metrics missing")
	}
	if paneH <= 560 {
		t.Fatalf("terminal height=%d did not exceed old fixed cap", paneH)
	}
	if paneH > height*terminalMaxPaneNum/terminalMaxPaneDen {
		t.Fatalf("terminal height=%d exceeds 75%% of window", paneH)
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(outDir, "terminal-75-percent.png")
	f, err := os.Create(outPath)
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
	t.Logf("wrote %s", outPath)
}
