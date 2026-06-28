// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package ui

import (
	"errors"
	"hexone/fm"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWindowsTerminalProcessStartsCmd(t *testing.T) {
	name, args := terminalCommandForShellOnGOOS("windows", "cmd", "")
	proc, err := startTerminalProcess(name, args, "", terminalEnv(os.Environ(), 8, 40), 8, 40)
	if err != nil {
		if terminalProcessUnsupported(err) {
			t.Skipf("ConPTY unsupported on this Windows host: %v", err)
		}
		t.Fatalf("startTerminalProcess: %v", err)
	}
	defer func() {
		_ = proc.Close()
		_ = proc.Kill()
	}()

	chunks := make(chan string, 8)
	readErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 512)
		for {
			n, err := proc.Read(buf)
			if n > 0 {
				chunks <- string(buf[:n])
			}
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
					err = nil
				}
				readErr <- err
				return
			}
		}
	}()
	waitErr := make(chan error, 1)
	go func() {
		waitErr <- proc.Wait()
	}()
	var out strings.Builder
	sent := false
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case chunk := <-chunks:
			out.WriteString(chunk)
			if !sent && strings.Contains(out.String(), "]7;") {
				if _, err := proc.Write([]byte("echo hexone-conpty\r\n")); err != nil {
					t.Fatalf("write ConPTY input: %v", err)
				}
				sent = true
			}
			if strings.Contains(out.String(), "hexone-conpty") {
				return
			}
		case err := <-readErr:
			if err != nil {
				t.Fatalf("read ConPTY output: %v", err)
			}
			if !strings.Contains(out.String(), "hexone-conpty") {
				t.Fatalf("ConPTY output=%q, want marker", out.String())
			}
			return
		case err := <-waitErr:
			if err != nil {
				t.Fatalf("ConPTY command exited with error: %v; output=%q", err, out.String())
			}
			if !strings.Contains(out.String(), "hexone-conpty") {
				t.Fatalf("ConPTY command exited without marker; output=%q", out.String())
			}
			return
		case <-timer.C:
			_ = proc.Close()
			_ = proc.Kill()
			t.Fatalf("timed out reading ConPTY output; partial=%q pid=%d", out.String(), proc.PID())
		}
	}
}

func TestWindowsLiveTerminalCanSwitchShellWithoutOverlappingPTYs(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Viewer.Shell = "cmd"
	ui := NewUI(cfg)
	firstDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	secondDir := filepath.Dir(firstDir)
	first := newTerminalSession(nil, 8)
	first.start(firstDir, "cmd")
	second := newTerminalSession(nil, 8)
	second.setActive(true)
	second.start(secondDir, "cmd")
	ui.terminalTabs = terminalTabSet{
		sessions: []*terminalSession{first, second},
		active:   1,
	}
	ui.terminal = second

	cfg.Viewer.Shell = "powershell"
	if !ui.applyTerminalShellRuntime() {
		t.Fatal("live shell change should replace the terminal session")
	}
	if ui.terminal == second || !first.closing || !second.closing {
		t.Fatal("old live terminal sessions were not closed")
	}
	defer ui.closeAllTerminalTabs()
	if len(ui.terminalTabs.sessions) != 2 || !ui.terminalTabs.sessions[0].running || !ui.terminalTabs.sessions[1].running {
		t.Fatal("replacement shells were not prestarted")
	}
	if got, want := filepath.Clean(ui.terminalTabs.sessions[0].startDir), filepath.Clean(firstDir); got != want {
		t.Fatalf("first replacement shell start dir=%q want preserved %q", got, want)
	}
	if got, want := filepath.Clean(ui.terminalTabs.sessions[1].startDir), filepath.Clean(secondDir); got != want {
		t.Fatalf("second replacement shell start dir=%q want preserved %q", got, want)
	}
}
