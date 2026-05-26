// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"os"
	"path/filepath"
	"strings"
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

func TestFileOverwriteDiffRowsHighlightOnlyChangedTimePart(t *testing.T) {
	srcTime := time.Date(2026, 5, 8, 14, 40, 26, 0, time.UTC)
	dstTime := time.Date(2026, 5, 8, 14, 41, 12, 0, time.UTC)
	rows := fileOverwriteDiffRows(
		fileCopyPathInfo{Exists: true, Size: 38 << 20, ModTime: srcTime},
		fileCopyPathInfo{Exists: true, Size: 38 << 20, ModTime: dstTime},
	)

	for _, row := range rows {
		assertOverwriteDiffPart(t, row, "38.0 MB", false)
		assertOverwriteDiffPart(t, row, "2026-05-08", false)
	}
	assertOverwriteDiffPart(t, rows[0], "14:40:26", true)
	assertOverwriteDiffPart(t, rows[1], "14:41:12", false)
}

func TestFileOverwriteDiffRowsHighlightSizeAndDateSeparately(t *testing.T) {
	srcTime := time.Date(2026, 5, 8, 14, 40, 26, 0, time.UTC)
	dstTime := time.Date(2026, 5, 9, 14, 40, 26, 0, time.UTC)
	rows := fileOverwriteDiffRows(
		fileCopyPathInfo{Exists: true, Size: 38 << 20, ModTime: srcTime},
		fileCopyPathInfo{Exists: true, Size: 40 << 20, ModTime: dstTime},
	)

	assertOverwriteDiffPart(t, rows[0], "38.0 MB", true)
	assertOverwriteDiffPart(t, rows[1], "40.0 MB", false)
	assertOverwriteDiffPart(t, rows[0], "2026-05-08", true)
	assertOverwriteDiffPart(t, rows[1], "2026-05-09", false)
	assertOverwriteDiffPart(t, rows[0], "14:40:26", false)
	assertOverwriteDiffPart(t, rows[1], "14:40:26", false)
}

func assertOverwriteDiffPart(t *testing.T, row fileOverwriteDiffRow, text string, wantHighlight bool) {
	t.Helper()
	for _, part := range []fileOverwriteDiffPart{row.Size, row.Date, row.Time} {
		if part.Text == text {
			if part.Highlight != wantHighlight {
				t.Fatalf("part %q highlight = %v, want %v in row %+v", text, part.Highlight, wantHighlight, row)
			}
			return
		}
	}
	t.Fatalf("part %q not found in row %+v", text, row)
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
