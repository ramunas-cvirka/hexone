// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

func TestFileDeleteTrashModeIsLocalOnly(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.General.UseTrash = true
	if !fileDeleteShouldUseTrash(cfg, nil) {
		t.Fatal("local deletion should use Trash when configured")
	}
	if fileDeleteShouldUseTrash(cfg, &paneSSHSession{}) {
		t.Fatal("SSH deletion must remain permanent")
	}
	cfg.General.UseTrash = false
	if fileDeleteShouldUseTrash(cfg, nil) {
		t.Fatal("Trash should remain off by default")
	}
}

func TestSubmitFileDeleteUsesSingleBatchedTrashOperation(t *testing.T) {
	original := fileDeleteMovePathsToTrash
	defer func() { fileDeleteMovePathsToTrash = original }()

	called := make(chan []string, 1)
	fileDeleteMovePathsToTrash = func(paths []string) error {
		called <- append([]string(nil), paths...)
		return nil
	}
	st := &fileDeleteState{
		useTrash: true,
		targets: []fileDeleteTarget{
			{Path: "/tmp/alpha.txt", Name: "alpha.txt"},
			{Path: "/tmp/beta", Name: "beta"},
		},
	}
	ui := NewUI(fm.DefaultConfig())
	ui.fileDelete = st
	ui.submitFileDeleteDialog(time.Now())

	select {
	case got := <-called:
		want := []string{"/tmp/alpha.txt", "/tmp/beta"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Trash paths=%v want %v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Trash operation was not called")
	}
	if res := <-st.doneCh; res.err != nil {
		t.Fatalf("Trash operation result: %v", res.err)
	}
}

func TestDeleteWithoutConfirmationSubmitsImmediately(t *testing.T) {
	original := fileDeleteMovePathsToTrash
	defer func() { fileDeleteMovePathsToTrash = original }()

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	fileDeleteMovePathsToTrash = func(paths []string) error {
		started <- struct{}{}
		<-release
		return nil
	}

	root := t.TempDir()
	target := filepath.Join(root, "alpha.txt")
	if err := os.WriteFile(target, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	cfg := fm.DefaultConfig()
	cfg.General.UseTrash = true
	cfg.General.DeleteWithoutConfirm = true
	ui := NewUI(cfg)
	pane := ui.filePanes[0]
	waitForPaneLoads(t, ui, pane)
	listing, err := filesys.ReadDir(root)
	if err != nil {
		t.Fatalf("read target directory: %v", err)
	}
	pane.applyListing(listing, target, "", 0)
	pane.table.Selected = pane.findEntryPathIndex(target)
	if pane.table.Selected < 0 {
		t.Fatal("target was not selected")
	}

	ui.startFileDeleteDialog(0, time.Now())
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("delete-without-confirmation did not start the operation")
	}
	st := ui.fileDelete
	if st == nil || !st.running || !st.useTrash {
		t.Fatalf("immediate delete state=%#v", st)
	}
	close(release)
	if res := <-st.doneCh; res.err != nil {
		t.Fatalf("immediate Trash result: %v", res.err)
	}
}

func TestDeleteConfirmationRemainsEnabledByDefault(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "alpha.txt")
	if err := os.WriteFile(target, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	waitForPaneLoads(t, ui, pane)
	listing, err := filesys.ReadDir(root)
	if err != nil {
		t.Fatalf("read target directory: %v", err)
	}
	pane.applyListing(listing, target, "", 0)
	pane.table.Selected = pane.findEntryPathIndex(target)

	ui.startFileDeleteDialog(0, time.Now())
	if ui.fileDelete == nil {
		t.Fatal("default delete should open a confirmation dialog")
	}
	if ui.fileDelete.running {
		t.Fatal("default delete should not start before confirmation")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("default confirmation should leave target untouched: %v", err)
	}
}

func TestTrashSuccessAndErrorCopyIsRecoverable(t *testing.T) {
	if got, _ := fileDeleteSuccessNotice(2, 0, true); got != "moved 2 items to Trash" {
		t.Fatalf("Trash success notice=%q", got)
	}
	err := errors.New("volume does not support Trash")
	if got := formatFileDeleteOperationError(err, "/tmp/a", true); got != "move to Trash failed: volume does not support Trash" {
		t.Fatalf("Trash error=%q", got)
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
