// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/widget/table"
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/font"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestLayoutFilePaneModeGlyphKeepsSameCanvasAcrossModes(t *testing.T) {
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(16, 11)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}

	full := layoutFilePaneModeGlyph(gtx, table.ModeFull, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	gtx.Ops = new(op.Ops)
	brief := layoutFilePaneModeGlyph(gtx, table.ModeBrief, color.NRGBA{R: 255, G: 255, B: 255, A: 255})

	if full.Size != brief.Size {
		t.Fatalf("mode glyph size should stay stable across modes, got full=%v brief=%v", full.Size, brief.Size)
	}
}

func TestFavoriteMenuCardWidthStaysFixedWhileRevealExpandsAfterDelay(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	start := time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC)
	pane := newFilePaneState(".", ui.fmCfg)
	item := fileFavoriteItem{
		label:     "/Users/ramunas/projects/hexone/a/path/that/should/reveal/inline/after-hover.txt",
		targetDir: "/Users/ramunas/projects/hexone/a/path/that/should/reveal/inline/after-hover.txt",
		removable: true,
	}
	items := []fileFavoriteItem{item}
	pane.ensureFavoriteOptionClicks(len(items))
	pane.ensureFavoriteRemoveClicks(len(items))

	baseGtx := testFavoriteMenuLayoutContext(image.Pt(520, 240), start)
	baseWidth := filePaneFavoriteMenuWidth(baseGtx)

	pane.favoriteHoverKey = item.targetDir
	pane.favoriteHoverAt = start

	beforeGtx := testFavoriteMenuLayoutContext(image.Pt(520, 240), start.Add(filePaneFavoriteRevealDelay-time.Millisecond))
	beforeWidth := ui.layoutFilePaneFavoriteMenuCard(th, beforeGtx, pane, items, 1).Size.X
	if beforeWidth != baseWidth {
		t.Fatalf("favorite menu width before reveal=%d want base width %d", beforeWidth, baseWidth)
	}

	afterGtx := testFavoriteMenuLayoutContext(image.Pt(520, 240), start.Add(filePaneFavoriteRevealDelay))
	afterWidth := ui.layoutFilePaneFavoriteMenuCard(th, afterGtx, pane, items, 1).Size.X
	if afterWidth != beforeWidth {
		t.Fatalf("favorite menu width after reveal=%d want unchanged width %d", afterWidth, beforeWidth)
	}

	fullWidth, trimmedWidth, hiddenPrefixWidth, ellipsisWidth := ui.favoriteMenuLabelMetrics(th, afterGtx, item)
	revealWidth := ui.favoriteMenuRevealWidth(th, afterGtx, item)
	if revealWidth <= afterWidth {
		t.Fatalf("favorite reveal width=%d want > fixed menu width %d", revealWidth, afterWidth)
	}
	if hiddenPrefixWidth <= 0 {
		t.Fatalf("hidden prefix width=%d want positive", hiddenPrefixWidth)
	}
	if ellipsisWidth <= 0 {
		t.Fatalf("ellipsis width=%d want positive", ellipsisWidth)
	}
	if want := afterWidth + hiddenPrefixWidth - ellipsisWidth; revealWidth != want {
		t.Fatalf("favorite reveal width=%d want anchored expansion width %d", revealWidth, want)
	}
	if fullWidth <= trimmedWidth {
		t.Fatalf("full width=%d want > trimmed width %d", fullWidth, trimmedWidth)
	}
}

func TestFavoriteRevealAlphaFadesImmediatelyAfterHide(t *testing.T) {
	start := time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC)
	pane := newFilePaneState(".", fm.DefaultConfig())
	item := fileFavoriteItem{
		label:     "/tmp/example",
		targetDir: "/tmp/example",
	}
	pane.favoriteRevealKey = item.targetDir
	pane.favoriteRevealHideAt = start

	if got := filePaneFavoriteRevealAlpha(pane, pane.favoriteRevealHideAt, item); got != 1 {
		t.Fatalf("reveal alpha at hideAt=%v want 1", got)
	}
	mid := pane.favoriteRevealHideAt.Add(filePaneFavoriteRevealFadeDur / 2)
	if got := filePaneFavoriteRevealAlpha(pane, mid, item); got >= 1 || got <= 0 {
		t.Fatalf("reveal alpha mid-fade=%v want between 0 and 1", got)
	}
	end := pane.favoriteRevealHideAt.Add(filePaneFavoriteRevealFadeDur)
	if got := filePaneFavoriteRevealAlpha(pane, end, item); got != 0 {
		t.Fatalf("reveal alpha at fade end=%v want 0", got)
	}
}

func TestFavoriteRevealHotspotRectUsesLeadingEllipsisArea(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC)
	item := fileFavoriteItem{
		label:     "/Users/ramunas/projects/hexone/a/path/that/should/reveal/from/the/front.txt",
		targetDir: "/Users/ramunas/projects/hexone/a/path/that/should/reveal/from/the/front.txt",
		removable: true,
	}
	items := []fileFavoriteItem{item}
	gtx := testFavoriteMenuLayoutContext(image.Pt(520, 240), now)
	size := filePaneFavoriteMenuCardSize(gtx, items)
	menuRect := image.Rectangle{Min: image.Pt(260, 40), Max: image.Pt(260+size.X, 40+size.Y)}
	rect := ui.favoriteMenuRevealHotspotRect(th, gtx, menuRect, items, 0, item)
	_, _, hiddenPrefixWidth, ellipsisWidth := ui.favoriteMenuLabelMetrics(th, gtx, item)

	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		t.Fatalf("hotspot rect=%v want non-empty", rect)
	}
	if hiddenPrefixWidth <= 0 || ellipsisWidth <= 0 {
		t.Fatalf("expected truncated favorite label, got hiddenPrefix=%d ellipsis=%d", hiddenPrefixWidth, ellipsisWidth)
	}
	if want := menuRect.Min.X + gtx.Dp(unit.Dp(7)); rect.Min.X != want {
		t.Fatalf("hotspot min x=%d want text inset %d", rect.Min.X, want)
	}
	if rect.Dx() < ellipsisWidth {
		t.Fatalf("hotspot width=%d want at least ellipsis width %d", rect.Dx(), ellipsisWidth)
	}
	if rect.Max.X > menuRect.Min.X+gtx.Dp(unit.Dp(7))+filePaneFavoriteMenuTextWidth(gtx, item) {
		t.Fatalf("hotspot rect=%v should stay within text area of menu rect=%v", rect, menuRect)
	}
}

func TestFavoriteRevealHotspotKeyUsesPointerPosition(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC)
	pane := newFilePaneState(".", fm.DefaultConfig())
	item := fileFavoriteItem{
		label:     "/Users/ramunas/projects/hexone/a/path/that/should/reveal/from/the/front.txt",
		targetDir: "/Users/ramunas/projects/hexone/a/path/that/should/reveal/from/the/front.txt",
		removable: true,
	}
	items := []fileFavoriteItem{item}
	gtx := testFavoriteMenuLayoutContext(image.Pt(520, 240), now)
	size := filePaneFavoriteMenuCardSize(gtx, items)
	menuRect := image.Rectangle{Min: image.Pt(260, 40), Max: image.Pt(260+size.X, 40+size.Y)}
	hotspot := ui.favoriteMenuRevealHotspotRect(th, gtx, menuRect, items, 0, item)

	pane.favoritePointerPos = image.Pt(hotspot.Min.X+1, hotspot.Min.Y+hotspot.Dy()/2)
	pane.favoritePointerPosSet = true
	if got := ui.filePaneFavoriteRevealHotspotKey(th, gtx, pane, menuRect, items); got != item.targetDir {
		t.Fatalf("hotspot key=%q want %q", got, item.targetDir)
	}

	pane.favoritePointerPos = image.Pt(hotspot.Max.X+1, hotspot.Min.Y+hotspot.Dy()/2)
	if got := ui.filePaneFavoriteRevealHotspotKey(th, gtx, pane, menuRect, items); got != "" {
		t.Fatalf("hotspot key outside ellipsis area=%q want empty", got)
	}
}

func TestFavoriteMenuItemStyleHoverIncreasesContrast(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	theme := ui.filePanePopupTheme()
	item := fileFavoriteItem{
		label:     "/tmp/example",
		targetDir: "/tmp/example",
	}

	baseBg, _, baseWeight, _ := filePaneFavoriteMenuItemStyle(theme, item, 0, 1)
	hoverBg, _, hoverWeight, _ := filePaneFavoriteMenuItemStyle(theme, item, 1, 1)

	if hoverBg.A <= baseBg.A {
		t.Fatalf("hover bg alpha=%d want > base alpha=%d", hoverBg.A, baseBg.A)
	}
	if hoverBg == baseBg {
		t.Fatalf("hover bg=%v want different from base bg=%v", hoverBg, baseBg)
	}
	if got := favoriteMenuColorDistance(theme.Bg, hoverBg); got < 40 {
		t.Fatalf("hover bg distance from popup bg=%d want >= 40", got)
	}
	if hoverWeight != font.Medium || baseWeight != font.Normal {
		t.Fatalf("weights base=%v hover=%v want normal->medium", baseWeight, hoverWeight)
	}
}

func TestHandlePlatformInsertKeyMarksAndAdvances(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	pane := ui.activePane()
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta.txt", Path: "beta.txt"},
		},
	}, "", "", 0)

	now := time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC)
	if !ui.HandlePlatformInsertKey(now) {
		t.Fatal("HandlePlatformInsertKey should report a handled insert press")
	}
	if pane.noticeText != "" {
		t.Fatalf("notice=%q want empty", pane.noticeText)
	}
	if !pane.isMarkedRow(0) {
		t.Fatal("native insert should mark the selected row")
	}
	if got, want := pane.table.Selected, 1; got != want {
		t.Fatalf("selected row=%d want %d", got, want)
	}
}

func favoriteMenuColorDistance(a, b color.NRGBA) int {
	abs := func(v int) int {
		if v < 0 {
			return -v
		}
		return v
	}
	return abs(int(a.R)-int(b.R)) + abs(int(a.G)-int(b.G)) + abs(int(a.B)-int(b.B))
}

func testFavoriteMenuLayoutContext(size image.Point, now time.Time) layout.Context {
	return layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
		Now:    now,
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: size,
		},
	}
}
