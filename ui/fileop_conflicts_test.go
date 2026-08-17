// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInspectFileOverwriteConflictsCollectsAll(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	sources := make([]fileCopySource, 0, 12)
	for i := range 12 {
		name := "item-" + string(rune('a'+i)) + ".txt"
		srcPath := filepath.Join(srcDir, name)
		if err := os.WriteFile(srcPath, []byte("new-"+name), 0o644); err != nil {
			t.Fatalf("write source: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dstDir, name), []byte("old"), 0o644); err != nil {
			t.Fatalf("write destination: %v", err)
		}
		sources = append(sources, fileCopySource{Path: srcPath, Name: name})
	}

	conflicts, count, err := inspectFileOverwriteConflicts(
		copyEndpoint{dir: srcDir}, sources,
		copyEndpoint{dir: dstDir}, dstDir,
	)
	if err != nil {
		t.Fatalf("inspect conflicts: %v", err)
	}
	if count != 12 {
		t.Fatalf("conflict count=%d want 12", count)
	}
	if len(conflicts) != 12 {
		t.Fatalf("stored conflict count=%d want 12", len(conflicts))
	}
	if fileOverwriteConflictVisibleLimit != 5 {
		t.Fatalf("visible conflict limit=%d want 5", fileOverwriteConflictVisibleLimit)
	}
	if conflicts[0].Name != "item-a.txt" || conflicts[0].SrcInfo.Size == conflicts[0].DstInfo.Size {
		t.Fatalf("first comparison=%+v", conflicts[0])
	}
}

func TestMultiFileCopyPreviewShowsOverwriteComparisons(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	sources := writeConflictFixtures(t, srcDir, dstDir)
	st := &fileCopyState{
		sources:     sources,
		srcPath:     sources[0].Path,
		srcEndpoint: copyEndpoint{dir: srcDir},
		dstEndpoint: copyEndpoint{dir: dstDir},
	}
	st.dstEdit.SetText(dstDir)
	st.refreshPreview()

	if st.conflictCount != 2 || len(st.conflicts) != 2 {
		t.Fatalf("copy conflicts=%d preview=%d", st.conflictCount, len(st.conflicts))
	}
	if got := st.confirmLabel(); got != "Overwrite" {
		t.Fatalf("copy confirm label=%q want Overwrite", got)
	}
}

func TestFileOverwriteDiffStatusFlagsOlderSource(t *testing.T) {
	base := time.Date(2026, 8, 3, 9, 15, 34, 0, time.UTC)
	if got := fileOverwriteClockText(base); got != "09:15" {
		t.Fatalf("overwrite clock text=%q want 09:15", got)
	}
	if got := fileOverwriteTimestampText(base); got != "2026-08-03 09:15:34" {
		t.Fatalf("overwrite timestamp text=%q", got)
	}
	tests := []struct {
		name     string
		src      time.Time
		dst      time.Time
		wantText string
		want     fileOverwriteTimeRelation
	}{
		{name: "newer", src: base.Add(time.Minute), dst: base, wantText: "▲ NEWER", want: fileOverwriteSourceNewer},
		{name: "older", src: base, dst: base.Add(time.Minute), wantText: "▼ OLDER", want: fileOverwriteSourceOlder},
		{name: "same", src: base, dst: base, wantText: "● SAME", want: fileOverwriteTimeSame},
		{name: "unknown", src: time.Time{}, dst: base, wantText: "? UNKNOWN", want: fileOverwriteTimeUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, relation := fileOverwriteDiffStatus(tt.src, tt.dst)
			if text != tt.wantText || relation != tt.want {
				t.Fatalf("status=(%q, %v) want (%q, %v)", text, relation, tt.wantText, tt.want)
			}
		})
	}

	conflicts := []fileOverwriteConflict{
		{SrcInfo: fileCopyPathInfo{ModTime: base}, DstInfo: fileCopyPathInfo{ModTime: base.Add(time.Minute)}},
		{SrcInfo: fileCopyPathInfo{ModTime: base.Add(time.Minute)}, DstInfo: fileCopyPathInfo{ModTime: base}},
		{SrcInfo: fileCopyPathInfo{ModTime: base.Add(-time.Minute)}, DstInfo: fileCopyPathInfo{ModTime: base}},
	}
	if got := fileOverwriteOlderSourceCount(conflicts); got != 2 {
		t.Fatalf("older source count=%d want 2", got)
	}
}

func TestMultiFileMovePreviewAndConfirmedOverwrite(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	copySources := writeConflictFixtures(t, srcDir, dstDir)
	moveSources := make([]fileMoveSource, 0, len(copySources))
	for _, source := range copySources {
		moveSources = append(moveSources, fileMoveSource{Path: source.Path, Name: source.Name})
	}
	st := &fileMoveState{
		sources:  moveSources,
		srcPath:  moveSources[0].Path,
		endpoint: copyEndpoint{dir: srcDir},
	}
	st.dstEdit.SetText(dstDir)
	st.refreshPreview()

	if st.conflictCount != 2 || len(st.conflicts) != 2 {
		t.Fatalf("move conflicts=%d preview=%d", st.conflictCount, len(st.conflicts))
	}
	if label, _ := st.actionLabels(); label != "Overwrite" {
		t.Fatalf("move action label=%q want Overwrite", label)
	}
	if _, _, plans, err := st.buildMovePlans(dstDir); err != nil || len(plans) != 2 {
		t.Fatalf("build move plans len=%d err=%v", len(plans), err)
	}

	ui := &UI{fileMove: st}
	// Simulate a destination collision appearing after the last preview. The
	// first click must reveal the comparisons, not overwrite unseen files.
	st.conflicts = nil
	st.conflictCount = 0
	ui.submitFileMoveDialog(time.Now())
	if st.running || st.conflictCount != 2 {
		t.Fatalf("first submit running=%v conflicts=%d, want confirmation", st.running, st.conflictCount)
	}
	ui.submitFileMoveDialog(time.Now())
	select {
	case err := <-st.doneCh:
		if err != nil {
			t.Fatalf("move overwrite: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("move overwrite timed out")
	}
	for _, source := range moveSources {
		if _, err := os.Lstat(source.Path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("source %q still exists: %v", source.Path, err)
		}
		contents, err := os.ReadFile(filepath.Join(dstDir, source.Name))
		if err != nil {
			t.Fatalf("read moved destination: %v", err)
		}
		if string(contents) != "new-"+source.Name {
			t.Fatalf("destination %q contents=%q", source.Name, contents)
		}
	}
}

func writeConflictFixtures(t *testing.T, srcDir, dstDir string) []fileCopySource {
	t.Helper()
	names := []string{"alpha.txt", "beta.txt"}
	sources := make([]fileCopySource, 0, len(names))
	for _, name := range names {
		srcPath := filepath.Join(srcDir, name)
		if err := os.WriteFile(srcPath, []byte("new-"+name), 0o644); err != nil {
			t.Fatalf("write source: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dstDir, name), []byte("old-"+name), 0o644); err != nil {
			t.Fatalf("write destination: %v", err)
		}
		sources = append(sources, fileCopySource{Path: srcPath, Name: name})
	}
	return sources
}
