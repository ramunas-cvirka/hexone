// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"hexone/fm"
	uitheme "hexone/ui/theme"
	"image"
	"image/color"
	"strconv"
	"strings"

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

type sshModalState struct {
	backdropClick widget.Clickable
	closeClick    widget.Clickable
	saveClick     widget.Clickable
	connectClick  widget.Clickable
	cancelClick   widget.Clickable
	addClick      widget.Clickable

	setups            []fm.SSHSetup
	selected          int
	setupClicks       []widget.Clickable
	setupRemoveClicks []widget.Clickable
	setupList         layout.List

	nameEdit    widget.Editor
	hostEdit    widget.Editor
	portEdit    widget.Editor
	userEdit    widget.Editor
	passEdit    widget.Editor
	keyPathEdit widget.Editor
	keyPassEdit widget.Editor

	savedSetups []fm.SSHSetup
	footerAnim  segmentedAnimState
	errText     string
	keyFocus    dialogKeyboardFocusState
	focus       sshModalFocus
	actionFocus sshModalAction
}

type sshModalFocus uint8

const (
	sshModalFocusNone sshModalFocus = iota
	sshModalFocusAdd
	sshModalFocusSetupsList
	sshModalFocusRemove
	sshModalFocusHost
	sshModalFocusPort
	sshModalFocusUser
	sshModalFocusPassword
	sshModalFocusKeyPath
	sshModalFocusPassphrase
	sshModalFocusActions
)

type sshModalAction uint8

const (
	sshModalActionCancel sshModalAction = iota
	sshModalActionSave
	sshModalActionConnect
)

func (st *sshModalState) currentEditorSetup() (fm.SSHSetup, bool) {
	if st == nil {
		return fm.SSHSetup{}, false
	}
	setup := fm.SSHSetup{
		Name:          strings.TrimSpace(st.nameEdit.Text()),
		Host:          strings.TrimSpace(st.hostEdit.Text()),
		User:          strings.TrimSpace(st.userEdit.Text()),
		Password:      st.passEdit.Text(),
		KeyPath:       strings.TrimSpace(st.keyPathEdit.Text()),
		KeyPassphrase: st.keyPassEdit.Text(),
	}
	portText := strings.TrimSpace(st.portEdit.Text())
	if p, err := strconv.Atoi(portText); err == nil && p > 0 && p <= 65535 {
		setup.Port = p
	} else {
		setup.Port = 22
	}
	nonEmpty := setup.Name != "" || setup.Host != "" || setup.User != "" ||
		setup.Password != "" || setup.KeyPath != "" || setup.KeyPassphrase != ""
	return setup, nonEmpty
}

func (ui *UI) openSSHModal() {
	if ui == nil {
		return
	}
	ui.closeFunctionBarToolsMenu()
	if ui.fmCfg == nil {
		ui.fmCfg = fm.DefaultConfig()
	}

	st := ui.sshModal
	if st == nil {
		st = &sshModalState{}
		st.setupList.Axis = layout.Vertical

		st.nameEdit.SingleLine = true
		st.nameEdit.Submit = true
		st.hostEdit.SingleLine = true
		st.hostEdit.Submit = true
		st.portEdit.SingleLine = true
		st.portEdit.Submit = true
		st.portEdit.Filter = "0123456789"
		st.userEdit.SingleLine = true
		st.userEdit.Submit = true
		st.passEdit.SingleLine = true
		st.passEdit.Submit = true
		st.keyPathEdit.SingleLine = true
		st.keyPathEdit.Submit = true
		st.keyPassEdit.SingleLine = true
		st.keyPassEdit.Submit = true
	}

	st.loadFromConfig(ui.fmCfg)
	st.focusKeyboard()
	ui.sshModal = st
}

func (ui *UI) closeSSHModal() {
	ui.sshModal = nil
}

func (st *sshModalState) loadFromConfig(cfg *fm.Config) {
	st.loadFromConfigWithSelected(cfg, 0)
}

func (st *sshModalState) loadFromConfigWithSelected(cfg *fm.Config, selected int) {
	if st == nil || cfg == nil {
		return
	}
	st.setups = cloneSSHSetups(cfg.SSH.Setups)
	st.savedSetups = cloneSSHSetups(cfg.SSH.Setups)
	if len(st.setups) > 0 {
		if selected < 0 {
			selected = 0
		}
		if selected >= len(st.setups) {
			selected = len(st.setups) - 1
		}
		st.selected = selected
		st.setupList.Position.First = st.selected
		st.setupList.Position.Offset = 0
	} else {
		st.selected = -1
		st.setupList.Position = layout.Position{}
	}
	st.loadEditorsFromSelected()
	st.errText = ""
	st.focus = st.primaryFocus()
	st.actionFocus = sshModalActionConnect
}

func cloneSSHSetups(src []fm.SSHSetup) []fm.SSHSetup {
	if len(src) == 0 {
		return nil
	}
	out := make([]fm.SSHSetup, len(src))
	copy(out, src)
	return out
}

func sshSetupIdentity(setup fm.SSHSetup) string {
	user := strings.TrimSpace(setup.User)
	host := strings.TrimSpace(setup.Host)
	port := setup.Port
	if port <= 0 {
		port = 22
	}
	switch {
	case user != "" && host != "":
		return user + "@" + host + ":" + strconv.Itoa(port)
	case host != "":
		return host + ":" + strconv.Itoa(port)
	case user != "":
		return user + "@?:" + strconv.Itoa(port)
	default:
		return "?:22"
	}
}

func (st *sshModalState) clearEditors() {
	st.nameEdit.SetText("")
	st.hostEdit.SetText("")
	st.portEdit.SetText("22")
	st.userEdit.SetText("")
	st.passEdit.SetText("")
	st.keyPathEdit.SetText("")
	st.keyPassEdit.SetText("")
}

func (st *sshModalState) loadEditorsFromSelected() {
	if st == nil {
		return
	}
	if st.selected < 0 || st.selected >= len(st.setups) {
		st.clearEditors()
		return
	}
	cur := st.setups[st.selected]
	st.nameEdit.SetText(cur.Name)
	st.hostEdit.SetText(cur.Host)
	port := cur.Port
	if port <= 0 {
		port = 22
	}
	st.portEdit.SetText(strconv.Itoa(port))
	st.userEdit.SetText(cur.User)
	st.passEdit.SetText(cur.Password)
	st.keyPathEdit.SetText(cur.KeyPath)
	st.keyPassEdit.SetText(cur.KeyPassphrase)
}

func (st *sshModalState) syncSelectedFromEditors() {
	if st == nil || st.selected < 0 || st.selected >= len(st.setups) {
		return
	}
	cur := &st.setups[st.selected]
	cur.Name = strings.TrimSpace(st.nameEdit.Text())
	cur.Host = strings.TrimSpace(st.hostEdit.Text())
	cur.User = strings.TrimSpace(st.userEdit.Text())
	cur.Password = st.passEdit.Text()
	cur.KeyPath = strings.TrimSpace(st.keyPathEdit.Text())
	cur.KeyPassphrase = st.keyPassEdit.Text()

	portText := strings.TrimSpace(st.portEdit.Text())
	if p, err := strconv.Atoi(portText); err == nil && p > 0 && p <= 65535 {
		cur.Port = p
	} else if cur.Port <= 0 {
		cur.Port = 22
	}
}

func (st *sshModalState) focusKeyboard() {
	if st == nil {
		return
	}
	st.keyFocus.focusKeyboard()
}

func (st *sshModalState) hasFocusedEditor(gtx layout.Context) bool {
	if st == nil {
		return false
	}
	return gtx.Focused(&st.nameEdit) ||
		gtx.Focused(&st.hostEdit) ||
		gtx.Focused(&st.portEdit) ||
		gtx.Focused(&st.userEdit) ||
		gtx.Focused(&st.passEdit) ||
		gtx.Focused(&st.keyPathEdit) ||
		gtx.Focused(&st.keyPassEdit)
}

func (st *sshModalState) syncFocus(gtx layout.Context) {
	if st == nil {
		return
	}
	switch {
	case gtx.Focused(&st.hostEdit):
		st.focus = sshModalFocusHost
	case gtx.Focused(&st.portEdit):
		st.focus = sshModalFocusPort
	case gtx.Focused(&st.userEdit):
		st.focus = sshModalFocusUser
	case gtx.Focused(&st.passEdit):
		st.focus = sshModalFocusPassword
	case gtx.Focused(&st.keyPathEdit):
		st.focus = sshModalFocusKeyPath
	case gtx.Focused(&st.keyPassEdit):
		st.focus = sshModalFocusPassphrase
	}
}

func normalizeSSHSetupForState(raw fm.SSHSetup) fm.SSHSetup {
	setup := fm.SSHSetup{
		Name:          strings.TrimSpace(raw.Name),
		Host:          strings.TrimSpace(raw.Host),
		Port:          raw.Port,
		User:          strings.TrimSpace(raw.User),
		Password:      raw.Password,
		KeyPath:       strings.TrimSpace(raw.KeyPath),
		KeyPassphrase: raw.KeyPassphrase,
	}
	if setup.Port <= 0 {
		setup.Port = 22
	}
	return setup
}

func sshSetupSlicesEqualForState(a, b []fm.SSHSetup) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if normalizeSSHSetupForState(a[i]) != normalizeSSHSetupForState(b[i]) {
			return false
		}
	}
	return true
}

func (st *sshModalState) draftSetups() []fm.SSHSetup {
	if st == nil {
		return nil
	}
	draft := cloneSSHSetups(st.setups)
	if st.selected >= 0 && st.selected < len(draft) {
		setup, _ := st.currentEditorSetup()
		draft[st.selected] = setup
		return draft
	}
	setup, hasInput := st.currentEditorSetup()
	if hasInput {
		draft = append(draft, setup)
	}
	return draft
}

func (st *sshModalState) editorMatchesSetup(setup fm.SSHSetup) bool {
	if st == nil {
		return true
	}
	port := setup.Port
	if port <= 0 {
		port = 22
	}
	return strings.TrimSpace(st.hostEdit.Text()) == strings.TrimSpace(setup.Host) &&
		strings.TrimSpace(st.portEdit.Text()) == strconv.Itoa(port) &&
		strings.TrimSpace(st.userEdit.Text()) == strings.TrimSpace(setup.User) &&
		st.passEdit.Text() == setup.Password &&
		strings.TrimSpace(st.keyPathEdit.Text()) == strings.TrimSpace(setup.KeyPath) &&
		st.keyPassEdit.Text() == setup.KeyPassphrase
}

func (st *sshModalState) hasUnsavedChanges() bool {
	if st == nil {
		return false
	}
	if st.selected >= 0 && st.selected < len(st.setups) && !st.editorMatchesSetup(st.setups[st.selected]) {
		return true
	}
	return !sshSetupSlicesEqualForState(st.draftSetups(), st.savedSetups)
}

func (st *sshModalState) defaultAction() sshModalAction {
	if st == nil {
		return sshModalActionConnect
	}
	if st.hasUnsavedChanges() {
		return sshModalActionSave
	}
	return sshModalActionConnect
}

func (st *sshModalState) primaryFocus() sshModalFocus {
	if st == nil {
		return sshModalFocusNone
	}
	if len(st.setups) > 0 {
		return sshModalFocusSetupsList
	}
	return sshModalFocusAdd
}

func (st *sshModalState) visibleFocus() sshModalFocus {
	if st == nil {
		return sshModalFocusNone
	}
	if st.focus == sshModalFocusNone {
		return st.primaryFocus()
	}
	return st.focus
}

func (st *sshModalState) focusOrder() []sshModalFocus {
	if st == nil {
		return nil
	}
	order := make([]sshModalFocus, 0, 10)
	order = append(order, sshModalFocusAdd)
	if len(st.setups) > 0 {
		order = append(order, sshModalFocusSetupsList)
	}
	if st.selected >= 0 && st.selected < len(st.setups) {
		order = append(order, sshModalFocusRemove)
	}
	order = append(order,
		sshModalFocusHost,
		sshModalFocusPort,
		sshModalFocusUser,
		sshModalFocusPassword,
		sshModalFocusKeyPath,
		sshModalFocusPassphrase,
		sshModalFocusActions,
	)
	return order
}

func (st *sshModalState) canFocus(target sshModalFocus) bool {
	if st == nil {
		return false
	}
	switch target {
	case sshModalFocusAdd, sshModalFocusHost, sshModalFocusPort, sshModalFocusUser, sshModalFocusPassword, sshModalFocusKeyPath, sshModalFocusPassphrase, sshModalFocusActions:
		return true
	case sshModalFocusSetupsList:
		return len(st.setups) > 0
	case sshModalFocusRemove:
		return st.selected >= 0 && st.selected < len(st.setups)
	default:
		return false
	}
}

func (st *sshModalState) focusOrderEditors() []sshModalFocus {
	if st == nil {
		return nil
	}
	return []sshModalFocus{
		sshModalFocusHost,
		sshModalFocusPort,
		sshModalFocusUser,
		sshModalFocusPassword,
		sshModalFocusKeyPath,
		sshModalFocusPassphrase,
	}
}

func (st *sshModalState) editorForFocus(target sshModalFocus) *widget.Editor {
	if st == nil {
		return nil
	}
	switch target {
	case sshModalFocusHost:
		return &st.hostEdit
	case sshModalFocusPort:
		return &st.portEdit
	case sshModalFocusUser:
		return &st.userEdit
	case sshModalFocusPassword:
		return &st.passEdit
	case sshModalFocusKeyPath:
		return &st.keyPathEdit
	case sshModalFocusPassphrase:
		return &st.keyPassEdit
	default:
		return nil
	}
}

func (st *sshModalState) setFocus(gtx layout.Context, target sshModalFocus) bool {
	if st == nil {
		return false
	}
	if !st.canFocus(target) {
		return false
	}
	changed := st.focus != target
	st.focus = target
	if target == sshModalFocusActions {
		st.actionFocus = st.defaultAction()
	}
	if ed := st.editorForFocus(target); ed != nil {
		gtx.Execute(key.FocusCmd{Tag: ed})
		return changed
	}
	gtx.Execute(key.FocusCmd{Tag: &st.keyFocus.tag})
	return changed
}

func (st *sshModalState) stepFocus(gtx layout.Context, step int) bool {
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

func (st *sshModalState) stepAction(step int) bool {
	if st == nil {
		return false
	}
	order := []sshModalAction{sshModalActionCancel, sshModalActionSave, sshModalActionConnect}
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

func (st *sshModalState) actionVisualState(target sshModalAction) dialogActionVisualState {
	if st == nil {
		return dialogActionVisualState{}
	}
	if st.focus == sshModalFocusActions {
		active := st.actionFocus == target
		return dialogActionVisualState{Focused: active, Default: active}
	}
	return dialogActionVisualState{Default: target == st.defaultAction()}
}

func (st *sshModalState) ensureSelectedVisible() {
	if st == nil || st.selected < 0 || st.selected >= len(st.setups) {
		return
	}
	if rowIndexVisible(st.setupList.Position, st.selected, len(st.setups)) {
		return
	}
	visible := st.setupList.Position.Count
	if visible <= 0 {
		st.setupList.Position.First = st.selected
		st.setupList.Position.Offset = 0
		return
	}
	first := st.setupList.Position.First
	if first < 0 {
		first = 0
	}
	last := first + visible - 1
	if st.selected < first {
		first = st.selected
	} else if st.selected > last {
		first = st.selected - (visible - 1)
	}
	if first < 0 {
		first = 0
	}
	if first >= len(st.setups) {
		first = len(st.setups) - 1
	}
	st.setupList.Position.First = first
	st.setupList.Position.Offset = 0
}

func (st *sshModalState) setSelected(index int) bool {
	if st == nil || len(st.setups) == 0 {
		return false
	}
	if index < 0 {
		index = 0
	}
	if index >= len(st.setups) {
		index = len(st.setups) - 1
	}
	if index == st.selected {
		return false
	}
	st.syncSelectedFromEditors()
	st.selected = index
	st.loadEditorsFromSelected()
	st.errText = ""
	st.ensureSelectedVisible()
	return true
}

func (st *sshModalState) stepSelected(step int) bool {
	if st == nil || step == 0 || len(st.setups) == 0 {
		return false
	}
	next := st.selected
	if next < 0 {
		if step > 0 {
			next = 0
		} else {
			next = len(st.setups) - 1
		}
	} else {
		next += step
	}
	return st.setSelected(next)
}

func (st *sshModalState) addSetup() bool {
	if st == nil {
		return false
	}
	if st.selected >= 0 && st.selected < len(st.setups) {
		st.syncSelectedFromEditors()
	}
	setup, hasInput := st.currentEditorSetup()
	if hasInput && st.selected < 0 {
		st.setups = append(st.setups, setup)
	} else {
		st.setups = append(st.setups, fm.SSHSetup{Port: 22})
	}
	st.selected = len(st.setups) - 1
	st.loadEditorsFromSelected()
	st.ensureSelectedVisible()
	st.errText = ""
	return true
}

func (st *sshModalState) removeSetup(index int) bool {
	if st == nil || index < 0 || index >= len(st.setups) {
		return false
	}
	st.setups = append(st.setups[:index], st.setups[index+1:]...)
	if len(st.setups) == 0 {
		st.selected = -1
	} else {
		if st.selected == index {
			if index >= len(st.setups) {
				st.selected = len(st.setups) - 1
			}
		} else if st.selected > index {
			st.selected--
		}
	}
	st.loadEditorsFromSelected()
	st.errText = ""
	return true
}

func (st *sshModalState) validatedSetupsWithSelected() ([]fm.SSHSetup, int, error) {
	if st == nil {
		return nil, -1, nil
	}
	out := make([]fm.SSHSetup, 0, len(st.setups))
	selected := -1
	for i, raw := range st.setups {
		setup := fm.SSHSetup{
			Name:          strings.TrimSpace(raw.Name),
			Host:          strings.TrimSpace(raw.Host),
			Port:          raw.Port,
			User:          strings.TrimSpace(raw.User),
			Password:      raw.Password,
			KeyPath:       strings.TrimSpace(raw.KeyPath),
			KeyPassphrase: raw.KeyPassphrase,
		}
		if setup.Port <= 0 {
			setup.Port = 22
		}
		if setup.Port > 65535 {
			return nil, -1, fmt.Errorf("setup %d port must be between 1 and 65535", i+1)
		}
		empty := setup.Name == "" && setup.Host == "" && setup.User == "" &&
			setup.Password == "" && setup.KeyPath == "" && setup.KeyPassphrase == ""
		if empty {
			continue
		}
		if setup.Host == "" {
			return nil, -1, fmt.Errorf("setup %d host is required", i+1)
		}
		if setup.User == "" {
			return nil, -1, fmt.Errorf("setup %d user is required", i+1)
		}
		// Keep name derived from connection identity.
		setup.Name = sshSetupIdentity(setup)
		if i == st.selected {
			selected = len(out)
		}
		out = append(out, setup)
	}
	return out, selected, nil
}

func (st *sshModalState) ensureSetupClicks(n int) {
	if st == nil {
		return
	}
	if n <= 0 {
		st.setupClicks = nil
		return
	}
	if len(st.setupClicks) == n {
		return
	}
	next := make([]widget.Clickable, n)
	copy(next, st.setupClicks)
	st.setupClicks = next
}

func (st *sshModalState) ensureSetupRemoveClicks(n int) {
	if st == nil {
		return
	}
	if n <= 0 {
		st.setupRemoveClicks = nil
		return
	}
	if len(st.setupRemoveClicks) == n {
		return
	}
	next := make([]widget.Clickable, n)
	copy(next, st.setupRemoveClicks)
	st.setupRemoveClicks = next
}

func (ui *UI) saveSSHModal() error {
	st := ui.sshModal
	if st == nil {
		return nil
	}
	if ui.fmCfg == nil {
		ui.fmCfg = fm.DefaultConfig()
	}
	if st.selected >= 0 && st.selected < len(st.setups) {
		st.syncSelectedFromEditors()
	} else {
		setup, hasInput := st.currentEditorSetup()
		if hasInput {
			st.setups = append(st.setups, setup)
			st.selected = len(st.setups) - 1
		}
	}
	setups, selected, err := st.validatedSetupsWithSelected()
	if err != nil {
		return err
	}
	ui.fmCfg.SSH.Setups = setups
	if err := ui.saveFMConfig(); err != nil {
		return err
	}
	st.loadFromConfigWithSelected(ui.fmCfg, selected)
	return nil
}

func (ui *UI) activateSSHModalAction(gtx layout.Context, st *sshModalState, action sshModalAction) bool {
	if ui == nil || st == nil {
		return false
	}
	switch action {
	case sshModalActionCancel:
		st.footerAnim.setPulse("cancel", gtx.Now)
		ui.closeSSHModal()
		return true
	case sshModalActionSave:
		st.footerAnim.setPulse("save", gtx.Now)
		if err := ui.saveSSHModal(); err != nil {
			st.errText = err.Error()
		} else {
			st.focus = st.primaryFocus()
			st.focusKeyboard()
		}
	case sshModalActionConnect:
		st.footerAnim.setPulse("connect", gtx.Now)
		if err := ui.connectSSHModalToActivePane(gtx.Now); err != nil {
			st.errText = err.Error()
		} else {
			ui.closeSSHModal()
			return true
		}
	}
	st.actionFocus = st.defaultAction()
	return false
}

func (ui *UI) layoutSSHModal(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.sshModal
	if st == nil {
		return layout.Dimensions{}
	}
	st.keyFocus.attach(gtx)
	st.syncFocus(gtx)

	// Explicitly drain Ctrl/Cmd+F while modal is open to avoid macOS beep
	// without stealing normal editor shortcuts such as paste/select-all.
	anyMods := ^key.Modifiers(0)
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: "F", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "f", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "F", Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "f", Required: key.ModShortcut, Optional: anyMods},
		)
		if !ok {
			break
		}
		_, _ = ev.(key.Event)
	}

	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if ok && ke.State == key.Press && ke.Name == key.NameEscape {
			ui.closeSSHModal()
			return layout.Dimensions{}
		}
	}
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameTab, Optional: anyMods})
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		step, ok := dialogTabStep(ke.Modifiers)
		if !ok {
			continue
		}
		if st.stepFocus(gtx, step) {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	for {
		ev, ok := gtx.Event(
			key.FocusFilter{Target: &st.keyFocus.tag},
			key.Filter{Focus: &st.keyFocus.tag, Name: key.NameUpArrow},
			key.Filter{Focus: &st.keyFocus.tag, Name: key.NameDownArrow},
			key.Filter{Focus: &st.keyFocus.tag, Name: key.NameLeftArrow},
			key.Filter{Focus: &st.keyFocus.tag, Name: key.NameRightArrow},
			key.Filter{Focus: &st.keyFocus.tag, Name: key.NameEnter},
			key.Filter{Focus: &st.keyFocus.tag, Name: key.NameReturn},
		)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press || ke.Modifiers != 0 {
			continue
		}
		switch ke.Name {
		case key.NameUpArrow:
			switch st.focus {
			case sshModalFocusRemove:
				st.focus = sshModalFocusSetupsList
				if st.stepSelected(-1) || len(st.setups) > 0 {
					gtx.Execute(op.InvalidateCmd{})
				}
			case sshModalFocusSetupsList, sshModalFocusNone:
				if st.stepSelected(-1) {
					gtx.Execute(op.InvalidateCmd{})
				}
			}
		case key.NameDownArrow:
			switch st.focus {
			case sshModalFocusRemove:
				st.focus = sshModalFocusSetupsList
				if st.stepSelected(1) || len(st.setups) > 0 {
					gtx.Execute(op.InvalidateCmd{})
				}
			case sshModalFocusSetupsList, sshModalFocusNone:
				if st.stepSelected(1) {
					gtx.Execute(op.InvalidateCmd{})
				}
			}
		case key.NameLeftArrow:
			if st.focus == sshModalFocusActions && st.stepAction(-1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameRightArrow:
			if st.focus == sshModalFocusActions && st.stepAction(1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameEnter, key.NameReturn:
			switch st.focus {
			case sshModalFocusRemove:
				if st.removeSetup(st.selected) {
					st.focus = st.primaryFocus()
					st.actionFocus = st.defaultAction()
					st.focusKeyboard()
					gtx.Execute(op.InvalidateCmd{})
				}
			case sshModalFocusAdd:
				if st.addSetup() {
					st.focus = st.primaryFocus()
					st.actionFocus = st.defaultAction()
					st.focusKeyboard()
					gtx.Execute(op.InvalidateCmd{})
				}
			case sshModalFocusActions:
				if ui.activateSSHModalAction(gtx, st, st.actionFocus) {
					return layout.Dimensions{}
				}
				gtx.Execute(op.InvalidateCmd{})
			default:
				if ui.activateSSHModalAction(gtx, st, st.defaultAction()) {
					return layout.Dimensions{}
				}
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}

	for _, ed := range []*widget.Editor{&st.hostEdit, &st.portEdit, &st.userEdit, &st.passEdit, &st.keyPathEdit, &st.keyPassEdit} {
		for {
			ev, ok := ed.Update(gtx)
			if !ok {
				break
			}
			switch e := ev.(type) {
			case widget.SubmitEvent:
				ed.SetText(e.Text)
				if ui.activateSSHModalAction(gtx, st, st.defaultAction()) {
					return layout.Dimensions{}
				}
				gtx.Execute(op.InvalidateCmd{})
			case widget.ChangeEvent:
				st.errText = ""
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}

	for st.backdropClick.Clicked(gtx) {
	}
	if st.closeClick.Clicked(gtx) {
		ui.closeSSHModal()
		return layout.Dimensions{}
	}
	if st.cancelClick.Clicked(gtx) {
		if ui.activateSSHModalAction(gtx, st, sshModalActionCancel) {
			return layout.Dimensions{}
		}
	}
	if st.saveClick.Clicked(gtx) {
		if ui.activateSSHModalAction(gtx, st, sshModalActionSave) {
			return layout.Dimensions{}
		}
	}
	if st.connectClick.Clicked(gtx) {
		if ui.activateSSHModalAction(gtx, st, sshModalActionConnect) {
			return layout.Dimensions{}
		}
	}
	if st.addClick.Clicked(gtx) {
		if st.addSetup() {
			st.focus = sshModalFocusSetupsList
			st.actionFocus = st.defaultAction()
		}
		st.focusKeyboard()
		gtx.Execute(op.InvalidateCmd{})
	}

	st.ensureSetupClicks(len(st.setups))
	st.ensureSetupRemoveClicks(len(st.setups))
	removed := map[int]struct{}{}
	for i := range st.setupRemoveClicks {
		if !st.setupRemoveClicks[i].Clicked(gtx) {
			continue
		}
		removed[i] = struct{}{}
		st.removeSetup(i)
		if len(st.setups) > 0 {
			st.focus = sshModalFocusSetupsList
		} else {
			st.focus = sshModalFocusAdd
		}
		st.actionFocus = st.defaultAction()
		st.focusKeyboard()
		gtx.Execute(op.InvalidateCmd{})
		break
	}
	for i := range st.setupClicks {
		if _, ok := removed[i]; ok {
			continue
		}
		if st.setupClicks[i].Clicked(gtx) {
			if st.setSelected(i) {
				gtx.Execute(op.InvalidateCmd{})
			}
			st.focus = sshModalFocusSetupsList
			st.actionFocus = st.defaultAction()
			st.focusKeyboard()
		}
	}

	return st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{A: 140}, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

		width := gtx.Dp(unit.Dp(760))
		height := gtx.Dp(unit.Dp(240))
		maxW := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(20))
		maxH := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(20))
		if width > maxW {
			width = maxW
		}
		if height > maxH {
			height = maxH
		}
		if width < 560 {
			width = 560
		}
		if height < 210 {
			height = 210
		}

		m := op.Record(gtx.Ops)
		card := fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
				return fillRoundedBox(
					gtx,
					gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
					color.NRGBA{R: 20, G: 20, B: 20, A: 252},
					color.NRGBA{R: 255, G: 255, B: 255, A: 18},
					func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSSHModalHeader(th, gtx, st)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSSHModalBody(th, gtx, st)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSSHModalFooter(th, gtx, st)
								}),
							)
						})
					},
				)
			})
		})
		call := m.Stop()

		x := (gtx.Constraints.Max.X - card.Size.X) / 2
		y := (gtx.Constraints.Max.Y - card.Size.Y) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
}

func (ui *UI) layoutSSHModalHeader(th *material.Theme, gtx layout.Context, st *sshModalState) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body1(th, "SSH Sessions")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.Font.Weight = font.Bold
			lbl.TextSize = scaleModalThemeFontSize(th, 12)
			lbl.Color = txtColor
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutTinyIconModeButton(th, gtx, &st.closeClick, uitheme.CloseIcon(), false)
		}),
	)
}

func (ui *UI) layoutSSHModalBody(th *material.Theme, gtx layout.Context, st *sshModalState) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(250)), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSSHSetupsList(th, gtx, st)
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSSHSetupForm(th, gtx, st)
		}),
	)
}

func (ui *UI) layoutSSHSetupsList(th *material.Theme, gtx layout.Context, st *sshModalState) layout.Dimensions {
	visibleFocus := st.visibleFocus()
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 24, G: 24, B: 24, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(5)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Caption(th, "Saved setups")
								lbl.Font.Typeface = ui.mainTypeface()
								lbl.TextSize = scaleModalThemeFontSize(th, 9)
								lbl.Color = hintColor
								return lbl.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSSHAddButton(th, gtx, &st.addClick, visibleFocus == sshModalFocusAdd)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if len(st.setups) == 0 {
							lbl := material.Body2(th, "No setups yet. Press + to add one.")
							lbl.Font.Typeface = ui.mainTypeface()
							lbl.TextSize = scaleModalThemeFontSize(th, 10)
							lbl.Color = hintColor
							lbl.MaxLines = 3
							return lbl.Layout(gtx)
						}
						return st.setupList.Layout(gtx, len(st.setups), func(gtx layout.Context, index int) layout.Dimensions {
							return ui.layoutSSHSetupRow(th, gtx, st, index)
						})
					}),
				)
			})
		},
	)
}

func (ui *UI) layoutSSHAddButton(th *material.Theme, gtx layout.Context, c *widget.Clickable, focused bool) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
		border := color.NRGBA{R: 255, G: 255, B: 255, A: 18}
		fg := color.NRGBA{R: 232, G: 232, B: 232, A: 255}
		if c.Hovered() {
			bg = color.NRGBA{R: 34, G: 34, B: 34, A: 255}
			border = color.NRGBA{R: 255, G: 255, B: 255, A: 30}
		}
		if c.Pressed() {
			bg = color.NRGBA{R: 44, G: 44, B: 44, A: 255}
		}
		if focused {
			bg = mixNRGBA(bg, color.NRGBA{R: 42, G: 54, B: 80, A: 255}, 0.6)
			border = mixNRGBA(border, color.NRGBA{R: 150, G: 180, B: 255, A: 255}, 0.72)
			border.A = 168
			fg = mixNRGBA(fg, color.NRGBA{R: 244, G: 248, B: 255, A: 255}, 0.4)
		}
		return fillRoundedBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneControlCornerDp)),
			bg,
			border,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body1(th, "+")
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.Font.Weight = font.Bold
					lbl.TextSize = scaleModalThemeFontSize(th, 12)
					lbl.Color = fg
					return lbl.Layout(gtx)
				})
			},
		)
	})
}

func (ui *UI) layoutSSHSetupRow(th *material.Theme, gtx layout.Context, st *sshModalState, index int) layout.Dimensions {
	setup := st.setups[index]
	label := sshSetupIdentity(setup)
	visibleFocus := st.visibleFocus()

	active := index == st.selected
	bg := color.NRGBA{R: 24, G: 24, B: 24, A: 240}
	bd := color.NRGBA{R: 255, G: 255, B: 255, A: 14}
	if active {
		bg = color.NRGBA{R: 40, G: 40, B: 40, A: 255}
		bd = color.NRGBA{R: 255, G: 255, B: 255, A: 42}
	} else if st.setupClicks[index].Hovered() {
		bg = color.NRGBA{R: 32, G: 32, B: 32, A: 255}
	}
	if active && visibleFocus == sshModalFocusSetupsList {
		bg = mixNRGBA(bg, color.NRGBA{R: 42, G: 54, B: 80, A: 255}, 0.35)
		bd = mixNRGBA(bd, color.NRGBA{R: 150, G: 180, B: 255, A: 255}, 0.72)
		bd.A = 168
	}

	return layout.Inset{Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneControlCornerDp)),
			bg,
			bd,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(4), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return st.setupClicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, label)
								lbl.Font.Typeface = ui.mainTypeface()
								lbl.TextSize = scaleModalThemeFontSize(th, 10)
								lbl.Font.Weight = font.Medium
								lbl.Color = txtColor
								lbl.MaxLines = 1
								lbl.Truncator = "..."
								return lbl.Layout(gtx)
							})
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutTinyIconModeButtonState(gtx, &st.setupRemoveClicks[index], uitheme.CloseIcon(), false, active && visibleFocus == sshModalFocusRemove)
						}),
					)
				})
			},
		)
	})
}

func (ui *UI) layoutSSHSetupForm(th *material.Theme, gtx layout.Context, st *sshModalState) layout.Dimensions {
	identity, _ := st.currentEditorSetup()
	identityLabel := sshSetupIdentity(identity)
	visibleFocus := st.visibleFocus()
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 24, G: 24, B: 24, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(5)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "Setup details")
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleModalThemeFontSize(th, 9)
						lbl.Color = hintColor
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, identityLabel)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.Font.Weight = font.Medium
						lbl.TextSize = scaleModalThemeFontSize(th, 10)
						lbl.Color = color.NRGBA{R: 220, G: 220, B: 220, A: 255}
						lbl.MaxLines = 1
						lbl.Truncator = "..."
						return lbl.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSSHField(th, gtx, "IP / Host", &st.hostEdit, "example.com", true, visibleFocus == sshModalFocusHost)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, gtx.Dp(unit.Dp(72)), func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSSHField(th, gtx, "Port", &st.portEdit, "22", true, visibleFocus == sshModalFocusPort)
								})
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSSHField(th, gtx, "User", &st.userEdit, "root", true, visibleFocus == sshModalFocusUser)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSSHField(th, gtx, "Password", &st.passEdit, "optional", true, visibleFocus == sshModalFocusPassword)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(1.3, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSSHField(th, gtx, "Key path", &st.keyPathEdit, "C:\\Users\\me\\.ssh\\id_ed25519", true, visibleFocus == sshModalFocusKeyPath)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Flexed(0.7, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSSHField(th, gtx, "Passphrase", &st.keyPassEdit, "optional", true, visibleFocus == sshModalFocusPassphrase)
							}),
						)
					}),
				)
			})
		},
	)
}

func (ui *UI) layoutSSHField(th *material.Theme, gtx layout.Context, label string, edState *widget.Editor, hint string, enabled, focused bool) layout.Dimensions {
	rowLabel := material.Caption(th, label)
	rowLabel.Font.Typeface = ui.mainTypeface()
	rowLabel.TextSize = scaleModalThemeFontSize(th, 9)
	rowLabel.Color = hintColor

	edState.ReadOnly = !enabled
	ed := material.Editor(th, edState, hint)
	ed.Font.Typeface = ui.mainTypeface()
	ed.TextSize = scaleModalThemeFontSize(th, 10)
	ed.Color = txtColor
	ed.HintColor = hintColor
	if !enabled {
		ed.Color = color.NRGBA{R: 132, G: 132, B: 132, A: 255}
		ed.HintColor = color.NRGBA{R: 98, G: 98, B: 98, A: 255}
	}
	menuID := "ssh-" + strings.ToLower(strings.ReplaceAll(label, " ", "-"))

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(rowLabel.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutEditorWithContextMenu(th, gtx, menuID, edState, enabled, func(gtx layout.Context) layout.Dimensions {
				return layoutNeutralEditorBox(gtx, focused || gtx.Focused(edState), enabled, ed.Layout)
			})
		}),
	)
}

func (ui *UI) layoutSSHModalFooter(th *material.Theme, gtx layout.Context, st *sshModalState) layout.Dimensions {
	hoverFooterKey := ""
	if st.cancelClick.Hovered() {
		hoverFooterKey = "cancel"
	}
	if st.saveClick.Hovered() {
		hoverFooterKey = "save"
	}
	if st.connectClick.Hovered() {
		hoverFooterKey = "connect"
	}
	st.footerAnim.setHover(hoverFooterKey, gtx.Now)
	hoverCancel, hoverAnimCancel := st.footerAnim.hoverFill(gtx.Now, "cancel")
	hoverSave, hoverAnimSave := st.footerAnim.hoverFill(gtx.Now, "save")
	hoverConnect, hoverAnimConnect := st.footerAnim.hoverFill(gtx.Now, "connect")
	pulseCancel, pulseAnimCancel := st.footerAnim.pulseFill(gtx.Now, "cancel")
	pulseSave, pulseAnimSave := st.footerAnim.pulseFill(gtx.Now, "save")
	pulseConnect, pulseAnimConnect := st.footerAnim.pulseFill(gtx.Now, "connect")
	cancelVisual := st.actionVisualState(sshModalActionCancel)
	saveVisual := st.actionVisualState(sshModalActionSave)
	connectVisual := st.actionVisualState(sshModalActionConnect)
	if hoverAnimCancel || hoverAnimSave || hoverAnimConnect || pulseAnimCancel || pulseAnimSave || pulseAnimConnect {
		gtx.Execute(op.InvalidateCmd{})
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if st.errText == "" {
				return layout.Dimensions{}
			}
			lbl := material.Caption(th, st.errText)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, 9)
			lbl.Color = color.NRGBA{R: 255, G: 170, B: 170, A: 255}
			lbl.MaxLines = 2
			lbl.Truncator = "..."
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, ui.sshFooterActionWidthPx(th, gtx), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutDialogActionTripleState(
					th,
					gtx,
					&st.cancelClick,
					"Cancel",
					hoverCancel,
					pulseCancel,
					false,
					&st.saveClick,
					"Save",
					hoverSave,
					pulseSave,
					false,
					&st.connectClick,
					"Connect",
					hoverConnect,
					pulseConnect,
					false,
					cancelVisual,
					saveVisual,
					connectVisual,
				)
			})
		}),
	)
}

func (ui *UI) sshFooterActionWidthPx(th *material.Theme, gtx layout.Context) int {
	leftW, _ := ui.dialogActionSegmentMetricsPx(th, gtx, "Cancel")
	middleW, _ := ui.dialogActionSegmentMetricsPx(th, gtx, "Save")
	rightW, _ := ui.dialogActionSegmentMetricsPx(th, gtx, "Connect")
	sepW := gtx.Dp(unit.Dp(1))
	if sepW < 1 {
		sepW = 1
	}
	inset := gtx.Dp(unit.Dp(1)) * 2
	return leftW + middleW + rightW + sepW*2 + inset
}
