// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build pdfium

package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// testPDFWithText builds a single-page 200x100pt PDF drawing "Hello" at
// baseline (10, 50) in Helvetica 12.
func testPDFWithText() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	writeObject := func(n int, body string) {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", n, body)
	}
	content := "BT /F1 12 Tf 10 50 Td (Hello) Tj ET"
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>")
	writeObject(4, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	writeObject(5, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))

	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", len(offsets))
	b.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return b.Bytes()
}

func TestPDFiumDocInfoReturnsPageSizes(t *testing.T) {
	if !viewerPDFPreviewBackend.Available() {
		t.Skip("pdfium backend unavailable")
	}
	info, err := viewerPDFPreviewBackend.DocInfo(viewerPDFRenderRequest{Data: testPDFWithText()})
	if err != nil {
		t.Fatalf("DocInfo: %v", err)
	}
	if info.PageCount != 1 || len(info.PageSizes) != 1 {
		t.Fatalf("PageCount=%d sizes=%d want 1/1", info.PageCount, len(info.PageSizes))
	}
	if info.PageSizes[0].W != 200 || info.PageSizes[0].H != 100 {
		t.Fatalf("page size=%+v want 200x100pt", info.PageSizes[0])
	}
}

func TestPDFiumPageTextFlipsToTopLeftOrigin(t *testing.T) {
	if !viewerPDFPreviewBackend.Available() {
		t.Skip("pdfium backend unavailable")
	}
	text, err := viewerPDFPreviewBackend.PageText(viewerPDFRenderRequest{Data: testPDFWithText(), Page: 0})
	if err != nil {
		t.Fatalf("PageText: %v", err)
	}
	var got strings.Builder
	for _, ch := range text.Chars {
		got.WriteRune(ch.Rune)
	}
	if !strings.Contains(got.String(), "Hello") {
		t.Fatalf("page text=%q want to contain %q", got.String(), "Hello")
	}
	for i, ch := range text.Chars {
		if ch.Rune == '\r' || ch.Rune == '\n' {
			continue
		}
		if ch.Top >= ch.Bottom {
			t.Fatalf("char %d %q: Top=%f Bottom=%f want top-left origin (Top < Bottom)", i, ch.Rune, ch.Top, ch.Bottom)
		}
		if ch.Top < 0 || ch.Bottom > 100 {
			t.Fatalf("char %d %q: rect outside page: Top=%f Bottom=%f", i, ch.Rune, ch.Top, ch.Bottom)
		}
		// Baseline is 50pt from the bottom of a 100pt page, so glyphs sit in
		// the upper half after flipping.
		if ch.Bottom > 55 {
			t.Fatalf("char %d %q: Bottom=%f want <= 55 (upper half of the page)", i, ch.Rune, ch.Bottom)
		}
	}
}
