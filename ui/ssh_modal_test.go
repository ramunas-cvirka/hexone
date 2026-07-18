// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/filesys"
	"image"
	"path/filepath"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"hexone/fm"
)

func newSSHModalTestLayout(ui *UI) (*material.Theme, *input.Router, layout.Context, func(), func()) {
	th := material.NewTheme()
	router := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(1024, 720),
		},
	}

	layoutModal := func() {
		gtx.Ops.Reset()
		ui.layoutSSHModal(th, gtx)
	}
	frame := func() {
		layoutModal()
		router.Frame(gtx.Ops)
	}
	return th, router, gtx, layoutModal, frame
}

func TestSSHModalFocusedEditorKeepsPasteShortcut(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSSHModal()
	if ui.sshModal == nil {
		t.Fatal("ssh modal should be open")
	}
	st := ui.sshModal
	_, router, gtx, layoutModal, frame := newSSHModalTestLayout(ui)

	frame()
	gtx.Execute(key.FocusCmd{Tag: &st.hostEdit})
	frame()
	frame()
	frame()
	if !gtx.Focused(&st.hostEdit) {
		t.Fatal("host editor did not gain focus")
	}

	router.Queue(key.Event{Name: "V", Modifiers: key.ModShortcut, State: key.Press})
	layoutModal()
	if !router.ClipboardRequested() {
		t.Fatal("Ctrl/Cmd+V should reach the focused SSH editor")
	}
}

func TestSSHModalSecretsAreNotMasked(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSSHModal()
	if ui.sshModal == nil {
		t.Fatal("ssh modal should be open")
	}
	st := ui.sshModal
	if st.passEdit.Mask != 0 {
		t.Fatalf("password mask=%q want none", st.passEdit.Mask)
	}
	if st.keyPassEdit.Mask != 0 {
		t.Fatalf("key passphrase mask=%q want none", st.keyPassEdit.Mask)
	}
}

func TestSSHModalFlatAddButtonUsesTabControlGeometry(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	var click widget.Clickable
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(100, 100),
		},
	}

	if got, want := ui.layoutSSHAddButton(th, gtx, &click, false).Size, image.Pt(22, 20); got != want {
		t.Fatalf("SSH add button size=%v want %v", got, want)
	}
}

func TestSSHModalFooterClaimsFullWidthForRightAlignedActions(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(500, 60),
		},
	}

	if got, want := ui.layoutSSHModalFooter(th, gtx, &sshModalState{}).Size.X, 500; got != want {
		t.Fatalf("SSH footer width=%d want %d", got, want)
	}
}

func TestSSHModalArrowKeysStepSelectedSetup(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.SSH.Setups = []fm.SSHSetup{
		{Host: "one.test", Port: 22, User: "alice"},
		{Host: "two.test", Port: 2200, User: "bob"},
	}
	ui := NewUI(cfg)
	ui.openSSHModal()
	if ui.sshModal == nil {
		t.Fatal("ssh modal should be open")
	}
	st := ui.sshModal
	_, router, _, layoutModal, frame := newSSHModalTestLayout(ui)

	frame()
	frame()

	router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	layoutModal()
	if st.selected != 1 {
		t.Fatalf("selected=%d want 1 after Down", st.selected)
	}
	if got := st.hostEdit.Text(); got != "two.test" {
		t.Fatalf("hostEdit=%q want %q", got, "two.test")
	}

	router.Queue(key.Event{Name: key.NameUpArrow, State: key.Press})
	layoutModal()
	if st.selected != 0 {
		t.Fatalf("selected=%d want 0 after Up", st.selected)
	}
	if got := st.hostEdit.Text(); got != "one.test" {
		t.Fatalf("hostEdit=%q want %q", got, "one.test")
	}
}

func TestSSHModalTabOrderCoversRemoveFieldsAddAndActions(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.SSH.Setups = []fm.SSHSetup{
		{Host: "one.test", Port: 22, User: "alice", Password: "secret"},
	}
	ui := NewUI(cfg)
	ui.openSSHModal()
	if ui.sshModal == nil {
		t.Fatal("ssh modal should be open")
	}
	st := ui.sshModal
	_, router, _, _, frame := newSSHModalTestLayout(ui)

	pressTab := func(mods key.Modifiers) {
		router.Queue(key.Event{Name: key.NameTab, Modifiers: mods, State: key.Press})
		frame()
	}

	frame()
	frame()

	if st.focus != sshModalFocusSetupsList {
		t.Fatalf("initial focus=%v want %v", st.focus, sshModalFocusSetupsList)
	}
	pressTab(key.ModShift)
	if st.focus != sshModalFocusAdd {
		t.Fatalf("focus after initial shift+tab = %v, want %v", st.focus, sshModalFocusAdd)
	}
	pressTab(0)
	if st.focus != sshModalFocusSetupsList {
		t.Fatalf("focus after tab back from add = %v, want %v", st.focus, sshModalFocusSetupsList)
	}
	if got := st.defaultAction(); got != sshModalActionConnect {
		t.Fatalf("defaultAction=%v want %v", got, sshModalActionConnect)
	}

	wantOrder := []sshModalFocus{
		sshModalFocusRemove,
		sshModalFocusHost,
		sshModalFocusPort,
		sshModalFocusUser,
		sshModalFocusPassword,
		sshModalFocusKeyPath,
		sshModalFocusPassphrase,
		sshModalFocusActions,
		sshModalFocusAdd,
		sshModalFocusSetupsList,
	}
	for i, want := range wantOrder {
		pressTab(0)
		if st.focus != want {
			t.Fatalf("focus after tab %d = %v, want %v", i+1, st.focus, want)
		}
	}

	pressTab(0)
	if st.focus != sshModalFocusRemove {
		t.Fatalf("focus after wrap tab = %v, want %v", st.focus, sshModalFocusRemove)
	}

	pressTab(key.ModShift)
	if st.focus != sshModalFocusSetupsList {
		t.Fatalf("focus after shift+tab = %v, want %v", st.focus, sshModalFocusSetupsList)
	}
}

func TestSSHModalArrowFromRemoveReturnsToListNavigation(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.SSH.Setups = []fm.SSHSetup{
		{Host: "one.test", Port: 22, User: "alice"},
		{Host: "two.test", Port: 2200, User: "bob"},
	}
	ui := NewUI(cfg)
	ui.openSSHModal()
	if ui.sshModal == nil {
		t.Fatal("ssh modal should be open")
	}
	st := ui.sshModal
	_, router, _, _, frame := newSSHModalTestLayout(ui)

	frame()
	frame()

	router.Queue(key.Event{Name: key.NameTab, State: key.Press})
	frame()

	if st.focus != sshModalFocusRemove {
		t.Fatalf("focus=%v want %v before Down", st.focus, sshModalFocusRemove)
	}

	router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	frame()

	if st.focus != sshModalFocusSetupsList {
		t.Fatalf("focus=%v want %v after Down", st.focus, sshModalFocusSetupsList)
	}
	if st.selected != 1 {
		t.Fatalf("selected=%d want 1 after Down from remove", st.selected)
	}
}

func TestSSHModalEnterConnectsSelectedSetup(t *testing.T) {
	oldReadDir := readDirSFTPFunc
	oldOpen := openSSHClientsFunc
	oldGetwd := sftpGetwdFunc
	oldCloseSFTP := closeSFTPClientFunc
	oldCloseSSH := closeSSHClientFunc
	t.Cleanup(func() {
		readDirSFTPFunc = oldReadDir
		openSSHClientsFunc = oldOpen
		sftpGetwdFunc = oldGetwd
		closeSFTPClientFunc = oldCloseSFTP
		closeSSHClientFunc = oldCloseSSH
	})

	closeSFTPClientFunc = func(*sftp.Client) {}
	closeSSHClientFunc = func(*ssh.Client) {}

	cfg := fm.DefaultConfig()
	cfg.SSH.Setups = []fm.SSHSetup{
		{Host: "srv.test", Port: 2222, User: "ramunas", Password: "secret"},
	}
	ui := NewUI(cfg)
	ui.openSSHModal()
	if ui.sshModal == nil {
		t.Fatal("ssh modal should be open")
	}

	remoteClient := new(sftp.Client)
	openSSHClientsFunc = func(got fm.SSHSetup) (sshClientBundle, error) {
		if got.Host != "srv.test" || got.Port != 2222 || got.User != "ramunas" {
			t.Fatalf("unexpected connect setup: %+v", got)
		}
		return sshClientBundle{
			sshClient: new(ssh.Client),
			sftpBase:  new(ssh.Client),
			sftp:      remoteClient,
		}, nil
	}
	sftpGetwdFunc = func(client *sftp.Client) (string, error) {
		if client != remoteClient {
			t.Fatalf("unexpected sftp client %p", client)
		}
		return "/home/ramunas", nil
	}
	readDirSFTPFunc = func(client *sftp.Client, dir string) (filesys.Listing, error) {
		if client != remoteClient {
			t.Fatalf("unexpected readDir client %p", client)
		}
		return filesys.Listing{Dir: dir}, nil
	}

	_, router, _, layoutModal, frame := newSSHModalTestLayout(ui)
	frame()
	frame()

	router.Queue(key.Event{Name: key.NameReturn, State: key.Press})
	layoutModal()

	if ui.sshModal != nil {
		t.Fatalf("ssh modal should close after successful Enter connect; err=%q", ui.sshModal.errText)
	}
	pane := ui.activePane()
	if pane == nil || pane.remote == nil {
		t.Fatal("active pane should be connected after Enter connect")
	}
	if pane.remote.setup.Host != "srv.test" || pane.remote.setup.User != "ramunas" || pane.remote.setup.Port != 2222 {
		t.Fatalf("connected setup = %+v", pane.remote.setup)
	}
	if pane.dir != "/home/ramunas" {
		t.Fatalf("pane dir=%q want %q", pane.dir, "/home/ramunas")
	}
}

func TestSSHModalSaveKeepsSelectedSetup(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.SSH.Setups = []fm.SSHSetup{
		{Host: "one.test", Port: 22, User: "alice"},
	}
	ui := NewUI(cfg)
	ui.sshCredentials.store = newMemorySecretStore()
	ui.configPath = filepath.Join(t.TempDir(), "hexone-config.yaml")
	ui.openSSHModal()
	if ui.sshModal == nil {
		t.Fatal("ssh modal should be open")
	}
	st := ui.sshModal

	st.setups = append(st.setups, fm.SSHSetup{Port: 22})
	st.selected = len(st.setups) - 1
	st.loadEditorsFromSelected()
	st.hostEdit.SetText("two.test")
	st.portEdit.SetText("2200")
	st.userEdit.SetText("bob")

	if err := ui.saveSSHModal(); err != nil {
		t.Fatalf("saveSSHModal: %v", err)
	}
	if st.selected != 1 {
		t.Fatalf("selected=%d want 1 after save", st.selected)
	}
	if len(st.setups) != 2 {
		t.Fatalf("len(setups)=%d want 2", len(st.setups))
	}
	if got := st.hostEdit.Text(); got != "two.test" {
		t.Fatalf("hostEdit=%q want %q after save", got, "two.test")
	}
	if got := sshSetupIdentity(st.setups[st.selected]); got != "bob@two.test:2200" {
		t.Fatalf("selected identity=%q want %q", got, "bob@two.test:2200")
	}
}

func TestSSHModalDirtyDraftDefaultsToSaveAndEnterSaves(t *testing.T) {
	oldOpen := openSSHClientsFunc
	t.Cleanup(func() {
		openSSHClientsFunc = oldOpen
	})

	openSSHClientsFunc = func(fm.SSHSetup) (sshClientBundle, error) {
		t.Fatal("dirty Enter should save instead of connecting")
		return sshClientBundle{}, nil
	}

	cfg := fm.DefaultConfig()
	cfg.SSH.Setups = []fm.SSHSetup{
		{Host: "one.test", Port: 22, User: "alice", Password: "secret"},
	}
	ui := NewUI(cfg)
	ui.sshCredentials.store = newMemorySecretStore()
	ui.configPath = filepath.Join(t.TempDir(), "hexone-config.yaml")
	ui.openSSHModal()
	if ui.sshModal == nil {
		t.Fatal("ssh modal should be open")
	}
	st := ui.sshModal
	_, router, gtx, _, frame := newSSHModalTestLayout(ui)

	frame()
	frame()

	if got := st.defaultAction(); got != sshModalActionConnect {
		t.Fatalf("defaultAction=%v want %v before edit", got, sshModalActionConnect)
	}
	if got := st.saveLabel(); got != "Save" {
		t.Fatalf("clean save label=%q want %q", got, "Save")
	}

	st.hostEdit.SetText("two.test")
	if !st.hasUnsavedChanges() {
		t.Fatal("host edit should mark SSH modal dirty")
	}
	if got := st.defaultAction(); got != sshModalActionSave {
		t.Fatalf("defaultAction=%v want %v after edit", got, sshModalActionSave)
	}
	if got := st.saveLabel(); got != "Save (*)" {
		t.Fatalf("dirty save label=%q want %q", got, "Save (*)")
	}

	gtx.Execute(key.FocusCmd{Tag: &st.hostEdit})
	frame()
	frame()
	if !gtx.Focused(&st.hostEdit) {
		t.Fatal("host editor did not gain focus")
	}

	router.Queue(key.Event{Name: key.NameReturn, State: key.Press})
	frame()

	if ui.sshModal == nil {
		t.Fatal("ssh modal should remain open after save")
	}
	if got := ui.fmCfg.SSH.Setups[0].Host; got != "two.test" {
		t.Fatalf("saved host=%q want %q", got, "two.test")
	}
	if got := st.defaultAction(); got != sshModalActionConnect {
		t.Fatalf("defaultAction=%v want %v after save", got, sshModalActionConnect)
	}
	if got := st.saveLabel(); got != "Save" {
		t.Fatalf("saved label=%q want %q", got, "Save")
	}

	if !st.addSetup() {
		t.Fatal("addSetup should append a new SSH draft row")
	}
	if got := st.defaultAction(); got != sshModalActionSave {
		t.Fatalf("defaultAction=%v want %v after add", got, sshModalActionSave)
	}
	if got := st.saveLabel(); got != "Save (*)" {
		t.Fatalf("added-draft save label=%q want %q", got, "Save (*)")
	}
}
