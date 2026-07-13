// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"fmt"
	"hexone/filesys"
	"hexone/fm"
	"image"
	"image/color"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type fileMoveState struct {
	pane int
	row  int

	sources []fileMoveSource
	srcPath string
	srcName string
	srcInfo fileCopyPathInfo

	dstRaw  string
	dstPath string
	dstInfo fileCopyPathInfo

	endpoint copyEndpoint
	remote   *paneSSHSession

	dstEdit     widget.Editor
	dstEditWant bool

	backdropClick widget.Clickable
	closeClick    widget.Clickable
	confirmClick  widget.Clickable
	cancelClick   widget.Clickable

	running bool
	lastErr string

	doneCh      chan error
	actionsAnim segmentedAnimState
	keyFocus    dialogKeyboardFocusState
	focus       fileMoveDialogFocus
	actionFocus fileMoveDialogAction
}

type fileMoveDialogFocus uint8

const (
	fileMoveDialogFocusDestination fileMoveDialogFocus = iota
	fileMoveDialogFocusActions
)

type fileMoveDialogAction uint8

const (
	fileMoveDialogActionCancel fileMoveDialogAction = iota
	fileMoveDialogActionConfirm
)

type fileMoveSource struct {
	Path string
	Name string
}

type fileMovePlan struct {
	srcPath string
	dstPath string
}

const fileMoveSuccessNoticeDur = 1200 * time.Millisecond

func (ui *UI) startFileMoveDialog(idx int, now time.Time) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.model == nil || pane.table == nil {
		return
	}
	if pane.archiveBrowsing() {
		pane.setNotice("cannot rename or move files inside an archive", now)
		return
	}
	row := pane.table.Selected
	selected := pane.selectedEntriesForAction()
	if len(selected) == 0 {
		if entry := pane.selectedEntry(); entry != nil && entry.Kind == filesys.EntryParent {
			pane.setNotice("cannot rename/move parent entry", now)
			return
		}
		pane.setNotice("nothing selected to rename/move", now)
		return
	}
	entry := selected[0]

	ui.setActiveFilePane(idx)
	pane.stopPathEdit()
	pane.sortMenuOpen = false
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	ui.closeSortMenusExcept(idx)
	ui.closeFavoriteMenusExcept(idx)
	ui.closeContextMenusExcept(idx)

	var remote *paneSSHSession
	if pane.remoteConnected() {
		remote = pane.remote.clone()
		if remote == nil {
			pane.setNotice("remote session is not connected", now)
			return
		}
	}

	var (
		srcInfo fileCopyPathInfo
		err     error
	)
	if remote != nil {
		srcInfo, err = buildCopyPathInfoRemote(remote, entry.Path)
	} else {
		srcInfo, err = buildCopyPathInfo(entry.Path)
	}
	if err != nil {
		if remote != nil {
			remote.close()
		}
		pane.setNotice(err.Error(), now)
		return
	}
	sources := make([]fileMoveSource, 0, len(selected))
	for _, item := range selected {
		sources = append(sources, fileMoveSource{
			Path: item.Path,
			Name: item.DisplayName,
		})
	}

	st := &fileMoveState{
		pane:     idx,
		row:      row,
		sources:  sources,
		srcPath:  entry.Path,
		srcName:  entry.DisplayName,
		srcInfo:  srcInfo,
		remote:   remote,
		endpoint: copyEndpoint{pane: idx, remote: remote, dir: strings.TrimSpace(pane.dir)},
	}
	targetDir := ui.fileMoveDefaultTargetDir(idx, st.endpoint)
	targetDefault, err := resolveFileOpTargetPath(st.endpoint, targetDir, st.endpoint.baseName(entry.Path))
	if err != nil || strings.TrimSpace(targetDefault) == "" {
		targetDefault = entry.Path
	}
	if len(sources) > 1 {
		targetDefault = strings.TrimSpace(targetDir)
		if targetDefault == "" {
			if st.endpoint.isRemote() {
				targetDefault = "/"
			} else {
				targetDefault = "."
			}
		}
	}

	st.dstEdit.SingleLine = true
	st.dstEdit.Submit = true
	st.dstEdit.SetText(targetDefault)
	st.dstEdit.SetCaret(st.dstEdit.Len(), st.dstEdit.Len())
	st.dstEditWant = true
	st.focus = fileMoveDialogFocusDestination
	st.actionFocus = fileMoveDialogActionConfirm
	st.refreshPreview()

	ui.fileMove = st
	ui.rep.active = false
	ui.rep.pane = -1
	ui.clearFileMoveHotkeyHold()
}

func (ui *UI) clearFileMoveHotkeyHold() {
	if ui == nil || ui.held == nil {
		return
	}
	ui.held[fileActionKey(fileActionRenameMove)] = false
}

func (st *fileMoveState) destinationEditable() bool {
	return st != nil && !st.running
}

func (st *fileMoveState) focusOrder() []fileMoveDialogFocus {
	if st == nil {
		return nil
	}
	order := make([]fileMoveDialogFocus, 0, 3)
	if st.destinationEditable() {
		order = append(order, fileMoveDialogFocusDestination)
	}
	order = append(order, fileMoveDialogFocusActions)
	return order
}

func (st *fileMoveState) syncEditorFocus(gtx layout.Context) {
	if st == nil || !st.destinationEditable() {
		return
	}
	if gtx.Focused(&st.dstEdit) {
		st.focus = fileMoveDialogFocusDestination
	}
}

func (st *fileMoveState) setFocus(target fileMoveDialogFocus) bool {
	if st == nil {
		return false
	}
	if target == fileMoveDialogFocusDestination && !st.destinationEditable() {
		return false
	}
	changed := st.focus != target
	st.focus = target
	switch target {
	case fileMoveDialogFocusDestination:
		st.dstEditWant = true
	default:
		st.dstEditWant = false
		st.keyFocus.focusKeyboard()
	}
	return changed
}

func (st *fileMoveState) stepFocus(step int) bool {
	order := st.focusOrder()
	if len(order) == 0 {
		return false
	}
	current := -1
	for i, target := range order {
		if target == st.focus {
			current = i
			break
		}
	}
	return st.setFocus(order[dialogWrappedIndex(current, len(order), step)])
}

func (st *fileMoveState) stepAction(step int) bool {
	if st == nil {
		return false
	}
	order := []fileMoveDialogAction{fileMoveDialogActionCancel, fileMoveDialogActionConfirm}
	current := 0
	for i, action := range order {
		if action == st.actionFocus {
			current = i
			break
		}
	}
	next := order[dialogWrappedIndex(current, len(order), step)]
	if next == st.actionFocus {
		return false
	}
	st.actionFocus = next
	return true
}

func (st *fileMoveState) actionVisualState(target fileMoveDialogAction) dialogActionVisualState {
	if st == nil || st.running {
		return dialogActionVisualState{}
	}
	if st.focus == fileMoveDialogFocusActions {
		active := st.actionFocus == target
		return dialogActionVisualState{Focused: active, Default: active}
	}
	return dialogActionVisualState{Default: target == fileMoveDialogActionConfirm}
}

func (st *fileMoveState) multiSource() bool {
	return st != nil && len(st.sources) > 1
}

func (st *fileMoveState) sourceCount() int {
	if st == nil {
		return 0
	}
	if len(st.sources) > 0 {
		return len(st.sources)
	}
	if strings.TrimSpace(st.srcPath) != "" {
		return 1
	}
	return 0
}

func (st *fileMoveState) sourceSummary() string {
	count := st.sourceCount()
	if count <= 1 {
		return st.srcPath
	}
	return fmt.Sprintf("%d items selected", count)
}

func (st *fileMoveState) sourceOperationSummary() string {
	return fileOpSourceCountText(st.sourceCount())
}

func (st *fileMoveState) sourceLocation() string {
	if st == nil {
		return ""
	}
	if st.multiSource() {
		return st.sourceSummary()
	}
	if len(st.sources) > 0 {
		return fileOpPreviewLabel(st.sources[0].Name, st.sources[0].Path)
	}
	return st.endpoint.baseName(st.srcPath)
}

func (st *fileMoveState) refreshPreview() {
	if st == nil {
		return
	}
	raw := strings.TrimSpace(st.dstEdit.Text())
	st.dstRaw = raw
	st.dstPath = ""
	st.dstInfo = fileCopyPathInfo{}
	if raw == "" {
		return
	}
	if st.multiSource() {
		dstDir, dstInfo, err := inspectExistingMoveDestinationDir(st.endpoint, raw)
		if err != nil {
			return
		}
		st.dstPath = dstDir
		st.dstInfo = dstInfo
		return
	}

	dst, err := st.effectiveDestinationPath(raw)
	if err != nil {
		return
	}
	st.dstPath = dst

	info, err := endpointLstat(st.endpoint, dst)
	if err != nil {
		return
	}
	st.dstInfo = fileCopyPathInfo{
		Path:    dst,
		Exists:  true,
		IsDir:   info.IsDir(),
		ModTime: info.ModTime(),
	}
	if info.Mode().IsRegular() {
		st.dstInfo.Size = info.Size()
	}
}

func (ui *UI) submitFileMoveDialog(now time.Time) {
	st := ui.fileMove
	if st == nil || st.running {
		return
	}

	raw := strings.TrimSpace(st.dstEdit.Text())
	if raw == "" {
		st.lastErr = "destination path is empty"
		return
	}
	if st.multiSource() {
		dstDir, dstInfo, plans, err := st.buildMovePlans(raw)
		if err != nil {
			st.lastErr = err.Error()
			return
		}
		st.dstRaw = raw
		st.dstPath = dstDir
		st.dstInfo = dstInfo
		st.lastErr = ""
		st.running = true
		st.doneCh = make(chan error, 1)

		remote := st.remote
		doneCh := st.doneCh
		go func(plans []fileMovePlan) {
			for _, plan := range plans {
				if remote != nil {
					client := remote.sftpClient()
					if client == nil {
						doneCh <- errors.New("sftp session is not connected")
						return
					}
					if err := client.Rename(plan.srcPath, plan.dstPath); err != nil {
						doneCh <- err
						return
					}
					continue
				}
				if err := os.Rename(plan.srcPath, plan.dstPath); err != nil {
					doneCh <- err
					return
				}
			}
			doneCh <- nil
		}(plans)

		_ = now
		return
	}
	dst, err := st.effectiveDestinationPath(raw)
	if err != nil {
		st.lastErr = err.Error()
		return
	}
	if st.endpoint.samePath(st.srcPath, dst) {
		st.lastErr = "source and destination are the same"
		return
	}
	dstDir := st.endpoint.dirName(dst)
	dstDirInfo, err := endpointStat(st.endpoint, dstDir)
	if err != nil {
		st.lastErr = "destination directory does not exist"
		return
	}
	if dstDirInfo == nil || !dstDirInfo.IsDir() {
		st.lastErr = "destination parent is not a directory"
		return
	}

	st.dstRaw = raw
	st.dstPath = dst
	st.lastErr = ""
	st.running = true
	st.doneCh = make(chan error, 1)

	srcPath := st.srcPath
	dstPath := st.dstPath
	remote := st.remote
	doneCh := st.doneCh
	go func() {
		if remote != nil {
			client := remote.sftpClient()
			if client == nil {
				doneCh <- errors.New("sftp session is not connected")
				return
			}
			doneCh <- client.Rename(srcPath, dstPath)
			return
		}
		doneCh <- os.Rename(srcPath, dstPath)
	}()

	_ = now
}

func (ui *UI) pumpFileMoveState(gtx layout.Context) {
	st := ui.fileMove
	if st == nil || !st.running || st.doneCh == nil {
		return
	}

	select {
	case err := <-st.doneCh:
		st.running = false
		st.doneCh = nil
		if err != nil {
			st.lastErr = err.Error()
			gtx.Execute(op.InvalidateCmd{})
			return
		}
		ui.finishFileMove(gtx.Now)
	default:
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) finishFileMove(now time.Time) {
	st := ui.fileMove
	if st == nil {
		return
	}

	remoteMove := st.remote != nil
	var remoteSetup fm.SSHSetup
	if remoteMove {
		remoteSetup = st.remote.setup
	}
	cleanPath := filepath.Clean
	dirName := filepath.Dir
	joinPath := filepath.Join
	baseName := filepath.Base
	if remoteMove {
		cleanPath = path.Clean
		dirName = path.Dir
		joinPath = path.Join
		baseName = path.Base
	}

	sources := st.sources
	if len(sources) == 0 && strings.TrimSpace(st.srcPath) != "" {
		sources = []fileMoveSource{{Path: st.srcPath, Name: st.srcName}}
	}
	sourcePaths := make(map[string]struct{}, len(sources))
	destPaths := make(map[string]struct{}, len(sources))
	sourceDirs := make(map[string]struct{}, len(sources))
	destDirs := make(map[string]struct{}, 1)
	primaryDestPath := ""
	for i, src := range sources {
		srcPath := cleanPath(src.Path)
		sourcePaths[srcPath] = struct{}{}
		sourceDirs[dirName(srcPath)] = struct{}{}

		dstPath := cleanPath(st.dstPath)
		if st.multiSource() {
			dstPath = cleanPath(joinPath(st.dstPath, baseName(srcPath)))
		}
		destPaths[dstPath] = struct{}{}
		destDirs[dirName(dstPath)] = struct{}{}
		if i == 0 {
			primaryDestPath = dstPath
		}
	}

	if st.remote != nil {
		st.remote.close()
		st.remote = nil
	}
	noticeText, noticeDur := fileMoveSuccessNotice(len(sources))
	ui.fileMove = nil
	ui.clearFileMoveHotkeyHold()

	samePathFn := func(a, b string) bool {
		if remoteMove {
			return path.Clean(a) == path.Clean(b)
		}
		return samePath(a, b)
	}

	noticeShown := false
	for i, pane := range ui.filePanes {
		if pane == nil || pane.model == nil || pane.table == nil {
			continue
		}
		if remoteMove {
			if !pane.remoteConnected() || pane.remote == nil || !sameSSHRemoteTarget(pane.remote.setup, remoteSetup) {
				continue
			}
			curDir := path.Clean(pane.dir)
			if _, ok := sourceDirs[curDir]; !ok {
				if _, ok := destDirs[curDir]; !ok {
					continue
				}
			}
		} else {
			if pane.remoteConnected() {
				continue
			}
			curDir := filepath.Clean(pane.dir)
			if _, ok := sourceDirs[curDir]; !ok {
				if _, ok := destDirs[curDir]; !ok {
					continue
				}
			}
		}

		selectedPath := ""
		selectedRow := pane.table.Selected
		if sel := pane.selectedEntry(); sel != nil {
			selectedPath = sel.Path
		}

		paneDir := filepath.Clean(pane.dir)
		if remoteMove {
			paneDir = path.Clean(pane.dir)
		}
		primaryPath := ""
		if _, ok := destDirs[paneDir]; ok {
			primaryPath = primaryDestPath
		}
		secondaryPath := ""
		if selectedPath != "" {
			skip := false
			for moved := range sourcePaths {
				if samePathFn(selectedPath, moved) {
					skip = true
					break
				}
			}
			if !skip {
				for moved := range destPaths {
					if samePathFn(selectedPath, moved) {
						skip = true
						break
					}
				}
			}
			if !skip {
				secondaryPath = selectedPath
			}
		}

		row := selectedRow
		if i == st.pane {
			row = st.row
		}
		restorePos := sanitizePaneListPosition(pane.table.List.Position)
		restoreAnchor := pane.visibleAnchorPath()
		if _, ok := sourceDirs[paneDir]; ok {
			restoreAnchor = filePaneRestoreAnchorPathSkipping(pane, sourcePaths, remoteMove)
		}
		reloadNoticeText := ""
		reloadNoticeDur := time.Duration(0)
		if i == st.pane {
			reloadNoticeText = noticeText
			reloadNoticeDur = noticeDur
		}
		if ui.requestPaneLoadWithSelectionAndScroll(i, pane.dir, primaryPath, secondaryPath, row, restorePos, true, restoreAnchor, reloadNoticeText, reloadNoticeDur) && i == st.pane {
			noticeShown = true
		}
	}
	if !noticeShown && noticeText != "" && st.pane >= 0 && st.pane < len(ui.filePanes) && ui.filePanes[st.pane] != nil {
		ui.filePanes[st.pane].setNoticeFor(noticeText, now, noticeDur)
	}
	_ = now
}

func fileMoveSuccessNotice(count int) (string, time.Duration) {
	if count <= 0 {
		return "", 0
	}
	label := "items"
	if count == 1 {
		label = "item"
	}
	return fmt.Sprintf("moved %d %s", count, label), fileMoveSuccessNoticeDur
}

func (ui *UI) closeFileMoveDialog() {
	st := ui.fileMove
	if st != nil && st.remote != nil {
		st.remote.close()
		st.remote = nil
	}
	ui.fileMove = nil
	ui.clearFileMoveHotkeyHold()
}

func (ui *UI) layoutFileMoveDialog(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.fileMove
	if st == nil {
		return layout.Dimensions{}
	}

	st.keyFocus.attach(gtx)
	st.syncEditorFocus(gtx)
	anyMods := ^key.Modifiers(0)

	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &st.dstEdit, Name: key.NameEnter, Optional: anyMods},
			key.Filter{Focus: &st.dstEdit, Name: key.NameReturn, Optional: anyMods},
		)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press || st.running || ke.Modifiers != 0 {
			continue
		}
		st.actionsAnim.setPulse("confirm", gtx.Now)
		ui.submitFileMoveDialog(gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}

	for {
		filters := []event.Filter{
			key.Filter{Name: key.NameEscape, Optional: anyMods},
			key.Filter{Name: key.NameTab, Optional: anyMods},
		}
		if st.focus == fileMoveDialogFocusActions {
			filters = append(filters,
				key.Filter{Name: key.NameEnter, Optional: anyMods},
				key.Filter{Name: key.NameReturn, Optional: anyMods},
				key.Filter{Name: key.NameLeftArrow, Optional: anyMods},
				key.Filter{Name: key.NameRightArrow, Optional: anyMods},
			)
		}
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case key.NameEscape:
			if st.running {
				continue
			}
			ui.closeFileMoveDialog()
			return layout.Dimensions{}
		case key.NameTab:
			if st.running {
				continue
			}
			step, ok := dialogTabStep(ke.Modifiers)
			if !ok {
				continue
			}
			if st.stepFocus(step) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameLeftArrow:
			if st.running || ke.Modifiers != 0 || st.focus != fileMoveDialogFocusActions {
				continue
			}
			if st.stepAction(-1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameRightArrow:
			if st.running || ke.Modifiers != 0 || st.focus != fileMoveDialogFocusActions {
				continue
			}
			if st.stepAction(1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameEnter, key.NameReturn:
			if st.running || ke.Modifiers != 0 {
				continue
			}
			if st.focus != fileMoveDialogFocusActions {
				continue
			}
			switch st.actionFocus {
			case fileMoveDialogActionCancel:
				st.actionsAnim.setPulse("cancel", gtx.Now)
				ui.closeFileMoveDialog()
				return layout.Dimensions{}
			case fileMoveDialogActionConfirm:
				st.actionsAnim.setPulse("confirm", gtx.Now)
				ui.submitFileMoveDialog(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}

	if !st.running {
		for {
			ev, ok := st.dstEdit.Update(gtx)
			if !ok {
				break
			}
			submit, ok := ev.(widget.SubmitEvent)
			if ok {
				st.dstEdit.SetText(submit.Text)
				st.actionsAnim.setPulse("confirm", gtx.Now)
				ui.submitFileMoveDialog(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			if _, ok := ev.(widget.ChangeEvent); ok {
				st.lastErr = ""
				st.refreshPreview()
			}
		}
	}

	if st.cancelClick.Clicked(gtx) && !st.running {
		ui.closeFileMoveDialog()
		return layout.Dimensions{}
	}
	if st.closeClick.Clicked(gtx) && !st.running {
		ui.closeFileMoveDialog()
		return layout.Dimensions{}
	}
	if st.confirmClick.Clicked(gtx) && !st.running {
		st.actionsAnim.setPulse("confirm", gtx.Now)
		ui.submitFileMoveDialog(gtx.Now)
	}
	if st.running {
		for st.closeClick.Clicked(gtx) {
		}
	}
	for st.backdropClick.Clicked(gtx) {
	}

	if st.dstEditWant && !st.running {
		st.dstEditWant = false
		gtx.Execute(key.FocusCmd{Tag: &st.dstEdit})
	}

	return st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{A: 120}, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

		paneRect := ui.filePaneRectForOverlay(gtx, st.pane)
		width := gtx.Dp(ui.scaleInterfaceDp(unit.Dp(390)))
		maxWidth := paneRect.Dx() - gtx.Dp(unit.Dp(16))
		if maxWidth < 220 {
			maxWidth = 220
		}
		if width > maxWidth {
			width = maxWidth
		}
		if width < 220 {
			width = 220
		}

		m := op.Record(gtx.Ops)
		dialog := fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
			return fillRoundedBox(
				gtx,
				gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
				color.NRGBA{R: 20, G: 20, B: 20, A: 252},
				color.NRGBA{R: 255, G: 255, B: 255, A: 18},
				func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutFileMoveDialogBody(th, gtx, st)
					})
				},
			)
		})
		call := m.Stop()

		x := paneRect.Min.X + (paneRect.Dx()-dialog.Size.X)/2
		y := paneRect.Min.Y + (paneRect.Dy()-dialog.Size.Y)/2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()

		if st.running {
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(33 * time.Millisecond)})
		}
		return layout.Dimensions{Size: gtx.Constraints.Max, Baseline: dialog.Baseline}
	})
}

func (ui *UI) layoutFileMoveDialogBody(th *material.Theme, gtx layout.Context, st *fileMoveState) layout.Dimensions {
	hoverActionKey := ""
	if !st.running && st.cancelClick.Hovered() {
		hoverActionKey = "cancel"
	}
	if !st.running && st.confirmClick.Hovered() {
		hoverActionKey = "confirm"
	}
	st.actionsAnim.setHover(hoverActionKey, gtx.Now)
	hoverCancel, hoverAnimCancel := st.actionsAnim.hoverFill(gtx.Now, "cancel")
	hoverConfirm, hoverAnimConfirm := st.actionsAnim.hoverFill(gtx.Now, "confirm")
	pulseCancel, pulseAnimCancel := st.actionsAnim.pulseFill(gtx.Now, "cancel")
	pulseConfirm, pulseAnimConfirm := st.actionsAnim.pulseFill(gtx.Now, "confirm")
	if hoverAnimCancel || hoverAnimConfirm || pulseAnimCancel || pulseAnimConfirm {
		gtx.Execute(op.InvalidateCmd{})
	}

	meta := formatCopyPathInfo(st.srcInfo)
	sameTarget := st.previewSameTarget()
	showTargetDiff := st.dstInfo.Exists && !sameTarget && !st.multiSource()
	if st.multiSource() {
		meta = ""
		if st.dstInfo.Exists {
			meta = "dst: " + formatCopyPathInfo(st.dstInfo)
		}
	}
	if st.dstInfo.Exists && !sameTarget && !st.multiSource() {
		meta = "dst exists: " + formatCopyPathInfo(st.dstInfo)
	}
	sourceLabel := "Move"
	if st.running {
		sourceLabel = "Moving"
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					titleText := "Rename / Move"
					if st.multiSource() {
						titleText = "Move"
					}
					title := material.Body1(th, titleText)
					title.Font.Typeface = ui.interfaceTypeface()
					title.Font.Weight = font.Bold
					title.TextSize = ui.scaleDialogFontSize(12)
					title.Color = txtColor
					return title.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutFlatCloseButton(gtx, &st.closeClick, false)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(layoutDialogHorizontalDivider),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileOpTextRow(th, gtx, sourceLabel, st.sourceOperationSummary(), txtColor)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileOpRow(th, gtx, "To", func(gtx layout.Context) layout.Dimensions {
				if st.running {
					lbl := material.Body2(th, st.dstPath)
					lbl.Font.Typeface = ui.interfaceTypeface()
					lbl.TextSize = ui.fileOpDialogTextSize()
					lbl.Color = txtColor
					lbl.MaxLines = 1
					lbl.Truncator = "…"
					return fillFlatBox(
						gtx,
						color.NRGBA{R: 24, G: 24, B: 24, A: 255},
						color.NRGBA{R: 255, G: 255, B: 255, A: 20},
						func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, lbl.Layout)
						},
					)
				}
				ed := material.Editor(th, &st.dstEdit, "")
				ed.Font.Typeface = ui.interfaceTypeface()
				ed.TextSize = ui.fileOpDialogTextSize()
				ed.Color = txtColor
				ed.HintColor = hintColor
				return ui.layoutEditorWithContextMenu(th, gtx, "filemove-dst", &st.dstEdit, true, func(gtx layout.Context) layout.Dimensions {
					return layoutNeutralEditorBox(gtx, st.focus == fileMoveDialogFocusDestination, true, ed.Layout)
				})
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if showTargetDiff {
				return ui.layoutFileOverwriteDiffInfo(th, gtx, "Target Details", st.srcInfo, st.dstInfo)
			}
			if meta == "" {
				return layout.Dimensions{}
			}
			return ui.layoutFileOpTextRow(th, gtx, "Details", meta, color.NRGBA{R: 184, G: 184, B: 184, A: 255})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.lastErr == "" {
				actionLabel, runningLabel := st.actionLabels()
				if !st.running && (!st.dstInfo.Exists || sameTarget || st.multiSource()) {
					return layout.Dimensions{}
				}
				if st.running {
					return ui.layoutFileOpTextRow(th, gtx, "Status", runningLabel, hintColor)
				}
				lbl := material.Caption(th, "Destination for "+strings.ToLower(actionLabel)+" already exists.")
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.TextSize = ui.scaleDialogFontSize(9)
				lbl.Color = color.NRGBA{R: 196, G: 196, B: 196, A: 255}
				lbl.MaxLines = 2
				return lbl.Layout(gtx)
			}
			lbl := material.Caption(th, st.lastErr)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleDialogFontSize(9)
			lbl.Color = color.NRGBA{R: 220, G: 140, B: 140, A: 255}
			lbl.MaxLines = 2
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layoutDialogHorizontalDivider),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label, runningLabel := st.actionLabels()
				if st.running {
					label = runningLabel
				}
				return ui.layoutDialogActionPair(
					th, gtx,
					&st.cancelClick, "Cancel", hoverCancel, pulseCancel, st.running,
					&st.confirmClick, label, hoverConfirm, pulseConfirm, st.running,
					st.actionVisualState(fileMoveDialogActionCancel),
					st.actionVisualState(fileMoveDialogActionConfirm),
				)
			})
		}),
	)
}

func (st *fileMoveState) resolvedDestinationPath() string {
	if st == nil {
		return ""
	}
	if st.multiSource() {
		dst, _, err := inspectExistingMoveDestinationDir(st.endpoint, st.dstEdit.Text())
		if err != nil {
			return ""
		}
		return dst
	}
	dst, err := st.effectiveDestinationPath(st.dstEdit.Text())
	if err != nil {
		return ""
	}
	return dst
}

func (st *fileMoveState) previewSameTarget() bool {
	if st == nil {
		return false
	}
	if st.multiSource() {
		return false
	}
	dst := st.resolvedDestinationPath()
	if dst == "" {
		return false
	}
	return st.endpoint.samePath(st.srcPath, dst)
}

func (st *fileMoveState) actionLabels() (label string, running string) {
	if st == nil {
		return "Move", "Moving..."
	}
	if st.multiSource() {
		return "Move", "Moving..."
	}
	dst := st.resolvedDestinationPath()
	if dst == "" {
		return "Move", "Moving..."
	}
	srcDir := st.endpoint.dirName(st.srcPath)
	dstDir := st.endpoint.dirName(dst)
	if st.endpoint.samePath(srcDir, dstDir) {
		return "Rename", "Renaming..."
	}
	return "Move", "Moving..."
}

func (st *fileMoveState) effectiveDestinationPath(raw string) (string, error) {
	if st == nil {
		return "", errors.New("move state is nil")
	}
	dst, err := resolveFileOpTargetPath(st.endpoint, st.endpoint.dir, raw)
	if err != nil {
		return "", err
	}
	info, err := endpointStat(st.endpoint, dst)
	if err == nil && info != nil && info.IsDir() {
		dst = st.endpoint.join(dst, st.endpoint.baseName(st.srcPath))
	}
	return dst, nil
}

func (st *fileMoveState) buildMovePlans(raw string) (string, fileCopyPathInfo, []fileMovePlan, error) {
	if st == nil {
		return "", fileCopyPathInfo{}, nil, errors.New("move state is nil")
	}
	dstDir, dstInfo, err := inspectExistingMoveDestinationDir(st.endpoint, raw)
	if err != nil {
		return "", fileCopyPathInfo{}, nil, err
	}
	plans := make([]fileMovePlan, 0, len(st.sources))
	for _, src := range st.sources {
		srcPath, err := st.endpoint.normalizeSourcePath(src.Path)
		if err != nil {
			return "", fileCopyPathInfo{}, nil, err
		}
		dstPath := st.endpoint.join(dstDir, st.endpoint.baseName(srcPath))
		if st.endpoint.samePath(srcPath, dstPath) {
			return "", fileCopyPathInfo{}, nil, errors.New("source and destination are the same")
		}
		if existing, err := endpointLstat(st.endpoint, dstPath); err == nil && existing != nil {
			label := st.endpoint.baseName(dstPath)
			if strings.TrimSpace(label) == "" {
				label = dstPath
			}
			return "", fileCopyPathInfo{}, nil, fmt.Errorf("destination already exists: %s", label)
		}
		plans = append(plans, fileMovePlan{
			srcPath: srcPath,
			dstPath: dstPath,
		})
	}
	return dstDir, dstInfo, plans, nil
}

func inspectExistingMoveDestinationDir(ep copyEndpoint, raw string) (string, fileCopyPathInfo, error) {
	dstDir, err := resolveFileOpTargetPath(ep, ep.dir, raw)
	if err != nil {
		return "", fileCopyPathInfo{}, err
	}
	info := fileCopyPathInfo{Path: dstDir}
	dstStat, err := endpointStat(ep, dstDir)
	if err != nil {
		return "", fileCopyPathInfo{}, errors.New("destination directory does not exist")
	}
	if dstStat == nil || !dstStat.IsDir() {
		return "", fileCopyPathInfo{}, errors.New("destination must be a directory")
	}
	info.Exists = true
	info.IsDir = true
	info.ModTime = dstStat.ModTime()
	return dstDir, info, nil
}

func (ui *UI) fileMoveDefaultTargetDir(srcIdx int, srcEndpoint copyEndpoint) string {
	if ui == nil {
		return strings.TrimSpace(srcEndpoint.dir)
	}
	for i, pane := range ui.filePanes {
		if i == srcIdx || pane == nil {
			continue
		}
		if !copyEndpointsCompatible(srcEndpoint, copyEndpointFromPane(i, pane)) {
			continue
		}
		target := strings.TrimSpace(pane.dir)
		if target != "" {
			return target
		}
	}
	return strings.TrimSpace(srcEndpoint.dir)
}

func copyEndpointsCompatible(a, b copyEndpoint) bool {
	if a.isRemote() != b.isRemote() {
		return false
	}
	if a.isRemote() {
		if a.remote == nil || b.remote == nil {
			return false
		}
		return sameSSHRemoteTarget(a.remote.setup, b.remote.setup)
	}
	return true
}

func resolveFileOpTargetPath(ep copyEndpoint, baseDir, raw string) (string, error) {
	txt := strings.TrimSpace(raw)
	if txt == "" {
		return "", errors.New("path is empty")
	}
	if ep.isRemote() {
		base := strings.TrimSpace(baseDir)
		if base == "" {
			base = strings.TrimSpace(ep.dir)
		}
		if base == "" {
			base = "/"
		}
		if !path.IsAbs(base) {
			base = "/" + base
		}
		if !path.IsAbs(txt) {
			txt = path.Join(base, txt)
		}
		txt = path.Clean(txt)
		if txt == "" || txt == "." {
			txt = "/"
		}
		if !strings.HasPrefix(txt, "/") {
			txt = "/" + txt
		}
		return txt, nil
	}

	if !filepath.IsAbs(txt) {
		base := strings.TrimSpace(baseDir)
		if base == "" {
			base = strings.TrimSpace(ep.dir)
		}
		if base == "" {
			base = "."
		}
		txt = filepath.Join(base, txt)
	}
	abs, err := filepath.Abs(txt)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}
