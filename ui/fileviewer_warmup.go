// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"context"
	"fmt"
	"hexone/fm"
	"image"
	"image/png"
	"sync"
	"time"
)

const (
	viewerWarmupInitialDelay = 350 * time.Millisecond
	viewerWarmupPDFDelay     = 900 * time.Millisecond
)

var viewerWarmupOnce sync.Once

// StartViewerWarmup primes the expensive viewer runtime paths after startup.
// It is intentionally called from the real application entrypoint, not NewUI,
// so tests that construct many UI values do not repeatedly start background
// warmup work.
func StartViewerWarmup() {
	viewerWarmupOnce.Do(func() {
		go runViewerWarmup(viewerWarmupInitialDelay, viewerWarmupPDFDelay)
	})
}

func runViewerWarmup(initialDelay, pdfDelay time.Duration) {
	if initialDelay > 0 {
		time.Sleep(initialDelay)
	}
	warmViewerTextPipeline()
	warmViewerImagePipeline()

	if pdfDelay > 0 {
		time.Sleep(pdfDelay)
	}
	warmViewerPDFPipeline()
}

func warmViewerTextPipeline() {
	samples := []struct {
		path    string
		content string
	}{
		{
			path:    "warmup.go",
			content: "package main\n\nfunc main() {\n\tprintln(\"hexone\")\n}\n",
		},
		{
			path:    "warmup.json",
			content: "{\n  \"viewer\": true,\n  \"rows\": [1, 2, 3]\n}\n",
		},
		{
			path:    "warmup.yaml",
			content: "viewer:\n  mode: file\n  warmup: true\n",
		},
		{
			path:    "warmup.sh",
			content: "#!/bin/sh\nprintf '%s\\n' \"$PWD\"\n",
		},
	}
	for _, sample := range samples {
		content, info := decodeViewerText(sample.path, []byte(sample.content), fm.ViewerFileEncodingAuto)
		if info.binaryPreview {
			continue
		}
		content = normalizeViewerLineEndings(content)
		content = sanitizeViewerContent(content)
		_ = viewerBuildSyntaxDocument(context.Background(), sample.path, content)
	}
}

func warmViewerImagePipeline() {
	data := viewerWarmupPNGData()
	if len(data) == 0 {
		return
	}
	_, _ = decodeViewerImagePreview("warmup.png", data)
}

func warmViewerPDFPipeline() {
	_, _ = viewerPDFPreviewBackend.RenderPage(viewerPDFRenderRequest{
		Data:  viewerWarmupPDFData(),
		Page:  0,
		Width: 8,
	})
}

func viewerWarmupPNGData() []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		return nil
	}
	return b.Bytes()
}

func viewerWarmupPDFData() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	writeObject := func(n int, body string) {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", n, body)
	}
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 8 8] /Resources << >> /Contents 4 0 R >>")
	writeObject(4, "<< /Length 0 >>\nstream\n\nendstream")

	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", len(offsets))
	b.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return b.Bytes()
}
