// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/platform"
	"hexone/ui/widget/table"
	"image"
	"image/color"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
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

func TestLayoutFilePaneTableSelectedRowClickQueuesInlineNameEditWhenPaneFocused(t *testing.T) {
	ui, pane, router, gtx, th := testFilePaneTableClickSetup(t, 0)
	testFilePaneTableFrame(ui, th, router, &gtx, 0, pane)
	testQueueFilePaneTablePress(t, router, pane)

	gtx.Now = gtx.Now.Add(time.Millisecond)
	testFilePaneTableFrame(ui, th, router, &gtx, 0, pane)

	if got := pane.inlineNamePendingRow; got != 0 {
		t.Fatalf("pending inline name row=%d want selected row 0", got)
	}
	if pane.inlineNameEditing {
		t.Fatal("selected-row click should queue inline rename after the double-click window, not start immediately")
	}
}

func TestLayoutFilePaneTableSelectedRowClickActivatesInactivePaneWithoutInlineNameEdit(t *testing.T) {
	ui, pane, router, gtx, th := testFilePaneTableClickSetup(t, 1)
	testFilePaneTableFrame(ui, th, router, &gtx, 0, pane)
	testQueueFilePaneTablePress(t, router, pane)

	gtx.Now = gtx.Now.Add(time.Millisecond)
	testFilePaneTableFrame(ui, th, router, &gtx, 0, pane)

	if got := ui.activeFilePane; got != 0 {
		t.Fatalf("active pane=%d want clicked pane 0", got)
	}
	if pane.inlineNamePendingRow >= 0 || pane.inlineNameEditing {
		t.Fatalf("activation click should not start or queue inline rename, pending=%d editing=%v", pane.inlineNamePendingRow, pane.inlineNameEditing)
	}
	if pane.tableClickRow >= 0 || pane.tableClickCol >= 0 || !pane.tableClickAt.IsZero() {
		t.Fatal("activation click should not become the first click of a later double-click")
	}
}

func TestLayoutFilePaneTableSelectedRowClickReleasesTerminalFocusWithoutInlineNameEdit(t *testing.T) {
	ui, pane, router, gtx, th := testFilePaneTableClickSetup(t, 0)
	ui.terminal = newTerminalSession(nil)
	ui.terminal.setActive(true)
	testFilePaneTableFrame(ui, th, router, &gtx, 0, pane)

	gtx.Execute(key.FocusCmd{Tag: &ui.terminal.keyTag})
	if !ui.terminalFocused(gtx) {
		t.Fatal("terminal should start focused")
	}
	testQueueFilePaneTablePress(t, router, pane)

	gtx.Now = gtx.Now.Add(time.Millisecond)
	testFilePaneTableFrame(ui, th, router, &gtx, 0, pane)

	if ui.terminalFocused(gtx) {
		t.Fatal("selected-row activation click should release terminal focus")
	}
	if pane.inlineNamePendingRow >= 0 || pane.inlineNameEditing {
		t.Fatalf("terminal focus transfer click should not start or queue inline rename, pending=%d editing=%v", pane.inlineNamePendingRow, pane.inlineNameEditing)
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
	baseWidth := ui.filePaneFavoriteMenuWidth(baseGtx)

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

func TestFavoriteMenuGeometryScalesWithInterfaceFont(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	gtx := testFavoriteMenuLayoutContext(image.Pt(800, 600), time.Now())
	baseWidth := ui.filePaneFavoriteMenuWidth(gtx)
	baseRowHeight := ui.filePaneFavoriteMenuRowHeight(gtx)

	cfg.Interface.FontSizeSp = 20
	largeWidth := ui.filePaneFavoriteMenuWidth(gtx)
	largeRowHeight := ui.filePaneFavoriteMenuRowHeight(gtx)
	if largeWidth <= baseWidth {
		t.Fatalf("large favorite width=%d want > %d", largeWidth, baseWidth)
	}
	if largeRowHeight <= baseRowHeight {
		t.Fatalf("large favorite row height=%d want > %d", largeRowHeight, baseRowHeight)
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
	size := ui.filePaneFavoriteMenuCardSize(gtx, items)
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
	if rect.Max.X > menuRect.Min.X+gtx.Dp(unit.Dp(7))+ui.filePaneFavoriteMenuTextWidth(gtx, item) {
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
	size := ui.filePaneFavoriteMenuCardSize(gtx, items)
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

func TestHandlePlatformInsertKeyTogglesCurrentRowOffAndAdvances(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	pane := ui.activePane()
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta.txt", Path: "beta.txt"},
			{Name: "gamma.txt", Path: "gamma.txt"},
		},
	}, "", "", 0)

	now := time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC)
	if !ui.HandlePlatformInsertKey(now) {
		t.Fatal("first HandlePlatformInsertKey should report a handled insert press")
	}
	pane.table.Selected = 0
	if !ui.HandlePlatformInsertKey(now) {
		t.Fatal("second HandlePlatformInsertKey should report a handled insert press")
	}
	if pane.isMarkedRow(0) {
		t.Fatal("native insert should toggle the current row off when pressed again")
	}
	if got, want := pane.table.Selected, 1; got != want {
		t.Fatalf("selected row=%d want %d", got, want)
	}
}

func TestHeldPlatformInsertMarksSuccessiveRows(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	pane := ui.activePane()
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta.txt", Path: "beta.txt"},
			{Name: "gamma.txt", Path: "gamma.txt"},
		},
	}, "", "", 0)

	start := time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC)
	if !ui.HandlePlatformInsertKeyState(start, true) {
		t.Fatal("initial Insert down should mark and start repeat")
	}
	if !pane.isMarkedRow(0) || pane.table.Selected != 1 || !ui.rep.active {
		t.Fatalf("initial Insert state: marked0=%v selected=%d repeat=%v", pane.isMarkedRow(0), pane.table.Selected, ui.rep.active)
	}

	gtx := layout.Context{Ops: new(op.Ops)}
	for ui.rep.active {
		gtx.Now = ui.rep.next
		gtx.Ops.Reset()
		ui.handleFileManagerKeys(gtx)
	}
	for row := 0; row < 3; row++ {
		if !pane.isMarkedRow(row) {
			t.Fatalf("row %d should be marked after holding Insert", row)
		}
	}
	if got, want := pane.table.Selected, 2; got != want {
		t.Fatalf("selected row=%d want final row %d", got, want)
	}

	ui.HandlePlatformInsertKeyState(gtx.Now, false)
	if ui.held[fileActionKey(fileActionMarkSelectNext)] || ui.rep.active {
		t.Fatal("Insert release should clear held and repeat state")
	}
}

func TestPlatformInsertReleaseBeforeRepeatKeepsSinglePressBehavior(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	pane := ui.activePane()
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta.txt", Path: "beta.txt"},
		},
	}, "", "", 0)

	start := time.Now()
	if !ui.HandlePlatformInsertKeyState(start, true) {
		t.Fatal("Insert down should be handled")
	}
	ui.HandlePlatformInsertKeyState(start.Add(repeatStartDelay/2), false)
	gtx := layout.Context{Ops: new(op.Ops), Now: start.Add(repeatStartDelay)}
	ui.handleFileManagerKeys(gtx)

	if !pane.isMarkedRow(0) || pane.isMarkedRow(1) {
		t.Fatal("quick Insert tap should mark only the initial row")
	}
	if got, want := pane.table.Selected, 1; got != want {
		t.Fatalf("selected row=%d want %d", got, want)
	}
}

func TestGioInsertPressAndReleaseControlsRepeat(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	pane := ui.activePane()
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta.txt", Path: "beta.txt"},
		},
	}, "", "", 0)

	gtx, router := testKeyContext()
	gtx.Now = time.Now()
	router.Event(key.Filter{Name: keyNameInsert})
	router.Queue(key.Event{Name: keyNameInsert, State: key.Press})
	ui.handleFileManagerKeys(gtx)

	if !pane.isMarkedRow(0) || pane.table.Selected != 1 || !ui.rep.active {
		t.Fatal("Gio Insert press should mark, advance, and start repeat")
	}

	router.Event(key.Filter{Name: keyNameInsert})
	router.Queue(key.Event{Name: keyNameInsert, State: key.Release})
	ui.handleFileManagerKeys(gtx)
	if ui.rep.active || ui.held[fileActionKey(fileActionMarkSelectNext)] {
		t.Fatal("Gio Insert release should stop repeat")
	}
}

func TestHandleFileManagerCtrlASelectAllToggles(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	pane := ui.activePane()
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "..", Path: "..", Kind: filesys.EntryParent},
			{Name: "docs", Path: "docs", Kind: filesys.EntryDir},
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta.log", Path: "beta.log"},
		},
	}, "", "", 1)

	gtx, router := testKeyContext()
	router.Event(key.Filter{Name: "A", Required: key.ModCtrl})
	router.Queue(key.Event{Name: "A", Modifiers: key.ModCtrl, State: key.Press})

	ui.handleFileManagerKeys(gtx)
	if pane.isMarkedRow(0) || !pane.isMarkedRow(1) || !pane.isMarkedRow(2) || !pane.isMarkedRow(3) {
		t.Fatal("ctrl+a should mark every selectable row and skip parent rows")
	}

	router.Event(key.Filter{Name: "A", Required: key.ModCtrl})
	router.Queue(key.Event{Name: "A", Modifiers: key.ModCtrl, State: key.Press})

	ui.handleFileManagerKeys(gtx)
	if pane.hasMarkedRows() {
		t.Fatal("ctrl+a should clear all marks when everything is already selected")
	}
}

func TestHandleFileManagerCtrlEMatchesExtension(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	pane := ui.activePane()
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta.log", Path: "beta.log"},
			{Name: "gamma.txt", Path: "gamma.txt"},
		},
	}, "", "", 0)

	gtx, router := testKeyContext()
	router.Event(key.Filter{Name: "E", Required: key.ModCtrl})
	router.Queue(key.Event{Name: "E", Modifiers: key.ModCtrl, State: key.Press})

	ui.handleFileManagerKeys(gtx)
	if !pane.isMarkedRow(0) || !pane.isMarkedRow(2) {
		t.Fatal("ctrl+e should mark rows with the same extension as the selected file")
	}
	if pane.isMarkedRow(1) {
		t.Fatal("ctrl+e should not mark rows with a different extension")
	}
}

func TestFileContextMenuPanelWidthShrinksToMeasuredItems(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(420, 240)},
	}
	spec := fileContextMenuSpec{
		Title:   "File Ops",
		WidthDp: filePaneContextMenuCompactWidthDp,
		Items: []fileContextMenuItem{
			fileContextMenuActionItem("copy", "Copy..", filePaneMenuActionCopyDialog),
			fileContextMenuActionItem("move", "Move..", filePaneMenuActionMoveDialog),
			fileContextMenuActionItem("delete", "Delete..", filePaneMenuActionDeleteDialog),
		},
	}

	got := ui.fileContextMenuPanelSize(th, gtx, spec).X
	oldCap := gtx.Dp(unit.Dp(filePaneContextMenuCompactWidthDp))
	if got >= oldCap {
		t.Fatalf("compact menu width=%d should shrink below cap %d", got, oldCap)
	}
	if got < gtx.Dp(unit.Dp(80)) {
		t.Fatalf("compact menu width=%d is too narrow", got)
	}
}

func TestGlobalFunctionKeysAlt1OpensLeftDrivePicker(t *testing.T) {
	drives := platform.AvailableLocalDrives()
	if len(drives) == 0 {
		t.Skip("drive picker is only available when local drives are exposed")
	}

	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	pane.dir = drives[0]
	pane.localDirBeforeRemote = drives[0]

	gtx, router := testKeyContext()
	anyMods := ^key.Modifiers(0)
	router.Event(key.Filter{Name: "1", Required: key.ModAlt, Optional: anyMods})
	router.Queue(key.Event{Name: "1", Modifiers: key.ModAlt, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)

	if !pane.driveMenuOpen {
		t.Fatal("alt+1 should open the left drive picker")
	}
}

func TestGlobalFunctionKeysAlt2OpensRightDrivePicker(t *testing.T) {
	drives := platform.AvailableLocalDrives()
	if len(drives) == 0 {
		t.Skip("drive picker is only available when local drives are exposed")
	}

	ui := NewUI(fm.DefaultConfig())
	right := ui.filePanes[1]
	right.dir = drives[0]
	right.localDirBeforeRemote = drives[0]

	gtx, router := testKeyContext()
	anyMods := ^key.Modifiers(0)
	router.Event(key.Filter{Name: "2", Required: key.ModAlt, Optional: anyMods})
	router.Queue(key.Event{Name: "2", Modifiers: key.ModAlt, State: key.Press})

	ui.handleGlobalFunctionKeys(gtx)

	if !right.driveMenuOpen {
		t.Fatal("alt+2 should open the right drive picker")
	}
}

func TestDriveMenuSelectionDefaultsToCurrentDriveAndWraps(t *testing.T) {
	pane := newFilePaneState(".", fm.DefaultConfig())
	drives := []string{`C:\`, `D:\`, `E:\`}
	if filepath.VolumeName(drives[0]) == "" {
		t.Skip("volume-style drive paths are not supported on this platform")
	}
	pane.dir = filepath.Join(`D:\`, "work")

	if got, want := pane.currentDriveMenuSelection(drives), 1; got != want {
		t.Fatalf("default drive selection=%d want %d", got, want)
	}
	if !pane.moveDriveMenuSelection(1, drives) {
		t.Fatal("moveDriveMenuSelection should advance the highlighted drive")
	}
	if got, want := pane.currentDriveMenuSelection(drives), 2; got != want {
		t.Fatalf("selection after moving down=%d want %d", got, want)
	}
	if !pane.moveDriveMenuSelection(1, drives) {
		t.Fatal("moveDriveMenuSelection should wrap from the end to the start")
	}
	if got, want := pane.currentDriveMenuSelection(drives), 0; got != want {
		t.Fatalf("wrapped selection=%d want %d", got, want)
	}
	if !pane.moveDriveMenuSelection(-1, drives) {
		t.Fatal("moveDriveMenuSelection should wrap from the start to the end")
	}
	if got, want := pane.currentDriveMenuSelection(drives), 2; got != want {
		t.Fatalf("wrapped selection after moving up=%d want %d", got, want)
	}
}

func TestActivatePaneDriveMenuSelectionLoadsSelectedDrive(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	drives := []string{`C:\`, `D:\`}
	if filepath.VolumeName(drives[0]) == "" {
		t.Skip("volume-style drive paths are not supported on this platform")
	}
	pane.dir = filepath.Join(`C:\`, "logs")
	pane.localDirBeforeRemote = pane.dir
	pane.driveMenuOpen = true
	pane.driveMenuSelected = 1

	if !ui.activatePaneDriveMenuSelection(0, pane, drives) {
		t.Fatal("activatePaneDriveMenuSelection should request loading the selected drive")
	}
	if pane.driveMenuOpen {
		t.Fatal("activating a drive selection should close the drive picker")
	}
	if got, want := pane.loadingDir, filepath.Clean(`D:\`); got != want {
		t.Fatalf("loadingDir=%q want %q", got, want)
	}
}

func TestFilePaneModelFilenameRulesApplyCachedColorAndIcon(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Colors.Filenames.Text = "#8899AA"
	cfg.Colors.Filenames.AgeRules = []fm.FilenameAgeRule{
		{MaxAge: "1h", Text: "#112233", Icon: fm.FilenameIconRecent},
	}
	cfg.Colors.Filenames.PermissionRules = []fm.FilenamePermissionRule{
		{Permissions: "0755", Text: "#445566", Icon: fm.FilenameIconLocked},
	}

	now := time.Date(2026, time.March, 20, 12, 0, 0, 0, time.UTC)
	model := &filePaneModel{
		cfg:           cfg,
		baseTextColor: color.NRGBA{R: 210, G: 210, B: 210, A: 255},
		filenameTheme: newFilePaneFilenameTheme(cfg),
		entries: []filesys.Entry{
			{Name: "fresh.log", DisplayName: "fresh.log", Kind: filesys.EntryFile, PermOctal: "0640", ModTime: now.Add(-20 * time.Minute)},
			{Name: "script.sh", DisplayName: "script.sh", Kind: filesys.EntryFile, PermOctal: "0755", ModTime: now.Add(-48 * time.Hour)},
			{Name: "plain.txt", DisplayName: "plain.txt", Kind: filesys.EntryFile, PermOctal: "0640", ModTime: now.Add(-48 * time.Hour)},
			{Name: "bundle.zip", DisplayName: "bundle.zip", Kind: filesys.EntryFile, PermOctal: "0640", ModTime: now.Add(-48 * time.Hour)},
			{Name: "photo.png", DisplayName: "photo.png", Kind: filesys.EntryFile, PermOctal: "0640", ModTime: now.Add(-48 * time.Hour)},
			{Name: "clip.mp4", DisplayName: "clip.mp4", Kind: filesys.EntryFile, PermOctal: "0640", ModTime: now.Add(-48 * time.Hour)},
		},
	}
	model.rebuildFilenameVisuals(now)

	if _, st := model.Cell(0, 0); st.Color != (color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xFF}) {
		t.Fatalf("recent file color=%v want #112233", st.Color)
	} else if !st.PreserveColor {
		t.Fatal("recent file color should preserve custom filename color on row states")
	}
	if icon, ok := model.LeadingIcon(0, 0); !ok || icon.Widget == nil {
		t.Fatal("recent file should expose a custom cached icon")
	}

	if _, st := model.Cell(1, 0); st.Color != (color.NRGBA{R: 0x44, G: 0x55, B: 0x66, A: 0xFF}) {
		t.Fatalf("permission override color=%v want #445566", st.Color)
	} else if !st.PreserveColor {
		t.Fatal("permission override should preserve custom filename color on row states")
	}
	if icon, ok := model.LeadingIcon(1, 0); !ok || icon.Widget == nil {
		t.Fatal("permission override should expose a custom cached icon")
	}

	if _, st := model.Cell(2, 0); st.Color != (color.NRGBA{R: 0x88, G: 0x99, B: 0xAA, A: 0xFF}) {
		t.Fatalf("default filename color=%v want #8899AA", st.Color)
	} else if !st.PreserveColor {
		t.Fatal("default filename rule should preserve custom filename color on row states")
	}
	if icon, ok := model.LeadingIcon(2, 0); !ok || icon.Widget != nil {
		t.Fatal("default filename rule should keep the stock file icon")
	}

	if icon, ok := model.LeadingIcon(3, 0); !ok || icon.Widget == nil {
		t.Fatal("archive files should use the stock archive icon by default")
	}

	if icon, ok := model.LeadingIcon(4, 0); !ok || icon.Widget == nil {
		t.Fatal("image files should use the stock image icon by default")
	}

	if icon, ok := model.LeadingIcon(5, 0); !ok || icon.Widget == nil {
		t.Fatal("video files should use the stock video icon by default")
	}
}

func TestFilePaneFilenameThemeRulePrecedenceSupportsPartialPermissions(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Colors.Filenames.AgeRules = []fm.FilenameAgeRule{
		{MaxAge: "1h", Text: "#111111", Icon: fm.FilenameIconRecent},
	}
	cfg.Colors.Filenames.ExtensionRules = []fm.FilenameExtensionRule{
		{Extension: ".tar.gz", Text: "#222222", Icon: fm.FilenameIconArchive},
		{Extension: ".sh", Text: "#2A2A2A", Icon: fm.FilenameIconCode},
	}
	cfg.Colors.Filenames.SizeRules = []fm.FilenameSizeRule{
		{Size: "1k", Match: fm.FilenameSizeMatchAtMost, Text: "#333333", Icon: fm.FilenameIconImage},
	}
	cfg.Colors.Filenames.PermissionRules = []fm.FilenamePermissionRule{
		{Permissions: "0111", Match: fm.FilenamePermissionMatchAny, Text: "#444444", Icon: fm.FilenameIconLocked},
		{Permissions: "0222", Match: fm.FilenamePermissionMatchNone, Text: "#555555", Icon: fm.FilenameIconDocument},
	}

	theme := newFilePaneFilenameTheme(cfg)
	now := time.Date(2026, time.March, 20, 12, 0, 0, 0, time.UTC)

	execVisual := theme.visualForEntry(filesys.Entry{
		Name:      "deploy.sh",
		Kind:      filesys.EntryFile,
		PermOctal: "0755",
		SizeBytes: 512,
		ModTime:   now.Add(-20 * time.Minute),
	}, now)
	if execVisual.color != (color.NRGBA{R: 0x44, G: 0x44, B: 0x44, A: 0xFF}) {
		t.Fatalf("exec visual color=%v want permission override", execVisual.color)
	}
	if execVisual.iconKey != fm.FilenameIconLocked {
		t.Fatalf("exec visual icon=%q want %q", execVisual.iconKey, fm.FilenameIconLocked)
	}

	readonlyVisual := theme.visualForEntry(filesys.Entry{
		Name:      "README.tar.gz",
		Kind:      filesys.EntryFile,
		PermOctal: "0444",
		SizeBytes: 512,
		ModTime:   now.Add(-48 * time.Hour),
	}, now)
	if readonlyVisual.color != (color.NRGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xFF}) {
		t.Fatalf("readonly visual color=%v want permission none override", readonlyVisual.color)
	}
	if readonlyVisual.iconKey != fm.FilenameIconDocument {
		t.Fatalf("readonly visual icon=%q want %q", readonlyVisual.iconKey, fm.FilenameIconDocument)
	}

	sizeVisual := theme.visualForEntry(filesys.Entry{
		Name:      "bundle.tar.gz",
		Kind:      filesys.EntryFile,
		PermOctal: "0644",
		SizeBytes: 512,
		ModTime:   now.Add(-48 * time.Hour),
	}, now)
	if sizeVisual.color != (color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}) {
		t.Fatalf("size visual color=%v want size override", sizeVisual.color)
	}
	if sizeVisual.iconKey != fm.FilenameIconImage {
		t.Fatalf("size visual icon=%q want %q", sizeVisual.iconKey, fm.FilenameIconImage)
	}

	extVisual := theme.visualForEntry(filesys.Entry{
		Name:      "release.tar.gz",
		Kind:      filesys.EntryFile,
		PermOctal: "0644",
		SizeBytes: 2048,
		ModTime:   now.Add(-48 * time.Hour),
	}, now)
	if extVisual.color != (color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF}) {
		t.Fatalf("extension visual color=%v want extension override", extVisual.color)
	}
	if extVisual.iconKey != fm.FilenameIconArchive {
		t.Fatalf("extension visual icon=%q want %q", extVisual.iconKey, fm.FilenameIconArchive)
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

func testFilePaneTableClickSetup(t *testing.T, activePane int) (*UI, *filePaneState, *input.Router, layout.Context, *material.Theme) {
	t.Helper()
	cfg := fm.DefaultConfig()
	dir := t.TempDir()
	path := filepath.Join(dir, "alpha.txt")
	pane := newFilePaneState(dir, cfg)
	pane.applyListing(filesys.Listing{
		Dir: dir,
		Entries: []filesys.Entry{
			{Name: "alpha.txt", DisplayName: "alpha.txt", Path: path},
		},
	}, path, "", 0)
	other := newFilePaneState(t.TempDir(), cfg)
	ui := &UI{
		fmCfg:          cfg,
		filePanes:      []*filePaneState{pane, other},
		activeFilePane: activePane,
		held:           make(map[string]bool),
	}
	router := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Now:    time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(640, 260),
		},
	}
	return ui, pane, router, gtx, material.NewTheme()
}

func testFilePaneTableFrame(ui *UI, th *material.Theme, router *input.Router, gtx *layout.Context, idx int, pane *filePaneState) {
	gtx.Ops.Reset()
	gtx.Source = router.Source()
	ui.layoutFilePaneTable(th, *gtx, idx, pane)
	router.Frame(gtx.Ops)
}

func testQueueFilePaneTablePress(t *testing.T, router *input.Router, pane *filePaneState) {
	t.Helper()
	rect, ok := pane.table.CellRect(0, 0, pane.model.Len())
	if !ok {
		t.Fatal("selected table cell should be visible after layout")
	}
	router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Buttons:  pointer.ButtonPrimary,
		Position: f32.Pt(float32(rect.Min.X+rect.Dx()/2), float32(rect.Min.Y+rect.Dy()/2)),
	})
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
