// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

const (
	fileViewerImageScrollbarDp     = 10
	fileViewerImageScrollbarMinPx  = 8
	fileViewerPDFDocScrollbarDp    = 8
	fileViewerPDFDocScrollbarMinPx = 6
	fileViewerPDFDocScrollbarGapDp = 4
	fileViewerImageKeyStepPx       = 48
	fileViewerImageWheelStepPx     = 28
	fileViewerImageMinZoom         = float32(0.25)
	fileViewerImageMaxZoom         = float32(8)
	fileViewerImageZoomFactor      = float32(1.25)
)

type imagePreviewView struct {
	pointerTag struct{}

	surfaceRect  image.Rectangle
	viewportRect image.Rectangle
	vTrackRect   image.Rectangle
	vThumbRect   image.Rectangle
	hTrackRect   image.Rectangle
	hThumbRect   image.Rectangle
	pdfTrackRect image.Rectangle
	pdfThumbRect image.Rectangle

	zoom        float32
	scrollX     int
	scrollY     int
	visualX     float32
	visualY     float32
	visualReady bool
	visualAt    time.Time
	wheelCarryX float32
	wheelCarryY float32

	hoverVTrack   bool
	hoverVThumb   bool
	hoverHTrack   bool
	hoverHThumb   bool
	hoverPDFTrack bool
	hoverPDFThumb bool

	vDragging bool
	vDragID   pointer.ID
	vDragGrab int

	hDragging bool
	hDragID   pointer.ID
	hDragGrab int

	pdfDragging bool
	pdfDragID   pointer.ID
	pdfDragGrab int
	pdfDragPage int
}

func (v *imagePreviewView) effectiveZoom() float32 {
	if v == nil || v.zoom <= 0 {
		return 1
	}
	return v.zoom
}

func (v *imagePreviewView) reset() {
	if v == nil {
		return
	}
	v.surfaceRect = image.Rectangle{}
	v.viewportRect = image.Rectangle{}
	v.vTrackRect = image.Rectangle{}
	v.vThumbRect = image.Rectangle{}
	v.hTrackRect = image.Rectangle{}
	v.hThumbRect = image.Rectangle{}
	v.pdfTrackRect = image.Rectangle{}
	v.pdfThumbRect = image.Rectangle{}
	v.zoom = 1
	v.scrollX = 0
	v.scrollY = 0
	v.visualX = 0
	v.visualY = 0
	v.visualReady = false
	v.visualAt = time.Time{}
	v.wheelCarryX = 0
	v.wheelCarryY = 0
	v.hoverVTrack = false
	v.hoverVThumb = false
	v.hoverHTrack = false
	v.hoverHThumb = false
	v.hoverPDFTrack = false
	v.hoverPDFThumb = false
	v.vDragging = false
	v.vDragID = 0
	v.vDragGrab = 0
	v.hDragging = false
	v.hDragID = 0
	v.hDragGrab = 0
	v.pdfDragging = false
	v.pdfDragID = 0
	v.pdfDragGrab = 0
	v.pdfDragPage = 0
}

func (v *imagePreviewView) contentSize(img image.Image) image.Point {
	if img == nil {
		return image.Point{}
	}
	bounds := img.Bounds()
	zoom := v.effectiveZoom()
	w := int(math.Round(float64(float32(bounds.Dx()) * zoom)))
	h := int(math.Round(float64(float32(bounds.Dy()) * zoom)))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return image.Pt(w, h)
}

func (v *imagePreviewView) maxScroll(img image.Image) (int, int) {
	if v == nil || img == nil || v.viewportRect.Dx() <= 0 || v.viewportRect.Dy() <= 0 {
		return 0, 0
	}
	content := v.contentSize(img)
	return max(0, content.X-v.viewportRect.Dx()), max(0, content.Y-v.viewportRect.Dy())
}

func (v *imagePreviewView) clampScroll(img image.Image) bool {
	if v == nil {
		return false
	}
	maxX, maxY := v.maxScroll(img)
	nextX := v.scrollX
	nextY := v.scrollY
	if nextX < 0 {
		nextX = 0
	}
	if nextX > maxX {
		nextX = maxX
	}
	if nextY < 0 {
		nextY = 0
	}
	if nextY > maxY {
		nextY = maxY
	}
	changed := nextX != v.scrollX || nextY != v.scrollY
	v.scrollX = nextX
	v.scrollY = nextY
	return changed
}

func (v *imagePreviewView) syncVisualScroll() {
	if v == nil {
		return
	}
	v.visualX = float32(v.scrollX)
	v.visualY = float32(v.scrollY)
	v.visualReady = true
	v.visualAt = time.Time{}
}

func (v *imagePreviewView) smoothJumpLimit() float32 {
	if v == nil {
		return float32(fileViewerImageKeyStepPx * 6)
	}
	limit := float32(fileViewerImageKeyStepPx * 6)
	if view := float32(v.viewportRect.Dx()) * 0.75; view > limit {
		limit = view
	}
	if view := float32(v.viewportRect.Dy()) * 0.75; view > limit {
		limit = view
	}
	return limit
}

func (v *imagePreviewView) prepareVisualScroll(now time.Time, smooth bool, img image.Image) bool {
	if v == nil {
		return false
	}
	targetX := float32(v.scrollX)
	targetY := float32(v.scrollY)
	if !v.visualReady {
		v.visualX = targetX
		v.visualY = targetY
		v.visualReady = true
		v.visualAt = now
		return false
	}
	if !smooth || v.vDragging || v.hDragging {
		v.visualX = targetX
		v.visualY = targetY
		v.visualAt = now
		return false
	}
	if float32Abs(targetX-v.visualX) > v.smoothJumpLimit() || float32Abs(targetY-v.visualY) > v.smoothJumpLimit() {
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
		blend := float32(1 - math.Exp(-float64(dt)/float64(streamSmoothTau)))
		blend = clamp01(blend)
		v.visualX += (targetX - v.visualX) * blend
		v.visualY += (targetY - v.visualY) * blend
	}
	v.visualAt = now
	maxX, maxY := v.maxScroll(img)
	if v.visualX < 0 {
		v.visualX = 0
	}
	if v.visualY < 0 {
		v.visualY = 0
	}
	if v.visualX > float32(maxX) {
		v.visualX = float32(maxX)
	}
	if v.visualY > float32(maxY) {
		v.visualY = float32(maxY)
	}
	if float32Abs(targetX-v.visualX) < streamSmoothSnapEpsilon && float32Abs(targetY-v.visualY) < streamSmoothSnapEpsilon {
		v.visualX = targetX
		v.visualY = targetY
		return false
	}
	return true
}

func (v *imagePreviewView) displayScroll(img image.Image) (int, int) {
	if v == nil {
		return 0, 0
	}
	x := v.scrollX
	y := v.scrollY
	if v.visualReady {
		x = int(math.Round(float64(v.visualX)))
		y = int(math.Round(float64(v.visualY)))
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if img != nil {
		maxX, maxY := v.maxScroll(img)
		if x > maxX {
			x = maxX
		}
		if y > maxY {
			y = maxY
		}
	}
	return x, y
}

func (v *imagePreviewView) scrollByPixels(img image.Image, dx, dy int) bool {
	if v == nil || img == nil || (dx == 0 && dy == 0) {
		return false
	}
	beforeX, beforeY := v.scrollX, v.scrollY
	v.scrollX += dx
	v.scrollY += dy
	v.clampScroll(img)
	return v.scrollX != beforeX || v.scrollY != beforeY
}

func (v *imagePreviewView) scrollByKeyStep(img image.Image, xSteps, ySteps int) bool {
	return v.scrollByPixels(img, xSteps*fileViewerImageKeyStepPx, ySteps*fileViewerImageKeyStepPx)
}

func (v *imagePreviewView) scrollByPage(img image.Image, pages int) bool {
	if v == nil || img == nil || pages == 0 {
		return false
	}
	step := v.viewportRect.Dy() - fileViewerImageKeyStepPx
	if step < fileViewerImageKeyStepPx {
		step = fileViewerImageKeyStepPx
	}
	return v.scrollByPixels(img, 0, pages*step)
}

func (v *imagePreviewView) scrollToOrigin() bool {
	if v == nil {
		return false
	}
	if v.scrollX == 0 && v.scrollY == 0 {
		return false
	}
	v.scrollX = 0
	v.scrollY = 0
	return true
}

func (v *imagePreviewView) scrollToEnd(img image.Image) bool {
	if v == nil || img == nil {
		return false
	}
	maxX, maxY := v.maxScroll(img)
	if v.scrollX == maxX && v.scrollY == maxY {
		return false
	}
	v.scrollX = maxX
	v.scrollY = maxY
	return true
}

func (v *imagePreviewView) zoomBy(img image.Image, factor float32) bool {
	if v == nil || img == nil || factor <= 0 {
		return false
	}
	oldZoom := v.effectiveZoom()
	newZoom := oldZoom * factor
	if newZoom < fileViewerImageMinZoom {
		newZoom = fileViewerImageMinZoom
	}
	if newZoom > fileViewerImageMaxZoom {
		newZoom = fileViewerImageMaxZoom
	}
	if math.Abs(float64(newZoom-oldZoom)) < 0.0001 {
		return false
	}
	anchorX := float32(v.scrollX) / oldZoom
	anchorY := float32(v.scrollY) / oldZoom
	v.zoom = newZoom
	v.scrollX = int(math.Round(float64(anchorX * newZoom)))
	v.scrollY = int(math.Round(float64(anchorY * newZoom)))
	v.clampScroll(img)
	return true
}

func (v *imagePreviewView) scrollWheelY(img image.Image, delta float32) bool {
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
	return v.scrollByPixels(img, 0, step)
}

func (v *imagePreviewView) scrollWheelX(img image.Image, delta float32) bool {
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
	return v.scrollByPixels(img, step, 0)
}

func (v *imagePreviewView) computeLayout(size image.Point, inset, scrollbarPx int, img image.Image) {
	if v == nil {
		return
	}
	v.surfaceRect = image.Rectangle{}
	v.viewportRect = image.Rectangle{}
	v.vTrackRect = image.Rectangle{}
	v.vThumbRect = image.Rectangle{}
	v.hTrackRect = image.Rectangle{}
	v.hThumbRect = image.Rectangle{}
	if size.X <= 0 || size.Y <= 0 {
		v.scrollX = 0
		v.scrollY = 0
		return
	}
	if inset < 0 {
		inset = 0
	}
	surface := image.Rect(inset, inset, size.X-inset, size.Y-inset)
	if surface.Dx() < 1 {
		surface.Max.X = surface.Min.X + 1
	}
	if surface.Dy() < 1 {
		surface.Max.Y = surface.Min.Y + 1
	}
	v.surfaceRect = surface
	content := v.contentSize(img)
	needV := content.Y > surface.Dy()
	needH := content.X > surface.Dx()
	view := surface
	for i := 0; i < 3; i++ {
		view = surface
		if needV {
			view.Max.X -= scrollbarPx
		}
		if needH {
			view.Max.Y -= scrollbarPx
		}
		if view.Dx() < 1 {
			view.Max.X = view.Min.X + 1
		}
		if view.Dy() < 1 {
			view.Max.Y = view.Min.Y + 1
		}
		nextV := content.Y > view.Dy()
		nextH := content.X > view.Dx()
		if nextV == needV && nextH == needH {
			break
		}
		needV, needH = nextV, nextH
	}
	v.viewportRect = view
	v.clampScroll(img)
	v.computeScrollbars(content, scrollbarPx, needV, needH)
}

func (v *imagePreviewView) computeScrollbars(content image.Point, scrollbarPx int, needV, needH bool) {
	if v == nil || scrollbarPx <= 0 {
		return
	}
	displayX, displayY := v.displayScroll(nil)
	if content.X > 0 || content.Y > 0 {
		maxX := max(0, content.X-v.viewportRect.Dx())
		maxY := max(0, content.Y-v.viewportRect.Dy())
		if displayX > maxX {
			displayX = maxX
		}
		if displayY > maxY {
			displayY = maxY
		}
	}
	if needV {
		track := image.Rect(v.viewportRect.Max.X, v.surfaceRect.Min.Y, v.surfaceRect.Max.X, v.viewportRect.Max.Y)
		if track.Dx() < 1 {
			track.Max.X = track.Min.X + 1
		}
		if track.Dy() < 1 {
			track.Max.Y = track.Min.Y + 1
		}
		v.vTrackRect = track
		v.vThumbRect = viewerImagePreviewThumbRect(track, v.viewportRect.Dy(), content.Y, displayY, true)
	}
	if needH {
		track := image.Rect(v.surfaceRect.Min.X, v.viewportRect.Max.Y, v.viewportRect.Max.X, v.surfaceRect.Max.Y)
		if track.Dx() < 1 {
			track.Max.X = track.Min.X + 1
		}
		if track.Dy() < 1 {
			track.Max.Y = track.Min.Y + 1
		}
		v.hTrackRect = track
		v.hThumbRect = viewerImagePreviewThumbRect(track, v.viewportRect.Dx(), content.X, displayX, false)
	}
}

func viewerImagePreviewThumbRect(track image.Rectangle, visible, total, scroll int, vertical bool) image.Rectangle {
	if total <= 0 || visible <= 0 {
		return image.Rectangle{}
	}
	if visible > total {
		visible = total
	}
	if vertical {
		if track.Dy() <= 0 {
			return image.Rectangle{}
		}
		thumbH := int(float32(track.Dy()) * float32(visible) / float32(total))
		if thumbH < 18 {
			thumbH = 18
		}
		if thumbH > track.Dy() {
			thumbH = track.Dy()
		}
		thumbY := track.Min.Y
		maxScroll := total - visible
		maxTravel := track.Dy() - thumbH
		if maxScroll > 0 && maxTravel > 0 {
			thumbY += int(float32(scroll) / float32(maxScroll) * float32(maxTravel))
		}
		return image.Rect(track.Min.X+1, thumbY, max(track.Min.X+2, track.Max.X-1), thumbY+thumbH)
	}
	if track.Dx() <= 0 {
		return image.Rectangle{}
	}
	thumbW := int(float32(track.Dx()) * float32(visible) / float32(total))
	if thumbW < 18 {
		thumbW = 18
	}
	if thumbW > track.Dx() {
		thumbW = track.Dx()
	}
	thumbX := track.Min.X
	maxScroll := total - visible
	maxTravel := track.Dx() - thumbW
	if maxScroll > 0 && maxTravel > 0 {
		thumbX += int(float32(scroll) / float32(maxScroll) * float32(maxTravel))
	}
	return image.Rect(thumbX, track.Min.Y+1, thumbX+thumbW, max(track.Min.Y+2, track.Max.Y-1))
}

func (v *imagePreviewView) verticalThumbGrabY(pos image.Point) int {
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

func (v *imagePreviewView) horizontalThumbGrabX(pos image.Point) int {
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

func (v *imagePreviewView) setScrollFromVerticalDrag(img image.Image, posY, grab int) bool {
	if v == nil || img == nil || v.vTrackRect.Dy() <= 0 || v.vThumbRect.Dy() <= 0 {
		return false
	}
	content := v.contentSize(img)
	maxScroll := max(0, content.Y-v.viewportRect.Dy())
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
	next := int(float32(travel) / float32(maxTravel) * float32(maxScroll))
	if next == v.scrollY {
		return false
	}
	v.scrollY = next
	return true
}

func (v *imagePreviewView) setScrollFromHorizontalDrag(img image.Image, posX, grab int) bool {
	if v == nil || img == nil || v.hTrackRect.Dx() <= 0 || v.hThumbRect.Dx() <= 0 {
		return false
	}
	content := v.contentSize(img)
	maxScroll := max(0, content.X-v.viewportRect.Dx())
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
	next := int(float32(travel) / float32(maxTravel) * float32(maxScroll))
	if next == v.scrollX {
		return false
	}
	v.scrollX = next
	return true
}

func (v *imagePreviewView) updateHover(pos image.Point) {
	if v == nil {
		return
	}
	v.hoverVTrack = viewerPointInRect(pos, v.vTrackRect)
	v.hoverVThumb = viewerPointInRect(pos, v.vThumbRect)
	v.hoverHTrack = viewerPointInRect(pos, v.hTrackRect)
	v.hoverHThumb = viewerPointInRect(pos, v.hThumbRect)
	v.hoverPDFTrack = viewerPointInRect(pos, v.pdfTrackRect)
	v.hoverPDFThumb = viewerPointInRect(pos, v.pdfThumbRect)
}

func (v *imagePreviewView) clearHover() {
	if v == nil {
		return
	}
	v.hoverVTrack = false
	v.hoverVThumb = false
	v.hoverHTrack = false
	v.hoverHThumb = false
	v.hoverPDFTrack = false
	v.hoverPDFThumb = false
}

func (v *imagePreviewView) clearPDFDocumentScrollbar() {
	if v == nil {
		return
	}
	v.pdfTrackRect = image.Rectangle{}
	v.pdfThumbRect = image.Rectangle{}
	v.hoverPDFTrack = false
	v.hoverPDFThumb = false
}

func (v *imagePreviewView) pdfDocumentThumbGrabY(pos image.Point) int {
	if v == nil || v.pdfThumbRect.Dy() <= 0 {
		return 0
	}
	grab := pos.Y - v.pdfThumbRect.Min.Y
	if grab < 0 {
		grab = 0
	}
	if grab > v.pdfThumbRect.Dy() {
		grab = v.pdfThumbRect.Dy()
	}
	return grab
}

func (v *imagePreviewView) pdfDocumentPageFromVerticalDrag(posY, grab, pageCount int) (int, bool) {
	if v == nil || pageCount <= 1 || v.pdfTrackRect.Dy() <= 0 || v.pdfThumbRect.Dy() <= 0 {
		return 0, false
	}
	maxTravel := v.pdfTrackRect.Dy() - v.pdfThumbRect.Dy()
	if maxTravel <= 0 {
		return 0, false
	}
	travel := posY - v.pdfTrackRect.Min.Y - grab
	if travel < 0 {
		travel = 0
	}
	if travel > maxTravel {
		travel = maxTravel
	}
	page := int(math.Round(float64(float32(travel) / float32(maxTravel) * float32(pageCount-1))))
	if page < 0 {
		page = 0
	}
	if page >= pageCount {
		page = pageCount - 1
	}
	return page, true
}

func viewerPDFDocumentScrollbarVisible(st *fileViewerState) bool {
	return viewerPDFPreviewActive(st) && st.imagePreviewPageCount > 1
}

func viewerPDFDocumentThumbPage(st *fileViewerState) int {
	if st == nil {
		return 0
	}
	page := st.imagePreviewPage
	if st.imageView.pdfDragging {
		page = st.imageView.pdfDragPage
	} else if st.previewRenderActive {
		page = st.previewRenderPage
	}
	if page < 0 {
		page = 0
	}
	if page >= st.imagePreviewPageCount {
		page = st.imagePreviewPageCount - 1
	}
	return page
}

func (v *imagePreviewView) setPDFDocumentScrollbar(track image.Rectangle, page, pageCount int) {
	if v == nil {
		return
	}
	v.pdfTrackRect = image.Rectangle{}
	v.pdfThumbRect = image.Rectangle{}
	if pageCount <= 1 || track.Dx() <= 0 || track.Dy() <= 0 {
		return
	}
	if page < 0 {
		page = 0
	}
	if page >= pageCount {
		page = pageCount - 1
	}
	v.pdfTrackRect = track
	v.pdfThumbRect = viewerImagePreviewThumbRect(track, 1, pageCount, page, true)
}

func (ui *UI) layoutImageOutputView(_ *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	size := gtx.Constraints.Max
	if st == nil || st.imagePreview == nil || size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: size}
	}
	ui.paintFileViewerImageBackdrop(gtx, size)
	v := &st.imageView
	scrollbarPx := gtx.Dp(unit.Dp(fileViewerImageScrollbarDp))
	if scrollbarPx < fileViewerImageScrollbarMinPx {
		scrollbarPx = fileViewerImageScrollbarMinPx
	}
	layoutSize := size
	pdfDocGapPx := 0
	pdfDocScrollbarPx := 0
	if viewerPDFDocumentScrollbarVisible(st) {
		pdfDocScrollbarPx = gtx.Dp(unit.Dp(fileViewerPDFDocScrollbarDp))
		if pdfDocScrollbarPx < fileViewerPDFDocScrollbarMinPx {
			pdfDocScrollbarPx = fileViewerPDFDocScrollbarMinPx
		}
		pdfDocGapPx = gtx.Dp(unit.Dp(fileViewerPDFDocScrollbarGapDp))
		if reserve := pdfDocGapPx + pdfDocScrollbarPx; reserve > 0 && reserve < layoutSize.X {
			layoutSize.X -= reserve
		} else {
			pdfDocGapPx = 0
			pdfDocScrollbarPx = 0
		}
	}
	v.computeLayout(layoutSize, 0, scrollbarPx, st.imagePreview)
	ui.layoutPDFDocumentScrollbarGeometry(st, size, layoutSize, pdfDocGapPx, pdfDocScrollbarPx)
	if st.pendingImageScrollToEnd {
		if v.scrollToEnd(st.imagePreview) {
			v.syncVisualScroll()
		}
		st.pendingImageScrollToEnd = false
		v.computeLayout(layoutSize, 0, scrollbarPx, st.imagePreview)
		ui.layoutPDFDocumentScrollbarGeometry(st, size, layoutSize, pdfDocGapPx, pdfDocScrollbarPx)
	}
	ui.handleImagePreviewEvents(gtx, st)
	animating := v.prepareVisualScroll(gtx.Now, viewerSmoothScrolling(ui.fmCfg), st.imagePreview)
	v.computeLayout(layoutSize, 0, scrollbarPx, st.imagePreview)
	ui.layoutPDFDocumentScrollbarGeometry(st, size, layoutSize, pdfDocGapPx, pdfDocScrollbarPx)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(streamSmoothTick)})
	}
	ui.paintImagePreview(gtx, st)
	ui.paintImagePreviewScrollbars(gtx, st)
	ui.applyImagePreviewCursor(gtx, st)
	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &v.pointerTag)
	pass.Pop()
	return layout.Dimensions{Size: size}
}

func (ui *UI) layoutPDFDocumentScrollbarGeometry(st *fileViewerState, size, layoutSize image.Point, gapPx, scrollbarPx int) {
	if st == nil {
		return
	}
	v := &st.imageView
	if !viewerPDFDocumentScrollbarVisible(st) || scrollbarPx <= 0 {
		v.clearPDFDocumentScrollbar()
		return
	}
	track := image.Rect(layoutSize.X+gapPx, 0, layoutSize.X+gapPx+scrollbarPx, size.Y)
	if track.Min.X < 0 {
		track.Min.X = 0
	}
	if track.Max.X > size.X {
		track.Max.X = size.X
	}
	if track.Dx() <= 0 || track.Dy() <= 0 {
		v.clearPDFDocumentScrollbar()
		return
	}
	v.setPDFDocumentScrollbar(track, viewerPDFDocumentThumbPage(st), st.imagePreviewPageCount)
}

func (ui *UI) handleImagePreviewEvents(gtx layout.Context, st *fileViewerState) {
	if st == nil || st.imagePreview == nil {
		return
	}
	v := &st.imageView
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
		pos := pe.Position.Round()
		switch pe.Kind {
		case pointer.Scroll:
			changed := false
			if pe.Scroll.X != 0 {
				changed = v.scrollWheelX(st.imagePreview, pe.Scroll.X) || changed
			}
			if pe.Scroll.Y != 0 {
				changed = v.scrollWheelY(st.imagePreview, pe.Scroll.Y) || changed
			}
			if changed {
				st.markUserBrowsing(gtx.Now)
			}
		case pointer.Press:
			if pe.Buttons.Contain(pointer.ButtonSecondary) {
				st.openContextMenu(pos, gtx.Now)
				continue
			}
			if !pe.Buttons.Contain(pointer.ButtonPrimary) {
				v.updateHover(pos)
				continue
			}
			if st.menuOpen {
				st.closeContextMenu()
			}
			switch {
			case viewerPointInRect(pos, v.pdfTrackRect):
				v.pdfDragging = true
				v.pdfDragID = pe.PointerID
				v.pdfDragGrab = v.pdfDocumentThumbGrabY(pos)
				v.pdfDragPage = st.imagePreviewPage
				gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				if page, ok := v.pdfDocumentPageFromVerticalDrag(pos.Y, v.pdfDragGrab, st.imagePreviewPageCount); ok {
					if page != v.pdfDragPage {
						v.pdfDragPage = page
						st.markUserBrowsing(gtx.Now)
						gtx.Execute(op.InvalidateCmd{})
					}
				}
			case viewerPointInRect(pos, v.vTrackRect):
				v.vDragging = true
				v.vDragID = pe.PointerID
				v.vDragGrab = v.verticalThumbGrabY(pos)
				gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				if v.setScrollFromVerticalDrag(st.imagePreview, pos.Y, v.vDragGrab) {
					st.markUserBrowsing(gtx.Now)
				}
			case viewerPointInRect(pos, v.hTrackRect):
				v.hDragging = true
				v.hDragID = pe.PointerID
				v.hDragGrab = v.horizontalThumbGrabX(pos)
				gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				if v.setScrollFromHorizontalDrag(st.imagePreview, pos.X, v.hDragGrab) {
					st.markUserBrowsing(gtx.Now)
				}
			}
			v.updateHover(pos)
		case pointer.Drag:
			changed := false
			if v.pdfDragging && pe.PointerID == v.pdfDragID {
				if page, ok := v.pdfDocumentPageFromVerticalDrag(pos.Y, v.pdfDragGrab, st.imagePreviewPageCount); ok && page != v.pdfDragPage {
					v.pdfDragPage = page
					changed = true
				}
			}
			if v.vDragging && pe.PointerID == v.vDragID {
				changed = v.setScrollFromVerticalDrag(st.imagePreview, pos.Y, v.vDragGrab) || changed
			}
			if v.hDragging && pe.PointerID == v.hDragID {
				changed = v.setScrollFromHorizontalDrag(st.imagePreview, pos.X, v.hDragGrab) || changed
			}
			if changed {
				st.markUserBrowsing(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
			}
			v.updateHover(pos)
		case pointer.Release:
			if v.pdfDragging && pe.PointerID == v.pdfDragID {
				targetPage := v.pdfDragPage
				v.pdfDragging = false
				v.pdfDragGrab = 0
				v.pdfDragPage = st.imagePreviewPage
				if targetPage >= 0 && targetPage < st.imagePreviewPageCount && targetPage != st.imagePreviewPage {
					ui.startFileViewerPDFPageRender(gtx.Now, targetPage, false)
					st.markUserBrowsing(gtx.Now)
					gtx.Execute(op.InvalidateCmd{})
				}
			}
			if v.vDragging && pe.PointerID == v.vDragID {
				v.vDragging = false
				v.vDragGrab = 0
			}
			if v.hDragging && pe.PointerID == v.hDragID {
				v.hDragging = false
				v.hDragGrab = 0
			}
			v.updateHover(pos)
		case pointer.Cancel:
			if v.pdfDragging && pe.PointerID == v.pdfDragID {
				v.pdfDragging = false
				v.pdfDragGrab = 0
				v.pdfDragPage = st.imagePreviewPage
			}
			if v.vDragging && pe.PointerID == v.vDragID {
				v.vDragging = false
				v.vDragGrab = 0
			}
			if v.hDragging && pe.PointerID == v.hDragID {
				v.hDragging = false
				v.hDragGrab = 0
			}
			v.clearHover()
		case pointer.Move, pointer.Enter:
			v.updateHover(pos)
		case pointer.Leave:
			v.clearHover()
		}
	}
}

func (ui *UI) paintImagePreview(gtx layout.Context, st *fileViewerState) {
	if st == nil || st.imagePreview == nil {
		return
	}
	v := &st.imageView
	if v.viewportRect.Dx() <= 0 || v.viewportRect.Dy() <= 0 {
		return
	}
	defer clip.Rect(v.viewportRect).Push(gtx.Ops).Pop()
	displayX, displayY := v.displayScroll(st.imagePreview)
	offset := op.Offset(image.Pt(v.viewportRect.Min.X-displayX, v.viewportRect.Min.Y-displayY)).Push(gtx.Ops)
	defer offset.Pop()
	zoom := v.effectiveZoom()
	if zoom != 1 {
		defer op.Affine(f32.AffineId().Scale(f32.Point{}, f32.Pt(zoom, zoom))).Push(gtx.Ops).Pop()
	}
	paint.NewImageOp(st.imagePreview).Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

func (ui *UI) paintImagePreviewScrollbars(gtx layout.Context, st *fileViewerState) {
	if st == nil {
		return
	}
	v := &st.imageView
	theme := ui.fileViewerTheme()
	draw := func(track, thumb image.Rectangle, hoverTrack, hoverThumb, dragging bool) {
		if track.Dx() <= 0 || track.Dy() <= 0 || thumb.Dx() <= 0 || thumb.Dy() <= 0 {
			return
		}
		trackColor := theme.ScrollTrack
		thumbColor := theme.ScrollThumb
		if hoverTrack || hoverThumb {
			trackColor = theme.ScrollTrackHover
			thumbColor = theme.ScrollThumbHover
		}
		if dragging {
			thumbColor = theme.ScrollThumbDrag
		}
		paint.FillShape(gtx.Ops, trackColor, clip.Rect(track).Op())
		paint.FillShape(gtx.Ops, thumbColor, clip.Rect(thumb).Op())
	}
	draw(v.vTrackRect, v.vThumbRect, v.hoverVTrack, v.hoverVThumb, v.vDragging)
	draw(v.hTrackRect, v.hThumbRect, v.hoverHTrack, v.hoverHThumb, v.hDragging)
	draw(v.pdfTrackRect, v.pdfThumbRect, v.hoverPDFTrack, v.hoverPDFThumb, v.pdfDragging)
}

func (ui *UI) applyImagePreviewCursor(gtx layout.Context, st *fileViewerState) {
	if st == nil {
		return
	}
	v := &st.imageView
	if v.pdfDragging {
		defer clip.Rect(v.pdfTrackRect).Push(gtx.Ops).Pop()
		pointer.CursorGrabbing.Add(gtx.Ops)
		return
	}
	if v.vDragging {
		defer clip.Rect(v.vTrackRect).Push(gtx.Ops).Pop()
		pointer.CursorGrabbing.Add(gtx.Ops)
		return
	}
	if v.hDragging {
		defer clip.Rect(v.hTrackRect).Push(gtx.Ops).Pop()
		pointer.CursorGrabbing.Add(gtx.Ops)
		return
	}
	switch {
	case v.hoverPDFTrack || v.hoverPDFThumb:
		defer clip.Rect(v.pdfTrackRect).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	case v.hoverVTrack || v.hoverVThumb:
		defer clip.Rect(v.vTrackRect).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	case v.hoverHTrack || v.hoverHThumb:
		defer clip.Rect(v.hTrackRect).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	}
}

func (ui *UI) paintFileViewerImageBackdrop(gtx layout.Context, size image.Point) {
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	theme := ui.fileViewerTheme()
	base := mixNRGBA(theme.PanelBg, theme.HeaderBg, 0.12)
	base = mixNRGBA(base, theme.CommandBg, 0.08)
	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, base, clip.Rect(image.Rectangle{Max: size}).Op())
}
