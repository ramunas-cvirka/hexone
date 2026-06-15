// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type streamOutputView struct {
	lines         []string
	lineOffsets   []int
	lineRunes     []int
	syntax        viewerSyntaxDocument
	totalBytes    int
	maxCols       int
	topLine       int
	visibleLines  int
	scrollCarry   float32
	hCol          int
	visualTop     float32
	visualReady   bool
	visualAt      time.Time
	displayTop    int
	displayY      int
	displayCount  int
	lastScrollDir int

	pointerTag struct{}

	trackRect   image.Rectangle
	thumbRect   image.Rectangle
	hTrackRect  image.Rectangle
	hThumbRect  image.Rectangle
	textRect    image.Rectangle
	charAdvance float32
	charW       int
	lineH       int
	textPad     int

	metricsReady  bool
	metricsTextSp unit.Sp
	metricsPxDp   float32
	metricsPxSp   float32

	hoverTrack  bool
	hoverThumb  bool
	hoverHTrack bool
	hoverHThumb bool

	dragging      bool
	dragID        pointer.ID
	dragTopLine   int
	dragGrabY     int
	hDragging     bool
	hDragID       pointer.ID
	hDragCol      int
	hDragGrabX    int
	selectingText bool
	selectID      pointer.ID
	selAnchor     int
	selHead       int
	selStart      int
	selLen        int
	selActive     bool

	autoScrollActive bool
	autoScrollDir    int
	autoScrollStep   int
	autoScrollAt     time.Time
	autoScrollStopAt time.Time
	cancelPending    bool
	cancelUntil      time.Time
	pointerOutside   bool
	selectPos        image.Point
	selectDirty      bool

	wrapEnabled bool

	lastPrimaryPressAt  time.Time
	lastPrimaryPressPos image.Point
	lastPrimaryPressed  bool
}

type streamSelectionState struct {
	active bool
	anchor int
	head   int
}

const (
	streamAutoScrollTick      = 50 * time.Millisecond
	streamAutoScrollStopIn    = 180 * time.Millisecond
	streamCancelGrace         = 320 * time.Millisecond
	streamAutoScrollNearPx    = 20
	streamAutoScrollMidPx     = 64
	streamDoubleClickDur      = 420 * time.Millisecond
	streamDoubleClickDist     = 6
	streamTooltipGapDp        = 6
	streamTooltipInsetXDp     = 6
	streamTooltipInsetYDp     = 3
	streamTooltipMinWidthDp   = 72
	streamTooltipMinHeightDp  = 18
	streamSmoothTick          = 16 * time.Millisecond
	streamSmoothTau           = 28 * time.Millisecond
	streamSmoothAutoTau       = 12 * time.Millisecond
	streamSmoothSnapEpsilon   = 0.02
	streamSmoothJumpLines     = 6
	streamSmoothAutoJumpLines = 24
	streamSmoothAutoMaxLag    = 1.25
)

func (v *streamOutputView) SetContent(raw string) {
	oldTotal := len(v.lines)
	oldTop := v.topLine
	v.lines = splitStreamLines(raw)
	v.clearSyntax()
	v.rebuildLineOffsets()
	newTotal := len(v.lines)
	if oldTotal > 1 && newTotal > 1 {
		ratio := float64(oldTop) / float64(oldTotal-1)
		v.topLine = int(ratio * float64(newTotal-1))
	} else {
		v.topLine = 0
	}
	v.clampTop()
	v.resetVisualTop()
}

func (v *streamOutputView) SetContentAfterTrim(raw, removedPrefix string, followBottom bool) {
	if v == nil {
		return
	}
	oldTop := v.topLine
	oldDragTop := v.dragTopLine
	oldVisual := v.visualTop
	visualReady := v.visualReady
	removedLines := strings.Count(removedPrefix, "\n")
	removedBytes := len(removedPrefix)

	v.lines = splitStreamLines(raw)
	v.clearSyntax()
	v.rebuildLineOffsets()
	v.shiftSelectionAfterPrefixTrim(removedBytes)
	if followBottom {
		v.scrollToBottom()
		return
	}

	v.topLine = oldTop - removedLines
	v.dragTopLine = oldDragTop - removedLines
	if visualReady {
		v.visualTop = oldVisual - float32(removedLines)
		v.visualReady = true
		v.visualAt = time.Time{}
	}
	v.clampTop()
	if !visualReady {
		v.resetVisualTop()
	} else {
		v.updateDisplayState()
	}
}

func (v *streamOutputView) maxTopLine() int {
	maxTop := len(v.lines) - v.visibleLines
	if maxTop < 0 {
		maxTop = 0
	}
	return maxTop
}

func (v *streamOutputView) syncVisualTop() {
	v.visualTop = float32(v.topLine)
	v.visualReady = true
	v.visualAt = time.Time{}
	v.updateDisplayState()
}

func (v *streamOutputView) resetVisualTop() {
	v.lastScrollDir = 0
	v.syncVisualTop()
}

func (v *streamOutputView) smoothJumpThreshold() float32 {
	limit := float32(streamSmoothJumpLines)
	if visible := float32(v.visibleLines) * 0.75; visible > limit {
		limit = visible
	}
	return limit
}

func (v *streamOutputView) autoScrollSmoothActive() bool {
	return v.autoScrollActive && v.selectingText
}

func (v *streamOutputView) smoothJumpLimit() float32 {
	if v.autoScrollSmoothActive() {
		return streamSmoothAutoJumpLines
	}
	return v.smoothJumpThreshold()
}

func (v *streamOutputView) smoothTau() time.Duration {
	if v.autoScrollSmoothActive() {
		return streamSmoothAutoTau
	}
	return streamSmoothTau
}

func (v *streamOutputView) clampAutoScrollVisualLag(target float32) {
	if !v.autoScrollSmoothActive() {
		return
	}
	if delta := target - v.visualTop; delta > streamSmoothAutoMaxLag {
		v.visualTop = target - streamSmoothAutoMaxLag
	} else if delta < -streamSmoothAutoMaxLag {
		v.visualTop = target + streamSmoothAutoMaxLag
	}
}

func (v *streamOutputView) updateDisplayState() {
	total := len(v.lines)
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
		if maxTop := v.maxTopLine(); v.displayTop > maxTop {
			v.displayTop = maxTop
		}
		v.displayY = 0
		v.displayCount = total - v.displayTop
		if v.displayCount < 1 {
			v.displayCount = 1
		}
		return
	}

	visualTop := float32(v.topLine)
	if v.visualReady {
		visualTop = v.visualTop
	}
	maxTop := float32(v.maxTopLine())
	if visualTop < 0 {
		visualTop = 0
	}
	if visualTop > maxTop {
		visualTop = maxTop
	}
	top := int(math.Floor(float64(visualTop)))
	frac := visualTop - float32(top)
	if frac < 0 {
		frac = 0
	}
	if frac > 0 && top >= int(maxTop) {
		top = int(maxTop)
		frac = 0
	}

	offsetY := 0
	if frac > 0 {
		offsetY = -int(math.Round(float64(frac * float32(v.lineH))))
		if offsetY <= -v.lineH {
			offsetY = 0
			top++
		}
	}
	if top < 0 {
		top = 0
	}
	if max := v.maxTopLine(); top > max {
		top = max
		offsetY = 0
	}

	count := total - top
	maxRows := v.visibleLines
	if offsetY < 0 && top+maxRows < total {
		maxRows++
	}
	if count > maxRows {
		count = maxRows
	}
	if count < 1 {
		count = 1
	}
	v.displayTop = top
	v.displayY = offsetY
	v.displayCount = count
}

func (v *streamOutputView) prepareVisualScroll(now time.Time, smooth bool) bool {
	target := float32(v.topLine)
	if !v.visualReady {
		v.visualTop = target
		v.visualReady = true
		v.visualAt = now
		v.updateDisplayState()
		return false
	}
	autoScrollSmooth := v.autoScrollSmoothActive()
	if !smooth || v.dragging || v.hDragging || (v.selectingText && !autoScrollSmooth) || v.cancelPending {
		v.visualTop = target
		v.visualAt = now
		v.updateDisplayState()
		return false
	}
	if float32Abs(target-v.visualTop) > v.smoothJumpLimit() {
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
		blend := float32(1 - math.Exp(-float64(dt)/float64(v.smoothTau())))
		v.visualTop += (target - v.visualTop) * clamp01(blend)
	}
	v.clampAutoScrollVisualLag(target)
	v.visualAt = now
	if float32Abs(target-v.visualTop) < streamSmoothSnapEpsilon {
		v.visualTop = target
		v.updateDisplayState()
		return false
	}
	v.updateDisplayState()
	return true
}

func (v *streamOutputView) ensureTextMetrics(ui *UI, th *material.Theme, gtx layout.Context) {
	textSize := ui.viewerTextSize()
	if v.metricsReady &&
		v.metricsTextSp == textSize &&
		v.metricsPxDp == gtx.Metric.PxPerDp &&
		v.metricsPxSp == gtx.Metric.PxPerSp &&
		v.charAdvance > 0 &&
		v.charW > 0 &&
		v.lineH > 0 {
		return
	}
	lineHeight := measureStreamLineHeight(ui, th, gtx)
	if lineHeight < 1 {
		lineHeight = 1
	}
	charAdvance := measureStreamCharAdvance(ui, th, gtx)
	if charAdvance <= 0 {
		charAdvance = 1
	}
	charW := int(math.Ceil(float64(charAdvance)))
	if charW < 1 {
		charW = 1
	}
	v.lineH = lineHeight
	v.charAdvance = charAdvance
	v.charW = charW
	v.metricsReady = true
	v.metricsTextSp = textSize
	v.metricsPxDp = gtx.Metric.PxPerDp
	v.metricsPxSp = gtx.Metric.PxPerSp
}

func (v *streamOutputView) Append(chunk string) {
	if chunk == "" {
		return
	}
	if len(v.lines) == 0 {
		v.lines = []string{""}
		v.rebuildLineOffsets()
	}
	if len(v.lineOffsets) != len(v.lines) {
		v.rebuildLineOffsets()
	}
	parts := strings.Split(chunk, "\n")
	last := len(v.lines) - 1
	v.lines[last] += parts[0]
	updatedRunes := utf8.RuneCountInString(v.lines[last])
	if len(v.lineRunes) == len(v.lines) {
		v.lineRunes[last] = updatedRunes
		if updatedRunes > v.maxCols {
			v.maxCols = updatedRunes
		}
	}
	v.totalBytes += len(parts[0])
	for _, p := range parts[1:] {
		v.totalBytes++
		lineStart := v.totalBytes
		v.lines = append(v.lines, p)
		v.lineOffsets = append(v.lineOffsets, lineStart)
		runes := utf8.RuneCountInString(p)
		v.lineRunes = append(v.lineRunes, runes)
		if runes > v.maxCols {
			v.maxCols = runes
		}
		v.totalBytes += len(p)
	}
	v.clampTop()
	v.updateDisplayState()
}

func (v *streamOutputView) nearBottom() bool {
	if len(v.lines) == 0 {
		return true
	}
	vis := v.visibleLines
	if vis <= 0 {
		return true
	}
	maxTop := len(v.lines) - vis
	if maxTop < 0 {
		maxTop = 0
	}
	return maxTop-v.topLine <= 1
}

func (v *streamOutputView) scrollToBottom() {
	maxTop := len(v.lines) - v.visibleLines
	if maxTop < 0 {
		maxTop = 0
	}
	v.topLine = maxTop
	v.syncVisualTop()
}

func (v *streamOutputView) clampTop() {
	if v.topLine < 0 {
		v.topLine = 0
	}
	maxTop := v.maxTopLine()
	if v.topLine > maxTop {
		v.topLine = maxTop
	}
	if v.dragTopLine < 0 {
		v.dragTopLine = 0
	}
	if v.dragTopLine > maxTop {
		v.dragTopLine = maxTop
	}
	if v.visualReady {
		if v.visualTop < 0 {
			v.visualTop = 0
		}
		if v.visualTop > float32(maxTop) {
			v.visualTop = float32(maxTop)
		}
	}
	v.clampSelection()
}

func (v *streamOutputView) applyVerticalScrollbarDrag(top int) {
	if v == nil {
		return
	}
	v.dragTopLine = top
	v.topLine = top
	v.clampTop()
	v.syncVisualTop()
}

func (v *streamOutputView) updateScrollbarHover(pos image.Point) bool {
	if v == nil {
		return false
	}
	oldTrack, oldThumb := v.hoverTrack, v.hoverThumb
	oldHTrack, oldHThumb := v.hoverHTrack, v.hoverHThumb
	v.hoverTrack = viewerPointInRect(pos, v.trackRect)
	v.hoverThumb = viewerPointInRect(pos, v.thumbRect)
	v.hoverHTrack = viewerPointInRect(pos, v.hTrackRect)
	v.hoverHThumb = viewerPointInRect(pos, v.hThumbRect)
	return oldTrack != v.hoverTrack ||
		oldThumb != v.hoverThumb ||
		oldHTrack != v.hoverHTrack ||
		oldHThumb != v.hoverHThumb
}

func (v *streamOutputView) clearScrollbarHover() bool {
	if v == nil {
		return false
	}
	changed := v.hoverTrack || v.hoverThumb || v.hoverHTrack || v.hoverHThumb
	v.hoverTrack = false
	v.hoverThumb = false
	v.hoverHTrack = false
	v.hoverHThumb = false
	return changed
}

func (v *streamOutputView) clampSelection() {
	if !v.selActive {
		return
	}
	v.selAnchor = v.clampOffset(v.selAnchor)
	v.selHead = v.clampOffset(v.selHead)
	v.updateSelectionRange()
}

func (v *streamOutputView) shiftSelectionAfterPrefixTrim(removedBytes int) {
	if removedBytes <= 0 || !v.selActive {
		return
	}
	shift := func(offset int) int {
		if offset <= removedBytes {
			return 0
		}
		return offset - removedBytes
	}
	v.selAnchor = shift(v.selAnchor)
	v.selHead = shift(v.selHead)
	v.updateSelectionRange()
}

func (v *streamOutputView) rebuildLineOffsets() {
	if len(v.lines) == 0 {
		v.lines = []string{""}
	}
	v.lineOffsets = make([]int, len(v.lines))
	v.lineRunes = make([]int, len(v.lines))
	offset := 0
	maxCols := 0
	for i, line := range v.lines {
		v.lineOffsets[i] = offset
		runes := utf8.RuneCountInString(line)
		v.lineRunes[i] = runes
		if runes > maxCols {
			maxCols = runes
		}
		offset += len(line)
		if i+1 < len(v.lines) {
			offset++
		}
	}
	v.totalBytes = offset
	v.maxCols = maxCols
}

func (v *streamOutputView) clampOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	if offset > v.totalBytes {
		return v.totalBytes
	}
	return offset
}

func (v *streamOutputView) updateSelectionRange() {
	start := v.selAnchor
	end := v.selHead
	if end < start {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > v.totalBytes {
		end = v.totalBytes
	}
	v.selStart = start
	v.selLen = end - start
}

func (v *streamOutputView) beginSelection(offset int) {
	offset = v.clampOffset(offset)
	v.selAnchor = offset
	v.selHead = offset
	v.selStart = offset
	v.selLen = 0
	v.selActive = true
	v.selectDirty = false
	v.cancelPending = false
	v.cancelUntil = time.Time{}
	v.pointerOutside = false
}

func (v *streamOutputView) updateSelection(offset int) {
	if !v.selActive {
		v.beginSelection(offset)
		return
	}
	v.selHead = v.clampOffset(offset)
	v.updateSelectionRange()
}

func (v *streamOutputView) selectAll() {
	if len(v.lines) == 0 {
		v.clearSelection()
		return
	}
	v.selActive = true
	v.selAnchor = 0
	v.selHead = v.totalBytes
	v.updateSelectionRange()
	v.selectDirty = false
}

func (v *streamOutputView) hasSelection() bool {
	return v.selActive && v.selLen > 0
}

func (v *streamOutputView) selectionState() streamSelectionState {
	if v == nil || !v.selActive {
		return streamSelectionState{}
	}
	return streamSelectionState{
		active: true,
		anchor: v.selAnchor,
		head:   v.selHead,
	}
}

func (v *streamOutputView) clearSelection() {
	v.selActive = false
	v.selAnchor = 0
	v.selHead = 0
	v.selStart = 0
	v.selLen = 0
	v.selectDirty = false
	v.cancelPending = false
	v.cancelUntil = time.Time{}
	v.pointerOutside = false
	v.stopAutoScroll()
}

func (v *streamOutputView) restoreSelectionState(state streamSelectionState) {
	if v == nil {
		return
	}
	if !state.active {
		v.clearSelection()
		return
	}
	v.selActive = true
	v.selAnchor = v.clampOffset(state.anchor)
	v.selHead = v.clampOffset(state.head)
	v.updateSelectionRange()
	v.selectingText = false
	v.selectID = 0
	v.selectDirty = false
	v.clearCancelGrace()
	v.pointerOutside = false
	v.stopAutoScroll()
}

func (v *streamOutputView) stopTextSelectionDrag() {
	v.selectingText = false
	v.selectID = 0
	v.selectDirty = false
	v.clearCancelGrace()
	v.pointerOutside = false
	v.stopAutoScroll()
}

func (v *streamOutputView) selectionBounds() (int, int, bool) {
	if !v.hasSelection() {
		return 0, 0, false
	}
	start := v.selStart
	end := start + v.selLen
	return start, end, true
}

func (v *streamOutputView) stopAutoScroll() {
	v.autoScrollActive = false
	v.autoScrollDir = 0
	v.autoScrollStep = 0
	v.autoScrollAt = time.Time{}
	v.autoScrollStopAt = time.Time{}
}

func (v *streamOutputView) beginCancelGrace(now time.Time) {
	v.cancelPending = true
	v.cancelUntil = now.Add(streamCancelGrace)
}

func (v *streamOutputView) clearCancelGrace() {
	v.cancelPending = false
	v.cancelUntil = time.Time{}
}

func (v *streamOutputView) expireCancelGrace(now time.Time) bool {
	if v.autoScrollActive && v.selectingText {
		// While active autoscroll is running, cancel can be a transient outside
		// state. Do not terminate selection on timeout.
		return false
	}
	if !v.cancelPending || v.cancelUntil.IsZero() || now.Before(v.cancelUntil) {
		return false
	}
	v.clearCancelGrace()
	v.selectingText = false
	v.selectID = 0
	v.selectDirty = false
	v.pointerOutside = false
	v.stopAutoScroll()
	return true
}

func (v *streamOutputView) autoScrollParams(pos image.Point) (int, int) {
	if len(v.lines) == 0 || v.lineH <= 0 {
		return 0, 0
	}
	rendered := v.renderedLineCount()
	if rendered < 1 {
		return 0, 0
	}
	top := v.textRect.Min.Y
	bottom := v.textRect.Min.Y + rendered*v.lineH
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

func (v *streamOutputView) updateAutoScroll(pos image.Point, now time.Time) {
	if !v.selectingText {
		v.stopAutoScroll()
		return
	}
	if viewerPointInRect(pos, v.textRect) {
		v.pointerOutside = false
		v.selectPos = pos
		v.stopAutoScroll()
		return
	}
	dir, step := v.autoScrollParams(pos)
	if v.pointerOutside && v.autoScrollActive && dir == 0 {
		// Pointer movement outside the app can report unstable neutral
		// coordinates. Preserve the last active edge/step until re-entry or a
		// clearly outside position arrives.
		return
	}
	v.selectPos = pos
	if dir == 0 {
		v.stopAutoScroll()
		return
	}
	v.autoScrollStopAt = time.Time{}
	prevDir := v.autoScrollDir
	prevStep := v.autoScrollStep
	wasActive := v.autoScrollActive
	v.autoScrollActive = true
	v.autoScrollDir = dir
	v.autoScrollStep = step
	if !wasActive || prevDir != dir || prevStep != step {
		// React immediately when user changes drag side/distance.
		v.autoScrollAt = now
	} else if v.autoScrollAt.IsZero() {
		v.autoScrollAt = now.Add(streamAutoScrollTick)
	}
}

func (v *streamOutputView) runAutoScroll(now time.Time) bool {
	if !v.autoScrollActive || !v.selectingText || v.autoScrollDir == 0 || v.autoScrollStep <= 0 {
		return false
	}
	if now.Before(v.autoScrollAt) {
		return false
	}
	before := v.topLine
	v.scrollByLines(v.autoScrollDir * v.autoScrollStep)
	if v.selActive {
		v.updateSelection(v.textOffsetFromPoint(v.selectPos))
	}
	if v.topLine == before {
		// Hit top/bottom and can't continue.
		v.stopAutoScroll()
		return false
	}
	v.autoScrollAt = now.Add(streamAutoScrollTick)
	return true
}

func (v *streamOutputView) applyPendingSelection() bool {
	if !v.selectingText || !v.selActive || !v.selectDirty {
		return false
	}
	beforeStart, beforeLen := v.selStart, v.selLen
	v.updateSelection(v.textOffsetFromPoint(v.selectPos))
	v.selectDirty = false
	return beforeStart != v.selStart || beforeLen != v.selLen
}

func (v *streamOutputView) selectedText() string {
	start, end, ok := v.selectionBounds()
	if !ok || len(v.lines) == 0 {
		return ""
	}
	raw := strings.Join(v.lines, "\n")
	if start < 0 {
		start = 0
	}
	if end > len(raw) {
		end = len(raw)
	}
	if end <= start {
		return ""
	}
	return raw[start:end]
}

func (v *streamOutputView) selectionColsForLine(line int) (int, int, bool) {
	start, end, ok := v.selectionBounds()
	if !ok {
		return 0, 0, false
	}
	return v.rangeColsForLine(line, start, end)
}

func (v *streamOutputView) rangeColsForLine(line, start, end int) (int, int, bool) {
	if line < 0 || line >= len(v.lines) || len(v.lineOffsets) != len(v.lines) {
		return 0, 0, false
	}
	lineText := v.lines[line]
	lineStart := v.lineOffsets[line]
	lineEnd := lineStart + len(lineText)
	if end <= lineStart || start >= lineEnd {
		return 0, 0, false
	}
	fromByte := 0
	if start > lineStart {
		fromByte = start - lineStart
	}
	toByte := len(lineText)
	if end < lineEnd {
		toByte = end - lineStart
	}
	if fromByte < 0 {
		fromByte = 0
	}
	if toByte > len(lineText) {
		toByte = len(lineText)
	}
	if toByte < fromByte {
		toByte = fromByte
	}
	from := runeIndexAtByte(lineText, fromByte)
	to := runeIndexAtByte(lineText, toByte)
	if to <= from {
		return 0, 0, false
	}
	return from, to, true
}

func (v *streamOutputView) lineByteStart(line int) int {
	if line < 0 {
		line = 0
	}
	if line >= len(v.lines) {
		line = len(v.lines) - 1
	}
	if line < 0 {
		return 0
	}
	if len(v.lineOffsets) == len(v.lines) {
		return v.lineOffsets[line]
	}
	offset := 0
	for i := 0; i < line && i < len(v.lines); i++ {
		offset += len(v.lines[i])
		if i+1 < len(v.lines) {
			offset++
		}
	}
	return offset
}

func (v *streamOutputView) lineByteEnd(line int) int {
	start := v.lineByteStart(line)
	if line < 0 || line >= len(v.lines) {
		return start
	}
	return start + len(v.lines[line])
}

func (v *streamOutputView) renderedLineCount() int {
	if v.displayCount > 0 {
		return v.displayCount
	}
	if len(v.lines) == 0 {
		return 0
	}
	n := len(v.lines) - v.topLine
	if n < 0 {
		n = 0
	}
	maxRows := v.visibleLines
	if maxRows < 1 {
		maxRows = 1
	}
	if n > maxRows {
		n = maxRows
	}
	return n
}

func (v *streamOutputView) rowFromPointY(y int) (int, bool) {
	rendered := v.renderedLineCount()
	if rendered < 1 {
		return 0, false
	}
	if v.lineH <= 0 {
		return 0, false
	}
	topY := v.textRect.Min.Y + v.displayY
	if y < topY {
		return 0, false
	}
	relY := y - topY
	maxY := rendered * v.lineH
	if relY < 0 || relY >= maxY {
		return 0, false
	}
	row := relY / v.lineH
	if row < 0 {
		row = 0
	}
	if row >= rendered {
		row = rendered - 1
	}
	return row, true
}

func (v *streamOutputView) pointOverSelectableText(pos image.Point) bool {
	if len(v.lines) == 0 || v.charAdvance <= 0 || !viewerPointInRect(pos, v.textRect) {
		return false
	}
	row, ok := v.rowFromPointY(pos.Y)
	if !ok {
		return false
	}
	line := v.displayTop + row
	if line < 0 || line >= len(v.lines) {
		return false
	}
	x := pos.X - v.textRect.Min.X - v.textPad
	if x < 0 {
		return false
	}
	maxCol := v.lineCols(line)
	if !v.wrapEnabled {
		maxCol -= v.hCol
	}
	if maxCol <= 0 {
		return false
	}
	return x < v.colOffsetPx(maxCol)
}

func (v *streamOutputView) textOffsetFromPoint(pos image.Point) int {
	if len(v.lines) == 0 {
		return 0
	}
	rendered := v.renderedLineCount()
	if rendered > 0 {
		firstLine := v.displayTop
		if firstLine < 0 {
			firstLine = 0
		}
		if firstLine >= len(v.lines) {
			firstLine = len(v.lines) - 1
		}
		renderedTop := v.textRect.Min.Y + v.displayY
		renderedBottom := renderedTop + rendered*v.lineH
		if pos.Y < renderedTop {
			return v.clampOffset(v.lineByteStart(firstLine))
		}
		if pos.Y >= renderedBottom {
			lastLine := v.displayTop + rendered - 1
			if lastLine < 0 {
				lastLine = 0
			}
			if lastLine >= len(v.lines) {
				lastLine = len(v.lines) - 1
			}
			return v.clampOffset(v.lineByteEnd(lastLine))
		}
	}

	row, ok := v.rowFromPointY(pos.Y)
	if !ok {
		row = 0
		if row < 0 {
			row = 0
		}
	}
	line := v.displayTop + row
	if line < 0 {
		line = 0
	}
	if line >= len(v.lines) {
		line = len(v.lines) - 1
	}
	x := pos.X - v.textRect.Min.X - v.textPad
	col := 0
	if x > 0 {
		col = v.colFromX(x)
	}
	if !v.wrapEnabled {
		col += v.hCol
	}
	maxCol := v.lineCols(line)
	if col < 0 {
		col = 0
	}
	if col > maxCol {
		col = maxCol
	}
	base := 0
	if len(v.lineOffsets) == len(v.lines) {
		base = v.lineOffsets[line]
	}
	offset := base + byteIndexAtRune(v.lines[line], col)
	return v.clampOffset(offset)
}

func (v *streamOutputView) registerPrimaryPress(now time.Time, pos image.Point) bool {
	if !v.lastPrimaryPressed {
		v.lastPrimaryPressed = true
		v.lastPrimaryPressAt = now
		v.lastPrimaryPressPos = pos
		return false
	}
	dt := now.Sub(v.lastPrimaryPressAt)
	dx := pos.X - v.lastPrimaryPressPos.X
	if dx < 0 {
		dx = -dx
	}
	dy := pos.Y - v.lastPrimaryPressPos.Y
	if dy < 0 {
		dy = -dy
	}
	v.lastPrimaryPressAt = now
	v.lastPrimaryPressPos = pos
	if dt <= streamDoubleClickDur && dx <= streamDoubleClickDist && dy <= streamDoubleClickDist {
		v.lastPrimaryPressed = false
		return true
	}
	return false
}

func (v *streamOutputView) selectWordAtOffset(offset int, wordRE *regexp.Regexp) bool {
	if v == nil || wordRE == nil || len(v.lines) == 0 {
		return false
	}
	if len(v.lineOffsets) != len(v.lines) {
		v.rebuildLineOffsets()
	}
	offset = v.clampOffset(offset)
	line, local, ok := v.lineForOffset(offset)
	if !ok {
		return false
	}
	lineText := v.lines[line]
	if lineText == "" {
		return false
	}
	probe := local
	if probe >= len(lineText) {
		probe = len(lineText) - 1
	}
	if probe < 0 {
		probe = 0
	}

	var targetStart, targetEnd int
	for _, loc := range wordRE.FindAllStringIndex(lineText, -1) {
		if len(loc) != 2 {
			continue
		}
		if probe >= loc[0] && probe < loc[1] {
			targetStart, targetEnd = loc[0], loc[1]
			break
		}
		if probe > 0 && probe-1 >= loc[0] && probe-1 < loc[1] {
			targetStart, targetEnd = loc[0], loc[1]
			break
		}
	}
	if targetEnd <= targetStart {
		return false
	}
	lineStart := v.lineByteStart(line)
	v.selActive = true
	v.selAnchor = lineStart + targetStart
	v.selHead = lineStart + targetEnd
	v.updateSelectionRange()
	v.selectingText = false
	v.selectDirty = false
	v.stopAutoScroll()
	v.clearCancelGrace()
	v.pointerOutside = false
	return true
}

func (v *streamOutputView) lineForOffset(offset int) (line int, local int, ok bool) {
	if v == nil || len(v.lines) == 0 {
		return 0, 0, false
	}
	if len(v.lineOffsets) != len(v.lines) {
		v.rebuildLineOffsets()
	}
	offset = v.clampOffset(offset)
	idx := sort.Search(len(v.lineOffsets), func(i int) bool {
		return v.lineOffsets[i] > offset
	}) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(v.lines) {
		idx = len(v.lines) - 1
	}
	local = offset - v.lineOffsets[idx]
	if local < 0 {
		local = 0
	}
	if local > len(v.lines[idx]) {
		local = len(v.lines[idx])
	}
	return idx, local, true
}

func byteIndexAtRune(s string, runeIdx int) int {
	if runeIdx <= 0 {
		return 0
	}
	i := 0
	for bytePos := range s {
		if i == runeIdx {
			return bytePos
		}
		i++
	}
	return len(s)
}

func runeIndexAtByte(s string, byteIdx int) int {
	if byteIdx <= 0 {
		return 0
	}
	if byteIdx >= len(s) {
		return utf8.RuneCountInString(s)
	}
	runes := 0
	for i := range s {
		if i >= byteIdx {
			break
		}
		runes++
	}
	return runes
}

func measureTypefaceCharWidth(ui *UI, th *material.Theme, gtx layout.Context, face font.Typeface) int {
	return measureTypefaceCharWidthAt(th, gtx, face, ui.viewerTextSize())
}

func measureTypefaceCharWidthAt(th *material.Theme, gtx layout.Context, face font.Typeface, size unit.Sp) int {
	size = normalizeUIFontSize(size)
	fallback := int(float32(gtx.Sp(size))*0.62 + 0.5)
	if fallback < 5 {
		fallback = 5
	}
	if th == nil || th.Shaper == nil {
		return fallback
	}

	measureLabelWidth := func(sample string) int {
		lbl := material.Body2(th, sample)
		lbl.Font.Typeface = face
		lbl.Font.Weight = font.Normal
		lbl.TextSize = size
		lbl.MaxLines = 1
		lbl.Truncator = ""
		return measureLabelUnconstrained(gtx, lbl).Size.X
	}

	cw := fallback
	if single := measureLabelWidth("0"); single > cw {
		cw = single
	}
	if double := measureLabelWidth("00"); double > 0 {
		doubleCell := (double + 1) / 2
		if doubleCell > cw {
			cw = doubleCell
		}
	}
	if wide := measureLabelWidth("M"); wide > cw {
		cw = wide
	}
	// Leave a small safety margin so exact-width hex cells don't clip glyph
	// bounds at larger font sizes.
	return cw + 1
}

func measureStreamCharWidth(ui *UI, th *material.Theme, gtx layout.Context) int {
	return measureTypefaceCharWidth(ui, th, gtx, ui.viewerTypeface())
}

func measureTypefaceCharAdvance(ui *UI, th *material.Theme, gtx layout.Context, face font.Typeface) float32 {
	return measureTypefaceCharAdvanceAt(th, gtx, face, ui.viewerTextSize())
}

func measureTypefaceCharAdvanceAt(th *material.Theme, gtx layout.Context, face font.Typeface, size unit.Sp) float32 {
	size = normalizeUIFontSize(size)
	fallback := float32(gtx.Sp(size)) * 0.62
	if fallback < 5 {
		fallback = 5
	}
	if th == nil || th.Shaper == nil {
		return fallback
	}

	measureLabelWidth := func(sample string) int {
		lbl := material.Body2(th, sample)
		lbl.Font.Typeface = face
		lbl.Font.Weight = font.Normal
		lbl.TextSize = size
		lbl.MaxLines = 1
		lbl.Truncator = ""
		return measureLabelUnconstrained(gtx, lbl).Size.X
	}

	const sampleCount = 32
	if width := measureLabelWidth(strings.Repeat("0", sampleCount)); width > 0 {
		return float32(width) / sampleCount
	}
	if width := measureLabelWidth(strings.Repeat("M", sampleCount)); width > 0 {
		return float32(width) / sampleCount
	}
	return fallback
}

func measureStreamCharAdvance(ui *UI, th *material.Theme, gtx layout.Context) float32 {
	return measureTypefaceCharAdvance(ui, th, gtx, ui.viewerTypeface())
}

func measureTypefaceLineHeight(ui *UI, th *material.Theme, gtx layout.Context, face font.Typeface) int {
	return measureTypefaceLineHeightAt(th, gtx, face, ui.viewerTextSize())
}

func measureTypefaceLineHeightAt(th *material.Theme, gtx layout.Context, face font.Typeface, size unit.Sp) int {
	size = normalizeUIFontSize(size)
	fallback := gtx.Sp(size) + gtx.Dp(unit.Dp(3))
	if fallback < 12 {
		fallback = 12
	}
	if th == nil || th.Shaper == nil {
		return fallback
	}
	lbl := material.Body2(th, "Mg")
	lbl.Font.Typeface = face
	lbl.Font.Weight = font.Normal
	lbl.TextSize = size
	lbl.MaxLines = 1
	lbl.Truncator = ""
	h := measureLabelUnconstrained(gtx, lbl).Size.Y + gtx.Dp(unit.Dp(2))
	if h < 12 {
		return 12
	}
	return h
}

func measureStreamLineHeight(ui *UI, th *material.Theme, gtx layout.Context) int {
	return measureTypefaceLineHeight(ui, th, gtx, ui.viewerTypeface())
}

func (v *streamOutputView) scrollByLines(lines int) {
	if lines == 0 {
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

func (v *streamOutputView) scrollByDelta(delta float32) {
	if delta == 0 {
		return
	}
	// Accelerate wheel/trackpad scrolling in the viewer so it matches
	// file-list navigation better on long outputs.
	delta *= 2.25
	if delta > 3 {
		delta = 3
	} else if delta < -3 {
		delta = -3
	}
	if (delta > 0 && v.scrollCarry < 0) || (delta < 0 && v.scrollCarry > 0) {
		v.scrollCarry = 0
	}
	v.scrollCarry += delta

	steps := 0
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

func (v *streamOutputView) lineCols(line int) int {
	if line < 0 || line >= len(v.lines) {
		return 0
	}
	if len(v.lineRunes) == len(v.lines) {
		return v.lineRunes[line]
	}
	return utf8.RuneCountInString(v.lines[line])
}

func (v *streamOutputView) colOffsetPx(cols int) int {
	if cols == 0 || v.charAdvance <= 0 {
		return 0
	}
	return int(math.Round(float64(v.charAdvance) * float64(cols)))
}

func (v *streamOutputView) colFromX(x int) int {
	if x <= 0 || v.charAdvance <= 0 {
		return 0
	}
	return int(math.Floor(float64(x) / float64(v.charAdvance)))
}

func (v *streamOutputView) minCellWidthPx() int {
	if v.charW > 0 {
		return v.charW
	}
	if v.charAdvance > 0 {
		if w := int(math.Ceil(float64(v.charAdvance))); w > 0 {
			return w
		}
	}
	return 1
}

func (v *streamOutputView) visibleCols(textW int) int {
	if textW <= 0 || v.charAdvance <= 0 {
		return 1
	}
	cols := int(math.Floor(float64(textW-v.textPad) / float64(v.charAdvance)))
	if cols < 1 {
		cols = 1
	}
	return cols
}

func (v *streamOutputView) linePaintSpec(line string) (text string, offsetX int) {
	offsetX = v.textPad
	if !v.wrapEnabled {
		fromCol := v.hCol
		if fromCol < 0 {
			fromCol = 0
		}
		maxCol := utf8.RuneCountInString(line)
		if fromCol > maxCol {
			fromCol = maxCol
		}
		return line[byteIndexAtRune(line, fromCol):], offsetX
	}
	return line, offsetX
}

func (v *streamOutputView) maxHCol(textW int) int {
	if v.wrapEnabled {
		return 0
	}
	maxCols := v.maxCols
	visible := v.visibleCols(textW)
	maxH := maxCols - visible
	if maxH < 0 {
		maxH = 0
	}
	return maxH
}

func (v *streamOutputView) clampHCol(textW int) {
	maxH := v.maxHCol(textW)
	if v.hCol < 0 {
		v.hCol = 0
	}
	if v.hCol > maxH {
		v.hCol = maxH
	}
	if v.hDragCol < 0 {
		v.hDragCol = 0
	}
	if v.hDragCol > maxH {
		v.hDragCol = maxH
	}
}

func (v *streamOutputView) scrollHByDelta(delta float32, textW int) {
	if v.wrapEnabled || delta == 0 {
		return
	}
	delta *= 2.0
	if delta > 6 {
		delta = 6
	} else if delta < -6 {
		delta = -6
	}
	steps := int(delta)
	if steps == 0 {
		if delta > 0 {
			steps = 1
		} else if delta < 0 {
			steps = -1
		}
	}
	// Positive horizontal wheel movement should scroll right.
	v.hCol += steps
	v.clampHCol(textW)
}

func (v *streamOutputView) computeScrollbar(size image.Point, viewportH, scrollbarW int) {
	v.trackRect = image.Rectangle{}
	v.thumbRect = image.Rectangle{}
	if size.X <= 0 || viewportH <= 0 || scrollbarW <= 0 {
		return
	}
	total := len(v.lines)
	if total <= 0 {
		total = 1
	}
	visible := v.visibleLines
	if visible < 1 {
		visible = 1
	}
	if visible > total {
		visible = total
	}
	maxTop := total - visible
	if maxTop < 0 {
		maxTop = 0
	}

	trackX := size.X - scrollbarW
	if trackX < 0 {
		trackX = 0
	}
	v.trackRect = image.Rect(trackX, 0, size.X, viewportH)

	v.clampTop()
	topForThumb := float32(v.topLine)
	if v.dragging {
		topForThumb = float32(v.dragTopLine)
	} else if v.visualReady {
		topForThumb = v.visualTop
	}
	if topForThumb < 0 {
		topForThumb = 0
	}
	if topForThumb > float32(maxTop) {
		topForThumb = float32(maxTop)
	}

	v.thumbRect = viewerScrollbarThumbForScroll(v.trackRect, visible, total, float64(topForThumb), true)
}

func (v *streamOutputView) estimatedTopFromY(y int) int {
	return v.estimatedTopFromDragY(y, v.thumbRect.Dy()/2)
}

func (v *streamOutputView) estimatedTopFromDragY(y, grabY int) int {
	total := len(v.lines)
	if total <= 0 {
		return 0
	}
	visible := v.visibleLines
	if visible < 1 {
		visible = 1
	}
	maxTop := total - visible
	if maxTop <= 0 {
		return 0
	}
	track := v.trackRect
	thumb := v.thumbRect
	maxTravel := track.Dy() - thumb.Dy()
	if maxTravel <= 0 {
		return 0
	}
	if grabY < 0 {
		grabY = 0
	}
	if grabY > thumb.Dy() {
		grabY = thumb.Dy()
	}
	dragY := y - track.Min.Y - grabY
	if dragY < 0 {
		dragY = 0
	}
	if dragY > maxTravel {
		dragY = maxTravel
	}
	ratio := float32(dragY) / float32(maxTravel)
	top := int(ratio*float32(maxTop) + 0.5)
	if top < 0 {
		top = 0
	}
	if top > maxTop {
		top = maxTop
	}
	return top
}

func (v *streamOutputView) verticalThumbGrabY(pos image.Point) int {
	if thumb := v.thumbRect; thumb.Dy() > 0 && viewerPointInRect(pos, thumb) {
		return pos.Y - thumb.Min.Y
	}
	return v.thumbRect.Dy() / 2
}

func (v *streamOutputView) computeHorizontalScrollbar(size image.Point, barH int) {
	v.hTrackRect = image.Rectangle{}
	v.hThumbRect = image.Rectangle{}
	if v.wrapEnabled || barH <= 0 || size.X <= 0 || size.Y <= 0 || v.charW <= 0 {
		return
	}
	textW := v.textRect.Dx()
	if textW <= 0 {
		return
	}
	maxH := v.maxHCol(textW)
	if maxH <= 0 {
		return
	}
	y0 := v.textRect.Max.Y
	if y0 >= size.Y {
		return
	}
	y1 := y0 + barH
	if y1 > size.Y {
		y1 = size.Y
	}
	if y1 <= y0 {
		return
	}
	v.hTrackRect = image.Rect(0, y0, textW, y1)
	visible := v.visibleCols(textW)
	total := v.maxCols
	if total < 1 {
		total = 1
	}
	left := v.hCol
	if v.hDragging {
		left = v.hDragCol
	}
	if left < 0 {
		left = 0
	}
	if left > maxH {
		left = maxH
	}
	v.hThumbRect = viewerScrollbarThumbForScroll(v.hTrackRect, visible, total, float64(left), false)
}

func (v *streamOutputView) estimatedHColFromX(x int) int {
	return v.estimatedHColFromDragX(x, v.hThumbRect.Dx()/2)
}

func (v *streamOutputView) estimatedHColFromDragX(x, grabX int) int {
	track := v.hTrackRect
	thumb := v.hThumbRect
	textW := v.textRect.Dx()
	maxH := v.maxHCol(textW)
	if track.Dx() <= 0 || thumb.Dx() <= 0 || maxH <= 0 {
		return 0
	}
	maxTravel := track.Dx() - thumb.Dx()
	if maxTravel <= 0 {
		return 0
	}
	if grabX < 0 {
		grabX = 0
	}
	if grabX > thumb.Dx() {
		grabX = thumb.Dx()
	}
	dragX := x - track.Min.X - grabX
	if dragX < 0 {
		dragX = 0
	}
	if dragX > maxTravel {
		dragX = maxTravel
	}
	ratio := float32(dragX) / float32(maxTravel)
	col := int(ratio*float32(maxH) + 0.5)
	if col < 0 {
		col = 0
	}
	if col > maxH {
		col = maxH
	}
	return col
}

func (v *streamOutputView) horizontalThumbGrabX(pos image.Point) int {
	if thumb := v.hThumbRect; thumb.Dx() > 0 && viewerPointInRect(pos, thumb) {
		return pos.X - thumb.Min.X
	}
	return v.hThumbRect.Dx() / 2
}

func splitStreamLines(raw string) []string {
	if raw == "" {
		return []string{""}
	}
	return strings.Split(raw, "\n")
}

func (ui *UI) layoutStreamOutputView(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	v := &st.stream
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: size}
	}
	if len(v.lines) == 0 {
		v.lines = []string{""}
	}
	v.wrapEnabled = st.wrapEnabled

	v.ensureTextMetrics(ui, th, gtx)
	lineHeight := v.lineH

	scrollbarW := viewerScrollbarThickness(gtx, size.X)
	textW := size.X - scrollbarW
	if textW < 1 {
		textW = size.X
		scrollbarW = 0
	}
	v.textPad = gtx.Dp(unit.Dp(2))
	if reflowFileViewerBinaryPreview(st, v.visibleCols(textW)) {
		ui.refreshFileViewerFind(gtx.Now, true)
	}
	hbarH := 0
	if !v.wrapEnabled && v.maxHCol(textW) > 0 {
		hbarH = viewerScrollbarThickness(gtx, size.Y)
		if hbarH > size.Y/2 {
			hbarH = size.Y / 2
		}
	}
	textH := size.Y - hbarH
	if textH < 1 {
		textH = 1
		hbarH = 0
	}
	v.textRect = image.Rect(0, 0, textW, textH)
	v.visibleLines = textH / lineHeight
	if v.visibleLines < 1 {
		v.visibleLines = 1
	}
	v.clampTop()
	v.clampHCol(textW)
	v.clampSelection()

	v.prepareVisualScroll(gtx.Now, viewerSmoothScrolling(ui.fmCfg))
	v.computeScrollbar(size, textH, scrollbarW)
	v.computeHorizontalScrollbar(size, hbarH)
	ui.handleStreamOutputEvents(gtx, st)
	if v.expireCancelGrace(gtx.Now) {
		st.markUserBrowsing(gtx.Now)
	}
	if v.applyPendingSelection() {
		st.markUserBrowsing(gtx.Now)
	}
	if v.runAutoScroll(gtx.Now) {
		st.markUserBrowsing(gtx.Now)
	}
	animating := v.prepareVisualScroll(gtx.Now, viewerSmoothScrolling(ui.fmCfg))
	if v.autoScrollSmoothActive() && v.selActive {
		beforeStart, beforeLen := v.selStart, v.selLen
		v.updateSelection(v.textOffsetFromPoint(v.selectPos))
		if v.selStart != beforeStart || v.selLen != beforeLen {
			st.markUserBrowsing(gtx.Now)
		}
	}
	v.computeScrollbar(size, textH, scrollbarW)
	v.computeHorizontalScrollbar(size, hbarH)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(streamSmoothTick)})
	}
	if v.autoScrollActive && !v.autoScrollAt.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: v.autoScrollAt})
	}
	if v.cancelPending && !v.cancelUntil.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: v.cancelUntil})
	}

	ui.drawStreamOutputText(th, gtx, st, v.textRect.Dx(), lineHeight)
	ui.drawStreamOutputScrollbar(gtx, st)
	ui.drawStreamOutputTooltip(th, gtx, st)
	ui.applyStreamOutputCursor(gtx, st)

	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &v.pointerTag)
	pass.Pop()

	return layout.Dimensions{Size: size}
}

func (ui *UI) handleStreamOutputEvents(gtx layout.Context, st *fileViewerState) {
	if st == nil {
		return
	}
	v := &st.stream
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
		if pe.Kind == pointer.Press || pe.Kind == pointer.Drag || pe.Kind == pointer.Move || pe.Kind == pointer.Enter || pe.Kind == pointer.Scroll {
			v.clearCancelGrace()
		}
		pos := pe.Position.Round()
		switch pe.Kind {
		case pointer.Scroll:
			if pe.Scroll.X != 0 || pe.Scroll.Y != 0 {
				v.stopTextSelectionDrag()
			}
			if pe.Scroll.X != 0 {
				v.scrollHByDelta(pe.Scroll.X, v.textRect.Dx())
				st.markUserBrowsing(gtx.Now)
			}
			if pe.Scroll.Y != 0 {
				v.scrollByDelta(pe.Scroll.Y)
				st.markUserBrowsing(gtx.Now)
			}
		case pointer.Press:
			v.pointerOutside = false
			if pe.Buttons.Contain(pointer.ButtonSecondary) {
				st.openContextMenu(pos, gtx.Now)
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) && viewerPointInRect(pos, v.trackRect) {
				v.stopTextSelectionDrag()
				v.dragging = true
				v.dragID = pe.PointerID
				v.dragGrabY = v.verticalThumbGrabY(pos)
				v.applyVerticalScrollbarDrag(v.estimatedTopFromDragY(pos.Y, v.dragGrabY))
				gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				gtx.Execute(op.InvalidateCmd{})
				st.markUserBrowsing(gtx.Now)
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) && viewerPointInRect(pos, v.hTrackRect) {
				v.stopTextSelectionDrag()
				v.hDragging = true
				v.hDragID = pe.PointerID
				v.hDragGrabX = v.horizontalThumbGrabX(pos)
				v.hDragCol = v.estimatedHColFromDragX(pos.X, v.hDragGrabX)
				v.hCol = v.hDragCol
				v.clampHCol(v.textRect.Dx())
				gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				gtx.Execute(op.InvalidateCmd{})
				st.markUserBrowsing(gtx.Now)
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) {
				if st.menuOpen {
					st.closeContextMenu()
				}
				if viewerPointInRect(pos, v.textRect) {
					v.syncVisualTop()
					doubleClick := v.registerPrimaryPress(gtx.Now, pos)
					if doubleClick && v.selectWordAtOffset(v.textOffsetFromPoint(pos), st.wordSelectRE) {
						st.markUserBrowsing(gtx.Now)
						continue
					}
					v.selectingText = true
					v.selectID = pe.PointerID
					v.selectPos = pos
					v.beginSelection(v.textOffsetFromPoint(pos))
					v.updateAutoScroll(pos, gtx.Now)
					gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				} else {
					v.selectingText = false
					v.clearSelection()
				}
				st.markUserBrowsing(gtx.Now)
			}
		case pointer.Drag:
			if v.dragging && pe.PointerID == v.dragID {
				v.applyVerticalScrollbarDrag(v.estimatedTopFromDragY(pos.Y, v.dragGrabY))
				st.markUserBrowsing(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
			}
			if v.hDragging && pe.PointerID == v.hDragID {
				v.hDragCol = v.estimatedHColFromDragX(pos.X, v.hDragGrabX)
				v.hCol = v.hDragCol
				v.clampHCol(v.textRect.Dx())
				st.markUserBrowsing(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
			}
			if v.selectingText && pe.PointerID == v.selectID {
				if !pe.Buttons.Contain(pointer.ButtonPrimary) {
					wasAutoScroll := v.autoScrollActive
					v.stopTextSelectionDrag()
					if wasAutoScroll {
						v.syncVisualTop()
					}
					break
				}
				v.selectPos = pos
				v.selectDirty = true
				v.updateAutoScroll(pos, gtx.Now)
				st.markUserBrowsing(gtx.Now)
			}
			if v.updateScrollbarHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Release:
			if v.dragging && pe.PointerID == v.dragID {
				v.dragging = false
				v.dragGrabY = 0
				v.clampTop()
				v.syncVisualTop()
			}
			if v.hDragging && pe.PointerID == v.hDragID {
				v.hDragging = false
				v.hDragGrabX = 0
				v.clampHCol(v.textRect.Dx())
			}
			if v.selectingText && pe.PointerID == v.selectID {
				wasAutoScroll := v.autoScrollActive
				v.stopTextSelectionDrag()
				if wasAutoScroll {
					v.syncVisualTop()
				}
			}
			if v.updateScrollbarHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Cancel:
			v.pointerOutside = true
			if v.selectingText && pe.PointerID == v.selectID {
				// Don't abort immediately; cancellations can be transient while
				// dragging beyond window bounds.
				v.beginCancelGrace(gtx.Now)
			} else {
				v.clearCancelGrace()
			}
			if v.dragging && pe.PointerID == v.dragID {
				v.dragging = false
				v.dragGrabY = 0
				v.clampTop()
			}
			if v.hDragging && pe.PointerID == v.hDragID {
				v.hDragging = false
				v.hDragGrabX = 0
				v.clampHCol(v.textRect.Dx())
			}
			if v.clearScrollbarHover() {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Move, pointer.Enter:
			if pe.Kind == pointer.Enter {
				v.pointerOutside = false
			}
			if v.selectingText {
				if pe.PointerID == v.selectID && !pe.Buttons.Contain(pointer.ButtonPrimary) {
					wasAutoScroll := v.autoScrollActive
					v.stopTextSelectionDrag()
					if wasAutoScroll {
						v.syncVisualTop()
					}
					break
				}
				v.selectPos = pos
				v.selectDirty = true
				v.updateAutoScroll(pos, gtx.Now)
				st.markUserBrowsing(gtx.Now)
			}
			if v.updateScrollbarHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Leave:
			v.pointerOutside = true
			if v.selectingText {
				if pe.PointerID == v.selectID && !pe.Buttons.Contain(pointer.ButtonPrimary) {
					wasAutoScroll := v.autoScrollActive
					v.stopTextSelectionDrag()
					if wasAutoScroll {
						v.syncVisualTop()
					}
					break
				}
				v.selectPos = pos
				v.selectDirty = true
				v.updateAutoScroll(pos, gtx.Now)
				st.markUserBrowsing(gtx.Now)
			}
			if v.clearScrollbarHover() {
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
}

func (ui *UI) drawStreamOutputText(th *material.Theme, gtx layout.Context, st *fileViewerState, textW, lineHeight int) {
	if st == nil {
		return
	}
	v := &st.stream
	theme := ui.fileViewerTheme()
	findFill, _ := fileViewerFindHighlightColors(theme)
	if textW <= 0 || lineHeight <= 0 {
		return
	}
	lineFace := ui.viewerTypeface()
	lineSize := ui.viewerTextSize()
	textH := v.textRect.Dy()
	if textH <= 0 {
		return
	}
	textClip := clip.Rect(image.Rect(0, 0, textW, textH)).Push(gtx.Ops)
	defer textClip.Pop()

	start := v.displayTop
	if start < 0 {
		start = 0
	}
	end := start + v.renderedLineCount()
	if end > len(v.lines) {
		end = len(v.lines)
	}
	y := v.displayY
	for i := start; i < end; i++ {
		line := v.lines[i]
		lineGTX := gtx
		lineGTX.Constraints = layout.Constraints{
			Min: image.Pt(0, lineHeight),
			Max: image.Pt(1<<20, lineHeight),
		}
		lineDraw, textX := v.linePaintSpec(line)
		offset := op.Offset(image.Pt(textX, y)).Push(gtx.Ops)
		if from, to, ok := fileViewerFindColsForLine(st, i); ok {
			x0 := v.textPad + v.colOffsetPx(from-v.hCol)
			x1 := v.textPad + v.colOffsetPx(to-v.hCol)
			if x1 <= x0 {
				x1 = x0 + v.minCellWidthPx()
			}
			if x0 < textW {
				if x0 < 0 {
					x0 = 0
				}
				if x1 > textW {
					x1 = textW
				}
				if x1 > x0 {
					if findRect := viewerLineContentRect(ui, th, gtx, lineFace, lineSize, lineHeight, x0, x1); !findRect.Empty() {
						paint.FillShape(gtx.Ops, findFill, clip.Rect(findRect).Op())
					}
				}
			}
		}
		if from, to, ok := v.selectionColsForLine(i); ok {
			x0 := v.textPad + v.colOffsetPx(from-v.hCol)
			x1 := v.textPad + v.colOffsetPx(to-v.hCol)
			if x1 <= x0 {
				x1 = x0 + v.minCellWidthPx()
			}
			if x0 < textW {
				if x0 < 0 {
					x0 = 0
				}
				if x1 > textW {
					x1 = textW
				}
				if x1 > x0 {
					if selRect := viewerLineSelectionRect(lineHeight, x0, x1); !selRect.Empty() {
						paint.FillShape(gtx.Ops, theme.Selection, clip.Rect(selRect).Op())
					}
				}
			}
		}
		if spans, ok := v.syntaxLine(i); ok && len(spans) > 0 {
			ui.drawStreamOutputSyntaxLine(th, lineGTX, v, line, lineDraw, spans, v.hCol, textW, lineFace, lineSize, theme)
		} else {
			ui.drawStreamOutputPlainLine(th, lineGTX, lineDraw, lineFace, lineSize, theme.Text)
		}
		offset.Pop()
		y += lineHeight
		if y >= textH {
			break
		}
	}
}

func (ui *UI) drawStreamOutputPlainLine(th *material.Theme, gtx layout.Context, text string, face font.Typeface, size unit.Sp, textColor color.NRGBA) {
	_ = func(gtx layout.Context) layout.Dimensions {
		lbl := material.Body2(th, text)
		lbl.Font.Typeface = face
		lbl.Font.Weight = font.Normal
		lbl.TextSize = size
		lbl.Color = textColor
		lbl.MaxLines = 1
		lbl.Truncator = ""
		return layoutVCenteredLabel(gtx, lbl)
	}(gtx)
}

func (ui *UI) drawStreamOutputSyntaxLine(th *material.Theme, gtx layout.Context, v *streamOutputView, line, fallback string, spans []viewerSyntaxSpan, hCol, textW int, face font.Typeface, size unit.Sp, theme fileViewerTheme) {
	if len(spans) == 0 {
		ui.drawStreamOutputPlainLine(th, gtx, fallback, face, size, theme.Text)
		return
	}
	visibleCols := 0
	if v != nil {
		visibleCols = v.visibleCols(textW)
	}
	if visibleCols < 1 {
		visibleCols = 1
	}
	maxVisibleCol := hCol + visibleCols + 1
	drew := false
	for _, span := range spans {
		if span.colEnd <= hCol {
			continue
		}
		if span.colStart >= maxVisibleCol {
			break
		}
		visibleFrom := span.colStart
		if visibleFrom < hCol {
			visibleFrom = hCol
		}
		visibleTo := span.colEnd
		if visibleTo > maxVisibleCol {
			visibleTo = maxVisibleCol
		}
		if visibleTo <= visibleFrom {
			continue
		}
		segment := line[span.byteStart:span.byteEnd]
		relFrom := visibleFrom - span.colStart
		relTo := visibleTo - span.colStart
		if relFrom > 0 || relTo < span.colEnd-span.colStart {
			byteFrom := byteIndexAtRune(segment, relFrom)
			byteTo := byteIndexAtRune(segment, relTo)
			if byteTo <= byteFrom {
				continue
			}
			segment = segment[byteFrom:byteTo]
		}
		x := v.textPad + v.colOffsetPx(visibleFrom-hCol)
		if x >= textW {
			break
		}
		offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		ui.drawStreamOutputPlainLine(th, gtx, segment, face, size, viewerSyntaxColor(theme, span.role))
		offset.Pop()
		drew = true
	}
	if !drew {
		ui.drawStreamOutputPlainLine(th, gtx, fallback, face, size, theme.Text)
	}
}

func (ui *UI) drawStreamOutputScrollbar(gtx layout.Context, st *fileViewerState) {
	if st == nil {
		return
	}
	v := &st.stream
	theme := ui.fileViewerTheme()
	track := v.trackRect
	thumb := v.thumbRect
	if track.Dx() > 0 && track.Dy() > 0 {
		paintViewerScrollbar(gtx, theme, track, thumb, v.hoverTrack, v.hoverThumb, v.dragging)
	}

	hTrack := v.hTrackRect
	hThumb := v.hThumbRect
	if hTrack.Dx() <= 0 || hTrack.Dy() <= 0 {
		return
	}
	paintViewerScrollbar(gtx, theme, hTrack, hThumb, v.hoverHTrack, v.hoverHThumb, v.hDragging)
}

func (ui *UI) drawStreamOutputTooltip(th *material.Theme, gtx layout.Context, st *fileViewerState) {
	if st == nil {
		return
	}
	v := &st.stream
	if !v.dragging {
		return
	}
	theme := ui.fileViewerTheme()
	total := len(v.lines)
	if total < 1 {
		total = 1
	}
	line := v.dragTopLine + 1
	if line < 1 {
		line = 1
	}
	if line > total {
		line = total
	}
	percent := 0.0
	maxTop := total - v.visibleLines
	if maxTop > 0 {
		percent = float64(v.dragTopLine) * 100 / float64(maxTop)
	}
	msg := fmt.Sprintf("~ line %d/%d (%.1f%%)", line, total, percent)
	gap := gtx.Dp(unit.Dp(streamTooltipGapDp))
	if gap < 1 {
		gap = 1
	}
	edgeInset := gtx.Dp(unit.Dp(fileViewerTooltipEdgeInsetDp))
	if edgeInset < 2 {
		edgeInset = 2
	}
	maxBoxW := v.trackRect.Min.X - gap - edgeInset
	if maxBoxW < 1 {
		return
	}
	box := ui.measureStreamOutputTooltipBox(th, gtx, msg, maxBoxW)
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
	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
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
				lbl := ui.streamOutputTooltipLabel(th, msg)
				lbl.Color = theme.TooltipText
				return layoutVCenteredLabel(gtx, lbl)
			})
		},
	)
	offset.Pop()
}

func (ui *UI) measureStreamOutputTooltipBox(th *material.Theme, gtx layout.Context, msg string, maxBoxW int) image.Point {
	lbl := ui.streamOutputTooltipLabel(th, msg)
	textDims := measureLabelUnconstrained(gtx, lbl).Size
	fontPx := gtx.Sp(scaleThemeFontSize(th, 9))
	if fontPx < 8 {
		fontPx = 8
	}
	charW := int(float32(fontPx)*0.58 + 0.5)
	if charW < 4 {
		charW = 4
	}
	estimatedW := utf8.RuneCountInString(msg) * charW
	if textDims.X < estimatedW {
		textDims.X = estimatedW
	}
	estimatedH := fontPx + gtx.Dp(unit.Dp(2))
	if textDims.Y < estimatedH {
		textDims.Y = estimatedH
	}
	boxW := textDims.X + gtx.Dp(unit.Dp(streamTooltipInsetXDp*2))
	boxH := textDims.Y + gtx.Dp(unit.Dp(streamTooltipInsetYDp*2))
	if minW := gtx.Dp(unit.Dp(streamTooltipMinWidthDp)); boxW < minW {
		boxW = minW
	}
	if minH := gtx.Dp(unit.Dp(streamTooltipMinHeightDp)); boxH < minH {
		boxH = minH
	}
	if maxBoxW > 0 && boxW > maxBoxW {
		boxW = maxBoxW
	}
	return image.Pt(boxW, boxH)
}

func (ui *UI) streamOutputTooltipLabel(th *material.Theme, msg string) material.LabelStyle {
	lbl := material.Caption(th, msg)
	lbl.Font.Typeface = ui.viewerTypeface()
	lbl.TextSize = scaleThemeFontSize(th, 9)
	lbl.MaxLines = 1
	lbl.Truncator = ""
	return lbl
}

func (ui *UI) applyStreamOutputCursor(gtx layout.Context, st *fileViewerState) {
	if st == nil {
		return
	}
	v := &st.stream
	if v.dragging {
		pointer.CursorGrabbing.Add(gtx.Ops)
		return
	}
	if v.hDragging {
		pointer.CursorGrabbing.Add(gtx.Ops)
		return
	}
	if v.hoverThumb || v.hoverTrack {
		defer clip.Rect(v.trackRect).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		return
	}
	if v.hoverHThumb || v.hoverHTrack {
		defer clip.Rect(v.hTrackRect).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		return
	}
	if v.textRect.Dx() > 0 && v.textRect.Dy() > 0 {
		defer clip.Rect(v.textRect).Push(gtx.Ops).Pop()
		pointer.CursorText.Add(gtx.Ops)
	}
}
