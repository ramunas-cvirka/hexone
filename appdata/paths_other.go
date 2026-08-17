// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !linux && !windows

package appdata

func ConfigDir() string {
	return ""
}

func ConfigPath() string {
	return configFileName
}

func SessionPath() string {
	return sessionFileName
}

func HTTPCollectionsPath() string {
	return httpCollectionsFileName
}

func ProtocolPath() string {
	return protocolsFileName
}

func ProtocolSamplePath() string {
	return ""
}
