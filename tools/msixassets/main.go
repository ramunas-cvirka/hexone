// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"hexone/appicon"
	"log"
	"os"
	"path/filepath"
)

var assetSpecs = []struct {
	name string
	size int
}{
	{name: "StoreLogo.png", size: 50},
	{name: "StoreLogo.scale-125.png", size: 63},
	{name: "StoreLogo.scale-150.png", size: 75},
	{name: "StoreLogo.scale-200.png", size: 100},
	{name: "StoreLogo.scale-400.png", size: 200},
	{name: "Square44x44Logo.png", size: 44},
	{name: "Square44x44Logo.scale-125.png", size: 55},
	{name: "Square44x44Logo.scale-150.png", size: 66},
	{name: "Square44x44Logo.scale-200.png", size: 88},
	{name: "Square44x44Logo.scale-400.png", size: 176},
	{name: "Square44x44Logo.targetsize-16.png", size: 16},
	{name: "Square44x44Logo.targetsize-24.png", size: 24},
	{name: "Square44x44Logo.targetsize-32.png", size: 32},
	{name: "Square44x44Logo.targetsize-44.png", size: 44},
	{name: "Square44x44Logo.targetsize-48.png", size: 48},
	{name: "Square44x44Logo.targetsize-256.png", size: 256},
	{name: "Square44x44Logo.targetsize-16_altform-unplated.png", size: 16},
	{name: "Square44x44Logo.targetsize-24_altform-unplated.png", size: 24},
	{name: "Square44x44Logo.targetsize-32_altform-unplated.png", size: 32},
	{name: "Square44x44Logo.targetsize-44_altform-unplated.png", size: 44},
	{name: "Square44x44Logo.targetsize-48_altform-unplated.png", size: 48},
	{name: "Square44x44Logo.targetsize-256_altform-unplated.png", size: 256},
	{name: "Square150x150Logo.png", size: 150},
	{name: "Square150x150Logo.scale-125.png", size: 188},
	{name: "Square150x150Logo.scale-150.png", size: 225},
	{name: "Square150x150Logo.scale-200.png", size: 300},
	{name: "Square150x150Logo.scale-400.png", size: 600},
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: msixassets <output-dir>")
	}

	outDir := os.Args[1]
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	for _, asset := range assetSpecs {
		if err := appicon.WriteWindowsPackagePNG(filepath.Join(outDir, asset.name), asset.size); err != nil {
			log.Fatal(err)
		}
	}
}
