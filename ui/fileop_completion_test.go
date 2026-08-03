// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/widget/material"
	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/widget/table"
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

func TestCopyProgressTextIncludesTransferSpeed(t *testing.T) {
	progress := filesys.CopyProgress{
		EntriesDone:  1,
		EntriesTotal: 2,
		BytesDone:    2 << 20,
		BytesTotal:   4 << 20,
	}

	got := copyProgressText(progress, 1<<20)
	if !strings.Contains(got, "2.0 MB / 4.0 MB") {
		t.Fatalf("progress text = %q, want copied and total size", got)
	}
	if !strings.Contains(got, "1.0 MB/s") {
		t.Fatalf("progress text = %q, want transfer speed", got)
	}
}

func TestFileCopySpeedSamplesAtInterval(t *testing.T) {
	now := time.Unix(100, 0)
	st := &fileCopyState{
		progress: filesys.CopyProgress{BytesDone: 1 << 20},
	}
	st.sampleCopySpeed(now)

	st.progress.BytesDone = 2 << 20
	st.sampleCopySpeed(now.Add(500 * time.Millisecond))
	if st.speedBytes != 0 {
		t.Fatalf("speed before sample interval = %d, want 0", st.speedBytes)
	}

	st.sampleCopySpeed(now.Add(fileCopySpeedSampleInterval))
	if st.speedBytes <= 0 {
		t.Fatalf("speed after sample interval = %d, want > 0", st.speedBytes)
	}
	sampled := st.speedBytes

	st.progress.BytesDone = 4 << 20
	st.sampleCopySpeed(now.Add(fileCopySpeedSampleInterval + 200*time.Millisecond))
	if st.speedBytes != sampled {
		t.Fatalf("speed changed before next sample interval = %d, want %d", st.speedBytes, sampled)
	}

	st.sampleCopySpeed(now.Add(2 * fileCopySpeedSampleInterval))
	if st.speedBytes <= 0 {
		t.Fatalf("next sampled speed = %d, want > 0", st.speedBytes)
	}
	sampled = st.speedBytes

	st.sampleCopySpeed(now.Add(3 * fileCopySpeedSampleInterval))
	if st.speedBytes != sampled {
		t.Fatalf("speed cleared too soon after quiet sample = %d, want %d", st.speedBytes, sampled)
	}

	st.sampleCopySpeed(now.Add(5 * fileCopySpeedSampleInterval))
	if st.speedBytes != 0 {
		t.Fatalf("stale speed = %d, want 0", st.speedBytes)
	}
}

func TestFinishFileMoveKeepsSourceScrollStableAndShowsNotice(t *testing.T) {
	srcDir := t.TempDir()
	createFileOpRows(t, srcDir, 30)
	srcPath := filepath.Join(srcDir, fileDeleteTestName(10))

	dstDir := t.TempDir()
	createFileOpRows(t, dstDir, 30)
	ui := NewUI(fm.DefaultConfig())
	srcPane := ui.filePanes[0]
	dstPane := ui.filePanes[1]
	waitForPaneLoads(t, ui, srcPane, dstPane)
	prepareFileOpPane(t, srcPane, srcDir, 10)
	prepareFileOpPane(t, dstPane, dstDir, 20)
	srcPane.table.List.Position.Offset = -5
	dstPane.table.List.Position.Offset = -9
	dstSelectedPath := dstPane.selectedEntry().Path
	ui.setActiveFilePane(0)

	dstPath := filepath.Join(dstDir, "aaa-moved.txt")
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
	waitForPaneLoads(t, ui, srcPane, dstPane)

	if got := srcPane.table.List.Position.First; got != 11 {
		t.Fatalf("source first visible row = %d, want 11", got)
	}
	if got := srcPane.table.List.Position.Offset; got != -5 {
		t.Fatalf("source pixel offset = %d, want -5", got)
	}
	if got := srcPane.selectedEntry(); got == nil || got.Path != filepath.Join(srcDir, fileDeleteTestName(11)) {
		t.Fatalf("source selection = %+v, want nearby file %q", got, filepath.Join(srcDir, fileDeleteTestName(11)))
	}
	if got := dstPane.table.List.Position.First; got != 22 {
		t.Fatalf("destination first visible row = %d, want 22", got)
	}
	if got := dstPane.table.List.Position.Offset; got != -9 {
		t.Fatalf("destination pixel offset = %d, want -9", got)
	}
	if got := dstPane.selectedEntry(); got == nil || got.Path != dstSelectedPath {
		t.Fatalf("destination selection = %+v, want preserved path %q", got, dstSelectedPath)
	}
	if got := ui.activeFilePane; got != 0 {
		t.Fatalf("activeFilePane = %d, want 0", got)
	}
	if got, want := srcPane.noticeText, "moved 1 item"; got != want {
		t.Fatalf("source noticeText = %q, want %q", got, want)
	}
}

func TestFinishFileMoveKeepsBriefViewportsStable(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	createFileOpRows(t, srcDir, 120)
	createFileOpRows(t, dstDir, 120)

	ui := NewUI(fm.DefaultConfig())
	srcPane := ui.filePanes[0]
	dstPane := ui.filePanes[1]
	waitForPaneLoads(t, ui, srcPane, dstPane)
	prepareFileOpPane(t, srcPane, srcDir, 0)
	prepareFileOpPane(t, dstPane, dstDir, 0)

	gtx := testPathLayoutContext()
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max = image.Pt(480, 180)
	th := material.NewTheme()
	for _, pane := range []*filePaneState{srcPane, dstPane} {
		pane.table.SetMode(table.ModeBrief)
		pane.table.Layout(th, gtx, pane.model)
	}
	srcPane.table.List.Position.First = 3
	dstPane.table.List.Position.First = 4
	for _, pane := range []*filePaneState{srcPane, dstPane} {
		pane.table.Layout(th, gtx, pane.model)
	}

	srcRow := srcPane.table.FirstVisibleRow() + 2
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
		t.Fatalf("os.Rename: %v", err)
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
	for _, pane := range []*filePaneState{srcPane, dstPane} {
		pane.table.Layout(th, gtx, pane.model)
	}

	if got := sanitizePaneListPosition(srcPane.table.List.Position); got != srcPos {
		t.Fatalf("source brief position = %+v, want %+v", got, srcPos)
	}
	if got := sanitizePaneListPosition(dstPane.table.List.Position); got != dstPos {
		t.Fatalf("destination brief position = %+v, want %+v", got, dstPos)
	}
	if got := srcPane.selectedEntry(); got == nil || got.Path == srcPath {
		t.Fatalf("source selection = %+v, want a nearby surviving file", got)
	}
	if got := dstPane.selectedEntry(); got == nil || got.Path != dstSelectedPath {
		t.Fatalf("destination selection = %+v, want preserved path %q", got, dstSelectedPath)
	}
	if got := ui.activeFilePane; got != 0 {
		t.Fatalf("activeFilePane = %d, want 0", got)
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

func TestFinishFolderCreateRevealsSelectionInReducedPane(t *testing.T) {
	dir := t.TempDir()
	createFileOpRows(t, dir, 30)

	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	waitForPaneLoads(t, ui, pane)
	prepareFileOpPane(t, pane, dir, 10)

	createdPath := filepath.Join(dir, "aaa-new-folder")
	if err := os.Mkdir(createdPath, 0o755); err != nil {
		t.Fatalf("os.Mkdir(%q): %v", createdPath, err)
	}
	ui.fileCreate = &fileCreateState{
		pane:       0,
		targetPath: createdPath,
		kind:       fileCreateKindFolder,
	}

	ui.finishFileCreate(time.Now())
	waitForPaneLoads(t, ui, pane)

	createdRow := pane.findEntryPathIndex(createdPath)
	if createdRow < 0 {
		t.Fatal("created folder missing from refreshed listing")
	}
	if pane.table.Selected != createdRow {
		t.Fatalf("selected row=%d want created folder row %d", pane.table.Selected, createdRow)
	}

	gtx := testPathLayoutContext()
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.Y = 84 // Model the shorter file pane while the terminal drawer is open.
	pane.table.Layout(material.NewTheme(), gtx, pane.model)
	pos := pane.table.List.Position
	if createdRow < pos.First || createdRow >= pos.First+pos.Count {
		t.Fatalf("created folder row %d is outside visible rows [%d,%d)", createdRow, pos.First, pos.First+pos.Count)
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
