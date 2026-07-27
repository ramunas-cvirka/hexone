// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build uiverify

package ui

import (
	resources "hexone"
	"hexone/filesys"
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

func TestHeadlessPathHeader(t *testing.T) {
	outDir := os.Getenv("PATH_HEADER_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ui := NewUI(fm.DefaultConfig())
	left := ui.filePanes[0]
	left.cancelPendingLoad()
	left.applyListing(pathHeaderVerifyListing("/Users/ramunas/go/src/gpstrack-go"), "", "", 0)
	leftSibling := newFilePaneState("/Users/ramunas/go/src", ui.fmCfg)
	leftSibling.cancelPendingLoad()
	leftSibling.applyListing(pathHeaderVerifyListing("/Users/ramunas/go/src"), "", "", 0)
	rightSibling := newFilePaneState("/Users/ramunas/go/src/git", ui.fmCfg)
	rightSibling.cancelPendingLoad()
	rightSibling.applyListing(pathHeaderVerifyListing("/Users/ramunas/go/src/git"), "", "", 0)
	ui.filePaneTabs[0].tabs = []*filePaneState{leftSibling, left, rightSibling}
	ui.filePaneTabs[0].active = 1
	ui.filePanes[0] = left
	if len(ui.filePanes) > 1 {
		ui.filePanes[1].cancelPendingLoad()
		ui.filePanes[1].applyListing(pathHeaderVerifyListing("/srv/assets/production"), "", "", 0)
	}
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(pathHeaderVerifyFontCollection(t)))
	router := new(input.Router)

	render := func(name string, size image.Point) {
		win, err := headless.NewWindow(size.X, size.Y)
		if err != nil {
			t.Fatalf("headless window: %v", err)
		}
		defer win.Release()
		base := time.Now()
		for frame := 0; frame < 2; frame++ {
			var ops op.Ops
			gtx := layout.Context{
				Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Constraints: layout.Exact(size), Now: base.Add(time.Duration(frame) * 100 * time.Millisecond), Source: router.Source(),
			}
			ui.Layout(th, gtx)
			router.Frame(&ops)
			if err := win.Frame(&ops); err != nil {
				t.Fatal(err)
			}
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
	}

	render("path-header-default.png", image.Pt(1100, 620))
	ui.fmCfg.Tabs.FontSizeSp = 24
	render("path-header-large-tabs.png", image.Pt(1100, 620))
	if left.tabHeight <= tabStripHeightDp {
		t.Fatalf("large tab font kept compact strip height %d", left.tabHeight)
	}
	left.openSortMenu(time.Now().Add(-time.Second))
	render("path-header-large-tabs-popup.png", image.Pt(1100, 620))
	left.closeSortMenu()
	ui.fmCfg.Tabs.FontSizeSp = 10
	fontVariants := []struct {
		name   string
		family string
	}{
		{name: "firacode", family: resources.BundledFontFamilyFiraCodeNerdFontMono},
		{name: "jetbrains", family: resources.BundledFontFamilyJetBrainsMonoNerdFontMono},
		{name: "hack", family: resources.BundledFontFamilyHackNerdFontMono},
		{name: "iosevka", family: resources.BundledFontFamilyIosevkaNerdFontMono},
	}
	for _, variant := range fontVariants {
		ui.fmCfg.CurrentDir.Typeface = variant.family
		render("path-header-"+variant.name+".png", image.Pt(1100, 620))
	}
	ui.fmCfg.CurrentDir.Typeface = resources.BundledFontFamilyIosevkaNerdFontMono
	left.sortDesc = true
	render("path-header-iosevka-descending.png", image.Pt(1100, 620))
	left.sortDesc = false
	render("path-header-compact.png", image.Pt(760, 520))
	dotPrefixedDir := filepath.Join(string(filepath.Separator), "Games", "Diablo II Resurrected", ".battle.net", "ecache")
	left.applyListing(pathHeaderVerifyListing(dotPrefixedDir), "", "", 0)
	for _, variant := range fontVariants {
		ui.fmCfg.CurrentDir.Typeface = variant.family
		render("path-header-ellipsis-"+variant.name+".png", image.Pt(760, 520))
	}
	ui.fmCfg.CurrentDir.Typeface = resources.BundledFontFamilyIosevkaNerdFontMono
	left.applyListing(pathHeaderVerifyListing("/Users/ramunas/go/src/hexone/ui"), "", "", 0)
	if err := left.setFilter("*.go;*.md"); err != nil {
		t.Fatal(err)
	}
	render("path-header-filtered.png", image.Pt(1100, 620))
	if !ui.activateFilePanePathSegment(0, left, left.dir) {
		t.Fatal("current-directory breadcrumb did not reset the filter")
	}
	render("path-header-filter-reset.png", image.Pt(1100, 620))
	left.beginPathEdit()
	render("path-header-editor.png", image.Pt(1100, 620))
	left.stopPathEdit()
	ui.fmCfg.CurrentDir.FontSizeSp = 18
	render("path-header-large-font.png", image.Pt(1100, 620))
	ui.fmCfg.CurrentDir.FontSizeSp = 11
	left.openSortMenu(time.Now().Add(-time.Second))
	render("path-header-sort-popup.png", image.Pt(1100, 620))
	left.closeSortMenu()
	left.openFavoriteMenu(time.Now().Add(-time.Second))
	render("path-header-favorite-popup.png", image.Pt(1100, 620))
	left.closeFavoriteMenu()
	ui.openSettingsModal()
	ui.settingsModal.activeTab = "fonts"
	render("settings-current-dir-font.png", image.Pt(900, 620))
	ui.settingsModal.currentDirFontSizeSp++
	render("settings-current-dir-font-dirty.png", image.Pt(900, 620))
	ui.closeSettingsModal()
	ui.fmCfg.Interface.FontSizeSp = 18
	ui.fmCfg.CurrentDir.FontSizeSp = 16
	ui.openSettingsModal()
	ui.settingsModal.activeTab = "fonts"
	render("settings-font-label-width.png", image.Pt(800, 600))
	ui.settingsModal.activeTab = "colors"
	ui.settingsModal.colorScope = "panes"
	ui.settingsModal.setColorCategory("focused_selected")
	ui.settingsModal.setColorTextTransparent(true)
	render("settings-color-checkbox-alignment.png", image.Pt(800, 600))
}

func pathHeaderVerifyFontCollection(t *testing.T) []text.FontFace {
	t.Helper()
	collection := make([]text.FontFace, 0, len(resources.BundledFontFamilies())*2)
	for _, family := range resources.BundledFontFamilies() {
		for _, variant := range []struct {
			path   string
			weight font.Weight
		}{
			{path: family.RegularPath, weight: font.Normal},
			{path: family.BoldPath, weight: font.Bold},
		} {
			data, ok := resources.BundledFont(variant.path)
			if !ok {
				t.Fatalf("missing bundled font %s", variant.path)
			}
			face, err := opentype.Parse(data)
			if err != nil {
				t.Fatalf("parse bundled font %s: %v", variant.path, err)
			}
			collection = append(collection, text.FontFace{
				Font: font.Font{Typeface: font.Typeface(family.Name), Weight: variant.weight},
				Face: face,
			})
		}
	}
	return collection
}

func pathHeaderVerifyListing(dir string) filesys.Listing {
	return filesys.Listing{Dir: dir, Entries: []filesys.Entry{
		{Name: "..", DisplayName: "..", Kind: filesys.EntryParent, Path: filepath.Dir(dir)},
		{Name: "cmd", DisplayName: "cmd", Kind: filesys.EntryDir, Path: filepath.Join(dir, "cmd")},
		{Name: "assets", DisplayName: "assets", Kind: filesys.EntryDir, Path: filepath.Join(dir, "assets")},
		{Name: "main.go", DisplayName: "main.go", Kind: filesys.EntryFile, Path: filepath.Join(dir, "main.go"), SizeText: "4.2 KB", DateText: "Jul 21 12:30"},
		{Name: "README.md", DisplayName: "README.md", Kind: filesys.EntryFile, Path: filepath.Join(dir, "README.md"), SizeText: "8.1 KB", DateText: "Jul 21 11:48"},
		{Name: "notes.txt", DisplayName: "notes.txt", Kind: filesys.EntryFile, Path: filepath.Join(dir, "notes.txt"), SizeText: "932 B", DateText: "Jul 20 19:02"},
	}}
}
