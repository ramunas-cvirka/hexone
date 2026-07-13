// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"image/color"
	"testing"
)

func TestFilePaneActiveBorderColorTracksPaneContrast(t *testing.T) {
	dark := filePaneActiveBorderColor(color.NRGBA{R: 18, G: 22, B: 30, A: 255})
	if dark != (color.NRGBA{R: 155, G: 157, B: 160, A: 89}) {
		t.Fatalf("dark pane accent=%v want %v", dark, color.NRGBA{R: 155, G: 157, B: 160, A: 89})
	}

	light := filePaneActiveBorderColor(color.NRGBA{R: 240, G: 244, B: 250, A: 255})
	if light != (color.NRGBA{R: 115, G: 117, B: 120, A: 89}) {
		t.Fatalf("light pane accent=%v want %v", light, color.NRGBA{R: 115, G: 117, B: 120, A: 89})
	}
}

func TestFilePaneInactiveShadeColorStaysSubtle(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.General.DimInactivePanes = true

	dark := filePaneInactiveShadeColor(cfg, color.NRGBA{R: 18, G: 22, B: 30, A: 255})
	if dark != (color.NRGBA{R: 50, G: 50, B: 50, A: 72}) {
		t.Fatalf("dark pane shade=%v want %v", dark, color.NRGBA{R: 50, G: 50, B: 50, A: 72})
	}

	light := filePaneInactiveShadeColor(cfg, color.NRGBA{R: 240, G: 244, B: 250, A: 255})
	if light != (color.NRGBA{R: 200, G: 200, B: 200, A: 54}) {
		t.Fatalf("light pane shade=%v want %v", light, color.NRGBA{R: 200, G: 200, B: 200, A: 54})
	}

	cfg.General.DimInactivePanes = false
	disabled := filePaneInactiveShadeColor(cfg, color.NRGBA{R: 18, G: 22, B: 30, A: 255})
	if disabled != (color.NRGBA{}) {
		t.Fatalf("disabled shade=%v want zero", disabled)
	}
}

func TestFilePaneScrollbarDefaultsContrastWithPaneBackground(t *testing.T) {
	dark := filePanePaletteFromConfig(&fm.Config{
		Colors: fm.ColorsConfig{
			FilePaneBackground: fm.DefaultFilePaneBackgroundHex,
			FilePaneText:       fm.DefaultFilePaneTextHex,
			HoverText:          fm.DefaultFilePaneHoverTextHex,
			SelectionText:      fm.DefaultFilePaneSelectionTextHex,
			CurrentDirText:     fm.DefaultCurrentDirTextHex,
		},
	})
	if contrastScore(dark.PaneBg, dark.ScrollThumb) < 4 {
		t.Fatalf("dark scrollbar contrast=%0.2f want >= 4", contrastScore(dark.PaneBg, dark.ScrollThumb))
	}

	light := filePanePaletteFromConfig(&fm.Config{
		Colors: fm.ColorsConfig{
			FilePaneBackground: "#F3F4F6",
			FilePaneText:       "#1F2937",
			HoverText:          "#111827",
			SelectionText:      "#0B1220",
			CurrentDirText:     "#111827",
		},
	})
	if contrastScore(light.PaneBg, light.ScrollThumb) < 4 {
		t.Fatalf("light scrollbar contrast=%0.2f want >= 4", contrastScore(light.PaneBg, light.ScrollThumb))
	}
}

func TestFilePaneScrollbarUsesConfigOverrides(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Colors.ScrollbarThumb = "#ABCDEF"
	cfg.Colors.ScrollbarTrack = "#123456"

	palette := filePanePaletteFromConfig(cfg)

	if got := fm.FormatHexColor(palette.ScrollThumb); got != "#ABCDEF" {
		t.Fatalf("scrollbar thumb=%q want override", got)
	}
	if got := fm.FormatHexColor(palette.ScrollTrack); got != "#123456" {
		t.Fatalf("scrollbar track=%q want override", got)
	}
}

func TestFilePaneTransparentRowTextDisablesTableOverride(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Colors.HoverText = fm.TransparentColor
	cfg.Colors.SelectionText = fm.TransparentColor
	cfg.Colors.SelectedFilesText = fm.TransparentColor
	cfg.Colors.FocusedSelectedText = fm.TransparentColor

	palette := filePanePaletteFromConfig(cfg)
	if palette.HoverFg != (color.NRGBA{}) {
		t.Fatalf("HoverFg=%v want transparent", palette.HoverFg)
	}
	if palette.SelectedFg != (color.NRGBA{}) {
		t.Fatalf("SelectedFg=%v want transparent", palette.SelectedFg)
	}
	if palette.MarkedFg != (color.NRGBA{}) {
		t.Fatalf("MarkedFg=%v want transparent", palette.MarkedFg)
	}
	if palette.MarkedSelFg != (color.NRGBA{}) {
		t.Fatalf("MarkedSelFg=%v want transparent", palette.MarkedSelFg)
	}

	pane := newFilePaneState("", cfg)
	if pane.table.HoverFg != nil {
		t.Fatalf("HoverFg table override=%v want nil", *pane.table.HoverFg)
	}
	if pane.table.SelectedFg != nil {
		t.Fatalf("SelectedFg table override=%v want nil", *pane.table.SelectedFg)
	}
	if pane.table.MarkedFg != nil {
		t.Fatalf("MarkedFg table override=%v want nil", *pane.table.MarkedFg)
	}
	if pane.table.MarkedSelFg != nil {
		t.Fatalf("MarkedSelFg table override=%v want nil", *pane.table.MarkedSelFg)
	}
}
