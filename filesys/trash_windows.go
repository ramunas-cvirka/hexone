// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package filesys

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	fileOperationDelete    = 0x0003
	fileOperationSilent    = 0x0004
	fileOperationNoConfirm = 0x0010
	fileOperationAllowUndo = 0x0040
	fileOperationNoErrorUI = 0x0400
)

var shell32 = windows.NewLazySystemDLL("shell32.dll")
var shFileOperationW = shell32.NewProc("SHFileOperationW")

type shellFileOperation struct {
	window        uintptr
	function      uint32
	from          *uint16
	to            *uint16
	flags         uint16
	aborted       int32
	nameMappings  uintptr
	progressTitle *uint16
}

func movePathsToTrash(paths []string) error {
	from := make([]uint16, 0, 256)
	for _, path := range paths {
		value, err := windows.UTF16FromString(path)
		if err != nil {
			return err
		}
		from = append(from, value...)
	}
	from = append(from, 0)
	operation := shellFileOperation{
		function: fileOperationDelete,
		from:     &from[0],
		flags: fileOperationSilent |
			fileOperationNoConfirm |
			fileOperationAllowUndo |
			fileOperationNoErrorUI,
	}
	result, _, _ := shFileOperationW.Call(uintptr(unsafe.Pointer(&operation)))
	runtime.KeepAlive(from)
	runtime.KeepAlive(operation)
	if result != 0 {
		return fmt.Errorf("recycle bin operation failed: %w", syscall.Errno(result))
	}
	if operation.aborted != 0 {
		return fmt.Errorf("recycle bin operation was cancelled")
	}
	return nil
}
