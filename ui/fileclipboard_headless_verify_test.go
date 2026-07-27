// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build fileclipboardverify

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
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func fileClipboardVerifyFontCollection(t *testing.T) []text.FontFace {
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

func TestHeadlessFileClipboardContextMenu(t *testing.T) {
	outDir := os.Getenv("FILE_CLIPBOARD_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range []string{"alpha.txt", "bravo.txt", "charlie.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	clipboardDir := t.TempDir()
	clipboardPaths := make([]string, 0, 3)
	for _, name := range []string{"pasted-one.txt", "pasted-two.txt", "pasted-three.txt"} {
		path := filepath.Join(clipboardDir, name)
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		clipboardPaths = append(clipboardPaths, path)
	}

	const width, height = 1100, 760
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatal(err)
	}
	defer win.Release()
	cfg := fm.DefaultConfig()
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(fileClipboardVerifyFontCollection(t)))
	th.Face = font.Typeface(cfg.General.Typeface)
	th.TextSize = unit.Sp(cfg.General.FontSizeSp)
	ui := NewUI(cfg)
	router := new(input.Router)
	frame := func() *image.RGBA {
		var ops op.Ops
		gtx := layout.Context{
			Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)), Now: time.Now(), Source: router.Source(),
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
		return img
	}

	pane := ui.filePanes[0]
	selectedPath := filepath.Join(dir, "alpha.txt")
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

	oldRead := readFileClipboardFilesFunc
	readFileClipboardFilesFunc = func() ([]string, error) {
		return clipboardPaths, nil
	}
	defer func() { readFileClipboardFilesFunc = oldRead }()
	ui.openFilePaneContextMenu(0, pane.table.Selected, image.Pt(220, 210), time.Now())
	img := frame()
	time.Sleep(180 * time.Millisecond)
	img = frame()

	outPath := filepath.Join(outDir, "file-clipboard-context-menu.png")
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

	ui.handleFilePaneContextMenuAction(0, pane, pane.table.Selected, filePaneMenuActionPasteFiles, time.Now())
	img = frame()
	directPath := filepath.Join(outDir, "file-clipboard-direct-paste.png")
	file, err = os.Create(directPath)
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
	t.Logf("wrote %s", directPath)

	for deadline := time.Now().Add(3 * time.Second); ui.fileCopy != nil && time.Now().Before(deadline); {
		frame()
		time.Sleep(12 * time.Millisecond)
	}
	if ui.fileCopy != nil {
		t.Fatal("direct paste did not complete")
	}
	for _, source := range clipboardPaths {
		target := filepath.Join(dir, filepath.Base(source))
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("missing pasted file %s: %v", target, err)
		}
	}
}
