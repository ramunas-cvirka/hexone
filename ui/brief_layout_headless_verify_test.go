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
	"hexone/ui/widget/table"
)

func TestHeadlessBriefLayoutUsesDirectoryContentWidth(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	cfg := fm.DefaultConfig()
	cfg.Columns.BriefChars = 40
	dir := t.TempDir()
	names := []string{
		"mobilogix", "motor", "mta6", "mxt", "navigil", "navis",
		"navtelecom", "net", "noran", "nto", "nyitech", "oigo",
		"omnicomm", "opengts", "orbcomm", "osmand", "outsafe",
		"pacifictrack", "pebbell", "polte", "portman", "xirgo",
		"xt2400", "ywt", ".DS_Store", "devonthink_index.applescript",
		"protocol_sources.tsv", "README.md", "scan_mbox.py", "sort_mbox.py",
	}
	for _, name := range names {
		path := filepath.Join(dir, name)
		if filepath.Ext(name) == "" {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatalf("create directory %s: %v", name, err)
			}
		} else if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatalf("create file %s: %v", name, err)
		}
	}

	const width, height = 1200, 620
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("create headless window: %v", err)
	}
	defer win.Release()

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	ui := NewUI(cfg)
	for index, pane := range ui.filePanes {
		if pane == nil {
			continue
		}
		pane.table.SetMode(table.ModeBrief)
		ui.requestPaneLoadWithSelection(index, dir, "", "", 0)
	}
	router := new(input.Router)
	frame := func() *image.RGBA {
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
			t.Fatalf("render frame: %v", err)
		}
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		if err := win.Screenshot(img); err != nil {
			t.Fatalf("capture frame: %v", err)
		}
		return img
	}

	var img *image.RGBA
	loaded := false
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		img = frame()
		loaded = true
		for _, pane := range ui.filePanes {
			if pane == nil || pane.dir != dir || pane.findEntryIndex("devonthink_index.applescript") < 0 {
				loaded = false
				break
			}
		}
		if loaded {
			break
		}
		time.Sleep(12 * time.Millisecond)
	}
	if !loaded {
		t.Fatal("synthetic brief-mode directory did not load")
	}
	for frameIndex := 0; frameIndex < 4; frameIndex++ {
		img = frame()
	}

	path := filepath.Join(outDir, "brief-content-width.png")
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
