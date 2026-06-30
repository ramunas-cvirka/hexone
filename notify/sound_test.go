// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestOperationCompleteSoundPathWritesBundledWAV(t *testing.T) {
	oldCacheDir := operationSoundCacheDir
	oldFile := operationSoundFile
	t.Cleanup(func() {
		operationSoundCacheDir = oldCacheDir
		operationSoundFile = oldFile
	})

	cacheDir := t.TempDir()
	operationSoundCacheDir = func() (string, error) {
		return cacheDir, nil
	}
	operationSoundFile = &operationSoundFileState{}

	path, err := operationCompleteSoundPath()
	if err != nil {
		t.Fatalf("operationCompleteSoundPath error = %v", err)
	}
	if got, want := path, filepath.Join(cacheDir, "hexone", operationCompleteSoundFileName); got != want {
		t.Fatalf("sound path = %q, want %q", got, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sound path: %v", err)
	}
	if !bytes.Equal(data, operationCompleteSoundWAV) {
		t.Fatal("written sound data does not match embedded WAV")
	}
}
