package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"strings"
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
	dragTop    int64

	selecting  bool
	selectID   pointer.ID
	dragAnchor int64

	selectionStart int64
	selectionLen   int64

	groupBytes int

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
	v.charW = measureStreamCharWidth(ui, th, gtx)
	if v.charW < 1 {
		v.charW = 1
	}
	v.lineH = measureStreamLineHeight(ui, th, gtx)
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
	thumbY := 0
	if maxTop > 0 && maxTravel > 0 {
		thumbY = int(float64(v.topLine) * float64(maxTravel) / float64(maxTop))
	}
	v.thumbRect = image.Rect(v.trackRect.Min.X+1, v.trackRect.Min.Y+thumbY, v.trackRect.Max.X-1, v.trackRect.Min.Y+thumbY+thumbH)
}

func (v *hexViewerState) estimatedTopFromY(y int) int64 {
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
	dragY := y - v.trackRect.Min.Y - v.thumbRect.Dy()/2
	if dragY < 0 {
		dragY = 0
	}
	if dragY > maxTravel {
		dragY = maxTravel
	}
	return int64(float64(dragY) * float64(maxTop) / float64(maxTravel))
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
	if v == nil || v.bytesPerLine <= 0 || v.lineH <= 0 {
		return 0, false
	}
	row := pos.Y / v.lineH
	if row < 0 || row >= v.visibleLines {
		return 0, false
	}
	line := v.topLine + int64(row)
	lineStart := line * int64(v.bytesPerLine)
	if lineStart >= v.fileSize {
		return 0, false
	}
	if viewerPointInRect(pos, v.textRect) {
		col := (pos.X - v.textRect.Min.X) / v.charW
		if col < 0 || col >= v.bytesPerLine {
			return 0, false
		}
		byteOffset := lineStart + int64(col)
		if byteOffset >= v.fileSize {
			return 0, false
		}
		return byteOffset, true
	}
	if !viewerPointInRect(pos, v.hexRect) {
		return 0, false
	}
	col := (pos.X - v.hexRect.Min.X) / v.charW
	for i := 0; i < v.bytesPerLine; i++ {
		startCol := hexByteColumn(i, v.groupBytes)
		if col >= startCol && col < startCol+2 {
			byteOffset := lineStart + int64(i)
			if byteOffset >= v.fileSize {
				return 0, false
			}
			return byteOffset, true
		}
	}
	return 0, false
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
	if windowBytes < hexViewerMinChunkBytes {
		windowBytes = hexViewerMinChunkBytes
	}
	wantStart := visibleStart - windowBytes
	if wantStart < 0 {
		wantStart = 0
	}
	wantEnd := visibleEnd + windowBytes
	if v.fileSize > 0 && wantEnd > v.fileSize {
		wantEnd = v.fileSize
	}
	if wantEnd < wantStart+int64(hexViewerMinChunkBytes) {
		wantEnd = wantStart + int64(hexViewerMinChunkBytes)
	}
	if v.fileSize > 0 && wantEnd > v.fileSize {
		wantEnd = v.fileSize
	}
	if !force && len(v.buffer) > 0 && v.bufferCovers(visibleStart, visibleEnd) {
		return
	}

	st.seq++
	seq := st.seq
	st.loading = true
	st.err = ""
	if len(v.buffer) == 0 {
		st.status = "loading..."
	}
	if st.loadCancel != nil {
		st.loadCancel()
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
				st.hex.bufferStart = res.start
				st.hex.buffer = res.data
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
	ui.startHexViewerLoad(st, false)

	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	ui.drawHexOutput(gtx, th, st)
	ui.drawHexScrollbar(gtx, st)
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
	sepColor := color.NRGBA{R: 255, G: 255, B: 255, A: 16}
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
		hexText := formatHexLine(lineBytes, v.bytesPerLine, v.groupBytes)
		asciiText := formatHexTextLine(lineBytes, v.bytesPerLine)
		lineGTX := gtx
		lineGTX.Constraints = layout.Exact(image.Pt(v.textRect.Max.X, v.lineH))
		offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		ui.drawHexLineSelections(gtx, st, line, len(lineBytes))
		ui.drawHexLineLabel(th, gtx, image.Pt(v.offsetRect.Min.X, 0), v.offsetRect.Dx(), offsetText, color.NRGBA{R: 154, G: 166, B: 186, A: 255})
		ui.drawHexLineLabel(th, gtx, image.Pt(v.hexRect.Min.X, 0), v.hexRect.Dx(), hexText, color.NRGBA{R: 222, G: 228, B: 242, A: 255})
		ui.drawHexLineLabel(th, gtx, image.Pt(v.textRect.Min.X, 0), v.textRect.Dx(), asciiText, color.NRGBA{R: 198, G: 214, B: 232, A: 255})
		offset.Pop()
		_ = lineGTX
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
	_ = layout.Inset{Left: unit.Dp(1)}.Layout(lineGTX, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Body2(th, text)
		lbl.Font.Typeface = ui.mainTypeface()
		lbl.Font.Weight = font.Normal
		lbl.TextSize = ui.viewerTextSize()
		lbl.Color = fg
		lbl.MaxLines = 1
		lbl.Truncator = ""
		return lbl.Layout(gtx)
	})
	offset.Pop()
}

func (ui *UI) drawHexLineSelections(gtx layout.Context, st *fileViewerState, line int64, lineLen int) {
	v := st.hex
	if v == nil || !v.hasSelection() || lineLen <= 0 {
		return
	}
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
	hexSel := color.NRGBA{R: 98, G: 138, B: 212, A: 118}
	textSel := color.NRGBA{R: 98, G: 138, B: 212, A: 138}
	for off := selStart; off < selEnd; off++ {
		idx := int(off - lineStart)
		hexX := v.hexRect.Min.X + hexByteColumn(idx, v.groupBytes)*v.charW
		hexRect := image.Rect(hexX, 1, hexX+v.charW*2, v.lineH-1)
		paint.FillShape(gtx.Ops, hexSel, clip.Rect(hexRect).Op())
		textX := v.textRect.Min.X + idx*v.charW
		textRect := image.Rect(textX, 1, textX+v.charW, v.lineH-1)
		paint.FillShape(gtx.Ops, textSel, clip.Rect(textRect).Op())
	}
}

func (ui *UI) drawHexScrollbar(gtx layout.Context, st *fileViewerState) {
	v := st.hex
	if v == nil || v.trackRect.Dx() <= 0 || v.trackRect.Dy() <= 0 || v.thumbRect.Dy() <= 0 {
		return
	}
	trackColor := color.NRGBA{R: 255, G: 255, B: 255, A: 24}
	if v.hoverTrack || v.dragging {
		trackColor = color.NRGBA{R: 255, G: 255, B: 255, A: 40}
	}
	thumbColor := color.NRGBA{R: 173, G: 197, B: 238, A: 178}
	if v.dragging {
		thumbColor = color.NRGBA{R: 204, G: 224, B: 255, A: 236}
	}
	paint.FillShape(gtx.Ops, trackColor, clip.Rect(v.trackRect).Op())
	paint.FillShape(gtx.Ops, thumbColor, clip.Rect(v.thumbRect).Op())
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
			if pe.Buttons.Contain(pointer.ButtonSecondary) {
				st.menuOpen = true
				st.menuPos = pos
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) && viewerPointInRect(pos, v.trackRect) {
				v.dragging = true
				v.dragID = pe.PointerID
				v.dragTop = v.estimatedTopFromY(pos.Y)
				gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				st.markUserBrowsing(gtx.Now)
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) {
				if st.menuOpen {
					st.menuOpen = false
				}
				if byteOff, ok := hexByteAtPoint(v, pos); ok {
					v.selecting = true
					v.selectID = pe.PointerID
					v.dragAnchor = byteOff
					v.setSelectionRange(byteOff, 1)
					gtx.Execute(pointer.GrabCmd{Tag: &v.pointerTag, ID: pe.PointerID})
				} else {
					v.clearSelection()
				}
				st.markUserBrowsing(gtx.Now)
			}
		case pointer.Drag:
			if v.dragging && pe.PointerID == v.dragID {
				v.dragTop = v.estimatedTopFromY(pos.Y)
				v.topLine = v.dragTop
				v.clampTop()
				ui.startHexViewerLoad(st, false)
				st.markUserBrowsing(gtx.Now)
			}
			if v.selecting && pe.PointerID == v.selectID {
				if byteOff, ok := hexByteAtPoint(v, pos); ok {
					v.setSelectionFromAnchor(v.dragAnchor, byteOff)
				}
				st.markUserBrowsing(gtx.Now)
			}
			v.hoverTrack = viewerPointInRect(pos, v.trackRect)
		case pointer.Release:
			if v.dragging && pe.PointerID == v.dragID {
				v.dragging = false
			}
			if v.selecting && pe.PointerID == v.selectID {
				v.selecting = false
			}
			v.hoverTrack = viewerPointInRect(pos, v.trackRect)
		case pointer.Cancel:
			if v.dragging && pe.PointerID == v.dragID {
				v.dragging = false
			}
			if v.selecting && pe.PointerID == v.selectID {
				v.selecting = false
			}
			v.hoverTrack = false
		case pointer.Move, pointer.Enter:
			v.hoverTrack = viewerPointInRect(pos, v.trackRect)
		case pointer.Leave:
			v.hoverTrack = false
		}
	}
}
