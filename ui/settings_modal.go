// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	resources "hexone"
	"hexone/fm"
	uitheme "hexone/ui/theme"
	"hexone/ui/widget/table"
	"image"
	"image/color"
	"math"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"go.yaml.in/yaml/v4"
)

type settingsModalState struct {
	backdropClick widget.Clickable
	closeClick    widget.Clickable
	saveClick     widget.Clickable
	cancelClick   widget.Clickable
	keyFocus      dialogKeyboardFocusState

	tabGeneralClick  widget.Clickable
	tabFontsClick    widget.Clickable
	tabTerminalClick widget.Clickable
	tabColorsClick   widget.Clickable
	tabViewerClick   widget.Clickable
	tabAssocClick    widget.Clickable
	tabConfigClick   widget.Clickable
	activeTab        string
	focus            settingsKeyboardFocus
	focusPending     settingsKeyboardFocus
	popupFocusKind   settingsPopupKeyboardKind
	popupFocusIndex  int
	popupFocusAction settingsPopupKeyboardAction
	navPrevTab       string
	navAnimAt        time.Time
	navHoverKey      string
	navHoverPrev     string
	navHoverAt       time.Time
	navPulseKey      string
	navPulseAt       time.Time

	colorScopePaneClick          widget.Clickable
	colorScopeViewerClick        widget.Clickable
	colorScopeFilenameClick      widget.Clickable
	colorScope                   string
	colorScopeAnim               settingsChoiceAnim
	colorCategoryClick           widget.Clickable
	colorBgPickerClick           widget.Clickable
	colorTextPickerClick         widget.Clickable
	colorValueEdit               widget.Editor
	colorTextValueEdit           widget.Editor
	colorCategoryOpen            bool
	colorCategoryOpenedAt        time.Time
	colorCategoryHoverID         string
	colorCategoryHoverAnim       segmentedAnimState
	colorPickerOpen              bool
	colorPickerTarget            string
	colorPickerBase              string
	colorPickerShade             widget.Float
	colorPickerSetClick          widget.Clickable
	colorTextTransparentBool     widget.Bool
	popupGlobalPointerTag        uiEventTag
	colorCategoryPopupTag        uiEventTag
	colorPickerPopupTag          uiEventTag
	filenameIconPickerPopupTag   uiEventTag
	filenamePermPickerPopupTag   uiEventTag
	viewTargetPickerPopupTag     uiEventTag
	viewAssocPickerPopupTag      uiEventTag
	viewRulePickerPopupTag       uiEventTag
	colorCategory                string
	colorOptionClicks            []widget.Clickable
	colorSwatchClicks            []widget.Clickable
	colorPaneBackground          string
	colorPaneText                string
	colorHover                   string
	colorHoverText               string
	colorPopupHover              string
	colorPopupHoverText          string
	colorSelection               string
	colorSelectionText           string
	colorSelectedFiles           string
	colorSelectedFilesText       string
	colorFocusedSelected         string
	colorFocusedSelectedText     string
	colorCurrentDir              string
	colorCurrentDirText          string
	colorScrollbarThumb          string
	colorScrollbarTrack          string
	colorViewerBackground        string
	colorViewerText              string
	colorViewerSelection         string
	colorViewerHexSelection      string
	colorViewerHexOffsetText     string
	colorViewerHexBytesText      string
	colorViewerHexASCIIText      string
	viewerPreviewMode            string
	viewerPreviewFileClick       widget.Clickable
	viewerPreviewHexClick        widget.Clickable
	viewerPreviewModeAnim        settingsChoiceAnim
	filenameDefaultText          string
	filenameDefaultTextEdit      widget.Editor
	filenameDefaultIcon          string
	filenameDefaultTarget        string
	filenameDefaultTargetAnim    settingsChoiceAnim
	filenameDefaultTargetClicks  [3]widget.Clickable
	filenameDefaultIconClick     widget.Clickable
	filenameDefaultTextPicker    widget.Clickable
	filenameIconPickerOpen       bool
	filenameIconPickerTarget     string
	filenameIconSwatchClicks     []widget.Clickable
	filenameRuleMode             string
	filenameRuleModeAnim         settingsChoiceAnim
	filenameRuleModeAgeClick     widget.Clickable
	filenameRuleModePermClick    widget.Clickable
	filenameRuleModeExtClick     widget.Clickable
	filenameRuleModeSizeClick    widget.Clickable
	filenameAgeOffsetEdit        widget.Editor
	filenameAgeUnit              string
	filenameAgeUnitAnim          settingsChoiceAnim
	filenameAgeUnitClicks        [4]widget.Clickable
	filenameAgeTextEdit          widget.Editor
	filenameAgeIcon              string
	filenameAgeTarget            string
	filenameAgeTargetAnim        settingsChoiceAnim
	filenameAgeTargetClicks      [3]widget.Clickable
	filenameAgeIconClick         widget.Clickable
	filenameAgeTextPicker        widget.Clickable
	filenameAgeApplyClick        widget.Clickable
	filenameAgeRemoveClick       widget.Clickable
	filenameAgeList              widget.List
	filenameAgeEntries           []fm.FilenameAgeRule
	filenameAgeSavedEntries      []fm.FilenameAgeRule
	filenameAgeLookup            string
	filenameAgeEditingKey        string
	filenameAgeRowClicks         map[string]*widget.Clickable
	filenameAgeRowRemove         map[string]*widget.Clickable
	filenameAgeInfoText          string
	filenamePermEdit             widget.Editor
	filenamePermMatch            string
	filenamePermMatchAnim        settingsChoiceAnim
	filenamePermMatchClicks      [4]widget.Clickable
	filenamePermChecks           [9]widget.Bool
	filenamePermPickerOpen       bool
	filenamePermPickerClick      widget.Clickable
	filenamePermTextEdit         widget.Editor
	filenamePermIcon             string
	filenamePermTarget           string
	filenamePermTargetAnim       settingsChoiceAnim
	filenamePermTargetClicks     [3]widget.Clickable
	filenamePermIconClick        widget.Clickable
	filenamePermTextPicker       widget.Clickable
	filenamePermApplyClick       widget.Clickable
	filenamePermRemoveClick      widget.Clickable
	filenamePermList             widget.List
	filenamePermEntries          []fm.FilenamePermissionRule
	filenamePermSavedEntries     []fm.FilenamePermissionRule
	filenamePermLookup           string
	filenamePermEditingKey       string
	filenamePermRowClicks        map[string]*widget.Clickable
	filenamePermRowRemove        map[string]*widget.Clickable
	filenamePermInfoText         string
	filenameExtEdit              widget.Editor
	filenameExtTextEdit          widget.Editor
	filenameExtIcon              string
	filenameExtIconClick         widget.Clickable
	filenameExtTextPicker        widget.Clickable
	filenameExtApplyClick        widget.Clickable
	filenameExtRemoveClick       widget.Clickable
	filenameExtList              widget.List
	filenameExtEntries           []fm.FilenameExtensionRule
	filenameExtSavedEntries      []fm.FilenameExtensionRule
	filenameExtLookup            string
	filenameExtEditingKey        string
	filenameExtRowClicks         map[string]*widget.Clickable
	filenameExtRowRemove         map[string]*widget.Clickable
	filenameExtInfoText          string
	filenameSizeEdit             widget.Editor
	filenameSizeMatch            string
	filenameSizeMatchAnim        settingsChoiceAnim
	filenameSizeMatchClicks      [2]widget.Clickable
	filenameSizeUnit             string
	filenameSizeUnitAnim         settingsChoiceAnim
	filenameSizeUnitClicks       [5]widget.Clickable
	filenameSizeTextEdit         widget.Editor
	filenameSizeIcon             string
	filenameSizeIconClick        widget.Clickable
	filenameSizeTextPicker       widget.Clickable
	filenameSizeApplyClick       widget.Clickable
	filenameSizeRemoveClick      widget.Clickable
	filenameSizeList             widget.List
	filenameSizeEntries          []fm.FilenameSizeRule
	filenameSizeSavedEntries     []fm.FilenameSizeRule
	filenameSizeLookup           string
	filenameSizeEditingKey       string
	filenameSizeRowClicks        map[string]*widget.Clickable
	filenameSizeRowRemove        map[string]*widget.Clickable
	filenameSizeInfoText         string
	viewCommandEdit              widget.Editor
	viewShellEdit                widget.Editor
	viewShellOptions             []terminalShellOption
	viewShellClicks              []widget.Clickable
	viewShellAnim                settingsChoiceAnim
	viewRemoteSearchCommandEdit  widget.Editor
	interfaceFontFamily          string
	currentDirFontFamily         string
	paneFontFamily               string
	tabsFontFamily               string
	viewFontFamily               string
	terminalFontFamily           string
	interfaceFontSizeSp          float32
	currentDirFontSizeSp         float32
	paneFontSizeSp               float32
	tabsFontSizeSp               float32
	viewFontSizeSp               float32
	terminalFontSizeSp           float32
	paneFileWeight               string
	paneDirWeight                string
	panePermissionsWeight        string
	paneSizeWeight               string
	paneDateWeight               string
	paneSettingsMode             string
	paneSettingsModeAnim         settingsChoiceAnim
	paneSettingsFullClick        widget.Clickable
	paneSettingsBriefClick       widget.Clickable
	paneSettingsOtherClick       widget.Clickable
	paneFullChars                float32
	paneBriefChars               float32
	paneFullCharsStepper         settingsNumberStepperState
	paneBriefCharsStepper        settingsNumberStepperState
	paneFullCharsHelpClick       widget.Clickable
	paneBriefCharsHelpClick      widget.Clickable
	paneShowPermissions          bool
	panePermissionFormat         string
	panePermissionFormatAnim     settingsChoiceAnim
	panePermissionFormatClicks   [4]widget.Clickable
	paneDatePreset               string
	paneTimePreset               string
	paneDatePresetAnim           settingsChoiceAnim
	paneTimePresetAnim           settingsChoiceAnim
	paneDatePresetClicks         [4]widget.Clickable
	paneTimePresetClicks         [4]widget.Clickable
	paneDateFormatEdit           widget.Editor
	paneDateFallbackFormats      []string
	interfaceFontSizeStepper     settingsNumberStepperState
	currentDirFontSizeStepper    settingsNumberStepperState
	paneFontSizeStepper          settingsNumberStepperState
	tabsFontSizeStepper          settingsNumberStepperState
	viewFontSizeStepper          settingsNumberStepperState
	terminalFontSizeStepper      settingsNumberStepperState
	interfaceFontFamilyClicks    []widget.Clickable
	currentDirFontFamilyClicks   []widget.Clickable
	paneFontFamilyClicks         []widget.Clickable
	tabsFontFamilyClicks         []widget.Clickable
	viewFontFamilyClicks         []widget.Clickable
	terminalFontFamilyClicks     []widget.Clickable
	interfaceFontPickerAnim      settingsChoiceAnim
	currentDirFontPickerAnim     settingsChoiceAnim
	paneFontPickerAnim           settingsChoiceAnim
	tabsFontPickerAnim           settingsChoiceAnim
	viewFontPickerAnim           settingsChoiceAnim
	terminalFontPickerAnim       settingsChoiceAnim
	paneFileWeightAnim           settingsChoiceAnim
	paneDirWeightAnim            settingsChoiceAnim
	panePermissionsWeightAnim    settingsChoiceAnim
	paneSizeWeightAnim           settingsChoiceAnim
	paneDateWeightAnim           settingsChoiceAnim
	paneFileWeightClicks         [2]widget.Clickable
	paneDirWeightClicks          [2]widget.Clickable
	panePermissionsWeightClicks  [2]widget.Clickable
	paneSizeWeightClicks         [2]widget.Clickable
	paneDateWeightClicks         [2]widget.Clickable
	terminalAcceleratedKeysBool  widget.Bool
	terminalPreviewStart         int
	terminalPreviewEnd           int
	terminalPreviewStartStepper  settingsNumberStepperState
	terminalPreviewEndStepper    settingsNumberStepperState
	generalDimInactiveBool       widget.Bool
	generalFavoritesNewTabBool   widget.Bool
	generalWheelMovesSelection   widget.Bool
	generalUseTrash              widget.Bool
	generalDeleteWithoutConfirm  widget.Bool
	generalCompletionSound       string
	generalCompletionSoundAnim   settingsChoiceAnim
	generalCompletionSoundClicks [3]widget.Clickable
	viewSmoothScrollingBool      widget.Bool
	viewShowLineNumbersBool      widget.Bool
	viewHideFunctionBarBool      widget.Bool
	generalTabList               widget.List
	viewerTabList                widget.List
	colorsTabList                widget.List
	viewTargetKeyEdit            widget.Editor
	viewTargetCommandEdit        widget.Editor
	viewTargetApplyClick         widget.Clickable
	viewTargetPickClick          widget.Clickable
	viewTargetRemoveClick        widget.Clickable
	viewTargetPickOpen           bool
	viewTargetPickList           widget.List
	viewTargetPickRemember       int
	viewTargetRowClicks          map[string]*widget.Clickable
	viewTargetRowRemoveClicks    map[string]*widget.Clickable
	viewTargetEntries            []viewerCommandTargetEntry
	viewTargetSavedEntries       []viewerCommandTargetEntry
	viewTargetLookupKey          string
	viewTargetEditingKey         string
	viewRulePatternEdit          widget.Editor
	viewRuleCommandEdit          widget.Editor
	viewRuleApplyClick           widget.Clickable
	viewRulePickClick            widget.Clickable
	viewRuleRemoveClick          widget.Clickable
	viewRulePickOpen             bool
	viewRulePickList             widget.List
	viewRulePickRemember         int
	viewRuleRowClicks            map[string]*widget.Clickable
	viewRuleRowRemoveClicks      map[string]*widget.Clickable
	viewRuleEntries              []fm.ViewerCommandRule
	viewRuleSavedEntries         []fm.ViewerCommandRule
	viewRuleLookupPattern        string
	viewRuleEditingPattern       string
	viewAssocExtEdit             widget.Editor
	viewAssocAppEdit             widget.Editor
	viewAssocApplyClick          widget.Clickable
	viewAssocPickClick           widget.Clickable
	viewAssocRemoveClick         widget.Clickable
	viewAssocPickOpen            bool
	viewAssocPickList            layout.List
	viewAssocPickRemember        int
	viewAssocRowClicks           map[string]*widget.Clickable
	viewAssocEntries             []fm.ViewerAssociation
	viewAssocSavedEntries        []fm.ViewerAssociation
	viewAssocLookupExt           string
	viewAssocEditingExt          string

	footerFocus     settingsFooterAction
	footerHoverKey  string
	footerHoverPrev string
	footerHoverAt   time.Time
	footerPulseKey  string
	footerPulseAt   time.Time

	configEdit       widget.Editor
	configPathSelect widget.Selectable
	configScrollbar  widget.Scrollbar

	errText        string
	targetInfoText string
	ruleInfoText   string
	assocInfoText  string
	baselineDraft  string
	baselineConfig string
}

type settingsNumberStepperState struct {
	valueClick widget.Clickable
	upClick    widget.Clickable
	downClick  widget.Clickable
}

const (
	settingsFontSizeMin  float32 = 6
	settingsFontSizeStep float32 = 1
)

type settingsChoiceAnim struct {
	prev    string
	hasPrev bool
	at      time.Time
	anim    segmentedAnimState
}

type viewerAssociationProgram struct {
	AppPath    string
	Extensions []string
	MatchRank  int
}

type viewerCommandTargetEntry struct {
	Key     string
	Command string
}

type viewerSettingsSectionStyle struct {
	Fill        color.NRGBA
	Border      color.NRGBA
	BadgeFill   color.NRGBA
	BadgeBorder color.NRGBA
	BadgeText   color.NRGBA
}

type settingsColorOption struct {
	key   string
	label string
}

var settingsPaneColorOptions = []settingsColorOption{
	{key: "normal", label: "Normal"},
	{key: "hover", label: "Hover"},
	{key: "popup_hover", label: "Popup Hover"},
	{key: "selection", label: "Focused"},
	{key: "selected_files", label: "Selected Files"},
	{key: "focused_selected", label: "Focused + Selected Files"},
	{key: "current_dir", label: "Current Dir"},
	{key: "scrollbar", label: "Scrollbar"},
}

var settingsViewerColorOptions = []settingsColorOption{
	{key: "normal", label: "Normal"},
	{key: "selection", label: "Selection"},
	{key: "hex_selection", label: "Hex Selection"},
	{key: "hex_offset", label: "Hex Offset"},
	{key: "hex_bytes", label: "Hex Bytes"},
	{key: "hex_ascii", label: "Hex ASCII"},
}

var settingsTabOrder = []string{
	"general",
	"fonts",
	"colors",
	"terminal",
	"viewer",
	"associations",
	"config",
}

type settingsColorSwatchGroup struct {
	hexes []string
}

const settingsColorHiveRadius = 6

func settingsColorSwatchGroups(_ string) []settingsColorSwatchGroup {
	return settingsColorHiveGroups()
}

func settingsColorHiveGroups() []settingsColorSwatchGroup {
	groups := make([]settingsColorSwatchGroup, 0, settingsColorHiveRadius*2+1)
	for axialRow := -settingsColorHiveRadius; axialRow <= settingsColorHiveRadius; axialRow++ {
		qMin := max(-settingsColorHiveRadius, -axialRow-settingsColorHiveRadius)
		qMax := min(settingsColorHiveRadius, -axialRow+settingsColorHiveRadius)
		group := settingsColorSwatchGroup{}
		for q := qMin; q <= qMax; q++ {
			group.hexes = append(group.hexes, settingsColorHiveHex(q, axialRow, settingsColorHiveRadius))
		}
		groups = append(groups, group)
	}
	return groups
}

func settingsColorHiveHex(q, r, radius int) string {
	x := math.Sqrt(3) * (float64(q) + float64(r)/2)
	y := 1.5 * float64(r)
	distance := max(absInt(q), absInt(r), absInt(-q-r))
	if distance == 0 || radius <= 0 {
		return "#FFFFFF"
	}
	hue := math.Mod(math.Atan2(y, x)*180/math.Pi+330+360, 360)
	outer := settingsHSVColor(hue, 1, 0.94)
	// A tint field matches the reference palette: each ray owns one hue,
	// while every step away from white has a deliberately different tint.
	amount := math.Pow(float64(distance)/float64(radius), 0.82)
	return fm.FormatHexColor(mixNRGBA(
		color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		outer,
		float32(amount),
	))
}

func settingsHSVColor(hue, saturation, value float64) color.NRGBA {
	hue = math.Mod(hue+360, 360) / 60
	saturation = math.Max(0, math.Min(1, saturation))
	value = math.Max(0, math.Min(1, value))
	chroma := value * saturation
	x := chroma * (1 - math.Abs(math.Mod(hue, 2)-1))
	m := value - chroma
	var r, g, b float64
	switch int(math.Floor(hue)) % 6 {
	case 0:
		r, g, b = chroma, x, 0
	case 1:
		r, g, b = x, chroma, 0
	case 2:
		r, g, b = 0, chroma, x
	case 3:
		r, g, b = 0, x, chroma
	case 4:
		r, g, b = x, 0, chroma
	case 5:
		r, g, b = chroma, 0, x
	}
	return color.NRGBA{
		R: uint8(math.Round((r + m) * 255)),
		G: uint8(math.Round((g + m) * 255)),
		B: uint8(math.Round((b + m) * 255)),
		A: 255,
	}
}

func settingsColorShade(baseHex string, value float32) string {
	base, ok := fm.ParseHexColor(baseHex)
	if !ok {
		base, _ = fm.ParseHexColor(fm.DefaultFilePaneSelectionHex)
	}
	value = max(float32(0), min(float32(1), value))
	if value <= 0.5 {
		return fm.FormatHexColor(mixNRGBA(color.NRGBA{A: 255}, base, value*2))
	}
	return fm.FormatHexColor(mixNRGBA(base, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, (value-0.5)*2))
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func settingsColorSwatchCount(groups []settingsColorSwatchGroup) int {
	total := 0
	for _, group := range groups {
		total += len(group.hexes)
	}
	return total
}

func (ui *UI) openSettingsModal() {
	if ui == nil {
		return
	}
	ui.closeFunctionBarToolsMenu()
	if err := ui.ensureFMConfigLoaded(); err != nil {
		return
	}
	st := ui.settingsModal
	if st == nil {
		st = &settingsModalState{
			activeTab:              "general",
			footerFocus:            settingsFooterActionSave,
			popupFocusIndex:        -1,
			viewTargetPickRemember: -1,
			viewRulePickRemember:   -1,
			viewAssocPickRemember:  -1,
		}
		st.viewCommandEdit.SingleLine = true
		st.viewCommandEdit.Submit = false
		st.colorValueEdit.SingleLine = true
		st.colorValueEdit.Submit = false
		st.colorTextValueEdit.SingleLine = true
		st.colorTextValueEdit.Submit = false
		st.filenameDefaultTextEdit.SingleLine = true
		st.filenameDefaultTextEdit.Submit = false
		st.filenameAgeOffsetEdit.SingleLine = true
		st.filenameAgeOffsetEdit.Submit = false
		st.filenameAgeTextEdit.SingleLine = true
		st.filenameAgeTextEdit.Submit = false
		st.filenameAgeList.Axis = layout.Vertical
		st.filenamePermEdit.SingleLine = true
		st.filenamePermEdit.Submit = false
		st.filenamePermTextEdit.SingleLine = true
		st.filenamePermTextEdit.Submit = false
		st.filenamePermList.Axis = layout.Vertical
		st.filenameExtEdit.SingleLine = true
		st.filenameExtEdit.Submit = false
		st.filenameExtTextEdit.SingleLine = true
		st.filenameExtTextEdit.Submit = false
		st.filenameExtList.Axis = layout.Vertical
		st.filenameSizeEdit.SingleLine = true
		st.filenameSizeEdit.Submit = false
		st.filenameSizeTextEdit.SingleLine = true
		st.filenameSizeTextEdit.Submit = false
		st.filenameSizeList.Axis = layout.Vertical
		st.viewShellEdit.SingleLine = true
		st.viewShellEdit.Submit = false
		st.viewRemoteSearchCommandEdit.SingleLine = true
		st.viewRemoteSearchCommandEdit.Submit = false
		st.paneDateFormatEdit.SingleLine = true
		st.paneDateFormatEdit.Submit = false
		st.generalTabList.Axis = layout.Vertical
		st.viewerTabList.Axis = layout.Vertical
		st.colorsTabList.Axis = layout.Vertical
		st.viewTargetKeyEdit.SingleLine = true
		st.viewTargetKeyEdit.Submit = false
		st.viewTargetCommandEdit.SingleLine = true
		st.viewTargetCommandEdit.Submit = false
		st.viewTargetPickList.Axis = layout.Vertical
		st.viewRulePatternEdit.SingleLine = true
		st.viewRulePatternEdit.Submit = false
		st.viewRuleCommandEdit.SingleLine = true
		st.viewRuleCommandEdit.Submit = false
		st.viewRulePickList.Axis = layout.Vertical
		st.viewAssocExtEdit.SingleLine = true
		st.viewAssocExtEdit.Submit = false
		st.viewAssocAppEdit.SingleLine = true
		st.viewAssocAppEdit.Submit = false
		st.viewAssocPickList.Axis = layout.Vertical
		st.configEdit.SingleLine = false
		st.configEdit.Submit = false
	}
	st.viewShellOptions = terminalDetectedShellOptions()
	st.loadFromConfig(ui.fmCfg)
	st.keyFocus.focusKeyboard()
	st.normalizeKeyboardFocus()
	ui.settingsModal = st
}

func (ui *UI) closeSettingsModal() {
	ui.settingsModal = nil
}

func (st *settingsModalState) loadFromConfig(cfg *fm.Config) {
	if st == nil || cfg == nil {
		return
	}
	switch st.colorCategory {
	case "normal", "hover", "popup_hover", "selection", "selected_files", "focused_selected", "current_dir", "scrollbar":
	default:
		st.colorCategory = "selection"
	}
	switch st.colorScope {
	case "viewer", "filenames":
	default:
		st.colorScope = "panes"
	}
	st.colorPaneBackground = cfg.Colors.FilePaneBackground
	st.colorPaneText = cfg.Colors.FilePaneText
	st.colorHover = cfg.Colors.Hover
	st.colorHoverText = cfg.Colors.HoverText
	st.colorPopupHover = cfg.Colors.PopupHover
	st.colorPopupHoverText = cfg.Colors.PopupHoverText
	st.colorSelection = cfg.Colors.Selection
	st.colorSelectionText = cfg.Colors.SelectionText
	st.colorSelectedFiles = cfg.Colors.SelectedFiles
	st.colorSelectedFilesText = cfg.Colors.SelectedFilesText
	st.colorFocusedSelected = cfg.Colors.FocusedSelected
	st.colorFocusedSelectedText = cfg.Colors.FocusedSelectedText
	st.colorCurrentDir = cfg.Colors.CurrentDirBg
	st.colorCurrentDirText = cfg.Colors.CurrentDirText
	st.colorScrollbarThumb = cfg.Colors.ScrollbarThumb
	st.colorScrollbarTrack = cfg.Colors.ScrollbarTrack
	st.colorViewerBackground = cfg.Viewer.Background
	st.colorViewerText = cfg.Viewer.Text
	st.colorViewerSelection = cfg.Viewer.Selection
	st.colorViewerHexSelection = cfg.Viewer.HexSelection
	st.colorViewerHexOffsetText = cfg.Viewer.HexOffsetText
	st.colorViewerHexBytesText = cfg.Viewer.HexBytesText
	st.colorViewerHexASCIIText = cfg.Viewer.HexASCIIText
	if st.viewerPreviewMode != "file" && st.viewerPreviewMode != "hex" {
		st.viewerPreviewMode = "file"
		st.viewerPreviewModeAnim = settingsChoiceAnim{}
	}
	st.loadFilenameColorsFromConfig(cfg)
	st.colorCategory = normalizeSettingsColorCategory(st.colorScope, st.colorCategory)
	st.syncColorEditors()
	st.colorCategoryOpen = false
	st.colorCategoryOpenedAt = time.Time{}
	st.colorCategoryHoverID = ""
	st.colorCategoryHoverAnim = segmentedAnimState{}
	st.colorPickerOpen = false
	st.colorPickerTarget = ""
	st.colorPickerBase = ""
	st.colorPickerShade.Value = 0.5
	st.filenameIconPickerOpen = false
	st.filenameIconPickerTarget = ""
	st.viewCommandEdit.SetText(cfg.Viewer.Command)
	st.viewShellEdit.SetText(normalizeViewerShellInput(cfg.Viewer.Shell))
	st.viewShellAnim = settingsChoiceAnim{}
	st.viewRemoteSearchCommandEdit.SetText(fm.NormalizeViewerRemoteSearchCommand(cfg.Viewer.RemoteSearchCommand))
	st.interfaceFontFamily = cfg.Interface.Typeface
	st.currentDirFontFamily = cfg.CurrentDir.Typeface
	st.paneFontFamily = cfg.General.Typeface
	st.tabsFontFamily = cfg.Tabs.Typeface
	st.viewFontFamily = cfg.Viewer.Typeface
	st.terminalFontFamily = cfg.Terminal.Typeface
	st.interfaceFontSizeSp = settingsNormalizedFontSize(cfg.Interface.FontSizeSp, 14)
	st.currentDirFontSizeSp = settingsNormalizedFontSize(cfg.CurrentDir.FontSizeSp, 11)
	st.paneFontSizeSp = settingsNormalizedFontSize(cfg.General.FontSizeSp, 14)
	st.tabsFontSizeSp = settingsNormalizedFontSize(cfg.Tabs.FontSizeSp, 10)
	st.viewFontSizeSp = settingsNormalizedFontSize(cfg.Viewer.FontSizeSp, 13)
	st.terminalFontSizeSp = settingsNormalizedFontSize(cfg.Terminal.FontSizeSp, 13)
	st.paneFileWeight = fm.NormalizeFontWeight(cfg.General.FileWeight, fm.FontWeightRegular)
	st.paneDirWeight = fm.NormalizeFontWeight(cfg.General.DirWeight, fm.FontWeightBold)
	st.panePermissionsWeight = fm.NormalizeFontWeight(cfg.General.PermissionsWeight, fm.FontWeightRegular)
	st.paneSizeWeight = fm.NormalizeFontWeight(cfg.General.SizeWeight, fm.FontWeightRegular)
	st.paneDateWeight = fm.NormalizeFontWeight(cfg.General.DateWeight, fm.FontWeightRegular)
	st.paneSettingsMode = normalizeSettingsPaneMode(st.paneSettingsMode)
	st.paneSettingsModeAnim = settingsChoiceAnim{}
	st.paneFullChars = settingsNormalizePaneChars(cfg.Columns.NameChars, 20)
	st.paneBriefChars = settingsNormalizePaneChars(cfg.Columns.BriefChars, 16)
	st.paneShowPermissions = cfg.Columns.ShowPermissions
	st.panePermissionFormat = settingsNormalizePermissionFormat(cfg.Columns.PermissionFormat)
	st.panePermissionFormatAnim = settingsChoiceAnim{}
	st.loadPaneDateFormat(cfg.DateFormats)
	st.terminalAcceleratedKeysBool.Value = cfg.Terminal.AcceleratedKeys
	st.terminalPreviewStart, st.terminalPreviewEnd = fm.NormalizeTerminalPreviewRange(cfg.Terminal.PreviewStart, cfg.Terminal.PreviewEnd)
	st.interfaceFontPickerAnim = settingsChoiceAnim{}
	st.paneFontPickerAnim = settingsChoiceAnim{}
	st.tabsFontPickerAnim = settingsChoiceAnim{}
	st.viewFontPickerAnim = settingsChoiceAnim{}
	st.terminalFontPickerAnim = settingsChoiceAnim{}
	st.paneFileWeightAnim = settingsChoiceAnim{}
	st.paneDirWeightAnim = settingsChoiceAnim{}
	st.panePermissionsWeightAnim = settingsChoiceAnim{}
	st.paneSizeWeightAnim = settingsChoiceAnim{}
	st.paneDateWeightAnim = settingsChoiceAnim{}
	st.generalDimInactiveBool.Value = cfg.General.DimInactivePanes
	st.generalFavoritesNewTabBool.Value = cfg.General.OpenFavoritesInNewTab
	st.generalWheelMovesSelection.Value = cfg.General.WheelMovesSelection
	st.generalUseTrash.Value = cfg.General.UseTrash
	st.generalDeleteWithoutConfirm.Value = cfg.General.DeleteWithoutConfirm
	st.generalCompletionSound = fm.NormalizeCompletionSound(cfg.General.CompletionSound)
	st.generalCompletionSoundAnim = settingsChoiceAnim{}
	st.viewSmoothScrollingBool.Value = cfg.Viewer.SmoothScrolling
	st.viewShowLineNumbersBool.Value = cfg.Viewer.ShowLineNumbers
	st.viewHideFunctionBarBool.Value = cfg.Viewer.HideFunctionBarWhenOpen
	st.generalTabList.Position.First = 0
	st.generalTabList.Position.Offset = 0
	st.viewerTabList.Position.First = 0
	st.viewerTabList.Position.Offset = 0
	st.colorsTabList.Position.First = 0
	st.colorsTabList.Position.Offset = 0
	st.viewTargetEntries = viewerCommandTargetEntries(cfg.Viewer.CommandByTarget)
	st.viewTargetSavedEntries = append([]viewerCommandTargetEntry(nil), st.viewTargetEntries...)
	st.viewTargetPickOpen = false
	st.viewTargetPickList.Position.First = 0
	st.viewTargetPickList.Position.Offset = 0
	st.loadViewerCommandTargetFields("", "")
	st.viewRuleEntries = append([]fm.ViewerCommandRule(nil), cfg.Viewer.CommandRules...)
	st.viewRuleSavedEntries = append([]fm.ViewerCommandRule(nil), st.viewRuleEntries...)
	st.viewRulePickOpen = false
	st.viewRulePickList.Position.First = 0
	st.viewRulePickList.Position.Offset = 0
	st.loadViewerCommandRuleFields("", "")
	st.viewAssocEntries = append([]fm.ViewerAssociation(nil), fm.FlattenAssociationPrograms(cfg.Associations)...)
	st.viewAssocSavedEntries = append([]fm.ViewerAssociation(nil), st.viewAssocEntries...)
	st.viewAssocPickOpen = false
	st.viewAssocPickList.Position.First = 0
	st.viewAssocPickList.Position.Offset = 0
	st.loadViewerAssociationFields("", "")
	if raw, err := yaml.Marshal(cfg); err == nil {
		st.configEdit.SetText(string(raw))
	}
	st.errText = ""
	st.targetInfoText = ""
	st.ruleInfoText = ""
	st.assocInfoText = ""
	st.baselineConfig = st.configEdit.Text()
	st.baselineDraft = st.draftSignature()
}

func settingsColorOptionsForScope(scope string) []settingsColorOption {
	switch scope {
	case "viewer":
		return settingsViewerColorOptions
	case "filenames":
		return nil
	default:
		return settingsPaneColorOptions
	}
}

func normalizeSettingsColorCategory(scope, key string) string {
	if scope == "filenames" {
		return ""
	}
	options := settingsColorOptionsForScope(scope)
	for _, opt := range options {
		if opt.key == key {
			return key
		}
	}
	if scope == "viewer" {
		return "normal"
	}
	return "selection"
}

func settingsColorLabel(scope, key string) string {
	if scope == "filenames" {
		return ""
	}
	for _, opt := range settingsColorOptionsForScope(scope) {
		if opt.key == key {
			return opt.label
		}
	}
	options := settingsColorOptionsForScope(scope)
	if len(options) == 0 {
		return ""
	}
	return options[0].label
}

func (st *settingsModalState) colorValue(key string) string {
	if st == nil {
		return ""
	}
	if st.colorScope == "viewer" {
		switch key {
		case "selection":
			return st.colorViewerSelection
		case "hex_selection":
			return st.colorViewerHexSelection
		case "hex_offset":
			return st.colorViewerHexOffsetText
		case "hex_bytes":
			return st.colorViewerHexBytesText
		case "hex_ascii":
			return st.colorViewerHexASCIIText
		default:
			return st.colorViewerBackground
		}
	}
	switch key {
	case "focused_selected":
		return st.colorFocusedSelected
	case "current_dir":
		return st.colorCurrentDir
	case "scrollbar":
		return st.colorScrollbarThumb
	case "hover":
		return st.colorHover
	case "popup_hover":
		return st.colorPopupHover
	case "selected_files":
		return st.colorSelectedFiles
	case "normal":
		return st.colorPaneBackground
	default:
		return st.colorSelection
	}
}

func (st *settingsModalState) setColorValue(key, value string) {
	if st == nil {
		return
	}
	if st.colorScope == "viewer" {
		switch key {
		case "selection":
			st.colorViewerSelection = value
		case "hex_selection":
			st.colorViewerHexSelection = value
		case "hex_offset":
			st.colorViewerHexOffsetText = value
		case "hex_bytes":
			st.colorViewerHexBytesText = value
		case "hex_ascii":
			st.colorViewerHexASCIIText = value
		default:
			st.colorViewerBackground = value
		}
		return
	}
	switch key {
	case "focused_selected":
		st.colorFocusedSelected = value
	case "current_dir":
		st.colorCurrentDir = value
	case "scrollbar":
		st.colorScrollbarThumb = value
	case "hover":
		st.colorHover = value
	case "popup_hover":
		st.colorPopupHover = value
	case "selected_files":
		st.colorSelectedFiles = value
	case "normal":
		st.colorPaneBackground = value
	default:
		st.colorSelection = value
	}
}

func (st *settingsModalState) colorTextValue(key string) string {
	if st == nil {
		return ""
	}
	if st.colorScope == "viewer" {
		return st.colorViewerText
	}
	switch key {
	case "focused_selected":
		return st.colorFocusedSelectedText
	case "current_dir":
		return st.colorCurrentDirText
	case "scrollbar":
		return st.colorScrollbarTrack
	case "hover":
		return st.colorHoverText
	case "popup_hover":
		return st.colorPopupHoverText
	case "selected_files":
		return st.colorSelectedFilesText
	case "normal":
		return st.colorPaneText
	default:
		return st.colorSelectionText
	}
}

func (st *settingsModalState) setColorTextValue(key, value string) {
	if st == nil {
		return
	}
	if st.colorScope == "viewer" {
		st.colorViewerText = value
		return
	}
	switch key {
	case "focused_selected":
		st.colorFocusedSelectedText = value
	case "current_dir":
		st.colorCurrentDirText = value
	case "scrollbar":
		st.colorScrollbarTrack = value
	case "hover":
		st.colorHoverText = value
	case "popup_hover":
		st.colorPopupHoverText = value
	case "selected_files":
		st.colorSelectedFilesText = value
	case "normal":
		st.colorPaneText = value
	default:
		st.colorSelectionText = value
	}
}

func settingsPaneTextAllowsTransparent(key string) bool {
	switch key {
	case "hover", "selection", "selected_files", "focused_selected":
		return true
	default:
		return false
	}
}

func normalizeSettingsPaneRowTextColor(raw, label string) (string, error) {
	txt := strings.TrimSpace(raw)
	if fm.IsTransparentColor(txt) {
		return fm.TransparentColor, nil
	}
	if c, ok := fm.ParseHexColor(txt); ok {
		return fm.FormatHexColor(c), nil
	}
	return "", fmt.Errorf("%s color must use #RRGGBB or transparent", label)
}

func settingsDefaultPaneRowTextColor(key string) string {
	switch key {
	case "hover":
		return fm.DefaultFilePaneHoverTextHex
	case "selected_files":
		return fm.DefaultFilePaneSelectedTextHex
	case "focused_selected":
		return fm.DefaultFilePaneFocusedSelectedTextHex
	default:
		return fm.DefaultFilePaneSelectionTextHex
	}
}

func (st *settingsModalState) syncColorTextTransparentCheckbox() {
	if st == nil || st.colorScope != "panes" || !settingsPaneTextAllowsTransparent(st.colorCategory) {
		if st != nil {
			st.colorTextTransparentBool.Value = false
		}
		return
	}
	st.colorTextTransparentBool.Value = fm.IsTransparentColor(st.colorTextValue(st.colorCategory))
}

func (st *settingsModalState) setColorTextTransparent(enabled bool) bool {
	if st == nil || st.colorScope != "panes" || !settingsPaneTextAllowsTransparent(st.colorCategory) {
		return false
	}
	next := settingsDefaultPaneRowTextColor(st.colorCategory)
	if enabled {
		next = fm.TransparentColor
	}
	if st.colorTextValue(st.colorCategory) == next && st.colorTextTransparentBool.Value == enabled {
		return false
	}
	st.colorTextTransparentBool.Value = enabled
	st.setColorTextValue(st.colorCategory, next)
	st.colorTextValueEdit.SetText(next)
	st.errText = ""
	return true
}

func settingsViewerCategoryHasText(key string) bool {
	return key == "normal"
}

func (st *settingsModalState) syncColorEditors() {
	if st == nil {
		return
	}
	st.colorValueEdit.SetText(st.colorValue(st.colorCategory))
	st.colorTextValueEdit.SetText(st.colorTextValue(st.colorCategory))
	st.syncColorTextTransparentCheckbox()
}

func (st *settingsModalState) setColorCategory(key string) {
	if st == nil || key == "" {
		return
	}
	st.colorCategory = normalizeSettingsColorCategory(st.colorScope, key)
	st.closeSettingsPopupsExcept("")
	st.syncColorEditors()
}

func (st *settingsModalState) setColorScope(next string, now time.Time) {
	if st == nil || next == "" || st.colorScope == next {
		return
	}
	st.colorScopeAnim.setValue(&st.colorScope, next, now)
	st.colorCategory = normalizeSettingsColorCategory(st.colorScope, st.colorCategory)
	st.closeSettingsPopupsExcept("")
	st.syncColorEditors()
}

func (st *settingsModalState) openColorCategoryPopup(now time.Time) {
	if st == nil {
		return
	}
	st.closeSettingsPopupsExcept("color-category")
	st.colorCategoryOpen = true
	st.colorCategoryOpenedAt = now
	st.colorCategoryHoverID = ""
	st.colorCategoryHoverAnim = segmentedAnimState{}
}

func (st *settingsModalState) closeColorCategoryPopup() {
	if st == nil {
		return
	}
	st.colorCategoryOpen = false
	st.colorCategoryOpenedAt = time.Time{}
	st.colorCategoryHoverID = ""
	st.colorCategoryHoverAnim = segmentedAnimState{}
}

func (st *settingsModalState) toggleColorPicker(target string) {
	if st == nil || target == "" {
		return
	}
	if st.colorPickerOpen && st.colorPickerTarget == target {
		st.closeSettingsPopupsExcept("")
		return
	}
	st.closeSettingsPopupsExcept("color-picker")
	st.colorPickerOpen = true
	st.colorPickerTarget = target
	st.colorPickerBase = fm.NormalizeHexColor(st.colorPickerHexValue(target), fm.DefaultFilePaneSelectionHex)
	st.colorPickerShade.Value = 0.5
}

func (st *settingsModalState) toggleFilenameIconPicker(target string) {
	if st == nil || target == "" {
		return
	}
	if st.filenameIconPickerOpen && st.filenameIconPickerTarget == target {
		st.closeSettingsPopupsExcept("")
		return
	}
	st.closeSettingsPopupsExcept("filename-icon-picker")
	st.filenameIconPickerOpen = true
	st.filenameIconPickerTarget = target
}

func (st *settingsModalState) toggleFilenamePermissionPicker() {
	if st == nil {
		return
	}
	if st.filenamePermPickerOpen {
		st.closeSettingsPopupsExcept("")
		return
	}
	st.closeSettingsPopupsExcept("filename-perm-picker")
	st.filenamePermPickerOpen = true
	st.resetPopupKeyboardFocus()
}

func (st *settingsModalState) colorPickerHexValue(target string) string {
	if st == nil {
		return ""
	}
	switch target {
	case "text":
		return st.colorTextValue(st.colorCategory)
	case "filename-default-text":
		return strings.TrimSpace(st.filenameDefaultTextEdit.Text())
	case "filename-age-text":
		return strings.TrimSpace(st.filenameAgeTextEdit.Text())
	case "filename-perm-text":
		return strings.TrimSpace(st.filenamePermTextEdit.Text())
	case "filename-ext-text":
		return strings.TrimSpace(st.filenameExtTextEdit.Text())
	case "filename-size-text":
		return strings.TrimSpace(st.filenameSizeTextEdit.Text())
	default:
		return st.colorValue(st.colorCategory)
	}
}

func (st *settingsModalState) setColorPickerHexValue(target, hex string) {
	if st == nil || hex == "" {
		return
	}
	switch target {
	case "text":
		st.setColorTextValue(st.colorCategory, hex)
		st.colorTextValueEdit.SetText(hex)
	case "filename-default-text":
		st.filenameDefaultText = hex
		st.filenameDefaultTextEdit.SetText(hex)
	case "filename-age-text":
		st.filenameAgeTextEdit.SetText(hex)
	case "filename-perm-text":
		st.filenamePermTextEdit.SetText(hex)
	case "filename-ext-text":
		st.filenameExtTextEdit.SetText(hex)
	case "filename-size-text":
		st.filenameSizeTextEdit.SetText(hex)
	default:
		st.setColorValue(st.colorCategory, hex)
		st.colorValueEdit.SetText(hex)
	}
}

func (st *settingsModalState) colorPickerSwatchGroups(target string) []settingsColorSwatchGroup {
	base := ""
	if st != nil && st.colorPickerOpen && st.colorPickerTarget == target {
		base = st.colorPickerBase
	}
	return settingsColorSwatchGroups(base)
}

func (st *settingsModalState) anyPopupOpen() bool {
	return st != nil && (st.colorCategoryOpen || st.colorPickerOpen || st.filenameIconPickerOpen || st.filenamePermPickerOpen || st.viewTargetPickOpen || st.viewRulePickOpen || st.viewAssocPickOpen)
}

func (st *settingsModalState) popupToggleHovered() bool {
	if st == nil {
		return false
	}
	return st.colorCategoryClick.Hovered() ||
		st.colorBgPickerClick.Hovered() ||
		st.colorTextPickerClick.Hovered() ||
		st.filenameDefaultTextPicker.Hovered() ||
		st.filenameDefaultIconClick.Hovered() ||
		st.filenameAgeTextPicker.Hovered() ||
		st.filenameAgeIconClick.Hovered() ||
		st.filenamePermPickerClick.Hovered() ||
		st.filenamePermTextPicker.Hovered() ||
		st.filenamePermIconClick.Hovered() ||
		st.filenameExtTextPicker.Hovered() ||
		st.filenameExtIconClick.Hovered() ||
		st.filenameSizeTextPicker.Hovered() ||
		st.filenameSizeIconClick.Hovered() ||
		st.viewTargetPickClick.Hovered() ||
		st.viewRulePickClick.Hovered() ||
		st.viewAssocPickClick.Hovered()
}

func (st *settingsModalState) closeSettingsPopupsExcept(except string) {
	if st == nil {
		return
	}
	st.resetPopupKeyboardFocus()
	if except != "color-category" {
		st.closeColorCategoryPopup()
	}
	if except != "color-picker" {
		st.colorPickerOpen = false
		st.colorPickerTarget = ""
		st.colorPickerBase = ""
	}
	if except != "filename-icon-picker" {
		st.filenameIconPickerOpen = false
		st.filenameIconPickerTarget = ""
	}
	if except != "filename-perm-picker" {
		st.filenamePermPickerOpen = false
	}
	if except != "target-picker" {
		st.viewTargetPickOpen = false
	}
	if except != "rule-picker" {
		st.viewRulePickOpen = false
	}
	if except != "assoc-picker" {
		st.viewAssocPickOpen = false
	}
}

func (st *settingsModalState) scrollPopupKeyboardFocusIntoView() {
	if st == nil || st.popupFocusIndex < 0 {
		return
	}
	ensureVisible := func(pos *layout.Position, total int) {
		if pos == nil || st.popupFocusIndex < 0 || st.popupFocusIndex >= total {
			return
		}
		visible := pos.Count
		if visible <= 0 {
			pos.First = st.popupFocusIndex
			pos.Offset = 0
			return
		}
		if visible > 1 {
			visible--
		}
		first := pos.First
		if first < 0 {
			first = 0
		}
		last := first + visible - 1
		if last >= total {
			last = total - 1
		}
		if st.popupFocusIndex >= first && st.popupFocusIndex <= last {
			return
		}
		if st.popupFocusIndex < first {
			first = st.popupFocusIndex
		} else if st.popupFocusIndex > last {
			first = st.popupFocusIndex - (visible - 1)
		}
		if first < 0 {
			first = 0
		}
		if first >= total {
			first = total - 1
		}
		pos.First = first
		pos.Offset = 0
	}
	switch st.popupFocusKind {
	case settingsPopupKeyboardViewerTarget:
		entries, _ := st.viewerCommandTargetPickerEntries()
		ensureVisible(&st.viewTargetPickList.Position, len(entries))
	case settingsPopupKeyboardViewerRule:
		rules, _ := st.viewerCommandRulePickerRules()
		ensureVisible(&st.viewRulePickList.Position, len(rules))
	case settingsPopupKeyboardViewerAssoc:
		programs, _ := st.viewerAssociationPickerPrograms()
		ensureVisible(&st.viewAssocPickList.Position, len(programs))
	}
}

func registerSettingsPopupArea(gtx layout.Context, tag event.Tag, size image.Point) {
	if tag == nil || size.X <= 0 || size.Y <= 0 {
		return
	}
	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, tag)
	pass.Pop()
}

func settingsPopupPressed(gtx layout.Context, tag event.Tag) bool {
	if tag == nil {
		return false
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: tag,
			Kinds:  pointer.Press,
		})
		if !ok {
			return false
		}
		pe, ok := ev.(pointer.Event)
		if ok && pe.Kind == pointer.Press && pe.Buttons.Contain(pointer.ButtonPrimary) {
			return true
		}
	}
}

func (ui *UI) handleSettingsPopupOutsideClick(gtx layout.Context, st *settingsModalState) {
	if ui == nil || st == nil || !st.anyPopupOpen() {
		return
	}
	pressedPopupToggle := st.popupToggleHovered()
	pressedColorCategoryPopup := settingsPopupPressed(gtx, &st.colorCategoryPopupTag)
	pressedColorPickerPopup := settingsPopupPressed(gtx, &st.colorPickerPopupTag)
	pressedFilenameIconPopup := settingsPopupPressed(gtx, &st.filenameIconPickerPopupTag)
	pressedFilenamePermPopup := settingsPopupPressed(gtx, &st.filenamePermPickerPopupTag)
	pressedTargetPickerPopup := settingsPopupPressed(gtx, &st.viewTargetPickerPopupTag)
	pressedRulePickerPopup := settingsPopupPressed(gtx, &st.viewRulePickerPopupTag)
	pressedAssocPickerPopup := settingsPopupPressed(gtx, &st.viewAssocPickerPopupTag)
	closed := false
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &st.popupGlobalPointerTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok || pe.Kind != pointer.Press || !pe.Buttons.Contain(pointer.ButtonPrimary) {
			continue
		}

		if st.colorCategoryOpen {
			if pressedPopupToggle || pressedColorCategoryPopup {
				continue
			}
			st.closeColorCategoryPopup()
			closed = true
			continue
		}

		if st.colorPickerOpen {
			if pressedPopupToggle || pressedColorPickerPopup {
				continue
			}
			st.colorPickerOpen = false
			st.colorPickerTarget = ""
			st.colorPickerBase = ""
			closed = true
			continue
		}

		if st.filenameIconPickerOpen {
			if pressedPopupToggle || pressedFilenameIconPopup {
				continue
			}
			st.filenameIconPickerOpen = false
			st.filenameIconPickerTarget = ""
			closed = true
			continue
		}

		if st.filenamePermPickerOpen {
			if pressedPopupToggle || pressedFilenamePermPopup {
				continue
			}
			st.filenamePermPickerOpen = false
			closed = true
			continue
		}

		if st.viewTargetPickOpen {
			if pressedPopupToggle || pressedTargetPickerPopup {
				continue
			}
			st.viewTargetPickOpen = false
			closed = true
			continue
		}

		if st.viewRulePickOpen {
			if pressedPopupToggle || pressedRulePickerPopup {
				continue
			}
			st.viewRulePickOpen = false
			closed = true
			continue
		}

		if st.viewAssocPickOpen {
			if pressedPopupToggle || pressedAssocPickerPopup {
				continue
			}
			st.viewAssocPickOpen = false
			closed = true
		}
	}
	if closed {
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) registerSettingsPopupGlobalPointer(gtx layout.Context, st *settingsModalState) {
	if ui == nil || st == nil || !st.anyPopupOpen() {
		return
	}
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &st.popupGlobalPointerTag)
	pass.Pop()
}

func (st *settingsModalState) ensureColorOptionClicks(n int) {
	if n <= cap(st.colorOptionClicks) {
		st.colorOptionClicks = st.colorOptionClicks[:n]
		return
	}
	old := st.colorOptionClicks
	st.colorOptionClicks = make([]widget.Clickable, n)
	copy(st.colorOptionClicks, old)
}

func (st *settingsModalState) ensureColorSwatchClicks(n int) {
	if n <= cap(st.colorSwatchClicks) {
		st.colorSwatchClicks = st.colorSwatchClicks[:n]
		return
	}
	old := st.colorSwatchClicks
	st.colorSwatchClicks = make([]widget.Clickable, n)
	copy(st.colorSwatchClicks, old)
}

func (st *settingsModalState) handleColorPickerActions(gtx layout.Context, groups []settingsColorSwatchGroup) {
	if st == nil || !st.colorPickerOpen {
		return
	}
	clickIdx := 0
	for _, group := range groups {
		for _, hex := range group.hexes {
			if clickIdx >= len(st.colorSwatchClicks) {
				break
			}
			if st.colorSwatchClicks[clickIdx].Clicked(gtx) {
				st.setPopupKeyboardFocus(settingsPopupKeyboardColor, clickIdx, settingsPopupKeyboardActionRow)
				st.colorPickerBase = fm.NormalizeHexColor(hex, fm.DefaultFilePaneSelectionHex)
				st.colorPickerShade.Value = 0.5
				st.errText = ""
			}
			clickIdx++
		}
	}
	if st.colorPickerSetClick.Clicked(gtx) {
		st.setColorPickerHexValue(st.colorPickerTarget, settingsColorShade(st.colorPickerBase, st.colorPickerShade.Value))
		st.colorPickerOpen = false
		st.colorPickerTarget = ""
		st.colorPickerBase = ""
		st.errText = ""
		st.resetPopupKeyboardFocus()
	}
}

func (st *settingsModalState) ensureFilenameIconSwatchClicks(n int) {
	if n <= cap(st.filenameIconSwatchClicks) {
		st.filenameIconSwatchClicks = st.filenameIconSwatchClicks[:n]
		return
	}
	old := st.filenameIconSwatchClicks
	st.filenameIconSwatchClicks = make([]widget.Clickable, n)
	copy(st.filenameIconSwatchClicks, old)
}

func (st *settingsModalState) draftFilePanePalette(cfg *fm.Config) (filePanePalette, string) {
	bgFallback := fm.DefaultFilePaneBackgroundHex
	paneTextFallback := fm.DefaultFilePaneTextHex
	hoverFallback := fm.DefaultFilePaneHoverHex
	hoverTextFallback := fm.DefaultFilePaneHoverTextHex
	popupHoverFallback := fm.DefaultPopupHoverHex
	popupHoverTextFallback := fm.DefaultPopupHoverTextHex
	selectionFallback := fm.DefaultFilePaneSelectionHex
	selectionTextFallback := fm.DefaultFilePaneSelectionTextHex
	selectedFilesFallback := fm.DefaultFilePaneSelectedFilesHex
	selectedFilesTextFallback := fm.DefaultFilePaneSelectedTextHex
	focusedSelectedFallback := fm.DefaultFilePaneFocusedSelectedHex
	focusedSelectedTextFallback := fm.DefaultFilePaneFocusedSelectedTextHex
	currentDirFallback := fm.DefaultCurrentDirBackgroundHex
	currentDirTextFallback := fm.DefaultCurrentDirTextHex
	if cfg != nil {
		bgFallback = cfg.Colors.FilePaneBackground
		paneTextFallback = cfg.Colors.FilePaneText
		hoverFallback = cfg.Colors.Hover
		hoverTextFallback = cfg.Colors.HoverText
		popupHoverFallback = cfg.Colors.PopupHover
		popupHoverTextFallback = cfg.Colors.PopupHoverText
		selectionFallback = cfg.Colors.Selection
		selectionTextFallback = cfg.Colors.SelectionText
		selectedFilesFallback = cfg.Colors.SelectedFiles
		selectedFilesTextFallback = cfg.Colors.SelectedFilesText
		focusedSelectedFallback = cfg.Colors.FocusedSelected
		focusedSelectedTextFallback = cfg.Colors.FocusedSelectedText
		currentDirFallback = cfg.Colors.CurrentDirBg
		currentDirTextFallback = cfg.Colors.CurrentDirText
	}
	bgRaw := strings.TrimSpace(st.colorPaneBackground)
	paneTextRaw := strings.TrimSpace(st.colorPaneText)
	hoverRaw := strings.TrimSpace(st.colorHover)
	hoverTextRaw := strings.TrimSpace(st.colorHoverText)
	popupHoverRaw := strings.TrimSpace(st.colorPopupHover)
	popupHoverTextRaw := strings.TrimSpace(st.colorPopupHoverText)
	selectionRaw := strings.TrimSpace(st.colorSelection)
	selectionTextRaw := strings.TrimSpace(st.colorSelectionText)
	selectedFilesRaw := strings.TrimSpace(st.colorSelectedFiles)
	selectedFilesTextRaw := strings.TrimSpace(st.colorSelectedFilesText)
	focusedSelectedRaw := strings.TrimSpace(st.colorFocusedSelected)
	focusedSelectedTextRaw := strings.TrimSpace(st.colorFocusedSelectedText)
	currentDirRaw := strings.TrimSpace(st.colorCurrentDir)
	currentDirTextRaw := strings.TrimSpace(st.colorCurrentDirText)
	scrollbarThumbRaw := strings.TrimSpace(st.colorScrollbarThumb)
	scrollbarTrackRaw := strings.TrimSpace(st.colorScrollbarTrack)

	errText := ""
	for _, field := range []struct {
		label            string
		value            string
		allowTransparent bool
	}{
		{label: "Pane background", value: bgRaw},
		{label: "Pane text", value: paneTextRaw},
		{label: "Hover background", value: hoverRaw},
		{label: "Hover text", value: hoverTextRaw, allowTransparent: true},
		{label: "Popup hover background", value: popupHoverRaw},
		{label: "Popup hover text", value: popupHoverTextRaw},
		{label: "Focused selection background", value: selectionRaw},
		{label: "Focused selection text", value: selectionTextRaw, allowTransparent: true},
		{label: "Selected files background", value: selectedFilesRaw},
		{label: "Selected files text", value: selectedFilesTextRaw, allowTransparent: true},
		{label: "Focused + selected files background", value: focusedSelectedRaw},
		{label: "Focused + selected files text", value: focusedSelectedTextRaw, allowTransparent: true},
		{label: "Current dir background", value: currentDirRaw},
		{label: "Current dir text", value: currentDirTextRaw},
		{label: "Scrollbar thumb", value: scrollbarThumbRaw},
		{label: "Scrollbar track", value: scrollbarTrackRaw},
	} {
		if field.value == "" {
			continue
		}
		if field.allowTransparent && fm.IsTransparentColor(field.value) {
			continue
		}
		if _, ok := fm.ParseHexColor(field.value); !ok {
			if field.allowTransparent {
				errText = field.label + " must use #RRGGBB or transparent"
			} else {
				errText = field.label + " must use #RRGGBB"
			}
			break
		}
	}

	draft := fm.DefaultConfig()
	draft.Colors.FilePaneBackground = fm.NormalizeHexColor(bgRaw, bgFallback)
	draft.Colors.FilePaneText = fm.NormalizeHexColor(paneTextRaw, paneTextFallback)
	draft.Colors.Hover = fm.NormalizeHexColor(hoverRaw, hoverFallback)
	draft.Colors.HoverText = fm.NormalizeHexOrTransparentColor(hoverTextRaw, hoverTextFallback)
	draft.Colors.PopupHover = fm.NormalizeHexColor(popupHoverRaw, popupHoverFallback)
	draft.Colors.PopupHoverText = fm.NormalizeHexColor(popupHoverTextRaw, popupHoverTextFallback)
	draft.Colors.Selection = fm.NormalizeHexColor(selectionRaw, selectionFallback)
	draft.Colors.SelectionText = fm.NormalizeHexOrTransparentColor(selectionTextRaw, selectionTextFallback)
	draft.Colors.SelectedFiles = fm.NormalizeHexColor(selectedFilesRaw, selectedFilesFallback)
	draft.Colors.SelectedFilesText = fm.NormalizeHexOrTransparentColor(selectedFilesTextRaw, selectedFilesTextFallback)
	draft.Colors.FocusedSelected = fm.NormalizeHexColor(focusedSelectedRaw, focusedSelectedFallback)
	draft.Colors.FocusedSelectedText = fm.NormalizeHexOrTransparentColor(focusedSelectedTextRaw, focusedSelectedTextFallback)
	draft.Colors.CurrentDirBg = fm.NormalizeHexColor(currentDirRaw, currentDirFallback)
	draft.Colors.CurrentDirText = fm.NormalizeHexColor(currentDirTextRaw, currentDirTextFallback)
	draft.Colors.ScrollbarThumb = fm.NormalizeOptionalHexColor(scrollbarThumbRaw)
	draft.Colors.ScrollbarTrack = fm.NormalizeOptionalHexColor(scrollbarTrackRaw)
	return filePanePaletteFromConfig(draft), errText
}

func filePanePaletteToConfigColors(palette filePanePalette) fm.ColorsConfig {
	return fm.ColorsConfig{
		FilePaneBackground:  fm.FormatHexColor(palette.PaneBg),
		FilePaneText:        fm.FormatHexColor(palette.PaneFg),
		Hover:               fm.FormatHexColor(palette.HoverBg),
		HoverText:           formatPaneRowTextConfigColor(palette.HoverFg),
		PopupHover:          fm.FormatHexColor(palette.PopupHoverBg),
		PopupHoverText:      fm.FormatHexColor(palette.PopupHoverFg),
		Selection:           fm.FormatHexColor(palette.SelectedBg),
		SelectionText:       formatPaneRowTextConfigColor(palette.SelectedFg),
		SelectedFiles:       fm.FormatHexColor(palette.MarkedBg),
		SelectedFilesText:   formatPaneRowTextConfigColor(palette.MarkedFg),
		FocusedSelected:     fm.FormatHexColor(palette.MarkedSelBg),
		FocusedSelectedText: formatPaneRowTextConfigColor(palette.MarkedSelFg),
		CurrentDirBg:        fm.FormatHexColor(palette.CurrentDirBg),
		CurrentDirText:      fm.FormatHexColor(palette.CurrentDirFg),
		ScrollbarThumb:      fm.FormatHexColor(palette.ScrollThumb),
		ScrollbarTrack:      fm.FormatHexColor(palette.ScrollTrack),
	}
}

func formatPaneRowTextConfigColor(c color.NRGBA) string {
	if c.A == 0 {
		return fm.TransparentColor
	}
	return fm.FormatHexColor(c)
}

func (st *settingsModalState) draftViewerTheme(cfg *fm.Config) (fileViewerTheme, string) {
	palette, errText := st.draftFilePanePalette(cfg)
	draft := fm.DefaultConfig()
	if cfg != nil {
		draft.General = cfg.General
		draft.Viewer = cfg.Viewer
	}
	draft.Colors = filePanePaletteToConfigColors(palette)

	viewBgFallback := fm.DefaultViewerBackgroundHex
	viewTextFallback := fm.DefaultViewerTextHex
	viewSelectionFallback := fm.DefaultViewerSelectionHex
	if cfg != nil {
		viewBgFallback = cfg.Viewer.Background
		viewTextFallback = cfg.Viewer.Text
		viewSelectionFallback = cfg.Viewer.Selection
	}
	viewBg := strings.TrimSpace(st.colorViewerBackground)
	if viewBg != "" {
		if _, ok := fm.ParseHexColor(viewBg); !ok && errText == "" {
			errText = "Viewer background must use #RRGGBB"
		}
	}
	viewText := strings.TrimSpace(st.colorViewerText)
	if viewText != "" {
		if _, ok := fm.ParseHexColor(viewText); !ok && errText == "" {
			errText = "Viewer text must use #RRGGBB"
		}
	}
	viewSelection := strings.TrimSpace(st.colorViewerSelection)
	if viewSelection != "" {
		if _, ok := fm.ParseHexColor(viewSelection); !ok && errText == "" {
			errText = "Viewer selection must use #RRGGBB"
		}
	}
	hexColors := []struct {
		label string
		value string
	}{
		{label: "Hex selection", value: strings.TrimSpace(st.colorViewerHexSelection)},
		{label: "Hex offset text", value: strings.TrimSpace(st.colorViewerHexOffsetText)},
		{label: "Hex bytes text", value: strings.TrimSpace(st.colorViewerHexBytesText)},
		{label: "Hex ASCII text", value: strings.TrimSpace(st.colorViewerHexASCIIText)},
	}
	for _, field := range hexColors {
		if field.value == "" {
			continue
		}
		if _, ok := fm.ParseHexColor(field.value); !ok && errText == "" {
			errText = field.label + " must use #RRGGBB"
		}
	}
	draft.Viewer.Background = fm.NormalizeHexColor(viewBg, viewBgFallback)
	draft.Viewer.Text = fm.NormalizeHexColor(viewText, viewTextFallback)
	draft.Viewer.Selection = fm.NormalizeHexColor(viewSelection, viewSelectionFallback)
	draft.Viewer.HexSelection = fm.NormalizeOptionalHexColor(st.colorViewerHexSelection)
	draft.Viewer.HexOffsetText = fm.NormalizeOptionalHexColor(st.colorViewerHexOffsetText)
	draft.Viewer.HexBytesText = fm.NormalizeOptionalHexColor(st.colorViewerHexBytesText)
	draft.Viewer.HexASCIIText = fm.NormalizeOptionalHexColor(st.colorViewerHexASCIIText)
	return fileViewerThemeFromConfig(draft), errText
}

func viewerCommandTargetEntries(raw map[string]string) []viewerCommandTargetEntry {
	if len(raw) == 0 {
		return nil
	}
	keys := make([]string, 0, len(raw))
	for rawKey, rawCommand := range raw {
		key := strings.TrimSpace(rawKey)
		command := strings.TrimSpace(rawCommand)
		if key == "" || command == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	out := make([]viewerCommandTargetEntry, 0, len(keys))
	for _, key := range keys {
		command := strings.TrimSpace(raw[key])
		if command == "" {
			continue
		}
		out = append(out, viewerCommandTargetEntry{Key: key, Command: command})
	}
	return out
}

func viewerCommandTargetMap(raw []viewerCommandTargetEntry) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]string, len(raw))
	for _, item := range raw {
		key := normalizeViewerCommandTargetInput(item.Key)
		command := strings.TrimSpace(item.Command)
		if key == "" || command == "" {
			continue
		}
		out[key] = command
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeViewerCommandTargetInput(raw string) string {
	target := strings.TrimSpace(raw)
	if target == "" {
		return ""
	}
	lower := strings.ToLower(target)
	switch {
	case strings.HasPrefix(lower, "local:"):
		localPath := strings.TrimSpace(target[len("local:"):])
		if localPath == "" {
			return ""
		}
		localPath = filepath.Clean(localPath)
		if runtime.GOOS == "windows" {
			localPath = strings.ToLower(localPath)
		}
		return "local:" + localPath
	case strings.HasPrefix(lower, "ssh:"):
		return target
	case filepath.IsAbs(target):
		return viewerCommandTargetKey(target, nil)
	default:
		return target
	}
}

func viewerCommandTargetDisplayKey(raw string) string {
	key := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(strings.ToLower(key), "local:"):
		return strings.TrimSpace(key[len("local:"):])
	default:
		return key
	}
}

func viewerCommandTargetRowTitle(raw string) string {
	key := strings.TrimSpace(raw)
	if key == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(key), "local:") {
		label := strings.TrimSpace(key[len("local:"):])
		if base := filepath.Base(label); base != "" && base != "." && base != string(filepath.Separator) {
			return base
		}
		return label
	}
	if strings.HasPrefix(strings.ToLower(key), "ssh:") {
		trimmed := strings.TrimSpace(key[len("ssh:"):])
		if idx := strings.LastIndex(trimmed, ":"); idx >= 0 && idx+1 < len(trimmed) {
			remotePath := trimmed[idx+1:]
			if base := path.Base(remotePath); base != "" && base != "." && base != "/" {
				return base
			}
		}
		return trimmed
	}
	return key
}

func (st *settingsModalState) viewerCommandTargetRowClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.viewTargetRowClicks == nil {
		st.viewTargetRowClicks = make(map[string]*widget.Clickable)
	}
	if click := st.viewTargetRowClicks[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.viewTargetRowClicks[key] = click
	return click
}

func (st *settingsModalState) viewerCommandTargetRowRemoveClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.viewTargetRowRemoveClicks == nil {
		st.viewTargetRowRemoveClicks = make(map[string]*widget.Clickable)
	}
	if click := st.viewTargetRowRemoveClicks[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.viewTargetRowRemoveClicks[key] = click
	return click
}

func (st *settingsModalState) viewerCommandTargetIndex(key string) int {
	if st == nil || key == "" {
		return -1
	}
	for i, entry := range st.viewTargetEntries {
		if entry.Key == key {
			return i
		}
	}
	return -1
}

func (st *settingsModalState) viewerCommandTarget(key string) (viewerCommandTargetEntry, bool) {
	if idx := st.viewerCommandTargetIndex(key); idx >= 0 {
		return st.viewTargetEntries[idx], true
	}
	return viewerCommandTargetEntry{}, false
}

func (st *settingsModalState) viewerSavedCommandTarget(key string) (viewerCommandTargetEntry, bool) {
	if st == nil || key == "" {
		return viewerCommandTargetEntry{}, false
	}
	for _, entry := range st.viewTargetSavedEntries {
		if entry.Key == key {
			return entry, true
		}
	}
	return viewerCommandTargetEntry{}, false
}

func (st *settingsModalState) loadViewerCommandTargetFields(key, command string) {
	if st == nil {
		return
	}
	key = normalizeViewerCommandTargetInput(key)
	st.viewTargetKeyEdit.SetText(key)
	st.viewTargetCommandEdit.SetText(strings.TrimSpace(command))
	st.viewTargetLookupKey = key
	st.viewTargetEditingKey = ""
	if key != "" && strings.TrimSpace(command) != "" {
		st.viewTargetEditingKey = key
	}
}

func (st *settingsModalState) applyPickedViewerCommandTarget(entry viewerCommandTargetEntry) {
	if st == nil {
		return
	}
	st.loadViewerCommandTargetFields(entry.Key, entry.Command)
	st.viewTargetPickOpen = false
	st.resetPopupKeyboardFocus()
	st.errText = ""
	st.targetInfoText = ""
}

func (st *settingsModalState) refreshViewerCommandTargetDraftInfo(autoApplyExisting bool) {
	if st == nil {
		return
	}
	_ = autoApplyExisting
	key := normalizeViewerCommandTargetInput(st.viewTargetKeyEdit.Text())
	command := strings.TrimSpace(st.viewTargetCommandEdit.Text())
	st.targetInfoText = ""
	if key == "" || command == "" {
		return
	}
	existingKey := key
	if st.viewTargetEditingKey != "" {
		existingKey = st.viewTargetEditingKey
	}
	existing, ok := st.viewerCommandTarget(existingKey)
	if !ok {
		st.targetInfoText = "Click Add"
		return
	}
	if existingKey == key && existing.Command == command {
		return
	}
	st.targetInfoText = "Click Update"
}

func (st *settingsModalState) viewerCommandTargetNoticeText() string {
	if st == nil {
		return ""
	}
	key := normalizeViewerCommandTargetInput(st.viewTargetKeyEdit.Text())
	if key == "" {
		return ""
	}
	command := strings.TrimSpace(st.viewTargetCommandEdit.Text())
	if st.viewTargetEditingKey != "" {
		if editingEntry, ok := st.viewerCommandTarget(st.viewTargetEditingKey); ok &&
			(st.viewTargetEditingKey != key || editingEntry.Command != command) {
			return "Click Update"
		}
	}
	savedEntry, savedExists := st.viewerSavedCommandTarget(key)
	currentEntry, currentExists := st.viewerCommandTarget(key)
	switch {
	case savedExists && !currentExists:
		return "Pending removal; Save to persist"
	case !currentExists && command != "":
		return "Click Add"
	case currentExists && command != "" && command != currentEntry.Command:
		return "Click Update"
	case savedExists && currentExists && currentEntry.Command != savedEntry.Command:
		return "Pending change; Save to persist"
	case !savedExists && currentExists:
		return "Pending add; Save to persist"
	}
	return ""
}

func viewerRemoteSearchCommandNoticeText() string {
	return strings.Join([]string{
		"Used by SSH hex remote search.",
		`Return a byte offset relative to {range_start}; use "off" to disable.`,
		"Placeholders: {path} {pattern} {pattern_hex} {pattern_base64}",
		"{range_start_1based} {range_len} {direction} {match_limit} {result_select}",
	}, "\n")
}

func (st *settingsModalState) syncViewerCommandTargetEditors() {
	if st == nil {
		return
	}
	key := normalizeViewerCommandTargetInput(st.viewTargetKeyEdit.Text())
	if key == st.viewTargetLookupKey {
		return
	}
	st.viewTargetLookupKey = key
	if st.viewTargetEditingKey != "" {
		return
	}
	if entry, ok := st.viewerCommandTarget(key); ok {
		st.loadViewerCommandTargetFields(entry.Key, entry.Command)
		return
	}
	if strings.TrimSpace(st.viewTargetCommandEdit.Text()) == "" {
		st.viewTargetCommandEdit.SetText("")
	}
}

func (st *settingsModalState) upsertCurrentViewerCommandTarget() (string, error) {
	if st == nil {
		return "Add", nil
	}
	entry, err := parseViewerCommandTargetFields(st.viewTargetKeyEdit.Text(), st.viewTargetCommandEdit.Text())
	if err != nil {
		return "Add", err
	}
	action := "Add"
	oldIdx := st.viewerCommandTargetIndex(st.viewTargetEditingKey)
	newIdx := st.viewerCommandTargetIndex(entry.Key)
	if oldIdx >= 0 {
		if newIdx >= 0 && newIdx != oldIdx {
			return "Update", fmt.Errorf("a command target for %s already exists", entry.Key)
		}
		st.viewTargetEntries[oldIdx] = entry
		action = "Update"
	} else if newIdx >= 0 {
		st.viewTargetEntries[newIdx] = entry
		action = "Update"
	} else {
		st.viewTargetEntries = append(st.viewTargetEntries, entry)
	}
	st.viewTargetEntries = viewerCommandTargetEntries(viewerCommandTargetMap(st.viewTargetEntries))
	st.loadViewerCommandTargetFields(entry.Key, entry.Command)
	st.viewTargetEditingKey = ""
	return action, nil
}

func (st *settingsModalState) removeCurrentViewerCommandTarget() bool {
	if st == nil {
		return false
	}
	key := st.viewTargetEditingKey
	if key == "" {
		key = normalizeViewerCommandTargetInput(st.viewTargetKeyEdit.Text())
	}
	return st.removeViewerCommandTarget(key)
}

func (st *settingsModalState) removeViewerCommandTarget(key string) bool {
	if st == nil || key == "" {
		return false
	}
	idx := st.viewerCommandTargetIndex(key)
	if idx < 0 {
		return false
	}
	st.viewTargetEntries = append(st.viewTargetEntries[:idx], st.viewTargetEntries[idx+1:]...)
	return true
}

func (st *settingsModalState) viewerCommandTargetPickerEntries() ([]viewerCommandTargetEntry, int) {
	if st == nil || len(st.viewTargetEntries) == 0 {
		return nil, 0
	}
	filter := strings.ToLower(strings.TrimSpace(st.viewTargetKeyEdit.Text()))
	if filter == "" {
		return append([]viewerCommandTargetEntry(nil), st.viewTargetEntries...), 0
	}
	matches := make([]viewerCommandTargetEntry, 0, len(st.viewTargetEntries))
	for _, entry := range st.viewTargetEntries {
		lowerKey := strings.ToLower(entry.Key)
		lowerCommand := strings.ToLower(entry.Command)
		if strings.Contains(lowerKey, filter) || strings.Contains(lowerCommand, filter) {
			matches = append(matches, entry)
		}
	}
	if len(matches) == 0 {
		return append([]viewerCommandTargetEntry(nil), st.viewTargetEntries...), 0
	}
	return matches, len(matches)
}

func (st *settingsModalState) openViewerCommandTargetPicker() {
	if st == nil {
		return
	}
	st.viewTargetPickOpen = true
	st.viewTargetRowClicks = nil
	st.viewTargetRowRemoveClicks = nil
	st.resetPopupKeyboardFocus()
}

func (st *settingsModalState) toggleViewerCommandTargetPicker() {
	if st == nil {
		return
	}
	if st.viewTargetPickOpen {
		st.closeSettingsPopupsExcept("")
		return
	}
	st.closeSettingsPopupsExcept("target-picker")
	st.openViewerCommandTargetPicker()
}

func (st *settingsModalState) viewerCommandRuleRowClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.viewRuleRowClicks == nil {
		st.viewRuleRowClicks = make(map[string]*widget.Clickable)
	}
	if click := st.viewRuleRowClicks[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.viewRuleRowClicks[key] = click
	return click
}

func (st *settingsModalState) viewerCommandRuleRowRemoveClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.viewRuleRowRemoveClicks == nil {
		st.viewRuleRowRemoveClicks = make(map[string]*widget.Clickable)
	}
	if click := st.viewRuleRowRemoveClicks[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.viewRuleRowRemoveClicks[key] = click
	return click
}

func (st *settingsModalState) viewerCommandRuleIndex(pattern string) int {
	if st == nil || pattern == "" {
		return -1
	}
	for i, rule := range st.viewRuleEntries {
		if rule.Pattern == pattern {
			return i
		}
	}
	return -1
}

func (st *settingsModalState) viewerCommandRule(pattern string) (fm.ViewerCommandRule, bool) {
	if idx := st.viewerCommandRuleIndex(pattern); idx >= 0 {
		return st.viewRuleEntries[idx], true
	}
	return fm.ViewerCommandRule{}, false
}

func (st *settingsModalState) viewerSavedCommandRule(pattern string) (fm.ViewerCommandRule, bool) {
	if st == nil || pattern == "" {
		return fm.ViewerCommandRule{}, false
	}
	for _, rule := range st.viewRuleSavedEntries {
		if rule.Pattern == pattern {
			return rule, true
		}
	}
	return fm.ViewerCommandRule{}, false
}

func (st *settingsModalState) loadViewerCommandRuleFields(pattern, command string) {
	if st == nil {
		return
	}
	st.viewRulePatternEdit.SetText(strings.TrimSpace(pattern))
	st.viewRuleCommandEdit.SetText(strings.TrimSpace(command))
	st.viewRuleLookupPattern = strings.TrimSpace(pattern)
	st.viewRuleEditingPattern = ""
	if st.viewRuleLookupPattern != "" && strings.TrimSpace(command) != "" {
		st.viewRuleEditingPattern = st.viewRuleLookupPattern
	}
}

func (st *settingsModalState) applyPickedViewerCommandRule(rule fm.ViewerCommandRule) {
	if st == nil {
		return
	}
	st.loadViewerCommandRuleFields(rule.Pattern, rule.Command)
	st.viewRulePickOpen = false
	st.resetPopupKeyboardFocus()
	st.errText = ""
	st.ruleInfoText = ""
}

func (st *settingsModalState) refreshViewerCommandRuleDraftInfo(autoApplyExisting bool) {
	if st == nil {
		return
	}
	_ = autoApplyExisting
	pattern := strings.TrimSpace(st.viewRulePatternEdit.Text())
	command := strings.TrimSpace(st.viewRuleCommandEdit.Text())
	st.ruleInfoText = ""
	if pattern == "" || command == "" {
		return
	}
	existingPattern := pattern
	if st.viewRuleEditingPattern != "" {
		existingPattern = st.viewRuleEditingPattern
	}
	existing, ok := st.viewerCommandRule(existingPattern)
	if !ok {
		st.ruleInfoText = "Click Add"
		return
	}
	if existingPattern == pattern && existing.Command == command {
		return
	}
	st.ruleInfoText = "Click Update"
}

func (st *settingsModalState) viewerCommandRuleNoticeText() string {
	if st == nil {
		return ""
	}
	pattern := strings.TrimSpace(st.viewRulePatternEdit.Text())
	if pattern == "" {
		return ""
	}
	command := strings.TrimSpace(st.viewRuleCommandEdit.Text())
	if st.viewRuleEditingPattern != "" {
		if editingRule, ok := st.viewerCommandRule(st.viewRuleEditingPattern); ok &&
			(st.viewRuleEditingPattern != pattern || editingRule.Command != command) {
			return "Click Update"
		}
	}
	savedRule, savedExists := st.viewerSavedCommandRule(pattern)
	currentRule, currentExists := st.viewerCommandRule(pattern)
	switch {
	case savedExists && !currentExists:
		return "Pending removal; Save to persist"
	case !currentExists && command != "":
		return "Click Add"
	case currentExists && command != "" && command != currentRule.Command:
		return "Click Update"
	case savedExists && currentExists && currentRule.Command != savedRule.Command:
		return "Pending change; Save to persist"
	case !savedExists && currentExists:
		return "Pending add; Save to persist"
	}
	return ""
}

func (st *settingsModalState) syncViewerCommandRuleEditors() {
	if st == nil {
		return
	}
	pattern := strings.TrimSpace(st.viewRulePatternEdit.Text())
	if pattern == st.viewRuleLookupPattern {
		return
	}
	st.viewRuleLookupPattern = pattern
	if st.viewRuleEditingPattern != "" {
		return
	}
	if rule, ok := st.viewerCommandRule(pattern); ok {
		st.loadViewerCommandRuleFields(rule.Pattern, rule.Command)
		return
	}
	if strings.TrimSpace(st.viewRuleCommandEdit.Text()) == "" {
		st.viewRuleCommandEdit.SetText("")
	}
}

func (st *settingsModalState) upsertCurrentViewerCommandRule() (string, error) {
	if st == nil {
		return "Add", nil
	}
	rule, err := parseViewerCommandRuleFields(st.viewRulePatternEdit.Text(), st.viewRuleCommandEdit.Text())
	if err != nil {
		return "Add", err
	}
	action := "Add"
	oldIdx := st.viewerCommandRuleIndex(st.viewRuleEditingPattern)
	newIdx := st.viewerCommandRuleIndex(rule.Pattern)
	if oldIdx >= 0 {
		if newIdx >= 0 && newIdx != oldIdx {
			return "Update", fmt.Errorf("a command rule for %q already exists", rule.Pattern)
		}
		st.viewRuleEntries[oldIdx] = rule
		action = "Update"
	} else if newIdx >= 0 {
		st.viewRuleEntries[newIdx] = rule
		action = "Update"
	} else {
		st.viewRuleEntries = append(st.viewRuleEntries, rule)
	}
	st.viewRuleEntries = fm.NormalizeViewerCommandRules(st.viewRuleEntries)
	st.loadViewerCommandRuleFields(rule.Pattern, rule.Command)
	st.viewRuleEditingPattern = ""
	return action, nil
}

func (st *settingsModalState) removeCurrentViewerCommandRule() bool {
	if st == nil {
		return false
	}
	pattern := st.viewRuleEditingPattern
	if pattern == "" {
		pattern = strings.TrimSpace(st.viewRulePatternEdit.Text())
	}
	return st.removeViewerCommandRule(pattern)
}

func (st *settingsModalState) removeViewerCommandRule(pattern string) bool {
	if st == nil || pattern == "" {
		return false
	}
	idx := st.viewerCommandRuleIndex(pattern)
	if idx < 0 {
		return false
	}
	st.viewRuleEntries = append(st.viewRuleEntries[:idx], st.viewRuleEntries[idx+1:]...)
	return true
}

func (st *settingsModalState) viewerCommandRulePickerRules() ([]fm.ViewerCommandRule, int) {
	if st == nil || len(st.viewRuleEntries) == 0 {
		return nil, 0
	}
	filter := strings.ToLower(strings.TrimSpace(st.viewRulePatternEdit.Text()))
	if filter == "" {
		return append([]fm.ViewerCommandRule(nil), st.viewRuleEntries...), 0
	}
	matches := make([]fm.ViewerCommandRule, 0, len(st.viewRuleEntries))
	for _, rule := range st.viewRuleEntries {
		lowerPattern := strings.ToLower(rule.Pattern)
		lowerCommand := strings.ToLower(rule.Command)
		if strings.Contains(lowerPattern, filter) || strings.Contains(lowerCommand, filter) {
			matches = append(matches, rule)
		}
	}
	if len(matches) == 0 {
		return append([]fm.ViewerCommandRule(nil), st.viewRuleEntries...), 0
	}
	return matches, len(matches)
}

func (st *settingsModalState) openViewerCommandRulePicker() {
	if st == nil {
		return
	}
	st.viewRulePickOpen = true
	st.viewRuleRowClicks = nil
	st.viewRuleRowRemoveClicks = nil
	st.resetPopupKeyboardFocus()
}

func (st *settingsModalState) toggleViewerCommandRulePicker() {
	if st == nil {
		return
	}
	if st.viewRulePickOpen {
		st.closeSettingsPopupsExcept("")
		return
	}
	st.closeSettingsPopupsExcept("rule-picker")
	st.openViewerCommandRulePicker()
}

func (st *settingsModalState) viewerAssocRowClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.viewAssocRowClicks == nil {
		st.viewAssocRowClicks = make(map[string]*widget.Clickable)
	}
	if click := st.viewAssocRowClicks[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.viewAssocRowClicks[key] = click
	return click
}

func (st *settingsModalState) viewerAssociationIndex(ext string) int {
	if st == nil || ext == "" {
		return -1
	}
	for i, assoc := range st.viewAssocEntries {
		if assoc.Extension == ext {
			return i
		}
	}
	return -1
}

func (st *settingsModalState) viewerAssociation(ext string) (fm.ViewerAssociation, bool) {
	if idx := st.viewerAssociationIndex(ext); idx >= 0 {
		return st.viewAssocEntries[idx], true
	}
	return fm.ViewerAssociation{}, false
}

func (st *settingsModalState) viewerSavedAssociation(ext string) (fm.ViewerAssociation, bool) {
	if st == nil || ext == "" {
		return fm.ViewerAssociation{}, false
	}
	for _, assoc := range st.viewAssocSavedEntries {
		if assoc.Extension == ext {
			return assoc, true
		}
	}
	return fm.ViewerAssociation{}, false
}

func (st *settingsModalState) loadViewerAssociationFields(ext, app string) {
	if st == nil {
		return
	}
	st.viewAssocExtEdit.SetText(viewerAssociationDisplayExtension(ext))
	st.viewAssocAppEdit.SetText(app)
	st.viewAssocLookupExt = fm.NormalizeViewerAssociationExtension(ext)
	st.viewAssocEditingExt = ""
	if st.viewAssocLookupExt != "" && fm.NormalizeViewerAssociationAppPath(app) != "" {
		st.viewAssocEditingExt = st.viewAssocLookupExt
	}
}

func (st *settingsModalState) applyPickedViewerAssociation(appPath string) {
	if st == nil {
		return
	}
	targetExt := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
	if targetExt == "" {
		st.viewAssocAppEdit.SetText(appPath)
		st.viewAssocLookupExt = ""
		st.viewAssocPickOpen = false
		st.resetPopupKeyboardFocus()
		st.errText = ""
		st.assocInfoText = ""
		return
	}
	st.viewAssocExtEdit.SetText(viewerAssociationDisplayExtension(targetExt))
	st.viewAssocAppEdit.SetText(appPath)
	st.viewAssocLookupExt = targetExt
	st.viewAssocPickOpen = false
	st.resetPopupKeyboardFocus()
	st.errText = ""
	st.assocInfoText = ""
}

func (st *settingsModalState) refreshViewerAssociationDraftInfo(autoApplyExisting bool) {
	if st == nil {
		return
	}
	_ = autoApplyExisting
	ext := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
	app := fm.NormalizeViewerAssociationAppPath(st.viewAssocAppEdit.Text())
	st.assocInfoText = ""
	if ext == "" || app == "" {
		return
	}
	existingExt := ext
	if st.viewAssocEditingExt != "" {
		existingExt = st.viewAssocEditingExt
	}
	existing, ok := st.viewerAssociation(existingExt)
	if !ok {
		st.assocInfoText = "Click Add"
		return
	}
	if existingExt == ext && existing.AppPath == app {
		return
	}
	st.assocInfoText = "Click Update"
}

func (st *settingsModalState) viewerAssociationNoticeText() string {
	if st == nil {
		return ""
	}
	ext := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
	if ext == "" {
		return ""
	}
	app := fm.NormalizeViewerAssociationAppPath(st.viewAssocAppEdit.Text())
	if st.viewAssocEditingExt != "" {
		if editingAssoc, ok := st.viewerAssociation(st.viewAssocEditingExt); ok &&
			(st.viewAssocEditingExt != ext || editingAssoc.AppPath != app) {
			return "Click Update"
		}
	}
	savedAssoc, savedExists := st.viewerSavedAssociation(ext)
	currentAssoc, currentExists := st.viewerAssociation(ext)
	switch {
	case savedExists && !currentExists:
		return "Pending removal; Save to persist"
	case !currentExists && app != "":
		return "Click Add"
	case currentExists && app != "" && app != currentAssoc.AppPath:
		return "Click Update"
	case savedExists && currentExists && currentAssoc.AppPath != savedAssoc.AppPath:
		return "Pending change; Save to persist"
	case !savedExists && currentExists:
		return "Pending add; Save to persist"
	}
	return ""
}

func (st *settingsModalState) syncViewerAssociationEditors() {
	if st == nil {
		return
	}
	ext := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
	if ext == st.viewAssocLookupExt {
		return
	}
	st.viewAssocLookupExt = ext
	if st.viewAssocEditingExt != "" {
		return
	}
	if assoc, ok := st.viewerAssociation(ext); ok {
		st.loadViewerAssociationFields(assoc.Extension, assoc.AppPath)
		return
	}
	if strings.TrimSpace(st.viewAssocAppEdit.Text()) == "" {
		st.viewAssocAppEdit.SetText("")
	}
}

func (st *settingsModalState) upsertCurrentViewerAssociation() (string, error) {
	if st == nil {
		return "Add", nil
	}
	assoc, err := parseViewerAssociationFields(st.viewAssocExtEdit.Text(), st.viewAssocAppEdit.Text())
	if err != nil {
		return "Add", err
	}
	action := "Add"
	oldIdx := st.viewerAssociationIndex(st.viewAssocEditingExt)
	newIdx := st.viewerAssociationIndex(assoc.Extension)
	if oldIdx >= 0 {
		if newIdx >= 0 && newIdx != oldIdx {
			return "Update", fmt.Errorf("an association for %s already exists", assoc.Extension)
		}
		st.viewAssocEntries[oldIdx] = assoc
		action = "Update"
	} else if newIdx >= 0 {
		st.viewAssocEntries[newIdx] = assoc
		action = "Update"
	} else {
		st.viewAssocEntries = append(st.viewAssocEntries, assoc)
	}
	st.viewAssocEntries = fm.NormalizeViewerAssociations(st.viewAssocEntries)
	st.viewAssocExtEdit.SetText(viewerAssociationDisplayExtension(assoc.Extension))
	st.viewAssocAppEdit.SetText(assoc.AppPath)
	st.viewAssocLookupExt = assoc.Extension
	st.viewAssocEditingExt = ""
	return action, nil
}

func (st *settingsModalState) removeCurrentViewerAssociation() bool {
	if st == nil {
		return false
	}
	ext := st.viewAssocEditingExt
	if ext == "" {
		ext = fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
	}
	idx := st.viewerAssociationIndex(ext)
	if idx < 0 {
		return false
	}
	st.viewAssocEntries = append(st.viewAssocEntries[:idx], st.viewAssocEntries[idx+1:]...)
	st.viewAssocExtEdit.SetText(viewerAssociationDisplayExtension(ext))
	st.viewAssocAppEdit.SetText("")
	st.viewAssocLookupExt = ext
	st.viewAssocEditingExt = ""
	return true
}

func (st *settingsModalState) viewerAssociationPickerPrograms() ([]viewerAssociationProgram, int) {
	if st == nil || len(st.viewAssocEntries) == 0 {
		return nil, 0
	}
	filter := strings.ToLower(strings.TrimSpace(st.viewAssocExtEdit.Text()))
	filter = strings.TrimLeft(filter, ".")
	byApp := make(map[string]*viewerAssociationProgram, len(st.viewAssocEntries))
	for _, assoc := range st.viewAssocEntries {
		app := fm.NormalizeViewerAssociationAppPath(assoc.AppPath)
		if app == "" {
			continue
		}
		group := byApp[app]
		if group == nil {
			group = &viewerAssociationProgram{AppPath: app}
			byApp[app] = group
		}
		dispExt := viewerAssociationDisplayExtension(assoc.Extension)
		group.Extensions = append(group.Extensions, dispExt)
		if filter == "" {
			continue
		}
		lowerExt := strings.ToLower(dispExt)
		rank := 0
		switch {
		case lowerExt == filter:
			rank = 3
		case strings.HasPrefix(lowerExt, filter):
			rank = 2
		case strings.Contains(lowerExt, filter):
			rank = 1
		}
		if rank > group.MatchRank {
			group.MatchRank = rank
		}
	}
	if len(byApp) == 0 {
		return nil, 0
	}
	out := make([]viewerAssociationProgram, 0, len(byApp))
	matchCount := 0
	for _, group := range byApp {
		sort.Strings(group.Extensions)
		if group.MatchRank > 0 {
			matchCount++
		}
		out = append(out, *group)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MatchRank != out[j].MatchRank {
			return out[i].MatchRank > out[j].MatchRank
		}
		iBase := strings.ToLower(filepath.Base(out[i].AppPath))
		jBase := strings.ToLower(filepath.Base(out[j].AppPath))
		if iBase != jBase {
			return iBase < jBase
		}
		return strings.ToLower(out[i].AppPath) < strings.ToLower(out[j].AppPath)
	})
	return out, matchCount
}

func (st *settingsModalState) openViewerAssociationPicker() {
	if st == nil {
		return
	}
	st.viewAssocPickOpen = true
	// Recreate row clickables on every open so stale click state from a prior
	// picker session cannot fire against a different visible row later.
	st.viewAssocRowClicks = nil
	st.resetPopupKeyboardFocus()
}

func (st *settingsModalState) toggleViewerAssociationPicker() {
	if st == nil {
		return
	}
	if st.viewAssocPickOpen {
		st.closeSettingsPopupsExcept("")
		return
	}
	st.closeSettingsPopupsExcept("assoc-picker")
	st.openViewerAssociationPicker()
}

func (st *settingsModalState) setActiveTab(next string, now time.Time) {
	if st == nil || next == "" || st.activeTab == next {
		return
	}
	st.navPrevTab = st.activeTab
	st.navAnimAt = now
	st.activeTab = next
	st.closeSettingsPopupsExcept("")
}

func settingsTabIndex(key string) int {
	for i, candidate := range settingsTabOrder {
		if candidate == key {
			return i
		}
	}
	return 0
}

func settingsShiftTab(key string, step int) string {
	if len(settingsTabOrder) == 0 {
		return ""
	}
	if step == 0 {
		return key
	}
	idx := settingsTabIndex(key)
	idx = (idx + step) % len(settingsTabOrder)
	if idx < 0 {
		idx += len(settingsTabOrder)
	}
	return settingsTabOrder[idx]
}

func (st *settingsModalState) tabPosition(now time.Time) (float32, bool) {
	if st == nil {
		return 0, false
	}
	current := float32(settingsTabIndex(st.activeTab))
	if st.navPrevTab == "" || st.navAnimAt.IsZero() || st.navPrevTab == st.activeTab {
		return current, false
	}
	elapsed := now.Sub(st.navAnimAt)
	if elapsed >= toolbarAnimDur {
		st.navPrevTab = ""
		st.navAnimAt = time.Time{}
		return current, false
	}
	prev := float32(settingsTabIndex(st.navPrevTab))
	t := smoothstep01(clamp01(float32(elapsed) / float32(toolbarAnimDur)))
	return prev + (current-prev)*t, true
}

func (st *settingsModalState) stepActiveTab(step int, now time.Time) bool {
	if st == nil {
		return false
	}
	next := settingsShiftTab(st.activeTab, step)
	if next == "" || next == st.activeTab {
		return false
	}
	st.setActiveTab(next, now)
	st.setPulse(next, now)
	return true
}

func (st *settingsModalState) hasFocusedEditor(gtx layout.Context) bool {
	if st == nil {
		return false
	}
	return gtx.Focused(&st.colorValueEdit) ||
		gtx.Focused(&st.colorTextValueEdit) ||
		gtx.Focused(&st.viewCommandEdit) ||
		gtx.Focused(&st.viewShellEdit) ||
		gtx.Focused(&st.viewRemoteSearchCommandEdit) ||
		gtx.Focused(&st.viewTargetKeyEdit) ||
		gtx.Focused(&st.viewTargetCommandEdit) ||
		gtx.Focused(&st.viewRulePatternEdit) ||
		gtx.Focused(&st.viewRuleCommandEdit) ||
		gtx.Focused(&st.viewAssocExtEdit) ||
		gtx.Focused(&st.viewAssocAppEdit) ||
		gtx.Focused(&st.filenameDefaultTextEdit) ||
		gtx.Focused(&st.filenameAgeOffsetEdit) ||
		gtx.Focused(&st.filenameAgeTextEdit) ||
		gtx.Focused(&st.filenamePermEdit) ||
		gtx.Focused(&st.filenamePermTextEdit) ||
		gtx.Focused(&st.filenameExtEdit) ||
		gtx.Focused(&st.filenameExtTextEdit) ||
		gtx.Focused(&st.filenameSizeEdit) ||
		gtx.Focused(&st.filenameSizeTextEdit) ||
		gtx.Focused(&st.configPathSelect) ||
		gtx.Focused(&st.configEdit)
}

func settingsViewerRowLabel(ui *UI, th *material.Theme, txt string, enabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lbl := material.Caption(th, txt)
		lbl.Font.Typeface = ui.interfaceTypeface()
		lbl.TextSize = ui.scaleModalFontSize(9)
		lbl.Color = hintColor
		if !enabled {
			lbl.Color = color.NRGBA{R: 102, G: 102, B: 102, A: 255}
		}
		return lbl.Layout(gtx)
	}
}

func settingsScrollableListStyle(th *material.Theme, list *widget.List) material.ListStyle {
	style := material.List(th, list)
	style.AnchorStrategy = material.Occupy
	style.ScrollbarStyle = settingsScrollbarStyle(th, &list.Scrollbar)
	return style
}

func settingsPopupListStyle(th *material.Theme, list *widget.List) material.ListStyle {
	style := material.List(th, list)
	style.AnchorStrategy = material.Overlay
	style.ScrollbarStyle = settingsScrollbarStyle(th, &list.Scrollbar)
	return style
}

func settingsScrollbarStyle(th *material.Theme, state *widget.Scrollbar) material.ScrollbarStyle {
	style := material.Scrollbar(th, state)
	style.Track.MajorPadding = unit.Dp(1)
	style.Track.MinorPadding = unit.Dp(1)
	style.Track.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 10}
	style.Indicator.MinorWidth = unit.Dp(7)
	style.Indicator.CornerRadius = unit.Dp(4)
	style.Indicator.Color = color.NRGBA{R: 136, G: 149, B: 170, A: 168}
	style.Indicator.HoverColor = color.NRGBA{R: 182, G: 198, B: 225, A: 232}
	return style
}

func viewerSettingsSectionStyleFor(kind string) viewerSettingsSectionStyle {
	switch kind {
	case "p1":
		return viewerSettingsSectionStyle{
			Fill:        color.NRGBA{R: 32, G: 26, B: 18, A: 255},
			Border:      color.NRGBA{R: 214, G: 164, B: 88, A: 74},
			BadgeFill:   color.NRGBA{R: 82, G: 58, B: 24, A: 255},
			BadgeBorder: color.NRGBA{R: 233, G: 188, B: 114, A: 112},
			BadgeText:   color.NRGBA{R: 248, G: 220, B: 170, A: 255},
		}
	case "p2":
		return viewerSettingsSectionStyle{
			Fill:        color.NRGBA{R: 18, G: 24, B: 34, A: 255},
			Border:      color.NRGBA{R: 102, G: 146, B: 224, A: 74},
			BadgeFill:   color.NRGBA{R: 30, G: 50, B: 86, A: 255},
			BadgeBorder: color.NRGBA{R: 130, G: 175, B: 240, A: 112},
			BadgeText:   color.NRGBA{R: 196, G: 221, B: 255, A: 255},
		}
	case "p3":
		return viewerSettingsSectionStyle{
			Fill:        color.NRGBA{R: 22, G: 25, B: 30, A: 255},
			Border:      color.NRGBA{R: 140, G: 156, B: 180, A: 62},
			BadgeFill:   color.NRGBA{R: 46, G: 52, B: 62, A: 255},
			BadgeBorder: color.NRGBA{R: 174, G: 190, B: 214, A: 104},
			BadgeText:   color.NRGBA{R: 220, G: 228, B: 239, A: 255},
		}
	default:
		return viewerSettingsSectionStyle{
			Fill:        color.NRGBA{R: 22, G: 22, B: 24, A: 255},
			Border:      color.NRGBA{R: 255, G: 255, B: 255, A: 22},
			BadgeFill:   color.NRGBA{R: 42, G: 44, B: 50, A: 255},
			BadgeBorder: color.NRGBA{R: 255, G: 255, B: 255, A: 34},
			BadgeText:   color.NRGBA{R: 216, G: 220, B: 227, A: 255},
		}
	}
}

func viewerSettingsBadgeSurface(accent color.NRGBA) (fill, border, text color.NRGBA) {
	if accent.A == 0 {
		accent = color.NRGBA{R: 176, G: 188, B: 204, A: 255}
	}
	fill = mixNRGBA(color.NRGBA{R: 30, G: 34, B: 40, A: 255}, accent, 0.22)
	fill.A = 255
	border = accent
	border.A = 112
	text = mixNRGBA(color.NRGBA{R: 222, G: 228, B: 236, A: 255}, accent, 0.48)
	text.A = 255
	return fill, border, text
}

func (ui *UI) layoutSettingsViewerBadge(th *material.Theme, gtx layout.Context, label string, fill, border, fg color.NRGBA) layout.Dimensions {
	if label == "" {
		return layout.Dimensions{}
	}
	return fillFlatBox(gtx, fill, border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, label)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.Font.Weight = font.Medium
			lbl.TextSize = ui.scaleModalFontSize(8)
			lbl.Color = fg
			lbl.MaxLines = 1
			lbl.Truncator = "..."
			return layoutVCenteredLabel(gtx, lbl)
		})
	})
}

func (ui *UI) layoutSettingsViewerCard(th *material.Theme, gtx layout.Context, style viewerSettingsSectionStyle, badge, title, note, status string, statusColor color.NRGBA, body layout.Widget) layout.Dimensions {
	return fillFlatBox(gtx, style.Fill, style.Border, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsViewerBadge(th, gtx, badge, style.BadgeFill, style.BadgeBorder, style.BadgeText)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body2(th, title)
							lbl.Font.Typeface = ui.interfaceTypeface()
							lbl.Font.Weight = font.Medium
							lbl.TextSize = ui.scaleModalFontSize(10)
							lbl.Color = txtColor
							lbl.MaxLines = 1
							lbl.Truncator = "..."
							return layoutVCenteredLabel(gtx, lbl)
						}),
					)
				}),
			}
			if status != "" {
				statusFill, statusBorder, statusText := viewerSettingsBadgeSurface(statusColor)
				children[0] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsViewerBadge(th, gtx, badge, style.BadgeFill, style.BadgeBorder, style.BadgeText)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body2(th, title)
							lbl.Font.Typeface = ui.interfaceTypeface()
							lbl.Font.Weight = font.Medium
							lbl.TextSize = ui.scaleModalFontSize(10)
							lbl.Color = txtColor
							lbl.MaxLines = 1
							lbl.Truncator = "..."
							return layoutVCenteredLabel(gtx, lbl)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsViewerBadge(th, gtx, status, statusFill, statusBorder, statusText)
						}),
					)
				})
			}
			if note != "" {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, note)
						lbl.Font.Typeface = ui.interfaceTypeface()
						lbl.TextSize = ui.scaleModalFontSize(9)
						lbl.Color = mixNRGBA(hintColor, style.BadgeText, 0.18)
						lbl.MaxLines = 1
						lbl.Truncator = "..."
						return lbl.Layout(gtx)
					}),
				)
			}
			if body != nil {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(body),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (st *settingsModalState) tabFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" {
		return 0, false
	}
	if st.navPrevTab == "" || st.navAnimAt.IsZero() || st.navPrevTab == st.activeTab {
		if key == st.activeTab {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(st.navAnimAt)
	if elapsed >= toolbarAnimDur {
		st.navPrevTab = ""
		st.navAnimAt = time.Time{}
		if key == st.activeTab {
			return 1, false
		}
		return 0, false
	}
	t := smoothstep01(float32(elapsed) / float32(toolbarAnimDur))
	if key == st.activeTab {
		return t, true
	}
	if key == st.navPrevTab {
		return 1 - t, true
	}
	return 0, true
}

func (st *settingsModalState) setHover(key string, now time.Time) {
	if st == nil || st.navHoverKey == key {
		return
	}
	st.navHoverPrev = st.navHoverKey
	st.navHoverKey = key
	st.navHoverAt = now
}

func (st *settingsModalState) hoverFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" {
		return 0, false
	}
	if st.navHoverAt.IsZero() || st.navHoverPrev == st.navHoverKey {
		if st.navHoverKey == key {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(st.navHoverAt)
	if elapsed >= toolbarHoverDur {
		st.navHoverPrev = ""
		st.navHoverAt = time.Time{}
		if st.navHoverKey == key {
			return 1, false
		}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarHoverDur))
	if key == st.navHoverKey {
		return t, true
	}
	if key == st.navHoverPrev {
		return 1 - t, true
	}
	return 0, true
}

func (st *settingsModalState) setPulse(key string, now time.Time) {
	if st == nil || key == "" {
		return
	}
	st.navPulseKey = key
	st.navPulseAt = now
}

func (st *settingsModalState) pulseFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" || st.navPulseKey != key || st.navPulseAt.IsZero() {
		return 0, false
	}
	elapsed := now.Sub(st.navPulseAt)
	if elapsed >= toolbarClickDur {
		st.navPulseKey = ""
		st.navPulseAt = time.Time{}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarClickDur))
	return 1 - t, true
}

func (st *settingsChoiceAnim) setValue(current *string, next string, now time.Time) {
	if st == nil || current == nil || *current == next {
		return
	}
	st.prev = *current
	st.hasPrev = true
	st.at = now
	*current = next
}

func (st *settingsChoiceAnim) fill(now time.Time, current, key string) (float32, bool) {
	if st == nil {
		return 0, false
	}
	if !st.hasPrev || st.at.IsZero() || st.prev == current {
		if key == current {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(st.at)
	if elapsed >= toolbarAnimDur {
		st.prev = ""
		st.hasPrev = false
		st.at = time.Time{}
		if key == current {
			return 1, false
		}
		return 0, false
	}
	t := smoothstep01(clamp01(float32(elapsed) / float32(toolbarAnimDur)))
	if key == current {
		return t, true
	}
	if key == st.prev {
		return 1 - t, true
	}
	return 0, true
}

func (st *settingsChoiceAnim) position(now time.Time, current string, keys []string) (float32, bool) {
	if len(keys) == 0 {
		return 0, false
	}
	idxOf := func(target string) int {
		for i, key := range keys {
			if key == target {
				return i
			}
		}
		return 0
	}
	currentIdx := float32(idxOf(current))
	if st == nil || !st.hasPrev || st.at.IsZero() || st.prev == current {
		return currentIdx, false
	}
	elapsed := now.Sub(st.at)
	if elapsed >= toolbarAnimDur {
		st.prev = ""
		st.hasPrev = false
		st.at = time.Time{}
		return currentIdx, false
	}
	prevIdx := float32(idxOf(st.prev))
	t := smoothstep01(clamp01(float32(elapsed) / float32(toolbarAnimDur)))
	return prevIdx + (currentIdx-prevIdx)*t, true
}

func (st *settingsModalState) setFooterHover(key string, now time.Time) {
	if st == nil || st.footerHoverKey == key {
		return
	}
	st.footerHoverPrev = st.footerHoverKey
	st.footerHoverKey = key
	st.footerHoverAt = now
}

func (st *settingsModalState) footerHoverFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" {
		return 0, false
	}
	if st.footerHoverAt.IsZero() || st.footerHoverPrev == st.footerHoverKey {
		if st.footerHoverKey == key {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(st.footerHoverAt)
	if elapsed >= toolbarHoverDur {
		st.footerHoverPrev = ""
		st.footerHoverAt = time.Time{}
		if st.footerHoverKey == key {
			return 1, false
		}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarHoverDur))
	if key == st.footerHoverKey {
		return t, true
	}
	if key == st.footerHoverPrev {
		return 1 - t, true
	}
	return 0, true
}

func (st *settingsModalState) setFooterPulse(key string, now time.Time) {
	if st == nil || key == "" {
		return
	}
	st.footerPulseKey = key
	st.footerPulseAt = now
}

func (st *settingsModalState) footerPulseFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" || st.footerPulseKey != key || st.footerPulseAt.IsZero() {
		return 0, false
	}
	elapsed := now.Sub(st.footerPulseAt)
	if elapsed >= toolbarClickDur {
		st.footerPulseKey = ""
		st.footerPulseAt = time.Time{}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarClickDur))
	return 1 - t, true
}

func (ui *UI) saveSettingsModal(now time.Time) error {
	st := ui.settingsModal
	if st == nil {
		return nil
	}
	if err := ui.ensureFMConfigLoaded(); err != nil {
		return err
	}
	if st.activeTab == "config" && st.configEdit.Text() != st.baselineConfig {
		next := fm.DefaultConfig()
		raw := strings.TrimSpace(st.configEdit.Text())
		if raw == "" {
			return fmt.Errorf("config yaml is empty")
		}
		if err := yaml.Unmarshal([]byte(raw), next); err != nil {
			return fmt.Errorf("invalid config yaml: %w", err)
		}
		ui.fmCfg = next
		if err := ui.saveFMConfigAllowDefaultReset("settings-config-tab"); err != nil {
			return err
		}
		ui.applyConfigRuntime(now)
		st.loadFromConfig(ui.fmCfg)
		return nil
	}

	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorPaneBackground)); !ok {
		return fmt.Errorf("pane background color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.FilePaneBackground = fm.FormatHexColor(c)
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorPaneText)); !ok {
		return fmt.Errorf("pane text color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.FilePaneText = fm.FormatHexColor(c)
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorHover)); !ok {
		return fmt.Errorf("hover color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.Hover = fm.FormatHexColor(c)
	}
	if text, err := normalizeSettingsPaneRowTextColor(st.colorHoverText, "hover text"); err != nil {
		return err
	} else {
		ui.fmCfg.Colors.HoverText = text
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorPopupHover)); !ok {
		return fmt.Errorf("popup hover color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.PopupHover = fm.FormatHexColor(c)
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorPopupHoverText)); !ok {
		return fmt.Errorf("popup hover text color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.PopupHoverText = fm.FormatHexColor(c)
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorSelection)); !ok {
		return fmt.Errorf("focused selection color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.Selection = fm.FormatHexColor(c)
	}
	if text, err := normalizeSettingsPaneRowTextColor(st.colorSelectionText, "focused selection text"); err != nil {
		return err
	} else {
		ui.fmCfg.Colors.SelectionText = text
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorSelectedFiles)); !ok {
		return fmt.Errorf("selected files color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.SelectedFiles = fm.FormatHexColor(c)
	}
	if text, err := normalizeSettingsPaneRowTextColor(st.colorSelectedFilesText, "selected files text"); err != nil {
		return err
	} else {
		ui.fmCfg.Colors.SelectedFilesText = text
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorFocusedSelected)); !ok {
		return fmt.Errorf("focused + selected files color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.FocusedSelected = fm.FormatHexColor(c)
	}
	if text, err := normalizeSettingsPaneRowTextColor(st.colorFocusedSelectedText, "focused + selected files text"); err != nil {
		return err
	} else {
		ui.fmCfg.Colors.FocusedSelectedText = text
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorCurrentDir)); !ok {
		return fmt.Errorf("current dir background color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.CurrentDirBg = fm.FormatHexColor(c)
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorCurrentDirText)); !ok {
		return fmt.Errorf("current dir text color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.CurrentDirText = fm.FormatHexColor(c)
	}
	if raw := strings.TrimSpace(st.colorScrollbarThumb); raw == "" {
		ui.fmCfg.Colors.ScrollbarThumb = ""
	} else if c, ok := fm.ParseHexColor(raw); !ok {
		return fmt.Errorf("scrollbar thumb color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.ScrollbarThumb = fm.FormatHexColor(c)
	}
	if raw := strings.TrimSpace(st.colorScrollbarTrack); raw == "" {
		ui.fmCfg.Colors.ScrollbarTrack = ""
	} else if c, ok := fm.ParseHexColor(raw); !ok {
		return fmt.Errorf("scrollbar track color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.ScrollbarTrack = fm.FormatHexColor(c)
	}
	filenameColors, filenameErr := st.draftFilenameColors()
	if filenameErr != "" {
		return fmt.Errorf("%s", filenameErr)
	}
	ui.fmCfg.Colors.Filenames = filenameColors

	cmd := strings.TrimSpace(st.viewCommandEdit.Text())
	if cmd == "" {
		cmd = "cat {path}"
	}
	shell := normalizeViewerShellInput(st.viewShellEdit.Text())
	if _, ok := fm.NormalizeKnownViewerShell(shell); !ok {
		return fmt.Errorf("shell must be auto, sh, pwsh, powershell, cmd, wsl, or wsl:<distro>")
	}
	viewerBgRaw := strings.TrimSpace(st.colorViewerBackground)
	c, ok := fm.ParseHexColor(viewerBgRaw)
	if !ok {
		return fmt.Errorf("viewer background color must use #RRGGBB")
	}
	viewerBg := fm.FormatHexColor(c)
	viewerTextRaw := strings.TrimSpace(st.colorViewerText)
	c, ok = fm.ParseHexColor(viewerTextRaw)
	if !ok {
		return fmt.Errorf("viewer text color must use #RRGGBB")
	}
	viewerText := fm.FormatHexColor(c)
	viewerSelectionRaw := strings.TrimSpace(st.colorViewerSelection)
	c, ok = fm.ParseHexColor(viewerSelectionRaw)
	if !ok {
		return fmt.Errorf("viewer selection color must use #RRGGBB")
	}
	viewerSelection := fm.FormatHexColor(c)
	parseOptionalViewerColor := func(label, raw string) (string, error) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return "", nil
		}
		parsed, ok := fm.ParseHexColor(raw)
		if !ok {
			return "", fmt.Errorf("%s color must use #RRGGBB", label)
		}
		return fm.FormatHexColor(parsed), nil
	}
	viewerHexSelection, err := parseOptionalViewerColor("hex selection", st.colorViewerHexSelection)
	if err != nil {
		return err
	}
	viewerHexOffsetText, err := parseOptionalViewerColor("hex offset text", st.colorViewerHexOffsetText)
	if err != nil {
		return err
	}
	viewerHexBytesText, err := parseOptionalViewerColor("hex bytes text", st.colorViewerHexBytesText)
	if err != nil {
		return err
	}
	viewerHexASCIIText, err := parseOptionalViewerColor("hex ASCII text", st.colorViewerHexASCIIText)
	if err != nil {
		return err
	}

	viewerFontSize := st.viewFontSizeSp
	if viewerFontSize < settingsFontSizeMin {
		return fmt.Errorf("viewer font size must be at least 6")
	}
	paneFontSize := st.paneFontSizeSp
	if paneFontSize < settingsFontSizeMin {
		return fmt.Errorf("pane font size must be at least 6")
	}
	tabsFontSize := st.tabsFontSizeSp
	if tabsFontSize < settingsFontSizeMin {
		return fmt.Errorf("tabs font size must be at least 6")
	}
	terminalFontSize := st.terminalFontSizeSp
	if terminalFontSize < settingsFontSizeMin {
		return fmt.Errorf("terminal font size must be at least 6")
	}
	interfaceFontSize := st.interfaceFontSizeSp
	if interfaceFontSize < settingsFontSizeMin {
		return fmt.Errorf("interface font size must be at least 6")
	}
	currentDirFontSize := st.currentDirFontSizeSp
	if currentDirFontSize < settingsFontSizeMin {
		return fmt.Errorf("current-dir font size must be at least 6")
	}
	if !resources.IsBundledFontFamily(st.interfaceFontFamily) && st.interfaceFontFamily != ui.fmCfg.Interface.Typeface {
		return fmt.Errorf("interface font family is invalid")
	}
	if !resources.IsBundledFontFamily(st.currentDirFontFamily) && st.currentDirFontFamily != ui.fmCfg.CurrentDir.Typeface {
		return fmt.Errorf("current-dir font family is invalid")
	}
	if !resources.IsBundledFontFamily(st.paneFontFamily) && st.paneFontFamily != ui.fmCfg.General.Typeface {
		return fmt.Errorf("pane font family is invalid")
	}
	if !resources.IsBundledFontFamily(st.viewFontFamily) && st.viewFontFamily != ui.fmCfg.Viewer.Typeface {
		return fmt.Errorf("viewer font family is invalid")
	}
	if !resources.IsBundledFontFamily(st.tabsFontFamily) && st.tabsFontFamily != ui.fmCfg.Tabs.Typeface {
		return fmt.Errorf("tabs font family is invalid")
	}
	if !resources.IsBundledMonospaceFontFamily(st.terminalFontFamily) && st.terminalFontFamily != ui.fmCfg.Terminal.Typeface {
		return fmt.Errorf("terminal font family is invalid")
	}
	ui.fmCfg.Interface.Typeface = st.interfaceFontFamily
	ui.fmCfg.Interface.FontSizeSp = interfaceFontSize
	ui.fmCfg.CurrentDir.Typeface = st.currentDirFontFamily
	ui.fmCfg.CurrentDir.FontSizeSp = currentDirFontSize
	ui.fmCfg.General.Typeface = st.paneFontFamily
	ui.fmCfg.General.FontSizeSp = paneFontSize
	ui.fmCfg.Tabs.Typeface = st.tabsFontFamily
	ui.fmCfg.Tabs.FontSizeSp = tabsFontSize
	ui.fmCfg.Viewer.Typeface = st.viewFontFamily
	ui.fmCfg.Terminal.Typeface = st.terminalFontFamily
	ui.fmCfg.Terminal.FontSizeSp = terminalFontSize
	ui.fmCfg.Terminal.AcceleratedKeys = st.terminalAcceleratedKeysBool.Value
	ui.fmCfg.Terminal.PreviewStart, ui.fmCfg.Terminal.PreviewEnd = fm.NormalizeTerminalPreviewRange(st.terminalPreviewStart, st.terminalPreviewEnd)
	ui.fmCfg.Viewer.Command = cmd
	ui.fmCfg.Viewer.Shell = shell
	ui.fmCfg.Viewer.RemoteSearchCommand = fm.NormalizeViewerRemoteSearchCommand(st.viewRemoteSearchCommandEdit.Text())
	ui.fmCfg.Viewer.Background = viewerBg
	ui.fmCfg.Viewer.Text = viewerText
	ui.fmCfg.Viewer.Selection = viewerSelection
	ui.fmCfg.Viewer.HexSelection = viewerHexSelection
	ui.fmCfg.Viewer.HexOffsetText = viewerHexOffsetText
	ui.fmCfg.Viewer.HexBytesText = viewerHexBytesText
	ui.fmCfg.Viewer.HexASCIIText = viewerHexASCIIText
	ui.fmCfg.Viewer.FontSizeSp = viewerFontSize
	ui.fmCfg.General.FileWeight = fm.NormalizeFontWeight(st.paneFileWeight, fm.FontWeightRegular)
	ui.fmCfg.General.DirWeight = fm.NormalizeFontWeight(st.paneDirWeight, fm.FontWeightBold)
	ui.fmCfg.General.PermissionsWeight = fm.NormalizeFontWeight(st.panePermissionsWeight, fm.FontWeightRegular)
	ui.fmCfg.General.SizeWeight = fm.NormalizeFontWeight(st.paneSizeWeight, fm.FontWeightRegular)
	ui.fmCfg.General.DateWeight = fm.NormalizeFontWeight(st.paneDateWeight, fm.FontWeightRegular)
	ui.fmCfg.Columns.NameChars = settingsNormalizePaneChars(st.paneFullChars, 20)
	ui.fmCfg.Columns.BriefChars = settingsNormalizePaneChars(st.paneBriefChars, 16)
	ui.fmCfg.Columns.ShowPermissions = st.paneShowPermissions
	ui.fmCfg.Columns.PermissionFormat = settingsNormalizePermissionFormat(st.panePermissionFormat)
	ui.fmCfg.DateFormats = st.paneDateFormats()
	ui.fmCfg.General.DimInactivePanes = st.generalDimInactiveBool.Value
	ui.fmCfg.General.OpenFavoritesInNewTab = st.generalFavoritesNewTabBool.Value
	ui.fmCfg.General.WheelMovesSelection = st.generalWheelMovesSelection.Value
	ui.fmCfg.General.UseTrash = st.generalUseTrash.Value
	ui.fmCfg.General.DeleteWithoutConfirm = st.generalDeleteWithoutConfirm.Value
	ui.fmCfg.General.CompletionSound = fm.NormalizeCompletionSound(st.generalCompletionSound)
	ui.fmCfg.Viewer.SmoothScrolling = st.viewSmoothScrollingBool.Value
	ui.fmCfg.Viewer.ShowLineNumbers = st.viewShowLineNumbersBool.Value
	ui.fmCfg.Viewer.HideFunctionBarWhenOpen = st.viewHideFunctionBarBool.Value
	ui.fmCfg.Viewer.CommandByTarget = viewerCommandTargetMap(st.viewTargetEntries)
	ui.fmCfg.Viewer.CommandRules = fm.NormalizeViewerCommandRules(st.viewRuleEntries)
	ui.fmCfg.Associations = fm.GroupViewerAssociations(st.viewAssocEntries)
	ui.fmCfg.Viewer.Associations = nil
	if err := ui.saveFMConfigWithOptions("settings-modal", false); err != nil {
		return err
	}
	ui.applyConfigRuntime(now)
	st.loadFromConfig(ui.fmCfg)
	return nil
}

func (ui *UI) layoutSettingsModal(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.settingsModal
	if st == nil {
		return layout.Dimensions{}
	}
	st.keyFocus.attach(gtx)
	st.syncFocusedWidget(gtx)
	st.normalizeKeyboardFocus()
	anyMods := ^key.Modifiers(0)
	bundledFamilies := resources.BundledFontFamilies()
	popupState := func() ([]viewerCommandTargetEntry, []fm.ViewerCommandRule, []viewerAssociationProgram, []settingsColorOption, []settingsColorSwatchGroup, []filenameIconOption) {
		targetEntries, _ := st.viewerCommandTargetPickerEntries()
		ruleEntries, _ := st.viewerCommandRulePickerRules()
		assocPrograms, _ := st.viewerAssociationPickerPrograms()
		colorOptions := []settingsColorOption(nil)
		if st.colorCategoryOpen {
			colorOptions = settingsColorOptionsForScope(st.colorScope)
		}
		colorGroups := []settingsColorSwatchGroup(nil)
		if st.colorPickerOpen {
			colorGroups = st.colorPickerSwatchGroups(st.colorPickerTarget)
		}
		iconOptions := []filenameIconOption(nil)
		if st.filenameIconPickerOpen {
			iconOptions = filenameIconOptions
		}
		return targetEntries, ruleEntries, assocPrograms, colorOptions, colorGroups, iconOptions
	}
	popupTargetEntries, popupRuleEntries, popupAssocPrograms, popupColorOptions, popupColorGroups, popupIconOptions := popupState()
	st.normalizePopupKeyboardFocus(len(popupTargetEntries), len(popupRuleEntries), len(popupAssocPrograms), popupColorOptions, popupColorGroups, popupIconOptions)

	for {
		filters := []event.Filter{
			key.Filter{Name: key.NameEscape, Optional: anyMods},
			key.Filter{Name: key.NameTab, Optional: anyMods},
		}
		if !st.hasFocusedEditor(gtx) {
			filters = append(filters,
				key.Filter{Name: key.NameEnter, Optional: anyMods},
				key.Filter{Name: key.NameReturn, Optional: anyMods},
				key.Filter{Name: key.NameSpace, Optional: anyMods},
				key.Filter{Name: key.NameLeftArrow, Optional: anyMods},
				key.Filter{Name: key.NameRightArrow, Optional: anyMods},
				key.Filter{Name: key.NameUpArrow, Optional: anyMods},
				key.Filter{Name: key.NameDownArrow, Optional: anyMods},
			)
		}
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case key.NameEscape:
			if st.colorPickerOpen || st.colorCategoryOpen {
				st.colorPickerOpen = false
				st.closeColorCategoryPopup()
				st.colorPickerTarget = ""
				st.colorPickerBase = ""
				gtx.Execute(op.InvalidateCmd{})
				break
			}
			if st.viewTargetPickOpen {
				st.viewTargetPickOpen = false
				st.resetPopupKeyboardFocus()
				gtx.Execute(op.InvalidateCmd{})
				break
			}
			if st.viewRulePickOpen {
				st.viewRulePickOpen = false
				st.resetPopupKeyboardFocus()
				gtx.Execute(op.InvalidateCmd{})
				break
			}
			if st.viewAssocPickOpen {
				st.viewAssocPickOpen = false
				st.resetPopupKeyboardFocus()
				gtx.Execute(op.InvalidateCmd{})
				break
			}
			if st.filenameIconPickerOpen {
				st.filenameIconPickerOpen = false
				st.filenameIconPickerTarget = ""
				st.resetPopupKeyboardFocus()
				gtx.Execute(op.InvalidateCmd{})
				break
			}
			ui.closeSettingsModal()
			return layout.Dimensions{}
		case key.NameTab:
			if st.anyPopupOpen() {
				step, ok := dialogTabStep(ke.Modifiers)
				if !ok {
					continue
				}
				if step > 0 {
					switch {
					case st.popupFocusKind == settingsPopupKeyboardNone:
						if st.enterPopupKeyboardFocus(popupTargetEntries, popupRuleEntries, popupAssocPrograms, popupColorOptions, popupColorGroups, popupIconOptions) {
							gtx.Execute(op.InvalidateCmd{})
						} else {
							owner := st.popupOwnerFocus()
							st.closeSettingsPopupsExcept("")
							if owner != settingsKeyboardFocusNone {
								st.setKeyboardFocus(owner)
								st.stepKeyboardFocus(1)
								gtx.Execute(op.InvalidateCmd{})
							}
						}
					case st.popupFocusAction == settingsPopupKeyboardActionRow && popupKeyboardSupportsRemove(st.popupFocusKind):
						if st.setPopupKeyboardFocus(st.popupFocusKind, st.popupFocusIndex, settingsPopupKeyboardActionRemove) {
							gtx.Execute(op.InvalidateCmd{})
						}
					default:
						owner := st.popupOwnerFocus()
						st.closeSettingsPopupsExcept("")
						if owner != settingsKeyboardFocusNone {
							st.setKeyboardFocus(owner)
							st.stepKeyboardFocus(1)
							gtx.Execute(op.InvalidateCmd{})
						}
					}
				} else {
					switch {
					case st.popupFocusKind == settingsPopupKeyboardNone:
						owner := st.popupOwnerFocus()
						st.closeSettingsPopupsExcept("")
						if owner != settingsKeyboardFocusNone {
							st.setKeyboardFocus(owner)
							st.stepKeyboardFocus(-1)
							gtx.Execute(op.InvalidateCmd{})
						}
					case st.popupFocusAction == settingsPopupKeyboardActionRemove:
						if st.setPopupKeyboardFocus(st.popupFocusKind, st.popupFocusIndex, settingsPopupKeyboardActionRow) {
							gtx.Execute(op.InvalidateCmd{})
						}
					default:
						owner := st.popupOwnerFocus()
						st.closeSettingsPopupsExcept("")
						if owner != settingsKeyboardFocusNone && st.setKeyboardFocus(owner) {
							gtx.Execute(op.InvalidateCmd{})
						}
					}
				}
				continue
			}
			step, ok := dialogTabStep(ke.Modifiers)
			if !ok {
				continue
			}
			if st.stepKeyboardFocus(step) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameSpace:
			if ke.Modifiers != 0 || st.anyPopupOpen() || st.hasFocusedEditor(gtx) {
				continue
			}
			if st.toggleFocusedCheckbox() {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameLeftArrow, key.NameRightArrow:
			if ke.Modifiers != 0 || st.hasFocusedEditor(gtx) {
				continue
			}
			if st.anyPopupOpen() {
				step := -1
				if ke.Name == key.NameRightArrow {
					step = 1
				}
				if st.stepPopupKeyboardMove(step, 0, len(popupTargetEntries), len(popupRuleEntries), len(popupAssocPrograms), popupColorOptions, popupColorGroups, popupIconOptions) {
					gtx.Execute(op.InvalidateCmd{})
				}
				continue
			}
			step := -1
			if ke.Name == key.NameRightArrow {
				step = 1
			}
			if st.stepFocusedHorizontalGroup(step, bundledFamilies, gtx.Now) {
				st.normalizeKeyboardFocus()
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameUpArrow:
			if ke.Modifiers != 0 || st.hasFocusedEditor(gtx) {
				continue
			}
			if st.anyPopupOpen() {
				if st.stepPopupKeyboardMove(0, -1, len(popupTargetEntries), len(popupRuleEntries), len(popupAssocPrograms), popupColorOptions, popupColorGroups, popupIconOptions) {
					gtx.Execute(op.InvalidateCmd{})
				}
				continue
			}
			if st.stepFocusedNumber(1) {
				st.errText = ""
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			if st.focus != settingsKeyboardFocusNav {
				continue
			}
			if st.stepActiveTab(-1, gtx.Now) {
				st.keyFocus.focusKeyboard()
				st.normalizeKeyboardFocus()
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameDownArrow:
			if ke.Modifiers != 0 || st.hasFocusedEditor(gtx) {
				continue
			}
			if st.anyPopupOpen() {
				if st.stepPopupKeyboardMove(0, 1, len(popupTargetEntries), len(popupRuleEntries), len(popupAssocPrograms), popupColorOptions, popupColorGroups, popupIconOptions) {
					gtx.Execute(op.InvalidateCmd{})
				}
				continue
			}
			if st.stepFocusedNumber(-1) {
				st.errText = ""
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			if st.focus != settingsKeyboardFocusNav {
				continue
			}
			if st.stepActiveTab(1, gtx.Now) {
				st.keyFocus.focusKeyboard()
				st.normalizeKeyboardFocus()
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameEnter, key.NameReturn:
			if ke.Modifiers != 0 || st.hasFocusedEditor(gtx) {
				continue
			}
			if st.anyPopupOpen() {
				if st.activatePopupKeyboardFocus(popupTargetEntries, popupRuleEntries, popupAssocPrograms, popupColorOptions, popupColorGroups, popupIconOptions) {
					gtx.Execute(op.InvalidateCmd{})
				}
				continue
			}
			if st.activateFocusedAction(gtx.Now) {
				if st.anyPopupOpen() {
					popupTargetEntries, popupRuleEntries, popupAssocPrograms, popupColorOptions, popupColorGroups, popupIconOptions = popupState()
					st.enterPopupKeyboardFocus(popupTargetEntries, popupRuleEntries, popupAssocPrograms, popupColorOptions, popupColorGroups, popupIconOptions)
				}
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			action := settingsFooterActionSave
			if st.focus == settingsKeyboardFocusFooter {
				action = st.normalizedFooterAction()
			}
			switch action {
			case settingsFooterActionCancel:
				st.setFooterPulse("cancel", gtx.Now)
				ui.closeSettingsModal()
				return layout.Dimensions{}
			default:
				st.setFooterPulse("save", gtx.Now)
				if err := ui.saveSettingsModal(gtx.Now); err != nil {
					st.errText = err.Error()
				} else {
					ui.closeSettingsModal()
					return layout.Dimensions{}
				}
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}

	for st.backdropClick.Clicked(gtx) {
	}
	if st.closeClick.Clicked(gtx) {
		ui.closeSettingsModal()
		return layout.Dimensions{}
	}
	if st.cancelClick.Clicked(gtx) {
		st.footerFocus = settingsFooterActionCancel
		st.setKeyboardFocus(settingsKeyboardFocusFooter)
		st.setFooterPulse("cancel", gtx.Now)
		ui.closeSettingsModal()
		return layout.Dimensions{}
	}
	if st.saveClick.Clicked(gtx) {
		st.footerFocus = settingsFooterActionSave
		st.setKeyboardFocus(settingsKeyboardFocusFooter)
		st.setFooterPulse("save", gtx.Now)
		if err := ui.saveSettingsModal(gtx.Now); err != nil {
			st.errText = err.Error()
		} else {
			ui.closeSettingsModal()
			return layout.Dimensions{}
		}
	}
	if st.tabGeneralClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusNav)
		st.setActiveTab("general", gtx.Now)
		st.setPulse("general", gtx.Now)
	}
	if st.tabFontsClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusNav)
		st.setActiveTab("fonts", gtx.Now)
		st.setPulse("fonts", gtx.Now)
	}
	if st.tabTerminalClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusNav)
		st.setActiveTab("terminal", gtx.Now)
		st.setPulse("terminal", gtx.Now)
	}
	if st.tabColorsClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusNav)
		st.setActiveTab("colors", gtx.Now)
		st.setPulse("colors", gtx.Now)
	}
	if st.tabViewerClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusNav)
		st.setActiveTab("viewer", gtx.Now)
		st.setPulse("viewer", gtx.Now)
	}
	if st.tabAssocClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusNav)
		st.setActiveTab("associations", gtx.Now)
		st.setPulse("associations", gtx.Now)
	}
	if st.tabConfigClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusNav)
		st.setActiveTab("config", gtx.Now)
		st.setPulse("config", gtx.Now)
	}
	st.normalizeKeyboardFocus()
	dims := st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{A: 140}, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

		width := gtx.Dp(unit.Dp(760))
		maxW := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(20))
		maxH := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(20))
		height := responsiveModalHeight(gtx, maxH)
		if width > maxW {
			width = maxW
		}
		if width < 520 {
			width = 520
		}

		m := op.Record(gtx.Ops)
		card := fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
				return fillRoundedBox(
					gtx,
					gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
					color.NRGBA{R: 20, G: 20, B: 20, A: 252},
					color.NRGBA{R: 255, G: 255, B: 255, A: 18},
					func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSettingsModalHeader(th, gtx, st)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSettingsModalBody(th, gtx, st)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSettingsModalFooter(th, gtx, st)
								}),
							)
							ui.applySettingsNavCursor(gtx, st)
							return dims
						})
					},
				)
			})
		})
		call := m.Stop()

		x := (gtx.Constraints.Max.X - card.Size.X) / 2
		y := (gtx.Constraints.Max.Y - card.Size.Y) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	ui.handleSettingsPopupOutsideClick(gtx, st)
	ui.registerSettingsPopupGlobalPointer(gtx, st)
	return dims
}

func (ui *UI) applySettingsNavCursor(gtx layout.Context, st *settingsModalState) {
	if ui == nil || st == nil {
		return
	}
	if st.tabViewerClick.Hovered() ||
		st.tabAssocClick.Hovered() ||
		st.tabColorsClick.Hovered() ||
		st.tabGeneralClick.Hovered() ||
		st.tabFontsClick.Hovered() ||
		st.tabTerminalClick.Hovered() ||
		st.tabConfigClick.Hovered() {
		pointer.CursorPointer.Add(gtx.Ops)
	}
}

func responsiveModalHeight(gtx layout.Context, available int) int {
	if available < 1 {
		return 1
	}
	height := available * 4 / 5
	minHeight := gtx.Dp(unit.Dp(460))
	if height < minHeight {
		height = minHeight
	}
	if height > available {
		height = available
	}
	return height
}

func (ui *UI) layoutSettingsModalHeader(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body1(th, "Global Settings")
					lbl.Font.Typeface = ui.interfaceTypeface()
					lbl.Font.Weight = font.Bold
					lbl.TextSize = ui.scaleModalFontSize(12)
					lbl.Color = txtColor
					return lbl.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutFlatCloseButton(gtx, &st.closeClick, false)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(layoutDialogHorizontalDivider),
	)
}

func fillSettingsNavSegmentBg(gtx layout.Context, bg color.NRGBA, radius int, roundTop, roundBottom bool, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		call.Add(gtx.Ops)
		return dims
	}
	if bg.A != 0 {
		rr := clip.RRect{Rect: image.Rect(0, 0, dims.Size.X, dims.Size.Y)}
		if roundTop {
			rr.NW = radius
			rr.NE = radius
		}
		if roundBottom {
			rr.SW = radius
			rr.SE = radius
		}
		paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
	}
	call.Add(gtx.Ops)
	return dims
}

func (ui *UI) layoutSettingsNavSegment(th *material.Theme, gtx layout.Context, c *widget.Clickable, label string, activeFill, hoverFill, pulseFill float32, roundTop, roundBottom bool) layout.Dimensions {
	if c == nil {
		return layout.Dimensions{}
	}
	dims := c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		activeFill = clamp01(activeFill)
		hoverFill = clamp01(hoverFill)
		pulseFill = clamp01(pulseFill)
		if c.Pressed() && pulseFill < 0.5 {
			pulseFill = 0.5
		}

		baseBlue := color.NRGBA{R: 40, G: 40, B: 40, A: 255}
		hoverDark := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
		hoverLight := color.NRGBA{R: 54, G: 54, B: 54, A: 255}
		pulseCol := color.NRGBA{R: 72, G: 72, B: 72, A: 255}

		bg := mixNRGBA(color.NRGBA{}, baseBlue, activeFill)
		darkMix := hoverFill * (1 - activeFill)
		lightMix := hoverFill * activeFill * 0.25
		bg = mixNRGBA(bg, hoverDark, darkMix)
		bg = mixNRGBA(bg, hoverLight, lightMix)
		bg = mixNRGBA(bg, pulseCol, pulseFill*0.35)

		fg := mixNRGBA(txtColor, color.NRGBA{R: 236, G: 236, B: 236, A: 255}, activeFill)
		fg = mixNRGBA(fg, color.NRGBA{R: 228, G: 228, B: 228, A: 255}, hoverFill*0.75)
		fg = mixNRGBA(fg, color.NRGBA{R: 246, G: 246, B: 246, A: 255}, pulseFill*0.25)
		radius := gtx.Dp(unit.Dp(filePaneControlCornerDp - 1))
		return fillSettingsNavSegmentBg(gtx, bg, radius, roundTop, roundBottom, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.Font.Weight = font.Medium
				lbl.TextSize = ui.scaleModalFontSize(10)
				lbl.Color = fg
				lbl.MaxLines = 1
				lbl.Alignment = text.Middle
				return layoutVCenteredLabel(gtx, lbl)
			})
		})
	})
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}

	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func layoutSettingsNavSeparator(gtx layout.Context) layout.Dimensions {
	h := gtx.Dp(unit.Dp(1))
	if h < 1 {
		h = 1
	}
	w := gtx.Constraints.Max.X
	if w < 1 {
		w = 1
	}
	paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 22}, clip.Rect(image.Rect(0, 0, w, h)).Op())
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func (ui *UI) layoutSettingsHSegment(th *material.Theme, gtx layout.Context, c *widget.Clickable, label string, activeFill, hoverFill, pulseFill, focusFill float32, stripH int, roundLeft, roundRight bool) layout.Dimensions {
	if c == nil {
		return layout.Dimensions{}
	}
	dims := fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			activeFill = clamp01(activeFill)
			hoverFill = clamp01(hoverFill)
			pulseFill = clamp01(pulseFill)
			focusFill = clamp01(focusFill)
			if c.Pressed() && pulseFill < 0.5 {
				pulseFill = 0.5
			}

			baseBlue := color.NRGBA{R: 40, G: 40, B: 40, A: 255}
			hoverDark := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
			hoverLight := color.NRGBA{R: 54, G: 54, B: 54, A: 255}
			pulseCol := color.NRGBA{R: 72, G: 72, B: 72, A: 255}

			bg := mixNRGBA(color.NRGBA{}, baseBlue, activeFill)
			darkMix := hoverFill * (1 - activeFill)
			lightMix := hoverFill * activeFill * 0.25
			bg = mixNRGBA(bg, hoverDark, darkMix)
			bg = mixNRGBA(bg, hoverLight, lightMix)
			bg = mixNRGBA(bg, pulseCol, pulseFill*0.35)
			bg = mixNRGBA(bg, color.NRGBA{R: 212, G: 196, B: 164, A: 30}, focusFill*0.42)

			fg := mixNRGBA(txtColor, color.NRGBA{R: 236, G: 236, B: 236, A: 255}, activeFill)
			fg = mixNRGBA(fg, color.NRGBA{R: 228, G: 228, B: 228, A: 255}, hoverFill*0.75)
			fg = mixNRGBA(fg, color.NRGBA{R: 246, G: 246, B: 246, A: 255}, pulseFill*0.25)
			fg = mixNRGBA(fg, color.NRGBA{R: 248, G: 242, B: 228, A: 255}, focusFill*0.22)

			radius := gtx.Dp(unit.Dp(filePaneControlCornerDp - 1))
			return fillSegmentBg(gtx, bg, radius, roundLeft, roundRight, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(9), Right: unit.Dp(9)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, label)
					lbl.Font.Typeface = ui.interfaceTypeface()
					lbl.Font.Weight = font.Medium
					lbl.TextSize = ui.scaleModalFontSize(10)
					lbl.Color = fg
					lbl.MaxLines = 1
					lbl.Alignment = text.Middle
					return layoutVCenteredLabel(gtx, lbl)
				})
			})
		})
	})
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func (ui *UI) layoutSettingsNavSliderSegment(th *material.Theme, gtx layout.Context, c *widget.Clickable, label string, activeFill, hoverFill, pulseFill, focusFill float32, stripH int) layout.Dimensions {
	if c == nil {
		return layout.Dimensions{}
	}
	dims := fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			activeFill = clamp01(activeFill)
			hoverFill = clamp01(hoverFill)
			pulseFill = clamp01(pulseFill)
			focusFill = clamp01(focusFill)
			if c.Pressed() && pulseFill < 0.5 {
				pulseFill = 0.5
			}

			bg := color.NRGBA{}
			bg = mixNRGBA(bg, color.NRGBA{R: 255, G: 255, B: 255, A: 10}, hoverFill*(1-activeFill))
			bg = mixNRGBA(bg, color.NRGBA{R: 255, G: 255, B: 255, A: 18}, pulseFill*0.25)
			bg = mixNRGBA(bg, color.NRGBA{R: 212, G: 196, B: 164, A: 28}, focusFill*0.4)

			fg := mixNRGBA(txtColor, color.NRGBA{R: 238, G: 238, B: 238, A: 255}, clamp01(activeFill*0.8+0.12))
			fg = mixNRGBA(fg, color.NRGBA{R: 232, G: 232, B: 232, A: 255}, hoverFill*0.75)
			fg = mixNRGBA(fg, color.NRGBA{R: 246, G: 246, B: 246, A: 255}, pulseFill*0.25)
			fg = mixNRGBA(fg, color.NRGBA{R: 248, G: 242, B: 228, A: 255}, focusFill*0.22)

			dims := fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, label)
					lbl.Font.Typeface = ui.interfaceTypeface()
					lbl.Font.Weight = font.Medium
					lbl.TextSize = ui.scaleModalFontSize(10)
					lbl.Color = fg
					lbl.MaxLines = 1
					lbl.Alignment = text.Middle
					return layoutVCenteredLabel(gtx, lbl)
				})
			})
			if focusFill > 0 && dims.Size.X > 0 && dims.Size.Y > 0 {
				accentW := gtx.Dp(unit.Dp(2))
				if accentW < 1 {
					accentW = 1
				}
				paint.FillShape(gtx.Ops, color.NRGBA{R: 228, G: 206, B: 162, A: uint8(72 + 64*focusFill)}, clip.Rect(image.Rect(0, 0, accentW, dims.Size.Y)).Op())
			}
			return dims
		})
	})
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func (ui *UI) layoutSettingsNavTabs(th *material.Theme, gtx layout.Context, st *settingsModalState, fillViewer, fillAssoc, fillColors, fillGeneral, fillFonts, fillTerminal, fillConfig, hoverViewer, hoverAssoc, hoverColors, hoverGeneral, hoverFonts, hoverTerminal, hoverConfig, pulseViewer, pulseAssoc, pulseColors, pulseGeneral, pulseFonts, pulseTerminal, pulseConfig float32) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	stripH := gtx.Dp(unit.Dp(30))
	if stripH < 1 {
		stripH = 1
	}
	sepH := gtx.Dp(unit.Dp(1))
	if sepH < 1 {
		sepH = 1
	}
	totalH := stripH*7 + sepH*6
	pos, animPos := st.tabPosition(gtx.Now)
	if animPos {
		gtx.Execute(op.InvalidateCmd{})
	}
	focusGeneral := float32(0)
	focusFonts := float32(0)
	focusTerminal := float32(0)
	focusViewer := float32(0)
	focusAssoc := float32(0)
	focusColors := float32(0)
	focusConfig := float32(0)
	if st.focus == settingsKeyboardFocusNav {
		switch st.activeTab {
		case "fonts":
			focusFonts = 1
		case "terminal":
			focusTerminal = 1
		case "viewer":
			focusViewer = 1
		case "associations":
			focusAssoc = 1
		case "colors":
			focusColors = 1
		case "config":
			focusConfig = 1
		default:
			focusGeneral = 1
		}
	}

	return fillBgExact(gtx, color.NRGBA{R: 24, G: 24, B: 24, A: 255}, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, totalH, func(gtx layout.Context) layout.Dimensions {
			w := gtx.Constraints.Max.X
			if w < 1 {
				w = 1
			}
			step := stripH + sepH
			sliderY := int(float32(step) * pos)
			maxSliderY := totalH - stripH
			if maxSliderY < 0 {
				maxSliderY = 0
			}
			if sliderY < 0 {
				sliderY = 0
			}
			if sliderY > maxSliderY {
				sliderY = maxSliderY
			}
			sliderRect := image.Rect(0, sliderY, w, sliderY+stripH)

			innerClip := clip.Rect(image.Rect(0, 0, w, totalH)).Push(gtx.Ops)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 54, G: 54, B: 54, A: 255}, clip.Rect(sliderRect).Op())

			dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.tabGeneralClick, "File panes", fillGeneral, hoverGeneral, pulseGeneral, focusGeneral, stripH)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutSettingsNavSeparator(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.tabFontsClick, "Fonts", fillFonts, hoverFonts, pulseFonts, focusFonts, stripH)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutSettingsNavSeparator(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.tabColorsClick, "Colors", fillColors, hoverColors, pulseColors, focusColors, stripH)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutSettingsNavSeparator(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.tabTerminalClick, "Terminal", fillTerminal, hoverTerminal, pulseTerminal, focusTerminal, stripH)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutSettingsNavSeparator(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.tabViewerClick, "Viewer", fillViewer, hoverViewer, pulseViewer, focusViewer, stripH)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutSettingsNavSeparator(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.tabAssocClick, "Associations", fillAssoc, hoverAssoc, pulseAssoc, focusAssoc, stripH)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutSettingsNavSeparator(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.tabConfigClick, "Config", fillConfig, hoverConfig, pulseConfig, focusConfig, stripH)
				}),
			)
			innerClip.Pop()
			return dims
		})
	})
}

func (ui *UI) layoutSettingsTabContent(th *material.Theme, gtx layout.Context, st *settingsModalState, tab string) layout.Dimensions {
	switch tab {
	case "general":
		return ui.layoutSettingsGeneralTab(th, gtx, st)
	case "fonts":
		return ui.layoutSettingsFontsTab(th, gtx, st)
	case "terminal":
		return ui.layoutSettingsTerminalTab(th, gtx, st)
	case "associations":
		return ui.layoutSettingsAssociationsTab(th, gtx, st)
	case "colors":
		return ui.layoutSettingsColorsTab(th, gtx, st)
	case "config":
		return ui.layoutSettingsConfigTab(th, gtx, st)
	default:
		return ui.layoutSettingsViewerTab(th, gtx, st)
	}
}

func (ui *UI) layoutSettingsGeneralTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	list := settingsScrollableListStyle(th, &st.generalTabList)
	return list.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Inset{Right: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsGeneralTabContent(th, gtx, st)
		})
	})
}

func (ui *UI) layoutSettingsGeneralTabContent(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	return ui.layoutSettingsFilePaneEditor(th, gtx, st)
}

func settingsPaneWeightOptions() []terminalShellOption {
	return []terminalShellOption{
		{Key: fm.FontWeightRegular, Label: "Regular"},
		{Key: fm.FontWeightBold, Label: "Bold"},
	}
}

func (ui *UI) layoutSettingsPaneWeightRow(th *material.Theme, gtx layout.Context, st *settingsModalState, label string, clicks []widget.Clickable, current *string, anim *settingsChoiceAnim, focus settingsKeyboardFocus, fallback string) layout.Dimensions {
	if st == nil || current == nil {
		return layout.Dimensions{}
	}
	options := settingsPaneWeightOptions()
	if len(clicks) < len(options) {
		return layout.Dimensions{}
	}
	active := fm.NormalizeFontWeight(*current, fallback)
	*current = active
	for i, opt := range options {
		for clicks[i].Clicked(gtx) {
			st.setKeyboardFocus(focus)
			anim.setValue(current, opt.Key, gtx.Now)
			anim.anim.setPulse(opt.Key, gtx.Now)
			active = opt.Key
			st.errText = ""
		}
	}
	labelW := gtx.Dp(unit.Dp(86))
	if labelW < 64 {
		labelW = 64
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, labelW, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.Font.Weight = font.Medium
				lbl.TextSize = ui.scaleModalFontSize(10)
				lbl.Color = txtColor
				lbl.MaxLines = 1
				return layoutVCenteredLabel(gtx, lbl)
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsShellPicker(th, gtx, options, clicks, active, anim, st.focus == focus)
		}),
	)
}

func (ui *UI) layoutSettingsTerminalTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	rowLabel := func(txt string) layout.Widget {
		return settingsViewerRowLabel(ui, th, txt, true)
	}
	shellOptions := st.viewShellOptions
	if len(shellOptions) == 0 {
		shellOptions = terminalShellOptionsFor(runtime.GOOS, terminalLookPath, nil)
	}
	st.ensureViewShellClicks(len(shellOptions))
	activeShell := normalizeViewerShellInput(st.viewShellEdit.Text())
	for i, opt := range shellOptions {
		if i >= len(st.viewShellClicks) {
			break
		}
		for st.viewShellClicks[i].Clicked(gtx) {
			st.setKeyboardFocus(settingsKeyboardFocusTerminalShell)
			current := activeShell
			st.viewShellAnim.setValue(&current, opt.Key, gtx.Now)
			st.viewShellAnim.anim.setPulse(opt.Key, gtx.Now)
			st.viewShellEdit.SetText(current)
			activeShell = current
			st.errText = ""
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(rowLabel("Shell")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsShellPicker(th, gtx, shellOptions, st.viewShellClicks, activeShell, &st.viewShellAnim, st.focus == settingsKeyboardFocusTerminalShell)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			ed := material.Editor(th, &st.viewShellEdit, "auto")
			ed.Font.Typeface = ui.interfaceTypeface()
			ed.TextSize = ui.scaleModalFontSize(10)
			ed.Color = txtColor
			ed.HintColor = hintColor
			width := gtx.Dp(unit.Dp(280))
			if maxW := gtx.Constraints.Max.X; maxW > 0 && width > maxW {
				width = maxW
			}
			dims := fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutEditorWithContextMenu(th, gtx, "settings-terminal-shell", &st.viewShellEdit, true, func(gtx layout.Context) layout.Dimensions {
					return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewShellEdit), true, ed.Layout)
				})
			})
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusTerminalShell, &st.viewShellEdit)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			before := st.terminalAcceleratedKeysBool.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.terminalAcceleratedKeysBool, "Accelerate Left, Right, Backspace, and Del", ui.scaleModalFontSize(10))
			if st.terminalAcceleratedKeysBool.Value != before {
				st.focus = settingsKeyboardFocusTerminalAcceleratedKeys
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusTerminalAcceleratedKeys, &st.terminalAcceleratedKeysBool)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(rowLabel("Search preview offsets (inclusive)")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsTerminalPreviewControl(th, gtx, st, "Start", &st.terminalPreviewStartStepper, st.terminalPreviewStart, settingsKeyboardFocusTerminalPreviewStart)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsTerminalPreviewControl(th, gtx, st, "End", &st.terminalPreviewEndStepper, st.terminalPreviewEnd, settingsKeyboardFocusTerminalPreviewEnd)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, "0 is the matching line; negative values include lines before it.")
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleModalFontSize(9)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}),
	)
}

func (ui *UI) layoutSettingsTerminalPreviewControl(th *material.Theme, gtx layout.Context, st *settingsModalState, label string, stepper *settingsNumberStepperState, value int, focus settingsKeyboardFocus) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, label)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleModalFontSize(10)
			lbl.Color = txtColor
			return fixedWidth(gtx, gtx.Dp(unit.Dp(44)), func(gtx layout.Context) layout.Dimensions {
				return layoutVCenteredLabel(gtx, lbl)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(74)), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsFontSizeStepper(th, gtx, st, stepper, float32(value), focus)
			})
		}),
	)
}

func (ui *UI) layoutSettingsHelpIcon(th *material.Theme, gtx layout.Context, click *widget.Clickable, helpText string) layout.Dimensions {
	if click == nil || strings.TrimSpace(helpText) == "" {
		return layout.Dimensions{}
	}
	size := gtx.Dp(unit.Dp(16))
	if size < 12 {
		size = 12
	}
	hovered := click.Hovered()
	dims := click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if gtx.Enabled() {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		rect := image.Rect(0, 0, size, size)
		bg := color.NRGBA{R: 34, G: 36, B: 40, A: 228}
		border := color.NRGBA{R: 255, G: 255, B: 255, A: 50}
		fg := hintColor
		if hovered {
			bg = color.NRGBA{R: 50, G: 54, B: 60, A: 246}
			border = color.NRGBA{R: 214, G: 198, B: 166, A: 138}
			fg = color.NRGBA{R: 230, G: 224, B: 208, A: 255}
		}
		rr := clip.UniformRRect(rect, size/2)
		paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
		paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())

		lbl := material.Caption(th, "?")
		lbl.Font.Typeface = ui.interfaceTypeface()
		lbl.Font.Weight = font.Bold
		lbl.TextSize = ui.scaleModalFontSize(9)
		lbl.Color = fg
		labelGtx := gtx
		labelGtx.Constraints = layout.Exact(rect.Size())
		return layout.Center.Layout(labelGtx, lbl.Layout)
	})
	if hovered {
		gtx.Execute(op.InvalidateCmd{})
		m := op.Record(gtx.Ops)
		tipGtx := gtx
		tipGtx.Constraints = layout.Constraints{
			Max: image.Pt(gtx.Dp(unit.Dp(260)), gtx.Dp(unit.Dp(120))),
		}
		ui.layoutSettingsHelpTooltip(th, tipGtx, helpText)
		call := m.Stop()
		offset := image.Pt(dims.Size.X+gtx.Dp(unit.Dp(6)), -gtx.Dp(unit.Dp(4)))
		deferred := op.Record(gtx.Ops)
		stack := op.Offset(offset).Push(gtx.Ops)
		call.Add(gtx.Ops)
		stack.Pop()
		op.Defer(gtx.Ops, deferred.Stop())
	}
	return dims
}

func (ui *UI) layoutSettingsHelpTooltip(th *material.Theme, gtx layout.Context, text string) layout.Dimensions {
	bg := color.NRGBA{R: 24, G: 26, B: 30, A: 252}
	border := color.NRGBA{R: 255, G: 255, B: 255, A: 38}
	return fillRoundedBox(gtx, gtx.Dp(unit.Dp(6)), bg, border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, text)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleModalFontSize(9)
			lbl.Color = color.NRGBA{R: 214, G: 218, B: 224, A: 255}
			lbl.MaxLines = 3
			lbl.Truncator = "..."
			return lbl.Layout(gtx)
		})
	})
}

func settingsCompletionSoundOptions() []terminalShellOption {
	return []terminalShellOption{
		{Key: fm.CompletionSoundNever, Label: "Never"},
		{Key: fm.CompletionSoundAlways, Label: "Always"},
		{Key: fm.CompletionSoundBackground, Label: "Background only"},
	}
}

func (ui *UI) layoutSettingsCompletionSoundRow(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return settingsViewerRowLabel(ui, th, "Completion sound", true)(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsShellPicker(
				th,
				gtx,
				settingsCompletionSoundOptions(),
				st.generalCompletionSoundClicks[:],
				fm.NormalizeCompletionSound(st.generalCompletionSound),
				&st.generalCompletionSoundAnim,
				st.focus == settingsKeyboardFocusGeneralCompletionSound,
			)
		}),
	)
}

func (ui *UI) layoutSettingsFontsTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	bundledFamilies := resources.BundledFontFamilies()
	st.ensureInterfaceFontFamilyClicks(len(bundledFamilies))
	st.ensureCurrentDirFontFamilyClicks(len(bundledFamilies))
	st.ensurePaneFontFamilyClicks(len(bundledFamilies))
	st.ensureTabsFontFamilyClicks(len(bundledFamilies))
	st.ensureViewFontFamilyClicks(len(bundledFamilies))
	st.ensureTerminalFontFamilyClicks(len(bundledFamilies))
	for i, family := range bundledFamilies {
		if st.interfaceFontFamilyClicks[i].Clicked(gtx) {
			st.setKeyboardFocus(settingsKeyboardFocusFontsInterfaceFont)
			st.interfaceFontPickerAnim.setValue(&st.interfaceFontFamily, family.Name, gtx.Now)
			st.interfaceFontPickerAnim.anim.setPulse(family.Name, gtx.Now)
			st.errText = ""
		}
		if st.currentDirFontFamilyClicks[i].Clicked(gtx) {
			st.setKeyboardFocus(settingsKeyboardFocusFontsCurrentDirFont)
			st.currentDirFontPickerAnim.setValue(&st.currentDirFontFamily, family.Name, gtx.Now)
			st.currentDirFontPickerAnim.anim.setPulse(family.Name, gtx.Now)
			st.errText = ""
		}
		if st.paneFontFamilyClicks[i].Clicked(gtx) {
			st.setKeyboardFocus(settingsKeyboardFocusGeneralPaneFont)
			st.paneFontPickerAnim.setValue(&st.paneFontFamily, family.Name, gtx.Now)
			st.paneFontPickerAnim.anim.setPulse(family.Name, gtx.Now)
			st.errText = ""
		}
		if st.viewFontFamilyClicks[i].Clicked(gtx) {
			st.setKeyboardFocus(settingsKeyboardFocusGeneralViewFont)
			st.viewFontPickerAnim.setValue(&st.viewFontFamily, family.Name, gtx.Now)
			st.viewFontPickerAnim.anim.setPulse(family.Name, gtx.Now)
			st.errText = ""
		}
		if st.tabsFontFamilyClicks[i].Clicked(gtx) {
			st.setKeyboardFocus(settingsKeyboardFocusFontsTabsFont)
			st.tabsFontPickerAnim.setValue(&st.tabsFontFamily, family.Name, gtx.Now)
			st.tabsFontPickerAnim.anim.setPulse(family.Name, gtx.Now)
			st.errText = ""
		}
		if st.terminalFontFamilyClicks[i].Clicked(gtx) {
			st.setKeyboardFocus(settingsKeyboardFocusFontsTerminalFont)
			st.terminalFontPickerAnim.setValue(&st.terminalFontFamily, family.Name, gtx.Now)
			st.terminalFontPickerAnim.anim.setPulse(family.Name, gtx.Now)
			st.errText = ""
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsFontRow(th, gtx, st, "Interface", bundledFamilies, st.interfaceFontFamilyClicks, st.interfaceFontFamily, &st.interfaceFontPickerAnim, st.focus == settingsKeyboardFocusFontsInterfaceFont, &st.interfaceFontSizeStepper, st.interfaceFontSizeSp, settingsKeyboardFocusFontsInterfaceFontSize)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsFontRow(th, gtx, st, "Current dir", bundledFamilies, st.currentDirFontFamilyClicks, st.currentDirFontFamily, &st.currentDirFontPickerAnim, st.focus == settingsKeyboardFocusFontsCurrentDirFont, &st.currentDirFontSizeStepper, st.currentDirFontSizeSp, settingsKeyboardFocusFontsCurrentDirFontSize)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsFontRow(th, gtx, st, "Pane", bundledFamilies, st.paneFontFamilyClicks, st.paneFontFamily, &st.paneFontPickerAnim, st.focus == settingsKeyboardFocusGeneralPaneFont, &st.paneFontSizeStepper, st.paneFontSizeSp, settingsKeyboardFocusGeneralPaneFontSize)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsFontRow(th, gtx, st, "Tabs", bundledFamilies, st.tabsFontFamilyClicks, st.tabsFontFamily, &st.tabsFontPickerAnim, st.focus == settingsKeyboardFocusFontsTabsFont, &st.tabsFontSizeStepper, st.tabsFontSizeSp, settingsKeyboardFocusFontsTabsFontSize)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsFontRow(th, gtx, st, "Viewer", bundledFamilies, st.viewFontFamilyClicks, st.viewFontFamily, &st.viewFontPickerAnim, st.focus == settingsKeyboardFocusGeneralViewFont, &st.viewFontSizeStepper, st.viewFontSizeSp, settingsKeyboardFocusGeneralViewFontSize)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsFontRow(th, gtx, st, "Terminal", bundledFamilies, st.terminalFontFamilyClicks, st.terminalFontFamily, &st.terminalFontPickerAnim, st.focus == settingsKeyboardFocusFontsTerminalFont, &st.terminalFontSizeStepper, st.terminalFontSizeSp, settingsKeyboardFocusFontsTerminalFontSize)
		}),
	)
}

func (ui *UI) layoutSettingsFontRow(th *material.Theme, gtx layout.Context, st *settingsModalState, label string, families []resources.BundledFontFamily, clicks []widget.Clickable, active string, anim *settingsChoiceAnim, pickerFocused bool, stepper *settingsNumberStepperState, value float32, sizeFocus settingsKeyboardFocus) layout.Dimensions {
	labelW := ui.settingsFontRowLabelWidth(th, gtx)
	sizeW := gtx.Dp(unit.Dp(74))
	if sizeW < 62 {
		sizeW = 62
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, labelW, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.Font.Weight = font.Medium
				lbl.TextSize = ui.scaleModalFontSize(10)
				lbl.Color = txtColor
				lbl.MaxLines = 1
				return layoutVCenteredLabel(gtx, lbl)
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsFontFamilyPicker(th, gtx, families, clicks, active, anim, pickerFocused)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, sizeW, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsFontSizeStepper(th, gtx, st, stepper, value, sizeFocus)
			})
		}),
	)
}

func (ui *UI) settingsFontRowLabelWidth(th *material.Theme, gtx layout.Context) int {
	probe := material.Body2(th, "Current dir")
	probe.Font.Typeface = ui.interfaceTypeface()
	probe.Font.Weight = font.Medium
	probe.TextSize = ui.scaleModalFontSize(10)
	probe.MaxLines = 1
	width := measureLabelUnconstrained(gtx, probe).Size.X + gtx.Dp(unit.Dp(4))
	minWidth := gtx.Dp(unit.Dp(96))
	if width < minWidth {
		width = minWidth
	}
	return width
}

func (ui *UI) layoutSettingsFontSizeStepper(th *material.Theme, gtx layout.Context, st *settingsModalState, stepper *settingsNumberStepperState, value float32, focus settingsKeyboardFocus) layout.Dimensions {
	if st == nil || stepper == nil {
		return layout.Dimensions{}
	}
	for stepper.valueClick.Clicked(gtx) {
		st.setKeyboardFocus(focus)
	}
	for stepper.upClick.Clicked(gtx) {
		st.setKeyboardFocus(focus)
		st.stepFocusedNumber(1)
		st.errText = ""
	}
	for stepper.downClick.Clicked(gtx) {
		st.setKeyboardFocus(focus)
		st.stepFocusedNumber(-1)
		st.errText = ""
	}
	focused := st.focus == focus
	textSize := ui.scaleModalFontSize(10)
	height := gtx.Dp(unit.Dp(22))
	if height < 18 {
		height = 18
	}
	if stepper.valueClick.Hovered() || stepper.upClick.Hovered() || stepper.downClick.Hovered() {
		pointer.CursorPointer.Add(gtx.Ops)
	}
	return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsFontSizeValue(th, gtx, &stepper.valueClick, value, focused, textSize)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, gtx.Dp(unit.Dp(17)), func(gtx layout.Context) layout.Dimensions {
					half := height / 2
					if half < 1 {
						half = 1
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, half, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSettingsFontSizeButton(gtx, &stepper.upClick, uitheme.ArrowUpIcon(), focused)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, height-half, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSettingsFontSizeButton(gtx, &stepper.downClick, uitheme.ArrowDownIcon(), focused)
							})
						}),
					)
				})
			}),
		)
	})
}

func (ui *UI) layoutSettingsFontSizeValue(th *material.Theme, gtx layout.Context, c *widget.Clickable, value float32, focused bool, textSize unit.Sp) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{R: 34, G: 34, B: 34, A: 255}
		border := color.NRGBA{R: 255, G: 255, B: 255, A: 18}
		fg := txtColor
		if c.Hovered() {
			bg = color.NRGBA{R: 42, G: 42, B: 42, A: 255}
			border = color.NRGBA{R: 255, G: 255, B: 255, A: 36}
		}
		if focused {
			bg = color.NRGBA{R: 48, G: 48, B: 48, A: 255}
			border = color.NRGBA{R: 160, G: 148, B: 122, A: 190}
			fg = color.NRGBA{R: 244, G: 238, B: 225, A: 255}
		}
		return fillFlatBox(gtx, bg, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, formatConfigFloat(value))
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.Font.Weight = font.Medium
				lbl.TextSize = textSize
				lbl.Color = fg
				lbl.MaxLines = 1
				lbl.Alignment = text.Middle
				return layoutVCenteredLabel(gtx, lbl)
			})
		})
	})
}

func (ui *UI) layoutSettingsFontSizeButton(gtx layout.Context, c *widget.Clickable, icon *widget.Icon, focused bool) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{R: 30, G: 30, B: 30, A: 255}
		border := color.NRGBA{R: 255, G: 255, B: 255, A: 18}
		iconColor := hintColor
		if c.Hovered() {
			bg = color.NRGBA{R: 46, G: 46, B: 46, A: 255}
			border = color.NRGBA{R: 255, G: 255, B: 255, A: 42}
			iconColor = txtColor
		}
		if focused {
			border = mixNRGBA(border, color.NRGBA{R: 160, G: 148, B: 122, A: 210}, 0.7)
		}
		return fillFlatBox(gtx, bg, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := gtx.Dp(unit.Dp(10))
				if size < 1 {
					size = 1
				}
				if icon != nil {
					iconGtx := gtx
					iconGtx.Constraints = layout.Exact(image.Pt(size, size))
					icon.Layout(iconGtx, iconColor)
				}
				return layout.Dimensions{Size: image.Pt(size, size)}
			})
		})
	})
}

func (ui *UI) layoutSettingsFontFamilyPicker(th *material.Theme, gtx layout.Context, families []resources.BundledFontFamily, clicks []widget.Clickable, active string, anim *settingsChoiceAnim, focused bool) layout.Dimensions {
	if len(families) == 0 {
		return layout.Dimensions{}
	}
	textSize := ui.scaleModalFontSize(10)
	keys := make([]string, len(families))
	hoverKey := ""
	for i, family := range families {
		keys[i] = family.Name
		if i < len(clicks) && clicks[i].Hovered() {
			hoverKey = family.Name
		}
	}
	if anim != nil {
		anim.anim.setHover(hoverKey, gtx.Now)
	}
	pos := float32(0)
	animating := false
	if anim != nil {
		pos, animating = anim.position(gtx.Now, active, keys)
	} else {
		for i, key := range keys {
			if key == active {
				pos = float32(i)
				break
			}
		}
	}
	stripH := gtx.Dp(unit.Dp(22))
	if stripH < 1 {
		stripH = 1
	}
	specs := make([]slidingTabSpec, 0, len(families))
	for i, family := range families {
		activeFill := float32(0)
		hoverFill := float32(0)
		pulseFill := float32(0)
		focusFill := float32(0)
		if anim != nil {
			activeFill, _ = anim.fill(gtx.Now, active, family.Name)
			hoverFill, _ = anim.anim.hoverFill(gtx.Now, family.Name)
			pulseFill, _ = anim.anim.pulseFill(gtx.Now, family.Name)
		} else if family.Name == active {
			activeFill = 1
		}
		if focused && family.Name == active {
			focusFill = 1
		}
		specs = append(specs, slidingTabSpec{
			Label:      settingsFontFamilyLabel(family),
			Typeface:   font.Typeface(family.Name),
			Click:      &clicks[i],
			ActiveFill: activeFill,
			HoverFill:  hoverFill,
			PulseFill:  pulseFill,
			FocusFill:  focusFill,
		})
	}
	for _, family := range families {
		if anim != nil {
			if _, ok := anim.fill(gtx.Now, active, family.Name); ok {
				animating = true
			}
			if _, ok := anim.anim.hoverFill(gtx.Now, family.Name); ok {
				animating = true
			}
			if _, ok := anim.anim.pulseFill(gtx.Now, family.Name); ok {
				animating = true
			}
		}
	}
	if animating {
		gtx.Execute(op.InvalidateCmd{})
	}
	return ui.layoutSlidingTabStrip(th, gtx, stripH, pos, textSize, specs)
}

func settingsFontFamilyLabel(family resources.BundledFontFamily) string {
	if strings.TrimSpace(family.Label) != "" {
		return family.Label
	}
	return family.Name
}

func (ui *UI) layoutSettingsShellPicker(th *material.Theme, gtx layout.Context, options []terminalShellOption, clicks []widget.Clickable, active string, anim *settingsChoiceAnim, focused bool) layout.Dimensions {
	if len(options) == 0 || len(clicks) < len(options) {
		return layout.Dimensions{}
	}
	textSize := ui.scaleModalFontSize(10)
	keys := make([]string, len(options))
	hoverKey := ""
	for i, opt := range options {
		keys[i] = opt.Key
		if i < len(clicks) && clicks[i].Hovered() {
			hoverKey = opt.Key
		}
	}
	if anim != nil {
		anim.anim.setHover(hoverKey, gtx.Now)
	}
	pos := float32(0)
	animating := false
	if anim != nil {
		pos, animating = anim.position(gtx.Now, active, keys)
	} else {
		for i, key := range keys {
			if key == active {
				pos = float32(i)
				break
			}
		}
	}
	stripH := gtx.Dp(unit.Dp(22))
	if stripH < 1 {
		stripH = 1
	}
	specs := make([]slidingTabSpec, 0, len(options))
	for i, opt := range options {
		activeFill := float32(0)
		hoverFill := float32(0)
		pulseFill := float32(0)
		focusFill := float32(0)
		if anim != nil {
			activeFill, _ = anim.fill(gtx.Now, active, opt.Key)
			hoverFill, _ = anim.anim.hoverFill(gtx.Now, opt.Key)
			pulseFill, _ = anim.anim.pulseFill(gtx.Now, opt.Key)
		} else if opt.Key == active {
			activeFill = 1
		}
		if focused && opt.Key == active {
			focusFill = 1
		}
		specs = append(specs, slidingTabSpec{
			Label:      opt.Label,
			Click:      &clicks[i],
			ActiveFill: activeFill,
			HoverFill:  hoverFill,
			PulseFill:  pulseFill,
			FocusFill:  focusFill,
		})
	}
	for _, opt := range options {
		if anim != nil {
			if _, ok := anim.fill(gtx.Now, active, opt.Key); ok {
				animating = true
			}
			if _, ok := anim.anim.hoverFill(gtx.Now, opt.Key); ok {
				animating = true
			}
			if _, ok := anim.anim.pulseFill(gtx.Now, opt.Key); ok {
				animating = true
			}
		}
	}
	if animating {
		gtx.Execute(op.InvalidateCmd{})
	}
	return ui.layoutSlidingTabStrip(th, gtx, stripH, pos, textSize, specs)
}

func (ui *UI) layoutSettingsModalBody(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	fillViewer, animViewer := st.tabFill(gtx.Now, "viewer")
	fillAssoc, animAssoc := st.tabFill(gtx.Now, "associations")
	fillColors, animColors := st.tabFill(gtx.Now, "colors")
	fillGeneral, animGeneral := st.tabFill(gtx.Now, "general")
	fillFonts, animFonts := st.tabFill(gtx.Now, "fonts")
	fillTerminal, animTerminal := st.tabFill(gtx.Now, "terminal")
	fillConfig, animConfig := st.tabFill(gtx.Now, "config")
	hoverKey := ""
	if st.tabViewerClick.Hovered() {
		hoverKey = "viewer"
	}
	if st.tabAssocClick.Hovered() {
		hoverKey = "associations"
	}
	if st.tabColorsClick.Hovered() {
		hoverKey = "colors"
	}
	if st.tabGeneralClick.Hovered() {
		hoverKey = "general"
	}
	if st.tabFontsClick.Hovered() {
		hoverKey = "fonts"
	}
	if st.tabTerminalClick.Hovered() {
		hoverKey = "terminal"
	}
	if st.tabConfigClick.Hovered() {
		hoverKey = "config"
	}
	st.setHover(hoverKey, gtx.Now)
	hoverViewer, hoverAnimViewer := st.hoverFill(gtx.Now, "viewer")
	hoverAssoc, hoverAnimAssoc := st.hoverFill(gtx.Now, "associations")
	hoverColors, hoverAnimColors := st.hoverFill(gtx.Now, "colors")
	hoverGeneral, hoverAnimGeneral := st.hoverFill(gtx.Now, "general")
	hoverFonts, hoverAnimFonts := st.hoverFill(gtx.Now, "fonts")
	hoverTerminal, hoverAnimTerminal := st.hoverFill(gtx.Now, "terminal")
	hoverConfig, hoverAnimConfig := st.hoverFill(gtx.Now, "config")
	pulseViewer, pulseAnimViewer := st.pulseFill(gtx.Now, "viewer")
	pulseAssoc, pulseAnimAssoc := st.pulseFill(gtx.Now, "associations")
	pulseColors, pulseAnimColors := st.pulseFill(gtx.Now, "colors")
	pulseGeneral, pulseAnimGeneral := st.pulseFill(gtx.Now, "general")
	pulseFonts, pulseAnimFonts := st.pulseFill(gtx.Now, "fonts")
	pulseTerminal, pulseAnimTerminal := st.pulseFill(gtx.Now, "terminal")
	pulseConfig, pulseAnimConfig := st.pulseFill(gtx.Now, "config")
	if animViewer || animAssoc || animColors || animGeneral || animFonts || animTerminal || animConfig ||
		hoverAnimViewer || hoverAnimAssoc || hoverAnimColors || hoverAnimGeneral || hoverAnimFonts || hoverAnimTerminal || hoverAnimConfig ||
		pulseAnimViewer || pulseAnimAssoc || pulseAnimColors || pulseAnimGeneral || pulseAnimFonts || pulseAnimTerminal || pulseAnimConfig {
		gtx.Execute(op.InvalidateCmd{})
	}

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(146)), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsNavTabs(
					th, gtx, st,
					fillViewer, fillAssoc, fillColors, fillGeneral, fillFonts, fillTerminal, fillConfig,
					hoverViewer, hoverAssoc, hoverColors, hoverGeneral, hoverFonts, hoverTerminal, hoverConfig,
					pulseViewer, pulseAssoc, pulseColors, pulseGeneral, pulseFonts, pulseTerminal, pulseConfig,
				)
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
		layout.Rigid(layoutDialogVerticalDivider),
		layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsTabContent(th, gtx, st, st.activeTab)
		}),
	)
}

func (ui *UI) layoutSettingsViewerTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	for {
		ev, ok := st.viewTargetKeyEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
			st.refreshViewerCommandTargetDraftInfo(false)
		}
	}
	for {
		ev, ok := st.viewTargetCommandEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
			st.refreshViewerCommandTargetDraftInfo(false)
		}
	}
	st.syncViewerCommandTargetEditors()
	for st.viewTargetApplyClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusViewerTargetApply)
		action, err := st.upsertCurrentViewerCommandTarget()
		if err != nil {
			st.errText = err.Error()
			continue
		}
		st.errText = ""
		st.viewTargetPickOpen = false
		if action == "Update" {
			st.targetInfoText = "Pending change; Save to persist"
		} else {
			st.targetInfoText = "Pending add; Save to persist"
		}
	}
	for st.viewTargetPickClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusViewerTargetBrowse)
		st.toggleViewerCommandTargetPicker()
	}

	for {
		ev, ok := st.viewRulePatternEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
			st.refreshViewerCommandRuleDraftInfo(false)
		}
	}
	for {
		ev, ok := st.viewRuleCommandEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
			st.refreshViewerCommandRuleDraftInfo(false)
		}
	}
	st.syncViewerCommandRuleEditors()
	for st.viewRuleApplyClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusViewerRuleApply)
		action, err := st.upsertCurrentViewerCommandRule()
		if err != nil {
			st.errText = err.Error()
			continue
		}
		st.errText = ""
		st.viewRulePickOpen = false
		if action == "Update" {
			st.ruleInfoText = "Pending change; Save to persist"
		} else {
			st.ruleInfoText = "Pending add; Save to persist"
		}
	}
	for st.viewRulePickClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusViewerRuleBrowse)
		st.toggleViewerCommandRulePicker()
	}

	currentTargetKey := normalizeViewerCommandTargetInput(st.viewTargetKeyEdit.Text())
	_, currentTargetExists := st.viewerCommandTarget(currentTargetKey)
	if !currentTargetExists && st.viewTargetEditingKey != "" {
		_, currentTargetExists = st.viewerCommandTarget(st.viewTargetEditingKey)
	}
	pickerTargets, pickerTargetMatchCount := st.viewerCommandTargetPickerEntries()

	currentRulePattern := strings.TrimSpace(st.viewRulePatternEdit.Text())
	_, currentRuleExists := st.viewerCommandRule(currentRulePattern)
	if !currentRuleExists && st.viewRuleEditingPattern != "" {
		_, currentRuleExists = st.viewerCommandRule(st.viewRuleEditingPattern)
	}
	pickerRules, pickerMatchCount := st.viewerCommandRulePickerRules()

	rowLabel := func(txt string, enabled bool) layout.Widget {
		return settingsViewerRowLabel(ui, th, txt, enabled)
	}

	savedTargetCount := 0
	pendingTargetCount := 0
	targetEntriesByKey := make(map[string]viewerCommandTargetEntry, len(st.viewTargetEntries))
	targetSavedByKey := make(map[string]viewerCommandTargetEntry, len(st.viewTargetSavedEntries))
	for _, entry := range st.viewTargetEntries {
		targetEntriesByKey[entry.Key] = entry
	}
	for _, entry := range st.viewTargetSavedEntries {
		targetSavedByKey[entry.Key] = entry
	}
	for _, entry := range st.viewTargetEntries {
		if saved, ok := targetSavedByKey[entry.Key]; ok && saved.Command == entry.Command {
			savedTargetCount++
			continue
		}
		pendingTargetCount++
	}
	for _, entry := range st.viewTargetSavedEntries {
		if _, ok := targetEntriesByKey[entry.Key]; !ok {
			pendingTargetCount++
		}
	}

	targetStatusText := ""
	targetStatusColor := color.NRGBA{R: 152, G: 205, B: 152, A: 255}
	switch {
	case currentTargetKey == "":
		switch {
		case savedTargetCount > 0 && pendingTargetCount > 0:
			targetStatusText = fmt.Sprintf("%d Saved / %d Pending", savedTargetCount, pendingTargetCount)
			targetStatusColor = color.NRGBA{R: 222, G: 190, B: 122, A: 255}
		case pendingTargetCount > 0:
			targetStatusText = fmt.Sprintf("%d Pending", pendingTargetCount)
			targetStatusColor = color.NRGBA{R: 222, G: 190, B: 122, A: 255}
		case savedTargetCount > 0:
			targetStatusText = fmt.Sprintf("%d Saved", savedTargetCount)
			targetStatusColor = color.NRGBA{R: 174, G: 190, B: 214, A: 255}
		}
	case currentTargetExists:
		if saved, ok := targetSavedByKey[currentTargetKey]; ok && saved.Command == st.viewTargetCommandEdit.Text() {
			targetStatusText = "Saved"
		} else {
			targetStatusText = "Pending"
			targetStatusColor = color.NRGBA{R: 222, G: 190, B: 122, A: 255}
		}
	case currentTargetKey != "":
		targetStatusText = "New"
		targetStatusColor = viewerSettingsSectionStyleFor("p1").BadgeText
	}

	savedRuleCount := 0
	pendingRuleCount := 0
	ruleEntriesByPattern := make(map[string]fm.ViewerCommandRule, len(st.viewRuleEntries))
	ruleSavedByPattern := make(map[string]fm.ViewerCommandRule, len(st.viewRuleSavedEntries))
	for _, rule := range st.viewRuleEntries {
		ruleEntriesByPattern[rule.Pattern] = rule
	}
	for _, rule := range st.viewRuleSavedEntries {
		ruleSavedByPattern[rule.Pattern] = rule
	}
	for _, rule := range st.viewRuleEntries {
		if saved, ok := ruleSavedByPattern[rule.Pattern]; ok && saved.Command == rule.Command {
			savedRuleCount++
			continue
		}
		pendingRuleCount++
	}
	for _, rule := range st.viewRuleSavedEntries {
		if _, ok := ruleEntriesByPattern[rule.Pattern]; !ok {
			pendingRuleCount++
		}
	}

	ruleStatusText := ""
	ruleStatusColor := color.NRGBA{R: 152, G: 205, B: 152, A: 255}
	switch {
	case currentRulePattern == "":
		switch {
		case savedRuleCount > 0 && pendingRuleCount > 0:
			ruleStatusText = fmt.Sprintf("%d Saved / %d Pending", savedRuleCount, pendingRuleCount)
			ruleStatusColor = color.NRGBA{R: 222, G: 190, B: 122, A: 255}
		case pendingRuleCount > 0:
			ruleStatusText = fmt.Sprintf("%d Pending", pendingRuleCount)
			ruleStatusColor = color.NRGBA{R: 222, G: 190, B: 122, A: 255}
		case savedRuleCount > 0:
			ruleStatusText = fmt.Sprintf("%d Saved", savedRuleCount)
			ruleStatusColor = color.NRGBA{R: 174, G: 190, B: 214, A: 255}
		}
	case currentRuleExists:
		if saved, ok := ruleSavedByPattern[currentRulePattern]; ok && saved.Command == st.viewRuleCommandEdit.Text() {
			ruleStatusText = "Saved"
		} else {
			ruleStatusText = "Pending"
			ruleStatusColor = color.NRGBA{R: 222, G: 190, B: 122, A: 255}
		}
	case currentRulePattern != "":
		ruleStatusText = "New"
		ruleStatusColor = viewerSettingsSectionStyleFor("p2").BadgeText
	}

	targetApplyLabel := "Add"
	if currentTargetExists {
		targetApplyLabel = "Update"
	}
	ruleApplyLabel := "Add"
	if currentRuleExists {
		ruleApplyLabel = "Update"
	}

	noticeLabel := func(text string, maxLines int, truncator string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, text)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleModalFontSize(9)
			lbl.Color = color.NRGBA{R: 152, G: 205, B: 152, A: 255}
			lbl.MaxLines = maxLines
			lbl.Truncator = truncator
			return lbl.Layout(gtx)
		}
	}

	sections := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(rowLabel("Remote search utility command (SSH hex find)", true)),
				layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(th, &st.viewRemoteSearchCommandEdit, fm.DefaultViewerRemoteSearchCommand)
					ed.Font.Typeface = ui.interfaceTypeface()
					ed.TextSize = ui.scaleModalFontSize(10)
					ed.Color = txtColor
					ed.HintColor = hintColor
					dims := ui.layoutEditorWithContextMenu(th, gtx, "settings-view-remote-search-command", &st.viewRemoteSearchCommandEdit, true, func(gtx layout.Context) layout.Dimensions {
						return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewRemoteSearchCommandEdit), true, ed.Layout)
					})
					st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusViewerRemoteSearch, &st.viewRemoteSearchCommandEdit)
					return dims
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, noticeLabel(viewerRemoteSearchCommandNoticeText(), 6, ""))
				}),
			)
		},
		func(gtx layout.Context) layout.Dimensions {
			before := st.viewSmoothScrollingBool.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.viewSmoothScrollingBool, "Smooth scrolling", ui.scaleModalFontSize(10))
			if st.viewSmoothScrollingBool.Value != before {
				st.focus = settingsKeyboardFocusViewerSmoothScrolling
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusViewerSmoothScrolling, &st.viewSmoothScrollingBool)
			return dims
		},
		func(gtx layout.Context) layout.Dimensions {
			before := st.viewShowLineNumbersBool.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.viewShowLineNumbersBool, "Show line numbers in text viewer (F5 toggles it)", ui.scaleModalFontSize(10))
			if st.viewShowLineNumbersBool.Value != before {
				st.focus = settingsKeyboardFocusViewerShowLineNumbers
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusViewerShowLineNumbers, &st.viewShowLineNumbersBool)
			return dims
		},
		func(gtx layout.Context) layout.Dimensions {
			before := st.viewHideFunctionBarBool.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.viewHideFunctionBarBool, "Auto-hide function bar while viewer is open (F11 toggles it)", ui.scaleModalFontSize(10))
			if st.viewHideFunctionBarBool.Value != before {
				st.focus = settingsKeyboardFocusViewerHideFunctionBar
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusViewerHideFunctionBar, &st.viewHideFunctionBarBool)
			return dims
		},
		func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsViewerCard(th, gtx, viewerSettingsSectionStyleFor("p1"), "Priority 1", "Exact target override", "", targetStatusText, targetStatusColor, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(rowLabel("Target (exact full path)", true)),
					layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(th, &st.viewTargetKeyEdit, "/Users/me/logs/app.log")
								ed.Font.Typeface = ui.interfaceTypeface()
								ed.TextSize = ui.scaleModalFontSize(10)
								ed.Color = txtColor
								ed.HintColor = hintColor
								dims := ui.layoutEditorWithContextMenu(th, gtx, "settings-view-target-key", &st.viewTargetKeyEdit, true, func(gtx layout.Context) layout.Dimensions {
									return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewTargetKeyEdit), true, ed.Layout)
								})
								st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusViewerTargetKey, &st.viewTargetKeyEdit)
								return dims
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSettingsFlatActionButton(th, gtx, &st.viewTargetPickClick, "Browse", st.viewTargetPickOpen, st.focus == settingsKeyboardFocusViewerTargetBrowse, false)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSettingsFlatActionButton(th, gtx, &st.viewTargetApplyClick, targetApplyLabel, currentTargetExists, st.focus == settingsKeyboardFocusViewerTargetApply, false)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !st.viewTargetPickOpen {
							return layout.Dimensions{}
						}
						return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsViewerCommandTargetPicker(th, gtx, st, pickerTargets, pickerTargetMatchCount)
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(rowLabel("Command ({filename} {fullpath} {path})", true)),
					layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(th, &st.viewTargetCommandEdit, "tail -n 200 -f {path}")
								ed.Font.Typeface = ui.interfaceTypeface()
								ed.TextSize = ui.scaleModalFontSize(10)
								ed.Color = txtColor
								ed.HintColor = hintColor
								dims := ui.layoutEditorWithContextMenu(th, gtx, "settings-view-target-command", &st.viewTargetCommandEdit, true, func(gtx layout.Context) layout.Dimensions {
									return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewTargetCommandEdit), true, ed.Layout)
								})
								st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusViewerTargetCommand, &st.viewTargetCommandEdit)
								return dims
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						infoText := st.targetInfoText
						if infoText == "" {
							infoText = st.viewerCommandTargetNoticeText()
						}
						if infoText == "" {
							return layout.Dimensions{}
						}
						return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, noticeLabel(infoText, 2, "..."))
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsViewerCard(th, gtx, viewerSettingsSectionStyleFor("p2"), "Priority 2", "Filename regex rule", "", ruleStatusText, ruleStatusColor, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(rowLabel("Regex (filename only, last match wins)", true)),
					layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(th, &st.viewRulePatternEdit, `(?i)\.log(?:\.\d+)?$`)
								ed.Font.Typeface = ui.interfaceTypeface()
								ed.TextSize = ui.scaleModalFontSize(10)
								ed.Color = txtColor
								ed.HintColor = hintColor
								dims := ui.layoutEditorWithContextMenu(th, gtx, "settings-view-rule-pattern", &st.viewRulePatternEdit, true, func(gtx layout.Context) layout.Dimensions {
									return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewRulePatternEdit), true, ed.Layout)
								})
								st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusViewerRulePattern, &st.viewRulePatternEdit)
								return dims
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSettingsFlatActionButton(th, gtx, &st.viewRulePickClick, "Browse", st.viewRulePickOpen, st.focus == settingsKeyboardFocusViewerRuleBrowse, false)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSettingsFlatActionButton(th, gtx, &st.viewRuleApplyClick, ruleApplyLabel, currentRuleExists, st.focus == settingsKeyboardFocusViewerRuleApply, false)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !st.viewRulePickOpen {
							return layout.Dimensions{}
						}
						return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsViewerCommandRulePicker(th, gtx, st, pickerRules, pickerMatchCount)
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(rowLabel("Command ({filename} {fullpath} {path})", true)),
					layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(th, &st.viewRuleCommandEdit, "tail -n 200 -f {path}")
								ed.Font.Typeface = ui.interfaceTypeface()
								ed.TextSize = ui.scaleModalFontSize(10)
								ed.Color = txtColor
								ed.HintColor = hintColor
								dims := ui.layoutEditorWithContextMenu(th, gtx, "settings-view-rule-command", &st.viewRuleCommandEdit, true, func(gtx layout.Context) layout.Dimensions {
									return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewRuleCommandEdit), true, ed.Layout)
								})
								st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusViewerRuleCommand, &st.viewRuleCommandEdit)
								return dims
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						infoText := st.ruleInfoText
						if infoText == "" {
							infoText = st.viewerCommandRuleNoticeText()
						}
						if infoText == "" {
							return layout.Dimensions{}
						}
						return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, noticeLabel(infoText, 2, "..."))
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsViewerCard(th, gtx, viewerSettingsSectionStyleFor("p3"), "Priority 3", "Fallback command", "", "", color.NRGBA{}, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(rowLabel("Command ({filename} {fullpath} {path})", true)),
					layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						ed := material.Editor(th, &st.viewCommandEdit, "cat {path}")
						ed.Font.Typeface = ui.interfaceTypeface()
						ed.TextSize = ui.scaleModalFontSize(10)
						ed.Color = txtColor
						ed.HintColor = hintColor
						dims := ui.layoutEditorWithContextMenu(th, gtx, "settings-view-command", &st.viewCommandEdit, true, func(gtx layout.Context) layout.Dimensions {
							return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewCommandEdit), true, ed.Layout)
						})
						st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusViewerCommand, &st.viewCommandEdit)
						return dims
					}),
				)
			})
		},
	}

	list := settingsScrollableListStyle(th, &st.viewerTabList)
	return list.Layout(gtx, len(sections), func(gtx layout.Context, i int) layout.Dimensions {
		bottom := unit.Dp(10)
		if i == len(sections)-1 {
			bottom = 0
		}
		return layout.Inset{Right: unit.Dp(2), Bottom: bottom}.Layout(gtx, sections[i])
	})
}

func layoutSettingsPickerRowBackground(gtx layout.Context, bg color.NRGBA, row layout.Widget) layout.Dimensions {
	width := gtx.Constraints.Max.X
	if width < gtx.Constraints.Min.X {
		width = gtx.Constraints.Min.X
	}
	if width <= 0 {
		return fillBgExact(gtx, bg, row)
	}
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fillBgExact(gtx, bg, row)
	})
}

func (ui *UI) layoutSettingsViewerCommandTargetPicker(th *material.Theme, gtx layout.Context, st *settingsModalState, entries []viewerCommandTargetEntry, matchCount int) layout.Dimensions {
	gtx2 := gtx
	maxH := gtx.Dp(unit.Dp(156))
	if gtx2.Constraints.Max.Y > maxH {
		gtx2.Constraints.Max.Y = maxH
	}
	dims := fillRoundedBox(
		gtx2,
		gtx2.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 18, G: 22, B: 30, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			if len(entries) == 0 {
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, "No exact overrides")
					lbl.Font.Typeface = ui.interfaceTypeface()
					lbl.TextSize = ui.scaleModalFontSize(9)
					lbl.Color = hintColor
					return lbl.Layout(gtx)
				})
			}
			var picked *viewerCommandTargetEntry
			removedKey := ""
			currentKey := normalizeViewerCommandTargetInput(st.viewTargetKeyEdit.Text())
			dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if matchCount <= 0 || matchCount >= len(entries) {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, fmt.Sprintf("%d matching overrides", matchCount))
						lbl.Font.Typeface = ui.interfaceTypeface()
						lbl.TextSize = ui.scaleModalFontSize(9)
						lbl.Color = hintColor
						lbl.MaxLines = 1
						lbl.Truncator = "..."
						return lbl.Layout(gtx)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					list := settingsPopupListStyle(th, &st.viewTargetPickList)
					return list.Layout(gtx, len(entries), func(gtx layout.Context, i int) layout.Dimensions {
						entry := entries[i]
						click := st.viewerCommandTargetRowClick(entry.Key)
						removeClick := st.viewerCommandTargetRowRemoveClick(entry.Key)
						rowFocused := st.popupKeyboardMatches(settingsPopupKeyboardViewerTarget, i, settingsPopupKeyboardActionRow)
						removeFocused := st.popupKeyboardMatches(settingsPopupKeyboardViewerTarget, i, settingsPopupKeyboardActionRemove)
						for click.Clicked(gtx) {
							st.setPopupKeyboardFocus(settingsPopupKeyboardViewerTarget, i, settingsPopupKeyboardActionRow)
							if picked == nil {
								entryCopy := entry
								picked = &entryCopy
							}
						}
						for removeClick.Clicked(gtx) {
							st.setPopupKeyboardFocus(settingsPopupKeyboardViewerTarget, i, settingsPopupKeyboardActionRemove)
							if removedKey == "" {
								removedKey = entry.Key
							}
						}
						selected := entry.Key == currentKey
						hovered := click.Hovered() || removeClick.Hovered()
						bg := color.NRGBA{A: 0}
						if selected {
							bg = color.NRGBA{R: 80, G: 120, B: 220, A: 45}
							if rowFocused || removeFocused {
								bg = color.NRGBA{R: 92, G: 132, B: 228, A: 62}
							}
						} else if rowFocused || removeFocused {
							bg = color.NRGBA{R: 74, G: 108, B: 182, A: 52}
						} else if hovered {
							bg = color.NRGBA{R: 255, G: 255, B: 255, A: 10}
						}
						return layoutSettingsPickerRowBackground(gtx, bg, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								displayKey := viewerCommandTargetDisplayKey(entry.Key)
								title := viewerCommandTargetRowTitle(entry.Key)
								headline := displayKey
								if title != "" && title != displayKey {
									headline = title
								}
								detail := entry.Command
								if headline != displayKey {
									detail = displayKey + " | " + entry.Command
								}
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											pointer.CursorPointer.Add(gtx.Ops)
											return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													lbl := material.Body2(th, headline)
													lbl.Font.Typeface = ui.interfaceTypeface()
													lbl.Font.Weight = font.Medium
													lbl.TextSize = ui.scaleModalFontSize(10)
													lbl.Color = txtColor
													lbl.MaxLines = 1
													lbl.Truncator = "..."
													return layoutVCenteredLabel(gtx, lbl)
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													lbl := material.Caption(th, detail)
													lbl.Font.Typeface = ui.interfaceTypeface()
													lbl.TextSize = ui.scaleModalFontSize(8)
													lbl.Color = hintColor
													lbl.MaxLines = 1
													lbl.Truncator = "..."
													return layoutVCenteredLabel(gtx, lbl)
												}),
											)
										})
									}),
									layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return ui.layoutSettingsFlatRemoveButton(gtx, removeClick, removeFocused)
									}),
								)
							})
						})
					})
				}),
			)
			if removedKey != "" {
				if st.removeViewerCommandTarget(removedKey) {
					st.errText = ""
					st.targetInfoText = "Pending removal; Save to persist"
				}
				picked = nil
			}
			if picked != nil {
				st.applyPickedViewerCommandTarget(*picked)
			}
			return dims
		},
	)
	registerSettingsPopupArea(gtx2, &st.viewTargetPickerPopupTag, dims.Size)
	return dims
}

func (ui *UI) layoutSettingsViewerCommandRulePicker(th *material.Theme, gtx layout.Context, st *settingsModalState, rules []fm.ViewerCommandRule, matchCount int) layout.Dimensions {
	gtx2 := gtx
	maxH := gtx.Dp(unit.Dp(156))
	if gtx2.Constraints.Max.Y > maxH {
		gtx2.Constraints.Max.Y = maxH
	}
	dims := fillRoundedBox(
		gtx2,
		gtx2.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 18, G: 22, B: 30, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			if len(rules) == 0 {
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, "No regex rules")
					lbl.Font.Typeface = ui.interfaceTypeface()
					lbl.TextSize = ui.scaleModalFontSize(9)
					lbl.Color = hintColor
					return lbl.Layout(gtx)
				})
			}
			var picked *fm.ViewerCommandRule
			removedPattern := ""
			currentPattern := strings.TrimSpace(st.viewRulePatternEdit.Text())
			dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if matchCount <= 0 || matchCount >= len(rules) {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, fmt.Sprintf("%d matching rules", matchCount))
						lbl.Font.Typeface = ui.interfaceTypeface()
						lbl.TextSize = ui.scaleModalFontSize(9)
						lbl.Color = hintColor
						lbl.MaxLines = 1
						lbl.Truncator = "..."
						return lbl.Layout(gtx)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					list := settingsPopupListStyle(th, &st.viewRulePickList)
					return list.Layout(gtx, len(rules), func(gtx layout.Context, i int) layout.Dimensions {
						rule := rules[i]
						click := st.viewerCommandRuleRowClick(rule.Pattern)
						removeClick := st.viewerCommandRuleRowRemoveClick(rule.Pattern)
						rowFocused := st.popupKeyboardMatches(settingsPopupKeyboardViewerRule, i, settingsPopupKeyboardActionRow)
						removeFocused := st.popupKeyboardMatches(settingsPopupKeyboardViewerRule, i, settingsPopupKeyboardActionRemove)
						for click.Clicked(gtx) {
							st.setPopupKeyboardFocus(settingsPopupKeyboardViewerRule, i, settingsPopupKeyboardActionRow)
							if picked == nil {
								ruleCopy := rule
								picked = &ruleCopy
							}
						}
						for removeClick.Clicked(gtx) {
							st.setPopupKeyboardFocus(settingsPopupKeyboardViewerRule, i, settingsPopupKeyboardActionRemove)
							if removedPattern == "" {
								removedPattern = rule.Pattern
							}
						}
						selected := rule.Pattern == currentPattern
						hovered := click.Hovered() || removeClick.Hovered()
						bg := color.NRGBA{A: 0}
						if selected {
							bg = color.NRGBA{R: 80, G: 120, B: 220, A: 45}
							if rowFocused || removeFocused {
								bg = color.NRGBA{R: 92, G: 132, B: 228, A: 62}
							}
						} else if rowFocused || removeFocused {
							bg = color.NRGBA{R: 74, G: 108, B: 182, A: 52}
						} else if hovered {
							bg = color.NRGBA{R: 255, G: 255, B: 255, A: 10}
						}
						return layoutSettingsPickerRowBackground(gtx, bg, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											pointer.CursorPointer.Add(gtx.Ops)
											return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													lbl := material.Body2(th, rule.Pattern)
													lbl.Font.Typeface = ui.interfaceTypeface()
													lbl.Font.Weight = font.Medium
													lbl.TextSize = ui.scaleModalFontSize(10)
													lbl.Color = txtColor
													lbl.MaxLines = 1
													lbl.Truncator = "..."
													return layoutVCenteredLabel(gtx, lbl)
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													lbl := material.Caption(th, rule.Command)
													lbl.Font.Typeface = ui.interfaceTypeface()
													lbl.TextSize = ui.scaleModalFontSize(8)
													lbl.Color = hintColor
													lbl.MaxLines = 1
													lbl.Truncator = "..."
													return layoutVCenteredLabel(gtx, lbl)
												}),
											)
										})
									}),
									layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return ui.layoutSettingsFlatRemoveButton(gtx, removeClick, removeFocused)
									}),
								)
							})
						})
					})
				}),
			)
			if removedPattern != "" {
				if st.removeViewerCommandRule(removedPattern) {
					st.errText = ""
					st.ruleInfoText = "Pending removal; Save to persist"
				}
				picked = nil
			}
			if picked != nil {
				st.applyPickedViewerCommandRule(*picked)
			}
			return dims
		},
	)
	registerSettingsPopupArea(gtx2, &st.viewRulePickerPopupTag, dims.Size)
	return dims
}

func (ui *UI) layoutSettingsColorScopeTabs(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	keys := []string{"panes", "viewer", "filenames"}
	if st.colorScopePaneClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusColorsScope)
		st.colorScopeAnim.anim.setPulse("panes", gtx.Now)
		st.setColorScope("panes", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.colorScopeViewerClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusColorsScope)
		st.colorScopeAnim.anim.setPulse("viewer", gtx.Now)
		st.setColorScope("viewer", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.colorScopeFilenameClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusColorsScope)
		st.colorScopeAnim.anim.setPulse("filenames", gtx.Now)
		st.setColorScope("filenames", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	hoverKey := ""
	if st.colorScopePaneClick.Hovered() {
		hoverKey = "panes"
	}
	if st.colorScopeViewerClick.Hovered() {
		hoverKey = "viewer"
	}
	if st.colorScopeFilenameClick.Hovered() {
		hoverKey = "filenames"
	}
	st.colorScopeAnim.anim.setHover(hoverKey, gtx.Now)
	fillPanes, animPanes := st.colorScopeAnim.fill(gtx.Now, st.colorScope, "panes")
	fillViewer, animViewer := st.colorScopeAnim.fill(gtx.Now, st.colorScope, "viewer")
	fillFilenames, animFilenames := st.colorScopeAnim.fill(gtx.Now, st.colorScope, "filenames")
	hoverPanes, hoverAnimPanes := st.colorScopeAnim.anim.hoverFill(gtx.Now, "panes")
	hoverViewer, hoverAnimViewer := st.colorScopeAnim.anim.hoverFill(gtx.Now, "viewer")
	hoverFilenames, hoverAnimFilenames := st.colorScopeAnim.anim.hoverFill(gtx.Now, "filenames")
	pulsePanes, pulseAnimPanes := st.colorScopeAnim.anim.pulseFill(gtx.Now, "panes")
	pulseViewer, pulseAnimViewer := st.colorScopeAnim.anim.pulseFill(gtx.Now, "viewer")
	pulseFilenames, pulseAnimFilenames := st.colorScopeAnim.anim.pulseFill(gtx.Now, "filenames")
	pos, animPos := st.colorScopeAnim.position(gtx.Now, st.colorScope, keys)
	focusPanes := float32(0)
	focusViewer := float32(0)
	focusFilenames := float32(0)
	if st.focus == settingsKeyboardFocusColorsScope {
		switch st.colorScope {
		case "viewer":
			focusViewer = 1
		case "filenames":
			focusFilenames = 1
		default:
			focusPanes = 1
		}
	}
	if animPanes || animViewer || animFilenames ||
		hoverAnimPanes || hoverAnimViewer || hoverAnimFilenames ||
		pulseAnimPanes || pulseAnimViewer || pulseAnimFilenames || animPos {
		gtx.Execute(op.InvalidateCmd{})
	}
	stripH := gtx.Dp(unit.Dp(22))
	if stripH < 1 {
		stripH = 1
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSlidingTabStrip(th, gtx, stripH, pos, ui.scaleModalFontSize(10), []slidingTabSpec{
				{
					Label:      "Panes",
					Click:      &st.colorScopePaneClick,
					ActiveFill: fillPanes,
					HoverFill:  hoverPanes,
					PulseFill:  pulsePanes,
					FocusFill:  focusPanes,
				},
				{
					Label:      "Viewer",
					Click:      &st.colorScopeViewerClick,
					ActiveFill: fillViewer,
					HoverFill:  hoverViewer,
					PulseFill:  pulseViewer,
					FocusFill:  focusViewer,
				},
				{
					Label:      "Filenames",
					Click:      &st.colorScopeFilenameClick,
					ActiveFill: fillFilenames,
					HoverFill:  hoverFilenames,
					PulseFill:  pulseFilenames,
					FocusFill:  focusFilenames,
				},
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, stripH+gtx.Dp(unit.Dp(2)))}
		}),
	)
}

func (st *settingsModalState) previewViewerConfig(cfg *fm.Config) *fm.Config {
	draft := fm.DefaultConfig()
	if cfg != nil {
		draft.General = cfg.General
		draft.Viewer = cfg.Viewer
		draft.Colors = cfg.Colors
	}

	palette, _ := st.draftFilePanePalette(cfg)
	draft.Colors = filePanePaletteToConfigColors(palette)

	if strings.TrimSpace(st.viewFontFamily) != "" && resources.IsBundledFontFamily(st.viewFontFamily) {
		draft.Viewer.Typeface = st.viewFontFamily
	}
	if st.viewFontSizeSp >= settingsFontSizeMin {
		draft.Viewer.FontSizeSp = st.viewFontSizeSp
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorViewerBackground)); ok {
		draft.Viewer.Background = fm.FormatHexColor(c)
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorViewerText)); ok {
		draft.Viewer.Text = fm.FormatHexColor(c)
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorViewerSelection)); ok {
		draft.Viewer.Selection = fm.FormatHexColor(c)
	}
	draft.Viewer.HexSelection = fm.NormalizeOptionalHexColor(st.colorViewerHexSelection)
	draft.Viewer.HexOffsetText = fm.NormalizeOptionalHexColor(st.colorViewerHexOffsetText)
	draft.Viewer.HexBytesText = fm.NormalizeOptionalHexColor(st.colorViewerHexBytesText)
	draft.Viewer.HexASCIIText = fm.NormalizeOptionalHexColor(st.colorViewerHexASCIIText)
	if draft.Viewer.Typeface == "" {
		draft.Viewer.Typeface = draft.General.Typeface
	}
	return draft
}

func (st *settingsModalState) previewViewerTypeface(ui *UI) font.Typeface {
	cfg := st.previewViewerConfig(nil)
	if ui != nil && ui.fmCfg != nil {
		cfg = st.previewViewerConfig(ui.fmCfg)
	}
	if cfg == nil || cfg.Viewer.Typeface == "" {
		if ui != nil {
			return ui.viewerTypeface()
		}
		return font.Typeface(resources.BundledFontFamilyFiraCodeNerdFontMono)
	}
	return font.Typeface(cfg.Viewer.Typeface)
}

func (st *settingsModalState) previewViewerTextSize(ui *UI) unit.Sp {
	cfg := st.previewViewerConfig(nil)
	if ui != nil && ui.fmCfg != nil {
		cfg = st.previewViewerConfig(ui.fmCfg)
	}
	if cfg == nil {
		if ui != nil {
			return ui.viewerTextSize()
		}
		return normalizeUIFontSize(13)
	}
	if cfg.Viewer.FontSizeSp < 6 {
		return normalizeUIFontSize(13)
	}
	return normalizeUIFontSize(unit.Sp(cfg.Viewer.FontSizeSp))
}

func (st *settingsModalState) previewViewerLineHeight(ui *UI, th *material.Theme, gtx layout.Context, monospace bool) int {
	face := st.previewViewerTypeface(ui)
	if monospace {
		face = ui.viewerMonospaceTypeface()
	}
	lineH := measureTypefaceLineHeight(ui, th, gtx, face)
	if lineH < 1 {
		lineH = 1
	}
	return lineH
}

func settingsViewerPreviewSelectionFill(theme fileViewerTheme, strong, hexMode bool) color.NRGBA {
	fill := theme.Selection
	if hexMode {
		fill = theme.HexSelection
		if strong {
			fill = theme.HexStrongSelection
		}
	} else if strong {
		fill = theme.StrongSelection
	}
	fill.A = 0xFF
	return fill
}

func settingsViewerPreviewSelectionRect(width, rowH int) image.Rectangle {
	if width <= 0 || rowH <= 0 {
		return image.Rectangle{}
	}
	return image.Rect(0, 0, width, rowH)
}

func (ui *UI) settingsViewerPreviewLabelStyle(th *material.Theme, face font.Typeface, size unit.Sp, txt string, fg color.NRGBA) material.LabelStyle {
	lbl := material.Body2(th, txt)
	lbl.Font.Typeface = face
	lbl.Font.Weight = font.Normal
	lbl.TextSize = size
	lbl.Color = fg
	lbl.MaxLines = 1
	lbl.Truncator = "..."
	return lbl
}

func (ui *UI) layoutSettingsViewerPreviewMonoCells(th *material.Theme, gtx layout.Context, size unit.Sp, text string, cellW, rowH int, fg color.NRGBA) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len([]rune(text)))
	for _, r := range text {
		cell := string(r)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, cellW, func(gtx layout.Context) layout.Dimensions {
				lineGtx := gtx
				lineGtx.Constraints.Min.Y = rowH
				lineGtx.Constraints.Max.Y = rowH
				lbl := ui.settingsViewerPreviewLabelStyle(th, ui.viewerMonospaceTypeface(), size, cell, fg)
				return layoutVCenteredLabel(lineGtx, lbl)
			})
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (ui *UI) layoutSettingsViewerPreviewTextRow(th *material.Theme, gtx layout.Context, st *settingsModalState, theme fileViewerTheme, txt string, fg color.NRGBA, selected bool) layout.Dimensions {
	rowH := st.previewViewerLineHeight(ui, th, gtx, false)
	return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rect(0, 0, gtx.Constraints.Max.X, rowH)).Push(gtx.Ops).Pop()
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				if selected {
					bg := settingsViewerPreviewSelectionFill(theme, false, false)
					if rect := settingsViewerPreviewSelectionRect(gtx.Constraints.Max.X, rowH); !rect.Empty() {
						paint.FillShape(gtx.Ops, bg, clip.Rect(rect).Op())
					}
				}
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, rowH)}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lineGtx := gtx
					lineGtx.Constraints.Min.Y = rowH
					lineGtx.Constraints.Max.Y = rowH
					lbl := ui.settingsViewerPreviewLabelStyle(th, st.previewViewerTypeface(ui), st.previewViewerTextSize(ui), txt, fg)
					return layoutVCenteredLabel(lineGtx, lbl)
				})
			}),
		)
	})
}

func (ui *UI) layoutSettingsViewerPreviewHexRow(th *material.Theme, gtx layout.Context, st *settingsModalState, theme fileViewerTheme, offset, hexText, ascii string, selected bool) layout.Dimensions {
	rowH := st.previewViewerLineHeight(ui, th, gtx, true)
	charW := measureTypefaceCharWidth(ui, th, gtx, ui.viewerMonospaceTypeface())
	if charW < 1 {
		charW = 1
	}
	leftPad := gtx.Dp(unit.Dp(6))
	if leftPad < 2 {
		leftPad = 2
	}
	columnGap := hexSectionColumnGap(gtx, charW)
	offsetDigits := len(strings.TrimSpace(offset))
	if offsetDigits < 8 {
		offsetDigits = 8
	}
	bytesPerLine := len(strings.Fields(hexText))
	if asciiChars := len([]rune(ascii)); asciiChars > bytesPerLine {
		bytesPerLine = asciiChars
	}
	if bytesPerLine < 1 {
		bytesPerLine = 1
	}
	offsetW := offsetDigits * charW
	hexW := hexLineColumns(bytesPerLine, 0) * charW
	asciiW := bytesPerLine * charW
	offsetColor := theme.OffsetText
	hexColor := theme.HexText
	asciiColor := theme.ASCIIText
	return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rect(0, 0, gtx.Constraints.Max.X, rowH)).Push(gtx.Ops).Pop()
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, leftPad, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(leftPad, rowH)}
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, offsetW, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsViewerPreviewMonoCells(th, gtx, st.previewViewerTextSize(ui), offset, charW, rowH, offsetColor)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, columnGap, func(gtx layout.Context) layout.Dimensions {
					x := gtx.Constraints.Max.X / 2
					paint.FillShape(gtx.Ops, theme.Separator, clip.Rect(image.Rect(x, 0, x+1, rowH)).Op())
					return layout.Dimensions{Size: image.Pt(columnGap, rowH)}
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, hexW, func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{}.Layout(gtx,
						layout.Expanded(func(gtx layout.Context) layout.Dimensions {
							if selected {
								bg := settingsViewerPreviewSelectionFill(theme, false, true)
								if rect := settingsViewerPreviewSelectionRect(gtx.Constraints.Max.X, rowH); !rect.Empty() {
									paint.FillShape(gtx.Ops, bg, clip.Rect(rect).Op())
								}
							}
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, rowH)}
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsViewerPreviewMonoCells(th, gtx, st.previewViewerTextSize(ui), hexText, charW, rowH, hexColor)
						}),
					)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, columnGap, func(gtx layout.Context) layout.Dimensions {
					x := gtx.Constraints.Max.X / 2
					paint.FillShape(gtx.Ops, theme.Separator, clip.Rect(image.Rect(x, 0, x+1, rowH)).Op())
					return layout.Dimensions{Size: image.Pt(columnGap, rowH)}
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, asciiW, func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{}.Layout(gtx,
						layout.Expanded(func(gtx layout.Context) layout.Dimensions {
							if selected {
								bg := settingsViewerPreviewSelectionFill(theme, true, true)
								if rect := settingsViewerPreviewSelectionRect(gtx.Constraints.Max.X, rowH); !rect.Empty() {
									paint.FillShape(gtx.Ops, bg, clip.Rect(rect).Op())
								}
							}
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, rowH)}
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsViewerPreviewMonoCells(th, gtx, st.previewViewerTextSize(ui), ascii, charW, rowH, asciiColor)
						}),
					)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, rowH)}
			}),
		)
	})
}

func (st *settingsModalState) normalizedViewerPreviewMode() string {
	if st != nil && st.viewerPreviewMode == "hex" {
		return "hex"
	}
	return "file"
}

func (ui *UI) layoutSettingsViewerPreviewModeToggle(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	if st.viewerPreviewFileClick.Clicked(gtx) {
		st.viewerPreviewModeAnim.anim.setPulse("file", gtx.Now)
		st.viewerPreviewModeAnim.setValue(&st.viewerPreviewMode, "file", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.viewerPreviewHexClick.Clicked(gtx) {
		st.viewerPreviewModeAnim.anim.setPulse("hex", gtx.Now)
		st.viewerPreviewModeAnim.setValue(&st.viewerPreviewMode, "hex", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	hoverKey := ""
	if st.viewerPreviewFileClick.Hovered() {
		hoverKey = "file"
	} else if st.viewerPreviewHexClick.Hovered() {
		hoverKey = "hex"
	}
	st.viewerPreviewModeAnim.anim.setHover(hoverKey, gtx.Now)
	mode := st.normalizedViewerPreviewMode()
	fileFill, fileAnim := st.viewerPreviewModeAnim.fill(gtx.Now, mode, "file")
	hexFill, hexAnim := st.viewerPreviewModeAnim.fill(gtx.Now, mode, "hex")
	fileHover, fileHoverAnim := st.viewerPreviewModeAnim.anim.hoverFill(gtx.Now, "file")
	hexHover, hexHoverAnim := st.viewerPreviewModeAnim.anim.hoverFill(gtx.Now, "hex")
	filePulse, filePulseAnim := st.viewerPreviewModeAnim.anim.pulseFill(gtx.Now, "file")
	hexPulse, hexPulseAnim := st.viewerPreviewModeAnim.anim.pulseFill(gtx.Now, "hex")
	pos, posAnim := st.viewerPreviewModeAnim.position(gtx.Now, mode, []string{"file", "hex"})
	if fileAnim || hexAnim || fileHoverAnim || hexHoverAnim || filePulseAnim || hexPulseAnim || posAnim {
		gtx.Execute(op.InvalidateCmd{})
	}
	stripH := gtx.Dp(unit.Dp(20))
	if stripH < 1 {
		stripH = 1
	}
	return ui.layoutSlidingTabStrip(th, gtx, stripH, pos, ui.scaleModalFontSize(9), []slidingTabSpec{
		{Label: "File", Click: &st.viewerPreviewFileClick, ActiveFill: fileFill, HoverFill: fileHover, PulseFill: filePulse},
		{Label: "Hex", Click: &st.viewerPreviewHexClick, ActiveFill: hexFill, HoverFill: hexHover, PulseFill: hexPulse},
	})
}

func (st *settingsModalState) previewViewerContentHeight(ui *UI, th *material.Theme, gtx layout.Context) int {
	lineH := st.previewViewerLineHeight(ui, th, gtx, st.normalizedViewerPreviewMode() == "hex")
	return lineH * 4
}

func settingsColorsPreviewHostHeight(gtx layout.Context) int {
	height := gtx.Dp(unit.Dp(188))
	if minHeight := gtx.Dp(unit.Dp(164)); height < minHeight {
		height = minHeight
	}
	if height < 1 {
		height = 1
	}
	return height
}

func (ui *UI) settingsPanePreviewHostHeight(gtx layout.Context) int {
	rowH := ui.settingsColorPreviewRowHeight(gtx)
	currentDirH := ui.settingsColorPreviewCurrentDirHeight(gtx)
	titleH := gtx.Sp(ui.scaleModalFontSize(9))
	height := gtx.Dp(unit.Dp(16+6+10+6)) + titleH + rowH*6 + currentDirH
	if minHeight := settingsColorsPreviewHostHeight(gtx); height < minHeight {
		height = minHeight
	}
	return height
}

func (ui *UI) layoutSettingsViewerPreviewContent(th *material.Theme, gtx layout.Context, st *settingsModalState, theme fileViewerTheme, previewUI *UI) layout.Dimensions {
	if st.normalizedViewerPreviewMode() == "hex" {
		return fixedWidth(gtx, gtx.Constraints.Max.X, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return previewUI.layoutSettingsViewerPreviewHexRow(th, gtx, st, theme, "00000000", "48 65 78 6F 6E 65", "Hexone", false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return previewUI.layoutSettingsViewerPreviewHexRow(th, gtx, st, theme, "00000006", "20 76 69 65 77 65", " viewe", true)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return previewUI.layoutSettingsViewerPreviewHexRow(th, gtx, st, theme, "0000000C", "72 20 70 72 65 76", "r prev", false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return previewUI.layoutSettingsViewerPreviewHexRow(th, gtx, st, theme, "00000012", "69 65 77 0A 00 FF", "iew...", false)
				}),
			)
		})
	}
	return fixedWidth(gtx, gtx.Constraints.Max.X, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return previewUI.layoutSettingsViewerPreviewTextRow(th, gtx, st, theme, "hexone viewer preview", theme.Text, false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return previewUI.layoutSettingsViewerPreviewTextRow(th, gtx, st, theme, "keyboard-first workflow", theme.Text, true)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return previewUI.layoutSettingsViewerPreviewTextRow(th, gtx, st, theme, "open files with Enter", theme.Text, false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return previewUI.layoutSettingsViewerPreviewTextRow(th, gtx, st, theme, "Tab cycles viewer modes", theme.Text, false)
			}),
		)
	})
}

func (ui *UI) layoutSettingsViewerPreview(th *material.Theme, gtx layout.Context, st *settingsModalState, theme fileViewerTheme) layout.Dimensions {
	previewCfg := st.previewViewerConfig(ui.fmCfg)
	previewUI := *ui
	previewUI.fmCfg = previewCfg
	previewUI.typeface = font.Typeface(previewCfg.General.Typeface)
	previewMode := st.normalizedViewerPreviewMode()
	previewName := "README.md"
	if previewMode == "hex" {
		previewName = "sample.bin"
	}
	previewState := &fileViewerState{
		mode: previewMode,
		name: previewName,
	}

	height := settingsColorsPreviewHostHeight(gtx)
	return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedClipBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneControlCornerDp)),
			theme.PanelBg,
			theme.PanelBorder,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(0), Right: unit.Dp(0), Top: unit.Dp(0), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return previewUI.layoutSettingsViewerPreviewHeader(th, gtx, previewState, theme)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(10), Top: unit.Dp(4), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return previewUI.layoutSettingsViewerPreviewContent(th, gtx, st, theme, &previewUI)
									}),
									layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return previewUI.layoutSettingsViewerPreviewScrollbar(gtx, theme)
									}),
								)
							})
						}),
					)
				})
			},
		)
	})
}

func (ui *UI) layoutSettingsViewerPreviewHeader(th *material.Theme, gtx layout.Context, previewState *fileViewerState, theme fileViewerTheme) layout.Dimensions {
	stripH := ui.viewerHeaderStripHeight(gtx)
	_ = theme
	return ui.layoutFileViewerHeaderRow(th, gtx, previewState, stripH)
}

func (ui *UI) layoutSettingsViewerPreviewScrollbar(gtx layout.Context, theme fileViewerTheme) layout.Dimensions {
	trackW := gtx.Dp(unit.Dp(8))
	trackH := gtx.Dp(unit.Dp(88))
	if trackW < 6 {
		trackW = 6
	}
	if trackH < 56 {
		trackH = 56
	}
	thumbH := trackH / 4
	if thumbH < 18 {
		thumbH = 18
	}
	track, thumb := settingsPreviewScrollbarGeometry(trackW, trackH, thumbH)
	return fixedWidth(gtx, trackW, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, trackH, func(gtx layout.Context) layout.Dimensions {
			paintSettingsPreviewRoundedRect(gtx, track, theme.ScrollTrackHover)
			paintSettingsPreviewRoundedRect(gtx, thumb, theme.ScrollThumbHover)
			return layout.Dimensions{Size: image.Pt(trackW, trackH)}
		})
	})
}

func settingsPreviewScrollbarGeometry(trackW, trackH, thumbH int) (track, thumb image.Rectangle) {
	if trackW < 1 {
		trackW = 1
	}
	if trackH < 1 {
		trackH = 1
	}
	if thumbH < 1 {
		thumbH = 1
	}
	if thumbH > trackH {
		thumbH = trackH
	}
	inset := 1
	if trackW <= 2 {
		inset = 0
	}
	thumbY := (trackH - thumbH) / 2
	track = image.Rect(0, 0, trackW, trackH)
	thumb = image.Rect(inset, thumbY, trackW-inset, thumbY+thumbH)
	if thumb.Empty() {
		thumb = track
	}
	return track, thumb
}

func paintSettingsPreviewRoundedRect(gtx layout.Context, rect image.Rectangle, fill color.NRGBA) {
	if fill.A == 0 || rect.Empty() {
		return
	}
	radius := rect.Dx()
	if rect.Dy() < radius {
		radius = rect.Dy()
	}
	radius /= 2
	if radius < 1 {
		radius = 1
	}
	paint.FillShape(gtx.Ops, fill, clip.UniformRRect(rect, radius).Op(gtx.Ops))
}

func (ui *UI) layoutSettingsColorsTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	list := settingsScrollableListStyle(th, &st.colorsTabList)
	return list.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Inset{Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorsTabContent(th, gtx, st)
		})
	})
}

func (ui *UI) layoutSettingsColorsTabContent(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	if st.colorScope == "filenames" {
		return ui.layoutSettingsFilenameColorsTab(th, gtx, st)
	}
	options := settingsColorOptionsForScope(st.colorScope)
	st.ensureColorOptionClicks(len(options))

	for {
		ev, ok := st.colorValueEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.setColorValue(st.colorCategory, st.colorValueEdit.Text())
			st.errText = ""
		}
	}
	for {
		ev, ok := st.colorTextValueEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.setColorTextValue(st.colorCategory, st.colorTextValueEdit.Text())
			st.errText = ""
		}
	}
	bgSwatchGroups := st.colorPickerSwatchGroups("background")
	textSwatchGroups := st.colorPickerSwatchGroups("text")
	activeSwatchGroups := bgSwatchGroups
	if st.colorPickerOpen {
		activeSwatchGroups = st.colorPickerSwatchGroups(st.colorPickerTarget)
	}
	st.ensureColorSwatchClicks(settingsColorSwatchCount(activeSwatchGroups))

	if st.colorCategoryOpen {
		for i, opt := range options {
			if i >= len(st.colorOptionClicks) {
				break
			}
			if st.colorOptionClicks[i].Clicked(gtx) {
				st.setPopupKeyboardFocus(settingsPopupKeyboardColorCategory, i, settingsPopupKeyboardActionRow)
				st.setColorCategory(opt.key)
				st.errText = ""
			}
		}
	}
	st.handleColorPickerActions(gtx, activeSwatchGroups)
	if st.colorCategoryClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusColorsCategory)
		if st.colorCategoryOpen {
			st.closeSettingsPopupsExcept("")
		} else {
			st.openColorCategoryPopup(gtx.Now)
		}
	}
	if st.colorBgPickerClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusColorsBgPicker)
		st.toggleColorPicker("background")
	}
	if st.colorTextPickerClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusColorsTextPicker)
		st.toggleColorPicker("text")
	}

	previewPalette, panePreviewErr := st.draftFilePanePalette(ui.fmCfg)
	previewTheme, viewerPreviewErr := st.draftViewerTheme(ui.fmCfg)
	currentBg := settingsPreviewColorForCategory(previewPalette, st.colorCategory, "background")
	currentText := settingsPreviewColorForCategory(previewPalette, st.colorCategory, "text")
	previewErr := panePreviewErr
	if st.colorScope == "viewer" {
		switch st.colorCategory {
		case "selection":
			currentBg = previewTheme.Selection
		case "hex_selection":
			currentBg = previewTheme.HexSelection
		case "hex_offset":
			currentBg = previewTheme.OffsetText
		case "hex_bytes":
			currentBg = previewTheme.HexText
		case "hex_ascii":
			currentBg = previewTheme.ASCIIText
		default:
			currentBg = previewTheme.PanelBg
		}
		currentText = previewTheme.Text
		previewErr = viewerPreviewErr
	}
	if parsed, ok := fm.ParseHexColor(strings.TrimSpace(st.colorValue(st.colorCategory))); ok {
		currentBg = parsed
	}
	if parsed, ok := fm.ParseHexColor(strings.TrimSpace(st.colorTextValue(st.colorCategory))); ok {
		currentText = parsed
	}
	showTextField := st.colorScope != "viewer" || settingsViewerCategoryHasText(st.colorCategory)
	bgFieldLabel := "Background"
	textFieldLabel := "Text"
	allowTransparentText := st.colorScope == "panes" && settingsPaneTextAllowsTransparent(st.colorCategory)
	st.syncColorTextTransparentCheckbox()
	if st.colorScope == "viewer" && strings.HasPrefix(st.colorCategory, "hex_") {
		bgFieldLabel = "Text"
	}
	if st.colorScope == "panes" && st.colorCategory == "scrollbar" {
		bgFieldLabel = "Thumb"
		textFieldLabel = "Track"
	}

	rowLabel := func(txt string, enabled bool) layout.Widget {
		return settingsViewerRowLabel(ui, th, txt, enabled)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(rowLabel("Palette", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorScopeTabs(th, gtx, st)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(rowLabel("Color target", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorCategoryField(th, gtx, st, options)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := "Colors (#RRGGBB)"
			if allowTransparentText {
				label = "Colors (#RRGGBB or transparent)"
			}
			return rowLabel(label, true)(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsColorValueField(th, gtx, st, bgFieldLabel, currentBg, &st.colorValueEdit, &st.colorBgPickerClick, "background", bgSwatchGroups, settingsKeyboardFocusColorsBgPicker, settingsKeyboardFocusColorsValue)
				}),
			}
			if showTextField {
				children = append(children,
					layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.layoutSettingsColorValueField(th, gtx, st, textFieldLabel, currentText, &st.colorTextValueEdit, &st.colorTextPickerClick, "text", textSwatchGroups, settingsKeyboardFocusColorsTextPicker, settingsKeyboardFocusColorsTextValue)
					}),
				)
				if allowTransparentText {
					children = append(children,
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsColorTransparentCheckbox(th, gtx, st)
						}),
					)
				}
			}
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
			}))
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx, children...)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			note := "Use the same category for both background and text. Popup Hover controls menu and submenu row hover colors."
			if st.colorScope == "viewer" {
				note = "File and Hex selection backgrounds can be set separately. Leave Hex Selection empty to derive it from Selection."
			} else if st.colorCategory == "scrollbar" {
				note = "Leave scrollbar fields empty to derive contrast from the active pane palette."
			} else if allowTransparentText {
				note = "Use transparent for Text to keep filename color customizations visible on this row state."
			}
			lbl := material.Caption(th, note)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleModalFontSize(9)
			lbl.Color = hintColor
			lbl.MaxLines = 2
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if previewErr == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, previewErr)
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.TextSize = ui.scaleModalFontSize(9)
				lbl.Color = color.NRGBA{R: 220, G: 140, B: 140, A: 255}
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(rowLabel("Preview", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.colorScope != "viewer" {
				return layout.Dimensions{}
			}
			return ui.layoutSettingsViewerPreviewModeToggle(th, gtx, st)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.colorScope == "viewer" {
				return layout.Spacer{Height: unit.Dp(4)}.Layout(gtx)
			}
			return layout.Dimensions{}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			hostH := settingsColorsPreviewHostHeight(gtx)
			if st.colorScope != "viewer" {
				hostH = ui.settingsPanePreviewHostHeight(gtx)
			}
			hostBg := previewPalette.PaneBg
			if st.colorScope == "viewer" {
				hostBg = previewTheme.PanelBg
			}
			return fixedHeight(gtx, hostH, func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						if gtx.Constraints.Max.X > 0 && hostH > 0 {
							paint.FillShape(gtx.Ops, hostBg, clip.Rect(image.Rect(0, 0, gtx.Constraints.Max.X, hostH)).Op())
						}
						return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, hostH)}
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						if st.colorScope == "viewer" {
							return ui.layoutSettingsViewerPreview(th, gtx, st, previewTheme)
						}
						return ui.layoutSettingsColorPreview(th, gtx, previewPalette)
					}),
				)
			})
		}),
	)
}

func (ui *UI) layoutSettingsColorCategoryField(th *material.Theme, gtx layout.Context, st *settingsModalState, options []settingsColorOption) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	width := settingsColorCategoryWidth(th, gtx, ui.fmCfg, ui.interfaceTypeface(), options)
	var btnH int
	dims := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d := ui.layoutSettingsColorCategoryButton(th, gtx, st, width)
			btnH = d.Size.Y
			return d
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, btnH)}
		}),
	)
	if st.colorCategoryOpen {
		alpha, offsetY, animating := popupOpenProgress(gtx.Now, st.colorCategoryOpenedAt)
		if animating {
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
		}
		m := op.Record(gtx.Ops)
		offset := op.Offset(image.Pt(0, btnH+gtx.Dp(unit.Dp(4))+offsetY))
		offset.Add(gtx.Ops)
		ui.layoutSettingsColorCategoryPopup(th, gtx, st, width, alpha, options)
		op.Defer(gtx.Ops, m.Stop())
	}
	return dims
}

func settingsColorCategoryWidth(th *material.Theme, gtx layout.Context, cfg *fm.Config, face font.Typeface, options []settingsColorOption) int {
	maxTextW := 0
	for _, opt := range options {
		lbl := material.Body2(th, opt.label+"  ▾")
		lbl.Font.Typeface = face
		lbl.TextSize = scaleModalConfigFontSize(cfg, 10)
		lbl.MaxLines = 1
		w := measureLabelUnconstrained(gtx, lbl).Size.X
		if w > maxTextW {
			maxTextW = w
		}
	}
	width := maxTextW + gtx.Dp(unit.Dp(24))
	minW := gtx.Dp(unit.Dp(168))
	if width < minW {
		width = minW
	}
	if max := gtx.Constraints.Max.X; max > 0 && width > max {
		width = max
	}
	if width < 1 {
		width = 1
	}
	return width
}

func (ui *UI) layoutSettingsColorCategoryButton(th *material.Theme, gtx layout.Context, st *settingsModalState, width int) layout.Dimensions {
	label := settingsColorLabel(st.colorScope, st.colorCategory) + "  ▾"
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		dims := st.colorCategoryClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			bg := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
			bd := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
			if st.colorCategoryClick.Hovered() || st.colorCategoryOpen {
				bg = color.NRGBA{R: 30, G: 34, B: 44, A: 255}
				bd = color.NRGBA{R: 130, G: 160, B: 255, A: 70}
			}
			if st.focus == settingsKeyboardFocusColorsCategory {
				bg = mixNRGBA(bg, color.NRGBA{R: 64, G: 54, B: 36, A: 255}, 0.32)
				bd = color.NRGBA{R: 214, G: 196, B: 164, A: 190}
			}
			return fillFlatBox(gtx, bg, bd, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, label)
					lbl.Font.Typeface = ui.interfaceTypeface()
					lbl.TextSize = ui.scaleModalFontSize(10)
					lbl.Color = txtColor
					lbl.MaxLines = 1
					return layoutVCenteredLabel(gtx, lbl)
				})
			})
		})
		if dims.Size.X > 0 && dims.Size.Y > 0 {
			defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return dims
	})
}

func (st *settingsModalState) hoveredColorCategoryKey() string {
	if st == nil {
		return ""
	}
	options := settingsColorOptionsForScope(st.colorScope)
	hoverID := ""
	for i, opt := range options {
		if i < len(st.colorOptionClicks) && st.colorOptionClicks[i].Hovered() {
			hoverID = opt.key
		}
	}
	return hoverID
}

func (ui *UI) layoutSettingsColorCategoryPopup(th *material.Theme, gtx layout.Context, st *settingsModalState, width int, alpha float32, options []settingsColorOption) layout.Dimensions {
	gtx2 := gtx
	maxH := gtx.Dp(unit.Dp(360))
	if gtx2.Constraints.Max.Y > maxH {
		gtx2.Constraints.Max.Y = maxH
	}
	theme := ui.filePanePopupTheme()
	hoverID := st.hoveredColorCategoryKey()
	if hoverID != st.colorCategoryHoverID {
		st.colorCategoryHoverID = hoverID
		st.colorCategoryHoverAnim.setHover(hoverID, gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	return fixedWidth(gtx2, width, func(gtx layout.Context) layout.Dimensions {
		dims := fillRoundedClipBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			scaleColorAlpha(theme.Bg, alpha),
			scaleColorAlpha(theme.Border, alpha),
			func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(options)+1)
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, ui.fileContextMenuTitleHeight(gtx), func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(4), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Caption(th, "Color Target")
							lbl.Font.Typeface = ui.interfaceTypeface()
							lbl.Font.Weight = font.Medium
							lbl.TextSize = scaleConfigFontSize(ui.fmCfg, 9)
							lbl.Color = scaleColorAlpha(theme.Title, alpha)
							lbl.MaxLines = 1
							lbl.Truncator = "…"
							return layoutVCenteredLabel(gtx, lbl)
						})
					})
				}))
				for i, opt := range options {
					i := i
					opt := opt
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						hoverFill, animating := st.colorCategoryHoverAnim.hoverFill(gtx.Now, opt.key)
						selected := st.popupKeyboardMatches(settingsPopupKeyboardColorCategory, i, settingsPopupKeyboardActionRow)
						if selected && hoverFill < 0.95 {
							hoverFill = 0.95
						}
						if animating {
							gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
						}
						item := fileContextMenuItem{ID: opt.key, Label: opt.label}
						dims, _, _ := ui.layoutFilePaneContextMenuItem(
							th,
							gtx,
							theme,
							&st.colorOptionClicks[i],
							item,
							st.colorCategory == opt.key,
							hoverFill,
							alpha,
							ui.fileContextMenuRowHeight(gtx, item),
						)
						if selected && dims.Size.X > 0 && dims.Size.Y > 0 {
							yPad := gtx.Dp(unit.Dp(3))
							if yPad*2 >= dims.Size.Y {
								yPad = 0
							}
							w := gtx.Dp(unit.Dp(3))
							if w < 1 {
								w = 1
							}
							x := gtx.Dp(unit.Dp(2))
							if x+w > dims.Size.X {
								x = 0
							}
							rect := image.Rect(x, yPad, x+w, dims.Size.Y-yPad)
							if rect.Dx() > 0 && rect.Dy() > 0 {
								accent := scaleColorAlpha(mixNRGBA(theme.ActiveText, theme.ActiveBg, 0.16), alpha)
								paint.FillShape(gtx.Ops, accent, clip.UniformRRect(rect, w).Op(gtx.Ops))
							}
						}
						return dims
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			},
		)
		registerSettingsPopupArea(gtx, &st.colorCategoryPopupTag, dims.Size)
		return dims
	})
}

func (ui *UI) layoutSettingsColorCategoryOption(th *material.Theme, gtx layout.Context, click *widget.Clickable, label string, selected bool) layout.Dimensions {
	dims := click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{}
		if selected {
			bg = color.NRGBA{R: 68, G: 92, B: 180, A: 54}
		} else if click.Hovered() {
			bg = color.NRGBA{R: 255, G: 255, B: 255, A: 10}
		}
		return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.TextSize = ui.scaleModalFontSize(10)
				lbl.Color = txtColor
				lbl.MaxLines = 1
				return layoutVCenteredLabel(gtx, lbl)
			})
		})
	})
	if dims.Size.X > 0 && dims.Size.Y > 0 {
		defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	}
	return dims
}

func (ui *UI) layoutSettingsColorValueField(th *material.Theme, gtx layout.Context, st *settingsModalState, label string, swatch color.NRGBA, edit *widget.Editor, picker *widget.Clickable, pickerTarget string, groups []settingsColorSwatchGroup, pickerFocusTarget, editorFocusTarget settingsKeyboardFocus) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	btnW := settingsColorPickerButtonWidth(th, gtx, ui.fmCfg, ui.interfaceTypeface())
	edW := settingsColorHexEditorWidth(th, gtx, ui.fmCfg, ui.interfaceTypeface())
	editorFocused := gtx.Focused(edit) || st.focus == editorFocusTarget || st.focusPending == editorFocusTarget
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, label)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleModalFontSize(9)
			lbl.Color = hintColor
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsColorPickerButton(th, gtx, st, swatch, picker, st.colorPickerOpen && st.colorPickerTarget == pickerTarget, st.focus == pickerFocusTarget, btnW)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, edW, func(gtx layout.Context) layout.Dimensions {
						ed := material.Editor(th, edit, "#RRGGBB")
						ed.Font.Typeface = ui.interfaceTypeface()
						ed.TextSize = ui.scaleModalFontSize(10)
						ed.Color = txtColor
						ed.HintColor = hintColor
						dims := ui.layoutEditorWithContextMenu(th, gtx, "settings-color-"+pickerTarget, edit, true, func(gtx layout.Context) layout.Dimensions {
							return layoutNeutralEditorBox(gtx, editorFocused, true, ed.Layout)
						})
						st.applyPendingWidgetFocus(gtx, editorFocusTarget, edit)
						return dims
					})
				}),
			)
		}),
	)
	if st.colorPickerOpen && st.colorPickerTarget == pickerTarget {
		m := op.Record(gtx.Ops)
		offset := op.Offset(image.Pt(0, dims.Size.Y+gtx.Dp(unit.Dp(4))))
		offset.Add(gtx.Ops)
		ui.layoutSettingsColorPickerPopup(th, gtx, st, groups)
		op.Defer(gtx.Ops, m.Stop())
	}
	return dims
}

func (ui *UI) layoutSettingsColorTransparentCheckbox(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	captionH := settingsColorValueCaptionHeight(th, gtx, ui.fmCfg, ui.interfaceTypeface())
	controlH := settingsColorValueControlHeight(th, gtx, ui.fmCfg, ui.interfaceTypeface())
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(0, captionH)}
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, controlH, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					before := st.colorTextTransparentBool.Value
					dims := ui.layoutThemeCheckbox(th, gtx, &st.colorTextTransparentBool, "Transparent", ui.scaleModalFontSize(10))
					if st.colorTextTransparentBool.Value != before {
						st.focus = settingsKeyboardFocusColorsTextTransparent
						st.setColorTextTransparent(st.colorTextTransparentBool.Value)
						st.closeSettingsPopupsExcept("")
					}
					st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusColorsTextTransparent, &st.colorTextTransparentBool)
					return dims
				})
			})
		}),
	)
}

func settingsColorValueCaptionHeight(th *material.Theme, gtx layout.Context, cfg *fm.Config, face font.Typeface) int {
	lbl := material.Caption(th, "Text")
	lbl.Font.Typeface = face
	lbl.TextSize = scaleModalConfigFontSize(cfg, 9)
	lbl.MaxLines = 1
	return measureLabelUnconstrained(gtx, lbl).Size.Y
}

func settingsColorValueControlHeight(th *material.Theme, gtx layout.Context, cfg *fm.Config, face font.Typeface) int {
	lbl := material.Body2(th, "Pick  ▾")
	lbl.Font.Typeface = face
	lbl.TextSize = scaleModalConfigFontSize(cfg, 10)
	lbl.MaxLines = 1
	contentH := measureLabelUnconstrained(gtx, lbl).Size.Y
	swatchH := gtx.Dp(unit.Dp(14))
	if contentH < swatchH {
		contentH = swatchH
	}
	return contentH + gtx.Dp(unit.Dp(8))
}

func settingsColorPickerButtonWidth(th *material.Theme, gtx layout.Context, cfg *fm.Config, face font.Typeface) int {
	lbl := material.Body2(th, "Pick  ▾")
	lbl.Font.Typeface = face
	lbl.TextSize = scaleModalConfigFontSize(cfg, 10)
	lbl.MaxLines = 1
	textW := measureLabelUnconstrained(gtx, lbl).Size.X
	width := gtx.Dp(unit.Dp(6)) + gtx.Dp(unit.Dp(14)) + gtx.Dp(unit.Dp(8)) + textW + gtx.Dp(unit.Dp(6))
	minW := gtx.Dp(unit.Dp(90))
	if width < minW {
		width = minW
	}
	return width
}

func settingsColorHexEditorWidth(th *material.Theme, gtx layout.Context, cfg *fm.Config, face font.Typeface) int {
	lbl := material.Body2(th, "#RRGGBB")
	lbl.Font.Typeface = face
	lbl.TextSize = scaleModalConfigFontSize(cfg, 10)
	lbl.MaxLines = 1
	width := measureLabelUnconstrained(gtx, lbl).Size.X + gtx.Dp(unit.Dp(28))
	minW := gtx.Dp(unit.Dp(104))
	if width < minW {
		width = minW
	}
	return width
}

func (ui *UI) layoutSettingsColorPickerButton(th *material.Theme, gtx layout.Context, _ *settingsModalState, swatch color.NRGBA, picker *widget.Clickable, open, focused bool, width int) layout.Dimensions {
	label := "Pick  ▾"
	if open {
		label = "Pick  ▴"
	}
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		dims := picker.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			bg := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
			bd := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
			if picker.Hovered() || open {
				bg = color.NRGBA{R: 30, G: 34, B: 44, A: 255}
				bd = color.NRGBA{R: 130, G: 160, B: 255, A: 70}
			}
			if focused {
				bg = mixNRGBA(bg, color.NRGBA{R: 64, G: 54, B: 36, A: 255}, 0.32)
				bd = color.NRGBA{R: 214, G: 196, B: 164, A: 190}
			}
			return fillFlatBox(gtx, bg, bd, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							border := color.NRGBA{R: 255, G: 255, B: 255, A: 28}
							if focused {
								border = color.NRGBA{R: 244, G: 236, B: 220, A: 168}
							}
							return fillRoundedBox(gtx, gtx.Dp(unit.Dp(4)), swatch, border, func(gtx layout.Context) layout.Dimensions {
								size := gtx.Dp(unit.Dp(14))
								if size < 1 {
									size = 1
								}
								return layout.Dimensions{Size: image.Pt(size, size)}
							})
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body2(th, label)
							lbl.Font.Typeface = ui.interfaceTypeface()
							lbl.TextSize = ui.scaleModalFontSize(10)
							lbl.Color = txtColor
							lbl.MaxLines = 1
							return layoutVCenteredLabel(gtx, lbl)
						}),
					)
				})
			})
		})
		if dims.Size.X > 0 && dims.Size.Y > 0 {
			defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return dims
	})
}

func (ui *UI) layoutSettingsColorPickerPopup(th *material.Theme, gtx layout.Context, st *settingsModalState, groups []settingsColorSwatchGroup) layout.Dimensions {
	st.colorPickerShade.Update(gtx)
	current := settingsColorShade(st.colorPickerBase, st.colorPickerShade.Value)
	width := settingsColorPickerPopupWidth(gtx)
	if max := gtx.Constraints.Max.X; max > 0 && width > max {
		width = max
	}
	if width < 1 {
		width = 1
	}
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		dims := fillRoundedBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneControlCornerDp)),
			color.NRGBA{R: 18, G: 22, B: 30, A: 255},
			color.NRGBA{R: 255, G: 255, B: 255, A: 18},
			func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					clickIdx := 0
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsColorHive(gtx, st, groups, st.colorPickerBase, &clickIdx)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsColorShadeSlider(gtx, st)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsColorPickerCommit(th, gtx, st, current)
						}),
					)
				})
			},
		)
		registerSettingsPopupArea(gtx, &st.colorPickerPopupTag, dims.Size)
		return dims
	})
}

func settingsColorPickerPopupWidth(gtx layout.Context) int {
	swatch := gtx.Dp(unit.Dp(17))
	inset := gtx.Dp(unit.Dp(6))
	width := inset*2 + swatch*(settingsColorHiveRadius*2+1)
	if width < 1 {
		width = 1
	}
	return width
}

func (ui *UI) layoutSettingsColorHive(gtx layout.Context, st *settingsModalState, groups []settingsColorSwatchGroup, current string, clickIdx *int) layout.Dimensions {
	cellW := gtx.Dp(unit.Dp(17))
	cellH := gtx.Dp(unit.Dp(19))
	rowStep := gtx.Dp(unit.Dp(14))
	maxColumns := settingsColorHiveRadius*2 + 1
	gridWidth := maxColumns * cellW
	width := gridWidth
	if gtx.Constraints.Max.X > width {
		width = gtx.Constraints.Max.X
	}
	baseX := (width - gridWidth) / 2
	height := cellH
	if len(groups) > 1 {
		height += (len(groups) - 1) * rowStep
	}
	for row, group := range groups {
		x := baseX + (maxColumns-len(group.hexes))*cellW/2
		y := row * rowStep
		for _, hex := range group.hexes {
			swIdx := *clickIdx
			*clickIdx = *clickIdx + 1
			selected := strings.EqualFold(current, fm.NormalizeHexColor(hex, hex))
			focused := st.popupKeyboardMatches(settingsPopupKeyboardColor, swIdx, settingsPopupKeyboardActionRow)
			if st.popupFocusKind == settingsPopupKeyboardColor {
				selected = false
			}
			cellGtx := gtx
			cellGtx.Constraints = layout.Exact(image.Pt(cellW, cellH))
			offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
			ui.layoutSettingsColorHexSwatch(cellGtx, &st.colorSwatchClicks[swIdx], parseConfigColorHexFallback(hex, fm.DefaultFilePaneBackgroundHex), selected, focused)
			offset.Pop()
			x += cellW
		}
	}
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func (ui *UI) layoutSettingsColorShadeSlider(gtx layout.Context, st *settingsModalState) layout.Dimensions {
	w := gtx.Constraints.Max.X
	if w < 1 {
		w = 1
	}
	h := gtx.Dp(unit.Dp(22))
	margin := gtx.Dp(unit.Dp(6))
	trackH := gtx.Dp(unit.Dp(7))
	track := image.Rect(margin, (h-trackH)/2, w-margin, (h+trackH)/2)
	base := parseConfigColorHexFallback(st.colorPickerBase, fm.DefaultFilePaneSelectionHex)
	mid := track.Min.X + track.Dx()/2

	trackClip := clip.UniformRRect(track, trackH/2).Push(gtx.Ops)
	leftClip := clip.Rect(image.Rect(track.Min.X, track.Min.Y, mid, track.Max.Y)).Push(gtx.Ops)
	paint.LinearGradientOp{
		Stop1:  f32.Pt(float32(track.Min.X), 0),
		Color1: color.NRGBA{A: 255},
		Stop2:  f32.Pt(float32(mid), 0),
		Color2: base,
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	leftClip.Pop()
	rightClip := clip.Rect(image.Rect(mid, track.Min.Y, track.Max.X, track.Max.Y)).Push(gtx.Ops)
	paint.LinearGradientOp{
		Stop1:  f32.Pt(float32(mid), 0),
		Color1: base,
		Stop2:  f32.Pt(float32(track.Max.X), 0),
		Color2: color.NRGBA{R: 255, G: 255, B: 255, A: 255},
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	rightClip.Pop()
	trackClip.Pop()

	sliderGtx := gtx
	sliderGtx.Constraints = layout.Exact(image.Pt(track.Dx(), h))
	offset := op.Offset(image.Pt(track.Min.X, 0)).Push(gtx.Ops)
	st.colorPickerShade.Layout(sliderGtx, layout.Horizontal, unit.Dp(6))
	offset.Pop()

	thumbX := track.Min.X + int(st.colorPickerShade.Value*float32(track.Dx()))
	thumbR := gtx.Dp(unit.Dp(5))
	thumb := image.Rect(thumbX-thumbR, h/2-thumbR, thumbX+thumbR, h/2+thumbR)
	preview := parseConfigColorHexFallback(settingsColorShade(st.colorPickerBase, st.colorPickerShade.Value), fm.DefaultFilePaneSelectionHex)
	paint.FillShape(gtx.Ops, color.NRGBA{R: 238, G: 242, B: 250, A: 255}, clip.Ellipse(thumb).Op(gtx.Ops))
	inner := thumb.Inset(gtx.Dp(unit.Dp(2)))
	paint.FillShape(gtx.Ops, preview, clip.Ellipse(inner).Op(gtx.Ops))
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func (ui *UI) layoutSettingsColorPickerCommit(th *material.Theme, gtx layout.Context, st *settingsModalState, current string) layout.Dimensions {
	preview := parseConfigColorHexFallback(current, fm.DefaultFilePaneSelectionHex)
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Dp(unit.Dp(28)), gtx.Dp(unit.Dp(32)))
			paint.FillShape(gtx.Ops, preview, clip.Outline{Path: settingsColorHexPath(gtx, size, 1)}.Op())
			border := scaleColorAlpha(bestContrastColor(preview,
				color.NRGBA{R: 248, G: 250, B: 255, A: 255},
				color.NRGBA{R: 18, G: 22, B: 30, A: 255},
			), 0.8)
			paint.FillShape(gtx.Ops, border, clip.Stroke{Path: settingsColorHexPath(gtx, size, 1), Width: 1.5}.Op())
			return layout.Dimensions{Size: size}
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, current)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleModalFontSize(9)
			lbl.Color = txtColor
			lbl.MaxLines = 1
			return layoutVCenteredLabel(gtx, lbl)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 1)}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			focused := st.popupKeyboardMatches(settingsPopupKeyboardColor, len(st.colorSwatchClicks), settingsPopupKeyboardActionRow)
			return ui.layoutSettingsFlatActionButton(th, gtx, &st.colorPickerSetClick, "Set", false, focused, false)
		}),
	)
}

func (ui *UI) layoutSettingsFlatRemoveButton(gtx layout.Context, click *widget.Clickable, focused bool) layout.Dimensions {
	buttonSize := gtx.Dp(unit.Dp(20))
	iconSize := gtx.Dp(ui.scaleInterfaceDp(10))
	if iconSize < 1 {
		iconSize = 1
	}
	dims := fixedWidth(gtx, buttonSize, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, buttonSize, func(gtx layout.Context) layout.Dimensions {
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				bg := color.NRGBA{}
				iconColor := scaleColorAlpha(txtColor, 0.72)
				if click.Hovered() || focused {
					bg = color.NRGBA{R: 112, G: 40, B: 52, A: 238}
					iconColor = color.NRGBA{R: 255, G: 150, B: 164, A: 255}
				}
				return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						drawTabCloseIcon(gtx, iconSize, iconColor)
						return layout.Dimensions{Size: image.Pt(iconSize, iconSize)}
					})
				})
			})
		})
	})
	if dims.Size.X > 0 && dims.Size.Y > 0 {
		defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	}
	return dims
}

func (ui *UI) layoutSettingsFlatActionButton(th *material.Theme, gtx layout.Context, click *widget.Clickable, label string, active, focused, destructive bool) layout.Dimensions {
	dims := click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{R: 23, G: 28, B: 38, A: 255}
		fg := txtColor
		line := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
		if active {
			bg = color.NRGBA{R: 47, G: 66, B: 112, A: 255}
			fg = color.NRGBA{R: 240, G: 246, B: 255, A: 255}
			line = color.NRGBA{R: 130, G: 166, B: 235, A: 190}
		}
		if click.Hovered() || focused {
			bg = color.NRGBA{R: 36, G: 45, B: 62, A: 255}
			fg = color.NRGBA{R: 238, G: 244, B: 255, A: 255}
			line = color.NRGBA{R: 140, G: 174, B: 235, A: 180}
			if destructive {
				bg = color.NRGBA{R: 112, G: 40, B: 52, A: 238}
				fg = color.NRGBA{R: 255, G: 170, B: 182, A: 255}
				line = color.NRGBA{R: 255, G: 128, B: 148, A: 220}
			}
		}
		dims := fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.Font.Weight = font.Medium
				lbl.TextSize = ui.scaleModalFontSize(10)
				lbl.Color = fg
				lbl.MaxLines = 1
				return layoutVCenteredLabel(gtx, lbl)
			})
		})
		if dims.Size.X > 0 && dims.Size.Y > 0 {
			paint.FillShape(gtx.Ops, line, clip.Rect(image.Rect(0, dims.Size.Y-1, dims.Size.X, dims.Size.Y)).Op())
		}
		return dims
	})
	if dims.Size.X > 0 && dims.Size.Y > 0 {
		defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	}
	return dims
}

func (ui *UI) layoutSettingsColorHexSwatch(gtx layout.Context, click *widget.Clickable, swatch color.NRGBA, selected, focused bool) layout.Dimensions {
	size := image.Pt(gtx.Dp(unit.Dp(17)), gtx.Dp(unit.Dp(19)))
	gtx.Constraints = layout.Exact(size)
	hit := clip.Outline{Path: settingsColorHexPath(gtx, size, 0)}.Op().Push(gtx.Ops)
	dims := click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, swatch, clip.Outline{Path: settingsColorHexPath(gtx, size, 0.7)}.Op())
		contrast := bestContrastColor(swatch,
			color.NRGBA{R: 248, G: 250, B: 255, A: 255},
			color.NRGBA{R: 18, G: 22, B: 30, A: 255},
		)
		border := color.NRGBA{R: 255, G: 255, B: 255, A: 18}
		width := float32(1)
		if click.Hovered() {
			border = scaleColorAlpha(contrast, 0.64)
			width = 1.5
		}
		if selected || focused {
			border = scaleColorAlpha(contrast, 0.96)
			width = 2
		}
		paint.FillShape(gtx.Ops, border, clip.Stroke{Path: settingsColorHexPath(gtx, size, 1), Width: width}.Op())
		return layout.Dimensions{Size: size}
	})
	hit.Pop()
	if dims.Size.X > 0 && dims.Size.Y > 0 {
		defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	}
	return dims
}

func settingsColorHexPath(gtx layout.Context, size image.Point, inset float32) clip.PathSpec {
	w := float32(size.X) - inset
	h := float32(size.Y) - inset
	cx := float32(size.X) / 2
	left := inset
	top := inset
	quarter := (h - top) * 0.25
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(cx, top))
	path.LineTo(f32.Pt(w, top+quarter))
	path.LineTo(f32.Pt(w, top+quarter*3))
	path.LineTo(f32.Pt(cx, h))
	path.LineTo(f32.Pt(left, top+quarter*3))
	path.LineTo(f32.Pt(left, top+quarter))
	path.Close()
	return path.End()
}

func (ui *UI) layoutSettingsColorPreview(th *material.Theme, gtx layout.Context, palette filePanePalette) layout.Dimensions {
	return fillFlatBox(
		gtx,
		palette.PaneBg,
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "Pane Preview")
						lbl.Font.Typeface = ui.interfaceTypeface()
						lbl.TextSize = ui.scaleModalFontSize(9)
						lbl.Color = color.NRGBA{R: 176, G: 190, B: 215, A: 255}
						return lbl.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSettingsColorPreviewRows(th, gtx, palette)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSettingsPanePreviewScrollbar(gtx, palette)
							}),
						)
					}),
				)
			})
		},
	)
}

func (ui *UI) layoutSettingsColorPreviewRows(th *material.Theme, gtx layout.Context, palette filePanePalette) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorPreviewRow(th, gtx, palette.PaneBg, palette.PaneFg, "Normal", "alpha.txt")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorPreviewRow(th, gtx, palette.HoverBg, settingsEffectivePaneRowTextColor(palette, palette.HoverFg), "Hover", "beta.txt")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorPreviewRow(th, gtx, palette.PopupHoverBg, palette.PopupHoverFg, "Popup Hover", "menu item")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorPreviewRow(th, gtx, palette.SelectedBg, settingsEffectivePaneRowTextColor(palette, palette.SelectedFg), "Focused", "gamma.txt")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorPreviewRow(th, gtx, palette.MarkedBg, settingsEffectivePaneRowTextColor(palette, palette.MarkedFg), "Selected Files", "delta.txt")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorPreviewRow(th, gtx, palette.MarkedSelBg, settingsEffectivePaneRowTextColor(palette, palette.MarkedSelFg), "Focused + Selected Files", "omega.txt")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorPreviewCurrentDir(th, gtx, palette)
		}),
	)
}

func (ui *UI) layoutSettingsPanePreviewScrollbar(gtx layout.Context, palette filePanePalette) layout.Dimensions {
	trackW := gtx.Dp(unit.Dp(8))
	trackH := gtx.Constraints.Max.Y
	if trackW < 6 {
		trackW = 6
	}
	if trackH < 1 {
		trackH = gtx.Dp(unit.Dp(88))
	}
	thumbH := trackH / 3
	if minThumb := gtx.Dp(unit.Dp(22)); thumbH < minThumb {
		thumbH = minThumb
	}
	track, thumb := settingsPreviewScrollbarGeometry(trackW, trackH, thumbH)
	return fixedWidth(gtx, trackW, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, trackH, func(gtx layout.Context) layout.Dimensions {
			paintSettingsPreviewRoundedRect(gtx, track, palette.ScrollTrackH)
			paintSettingsPreviewRoundedRect(gtx, thumb, palette.ScrollThumbH)
			return layout.Dimensions{Size: image.Pt(trackW, trackH)}
		})
	})
}

func (ui *UI) settingsColorPreviewRowHeight(gtx layout.Context) int {
	rowH := gtx.Dp(scaleFilePaneDp(ui.fmCfg, 18))
	if rowH < 1 {
		rowH = 1
	}
	return rowH
}

func (ui *UI) settingsColorPreviewPathStripHeight(gtx layout.Context) int {
	return ui.filePaneHeaderStripHeight(gtx, nil)
}

func (ui *UI) settingsColorPreviewPathContainerHeight(gtx layout.Context) int {
	height := ui.settingsColorPreviewPathStripHeight(gtx) + gtx.Dp(unit.Dp(1))*2
	if height < 1 {
		height = 1
	}
	return height
}

func (ui *UI) settingsColorPreviewCurrentDirHeight(gtx layout.Context) int {
	rowH := ui.settingsColorPreviewRowHeight(gtx)
	if pathH := ui.settingsColorPreviewPathContainerHeight(gtx); pathH > rowH {
		rowH = pathH
	}
	return rowH
}

func (ui *UI) layoutSettingsColorPreviewCurrentDir(th *material.Theme, gtx layout.Context, palette filePanePalette) layout.Dimensions {
	rowH := ui.settingsColorPreviewCurrentDirHeight(gtx)
	pathStripH := ui.settingsColorPreviewPathStripHeight(gtx)
	pathContainerH := ui.settingsColorPreviewPathContainerHeight(gtx)
	stateW := settingsColorPreviewStateWidth(th, gtx, ui.fmCfg, ui.mainTypeface())
	stateColor := settingsColorPreviewStateColor(palette.PaneBg)
	rowBg, rowBorder := filePanePathRowColors(palette)
	pathColor := filePanePathBaseColor(palette)
	return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
		return fillBgExact(gtx, palette.PaneBg, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, stateW, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(th, scaleConfigFontSize(ui.fmCfg, 13), "Current Dir")
							lbl.Font.Typeface = ui.mainTypeface()
							lbl.Font.Weight = font.Normal
							lbl.Color = stateColor
							lbl.MaxLines = 1
							return layoutVCenteredLabel(gtx, lbl)
						})
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, pathContainerH, func(gtx layout.Context) layout.Dimensions {
								return fillFlatBox(gtx, rowBg, rowBorder, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(1), Right: unit.Dp(1), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return fixedHeight(gtx, pathStripH, func(gtx layout.Context) layout.Dimensions {
											return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												gtx.Constraints.Min.X = gtx.Constraints.Max.X
												return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
													return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
														return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
															layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																return layoutFilePaneFrameLine(gtx, ui.filePaneFrameEdgeWidth(th, gtx, nil), filePanePathFrameColor(palette))
															}),
															layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																return ui.layoutFilePaneFrameBracket(gtx, nil, true, filePanePathFrameColor(palette))
															}),
															layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																return ui.layoutFilePanePathSegmentLabel(th, gtx, nil, "C:", color.NRGBA{}, pathColor, color.NRGBA{}, font.Normal)
															}),
															ui.filePaneBreadcrumbSeparator(th, nil, palette, "›"),
															layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																return ui.layoutFilePanePathSegmentLabel(th, gtx, nil, "AsmSource", color.NRGBA{}, pathColor, color.NRGBA{}, font.Normal)
															}),
															ui.filePaneBreadcrumbSeparator(th, nil, palette, "›"),
															layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																return ui.layoutFilePanePathSegmentLabel(th, gtx, nil, "tests", color.NRGBA{}, pathColor, color.NRGBA{}, font.Medium)
															}),
															ui.filePaneBreadcrumbSeparator(th, nil, palette, ">"),
															layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																return ui.layoutFilePanePathSegmentLabel(th, gtx, nil, "*.*", color.NRGBA{}, pathColor, color.NRGBA{}, font.Normal)
															}),
															layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																return ui.layoutFilePaneFrameBracket(gtx, nil, false, filePanePathFrameColor(palette))
															}),
														)
													})
												})
											})
										})
									})
								})
							})
						})
					}),
				)
			})
		})
	})
}

func (ui *UI) layoutSettingsColorPreviewRow(th *material.Theme, gtx layout.Context, bg, fg color.NRGBA, stateLabel, fileName string) layout.Dimensions {
	rowH := ui.settingsColorPreviewRowHeight(gtx)
	nameSize := scaleConfigFontSize(ui.fmCfg, 13)
	stateSize := nameSize
	stateW := settingsColorPreviewStateWidth(th, gtx, ui.fmCfg, ui.mainTypeface())
	stateColor := settingsColorPreviewStateColor(bg)
	return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
		return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, stateW, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(th, stateSize, stateLabel)
							lbl.Font.Typeface = ui.mainTypeface()
							lbl.Font.Weight = font.Normal
							lbl.Color = stateColor
							lbl.MaxLines = 1
							return layoutVCenteredLabel(gtx, lbl)
						})
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(th, nameSize, fileName)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.Font.Weight = font.Medium
						lbl.Color = fg
						lbl.MaxLines = 1
						return layoutVCenteredLabel(gtx, lbl)
					}),
				)
			})
		})
	})
}

func settingsColorPreviewStateWidth(th *material.Theme, gtx layout.Context, cfg *fm.Config, face font.Typeface) int {
	labels := []string{
		"Normal",
		"Hover",
		"Popup Hover",
		"Focused",
		"Selected Files",
		"Focused + Selected Files",
		"Current Dir",
		"Scrollbar",
	}
	maxW := 0
	for _, txt := range labels {
		lbl := material.Label(th, scaleConfigFontSize(cfg, 13), txt)
		lbl.Font.Typeface = face
		lbl.Font.Weight = font.Normal
		lbl.MaxLines = 1
		w := measureLabelUnconstrained(gtx, lbl).Size.X
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

func settingsColorPreviewStateColor(bg color.NRGBA) color.NRGBA {
	muted := mixNRGBA(contrastTextColor(bg), hintColor, 0.55)
	if contrastScore(bg, muted) < 2.8 {
		muted = mixNRGBA(contrastTextColor(bg), hintColor, 0.35)
	}
	return muted
}

func settingsEffectivePaneRowTextColor(palette filePanePalette, fg color.NRGBA) color.NRGBA {
	if fg.A == 0 {
		return palette.PaneFg
	}
	return fg
}

func settingsPreviewColorForCategory(palette filePanePalette, key, part string) color.NRGBA {
	switch key {
	case "normal":
		if part == "text" {
			return palette.PaneFg
		}
		return palette.PaneBg
	case "hover":
		if part == "text" {
			return settingsEffectivePaneRowTextColor(palette, palette.HoverFg)
		}
		return palette.HoverBg
	case "popup_hover":
		if part == "text" {
			return palette.PopupHoverFg
		}
		return palette.PopupHoverBg
	case "selected_files":
		if part == "text" {
			return settingsEffectivePaneRowTextColor(palette, palette.MarkedFg)
		}
		return palette.MarkedBg
	case "focused_selected":
		if part == "text" {
			return settingsEffectivePaneRowTextColor(palette, palette.MarkedSelFg)
		}
		return palette.MarkedSelBg
	case "current_dir":
		if part == "text" {
			return palette.CurrentDirFg
		}
		return palette.CurrentDirBg
	case "scrollbar":
		if part == "text" {
			return palette.ScrollTrack
		}
		return palette.ScrollThumb
	default:
		if part == "text" {
			return settingsEffectivePaneRowTextColor(palette, palette.SelectedFg)
		}
		return palette.SelectedBg
	}
}

func (ui *UI) layoutSettingsAssociationsTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	for {
		ev, ok := st.viewAssocExtEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
			st.refreshViewerAssociationDraftInfo(false)
		}
	}
	for {
		ev, ok := st.viewAssocAppEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
			st.refreshViewerAssociationDraftInfo(false)
		}
	}
	st.syncViewerAssociationEditors()
	for st.viewAssocApplyClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusAssociationsApply)
		action, err := st.upsertCurrentViewerAssociation()
		if err != nil {
			st.errText = err.Error()
			continue
		}
		st.errText = ""
		st.viewAssocPickOpen = false
		if action == "Update" {
			st.assocInfoText = "Pending change; Save to persist"
		} else {
			st.assocInfoText = "Pending add; Save to persist"
		}
	}
	for st.viewAssocRemoveClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusAssociationsRemove)
		ext := st.viewAssocEditingExt
		if ext == "" {
			ext = fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
		}
		if ext == "" {
			st.errText = "association extension is required"
			continue
		}
		_, savedExists := st.viewerSavedAssociation(ext)
		if !st.removeCurrentViewerAssociation() {
			st.errText = "no association set for " + viewerAssociationDisplayExtension(ext)
			continue
		}
		st.errText = ""
		if savedExists {
			st.assocInfoText = "Pending removal; Save to persist"
		} else {
			st.assocInfoText = ""
		}
		st.viewAssocPickOpen = false
	}
	for st.viewAssocPickClick.Clicked(gtx) {
		st.setKeyboardFocus(settingsKeyboardFocusAssociationsBrowse)
		st.toggleViewerAssociationPicker()
	}

	currentAssocExt := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
	currentAssoc, currentAssocExists := st.viewerAssociation(currentAssocExt)
	if !currentAssocExists && st.viewAssocEditingExt != "" {
		currentAssoc, currentAssocExists = st.viewerAssociation(st.viewAssocEditingExt)
	}
	_, currentAssocSaved := st.viewerSavedAssociation(currentAssocExt)
	pickerPrograms, pickerMatchCount := st.viewerAssociationPickerPrograms()
	savedAssocCount := 0
	pendingAssocCount := 0
	assocEntriesByExt := make(map[string]fm.ViewerAssociation, len(st.viewAssocEntries))
	assocSavedByExt := make(map[string]fm.ViewerAssociation, len(st.viewAssocSavedEntries))
	for _, assoc := range st.viewAssocEntries {
		assocEntriesByExt[assoc.Extension] = assoc
	}
	for _, assoc := range st.viewAssocSavedEntries {
		assocSavedByExt[assoc.Extension] = assoc
	}
	for _, assoc := range st.viewAssocEntries {
		if saved, ok := assocSavedByExt[assoc.Extension]; ok && saved.AppPath == assoc.AppPath {
			savedAssocCount++
			continue
		}
		pendingAssocCount++
	}
	for _, assoc := range st.viewAssocSavedEntries {
		if _, ok := assocEntriesByExt[assoc.Extension]; !ok {
			pendingAssocCount++
		}
	}

	statusText := ""
	statusColor := color.NRGBA{R: 152, G: 205, B: 152, A: 255}
	switch {
	case currentAssocExt == "":
		switch {
		case savedAssocCount > 0 && pendingAssocCount > 0:
			statusText = fmt.Sprintf("%d Saved / %d Pending", savedAssocCount, pendingAssocCount)
			statusColor = color.NRGBA{R: 222, G: 190, B: 122, A: 255}
		case pendingAssocCount > 0:
			statusText = fmt.Sprintf("%d Pending", pendingAssocCount)
			statusColor = color.NRGBA{R: 222, G: 190, B: 122, A: 255}
		case savedAssocCount > 0:
			statusText = fmt.Sprintf("%d Saved", savedAssocCount)
			statusColor = color.NRGBA{R: 174, G: 190, B: 214, A: 255}
		}
	case currentAssocExists:
		if saved, ok := assocSavedByExt[currentAssocExt]; ok && saved.AppPath == currentAssoc.AppPath {
			statusText = "Saved"
		} else {
			statusText = "Pending"
			statusColor = color.NRGBA{R: 222, G: 190, B: 122, A: 255}
		}
	case currentAssocSaved:
		statusText = "Pending"
		statusColor = color.NRGBA{R: 222, G: 190, B: 122, A: 255}
	case currentAssocExt != "":
		statusText = "New"
		statusColor = viewerSettingsSectionStyleFor("p1").BadgeText
	}
	assocApplyLabel := "Add"
	if currentAssocExists {
		assocApplyLabel = "Update"
	}
	rowLabel := func(txt string, enabled bool) layout.Widget {
		return settingsViewerRowLabel(ui, th, txt, enabled)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(rowLabel("F4 app override (local files only)", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, "Browse copies a known app path into the editor; Add or Update queues it until Save.")
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleModalFontSize(9)
			lbl.Color = hintColor
			lbl.MaxLines = 1
			lbl.Truncator = "..."
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(rowLabel("Extension", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, gtx.Dp(unit.Dp(108)), func(gtx layout.Context) layout.Dimensions {
						ed := material.Editor(th, &st.viewAssocExtEdit, "mp3")
						ed.Font.Typeface = ui.interfaceTypeface()
						ed.TextSize = ui.scaleModalFontSize(10)
						ed.Color = txtColor
						ed.HintColor = hintColor
						dims := ui.layoutEditorWithContextMenu(th, gtx, "settings-view-assoc-ext", &st.viewAssocExtEdit, true, func(gtx layout.Context) layout.Dimensions {
							return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewAssocExtEdit), true, ed.Layout)
						})
						st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusAssociationsExt, &st.viewAssocExtEdit)
						return dims
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsFlatActionButton(th, gtx, &st.viewAssocPickClick, "Browse", st.viewAssocPickOpen, st.focus == settingsKeyboardFocusAssociationsBrowse, false)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsFlatActionButton(th, gtx, &st.viewAssocApplyClick, assocApplyLabel, currentAssocExists, st.focus == settingsKeyboardFocusAssociationsApply, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if statusText == "" {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, statusText)
						lbl.Font.Typeface = ui.interfaceTypeface()
						lbl.TextSize = ui.scaleModalFontSize(9)
						lbl.Color = statusColor
						lbl.MaxLines = 1
						lbl.Truncator = "..."
						return lbl.Layout(gtx)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !st.viewAssocPickOpen {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsViewerAssocPicker(th, gtx, st, pickerPrograms, pickerMatchCount)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(rowLabel("App path", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(th, &st.viewAssocAppEdit, `C:\Program Files\App\player.exe`)
					ed.Font.Typeface = ui.interfaceTypeface()
					ed.TextSize = ui.scaleModalFontSize(10)
					ed.Color = txtColor
					ed.HintColor = hintColor
					dims := ui.layoutEditorWithContextMenu(th, gtx, "settings-view-assoc-app", &st.viewAssocAppEdit, true, func(gtx layout.Context) layout.Dimensions {
						return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewAssocAppEdit), true, ed.Layout)
					})
					st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusAssociationsApp, &st.viewAssocAppEdit)
					return dims
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !currentAssocExists {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutSettingsFlatRemoveButton(gtx, &st.viewAssocRemoveClick, st.focus == settingsKeyboardFocusAssociationsRemove)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			infoText := st.assocInfoText
			if infoText == "" {
				infoText = st.viewerAssociationNoticeText()
			}
			if infoText == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, infoText)
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.TextSize = ui.scaleModalFontSize(9)
				lbl.Color = color.NRGBA{R: 152, G: 205, B: 152, A: 255}
				lbl.MaxLines = 2
				lbl.Truncator = "..."
				return lbl.Layout(gtx)
			})
		}),
	)
}

func (ui *UI) layoutSettingsViewerAssocPicker(th *material.Theme, gtx layout.Context, st *settingsModalState, programs []viewerAssociationProgram, matchCount int) layout.Dimensions {
	gtx2 := gtx
	maxH := gtx.Dp(unit.Dp(168))
	if gtx2.Constraints.Max.Y > maxH {
		gtx2.Constraints.Max.Y = maxH
	}
	dims := fillRoundedBox(
		gtx2,
		gtx2.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 18, G: 22, B: 30, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			if len(programs) == 0 {
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, "No apps yet")
					lbl.Font.Typeface = ui.interfaceTypeface()
					lbl.TextSize = ui.scaleModalFontSize(9)
					lbl.Color = hintColor
					return lbl.Layout(gtx)
				})
			}
			var picked *viewerAssociationProgram
			currentAppPath := fm.NormalizeViewerAssociationAppPath(st.viewAssocAppEdit.Text())
			dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if matchCount <= 0 || matchCount >= len(programs) {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, fmt.Sprintf("%d similar apps, then all apps", matchCount))
						lbl.Font.Typeface = ui.interfaceTypeface()
						lbl.TextSize = ui.scaleModalFontSize(9)
						lbl.Color = hintColor
						lbl.MaxLines = 1
						lbl.Truncator = "..."
						return lbl.Layout(gtx)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return st.viewAssocPickList.Layout(gtx, len(programs), func(gtx layout.Context, i int) layout.Dimensions {
						program := programs[i]
						click := st.viewerAssocRowClick(program.AppPath)
						rowFocused := st.popupKeyboardMatches(settingsPopupKeyboardViewerAssoc, i, settingsPopupKeyboardActionRow)
						// Clickable.Layout drains queued clicks before painting, so row
						// actions must be drained before Layout and then applied once
						// after the list finishes to avoid mid-layout state changes.
						for click.Clicked(gtx) {
							st.setPopupKeyboardFocus(settingsPopupKeyboardViewerAssoc, i, settingsPopupKeyboardActionRow)
							if picked == nil {
								programCopy := program
								picked = &programCopy
							}
						}
						selected := strings.EqualFold(program.AppPath, currentAppPath)
						bg := color.NRGBA{A: 0}
						if selected {
							bg = color.NRGBA{R: 80, G: 120, B: 220, A: 45}
							if rowFocused {
								bg = color.NRGBA{R: 92, G: 132, B: 228, A: 62}
							}
						} else if rowFocused {
							bg = color.NRGBA{R: 74, G: 108, B: 182, A: 52}
						} else if click.Hovered() {
							bg = color.NRGBA{R: 255, G: 255, B: 255, A: 10}
						}
						dims := layoutSettingsPickerRowBackground(gtx, bg, func(gtx layout.Context) layout.Dimensions {
							return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									pointer.CursorPointer.Add(gtx.Ops)
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											usedBy := strings.Join(program.Extensions, ", ")
											lbl := material.Body2(th, filepath.Base(program.AppPath)+" used by "+usedBy)
											lbl.Font.Typeface = ui.interfaceTypeface()
											lbl.Font.Weight = font.Medium
											lbl.TextSize = ui.scaleModalFontSize(10)
											lbl.Color = txtColor
											lbl.MaxLines = 1
											lbl.Truncator = "..."
											return layoutVCenteredLabel(gtx, lbl)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											lbl := material.Caption(th, program.AppPath)
											lbl.Font.Typeface = ui.interfaceTypeface()
											lbl.TextSize = ui.scaleModalFontSize(8)
											lbl.Color = hintColor
											lbl.MaxLines = 1
											lbl.Truncator = "..."
											return layoutVCenteredLabel(gtx, lbl)
										}),
									)
								})
							})
						})
						return dims
					})
				}),
			)
			if picked != nil {
				st.applyPickedViewerAssociation(picked.AppPath)
			}
			return dims
		},
	)
	registerSettingsPopupArea(gtx2, &st.viewAssocPickerPopupTag, dims.Size)
	return dims
}

func (ui *UI) layoutSettingsModalFooter(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	hoverFooterKey := ""
	if st.cancelClick.Hovered() {
		hoverFooterKey = "cancel"
	}
	if st.saveClick.Hovered() {
		hoverFooterKey = "save"
	}
	st.setFooterHover(hoverFooterKey, gtx.Now)
	hoverCancel, hoverAnimCancel := st.footerHoverFill(gtx.Now, "cancel")
	hoverSave, hoverAnimSave := st.footerHoverFill(gtx.Now, "save")
	pulseCancel, pulseAnimCancel := st.footerPulseFill(gtx.Now, "cancel")
	pulseSave, pulseAnimSave := st.footerPulseFill(gtx.Now, "save")
	cancelVisual := st.footerActionVisualState(settingsFooterActionCancel)
	saveVisual := st.footerActionVisualState(settingsFooterActionSave)
	saveLabel := st.saveLabel()
	if hoverAnimCancel || hoverAnimSave || pulseAnimCancel || pulseAnimSave {
		gtx.Execute(op.InvalidateCmd{})
	}

	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(layoutDialogHorizontalDivider),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.errText == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, st.errText)
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.TextSize = ui.scaleModalFontSize(9)
				lbl.Color = color.NRGBA{R: 255, G: 170, B: 170, A: 255}
				lbl.MaxLines = 2
				lbl.Truncator = "..."
				return layout.W.Layout(gtx, lbl.Layout)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutDialogActionPairState(
					th,
					gtx,
					&st.cancelClick,
					"Cancel",
					hoverCancel,
					pulseCancel,
					false,
					&st.saveClick,
					saveLabel,
					hoverSave,
					pulseSave,
					false,
					cancelVisual,
					saveVisual,
				)
			})
		}),
	)
}

func (ui *UI) layoutSettingsConfigEditor(th *material.Theme, gtx layout.Context, st *settingsModalState, scrollbarStyle material.ScrollbarStyle) (layout.Dimensions, editorScrollMetrics, bool) {
	if st == nil {
		return layout.Dimensions{}, editorScrollMetrics{}, false
	}
	editorDims := layout.Dimensions{}
	metrics := editorScrollMetrics{}
	scrollable := false
	dims := fixedHeight(gtx, gtx.Constraints.Max.Y, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{Alignment: layout.NE}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				ed := material.Editor(th, &st.configEdit, "")
				ed.Font.Typeface = ui.interfaceTypeface()
				ed.TextSize = ui.scaleModalFontSize(10)
				ed.Color = txtColor
				ed.HintColor = hintColor
				editorFocused := gtx.Focused(&st.configEdit) || st.focus == settingsKeyboardFocusConfigEditor || st.focusPending == settingsKeyboardFocusConfigEditor
				editorDims = ui.layoutEditorWithContextMenu(th, gtx, "settings-config", &st.configEdit, true, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return fixedHeight(gtx, gtx.Constraints.Max.Y, func(gtx layout.Context) layout.Dimensions {
						return layoutNeutralEditorBox(gtx, editorFocused, true, ed.Layout)
					})
				})
				st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusConfigEditor, &st.configEdit)
				metrics, scrollable = editorVerticalScrollMetrics(&st.configEdit)
				return editorDims
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				if !scrollable || editorDims.Size.Y <= 0 {
					return layout.Dimensions{}
				}
				height := editorDims.Size.Y
				if height < 1 {
					height = 1
				}
				start := clamp01(float32(metrics.Offset) / float32(metrics.Content))
				end := clamp01(float32(metrics.Offset+metrics.Viewport) / float32(metrics.Content))
				trackH := height - gtx.Dp(unit.Dp(4))
				if trackH < 1 {
					trackH = 1
				}
				return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Right: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, gtx.Dp(scrollbarStyle.Width()), func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, trackH, func(gtx layout.Context) layout.Dimensions {
							return scrollbarStyle.Layout(gtx, layout.Vertical, start, end)
						})
					})
				})
			}),
		)
	})
	return dims, metrics, scrollable
}

func (ui *UI) layoutSettingsConfigTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	cfgPath := ui.configDisplayPath()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, "Edit hexone.yaml directly")
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleModalFontSize(9)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, "Located at:")
					lbl.Font.Typeface = ui.interfaceTypeface()
					lbl.TextSize = ui.scaleModalFontSize(9)
					lbl.Color = hintColor
					return lbl.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return fillFlatBox(
						gtx,
						color.NRGBA{R: 26, G: 29, B: 34, A: 255},
						color.NRGBA{R: 128, G: 152, B: 196, A: 74},
						func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := ui.settingsConfigPathLabel(th, st, cfgPath)
								dims := lbl.Layout(gtx)
								st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusConfigPath, &st.configPathSelect)
								return dims
							})
						},
					)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			scrollbarStyle := settingsScrollbarStyle(th, &st.configScrollbar)
			dims, metrics, scrollable := ui.layoutSettingsConfigEditor(th, gtx, st, scrollbarStyle)
			if scrollable {
				if delta := st.configScrollbar.ScrollDistance(); delta != 0 {
					editorScrollToVerticalOffset(&st.configEdit, metrics.Offset+int(delta*float32(metrics.Content)))
					gtx.Execute(op.InvalidateCmd{})
				}
			}
			return dims
		}),
	)
}

func (ui *UI) settingsConfigPathLabel(th *material.Theme, st *settingsModalState, cfgPath string) material.LabelStyle {
	lbl := material.Body2(th, cfgPath)
	lbl.Font.Typeface = ui.interfaceTypeface()
	lbl.TextSize = ui.scaleModalFontSize(8)
	lbl.Color = color.NRGBA{R: 194, G: 212, B: 255, A: 255}
	lbl.SelectionColor = color.NRGBA{R: 80, G: 120, B: 220, A: 88}
	lbl.State = &st.configPathSelect
	// File paths have many valid Unicode line-break opportunities (notably after
	// a drive prefix), which can leave "C:" stranded on its own line. Pack the
	// path by grapheme instead, while retaining the original full selectable text.
	lbl.WrapPolicy = text.WrapGraphemes
	lbl.MaxLines = 2
	lbl.Truncator = "…"
	return lbl
}

func formatConfigFloat(v float32) string {
	if v <= 0 {
		return ""
	}
	return strconv.FormatFloat(float64(v), 'f', -1, 32)
}

func (st *settingsModalState) ensurePaneFontFamilyClicks(n int) {
	if n <= cap(st.paneFontFamilyClicks) {
		st.paneFontFamilyClicks = st.paneFontFamilyClicks[:n]
		return
	}
	old := st.paneFontFamilyClicks
	st.paneFontFamilyClicks = make([]widget.Clickable, n)
	copy(st.paneFontFamilyClicks, old)
}

func (st *settingsModalState) ensureInterfaceFontFamilyClicks(n int) {
	if n <= cap(st.interfaceFontFamilyClicks) {
		st.interfaceFontFamilyClicks = st.interfaceFontFamilyClicks[:n]
		return
	}
	old := st.interfaceFontFamilyClicks
	st.interfaceFontFamilyClicks = make([]widget.Clickable, n)
	copy(st.interfaceFontFamilyClicks, old)
}

func (st *settingsModalState) ensureCurrentDirFontFamilyClicks(n int) {
	if n <= cap(st.currentDirFontFamilyClicks) {
		st.currentDirFontFamilyClicks = st.currentDirFontFamilyClicks[:n]
		return
	}
	old := st.currentDirFontFamilyClicks
	st.currentDirFontFamilyClicks = make([]widget.Clickable, n)
	copy(st.currentDirFontFamilyClicks, old)
}

func (st *settingsModalState) ensureViewFontFamilyClicks(n int) {
	if n <= cap(st.viewFontFamilyClicks) {
		st.viewFontFamilyClicks = st.viewFontFamilyClicks[:n]
		return
	}
	old := st.viewFontFamilyClicks
	st.viewFontFamilyClicks = make([]widget.Clickable, n)
	copy(st.viewFontFamilyClicks, old)
}

func (st *settingsModalState) ensureTabsFontFamilyClicks(n int) {
	if n <= cap(st.tabsFontFamilyClicks) {
		st.tabsFontFamilyClicks = st.tabsFontFamilyClicks[:n]
		return
	}
	old := st.tabsFontFamilyClicks
	st.tabsFontFamilyClicks = make([]widget.Clickable, n)
	copy(st.tabsFontFamilyClicks, old)
}

func (st *settingsModalState) ensureTerminalFontFamilyClicks(n int) {
	if n <= cap(st.terminalFontFamilyClicks) {
		st.terminalFontFamilyClicks = st.terminalFontFamilyClicks[:n]
		return
	}
	old := st.terminalFontFamilyClicks
	st.terminalFontFamilyClicks = make([]widget.Clickable, n)
	copy(st.terminalFontFamilyClicks, old)
}

func (st *settingsModalState) ensureViewShellClicks(n int) {
	if n <= cap(st.viewShellClicks) {
		st.viewShellClicks = st.viewShellClicks[:n]
		return
	}
	old := st.viewShellClicks
	st.viewShellClicks = make([]widget.Clickable, n)
	copy(st.viewShellClicks, old)
}

func viewerAssociationDisplayExtension(ext string) string {
	ext = strings.TrimSpace(ext)
	ext = strings.TrimPrefix(ext, ".")
	return ext
}

func parseViewerCommandTargetFields(keyRaw, commandRaw string) (viewerCommandTargetEntry, error) {
	key := normalizeViewerCommandTargetInput(keyRaw)
	if key == "" {
		return viewerCommandTargetEntry{}, fmt.Errorf("target key is required")
	}
	command := strings.TrimSpace(commandRaw)
	if command == "" {
		return viewerCommandTargetEntry{}, fmt.Errorf("exact override command is required")
	}
	return viewerCommandTargetEntry{
		Key:     key,
		Command: command,
	}, nil
}

func parseViewerCommandRuleFields(patternRaw, commandRaw string) (fm.ViewerCommandRule, error) {
	pattern := strings.TrimSpace(patternRaw)
	if pattern == "" {
		return fm.ViewerCommandRule{}, fmt.Errorf("regex pattern is required")
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return fm.ViewerCommandRule{}, fmt.Errorf("regex pattern is invalid: %w", err)
	}
	command := strings.TrimSpace(commandRaw)
	if command == "" {
		return fm.ViewerCommandRule{}, fmt.Errorf("rule command is required")
	}
	return fm.ViewerCommandRule{
		Pattern: pattern,
		Command: command,
	}, nil
}

func parseViewerAssociationFields(extRaw, appRaw string) (fm.ViewerAssociation, error) {
	ext := fm.NormalizeViewerAssociationExtension(extRaw)
	if ext == "" {
		return fm.ViewerAssociation{}, fmt.Errorf("association extension is invalid")
	}
	app := fm.NormalizeViewerAssociationAppPath(appRaw)
	if app == "" {
		return fm.ViewerAssociation{}, fmt.Errorf("association app path is required")
	}
	return fm.ViewerAssociation{
		Extension: ext,
		AppPath:   app,
	}, nil
}

func normalizeViewerShellInput(raw string) string {
	if shell, ok := fm.NormalizeKnownViewerShell(raw); ok {
		return shell
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

func (ui *UI) applyConfigRuntime(now time.Time) {
	if ui == nil {
		return
	}
	if ui.fmCfg == nil {
		return
	}
	ui.fileKeys = newFileKeyMap(ui.fmCfg)
	ui.typeface = font.Typeface(ui.fmCfg.General.Typeface)
	ui.textSize = fontSizeFromConfig(ui.fmCfg)
	ui.applyTerminalShellRuntime()
	if ui.tab2State != nil {
		ui.tab2State.typeface = ui.interfaceTypeface()
	}
	ui.reloadFilePanesForConfig(now)
	ui.refreshFileViewerNow(now)
}

func (ui *UI) reloadFilePanesForConfig(now time.Time) {
	if ui == nil || ui.fmCfg == nil || len(ui.filePanes) == 0 {
		return
	}
	active := ui.activeFilePane
	next := make([]*filePaneState, len(ui.filePanes))
	for i, old := range ui.filePanes {
		if old == nil {
			continue
		}
		dir := old.dir
		reloadDir := dir
		baseDir := dir
		var reloadRemote *paneSSHSession
		localBeforeRemote := old.localDirBeforeRemote
		if old.remoteConnected() && old.remote != nil {
			reloadRemote = old.remote.clone()
			if strings.TrimSpace(baseDir) == "" || strings.HasPrefix(baseDir, "/") {
				baseDir = strings.TrimSpace(localBeforeRemote)
			}
			if strings.TrimSpace(baseDir) == "" {
				baseDir = "."
			}
			if strings.TrimSpace(reloadDir) == "" {
				reloadDir = reloadRemote.homeDir()
			}
			reloadDir = path.Clean(reloadDir)
			if reloadDir == "" || reloadDir == "." {
				reloadDir = "/"
			}
		}

		selectedPath := ""
		mode := table.ModeFull
		if old.table != nil {
			mode = old.table.Mode
		}
		if sel := old.selectedEntry(); sel != nil {
			selectedPath = sel.Path
		}

		pane := newFilePaneState(baseDir, ui.fmCfg)
		pane.table.SetMode(mode)
		idx := i
		pane.table.OnClick = func(row int) {
			_ = row
			ui.setActiveFilePane(idx)
		}
		pane.table.OnDoubleClick = func(row int) {
			ui.queueFilePaneSystemOpen(idx, row)
		}
		pane.table.OnActivate = func(row int) {
			ui.queueFilePaneOpen(idx, row)
		}
		if reloadRemote != nil {
			pane.remote = reloadRemote
			pane.localDirBeforeRemote = localBeforeRemote
			if pane.localDirBeforeRemote == "" {
				pane.localDirBeforeRemote = baseDir
			}
			if err := pane.load(reloadDir); err != nil {
				pane.setNotice("remote reload failed: "+err.Error(), now)
			} else {
				pane.applySelection(selectedPath, "", pane.table.Selected, true)
			}
		} else {
			startLocalPaneLoad(pane, filepath.Clean(dir), selectedPath, "", pane.table.Selected)
		}
		if old.remote != nil {
			old.remote.close()
			old.remote = nil
		}
		next[i] = pane
	}
	ui.filePanes = next
	if active < 0 {
		active = 0
	}
	if active >= len(ui.filePanes) {
		active = len(ui.filePanes) - 1
	}
	ui.setActiveFilePane(active)
}
