// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package main

import (
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	windowsVKInsert          = 0x2D
	windowsVKMenu            = 0x12
	nativeInsertPollInterval = 25 * time.Millisecond
	windowsWMSysChar         = 0x0106
	windowsWMSysCommand      = 0x0112
	windowsSCKeyMenu         = 0xF100
)

var (
	user32DLL                    = syscall.NewLazyDLL("user32.dll")
	kernel32DLL                  = syscall.NewLazyDLL("kernel32.dll")
	procGetAsyncKeyState         = user32DLL.NewProc("GetAsyncKeyState")
	procGetForegroundWindow      = user32DLL.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessID = user32DLL.NewProc("GetWindowThreadProcessId")
	procSetWindowLongPtr         = user32DLL.NewProc("SetWindowLongPtrW")
	procCallWindowProc           = user32DLL.NewProc("CallWindowProcW")
	procGetCurrentProcessID      = kernel32DLL.NewProc("GetCurrentProcessId")
	nativeInsertInvalidateMu     sync.Mutex
	nativeInsertInvalidate       func()
	nativeInsertPressMu          sync.Mutex
	nativeInsertPresses          int
	nativeInsertMonitorMu        sync.Mutex
	nativeInsertMonitorStop      chan struct{}
	nativeInsertMonitorDone      chan struct{}
	windowsSubclassMu            sync.Mutex
	windowsSubclassHWND          uintptr
	windowsOriginalWndProc       uintptr
	windowsSubclassWndProc       = syscall.NewCallback(hexoneWindowsWndProc)
	windowsGWLWndProc            = ^uintptr(3)
)

func setNativeInsertInvalidate(fn func()) {
	nativeInsertInvalidateMu.Lock()
	nativeInsertInvalidate = fn
	nativeInsertInvalidateMu.Unlock()
}

func installNativeInsertMonitor(func(func())) {
	nativeInsertMonitorMu.Lock()
	defer nativeInsertMonitorMu.Unlock()
	if nativeInsertMonitorStop != nil {
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	nativeInsertMonitorStop = stop
	nativeInsertMonitorDone = done
	go nativeInsertMonitorLoop(stop, done)
}

func removeNativeInsertMonitor(func(func())) {
	nativeInsertMonitorMu.Lock()
	stop := nativeInsertMonitorStop
	done := nativeInsertMonitorDone
	nativeInsertMonitorStop = nil
	nativeInsertMonitorDone = nil
	nativeInsertMonitorMu.Unlock()
	if stop != nil {
		close(stop)
	}
	if done != nil {
		<-done
	}
	restoreWindowsWndProcSubclass()
}

func nativeInsertMonitorLoop(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(nativeInsertPollInterval)
	defer ticker.Stop()
	var insertWasDown bool
	var altWasDown bool
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			hwnd := windowsForegroundWindowForCurrentProcess()
			if hwnd != 0 {
				ensureWindowsWndProcSubclass(hwnd)
			}
			insertDown := hwnd != 0 && windowsInsertKeyDown()
			altDown := hwnd != 0 && windowsAltKeyDown()
			if insertDown && !insertWasDown {
				nativeInsertPressMu.Lock()
				nativeInsertPresses++
				nativeInsertPressMu.Unlock()
				nativeInsertInvalidateMu.Lock()
				invalidate := nativeInsertInvalidate
				nativeInsertInvalidateMu.Unlock()
				if invalidate != nil {
					go invalidate()
				}
			}
			if altDown != altWasDown {
				nativeInsertInvalidateMu.Lock()
				invalidate := nativeInsertInvalidate
				nativeInsertInvalidateMu.Unlock()
				if invalidate != nil {
					go invalidate()
				}
			}
			insertWasDown = insertDown
			altWasDown = altDown
		}
	}
}

func windowsInsertKeyDown() bool {
	state, _, _ := procGetAsyncKeyState.Call(windowsVKInsert)
	return uint16(state)&0x8000 != 0
}

func windowsAltKeyDown() bool {
	state, _, _ := procGetAsyncKeyState.Call(windowsVKMenu)
	return uint16(state)&0x8000 != 0
}

func platformAltKeyDown() bool {
	return windowsForegroundWindowForCurrentProcess() != 0 && windowsAltKeyDown()
}

func windowsForegroundBelongsToCurrentProcess() bool {
	return windowsForegroundWindowForCurrentProcess() != 0
}

func windowsForegroundWindowForCurrentProcess() uintptr {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return 0
	}
	var pid uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	currentPID, _, _ := procGetCurrentProcessID.Call()
	if pid == 0 || pid != uint32(currentPID) {
		return 0
	}
	return hwnd
}

func ensureWindowsWndProcSubclass(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	windowsSubclassMu.Lock()
	defer windowsSubclassMu.Unlock()
	if windowsSubclassHWND == hwnd && windowsOriginalWndProc != 0 {
		return
	}
	if windowsSubclassHWND != 0 && windowsOriginalWndProc != 0 {
		procSetWindowLongPtr.Call(windowsSubclassHWND, uintptr(windowsGWLWndProc), windowsOriginalWndProc)
		windowsSubclassHWND = 0
		windowsOriginalWndProc = 0
	}
	orig, _, _ := procSetWindowLongPtr.Call(hwnd, uintptr(windowsGWLWndProc), windowsSubclassWndProc)
	if orig == 0 {
		return
	}
	windowsSubclassHWND = hwnd
	windowsOriginalWndProc = orig
}

func restoreWindowsWndProcSubclass() {
	windowsSubclassMu.Lock()
	defer windowsSubclassMu.Unlock()
	if windowsSubclassHWND == 0 || windowsOriginalWndProc == 0 {
		return
	}
	procSetWindowLongPtr.Call(windowsSubclassHWND, uintptr(windowsGWLWndProc), windowsOriginalWndProc)
	windowsSubclassHWND = 0
	windowsOriginalWndProc = 0
}

func hexoneWindowsWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case windowsWMSysCommand:
		if wParam&0xFFF0 == windowsSCKeyMenu {
			return 0
		}
	case windowsWMSysChar:
		// Hexone doesn't use native Windows menu mnemonics, so swallow
		// translated Alt-character messages to avoid the system beep.
		return 0
	}

	windowsSubclassMu.Lock()
	orig := windowsOriginalWndProc
	trackedHWND := windowsSubclassHWND
	windowsSubclassMu.Unlock()
	if orig != 0 && (trackedHWND == 0 || hwnd == trackedHWND) {
		ret, _, _ := procCallWindowProc.Call(orig, hwnd, msg, wParam, lParam)
		return ret
	}
	return 0
}

func consumeNativeInsertPresses() int {
	nativeInsertPressMu.Lock()
	count := nativeInsertPresses
	nativeInsertPresses = 0
	nativeInsertPressMu.Unlock()
	return count
}
