// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"image/color"
	"math"
	"sort"
	"strings"
	"time"

	"hexone/filesys"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget/material"
)

const (
	pdfDocPageGapPx        = 14
	pdfDocMinZoom          = float32(0.25)
	pdfDocMaxZoom          = float32(8)
	pdfDocMinRenderWidth   = 64
	pdfDocMaxRenderWidth   = 2800
	pdfDocMaxInflight      = 2
	pdfDocInflightExpiry   = 5 * time.Second
	pdfDocPendingPollDelay = 40 * time.Millisecond
	// pdfDocFallbackPageWidthPt approximates US Letter width until the real
	// page sizes arrive from the renderer.
	pdfDocFallbackPageWidthPt = 612.0
	// pdfDocLinkClickSlopPx is how far the pointer may travel between press
	// and release for the gesture to still count as a link click rather
	// than a selection or pan drag.
	pdfDocLinkClickSlopPx = 4
)

// pdfDocPaperColor backs pages that have not been rasterized yet; pdfium
// renders pages on white, so this keeps loading seamless.
var pdfDocPaperColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}

// pdfDocTextPos is a caret position inside the document text: the boundary
// before char Index on page Page.
type pdfDocTextPos struct {
	Page  int
	Index int
}

func (a pdfDocTextPos) less(b pdfDocTextPos) bool {
	if a.Page != b.Page {
		return a.Page < b.Page
	}
	return a.Index < b.Index
}

type pdfDocPageRender struct {
	img   image.Image
	width int
}

type pdfDocInflight struct {
	width     int
	startedAt time.Time
}

// pdfDocResult carries one async result (doc info, page render, or page
// text) from a worker goroutine to the UI loop.
type pdfDocResult struct {
	seq    int
	err    string
	info   *viewerPDFDocInfo
	page   int
	render *pdfDocPageRender
	text   *viewerPDFPageText
	links  *viewerPDFPageLinks
	toc    *[]viewerPDFTOCEntry
}

// pdfDocView is a continuous, vertically stacked view over all pages of a
// PDF document. Scroll position is measured against the combined height of
// all pages, so the scrollbar represents the whole document.
type pdfDocView struct {
	pointerTag struct{}

	// Document geometry in PDF points.
	pageSizes   []viewerPDFPageSize
	infoLoaded  bool
	infoPending bool

	// Layout cache in document pixels at layoutScale.
	layoutScale   float64
	layoutTops    []float64
	layoutHeights []float64
	layoutWidths  []float64
	contentW      float64
	contentH      float64

	zoom float32 // 1 == fit-width

	surfaceRect  image.Rectangle
	viewportRect image.Rectangle

	scrollX     float64
	scrollY     float64
	visualX     float64
	visualY     float64
	visualReady bool
	visualAt    time.Time
	wheelCarryX float32
	wheelCarryY float32

	vTrackRect  image.Rectangle
	vThumbRect  image.Rectangle
	hTrackRect  image.Rectangle
	hThumbRect  image.Rectangle
	hoverVTrack bool
	hoverVThumb bool
	hoverHTrack bool
	hoverHThumb bool
	hoverText   bool
	hoverLink   bool

	vDragging bool
	vDragID   pointer.ID
	vDragGrab int

	hDragging bool
	hDragID   pointer.ID
	hDragGrab int

	panning bool
	panID   pointer.ID
	panLast f32.Point

	selecting  bool
	selID      pointer.ID
	selActive  bool
	selStart   pdfDocTextPos
	selEnd     pdfDocTextPos
	selLastPos image.Point

	// A press on a link annotation arms it; the link fires on release
	// unless the pointer dragged past the click slop in between.
	linkArmed    bool
	linkID       pointer.ID
	linkPressPos image.Point
	linkDest     int

	lastClickAt  time.Time
	lastClickPos image.Point

	pages         map[int]pdfDocPageRender
	text          map[int]viewerPDFPageText
	links         map[int][]viewerPDFPageLink
	toc           []viewerPDFTOCEntry
	tocLoaded     bool
	tocPending    bool
	textPending   map[int]time.Time
	linkPending   map[int]time.Time
	renderPending map[int]pdfDocInflight
}

func (v *pdfDocView) reset() {
	if v == nil {
		return
	}
	*v = pdfDocView{}
}

func (v *pdfDocView) pageCount() int {
	if v == nil {
		return 0
	}
	return len(v.pageSizes)
}

// seedFallbackSizes fills page sizes from the first rendered page's aspect
// ratio until the real document info arrives.
func (v *pdfDocView) seedFallbackSizes(pageCount int, firstPagePx image.Point) {
	if v == nil || v.infoLoaded || pageCount <= 0 {
		return
	}
	if len(v.pageSizes) == pageCount {
		return
	}
	aspect := 792.0 / 612.0
	if firstPagePx.X > 0 && firstPagePx.Y > 0 {
		aspect = float64(firstPagePx.Y) / float64(firstPagePx.X)
	}
	size := viewerPDFPageSize{
		W: pdfDocFallbackPageWidthPt,
		H: pdfDocFallbackPageWidthPt * aspect,
	}
	v.pageSizes = make([]viewerPDFPageSize, pageCount)
	for i := range v.pageSizes {
		v.pageSizes[i] = size
	}
	v.layoutScale = 0
}

// configure installs the real page sizes, preserving the current reading
// position (page and intra-page fraction).
func (v *pdfDocView) configure(info viewerPDFDocInfo) {
	if v == nil || info.PageCount <= 0 || len(info.PageSizes) != info.PageCount {
		return
	}
	page, frac := v.readingPosition()
	v.pageSizes = info.PageSizes
	v.infoLoaded = true
	v.infoPending = false
	v.layoutScale = 0
	v.relayout()
	v.restoreReadingPosition(page, frac)
}

// readingPosition captures the topmost visible page and how far into it the
// viewport top currently is.
func (v *pdfDocView) readingPosition() (int, float64) {
	if v == nil || len(v.layoutTops) == 0 {
		return 0, 0
	}
	page := v.pageAt(v.scrollY)
	frac := 0.0
	if h := v.layoutHeights[page]; h > 0 {
		frac = (v.scrollY - v.layoutTops[page]) / h
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
	}
	return page, frac
}

func (v *pdfDocView) restoreReadingPosition(page int, frac float64) {
	if v == nil || len(v.layoutTops) == 0 {
		return
	}
	if page < 0 {
		page = 0
	}
	if page >= len(v.layoutTops) {
		page = len(v.layoutTops) - 1
	}
	v.scrollY = v.layoutTops[page] + frac*v.layoutHeights[page]
	v.clampScroll()
	v.syncVisualScroll()
}

func (v *pdfDocView) effectiveZoom() float32 {
	if v == nil || v.zoom <= 0 {
		return 1
	}
	return v.zoom
}

// fitScale returns pixels-per-point so the widest page spans the viewport
// width at zoom 1 (fit-width).
func (v *pdfDocView) fitScale() float64 {
	if v == nil || len(v.pageSizes) == 0 {
		return 1
	}
	vw := float64(v.viewportRect.Dx())
	if vw <= 0 {
		return 1
	}
	maxW := 0.0
	for _, size := range v.pageSizes {
		maxW = math.Max(maxW, size.W)
	}
	if maxW <= 0 {
		return 1
	}
	return vw / maxW
}

func (v *pdfDocView) effectiveScale() float64 {
	scale := v.fitScale() * float64(v.effectiveZoom())
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return 1
	}
	return scale
}

func (v *pdfDocView) relayout() {
	if v == nil {
		return
	}
	scale := v.effectiveScale()
	if scale == v.layoutScale && len(v.layoutTops) == len(v.pageSizes) {
		return
	}
	n := len(v.pageSizes)
	v.layoutScale = scale
	v.layoutTops = make([]float64, n)
	v.layoutHeights = make([]float64, n)
	v.layoutWidths = make([]float64, n)
	v.contentW = 0
	y := 0.0
	for i, size := range v.pageSizes {
		w := size.W * scale
		h := size.H * scale
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		v.layoutTops[i] = y
		v.layoutHeights[i] = h
		v.layoutWidths[i] = w
		v.contentW = math.Max(v.contentW, w)
		y += h
		if i != n-1 {
			y += pdfDocPageGapPx
		}
	}
	v.contentH = y
}

// pageAt returns the page whose vertical band contains doc-space y. Points
// inside a gap resolve to the page above it.
func (v *pdfDocView) pageAt(y float64) int {
	n := len(v.layoutTops)
	if n == 0 {
		return 0
	}
	i := sort.Search(n, func(i int) bool {
		return v.layoutTops[i] > y
	}) - 1
	if i < 0 {
		i = 0
	}
	return i
}

// pageRange returns the inclusive page range overlapping doc-space [y0, y1].
func (v *pdfDocView) pageRange(y0, y1 float64) (int, int) {
	n := len(v.layoutTops)
	if n == 0 {
		return 0, -1
	}
	first := v.pageAt(y0)
	last := first
	for last+1 < n && v.layoutTops[last+1] < y1 {
		last++
	}
	return first, last
}

// currentPage is the page under the viewport center, used for the page
// label and page stepping.
func (v *pdfDocView) currentPage() int {
	if v == nil || len(v.layoutTops) == 0 {
		return 0
	}
	return v.pageAt(v.scrollY + float64(v.viewportRect.Dy())/2)
}

// pageDocRect returns the page rectangle in doc space (before viewport
// centering offsets are applied).
func (v *pdfDocView) pageDocRect(i int) (x, y, w, h float64) {
	if v == nil || i < 0 || i >= len(v.layoutTops) {
		return 0, 0, 0, 0
	}
	w = v.layoutWidths[i]
	h = v.layoutHeights[i]
	x = (v.contentW - w) / 2
	y = v.layoutTops[i]
	return x, y, w, h
}

func (v *pdfDocView) maxScroll() (float64, float64) {
	if v == nil {
		return 0, 0
	}
	maxX := v.contentW - float64(v.viewportRect.Dx())
	maxY := v.contentH - float64(v.viewportRect.Dy())
	return math.Max(0, maxX), math.Max(0, maxY)
}

func (v *pdfDocView) clampScroll() bool {
	if v == nil {
		return false
	}
	maxX, maxY := v.maxScroll()
	nextX := math.Min(math.Max(v.scrollX, 0), maxX)
	nextY := math.Min(math.Max(v.scrollY, 0), maxY)
	changed := nextX != v.scrollX || nextY != v.scrollY
	v.scrollX = nextX
	v.scrollY = nextY
	return changed
}

func (v *pdfDocView) scrollBy(dx, dy float64) bool {
	if v == nil || (dx == 0 && dy == 0) {
		return false
	}
	beforeX, beforeY := v.scrollX, v.scrollY
	v.scrollX += dx
	v.scrollY += dy
	v.clampScroll()
	return v.scrollX != beforeX || v.scrollY != beforeY
}

func (v *pdfDocView) scrollToPage(page int) bool {
	if v == nil || len(v.layoutTops) == 0 {
		return false
	}
	if page < 0 {
		page = 0
	}
	if page >= len(v.layoutTops) {
		page = len(v.layoutTops) - 1
	}
	before := v.scrollY
	v.scrollY = v.layoutTops[page]
	v.clampScroll()
	return v.scrollY != before
}

func (v *pdfDocView) scrollToStart() bool {
	if v == nil || (v.scrollX == 0 && v.scrollY == 0) {
		return false
	}
	v.scrollX = 0
	v.scrollY = 0
	return true
}

func (v *pdfDocView) scrollToEnd() bool {
	if v == nil {
		return false
	}
	_, maxY := v.maxScroll()
	if v.scrollY == maxY {
		return false
	}
	v.scrollY = maxY
	return true
}

func (v *pdfDocView) scrollByViewport(pages int) bool {
	if v == nil || pages == 0 {
		return false
	}
	step := v.viewportRect.Dy() - fileViewerImageKeyStepPx
	if step < fileViewerImageKeyStepPx {
		step = fileViewerImageKeyStepPx
	}
	return v.scrollBy(0, float64(pages*step))
}

// zoomBy scales the document around the viewport center. Zoom 1 keeps the
// page fitted to the viewport width.
func (v *pdfDocView) zoomBy(factor float32) bool {
	if v == nil || factor <= 0 {
		return false
	}
	return v.setZoom(v.effectiveZoom() * factor)
}

func (v *pdfDocView) setZoom(newZoom float32) bool {
	if v == nil || newZoom <= 0 {
		return false
	}
	oldZoom := v.effectiveZoom()
	if newZoom < pdfDocMinZoom {
		newZoom = pdfDocMinZoom
	}
	if newZoom > pdfDocMaxZoom {
		newZoom = pdfDocMaxZoom
	}
	if math.Abs(float64(newZoom-oldZoom)) < 0.0001 {
		return false
	}
	ratio := float64(newZoom) / float64(oldZoom)
	halfW := float64(v.viewportRect.Dx()) / 2
	halfH := float64(v.viewportRect.Dy()) / 2
	v.scrollX = (v.scrollX+halfW)*ratio - halfW
	v.scrollY = (v.scrollY+halfH)*ratio - halfH
	v.zoom = newZoom
	v.relayout()
	v.clampScroll()
	v.syncVisualScroll()
	return true
}

func (v *pdfDocView) resetZoom() bool {
	if v == nil || v.effectiveZoom() == 1 {
		return false
	}
	page, frac := v.readingPosition()
	v.zoom = 1
	v.relayout()
	v.restoreReadingPosition(page, frac)
	return true
}

func (v *pdfDocView) syncVisualScroll() {
	if v == nil {
		return
	}
	v.visualX = v.scrollX
	v.visualY = v.scrollY
	v.visualReady = true
	v.visualAt = time.Time{}
}

func (v *pdfDocView) smoothJumpLimit() float64 {
	limit := float64(fileViewerImageKeyStepPx * 6)
	if v == nil {
		return limit
	}
	if view := float64(v.viewportRect.Dx()) * 0.75; view > limit {
		limit = view
	}
	if view := float64(v.viewportRect.Dy()) * 0.75; view > limit {
		limit = view
	}
	return limit
}

func (v *pdfDocView) prepareVisualScroll(now time.Time, smooth bool) bool {
	if v == nil {
		return false
	}
	targetX := v.scrollX
	targetY := v.scrollY
	if !v.visualReady {
		v.visualX = targetX
		v.visualY = targetY
		v.visualReady = true
		v.visualAt = now
		return false
	}
	if !smooth || v.vDragging || v.hDragging || v.panning {
		v.visualX = targetX
		v.visualY = targetY
		v.visualAt = now
		return false
	}
	if math.Abs(targetX-v.visualX) > v.smoothJumpLimit() || math.Abs(targetY-v.visualY) > v.smoothJumpLimit() {
		v.visualX = targetX
		v.visualY = targetY
		v.visualAt = now
		return false
	}
	if v.visualAt.IsZero() {
		v.visualAt = now
	}
	dt := now.Sub(v.visualAt)
	if dt < 0 {
		dt = 0
	}
	if dt > 120*time.Millisecond {
		v.visualX = targetX
		v.visualY = targetY
		v.visualAt = now
		return false
	}
	if dt == 0 && (targetX != v.visualX || targetY != v.visualY) {
		dt = streamSmoothTick
	}
	if dt > 0 {
		blend := float64(1 - math.Exp(-float64(dt)/float64(streamSmoothTau)))
		if blend < 0 {
			blend = 0
		}
		if blend > 1 {
			blend = 1
		}
		v.visualX += (targetX - v.visualX) * blend
		v.visualY += (targetY - v.visualY) * blend
	}
	v.visualAt = now
	maxX, maxY := v.maxScroll()
	v.visualX = math.Min(math.Max(v.visualX, 0), maxX)
	v.visualY = math.Min(math.Max(v.visualY, 0), maxY)
	if math.Abs(targetX-v.visualX) < streamSmoothSnapEpsilon && math.Abs(targetY-v.visualY) < streamSmoothSnapEpsilon {
		v.visualX = targetX
		v.visualY = targetY
		return false
	}
	return true
}

func (v *pdfDocView) displayScroll() (int, int) {
	if v == nil {
		return 0, 0
	}
	x, y := v.scrollX, v.scrollY
	if v.visualReady {
		x, y = v.visualX, v.visualY
	}
	maxX, maxY := v.maxScroll()
	x = math.Min(math.Max(x, 0), maxX)
	y = math.Min(math.Max(y, 0), maxY)
	return int(math.Round(x)), int(math.Round(y))
}

// docOrigin returns the screen position of doc-space (0,0) given the
// currently displayed scroll offsets. Content smaller than the viewport is
// centered.
func (v *pdfDocView) docOrigin() image.Point {
	displayX, displayY := v.displayScroll()
	x := v.viewportRect.Min.X - displayX
	y := v.viewportRect.Min.Y - displayY
	if extra := float64(v.viewportRect.Dx()) - v.contentW; extra > 0 {
		x += int(extra / 2)
	}
	if extra := float64(v.viewportRect.Dy()) - v.contentH; extra > 0 {
		y += int(extra / 2)
	}
	return image.Pt(x, y)
}

func (v *pdfDocView) scrollWheelY(delta float32) bool {
	if v == nil || delta == 0 {
		return false
	}
	if (delta > 0 && v.wheelCarryY < 0) || (delta < 0 && v.wheelCarryY > 0) {
		v.wheelCarryY = 0
	}
	v.wheelCarryY += delta * fileViewerImageWheelStepPx
	step := int(v.wheelCarryY)
	if step == 0 {
		return false
	}
	v.wheelCarryY -= float32(step)
	return v.scrollBy(0, float64(step))
}

func (v *pdfDocView) scrollWheelX(delta float32) bool {
	if v == nil || delta == 0 {
		return false
	}
	if (delta > 0 && v.wheelCarryX < 0) || (delta < 0 && v.wheelCarryX > 0) {
		v.wheelCarryX = 0
	}
	v.wheelCarryX += delta * fileViewerImageWheelStepPx
	step := int(v.wheelCarryX)
	if step == 0 {
		return false
	}
	v.wheelCarryX -= float32(step)
	return v.scrollBy(float64(step), 0)
}

// renderWidthFor is the pixel width a page should be rasterized at for the
// current scale.
func (v *pdfDocView) renderWidthFor(page int) int {
	if v == nil || page < 0 || page >= len(v.layoutWidths) {
		return pdfDocMinRenderWidth
	}
	w := int(math.Round(v.layoutWidths[page]))
	if w < pdfDocMinRenderWidth {
		w = pdfDocMinRenderWidth
	}
	if w > pdfDocMaxRenderWidth {
		w = pdfDocMaxRenderWidth
	}
	return w
}

func (v *pdfDocView) storeRender(page int, render pdfDocPageRender) {
	if v == nil || render.img == nil || page < 0 {
		return
	}
	if v.pages == nil {
		v.pages = make(map[int]pdfDocPageRender, 8)
	}
	v.pages[page] = render
}

func (v *pdfDocView) storeText(text viewerPDFPageText) {
	if v == nil || text.Page < 0 {
		return
	}
	if v.text == nil {
		v.text = make(map[int]viewerPDFPageText, 8)
	}
	v.text[text.Page] = text
}

func (v *pdfDocView) storeLinks(links viewerPDFPageLinks) {
	if v == nil || links.Page < 0 {
		return
	}
	if v.links == nil {
		v.links = make(map[int][]viewerPDFPageLink, 8)
	}
	v.links[links.Page] = links.Links
}

// prune drops cached renders, text, and links far away from the visible
// range, keeping text for pages inside the current selection so copy keeps
// working.
func (v *pdfDocView) prune(first, last int) {
	if v == nil {
		return
	}
	selFirst, selLast := -1, -1
	if v.selActive {
		start, end := v.selectionRange()
		selFirst, selLast = start.Page, end.Page
	}
	for page := range v.pages {
		if page < first-2 || page > last+2 {
			delete(v.pages, page)
		}
	}
	for page := range v.text {
		if page >= first-4 && page <= last+4 {
			continue
		}
		if selFirst >= 0 && page >= selFirst && page <= selLast {
			continue
		}
		delete(v.text, page)
	}
	for page := range v.links {
		if page < first-4 || page > last+4 {
			delete(v.links, page)
		}
	}
}

func (v *pdfDocView) selectionRange() (pdfDocTextPos, pdfDocTextPos) {
	if v == nil || !v.selActive {
		return pdfDocTextPos{}, pdfDocTextPos{}
	}
	start, end := v.selStart, v.selEnd
	if end.less(start) {
		start, end = end, start
	}
	return start, end
}

func (v *pdfDocView) hasSelection() bool {
	if v == nil || !v.selActive {
		return false
	}
	return v.selStart != v.selEnd
}

func (v *pdfDocView) clearSelection() bool {
	if v == nil || !v.selActive {
		return false
	}
	v.selActive = false
	v.selecting = false
	v.selStart = pdfDocTextPos{}
	v.selEnd = pdfDocTextPos{}
	return true
}

// selectionOnPage returns the selected char index range [from, to) on the
// given page.
func (v *pdfDocView) selectionOnPage(page int) (int, int, bool) {
	if v == nil || !v.hasSelection() {
		return 0, 0, false
	}
	start, end := v.selectionRange()
	if page < start.Page || page > end.Page {
		return 0, 0, false
	}
	text, ok := v.text[page]
	if !ok {
		return 0, 0, false
	}
	from := 0
	to := len(text.Chars)
	if page == start.Page {
		from = start.Index
	}
	if page == end.Page {
		to = end.Index
	}
	if from < 0 {
		from = 0
	}
	if to > len(text.Chars) {
		to = len(text.Chars)
	}
	if from >= to {
		return 0, 0, false
	}
	return from, to, true
}

func (v *pdfDocView) selectedText() string {
	if v == nil || !v.hasSelection() {
		return ""
	}
	start, end := v.selectionRange()
	var b strings.Builder
	for page := start.Page; page <= end.Page; page++ {
		from, to, ok := v.selectionOnPage(page)
		if !ok {
			continue
		}
		text := v.text[page]
		for _, ch := range text.Chars[from:to] {
			if ch.Rune == '\r' {
				continue
			}
			b.WriteRune(ch.Rune)
		}
	}
	return b.String()
}

// textCharIndexAt finds the character directly under pos (with a small
// tolerance). Unlike textPosAt it does not snap to the nearest text: empty
// page areas miss. after reports whether pos is past the char's horizontal
// midpoint.
func (v *pdfDocView) textCharIndexAt(pos image.Point) (page, index int, after, ok bool) {
	if v == nil || len(v.layoutTops) == 0 || v.layoutScale <= 0 {
		return 0, 0, false, false
	}
	origin := v.docOrigin()
	docX := float64(pos.X - origin.X)
	docY := float64(pos.Y - origin.Y)
	page = v.pageAt(docY)
	text, loaded := v.text[page]
	if !loaded {
		return 0, 0, false, false
	}
	px, py, _, _ := v.pageDocRect(page)
	ptX := (docX - px) / v.layoutScale
	ptY := (docY - py) / v.layoutScale
	for i, ch := range text.Chars {
		w := ch.Right - ch.Left
		h := ch.Bottom - ch.Top
		if w <= 0 || h <= 0 {
			continue
		}
		pad := h * 0.25
		if ptX >= ch.Left-pad && ptX <= ch.Right+pad && ptY >= ch.Top-pad && ptY <= ch.Bottom+pad {
			return page, i, ptX > (ch.Left+ch.Right)/2, true
		}
	}
	return 0, 0, false, false
}

// textHitAt reports whether pos lies on a text character and returns the
// caret position to anchor a selection at. A drag on empty page areas pans
// instead of selecting.
func (v *pdfDocView) textHitAt(pos image.Point) (pdfDocTextPos, bool) {
	page, index, after, ok := v.textCharIndexAt(pos)
	if !ok {
		return pdfDocTextPos{}, false
	}
	if after {
		index++
	}
	return pdfDocTextPos{Page: page, Index: index}, true
}

func pdfDocWordRune(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// selectWordAt selects the alphanumeric run ([0-9a-zA-Z]+) under pos.
func (v *pdfDocView) selectWordAt(pos image.Point) bool {
	page, index, _, ok := v.textCharIndexAt(pos)
	if !ok {
		return false
	}
	chars := v.text[page].Chars
	if index >= len(chars) || !pdfDocWordRune(chars[index].Rune) {
		return false
	}
	start := index
	for start > 0 && pdfDocWordRune(chars[start-1].Rune) {
		start--
	}
	end := index + 1
	for end < len(chars) && pdfDocWordRune(chars[end].Rune) {
		end++
	}
	v.selActive = true
	v.selecting = false
	v.selStart = pdfDocTextPos{Page: page, Index: start}
	v.selEnd = pdfDocTextPos{Page: page, Index: end}
	return true
}

// autoScrollSelection scrolls the document while a selection drag is held
// past the top or bottom viewport edge and extends the selection toward
// the pointer. It returns true while the scroll is still moving.
func (v *pdfDocView) autoScrollSelection() bool {
	if v == nil || !v.selecting {
		return false
	}
	overshoot := 0
	if v.selLastPos.Y < v.viewportRect.Min.Y {
		overshoot = v.selLastPos.Y - v.viewportRect.Min.Y
	} else if v.selLastPos.Y > v.viewportRect.Max.Y {
		overshoot = v.selLastPos.Y - v.viewportRect.Max.Y
	}
	if overshoot == 0 {
		return false
	}
	// Speed scales with how far past the edge the pointer sits.
	step := float64(overshoot) * 0.3
	if step > 0 {
		step = math.Min(math.Max(step, 2), 64)
	} else {
		step = math.Max(math.Min(step, -2), -64)
	}
	if !v.scrollBy(0, step) {
		return false
	}
	v.syncVisualScroll()
	if sel, ok := v.textPosAt(v.selLastPos); ok {
		v.selEnd = sel
	}
	return true
}

// linkAt returns the link annotation under the given screen position.
func (v *pdfDocView) linkAt(pos image.Point) (viewerPDFPageLink, bool) {
	if v == nil || len(v.layoutTops) == 0 || v.layoutScale <= 0 {
		return viewerPDFPageLink{}, false
	}
	origin := v.docOrigin()
	docX := float64(pos.X - origin.X)
	docY := float64(pos.Y - origin.Y)
	page := v.pageAt(docY)
	links := v.links[page]
	if len(links) == 0 {
		return viewerPDFPageLink{}, false
	}
	px, py, _, _ := v.pageDocRect(page)
	ptX := (docX - px) / v.layoutScale
	ptY := (docY - py) / v.layoutScale
	for _, link := range links {
		if ptX >= link.Left && ptX <= link.Right && ptY >= link.Top && ptY <= link.Bottom {
			return link, true
		}
	}
	return viewerPDFPageLink{}, false
}

// armLink records a primary press over a link; the link fires when the same
// pointer releases within the click slop.
func (v *pdfDocView) armLink(pos image.Point, id pointer.ID) bool {
	link, ok := v.linkAt(pos)
	if !ok {
		return false
	}
	v.linkArmed = true
	v.linkID = id
	v.linkPressPos = pos
	v.linkDest = link.DestPage
	return true
}

// disarmLinkOnDrag cancels a pending link click once the pointer travels
// past the slop, so a text-selection or pan drag never navigates.
func (v *pdfDocView) disarmLinkOnDrag(pos image.Point) {
	if v == nil || !v.linkArmed {
		return
	}
	if absInt(pos.X-v.linkPressPos.X) > pdfDocLinkClickSlopPx || absInt(pos.Y-v.linkPressPos.Y) > pdfDocLinkClickSlopPx {
		v.linkArmed = false
	}
}

// releaseLink consumes the armed link on release and reports the page to
// navigate to when the gesture stayed a click.
func (v *pdfDocView) releaseLink(pos image.Point, id pointer.ID) (int, bool) {
	if v == nil || !v.linkArmed || id != v.linkID {
		return 0, false
	}
	v.linkArmed = false
	if absInt(pos.X-v.linkPressPos.X) > pdfDocLinkClickSlopPx || absInt(pos.Y-v.linkPressPos.Y) > pdfDocLinkClickSlopPx {
		return 0, false
	}
	return v.linkDest, true
}

// textPosAt maps a screen position to the nearest caret position. The
// second return reports whether text for the hit page is loaded.
func (v *pdfDocView) textPosAt(pos image.Point) (pdfDocTextPos, bool) {
	if v == nil || len(v.layoutTops) == 0 || v.layoutScale <= 0 {
		return pdfDocTextPos{}, false
	}
	origin := v.docOrigin()
	docX := float64(pos.X - origin.X)
	docY := float64(pos.Y - origin.Y)
	page := v.pageAt(docY)
	text, ok := v.text[page]
	if !ok {
		return pdfDocTextPos{Page: page}, false
	}
	px, py, _, _ := v.pageDocRect(page)
	// Position inside the page in PDF points.
	ptX := (docX - px) / v.layoutScale
	ptY := (docY - py) / v.layoutScale
	bestIdx := -1
	bestScore := math.MaxFloat64
	bestAfter := false
	for i, ch := range text.Chars {
		w := ch.Right - ch.Left
		h := ch.Bottom - ch.Top
		if w <= 0 || h <= 0 {
			continue
		}
		dy := 0.0
		if ptY < ch.Top {
			dy = ch.Top - ptY
		} else if ptY > ch.Bottom {
			dy = ptY - ch.Bottom
		}
		dx := 0.0
		if ptX < ch.Left {
			dx = ch.Left - ptX
		} else if ptX > ch.Right {
			dx = ptX - ch.Right
		}
		score := dy*40 + dx
		if score < bestScore {
			bestScore = score
			bestIdx = i
			bestAfter = ptX > (ch.Left+ch.Right)/2
		}
	}
	if bestIdx < 0 {
		return pdfDocTextPos{Page: page}, false
	}
	idx := bestIdx
	if bestAfter {
		idx++
	}
	return pdfDocTextPos{Page: page, Index: idx}, true
}

// selectionRectsOnPage merges consecutive selected char boxes into line-run
// rectangles in page points.
func (v *pdfDocView) selectionRectsOnPage(page int) [][4]float64 {
	from, to, ok := v.selectionOnPage(page)
	if !ok {
		return nil
	}
	text := v.text[page]
	var rects [][4]float64
	var cur [4]float64
	curValid := false
	flush := func() {
		if curValid {
			rects = append(rects, cur)
			curValid = false
		}
	}
	for _, ch := range text.Chars[from:to] {
		w := ch.Right - ch.Left
		h := ch.Bottom - ch.Top
		if w <= 0 || h <= 0 {
			continue
		}
		if curValid {
			overlap := math.Min(cur[3], ch.Bottom) - math.Max(cur[1], ch.Top)
			minH := math.Min(cur[3]-cur[1], h)
			sameLine := overlap > minH*0.5
			contiguous := ch.Left >= cur[0]-1 && ch.Left <= cur[2]+math.Max(4, h*2)
			if sameLine && contiguous {
				cur[0] = math.Min(cur[0], ch.Left)
				cur[1] = math.Min(cur[1], ch.Top)
				cur[2] = math.Max(cur[2], ch.Right)
				cur[3] = math.Max(cur[3], ch.Bottom)
				continue
			}
			flush()
		}
		cur = [4]float64{ch.Left, ch.Top, ch.Right, ch.Bottom}
		curValid = true
	}
	flush()
	return rects
}

// computeLayout sizes the viewport, reserving room for scrollbars, and
// refreshes the layout cache and scrollbar geometry.
func (v *pdfDocView) computeLayout(size image.Point, scrollbarPx int) {
	if v == nil {
		return
	}
	v.vTrackRect = image.Rectangle{}
	v.vThumbRect = image.Rectangle{}
	v.hTrackRect = image.Rectangle{}
	v.hThumbRect = image.Rectangle{}
	if size.X <= 0 || size.Y <= 0 {
		v.surfaceRect = image.Rectangle{}
		v.viewportRect = image.Rectangle{}
		return
	}
	surface := image.Rect(0, 0, size.X, size.Y)
	v.surfaceRect = surface
	view := surface
	needV, needH := false, false
	for i := 0; i < 3; i++ {
		view = surface
		if needV && scrollbarPx > 0 {
			view.Max.X -= scrollbarPx
		}
		if needH && scrollbarPx > 0 {
			view.Max.Y -= scrollbarPx
		}
		if view.Dx() < 1 {
			view.Max.X = view.Min.X + 1
		}
		if view.Dy() < 1 {
			view.Max.Y = view.Min.Y + 1
		}
		v.viewportRect = view
		v.layoutScale = 0 // fit scale depends on the viewport
		v.relayout()
		nextV := v.contentH > float64(view.Dy())+0.5
		nextH := v.contentW > float64(view.Dx())+0.5
		if nextV == needV && nextH == needH {
			break
		}
		needV, needH = nextV, nextH
	}
	v.clampScroll()
	v.computeScrollbars(scrollbarPx, needV, needH)
}

func (v *pdfDocView) computeScrollbars(scrollbarPx int, needV, needH bool) {
	if v == nil {
		return
	}
	displayX, displayY := v.displayScroll()
	if needV && scrollbarPx > 0 {
		track := image.Rect(v.viewportRect.Max.X, v.surfaceRect.Min.Y, v.surfaceRect.Max.X, v.viewportRect.Max.Y)
		if track.Dx() > 0 && track.Dy() > 0 {
			v.vTrackRect = track
			v.vThumbRect = viewerScrollbarThumbForScroll(track, v.viewportRect.Dy(), int(math.Ceil(v.contentH)), float64(displayY), true)
		}
	}
	if needH && scrollbarPx > 0 {
		track := image.Rect(v.surfaceRect.Min.X, v.viewportRect.Max.Y, v.viewportRect.Max.X, v.surfaceRect.Max.Y)
		if track.Dx() > 0 && track.Dy() > 0 {
			v.hTrackRect = track
			v.hThumbRect = viewerScrollbarThumbForScroll(track, v.viewportRect.Dx(), int(math.Ceil(v.contentW)), float64(displayX), false)
		}
	}
}

func (v *pdfDocView) verticalThumbGrabY(pos image.Point) int {
	if v == nil || v.vThumbRect.Dy() <= 0 {
		return 0
	}
	grab := pos.Y - v.vThumbRect.Min.Y
	if grab < 0 {
		grab = 0
	}
	if grab > v.vThumbRect.Dy() {
		grab = v.vThumbRect.Dy()
	}
	return grab
}

func (v *pdfDocView) horizontalThumbGrabX(pos image.Point) int {
	if v == nil || v.hThumbRect.Dx() <= 0 {
		return 0
	}
	grab := pos.X - v.hThumbRect.Min.X
	if grab < 0 {
		grab = 0
	}
	if grab > v.hThumbRect.Dx() {
		grab = v.hThumbRect.Dx()
	}
	return grab
}

func (v *pdfDocView) setScrollFromVerticalDrag(posY, grab int) bool {
	if v == nil || v.vTrackRect.Dy() <= 0 || v.vThumbRect.Dy() <= 0 {
		return false
	}
	_, maxScroll := v.maxScroll()
	if maxScroll <= 0 {
		return false
	}
	maxTravel := v.vTrackRect.Dy() - v.vThumbRect.Dy()
	if maxTravel <= 0 {
		return false
	}
	travel := posY - v.vTrackRect.Min.Y - grab
	if travel < 0 {
		travel = 0
	}
	if travel > maxTravel {
		travel = maxTravel
	}
	next := float64(travel) / float64(maxTravel) * maxScroll
	if next == v.scrollY {
		return false
	}
	v.scrollY = next
	return true
}

func (v *pdfDocView) setScrollFromHorizontalDrag(posX, grab int) bool {
	if v == nil || v.hTrackRect.Dx() <= 0 || v.hThumbRect.Dx() <= 0 {
		return false
	}
	maxScroll, _ := v.maxScroll()
	if maxScroll <= 0 {
		return false
	}
	maxTravel := v.hTrackRect.Dx() - v.hThumbRect.Dx()
	if maxTravel <= 0 {
		return false
	}
	travel := posX - v.hTrackRect.Min.X - grab
	if travel < 0 {
		travel = 0
	}
	if travel > maxTravel {
		travel = maxTravel
	}
	next := float64(travel) / float64(maxTravel) * maxScroll
	if next == v.scrollX {
		return false
	}
	v.scrollX = next
	return true
}

func (v *pdfDocView) updateHover(pos image.Point) bool {
	if v == nil {
		return false
	}
	oldVTrack, oldVThumb := v.hoverVTrack, v.hoverVThumb
	oldHTrack, oldHThumb := v.hoverHTrack, v.hoverHThumb
	oldText, oldLink := v.hoverText, v.hoverLink
	v.hoverVTrack = viewerPointInRect(pos, v.vTrackRect)
	v.hoverVThumb = viewerPointInRect(pos, v.vThumbRect)
	v.hoverHTrack = viewerPointInRect(pos, v.hTrackRect)
	v.hoverHThumb = viewerPointInRect(pos, v.hThumbRect)
	v.hoverText = false
	v.hoverLink = false
	if !v.hoverVTrack && !v.hoverHTrack && viewerPointInRect(pos, v.viewportRect) {
		_, v.hoverLink = v.linkAt(pos)
		if !v.hoverLink {
			_, v.hoverText = v.textHitAt(pos)
		}
	}
	return oldVTrack != v.hoverVTrack ||
		oldVThumb != v.hoverVThumb ||
		oldHTrack != v.hoverHTrack ||
		oldHThumb != v.hoverHThumb ||
		oldText != v.hoverText ||
		oldLink != v.hoverLink
}

func (v *pdfDocView) clearHover() bool {
	if v == nil {
		return false
	}
	changed := v.hoverVTrack || v.hoverVThumb || v.hoverHTrack || v.hoverHThumb || v.hoverText || v.hoverLink
	v.hoverVTrack = false
	v.hoverVThumb = false
	v.hoverHTrack = false
	v.hoverHThumb = false
	v.hoverText = false
	v.hoverLink = false
	return changed
}

func sendPDFDocResult(ch chan pdfDocResult, res pdfDocResult) {
	if ch == nil {
		return
	}
	select {
	case ch <- res:
	default:
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- res:
		default:
		}
	}
}

// pdfDocRenderSource returns the document source for background pdfium
// calls.
func pdfDocRenderSource(st *fileViewerState) (string, []byte) {
	if st == nil {
		return "", nil
	}
	localPath := ""
	if viewerPDFPreviewUsesLocalPath && st.remote == nil && !filesys.ArchiveMemberPath(st.path) {
		localPath = st.path
	}
	return localPath, st.imagePreviewData
}

func (v *pdfDocView) renderInflightCount(now time.Time) int {
	count := 0
	for page, inflight := range v.renderPending {
		if now.Sub(inflight.startedAt) > pdfDocInflightExpiry {
			delete(v.renderPending, page)
			continue
		}
		count++
	}
	return count
}

// ensurePDFDocAssets requests missing document info, page renders, and page
// text around the target scroll position.
func (ui *UI) ensurePDFDocAssets(gtx layout.Context, st *fileViewerState) {
	v := &st.pdfDoc
	if st.pdfDocCh == nil || len(v.pageSizes) == 0 {
		return
	}
	localPath, data := pdfDocRenderSource(st)
	seq := st.seq
	ch := st.pdfDocCh
	// Keep one stable backend for every job scheduled by this pass. Besides
	// avoiding repeated global lookups in the workers, this ensures background
	// jobs cannot observe a backend replacement after this function returns.
	backend := viewerPDFPreviewBackend

	if !v.infoLoaded && !v.infoPending {
		v.infoPending = true
		go func() {
			info, err := backend.DocInfo(viewerPDFRenderRequest{
				Data:      data,
				LocalPath: localPath,
			})
			res := pdfDocResult{seq: seq}
			if err != nil {
				res.err = err.Error()
			} else {
				res.info = &info
			}
			sendPDFDocResult(ch, res)
		}()
	}
	if v.infoLoaded && !v.tocLoaded && !v.tocPending {
		v.tocPending = true
		go func() {
			toc, err := backend.TOC(viewerPDFRenderRequest{
				Data:      data,
				LocalPath: localPath,
			})
			if err != nil {
				toc = nil
			}
			res := pdfDocResult{seq: seq, toc: &toc}
			sendPDFDocResult(ch, res)
		}()
	}

	viewH := float64(v.viewportRect.Dy())
	first, last := v.pageRange(v.scrollY, v.scrollY+viewH)
	if last < first {
		return
	}
	inflight := v.renderInflightCount(gtx.Now)
	// Visible pages first, then one page of lookahead on both sides.
	order := make([]int, 0, last-first+3)
	for p := first; p <= last; p++ {
		order = append(order, p)
	}
	order = append(order, last+1, first-1)
	for _, page := range order {
		if page < 0 || page >= v.pageCount() {
			continue
		}
		if inflight >= pdfDocMaxInflight {
			break
		}
		target := v.renderWidthFor(page)
		if entry, ok := v.pages[page]; ok && entry.width == target {
			continue
		}
		if pending, ok := v.renderPending[page]; ok && pending.width == target {
			continue
		}
		if v.renderPending == nil {
			v.renderPending = make(map[int]pdfDocInflight, 4)
		}
		v.renderPending[page] = pdfDocInflight{width: target, startedAt: gtx.Now}
		inflight++
		go func(page, width int) {
			rendered, err := backend.RenderPage(viewerPDFRenderRequest{
				Data:      data,
				LocalPath: localPath,
				Page:      page,
				Width:     width,
			})
			res := pdfDocResult{seq: seq, page: page}
			if err != nil {
				res.err = err.Error()
			} else {
				res.render = &pdfDocPageRender{img: rendered.Image, width: width}
			}
			sendPDFDocResult(ch, res)
		}(page, target)
	}

	for page := first; page <= last; page++ {
		if _, ok := v.text[page]; ok {
			continue
		}
		if at, ok := v.textPending[page]; ok && gtx.Now.Sub(at) <= pdfDocInflightExpiry {
			continue
		}
		if v.textPending == nil {
			v.textPending = make(map[int]time.Time, 4)
		}
		v.textPending[page] = gtx.Now
		go func(page int) {
			text, err := backend.PageText(viewerPDFRenderRequest{
				Data:      data,
				LocalPath: localPath,
				Page:      page,
			})
			res := pdfDocResult{seq: seq, page: page}
			if err != nil {
				res.err = err.Error()
			} else {
				res.text = &text
			}
			sendPDFDocResult(ch, res)
		}(page)
	}

	for page := first; page <= last; page++ {
		if _, ok := v.links[page]; ok {
			continue
		}
		if at, ok := v.linkPending[page]; ok && gtx.Now.Sub(at) <= pdfDocInflightExpiry {
			continue
		}
		if v.linkPending == nil {
			v.linkPending = make(map[int]time.Time, 4)
		}
		v.linkPending[page] = gtx.Now
		go func(page int) {
			links, err := backend.PageLinks(viewerPDFRenderRequest{
				Data:      data,
				LocalPath: localPath,
				Page:      page,
			})
			res := pdfDocResult{seq: seq, page: page}
			if err != nil {
				res.err = err.Error()
			} else {
				res.links = &links
			}
			sendPDFDocResult(ch, res)
		}(page)
	}

	v.prune(first, last)
	if len(v.renderPending) > 0 || len(v.textPending) > 0 || len(v.linkPending) > 0 || v.infoPending || v.tocPending {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(pdfDocPendingPollDelay)})
	}
}

// pumpPDFDocResults drains async doc info / render / text results.
func (ui *UI) pumpPDFDocResults(gtx layout.Context, st *fileViewerState) {
	if st == nil || st.pdfDocCh == nil {
		return
	}
	v := &st.pdfDoc
	for {
		select {
		case res := <-st.pdfDocCh:
			if res.seq != st.seq {
				continue
			}
			switch {
			case res.info != nil:
				v.configure(*res.info)
				gtx.Execute(op.InvalidateCmd{})
			case res.render != nil:
				delete(v.renderPending, res.page)
				v.storeRender(res.page, *res.render)
				gtx.Execute(op.InvalidateCmd{})
			case res.text != nil:
				delete(v.textPending, res.page)
				v.storeText(*res.text)
				gtx.Execute(op.InvalidateCmd{})
			case res.links != nil:
				delete(v.linkPending, res.page)
				v.storeLinks(*res.links)
				gtx.Execute(op.InvalidateCmd{})
			case res.toc != nil:
				v.toc = normalizeViewerPDFTOC(*res.toc)
				v.tocLoaded = true
				v.tocPending = false
				st.tocExpanded = nil
				gtx.Execute(op.InvalidateCmd{})
			default:
				if res.err != "" {
					v.infoPending = false
					delete(v.renderPending, res.page)
					delete(v.textPending, res.page)
					delete(v.linkPending, res.page)
				}
			}
		default:
			return
		}
	}
}

// seedPDFDocPreviewRender installs the single-page preview bitmap as the
// render for the page it depicts, so the first paint is not blank while the
// real renders load. It must key off imagePreviewSeedPage: imagePreviewPage
// tracks the current page while scrolling, and seeding by it would flash
// this bitmap onto whatever unrendered page a fast scroll lands on.
func seedPDFDocPreviewRender(st *fileViewerState) {
	if st == nil || st.imagePreview == nil {
		return
	}
	v := &st.pdfDoc
	if _, ok := v.pages[st.imagePreviewSeedPage]; ok {
		return
	}
	width := 0
	if st.imagePreviewSize.X > 0 {
		width = st.imagePreviewSize.X
	}
	v.storeRender(st.imagePreviewSeedPage, pdfDocPageRender{img: st.imagePreview, width: width})
}

// layoutPDFDocOutputView renders the continuous PDF document view.
func (ui *UI) layoutPDFDocOutputView(_ *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	size := gtx.Constraints.Max
	if st == nil || size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: size}
	}
	ui.paintFileViewerImageBackdrop(gtx, size)
	v := &st.pdfDoc
	v.seedFallbackSizes(st.imagePreviewPageCount, st.imagePreviewSize)
	if len(v.pageSizes) == 0 {
		return layout.Dimensions{Size: size}
	}
	seedPDFDocPreviewRender(st)

	scrollbarPx := viewerScrollbarThickness(gtx, min(size.X, size.Y))
	v.computeLayout(size, scrollbarPx)
	ui.handlePDFDocEvents(gtx, st)
	if v.autoScrollSelection() {
		st.markUserBrowsing(gtx.Now)
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(streamSmoothTick)})
	}
	animating := v.prepareVisualScroll(gtx.Now, viewerSmoothScrolling(ui.fmCfg))
	v.computeLayout(size, scrollbarPx)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(streamSmoothTick)})
	}
	ui.ensurePDFDocAssets(gtx, st)
	st.imagePreviewPage = v.currentPage()
	ui.paintPDFDocPages(gtx, st)
	ui.paintPDFDocScrollbars(gtx, st)
	ui.applyPDFDocCursor(gtx, st)
	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &v.pointerTag)
	pass.Pop()
	return layout.Dimensions{Size: size}
}

func (ui *UI) handlePDFDocEvents(gtx layout.Context, st *fileViewerState) {
	v := &st.pdfDoc
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &v.pointerTag,
			Kinds:  pointer.Scroll | pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Move | pointer.Enter | pointer.Leave,
			ScrollX: pointer.ScrollRange{
				Min: -120,
				Max: 120,
			},
			ScrollY: pointer.ScrollRange{
				Min: -120,
				Max: 120,
			},
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		if ui.terminalFocused(gtx) && terminalSurfaceFocusPointerEvent(pe) {
			ui.releaseTerminalKeyboardFocus(gtx)
		}
		pos := pe.Position.Round()
		switch pe.Kind {
		case pointer.Scroll:
			changed := false
			if pe.Scroll.X != 0 {
				changed = v.scrollWheelX(pe.Scroll.X) || changed
			}
			if pe.Scroll.Y != 0 {
				changed = v.scrollWheelY(pe.Scroll.Y) || changed
			}
			if changed {
				st.markUserBrowsing(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Press:
			if pe.Buttons.Contain(pointer.ButtonSecondary) {
				st.openContextMenu(pos, gtx.Now)
				continue
			}
			if !pe.Buttons.Contain(pointer.ButtonPrimary) {
				if v.updateHover(pos) {
					gtx.Execute(op.InvalidateCmd{})
				}
				continue
			}
			if st.menuOpen {
				st.closeContextMenu()
			}
			switch {
			case viewerPointInRect(pos, v.vTrackRect):
				v.vDragging = true
				v.vDragID = pe.PointerID
				v.vDragGrab = v.verticalThumbGrabY(pos)
				gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				if v.setScrollFromVerticalDrag(pos.Y, v.vDragGrab) {
					st.markUserBrowsing(gtx.Now)
				}
			case viewerPointInRect(pos, v.hTrackRect):
				v.hDragging = true
				v.hDragID = pe.PointerID
				v.hDragGrab = v.horizontalThumbGrabX(pos)
				gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				if v.setScrollFromHorizontalDrag(pos.X, v.hDragGrab) {
					st.markUserBrowsing(gtx.Now)
				}
			case viewerPointInRect(pos, v.viewportRect):
				doubleClick := !v.lastClickAt.IsZero() &&
					gtx.Now.Sub(v.lastClickAt) <= 400*time.Millisecond &&
					absInt(pos.X-v.lastClickPos.X) <= 6 &&
					absInt(pos.Y-v.lastClickPos.Y) <= 6
				v.lastClickAt = gtx.Now
				v.lastClickPos = pos
				if doubleClick && v.selectWordAt(pos) {
					st.markUserBrowsing(gtx.Now)
					gtx.Execute(op.InvalidateCmd{})
					break
				}
				// A press on a link arms it; it only navigates if the
				// gesture stays a click (Shift keeps selection intent).
				if !pe.Modifiers.Contain(key.ModShift) {
					v.armLink(pos, pe.PointerID)
				}
				// Drag selects when the press lands on text and pans
				// otherwise; Shift forces selection anchored to the
				// nearest text.
				anchor, overText := v.textHitAt(pos)
				if !overText && pe.Modifiers.Contain(key.ModShift) {
					anchor, overText = v.textPosAt(pos)
				}
				if overText {
					v.selecting = true
					v.selActive = true
					v.selID = pe.PointerID
					v.selStart = anchor
					v.selEnd = anchor
					v.selLastPos = pos
					gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
					st.markUserBrowsing(gtx.Now)
				} else {
					if v.clearSelection() {
						gtx.Execute(op.InvalidateCmd{})
					}
					v.panning = true
					v.panID = pe.PointerID
					v.panLast = pe.Position
					gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
					st.markUserBrowsing(gtx.Now)
				}
			}
			if v.updateHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Drag:
			v.disarmLinkOnDrag(pos)
			changed := false
			if v.panning && pe.PointerID == v.panID {
				dx := float64(v.panLast.X - pe.Position.X)
				dy := float64(v.panLast.Y - pe.Position.Y)
				v.panLast = pe.Position
				if v.scrollBy(dx, dy) {
					v.syncVisualScroll()
					changed = true
				}
			}
			if v.selecting && pe.PointerID == v.selID {
				v.selLastPos = pos
				if sel, ok := v.textPosAt(pos); ok && sel != v.selEnd {
					v.selEnd = sel
					changed = true
				}
			}
			if v.vDragging && pe.PointerID == v.vDragID {
				changed = v.setScrollFromVerticalDrag(pos.Y, v.vDragGrab) || changed
			}
			if v.hDragging && pe.PointerID == v.hDragID {
				changed = v.setScrollFromHorizontalDrag(pos.X, v.hDragGrab) || changed
			}
			if changed {
				st.markUserBrowsing(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
			}
			if v.updateHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Release, pointer.Cancel:
			if v.panning && pe.PointerID == v.panID {
				v.panning = false
			}
			if v.selecting && pe.PointerID == v.selID {
				v.selecting = false
				if !v.hasSelection() {
					v.clearSelection()
				}
			}
			if pe.Kind == pointer.Cancel {
				v.linkArmed = false
			} else if dest, ok := v.releaseLink(pos, pe.PointerID); ok && !v.hasSelection() {
				if v.scrollToPage(dest) {
					v.syncVisualScroll()
				}
				st.markUserBrowsing(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
			}
			if v.vDragging && pe.PointerID == v.vDragID {
				v.vDragging = false
				v.vDragGrab = 0
			}
			if v.hDragging && pe.PointerID == v.hDragID {
				v.hDragging = false
				v.hDragGrab = 0
			}
			if pe.Kind == pointer.Cancel {
				if v.clearHover() {
					gtx.Execute(op.InvalidateCmd{})
				}
			} else if v.updateHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Move, pointer.Enter:
			if v.updateHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Leave:
			if v.clearHover() {
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
}

func (ui *UI) paintPDFDocPages(gtx layout.Context, st *fileViewerState) {
	v := &st.pdfDoc
	if v.viewportRect.Dx() <= 0 || v.viewportRect.Dy() <= 0 || len(v.layoutTops) == 0 {
		return
	}
	theme := ui.fileViewerTheme()
	defer clip.Rect(v.viewportRect).Push(gtx.Ops).Pop()
	origin := v.docOrigin()
	displayX, displayY := v.displayScroll()
	_ = displayX
	first, last := v.pageRange(float64(displayY), float64(displayY+v.viewportRect.Dy()))
	for page := first; page <= last; page++ {
		px, py, pw, ph := v.pageDocRect(page)
		rect := image.Rect(
			origin.X+int(math.Round(px)),
			origin.Y+int(math.Round(py)),
			origin.X+int(math.Round(px+pw)),
			origin.Y+int(math.Round(py+ph)),
		)
		if rect.Dx() <= 0 || rect.Dy() <= 0 {
			continue
		}
		// Paper backdrop keeps not-yet-rendered pages visible.
		paint.FillShape(gtx.Ops, pdfDocPaperColor, clip.Rect(rect).Op())
		if entry, ok := v.pages[page]; ok && entry.img != nil {
			func() {
				defer clip.Rect(rect).Push(gtx.Ops).Pop()
				offset := op.Offset(rect.Min).Push(gtx.Ops)
				bounds := entry.img.Bounds()
				if bounds.Dx() > 0 {
					scale := float32(rect.Dx()) / float32(bounds.Dx())
					if scale != 1 {
						op.Affine(f32.AffineId().Scale(f32.Point{}, f32.Pt(scale, scale))).Add(gtx.Ops)
					}
				}
				paint.NewImageOp(entry.img).Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				offset.Pop()
			}()
		}
		if v.hasSelection() {
			for _, r := range v.selectionRectsOnPage(page) {
				selRect := image.Rect(
					origin.X+int(math.Round(px+r[0]*v.layoutScale)),
					origin.Y+int(math.Round(py+r[1]*v.layoutScale)),
					origin.X+int(math.Round(px+r[2]*v.layoutScale)),
					origin.Y+int(math.Round(py+r[3]*v.layoutScale)),
				)
				if selRect.Dx() <= 0 || selRect.Dy() <= 0 {
					continue
				}
				paint.FillShape(gtx.Ops, theme.Selection, clip.Rect(selRect).Op())
			}
		}
	}
}

func (ui *UI) paintPDFDocScrollbars(gtx layout.Context, st *fileViewerState) {
	v := &st.pdfDoc
	theme := ui.fileViewerTheme()
	paintViewerScrollbar(gtx, theme, v.vTrackRect, v.vThumbRect, v.hoverVTrack, v.hoverVThumb, v.vDragging)
	paintViewerScrollbar(gtx, theme, v.hTrackRect, v.hThumbRect, v.hoverHTrack, v.hoverHThumb, v.hDragging)
}

func (ui *UI) applyPDFDocCursor(gtx layout.Context, st *fileViewerState) {
	v := &st.pdfDoc
	if v.panning || v.vDragging || v.hDragging {
		pointer.CursorGrabbing.Add(gtx.Ops)
		return
	}
	if v.selecting {
		pointer.CursorText.Add(gtx.Ops)
		return
	}
	switch {
	case v.hoverVTrack || v.hoverVThumb:
		defer clip.Rect(v.vTrackRect).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	case v.hoverHTrack || v.hoverHThumb:
		defer clip.Rect(v.hTrackRect).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	case v.hoverLink:
		defer clip.Rect(v.viewportRect).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	case v.hoverText:
		defer clip.Rect(v.viewportRect).Push(gtx.Ops).Pop()
		pointer.CursorText.Add(gtx.Ops)
	}
}
