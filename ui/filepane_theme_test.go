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
