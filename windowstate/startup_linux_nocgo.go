// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build linux && !cgo

package windowstate

import "gioui.org/unit"

func platformStartupScreenSize() (unit.Dp, unit.Dp, bool) {
	return 0, 0, false
}
