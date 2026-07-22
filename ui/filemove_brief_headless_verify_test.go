// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build uiverify

package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/widget/material"
	"hexone/fm"
	"hexone/ui/widget/table"
)

func TestHeadlessFileMoveKeepsBriefViewportsStable(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	createFileOpRows(t, srcDir, 120)
	createFileOpRows(t, dstDir, 120)

	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	waitForPaneLoads(t, ui, ui.filePanes...)
	for i, pane := range ui.filePanes {
		pane.table.SetMode(table.ModeBrief)
		dir := srcDir
		if i == 1 {
			dir = dstDir
		}
		ui.requestPaneLoadWithSelection(i, dir, "", "", 0)
	}
	waitForPaneLoads(t, ui, ui.filePanes...)
	renderHeadlessFileOperation(t, th, ui, filepath.Join(outDir, "brief-move-layout.png"))

	srcPane := ui.filePanes[0]
	dstPane := ui.filePanes[1]
	srcPane.table.List.Position.First = 2
	dstPane.table.List.Position.First = 3
	renderHeadlessFileOperation(t, th, ui, filepath.Join(outDir, "brief-move-before.png"))

	srcRow := srcPane.table.FirstVisibleRow() + 1
	dstRow := dstPane.table.FirstVisibleRow() + 1
	srcPane.table.SetSelected(srcRow, srcPane.model.Len(), false)
	dstPane.table.SetSelected(dstRow, dstPane.model.Len(), false)
	srcPath := srcPane.selectedEntry().Path
	dstSelectedPath := dstPane.selectedEntry().Path
	srcPos := sanitizePaneListPosition(srcPane.table.List.Position)
	dstPos := sanitizePaneListPosition(dstPane.table.List.Position)
	ui.setActiveFilePane(0)

	dstPath := filepath.Join(dstDir, "aaa-moved.txt")
	if err := os.Rename(srcPath, dstPath); err != nil {
		t.Fatalf("move test file: %v", err)
	}
	ui.fileMove = &fileMoveState{
		pane: 0,
		row:  srcRow,
		sources: []fileMoveSource{{
			Path: srcPath,
			Name: filepath.Base(srcPath),
		}},
		dstPath: dstPath,
	}
	ui.finishFileMove(time.Now())
	waitForPaneLoads(t, ui, srcPane, dstPane)
	renderHeadlessFileOperation(t, th, ui, filepath.Join(outDir, "brief-move-after.png"))

	if got := sanitizePaneListPosition(srcPane.table.List.Position); got.First != srcPos.First || got.Offset != srcPos.Offset {
		t.Fatalf("source brief position = %+v, want first=%d offset=%d", got, srcPos.First, srcPos.Offset)
	}
	if got := sanitizePaneListPosition(dstPane.table.List.Position); got.First != dstPos.First || got.Offset != dstPos.Offset {
		t.Fatalf("destination brief position = %+v, want first=%d offset=%d", got, dstPos.First, dstPos.Offset)
	}
	if got := dstPane.selectedEntry(); got == nil || got.Path != dstSelectedPath {
		t.Fatalf("destination selection = %+v, want %q", got, dstSelectedPath)
	}
	if ui.activeFilePane != 0 {
		t.Fatalf("active file pane = %d, want 0", ui.activeFilePane)
	}
}
