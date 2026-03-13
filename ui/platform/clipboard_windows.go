// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package platform

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	clipboardUser32      = windows.NewLazySystemDLL("user32.dll")
	clipboardKernel32    = windows.NewLazySystemDLL("kernel32.dll")
	procOpenClipboard    = clipboardUser32.NewProc("OpenClipboard")
	procCloseClipboard   = clipboardUser32.NewProc("CloseClipboard")
	procGetClipboardData = clipboardUser32.NewProc("GetClipboardData")
	procGlobalLock       = clipboardKernel32.NewProc("GlobalLock")
	procGlobalUnlock     = clipboardKernel32.NewProc("GlobalUnlock")
)

const clipboardCFUnicodeText = 13

func ReadClipboardTextNow() (string, error) {
	r, _, err := procOpenClipboard.Call(0)
	if r == 0 {
		return "", fmt.Errorf("open clipboard: %w", err)
	}
	defer procCloseClipboard.Call()

	mem, _, err := procGetClipboardData.Call(clipboardCFUnicodeText)
	if mem == 0 {
		return "", fmt.Errorf("get clipboard data: %w", err)
	}

	ptr, _, err := procGlobalLock.Call(mem)
	if ptr == 0 {
		return "", fmt.Errorf("global lock clipboard: %w", err)
	}
	defer procGlobalUnlock.Call(mem)

	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(ptr))), nil
}
