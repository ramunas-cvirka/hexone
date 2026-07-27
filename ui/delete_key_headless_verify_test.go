// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build deletekeyverify

package ui

import (
	resources "hexone"
	"hexone/fm"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestHeadlessFilePaneDeleteKey(t *testing.T) {
	outDir := os.Getenv("DELETE_KEY_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	selectedPath := filepath.Join(dir, "selected-for-delete.txt")
	if err := os.WriteFile(selectedPath, []byte("keep until confirmed"), 0o600); err != nil {
		t.Fatal(err)
	}

	const width, height = 1100, 760
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatal(err)
	}
	defer win.Release()

	cfg := fm.DefaultConfig()
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(deleteKeyVerifyFontCollection(t)))
	th.Face = font.Typeface(cfg.General.Typeface)
	th.TextSize = unit.Sp(cfg.General.FontSizeSp)
	ui := NewUI(cfg)
	router := new(input.Router)
	now := time.Now()
	frame := func() *image.RGBA {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)),
			Now:         now,
			Source:      router.Source(),
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
		now = now.Add(16 * time.Millisecond)
		return img
	}

	pane := ui.filePanes[0]
	ui.requestPaneLoadWithSelection(0, dir, selectedPath, "", 0)
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		frame()
		if entry := pane.selectedEntry(); entry != nil && entry.Path == selectedPath {
			break
		}
		time.Sleep(12 * time.Millisecond)
	}
	if entry := pane.selectedEntry(); entry == nil || entry.Path != selectedPath {
		t.Fatalf("file selection did not load: %+v", entry)
	}

	// The preceding frame registered the real application key filters.
	router.Queue(key.Event{Name: key.NameDeleteForward, State: key.Press})
	frame()
	img := frame()
	if ui.fileDelete == nil {
		t.Fatal("Delete key did not open the file delete confirmation")
	}
	if ui.fileDelete.targetPath != selectedPath {
		t.Fatalf("delete target = %q, want %q", ui.fileDelete.targetPath, selectedPath)
	}
	if _, err := os.Stat(selectedPath); err != nil {
		t.Fatalf("confirmation should leave the selected file in place: %v", err)
	}

	outPath := filepath.Join(outDir, "file-pane-delete-key.png")
	file, err := os.Create(outPath)
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
	t.Logf("wrote %s", outPath)
}

func deleteKeyVerifyFontCollection(t *testing.T) []text.FontFace {
	t.Helper()
	collection := make([]text.FontFace, 0, 8)
	for _, family := range resources.BundledFontFamilies() {
		regularData, ok := resources.BundledFont(family.RegularPath)
		if !ok {
			t.Fatalf("missing bundled font %s", family.RegularPath)
		}
		regular, err := opentype.Parse(regularData)
		if err != nil {
			t.Fatalf("parse %s: %v", family.RegularPath, err)
		}
		boldData, ok := resources.BundledFont(family.BoldPath)
		if !ok {
			t.Fatalf("missing bundled font %s", family.BoldPath)
		}
		bold, err := opentype.Parse(boldData)
		if err != nil {
			t.Fatalf("parse %s: %v", family.BoldPath, err)
		}
		collection = append(collection,
			text.FontFace{Font: font.Font{Typeface: font.Typeface(family.Name), Weight: font.Normal}, Face: regular},
			text.FontFace{Font: font.Font{Typeface: font.Typeface(family.Name), Weight: font.Bold}, Face: bold},
		)
	}
	return collection
}
