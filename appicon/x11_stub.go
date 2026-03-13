// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build ((linux && !android) || freebsd || openbsd) && nox11

package appicon

import "unsafe"

func setX11WindowIcon(_ unsafe.Pointer, _ uintptr) {}
