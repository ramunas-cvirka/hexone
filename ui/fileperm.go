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
	"runtime"
	"strconv"
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

var permBitMasks = [...]os.FileMode{
	0o400, 0o200, 0o100,
	0o040, 0o020, 0o010,
	0o004, 0o002, 0o001,
}

type filePermState struct {
	pane int
	row  int

	targetPath string
	targetName string

	endpoint copyEndpoint
	remote   *paneSSHSession

	permMode os.FileMode
	permEdit widget.Editor

	backdropClick widget.Clickable
	closeClick    widget.Clickable
	confirmClick  widget.Clickable
	cancelClick   widget.Clickable

	checks [9]widget.Bool

	running bool
	lastErr string
	doneCh  chan error

	actionsAnim segmentedAnimState
}

func (ui *UI) startFilePermDialog(idx, row int, now time.Time) bool {
	if idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.model == nil || pane.table == nil {
		return false
	}
	if pane.archiveBrowsing() {
		pane.setNotice("cannot change permissions inside an archive", now)
		return false
	}
	entry := pane.model.Entry(row)
	if entry == nil || entry.Path == "" {
		pane.setNotice("nothing selected", now)
		return false
	}
	if entry.Kind == filesys.EntryParent {
		pane.setNotice("cannot change parent entry permissions", now)
		return false
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
			return false
		}
	}

	endpoint := copyEndpoint{pane: idx, remote: remote, dir: strings.TrimSpace(pane.dir)}
	if runtime.GOOS == "windows" && !endpoint.isRemote() {
		if remote != nil {
			remote.close()
		}
		pane.setNotice("local Windows permissions are ACL-based; chmod rwx is unsupported", now)
		return false
	}
	info, err := endpointStat(endpoint, entry.Path)
	if err != nil {
		if remote != nil {
			remote.close()
		}
		pane.setNotice("failed to read permissions: "+err.Error(), now)
		return false
	}

	st := &filePermState{
		pane:       idx,
		row:        row,
		targetPath: entry.Path,
		targetName: entry.DisplayName,
		endpoint:   endpoint,
		remote:     remote,
		permMode:   info.Mode().Perm(),
	}
	st.permEdit.SingleLine = true
	st.permEdit.Submit = true
	st.syncChecksFromMode()
	st.permEdit.SetText(formatPermDigits(st.permMode))
	st.permEdit.SetCaret(st.permEdit.Len(), st.permEdit.Len())

	ui.filePerm = st
	ui.rep.active = false
	ui.rep.pane = -1
	return true
}

func (ui *UI) closeFilePermDialog() {
	st := ui.filePerm
	if st != nil && st.remote != nil {
		st.remote.close()
		st.remote = nil
	}
	ui.filePerm = nil
}

func (ui *UI) submitFilePermDialog(now time.Time) {
	st := ui.filePerm
	if st == nil || st.running {
		return
	}
	mode, ok := parsePermDigits(st.permEdit.Text())
	if !ok {
		st.lastErr = "enter octal permissions (e.g. 0755)"
		return
	}
	st.permMode = mode.Perm()
	st.lastErr = ""
	st.running = true
	st.doneCh = make(chan error, 1)

	endpoint := st.endpoint
	targetPath := st.targetPath
	perm := st.permMode
	doneCh := st.doneCh
	go func() {
		doneCh <- endpointChmod(endpoint, targetPath, perm)
	}()

	_ = now
}

func (ui *UI) pumpFilePermState(gtx layout.Context) {
	st := ui.filePerm
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
		ui.finishFilePermDialog(gtx.Now)
	default:
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) finishFilePermDialog(now time.Time) {
	st := ui.filePerm
	if st == nil {
		return
	}
	ui.refreshPanesAfterPermChange(st, now)
	ui.closeFilePermDialog()
}

func (ui *UI) refreshPanesAfterPermChange(st *filePermState, _ time.Time) {
	if st == nil {
		return
	}
	target := st.targetPath
	targetDir := st.endpoint.dirName(target)
	remoteChange := st.endpoint.isRemote()

	for i, pane := range ui.filePanes {
		if pane == nil || pane.model == nil || pane.table == nil {
			continue
		}
		if remoteChange {
			if !pane.remoteConnected() || pane.remote == nil || !sameSSHRemoteTarget(pane.remote.setup, st.remote.setup) {
				continue
			}
			if path.Clean(pane.dir) != path.Clean(targetDir) {
				continue
			}
		} else {
			if pane.remoteConnected() {
				continue
			}
			if !samePath(filepath.Clean(pane.dir), filepath.Clean(targetDir)) {
				continue
			}
		}

		selectedPath := ""
		if sel := pane.selectedEntry(); sel != nil {
			selectedPath = sel.Path
		}
		ui.requestPaneLoadWithSelection(i, pane.dir, target, selectedPath, pane.table.Selected)
	}
}

func (ui *UI) layoutFilePermDialog(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.filePerm
	if st == nil {
		return layout.Dimensions{}
	}

	if changed := st.syncPermCheckboxesFromUI(gtx); changed {
		st.permMode = st.permModeFromChecks()
		st.permEdit.SetText(formatPermDigits(st.permMode))
		st.permEdit.SetCaret(st.permEdit.Len(), st.permEdit.Len())
		st.lastErr = ""
	}
	for {
		ev, ok := st.permEdit.Update(gtx)
		if !ok {
			break
		}
		switch ev.(type) {
		case widget.ChangeEvent:
			st.onPermEditChanged()
		case widget.SubmitEvent:
			if !st.running {
				st.actionsAnim.setPulse("confirm", gtx.Now)
				ui.submitFilePermDialog(gtx.Now)
			}
		}
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
					ui.closeFilePermDialog()
				}
			case key.NameEnter, key.NameReturn:
				if !st.running {
					st.actionsAnim.setPulse("confirm", gtx.Now)
					ui.submitFilePermDialog(gtx.Now)
				}
			}
		}
	}

	if st.cancelClick.Clicked(gtx) && !st.running {
		ui.closeFilePermDialog()
		return layout.Dimensions{}
	}
	if st.closeClick.Clicked(gtx) && !st.running {
		ui.closeFilePermDialog()
		return layout.Dimensions{}
	}
	if st.confirmClick.Clicked(gtx) && !st.running {
		st.actionsAnim.setPulse("confirm", gtx.Now)
		ui.submitFilePermDialog(gtx.Now)
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
		width := gtx.Dp(unit.Dp(254))
		maxWidth := paneRect.Dx() - gtx.Dp(unit.Dp(16))
		if maxWidth < 216 {
			maxWidth = 216
		}
		if width > maxWidth {
			width = maxWidth
		}
		if width < 216 {
			width = 216
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
						return ui.layoutFilePermDialogBody(th, gtx, st)
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

func (ui *UI) layoutFilePermDialogBody(th *material.Theme, gtx layout.Context, st *filePermState) layout.Dimensions {
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

	target := st.targetName
	if target == "" {
		if st.endpoint.isRemote() {
			target = path.Base(st.targetPath)
		} else {
			target = filepath.Base(st.targetPath)
		}
	}
	targetLabel := material.Body2(th, target)
	targetLabel.Font.Typeface = ui.mainTypeface()
	targetLabel.TextSize = scaleDialogThemeFontSize(th, 10)
	targetLabel.Font.Weight = font.Medium
	targetLabel.Color = txtColor
	targetLabel.MaxLines = 1
	targetLabel.Truncator = "…"

	pathLabel := material.Caption(th, st.targetPath)
	pathLabel.Font.Typeface = ui.mainTypeface()
	pathLabel.TextSize = scaleDialogThemeFontSize(th, 9)
	pathLabel.Color = hintColor
	pathLabel.MaxLines = 1
	pathLabel.Truncator = "…"

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					title := material.Body1(th, "Permissions")
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
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(targetLabel.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(pathLabel.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutPermMatrix(th, gtx, st)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutPermDigitsEditor(th, gtx, st)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
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
			lbl := material.Caption(th, "Applying permissions...")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleDialogThemeFontSize(th, 9)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := "Apply"
				if st.running {
					label = "Applying..."
				}
				_, valid := parsePermDigits(st.permEdit.Text())
				return ui.layoutDialogActionPair(
					th, gtx,
					&st.cancelClick, "Cancel", hoverCancel, pulseCancel, st.running,
					&st.confirmClick, label, hoverConfirm, pulseConfirm, st.running || !valid,
					dialogActionVisualState{},
					dialogActionVisualState{Default: !st.running && valid},
				)
			})
		}),
	)
}

func (ui *UI) layoutPermMatrix(th *material.Theme, gtx layout.Context, st *filePermState) layout.Dimensions {
	const (
		rowLabelDp  = unit.Dp(56)
		colWidthDp  = unit.Dp(46)
		headerGapDp = unit.Dp(2)
		rowGapDp    = unit.Dp(4)
	)

	rowLabel := func(title string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, title)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleDialogThemeFontSize(th, 10)
			lbl.Color = color.NRGBA{R: 178, G: 178, B: 178, A: 255}
			return lbl.Layout(gtx)
		}
	}
	check := func(idx int) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutThemeCheckbox(th, gtx, &st.checks[idx], "", scaleDialogThemeFontSize(th, 10))
			})
		}
	}
	colLabel := func(title string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, title)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleDialogThemeFontSize(th, 10)
			lbl.Font.Weight = font.Medium
			lbl.Color = color.NRGBA{R: 196, G: 196, B: 196, A: 255}
			return layout.Center.Layout(gtx, lbl.Layout)
		}
	}
	colSpacer := func(gtx layout.Context) layout.Dimensions {
		return layout.Spacer{Width: unit.Dp(1)}.Layout(gtx)
	}
	cell := func(w unit.Dp, inner layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(w), inner)
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(cell(rowLabelDp, rowLabel(""))),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(cell(colWidthDp, colLabel("Read"))),
				layout.Rigid(colSpacer),
				layout.Rigid(cell(colWidthDp, colLabel("Write"))),
				layout.Rigid(colSpacer),
				layout.Rigid(cell(colWidthDp, colLabel("Execute"))),
			)
		}),
		layout.Rigid(layout.Spacer{Height: headerGapDp}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(cell(rowLabelDp, rowLabel("Owner"))),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(cell(colWidthDp, check(0))),
				layout.Rigid(colSpacer),
				layout.Rigid(cell(colWidthDp, check(1))),
				layout.Rigid(colSpacer),
				layout.Rigid(cell(colWidthDp, check(2))),
			)
		}),
		layout.Rigid(layout.Spacer{Height: rowGapDp}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(cell(rowLabelDp, rowLabel("Group"))),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(cell(colWidthDp, check(3))),
				layout.Rigid(colSpacer),
				layout.Rigid(cell(colWidthDp, check(4))),
				layout.Rigid(colSpacer),
				layout.Rigid(cell(colWidthDp, check(5))),
			)
		}),
		layout.Rigid(layout.Spacer{Height: rowGapDp}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(cell(rowLabelDp, rowLabel("Other"))),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(cell(colWidthDp, check(6))),
				layout.Rigid(colSpacer),
				layout.Rigid(cell(colWidthDp, check(7))),
				layout.Rigid(colSpacer),
				layout.Rigid(cell(colWidthDp, check(8))),
			)
		}),
	)
}

func (ui *UI) layoutPermDigitsEditor(th *material.Theme, gtx layout.Context, st *filePermState) layout.Dimensions {
	sym := material.Caption(th, formatPermSymbolicGrouped(st.permMode))
	sym.Font.Typeface = ui.mainTypeface()
	sym.TextSize = scaleDialogThemeFontSize(th, 9)
	sym.Color = color.NRGBA{R: 186, G: 202, B: 216, A: 255}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, "Octal")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleDialogThemeFontSize(th, 9)
			lbl.Color = color.NRGBA{R: 178, G: 178, B: 178, A: 255}
			return fixedWidth(gtx, gtx.Dp(unit.Dp(54)), lbl.Layout)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			ed := material.Editor(th, &st.permEdit, "0755")
			ed.Font.Typeface = ui.mainTypeface()
			ed.TextSize = scaleDialogThemeFontSize(th, 10)
			ed.Color = txtColor
			ed.HintColor = hintColor
			return fixedWidth(gtx, gtx.Dp(unit.Dp(84)), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutEditorWithContextMenu(th, gtx, "fileperm-digits", &st.permEdit, !st.running, func(gtx layout.Context) layout.Dimensions {
					return layoutNeutralEditorBox(gtx, gtx.Focused(&st.permEdit), !st.running, ed.Layout)
				})
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return sym.Layout(gtx)
		}),
	)
}

func (st *filePermState) syncChecksFromMode() {
	if st == nil {
		return
	}
	mode := st.permMode.Perm()
	for i := range st.checks {
		st.checks[i].Value = mode&permBitMasks[i] != 0
	}
}

func (st *filePermState) permModeFromChecks() os.FileMode {
	if st == nil {
		return 0
	}
	var mode os.FileMode
	for i := range st.checks {
		if st.checks[i].Value {
			mode |= permBitMasks[i]
		}
	}
	return mode.Perm()
}

func (st *filePermState) syncPermCheckboxesFromUI(gtx layout.Context) bool {
	if st == nil {
		return false
	}
	changed := false
	for i := range st.checks {
		if st.checks[i].Update(gtx) {
			changed = true
		}
	}
	return changed
}

func (st *filePermState) onPermEditChanged() {
	if st == nil {
		return
	}
	clean := normalizePermDigits(st.permEdit.Text())
	if clean != st.permEdit.Text() {
		st.permEdit.SetText(clean)
		st.permEdit.SetCaret(st.permEdit.Len(), st.permEdit.Len())
	}
	mode, ok := parsePermDigits(clean)
	if !ok {
		return
	}
	st.permMode = mode.Perm()
	st.syncChecksFromMode()
	st.lastErr = ""
}

func normalizePermDigits(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range raw {
		if r < '0' || r > '7' {
			continue
		}
		b.WriteRune(r)
	}
	out := b.String()
	if len(out) > 4 {
		out = out[len(out)-4:]
	}
	return out
}

func parsePermDigits(raw string) (os.FileMode, bool) {
	txt := normalizePermDigits(raw)
	if len(txt) == 4 {
		txt = txt[1:]
	}
	if len(txt) != 3 {
		return 0, false
	}
	val, err := strconv.ParseUint(txt, 8, 16)
	if err != nil {
		return 0, false
	}
	return os.FileMode(val).Perm(), true
}

func formatPermDigits(mode os.FileMode) string {
	return fmt.Sprintf("%04o", mode.Perm())
}

func formatPermSymbolic(mode os.FileMode) string {
	chars := []byte("---------")
	if mode&0o400 != 0 {
		chars[0] = 'r'
	}
	if mode&0o200 != 0 {
		chars[1] = 'w'
	}
	if mode&0o100 != 0 {
		chars[2] = 'x'
	}
	if mode&0o040 != 0 {
		chars[3] = 'r'
	}
	if mode&0o020 != 0 {
		chars[4] = 'w'
	}
	if mode&0o010 != 0 {
		chars[5] = 'x'
	}
	if mode&0o004 != 0 {
		chars[6] = 'r'
	}
	if mode&0o002 != 0 {
		chars[7] = 'w'
	}
	if mode&0o001 != 0 {
		chars[8] = 'x'
	}
	return string(chars)
}

func formatPermSymbolicGrouped(mode os.FileMode) string {
	s := formatPermSymbolic(mode)
	if len(s) != 9 {
		return s
	}
	return s[0:3] + "-" + s[3:6] + "-" + s[6:9]
}

func endpointChmod(ep copyEndpoint, p string, mode os.FileMode) error {
	if ep.isRemote() {
		client := ep.remote.sftpClient()
		if client == nil {
			return errors.New("sftp session is not connected")
		}
		return client.Chmod(p, mode.Perm())
	}
	return os.Chmod(p, mode.Perm())
}
