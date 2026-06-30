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
	"regexp"
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

const multiRenameSuccessNoticeDur = 1800 * time.Millisecond

type multiRenameCaseMode uint8

const (
	multiRenameCaseKeep multiRenameCaseMode = iota
	multiRenameCaseUpper
	multiRenameCaseLower
)

type multiRenameScope uint8

const (
	multiRenameScopeName multiRenameScope = iota
	multiRenameScopeExtension
	multiRenameScopeBoth
)

type multiRenameTarget struct {
	path    string
	oldName string
	newName string
	kind    filesys.EntryKind
}

type multiRenameState struct {
	pane     int
	row      int
	endpoint copyEndpoint
	targets  []multiRenameTarget

	searchEdit  widget.Editor
	replaceEdit widget.Editor
	prefixEdit  widget.Editor
	suffixEdit  widget.Editor
	startEdit   widget.Editor
	stepEdit    widget.Editor
	digitsEdit  widget.Editor

	caseSensitive    widget.Bool
	sequence         widget.Bool
	sequenceAtEnd    bool
	caseMode         multiRenameCaseMode
	scope            multiRenameScope
	caseClicks       [3]widget.Clickable
	scopeClicks      [3]widget.Clickable
	positionClicks   [2]widget.Clickable
	caseTabsAnim     segmentedAnimState
	scopeTabsAnim    segmentedAnimState
	positionTabsAnim segmentedAnimState

	previewList widget.List

	backdropClick widget.Clickable
	closeClick    widget.Clickable
	cancelClick   widget.Clickable
	renameClick   widget.Clickable

	actionsAnim  segmentedAnimState
	running      bool
	lastErr      string
	operationErr string
	lastInput    string
	doneCh       chan multiRenameResult
	focusWant    bool
	keyFocus     dialogKeyboardFocusState
	focus        multiRenameFocus
	actionFocus  multiRenameAction
}

type multiRenamePlanItem struct {
	src  string
	dst  string
	temp string
}

type multiRenameResult struct {
	renamed int
	err     error
}

type multiRenameFocus uint8

const (
	multiRenameFocusNone multiRenameFocus = iota
	multiRenameFocusFind
	multiRenameFocusReplace
	multiRenameFocusPrefix
	multiRenameFocusSuffix
	multiRenameFocusScope
	multiRenameFocusCase
	multiRenameFocusCaseSensitive
	multiRenameFocusCounter
	multiRenameFocusCounterStart
	multiRenameFocusCounterStep
	multiRenameFocusCounterDigits
	multiRenameFocusCounterPosition
	multiRenameFocusActions
)

type multiRenameAction uint8

const (
	multiRenameActionCancel multiRenameAction = iota
	multiRenameActionRename
)

func (st *multiRenameState) focusOrder() []multiRenameFocus {
	if st == nil {
		return nil
	}
	order := []multiRenameFocus{
		multiRenameFocusFind,
		multiRenameFocusReplace,
		multiRenameFocusPrefix,
		multiRenameFocusSuffix,
		multiRenameFocusScope,
		multiRenameFocusCase,
		multiRenameFocusCaseSensitive,
		multiRenameFocusCounter,
	}
	if st.sequence.Value {
		order = append(order,
			multiRenameFocusCounterStart,
			multiRenameFocusCounterStep,
			multiRenameFocusCounterDigits,
			multiRenameFocusCounterPosition,
		)
	}
	return append(order, multiRenameFocusActions)
}

func (st *multiRenameState) editorForFocus(target multiRenameFocus) *widget.Editor {
	if st == nil {
		return nil
	}
	switch target {
	case multiRenameFocusFind:
		return &st.searchEdit
	case multiRenameFocusReplace:
		return &st.replaceEdit
	case multiRenameFocusPrefix:
		return &st.prefixEdit
	case multiRenameFocusSuffix:
		return &st.suffixEdit
	case multiRenameFocusCounterStart:
		return &st.startEdit
	case multiRenameFocusCounterStep:
		return &st.stepEdit
	case multiRenameFocusCounterDigits:
		return &st.digitsEdit
	default:
		return nil
	}
}

func (st *multiRenameState) canFocus(target multiRenameFocus) bool {
	if st == nil {
		return false
	}
	switch target {
	case multiRenameFocusCounterStart, multiRenameFocusCounterStep, multiRenameFocusCounterDigits, multiRenameFocusCounterPosition:
		return st.sequence.Value
	case multiRenameFocusFind, multiRenameFocusReplace, multiRenameFocusPrefix, multiRenameFocusSuffix,
		multiRenameFocusScope, multiRenameFocusCase, multiRenameFocusCaseSensitive,
		multiRenameFocusCounter, multiRenameFocusActions:
		return true
	default:
		return false
	}
}

func (st *multiRenameState) syncFocus(gtx layout.Context) {
	if st == nil || st.running {
		return
	}
	switch {
	case gtx.Focused(&st.searchEdit):
		st.focus = multiRenameFocusFind
	case gtx.Focused(&st.replaceEdit):
		st.focus = multiRenameFocusReplace
	case gtx.Focused(&st.prefixEdit):
		st.focus = multiRenameFocusPrefix
	case gtx.Focused(&st.suffixEdit):
		st.focus = multiRenameFocusSuffix
	case gtx.Focused(&st.startEdit):
		st.focus = multiRenameFocusCounterStart
	case gtx.Focused(&st.stepEdit):
		st.focus = multiRenameFocusCounterStep
	case gtx.Focused(&st.digitsEdit):
		st.focus = multiRenameFocusCounterDigits
	case gtx.Focused(&st.caseSensitive):
		st.focus = multiRenameFocusCaseSensitive
	case gtx.Focused(&st.sequence):
		st.focus = multiRenameFocusCounter
	}
}

func (st *multiRenameState) setFocus(gtx layout.Context, target multiRenameFocus) bool {
	if st == nil || !st.canFocus(target) {
		return false
	}
	changed := st.focus != target
	st.focus = target
	if target == multiRenameFocusActions {
		st.actionFocus = multiRenameActionRename
	}
	if ed := st.editorForFocus(target); ed != nil {
		gtx.Execute(key.FocusCmd{Tag: ed})
		return changed
	}
	switch target {
	case multiRenameFocusCaseSensitive:
		gtx.Execute(key.FocusCmd{Tag: &st.caseSensitive})
	case multiRenameFocusCounter:
		gtx.Execute(key.FocusCmd{Tag: &st.sequence})
	default:
		gtx.Execute(key.FocusCmd{Tag: &st.keyFocus.tag})
	}
	return changed
}

func (st *multiRenameState) stepFocus(gtx layout.Context, step int) bool {
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
	return st.setFocus(gtx, order[dialogWrappedIndex(current, len(order), step)])
}

func (st *multiRenameState) stepAction(step int) bool {
	if st == nil {
		return false
	}
	next := multiRenameActionRename
	if st.actionFocus == multiRenameActionRename {
		next = multiRenameActionCancel
	}
	if step == 0 || next == st.actionFocus {
		return false
	}
	st.actionFocus = next
	return true
}

func (st *multiRenameState) actionVisualState(target multiRenameAction) dialogActionVisualState {
	if st == nil || st.running {
		return dialogActionVisualState{}
	}
	if st.focus == multiRenameFocusActions {
		active := st.actionFocus == target
		return dialogActionVisualState{Focused: active, Default: active}
	}
	return dialogActionVisualState{Default: target == multiRenameActionRename}
}

func (ui *UI) startMultiRename(idx int, now time.Time) bool {
	if ui == nil || idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.model == nil || pane.table == nil {
		return false
	}
	if pane.archiveBrowsing() {
		pane.setNotice("cannot rename files inside an archive", now)
		return false
	}
	selected := pane.selectedEntriesForAction()
	if len(selected) == 0 {
		pane.setNotice("select one or more files to rename", now)
		return false
	}

	ep := copyEndpointFromPane(idx, pane)
	if pane.remoteConnected() {
		ep.remote = pane.remote.clone()
		if ep.remote == nil {
			pane.setNotice("sftp session is not connected", now)
			return false
		}
	}
	targets := make([]multiRenameTarget, 0, len(selected))
	for _, entry := range selected {
		name := entry.Name
		if name == "" {
			name = entry.DisplayName
		}
		targets = append(targets, multiRenameTarget{
			path: entry.Path, oldName: name, newName: name, kind: entry.Kind,
		})
	}
	st := &multiRenameState{
		pane: idx, row: pane.table.Selected, endpoint: ep, targets: targets,
		caseMode: multiRenameCaseKeep, focusWant: true,
		focus: multiRenameFocusFind, actionFocus: multiRenameActionRename,
	}
	for _, ed := range []*widget.Editor{&st.searchEdit, &st.replaceEdit, &st.prefixEdit, &st.suffixEdit, &st.startEdit, &st.stepEdit, &st.digitsEdit} {
		ed.SingleLine = true
		ed.Submit = true
	}
	st.startEdit.SetText("1")
	st.stepEdit.SetText("1")
	st.digitsEdit.SetText("2")
	st.previewList.Axis = layout.Vertical
	ui.multiRename = st
	ui.setActiveFilePane(idx)
	ui.closeFunctionBarPopups()
	pane.stopPathEdit()
	pane.closeSortMenu()
	pane.closeFavoriteMenu()
	pane.closeDriveMenu()
	pane.closeContextMenu()
	ui.rep.active = false
	ui.rep.pane = -1
	st.refreshPreview()
	return true
}

func (ui *UI) closeMultiRename() {
	if ui == nil || ui.multiRename == nil {
		return
	}
	if ui.multiRename.endpoint.remote != nil {
		ui.multiRename.endpoint.remote.close()
	}
	ui.multiRename = nil
	ui.closeEditorContextMenu()
}

func (st *multiRenameState) hasDirectoryTarget() bool {
	if st == nil {
		return false
	}
	for _, target := range st.targets {
		if target.kind == filesys.EntryDir {
			return true
		}
	}
	return false
}

// effectiveScope is the scope actually applied. Directories have no extension,
// so any directory in the selection (including mixed file+dir selections) locks
// the operation to the name portion only.
func (st *multiRenameState) effectiveScope() multiRenameScope {
	if st == nil {
		return multiRenameScopeName
	}
	if st.hasDirectoryTarget() {
		return multiRenameScopeName
	}
	return st.scope
}

func splitMultiRenameName(name string, kind filesys.EntryKind) (string, string) {
	if kind == filesys.EntryDir {
		return name, ""
	}
	i := strings.LastIndex(name, ".")
	if i <= 0 {
		return name, ""
	}
	return name[:i], name[i+1:]
}

func multiRenameReplace(text, search, replacement string, caseSensitive bool) string {
	if search == "" {
		return text
	}
	if caseSensitive {
		return strings.ReplaceAll(text, search, replacement)
	}
	re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(search))
	if err != nil {
		return text
	}
	return re.ReplaceAllStringFunc(text, func(string) string { return replacement })
}

func multiRenameApply(oldName string, kind filesys.EntryKind, search, replacement, prefix, suffix string, caseSensitive bool, scope multiRenameScope, sequence, sequenceAtEnd bool, sequenceValue, sequenceWidth int, caseMode multiRenameCaseMode) string {
	namePart, extensionPart := splitMultiRenameName(oldName, kind)
	if scope == multiRenameScopeExtension && kind == filesys.EntryDir {
		return oldName
	}
	target := namePart
	switch scope {
	case multiRenameScopeExtension:
		target = extensionPart
	case multiRenameScopeBoth:
		target = oldName
	}
	target = multiRenameReplace(target, search, replacement, caseSensitive)
	target = prefix + target + suffix
	if sequence {
		counter := fmt.Sprintf("%0*d", sequenceWidth, sequenceValue)
		if sequenceAtEnd {
			target += counter
		} else {
			target = counter + target
		}
	}
	switch caseMode {
	case multiRenameCaseLower:
		target = strings.ToLower(target)
	case multiRenameCaseUpper:
		target = strings.ToUpper(target)
	}
	switch scope {
	case multiRenameScopeExtension:
		if target == "" {
			return namePart
		}
		return namePart + "." + target
	case multiRenameScopeBoth:
		return target
	default:
		if extensionPart == "" {
			return target
		}
		return target + "." + extensionPart
	}
}

func multiRenameValidName(name string, remote bool) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("a resulting name is empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("%q is not a valid name", name)
	}
	if strings.ContainsRune(name, 0) {
		return errors.New("a resulting name contains an invalid character")
	}
	if remote {
		if strings.Contains(name, "/") {
			return fmt.Errorf("%q contains '/'", name)
		}
		return nil
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%q contains a path separator", name)
	}
	if runtime.GOOS == "windows" && strings.ContainsAny(name, `<>:"|?*`) {
		return fmt.Errorf("%q contains a character Windows does not allow", name)
	}
	if runtime.GOOS == "windows" {
		for _, r := range name {
			if r < 32 {
				return fmt.Errorf("%q contains a control character", name)
			}
		}
		if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
			return fmt.Errorf("%q cannot end with a dot or space on Windows", name)
		}
		base := strings.ToUpper(strings.SplitN(strings.TrimRight(name, " ."), ".", 2)[0])
		reserved := base == "CON" || base == "PRN" || base == "AUX" || base == "NUL"
		if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
			reserved = true
		}
		if reserved {
			return fmt.Errorf("%q is a reserved Windows name", name)
		}
	}
	return nil
}

func (st *multiRenameState) refreshPreview() {
	if st == nil {
		return
	}
	scope := st.effectiveScope()
	signature := fmt.Sprintf("%q\x00%q\x00%q\x00%q\x00%q\x00%q\x00%q\x00%t\x00%d\x00%t\x00%t\x00%d",
		st.searchEdit.Text(), st.replaceEdit.Text(), st.prefixEdit.Text(), st.suffixEdit.Text(), st.startEdit.Text(), st.stepEdit.Text(), st.digitsEdit.Text(),
		st.caseSensitive.Value, scope, st.sequence.Value, st.sequenceAtEnd, st.caseMode)
	if signature != st.lastInput {
		st.operationErr = ""
		st.lastInput = signature
	}
	st.lastErr = ""
	start := 1
	step := 1
	digits := 2
	if st.sequence.Value {
		var err error
		start, err = strconv.Atoi(strings.TrimSpace(st.startEdit.Text()))
		if err != nil || start < 0 {
			st.lastErr = "Sequence start must be a non-negative number"
		}
		step, err = strconv.Atoi(strings.TrimSpace(st.stepEdit.Text()))
		if st.lastErr == "" && (err != nil || step < 1) {
			st.lastErr = "Counter step must be a positive number"
		}
		digits, err = strconv.Atoi(strings.TrimSpace(st.digitsEdit.Text()))
		if st.lastErr == "" && (err != nil || digits < 1 || digits > 9) {
			st.lastErr = "Counter digits must be between 1 and 9"
		}
		maxInt := int(^uint(0) >> 1)
		increments := len(st.targets) - 1
		if st.lastErr == "" && increments > 0 && step > (maxInt-start)/increments {
			st.lastErr = "Counter range is too large"
		}
	}
	for i := range st.targets {
		st.targets[i].newName = multiRenameApply(
			st.targets[i].oldName, st.targets[i].kind,
			st.searchEdit.Text(), st.replaceEdit.Text(), st.prefixEdit.Text(), st.suffixEdit.Text(),
			st.caseSensitive.Value, scope, st.sequence.Value, st.sequenceAtEnd, start+i*step, digits, st.caseMode,
		)
		if st.lastErr == "" {
			if err := multiRenameValidName(st.targets[i].newName, st.endpoint.isRemote()); err != nil {
				st.lastErr = err.Error()
			}
		}
	}
	if st.lastErr == "" {
		for i := range st.targets {
			for j := 0; j < i; j++ {
				if multiRenameNamesEqual(st.endpoint, st.targets[i].newName, st.targets[j].newName) {
					st.lastErr = fmt.Sprintf("More than one item would be named %q", st.targets[i].newName)
					return
				}
			}
		}
	}
}

func multiRenameNamesEqual(ep copyEndpoint, a, b string) bool {
	if ep.isRemote() || os.PathSeparator != '\\' {
		return a == b
	}
	return strings.EqualFold(a, b)
}

func (st *multiRenameState) changedCount() int {
	if st == nil || st.lastErr != "" {
		return 0
	}
	n := 0
	for _, target := range st.targets {
		if target.newName != target.oldName {
			n++
		}
	}
	return n
}

func (st *multiRenameState) buildPlan() ([]multiRenamePlanItem, error) {
	st.refreshPreview()
	if st.lastErr != "" {
		return nil, errors.New(st.lastErr)
	}
	plan := make([]multiRenamePlanItem, 0, len(st.targets))
	sourcePaths := make([]string, 0, len(st.targets))
	for _, target := range st.targets {
		src, err := st.endpoint.normalizeSourcePath(target.path)
		if err != nil {
			return nil, err
		}
		sourcePaths = append(sourcePaths, src)
	}
	for i, target := range st.targets {
		if target.newName == target.oldName {
			continue
		}
		src := sourcePaths[i]
		dst := st.endpoint.join(st.endpoint.dirName(src), target.newName)
		occupiedBySelection := false
		for _, selectedPath := range sourcePaths {
			if st.endpoint.samePath(dst, selectedPath) {
				occupiedBySelection = true
				break
			}
		}
		if !occupiedBySelection {
			if _, err := endpointLstat(st.endpoint, dst); err == nil {
				return nil, fmt.Errorf("%q already exists", target.newName)
			} else if !os.IsNotExist(err) {
				return nil, err
			}
		}
		plan = append(plan, multiRenamePlanItem{src: src, dst: dst})
	}
	if len(plan) == 0 {
		return nil, errors.New("the filenames would not change")
	}
	return plan, nil
}

func endpointRename(ep copyEndpoint, oldPath, newPath string) error {
	if ep.isRemote() {
		client := ep.remote.sftpClient()
		if client == nil {
			return errors.New("sftp session is not connected")
		}
		return client.Rename(oldPath, newPath)
	}
	return os.Rename(oldPath, newPath)
}

func multiRenameTempPath(ep copyEndpoint, src string, index int) (string, error) {
	dir := ep.dirName(src)
	stamp := time.Now().UnixNano()
	for attempt := 0; attempt < 100; attempt++ {
		name := fmt.Sprintf(".hexone-rename-%x-%d-%d", stamp, index, attempt)
		candidate := ep.join(dir, name)
		if _, err := endpointLstat(ep, candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", errors.New("could not reserve temporary rename paths")
}

func executeMultiRename(ep copyEndpoint, plan []multiRenamePlanItem) multiRenameResult {
	for i := range plan {
		temp, err := multiRenameTempPath(ep, plan[i].src, i)
		if err != nil {
			return multiRenameResult{err: err}
		}
		plan[i].temp = temp
	}
	staged := 0
	for i := range plan {
		if err := endpointRename(ep, plan[i].src, plan[i].temp); err != nil {
			for j := staged - 1; j >= 0; j-- {
				_ = endpointRename(ep, plan[j].temp, plan[j].src)
			}
			return multiRenameResult{err: fmt.Errorf("rename %q: %w", ep.baseName(plan[i].src), err)}
		}
		staged++
	}
	completed := 0
	for i := range plan {
		if err := endpointRename(ep, plan[i].temp, plan[i].dst); err != nil {
			rollbackErr := error(nil)
			for j := completed - 1; j >= 0; j-- {
				if moveErr := endpointRename(ep, plan[j].dst, plan[j].temp); moveErr != nil && rollbackErr == nil {
					rollbackErr = moveErr
				}
			}
			for j := len(plan) - 1; j >= 0; j-- {
				if moveErr := endpointRename(ep, plan[j].temp, plan[j].src); moveErr != nil && rollbackErr == nil {
					rollbackErr = moveErr
				}
			}
			if rollbackErr != nil {
				return multiRenameResult{err: fmt.Errorf("rename failed: %v (rollback also failed: %v)", err, rollbackErr)}
			}
			return multiRenameResult{err: fmt.Errorf("rename %q: %w", ep.baseName(plan[i].src), err)}
		}
		completed++
	}
	return multiRenameResult{renamed: len(plan)}
}

func (ui *UI) submitMultiRename(now time.Time) {
	st := ui.multiRename
	if st == nil || st.running {
		return
	}
	plan, err := st.buildPlan()
	if err != nil {
		st.operationErr = err.Error()
		return
	}
	st.running = true
	st.lastErr = ""
	st.operationErr = ""
	st.doneCh = make(chan multiRenameResult, 1)
	done := st.doneCh
	ep := st.endpoint
	go func() { done <- executeMultiRename(ep, plan) }()
	_ = now
}

func (ui *UI) pollMultiRename(now time.Time) {
	st := ui.multiRename
	if st == nil || !st.running || st.doneCh == nil {
		return
	}
	select {
	case result := <-st.doneCh:
		st.running = false
		st.doneCh = nil
		if result.err != nil {
			st.operationErr = result.err.Error()
			return
		}
		paneIdx, row := st.pane, st.row
		primary := ""
		for _, target := range st.targets {
			if target.newName != target.oldName {
				primary = st.endpoint.join(st.endpoint.dirName(target.path), target.newName)
				break
			}
		}
		dir := st.endpoint.dir
		message := fmt.Sprintf("renamed %d item", result.renamed)
		if result.renamed != 1 {
			message += "s"
		}
		ui.closeMultiRename()
		ui.requestPaneLoadWithSelectionAndScroll(paneIdx, dir, primary, "", row, layout.Position{}, false, "", message, multiRenameSuccessNoticeDur)
		gt := ui.filePanes[paneIdx]
		if gt != nil {
			gt.clearMarkedRows()
		}
		if ui.invalidate != nil {
			ui.invalidate()
		}
	default:
	}
}

func (st *multiRenameState) stepChoice(focus multiRenameFocus, step int, now time.Time) bool {
	if st == nil || step == 0 {
		return false
	}
	switch focus {
	case multiRenameFocusScope:
		if st.hasDirectoryTarget() {
			return false
		}
		current := int(st.scope)
		next := dialogWrappedIndex(current, len(st.scopeClicks), step)
		if next == current {
			return false
		}
		st.scope = multiRenameScope(next)
		st.scopeTabsAnim.setPulse(fmt.Sprintf("scope:%d", next), now)
	case multiRenameFocusCase:
		current := int(st.caseMode)
		next := dialogWrappedIndex(current, len(st.caseClicks), step)
		if next == current {
			return false
		}
		st.caseMode = multiRenameCaseMode(next)
		st.caseTabsAnim.setPulse(fmt.Sprintf("case:%d", next), now)
	case multiRenameFocusCounterPosition:
		current := 0
		if st.sequenceAtEnd {
			current = 1
		}
		next := dialogWrappedIndex(current, len(st.positionClicks), step)
		if next == current {
			return false
		}
		st.sequenceAtEnd = next == 1
		st.positionTabsAnim.setPulse(fmt.Sprintf("position:%d", next), now)
	default:
		return false
	}
	st.refreshPreview()
	return true
}

func (ui *UI) activateMultiRenameAction(gtx layout.Context, st *multiRenameState, action multiRenameAction) bool {
	if ui == nil || st == nil || st.running {
		return false
	}
	switch action {
	case multiRenameActionCancel:
		st.actionsAnim.setPulse("cancel", gtx.Now)
		ui.closeMultiRename()
		return true
	case multiRenameActionRename:
		if st.changedCount() == 0 {
			return false
		}
		st.actionsAnim.setPulse("rename", gtx.Now)
		ui.submitMultiRename(gtx.Now)
	}
	return false
}

func (ui *UI) handleMultiRenamePreLayoutInput(gtx layout.Context) {
	st := ui.multiRename
	if ui == nil || st == nil {
		return
	}
	ui.pollMultiRename(gtx.Now)
	st = ui.multiRename
	if st == nil {
		return
	}
	st.keyFocus.attach(gtx)
	st.syncFocus(gtx)
	anyMods := ^key.Modifiers(0)
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape, Optional: anyMods},
			key.Filter{Name: key.NameTab, Optional: anyMods},
		)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press || st.running {
			continue
		}
		switch ke.Name {
		case key.NameEscape:
			if ke.Modifiers == 0 {
				ui.closeMultiRename()
				return
			}
		case key.NameTab:
			step, ok := dialogTabStep(ke.Modifiers)
			if ok && st.stepFocus(gtx, step) {
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
	for {
		ev, ok := gtx.Event(
			key.FocusFilter{Target: &st.keyFocus.tag},
			key.Filter{Focus: &st.keyFocus.tag, Name: key.NameLeftArrow},
			key.Filter{Focus: &st.keyFocus.tag, Name: key.NameRightArrow},
			key.Filter{Focus: &st.keyFocus.tag, Name: key.NameEnter},
			key.Filter{Focus: &st.keyFocus.tag, Name: key.NameReturn},
			key.Filter{Focus: &st.keyFocus.tag, Name: key.NameSpace},
		)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press || ke.Modifiers != 0 || st.running {
			continue
		}
		switch ke.Name {
		case key.NameLeftArrow:
			if st.focus == multiRenameFocusActions {
				if st.stepAction(-1) {
					gtx.Execute(op.InvalidateCmd{})
				}
			} else if st.stepChoice(st.focus, -1, gtx.Now) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameRightArrow:
			if st.focus == multiRenameFocusActions {
				if st.stepAction(1) {
					gtx.Execute(op.InvalidateCmd{})
				}
			} else if st.stepChoice(st.focus, 1, gtx.Now) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameEnter, key.NameReturn, key.NameSpace:
			if st.focus == multiRenameFocusActions {
				if ui.activateMultiRenameAction(gtx, st, st.actionFocus) {
					return
				}
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
	changed := false
	for _, ed := range []*widget.Editor{&st.searchEdit, &st.replaceEdit, &st.prefixEdit, &st.suffixEdit, &st.startEdit, &st.stepEdit, &st.digitsEdit} {
		for {
			ev, ok := ed.Update(gtx)
			if !ok {
				break
			}
			switch e := ev.(type) {
			case widget.ChangeEvent:
				changed = true
			case widget.SubmitEvent:
				ed.SetText(e.Text)
				changed = true
				st.refreshPreview()
				if st.changedCount() > 0 {
					ui.activateMultiRenameAction(gtx, st, multiRenameActionRename)
					gtx.Execute(op.InvalidateCmd{})
				}
			}
		}
	}
	if changed {
		st.refreshPreview()
	}
}

func (ui *UI) layoutMultiRename(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.multiRename
	if st == nil {
		return layout.Dimensions{}
	}
	ui.pollMultiRename(gtx.Now)
	st = ui.multiRename
	if st == nil {
		return layout.Dimensions{}
	}
	if st.focusWant && !st.running {
		st.focusWant = false
		gtx.Execute(key.FocusCmd{Tag: &st.searchEdit})
	}
	for i := range st.caseClicks {
		for st.caseClicks[i].Clicked(gtx) {
			st.setFocus(gtx, multiRenameFocusCase)
			st.caseMode = multiRenameCaseMode(i)
			st.caseTabsAnim.setPulse(fmt.Sprintf("case:%d", i), gtx.Now)
			st.refreshPreview()
		}
	}
	scopeLocked := st.hasDirectoryTarget()
	for i := range st.scopeClicks {
		for st.scopeClicks[i].Clicked(gtx) {
			// Directories have no extension, so the "Extension" and "Both"
			// tabs are disabled; ignore stray clicks on them.
			if scopeLocked && multiRenameScope(i) != multiRenameScopeName {
				continue
			}
			st.setFocus(gtx, multiRenameFocusScope)
			st.scope = multiRenameScope(i)
			st.scopeTabsAnim.setPulse(fmt.Sprintf("scope:%d", i), gtx.Now)
			st.refreshPreview()
		}
	}
	for i := range st.positionClicks {
		for st.positionClicks[i].Clicked(gtx) {
			st.setFocus(gtx, multiRenameFocusCounterPosition)
			st.sequenceAtEnd = i == 1
			st.positionTabsAnim.setPulse(fmt.Sprintf("position:%d", i), gtx.Now)
			st.refreshPreview()
		}
	}
	if !st.running {
		if st.cancelClick.Clicked(gtx) || st.closeClick.Clicked(gtx) {
			ui.activateMultiRenameAction(gtx, st, multiRenameActionCancel)
			return layout.Dimensions{}
		}
		if st.renameClick.Clicked(gtx) && st.changedCount() > 0 {
			ui.activateMultiRenameAction(gtx, st, multiRenameActionRename)
		}
	} else {
		for st.cancelClick.Clicked(gtx) {
		}
		for st.closeClick.Clicked(gtx) {
		}
		for st.renameClick.Clicked(gtx) {
		}
	}
	for st.backdropClick.Clicked(gtx) {
	}
	st.refreshPreview()

	return st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{A: 130}, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())
		width := gtx.Dp(unit.Dp(760))
		if maxWidth := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(28)); width > maxWidth {
			width = maxWidth
		}
		if width < gtx.Dp(unit.Dp(340)) {
			width = gtx.Dp(unit.Dp(340))
		}
		m := op.Record(gtx.Ops)
		dialog := fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
			return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneOverlayCornerDp)), color.NRGBA{R: 20, G: 20, B: 20, A: 252}, color.NRGBA{R: 255, G: 255, B: 255, A: 18}, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutMultiRenameBody(th, gtx, st)
				})
			})
		})
		call := m.Stop()
		x := (gtx.Constraints.Max.X - dialog.Size.X) / 2
		y := (gtx.Constraints.Max.Y - dialog.Size.Y) / 2
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

func (ui *UI) layoutMultiRenameBody(th *material.Theme, gtx layout.Context, st *multiRenameState) layout.Dimensions {
	if st.running {
		gtx = gtx.Disabled()
	}
	hoverKey := ""
	if !st.running && st.cancelClick.Hovered() {
		hoverKey = "cancel"
	}
	if !st.running && st.changedCount() > 0 && st.renameClick.Hovered() {
		hoverKey = "rename"
	}
	st.actionsAnim.setHover(hoverKey, gtx.Now)
	hoverCancel, animCancel := st.actionsAnim.hoverFill(gtx.Now, "cancel")
	hoverRename, animRename := st.actionsAnim.hoverFill(gtx.Now, "rename")
	pulseCancel, pulseAnimCancel := st.actionsAnim.pulseFill(gtx.Now, "cancel")
	pulseRename, pulseAnimRename := st.actionsAnim.pulseFill(gtx.Now, "rename")
	if animCancel || animRename || pulseAnimCancel || pulseAnimRename {
		gtx.Execute(op.InvalidateCmd{})
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					title := material.Body1(th, fmt.Sprintf("Multi-Rename  ·  %d selected", len(st.targets)))
					title.Font.Typeface = ui.interfaceTypeface()
					title.Font.Weight = font.Bold
					title.TextSize = ui.scaleDialogFontSize(12)
					title.Color = txtColor
					return title.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutTinyIconModeButton(th, gtx, &st.closeClick, uitheme.CloseIcon(), st.running)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutMultiRenameEditorRow(th, gtx, st)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutMultiRenameScopeControl(th, gtx, st)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutMultiRenameCaseControl(th, gtx, st)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutMultiRenameCounterControl(th, gtx, st)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.layoutMultiRenamePreview(th, gtx, st) }),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			text, col := fmt.Sprintf("%d of %d filenames will change", st.changedCount(), len(st.targets)), color.NRGBA{R: 170, G: 170, B: 170, A: 255}
			if st.running {
				text = "Renaming…"
			} else if st.operationErr != "" {
				text, col = st.operationErr, color.NRGBA{R: 220, G: 140, B: 140, A: 255}
			} else if st.lastErr != "" {
				text, col = st.lastErr, color.NRGBA{R: 220, G: 140, B: 140, A: 255}
			}
			lbl := material.Caption(th, text)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleDialogFontSize(9)
			lbl.Color = col
			lbl.MaxLines = 1
			lbl.Truncator = "…"
			return fixedHeight(gtx, gtx.Dp(unit.Dp(18)), lbl.Layout)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutDialogActionPair(th, gtx,
					&st.cancelClick, "Cancel", hoverCancel, pulseCancel, st.running,
					&st.renameClick, "Rename", hoverRename, pulseRename, st.running || st.changedCount() == 0,
					st.actionVisualState(multiRenameActionCancel), st.actionVisualState(multiRenameActionRename),
				)
			})
		}),
	)
}

func (ui *UI) layoutMultiRenameEditorRow(th *material.Theme, gtx layout.Context, st *multiRenameState) layout.Dimensions {
	type fieldSpec struct {
		label string
		hint  string
		edit  *widget.Editor
		focus multiRenameFocus
	}
	fields := []fieldSpec{
		{label: "Find", hint: "text to replace", edit: &st.searchEdit, focus: multiRenameFocusFind},
		{label: "Replace with", hint: "new text", edit: &st.replaceEdit, focus: multiRenameFocusReplace},
		{label: "Prefix", hint: "add before", edit: &st.prefixEdit, focus: multiRenameFocusPrefix},
		{label: "Suffix", hint: "add after", edit: &st.suffixEdit, focus: multiRenameFocusSuffix},
	}
	children := make([]layout.FlexChild, 0, len(fields)*2-1)
	for i, field := range fields {
		field := field
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
		}
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSSHField(th, gtx, field.label, field.edit, field.hint, !st.running, st.focus == field.focus || gtx.Focused(field.edit))
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func (ui *UI) layoutMultiRenameCaseControl(th *material.Theme, gtx layout.Context, st *multiRenameState) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(ui.multiRenameGroupLabel(th, "Letter case")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutMultiRenameTabs(th, gtx, []string{"Keep", "To upper", "To lower"}, st.caseClicks[:], int(st.caseMode), &st.caseTabsAnim, "case", st.focus == multiRenameFocusCase)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutThemeCheckbox(th, gtx, &st.caseSensitive, "Find is case sensitive", ui.scaleDialogFontSize(8))
		}),
	)
}

func (ui *UI) layoutMultiRenameScopeControl(th *material.Theme, gtx layout.Context, st *multiRenameState) layout.Dimensions {
	locked := st.hasDirectoryTarget()
	active := int(st.effectiveScope())
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(ui.multiRenameGroupLabel(th, "Apply actions to")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutMultiRenameTabs(th, gtx, []string{"Name", "Extension", "Both"}, st.scopeClicks[:], active, &st.scopeTabsAnim, "scope", st.focus == multiRenameFocusScope, false, locked, locked)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			text := "Extension stays unchanged"
			switch {
			case locked:
				text = "Folder selected · only the name is renamed"
			case st.scope == multiRenameScopeExtension:
				text = "Name stays unchanged"
			case st.scope == multiRenameScopeBoth:
				text = "Complete filename is edited"
			}
			lbl := material.Caption(th, text)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleDialogFontSize(8)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}),
	)
}

func (ui *UI) multiRenameGroupLabel(th *material.Theme, text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lbl := material.Caption(th, text)
		lbl.Font.Typeface = ui.interfaceTypeface()
		lbl.Font.Weight = font.Medium
		lbl.TextSize = ui.scaleDialogFontSize(8)
		lbl.Color = hintColor
		return lbl.Layout(gtx)
	}
}

func (ui *UI) layoutMultiRenameTabs(th *material.Theme, gtx layout.Context, labels []string, clicks []widget.Clickable, active int, anim *segmentedAnimState, keyPrefix string, focused bool, disabled ...bool) layout.Dimensions {
	if len(labels) == 0 || len(clicks) < len(labels) {
		return layout.Dimensions{}
	}
	if active < 0 || active >= len(labels) {
		active = 0
	}
	tabDisabled := func(i int) bool {
		return i < len(disabled) && disabled[i]
	}
	hoverKey := ""
	for i := range labels {
		if !tabDisabled(i) && clicks[i].Hovered() {
			hoverKey = fmt.Sprintf("%s:%d", keyPrefix, i)
			break
		}
	}
	anim.setHover(hoverKey, gtx.Now)
	specs := make([]slidingTabSpec, 0, len(labels))
	animating := false
	for i, label := range labels {
		key := fmt.Sprintf("%s:%d", keyPrefix, i)
		hover, hoverAnim := anim.hoverFill(gtx.Now, key)
		pulse, pulseAnim := anim.pulseFill(gtx.Now, key)
		if hoverAnim || pulseAnim {
			animating = true
		}
		if tabDisabled(i) {
			hover, pulse = 0, 0
		}
		fill := float32(0)
		if i == active {
			fill = 1
		}
		focusFill := float32(0)
		if focused && i == active {
			focusFill = 1
		}
		specs = append(specs, slidingTabSpec{Label: label, Click: &clicks[i], ActiveFill: fill, HoverFill: hover, PulseFill: pulse, FocusFill: focusFill, Disabled: tabDisabled(i)})
	}
	if animating {
		gtx.Execute(op.InvalidateCmd{})
	}
	return ui.layoutSlidingTabStrip(th, gtx, gtx.Dp(unit.Dp(22)), float32(active), ui.scaleDialogFontSize(9), specs)
}

func (ui *UI) layoutMultiRenameCounterControl(th *material.Theme, gtx layout.Context, st *multiRenameState) layout.Dimensions {
	placementActive := 0
	if st.sequenceAtEnd {
		placementActive = 1
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(132)), func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(ui.multiRenameGroupLabel(th, "Dynamic element")),
					layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.layoutThemeCheckbox(th, gtx, &st.sequence, "Add counter", ui.scaleDialogFontSize(9))
					}),
				)
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(58)), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSSHField(th, gtx, "Start", &st.startEdit, "1", st.sequence.Value && !st.running, st.focus == multiRenameFocusCounterStart || gtx.Focused(&st.startEdit))
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(58)), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSSHField(th, gtx, "Step", &st.stepEdit, "1", st.sequence.Value && !st.running, st.focus == multiRenameFocusCounterStep || gtx.Focused(&st.stepEdit))
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(58)), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSSHField(th, gtx, "Digits", &st.digitsEdit, "2", st.sequence.Value && !st.running, st.focus == multiRenameFocusCounterDigits || gtx.Focused(&st.digitsEdit))
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if !st.sequence.Value {
				gtx = gtx.Disabled()
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(ui.multiRenameGroupLabel(th, "Counter position")),
				layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutMultiRenameTabs(th, gtx, []string{"Before name", "After name"}, st.positionClicks[:], placementActive, &st.positionTabsAnim, "position", st.focus == multiRenameFocusCounterPosition)
				}),
			)
		}),
	)
}

func (ui *UI) layoutMultiRenamePreview(th *material.Theme, gtx layout.Context, st *multiRenameState) layout.Dimensions {
	header := func(text string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, text)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.Font.Weight = font.Bold
			lbl.TextSize = ui.scaleDialogFontSize(9)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}
	}
	return fillRoundedBox(gtx, gtx.Dp(unit.Dp(4)), color.NRGBA{R: 24, G: 24, B: 24, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 20}, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, layout.Flexed(1, header("Original name")), layout.Rigid(layout.Spacer{Width: unit.Dp(18)}.Layout), layout.Flexed(1, header("New name")))
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 18}, clip.Rect(image.Rect(0, 0, gtx.Constraints.Max.X, 1)).Op())
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, gtx.Dp(unit.Dp(180)), func(gtx layout.Context) layout.Dimensions {
						return material.List(th, &st.previewList).Layout(gtx, len(st.targets), func(gtx layout.Context, i int) layout.Dimensions {
							target := st.targets[i]
							changed := target.newName != target.oldName
							cell := func(text string, changedCell bool) layout.Widget {
								return func(gtx layout.Context) layout.Dimensions {
									lbl := material.Body2(th, text)
									lbl.Font.Typeface = ui.interfaceTypeface()
									lbl.TextSize = ui.scaleDialogFontSize(9)
									lbl.Color = color.NRGBA{R: 185, G: 185, B: 185, A: 255}
									if changedCell {
										lbl.Color = color.NRGBA{R: 225, G: 225, B: 225, A: 255}
									}
									lbl.MaxLines = 1
									lbl.Truncator = "…"
									return lbl.Layout(gtx)
								}
							}
							return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, layout.Flexed(1, cell(target.oldName, false)), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									arrow := material.Caption(th, "→")
									arrow.Font.Typeface = ui.interfaceTypeface()
									arrow.TextSize = ui.scaleDialogFontSize(9)
									arrow.Color = hintColor
									return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5)}.Layout(gtx, arrow.Layout)
								}), layout.Flexed(1, cell(target.newName, changed)))
							})
						})
					})
				}),
			)
		})
	})
}
