// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	resources "hexone"
	"hexone/fm"
	"strings"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
)

type settingsKeyboardFocus int

const (
	settingsKeyboardFocusNone settingsKeyboardFocus = iota
	settingsKeyboardFocusNav
	settingsKeyboardFocusGeneralDimInactive
	settingsKeyboardFocusGeneralPaneFont
	settingsKeyboardFocusGeneralPaneFontSize
	settingsKeyboardFocusGeneralViewFont
	settingsKeyboardFocusGeneralViewFontSize
	settingsKeyboardFocusViewerShell
	settingsKeyboardFocusViewerRemoteSearch
	settingsKeyboardFocusViewerSmoothScrolling
	settingsKeyboardFocusViewerHideFunctionBar
	settingsKeyboardFocusViewerTargetKey
	settingsKeyboardFocusViewerTargetBrowse
	settingsKeyboardFocusViewerTargetApply
	settingsKeyboardFocusViewerTargetCommand
	settingsKeyboardFocusViewerRulePattern
	settingsKeyboardFocusViewerRuleBrowse
	settingsKeyboardFocusViewerRuleApply
	settingsKeyboardFocusViewerRuleCommand
	settingsKeyboardFocusViewerCommand
	settingsKeyboardFocusAssociationsExt
	settingsKeyboardFocusAssociationsBrowse
	settingsKeyboardFocusAssociationsApply
	settingsKeyboardFocusAssociationsApp
	settingsKeyboardFocusAssociationsRemove
	settingsKeyboardFocusColorsScope
	settingsKeyboardFocusColorsCategory
	settingsKeyboardFocusColorsBgPicker
	settingsKeyboardFocusColorsValue
	settingsKeyboardFocusColorsTextPicker
	settingsKeyboardFocusColorsTextValue
	settingsKeyboardFocusFilenameDefaultTextPicker
	settingsKeyboardFocusFilenameDefaultText
	settingsKeyboardFocusFilenameDefaultIconPicker
	settingsKeyboardFocusFilenameRuleMode
	settingsKeyboardFocusFilenameAgeOffset
	settingsKeyboardFocusFilenameAgeUnit
	settingsKeyboardFocusFilenameAgeTextPicker
	settingsKeyboardFocusFilenameAgeText
	settingsKeyboardFocusFilenameAgeIconPicker
	settingsKeyboardFocusFilenameAgeApply
	settingsKeyboardFocusFilenameAgeRemove
	settingsKeyboardFocusFilenamePermMask
	settingsKeyboardFocusFilenamePermPicker
	settingsKeyboardFocusFilenamePermMatch
	settingsKeyboardFocusFilenamePermTextPicker
	settingsKeyboardFocusFilenamePermText
	settingsKeyboardFocusFilenamePermIconPicker
	settingsKeyboardFocusFilenamePermApply
	settingsKeyboardFocusFilenamePermRemove
	settingsKeyboardFocusFilenameExt
	settingsKeyboardFocusFilenameExtTextPicker
	settingsKeyboardFocusFilenameExtText
	settingsKeyboardFocusFilenameExtIconPicker
	settingsKeyboardFocusFilenameExtApply
	settingsKeyboardFocusFilenameExtRemove
	settingsKeyboardFocusFilenameSize
	settingsKeyboardFocusFilenameSizeMatch
	settingsKeyboardFocusFilenameSizeTextPicker
	settingsKeyboardFocusFilenameSizeText
	settingsKeyboardFocusFilenameSizeIconPicker
	settingsKeyboardFocusFilenameSizeApply
	settingsKeyboardFocusFilenameSizeRemove
	settingsKeyboardFocusConfigPath
	settingsKeyboardFocusConfigEditor
	settingsKeyboardFocusFooter
)

type settingsPopupKeyboardKind int

const (
	settingsPopupKeyboardNone settingsPopupKeyboardKind = iota
	settingsPopupKeyboardViewerTarget
	settingsPopupKeyboardViewerRule
	settingsPopupKeyboardViewerAssoc
	settingsPopupKeyboardColorCategory
	settingsPopupKeyboardColor
	settingsPopupKeyboardFilenameIcon
)

type settingsPopupKeyboardAction int

const (
	settingsPopupKeyboardActionRow settingsPopupKeyboardAction = iota
	settingsPopupKeyboardActionRemove
)

type settingsPopupKeyboardItem struct {
	kind   settingsPopupKeyboardKind
	index  int
	action settingsPopupKeyboardAction
}

type settingsFooterAction int

const (
	settingsFooterActionNone settingsFooterAction = iota
	settingsFooterActionCancel
	settingsFooterActionSave
)

func settingsChoiceStep(current string, keys []string, step int) string {
	if len(keys) == 0 {
		return ""
	}
	idx := -1
	for i, key := range keys {
		if key == current {
			idx = i
			break
		}
	}
	return keys[dialogWrappedIndex(idx, len(keys), step)]
}

func (st *settingsModalState) showColorTextField() bool {
	if st == nil {
		return false
	}
	return st.colorScope != "viewer" || settingsViewerCategoryHasText(st.colorCategory)
}

func (st *settingsModalState) isWidgetFocusTarget(target settingsKeyboardFocus) bool {
	switch target {
	case settingsKeyboardFocusGeneralDimInactive,
		settingsKeyboardFocusGeneralPaneFontSize,
		settingsKeyboardFocusGeneralViewFontSize,
		settingsKeyboardFocusViewerShell,
		settingsKeyboardFocusViewerRemoteSearch,
		settingsKeyboardFocusViewerSmoothScrolling,
		settingsKeyboardFocusViewerHideFunctionBar,
		settingsKeyboardFocusViewerTargetKey,
		settingsKeyboardFocusViewerTargetCommand,
		settingsKeyboardFocusViewerRulePattern,
		settingsKeyboardFocusViewerRuleCommand,
		settingsKeyboardFocusViewerCommand,
		settingsKeyboardFocusAssociationsExt,
		settingsKeyboardFocusAssociationsApp,
		settingsKeyboardFocusColorsValue,
		settingsKeyboardFocusColorsTextValue,
		settingsKeyboardFocusFilenameDefaultText,
		settingsKeyboardFocusFilenameAgeOffset,
		settingsKeyboardFocusFilenameAgeText,
		settingsKeyboardFocusFilenamePermMask,
		settingsKeyboardFocusFilenamePermText,
		settingsKeyboardFocusFilenameExt,
		settingsKeyboardFocusFilenameExtText,
		settingsKeyboardFocusFilenameSize,
		settingsKeyboardFocusFilenameSizeText,
		settingsKeyboardFocusConfigEditor:
		return true
	default:
		return false
	}
}

func (st *settingsModalState) applyPendingWidgetFocus(gtx layout.Context, target settingsKeyboardFocus, tag any) {
	if st == nil || st.focusPending != target || tag == nil {
		return
	}
	gtx.Execute(key.FocusCmd{Tag: tag})
	st.focusPending = settingsKeyboardFocusNone
}

func (st *settingsModalState) syncFocusedWidget(gtx layout.Context) {
	if st == nil {
		return
	}
	switch {
	case gtx.Focused(&st.generalDimInactiveBool):
		st.focus = settingsKeyboardFocusGeneralDimInactive
	case gtx.Focused(&st.paneFontSizeEdit):
		st.focus = settingsKeyboardFocusGeneralPaneFontSize
	case gtx.Focused(&st.viewFontSizeEdit):
		st.focus = settingsKeyboardFocusGeneralViewFontSize
	case gtx.Focused(&st.viewShellEdit):
		st.focus = settingsKeyboardFocusViewerShell
	case gtx.Focused(&st.viewRemoteSearchCommandEdit):
		st.focus = settingsKeyboardFocusViewerRemoteSearch
	case gtx.Focused(&st.viewSmoothScrollingBool):
		st.focus = settingsKeyboardFocusViewerSmoothScrolling
	case gtx.Focused(&st.viewHideFunctionBarBool):
		st.focus = settingsKeyboardFocusViewerHideFunctionBar
	case gtx.Focused(&st.viewTargetKeyEdit):
		st.focus = settingsKeyboardFocusViewerTargetKey
	case gtx.Focused(&st.viewTargetCommandEdit):
		st.focus = settingsKeyboardFocusViewerTargetCommand
	case gtx.Focused(&st.viewRulePatternEdit):
		st.focus = settingsKeyboardFocusViewerRulePattern
	case gtx.Focused(&st.viewRuleCommandEdit):
		st.focus = settingsKeyboardFocusViewerRuleCommand
	case gtx.Focused(&st.viewCommandEdit):
		st.focus = settingsKeyboardFocusViewerCommand
	case gtx.Focused(&st.viewAssocExtEdit):
		st.focus = settingsKeyboardFocusAssociationsExt
	case gtx.Focused(&st.viewAssocAppEdit):
		st.focus = settingsKeyboardFocusAssociationsApp
	case gtx.Focused(&st.colorValueEdit):
		st.focus = settingsKeyboardFocusColorsValue
	case gtx.Focused(&st.colorTextValueEdit):
		st.focus = settingsKeyboardFocusColorsTextValue
	case gtx.Focused(&st.filenameDefaultTextEdit):
		st.focus = settingsKeyboardFocusFilenameDefaultText
	case gtx.Focused(&st.filenameAgeOffsetEdit):
		st.focus = settingsKeyboardFocusFilenameAgeOffset
	case gtx.Focused(&st.filenameAgeTextEdit):
		st.focus = settingsKeyboardFocusFilenameAgeText
	case gtx.Focused(&st.filenamePermEdit):
		st.focus = settingsKeyboardFocusFilenamePermMask
	case gtx.Focused(&st.filenamePermTextEdit):
		st.focus = settingsKeyboardFocusFilenamePermText
	case gtx.Focused(&st.filenameExtEdit):
		st.focus = settingsKeyboardFocusFilenameExt
	case gtx.Focused(&st.filenameExtTextEdit):
		st.focus = settingsKeyboardFocusFilenameExtText
	case gtx.Focused(&st.filenameSizeEdit):
		st.focus = settingsKeyboardFocusFilenameSize
	case gtx.Focused(&st.filenameSizeTextEdit):
		st.focus = settingsKeyboardFocusFilenameSizeText
	case gtx.Focused(&st.configEdit):
		st.focus = settingsKeyboardFocusConfigEditor
	}
}

func (st *settingsModalState) normalizedFooterAction() settingsFooterAction {
	if st == nil {
		return settingsFooterActionSave
	}
	switch st.footerFocus {
	case settingsFooterActionCancel, settingsFooterActionSave:
		return st.footerFocus
	default:
		return settingsFooterActionSave
	}
}

func (st *settingsModalState) footerActionVisualState(target settingsFooterAction) dialogActionVisualState {
	if st == nil {
		return dialogActionVisualState{}
	}
	if st.focus == settingsKeyboardFocusFooter {
		active := st.normalizedFooterAction() == target
		return dialogActionVisualState{Focused: active, Default: active}
	}
	return dialogActionVisualState{Default: target == settingsFooterActionSave}
}

func (st *settingsModalState) focusOrder() []settingsKeyboardFocus {
	if st == nil {
		return nil
	}
	order := []settingsKeyboardFocus{settingsKeyboardFocusNav}
	switch st.activeTab {
	case "general":
		order = append(order, settingsKeyboardFocusGeneralDimInactive)
		if len(resources.BundledFontFamilies()) > 0 {
			order = append(order, settingsKeyboardFocusGeneralPaneFont)
		}
		order = append(order, settingsKeyboardFocusGeneralPaneFontSize)
		if len(resources.BundledFontFamilies()) > 0 {
			order = append(order, settingsKeyboardFocusGeneralViewFont)
		}
		order = append(order, settingsKeyboardFocusGeneralViewFontSize)
	case "viewer":
		order = append(order,
			settingsKeyboardFocusViewerShell,
			settingsKeyboardFocusViewerRemoteSearch,
			settingsKeyboardFocusViewerSmoothScrolling,
			settingsKeyboardFocusViewerHideFunctionBar,
			settingsKeyboardFocusViewerTargetKey,
			settingsKeyboardFocusViewerTargetBrowse,
			settingsKeyboardFocusViewerTargetApply,
			settingsKeyboardFocusViewerTargetCommand,
			settingsKeyboardFocusViewerRulePattern,
			settingsKeyboardFocusViewerRuleBrowse,
			settingsKeyboardFocusViewerRuleApply,
			settingsKeyboardFocusViewerRuleCommand,
			settingsKeyboardFocusViewerCommand,
		)
	case "associations":
		order = append(order,
			settingsKeyboardFocusAssociationsExt,
			settingsKeyboardFocusAssociationsBrowse,
			settingsKeyboardFocusAssociationsApply,
			settingsKeyboardFocusAssociationsApp,
		)
		if ext := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text()); ext != "" {
			if _, exists := st.viewerAssociation(ext); exists {
				order = append(order, settingsKeyboardFocusAssociationsRemove)
			}
		}
	case "colors":
		order = append(order, settingsKeyboardFocusColorsScope)
		if st.colorScope == "filenames" {
			order = append(order, settingsKeyboardFocusFilenameDefaultTextPicker, settingsKeyboardFocusFilenameDefaultText, settingsKeyboardFocusFilenameDefaultIconPicker, settingsKeyboardFocusFilenameRuleMode)
			switch normalizeFilenameRuleMode(st.filenameRuleMode) {
			case "permissions":
				order = append(order,
					settingsKeyboardFocusFilenamePermMask,
					settingsKeyboardFocusFilenamePermPicker,
					settingsKeyboardFocusFilenamePermMatch,
					settingsKeyboardFocusFilenamePermTextPicker,
					settingsKeyboardFocusFilenamePermText,
					settingsKeyboardFocusFilenamePermIconPicker,
					settingsKeyboardFocusFilenamePermApply,
					settingsKeyboardFocusFilenamePermRemove,
				)
			case "extensions":
				order = append(order,
					settingsKeyboardFocusFilenameExt,
					settingsKeyboardFocusFilenameExtTextPicker,
					settingsKeyboardFocusFilenameExtText,
					settingsKeyboardFocusFilenameExtIconPicker,
					settingsKeyboardFocusFilenameExtApply,
					settingsKeyboardFocusFilenameExtRemove,
				)
			case "sizes":
				order = append(order,
					settingsKeyboardFocusFilenameSize,
					settingsKeyboardFocusFilenameSizeMatch,
					settingsKeyboardFocusFilenameSizeTextPicker,
					settingsKeyboardFocusFilenameSizeText,
					settingsKeyboardFocusFilenameSizeIconPicker,
					settingsKeyboardFocusFilenameSizeApply,
					settingsKeyboardFocusFilenameSizeRemove,
				)
			default:
				order = append(order,
					settingsKeyboardFocusFilenameAgeOffset,
					settingsKeyboardFocusFilenameAgeUnit,
					settingsKeyboardFocusFilenameAgeTextPicker,
					settingsKeyboardFocusFilenameAgeText,
					settingsKeyboardFocusFilenameAgeIconPicker,
					settingsKeyboardFocusFilenameAgeApply,
					settingsKeyboardFocusFilenameAgeRemove,
				)
			}
		} else {
			order = append(order, settingsKeyboardFocusColorsCategory, settingsKeyboardFocusColorsBgPicker, settingsKeyboardFocusColorsValue)
			if st.showColorTextField() {
				order = append(order, settingsKeyboardFocusColorsTextPicker, settingsKeyboardFocusColorsTextValue)
			}
		}
	case "config":
		order = append(order, settingsKeyboardFocusConfigEditor)
	}
	order = append(order, settingsKeyboardFocusFooter)
	return order
}

func (st *settingsModalState) normalizeKeyboardFocus() bool {
	order := st.focusOrder()
	if len(order) == 0 {
		return false
	}
	for _, focus := range order {
		if focus == st.focus {
			return false
		}
	}
	st.focus = order[0]
	return true
}

func (st *settingsModalState) setKeyboardFocus(target settingsKeyboardFocus) bool {
	if st == nil {
		return false
	}
	order := st.focusOrder()
	if len(order) == 0 {
		return false
	}
	valid := false
	for _, focus := range order {
		if focus == target {
			valid = true
			break
		}
	}
	if !valid {
		target = order[0]
	}
	changed := st.focus != target
	st.focus = target
	if target == settingsKeyboardFocusFooter && st.footerFocus == settingsFooterActionNone {
		st.footerFocus = settingsFooterActionSave
	}
	if st.isWidgetFocusTarget(target) {
		st.focusPending = target
		st.keyFocus.wantFocus = false
	} else {
		st.focusPending = settingsKeyboardFocusNone
		st.keyFocus.focusKeyboard()
	}
	return changed
}

func (st *settingsModalState) stepKeyboardFocus(step int) bool {
	order := st.focusOrder()
	if len(order) == 0 {
		return false
	}
	current := -1
	for i, focus := range order {
		if focus == st.focus {
			current = i
			break
		}
	}
	next := order[dialogWrappedIndex(current, len(order), step)]
	return st.setKeyboardFocus(next)
}

func (st *settingsModalState) stepFooterAction(step int) bool {
	if st == nil {
		return false
	}
	order := []settingsFooterAction{settingsFooterActionCancel, settingsFooterActionSave}
	current := 0
	for i, action := range order {
		if action == st.normalizedFooterAction() {
			current = i
			break
		}
	}
	next := order[dialogWrappedIndex(current, len(order), step)]
	if next == st.normalizedFooterAction() {
		return false
	}
	st.footerFocus = next
	return true
}

func (st *settingsModalState) toggleFocusedCheckbox() bool {
	if st == nil {
		return false
	}
	switch st.focus {
	case settingsKeyboardFocusGeneralDimInactive:
		st.generalDimInactiveBool.Value = !st.generalDimInactiveBool.Value
		return true
	case settingsKeyboardFocusViewerSmoothScrolling:
		st.viewSmoothScrollingBool.Value = !st.viewSmoothScrollingBool.Value
		return true
	case settingsKeyboardFocusViewerHideFunctionBar:
		st.viewHideFunctionBarBool.Value = !st.viewHideFunctionBarBool.Value
		return true
	default:
		return false
	}
}

func (st *settingsModalState) activateFocusedAction(now time.Time) bool {
	if st == nil {
		return false
	}
	switch st.focus {
	case settingsKeyboardFocusViewerTargetBrowse:
		st.toggleViewerCommandTargetPicker()
		return true
	case settingsKeyboardFocusViewerTargetApply:
		action, err := st.upsertCurrentViewerCommandTarget()
		if err != nil {
			st.errText = err.Error()
			return true
		}
		st.errText = ""
		st.viewTargetPickOpen = false
		if action == "Update" {
			st.targetInfoText = "Pending change; Save to persist"
		} else {
			st.targetInfoText = "Pending add; Save to persist"
		}
		return true
	case settingsKeyboardFocusViewerRuleBrowse:
		st.toggleViewerCommandRulePicker()
		return true
	case settingsKeyboardFocusViewerRuleApply:
		action, err := st.upsertCurrentViewerCommandRule()
		if err != nil {
			st.errText = err.Error()
			return true
		}
		st.errText = ""
		st.viewRulePickOpen = false
		if action == "Update" {
			st.ruleInfoText = "Pending change; Save to persist"
		} else {
			st.ruleInfoText = "Pending add; Save to persist"
		}
		return true
	case settingsKeyboardFocusAssociationsBrowse:
		st.toggleViewerAssociationPicker()
		return true
	case settingsKeyboardFocusAssociationsApply:
		action, err := st.upsertCurrentViewerAssociation()
		if err != nil {
			st.errText = err.Error()
			return true
		}
		st.errText = ""
		st.viewAssocPickOpen = false
		if action == "Update" {
			st.assocInfoText = "Pending change; Save to persist"
		} else {
			st.assocInfoText = "Pending add; Save to persist"
		}
		return true
	case settingsKeyboardFocusAssociationsRemove:
		ext := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
		if ext == "" {
			st.errText = "association extension is required"
			return true
		}
		_, savedExists := st.viewerSavedAssociation(ext)
		if !st.removeCurrentViewerAssociation() {
			st.errText = "no association set for " + viewerAssociationDisplayExtension(ext)
			return true
		}
		st.errText = ""
		if savedExists {
			st.assocInfoText = "Pending removal; Save to persist"
		} else {
			st.assocInfoText = ""
		}
		st.viewAssocPickOpen = false
		return true
	case settingsKeyboardFocusColorsCategory:
		st.openColorCategoryPopup(now)
		return true
	case settingsKeyboardFocusColorsBgPicker:
		st.toggleColorPicker("background")
		return true
	case settingsKeyboardFocusColorsTextPicker:
		st.toggleColorPicker("text")
		return true
	case settingsKeyboardFocusFilenameDefaultTextPicker:
		st.toggleColorPicker("filename-default-text")
		return true
	case settingsKeyboardFocusFilenameDefaultIconPicker:
		st.toggleFilenameIconPicker("filename-default-icon")
		return true
	case settingsKeyboardFocusFilenameAgeTextPicker:
		st.toggleColorPicker("filename-age-text")
		return true
	case settingsKeyboardFocusFilenameAgeIconPicker:
		st.toggleFilenameIconPicker("filename-age-icon")
		return true
	case settingsKeyboardFocusFilenamePermPicker:
		st.toggleFilenamePermissionPicker()
		return true
	case settingsKeyboardFocusFilenamePermTextPicker:
		st.toggleColorPicker("filename-perm-text")
		return true
	case settingsKeyboardFocusFilenamePermIconPicker:
		st.toggleFilenameIconPicker("filename-perm-icon")
		return true
	case settingsKeyboardFocusFilenameAgeApply:
		action, err := st.upsertCurrentFilenameAgeRule()
		if err != nil {
			st.errText = err.Error()
			return true
		}
		st.errText = ""
		st.filenameAgeInfoText = "Pending " + strings.ToLower(action) + "; Save to persist"
		return true
	case settingsKeyboardFocusFilenameAgeRemove:
		if st.removeCurrentFilenameAgeRule() {
			st.errText = ""
			st.filenameAgeInfoText = "Pending removal; Save to persist"
		}
		return true
	case settingsKeyboardFocusFilenamePermApply:
		action, err := st.upsertCurrentFilenamePermissionRule()
		if err != nil {
			st.errText = err.Error()
			return true
		}
		st.errText = ""
		st.filenamePermInfoText = "Pending " + strings.ToLower(action) + "; Save to persist"
		return true
	case settingsKeyboardFocusFilenamePermRemove:
		if st.removeCurrentFilenamePermissionRule() {
			st.errText = ""
			st.filenamePermInfoText = "Pending removal; Save to persist"
		}
		return true
	case settingsKeyboardFocusFilenameExtTextPicker:
		st.toggleColorPicker("filename-ext-text")
		return true
	case settingsKeyboardFocusFilenameExtIconPicker:
		st.toggleFilenameIconPicker("filename-ext-icon")
		return true
	case settingsKeyboardFocusFilenameExtApply:
		action, err := st.upsertCurrentFilenameExtensionRule()
		if err != nil {
			st.errText = err.Error()
			return true
		}
		st.errText = ""
		st.filenameExtInfoText = "Pending " + strings.ToLower(action) + "; Save to persist"
		return true
	case settingsKeyboardFocusFilenameExtRemove:
		if st.removeCurrentFilenameExtensionRule() {
			st.errText = ""
			st.filenameExtInfoText = "Pending removal; Save to persist"
		}
		return true
	case settingsKeyboardFocusFilenameSizeTextPicker:
		st.toggleColorPicker("filename-size-text")
		return true
	case settingsKeyboardFocusFilenameSizeIconPicker:
		st.toggleFilenameIconPicker("filename-size-icon")
		return true
	case settingsKeyboardFocusFilenameSizeApply:
		action, err := st.upsertCurrentFilenameSizeRule()
		if err != nil {
			st.errText = err.Error()
			return true
		}
		st.errText = ""
		st.filenameSizeInfoText = "Pending " + strings.ToLower(action) + "; Save to persist"
		return true
	case settingsKeyboardFocusFilenameSizeRemove:
		if st.removeCurrentFilenameSizeRule() {
			st.errText = ""
			st.filenameSizeInfoText = "Pending removal; Save to persist"
		}
		return true
	}
	return false
}

func settingsColorPickerFocusTarget(target string) settingsKeyboardFocus {
	switch target {
	case "category":
		return settingsKeyboardFocusColorsCategory
	case "background":
		return settingsKeyboardFocusColorsBgPicker
	case "text":
		return settingsKeyboardFocusColorsTextPicker
	case "filename-default-text":
		return settingsKeyboardFocusFilenameDefaultTextPicker
	case "filename-age-text":
		return settingsKeyboardFocusFilenameAgeTextPicker
	case "filename-perm-text":
		return settingsKeyboardFocusFilenamePermTextPicker
	case "filename-ext-text":
		return settingsKeyboardFocusFilenameExtTextPicker
	case "filename-size-text":
		return settingsKeyboardFocusFilenameSizeTextPicker
	default:
		return settingsKeyboardFocusNone
	}
}

func settingsFilenameIconPickerFocusTarget(target string) settingsKeyboardFocus {
	switch target {
	case "filename-default-icon":
		return settingsKeyboardFocusFilenameDefaultIconPicker
	case "filename-age-icon":
		return settingsKeyboardFocusFilenameAgeIconPicker
	case "filename-perm-icon":
		return settingsKeyboardFocusFilenamePermIconPicker
	case "filename-ext-icon":
		return settingsKeyboardFocusFilenameExtIconPicker
	case "filename-size-icon":
		return settingsKeyboardFocusFilenameSizeIconPicker
	default:
		return settingsKeyboardFocusNone
	}
}

func (st *settingsModalState) resetPopupKeyboardFocus() {
	if st == nil {
		return
	}
	st.popupFocusKind = settingsPopupKeyboardNone
	st.popupFocusIndex = -1
	st.popupFocusAction = settingsPopupKeyboardActionRow
}

func (st *settingsModalState) setPopupKeyboardFocus(kind settingsPopupKeyboardKind, index int, action settingsPopupKeyboardAction) bool {
	if st == nil {
		return false
	}
	if kind == settingsPopupKeyboardNone || index < 0 {
		changed := st.popupFocusKind != settingsPopupKeyboardNone || st.popupFocusIndex >= 0
		st.resetPopupKeyboardFocus()
		return changed
	}
	if kind == settingsPopupKeyboardViewerAssoc {
		action = settingsPopupKeyboardActionRow
	}
	changed := st.popupFocusKind != kind || st.popupFocusIndex != index || st.popupFocusAction != action
	st.popupFocusKind = kind
	st.popupFocusIndex = index
	st.popupFocusAction = action
	switch kind {
	case settingsPopupKeyboardViewerTarget:
		st.viewTargetPickRemember = index
	case settingsPopupKeyboardViewerRule:
		st.viewRulePickRemember = index
	case settingsPopupKeyboardViewerAssoc:
		st.viewAssocPickRemember = index
	}
	st.scrollPopupKeyboardFocusIntoView()
	return changed
}

func (st *settingsModalState) popupKeyboardMatches(kind settingsPopupKeyboardKind, index int, action settingsPopupKeyboardAction) bool {
	return st != nil && st.popupFocusKind == kind && st.popupFocusIndex == index && st.popupFocusAction == action
}

func (st *settingsModalState) popupKeyboardItems(targetCount, ruleCount, assocCount, colorCategoryCount int) []settingsPopupKeyboardItem {
	if st == nil {
		return nil
	}
	switch {
	case st.colorCategoryOpen:
		items := make([]settingsPopupKeyboardItem, 0, colorCategoryCount)
		for i := 0; i < colorCategoryCount; i++ {
			items = append(items, settingsPopupKeyboardItem{kind: settingsPopupKeyboardColorCategory, index: i, action: settingsPopupKeyboardActionRow})
		}
		return items
	case st.viewTargetPickOpen:
		items := make([]settingsPopupKeyboardItem, 0, targetCount*2)
		for i := 0; i < targetCount; i++ {
			items = append(items,
				settingsPopupKeyboardItem{kind: settingsPopupKeyboardViewerTarget, index: i, action: settingsPopupKeyboardActionRow},
				settingsPopupKeyboardItem{kind: settingsPopupKeyboardViewerTarget, index: i, action: settingsPopupKeyboardActionRemove},
			)
		}
		return items
	case st.viewRulePickOpen:
		items := make([]settingsPopupKeyboardItem, 0, ruleCount*2)
		for i := 0; i < ruleCount; i++ {
			items = append(items,
				settingsPopupKeyboardItem{kind: settingsPopupKeyboardViewerRule, index: i, action: settingsPopupKeyboardActionRow},
				settingsPopupKeyboardItem{kind: settingsPopupKeyboardViewerRule, index: i, action: settingsPopupKeyboardActionRemove},
			)
		}
		return items
	case st.viewAssocPickOpen:
		items := make([]settingsPopupKeyboardItem, 0, assocCount)
		for i := 0; i < assocCount; i++ {
			items = append(items, settingsPopupKeyboardItem{kind: settingsPopupKeyboardViewerAssoc, index: i, action: settingsPopupKeyboardActionRow})
		}
		return items
	default:
		return nil
	}
}

func (st *settingsModalState) normalizePopupKeyboardFocus(targetCount, ruleCount, assocCount int, colorOptions []settingsColorOption, colorGroups []settingsColorSwatchGroup, iconOptions []filenameIconOption) bool {
	items := st.popupKeyboardItems(targetCount, ruleCount, assocCount, len(colorOptions))
	if len(items) == 0 {
		switch st.popupFocusKind {
		case settingsPopupKeyboardColorCategory:
			if st.colorCategoryOpen {
				if len(colorOptions) == 0 {
					return st.setPopupKeyboardFocus(settingsPopupKeyboardNone, -1, settingsPopupKeyboardActionRow)
				}
				if st.popupFocusIndex >= 0 && st.popupFocusIndex < len(colorOptions) {
					return false
				}
				kind, index, ok := st.popupKeyboardDefaultFocus(nil, nil, nil, colorOptions, colorGroups, iconOptions)
				if !ok {
					return st.setPopupKeyboardFocus(settingsPopupKeyboardNone, -1, settingsPopupKeyboardActionRow)
				}
				return st.setPopupKeyboardFocus(kind, index, settingsPopupKeyboardActionRow)
			}
		case settingsPopupKeyboardColor:
			if st.colorPickerOpen {
				if len(colorGroups) == 0 {
					return st.setPopupKeyboardFocus(settingsPopupKeyboardNone, -1, settingsPopupKeyboardActionRow)
				}
				if st.popupFocusIndex >= 0 && st.popupFocusIndex < settingsColorSwatchCount(colorGroups) {
					return false
				}
				kind, index, ok := st.popupKeyboardDefaultFocus(nil, nil, nil, colorOptions, colorGroups, iconOptions)
				if !ok {
					return st.setPopupKeyboardFocus(settingsPopupKeyboardNone, -1, settingsPopupKeyboardActionRow)
				}
				return st.setPopupKeyboardFocus(kind, index, settingsPopupKeyboardActionRow)
			}
		case settingsPopupKeyboardFilenameIcon:
			if st.filenameIconPickerOpen {
				if len(iconOptions) == 0 {
					return st.setPopupKeyboardFocus(settingsPopupKeyboardNone, -1, settingsPopupKeyboardActionRow)
				}
				if st.popupFocusIndex >= 0 && st.popupFocusIndex < len(iconOptions) {
					return false
				}
				kind, index, ok := st.popupKeyboardDefaultFocus(nil, nil, nil, colorOptions, colorGroups, iconOptions)
				if !ok {
					return st.setPopupKeyboardFocus(settingsPopupKeyboardNone, -1, settingsPopupKeyboardActionRow)
				}
				return st.setPopupKeyboardFocus(kind, index, settingsPopupKeyboardActionRow)
			}
		}
		return st.setPopupKeyboardFocus(settingsPopupKeyboardNone, -1, settingsPopupKeyboardActionRow)
	}
	if st.popupFocusKind == settingsPopupKeyboardNone {
		return false
	}
	for _, item := range items {
		if st.popupKeyboardMatches(item.kind, item.index, item.action) {
			return false
		}
	}
	last := items[len(items)-1]
	if st.popupFocusKind == last.kind && st.popupFocusIndex > last.index {
		return st.setPopupKeyboardFocus(last.kind, last.index, settingsPopupKeyboardActionRow)
	}
	first := items[0]
	return st.setPopupKeyboardFocus(first.kind, first.index, settingsPopupKeyboardActionRow)
}

func (st *settingsModalState) stepPopupKeyboardFocus(step, targetCount, ruleCount, assocCount, colorCategoryCount int) bool {
	items := st.popupKeyboardItems(targetCount, ruleCount, assocCount, colorCategoryCount)
	if len(items) == 0 {
		return false
	}
	if st.popupFocusKind == settingsPopupKeyboardNone {
		return false
	}
	current := -1
	for i, item := range items {
		if st.popupKeyboardMatches(item.kind, item.index, item.action) {
			current = i
			break
		}
	}
	next := items[dialogWrappedIndex(current, len(items), step)]
	return st.setPopupKeyboardFocus(next.kind, next.index, next.action)
}

func (st *settingsModalState) stepPopupKeyboardRow(step, targetCount, ruleCount, assocCount, colorCategoryCount int) bool {
	items := st.popupKeyboardItems(targetCount, ruleCount, assocCount, colorCategoryCount)
	if len(items) == 0 {
		return false
	}
	if st.popupFocusKind == settingsPopupKeyboardNone {
		return false
	}
	kind := items[0].kind
	count := 0
	switch kind {
	case settingsPopupKeyboardColorCategory:
		count = colorCategoryCount
	case settingsPopupKeyboardViewerTarget:
		count = targetCount
	case settingsPopupKeyboardViewerRule:
		count = ruleCount
	case settingsPopupKeyboardViewerAssoc:
		count = assocCount
	}
	if count <= 0 {
		return false
	}
	currentIndex := st.popupFocusIndex
	if currentIndex < 0 || currentIndex >= count || st.popupFocusKind != kind {
		currentIndex = 0
	}
	action := st.popupFocusAction
	if kind == settingsPopupKeyboardViewerAssoc {
		action = settingsPopupKeyboardActionRow
	}
	nextIndex := dialogWrappedIndex(currentIndex, count, step)
	return st.setPopupKeyboardFocus(kind, nextIndex, action)
}

func (st *settingsModalState) popupOwnerFocus() settingsKeyboardFocus {
	if st == nil {
		return settingsKeyboardFocusNone
	}
	switch {
	case st.colorCategoryOpen:
		return settingsKeyboardFocusColorsCategory
	case st.colorPickerOpen:
		return settingsColorPickerFocusTarget(st.colorPickerTarget)
	case st.filenameIconPickerOpen:
		return settingsFilenameIconPickerFocusTarget(st.filenameIconPickerTarget)
	case st.filenamePermPickerOpen:
		return settingsKeyboardFocusFilenamePermPicker
	case st.viewTargetPickOpen:
		return settingsKeyboardFocusViewerTargetBrowse
	case st.viewRulePickOpen:
		return settingsKeyboardFocusViewerRuleBrowse
	case st.viewAssocPickOpen:
		return settingsKeyboardFocusAssociationsBrowse
	default:
		return settingsKeyboardFocusNone
	}
}

func popupKeyboardSupportsRemove(kind settingsPopupKeyboardKind) bool {
	switch kind {
	case settingsPopupKeyboardViewerTarget, settingsPopupKeyboardViewerRule:
		return true
	default:
		return false
	}
}

func settingsPopupGridStep(index, dx, dy int, rowLengths []int) int {
	if index < 0 || len(rowLengths) == 0 {
		return -1
	}
	row := 0
	col := 0
	remaining := index
	for i, width := range rowLengths {
		if width <= 0 {
			continue
		}
		if remaining < width {
			row = i
			col = remaining
			break
		}
		remaining -= width
	}
	if dy != 0 {
		row += dy
		if row < 0 {
			row = 0
		}
		if row >= len(rowLengths) {
			row = len(rowLengths) - 1
		}
		if width := rowLengths[row]; width > 0 && col >= width {
			col = width - 1
		}
	}
	if dx != 0 {
		col += dx
		if col < 0 {
			col = 0
		}
		if width := rowLengths[row]; width > 0 && col >= width {
			col = width - 1
		}
	}
	next := 0
	for i := 0; i < row; i++ {
		next += rowLengths[i]
	}
	return next + col
}

func settingsColorPopupRowLengths(groups []settingsColorSwatchGroup) []int {
	rows := make([]int, 0, len(groups))
	for _, group := range groups {
		if len(group.hexes) > 0 {
			rows = append(rows, len(group.hexes))
		}
	}
	return rows
}

func settingsIconPopupRowLengths(options []filenameIconOption) []int {
	if len(options) == 0 {
		return nil
	}
	rows := make([]int, 0, (len(options)+3)/4)
	for start := 0; start < len(options); start += 4 {
		width := 4
		if remain := len(options) - start; remain < width {
			width = remain
		}
		rows = append(rows, width)
	}
	return rows
}

func (st *settingsModalState) stepPopupKeyboardMove(dx, dy int, targetCount, ruleCount, assocCount int, colorOptions []settingsColorOption, colorGroups []settingsColorSwatchGroup, iconOptions []filenameIconOption) bool {
	if st == nil || st.popupFocusKind == settingsPopupKeyboardNone {
		return false
	}
	switch st.popupFocusKind {
	case settingsPopupKeyboardColorCategory, settingsPopupKeyboardViewerTarget, settingsPopupKeyboardViewerRule, settingsPopupKeyboardViewerAssoc:
		if dx != 0 {
			return false
		}
		return st.stepPopupKeyboardRow(dy, targetCount, ruleCount, assocCount, len(colorOptions))
	case settingsPopupKeyboardColor:
		rows := settingsColorPopupRowLengths(colorGroups)
		if len(rows) == 0 {
			return false
		}
		next := settingsPopupGridStep(st.popupFocusIndex, dx, dy, rows)
		return st.setPopupKeyboardFocus(settingsPopupKeyboardColor, next, settingsPopupKeyboardActionRow)
	case settingsPopupKeyboardFilenameIcon:
		rows := settingsIconPopupRowLengths(iconOptions)
		if len(rows) == 0 {
			return false
		}
		next := settingsPopupGridStep(st.popupFocusIndex, dx, dy, rows)
		return st.setPopupKeyboardFocus(settingsPopupKeyboardFilenameIcon, next, settingsPopupKeyboardActionRow)
	default:
		return false
	}
}

func (st *settingsModalState) popupKeyboardDefaultFocus(targetEntries []viewerCommandTargetEntry, ruleEntries []fm.ViewerCommandRule, assocPrograms []viewerAssociationProgram, colorOptions []settingsColorOption, colorGroups []settingsColorSwatchGroup, iconOptions []filenameIconOption) (settingsPopupKeyboardKind, int, bool) {
	if st == nil {
		return settingsPopupKeyboardNone, -1, false
	}
	switch {
	case st.colorCategoryOpen:
		if len(colorOptions) == 0 {
			return settingsPopupKeyboardNone, -1, false
		}
		for i, opt := range colorOptions {
			if opt.key == st.colorCategory {
				return settingsPopupKeyboardColorCategory, i, true
			}
		}
		return settingsPopupKeyboardColorCategory, 0, true
	case st.colorPickerOpen:
		if len(colorGroups) == 0 {
			return settingsPopupKeyboardNone, -1, false
		}
		currentHex := fm.NormalizeHexColor(st.colorPickerHexValue(st.colorPickerTarget), fm.DefaultFilePaneSelectionHex)
		index := 0
		for _, group := range colorGroups {
			for _, hex := range group.hexes {
				if strings.EqualFold(currentHex, fm.NormalizeHexColor(hex, hex)) {
					return settingsPopupKeyboardColor, index, true
				}
				index++
			}
		}
		return settingsPopupKeyboardColor, 0, true
	case st.filenameIconPickerOpen:
		if len(iconOptions) == 0 {
			return settingsPopupKeyboardNone, -1, false
		}
		currentIcon := fm.NormalizeFilenameIcon(st.filenameIconPickerValue(st.filenameIconPickerTarget))
		for i, opt := range iconOptions {
			if opt.key == currentIcon {
				return settingsPopupKeyboardFilenameIcon, i, true
			}
		}
		return settingsPopupKeyboardFilenameIcon, 0, true
	case st.viewTargetPickOpen:
		if len(targetEntries) == 0 {
			return settingsPopupKeyboardNone, -1, false
		}
		if st.viewTargetPickRemember >= 0 && st.viewTargetPickRemember < len(targetEntries) {
			return settingsPopupKeyboardViewerTarget, st.viewTargetPickRemember, true
		}
		currentKey := normalizeViewerCommandTargetInput(st.viewTargetKeyEdit.Text())
		for i, entry := range targetEntries {
			if entry.Key == currentKey {
				return settingsPopupKeyboardViewerTarget, i, true
			}
		}
		return settingsPopupKeyboardViewerTarget, 0, true
	case st.viewRulePickOpen:
		if len(ruleEntries) == 0 {
			return settingsPopupKeyboardNone, -1, false
		}
		if st.viewRulePickRemember >= 0 && st.viewRulePickRemember < len(ruleEntries) {
			return settingsPopupKeyboardViewerRule, st.viewRulePickRemember, true
		}
		currentPattern := strings.TrimSpace(st.viewRulePatternEdit.Text())
		for i, rule := range ruleEntries {
			if rule.Pattern == currentPattern {
				return settingsPopupKeyboardViewerRule, i, true
			}
		}
		return settingsPopupKeyboardViewerRule, 0, true
	case st.viewAssocPickOpen:
		if len(assocPrograms) == 0 {
			return settingsPopupKeyboardNone, -1, false
		}
		if st.viewAssocPickRemember >= 0 && st.viewAssocPickRemember < len(assocPrograms) {
			return settingsPopupKeyboardViewerAssoc, st.viewAssocPickRemember, true
		}
		currentAppPath := fm.NormalizeViewerAssociationAppPath(st.viewAssocAppEdit.Text())
		for i, program := range assocPrograms {
			if strings.EqualFold(program.AppPath, currentAppPath) {
				return settingsPopupKeyboardViewerAssoc, i, true
			}
		}
		return settingsPopupKeyboardViewerAssoc, 0, true
	default:
		return settingsPopupKeyboardNone, -1, false
	}
}

func (st *settingsModalState) enterPopupKeyboardFocus(targetEntries []viewerCommandTargetEntry, ruleEntries []fm.ViewerCommandRule, assocPrograms []viewerAssociationProgram, colorOptions []settingsColorOption, colorGroups []settingsColorSwatchGroup, iconOptions []filenameIconOption) bool {
	kind, index, ok := st.popupKeyboardDefaultFocus(targetEntries, ruleEntries, assocPrograms, colorOptions, colorGroups, iconOptions)
	if !ok {
		return false
	}
	return st.setPopupKeyboardFocus(kind, index, settingsPopupKeyboardActionRow)
}

func (st *settingsModalState) activatePopupKeyboardFocus(targetEntries []viewerCommandTargetEntry, ruleEntries []fm.ViewerCommandRule, assocPrograms []viewerAssociationProgram, colorOptions []settingsColorOption, colorGroups []settingsColorSwatchGroup, iconOptions []filenameIconOption) bool {
	if st == nil {
		return false
	}
	switch st.popupFocusKind {
	case settingsPopupKeyboardColorCategory:
		if st.popupFocusIndex < 0 || st.popupFocusIndex >= len(colorOptions) {
			return false
		}
		st.setColorCategory(colorOptions[st.popupFocusIndex].key)
		st.errText = ""
		return true
	case settingsPopupKeyboardColor:
		if st.popupFocusIndex < 0 {
			return false
		}
		index := 0
		for _, group := range colorGroups {
			for _, hex := range group.hexes {
				if index == st.popupFocusIndex {
					st.setColorPickerHexValue(st.colorPickerTarget, hex)
					st.colorPickerOpen = false
					st.colorPickerTarget = ""
					st.errText = ""
					st.resetPopupKeyboardFocus()
					return true
				}
				index++
			}
		}
		return false
	case settingsPopupKeyboardFilenameIcon:
		if st.popupFocusIndex < 0 || st.popupFocusIndex >= len(iconOptions) {
			return false
		}
		target := st.filenameIconPickerTarget
		st.setFilenameIconPickerValue(target, iconOptions[st.popupFocusIndex].key)
		st.filenameIconPickerOpen = false
		st.filenameIconPickerTarget = ""
		st.errText = ""
		st.refreshFilenameIconPickerTarget(target)
		st.resetPopupKeyboardFocus()
		return true
	case settingsPopupKeyboardViewerTarget:
		if st.popupFocusIndex < 0 || st.popupFocusIndex >= len(targetEntries) {
			return false
		}
		entry := targetEntries[st.popupFocusIndex]
		if st.popupFocusAction == settingsPopupKeyboardActionRemove {
			if st.removeViewerCommandTarget(entry.Key) {
				st.errText = ""
				st.targetInfoText = "Pending removal; Save to persist"
			}
			st.setPopupKeyboardFocus(settingsPopupKeyboardViewerTarget, st.popupFocusIndex, settingsPopupKeyboardActionRow)
			return true
		}
		st.applyPickedViewerCommandTarget(entry)
		return true
	case settingsPopupKeyboardViewerRule:
		if st.popupFocusIndex < 0 || st.popupFocusIndex >= len(ruleEntries) {
			return false
		}
		rule := ruleEntries[st.popupFocusIndex]
		if st.popupFocusAction == settingsPopupKeyboardActionRemove {
			if st.removeViewerCommandRule(rule.Pattern) {
				st.errText = ""
				st.ruleInfoText = "Pending removal; Save to persist"
			}
			st.setPopupKeyboardFocus(settingsPopupKeyboardViewerRule, st.popupFocusIndex, settingsPopupKeyboardActionRow)
			return true
		}
		st.applyPickedViewerCommandRule(rule)
		return true
	case settingsPopupKeyboardViewerAssoc:
		if st.popupFocusIndex < 0 || st.popupFocusIndex >= len(assocPrograms) {
			return false
		}
		st.applyPickedViewerAssociation(assocPrograms[st.popupFocusIndex].AppPath)
		return true
	default:
		return false
	}
}

func (st *settingsModalState) stepPaneFontFamily(step int, families []resources.BundledFontFamily, now time.Time) bool {
	if st == nil || len(families) == 0 {
		return false
	}
	keys := make([]string, len(families))
	current := st.paneFontFamily
	if current == "" {
		current = families[0].Name
	}
	for i, family := range families {
		keys[i] = family.Name
	}
	next := settingsChoiceStep(current, keys, step)
	if next == "" || next == current {
		return false
	}
	st.paneFontPickerAnim.setValue(&st.paneFontFamily, next, now)
	st.paneFontPickerAnim.anim.setPulse(next, now)
	st.errText = ""
	return true
}

func (st *settingsModalState) stepViewFontFamily(step int, families []resources.BundledFontFamily, now time.Time) bool {
	if st == nil || len(families) == 0 {
		return false
	}
	keys := make([]string, len(families))
	current := st.viewFontFamily
	if current == "" {
		current = families[0].Name
	}
	for i, family := range families {
		keys[i] = family.Name
	}
	next := settingsChoiceStep(current, keys, step)
	if next == "" || next == current {
		return false
	}
	st.viewFontPickerAnim.setValue(&st.viewFontFamily, next, now)
	st.viewFontPickerAnim.anim.setPulse(next, now)
	st.errText = ""
	return true
}

func (st *settingsModalState) stepColorScope(step int, now time.Time) bool {
	if st == nil {
		return false
	}
	keys := []string{"panes", "viewer", "filenames"}
	current := st.colorScope
	if current == "" {
		current = "panes"
	}
	next := settingsChoiceStep(current, keys, step)
	if next == "" || next == current {
		return false
	}
	st.colorScopeAnim.anim.setPulse(next, now)
	st.setColorScope(next, now)
	return true
}

func (st *settingsModalState) stepFilenameRuleMode(step int, now time.Time) bool {
	if st == nil {
		return false
	}
	keys := []string{"age", "permissions", "extensions", "sizes"}
	current := normalizeFilenameRuleMode(st.filenameRuleMode)
	next := settingsChoiceStep(current, keys, step)
	if next == "" || next == current {
		return false
	}
	st.filenameRuleModeAnim.anim.setPulse(next, now)
	st.filenameRuleModeAnim.setValue(&st.filenameRuleMode, next, now)
	return true
}

func (st *settingsModalState) stepFilenameAgeUnit(step int, now time.Time) bool {
	if st == nil || len(filenameAgeUnitOptions) == 0 {
		return false
	}
	keys := make([]string, len(filenameAgeUnitOptions))
	current := normalizeFilenameAgeUnit(st.filenameAgeUnit)
	for i, opt := range filenameAgeUnitOptions {
		keys[i] = opt.key
	}
	next := settingsChoiceStep(current, keys, step)
	if next == "" || next == current {
		return false
	}
	st.filenameAgeUnitAnim.anim.setPulse(next, now)
	st.filenameAgeUnitAnim.setValue(&st.filenameAgeUnit, next, now)
	st.errText = ""
	return true
}

func (st *settingsModalState) stepFilenamePermMatch(step int, now time.Time) bool {
	if st == nil || len(filenamePermissionMatchOptions) == 0 {
		return false
	}
	keys := make([]string, len(filenamePermissionMatchOptions))
	current := normalizeFilenamePermissionMatch(st.filenamePermMatch)
	for i, opt := range filenamePermissionMatchOptions {
		keys[i] = opt.key
	}
	next := settingsChoiceStep(current, keys, step)
	if next == "" || next == current {
		return false
	}
	st.filenamePermMatchAnim.anim.setPulse(next, now)
	st.filenamePermMatchAnim.setValue(&st.filenamePermMatch, next, now)
	st.errText = ""
	return true
}

func (st *settingsModalState) stepFilenameSizeMatch(step int, now time.Time) bool {
	if st == nil || len(filenameSizeMatchOptions) == 0 {
		return false
	}
	keys := make([]string, len(filenameSizeMatchOptions))
	current := normalizeFilenameSizeMatch(st.filenameSizeMatch)
	for i, opt := range filenameSizeMatchOptions {
		keys[i] = opt.key
	}
	next := settingsChoiceStep(current, keys, step)
	if next == "" || next == current {
		return false
	}
	st.filenameSizeMatchAnim.anim.setPulse(next, now)
	st.filenameSizeMatchAnim.setValue(&st.filenameSizeMatch, next, now)
	st.errText = ""
	return true
}

func (st *settingsModalState) stepFocusedHorizontalGroup(step int, families []resources.BundledFontFamily, now time.Time) bool {
	if st == nil {
		return false
	}
	switch st.focus {
	case settingsKeyboardFocusGeneralPaneFont:
		return st.stepPaneFontFamily(step, families, now)
	case settingsKeyboardFocusGeneralViewFont:
		return st.stepViewFontFamily(step, families, now)
	case settingsKeyboardFocusColorsScope:
		return st.stepColorScope(step, now)
	case settingsKeyboardFocusFilenameRuleMode:
		return st.stepFilenameRuleMode(step, now)
	case settingsKeyboardFocusFilenameAgeUnit:
		return st.stepFilenameAgeUnit(step, now)
	case settingsKeyboardFocusFilenamePermMatch:
		return st.stepFilenamePermMatch(step, now)
	case settingsKeyboardFocusFilenameSizeMatch:
		return st.stepFilenameSizeMatch(step, now)
	case settingsKeyboardFocusFooter:
		return st.stepFooterAction(step)
	default:
		return false
	}
}
