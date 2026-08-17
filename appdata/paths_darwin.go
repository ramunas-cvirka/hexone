// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package appdata

import (
	"os"
	"path/filepath"

	"gioui.org/app"
)

func ConfigDir() string {
	return dataDir()
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
	base := dataDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, protocolsSampleFileName)
}

func dataDir() string {
	base, err := app.DataDir()
	if err != nil || base == "" {
		return ""
	}
	base = filepath.Join(base, configDirName)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return ""
	}
	return base
}

func dataFilePath(name string) string {
	base := dataDir()
	if base == "" {
		return name
	}
	return filepath.Join(base, name)
}
