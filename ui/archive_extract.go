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

var (
	errArchiveExtractAborted = errors.New("extract aborted")
	errArchiveExtractEmpty   = errors.New("archive is empty")
)

const (
	archiveExtractSuccessNoticeDur      = 1200 * time.Millisecond
	archiveExtractStatusRefreshInterval = 250 * time.Millisecond
)

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

type archiveExtractDone struct {
	err          error
	dstDir       string
	totalEntries int
}

type archiveExtractState struct {
	pane         int
	archivePath  string
	dstDir       string
	totalEntries int
	startedAt    time.Time

	doneCh         chan archiveExtractDone
	progressCh     chan filesys.CopyProgress
	conflictReqCh  chan archiveExtractConflict
	conflictRespCh chan archiveExtractConflictDecision
	conflict       *archiveExtractConflict
	progress       filesys.CopyProgress

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
	if archiveExtractNeedsWrapper(rootEntries) {
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

func archiveExtractNeedsWrapper(entries []filesys.Entry) bool {
	return len(entries) > 1
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

func runArchiveExtractPlans(rootDir string, plans []archiveExtractPlan, dstEp copyEndpoint, dstDir string, onConflict func(archiveExtractConflict) archiveExtractConflictDecision, report func(filesys.CopyProgress)) error {
	srcEp := copyEndpoint{archive: true}
	progress := filesys.CopyProgress{}
	for _, plan := range plans {
		progress.EntriesTotal += len(plan.entries)
		for _, entry := range plan.entries {
			if !entry.isDir && !entry.isSymlink && entry.size > 0 {
				progress.BytesTotal += entry.size
			}
		}
	}
	reportCopyProgress(report, progress)

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
			reportCopyProgress(report, progress)
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

			if err := copyTransferEntry(srcEp, dstEp, entry, dstEntryPath, &progress, report); err != nil {
				return err
			}
			progress.EntriesDone++
			reportCopyProgress(report, progress)
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
	archivePath := entry.Path

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

	progressCh := make(chan filesys.CopyProgress, 32)
	st := &archiveExtractState{
		pane:           idx,
		archivePath:    archivePath,
		dstDir:         dstDir,
		startedAt:      now,
		doneCh:         make(chan archiveExtractDone, 1),
		progressCh:     progressCh,
		conflictReqCh:  make(chan archiveExtractConflict, 1),
		conflictRespCh: make(chan archiveExtractConflictDecision, 1),
	}
	ui.archiveExtract = st

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

		effectiveDstDir, extractRoot, plans, totalEntries, err := buildArchiveExtractPlans(archivePath, dstDir)
		if err != nil {
			st.doneCh <- archiveExtractDone{err: err}
			return
		}
		if len(plans) == 0 {
			st.doneCh <- archiveExtractDone{err: errArchiveExtractEmpty}
			return
		}

		sendProgress(archiveExtractInitialProgress(plans))
		err = runArchiveExtractPlans(extractRoot, plans, copyEndpoint{dir: effectiveDstDir}, effectiveDstDir, func(conflict archiveExtractConflict) archiveExtractConflictDecision {
			st.conflictReqCh <- conflict
			decision, ok := <-st.conflictRespCh
			if !ok {
				return archiveExtractDecisionAbort
			}
			return decision
		}, sendProgress)
		st.doneCh <- archiveExtractDone{err: err, dstDir: effectiveDstDir, totalEntries: totalEntries}
	}()
}

func archiveExtractInitialProgress(plans []archiveExtractPlan) filesys.CopyProgress {
	progress := filesys.CopyProgress{}
	for _, plan := range plans {
		progress.EntriesTotal += len(plan.entries)
		for _, entry := range plan.entries {
			if !entry.isDir && !entry.isSymlink && entry.size > 0 {
				progress.BytesTotal += entry.size
			}
		}
	}
	return progress
}

func (ui *UI) pumpArchiveExtractState(gtx layout.Context) {
	st := ui.archiveExtract
	if st == nil {
		return
	}

	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(archiveExtractStatusRefreshInterval)})

	for {
		select {
		case progress := <-st.progressCh:
			st.progress = progress
		default:
			goto doneProgress
		}
	}
doneProgress:

	select {
	case done := <-st.doneCh:
		if done.dstDir != "" {
			st.dstDir = done.dstDir
		}
		st.totalEntries = done.totalEntries
		ui.finishArchiveExtract(gtx.Now, done.err)
		return
	default:
	}

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
}

func (ui *UI) archiveExtractPane() *filePaneState {
	st := ui.archiveExtract
	if st == nil || st.pane < 0 || st.pane >= len(ui.filePanes) {
		return nil
	}
	return ui.filePanes[st.pane]
}

type archiveExtractStatusParts struct {
	filename string
	details  []string
}

func archiveExtractStatusLabel(st *archiveExtractState, now time.Time) string {
	parts := archiveExtractStatusPartsFor(st, now)
	if parts.filename == "" {
		return ""
	}
	line := "[Extracting] " + parts.filename
	if len(parts.details) > 0 {
		line += " | " + strings.Join(parts.details, " | ")
	}
	return line
}

func archiveExtractStatusLineForWidth(st *archiveExtractState, now time.Time, maxWidth int, measure func(string) int) string {
	parts := archiveExtractStatusPartsFor(st, now)
	if parts.filename == "" {
		return ""
	}
	details := append([]string(nil), parts.details...)
	name := parts.filename
	if measure == nil || maxWidth <= 0 {
		return archiveExtractBuildStatusLine(name, details)
	}
	for {
		line := archiveExtractBuildStatusLine(name, details)
		if measure(line) <= maxWidth {
			return line
		}

		nameMax := maxWidth - measure(archiveExtractBuildStatusLine("", details))
		name = archiveExtractTrimMiddleToWidth(parts.filename, nameMax, measure)
		line = archiveExtractBuildStatusLine(name, details)
		if measure(line) <= maxWidth || len(details) <= 1 {
			return line
		}
		details = details[:len(details)-1]
		name = parts.filename
	}
}

func archiveExtractStatusLineWithSeparatorForWidth(st *archiveExtractState, now time.Time, maxWidth int, measure func(string) int, trailing bool) string {
	separator := "| "
	if trailing {
		separator = " |"
	}
	lineMax := maxWidth
	if measure != nil && maxWidth > 0 {
		lineMax -= measure(separator)
		if lineMax < 0 {
			lineMax = 0
		}
	}
	line := archiveExtractStatusLineForWidth(st, now, lineMax, measure)
	if strings.TrimSpace(line) == "" {
		return ""
	}
	if trailing {
		return line + separator
	}
	return separator + line
}

func archiveExtractStatusPartsFor(st *archiveExtractState, now time.Time) archiveExtractStatusParts {
	if st == nil {
		return archiveExtractStatusParts{}
	}
	progress := st.progress
	name := strings.TrimSpace(copyProgressCurrent(progress))
	if name == "" {
		name = strings.TrimSpace(filepath.Base(st.archivePath))
	}
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "archive"
	}

	parts := archiveExtractStatusParts{filename: name}
	if progress.BytesTotal > 0 || progress.EntriesTotal > 0 {
		frac := copyProgressFraction(progress)
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
		percent := int(frac*100 + 0.5)
		parts.details = append(parts.details, fmt.Sprintf("%s %d%%", archiveExtractStatusBar(frac), percent))
	} else {
		parts.details = append(parts.details, "preparing")
	}

	if speed := archiveExtractSpeed(progress, st.startedAt, now); speed > 0 {
		parts.details = append(parts.details, formatCopySize(speed)+"/s")
		if eta := archiveExtractETA(progress, speed); eta > 0 {
			parts.details = append(parts.details, formatArchiveExtractETA(eta)+" left")
		}
	}
	return parts
}

func archiveExtractBuildStatusLine(filename string, details []string) string {
	line := "[Extracting] " + filename
	if len(details) > 0 {
		line += " | " + strings.Join(details, " | ")
	}
	return line
}

func archiveExtractTrimMiddleToWidth(text string, maxWidth int, measure func(string) int) string {
	text = strings.TrimSpace(text)
	if text == "" || measure == nil || measure(text) <= maxWidth {
		return text
	}
	const ellipsis = "…"
	if maxWidth <= 0 || measure(ellipsis) > maxWidth {
		return ""
	}
	runes := []rune(text)
	for keep := len(runes) - 1; keep > 0; keep-- {
		head := (keep + 1) / 2
		tail := keep / 2
		candidate := string(runes[:head]) + ellipsis + string(runes[len(runes)-tail:])
		if measure(candidate) <= maxWidth {
			return candidate
		}
	}
	return ellipsis
}

func archiveExtractStatusBar(frac float32) string {
	const width = 10
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	done := int(frac*width + 0.5)
	if done < 0 {
		done = 0
	}
	if done > width {
		done = width
	}
	return strings.Repeat("█", done) + strings.Repeat("░", width-done)
}

func archiveExtractSpeed(progress filesys.CopyProgress, startedAt, now time.Time) int64 {
	if progress.BytesDone <= 0 || startedAt.IsZero() || !now.After(startedAt) {
		return 0
	}
	elapsed := now.Sub(startedAt)
	if elapsed < 500*time.Millisecond {
		return 0
	}
	speed := int64(float64(progress.BytesDone) / elapsed.Seconds())
	if speed < 0 {
		return 0
	}
	return speed
}

func archiveExtractETA(progress filesys.CopyProgress, speed int64) time.Duration {
	if progress.BytesTotal <= 0 || progress.BytesDone >= progress.BytesTotal || speed <= 0 {
		return 0
	}
	remaining := progress.BytesTotal - progress.BytesDone
	return time.Duration(float64(remaining) / float64(speed) * float64(time.Second))
}

func formatArchiveExtractETA(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	d = d.Round(time.Second)
	if d < time.Second {
		d = time.Second
	}
	total := int(d.Seconds())
	hours := total / 3600
	minutes := (total / 60) % 60
	seconds := total % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
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
	case errors.Is(err, errArchiveExtractEmpty):
		return "archive is empty", 0
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
	title.Font.Typeface = ui.interfaceTypeface()
	title.Font.Weight = font.Bold
	title.TextSize = ui.scaleDialogFontSize(12)
	title.Color = txtColor

	subtitle := material.Caption(th, "Destination already exists")
	subtitle.Font.Typeface = ui.interfaceTypeface()
	subtitle.TextSize = ui.scaleDialogFontSize(9)
	subtitle.Color = hintColor

	pathText := material.Body2(th, st.conflict.displayPath)
	pathText.Font.Typeface = ui.interfaceTypeface()
	pathText.TextSize = ui.scaleDialogFontSize(10)
	pathText.Color = color.NRGBA{R: 220, G: 220, B: 220, A: 255}
	pathText.MaxLines = 2
	pathText.Truncator = "…"

	hint := material.Caption(th, "Enter overwrites this item. Esc aborts.")
	hint.Font.Typeface = ui.interfaceTypeface()
	hint.TextSize = ui.scaleDialogFontSize(9)
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
