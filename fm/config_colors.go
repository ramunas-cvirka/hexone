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
	DefaultFilePaneTextHex                = "#D2D2D2"
	DefaultFilePaneHoverHex               = "#2D2D2D"
	DefaultFilePaneHoverTextHex           = "#E8E8E8"
	DefaultFilePaneSelectionHex           = "#3C3C50"
	DefaultFilePaneSelectionTextHex       = "#F4F4F4"
	DefaultFilePaneSelectedFilesHex       = "#4A4A4A"
	DefaultFilePaneSelectedTextHex        = "#F4F4F4"
	DefaultFilePaneFocusedSelectedHex     = "#58586C"
	DefaultFilePaneFocusedSelectedTextHex = "#F4F4F4"
	DefaultCurrentDirBackgroundHex        = "#363636"
	DefaultCurrentDirTextHex              = "#F0F0F0"
)

type ColorsConfig struct {
	FilePaneBackground  string               `yaml:"file_pane_background"`
	FilePaneText        string               `yaml:"file_pane_text"`
	Hover               string               `yaml:"hover"`
	HoverText           string               `yaml:"hover_text"`
	Selection           string               `yaml:"selection"`
	SelectionText       string               `yaml:"selection_text"`
	SelectedFiles       string               `yaml:"selected_files"`
	SelectedFilesText   string               `yaml:"selected_files_text"`
	FocusedSelected     string               `yaml:"focused_selected"`
	FocusedSelectedText string               `yaml:"focused_selected_text"`
	CurrentDirBg        string               `yaml:"current_dir_background"`
	CurrentDirText      string               `yaml:"current_dir_text"`
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

func NormalizeOptionalHexColor(raw string) string {
	if c, ok := ParseHexColor(raw); ok {
		return FormatHexColor(c)
	}
	return ""
}
