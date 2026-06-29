// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"testing"

	"gioui.org/font"
	"gioui.org/unit"
	"gioui.org/widget/material"
	resources "hexone"
	"hexone/fm"
)

func TestScaleModalFontSizeTracksInterfaceFont(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	base := ui.scaleModalFontSize(10)
	want := unit.Sp(float32(scaleInterfaceConfigFontSize(cfg, 10)) * modalFontScale)
	if base != want {
		t.Fatalf("base modal size = %v, want %v", base, want)
	}

	cfg.Interface.FontSizeSp = 20
	larger := ui.scaleModalFontSize(10)
	if larger <= base {
		t.Fatalf("larger modal size = %v, want > %v", larger, base)
	}
}

func TestSyncThemeRuntimeAppliesPaneFont(t *testing.T) {
	ui := &UI{
		typeface: font.Typeface(resources.BundledFontFamilyJetBrainsMonoNerdFontMono),
		textSize: unit.Sp(18),
	}
	th := material.NewTheme()

	ui.syncThemeRuntime(th)

	if th.Face != ui.mainTypeface() {
		t.Fatalf("theme face = %q, want %q", th.Face, ui.mainTypeface())
	}
	if th.TextSize != ui.mainTextSize() {
		t.Fatalf("theme text size = %v, want %v", th.TextSize, ui.mainTextSize())
	}
}

func TestDialogActionSegmentMetricsUseInterfaceFont(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	th := material.NewTheme()
	gtx := testLabelLayoutContext(image.Pt(320, 80))

	ui.syncThemeRuntime(th)
	baseW, baseH := ui.dialogActionSegmentMetricsPx(th, gtx, "Deleting...")

	ui.textSize = unit.Sp(20)
	ui.syncThemeRuntime(th)
	paneLargeW, paneLargeH := ui.dialogActionSegmentMetricsPx(th, gtx, "Deleting...")
	if paneLargeW != baseW || paneLargeH != baseH {
		t.Fatalf("pane font changed dialog metrics from %dx%d to %dx%d", baseW, baseH, paneLargeW, paneLargeH)
	}

	cfg.Interface.FontSizeSp = 20
	largeW, largeH := ui.dialogActionSegmentMetricsPx(th, gtx, "Deleting...")

	if largeW <= baseW {
		t.Fatalf("large width = %d, want > %d", largeW, baseW)
	}
	if largeH <= baseH {
		t.Fatalf("large height = %d, want > %d", largeH, baseH)
	}
}
