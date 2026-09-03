// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build uiverify

package ui

import (
	"image"
	"image/color"
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

// TestHeadlessSettingsStatusBarTab renders the file pane Status bar tab: the
// master switch, the hide-in-full row, the five field checkboxes, the
// date-layout picker, and the preview — a pane mock inside the shared 154dp
// frame, a brief-mode grid of sample rows with one highlighted and a single
// status bar strip along the bottom edge, rendered through the live bar's
// anchored layout and scaled into the frame when the configuration needs a
// wider pane than the frame is.
//
// Named to be picked up by `-run TestHeadlessSettings` alongside
// TestHeadlessSettingsConfig.
func TestHeadlessSettingsStatusBarTab(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	// Taller than the sibling settings capture on purpose: this tab is seven
	// checkboxes, a picker and the 154dp preview frame, and at 600px the
	// frame's bottom edge — where the status bar strips sit — falls outside the
	// scroll viewport, so the one thing worth looking at would be clipped away.
	const width, height = 800, 820
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	render := func(t *testing.T, ui *UI, router *input.Router, label string) *image.RGBA {
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
	writePNG := func(t *testing.T, label string, img *image.RGBA) {
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

	newTab := func(t *testing.T) (*UI, *settingsModalState) {
		t.Helper()
		ui := NewUI(fm.DefaultConfig())
		ui.openSettingsModal()
		if ui.settingsModal == nil {
			t.Fatal("settings modal did not open")
		}
		ui.settingsModal.activeTab = "general"
		ui.settingsModal.paneSettingsMode = "statusbar"
		return ui, ui.settingsModal
	}

	// Defaults: size, date and free space ticked; the date picker on auto.
	ui, _ := newTab(t)
	writePNG(t, "file-panes-statusbar", render(t, ui, new(input.Router), "file-panes-statusbar"))

	// Every field on, which is the widest strip the preview can produce; the
	// mock scales down further to keep every column visible.
	ui, st := newTab(t)
	for i := range st.statusBarFieldBools {
		st.statusBarFieldBools[i].Value = true
	}
	writePNG(t, "file-panes-statusbar-all-fields", render(t, ui, new(input.Router), "file-panes-statusbar-all-fields"))

	// The date-layout picker on the US form: the picker highlights its rendered
	// sample, and the preview's date column follows it in the same frame.
	ui, st = newTab(t)
	st.statusBarDateFormat = fm.StatusBarDateFormatUS
	writePNG(t, "file-panes-statusbar-date-us", render(t, ui, new(input.Router), "file-panes-statusbar-date-us"))

	// Bar switched off: the hide-in-full box, the five field boxes and the
	// date-layout picker grey out, and the preview shows an empty pane.
	ui, st = newTab(t)
	st.statusBarEnabledBool.Value = false
	writePNG(t, "file-panes-statusbar-disabled", render(t, ui, new(input.Router), "file-panes-statusbar-disabled"))

	// Keyboard focus on a field checkbox, which is only reachable because the
	// five fields have focus targets of their own.
	ui, st = newTab(t)
	st.setKeyboardFocus(settingsKeyboardFocusStatusBarField(filePaneStatusFieldOwner))
	writePNG(t, "file-panes-statusbar-field-focused", render(t, ui, new(input.Router), "file-panes-statusbar-field-focused"))
}

// The preview-anchor capture is rendered wider than any width
// settingsStatusBarPreviewPaneWidth can ask for, so
// layoutSettingsStatusBarPreviewPane takes its unscaled path and the pixel
// assertions can use the live bar's exact geometry — the 8px insets — without a
// scale factor blurring them.
const (
	statusBarPreviewVerifyWidth  = 960
	statusBarPreviewVerifyHeight = 200
)

// TestHeadlessSettingsStatusBarPreviewAnchors reads the preview widget's own
// pixels the way TestHeadlessFilePaneStatusBar reads the live strip's: the mock
// draws a grid of sample rows with one highlighted, exactly ONE strip along the
// frame's bottom edge, the strip's name starts at the left inset and its
// free-space text ends flush on the right one, and switching the bar off leaves
// a uniform empty pane. This is what proves the preview renders a pane with an
// anchored status bar rather than a lookalike.
func TestHeadlessSettingsStatusBarPreviewAnchors(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}

	win, err := headless.NewWindow(statusBarPreviewVerifyWidth, statusBarPreviewVerifyHeight)
	if err != nil {
		t.Fatalf("create headless window: %v", err)
	}
	defer win.Release()

	render := func() *image.RGBA {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(statusBarPreviewVerifyWidth, statusBarPreviewVerifyHeight)},
			Now:         time.Now(),
		}
		ui.layoutSettingsStatusBarPreview(th, gtx, st)
		if err := win.Frame(&ops); err != nil {
			t.Fatalf("render frame: %v", err)
		}
		img := image.NewRGBA(image.Rect(0, 0, statusBarPreviewVerifyWidth, statusBarPreviewVerifyHeight))
		if err := win.Screenshot(img); err != nil {
			t.Fatalf("capture frame: %v", err)
		}
		return img
	}

	img := render()
	writeStatusBarVerifyPNG(t, outDir, "settings-preview", img)

	// The frame is 154dp tall with a 23dp header; the single strip is pinned to
	// its bottom edge. Strip height is the live bar's: one line of the strip's
	// text style plus 4dp of inset above and below.
	const frameBottom = 154
	const contentTop = 23
	probe := material.Body2(th, "0")
	probe.Font.Typeface = ui.mainTypeface()
	probe.TextSize = scaleThemeFontSize(th, 11)
	probe.MaxLines = 1
	probe.Truncator = ""
	var measureOps op.Ops
	mgtx := layout.Context{
		Ops:         &measureOps,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(statusBarPreviewVerifyWidth, statusBarPreviewVerifyHeight)},
		Now:         time.Now(),
	}
	stripH := measureLabelUnconstrained(mgtx, probe).Size.Y + 2*mgtx.Dp(unit.Dp(4))

	x0, x1 := 0, statusBarPreviewVerifyWidth
	stripTop := frameBottom - stripH

	palette := ui.settingsPaneDraftPalette(st)
	paneBg := color.RGBA{R: palette.PaneBg.R, G: palette.PaneBg.G, B: palette.PaneBg.B, A: palette.PaneBg.A}

	// The strip's own fill, read from its bottom inset row.
	stripBg, uniform := statusBarVerifyRowColor(img, frameBottom-1, x0, x1)
	if !uniform {
		t.Fatalf("the strip's bottom inset row is not uniform; its text overflows the strip")
	}
	// filePaneVolumeBadgeColors gives the strip the pane's own fill and marks it
	// off with a border alone, so stripBg and paneBg may well be the same colour
	// — the border row below is what separates them.

	// The strip draws a 1px top border differing from both fills; counting those
	// uniform rows finds exactly one strip. Two would mean the retired
	// cursor/marked pair came back, zero that no strip drew at all.
	borders := 0
	borderY := -1
	for y := contentTop; y < frameBottom; y++ {
		if c, ok := statusBarVerifyRowColor(img, y, x0, x1); ok && c != paneBg && c != stripBg {
			borders++
			borderY = y
		}
	}
	if borders != 1 {
		t.Errorf("found %d uniform border rows in the preview frame, want exactly 1 — the mock has a single status bar", borders)
	} else if borderY != stripTop {
		t.Errorf("the strip's border row is at y=%d, want the bottom-anchored strip to start at y=%d", borderY, stripTop)
	}

	// Left anchor: the 8px inset is clean and the name column's first glyph
	// starts right after it.
	textTop, textBottom := stripTop+2, frameBottom-2
	if ink := statusBarVerifyDiffCount(img, textTop, textBottom, 0, 8, stripBg); ink != 0 {
		t.Errorf("%d ink pixels inside the strip's left inset x=[0,8); the left cluster is not anchored at the inset", ink)
	}
	if ink := statusBarVerifyDiffCount(img, textTop, textBottom, 8, 28, stripBg); ink == 0 {
		t.Errorf("no ink in x=[8,28); the name column does not begin at the left inset")
	}

	// Right anchor: the free-space text ends flush against the right inset —
	// ink just inside it, none within it.
	if ink := statusBarVerifyDiffCount(img, textTop, textBottom, x1-8, x1, stripBg); ink != 0 {
		t.Errorf("%d ink pixels inside the strip's right inset x=[%d,%d); the free region overruns its anchor", ink, x1-8, x1)
	}
	if ink := statusBarVerifyDiffCount(img, textTop, textBottom, x1-24, x1-8, stripBg); ink == 0 {
		t.Errorf("no ink in x=[%d,%d); the free-space text does not end at the right inset", x1-24, x1-8)
	}

	// The cursor row: the mock paints settingsStatusBarPreviewCursor with the
	// draft palette's selected background, in the column and row the grid's own
	// arithmetic puts it. Without this the strip could describe a row nothing
	// visibly highlights.
	selBg := color.RGBA{R: palette.SelectedBg.R, G: palette.SelectedBg.G, B: palette.SelectedBg.B, A: palette.SelectedBg.A}
	colW := statusBarPreviewVerifyWidth / settingsStatusBarPreviewGridColumns
	cursorCol := settingsStatusBarPreviewCursor / settingsStatusBarPreviewGridRows
	cursorRow := settingsStatusBarPreviewCursor % settingsStatusBarPreviewGridRows
	const gridTop, rowH = contentTop + 6, 18
	cellY := gridTop + cursorRow*rowH + rowH/2
	cellX0, cellX1 := cursorCol*colW, (cursorCol+1)*colW
	sel := 0
	for x := cellX0; x < cellX1; x++ {
		if c := img.RGBAAt(x, cellY); c == selBg {
			sel++
		}
	}
	if sel == 0 {
		t.Errorf("no selected-background pixels across the cursor cell (col %d, row %d) at y=%d; the mock does not highlight the row its strip describes",
			cursorCol, cursorRow, cellY)
	}

	// Master switch off: the strip's band goes back to bare pane background — no
	// border row, no text — while the pane above it is untouched. The scrollbar
	// slides down into the bottom of that band once the strip is gone, so the
	// rows it occupies are excluded.
	st.statusBarEnabledBool.Value = false
	img = render()
	writeStatusBarVerifyPNG(t, outDir, "settings-preview-disabled", img)
	const scrollbarH = 8
	for y := stripTop; y < frameBottom-scrollbarH; y++ {
		if got, ok := statusBarVerifyRowColor(img, y, x0, x1); !ok || got != paneBg {
			t.Errorf("row %d of the disabled preview is %v (uniform=%t), want the uniform pane background %v; a strip is painted with the bar switched off", y, got, ok, paneBg)
			break
		}
	}
	// ...and the pane keeps its rows: the switch drops the bar, not the mock.
	if sel := statusBarVerifyDiffCount(img, gridTop, gridTop+settingsStatusBarPreviewGridRows*rowH, 0, colW, paneBg); sel == 0 {
		t.Error("the disabled preview's first grid column is bare pane background; the master switch emptied the whole mock instead of dropping its status bar")
	}
}
