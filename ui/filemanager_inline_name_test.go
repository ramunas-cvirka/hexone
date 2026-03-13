// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hexone/filesys"
	"hexone/fm"
)

func TestValidateInlineFileNameTargetRejectsLocalPathSeparators(t *testing.T) {
	ep := copyEndpoint{dir: t.TempDir()}
	src := filepath.Join(ep.dir, "alpha.txt")

	if _, _, err := validateInlineFileNameTarget(ep, src, "alpha.txt", `bad\name.txt`); err == nil {
		t.Fatal("validateInlineFileNameTarget should reject local path separators")
	}
}

func TestFinishInlineFileNameEditRenamesLocalFile(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "alpha.txt")
	if err := os.WriteFile(srcPath, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	cfg := fm.DefaultConfig()
	pane := newFilePaneState(dir, cfg)
	pane.applyListing(filesys.Listing{
		Dir: dir,
		Entries: []filesys.Entry{
			{Name: "alpha.txt", DisplayName: "alpha.txt", Path: srcPath},
		},
	}, "", "", 0)

	ui := &UI{
		fmCfg:          cfg,
		filePanes:      []*filePaneState{pane},
		activeFilePane: 0,
		held:           make(map[string]bool),
	}

	now := time.Date(2026, time.March, 9, 12, 0, 0, 0, time.UTC)
	if !ui.startInlineFileNameEdit(0, 0, now) {
		t.Fatal("startInlineFileNameEdit should succeed")
	}
	pane.inlineNameEdit.SetText("beta.txt")

	if !ui.finishInlineFileNameEdit(0, now, true, false) {
		t.Fatal("finishInlineFileNameEdit should commit rename")
	}
	if pane.inlineNameEditing {
		t.Fatal("inline name editor should close after successful rename")
	}

	dstPath := filepath.Join(dir, "beta.txt")
	if _, err := os.Stat(dstPath); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Fatalf("source file still exists or unexpected stat error: %v", err)
	}
}

func TestActivatePendingInlineNameEditStartsAfterDelay(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "alpha.txt")
	if err := os.WriteFile(srcPath, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	cfg := fm.DefaultConfig()
	pane := newFilePaneState(dir, cfg)
	pane.applyListing(filesys.Listing{
		Dir: dir,
		Entries: []filesys.Entry{
			{Name: "alpha.txt", DisplayName: "alpha.txt", Path: srcPath},
		},
	}, "", "", 0)

	ui := &UI{
		fmCfg:          cfg,
		filePanes:      []*filePaneState{pane},
		activeFilePane: 0,
		held:           make(map[string]bool),
	}

	now := time.Date(2026, time.March, 9, 12, 0, 0, 0, time.UTC)
	pane.queueInlineNameEdit(0, now.Add(filePaneTableDoubleClickWindow))

	if ui.activatePendingInlineNameEdit(0, now.Add(filePaneTableDoubleClickWindow-time.Millisecond)) {
		t.Fatal("activatePendingInlineNameEdit should not start before the delay expires")
	}
	if pane.inlineNameEditing {
		t.Fatal("inline rename should stay inactive before the delay expires")
	}
	if !ui.activatePendingInlineNameEdit(0, now.Add(filePaneTableDoubleClickWindow)) {
		t.Fatal("activatePendingInlineNameEdit should start once the delay expires")
	}
	if !pane.inlineNameEditing || pane.inlineNameRow != 0 {
		t.Fatal("inline rename should target the queued row")
	}
}

func TestActivatePendingInlineNameEditCancelsWhenSelectionMoved(t *testing.T) {
	dir := t.TempDir()
	srcPathA := filepath.Join(dir, "alpha.txt")
	srcPathB := filepath.Join(dir, "beta.txt")
	if err := os.WriteFile(srcPathA, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write alpha: %v", err)
	}
	if err := os.WriteFile(srcPathB, []byte("beta"), 0o644); err != nil {
		t.Fatalf("write beta: %v", err)
	}

	cfg := fm.DefaultConfig()
	pane := newFilePaneState(dir, cfg)
	pane.applyListing(filesys.Listing{
		Dir: dir,
		Entries: []filesys.Entry{
			{Name: "alpha.txt", DisplayName: "alpha.txt", Path: srcPathA},
			{Name: "beta.txt", DisplayName: "beta.txt", Path: srcPathB},
		},
	}, "", "", 0)

	ui := &UI{
		fmCfg:          cfg,
		filePanes:      []*filePaneState{pane},
		activeFilePane: 0,
		held:           make(map[string]bool),
	}

	now := time.Date(2026, time.March, 9, 12, 0, 0, 0, time.UTC)
	pane.queueInlineNameEdit(0, now.Add(filePaneTableDoubleClickWindow))
	pane.table.SetSelected(1, pane.model.Len(), false)

	if ui.activatePendingInlineNameEdit(0, now.Add(filePaneTableDoubleClickWindow)) {
		t.Fatal("activatePendingInlineNameEdit should not start after selection moves")
	}
	if pane.inlineNameEditing {
		t.Fatal("inline rename should remain inactive after selection moves")
	}
	if pane.inlineNamePendingRow >= 0 {
		t.Fatal("pending inline rename should clear after expiration")
	}
}

func TestStartInlineFileNameEditIgnoresParentEntryWithoutNotice(t *testing.T) {
	cfg := fm.DefaultConfig()
	pane := newFilePaneState(".", cfg)
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "..", DisplayName: "..", Path: "..", Kind: filesys.EntryParent},
		},
	}, "", "", 0)

	ui := &UI{
		fmCfg:          cfg,
		filePanes:      []*filePaneState{pane},
		activeFilePane: 0,
		held:           make(map[string]bool),
	}

	now := time.Date(2026, time.March, 9, 12, 0, 0, 0, time.UTC)
	if ui.startInlineFileNameEdit(0, 0, now) {
		t.Fatal("startInlineFileNameEdit should ignore parent entry")
	}
	if pane.inlineNameEditing {
		t.Fatal("parent entry should not enter inline rename")
	}
	if pane.noticeText != "" {
		t.Fatalf("parent entry rename should not show a notice, got %q", pane.noticeText)
	}
}

func TestHandleFilePaneContextMenuRenameUsesCapturedRow(t *testing.T) {
	dir := t.TempDir()
	srcPathA := filepath.Join(dir, "alpha.txt")
	srcPathB := filepath.Join(dir, "beta.txt")
	if err := os.WriteFile(srcPathA, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write alpha: %v", err)
	}
	if err := os.WriteFile(srcPathB, []byte("beta"), 0o644); err != nil {
		t.Fatalf("write beta: %v", err)
	}

	cfg := fm.DefaultConfig()
	pane := newFilePaneState(dir, cfg)
	pane.applyListing(filesys.Listing{
		Dir: dir,
		Entries: []filesys.Entry{
			{Name: "alpha.txt", DisplayName: "alpha.txt", Path: srcPathA},
			{Name: "beta.txt", DisplayName: "beta.txt", Path: srcPathB},
		},
	}, "", "", 0)

	ui := &UI{
		fmCfg:          cfg,
		filePanes:      []*filePaneState{pane},
		activeFilePane: 0,
		held:           make(map[string]bool),
	}

	now := time.Date(2026, time.March, 9, 12, 0, 0, 0, time.UTC)
	pane.openContextMenu(1, image.Point{}, now)
	row := pane.ctxMenuRow
	pane.closeContextMenu()
	ui.handleFilePaneContextMenuAction(0, pane, row, filePaneMenuActionRename, now)

	if !pane.inlineNameEditing || pane.inlineNameRow != 1 {
		t.Fatal("context-menu rename should start editing the clicked row even after the menu closes")
	}
}
