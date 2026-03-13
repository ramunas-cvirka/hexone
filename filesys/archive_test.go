// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveNameSupported(t *testing.T) {
	if !ArchiveNameSupported("bundle.zip") {
		t.Fatal("zip archive should be supported")
	}
	if !ArchiveNameSupported("bundle.tar.gz") {
		t.Fatal("tar.gz archive should be supported")
	}
	if !ArchiveNameSupported("bundle.rar") {
		t.Fatal("rar archive should be supported")
	}
	if !ArchiveNameSupported("bundle.7z") {
		t.Fatal("7z archive should be supported")
	}
	if ArchiveNameSupported("notes.txt.gz") {
		t.Fatal("compressed regular file should not be treated as archive")
	}
}

func TestParseArchivePath(t *testing.T) {
	archivePath := filepath.Join(string(filepath.Separator), "tmp", "sample.zip")
	loc, ok := ParseArchivePath(filepath.Join(archivePath, "docs", "readme.txt"))
	if !ok {
		t.Fatal("expected archive path to be recognized")
	}
	if loc.ArchivePath != archivePath {
		t.Fatalf("archive path = %q, want %q", loc.ArchivePath, archivePath)
	}
	if loc.InnerPath != "docs/readme.txt" {
		t.Fatalf("inner path = %q, want %q", loc.InnerPath, "docs/readme.txt")
	}
}

func TestReadDirArchiveRootAndNestedPath(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeTestArchive(archivePath, map[string]string{
		"docs/readme.txt": "hello",
		"top.txt":         "root",
	}); err != nil {
		t.Fatalf("writeTestArchive: %v", err)
	}

	rootListing, err := ReadDir(archivePath)
	if err != nil {
		t.Fatalf("ReadDir archive root: %v", err)
	}
	if got, want := rootListing.Dir, archivePath; got != want {
		t.Fatalf("listing dir = %q, want %q", got, want)
	}
	if len(rootListing.Entries) < 3 {
		t.Fatalf("root entries len = %d, want at least 3", len(rootListing.Entries))
	}
	if rootListing.Entries[0].Kind != EntryParent {
		t.Fatalf("first entry kind = %v, want parent", rootListing.Entries[0].Kind)
	}
	if rootListing.Entries[1].Name != "docs" || rootListing.Entries[1].Kind != EntryDir || !rootListing.Entries[1].CanEnter {
		t.Fatalf("docs entry = %#v, want enterable directory", rootListing.Entries[1])
	}
	if rootListing.Entries[2].Name != "top.txt" || rootListing.Entries[2].Kind != EntryFile {
		t.Fatalf("top entry = %#v, want top.txt file", rootListing.Entries[2])
	}

	nestedPath := filepath.Join(archivePath, "docs")
	nestedListing, err := ReadDir(nestedPath)
	if err != nil {
		t.Fatalf("ReadDir nested archive dir: %v", err)
	}
	if got, want := nestedListing.Dir, nestedPath; got != want {
		t.Fatalf("nested listing dir = %q, want %q", got, want)
	}
	if len(nestedListing.Entries) != 2 {
		t.Fatalf("nested entries len = %d, want 2", len(nestedListing.Entries))
	}
	if nestedListing.Entries[0].Kind != EntryParent || nestedListing.Entries[0].Path != archivePath {
		t.Fatalf("nested parent entry = %#v, want archive root parent", nestedListing.Entries[0])
	}
	if nestedListing.Entries[1].Path != filepath.Join(archivePath, "docs", "readme.txt") {
		t.Fatalf("nested file path = %q", nestedListing.Entries[1].Path)
	}
}

func TestOpenLocalPathAndReadChunkArchiveMember(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	if err := writeTestArchive(archivePath, map[string]string{
		"docs/readme.txt": "hello archive",
	}); err != nil {
		t.Fatalf("writeTestArchive: %v", err)
	}

	memberPath := filepath.Join(archivePath, "docs", "readme.txt")
	file, info, err := OpenLocalPath(memberPath)
	if err != nil {
		t.Fatalf("OpenLocalPath: %v", err)
	}
	file.Close()
	if info.IsDir() {
		t.Fatal("archive member should be a file")
	}

	chunk, size, err := ReadLocalFileChunk(memberPath, 6, 7)
	if err != nil {
		t.Fatalf("ReadLocalFileChunk: %v", err)
	}
	if got := string(chunk); got != "archive" {
		t.Fatalf("chunk = %q, want %q", got, "archive")
	}
	if size != int64(len("hello archive")) {
		t.Fatalf("size = %d, want %d", size, len("hello archive"))
	}
}

func writeTestArchive(dst string, files map[string]string) error {
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
