package ui

import (
	"errors"
	"hexone/filesys"
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

type fileDeleteState struct {
	pane int
	row  int

	targetPath string
	targetName string
	targetInfo fileCopyPathInfo
	remote     *paneSSHSession

	backdropClick widget.Clickable
	closeClick    widget.Clickable
	confirmClick  widget.Clickable
	cancelClick   widget.Clickable

	running bool
	lastErr string

	doneCh      chan error
	actionsAnim segmentedAnimState
}

func (ui *UI) startFileDeleteDialog(idx int, now time.Time) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.model == nil || pane.table == nil {
		return
	}
	row := pane.table.Selected
	entry := pane.model.Entry(row)
	if entry == nil || entry.Path == "" {
		pane.setNotice("nothing selected to delete", now)
		return
	}
	if entry.Kind == filesys.EntryParent {
		pane.setNotice("cannot delete parent entry", now)
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

	ui.fileDelete = &fileDeleteState{
		pane:       idx,
		row:        row,
		targetPath: entry.Path,
		targetName: entry.DisplayName,
		targetInfo: info,
		remote:     remote,
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

func (ui *UI) submitFileDeleteDialog(now time.Time) {
	st := ui.fileDelete
	if st == nil || st.running {
		return
	}
	st.lastErr = ""
	st.running = true
	doneCh := make(chan error, 1)
	st.doneCh = doneCh

	target := st.targetPath
	remote := st.remote
	go func() {
		if remote != nil {
			doneCh <- deleteRemotePath(remote, target)
			return
		}
		doneCh <- filesys.DeletePath(target)
	}()

	_ = now
}

func (ui *UI) pumpFileDeleteState(gtx layout.Context) {
	st := ui.fileDelete
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
	deletedPath := filepath.Clean(st.targetPath)
	deletedDir := filepath.Clean(filepath.Dir(deletedPath))
	if remoteDelete {
		deletedPath = path.Clean(st.targetPath)
		deletedDir = path.Clean(path.Dir(deletedPath))
	}
	preferRow := st.row

	ui.fileDelete = nil
	ui.clearFileDeleteHotkeyHold()

	for i, pane := range ui.filePanes {
		if pane == nil || pane.model == nil || pane.table == nil {
			continue
		}
		if remoteDelete {
			if !pane.remoteConnected() || pane.remote == nil || !sameSSHRemoteTarget(pane.remote.setup, st.remote.setup) {
				continue
			}
			curDir := path.Clean(pane.dir)
			if curDir != deletedDir {
				continue
			}
		} else {
			if pane.remoteConnected() {
				continue
			}
			curDir := filepath.Clean(pane.dir)
			if !samePath(curDir, deletedDir) {
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
		if err := pane.load(pane.dir); err != nil {
			pane.setNotice(err.Error(), now)
			continue
		}

		sameSelected := false
		if selectedPath != "" {
			if remoteDelete {
				sameSelected = selectedPath == deletedPath
			} else {
				sameSelected = samePath(selectedPath, deletedPath)
			}
		}
		if selectedPath != "" && !sameSelected {
			if idx := pane.findEntryPathIndex(selectedPath); idx >= 0 {
				pane.table.SetSelected(idx, pane.model.Len(), false)
				continue
			}
		}

		row := 0
		if i == paneIdx {
			row = preferRow
		} else {
			row = pane.table.Selected
		}
		if row < 0 {
			row = 0
		}
		if pane.model.Len() > 0 {
			if row >= pane.model.Len() {
				row = pane.model.Len() - 1
			}
			pane.table.SetSelected(row, pane.model.Len(), false)
		}
	}
	if st.remote != nil {
		st.remote.close()
		st.remote = nil
	}
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

	for _, name := range []key.Name{key.NameEscape, key.NameEnter, key.NameReturn} {
		for {
			ev, ok := gtx.Event(key.Filter{Name: name})
			if !ok {
				break
			}
			ke, ok := ev.(key.Event)
			if !ok || ke.State != key.Press {
				continue
			}
			switch name {
			case key.NameEscape:
				if !st.running {
					ui.closeFileDeleteDialog()
				}
			case key.NameEnter, key.NameReturn:
				if !st.running {
					st.actionsAnim.setPulse("confirm", gtx.Now)
					ui.submitFileDeleteDialog(gtx.Now)
				}
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
	desc.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
	desc.Color = color.NRGBA{R: 206, G: 186, B: 148, A: 255}

	target := st.targetName
	if target == "" {
		if st.remote != nil {
			target = path.Base(st.targetPath)
		} else {
			target = filepath.Base(st.targetPath)
		}
	}
	targetLabel := material.Body2(th, target)
	targetLabel.Font.Typeface = ui.mainTypeface()
	targetLabel.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
	targetLabel.Font.Weight = font.Medium
	targetLabel.Color = color.NRGBA{R: 220, G: 220, B: 220, A: 255}
	targetLabel.MaxLines = 1
	targetLabel.Truncator = "…"

	pathLabel := material.Caption(th, st.targetPath)
	pathLabel.Font.Typeface = ui.mainTypeface()
	pathLabel.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
	pathLabel.Color = color.NRGBA{R: 172, G: 172, B: 172, A: 255}
	pathLabel.MaxLines = 1
	pathLabel.Truncator = "…"

	meta := material.Caption(th, formatCopyPathInfo(st.targetInfo))
	meta.Font.Typeface = ui.mainTypeface()
	meta.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
	meta.Color = color.NRGBA{R: 184, G: 184, B: 184, A: 255}
	meta.MaxLines = 1

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					title := material.Body1(th, "Delete")
					title.Font.Typeface = ui.mainTypeface()
					title.Font.Weight = font.Bold
					title.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 12)
					title.Color = txtColor
					return title.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutTinyIconModeButton(th, gtx, &st.closeClick, uiCloseIcon(), false)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Rigid(desc.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(targetLabel.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(pathLabel.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(meta.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.lastErr == "" {
				return layout.Dimensions{}
			}
			lbl := material.Caption(th, st.lastErr)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
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
			lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
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
