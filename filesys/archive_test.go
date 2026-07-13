// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"io"
	"io/fs"
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

func TestOpenArchiveFSReusesIndexedArchive(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "cached.zip")
	if err := writeTestArchive(archivePath, map[string]string{"docs/readme.txt": "hello"}); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	loc := ArchivePath{ArchivePath: archivePath, InnerPath: "."}
	first, err := openArchiveFSForLocation(loc)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if _, err := fs.ReadDir(first, "."); err != nil {
		t.Fatalf("index archive: %v", err)
	}
	second, err := openArchiveFSForLocation(loc)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	if first != second {
		t.Fatal("unchanged archive should reuse its indexed filesystem")
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

func TestOpenMultiVolumeRarArchiveMember(t *testing.T) {
	tests := []struct {
		name   string
		first  string
		second string
	}{
		{name: "new-style part rar", first: "test.part01.rar", second: "test.part02.rar"},
		{name: "old-style r00", first: "test.rar", second: "test.r00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			archivePath := writeTestMultiVolumeRar(t, root, tt.first, tt.second)

			listing, err := ReadDir(archivePath)
			if err != nil {
				t.Fatalf("ReadDir multi-volume rar: %v", err)
			}
			if len(listing.Entries) < 2 || listing.Entries[1].Name != "test.txt" {
				t.Fatalf("root entries = %#v, want test.txt", listing.Entries)
			}

			memberPath := filepath.Join(archivePath, "test.txt")
			file, info, err := OpenLocalPath(memberPath)
			if err != nil {
				t.Fatalf("OpenLocalPath multi-volume rar member: %v", err)
			}
			defer file.Close()
			if info.IsDir() {
				t.Fatal("rar member should be a file")
			}

			h := sha1.New()
			if _, err := io.Copy(h, file); err != nil {
				t.Fatalf("read multi-volume rar member: %v", err)
			}
			if got, want := hex.EncodeToString(h.Sum(nil)), "4da7f88f69b44a3fdb705667019a65f4c6e058a3"; got != want {
				t.Fatalf("sha1 = %s, want %s", got, want)
			}
		})
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

func writeTestMultiVolumeRar(t *testing.T, dir, first, second string) string {
	t.Helper()
	archivePath := filepath.Join(dir, first)
	writeBase64File(t, archivePath, testMultiVolumeRarPart01)
	writeBase64File(t, filepath.Join(dir, second), testMultiVolumeRarPart02)
	return archivePath
}

func writeBase64File(t *testing.T, path, encoded string) {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("DecodeString(%q): %v", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q): %v", filepath.Base(path), err)
	}
}

const testMultiVolumeRarPart01 = "UmFyIRoHAQBt4SgnCwEFBwEGAQGAgIAAEP5UsygCEwvhhgAEv8UApIMC4JlDmoADAQh0ZXN0LnR4dAoDEyO9GGi6v4cPz7QlBEVUMzUFU/JQNHL7mGuFIr5z/J6B4bcvXfL11LjidU6p04HF6D3xMapGjQBCKOxFtYcNiyPkggDd6gOAjCMD/5QACQANA/frHrT1r6z629b+uPXPrr1jyJ3Fx3Gx3Hx3Ix3Jx3Kxx+bmdzdJp5RtpNJpNJpNJpNZrNfPaW1ms1ms1mszMzMz5yLZmZmZmZtNptNpt588ttNptNpvN5vN5vN/PrbbzebzicTicTicTjzpluJxOZzOZzOZzOZz52u3M6nU6nU6nU6nU6nXgL7BnvsG++w899hHvsKe+wn32HvvsK++wt782/0bXeDjwc+D94OvB/8Hfg87Hvc7zb6TSaTSaTSaTSaTWazWazWazWazWazMzMzMzMzMzMzM2m02m02m36/jzH6fj57+I+n9jaf8f2/p5j+//juvx83/+x/K7g/r/v7r/pp/T/jDv+/PW/fbg+Ly9rzeMC+MDeMD+MEeME+MHvGCvGD/jBf1gz6wb4w8+sI+sKfWE/WHv1hX6wt9YX+sMeMM97r7QxMTExMTExMTE0mk0mk0mk0mk0mk1ms1ms1ms1ms1mszMzMzMzMzMzMzNptNptNptNptNptN5vN5vN5vN5vN5vOJxOJxOJxOJxOJxOZzOZzOZzOZzOZzOp1Op1Op1Op1Op5fOJiYmJiYmJiYn+vwF13j7QxMTExMTExMTy+cTExMTExMTExMTExMTExMTExMTExMTExMTExMTExMTExMTE7yBBiIECBAgQIECBAgQIECBAgQIFdp8Wutda611rrXWutdYECBAgRpBGk0mk0mk0hpECBAgQIECBAgQIECBAgQIECBAgQIECBAgQIECBAgV2vxa611rrXWutda611gRiCBAg1iBAgQI1msFbzLPx9oazWazWeXygQIECBAgQIECBAgQIECBAgQIECBAgQIECBXZ+LXWutda611rrXWusCBAgQIECBAgQIECBAjMzMwRmZmZmGYgQIECBAgQIECBAgQIECBAgQIECBArtvi11rrXWutda611rrAgRiCBBtECBAgQIECBAgQIECBG02m02m0NvL52m07y+0BAgQIECBAgQIECBAgQIECBArt/i11rrXWutda611rrAgQIECBAgQIECBAgQIECBAgQIECBAgQIItHUSYDBQQBAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="

const testMultiVolumeRarPart02 = "UmFyIRoHAQCpwsTKDAEFBwMBBgEBgICAAKB9hWAoAgsLyIEABL/FAKSDApFhDOCAAwEIdGVzdC50eHQKAxMjvRhour+HD0G8QbxAgQIECBAgQIECBAgV3Hxa611rrXWutda611gQIEYgg4iBAgQIECBAgQIECBAgQIECBAgQIECBAgQI45+5faHM8vlAgQIOIgQIECu5+LXWutda611rrXWusCBAgQIECBAgQIECBAgQIECBAgQIECBAgQIECBAgQIECBAg5nfgfmBAgQdRXWutda611rrXWusCBAgRiHUQIECBAgQIECBAgQIECBAgQIECBAgQIECBAgQIECDqdTqdTqdTqdTqdTrr5dvOWHXdWUQMFBAA="
