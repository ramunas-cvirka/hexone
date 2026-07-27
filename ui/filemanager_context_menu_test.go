// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"hexone/filesys"
	"hexone/fm"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func TestFilePaneContextMenuSpecForFileUsesNestedSupportedActions(t *testing.T) {
	prevManagerName := systemFileManagerNameFunc
	systemFileManagerNameFunc = func() string { return "Test Manager" }
	defer func() { systemFileManagerNameFunc = prevManagerName }()

	cfg := fm.DefaultConfig()
	cfg.Associations = []fm.AssociationProgram{
		{AppPath: "/Applications/VLC.app", Extensions: []string{".mp4"}},
	}

	ui := NewUI(cfg)
	pane := ui.filePanes[0]
	pane.model = &filePaneModel{
		entries: []filesys.Entry{{
			Path:        "/tmp/movie.mp4",
			DisplayName: "movie.mp4",
			Kind:        filesys.EntryFile,
		}},
		cfg: cfg,
	}
	pane.ctxMenuRow = 0

	spec := ui.filePaneContextMenuSpec(0, pane)
	assertMenuHasLabel(t, spec.Items, "Open")
	assertMenuHasLabel(t, spec.Items, "Reveal in Test Manager")
	openWith := assertMenuHasLabel(t, spec.Items, "Open With")
	if openWith.Submenu == nil {
		t.Fatal("Open With submenu should exist for local files")
	}
	assertMenuHasLabel(t, spec.Items, "Rename")
	ops := assertMenuHasLabel(t, spec.Items, "File Ops")
	if ops.Submenu == nil {
		t.Fatal("File Ops submenu should exist")
	}
	assertMenuHasLabel(t, spec.Items, "Copy Path")
	assertMenuHasLabel(t, spec.Items, "Permissions..")
	assertMenuMissingLabel(t, spec.Items, "Properties")

	assertMenuHasLabel(t, openWith.Submenu.Items, "System Default")
	assertMenuHasLabel(t, ops.Submenu.Items, "Copy..")
	assertMenuHasLabel(t, ops.Submenu.Items, "Move..")
	assertMenuHasLabel(t, ops.Submenu.Items, "Delete..")
}

func TestFilePaneContextMenuSpecForDirectoryIncludesSystemFileManager(t *testing.T) {
	prevManagerName := systemFileManagerNameFunc
	systemFileManagerNameFunc = func() string { return "Test Manager" }
	defer func() { systemFileManagerNameFunc = prevManagerName }()

	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	pane.model = &filePaneModel{
		entries: []filesys.Entry{{
			Path:        "/tmp/docs",
			DisplayName: "docs",
			Kind:        filesys.EntryDir,
		}},
		cfg: ui.fmCfg,
	}
	pane.ctxMenuRow = 0

	spec := ui.filePaneContextMenuSpec(0, pane)
	assertMenuHasLabel(t, spec.Items, "Open in Test Manager")
	assertMenuMissingLabel(t, spec.Items, "Reveal in Test Manager")
}

func TestFilePaneContextMenuOffersNativeCopyAndDetectedPasteFiles(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	pane.model = &filePaneModel{
		entries: []filesys.Entry{{
			Path:        "/tmp/docs",
			DisplayName: "docs",
			Kind:        filesys.EntryDir,
		}},
		cfg: ui.fmCfg,
	}
	pane.table.Selected = 0
	pane.ctxMenuRow = 0
	pane.ctxMenuClipboardFileCount = 3

	spec := ui.filePaneContextMenuSpec(0, pane)
	assertMenuHasLabel(t, spec.Items, "Copy File")
	assertMenuHasLabel(t, spec.Items, "Paste 3 Files")
}

func TestFilePaneBackgroundContextMenuOnlyShowsPasteForFileClipboard(t *testing.T) {
	oldRead := readFileClipboardFilesFunc
	defer func() { readFileClipboardFilesFunc = oldRead }()

	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	readFileClipboardFilesFunc = func() ([]string, error) {
		return []string{"/tmp/one.txt"}, nil
	}

	ui.openFilePaneContextMenu(0, -1, image.Point{}, time.Now())
	spec := ui.filePaneContextMenuSpec(0, pane)
	assertMenuHasLabel(t, spec.Items, "Paste File")

	pane.closeContextMenu()
	readFileClipboardFilesFunc = func() ([]string, error) {
		return nil, errors.New("clipboard unavailable")
	}
	ui.openFilePaneContextMenu(0, -1, image.Point{}, time.Now())
	spec = ui.filePaneContextMenuSpec(0, pane)
	assertMenuMissingLabel(t, spec.Items, "Paste File")
	assertMenuMissingLabel(t, spec.Items, "Paste Files")
}

func TestFilePaneCopyFilesActionWritesMarkedLocalPaths(t *testing.T) {
	oldWrite := writeFileClipboardFilesFunc
	defer func() { writeFileClipboardFilesFunc = oldWrite }()

	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	pane.model = &filePaneModel{
		entries: []filesys.Entry{
			{Path: "/tmp/one.txt", DisplayName: "one.txt", Kind: filesys.EntryFile},
			{Path: "/tmp/two.txt", DisplayName: "two.txt", Kind: filesys.EntryFile},
		},
		cfg: ui.fmCfg,
	}
	pane.markRow(0)
	pane.markRow(1)
	var got []string
	writeFileClipboardFilesFunc = func(paths []string) error {
		got = append([]string(nil), paths...)
		return nil
	}

	ui.handleFilePaneContextMenuAction(0, pane, 0, filePaneMenuActionCopyFiles, time.Now())

	if len(got) != 2 || got[0] != "/tmp/one.txt" || got[1] != "/tmp/two.txt" {
		t.Fatalf("clipboard paths=%v want both marked paths", got)
	}
	if pane.noticeText != "2 files copied to clipboard" {
		t.Fatalf("notice=%q", pane.noticeText)
	}
}

func TestFilePanePasteFilesActionStartsCopyImmediately(t *testing.T) {
	oldRead := readFileClipboardFilesFunc
	defer func() { readFileClipboardFilesFunc = oldRead }()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "sample.txt")
	if err := os.WriteFile(src, []byte("sample"), 0o600); err != nil {
		t.Fatal(err)
	}

	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	pane.dir = dstDir
	readFileClipboardFilesFunc = func() ([]string, error) {
		return []string{src}, nil
	}

	actionNow := time.Now()
	ui.handleFilePaneContextMenuAction(0, pane, -1, filePaneMenuActionPasteFiles, actionNow)

	if ui.fileCopy == nil {
		t.Fatal("paste should start the copy workflow")
	}
	if !ui.fileCopy.running {
		t.Fatal("paste should start copying without waiting for modal confirmation")
	}
	if !ui.fileCopy.directPaste {
		t.Fatal("paste should use direct-paste progress behavior")
	}
	if got := ui.fileCopy.srcPath; got != src {
		t.Fatalf("copy source=%q want %q", got, src)
	}
	if got := ui.fileCopy.dstPath; got != filepath.Join(dstDir, "sample.txt") {
		t.Fatalf("copy destination=%q", got)
	}
	var ops op.Ops
	dims := ui.layoutFileCopyDialog(material.NewTheme(), layout.Context{
		Ops:         &ops,
		Constraints: layout.Exact(image.Pt(800, 600)),
		Now:         actionNow.Add(10 * time.Second),
	})
	if dims.Size != (image.Point{}) {
		t.Fatalf("direct paste displayed a confirmation modal: size=%v", dims.Size)
	}
	select {
	case err := <-ui.fileCopy.doneCh:
		if err != nil {
			t.Fatalf("direct paste failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("direct paste did not finish")
	}
	if data, err := os.ReadFile(filepath.Join(dstDir, "sample.txt")); err != nil || string(data) != "sample" {
		t.Fatalf("pasted file data=%q err=%v", data, err)
	}
}

func TestFilePanePasteFilesActionDoesNotOverwriteWithoutConfirmation(t *testing.T) {
	oldRead := readFileClipboardFilesFunc
	defer func() { readFileClipboardFilesFunc = oldRead }()

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "sample.txt")
	dst := filepath.Join(dstDir, "sample.txt")
	if err := os.WriteFile(src, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	pane.dir = dstDir
	readFileClipboardFilesFunc = func() ([]string, error) {
		return []string{src}, nil
	}

	ui.handleFilePaneContextMenuAction(0, pane, -1, filePaneMenuActionPasteFiles, time.Now())

	if ui.fileCopy != nil {
		t.Fatal("conflicting direct paste should not start an overwrite")
	}
	if data, err := os.ReadFile(dst); err != nil || string(data) != "existing" {
		t.Fatalf("existing destination changed: data=%q err=%v", data, err)
	}
	if pane.noticeText != "paste failed: sample.txt already exists" {
		t.Fatalf("notice=%q", pane.noticeText)
	}
}

func TestFileOpenWithAppsForPathSortsBestMatchFirst(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Associations = []fm.AssociationProgram{
		{AppPath: "/Applications/Gzip.app", Extensions: []string{".gz"}},
		{AppPath: "/Applications/Archive.app", Extensions: []string{".tar.gz"}},
		{AppPath: "/Applications/Text.app", Extensions: []string{".txt"}},
	}

	apps := fileOpenWithAppsForPath("/tmp/archive.tar.gz", cfg)
	if len(apps) != 3 {
		t.Fatalf("apps len=%d want 3", len(apps))
	}
	if got := apps[0].AppPath; got != "/Applications/Archive.app" {
		t.Fatalf("best match=%q want archive app", got)
	}
	if got := apps[1].AppPath; got != "/Applications/Gzip.app" {
		t.Fatalf("second match=%q want gzip app", got)
	}
}

func TestFilePaneContextCopyPathRemoteUsesSSHURL(t *testing.T) {
	cfg := fm.DefaultConfig()
	pane := newFilePaneState("/", cfg)
	pane.remote = &paneSSHSession{
		setup: fm.SSHSetup{
			User: "ramunas",
			Host: "example.test",
			Port: 2222,
		},
	}
	pane.dir = "/var/log"

	ui := &UI{
		fmCfg:     cfg,
		filePanes: []*filePaneState{pane},
	}

	got := ui.filePaneContextCopyPath(0, -1)
	want := "ssh://ramunas@example.test:2222/var/log"
	if got != want {
		t.Fatalf("copy path=%q want %q", got, want)
	}
}

func TestFileContextMenuParentRectRejectsStalePath(t *testing.T) {
	pane := &filePaneState{
		ctxMenuPath:      nil,
		ctxMenuItemRects: map[string]image.Rectangle{"open-with": image.Rect(10, 20, 40, 60)},
	}
	if rect, ok := fileContextMenuParentRect(pane, 1); ok {
		t.Fatalf("stale parent rect=%v should not resolve without a matching path", rect)
	}
}

func TestFileContextMenuParentRectReturnsMappedRect(t *testing.T) {
	want := image.Rect(10, 20, 40, 60)
	pane := &filePaneState{
		ctxMenuPath:      []string{"open-with"},
		ctxMenuItemRects: map[string]image.Rectangle{"open-with": want},
	}
	got, ok := fileContextMenuParentRect(pane, 1)
	if !ok {
		t.Fatal("parent rect should resolve for a valid submenu path")
	}
	if got != want {
		t.Fatalf("parent rect=%v want %v", got, want)
	}
}

func TestOpenContextMenuResetsItemClickables(t *testing.T) {
	pane := &filePaneState{
		ctxMenuClicks: map[string]*widget.Clickable{
			"system-file-manager": {},
		},
	}

	pane.openContextMenu(3, image.Pt(40, 60), time.Date(2026, time.March, 12, 10, 0, 0, 0, time.UTC))

	if pane.ctxMenuClicks != nil {
		t.Fatal("openContextMenu should reset item clickables")
	}
}

func TestCloseContextMenuResetsItemClickables(t *testing.T) {
	pane := &filePaneState{
		ctxMenuOpen: true,
		ctxMenuClicks: map[string]*widget.Clickable{
			"system-file-manager": {},
		},
	}

	pane.closeContextMenu()

	if pane.ctxMenuClicks != nil {
		t.Fatal("closeContextMenu should reset item clickables")
	}
}

func assertMenuHasLabel(t *testing.T, items []fileContextMenuItem, label string) fileContextMenuItem {
	t.Helper()
	for _, item := range items {
		if item.Separator {
			continue
		}
		if item.Label == label {
			return item
		}
	}
	t.Fatalf("missing menu item %q", label)
	return fileContextMenuItem{}
}

func assertMenuMissingLabel(t *testing.T, items []fileContextMenuItem, label string) {
	t.Helper()
	for _, item := range items {
		if item.Separator {
			continue
		}
		if item.Label == label {
			t.Fatalf("unexpected menu item %q", label)
		}
	}
}
