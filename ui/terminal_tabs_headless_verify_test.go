// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build uiverify

package ui

import (
	"hexone/fm"
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
)

func TestHeadlessTerminalTabRail(t *testing.T) {
	outDir := os.Getenv("TERMINAL_TABS_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	dirs := []string{
		filepath.Join(root, "src"),
		filepath.Join(root, "gpstrack-go"),
		filepath.Join(root, "git"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := fm.DefaultConfig()
	cfg.Terminal.HeightRows = 6
	ui := NewUI(cfg)
	sessions := make([]*terminalSession, len(dirs))
	for i, dir := range dirs {
		st := newTerminalSession(nil)
		st.startDir = dir
		st.setActive(i == 1)
		st.startAttempted = true
		st.term.SetTitle(filepath.Base(dir))
		sessions[i] = st
	}
	ui.terminalTabs.sessions = sessions
	ui.terminalTabs.active = 1
	ui.terminal = sessions[1]
	ui.terminal.wantFocus = true

	const width, height = 900, 190
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatal(err)
	}
	defer win.Release()
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	router := new(input.Router)
	base := time.Now()
	for frame := 0; frame < 3; frame++ {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)),
			Now:         base.Add(time.Duration(frame) * 50 * time.Millisecond),
		}
		ui.layoutTerminalPane(th, gtx)
		router.Frame(&ops)
		if err := win.Frame(&ops); err != nil {
			t.Fatal(err)
		}
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(outDir, "terminal-tabs-open-rail.png"))
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
}
