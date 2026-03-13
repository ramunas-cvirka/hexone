// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !linux

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

func ProtocolPath() string {
	return protocolsFileName
}

func ProtocolSamplePath() string {
	return ""
}
