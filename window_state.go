package main

import (
	"hexone/fm"

	"gioui.org/app"
	"gioui.org/unit"
)

func applyWindowOptionsFromSession(window *app.Window, session *fm.SessionState) {
	if window == nil || session == nil {
		return
	}
	opts := make([]app.Option, 0, 3)
	if session.Window.Width > 0 && session.Window.Height > 0 {
		opts = append(opts, app.Size(unit.Dp(session.Window.Width), unit.Dp(session.Window.Height)))
	}
	if session.Window.HasPosition && session.Window.Mode == "windowed" {
		opts = append(opts, app.Position(unit.Dp(session.Window.X), unit.Dp(session.Window.Y)))
	}
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
	if len(opts) > 0 {
		window.Option(opts...)
	}
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
