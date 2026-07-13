// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"hexone/appicon"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestAssetSpecsRenderAtDeclaredSizes(t *testing.T) {
	dir := t.TempDir()
	seen := make(map[string]struct{}, len(assetSpecs))
	for _, asset := range assetSpecs {
		if _, ok := seen[asset.name]; ok {
			t.Fatalf("duplicate asset name %q", asset.name)
		}
		seen[asset.name] = struct{}{}

		path := filepath.Join(dir, asset.name)
		if err := appicon.WriteWindowsPackagePNG(path, asset.size); err != nil {
			t.Fatalf("WriteWindowsPackagePNG(%q): %v", asset.name, err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatalf("open %q: %v", asset.name, err)
		}
		config, err := png.DecodeConfig(file)
		file.Close()
		if err != nil {
			t.Fatalf("decode %q: %v", asset.name, err)
		}
		if config.Width != asset.size || config.Height != asset.size {
			t.Fatalf("%s is %dx%d, want %dx%d", asset.name, config.Width, config.Height, asset.size, asset.size)
		}
		if info, err := os.Stat(path); err != nil {
			t.Fatalf("stat %q: %v", asset.name, err)
		} else if info.Size() >= 204800 {
			t.Errorf("%s is %d bytes; Windows package images must be smaller than 204800 bytes", asset.name, info.Size())
		}
	}

	for _, required := range []string{
		"StoreLogo.png",
		"StoreLogo.scale-200.png",
		"StoreLogo.scale-400.png",
		"Square44x44Logo.png",
		"Square44x44Logo.targetsize-20.png",
		"Square44x44Logo.targetsize-256.png",
		"Square44x44Logo.targetsize-44_altform-unplated.png",
		"Square44x44Logo.targetsize-96_altform-lightunplated.png",
		"Square44x44Logo.targetsize-256_altform-unplated.png",
		"Square150x150Logo.png",
	} {
		if _, ok := seen[required]; !ok {
			t.Errorf("required asset %q is missing", required)
		}
	}
}

func TestAppIconHasEveryDocumentedTargetSizeAndThemeForm(t *testing.T) {
	seen := make(map[string]struct{}, len(assetSpecs))
	for _, asset := range assetSpecs {
		seen[asset.name] = struct{}{}
	}
	for _, size := range []int{16, 20, 24, 30, 32, 36, 40, 44, 48, 60, 64, 72, 80, 96, 256} {
		for _, suffix := range []string{"", "_altform-unplated", "_altform-lightunplated"} {
			name := fmt.Sprintf("Square44x44Logo.targetsize-%d%s.png", size, suffix)
			if _, ok := seen[name]; !ok {
				t.Errorf("required app icon variant %q is missing", name)
			}
		}
	}
}
