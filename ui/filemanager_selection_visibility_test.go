// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"testing"

	"gioui.org/layout"
	"gioui.org/widget/material"
	"hexone/filesys"
)

func TestFilePaneApplyListingEnsuresSelectedPathVisible(t *testing.T) {
	pane := newFilePaneState(".", nil)

	entries := make([]filesys.Entry, 0, 32)
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("row-%02d.txt", i)
		entries = append(entries, filesys.Entry{
			Name:        name,
			DisplayName: name,
			Path:        name,
			Kind:        filesys.EntryFile,
		})
	}

	pane.applyListing(filesys.Listing{
		Dir:     ".",
		Entries: entries,
	}, "row-20.txt", "", 0)

	if got := pane.table.List.Position.First; got != 0 {
		t.Fatalf("pre-layout first row = %d, want 0 before ensure runs", got)
	}

	th := material.NewTheme()
	gtx := testPathLayoutContext()
	pane.table.Layout(th, gtx, pane.model)

	if pane.table.Selected != 20 {
		t.Fatalf("selected row = %d, want 20", pane.table.Selected)
	}
	if got := pane.table.List.Position.First; got == 0 {
		t.Fatal("selected row should be scrolled into view after layout")
	}
}

func TestFilePaneApplyListingWithRestoreKeepsExactScrollPosition(t *testing.T) {
	pane := newFilePaneState(".", nil)

	entries := make([]filesys.Entry, 0, 32)
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("row-%02d.txt", i)
		entries = append(entries, filesys.Entry{
			Name:        name,
			DisplayName: name,
			Path:        name,
			Kind:        filesys.EntryFile,
		})
	}

	restorePos := layout.Position{First: 19, Offset: 0}
	pane.rememberDirScroll(".")
	pane.navScrollByDir["."] = restorePos

	gotRestore, ok := pane.restoreDirScroll(".")
	if !ok {
		t.Fatal("restoreDirScroll should find saved position")
	}

	pane.applyListingWithRestore(filesys.Listing{
		Dir:     ".",
		Entries: entries,
	}, "row-20.txt", "", 0, gotRestore, true, "")

	th := material.NewTheme()
	gtx := testPathLayoutContext()
	pane.table.Layout(th, gtx, pane.model)

	if pane.table.Selected != 20 {
		t.Fatalf("selected row = %d, want 20", pane.table.Selected)
	}
	if got := pane.table.List.Position.First; got != restorePos.First {
		t.Fatalf("restored first row = %d, want %d", got, restorePos.First)
	}
	if got := pane.table.List.Position.Offset; got != restorePos.Offset {
		t.Fatalf("restored offset = %d, want %d", got, restorePos.Offset)
	}
}

func TestFilePaneApplyListingWithRestoreAnchorKeepsTopVisibleEntry(t *testing.T) {
	pane := newFilePaneState(".", nil)

	oldEntries := make([]filesys.Entry, 0, 32)
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("row-%02d.txt", i)
		oldEntries = append(oldEntries, filesys.Entry{
			Name:        name,
			DisplayName: name,
			Path:        name,
			Kind:        filesys.EntryFile,
		})
	}
	pane.model.entries = append([]filesys.Entry(nil), oldEntries...)

	restorePos := layout.Position{First: 10, Offset: 0}
	restoreAnchor := "row-10.txt"

	newEntries := make([]filesys.Entry, 0, 35)
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("aaa-extra-%02d.txt", i)
		newEntries = append(newEntries, filesys.Entry{
			Name:        name,
			DisplayName: name,
			Path:        name,
			Kind:        filesys.EntryFile,
		})
	}
	newEntries = append(newEntries, oldEntries...)

	pane.applyListingWithRestore(filesys.Listing{
		Dir:     ".",
		Entries: newEntries,
	}, "row-20.txt", "", 0, restorePos, true, restoreAnchor)

	th := material.NewTheme()
	gtx := testPathLayoutContext()
	pane.table.Layout(th, gtx, pane.model)

	if pane.table.Selected != 23 {
		t.Fatalf("selected row = %d, want 23", pane.table.Selected)
	}
	if got := pane.table.List.Position.First; got != 13 {
		t.Fatalf("restored first row = %d, want 13 to keep anchor path visible at top", got)
	}
}
