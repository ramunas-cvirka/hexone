package ui

import (
	"errors"
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
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type fileCreateKind uint8

const (
	fileCreateKindFolder fileCreateKind = iota
	fileCreateKindFile
)

type fileCreateState struct {
	pane int

	baseDir  string
	kind     fileCreateKind
	kindPrev fileCreateKind
	kindAt   time.Time

	targetRaw  string
	targetPath string
	targetInfo fileCopyPathInfo

	endpoint copyEndpoint
	remote   *paneSSHSession

	nameEdit     widget.Editor
	nameEditWant bool

	kindFolderClick widget.Clickable
	kindFileClick   widget.Clickable

	backdropClick widget.Clickable
	closeClick    widget.Clickable
	confirmClick  widget.Clickable
	cancelClick   widget.Clickable

	running bool
	lastErr string

	doneCh      chan error
	actionsAnim segmentedAnimState
	kindAnim    segmentedAnimState
}

func (ui *UI) startFileCreateDialog(idx int, now time.Time) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil {
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

	baseDir := strings.TrimSpace(pane.dir)
	if baseDir == "" {
		if remote != nil {
			baseDir = "/"
		} else {
			baseDir = "."
		}
	}

	st := &fileCreateState{
		pane:     idx,
		baseDir:  baseDir,
		kind:     fileCreateKindFolder,
		kindPrev: fileCreateKindFolder,
		remote:   remote,
		endpoint: copyEndpoint{pane: idx, remote: remote, dir: strings.TrimSpace(pane.dir)},
	}
	st.nameEdit.SingleLine = true
	st.nameEdit.Submit = true
	st.nameEdit.SetText("")
	st.nameEditWant = true

	ui.fileCreate = st
	ui.rep.active = false
	ui.rep.pane = -1
	ui.clearFileCreateHotkeyHold()
}

func (st *fileCreateState) setKind(next fileCreateKind, now time.Time) {
	if st == nil || st.kind == next {
		return
	}
	st.kindPrev = st.kind
	st.kindAt = now
	st.kind = next
}

func (st *fileCreateState) kindFill(now time.Time, key fileCreateKind) (float32, bool) {
	if st == nil {
		return 0, false
	}
	if st.kindAt.IsZero() || st.kindPrev == st.kind {
		if key == st.kind {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(st.kindAt)
	if elapsed >= toolbarAnimDur {
		st.kindPrev = st.kind
		st.kindAt = time.Time{}
		if key == st.kind {
			return 1, false
		}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarAnimDur))
	if key == st.kind {
		return t, true
	}
	if key == st.kindPrev {
		return 1 - t, true
	}
	return 0, true
}

func (ui *UI) clearFileCreateHotkeyHold() {
	if ui == nil || ui.held == nil {
		return
	}
	ui.held[fileActionKey(fileActionCreate)] = false
}

func (st *fileCreateState) refreshPreview() {
	if st == nil {
		return
	}
	raw := strings.TrimSpace(st.nameEdit.Text())
	st.targetRaw = raw
	st.targetPath = ""
	st.targetInfo = fileCopyPathInfo{}
	if raw == "" {
		return
	}

	target, err := resolveFileOpTargetPath(st.endpoint, st.baseDir, raw)
	if err != nil {
		return
	}
	st.targetPath = target

	info, err := endpointLstat(st.endpoint, target)
	if err != nil {
		return
	}
	st.targetInfo = fileCopyPathInfo{
		Path:    target,
		Exists:  true,
		IsDir:   info.IsDir(),
		ModTime: info.ModTime(),
	}
	if info.Mode().IsRegular() {
		st.targetInfo.Size = info.Size()
	}
}

func (ui *UI) submitFileCreateDialog(now time.Time) {
	st := ui.fileCreate
	if st == nil || st.running {
		return
	}

	raw := strings.TrimSpace(st.nameEdit.Text())
	if raw == "" {
		st.lastErr = "name/path is empty"
		return
	}
	target, err := resolveFileOpTargetPath(st.endpoint, st.baseDir, raw)
	if err != nil {
		st.lastErr = err.Error()
		return
	}
	if info, err := endpointLstat(st.endpoint, target); err == nil && info != nil {
		st.lastErr = "target already exists"
		st.refreshPreview()
		return
	}

	st.lastErr = ""
	st.targetRaw = raw
	st.targetPath = target
	st.running = true
	st.doneCh = make(chan error, 1)

	targetPath := st.targetPath
	kind := st.kind
	remote := st.remote
	doneCh := st.doneCh
	go func() {
		if remote != nil {
			client := remote.sftpClient()
			if client == nil {
				doneCh <- errors.New("sftp session is not connected")
				return
			}
			if kind == fileCreateKindFolder {
				doneCh <- client.Mkdir(targetPath)
				return
			}
			f, err := client.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY)
			if err != nil {
				doneCh <- err
				return
			}
			doneCh <- f.Close()
			return
		}

		if kind == fileCreateKindFolder {
			doneCh <- os.Mkdir(targetPath, 0o755)
			return
		}
		f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			doneCh <- err
			return
		}
		doneCh <- f.Close()
	}()

	_ = now
}

func (ui *UI) pumpFileCreateState(gtx layout.Context) {
	st := ui.fileCreate
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
		ui.finishFileCreate(gtx.Now)
	default:
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) finishFileCreate(now time.Time) {
	st := ui.fileCreate
	if st == nil {
		return
	}

	remoteCreate := st.remote != nil
	var remoteSetup fm.SSHSetup
	if remoteCreate {
		remoteSetup = st.remote.setup
	}
	createdPath := filepath.Clean(st.targetPath)
	createdDir := filepath.Clean(filepath.Dir(createdPath))
	if remoteCreate {
		createdPath = path.Clean(st.targetPath)
		createdDir = path.Clean(path.Dir(createdPath))
	}

	if st.remote != nil {
		st.remote.close()
		st.remote = nil
	}
	ui.fileCreate = nil
	ui.clearFileCreateHotkeyHold()

	for _, pane := range ui.filePanes {
		if pane == nil || pane.model == nil || pane.table == nil {
			continue
		}
		if remoteCreate {
			if !pane.remoteConnected() || pane.remote == nil || !sameSSHRemoteTarget(pane.remote.setup, remoteSetup) {
				continue
			}
			curDir := path.Clean(pane.dir)
			if curDir != createdDir {
				continue
			}
		} else {
			if pane.remoteConnected() {
				continue
			}
			curDir := filepath.Clean(pane.dir)
			if !samePath(curDir, createdDir) {
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

		if idx := pane.findEntryPathIndex(createdPath); idx >= 0 {
			pane.table.SetSelected(idx, pane.model.Len(), false)
			continue
		}
		if selectedPath != "" {
			if idx := pane.findEntryPathIndex(selectedPath); idx >= 0 {
				pane.table.SetSelected(idx, pane.model.Len(), false)
				continue
			}
		}

		row := selectedRow
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

func (ui *UI) closeFileCreateDialog() {
	st := ui.fileCreate
	if st != nil && st.remote != nil {
		st.remote.close()
		st.remote = nil
	}
	ui.fileCreate = nil
	ui.clearFileCreateHotkeyHold()
}

func (ui *UI) layoutFileCreateDialog(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.fileCreate
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
					ui.closeFileCreateDialog()
				}
			case key.NameEnter, key.NameReturn:
				if !st.running {
					st.actionsAnim.setPulse("confirm", gtx.Now)
					ui.submitFileCreateDialog(gtx.Now)
				}
			}
		}
	}

	if !st.running {
		for {
			ev, ok := st.nameEdit.Update(gtx)
			if !ok {
				break
			}
			submit, ok := ev.(widget.SubmitEvent)
			if ok {
				st.nameEdit.SetText(submit.Text)
				st.actionsAnim.setPulse("confirm", gtx.Now)
				ui.submitFileCreateDialog(gtx.Now)
				continue
			}
			if _, ok := ev.(widget.ChangeEvent); ok {
				st.lastErr = ""
				st.refreshPreview()
			}
		}
	}

	if st.kindFolderClick.Clicked(gtx) && !st.running {
		st.setKind(fileCreateKindFolder, gtx.Now)
		st.kindAnim.setPulse("folder", gtx.Now)
	}
	if st.kindFileClick.Clicked(gtx) && !st.running {
		st.setKind(fileCreateKindFile, gtx.Now)
		st.kindAnim.setPulse("file", gtx.Now)
	}

	if st.cancelClick.Clicked(gtx) && !st.running {
		ui.closeFileCreateDialog()
		return layout.Dimensions{}
	}
	if st.closeClick.Clicked(gtx) && !st.running {
		ui.closeFileCreateDialog()
		return layout.Dimensions{}
	}
	if st.confirmClick.Clicked(gtx) && !st.running {
		st.actionsAnim.setPulse("confirm", gtx.Now)
		ui.submitFileCreateDialog(gtx.Now)
	}
	if st.running {
		for st.closeClick.Clicked(gtx) {
		}
	}
	for st.backdropClick.Clicked(gtx) {
	}

	if st.nameEditWant && !st.running {
		st.nameEditWant = false
		gtx.Execute(key.FocusCmd{Tag: &st.nameEdit})
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
						return ui.layoutFileCreateDialogBody(th, gtx, st)
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

func (ui *UI) layoutFileCreateDialogBody(th *material.Theme, gtx layout.Context, st *fileCreateState) layout.Dimensions {
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

	hoverKindKey := ""
	if !st.running && st.kindFolderClick.Hovered() {
		hoverKindKey = "folder"
	}
	if !st.running && st.kindFileClick.Hovered() {
		hoverKindKey = "file"
	}
	st.kindAnim.setHover(hoverKindKey, gtx.Now)
	hoverFolder, hoverFolderAnim := st.kindAnim.hoverFill(gtx.Now, "folder")
	hoverFile, hoverFileAnim := st.kindAnim.hoverFill(gtx.Now, "file")
	pulseFolder, pulseFolderAnim := st.kindAnim.pulseFill(gtx.Now, "folder")
	pulseFile, pulseFileAnim := st.kindAnim.pulseFill(gtx.Now, "file")
	fillFolder, fillFolderAnim := st.kindFill(gtx.Now, fileCreateKindFolder)
	fillFile, fillFileAnim := st.kindFill(gtx.Now, fileCreateKindFile)
	if hoverFolderAnim || hoverFileAnim || pulseFolderAnim || pulseFileAnim || fillFolderAnim || fillFileAnim {
		gtx.Execute(op.InvalidateCmd{})
	}

	pathInfo := st.targetPath
	if pathInfo == "" {
		pathInfo = st.baseDir
	}
	if st.targetInfo.Exists {
		pathInfo = pathInfo + " (exists)"
	}

	kindTitle := "Folder"
	confirmLabel := "Create"
	if st.kind == fileCreateKindFile {
		kindTitle = "File"
	}
	if st.running {
		confirmLabel = "Creating..."
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					title := material.Body1(th, "Create")
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
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, "Type")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileCreateKindTabs(
				th, gtx, st,
				fillFolder, fillFile,
				hoverFolder, hoverFile,
				pulseFolder, pulseFile,
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, kindTitle+" name/path")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.running {
				lbl := material.Body2(th, strings.TrimSpace(st.targetPath))
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
			ed := material.Editor(th, &st.nameEdit, "")
			ed.Font.Typeface = ui.mainTypeface()
			ed.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
			ed.Color = txtColor
			ed.HintColor = hintColor
			return layoutNeutralEditorBox(gtx, gtx.Focused(&st.nameEdit), true, ed.Layout)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, pathInfo)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
			lbl.Color = color.NRGBA{R: 184, G: 184, B: 184, A: 255}
			lbl.MaxLines = 1
			lbl.Truncator = "…"
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.lastErr == "" {
				if st.running {
					lbl := material.Caption(th, "Creating...")
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
					lbl.Color = hintColor
					return lbl.Layout(gtx)
				}
				return layout.Dimensions{}
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
				return ui.layoutDialogActionPair(
					th, gtx,
					&st.cancelClick, "Cancel", hoverCancel, pulseCancel, st.running,
					&st.confirmClick, confirmLabel, hoverConfirm, pulseConfirm, st.running,
				)
			})
		}),
	)
}

func (st *fileCreateState) kindPosition(now time.Time) (float32, bool) {
	if st == nil {
		return 0, false
	}
	current := float32(0)
	if st.kind == fileCreateKindFile {
		current = 1
	}
	if st.kindAt.IsZero() || st.kindPrev == st.kind {
		return current, false
	}
	prev := float32(0)
	if st.kindPrev == fileCreateKindFile {
		prev = 1
	}
	elapsed := now.Sub(st.kindAt)
	if elapsed >= toolbarAnimDur {
		st.kindPrev = st.kind
		st.kindAt = time.Time{}
		return current, false
	}
	t := smoothstep01(clamp01(float32(elapsed) / float32(toolbarAnimDur)))
	return prev + (current-prev)*t, true
}

func (ui *UI) layoutFileCreateKindTabs(th *material.Theme, gtx layout.Context, st *fileCreateState, fillFolder, fillFile, hoverFolder, hoverFile, pulseFolder, pulseFile float32) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	stripH := gtx.Dp(unit.Dp(22))
	if stripH < 1 {
		stripH = 1
	}
	pos, animPos := st.kindPosition(gtx.Now)
	if animPos {
		gtx.Execute(op.InvalidateCmd{})
	}
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 24, G: 24, B: 24, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 22},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
					w := gtx.Constraints.Max.X
					if w < 2 {
						w = 2
					}
					innerR := gtx.Dp(unit.Dp(filePaneControlCornerDp - 1))
					if innerR < 1 {
						innerR = 1
					}
					half := w / 2
					if half < 1 {
						half = 1
					}
					sliderX := int(float32(half) * pos)
					if sliderX < 0 {
						sliderX = 0
					}
					if sliderX > w-half {
						sliderX = w - half
					}
					sliderRect := image.Rect(sliderX, 0, sliderX+half, stripH)
					innerClip := clip.UniformRRect(image.Rect(0, 0, w, stripH), innerR).Push(gtx.Ops)
					paint.FillShape(gtx.Ops, color.NRGBA{R: 54, G: 54, B: 54, A: 255}, clip.UniformRRect(sliderRect, innerR).Op(gtx.Ops))

					dims := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, half, func(gtx layout.Context) layout.Dimensions {
								return st.kindFolderClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									active := clamp01(fillFolder*0.8 + (1-pos)*0.45)
									fg := mixNRGBA(txtColor, color.NRGBA{R: 236, G: 236, B: 236, A: 255}, active)
									fg = mixNRGBA(fg, color.NRGBA{R: 244, G: 244, B: 244, A: 255}, clamp01(hoverFolder*0.75+pulseFolder*0.25))
									return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										lbl := material.Body2(th, "Folder")
										lbl.Font.Typeface = ui.mainTypeface()
										lbl.Font.Weight = font.Medium
										lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
										lbl.Color = fg
										lbl.MaxLines = 1
										dims := lbl.Layout(gtx)
										defer clip.Rect(image.Rectangle{Max: image.Pt(half, stripH)}).Push(gtx.Ops).Pop()
										pointer.CursorPointer.Add(gtx.Ops)
										return dims
									})
								})
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, w-half, func(gtx layout.Context) layout.Dimensions {
								return st.kindFileClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									active := clamp01(fillFile*0.8 + pos*0.45)
									fg := mixNRGBA(txtColor, color.NRGBA{R: 236, G: 236, B: 236, A: 255}, active)
									fg = mixNRGBA(fg, color.NRGBA{R: 244, G: 244, B: 244, A: 255}, clamp01(hoverFile*0.75+pulseFile*0.25))
									return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										lbl := material.Body2(th, "File")
										lbl.Font.Typeface = ui.mainTypeface()
										lbl.Font.Weight = font.Medium
										lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
										lbl.Color = fg
										lbl.MaxLines = 1
										dims := lbl.Layout(gtx)
										defer clip.Rect(image.Rectangle{Max: image.Pt(w-half, stripH)}).Push(gtx.Ops).Pop()
										pointer.CursorPointer.Add(gtx.Ops)
										return dims
									})
								})
							})
						}),
					)
					innerClip.Pop()
					return dims
				})
			})
		},
	)
}
