// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"hexone/filesys"
	"image"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
)

func adaptiveInlineCellPadX(gtx layout.Context, requested unit.Dp, cellW int) unit.Dp {
	if requested <= 0 || cellW <= 0 {
		return 0
	}
	pad := requested
	const minContentPx = 8
	for pad > 0 && cellW-2*gtx.Dp(pad) < minContentPx {
		pad--
	}
	if pad < 0 {
		return 0
	}
	return pad
}

func inlineLeadingIconMetrics(cellH int) (size, gap int) {
	size = cellH - 6
	if size < 7 {
		size = 7
	}
	if size > 10 {
		size = 10
	}
	return size, 4
}

func inlineCanShowLeadingIcon(contentW, cellH int) bool {
	iconW, gapW := inlineLeadingIconMetrics(cellH)
	const minTextPx = 8
	return contentW >= iconW+gapW+minTextPx
}

func (ui *UI) inlineFileNameEditRect(gtx layout.Context, pane *filePaneState) (image.Rectangle, bool) {
	if ui == nil || pane == nil || pane.table == nil || pane.model == nil || !pane.inlineNameEditing {
		return image.Rectangle{}, false
	}
	row := pane.inlineNameRow
	total := pane.model.Len()
	cellRect, ok := pane.table.CellRect(row, 0, total)
	if !ok {
		return image.Rectangle{}, false
	}
	rect := cellRect
	rowPadY := gtx.Dp(pane.table.RowPadY)
	rect.Min.Y += rowPadY
	rect.Max.Y -= rowPadY
	if rect.Dy() < 1 || rect.Dx() < 1 {
		return image.Rectangle{}, false
	}

	if len(pane.table.Columns) > 0 {
		padX := gtx.Dp(adaptiveInlineCellPadX(gtx, pane.table.Columns[0].PadX, rect.Dx()))
		rect.Min.X += padX
		rect.Max.X -= padX
	}
	if rect.Dy() < 1 || rect.Dx() < 1 {
		return image.Rectangle{}, false
	}

	if icon, ok := pane.model.LeadingIcon(row, 0); ok && icon.Kind != 0 {
		contentW := rect.Dx()
		cellH := rect.Dy()
		if inlineCanShowLeadingIcon(contentW, cellH) {
			iconW, gapW := inlineLeadingIconMetrics(cellH)
			rect.Min.X += iconW + gapW
		}
	}
	if rect.Dx() < 8 || rect.Dy() < 1 {
		return image.Rectangle{}, false
	}
	return rect, true
}

func (ui *UI) startInlineFileNameEdit(idx, row int, now time.Time) bool {
	if idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.model == nil || pane.table == nil {
		return false
	}
	entry := pane.model.Entry(row)
	if entry == nil {
		return false
	}
	if entry.Kind == filesys.EntryParent || pane.archiveBrowsing() {
		return false
	}
	if pane.inlineNameEditing && pane.inlineNameRow == row {
		return true
	}
	if pane.inlineNameEditing {
		ui.finishInlineFileNameEdit(idx, now, true, true)
	}

	ui.setActiveFilePane(idx)
	ui.closeSortMenusExcept(idx)
	ui.closeDriveMenusExcept(idx)
	ui.closeFavoriteMenusExcept(idx)
	ui.closeContextMenusExcept(idx)
	pane.sortMenuOpen = false
	pane.closeDriveMenu()
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	pane.clearPendingPathNavigate()
	pane.stopPathEdit()
	pane.clearMarkedRows()
	prev := pane.table.Selected
	pane.table.SetSelected(row, pane.model.Len(), false)
	if pane.table.OnSelect != nil && prev != pane.table.Selected {
		pane.table.OnSelect(pane.table.Selected)
	}
	if !pane.beginInlineNameEdit(row) {
		pane.setNotice("cannot rename this entry", now)
		return false
	}
	return true
}

func (ui *UI) activatePendingInlineNameEdit(idx int, now time.Time) bool {
	if ui == nil || idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.table == nil || pane.inlineNamePendingRow < 0 {
		return false
	}
	if now.Before(pane.inlineNamePendingAt) {
		return false
	}

	row := pane.inlineNamePendingRow
	pane.clearPendingInlineNameEdit()
	if pane.inlineNameEditing || pane.model == nil || row < 0 || row >= pane.model.Len() {
		return false
	}
	if row != pane.table.Selected || pane.hasMarkedRows() {
		return false
	}
	return ui.startInlineFileNameEdit(idx, row, now)
}

func validateInlineFileNameTarget(ep copyEndpoint, srcPath, originalName, raw string) (string, string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", "", errors.New("name is empty")
	}
	if name == "." || name == ".." {
		return "", "", errors.New("invalid name")
	}
	if ep.isRemote() {
		if strings.Contains(name, "/") {
			return "", "", errors.New("name cannot contain '/'")
		}
	} else if strings.ContainsAny(name, `/\`) {
		return "", "", errors.New("name cannot contain path separators")
	}
	srcNorm, err := ep.normalizeSourcePath(srcPath)
	if err != nil {
		return "", "", err
	}
	if name == originalName {
		return srcNorm, "", nil
	}
	dstPath := ep.join(ep.dirName(srcNorm), name)
	if !ep.samePath(srcNorm, dstPath) {
		if existing, err := endpointLstat(ep, dstPath); err == nil && existing != nil {
			return "", "", errors.New("destination already exists")
		}
	}
	return srcNorm, dstPath, nil
}

func (ui *UI) finishInlineFileNameEdit(idx int, now time.Time, commit, forceClose bool) bool {
	if idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil || !pane.inlineNameEditing {
		return true
	}
	if !commit {
		pane.cancelInlineNameEdit()
		return true
	}

	row := pane.inlineNameRow
	srcPath := pane.inlineNamePath
	originalName := pane.inlineNameOriginal
	raw := pane.inlineNameEdit.Text()
	ep := copyEndpointFromPane(idx, pane)
	srcNorm, dstPath, err := validateInlineFileNameTarget(ep, srcPath, originalName, raw)
	if err != nil {
		pane.setNotice(err.Error(), now)
		if forceClose {
			pane.stopInlineNameEdit()
		}
		return false
	}
	if dstPath == "" || ep.samePath(srcNorm, dstPath) && strings.TrimSpace(raw) == originalName {
		pane.stopInlineNameEdit()
		return true
	}

	if ep.isRemote() {
		client := ep.remote.sftpClient()
		if client == nil {
			pane.setNotice("sftp session is not connected", now)
			if forceClose {
				pane.stopInlineNameEdit()
			}
			return false
		}
		if err := client.Rename(srcNorm, dstPath); err != nil {
			pane.setNotice(err.Error(), now)
			if forceClose {
				pane.stopInlineNameEdit()
			}
			return false
		}
	} else if err := os.Rename(srcNorm, dstPath); err != nil {
		pane.setNotice(err.Error(), now)
		if forceClose {
			pane.stopInlineNameEdit()
		}
		return false
	}

	pane.stopInlineNameEdit()
	ui.reloadPanesAfterInlineRename(idx, row, ep, srcNorm, dstPath)
	return true
}

func (ui *UI) reloadPanesAfterInlineRename(srcIdx, row int, ep copyEndpoint, srcPath, dstPath string) {
	if ui == nil {
		return
	}
	srcDir := ep.dirName(srcPath)
	dstDir := ep.dirName(dstPath)
	for i, pane := range ui.filePanes {
		if pane == nil || pane.model == nil || pane.table == nil {
			continue
		}
		paneEp := copyEndpointFromPane(i, pane)
		if paneEp.isRemote() != ep.isRemote() {
			continue
		}
		if ep.isRemote() {
			if pane.remote == nil || ep.remote == nil || !sameSSHRemoteTarget(pane.remote.setup, ep.remote.setup) {
				continue
			}
			curDir := path.Clean(pane.dir)
			if curDir != path.Clean(srcDir) && curDir != path.Clean(dstDir) {
				continue
			}
		} else {
			if pane.remoteConnected() {
				continue
			}
			curDir := filepath.Clean(pane.dir)
			if curDir != filepath.Clean(srcDir) && curDir != filepath.Clean(dstDir) {
				continue
			}
		}

		selectedPath := ""
		if sel := pane.selectedEntry(); sel != nil {
			selectedPath = sel.Path
		}
		primaryPath := ""
		if endpointSamePath(ep, dstDir, paneEp, pane.dir) {
			primaryPath = dstPath
		}
		secondaryPath := ""
		if selectedPath != "" &&
			!endpointSamePath(ep, srcPath, paneEp, selectedPath) &&
			!endpointSamePath(ep, dstPath, paneEp, selectedPath) {
			secondaryPath = selectedPath
		}

		fallbackRow := pane.table.Selected
		if i == srcIdx {
			fallbackRow = row
		}
		ui.requestPaneLoadWithSelection(i, pane.dir, primaryPath, secondaryPath, fallbackRow)
	}
}
