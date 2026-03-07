//go:build windows

package appicon

import "gioui.org/app"

// Windows uses the embedded executable icon resource from hexone_windows.syso.
// Avoid duplicating that work at runtime; it added startup cost and made the
// icon path harder to reason about.
type Setter struct{}

func NewSetter() *Setter {
	return &Setter{}
}

func (s *Setter) HandleViewEvent(_ app.ViewEvent) {}
