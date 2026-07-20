// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/io/key"
)

func TestStripTerminalPrompt(t *testing.T) {
	tests := map[string]string{
		"user@host:/work$ go test ./...": "go test ./...",
		"PS C:\\src\\app> git status":    "git status",
		"❯ make release":                 "make release",
		"~/go/src/gpstrack-go gp-521-onboard-naviset *4 ❮git checkout master": "git checkout master",
		"plain command":           "plain command",
		"echo hello > output.txt": "echo hello > output.txt",
	}
	for input, want := range tests {
		if got := stripTerminalPrompt(input); got != want {
			t.Fatalf("stripTerminalPrompt(%q)=%q want %q", input, got, want)
		}
	}
}

func TestTerminalCurrentCommandDraftUsesPromptLine(t *testing.T) {
	st := newTerminalSession(nil)
	if _, err := st.term.Write([]byte("user@host:/work$ go test ./...")); err != nil {
		t.Fatal(err)
	}
	if got, want := st.currentCommandDraft(), "go test ./..."; got != want {
		t.Fatalf("currentCommandDraft()=%q want %q", got, want)
	}
}

func TestTerminalCurrentCommandDraftTracksTypedAndExecutedCommand(t *testing.T) {
	st := newTerminalSession(nil)
	proc := &terminalWriteProcess{}
	st.procMu.Lock()
	st.pty = proc
	st.running = true
	st.procMu.Unlock()

	st.writeString("git checkout mastXer")
	st.write([]byte("\x1b[D\x1b[D"))
	st.write([]byte{0x7f})
	if got, want := st.currentCommandDraft(), "git checkout master"; got != want {
		t.Fatalf("typed currentCommandDraft()=%q want %q", got, want)
	}

	st.write([]byte("\r"))
	if got, want := st.currentCommandDraft(), "git checkout master"; got != want {
		t.Fatalf("executed currentCommandDraft()=%q want %q", got, want)
	}
}

func TestTerminalCurrentCommandDraftUsesTrackedCommandBeforeCustomPrompt(t *testing.T) {
	st := newTerminalSession(nil)
	proc := &terminalWriteProcess{}
	st.procMu.Lock()
	st.pty = proc
	st.running = true
	st.procMu.Unlock()

	st.writeString("git checkout master")
	st.write([]byte("\r"))
	if _, err := st.term.Write([]byte("~/go/src/gpstrack-go gp-521-onboard-naviset *4 ❮")); err != nil {
		t.Fatal(err)
	}
	if got, want := st.currentCommandDraft(), "git checkout master"; got != want {
		t.Fatalf("currentCommandDraft()=%q want %q", got, want)
	}
}

func TestTerminalSnippetContextFindsRepository(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "cmd", "app")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := localTerminalSnippetContext(nested)
	if got, want := ctx.directory, terminalSnippetLocalKey(nested); got != want {
		t.Fatalf("directory=%q want %q", got, want)
	}
	if got, want := ctx.repository, terminalSnippetLocalKey(root); got != want {
		t.Fatalf("repository=%q want %q", got, want)
	}
}

func TestApplicableTerminalSnippetsRespectSingleScope(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	st := newTerminalSession(nil)
	st.startDir = nested
	ui := &UI{
		terminal: st,
		fmCfg:    fm.DefaultConfig(),
	}
	ctx := localTerminalSnippetContext(nested)
	ui.fmCfg.TerminalSnippets = []fm.TerminalSnippet{
		{Name: "Global", Command: "date", Scope: fm.TerminalSnippetScopeGlobal},
		{Name: "Folder", Command: "ls", Scope: fm.TerminalSnippetScopeDirectory, Context: ctx.directory},
		{Name: "Repo", Command: "go test ./...", Scope: fm.TerminalSnippetScopeRepository, Context: ctx.repository},
		{Name: "Elsewhere", Command: "pwd", Scope: fm.TerminalSnippetScopeDirectory, Context: terminalSnippetLocalKey(root)},
	}
	items := ui.applicableTerminalSnippets()
	if got, want := len(items), 3; got != want {
		t.Fatalf("applicable count=%d want %d", got, want)
	}
	if got, want := items[0].snippet.Name, "Repo"; got != want {
		t.Fatalf("first snippet=%q want %q", got, want)
	}
	if got, want := items[1].snippet.Name, "Folder"; got != want {
		t.Fatalf("second snippet=%q want %q", got, want)
	}
	if got, want := items[2].snippet.Name, "Global"; got != want {
		t.Fatalf("third snippet=%q want %q", got, want)
	}
}

func TestInsertTerminalSnippetDoesNotExecute(t *testing.T) {
	st := newTerminalSession(nil)
	proc := &terminalWriteProcess{}
	st.procMu.Lock()
	st.pty = proc
	st.running = true
	st.procMu.Unlock()
	ui := &UI{terminal: st}
	if !ui.insertTerminalSnippet(fm.TerminalSnippet{Name: "Tests", Command: "go test ./...", Scope: fm.TerminalSnippetScopeGlobal}) {
		t.Fatal("insertTerminalSnippet returned false")
	}
	if got, want := proc.String(), "go test ./..."; got != want {
		t.Fatalf("terminal write=%q want %q", got, want)
	}
}

func TestTerminalSnippetMenuKeyboardSelectionInserts(t *testing.T) {
	st := newTerminalSession(nil)
	proc := &terminalWriteProcess{}
	st.procMu.Lock()
	st.pty = proc
	st.running = true
	st.procMu.Unlock()
	ui := &UI{
		terminal:                    st,
		fmCfg:                       fm.DefaultConfig(),
		terminalSnippetMenuOpen:     true,
		terminalSnippetMenuSelected: 0,
	}
	ui.fmCfg.TerminalSnippets = []fm.TerminalSnippet{{
		Name:    "Tests",
		Command: "go test ./...",
		Scope:   fm.TerminalSnippetScopeGlobal,
	}}
	gtx, router := testKeyContext()
	router.Event(key.Filter{Name: key.NameDownArrow})
	router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	ui.handleTerminalSnippetMenuKeys(gtx)
	if got, want := ui.terminalSnippetMenuSelected, 1; got != want {
		t.Fatalf("selected=%d want %d", got, want)
	}

	router.Event(key.Filter{Name: key.NameEnter})
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	ui.handleTerminalSnippetMenuKeys(gtx)
	if got, want := proc.String(), "go test ./..."; got != want {
		t.Fatalf("terminal write=%q want %q", got, want)
	}
	if ui.terminalSnippetMenuOpen {
		t.Fatal("menu should close after insertion")
	}
}
