// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: msixassets <output-dir>")
	}

	src, err := readPNG(filepath.Join("assets", "hexone_icon_art.png"))
	if err != nil {
		log.Fatal(err)
	}

	outDir := os.Args[1]
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	for _, asset := range []struct {
		name string
		size int
	}{
		{name: "logo.png", size: 50},
		{name: "Square44x44Logo.png", size: 44},
		{name: "Square150x150Logo.png", size: 150},
	} {
		if err := writeResizedPNG(filepath.Join(outDir, asset.name), src, asset.size); err != nil {
			log.Fatal(err)
		}
	}
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return img, nil
}

func writeResizedPNG(path string, src image.Image, size int) error {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := png.Encode(f, dst); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return nil
}
