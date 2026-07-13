// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowstate

import (
	"gioui.org/unit"
	"golang.org/x/sys/windows"
)

var (
	startupUser32GetSystemMetrics = windows.NewLazySystemDLL("user32.dll").NewProc("GetSystemMetrics")
	startupUser32GetDPIForSystem  = windows.NewLazySystemDLL("user32.dll").NewProc("GetDpiForSystem")
	startupUser32GetDC            = windows.NewLazySystemDLL("user32.dll").NewProc("GetDC")
	startupUser32ReleaseDC        = windows.NewLazySystemDLL("user32.dll").NewProc("ReleaseDC")
	startupGDI32GetDeviceCaps     = windows.NewLazySystemDLL("gdi32.dll").NewProc("GetDeviceCaps")
)

const (
	startupSMCXScreen = 0
	startupSMCYScreen = 1
	startupLogPixelsX = 88
	startupDefaultDPI = 96
)

func platformStartupScreenSize() (unit.Dp, unit.Dp, bool) {
	width, _, _ := startupUser32GetSystemMetrics.Call(startupSMCXScreen)
	height, _, _ := startupUser32GetSystemMetrics.Call(startupSMCYScreen)
	if width == 0 || height == 0 {
		return 0, 0, false
	}

	dpi := startupPrimaryDPI()
	scale := float32(dpi) / startupDefaultDPI
	return unit.Dp(float32(width) / scale), unit.Dp(float32(height) / scale), true
}

func startupPrimaryDPI() uint32 {
	if err := startupUser32GetDPIForSystem.Find(); err == nil {
		if dpi, _, _ := startupUser32GetDPIForSystem.Call(); dpi > 0 {
			return uint32(dpi)
		}
	}

	hdc, _, _ := startupUser32GetDC.Call(0)
	if hdc != 0 {
		dpi, _, _ := startupGDI32GetDeviceCaps.Call(hdc, startupLogPixelsX)
		startupUser32ReleaseDC.Call(0, hdc)
		if dpi > 0 {
			return uint32(dpi)
		}
	}
	return startupDefaultDPI
}
