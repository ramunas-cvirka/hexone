//go:build !windows

package main

import (
	"hexone/fm"

	"gioui.org/app"
)

type windowStateTracker struct {
	cfg     app.Config
	haveCfg bool
}

func newWindowStateTracker(_ *fm.SessionState) *windowStateTracker {
	return &windowStateTracker{}
}

func (t *windowStateTracker) ObserveView(_ app.ViewEvent) {
}

func (t *windowStateTracker) ObserveConfig(cfg app.Config) {
	if t == nil {
		return
	}
	t.cfg = cfg
	t.haveCfg = true
}

func (t *windowStateTracker) ObserveFrame() {
}

func (t *windowStateTracker) ApplyToSession(s *fm.SessionState) {
	if t == nil || s == nil || !t.haveCfg {
		return
	}
	s.Window.Width = t.cfg.Size.X
	s.Window.Height = t.cfg.Size.Y
	s.Window.Mode = windowModeToSessionMode(t.cfg.Mode)
}
