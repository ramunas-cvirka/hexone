// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/platform"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
)

const (
	filePaneMenuActionOpen              = "open"
	filePaneMenuActionOpenOtherPane     = "open-other-pane"
	filePaneMenuActionRename            = "rename"
	filePaneMenuActionCopyDialog        = "copy-dialog"
	filePaneMenuActionMoveDialog        = "move-dialog"
	filePaneMenuActionDeleteDialog      = "delete-dialog"
	filePaneMenuActionCopyPath          = "copy-path"
	filePaneMenuActionPermissions       = "permissions"
	filePaneMenuActionNewFolder         = "new-folder"
	filePaneMenuActionNewFile           = "new-file"
	filePaneMenuActionExtractHere       = "extract-here"
	filePaneMenuActionRefresh           = "refresh"
	filePaneMenuActionOpenWithSystem    = "open-with-system"
	filePaneMenuActionSystemFileManager = "system-file-manager"
	filePaneMenuActionOpenWithAppPrefix = "open-with-app:"
	filePaneContextMenuRootWidthDp      = 180
	filePaneContextMenuCompactWidthDp   = 124
	filePaneContextMenuOpenWithWidthDp  = 176
	filePaneContextMenuItemDetailTextSp = 9
)

type fileContextMenuSpec struct {
	Key     string
	Title   string
	WidthDp int
	Items   []fileContextMenuItem
}

type fileContextMenuItem struct {
	ID        string
	Label     string
	Action    string
	Detail    string
	Disabled  bool
	Separator bool
	Submenu   *fileContextMenuSpec
}

type fileContextMenuActionResult struct {
	ClipboardText string
}

type fileOpenWithApp struct {
	AppPath  string
	Label    string
	MatchLen int
}

func fileContextMenuActionItem(id, label, action string) fileContextMenuItem {
	return fileContextMenuItem{ID: id, Label: label, Action: action}
}

func fileContextMenuSeparator(id string) fileContextMenuItem {
	return fileContextMenuItem{ID: id, Separator: true}
}

func fileContextMenuSubmenuItem(id, label string, submenu *fileContextMenuSpec) fileContextMenuItem {
	return fileContextMenuItem{ID: id, Label: label, Submenu: submenu}
}

func (item fileContextMenuItem) hasSubmenu() bool {
	return item.Submenu != nil && len(item.Submenu.Items) > 0
}

func fileContextMenuVisiblePanels(root fileContextMenuSpec, path []string) []fileContextMenuSpec {
	panels := []fileContextMenuSpec{root}
	cur := root
	for _, id := range path {
		item, ok := fileContextMenuItemByID(cur, id)
		if !ok || !item.hasSubmenu() {
			break
		}
		cur = *item.Submenu
		panels = append(panels, cur)
	}
	return panels
}

func normalizeFileContextMenuPath(root fileContextMenuSpec, path []string) []string {
	if len(path) == 0 {
		return nil
	}
	out := make([]string, 0, len(path))
	cur := root
	for _, id := range path {
		item, ok := fileContextMenuItemByID(cur, id)
		if !ok || !item.hasSubmenu() {
			break
		}
		out = append(out, id)
		cur = *item.Submenu
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func fileContextMenuItemByID(spec fileContextMenuSpec, id string) (fileContextMenuItem, bool) {
	for _, item := range spec.Items {
		if item.ID == id {
			return item, true
		}
	}
	return fileContextMenuItem{}, false
}

func replaceFileContextMenuPathLevel(path []string, level int, id string) []string {
	if level < 0 {
		level = 0
	}
	if level > len(path) {
		level = len(path)
	}
	next := append([]string(nil), path[:level]...)
	if strings.TrimSpace(id) != "" {
		next = append(next, id)
	}
	return next
}

func filePaneSystemFileManagerLabel(entry *filesys.Entry) string {
	name := strings.TrimSpace(systemFileManagerNameFunc())
	if name == "" {
		name = strings.TrimSpace(platform.SystemFileManagerName())
	}
	if name == "" {
		name = "File Manager"
	}
	if entry != nil && entry.Kind != filesys.EntryDir && entry.Kind != filesys.EntryParent {
		return "Reveal in " + name
	}
	return "Open in " + name
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (ui *UI) filePaneContextMenuSpec(idx int, pane *filePaneState) fileContextMenuSpec {
	entry := pane.contextMenuEntry()
	root := fileContextMenuSpec{
		Key:     "root",
		Title:   "This Folder",
		WidthDp: filePaneContextMenuRootWidthDp,
	}
	if pane == nil {
		return root
	}
	if entry == nil {
		if pane.archiveBrowsing() {
			root.Items = []fileContextMenuItem{
				fileContextMenuActionItem("refresh", "Refresh", filePaneMenuActionRefresh),
				fileContextMenuActionItem("copy-path", "Copy Path", filePaneMenuActionCopyPath),
			}
			return root
		}
		root.Items = []fileContextMenuItem{
			fileContextMenuSubmenuItem("new", "New", ui.filePaneNewMenuSpec()),
			fileContextMenuActionItem("refresh", "Refresh", filePaneMenuActionRefresh),
			fileContextMenuActionItem("copy-path", "Copy Path", filePaneMenuActionCopyPath),
		}
		if !pane.remoteConnected() {
			root.Items = append(root.Items[:2], append([]fileContextMenuItem{
				fileContextMenuActionItem("system-file-manager", filePaneSystemFileManagerLabel(nil), filePaneMenuActionSystemFileManager),
			}, root.Items[2:]...)...)
		}
		return root
	}

	title := entry.DisplayName
	if title == "" {
		title = entry.Name
	}
	if title == "" {
		title = entry.Path
	}
	if title == "" {
		title = root.Title
	}
	root.Title = title

	otherPaneAvailable := ui != nil && ui.contextMenuOtherPaneIndex(idx) >= 0
	localFileManagerAvailable := pane.writableLocalView()
	readOnlyArchive := pane.archiveBrowsing()
	fileOpsMenu := ui.filePaneFileOpsMenuSpec(readOnlyArchive)
	canExtractHere := !readOnlyArchive && entry != nil && entry.Kind == filesys.EntryFile && entry.CanEnter

	switch entry.Kind {
	case filesys.EntryParent:
		root.Items = append(root.Items, fileContextMenuActionItem("open", "Open", filePaneMenuActionOpen))
		if otherPaneAvailable {
			root.Items = append(root.Items, fileContextMenuActionItem("open-other", "Open in Other Pane", filePaneMenuActionOpenOtherPane))
		}
		if localFileManagerAvailable {
			root.Items = append(root.Items, fileContextMenuActionItem("system-file-manager", filePaneSystemFileManagerLabel(entry), filePaneMenuActionSystemFileManager))
		}
		root.Items = append(root.Items,
			fileContextMenuSeparator("sep-copy-path"),
			fileContextMenuActionItem("copy-path", "Copy Path", filePaneMenuActionCopyPath),
		)
	case filesys.EntryDir:
		root.Items = append(root.Items, fileContextMenuActionItem("open", "Open", filePaneMenuActionOpen))
		if otherPaneAvailable {
			root.Items = append(root.Items, fileContextMenuActionItem("open-other", "Open in Other Pane", filePaneMenuActionOpenOtherPane))
		}
		if localFileManagerAvailable {
			root.Items = append(root.Items, fileContextMenuActionItem("system-file-manager", filePaneSystemFileManagerLabel(entry), filePaneMenuActionSystemFileManager))
		}
		if readOnlyArchive {
			root.Items = append(root.Items,
				fileContextMenuSeparator("sep-edit"),
				fileContextMenuSubmenuItem("ops", "File Ops", fileOpsMenu),
				fileContextMenuSeparator("sep-meta"),
				fileContextMenuActionItem("copy-path", "Copy Path", filePaneMenuActionCopyPath),
			)
			return root
		}
		root.Items = append(root.Items,
			fileContextMenuSeparator("sep-edit"),
			fileContextMenuActionItem("rename", "Rename", filePaneMenuActionRename),
			fileContextMenuSubmenuItem("ops", "File Ops", fileOpsMenu),
			fileContextMenuSeparator("sep-meta"),
			fileContextMenuActionItem("copy-path", "Copy Path", filePaneMenuActionCopyPath),
			fileContextMenuActionItem("permissions", "Permissions..", filePaneMenuActionPermissions),
		)
	default:
		root.Items = append(root.Items, fileContextMenuActionItem("open", "Open", filePaneMenuActionOpen))
		if otherPaneAvailable {
			root.Items = append(root.Items, fileContextMenuActionItem("open-other", "Open in Other Pane", filePaneMenuActionOpenOtherPane))
		}
		if canExtractHere {
			root.Items = append(root.Items, fileContextMenuActionItem("extract-here", "Extract here", filePaneMenuActionExtractHere))
		}
		if menu := ui.filePaneOpenWithMenuSpec(pane, entry); menu != nil {
			root.Items = append(root.Items, fileContextMenuSubmenuItem("open-with", "Open With", menu))
		}
		if localFileManagerAvailable {
			root.Items = append(root.Items, fileContextMenuActionItem("system-file-manager", filePaneSystemFileManagerLabel(entry), filePaneMenuActionSystemFileManager))
		}
		if readOnlyArchive {
			root.Items = append(root.Items,
				fileContextMenuSeparator("sep-edit"),
				fileContextMenuSubmenuItem("ops", "File Ops", fileOpsMenu),
				fileContextMenuSeparator("sep-meta"),
				fileContextMenuActionItem("copy-path", "Copy Path", filePaneMenuActionCopyPath),
			)
			return root
		}
		root.Items = append(root.Items,
			fileContextMenuSeparator("sep-edit"),
			fileContextMenuActionItem("rename", "Rename", filePaneMenuActionRename),
			fileContextMenuSubmenuItem("ops", "File Ops", fileOpsMenu),
			fileContextMenuSeparator("sep-meta"),
			fileContextMenuActionItem("copy-path", "Copy Path", filePaneMenuActionCopyPath),
			fileContextMenuActionItem("permissions", "Permissions..", filePaneMenuActionPermissions),
		)
	}
	return root
}

func (ui *UI) filePaneFileOpsMenuSpec(readOnly bool) *fileContextMenuSpec {
	items := []fileContextMenuItem{
		fileContextMenuActionItem("copy", "Copy..", filePaneMenuActionCopyDialog),
	}
	if !readOnly {
		items = append(items,
			fileContextMenuActionItem("move", "Move..", filePaneMenuActionMoveDialog),
			fileContextMenuActionItem("delete", "Delete..", filePaneMenuActionDeleteDialog),
		)
	}
	return &fileContextMenuSpec{
		Key:     "ops",
		Title:   "File Ops",
		WidthDp: filePaneContextMenuCompactWidthDp,
		Items:   items,
	}
}

func (ui *UI) filePaneNewMenuSpec() *fileContextMenuSpec {
	return &fileContextMenuSpec{
		Key:     "new",
		Title:   "New",
		WidthDp: filePaneContextMenuCompactWidthDp,
		Items: []fileContextMenuItem{
			fileContextMenuActionItem("new-folder", "Folder..", filePaneMenuActionNewFolder),
			fileContextMenuActionItem("new-file", "File..", filePaneMenuActionNewFile),
		},
	}
}

func (ui *UI) filePaneOpenWithMenuSpec(pane *filePaneState, entry *filesys.Entry) *fileContextMenuSpec {
	if pane == nil || entry == nil || entry.Kind != filesys.EntryFile || pane.remoteConnected() || pane.archiveBrowsing() || filesys.ArchiveMemberPath(entry.Path) {
		return nil
	}
	items := []fileContextMenuItem{
		fileContextMenuActionItem("open-with-system", "System Default", filePaneMenuActionOpenWithSystem),
	}
	apps := fileOpenWithAppsForPath(entry.Path, ui.fmCfg)
	if len(apps) > 0 {
		items = append(items, fileContextMenuSeparator("sep-open-with-apps"))
		for i, app := range apps {
			items = append(items, fileContextMenuItem{
				ID:     "open-with-app-" + strconv.Itoa(i),
				Label:  app.Label,
				Action: filePaneMenuActionOpenWithAppPrefix + app.AppPath,
				Detail: app.AppPath,
			})
		}
	}
	return &fileContextMenuSpec{
		Key:     "open-with",
		Title:   "Open With",
		WidthDp: filePaneContextMenuOpenWithWidthDp,
		Items:   items,
	}
}

func fileOpenWithAppsForPath(filePath string, cfg *fm.Config) []fileOpenWithApp {
	if cfg == nil {
		return nil
	}
	name := strings.ToLower(filepath.Base(filePath))
	if strings.TrimSpace(name) == "" {
		return nil
	}

	flat := fm.FlattenAssociationPrograms(cfg.Associations)
	if len(flat) == 0 {
		flat = fm.NormalizeViewerAssociations(cfg.Viewer.Associations)
	}
	if len(flat) == 0 {
		return nil
	}

	byApp := make(map[string]*fileOpenWithApp, len(flat))
	for _, assoc := range flat {
		appPath := fm.NormalizeViewerAssociationAppPath(assoc.AppPath)
		if appPath == "" {
			continue
		}
		app := byApp[appPath]
		if app == nil {
			label := filepath.Base(appPath)
			if label == "" {
				label = appPath
			}
			app = &fileOpenWithApp{
				AppPath: appPath,
				Label:   label,
			}
			byApp[appPath] = app
		}
		if assoc.Extension != "" && strings.HasSuffix(name, assoc.Extension) && len(assoc.Extension) > app.MatchLen {
			app.MatchLen = len(assoc.Extension)
		}
	}
	if len(byApp) == 0 {
		return nil
	}

	apps := make([]fileOpenWithApp, 0, len(byApp))
	for _, app := range byApp {
		apps = append(apps, *app)
	}
	sort.Slice(apps, func(i, j int) bool {
		if apps[i].MatchLen != apps[j].MatchLen {
			return apps[i].MatchLen > apps[j].MatchLen
		}
		iBase := strings.ToLower(apps[i].Label)
		jBase := strings.ToLower(apps[j].Label)
		if iBase != jBase {
			return iBase < jBase
		}
		return strings.ToLower(apps[i].AppPath) < strings.ToLower(apps[j].AppPath)
	})
	return apps
}

func (ui *UI) handleFilePaneContextMenuAction(idx int, pane *filePaneState, row int, action string, now time.Time) fileContextMenuActionResult {
	result := fileContextMenuActionResult{}
	if pane == nil {
		return result
	}
	switch action {
	case filePaneMenuActionOpen:
		ui.openFilePaneContextTarget(idx, row, now)
	case filePaneMenuActionOpenOtherPane:
		ui.openFilePaneContextTargetInOtherPane(idx, row, now)
	case filePaneMenuActionRename:
		_ = ui.startInlineFileNameEdit(idx, row, now)
	case filePaneMenuActionCopyDialog:
		ui.startFileCopyDialog(idx, now)
	case filePaneMenuActionMoveDialog:
		ui.startFileMoveDialog(idx, now)
	case filePaneMenuActionDeleteDialog:
		ui.startFileDeleteDialog(idx, now)
	case filePaneMenuActionExtractHere:
		ui.startArchiveExtractHere(idx, row, now)
	case filePaneMenuActionCopyPath:
		result.ClipboardText = ui.filePaneContextCopyPath(idx, row)
		if result.ClipboardText != "" {
			pane.setNotice("path copied", now)
		}
	case filePaneMenuActionPermissions:
		_ = ui.startFilePermDialog(idx, row, now)
	case filePaneMenuActionNewFolder:
		ui.startFileCreateDialog(idx, now)
	case filePaneMenuActionNewFile:
		ui.startFileCreateDialog(idx, now)
		if ui.fileCreate != nil {
			ui.fileCreate.setKind(fileCreateKindFile, now)
		}
	case filePaneMenuActionRefresh:
		ui.refreshFilePane(idx, now)
	case filePaneMenuActionSystemFileManager:
		ui.startFileSystemFileManagerAction(idx, row, now)
	case filePaneMenuActionOpenWithSystem:
		ui.startFileExternalOpenWithAction(idx, row, "", now)
	default:
		if strings.HasPrefix(action, filePaneMenuActionOpenWithAppPrefix) {
			appPath := strings.TrimPrefix(action, filePaneMenuActionOpenWithAppPrefix)
			ui.startFileExternalOpenWithAction(idx, row, appPath, now)
			return result
		}
		pane.setNotice("action is not implemented yet", now)
	}
	return result
}

func (ui *UI) writeClipboardText(gtx layout.Context, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	gtx.Execute(clipboard.WriteCmd{
		Type: "application/text",
		Data: io.NopCloser(strings.NewReader(text)),
	})
}

func (ui *UI) refreshFilePane(idx int, now time.Time) {
	if ui == nil || idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return
	}
	primaryPath := ""
	fallbackRow := 0
	if pane.table != nil {
		fallbackRow = pane.table.Selected
	}
	if entry := pane.selectedEntry(); entry != nil {
		primaryPath = entry.Path
	}
	if !ui.requestPaneLoadWithSelection(idx, pane.dir, primaryPath, "", fallbackRow) {
		pane.setNotice("refresh failed", now)
	}
}

func (ui *UI) contextMenuOtherPaneIndex(idx int) int {
	if ui == nil {
		return -1
	}
	for i, pane := range ui.filePanes {
		if i == idx || pane == nil {
			continue
		}
		return i
	}
	return -1
}

func (ui *UI) openFilePaneContextTarget(idx, row int, now time.Time) {
	if ui == nil || idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.model == nil {
		return
	}
	entry := pane.model.Entry(row)
	if entry == nil {
		return
	}
	if entry.CanEnter && !pane.remoteConnected() {
		ui.queueFilePaneOpen(idx, row)
		return
	}
	switch entry.Kind {
	case filesys.EntryDir, filesys.EntryParent:
		ui.queueFilePaneOpen(idx, row)
	case filesys.EntryFile, filesys.EntryBroken:
		if pane.remoteConnected() || pane.archiveBrowsing() || filesys.ArchiveMemberPath(entry.Path) {
			ui.startFileViewer(idx, now)
			return
		}
		ui.startFileExternalOpenAction(idx, now)
	}
}

func (ui *UI) openFilePaneContextTargetInOtherPane(idx, row int, now time.Time) {
	if ui == nil || idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.model == nil {
		return
	}
	entry := pane.model.Entry(row)
	if entry == nil {
		return
	}
	otherIdx := ui.contextMenuOtherPaneIndex(idx)
	if otherIdx < 0 {
		pane.setNotice("no other pane available", now)
		return
	}

	targetDir := entry.Path
	primaryPath := ""
	switch entry.Kind {
	case filesys.EntryParent:
		primaryPath = pane.dir
	case filesys.EntryDir:
		// targetDir already points at the directory to open.
	default:
		if entry.CanEnter && !pane.remoteConnected() {
			break
		}
		primaryPath = entry.Path
		if pane.remoteConnected() {
			targetDir = path.Dir(entry.Path)
		} else {
			targetDir = filepath.Dir(entry.Path)
		}
	}

	if pane.remoteConnected() && pane.remote != nil {
		other := ui.filePanes[otherIdx]
		if other == nil || !other.remoteConnected() || other.remote == nil || !sameSSHRemoteTarget(other.remote.setup, pane.remote.setup) {
			if err := ui.connectPaneSSH(otherIdx, pane.remote.setup, targetDir, now); err != nil {
				pane.setNotice("open in other pane failed: "+err.Error(), now)
				return
			}
		}
	} else if other := ui.filePanes[otherIdx]; other != nil && other.remoteConnected() {
		ui.disconnectPaneSSH(otherIdx, now)
	}
	if !ui.requestPaneLoadWithSelection(otherIdx, targetDir, primaryPath, "", 0) {
		pane.setNotice("open in other pane failed", now)
	}
}

func (ui *UI) filePaneContextCopyPath(idx, row int) string {
	if ui == nil || idx < 0 || idx >= len(ui.filePanes) {
		return ""
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return ""
	}
	target := strings.TrimSpace(pane.dir)
	if pane.remoteConnected() && target == "" {
		target = "/"
	}
	if row >= 0 && pane.model != nil {
		if entry := pane.model.Entry(row); entry != nil && strings.TrimSpace(entry.Path) != "" {
			target = entry.Path
		}
	}
	if target == "" {
		return ""
	}
	if !pane.remoteConnected() || pane.remote == nil {
		return target
	}
	return formatRemoteFavoriteLocation(remoteFavoriteLocation{
		User: pane.remote.setup.User,
		Host: pane.remote.setup.Host,
		Port: pane.remote.setup.Port,
		Dir:  normalizeRemoteFavoriteDir(target),
	})
}
