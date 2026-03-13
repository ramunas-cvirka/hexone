// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package appdata

import (
	"os"
	"path/filepath"
)

func ConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		return ""
	}
	base = filepath.Join(base, configDirName)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return ""
	}
	return base
}

func ConfigPath() string {
	return dataFilePath(configFileName)
}

func SessionPath() string {
	return dataFilePath(sessionFileName)
}

func ProtocolPath() string {
	return dataFilePath(protocolsFileName)
}

func ProtocolSamplePath() string {
	base := ConfigDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, protocolsSampleFileName)
}

func dataFilePath(name string) string {
	base := ConfigDir()
	if base == "" {
		return name
	}
	return filepath.Join(base, name)
}
