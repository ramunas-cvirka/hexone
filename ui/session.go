// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"hexone/ui/widget/table"
	"strings"
)

func (ui *UI) SnapshotSession() *fm.SessionState {
	s := fm.DefaultSession()
	if ui == nil {
		return s
	}
	ui.ensureFilePaneTabs()

	switch ui.Tabs.Value {
	case "tab0", "tab1", "tab2", "tab3":
		s.ActiveTab = ui.Tabs.Value
	default:
		s.ActiveTab = "tab0"
	}
	s.ActivePane = ui.activeFilePane

	if len(ui.filePanes) == 0 {
		s.Panes = nil
		return s
	}

	s.Panes = make([]fm.SessionPane, len(ui.filePanes))
	for i, pane := range ui.filePanes {
		s.Panes[i] = sessionPaneFromFilePane(pane)
	}
	if len(ui.filePaneTabs) > 0 {
		s.FilePaneTabs = make([]fm.SessionPaneTabs, 0, len(ui.filePaneTabs))
		for _, set := range ui.filePaneTabs {
			if len(set.tabs) == 0 {
				continue
			}
			group := fm.SessionPaneTabs{
				Active: set.active,
				Tabs:   make([]fm.SessionPane, 0, len(set.tabs)),
			}
			for _, pane := range set.tabs {
				group.Tabs = append(group.Tabs, sessionPaneFromFilePane(pane))
			}
			s.FilePaneTabs = append(s.FilePaneTabs, group)
		}
	}
	return s
}

func (ui *UI) ApplySession(s *fm.SessionState) {
	if ui == nil || s == nil {
		return
	}

	switch strings.ToLower(strings.TrimSpace(s.ActiveTab)) {
	case "tab0", "tab1", "tab2", "tab3":
		ui.Tabs.Value = strings.ToLower(strings.TrimSpace(s.ActiveTab))
	}

	if len(s.FilePaneTabs) > 0 {
		ui.applySessionPaneTabs(s.FilePaneTabs)
	} else if len(s.Panes) > 0 {
		limit := len(s.Panes)
		if len(ui.filePanes) < limit {
			limit = len(ui.filePanes)
		}
		for i := 0; i < limit; i++ {
			pane := ui.filePanes[i]
			if pane == nil {
				continue
			}
			paneState := s.Panes[i]
			pane.sortKey = parseFileSortKey(paneState.SortKey)
			pane.sortDesc = paneState.SortDescending
			if err := pane.setFilter(paneState.Filter); err != nil {
				_ = pane.setFilter(filePaneDefaultFilter)
			}
			if pane.table != nil {
				switch strings.ToLower(strings.TrimSpace(paneState.Mode)) {
				case "brief":
					pane.table.SetMode(table.ModeBrief)
				default:
					pane.table.SetMode(table.ModeFull)
				}
			}
			targetDir := strings.TrimSpace(paneState.Dir)
			selectedPath := strings.TrimSpace(paneState.SelectedPath)
			if targetDir != "" {
				ui.requestPaneLoadWithSelection(i, targetDir, selectedPath, "", 0)
				continue
			}
			pane.applySort(selectedPath)
			if selectedPath != "" && pane.table != nil && pane.model != nil {
				if idx := pane.findEntryPathIndex(selectedPath); idx >= 0 {
					pane.table.SetSelected(idx, pane.model.Len(), false)
				}
			}
		}
	}

	active := s.ActivePane
	if active < 0 {
		active = 0
	}
	if active >= len(ui.filePanes) {
		active = len(ui.filePanes) - 1
	}
	if active >= 0 {
		ui.setActiveFilePane(active)
	}
}

func sessionPaneFromFilePane(pane *filePaneState) fm.SessionPane {
	if pane == nil {
		return fm.SessionPane{}
	}
	dir := pane.dir
	if !pane.remoteConnected() && pane.loading && strings.TrimSpace(pane.loadingDir) != "" {
		dir = pane.loadingDir
	}
	if pane.remoteConnected() && strings.TrimSpace(pane.localDirBeforeRemote) != "" {
		dir = pane.localDirBeforeRemote
	}
	selectedPath := ""
	if !pane.remoteConnected() {
		if sel := pane.selectedEntry(); sel != nil {
			selectedPath = sel.Path
		}
	}
	return fm.SessionPane{
		Dir:            dir,
		SelectedPath:   selectedPath,
		SortKey:        pane.sessionSortKey(),
		SortDescending: pane.sortDesc,
		Mode:           pane.sessionMode(),
		Filter:         pane.displayFilter(),
	}
}

func (ui *UI) applySessionPaneTabs(groups []fm.SessionPaneTabs) {
	if ui == nil || len(ui.filePanes) == 0 {
		return
	}
	sets := make([]filePaneTabSet, len(ui.filePanes))
	limit := len(groups)
	if limit > len(ui.filePanes) {
		limit = len(ui.filePanes)
	}
	for i := 0; i < len(ui.filePanes); i++ {
		if i >= limit || len(groups[i].Tabs) == 0 {
			if ui.filePanes[i] != nil {
				sets[i].tabs = []*filePaneState{ui.filePanes[i]}
				ui.installFilePaneHandlers(i, ui.filePanes[i])
			}
			continue
		}
		group := groups[i]
		sets[i].tabs = make([]*filePaneState, 0, len(group.Tabs))
		for _, paneState := range group.Tabs {
			targetDir := strings.TrimSpace(paneState.Dir)
			if targetDir == "" {
				targetDir = "."
			}
			pane := newFilePaneState(targetDir, ui.fmCfg)
			ui.installFilePaneHandlers(i, pane)
			applySessionPaneOptions(pane, paneState)
			selectedPath := strings.TrimSpace(paneState.SelectedPath)
			startLocalPaneLoad(pane, targetDir, selectedPath, "", 0)
			sets[i].tabs = append(sets[i].tabs, pane)
		}
		sets[i].active = clampTabIndex(group.Active, len(sets[i].tabs))
		ui.filePanes[i] = sets[i].tabs[sets[i].active]
	}
	ui.filePaneTabs = sets
	ui.ensureFilePaneTabs()
}

func applySessionPaneOptions(pane *filePaneState, paneState fm.SessionPane) {
	if pane == nil {
		return
	}
	pane.sortKey = parseFileSortKey(paneState.SortKey)
	pane.sortDesc = paneState.SortDescending
	if err := pane.setFilter(paneState.Filter); err != nil {
		_ = pane.setFilter(filePaneDefaultFilter)
	}
	if pane.table != nil {
		switch strings.ToLower(strings.TrimSpace(paneState.Mode)) {
		case "brief":
			pane.table.SetMode(table.ModeBrief)
		default:
			pane.table.SetMode(table.ModeFull)
		}
	}
}
