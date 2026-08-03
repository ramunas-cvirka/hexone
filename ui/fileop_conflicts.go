// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"os"
	"strings"
)

const fileOverwriteConflictVisibleLimit = 5

type fileOverwriteConflict struct {
	Name    string
	SrcInfo fileCopyPathInfo
	DstInfo fileCopyPathInfo
}

func inspectFileOverwriteConflicts(srcEp copyEndpoint, sources []fileCopySource, dstEp copyEndpoint, dstDir string) ([]fileOverwriteConflict, int, error) {
	conflicts := make([]fileOverwriteConflict, 0, len(sources))
	count := 0
	for _, source := range sources {
		srcPath, err := srcEp.normalizeSourcePath(source.Path)
		if err != nil {
			return nil, 0, err
		}
		srcStat, err := endpointLstat(srcEp, srcPath)
		if err != nil {
			return nil, 0, err
		}

		dstPath := dstEp.join(dstDir, srcEp.baseName(srcPath))
		if endpointSamePath(srcEp, srcPath, dstEp, dstPath) {
			continue
		}
		dstStat, err := endpointLstat(dstEp, dstPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
				continue
			}
			return nil, 0, err
		}

		count++
		name := strings.TrimSpace(source.Name)
		if name == "" {
			name = srcEp.baseName(srcPath)
		}
		conflicts = append(conflicts, fileOverwriteConflict{
			Name:    name,
			SrcInfo: fileCopyPathInfoFromStat(srcPath, srcStat),
			DstInfo: fileCopyPathInfoFromStat(dstPath, dstStat),
		})
	}
	return conflicts, count, nil
}

func fileCopyPathInfoFromStat(path string, info os.FileInfo) fileCopyPathInfo {
	result := fileCopyPathInfo{Path: path}
	if info == nil {
		return result
	}
	result.Exists = true
	result.IsDir = info.IsDir()
	result.ModTime = info.ModTime()
	if info.Mode().IsRegular() {
		result.Size = info.Size()
	}
	return result
}

func moveSourcesForConflictPreview(sources []fileMoveSource) []fileCopySource {
	result := make([]fileCopySource, 0, len(sources))
	for _, source := range sources {
		result = append(result, fileCopySource{Path: source.Path, Name: source.Name})
	}
	return result
}

func sameFileOverwriteConflictPreview(a []fileOverwriteConflict, aCount int, b []fileOverwriteConflict, bCount int) bool {
	if aCount != bCount || len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || !sameFileCopyPathInfo(a[i].SrcInfo, b[i].SrcInfo) || !sameFileCopyPathInfo(a[i].DstInfo, b[i].DstInfo) {
			return false
		}
	}
	return true
}

func sameFileCopyPathInfo(a, b fileCopyPathInfo) bool {
	return a.Path == b.Path &&
		a.Exists == b.Exists &&
		a.IsDir == b.IsDir &&
		a.Size == b.Size &&
		a.ModTime.Equal(b.ModTime)
}
