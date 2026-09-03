// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type EntryKind uint8

const (
	EntryFile EntryKind = iota
	EntryDir
	EntryParent
	EntryBroken
)

type Entry struct {
	Name        string
	DisplayName string
	IsSymlink   bool
	LinkTarget  string
	PermText    string
	PermOctal   string
	SizeText    string
	SizeBytes   int64
	DateText    string
	OwnerText   string
	ModTime     time.Time
	Kind        EntryKind
	Path        string
	CanEnter    bool
}

type Listing struct {
	Dir     string
	Entries []Entry
}

func ReadDir(dir string) (Listing, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Listing{}, err
	}
	abs = filepath.Clean(abs)

	if loc, ok := ParseArchivePath(abs); ok {
		return readArchiveDir(abs, loc)
	}

	return readLocalDir(abs)
}

func readLocalDir(abs string) (Listing, error) {
	items, err := os.ReadDir(abs)
	if err != nil {
		return Listing{}, err
	}

	out := Listing{
		Dir:     abs,
		Entries: make([]Entry, 0, len(items)+1),
	}

	appendParentEntry(&out, abs)

	rows := make([]sortableEntry, 0, len(items))
	for _, item := range items {
		name := item.Name()
		full := filepath.Join(abs, name)
		row := Entry{
			Name:      name,
			Path:      full,
			Kind:      EntryFile,
			PermText:  "—",
			PermOctal: "—",
			SizeText:  "",
			DateText:  "—",
		}

		info, err := os.Lstat(full)
		if err != nil {
			row.Kind = EntryBroken
			row.DisplayName = name
			rows = append(rows, sortableEntry{entry: row, key: strings.ToLower(name), group: 2})
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			target, _ := os.Readlink(full)
			var targetInfo os.FileInfo
			if statInfo, statErr := os.Stat(full); statErr == nil {
				targetInfo = statInfo
			}
			populateSymlinkListingEntry(&row, name, info, targetInfo, target)
		} else {
			populateListingEntry(&row, name, info)
		}
		row.OwnerText = localOwnerText(info)
		if row.Kind == EntryFile && ArchiveNameSupported(name) {
			row.CanEnter = true
		}
		rows = append(rows, sortableEntry{entry: row, key: strings.ToLower(name), group: listingEntryGroup(row)})
	}

	appendSortedEntries(&out, rows)
	return out, nil
}

func readArchiveDir(abs string, loc ArchivePath) (Listing, error) {
	fsys, err := openArchiveFSForLocation(loc)
	if err != nil {
		return Listing{}, err
	}
	innerPath := loc.InnerPath
	if innerPath == "" {
		innerPath = "."
	}

	items, err := fs.ReadDir(fsys, innerPath)
	if err != nil {
		return Listing{}, err
	}

	out := Listing{
		Dir:     abs,
		Entries: make([]Entry, 0, len(items)+1),
	}
	appendParentEntry(&out, abs)

	rows := make([]sortableEntry, 0, len(items))
	for _, item := range items {
		name := item.Name()
		full := filepath.Join(abs, name)
		row := Entry{
			Name:      name,
			Path:      full,
			Kind:      EntryFile,
			PermText:  "—",
			PermOctal: "—",
			SizeText:  "",
			DateText:  "—",
		}

		info, err := item.Info()
		if err != nil {
			row.Kind = EntryBroken
			row.DisplayName = name
			rows = append(rows, sortableEntry{entry: row, key: strings.ToLower(name), group: 2})
			continue
		}

		populateListingEntry(&row, name, info)
		rows = append(rows, sortableEntry{entry: row, key: strings.ToLower(name), group: listingEntryGroup(row)})
	}

	appendSortedEntries(&out, rows)
	return out, nil
}

type sortableEntry struct {
	entry Entry
	key   string
	group int
}

func appendParentEntry(out *Listing, dir string) {
	if out == nil {
		return
	}
	parent := filepath.Dir(dir)
	if parent == dir {
		return
	}
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

func populateListingEntry(row *Entry, name string, info os.FileInfo) {
	if row == nil {
		return
	}
	row.DisplayName = name
	row.PermText = formatPerms(info.Mode())
	row.PermOctal = formatPermOctal(info.Mode())
	if info.IsDir() {
		row.Kind = EntryDir
		row.SizeText = ""
		row.CanEnter = true
	} else {
		row.Kind = EntryFile
		row.SizeBytes = info.Size()
		row.SizeText = formatSize(info.Size())
	}
	row.DateText = formatDate(info.ModTime())
	row.ModTime = info.ModTime()
}

func populateSymlinkListingEntry(row *Entry, name string, linkInfo os.FileInfo, targetInfo os.FileInfo, target string) {
	if row == nil {
		return
	}
	populateListingEntry(row, name, linkInfo)
	row.IsSymlink = true
	row.LinkTarget = target
	if row.PermText != "" && row.PermText != "—" && !strings.HasPrefix(row.PermText, "l") {
		row.PermText = "l" + row.PermText
	}
	if targetInfo == nil {
		row.Kind = EntryBroken
		row.CanEnter = false
		return
	}
	if targetInfo.IsDir() {
		row.Kind = EntryDir
		row.CanEnter = true
	}
}

func listingEntryGroup(row Entry) int {
	if row.Kind == EntryDir {
		return 0
	}
	if row.Kind == EntryBroken {
		return 2
	}
	return 1
}

func appendSortedEntries(out *Listing, rows []sortableEntry) {
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
}

func formatSize(size int64) string {
	if size < 1024 {
		return itoa(size) + " B"
	}

	type unitDef struct {
		name string
		size int64
	}

	units := []unitDef{
		{name: "TB", size: 1 << 40},
		{name: "GB", size: 1 << 30},
		{name: "MB", size: 1 << 20},
		{name: "KB", size: 1 << 10},
	}

	for _, unit := range units {
		if size < unit.size {
			continue
		}
		whole := (size * 10) / unit.size
		intPart := whole / 10
		fracPart := whole % 10
		return itoa(intPart) + "." + itoa(fracPart) + " " + unit.name
	}

	return itoa(size) + " B"
}

func formatDate(ts time.Time) string {
	if ts.IsZero() {
		return "—"
	}
	return ts.Format("Jan 02 2006")
}

func formatPerms(mode os.FileMode) string {
	const chars = "rwxrwxrwx"
	bits := []os.FileMode{
		0o400, 0o200, 0o100,
		0o040, 0o020, 0o010,
		0o004, 0o002, 0o001,
	}

	out := make([]byte, len(bits))
	for i, bit := range bits {
		if mode&bit != 0 {
			out[i] = chars[i]
		} else {
			out[i] = '-'
		}
	}
	return string(out)
}

func formatPermOctal(mode os.FileMode) string {
	perm := int64(mode.Perm())
	var out [4]byte
	for i := 3; i >= 0; i-- {
		out[i] = byte('0' + perm%8)
		perm /= 8
	}
	return string(out[:])
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [32]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + (v % 10))
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
