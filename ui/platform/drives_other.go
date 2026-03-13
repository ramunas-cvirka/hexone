// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package platform

func AvailableLocalDrives() []string {
	return nil
}
