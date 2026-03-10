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

	theme := (&UI{fmCfg: cfg}).fileViewerTheme()

	if got := fm.FormatHexColor(theme.PanelBg); got != "#112233" {
		t.Fatalf("PanelBg=%q want %q", got, "#112233")
	}
	if got := fm.FormatHexColor(theme.Text); got != "#F1E2D3" {
		t.Fatalf("Text=%q want %q", got, "#F1E2D3")
	}
}
