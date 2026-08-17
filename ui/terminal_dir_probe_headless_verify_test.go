// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build terminaldirprobeverify

package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func TestHeadlessTerminalDirProbe(t *testing.T) {
	const width, height = 920, 520
	outDir := os.Getenv("TERMINAL_DIR_PROBE_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldSSHTarget := terminalProcessSSHTarget
	oldReadDir := readDirSFTPFunc
	oldOpen := openSSHClientsFunc
	t.Cleanup(func() {
		terminalProcessSSHTarget = oldSSHTarget
		readDirSFTPFunc = oldReadDir
		openSSHClientsFunc = oldOpen
	})
	target := terminalSSHTarget{User: "root", Host: "server.test", Port: 22}
	terminalProcessSSHTarget = func(int) (terminalSSHTarget, bool) { return target, true }

	cfg := fm.DefaultConfig()
	cfg.Terminal.HeightRows = 10
	cfg.SSH.Setups = []fm.SSHSetup{{Host: target.Host, Port: target.Port, User: target.User, Password: "secret"}}
	hexUI := NewUI(cfg)
	pane := newFilePaneState(t.TempDir(), cfg)
	hexUI.filePanes = []*filePaneState{pane}
	hexUI.filePaneTabs = []filePaneTabSet{{tabs: []*filePaneState{pane}}}
	st := newTerminalSession(nil, cfg.Terminal.HeightRows)
	st.setActive(true)
	st.startAttempted = true
	st.writeOutput([]byte("root@server:~# "))
	proc := &terminalWriteProcess{}
	st.procMu.Lock()
	st.pty = proc
	st.running = true
	st.procMu.Unlock()
	hexUI.terminal = st
	hexUI.terminalTabs.sessions = []*terminalSession{st}
	hexUI.terminalTabs.active = 0

	client := new(sftp.Client)
	openSSHClientsFunc = func(fm.SSHSetup) (sshClientBundle, error) {
		return sshClientBundle{sshClient: new(ssh.Client), sftpBase: new(ssh.Client), sftp: client}, nil
	}
	readDirSFTPFunc = func(got *sftp.Client, dir string) (filesys.Listing, error) {
		return filesys.Listing{Dir: dir}, nil
	}

	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatal(err)
	}
	defer win.Release()
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	router := new(input.Router)
	render := func(at time.Time, name string) {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Now:         at,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)),
		}
		hexUI.Layout(th, gtx)
		router.Frame(&ops)
		if err := win.Frame(&ops); err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		if err := win.Screenshot(img); err != nil {
			t.Fatal(err)
		}
		file, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, img); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}

	base := time.Now()
	if !hexUI.setPaneDirToTerminalDir(0, base) || hexUI.terminalDirProbe == nil {
		t.Fatal("terminal dir probe did not start")
	}
	render(base.Add(16*time.Millisecond), "terminal-dir-probe-querying.png")
	host := hexUI.terminalDirProbe.host
	st.writeOutput([]byte("\r\n\x1b]7;file://" + host + "/var/log/app\x07\r\nroot@server:/var/log/app# "))
	render(base.Add(32*time.Millisecond), "terminal-dir-probe-complete.png")
	if len(hexUI.filePaneTabs[0].tabs) != 2 || hexUI.filePaneTabs[0].active != 1 {
		t.Fatalf("remote sync did not create and activate a tab: %+v", hexUI.filePaneTabs[0])
	}
	synced := hexUI.filePanes[0]
	if pane.remote != nil || synced == pane || synced.remote == nil || synced.dir != "/var/log/app" {
		t.Fatalf("pane tabs not preserved and synced: local=%+v active=%+v", pane, synced)
	}
	if !hexUI.activateFilePaneTab(0, 0) {
		t.Fatal("failed to reactivate local tab")
	}
	loc := terminalOSC7Location{Host: target.Host, User: target.User, Port: target.Port, HasPort: true, Dir: "/var/log/reused"}
	if !hexUI.setPaneDirToTerminalRemoteDirForTarget(0, loc, target, base.Add(48*time.Millisecond)) {
		t.Fatal("failed to reuse inactive remote tab")
	}
	render(base.Add(64*time.Millisecond), "terminal-dir-probe-reused-tab.png")
	if len(hexUI.filePaneTabs[0].tabs) != 2 || hexUI.filePaneTabs[0].active != 1 || hexUI.filePanes[0] != synced || synced.dir != "/var/log/reused" {
		t.Fatalf("inactive remote tab was not reused: %+v", hexUI.filePaneTabs[0])
	}
	t.Logf("wrote terminal dir probe frames to %s", outDir)
}
