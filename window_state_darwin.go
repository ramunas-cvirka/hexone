//go:build darwin && !ios

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#import <dispatch/dispatch.h>
#import <objc/runtime.h>
#include <pthread.h>
#include <stdint.h>

static pthread_mutex_t hexoneWindowStateMu = PTHREAD_MUTEX_INITIALIZER;

static int hexoneStartupHasPos = 0;
static double hexoneStartupX = 0;
static double hexoneStartupY = 0;

static int hexoneLastHasPos = 0;
static double hexoneLastX = 0;
static double hexoneLastY = 0;
static double hexoneLastScale = 1.0;

static void hexoneUpdateLastWindowState(NSWindow *window) {
	if (window == nil) {
		return;
	}
	NSRect frame = [window frame];
	NSScreen *screen = [window screen];
	double scale = 1.0;
	if (screen != nil) {
		double s = [screen backingScaleFactor];
		if (s > 0.0) {
			scale = s;
		}
	}
	pthread_mutex_lock(&hexoneWindowStateMu);
	hexoneLastHasPos = 1;
	hexoneLastX = frame.origin.x;
	hexoneLastY = frame.origin.y;
	hexoneLastScale = scale;
	pthread_mutex_unlock(&hexoneWindowStateMu);
}

static int hexoneConsumeStartupWindowPos(double *x, double *y) {
	int has = 0;
	pthread_mutex_lock(&hexoneWindowStateMu);
	if (hexoneStartupHasPos) {
		has = 1;
		*x = hexoneStartupX;
		*y = hexoneStartupY;
		hexoneStartupHasPos = 0;
	}
	pthread_mutex_unlock(&hexoneWindowStateMu);
	return has;
}

static void hexoneSetStartupWindowPosition(int enabled, double x, double y) {
	pthread_mutex_lock(&hexoneWindowStateMu);
	if (enabled) {
		hexoneStartupHasPos = 1;
		hexoneStartupX = x;
		hexoneStartupY = y;
	} else {
		hexoneStartupHasPos = 0;
	}
	pthread_mutex_unlock(&hexoneWindowStateMu);
}

static int hexoneGetLastWindowState(double *x, double *y, double *scale) {
	int has = 0;
	pthread_mutex_lock(&hexoneWindowStateMu);
	if (hexoneLastHasPos) {
		has = 1;
		*x = hexoneLastX;
		*y = hexoneLastY;
		*scale = hexoneLastScale > 0.0 ? hexoneLastScale : 1.0;
	}
	pthread_mutex_unlock(&hexoneWindowStateMu);
	return has;
}

static int hexoneHasLastWindowState(void) {
	int has = 0;
	pthread_mutex_lock(&hexoneWindowStateMu);
	has = hexoneLastHasPos;
	pthread_mutex_unlock(&hexoneWindowStateMu);
	return has;
}

static void hexoneCaptureWindowStateForViewAsync(uintptr_t viewRef) {
	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			NSView *view = (__bridge NSView *)((void *)viewRef);
			if (view == nil) {
				return;
			}
			NSWindow *window = [view window];
			if (window == nil) {
				return;
			}
			hexoneUpdateLastWindowState(window);
		}
	});
}

static void (*hexoneOrigMakeKeyAndOrderFront)(id, SEL, id) = NULL;

static void hexoneSwizzledMakeKeyAndOrderFront(id self, SEL _cmd, id sender) {
	NSWindow *window = (NSWindow *)self;
	double sx = 0;
	double sy = 0;
	if (hexoneConsumeStartupWindowPos(&sx, &sy)) {
		NSRect frame = [window frame];
		frame.origin.x = sx;
		frame.origin.y = sy;
		[window setFrame:frame display:NO];
	}
	if (hexoneOrigMakeKeyAndOrderFront != NULL) {
		hexoneOrigMakeKeyAndOrderFront(self, _cmd, sender);
	}
	hexoneUpdateLastWindowState(window);
}

@interface HexoneWindowObserver : NSObject
@end

@implementation HexoneWindowObserver
- (void)onWindowChanged:(NSNotification *)notification {
	id obj = [notification object];
	if ([obj isKindOfClass:[NSWindow class]]) {
		hexoneUpdateLastWindowState((NSWindow *)obj);
	}
}
@end

static HexoneWindowObserver *hexoneWindowObserver = nil;
static pthread_once_t hexoneHooksOnce = PTHREAD_ONCE_INIT;

static void hexoneInstallWindowHooks(void) {
	Class cls = [NSWindow class];
	Method m = class_getInstanceMethod(cls, @selector(makeKeyAndOrderFront:));
	if (m != NULL && hexoneOrigMakeKeyAndOrderFront == NULL) {
		hexoneOrigMakeKeyAndOrderFront = (void (*)(id, SEL, id))method_getImplementation(m);
		method_setImplementation(m, (IMP)hexoneSwizzledMakeKeyAndOrderFront);
	}
	if (hexoneWindowObserver == nil) {
		hexoneWindowObserver = [HexoneWindowObserver new];
		NSNotificationCenter *nc = [NSNotificationCenter defaultCenter];
		[nc addObserver:hexoneWindowObserver selector:@selector(onWindowChanged:) name:NSWindowDidMoveNotification object:nil];
		[nc addObserver:hexoneWindowObserver selector:@selector(onWindowChanged:) name:NSWindowDidResizeNotification object:nil];
		[nc addObserver:hexoneWindowObserver selector:@selector(onWindowChanged:) name:NSWindowDidBecomeKeyNotification object:nil];
	}
}

static void hexoneEnsureWindowHooksInstalled(void) {
	pthread_once(&hexoneHooksOnce, hexoneInstallWindowHooks);
}

__attribute__((constructor))
static void hexoneInitWindowHooks(void) {
	hexoneEnsureWindowHooksInstalled();
}
*/
import "C"

import (
	"hexone/fm"
	"math"

	"gioui.org/app"
	"gioui.org/unit"
)

func preparePlatformWindowRestore(session *fm.SessionState) {
	C.hexoneEnsureWindowHooksInstalled()
	if session == nil || !session.Window.HasPosition || session.Window.Mode != "windowed" {
		C.hexoneSetStartupWindowPosition(C.int(0), 0, 0)
		return
	}
	pxPerDp := session.Window.PxPerDp
	if pxPerDp <= 0 {
		pxPerDp = 1
	}
	x := float64(float32(session.Window.X) / pxPerDp)
	y := float64(float32(session.Window.Y) / pxPerDp)
	C.hexoneSetStartupWindowPosition(C.int(1), C.double(x), C.double(y))
}

type windowStateTracker struct {
	cfg     app.Config
	haveCfg bool

	metric     unit.Metric
	haveMetric bool

	view     uintptr
	haveView bool

	fallbackHasPosition bool
	fallbackX           int
	fallbackY           int
	fallbackPxPerDp     float32
}

func newWindowStateTracker(session *fm.SessionState) *windowStateTracker {
	t := &windowStateTracker{}
	if session != nil && session.Window.HasPosition {
		t.fallbackHasPosition = true
		t.fallbackX = session.Window.X
		t.fallbackY = session.Window.Y
		t.fallbackPxPerDp = session.Window.PxPerDp
	}
	return t
}

func (t *windowStateTracker) ObserveView(v app.ViewEvent) {
	if t == nil {
		return
	}
	ev, ok := v.(app.AppKitViewEvent)
	if !ok || !ev.Valid() || ev.View == 0 {
		t.haveView = false
		t.view = 0
		return
	}
	t.view = ev.View
	t.haveView = true
	C.hexoneCaptureWindowStateForViewAsync(C.uintptr_t(t.view))
}

func (t *windowStateTracker) ObserveConfig(cfg app.Config) {
	if t == nil {
		return
	}
	t.cfg = cfg
	t.haveCfg = true
	if t.haveView && t.view != 0 {
		C.hexoneCaptureWindowStateForViewAsync(C.uintptr_t(t.view))
	}
}

func (t *windowStateTracker) ObserveFrame(metric unit.Metric) {
	if t == nil {
		return
	}
	t.metric = metric
	t.haveMetric = true
	if t.haveView && t.view != 0 && C.hexoneHasLastWindowState() == 0 {
		C.hexoneCaptureWindowStateForViewAsync(C.uintptr_t(t.view))
	}
}

func (t *windowStateTracker) ApplyToSession(s *fm.SessionState) {
	if t == nil || s == nil {
		return
	}
	if t.haveCfg {
		s.Window.Width = t.cfg.Size.X
		s.Window.Height = t.cfg.Size.Y
		s.Window.Mode = windowModeToSessionMode(t.cfg.Mode)
	}
	var x, y, scale C.double
	if C.hexoneGetLastWindowState(&x, &y, &scale) != 0 {
		sc := float64(scale)
		if sc <= 0 {
			sc = 1
		}
		s.Window.X = int(math.Round(float64(x) * sc))
		s.Window.Y = int(math.Round(float64(y) * sc))
		s.Window.HasPosition = true
		s.Window.PxPerDp = float32(sc)
	} else if t.fallbackHasPosition {
		s.Window.X = t.fallbackX
		s.Window.Y = t.fallbackY
		s.Window.HasPosition = true
		if t.fallbackPxPerDp > 0 {
			s.Window.PxPerDp = t.fallbackPxPerDp
		}
	}
	if t.haveMetric && t.metric.PxPerDp > 0 {
		s.Window.PxPerDp = t.metric.PxPerDp
	}
}
