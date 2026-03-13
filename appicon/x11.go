// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build ((linux && !android) || freebsd || openbsd) && !nox11

package appicon

/*
#cgo freebsd openbsd CFLAGS: -I/usr/X11R6/include -I/usr/local/include
#cgo freebsd openbsd LDFLAGS: -L/usr/X11R6/lib -L/usr/local/lib
#cgo freebsd openbsd LDFLAGS: -lX11
#cgo linux pkg-config: x11

#include <stdint.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>

static void hexoneSetX11WindowIcon(Display *display, Window window, const uint32_t *data, int length) {
	Atom icon = XInternAtom(display, "_NET_WM_ICON", False);
	Atom cardinal = XInternAtom(display, "CARDINAL", False);
	XChangeProperty(display, window, icon, cardinal, 32, PropModeReplace, (const unsigned char *)data, length);
	XFlush(display);
}
*/
import "C"

import "unsafe"

func setX11WindowIcon(display unsafe.Pointer, window uintptr) {
	data := defaultAppIconX11Data()
	if len(data) == 0 {
		return
	}
	C.hexoneSetX11WindowIcon(
		(*C.Display)(display),
		C.Window(window),
		(*C.uint32_t)(unsafe.Pointer(&data[0])),
		C.int(len(data)),
	)
}
