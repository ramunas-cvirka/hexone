// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
	"sync"
)

const operationCompleteSoundFileName = "operation-complete-low-soft-confirm.wav"

//go:embed operation_complete.wav
var operationCompleteSoundWAV []byte

type operationSoundFileState struct {
	once sync.Once
	path string
	err  error
}

var (
	operationSoundCacheDir = os.UserCacheDir
	operationSoundFile     = &operationSoundFileState{}
)

// PlayOperationComplete plays a short best-effort desktop completion sound.
func PlayOperationComplete() {
	go playOperationComplete()
}

func operationCompleteSoundPath() (string, error) {
	operationSoundFile.once.Do(func() {
		dir, err := operationSoundCacheDir()
		if err != nil || dir == "" {
			dir = os.TempDir()
		}
		dir = filepath.Join(dir, "hexone")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			operationSoundFile.err = err
			return
		}

		path := filepath.Join(dir, operationCompleteSoundFileName)
		if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, operationCompleteSoundWAV) {
			operationSoundFile.path = path
			return
		}
		if err := os.WriteFile(path, operationCompleteSoundWAV, 0o600); err != nil {
			operationSoundFile.err = err
			return
		}
		operationSoundFile.path = path
	})
	return operationSoundFile.path, operationSoundFile.err
}
