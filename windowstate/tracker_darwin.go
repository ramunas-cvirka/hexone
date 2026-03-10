//go:build darwin && !ios

package windowstate

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
static int hexoneStartupApplySize = 0;
static double hexoneStartupWidth = 0;
static double hexoneStartupHeight = 0;

static int hexoneLastHasPos = 0;
static double hexoneLastX = 0;
static double hexoneLastY = 0;
static double hexoneLastContentWidth = 0;
static double hexoneLastContentHeight = 0;
static double hexoneLastScale = 1.0;

static void hexoneUpdateLastWindowState(NSWindow *window) {
	if (window == nil) {
		return;
	}
	NSRect frame = [window frame];
	NSRect contentRect = [window contentRectForFrameRect:frame];
	if (contentRect.size.width <= 0.0 || contentRect.size.height <= 0.0) {
		NSView *contentView = [window contentView];
		if (contentView != nil) {
			contentRect = [contentView frame];
			if (contentRect.size.width <= 0.0 || contentRect.size.height <= 0.0) {
				contentRect = [contentView bounds];
			}
		}
	}
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
	if (contentRect.size.width > 0.0) {
		hexoneLastContentWidth = contentRect.size.width;
	}
	if (contentRect.size.height > 0.0) {
		hexoneLastContentHeight = contentRect.size.height;
	}
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

static int hexoneConsumeStartupWindowSize(double *width, double *height) {
	int has = 0;
	pthread_mutex_lock(&hexoneWindowStateMu);
	if (hexoneStartupApplySize) {
		has = 1;
		*width = hexoneStartupWidth;
		*height = hexoneStartupHeight;
		hexoneStartupApplySize = 0;
	}
	pthread_mutex_unlock(&hexoneWindowStateMu);
	return has;
}

static int hexonePeekStartupWindowPos(double *x, double *y) {
	int has = 0;
	pthread_mutex_lock(&hexoneWindowStateMu);
	if (hexoneStartupHasPos) {
		has = 1;
		*x = hexoneStartupX;
		*y = hexoneStartupY;
	}
	pthread_mutex_unlock(&hexoneWindowStateMu);
	return has;
}

static void hexoneSetStartupWindowState(int enabledPos, double x, double y, int applySize, double width, double height) {
	pthread_mutex_lock(&hexoneWindowStateMu);
	if (enabledPos) {
		hexoneStartupHasPos = 1;
		hexoneStartupX = x;
		hexoneStartupY = y;
	} else {
		hexoneStartupHasPos = 0;
	}
	if (applySize && width > 0.0 && height > 0.0) {
		hexoneStartupApplySize = 1;
		hexoneStartupWidth = width;
		hexoneStartupHeight = height;
	} else {
		hexoneStartupApplySize = 0;
	}
	pthread_mutex_unlock(&hexoneWindowStateMu);
}

static int hexoneGetLastWindowState(double *x, double *y, double *width, double *height, double *scale) {
	int has = 0;
	pthread_mutex_lock(&hexoneWindowStateMu);
	if (hexoneLastHasPos) {
		has = 1;
		*x = hexoneLastX;
		*y = hexoneLastY;
		*width = hexoneLastContentWidth;
		*height = hexoneLastContentHeight;
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

static void hexoneCaptureWindowStateForViewSync(uintptr_t viewRef) {
	void (^capture)(void) = ^{
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
	};
	if ([NSThread isMainThread]) {
		capture();
		return;
	}
	dispatch_sync(dispatch_get_main_queue(), capture);
}

static void hexoneApplyStartupWindowState(NSWindow *window, int consume) {
	if (window == nil) {
		return;
	}
	double width = 0;
	double height = 0;
	if (consume && hexoneConsumeStartupWindowSize(&width, &height)) {
		[window setContentSize:NSMakeSize(width, height)];
	}
	double sx = 0;
	double sy = 0;
	int has = consume ? hexoneConsumeStartupWindowPos(&sx, &sy) : hexonePeekStartupWindowPos(&sx, &sy);
	if (!has) {
		return;
	}
	NSRect frame = [window frame];
	frame.origin.x = sx;
	frame.origin.y = sy;
	[window setFrame:frame display:NO];
}

static void hexoneRequestWindowRedraw(NSWindow *window) {
	if (window == nil) {
		return;
	}
	NSView *view = [window contentView];
	if (view == nil) {
		return;
	}
	[view setNeedsDisplay:YES];
	[view displayIfNeeded];
}

static void (*hexoneOrigMakeKeyAndOrderFront)(id, SEL, id) = NULL;
static void (*hexoneOrigZoom)(id, SEL, id) = NULL;
static void (*hexoneOrigToggleFullScreen)(id, SEL, id) = NULL;

static void hexoneSwizzledMakeKeyAndOrderFront(id self, SEL _cmd, id sender) {
	NSWindow *window = (NSWindow *)self;
	hexoneApplyStartupWindowState(window, 1);
	if (hexoneOrigMakeKeyAndOrderFront != NULL) {
		hexoneOrigMakeKeyAndOrderFront(self, _cmd, sender);
	}
	hexoneUpdateLastWindowState(window);
}

static void hexoneSwizzledZoom(id self, SEL _cmd, id sender) {
	NSWindow *window = (NSWindow *)self;
	hexoneApplyStartupWindowState(window, 0);
	if (hexoneOrigZoom != NULL) {
		hexoneOrigZoom(self, _cmd, sender);
	}
	hexoneUpdateLastWindowState(window);
}

static void hexoneSwizzledToggleFullScreen(id self, SEL _cmd, id sender) {
	NSWindow *window = (NSWindow *)self;
	hexoneApplyStartupWindowState(window, 0);
	if (hexoneOrigToggleFullScreen != NULL) {
		hexoneOrigToggleFullScreen(self, _cmd, sender);
	}
}

@interface HexoneWindowObserver : NSObject
@end

@implementation HexoneWindowObserver
- (void)onWindowChanged:(NSNotification *)notification {
	id obj = [notification object];
	if ([obj isKindOfClass:[NSWindow class]]) {
		NSWindow *window = (NSWindow *)obj;
		hexoneUpdateLastWindowState(window);
		hexoneRequestWindowRedraw(window);
	}
}
- (void)onWindowWillEnterFullScreen:(NSNotification *)notification {
	id obj = [notification object];
	if ([obj isKindOfClass:[NSWindow class]]) {
		hexoneRequestWindowRedraw((NSWindow *)obj);
	}
}
- (void)onWindowDidEnterFullScreen:(NSNotification *)notification {
	id obj = [notification object];
	if ([obj isKindOfClass:[NSWindow class]]) {
		NSWindow *window = (NSWindow *)obj;
		hexoneRequestWindowRedraw(window);
		hexoneUpdateLastWindowState(window);
	}
}
- (void)onWindowWillExitFullScreen:(NSNotification *)notification {
	id obj = [notification object];
	if ([obj isKindOfClass:[NSWindow class]]) {
		hexoneRequestWindowRedraw((NSWindow *)obj);
	}
}
- (void)onWindowDidExitFullScreen:(NSNotification *)notification {
	id obj = [notification object];
	if ([obj isKindOfClass:[NSWindow class]]) {
		NSWindow *window = (NSWindow *)obj;
		hexoneRequestWindowRedraw(window);
		hexoneUpdateLastWindowState(window);
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
	Method zoom = class_getInstanceMethod(cls, @selector(zoom:));
	if (zoom != NULL && hexoneOrigZoom == NULL) {
		hexoneOrigZoom = (void (*)(id, SEL, id))method_getImplementation(zoom);
		method_setImplementation(zoom, (IMP)hexoneSwizzledZoom);
	}
	Method toggleFullScreen = class_getInstanceMethod(cls, @selector(toggleFullScreen:));
	if (toggleFullScreen != NULL && hexoneOrigToggleFullScreen == NULL) {
		hexoneOrigToggleFullScreen = (void (*)(id, SEL, id))method_getImplementation(toggleFullScreen);
		method_setImplementation(toggleFullScreen, (IMP)hexoneSwizzledToggleFullScreen);
	}
	if (hexoneWindowObserver == nil) {
		hexoneWindowObserver = [HexoneWindowObserver new];
		NSNotificationCenter *nc = [NSNotificationCenter defaultCenter];
		[nc addObserver:hexoneWindowObserver selector:@selector(onWindowChanged:) name:NSWindowDidMoveNotification object:nil];
		[nc addObserver:hexoneWindowObserver selector:@selector(onWindowChanged:) name:NSWindowDidResizeNotification object:nil];
		[nc addObserver:hexoneWindowObserver selector:@selector(onWindowChanged:) name:NSWindowDidBecomeKeyNotification object:nil];
		[nc addObserver:hexoneWindowObserver selector:@selector(onWindowWillEnterFullScreen:) name:NSWindowWillEnterFullScreenNotification object:nil];
		[nc addObserver:hexoneWindowObserver selector:@selector(onWindowDidEnterFullScreen:) name:NSWindowDidEnterFullScreenNotification object:nil];
		[nc addObserver:hexoneWindowObserver selector:@selector(onWindowWillExitFullScreen:) name:NSWindowWillExitFullScreenNotification object:nil];
		[nc addObserver:hexoneWindowObserver selector:@selector(onWindowDidExitFullScreen:) name:NSWindowDidExitFullScreenNotification object:nil];
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

func applyDarwinWindowState(s *fm.SessionState, x, y, width, height, scale float64) {
	if s == nil {
		return
	}
	if scale <= 0 {
		scale = 1
	}
	s.Window.X = int(math.Round(x * scale))
	s.Window.Y = int(math.Round(y * scale))
	s.Window.HasPosition = true
	if width > 0 {
		s.Window.Width = int(math.Round(width * scale))
	}
	if height > 0 {
		s.Window.Height = int(math.Round(height * scale))
	}
	s.Window.PxPerDp = float32(scale)
}

func applyDarwinMetricFallback(s *fm.SessionState, metric unit.Metric, allow bool) {
	if s == nil || !allow || metric.PxPerDp <= 0 {
		return
	}
	s.Window.PxPerDp = metric.PxPerDp
}

func preparePlatformWindowRestore(session *fm.SessionState) {
	C.hexoneEnsureWindowHooksInstalled()
	if session == nil {
		C.hexoneSetStartupWindowState(C.int(0), 0, 0, C.int(0), 0, 0)
		return
	}
	pxPerDp := session.Window.PxPerDp
	if pxPerDp <= 0 {
		pxPerDp = 1
	}
	enablePos := C.int(0)
	var x, y float64
	if session.Window.HasPosition && !sessionWindowPositionLooksHidden(session.Window.X, session.Window.Y) {
		enablePos = C.int(1)
		x = float64(float32(session.Window.X) / pxPerDp)
		y = float64(float32(session.Window.Y) / pxPerDp)
	}
	enableSize := C.int(0)
	var width, height float64
	if session.Window.Mode == "windowed" && session.Window.Width > 0 && session.Window.Height > 0 {
		enableSize = C.int(1)
		width = float64(float32(session.Window.Width) / pxPerDp)
		height = float64(float32(session.Window.Height) / pxPerDp)
	}
	C.hexoneSetStartupWindowState(enablePos, C.double(x), C.double(y), enableSize, C.double(width), C.double(height))
}

type Tracker struct {
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

func NewTracker(session *fm.SessionState) *Tracker {
	t := &Tracker{}
	if session != nil && session.Window.HasPosition {
		t.fallbackHasPosition = true
		t.fallbackX = session.Window.X
		t.fallbackY = session.Window.Y
		t.fallbackPxPerDp = session.Window.PxPerDp
	}
	return t
}

func (t *Tracker) ObserveView(v app.ViewEvent) {
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

func (t *Tracker) ObserveConfig(cfg app.Config) {
	if t == nil {
		return
	}
	t.cfg = cfg
	t.haveCfg = true
	if t.haveView && t.view != 0 {
		C.hexoneCaptureWindowStateForViewAsync(C.uintptr_t(t.view))
	}
}

func (t *Tracker) ObserveFrame(metric unit.Metric) {
	if t == nil {
		return
	}
	t.metric = metric
	t.haveMetric = true
	if t.haveView && t.view != 0 && C.hexoneHasLastWindowState() == 0 {
		C.hexoneCaptureWindowStateForViewAsync(C.uintptr_t(t.view))
	}
}

func (t *Tracker) ApplyToSession(s *fm.SessionState) {
	if t == nil || s == nil {
		return
	}
	if t.haveView && t.view != 0 {
		C.hexoneCaptureWindowStateForViewSync(C.uintptr_t(t.view))
	}
	if t.haveCfg {
		s.Window.Width = t.cfg.Size.X
		s.Window.Height = t.cfg.Size.Y
		s.Window.Mode = windowModeToSessionMode(t.cfg.Mode)
	}
	appliedNativeState := false
	var x, y, width, height, scale C.double
	if C.hexoneGetLastWindowState(&x, &y, &width, &height, &scale) != 0 {
		applyDarwinWindowState(s, float64(x), float64(y), float64(width), float64(height), float64(scale))
		appliedNativeState = true
	} else if t.fallbackHasPosition {
		s.Window.X = t.fallbackX
		s.Window.Y = t.fallbackY
		s.Window.HasPosition = true
		if t.fallbackPxPerDp > 0 {
			s.Window.PxPerDp = t.fallbackPxPerDp
		}
	}
	applyDarwinMetricFallback(s, t.metric, !appliedNativeState && t.haveMetric)
}
