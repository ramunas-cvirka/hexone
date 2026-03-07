//go:build ((linux && !android) || freebsd || openbsd) && nox11

package appicon

import "unsafe"

func setX11WindowIcon(_ unsafe.Pointer, _ uintptr) {}
