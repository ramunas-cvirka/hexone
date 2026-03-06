package ui

import (
	"errors"
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
}

func (ui *UI) startFileMoveDialog(idx int, now time.Time) {
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
		pane.setNotice("nothing selected to rename/move", now)
		return
	}
	if entry.Kind == filesys.EntryParent {
		pane.setNotice("cannot rename/move parent entry", now)
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

	st := &fileMoveState{
		pane:     idx,
		row:      row,
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

	st.dstEdit.SingleLine = true
	st.dstEdit.Submit = true
	st.dstEdit.SetText(targetDefault)
	st.dstEdit.SetCaret(st.dstEdit.Len(), st.dstEdit.Len())
	st.dstEditWant = true
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
	srcPath := filepath.Clean(st.srcPath)
	dstPath := filepath.Clean(st.dstPath)
	srcDir := filepath.Clean(filepath.Dir(srcPath))
	dstDir := filepath.Clean(filepath.Dir(dstPath))
	if remoteMove {
		srcPath = path.Clean(st.srcPath)
		dstPath = path.Clean(st.dstPath)
		srcDir = path.Clean(path.Dir(srcPath))
		dstDir = path.Clean(path.Dir(dstPath))
	}

	if st.remote != nil {
		st.remote.close()
		st.remote = nil
	}
	ui.fileMove = nil
	ui.clearFileMoveHotkeyHold()

	samePathFn := func(a, b string) bool {
		if remoteMove {
			return path.Clean(a) == path.Clean(b)
		}
		return samePath(a, b)
	}

	for i, pane := range ui.filePanes {
		if pane == nil || pane.model == nil || pane.table == nil {
			continue
		}
		if remoteMove {
			if !pane.remoteConnected() || pane.remote == nil || !sameSSHRemoteTarget(pane.remote.setup, remoteSetup) {
				continue
			}
			curDir := path.Clean(pane.dir)
			if curDir != srcDir && curDir != dstDir {
				continue
			}
		} else {
			if pane.remoteConnected() {
				continue
			}
			curDir := filepath.Clean(pane.dir)
			if !samePath(curDir, srcDir) && !samePath(curDir, dstDir) {
				continue
			}
		}

		selectedPath := ""
		selectedRow := pane.table.Selected
		if sel := pane.selectedEntry(); sel != nil {
			selectedPath = sel.Path
		}

		if err := pane.load(pane.dir); err != nil {
			pane.setNotice(err.Error(), now)
			continue
		}

		paneDir := filepath.Clean(pane.dir)
		if remoteMove {
			paneDir = path.Clean(pane.dir)
		}
		if paneDir == dstDir {
			if idx := pane.findEntryPathIndex(dstPath); idx >= 0 {
				pane.table.SetSelected(idx, pane.model.Len(), false)
				continue
			}
		}

		if selectedPath != "" && !samePathFn(selectedPath, srcPath) {
			if idx := pane.findEntryPathIndex(selectedPath); idx >= 0 {
				pane.table.SetSelected(idx, pane.model.Len(), false)
				continue
			}
		}

		row := selectedRow
		if i == st.pane {
			row = st.row
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
					ui.closeFileMoveDialog()
				}
			case key.NameEnter, key.NameReturn:
				if !st.running {
					st.actionsAnim.setPulse("confirm", gtx.Now)
					ui.submitFileMoveDialog(gtx.Now)
				}
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
		width := gtx.Dp(unit.Dp(320))
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

	sourceHdr := material.Caption(th, "Source")
	sourceHdr.Font.Typeface = ui.mainTypeface()
	sourceHdr.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
	sourceHdr.Color = hintColor

	sourcePath := material.Body2(th, st.srcPath)
	sourcePath.Font.Typeface = ui.mainTypeface()
	sourcePath.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
	sourcePath.Color = txtColor
	sourcePath.MaxLines = 1
	sourcePath.Truncator = "…"

	dstHdr := material.Caption(th, "Destination")
	dstHdr.Font.Typeface = ui.mainTypeface()
	dstHdr.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
	dstHdr.Color = hintColor

	meta := formatCopyPathInfo(st.srcInfo)
	sameTarget := st.previewSameTarget()
	if st.dstInfo.Exists && !sameTarget {
		meta = "dst exists: " + formatCopyPathInfo(st.dstInfo)
	}
	metaLbl := material.Caption(th, meta)
	metaLbl.Font.Typeface = ui.mainTypeface()
	metaLbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
	metaLbl.Color = color.NRGBA{R: 184, G: 184, B: 184, A: 255}
	metaLbl.MaxLines = 1
	metaLbl.Truncator = "…"

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					title := material.Body1(th, "Rename / Move")
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
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(sourceHdr.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout),
		layout.Rigid(sourcePath.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(dstHdr.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.running {
				lbl := material.Body2(th, st.dstPath)
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
				lbl.Color = txtColor
				lbl.MaxLines = 1
				lbl.Truncator = "…"
				return fillRoundedBox(
					gtx,
					gtx.Dp(unit.Dp(filePaneControlCornerDp)),
					color.NRGBA{R: 24, G: 24, B: 24, A: 255},
					color.NRGBA{R: 255, G: 255, B: 255, A: 20},
					func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, lbl.Layout)
					},
				)
			}
			ed := material.Editor(th, &st.dstEdit, "")
			ed.Font.Typeface = ui.mainTypeface()
			ed.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
			ed.Color = txtColor
			ed.HintColor = hintColor
			return layoutNeutralEditorBox(gtx, gtx.Focused(&st.dstEdit), true, ed.Layout)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Rigid(metaLbl.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.lastErr == "" {
				actionLabel, runningLabel := st.actionLabels()
				if !st.running && (!st.dstInfo.Exists || sameTarget) {
					return layout.Dimensions{}
				}
				if st.running {
					lbl := material.Caption(th, runningLabel)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
					lbl.Color = hintColor
					return lbl.Layout(gtx)
				}
				lbl := material.Caption(th, "Destination for "+strings.ToLower(actionLabel)+" already exists.")
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
				lbl.Color = color.NRGBA{R: 196, G: 196, B: 196, A: 255}
				lbl.MaxLines = 2
				return lbl.Layout(gtx)
			}
			lbl := material.Caption(th, st.lastErr)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
			lbl.Color = color.NRGBA{R: 220, G: 140, B: 140, A: 255}
			lbl.MaxLines = 2
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
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
				)
			})
		}),
	)
}

func (st *fileMoveState) resolvedDestinationPath() string {
	if st == nil {
		return ""
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
