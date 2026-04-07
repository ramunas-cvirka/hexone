// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package platform

import "golang.org/x/sys/windows"

type VolumeUsage struct {
	FreeBytes  uint64
	TotalBytes uint64
}

func LocalVolumeUsage(path string) (VolumeUsage, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return VolumeUsage{}, err
	}

	var freeBytesAvailableToCaller uint64
	var totalNumberOfBytes uint64
	var totalNumberOfFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &freeBytesAvailableToCaller, &totalNumberOfBytes, &totalNumberOfFreeBytes); err != nil {
		return VolumeUsage{}, err
	}

	return VolumeUsage{
		FreeBytes:  freeBytesAvailableToCaller,
		TotalBytes: totalNumberOfBytes,
	}, nil
}
