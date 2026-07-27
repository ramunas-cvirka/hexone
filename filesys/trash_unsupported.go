// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !linux && !windows

package filesys

import "fmt"

func movePathsToTrash(paths []string) error {
	return fmt.Errorf("%w on this platform", ErrTrashUnsupported)
}
