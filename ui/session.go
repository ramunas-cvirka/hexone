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

	switch ui.Tabs.Value {
	case "tab0", "tab1", "tab2":
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
		if pane == nil {
			continue
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

		s.Panes[i] = fm.SessionPane{
			Dir:            dir,
			SelectedPath:   selectedPath,
			SortKey:        pane.sessionSortKey(),
			SortDescending: pane.sortDesc,
			Mode:           pane.sessionMode(),
		}
	}
	return s
}

func (ui *UI) ApplySession(s *fm.SessionState) {
	if ui == nil || s == nil {
		return
	}

	switch strings.ToLower(strings.TrimSpace(s.ActiveTab)) {
	case "tab0", "tab1", "tab2":
		ui.Tabs.Value = strings.ToLower(strings.TrimSpace(s.ActiveTab))
	}

	if len(s.Panes) > 0 {
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
