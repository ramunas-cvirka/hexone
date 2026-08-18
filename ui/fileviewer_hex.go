// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"fmt"
	"hexone/filesys"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

const (
	hexViewerMinBytesPerLine = 4
	hexViewerMaxBytesPerLine = 64
	hexViewerMinChunkBytes   = 4096
	hexViewerSectionPadDp    = 8
	hexViewerWheelAccel      = 2.25
	hexViewerWheelMaxLines   = 3
)

type hexViewerState struct {
	resultCh chan fileViewerHexResult

	fileSize      int64
	bufferStart   int64
	buffer        []byte
	bytesPerLine  int
	topLine       int64
	visibleLines  int
	scrollCarry   float32
	visualTop     float64
	visualReady   bool
	visualAt      time.Time
	displayTop    int64
	displayY      int
	displayCount  int
	lastPaintTop  int64
	lastPaintSet  bool
	lastScrollDir int
	loadStart     int64
	loadEnd       int64

	offsetDigits int
	charW        int
	lineH        int
	leftPad      int
	columnGap    int

	offsetRect image.Rectangle
	hexRect    image.Rectangle
	textRect   image.Rectangle
	trackRect  image.Rectangle
	thumbRect  image.Rectangle

	hoverTrack bool
	dragging   bool
	dragID     pointer.ID
	dragGrabY  int
	dragTop    int64

	selecting  bool
	selectID   pointer.ID
	dragAnchor int64

	selectionStart   int64
	selectionLen     int64
	autoScrollActive bool
	autoScrollDir    int
	autoScrollStep   int
	autoScrollAt     time.Time
	cancelPending    bool
	cancelUntil      time.Time
	pointerOutside   bool
	selectPos        image.Point

	groupBytes int
	hexByteX   []int
	edits      map[int64]byte
	editCaret  int64
	editNibble int
	editASCII  bool
	editKeyTag fileViewerEventTag

	pointerTag fileViewerEventTag
}

type fileViewerHexResult struct {
	seq   int
	start int64
	size  int64
	data  []byte
	err   string
}

func newHexViewerState() *hexViewerState {
	return &hexViewerState{
		resultCh:   make(chan fileViewerHexResult, 1),
		groupBytes: 0,
	}
}

func (v *hexViewerState) ensureMetrics(ui *UI, th *material.Theme, gtx layout.Context) {
	if v == nil {
		return
	}
	v.charW = measureTypefaceCharWidth(ui, th, gtx, ui.viewerMonospaceTypeface())
	if v.charW < 1 {
		v.charW = 1
	}
	v.lineH = measureTypefaceLineHeight(ui, th, gtx, ui.viewerMonospaceTypeface())
	if v.lineH < 1 {
		v.lineH = 1
	}
	v.leftPad = gtx.Dp(unit.Dp(6))
	if v.leftPad < 2 {
		v.leftPad = 2
	}
	v.columnGap = hexSectionColumnGap(gtx, v.charW)
}

func hexSectionColumnGap(gtx layout.Context, charW int) int {
	pad := gtx.Dp(unit.Dp(hexViewerSectionPadDp))
	if pad < charW {
		pad = charW
	}
	if pad < 2 {
		pad = 2
	}
	// Keep one full, equal pad on either side of the one-pixel separator.
	return pad*2 + 1
}

func (v *hexViewerState) totalLines() int64 {
	if v == nil || v.bytesPerLine <= 0 || v.fileSize <= 0 {
		return 1
	}
	lines := v.fileSize / int64(v.bytesPerLine)
	if v.fileSize%int64(v.bytesPerLine) != 0 {
		lines++
	}
	if lines < 1 {
		lines = 1
	}
	return lines
}

func (v *hexViewerState) clampTop() {
	if v == nil {
		return
	}
	if v.topLine < 0 {
		v.topLine = 0
	}
	maxTop := v.totalLines() - int64(v.visibleLines)
	if maxTop < 0 {
		maxTop = 0
	}
	if v.topLine > maxTop {
		v.topLine = maxTop
	}
	if v.dragTop < 0 {
		v.dragTop = 0
	}
	if v.dragTop > maxTop {
		v.dragTop = maxTop
	}
	if v.visualReady {
		if v.visualTop < 0 {
			v.visualTop = 0
		}
		if v.visualTop > float64(maxTop) {
			v.visualTop = float64(maxTop)
		}
	}
}

func (v *hexViewerState) syncVisualTop() {
	if v == nil {
		return
	}
	v.visualTop = float64(v.topLine)
	v.visualReady = true
	v.visualAt = time.Time{}
	v.updateDisplayState()
}

func (v *hexViewerState) clampByteOffset(off int64) int64 {
	if v == nil || off < 0 {
		return 0
	}
	if v.fileSize <= 0 {
		return 0
	}
	if off >= v.fileSize {
		return v.fileSize - 1
	}
	return off
}

func (v *hexViewerState) clearSelection() {
	if v == nil {
		return
	}
	v.dragAnchor = 0
	v.selectionStart = 0
	v.selectionLen = 0
	v.cancelPending = false
	v.cancelUntil = time.Time{}
	v.pointerOutside = false
	v.stopAutoScroll()
}

func (v *hexViewerState) stopSelectionDrag() {
	if v == nil {
		return
	}
	v.selecting = false
	v.selectID = 0
	v.clearCancelGrace()
	v.pointerOutside = false
	v.stopAutoScroll()
}

func (v *hexViewerState) hasSelection() bool {
	return v != nil && v.selectionLen > 0
}

func (v *hexViewerState) selectionEnd() int64 {
	if v == nil || v.selectionLen <= 0 {
		return 0
	}
	return v.selectionStart + v.selectionLen
}

func (v *hexViewerState) setSelectionRange(start, length int64) {
	if v == nil || v.fileSize <= 0 {
		return
	}
	if start < 0 {
		start = 0
	}
	if start >= v.fileSize {
		start = v.fileSize - 1
	}
	if length < 0 {
		length = 0
	}
	if start+length > v.fileSize {
		length = v.fileSize - start
	}
	v.selectionStart = start
	v.selectionLen = length
}

func (v *hexViewerState) setSelectionFromAnchor(anchor, head int64) {
	if v == nil || v.fileSize <= 0 {
		return
	}
	anchor = v.clampByteOffset(anchor)
	head = v.clampByteOffset(head)

	start := anchor
	end := head
	if end < start {
		start, end = end, start
	}

	v.selectionStart = start
	v.selectionLen = end - start + 1
}

func (v *hexViewerState) stopAutoScroll() {
	v.autoScrollActive = false
	v.autoScrollDir = 0
	v.autoScrollStep = 0
	v.autoScrollAt = time.Time{}
}

func (v *hexViewerState) beginCancelGrace(now time.Time) {
	v.cancelPending = true
	v.cancelUntil = now.Add(streamCancelGrace)
}

func (v *hexViewerState) clearCancelGrace() {
	v.cancelPending = false
	v.cancelUntil = time.Time{}
}

func (v *hexViewerState) expireCancelGrace(now time.Time) bool {
	if v.autoScrollActive && v.selecting {
		return false
	}
	if !v.cancelPending || v.cancelUntil.IsZero() || now.Before(v.cancelUntil) {
		return false
	}
	v.clearCancelGrace()
	v.selecting = false
	v.selectID = 0
	v.pointerOutside = false
	v.stopAutoScroll()
	return true
}

func (v *hexViewerState) renderedLineCount() int {
	if v == nil {
		return 0
	}
	if v.displayCount > 0 {
		return v.displayCount
	}
	total := v.totalLines() - v.topLine
	if total < 0 {
		total = 0
	}
	if v.visibleLines < 1 {
		if total > 0 {
			return 1
		}
		return 0
	}
	if total > int64(v.visibleLines) {
		total = int64(v.visibleLines)
	}
	return int(total)
}

func (v *hexViewerState) smoothJumpThreshold() float64 {
	limit := float64(streamSmoothJumpLines)
	if visible := float64(v.visibleLines) * 0.75; visible > limit {
		limit = visible
	}
	return limit
}

func (v *hexViewerState) selectionAutoScrollActive() bool {
	return v != nil && v.autoScrollActive && v.selecting
}

func (v *hexViewerState) updateDisplayState() {
	if v == nil {
		return
	}
	total := v.totalLines()
	if total <= 0 {
		v.displayTop = 0
		v.displayY = 0
		v.displayCount = 0
		return
	}
	if v.visibleLines < 1 || v.lineH <= 0 {
		v.displayTop = v.topLine
		if v.displayTop < 0 {
			v.displayTop = 0
		}
		if maxTop := v.totalLines() - int64(v.visibleLines); v.displayTop > maxTop && maxTop >= 0 {
			v.displayTop = maxTop
		}
		v.displayY = 0
		v.displayCount = int(total - v.displayTop)
		if v.displayCount < 1 {
			v.displayCount = 1
		}
		return
	}

	visualTop := float64(v.topLine)
	if v.visualReady {
		visualTop = v.visualTop
	}
	maxTop := float64(v.totalLines() - int64(v.visibleLines))
	if maxTop < 0 {
		maxTop = 0
	}
	if visualTop < 0 {
		visualTop = 0
	}
	if visualTop > maxTop {
		visualTop = maxTop
	}
	top := int64(math.Floor(visualTop))
	frac := visualTop - float64(top)
	if frac < 0 {
		frac = 0
	}
	if frac > 0 && float64(top) >= maxTop {
		top = int64(maxTop)
		frac = 0
	}

	offsetY := 0
	if frac > 0 {
		offsetY = -int(math.Round(frac * float64(v.lineH)))
		if offsetY <= -v.lineH {
			offsetY = 0
			top++
		}
	}
	if top < 0 {
		top = 0
	}
	if max := v.totalLines() - int64(v.visibleLines); top > max && max >= 0 {
		top = max
		offsetY = 0
	}

	count := total - top
	maxRows := v.visibleLines
	if offsetY < 0 && top+int64(maxRows) < total {
		maxRows++
	}
	if count > int64(maxRows) {
		count = int64(maxRows)
	}
	if count < 1 {
		count = 1
	}
	v.displayTop = top
	v.displayY = offsetY
	v.displayCount = int(count)
}

func (v *hexViewerState) prepareVisualScroll(now time.Time, smooth bool) bool {
	if v == nil {
		return false
	}
	target := float64(v.topLine)
	if !v.visualReady {
		v.visualTop = target
		v.visualReady = true
		v.visualAt = now
		v.updateDisplayState()
		return false
	}
	if !smooth || v.dragging || v.selecting || v.cancelPending {
		v.visualTop = target
		v.visualAt = now
		v.updateDisplayState()
		return false
	}
	if math.Abs(target-v.visualTop) > v.smoothJumpThreshold() {
		v.visualTop = target
		v.visualAt = now
		v.updateDisplayState()
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
		v.visualTop = target
		v.visualAt = now
		v.updateDisplayState()
		return false
	}
	if dt == 0 && target != v.visualTop {
		dt = streamSmoothTick
	}
	if dt > 0 {
		blend := float64(clamp01(float32(1 - math.Exp(-float64(dt)/float64(streamSmoothTau)))))
		v.visualTop += (target - v.visualTop) * blend
	}
	v.visualAt = now
	if math.Abs(target-v.visualTop) < streamSmoothSnapEpsilon {
		v.visualTop = target
		v.updateDisplayState()
		return false
	}
	v.updateDisplayState()
	return true
}

func (v *hexViewerState) autoScrollParams(pos image.Point) (int, int) {
	if v == nil || v.lineH <= 0 {
		return 0, 0
	}
	rendered := v.renderedLineCount()
	if rendered < 1 {
		return 0, 0
	}
	body := v.hexRect.Union(v.textRect)
	if body.Dx() <= 0 || body.Dy() <= 0 {
		return 0, 0
	}
	top := body.Min.Y
	bottom := body.Min.Y + rendered*v.lineH
	dist := 0
	dir := 0
	switch {
	case pos.Y < top:
		dir = -1
		dist = top - pos.Y
	case pos.Y >= bottom:
		dir = 1
		dist = pos.Y - bottom + 1
	default:
		return 0, 0
	}
	step := 1
	if dist > streamAutoScrollMidPx {
		step = 7
	} else if dist > streamAutoScrollNearPx {
		step = 4
	} else {
		step = 2
	}
	return dir, step
}

func (v *hexViewerState) updateAutoScroll(pos image.Point, now time.Time) {
	if !v.selecting {
		v.stopAutoScroll()
		return
	}
	body := v.hexRect.Union(v.textRect)
	if viewerPointInRect(pos, body) {
		v.pointerOutside = false
		v.selectPos = pos
		v.stopAutoScroll()
		return
	}
	dir, step := v.autoScrollParams(pos)
	if v.pointerOutside && v.autoScrollActive && dir == 0 {
		// Pointer movement outside the app can jitter between synthetic neutral
		// coordinates. Keep the current autoscroll state stable until re-entry
		// or a clearly outside position arrives.
		return
	}
	v.selectPos = pos
	if dir == 0 {
		v.stopAutoScroll()
		return
	}
	prevDir := v.autoScrollDir
	prevStep := v.autoScrollStep
	wasActive := v.autoScrollActive
	v.autoScrollActive = true
	v.autoScrollDir = dir
	v.autoScrollStep = step
	if !wasActive || prevDir != dir || prevStep != step {
		v.autoScrollAt = now
	} else if v.autoScrollAt.IsZero() {
		v.autoScrollAt = now.Add(streamAutoScrollTick)
	}
}

func (v *hexViewerState) runAutoScroll(now time.Time) bool {
	if v == nil || !v.autoScrollActive || !v.selecting || v.autoScrollDir == 0 || v.autoScrollStep <= 0 {
		return false
	}
	if now.Before(v.autoScrollAt) {
		return false
	}
	before := v.topLine
	v.topLine += int64(v.autoScrollDir * v.autoScrollStep)
	v.clampTop()
	if byteOff, ok := hexSelectionByteAtPoint(v, v.selectPos); ok {
		v.setSelectionFromAnchor(v.dragAnchor, byteOff)
	}
	if v.topLine == before {
		v.stopAutoScroll()
		return false
	}
	v.autoScrollAt = now.Add(streamAutoScrollTick)
	return true
}

func (v *hexViewerState) visibleByteRange() (int64, int64) {
	if v == nil || v.bytesPerLine <= 0 {
		return 0, 0
	}
	start := v.topLine * int64(v.bytesPerLine)
	end := start + int64(v.visibleLines*v.bytesPerLine)
	if end < start {
		end = start
	}
	if v.fileSize > 0 && end > v.fileSize {
		end = v.fileSize
	}
	return start, end
}

func (v *hexViewerState) bufferCovers(start, end int64) bool {
	if v == nil {
		return false
	}
	bufferEnd := v.bufferStart + int64(len(v.buffer))
	return start >= v.bufferStart && end <= bufferEnd
}

func (v *hexViewerState) needsPrefetch() bool {
	if v == nil || len(v.buffer) == 0 || v.bytesPerLine <= 0 {
		return true
	}
	visibleStart, visibleEnd := v.visibleByteRange()
	bufferEnd := v.bufferStart + int64(len(v.buffer))

	margin := int64(v.visibleLines*v.bytesPerLine) / 2
	if margin < int64(hexViewerMinChunkBytes/4) {
		margin = int64(hexViewerMinChunkBytes / 4)
	}

	needsBefore := v.bufferStart > 0 && visibleStart < v.bufferStart+margin
	needsAfter := (v.fileSize <= 0 || bufferEnd < v.fileSize) && visibleEnd > bufferEnd-margin
	return needsBefore || needsAfter
}

func (v *hexViewerState) mergeBuffer(start int64, data []byte, maxBytes int64) {
	if v == nil || len(data) == 0 {
		return
	}
	if len(v.buffer) == 0 {
		v.bufferStart = start
		v.buffer = append([]byte(nil), data...)
		return
	}

	oldStart := v.bufferStart
	oldEnd := v.bufferStart + int64(len(v.buffer))
	newStart := start
	newEnd := start + int64(len(data))

	// If the new chunk is separate, keep the one that covers the viewport and replace otherwise.
	if newEnd < oldStart || newStart > oldEnd {
		visibleStart, visibleEnd := v.visibleByteRange()
		if v.bufferCovers(visibleStart, visibleEnd) {
			return
		}
		v.bufferStart = newStart
		v.buffer = append([]byte(nil), data...)
		return
	}

	mergedStart := oldStart
	if newStart < mergedStart {
		mergedStart = newStart
	}
	mergedEnd := oldEnd
	if newEnd > mergedEnd {
		mergedEnd = newEnd
	}

	mergedLen := mergedEnd - mergedStart
	merged := make([]byte, mergedLen)
	copy(merged[oldStart-mergedStart:], v.buffer)
	copy(merged[newStart-mergedStart:], data)

	if maxBytes > 0 && int64(len(merged)) > maxBytes {
		visibleStart, visibleEnd := v.visibleByteRange()
		keepStart := visibleStart - maxBytes/3
		if keepStart < mergedStart {
			keepStart = mergedStart
		}
		keepEnd := keepStart + maxBytes
		if keepEnd < visibleEnd {
			keepEnd = visibleEnd
			keepStart = keepEnd - maxBytes
		}
		if keepEnd > mergedEnd {
			keepEnd = mergedEnd
			keepStart = keepEnd - maxBytes
			if keepStart < mergedStart {
				keepStart = mergedStart
			}
		}
		merged = merged[keepStart-mergedStart : keepEnd-mergedStart]
		mergedStart = keepStart
	}

	v.bufferStart = mergedStart
	v.buffer = merged
}

func (v *hexViewerState) lineBytes(line int64) ([]byte, int64) {
	if v == nil || v.bytesPerLine <= 0 {
		return nil, 0
	}
	start := line * int64(v.bytesPerLine)
	if start >= v.fileSize {
		return nil, start
	}
	end := start + int64(v.bytesPerLine)
	if end > v.fileSize {
		end = v.fileSize
	}
	if !v.bufferCovers(start, end) {
		return nil, start
	}
	relStart := int(start - v.bufferStart)
	relEnd := int(end - v.bufferStart)
	if relStart < 0 || relEnd < relStart || relEnd > len(v.buffer) {
		return nil, start
	}
	data := v.buffer[relStart:relEnd]
	if len(v.edits) == 0 {
		return data, start
	}
	var edited []byte
	for off, value := range v.edits {
		if off < start || off >= end {
			continue
		}
		if edited == nil {
			edited = append([]byte(nil), data...)
		}
		edited[off-start] = value
	}
	if edited != nil {
		return edited, start
	}
	return data, start
}

// displayStartWithFallback keeps the last fully painted viewport visible
// while a newly requested scroll/jump target is still outside the buffer.
func (v *hexViewerState) displayStartWithFallback(start, end int64) (int64, bool) {
	if v == nil || v.bytesPerLine <= 0 || len(v.buffer) == 0 {
		return start, false
	}
	visibleStart := start * int64(v.bytesPerLine)
	visibleEnd := end * int64(v.bytesPerLine)
	if visibleEnd > v.fileSize {
		visibleEnd = v.fileSize
	}
	if v.bufferCovers(visibleStart, visibleEnd) {
		v.lastPaintTop = start
		v.lastPaintSet = true
		return start, false
	}
	fallback := v.lastPaintTop
	if !v.lastPaintSet {
		fallback = v.bufferStart / int64(v.bytesPerLine)
	}
	fallbackStart := fallback * int64(v.bytesPerLine)
	fallbackEnd := fallbackStart + (end-start)*int64(v.bytesPerLine)
	if fallbackEnd > v.fileSize {
		fallbackEnd = v.fileSize
	}
	if !v.bufferCovers(fallbackStart, fallbackEnd) {
		fallback = v.bufferStart / int64(v.bytesPerLine)
	}
	return fallback, true
}

func (v *hexViewerState) scrollByLines(lines int64) {
	if v == nil || lines == 0 {
		return
	}
	dir := 0
	if lines > 0 {
		dir = 1
	} else {
		dir = -1
	}
	if dir != 0 && v.lastScrollDir != 0 && dir != v.lastScrollDir {
		v.syncVisualTop()
	}
	v.lastScrollDir = dir
	v.topLine += lines
	v.clampTop()
}

func (v *hexViewerState) scrollByDelta(delta float32) {
	if v == nil || delta == 0 {
		return
	}
	// Gio's wheel delta varies significantly by input device. Accelerate fine
	// trackpad deltas so slow gestures respond promptly, while capping coarse
	// mouse-wheel events to avoid large jumps.
	delta *= hexViewerWheelAccel
	if delta > hexViewerWheelMaxLines {
		delta = hexViewerWheelMaxLines
	} else if delta < -hexViewerWheelMaxLines {
		delta = -hexViewerWheelMaxLines
	}
	if (delta > 0 && v.scrollCarry < 0) || (delta < 0 && v.scrollCarry > 0) {
		v.scrollCarry = 0
	}
	v.scrollCarry += delta

	steps := int64(0)
	for v.scrollCarry >= 1 {
		steps++
		v.scrollCarry -= 1
	}
	for v.scrollCarry <= -1 {
		steps--
		v.scrollCarry += 1
	}
	if steps == 0 {
		return
	}
	v.scrollByLines(steps)
}

func (v *hexViewerState) computeLayout(size image.Point, scrollbarW int) {
	if v == nil || size.X <= 0 || size.Y <= 0 {
		return
	}
	if scrollbarW < 0 {
		scrollbarW = 0
	}
	if scrollbarW > size.X {
		scrollbarW = size.X
	}
	trackGap := scrollbarW / 2
	if scrollbarW <= 0 {
		trackGap = 0
	} else if trackGap < 4 {
		trackGap = 4
	}
	offsetDigits := v.offsetDigits
	if offsetDigits < 8 {
		offsetDigits = 8
	}
	offsetW := offsetDigits * v.charW
	availW := size.X - v.leftPad*2 - scrollbarW - trackGap - offsetW - v.columnGap*2
	if availW < v.charW*8 {
		availW = v.charW * 8
	}
	newBPL := computeHexBytesPerLine(availW, v.charW, offsetDigits, v.groupBytes)
	if newBPL != v.bytesPerLine {
		oldBPL := v.bytesPerLine
		if oldBPL < 1 {
			oldBPL = newBPL
		}
		anchor := v.topLine * int64(oldBPL)
		v.bytesPerLine = newBPL
		v.topLine = anchor / int64(newBPL)
		v.visualReady = false
		v.displayTop = v.topLine
		v.displayY = 0
		v.displayCount = 0
	}
	v.offsetDigits = offsetDigits
	v.visibleLines = size.Y / v.lineH
	if v.visibleLines < 1 {
		v.visibleLines = 1
	}
	v.clampTop()

	hexCols := hexLineColumns(v.bytesPerLine, v.groupBytes)
	hexW := hexCols * v.charW
	textW := v.bytesPerLine * v.charW
	left := v.leftPad
	top := 0
	v.offsetRect = image.Rect(left, top, left+offsetW, size.Y)
	v.hexRect = image.Rect(v.offsetRect.Max.X+v.columnGap, top, v.offsetRect.Max.X+v.columnGap+hexW, size.Y)
	v.textRect = image.Rect(v.hexRect.Max.X+v.columnGap, top, v.hexRect.Max.X+v.columnGap+textW, size.Y)

	if scrollbarW > 0 {
		trackX := size.X - scrollbarW
		if trackX < v.textRect.Max.X+trackGap {
			trackX = v.textRect.Max.X + trackGap
		}
		if trackX > size.X-scrollbarW {
			trackX = size.X - scrollbarW
		}
		v.trackRect = image.Rect(trackX, 0, trackX+scrollbarW, size.Y)
	} else {
		v.trackRect = image.Rectangle{}
	}
	v.computeScrollbar()
	v.rebuildHexByteX()
}

func (v *hexViewerState) rebuildHexByteX() {
	if v == nil || v.bytesPerLine <= 0 || v.charW <= 0 {
		v.hexByteX = nil
		return
	}
	v.hexByteX = make([]int, v.bytesPerLine)
	x := 0
	for i := 0; i < v.bytesPerLine; i++ {
		v.hexByteX[i] = x
		x += 2 * v.charW
		if i+1 < v.bytesPerLine {
			x += v.charW
			if v.groupBytes > 1 && (i+1)%v.groupBytes == 0 {
				x += v.charW
			}
		}
	}
}

func (v *hexViewerState) hexByteLeft(i int) int {
	if v == nil || len(v.hexByteX) == 0 || i <= 0 {
		return 0
	}
	if i >= len(v.hexByteX) {
		return v.hexByteX[len(v.hexByteX)-1]
	}
	return v.hexByteX[i]
}

func (v *hexViewerState) hexByteRight(i int) int {
	return v.hexByteLeft(i) + 2*v.charW
}

func (v *hexViewerState) computeScrollbar() {
	if v == nil {
		return
	}
	totalLines := v.totalLines()
	if totalLines <= int64(v.visibleLines) || v.trackRect.Dy() <= 0 {
		v.thumbRect = image.Rectangle{}
		return
	}
	maxTop := totalLines - int64(v.visibleLines)
	thumbH := int(float64(v.trackRect.Dy()) * float64(v.visibleLines) / float64(totalLines))
	if thumbH < fileViewerScrollbarMinThumbPx {
		thumbH = fileViewerScrollbarMinThumbPx
	}
	if thumbH > v.trackRect.Dy() {
		thumbH = v.trackRect.Dy()
	}
	maxTravel := v.trackRect.Dy() - thumbH
	if v.dragging {
		topForThumb := v.dragTop
		if topForThumb < 0 {
			topForThumb = 0
		}
		if topForThumb > maxTop {
			topForThumb = maxTop
		}
		thumbY := 0
		if maxTop > 0 && maxTravel > 0 {
			thumbY = int(math.Round(float64(topForThumb) * float64(maxTravel) / float64(maxTop)))
		}
		v.thumbRect = viewerScrollbarThumbFromPosition(v.trackRect, thumbY, thumbH, true)
		return
	}
	thumbY := 0
	if maxTop > 0 && maxTravel > 0 {
		topForThumb := float64(v.topLine)
		if v.visualReady {
			topForThumb = v.visualTop
		}
		if topForThumb < 0 {
			topForThumb = 0
		}
		if topForThumb > float64(maxTop) {
			topForThumb = float64(maxTop)
		}
		thumbY = int(math.Round(topForThumb * float64(maxTravel) / float64(maxTop)))
	}
	v.thumbRect = viewerScrollbarThumbFromPosition(v.trackRect, thumbY, thumbH, true)
}

func (v *hexViewerState) updateScrollbarHover(pos image.Point) bool {
	if v == nil {
		return false
	}
	old := v.hoverTrack
	v.hoverTrack = viewerPointInRect(pos, v.trackRect)
	return old != v.hoverTrack
}

func (v *hexViewerState) clearScrollbarHover() bool {
	if v == nil {
		return false
	}
	changed := v.hoverTrack
	v.hoverTrack = false
	return changed
}

func (v *hexViewerState) lineFromPointY(y int) (int64, bool) {
	if v == nil {
		return 0, false
	}
	total := v.totalLines()
	if total < 1 {
		return 0, false
	}

	line := v.displayTop
	topY := v.hexRect.Min.Y + v.displayY
	rendered := v.renderedLineCount()
	if !v.visualReady && v.displayCount == 0 {
		line = v.topLine
		topY = v.hexRect.Min.Y
		rendered = int(total - v.topLine)
		if rendered > v.visibleLines {
			rendered = v.visibleLines
		}
	}
	if rendered < 1 {
		rendered = 1
	}

	renderedBottom := topY + rendered*v.lineH
	switch {
	case y < topY:
		// Clamp above the rendered rows to the first visible line.
	case y >= renderedBottom:
		line += int64(rendered - 1)
	default:
		row := (y - topY) / v.lineH
		if row < 0 {
			row = 0
		}
		if row >= rendered {
			row = rendered - 1
		}
		line += int64(row)
	}

	if line < 0 {
		line = 0
	}
	maxLine := total - 1
	if maxLine < 0 {
		maxLine = 0
	}
	if line > maxLine {
		line = maxLine
	}
	return line, true
}

func (v *hexViewerState) estimatedTopFromDragY(y, grabY int) int64 {
	if v == nil {
		return 0
	}
	totalLines := v.totalLines()
	if totalLines <= int64(v.visibleLines) {
		return 0
	}
	maxTop := totalLines - int64(v.visibleLines)
	maxTravel := v.trackRect.Dy() - v.thumbRect.Dy()
	if maxTravel <= 0 {
		return 0
	}
	if grabY < 0 {
		grabY = 0
	}
	if grabY > v.thumbRect.Dy() {
		grabY = v.thumbRect.Dy()
	}
	dragY := y - v.trackRect.Min.Y - grabY
	if dragY < 0 {
		dragY = 0
	}
	if dragY > maxTravel {
		dragY = maxTravel
	}
	return int64(float64(dragY) * float64(maxTop) / float64(maxTravel))
}

func (v *hexViewerState) verticalThumbGrabY(pos image.Point) int {
	if thumb := v.thumbRect; thumb.Dy() > 0 && viewerPointInRect(pos, thumb) {
		return pos.Y - thumb.Min.Y
	}
	return v.thumbRect.Dy() / 2
}

func computeHexBytesPerLine(width, charW, offsetDigits, groupBytes int) int {
	if charW < 1 {
		return 16
	}
	_ = offsetDigits
	best := hexViewerMinBytesPerLine
	for n := hexViewerMinBytesPerLine; n <= hexViewerMaxBytesPerLine; n++ {
		needed := (hexLineColumns(n, groupBytes) + n) * charW
		if needed <= width {
			best = n
		} else {
			break
		}
	}
	return best
}

func hexLineColumns(bytesPerLine, groupBytes int) int {
	if bytesPerLine <= 0 {
		return 0
	}
	cols := 0
	for i := 0; i < bytesPerLine; i++ {
		cols += 2
		if i+1 < bytesPerLine {
			cols++
			if groupBytes > 1 && (i+1)%groupBytes == 0 {
				cols++
			}
		}
	}
	return cols
}

func formatHexOffset(offset int64, digits int) string {
	if digits < 8 {
		digits = 8
	}
	return fmt.Sprintf("%0*X", digits, offset)
}

func formatHexLine(data []byte, bytesPerLine, groupBytes int) string {
	if bytesPerLine < 1 {
		bytesPerLine = 1
	}
	var b strings.Builder
	b.Grow(hexLineColumns(bytesPerLine, groupBytes))
	for i := 0; i < bytesPerLine; i++ {
		if i < len(data) {
			fmt.Fprintf(&b, "%02X", data[i])
		} else {
			b.WriteString("  ")
		}
		if i+1 < bytesPerLine {
			b.WriteByte(' ')
			if groupBytes > 1 && (i+1)%groupBytes == 0 {
				b.WriteByte(' ')
			}
		}
	}
	return b.String()
}

func formatHexTextLine(data []byte, bytesPerLine int) string {
	if bytesPerLine < 1 {
		bytesPerLine = 1
	}
	var b strings.Builder
	b.Grow(bytesPerLine)
	for i := 0; i < bytesPerLine; i++ {
		if i >= len(data) {
			b.WriteByte(' ')
			continue
		}
		r := rune(data[i])
		if r < 0x20 || !unicode.IsPrint(r) {
			b.WriteByte('.')
			continue
		}
		b.WriteByte(data[i])
	}
	return b.String()
}

func formatHexSelectionCopy(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	const digits = "0123456789ABCDEF"
	out := make([]byte, len(data)*2)
	for i, v := range data {
		out[i*2] = digits[v>>4]
		out[i*2+1] = digits[v&0x0F]
	}
	return string(out)
}

func formatHexSelectionTextCopy(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	const digits = "0123456789ABCDEF"
	var out strings.Builder
	out.Grow(len(data))
	decodeUTF8 := utf8.Valid(data)
	for index := 0; index < len(data); index++ {
		value := data[index]
		switch {
		case value == '\r' && index+1 < len(data) && data[index+1] == '\n':
			out.WriteString("\r\n")
			index++
		case value == '\n':
			out.WriteByte('\n')
		case value == '\\':
			out.WriteString(`\\`)
		case value >= 0x20 && value <= 0x7E:
			out.WriteByte(value)
		default:
			r, size := utf8.DecodeRune(data[index:])
			if decodeUTF8 && size > 1 && unicode.IsPrint(r) {
				out.Write(data[index : index+size])
				index += size - 1
				continue
			}
			out.WriteString(`\x`)
			out.WriteByte(digits[value>>4])
			out.WriteByte(digits[value&0x0F])
		}
	}
	return out.String()
}

func hexByteAtPoint(v *hexViewerState, pos image.Point) (int64, bool) {
	if v == nil || v.bytesPerLine <= 0 || v.lineH <= 0 || v.fileSize <= 0 {
		return 0, false
	}

	// Only react inside hex/text bands horizontally.
	inHex := viewerPointInRect(pos, v.hexRect)
	inText := viewerPointInRect(pos, v.textRect)
	if !inHex && !inText {
		return 0, false
	}

	line, ok := v.lineFromPointY(pos.Y)
	if !ok {
		line = 0
	}

	lineStart := line * int64(v.bytesPerLine)
	if lineStart >= v.fileSize {
		return v.fileSize - 1, true
	}

	var idx int
	if inText {
		idx = (pos.X - v.textRect.Min.X) / v.charW
	} else {
		bestIdx := 0
		bestDist := int(^uint(0) >> 1)
		for i := 0; i < v.bytesPerLine; i++ {
			x0 := v.hexRect.Min.X + v.hexByteLeft(i)
			x1 := v.hexRect.Min.X + v.hexByteRight(i)

			var dist int
			switch {
			case pos.X < x0:
				dist = x0 - pos.X
			case pos.X >= x1:
				dist = pos.X - (x1 - 1)
			default:
				dist = 0
			}

			if dist < bestDist {
				bestDist = dist
				bestIdx = i
			}
		}
		idx = bestIdx
	}

	if idx < 0 {
		idx = 0
	}
	if idx >= v.bytesPerLine {
		idx = v.bytesPerLine - 1
	}

	byteOffset := lineStart + int64(idx)
	if byteOffset >= v.fileSize {
		byteOffset = v.fileSize - 1
	}
	if byteOffset < 0 {
		byteOffset = 0
	}
	return byteOffset, true
}

func hexSelectionByteAtPoint(v *hexViewerState, pos image.Point) (int64, bool) {
	if v == nil {
		return 0, false
	}
	body := v.hexRect.Union(v.textRect)
	if body.Dx() <= 0 || body.Dy() <= 0 {
		return 0, false
	}
	if pos.Y < body.Min.Y {
		pos.Y = body.Min.Y
	} else if pos.Y >= body.Max.Y {
		pos.Y = body.Max.Y - 1
	}
	if viewerPointInRect(pos, v.hexRect) || viewerPointInRect(pos, v.textRect) {
		return hexByteAtPoint(v, pos)
	}
	if pos.X < v.hexRect.Min.X {
		pos.X = v.hexRect.Min.X
	} else if pos.X >= v.textRect.Max.X {
		pos.X = v.textRect.Max.X - 1
	} else if pos.X >= v.hexRect.Max.X && pos.X < v.textRect.Min.X {
		hexGap := pos.X - (v.hexRect.Max.X - 1)
		textGap := v.textRect.Min.X - pos.X
		if hexGap <= textGap {
			pos.X = v.hexRect.Max.X - 1
		} else {
			pos.X = v.textRect.Min.X
		}
	}
	return hexByteAtPoint(v, pos)
}

func hexEditASCIIAtPoint(v *hexViewerState, pos image.Point) bool {
	if v == nil {
		return false
	}
	if viewerPointInRect(pos, v.textRect) {
		return true
	}
	if viewerPointInRect(pos, v.hexRect) {
		return false
	}
	return pos.X >= v.hexRect.Max.X+(v.textRect.Min.X-v.hexRect.Max.X)/2
}

func (ui *UI) ensureHexViewer(st *fileViewerState) *hexViewerState {
	if st == nil {
		return nil
	}
	if st.hex == nil {
		st.hex = newHexViewerState()
	}
	return st.hex
}

func (ui *UI) startHexViewerLoad(st *fileViewerState, force bool) {
	if st == nil {
		return
	}
	v := ui.ensureHexViewer(st)
	if v == nil || v.bytesPerLine <= 0 {
		return
	}
	visibleStart, visibleEnd := v.visibleByteRange()
	windowBytes := int64(v.visibleLines * v.bytesPerLine)
	padBytes := windowBytes * 4
	if padBytes < hexViewerMinChunkBytes {
		padBytes = hexViewerMinChunkBytes
	}
	wantStart := visibleStart - padBytes
	if wantStart < 0 {
		wantStart = 0
	}
	wantEnd := visibleEnd + padBytes
	if v.fileSize > 0 && wantEnd > v.fileSize {
		wantEnd = v.fileSize
	}
	if wantEnd < wantStart+int64(hexViewerMinChunkBytes) {
		wantEnd = wantStart + int64(hexViewerMinChunkBytes)
	}
	if v.fileSize > 0 && wantEnd > v.fileSize {
		wantEnd = v.fileSize
	}
	if !force && len(v.buffer) > 0 && !v.needsPrefetch() {
		return
	}
	if st.loading {
		// A fast scroll or find jump can move outside the in-flight request.
		// Supersede that request immediately instead of waiting for it and then
		// starting a second load for the actual target.
		if visibleStart >= v.loadStart && visibleEnd <= v.loadEnd {
			return
		}
		if st.loadCancel != nil {
			st.loadCancel()
		}
		st.loadCancel = nil
		st.loading = false
	}

	st.seq++
	seq := st.seq
	st.loading = true
	v.loadStart = wantStart
	v.loadEnd = wantEnd
	st.err = ""
	if len(v.buffer) == 0 {
		st.status = "loading..."
	}
	ctx, cancel := context.WithCancel(context.Background())
	st.loadCancel = cancel
	path := st.path
	remote := st.remote
	ch := v.resultCh
	go func() {
		res := readHexViewerChunk(path, remote, wantStart, wantEnd-wantStart)
		res.seq = seq
		if ctx.Err() != nil {
			return
		}
		sendHexViewerResult(ch, res)
		ui.invalidateFromWorker()
	}()
}

func sendHexViewerResult(ch chan fileViewerHexResult, res fileViewerHexResult) {
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

func readHexViewerChunk(path string, remote *paneSSHSession, start, length int64) fileViewerHexResult {
	res := fileViewerHexResult{start: start}
	if start < 0 {
		start = 0
	}
	if length < 1 {
		length = hexViewerMinChunkBytes
	}

	var (
		size int64
		file interface {
			io.ReaderAt
			io.Closer
		}
		err error
	)
	if remote == nil {
		if filesys.ArchiveMemberPath(path) {
			chunk, chunkSize, chunkErr := filesys.ReadLocalFileChunk(path, start, length)
			if chunkErr != nil {
				res.err = chunkErr.Error()
				return res
			}
			res.size = chunkSize
			res.data = chunk
			return res
		}
		info, statErr := filesys.StatLocalFilesystemPath(path)
		if statErr != nil {
			res.err = statErr.Error()
			return res
		}
		if info.IsDir() {
			res.err = "viewer supports files only"
			return res
		}
		if !info.Mode().IsRegular() {
			res.err = viewerUnsupportedFileNotice(path, info.Mode())
			return res
		}
		size = info.Size()
		file, err = os.Open(path)
	} else {
		client := remote.sftpClient()
		if client == nil {
			res.err = "sftp session is not connected"
			return res
		}
		info, statErr := client.Stat(path)
		if statErr != nil {
			res.err = statErr.Error()
			return res
		}
		if info.IsDir() {
			res.err = "viewer supports files only"
			return res
		}
		if !info.Mode().IsRegular() {
			res.err = viewerUnsupportedFileNotice(path, info.Mode())
			return res
		}
		size = info.Size()
		file, err = client.Open(path)
	}
	if err != nil {
		res.err = err.Error()
		return res
	}
	defer file.Close()

	res.size = size
	if start > size {
		start = size
	}
	if start+length > size {
		length = size - start
	}
	if length < 0 {
		length = 0
	}
	buf := make([]byte, length)
	if length > 0 {
		n, readErr := file.ReadAt(buf, start)
		if readErr != nil && readErr != io.EOF {
			res.err = readErr.Error()
			return res
		}
		buf = buf[:n]
	}
	res.start = start
	res.data = buf
	return res
}

func (ui *UI) pumpHexViewerState(gtx layout.Context, st *fileViewerState) {
	if st == nil || st.hex == nil || st.hex.resultCh == nil {
		return
	}
	for {
		select {
		case res := <-st.hex.resultCh:
			if res.seq != st.seq {
				continue
			}
			st.loading = false
			st.loadCancel = nil
			st.hex.loadStart = 0
			st.hex.loadEnd = 0
			if res.err != "" {
				st.err = res.err
				st.status = ""
			} else {
				st.err = ""
				st.status = ""
				st.hex.fileSize = res.size
				maxKeep := int64(st.hex.visibleLines * st.hex.bytesPerLine * 12)
				if maxKeep < int64(hexViewerMinChunkBytes*2) {
					maxKeep = int64(hexViewerMinChunkBytes * 2)
				}
				st.hex.mergeBuffer(res.start, res.data, maxKeep)
				st.hex.offsetDigits = viewerHexOffsetDigits(res.size)
				st.hex.clampTop()
				st.captureWatchState()
			}
			gtx.Execute(op.InvalidateCmd{})
		default:
			return
		}
	}
}

func viewerHexOffsetDigits(size int64) int {
	if size <= 0xFFFFFFFF {
		return 8
	}
	return 16
}

func (ui *UI) layoutHexOutputView(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	v := ui.ensureHexViewer(st)
	if v == nil {
		return layout.Dimensions{}
	}
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: size}
	}

	v.ensureMetrics(ui, th, gtx)
	scrollbarW := viewerScrollbarThickness(gtx, size.X)
	v.computeLayout(size, scrollbarW)
	v.prepareVisualScroll(gtx.Now, viewerSmoothScrolling(ui.fmCfg))
	v.computeScrollbar()
	ui.handleHexViewerEvents(gtx, st)
	if st.editMode {
		st.editFocus = false
		gtx.Execute(key.FocusCmd{Tag: &v.editKeyTag})
	}
	if v.expireCancelGrace(gtx.Now) {
		st.markUserBrowsing(gtx.Now)
	}
	if v.runAutoScroll(gtx.Now) {
		st.markUserBrowsing(gtx.Now)
		ui.startHexViewerLoad(st, false)
	}
	animating := v.prepareVisualScroll(gtx.Now, viewerSmoothScrolling(ui.fmCfg))
	if v.selectionAutoScrollActive() && v.selecting {
		beforeStart, beforeLen := v.selectionStart, v.selectionLen
		if byteOff, ok := hexSelectionByteAtPoint(v, v.selectPos); ok {
			v.setSelectionFromAnchor(v.dragAnchor, byteOff)
		}
		if v.selectionStart != beforeStart || v.selectionLen != beforeLen {
			st.markUserBrowsing(gtx.Now)
		}
	}
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(streamSmoothTick)})
	}
	if v.autoScrollActive && !v.autoScrollAt.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: v.autoScrollAt})
	}
	if v.cancelPending && !v.cancelUntil.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: v.cancelUntil})
	}
	v.computeScrollbar()
	ui.startHexViewerLoad(st, false)

	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	ui.drawHexOutput(gtx, th, st)
	ui.drawHexScrollbar(gtx, st)
	ui.drawHexScrollTooltip(gtx, th, st)
	ui.applyHexViewerCursor(gtx, st)

	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &v.pointerTag)
	pass.Pop()
	if st.editMode {
		event.Op(gtx.Ops, &v.editKeyTag)
	}
	return layout.Dimensions{Size: size}
}

func (ui *UI) drawHexOutput(gtx layout.Context, th *material.Theme, st *fileViewerState) {
	v := st.hex
	if v == nil {
		return
	}
	theme := ui.fileViewerTheme()
	for _, rect := range hexSectionSeparatorRects(v) {
		if !rect.Empty() {
			paint.FillShape(gtx.Ops, theme.Separator, clip.Rect(rect).Op())
		}
	}

	y := v.displayY
	total := v.totalLines()
	start := v.displayTop
	if !v.visualReady && v.displayCount == 0 {
		start = v.topLine
		y = 0
	}
	end := start + int64(v.renderedLineCount())
	if end > total {
		end = total
	}
	if fallbackStart, fallback := v.displayStartWithFallback(start, end); fallback {
		start = fallbackStart
		y = 0
		end = start + int64(v.renderedLineCount())
		if end > total {
			end = total
		}
	}
	for line := start; line < end; line++ {
		lineBytes, lineStart := v.lineBytes(line)
		offsetText := formatHexOffset(lineStart, v.offsetDigits)
		offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		ui.drawHexLineFindMatch(gtx, th, st, line, len(lineBytes))
		ui.drawHexLineSelections(gtx, th, st, line, len(lineBytes))
		ui.drawHexEditCursorBackground(gtx, st, line, len(lineBytes))
		ui.drawHexOffsetLine(th, gtx, v, offsetText, theme.OffsetText)
		ui.drawHexBytesLine(th, gtx, st, lineStart, lineBytes, theme.HexText, theme.ModifiedText)
		ui.drawHexASCIIline(th, gtx, st, lineStart, lineBytes, theme.ASCIIText, theme.ModifiedText)
		offset.Pop()
		y += v.lineH
	}
}

func (ui *UI) drawHexEditCursorBackground(gtx layout.Context, st *fileViewerState, line int64, lineLen int) {
	if st == nil || !st.editMode || st.mode != "hex" || st.hex == nil || lineLen <= 0 {
		return
	}
	v := st.hex
	lineStart := line * int64(v.bytesPerLine)
	if v.editCaret < lineStart || v.editCaret >= lineStart+int64(lineLen) {
		return
	}
	idx := int(v.editCaret - lineStart)
	x := v.hexRect.Min.X + v.hexByteLeft(idx)
	width := 3 * v.charW
	if v.editASCII {
		x = v.textRect.Min.X + idx*v.charW
		width = v.charW
	}
	maxX := v.hexRect.Max.X
	if v.editASCII {
		maxX = v.textRect.Max.X
	}
	if x+width > maxX {
		width = maxX - x
	}
	if width <= 0 {
		return
	}
	paint.FillShape(gtx.Ops, ui.fileViewerTheme().EditCursorBg, clip.Rect(image.Rect(x, 0, x+width, v.lineH)).Op())
}

func setHexViewerEditCaret(v *hexViewerState, byteOffset int64, ascii bool) {
	if v == nil {
		return
	}
	v.editCaret = v.clampByteOffset(byteOffset)
	v.editNibble = 0
	v.editASCII = ascii
	v.setSelectionRange(v.editCaret, 1)
}

func hexSectionSeparatorRects(v *hexViewerState) [2]image.Rectangle {
	if v == nil {
		return [2]image.Rectangle{}
	}
	height := v.offsetRect.Dy()
	if height <= 0 {
		return [2]image.Rectangle{}
	}
	firstGap := v.hexRect.Min.X - v.offsetRect.Max.X
	secondGap := v.textRect.Min.X - v.hexRect.Max.X
	firstX := v.offsetRect.Max.X + (firstGap-1)/2
	secondX := v.hexRect.Max.X + (secondGap-1)/2
	return [2]image.Rectangle{
		image.Rect(firstX, v.offsetRect.Min.Y, firstX+1, v.offsetRect.Min.Y+height),
		image.Rect(secondX, v.offsetRect.Min.Y, secondX+1, v.offsetRect.Min.Y+height),
	}
}

func (ui *UI) drawMonoCell(th *material.Theme, gtx layout.Context, pos image.Point, width, rowH int, text string, fg color.NRGBA) {
	if width <= 0 || rowH <= 0 || text == "" {
		return
	}
	offset := op.Offset(pos).Push(gtx.Ops)
	lineGTX := gtx
	lineGTX.Constraints = layout.Exact(image.Pt(width, rowH))
	lbl := material.Body2(th, text)
	lbl.Font.Typeface = ui.viewerMonospaceTypeface()
	lbl.Font.Weight = font.Normal
	lbl.TextSize = ui.viewerTextSize()
	lbl.Color = fg
	lbl.MaxLines = 1
	lbl.Truncator = ""
	layoutVCenteredLabel(lineGTX, lbl)
	offset.Pop()
}

func (ui *UI) drawHexOffsetLine(th *material.Theme, gtx layout.Context, v *hexViewerState, text string, fg color.NRGBA) {
	if v == nil {
		return
	}
	for i, r := range text {
		x := v.offsetRect.Min.X + i*v.charW
		ui.drawMonoCell(th, gtx, image.Pt(x, 0), v.charW, v.lineH, string(r), fg)
	}
}

func (ui *UI) drawHexBytesLine(th *material.Theme, gtx layout.Context, st *fileViewerState, lineStart int64, data []byte, fg, modified color.NRGBA) {
	if st == nil || st.hex == nil {
		return
	}
	v := st.hex
	activeColor := functionBarKeyTextColor(ui.fileViewerTheme().HeaderText)
	for i, b := range data {
		txt := fmt.Sprintf("%02X", b)
		x := v.hexRect.Min.X + v.hexByteLeft(i)
		byteOffset := lineStart + int64(i)
		byteColor := fg
		if _, changed := v.edits[byteOffset]; changed {
			byteColor = modified
		}
		if st.editMode && hexEditNibbleActiveAt(v, byteOffset) {
			for nibble, r := range txt {
				nibbleColor := byteColor
				if nibble == v.editNibble {
					nibbleColor = activeColor
				}
				ui.drawMonoCell(th, gtx, image.Pt(x+nibble*v.charW, 0), v.charW, v.lineH, string(r), nibbleColor)
			}
			continue
		}
		ui.drawMonoCell(th, gtx, image.Pt(x, 0), 2*v.charW, v.lineH, txt, byteColor)
	}
}

func hexEditNibbleActiveAt(v *hexViewerState, byteOffset int64) bool {
	if v == nil || v.editASCII {
		return false
	}
	if v.selectionLen > 1 {
		return byteOffset >= v.selectionStart && byteOffset < v.selectionEnd()
	}
	return v.editCaret == byteOffset
}

func hexEditASCIIActiveAt(v *hexViewerState, byteOffset int64) bool {
	if v == nil || !v.editASCII {
		return false
	}
	if v.selectionLen > 1 {
		return byteOffset >= v.selectionStart && byteOffset < v.selectionEnd()
	}
	return v.editCaret == byteOffset
}

func (ui *UI) drawHexASCIIline(th *material.Theme, gtx layout.Context, st *fileViewerState, lineStart int64, data []byte, fg, modified color.NRGBA) {
	if st == nil || st.hex == nil {
		return
	}
	v := st.hex
	activeColor := functionBarKeyTextColor(ui.fileViewerTheme().HeaderText)
	for i, b := range data {
		r := rune(b)
		ch := "."
		if r >= 0x20 && unicode.IsPrint(r) {
			ch = string(b)
		}
		x := v.textRect.Min.X + i*v.charW
		byteOffset := lineStart + int64(i)
		byteColor := fg
		if _, changed := v.edits[byteOffset]; changed {
			byteColor = modified
		}
		if st.editMode && hexEditASCIIActiveAt(v, byteOffset) {
			byteColor = activeColor
		}
		ui.drawMonoCell(th, gtx, image.Pt(x, 0), v.charW, v.lineH, ch, byteColor)
	}
}

func (ui *UI) drawHexLineSelections(gtx layout.Context, th *material.Theme, st *fileViewerState, line int64, lineLen int) {
	v := st.hex
	if v == nil || !v.hasSelection() || lineLen <= 0 {
		return
	}
	if st.editMode && st.mode == "hex" && v.selectionLen <= 1 {
		return
	}
	theme := ui.fileViewerTheme()
	hexSelection, textSelection := hexSelectionLaneColors(theme, v, st.editMode && st.mode == "hex")
	ui.drawHexLineRangeHighlight(gtx, th, st, line, lineLen, v.selectionStart, v.selectionEnd(), hexSelection, textSelection, true)
}

func hexSelectionLaneColors(theme fileViewerTheme, v *hexViewerState, editMode bool) (color.NRGBA, color.NRGBA) {
	hexSelection := theme.HexSelection
	textSelection := theme.HexStrongSelection
	if editMode && v != nil {
		textSelection = theme.HexSelection
		if v.editASCII {
			textSelection = theme.EditCursorBg
		} else {
			hexSelection = theme.EditCursorBg
		}
	}
	return hexSelection, textSelection
}

func (ui *UI) drawHexLineFindMatch(gtx layout.Context, th *material.Theme, st *fileViewerState, line int64, lineLen int) {
	if st == nil || !st.find.open || !st.find.currentValid || st.find.currentLen <= 0 {
		return
	}
	theme := ui.fileViewerTheme()
	findHex, findText := fileViewerFindHighlightColors(theme)
	ui.drawHexLineRangeHighlight(gtx, th, st, line, lineLen, st.find.currentStart, st.find.currentStart+st.find.currentLen, findHex, findText, false)
}

func (ui *UI) drawHexLineRangeHighlight(gtx layout.Context, th *material.Theme, st *fileViewerState, line int64, lineLen int, rangeStart, rangeEnd int64, hexSel, textSel color.NRGBA, fullRow bool) {
	v := st.hex
	if v == nil || lineLen <= 0 || rangeEnd <= rangeStart {
		return
	}
	lineStart := line * int64(v.bytesPerLine)
	lineEnd := lineStart + int64(lineLen)
	if lineEnd <= rangeStart || lineStart >= rangeEnd {
		return
	}

	selStart := rangeStart
	if selStart < lineStart {
		selStart = lineStart
	}
	selEnd := rangeEnd
	if selEnd > lineEnd {
		selEnd = lineEnd
	}
	if selEnd <= selStart {
		return
	}

	firstIdx := int(selStart - lineStart)
	lastIdx := int(selEnd - lineStart - 1)

	hexX0 := v.hexRect.Min.X + v.hexByteLeft(firstIdx)
	var hexX1 int
	if lastIdx+1 < lineLen {
		hexX1 = v.hexRect.Min.X + v.hexByteLeft(lastIdx+1)
	} else {
		hexX1 = v.hexRect.Min.X + v.hexByteRight(lastIdx)
	}
	hexRect := viewerLineContentRect(ui, th, gtx, ui.viewerMonospaceTypeface(), ui.viewerTextSize(), v.lineH, hexX0, hexX1)
	if fullRow {
		hexRect = viewerLineSelectionRect(v.lineH, hexX0, hexX1)
	}
	if !hexRect.Empty() {
		paint.FillShape(gtx.Ops, hexSel, clip.Rect(hexRect).Op())
	}

	textX0 := v.textRect.Min.X + firstIdx*v.charW
	textX1 := v.textRect.Min.X + (lastIdx+1)*v.charW
	textRect := viewerLineContentRect(ui, th, gtx, ui.viewerMonospaceTypeface(), ui.viewerTextSize(), v.lineH, textX0, textX1)
	if fullRow {
		textRect = viewerLineSelectionRect(v.lineH, textX0, textX1)
	}
	if !textRect.Empty() {
		paint.FillShape(gtx.Ops, textSel, clip.Rect(textRect).Op())
	}
}

func (ui *UI) drawHexScrollbar(gtx layout.Context, st *fileViewerState) {
	v := st.hex
	if v == nil || v.trackRect.Dx() <= 0 || v.trackRect.Dy() <= 0 || v.thumbRect.Dy() <= 0 {
		return
	}
	paintViewerScrollbar(gtx, ui.fileViewerTheme(), v.trackRect, v.thumbRect, v.hoverTrack, v.hoverTrack, v.dragging)
}

func (ui *UI) drawHexScrollTooltip(gtx layout.Context, th *material.Theme, st *fileViewerState) {
	v := st.hex
	if v == nil || !v.dragging {
		return
	}
	total := v.totalLines()
	if total < 1 {
		total = 1
	}
	line := v.dragTop + 1
	if line < 1 {
		line = 1
	}
	if line > total {
		line = total
	}
	maxTop := total - int64(v.visibleLines)
	percent := 0.0
	if maxTop > 0 {
		percent = float64(v.dragTop) * 100 / float64(maxTop)
	}
	offset := v.dragTop * int64(v.bytesPerLine)
	msg := fmt.Sprintf("~ 0x%X  line %d/%d (%.1f%%)", offset, line, total, percent)
	theme := ui.fileViewerTheme()

	gap := gtx.Dp(unit.Dp(streamTooltipGapDp))
	if gap < 1 {
		gap = 1
	}
	edgeInset := gtx.Dp(unit.Dp(fileViewerTooltipEdgeInsetDp))
	if edgeInset < 2 {
		edgeInset = 2
	}
	maxBoxW := gtx.Constraints.Max.X - edgeInset*2
	if maxBoxW < 1 {
		return
	}
	box := ui.measureHexScrollTooltipBox(th, gtx, msg, maxBoxW)
	boxW, boxH := box.X, box.Y
	x := v.trackRect.Min.X - gap - boxW
	if x < edgeInset {
		x = edgeInset
	}
	y := v.thumbRect.Min.Y + v.thumbRect.Dy()/2 - boxH/2
	if y < edgeInset {
		y = edgeInset
	}
	maxY := gtx.Constraints.Max.Y - boxH - edgeInset
	if maxY < edgeInset {
		maxY = edgeInset
	}
	if y > maxY {
		y = maxY
	}

	cgtx := gtx
	cgtx.Constraints = layout.Exact(image.Pt(boxW, boxH))
	stack := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	_ = fillRoundedBox(
		cgtx,
		cgtx.Dp(unit.Dp(6)),
		theme.TooltipBg,
		theme.TooltipBorder,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Left:   unit.Dp(streamTooltipInsetXDp),
				Right:  unit.Dp(streamTooltipInsetXDp),
				Top:    unit.Dp(streamTooltipInsetYDp),
				Bottom: unit.Dp(streamTooltipInsetYDp),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := ui.hexScrollTooltipLabel(th, msg)
				lbl.Color = theme.TooltipText
				return layoutVCenteredLabel(gtx, lbl)
			})
		},
	)
	stack.Pop()
}

func (ui *UI) measureHexScrollTooltipBox(th *material.Theme, gtx layout.Context, msg string, maxBoxW int) image.Point {
	return ui.measureStreamOutputTooltipBox(th, gtx, msg, maxBoxW)
}

func (ui *UI) hexScrollTooltipLabel(th *material.Theme, msg string) material.LabelStyle {
	return ui.streamOutputTooltipLabel(th, msg)
}

func (ui *UI) applyHexViewerCursor(gtx layout.Context, st *fileViewerState) {
	v := st.hex
	if v == nil {
		return
	}
	if v.dragging {
		pointer.CursorGrabbing.Add(gtx.Ops)
		return
	}
	if v.hoverTrack {
		defer clip.Rect(v.trackRect).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		return
	}
	body := v.hexRect.Union(v.textRect)
	if body.Dx() > 0 && body.Dy() > 0 {
		defer clip.Rect(body).Push(gtx.Ops).Pop()
		pointer.CursorText.Add(gtx.Ops)
	}
}

func (ui *UI) handleHexViewerEvents(gtx layout.Context, st *fileViewerState) {
	v := st.hex
	if v == nil {
		return
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &v.pointerTag,
			Kinds:  pointer.Scroll | pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Move | pointer.Enter | pointer.Leave,
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
			if pe.Scroll.Y != 0 {
				v.scrollByDelta(pe.Scroll.Y)
				st.markUserBrowsing(gtx.Now)
				ui.startHexViewerLoad(st, false)
			}
		case pointer.Press:
			v.pointerOutside = false
			if pe.Buttons.Contain(pointer.ButtonSecondary) {
				st.openContextMenu(pos, gtx.Now)
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) && viewerPointInRect(pos, v.trackRect) {
				v.dragging = true
				v.dragID = pe.PointerID
				v.dragGrabY = v.verticalThumbGrabY(pos)
				v.dragTop = v.estimatedTopFromDragY(pos.Y, v.dragGrabY)
				v.updateScrollbarHover(pos)
				gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				st.markUserBrowsing(gtx.Now)
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) {
				if st.menuOpen {
					st.closeContextMenu()
				}
				if viewerPointInRect(pos, v.hexRect) || viewerPointInRect(pos, v.textRect) {
					v.syncVisualTop()
				}
				if byteOff, ok := hexByteAtPoint(v, pos); ok {
					if st.editMode {
						setHexViewerEditCaret(v, byteOff, viewerPointInRect(pos, v.textRect))
						st.editFocus = true
						gtx.Execute(key.FocusCmd{Tag: &v.editKeyTag})
					}
					v.selecting = true
					v.selectID = pe.PointerID
					v.dragAnchor = byteOff
					v.setSelectionRange(byteOff, 1)
					v.selectPos = pos
					v.clearCancelGrace()
					v.pointerOutside = false
					v.stopAutoScroll()
					gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				} else if viewerPointInRect(pos, v.hexRect) || viewerPointInRect(pos, v.textRect) {
					// Keep nearest-byte behavior for awkward edge presses.
					if st.editMode {
						setHexViewerEditCaret(v, v.fileSize-1, viewerPointInRect(pos, v.textRect))
						st.editFocus = true
						gtx.Execute(key.FocusCmd{Tag: &v.editKeyTag})
					}
					v.selecting = true
					v.selectID = pe.PointerID
					last := v.fileSize - 1
					if last < 0 {
						last = 0
					}
					v.dragAnchor = last
					v.setSelectionRange(last, 1)
					v.selectPos = pos
					v.clearCancelGrace()
					v.pointerOutside = false
					v.stopAutoScroll()
					gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				} else {
					v.clearSelection()
				}
				st.markUserBrowsing(gtx.Now)
			}
		case pointer.Drag:
			if v.dragging && pe.PointerID == v.dragID {
				v.dragTop = v.estimatedTopFromDragY(pos.Y, v.dragGrabY)
				v.clampTop()
				st.markUserBrowsing(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
			}
			if v.selecting && pe.PointerID == v.selectID {
				if !pe.Buttons.Contain(pointer.ButtonPrimary) {
					wasAutoScroll := v.autoScrollActive
					v.stopSelectionDrag()
					if wasAutoScroll {
						v.syncVisualTop()
					}
					break
				}
				v.selectPos = pos
				if byteOff, ok := hexSelectionByteAtPoint(v, pos); ok {
					v.setSelectionFromAnchor(v.dragAnchor, byteOff)
					if st.editMode {
						v.editCaret = v.clampByteOffset(byteOff)
						v.editNibble = 0
						v.editASCII = hexEditASCIIAtPoint(v, pos)
					}
				}
				v.updateAutoScroll(pos, gtx.Now)
				st.markUserBrowsing(gtx.Now)
			}
			if v.updateScrollbarHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Release:
			if v.dragging && pe.PointerID == v.dragID {
				v.topLine = v.dragTop
				v.dragging = false
				v.dragGrabY = 0
				v.clampTop()
				v.syncVisualTop()
				st.markUserBrowsing(gtx.Now)
			}
			if v.selecting && pe.PointerID == v.selectID {
				wasAutoScroll := v.autoScrollActive
				v.stopSelectionDrag()
				if wasAutoScroll {
					v.syncVisualTop()
				}
			}
			if v.updateScrollbarHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Cancel:
			v.pointerOutside = true
			if v.dragging && pe.PointerID == v.dragID {
				v.dragging = false
				v.dragGrabY = 0
			}
			if v.selecting && pe.PointerID == v.selectID {
				v.beginCancelGrace(gtx.Now)
			} else {
				v.clearCancelGrace()
			}
			if v.clearScrollbarHover() {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Move, pointer.Enter:
			if pe.Kind == pointer.Enter {
				v.pointerOutside = false
			}
			if v.selecting {
				if pe.PointerID == v.selectID && !pe.Buttons.Contain(pointer.ButtonPrimary) {
					wasAutoScroll := v.autoScrollActive
					v.stopSelectionDrag()
					if wasAutoScroll {
						v.syncVisualTop()
					}
					break
				}
				v.selectPos = pos
				v.updateAutoScroll(pos, gtx.Now)
				st.markUserBrowsing(gtx.Now)
			}
			if v.updateScrollbarHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Leave:
			v.pointerOutside = true
			if v.selecting {
				if pe.PointerID == v.selectID && !pe.Buttons.Contain(pointer.ButtonPrimary) {
					wasAutoScroll := v.autoScrollActive
					v.stopSelectionDrag()
					if wasAutoScroll {
						v.syncVisualTop()
					}
					break
				}
				v.selectPos = pos
				v.updateAutoScroll(pos, gtx.Now)
				st.markUserBrowsing(gtx.Now)
			}
			if v.clearScrollbarHover() {
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
}
