// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package filesys

import (
	"os"

	"github.com/pkg/sftp"
)

// statOwnerIDs extracts uid/gid from an os.FileInfo. Windows local files carry
// a *syscall.Win32FileAttributeData, which has no uid/gid, so only remote SFTP
// entries resolve here. Local files report ok == false and the status bar omits
// the field.
func statOwnerIDs(info os.FileInfo) (uid, gid uint32, ok bool) {
	if info == nil {
		return 0, 0, false
	}
	if sys, isRemote := info.Sys().(*sftp.FileStat); isRemote {
		return sys.UID, sys.GID, true
	}
	return 0, 0, false
}
