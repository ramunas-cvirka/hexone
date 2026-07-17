// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"image/color"
	"strings"
	"time"
	"unicode/utf8"

	"hexone/filesys"
	"hexone/fm"
	uitheme "hexone/ui/theme"
	"hexone/ui/widget/table"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
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
	case "other":
		return "other"
	default:
		return "full"
	}
}

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
	next := settingsChoiceStep(current, []string{"full", "brief", "other"}, step)
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
	pos, posAnim := st.paneSettingsModeAnim.position(gtx.Now, mode, []string{"full", "brief", "other"})
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

func (ui *UI) layoutSettingsPanePreviewFrame(th *material.Theme, gtx layout.Context, title string, st *settingsModalState, content layout.Widget) layout.Dimensions {
	palette := ui.settingsPaneDraftPalette(st)
	height := gtx.Dp(unit.Dp(154))
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
