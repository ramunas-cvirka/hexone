//go:build darwin

package appicon

import "gioui.org/app"

// On macOS, let the bundle icon drive Dock/Finder presentation so the pinned
// icon and the running icon stay identical.
type Setter struct{}

func NewSetter() *Setter {
	return &Setter{}
}

func (s *Setter) HandleViewEvent(_ app.ViewEvent) {}
