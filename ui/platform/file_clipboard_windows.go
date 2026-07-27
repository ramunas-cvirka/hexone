// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package platform

import (
	"errors"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	clipboardCFHDrop           = 15
	clipboardGMEMMoveable      = 0x0002
	clipboardGMEMZeroInit      = 0x0040
	clipboardDropEffectCopy    = 1
	clipboardDropFilesHeader   = 20
	clipboardDragQueryAllFiles = 0xFFFFFFFF
)

var (
	clipboardShell32               = windows.NewLazySystemDLL("shell32.dll")
	procEmptyClipboard             = clipboardUser32.NewProc("EmptyClipboard")
	procSetClipboardData           = clipboardUser32.NewProc("SetClipboardData")
	procIsClipboardFormatAvailable = clipboardUser32.NewProc("IsClipboardFormatAvailable")
	procRegisterClipboardFormatW   = clipboardUser32.NewProc("RegisterClipboardFormatW")
	procGetActiveWindow            = clipboardUser32.NewProc("GetActiveWindow")
	procGetForegroundWindow        = clipboardUser32.NewProc("GetForegroundWindow")
	procGlobalAlloc                = clipboardKernel32.NewProc("GlobalAlloc")
	procGlobalFree                 = clipboardKernel32.NewProc("GlobalFree")
	procDragQueryFileW             = clipboardShell32.NewProc("DragQueryFileW")
)

func ReadClipboardFiles() ([]string, error) {
	available, _, _ := procIsClipboardFormatAvailable.Call(clipboardCFHDrop)
	if available == 0 {
		return nil, nil
	}
	if err := openSystemClipboard(0); err != nil {
		return nil, err
	}
	defer procCloseClipboard.Call()

	mem, _, err := procGetClipboardData.Call(clipboardCFHDrop)
	if mem == 0 {
		return nil, fmt.Errorf("get file clipboard data: %w", err)
	}

	count, _, _ := procDragQueryFileW.Call(mem, clipboardDragQueryAllFiles, 0, 0)
	values := make([]string, 0, count)
	for i := uintptr(0); i < count; i++ {
		length, _, _ := procDragQueryFileW.Call(mem, i, 0, 0)
		if length == 0 {
			continue
		}
		buffer := make([]uint16, length+1)
		copied, _, _ := procDragQueryFileW.Call(
			mem,
			i,
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(len(buffer)),
		)
		if copied == 0 {
			continue
		}
		values = append(values, windows.UTF16ToString(buffer[:copied]))
	}
	return normalizeClipboardFilePaths(values), nil
}

func WriteClipboardFiles(paths []string) error {
	paths = normalizeClipboardFilePaths(paths)
	if len(paths) == 0 {
		return errors.New("no local files to copy")
	}

	utf16Paths := make([]uint16, 0, 260*len(paths))
	for _, path := range paths {
		value, err := windows.UTF16FromString(path)
		if err != nil {
			return fmt.Errorf("encode clipboard path: %w", err)
		}
		utf16Paths = append(utf16Paths, value...)
	}
	utf16Paths = append(utf16Paths, 0)

	bytes := clipboardDropFilesHeader + len(utf16Paths)*2
	mem, _, err := procGlobalAlloc.Call(clipboardGMEMMoveable|clipboardGMEMZeroInit, uintptr(bytes))
	if mem == 0 {
		return fmt.Errorf("allocate file clipboard data: %w", err)
	}
	owned := true
	defer func() {
		if owned {
			procGlobalFree.Call(mem)
		}
	}()

	ptr, _, err := procGlobalLock.Call(mem)
	if ptr == 0 {
		return fmt.Errorf("lock file clipboard data: %w", err)
	}
	*(*uint32)(unsafe.Pointer(ptr)) = clipboardDropFilesHeader
	*(*uint32)(unsafe.Pointer(ptr + 16)) = 1
	dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr+clipboardDropFilesHeader)), len(utf16Paths))
	copy(dst, utf16Paths)
	procGlobalUnlock.Call(mem)

	owner, _, _ := procGetActiveWindow.Call()
	if owner == 0 {
		owner, _, _ = procGetForegroundWindow.Call()
	}
	if owner == 0 {
		return errors.New("no window is available to own the clipboard")
	}
	if err := openSystemClipboard(owner); err != nil {
		return err
	}
	defer procCloseClipboard.Call()
	if ok, _, err := procEmptyClipboard.Call(); ok == 0 {
		return fmt.Errorf("empty clipboard: %w", err)
	}
	if ok, _, err := procSetClipboardData.Call(clipboardCFHDrop, mem); ok == 0 {
		return fmt.Errorf("set file clipboard data: %w", err)
	}
	owned = false
	_ = writeClipboardPreferredDropEffect(clipboardDropEffectCopy)
	return nil
}

func openSystemClipboard(owner uintptr) error {
	var lastErr error
	for attempt := 0; attempt < 6; attempt++ {
		ok, _, err := procOpenClipboard.Call(owner)
		if ok != 0 {
			return nil
		}
		lastErr = err
		if attempt < 5 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	return fmt.Errorf("open clipboard: %w", lastErr)
}

func writeClipboardPreferredDropEffect(effect uint32) error {
	name, err := windows.UTF16PtrFromString("Preferred DropEffect")
	if err != nil {
		return err
	}
	format, _, callErr := procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(name)))
	if format == 0 {
		return fmt.Errorf("register clipboard drop effect: %w", callErr)
	}
	mem, _, callErr := procGlobalAlloc.Call(clipboardGMEMMoveable|clipboardGMEMZeroInit, 4)
	if mem == 0 {
		return fmt.Errorf("allocate clipboard drop effect: %w", callErr)
	}
	owned := true
	defer func() {
		if owned {
			procGlobalFree.Call(mem)
		}
	}()
	ptr, _, callErr := procGlobalLock.Call(mem)
	if ptr == 0 {
		return fmt.Errorf("lock clipboard drop effect: %w", callErr)
	}
	*(*uint32)(unsafe.Pointer(ptr)) = effect
	procGlobalUnlock.Call(mem)
	if ok, _, callErr := procSetClipboardData.Call(format, mem); ok == 0 {
		return fmt.Errorf("set clipboard drop effect: %w", callErr)
	}
	owned = false
	return nil
}
