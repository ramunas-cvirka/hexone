// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin || ios

package main

func setNativeInsertInvalidate(func()) {}

func installNativeInsertMonitor(func(func())) {}

func removeNativeInsertMonitor(func(func())) {}

func consumeNativeInsertPresses() int {
	return 0
}
