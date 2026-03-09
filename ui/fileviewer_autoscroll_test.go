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

func TestStreamAutoScrollParamsUseDistanceTiers(t *testing.T) {
	v := &streamOutputView{
		lines:        []string{"0", "1", "2", "3", "4", "5"},
		lineH:        16,
		textRect:     image.Rect(0, 0, 120, 64),
		visibleLines: 4,
	}

	nearDir, nearStep := v.autoScrollParams(image.Pt(40, 70))
	farDir, farStep := v.autoScrollParams(image.Pt(40, 150))

	if nearDir != 1 || farDir != 1 {
		t.Fatalf("stream autoscroll dirs near=%d far=%d want both 1", nearDir, farDir)
	}
	if nearStep >= farStep {
		t.Fatalf("stream autoscroll steps near=%d far=%d want far > near", nearStep, farStep)
	}
}

func TestStreamUpdateAutoScrollOutsideWindowMoveUpdatesDirectionAndSpeed(t *testing.T) {
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

	if got := v.selectPos; got != image.Pt(10, -200) {
		t.Fatalf("stream selectPos=%v want updated %v", got, image.Pt(10, -200))
	}
	if v.autoScrollDir != -1 || v.autoScrollStep != 7 {
		t.Fatalf("stream autoscroll dir=%d step=%d want -1/7", v.autoScrollDir, v.autoScrollStep)
	}
}

func TestStreamUpdateAutoScrollStopsImmediatelyOnReentry(t *testing.T) {
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

	v.updateAutoScroll(image.Pt(40, 30), now.Add(streamAutoScrollTick))

	if v.autoScrollActive || v.autoScrollDir != 0 || v.autoScrollStep != 0 {
		t.Fatalf("stream autoscroll should stop on reentry, got active=%v dir=%d step=%d", v.autoScrollActive, v.autoScrollDir, v.autoScrollStep)
	}
	if got := v.selectPos; got != image.Pt(40, 30) {
		t.Fatalf("stream selectPos=%v want reentry point %v", got, image.Pt(40, 30))
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

func TestHexAutoScrollParamsUseDistanceTiers(t *testing.T) {
	v := &hexViewerState{
		fileSize:     4096,
		bytesPerLine: 16,
		lineH:        16,
		hexRect:      image.Rect(0, 0, 160, 64),
		textRect:     image.Rect(200, 0, 328, 64),
		visibleLines: 4,
	}

	nearDir, nearStep := v.autoScrollParams(image.Pt(220, 70))
	farDir, farStep := v.autoScrollParams(image.Pt(220, 150))

	if nearDir != 1 || farDir != 1 {
		t.Fatalf("hex autoscroll dirs near=%d far=%d want both 1", nearDir, farDir)
	}
	if nearStep >= farStep {
		t.Fatalf("hex autoscroll steps near=%d far=%d want far > near", nearStep, farStep)
	}
}

func TestHexUpdateAutoScrollOutsideWindowMoveUpdatesDirectionAndSpeed(t *testing.T) {
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

	if got := v.selectPos; got != image.Pt(10, -200) {
		t.Fatalf("hex selectPos=%v want updated %v", got, image.Pt(10, -200))
	}
	if v.autoScrollDir != -1 || v.autoScrollStep != 7 {
		t.Fatalf("hex autoscroll dir=%d step=%d want -1/7", v.autoScrollDir, v.autoScrollStep)
	}
}

func TestHexUpdateAutoScrollStopsImmediatelyOnReentry(t *testing.T) {
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

	v.updateAutoScroll(image.Pt(220, 30), now.Add(streamAutoScrollTick))

	if v.autoScrollActive || v.autoScrollDir != 0 || v.autoScrollStep != 0 {
		t.Fatalf("hex autoscroll should stop on reentry, got active=%v dir=%d step=%d", v.autoScrollActive, v.autoScrollDir, v.autoScrollStep)
	}
	if got := v.selectPos; got != image.Pt(220, 30) {
		t.Fatalf("hex selectPos=%v want reentry point %v", got, image.Pt(220, 30))
	}
}
