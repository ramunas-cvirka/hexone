// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"math"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"hexone/fm"
)

func TestNormalizeViewerModeSupportsHex(t *testing.T) {
	if got := normalizeViewerMode("hex"); got != "hex" {
		t.Fatalf("normalizeViewerMode(hex) = %q, want hex", got)
	}
	if got := normalizeViewerMode(" HEX "); got != "hex" {
		t.Fatalf("normalizeViewerMode(trimmed hex) = %q, want hex", got)
	}
}

func TestComputeHexBytesPerLineGrowsWithWidth(t *testing.T) {
	narrow := computeHexBytesPerLine(240, 8, 8, 0)
	wide := computeHexBytesPerLine(640, 8, 8, 0)
	if narrow < hexViewerMinBytesPerLine {
		t.Fatalf("narrow bytes/line = %d, want at least %d", narrow, hexViewerMinBytesPerLine)
	}
	if wide <= narrow {
		t.Fatalf("wide bytes/line = %d, want > narrow %d", wide, narrow)
	}
	if wide > hexViewerMaxBytesPerLine {
		t.Fatalf("wide bytes/line = %d, want <= %d", wide, hexViewerMaxBytesPerLine)
	}
}

func TestHexSectionSeparatorsStayInsideEqualColumnGaps(t *testing.T) {
	v := &hexViewerState{
		offsetRect: image.Rect(6, 0, 70, 120),
		hexRect:    image.Rect(91, 0, 211, 120),
		textRect:   image.Rect(232, 0, 296, 120),
	}
	rects := hexSectionSeparatorRects(v)

	if got, want := v.hexRect.Min.X-v.offsetRect.Max.X, v.textRect.Min.X-v.hexRect.Max.X; got != want {
		t.Fatalf("column gaps differ: %d and %d", got, want)
	}
	if rects[0].Min.X <= v.offsetRect.Max.X || rects[0].Max.X >= v.hexRect.Min.X {
		t.Fatalf("first separator %v overlaps a content hit-box", rects[0])
	}
	if rects[1].Min.X <= v.hexRect.Max.X || rects[1].Max.X >= v.textRect.Min.X {
		t.Fatalf("second separator %v overlaps a content hit-box", rects[1])
	}
	for i, bounds := range [][2]int{{v.offsetRect.Max.X, v.hexRect.Min.X}, {v.hexRect.Max.X, v.textRect.Min.X}} {
		leftPad := rects[i].Min.X - bounds[0]
		rightPad := bounds[1] - rects[i].Max.X
		if leftPad != rightPad {
			t.Fatalf("separator %d padding differs: left=%d right=%d", i, leftPad, rightPad)
		}
	}
}

func TestHexSectionColumnGapProvidesOneCharacterPerSide(t *testing.T) {
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	gap := hexSectionColumnGap(gtx, 9)
	if gap%2 != 1 {
		t.Fatalf("column gap=%d want odd width for equal separator padding", gap)
	}
	if pad := (gap - 1) / 2; pad < 9 {
		t.Fatalf("column side padding=%d want at least one 9px character", pad)
	}
}

func TestFormatHexLineAndTextLine(t *testing.T) {
	data := []byte{0x41, 0x00, 0x7A}
	if got, want := formatHexLine(data, 4, 0), "41 00 7A   "; got != want {
		t.Fatalf("formatHexLine = %q, want %q", got, want)
	}
	if got, want := formatHexTextLine(data, 4), "A.z "; got != want {
		t.Fatalf("formatHexTextLine = %q, want %q", got, want)
	}
}

func TestFormatHexLineWithGrouping(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04}
	if got, want := formatHexLine(data, 4, 2), "01 02  03 04"; got != want {
		t.Fatalf("formatHexLine = %q, want %q", got, want)
	}
}

func TestFormatHexSelectionCopyUsesContinuousHex(t *testing.T) {
	data := []byte{0x48, 0x65, 0x78, 0x6F}
	if got, want := formatHexSelectionCopy(data), "4865786F"; got != want {
		t.Fatalf("formatHexSelectionCopy = %q, want %q", got, want)
	}
}

func TestFormatHexSelectionTextCopyEscapesNonTextBytes(t *testing.T) {
	data := []byte{'H', 'i', 0x00, '\\', '\n', 0xFF}
	if got, want := formatHexSelectionTextCopy(data), "Hi\\x00\\\\\n\\xFF"; got != want {
		t.Fatalf("formatHexSelectionTextCopy = %q, want %q", got, want)
	}
}

func TestFormatHexSelectionTextCopyDecodesUTF8(t *testing.T) {
	data := []byte("po muziejaus grįžti į Hotel Royal\n**~14:30–18:15** — poilsis\nPradžia: Eilė: CHF 300 už abu")
	if got, want := formatHexSelectionTextCopy(data), string(data); got != want {
		t.Fatalf("formatHexSelectionTextCopy = %q, want %q", got, want)
	}
}

func TestFormatHexSelectionTextCopyRequiresValidUTF8Selection(t *testing.T) {
	data := append([]byte("Aį"), 0xFF)
	data = append(data, []byte("—B")...)
	if got, want := formatHexSelectionTextCopy(data), `A\xC4\xAF\xFF\xE2\x80\x94B`; got != want {
		t.Fatalf("formatHexSelectionTextCopy = %q, want %q", got, want)
	}
}

func TestFormatHexSelectionTextCopyPreservesLineEndings(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "unix LF", data: []byte("alpha\nbeta\n"), want: "alpha\nbeta\n"},
		{name: "windows CRLF", data: []byte("alpha\r\nbeta\r\n"), want: "alpha\r\nbeta\r\n"},
		{name: "mixed line endings", data: []byte("unix\nwindows\r\n"), want: "unix\nwindows\r\n"},
		{name: "isolated CR remains escaped", data: []byte("alpha\rbeta"), want: `alpha\x0Dbeta`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatHexSelectionTextCopy(tc.data); got != tc.want {
				t.Fatalf("formatHexSelectionTextCopy=%q want %q", got, tc.want)
			}
		})
	}
}

func TestCopyFileViewerHexAsTextWritesActualCRLFToClipboard(t *testing.T) {
	oldWriteNow := writeFileViewerClipboardNow
	writeFileViewerClipboardNow = func(string) error { return nil }
	t.Cleanup(func() { writeFileViewerClipboardNow = oldWriteNow })

	data := []byte("first\r\nsecond\r\n")
	v := newHexViewerState()
	v.fileSize = int64(len(data))
	v.buffer = data
	v.setSelectionRange(0, int64(len(data)))
	st := &fileViewerState{mode: "hex", hex: v}
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	router := new(input.Router)
	gtx := layout.Context{Ops: new(op.Ops), Source: router.Source()}

	if !ui.copyFileViewerHex(gtx, false, true) {
		t.Fatalf("copy as text failed: %s", st.status)
	}
	_, copied, ok := router.WriteClipboard()
	if !ok || string(copied) != string(data) {
		t.Fatalf("clipboard=(%v,%q) want exact CRLF text %q", ok, copied, data)
	}
}

func TestCopyFileViewerTextRejectsHexSelectionOverLimit(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		mode: "hex",
		hex:  newHexViewerState(),
	}
	st.hex.fileSize = viewerHexCopyMaxBytes + 1
	st.hex.buffer = make([]byte, viewerHexCopyMaxBytes+1)
	st.hex.setSelectionRange(0, viewerHexCopyMaxBytes+1)
	ui.fileViewer = st

	if ui.copyFileViewerText(layout.Context{}, false) {
		t.Fatal("copyFileViewerText should reject oversize hex selection")
	}
	if got, want := st.status, "hex copy is limited to 1 MiB"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestCopyFileViewerHexUsesRemoteFindMatchBeforeChunkLoads(t *testing.T) {
	oldWriteNow := writeFileViewerClipboardNow
	writeFileViewerClipboardNow = func(string) error { return nil }
	t.Cleanup(func() { writeFileViewerClipboardNow = oldWriteNow })

	for _, tc := range []struct {
		name     string
		query    string
		hexInput bool
		asText   bool
		want     string
	}{
		{name: "text query as hex", query: "needle", want: "6E6565646C65"},
		{name: "text query as text", query: "needle", asText: true, want: "needle"},
		{name: "hex query as hex", query: "DE AD BE EF", hexInput: true, want: "DEADBEEF"},
		{name: "hex query as text", query: "DE AD BE EF", hexInput: true, asText: true, want: `\xDE\xAD\xBE\xEF`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := newHexViewerState()
			v.fileSize = 4096
			v.buffer = []byte("old")
			st := &fileViewerState{
				mode:   "hex",
				hex:    v,
				remote: &paneSSHSession{},
			}
			st.find.open = true
			st.find.hexInput = tc.hexInput
			st.find.currentValid = true
			st.find.currentStart = 2048
			pattern, errText := viewerFindPatternBytes(tc.query, tc.hexInput)
			if errText != "" {
				t.Fatalf("test query is invalid: %s", errText)
			}
			st.find.currentLen = int64(len(pattern))
			st.find.editor.SetText(tc.query)
			ui := NewUI(fm.DefaultConfig())
			ui.fileViewer = st
			router := new(input.Router)
			gtx := layout.Context{Ops: new(op.Ops), Source: router.Source()}

			if !ui.copyFileViewerHex(gtx, false, tc.asText) {
				t.Fatalf("copy failed: status=%q", st.status)
			}
			_, got, ok := router.WriteClipboard()
			if !ok || string(got) != tc.want {
				t.Fatalf("clipboard=(%t, %q), want (true, %q)", ok, got, tc.want)
			}
		})
	}
}

func TestCopyFileViewerHexUsesExplicitSelectionBeforeFindMatch(t *testing.T) {
	oldWriteNow := writeFileViewerClipboardNow
	writeFileViewerClipboardNow = func(string) error { return nil }
	t.Cleanup(func() { writeFileViewerClipboardNow = oldWriteNow })

	data := []byte("selected lines\n")
	v := newHexViewerState()
	v.fileSize = int64(len(data))
	v.buffer = data
	v.setSelectionRange(0, int64(len(data)))
	st := &fileViewerState{mode: "hex", hex: v}
	st.find.open = true
	st.find.currentValid = true
	st.find.currentStart = 0
	st.find.currentLen = int64(len("failed"))
	st.find.editor.SetText("failed")

	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	router := new(input.Router)
	gtx := layout.Context{Ops: new(op.Ops), Source: router.Source()}

	if !ui.copyFileViewerHex(gtx, false, true) {
		t.Fatalf("copy failed: status=%q", st.status)
	}
	_, got, ok := router.WriteClipboard()
	if !ok || string(got) != string(data) {
		t.Fatalf("clipboard=(%t, %q), want selected range %q", ok, got, data)
	}
}

func TestFileViewerCopyUsesPlatformShortcut(t *testing.T) {
	oldWriteNow := writeFileViewerClipboardNow
	writeFileViewerClipboardNow = func(string) error { return nil }
	t.Cleanup(func() { writeFileViewerClipboardNow = oldWriteNow })

	v := newHexViewerState()
	v.fileSize = 3
	v.buffer = []byte{0xCA, 0xFE, 0x01}
	v.setSelectionRange(0, 3)
	st := &fileViewerState{mode: "hex", hex: v}
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	router := new(input.Router)
	gtx := layout.Context{Ops: new(op.Ops), Source: router.Source()}
	frame := func() {
		gtx.Ops.Reset()
		ui.handleFileViewerKeys(gtx)
		router.Frame(gtx.Ops)
	}
	frame()
	router.Queue(key.Event{Name: "c", Modifiers: key.ModShortcut, State: key.Press})
	frame()

	_, got, ok := router.WriteClipboard()
	if !ok || string(got) != "CAFE01" {
		t.Fatalf("clipboard=(%t, %q), want (true, %q)", ok, got, "CAFE01")
	}
}

func TestHexViewerComputeScrollbarUsesDragTopWhileDragging(t *testing.T) {
	v := &hexViewerState{
		fileSize:     4096,
		bytesPerLine: 16,
		visibleLines: 8,
		topLine:      4,
		dragTop:      40,
		dragging:     true,
		trackRect:    image.Rect(0, 0, 6, 160),
	}

	v.computeScrollbar()
	dragThumbY := v.thumbRect.Min.Y

	v.dragging = false
	v.computeScrollbar()
	liveThumbY := v.thumbRect.Min.Y

	if dragThumbY <= liveThumbY {
		t.Fatalf("drag thumb Y=%d, want > live thumb Y=%d", dragThumbY, liveThumbY)
	}

	const totalLines = 256
	const thumbH = fileViewerScrollbarMinThumbPx
	wantDragYf := float64(40*(160-thumbH)) / float64(totalLines-8)
	wantDragY := int(math.Round(wantDragYf))
	if dragThumbY != wantDragY {
		t.Fatalf("drag thumb Y=%d, want %d", dragThumbY, wantDragY)
	}
}

func TestHexDisplayStartFallsBackUntilJumpTargetIsBuffered(t *testing.T) {
	v := &hexViewerState{
		fileSize:     16 * 1000,
		bytesPerLine: 16,
		bufferStart:  0,
		buffer:       make([]byte, 4096),
		lastPaintTop: 2,
		lastPaintSet: true,
	}

	start, fallback := v.displayStartWithFallback(700, 710)
	if !fallback || start != 2 {
		t.Fatalf("unbuffered jump display=(%d,%v) want last painted top 2", start, fallback)
	}

	start, fallback = v.displayStartWithFallback(4, 14)
	if fallback || start != 4 {
		t.Fatalf("buffered display=(%d,%v) want target top 4", start, fallback)
	}
	if !v.lastPaintSet || v.lastPaintTop != 4 {
		t.Fatalf("last painted state=(%d,%v) want 4,true", v.lastPaintTop, v.lastPaintSet)
	}
}

func TestHexScrollByDeltaRespondsToFineWheelInput(t *testing.T) {
	v := &hexViewerState{
		fileSize:     16 * 100,
		bytesPerLine: 16,
		visibleLines: 8,
	}

	v.scrollByDelta(0.5)

	if got, want := v.topLine, int64(1); got != want {
		t.Fatalf("topLine=%d want %d after fine wheel delta", got, want)
	}
}

func TestHexScrollByDeltaCapsCoarseWheelInput(t *testing.T) {
	v := &hexViewerState{
		fileSize:     16 * 100,
		bytesPerLine: 16,
		visibleLines: 8,
	}

	v.scrollByDelta(120)

	if got, want := v.topLine, int64(hexViewerWheelMaxLines); got != want {
		t.Fatalf("topLine=%d want capped coarse-wheel step %d", got, want)
	}
}

func TestHexScrollByDeltaDropsCarryWhenDirectionChanges(t *testing.T) {
	v := &hexViewerState{
		fileSize:     16 * 100,
		bytesPerLine: 16,
		visibleLines: 8,
		topLine:      10,
	}

	v.scrollByDelta(0.25)
	v.scrollByDelta(-0.5)

	if got, want := v.topLine, int64(9); got != want {
		t.Fatalf("topLine=%d want %d after reversing a fine wheel gesture", got, want)
	}
}

func TestHexScrollTooltipMeasurementExpandsPastOldFixedWidth(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(720, 120),
		},
	}

	msg := "~ 0x123456789ABCDEF0  line 1234567/7654321 (100.0%)"
	box := ui.measureHexScrollTooltipBox(th, gtx, msg, 640)

	if box.X <= 198 {
		t.Fatalf("hex tooltip width=%d want content width beyond old fixed 198px", box.X)
	}
}

func TestHexPrepareVisualScrollAnimatesSmallStep(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	v := &hexViewerState{
		fileSize:     4096,
		bytesPerLine: 16,
		visibleLines: 4,
		lineH:        16,
		topLine:      0,
	}
	v.syncVisualTop()
	v.topLine = 2

	if animating := v.prepareVisualScroll(now.Add(streamSmoothTick), true); !animating {
		t.Fatal("prepareVisualScroll should animate short hex viewer scroll steps")
	}
	if v.visualTop <= 0 || v.visualTop >= 2 {
		t.Fatalf("visualTop=%v want between 0 and 2", v.visualTop)
	}
	if v.displayTop == 2 && v.displayY == 0 {
		t.Fatalf("display state=%d/%d want interpolated position", v.displayTop, v.displayY)
	}
}

func TestHexPrepareVisualScrollSnapsLargeJump(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	v := &hexViewerState{
		fileSize:     4096,
		bytesPerLine: 16,
		visibleLines: 4,
		lineH:        16,
		topLine:      0,
	}
	v.syncVisualTop()
	v.topLine = 20

	if animating := v.prepareVisualScroll(now.Add(streamSmoothTick), true); animating {
		t.Fatal("prepareVisualScroll should snap instead of animating large hex jumps")
	}
	if v.visualTop != 20 {
		t.Fatalf("visualTop=%v want 20", v.visualTop)
	}
	if v.displayTop != 20 || v.displayY != 0 {
		t.Fatalf("display state=%d/%d want snapped 20/0", v.displayTop, v.displayY)
	}
}

func TestHexPrepareVisualScrollSnapsDuringSelectionAutoScroll(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	v := &hexViewerState{
		fileSize:         4096,
		bytesPerLine:     16,
		visibleLines:     4,
		lineH:            16,
		topLine:          0,
		selecting:        true,
		autoScrollActive: true,
		autoScrollDir:    1,
		autoScrollStep:   4,
	}
	v.syncVisualTop()
	v.topLine = 7

	if animating := v.prepareVisualScroll(now.Add(streamSmoothTick), true); animating {
		t.Fatal("hex selection autoscroll should bypass smooth scrolling")
	}
	if v.visualTop != 7 {
		t.Fatalf("visualTop=%v want snapped target 7", v.visualTop)
	}
	if v.displayTop != 7 || v.displayY != 0 {
		t.Fatalf("display state=%d/%d want snapped 7/0", v.displayTop, v.displayY)
	}
}

func TestHexStopSelectionDragPreservesExistingSelectionRange(t *testing.T) {
	v := &hexViewerState{
		selecting:        true,
		selectID:         4,
		dragAnchor:       32,
		selectionStart:   32,
		selectionLen:     48,
		cancelPending:    true,
		pointerOutside:   true,
		autoScrollActive: true,
	}

	v.stopSelectionDrag()

	if v.selecting || v.selectID != 0 || v.cancelPending || v.pointerOutside || v.autoScrollActive {
		t.Fatalf("selection drag not fully stopped: selecting=%v id=%d cancel=%v outside=%v auto=%v",
			v.selecting, v.selectID, v.cancelPending, v.pointerOutside, v.autoScrollActive)
	}
	if v.selectionStart != 32 || v.selectionLen != 48 {
		t.Fatalf("selection changed unexpectedly: start=%d len=%d", v.selectionStart, v.selectionLen)
	}
}

func TestHexSelectionByteAtPointClampsOutsideViewerArea(t *testing.T) {
	v := &hexViewerState{
		fileSize:     512,
		bytesPerLine: 16,
		topLine:      10,
		visibleLines: 4,
		charW:        8,
		lineH:        16,
		hexRect:      image.Rect(0, 0, 160, 64),
		textRect:     image.Rect(200, 0, 328, 64),
	}

	got, ok := hexSelectionByteAtPoint(v, image.Pt(420, 90))
	if !ok {
		t.Fatal("hexSelectionByteAtPoint should clamp outside points")
	}
	if want := int64(223); got != want {
		t.Fatalf("byte offset=%d want %d", got, want)
	}
}

func TestHexSelectionByteAtPointClampsBelowRenderedContentToLastLine(t *testing.T) {
	v := &hexViewerState{
		fileSize:     40,
		bytesPerLine: 16,
		topLine:      0,
		visibleLines: 4,
		charW:        8,
		lineH:        16,
		hexRect:      image.Rect(0, 0, 160, 64),
		textRect:     image.Rect(200, 0, 328, 64),
	}
	v.syncVisualTop()

	got, ok := hexSelectionByteAtPoint(v, image.Pt(420, 90))
	if !ok {
		t.Fatal("hexSelectionByteAtPoint should clamp short content to the last rendered line")
	}
	if want := int64(39); got != want {
		t.Fatalf("byte offset=%d want %d", got, want)
	}
}

func TestHexByteAtPointUsesDisplayedSmoothScrollState(t *testing.T) {
	v := &hexViewerState{
		fileSize:     512,
		bytesPerLine: 16,
		topLine:      2,
		visibleLines: 2,
		charW:        8,
		lineH:        10,
		hexRect:      image.Rect(0, 0, 160, 20),
		textRect:     image.Rect(200, 0, 328, 20),
	}
	v.visualTop = 1.5
	v.visualReady = true
	v.updateDisplayState()

	if got, want := func() int64 {
		byteOff, _ := hexByteAtPoint(v, image.Pt(201, 1))
		return byteOff
	}(), int64(16); got != want {
		t.Fatalf("byte offset at top=%d want line 1 start %d", got, want)
	}
	if got, want := func() int64 {
		byteOff, _ := hexByteAtPoint(v, image.Pt(201, 6))
		return byteOff
	}(), int64(32); got != want {
		t.Fatalf("byte offset mid viewport=%d want line 2 start %d", got, want)
	}
}

func TestHexViewerRunAutoScrollScrollsAndExtendsSelection(t *testing.T) {
	v := &hexViewerState{
		fileSize:     16000,
		bytesPerLine: 16,
		topLine:      10,
		visibleLines: 4,
		charW:        8,
		lineH:        16,
		hexRect:      image.Rect(0, 0, 160, 64),
		textRect:     image.Rect(200, 0, 328, 64),
		selecting:    true,
		dragAnchor:   160,
	}
	v.setSelectionRange(160, 1)

	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	v.updateAutoScroll(image.Pt(208, 90), now)
	if !v.autoScrollActive {
		t.Fatal("updateAutoScroll should activate outside selection scrolling")
	}
	if !v.runAutoScroll(now) {
		t.Fatal("runAutoScroll should advance when tick is due")
	}
	if got := v.topLine; got != 14 {
		t.Fatalf("topLine=%d want 14", got)
	}
	if v.selectionLen <= 1 {
		t.Fatalf("selectionLen=%d want >1 after autoscroll extension", v.selectionLen)
	}
}

func TestHexRunAutoScrollSnapsVisualTopBeforeRender(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	v := &hexViewerState{
		fileSize:         16000,
		bytesPerLine:     16,
		topLine:          1,
		visibleLines:     4,
		charW:            8,
		lineH:            16,
		hexRect:          image.Rect(0, 0, 160, 64),
		textRect:         image.Rect(200, 0, 328, 64),
		selecting:        true,
		dragAnchor:       16,
		autoScrollActive: true,
		autoScrollDir:    1,
		autoScrollStep:   4,
		autoScrollAt:     now,
		selectPos:        image.Pt(208, 90),
	}
	v.setSelectionRange(16, 1)
	v.syncVisualTop()

	if !v.runAutoScroll(now) {
		t.Fatal("runAutoScroll should advance the hex viewer")
	}
	if v.topLine != 5 {
		t.Fatalf("topLine=%d want 5", v.topLine)
	}
	if v.visualTop != 1 {
		t.Fatalf("visualTop=%v want preserved previous display position 1", v.visualTop)
	}
	if animating := v.prepareVisualScroll(now, true); animating {
		t.Fatal("hex selection autoscroll should not schedule a smooth animation")
	}
	if v.visualTop != 5 {
		t.Fatalf("visualTop=%v want snapped logical top 5", v.visualTop)
	}
}
