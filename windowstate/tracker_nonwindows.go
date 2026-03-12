//go:build !windows && !darwin

package windowstate

import (
	"hexone/fm"

	"gioui.org/app"
	"gioui.org/unit"
)

type Tracker struct {
	cfg     app.Config
	haveCfg bool

	metric     unit.Metric
	haveMetric bool
}

func NewTracker(_ *fm.SessionState) *Tracker {
	return &Tracker{}
}

func (t *Tracker) ObserveView(_ app.ViewEvent) {
}

func (t *Tracker) ObserveConfig(cfg app.Config) {
	if t == nil {
		return
	}
	t.cfg = cfg
	t.haveCfg = true
}

func (t *Tracker) ObserveFrame(metric unit.Metric) {
	if t == nil {
		return
	}
	t.metric = metric
	t.haveMetric = true
}

func (t *Tracker) ApplyToSession(s *fm.SessionState) {
	if t == nil || s == nil || !t.haveCfg {
		return
	}
	s.Window.Width = t.cfg.Size.X
	s.Window.Height = t.cfg.Size.Y
	s.Window.Mode = windowModeToSessionMode(t.cfg.Mode)
	s.Window.HasPosition = false
	s.Window.X = 0
	s.Window.Y = 0
	if t.haveMetric && t.metric.PxPerDp > 0 {
		s.Window.PxPerDp = t.metric.PxPerDp
	}
}
