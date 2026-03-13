// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type CopyProgress struct {
	EntriesDone  int
	EntriesTotal int
	BytesDone    int64
	BytesTotal   int64
	CurrentPath  string
}

// CopyPath copies a file or directory from srcPath to dstPath.
//
// If dstPath points to an existing directory, the source base name is appended.
// Existing destination files are overwritten.
func CopyPath(srcPath, dstPath string, report func(CopyProgress)) error {
	srcAbs, err := filepath.Abs(srcPath)
	if err != nil {
		return err
	}
	dstAbs, err := filepath.Abs(dstPath)
	if err != nil {
		return err
	}
	srcAbs = filepath.Clean(srcAbs)
	dstAbs = filepath.Clean(dstAbs)

	srcInfo, err := os.Lstat(srcAbs)
	if err != nil {
		return err
	}

	if dstInfo, err := os.Stat(dstAbs); err == nil && dstInfo.IsDir() {
		dstAbs = filepath.Join(dstAbs, filepath.Base(srcAbs))
	}

	if sameFilePath(srcAbs, dstAbs) {
		return errors.New("source and destination are the same")
	}

	if srcInfo.IsDir() {
		if pathWithin(dstAbs, srcAbs) {
			return errors.New("destination cannot be inside source directory")
		}
		return copyDirectory(srcAbs, dstAbs, report)
	}

	return copySingleEntry(srcAbs, dstAbs, srcInfo, report)
}

type copyEntry struct {
	src  string
	dst  string
	info os.FileInfo
}

func copySingleEntry(src, dst string, info os.FileInfo, report func(CopyProgress)) error {
	progress := CopyProgress{
		EntriesTotal: 1,
		CurrentPath:  src,
	}
	if info.Mode().IsRegular() {
		progress.BytesTotal = info.Size()
	}
	reportCopyProgress(report, progress)

	if err := copyEntryAt(copyEntry{src: src, dst: dst, info: info}, &progress, report); err != nil {
		return err
	}
	progress.EntriesDone = 1
	reportCopyProgress(report, progress)
	return nil
}

func copyDirectory(srcRoot, dstRoot string, report func(CopyProgress)) error {
	entries := make([]copyEntry, 0, 64)
	var bytesTotal int64

	if err := filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := d.Info()
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		dst := dstRoot
		if rel != "." {
			dst = filepath.Join(dstRoot, rel)
		}

		entries = append(entries, copyEntry{
			src:  path,
			dst:  dst,
			info: info,
		})
		if info.Mode().IsRegular() {
			bytesTotal += info.Size()
		}
		return nil
	}); err != nil {
		return err
	}

	progress := CopyProgress{
		EntriesTotal: len(entries),
		BytesTotal:   bytesTotal,
	}
	reportCopyProgress(report, progress)

	for _, entry := range entries {
		progress.CurrentPath = entry.src
		reportCopyProgress(report, progress)
		if err := copyEntryAt(entry, &progress, report); err != nil {
			return err
		}
		progress.EntriesDone++
		reportCopyProgress(report, progress)
	}

	return nil
}

func copyEntryAt(entry copyEntry, progress *CopyProgress, report func(CopyProgress)) error {
	info := entry.info
	switch {
	case info.IsDir():
		if err := os.MkdirAll(entry.dst, info.Mode().Perm()); err != nil {
			return err
		}
		if err := os.Chmod(entry.dst, info.Mode().Perm()); err != nil {
			return err
		}
		return os.Chtimes(entry.dst, info.ModTime(), info.ModTime())
	case info.Mode()&os.ModeSymlink != 0:
		if err := copySymlink(entry.src, entry.dst); err != nil {
			return err
		}
		return nil
	default:
		return copyRegularFile(entry.src, entry.dst, info, progress, report)
	}
}

func copyRegularFile(src, dst string, info os.FileInfo, progress *CopyProgress, report func(CopyProgress)) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	buf := make([]byte, 1<<20)
	for {
		nr, readErr := in.Read(buf)
		if nr > 0 {
			chunk := buf[:nr]
			for len(chunk) > 0 {
				nw, writeErr := out.Write(chunk)
				if nw > 0 {
					chunk = chunk[nw:]
					progress.BytesDone += int64(nw)
					reportCopyProgress(report, *progress)
				}
				if writeErr != nil {
					return writeErr
				}
				if nw == 0 {
					return io.ErrShortWrite
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}

func copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if _, err := os.Lstat(dst); err == nil {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}
	return os.Symlink(target, dst)
}

func reportCopyProgress(report func(CopyProgress), progress CopyProgress) {
	if report == nil {
		return
	}
	report(progress)
}

func sameFilePath(a, b string) bool {
	if a == b {
		return true
	}
	if os.PathSeparator == '\\' {
		return strings.EqualFold(a, b)
	}
	return false
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	prefix := ".." + string(filepath.Separator)
	return rel != ".." && !strings.HasPrefix(rel, prefix)
}
