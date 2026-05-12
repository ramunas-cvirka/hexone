// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"errors"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/pkg/sftp"
)

// ReadDirSFTP builds a Listing from an SFTP directory while reusing the same
// Entry shape as the local filesystem listing.
func ReadDirSFTP(client *sftp.Client, dir string) (Listing, error) {
	if client == nil {
		return Listing{}, errors.New("sftp client is nil")
	}

	base := strings.TrimSpace(dir)
	if base == "" {
		base = "."
	}
	if !path.IsAbs(base) {
		if cwd, err := client.Getwd(); err == nil && cwd != "" {
			base = path.Join(cwd, base)
		}
	}
	base = path.Clean(base)

	items, err := client.ReadDir(base)
	if err != nil {
		return Listing{}, err
	}

	out := Listing{
		Dir:     base,
		Entries: make([]Entry, 0, len(items)+1),
	}

	parent := path.Dir(base)
	if parent != base {
		out.Entries = append(out.Entries, Entry{
			Name:        "..",
			DisplayName: "..",
			PermText:    "",
			PermOctal:   "",
			SizeText:    "",
			DateText:    "",
			Kind:        EntryParent,
			Path:        parent,
			CanEnter:    true,
		})
	}

	type sortable struct {
		entry Entry
		key   string
		group int
	}

	rows := make([]sortable, 0, len(items))
	for _, item := range items {
		name := item.Name()
		full := path.Join(base, name)
		row := Entry{
			Name:      name,
			Path:      full,
			Kind:      EntryFile,
			PermText:  "—",
			PermOctal: "—",
			SizeText:  "",
			DateText:  "—",
		}

		if item.Mode()&os.ModeSymlink != 0 {
			target, _ := client.ReadLink(full)
			var targetInfo os.FileInfo
			if statInfo, statErr := client.Stat(full); statErr == nil {
				targetInfo = statInfo
			}
			populateSymlinkListingEntry(&row, name, item, targetInfo, target)
		} else {
			populateListingEntry(&row, name, item)
		}

		rows = append(rows, sortable{entry: row, key: strings.ToLower(name), group: listingEntryGroup(row)})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].group != rows[j].group {
			return rows[i].group < rows[j].group
		}
		if rows[i].key != rows[j].key {
			return rows[i].key < rows[j].key
		}
		return rows[i].entry.Name < rows[j].entry.Name
	})

	for _, row := range rows {
		out.Entries = append(out.Entries, row.entry)
	}

	return out, nil
}
