// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package fm

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

const (
	DefaultFilePaneBackgroundHex          = "#202020"
	DefaultFilePaneTextHex                = "#BABABA"
	DefaultFilePaneHoverHex               = "#2A2A2A"
	DefaultFilePaneHoverTextHex           = "#E8E8E8"
	DefaultPopupHoverHex                  = "#485F96"
	DefaultPopupHoverTextHex              = "#F6F9FF"
	DefaultFilePaneSelectionHex           = "#3A3A3A"
	DefaultFilePaneSelectionTextHex       = "#F4F4F4"
	DefaultFilePaneSelectedFilesHex       = "#002CF0"
	DefaultFilePaneSelectedTextHex        = "#FBC4DF"
	DefaultFilePaneFocusedSelectedHex     = "#0000F0"
	DefaultFilePaneFocusedSelectedTextHex = "#F66EB2"
	DefaultCurrentDirBackgroundHex        = "#363636"
	DefaultCurrentDirTextHex              = "#F0F0F0"
	DefaultViewerBackgroundHex            = "#202020"
	DefaultViewerTextHex                  = "#D2D2D2"
	DefaultViewerSelectionHex             = "#3C3C50"
	TransparentColor                      = "transparent"
)

type ColorsConfig struct {
	FilePaneBackground  string               `yaml:"file_pane_background"`
	FilePaneText        string               `yaml:"file_pane_text"`
	Hover               string               `yaml:"hover"`
	HoverText           string               `yaml:"hover_text"`
	PopupHover          string               `yaml:"popup_hover"`
	PopupHoverText      string               `yaml:"popup_hover_text"`
	Selection           string               `yaml:"selection"`
	SelectionText       string               `yaml:"selection_text"`
	SelectedFiles       string               `yaml:"selected_files"`
	SelectedFilesText   string               `yaml:"selected_files_text"`
	FocusedSelected     string               `yaml:"focused_selected"`
	FocusedSelectedText string               `yaml:"focused_selected_text"`
	CurrentDirBg        string               `yaml:"current_dir_background"`
	CurrentDirText      string               `yaml:"current_dir_text"`
	ScrollbarThumb      string               `yaml:"scrollbar_thumb,omitempty"`
	ScrollbarTrack      string               `yaml:"scrollbar_track,omitempty"`
	Filenames           FilenameColorsConfig `yaml:"filenames,omitempty"`
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

func IsTransparentColor(raw string) bool {
	return strings.EqualFold(strings.TrimSpace(raw), TransparentColor)
}

func NormalizeHexOrTransparentColor(raw, fallback string) string {
	if IsTransparentColor(raw) {
		return TransparentColor
	}
	if c, ok := ParseHexColor(raw); ok {
		return FormatHexColor(c)
	}
	if IsTransparentColor(fallback) {
		return TransparentColor
	}
	if c, ok := ParseHexColor(fallback); ok {
		return FormatHexColor(c)
	}
	return DefaultFilePaneBackgroundHex
}

func NormalizeOptionalHexColor(raw string) string {
	if c, ok := ParseHexColor(raw); ok {
		return FormatHexColor(c)
	}
	return ""
}
