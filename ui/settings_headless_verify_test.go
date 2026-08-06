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

	"hexone/fm"
)

func TestHeadlessSettingsConfig(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	const width, height = 800, 600
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	if ui.settingsModal == nil {
		t.Fatal("settings modal did not open")
	}
	ui.configPath = `C:\Users\ramuc\AppData\Local\Packages\RamnasCvirka.hexone_wgc727vgx32zp\LocalState\hexone.yaml`

	router := new(input.Router)
	render := func(label string) *image.RGBA {
		t.Helper()
		win, err := headless.NewWindow(width, height)
		if err != nil {
			t.Fatalf("headless window for %s: %v", label, err)
		}
		defer win.Release()
		var img *image.RGBA
		base := time.Now()
		for i := 0; i < 4; i++ {
			var ops op.Ops
			gtx := layout.Context{
				Ops:         &ops,
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Constraints: layout.Exact(image.Pt(width, height)),
				Now:         base.Add(time.Duration(i) * 100 * time.Millisecond),
				Source:      router.Source(),
			}
			ui.Layout(th, gtx)
			router.Frame(&ops)
			if err := win.Frame(&ops); err != nil {
				t.Fatalf("render %s frame: %v", label, err)
			}
			img = image.NewRGBA(image.Rect(0, 0, width, height))
			if err := win.Screenshot(img); err != nil {
				t.Fatalf("capture %s frame: %v", label, err)
			}
		}
		return img
	}
	writePNG := func(label string, img *image.RGBA) {
		t.Helper()
		path := filepath.Join(outDir, "settings-"+label+".png")
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("create screenshot: %v", err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatalf("encode screenshot: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("close screenshot: %v", err)
		}
		t.Logf("wrote %s", path)
	}
	ui.settingsModal.activeTab = "config"
	writePNG("config", render("config"))
	ui = NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	ui.settingsModal.activeTab = "viewer"
	router = new(input.Router)
	writePNG("viewer-line-numbers-on", render("viewer-line-numbers-on"))
	ui.settingsModal.viewShowLineNumbersBool.Value = false
	writePNG("viewer-line-numbers-dirty", render("viewer-line-numbers-dirty"))
	ui = NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	ui.settingsModal.activeTab = "terminal"
	ui.settingsModal.terminalPreviewStart = -1
	ui.settingsModal.terminalPreviewEnd = 3
	router = new(input.Router)
	writePNG("terminal-preview-range", render("terminal-preview-range"))
	for _, mode := range []string{"full", "brief", "other"} {
		ui = NewUI(fm.DefaultConfig())
		ui.openSettingsModal()
		ui.settingsModal.activeTab = "general"
		ui.settingsModal.paneSettingsMode = mode
		if mode == "other" {
			ui.settingsModal.generalUseTrash.Value = true
		} else {
			ui.settingsModal.paneFullChars++
		}
		router = new(input.Router)
		writePNG("file-panes-"+mode, render("file-panes-"+mode))
		if mode == "brief" {
			router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(294, 173)})
			writePNG("file-panes-brief-help", render("file-panes-brief-help"))
		}
		if mode == "full" {
			router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(294, 173)})
			writePNG("file-panes-full-help", render("file-panes-full-help"))
			ui.settingsModal.focus = settingsKeyboardFocusFilePaneMode
			router = new(input.Router)
			writePNG("file-panes-full-focused", render("file-panes-full-focused"))
		}
		if mode == "brief" {
			ui.settingsModal.paneBriefChars = 48
			router = new(input.Router)
			writePNG("file-panes-brief-wide", render("file-panes-brief-wide"))
		}
	}
}
