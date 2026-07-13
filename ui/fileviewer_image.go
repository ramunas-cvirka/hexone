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
	"gioui.org/widget/material"
	xdraw "golang.org/x/image/draw"
)

const (
	fileViewerImageKeyStepPx   = 48
	fileViewerImageWheelStepPx = 28
	fileViewerImageMinZoom     = float32(0.01)
	fileViewerImageMaxZoom     = float32(8)
	fileViewerImageZoomFactor  = float32(1.25)
)

type imagePreviewView struct {
	pointerTag struct{}

	surfaceRect  image.Rectangle
	viewportRect image.Rectangle
	vTrackRect   image.Rectangle
	vThumbRect   image.Rectangle
	hTrackRect   image.Rectangle
	hThumbRect   image.Rectangle

	zoom        float32
	zoomReady   bool
	alignTop    bool
	scrollX     int
	scrollY     int
	visualX     float32
	visualY     float32
	visualReady bool
	visualAt    time.Time
	wheelCarryX float32
	wheelCarryY float32

	hoverVTrack bool
	hoverVThumb bool
	hoverHTrack bool
	hoverHThumb bool

	vDragging bool
	vDragID   pointer.ID
	vDragGrab int

	hDragging bool
	hDragID   pointer.ID
	hDragGrab int

	panning bool
	panID   pointer.ID
	panLast f32.Point

	// downscale cache: when zoom < 1 the image is pre-downsampled in
	// software for better contrast than GPU bilinear filtering.
	downscaleSource image.Image
	downscaleZoom   float32
	downscaled      image.Image
	downscaleCh     chan imagePreviewDownscaleResult
	downscaleBusy   bool
}

type imagePreviewDownscaleResult struct {
	source image.Image
	zoom   float32
	img    image.Image
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
	v.zoom = 1
	v.zoomReady = false
	v.alignTop = false
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
	v.vDragging = false
	v.vDragID = 0
	v.vDragGrab = 0
	v.hDragging = false
	v.hDragID = 0
	v.hDragGrab = 0
	v.panning = false
	v.panID = 0
	v.panLast = f32.Point{}
	v.downscaleSource = nil
	v.downscaleZoom = 0
	v.downscaled = nil
	v.downscaleCh = nil
	v.downscaleBusy = false
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

func clampImagePreviewZoom(zoom float32) float32 {
	if zoom < fileViewerImageMinZoom {
		return fileViewerImageMinZoom
	}
	if zoom > fileViewerImageMaxZoom {
		return fileViewerImageMaxZoom
	}
	return zoom
}

func (v *imagePreviewView) initializeZoom(size image.Point, scrollbarPx int, img image.Image) {
	if v == nil || v.zoomReady || img == nil || size.X <= 0 || size.Y <= 0 {
		return
	}
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	v.zoom = 1
	v.alignTop = bounds.Dx() > size.X || bounds.Dy() > size.Y
	if bounds.Dx() > size.X {
		availableW := size.X
		zoom := float32(availableW) / float32(bounds.Dx())
		if int(math.Ceil(float64(float32(bounds.Dy())*zoom))) > size.Y && scrollbarPx > 0 {
			availableW -= scrollbarPx
			if availableW < 1 {
				availableW = 1
			}
			zoom = float32(availableW) / float32(bounds.Dx())
		}
		v.zoom = clampImagePreviewZoom(min(zoom, 1))
	}
	v.zoomReady = true
	v.scrollX = 0
	v.scrollY = 0
	v.syncVisualScroll()
}

func (v *imagePreviewView) fitWidthZoom(img image.Image) float32 {
	if v == nil || img == nil {
		return 1
	}
	width := v.viewportRect.Dx()
	if width <= 0 {
		width = v.surfaceRect.Dx()
	}
	if width <= 0 || img.Bounds().Dx() <= 0 {
		return 1
	}
	return clampImagePreviewZoom(float32(width) / float32(img.Bounds().Dx()))
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
	if !smooth || v.vDragging || v.hDragging || v.panning {
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

func (v *imagePreviewView) setZoom(img image.Image, newZoom float32) bool {
	if v == nil || img == nil || newZoom <= 0 {
		return false
	}
	oldZoom := v.effectiveZoom()
	newZoom = clampImagePreviewZoom(newZoom)
	if math.Abs(float64(newZoom-oldZoom)) < 0.0001 {
		return false
	}
	anchorX := float32(v.scrollX) / oldZoom
	anchorY := float32(v.scrollY) / oldZoom
	v.zoom = newZoom
	v.zoomReady = true
	v.scrollX = int(math.Round(float64(anchorX * newZoom)))
	v.scrollY = int(math.Round(float64(anchorY * newZoom)))
	v.clampScroll(img)
	return true
}

func (v *imagePreviewView) zoomBy(img image.Image, factor float32) bool {
	if v == nil || factor <= 0 {
		return false
	}
	return v.setZoom(img, v.effectiveZoom()*factor)
}

func (v *imagePreviewView) fitWidth(img image.Image) bool {
	if v == nil || img == nil {
		return false
	}
	changed := v.setZoom(img, v.fitWidthZoom(img))
	v.alignTop = true
	if v.scrollToOrigin() {
		changed = true
	}
	v.syncVisualScroll()
	return changed
}

func (v *imagePreviewView) contentOrigin(img image.Image) image.Point {
	if v == nil {
		return image.Point{}
	}
	displayX, displayY := v.displayScroll(img)
	origin := image.Pt(v.viewportRect.Min.X-displayX, v.viewportRect.Min.Y-displayY)
	content := v.contentSize(img)
	if extra := v.viewportRect.Dx() - content.X; extra > 0 {
		origin.X += extra / 2
	}
	if !v.alignTop {
		if extra := v.viewportRect.Dy() - content.Y; extra > 0 {
			origin.Y += extra / 2
		}
	}
	return origin
}

func (v *imagePreviewView) pumpDownscale() bool {
	if v == nil || v.downscaleCh == nil {
		return false
	}
	select {
	case result := <-v.downscaleCh:
		v.downscaleBusy = false
		if result.source != nil && result.img != nil && result.zoom == v.effectiveZoom() {
			v.downscaleSource = result.source
			v.downscaleZoom = result.zoom
			v.downscaled = result.img
			return true
		}
	default:
	}
	return false
}

func (v *imagePreviewView) requestDownscale(src image.Image, zoom float32) {
	if v == nil || src == nil || zoom <= 0 || zoom >= 1 {
		return
	}
	if v.downscaleSource == src && v.downscaleZoom == zoom && v.downscaled != nil {
		return
	}
	if v.downscaleBusy {
		return
	}
	if v.downscaleCh == nil {
		v.downscaleCh = make(chan imagePreviewDownscaleResult, 1)
	}
	v.downscaleBusy = true
	ch := v.downscaleCh
	go func() {
		b := src.Bounds()
		w := max(1, int(math.Round(float64(float32(b.Dx())*zoom))))
		h := max(1, int(math.Round(float64(float32(b.Dy())*zoom))))
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
		ch <- imagePreviewDownscaleResult{source: src, zoom: zoom, img: dst}
	}()
}

func (v *imagePreviewView) scrollWheelYStep(img image.Image, delta float32) (bool, bool) {
	if v == nil || delta == 0 {
		return false, false
	}
	if (delta > 0 && v.wheelCarryY < 0) || (delta < 0 && v.wheelCarryY > 0) {
		v.wheelCarryY = 0
	}
	v.wheelCarryY += delta * fileViewerImageWheelStepPx
	step := int(v.wheelCarryY)
	if step == 0 {
		return false, false
	}
	v.wheelCarryY -= float32(step)
	if v.scrollByPixels(img, 0, step) {
		return true, false
	}
	return false, true
}

func (v *imagePreviewView) scrollWheelY(img image.Image, delta float32) bool {
	changed, _ := v.scrollWheelYStep(img, delta)
	return changed
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

func (v *imagePreviewView) computeLayout(size image.Point, inset, verticalScrollbarPx, horizontalScrollbarPx int, img image.Image) {
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
		if needV && verticalScrollbarPx > 0 {
			view.Max.X -= verticalScrollbarPx
		}
		if needH && horizontalScrollbarPx > 0 {
			view.Max.Y -= horizontalScrollbarPx
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
	v.computeScrollbars(content, verticalScrollbarPx, horizontalScrollbarPx, needV, needH)
}

func (v *imagePreviewView) computeScrollbars(content image.Point, verticalScrollbarPx, horizontalScrollbarPx int, needV, needH bool) {
	if v == nil {
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
	if needV && verticalScrollbarPx > 0 {
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
	if needH && horizontalScrollbarPx > 0 {
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
	return viewerScrollbarThumbForScroll(track, visible, total, float64(scroll), vertical)
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

func (v *imagePreviewView) updateHover(pos image.Point) bool {
	if v == nil {
		return false
	}
	oldVTrack, oldVThumb := v.hoverVTrack, v.hoverVThumb
	oldHTrack, oldHThumb := v.hoverHTrack, v.hoverHThumb
	v.hoverVTrack = viewerPointInRect(pos, v.vTrackRect)
	v.hoverVThumb = viewerPointInRect(pos, v.vThumbRect)
	v.hoverHTrack = viewerPointInRect(pos, v.hTrackRect)
	v.hoverHThumb = viewerPointInRect(pos, v.hThumbRect)
	return oldVTrack != v.hoverVTrack ||
		oldVThumb != v.hoverVThumb ||
		oldHTrack != v.hoverHTrack ||
		oldHThumb != v.hoverHThumb
}

func (v *imagePreviewView) clearHover() bool {
	if v == nil {
		return false
	}
	changed := v.hoverVTrack || v.hoverVThumb || v.hoverHTrack || v.hoverHThumb
	v.hoverVTrack = false
	v.hoverVThumb = false
	v.hoverHTrack = false
	v.hoverHThumb = false
	return changed
}

func (ui *UI) layoutImageOutputView(_ *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	size := gtx.Constraints.Max
	if st == nil || st.imagePreview == nil || size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: size}
	}
	ui.paintFileViewerImageBackdrop(gtx, size)
	v := &st.imageView
	scrollbarPx := viewerScrollbarThickness(gtx, min(size.X, size.Y))
	v.initializeZoom(size, scrollbarPx, st.imagePreview)
	if v.pumpDownscale() {
		gtx.Execute(op.InvalidateCmd{})
	}
	v.computeLayout(size, 0, scrollbarPx, scrollbarPx, st.imagePreview)
	ui.handleImagePreviewEvents(gtx, st)
	animating := v.prepareVisualScroll(gtx.Now, viewerSmoothScrolling(ui.fmCfg), st.imagePreview)
	v.computeLayout(size, 0, scrollbarPx, scrollbarPx, st.imagePreview)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(streamSmoothTick)})
	}
	ui.paintImagePreview(gtx, st)
	if v.downscaleBusy {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	ui.paintImagePreviewScrollbars(gtx, st)
	ui.applyImagePreviewCursor(gtx, st)
	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &v.pointerTag)
	pass.Pop()
	return layout.Dimensions{Size: size}
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
		if ui.terminalFocused(gtx) && terminalSurfaceFocusPointerEvent(pe) {
			ui.releaseTerminalKeyboardFocus(gtx)
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
			case viewerPointInRect(pos, v.viewportRect):
				v.panning = true
				v.panID = pe.PointerID
				v.panLast = pe.Position
				gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				st.markUserBrowsing(gtx.Now)
			}
			if v.updateHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Drag:
			changed := false
			if v.panning && pe.PointerID == v.panID {
				dx := int(math.Round(float64(v.panLast.X - pe.Position.X)))
				dy := int(math.Round(float64(v.panLast.Y - pe.Position.Y)))
				v.panLast = pe.Position
				if v.scrollByPixels(st.imagePreview, dx, dy) {
					v.syncVisualScroll()
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
			if v.updateHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Release:
			if v.panning && pe.PointerID == v.panID {
				v.panning = false
			}
			if v.vDragging && pe.PointerID == v.vDragID {
				v.vDragging = false
				v.vDragGrab = 0
			}
			if v.hDragging && pe.PointerID == v.hDragID {
				v.hDragging = false
				v.hDragGrab = 0
			}
			if v.updateHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Cancel:
			if v.panning && pe.PointerID == v.panID {
				v.panning = false
			}
			if v.vDragging && pe.PointerID == v.vDragID {
				v.vDragging = false
				v.vDragGrab = 0
			}
			if v.hDragging && pe.PointerID == v.hDragID {
				v.hDragging = false
				v.hDragGrab = 0
			}
			if v.clearHover() {
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

func (ui *UI) paintImagePreview(gtx layout.Context, st *fileViewerState) {
	if st == nil || st.imagePreview == nil {
		return
	}
	v := &st.imageView
	if v.viewportRect.Dx() <= 0 || v.viewportRect.Dy() <= 0 {
		return
	}
	defer clip.Rect(v.viewportRect).Push(gtx.Ops).Pop()
	offset := op.Offset(v.contentOrigin(st.imagePreview)).Push(gtx.Ops)
	defer offset.Pop()
	zoom := v.effectiveZoom()
	img := image.Image(st.imagePreview)
	softwareScaled := false
	if zoom < 1 {
		// GPU bilinear filtering loses contrast when downscaling (it only
		// samples 4 nearby pixels). Pre-downsample in software with
		// CatmullRom, which uses a 4x4 kernel with negative lobes that
		// preserve edge sharpness — important for PDF text.
		src := img
		if v.downscaleSource == src && v.downscaleZoom == zoom && v.downscaled != nil {
			img = v.downscaled
			softwareScaled = true
		} else {
			v.requestDownscale(src, zoom)
		}
	}
	if !softwareScaled && zoom != 1 {
		defer op.Affine(f32.AffineId().Scale(f32.Point{}, f32.Pt(zoom, zoom))).Push(gtx.Ops).Pop()
	}
	paint.NewImageOp(img).Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

func (ui *UI) paintImagePreviewScrollbars(gtx layout.Context, st *fileViewerState) {
	if st == nil {
		return
	}
	v := &st.imageView
	theme := ui.fileViewerTheme()
	draw := func(track, thumb image.Rectangle, hoverTrack, hoverThumb, dragging bool) {
		paintViewerScrollbar(gtx, theme, track, thumb, hoverTrack, hoverThumb, dragging)
	}
	draw(v.vTrackRect, v.vThumbRect, v.hoverVTrack, v.hoverVThumb, v.vDragging)
	draw(v.hTrackRect, v.hThumbRect, v.hoverHTrack, v.hoverHThumb, v.hDragging)
}

func (ui *UI) applyImagePreviewCursor(gtx layout.Context, st *fileViewerState) {
	if st == nil {
		return
	}
	v := &st.imageView
	if v.panning || v.vDragging {
		pointer.CursorGrabbing.Add(gtx.Ops)
		return
	}
	if v.hDragging {
		pointer.CursorGrabbing.Add(gtx.Ops)
		return
	}
	switch {
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
