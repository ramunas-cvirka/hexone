//go:build !windows && !darwin && !(linux || freebsd || openbsd)

package appicon

import "gioui.org/app"

type Setter struct{}

func NewSetter() *Setter {
	return &Setter{}
}

func (s *Setter) HandleViewEvent(_ app.ViewEvent) {}
