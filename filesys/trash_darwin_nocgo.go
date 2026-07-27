// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && !cgo

package filesys

import "fmt"

func movePathsToTrash(paths []string) error {
	return fmt.Errorf("%w: macOS trash support requires cgo", ErrTrashUnsupported)
}
