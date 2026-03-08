package ui

import (
	"image"
	"testing"
	"time"
)

func TestStreamUpdateAutoScrollDoesNotDelayDueTickOnMove(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	later := now.Add(2 * streamAutoScrollTick)
	v := &streamOutputView{
		lines:            []string{"0", "1", "2", "3", "4", "5"},
		lineH:            16,
		textRect:         image.Rect(0, 0, 120, 64),
		topLine:          1,
		visibleLines:     4,
		selectingText:    true,
		autoScrollActive: true,
		autoScrollDir:    1,
		autoScrollStep:   4,
		autoScrollAt:     now,
	}

	v.updateAutoScroll(image.Pt(40, 90), later)

	if v.autoScrollAt.After(later) {
		t.Fatalf("stream autoScrollAt=%v should stay due at or before %v", v.autoScrollAt, later)
	}
}

func TestStreamUpdateAutoScrollIgnoresOutsideWindowMoveWhileActive(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	v := &streamOutputView{
		lines:            []string{"0", "1", "2", "3", "4", "5"},
		lineH:            16,
		textRect:         image.Rect(0, 0, 120, 64),
		selectingText:    true,
		pointerOutside:   true,
		autoScrollActive: true,
		autoScrollDir:    1,
		autoScrollStep:   4,
		autoScrollAt:     now,
		selectPos:        image.Pt(40, 90),
	}

	v.updateAutoScroll(image.Pt(10, -200), now.Add(streamAutoScrollTick))

	if got := v.selectPos; got != image.Pt(40, 90) {
		t.Fatalf("stream selectPos=%v want preserved %v", got, image.Pt(40, 90))
	}
	if v.autoScrollDir != 1 || v.autoScrollStep != 4 {
		t.Fatalf("stream autoscroll changed to dir=%d step=%d", v.autoScrollDir, v.autoScrollStep)
	}
}

func TestHexUpdateAutoScrollDoesNotDelayDueTickOnMove(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	later := now.Add(2 * streamAutoScrollTick)
	v := &hexViewerState{
		fileSize:         4096,
		bytesPerLine:     16,
		lineH:            16,
		hexRect:          image.Rect(0, 0, 160, 64),
		textRect:         image.Rect(200, 0, 328, 64),
		selecting:        true,
		autoScrollActive: true,
		autoScrollDir:    1,
		autoScrollStep:   4,
		autoScrollAt:     now,
	}

	v.updateAutoScroll(image.Pt(220, 90), later)

	if v.autoScrollAt.After(later) {
		t.Fatalf("hex autoScrollAt=%v should stay due at or before %v", v.autoScrollAt, later)
	}
}

func TestHexUpdateAutoScrollIgnoresOutsideWindowMoveWhileActive(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	v := &hexViewerState{
		fileSize:         4096,
		bytesPerLine:     16,
		lineH:            16,
		hexRect:          image.Rect(0, 0, 160, 64),
		textRect:         image.Rect(200, 0, 328, 64),
		selecting:        true,
		pointerOutside:   true,
		autoScrollActive: true,
		autoScrollDir:    1,
		autoScrollStep:   4,
		autoScrollAt:     now,
		selectPos:        image.Pt(220, 90),
	}

	v.updateAutoScroll(image.Pt(10, -200), now.Add(streamAutoScrollTick))

	if got := v.selectPos; got != image.Pt(220, 90) {
		t.Fatalf("hex selectPos=%v want preserved %v", got, image.Pt(220, 90))
	}
	if v.autoScrollDir != 1 || v.autoScrollStep != 4 {
		t.Fatalf("hex autoscroll changed to dir=%d step=%d", v.autoScrollDir, v.autoScrollStep)
	}
}
