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
	nativeInsertPollInterval = 25 * time.Millisecond
)

var (
	user32DLL                    = syscall.NewLazyDLL("user32.dll")
	kernel32DLL                  = syscall.NewLazyDLL("kernel32.dll")
	procGetAsyncKeyState         = user32DLL.NewProc("GetAsyncKeyState")
	procGetForegroundWindow      = user32DLL.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessID = user32DLL.NewProc("GetWindowThreadProcessId")
	procGetCurrentProcessID      = kernel32DLL.NewProc("GetCurrentProcessId")
	nativeInsertInvalidateMu     sync.Mutex
	nativeInsertInvalidate       func()
	nativeInsertPressMu          sync.Mutex
	nativeInsertPresses          int
	nativeInsertMonitorMu        sync.Mutex
	nativeInsertMonitorStop      chan struct{}
	nativeInsertMonitorDone      chan struct{}
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
}

func nativeInsertMonitorLoop(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(nativeInsertPollInterval)
	defer ticker.Stop()
	var insertWasDown bool
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			insertDown := windowsForegroundBelongsToCurrentProcess() && windowsInsertKeyDown()
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
			insertWasDown = insertDown
		}
	}
}

func windowsInsertKeyDown() bool {
	state, _, _ := procGetAsyncKeyState.Call(windowsVKInsert)
	return uint16(state)&0x8000 != 0
}

func windowsForegroundBelongsToCurrentProcess() bool {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}
	var pid uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	currentPID, _, _ := procGetCurrentProcessID.Call()
	return pid != 0 && pid == uint32(currentPID)
}

func consumeNativeInsertPresses() int {
	nativeInsertPressMu.Lock()
	count := nativeInsertPresses
	nativeInsertPresses = 0
	nativeInsertPressMu.Unlock()
	return count
}
