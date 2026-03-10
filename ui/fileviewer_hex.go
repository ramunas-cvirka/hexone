package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

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

const (
	hexViewerMinBytesPerLine = 4
	hexViewerMaxBytesPerLine = 64
	hexViewerMinChunkBytes   = 4096
)

type hexViewerState struct {
	resultCh chan fileViewerHexResult
	seq      int

	fileSize     int64
	bufferStart  int64
	buffer       []byte
	bytesPerLine int
	topLine      int64
	visibleLines int
	scrollCarry  float32

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
	v.columnGap = gtx.Dp(unit.Dp(12))
	if v.columnGap < v.charW {
		v.columnGap = v.charW
	}
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
	v.stopAutoScroll()
	return true
}

func (v *hexViewerState) renderedLineCount() int {
	if v == nil {
		return 0
	}
	total := int(v.totalLines() - v.topLine)
	if total < 0 {
		total = 0
	}
	if v.visibleLines < 1 {
		if total > 0 {
			return 1
		}
		return 0
	}
	if total > v.visibleLines {
		total = v.visibleLines
	}
	return total
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

func (v *hexViewerState) selectedBytes() ([]byte, bool) {
	if v == nil || !v.hasSelection() {
		return nil, false
	}
	endSel := v.selectionEnd()
	if v.selectionStart < v.bufferStart {
		return nil, false
	}
	bufferEnd := v.bufferStart + int64(len(v.buffer))
	if endSel > bufferEnd {
		return nil, false
	}
	start := int(v.selectionStart - v.bufferStart)
	end := int(endSel - v.bufferStart)
	if start < 0 || end < start || end > len(v.buffer) {
		return nil, false
	}
	out := make([]byte, end-start)
	copy(out, v.buffer[start:end])
	return out, true
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

	return visibleStart < v.bufferStart+margin || visibleEnd > bufferEnd-margin
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
	return v.buffer[relStart:relEnd], start
}

func (v *hexViewerState) scrollByDelta(delta float32) {
	if v == nil || delta == 0 {
		return
	}
	if (delta > 0 && v.scrollCarry < 0) || (delta < 0 && v.scrollCarry > 0) {
		v.scrollCarry = 0
	}
	v.scrollCarry += delta

	// Gio wheel delta is device-dependent and can be much larger than 1 per notch.
	// Normalize it so one wheel notch advances roughly one line instead of a page.
	const wheelStep float32 = 80

	steps := int64(0)
	for v.scrollCarry >= wheelStep {
		steps++
		v.scrollCarry -= wheelStep
	}
	for v.scrollCarry <= -wheelStep {
		steps--
		v.scrollCarry += wheelStep
	}
	if steps == 0 {
		return
	}
	v.topLine += steps
	v.clampTop()
}

func (v *hexViewerState) computeLayout(size image.Point) {
	if v == nil || size.X <= 0 || size.Y <= 0 {
		return
	}
	scrollbarW := 6
	trackGap := 4
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

	trackX := size.X - scrollbarW
	if trackX < v.textRect.Max.X+trackGap {
		trackX = v.textRect.Max.X + trackGap
	}
	if trackX > size.X-scrollbarW {
		trackX = size.X - scrollbarW
	}
	v.trackRect = image.Rect(trackX, 0, trackX+scrollbarW, size.Y)
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
	thumbH := int(float32(v.trackRect.Dy()) * float32(v.visibleLines) / float32(totalLines))
	if thumbH < 18 {
		thumbH = 18
	}
	if thumbH > v.trackRect.Dy() {
		thumbH = v.trackRect.Dy()
	}
	maxTop := totalLines - int64(v.visibleLines)
	maxTravel := v.trackRect.Dy() - thumbH
	topForThumb := v.topLine
	if v.dragging {
		topForThumb = v.dragTop
	}
	if topForThumb < 0 {
		topForThumb = 0
	}
	if topForThumb > maxTop {
		topForThumb = maxTop
	}
	thumbY := 0
	if maxTop > 0 && maxTravel > 0 {
		thumbY = int(float64(topForThumb) * float64(maxTravel) / float64(maxTop))
	}
	v.thumbRect = image.Rect(v.trackRect.Min.X+1, v.trackRect.Min.Y+thumbY, v.trackRect.Max.X-1, v.trackRect.Min.Y+thumbY+thumbH)
}

func (v *hexViewerState) estimatedTopFromY(y int) int64 {
	return v.estimatedTopFromDragY(y, v.thumbRect.Dy()/2)
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

func hexByteColumn(byteIndex, groupBytes int) int {
	if byteIndex <= 0 {
		return 0
	}
	col := 0
	for i := 0; i < byteIndex; i++ {
		col += 2
		col++
		if groupBytes > 1 && (i+1)%groupBytes == 0 {
			col++
		}
	}
	return col
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
	var b strings.Builder
	b.Grow(len(data)*3 - 1)
	for i, v := range data {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%02X", v)
	}
	return b.String()
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

	// Allow below-the-content selection start/drag by clamping row.
	row := (pos.Y - v.hexRect.Min.Y) / v.lineH
	if row < 0 {
		row = 0
	}
	if row >= v.visibleLines {
		row = v.visibleLines - 1
	}
	if row < 0 {
		row = 0
	}

	line := v.topLine + int64(row)
	maxLine := v.totalLines() - 1
	if maxLine < 0 {
		maxLine = 0
	}
	if line > maxLine {
		line = maxLine
	}
	if line < 0 {
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
		return
	}

	st.seq++
	seq := st.seq
	st.loading = true
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
		info, statErr := os.Stat(path)
		if statErr != nil {
			res.err = statErr.Error()
			return res
		}
		if info.IsDir() {
			res.err = "viewer supports files only"
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
	v.computeLayout(size)
	ui.handleHexViewerEvents(gtx, st)
	if v.expireCancelGrace(gtx.Now) {
		st.markUserBrowsing(gtx.Now)
	}
	if v.runAutoScroll(gtx.Now) {
		st.markUserBrowsing(gtx.Now)
		ui.startHexViewerLoad(st, false)
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
	return layout.Dimensions{Size: size}
}

func (ui *UI) drawHexOutput(gtx layout.Context, th *material.Theme, st *fileViewerState) {
	v := st.hex
	if v == nil {
		return
	}
	theme := ui.fileViewerTheme()
	sepColor := theme.Separator
	paint.FillShape(gtx.Ops, sepColor, clip.Rect(image.Rect(v.offsetRect.Max.X+v.columnGap/2, 0, v.offsetRect.Max.X+v.columnGap/2+1, v.offsetRect.Max.Y)).Op())
	paint.FillShape(gtx.Ops, sepColor, clip.Rect(image.Rect(v.hexRect.Max.X+v.columnGap/2, 0, v.hexRect.Max.X+v.columnGap/2+1, v.hexRect.Max.Y)).Op())

	y := 0
	total := v.totalLines()
	end := v.topLine + int64(v.visibleLines)
	if end > total {
		end = total
	}
	for line := v.topLine; line < end; line++ {
		lineBytes, lineStart := v.lineBytes(line)
		offsetText := formatHexOffset(lineStart, v.offsetDigits)
		offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		ui.drawHexLineSelections(gtx, st, line, len(lineBytes))
		ui.drawHexLineLabel(th, gtx, image.Pt(v.offsetRect.Min.X, 0), v.offsetRect.Dx(), offsetText, theme.OffsetText)
		ui.drawHexBytesLine(th, gtx, v, lineBytes, theme.Text)
		ui.drawHexASCIIline(th, gtx, v, lineBytes, theme.ASCIIText)
		offset.Pop()
		y += v.lineH
	}
}

func (ui *UI) drawHexLineLabel(th *material.Theme, gtx layout.Context, pos image.Point, width int, text string, fg color.NRGBA) {
	if width <= 0 {
		return
	}
	offset := op.Offset(pos).Push(gtx.Ops)
	lineGTX := gtx
	lineGTX.Constraints = layout.Exact(image.Pt(width, gtx.Constraints.Max.Y))
	lbl := material.Body2(th, text)
	lbl.Font.Typeface = ui.viewerMonospaceTypeface()
	lbl.Font.Weight = font.Normal
	lbl.TextSize = ui.viewerTextSize()
	lbl.Color = fg
	lbl.MaxLines = 1
	lbl.Truncator = ""
	lbl.Layout(lineGTX)
	offset.Pop()
}

func (ui *UI) drawMonoCell(th *material.Theme, gtx layout.Context, pos image.Point, width int, text string, fg color.NRGBA) {
	if width <= 0 || text == "" {
		return
	}
	offset := op.Offset(pos).Push(gtx.Ops)
	lineGTX := gtx
	lineGTX.Constraints = layout.Exact(image.Pt(width, gtx.Constraints.Max.Y))
	lbl := material.Body2(th, text)
	lbl.Font.Typeface = ui.viewerMonospaceTypeface()
	lbl.Font.Weight = font.Normal
	lbl.TextSize = ui.viewerTextSize()
	lbl.Color = fg
	lbl.MaxLines = 1
	lbl.Truncator = ""
	lbl.Layout(lineGTX)
	offset.Pop()
}

func (ui *UI) drawHexBytesLine(th *material.Theme, gtx layout.Context, v *hexViewerState, data []byte, fg color.NRGBA) {
	if v == nil {
		return
	}
	for i, b := range data {
		txt := fmt.Sprintf("%02X", b)
		x := v.hexRect.Min.X + v.hexByteLeft(i)
		ui.drawMonoCell(th, gtx, image.Pt(x, 0), 2*v.charW, txt, fg)
	}
}

func (ui *UI) drawHexASCIIline(th *material.Theme, gtx layout.Context, v *hexViewerState, data []byte, fg color.NRGBA) {
	if v == nil {
		return
	}
	for i, b := range data {
		r := rune(b)
		ch := "."
		if r >= 0x20 && unicode.IsPrint(r) {
			ch = string(b)
		}
		x := v.textRect.Min.X + i*v.charW
		ui.drawMonoCell(th, gtx, image.Pt(x, 0), v.charW, ch, fg)
	}
}

func (ui *UI) drawHexLineSelections(gtx layout.Context, st *fileViewerState, line int64, lineLen int) {
	v := st.hex
	if v == nil || !v.hasSelection() || lineLen <= 0 {
		return
	}
	theme := ui.fileViewerTheme()

	lineStart := line * int64(v.bytesPerLine)
	lineEnd := lineStart + int64(lineLen)
	selectionEnd := v.selectionEnd()
	if lineEnd <= v.selectionStart || lineStart >= selectionEnd {
		return
	}

	selStart := v.selectionStart
	if selStart < lineStart {
		selStart = lineStart
	}
	selEnd := selectionEnd
	if selEnd > lineEnd {
		selEnd = lineEnd
	}
	if selEnd <= selStart {
		return
	}

	firstIdx := int(selStart - lineStart)
	lastIdx := int(selEnd - lineStart - 1)

	hexSel := theme.Selection
	textSel := theme.StrongSelection

	hexX0 := v.hexRect.Min.X + v.hexByteLeft(firstIdx)
	var hexX1 int
	if lastIdx+1 < lineLen {
		hexX1 = v.hexRect.Min.X + v.hexByteLeft(lastIdx+1)
	} else {
		hexX1 = v.hexRect.Min.X + v.hexByteRight(lastIdx)
	}
	paint.FillShape(gtx.Ops, hexSel, clip.Rect(image.Rect(hexX0, 1, hexX1, v.lineH-1)).Op())

	textX0 := v.textRect.Min.X + firstIdx*v.charW
	textX1 := v.textRect.Min.X + (lastIdx+1)*v.charW
	paint.FillShape(gtx.Ops, textSel, clip.Rect(image.Rect(textX0, 1, textX1, v.lineH-1)).Op())
}

func (ui *UI) drawHexScrollbar(gtx layout.Context, st *fileViewerState) {
	v := st.hex
	if v == nil || v.trackRect.Dx() <= 0 || v.trackRect.Dy() <= 0 || v.thumbRect.Dy() <= 0 {
		return
	}
	theme := ui.fileViewerTheme()
	trackColor := theme.ScrollTrack
	if v.hoverTrack || v.dragging {
		trackColor = theme.ScrollTrackHover
	}
	thumbColor := theme.ScrollThumb
	if v.hoverTrack {
		thumbColor = theme.ScrollThumbHover
	}
	if v.dragging {
		thumbColor = theme.ScrollThumbDrag
	}
	paint.FillShape(gtx.Ops, trackColor, clip.Rect(v.trackRect).Op())
	paint.FillShape(gtx.Ops, thumbColor, clip.Rect(v.thumbRect).Op())
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

	boxW := gtx.Dp(unit.Dp(198))
	boxH := gtx.Dp(unit.Dp(24))
	if boxW < 96 {
		boxW = 96
	}
	if boxH < 16 {
		boxH = 16
	}
	x := v.trackRect.Min.X - boxW - gtx.Dp(unit.Dp(6))
	if x < 2 {
		x = 2
	}
	y := v.thumbRect.Min.Y + v.thumbRect.Dy()/2 - boxH/2
	if y < 2 {
		y = 2
	}
	maxY := gtx.Constraints.Max.Y - boxH - 2
	if maxY < 2 {
		maxY = 2
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
			return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, msg)
				lbl.Font.Typeface = ui.viewerTypeface()
				lbl.TextSize = scaleThemeFontSize(th, 9)
				lbl.Color = theme.TooltipText
				lbl.MaxLines = 1
				lbl.Truncator = ""
				return layoutVCenteredLabel(gtx, lbl)
			})
		},
	)
	stack.Pop()
}

func (ui *UI) applyHexViewerCursor(gtx layout.Context, st *fileViewerState) {
	v := st.hex
	if v == nil {
		return
	}
	if v.dragging {
		defer clip.Rect(v.trackRect).Push(gtx.Ops).Pop()
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
				gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				st.markUserBrowsing(gtx.Now)
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) {
				if st.menuOpen {
					st.closeContextMenu()
				}
				if byteOff, ok := hexByteAtPoint(v, pos); ok {
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
			}
			if v.selecting && pe.PointerID == v.selectID {
				v.selectPos = pos
				if byteOff, ok := hexSelectionByteAtPoint(v, pos); ok {
					v.setSelectionFromAnchor(v.dragAnchor, byteOff)
				}
				v.updateAutoScroll(pos, gtx.Now)
				st.markUserBrowsing(gtx.Now)
			}
			v.hoverTrack = viewerPointInRect(pos, v.trackRect)
		case pointer.Release:
			if v.dragging && pe.PointerID == v.dragID {
				v.topLine = v.dragTop
				v.dragging = false
				v.dragGrabY = 0
				v.clampTop()
				st.markUserBrowsing(gtx.Now)
			}
			if v.selecting && pe.PointerID == v.selectID {
				if v.cancelPending {
					v.beginCancelGrace(gtx.Now)
				} else {
					v.selecting = false
					v.clearCancelGrace()
					v.stopAutoScroll()
				}
			}
			v.hoverTrack = viewerPointInRect(pos, v.trackRect)
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
			v.hoverTrack = false
		case pointer.Move, pointer.Enter:
			if pe.Kind == pointer.Enter {
				v.pointerOutside = false
			}
			if v.selecting {
				v.selectPos = pos
				v.updateAutoScroll(pos, gtx.Now)
				st.markUserBrowsing(gtx.Now)
			}
			v.hoverTrack = viewerPointInRect(pos, v.trackRect)
		case pointer.Leave:
			v.pointerOutside = true
			if v.selecting {
				v.selectPos = pos
				v.updateAutoScroll(pos, gtx.Now)
				st.markUserBrowsing(gtx.Now)
			}
			v.hoverTrack = false
		}
	}
}
