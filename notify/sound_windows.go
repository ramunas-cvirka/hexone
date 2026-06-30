// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package notify

import (
	"syscall"
	"unsafe"
)

const (
	sndNodefault = 0x0002
	sndFilename  = 0x00020000
)

var playSound = syscall.NewLazyDLL("winmm.dll").NewProc("PlaySoundW")

func playOperationComplete() {
	path, err := operationCompleteSoundPath()
	if err != nil {
		return
	}
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	_, _, _ = playSound.Call(uintptr(unsafe.Pointer(ptr)), 0, sndFilename|sndNodefault)
}
