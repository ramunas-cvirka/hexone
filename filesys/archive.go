// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mholt/archives"
)

type ArchivePath struct {
	DisplayPath string
	ArchivePath string
	InnerPath   string
}

var archiveNameMatchCache sync.Map

func ArchiveNameSupported(name string) bool {
	key := strings.ToLower(strings.TrimSpace(filepath.Base(name)))
	if key == "" {
		return false
	}
	if cached, ok := archiveNameMatchCache.Load(key); ok {
		return cached.(bool)
	}
	format, _, err := archives.Identify(context.Background(), key, nil)
	if err != nil {
		archiveNameMatchCache.Store(key, false)
		return false
	}
	_, ok := format.(archives.Extractor)
	archiveNameMatchCache.Store(key, ok)
	return ok
}

func ParseArchivePath(raw string) (ArchivePath, bool) {
	clean := filepath.Clean(strings.TrimSpace(raw))
	if clean == "" {
		return ArchivePath{}, false
	}
	archivePath, innerPath, ok := splitArchivePath(clean)
	if !ok {
		return ArchivePath{}, false
	}
	if info, err := os.Stat(archivePath); err == nil && info.IsDir() {
		return ArchivePath{}, false
	}
	return ArchivePath{
		DisplayPath: clean,
		ArchivePath: filepath.Clean(archivePath),
		InnerPath:   innerPath,
	}, true
}

func ArchivePathActive(raw string) bool {
	_, ok := ParseArchivePath(raw)
	return ok
}

func ArchiveMemberPath(raw string) bool {
	loc, ok := ParseArchivePath(raw)
	return ok && loc.InnerPath != "."
}

func OpenArchiveFS(raw string) (fs.FS, ArchivePath, error) {
	loc, ok := ParseArchivePath(raw)
	if !ok {
		return nil, ArchivePath{}, errors.New("path is not inside an archive")
	}
	fsys, err := openArchiveFSForLocation(loc)
	return fsys, loc, err
}

func StatLocalPath(raw string) (os.FileInfo, error) {
	if loc, ok := ParseArchivePath(raw); ok {
		fsys, err := openArchiveFSForLocation(loc)
		if err != nil {
			return nil, err
		}
		return fs.Stat(fsys, loc.InnerPath)
	}
	return os.Stat(raw)
}

func LstatLocalPath(raw string) (os.FileInfo, error) {
	if loc, ok := ParseArchivePath(raw); ok {
		fsys, err := openArchiveFSForLocation(loc)
		if err != nil {
			return nil, err
		}
		return fs.Stat(fsys, loc.InnerPath)
	}
	return os.Lstat(raw)
}

func OpenLocalPath(raw string) (io.ReadCloser, os.FileInfo, error) {
	if loc, ok := ParseArchivePath(raw); ok {
		fsys, err := openArchiveFSForLocation(loc)
		if err != nil {
			return nil, nil, err
		}
		info, err := fs.Stat(fsys, loc.InnerPath)
		if err != nil {
			return nil, nil, err
		}
		file, err := fsys.Open(loc.InnerPath)
		if err != nil {
			return nil, nil, err
		}
		return file, info, nil
	}
	info, err := os.Stat(raw)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Open(raw)
	if err != nil {
		return nil, nil, err
	}
	return file, info, nil
}

func ReadLocalFileChunk(raw string, start, length int64) ([]byte, int64, error) {
	reader, info, err := OpenLocalPath(raw)
	if err != nil {
		return nil, 0, err
	}
	defer reader.Close()

	if info.IsDir() {
		return nil, 0, errors.New("viewer supports files only")
	}

	size := info.Size()
	if start < 0 {
		start = 0
	}
	if length < 0 {
		length = 0
	}
	if size >= 0 {
		if start > size {
			start = size
		}
		if start+length > size {
			length = size - start
		}
	}
	if length <= 0 {
		return nil, size, nil
	}

	buf := make([]byte, length)
	if ra, ok := reader.(interface {
		ReadAt([]byte, int64) (int, error)
	}); ok {
		n, err := ra.ReadAt(buf, start)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, size, err
		}
		return buf[:n], size, nil
	}

	if start > 0 {
		if _, err := io.CopyN(io.Discard, reader, start); err != nil && !errors.Is(err, io.EOF) {
			return nil, size, err
		}
	}
	n, err := io.ReadFull(reader, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, size, err
	}
	return buf[:n], size, nil
}

func splitArchivePath(raw string) (archivePath, innerPath string, ok bool) {
	clean := filepath.ToSlash(filepath.Clean(raw))
	parts := strings.Split(clean, "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if !ArchiveNameSupported(part) {
			continue
		}
		archivePath = filepath.Clean(filepath.FromSlash(strings.Join(parts[:i+1], "/")))
		if i+1 >= len(parts) {
			return archivePath, ".", true
		}
		innerPath = path.Clean(strings.Join(parts[i+1:], "/"))
		if innerPath == "" || innerPath == "/" {
			innerPath = "."
		}
		return archivePath, innerPath, true
	}
	return filepath.Clean(raw), "", false
}

func openArchiveFSForLocation(loc ArchivePath) (fs.FS, error) {
	fsys, err := archives.FileSystem(context.Background(), loc.ArchivePath, nil)
	if err != nil {
		return nil, err
	}
	if _, ok := fsys.(*archives.ArchiveFS); !ok {
		return nil, errors.New("path is not an archive")
	}
	return fsys, nil
}
