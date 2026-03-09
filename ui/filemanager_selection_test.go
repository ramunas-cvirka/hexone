package ui

import (
	"hexone/filesys"
	"testing"
	"time"
)

func TestFilePaneMarkCurrentAndAdvance(t *testing.T) {
	pane := newFilePaneState(".", nil)
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta.txt", Path: "beta.txt"},
			{Name: "gamma.txt", Path: "gamma.txt"},
		},
	}, "", "", 1)

	if !pane.markCurrentAndAdvance() {
		t.Fatal("markCurrentAndAdvance should change selection state")
	}
	if pane.table.Selected != 2 {
		t.Fatalf("selected row = %d, want 2", pane.table.Selected)
	}
	if !pane.isMarkedRow(1) {
		t.Fatal("row 1 should be marked after Insert-style selection")
	}

	selected := pane.selectedEntriesForAction()
	if len(selected) != 1 || selected[0].Path != "beta.txt" {
		t.Fatalf("selected entries = %#v, want beta.txt only", selected)
	}
}

func TestFilePaneReplaceMarkedRangeSkipsParent(t *testing.T) {
	pane := newFilePaneState(".", nil)
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "..", Path: "..", Kind: filesys.EntryParent},
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta.txt", Path: "beta.txt"},
		},
	}, "", "", 0)

	if !pane.replaceMarkedRange(0, 2) {
		t.Fatal("replaceMarkedRange should mark selectable rows")
	}
	if pane.isMarkedRow(0) {
		t.Fatal("parent row should not be marked")
	}
	if !pane.isMarkedRow(1) || !pane.isMarkedRow(2) {
		t.Fatal("range selection should mark rows 1 and 2")
	}

	selected := pane.selectedEntriesForAction()
	if len(selected) != 2 {
		t.Fatalf("selected entry count = %d, want 2", len(selected))
	}
	if selected[0].Path != "alpha.txt" || selected[1].Path != "beta.txt" {
		t.Fatalf("selected entries = %#v, want alpha.txt and beta.txt", selected)
	}
}

func TestFilePaneSelectedEntriesFallbackToCurrentRow(t *testing.T) {
	pane := newFilePaneState(".", nil)
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta.txt", Path: "beta.txt"},
		},
	}, "", "", 1)

	selected := pane.selectedEntriesForAction()
	if len(selected) != 1 {
		t.Fatalf("selected entry count = %d, want 1", len(selected))
	}
	if selected[0].Path != "beta.txt" {
		t.Fatalf("selected path = %q, want beta.txt", selected[0].Path)
	}
}

func TestFilePaneApplyListingClearsTableClickState(t *testing.T) {
	pane := newFilePaneState(".", nil)
	pane.tableClickRow = 3
	pane.tableClickCol = 0
	pane.tableClickAt = time.Now()

	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "..", Path: "..", Kind: filesys.EntryParent},
		},
	}, "", "", 0)

	if pane.tableClickRow != -1 || pane.tableClickCol != -1 {
		t.Fatalf("table click state = row %d col %d, want cleared", pane.tableClickRow, pane.tableClickCol)
	}
	if !pane.tableClickAt.IsZero() {
		t.Fatal("table click time should be cleared")
	}
}
