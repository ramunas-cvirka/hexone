// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build linux && cgo

package windowstate

/*
#cgo pkg-config: x11 wayland-client
#include <stdlib.h>
#include <string.h>
#include <X11/Xlib.h>
#include <X11/Xresource.h>
#include <wayland-client.h>

static int hexoneX11ScreenSize(double *width, double *height, double *scale) {
	Display *display = XOpenDisplay(NULL);
	if (display == NULL) {
		return 0;
	}
	int screen = XDefaultScreen(display);
	*width = XDisplayWidth(display, screen);
	*height = XDisplayHeight(display, screen);
	*scale = 1.0;

	char *resources = XResourceManagerString(display);
	if (resources != NULL) {
		XrmInitialize();
		XrmDatabase db = XrmGetStringDatabase(resources);
		if (db != NULL) {
			char *type = NULL;
			XrmValue value = {0};
			if (XrmGetResource(db, "Xft.dpi", "Xft.Dpi", &type, &value) && value.addr != NULL) {
				double dpi = strtod(value.addr, NULL);
				if (dpi > 0.0) {
					*scale = dpi / 96.0;
				}
			}
			XrmDestroyDatabase(db);
		}
	}
	XCloseDisplay(display);
	return *width > 0.0 && *height > 0.0;
}

struct hexone_output {
	struct wl_output *output;
	int width;
	int height;
	int scale;
	struct hexone_output *next;
};

struct hexone_wayland_state {
	struct hexone_output *outputs;
};

static void hexoneOutputGeometry(void *data, struct wl_output *output, int32_t x, int32_t y,
		int32_t physicalWidth, int32_t physicalHeight, int32_t subpixel,
		const char *make, const char *model, int32_t transform) {
	(void)data; (void)output; (void)x; (void)y; (void)physicalWidth; (void)physicalHeight;
	(void)subpixel; (void)make; (void)model; (void)transform;
}

static void hexoneOutputMode(void *data, struct wl_output *output, uint32_t flags,
		int32_t width, int32_t height, int32_t refresh) {
	(void)output; (void)refresh;
	struct hexone_output *item = data;
	if (flags & WL_OUTPUT_MODE_CURRENT) {
		item->width = width;
		item->height = height;
	}
}

static void hexoneOutputDone(void *data, struct wl_output *output) {
	(void)data; (void)output;
}

static void hexoneOutputScale(void *data, struct wl_output *output, int32_t factor) {
	(void)output;
	struct hexone_output *item = data;
	if (factor > 0) {
		item->scale = factor;
	}
}

static const struct wl_output_listener hexoneOutputListener = {
	hexoneOutputGeometry,
	hexoneOutputMode,
	hexoneOutputDone,
	hexoneOutputScale,
};

static void hexoneRegistryGlobal(void *data, struct wl_registry *registry, uint32_t name,
		const char *interface, uint32_t version) {
	if (strcmp(interface, wl_output_interface.name) != 0) {
		return;
	}
	struct hexone_wayland_state *state = data;
	struct hexone_output *item = calloc(1, sizeof(*item));
	if (item == NULL) {
		return;
	}
	item->scale = 1;
	uint32_t bindVersion = version < 2 ? version : 2;
	item->output = wl_registry_bind(registry, name, &wl_output_interface, bindVersion);
	if (item->output == NULL) {
		free(item);
		return;
	}
	item->next = state->outputs;
	state->outputs = item;
	wl_output_add_listener(item->output, &hexoneOutputListener, item);
}

static void hexoneRegistryGlobalRemove(void *data, struct wl_registry *registry, uint32_t name) {
	(void)data; (void)registry; (void)name;
}

static const struct wl_registry_listener hexoneRegistryListener = {
	hexoneRegistryGlobal,
	hexoneRegistryGlobalRemove,
};

static int hexoneWaylandScreenSize(double *width, double *height) {
	struct wl_display *display = wl_display_connect(NULL);
	if (display == NULL) {
		return 0;
	}
	struct hexone_wayland_state state = {0};
	struct wl_registry *registry = wl_display_get_registry(display);
	if (registry == NULL) {
		wl_display_disconnect(display);
		return 0;
	}
	wl_registry_add_listener(registry, &hexoneRegistryListener, &state);
	int ok = wl_display_roundtrip(display) >= 0 && wl_display_roundtrip(display) >= 0;
	if (ok) {
		for (struct hexone_output *item = state.outputs; item != NULL; item = item->next) {
			if (item->width > 0 && item->height > 0 && item->scale > 0) {
				*width = (double)item->width / item->scale;
				*height = (double)item->height / item->scale;
				break;
			}
		}
	}
	while (state.outputs != NULL) {
		struct hexone_output *item = state.outputs;
		state.outputs = item->next;
		wl_output_destroy(item->output);
		free(item);
	}
	wl_registry_destroy(registry);
	wl_display_disconnect(display);
	return *width > 0.0 && *height > 0.0;
}
*/
import "C"

import (
	"os"

	"gioui.org/unit"
)

func platformStartupScreenSize() (unit.Dp, unit.Dp, bool) {
	var width, height C.double
	if os.Getenv("WAYLAND_DISPLAY") != "" && C.hexoneWaylandScreenSize(&width, &height) != 0 {
		return unit.Dp(width), unit.Dp(height), true
	}

	var scale C.double
	if C.hexoneX11ScreenSize(&width, &height, &scale) != 0 && scale > 0 {
		return unit.Dp(width / scale), unit.Dp(height / scale), true
	}
	if C.hexoneWaylandScreenSize(&width, &height) != 0 {
		return unit.Dp(width), unit.Dp(height), true
	}
	return 0, 0, false
}
