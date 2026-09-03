// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"image/color"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"hexone/filesys"
	"hexone/fm"
	uitheme "hexone/ui/theme"
	"hexone/ui/widget/table"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const (
	settingsPaneCharsMin  float32 = 4
	settingsPaneCharsMax  float32 = 80
	settingsPaneCharsStep float32 = 1
)

var settingsPanePreviewTime = time.Date(2026, time.July, 11, 16, 47, 9, 0, time.Local)

const (
	settingsFullPaneWidthExplanation  = "Limits the filename width when the pane gets narrow enough to require shrinking or hiding other columns."
	settingsBriefPaneWidthExplanation = "Only applies when the longest filename exceeds this value. Longer names are trimmed to this width."
)

func normalizeSettingsPaneMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "brief":
		return "brief"
	case "statusbar":
		return "statusbar"
	case "other":
		return "other"
	default:
		return "full"
	}
}

// settingsPaneModeKeys is the file pane section's tab order. Three separate
// places need it in the same order — the tab strip's buttons, the sliding
// indicator's position lookup and the Left/Right arrow stepper — and a strip
// whose keys disagree with the indicator's puts the highlight on the wrong tab.
// It is a package value only so those three readers share one list; nothing
// writes to it, and nothing should. Treat it as read-only.
var settingsPaneModeKeys = []string{"full", "brief", "statusbar", "other"}

func settingsNormalizePaneChars(value, fallback float32) float32 {
	if value <= 0 {
		value = fallback
	}
	if value < settingsPaneCharsMin {
		return settingsPaneCharsMin
	}
	if value > settingsPaneCharsMax {
		return settingsPaneCharsMax
	}
	return value
}

func settingsNormalizePermissionFormat(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "symbolic":
		return "symbolic"
	case "octal":
		return "octal"
	default:
		return "auto"
	}
}

func (st *settingsModalState) stepPaneChars(focus settingsKeyboardFocus, step int) bool {
	if st == nil || step == 0 {
		return false
	}
	var value *float32
	switch focus {
	case settingsKeyboardFocusFilePaneFullChars:
		value = &st.paneFullChars
	case settingsKeyboardFocusFilePaneBriefChars:
		value = &st.paneBriefChars
	default:
		return false
	}
	next := settingsNormalizePaneChars(*value+float32(step)*settingsPaneCharsStep, *value)
	if next == *value {
		return false
	}
	*value = next
	return true
}

func settingsPanePermissionOptions() []terminalShellOption {
	return []terminalShellOption{
		{Key: "off", Label: "Off"},
		{Key: "auto", Label: "Auto"},
		{Key: "symbolic", Label: "rwx"},
		{Key: "octal", Label: "0755"},
	}
}

func settingsPanePermissionChoice(show bool, format string) string {
	if !show {
		return "off"
	}
	return settingsNormalizePermissionFormat(format)
}

func (st *settingsModalState) panePermissionChoice() string {
	if st == nil {
		return "off"
	}
	return settingsPanePermissionChoice(st.paneShowPermissions, st.panePermissionFormat)
}

func (st *settingsModalState) setPanePermissionChoice(next string, now time.Time) bool {
	if st == nil {
		return false
	}
	switch next {
	case "off", "auto", "symbolic", "octal":
	default:
		next = "auto"
	}
	current := st.panePermissionChoice()
	if current == next {
		return false
	}
	animatedChoice := current
	st.panePermissionFormatAnim.setValue(&animatedChoice, next, now)
	st.panePermissionFormatAnim.anim.setPulse(next, now)
	st.paneShowPermissions = next != "off"
	if next != "off" {
		st.panePermissionFormat = next
	}
	return true
}

func (st *settingsModalState) stepPanePermissionFormat(step int, now time.Time) bool {
	if st == nil || step == 0 {
		return false
	}
	options := settingsPanePermissionOptions()
	keys := make([]string, len(options))
	for i, option := range options {
		keys[i] = option.Key
	}
	current := st.panePermissionChoice()
	next := settingsChoiceStep(current, keys, step)
	if next == "" || next == current {
		return false
	}
	return st.setPanePermissionChoice(next, now)
}

func settingsPaneDateOptions() []terminalShellOption {
	return []terminalShellOption{
		{Key: "friendly", Label: "Jul 11 2026"},
		{Key: "iso", Label: "2026-07-11"},
		{Key: "day_first", Label: "11 Jul 2026"},
		{Key: "slash", Label: "07/11/2026"},
	}
}

func settingsPaneTimeOptions() []terminalShellOption {
	return []terminalShellOption{
		{Key: "none", Label: "No time"},
		{Key: "minutes", Label: "16:47"},
		{Key: "seconds", Label: "16:47:09"},
		{Key: "twelve", Label: "4:47 PM"},
	}
}

func settingsPaneDateLayout(key string) string {
	switch key {
	case "iso":
		return "2006-01-02"
	case "day_first":
		return "02 Jan 2006"
	case "slash":
		return "01/02/2006"
	default:
		return "Jan 02 2006"
	}
}

func settingsPaneTimeLayout(key string) string {
	switch key {
	case "minutes":
		return "15:04"
	case "seconds":
		return "15:04:05"
	case "twelve":
		return "3:04 PM"
	default:
		return ""
	}
}

func settingsPaneCombinedDateLayout(dateKey, timeKey string) string {
	dateLayout := settingsPaneDateLayout(dateKey)
	timeLayout := settingsPaneTimeLayout(timeKey)
	if timeLayout == "" {
		return dateLayout
	}
	return dateLayout + " " + timeLayout
}

func settingsDetectPaneDatePresets(format string) (dateKey, timeKey string) {
	format = strings.TrimSpace(format)
	timeKey = "none"
	for _, option := range settingsPaneTimeOptions()[1:] {
		timeLayout := settingsPaneTimeLayout(option.Key)
		if strings.HasSuffix(format, " "+timeLayout) {
			timeKey = option.Key
			format = strings.TrimSpace(strings.TrimSuffix(format, timeLayout))
			break
		}
	}
	for _, option := range settingsPaneDateOptions() {
		if format == settingsPaneDateLayout(option.Key) {
			return option.Key, timeKey
		}
	}
	return "custom", timeKey
}

func (st *settingsModalState) loadPaneDateFormat(formats []string) {
	if st == nil {
		return
	}
	primary := "Jan 02 2006"
	if len(formats) > 0 && strings.TrimSpace(formats[0]) != "" {
		primary = strings.TrimSpace(formats[0])
	}
	st.paneDatePreset, st.paneTimePreset = settingsDetectPaneDatePresets(primary)
	st.paneDateFormatEdit.SetText(primary)
	st.paneDatePresetAnim = settingsChoiceAnim{}
	st.paneTimePresetAnim = settingsChoiceAnim{}
	st.paneDateFallbackFormats = append(st.paneDateFallbackFormats[:0], formats[1:]...)
}

func (st *settingsModalState) paneDateFormats() []string {
	if st == nil {
		return []string{"Jan 02 2006"}
	}
	if st.paneDatePreset != "custom" {
		return settingsGeneratedPaneDateFormats(st.paneDatePreset, st.paneTimePreset)
	}
	primary := strings.TrimSpace(st.paneDateFormatEdit.Text())
	if primary == "" {
		primary = "Jan 02 2006"
	}
	out := []string{primary}
	seen := map[string]bool{primary: true}
	for _, candidate := range st.paneDateFallbackFormats {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		out = append(out, candidate)
	}
	return out
}

func settingsGeneratedPaneDateFormats(dateKey, timeKey string) []string {
	fullDate := settingsPaneDateLayout(dateKey)
	mediumDate := "Jan 02"
	compactDate := "01-02"
	switch dateKey {
	case "iso":
		mediumDate, compactDate = "01-02", "01-02"
	case "day_first":
		mediumDate, compactDate = "02 Jan", "02-01"
	case "slash":
		mediumDate, compactDate = "01/02", "01/02"
	}
	timeLayout := settingsPaneTimeLayout(timeKey)
	candidates := make([]string, 0, 7)
	if timeLayout != "" {
		candidates = append(candidates, fullDate+" "+timeLayout)
		if timeKey == "seconds" {
			candidates = append(candidates, fullDate+" 15:04")
		}
		candidates = append(candidates, mediumDate+" "+timeLayout)
	}
	candidates = append(candidates, fullDate, mediumDate, compactDate)
	out := make([]string, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, format := range candidates {
		format = strings.TrimSpace(format)
		if format == "" || seen[format] {
			continue
		}
		seen[format] = true
		out = append(out, format)
	}
	return out
}

func (st *settingsModalState) applyPaneDatePresets() {
	if st == nil || st.paneDatePreset == "custom" {
		return
	}
	st.paneDateFormatEdit.SetText(settingsPaneCombinedDateLayout(st.paneDatePreset, st.paneTimePreset))
}

func (st *settingsModalState) stepPaneSettingsMode(step int, now time.Time) bool {
	if st == nil || step == 0 {
		return false
	}
	current := normalizeSettingsPaneMode(st.paneSettingsMode)
	next := settingsChoiceStep(current, settingsPaneModeKeys, step)
	if next == current || next == "" {
		return false
	}
	st.paneSettingsModeAnim.setValue(&st.paneSettingsMode, next, now)
	st.paneSettingsModeAnim.anim.setPulse(next, now)
	return true
}

func (st *settingsModalState) stepPaneDatePreset(step int, now time.Time) bool {
	if st == nil || step == 0 {
		return false
	}
	options := settingsPaneDateOptions()
	keys := make([]string, len(options))
	for i := range options {
		keys[i] = options[i].Key
	}
	current := st.paneDatePreset
	if current == "custom" {
		current = keys[0]
	}
	next := settingsChoiceStep(current, keys, step)
	if next == "" || next == st.paneDatePreset {
		return false
	}
	st.paneDatePresetAnim.setValue(&st.paneDatePreset, next, now)
	st.paneDatePresetAnim.anim.setPulse(next, now)
	st.applyPaneDatePresets()
	return true
}

func (st *settingsModalState) stepPaneTimePreset(step int, now time.Time) bool {
	if st == nil || step == 0 {
		return false
	}
	options := settingsPaneTimeOptions()
	keys := make([]string, len(options))
	for i := range options {
		keys[i] = options[i].Key
	}
	current := st.paneTimePreset
	next := settingsChoiceStep(current, keys, step)
	if next == "" || next == current {
		return false
	}
	st.paneTimePresetAnim.setValue(&st.paneTimePreset, next, now)
	st.paneTimePresetAnim.anim.setPulse(next, now)
	if st.paneDatePreset == "custom" {
		st.paneDatePreset = "friendly"
	}
	st.applyPaneDatePresets()
	return true
}

func (ui *UI) layoutSettingsFilePaneEditor(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	mode := normalizeSettingsPaneMode(st.paneSettingsMode)
	st.paneSettingsMode = mode
	modeClicks := []struct {
		key   string
		label string
		click *widget.Clickable
	}{
		{key: "full", label: "Full mode", click: &st.paneSettingsFullClick},
		{key: "brief", label: "Brief mode", click: &st.paneSettingsBriefClick},
		{key: "statusbar", label: "Status bar", click: &st.paneSettingsStatusBarClick},
		{key: "other", label: "Other", click: &st.paneSettingsOtherClick},
	}
	for _, item := range modeClicks {
		for item.click.Clicked(gtx) {
			st.paneSettingsModeAnim.setValue(&st.paneSettingsMode, item.key, gtx.Now)
			st.paneSettingsModeAnim.anim.setPulse(item.key, gtx.Now)
			st.setKeyboardFocus(settingsKeyboardFocusFilePaneMode)
			st.generalTabList.Position = layout.Position{}
		}
	}
	hoverKey := ""
	for _, item := range modeClicks {
		if item.click.Hovered() {
			hoverKey = item.key
		}
	}
	st.paneSettingsModeAnim.anim.setHover(hoverKey, gtx.Now)
	mode = normalizeSettingsPaneMode(st.paneSettingsMode)
	pos, posAnim := st.paneSettingsModeAnim.position(gtx.Now, mode, settingsPaneModeKeys)
	tabs := make([]slidingTabSpec, 0, len(modeClicks))
	animating := posAnim
	for _, item := range modeClicks {
		active, a := st.paneSettingsModeAnim.fill(gtx.Now, mode, item.key)
		hover, h := st.paneSettingsModeAnim.anim.hoverFill(gtx.Now, item.key)
		pulse, p := st.paneSettingsModeAnim.anim.pulseFill(gtx.Now, item.key)
		focus := float32(0)
		if st.focus == settingsKeyboardFocusFilePaneMode && item.key == mode {
			focus = 1
		}
		animating = animating || a || h || p
		tabs = append(tabs, slidingTabSpec{Label: item.label, Click: item.click, ActiveFill: active, HoverFill: hover, PulseFill: pulse, FocusFill: focus})
	}
	if animating {
		gtx.Execute(op.InvalidateCmd{})
	}
	stripH := gtx.Dp(unit.Dp(24))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSlidingTabStrip(th, gtx, stripH, pos, ui.scaleModalFontSize(10), tabs)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			switch mode {
			case "brief":
				return ui.layoutSettingsPaneBriefTab(th, gtx, st)
			case "statusbar":
				return ui.layoutSettingsPaneStatusBarTab(th, gtx, st)
			case "other":
				return ui.layoutSettingsPaneOtherTab(th, gtx, st)
			default:
				return ui.layoutSettingsPaneFullTab(th, gtx, st)
			}
		}),
	)
}

func (ui *UI) settingsPaneControlLabel(th *material.Theme, txt string) layout.Widget {
	return settingsViewerRowLabel(ui, th, txt, true)
}

func (ui *UI) layoutSettingsPaneCharsStepper(th *material.Theme, gtx layout.Context, st *settingsModalState, stepper *settingsNumberStepperState, value float32, focus settingsKeyboardFocus) layout.Dimensions {
	if st == nil || stepper == nil {
		return layout.Dimensions{}
	}
	for stepper.valueClick.Clicked(gtx) {
		st.setKeyboardFocus(focus)
	}
	for stepper.upClick.Clicked(gtx) {
		st.setKeyboardFocus(focus)
		st.stepPaneChars(focus, 1)
	}
	for stepper.downClick.Clicked(gtx) {
		st.setKeyboardFocus(focus)
		st.stepPaneChars(focus, -1)
	}
	focused := st.focus == focus
	height := gtx.Dp(unit.Dp(22))
	if stepper.valueClick.Hovered() || stepper.upClick.Hovered() || stepper.downClick.Hovered() {
		pointer.CursorPointer.Add(gtx.Ops)
	}
	return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsFontSizeValue(th, gtx, &stepper.valueClick, value, focused, ui.scaleModalFontSize(10))
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, gtx.Dp(unit.Dp(17)), func(gtx layout.Context) layout.Dimensions {
					half := height / 2
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

func (ui *UI) layoutSettingsPaneWidthRow(th *material.Theme, gtx layout.Context, st *settingsModalState, label, explanation string, stepper *settingsNumberStepperState, helpClick *widget.Clickable, value float32, focus settingsKeyboardFocus) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(ui.settingsPaneControlLabel(th, label)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, gtx.Dp(unit.Dp(78)), func(gtx layout.Context) layout.Dimensions {
						return ui.layoutSettingsPaneCharsStepper(th, gtx, st, stepper, value, focus)
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsHelpIcon(th, gtx, helpClick, explanation)
				}),
			)
		}),
	)
}

func (ui *UI) layoutSettingsPaneFullTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	permissionOptions := settingsPanePermissionOptions()
	for i, option := range permissionOptions {
		for st.panePermissionFormatClicks[i].Clicked(gtx) {
			st.setKeyboardFocus(settingsKeyboardFocusFilePanePermissionFormat)
			st.setPanePermissionChoice(option.Key, gtx.Now)
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsPaneWidthRow(th, gtx, st, "Filename column width", settingsFullPaneWidthExplanation, &st.paneFullCharsStepper, &st.paneFullCharsHelpClick, st.paneFullChars, settingsKeyboardFocusFilePaneFullChars)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(ui.settingsPaneControlLabel(th, "Permissions column")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(256)), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsShellPicker(th, gtx, permissionOptions, st.panePermissionFormatClicks[:], st.panePermissionChoice(), &st.panePermissionFormatAnim, st.focus == settingsKeyboardFocusFilePanePermissionFormat)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.layoutSettingsPaneDateBuilder(th, gtx, st) }),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.layoutSettingsFullPanePreview(th, gtx, st) }),
	)
}

func (ui *UI) layoutSettingsPaneBriefTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsPaneWidthRow(th, gtx, st, "Maximum filename width", settingsBriefPaneWidthExplanation, &st.paneBriefCharsStepper, &st.paneBriefCharsHelpClick, st.paneBriefChars, settingsKeyboardFocusFilePaneBriefChars)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.layoutSettingsBriefPanePreview(th, gtx, st) }),
	)
}

func (ui *UI) layoutSettingsPaneOtherTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	for i, option := range settingsCompletionSoundOptions() {
		for st.generalCompletionSoundClicks[i].Clicked(gtx) {
			st.setKeyboardFocus(settingsKeyboardFocusGeneralCompletionSound)
			st.generalCompletionSoundAnim.setValue(&st.generalCompletionSound, option.Key, gtx.Now)
			st.generalCompletionSoundAnim.anim.setPulse(option.Key, gtx.Now)
		}
	}
	sectionLabel := func(txt string) layout.Widget { return settingsViewerRowLabel(ui, th, txt, true) }
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			before := st.generalDimInactiveBool.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.generalDimInactiveBool, "Gray out inactive pane", ui.scaleModalFontSize(10))
			if before != st.generalDimInactiveBool.Value {
				st.focus = settingsKeyboardFocusGeneralDimInactive
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusGeneralDimInactive, &st.generalDimInactiveBool)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			before := st.generalFavoritesNewTabBool.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.generalFavoritesNewTabBool, "Open ☆ favorites in a new tab", ui.scaleModalFontSize(10))
			if before != st.generalFavoritesNewTabBool.Value {
				st.focus = settingsKeyboardFocusGeneralFavoritesNewTab
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusGeneralFavoritesNewTab, &st.generalFavoritesNewTabBool)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			before := st.generalWheelMovesSelection.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.generalWheelMovesSelection, "Mouse wheel moves the active item", ui.scaleModalFontSize(10))
			if before != st.generalWheelMovesSelection.Value {
				st.focus = settingsKeyboardFocusGeneralWheelMovesSelection
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusGeneralWheelMovesSelection, &st.generalWheelMovesSelection)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			before := st.generalUseTrash.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.generalUseTrash, "Move deleted items to Trash / Recycle Bin", ui.scaleModalFontSize(10))
			if before != st.generalUseTrash.Value {
				st.focus = settingsKeyboardFocusGeneralUseTrash
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusGeneralUseTrash, &st.generalUseTrash)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			before := st.generalDeleteWithoutConfirm.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.generalDeleteWithoutConfirm, "Delete without confirmation", ui.scaleModalFontSize(10))
			if before != st.generalDeleteWithoutConfirm.Value {
				st.focus = settingsKeyboardFocusGeneralDeleteWithoutConfirm
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusGeneralDeleteWithoutConfirm, &st.generalDeleteWithoutConfirm)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.layoutSettingsCompletionSoundRow(th, gtx, st) }),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(sectionLabel("Font weights")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsPaneWeightRow(th, gtx, st, "Files", st.paneFileWeightClicks[:], &st.paneFileWeight, &st.paneFileWeightAnim, settingsKeyboardFocusFilePaneFileWeight, fm.FontWeightRegular)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsPaneWeightRow(th, gtx, st, "Dirs", st.paneDirWeightClicks[:], &st.paneDirWeight, &st.paneDirWeightAnim, settingsKeyboardFocusFilePaneDirWeight, fm.FontWeightBold)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsPaneWeightRow(th, gtx, st, "Permissions", st.panePermissionsWeightClicks[:], &st.panePermissionsWeight, &st.panePermissionsWeightAnim, settingsKeyboardFocusFilePanePermissionsWeight, fm.FontWeightRegular)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsPaneWeightRow(th, gtx, st, "Size", st.paneSizeWeightClicks[:], &st.paneSizeWeight, &st.paneSizeWeightAnim, settingsKeyboardFocusFilePaneSizeWeight, fm.FontWeightRegular)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsPaneWeightRow(th, gtx, st, "Date", st.paneDateWeightClicks[:], &st.paneDateWeight, &st.paneDateWeightAnim, settingsKeyboardFocusFilePaneDateWeight, fm.FontWeightRegular)
		}),
	)
}

func (ui *UI) layoutSettingsPaneDateBuilder(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	dateOptions := settingsPaneDateOptions()
	timeOptions := settingsPaneTimeOptions()
	for i, option := range dateOptions {
		for st.paneDatePresetClicks[i].Clicked(gtx) {
			st.setKeyboardFocus(settingsKeyboardFocusFilePaneDateStyle)
			st.paneDatePresetAnim.setValue(&st.paneDatePreset, option.Key, gtx.Now)
			st.paneDatePresetAnim.anim.setPulse(option.Key, gtx.Now)
			st.applyPaneDatePresets()
		}
	}
	for i, option := range timeOptions {
		for st.paneTimePresetClicks[i].Clicked(gtx) {
			st.setKeyboardFocus(settingsKeyboardFocusFilePaneTimeStyle)
			st.paneTimePresetAnim.setValue(&st.paneTimePreset, option.Key, gtx.Now)
			st.paneTimePresetAnim.anim.setPulse(option.Key, gtx.Now)
			if st.paneDatePreset == "custom" {
				st.paneDatePreset = "friendly"
			}
			st.applyPaneDatePresets()
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(settingsViewerRowLabel(ui, th, "Date & time format", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsShellPicker(th, gtx, dateOptions, st.paneDatePresetClicks[:], st.paneDatePreset, &st.paneDatePresetAnim, st.focus == settingsKeyboardFocusFilePaneDateStyle)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsShellPicker(th, gtx, timeOptions, st.paneTimePresetClicks[:], st.paneTimePreset, &st.paneTimePresetAnim, st.focus == settingsKeyboardFocusFilePaneTimeStyle)
		}),
	)
}

type settingsPanePreviewRow struct {
	name string
	kind filesys.EntryKind
	size string
}

var settingsPanePreviewRows = []settingsPanePreviewRow{
	{name: "..", kind: filesys.EntryParent},
	{name: "Projects", kind: filesys.EntryDir},
	{name: "devonthink_index.applescript", kind: filesys.EntryFile, size: "12.4 KB"},
	{name: "photos.rar", kind: filesys.EntryFile, size: "824 MB"},
	{name: "hexone.exe", kind: filesys.EntryFile, size: "38.2 MB"},
}

var settingsBriefPanePreviewRows = []settingsPanePreviewRow{
	{name: "..", kind: filesys.EntryParent},
	{name: "Projects", kind: filesys.EntryDir},
	{name: "devonthink_index.applescript", kind: filesys.EntryFile},
	{name: "photos.rar", kind: filesys.EntryFile},
	{name: "hexone.exe", kind: filesys.EntryFile},
	{name: "Documents", kind: filesys.EntryDir},
	{name: "archive-part1.rar", kind: filesys.EntryFile},
	{name: "quarterly-financial-report-2026-final.xlsx", kind: filesys.EntryFile},
	{name: "music.flac", kind: filesys.EntryFile},
	{name: "todo.md", kind: filesys.EntryFile},
	{name: "Downloads", kind: filesys.EntryDir},
	{name: "backup-2026.zip", kind: filesys.EntryFile},
	{name: "customer-database-production-backup-2026-07-17.tar.zst", kind: filesys.EntryFile},
	{name: "server.log", kind: filesys.EntryFile},
	{name: "installer.msi", kind: filesys.EntryFile},
	{name: "Source", kind: filesys.EntryDir},
	{name: "main.go", kind: filesys.EntryFile},
	{name: "README.md", kind: filesys.EntryFile},
	{name: "sample.mp4", kind: filesys.EntryFile},
	{name: "data.csv", kind: filesys.EntryFile},
}

func settingsBriefPreviewLongestNameChars() float32 {
	longest := float32(1)
	for _, row := range settingsBriefPanePreviewRows {
		if chars := float32(utf8.RuneCountInString(row.name)); chars > longest {
			longest = chars
		}
	}
	return longest
}

func settingsBriefPreviewColumnChars(configured float32) float32 {
	configured = settingsNormalizePaneChars(configured, 16)
	longest := settingsBriefPreviewLongestNameChars()
	if longest > configured {
		return configured
	}
	return longest
}

func (ui *UI) settingsPanePreviewIcon(row settingsPanePreviewRow) table.LeadingIcon {
	entry := filesys.Entry{Name: row.name, DisplayName: row.name, Kind: row.kind}
	model := &filePaneModel{
		entries:       []filesys.Entry{entry},
		cfg:           ui.fmCfg,
		filenameTheme: newFilePaneFilenameTheme(ui.fmCfg),
	}
	model.rebuildFilenameVisuals(settingsPanePreviewTime)
	icon, _ := model.LeadingIcon(0, 0)
	return icon
}

func (ui *UI) layoutSettingsPanePreviewNameCell(th *material.Theme, gtx layout.Context, st *settingsModalState, row settingsPanePreviewRow, width int, fg color.NRGBA) layout.Dimensions {
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(fm.ColumnPadDp()), Right: unit.Dp(fm.ColumnPadDp())}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			icon := ui.settingsPanePreviewIcon(row)
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return table.LayoutLeadingIcon(gtx, icon)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(th, scaleConfigFontSize(ui.fmCfg, 13), row.name)
					lbl.Font.Typeface = font.Typeface(st.paneFontFamily)
					lbl.Font.Weight = font.Normal
					if row.kind == filesys.EntryDir || row.kind == filesys.EntryParent {
						lbl.Font.Weight = font.Bold
					}
					lbl.Color = fg
					lbl.MaxLines = 1
					lbl.Truncator = "…"
					return layoutVCenteredLabel(gtx, lbl)
				}),
			)
		})
	})
}

func (ui *UI) settingsPaneDraftPalette(st *settingsModalState) filePanePalette {
	if st != nil {
		if palette, errText := st.draftFilePanePalette(ui.fmCfg); errText == "" {
			return palette
		}
	}
	return filePanePaletteFromConfig(ui.fmCfg)
}

// settingsPanePreviewFrameHeightDp is the fixed height of all three pane
// preview frames. Named because the status bar mock's geometry — and the tests
// that read the strip out of the frame's bottom edge — are stated against it.
const settingsPanePreviewFrameHeightDp = unit.Dp(154)

func (ui *UI) layoutSettingsPanePreviewFrame(th *material.Theme, gtx layout.Context, title string, st *settingsModalState, content layout.Widget) layout.Dimensions {
	palette := ui.settingsPaneDraftPalette(st)
	height := gtx.Dp(settingsPanePreviewFrameHeightDp)
	return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, gtx.Dp(unit.Dp(23)), func(gtx layout.Context) layout.Dimensions {
					return fillBgExact(gtx, mixNRGBA(palette.PaneBg, palette.CurrentDirBg, 0.22), func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									lbl := material.Caption(th, `C:\Users\demo\Downloads`)
									lbl.Font.Typeface = ui.interfaceTypeface()
									lbl.TextSize = ui.scaleModalFontSize(8)
									lbl.Color = palette.CurrentDirFg
									return layoutVCenteredLabel(gtx, lbl)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									lbl := material.Caption(th, title)
									lbl.Font.Typeface = ui.interfaceTypeface()
									lbl.Font.Weight = font.Medium
									lbl.TextSize = ui.scaleModalFontSize(8)
									lbl.Color = settingsColorPreviewStateColor(palette.PaneBg)
									return layoutVCenteredLabel(gtx, lbl)
								}),
							)
						})
					})
				})
			}),
			layout.Flexed(1, content),
		)
	})
}

func (ui *UI) layoutSettingsFullPanePreview(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	return ui.layoutSettingsPanePreviewFrame(th, gtx, "FULL MODE PREVIEW", st, func(gtx layout.Context) layout.Dimensions {
		palette := ui.settingsPaneDraftPalette(st)
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: unit.Dp(6), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsFullPaneRows(th, gtx, st, palette)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsFilePanePreviewScrollbar(gtx, palette)
			}),
		)
	})
}

func (ui *UI) layoutSettingsFullPaneRows(th *material.Theme, gtx layout.Context, st *settingsModalState, palette filePanePalette) layout.Dimensions {
	draft := *ui.fmCfg
	draft.Columns.NameChars = settingsNormalizePaneChars(st.paneFullChars, 20)
	draft.Columns.PermissionFormat = settingsNormalizePermissionFormat(st.panePermissionFormat)
	draft.DateFormats = st.paneDateFormats()
	nameW := gtx.Dp(scaleFilePaneDp(&draft, fm.NameWidthDp(&draft)))
	permW := gtx.Dp(scaleFilePaneDp(&draft, fm.PermWidthDp(&draft)))
	sizeW := gtx.Dp(scaleFilePaneDp(&draft, fm.SizeWidthDp(&draft)))
	dateW := gtx.Dp(scaleFilePaneDp(&draft, fm.DateWidthDp(&draft)))
	gapDp := scaleFilePaneDp(&draft, fm.FullColumnGapDp())
	gapW := gtx.Dp(gapDp)
	gapCount := 3
	if !st.paneShowPermissions {
		permW = 0
		gapCount = 2
	}
	nameW = gtx.Constraints.Max.X - permW - sizeW - dateW - gapCount*gapW
	minNameW := gtx.Dp(scaleFilePaneDp(&draft, fm.NameMinWidthDp(&draft)))
	if nameW < minNameW {
		nameW = minNameW
	}
	format := strings.TrimSpace(st.paneDateFormatEdit.Text())
	perm := "rwxr-xr-x"
	if settingsNormalizePermissionFormat(st.panePermissionFormat) == "octal" {
		perm = "0755"
	}
	cell := func(gtx layout.Context, txt string, width int, weight font.Weight, align text.Alignment, fg color.NRGBA) layout.Dimensions {
		return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(fm.ColumnPadDp()), Right: unit.Dp(fm.ColumnPadDp())}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(th, scaleConfigFontSize(ui.fmCfg, 13), txt)
				lbl.Font.Typeface = font.Typeface(st.paneFontFamily)
				lbl.Font.Weight = weight
				lbl.Color = fg
				lbl.Alignment = align
				lbl.MaxLines = 1
				lbl.Truncator = "…"
				return layoutVCenteredLabel(gtx, lbl)
			})
		})
	}
	previewRows := settingsPanePreviewRows[:4]
	children := make([]layout.FlexChild, 0, len(previewRows))
	gap := func() layout.FlexChild {
		return layout.Rigid(layout.Spacer{Width: gapDp}.Layout)
	}
	for i, row := range previewRows {
		i, row := i, row
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			rowH := gtx.Dp(unit.Dp(18))
			bg := palette.PaneBg
			fg := palette.PaneFg
			if i == 2 {
				bg = palette.SelectedBg
				fg = settingsEffectivePaneRowTextColor(palette, palette.SelectedFg)
			}
			return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
				return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
					cols := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.layoutSettingsPanePreviewNameCell(th, gtx, st, row, nameW, fg)
					})}
					if permW > 0 {
						cols = append(cols, gap(), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return cell(gtx, perm, permW, font.Normal, text.Start, fg) }))
					}
					cols = append(cols,
						gap(),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return cell(gtx, row.size, sizeW, font.Normal, text.End, fg)
						}),
						gap(),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return cell(gtx, settingsPanePreviewTime.Format(format), dateW, font.Normal, text.Start, fg)
						}),
					)
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, cols...)
				})
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (ui *UI) layoutSettingsBriefPanePreview(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	return ui.layoutSettingsPanePreviewFrame(th, gtx, "BRIEF MODE PREVIEW", st, func(gtx layout.Context) layout.Dimensions {
		palette := ui.settingsPaneDraftPalette(st)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					colW := gtx.Dp(unit.Dp(settingsBriefPreviewColumnChars(st.paneBriefChars)*6 + 22))
					count := gtx.Constraints.Max.X / max(1, colW)
					if count < 1 {
						count = 1
					}
					if count > 4 {
						count = 4
					}
					columns := make([]layout.FlexChild, 0, count)
					for col := 0; col < count; col++ {
						col := col
						columns = append(columns, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, colW, func(gtx layout.Context) layout.Dimensions {
								rows := make([]layout.FlexChild, 0, 5)
								for row := 0; row < 5; row++ {
									idx := col*5 + row
									entry := settingsBriefPanePreviewRows[idx]
									rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										bg := palette.PaneBg
										if idx == 7 {
											bg = palette.SelectedBg
										}
										return fixedHeight(gtx, gtx.Dp(unit.Dp(18)), func(gtx layout.Context) layout.Dimensions {
											return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
												fg := palette.PaneFg
												if idx == 7 {
													fg = settingsEffectivePaneRowTextColor(palette, palette.SelectedFg)
												}
												return ui.layoutSettingsPanePreviewNameCell(th, gtx, st, entry, max(1, gtx.Constraints.Max.X), fg)
											})
										})
									}))
								}
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
							})
						}))
					}
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, columns...)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, gtx.Dp(unit.Dp(8)), func(gtx layout.Context) layout.Dimensions {
					track, thumb := settingsPreviewScrollbarGeometry(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(8)), max(gtx.Dp(unit.Dp(64)), gtx.Constraints.Max.X/3))
					paintSettingsPreviewRoundedRect(gtx, track, palette.ScrollTrackH)
					paintSettingsPreviewRoundedRect(gtx, thumb, palette.ScrollThumbH)
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(8)))}
				})
			}),
		)
	})
}

func (ui *UI) layoutSettingsFilePanePreviewScrollbar(gtx layout.Context, palette filePanePalette) layout.Dimensions {
	trackW := gtx.Dp(unit.Dp(8))
	trackH := gtx.Constraints.Max.Y
	thumbH := max(gtx.Dp(unit.Dp(24)), trackH/3)
	track, thumb := settingsPreviewScrollbarGeometry(trackW, trackH, thumbH)
	return fixedWidth(gtx, trackW, func(gtx layout.Context) layout.Dimensions {
		paintSettingsPreviewRoundedRect(gtx, track, palette.ScrollTrackH)
		paintSettingsPreviewRoundedRect(gtx, thumb, palette.ScrollThumbH)
		return layout.Dimensions{Size: image.Pt(trackW, trackH)}
	})
}

// settingsStatusBarFieldRow pairs a status bar field with its checkbox label.
type settingsStatusBarFieldRow struct {
	field filePaneStatusField
	label string
}

// settingsStatusBarFieldRows lists the field checkboxes in display order, which
// is the enum's own order and therefore the order the bar renders them in. A
// field missing from here has no checkbox at all, so the user can never turn it
// on; TestSettingsStatusBarFieldRowsCoverEveryField guards that.
//
// The name has no row on purpose — it is always shown (Revision 2) — and the
// old "Marked selection" row is gone with its field: the marked-mode summary is
// automatic now, not a checkbox.
func settingsStatusBarFieldRows() []settingsStatusBarFieldRow {
	return []settingsStatusBarFieldRow{
		{filePaneStatusFieldSize, "Size"},
		{filePaneStatusFieldDate, "Date"},
		{filePaneStatusFieldPerms, "Permissions"},
		{filePaneStatusFieldOwner, "Owner / group"},
		{filePaneStatusFieldFree, "Free space"},
	}
}

// settingsStatusBarDateSampleTime is the fixed instant the date-layout picker's
// option labels render, chosen so the three fixed layouts read as the design
// doc's examples: 2026-08-18 16:40. A fixed instant rather than time.Now(),
// for the same reason as settingsPanePreviewTime: the labels must not depend on
// when the settings modal happens to be open.
var settingsStatusBarDateSampleTime = time.Date(2026, time.August, 18, 16, 40, 0, 0, time.Local)

// settingsStatusBarDateFormatOptions lists the status_bar.date_format picker's
// options in the order stepStatusBarDateFormat walks them. The three fixed
// layouts are labelled with rendered samples rather than key names, and the
// samples are derived from fm.StatusBarDateLayout so a layout change in fm
// re-labels the picker by itself. "auto" has no layout of its own — it follows
// the Full-mode column date builder — so it is the one worded label.
func settingsStatusBarDateFormatOptions() []terminalShellOption {
	options := []terminalShellOption{{Key: fm.StatusBarDateFormatAuto, Label: "Match columns"}}
	for _, key := range []string{fm.StatusBarDateFormatISO, fm.StatusBarDateFormatUS, fm.StatusBarDateFormatShort} {
		options = append(options, terminalShellOption{
			Key:   key,
			Label: settingsStatusBarDateSampleTime.Format(fm.StatusBarDateLayout(key)),
		})
	}
	return options
}

func (st *settingsModalState) setStatusBarDateFormat(next string, now time.Time) bool {
	if st == nil {
		return false
	}
	next = fm.NormalizeStatusBarDateFormat(next)
	current := fm.NormalizeStatusBarDateFormat(st.statusBarDateFormat)
	if current == next {
		st.statusBarDateFormat = next
		return false
	}
	st.statusBarDateFormat = current
	st.statusBarDateFormatAnim.setValue(&st.statusBarDateFormat, next, now)
	st.statusBarDateFormatAnim.anim.setPulse(next, now)
	return true
}

func (st *settingsModalState) stepStatusBarDateFormat(step int, now time.Time) bool {
	if st == nil || step == 0 {
		return false
	}
	options := settingsStatusBarDateFormatOptions()
	keys := make([]string, len(options))
	for i, option := range options {
		keys[i] = option.Key
	}
	current := fm.NormalizeStatusBarDateFormat(st.statusBarDateFormat)
	next := settingsChoiceStep(current, keys, step)
	if next == "" || next == current {
		return false
	}
	return st.setStatusBarDateFormat(next, now)
}

func (ui *UI) layoutSettingsPaneStatusBarTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	dateFormatOptions := settingsStatusBarDateFormatOptions()
	for i, option := range dateFormatOptions {
		for st.statusBarDateFormatClicks[i].Clicked(gtx) {
			// Drained unconditionally, acted on only while the bar is on. The
			// picker's strip is laid out under gtx.Disabled() when the bar is
			// off, but this loop runs on the tab's enabled context, so the
			// events still reach the clickables — and a click merely left in
			// the queue would fire later, when the master switch is turned back
			// on, as a change the user never made.
			if !st.statusBarEnabledBool.Value {
				continue
			}
			st.setKeyboardFocus(settingsKeyboardFocusStatusBarDateFormat)
			st.setStatusBarDateFormat(option.Key, gtx.Now)
		}
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			before := st.statusBarEnabledBool.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.statusBarEnabledBool, "Show pane status bar", ui.scaleModalFontSize(10))
			if before != st.statusBarEnabledBool.Value {
				st.focus = settingsKeyboardFocusStatusBarEnabled
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusStatusBarEnabled, &st.statusBarEnabledBool)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Read live rather than snapshotted above: the master switch is the
			// preceding Rigid, and Flex runs rigids in order, so a click on it
			// greys this row in the same frame.
			if !st.statusBarEnabledBool.Value {
				gtx = gtx.Disabled()
			}
			before := st.statusBarHideInFullBool.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.statusBarHideInFullBool, "Hide it in full mode", ui.scaleModalFontSize(10))
			if before != st.statusBarHideInFullBool.Value {
				st.focus = settingsKeyboardFocusStatusBarHideInFull
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusStatusBarHideInFull, &st.statusBarHideInFullBool)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(settingsViewerRowLabel(ui, th, "Fields", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
	}

	for _, row := range settingsStatusBarFieldRows() {
		focus := settingsKeyboardFocusStatusBarField(row.field)
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// Greying here is the whole mouse half of the tab's stated
				// safety property — you cannot change which fields the bar
				// shows while the bar is off; focusOrder() is the keyboard
				// half. Only the pointer scans in
				// settings_statusbar_tab_test.go notice if this goes.
				if !st.statusBarEnabledBool.Value {
					gtx = gtx.Disabled()
				}
				box := &st.statusBarFieldBools[row.field]
				before := box.Value
				dims := ui.layoutThemeCheckbox(th, gtx, box, row.label, ui.scaleModalFontSize(10))
				if before != box.Value {
					st.focus = focus
				}
				st.applyPendingWidgetFocus(gtx, focus, box)
				return dims
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		)
	}

	children = append(children,
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Greyed with the field checkboxes above, and for the same reason:
			// the picker configures what the bar shows, so it must not be
			// editable while the bar is off. The keyboard half is focusOrder,
			// which lists the picker only while the master switch is on. The
			// label greys through its own enabled flag — settingsViewerRowLabel
			// does not read gtx.Enabled() — while the picker below greys off
			// the disabled context, the way the checkboxes do.
			return settingsViewerRowLabel(ui, th, "Date layout", st.statusBarEnabledBool.Value)(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !st.statusBarEnabledBool.Value {
				gtx = gtx.Disabled()
			}
			return fixedWidth(gtx, gtx.Dp(unit.Dp(430)), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsShellPicker(th, gtx, dateFormatOptions, st.statusBarDateFormatClicks[:],
					fm.NormalizeStatusBarDateFormat(st.statusBarDateFormat), &st.statusBarDateFormatAnim,
					st.focus == settingsKeyboardFocusStatusBarDateFormat)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsStatusBarPreview(th, gtx, st)
		}),
	)

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// layoutSettingsStatusBarPreview renders the pane mock the Status bar tab
// previews: a listing of sample rows with ONE status bar strip pinned along the
// bottom edge, which is exactly the shape a real pane has. The frame, the
// header and the corner caption are the same
// layoutSettingsPanePreviewFrame the Full and Brief previews sit in, so the
// three tabs show one family of pictures rather than three kinds of diagram.
//
// The strip is laid out through the live bar's own path —
// buildFilePaneStatusBarPlan, the measured column widths and
// layoutFilePaneStatusBarInfoRow's anchored flex row — so it cannot drift from
// what ships. Only the cursor mode is previewed: the marked-mode summary
// (filePaneStatusMarkedSummary) is automatic and unconfigurable, so a second
// strip for it would show the user nothing they can change.
// The master switch drops the strip and nothing else:
// filePaneStatusBarVisible hides the bar, not the pane, so the mock keeps its
// rows and the tab reads as a straight before/after of the one thing the switch
// controls. A preview that kept drawing a strip with the bar off would be
// advertising something that cannot ship; one that emptied the whole frame
// would look like a picture that failed to draw.
//
// Deliberately asymmetric with HideInFull, the other half of
// filePaneStatusBarVisible: that one hangs on the pane's view mode. The mock is
// a brief pane (see layoutSettingsStatusBarPreviewGrid), the mode HideInFull
// never applies to, so there is nothing here for the preview to reflect.
func (ui *UI) layoutSettingsStatusBarPreview(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	return ui.layoutSettingsPanePreviewFrame(th, gtx, "STATUS BAR PREVIEW", st, func(gtx layout.Context) layout.Dimensions {
		palette := ui.settingsPaneDraftPalette(st)
		return fillBgExact(gtx, palette.PaneBg, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsStatusBarPreviewPane(th, gtx, st, palette, st.statusBarEnabledBool.Value)
		})
	})
}

// settingsStatusBarPreviewConfig is the draft config the preview pane renders
// under — the same draft shape as layoutSettingsFullPaneRows, because the
// preview must follow the edits in progress, not the config last written to
// disk. Permission format, the column date builder and the bar's own date
// layout are the three status bar inputs the config carries; the field list
// comes from the checkboxes and is passed separately. The first two are read
// from the same state the Full mode tab edits and in the same way that tab's
// own preview reads them, so the two previews can never disagree about the
// sample.
func (ui *UI) settingsStatusBarPreviewConfig(st *settingsModalState) fm.Config {
	draft := *ui.fmCfg
	draft.Columns.PermissionFormat = settingsNormalizePermissionFormat(st.panePermissionFormat)
	draft.DateFormats = st.paneDateFormats()
	draft.StatusBar.DateFormat = fm.NormalizeStatusBarDateFormat(st.statusBarDateFormat)
	return draft
}

// The sample volume the free-space field reports on. Package constants rather
// than locals because the preview test rebuilds the field's text through
// formatFilePaneStatusFree.
const (
	settingsStatusBarPreviewFreeBytes  = uint64(41) << 30
	settingsStatusBarPreviewTotalBytes = uint64(100) << 30
)

// The mock's grid shape. Four rows of three columns is the largest listing that
// still leaves the strip room inside the 154dp frame at scale 1, and it fills
// the frame the way the Brief preview's grid does. len must equal
// settingsStatusBarPreviewEntries; TestSettingsStatusBarPreviewGridFitsTheSample
// guards that.
const (
	settingsStatusBarPreviewGridColumns = 3
	settingsStatusBarPreviewGridRows    = 4
)

// settingsStatusBarPreviewCursor is the entry the mock highlights AND the entry
// the strip describes — the whole point of the rework. Index 3 is the bottom of
// the leftmost column, directly above the strip's left-anchored name, so the
// correspondence is the shortest possible eye movement.
const settingsStatusBarPreviewCursor = 3

// settingsStatusBarPreviewEntries is the single source of truth for the Status
// bar preview: settingsStatusBarPreviewPane feeds it to the strip and
// layoutSettingsStatusBarPreviewGrid draws the same slice as the mock's rows,
// so the two cannot disagree about the sample the way the old preview did (its
// strip said report.pdf over a frame that showed no files at all).
//
// filesys.Entry rather than the settingsPanePreviewRow the sibling previews
// use, because the strip needs SizeBytes, ModTime, PermText, PermOctal and
// OwnerText — five attributes the shared row type deliberately does not carry
// and that the Full and Brief samples would never read. The grid only needs the
// name and the kind, and takes them from the entry.
//
// The cursor entry carries a ModTime and no DateText on purpose. The Date field
// reads the entry through filePaneModel.formatDate, which applies
// cfg.DateFormats and only falls back to DateText for an entry with no
// modification time at all, so a literal DateText here would pin the preview to
// a string the shipping bar cannot produce. settingsPanePreviewTime is the same
// fixed instant the Full mode preview samples, so the two show one date, and
// neither depends on when the settings modal happens to be open.
//
// Names are kept short for a reason beyond taste: the widest display name in
// the listing IS the bar's name column (computeFilePaneStatusColumnWidths), and
// that column is the largest single term in the width the mock has to be laid
// out at — see settingsStatusBarPreviewPaneWidth.
var settingsStatusBarPreviewEntries = []filesys.Entry{
	{Name: "..", Kind: filesys.EntryParent},
	{Name: "Projects", Kind: filesys.EntryDir, ModTime: settingsPanePreviewTime, PermText: "drwxr-xr-x", PermOctal: "0755", OwnerText: "demo:staff"},
	{Name: "Reports", Kind: filesys.EntryDir, ModTime: settingsPanePreviewTime, PermText: "drwxr-xr-x", PermOctal: "0755", OwnerText: "demo:staff"},
	{Name: "photos.rar", Kind: filesys.EntryFile, SizeBytes: 2516582, ModTime: settingsPanePreviewTime, PermText: "-rw-r--r--", PermOctal: "0644", OwnerText: "demo:staff"},
	{Name: "notes.md", Kind: filesys.EntryFile, SizeBytes: 4096, ModTime: settingsPanePreviewTime, PermText: "-rw-r--r--", PermOctal: "0644", OwnerText: "demo:staff"},
	{Name: "main.go", Kind: filesys.EntryFile, SizeBytes: 18342, ModTime: settingsPanePreviewTime, PermText: "-rw-r--r--", PermOctal: "0644", OwnerText: "demo:staff"},
	{Name: "todo.md", Kind: filesys.EntryFile, SizeBytes: 1204, ModTime: settingsPanePreviewTime, PermText: "-rw-r--r--", PermOctal: "0644", OwnerText: "demo:staff"},
	{Name: "data.csv", Kind: filesys.EntryFile, SizeBytes: 92160, ModTime: settingsPanePreviewTime, PermText: "-rw-r--r--", PermOctal: "0644", OwnerText: "demo:staff"},
	{Name: "backup.zip", Kind: filesys.EntryFile, SizeBytes: 41943040, ModTime: settingsPanePreviewTime, PermText: "-rw-r--r--", PermOctal: "0644", OwnerText: "demo:staff"},
	{Name: "server.log", Kind: filesys.EntryFile, SizeBytes: 786432, ModTime: settingsPanePreviewTime, PermText: "-rw-r--r--", PermOctal: "0644", OwnerText: "demo:staff"},
	{Name: "hexone.exe", Kind: filesys.EntryFile, SizeBytes: 40061747, ModTime: settingsPanePreviewTime, PermText: "-rwxr-xr-x", PermOctal: "0755", OwnerText: "demo:staff"},
	{Name: "music.flac", Kind: filesys.EntryFile, SizeBytes: 31457280, ModTime: settingsPanePreviewTime, PermText: "-rw-r--r--", PermOctal: "0644", OwnerText: "demo:staff"},
}

// settingsStatusBarPreviewPane builds the sample pane the strip renders from.
// It shares settingsStatusBarPreviewEntries with the mock's grid and puts the
// cursor on settingsStatusBarPreviewCursor, the row the grid highlights.
//
// No marked rows: the preview shows the cursor mode only (see
// layoutSettingsStatusBarPreview).
func settingsStatusBarPreviewPane(cfg *fm.Config) *filePaneState {
	pane := &filePaneState{}
	pane.model = &filePaneModel{cfg: cfg, entries: settingsStatusBarPreviewEntries}
	pane.table = table.New(nil)
	pane.table.Selected = settingsStatusBarPreviewCursor
	return pane
}

// layoutSettingsStatusBarPreviewPane draws the mock: the sample listing on top,
// a brief-mode scrollbar, and — when showStrip — the status bar pinned along
// the bottom edge. With the master switch off the rows and the scrollbar stay
// and only the strip goes, which is exactly what filePaneStatusBarVisible does
// to a real pane.
//
// The mock is laid out at settingsStatusBarPreviewPaneWidth and scaled into the
// frame when the frame is narrower — see that function for the measurements
// behind it. The scale covers the WHOLE mock rather than the strip alone: a
// strip drawn at a different scale from the rows an inch above it would misstate
// the one proportion the picture exists to show. Only the strip needs the extra
// width, so with the strip gone the mock is drawn at the frame's own size.
func (ui *UI) layoutSettingsStatusBarPreviewPane(th *material.Theme, gtx layout.Context, st *settingsModalState, palette filePanePalette, showStrip bool) layout.Dimensions {
	frame := gtx.Constraints.Max
	if frame.X < 1 || frame.Y < 1 {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	draft := ui.settingsStatusBarPreviewConfig(st)
	fields := filePaneStatusFields(fm.NormalizeStatusBarFields(st.statusBarSelectedFields()))
	freeLabel := ""
	if slices.Contains(fields, filePaneStatusFieldFree) {
		// The volume reading is a fixed sample; the live path's async lookup has
		// nothing to look up here.
		freeLabel = formatFilePaneStatusFree(settingsStatusBarPreviewFreeBytes, settingsStatusBarPreviewTotalBytes)
	}
	pane := settingsStatusBarPreviewPane(&draft)

	content := func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.N.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutSettingsStatusBarPreviewGrid(th, gtx, st, palette, pane.table.Selected)
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsPanePreviewHScrollbar(gtx, palette)
			}),
		}
		if showStrip {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsStatusBarPreviewStrip(th, gtx, pane, fields, freeLabel, palette)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}

	layoutW := frame.X
	if showStrip {
		layoutW = max(frame.X, ui.settingsStatusBarPreviewPaneWidth(th, gtx, pane, fields, freeLabel))
	}
	if layoutW == frame.X {
		return content(gtx)
	}
	scale := float32(frame.X) / float32(layoutW)
	// Ceil, so the scaled mock covers the frame's last row rather than leaving a
	// sliver of bare pane background under the strip; the clip below swallows the
	// sub-pixel overshoot.
	layoutH := (frame.Y*layoutW + frame.X - 1) / frame.X
	wide := gtx
	wide.Constraints = layout.Constraints{Max: image.Pt(layoutW, layoutH)}
	m := op.Record(gtx.Ops)
	content(wide)
	call := m.Stop()
	defer clip.Rect(image.Rect(0, 0, frame.X, frame.Y)).Push(gtx.Ops).Pop()
	defer op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(scale, scale))).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: frame}
}

// settingsStatusBarPreviewPaneWidth is the pane width the mock is laid out at:
// the narrowest width at which the configured bar does not degrade — every
// ticked field keeps its column and the cursor name renders in full. It is
// measured, not a constant, so it follows the draft configuration, the display
// metrics and the font scale.
//
// Why the mock is scaled at all. The preview frame is the settings modal's
// content column, a fixed 558dp, which leaves the strip 542dp between its
// insets. Measured at PxPerDp 1 with the default configuration, the strip's
// terms are: name 80 (the widest sample name), size 72, date 136, permissions
// 80, owner 80, free space 152, four column separators at 40 each and the
// region separator at 40 — 816dp for the full field set, and 832dp with the US
// date layout, against 558dp of frame. Nothing closes a 258dp gap: shorter
// sample values buy about 48dp all told, the name column's floor is only 32dp
// below its natural width here, and growing the frame means widening the whole
// settings modal.
//
// Letting the strip degrade at the frame width instead is the honest-looking
// option and it is the one that actually lies. With the default size/date/free
// set the row needs 576dp and fits in 542 only by compacting the name; ticking
// Owner then needs 696 and buildFilePaneStatusBarPlan drops Owner again, so the
// checkbox changes nothing at all. Permissions behaves the same way. A dead
// checkbox is the exact failure this preview exists to prevent, so the mock is
// drawn as a miniature of a pane wide enough to show what was ticked — and at
// the default field set that width is within 4% of the frame's own, so the
// common case is barely a miniature at all.
func (ui *UI) settingsStatusBarPreviewPaneWidth(th *material.Theme, gtx layout.Context, pane *filePaneState, fields []filePaneStatusField, freeLabel string) int {
	measure := func(text string) int { return ui.measureFilePaneStatusBarTextWidth(th, gtx, text) }
	widths := computeFilePaneStatusColumnWidths(pane, measure)
	// The name column at its full measured width, not the compaction floor: the
	// strip's name has to read as the same string as the highlighted row above
	// it, and "ph….rar" over "photos.rar" is not that.
	total := widths.namePx
	for _, field := range fields {
		if field == filePaneStatusFieldFree {
			continue
		}
		total += widths.sepPx + filePaneStatusFieldColumnPx(widths, field)
	}
	if freeLabel != "" {
		total += widths.regionSepPx + measure(freeLabel)
	}
	return total + 2*gtx.Dp(filePaneStatusBarInsetX)
}

// layoutSettingsStatusBarPreviewGrid draws the sample listing as a brief-mode
// grid — names only, filled column by column, exactly like
// layoutSettingsBriefPanePreview.
//
// Brief rather than full-mode rows for two reasons. Brief is the mode the bar
// was built for (see filePaneStatusDropOrder: size and date survive longest
// "because reading them in brief mode is the reason this bar exists"), and a
// full-mode grid would print the same size, date and permissions the strip
// below is showing, making the bar look redundant in its own preview.
func (ui *UI) layoutSettingsStatusBarPreviewGrid(th *material.Theme, gtx layout.Context, st *settingsModalState, palette filePanePalette, cursor int) layout.Dimensions {
	colW := max(1, gtx.Constraints.Max.X/settingsStatusBarPreviewGridColumns)
	columns := make([]layout.FlexChild, 0, settingsStatusBarPreviewGridColumns)
	for col := 0; col < settingsStatusBarPreviewGridColumns; col++ {
		col := col
		columns = append(columns, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, colW, func(gtx layout.Context) layout.Dimensions {
				rows := make([]layout.FlexChild, 0, settingsStatusBarPreviewGridRows)
				for row := 0; row < settingsStatusBarPreviewGridRows; row++ {
					idx := col*settingsStatusBarPreviewGridRows + row
					rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.layoutSettingsStatusBarPreviewCell(th, gtx, st, palette, idx, idx == cursor)
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
			})
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, columns...)
}

// layoutSettingsStatusBarPreviewCell draws one grid cell. The cursor cell takes
// the draft palette's selected colours, the same pair
// layoutSettingsBriefPanePreview highlights its own sample row with.
//
// It also tags itself semantic.SelectedOp(true), which is both accurate for a
// screen reader and the hook
// TestSettingsStatusBarPreviewStripDescribesTheHighlightedRow uses to find the
// highlighted name without reimplementing the grid's arithmetic. Without a
// marker in the tree, nothing could catch a mock that highlights one row while
// its strip describes another — the drift this rework exists to remove.
func (ui *UI) layoutSettingsStatusBarPreviewCell(th *material.Theme, gtx layout.Context, st *settingsModalState, palette filePanePalette, idx int, selected bool) layout.Dimensions {
	entry := settingsStatusBarPreviewEntries[idx]
	row := settingsPanePreviewRow{name: entry.Name, kind: entry.Kind}
	bg, fg := palette.PaneBg, palette.PaneFg
	if selected {
		bg = palette.SelectedBg
		fg = settingsEffectivePaneRowTextColor(palette, palette.SelectedFg)
	}
	height := gtx.Dp(unit.Dp(18))
	return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
		return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			width := max(1, gtx.Constraints.Max.X)
			if selected {
				defer clip.Rect(image.Rect(0, 0, width, height)).Push(gtx.Ops).Pop()
				semantic.SelectedOp(true).Add(gtx.Ops)
			}
			return ui.layoutSettingsPanePreviewNameCell(th, gtx, st, row, width, fg)
		})
	})
}

// layoutSettingsPanePreviewHScrollbar is the brief preview's horizontal
// scrollbar, shared so the status bar mock reads as the same kind of pane.
func (ui *UI) layoutSettingsPanePreviewHScrollbar(gtx layout.Context, palette filePanePalette) layout.Dimensions {
	h := gtx.Dp(unit.Dp(8))
	return fixedHeight(gtx, h, func(gtx layout.Context) layout.Dimensions {
		track, thumb := settingsPreviewScrollbarGeometry(gtx.Constraints.Max.X, h, max(gtx.Dp(unit.Dp(64)), gtx.Constraints.Max.X/3))
		paintSettingsPreviewRoundedRect(gtx, track, palette.ScrollTrackH)
		paintSettingsPreviewRoundedRect(gtx, thumb, palette.ScrollThumbH)
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
	})
}

// layoutSettingsStatusBarPreviewStrip lays the mock's strip out through the
// live bar's exact rendering — layoutFilePaneStatusBarBox, the 8dp/4dp insets
// and layoutFilePaneStatusBarInfoRow — at whatever width its caller hands it.
// It does no scaling of its own; layoutSettingsStatusBarPreviewPane scales the
// whole mock, strip and rows together.
func (ui *UI) layoutSettingsStatusBarPreviewStrip(
	th *material.Theme,
	gtx layout.Context,
	pane *filePaneState,
	fields []filePaneStatusField,
	freeLabel string,
	palette filePanePalette,
) layout.Dimensions {
	bg, border, textColor := filePaneVolumeBadgeColors(palette)
	return layoutFilePaneStatusBarBox(gtx, bg, border, func(gtx layout.Context) layout.Dimensions {
		inset := layout.Inset{
			Left:   filePaneStatusBarInsetX,
			Right:  filePaneStatusBarInsetX,
			Top:    unit.Dp(4),
			Bottom: unit.Dp(4),
		}
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePaneStatusBarInfoRow(th, gtx, pane, fields, freeLabel, textColor)
		})
	})
}
