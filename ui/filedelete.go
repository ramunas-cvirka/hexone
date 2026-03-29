// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"fmt"
	"hexone/filesys"
	uitheme "hexone/ui/theme"
	"image"
	"image/color"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const fileDeleteSuccessNoticeDur = 1200 * time.Millisecond

type fileDeleteState struct {
	pane int
	row  int

	targets    []fileDeleteTarget
	targetPath string
	targetName string
	targetInfo fileCopyPathInfo
	remote     *paneSSHSession

	deletedNestedCount int
	deletedCountKnown  bool

	backdropClick widget.Clickable
	closeClick    widget.Clickable
	confirmClick  widget.Clickable
	cancelClick   widget.Clickable

	running bool
	lastErr string

	doneCh      chan fileDeleteResult
	actionsAnim segmentedAnimState
	keyFocus    dialogKeyboardFocusState
	focus       fileDeleteDialogFocus
	actionFocus fileDeleteDialogAction
}

type fileDeleteDialogFocus uint8

const (
	fileDeleteDialogFocusActions fileDeleteDialogFocus = iota
)

type fileDeleteDialogAction uint8

const (
	fileDeleteDialogActionCancel fileDeleteDialogAction = iota
	fileDeleteDialogActionConfirm
)

type fileDeleteTarget struct {
	Path string
	Name string
}

type fileDeleteResult struct {
	err               error
	deletedNested     int
	deletedCountKnown bool
}

func (ui *UI) startFileDeleteDialog(idx int, now time.Time) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.model == nil || pane.table == nil {
		return
	}
	if pane.archiveBrowsing() {
		pane.setNotice("cannot delete files inside an archive", now)
		return
	}
	row := pane.table.Selected
	selected := pane.selectedEntriesForAction()
	if len(selected) == 0 {
		if entry := pane.selectedEntry(); entry != nil && entry.Kind == filesys.EntryParent {
			pane.setNotice("cannot delete parent entry", now)
			return
		}
		pane.setNotice("nothing selected to delete", now)
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
		info fileCopyPathInfo
		err  error
	)
	if remote != nil {
		info, err = buildCopyPathInfoRemote(remote, entry.Path)
	} else {
		info, err = buildCopyPathInfo(entry.Path)
	}
	if err != nil {
		if remote != nil {
			remote.close()
		}
		pane.setNotice(err.Error(), now)
		return
	}
	targets := make([]fileDeleteTarget, 0, len(selected))
	for _, item := range selected {
		targets = append(targets, fileDeleteTarget{
			Path: item.Path,
			Name: item.DisplayName,
		})
	}

	ui.fileDelete = &fileDeleteState{
		pane:        idx,
		row:         row,
		targets:     targets,
		targetPath:  entry.Path,
		targetName:  entry.DisplayName,
		targetInfo:  info,
		remote:      remote,
		focus:       fileDeleteDialogFocusActions,
		actionFocus: fileDeleteDialogActionConfirm,
		keyFocus:    dialogKeyboardFocusState{wantFocus: true},
	}
	ui.rep.active = false
	ui.rep.pane = -1
	ui.clearFileDeleteHotkeyHold()
}

func (ui *UI) clearFileDeleteHotkeyHold() {
	if ui == nil || ui.held == nil {
		return
	}
	ui.held[fileActionKey(fileActionDelete)] = false
}

func (st *fileDeleteState) multiTarget() bool {
	return st != nil && len(st.targets) > 1
}

func (st *fileDeleteState) targetCount() int {
	if st == nil {
		return 0
	}
	if len(st.targets) > 0 {
		return len(st.targets)
	}
	if strings.TrimSpace(st.targetPath) != "" {
		return 1
	}
	return 0
}

func (st *fileDeleteState) targetSummary() string {
	count := st.targetCount()
	if count <= 1 {
		target := strings.TrimSpace(st.targetName)
		if target != "" {
			return target
		}
		return st.targetPath
	}
	return fmt.Sprintf("%d items selected", count)
}

func (st *fileDeleteState) targetPreviewLines() []string {
	if st == nil || len(st.targets) == 0 {
		return nil
	}
	labels := make([]string, 0, len(st.targets))
	for _, target := range st.targets {
		labels = append(labels, fileOpPreviewLabel(target.Name, target.Path))
	}
	return fileOpPreviewLines(labels)
}

func (st *fileDeleteState) focusOrder() []fileDeleteDialogFocus {
	if st == nil {
		return nil
	}
	return []fileDeleteDialogFocus{
		fileDeleteDialogFocusActions,
	}
}

func (st *fileDeleteState) setFocus(target fileDeleteDialogFocus) bool {
	if st == nil {
		return false
	}
	changed := st.focus != target
	st.focus = target
	st.keyFocus.focusKeyboard()
	return changed
}

func (st *fileDeleteState) stepFocus(step int) bool {
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

func (st *fileDeleteState) stepAction(step int) bool {
	if st == nil {
		return false
	}
	order := []fileDeleteDialogAction{fileDeleteDialogActionCancel, fileDeleteDialogActionConfirm}
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

func (st *fileDeleteState) actionVisualState(target fileDeleteDialogAction) dialogActionVisualState {
	if st == nil || st.running {
		return dialogActionVisualState{}
	}
	if st.focus == fileDeleteDialogFocusActions {
		active := st.actionFocus == target
		return dialogActionVisualState{Focused: active, Default: active}
	}
	return dialogActionVisualState{Default: target == fileDeleteDialogActionConfirm}
}

func (ui *UI) submitFileDeleteDialog(now time.Time) {
	st := ui.fileDelete
	if st == nil || st.running {
		return
	}
	st.lastErr = ""
	st.running = true
	doneCh := make(chan fileDeleteResult, 1)
	st.doneCh = doneCh

	targets := st.targets
	if len(targets) == 0 && strings.TrimSpace(st.targetPath) != "" {
		targets = []fileDeleteTarget{{Path: st.targetPath, Name: st.targetName}}
	}
	remote := st.remote
	go func() {
		res := fileDeleteResult{}
		if nestedCount, err := countDeleteNestedEntries(targets, remote); err == nil {
			res.deletedNested = nestedCount
			res.deletedCountKnown = true
		}
		for _, target := range targets {
			if remote != nil {
				if err := deleteRemotePath(remote, target.Path); err != nil {
					res.err = err
					doneCh <- res
					return
				}
				continue
			}
			if err := filesys.DeletePath(target.Path); err != nil {
				res.err = err
				doneCh <- res
				return
			}
		}
		doneCh <- res
	}()

	_ = now
}

func (ui *UI) pumpFileDeleteState(gtx layout.Context) {
	st := ui.fileDelete
	if st == nil || !st.running || st.doneCh == nil {
		return
	}

	select {
	case res := <-st.doneCh:
		st.running = false
		st.doneCh = nil
		if res.err != nil {
			st.lastErr = res.err.Error()
			gtx.Execute(op.InvalidateCmd{})
			return
		}
		st.deletedNestedCount = res.deletedNested
		st.deletedCountKnown = res.deletedCountKnown
		ui.finishFileDelete(gtx.Now)
	default:
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) finishFileDelete(now time.Time) {
	st := ui.fileDelete
	if st == nil {
		return
	}
	remoteDelete := st.remote != nil
	paneIdx := st.pane
	cleanPath := filepath.Clean
	dirName := filepath.Dir
	if remoteDelete {
		cleanPath = path.Clean
		dirName = path.Dir
	}
	targets := st.targets
	if len(targets) == 0 && strings.TrimSpace(st.targetPath) != "" {
		targets = []fileDeleteTarget{{Path: st.targetPath, Name: st.targetName}}
	}
	deletedPaths := make(map[string]struct{}, len(targets))
	deletedDirs := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		deletedPath := cleanPath(target.Path)
		deletedPaths[deletedPath] = struct{}{}
		deletedDirs[dirName(deletedPath)] = struct{}{}
	}
	preferRow := st.row
	nestedCount := 0
	if st.deletedCountKnown {
		nestedCount = st.deletedNestedCount
	}
	noticeText, noticeDur := fileDeleteSuccessNotice(len(targets), nestedCount)

	ui.fileDelete = nil
	ui.clearFileDeleteHotkeyHold()

	originReloaded := false
	for i, pane := range ui.filePanes {
		if pane == nil || pane.model == nil || pane.table == nil {
			continue
		}
		if remoteDelete {
			if !pane.remoteConnected() || pane.remote == nil || !sameSSHRemoteTarget(pane.remote.setup, st.remote.setup) {
				continue
			}
			curDir := path.Clean(pane.dir)
			if _, ok := deletedDirs[curDir]; !ok {
				continue
			}
		} else {
			if pane.remoteConnected() {
				continue
			}
			curDir := filepath.Clean(pane.dir)
			if _, ok := deletedDirs[curDir]; !ok {
				continue
			}
		}

		selectedPath := ""
		if sel := pane.selectedEntry(); sel != nil {
			selectedPath = filepath.Clean(sel.Path)
			if remoteDelete {
				selectedPath = path.Clean(sel.Path)
			}
		}

		sameSelected := false
		if selectedPath != "" {
			for deletedPath := range deletedPaths {
				if remoteDelete {
					sameSelected = selectedPath == deletedPath
				} else {
					sameSelected = samePath(selectedPath, deletedPath)
				}
				if sameSelected {
					break
				}
			}
		}
		row := 0
		if i == paneIdx {
			row = preferRow
		} else {
			row = pane.table.Selected
		}

		primaryPath := ""
		if selectedPath != "" && !sameSelected {
			primaryPath = selectedPath
		}
		restorePos := sanitizePaneListPosition(pane.table.List.Position)
		restoreAnchor := filePaneRestoreAnchorPathSkipping(pane, deletedPaths, remoteDelete)
		reloadNoticeText := ""
		reloadNoticeDur := time.Duration(0)
		if i == paneIdx {
			reloadNoticeText = noticeText
			reloadNoticeDur = noticeDur
		}
		if ui.requestPaneLoadWithSelectionAndScroll(i, pane.dir, primaryPath, "", row, restorePos, true, restoreAnchor, reloadNoticeText, reloadNoticeDur) && i == paneIdx {
			originReloaded = true
		}
	}
	if !originReloaded {
		if paneIdx >= 0 && paneIdx < len(ui.filePanes) && ui.filePanes[paneIdx] != nil && noticeText != "" {
			ui.filePanes[paneIdx].setNoticeFor(noticeText, now, noticeDur)
		}
	}
	if st.remote != nil {
		st.remote.close()
		st.remote = nil
	}
}

func fileDeleteSuccessNotice(count, nestedCount int) (string, time.Duration) {
	if count <= 0 {
		return "", 0
	}
	label := "items"
	if count == 1 {
		label = "item"
	}
	msg := fmt.Sprintf("deleted %d %s", count, label)
	if nestedCount > 0 {
		nestedLabel := "nested items"
		if nestedCount == 1 {
			nestedLabel = "nested item"
		}
		msg = fmt.Sprintf("%s (%d %s)", msg, nestedCount, nestedLabel)
	}
	return msg, fileDeleteSuccessNoticeDur
}

func countDeleteNestedEntries(targets []fileDeleteTarget, remote *paneSSHSession) (int, error) {
	ep := copyEndpoint{remote: remote}
	total := 0
	for _, target := range targets {
		targetPath := strings.TrimSpace(target.Path)
		if targetPath == "" {
			continue
		}
		info, err := endpointLstat(ep, targetPath)
		if err != nil {
			return 0, err
		}
		entries, _, err := collectTransferEntries(ep, targetPath, info)
		if err != nil {
			return 0, err
		}
		if len(entries) > 1 {
			total += len(entries) - 1
		}
	}
	return total, nil
}

func filePaneRestoreAnchorPathSkipping(pane *filePaneState, skippedPaths map[string]struct{}, remote bool) string {
	if pane == nil || pane.table == nil || pane.model == nil || pane.model.Len() == 0 {
		return ""
	}
	first := pane.table.List.Position.First
	if first < 0 {
		first = 0
	}
	if first >= pane.model.Len() {
		first = pane.model.Len() - 1
	}

	if pathVal := filePaneFirstVisiblePathFrom(pane, first, skippedPaths, remote); pathVal != "" {
		return pathVal
	}
	for i := first - 1; i >= 0; i-- {
		if pathVal := filePaneVisibleEntryPath(pane.model.Entry(i), skippedPaths, remote); pathVal != "" {
			return pathVal
		}
	}
	return ""
}

func filePaneFirstVisiblePathFrom(pane *filePaneState, start int, skippedPaths map[string]struct{}, remote bool) string {
	if pane == nil || pane.model == nil {
		return ""
	}
	for i := start; i < pane.model.Len(); i++ {
		if pathVal := filePaneVisibleEntryPath(pane.model.Entry(i), skippedPaths, remote); pathVal != "" {
			return pathVal
		}
	}
	return ""
}

func filePaneVisibleEntryPath(entry *filesys.Entry, skippedPaths map[string]struct{}, remote bool) string {
	if entry == nil || entry.Path == "" || entry.Kind == filesys.EntryParent {
		return ""
	}
	if filePanePathSkipped(entry.Path, skippedPaths, remote) {
		return ""
	}
	return entry.Path
}

func filePanePathSkipped(pathVal string, skippedPaths map[string]struct{}, remote bool) bool {
	if len(skippedPaths) == 0 || strings.TrimSpace(pathVal) == "" {
		return false
	}
	if remote {
		_, ok := skippedPaths[path.Clean(pathVal)]
		return ok
	}
	clean := filepath.Clean(pathVal)
	if _, ok := skippedPaths[clean]; ok {
		return true
	}
	if os.PathSeparator != '\\' {
		return false
	}
	for skippedPath := range skippedPaths {
		if samePath(clean, skippedPath) {
			return true
		}
	}
	return false
}

func (ui *UI) closeFileDeleteDialog() {
	st := ui.fileDelete
	if st != nil && st.remote != nil {
		st.remote.close()
		st.remote = nil
	}
	ui.fileDelete = nil
	ui.clearFileDeleteHotkeyHold()
}

func (ui *UI) layoutFileDeleteDialog(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.fileDelete
	if st == nil {
		return layout.Dimensions{}
	}

	st.keyFocus.attach(gtx)
	anyMods := ^key.Modifiers(0)

	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape, Optional: anyMods},
			key.Filter{Name: key.NameTab, Optional: anyMods},
			key.Filter{Name: key.NameEnter, Optional: anyMods},
			key.Filter{Name: key.NameReturn, Optional: anyMods},
			key.Filter{Name: key.NameLeftArrow, Optional: anyMods},
			key.Filter{Name: key.NameRightArrow, Optional: anyMods},
		)
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
			ui.closeFileDeleteDialog()
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
			if st.running || ke.Modifiers != 0 || st.focus != fileDeleteDialogFocusActions {
				continue
			}
			if st.stepAction(-1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameRightArrow:
			if st.running || ke.Modifiers != 0 || st.focus != fileDeleteDialogFocusActions {
				continue
			}
			if st.stepAction(1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameEnter, key.NameReturn:
			if st.running || ke.Modifiers != 0 {
				continue
			}
			switch st.actionFocus {
			case fileDeleteDialogActionCancel:
				st.actionsAnim.setPulse("cancel", gtx.Now)
				ui.closeFileDeleteDialog()
				return layout.Dimensions{}
			case fileDeleteDialogActionConfirm:
				st.actionsAnim.setPulse("confirm", gtx.Now)
				ui.submitFileDeleteDialog(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}

	if st.cancelClick.Clicked(gtx) && !st.running {
		ui.closeFileDeleteDialog()
		return layout.Dimensions{}
	}
	if st.closeClick.Clicked(gtx) && !st.running {
		ui.closeFileDeleteDialog()
		return layout.Dimensions{}
	}
	if st.confirmClick.Clicked(gtx) && !st.running {
		st.actionsAnim.setPulse("confirm", gtx.Now)
		ui.submitFileDeleteDialog(gtx.Now)
	}
	if st.running {
		for st.closeClick.Clicked(gtx) {
		}
	}
	for st.backdropClick.Clicked(gtx) {
	}

	return st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{A: 120}, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

		paneRect := ui.filePaneRectForOverlay(gtx, st.pane)
		width := gtx.Dp(unit.Dp(300))
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
						return ui.layoutFileDeleteDialogBody(th, gtx, st)
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

func (ui *UI) layoutFileDeleteDialogBody(th *material.Theme, gtx layout.Context, st *fileDeleteState) layout.Dimensions {
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

	desc := material.Caption(th, "This action cannot be undone.")
	desc.Font.Typeface = ui.mainTypeface()
	desc.TextSize = scaleDialogThemeFontSize(th, 9)
	desc.Color = color.NRGBA{R: 206, G: 186, B: 148, A: 255}

	target := st.targetSummary()
	if target == "" {
		if st.remote != nil {
			target = path.Base(st.targetPath)
		} else {
			target = filepath.Base(st.targetPath)
		}
	}
	targetLabel := material.Body2(th, target)
	targetLabel.Font.Typeface = ui.mainTypeface()
	targetLabel.TextSize = scaleDialogThemeFontSize(th, 10)
	targetLabel.Font.Weight = font.Medium
	targetLabel.Color = color.NRGBA{R: 220, G: 220, B: 220, A: 255}
	targetLabel.MaxLines = 1
	targetLabel.Truncator = "…"

	pathLabel := material.Caption(th, st.targetPath)
	pathLabel.Font.Typeface = ui.mainTypeface()
	pathLabel.TextSize = scaleDialogThemeFontSize(th, 9)
	pathLabel.Color = color.NRGBA{R: 172, G: 172, B: 172, A: 255}
	pathLabel.MaxLines = 1
	pathLabel.Truncator = "…"

	metaText := formatCopyPathInfo(st.targetInfo)
	if st.multiTarget() {
		metaText = fmt.Sprintf("%d items will be deleted", st.targetCount())
	}
	meta := material.Caption(th, metaText)
	meta.Font.Typeface = ui.mainTypeface()
	meta.TextSize = scaleDialogThemeFontSize(th, 9)
	meta.Color = color.NRGBA{R: 184, G: 184, B: 184, A: 255}
	meta.MaxLines = 1

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					title := material.Body1(th, "Delete")
					title.Font.Typeface = ui.mainTypeface()
					title.Font.Weight = font.Bold
					title.TextSize = scaleDialogThemeFontSize(th, 12)
					title.Color = txtColor
					return title.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutTinyIconModeButton(th, gtx, &st.closeClick, uitheme.CloseIcon(), false)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Rigid(desc.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(targetLabel.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.multiTarget() {
				return ui.layoutFileOpPreviewList(th, gtx, st.targetPreviewLines())
			}
			return pathLabel.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(meta.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.lastErr == "" {
				return layout.Dimensions{}
			}
			lbl := material.Caption(th, st.lastErr)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleDialogThemeFontSize(th, 9)
			lbl.Color = color.NRGBA{R: 220, G: 140, B: 140, A: 255}
			lbl.MaxLines = 2
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !st.running {
				return layout.Dimensions{}
			}
			lbl := material.Caption(th, "Deleting...")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleDialogThemeFontSize(th, 9)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := "Delete"
				if st.running {
					label = "Deleting..."
				}
				return ui.layoutDialogActionPair(
					th, gtx,
					&st.cancelClick, "Cancel", hoverCancel, pulseCancel, st.running,
					&st.confirmClick, label, hoverConfirm, pulseConfirm, st.running,
					st.actionVisualState(fileDeleteDialogActionCancel),
					st.actionVisualState(fileDeleteDialogActionConfirm),
				)
			})
		}),
	)
}

func buildCopyPathInfo(path string) (fileCopyPathInfo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fileCopyPathInfo{}, err
	}
	abs = filepath.Clean(abs)
	st, err := os.Lstat(abs)
	if err != nil {
		return fileCopyPathInfo{}, err
	}
	info := fileCopyPathInfo{
		Path:    abs,
		Exists:  true,
		IsDir:   st.IsDir(),
		ModTime: st.ModTime(),
	}
	if st.Mode().IsRegular() {
		info.Size = st.Size()
	}
	return info, nil
}

func buildCopyPathInfoRemote(remote *paneSSHSession, p string) (fileCopyPathInfo, error) {
	if remote == nil {
		return fileCopyPathInfo{}, errors.New("remote session is nil")
	}
	client := remote.sftpClient()
	if client == nil {
		return fileCopyPathInfo{}, errors.New("sftp session is not connected")
	}
	clean := path.Clean(strings.TrimSpace(p))
	if clean == "" {
		clean = "/"
	}
	st, err := client.Lstat(clean)
	if err != nil {
		return fileCopyPathInfo{}, err
	}
	info := fileCopyPathInfo{
		Path:    clean,
		Exists:  true,
		IsDir:   st.IsDir(),
		ModTime: st.ModTime(),
	}
	if st.Mode().IsRegular() {
		info.Size = st.Size()
	}
	return info, nil
}

func deleteRemotePath(remote *paneSSHSession, p string) error {
	if remote == nil {
		return errors.New("remote session is nil")
	}
	client := remote.sftpClient()
	if client == nil {
		return errors.New("sftp session is not connected")
	}
	target := path.Clean(strings.TrimSpace(p))
	if target == "" {
		target = "/"
	}
	info, err := client.Lstat(target)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return client.Remove(target)
	}
	return client.RemoveAll(target)
}

func (ui *UI) filePaneRectForOverlay(gtx layout.Context, paneIdx int) image.Rectangle {
	inset := gtx.Dp(unit.Dp(8))
	max := gtx.Constraints.Max
	contentW := max.X - inset*2
	contentH := max.Y - inset*2
	if contentW < 1 {
		contentW = max.X
	}
	if contentH < 1 {
		contentH = max.Y
	}

	n := len(ui.filePanes)
	if n < 1 {
		return image.Rect(inset, inset, inset+contentW, inset+contentH)
	}
	if paneIdx < 0 {
		paneIdx = 0
	}
	if paneIdx >= n {
		paneIdx = n - 1
	}

	gap := gtx.Dp(unit.Dp(4))
	totalGap := gap * (n - 1)
	usable := contentW - totalGap
	if usable < n {
		usable = n
	}
	base := usable / n
	rem := usable % n

	x := inset
	for i := 0; i < paneIdx; i++ {
		w := base
		if i < rem {
			w++
		}
		x += w + gap
	}
	w := base
	if paneIdx < rem {
		w++
	}
	return image.Rect(x, inset, x+w, inset+contentH)
}
