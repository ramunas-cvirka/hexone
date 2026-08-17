// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"
)

const backupSuffix = ".bak"

func Load(path string) (*File, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("HTTP collection path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse HTTP collections %s: %w", path, err)
	}
	if file.Version > CurrentVersion {
		return nil, fmt.Errorf("HTTP collections version %d is newer than supported version %d", file.Version, CurrentVersion)
	}
	file.Normalize()
	return &file, nil
}

func LoadOrCreate(path string) (*File, error) {
	file, err := Load(path)
	if err == nil {
		return file, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	file = DefaultFile()
	if err := Save(path, file); err != nil {
		return nil, err
	}
	return file, nil
}

func Save(path string, file *File) error {
	return save(path, file, true)
}

func save(path string, file *File, backup bool) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("HTTP collection path is empty")
	}
	if file == nil {
		return errors.New("HTTP collections are nil")
	}
	file.Normalize()
	data, err := yaml.Marshal(file)
	if err != nil {
		return err
	}
	return writeAtomic(path, data, backup)
}

func writeAtomic(path string, data []byte, backup bool) error {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	mode := os.FileMode(0o600)
	if _, err := os.Stat(path); err == nil {
		if backup {
			if err := copyFile(path, path+backupSuffix, mode); err != nil {
				return fmt.Errorf("backup HTTP collections: %w", err)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := replaceFile(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, mode); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}
