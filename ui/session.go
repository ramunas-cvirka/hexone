package ui

import (
	"hexone/fm"
	"strings"
	"time"
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
		selectedPath := ""
		if sel := pane.selectedEntry(); sel != nil {
			selectedPath = sel.Path
		}
		s.Panes[i] = fm.SessionPane{
			Dir:          pane.dir,
			SelectedPath: selectedPath,
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
		now := time.Now()
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
			targetDir := strings.TrimSpace(paneState.Dir)
			if targetDir != "" {
				if err := pane.load(targetDir); err != nil {
					pane.setNotice(err.Error(), now)
					continue
				}
			}
			selectedPath := strings.TrimSpace(paneState.SelectedPath)
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
