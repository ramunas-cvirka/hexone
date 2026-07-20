// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import "fmt"

func (st *settingsModalState) draftSignature() string {
	if st == nil {
		return ""
	}
	snapshot := struct {
		PaneColors, ViewerColors, FilenameDefaults string
		FilenameRules                              string
		ViewerFields, ViewerEntries                string
		Fonts, PaneAppearance, PaneBehavior        string
		PaneColumns, PaneDates, Terminal           string
		ConfigYAML                                 string
	}{
		PaneColors: fmt.Sprintf("%q", []string{
			st.colorPaneBackground, st.colorPaneText, st.colorHover, st.colorHoverText,
			st.colorPopupHover, st.colorPopupHoverText, st.colorSelection, st.colorSelectionText,
			st.colorSelectedFiles, st.colorSelectedFilesText, st.colorFocusedSelected,
			st.colorFocusedSelectedText, st.colorCurrentDir, st.colorCurrentDirText,
			st.colorScrollbarThumb, st.colorScrollbarTrack,
		}),
		ViewerColors: fmt.Sprintf("%q", []string{
			st.colorViewerBackground, st.colorViewerText, st.colorViewerSelection,
			st.colorViewerHexSelection, st.colorViewerHexOffsetText,
			st.colorViewerHexBytesText, st.colorViewerHexASCIIText,
		}),
		FilenameDefaults: fmt.Sprintf("%q", []string{
			st.filenameDefaultText, st.filenameDefaultIcon, st.filenameDefaultTarget,
		}),
		FilenameRules: fmt.Sprintf("%#v|%#v|%#v|%#v",
			st.filenameAgeEntries, st.filenamePermEntries, st.filenameExtEntries, st.filenameSizeEntries),
		ViewerFields: fmt.Sprintf("%q|%t|%t",
			[]string{st.viewCommandEdit.Text(), st.viewShellEdit.Text(), st.viewRemoteSearchCommandEdit.Text()},
			st.viewSmoothScrollingBool.Value, st.viewHideFunctionBarBool.Value),
		ViewerEntries: fmt.Sprintf("%#v|%#v|%#v", st.viewTargetEntries, st.viewRuleEntries, st.viewAssocEntries),
		Fonts: fmt.Sprintf("%q|%v", []string{
			st.interfaceFontFamily, st.paneFontFamily, st.tabsFontFamily,
			st.viewFontFamily, st.terminalFontFamily,
		}, []float32{
			st.interfaceFontSizeSp, st.paneFontSizeSp, st.tabsFontSizeSp,
			st.viewFontSizeSp, st.terminalFontSizeSp,
		}),
		PaneAppearance: fmt.Sprintf("%q", []string{
			st.paneFileWeight, st.paneDirWeight, st.panePermissionsWeight,
			st.paneSizeWeight, st.paneDateWeight,
		}),
		PaneBehavior: fmt.Sprintf("%t|%t|%t|%t|%t|%q",
			st.generalDimInactiveBool.Value,
			st.generalFavoritesNewTabBool.Value,
			st.generalWheelMovesSelection.Value,
			st.generalUseTrash.Value,
			st.generalDeleteWithoutConfirm.Value,
			st.generalCompletionSound),
		PaneColumns: fmt.Sprintf("%v|%v|%t|%q",
			st.paneFullChars, st.paneBriefChars, st.paneShowPermissions, st.panePermissionFormat),
		PaneDates:  fmt.Sprintf("%q|%q", st.paneDateFormatEdit.Text(), st.paneDateFallbackFormats),
		Terminal:   fmt.Sprintf("%t", st.terminalAcceleratedKeysBool.Value),
		ConfigYAML: st.configEdit.Text(),
	}
	return fmt.Sprintf("%#v", snapshot)
}

func (st *settingsModalState) dirty() bool {
	return st != nil && st.baselineDraft != "" && st.draftSignature() != st.baselineDraft
}

func (st *settingsModalState) saveLabel() string {
	if st != nil && st.dirty() {
		return "Save (*)"
	}
	return "Save"
}
