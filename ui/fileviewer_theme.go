// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"image/color"
	"strings"
)

type fileViewerTheme struct {
	Backdrop           color.NRGBA
	HeaderBg           color.NRGBA
	HeaderText         color.NRGBA
	PanelBg            color.NRGBA
	PanelBorder        color.NRGBA
	Text               color.NRGBA
	HexText            color.NRGBA
	Muted              color.NRGBA
	Hint               color.NRGBA
	Error              color.NRGBA
	StatusAccent       color.NRGBA
	StatusWarn         color.NRGBA
	StatusError        color.NRGBA
	Divider            color.NRGBA
	Selection          color.NRGBA
	StrongSelection    color.NRGBA
	HexSelection       color.NRGBA
	HexStrongSelection color.NRGBA
	ScrollTrack        color.NRGBA
	ScrollTrackHover   color.NRGBA
	ScrollThumb        color.NRGBA
	ScrollThumbHover   color.NRGBA
	ScrollThumbDrag    color.NRGBA
	TooltipBg          color.NRGBA
	TooltipBorder      color.NRGBA
	TooltipText        color.NRGBA
	Separator          color.NRGBA
	OffsetText         color.NRGBA
	ASCIIText          color.NRGBA
	CommandBg          color.NRGBA
	CommandBgHover     color.NRGBA
	CommandBorder      color.NRGBA
	CommandBorderHover color.NRGBA
	CommandText        color.NRGBA
	CommandStaticText  color.NRGBA
	CommandHint        color.NRGBA
	HistoryBg          color.NRGBA
	HistoryBorder      color.NRGBA
	HistoryText        color.NRGBA
	HistoryMuted       color.NRGBA
	HistoryChipBg      color.NRGBA
	HistoryChipBgHover color.NRGBA
	HistoryChipBorder  color.NRGBA
	HistoryChipBorderH color.NRGBA
	HistoryChipText    color.NRGBA
}

func fileViewerThemeFromConfig(cfg *fm.Config) fileViewerTheme {
	palette := filePanePaletteFromConfig(nil)
	if cfg != nil {
		palette = filePanePaletteFromConfig(cfg)
	}
	ui := &UI{fmCfg: cfg}
	popup := ui.filePanePopupTheme()

	baseBg := mixNRGBA(palette.PaneBg, palette.CurrentDirBg, 0.12)
	baseBg.A = 0xFF
	if cfg != nil {
		if c, ok := fm.ParseHexColor(strings.TrimSpace(cfg.Viewer.Background)); ok {
			baseBg = c
		}
	}

	baseText := bestContrastColor(baseBg, palette.PaneFg, palette.CurrentDirFg, palette.HoverFg, palette.SelectedFg, popup.Text)
	if cfg != nil {
		if c, ok := fm.ParseHexColor(strings.TrimSpace(cfg.Viewer.Text)); ok {
			baseText = c
		}
	}
	baseText.A = 0xFF
	selectionBase := palette.SelectedBg
	if cfg != nil {
		if c, ok := fm.ParseHexColor(strings.TrimSpace(cfg.Viewer.Selection)); ok {
			selectionBase = c
		}
	}
	selectionBase.A = 0xFF

	backdrop := mixNRGBA(baseBg, color.NRGBA{A: 0xFF}, 0.46)
	backdrop.A = 252

	headerBg := mixNRGBA(baseBg, popup.Bg, 0.26)
	headerBg.A = 0xFF

	panelBorder := filePaneActiveBorderColor(baseBg)
	panelBorder.A = 0

	headerText := bestContrastColor(headerBg, baseText, popup.Text, palette.CurrentDirFg, palette.SelectedFg)
	headerText.A = 0xFF

	muted := mixNRGBA(baseText, baseBg, 0.38)
	muted.A = 226
	hint := mixNRGBA(baseText, baseBg, 0.52)
	hint.A = 218

	errorText := bestContrastColor(baseBg, color.NRGBA{R: 255, G: 186, B: 186, A: 255}, headerText)
	errorText = mixNRGBA(errorText, color.NRGBA{R: 255, G: 164, B: 164, A: 255}, 0.28)
	errorText.A = 0xFF

	statusAccent := mixNRGBA(headerText, palette.CurrentDirFg, 0.18)
	statusAccent.A = 0xFF
	statusWarn := mixNRGBA(bestContrastColor(baseBg, headerText, baseText), color.NRGBA{R: 255, G: 208, B: 136, A: 255}, 0.34)
	statusWarn.A = 0xFF
	statusError := mixNRGBA(errorText, color.NRGBA{R: 255, G: 170, B: 170, A: 255}, 0.18)
	statusError.A = 0xFF

	divider := mixNRGBA(headerText, baseBg, 0.8)
	divider.A = 36

	selectionText := bestContrastColor(selectionBase, popup.ActiveText, popup.HoverText, baseText)
	selection := mixNRGBA(baseBg, selectionBase, 0.88)
	selection = mixNRGBA(selection, selectionText, 0.08)
	selection.A = 168
	strongSelection := mixNRGBA(baseBg, selectionBase, 0.95)
	strongSelection = mixNRGBA(strongSelection, selectionText, 0.14)
	strongSelection.A = 214
	hexSelectionBase := selectionBase
	if cfg != nil {
		if c, ok := fm.ParseHexColor(strings.TrimSpace(cfg.Viewer.HexSelection)); ok {
			hexSelectionBase = c
		}
	}
	hexSelectionText := bestContrastColor(hexSelectionBase, popup.ActiveText, popup.HoverText, baseText)
	hexSelection := mixNRGBA(baseBg, hexSelectionBase, 0.88)
	hexSelection = mixNRGBA(hexSelection, hexSelectionText, 0.08)
	hexSelection.A = 168
	hexStrongSelection := mixNRGBA(baseBg, hexSelectionBase, 0.95)
	hexStrongSelection = mixNRGBA(hexStrongSelection, hexSelectionText, 0.14)
	hexStrongSelection.A = 214

	scrollThumbOverride := ""
	scrollTrackOverride := ""
	if cfg != nil {
		scrollThumbOverride = cfg.Colors.ScrollbarThumb
		scrollTrackOverride = cfg.Colors.ScrollbarTrack
	}
	scrollTrack, scrollTrackHover, scrollThumb, scrollThumbHover, scrollThumbDrag := filePaneScrollbarColors(baseBg, baseText, headerText, popup.HoverText, selectionText, scrollThumbOverride, scrollTrackOverride)

	tooltipBg := mixNRGBA(baseBg, popup.Bg, 0.44)
	tooltipBg.A = 246
	tooltipBorder := mixNRGBA(panelBorder, popup.Border, 0.56)
	tooltipBorder.A = 88
	tooltipText := bestContrastColor(tooltipBg, headerText, popup.Text, baseText)
	tooltipText.A = 0xFF

	separatorBase := bestContrastColor(baseBg, baseText, headerText, palette.CurrentDirFg, popup.HoverText)
	separator := mixNRGBA(separatorBase, baseBg, 0.48)
	separator.A = 176
	offsetText := mixNRGBA(headerText, baseBg, 0.40)
	offsetText.A = 0xFF
	hexText := baseText
	asciiText := mixNRGBA(baseText, palette.CurrentDirFg, 0.22)
	asciiText.A = 0xFF
	if cfg != nil {
		if c, ok := fm.ParseHexColor(strings.TrimSpace(cfg.Viewer.HexOffsetText)); ok {
			offsetText = c
		}
		if c, ok := fm.ParseHexColor(strings.TrimSpace(cfg.Viewer.HexBytesText)); ok {
			hexText = c
		}
		if c, ok := fm.ParseHexColor(strings.TrimSpace(cfg.Viewer.HexASCIIText)); ok {
			asciiText = c
		}
	}

	commandBg := mixNRGBA(baseBg, palette.CurrentDirBg, 0.32)
	commandBg.A = 0xFF
	commandBgHover := mixNRGBA(commandBg, palette.HoverBg, 0.42)
	commandBgHover.A = 0xFF
	commandBorder := mixNRGBA(panelBorder, palette.CurrentDirFg, 0.26)
	commandBorder.A = 132
	commandBorderHover := mixNRGBA(commandBorder, popup.HoverText, 0.32)
	commandBorderHover.A = 168
	commandText := bestContrastColor(commandBg, baseText, palette.CurrentDirFg, popup.Text)
	commandText.A = 0xFF
	commandStaticText := mixNRGBA(commandText, popup.ActiveText, 0.34)
	if contrastScore(commandBg, commandStaticText) < contrastScore(commandBg, commandText) {
		commandStaticText = bestContrastColor(commandBg, popup.ActiveText, popup.HoverText, commandText)
	}
	commandStaticText.A = 0xFF
	commandHint := mixNRGBA(commandText, commandBg, 0.40)
	commandHint.A = 0xFF

	historyBg := mixNRGBA(baseBg, popup.Bg, 0.34)
	historyBg.A = 0xFF
	historyText := bestContrastColor(historyBg, baseText, popup.Text, headerText)
	historyText.A = 0xFF
	historyBorder := mixNRGBA(historyText, historyBg, 0.74)
	historyBorder.A = 78
	historyMuted := mixNRGBA(historyText, historyBg, 0.44)
	historyMuted.A = 0xFF
	historyChipBg := mixNRGBA(historyBg, historyText, 0.08)
	historyChipBg.A = 0xFF
	historyChipBgHover := mixNRGBA(historyChipBg, historyText, 0.09)
	historyChipBgHover.A = 0xFF
	historyChipBorder := mixNRGBA(historyText, historyChipBg, 0.70)
	historyChipBorder.A = 86
	historyChipBorderH := mixNRGBA(historyText, historyChipBgHover, 0.54)
	historyChipBorderH.A = 122
	historyChipText := bestContrastColor(historyChipBg, commandText, historyText, popup.Text)
	historyChipText.A = 0xFF

	return fileViewerTheme{
		Backdrop:           backdrop,
		HeaderBg:           headerBg,
		HeaderText:         headerText,
		PanelBg:            baseBg,
		PanelBorder:        panelBorder,
		Text:               baseText,
		HexText:            hexText,
		Muted:              muted,
		Hint:               hint,
		Error:              errorText,
		StatusAccent:       statusAccent,
		StatusWarn:         statusWarn,
		StatusError:        statusError,
		Divider:            divider,
		Selection:          selection,
		StrongSelection:    strongSelection,
		HexSelection:       hexSelection,
		HexStrongSelection: hexStrongSelection,
		ScrollTrack:        scrollTrack,
		ScrollTrackHover:   scrollTrackHover,
		ScrollThumb:        scrollThumb,
		ScrollThumbHover:   scrollThumbHover,
		ScrollThumbDrag:    scrollThumbDrag,
		TooltipBg:          tooltipBg,
		TooltipBorder:      tooltipBorder,
		TooltipText:        tooltipText,
		Separator:          separator,
		OffsetText:         offsetText,
		ASCIIText:          asciiText,
		CommandBg:          commandBg,
		CommandBgHover:     commandBgHover,
		CommandBorder:      commandBorder,
		CommandBorderHover: commandBorderHover,
		CommandText:        commandText,
		CommandStaticText:  commandStaticText,
		CommandHint:        commandHint,
		HistoryBg:          historyBg,
		HistoryBorder:      historyBorder,
		HistoryText:        historyText,
		HistoryMuted:       historyMuted,
		HistoryChipBg:      historyChipBg,
		HistoryChipBgHover: historyChipBgHover,
		HistoryChipBorder:  historyChipBorder,
		HistoryChipBorderH: historyChipBorderH,
		HistoryChipText:    historyChipText,
	}
}

func (ui *UI) fileViewerTheme() fileViewerTheme {
	if ui == nil {
		return fileViewerThemeFromConfig(nil)
	}
	return fileViewerThemeFromConfig(ui.fmCfg)
}
