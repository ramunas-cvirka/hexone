// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"hexone/filesys"
	"hexone/fm"
)

func TestFinishFileCopyKeepsDestinationScrollStableAndShowsNotice(t *testing.T) {
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("src"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q): %v", srcPath, err)
	}

	dstDir := t.TempDir()
	createFileOpRows(t, dstDir, 30)

	ui := NewUI(fm.DefaultConfig())
	srcPane := ui.filePanes[0]
	dstPane := ui.filePanes[1]
	waitForPaneLoads(t, ui, srcPane, dstPane)
	prepareFileOpPane(t, srcPane, srcDir, 0)
	prepareFileOpPane(t, dstPane, dstDir, 10)

	dstPath := filepath.Join(dstDir, "aaa-copy.txt")
	if err := os.WriteFile(dstPath, []byte("copied"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q): %v", dstPath, err)
	}

	ui.fileCopy = &fileCopyState{
		srcPane: 0,
		dstPane: 1,
		sources: []fileCopySource{{
			Path: srcPath,
			Name: "source.txt",
		}},
	}

	ui.finishFileCopy(time.Now())
	waitForPaneLoads(t, ui, srcPane, dstPane)

	if got := dstPane.table.List.Position.First; got != 12 {
		t.Fatalf("destination first visible row = %d, want 12", got)
	}
	if got := ui.activeFilePane; got != 0 {
		t.Fatalf("activeFilePane = %d, want 0", got)
	}
	if got, want := dstPane.noticeText, "copied 1 item"; got != want {
		t.Fatalf("destination noticeText = %q, want %q", got, want)
	}
}

func TestFinishFileCopyShowsNestedCopyCount(t *testing.T) {
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "folder")
	srcSubDir := filepath.Join(srcPath, "sub")
	if err := os.MkdirAll(srcSubDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q): %v", srcSubDir, err)
	}
	for _, name := range []string{
		filepath.Join(srcPath, "a.txt"),
		filepath.Join(srcSubDir, "b.txt"),
	} {
		if err := os.WriteFile(name, []byte("src"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q): %v", name, err)
		}
	}

	dstDir := t.TempDir()
	createFileOpRows(t, dstDir, 30)
	dstPath := filepath.Join(dstDir, "folder")
	dstSubDir := filepath.Join(dstPath, "sub")
	if err := os.MkdirAll(dstSubDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q): %v", dstSubDir, err)
	}
	for _, name := range []string{
		filepath.Join(dstPath, "a.txt"),
		filepath.Join(dstSubDir, "b.txt"),
	} {
		if err := os.WriteFile(name, []byte("dst"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q): %v", name, err)
		}
	}

	ui := NewUI(fm.DefaultConfig())
	srcPane := ui.filePanes[0]
	dstPane := ui.filePanes[1]
	waitForPaneLoads(t, ui, srcPane, dstPane)
	prepareFileOpPane(t, srcPane, srcDir, 0)
	prepareFileOpPane(t, dstPane, dstDir, 10)

	ui.fileCopy = &fileCopyState{
		srcPane: 0,
		dstPane: 1,
		sources: []fileCopySource{{
			Path: srcPath,
			Name: "folder",
		}},
		progress: filesys.CopyProgress{
			EntriesTotal: 4,
		},
	}

	ui.finishFileCopy(time.Now())
	waitForPaneLoads(t, ui, srcPane, dstPane)

	if got, want := dstPane.noticeText, "copied 1 item (3 nested items)"; got != want {
		t.Fatalf("destination noticeText = %q, want %q", got, want)
	}
}

func TestFinishFileMoveKeepsSourceScrollStableAndShowsNotice(t *testing.T) {
	srcDir := t.TempDir()
	createFileOpRows(t, srcDir, 30)
	srcPath := filepath.Join(srcDir, fileDeleteTestName(10))

	dstDir := t.TempDir()
	ui := NewUI(fm.DefaultConfig())
	srcPane := ui.filePanes[0]
	waitForPaneLoads(t, ui, srcPane)
	prepareFileOpPane(t, srcPane, srcDir, 10)

	dstPath := filepath.Join(dstDir, fileDeleteTestName(10))
	if err := os.Rename(srcPath, dstPath); err != nil {
		t.Fatalf("os.Rename: %v", err)
	}

	ui.fileMove = &fileMoveState{
		pane: 0,
		row:  srcPane.table.Selected,
		sources: []fileMoveSource{{
			Path: srcPath,
			Name: fileDeleteTestName(10),
		}},
		dstPath: dstPath,
	}

	ui.finishFileMove(time.Now())
	waitForPaneLoads(t, ui, srcPane)

	if got := srcPane.table.List.Position.First; got != 11 {
		t.Fatalf("source first visible row = %d, want 11", got)
	}
	if got, want := srcPane.noticeText, "moved 1 item"; got != want {
		t.Fatalf("source noticeText = %q, want %q", got, want)
	}
}

func TestFinishFileCreateKeepsScrollStableAndShowsNotice(t *testing.T) {
	dir := t.TempDir()
	createFileOpRows(t, dir, 30)

	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	waitForPaneLoads(t, ui, pane)
	prepareFileOpPane(t, pane, dir, 10)

	createdPath := filepath.Join(dir, "aaa-new.txt")
	if err := os.WriteFile(createdPath, []byte("new"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q): %v", createdPath, err)
	}

	ui.fileCreate = &fileCreateState{
		pane:       0,
		targetPath: createdPath,
		kind:       fileCreateKindFile,
	}

	ui.finishFileCreate(time.Now())
	waitForPaneLoads(t, ui, pane)

	if got := pane.table.List.Position.First; got != 12 {
		t.Fatalf("first visible row = %d, want 12", got)
	}
	if got, want := pane.noticeText, "created file"; got != want {
		t.Fatalf("noticeText = %q, want %q", got, want)
	}
}

func prepareFileOpPane(t *testing.T, pane *filePaneState, dir string, firstVisible int) {
	t.Helper()
	listing, err := filesys.ReadDir(dir)
	if err != nil {
		t.Fatalf("filesys.ReadDir(%q): %v", dir, err)
	}
	selectedPath := filepath.Join(dir, fileDeleteTestName(firstVisible))
	pane.applyListing(listing, selectedPath, "", 0)
	selectedRow := pane.findEntryPathIndex(selectedPath)
	if selectedRow < 0 {
		selectedRow = 0
	}
	pane.table.List.Position.First = selectedRow
	pane.table.List.Position.Offset = 0
	pane.table.Selected = selectedRow
}

func createFileOpRows(t *testing.T, dir string, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		name := filepath.Join(dir, fileDeleteTestName(i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q): %v", name, err)
		}
	}
}
