// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hexone/filesys"
	"hexone/fm"
)

func TestFilePaneContextMenuSpecForArchiveFileIncludesExtractHere(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	pane.model = &filePaneModel{
		entries: []filesys.Entry{{
			Path:        "/tmp/bundle.zip",
			DisplayName: "bundle.zip",
			Kind:        filesys.EntryFile,
			CanEnter:    true,
		}},
		cfg: ui.fmCfg,
	}
	pane.ctxMenuRow = 0

	spec := ui.filePaneContextMenuSpec(0, pane)
	assertMenuHasLabel(t, spec.Items, "Extract here")
}

func TestFilePaneContextMenuSpecForArchiveMemberIsReadOnly(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"docs/readme.txt": "hello",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	pane.dir = archivePath
	pane.model = &filePaneModel{
		entries: []filesys.Entry{{
			Path:        filepath.Join(archivePath, "docs", "readme.txt"),
			DisplayName: "readme.txt",
			Kind:        filesys.EntryFile,
		}},
		cfg: ui.fmCfg,
	}
	pane.ctxMenuRow = 0

	spec := ui.filePaneContextMenuSpec(0, pane)
	assertMenuMissingLabel(t, spec.Items, "Rename")
	assertMenuMissingLabel(t, spec.Items, "Permissions..")
	assertMenuMissingLabel(t, spec.Items, "Open With")

	ops := assertMenuHasLabel(t, spec.Items, "File Ops")
	if ops.Submenu == nil {
		t.Fatal("File Ops submenu should exist")
	}
	assertMenuHasLabel(t, ops.Submenu.Items, "Copy..")
	assertMenuMissingLabel(t, ops.Submenu.Items, "Move..")
	assertMenuMissingLabel(t, ops.Submenu.Items, "Delete..")
}

func TestDoubleClickArchiveFileNavigatesIntoArchive(t *testing.T) {
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
	pane.table.Selected = 0

	if !ui.activateFilePaneDoubleClick(0, 0) {
		t.Fatal("activateFilePaneDoubleClick returned false")
	}
	if !pane.loading {
		t.Fatal("archive file double click should trigger navigation load")
	}
	if got, want := pane.loadingDir, archivePath; got != want {
		t.Fatalf("loading dir = %q, want %q", got, want)
	}
}

func TestRunCopyBetweenEndpointsCopiesArchiveMember(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"docs/readme.txt": "hello archive",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	dstDir := t.TempDir()
	srcPath := filepath.Join(archivePath, "docs", "readme.txt")
	dstPath := filepath.Join(dstDir, "readme.txt")

	err := runCopyBetweenEndpoints(
		copyEndpoint{dir: archivePath, archive: true},
		srcPath,
		copyEndpoint{dir: dstDir},
		dstPath,
		nil,
	)
	if err != nil {
		t.Fatalf("runCopyBetweenEndpoints: %v", err)
	}
	data, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if got := string(data); got != "hello archive" {
		t.Fatalf("copied content = %q, want %q", got, "hello archive")
	}
}

func TestReadViewerFileReadsArchiveMember(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeArchiveSupportTestZip(archivePath, map[string]string{
		"docs/readme.txt": "hello archive",
	}); err != nil {
		t.Fatalf("writeArchiveSupportTestZip: %v", err)
	}

	memberPath := filepath.Join(archivePath, "docs", "readme.txt")
	content, status, errText, _ := readViewerFile(memberPath, fm.ViewerFileEncodingAuto, 1<<20, time.Now(), nil)
	if errText != "" {
		t.Fatalf("readViewerFile err = %q", errText)
	}
	if content != "hello archive" {
		t.Fatalf("content = %q, want %q", content, "hello archive")
	}
	if status != "archive entry: 13 bytes" {
		t.Fatalf("status = %q, want %q", status, "archive entry: 13 bytes")
	}
}

func writeArchiveSupportTestZip(dst string, files map[string]string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			zw.Close()
			return err
		}
		if _, err := w.Write([]byte(body)); err != nil {
			zw.Close()
			return err
		}
	}
	return zw.Close()
}
