// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package appdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsDataDirPortable(t *testing.T) {
	executable := filepath.Join(`C:\portable\hexone`, "hexone.exe")
	want := filepath.Dir(executable)
	if got := windowsDataDir("", "", executable); got != want {
		t.Fatalf("windowsDataDir() = %q, want %q", got, want)
	}
}

func TestWindowsDataDirMSIX(t *testing.T) {
	const packageFamilyName = "RamnasCvirka.hexone_wgc727vgx32zp"
	userCacheDir := filepath.Join(`C:\Users\tester`, "AppData", "Local")
	want := filepath.Join(userCacheDir, "Packages", packageFamilyName, "LocalState")
	if got := windowsDataDir(packageFamilyName, userCacheDir, ""); got != want {
		t.Fatalf("windowsDataDir() = %q, want %q", got, want)
	}
}

func TestCurrentProcessIsUnpackaged(t *testing.T) {
	if got := currentPackageFamilyName(); got != "" {
		t.Fatalf("currentPackageFamilyName() = %q for unpackaged test process", got)
	}
}

func TestPortablePathsUseExecutableDirectory(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable(): %v", err)
	}
	base := filepath.Dir(executable)
	tests := map[string]struct {
		got  string
		want string
	}{
		"config directory": {got: ConfigDir(), want: base},
		"config":           {got: ConfigPath(), want: filepath.Join(base, configFileName)},
		"session":          {got: SessionPath(), want: filepath.Join(base, sessionFileName)},
		"protocol":         {got: ProtocolPath(), want: filepath.Join(base, protocolsFileName)},
		"protocol sample":  {got: ProtocolSamplePath(), want: ""},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("path = %q, want %q", test.got, test.want)
			}
		})
	}
}
