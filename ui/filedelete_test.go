// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hexone/filesys"
	"hexone/fm"
)

func TestFinishFileDeleteKeepsScrollStableAndShowsNotice(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 30; i++ {
		name := filepath.Join(root, fileDeleteTestName(i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q): %v", name, err)
		}
	}

	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	waitForPaneLoads(t, ui, pane)
	listing, err := filesys.ReadDir(root)
	if err != nil {
		t.Fatalf("filesys.ReadDir: %v", err)
	}
	pane.applyListing(listing, filepath.Join(root, fileDeleteTestName(10)), "", 10)
	pane.table.List.Position.First = 10
	pane.table.List.Position.Offset = 0
	pane.table.Selected = 10

	targetPath := filepath.Join(root, fileDeleteTestName(10))
	if err := os.Remove(targetPath); err != nil {
		t.Fatalf("os.Remove: %v", err)
	}

	ui.fileDelete = &fileDeleteState{
		pane: 0,
		row:  10,
		targets: []fileDeleteTarget{{
			Path: targetPath,
			Name: fileDeleteTestName(10),
		}},
	}

	now := time.Now()
	ui.finishFileDelete(now)
	waitForPaneLoads(t, ui, pane)

	if got := pane.table.List.Position.First; got != 10 {
		t.Fatalf("first visible row = %d, want 10", got)
	}
	if pane.noticeText != "deleted 1 item" {
		t.Fatalf("noticeText = %q, want %q", pane.noticeText, "deleted 1 item")
	}
	if got := pane.noticeUntil.Sub(pane.noticeShownAt); got != fileDeleteSuccessNoticeDur {
		t.Fatalf("notice duration = %v, want %v", got, fileDeleteSuccessNoticeDur)
	}
}

func TestFinishFileDeleteShowsNestedDeleteCount(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "folder")
	targetSubDir := filepath.Join(targetDir, "sub")
	if err := os.MkdirAll(targetSubDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll: %v", err)
	}
	for _, name := range []string{
		filepath.Join(targetDir, "a.txt"),
		filepath.Join(targetSubDir, "b.txt"),
	} {
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q): %v", name, err)
		}
	}

	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	waitForPaneLoads(t, ui, pane)
	listing, err := filesys.ReadDir(root)
	if err != nil {
		t.Fatalf("filesys.ReadDir: %v", err)
	}
	pane.applyListing(listing, targetDir, "", 0)
	selectedRow := pane.findEntryPathIndex(targetDir)
	if selectedRow < 0 {
		t.Fatalf("target row not found")
	}
	pane.table.List.Position.First = selectedRow
	pane.table.List.Position.Offset = 0
	pane.table.Selected = selectedRow

	if err := os.RemoveAll(targetDir); err != nil {
		t.Fatalf("os.RemoveAll: %v", err)
	}

	ui.fileDelete = &fileDeleteState{
		pane:               0,
		row:                selectedRow,
		deletedNestedCount: 3,
		deletedCountKnown:  true,
		targets: []fileDeleteTarget{{
			Path: targetDir,
			Name: "folder",
		}},
	}

	ui.finishFileDelete(time.Now())
	waitForPaneLoads(t, ui, pane)

	if got, want := pane.noticeText, "deleted 1 item (3 nested items)"; got != want {
		t.Fatalf("noticeText = %q, want %q", got, want)
	}
}

func TestCountDeleteNestedEntriesCountsLocalDescendants(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "folder")
	targetSubDir := filepath.Join(targetDir, "sub")
	if err := os.MkdirAll(targetSubDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll: %v", err)
	}
	for _, name := range []string{
		filepath.Join(targetDir, "a.txt"),
		filepath.Join(targetSubDir, "b.txt"),
	} {
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q): %v", name, err)
		}
	}

	got, err := countDeleteNestedEntries([]fileDeleteTarget{{
		Path: targetDir,
		Name: "folder",
	}}, nil)
	if err != nil {
		t.Fatalf("countDeleteNestedEntries: %v", err)
	}
	if want := 3; got != want {
		t.Fatalf("nested count = %d, want %d", got, want)
	}
}

func TestFilePaneRestoreAnchorPathSkippingSkipsDeletedVisibleEntry(t *testing.T) {
	pane := newFilePaneState(".", nil)
	entries := make([]filesys.Entry, 0, 20)
	for i := 0; i < 20; i++ {
		name := fileDeleteTestName(i)
		entries = append(entries, filesys.Entry{
			Name:        name,
			DisplayName: name,
			Path:        name,
			Kind:        filesys.EntryFile,
		})
	}
	pane.model.entries = entries
	pane.table.List.Position.First = 10

	deleted := map[string]struct{}{
		filepath.Clean(fileDeleteTestName(10)): {},
	}
	if got, want := filePaneRestoreAnchorPathSkipping(pane, deleted, false), fileDeleteTestName(11); got != want {
		t.Fatalf("restore anchor = %q, want %q", got, want)
	}
}

func TestFormatFileDeleteErrorSimplifiesPermissionDenied(t *testing.T) {
	err := &os.PathError{Op: "remove", Path: "/.VolumeIcon.icns", Err: errors.New("operation not permitted")}
	if got, want := formatFileDeleteError(err, ""), "permission denied: /.VolumeIcon.icns"; got != want {
		t.Fatalf("formatted error = %q, want %q", got, want)
	}
	if got, want := formatFileDeleteError(err, "/override"), "permission denied: /override"; got != want {
		t.Fatalf("formatted target error = %q, want %q", got, want)
	}
}

func waitForPaneLoads(t *testing.T, ui *UI, panes ...*filePaneState) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ui.pumpFilePaneLoads(testPathLayoutContext())
		allDone := true
		for _, pane := range panes {
			if pane != nil && pane.loading {
				allDone = false
				break
			}
		}
		if allDone {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("pane load did not finish")
}

func fileDeleteTestName(i int) string {
	return fmt.Sprintf("row-%02d.txt", i)
}
