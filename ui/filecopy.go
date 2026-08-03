// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"errors"
	"fmt"
	"hexone/filesys"
	"image"
	"image/color"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
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

var dialogDividerColor = color.NRGBA{R: 255, G: 255, B: 255, A: 30}

const (
	fileOverwriteConnectorWidthDp = unit.Dp(12)
	fileOverwriteConnectorGapDp   = unit.Dp(7)
	fileOverwriteCellInsetDp      = unit.Dp(12)
)

type fileCopyState struct {
	pane    int
	srcPane int
	dstPane int
	op      fileCopyOp

	sources []fileCopySource
	srcPath string
	dstPath string
	dstRaw  string

	srcEndpoint copyEndpoint
	dstEndpoint copyEndpoint
	directPaste bool

	dstEdit     widget.Editor
	dstEditWant bool
	dstLocked   bool

	backdropClick widget.Clickable
	closeClick    widget.Clickable
	confirmClick  widget.Clickable
	cancelClick   widget.Clickable

	running  bool
	progress filesys.CopyProgress
	lastErr  string

	srcInfo       fileCopyPathInfo
	dstInfo       fileCopyPathInfo
	conflicts     []fileOverwriteConflict
	conflictCount int
	conflictList  widget.List

	progressCh    chan filesys.CopyProgress
	doneCh        chan error
	previewCh     chan fileCopyPreviewResult
	previewID     uint64
	previewing    bool
	previewCancel context.CancelFunc
	startedAt     time.Time
	speedBytes    int64
	speedAt       time.Time
	speedDone     int64
	speedSeenAt   time.Time
	cancelFunc    context.CancelFunc
	cancelUntil   time.Time
	canceling     bool
	actionsAnim   segmentedAnimState
	keyFocus      dialogKeyboardFocusState
	focus         fileCopyDialogFocus
	actionFocus   fileCopyDialogAction
}

type fileCopyDialogFocus uint8

const (
	fileCopyDialogFocusDestination fileCopyDialogFocus = iota
	fileCopyDialogFocusActions
)

type fileCopyDialogAction uint8

const (
	fileCopyDialogActionCancel fileCopyDialogAction = iota
	fileCopyDialogActionConfirm
)

type fileCopySource struct {
	Path string
	Name string
}

type fileCopyOp uint8

const (
	fileCopyOpCopy fileCopyOp = iota
	fileCopyOpExtract
)

const (
	fileCopySuccessNoticeDur       = 1200 * time.Millisecond
	fileCopyCanceledNoticeDur      = 1200 * time.Millisecond
	fileCopyCancelConfirmCountdown = 5 * time.Second
	fileCopySpeedSampleInterval    = 750 * time.Millisecond
	fileCopySpeedStaleAfter        = 3 * fileCopySpeedSampleInterval
	fileCopyRemotePreviewDebounce  = 180 * time.Millisecond
)

type fileCopyPathInfo struct {
	Path    string
	Exists  bool
	IsDir   bool
	Size    int64
	ModTime time.Time
}

type fileCopyPreviewResult struct {
	id            uint64
	raw           string
	dstPath       string
	srcInfo       fileCopyPathInfo
	dstInfo       fileCopyPathInfo
	conflicts     []fileOverwriteConflict
	conflictCount int
	err           error
}

type segmentedAnimState struct {
	hoverKey  string
	hoverPrev string
	hoverAt   time.Time
	pulseKey  string
	pulseSet  bool
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
	if st == nil {
		return
	}
	st.pulseKey = key
	st.pulseSet = true
	st.pulseAt = now
}

func (st *segmentedAnimState) pulseFill(now time.Time, key string) (float32, bool) {
	if st == nil || !st.pulseSet || st.pulseKey != key || st.pulseAt.IsZero() {
		return 0, false
	}
	elapsed := now.Sub(st.pulseAt)
	if elapsed >= toolbarClickDur {
		st.pulseKey = ""
		st.pulseSet = false
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
	if ui.fileCopy != nil {
		pane.setNotice("copy already in progress", now)
		return
	}
	selected := pane.selectedEntriesForAction()
	if len(selected) == 0 {
		if entry := pane.selectedEntry(); entry != nil && entry.Kind == filesys.EntryParent {
			pane.setNotice("cannot copy parent entry", now)
			return
		}
		pane.setNotice("nothing selected to copy", now)
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

	srcEndpoint := copyEndpointFromPane(idx, pane)
	dstPaneIdx, dstEndpoint, dstDir := ui.defaultFileCopyDestination(idx, pane, srcEndpoint)
	sources := make([]fileCopySource, 0, len(selected))
	for _, entry := range selected {
		sources = append(sources, fileCopySource{
			Path: entry.Path,
			Name: entry.DisplayName,
		})
	}
	first := sources[0]
	dstDefault := dstEndpoint.join(dstDir, srcEndpoint.baseName(first.Path))
	if len(sources) > 1 {
		dstDefault = dstDir
	}

	st := &fileCopyState{
		pane:        idx,
		srcPane:     idx,
		dstPane:     dstPaneIdx,
		op:          fileCopyOpCopy,
		sources:     sources,
		srcPath:     first.Path,
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
	st.focus = fileCopyDialogFocusDestination
	st.actionFocus = fileCopyDialogActionConfirm
	st.refreshPreview()

	ui.fileCopy = st
	ui.rep.active = false
	ui.rep.pane = -1
	ui.clearFileCopyHotkeyHold()
}

func (ui *UI) startClipboardFilePaste(idx int, clipboardPaths []string, now time.Time) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil || !pane.writableLocalView() {
		return
	}

	sources := make([]fileCopySource, 0, len(clipboardPaths))
	seen := make(map[string]struct{}, len(clipboardPaths))
	for _, raw := range clipboardPaths {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		abs, err := filepath.Abs(raw)
		if err != nil {
			continue
		}
		clean := filepath.Clean(abs)
		key := clean
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		sources = append(sources, fileCopySource{
			Path: clean,
			Name: filepath.Base(clean),
		})
	}
	if len(sources) == 0 {
		pane.setNotice("clipboard contains no usable local files", now)
		return
	}

	ui.setActiveFilePane(idx)
	pane.stopPathEdit()
	pane.closeSortMenu()
	pane.closeDriveMenu()
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	ui.closeSortMenusExcept(idx)
	ui.closeDriveMenusExcept(idx)
	ui.closeFavoriteMenusExcept(idx)
	ui.closeContextMenusExcept(idx)

	first := sources[0]
	srcEndpoint := copyEndpoint{
		pane: idx,
		dir:  filepath.Dir(first.Path),
	}
	dstEndpoint := copyEndpointFromPane(idx, pane)
	dstDir := pane.dir
	dstDefault := dstEndpoint.join(dstDir, srcEndpoint.baseName(first.Path))
	if len(sources) > 1 {
		dstDefault = dstDir
	}
	if err := validateDirectPasteDestinations(sources, dstEndpoint, dstDir); err != nil {
		pane.setNotice("paste failed: "+err.Error(), now)
		return
	}

	st := &fileCopyState{
		pane:        idx,
		srcPane:     idx,
		dstPane:     idx,
		op:          fileCopyOpCopy,
		sources:     sources,
		srcPath:     first.Path,
		dstPath:     dstDefault,
		dstRaw:      dstDefault,
		srcEndpoint: srcEndpoint,
		dstEndpoint: dstEndpoint,
		directPaste: true,
	}
	st.dstEdit.SingleLine = true
	st.dstEdit.Submit = true
	st.dstEdit.SetText(dstDefault)
	st.dstEdit.SetCaret(st.dstEdit.Len(), st.dstEdit.Len())

	ui.fileCopy = st
	ui.rep.active = false
	ui.rep.pane = -1
	ui.clearFileCopyHotkeyHold()
	ui.submitFileCopyDialog(now)
}

func validateDirectPasteDestinations(sources []fileCopySource, dstEndpoint copyEndpoint, dstDir string) error {
	targets := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		name := filepath.Base(filepath.Clean(source.Path))
		target := dstEndpoint.join(dstDir, name)
		key := target
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if _, duplicate := targets[key]; duplicate {
			return fmt.Errorf("multiple clipboard files are named %s", name)
		}
		targets[key] = struct{}{}
		if _, err := os.Lstat(target); err == nil {
			return fmt.Errorf("%s already exists", name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (ui *UI) closeFileCopyDialog() {
	if ui.fileCopy != nil && ui.fileCopy.previewCancel != nil {
		ui.fileCopy.previewCancel()
	}
	ui.fileCopy = nil
	ui.clearFileCopyHotkeyHold()
}

func (st *fileCopyState) destinationEditable() bool {
	return st != nil && !st.running && !st.dstLocked
}

func (st *fileCopyState) focusOrder() []fileCopyDialogFocus {
	if st == nil {
		return nil
	}
	order := make([]fileCopyDialogFocus, 0, 3)
	if st.destinationEditable() {
		order = append(order, fileCopyDialogFocusDestination)
	}
	order = append(order, fileCopyDialogFocusActions)
	return order
}

func (st *fileCopyState) syncEditorFocus(gtx layout.Context) {
	if st == nil || !st.destinationEditable() {
		return
	}
	if gtx.Focused(&st.dstEdit) {
		st.focus = fileCopyDialogFocusDestination
	}
}

func (st *fileCopyState) setFocus(target fileCopyDialogFocus) bool {
	if st == nil {
		return false
	}
	if target == fileCopyDialogFocusDestination && !st.destinationEditable() {
		return false
	}
	changed := st.focus != target
	st.focus = target
	switch target {
	case fileCopyDialogFocusDestination:
		st.dstEditWant = true
	default:
		st.dstEditWant = false
		st.keyFocus.focusKeyboard()
	}
	return changed
}

func (st *fileCopyState) stepFocus(step int) bool {
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

func (st *fileCopyState) stepAction(step int) bool {
	if st == nil {
		return false
	}
	if st.running {
		if st.actionFocus != fileCopyDialogActionCancel {
			st.actionFocus = fileCopyDialogActionCancel
			return true
		}
		return false
	}
	order := []fileCopyDialogAction{fileCopyDialogActionCancel, fileCopyDialogActionConfirm}
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

func (st *fileCopyState) actionVisualState(target fileCopyDialogAction) dialogActionVisualState {
	if st == nil {
		return dialogActionVisualState{}
	}
	if st.running {
		if target == fileCopyDialogActionCancel && !st.canceling {
			return dialogActionVisualState{Focused: true, Default: true}
		}
		return dialogActionVisualState{}
	}
	if st.focus == fileCopyDialogFocusActions {
		active := st.actionFocus == target
		return dialogActionVisualState{Focused: active, Default: active}
	}
	return dialogActionVisualState{Default: target == fileCopyDialogActionConfirm}
}

func (st *fileCopyState) beginCopyRun(cancel context.CancelFunc, now time.Time) {
	if st == nil {
		return
	}
	st.running = true
	st.startedAt = now
	st.speedBytes = 0
	st.speedAt = now
	st.speedDone = st.progress.BytesDone
	st.speedSeenAt = time.Time{}
	st.cancelFunc = cancel
	st.cancelUntil = time.Time{}
	st.canceling = false
	st.focus = fileCopyDialogFocusActions
	st.actionFocus = fileCopyDialogActionCancel
	st.dstEditWant = false
	st.keyFocus.focusKeyboard()
}

func (st *fileCopyState) sampleCopySpeed(now time.Time) {
	if st == nil {
		return
	}
	if st.speedAt.IsZero() {
		st.speedAt = now
		st.speedDone = st.progress.BytesDone
		st.speedBytes = 0
		st.speedSeenAt = time.Time{}
		return
	}
	elapsed := now.Sub(st.speedAt)
	if elapsed < fileCopySpeedSampleInterval {
		return
	}
	delta := st.progress.BytesDone - st.speedDone
	st.speedAt = now
	st.speedDone = st.progress.BytesDone
	if delta > 0 {
		st.speedBytes = int64(float64(delta) / elapsed.Seconds())
		st.speedSeenAt = now
		return
	}
	if st.speedSeenAt.IsZero() || now.Sub(st.speedSeenAt) >= fileCopySpeedStaleAfter {
		st.speedBytes = 0
	}
}

func (st *fileCopyState) cancelConfirmActive(now time.Time) bool {
	return st != nil && !st.cancelUntil.IsZero() && now.Before(st.cancelUntil)
}

func (st *fileCopyState) expireCancelConfirm(now time.Time) bool {
	if st == nil || st.cancelUntil.IsZero() || now.Before(st.cancelUntil) {
		return false
	}
	st.cancelUntil = time.Time{}
	return true
}

func (st *fileCopyState) cancelButtonLabel(now time.Time) string {
	if st == nil {
		return "Cancel"
	}
	if st.canceling {
		return "Canceling..."
	}
	if !st.cancelConfirmActive(now) {
		return "Cancel"
	}
	remaining := st.cancelUntil.Sub(now)
	seconds := int((remaining + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return fmt.Sprintf("Confirm %ds", seconds)
}

func (ui *UI) requestOrConfirmFileCopyCancel(now time.Time) {
	st := ui.fileCopy
	if st == nil || !st.running || st.canceling {
		return
	}
	st.actionsAnim.setPulse("cancel", now)
	st.focus = fileCopyDialogFocusActions
	st.actionFocus = fileCopyDialogActionCancel
	st.keyFocus.focusKeyboard()
	if st.cancelConfirmActive(now) {
		st.cancelUntil = time.Time{}
		st.canceling = true
		if st.cancelFunc != nil {
			st.cancelFunc()
		}
		return
	}
	st.cancelUntil = now.Add(fileCopyCancelConfirmCountdown)
}

func (ui *UI) defaultFileCopyDestination(srcIdx int, srcPane *filePaneState, srcEndpoint copyEndpoint) (int, copyEndpoint, string) {
	for i, other := range ui.filePanes {
		if i == srcIdx || other == nil {
			continue
		}
		otherEndpoint := copyEndpointFromPane(i, other)
		if otherEndpoint.isArchive() {
			continue
		}
		dstDir := strings.TrimSpace(otherEndpoint.dir)
		if dstDir == "" {
			continue
		}
		return i, otherEndpoint, dstDir
	}

	if srcEndpoint.isArchive() {
		dstDir := srcPane.archiveParentDir()
		if strings.TrimSpace(dstDir) == "" {
			dstDir = "."
		}
		return -1, copyEndpoint{pane: -1, dir: dstDir}, dstDir
	}

	dstEndpoint := copyEndpointFromPane(srcIdx, srcPane)
	dstDir := strings.TrimSpace(dstEndpoint.dir)
	if dstDir == "" {
		if dstEndpoint.isRemote() {
			dstDir = "/"
		} else {
			dstDir = "."
		}
	}
	return srcIdx, dstEndpoint, dstDir
}

func (ui *UI) clearFileCopyHotkeyHold() {
	if ui == nil || ui.held == nil {
		return
	}
	ui.held[fileActionKey(fileActionCopy)] = false
}

func (st *fileCopyState) multiSource() bool {
	return st != nil && len(st.sources) > 1
}

func (st *fileCopyState) sourceCount() int {
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

func (st *fileCopyState) sourceSummary() string {
	if st == nil {
		return ""
	}
	if st.op == fileCopyOpExtract {
		count := st.sourceCount()
		if count <= 1 {
			return st.srcPath
		}
		return fmt.Sprintf("%s (%d items)", st.srcPath, count)
	}
	count := st.sourceCount()
	if count <= 1 {
		return st.srcPath
	}
	return fmt.Sprintf("%d items selected", count)
}

func (st *fileCopyState) sourceOperationSummary() string {
	return fileOpSourceCountText(st.sourceCount())
}

func (st *fileCopyState) sourceLocation() string {
	if st == nil {
		return ""
	}
	if st.multiSource() {
		return st.sourceSummary()
	}
	if st.op == fileCopyOpExtract {
		if len(st.sources) > 0 {
			return fileOpPreviewLabel(st.sources[0].Name, st.sources[0].Path)
		}
		return st.srcEndpoint.baseName(st.srcPath)
	}
	if len(st.sources) > 0 {
		return fileOpPreviewLabel(st.sources[0].Name, st.sources[0].Path)
	}
	return st.srcEndpoint.baseName(st.srcPath)
}

func (st *fileCopyState) progressCurrentLabel() string {
	if st == nil {
		return ""
	}
	current := copyProgressCurrent(st.progress)
	if current == "" {
		return current
	}
	rootPath := strings.TrimSpace(st.progress.CurrentRootPath)
	if rootPath == "" {
		return current
	}
	rootName := st.srcEndpoint.baseName(rootPath)
	currentPath := strings.TrimSpace(st.progress.CurrentPath)
	if st.srcEndpoint.samePath(rootPath, currentPath) {
		return current
	}
	if rootName == "" || rootName == "." || rootName == string(filepath.Separator) {
		return current
	}
	return rootName + "  ›  " + current
}

func (st *fileCopyState) title() string {
	if st != nil && st.op == fileCopyOpExtract {
		return "Extract"
	}
	return "Copy"
}

func (st *fileCopyState) confirmLabel() string {
	if st == nil {
		return "Copy"
	}
	if st.op == fileCopyOpExtract {
		if st.running {
			return "Extracting..."
		}
		return "Extract"
	}
	if st.running {
		return "Copying..."
	}
	if (!st.multiSource() && st.dstInfo.Exists) || (st.multiSource() && st.conflictCount > 0) {
		return "Overwrite"
	}
	return "Copy"
}

func (st *fileCopyState) refreshPreview() {
	if st == nil {
		return
	}
	raw := strings.TrimSpace(st.dstEdit.Text())
	st.dstRaw = raw
	st.conflicts = nil
	st.conflictCount = 0
	if raw == "" {
		if st.previewCancel != nil {
			st.previewCancel()
			st.previewCancel = nil
		}
		st.previewID++
		st.previewing = false
		st.dstPath = ""
		st.dstInfo = fileCopyPathInfo{}
		st.conflicts = nil
		st.conflictCount = 0
		return
	}

	// SFTP metadata calls can take seconds on a high-latency or unhealthy
	// connection. Never perform them from Gio's event/layout goroutine.
	if st.srcEndpoint.isRemote() || st.dstEndpoint.isRemote() {
		if st.previewCancel != nil {
			st.previewCancel()
		}
		ctx, cancel := context.WithCancel(context.Background())
		st.previewCancel = cancel
		st.previewID++
		id := st.previewID
		st.previewing = true
		st.srcInfo = fileCopyPathInfo{}
		st.dstInfo = fileCopyPathInfo{}
		st.conflicts = nil
		st.conflictCount = 0
		if dstPath, err := st.dstEndpoint.normalizePath(raw); err == nil {
			st.dstPath = dstPath
		} else {
			st.dstPath = raw
		}
		st.previewCh = make(chan fileCopyPreviewResult, 1)
		ch := st.previewCh
		srcEndpoint := st.srcEndpoint
		dstEndpoint := st.dstEndpoint
		srcPath := st.srcPath
		sources := append([]fileCopySource(nil), st.sources...)
		multi := st.multiSource()
		go func() {
			timer := time.NewTimer(fileCopyRemotePreviewDebounce)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			res := fileCopyPreviewResult{id: id, raw: raw}
			if multi {
				res.dstPath, res.dstInfo, res.err = inspectCopyDestinationDir(dstEndpoint, raw)
				if res.err == nil {
					res.conflicts, res.conflictCount, res.err = inspectFileOverwriteConflicts(srcEndpoint, sources, dstEndpoint, res.dstPath)
				}
			} else {
				res.dstPath, res.srcInfo, res.dstInfo, res.err = inspectCopyPaths(srcEndpoint, srcPath, dstEndpoint, raw)
			}
			ch <- res
		}()
		return
	}
	if st.multiSource() {
		effectiveDst, dstInfo, err := inspectCopyDestinationDir(st.dstEndpoint, raw)
		if err != nil {
			st.dstPath = raw
			st.dstInfo = fileCopyPathInfo{}
			return
		}
		st.dstPath = effectiveDst
		st.dstInfo = dstInfo
		st.srcInfo = fileCopyPathInfo{}
		st.conflicts, st.conflictCount, _ = inspectFileOverwriteConflicts(st.srcEndpoint, st.sources, st.dstEndpoint, effectiveDst)
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
	st.conflicts = nil
	st.conflictCount = 0
}

func (st *fileCopyState) pumpPreview() {
	if st == nil || st.previewCh == nil {
		return
	}
	for {
		select {
		case res := <-st.previewCh:
			if res.id != st.previewID || res.raw != strings.TrimSpace(st.dstEdit.Text()) {
				continue
			}
			st.previewing = false
			st.previewCancel = nil
			if res.err != nil {
				st.dstPath = res.raw
				continue
			}
			st.dstPath = res.dstPath
			st.srcInfo = res.srcInfo
			st.dstInfo = res.dstInfo
			st.conflicts = res.conflicts
			st.conflictCount = res.conflictCount
		default:
			return
		}
	}
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
	if st.multiSource() {
		effectiveDst, dstInfo, err := inspectCopyDestinationDir(st.dstEndpoint, dst)
		if err != nil {
			st.lastErr = err.Error()
			return
		}
		conflicts, conflictCount, err := inspectFileOverwriteConflicts(st.srcEndpoint, st.sources, st.dstEndpoint, effectiveDst)
		if err != nil {
			st.lastErr = err.Error()
			return
		}
		previewMatches := sameFileOverwriteConflictPreview(st.conflicts, st.conflictCount, conflicts, conflictCount)
		st.dstPath = effectiveDst
		st.dstInfo = dstInfo
		st.conflicts = conflicts
		st.conflictCount = conflictCount
		if conflictCount > 0 && !previewMatches {
			// A collision appeared or changed since the preview was rendered.
			// Require another explicit click now that the comparison is visible.
			st.lastErr = ""
			return
		}
	}

	st.dstRaw = dst
	if st.previewCancel != nil {
		st.previewCancel()
		st.previewCancel = nil
	}
	st.previewID++
	st.previewing = false
	st.lastErr = ""
	st.progress = filesys.CopyProgress{}
	ctx, cancel := context.WithCancel(context.Background())
	st.beginCopyRun(cancel, now)

	progressCh := make(chan filesys.CopyProgress, 32)
	doneCh := make(chan error, 1)
	st.progressCh = progressCh
	st.doneCh = doneCh

	srcEndpoint := st.srcEndpoint
	dstEndpoint := st.dstEndpoint
	srcPath := st.srcPath
	sources := append([]fileCopySource(nil), st.sources...)
	multi := st.multiSource()
	go func(ctx context.Context) {
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
		if endpointsUseSameRemote(srcEndpoint, dstEndpoint) {
			remoteSources := sources
			if !multi {
				remoteSources = []fileCopySource{{Path: srcPath, Name: srcEndpoint.baseName(srcPath)}}
			}
			destinationDir := dst
			if multi {
				var err error
				destinationDir, _, err = inspectCopyDestinationDir(dstEndpoint, dst)
				if err != nil {
					doneCh <- err
					return
				}
			}
			remoteProgress := filesys.CopyProgress{
				Streaming:       true,
				ScanDone:        true,
				FilesDiscovered: len(remoteSources),
				EntriesTotal:    len(remoteSources),
			}
			sendProgress(remoteProgress)
			for _, source := range remoteSources {
				if err := ctx.Err(); err != nil {
					doneCh <- err
					return
				}
				target := destinationDir
				if multi {
					target = dstEndpoint.join(destinationDir, srcEndpoint.baseName(source.Path))
				}
				remoteProgress.CurrentPath = source.Path
				remoteProgress.CurrentRootPath = source.Path
				sendProgress(remoteProgress)
				_, err := runSameRemoteCopyContext(ctx, srcEndpoint, source.Path, dstEndpoint, target)
				if err != nil && !errors.Is(err, errRemoteCommandUnavailable) {
					doneCh <- err
					return
				}
				if err == nil {
					remoteProgress.FilesCopied++
					remoteProgress.EntriesDone++
					sendProgress(remoteProgress)
					continue
				}
				// No SSH command channel is available; retain the portable SFTP
				// implementation below.
				break
			}
			if remoteProgress.EntriesDone == len(remoteSources) {
				doneCh <- nil
				return
			}
		}

		doneCh <- runStreamingCopyContext(ctx, srcEndpoint, sources, srcPath, dstEndpoint, dst, multi, sendProgress)
	}(ctx)
}

func (ui *UI) pumpFileCopyState(gtx layout.Context) {
	st := ui.fileCopy
	if st == nil {
		return
	}
	st.pumpPreview()

	if st.running {
		if st.expireCancelConfirm(gtx.Now) {
			gtx.Execute(op.InvalidateCmd{})
		}
		for {
			select {
			case p := <-st.progressCh:
				st.progress = p
			default:
				goto doneProgress
			}
		}
	doneProgress:
		st.sampleCopySpeed(gtx.Now)
		select {
		case err := <-st.doneCh:
			if st.cancelFunc != nil {
				st.cancelFunc()
			}
			st.running = false
			st.progressCh = nil
			st.doneCh = nil
			st.cancelFunc = nil
			st.cancelUntil = time.Time{}
			st.canceling = false
			st.speedBytes = 0
			st.speedSeenAt = time.Time{}
			if err != nil {
				if errors.Is(err, context.Canceled) {
					ui.finishFileCopyCanceled(gtx.Now)
					return
				}
				if st.directPaste {
					ui.finishDirectFilePasteError(gtx.Now, err)
					return
				}
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

func (ui *UI) finishDirectFilePasteError(now time.Time, err error) {
	st := ui.fileCopy
	if st == nil {
		return
	}
	paneIdx := st.dstPane
	ui.fileCopy = nil
	ui.clearFileCopyHotkeyHold()
	if paneIdx < 0 || paneIdx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[paneIdx]
	if pane == nil {
		return
	}
	notice := "paste failed"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		notice += ": " + err.Error()
	}
	if pane.table == nil {
		pane.setNotice(notice, now)
		return
	}
	selectedPath := ""
	if selected := pane.selectedEntry(); selected != nil {
		selectedPath = selected.Path
	}
	restorePos := sanitizePaneListPosition(pane.table.List.Position)
	restoreAnchor := pane.visibleAnchorPath()
	if !ui.requestPaneLoadWithSelectionAndScroll(
		paneIdx,
		pane.dir,
		selectedPath,
		"",
		pane.table.Selected,
		restorePos,
		true,
		restoreAnchor,
		notice,
		fileCopyCanceledNoticeDur,
	) {
		pane.setNotice(notice, now)
	}
}

func (ui *UI) finishFileCopyCanceled(now time.Time) {
	st := ui.fileCopy
	if st == nil {
		return
	}

	srcPaneIdx := st.srcPane
	dstPaneIdx := st.dstPane
	noticePaneIdx := dstPaneIdx
	if noticePaneIdx < 0 {
		noticePaneIdx = srcPaneIdx
	}
	noticeText := "copy canceled"
	ui.fileCopy = nil
	ui.clearFileCopyHotkeyHold()

	noticeShown := false
	reloadPane := func(paneIdx int) {
		if paneIdx < 0 || paneIdx >= len(ui.filePanes) {
			return
		}
		pane := ui.filePanes[paneIdx]
		if pane == nil || pane.table == nil {
			return
		}
		selectedPath := ""
		if sel := pane.selectedEntry(); sel != nil {
			selectedPath = sel.Path
		}
		restorePos := sanitizePaneListPosition(pane.table.List.Position)
		restoreAnchor := pane.visibleAnchorPath()
		reloadNoticeText := ""
		reloadNoticeDur := time.Duration(0)
		if paneIdx == noticePaneIdx {
			reloadNoticeText = noticeText
			reloadNoticeDur = fileCopyCanceledNoticeDur
		}
		if ui.requestPaneLoadWithSelectionAndScroll(paneIdx, pane.dir, selectedPath, "", pane.table.Selected, restorePos, true, restoreAnchor, reloadNoticeText, reloadNoticeDur) && paneIdx == noticePaneIdx {
			noticeShown = true
		}
	}
	reloadPane(srcPaneIdx)
	if dstPaneIdx != srcPaneIdx {
		reloadPane(dstPaneIdx)
	}
	ui.setActiveFilePane(srcPaneIdx)
	if !noticeShown && noticeText != "" && noticePaneIdx >= 0 && noticePaneIdx < len(ui.filePanes) && ui.filePanes[noticePaneIdx] != nil {
		ui.filePanes[noticePaneIdx].setNoticeFor(noticeText, now, fileCopyCanceledNoticeDur)
	}
}

func (ui *UI) finishFileCopy(now time.Time) {
	st := ui.fileCopy
	if st == nil {
		return
	}

	srcPaneIdx := st.srcPane
	dstPaneIdx := st.dstPane
	noticeText, noticeDur := fileCopySuccessNotice(st)
	noticePaneIdx := dstPaneIdx
	if noticePaneIdx < 0 {
		noticePaneIdx = srcPaneIdx
	}
	ui.maybePlayBackgroundOperationSound(st.startedAt, now)
	ui.fileCopy = nil // close dialog first
	ui.clearFileCopyHotkeyHold()

	noticeShown := false
	reloadPane := func(paneIdx int) {
		if paneIdx < 0 || paneIdx >= len(ui.filePanes) {
			return
		}
		pane := ui.filePanes[paneIdx]
		if pane == nil || pane.table == nil {
			return
		}
		selectedPath := ""
		if sel := pane.selectedEntry(); sel != nil {
			selectedPath = sel.Path
		}
		restorePos := sanitizePaneListPosition(pane.table.List.Position)
		restoreAnchor := pane.visibleAnchorPath()
		reloadNoticeText := ""
		reloadNoticeDur := time.Duration(0)
		if paneIdx == noticePaneIdx {
			reloadNoticeText = noticeText
			reloadNoticeDur = noticeDur
		}
		if ui.requestPaneLoadWithSelectionAndScroll(paneIdx, pane.dir, selectedPath, "", pane.table.Selected, restorePos, true, restoreAnchor, reloadNoticeText, reloadNoticeDur) && paneIdx == noticePaneIdx {
			noticeShown = true
		}
	}
	reloadPane(srcPaneIdx)
	if dstPaneIdx != srcPaneIdx {
		reloadPane(dstPaneIdx)
	}
	ui.setActiveFilePane(srcPaneIdx)
	if !noticeShown && noticeText != "" && noticePaneIdx >= 0 && noticePaneIdx < len(ui.filePanes) && ui.filePanes[noticePaneIdx] != nil {
		ui.filePanes[noticePaneIdx].setNoticeFor(noticeText, now, noticeDur)
	}
}

func fileCopySuccessNotice(st *fileCopyState) (string, time.Duration) {
	if st == nil {
		return "", 0
	}
	count := st.sourceCount()
	if count <= 0 {
		return "", 0
	}
	label := "items"
	if count == 1 {
		label = "item"
	}
	msg := fmt.Sprintf("copied %d %s", count, label)
	if st.progress.Streaming {
		return msg, fileCopySuccessNoticeDur
	}
	if nestedCount := st.progress.EntriesTotal - count; nestedCount > 0 {
		nestedLabel := "nested items"
		if nestedCount == 1 {
			nestedLabel = "nested item"
		}
		msg = fmt.Sprintf("%s (%d %s)", msg, nestedCount, nestedLabel)
	}
	return msg, fileCopySuccessNoticeDur
}

func (ui *UI) layoutFileCopyDialog(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.fileCopy
	if st == nil {
		return layout.Dimensions{}
	}
	if st.directPaste {
		return layout.Dimensions{}
	}

	st.keyFocus.attach(gtx)
	st.syncEditorFocus(gtx)
	if st.expireCancelConfirm(gtx.Now) {
		gtx.Execute(op.InvalidateCmd{})
	}
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
		ui.submitFileCopyDialog(gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}

	for {
		filters := []event.Filter{
			key.Filter{Name: key.NameEscape, Optional: anyMods},
		}
		if st.running {
			filters = append(filters,
				key.Filter{Name: key.NameEnter, Optional: anyMods},
				key.Filter{Name: key.NameReturn, Optional: anyMods},
			)
		} else {
			filters = append(filters, key.Filter{Name: key.NameTab, Optional: anyMods})
		}
		if !st.running && st.focus == fileCopyDialogFocusActions {
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
				ui.requestOrConfirmFileCopyCancel(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			ui.closeFileCopyDialog()
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
			if st.running || ke.Modifiers != 0 || st.focus != fileCopyDialogFocusActions {
				continue
			}
			if st.stepAction(-1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameRightArrow:
			if st.running || ke.Modifiers != 0 || st.focus != fileCopyDialogFocusActions {
				continue
			}
			if st.stepAction(1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameEnter, key.NameReturn:
			if st.running {
				if ke.Modifiers == 0 {
					ui.requestOrConfirmFileCopyCancel(gtx.Now)
					gtx.Execute(op.InvalidateCmd{})
				}
				continue
			}
			if ke.Modifiers != 0 {
				continue
			}
			if st.focus != fileCopyDialogFocusActions {
				continue
			}
			switch st.actionFocus {
			case fileCopyDialogActionCancel:
				st.actionsAnim.setPulse("cancel", gtx.Now)
				ui.closeFileCopyDialog()
				return layout.Dimensions{}
			case fileCopyDialogActionConfirm:
				st.actionsAnim.setPulse("confirm", gtx.Now)
				ui.submitFileCopyDialog(gtx.Now)
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
			ui.closeFileCopyDialog()
			return layout.Dimensions{}
		}
		if st.closeClick.Clicked(gtx) {
			ui.closeFileCopyDialog()
			return layout.Dimensions{}
		}
		if st.confirmClick.Clicked(gtx) {
			st.actionsAnim.setPulse("confirm", gtx.Now)
			ui.submitFileCopyDialog(gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		}
	} else {
		for st.cancelClick.Clicked(gtx) {
			ui.requestOrConfirmFileCopyCancel(gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		}
		for st.closeClick.Clicked(gtx) {
			ui.requestOrConfirmFileCopyCancel(gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
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

		dialogWidth := unit.Dp(500)
		if !st.running && ((!st.multiSource() && st.dstInfo.Exists) || st.conflictCount > 0) {
			dialogWidth = unit.Dp(620)
		}
		width := gtx.Dp(ui.scaleInterfaceDp(dialogWidth))
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

		if st.running || st.previewing {
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(33 * time.Millisecond)})
		}
		return layout.Dimensions{Size: gtx.Constraints.Max, Baseline: dialog.Baseline}
	})
}

func (ui *UI) layoutFileCopyDialogBody(th *material.Theme, gtx layout.Context, st *fileCopyState) layout.Dimensions {
	hoverActionKey := ""
	if (!st.running || !st.canceling) && st.cancelClick.Hovered() {
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

	progress := st.progress
	progressDisplay := buildFileCopyProgressDisplay(progress, st.speedBytes)
	current := st.progressCurrentLabel()
	progressFrac := copyProgressFraction(progress)
	sourceLabel := "Copy"
	sourceValue := st.sourceOperationSummary()
	if st.running {
		sourceLabel = "Copying"
	}
	if st.op == fileCopyOpExtract {
		sourceLabel = "Archive"
		sourceValue = st.sourceLocation()
	}
	overwriteLabel := ""
	if !st.running && st.dstInfo.Exists && !st.multiSource() {
		overwriteLabel = "Destination exists. Overwrite will replace it."
	} else if !st.running && st.multiSource() && st.conflictCount > 0 {
		overwriteLabel = fmt.Sprintf("%d existing %s will be overwritten.", st.conflictCount, pluralizeFileDestinations(st.conflictCount))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					title := material.Body1(th, st.title())
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
			return ui.layoutFileOpTextRow(th, gtx, sourceLabel, sourceValue, txtColor)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileOpRow(th, gtx, "To", func(gtx layout.Context) layout.Dimensions {
				if st.running || st.dstLocked {
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
				return ui.layoutEditorWithContextMenu(th, gtx, "filecopy-dst", &st.dstEdit, true, func(gtx layout.Context) layout.Dimensions {
					return layoutNeutralEditorBox(gtx, st.focus == fileCopyDialogFocusDestination, true, ed.Layout)
				})
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.running {
				return layout.Dimensions{}
			}
			gtx = fileOpDialogReserveFooter(gtx)
			if st.multiSource() {
				return ui.layoutFileOverwriteConflicts(th, gtx, st.conflicts, st.conflictCount, &st.conflictList)
			}
			if !st.dstInfo.Exists {
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
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.TextSize = ui.scaleDialogFontSize(9)
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
				return ui.layoutFileCopyProgress(th, gtx, progressFrac, current, progressDisplay)
			}
			lbl := material.Body2(th, st.lastErr)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleDialogFontSize(10)
			lbl.Color = color.NRGBA{R: 220, G: 140, B: 140, A: 255}
			lbl.MaxLines = 2
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layoutDialogHorizontalDivider),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if st.running {
					return ui.layoutDialogActionSingle(
						th, gtx,
						&st.cancelClick, st.cancelButtonLabel(gtx.Now), hoverCancel, pulseCancel, st.canceling,
						st.actionVisualState(fileCopyDialogActionCancel),
					)
				}
				return ui.layoutDialogActionPair(
					th, gtx,
					&st.cancelClick, "Cancel", hoverCancel, pulseCancel, st.running,
					&st.confirmClick, st.confirmLabel(), hoverConfirm, pulseConfirm, st.running,
					st.actionVisualState(fileCopyDialogActionCancel),
					st.actionVisualState(fileCopyDialogActionConfirm),
				)
			})
		}),
	)
}

func fileOpDialogReserveFooter(gtx layout.Context) layout.Context {
	const footerHeight = unit.Dp(64)
	gtx.Constraints.Min.Y = 0
	gtx.Constraints.Max.Y = max(1, gtx.Constraints.Max.Y-gtx.Dp(footerHeight))
	return gtx
}

type fileCopyProgressDisplay struct {
	Indeterminate  bool
	PrimaryLabel   string
	PrimaryValue   string
	SecondaryLabel string
	SecondaryValue string
}

func (ui *UI) layoutFileCopyProgress(th *material.Theme, gtx layout.Context, frac float32, current string, display fileCopyProgressDisplay) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if current == "" {
				return layout.Dimensions{}
			}
			return ui.layoutFileOpTextRow(th, gtx, "Current", current, txtColor)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFileCopyTextBar(th, gtx, frac, display.Indeterminate)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if display.PrimaryValue == "" {
				return layout.Dimensions{}
			}
			return ui.layoutFileOpTextRow(th, gtx, display.PrimaryLabel, display.PrimaryValue, txtColor)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if display.SecondaryValue == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFileOpTextRow(th, gtx, display.SecondaryLabel, display.SecondaryValue, color.NRGBA{R: 184, G: 184, B: 184, A: 255})
			})
		}),
	)
}

func (ui *UI) layoutFileCopyOverwriteInfo(th *material.Theme, gtx layout.Context, st *fileCopyState) layout.Dimensions {
	return ui.layoutFileOverwriteDiffInfo(th, gtx, "Overwrite Details", st.srcInfo, st.dstInfo)
}

func pluralizeFileDestinations(count int) string {
	if count == 1 {
		return "destination"
	}
	return "destinations"
}

type fileOverwriteDiffStyle struct {
	Title color.NRGBA
	Text  color.NRGBA
	Muted color.NRGBA
	Newer color.NRGBA
	Older color.NRGBA
	Same  color.NRGBA
}

func (ui *UI) layoutFileOverwriteDiffInfo(th *material.Theme, gtx layout.Context, title string, srcInfo, dstInfo fileCopyPathInfo) layout.Dimensions {
	name := path.Base(filepath.ToSlash(srcInfo.Path))
	if name == "." || name == "/" || strings.TrimSpace(name) == "" {
		name = "item"
	}
	return ui.layoutFileOverwriteConflictPanel(th, gtx, title, []fileOverwriteConflict{{
		Name: name, SrcInfo: srcInfo, DstInfo: dstInfo,
	}}, nil)
}

func (ui *UI) layoutFileOverwriteConflicts(th *material.Theme, gtx layout.Context, conflicts []fileOverwriteConflict, total int, listState *widget.List) layout.Dimensions {
	if total <= 0 || len(conflicts) == 0 {
		return layout.Dimensions{}
	}
	title := fmt.Sprintf("Overwrite Details • %d %s", total, pluralizeFileDestinations(total))
	return ui.layoutFileOverwriteConflictPanel(th, gtx, title, conflicts, listState)
}

func (ui *UI) layoutFileOverwriteConflictPanel(th *material.Theme, gtx layout.Context, title string, conflicts []fileOverwriteConflict, listState *widget.List) layout.Dimensions {
	if len(conflicts) == 0 {
		return layout.Dimensions{}
	}
	if listState == nil {
		listState = &widget.List{}
	}
	listState.Axis = layout.Vertical
	panelBg := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
	panelBorder := color.NRGBA{R: 255, G: 255, B: 255, A: 18}
	style := ui.fileOverwriteDiffStyle(panelBg)
	weights := ui.fileOverwriteLedgerWeightsFor(th, gtx, conflicts)
	olderCount := fileOverwriteOlderSourceCount(conflicts)
	entry := func(gtx layout.Context, index int) layout.Dimensions {
		return ui.layoutFileOverwriteLedgerEntry(th, gtx, conflicts[index], style, weights)
	}
	measure := op.Record(gtx.Ops)
	entryDims := entry(gtx, 0)
	measure.Stop()
	visibleCount := min(fileOverwriteConflictVisibleLimit, len(conflicts))
	viewportHeight := entryDims.Size.Y * visibleCount

	return fillFlatBox(
		gtx,
		panelBg,
		panelBorder,
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, title)
						lbl.Font.Typeface = ui.interfaceTypeface()
						lbl.TextSize = ui.fileOpDialogTextSize()
						lbl.Color = style.Title
						return lbl.Layout(gtx)
					}),
				}
				if olderCount > 0 {
					children = append(children,
						layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							warning := fmt.Sprintf("⚠ %d older %s will replace newer destination data", olderCount, fileOpItemWord(olderCount))
							lbl := material.Caption(th, warning)
							lbl.Font.Typeface = ui.interfaceTypeface()
							lbl.TextSize = ui.fileOpDialogTextSize()
							lbl.Color = style.Older
							lbl.MaxLines = 1
							lbl.Truncator = "…"
							return lbl.Layout(gtx)
						}),
					)
				}
				children = append(children,
					layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						height := min(viewportHeight, gtx.Constraints.Max.Y)
						return ui.layoutFileOverwriteLedger(th, gtx, conflicts, style, weights, listState, height, entryDims.Size.Y, entry)
					}),
				)
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		},
	)
}

func fileOpItemWord(count int) string {
	if count == 1 {
		return "source"
	}
	return "sources"
}

func (ui *UI) layoutFileOverwriteLedgerEntry(th *material.Theme, gtx layout.Context, conflict fileOverwriteConflict, style fileOverwriteDiffStyle, weights [5]float32) layout.Dimensions {
	// The destination is the value being replaced (OLD); the source is the
	// incoming replacement (NEW). STAT therefore compares NEW against OLD.
	status, relation := fileOverwriteDiffStatus(conflict.SrcInfo.ModTime, conflict.DstInfo.ModTime)
	statusColor := style.Same
	boldOld := false
	boldNew := false
	switch relation {
	case fileOverwriteSourceNewer:
		statusColor = style.Newer
		boldNew = true
	case fileOverwriteSourceOlder:
		statusColor = style.Older
		boldOld = true
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileOverwriteLedgerPairCells(gtx, [5]layout.Widget{
				ui.fileOverwriteTableFilename(th, conflict.Name, style.Text),
				ui.fileOverwriteLedgerPair(
					ui.fileOverwriteTableText(th, "OLD", style.Muted, false),
					ui.fileOverwriteTableText(th, "NEW", style.Muted, false),
				),
				ui.fileOverwriteLedgerPairValue(
					th,
					fileOverwriteSizeText(conflict.DstInfo),
					fileOverwriteSizeText(conflict.SrcInfo),
					style.Text,
					style.Muted,
					boldOld,
					boldNew,
				),
				ui.fileOverwriteLedgerPairValue(
					th,
					fileOverwriteTimestampText(conflict.DstInfo.ModTime),
					fileOverwriteTimestampText(conflict.SrcInfo.ModTime),
					style.Text,
					style.Muted,
					boldOld,
					boldNew,
				),
				ui.fileOverwriteLedgerPair(
					ui.fileOverwriteTableText(th, "", style.Text, false),
					ui.fileOverwriteTableText(th, status, statusColor, false),
				),
			}, weights)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFileOverwriteRule(gtx, style.Muted)
			})
		}),
	)
}

func (ui *UI) layoutFileOverwriteLedgerPairCells(gtx layout.Context, cells [5]layout.Widget, weights [5]float32) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(cells))
	for i := range cells {
		i := i
		children = append(children, layout.Flexed(weights[i], func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(4)}.Layout(gtx, cells[i])
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (ui *UI) fileOverwriteLedgerPair(top, bottom layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, top)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, bottom)
			}),
		)
	}
}

// fileOverwriteLedgerPairValue draws one continuous old-to-new
// connector. Keeping it as vector geometry avoids gaps and alignment changes
// between fonts that render box-drawing and return-arrow glyphs differently.
func (ui *UI) fileOverwriteLedgerPairValue(th *material.Theme, oldValue, newValue string, textColor, connectorColor color.NRGBA, boldOld, boldNew bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		cellWidth := gtx.Constraints.Max.X
		connectorWidth := gtx.Dp(fileOverwriteConnectorWidthDp)
		connectorInset := fileOverwriteConnectorWidthDp + fileOverwriteConnectorGapDp
		macro := op.Record(gtx.Ops)
		contentGtx := gtx
		contentGtx.Constraints.Min.X = 0
		dims := ui.fileOverwriteLedgerPair(
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: connectorInset}.Layout(gtx, ui.fileOverwriteTableText(th, oldValue, textColor, boldOld))
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: connectorInset}.Layout(gtx, ui.fileOverwriteTableText(th, newValue, textColor, boldNew))
			},
		)(contentGtx)
		content := macro.Stop()
		content.Add(gtx.Ops)

		if dims.Size.X <= connectorWidth || dims.Size.Y < 4 {
			dims.Size.X = cellWidth
			return dims
		}
		x := float32(dims.Size.X) - 1.5
		arm := float32(max(4, connectorWidth-3))
		topY := float32(dims.Size.Y) * 0.25
		bottomY := float32(dims.Size.Y) * 0.75
		arrow := float32(min(3, max(2, connectorWidth/4)))
		var connector clip.Path
		connector.Begin(gtx.Ops)
		connector.MoveTo(f32.Pt(x-arm, topY))
		connector.LineTo(f32.Pt(x, topY))
		connector.LineTo(f32.Pt(x, bottomY))
		connector.LineTo(f32.Pt(x-arm, bottomY))
		connector.MoveTo(f32.Pt(x-arm+arrow, bottomY-arrow))
		connector.LineTo(f32.Pt(x-arm, bottomY))
		connector.LineTo(f32.Pt(x-arm+arrow, bottomY+arrow))
		paint.FillShape(gtx.Ops, connectorColor, clip.Stroke{Path: connector.End(), Width: 1}.Op())
		dims.Size.X = cellWidth
		return dims
	}
}

func (ui *UI) fileOverwriteLedgerWeightsFor(th *material.Theme, gtx layout.Context, conflicts []fileOverwriteConflict) [5]float32 {
	const (
		baseFile = float32(0.16)
		maxFile  = float32(0.38)
		state    = float32(0.10)
		stat     = float32(0.14)
		minSize  = float32(0.12)
		minTime  = float32(0.24)
	)
	maxNameWidth := 0
	maxSizeWidth := 0
	maxTimeWidth := 0
	measureValue := func(text string) int {
		lbl := material.Caption(th, text)
		lbl.Font.Typeface = ui.interfaceTypeface()
		lbl.Font.Weight = font.Bold
		lbl.TextSize = ui.fileOpDialogTextSize()
		lbl.MaxLines = 1
		return measureLabelUnconstrained(gtx, lbl).Size.X
	}
	for _, conflict := range conflicts {
		lbl := material.Caption(th, conflict.Name)
		lbl.Font.Typeface = ui.interfaceTypeface()
		lbl.TextSize = ui.fileOpDialogTextSize()
		lbl.MaxLines = 1
		if width := measureLabelUnconstrained(gtx, lbl).Size.X; width > maxNameWidth {
			maxNameWidth = width
		}
		for _, value := range []string{fileOverwriteSizeText(conflict.DstInfo), fileOverwriteSizeText(conflict.SrcInfo)} {
			if width := measureValue(value); width > maxSizeWidth {
				maxSizeWidth = width
			}
		}
		for _, value := range []string{fileOverwriteTimestampText(conflict.DstInfo.ModTime), fileOverwriteTimestampText(conflict.SrcInfo.ModTime)} {
			if width := measureValue(value); width > maxTimeWidth {
				maxTimeWidth = width
			}
		}
	}
	tableWidth := max(1, gtx.Constraints.Max.X)
	file := float32(maxNameWidth+gtx.Dp(unit.Dp(16))) / float32(tableWidth)
	if file < baseFile {
		file = baseFile
	}
	if file > maxFile {
		file = maxFile
	}
	// Include breathing room for flex rounding and for the scrollbar that is
	// present when the conflict list exceeds five entries.
	valueChrome := gtx.Dp(fileOverwriteCellInsetDp + fileOverwriteConnectorWidthDp + fileOverwriteConnectorGapDp + unit.Dp(10))
	size := max(minSize, float32(maxSizeWidth+valueChrome)/float32(tableWidth))
	modified := max(minTime, float32(maxTimeWidth+valueChrome)/float32(tableWidth))
	if capacity := 1 - state - stat - size - modified; file > capacity {
		file = max(baseFile, capacity)
	}
	if total := file + state + size + modified + stat; total < 1 {
		modified += 1 - total
	}
	return [5]float32{file, state, size, modified, stat}
}

func (ui *UI) layoutFileOverwriteLedger(th *material.Theme, gtx layout.Context, conflicts []fileOverwriteConflict, style fileOverwriteDiffStyle, weights [5]float32, listState *widget.List, viewportHeight, entryHeight int, entry layout.ListElement) layout.Dimensions {
	scrolling := len(conflicts) > fileOverwriteConflictVisibleLimit
	scrollbarStyle := settingsScrollbarStyle(th, &listState.Scrollbar)
	scrollbarWidth := 0
	if scrolling {
		scrollbarWidth = gtx.Dp(scrollbarStyle.Width())
	}
	tableWidth := max(1, gtx.Constraints.Max.X-scrollbarWidth)
	header := func(gtx layout.Context) layout.Dimensions {
		return fixedWidth(gtx, tableWidth, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFileOverwriteLedgerCells(gtx, [5]layout.Widget{
				ui.fileOverwriteTableText(th, "FILE", style.Muted, false),
				ui.fileOverwriteTableText(th, "STATE", style.Muted, false),
				ui.fileOverwriteTableText(th, "SIZE", style.Muted, false),
				ui.fileOverwriteTableText(th, "MODIFIED TIME", style.Muted, false),
				ui.fileOverwriteTableText(th, "STAT", style.Muted, false),
			}, weights)
		})
	}
	measure := op.Record(gtx.Ops)
	headerDims := header(gtx)
	measure.Stop()
	ruleHeight := 1
	maxViewportHeight := gtx.Constraints.Max.Y - headerDims.Size.Y - ruleHeight
	if maxViewportHeight < 1 {
		maxViewportHeight = 1
	}
	viewportHeight = min(max(1, viewportHeight), maxViewportHeight)
	if entryHeight > 0 && entryHeight <= maxViewportHeight {
		viewportHeight = max(entryHeight, (viewportHeight/entryHeight)*entryHeight)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(header),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, tableWidth, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFileOverwriteRule(gtx, style.Muted)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, viewportHeight, func(gtx layout.Context) layout.Dimensions {
				if !scrolling {
					return listState.List.Layout(gtx, len(conflicts), entry)
				}
				listStyle := material.List(th, listState)
				listStyle.AnchorStrategy = material.Occupy
				listStyle.ScrollbarStyle = scrollbarStyle
				return listStyle.Layout(gtx, len(conflicts), entry)
			})
		}),
	)
}

func (ui *UI) layoutFileOverwriteLedgerCells(gtx layout.Context, cells [5]layout.Widget, weights [5]float32) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(cells))
	for i := range cells {
		i := i
		children = append(children, layout.Flexed(weights[i], func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(4), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, cells[i])
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (ui *UI) layoutFileOverwriteRule(gtx layout.Context, ruleColor color.NRGBA) layout.Dimensions {
	width := max(1, gtx.Constraints.Max.X)
	paint.FillShape(gtx.Ops, ruleColor, clip.Rect(image.Rect(0, 0, width, 1)).Op())
	return layout.Dimensions{Size: image.Pt(width, 1)}
}

func (ui *UI) fileOverwriteTableFilename(th *material.Theme, text string, textColor color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lbl := material.Caption(th, text)
		lbl.Font.Typeface = ui.interfaceTypeface()
		lbl.TextSize = ui.fileOpDialogTextSize()
		lbl.Color = textColor
		lbl.MaxLines = 1
		lbl.Text = middleTruncateLabelToFit(gtx, lbl, text)
		lbl.Truncator = ""
		return lbl.Layout(gtx)
	}
}

func middleTruncateLabelToFit(gtx layout.Context, lbl material.LabelStyle, text string) string {
	maxWidth := gtx.Constraints.Max.X
	if text == "" || maxWidth <= 0 {
		return text
	}
	lbl.Text = text
	if measureLabelUnconstrained(gtx, lbl).Size.X <= maxWidth {
		return text
	}
	runes := []rune(text)
	best := ""
	lo, hi := 1, len(runes)
	for lo <= hi {
		mid := (lo + hi) / 2
		candidate := middleTruncateRunes(text, mid)
		lbl.Text = candidate
		if measureLabelUnconstrained(gtx, lbl).Size.X <= maxWidth {
			best = candidate
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}

func (ui *UI) fileOverwriteTableText(th *material.Theme, text string, textColor color.NRGBA, bold bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lbl := material.Caption(th, text)
		lbl.Font.Typeface = ui.interfaceTypeface()
		if bold {
			lbl.Font.Weight = font.Bold
		}
		lbl.TextSize = ui.fileOpDialogTextSize()
		lbl.Color = textColor
		lbl.MaxLines = 1
		return lbl.Layout(gtx)
	}
}

func (ui *UI) fileOverwriteDiffStyle(panelBg color.NRGBA) fileOverwriteDiffStyle {
	popup := ui.filePanePopupTheme()
	text := bestContrastColor(panelBg, popup.Text, txtColor)
	muted := mixNRGBA(text, panelBg, 0.42)
	muted.A = 220
	title := bestContrastColor(panelBg, popup.Title, text)

	return fileOverwriteDiffStyle{
		Title: title,
		Text:  text,
		Muted: muted,
		Newer: color.NRGBA{R: 126, G: 210, B: 160, A: 255},
		Older: color.NRGBA{R: 238, G: 194, B: 92, A: 255},
		Same:  color.NRGBA{R: 255, G: 255, B: 255, A: 255},
	}
}

type fileOverwriteTimeRelation uint8

const (
	fileOverwriteTimeUnknown fileOverwriteTimeRelation = iota
	fileOverwriteTimeSame
	fileOverwriteSourceNewer
	fileOverwriteSourceOlder
)

func fileOverwriteDiffStatus(newTime, oldTime time.Time) (string, fileOverwriteTimeRelation) {
	if newTime.IsZero() || oldTime.IsZero() {
		return "? UNKNOWN", fileOverwriteTimeUnknown
	}
	if newTime.After(oldTime) {
		return "▲ NEWER", fileOverwriteSourceNewer
	}
	if newTime.Before(oldTime) {
		return "▼ OLDER", fileOverwriteSourceOlder
	}
	return "● SAME", fileOverwriteTimeSame
}

func fileOverwriteClockText(t time.Time) string {
	if t.IsZero() {
		return "n/a"
	}
	return t.Format("15:04")
}

func fileOverwriteTimestampText(t time.Time) string {
	if t.IsZero() {
		return "n/a"
	}
	return t.Format("2006-01-02 15:04:05")
}

func fileOverwriteOlderSourceCount(conflicts []fileOverwriteConflict) int {
	count := 0
	for _, conflict := range conflicts {
		_, relation := fileOverwriteDiffStatus(conflict.SrcInfo.ModTime, conflict.DstInfo.ModTime)
		if relation == fileOverwriteSourceOlder {
			count++
		}
	}
	return count
}

func fileOverwriteSizeText(info fileCopyPathInfo) string {
	if !info.Exists {
		return "missing"
	}
	if info.IsDir {
		return "dir"
	}
	return formatCopySize(info.Size)
}

func (ui *UI) layoutDialogActionSegment(th *material.Theme, gtx layout.Context, click *widget.Clickable, label string, hoverFill, pulseFill float32, segW, stripH int, roundLeft, roundRight, disabled bool, state dialogActionVisualState) layout.Dimensions {
	if click == nil {
		return layout.Dimensions{}
	}
	hoverFill = clamp01(hoverFill)
	pulseFill = clamp01(pulseFill)
	focusFill := float32(0)
	defaultFill := float32(0)
	if state.Focused {
		focusFill = 1
	}
	if state.Default {
		defaultFill = 1
	}
	if disabled {
		hoverFill = 0
		pulseFill = 0
		if focusFill == 0 {
			defaultFill = 0
		}
	}
	dims := fixedWidth(gtx, segW, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if click.Pressed() && !disabled && pulseFill < 0.5 {
					pulseFill = 0.5
				}

				hoverDark := color.NRGBA{R: 34, G: 34, B: 34, A: 255}
				pulseCol := color.NRGBA{R: 48, G: 48, B: 48, A: 255}
				defaultCol := color.NRGBA{R: 60, G: 54, B: 44, A: 255}
				focusCol := color.NRGBA{R: 72, G: 66, B: 54, A: 255}

				bg := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
				bg = mixNRGBA(bg, hoverDark, hoverFill)
				bg = mixNRGBA(bg, pulseCol, pulseFill*0.3)
				bg = mixNRGBA(bg, defaultCol, defaultFill*0.62)
				bg = mixNRGBA(bg, focusCol, focusFill*0.36)
				fg := mixNRGBA(txtColor, color.NRGBA{R: 236, G: 236, B: 236, A: 255}, hoverFill*0.75)
				fg = mixNRGBA(fg, color.NRGBA{R: 248, G: 248, B: 248, A: 255}, pulseFill*0.25)
				fg = mixNRGBA(fg, color.NRGBA{R: 244, G: 234, B: 206, A: 255}, defaultFill*0.32)
				fg = mixNRGBA(fg, color.NRGBA{R: 250, G: 246, B: 236, A: 255}, focusFill*0.3)

				if disabled {
					disabledBg := color.NRGBA{R: 24, G: 24, B: 24, A: 170}
					disabledFg := color.NRGBA{R: 160, G: 166, B: 180, A: 255}
					bg = disabledBg
					fg = disabledFg
					if focusFill > 0 || defaultFill > 0 {
						bg = mixNRGBA(bg, defaultCol, defaultFill*0.52)
						bg = mixNRGBA(bg, focusCol, focusFill*0.32)
						fg = mixNRGBA(fg, color.NRGBA{R: 244, G: 234, B: 206, A: 255}, clamp01(defaultFill*0.24+focusFill*0.24))
					}
				}

				radius := gtx.Dp(unit.Dp(filePaneControlCornerDp - 1))
				return fillSegmentBg(gtx, bg, radius, roundLeft, roundRight, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(9), Right: unit.Dp(9), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body2(th, label)
							lbl.Font.Typeface = ui.interfaceTypeface()
							lbl.Font.Weight = font.Medium
							lbl.TextSize = ui.scaleDialogFontSize(10)
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
	if defaultFill > 0 {
		radius := gtx.Dp(unit.Dp(filePaneControlCornerDp - 1))
		rr := clip.RRect{Rect: image.Rect(0, 0, dims.Size.X, dims.Size.Y)}
		if roundLeft {
			rr.NW = radius
			rr.SW = radius
		}
		if roundRight {
			rr.NE = radius
			rr.SE = radius
		}
		outline := color.NRGBA{R: 200, G: 182, B: 144, A: uint8(88 + 64*defaultFill)}
		paint.FillShape(gtx.Ops, outline, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())
	}
	if focusFill > 0 {
		yPad := gtx.Dp(unit.Dp(3))
		if yPad*2 >= dims.Size.Y {
			yPad = 0
		}
		w := gtx.Dp(unit.Dp(3))
		if w < 1 {
			w = 1
		}
		x := gtx.Dp(unit.Dp(2))
		if x+w > dims.Size.X {
			x = 0
		}
		rect := image.Rect(x, yPad, x+w, dims.Size.Y-yPad)
		if rect.Dx() > 0 && rect.Dy() > 0 {
			paint.FillShape(gtx.Ops, color.NRGBA{R: 214, G: 198, B: 166, A: uint8(160 + 64*focusFill)}, clip.UniformRRect(rect, w).Op(gtx.Ops))
		}
	}
	if !disabled {
		defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	}
	return dims
}

func (ui *UI) dialogActionSegmentMetricsPx(th *material.Theme, gtx layout.Context, label string) (width, height int) {
	lbl := material.Body2(th, strings.TrimSpace(label))
	lbl.Font.Typeface = ui.interfaceTypeface()
	lbl.Font.Weight = font.Medium
	lbl.TextSize = ui.scaleDialogFontSize(10)
	lbl.MaxLines = 1
	dims := measureLabelUnconstrained(gtx, lbl)

	width = dims.Size.X + gtx.Dp(unit.Dp(22))
	minW := gtx.Dp(unit.Dp(64))
	if width < minW {
		width = minW
	}

	height = dims.Size.Y + gtx.Dp(unit.Dp(6))
	minH := gtx.Dp(unit.Dp(22))
	if height < minH {
		height = minH
	}
	return width, height
}

func (ui *UI) layoutDialogActionPair(th *material.Theme, gtx layout.Context, leftClick *widget.Clickable, leftLabel string, leftHover, leftPulse float32, leftDisabled bool, rightClick *widget.Clickable, rightLabel string, rightHover, rightPulse float32, rightDisabled bool, leftState, rightState dialogActionVisualState) layout.Dimensions {
	return ui.layoutDialogActionPairState(th, gtx, leftClick, leftLabel, leftHover, leftPulse, leftDisabled, rightClick, rightLabel, rightHover, rightPulse, rightDisabled, leftState, rightState)
}

func (ui *UI) layoutDialogActionSingle(th *material.Theme, gtx layout.Context, click *widget.Clickable, label string, hover, pulse float32, disabled bool, state dialogActionVisualState) layout.Dimensions {
	segW, stripH := ui.dialogActionSegmentMetricsPx(th, gtx, label)
	maxW := gtx.Constraints.Max.X
	if maxW > 0 && segW > maxW {
		segW = maxW
	}
	if segW < 1 {
		segW = 1
	}
	if stripH < 1 {
		stripH = 1
	}
	return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return ui.layoutDialogFlatActionSegment(th, gtx, click, label, hover, pulse, segW, stripH, disabled, state)
	})
}

func (ui *UI) layoutDialogActionPairState(th *material.Theme, gtx layout.Context, leftClick *widget.Clickable, leftLabel string, leftHover, leftPulse float32, leftDisabled bool, rightClick *widget.Clickable, rightLabel string, rightHover, rightPulse float32, rightDisabled bool, leftState, rightState dialogActionVisualState) layout.Dimensions {
	leftW, leftH := ui.dialogActionSegmentMetricsPx(th, gtx, leftLabel)
	rightW, rightH := ui.dialogActionSegmentMetricsPx(th, gtx, rightLabel)
	stripH := leftH
	if rightH > stripH {
		stripH = rightH
	}
	if stripH < 1 {
		stripH = 1
	}
	gap := dialogActionGapPx(gtx)
	maxW := gtx.Constraints.Max.X
	if maxW > 0 && leftW+gap+rightW > maxW {
		segW := (maxW - gap) / 2
		minSegW := gtx.Dp(unit.Dp(52))
		if segW < minSegW {
			segW = minSegW
		}
		leftW = segW
		rightW = segW
	}
	return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutDialogFlatActionSegment(th, gtx, leftClick, leftLabel, leftHover, leftPulse, leftW, stripH, leftDisabled, leftState)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutDialogFlatActionSegment(th, gtx, rightClick, rightLabel, rightHover, rightPulse, rightW, stripH, rightDisabled, rightState)
			}),
		)
	})
}

func (ui *UI) layoutDialogActionTriple(th *material.Theme, gtx layout.Context, leftClick *widget.Clickable, leftLabel string, leftHover, leftPulse float32, leftDisabled bool, middleClick *widget.Clickable, middleLabel string, middleHover, middlePulse float32, middleDisabled bool, rightClick *widget.Clickable, rightLabel string, rightHover, rightPulse float32, rightDisabled bool, leftState, middleState, rightState dialogActionVisualState) layout.Dimensions {
	return ui.layoutDialogActionTripleState(th, gtx, leftClick, leftLabel, leftHover, leftPulse, leftDisabled, middleClick, middleLabel, middleHover, middlePulse, middleDisabled, rightClick, rightLabel, rightHover, rightPulse, rightDisabled, leftState, middleState, rightState)
}

func (ui *UI) layoutDialogActionTripleState(th *material.Theme, gtx layout.Context, leftClick *widget.Clickable, leftLabel string, leftHover, leftPulse float32, leftDisabled bool, middleClick *widget.Clickable, middleLabel string, middleHover, middlePulse float32, middleDisabled bool, rightClick *widget.Clickable, rightLabel string, rightHover, rightPulse float32, rightDisabled bool, leftState, middleState, rightState dialogActionVisualState) layout.Dimensions {
	leftW, leftH := ui.dialogActionSegmentMetricsPx(th, gtx, leftLabel)
	middleW, middleH := ui.dialogActionSegmentMetricsPx(th, gtx, middleLabel)
	rightW, rightH := ui.dialogActionSegmentMetricsPx(th, gtx, rightLabel)

	stripH := leftH
	if middleH > stripH {
		stripH = middleH
	}
	if rightH > stripH {
		stripH = rightH
	}
	if stripH < 1 {
		stripH = 1
	}

	gap := dialogActionGapPx(gtx)
	maxW := gtx.Constraints.Max.X
	totalW := leftW + middleW + rightW + gap*2
	if maxW > 0 && totalW > maxW {
		segW := (maxW - gap*2) / 3
		minSegW := gtx.Dp(unit.Dp(52))
		if segW < minSegW {
			segW = minSegW
		}
		leftW = segW
		middleW = segW
		rightW = segW
	}

	return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutDialogFlatActionSegment(th, gtx, leftClick, leftLabel, leftHover, leftPulse, leftW, stripH, leftDisabled, leftState)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutDialogFlatActionSegment(th, gtx, middleClick, middleLabel, middleHover, middlePulse, middleW, stripH, middleDisabled, middleState)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutDialogFlatActionSegment(th, gtx, rightClick, rightLabel, rightHover, rightPulse, rightW, stripH, rightDisabled, rightState)
			}),
		)
	})
}

func dialogActionGapPx(gtx layout.Context) int {
	gap := gtx.Dp(unit.Dp(4))
	if gap < 1 {
		gap = 1
	}
	return gap
}

func layoutDialogHorizontalDivider(gtx layout.Context) layout.Dimensions {
	h := gtx.Dp(unit.Dp(1))
	if h < 1 {
		h = 1
	}
	w := gtx.Constraints.Max.X
	if w < 1 {
		w = 1
	}
	paint.FillShape(gtx.Ops, dialogDividerColor, clip.Rect(image.Rect(0, 0, w, h)).Op())
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func layoutDialogVerticalDivider(gtx layout.Context) layout.Dimensions {
	w := gtx.Dp(unit.Dp(1))
	if w < 1 {
		w = 1
	}
	h := gtx.Constraints.Max.Y
	if h < 1 {
		h = 1
	}
	paint.FillShape(gtx.Ops, dialogDividerColor, clip.Rect(image.Rect(0, 0, w, h)).Op())
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func (ui *UI) layoutDialogFlatActionSegment(th *material.Theme, gtx layout.Context, click *widget.Clickable, label string, hoverFill, pulseFill float32, segW, stripH int, disabled bool, state dialogActionVisualState) layout.Dimensions {
	if click == nil {
		return layout.Dimensions{}
	}
	hoverFill = clamp01(hoverFill)
	pulseFill = clamp01(pulseFill)
	focusFill := float32(0)
	defaultFill := float32(0)
	if state.Focused {
		focusFill = 1
	}
	if state.Default {
		defaultFill = 1
	}
	if disabled {
		hoverFill = 0
		pulseFill = 0
		if focusFill == 0 {
			defaultFill = 0
		}
	}

	dims := fixedWidth(gtx, segW, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if click.Pressed() && !disabled && pulseFill < 0.5 {
					pulseFill = 0.5
				}

				bg := color.NRGBA{}
				bg = mixNRGBA(bg, color.NRGBA{R: 34, G: 34, B: 34, A: 190}, hoverFill)
				bg = mixNRGBA(bg, color.NRGBA{R: 48, G: 48, B: 48, A: 210}, pulseFill*0.35)
				bg = mixNRGBA(bg, color.NRGBA{R: 60, G: 54, B: 44, A: 210}, defaultFill*0.5)
				bg = mixNRGBA(bg, color.NRGBA{R: 72, G: 66, B: 54, A: 230}, focusFill*0.42)

				fg := mixNRGBA(txtColor, color.NRGBA{R: 236, G: 236, B: 236, A: 255}, hoverFill*0.75)
				fg = mixNRGBA(fg, color.NRGBA{R: 248, G: 248, B: 248, A: 255}, pulseFill*0.25)
				fg = mixNRGBA(fg, color.NRGBA{R: 244, G: 234, B: 206, A: 255}, defaultFill*0.32)
				fg = mixNRGBA(fg, color.NRGBA{R: 250, G: 246, B: 236, A: 255}, focusFill*0.3)

				line := color.NRGBA{}
				lineH := 1
				if hoverFill > 0 {
					line = mixNRGBA(line, color.NRGBA{R: 255, G: 255, B: 255, A: 70}, hoverFill)
				}
				if defaultFill > 0 {
					line = mixNRGBA(line, color.NRGBA{R: 200, G: 182, B: 144, A: 168}, defaultFill)
				}
				if focusFill > 0 {
					line = color.NRGBA{R: 214, G: 198, B: 166, A: 225}
					lineH = 2
				}
				if disabled {
					fg = color.NRGBA{R: 160, G: 166, B: 180, A: 255}
					bg = color.NRGBA{}
				}

				dims := fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body2(th, label)
							lbl.Font.Typeface = ui.interfaceTypeface()
							lbl.Font.Weight = font.Medium
							lbl.TextSize = ui.scaleDialogFontSize(10)
							lbl.Color = fg
							lbl.MaxLines = 1
							return lbl.Layout(gtx)
						})
					})
				})
				if dims.Size.X > 0 && dims.Size.Y >= lineH && line.A != 0 {
					paint.FillShape(gtx.Ops, line, clip.Rect(image.Rect(0, dims.Size.Y-lineH, dims.Size.X, dims.Size.Y)).Op())
				}
				return dims
			})
		})
	})
	if dims.Size.X > 0 && dims.Size.Y > 0 && !disabled {
		defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	}
	return dims
}

func (ui *UI) layoutFileCopyTextBar(th *material.Theme, gtx layout.Context, frac float32, indeterminate bool) layout.Dimensions {
	barText := func(width int) string {
		cells := textCellProgressBar(frac, width)
		if indeterminate {
			cells = textCellActivityBar(gtx.Now, width)
		}
		return "[" + cells + "]"
	}
	barFits := func(width int) bool {
		probe := material.Body2(th, barText(width))
		probe.Font.Typeface = ui.interfaceTypeface()
		probe.TextSize = ui.fileOpDialogTextSize()
		probe.MaxLines = 1
		return measureLabelUnconstrained(gtx, probe).Size.X <= gtx.Constraints.Max.X
	}
	// Measure shaped runs rather than multiplying a single glyph width. Some
	// configured fonts compact adjacent block cells, which otherwise leaves a
	// conspicuous unused strip at the right edge.
	barWidth := 8
	low, high := 8, 160
	for low <= high {
		middle := low + (high-low)/2
		if barFits(middle) {
			barWidth = middle
			low = middle + 1
		} else {
			high = middle - 1
		}
	}
	lbl := material.Body2(th, barText(barWidth))
	lbl.Font.Typeface = ui.interfaceTypeface()
	lbl.TextSize = ui.fileOpDialogTextSize()
	lbl.Color = color.NRGBA{R: 190, G: 190, B: 190, A: 255}
	lbl.MaxLines = 1
	return lbl.Layout(gtx)
}

func copyProgressFraction(progress filesys.CopyProgress) float32 {
	if progress.Streaming {
		if progress.CurrentBytesTotal > 0 {
			return float32(progress.CurrentBytesDone) / float32(progress.CurrentBytesTotal)
		}
		// Server-side same-host copies cannot expose byte progress, so use the
		// selected-file counter as a compact fallback.
		if progress.ScanDone && progress.FilesDiscovered > 0 {
			return float32(progress.FilesCopied) / float32(progress.FilesDiscovered)
		}
		return 0
	}
	if progress.BytesTotal > 0 {
		return float32(progress.BytesDone) / float32(progress.BytesTotal)
	}
	if progress.EntriesTotal > 0 {
		return float32(progress.EntriesDone) / float32(progress.EntriesTotal)
	}
	return 0
}

const fileCopyMeaningfulProgressMinBytes = 1 << 20

func copyProgressIndeterminate(progress filesys.CopyProgress) bool {
	return progress.Streaming && progress.CurrentBytesTotal < fileCopyMeaningfulProgressMinBytes
}

func buildFileCopyProgressDisplay(progress filesys.CopyProgress, speed int64) fileCopyProgressDisplay {
	display := fileCopyProgressDisplay{
		Indeterminate: copyProgressIndeterminate(progress),
		PrimaryLabel:  "Processed",
	}
	if !progress.Streaming {
		display.PrimaryValue = copyProgressText(progress, speed)
		return display
	}

	if progress.CurrentBytesTotal >= fileCopyMeaningfulProgressMinBytes {
		percent := 0
		if progress.CurrentBytesTotal > 0 {
			percent = int(float64(progress.CurrentBytesDone) * 100 / float64(progress.CurrentBytesTotal))
		}
		display.PrimaryValue = fmt.Sprintf(
			"%s / %s (%d%%)",
			formatCopySize(progress.CurrentBytesDone),
			formatCopySize(progress.CurrentBytesTotal),
			percent,
		)
		if speed > 0 {
			display.PrimaryValue += " @ " + formatCopySize(speed) + "/s"
		}
		display.SecondaryLabel = "Remaining"
		display.SecondaryValue = formatCopyRemaining(progress.CurrentBytesTotal-progress.CurrentBytesDone, speed)
		return display
	}

	display.PrimaryValue = fileOpCountText(progress.FilesCopied, "file", "files")
	if progress.BytesDone > 0 {
		display.PrimaryValue += " (" + formatCopySize(progress.BytesDone) + ")"
	}
	if speed > 0 {
		display.PrimaryValue += " @ " + formatCopySize(speed) + "/s"
	}
	display.SecondaryLabel = "Discovered"
	if progress.FilesDiscovered <= 0 {
		display.SecondaryValue = "Scanning..."
	} else {
		display.SecondaryValue = fileOpCountText(progress.FilesDiscovered, "file", "files")
		if !progress.ScanDone {
			display.SecondaryValue += "..."
		}
	}
	return display
}

func formatCopyRemaining(remainingBytes, speed int64) string {
	if speed <= 0 {
		return "Calculating..."
	}
	if remainingBytes < 0 {
		remainingBytes = 0
	}
	seconds := (remainingBytes + speed - 1) / speed
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	seconds %= 60
	return fmt.Sprintf("~%02d:%02d:%02d", hours, minutes, seconds)
}

func copyProgressCountText(progress filesys.CopyProgress) string {
	if !progress.Streaming {
		return ""
	}
	if progress.FilesDiscovered <= 0 {
		return "Scanning..."
	}
	return fmt.Sprintf("%d copied  •  %d discovered", progress.FilesCopied, progress.FilesDiscovered)
}

func copyProgressTransferText(progress filesys.CopyProgress, speed int64) string {
	if !progress.Streaming {
		return ""
	}
	text := ""
	if progress.CurrentBytesTotal > 0 {
		text = formatCopySize(progress.CurrentBytesDone) + " / " + formatCopySize(progress.CurrentBytesTotal)
	} else if progress.BytesDone > 0 {
		text = formatCopySize(progress.BytesDone) + " copied"
	}
	if speed > 0 {
		if text != "" {
			text += "  •  "
		}
		text += formatCopySize(speed) + "/s"
	}
	return text
}

func copyProgressText(progress filesys.CopyProgress, speed int64) string {
	if progress.Streaming {
		return copyProgressCountText(progress)
	}
	if progress.EntriesTotal <= 0 && progress.BytesTotal <= 0 {
		if progress.EntriesDone > 0 {
			return fmt.Sprintf("Preparing... %d entries found", progress.EntriesDone)
		}
		return "Preparing..."
	}
	speedText := ""
	if speed > 0 {
		speedText = "  •  " + formatCopySize(speed) + "/s"
	}
	if progress.BytesTotal > 0 {
		return fmt.Sprintf(
			"%d/%d entries  •  %s / %s%s",
			progress.EntriesDone,
			progress.EntriesTotal,
			formatCopySize(progress.BytesDone),
			formatCopySize(progress.BytesTotal),
			speedText,
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

func directFilePasteStatusLineForWidth(st *fileCopyState, now time.Time, maxWidth int, measure func(string) int) string {
	parts := directFilePasteStatusPartsFor(st, now)
	if parts.filename == "" {
		return ""
	}
	details := append([]string(nil), parts.details...)
	name := parts.filename
	if measure == nil || maxWidth <= 0 {
		return directFilePasteBuildStatusLine(name, details)
	}
	for {
		line := directFilePasteBuildStatusLine(name, details)
		if measure(line) <= maxWidth {
			return line
		}
		nameMax := maxWidth - measure(directFilePasteBuildStatusLine("", details))
		name = archiveExtractTrimMiddleToWidth(parts.filename, nameMax, measure)
		line = directFilePasteBuildStatusLine(name, details)
		if measure(line) <= maxWidth || len(details) <= 1 {
			return line
		}
		details = details[:len(details)-1]
		name = parts.filename
	}
}

func directFilePasteStatusLineWithSeparatorForWidth(st *fileCopyState, now time.Time, maxWidth int, measure func(string) int, trailing bool) string {
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
	line := directFilePasteStatusLineForWidth(st, now, lineMax, measure)
	if strings.TrimSpace(line) == "" {
		return ""
	}
	if trailing {
		return line + separator
	}
	return separator + line
}

func directFilePasteStatusPartsFor(st *fileCopyState, now time.Time) archiveExtractStatusParts {
	if st == nil || !st.directPaste {
		return archiveExtractStatusParts{}
	}
	progress := st.progress
	name := strings.TrimSpace(copyProgressCurrent(progress))
	if name == "" {
		if count := st.sourceCount(); count > 1 {
			name = fmt.Sprintf("%d files", count)
		} else {
			name = filepath.Base(filepath.Clean(st.srcPath))
		}
	}
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "files"
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
		parts.details = append(parts.details, fmt.Sprintf("%s %d%%", archiveExtractStatusBar(frac), int(frac*100+0.5)))
	} else {
		parts.details = append(parts.details, "preparing")
	}

	speed := st.speedBytes
	if speed <= 0 {
		speed = archiveExtractSpeed(progress, st.startedAt, now)
	}
	if speed > 0 {
		parts.details = append(parts.details, formatCopySize(speed)+"/s")
		if eta := archiveExtractETA(progress, speed); eta > 0 {
			parts.details = append(parts.details, formatArchiveExtractETA(eta)+" left")
		}
	}
	return parts
}

func directFilePasteBuildStatusLine(filename string, details []string) string {
	line := "[Pasting] " + filename
	if len(details) > 0 {
		line += " | " + strings.Join(details, " | ")
	}
	return line
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

func inspectCopyDestinationDir(dstEp copyEndpoint, dstRaw string) (string, fileCopyPathInfo, error) {
	dstNorm, err := dstEp.normalizePath(dstRaw)
	if err != nil {
		return "", fileCopyPathInfo{}, err
	}
	info := fileCopyPathInfo{Path: dstNorm}
	if dstStat, err := endpointStat(dstEp, dstNorm); err == nil && dstStat != nil {
		info.Exists = true
		info.IsDir = dstStat.IsDir()
		info.ModTime = dstStat.ModTime()
		if dstStat.Mode().IsRegular() {
			info.Size = dstStat.Size()
		}
		if !dstStat.IsDir() {
			return "", info, fmt.Errorf("destination must be a directory when copying multiple items")
		}
	}
	return dstNorm, info, nil
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if os.PathSeparator == '\\' {
		return strings.EqualFold(a, b)
	}
	return a == b
}
