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

// WriteClipboardTextNow writes Unicode text with the same retry behavior used
// by the native file clipboard. Gio's Windows clipboard writer makes a single
// OpenClipboard attempt and silently drops the write when another process has
// the clipboard open.
func WriteClipboardTextNow(text string) error {
	u16, err := windows.UTF16FromString(text)
	if err != nil {
		return fmt.Errorf("encode clipboard text: %w", err)
	}

	bytes := len(u16) * 2
	mem, _, callErr := procGlobalAlloc.Call(clipboardGMEMMoveable|clipboardGMEMZeroInit, uintptr(bytes))
	if mem == 0 {
		return fmt.Errorf("allocate clipboard text: %w", callErr)
	}
	owned := true
	defer func() {
		if owned {
			procGlobalFree.Call(mem)
		}
	}()

	ptr, _, callErr := procGlobalLock.Call(mem)
	if ptr == 0 {
		return fmt.Errorf("lock clipboard text: %w", callErr)
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(u16)), u16)
	procGlobalUnlock.Call(mem)

	owner, _, _ := procGetActiveWindow.Call()
	if owner == 0 {
		owner, _, _ = procGetForegroundWindow.Call()
	}
	if err := openSystemClipboard(owner); err != nil {
		return err
	}
	defer procCloseClipboard.Call()
	if ok, _, callErr := procEmptyClipboard.Call(); ok == 0 {
		return fmt.Errorf("empty clipboard: %w", callErr)
	}
	if ok, _, callErr := procSetClipboardData.Call(clipboardCFUnicodeText, mem); ok == 0 {
		return fmt.Errorf("set clipboard text: %w", callErr)
	}
	owned = false
	return nil
}
