package ui

import (
	"image"
	"testing"
	"time"
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
	const thumbH = 18
	wantDragYf := float64(40*(160-thumbH)) / float64(totalLines-8)
	wantDragY := int(wantDragYf)
	if dragThumbY != wantDragY {
		t.Fatalf("drag thumb Y=%d, want %d", dragThumbY, wantDragY)
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
