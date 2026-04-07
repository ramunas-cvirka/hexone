// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package platform

import "syscall"

type VolumeUsage struct {
	FreeBytes  uint64
	TotalBytes uint64
}

func LocalVolumeUsage(path string) (VolumeUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return VolumeUsage{}, err
	}

	blockSize := uint64(stat.Bsize)
	return VolumeUsage{
		FreeBytes:  stat.Bavail * blockSize,
		TotalBytes: stat.Blocks * blockSize,
	}, nil
}
