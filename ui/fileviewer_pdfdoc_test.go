// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"math"
	"testing"
)

func testPDFDocView(pages int, w, h float64, viewport image.Rectangle) *pdfDocView {
	v := &pdfDocView{}
	v.viewportRect = viewport
	sizes := make([]viewerPDFPageSize, pages)
	for i := range sizes {
		sizes[i] = viewerPDFPageSize{W: w, H: h}
	}
	v.configure(viewerPDFDocInfo{PageCount: pages, PageSizes: sizes})
	return v
}

func TestPDFDocViewLayoutStacksPagesWithGaps(t *testing.T) {
	v := testPDFDocView(3, 100, 200, image.Rect(0, 0, 100, 200))

	// fit scale: min(100/100, 200/200) == 1
	if got := v.effectiveScale(); math.Abs(got-1) > 0.001 {
		t.Fatalf("effectiveScale=%f want 1", got)
	}
	wantH := 3*200 + 2*pdfDocPageGapPx
	if got := v.contentH; math.Abs(got-float64(wantH)) > 0.5 {
		t.Fatalf("contentH=%f want %d", got, wantH)
	}
	if got := v.layoutTops[1]; math.Abs(got-(200+pdfDocPageGapPx)) > 0.5 {
		t.Fatalf("tops[1]=%f want %d", got, 200+pdfDocPageGapPx)
	}
}

func TestPDFDocViewFitsPageWidthToViewport(t *testing.T) {
	v := testPDFDocView(2, 612, 792, image.Rect(0, 0, 400, 300))

	// Zoom 1 is fit-width: the page spans the viewport width and is
	// taller than the window (scrolled, not shrunk).
	if got := v.layoutWidths[0]; math.Abs(got-400) > 0.5 {
		t.Fatalf("page width=%f want 400 (fit width)", got)
	}
	wantH := 400.0 / 612.0 * 792.0
	if got := v.layoutHeights[0]; math.Abs(got-wantH) > 1 {
		t.Fatalf("page height=%f want %f (aspect preserved)", got, wantH)
	}
	if got := v.layoutHeights[0]; got <= 300 {
		t.Fatalf("page height=%f want > viewport height 300", got)
	}
}

func TestPDFDocViewScrollPositionRepresentsCombinedPages(t *testing.T) {
	v := testPDFDocView(10, 100, 200, image.Rect(0, 0, 100, 200))

	_, maxY := v.maxScroll()
	wantMax := v.contentH - 200
	if math.Abs(maxY-wantMax) > 0.5 {
		t.Fatalf("maxScroll=%f want %f (combined pages minus viewport)", maxY, wantMax)
	}
	v.scrollY = maxY
	v.computeLayout(image.Pt(100, 200), 10)
	if v.vTrackRect.Dy() <= 0 || v.vThumbRect.Dy() <= 0 {
		t.Fatal("expected a document scrollbar for a 10 page doc")
	}
	// The thumb length must reflect viewport/content ratio, not page count.
	ratio := float64(v.vThumbRect.Dy()) / float64(v.vTrackRect.Dy())
	wantRatio := float64(v.viewportRect.Dy()) / v.contentH
	if wantRatio < float64(fileViewerScrollbarMinThumbPx)/float64(v.vTrackRect.Dy()) {
		wantRatio = float64(fileViewerScrollbarMinThumbPx) / float64(v.vTrackRect.Dy())
	}
	if math.Abs(ratio-wantRatio) > 0.05 {
		t.Fatalf("thumb ratio=%f want ~%f", ratio, wantRatio)
	}
}

func TestPDFDocViewPageAtAndCurrentPage(t *testing.T) {
	v := testPDFDocView(5, 100, 200, image.Rect(0, 0, 100, 200))

	if got := v.pageAt(0); got != 0 {
		t.Fatalf("pageAt(0)=%d want 0", got)
	}
	if got := v.pageAt(v.layoutTops[3] + 5); got != 3 {
		t.Fatalf("pageAt(top3+5)=%d want 3", got)
	}
	// A point inside the gap resolves to the page above.
	if got := v.pageAt(v.layoutTops[1] - 2); got != 0 {
		t.Fatalf("pageAt(gap)=%d want 0", got)
	}
	v.scrollToPage(2)
	if got := v.currentPage(); got != 2 {
		t.Fatalf("currentPage=%d want 2", got)
	}
}

func TestPDFDocViewZoomAnchorsViewportCenter(t *testing.T) {
	v := testPDFDocView(4, 100, 200, image.Rect(0, 0, 100, 200))
	v.scrollY = 300
	centerBefore := (v.scrollY + 100) / v.effectiveScale()

	if !v.zoomBy(2) {
		t.Fatal("expected zoom change")
	}
	centerAfter := (v.scrollY + 100) / v.effectiveScale()
	if math.Abs(centerBefore-centerAfter) > 1 {
		t.Fatalf("zoom moved the anchor: before=%f after=%f (doc points)", centerBefore, centerAfter)
	}
	if got := v.effectiveZoom(); math.Abs(float64(got-2)) > 0.001 {
		t.Fatalf("zoom=%f want 2", got)
	}
	if !v.resetZoom() {
		t.Fatal("expected reset zoom change")
	}
	if got := v.effectiveZoom(); got != 1 {
		t.Fatalf("zoom=%f want 1 after reset", got)
	}
}

func TestPDFDocViewPanScrollsBothAxes(t *testing.T) {
	v := testPDFDocView(3, 100, 200, image.Rect(0, 0, 100, 200))
	v.zoomBy(3) // create horizontal overflow
	v.scrollX = 50
	v.scrollY = 50

	if !v.scrollBy(20, 30) {
		t.Fatal("expected pan scroll to move")
	}
	if v.scrollX != 70 || v.scrollY != 80 {
		t.Fatalf("scroll=(%f,%f) want (70,80)", v.scrollX, v.scrollY)
	}
	// Pan clamps at document edges.
	v.scrollBy(-1e6, -1e6)
	if v.scrollX != 0 || v.scrollY != 0 {
		t.Fatalf("scroll=(%f,%f) want origin after clamping", v.scrollX, v.scrollY)
	}
}

func testPDFDocPageTextLine(page int) viewerPDFPageText {
	// Three chars "abc" on one line, 10pt wide each, at y 20..30.
	chars := make([]viewerPDFTextChar, 0, 3)
	for i, r := range "abc" {
		chars = append(chars, viewerPDFTextChar{
			Rune:   r,
			Left:   10 + float64(i)*10,
			Top:    20,
			Right:  20 + float64(i)*10,
			Bottom: 30,
		})
	}
	return viewerPDFPageText{Page: page, Chars: chars}
}

func TestPDFDocViewTextSelectionAndCopy(t *testing.T) {
	v := testPDFDocView(2, 100, 200, image.Rect(0, 0, 100, 200))
	v.storeText(testPDFDocPageTextLine(0))
	v.storeText(testPDFDocPageTextLine(1))

	// Screen coords equal doc coords here (scale 1, no centering offsets).
	start, ok := v.textPosAt(image.Pt(11, 25))
	if !ok {
		t.Fatal("expected a text hit at the first char")
	}
	if start.Page != 0 || start.Index != 0 {
		t.Fatalf("start=%+v want page 0 index 0", start)
	}
	// End inside the second page's line, after the second char.
	endY := int(v.layoutTops[1]) + 26
	end, ok := v.textPosAt(image.Pt(26, endY))
	if !ok {
		t.Fatal("expected a text hit on page 1")
	}
	if end.Page != 1 || end.Index != 2 {
		t.Fatalf("end=%+v want page 1 index 2", end)
	}

	v.selActive = true
	v.selStart = start
	v.selEnd = end
	if got := v.selectedText(); got != "abcab" {
		t.Fatalf("selectedText=%q want %q", got, "abcab")
	}

	// Reversed selection yields the same text.
	v.selStart, v.selEnd = v.selEnd, v.selStart
	if got := v.selectedText(); got != "abcab" {
		t.Fatalf("reversed selectedText=%q want %q", got, "abcab")
	}

	rects := v.selectionRectsOnPage(0)
	if len(rects) != 1 {
		t.Fatalf("selection rects on page 0=%d want 1 merged line run", len(rects))
	}
	if rects[0][0] != 10 || rects[0][2] != 40 {
		t.Fatalf("merged run=%v want left 10 right 40", rects[0])
	}
}

func TestPDFDocViewPruneKeepsSelectionText(t *testing.T) {
	v := testPDFDocView(40, 100, 200, image.Rect(0, 0, 100, 200))
	for page := 0; page < 6; page++ {
		v.storeText(testPDFDocPageTextLine(page))
		v.storeRender(page, pdfDocPageRender{img: image.NewNRGBA(image.Rect(0, 0, 10, 20)), width: 10})
	}
	v.selActive = true
	v.selStart = pdfDocTextPos{Page: 0, Index: 0}
	v.selEnd = pdfDocTextPos{Page: 1, Index: 3}

	// Prune far away from the selection.
	v.prune(30, 32)

	if _, ok := v.text[0]; !ok {
		t.Fatal("selection text on page 0 must survive pruning")
	}
	if _, ok := v.text[1]; !ok {
		t.Fatal("selection text on page 1 must survive pruning")
	}
	if _, ok := v.text[5]; ok {
		t.Fatal("non-selected far text should be pruned")
	}
	if len(v.pages) != 0 {
		t.Fatalf("far page renders should be pruned, still cached: %d", len(v.pages))
	}
}

func TestPDFDocViewRenderWidthClamped(t *testing.T) {
	v := testPDFDocView(2, 612, 792, image.Rect(0, 0, 800, 600))
	v.zoom = pdfDocMaxZoom
	v.layoutScale = 0
	v.relayout()

	if got := v.renderWidthFor(0); got != pdfDocMaxRenderWidth {
		t.Fatalf("renderWidthFor=%d want clamp %d", got, pdfDocMaxRenderWidth)
	}
}

func TestPDFDocViewTextHitDecidesSelectVsPan(t *testing.T) {
	v := testPDFDocView(1, 100, 200, image.Rect(0, 0, 100, 200))
	v.storeText(testPDFDocPageTextLine(0))

	// On a glyph: selection anchor.
	if _, ok := v.textHitAt(image.Pt(15, 25)); !ok {
		t.Fatal("expected a text hit on a glyph")
	}
	// Empty page area well below the line: no hit, so a drag there pans.
	if _, ok := v.textHitAt(image.Pt(50, 120)); ok {
		t.Fatal("expected no text hit on empty page area")
	}
	// Nearest-caret lookup still works for Shift+drag from empty areas.
	if pos, ok := v.textPosAt(image.Pt(50, 120)); !ok || pos.Page != 0 {
		t.Fatalf("textPosAt=%+v ok=%v want nearest caret on page 0", pos, ok)
	}
}

func testPDFDocPageTextWords(page int) viewerPDFPageText {
	// "ab1 cd." — word chars, a space, then a word and punctuation.
	runes := []rune("ab1 cd.")
	chars := make([]viewerPDFTextChar, 0, len(runes))
	for i, r := range runes {
		chars = append(chars, viewerPDFTextChar{
			Rune:   r,
			Left:   10 + float64(i)*10,
			Top:    20,
			Right:  20 + float64(i)*10,
			Bottom: 30,
		})
	}
	return viewerPDFPageText{Page: page, Chars: chars}
}

func TestPDFDocViewDoubleClickSelectsWord(t *testing.T) {
	v := testPDFDocView(1, 100, 200, image.Rect(0, 0, 100, 200))
	v.storeText(testPDFDocPageTextWords(0))

	// Click in the middle of "ab1" (char index 1).
	if !v.selectWordAt(image.Pt(25, 25)) {
		t.Fatal("expected word selection on an alphanumeric char")
	}
	if got := v.selectedText(); got != "ab1" {
		t.Fatalf("selected word=%q want %q", got, "ab1")
	}
	// Click on "cd" (char index 4) selects only the alphanumeric run,
	// stopping at the punctuation.
	if !v.selectWordAt(image.Pt(55, 25)) {
		t.Fatal("expected word selection on second word")
	}
	if got := v.selectedText(); got != "cd" {
		t.Fatalf("selected word=%q want %q", got, "cd")
	}
	// A space is not a word char.
	if v.selectWordAt(image.Pt(45, 25)) {
		t.Fatal("space must not produce a word selection")
	}
	// Empty page area misses entirely.
	if v.selectWordAt(image.Pt(50, 150)) {
		t.Fatal("empty area must not produce a word selection")
	}
}

func TestPDFDocViewSelectionAutoScrollsAtViewportEdges(t *testing.T) {
	v := testPDFDocView(5, 100, 200, image.Rect(0, 0, 100, 200))
	for page := 0; page < 5; page++ {
		v.storeText(testPDFDocPageTextLine(page))
	}
	v.selecting = true
	v.selActive = true
	v.selStart = pdfDocTextPos{Page: 0, Index: 0}
	v.selEnd = v.selStart

	// Pointer held below the bottom edge scrolls down and extends the
	// selection toward the pointer.
	v.selLastPos = image.Pt(50, v.viewportRect.Max.Y+40)
	if !v.autoScrollSelection() {
		t.Fatal("expected auto-scroll while dragging past the bottom edge")
	}
	if v.scrollY <= 0 {
		t.Fatalf("scrollY=%f want > 0", v.scrollY)
	}
	for i := 0; i < 10000 && v.autoScrollSelection(); i++ {
	}
	_, maxY := v.maxScroll()
	if v.scrollY != maxY {
		t.Fatalf("scrollY=%f want %f (auto-scroll stops at the end)", v.scrollY, maxY)
	}
	if v.selEnd.Page != 4 {
		t.Fatalf("selEnd.Page=%d want 4 (selection extended across pages)", v.selEnd.Page)
	}

	// And back up past the top edge.
	v.selLastPos = image.Pt(50, v.viewportRect.Min.Y-40)
	if !v.autoScrollSelection() {
		t.Fatal("expected auto-scroll while dragging past the top edge")
	}
	if v.scrollY >= maxY {
		t.Fatalf("scrollY=%f want < %f after scrolling up", v.scrollY, maxY)
	}

	// No auto-scroll when the pointer is inside the viewport or when no
	// selection drag is active.
	v.selLastPos = image.Pt(50, 100)
	if v.autoScrollSelection() {
		t.Fatal("no auto-scroll while the pointer is inside the viewport")
	}
	v.selecting = false
	v.selLastPos = image.Pt(50, v.viewportRect.Max.Y+40)
	if v.autoScrollSelection() {
		t.Fatal("no auto-scroll without an active selection drag")
	}
}
