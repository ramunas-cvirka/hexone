//go:build darwin && !ios

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

// GioView is provided by gioui's macOS backend.
@interface GioView : NSView
@end

@interface GioView (HexoneNoBeep)
@end

@implementation GioView (HexoneNoBeep)
// Ctrl+F is mapped by AppKit to moveForward:. When gio doesn't claim it,
// super doCommandBySelector falls back to a beep. No-op here to suppress it.
- (void)moveForward:(id)sender {}
- (void)moveForwardAndModifySelection:(id)sender {}
@end
*/
import "C"
