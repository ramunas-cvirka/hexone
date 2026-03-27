// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"math"

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
	fileViewerImageScrollbarDp    = 10
	fileViewerImageScrollbarMinPx = 8
	fileViewerImageKeyStepPx      = 48
	fileViewerImageWheelStepPx    = 28
	fileViewerImageMinZoom        = float32(0.25)
	fileViewerImageMaxZoom        = float32(8)
	fileViewerImageZoomFactor     = float32(1.25)
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
	scrollX     int
	scrollY     int
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
	v.scrollX = 0
	v.scrollY = 0
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
	if needV {
		track := image.Rect(v.viewportRect.Max.X, v.surfaceRect.Min.Y, v.surfaceRect.Max.X, v.viewportRect.Max.Y)
		if track.Dx() < 1 {
			track.Max.X = track.Min.X + 1
		}
		if track.Dy() < 1 {
			track.Max.Y = track.Min.Y + 1
		}
		v.vTrackRect = track
		v.vThumbRect = viewerImagePreviewThumbRect(track, v.viewportRect.Dy(), content.Y, v.scrollY, true)
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
		v.hThumbRect = viewerImagePreviewThumbRect(track, v.viewportRect.Dx(), content.X, v.scrollX, false)
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
}

func (v *imagePreviewView) clearHover() {
	if v == nil {
		return
	}
	v.hoverVTrack = false
	v.hoverVThumb = false
	v.hoverHTrack = false
	v.hoverHThumb = false
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
	v.computeLayout(size, 0, scrollbarPx, st.imagePreview)
	ui.handleImagePreviewEvents(gtx, st)
	v.computeLayout(size, 0, scrollbarPx, st.imagePreview)
	ui.paintImagePreview(gtx, st)
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
			if v.vDragging && pe.PointerID == v.vDragID {
				changed = v.setScrollFromVerticalDrag(st.imagePreview, pos.Y, v.vDragGrab) || changed
			}
			if v.hDragging && pe.PointerID == v.hDragID {
				changed = v.setScrollFromHorizontalDrag(st.imagePreview, pos.X, v.hDragGrab) || changed
			}
			if changed {
				st.markUserBrowsing(gtx.Now)
			}
			v.updateHover(pos)
		case pointer.Release:
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
	offset := op.Offset(image.Pt(v.viewportRect.Min.X-v.scrollX, v.viewportRect.Min.Y-v.scrollY)).Push(gtx.Ops)
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
}

func (ui *UI) applyImagePreviewCursor(gtx layout.Context, st *fileViewerState) {
	if st == nil {
		return
	}
	v := &st.imageView
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
