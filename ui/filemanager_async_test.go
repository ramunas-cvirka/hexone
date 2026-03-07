package ui

import (
	"hexone/filesys"
	"testing"
	"time"
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
