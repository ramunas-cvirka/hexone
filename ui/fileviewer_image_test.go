// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"math"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"hexone/fm"
)

func TestImagePreviewInitialZoomFitsOversizedImageAndAlignsTop(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 400, 600))
	var v imagePreviewView
	v.initializeZoom(image.Pt(200, 150), 10, img)
	v.computeLayout(image.Pt(200, 150), 0, 10, 10, img)

	if !v.zoomReady || !v.alignTop {
		t.Fatalf("initial state zoomReady=%v alignTop=%v want true/true", v.zoomReady, v.alignTop)
	}
	if got := v.contentSize(img).X; got != v.viewportRect.Dx() {
		t.Fatalf("fitted image width=%d want viewport width %d", got, v.viewportRect.Dx())
	}
	if got := v.contentOrigin(img).Y; got != v.viewportRect.Min.Y {
		t.Fatalf("fitted image origin Y=%d want top %d", got, v.viewportRect.Min.Y)
	}
	if !v.zoomBy(img, fileViewerImageZoomFactor) {
		t.Fatal("zooming fitted image should change zoom")
	}
	v.computeLayout(image.Pt(200, 150), 0, 10, 10, img)
	if got := v.contentOrigin(img).Y; got != v.viewportRect.Min.Y {
		t.Fatalf("zoomed image origin Y=%d want top %d", got, v.viewportRect.Min.Y)
	}
}

func TestImagePreviewNativeSizeCentersSmallImage(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 80))
	var v imagePreviewView
	v.initializeZoom(image.Pt(300, 200), 10, img)
	v.computeLayout(image.Pt(300, 200), 0, 10, 10, img)

	if got := v.effectiveZoom(); got != 1 {
		t.Fatalf("small image zoom=%v want native size", got)
	}
	if got, want := v.contentOrigin(img), image.Pt(100, 60); got != want {
		t.Fatalf("small image origin=%v want centered %v", got, want)
	}
}

func TestImagePreviewDragPansContent(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		mode:                 "file",
		detectedImagePreview: true,
		imagePreview:         image.NewNRGBA(image.Rect(0, 0, 400, 600)),
	}
	ui.fileViewer = st
	th := material.NewTheme()
	router := new(input.Router)
	now := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(200, 150)},
	}
	frame := func() {
		gtx.Ops.Reset()
		gtx.Now = now
		ui.layoutImageOutputView(th, gtx, st)
		router.Frame(gtx.Ops)
		now = now.Add(16 * time.Millisecond)
	}
	frame()
	st.imageView.scrollY = 100
	st.imageView.syncVisualScroll()

	start := f32.Pt(80, 70)
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Position: start, Buttons: pointer.ButtonPrimary})
	frame()
	if !st.imageView.panning {
		t.Fatal("primary press on the image should start panning")
	}
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(start.X, start.Y+40), Buttons: pointer.ButtonPrimary})
	frame()
	if got := st.imageView.scrollY; got >= 100 {
		t.Fatalf("dragging image down left scrollY=%d want < 100", got)
	}
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(start.X, start.Y+40)})
	frame()
	if st.imageView.panning {
		t.Fatal("release should stop image panning")
	}
}

func TestImagePreviewViewComputeLayoutAddsScrollbarsWhenNeeded(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 320, 240))
	var v imagePreviewView

	v.computeLayout(image.Pt(160, 120), 0, 10, 10, img)

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

func TestViewerZoomPresetsApplyToImagesAndPDFs(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 400, 300))
	imageState := &fileViewerState{detectedImagePreview: true, imagePreview: img}
	imageState.imageView.viewportRect = image.Rect(0, 0, 200, 150)
	imageState.imageView.zoom = 1
	if !applyViewerZoomPreset(imageState, viewerZoomPresets[0]) {
		t.Fatal("fit-width image preset should change zoom")
	}
	if got := imageState.imageView.effectiveZoom(); math.Abs(float64(got-0.5)) > 0.0001 {
		t.Fatalf("image fit-width zoom=%v want 0.5", got)
	}
	if !imageState.imageView.alignTop {
		t.Fatal("fit-width image preset should align the image top")
	}
	if !applyViewerZoomPreset(imageState, viewerZoomPresets[4]) {
		t.Fatal("100% image preset should restore native zoom")
	}

	pdfState := &fileViewerState{detectedImagePreview: true, imagePreviewFormat: "pdf", imagePreviewPageCount: 1}
	pdfState.pdfDoc.viewportRect = image.Rect(0, 0, 200, 150)
	pdfState.pdfDoc.configure(viewerPDFDocInfo{PageCount: 1, PageSizes: []viewerPDFPageSize{{W: 612, H: 792}}})
	pdfState.pdfDoc.setZoom(2)
	if !applyViewerZoomPreset(pdfState, viewerZoomPresets[0]) {
		t.Fatal("fit-width PDF preset should reset zoom")
	}
	if got := pdfState.pdfDoc.effectiveZoom(); got != 1 {
		t.Fatalf("PDF fit-width zoom=%v want 1", got)
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
