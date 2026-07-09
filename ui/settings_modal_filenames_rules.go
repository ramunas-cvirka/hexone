// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"hexone/fm"
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type filenameRuleChoiceOption struct {
	key   string
	label string
}

var filenamePermissionMatchOptions = []filenameRuleChoiceOption{
	{key: fm.FilenamePermissionMatchExact, label: "Exact"},
	{key: fm.FilenamePermissionMatchAll, label: "Has All"},
	{key: fm.FilenamePermissionMatchAny, label: "Has Any"},
	{key: fm.FilenamePermissionMatchNone, label: "Has None"},
}

var filenameSizeMatchOptions = []filenameRuleChoiceOption{
	{key: fm.FilenameSizeMatchAtLeast, label: "At Least"},
	{key: fm.FilenameSizeMatchAtMost, label: "At Most"},
}

func normalizeFilenameSizeMatch(raw string) string {
	return fm.NormalizeFilenameSizeMatch(raw)
}

func formatFilenamePermissionRuleLabel(rule fm.FilenamePermissionRule) string {
	if normalizeFilenamePermissionMatch(rule.Match) == fm.FilenamePermissionMatchExact {
		return rule.Permissions
	}
	return filenamePermissionMatchLabel(rule.Match) + " " + rule.Permissions
}

func filenameExtensionRuleKey(raw, targetRaw string) string {
	ext := fm.NormalizeFilenameExtension(raw)
	if ext == "" {
		return ""
	}
	return ext
}

func filenameExtensionDisplayText(raw string) string {
	txt := strings.TrimSpace(raw)
	if ext := fm.NormalizeFilenameExtension(txt); ext != "" {
		txt = ext
	}
	return strings.TrimPrefix(txt, ".")
}

func formatFilenameExtensionRuleLabel(rule fm.FilenameExtensionRule) string {
	return filenameExtensionDisplayText(rule.Extension)
}

func filenameSizeRuleKey(sizeRaw, unitRaw, matchRaw string) string {
	size, ok := fm.NormalizeFilenameSize(filenameSizeValueFromFields(sizeRaw, unitRaw))
	if !ok {
		return ""
	}
	return normalizeFilenameSizeMatch(matchRaw) + ":" + size
}

func formatFilenameSizeRuleLabel(rule fm.FilenameSizeRule) string {
	size, ok := fm.NormalizeFilenameSize(rule.Size)
	if !ok {
		size = strings.TrimSpace(rule.Size)
	}
	if normalizeFilenameSizeMatch(rule.Match) == fm.FilenameSizeMatchAtMost {
		return "<= " + size
	}
	return ">= " + size
}

func settingsFilenamePreviewSampleSize(limit int64, match string) int64 {
	if normalizeFilenameSizeMatch(match) == fm.FilenameSizeMatchAtMost {
		if limit <= 1 {
			return 0
		}
		sample := limit / 2
		if sample >= limit {
			sample = limit - 1
		}
		if sample < 0 {
			sample = 0
		}
		return sample
	}
	if limit <= 0 {
		return 1
	}
	sample := limit + limit/2
	if sample <= limit {
		sample = limit + 1
	}
	return sample
}

func (ui *UI) layoutSettingsFilenameTargetTabs(th *material.Theme, gtx layout.Context, st *settingsModalState, current *string, anim *settingsChoiceAnim, clicks *[3]widget.Clickable, focus settingsKeyboardFocus) layout.Dimensions {
	if ui == nil || st == nil || current == nil || anim == nil || clicks == nil {
		return layout.Dimensions{}
	}
	keys := make([]string, len(filenameTargetOptions))
	hoverKey := ""
	for i, opt := range filenameTargetOptions {
		keys[i] = opt.key
		if clicks[i].Clicked(gtx) {
			st.setKeyboardFocus(focus)
			anim.anim.setPulse(opt.key, gtx.Now)
			anim.setValue(current, opt.key, gtx.Now)
			st.errText = ""
			gtx.Execute(op.InvalidateCmd{})
		}
		if clicks[i].Hovered() {
			hoverKey = opt.key
		}
	}
	active := normalizeFilenameTargetChoice(*current)
	*current = active
	anim.anim.setHover(hoverKey, gtx.Now)
	pos, animPos := anim.position(gtx.Now, active, keys)
	specs := make([]slidingTabSpec, 0, len(filenameTargetOptions))
	animating := animPos
	for i, opt := range filenameTargetOptions {
		activeFill, activeAnim := anim.fill(gtx.Now, active, opt.key)
		hoverFill, hoverAnim := anim.anim.hoverFill(gtx.Now, opt.key)
		pulseFill, pulseAnim := anim.anim.pulseFill(gtx.Now, opt.key)
		focusFill := float32(0)
		if st.focus == focus && active == opt.key {
			focusFill = 1
		}
		specs = append(specs, slidingTabSpec{
			Label:      opt.label,
			Click:      &clicks[i],
			ActiveFill: activeFill,
			HoverFill:  hoverFill,
			PulseFill:  pulseFill,
			FocusFill:  focusFill,
		})
		animating = animating || activeAnim || hoverAnim || pulseAnim
	}
	if animating {
		gtx.Execute(op.InvalidateCmd{})
	}
	stripH := gtx.Dp(unit.Dp(24))
	if stripH < 1 {
		stripH = 1
	}
	return ui.layoutSlidingTabStrip(th, gtx, stripH, pos, ui.scaleModalFontSize(10), specs)
}

func (ui *UI) layoutSettingsFilenamePermissionMatchTabs(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	if ui == nil || st == nil {
		return layout.Dimensions{}
	}
	keys := make([]string, len(filenamePermissionMatchOptions))
	hoverKey := ""
	for i, opt := range filenamePermissionMatchOptions {
		keys[i] = opt.key
		if st.filenamePermMatchClicks[i].Clicked(gtx) {
			st.setKeyboardFocus(settingsKeyboardFocusFilenamePermMatch)
			st.filenamePermMatchAnim.anim.setPulse(opt.key, gtx.Now)
			st.filenamePermMatchAnim.setValue(&st.filenamePermMatch, opt.key, gtx.Now)
			st.errText = ""
			gtx.Execute(op.InvalidateCmd{})
		}
		if st.filenamePermMatchClicks[i].Hovered() {
			hoverKey = opt.key
		}
	}
	active := normalizeFilenamePermissionMatch(st.filenamePermMatch)
	st.filenamePermMatch = active
	st.filenamePermMatchAnim.anim.setHover(hoverKey, gtx.Now)
	pos, animPos := st.filenamePermMatchAnim.position(gtx.Now, active, keys)
	specs := make([]slidingTabSpec, 0, len(filenamePermissionMatchOptions))
	animating := animPos
	for i, opt := range filenamePermissionMatchOptions {
		activeFill, activeAnim := st.filenamePermMatchAnim.fill(gtx.Now, active, opt.key)
		hoverFill, hoverAnim := st.filenamePermMatchAnim.anim.hoverFill(gtx.Now, opt.key)
		pulseFill, pulseAnim := st.filenamePermMatchAnim.anim.pulseFill(gtx.Now, opt.key)
		focusFill := float32(0)
		if st.focus == settingsKeyboardFocusFilenamePermMatch && active == opt.key {
			focusFill = 1
		}
		specs = append(specs, slidingTabSpec{
			Label:      opt.label,
			Click:      &st.filenamePermMatchClicks[i],
			ActiveFill: activeFill,
			HoverFill:  hoverFill,
			PulseFill:  pulseFill,
			FocusFill:  focusFill,
		})
		animating = animating || activeAnim || hoverAnim || pulseAnim
	}
	if animating {
		gtx.Execute(op.InvalidateCmd{})
	}
	stripH := gtx.Dp(unit.Dp(24))
	if stripH < 1 {
		stripH = 1
	}
	return ui.layoutSlidingTabStrip(th, gtx, stripH, pos, ui.scaleModalFontSize(10), specs)
}

func settingsFilenamePermissionPickerLabel(open bool) string {
	if open {
		return "Bits  ▴"
	}
	return "Bits  ▾"
}

func (ui *UI) layoutSettingsFilenamePermissionPickerField(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	if ui == nil || st == nil {
		return layout.Dimensions{}
	}
	dims := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(82)), func(gtx layout.Context) layout.Dimensions {
				ed := material.Editor(th, &st.filenamePermEdit, "0755")
				ed.Font.Typeface = ui.interfaceTypeface()
				ed.TextSize = ui.scaleModalFontSize(10)
				ed.Color = txtColor
				ed.HintColor = hintColor
				dims := ui.layoutEditorWithContextMenu(th, gtx, "settings-filename-perm", &st.filenamePermEdit, true, func(gtx layout.Context) layout.Dimensions {
					return layoutNeutralEditorBox(gtx, gtx.Focused(&st.filenamePermEdit), true, ed.Layout)
				})
				st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusFilenamePermMask, &st.filenamePermEdit)
				return dims
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutTinyModeButtonState(th, gtx, ui.interfaceTypeface(), &st.filenamePermPickerClick, settingsFilenamePermissionPickerLabel(st.filenamePermPickerOpen), st.filenamePermPickerOpen, st.focus == settingsKeyboardFocusFilenamePermPicker)
		}),
	)
	if st.filenamePermPickerOpen {
		m := op.Record(gtx.Ops)
		offset := op.Offset(image.Pt(0, dims.Size.Y+gtx.Dp(unit.Dp(4))))
		offset.Add(gtx.Ops)
		ui.layoutSettingsFilenamePermissionPickerPopup(th, gtx, st)
		op.Defer(gtx.Ops, m.Stop())
	}
	return dims
}

func (ui *UI) layoutSettingsFilenamePermissionPickerPopup(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	if ui == nil || st == nil {
		return layout.Dimensions{}
	}
	dims := ui.layoutSettingsFilenamePermissionMaskEditor(th, gtx, st)
	registerSettingsPopupArea(gtx, &st.filenamePermPickerPopupTag, dims.Size)
	return dims
}

func (ui *UI) layoutSettingsFilenameSizeMatchTabs(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	if ui == nil || st == nil {
		return layout.Dimensions{}
	}
	keys := make([]string, len(filenameSizeMatchOptions))
	hoverKey := ""
	for i, opt := range filenameSizeMatchOptions {
		keys[i] = opt.key
		if st.filenameSizeMatchClicks[i].Clicked(gtx) {
			st.setKeyboardFocus(settingsKeyboardFocusFilenameSizeMatch)
			st.filenameSizeMatchAnim.anim.setPulse(opt.key, gtx.Now)
			st.filenameSizeMatchAnim.setValue(&st.filenameSizeMatch, opt.key, gtx.Now)
			st.errText = ""
			gtx.Execute(op.InvalidateCmd{})
		}
		if st.filenameSizeMatchClicks[i].Hovered() {
			hoverKey = opt.key
		}
	}
	active := normalizeFilenameSizeMatch(st.filenameSizeMatch)
	st.filenameSizeMatch = active
	st.filenameSizeMatchAnim.anim.setHover(hoverKey, gtx.Now)
	pos, animPos := st.filenameSizeMatchAnim.position(gtx.Now, active, keys)
	specs := make([]slidingTabSpec, 0, len(filenameSizeMatchOptions))
	animating := animPos
	for i, opt := range filenameSizeMatchOptions {
		activeFill, activeAnim := st.filenameSizeMatchAnim.fill(gtx.Now, active, opt.key)
		hoverFill, hoverAnim := st.filenameSizeMatchAnim.anim.hoverFill(gtx.Now, opt.key)
		pulseFill, pulseAnim := st.filenameSizeMatchAnim.anim.pulseFill(gtx.Now, opt.key)
		focusFill := float32(0)
		if st.focus == settingsKeyboardFocusFilenameSizeMatch && active == opt.key {
			focusFill = 1
		}
		specs = append(specs, slidingTabSpec{
			Label:      opt.label,
			Click:      &st.filenameSizeMatchClicks[i],
			ActiveFill: activeFill,
			HoverFill:  hoverFill,
			PulseFill:  pulseFill,
			FocusFill:  focusFill,
		})
		animating = animating || activeAnim || hoverAnim || pulseAnim
	}
	if animating {
		gtx.Execute(op.InvalidateCmd{})
	}
	stripH := gtx.Dp(unit.Dp(24))
	if stripH < 1 {
		stripH = 1
	}
	return ui.layoutSlidingTabStrip(th, gtx, stripH, pos, ui.scaleModalFontSize(10), specs)
}

func (ui *UI) layoutSettingsFilenamePermissionMaskEditor(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	if ui == nil || st == nil {
		return layout.Dimensions{}
	}
	const (
		rowLabelDp = unit.Dp(52)
		colWidthDp = unit.Dp(42)
		rowGapDp   = unit.Dp(3)
	)
	rowLabel := func(title string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, title)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleModalFontSize(9)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}
	}
	colLabel := func(title string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, title)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleModalFontSize(9)
			lbl.Font.Weight = font.Medium
			lbl.Color = color.NRGBA{R: 196, G: 196, B: 196, A: 255}
			return layout.Center.Layout(gtx, lbl.Layout)
		}
	}
	check := func(idx int) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutThemeCheckbox(th, gtx, &st.filenamePermChecks[idx], "", ui.scaleModalFontSize(9))
			})
		}
	}
	cell := func(w unit.Dp, inner layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(w), inner)
		}
	}
	row := func(title string, first int) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(cell(rowLabelDp, rowLabel(title))),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(cell(colWidthDp, check(first))),
				layout.Rigid(cell(colWidthDp, check(first+1))),
				layout.Rigid(cell(colWidthDp, check(first+2))),
			)
		}
	}
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
		color.NRGBA{R: 20, G: 24, B: 32, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(cell(rowLabelDp, rowLabel(""))),
							layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
							layout.Rigid(cell(colWidthDp, colLabel("Read"))),
							layout.Rigid(cell(colWidthDp, colLabel("Write"))),
							layout.Rigid(cell(colWidthDp, colLabel("Exec"))),
						)
					}),
					layout.Rigid(layout.Spacer{Height: rowGapDp}.Layout),
					layout.Rigid(row("Owner", 0)),
					layout.Rigid(layout.Spacer{Height: rowGapDp}.Layout),
					layout.Rigid(row("Group", 3)),
					layout.Rigid(layout.Spacer{Height: rowGapDp}.Layout),
					layout.Rigid(row("Other", 6)),
				)
			})
		},
	)
}

func (st *settingsModalState) filenameExtensionRuleRowClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.filenameExtRowClicks == nil {
		st.filenameExtRowClicks = make(map[string]*widget.Clickable)
	}
	if click := st.filenameExtRowClicks[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.filenameExtRowClicks[key] = click
	return click
}

func (st *settingsModalState) filenameExtensionRuleRowRemoveClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.filenameExtRowRemove == nil {
		st.filenameExtRowRemove = make(map[string]*widget.Clickable)
	}
	if click := st.filenameExtRowRemove[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.filenameExtRowRemove[key] = click
	return click
}

func (st *settingsModalState) filenameExtensionRuleIndex(key string) int {
	if st == nil || key == "" {
		return -1
	}
	for i, rule := range st.filenameExtEntries {
		if filenameExtensionRuleKey(rule.Extension, rule.Target) == key {
			return i
		}
	}
	return -1
}

func (st *settingsModalState) filenameExtensionRule(key string) (fm.FilenameExtensionRule, bool) {
	if idx := st.filenameExtensionRuleIndex(key); idx >= 0 {
		return st.filenameExtEntries[idx], true
	}
	return fm.FilenameExtensionRule{}, false
}

func (st *settingsModalState) filenameSavedExtensionRule(key string) (fm.FilenameExtensionRule, bool) {
	if st == nil || key == "" {
		return fm.FilenameExtensionRule{}, false
	}
	for _, rule := range st.filenameExtSavedEntries {
		if filenameExtensionRuleKey(rule.Extension, rule.Target) == key {
			return rule, true
		}
	}
	return fm.FilenameExtensionRule{}, false
}

func (st *settingsModalState) loadFilenameExtensionFields(ext, textHex, icon, target string) {
	if st == nil {
		return
	}
	ext = fm.NormalizeFilenameExtension(ext)
	st.filenameExtEdit.SetText(filenameExtensionDisplayText(ext))
	st.filenameExtTextEdit.SetText(textHex)
	st.filenameExtIcon = fm.NormalizeFilenameIcon(icon)
	st.filenameExtLookup = filenameExtensionRuleKey(ext, "")
	st.filenameExtEditingKey = ""
	if st.filenameExtLookup != "" && (fm.NormalizeOptionalHexColor(textHex) != "" || st.filenameExtIcon != "") {
		st.filenameExtEditingKey = st.filenameExtLookup
	}
}

func (st *settingsModalState) syncFilenameExtensionEditors() {
	if st == nil {
		return
	}
	key := filenameExtensionRuleKey(st.filenameExtEdit.Text(), "")
	if key == st.filenameExtLookup {
		return
	}
	st.filenameExtLookup = key
	if st.filenameExtEditingKey != "" {
		return
	}
	if rule, ok := st.filenameExtensionRule(key); ok {
		st.loadFilenameExtensionFields(rule.Extension, rule.Text, rule.Icon, rule.Target)
		return
	}
	st.filenameExtTextEdit.SetText("")
	st.filenameExtIcon = ""
}

func (st *settingsModalState) refreshFilenameExtensionDraftInfo() {
	if st == nil {
		return
	}
	st.filenameExtInfoText = ""
	key := filenameExtensionRuleKey(st.filenameExtEdit.Text(), "")
	if key == "" {
		return
	}
	textHex := fm.NormalizeOptionalHexColor(strings.TrimSpace(st.filenameExtTextEdit.Text()))
	icon := fm.NormalizeFilenameIcon(st.filenameExtIcon)
	if textHex == "" && icon == "" {
		st.filenameExtInfoText = "Pick a color, icon, or both"
		return
	}
	existingKey := key
	if st.filenameExtEditingKey != "" {
		existingKey = st.filenameExtEditingKey
	}
	existing, ok := st.filenameExtensionRule(existingKey)
	if !ok {
		st.filenameExtInfoText = "Click Add"
		return
	}
	if existingKey == key && existing.Text == textHex && existing.Icon == icon {
		return
	}
	st.filenameExtInfoText = "Click Update"
}

func (st *settingsModalState) filenameExtensionNoticeText() string {
	if st == nil {
		return ""
	}
	key := filenameExtensionRuleKey(st.filenameExtEdit.Text(), "")
	if key == "" {
		return ""
	}
	textHex := fm.NormalizeOptionalHexColor(strings.TrimSpace(st.filenameExtTextEdit.Text()))
	icon := fm.NormalizeFilenameIcon(st.filenameExtIcon)
	if st.filenameExtEditingKey != "" {
		if editingRule, ok := st.filenameExtensionRule(st.filenameExtEditingKey); ok &&
			(st.filenameExtEditingKey != key || editingRule.Text != textHex || editingRule.Icon != icon) {
			return "Click Update"
		}
	}
	savedRule, savedExists := st.filenameSavedExtensionRule(key)
	currentRule, currentExists := st.filenameExtensionRule(key)
	switch {
	case savedExists && !currentExists:
		return "Pending removal; Save to persist"
	case !currentExists && (textHex != "" || icon != ""):
		return "Click Add"
	case currentExists && (currentRule.Text != textHex || currentRule.Icon != icon):
		return "Click Update"
	case savedExists && currentExists && (savedRule.Text != currentRule.Text || savedRule.Icon != currentRule.Icon):
		return "Pending change; Save to persist"
	case !savedExists && currentExists:
		return "Pending add; Save to persist"
	}
	return ""
}

func parseFilenameExtensionRuleFields(extRaw, textRaw, iconRaw, targetRaw string) (fm.FilenameExtensionRule, error) {
	ext := fm.NormalizeFilenameExtension(extRaw)
	if ext == "" {
		return fm.FilenameExtensionRule{}, fmt.Errorf("extension must look like go, md, or tar.gz")
	}
	textHex := strings.TrimSpace(textRaw)
	if textHex != "" {
		if _, ok := fm.ParseHexColor(textHex); !ok {
			return fm.FilenameExtensionRule{}, fmt.Errorf("extension color must use #RRGGBB")
		}
		textHex = fm.NormalizeOptionalHexColor(textHex)
	}
	icon := fm.NormalizeFilenameIcon(iconRaw)
	if textHex == "" && icon == "" {
		return fm.FilenameExtensionRule{}, fmt.Errorf("extension rule needs a color, an icon, or both")
	}
	return fm.FilenameExtensionRule{
		Extension: ext,
		Text:      textHex,
		Icon:      icon,
	}, nil
}

func (st *settingsModalState) upsertCurrentFilenameExtensionRule() (string, error) {
	if st == nil {
		return "Add", nil
	}
	rule, err := parseFilenameExtensionRuleFields(st.filenameExtEdit.Text(), st.filenameExtTextEdit.Text(), st.filenameExtIcon, "")
	if err != nil {
		return "Add", err
	}
	action := "Add"
	key := filenameExtensionRuleKey(rule.Extension, "")
	oldIdx := st.filenameExtensionRuleIndex(st.filenameExtEditingKey)
	newIdx := st.filenameExtensionRuleIndex(key)
	if oldIdx >= 0 {
		if newIdx >= 0 && newIdx != oldIdx {
			return "Update", fmt.Errorf("an extension rule for %s already exists", rule.Extension)
		}
		st.filenameExtEntries[oldIdx] = rule
		action = "Update"
	} else if newIdx >= 0 {
		st.filenameExtEntries[newIdx] = rule
		action = "Update"
	} else {
		st.filenameExtEntries = append(st.filenameExtEntries, rule)
	}
	st.filenameExtEntries = fm.NormalizeFilenameExtensionRules(st.filenameExtEntries)
	st.loadFilenameExtensionFields(rule.Extension, rule.Text, rule.Icon, rule.Target)
	st.filenameExtEditingKey = ""
	return action, nil
}

func (st *settingsModalState) removeCurrentFilenameExtensionRule() bool {
	if st == nil {
		return false
	}
	key := st.filenameExtEditingKey
	if key == "" {
		key = filenameExtensionRuleKey(st.filenameExtEdit.Text(), "")
	}
	idx := st.filenameExtensionRuleIndex(key)
	if idx < 0 {
		return false
	}
	st.filenameExtEntries = append(st.filenameExtEntries[:idx], st.filenameExtEntries[idx+1:]...)
	st.loadFilenameExtensionFields(fm.NormalizeFilenameExtension(st.filenameExtEdit.Text()), "", "", "")
	return true
}

func (st *settingsModalState) filenameSizeRuleRowClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.filenameSizeRowClicks == nil {
		st.filenameSizeRowClicks = make(map[string]*widget.Clickable)
	}
	if click := st.filenameSizeRowClicks[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.filenameSizeRowClicks[key] = click
	return click
}

func (st *settingsModalState) filenameSizeRuleRowRemoveClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.filenameSizeRowRemove == nil {
		st.filenameSizeRowRemove = make(map[string]*widget.Clickable)
	}
	if click := st.filenameSizeRowRemove[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.filenameSizeRowRemove[key] = click
	return click
}

func (st *settingsModalState) filenameSizeRuleIndex(key string) int {
	if st == nil || key == "" {
		return -1
	}
	for i, rule := range st.filenameSizeEntries {
		if filenameSizeRuleKey(rule.Size, "", rule.Match) == key {
			return i
		}
	}
	return -1
}

func (st *settingsModalState) filenameSizeRule(key string) (fm.FilenameSizeRule, bool) {
	if idx := st.filenameSizeRuleIndex(key); idx >= 0 {
		return st.filenameSizeEntries[idx], true
	}
	return fm.FilenameSizeRule{}, false
}

func (st *settingsModalState) filenameSavedSizeRule(key string) (fm.FilenameSizeRule, bool) {
	if st == nil || key == "" {
		return fm.FilenameSizeRule{}, false
	}
	for _, rule := range st.filenameSizeSavedEntries {
		if filenameSizeRuleKey(rule.Size, "", rule.Match) == key {
			return rule, true
		}
	}
	return fm.FilenameSizeRule{}, false
}

func (st *settingsModalState) loadFilenameSizeFields(size, match, textHex, icon, target string) {
	if st == nil {
		return
	}
	value, unit, ok := splitFilenameSizeValue(size)
	if !ok {
		value = strings.TrimSpace(size)
		unit = normalizeFilenameSizeUnit(st.filenameSizeUnit)
	}
	st.filenameSizeEdit.SetText(value)
	st.filenameSizeUnit = unit
	st.filenameSizeMatch = normalizeFilenameSizeMatch(match)
	st.filenameSizeTextEdit.SetText(textHex)
	st.filenameSizeIcon = fm.NormalizeFilenameIcon(icon)
	st.filenameSizeLookup = filenameSizeRuleKey(value, unit, st.filenameSizeMatch)
	st.filenameSizeEditingKey = ""
	if st.filenameSizeLookup != "" && (fm.NormalizeOptionalHexColor(textHex) != "" || st.filenameSizeIcon != "") {
		st.filenameSizeEditingKey = st.filenameSizeLookup
	}
}

func (st *settingsModalState) syncFilenameSizeEditors() {
	if st == nil {
		return
	}
	key := filenameSizeRuleKey(st.filenameSizeEdit.Text(), st.filenameSizeUnit, st.filenameSizeMatch)
	if key == st.filenameSizeLookup {
		return
	}
	st.filenameSizeLookup = key
	if st.filenameSizeEditingKey != "" {
		return
	}
	if rule, ok := st.filenameSizeRule(key); ok {
		st.loadFilenameSizeFields(rule.Size, rule.Match, rule.Text, rule.Icon, rule.Target)
		return
	}
	st.filenameSizeTextEdit.SetText("")
	st.filenameSizeIcon = ""
}

func (st *settingsModalState) refreshFilenameSizeDraftInfo() {
	if st == nil {
		return
	}
	st.filenameSizeInfoText = ""
	key := filenameSizeRuleKey(st.filenameSizeEdit.Text(), st.filenameSizeUnit, st.filenameSizeMatch)
	if key == "" {
		return
	}
	textHex := fm.NormalizeOptionalHexColor(strings.TrimSpace(st.filenameSizeTextEdit.Text()))
	icon := fm.NormalizeFilenameIcon(st.filenameSizeIcon)
	if textHex == "" && icon == "" {
		st.filenameSizeInfoText = "Pick a color, icon, or both"
		return
	}
	existingKey := key
	if st.filenameSizeEditingKey != "" {
		existingKey = st.filenameSizeEditingKey
	}
	existing, ok := st.filenameSizeRule(existingKey)
	if !ok {
		st.filenameSizeInfoText = "Click Add"
		return
	}
	if existingKey == key && existing.Text == textHex && existing.Icon == icon {
		return
	}
	st.filenameSizeInfoText = "Click Update"
}

func (st *settingsModalState) filenameSizeNoticeText() string {
	if st == nil {
		return ""
	}
	key := filenameSizeRuleKey(st.filenameSizeEdit.Text(), st.filenameSizeUnit, st.filenameSizeMatch)
	if key == "" {
		return ""
	}
	textHex := fm.NormalizeOptionalHexColor(strings.TrimSpace(st.filenameSizeTextEdit.Text()))
	icon := fm.NormalizeFilenameIcon(st.filenameSizeIcon)
	if st.filenameSizeEditingKey != "" {
		if editingRule, ok := st.filenameSizeRule(st.filenameSizeEditingKey); ok &&
			(st.filenameSizeEditingKey != key || editingRule.Text != textHex || editingRule.Icon != icon) {
			return "Click Update"
		}
	}
	savedRule, savedExists := st.filenameSavedSizeRule(key)
	currentRule, currentExists := st.filenameSizeRule(key)
	switch {
	case savedExists && !currentExists:
		return "Pending removal; Save to persist"
	case !currentExists && (textHex != "" || icon != ""):
		return "Click Add"
	case currentExists && (currentRule.Text != textHex || currentRule.Icon != icon):
		return "Click Update"
	case savedExists && currentExists && (savedRule.Text != currentRule.Text || savedRule.Icon != currentRule.Icon):
		return "Pending change; Save to persist"
	case !savedExists && currentExists:
		return "Pending add; Save to persist"
	}
	return ""
}

func parseFilenameSizeRuleFields(sizeRaw, unitRaw, matchRaw, textRaw, iconRaw string) (fm.FilenameSizeRule, error) {
	size, ok := fm.NormalizeFilenameSize(filenameSizeValueFromFields(sizeRaw, unitRaw))
	if !ok {
		return fm.FilenameSizeRule{}, fmt.Errorf("size must use a whole number")
	}
	textHex := strings.TrimSpace(textRaw)
	if textHex != "" {
		if _, ok := fm.ParseHexColor(textHex); !ok {
			return fm.FilenameSizeRule{}, fmt.Errorf("size color must use #RRGGBB")
		}
		textHex = fm.NormalizeOptionalHexColor(textHex)
	}
	icon := fm.NormalizeFilenameIcon(iconRaw)
	if textHex == "" && icon == "" {
		return fm.FilenameSizeRule{}, fmt.Errorf("size rule needs a color, an icon, or both")
	}
	return fm.FilenameSizeRule{
		Size:  size,
		Match: normalizeFilenameSizeMatch(matchRaw),
		Text:  textHex,
		Icon:  icon,
	}, nil
}

func (st *settingsModalState) upsertCurrentFilenameSizeRule() (string, error) {
	if st == nil {
		return "Add", nil
	}
	rule, err := parseFilenameSizeRuleFields(st.filenameSizeEdit.Text(), st.filenameSizeUnit, st.filenameSizeMatch, st.filenameSizeTextEdit.Text(), st.filenameSizeIcon)
	if err != nil {
		return "Add", err
	}
	action := "Add"
	key := filenameSizeRuleKey(rule.Size, "", rule.Match)
	oldIdx := st.filenameSizeRuleIndex(st.filenameSizeEditingKey)
	newIdx := st.filenameSizeRuleIndex(key)
	if oldIdx >= 0 {
		if newIdx >= 0 && newIdx != oldIdx {
			return "Update", fmt.Errorf("a size rule for %s already exists", key)
		}
		st.filenameSizeEntries[oldIdx] = rule
		action = "Update"
	} else if newIdx >= 0 {
		st.filenameSizeEntries[newIdx] = rule
		action = "Update"
	} else {
		st.filenameSizeEntries = append(st.filenameSizeEntries, rule)
	}
	st.filenameSizeEntries = fm.NormalizeFilenameSizeRules(st.filenameSizeEntries)
	st.loadFilenameSizeFields(rule.Size, rule.Match, rule.Text, rule.Icon, rule.Target)
	st.filenameSizeEditingKey = ""
	return action, nil
}

func (st *settingsModalState) removeCurrentFilenameSizeRule() bool {
	if st == nil {
		return false
	}
	key := st.filenameSizeEditingKey
	if key == "" {
		key = filenameSizeRuleKey(st.filenameSizeEdit.Text(), st.filenameSizeUnit, st.filenameSizeMatch)
	}
	idx := st.filenameSizeRuleIndex(key)
	if idx < 0 {
		return false
	}
	st.filenameSizeEntries = append(st.filenameSizeEntries[:idx], st.filenameSizeEntries[idx+1:]...)
	st.loadFilenameSizeFields(st.filenameSizeEdit.Text(), st.filenameSizeMatch, "", "", "")
	return true
}

func (ui *UI) layoutSettingsFilenameExtensionList(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	items := make([]settingsFilenameRuleListItem, 0, len(st.filenameExtEntries))
	for _, rule := range st.filenameExtEntries {
		colorText := rule.Text
		if colorText == "" {
			colorText = "default color"
		}
		items = append(items, settingsFilenameRuleListItem{
			key:      filenameExtensionRuleKey(rule.Extension, rule.Target),
			title:    formatFilenameExtensionRuleLabel(rule),
			detail:   filenameIconLabel(rule.Icon) + " • " + colorText,
			colorHex: rule.Text,
			iconKey:  rule.Icon,
		})
	}
	currentKey := filenameExtensionRuleKey(st.filenameExtEdit.Text(), "")
	return ui.layoutSettingsFilenameRuleList(th, gtx, &st.filenameExtList, items, "No extension overrides yet", currentKey, st.filenameExtensionRuleRowClick, st.filenameExtensionRuleRowRemoveClick, func(key string) {
		rule, ok := st.filenameExtensionRule(key)
		if !ok {
			return
		}
		st.loadFilenameExtensionFields(rule.Extension, rule.Text, rule.Icon, rule.Target)
		st.filenameExtInfoText = ""
	}, func(key string) {
		if idx := st.filenameExtensionRuleIndex(key); idx >= 0 {
			st.filenameExtEntries = append(st.filenameExtEntries[:idx], st.filenameExtEntries[idx+1:]...)
			st.filenameExtInfoText = "Pending removal; Save to persist"
		}
	})
}

func (ui *UI) layoutSettingsFilenameSizeList(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	items := make([]settingsFilenameRuleListItem, 0, len(st.filenameSizeEntries))
	for _, rule := range st.filenameSizeEntries {
		colorText := rule.Text
		if colorText == "" {
			colorText = "default color"
		}
		items = append(items, settingsFilenameRuleListItem{
			key:      filenameSizeRuleKey(rule.Size, "", rule.Match),
			title:    formatFilenameSizeRuleLabel(rule),
			detail:   filenameIconLabel(rule.Icon) + " • " + colorText,
			colorHex: rule.Text,
			iconKey:  rule.Icon,
		})
	}
	currentKey := filenameSizeRuleKey(st.filenameSizeEdit.Text(), st.filenameSizeUnit, st.filenameSizeMatch)
	return ui.layoutSettingsFilenameRuleList(th, gtx, &st.filenameSizeList, items, "No size overrides yet", currentKey, st.filenameSizeRuleRowClick, st.filenameSizeRuleRowRemoveClick, func(key string) {
		rule, ok := st.filenameSizeRule(key)
		if !ok {
			return
		}
		st.loadFilenameSizeFields(rule.Size, rule.Match, rule.Text, rule.Icon, rule.Target)
		st.filenameSizeInfoText = ""
	}, func(key string) {
		if idx := st.filenameSizeRuleIndex(key); idx >= 0 {
			st.filenameSizeEntries = append(st.filenameSizeEntries[:idx], st.filenameSizeEntries[idx+1:]...)
			st.filenameSizeInfoText = "Pending removal; Save to persist"
		}
	})
}
