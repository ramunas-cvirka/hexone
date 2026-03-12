package windowstate

import (
	"hexone/fm"

	"gioui.org/app"
	"gioui.org/unit"
)

func ApplyWindowOptions(window *app.Window, session *fm.SessionState) {
	if window == nil || session == nil {
		return
	}
	preparePlatformWindowRestore(session)
	opts := windowOptionsForSession(session)
	if len(opts) > 0 {
		window.Option(opts...)
	}
}

func windowOptionsForSession(session *fm.SessionState) []app.Option {
	if session == nil {
		return nil
	}
	pxPerDp := session.Window.PxPerDp
	if pxPerDp <= 0 {
		pxPerDp = 1
	}
	pxToDp := func(px int) unit.Dp {
		return unit.Dp(float32(px) / pxPerDp)
	}

	opts := make([]app.Option, 0, 3)
	if session.Window.Width > 0 && session.Window.Height > 0 {
		opts = append(opts, app.Size(pxToDp(session.Window.Width), pxToDp(session.Window.Height)))
	}
	// Upstream Gio no longer exposes a public window-position option.
	switch sessionModeToWindowMode(session.Window.Mode) {
	case app.Maximized:
		opts = append(opts, app.Maximized.Option())
	case app.Fullscreen:
		opts = append(opts, app.Fullscreen.Option())
	case app.Minimized:
		opts = append(opts, app.Minimized.Option())
	default:
		// Keep windowed by default.
	}
	return opts
}

func sessionWindowRestorePosition(session *fm.SessionState) (x, y int, ok bool) {
	if session == nil || !session.Window.HasPosition {
		return 0, 0, false
	}
	if sessionWindowPositionLooksHidden(session.Window.X, session.Window.Y) {
		return 0, 0, false
	}
	return session.Window.X, session.Window.Y, true
}

func moveWindowRectOrigin(left, top, right, bottom int32, x, y int) (int32, int32, int32, int32) {
	width := right - left
	height := bottom - top
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	left = int32(x)
	top = int32(y)
	right = left + width
	bottom = top + height
	return left, top, right, bottom
}

// Windows reports minimized/off-screen windows at approximately (-32000, -32000).
// Restoring that position makes the next launch appear "missing" even though the
// process is running normally.
func sessionWindowPositionLooksHidden(x, y int) bool {
	return x <= -32000 || y <= -32000
}

func sessionModeToWindowMode(raw string) app.WindowMode {
	switch raw {
	case "maximized":
		return app.Maximized
	case "fullscreen":
		return app.Fullscreen
	case "minimized":
		return app.Minimized
	default:
		return app.Windowed
	}
}

func windowModeToSessionMode(mode app.WindowMode) string {
	switch mode {
	case app.Maximized:
		return "maximized"
	case app.Fullscreen:
		return "fullscreen"
	case app.Minimized:
		return "minimized"
	default:
		return "windowed"
	}
}
