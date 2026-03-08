package ui

import (
	"hexone/fm"
	"image/color"
	"math"
)

type filePanePalette struct {
	PaneBg      color.NRGBA
	PaneFg      color.NRGBA
	HoverBg     color.NRGBA
	HoverFg     color.NRGBA
	SelectedBg  color.NRGBA
	SelectedFg  color.NRGBA
	MarkedBg    color.NRGBA
	MarkedFg    color.NRGBA
	MarkedSelBg color.NRGBA
	MarkedSelFg color.NRGBA
}

func filePanePaletteFromConfig(cfg *fm.Config) filePanePalette {
	bg := parseConfigColorHexFallback("", fm.DefaultFilePaneBackgroundHex)
	fg := parseConfigColorHexFallback("", fm.DefaultFilePaneTextHex)
	hover := parseConfigColorHexFallback("", fm.DefaultFilePaneHoverHex)
	hoverFg := parseConfigColorHexFallback("", fm.DefaultFilePaneHoverTextHex)
	selected := parseConfigColorHexFallback("", fm.DefaultFilePaneSelectionHex)
	selectedFg := parseConfigColorHexFallback("", fm.DefaultFilePaneSelectionTextHex)
	marked := parseConfigColorHexFallback("", fm.DefaultFilePaneSelectedFilesHex)
	markedFg := parseConfigColorHexFallback("", fm.DefaultFilePaneSelectedTextHex)
	markedSel := parseConfigColorHexFallback("", fm.DefaultFilePaneFocusedSelectedHex)
	markedSelFg := parseConfigColorHexFallback("", fm.DefaultFilePaneFocusedSelectedTextHex)
	if cfg != nil {
		bg = parseConfigColorHexFallback(cfg.Colors.FilePaneBackground, fm.DefaultFilePaneBackgroundHex)
		fg = parseConfigColorHexFallback(cfg.Colors.FilePaneText, fm.DefaultFilePaneTextHex)
		hover = parseConfigColorHexFallback(cfg.Colors.Hover, fm.DefaultFilePaneHoverHex)
		hoverFg = parseConfigColorHexFallback(cfg.Colors.HoverText, fm.DefaultFilePaneHoverTextHex)
		selected = parseConfigColorHexFallback(cfg.Colors.Selection, fm.DefaultFilePaneSelectionHex)
		selectedFg = parseConfigColorHexFallback(cfg.Colors.SelectionText, fm.DefaultFilePaneSelectionTextHex)
		marked = parseConfigColorHexFallback(cfg.Colors.SelectedFiles, fm.DefaultFilePaneSelectedFilesHex)
		markedFg = parseConfigColorHexFallback(cfg.Colors.SelectedFilesText, fm.DefaultFilePaneSelectedTextHex)
		markedSel = parseConfigColorHexFallback(cfg.Colors.FocusedSelected, fm.DefaultFilePaneFocusedSelectedHex)
		markedSelFg = parseConfigColorHexFallback(cfg.Colors.FocusedSelectedText, fm.DefaultFilePaneFocusedSelectedTextHex)
	}
	return filePanePalette{
		PaneBg:      bg,
		PaneFg:      fg,
		HoverBg:     hover,
		HoverFg:     hoverFg,
		SelectedBg:  selected,
		SelectedFg:  selectedFg,
		MarkedBg:    marked,
		MarkedFg:    markedFg,
		MarkedSelBg: markedSel,
		MarkedSelFg: markedSelFg,
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
