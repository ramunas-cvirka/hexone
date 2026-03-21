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

func TestViewerLineContentRectAddsConsolasOpticalNudge(t *testing.T) {
	th := material.NewTheme()
	gtx := testLabelLayoutContext(image.Pt(320, 64))

	firaUI := NewUI(fm.DefaultConfig())
	firaUI.fmCfg.Viewer.Typeface = resources.BundledFontFamilyFiraCode
	consolasUI := NewUI(fm.DefaultConfig())
	consolasUI.fmCfg.Viewer.Typeface = resources.BundledFontFamilyConsolas

	rowH := measureTypefaceLineHeight(firaUI, th, gtx, firaUI.viewerTypeface())
	if other := measureTypefaceLineHeight(consolasUI, th, gtx, consolasUI.viewerTypeface()); other > rowH {
		rowH = other
	}

	firaRect := viewerLineContentRect(firaUI, th, gtx, firaUI.viewerTypeface(), firaUI.viewerTextSize(), rowH, 8, 48)
	consolasRect := viewerLineContentRect(consolasUI, th, gtx, consolasUI.viewerTypeface(), consolasUI.viewerTextSize(), rowH, 8, 48)

	if firaRect.Empty() {
		t.Fatal("fira rect should not be empty")
	}
	if consolasRect.Empty() {
		t.Fatal("consolas rect should not be empty")
	}
	if consolasRect.Min.Y <= firaRect.Min.Y {
		t.Fatalf("consolas rect top=%d want > fira top=%d", consolasRect.Min.Y, firaRect.Min.Y)
	}
	if consolasRect.Max.Y > rowH {
		t.Fatalf("consolas rect maxY=%d want <= %d", consolasRect.Max.Y, rowH)
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
