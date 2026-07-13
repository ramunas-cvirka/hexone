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

func TestFileViewerThemeUsesHexSectionTextOverrides(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Viewer.HexSelection = "#294C80"
	cfg.Viewer.HexOffsetText = "#A1B2C3"
	cfg.Viewer.HexBytesText = "#D4E5F6"
	cfg.Viewer.HexASCIIText = "#789ABC"

	theme := (&UI{fmCfg: cfg}).fileViewerTheme()

	if got := contrastScore(theme.PanelBg, theme.HexSelection); got < 1.45 {
		t.Fatalf("HexSelection contrast=%0.2f want >= 1.45", got)
	}
	if theme.HexSelection == theme.Selection {
		t.Fatal("explicit hex selection should be independent from file selection")
	}
	if got := fm.FormatHexColor(theme.OffsetText); got != "#A1B2C3" {
		t.Fatalf("OffsetText=%q want #A1B2C3", got)
	}
	if got := fm.FormatHexColor(theme.HexText); got != "#D4E5F6" {
		t.Fatalf("HexText=%q want #D4E5F6", got)
	}
	if got := fm.FormatHexColor(theme.ASCIIText); got != "#789ABC" {
		t.Fatalf("ASCIIText=%q want #789ABC", got)
	}
	if got := fm.FormatHexColor(theme.Text); got != fm.DefaultViewerTextHex {
		t.Fatalf("general viewer Text=%q changed with hex override", got)
	}
}

func TestFileViewerHexSeparatorContrastsWithDarkAndLightThemes(t *testing.T) {
	for _, background := range []string{"#101820", "#F4F5F7"} {
		cfg := fm.DefaultConfig()
		cfg.Viewer.Background = background
		theme := (&UI{fmCfg: cfg}).fileViewerTheme()
		if got := contrastScore(theme.PanelBg, theme.Separator); got < 2.0 {
			t.Fatalf("background %s separator contrast=%0.2f want >= 2.0", background, got)
		}
		if theme.Separator.A < 128 {
			t.Fatalf("background %s separator alpha=%d is too faint", background, theme.Separator.A)
		}
	}
}

func TestFileViewerThemeUsesScrollbarOverrides(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Viewer.Background = "#112233"
	cfg.Viewer.Text = "#F1E2D3"
	cfg.Colors.ScrollbarThumb = "#AA3300"
	cfg.Colors.ScrollbarTrack = "#001122"

	theme := (&UI{fmCfg: cfg}).fileViewerTheme()

	if got := fm.FormatHexColor(theme.ScrollThumb); got != "#AA3300" {
		t.Fatalf("ScrollThumb=%q want %q", got, "#AA3300")
	}
	if got := fm.FormatHexColor(theme.ScrollTrack); got != "#001122" {
		t.Fatalf("ScrollTrack=%q want %q", got, "#001122")
	}
}

func TestPopupThemeUsesPopupHoverOverrides(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Colors.PopupHover = "#123456"
	cfg.Colors.PopupHoverText = "#FEDCBA"

	theme := (&UI{fmCfg: cfg}).filePanePopupTheme()

	if got := fm.FormatHexColor(theme.HoverBg); got != "#123456" {
		t.Fatalf("HoverBg=%q want %q", got, "#123456")
	}
	if got := fm.FormatHexColor(theme.HoverText); got != "#FEDCBA" {
		t.Fatalf("HoverText=%q want %q", got, "#FEDCBA")
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
