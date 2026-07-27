// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && cgo

package filesys

/*
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
char *hexoneMovePathToTrash(const char *path);
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func movePathsToTrash(paths []string) error {
	for _, path := range paths {
		cPath := C.CString(path)
		errText := C.hexoneMovePathToTrash(cPath)
		C.free(unsafe.Pointer(cPath))
		if errText == nil {
			continue
		}
		message := C.GoString(errText)
		C.free(unsafe.Pointer(errText))
		return fmt.Errorf("trash %s: %s", path, message)
	}
	return nil
}
