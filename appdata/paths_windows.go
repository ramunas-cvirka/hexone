// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package appdata

import (
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getCurrentPackageFamilyName = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCurrentPackageFamilyName")

func ConfigDir() string {
	packageFamilyName := currentPackageFamilyName()
	if packageFamilyName != "" {
		base, err := os.UserCacheDir()
		if err != nil || base == "" {
			return ""
		}
		dir := windowsDataDir(packageFamilyName, base, "")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return ""
		}
		return dir
	}

	executable, err := os.Executable()
	if err != nil || executable == "" {
		return ""
	}
	return windowsDataDir("", "", executable)
}

func ConfigPath() string {
	return dataFilePath(configFileName)
}

func SessionPath() string {
	return dataFilePath(sessionFileName)
}

func HTTPCollectionsPath() string {
	return dataFilePath(httpCollectionsFileName)
}

func ProtocolPath() string {
	return dataFilePath(protocolsFileName)
}

func ProtocolSamplePath() string {
	if currentPackageFamilyName() == "" {
		return ""
	}
	return dataFilePath(protocolsSampleFileName)
}

func dataFilePath(name string) string {
	base := ConfigDir()
	if base == "" {
		return name
	}
	return filepath.Join(base, name)
}

func windowsDataDir(packageFamilyName, userCacheDir, executable string) string {
	if packageFamilyName != "" {
		return filepath.Join(userCacheDir, "Packages", packageFamilyName, "LocalState")
	}
	return filepath.Dir(executable)
}

func currentPackageFamilyName() string {
	var length uint32
	result, _, _ := getCurrentPackageFamilyName.Call(
		uintptr(unsafe.Pointer(&length)),
		0,
	)
	if windows.Errno(result) != windows.ERROR_INSUFFICIENT_BUFFER || length == 0 {
		return ""
	}

	buffer := make([]uint16, length)
	result, _, _ = getCurrentPackageFamilyName.Call(
		uintptr(unsafe.Pointer(&length)),
		uintptr(unsafe.Pointer(&buffer[0])),
	)
	if result != 0 {
		return ""
	}
	return windows.UTF16ToString(buffer)
}
