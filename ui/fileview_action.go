package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/platform"
	"path/filepath"
	"strings"
	"time"
)

var openFileWithConfiguredAppFunc = platform.OpenFileWithConfiguredApp
var openFileWithSystemAssociationFunc = platform.OpenFileWithSystemAssociation

func (ui *UI) startFileExternalOpenAction(idx int, now time.Time) {
	if ui == nil || idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return
	}
	entry := pane.selectedEntry()
	if entry == nil || entry.Path == "" {
		pane.setNotice("nothing selected to open externally", now)
		return
	}
	if pane.remoteConnected() {
		pane.setNotice("external open supports local files only", now)
		return
	}
	switch entry.Kind {
	case filesys.EntryDir, filesys.EntryParent:
		pane.setNotice("external open supports files only", now)
		return
	}
	assoc, ok := viewerAssociationForPath(entry.Path, ui.fmCfg)
	var err error
	if ok {
		err = openFileWithConfiguredAppFunc(assoc.AppPath, entry.Path)
	} else {
		err = openFileWithSystemAssociationFunc(entry.Path)
	}
	if err != nil {
		pane.setNotice("external open failed: "+err.Error(), now)
		return
	}
	ui.setActiveFilePane(idx)
	pane.stopPathEdit()
	pane.sortMenuOpen = false
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	ui.closeSortMenusExcept(idx)
	ui.closeFavoriteMenusExcept(idx)
	ui.closeContextMenusExcept(idx)
}

func viewerAssociationForPath(path string, cfg *fm.Config) (fm.ViewerAssociation, bool) {
	if cfg == nil || path == "" {
		return fm.ViewerAssociation{}, false
	}
	name := strings.ToLower(filepath.Base(path))
	var best fm.ViewerAssociation
	bestLen := -1
	assocs := fm.FlattenAssociationPrograms(cfg.Associations)
	if len(assocs) == 0 {
		assocs = fm.NormalizeViewerAssociations(cfg.Viewer.Associations)
	}
	for _, assoc := range assocs {
		if assoc.Extension == "" || assoc.AppPath == "" {
			continue
		}
		if strings.HasSuffix(name, assoc.Extension) && len(assoc.Extension) > bestLen {
			best = assoc
			bestLen = len(assoc.Extension)
		}
	}
	if bestLen < 0 {
		return fm.ViewerAssociation{}, false
	}
	return best, true
}
