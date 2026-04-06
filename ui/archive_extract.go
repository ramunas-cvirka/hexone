// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"fmt"
	"hexone/filesys"
	"image"
	"image/color"
	"io/fs"
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

	uitheme "hexone/ui/theme"
)

var errArchiveExtractAborted = errors.New("extract aborted")

const archiveExtractSuccessNoticeDur = 1200 * time.Millisecond

type archiveExtractConflictDecision uint8

const (
	archiveExtractDecisionOverwrite archiveExtractConflictDecision = iota + 1
	archiveExtractDecisionOverwriteAll
	archiveExtractDecisionAbort
)

type archiveExtractPlan struct {
	dstPath string
	entries []transferEntry
}

type archiveExtractConflict struct {
	dstPath     string
	displayPath string
}

type archiveExtractState struct {
	pane         int
	archivePath  string
	dstDir       string
	totalEntries int

	doneCh         chan error
	conflictReqCh  chan archiveExtractConflict
	conflictRespCh chan archiveExtractConflictDecision
	conflict       *archiveExtractConflict

	backdropClick     widget.Clickable
	closeClick        widget.Clickable
	overwriteClick    widget.Clickable
	overwriteAllClick widget.Clickable
	abortClick        widget.Clickable
	actionsAnim       segmentedAnimState
}

func buildArchiveExtractPlans(archivePath, dstDir string) (string, string, []archiveExtractPlan, int, error) {
	srcEp := copyEndpoint{dir: archivePath, archive: true}
	dstEp := copyEndpoint{dir: dstDir}

	effectiveDstDir, _, err := inspectCopyDestinationDir(dstEp, dstDir)
	if err != nil {
		return "", "", nil, 0, err
	}

	listing, err := filesys.ReadDir(archivePath)
	if err != nil {
		return "", "", nil, 0, err
	}

	rootEntries := make([]filesys.Entry, 0, len(listing.Entries))
	for _, item := range listing.Entries {
		if item.Kind == filesys.EntryParent || item.Path == "" {
			continue
		}
		rootEntries = append(rootEntries, item)
	}

	extractRoot := effectiveDstDir
	if !archiveExtractRootIsEnclosed(rootEntries) {
		extractRoot = dstEp.join(effectiveDstDir, archiveExtractWrapperName(archivePath))
	}

	plans := make([]archiveExtractPlan, 0, len(rootEntries))
	totalEntries := 0
	for _, item := range rootEntries {
		srcPath, err := srcEp.normalizeSourcePath(item.Path)
		if err != nil {
			return "", "", nil, 0, err
		}
		srcInfo, err := endpointLstat(srcEp, srcPath)
		if err != nil {
			return "", "", nil, 0, err
		}
		entries, _, err := collectTransferEntries(srcEp, srcPath, srcInfo)
		if err != nil {
			return "", "", nil, 0, err
		}
		plans = append(plans, archiveExtractPlan{
			dstPath: dstEp.join(extractRoot, srcEp.baseName(srcPath)),
			entries: entries,
		})
		totalEntries += len(entries)
	}

	return effectiveDstDir, extractRoot, plans, totalEntries, nil
}

func archiveExtractRootIsEnclosed(entries []filesys.Entry) bool {
	return len(entries) == 1 && entries[0].Kind == filesys.EntryDir
}

func archiveExtractWrapperName(archivePath string) string {
	name := strings.TrimSpace(filepath.Base(archivePath))
	if name == "" {
		return "archive"
	}

	lastGood := name
	for {
		ext := filepath.Ext(lastGood)
		if ext == "" {
			break
		}
		next := strings.TrimSuffix(lastGood, ext)
		if strings.TrimSpace(next) == "" {
			break
		}
		lastGood = next
		if !filesys.ArchiveNameSupported(lastGood) {
			break
		}
	}
	lastGood = strings.TrimSpace(lastGood)
	if lastGood == "" {
		base := strings.TrimSpace(strings.TrimSuffix(name, filepath.Ext(name)))
		if base != "" {
			return base
		}
		return name
	}
	return lastGood
}

func archiveExtractConflictDisplayPath(dstDir, dstPath string) string {
	base := filepath.Clean(dstDir)
	target := filepath.Clean(dstPath)
	rel, err := filepath.Rel(base, target)
	if err == nil {
		prefix := ".." + string(filepath.Separator)
		if rel != "." && rel != ".." && !strings.HasPrefix(rel, prefix) {
			return filepath.ToSlash(rel)
		}
	}
	name := filepath.Base(target)
	if name != "" && name != "." && name != string(filepath.Separator) {
		return name
	}
	return target
}

func archiveExtractEntryNeedsOverwrite(dstEp copyEndpoint, entry transferEntry, dstPath string) (bool, error) {
	info, err := endpointLstat(dstEp, dstPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if entry.isDir && info.IsDir() {
		return false, nil
	}
	return true, nil
}

func runArchiveExtractPlans(rootDir string, plans []archiveExtractPlan, dstEp copyEndpoint, dstDir string, onConflict func(archiveExtractConflict) archiveExtractConflictDecision) error {
	srcEp := copyEndpoint{archive: true}
	progress := filesys.CopyProgress{}
	for _, plan := range plans {
		progress.EntriesTotal += len(plan.entries)
	}

	overwriteAll := false
	if rootDir != "" && !samePath(rootDir, dstDir) {
		needsOverwrite, err := archiveExtractEntryNeedsOverwrite(dstEp, transferEntry{isDir: true}, rootDir)
		if err != nil {
			return err
		}
		if needsOverwrite {
			decision := archiveExtractDecisionOverwrite
			if onConflict == nil {
				return fmt.Errorf("destination already exists: %s", archiveExtractConflictDisplayPath(dstDir, rootDir))
			}
			decision = onConflict(archiveExtractConflict{
				dstPath:     rootDir,
				displayPath: archiveExtractConflictDisplayPath(dstDir, rootDir),
			})
			switch decision {
			case archiveExtractDecisionOverwrite:
			case archiveExtractDecisionOverwriteAll:
				overwriteAll = decision == archiveExtractDecisionOverwriteAll
			default:
				return errArchiveExtractAborted
			}
			if err := removeEndpointPathIfExists(dstEp, rootDir); err != nil {
				return err
			}
		}
		if err := ensureEndpointDir(dstEp, rootDir); err != nil {
			return err
		}
	}

	for _, plan := range plans {
		for _, entry := range plan.entries {
			progress.CurrentPath = entry.srcPath
			dstEntryPath := dstEp.join(plan.dstPath, entry.rel)

			needsOverwrite, err := archiveExtractEntryNeedsOverwrite(dstEp, entry, dstEntryPath)
			if err != nil {
				return err
			}
			if needsOverwrite {
				decision := archiveExtractDecisionOverwriteAll
				if !overwriteAll {
					if onConflict == nil {
						return fmt.Errorf("destination already exists: %s", archiveExtractConflictDisplayPath(dstDir, dstEntryPath))
					}
					decision = onConflict(archiveExtractConflict{
						dstPath:     dstEntryPath,
						displayPath: archiveExtractConflictDisplayPath(dstDir, dstEntryPath),
					})
				}
				switch decision {
				case archiveExtractDecisionOverwrite:
				case archiveExtractDecisionOverwriteAll:
					overwriteAll = true
				default:
					return errArchiveExtractAborted
				}
				if err := removeEndpointPathIfExists(dstEp, dstEntryPath); err != nil {
					return err
				}
			}

			if err := copyTransferEntry(srcEp, dstEp, entry, dstEntryPath, &progress, nil); err != nil {
				return err
			}
			progress.EntriesDone++
		}
	}

	return nil
}

func (ui *UI) archiveExtractConflictOpen() bool {
	return ui != nil && ui.archiveExtract != nil && ui.archiveExtract.conflict != nil
}

func (ui *UI) startArchiveExtractHere(idx, row int, now time.Time) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.model == nil {
		return
	}
	if ui.archiveExtract != nil {
		pane.setNotice("extract already in progress", now)
		return
	}

	entry := pane.model.Entry(row)
	if entry == nil || entry.Kind != filesys.EntryFile || !entry.CanEnter || pane.remoteConnected() || pane.archiveBrowsing() {
		pane.setNotice("nothing selected to extract", now)
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
	ui.rep.active = false
	ui.rep.pane = -1

	dstDir := strings.TrimSpace(pane.dir)
	if dstDir == "" {
		dstDir = "."
	}

	effectiveDstDir, extractRoot, plans, totalEntries, err := buildArchiveExtractPlans(entry.Path, dstDir)
	if err != nil {
		pane.setNotice("extract failed: "+err.Error(), now)
		return
	}
	if len(plans) == 0 {
		pane.setNotice("archive is empty", now)
		return
	}

	st := &archiveExtractState{
		pane:           idx,
		archivePath:    entry.Path,
		dstDir:         effectiveDstDir,
		totalEntries:   totalEntries,
		doneCh:         make(chan error, 1),
		conflictReqCh:  make(chan archiveExtractConflict, 1),
		conflictRespCh: make(chan archiveExtractConflictDecision, 1),
	}
	ui.archiveExtract = st
	pane.setNotice("extracting archive", now)

	go func() {
		err := runArchiveExtractPlans(extractRoot, plans, copyEndpoint{dir: effectiveDstDir}, effectiveDstDir, func(conflict archiveExtractConflict) archiveExtractConflictDecision {
			st.conflictReqCh <- conflict
			decision, ok := <-st.conflictRespCh
			if !ok {
				return archiveExtractDecisionAbort
			}
			return decision
		})
		st.doneCh <- err
	}()
}

func (ui *UI) pumpArchiveExtractState(gtx layout.Context) {
	st := ui.archiveExtract
	if st == nil {
		return
	}

	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(33 * time.Millisecond)})

	if st.conflict == nil {
		select {
		case conflict := <-st.conflictReqCh:
			st.conflict = &conflict
		default:
		}
	}
	if st.conflict != nil {
		return
	}

	select {
	case err := <-st.doneCh:
		ui.finishArchiveExtract(gtx.Now, err)
	default:
	}
}

func (ui *UI) finishArchiveExtract(now time.Time, err error) {
	st := ui.archiveExtract
	if st == nil {
		return
	}
	ui.archiveExtract = nil

	pane := (*filePaneState)(nil)
	if st.pane >= 0 && st.pane < len(ui.filePanes) {
		pane = ui.filePanes[st.pane]
	}
	noticeText, noticeDur := archiveExtractOutcomeNotice(err, st.totalEntries)

	originReloaded := false
	for i, other := range ui.filePanes {
		if other == nil || other.remoteConnected() || other.archiveBrowsing() {
			continue
		}
		currentDir := strings.TrimSpace(other.dir)
		if currentDir == "" {
			currentDir = "."
		}
		absDir, absErr := filepath.Abs(currentDir)
		if absErr != nil || !samePath(absDir, st.dstDir) {
			continue
		}
		selectedPath := ""
		if sel := other.selectedEntry(); sel != nil {
			selectedPath = sel.Path
		}
		restorePos := sanitizePaneListPosition(other.table.List.Position)
		restoreAnchor := other.visibleAnchorPath()
		reloadNoticeText := ""
		reloadNoticeDur := time.Duration(0)
		if i == st.pane {
			reloadNoticeText = noticeText
			reloadNoticeDur = noticeDur
		}
		if ui.requestPaneLoadWithSelectionAndScroll(i, other.dir, selectedPath, "", other.table.Selected, restorePos, true, restoreAnchor, reloadNoticeText, reloadNoticeDur) && i == st.pane {
			originReloaded = true
		}
	}
	if (originReloaded || noticeText == "") || pane == nil {
		return
	}
	if noticeDur > 0 {
		pane.setNoticeFor(noticeText, now, noticeDur)
		return
	}
	pane.setNotice(noticeText, now)
}

func archiveExtractOutcomeNotice(err error, totalEntries int) (string, time.Duration) {
	switch {
	case err == nil:
		label := "items"
		if totalEntries == 1 {
			label = "item"
		}
		return fmt.Sprintf("extracted %d %s", totalEntries, label), archiveExtractSuccessNoticeDur
	case errors.Is(err, errArchiveExtractAborted):
		return "extract aborted", 0
	default:
		return "extract failed: " + err.Error(), 0
	}
}

func (ui *UI) resolveArchiveExtractConflict(decision archiveExtractConflictDecision) {
	st := ui.archiveExtract
	if st == nil || st.conflict == nil {
		return
	}
	st.conflict = nil
	st.conflictRespCh <- decision
}

func (ui *UI) layoutArchiveExtractConflictDialog(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.archiveExtract
	if st == nil || st.conflict == nil {
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
				ui.resolveArchiveExtractConflict(archiveExtractDecisionAbort)
			case key.NameEnter, key.NameReturn:
				st.actionsAnim.setPulse("overwrite", gtx.Now)
				ui.resolveArchiveExtractConflict(archiveExtractDecisionOverwrite)
			}
		}
	}

	if st.closeClick.Clicked(gtx) {
		ui.resolveArchiveExtractConflict(archiveExtractDecisionAbort)
		return layout.Dimensions{}
	}
	if st.overwriteClick.Clicked(gtx) {
		st.actionsAnim.setPulse("overwrite", gtx.Now)
		ui.resolveArchiveExtractConflict(archiveExtractDecisionOverwrite)
		return layout.Dimensions{}
	}
	if st.overwriteAllClick.Clicked(gtx) {
		st.actionsAnim.setPulse("overwrite-all", gtx.Now)
		ui.resolveArchiveExtractConflict(archiveExtractDecisionOverwriteAll)
		return layout.Dimensions{}
	}
	if st.abortClick.Clicked(gtx) {
		st.actionsAnim.setPulse("abort", gtx.Now)
		ui.resolveArchiveExtractConflict(archiveExtractDecisionAbort)
		return layout.Dimensions{}
	}
	for st.backdropClick.Clicked(gtx) {
	}

	hoverActionKey := ""
	if st.overwriteClick.Hovered() {
		hoverActionKey = "overwrite"
	}
	if st.overwriteAllClick.Hovered() {
		hoverActionKey = "overwrite-all"
	}
	if st.abortClick.Hovered() {
		hoverActionKey = "abort"
	}
	st.actionsAnim.setHover(hoverActionKey, gtx.Now)
	hoverOverwrite, hoverAnimOverwrite := st.actionsAnim.hoverFill(gtx.Now, "overwrite")
	hoverOverwriteAll, hoverAnimOverwriteAll := st.actionsAnim.hoverFill(gtx.Now, "overwrite-all")
	hoverAbort, hoverAnimAbort := st.actionsAnim.hoverFill(gtx.Now, "abort")
	pulseOverwrite, pulseAnimOverwrite := st.actionsAnim.pulseFill(gtx.Now, "overwrite")
	pulseOverwriteAll, pulseAnimOverwriteAll := st.actionsAnim.pulseFill(gtx.Now, "overwrite-all")
	pulseAbort, pulseAnimAbort := st.actionsAnim.pulseFill(gtx.Now, "abort")
	if hoverAnimOverwrite || hoverAnimOverwriteAll || hoverAnimAbort || pulseAnimOverwrite || pulseAnimOverwriteAll || pulseAnimAbort {
		gtx.Execute(op.InvalidateCmd{})
	}

	return st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{A: 120}, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

		paneRect := ui.filePaneRectForOverlay(gtx, st.pane)
		width := gtx.Dp(unit.Dp(360))
		maxWidth := paneRect.Dx() - gtx.Dp(unit.Dp(16))
		if width > maxWidth {
			width = maxWidth
		}
		if width < 260 {
			width = 260
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
						return ui.layoutArchiveExtractConflictBody(
							th,
							gtx,
							st,
							hoverOverwrite,
							hoverOverwriteAll,
							hoverAbort,
							pulseOverwrite,
							pulseOverwriteAll,
							pulseAbort,
						)
					})
				},
			)
		})
		call := m.Stop()

		x := paneRect.Min.X + (paneRect.Dx()-dialog.Size.X)/2
		if x < 0 {
			x = 0
		}
		y := paneRect.Min.Y + (paneRect.Dy()-dialog.Size.Y)/2
		if y < 0 {
			y = 0
		}
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()

		return layout.Dimensions{Size: gtx.Constraints.Max, Baseline: dialog.Baseline}
	})
}

func (ui *UI) layoutArchiveExtractConflictBody(th *material.Theme, gtx layout.Context, st *archiveExtractState, hoverOverwrite, hoverOverwriteAll, hoverAbort, pulseOverwrite, pulseOverwriteAll, pulseAbort float32) layout.Dimensions {
	title := material.Body1(th, "Extract here")
	title.Font.Typeface = ui.mainTypeface()
	title.Font.Weight = font.Bold
	title.TextSize = scaleDialogThemeFontSize(th, 12)
	title.Color = txtColor

	subtitle := material.Caption(th, "Destination already exists")
	subtitle.Font.Typeface = ui.mainTypeface()
	subtitle.TextSize = scaleDialogThemeFontSize(th, 9)
	subtitle.Color = hintColor

	pathText := material.Body2(th, st.conflict.displayPath)
	pathText.Font.Typeface = ui.mainTypeface()
	pathText.TextSize = scaleDialogThemeFontSize(th, 10)
	pathText.Color = color.NRGBA{R: 220, G: 220, B: 220, A: 255}
	pathText.MaxLines = 2
	pathText.Truncator = "…"

	hint := material.Caption(th, "Enter overwrites this item. Esc aborts.")
	hint.Font.Typeface = ui.mainTypeface()
	hint.TextSize = scaleDialogThemeFontSize(th, 9)
	hint.Color = color.NRGBA{R: 184, G: 184, B: 184, A: 255}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, title.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutTinyIconModeButton(th, gtx, &st.closeClick, uitheme.CloseIcon(), false)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(subtitle.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fillRoundedBox(
				gtx,
				gtx.Dp(unit.Dp(filePaneControlCornerDp)),
				color.NRGBA{R: 24, G: 24, B: 24, A: 255},
				color.NRGBA{R: 255, G: 255, B: 255, A: 18},
				func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, pathText.Layout)
				},
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(hint.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutDialogActionTriple(
					th, gtx,
					&st.overwriteClick, "Overwrite", hoverOverwrite, pulseOverwrite, false,
					&st.overwriteAllClick, "Overwrite all", hoverOverwriteAll, pulseOverwriteAll, false,
					&st.abortClick, "Abort", hoverAbort, pulseAbort, false,
					dialogActionVisualState{},
					dialogActionVisualState{},
					dialogActionVisualState{},
				)
			})
		}),
	)
}
