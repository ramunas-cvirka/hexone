// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"testing"

	"gioui.org/unit"
	"gioui.org/widget/material"
	resources "hexone"
	"hexone/fm"
)

func TestViewerLineContentRectSupportsBundledNerdFonts(t *testing.T) {
	th := material.NewTheme()
	gtx := testLabelLayoutContext(image.Pt(320, 64))

	for _, family := range resources.BundledFontFamilies() {
		ui := NewUI(fm.DefaultConfig())
		ui.fmCfg.Viewer.Typeface = family.Name
		rowH := measureTypefaceLineHeight(ui, th, gtx, ui.viewerTypeface())
		rect := viewerLineContentRect(ui, th, gtx, ui.viewerTypeface(), ui.viewerTextSize(), rowH, 8, 48)

		if rect.Empty() {
			t.Fatalf("%s rect should not be empty", family.Name)
		}
		if rect.Max.Y > rowH {
			t.Fatalf("%s rect maxY=%d want <= %d", family.Name, rect.Max.Y, rowH)
		}
	}
}

func TestViewerLineContentRectFallsBackToFullRowWithoutShaper(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	gtx := testLabelLayoutContext(image.Pt(240, 48))

	rect := viewerLineContentRect(ui, nil, gtx, ui.viewerTypeface(), unit.Sp(13), 18, 4, 40)

	if got := rect; got != image.Rect(4, 0, 40, 18) {
		t.Fatalf("rect=%v want %v", got, image.Rect(4, 0, 40, 18))
	}
}

func TestViewerLineSelectionRectUsesFullRow(t *testing.T) {
	rect := viewerLineSelectionRect(20, 6, 44)

	if got := rect; got != image.Rect(6, 0, 44, 20) {
		t.Fatalf("rect=%v want %v", got, image.Rect(6, 0, 44, 20))
	}
}
