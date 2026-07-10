// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build pdfium && pdfverify

// Headless verification driver for PDF link-annotation navigation: opens a
// real PDF with TOC link annotations, clicks a TOC line, and checks the
// view jumps to the link target. Run with:
//
//	PDF_DRIVE_OUT=<dir> PDF_DRIVE_FILE=<pdf> go test -tags "pdfium pdfverify" ./ui/ -run TestHeadlessPDFDocLinkClick -v
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
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"hexone/fm"
)

func TestHeadlessPDFDocLinkClick(t *testing.T) {
	outDir := os.Getenv("PDF_DRIVE_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	pdfPath := os.Getenv("PDF_DRIVE_FILE")
	if pdfPath == "" {
		t.Skip("PDF_DRIVE_FILE not set")
	}
	tocPage := 8 // page with TOC link annotations in the sample document

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

	// Jump to the TOC page and wait for its link annotations to load.
	v := &st.pdfDoc
	v.scrollToPage(tocPage)
	v.syncVisualScroll()
	var links []viewerPDFPageLink
	for i := 0; i < 300; i++ {
		frame(time.Now())
		if l, ok := v.links[tocPage]; ok && len(l) > 0 {
			links = l
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	if len(links) == 0 {
		t.Fatalf("no link annotations loaded for page %d", tocPage)
	}
	img := pump(500 * time.Millisecond)
	shoot(img, "toc-page")

	// Widget-local center of the first TOC link.
	link := links[0]
	origin := v.docOrigin()
	px, py, _, _ := v.pageDocRect(tocPage)
	localX := float32(origin.X) + float32(px+(link.Left+link.Right)/2*v.layoutScale)
	localY := float32(origin.Y) + float32(py+(link.Top+link.Bottom)/2*v.layoutScale)

	// Queued positions are window coordinates while the doc view rects are
	// widget-local; scan plausible header offsets until the hover lands on
	// the link.
	var clickPos f32.Point
	found := false
	for dy := float32(0); dy <= 120; dy += 4 {
		pos := f32.Pt(localX, localY+dy)
		router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pos})
		frame(time.Now())
		if v.hoverLink {
			clickPos = pos
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no window position hovered the TOC link at local (%f,%f)", localX, localY)
	}
	t.Logf("hovering TOC link -> dest page %d at %v", link.DestPage, clickPos)

	pageBefore := v.currentPage()
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Position: clickPos, Buttons: pointer.ButtonPrimary})
	frame(time.Now())
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: clickPos})
	img = pump(600 * time.Millisecond)
	shoot(img, "after-link-click")
	t.Logf("click: page %d -> %d (want %d) label=%q", pageBefore, v.currentPage(), link.DestPage, viewerPDFPageLabel(st))
	if got := v.currentPage(); got != link.DestPage {
		t.Errorf("currentPage=%d want %d after clicking the TOC link", got, link.DestPage)
	}

	// A drag that starts on a link must select text, not navigate. Start
	// over the entry title at the link's left edge (the line center falls
	// on dot leaders whose glyph boxes are too small to hit) and wait out
	// the double-click window first.
	v.scrollToPage(tocPage)
	v.syncVisualScroll()
	pump(500 * time.Millisecond)
	windowOffsetY := clickPos.Y - localY
	dragPos := f32.Pt(
		float32(origin.X)+float32(px+(link.Left+8)*v.layoutScale),
		float32(origin.Y)+float32(py+(link.Top+link.Bottom)/2*v.layoutScale)+windowOffsetY,
	)
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Position: dragPos, Buttons: pointer.ButtonPrimary})
	frame(time.Now())
	for i := 1; i <= 8; i++ {
		router.Queue(pointer.Event{
			Kind:     pointer.Move,
			Source:   pointer.Mouse,
			Position: f32.Pt(dragPos.X+float32(i)*12, dragPos.Y),
			Buttons:  pointer.ButtonPrimary,
		})
		frame(time.Now())
	}
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(dragPos.X+96, dragPos.Y)})
	img = pump(300 * time.Millisecond)
	shoot(img, "drag-on-link-selects")
	t.Logf("drag over link: page=%d selection=%q", v.currentPage(), v.selectedText())
	if got := v.currentPage(); got != tocPage {
		t.Errorf("drag over a link navigated away: page=%d want %d", got, tocPage)
	}
	if !v.hasSelection() {
		t.Error("drag starting on a link did not select text")
	}
}
