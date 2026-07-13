// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && !ios

package windowstate

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#import <dispatch/dispatch.h>

static void hexonePrimaryScreenSize(double *width, double *height) {
	void (^readScreen)(void) = ^{
		NSScreen *screen = [NSScreen mainScreen];
		if (screen == nil) {
			screen = [[NSScreen screens] firstObject];
		}
		if (screen != nil) {
			NSRect frame = [screen frame];
			*width = frame.size.width;
			*height = frame.size.height;
		}
	};
	if ([NSThread isMainThread]) {
		readScreen();
	} else {
		dispatch_sync(dispatch_get_main_queue(), readScreen);
	}
}
*/
import "C"

import "gioui.org/unit"

func platformStartupScreenSize() (unit.Dp, unit.Dp, bool) {
	var width, height C.double
	C.hexonePrimaryScreenSize(&width, &height)
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	// Gio Dp maps to AppKit points, while the renderer applies the screen's
	// backing scale separately.
	return unit.Dp(width), unit.Dp(height), true
}
