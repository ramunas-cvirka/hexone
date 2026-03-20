// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"hexone/filesys"
	"hexone/fm"
	uitheme "hexone/ui/theme"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type filenameAgeUnitOption struct {
	key      string
	label    string
	singular string
	plural   string
}

type settingsFilenameRuleListItem struct {
	key      string
	title    string
	detail   string
	colorHex string
	iconKey  string
}

var filenameAgeUnitOptions = []filenameAgeUnitOption{
	{key: "m", label: "Min", singular: "minute", plural: "minutes"},
	{key: "h", label: "Hour", singular: "hour", plural: "hours"},
	{key: "d", label: "Day", singular: "day", plural: "days"},
	{key: "w", label: "Week", singular: "week", plural: "weeks"},
}

func normalizeFilenameAgeUnit(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "m", "min", "mins", "minute", "minutes":
		return "m"
	case "d", "day", "days":
		return "d"
	case "w", "wk", "wks", "week", "weeks":
		return "w"
	case "h", "hr", "hrs", "hour", "hours":
		fallthrough
	default:
		return "h"
	}
}

func filenameAgeUnitOptionForKey(key string) filenameAgeUnitOption {
	key = normalizeFilenameAgeUnit(key)
	for _, opt := range filenameAgeUnitOptions {
		if opt.key == key {
			return opt
		}
	}
	return filenameAgeUnitOptions[1]
}

func splitFilenameAgeValue(raw string) (string, string, bool) {
	age, ok := fm.NormalizeFilenameAge(raw)
	if !ok || len(age) < 2 {
		return "", "", false
	}
	return age[:len(age)-1], string(age[len(age)-1]), true
}

func filenameAgeRuleKeyFromFields(offsetRaw, unit string) string {
	offset := strings.TrimSpace(offsetRaw)
	if offset == "" {
		return ""
	}
	age, ok := fm.NormalizeFilenameAge(offset + normalizeFilenameAgeUnit(unit))
	if !ok {
		return ""
	}
	return age
}

func formatFilenameAgeRuleLabel(rule fm.FilenameAgeRule) string {
	count, unit, ok := splitFilenameAgeValue(rule.MaxAge)
	if !ok {
		return strings.TrimSpace(rule.MaxAge)
	}
	opt := filenameAgeUnitOptionForKey(unit)
	noun := opt.plural
	if count == "1" {
		noun = opt.singular
	}
	return count + " " + noun
}

func settingsFilenamePreviewSampleAge(previous, maxAge time.Duration) time.Duration {
	if maxAge <= 0 {
		return time.Hour
	}
	if previous <= 0 {
		sample := maxAge / 2
		if sample <= 0 {
			sample = maxAge
		}
		if sample > maxAge {
			sample = maxAge
		}
		return sample
	}
	if previous >= maxAge {
		return maxAge
	}
	sample := previous + (maxAge-previous)/2
	if sample <= previous || sample > maxAge {
		return maxAge
	}
	return sample
}

func settingsFilenamePreviewSamplePermissions(rule fm.FilenamePermissionRule) string {
	want, ok := fm.ParseFilenamePermissions(rule.Permissions)
	if !ok {
		return ""
	}
	switch normalizeFilenamePermissionMatch(rule.Match) {
	case fm.FilenamePermissionMatchNone:
		return fmt.Sprintf("%04o", (^want)&0o777)
	default:
		return fmt.Sprintf("%04o", want)
	}
}

func normalizeFilenameRuleMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "permissions", "permission", "perm":
		return "permissions"
	case "extensions", "extension", "ext":
		return "extensions"
	case "sizes", "size":
		return "sizes"
	default:
		return "age"
	}
}

func nextFilenameIcon(current string) string {
	current = fm.NormalizeFilenameIcon(current)
	for i, opt := range filenameIconOptions {
		if opt.key == current {
			return filenameIconOptions[(i+1)%len(filenameIconOptions)].key
		}
	}
	return filenameIconOptions[0].key
}

func (st *settingsModalState) loadFilenameColorsFromConfig(cfg *fm.Config) {
	if st == nil {
		return
	}
	st.filenameDefaultText = ""
	st.filenameDefaultIcon = ""
	st.filenameRuleMode = normalizeFilenameRuleMode(st.filenameRuleMode)
	st.filenameRuleModeAnim = settingsChoiceAnim{}
	st.filenameAgeUnitAnim = settingsChoiceAnim{}
	st.filenamePermMatchAnim = settingsChoiceAnim{}
	st.filenameSizeMatchAnim = settingsChoiceAnim{}
	st.filenameAgeEntries = nil
	st.filenameAgeSavedEntries = nil
	st.filenamePermEntries = nil
	st.filenamePermSavedEntries = nil
	st.filenameExtEntries = nil
	st.filenameExtSavedEntries = nil
	st.filenameSizeEntries = nil
	st.filenameSizeSavedEntries = nil
	defaultEditorHex := ""
	if cfg != nil {
		st.filenameDefaultText = cfg.Colors.Filenames.Text
		st.filenameDefaultIcon = cfg.Colors.Filenames.Icon
		defaultEditorHex = cfg.Colors.Filenames.Text
		if strings.TrimSpace(defaultEditorHex) == "" {
			defaultEditorHex = cfg.Colors.FilePaneText
		}
		st.filenameAgeEntries = append([]fm.FilenameAgeRule(nil), cfg.Colors.Filenames.AgeRules...)
		st.filenameAgeSavedEntries = append([]fm.FilenameAgeRule(nil), st.filenameAgeEntries...)
		st.filenamePermEntries = append([]fm.FilenamePermissionRule(nil), cfg.Colors.Filenames.PermissionRules...)
		st.filenamePermSavedEntries = append([]fm.FilenamePermissionRule(nil), st.filenamePermEntries...)
		st.filenameExtEntries = append([]fm.FilenameExtensionRule(nil), cfg.Colors.Filenames.ExtensionRules...)
		st.filenameExtSavedEntries = append([]fm.FilenameExtensionRule(nil), st.filenameExtEntries...)
		st.filenameSizeEntries = append([]fm.FilenameSizeRule(nil), cfg.Colors.Filenames.SizeRules...)
		st.filenameSizeSavedEntries = append([]fm.FilenameSizeRule(nil), st.filenameSizeEntries...)
	}
	st.filenameDefaultTextEdit.SetText(defaultEditorHex)
	st.filenameAgeList.Position.First = 0
	st.filenameAgeList.Position.Offset = 0
	st.filenameAgeRowClicks = nil
	st.filenameAgeRowRemove = nil
	st.loadFilenameAgeFields("", "h", "", "")
	st.filenameAgeInfoText = ""
	st.filenamePermList.Position.First = 0
	st.filenamePermList.Position.Offset = 0
	st.filenamePermPickerOpen = false
	st.filenamePermRowClicks = nil
	st.filenamePermRowRemove = nil
	st.loadFilenamePermissionFields("", "", "", "")
	st.filenamePermInfoText = ""
	st.filenameExtList.Position.First = 0
	st.filenameExtList.Position.Offset = 0
	st.filenameExtRowClicks = nil
	st.filenameExtRowRemove = nil
	st.loadFilenameExtensionFields("", "", "")
	st.filenameExtInfoText = ""
	st.filenameSizeList.Position.First = 0
	st.filenameSizeList.Position.Offset = 0
	st.filenameSizeRowClicks = nil
	st.filenameSizeRowRemove = nil
	st.loadFilenameSizeFields("", "", "", "")
	st.filenameSizeInfoText = ""
}

func (st *settingsModalState) draftFilenameColors() (fm.FilenameColorsConfig, string) {
	if st == nil {
		return fm.FilenameColorsConfig{}, ""
	}
	out := fm.FilenameColorsConfig{
		Text: fm.NormalizeOptionalHexColor(strings.TrimSpace(st.filenameDefaultText)),
		Icon: fm.NormalizeFilenameIcon(st.filenameDefaultIcon),
	}
	if raw := strings.TrimSpace(st.filenameDefaultText); raw != "" {
		if _, ok := fm.ParseHexColor(raw); !ok {
			return out, "Filename default color must use #RRGGBB"
		}
	}
	for i, rule := range st.filenameAgeEntries {
		rawAge := strings.TrimSpace(rule.MaxAge)
		rawText := strings.TrimSpace(rule.Text)
		icon := fm.NormalizeFilenameIcon(rule.Icon)
		if rawText == "" && icon == "" {
			continue
		}
		age, ok := fm.NormalizeFilenameAge(rawAge)
		if !ok {
			return out, fmt.Sprintf("Age rule %d must use a positive offset in minutes, hours, days, or weeks", i+1)
		}
		if rawText != "" {
			if _, ok := fm.ParseHexColor(rawText); !ok {
				return out, fmt.Sprintf("Age rule %d color must use #RRGGBB", i+1)
			}
		}
		out.AgeRules = append(out.AgeRules, fm.FilenameAgeRule{
			MaxAge: age,
			Text:   fm.NormalizeOptionalHexColor(rawText),
			Icon:   icon,
		})
	}
	out.PermissionRules = fm.NormalizeFilenamePermissionRules(st.filenamePermEntries)
	out.ExtensionRules = fm.NormalizeFilenameExtensionRules(st.filenameExtEntries)
	out.SizeRules = fm.NormalizeFilenameSizeRules(st.filenameSizeEntries)
	return out, ""
}

func (st *settingsModalState) previewFilenameColors(colors fm.FilenameColorsConfig) fm.FilenameColorsConfig {
	if st == nil {
		return colors
	}
	if rule, err := parseFilenameAgeRuleFields(st.filenameAgeOffsetEdit.Text(), st.filenameAgeUnit, st.filenameAgeTextEdit.Text(), st.filenameAgeIcon); err == nil {
		rules := append([]fm.FilenameAgeRule(nil), colors.AgeRules...)
		rules = append(rules, rule)
		colors.AgeRules = fm.NormalizeFilenameAgeRules(rules)
	}
	if rule, err := parseFilenamePermissionRuleFields(st.filenamePermEdit.Text(), st.filenamePermMatch, st.filenamePermTextEdit.Text(), st.filenamePermIcon); err == nil {
		rules := append([]fm.FilenamePermissionRule(nil), colors.PermissionRules...)
		rules = append(rules, rule)
		colors.PermissionRules = fm.NormalizeFilenamePermissionRules(rules)
	}
	if rule, err := parseFilenameExtensionRuleFields(st.filenameExtEdit.Text(), st.filenameExtTextEdit.Text(), st.filenameExtIcon); err == nil {
		rules := append([]fm.FilenameExtensionRule(nil), colors.ExtensionRules...)
		rules = append(rules, rule)
		colors.ExtensionRules = fm.NormalizeFilenameExtensionRules(rules)
	}
	if rule, err := parseFilenameSizeRuleFields(st.filenameSizeEdit.Text(), st.filenameSizeMatch, st.filenameSizeTextEdit.Text(), st.filenameSizeIcon); err == nil {
		rules := append([]fm.FilenameSizeRule(nil), colors.SizeRules...)
		rules = append(rules, rule)
		colors.SizeRules = fm.NormalizeFilenameSizeRules(rules)
	}
	return colors
}

func (st *settingsModalState) previewFilenameTheme(cfg *fm.Config) (filePanePalette, filePaneFilenameTheme, fm.FilenameColorsConfig, string) {
	palette, errText := st.draftFilePanePalette(cfg)
	filenameColors, filenameErr := st.draftFilenameColors()
	filenameColors = st.previewFilenameColors(filenameColors)
	if errText == "" {
		errText = filenameErr
	}
	draft := fm.DefaultConfig()
	if cfg != nil {
		draft.Colors = cfg.Colors
	}
	draft.Colors = filePanePaletteToConfigColors(palette)
	draft.Colors.Filenames = filenameColors
	return palette, newFilePaneFilenameTheme(draft), filenameColors, errText
}

func (st *settingsModalState) filenameAgeRuleRowClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.filenameAgeRowClicks == nil {
		st.filenameAgeRowClicks = make(map[string]*widget.Clickable)
	}
	if click := st.filenameAgeRowClicks[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.filenameAgeRowClicks[key] = click
	return click
}

func (st *settingsModalState) filenameAgeRuleRowRemoveClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.filenameAgeRowRemove == nil {
		st.filenameAgeRowRemove = make(map[string]*widget.Clickable)
	}
	if click := st.filenameAgeRowRemove[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.filenameAgeRowRemove[key] = click
	return click
}

func (st *settingsModalState) filenameAgeRuleIndex(maxAge string) int {
	if st == nil || maxAge == "" {
		return -1
	}
	for i, rule := range st.filenameAgeEntries {
		if rule.MaxAge == maxAge {
			return i
		}
	}
	return -1
}

func (st *settingsModalState) filenameAgeRule(maxAge string) (fm.FilenameAgeRule, bool) {
	if idx := st.filenameAgeRuleIndex(maxAge); idx >= 0 {
		return st.filenameAgeEntries[idx], true
	}
	return fm.FilenameAgeRule{}, false
}

func (st *settingsModalState) filenameSavedAgeRule(maxAge string) (fm.FilenameAgeRule, bool) {
	if st == nil || maxAge == "" {
		return fm.FilenameAgeRule{}, false
	}
	for _, rule := range st.filenameAgeSavedEntries {
		if rule.MaxAge == maxAge {
			return rule, true
		}
	}
	return fm.FilenameAgeRule{}, false
}

func (st *settingsModalState) loadFilenameAgeFields(offset, unit, textHex, icon string) {
	if st == nil {
		return
	}
	st.filenameAgeOffsetEdit.SetText(strings.TrimSpace(offset))
	st.filenameAgeUnit = normalizeFilenameAgeUnit(unit)
	st.filenameAgeTextEdit.SetText(textHex)
	st.filenameAgeIcon = fm.NormalizeFilenameIcon(icon)
	st.filenameAgeLookup = filenameAgeRuleKeyFromFields(offset, st.filenameAgeUnit)
}

func (st *settingsModalState) syncFilenameAgeEditors() {
	if st == nil {
		return
	}
	maxAge := filenameAgeRuleKeyFromFields(st.filenameAgeOffsetEdit.Text(), st.filenameAgeUnit)
	if maxAge == st.filenameAgeLookup {
		return
	}
	st.filenameAgeLookup = maxAge
	if rule, ok := st.filenameAgeRule(maxAge); ok {
		st.filenameAgeTextEdit.SetText(rule.Text)
		st.filenameAgeIcon = rule.Icon
		return
	}
	st.filenameAgeTextEdit.SetText("")
	st.filenameAgeIcon = ""
}

func (st *settingsModalState) refreshFilenameAgeDraftInfo() {
	if st == nil {
		return
	}
	st.filenameAgeInfoText = ""
	offset := strings.TrimSpace(st.filenameAgeOffsetEdit.Text())
	if offset == "" {
		return
	}
	maxAge := filenameAgeRuleKeyFromFields(offset, st.filenameAgeUnit)
	if maxAge == "" {
		st.filenameAgeInfoText = "Offset must be a whole number greater than zero"
		return
	}
	textHex := fm.NormalizeOptionalHexColor(strings.TrimSpace(st.filenameAgeTextEdit.Text()))
	icon := fm.NormalizeFilenameIcon(st.filenameAgeIcon)
	if textHex == "" && icon == "" {
		st.filenameAgeInfoText = "Choose a color, an icon, or both"
		return
	}
	existing, ok := st.filenameAgeRule(maxAge)
	if !ok {
		st.filenameAgeInfoText = "Click Add"
		return
	}
	if existing.Text == textHex && existing.Icon == icon {
		return
	}
	st.filenameAgeInfoText = "Click Update"
}

func (st *settingsModalState) filenameAgeNoticeText() string {
	if st == nil {
		return ""
	}
	offset := strings.TrimSpace(st.filenameAgeOffsetEdit.Text())
	if offset == "" {
		return "Use a positive offset and choose minutes, hours, days, or weeks"
	}
	maxAge := filenameAgeRuleKeyFromFields(offset, st.filenameAgeUnit)
	if maxAge == "" {
		return "Offset must be a whole number greater than zero"
	}
	textHex := fm.NormalizeOptionalHexColor(strings.TrimSpace(st.filenameAgeTextEdit.Text()))
	icon := fm.NormalizeFilenameIcon(st.filenameAgeIcon)
	savedRule, savedExists := st.filenameSavedAgeRule(maxAge)
	currentRule, currentExists := st.filenameAgeRule(maxAge)
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

func parseFilenameAgeRuleFields(offsetRaw, unitRaw, textRaw, iconRaw string) (fm.FilenameAgeRule, error) {
	maxAge := filenameAgeRuleKeyFromFields(offsetRaw, unitRaw)
	if maxAge == "" {
		return fm.FilenameAgeRule{}, fmt.Errorf("age offset must be a whole number greater than zero")
	}
	textHex := strings.TrimSpace(textRaw)
	if textHex != "" {
		if _, ok := fm.ParseHexColor(textHex); !ok {
			return fm.FilenameAgeRule{}, fmt.Errorf("age rule color must use #RRGGBB")
		}
		textHex = fm.NormalizeOptionalHexColor(textHex)
	}
	icon := fm.NormalizeFilenameIcon(iconRaw)
	if textHex == "" && icon == "" {
		return fm.FilenameAgeRule{}, fmt.Errorf("age rule needs a color, an icon, or both")
	}
	return fm.FilenameAgeRule{
		MaxAge: maxAge,
		Text:   textHex,
		Icon:   icon,
	}, nil
}

func (st *settingsModalState) upsertCurrentFilenameAgeRule() (string, error) {
	if st == nil {
		return "Add", nil
	}
	rule, err := parseFilenameAgeRuleFields(st.filenameAgeOffsetEdit.Text(), st.filenameAgeUnit, st.filenameAgeTextEdit.Text(), st.filenameAgeIcon)
	if err != nil {
		return "Add", err
	}
	action := "Add"
	if idx := st.filenameAgeRuleIndex(rule.MaxAge); idx >= 0 {
		st.filenameAgeEntries[idx] = rule
		action = "Update"
	} else {
		st.filenameAgeEntries = append(st.filenameAgeEntries, rule)
	}
	st.filenameAgeEntries = fm.NormalizeFilenameAgeRules(st.filenameAgeEntries)
	if offset, unit, ok := splitFilenameAgeValue(rule.MaxAge); ok {
		st.loadFilenameAgeFields(offset, unit, rule.Text, rule.Icon)
	}
	return action, nil
}

func (st *settingsModalState) removeCurrentFilenameAgeRule() bool {
	if st == nil {
		return false
	}
	maxAge := filenameAgeRuleKeyFromFields(st.filenameAgeOffsetEdit.Text(), st.filenameAgeUnit)
	idx := st.filenameAgeRuleIndex(maxAge)
	if idx < 0 {
		return false
	}
	st.filenameAgeEntries = append(st.filenameAgeEntries[:idx], st.filenameAgeEntries[idx+1:]...)
	st.loadFilenameAgeFields("", st.filenameAgeUnit, "", "")
	return true
}

func (st *settingsModalState) filenamePermissionRuleRowClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.filenamePermRowClicks == nil {
		st.filenamePermRowClicks = make(map[string]*widget.Clickable)
	}
	if click := st.filenamePermRowClicks[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.filenamePermRowClicks[key] = click
	return click
}

func (st *settingsModalState) filenamePermissionRuleRowRemoveClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.filenamePermRowRemove == nil {
		st.filenamePermRowRemove = make(map[string]*widget.Clickable)
	}
	if click := st.filenamePermRowRemove[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.filenamePermRowRemove[key] = click
	return click
}

func normalizeFilenamePermissionMatch(raw string) string {
	return fm.NormalizeFilenamePermissionMatch(raw)
}

func filenamePermissionRuleKey(permRaw, matchRaw string) string {
	perm := fm.NormalizeFilenamePermissions(permRaw)
	if perm == "" {
		return ""
	}
	return normalizeFilenamePermissionMatch(matchRaw) + ":" + perm
}

func filenamePermissionMatchLabel(match string) string {
	switch normalizeFilenamePermissionMatch(match) {
	case fm.FilenamePermissionMatchAll:
		return "Has All Bits"
	case fm.FilenamePermissionMatchAny:
		return "Has Any Bit"
	case fm.FilenamePermissionMatchNone:
		return "Has No Bits"
	default:
		return "Exact"
	}
}

func (st *settingsModalState) setFilenamePermissionChecks(permRaw string) {
	if st == nil {
		return
	}
	mask, ok := fm.ParseFilenamePermissions(permRaw)
	for i := range st.filenamePermChecks {
		st.filenamePermChecks[i].Value = ok && mask&uint16(permBitMasks[i]) != 0
	}
}

func (st *settingsModalState) filenamePermissionMaskFromChecks() string {
	if st == nil {
		return ""
	}
	var mask uint16
	for i := range st.filenamePermChecks {
		if st.filenamePermChecks[i].Value {
			mask |= uint16(permBitMasks[i])
		}
	}
	return fmt.Sprintf("%04o", mask)
}

func (st *settingsModalState) syncFilenamePermissionTextFromChecks() {
	if st == nil {
		return
	}
	st.filenamePermEdit.SetText(st.filenamePermissionMaskFromChecks())
}

func (st *settingsModalState) filenamePermissionRuleIndex(key string) int {
	if st == nil || key == "" {
		return -1
	}
	for i, rule := range st.filenamePermEntries {
		if filenamePermissionRuleKey(rule.Permissions, rule.Match) == key {
			return i
		}
	}
	return -1
}

func (st *settingsModalState) filenamePermissionRule(key string) (fm.FilenamePermissionRule, bool) {
	if idx := st.filenamePermissionRuleIndex(key); idx >= 0 {
		return st.filenamePermEntries[idx], true
	}
	return fm.FilenamePermissionRule{}, false
}

func (st *settingsModalState) filenameSavedPermissionRule(key string) (fm.FilenamePermissionRule, bool) {
	if st == nil || key == "" {
		return fm.FilenamePermissionRule{}, false
	}
	for _, rule := range st.filenamePermSavedEntries {
		if filenamePermissionRuleKey(rule.Permissions, rule.Match) == key {
			return rule, true
		}
	}
	return fm.FilenamePermissionRule{}, false
}

func (st *settingsModalState) loadFilenamePermissionFields(perm, match, textHex, icon string) {
	if st == nil {
		return
	}
	perm = fm.NormalizeFilenamePermissions(perm)
	st.filenamePermEdit.SetText(perm)
	st.filenamePermMatch = normalizeFilenamePermissionMatch(match)
	st.filenamePermTextEdit.SetText(textHex)
	st.filenamePermIcon = fm.NormalizeFilenameIcon(icon)
	st.setFilenamePermissionChecks(perm)
	st.filenamePermLookup = filenamePermissionRuleKey(perm, st.filenamePermMatch)
}

func (st *settingsModalState) syncFilenamePermissionEditors() {
	if st == nil {
		return
	}
	perm := fm.NormalizeFilenamePermissions(st.filenamePermEdit.Text())
	if perm != "" {
		st.setFilenamePermissionChecks(perm)
	}
	key := filenamePermissionRuleKey(perm, st.filenamePermMatch)
	if key == st.filenamePermLookup {
		return
	}
	st.filenamePermLookup = key
	if rule, ok := st.filenamePermissionRule(key); ok {
		st.filenamePermTextEdit.SetText(rule.Text)
		st.filenamePermIcon = rule.Icon
		st.filenamePermMatch = normalizeFilenamePermissionMatch(rule.Match)
		return
	}
	st.filenamePermTextEdit.SetText("")
	st.filenamePermIcon = ""
}

func (st *settingsModalState) refreshFilenamePermissionDraftInfo() {
	if st == nil {
		return
	}
	st.filenamePermInfoText = ""
	key := filenamePermissionRuleKey(st.filenamePermEdit.Text(), st.filenamePermMatch)
	if key == "" {
		return
	}
	textHex := fm.NormalizeOptionalHexColor(strings.TrimSpace(st.filenamePermTextEdit.Text()))
	icon := fm.NormalizeFilenameIcon(st.filenamePermIcon)
	if textHex == "" && icon == "" {
		st.filenamePermInfoText = "Choose a color, an icon, or both"
		return
	}
	existing, ok := st.filenamePermissionRule(key)
	if !ok {
		st.filenamePermInfoText = "Click Add"
		return
	}
	if existing.Text == textHex && existing.Icon == icon && normalizeFilenamePermissionMatch(existing.Match) == normalizeFilenamePermissionMatch(st.filenamePermMatch) {
		return
	}
	st.filenamePermInfoText = "Click Update"
}

func (st *settingsModalState) filenamePermissionNoticeText() string {
	if st == nil {
		return ""
	}
	key := filenamePermissionRuleKey(st.filenamePermEdit.Text(), st.filenamePermMatch)
	if key == "" {
		return "Use octal bits like 0644, 0111, or 0222 and choose Exact / Any / All / None"
	}
	textHex := fm.NormalizeOptionalHexColor(strings.TrimSpace(st.filenamePermTextEdit.Text()))
	icon := fm.NormalizeFilenameIcon(st.filenamePermIcon)
	savedRule, savedExists := st.filenameSavedPermissionRule(key)
	currentRule, currentExists := st.filenamePermissionRule(key)
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

func parseFilenamePermissionRuleFields(permRaw, matchRaw, textRaw, iconRaw string) (fm.FilenamePermissionRule, error) {
	perm := fm.NormalizeFilenamePermissions(permRaw)
	if perm == "" {
		return fm.FilenamePermissionRule{}, fmt.Errorf("permission value must use octal like 0644, 0755, 0111, or 0222")
	}
	match := normalizeFilenamePermissionMatch(matchRaw)
	if match != fm.FilenamePermissionMatchExact && perm == "0000" {
		return fm.FilenamePermissionRule{}, fmt.Errorf("partial permission matches need at least one bit selected")
	}
	textHex := strings.TrimSpace(textRaw)
	if textHex != "" {
		if _, ok := fm.ParseHexColor(textHex); !ok {
			return fm.FilenamePermissionRule{}, fmt.Errorf("permission color must use #RRGGBB")
		}
		textHex = fm.NormalizeOptionalHexColor(textHex)
	}
	icon := fm.NormalizeFilenameIcon(iconRaw)
	if textHex == "" && icon == "" {
		return fm.FilenamePermissionRule{}, fmt.Errorf("permission rule needs a color, an icon, or both")
	}
	return fm.FilenamePermissionRule{
		Permissions: perm,
		Match:       match,
		Text:        textHex,
		Icon:        icon,
	}, nil
}

func (st *settingsModalState) upsertCurrentFilenamePermissionRule() (string, error) {
	if st == nil {
		return "Add", nil
	}
	rule, err := parseFilenamePermissionRuleFields(st.filenamePermEdit.Text(), st.filenamePermMatch, st.filenamePermTextEdit.Text(), st.filenamePermIcon)
	if err != nil {
		return "Add", err
	}
	action := "Add"
	key := filenamePermissionRuleKey(rule.Permissions, rule.Match)
	if idx := st.filenamePermissionRuleIndex(key); idx >= 0 {
		st.filenamePermEntries[idx] = rule
		action = "Update"
	} else {
		st.filenamePermEntries = append(st.filenamePermEntries, rule)
	}
	st.filenamePermEntries = fm.NormalizeFilenamePermissionRules(st.filenamePermEntries)
	st.loadFilenamePermissionFields(rule.Permissions, rule.Match, rule.Text, rule.Icon)
	return action, nil
}

func (st *settingsModalState) removeCurrentFilenamePermissionRule() bool {
	if st == nil {
		return false
	}
	key := filenamePermissionRuleKey(st.filenamePermEdit.Text(), st.filenamePermMatch)
	idx := st.filenamePermissionRuleIndex(key)
	if idx < 0 {
		return false
	}
	st.filenamePermEntries = append(st.filenamePermEntries[:idx], st.filenamePermEntries[idx+1:]...)
	st.loadFilenamePermissionFields(fm.NormalizeFilenamePermissions(st.filenamePermEdit.Text()), st.filenamePermMatch, "", "")
	return true
}

func (ui *UI) layoutSettingsFilenameColorValueField(th *material.Theme, gtx layout.Context, st *settingsModalState, key string, edit *widget.Editor, picker *widget.Clickable, pickerTarget string, groups []settingsColorSwatchGroup) layout.Dimensions {
	edW := settingsColorHexEditorWidth(th, gtx, ui.fmCfg, ui.mainTypeface())
	raw := strings.TrimSpace(edit.Text())
	swatch := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
	if c, ok := fm.ParseHexColor(raw); ok {
		swatch = c
	}
	btnW := settingsColorPickerButtonWidth(th, gtx, ui.fmCfg, ui.mainTypeface())
	dims := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorPickerButton(th, gtx, st, swatch, picker, st.colorPickerOpen && st.colorPickerTarget == pickerTarget, btnW)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, edW, func(gtx layout.Context) layout.Dimensions {
				ed := material.Editor(th, edit, "#RRGGBB")
				ed.Font.Typeface = ui.mainTypeface()
				ed.TextSize = scaleModalThemeFontSize(th, 10)
				ed.Color = txtColor
				ed.HintColor = hintColor
				return ui.layoutEditorWithContextMenu(th, gtx, key, edit, true, func(gtx layout.Context) layout.Dimensions {
					return layoutNeutralEditorBox(gtx, gtx.Focused(edit), true, ed.Layout)
				})
			})
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

func settingsFilenameIconButtonWidth(th *material.Theme, gtx layout.Context, face font.Typeface) int {
	maxW := 0
	for _, opt := range filenameIconOptions {
		lbl := material.Body2(th, opt.label)
		lbl.Font.Typeface = face
		lbl.TextSize = scaleModalThemeFontSize(th, 10)
		lbl.MaxLines = 1
		if w := measureLabelUnconstrained(gtx, lbl).Size.X; w > maxW {
			maxW = w
		}
	}
	width := maxW + gtx.Dp(unit.Dp(42))
	minW := gtx.Dp(unit.Dp(112))
	if width < minW {
		width = minW
	}
	return width
}

func (ui *UI) layoutSettingsFilenameIconCycleButton(th *material.Theme, gtx layout.Context, click *widget.Clickable, iconKey string) layout.Dimensions {
	width := settingsFilenameIconButtonWidth(th, gtx, ui.mainTypeface())
	label := filenameIconLabel(iconKey)
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		dims := click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			bg := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
			bd := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
			if click.Hovered() {
				bg = color.NRGBA{R: 30, G: 34, B: 44, A: 255}
				bd = color.NRGBA{R: 130, G: 160, B: 255, A: 70}
			}
			return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), bg, bd, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							size := gtx.Dp(unit.Dp(13))
							if size < 1 {
								size = 1
							}
							if ic := filenamePreviewIcon(iconKey); ic != nil {
								iconGtx := gtx
								iconGtx.Constraints = layout.Exact(image.Pt(size, size))
								ic.Layout(iconGtx, color.NRGBA{R: 216, G: 226, B: 244, A: 255})
							}
							return layout.Dimensions{Size: image.Pt(size, size)}
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body2(th, label)
							lbl.Font.Typeface = ui.mainTypeface()
							lbl.TextSize = scaleModalThemeFontSize(th, 10)
							lbl.Color = txtColor
							lbl.MaxLines = 1
							lbl.Truncator = "..."
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

func (ui *UI) layoutSettingsFilenameAgeUnitPicker(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	if ui == nil || st == nil {
		return layout.Dimensions{}
	}
	selected := normalizeFilenameAgeUnit(st.filenameAgeUnit)
	st.filenameAgeUnit = selected
	keys := make([]string, len(filenameAgeUnitOptions))
	hoverKey := ""
	for i, opt := range filenameAgeUnitOptions {
		keys[i] = opt.key
		if st.filenameAgeUnitClicks[i].Clicked(gtx) {
			st.filenameAgeUnitAnim.anim.setPulse(opt.key, gtx.Now)
			st.filenameAgeUnitAnim.setValue(&st.filenameAgeUnit, opt.key, gtx.Now)
			st.errText = ""
			gtx.Execute(op.InvalidateCmd{})
		}
		if st.filenameAgeUnitClicks[i].Hovered() {
			hoverKey = opt.key
		}
	}
	st.filenameAgeUnitAnim.anim.setHover(hoverKey, gtx.Now)
	pos, animPos := st.filenameAgeUnitAnim.position(gtx.Now, st.filenameAgeUnit, keys)
	specs := make([]slidingTabSpec, 0, len(filenameAgeUnitOptions))
	animating := animPos
	for i, opt := range filenameAgeUnitOptions {
		activeFill, activeAnim := st.filenameAgeUnitAnim.fill(gtx.Now, st.filenameAgeUnit, opt.key)
		hoverFill, hoverAnim := st.filenameAgeUnitAnim.anim.hoverFill(gtx.Now, opt.key)
		pulseFill, pulseAnim := st.filenameAgeUnitAnim.anim.pulseFill(gtx.Now, opt.key)
		specs = append(specs, slidingTabSpec{
			Label:      opt.label,
			Click:      &st.filenameAgeUnitClicks[i],
			ActiveFill: activeFill,
			HoverFill:  hoverFill,
			PulseFill:  pulseFill,
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
	return ui.layoutSlidingTabStrip(th, gtx, stripH, pos, scaleModalThemeFontSize(th, 10), specs)
}

func (ui *UI) layoutSettingsFilenameRuleModeTabs(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	if ui == nil || st == nil {
		return layout.Dimensions{}
	}
	keys := []string{"age", "permissions", "extensions", "sizes"}
	if st.filenameRuleModeAgeClick.Clicked(gtx) {
		st.filenameRuleModeAnim.anim.setPulse("age", gtx.Now)
		st.filenameRuleModeAnim.setValue(&st.filenameRuleMode, "age", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.filenameRuleModePermClick.Clicked(gtx) {
		st.filenameRuleModeAnim.anim.setPulse("permissions", gtx.Now)
		st.filenameRuleModeAnim.setValue(&st.filenameRuleMode, "permissions", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.filenameRuleModeExtClick.Clicked(gtx) {
		st.filenameRuleModeAnim.anim.setPulse("extensions", gtx.Now)
		st.filenameRuleModeAnim.setValue(&st.filenameRuleMode, "extensions", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.filenameRuleModeSizeClick.Clicked(gtx) {
		st.filenameRuleModeAnim.anim.setPulse("sizes", gtx.Now)
		st.filenameRuleModeAnim.setValue(&st.filenameRuleMode, "sizes", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	activeMode := normalizeFilenameRuleMode(st.filenameRuleMode)
	st.filenameRuleMode = activeMode
	hoverKey := ""
	if st.filenameRuleModeAgeClick.Hovered() {
		hoverKey = "age"
	}
	if st.filenameRuleModePermClick.Hovered() {
		hoverKey = "permissions"
	}
	if st.filenameRuleModeExtClick.Hovered() {
		hoverKey = "extensions"
	}
	if st.filenameRuleModeSizeClick.Hovered() {
		hoverKey = "sizes"
	}
	st.filenameRuleModeAnim.anim.setHover(hoverKey, gtx.Now)
	pos, animPos := st.filenameRuleModeAnim.position(gtx.Now, activeMode, keys)
	fillAge, animAge := st.filenameRuleModeAnim.fill(gtx.Now, activeMode, "age")
	fillPerm, animPerm := st.filenameRuleModeAnim.fill(gtx.Now, activeMode, "permissions")
	fillExt, animExt := st.filenameRuleModeAnim.fill(gtx.Now, activeMode, "extensions")
	fillSize, animSize := st.filenameRuleModeAnim.fill(gtx.Now, activeMode, "sizes")
	hoverAge, hoverAnimAge := st.filenameRuleModeAnim.anim.hoverFill(gtx.Now, "age")
	hoverPerm, hoverAnimPerm := st.filenameRuleModeAnim.anim.hoverFill(gtx.Now, "permissions")
	hoverExt, hoverAnimExt := st.filenameRuleModeAnim.anim.hoverFill(gtx.Now, "extensions")
	hoverSize, hoverAnimSize := st.filenameRuleModeAnim.anim.hoverFill(gtx.Now, "sizes")
	pulseAge, pulseAnimAge := st.filenameRuleModeAnim.anim.pulseFill(gtx.Now, "age")
	pulsePerm, pulseAnimPerm := st.filenameRuleModeAnim.anim.pulseFill(gtx.Now, "permissions")
	pulseExt, pulseAnimExt := st.filenameRuleModeAnim.anim.pulseFill(gtx.Now, "extensions")
	pulseSize, pulseAnimSize := st.filenameRuleModeAnim.anim.pulseFill(gtx.Now, "sizes")
	specs := []slidingTabSpec{
		{
			Label:      "By Age",
			Click:      &st.filenameRuleModeAgeClick,
			ActiveFill: fillAge,
			HoverFill:  hoverAge,
			PulseFill:  pulseAge,
		},
		{
			Label:      "By Permissions",
			Click:      &st.filenameRuleModePermClick,
			ActiveFill: fillPerm,
			HoverFill:  hoverPerm,
			PulseFill:  pulsePerm,
		},
		{
			Label:      "By Extension",
			Click:      &st.filenameRuleModeExtClick,
			ActiveFill: fillExt,
			HoverFill:  hoverExt,
			PulseFill:  pulseExt,
		},
		{
			Label:      "By Size",
			Click:      &st.filenameRuleModeSizeClick,
			ActiveFill: fillSize,
			HoverFill:  hoverSize,
			PulseFill:  pulseSize,
		},
	}
	if animPos || animAge || animPerm || animExt || animSize ||
		hoverAnimAge || hoverAnimPerm || hoverAnimExt || hoverAnimSize ||
		pulseAnimAge || pulseAnimPerm || pulseAnimExt || pulseAnimSize {
		gtx.Execute(op.InvalidateCmd{})
	}
	stripH := gtx.Dp(unit.Dp(24))
	if stripH < 1 {
		stripH = 1
	}
	return ui.layoutSlidingTabStrip(th, gtx, stripH, pos, scaleModalThemeFontSize(th, 10), specs)
}

func (ui *UI) layoutSettingsFilenameRuleList(th *material.Theme, gtx layout.Context, listState *widget.List, items []settingsFilenameRuleListItem, emptyText, currentKey string, rowClick func(string) *widget.Clickable, rowRemove func(string) *widget.Clickable, onPick func(string), onRemove func(string)) layout.Dimensions {
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
		color.NRGBA{R: 20, G: 24, B: 32, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			if len(items) == 0 {
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, emptyText)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleModalThemeFontSize(th, 9)
					lbl.Color = hintColor
					return lbl.Layout(gtx)
				})
			}

			removed := ""
			picked := ""
			list := settingsPopupListStyle(th, listState)
			dims := list.Layout(gtx, len(items), func(gtx layout.Context, i int) layout.Dimensions {
				item := items[i]
				click := rowClick(item.key)
				removeClick := rowRemove(item.key)
				for click.Clicked(gtx) {
					if picked == "" {
						picked = item.key
					}
				}
				for removeClick.Clicked(gtx) {
					if removed == "" {
						removed = item.key
					}
				}
				selected := item.key == currentKey
				hovered := click.Hovered() || removeClick.Hovered()
				bg := color.NRGBA{}
				if selected {
					bg = color.NRGBA{R: 80, G: 120, B: 220, A: 45}
				} else if hovered {
					bg = color.NRGBA{R: 255, G: 255, B: 255, A: 10}
				}
				return layoutSettingsPickerRowBackground(gtx, bg, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									pointer.CursorPointer.Add(gtx.Ops)
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													lbl := material.Body2(th, item.title)
													lbl.Font.Typeface = ui.mainTypeface()
													lbl.Font.Weight = font.Medium
													lbl.TextSize = scaleModalThemeFontSize(th, 10)
													lbl.Color = txtColor
													lbl.MaxLines = 1
													return layoutVCenteredLabel(gtx, lbl)
												}),
												layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													size := gtx.Dp(unit.Dp(12))
													if size < 1 {
														size = 1
													}
													if ic := filenamePreviewIcon(item.iconKey); ic != nil {
														iconGtx := gtx
														iconGtx.Constraints = layout.Exact(image.Pt(size, size))
														iconColor := parseConfigColorHexFallback(item.colorHex, fm.DefaultFilePaneTextHex)
														ic.Layout(iconGtx, iconColor)
													}
													return layout.Dimensions{Size: image.Pt(size, size)}
												}),
											)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													swatch := parseConfigColorHexFallback(item.colorHex, fm.DefaultFilePaneTextHex)
													return fillRoundedBox(gtx, gtx.Dp(unit.Dp(3)), swatch, color.NRGBA{R: 255, G: 255, B: 255, A: 24}, func(gtx layout.Context) layout.Dimensions {
														size := gtx.Dp(unit.Dp(10))
														if size < 1 {
															size = 1
														}
														return layout.Dimensions{Size: image.Pt(size, size)}
													})
												}),
												layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													lbl := material.Caption(th, item.detail)
													lbl.Font.Typeface = ui.mainTypeface()
													lbl.TextSize = scaleModalThemeFontSize(th, 8)
													lbl.Color = hintColor
													lbl.MaxLines = 1
													lbl.Truncator = "..."
													return layoutVCenteredLabel(gtx, lbl)
												}),
											)
										}),
									)
								})
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutTinyIconModeButton(th, gtx, removeClick, uitheme.CloseIcon(), false)
							}),
						)
					})
				})
			})
			if removed != "" && onRemove != nil {
				onRemove(removed)
			}
			if picked != "" && onPick != nil {
				onPick(picked)
			}
			return dims
		},
	)
}

func (ui *UI) layoutSettingsFilenameAgeList(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	items := make([]settingsFilenameRuleListItem, 0, len(st.filenameAgeEntries))
	for _, rule := range st.filenameAgeEntries {
		colorText := rule.Text
		if colorText == "" {
			colorText = "default color"
		}
		items = append(items, settingsFilenameRuleListItem{
			key:      rule.MaxAge,
			title:    formatFilenameAgeRuleLabel(rule),
			detail:   filenameIconLabel(rule.Icon) + " • " + colorText,
			colorHex: rule.Text,
			iconKey:  rule.Icon,
		})
	}
	currentAge := filenameAgeRuleKeyFromFields(st.filenameAgeOffsetEdit.Text(), st.filenameAgeUnit)
	return ui.layoutSettingsFilenameRuleList(th, gtx, &st.filenameAgeList, items, "No age overrides yet", currentAge, st.filenameAgeRuleRowClick, st.filenameAgeRuleRowRemoveClick, func(key string) {
		rule, ok := st.filenameAgeRule(key)
		if !ok {
			return
		}
		offset, unitKey, _ := splitFilenameAgeValue(rule.MaxAge)
		st.loadFilenameAgeFields(offset, unitKey, rule.Text, rule.Icon)
		st.filenameAgeInfoText = ""
	}, func(key string) {
		if idx := st.filenameAgeRuleIndex(key); idx >= 0 {
			st.filenameAgeEntries = append(st.filenameAgeEntries[:idx], st.filenameAgeEntries[idx+1:]...)
			st.filenameAgeInfoText = "Pending removal; Save to persist"
		}
	})
}

func (ui *UI) layoutSettingsFilenamePermissionList(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	items := make([]settingsFilenameRuleListItem, 0, len(st.filenamePermEntries))
	for _, rule := range st.filenamePermEntries {
		colorText := rule.Text
		if colorText == "" {
			colorText = "default color"
		}
		items = append(items, settingsFilenameRuleListItem{
			key:      filenamePermissionRuleKey(rule.Permissions, rule.Match),
			title:    formatFilenamePermissionRuleLabel(rule),
			detail:   filenamePermissionMatchLabel(rule.Match) + " • " + filenameIconLabel(rule.Icon) + " • " + colorText,
			colorHex: rule.Text,
			iconKey:  rule.Icon,
		})
	}
	currentPerm := filenamePermissionRuleKey(st.filenamePermEdit.Text(), st.filenamePermMatch)
	return ui.layoutSettingsFilenameRuleList(th, gtx, &st.filenamePermList, items, "No permission overrides yet", currentPerm, st.filenamePermissionRuleRowClick, st.filenamePermissionRuleRowRemoveClick, func(key string) {
		rule, ok := st.filenamePermissionRule(key)
		if !ok {
			return
		}
		st.loadFilenamePermissionFields(rule.Permissions, rule.Match, rule.Text, rule.Icon)
		st.filenamePermInfoText = ""
	}, func(key string) {
		if idx := st.filenamePermissionRuleIndex(key); idx >= 0 {
			st.filenamePermEntries = append(st.filenamePermEntries[:idx], st.filenamePermEntries[idx+1:]...)
			st.filenamePermInfoText = "Pending removal; Save to persist"
		}
	})
}

func (ui *UI) layoutSettingsFilenamePreview(th *material.Theme, gtx layout.Context, palette filePanePalette, theme filePaneFilenameTheme, colors fm.FilenameColorsConfig) layout.Dimensions {
	now := time.Now()
	rows := []struct {
		label string
		entry filesys.Entry
	}{
		{label: "Default", entry: filesys.Entry{Name: "plain.txt", DisplayName: "plain.txt", Kind: filesys.EntryFile, PermOctal: "0640", ModTime: now.Add(-30 * 24 * time.Hour)}},
	}
	previousAge := time.Duration(0)
	for _, rule := range colors.AgeRules {
		if len(rows) >= 4 {
			break
		}
		maxAge, ok := fm.ParseFilenameAge(rule.MaxAge)
		if !ok || maxAge <= 0 {
			continue
		}
		sampleAge := settingsFilenamePreviewSampleAge(previousAge, maxAge)
		rows = append(rows, struct {
			label string
			entry filesys.Entry
		}{
			label: "<= " + formatFilenameAgeRuleLabel(rule),
			entry: filesys.Entry{
				Name:        "recent-" + rule.MaxAge + ".txt",
				DisplayName: "recent-" + rule.MaxAge + ".txt",
				Kind:        filesys.EntryFile,
				PermOctal:   "0640",
				ModTime:     now.Add(-sampleAge),
			},
		})
		previousAge = maxAge
	}
	for _, rule := range colors.PermissionRules {
		if len(rows) >= 6 {
			break
		}
		samplePerm := settingsFilenamePreviewSamplePermissions(rule)
		if samplePerm == "" {
			continue
		}
		rows = append(rows, struct {
			label string
			entry filesys.Entry
		}{
			label: formatFilenamePermissionRuleLabel(rule),
			entry: filesys.Entry{
				Name:        "mode-" + samplePerm + ".txt",
				DisplayName: "mode-" + samplePerm + ".txt",
				Kind:        filesys.EntryFile,
				PermOctal:   samplePerm,
				ModTime:     now.Add(-30 * 24 * time.Hour),
			},
		})
	}
	for _, rule := range colors.ExtensionRules {
		if len(rows) >= 8 {
			break
		}
		suffix := fm.NormalizeFilenameExtension(rule.Extension)
		if suffix == "" {
			continue
		}
		rows = append(rows, struct {
			label string
			entry filesys.Entry
		}{
			label: suffix,
			entry: filesys.Entry{
				Name:        "sample" + suffix,
				DisplayName: "sample" + suffix,
				Kind:        filesys.EntryFile,
				PermOctal:   "0640",
				ModTime:     now.Add(-30 * 24 * time.Hour),
			},
		})
	}
	for _, rule := range colors.SizeRules {
		if len(rows) >= 10 {
			break
		}
		limit, ok := fm.ParseFilenameSize(rule.Size)
		if !ok {
			continue
		}
		sampleSize := settingsFilenamePreviewSampleSize(limit, rule.Match)
		rows = append(rows, struct {
			label string
			entry filesys.Entry
		}{
			label: formatFilenameSizeRuleLabel(rule),
			entry: filesys.Entry{
				Name:        "size-" + strings.TrimPrefix(rule.Size, ".") + ".bin",
				DisplayName: "size-" + strings.TrimPrefix(rule.Size, ".") + ".bin",
				Kind:        filesys.EntryFile,
				PermOctal:   "0640",
				SizeBytes:   sampleSize,
				ModTime:     now.Add(-30 * 24 * time.Hour),
			},
		})
	}
	stateW := settingsColorPreviewStateWidth(th, gtx, ui.fmCfg, ui.mainTypeface())
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
		palette.PaneBg,
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(rows)*2+2)
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, "Filename Preview")
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleModalThemeFontSize(th, 9)
					lbl.Color = color.NRGBA{R: 176, G: 190, B: 215, A: 255}
					return lbl.Layout(gtx)
				}))
				children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout))
				for i, row := range rows {
					row := row
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						visual := theme.visualForEntry(row.entry, now)
						fg := palette.PaneFg
						if visual.hasColor {
							fg = visual.color
						}
						return ui.layoutSettingsFilenamePreviewRow(th, gtx, palette.PaneBg, stateW, row.label, row.entry.DisplayName, fg, visual.iconKey)
					}))
					if i < len(rows)-1 {
						children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout))
					}
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		},
	)
}

func (ui *UI) layoutSettingsFilenamePreviewRow(th *material.Theme, gtx layout.Context, bg color.NRGBA, stateW int, stateLabel, fileName string, fg color.NRGBA, iconKey string) layout.Dimensions {
	rowH := gtx.Dp(scaleFilePaneDp(ui.fmCfg, 18))
	if rowH < 1 {
		rowH = 1
	}
	stateColor := settingsColorPreviewStateColor(bg)
	return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
		return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, stateW, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(th, scaleConfigFontSize(ui.fmCfg, 13), stateLabel)
							lbl.Font.Typeface = ui.mainTypeface()
							lbl.Font.Weight = font.Normal
							lbl.Color = stateColor
							lbl.MaxLines = 1
							return layoutVCenteredLabel(gtx, lbl)
						})
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						size := gtx.Dp(unit.Dp(13))
						if size < 1 {
							size = 1
						}
						if ic := filenamePreviewIcon(iconKey); ic != nil {
							iconGtx := gtx
							iconGtx.Constraints = layout.Exact(image.Pt(size, size))
							ic.Layout(iconGtx, fg)
						}
						return layout.Dimensions{Size: image.Pt(size, size)}
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(th, scaleConfigFontSize(ui.fmCfg, 13), fileName)
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

func (ui *UI) layoutSettingsFilenameColorsTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	for {
		ev, ok := st.filenameDefaultTextEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.filenameDefaultText = st.filenameDefaultTextEdit.Text()
			st.errText = ""
		}
	}
	for {
		ev, ok := st.filenameAgeOffsetEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
		}
	}
	for {
		ev, ok := st.filenameAgeTextEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
		}
	}
	for {
		ev, ok := st.filenamePermEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
		}
	}
	for {
		ev, ok := st.filenamePermTextEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
		}
	}
	for i := range st.filenamePermChecks {
		if st.filenamePermChecks[i].Update(gtx) {
			st.syncFilenamePermissionTextFromChecks()
			st.errText = ""
		}
	}
	for {
		ev, ok := st.filenameExtEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
		}
	}
	for {
		ev, ok := st.filenameExtTextEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
		}
	}
	for {
		ev, ok := st.filenameSizeEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
		}
	}
	for {
		ev, ok := st.filenameSizeTextEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
		}
	}
	defaultSwatchGroups := st.colorPickerSwatchGroups("filename-default-text")
	ageSwatchGroups := st.colorPickerSwatchGroups("filename-age-text")
	permSwatchGroups := st.colorPickerSwatchGroups("filename-perm-text")
	extSwatchGroups := st.colorPickerSwatchGroups("filename-ext-text")
	sizeSwatchGroups := st.colorPickerSwatchGroups("filename-size-text")
	activeSwatchGroups := defaultSwatchGroups
	if st.colorPickerOpen {
		switch st.colorPickerTarget {
		case "filename-age-text":
			activeSwatchGroups = ageSwatchGroups
		case "filename-perm-text":
			activeSwatchGroups = permSwatchGroups
		case "filename-ext-text":
			activeSwatchGroups = extSwatchGroups
		case "filename-size-text":
			activeSwatchGroups = sizeSwatchGroups
		case "filename-default-text":
			activeSwatchGroups = defaultSwatchGroups
		}
	}
	st.ensureColorSwatchClicks(settingsColorSwatchCount(activeSwatchGroups))
	if st.colorPickerOpen {
		clickIdx := 0
		for _, group := range activeSwatchGroups {
			for _, hex := range group.hexes {
				if clickIdx >= len(st.colorSwatchClicks) {
					break
				}
				if st.colorSwatchClicks[clickIdx].Clicked(gtx) {
					st.setColorPickerHexValue(st.colorPickerTarget, hex)
					st.colorPickerOpen = false
					st.colorPickerTarget = ""
					st.errText = ""
				}
				clickIdx++
			}
		}
	}
	if st.filenameDefaultTextPicker.Clicked(gtx) {
		st.toggleColorPicker("filename-default-text")
	}
	if st.filenameAgeTextPicker.Clicked(gtx) {
		st.toggleColorPicker("filename-age-text")
	}
	if st.filenamePermTextPicker.Clicked(gtx) {
		st.toggleColorPicker("filename-perm-text")
	}
	if st.filenamePermPickerClick.Clicked(gtx) {
		st.toggleFilenamePermissionPicker()
	}
	if st.filenameExtTextPicker.Clicked(gtx) {
		st.toggleColorPicker("filename-ext-text")
	}
	if st.filenameSizeTextPicker.Clicked(gtx) {
		st.toggleColorPicker("filename-size-text")
	}
	st.syncFilenameAgeEditors()
	st.refreshFilenameAgeDraftInfo()
	if st.filenameAgeIconClick.Clicked(gtx) {
		st.filenameAgeIcon = nextFilenameIcon(st.filenameAgeIcon)
		st.errText = ""
		st.refreshFilenameAgeDraftInfo()
	}
	if st.filenameAgeApplyClick.Clicked(gtx) {
		action, err := st.upsertCurrentFilenameAgeRule()
		if err != nil {
			st.errText = err.Error()
		} else {
			st.errText = ""
			st.filenameAgeInfoText = "Pending " + strings.ToLower(action) + "; Save to persist"
		}
	}
	if st.filenameAgeRemoveClick.Clicked(gtx) {
		if st.removeCurrentFilenameAgeRule() {
			st.errText = ""
			st.filenameAgeInfoText = "Pending removal; Save to persist"
		}
	}
	st.syncFilenamePermissionEditors()
	st.refreshFilenamePermissionDraftInfo()
	st.syncFilenameExtensionEditors()
	st.refreshFilenameExtensionDraftInfo()
	st.syncFilenameSizeEditors()
	st.refreshFilenameSizeDraftInfo()
	if st.filenameDefaultIconClick.Clicked(gtx) {
		st.filenameDefaultIcon = nextFilenameIcon(st.filenameDefaultIcon)
		st.errText = ""
	}
	if st.filenamePermIconClick.Clicked(gtx) {
		st.filenamePermIcon = nextFilenameIcon(st.filenamePermIcon)
		st.errText = ""
		st.refreshFilenamePermissionDraftInfo()
	}
	if st.filenamePermApplyClick.Clicked(gtx) {
		action, err := st.upsertCurrentFilenamePermissionRule()
		if err != nil {
			st.errText = err.Error()
		} else {
			st.errText = ""
			st.filenamePermInfoText = "Pending " + strings.ToLower(action) + "; Save to persist"
		}
	}
	if st.filenamePermRemoveClick.Clicked(gtx) {
		if st.removeCurrentFilenamePermissionRule() {
			st.errText = ""
			st.filenamePermInfoText = "Pending removal; Save to persist"
		}
	}
	if st.filenameExtIconClick.Clicked(gtx) {
		st.filenameExtIcon = nextFilenameIcon(st.filenameExtIcon)
		st.errText = ""
		st.refreshFilenameExtensionDraftInfo()
	}
	if st.filenameExtApplyClick.Clicked(gtx) {
		action, err := st.upsertCurrentFilenameExtensionRule()
		if err != nil {
			st.errText = err.Error()
		} else {
			st.errText = ""
			st.filenameExtInfoText = "Pending " + strings.ToLower(action) + "; Save to persist"
		}
	}
	if st.filenameExtRemoveClick.Clicked(gtx) {
		if st.removeCurrentFilenameExtensionRule() {
			st.errText = ""
			st.filenameExtInfoText = "Pending removal; Save to persist"
		}
	}
	if st.filenameSizeIconClick.Clicked(gtx) {
		st.filenameSizeIcon = nextFilenameIcon(st.filenameSizeIcon)
		st.errText = ""
		st.refreshFilenameSizeDraftInfo()
	}
	if st.filenameSizeApplyClick.Clicked(gtx) {
		action, err := st.upsertCurrentFilenameSizeRule()
		if err != nil {
			st.errText = err.Error()
		} else {
			st.errText = ""
			st.filenameSizeInfoText = "Pending " + strings.ToLower(action) + "; Save to persist"
		}
	}
	if st.filenameSizeRemoveClick.Clicked(gtx) {
		if st.removeCurrentFilenameSizeRule() {
			st.errText = ""
			st.filenameSizeInfoText = "Pending removal; Save to persist"
		}
	}

	palette, filenameTheme, filenameColors, previewErr := st.previewFilenameTheme(ui.fmCfg)
	rowLabel := func(txt string, enabled bool) layout.Widget {
		return settingsViewerRowLabel(ui, th, txt, enabled)
	}
	currentAge := filenameAgeRuleKeyFromFields(st.filenameAgeOffsetEdit.Text(), st.filenameAgeUnit)
	_, currentAgeExists := st.filenameAgeRule(currentAge)
	ageAction := "Add"
	if currentAgeExists {
		ageAction = "Update"
	}
	currentPerm := filenamePermissionRuleKey(st.filenamePermEdit.Text(), st.filenamePermMatch)
	_, currentPermExists := st.filenamePermissionRule(currentPerm)
	permAction := "Add"
	if currentPermExists {
		permAction = "Update"
	}
	currentExt := filenameExtensionRuleKey(st.filenameExtEdit.Text())
	_, currentExtExists := st.filenameExtensionRule(currentExt)
	extAction := "Add"
	if currentExtExists {
		extAction = "Update"
	}
	currentSize := filenameSizeRuleKey(st.filenameSizeEdit.Text(), st.filenameSizeMatch)
	_, currentSizeExists := st.filenameSizeRule(currentSize)
	sizeAction := "Add"
	if currentSizeExists {
		sizeAction = "Update"
	}
	st.filenameRuleMode = normalizeFilenameRuleMode(st.filenameRuleMode)
	if st.filenameRuleMode != "permissions" {
		st.filenamePermPickerOpen = false
	}

	activeRuleEditor := func(gtx layout.Context) layout.Dimensions {
		switch st.filenameRuleMode {
		case "permissions":
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsFilenamePermissionPickerField(th, gtx, st)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsFilenamePermissionMatchTabs(th, gtx, st)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsFilenameColorValueField(th, gtx, st, "settings-filename-perm-text", &st.filenamePermTextEdit, &st.filenamePermTextPicker, "filename-perm-text", permSwatchGroups)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsFilenameIconCycleButton(th, gtx, &st.filenamePermIconClick, st.filenamePermIcon)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutTinyModeButton(th, gtx, ui.mainTypeface(), &st.filenamePermApplyClick, permAction, currentPermExists)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutTinyModeButton(th, gtx, ui.mainTypeface(), &st.filenamePermRemoveClick, "Remove", false)
						}),
					)
				}),
			)
		case "extensions":
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, gtx.Dp(unit.Dp(124)), func(gtx layout.Context) layout.Dimensions {
						ed := material.Editor(th, &st.filenameExtEdit, "go")
						ed.Font.Typeface = ui.mainTypeface()
						ed.TextSize = scaleModalThemeFontSize(th, 10)
						ed.Color = txtColor
						ed.HintColor = hintColor
						return ui.layoutEditorWithContextMenu(th, gtx, "settings-filename-ext", &st.filenameExtEdit, true, func(gtx layout.Context) layout.Dimensions {
							return layoutNeutralEditorBox(gtx, gtx.Focused(&st.filenameExtEdit), true, ed.Layout)
						})
					})
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsFilenameColorValueField(th, gtx, st, "settings-filename-ext-text", &st.filenameExtTextEdit, &st.filenameExtTextPicker, "filename-ext-text", extSwatchGroups)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsFilenameIconCycleButton(th, gtx, &st.filenameExtIconClick, st.filenameExtIcon)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutTinyModeButton(th, gtx, ui.mainTypeface(), &st.filenameExtApplyClick, extAction, currentExtExists)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutTinyModeButton(th, gtx, ui.mainTypeface(), &st.filenameExtRemoveClick, "Remove", false)
						}),
					)
				}),
			)
		case "sizes":
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, gtx.Dp(unit.Dp(104)), func(gtx layout.Context) layout.Dimensions {
						ed := material.Editor(th, &st.filenameSizeEdit, "10m")
						ed.Font.Typeface = ui.mainTypeface()
						ed.TextSize = scaleModalThemeFontSize(th, 10)
						ed.Color = txtColor
						ed.HintColor = hintColor
						return ui.layoutEditorWithContextMenu(th, gtx, "settings-filename-size", &st.filenameSizeEdit, true, func(gtx layout.Context) layout.Dimensions {
							return layoutNeutralEditorBox(gtx, gtx.Focused(&st.filenameSizeEdit), true, ed.Layout)
						})
					})
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsFilenameSizeMatchTabs(th, gtx, st)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsFilenameColorValueField(th, gtx, st, "settings-filename-size-text", &st.filenameSizeTextEdit, &st.filenameSizeTextPicker, "filename-size-text", sizeSwatchGroups)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsFilenameIconCycleButton(th, gtx, &st.filenameSizeIconClick, st.filenameSizeIcon)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutTinyModeButton(th, gtx, ui.mainTypeface(), &st.filenameSizeApplyClick, sizeAction, currentSizeExists)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutTinyModeButton(th, gtx, ui.mainTypeface(), &st.filenameSizeRemoveClick, "Remove", false)
						}),
					)
				}),
			)
		default:
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, gtx.Dp(unit.Dp(72)), func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(th, &st.filenameAgeOffsetEdit, "15")
								ed.Font.Typeface = ui.mainTypeface()
								ed.TextSize = scaleModalThemeFontSize(th, 10)
								ed.Color = txtColor
								ed.HintColor = hintColor
								return ui.layoutEditorWithContextMenu(th, gtx, "settings-filename-age-offset", &st.filenameAgeOffsetEdit, true, func(gtx layout.Context) layout.Dimensions {
									return layoutNeutralEditorBox(gtx, gtx.Focused(&st.filenameAgeOffsetEdit), true, ed.Layout)
								})
							})
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsFilenameAgeUnitPicker(th, gtx, st)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsFilenameColorValueField(th, gtx, st, "settings-filename-age-text", &st.filenameAgeTextEdit, &st.filenameAgeTextPicker, "filename-age-text", ageSwatchGroups)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutSettingsFilenameIconCycleButton(th, gtx, &st.filenameAgeIconClick, st.filenameAgeIcon)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutTinyModeButton(th, gtx, ui.mainTypeface(), &st.filenameAgeApplyClick, ageAction, currentAgeExists)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutTinyModeButton(th, gtx, ui.mainTypeface(), &st.filenameAgeRemoveClick, "Remove", false)
						}),
					)
				}),
			)
		}
	}

	activeRuleInfoText := func() string {
		switch st.filenameRuleMode {
		case "permissions":
			if st.filenamePermInfoText != "" {
				return st.filenamePermInfoText
			}
			return st.filenamePermissionNoticeText()
		case "extensions":
			if st.filenameExtInfoText != "" {
				return st.filenameExtInfoText
			}
			return st.filenameExtensionNoticeText()
		case "sizes":
			if st.filenameSizeInfoText != "" {
				return st.filenameSizeInfoText
			}
			return st.filenameSizeNoticeText()
		default:
			if st.filenameAgeInfoText != "" {
				return st.filenameAgeInfoText
			}
			return st.filenameAgeNoticeText()
		}
	}

	activeRuleList := func(gtx layout.Context) layout.Dimensions {
		switch st.filenameRuleMode {
		case "permissions":
			return ui.layoutSettingsFilenamePermissionList(th, gtx, st)
		case "extensions":
			return ui.layoutSettingsFilenameExtensionList(th, gtx, st)
		case "sizes":
			return ui.layoutSettingsFilenameSizeList(th, gtx, st)
		default:
			return ui.layoutSettingsFilenameAgeList(th, gtx, st)
		}
	}

	activeRuleNote := "Smaller offsets win. Add as many age overrides as you need; each one matches files not older than its offset."
	switch st.filenameRuleMode {
	case "permissions":
		activeRuleNote = "Exact matches the whole mode, while Has All only requires the selected bits. For example, 0111 with Has Any catches executables, and 0222 with Has None catches read-only files."
	case "extensions":
		activeRuleNote = "Extension filters match lowercase filename suffixes like go, md, or tar.gz."
	case "sizes":
		activeRuleNote = "Size filters use whole values like 0b, 4k, 10m, or 1g. At Least and At Most can be mixed."
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(rowLabel("Palette", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsColorScopeTabs(th, gtx, st)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(rowLabel("Default filename", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsFilenameColorValueField(th, gtx, st, "settings-filename-default", &st.filenameDefaultTextEdit, &st.filenameDefaultTextPicker, "filename-default-text", defaultSwatchGroups)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsFilenameIconCycleButton(th, gtx, &st.filenameDefaultIconClick, st.filenameDefaultIcon)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, "This starts from the active pane text color, and only becomes a filename-only override after you change it.")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, 9)
			lbl.Color = hintColor
			lbl.MaxLines = 2
			return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, lbl.Layout)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(rowLabel("Filters", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsFilenameRuleModeTabs(th, gtx, st)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return activeRuleEditor(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			infoText := activeRuleInfoText()
			if infoText == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, infoText)
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleModalThemeFontSize(th, 9)
				lbl.Color = hintColor
				lbl.MaxLines = 2
				return lbl.Layout(gtx)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, gtx.Dp(unit.Dp(156)), func(gtx layout.Context) layout.Dimensions {
				return activeRuleList(gtx)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, activeRuleNote)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, 9)
			lbl.Color = hintColor
			lbl.MaxLines = 2
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, "Rule order is age, extension, size, then permissions. Later matching categories override earlier ones, and custom filename colors stay active on hovered and selected rows.")
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
				lbl.MaxLines = 2
				return lbl.Layout(gtx)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(rowLabel("Preview", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			hostH := settingsColorsPreviewHostHeight(gtx)
			return fixedHeight(gtx, hostH, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsFilenamePreview(th, gtx, palette, filenameTheme, filenameColors)
			})
		}),
	)
}
