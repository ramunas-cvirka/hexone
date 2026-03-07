//go:build windows

package windowstate

import (
	"hexone/fm"
	"unsafe"

	"gioui.org/app"
	"gioui.org/unit"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type Tracker struct {
	cfg     app.Config
	haveCfg bool

	metric     unit.Metric
	haveMetric bool

	hwnd     uintptr
	haveHWND bool

	titleBarApplied bool
}

func NewTracker(session *fm.SessionState) *Tracker {
	_ = session
	return &Tracker{}
}

func (t *Tracker) ObserveView(v app.ViewEvent) {
	if t == nil {
		return
	}
	ev, ok := v.(app.Win32ViewEvent)
	if !ok || !ev.Valid() || ev.HWND == 0 {
		return
	}
	if t.hwnd != ev.HWND {
		t.titleBarApplied = false
	}
	t.hwnd = ev.HWND
	t.haveHWND = true
	if !t.titleBarApplied {
		t.titleBarApplied = winSetImmersiveDarkMode(t.hwnd, winAppsUseDarkTheme())
	}
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
	if t == nil || s == nil {
		return
	}
	if t.haveCfg {
		s.Window.Width = t.cfg.Size.X
		s.Window.Height = t.cfg.Size.Y
		s.Window.Mode = windowModeToSessionMode(t.cfg.Mode)
		s.Window.HasPosition = t.cfg.HasPosition
		if t.cfg.HasPosition {
			s.Window.X = t.cfg.Position.X
			s.Window.Y = t.cfg.Position.Y
		}
	}
	if t.haveMetric && t.metric.PxPerDp > 0 {
		s.Window.PxPerDp = t.metric.PxPerDp
	}
	if !t.haveHWND {
		return
	}

	mode := app.Windowed
	if t.haveCfg {
		mode = t.cfg.Mode
	}

	switch mode {
	case app.Maximized, app.Minimized:
		wp, ok := winGetWindowPlacement(t.hwnd)
		if !ok {
			return
		}
		s.Window.X = int(wp.RcNormalPosition.Left)
		s.Window.Y = int(wp.RcNormalPosition.Top)
		s.Window.HasPosition = true
	default:
		r, ok := winGetWindowRect(t.hwnd)
		if !ok {
			return
		}
		if sessionWindowPositionLooksHidden(int(r.Left), int(r.Top)) {
			if wp, ok := winGetWindowPlacement(t.hwnd); ok && !sessionWindowPositionLooksHidden(int(wp.RcNormalPosition.Left), int(wp.RcNormalPosition.Top)) {
				s.Window.X = int(wp.RcNormalPosition.Left)
				s.Window.Y = int(wp.RcNormalPosition.Top)
				s.Window.HasPosition = true
				return
			}
			s.Window.X = 0
			s.Window.Y = 0
			s.Window.HasPosition = false
			return
		}
		s.Window.X = int(r.Left)
		s.Window.Y = int(r.Top)
		s.Window.HasPosition = true
	}
}

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type winPoint struct {
	X int32
	Y int32
}

type winWindowPlacement struct {
	Length           uint32
	Flags            uint32
	ShowCmd          uint32
	PtMinPosition    winPoint
	PtMaxPosition    winPoint
	RcNormalPosition winRect
}

var (
	user32ProcGetWindowRect      = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowRect")
	user32ProcGetWindowPlacement = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowPlacement")
	dwmapiProcSetWindowAttribute = windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
)

func winGetWindowRect(hwnd uintptr) (winRect, bool) {
	var r winRect
	ret, _, _ := user32ProcGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r, ret != 0
}

func winGetWindowPlacement(hwnd uintptr) (winWindowPlacement, bool) {
	wp := winWindowPlacement{
		Length: uint32(unsafe.Sizeof(winWindowPlacement{})),
	}
	ret, _, _ := user32ProcGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp)))
	return wp, ret != 0
}

func winSetImmersiveDarkMode(hwnd uintptr, enabled bool) bool {
	if hwnd == 0 {
		return false
	}
	var v int32
	if enabled {
		v = 1
	}
	// Build/version differences: try modern (20), then legacy (19).
	if winDwmSetWindowAttributeBool(hwnd, 20, v) {
		return true
	}
	return winDwmSetWindowAttributeBool(hwnd, 19, v)
}

func winDwmSetWindowAttributeBool(hwnd uintptr, attr uint32, value int32) bool {
	ret, _, _ := dwmapiProcSetWindowAttribute.Call(
		hwnd,
		uintptr(attr),
		uintptr(unsafe.Pointer(&value)),
		uintptr(unsafe.Sizeof(value)),
	)
	return ret == 0
}

func winAppsUseDarkTheme() bool {
	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return true
	}
	defer k.Close()

	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return true
	}
	return v == 0
}
