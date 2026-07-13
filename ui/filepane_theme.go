// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"image/color"
	"math"
)

type filePanePalette struct {
	PaneBg       color.NRGBA
	PaneFg       color.NRGBA
	HoverBg      color.NRGBA
	HoverFg      color.NRGBA
	PopupHoverBg color.NRGBA
	PopupHoverFg color.NRGBA
	SelectedBg   color.NRGBA
	SelectedFg   color.NRGBA
	MarkedBg     color.NRGBA
	MarkedFg     color.NRGBA
	MarkedSelBg  color.NRGBA
	MarkedSelFg  color.NRGBA
	CurrentDirBg color.NRGBA
	CurrentDirFg color.NRGBA
	ScrollTrack  color.NRGBA
	ScrollTrackH color.NRGBA
	ScrollThumb  color.NRGBA
	ScrollThumbH color.NRGBA
	ScrollThumbD color.NRGBA
}

func filePanePaletteFromConfig(cfg *fm.Config) filePanePalette {
	bg := parseConfigColorHexFallback("", fm.DefaultFilePaneBackgroundHex)
	fg := parseConfigColorHexFallback("", fm.DefaultFilePaneTextHex)
	hover := parseConfigColorHexFallback("", fm.DefaultFilePaneHoverHex)
	hoverFg := parseConfigColorHexFallback("", fm.DefaultFilePaneHoverTextHex)
	popupHover := parseConfigColorHexFallback("", fm.DefaultPopupHoverHex)
	popupHoverFg := parseConfigColorHexFallback("", fm.DefaultPopupHoverTextHex)
	selected := parseConfigColorHexFallback("", fm.DefaultFilePaneSelectionHex)
	selectedFg := parseConfigColorHexFallback("", fm.DefaultFilePaneSelectionTextHex)
	marked := parseConfigColorHexFallback("", fm.DefaultFilePaneSelectedFilesHex)
	markedFg := parseConfigColorHexFallback("", fm.DefaultFilePaneSelectedTextHex)
	markedSel := parseConfigColorHexFallback("", fm.DefaultFilePaneFocusedSelectedHex)
	markedSelFg := parseConfigColorHexFallback("", fm.DefaultFilePaneFocusedSelectedTextHex)
	currentDirBg := parseConfigColorHexFallback("", fm.DefaultCurrentDirBackgroundHex)
	currentDirFg := parseConfigColorHexFallback("", fm.DefaultCurrentDirTextHex)
	scrollThumbOverride := ""
	scrollTrackOverride := ""
	if cfg != nil {
		bg = parseConfigColorHexFallback(cfg.Colors.FilePaneBackground, fm.DefaultFilePaneBackgroundHex)
		fg = parseConfigColorHexFallback(cfg.Colors.FilePaneText, fm.DefaultFilePaneTextHex)
		hover = parseConfigColorHexFallback(cfg.Colors.Hover, fm.DefaultFilePaneHoverHex)
		hoverFg = parseConfigColorHexTransparentFallback(cfg.Colors.HoverText, fm.DefaultFilePaneHoverTextHex)
		popupHover = parseConfigColorHexFallback(cfg.Colors.PopupHover, fm.DefaultPopupHoverHex)
		popupHoverFg = parseConfigColorHexFallback(cfg.Colors.PopupHoverText, fm.DefaultPopupHoverTextHex)
		selected = parseConfigColorHexFallback(cfg.Colors.Selection, fm.DefaultFilePaneSelectionHex)
		selectedFg = parseConfigColorHexTransparentFallback(cfg.Colors.SelectionText, fm.DefaultFilePaneSelectionTextHex)
		marked = parseConfigColorHexFallback(cfg.Colors.SelectedFiles, fm.DefaultFilePaneSelectedFilesHex)
		markedFg = parseConfigColorHexTransparentFallback(cfg.Colors.SelectedFilesText, fm.DefaultFilePaneSelectedTextHex)
		markedSel = parseConfigColorHexFallback(cfg.Colors.FocusedSelected, fm.DefaultFilePaneFocusedSelectedHex)
		markedSelFg = parseConfigColorHexTransparentFallback(cfg.Colors.FocusedSelectedText, fm.DefaultFilePaneFocusedSelectedTextHex)
		currentDirBg = parseConfigColorHexFallback(cfg.Colors.CurrentDirBg, fm.DefaultCurrentDirBackgroundHex)
		currentDirFg = parseConfigColorHexFallback(cfg.Colors.CurrentDirText, fm.DefaultCurrentDirTextHex)
		scrollThumbOverride = cfg.Colors.ScrollbarThumb
		scrollTrackOverride = cfg.Colors.ScrollbarTrack
	}
	scrollTrack, scrollTrackHover, scrollThumb, scrollThumbHover, scrollThumbDrag := filePaneScrollbarColors(bg, fg, hoverFg, selectedFg, currentDirFg, scrollThumbOverride, scrollTrackOverride)
	return filePanePalette{
		PaneBg:       bg,
		PaneFg:       fg,
		HoverBg:      hover,
		HoverFg:      hoverFg,
		PopupHoverBg: popupHover,
		PopupHoverFg: popupHoverFg,
		SelectedBg:   selected,
		SelectedFg:   selectedFg,
		MarkedBg:     marked,
		MarkedFg:     markedFg,
		MarkedSelBg:  markedSel,
		MarkedSelFg:  markedSelFg,
		CurrentDirBg: currentDirBg,
		CurrentDirFg: currentDirFg,
		ScrollTrack:  scrollTrack,
		ScrollTrackH: scrollTrackHover,
		ScrollThumb:  scrollThumb,
		ScrollThumbH: scrollThumbHover,
		ScrollThumbD: scrollThumbDrag,
	}
}

func parseConfigColorHexFallback(raw, fallback string) color.NRGBA {
	if c, ok := fm.ParseHexColor(raw); ok {
		return c
	}
	if c, ok := fm.ParseHexColor(fallback); ok {
		return c
	}
	return color.NRGBA{R: 18, G: 22, B: 30, A: 255}
}

func parseConfigColorHexTransparentFallback(raw, fallback string) color.NRGBA {
	if fm.IsTransparentColor(raw) {
		return color.NRGBA{}
	}
	if c, ok := fm.ParseHexColor(raw); ok {
		return c
	}
	if fm.IsTransparentColor(fallback) {
		return color.NRGBA{}
	}
	if c, ok := fm.ParseHexColor(fallback); ok {
		return c
	}
	return color.NRGBA{R: 18, G: 22, B: 30, A: 255}
}

func contrastTextColor(bg color.NRGBA) color.NRGBA {
	luma := 0.2126*float64(bg.R) + 0.7152*float64(bg.G) + 0.0722*float64(bg.B)
	if luma >= 150 {
		return color.NRGBA{R: 26, G: 32, B: 42, A: 255}
	}
	return color.NRGBA{R: 244, G: 248, B: 255, A: 255}
}

func bestContrastColor(bg color.NRGBA, choices ...color.NRGBA) color.NRGBA {
	best := contrastTextColor(bg)
	bestScore := contrastScore(bg, best)
	for _, c := range choices {
		if c.A == 0 {
			continue
		}
		if score := contrastScore(bg, c); score > bestScore {
			best = c
			bestScore = score
		}
	}
	return best
}

func filePaneActiveBorderColor(bg color.NRGBA) color.NRGBA {
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.NRGBA{A: 255}
	accent := mixNRGBA(bg, white, 0.58)
	if relativeLuminance(bg) >= 0.42 {
		accent = mixNRGBA(bg, black, 0.52)
		if contrastScore(bg, accent) < 2.1 {
			accent = mixNRGBA(bg, black, 0.68)
		}
	} else if contrastScore(bg, accent) < 2.1 {
		accent = mixNRGBA(bg, white, 0.72)
	}
	accent.A = 89
	return accent
}

func filePaneInactiveShadeColor(cfg *fm.Config, bg color.NRGBA) color.NRGBA {
	if cfg != nil && !cfg.General.DimInactivePanes {
		return color.NRGBA{}
	}
	lum := relativeLuminance(bg)
	tone := int(math.Round(0.2126*float64(bg.R) + 0.7152*float64(bg.G) + 0.0722*float64(bg.B)))
	grayTone := clampU8(int(math.Round(float64(tone) * 0.78)))
	alpha := uint8(58)
	switch {
	case lum < 0.18:
		grayTone = clampU8(tone + 28)
		alpha = 72
	case lum < 0.40:
		grayTone = clampU8(tone + 18)
		alpha = 66
	case lum > 0.75:
		grayTone = clampU8(int(math.Round(float64(tone) * 0.82)))
		alpha = 54
	}
	return color.NRGBA{R: grayTone, G: grayTone, B: grayTone, A: alpha}
}

func filePaneScrollbarColors(bg, fg, hoverFg, selectedFg, currentDirFg color.NRGBA, thumbOverride, trackOverride string) (track, trackHover, thumb, thumbHover, thumbDrag color.NRGBA) {
	accent := bestContrastColor(bg, fg, hoverFg, selectedFg, currentDirFg)
	thumb = mixNRGBA(accent, bg, 0.16)
	thumb.A = 214
	if c, ok := fm.ParseHexColor(thumbOverride); ok {
		thumb = c
	}

	track = mixNRGBA(bg, thumb, 0.20)
	track.A = 58
	if c, ok := fm.ParseHexColor(trackOverride); ok {
		track = c
	}

	thumbHover = mixNRGBA(thumb, accent, 0.16)
	thumbHover.A = 234
	thumbDrag = mixNRGBA(thumb, accent, 0.30)
	thumbDrag.A = 248
	trackHover = mixNRGBA(track, thumb, 0.20)
	if trackHover.A < 82 {
		trackHover.A = 82
	}
	return track, trackHover, thumb, thumbHover, thumbDrag
}

func contrastScore(bg, fg color.NRGBA) float64 {
	bgLum := relativeLuminance(bg)
	fgLum := relativeLuminance(fg)
	if bgLum < fgLum {
		bgLum, fgLum = fgLum, bgLum
	}
	return (bgLum + 0.05) / (fgLum + 0.05)
}

func relativeLuminance(c color.NRGBA) float64 {
	channel := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}
