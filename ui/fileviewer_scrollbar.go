// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

const (
	fileViewerScrollbarWidthDp    = 10
	fileViewerScrollbarMinPx      = 6
	fileViewerScrollbarMinThumbPx = 22
)

func viewerScrollbarThickness(gtx layout.Context, minor int) int {
	if minor < fileViewerScrollbarMinPx {
		return 0
	}
	w := gtx.Dp(unit.Dp(fileViewerScrollbarWidthDp))
	if w < fileViewerScrollbarMinPx {
		w = fileViewerScrollbarMinPx
	}
	if w >= minor {
		w = minor - 1
	}
	if w < 1 {
		return 0
	}
	return w
}

func viewerScrollbarThumbForScroll(track image.Rectangle, visible, total int, scroll float64, vertical bool) image.Rectangle {
	return viewerScrollbarThumbForScroll64(track, int64(visible), int64(total), scroll, vertical)
}

func viewerScrollbarThumbForScroll64(track image.Rectangle, visible, total int64, scroll float64, vertical bool) image.Rectangle {
	if visible <= 0 || total <= 0 {
		return image.Rectangle{}
	}
	if visible > total {
		visible = total
	}
	trackLen := track.Dy()
	if !vertical {
		trackLen = track.Dx()
	}
	if trackLen <= 0 {
		return image.Rectangle{}
	}
	thumbLen := int(float64(trackLen) * float64(visible) / float64(total))
	if thumbLen < fileViewerScrollbarMinThumbPx {
		thumbLen = fileViewerScrollbarMinThumbPx
	}
	if thumbLen > trackLen {
		thumbLen = trackLen
	}
	maxScroll := float64(total - visible)
	thumbPos := 0
	if maxScroll > 0 && trackLen > thumbLen {
		if scroll < 0 {
			scroll = 0
		}
		if scroll > maxScroll {
			scroll = maxScroll
		}
		thumbPos = int(math.Round(scroll / maxScroll * float64(trackLen-thumbLen)))
	}
	return viewerScrollbarThumbFromPosition(track, thumbPos, thumbLen, vertical)
}

func viewerScrollbarThumbFromPosition(track image.Rectangle, thumbPos, thumbLen int, vertical bool) image.Rectangle {
	if track.Empty() {
		return image.Rectangle{}
	}
	trackLen := track.Dy()
	minorLen := track.Dx()
	if !vertical {
		trackLen = track.Dx()
		minorLen = track.Dy()
	}
	if trackLen <= 0 || minorLen <= 0 {
		return image.Rectangle{}
	}
	if thumbLen < 1 {
		thumbLen = 1
	}
	if thumbLen > trackLen {
		thumbLen = trackLen
	}
	travel := trackLen - thumbLen
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos > travel {
		thumbPos = travel
	}
	pad := minorLen / 4
	if pad < 1 {
		pad = 1
	}
	if pad*2 >= minorLen {
		pad = 0
	}

	thumb := track
	if vertical {
		thumb.Min.X += pad
		thumb.Max.X -= pad
		thumb.Min.Y = track.Min.Y + thumbPos
		thumb.Max.Y = thumb.Min.Y + thumbLen
	} else {
		thumb.Min.X = track.Min.X + thumbPos
		thumb.Max.X = thumb.Min.X + thumbLen
		thumb.Min.Y += pad
		thumb.Max.Y -= pad
	}
	if thumb.Empty() {
		return image.Rectangle{}
	}
	return thumb
}

func paintViewerScrollbar(gtx layout.Context, theme fileViewerTheme, track, thumb image.Rectangle, hoverTrack, hoverThumb, dragging bool) {
	if track.Dx() <= 0 || track.Dy() <= 0 {
		return
	}
	trackColor := theme.ScrollTrack
	thumbColor := theme.ScrollThumb
	if hoverTrack || hoverThumb || dragging {
		trackColor = theme.ScrollTrackHover
		thumbColor = theme.ScrollThumbHover
	}
	if dragging {
		thumbColor = theme.ScrollThumbDrag
	}
	paintViewerRoundedRect(gtx, track, trackColor)
	if thumb.Dx() > 0 && thumb.Dy() > 0 {
		paintViewerRoundedRect(gtx, thumb, thumbColor)
	}
}

func paintViewerRoundedRect(gtx layout.Context, rect image.Rectangle, fill color.NRGBA) {
	if fill.A == 0 || rect.Dx() <= 0 || rect.Dy() <= 0 {
		return
	}
	radius := rect.Dx()
	if rect.Dy() < radius {
		radius = rect.Dy()
	}
	radius /= 2
	if radius < 1 {
		radius = 1
	}
	paint.FillShape(gtx.Ops, fill, clip.UniformRRect(rect, radius).Op(gtx.Ops))
}
