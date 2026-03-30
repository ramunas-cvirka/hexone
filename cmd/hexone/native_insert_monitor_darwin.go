// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && !ios

package main

func platformAltKeyDown() bool {
	return false
}
