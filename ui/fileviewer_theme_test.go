// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"testing"
)

func TestFileViewerThemeUsesExplicitOverrides(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Colors.FilePaneBackground = "#101820"
	cfg.Colors.FilePaneText = "#C8D0D8"
	cfg.Viewer.Background = "#112233"
	cfg.Viewer.Text = "#F1E2D3"
	cfg.Viewer.Selection = "#3355CC"

	theme := (&UI{fmCfg: cfg}).fileViewerTheme()

	if got := fm.FormatHexColor(theme.PanelBg); got != "#112233" {
		t.Fatalf("PanelBg=%q want %q", got, "#112233")
	}
	if got := fm.FormatHexColor(theme.Text); got != "#F1E2D3" {
		t.Fatalf("Text=%q want %q", got, "#F1E2D3")
	}
}

func TestFileViewerSelectionContrastIsStrongerThanBefore(t *testing.T) {
	cfg := fm.DefaultConfig()
	theme := (&UI{fmCfg: cfg}).fileViewerTheme()

	if got := contrastScore(theme.PanelBg, theme.Selection); got < 1.45 {
		t.Fatalf("selection contrast=%0.2f want >= 1.45", got)
	}
	if got := contrastScore(theme.PanelBg, theme.StrongSelection); got < 1.7 {
		t.Fatalf("strong selection contrast=%0.2f want >= 1.70", got)
	}
	if contrastScore(theme.PanelBg, theme.StrongSelection) < contrastScore(theme.PanelBg, theme.Selection) {
		t.Fatal("strong selection should be at least as contrasty as normal selection")
	}
}
