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
	"hexone/fm"
)

func TestImagePreviewViewComputeLayoutAddsScrollbarsWhenNeeded(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 320, 240))
	var v imagePreviewView

	v.computeLayout(image.Pt(160, 120), 0, 10, img)

	if got := v.viewportRect.Min; got != image.Pt(0, 0) {
		t.Fatalf("viewport min=%v want origin", got)
	}
	if v.viewportRect.Dx() >= 160 {
		t.Fatalf("viewport width=%d want smaller than surface width", v.viewportRect.Dx())
	}
	if v.viewportRect.Dy() >= 120 {
		t.Fatalf("viewport height=%d want smaller than surface height", v.viewportRect.Dy())
	}
	if v.vTrackRect.Dx() <= 0 || v.vThumbRect.Dx() <= 0 {
		t.Fatal("expected vertical scrollbar for oversized image")
	}
	if v.hTrackRect.Dy() <= 0 || v.hThumbRect.Dy() <= 0 {
		t.Fatal("expected horizontal scrollbar for oversized image")
	}
}

func TestImagePreviewViewScrollByKeyStepMovesHorizontally(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 400, 300))
	v := imagePreviewView{
		zoom:         1,
		viewportRect: image.Rect(0, 0, 120, 90),
	}

	if !v.scrollByKeyStep(img, 1, 0) {
		t.Fatal("expected horizontal key scroll to move image")
	}
	if got := v.scrollX; got != fileViewerImageKeyStepPx {
		t.Fatalf("scrollX=%d want %d", got, fileViewerImageKeyStepPx)
	}
	if got := v.scrollY; got != 0 {
		t.Fatalf("scrollY=%d want 0", got)
	}
}

func TestImagePreviewViewZoomByKeepsTopLeftAnchor(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 800, 600))
	v := imagePreviewView{
		zoom:         1,
		viewportRect: image.Rect(0, 0, 200, 150),
		scrollX:      120,
		scrollY:      60,
	}
	wantX := float32(v.scrollX) / v.effectiveZoom()
	wantY := float32(v.scrollY) / v.effectiveZoom()

	if !v.zoomBy(img, fileViewerImageZoomFactor) {
		t.Fatal("expected zoom change")
	}
	gotX := float32(v.scrollX) / v.effectiveZoom()
	gotY := float32(v.scrollY) / v.effectiveZoom()
	if math.Abs(float64(gotX-wantX)) > 0.02 {
		t.Fatalf("anchorX=%f want %f", gotX, wantX)
	}
	if math.Abs(float64(gotY-wantY)) > 0.02 {
		t.Fatalf("anchorY=%f want %f", gotY, wantY)
	}
}

func TestImagePreviewViewPrepareVisualScrollAnimatesTowardTarget(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 800, 600))
	now := time.Date(2026, time.April, 8, 8, 0, 0, 0, time.UTC)
	v := imagePreviewView{
		zoom:         1,
		viewportRect: image.Rect(0, 0, 200, 150),
		scrollX:      120,
		scrollY:      80,
	}

	if anim := v.prepareVisualScroll(now, true, img); anim {
		t.Fatal("first prepareVisualScroll call should initialize without animating")
	}

	v.scrollX = 180
	v.scrollY = 140
	anim := v.prepareVisualScroll(now.Add(16*time.Millisecond), true, img)
	if !anim {
		t.Fatal("expected smooth image scroll animation")
	}
	if v.visualX <= 120 || v.visualX >= 180 {
		t.Fatalf("visualX=%f want between 120 and 180", v.visualX)
	}
	if v.visualY <= 80 || v.visualY >= 140 {
		t.Fatalf("visualY=%f want between 80 and 140", v.visualY)
	}
}

func TestViewerImageZoomFactorForKey(t *testing.T) {
	tests := []struct {
		name string
		key  key.Name
		mods key.Modifiers
		want float32
		ok   bool
	}{
		{name: "ctrl plus", key: "+", mods: key.ModCtrl | key.ModShift, want: fileViewerImageZoomFactor, ok: true},
		{name: "ctrl equals", key: "=", mods: key.ModCtrl, want: fileViewerImageZoomFactor, ok: true},
		{name: "cmd minus", key: "-", mods: key.ModShortcut, want: 1 / fileViewerImageZoomFactor, ok: true},
		{name: "plain minus", key: "-", mods: 0, want: 0, ok: false},
	}

	for _, tt := range tests {
		got, ok := viewerImageZoomFactorForKey(tt.key, tt.mods)
		if ok != tt.ok {
			t.Fatalf("%s: ok=%v want %v", tt.name, ok, tt.ok)
		}
		if math.Abs(float64(got-tt.want)) > 0.0001 {
			t.Fatalf("%s: factor=%f want %f", tt.name, got, tt.want)
		}
	}
}

func TestPerformFileViewerKeyScrollMovesImagePreview(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.UTC)
	st := &fileViewerState{
		mode:                 "file",
		detectedImagePreview: true,
		imagePreview:         image.NewNRGBA(image.Rect(0, 0, 400, 300)),
	}
	st.imageView.zoom = 1
	st.imageView.viewportRect = image.Rect(0, 0, 120, 90)
	ui.fileViewer = st

	if !ui.performFileViewerKeyScroll(now, key.NameRightArrow) {
		t.Fatal("expected Right to pan image preview")
	}
	if st.imageView.scrollX <= 0 {
		t.Fatalf("scrollX=%d want > 0", st.imageView.scrollX)
	}
}

func TestLayoutImageOutputViewAppliesPendingScrollToEnd(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		detectedImagePreview:    true,
		imagePreview:            image.NewNRGBA(image.Rect(0, 0, 400, 600)),
		pendingImageScrollToEnd: true,
	}
	router := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(160, 120),
		},
	}

	ui.layoutImageOutputView(nil, gtx, st)

	_, maxY := st.imageView.maxScroll(st.imagePreview)
	if got := st.imageView.scrollY; got != maxY {
		t.Fatalf("scrollY=%d want %d", got, maxY)
	}
	if st.pendingImageScrollToEnd {
		t.Fatal("pendingImageScrollToEnd should be consumed during layout")
	}
}

func TestLayoutImageOutputViewAddsPDFDocumentScrollbar(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		detectedImagePreview:  true,
		imagePreview:          image.NewNRGBA(image.Rect(0, 0, 400, 600)),
		imagePreviewFormat:    "pdf",
		imagePreviewPage:      2,
		imagePreviewPageCount: 8,
	}
	router := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(200, 140),
		},
	}

	ui.layoutImageOutputView(nil, gtx, st)

	if st.imageView.pdfTrackRect.Dx() <= 0 || st.imageView.pdfThumbRect.Dy() <= 0 {
		t.Fatal("expected pdf document scrollbar to be laid out")
	}
	if st.imageView.pdfTrackRect.Min.X <= st.imageView.surfaceRect.Max.X {
		t.Fatalf("pdf doc scrollbar should live in its own gutter, track=%v surface=%v", st.imageView.pdfTrackRect, st.imageView.surfaceRect)
	}
}

func TestImagePreviewViewPDFDocumentPageFromVerticalDrag(t *testing.T) {
	var v imagePreviewView
	track := image.Rect(0, 0, 8, 240)
	v.setPDFDocumentScrollbar(track, 0, 10)

	page, ok := v.pdfDocumentPageFromVerticalDrag(track.Max.Y, v.pdfDocumentThumbGrabY(image.Pt(4, track.Max.Y)), 10)
	if !ok {
		t.Fatal("expected document drag mapping to succeed")
	}
	if page != 9 {
		t.Fatalf("page=%d want 9", page)
	}
}
