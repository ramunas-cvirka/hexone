package ui

import (
	"fmt"
	"hexone/filesys"
	"image"
	"image/color"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

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

type fileCopyState struct {
	pane    int
	srcPane int
	dstPane int

	srcPath string
	dstPath string
	dstRaw  string

	srcEndpoint copyEndpoint
	dstEndpoint copyEndpoint

	dstEdit     widget.Editor
	dstEditWant bool

	backdropClick widget.Clickable
	closeClick    widget.Clickable
	confirmClick  widget.Clickable
	cancelClick   widget.Clickable

	running  bool
	progress filesys.CopyProgress
	lastErr  string

	srcInfo fileCopyPathInfo
	dstInfo fileCopyPathInfo

	progressCh  chan filesys.CopyProgress
	doneCh      chan error
	actionsAnim segmentedAnimState
}

type fileCopyPathInfo struct {
	Path    string
	Exists  bool
	IsDir   bool
	Size    int64
	ModTime time.Time
}

type segmentedAnimState struct {
	hoverKey  string
	hoverPrev string
	hoverAt   time.Time
	pulseKey  string
	pulseAt   time.Time
}

func (st *segmentedAnimState) setHover(key string, now time.Time) {
	if st == nil || st.hoverKey == key {
		return
	}
	st.hoverPrev = st.hoverKey
	st.hoverKey = key
	st.hoverAt = now
}

func (st *segmentedAnimState) hoverFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" {
		return 0, false
	}
	if st.hoverAt.IsZero() || st.hoverPrev == st.hoverKey {
		if st.hoverKey == key {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(st.hoverAt)
	if elapsed >= toolbarHoverDur {
		st.hoverPrev = ""
		st.hoverAt = time.Time{}
		if st.hoverKey == key {
			return 1, false
		}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarHoverDur))
	if key == st.hoverKey {
		return t, true
	}
	if key == st.hoverPrev {
		return 1 - t, true
	}
	return 0, true
}

func (st *segmentedAnimState) setPulse(key string, now time.Time) {
	if st == nil || key == "" {
		return
	}
	st.pulseKey = key
	st.pulseAt = now
}

func (st *segmentedAnimState) pulseFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" || st.pulseKey != key || st.pulseAt.IsZero() {
		return 0, false
	}
	elapsed := now.Sub(st.pulseAt)
	if elapsed >= toolbarClickDur {
		st.pulseKey = ""
		st.pulseAt = time.Time{}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarClickDur))
	return 1 - t, true
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
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	ui.closeSortMenusExcept(idx)
	ui.closeFavoriteMenusExcept(idx)
	ui.closeContextMenusExcept(idx)

	dstPaneIdx := idx
	for i, other := range ui.filePanes {
		if i == idx || other == nil || other.dir == "" {
			continue
		}
		dstPaneIdx = i
		break
	}
	dstPane := ui.filePanes[dstPaneIdx]
	srcEndpoint := copyEndpointFromPane(idx, pane)
	dstEndpoint := copyEndpointFromPane(dstPaneIdx, dstPane)
	dstDir := strings.TrimSpace(dstEndpoint.dir)
	if dstDir == "" {
		if dstEndpoint.isRemote() {
			dstDir = "/"
		} else {
			dstDir = "."
		}
	}
	dstDefault := dstEndpoint.join(dstDir, srcEndpoint.baseName(entry.Path))

	st := &fileCopyState{
		pane:        idx,
		srcPane:     idx,
		dstPane:     dstPaneIdx,
		srcPath:     entry.Path,
		dstPath:     dstDefault,
		dstRaw:      dstDefault,
		srcEndpoint: srcEndpoint,
		dstEndpoint: dstEndpoint,
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

	effectiveDst, srcInfo, dstInfo, err := inspectCopyPaths(st.srcEndpoint, st.srcPath, st.dstEndpoint, raw)
	if err != nil {
		st.dstPath = raw
		return
	}
	st.dstPath = effectiveDst
	st.srcInfo = srcInfo
	st.dstInfo = dstInfo
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
	effectiveDst, srcInfo, dstInfo, err := inspectCopyPaths(st.srcEndpoint, st.srcPath, st.dstEndpoint, dst)
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
	dst = st.dstPath
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
		doneCh <- runCopyBetweenEndpoints(st.srcEndpoint, src, st.dstEndpoint, dst, sendProgress)
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

	srcPaneIdx := st.srcPane
	dstPaneIdx := st.dstPane
	ui.fileCopy = nil // close dialog first
	ui.clearFileCopyHotkeyHold()

	reloadPane := func(paneIdx int) {
		if paneIdx < 0 || paneIdx >= len(ui.filePanes) {
			return
		}
		pane := ui.filePanes[paneIdx]
		if pane == nil {
			return
		}
		selectedPath := ""
		if sel := pane.selectedEntry(); sel != nil {
			selectedPath = sel.Path
		}
		if err := pane.load(pane.dir); err != nil {
			pane.setNotice(err.Error(), now)
			return
		}
		if selectedPath != "" && pane.table != nil && pane.model != nil {
			if idx := pane.findEntryPathIndex(selectedPath); idx >= 0 {
				pane.table.SetSelected(idx, pane.model.Len(), false)
			}
		}
	}
	reloadPane(srcPaneIdx)
	if dstPaneIdx != srcPaneIdx {
		reloadPane(dstPaneIdx)
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
		if st.closeClick.Clicked(gtx) {
			ui.fileCopy = nil
			ui.clearFileCopyHotkeyHold()
			return layout.Dimensions{}
		}
		if st.confirmClick.Clicked(gtx) {
			st.actionsAnim.setPulse("confirm", gtx.Now)
			ui.submitFileCopyDialog(gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		}
	} else {
		for st.cancelClick.Clicked(gtx) {
		}
		for st.closeClick.Clicked(gtx) {
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
				color.NRGBA{R: 20, G: 20, B: 20, A: 252},
				color.NRGBA{R: 255, G: 255, B: 255, A: 18},
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

	srcHdr := material.Caption(th, "Source")
	srcHdr.Font.Typeface = ui.mainTypeface()
	srcHdr.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
	srcHdr.Color = hintColor

	srcText := material.Body2(th, st.srcPath)
	srcText.Font.Typeface = ui.mainTypeface()
	srcText.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
	srcText.Color = color.NRGBA{R: 220, G: 220, B: 220, A: 255}
	srcText.MaxLines = 1
	srcText.Truncator = "…"

	dstHdr := material.Caption(th, "Destination")
	dstHdr.Font.Typeface = ui.mainTypeface()
	dstHdr.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
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
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					title := material.Body1(th, "Copy")
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
				lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
				lbl.Color = color.NRGBA{R: 196, G: 196, B: 196, A: 255}
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
			lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
			lbl.Color = color.NRGBA{R: 220, G: 140, B: 140, A: 255}
			lbl.MaxLines = 2
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := "Copy"
				if st.running {
					label = "Copying..."
				} else if st.dstInfo.Exists {
					label = "Overwrite"
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

func (ui *UI) layoutFileCopyProgress(th *material.Theme, gtx layout.Context, frac float32, status, current string) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutProgressBar(gtx, frac)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, status)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
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
			lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
			lbl.Color = color.NRGBA{R: 168, G: 168, B: 168, A: 255}
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
		color.NRGBA{R: 24, G: 24, B: 24, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "Overwrite Details")
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
						lbl.Color = color.NRGBA{R: 208, G: 208, B: 208, A: 255}
						return lbl.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "src: "+srcMeta)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
						lbl.Color = color.NRGBA{R: 184, G: 184, B: 184, A: 255}
						lbl.MaxLines = 1
						lbl.Truncator = "…"
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "dst: "+dstMeta)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
						lbl.Color = color.NRGBA{R: 184, G: 184, B: 184, A: 255}
						lbl.MaxLines = 1
						lbl.Truncator = "…"
						return lbl.Layout(gtx)
					}),
				)
			})
		},
	)
}

func (ui *UI) layoutDialogActionSegment(th *material.Theme, gtx layout.Context, click *widget.Clickable, label string, hoverFill, pulseFill float32, stripH int, roundLeft, roundRight, disabled bool) layout.Dimensions {
	if click == nil {
		return layout.Dimensions{}
	}
	hoverFill = clamp01(hoverFill)
	pulseFill = clamp01(pulseFill)
	if disabled {
		hoverFill = 0
		pulseFill = 0
	}
	segW := dialogActionSegmentWidthPx(gtx, label)
	dims := fixedWidth(gtx, segW, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if click.Pressed() && !disabled && pulseFill < 0.5 {
					pulseFill = 0.5
				}

				hoverDark := color.NRGBA{R: 34, G: 34, B: 34, A: 255}
				pulseCol := color.NRGBA{R: 48, G: 48, B: 48, A: 255}

				bg := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
				bg = mixNRGBA(bg, hoverDark, hoverFill)
				bg = mixNRGBA(bg, pulseCol, pulseFill*0.3)
				fg := mixNRGBA(txtColor, color.NRGBA{R: 236, G: 236, B: 236, A: 255}, hoverFill*0.75)
				fg = mixNRGBA(fg, color.NRGBA{R: 248, G: 248, B: 248, A: 255}, pulseFill*0.25)

				if disabled {
					bg = color.NRGBA{R: 24, G: 24, B: 24, A: 170}
					fg = color.NRGBA{R: 160, G: 166, B: 180, A: 255}
				}

				radius := gtx.Dp(unit.Dp(filePaneControlCornerDp - 1))
				return fillSegmentBg(gtx, bg, radius, roundLeft, roundRight, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(9), Right: unit.Dp(9)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body2(th, label)
							lbl.Font.Typeface = ui.mainTypeface()
							lbl.Font.Weight = font.Medium
							lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
							lbl.Color = fg
							lbl.MaxLines = 1
							return lbl.Layout(gtx)
						})
					})
				})
			})
		})
	})
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}
	if !disabled {
		defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	}
	return dims
}

func dialogActionSegmentWidthPx(gtx layout.Context, label string) int {
	runes := utf8.RuneCountInString(strings.TrimSpace(label))
	if runes < 2 {
		runes = 2
	}
	charW := gtx.Dp(unit.Dp(6))
	if charW < 4 {
		charW = 4
	}
	width := gtx.Dp(unit.Dp(26)) + runes*charW
	minW := gtx.Dp(unit.Dp(64))
	maxW := gtx.Dp(unit.Dp(176))
	if width < minW {
		width = minW
	}
	if width > maxW {
		width = maxW
	}
	return width
}

func (ui *UI) layoutDialogActionPair(th *material.Theme, gtx layout.Context, leftClick *widget.Clickable, leftLabel string, leftHover, leftPulse float32, leftDisabled bool, rightClick *widget.Clickable, rightLabel string, rightHover, rightPulse float32, rightDisabled bool) layout.Dimensions {
	stripH := gtx.Dp(unit.Dp(22))
	if stripH < 1 {
		stripH = 1
	}
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 24, G: 24, B: 24, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 22},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutDialogActionSegment(th, gtx, leftClick, leftLabel, leftHover, leftPulse, stripH, true, false, leftDisabled)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return toolbarSeparator(gtx, stripH)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutDialogActionSegment(th, gtx, rightClick, rightLabel, rightHover, rightPulse, stripH, false, true, rightDisabled)
						}),
					)
				})
			})
		},
	)
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
	paint.FillShape(gtx.Ops, color.NRGBA{R: 34, G: 34, B: 34, A: 255}, clip.Rect(bg).Op())
	if fillW > 0 {
		fg := image.Rect(0, 0, fillW, height)
		paint.FillShape(gtx.Ops, color.NRGBA{R: 140, G: 140, B: 140, A: 255}, clip.Rect(fg).Op())
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
	if strings.Contains(progress.CurrentPath, "/") {
		return path.Base(progress.CurrentPath)
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
