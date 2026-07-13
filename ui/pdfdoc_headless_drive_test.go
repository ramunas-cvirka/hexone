// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build pdfium && pdfverify

// Temporary headless verification driver: renders the real UI with the real
// pdfium backend into a headless GPU window, drives it with router events,
// and dumps PNG frames for inspection. Run with:
//
//	go test -tags "pdfium pdfverify" ./ui/ -run TestHeadlessPDFDocDrive -v
package ui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"hexone/fm"
)

func TestHeadlessPDFDocDrive(t *testing.T) {
	outDir := os.Getenv("PDF_DRIVE_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	pdfPath := os.Getenv("PDF_DRIVE_FILE")
	if pdfPath == "" {
		t.Skip("PDF_DRIVE_FILE not set")
	}

	const width, height = 1100, 800
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("headless window: %v", err)
	}
	defer win.Release()

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	router := new(input.Router)

	shotIdx := 0
	frame := func(now time.Time) *image.RGBA {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)),
			Now:         now,
			Source:      router.Source(),
		}
		ui.Layout(th, gtx)
		router.Frame(&ops)
		if err := win.Frame(&ops); err != nil {
			t.Fatalf("frame: %v", err)
		}
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		if err := win.Screenshot(img); err != nil {
			t.Fatalf("screenshot: %v", err)
		}
		return img
	}
	shoot := func(img *image.RGBA, name string) {
		shotIdx++
		path := filepath.Join(outDir, fmt.Sprintf("%02d-%s.png", shotIdx, name))
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatalf("encode %s: %v", path, err)
		}
		f.Close()
		t.Logf("wrote %s", path)
	}
	pump := func(d time.Duration) *image.RGBA {
		deadline := time.Now().Add(d)
		var img *image.RGBA
		for time.Now().Before(deadline) {
			img = frame(time.Now())
			time.Sleep(15 * time.Millisecond)
		}
		return img
	}

	// Navigate pane 0 to the sample directory through the real async load
	// path with the PDF preselected, then open the internal viewer.
	pane := ui.filePanes[0]
	dir := filepath.Dir(pdfPath)
	ui.requestPaneLoadWithSelection(0, dir, pdfPath, "", 0)
	selected := false
	for i := 0; i < 200; i++ {
		frame(time.Now())
		if pane.dir == dir {
			if entry := pane.selectedEntry(); entry != nil && entry.Path == pdfPath {
				selected = true
				break
			}
		}
		time.Sleep(15 * time.Millisecond)
	}
	if !selected {
		t.Fatalf("pane did not select sample pdf: dir=%q notice=%q", pane.dir, pane.noticeText)
	}
	ui.startFileViewer(0, time.Now())
	if ui.fileViewer == nil {
		t.Fatalf("viewer did not open: notice=%q", pane.noticeText)
	}

	// Wait for the async load + first page render.
	var st *fileViewerState
	for i := 0; i < 300; i++ {
		frame(time.Now())
		st = ui.fileViewer
		if st != nil && viewerPDFPreviewActive(st) && len(st.pdfDoc.pages) > 0 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	if st == nil || !viewerPDFPreviewActive(st) {
		t.Fatalf("viewer did not enter PDF mode: st=%+v", st)
	}
	img := pump(700 * time.Millisecond)
	shoot(img, "opened-fit")
	t.Logf("state: pages=%d zoom=%.2f contentH=%.1f viewport=%v scale=%.4f label=%q",
		st.imagePreviewPageCount, st.pdfDoc.effectiveZoom(), st.pdfDoc.contentH,
		st.pdfDoc.viewportRect, st.pdfDoc.layoutScale, viewerPDFPageLabel(st))
	if got, want := st.pdfDoc.layoutWidths[0], float64(st.pdfDoc.viewportRect.Dx()); got < want-1 || got > want+1 {
		t.Errorf("page width=%f want %f (fit width at zoom 1)", got, want)
	}

	center := f32.Pt(float32(st.pdfDoc.viewportRect.Min.X+st.pdfDoc.viewportRect.Dx()/2),
		float32(st.pdfDoc.viewportRect.Min.Y+200))

	// Wheel-scroll down across the first page boundary.
	for i := 0; i < 30; i++ {
		router.Queue(pointer.Event{
			Kind:     pointer.Scroll,
			Source:   pointer.Mouse,
			Position: center,
			Scroll:   f32.Pt(0, 10),
		})
		frame(time.Now())
	}
	img = pump(400 * time.Millisecond)
	shoot(img, "wheel-scrolled")
	t.Logf("after wheel: scrollY=%.1f label=%q", st.pdfDoc.scrollY, viewerPDFPageLabel(st))
	if st.pdfDoc.scrollY <= 0 {
		t.Error("wheel scroll did not move the document")
	}

	// Drag-pan: the document is at its end, so dragging the content
	// downward must scroll back up (content follows the pointer). Press on
	// the empty left page margin so the drag pans instead of selecting.
	panPos := f32.Pt(40, center.Y)
	scrollBefore := st.pdfDoc.scrollY
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Position: panPos, Buttons: pointer.ButtonPrimary})
	frame(time.Now())
	if !st.pdfDoc.panning {
		t.Error("press on the empty page margin should start panning")
	}
	if st.pdfDoc.selecting {
		t.Error("press on the empty page margin must not start a selection")
	}
	center = panPos
	for i := 1; i <= 10; i++ {
		router.Queue(pointer.Event{
			Kind:     pointer.Move,
			Source:   pointer.Mouse,
			Position: f32.Pt(center.X, center.Y+float32(i)*12),
			Buttons:  pointer.ButtonPrimary,
		})
		frame(time.Now())
	}
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(center.X, center.Y+120)})
	img = pump(200 * time.Millisecond)
	shoot(img, "drag-panned")
	t.Logf("after pan: scrollY=%.1f (before %.1f) label=%q", st.pdfDoc.scrollY, scrollBefore, viewerPDFPageLabel(st))
	if st.pdfDoc.scrollY >= scrollBefore-100 {
		t.Errorf("drag pan moved %.1f px, want ~-120", st.pdfDoc.scrollY-scrollBefore)
	}

	// Jump to the end via the End key path.
	ui.performFileViewerKeyScroll(time.Now(), key.NameEnd)
	img = pump(700 * time.Millisecond)
	shoot(img, "end-of-document")
	t.Logf("after End: scrollY=%.1f label=%q", st.pdfDoc.scrollY, viewerPDFPageLabel(st))
	if got := viewerPDFPageLabel(st); got != "Page 5/5" {
		t.Errorf("label=%q want Page 5/5 at document end", got)
	}

	// Back to start, zoom in, verify horizontal scrollbar appears.
	ui.performFileViewerKeyScroll(time.Now(), key.NameHome)
	st.pdfDoc.zoomBy(2)
	img = pump(700 * time.Millisecond)
	shoot(img, "zoomed-2x")
	t.Logf("after zoom: zoom=%.2f contentW=%.1f viewportW=%d", st.pdfDoc.effectiveZoom(), st.pdfDoc.contentW, st.pdfDoc.viewportRect.Dx())

	// Plain drag over text must select it. Queued event positions are
	// window coordinates while the doc view's rects are widget-local, so
	// probe down the left text column with real presses until one lands on
	// a glyph and starts a selection.
	st.pdfDoc.resetZoom()
	frame(time.Now())
	probeX := float32(200)
	var selStart f32.Point
	for y := float32(80); y < float32(height)-60; y += 8 {
		pos := f32.Pt(probeX, y)
		router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Position: pos, Buttons: pointer.ButtonPrimary})
		frame(time.Now())
		selecting := st.pdfDoc.selecting
		if !selecting {
			router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pos})
			frame(time.Now())
			continue
		}
		selStart = pos
		break
	}
	if !st.pdfDoc.selecting {
		t.Fatal("no probe press over the text column started a selection")
	}
	selEnd := f32.Pt(selStart.X+320, selStart.Y+40)
	steps := 8
	for i := 1; i <= steps; i++ {
		frac := float32(i) / float32(steps)
		pos := f32.Pt(selStart.X+(selEnd.X-selStart.X)*frac, selStart.Y+(selEnd.Y-selStart.Y)*frac)
		router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pos, Buttons: pointer.ButtonPrimary})
		frame(time.Now())
	}
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: selEnd})
	img = pump(400 * time.Millisecond)
	shoot(img, "text-selected")
	t.Logf("selection active=%v text=%q", st.pdfDoc.hasSelection(), st.pdfDoc.selectedText())
	if !st.pdfDoc.hasSelection() {
		t.Error("plain drag over text did not produce a selection")
	}

	// Double-click on the same glyph selects the whole word.
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Position: selStart, Buttons: pointer.ButtonPrimary})
	frame(time.Now())
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: selStart})
	frame(time.Now())
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Position: selStart, Buttons: pointer.ButtonPrimary})
	frame(time.Now())
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: selStart})
	img = pump(200 * time.Millisecond)
	shoot(img, "double-click-word")
	word := st.pdfDoc.selectedText()
	t.Logf("double-click selection=%q", word)
	if word == "" {
		t.Error("double-click did not select a word")
	}
	for _, r := range word {
		if !pdfDocWordRune(r) {
			t.Errorf("double-click selection %q contains non-word rune %q", word, r)
			break
		}
	}

	// Selection auto-scroll: hold a selection drag past the bottom window
	// edge and let the document scroll under the pointer. Wait out the
	// double-click window first so the press starts a fresh drag.
	pump(500 * time.Millisecond)
	scrollBefore = st.pdfDoc.scrollY
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Position: selStart, Buttons: pointer.ButtonPrimary})
	frame(time.Now())
	if !st.pdfDoc.selecting {
		t.Fatal("press on text did not start a selection drag for auto-scroll")
	}
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(selStart.X, float32(height)+80), Buttons: pointer.ButtonPrimary})
	img = pump(1200 * time.Millisecond)
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(selStart.X, float32(height)+80)})
	frame(time.Now())
	shoot(img, "selection-autoscroll")
	t.Logf("auto-scroll: scrollY %.1f -> %.1f selection ends on page %d text len=%d",
		scrollBefore, st.pdfDoc.scrollY, st.pdfDoc.selEnd.Page, len(st.pdfDoc.selectedText()))
	if st.pdfDoc.scrollY < scrollBefore+300 {
		t.Errorf("auto-scroll moved only %.1f px, want > 300", st.pdfDoc.scrollY-scrollBefore)
	}
	if len(st.pdfDoc.selectedText()) < 40 {
		t.Errorf("auto-scroll selection too short: %q", st.pdfDoc.selectedText())
	}

	// Copy through the real copy path.
	var gtxOps op.Ops
	gtx := layout.Context{
		Ops:         &gtxOps,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(width, height)),
		Now:         time.Now(),
		Source:      router.Source(),
	}
	if !ui.copyFileViewerText(gtx, false) {
		t.Error("copyFileViewerText failed with an active PDF selection")
	}
}
