// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && !ios

package main

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#import <Carbon/Carbon.h>

static id hexoneNativeInsertMonitor = nil;

extern void hexone_onNativeInsertMonitor(void);

static const char *hexoneNativeInsertKind(NSEvent *event) {
	if (event == nil || [event type] != NSEventTypeKeyDown) {
		return NULL;
	}
	NSString *chars = [event charactersIgnoringModifiers];
	if (chars != nil && [chars length] == 1) {
		switch ([chars characterAtIndex:0]) {
		case NSInsertFunctionKey:
			return "Insert";
		case NSHelpFunctionKey:
			return "Help";
		}
	}
	switch ([event keyCode]) {
	case kVK_Help:
		return "HelpKeyCode";
	default:
		return NULL;
	}
}

static void hexoneInstallNativeInsertMonitor(void) {
	@autoreleasepool {
		if (hexoneNativeInsertMonitor != nil) {
			return;
		}
		hexoneNativeInsertMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskKeyDown handler:^NSEvent * _Nullable(NSEvent *event) {
			const char *kind = hexoneNativeInsertKind(event);
			if (kind == NULL) {
				return event;
			}
			if (![event isARepeat]) {
				hexone_onNativeInsertMonitor();
			}
			return nil;
		}];
	}
}

static void hexoneRemoveNativeInsertMonitor(void) {
	@autoreleasepool {
		if (hexoneNativeInsertMonitor == nil) {
			return;
		}
		[NSEvent removeMonitor:hexoneNativeInsertMonitor];
		hexoneNativeInsertMonitor = nil;
	}
}
*/
import "C"

import "sync"

var (
	nativeInsertInvalidateMu sync.Mutex
	nativeInsertInvalidate   func()
	nativeInsertPressMu      sync.Mutex
	nativeInsertPresses      int
)

func setNativeInsertInvalidate(fn func()) {
	nativeInsertInvalidateMu.Lock()
	nativeInsertInvalidate = fn
	nativeInsertInvalidateMu.Unlock()
}

func installNativeInsertMonitor(runWindowFunc func(func())) {
	runOnWindowThread(runWindowFunc, func() {
		C.hexoneInstallNativeInsertMonitor()
	})
}

func removeNativeInsertMonitor(runWindowFunc func(func())) {
	runOnWindowThread(runWindowFunc, func() {
		C.hexoneRemoveNativeInsertMonitor()
	})
}

func runOnWindowThread(runWindowFunc func(func()), fn func()) {
	if fn == nil {
		return
	}
	if runWindowFunc == nil {
		fn()
		return
	}
	runWindowFunc(fn)
}

func consumeNativeInsertPresses() int {
	nativeInsertPressMu.Lock()
	count := nativeInsertPresses
	nativeInsertPresses = 0
	nativeInsertPressMu.Unlock()
	return count
}

//export hexone_onNativeInsertMonitor
func hexone_onNativeInsertMonitor() {
	nativeInsertPressMu.Lock()
	nativeInsertPresses++
	nativeInsertPressMu.Unlock()
	nativeInsertInvalidateMu.Lock()
	invalidate := nativeInsertInvalidate
	nativeInsertInvalidateMu.Unlock()
	if invalidate != nil {
		go invalidate()
	}
}
