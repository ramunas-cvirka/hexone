//go:build darwin

package appicon

/*
#cgo CFLAGS: -Werror -Wno-deprecated-declarations -fobjc-arc -x objective-c
#cgo LDFLAGS: -framework AppKit -framework Foundation

#include <AppKit/AppKit.h>
#include <Foundation/Foundation.h>

static void hexoneSetApplicationIcon(const void *bytes, int length) {
	@autoreleasepool {
		NSData *data = [NSData dataWithBytes:bytes length:(NSUInteger)length];
		if (data == nil) {
			return;
		}
		NSImage *image = [[NSImage alloc] initWithData:data];
		if (image == nil) {
			return;
		}
		[NSApp setApplicationIconImage:image];
	}
}
*/
import "C"

import (
	"unsafe"

	"gioui.org/app"
)

const defaultMacIconSize = 512

type Setter struct {
	applied bool
}

func NewSetter() *Setter {
	return &Setter{}
}

func (s *Setter) HandleViewEvent(ev app.ViewEvent) {
	view, ok := ev.(app.AppKitViewEvent)
	if !ok || !view.Valid() || s.applied {
		return
	}
	data, err := defaultAppIconPNG(defaultMacIconSize)
	if err != nil || len(data) == 0 {
		return
	}
	C.hexoneSetApplicationIcon(unsafe.Pointer(&data[0]), C.int(len(data)))
	s.applied = true
}
