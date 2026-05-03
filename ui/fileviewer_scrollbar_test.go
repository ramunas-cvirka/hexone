// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"testing"
)

func TestViewerScrollbarThumbUsesInsetGeometry(t *testing.T) {
	track := image.Rect(100, 0, 110, 200)

	thumb := viewerScrollbarThumbForScroll(track, 20, 100, 40, true)

	if thumb.Min.X <= track.Min.X || thumb.Max.X >= track.Max.X {
		t.Fatalf("thumb=%v want inset within track=%v", thumb, track)
	}
	if thumb.Dy() < fileViewerScrollbarMinThumbPx {
		t.Fatalf("thumb height=%d want at least %d", thumb.Dy(), fileViewerScrollbarMinThumbPx)
	}
}

func TestViewerHorizontalScrollbarThumbUsesInsetGeometry(t *testing.T) {
	track := image.Rect(0, 120, 240, 130)

	thumb := viewerScrollbarThumbForScroll(track, 30, 120, 45, false)

	if thumb.Min.Y <= track.Min.Y || thumb.Max.Y >= track.Max.Y {
		t.Fatalf("thumb=%v want inset within track=%v", thumb, track)
	}
	if thumb.Dx() < fileViewerScrollbarMinThumbPx {
		t.Fatalf("thumb width=%d want at least %d", thumb.Dx(), fileViewerScrollbarMinThumbPx)
	}
}
