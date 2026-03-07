//go:build (linux && !android) || freebsd || openbsd

package appicon

import (
	"unsafe"

	"gioui.org/app"
)

type Setter struct {
	x11Display unsafe.Pointer
	x11Window  uintptr
}

func NewSetter() *Setter {
	return &Setter{}
}

func (s *Setter) HandleViewEvent(ev app.ViewEvent) {
	switch view := ev.(type) {
	case app.X11ViewEvent:
		if !view.Valid() {
			s.x11Display = nil
			s.x11Window = 0
			return
		}
		if s.x11Display == view.Display && s.x11Window == view.Window {
			return
		}
		s.x11Display = view.Display
		s.x11Window = view.Window
		setX11WindowIcon(view.Display, view.Window)
	case app.WaylandViewEvent:
		// Wayland window icons are resolved from app_id + desktop metadata.
		// We still set app.ID in init so packaged builds can map to "hexone".
	}
}
