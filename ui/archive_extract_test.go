// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"os"
	"path/filepath"
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
	})
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
	case err := <-ui.archiveExtract.doneCh:
		ui.finishArchiveExtract(time.Now(), err)
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
	})
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
	if err := os.MkdirAll(filepath.Join(dstDir, "bundle"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll: %v", err)
	}
	dstFile := filepath.Join(dstDir, "bundle", "a.txt")
	if err := os.WriteFile(dstFile, []byte("old a"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	effectiveDstDir, extractRoot, plans, _, err := buildArchiveExtractPlans(archivePath, dstDir)
	if err != nil {
		t.Fatalf("buildArchiveExtractPlans: %v", err)
	}

	err = runArchiveExtractPlans(extractRoot, plans, copyEndpoint{dir: effectiveDstDir}, effectiveDstDir, func(conflict archiveExtractConflict) archiveExtractConflictDecision {
		return archiveExtractDecisionAbort
	})
	if !errors.Is(err, errArchiveExtractAborted) {
		t.Fatalf("runArchiveExtractPlans err = %v, want %v", err, errArchiveExtractAborted)
	}

	if got, want := string(mustReadFile(t, dstFile)), "old a"; got != want {
		t.Fatalf("a.txt content = %q, want %q", got, want)
	}
}

func TestBuildArchiveExtractPlansWrapsTopLevelFiles(t *testing.T) {
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
}

func TestArchiveExtractWrapperNameStripsArchiveSuffixChain(t *testing.T) {
	if got, want := archiveExtractWrapperName("/tmp/bundle.tar.gz"), "bundle"; got != want {
		t.Fatalf("archiveExtractWrapperName = %q, want %q", got, want)
	}
}

func TestBuildArchiveExtractPlansSkipsWrapperForSingleTopLevelDir(t *testing.T) {
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

func TestRunArchiveExtractPlansCanOverwriteWrapperPathConflict(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"a.txt": "new a",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	dstDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dstDir, "bundle"), []byte("not a dir"), 0o644); err != nil {
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
	})
	if err != nil {
		t.Fatalf("runArchiveExtractPlans: %v", err)
	}

	if len(conflicts) != 1 || conflicts[0] != "bundle" {
		t.Fatalf("conflicts = %v, want [bundle]", conflicts)
	}
	if got, want := string(mustReadFile(t, filepath.Join(dstDir, "bundle", "a.txt"))), "new a"; got != want {
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
