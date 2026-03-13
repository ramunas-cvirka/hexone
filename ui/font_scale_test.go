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
)

func TestScaleModalThemeFontSizeTracksPaneFont(t *testing.T) {
	th := material.NewTheme()
	th.TextSize = unit.Sp(14)

	base := scaleModalThemeFontSize(th, 10)
	want := unit.Sp(float32(scaleThemeFontSize(th, 10)) * modalFontScale)
	if base != want {
		t.Fatalf("base modal size = %v, want %v", base, want)
	}

	th.TextSize = unit.Sp(20)
	larger := scaleModalThemeFontSize(th, 10)
	if larger <= base {
		t.Fatalf("larger modal size = %v, want > %v", larger, base)
	}
}

func TestSyncThemeRuntimeAppliesPaneFont(t *testing.T) {
	ui := &UI{
		typeface: font.Typeface(resources.BundledFontFamilyConsolas),
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

func TestDialogActionSegmentMetricsGrowWithPaneFont(t *testing.T) {
	ui := &UI{
		typeface: font.Typeface(resources.BundledFontFamilyFiraCode),
		textSize: unit.Sp(14),
	}
	th := material.NewTheme()
	gtx := testLabelLayoutContext(image.Pt(320, 80))

	ui.syncThemeRuntime(th)
	baseW, baseH := ui.dialogActionSegmentMetricsPx(th, gtx, "Deleting...")

	ui.textSize = unit.Sp(20)
	ui.syncThemeRuntime(th)
	largeW, largeH := ui.dialogActionSegmentMetricsPx(th, gtx, "Deleting...")

	if largeW <= baseW {
		t.Fatalf("large width = %d, want > %d", largeW, baseW)
	}
	if largeH <= baseH {
		t.Fatalf("large height = %d, want > %d", largeH, baseH)
	}
}
