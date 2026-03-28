// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

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

func TestFilePaneMarkCurrentAndAdvanceTogglesCurrentRowOff(t *testing.T) {
	pane := newFilePaneState(".", nil)
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta.txt", Path: "beta.txt"},
			{Name: "gamma.txt", Path: "gamma.txt"},
		},
	}, "", "", 0)

	if !pane.markCurrentAndAdvance() {
		t.Fatal("first markCurrentAndAdvance should mark and advance")
	}
	pane.table.Selected = 0
	if !pane.markCurrentAndAdvance() {
		t.Fatal("second markCurrentAndAdvance should unmark and advance")
	}
	if pane.isMarkedRow(0) {
		t.Fatal("current row should be unmarked after toggling it off")
	}
	if got, want := pane.table.Selected, 1; got != want {
		t.Fatalf("selected row = %d, want %d", got, want)
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

func TestFilePaneToggleMarkAllSelectableTogglesOnAndOff(t *testing.T) {
	pane := newFilePaneState(".", nil)
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "..", Path: "..", Kind: filesys.EntryParent},
			{Name: "docs", Path: "docs", Kind: filesys.EntryDir},
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta", Path: "beta"},
		},
	}, "", "", 1)

	if !pane.toggleMarkAllSelectable() {
		t.Fatal("toggleMarkAllSelectable should mark all selectable rows")
	}
	if pane.isMarkedRow(0) {
		t.Fatal("parent row should never be marked")
	}
	for _, row := range []int{1, 2, 3} {
		if !pane.isMarkedRow(row) {
			t.Fatalf("row %d should be marked", row)
		}
	}

	if !pane.toggleMarkAllSelectable() {
		t.Fatal("toggleMarkAllSelectable should clear when everything is already marked")
	}
	if pane.hasMarkedRows() {
		t.Fatal("all marks should be cleared after toggling select all off")
	}
}

func TestFilePaneToggleMarkRowsMatchingCurrentSelectionMatchesExtensionAndTogglesOff(t *testing.T) {
	pane := newFilePaneState(".", nil)
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "alpha.txt", Path: "alpha.txt"},
			{Name: "beta.log", Path: "beta.log"},
			{Name: "gamma.txt", Path: "gamma.txt"},
		},
	}, "", "", 0)

	if !pane.toggleMarkRowsMatchingCurrentSelection() {
		t.Fatal("toggleMarkRowsMatchingCurrentSelection should mark matching extensions")
	}
	if !pane.isMarkedRow(0) || !pane.isMarkedRow(2) {
		t.Fatal(".txt rows should be marked together")
	}
	if pane.isMarkedRow(1) {
		t.Fatal(".log row should not be marked with .txt selection")
	}

	if !pane.toggleMarkRowsMatchingCurrentSelection() {
		t.Fatal("toggleMarkRowsMatchingCurrentSelection should clear matching rows when already selected")
	}
	if pane.hasMarkedRows() {
		t.Fatal("matching extension rows should be cleared on second toggle")
	}
}

func TestFilePaneToggleMarkRowsMatchingCurrentSelectionMatchesExtensionlessFiles(t *testing.T) {
	pane := newFilePaneState(".", nil)
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "README", Path: "README"},
			{Name: "docs", Path: "docs", Kind: filesys.EntryDir},
			{Name: "LICENSE", Path: "LICENSE"},
			{Name: "notes.txt", Path: "notes.txt"},
		},
	}, "", "", 0)
	pane.table.Selected = pane.findEntryPathIndex("README")

	if !pane.toggleMarkRowsMatchingCurrentSelection() {
		t.Fatal("toggleMarkRowsMatchingCurrentSelection should mark extensionless files")
	}
	if !pane.isMarkedRow(pane.findEntryPathIndex("README")) || !pane.isMarkedRow(pane.findEntryPathIndex("LICENSE")) {
		t.Fatal("extensionless files should be marked together")
	}
	if pane.isMarkedRow(pane.findEntryPathIndex("docs")) {
		t.Fatal("directories should not be grouped with extensionless files")
	}
	if pane.isMarkedRow(pane.findEntryPathIndex("notes.txt")) {
		t.Fatal("files with an extension should not be marked with extensionless files")
	}
}

func TestFilePaneToggleMarkRowsMatchingCurrentSelectionMatchesDirectories(t *testing.T) {
	pane := newFilePaneState(".", nil)
	pane.applyListing(filesys.Listing{
		Dir: ".",
		Entries: []filesys.Entry{
			{Name: "docs", Path: "docs", Kind: filesys.EntryDir},
			{Name: "README", Path: "README"},
			{Name: "src", Path: "src", Kind: filesys.EntryDir},
			{Name: "notes.txt", Path: "notes.txt"},
		},
	}, "", "", 0)
	pane.table.Selected = pane.findEntryPathIndex("docs")

	if !pane.toggleMarkRowsMatchingCurrentSelection() {
		t.Fatal("toggleMarkRowsMatchingCurrentSelection should mark directories together")
	}
	if !pane.isMarkedRow(pane.findEntryPathIndex("docs")) || !pane.isMarkedRow(pane.findEntryPathIndex("src")) {
		t.Fatal("all directories should be marked together")
	}
	if pane.isMarkedRow(pane.findEntryPathIndex("README")) || pane.isMarkedRow(pane.findEntryPathIndex("notes.txt")) {
		t.Fatal("non-directory rows should not be marked with directories")
	}
}
