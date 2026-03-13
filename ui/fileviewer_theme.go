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
	Muted              color.NRGBA
	Hint               color.NRGBA
	Error              color.NRGBA
	StatusAccent       color.NRGBA
	StatusWarn         color.NRGBA
	StatusError        color.NRGBA
	Divider            color.NRGBA
	Selection          color.NRGBA
	StrongSelection    color.NRGBA
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
	panelBorder.A = 38

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

	divider := mixNRGBA(headerText, headerBg, 0.78)
	divider.A = 28

	selectionText := bestContrastColor(selectionBase, popup.ActiveText, popup.HoverText, baseText)
	selection := mixNRGBA(baseBg, selectionBase, 0.88)
	selection = mixNRGBA(selection, selectionText, 0.08)
	selection.A = 168
	strongSelection := mixNRGBA(baseBg, selectionBase, 0.95)
	strongSelection = mixNRGBA(strongSelection, selectionText, 0.14)
	strongSelection.A = 214

	scrollAccentText := bestContrastColor(baseBg, popup.ActiveText, popup.HoverText, headerText, baseText)
	scrollAccent := mixNRGBA(popup.ActiveBg, scrollAccentText, 0.18)
	scrollAccent.A = 0xFF
	scrollTrack := mixNRGBA(baseBg, scrollAccent, 0.22)
	scrollTrack.A = 54
	scrollTrackHover := mixNRGBA(baseBg, scrollAccent, 0.34)
	scrollTrackHover.A = 88
	scrollThumb := mixNRGBA(scrollAccent, scrollAccentText, 0.16)
	scrollThumb.A = 212
	scrollThumbHover := mixNRGBA(scrollAccent, scrollAccentText, 0.28)
	scrollThumbHover.A = 232
	scrollThumbDrag := mixNRGBA(scrollAccent, scrollAccentText, 0.42)
	scrollThumbDrag.A = 248

	tooltipBg := mixNRGBA(baseBg, popup.Bg, 0.44)
	tooltipBg.A = 246
	tooltipBorder := mixNRGBA(panelBorder, popup.Border, 0.56)
	tooltipBorder.A = 88
	tooltipText := bestContrastColor(tooltipBg, headerText, popup.Text, baseText)
	tooltipText.A = 0xFF

	separator := mixNRGBA(baseText, baseBg, 0.76)
	separator.A = 16
	offsetText := mixNRGBA(headerText, baseBg, 0.40)
	offsetText.A = 0xFF
	asciiText := mixNRGBA(baseText, palette.CurrentDirFg, 0.22)
	asciiText.A = 0xFF

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

	historyBg := mixNRGBA(baseBg, popup.Bg, 0.22)
	historyBg.A = 0xFF
	historyBorder := mixNRGBA(panelBorder, popup.Border, 0.36)
	historyBorder.A = 22
	historyText := bestContrastColor(historyBg, baseText, popup.Text, headerText)
	historyText.A = 0xFF
	historyMuted := mixNRGBA(historyText, historyBg, 0.44)
	historyMuted.A = 0xFF
	historyChipBg := mixNRGBA(commandBg, popup.ButtonBg, 0.36)
	historyChipBg.A = 0xFF
	historyChipBgHover := mixNRGBA(historyChipBg, popup.HoverBg, 0.46)
	historyChipBgHover.A = 0xFF
	historyChipBorder := mixNRGBA(commandBorder, popup.ButtonBorder, 0.32)
	historyChipBorder.A = 46
	historyChipBorderH := mixNRGBA(commandBorderHover, popup.HoverText, 0.18)
	historyChipBorderH.A = 76
	historyChipText := bestContrastColor(historyChipBg, commandText, historyText, popup.Text)
	historyChipText.A = 0xFF

	return fileViewerTheme{
		Backdrop:           backdrop,
		HeaderBg:           headerBg,
		HeaderText:         headerText,
		PanelBg:            baseBg,
		PanelBorder:        panelBorder,
		Text:               baseText,
		Muted:              muted,
		Hint:               hint,
		Error:              errorText,
		StatusAccent:       statusAccent,
		StatusWarn:         statusWarn,
		StatusError:        statusError,
		Divider:            divider,
		Selection:          selection,
		StrongSelection:    strongSelection,
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
