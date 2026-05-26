// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"image"
	"strings"
	"testing"
)

func TestViewerWarmupPNGDecodes(t *testing.T) {
	data := viewerWarmupPNGData()
	info, ok := decodeViewerImagePreview("warmup.png", data)
	if !ok {
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		t.Fatalf("warmup png should decode as an image preview; media=%q format=%q config=%v err=%v", viewerBinaryMediaType("warmup.png", data), format, cfg, err)
	}
	if got, want := info.imageSize, image.Pt(1, 1); got != want {
		t.Fatalf("warmup png size=%v want %v", got, want)
	}
}

func TestViewerWarmupPDFDataLooksValid(t *testing.T) {
	data := string(viewerWarmupPDFData())
	for _, want := range []string{
		"%PDF-1.4\n",
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>",
		"xref\n0 5\n",
		"trailer\n<< /Size 5 /Root 1 0 R >>",
		"%%EOF\n",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("warmup pdf missing %q in:\n%s", want, data)
		}
	}
}
