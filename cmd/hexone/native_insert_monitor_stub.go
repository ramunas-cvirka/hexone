// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build (!darwin && !windows) || ios

package main

func setNativeInsertInvalidate(func()) {}

func installNativeInsertMonitor(func(func())) {}

func removeNativeInsertMonitor(func(func())) {}

func nativeInsertKeyDown() bool {
	return false
}

func nativeInsertKeyStateAvailable() bool {
	return false
}

func platformAltKeyDown() bool {
	return false
}
