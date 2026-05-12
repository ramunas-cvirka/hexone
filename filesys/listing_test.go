// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReadDirLocalSymlinksKeepLinkMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("local symlink creation requires extra privileges on Windows")
	}

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "target-dir"), 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "target.txt"), []byte("target"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	if err := os.Symlink("target-dir", filepath.Join(root, "dir-link")); err != nil {
		t.Fatalf("symlink dir: %v", err)
	}
	if err := os.Symlink("target.txt", filepath.Join(root, "file-link")); err != nil {
		t.Fatalf("symlink file: %v", err)
	}
	if err := os.Symlink("missing", filepath.Join(root, "broken-link")); err != nil {
		t.Fatalf("symlink broken: %v", err)
	}

	listing, err := ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", root, err)
	}

	dirLink := requireListingEntry(t, listing, "dir-link")
	if !dirLink.IsSymlink || dirLink.LinkTarget != "target-dir" {
		t.Fatalf("dir link metadata = %#v, want symlink to target-dir", dirLink)
	}
	if dirLink.Kind != EntryDir || !dirLink.CanEnter {
		t.Fatalf("dir link entry = %#v, want enterable directory link", dirLink)
	}
	if dirLink.SizeBytes != int64(len("target-dir")) || dirLink.SizeText == "" {
		t.Fatalf("dir link size = %d %q, want link target length", dirLink.SizeBytes, dirLink.SizeText)
	}
	if got := dirLink.PermText; got == "" || got[0] != 'l' {
		t.Fatalf("dir link perms = %q, want link-prefixed symbolic permissions", got)
	}

	fileLink := requireListingEntry(t, listing, "file-link")
	if !fileLink.IsSymlink || fileLink.LinkTarget != "target.txt" {
		t.Fatalf("file link metadata = %#v, want symlink to target.txt", fileLink)
	}
	if fileLink.Kind != EntryFile || fileLink.CanEnter {
		t.Fatalf("file link entry = %#v, want non-enterable file link", fileLink)
	}
	if fileLink.SizeBytes != int64(len("target.txt")) {
		t.Fatalf("file link size = %d, want link target length", fileLink.SizeBytes)
	}

	broken := requireListingEntry(t, listing, "broken-link")
	if !broken.IsSymlink || broken.LinkTarget != "missing" {
		t.Fatalf("broken link metadata = %#v, want symlink to missing", broken)
	}
	if broken.Kind != EntryBroken || broken.CanEnter {
		t.Fatalf("broken link entry = %#v, want broken non-enterable link", broken)
	}
}

func requireListingEntry(t *testing.T, listing Listing, name string) Entry {
	t.Helper()
	for _, entry := range listing.Entries {
		if entry.Name == name {
			return entry
		}
	}
	t.Fatalf("entry %q not found in %#v", name, listing.Entries)
	return Entry{}
}
