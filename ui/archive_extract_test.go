// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hexone/filesys"
	"hexone/fm"
)

func TestStartArchiveExtractHereDoesNotOpenCopyDialog(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"docs/readme.txt": "hello",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	pane.dir = root
	pane.model = &filePaneModel{
		entries: []filesys.Entry{{
			Path:        archivePath,
			DisplayName: "bundle.zip",
			Kind:        filesys.EntryFile,
			CanEnter:    true,
		}},
		cfg: ui.fmCfg,
	}

	ui.startArchiveExtractHere(0, 0, time.Now())

	if ui.fileCopy != nil {
		t.Fatal("extract here should not open the copy dialog")
	}
	if ui.archiveExtract == nil {
		t.Fatal("extract here should start an archive extract state")
	}
	if got := archiveExtractStatusLabel(ui.archiveExtract, time.Now()); !strings.Contains(got, "preparing") {
		t.Fatalf("initial extract status = %q, want preparing state before background plan progress", got)
	}
	waitForArchiveExtract(t, ui)
}

func TestRunArchiveExtractPlansOverwriteSingleConflict(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"docs/readme.txt": "new body",
		"docs/notes.txt":  "fresh",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	dstDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dstDir, "docs"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "docs", "readme.txt"), []byte("old body"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	effectiveDstDir, extractRoot, plans, _, err := buildArchiveExtractPlans(archivePath, dstDir)
	if err != nil {
		t.Fatalf("buildArchiveExtractPlans: %v", err)
	}

	var conflicts []string
	err = runArchiveExtractPlans(extractRoot, plans, copyEndpoint{dir: effectiveDstDir}, effectiveDstDir, func(conflict archiveExtractConflict) archiveExtractConflictDecision {
		conflicts = append(conflicts, conflict.displayPath)
		return archiveExtractDecisionOverwrite
	}, nil)
	if err != nil {
		t.Fatalf("runArchiveExtractPlans: %v", err)
	}

	if got, want := string(mustReadFile(t, filepath.Join(dstDir, "docs", "readme.txt"))), "new body"; got != want {
		t.Fatalf("readme content = %q, want %q", got, want)
	}
	if got, want := string(mustReadFile(t, filepath.Join(dstDir, "docs", "notes.txt"))), "fresh"; got != want {
		t.Fatalf("notes content = %q, want %q", got, want)
	}
	if len(conflicts) != 1 || conflicts[0] != "docs/readme.txt" {
		t.Fatalf("conflicts = %v, want [docs/readme.txt]", conflicts)
	}
}

func waitForArchiveExtract(t *testing.T, ui *UI) {
	t.Helper()
	if ui == nil || ui.archiveExtract == nil {
		return
	}
	select {
	case done := <-ui.archiveExtract.doneCh:
		if done.dstDir != "" {
			ui.archiveExtract.dstDir = done.dstDir
		}
		ui.archiveExtract.totalEntries = done.totalEntries
		ui.finishArchiveExtract(time.Now(), done.err)
	case <-time.After(2 * time.Second):
		t.Fatal("archive extract did not finish")
	}
}

func TestRunArchiveExtractPlansOverwriteAllSkipsFurtherPrompts(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"a.txt": "new a",
		"b.txt": "new b",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	dstDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dstDir, "bundle"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "bundle", "a.txt"), []byte("old a"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "bundle", "b.txt"), []byte("old b"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	effectiveDstDir, extractRoot, plans, _, err := buildArchiveExtractPlans(archivePath, dstDir)
	if err != nil {
		t.Fatalf("buildArchiveExtractPlans: %v", err)
	}

	conflictCalls := 0
	err = runArchiveExtractPlans(extractRoot, plans, copyEndpoint{dir: effectiveDstDir}, effectiveDstDir, func(conflict archiveExtractConflict) archiveExtractConflictDecision {
		conflictCalls++
		return archiveExtractDecisionOverwriteAll
	}, nil)
	if err != nil {
		t.Fatalf("runArchiveExtractPlans: %v", err)
	}

	if conflictCalls != 1 {
		t.Fatalf("conflict calls = %d, want 1", conflictCalls)
	}
	if got, want := string(mustReadFile(t, filepath.Join(dstDir, "bundle", "a.txt"))), "new a"; got != want {
		t.Fatalf("a.txt content = %q, want %q", got, want)
	}
	if got, want := string(mustReadFile(t, filepath.Join(dstDir, "bundle", "b.txt"))), "new b"; got != want {
		t.Fatalf("b.txt content = %q, want %q", got, want)
	}
}

func TestRunArchiveExtractPlansAbortLeavesExistingFile(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"a.txt": "new a",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	dstDir := t.TempDir()
	dstFile := filepath.Join(dstDir, "a.txt")
	if err := os.WriteFile(dstFile, []byte("old a"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	effectiveDstDir, extractRoot, plans, _, err := buildArchiveExtractPlans(archivePath, dstDir)
	if err != nil {
		t.Fatalf("buildArchiveExtractPlans: %v", err)
	}

	err = runArchiveExtractPlans(extractRoot, plans, copyEndpoint{dir: effectiveDstDir}, effectiveDstDir, func(conflict archiveExtractConflict) archiveExtractConflictDecision {
		return archiveExtractDecisionAbort
	}, nil)
	if !errors.Is(err, errArchiveExtractAborted) {
		t.Fatalf("runArchiveExtractPlans err = %v, want %v", err, errArchiveExtractAborted)
	}

	if got, want := string(mustReadFile(t, dstFile)), "old a"; got != want {
		t.Fatalf("a.txt content = %q, want %q", got, want)
	}
}

func TestRunArchiveExtractPlansReportsProgress(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"a.txt": "new a",
		"b.txt": "new b",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	dstDir := t.TempDir()
	effectiveDstDir, extractRoot, plans, totalEntries, err := buildArchiveExtractPlans(archivePath, dstDir)
	if err != nil {
		t.Fatalf("buildArchiveExtractPlans: %v", err)
	}

	var reports []filesys.CopyProgress
	err = runArchiveExtractPlans(extractRoot, plans, copyEndpoint{dir: effectiveDstDir}, effectiveDstDir, nil, func(progress filesys.CopyProgress) {
		reports = append(reports, progress)
	})
	if err != nil {
		t.Fatalf("runArchiveExtractPlans: %v", err)
	}
	if len(reports) == 0 {
		t.Fatal("expected progress reports")
	}
	last := reports[len(reports)-1]
	if last.EntriesDone != totalEntries || last.EntriesTotal != totalEntries {
		t.Fatalf("last entries progress = %d/%d, want %d/%d", last.EntriesDone, last.EntriesTotal, totalEntries, totalEntries)
	}
	if last.BytesDone != last.BytesTotal || last.BytesTotal <= 0 {
		t.Fatalf("last byte progress = %d/%d, want completed positive total", last.BytesDone, last.BytesTotal)
	}
}

func TestArchiveExtractStatusLabelShowsProgressSpeedAndETA(t *testing.T) {
	started := time.Unix(100, 0)
	st := &archiveExtractState{
		archivePath: filepath.Join("bundle.zip"),
		startedAt:   started,
		progress: filesys.CopyProgress{
			EntriesDone:  1,
			EntriesTotal: 2,
			BytesDone:    42 << 20,
			BytesTotal:   100 << 20,
			CurrentPath:  filepath.Join("bundle.zip", "movie.mkv"),
		},
	}

	got := archiveExtractStatusLabel(st, started.Add(time.Second))
	for _, want := range []string{
		"[Extracting] movie.mkv",
		"████░░░░░░ 42%",
		"42.0 MB/s",
		"left",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status = %q, want to contain %q", got, want)
		}
	}
}

func TestArchiveExtractStatusLineTrimsFilenameBeforeProgressFields(t *testing.T) {
	started := time.Unix(100, 0)
	st := &archiveExtractState{
		archivePath: "bundle.zip",
		startedAt:   started,
		progress: filesys.CopyProgress{
			BytesDone:   50 << 20,
			BytesTotal:  100 << 20,
			CurrentPath: filepath.Join("bundle.zip", "Community.S01E01.1080p.BluRay.x264-YELLOWBiRD.mkv"),
		},
	}
	measure := func(text string) int {
		return len([]rune(text))
	}

	got := archiveExtractStatusLineForWidth(st, started.Add(time.Second), 72, measure)
	if measure(got) > 72 {
		t.Fatalf("status width = %d, want <= 72: %q", measure(got), got)
	}
	for _, want := range []string{"[Extracting]", "█████░░░░░ 50%", "50.0 MB/s", "left"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status = %q, want to contain %q", got, want)
		}
	}
	if strings.Contains(got, "free") {
		t.Fatalf("status = %q, free-space should stay in its own bar", got)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("status = %q, want trimmed filename", got)
	}
	if strings.Contains(got, "Community.S01E01.1080p.BluRay.x264-YELLOWBiRD.mkv") {
		t.Fatalf("status = %q, want long filename trimmed", got)
	}
}

func TestArchiveExtractStatusLineWithSeparatorPrefixesAndSuffixesExtraction(t *testing.T) {
	started := time.Unix(100, 0)
	st := &archiveExtractState{
		archivePath: "bundle.zip",
		startedAt:   started,
		progress: filesys.CopyProgress{
			BytesDone:   50 << 20,
			BytesTotal:  100 << 20,
			CurrentPath: filepath.Join("bundle.zip", "movie.mkv"),
		},
	}
	measure := func(text string) int {
		return len([]rune(text))
	}

	got := archiveExtractStatusLineWithSeparatorForWidth(st, started.Add(time.Second), 80, measure, false)
	if !strings.HasPrefix(got, "| [Extracting]") {
		t.Fatalf("status = %q, want separator immediately before extraction label", got)
	}

	got = archiveExtractStatusLineWithSeparatorForWidth(st, started.Add(time.Second), 80, measure, true)
	if !strings.HasSuffix(got, " |") {
		t.Fatalf("status = %q, want separator after extraction label", got)
	}
}

func TestBuildArchiveExtractPlansWrapsMultipleTopLevelFiles(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"a.txt": "a",
		"b.txt": "b",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	dstDir := t.TempDir()
	effectiveDstDir, extractRoot, plans, totalEntries, err := buildArchiveExtractPlans(archivePath, dstDir)
	if err != nil {
		t.Fatalf("buildArchiveExtractPlans: %v", err)
	}

	if got, want := effectiveDstDir, dstDir; got != want {
		t.Fatalf("effectiveDstDir = %q, want %q", got, want)
	}
	if got, want := extractRoot, filepath.Join(dstDir, "bundle"); got != want {
		t.Fatalf("extractRoot = %q, want %q", got, want)
	}
	if totalEntries != 2 {
		t.Fatalf("totalEntries = %d, want 2", totalEntries)
	}
	if len(plans) != 2 {
		t.Fatalf("plan count = %d, want 2", len(plans))
	}
	if got, want := plans[0].dstPath, filepath.Join(dstDir, "bundle", "a.txt"); got != want {
		t.Fatalf("first plan dstPath = %q, want %q", got, want)
	}
	if got, want := plans[1].dstPath, filepath.Join(dstDir, "bundle", "b.txt"); got != want {
		t.Fatalf("second plan dstPath = %q, want %q", got, want)
	}
}

func TestBuildArchiveExtractPlansWrapsMixedTopLevelEntries(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"docs/readme.txt": "hello",
		"movie.mkv":       "video",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	dstDir := t.TempDir()
	effectiveDstDir, extractRoot, plans, totalEntries, err := buildArchiveExtractPlans(archivePath, dstDir)
	if err != nil {
		t.Fatalf("buildArchiveExtractPlans: %v", err)
	}

	if got, want := effectiveDstDir, dstDir; got != want {
		t.Fatalf("effectiveDstDir = %q, want %q", got, want)
	}
	if got, want := extractRoot, filepath.Join(dstDir, "bundle"); got != want {
		t.Fatalf("extractRoot = %q, want %q", got, want)
	}
	if totalEntries != 3 {
		t.Fatalf("totalEntries = %d, want 3", totalEntries)
	}
	if len(plans) != 2 {
		t.Fatalf("plan count = %d, want 2", len(plans))
	}
	if got, want := plans[0].dstPath, filepath.Join(dstDir, "bundle", "docs"); got != want {
		t.Fatalf("first plan dstPath = %q, want %q", got, want)
	}
	if got, want := plans[1].dstPath, filepath.Join(dstDir, "bundle", "movie.mkv"); got != want {
		t.Fatalf("second plan dstPath = %q, want %q", got, want)
	}
}

func TestBuildArchiveExtractPlansExtractsSingleTopLevelFileHere(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "movie.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"movie.iso": "disc image",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	dstDir := t.TempDir()
	effectiveDstDir, extractRoot, plans, totalEntries, err := buildArchiveExtractPlans(archivePath, dstDir)
	if err != nil {
		t.Fatalf("buildArchiveExtractPlans: %v", err)
	}

	if got, want := effectiveDstDir, dstDir; got != want {
		t.Fatalf("effectiveDstDir = %q, want %q", got, want)
	}
	if got, want := extractRoot, dstDir; got != want {
		t.Fatalf("extractRoot = %q, want %q", got, want)
	}
	if totalEntries != 1 {
		t.Fatalf("totalEntries = %d, want 1", totalEntries)
	}
	if len(plans) != 1 {
		t.Fatalf("plan count = %d, want 1", len(plans))
	}
	if got, want := plans[0].dstPath, filepath.Join(dstDir, "movie.iso"); got != want {
		t.Fatalf("plan dstPath = %q, want %q", got, want)
	}
}

func TestArchiveExtractWrapperNameStripsArchiveSuffixChain(t *testing.T) {
	if got, want := archiveExtractWrapperName("/tmp/bundle.tar.gz"), "bundle"; got != want {
		t.Fatalf("archiveExtractWrapperName = %q, want %q", got, want)
	}
}

func TestBuildArchiveExtractPlansExtractsSingleTopLevelDirHere(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"docs/readme.txt": "hello",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	dstDir := t.TempDir()
	effectiveDstDir, extractRoot, plans, totalEntries, err := buildArchiveExtractPlans(archivePath, dstDir)
	if err != nil {
		t.Fatalf("buildArchiveExtractPlans: %v", err)
	}

	if got, want := effectiveDstDir, dstDir; got != want {
		t.Fatalf("effectiveDstDir = %q, want %q", got, want)
	}
	if got, want := extractRoot, dstDir; got != want {
		t.Fatalf("extractRoot = %q, want %q", got, want)
	}
	if totalEntries != 2 {
		t.Fatalf("totalEntries = %d, want 2", totalEntries)
	}
	if len(plans) != 1 {
		t.Fatalf("plan count = %d, want 1", len(plans))
	}
	if got, want := plans[0].dstPath, filepath.Join(dstDir, "docs"); got != want {
		t.Fatalf("plan dstPath = %q, want %q", got, want)
	}
}

func TestRunArchiveExtractPlansCanOverwriteTopLevelFileConflict(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"a.txt": "new a",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	dstDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dstDir, "a.txt"), []byte("old a"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	effectiveDstDir, extractRoot, plans, _, err := buildArchiveExtractPlans(archivePath, dstDir)
	if err != nil {
		t.Fatalf("buildArchiveExtractPlans: %v", err)
	}

	var conflicts []string
	err = runArchiveExtractPlans(extractRoot, plans, copyEndpoint{dir: effectiveDstDir}, effectiveDstDir, func(conflict archiveExtractConflict) archiveExtractConflictDecision {
		conflicts = append(conflicts, conflict.displayPath)
		return archiveExtractDecisionOverwrite
	}, nil)
	if err != nil {
		t.Fatalf("runArchiveExtractPlans: %v", err)
	}

	if len(conflicts) != 1 || conflicts[0] != "a.txt" {
		t.Fatalf("conflicts = %v, want [a.txt]", conflicts)
	}
	if got, want := string(mustReadFile(t, filepath.Join(dstDir, "a.txt"))), "new a"; got != want {
		t.Fatalf("a.txt content = %q, want %q", got, want)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", path, err)
	}
	return data
}
