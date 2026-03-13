// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin || ios

package windowstate

import "hexone/fm"

func preparePlatformWindowRestore(_ *fm.SessionState) {}
