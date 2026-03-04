package ui

import (
	"fmt"
	"hexone/filesys"
	"image"
	"image/color"
	"os"
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

type fileCopyState struct {
	pane int

	srcPath string
	dstPath string
	dstRaw  string

	dstEdit     widget.Editor
	dstEditWant bool

	backdropClick widget.Clickable
	confirmClick  widget.Clickable
	cancelClick   widget.Clickable

	running  bool
	progress filesys.CopyProgress
	lastErr  string

	srcInfo fileCopyPathInfo
	dstInfo fileCopyPathInfo

	progressCh chan filesys.CopyProgress
	doneCh     chan error
}

type fileCopyPathInfo struct {
	Path    string
	Exists  bool
	IsDir   bool
	Size    int64
	ModTime time.Time
}

func (ui *UI) startFileCopyDialog(idx int, now time.Time) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return
	}

	entry := pane.selectedEntry()
	if entry == nil || entry.Path == "" {
		pane.setNotice("nothing selected to copy", now)
		return
	}
	if entry.Kind == filesys.EntryParent {
		pane.setNotice("cannot copy parent entry", now)
		return
	}

	ui.setActiveFilePane(idx)
	pane.stopPathEdit()
	pane.sortMenuOpen = false
	pane.closeContextMenu()
	ui.closeSortMenusExcept(idx)
	ui.closeContextMenusExcept(idx)

	dstDir := pane.dir
	for i, other := range ui.filePanes {
		if i == idx || other == nil || other.dir == "" {
			continue
		}
		dstDir = other.dir
		break
	}
	dstDefault := filepath.Join(dstDir, filepath.Base(entry.Path))

	st := &fileCopyState{
		pane:    idx,
		srcPath: entry.Path,
		dstPath: dstDefault,
		dstRaw:  dstDefault,
	}
	st.dstEdit.SingleLine = true
	st.dstEdit.Submit = true
	st.dstEdit.SetText(dstDefault)
	st.dstEdit.SetCaret(st.dstEdit.Len(), st.dstEdit.Len())
	st.dstEditWant = true
	st.refreshPreview()

	ui.fileCopy = st
	ui.rep.active = false
	ui.rep.pane = -1
	ui.clearFileCopyHotkeyHold()
}

func (ui *UI) clearFileCopyHotkeyHold() {
	if ui == nil || ui.held == nil {
		return
	}
	ui.held[fileActionKey(fileActionCopy)] = false
}

func (st *fileCopyState) refreshPreview() {
	if st == nil {
		return
	}
	raw := strings.TrimSpace(st.dstEdit.Text())
	st.dstRaw = raw
	if raw == "" {
		st.dstPath = ""
		st.dstInfo = fileCopyPathInfo{}
		return
	}

	effectiveDst, srcInfo, dstInfo, err := inspectCopyPaths(st.srcPath, raw)
	if err != nil {
		st.dstPath = raw
		return
	}
	st.dstPath = effectiveDst
	st.srcInfo = srcInfo
	st.dstInfo = dstInfo
}

func inspectCopyPaths(srcPath, dstRaw string) (string, fileCopyPathInfo, fileCopyPathInfo, error) {
	srcAbs, err := filepath.Abs(srcPath)
	if err != nil {
		return "", fileCopyPathInfo{}, fileCopyPathInfo{}, err
	}
	srcAbs = filepath.Clean(srcAbs)

	srcStat, err := os.Lstat(srcAbs)
	if err != nil {
		return "", fileCopyPathInfo{}, fileCopyPathInfo{}, err
	}

	dstAbs, err := filepath.Abs(dstRaw)
	if err != nil {
		return "", fileCopyPathInfo{}, fileCopyPathInfo{}, err
	}
	dstAbs = filepath.Clean(dstAbs)

	if dstDirInfo, err := os.Stat(dstAbs); err == nil && dstDirInfo.IsDir() {
		dstAbs = filepath.Join(dstAbs, filepath.Base(srcAbs))
	}

	srcInfo := fileCopyPathInfo{
		Path:    srcAbs,
		Exists:  true,
		IsDir:   srcStat.IsDir(),
		ModTime: srcStat.ModTime(),
	}
	if srcStat.Mode().IsRegular() {
		srcInfo.Size = srcStat.Size()
	}

	dstInfo := fileCopyPathInfo{Path: dstAbs}
	if dstStat, err := os.Lstat(dstAbs); err == nil {
		dstInfo.Exists = true
		dstInfo.IsDir = dstStat.IsDir()
		dstInfo.ModTime = dstStat.ModTime()
		if dstStat.Mode().IsRegular() {
			dstInfo.Size = dstStat.Size()
		}
	}
	return dstAbs, srcInfo, dstInfo, nil
}

func (ui *UI) submitFileCopyDialog(now time.Time) {
	st := ui.fileCopy
	if st == nil || st.running {
		return
	}

	dst := strings.TrimSpace(st.dstEdit.Text())
	if dst == "" {
		st.lastErr = "destination path is empty"
		return
	}

	st.dstRaw = dst
	effectiveDst, srcInfo, dstInfo, err := inspectCopyPaths(st.srcPath, dst)
	if err != nil {
		st.lastErr = err.Error()
		return
	}
	st.srcInfo = srcInfo
	st.dstInfo = dstInfo
	st.dstPath = effectiveDst

	st.lastErr = ""
	st.progress = filesys.CopyProgress{}
	st.running = true

	progressCh := make(chan filesys.CopyProgress, 32)
	doneCh := make(chan error, 1)
	st.progressCh = progressCh
	st.doneCh = doneCh

	src := st.srcPath
	go func() {
		sendProgress := func(p filesys.CopyProgress) {
			for {
				select {
				case progressCh <- p:
					return
				default:
				}
				select {
				case <-progressCh:
				default:
				}
			}
		}
		doneCh <- filesys.CopyPath(src, dst, sendProgress)
	}()

	_ = now
}

func (ui *UI) pumpFileCopyState(gtx layout.Context) {
	st := ui.fileCopy
	if st == nil {
		return
	}

	if st.running {
		for {
			select {
			case p := <-st.progressCh:
				st.progress = p
			default:
				goto doneProgress
			}
		}
	doneProgress:
		select {
		case err := <-st.doneCh:
			st.running = false
			st.progressCh = nil
			st.doneCh = nil
			if err != nil {
				st.lastErr = err.Error()
			} else {
				ui.finishFileCopy(gtx.Now)
				return
			}
		default:
		}
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) finishFileCopy(now time.Time) {
	st := ui.fileCopy
	if st == nil {
		return
	}

	srcPath := st.srcPath
	dstPath := st.dstPath
	ui.fileCopy = nil // close dialog first
	ui.clearFileCopyHotkeyHold()

	srcDir := filepath.Clean(filepath.Dir(srcPath))
	dstDir := filepath.Clean(filepath.Dir(dstPath))
	if info, err := os.Stat(dstPath); err == nil && info.IsDir() {
		dstDir = filepath.Clean(dstPath)
	}

	for _, pane := range ui.filePanes {
		if pane == nil {
			continue
		}
		cur := filepath.Clean(pane.dir)
		if !samePath(cur, srcDir) && !samePath(cur, dstDir) {
			continue
		}
		selectedPath := ""
		if sel := pane.selectedEntry(); sel != nil {
			selectedPath = sel.Path
		}
		if err := pane.load(pane.dir); err != nil {
			pane.setNotice(err.Error(), now)
			continue
		}
		if selectedPath != "" && pane.table != nil && pane.model != nil {
			if idx := pane.findEntryPathIndex(selectedPath); idx >= 0 {
				pane.table.SetSelected(idx, pane.model.Len(), false)
			}
		}
	}
}

func (ui *UI) layoutFileCopyDialog(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.fileCopy
	if st == nil {
		return layout.Dimensions{}
	}

	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		if !st.running {
			ui.fileCopy = nil
			ui.clearFileCopyHotkeyHold()
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
				ui.submitFileCopyDialog(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			if _, ok := ev.(widget.ChangeEvent); ok {
				st.lastErr = ""
				st.refreshPreview()
				continue
			}
		}
		if st.cancelClick.Clicked(gtx) {
			ui.fileCopy = nil
			ui.clearFileCopyHotkeyHold()
			return layout.Dimensions{}
		}
		if st.confirmClick.Clicked(gtx) {
			ui.submitFileCopyDialog(gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		}
	} else {
		for st.cancelClick.Clicked(gtx) {
		}
		for st.confirmClick.Clicked(gtx) {
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

		width := gtx.Dp(unit.Dp(320))
		maxWidth := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(16))
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
				color.NRGBA{R: 20, G: 24, B: 34, A: 252},
				color.NRGBA{R: 255, G: 255, B: 255, A: 28},
				func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutFileCopyDialogBody(th, gtx, st)
					})
				},
			)
		})
		call := m.Stop()

		x := (gtx.Constraints.Max.X - dialog.Size.X) / 2
		if x < 0 {
			x = 0
		}
		y := (gtx.Constraints.Max.Y - dialog.Size.Y) / 2
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

func (ui *UI) layoutFileCopyDialogBody(th *material.Theme, gtx layout.Context, st *fileCopyState) layout.Dimensions {
	title := material.Body1(th, "Copy")
	title.Font.Typeface = ui.mainTypeface()
	title.Font.Weight = font.Bold
	title.TextSize = scaleThemeFontSize(th, 12)
	title.Color = txtColor

	srcHdr := material.Caption(th, "Source")
	srcHdr.Font.Typeface = ui.mainTypeface()
	srcHdr.TextSize = scaleThemeFontSize(th, 9)
	srcHdr.Color = hintColor

	srcText := material.Body2(th, st.srcPath)
	srcText.Font.Typeface = ui.mainTypeface()
	srcText.TextSize = scaleThemeFontSize(th, 10)
	srcText.Color = color.NRGBA{R: 210, G: 220, B: 245, A: 255}
	srcText.MaxLines = 1
	srcText.Truncator = "…"

	dstHdr := material.Caption(th, "Destination")
	dstHdr.Font.Typeface = ui.mainTypeface()
	dstHdr.TextSize = scaleThemeFontSize(th, 9)
	dstHdr.Color = hintColor

	progress := st.progress
	status := copyProgressText(progress)
	current := copyProgressCurrent(progress)
	progressFrac := copyProgressFraction(progress)
	overwriteLabel := ""
	if !st.running && st.dstInfo.Exists {
		overwriteLabel = "Destination exists. Overwrite will replace it."
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(title.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(srcHdr.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout),
		layout.Rigid(srcText.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(dstHdr.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.running {
				lbl := material.Body2(th, st.dstPath)
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleThemeFontSize(th, 10)
				lbl.Color = txtColor
				lbl.MaxLines = 1
				lbl.Truncator = "…"
				return fillRoundedBox(
					gtx,
					gtx.Dp(unit.Dp(filePaneControlCornerDp)),
					color.NRGBA{R: 18, G: 22, B: 30, A: 255},
					color.NRGBA{R: 255, G: 255, B: 255, A: 20},
					func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, lbl.Layout)
					},
				)
			}
			ed := material.Editor(th, &st.dstEdit, "")
			ed.Font.Typeface = ui.mainTypeface()
			ed.TextSize = scaleThemeFontSize(th, 10)
			ed.Color = txtColor
			ed.HintColor = hintColor
			return fillRoundedBox(
				gtx,
				gtx.Dp(unit.Dp(filePaneControlCornerDp)),
				color.NRGBA{R: 18, G: 22, B: 30, A: 255},
				color.NRGBA{R: 110, G: 132, B: 190, A: 120},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, ed.Layout)
				},
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.running || !st.dstInfo.Exists {
				return layout.Dimensions{}
			}
			return ui.layoutFileCopyOverwriteInfo(th, gtx, st)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if overwriteLabel == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, overwriteLabel)
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleThemeFontSize(th, 9)
				lbl.Color = color.NRGBA{R: 245, G: 195, B: 120, A: 255}
				lbl.MaxLines = 2
				return lbl.Layout(gtx)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !st.running && st.lastErr == "" {
				return layout.Dimensions{}
			}
			if st.running {
				return ui.layoutFileCopyProgress(th, gtx, progressFrac, status, current)
			}
			lbl := material.Body2(th, st.lastErr)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleThemeFontSize(th, 10)
			lbl.Color = color.NRGBA{R: 240, G: 100, B: 100, A: 255}
			lbl.MaxLines = 2
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layoutFileCopyDialogButton(th, gtx, ui.mainTypeface(), &st.cancelClick, "Cancel", false, st.running)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := "Copy"
						if st.running {
							label = "Copying..."
						} else if st.dstInfo.Exists {
							label = "Overwrite"
						}
						return layoutFileCopyDialogButton(th, gtx, ui.mainTypeface(), &st.confirmClick, label, true, st.running)
					}),
				)
			})
		}),
	)
}

func (ui *UI) layoutFileCopyProgress(th *material.Theme, gtx layout.Context, frac float32, status, current string) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutProgressBar(gtx, frac)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, status)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleThemeFontSize(th, 9)
			lbl.Color = hintColor
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if current == "" {
				return layout.Dimensions{}
			}
			lbl := material.Caption(th, current)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleThemeFontSize(th, 9)
			lbl.Color = color.NRGBA{R: 170, G: 180, B: 205, A: 255}
			lbl.MaxLines = 1
			lbl.Truncator = "…"
			return lbl.Layout(gtx)
		}),
	)
}

func (ui *UI) layoutFileCopyOverwriteInfo(th *material.Theme, gtx layout.Context, st *fileCopyState) layout.Dimensions {
	srcMeta := formatCopyPathInfo(st.srcInfo)
	dstMeta := formatCopyPathInfo(st.dstInfo)
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 28, G: 26, B: 20, A: 255},
		color.NRGBA{R: 160, G: 130, B: 76, A: 120},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "Overwrite Details")
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleThemeFontSize(th, 9)
						lbl.Color = color.NRGBA{R: 236, G: 210, B: 162, A: 255}
						return lbl.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "src: "+srcMeta)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleThemeFontSize(th, 9)
						lbl.Color = color.NRGBA{R: 210, G: 220, B: 245, A: 255}
						lbl.MaxLines = 1
						lbl.Truncator = "…"
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "dst: "+dstMeta)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleThemeFontSize(th, 9)
						lbl.Color = color.NRGBA{R: 240, G: 200, B: 170, A: 255}
						lbl.MaxLines = 1
						lbl.Truncator = "…"
						return lbl.Layout(gtx)
					}),
				)
			})
		},
	)
}

func layoutFileCopyDialogButton(th *material.Theme, gtx layout.Context, typeface font.Typeface, click *widget.Clickable, label string, primary, disabled bool) layout.Dimensions {
	if click == nil {
		return layout.Dimensions{}
	}
	return fixedWidth(gtx, gtx.Dp(unit.Dp(82)), func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			bg := color.NRGBA{R: 30, G: 36, B: 50, A: 255}
			border := color.NRGBA{R: 255, G: 255, B: 255, A: 24}
			fg := txtColor
			if primary {
				bg = color.NRGBA{R: 68, G: 96, B: 186, A: 255}
				border = color.NRGBA{R: 120, G: 150, B: 255, A: 96}
				fg = color.NRGBA{R: 238, G: 244, B: 255, A: 255}
			}
			if click.Hovered() && !disabled {
				if primary {
					bg = color.NRGBA{R: 76, G: 104, B: 194, A: 255}
				} else {
					bg = color.NRGBA{R: 36, G: 42, B: 58, A: 255}
				}
			}
			if disabled {
				bg.A = 160
				border.A = 40
				fg = color.NRGBA{R: 170, G: 170, B: 180, A: 255}
			}
			return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), bg, border, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, label)
					lbl.Font.Typeface = typeface
					lbl.Font.Weight = font.Medium
					lbl.TextSize = scaleThemeFontSize(th, 10)
					lbl.Color = fg
					lbl.MaxLines = 1
					return layout.Center.Layout(gtx, lbl.Layout)
				})
			})
		})
	})
}

func layoutProgressBar(gtx layout.Context, frac float32) layout.Dimensions {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}

	height := gtx.Dp(unit.Dp(7))
	if height < 2 {
		height = 2
	}
	width := gtx.Constraints.Max.X
	if width < 1 {
		width = 1
	}
	fillW := int(float32(width) * frac)
	if fillW < 0 {
		fillW = 0
	}
	if fillW > width {
		fillW = width
	}

	bg := image.Rect(0, 0, width, height)
	paint.FillShape(gtx.Ops, color.NRGBA{R: 34, G: 40, B: 56, A: 255}, clip.Rect(bg).Op())
	if fillW > 0 {
		fg := image.Rect(0, 0, fillW, height)
		paint.FillShape(gtx.Ops, color.NRGBA{R: 110, G: 150, B: 245, A: 255}, clip.Rect(fg).Op())
	}
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func copyProgressFraction(progress filesys.CopyProgress) float32 {
	if progress.BytesTotal > 0 {
		return float32(progress.BytesDone) / float32(progress.BytesTotal)
	}
	if progress.EntriesTotal > 0 {
		return float32(progress.EntriesDone) / float32(progress.EntriesTotal)
	}
	return 0
}

func copyProgressText(progress filesys.CopyProgress) string {
	if progress.EntriesTotal <= 0 && progress.BytesTotal <= 0 {
		return "Preparing..."
	}
	if progress.BytesTotal > 0 {
		return fmt.Sprintf(
			"%d/%d entries  •  %s / %s",
			progress.EntriesDone,
			progress.EntriesTotal,
			formatCopySize(progress.BytesDone),
			formatCopySize(progress.BytesTotal),
		)
	}
	return fmt.Sprintf("%d/%d entries", progress.EntriesDone, progress.EntriesTotal)
}

func copyProgressCurrent(progress filesys.CopyProgress) string {
	if progress.CurrentPath == "" {
		return ""
	}
	return filepath.Base(progress.CurrentPath)
}

func formatCopySize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}

	type unitDef struct {
		name string
		size int64
	}
	units := []unitDef{
		{name: "TB", size: 1 << 40},
		{name: "GB", size: 1 << 30},
		{name: "MB", size: 1 << 20},
		{name: "KB", size: 1 << 10},
	}
	for _, u := range units {
		if size < u.size {
			continue
		}
		whole := (size * 10) / u.size
		return fmt.Sprintf("%d.%d %s", whole/10, whole%10, u.name)
	}
	return fmt.Sprintf("%d B", size)
}

func formatCopyPathInfo(info fileCopyPathInfo) string {
	if !info.Exists {
		return "missing"
	}
	if info.IsDir {
		if info.ModTime.IsZero() {
			return "dir"
		}
		return "dir, " + info.ModTime.Format("2006-01-02 15:04:05")
	}
	ts := "n/a"
	if !info.ModTime.IsZero() {
		ts = info.ModTime.Format("2006-01-02 15:04:05")
	}
	return formatCopySize(info.Size) + ", " + ts
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if os.PathSeparator == '\\' {
		return strings.EqualFold(a, b)
	}
	return a == b
}
