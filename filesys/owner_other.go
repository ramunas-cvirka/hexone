// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package filesys

import (
	"os"
	"syscall"

	"github.com/pkg/sftp"
)

// statOwnerIDs extracts uid/gid from an os.FileInfo. Local files carry a
// *syscall.Stat_t; SFTP entries carry a *sftp.FileStat. SFTP listings happen on
// every platform, so both arms are needed here and the sftp arm is repeated in
// the Windows file.
func statOwnerIDs(info os.FileInfo) (uid, gid uint32, ok bool) {
	if info == nil {
		return 0, 0, false
	}
	switch sys := info.Sys().(type) {
	case *syscall.Stat_t:
		return uint32(sys.Uid), uint32(sys.Gid), true
	case *sftp.FileStat:
		return sys.UID, sys.GID, true
	}
	return 0, 0, false
}
