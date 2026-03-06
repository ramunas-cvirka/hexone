//go:build !windows && !darwin

package main

import (
	"hexone/fm"

	"gioui.org/app"
	"gioui.org/unit"
)

type windowStateTracker struct {
	cfg     app.Config
	haveCfg bool

	metric     unit.Metric
	haveMetric bool
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

func (t *windowStateTracker) ObserveFrame(metric unit.Metric) {
	if t == nil {
		return
	}
	t.metric = metric
	t.haveMetric = true
}

func (t *windowStateTracker) ApplyToSession(s *fm.SessionState) {
	if t == nil || s == nil || !t.haveCfg {
		return
	}
	s.Window.Width = t.cfg.Size.X
	s.Window.Height = t.cfg.Size.Y
	s.Window.Mode = windowModeToSessionMode(t.cfg.Mode)
	s.Window.HasPosition = t.cfg.HasPosition
	if t.cfg.HasPosition {
		s.Window.X = t.cfg.Position.X
		s.Window.Y = t.cfg.Position.Y
	}
	if t.haveMetric && t.metric.PxPerDp > 0 {
		s.Window.PxPerDp = t.metric.PxPerDp
	}
}
