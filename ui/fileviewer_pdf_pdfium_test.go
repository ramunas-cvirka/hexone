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
	return testPDFWithTextPage("")
}

// testPDFWithTextPage is testPDFWithText with extra page dictionary entries
// (e.g. "/CropBox [...]" or "/Rotate 90") spliced into the page object.
func testPDFWithTextPage(pageExtra string) []byte {
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
	writeObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] "+pageExtra+" /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>")
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

// testPDFWithLinks builds a two-page 200x100pt PDF whose first page has two
// link annotations: an internal GoTo link over [10 40 100 60] targeting page
// two, and an external URI link.
func testPDFWithLinks() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	writeObject := func(n int, body string) {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", n, body)
	}
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, "<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>")
	writeObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Annots [5 0 R 6 0 R] >>")
	writeObject(4, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>")
	writeObject(5, "<< /Type /Annot /Subtype /Link /Rect [10 40 100 60] /Border [0 0 0] /Dest [4 0 R /XYZ null null null] >>")
	writeObject(6, "<< /Type /Annot /Subtype /Link /Rect [110 10 150 30] /Border [0 0 0] /A << /S /URI /URI (https://example.com) >> >>")

	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", len(offsets))
	b.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return b.Bytes()
}

func testPDFWithBookmarks() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	writeObject := func(n int, body string) {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", n, body)
	}
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R /Outlines 5 0 R >>")
	writeObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>")
	writeObject(4, "<< >>")
	writeObject(5, "<< /Type /Outlines /First 6 0 R /Last 7 0 R /Count 3 >>")
	writeObject(6, "<< /Title (Introduction) /Parent 5 0 R /Next 7 0 R /First 8 0 R /Last 8 0 R /Count 1 /Dest [3 0 R /Fit] >>")
	writeObject(7, "<< /Title (Details) /Parent 5 0 R /Prev 6 0 R /Dest [3 0 R /Fit] >>")
	writeObject(8, "<< /Title (Getting Started) /Parent 6 0 R /Dest [3 0 R /Fit] >>")

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

// TestPDFiumPageTextHonorsCropBoxOrigin covers PDFs whose displayed area
// (CropBox) does not start at the user-space origin: pdfium renders and
// reports page sizes against the crop box, but char boxes stay in raw user
// space, so they must be shifted by the box origin.
func TestPDFiumPageTextHonorsCropBoxOrigin(t *testing.T) {
	if !viewerPDFPreviewBackend.Available() {
		t.Skip("pdfium backend unavailable")
	}
	// Crop is [5 40 195 90] => displayed page is 190x50pt and the "Hello"
	// baseline at user-space (10, 50) sits at display (5, 40).
	data := testPDFWithTextPage("/CropBox [5 40 195 90]")
	info, err := viewerPDFPreviewBackend.DocInfo(viewerPDFRenderRequest{Data: data})
	if err != nil {
		t.Fatalf("DocInfo: %v", err)
	}
	if info.PageSizes[0].W != 190 || info.PageSizes[0].H != 50 {
		t.Fatalf("page size=%+v want 190x50pt", info.PageSizes[0])
	}
	text, err := viewerPDFPreviewBackend.PageText(viewerPDFRenderRequest{Data: data, Page: 0})
	if err != nil {
		t.Fatalf("PageText: %v", err)
	}
	if len(text.Chars) == 0 || text.Chars[0].Rune != 'H' {
		t.Fatalf("chars=%+v want to start with 'H'", text.Chars)
	}
	for i, ch := range text.Chars {
		if ch.Rune == '\r' || ch.Rune == '\n' {
			continue
		}
		if ch.Left < 0 || ch.Right > 190 || ch.Top < 0 || ch.Bottom > 50 {
			t.Fatalf("char %d %q: box (%f,%f)-(%f,%f) outside 190x50 crop", i, ch.Rune, ch.Left, ch.Top, ch.Right, ch.Bottom)
		}
	}
	h := text.Chars[0]
	// 12pt Helvetica 'H' on baseline user y=50: display left ~= 10-5, bottom
	// ~= 90-50, top ~= bottom minus cap height (~8.6pt).
	if h.Left < 4 || h.Left > 8 {
		t.Fatalf("'H' Left=%f want ~5", h.Left)
	}
	if h.Bottom < 38 || h.Bottom > 42 {
		t.Fatalf("'H' Bottom=%f want ~40", h.Bottom)
	}
	if h.Top < 28 || h.Top > 34 {
		t.Fatalf("'H' Top=%f want ~31", h.Top)
	}
}

// TestPDFiumPageTextHonorsPageRotation covers /Rotate 90 pages: the bitmap
// is rendered rotated, so char boxes must be rotated into display space too.
func TestPDFiumPageTextHonorsPageRotation(t *testing.T) {
	if !viewerPDFPreviewBackend.Available() {
		t.Skip("pdfium backend unavailable")
	}
	data := testPDFWithTextPage("/Rotate 90")
	info, err := viewerPDFPreviewBackend.DocInfo(viewerPDFRenderRequest{Data: data})
	if err != nil {
		t.Fatalf("DocInfo: %v", err)
	}
	if info.PageSizes[0].W != 100 || info.PageSizes[0].H != 200 {
		t.Fatalf("page size=%+v want 100x200pt", info.PageSizes[0])
	}
	text, err := viewerPDFPreviewBackend.PageText(viewerPDFRenderRequest{Data: data, Page: 0})
	if err != nil {
		t.Fatalf("PageText: %v", err)
	}
	if len(text.Chars) == 0 || text.Chars[0].Rune != 'H' {
		t.Fatalf("chars=%+v want to start with 'H'", text.Chars)
	}
	for i, ch := range text.Chars {
		if ch.Rune == '\r' || ch.Rune == '\n' {
			continue
		}
		if ch.Left < 0 || ch.Right > 100 || ch.Top < 0 || ch.Bottom > 200 {
			t.Fatalf("char %d %q: box (%f,%f)-(%f,%f) outside rotated 100x200 page", i, ch.Rune, ch.Left, ch.Top, ch.Right, ch.Bottom)
		}
	}
	// Rotated 90 degrees clockwise, the baseline at user y=50 becomes a
	// vertical line at display x=50 with glyphs extending right of it, and
	// the 'H' at user x~10 lands near display y~10.
	h := text.Chars[0]
	if h.Left < 48 || h.Left > 52 {
		t.Fatalf("'H' Left=%f want ~50", h.Left)
	}
	if h.Right < 56 || h.Right > 61 {
		t.Fatalf("'H' Right=%f want ~59", h.Right)
	}
	if h.Top < 8 || h.Top > 13 {
		t.Fatalf("'H' Top=%f want ~10", h.Top)
	}
}

// TestPDFiumPageLinksReturnsInternalLinks checks that link annotations with
// an internal destination come back with top-left-origin display rects and a
// resolved destination page, while external URI links are skipped.
func TestPDFiumPageLinksReturnsInternalLinks(t *testing.T) {
	if !viewerPDFPreviewBackend.Available() {
		t.Skip("pdfium backend unavailable")
	}
	links, err := viewerPDFPreviewBackend.PageLinks(viewerPDFRenderRequest{Data: testPDFWithLinks(), Page: 0})
	if err != nil {
		t.Fatalf("PageLinks: %v", err)
	}
	if links.Page != 0 {
		t.Fatalf("Page=%d want 0", links.Page)
	}
	if len(links.Links) != 1 {
		t.Fatalf("links=%+v want exactly the one internal link", links.Links)
	}
	l := links.Links[0]
	if l.DestPage != 1 {
		t.Fatalf("DestPage=%d want 1", l.DestPage)
	}
	// Rect [10 40 100 60] on a 100pt-tall page flips to top-left origin.
	if l.Left != 10 || l.Top != 40 || l.Right != 100 || l.Bottom != 60 {
		t.Fatalf("rect=(%f,%f)-(%f,%f) want (10,40)-(100,60)", l.Left, l.Top, l.Right, l.Bottom)
	}

	empty, err := viewerPDFPreviewBackend.PageLinks(viewerPDFRenderRequest{Data: testPDFWithLinks(), Page: 1})
	if err != nil {
		t.Fatalf("PageLinks page 1: %v", err)
	}
	if len(empty.Links) != 0 {
		t.Fatalf("page 1 links=%+v want none", empty.Links)
	}
}

func TestPDFiumTOCReportsNoEntriesForDocumentWithoutOutlines(t *testing.T) {
	if !viewerPDFPreviewBackend.Available() {
		t.Skip("pdfium backend unavailable")
	}
	toc, err := viewerPDFPreviewBackend.TOC(viewerPDFRenderRequest{Data: testPDFWithText()})
	if err != nil {
		t.Fatalf("TOC: %v", err)
	}
	if len(toc) != 0 {
		t.Fatalf("TOC=%+v want no entries", toc)
	}
}

func TestPDFiumTOCReturnsNavigableBookmarks(t *testing.T) {
	if !viewerPDFPreviewBackend.Available() {
		t.Skip("pdfium backend unavailable")
	}
	toc, err := viewerPDFPreviewBackend.TOC(viewerPDFRenderRequest{Data: testPDFWithBookmarks()})
	if err != nil {
		t.Fatalf("TOC: %v", err)
	}
	if len(toc) != 3 {
		t.Fatalf("TOC=%+v want three entries", toc)
	}
	if toc[0].Title != "Introduction" || toc[0].Page != 0 || !toc[0].HasChildren ||
		toc[1].Title != "Getting Started" || toc[1].Page != 0 || toc[1].Level != 1 || toc[1].ParentID != toc[0].ID ||
		toc[2].Title != "Details" || toc[2].Page != 0 || toc[2].Level != 0 {
		t.Fatalf("TOC=%+v", toc)
	}
}
