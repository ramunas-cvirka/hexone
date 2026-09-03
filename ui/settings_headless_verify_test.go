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
	// Kept in settingsPaneModeKeys order, so the sweep reads the way the tab
	// strip does.
	for _, mode := range []string{"full", "brief", "statusbar", "other"} {
		ui = NewUI(fm.DefaultConfig())
		ui.openSettingsModal()
		ui.settingsModal.activeTab = "general"
		ui.settingsModal.paneSettingsMode = mode
		label := "file-panes-" + mode
		switch mode {
		case "other":
			ui.settingsModal.generalUseTrash.Value = true
		case "statusbar":
			// Every field ticked, which is what the preview's representative
			// width exists to keep showing: all six, none dropped.
			ui.settingsModal.statusBarEnabledBool.Value = true
			for i := range ui.settingsModal.statusBarFieldBools {
				ui.settingsModal.statusBarFieldBools[i].Value = true
			}
			// settings_statusbar_headless_verify_test.go already writes
			// settings-file-panes-statusbar.png; sharing the name would let
			// whichever test ran last win the file.
			label += "-sweep"
		default:
			ui.settingsModal.paneFullChars++
		}
		router = new(input.Router)
		writePNG(label, render(label))
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

// TestHeadlessSettingsStatusBarPreviewHidesTheStripWhenTheBarIsOff renders the
// status bar preview on its own and looks at the pixels, because that is the
// only place the gate is observable: switching the bar off leaves the pane mock
// standing and takes only its strip, and both branches report the same
// dimensions and leave no state behind for a plain go test to read.
//
// The bug this pins: with "Show pane status bar" unticked the preview still
// drew a full populated strip, contradicting filePaneStatusBarVisible, which
// renders no strip at all for that config. The preview was advertising
// something that could never ship.
func TestHeadlessSettingsStatusBarPreviewHidesTheStripWhenTheBarIsOff(t *testing.T) {
	// The preview frame is a fixed 154dp tall; the window only has to hold it.
	const width, height = 560, 160
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "general"
	st.paneSettingsMode = "statusbar"
	for i := range st.statusBarFieldBools {
		st.statusBarFieldBools[i].Value = true
	}

	router := new(input.Router)
	render := func(enabled bool) *image.RGBA {
		t.Helper()
		st.statusBarEnabledBool.Value = enabled
		win, err := headless.NewWindow(width, height)
		if err != nil {
			t.Fatalf("headless window: %v", err)
		}
		defer win.Release()
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)),
			Now:         time.Now(),
			Source:      router.Source(),
		}
		ui.layoutSettingsStatusBarPreview(th, gtx, st)
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

	paneBg := ui.settingsPaneDraftPalette(st).PaneBg
	// The strip's own band: the frame is 154dp tall and the strip is pinned to
	// its bottom edge, one line of the strip's text style plus 4dp of inset above
	// and below. Only that band is read — the mock keeps its sample rows either
	// way, and it is the strip alone the master switch controls. The bottom 8dp
	// are excluded because the brief-mode scrollbar slides down into them once
	// the strip is gone.
	probe := material.Body2(th, "0")
	probe.Font.Typeface = ui.mainTypeface()
	probe.TextSize = scaleThemeFontSize(th, 11)
	probe.MaxLines = 1
	probe.Truncator = ""
	var measureOps op.Ops
	mgtx := layout.Context{
		Ops:         &measureOps,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(width, height)},
		Now:         time.Now(),
	}
	stripH := measureLabelUnconstrained(mgtx, probe).Size.Y + 2*mgtx.Dp(unit.Dp(4))
	const frameBottom, scrollbarH = 154, 8
	bodyTop, bodyBottom := frameBottom-stripH, frameBottom-scrollbarH-1
	countOffBg := func(img *image.RGBA) int {
		n := 0
		for y := bodyTop; y <= bodyBottom; y++ {
			for x := 0; x < width; x++ {
				r, g, b, a := img.At(x, y).RGBA()
				if uint8(r>>8) != paneBg.R || uint8(g>>8) != paneBg.G || uint8(b>>8) != paneBg.B || uint8(a>>8) != paneBg.A {
					n++
				}
			}
		}
		return n
	}

	if got := countOffBg(render(false)); got != 0 {
		t.Fatalf("with the bar off, %d pixels of the strip's band are not the pane background %v; the preview is still drawing a status bar strip", got, paneBg)
	}
	if got := countOffBg(render(true)); got == 0 {
		t.Fatal("with the bar on, the strip's band is bare background; the strip did not draw, so the assertion above proves nothing")
	}
}
