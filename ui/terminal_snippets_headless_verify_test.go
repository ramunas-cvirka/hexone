// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build snippetverify

package ui

import (
	"hexone/fm"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestHeadlessTerminalSnippets(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cfg := fm.DefaultConfig()
	ctx := localTerminalSnippetContext(dir)
	cfg.TerminalSnippets = []fm.TerminalSnippet{
		{Name: "Run all tests", Command: "go test ./...", Scope: fm.TerminalSnippetScopeRepository, Context: ctx.repository},
		{Name: "List this folder", Command: "ls -la", Scope: fm.TerminalSnippetScopeDirectory, Context: ctx.directory},
		{Name: "Show disk usage", Command: "du -sh .", Scope: fm.TerminalSnippetScopeGlobal},
	}
	ui := NewUI(cfg)
	ui.terminal = newTerminalSession(nil)
	ui.terminal.startDir = dir
	ui.terminal.setActive(true)
	ui.terminal.startAttempted = true
	ui.terminalTabs.sessions = []*terminalSession{ui.terminal}
	if _, err := ui.terminal.term.Write([]byte("~/go/src/gpstrack-go gp-521-onboard-naviset *4 ❮git checkout master")); err != nil {
		t.Fatal(err)
	}

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	router := new(input.Router)
	render := func(name string) {
		t.Helper()
		const width, height = 1000, 700
		win, err := headless.NewWindow(width, height)
		if err != nil {
			t.Fatal(err)
		}
		defer win.Release()
		base := time.Now()
		var screenshot *image.RGBA
		for frame := 0; frame < 5; frame++ {
			var ops op.Ops
			gtx := layout.Context{
				Ops:         &ops,
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Constraints: layout.Exact(image.Pt(width, height)),
				Now:         base.Add(time.Duration(frame) * 90 * time.Millisecond),
				Source:      router.Source(),
			}
			ui.Layout(th, gtx)
			router.Frame(&ops)
			if err := win.Frame(&ops); err != nil {
				t.Fatal(err)
			}
			screenshot = image.NewRGBA(image.Rect(0, 0, width, height))
			if err := win.Screenshot(screenshot); err != nil {
				t.Fatal(err)
			}
		}
		file, err := os.Create(filepath.Join(outDir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, screenshot); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}

	ui.terminalSnippetMenuOpen = true
	ui.terminalSnippetMenuOpenedAt = time.Now()
	render("terminal-snippets-menu")
	ui.terminalSnippetMenuSelected = -1
	router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(850, 282)})
	render("terminal-snippets-hover")
	ui.openTerminalSnippetEditor()
	if got, want := ui.terminalSnippetEditor.commandEdit.Text(), "git checkout master"; got != want {
		t.Fatalf("snippet editor command=%q want %q", got, want)
	}
	router = new(input.Router)
	render("terminal-snippets-editor")
}
