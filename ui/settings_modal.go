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
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

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

	colorScopePaneClick         widget.Clickable
	colorScopeViewerClick       widget.Clickable
	colorScopeFilenameClick     widget.Clickable
	colorScope                  string
	colorScopeAnim              settingsChoiceAnim
	colorCategoryClick          widget.Clickable
	colorBgPickerClick          widget.Clickable
	colorTextPickerClick        widget.Clickable
	colorValueEdit              widget.Editor
	colorTextValueEdit          widget.Editor
	colorCategoryOpen           bool
	colorCategoryOpenedAt       time.Time
	colorCategoryHoverID        string
	colorCategoryHoverAnim      segmentedAnimState
	colorPickerOpen             bool
	colorPickerTarget           string
	popupGlobalPointerTag       uiEventTag
	colorCategoryPopupTag       uiEventTag
	colorPickerPopupTag         uiEventTag
	filenameIconPickerPopupTag  uiEventTag
	filenamePermPickerPopupTag  uiEventTag
	viewTargetPickerPopupTag    uiEventTag
	viewAssocPickerPopupTag     uiEventTag
	viewRulePickerPopupTag      uiEventTag
	colorCategory               string
	colorOptionClicks           []widget.Clickable
	colorSwatchClicks           []widget.Clickable
	colorPaneBackground         string
	colorPaneText               string
	colorHover                  string
	colorHoverText              string
	colorSelection              string
	colorSelectionText          string
	colorSelectedFiles          string
	colorSelectedFilesText      string
	colorFocusedSelected        string
	colorFocusedSelectedText    string
	colorCurrentDir             string
	colorCurrentDirText         string
	colorScrollbarThumb         string
	colorScrollbarTrack         string
	colorViewerBackground       string
	colorViewerText             string
	colorViewerSelection        string
	filenameDefaultText         string
	filenameDefaultTextEdit     widget.Editor
	filenameDefaultIcon         string
	filenameDefaultIconClick    widget.Clickable
	filenameDefaultTextPicker   widget.Clickable
	filenameIconPickerOpen      bool
	filenameIconPickerTarget    string
	filenameIconSwatchClicks    []widget.Clickable
	filenameRuleMode            string
	filenameRuleModeAnim        settingsChoiceAnim
	filenameRuleModeAgeClick    widget.Clickable
	filenameRuleModePermClick   widget.Clickable
	filenameRuleModeExtClick    widget.Clickable
	filenameRuleModeSizeClick   widget.Clickable
	filenameAgeOffsetEdit       widget.Editor
	filenameAgeUnit             string
	filenameAgeUnitAnim         settingsChoiceAnim
	filenameAgeUnitClicks       [4]widget.Clickable
	filenameAgeTextEdit         widget.Editor
	filenameAgeIcon             string
	filenameAgeIconClick        widget.Clickable
	filenameAgeTextPicker       widget.Clickable
	filenameAgeApplyClick       widget.Clickable
	filenameAgeRemoveClick      widget.Clickable
	filenameAgeList             widget.List
	filenameAgeEntries          []fm.FilenameAgeRule
	filenameAgeSavedEntries     []fm.FilenameAgeRule
	filenameAgeLookup           string
	filenameAgeRowClicks        map[string]*widget.Clickable
	filenameAgeRowRemove        map[string]*widget.Clickable
	filenameAgeInfoText         string
	filenamePermEdit            widget.Editor
	filenamePermMatch           string
	filenamePermMatchAnim       settingsChoiceAnim
	filenamePermMatchClicks     [4]widget.Clickable
	filenamePermChecks          [9]widget.Bool
	filenamePermPickerOpen      bool
	filenamePermPickerClick     widget.Clickable
	filenamePermTextEdit        widget.Editor
	filenamePermIcon            string
	filenamePermIconClick       widget.Clickable
	filenamePermTextPicker      widget.Clickable
	filenamePermApplyClick      widget.Clickable
	filenamePermRemoveClick     widget.Clickable
	filenamePermList            widget.List
	filenamePermEntries         []fm.FilenamePermissionRule
	filenamePermSavedEntries    []fm.FilenamePermissionRule
	filenamePermLookup          string
	filenamePermRowClicks       map[string]*widget.Clickable
	filenamePermRowRemove       map[string]*widget.Clickable
	filenamePermInfoText        string
	filenameExtEdit             widget.Editor
	filenameExtTextEdit         widget.Editor
	filenameExtIcon             string
	filenameExtIconClick        widget.Clickable
	filenameExtTextPicker       widget.Clickable
	filenameExtApplyClick       widget.Clickable
	filenameExtRemoveClick      widget.Clickable
	filenameExtList             widget.List
	filenameExtEntries          []fm.FilenameExtensionRule
	filenameExtSavedEntries     []fm.FilenameExtensionRule
	filenameExtLookup           string
	filenameExtRowClicks        map[string]*widget.Clickable
	filenameExtRowRemove        map[string]*widget.Clickable
	filenameExtInfoText         string
	filenameSizeEdit            widget.Editor
	filenameSizeMatch           string
	filenameSizeMatchAnim       settingsChoiceAnim
	filenameSizeMatchClicks     [2]widget.Clickable
	filenameSizeTextEdit        widget.Editor
	filenameSizeIcon            string
	filenameSizeIconClick       widget.Clickable
	filenameSizeTextPicker      widget.Clickable
	filenameSizeApplyClick      widget.Clickable
	filenameSizeRemoveClick     widget.Clickable
	filenameSizeList            widget.List
	filenameSizeEntries         []fm.FilenameSizeRule
	filenameSizeSavedEntries    []fm.FilenameSizeRule
	filenameSizeLookup          string
	filenameSizeRowClicks       map[string]*widget.Clickable
	filenameSizeRowRemove       map[string]*widget.Clickable
	filenameSizeInfoText        string
	viewCommandEdit             widget.Editor
	viewShellEdit               widget.Editor
	viewRemoteSearchCommandEdit widget.Editor
	paneFontSizeEdit            widget.Editor
	viewFontSizeEdit            widget.Editor
	paneFontFamily              string
	viewFontFamily              string
	paneFontFamilyClicks        []widget.Clickable
	viewFontFamilyClicks        []widget.Clickable
	paneFontPickerAnim          settingsChoiceAnim
	viewFontPickerAnim          settingsChoiceAnim
	generalDimInactiveBool      widget.Bool
	viewSmoothScrollingBool     widget.Bool
	viewHideFunctionBarBool     widget.Bool
	viewerTabList               widget.List
	viewTargetKeyEdit           widget.Editor
	viewTargetCommandEdit       widget.Editor
	viewTargetApplyClick        widget.Clickable
	viewTargetPickClick         widget.Clickable
	viewTargetRemoveClick       widget.Clickable
	viewTargetPickOpen          bool
	viewTargetPickList          widget.List
	viewTargetPickRemember      int
	viewTargetRowClicks         map[string]*widget.Clickable
	viewTargetRowRemoveClicks   map[string]*widget.Clickable
	viewTargetEntries           []viewerCommandTargetEntry
	viewTargetSavedEntries      []viewerCommandTargetEntry
	viewTargetLookupKey         string
	viewRulePatternEdit         widget.Editor
	viewRuleCommandEdit         widget.Editor
	viewRuleApplyClick          widget.Clickable
	viewRulePickClick           widget.Clickable
	viewRuleRemoveClick         widget.Clickable
	viewRulePickOpen            bool
	viewRulePickList            widget.List
	viewRulePickRemember        int
	viewRuleRowClicks           map[string]*widget.Clickable
	viewRuleRowRemoveClicks     map[string]*widget.Clickable
	viewRuleEntries             []fm.ViewerCommandRule
	viewRuleSavedEntries        []fm.ViewerCommandRule
	viewRuleLookupPattern       string
	viewAssocExtEdit            widget.Editor
	viewAssocAppEdit            widget.Editor
	viewAssocApplyClick         widget.Clickable
	viewAssocPickClick          widget.Clickable
	viewAssocRemoveClick        widget.Clickable
	viewAssocPickOpen           bool
	viewAssocPickList           layout.List
	viewAssocPickRemember       int
	viewAssocRowClicks          map[string]*widget.Clickable
	viewAssocEntries            []fm.ViewerAssociation
	viewAssocSavedEntries       []fm.ViewerAssociation
	viewAssocLookupExt          string

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
}

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
	{key: "selection", label: "Focused"},
	{key: "selected_files", label: "Selected Files"},
	{key: "focused_selected", label: "Focused + Selected Files"},
	{key: "current_dir", label: "Current Dir"},
	{key: "scrollbar", label: "Scrollbar"},
}

var settingsViewerColorOptions = []settingsColorOption{
	{key: "normal", label: "Normal"},
	{key: "selection", label: "Selection"},
}

var settingsTabOrder = []string{
	"general",
	"viewer",
	"associations",
	"colors",
	"config",
}

type settingsColorSwatchGroup struct {
	label string
	hexes []string
}

var settingsColorSwatchBases = []settingsColorSwatchGroup{
	{label: "Slate", hexes: settingsShadeRamp("#243244")},
	{label: "Steel", hexes: settingsShadeRamp("#3F556C")},
	{label: "Blue", hexes: settingsShadeRamp(fm.DefaultFilePaneSelectionHex)},
	{label: "Indigo", hexes: settingsShadeRamp("#5B4BC9")},
	{label: "Teal", hexes: settingsShadeRamp("#2D9AA5")},
	{label: "Green", hexes: settingsShadeRamp(fm.DefaultFilePaneSelectedFilesHex)},
	{label: "Olive", hexes: settingsShadeRamp("#7F8E3E")},
	{label: "Amber", hexes: settingsShadeRamp("#A56D2D")},
	{label: "Rose", hexes: settingsShadeRamp("#B94F63")},
	{label: "Orange", hexes: settingsShadeRamp("#D96A3B")},
	{label: "Gray", hexes: settingsShadeRamp("#7F8791")},
}

func settingsShadeRamp(hex string) []string {
	base, ok := fm.ParseHexColor(hex)
	if !ok {
		return nil
	}
	black := color.NRGBA{A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	return []string{
		fm.FormatHexColor(mixNRGBA(base, black, 0.34)),
		fm.FormatHexColor(mixNRGBA(base, black, 0.16)),
		fm.FormatHexColor(base),
		fm.FormatHexColor(mixNRGBA(base, white, 0.16)),
		fm.FormatHexColor(mixNRGBA(base, white, 0.34)),
	}
}

func settingsColorSwatchGroups(current string) []settingsColorSwatchGroup {
	currentHex := fm.NormalizeHexColor(current, fm.DefaultFilePaneSelectionHex)
	groups := make([]settingsColorSwatchGroup, 0, len(settingsColorSwatchBases)+1)
	groups = append(groups, settingsColorSwatchGroup{
		label: "Nearby",
		hexes: settingsShadeRamp(currentHex),
	})
	groups = append(groups, settingsColorSwatchBases...)
	return groups
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
		st.paneFontSizeEdit.SingleLine = true
		st.paneFontSizeEdit.Submit = false
		st.viewFontSizeEdit.SingleLine = true
		st.viewFontSizeEdit.Submit = false
		st.viewerTabList.Axis = layout.Vertical
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
	case "normal", "hover", "selection", "selected_files", "focused_selected", "current_dir", "scrollbar":
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
	st.loadFilenameColorsFromConfig(cfg)
	st.colorCategory = normalizeSettingsColorCategory(st.colorScope, st.colorCategory)
	st.syncColorEditors()
	st.colorCategoryOpen = false
	st.colorCategoryOpenedAt = time.Time{}
	st.colorCategoryHoverID = ""
	st.colorCategoryHoverAnim = segmentedAnimState{}
	st.colorPickerOpen = false
	st.colorPickerTarget = ""
	st.filenameIconPickerOpen = false
	st.filenameIconPickerTarget = ""
	st.viewCommandEdit.SetText(cfg.Viewer.Command)
	st.viewShellEdit.SetText(normalizeViewerShellInput(cfg.Viewer.Shell))
	st.viewRemoteSearchCommandEdit.SetText(fm.NormalizeViewerRemoteSearchCommand(cfg.Viewer.RemoteSearchCommand))
	st.paneFontSizeEdit.SetText(formatConfigFloat(cfg.General.FontSizeSp))
	st.viewFontSizeEdit.SetText(formatConfigFloat(cfg.Viewer.FontSizeSp))
	st.paneFontFamily = cfg.General.Typeface
	st.viewFontFamily = cfg.Viewer.Typeface
	st.paneFontPickerAnim = settingsChoiceAnim{}
	st.viewFontPickerAnim = settingsChoiceAnim{}
	st.generalDimInactiveBool.Value = cfg.General.DimInactivePanes
	st.viewSmoothScrollingBool.Value = cfg.Viewer.SmoothScrolling
	st.viewHideFunctionBarBool.Value = cfg.Viewer.HideFunctionBarWhenOpen
	st.viewerTabList.Position.First = 0
	st.viewerTabList.Position.Offset = 0
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
	case "selected_files":
		st.colorSelectedFilesText = value
	case "normal":
		st.colorPaneText = value
	default:
		st.colorSelectionText = value
	}
}

func settingsViewerCategoryHasText(key string) bool {
	switch key {
	case "selection":
		return false
	default:
		return true
	}
}

func (st *settingsModalState) syncColorEditors() {
	if st == nil {
		return
	}
	st.colorValueEdit.SetText(st.colorValue(st.colorCategory))
	st.colorTextValueEdit.SetText(st.colorTextValue(st.colorCategory))
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
	return settingsColorSwatchGroups(st.colorPickerHexValue(target))
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
		label string
		value string
	}{
		{label: "Pane background", value: bgRaw},
		{label: "Pane text", value: paneTextRaw},
		{label: "Hover background", value: hoverRaw},
		{label: "Hover text", value: hoverTextRaw},
		{label: "Focused selection background", value: selectionRaw},
		{label: "Focused selection text", value: selectionTextRaw},
		{label: "Selected files background", value: selectedFilesRaw},
		{label: "Selected files text", value: selectedFilesTextRaw},
		{label: "Focused + selected files background", value: focusedSelectedRaw},
		{label: "Focused + selected files text", value: focusedSelectedTextRaw},
		{label: "Current dir background", value: currentDirRaw},
		{label: "Current dir text", value: currentDirTextRaw},
		{label: "Scrollbar thumb", value: scrollbarThumbRaw},
		{label: "Scrollbar track", value: scrollbarTrackRaw},
	} {
		if field.value == "" {
			continue
		}
		if _, ok := fm.ParseHexColor(field.value); !ok {
			errText = field.label + " must use #RRGGBB"
			break
		}
	}

	draft := fm.DefaultConfig()
	draft.Colors.FilePaneBackground = fm.NormalizeHexColor(bgRaw, bgFallback)
	draft.Colors.FilePaneText = fm.NormalizeHexColor(paneTextRaw, paneTextFallback)
	draft.Colors.Hover = fm.NormalizeHexColor(hoverRaw, hoverFallback)
	draft.Colors.HoverText = fm.NormalizeHexColor(hoverTextRaw, hoverTextFallback)
	draft.Colors.Selection = fm.NormalizeHexColor(selectionRaw, selectionFallback)
	draft.Colors.SelectionText = fm.NormalizeHexColor(selectionTextRaw, selectionTextFallback)
	draft.Colors.SelectedFiles = fm.NormalizeHexColor(selectedFilesRaw, selectedFilesFallback)
	draft.Colors.SelectedFilesText = fm.NormalizeHexColor(selectedFilesTextRaw, selectedFilesTextFallback)
	draft.Colors.FocusedSelected = fm.NormalizeHexColor(focusedSelectedRaw, focusedSelectedFallback)
	draft.Colors.FocusedSelectedText = fm.NormalizeHexColor(focusedSelectedTextRaw, focusedSelectedTextFallback)
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
		HoverText:           fm.FormatHexColor(palette.HoverFg),
		Selection:           fm.FormatHexColor(palette.SelectedBg),
		SelectionText:       fm.FormatHexColor(palette.SelectedFg),
		SelectedFiles:       fm.FormatHexColor(palette.MarkedBg),
		SelectedFilesText:   fm.FormatHexColor(palette.MarkedFg),
		FocusedSelected:     fm.FormatHexColor(palette.MarkedSelBg),
		FocusedSelectedText: fm.FormatHexColor(palette.MarkedSelFg),
		CurrentDirBg:        fm.FormatHexColor(palette.CurrentDirBg),
		CurrentDirText:      fm.FormatHexColor(palette.CurrentDirFg),
		ScrollbarThumb:      fm.FormatHexColor(palette.ScrollThumb),
		ScrollbarTrack:      fm.FormatHexColor(palette.ScrollTrack),
	}
}

func (st *settingsModalState) draftViewerTheme(cfg *fm.Config) (fileViewerTheme, string) {
	palette, errText := st.draftFilePanePalette(cfg)
	draft := fm.DefaultConfig()
	if cfg != nil {
		draft.General = cfg.General
		draft.Viewer = cfg.Viewer
	}
	draft.Colors = filePanePaletteToConfigColors(palette)

	viewBgFallback := fm.DefaultFilePaneBackgroundHex
	viewTextFallback := fm.DefaultFilePaneTextHex
	viewSelectionFallback := fm.DefaultFilePaneSelectionHex
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
	draft.Viewer.Background = fm.NormalizeHexColor(viewBg, viewBgFallback)
	draft.Viewer.Text = fm.NormalizeHexColor(viewText, viewTextFallback)
	draft.Viewer.Selection = fm.NormalizeHexColor(viewSelection, viewSelectionFallback)
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
	existing, ok := st.viewerCommandTarget(key)
	if !ok {
		st.targetInfoText = "Click Add"
		return
	}
	if existing.Command == command {
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
	if entry, ok := st.viewerCommandTarget(key); ok {
		st.viewTargetCommandEdit.SetText(entry.Command)
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
	if idx := st.viewerCommandTargetIndex(entry.Key); idx >= 0 {
		st.viewTargetEntries[idx] = entry
		action = "Update"
	} else {
		st.viewTargetEntries = append(st.viewTargetEntries, entry)
	}
	st.viewTargetEntries = viewerCommandTargetEntries(viewerCommandTargetMap(st.viewTargetEntries))
	st.loadViewerCommandTargetFields(entry.Key, entry.Command)
	return action, nil
}

func (st *settingsModalState) removeCurrentViewerCommandTarget() bool {
	if st == nil {
		return false
	}
	key := normalizeViewerCommandTargetInput(st.viewTargetKeyEdit.Text())
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
	existing, ok := st.viewerCommandRule(pattern)
	if !ok {
		st.ruleInfoText = "Click Add"
		return
	}
	if existing.Command == command {
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
	if rule, ok := st.viewerCommandRule(pattern); ok {
		st.viewRuleCommandEdit.SetText(rule.Command)
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
	if idx := st.viewerCommandRuleIndex(rule.Pattern); idx >= 0 {
		st.viewRuleEntries[idx] = rule
		action = "Update"
	} else {
		st.viewRuleEntries = append(st.viewRuleEntries, rule)
	}
	st.viewRuleEntries = fm.NormalizeViewerCommandRules(st.viewRuleEntries)
	st.loadViewerCommandRuleFields(rule.Pattern, rule.Command)
	return action, nil
}

func (st *settingsModalState) removeCurrentViewerCommandRule() bool {
	if st == nil {
		return false
	}
	pattern := strings.TrimSpace(st.viewRulePatternEdit.Text())
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
	existing, ok := st.viewerAssociation(ext)
	if !ok {
		st.assocInfoText = "Click Add"
		return
	}
	if existing.AppPath == app {
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
	if assoc, ok := st.viewerAssociation(ext); ok {
		st.viewAssocAppEdit.SetText(assoc.AppPath)
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
	if idx := st.viewerAssociationIndex(assoc.Extension); idx >= 0 {
		st.viewAssocEntries[idx] = assoc
		action = "Update"
	} else {
		st.viewAssocEntries = append(st.viewAssocEntries, assoc)
	}
	st.viewAssocEntries = fm.NormalizeViewerAssociations(st.viewAssocEntries)
	st.viewAssocExtEdit.SetText(viewerAssociationDisplayExtension(assoc.Extension))
	st.viewAssocAppEdit.SetText(assoc.AppPath)
	st.viewAssocLookupExt = assoc.Extension
	return action, nil
}

func (st *settingsModalState) removeCurrentViewerAssociation() bool {
	if st == nil {
		return false
	}
	ext := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
	idx := st.viewerAssociationIndex(ext)
	if idx < 0 {
		return false
	}
	st.viewAssocEntries = append(st.viewAssocEntries[:idx], st.viewAssocEntries[idx+1:]...)
	st.viewAssocExtEdit.SetText(viewerAssociationDisplayExtension(ext))
	st.viewAssocAppEdit.SetText("")
	st.viewAssocLookupExt = ext
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
		gtx.Focused(&st.paneFontSizeEdit) ||
		gtx.Focused(&st.viewFontSizeEdit) ||
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
		lbl.Font.Typeface = ui.mainTypeface()
		lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
	return fillRoundedBox(gtx, gtx.Dp(unit.Dp(8)), fill, border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, label)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.Font.Weight = font.Medium
			lbl.TextSize = scaleModalThemeFontSize(th, 8)
			lbl.Color = fg
			lbl.MaxLines = 1
			lbl.Truncator = "..."
			return layoutVCenteredLabel(gtx, lbl)
		})
	})
}

func (ui *UI) layoutSettingsViewerCard(th *material.Theme, gtx layout.Context, style viewerSettingsSectionStyle, badge, title, note, status string, statusColor color.NRGBA, body layout.Widget) layout.Dimensions {
	return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneOverlayCornerDp)), style.Fill, style.Border, func(gtx layout.Context) layout.Dimensions {
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
							lbl.Font.Typeface = ui.mainTypeface()
							lbl.Font.Weight = font.Medium
							lbl.TextSize = scaleModalThemeFontSize(th, 10)
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
							lbl.Font.Typeface = ui.mainTypeface()
							lbl.Font.Weight = font.Medium
							lbl.TextSize = scaleModalThemeFontSize(th, 10)
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
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
	if st.activeTab == "config" {
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
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorHoverText)); !ok {
		return fmt.Errorf("hover text color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.HoverText = fm.FormatHexColor(c)
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorSelection)); !ok {
		return fmt.Errorf("focused selection color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.Selection = fm.FormatHexColor(c)
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorSelectionText)); !ok {
		return fmt.Errorf("focused selection text color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.SelectionText = fm.FormatHexColor(c)
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorSelectedFiles)); !ok {
		return fmt.Errorf("selected files color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.SelectedFiles = fm.FormatHexColor(c)
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorSelectedFilesText)); !ok {
		return fmt.Errorf("selected files text color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.SelectedFilesText = fm.FormatHexColor(c)
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorFocusedSelected)); !ok {
		return fmt.Errorf("focused + selected files color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.FocusedSelected = fm.FormatHexColor(c)
	}
	if c, ok := fm.ParseHexColor(strings.TrimSpace(st.colorFocusedSelectedText)); !ok {
		return fmt.Errorf("focused + selected files text color must use #RRGGBB")
	} else {
		ui.fmCfg.Colors.FocusedSelectedText = fm.FormatHexColor(c)
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
	switch shell {
	case "auto", "sh", "powershell":
	default:
		return fmt.Errorf("viewer shell must be auto, sh, or powershell")
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

	viewerFontSize, err := strconv.ParseFloat(strings.TrimSpace(st.viewFontSizeEdit.Text()), 32)
	if err != nil || viewerFontSize < 6 {
		return fmt.Errorf("viewer font size must be at least 6")
	}
	paneFontSize, err := strconv.ParseFloat(strings.TrimSpace(st.paneFontSizeEdit.Text()), 32)
	if err != nil || paneFontSize < 6 {
		return fmt.Errorf("pane font size must be at least 6")
	}
	if !resources.IsBundledFontFamily(st.paneFontFamily) && st.paneFontFamily != ui.fmCfg.General.Typeface {
		return fmt.Errorf("pane font family is invalid")
	}
	if !resources.IsBundledFontFamily(st.viewFontFamily) && st.viewFontFamily != ui.fmCfg.Viewer.Typeface {
		return fmt.Errorf("viewer font family is invalid")
	}
	ui.fmCfg.General.Typeface = st.paneFontFamily
	ui.fmCfg.General.FontSizeSp = float32(paneFontSize)
	ui.fmCfg.Viewer.Typeface = st.viewFontFamily
	ui.fmCfg.Viewer.Command = cmd
	ui.fmCfg.Viewer.Shell = shell
	ui.fmCfg.Viewer.RemoteSearchCommand = fm.NormalizeViewerRemoteSearchCommand(st.viewRemoteSearchCommandEdit.Text())
	ui.fmCfg.Viewer.Background = viewerBg
	ui.fmCfg.Viewer.Text = viewerText
	ui.fmCfg.Viewer.Selection = viewerSelection
	ui.fmCfg.Viewer.FontSizeSp = float32(viewerFontSize)
	ui.fmCfg.General.DimInactivePanes = st.generalDimInactiveBool.Value
	ui.fmCfg.Viewer.SmoothScrolling = st.viewSmoothScrollingBool.Value
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
		height := gtx.Dp(unit.Dp(460))
		maxW := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(20))
		maxH := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(20))
		if width > maxW {
			width = maxW
		}
		if height > maxH {
			height = maxH
		}
		if width < 520 {
			width = 520
		}
		if height < 320 {
			height = 320
		}

		m := op.Record(gtx.Ops)
		card := fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
			return minHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
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
								layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSettingsModalBody(th, gtx, st)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
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
		st.tabConfigClick.Hovered() {
		pointer.CursorPointer.Add(gtx.Ops)
	}
}

func (ui *UI) layoutSettingsModalHeader(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body1(th, "Global Settings")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.Font.Weight = font.Bold
			lbl.TextSize = scaleModalThemeFontSize(th, 12)
			lbl.Color = txtColor
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutTinyIconModeButton(th, gtx, &st.closeClick, uitheme.CloseIcon(), false)
		}),
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
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.Font.Weight = font.Medium
				lbl.TextSize = scaleModalThemeFontSize(th, 10)
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
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.Font.Weight = font.Medium
					lbl.TextSize = scaleModalThemeFontSize(th, 10)
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
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.Font.Weight = font.Medium
					lbl.TextSize = scaleModalThemeFontSize(th, 10)
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

func (ui *UI) layoutSettingsNavTabs(th *material.Theme, gtx layout.Context, st *settingsModalState, fillViewer, fillAssoc, fillColors, fillGeneral, fillConfig, hoverViewer, hoverAssoc, hoverColors, hoverGeneral, hoverConfig, pulseViewer, pulseAssoc, pulseColors, pulseGeneral, pulseConfig float32) layout.Dimensions {
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
	totalH := stripH*5 + sepH*4
	pos, animPos := st.tabPosition(gtx.Now)
	if animPos {
		gtx.Execute(op.InvalidateCmd{})
	}
	focusGeneral := float32(0)
	focusViewer := float32(0)
	focusAssoc := float32(0)
	focusColors := float32(0)
	focusConfig := float32(0)
	if st.focus == settingsKeyboardFocusNav {
		switch st.activeTab {
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
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.tabGeneralClick, "General", fillGeneral, hoverGeneral, pulseGeneral, focusGeneral, stripH)
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
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.tabColorsClick, "Colors", fillColors, hoverColors, pulseColors, focusColors, stripH)
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
	rowLabel := func(txt string) layout.Widget {
		return settingsViewerRowLabel(ui, th, txt, true)
	}
	bundledFamilies := resources.BundledFontFamilies()
	st.ensurePaneFontFamilyClicks(len(bundledFamilies))
	st.ensureViewFontFamilyClicks(len(bundledFamilies))
	for i, family := range bundledFamilies {
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
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(rowLabel("Workspace")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			before := st.generalDimInactiveBool.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.generalDimInactiveBool, "Gray out inactive pane", scaleModalThemeFontSize(th, 10))
			if st.generalDimInactiveBool.Value != before {
				st.focus = settingsKeyboardFocusGeneralDimInactive
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusGeneralDimInactive, &st.generalDimInactiveBool)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, "Favorites are managed from the '☆' menu. Use the Config tab for full hexone.yaml editing.")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, 11)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
		layout.Rigid(rowLabel("Fonts")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(settingsViewerRowLabel(ui, th, "Pane font face", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsFontFamilyPicker(th, gtx, bundledFamilies, st.paneFontFamilyClicks, st.paneFontFamily, &st.paneFontPickerAnim, st.focus == settingsKeyboardFocusGeneralPaneFont)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(settingsViewerRowLabel(ui, th, "Pane font size (sp)", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			ed := material.Editor(th, &st.paneFontSizeEdit, "14")
			ed.Font.Typeface = ui.mainTypeface()
			ed.TextSize = scaleModalThemeFontSize(th, 10)
			ed.Color = txtColor
			ed.HintColor = hintColor
			dims := ui.layoutEditorWithContextMenu(th, gtx, "settings-pane-font-size", &st.paneFontSizeEdit, true, func(gtx layout.Context) layout.Dimensions {
				return layoutNeutralEditorBox(gtx, gtx.Focused(&st.paneFontSizeEdit), true, ed.Layout)
			})
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusGeneralPaneFontSize, &st.paneFontSizeEdit)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(settingsViewerRowLabel(ui, th, "Viewer font face", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsFontFamilyPicker(th, gtx, bundledFamilies, st.viewFontFamilyClicks, st.viewFontFamily, &st.viewFontPickerAnim, st.focus == settingsKeyboardFocusGeneralViewFont)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(settingsViewerRowLabel(ui, th, "Viewer font size (sp)", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			ed := material.Editor(th, &st.viewFontSizeEdit, "13")
			ed.Font.Typeface = ui.mainTypeface()
			ed.TextSize = scaleModalThemeFontSize(th, 10)
			ed.Color = txtColor
			ed.HintColor = hintColor
			dims := ui.layoutEditorWithContextMenu(th, gtx, "settings-view-font", &st.viewFontSizeEdit, true, func(gtx layout.Context) layout.Dimensions {
				return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewFontSizeEdit), true, ed.Layout)
			})
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusGeneralViewFontSize, &st.viewFontSizeEdit)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, "Viewer font is reused across file, hex, and command modes. Fira Code or Consolas are the practical choices for dense output.")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, 11)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}),
	)
}

func (ui *UI) layoutSettingsFontFamilyPicker(th *material.Theme, gtx layout.Context, families []resources.BundledFontFamily, clicks []widget.Clickable, active string, anim *settingsChoiceAnim, focused bool) layout.Dimensions {
	if len(families) == 0 {
		return layout.Dimensions{}
	}
	textSize := scaleModalThemeFontSize(th, 10)
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
			Label:      family.Name,
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

func (ui *UI) layoutSettingsModalBody(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	fillViewer, animViewer := st.tabFill(gtx.Now, "viewer")
	fillAssoc, animAssoc := st.tabFill(gtx.Now, "associations")
	fillColors, animColors := st.tabFill(gtx.Now, "colors")
	fillGeneral, animGeneral := st.tabFill(gtx.Now, "general")
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
	if st.tabConfigClick.Hovered() {
		hoverKey = "config"
	}
	st.setHover(hoverKey, gtx.Now)
	hoverViewer, hoverAnimViewer := st.hoverFill(gtx.Now, "viewer")
	hoverAssoc, hoverAnimAssoc := st.hoverFill(gtx.Now, "associations")
	hoverColors, hoverAnimColors := st.hoverFill(gtx.Now, "colors")
	hoverGeneral, hoverAnimGeneral := st.hoverFill(gtx.Now, "general")
	hoverConfig, hoverAnimConfig := st.hoverFill(gtx.Now, "config")
	pulseViewer, pulseAnimViewer := st.pulseFill(gtx.Now, "viewer")
	pulseAssoc, pulseAnimAssoc := st.pulseFill(gtx.Now, "associations")
	pulseColors, pulseAnimColors := st.pulseFill(gtx.Now, "colors")
	pulseGeneral, pulseAnimGeneral := st.pulseFill(gtx.Now, "general")
	pulseConfig, pulseAnimConfig := st.pulseFill(gtx.Now, "config")
	if animViewer || animAssoc || animColors || animGeneral || animConfig ||
		hoverAnimViewer || hoverAnimAssoc || hoverAnimColors || hoverAnimGeneral || hoverAnimConfig ||
		pulseAnimViewer || pulseAnimAssoc || pulseAnimColors || pulseAnimGeneral || pulseAnimConfig {
		gtx.Execute(op.InvalidateCmd{})
	}

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(146)), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsNavTabs(
					th, gtx, st,
					fillViewer, fillAssoc, fillColors, fillGeneral, fillConfig,
					hoverViewer, hoverAssoc, hoverColors, hoverGeneral, hoverConfig,
					pulseViewer, pulseAssoc, pulseColors, pulseGeneral, pulseConfig,
				)
			})
		}),
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
	pickerTargets, pickerTargetMatchCount := st.viewerCommandTargetPickerEntries()

	currentRulePattern := strings.TrimSpace(st.viewRulePatternEdit.Text())
	_, currentRuleExists := st.viewerCommandRule(currentRulePattern)
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
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, 9)
			lbl.Color = color.NRGBA{R: 152, G: 205, B: 152, A: 255}
			lbl.MaxLines = maxLines
			lbl.Truncator = truncator
			return lbl.Layout(gtx)
		}
	}

	sections := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(rowLabel("Shell (all viewer commands)", true)),
				layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(th, &st.viewShellEdit, "auto")
					ed.Font.Typeface = ui.mainTypeface()
					ed.TextSize = scaleModalThemeFontSize(th, 10)
					ed.Color = txtColor
					ed.HintColor = hintColor
					dims := ui.layoutEditorWithContextMenu(th, gtx, "settings-view-shell", &st.viewShellEdit, true, func(gtx layout.Context) layout.Dimensions {
						return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewShellEdit), true, ed.Layout)
					})
					st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusViewerShell, &st.viewShellEdit)
					return dims
				}),
			)
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(rowLabel("Remote search utility command (SSH hex find)", true)),
				layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(th, &st.viewRemoteSearchCommandEdit, fm.DefaultViewerRemoteSearchCommand)
					ed.Font.Typeface = ui.mainTypeface()
					ed.TextSize = scaleModalThemeFontSize(th, 10)
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
			dims := ui.layoutThemeCheckbox(th, gtx, &st.viewSmoothScrollingBool, "Smooth scrolling", scaleModalThemeFontSize(th, 10))
			if st.viewSmoothScrollingBool.Value != before {
				st.focus = settingsKeyboardFocusViewerSmoothScrolling
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusViewerSmoothScrolling, &st.viewSmoothScrollingBool)
			return dims
		},
		func(gtx layout.Context) layout.Dimensions {
			before := st.viewHideFunctionBarBool.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.viewHideFunctionBarBool, "Auto-hide function bar while viewer is open (F11 toggles it)", scaleModalThemeFontSize(th, 10))
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
								ed.Font.Typeface = ui.mainTypeface()
								ed.TextSize = scaleModalThemeFontSize(th, 10)
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
								return layoutTinyModeButtonState(th, gtx, ui.mainTypeface(), &st.viewTargetPickClick, "Browse", st.viewTargetPickOpen, st.focus == settingsKeyboardFocusViewerTargetBrowse)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutTinyModeButtonState(th, gtx, ui.mainTypeface(), &st.viewTargetApplyClick, targetApplyLabel, currentTargetExists, st.focus == settingsKeyboardFocusViewerTargetApply)
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
								ed.Font.Typeface = ui.mainTypeface()
								ed.TextSize = scaleModalThemeFontSize(th, 10)
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
								ed.Font.Typeface = ui.mainTypeface()
								ed.TextSize = scaleModalThemeFontSize(th, 10)
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
								return layoutTinyModeButtonState(th, gtx, ui.mainTypeface(), &st.viewRulePickClick, "Browse", st.viewRulePickOpen, st.focus == settingsKeyboardFocusViewerRuleBrowse)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutTinyModeButtonState(th, gtx, ui.mainTypeface(), &st.viewRuleApplyClick, ruleApplyLabel, currentRuleExists, st.focus == settingsKeyboardFocusViewerRuleApply)
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
								ed.Font.Typeface = ui.mainTypeface()
								ed.TextSize = scaleModalThemeFontSize(th, 10)
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
						ed.Font.Typeface = ui.mainTypeface()
						ed.TextSize = scaleModalThemeFontSize(th, 10)
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
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
													lbl.Font.Typeface = ui.mainTypeface()
													lbl.Font.Weight = font.Medium
													lbl.TextSize = scaleModalThemeFontSize(th, 10)
													lbl.Color = txtColor
													lbl.MaxLines = 1
													lbl.Truncator = "..."
													return layoutVCenteredLabel(gtx, lbl)
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													lbl := material.Caption(th, detail)
													lbl.Font.Typeface = ui.mainTypeface()
													lbl.TextSize = scaleModalThemeFontSize(th, 8)
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
										return layoutTinyIconModeButtonState(gtx, removeClick, uitheme.CloseIcon(), false, removeFocused)
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
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
													lbl.Font.Typeface = ui.mainTypeface()
													lbl.Font.Weight = font.Medium
													lbl.TextSize = scaleModalThemeFontSize(th, 10)
													lbl.Color = txtColor
													lbl.MaxLines = 1
													lbl.Truncator = "..."
													return layoutVCenteredLabel(gtx, lbl)
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													lbl := material.Caption(th, rule.Command)
													lbl.Font.Typeface = ui.mainTypeface()
													lbl.TextSize = scaleModalThemeFontSize(th, 8)
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
										return layoutTinyIconModeButtonState(gtx, removeClick, uitheme.CloseIcon(), false, removeFocused)
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
			return ui.layoutSlidingTabStrip(th, gtx, stripH, pos, scaleModalThemeFontSize(th, 10), []slidingTabSpec{
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
	if size, err := strconv.ParseFloat(strings.TrimSpace(st.viewFontSizeEdit.Text()), 32); err == nil && size >= 6 {
		draft.Viewer.FontSizeSp = float32(size)
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
		return font.Typeface(resources.BundledFontFamilyFiraCode)
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

func settingsViewerPreviewSelectionFill(theme fileViewerTheme, strong bool) color.NRGBA {
	fill := theme.Selection
	if strong {
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

func (ui *UI) layoutSettingsViewerPreviewTextRow(th *material.Theme, gtx layout.Context, st *settingsModalState, theme fileViewerTheme, txt string, fg color.NRGBA, selected bool) layout.Dimensions {
	rowH := st.previewViewerLineHeight(ui, th, gtx, false)
	return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rect(0, 0, gtx.Constraints.Max.X, rowH)).Push(gtx.Ops).Pop()
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				if selected {
					bg := settingsViewerPreviewSelectionFill(theme, false)
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
	columnGap := gtx.Dp(unit.Dp(12))
	if columnGap < charW {
		columnGap = charW
	}
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
	hexColor := theme.Text
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
					lineGtx := gtx
					lineGtx.Constraints.Min.Y = rowH
					lineGtx.Constraints.Max.Y = rowH
					lbl := ui.settingsViewerPreviewLabelStyle(th, ui.viewerMonospaceTypeface(), st.previewViewerTextSize(ui), offset, offsetColor)
					return layoutVCenteredLabel(lineGtx, lbl)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, columnGap, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(columnGap, rowH)}
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, hexW, func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{}.Layout(gtx,
						layout.Expanded(func(gtx layout.Context) layout.Dimensions {
							if selected {
								bg := settingsViewerPreviewSelectionFill(theme, false)
								if rect := settingsViewerPreviewSelectionRect(gtx.Constraints.Max.X, rowH); !rect.Empty() {
									paint.FillShape(gtx.Ops, bg, clip.Rect(rect).Op())
								}
							}
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, rowH)}
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							lineGtx := gtx
							lineGtx.Constraints.Min.Y = rowH
							lineGtx.Constraints.Max.Y = rowH
							lbl := ui.settingsViewerPreviewLabelStyle(th, ui.viewerMonospaceTypeface(), st.previewViewerTextSize(ui), hexText, hexColor)
							return layoutVCenteredLabel(lineGtx, lbl)
						}),
					)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, columnGap, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(columnGap, rowH)}
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, asciiW, func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{}.Layout(gtx,
						layout.Expanded(func(gtx layout.Context) layout.Dimensions {
							if selected {
								bg := settingsViewerPreviewSelectionFill(theme, true)
								if rect := settingsViewerPreviewSelectionRect(gtx.Constraints.Max.X, rowH); !rect.Empty() {
									paint.FillShape(gtx.Ops, bg, clip.Rect(rect).Op())
								}
							}
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, rowH)}
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							lineGtx := gtx
							lineGtx.Constraints.Min.Y = rowH
							lineGtx.Constraints.Max.Y = rowH
							lbl := ui.settingsViewerPreviewLabelStyle(th, ui.viewerMonospaceTypeface(), st.previewViewerTextSize(ui), ascii, asciiColor)
							return layoutVCenteredLabel(lineGtx, lbl)
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

func (st *settingsModalState) previewViewerContentHeight(ui *UI, th *material.Theme, gtx layout.Context) int {
	lineH := st.previewViewerLineHeight(ui, th, gtx, false)
	return lineH * 4
}

func settingsColorsPreviewHostHeight(gtx layout.Context) int {
	height := gtx.Dp(unit.Dp(168))
	if minHeight := gtx.Dp(unit.Dp(148)); height < minHeight {
		height = minHeight
	}
	if height < 1 {
		height = 1
	}
	return height
}

func (ui *UI) layoutSettingsViewerPreviewContent(th *material.Theme, gtx layout.Context, st *settingsModalState, theme fileViewerTheme, previewUI *UI) layout.Dimensions {
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
	previewState := &fileViewerState{
		mode: "file",
		name: "README.md",
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
	if st.colorPickerOpen {
		clickIdx := 0
		for _, group := range activeSwatchGroups {
			for _, hex := range group.hexes {
				if clickIdx >= len(st.colorSwatchClicks) {
					break
				}
				if st.colorSwatchClicks[clickIdx].Clicked(gtx) {
					st.setPopupKeyboardFocus(settingsPopupKeyboardColor, clickIdx, settingsPopupKeyboardActionRow)
					st.setColorPickerHexValue(st.colorPickerTarget, hex)
					st.colorPickerOpen = false
					st.colorPickerTarget = ""
					st.errText = ""
				}
				clickIdx++
			}
		}
	}
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
		layout.Rigid(rowLabel("Colors (#RRGGBB)", true)),
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
			}
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
			}))
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx, children...)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			note := "Use the same category for both background and text. Hover and Focused + Selected Files are tuned separately."
			if st.colorScope == "viewer" {
				note = "Viewer background/text and selection are saved separately from pane colors. Selection only needs a background override."
			} else if st.colorCategory == "scrollbar" {
				note = "Leave scrollbar fields empty to derive contrast from the active pane palette."
			}
			lbl := material.Caption(th, note)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleModalThemeFontSize(th, 9)
				lbl.Color = color.NRGBA{R: 220, G: 140, B: 140, A: 255}
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(rowLabel("Preview", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			hostH := settingsColorsPreviewHostHeight(gtx)
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
	width := settingsColorCategoryWidth(th, gtx, ui.fmCfg, ui.mainTypeface(), options)
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

func settingsColorCategoryWidth(th *material.Theme, gtx layout.Context, _ *fm.Config, face font.Typeface, options []settingsColorOption) int {
	maxTextW := 0
	for _, opt := range options {
		lbl := material.Body2(th, opt.label+"  ▾")
		lbl.Font.Typeface = face
		lbl.TextSize = scaleModalThemeFontSize(th, 10)
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
			return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), bg, bd, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, label)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleModalThemeFontSize(th, 10)
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
							lbl.Font.Typeface = ui.mainTypeface()
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
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleModalThemeFontSize(th, 10)
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
	btnW := settingsColorPickerButtonWidth(th, gtx, ui.fmCfg, ui.mainTypeface())
	edW := settingsColorHexEditorWidth(th, gtx, ui.fmCfg, ui.mainTypeface())
	editorFocused := gtx.Focused(edit) || st.focus == editorFocusTarget || st.focusPending == editorFocusTarget
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, label)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
						ed.Font.Typeface = ui.mainTypeface()
						ed.TextSize = scaleModalThemeFontSize(th, 10)
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

func settingsColorPickerButtonWidth(th *material.Theme, gtx layout.Context, _ *fm.Config, face font.Typeface) int {
	lbl := material.Body2(th, "Pick  ▾")
	lbl.Font.Typeface = face
	lbl.TextSize = scaleModalThemeFontSize(th, 10)
	lbl.MaxLines = 1
	textW := measureLabelUnconstrained(gtx, lbl).Size.X
	width := gtx.Dp(unit.Dp(6)) + gtx.Dp(unit.Dp(14)) + gtx.Dp(unit.Dp(8)) + textW + gtx.Dp(unit.Dp(6))
	minW := gtx.Dp(unit.Dp(90))
	if width < minW {
		width = minW
	}
	return width
}

func settingsColorHexEditorWidth(th *material.Theme, gtx layout.Context, _ *fm.Config, face font.Typeface) int {
	lbl := material.Body2(th, "#RRGGBB")
	lbl.Font.Typeface = face
	lbl.TextSize = scaleModalThemeFontSize(th, 10)
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
			return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), bg, bd, func(gtx layout.Context) layout.Dimensions {
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
							lbl.Font.Typeface = ui.mainTypeface()
							lbl.TextSize = scaleModalThemeFontSize(th, 10)
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
	current := fm.NormalizeHexColor(st.colorPickerHexValue(st.colorPickerTarget), fm.DefaultFilePaneSelectionHex)
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
					children := make([]layout.FlexChild, 0, len(groups)*2)
					clickIdx := 0
					for groupIdx, group := range groups {
						groupIdx := groupIdx
						group := group
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsColorSwatchGroup(th, gtx, st, group, current, &clickIdx)
						}))
						if groupIdx < len(groups)-1 {
							children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout))
						}
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			},
		)
		registerSettingsPopupArea(gtx, &st.colorPickerPopupTag, dims.Size)
		return dims
	})
}

func settingsColorPickerPopupWidth(gtx layout.Context) int {
	labelW := gtx.Dp(unit.Dp(44))
	gap := gtx.Dp(unit.Dp(4))
	swatch := gtx.Dp(unit.Dp(20))
	inset := gtx.Dp(unit.Dp(6))
	width := inset*2 + labelW + gap + swatch*5 + gap*4
	if width < 1 {
		width = 1
	}
	return width
}

func (ui *UI) layoutSettingsColorSwatchGroup(th *material.Theme, gtx layout.Context, st *settingsModalState, group settingsColorSwatchGroup, current string, clickIdx *int) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(44)), func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, group.label)
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleModalThemeFontSize(th, 8)
				lbl.Color = hintColor
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(group.hexes)*2)
			for i, hex := range group.hexes {
				swIdx := *clickIdx
				*clickIdx = *clickIdx + 1
				hex := hex
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					selected := strings.EqualFold(current, fm.NormalizeHexColor(hex, hex))
					focused := st.popupKeyboardMatches(settingsPopupKeyboardColor, swIdx, settingsPopupKeyboardActionRow)
					if st.popupFocusKind == settingsPopupKeyboardColor {
						selected = false
					}
					return ui.layoutSettingsColorSwatch(gtx, &st.colorSwatchClicks[swIdx], parseConfigColorHexFallback(hex, fm.DefaultFilePaneBackgroundHex), selected, focused)
				}))
				if i < len(group.hexes)-1 {
					children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout))
				}
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		}),
	)
}

func (ui *UI) layoutSettingsColorSwatch(gtx layout.Context, click *widget.Clickable, swatch color.NRGBA, selected, focused bool) layout.Dimensions {
	size := gtx.Dp(unit.Dp(20))
	if size < 1 {
		size = 1
	}
	return fixedWidth(gtx, size, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, size, func(gtx layout.Context) layout.Dimensions {
			dims := click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				border := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
				if click.Hovered() {
					border = color.NRGBA{R: 230, G: 236, B: 255, A: 120}
				}
				contrast := bestContrastColor(swatch,
					color.NRGBA{R: 248, G: 250, B: 255, A: 255},
					color.NRGBA{R: 18, G: 22, B: 30, A: 255},
				)
				if selected {
					border = scaleColorAlpha(contrast, 0.8)
				}
				if focused {
					border = scaleColorAlpha(contrast, 0.92)
				}
				return fillRoundedBox(gtx, gtx.Dp(unit.Dp(4)), swatch, border, func(gtx layout.Context) layout.Dimensions {
					if !focused {
						return layout.Dimensions{Size: image.Pt(size, size)}
					}
					return layout.Stack{}.Layout(gtx,
						layout.Expanded(func(gtx layout.Context) layout.Dimensions {
							return layout.Dimensions{Size: image.Pt(size, size)}
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(2), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									barW := gtx.Dp(unit.Dp(3))
									if barW < 1 {
										barW = 1
									}
									return fixedWidth(gtx, barW, func(gtx layout.Context) layout.Dimensions {
										return fixedHeight(gtx, gtx.Constraints.Max.Y, func(gtx layout.Context) layout.Dimensions {
											radius := barW
											if radius < 1 {
												radius = 1
											}
											paint.FillShape(gtx.Ops, contrast, clip.UniformRRect(image.Rect(0, 0, barW, gtx.Constraints.Max.Y), radius).Op(gtx.Ops))
											return layout.Dimensions{Size: image.Pt(barW, gtx.Constraints.Max.Y)}
										})
									})
								})
							})
						}),
					)
				})
			})
			if dims.Size.X > 0 && dims.Size.Y > 0 {
				defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
				pointer.CursorPointer.Add(gtx.Ops)
			}
			return dims
		})
	})
}

func (ui *UI) layoutSettingsColorPreview(th *material.Theme, gtx layout.Context, palette filePanePalette) layout.Dimensions {
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
		palette.PaneBg,
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "Pane Preview")
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
			return ui.layoutSettingsColorPreviewRow(th, gtx, palette.HoverBg, palette.HoverFg, "Hover", "beta.txt")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorPreviewRow(th, gtx, palette.SelectedBg, palette.SelectedFg, "Focused", "gamma.txt")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorPreviewRow(th, gtx, palette.MarkedBg, palette.MarkedFg, "Selected Files", "delta.txt")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorPreviewRow(th, gtx, palette.MarkedSelBg, palette.MarkedSelFg, "Focused + Selected Files", "omega.txt")
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

func (ui *UI) layoutSettingsColorPreviewCurrentDir(th *material.Theme, gtx layout.Context, palette filePanePalette) layout.Dimensions {
	rowH := gtx.Dp(unit.Dp(22))
	if rowH < 1 {
		rowH = 1
	}
	nameSize := scaleConfigFontSize(ui.fmCfg, 13)
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
						return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), rowBg, rowBorder, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(1), Right: unit.Dp(1), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
									return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										gtx.Constraints.Min.X = gtx.Constraints.Max.X
										return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
													layout.Rigid(func(gtx layout.Context) layout.Dimensions {
														lbl := material.Label(th, nameSize, `C:\AsmSource\`)
														lbl.Font.Typeface = ui.mainTypeface()
														lbl.Font.Weight = font.Normal
														lbl.Color = pathColor
														lbl.MaxLines = 1
														return layoutVCenteredLabel(gtx, lbl)
													}),
													layout.Rigid(func(gtx layout.Context) layout.Dimensions {
														return ui.layoutFilePanePathSegmentLabel(th, gtx, "tests", color.NRGBA{}, pathColor, color.NRGBA{}, font.Medium)
													}),
												)
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
	rowH := gtx.Dp(scaleFilePaneDp(ui.fmCfg, 18))
	if rowH < 1 {
		rowH = 1
	}
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

func settingsPreviewColorForCategory(palette filePanePalette, key, part string) color.NRGBA {
	switch key {
	case "normal":
		if part == "text" {
			return palette.PaneFg
		}
		return palette.PaneBg
	case "hover":
		if part == "text" {
			return palette.HoverFg
		}
		return palette.HoverBg
	case "selected_files":
		if part == "text" {
			return palette.MarkedFg
		}
		return palette.MarkedBg
	case "focused_selected":
		if part == "text" {
			return palette.MarkedSelFg
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
			return palette.SelectedFg
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
		ext := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
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
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
						ed.Font.Typeface = ui.mainTypeface()
						ed.TextSize = scaleModalThemeFontSize(th, 10)
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
					return layoutTinyModeButtonState(th, gtx, ui.mainTypeface(), &st.viewAssocPickClick, "Browse", st.viewAssocPickOpen, st.focus == settingsKeyboardFocusAssociationsBrowse)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutTinyModeButtonState(th, gtx, ui.mainTypeface(), &st.viewAssocApplyClick, assocApplyLabel, currentAssocExists, st.focus == settingsKeyboardFocusAssociationsApply)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if statusText == "" {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, statusText)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
					ed.Font.Typeface = ui.mainTypeface()
					ed.TextSize = scaleModalThemeFontSize(th, 10)
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
						return layoutTinyIconModeButtonState(gtx, &st.viewAssocRemoveClick, uitheme.CloseIcon(), false, st.focus == settingsKeyboardFocusAssociationsRemove)
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
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleModalThemeFontSize(th, 9)
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
											lbl.Font.Typeface = ui.mainTypeface()
											lbl.Font.Weight = font.Medium
											lbl.TextSize = scaleModalThemeFontSize(th, 10)
											lbl.Color = txtColor
											lbl.MaxLines = 1
											lbl.Truncator = "..."
											return layoutVCenteredLabel(gtx, lbl)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											lbl := material.Caption(th, program.AppPath)
											lbl.Font.Typeface = ui.mainTypeface()
											lbl.TextSize = scaleModalThemeFontSize(th, 8)
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
	if hoverAnimCancel || hoverAnimSave || pulseAnimCancel || pulseAnimSave {
		gtx.Execute(op.InvalidateCmd{})
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutDialogActionPairState(
				th,
				gtx,
				&st.cancelClick,
				"Cancel",
				hoverCancel,
				pulseCancel,
				false,
				&st.saveClick,
				"Save",
				hoverSave,
				pulseSave,
				false,
				cancelVisual,
				saveVisual,
			)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if st.errText == "" {
				return layout.Dimensions{}
			}
			return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, st.errText)
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleModalThemeFontSize(th, 9)
				lbl.Color = color.NRGBA{R: 255, G: 170, B: 170, A: 255}
				lbl.MaxLines = 2
				lbl.Truncator = "..."
				return lbl.Layout(gtx)
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
				ed.Font.Typeface = ui.mainTypeface()
				ed.TextSize = scaleModalThemeFontSize(th, 10)
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
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, 9)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, "Located at:")
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleModalThemeFontSize(th, 9)
					lbl.Color = hintColor
					return lbl.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return fillRoundedBox(
						gtx,
						gtx.Dp(unit.Dp(filePaneControlCornerDp-1)),
						color.NRGBA{R: 26, G: 29, B: 34, A: 255},
						color.NRGBA{R: 128, G: 152, B: 196, A: 74},
						func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, cfgPath)
								lbl.Font.Typeface = ui.mainTypeface()
								lbl.TextSize = scaleModalThemeFontSize(th, 8)
								lbl.Color = color.NRGBA{R: 194, G: 212, B: 255, A: 255}
								lbl.SelectionColor = color.NRGBA{R: 80, G: 120, B: 220, A: 88}
								lbl.State = &st.configPathSelect
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

func (st *settingsModalState) ensureViewFontFamilyClicks(n int) {
	if n <= cap(st.viewFontFamilyClicks) {
		st.viewFontFamilyClicks = st.viewFontFamilyClicks[:n]
		return
	}
	old := st.viewFontFamilyClicks
	st.viewFontFamilyClicks = make([]widget.Clickable, n)
	copy(st.viewFontFamilyClicks, old)
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
	shell := strings.ToLower(strings.TrimSpace(raw))
	switch shell {
	case "", "auto":
		return "auto"
	case "sh":
		return "sh"
	case "pwsh", "powershell":
		return "powershell"
	default:
		return shell
	}
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
	if ui.tab2State != nil {
		ui.tab2State.typeface = ui.mainTypeface()
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
