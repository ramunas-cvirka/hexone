// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"path/filepath"
	"testing"

	"hexone/filesys"
	"hexone/fm"
)

func TestFilePaneWildcardFilterKeepsNavigationRows(t *testing.T) {
	dir := t.TempDir()
	pane := newFilePaneState(dir, fm.DefaultConfig())
	pane.applyListing(filesys.Listing{Dir: dir, Entries: []filesys.Entry{
		{Name: "..", DisplayName: "..", Kind: filesys.EntryParent, Path: filepath.Dir(dir)},
		{Name: "src", DisplayName: "src", Kind: filesys.EntryDir, Path: filepath.Join(dir, "src")},
		{Name: "main.go", DisplayName: "main.go", Kind: filesys.EntryFile, Path: filepath.Join(dir, "main.go")},
		{Name: "README.md", DisplayName: "README.md", Kind: filesys.EntryFile, Path: filepath.Join(dir, "README.md")},
		{Name: "LICENSE", DisplayName: "LICENSE", Kind: filesys.EntryFile, Path: filepath.Join(dir, "LICENSE")},
	}}, "", "", 0)

	if err := pane.setFilter("*.go;*.md"); err != nil {
		t.Fatalf("set filter: %v", err)
	}

	want := []string{"..", "src", "main.go", "README.md"}
	if pane.model.Len() != len(want) {
		t.Fatalf("filtered rows=%d want %d: %#v", pane.model.Len(), len(want), pane.model.entries)
	}
	for i, name := range want {
		if got := pane.model.entries[i].Name; got != name {
			t.Fatalf("row %d name=%q want %q", i, got, name)
		}
	}
}

func TestFilePaneRegexFilterAndDefaultMask(t *testing.T) {
	dir := t.TempDir()
	pane := newFilePaneState(dir, fm.DefaultConfig())
	pane.applyListing(filesys.Listing{Dir: dir, Entries: []filesys.Entry{
		{Name: "alpha-12.txt", DisplayName: "alpha-12.txt", Path: filepath.Join(dir, "alpha-12.txt")},
		{Name: "alpha.txt", DisplayName: "alpha.txt", Path: filepath.Join(dir, "alpha.txt")},
		{Name: "LICENSE", DisplayName: "LICENSE", Path: filepath.Join(dir, "LICENSE")},
	}}, "", "", 0)

	if err := pane.setFilter(`re:^alpha-[0-9]+\.txt$`); err != nil {
		t.Fatalf("set regex filter: %v", err)
	}
	if got, want := pane.model.Len(), 1; got != want || pane.model.entries[0].Name != "alpha-12.txt" {
		t.Fatalf("regex rows=%#v", pane.model.entries)
	}

	if err := pane.setFilter(""); err != nil {
		t.Fatalf("clear filter: %v", err)
	}
	if got, want := pane.displayFilter(), filePaneDefaultFilter; got != want {
		t.Fatalf("default filter=%q want %q", got, want)
	}
	if got, want := pane.model.Len(), 3; got != want {
		t.Fatalf("default mask rows=%d want %d", got, want)
	}
}

func TestFilePaneInvalidFilterDoesNotReplaceActiveFilter(t *testing.T) {
	dir := t.TempDir()
	pane := newFilePaneState(dir, fm.DefaultConfig())
	pane.applyListing(filesys.Listing{Dir: dir, Entries: []filesys.Entry{
		{Name: "main.go", DisplayName: "main.go", Path: filepath.Join(dir, "main.go")},
		{Name: "README.md", DisplayName: "README.md", Path: filepath.Join(dir, "README.md")},
	}}, "", "", 0)
	if err := pane.setFilter("*.go"); err != nil {
		t.Fatalf("set initial filter: %v", err)
	}

	if err := pane.setFilter("re:["); err == nil {
		t.Fatal("invalid regex should fail")
	}
	if got, want := pane.displayFilter(), "*.go"; got != want {
		t.Fatalf("filter after invalid input=%q want %q", got, want)
	}
	if got, want := pane.model.Len(), 1; got != want || pane.model.entries[0].Name != "main.go" {
		t.Fatalf("rows changed after invalid filter: %#v", pane.model.entries)
	}
}

func TestFilePaneCombinedPathFilterEditorCountsAsPathEditing(t *testing.T) {
	pane := newFilePaneState(t.TempDir(), fm.DefaultConfig())
	ui := &UI{filePanes: []*filePaneState{pane}}
	pane.filterText = "*.go"
	pane.beginPathEdit()
	if !ui.pathEditActive() {
		t.Fatal("combined editor should suppress file-manager shortcuts")
	}
	if got, want := pane.pathEdit.Text(), filepath.Join(pane.dir, "*.go"); got != want {
		t.Fatalf("combined editor text=%q want %q", got, want)
	}
	pane.stopPathEdit()
	if ui.pathEditActive() {
		t.Fatal("closed combined editor should release file-manager shortcuts")
	}
}

func TestFilePaneCombinedEditorCancelDiscardsDraft(t *testing.T) {
	pane := newFilePaneState(t.TempDir(), fm.DefaultConfig())
	originalDir := pane.dir
	pane.filterText = "*.go"
	pane.beginPathEdit()
	pane.pathEdit.SetText(filepath.Join(string(filepath.Separator), "somewhere", "else", "*.md"))

	pane.stopPathEdit()

	if got, want := pane.displayFilter(), "*.go"; got != want {
		t.Fatalf("filter after cancel=%q want %q", got, want)
	}
	if got, want := pane.dir, originalDir; got != want {
		t.Fatalf("directory changed after cancel: %q", got)
	}
}

func TestSplitFilePanePathFilterExpressionKeepsRegexGreaterThan(t *testing.T) {
	pathText, filterText := splitFilePanePathFilterExpression(`/srv/app > re:^build > release$`, "*.go", false)
	if got, want := pathText, "/srv/app"; got != want {
		t.Fatalf("path=%q want %q", got, want)
	}
	if got, want := filterText, `re:^build > release$`; got != want {
		t.Fatalf("filter=%q want %q", got, want)
	}

	pathText, filterText = splitFilePanePathFilterExpression(`/srv/app/re:^build > release$`, "*.go", true)
	if got, want := pathText, "/srv/app"; got != want {
		t.Fatalf("OS-style path=%q want %q", got, want)
	}
	if got, want := filterText, `re:^build > release$`; got != want {
		t.Fatalf("OS-style filter=%q want %q", got, want)
	}
}

func TestSplitFilePaneOSStylePathFilterExpression(t *testing.T) {
	dir := filepath.Join(string(filepath.Separator), "srv", "app")
	pathText, filterText := splitFilePanePathFilterExpression(filepath.Join(dir, "*.go"), "*.*", false)
	if got, want := pathText, dir; got != want {
		t.Fatalf("path=%q want %q", got, want)
	}
	if got, want := filterText, "*.go"; got != want {
		t.Fatalf("filter=%q want %q", got, want)
	}
}

func TestFormatRemoteFilePanePathFilterExpression(t *testing.T) {
	if got, want := formatFilePanePathFilterExpression("/srv/app", "*.go", true), "/srv/app/*.go"; got != want {
		t.Fatalf("expression=%q want %q", got, want)
	}
}

func TestSubmitPanePathEditAppliesFilterWithCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	pane := newFilePaneState(dir, fm.DefaultConfig())
	pane.applyListing(filesys.Listing{Dir: dir, Entries: []filesys.Entry{
		{Name: "main.go", DisplayName: "main.go", Path: filepath.Join(dir, "main.go")},
		{Name: "README.md", DisplayName: "README.md", Path: filepath.Join(dir, "README.md")},
	}}, "", "", 0)
	pane.beginPathEdit()
	ui := &UI{fmCfg: fm.DefaultConfig(), filePanes: []*filePaneState{pane}}

	if !ui.submitPanePathEdit(0, filepath.Join(dir, "*.go")) {
		t.Fatal("combined path/filter submit should succeed")
	}
	if pane.pathEditing {
		t.Fatal("successful submit should leave editing mode")
	}
	if got, want := pane.displayFilter(), "*.go"; got != want {
		t.Fatalf("filter=%q want %q", got, want)
	}
	if got, want := pane.model.Len(), 1; got != want || pane.model.entries[0].Name != "main.go" {
		t.Fatalf("filtered rows=%#v", pane.model.entries)
	}
}
