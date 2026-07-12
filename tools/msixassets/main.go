// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"hexone/appicon"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

type assetSpec struct {
	name string
	size int
}

var assetSpecs = msixAssetSpecs()

func msixAssetSpecs() []assetSpec {
	assets := []assetSpec{
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
		{name: "Square150x150Logo.png", size: 150},
		{name: "Square150x150Logo.scale-125.png", size: 188},
		{name: "Square150x150Logo.scale-150.png", size: 225},
		{name: "Square150x150Logo.scale-200.png", size: 300},
		{name: "Square150x150Logo.scale-400.png", size: 600},
	}

	// Square44x44Logo is the resource referenced by the manifest. Target-size
	// variants supply the shell app icon; alternate forms prevent Windows from
	// shrinking the artwork onto an automatic contrast plate.
	targetSizes := []int{16, 20, 24, 30, 32, 36, 40, 44, 48, 60, 64, 72, 80, 96, 256}
	for _, size := range targetSizes {
		for _, suffix := range []string{"", "_altform-unplated", "_altform-lightunplated"} {
			assets = append(assets, assetSpec{
				name: "Square44x44Logo.targetsize-" + strconv.Itoa(size) + suffix + ".png",
				size: size,
			})
		}
	}
	return assets
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
