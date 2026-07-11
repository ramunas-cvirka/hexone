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
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("headless window: %v", err)
	}
	defer win.Release()

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
		var img *image.RGBA
		for i := 0; i < 4; i++ {
			var ops op.Ops
			gtx := layout.Context{
				Ops:         &ops,
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Constraints: layout.Exact(image.Pt(width, height)),
				Now:         time.Now(),
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
	for _, mode := range []string{"full", "brief", "other"} {
		ui = NewUI(fm.DefaultConfig())
		ui.openSettingsModal()
		ui.settingsModal.activeTab = "general"
		ui.settingsModal.paneSettingsMode = mode
		ui.settingsModal.paneFullChars++ // verify the dirty Save label.
		router = new(input.Router)
		writePNG("file-panes-"+mode, render("file-panes-"+mode))
	}
}
