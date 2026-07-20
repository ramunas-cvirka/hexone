// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package filesys

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func movePathsToTrash(paths []string) error {
	if gioPath, err := exec.LookPath("gio"); err == nil {
		args := append([]string{"trash"}, paths...)
		if output, err := exec.Command(gioPath, args...).CombinedOutput(); err != nil {
			message := strings.TrimSpace(string(output))
			if message == "" {
				message = err.Error()
			}
			return fmt.Errorf("trash: %s", message)
		}
		return nil
	}
	return movePathsToFreeDesktopHomeTrash(paths)
}

func movePathsToFreeDesktopHomeTrash(paths []string) error {
	dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	trashRoot := filepath.Join(dataHome, "Trash")
	filesDir := filepath.Join(trashRoot, "files")
	infoDir := filepath.Join(trashRoot, "info")
	if err := os.MkdirAll(filesDir, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(infoDir, 0o700); err != nil {
		return err
	}

	for _, source := range paths {
		if err := movePathToFreeDesktopTrash(source, filesDir, infoDir); err != nil {
			return err
		}
	}
	return nil
}

func movePathToFreeDesktopTrash(source, filesDir, infoDir string) error {
	base := filepath.Base(source)
	for suffix := 0; ; suffix++ {
		name := base
		if suffix > 0 {
			name = fmt.Sprintf("%s.%d", base, suffix)
		}
		target := filepath.Join(filesDir, name)
		if _, err := os.Lstat(target); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		infoPath := filepath.Join(infoDir, name+".trashinfo")
		info, err := os.OpenFile(infoPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		contents := fmt.Sprintf(
			"[Trash Info]\nPath=%s\nDeletionDate=%s\n",
			freeDesktopTrashEscape(source),
			time.Now().Local().Format("2006-01-02T15:04:05"),
		)
		if _, err := info.WriteString(contents); err != nil {
			info.Close()
			os.Remove(infoPath)
			return err
		}
		if err := info.Sync(); err != nil {
			info.Close()
			os.Remove(infoPath)
			return err
		}
		if err := info.Close(); err != nil {
			os.Remove(infoPath)
			return err
		}
		if err := os.Rename(source, target); err != nil {
			os.Remove(infoPath)
			if errors.Is(err, syscall.EXDEV) {
				return fmt.Errorf("%w: cross-device trash requires gio", ErrTrashUnsupported)
			}
			return err
		}
		return nil
	}
}

func freeDesktopTrashEscape(value string) string {
	const hex = "0123456789ABCDEF"
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		b := value[i]
		if (b >= 'a' && b <= 'z') ||
			(b >= 'A' && b <= 'Z') ||
			(b >= '0' && b <= '9') ||
			strings.ContainsRune("-._~/", rune(b)) {
			out.WriteByte(b)
			continue
		}
		out.WriteByte('%')
		out.WriteByte(hex[b>>4])
		out.WriteByte(hex[b&0x0F])
	}
	return out.String()
}
