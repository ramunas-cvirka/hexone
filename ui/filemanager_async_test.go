// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/filesys"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/layout"
)

func TestFilePaneApplyListingSelectionPriority(t *testing.T) {
	pane := newFilePaneState(".", nil)
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta.txt", Path: "beta.txt"},
		},
	}, "beta.txt", "alpha.txt", 0)

	if pane.table.Selected != 1 {
		t.Fatalf("selected row = %d, want 1", pane.table.Selected)
	}
}

func TestStartLocalPaneLoadIsAsync(t *testing.T) {
	dir := t.TempDir()
	pane := newFilePaneState(dir, nil)

	if !startLocalPaneLoad(pane, dir, "", "", 0) {
		t.Fatal("startLocalPaneLoad returned false")
	}
	if !pane.loading {
		t.Fatal("pane should be loading immediately")
	}
	if pane.loadingDir == "" {
		t.Fatal("loadingDir should be populated")
	}

	select {
	case res := <-pane.loadResultCh:
		if res.err != nil {
			t.Fatalf("unexpected load error: %v", res.err)
		}
		if res.listing.Dir == "" {
			t.Fatal("listing dir should not be empty")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pane load result")
	}
}

func TestFilePaneLoadingHintVisibleAfterDelay(t *testing.T) {
	pane := newFilePaneState(".", nil)
	pane.loading = true
	pane.loadingDir = `D:\`
	pane.loadingStartedAt = time.Now()

	if pane.loadingHintVisible(time.Now()) {
		t.Fatal("loading hint should stay hidden before the delay")
	}
	if !pane.loadingHintVisible(time.Now().Add(filePaneLoadingHintDelay + 10*time.Millisecond)) {
		t.Fatal("loading hint should appear after the delay")
	}
}

func TestPumpFilePaneLocalRefreshUpdatesAddDeleteRenameWithoutLosingViewState(t *testing.T) {
	dir := t.TempDir()
	alphaPath := filepath.Join(dir, "alpha.txt")
	bravoPath := filepath.Join(dir, "bravo.txt")
	charliePath := filepath.Join(dir, "charlie.txt")
	deltaPath := filepath.Join(dir, "delta.txt")
	for _, item := range []string{alphaPath, bravoPath, charliePath, deltaPath} {
		if err := os.WriteFile(item, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %q: %v", item, err)
		}
	}

	listing, err := filesys.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}

	ui := &UI{
		filePanes: []*filePaneState{newFilePaneState(dir, nil)},
	}
	pane := ui.filePanes[0]
	pane.applyListing(listing, deltaPath, "", 0)
	charlieRow := pane.findEntryPathIndex(charliePath)
	if charlieRow < 0 {
		t.Fatal("charlie row missing from initial listing")
	}
	if !pane.markRow(charlieRow) {
		t.Fatal("expected charlie row to be markable")
	}
	pane.table.List.Position = layout.Position{First: charlieRow, Offset: 0}

	start := time.Date(2026, time.April, 8, 12, 0, 0, 0, time.UTC)
	gtx := testPathLayoutContext()
	gtx.Now = start
	ui.pumpFilePaneLocalRefresh(gtx)
	if pane.loading {
		t.Fatal("first local refresh pass should only capture baseline state")
	}

	zuluPath := filepath.Join(dir, "zulu.txt")
	docsPath := filepath.Join(dir, "docs")
	if err := os.Remove(alphaPath); err != nil {
		t.Fatalf("remove alpha: %v", err)
	}
	if err := os.Rename(bravoPath, zuluPath); err != nil {
		t.Fatalf("rename bravo: %v", err)
	}
	if err := os.Mkdir(docsPath, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}

	gtx = testPathLayoutContext()
	gtx.Now = start.Add(filePaneDirWatchPoll + time.Millisecond)
	ui.pumpFilePaneLocalRefresh(gtx)
	if !pane.loading {
		t.Fatal("directory change should trigger a quiet background refresh")
	}
	if !pane.loadQuiet {
		t.Fatal("background refresh should stay quiet")
	}
	if pane.loadingHintVisible(gtx.Now.Add(filePaneLoadingHintDelay + time.Second)) {
		t.Fatal("quiet background refresh should not show the loading hint")
	}

	deadline := time.Now().Add(2 * time.Second)
	for pane.loading && time.Now().Before(deadline) {
		ui.pumpFilePaneLoads(gtx)
		time.Sleep(10 * time.Millisecond)
	}
	if pane.loading {
		t.Fatal("timed out waiting for refreshed listing")
	}

	if pane.findEntryPathIndex(alphaPath) >= 0 {
		t.Fatal("deleted file should be removed after refresh")
	}
	if pane.findEntryPathIndex(zuluPath) < 0 {
		t.Fatal("renamed file should appear after refresh")
	}
	docsRow := pane.findEntryPathIndex(docsPath)
	if docsRow < 0 {
		t.Fatal("new directory should appear after refresh")
	}
	if entry := pane.model.Entry(docsRow); entry == nil || entry.Kind != filesys.EntryDir {
		t.Fatalf("docs entry = %#v, want directory", entry)
	}
	if sel := pane.selectedEntry(); sel == nil || sel.Path != deltaPath {
		t.Fatalf("selected path = %#v, want %q", sel, deltaPath)
	}
	charlieRow = pane.findEntryPathIndex(charliePath)
	if charlieRow < 0 {
		t.Fatal("charlie row missing after refresh")
	}
	if !pane.isMarkedRow(charlieRow) {
		t.Fatal("marked file should stay marked after refresh")
	}
	if got := pane.table.List.Position.First; got != charlieRow {
		t.Fatalf("first visible row = %d, want %d to keep anchor path stable", got, charlieRow)
	}
}
