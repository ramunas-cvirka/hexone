// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && !ios

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#import <objc/runtime.h>

// GioView is provided by gioui's macOS backend.
@interface GioView : NSView
@property uintptr_t handle;
@end

extern _Bool gio_onCommandBySelector(uintptr_t h);

static BOOL hexoneIsFunctionKeyEvent(NSEvent *event) {
	if (event == nil || [event type] != NSEventTypeKeyDown) {
		return NO;
	}
	NSString *chars = [event charactersIgnoringModifiers];
	if (chars == nil || [chars length] != 1) {
		return NO;
	}
	switch ([chars characterAtIndex:0]) {
	case NSF1FunctionKey:
	case NSF2FunctionKey:
	case NSF3FunctionKey:
	case NSF4FunctionKey:
	case NSF5FunctionKey:
	case NSF6FunctionKey:
	case NSF7FunctionKey:
	case NSF8FunctionKey:
	case NSF9FunctionKey:
	case NSF10FunctionKey:
	case NSF11FunctionKey:
	case NSF12FunctionKey:
		return YES;
	default:
		return NO;
	}
}

@interface GioView (HexoneNoBeep)
@end

@implementation GioView (HexoneNoBeep)
+ (void)load {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		Method original = class_getInstanceMethod(self, @selector(doCommandBySelector:));
		Method replacement = class_getInstanceMethod(self, @selector(hexone_doCommandBySelector:));
		if (original != NULL && replacement != NULL) {
			method_exchangeImplementations(original, replacement);
		}
	});
}

// Ctrl+F is mapped by AppKit to moveForward:. When gio doesn't claim it,
// super doCommandBySelector falls back to a beep. No-op here to suppress it.
- (void)moveForward:(id)sender {}
- (void)moveForwardAndModifySelection:(id)sender {}
// AppKit can also re-enter doCommandBySelector for function keys after gio
// already dispatched the keyDown event. Claim those fallback passes locally
// to stop the system beep without patching gio itself.
- (void)hexone_doCommandBySelector:(SEL)action {
	if (gio_onCommandBySelector(self.handle)) {
		return;
	}
	if (hexoneIsFunctionKeyEvent(NSApp.currentEvent)) {
		return;
	}
	[self hexone_doCommandBySelector:action];
}
// Deliver the first click even when the window is inactive. Without this,
// macOS can use the first click only to activate the window, which makes
// context-menu actions feel like they need a second click.
- (BOOL)acceptsFirstMouse:(NSEvent *)event { return YES; }
@end
*/
import "C"
