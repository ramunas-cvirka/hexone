package fm

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

const (
	DefaultFilePaneBackgroundHex          = "#161E28"
	DefaultFilePaneTextHex                = "#D2D2D2"
	DefaultFilePaneHoverHex               = "#1F2E48"
	DefaultFilePaneHoverTextHex           = "#F4F8FF"
	DefaultFilePaneSelectionHex           = "#456FCC"
	DefaultFilePaneSelectionTextHex       = "#F4F8FF"
	DefaultFilePaneSelectedFilesHex       = "#2F8B63"
	DefaultFilePaneSelectedTextHex        = "#F4F8FF"
	DefaultFilePaneFocusedSelectedHex     = "#3A7C99"
	DefaultFilePaneFocusedSelectedTextHex = "#F4F8FF"
)

type ColorsConfig struct {
	FilePaneBackground  string `yaml:"file_pane_background"`
	FilePaneText        string `yaml:"file_pane_text"`
	Hover               string `yaml:"hover"`
	HoverText           string `yaml:"hover_text"`
	Selection           string `yaml:"selection"`
	SelectionText       string `yaml:"selection_text"`
	SelectedFiles       string `yaml:"selected_files"`
	SelectedFilesText   string `yaml:"selected_files_text"`
	FocusedSelected     string `yaml:"focused_selected"`
	FocusedSelectedText string `yaml:"focused_selected_text"`
}

func ParseHexColor(raw string) (color.NRGBA, bool) {
	txt := strings.TrimSpace(raw)
	txt = strings.TrimPrefix(txt, "#")
	if len(txt) != 6 {
		return color.NRGBA{}, false
	}
	v, err := strconv.ParseUint(txt, 16, 32)
	if err != nil {
		return color.NRGBA{}, false
	}
	return color.NRGBA{
		R: uint8(v >> 16),
		G: uint8(v >> 8),
		B: uint8(v),
		A: 0xFF,
	}, true
}

func FormatHexColor(c color.NRGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

func NormalizeHexColor(raw, fallback string) string {
	if c, ok := ParseHexColor(raw); ok {
		return FormatHexColor(c)
	}
	if c, ok := ParseHexColor(fallback); ok {
		return FormatHexColor(c)
	}
	return DefaultFilePaneBackgroundHex
}
