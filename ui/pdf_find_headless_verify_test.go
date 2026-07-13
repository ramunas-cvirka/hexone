// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build pdfium && pdfverify

package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"hexone/fm"
)

func testPDFFindVisualDocument() []byte {
	const pageCount = 12
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	writeObject := func(n int, body string) {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", n, body)
	}
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	var kids strings.Builder
	for page := 0; page < pageCount; page++ {
		fmt.Fprintf(&kids, "%d 0 R ", 3+page)
	}
	writeObject(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids.String(), pageCount))
	fontObject := 3 + pageCount
	firstContentObject := fontObject + 1
	for page := 0; page < pageCount; page++ {
		writeObject(3+page, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 420 560] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>", fontObject, firstContentObject+page))
	}
	writeObject(fontObject, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	for page := 0; page < pageCount; page++ {
		line := fmt.Sprintf("Result %02d includes needle with compact supporting context", page+1)
		content := fmt.Sprintf("BT /F1 18 Tf 38 500 Td (Search results) Tj 0 -42 Td /F1 12 Tf (%s) Tj 0 -28 Td (Compact PDF find preview page %d) Tj ET", line, page+1)
		writeObject(firstContentObject+page, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", len(offsets))
	b.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return b.Bytes()
}

func TestHeadlessPDFViewerFind(t *testing.T) {
	if !viewerPDFPreviewBackend.Available() {
		t.Skip("pdfium backend unavailable")
	}
	outDir := os.Getenv("PDF_FIND_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pdfPath := filepath.Join(t.TempDir(), "find-preview.pdf")
	if err := os.WriteFile(pdfPath, testPDFFindVisualDocument(), 0o600); err != nil {
		t.Fatal(err)
	}

	const width, height = 1100, 760
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("headless window: %v", err)
	}
	defer win.Release()
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	ui := NewUI(fm.DefaultConfig())
	router := new(input.Router)
	frame := func() *image.RGBA {
		var ops op.Ops
		gtx := layout.Context{
			Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)), Now: time.Now(), Source: router.Source(),
		}
		ui.Layout(th, gtx)
		router.Frame(&ops)
		if err := win.Frame(&ops); err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		if err := win.Screenshot(img); err != nil {
			t.Fatal(err)
		}
		return img
	}
	pumpUntil := func(deadline time.Time, ready func() bool) *image.RGBA {
		var img *image.RGBA
		for time.Now().Before(deadline) {
			img = frame()
			if ready() {
				return img
			}
			time.Sleep(12 * time.Millisecond)
		}
		return img
	}

	pane := ui.filePanes[0]
	ui.requestPaneLoadWithSelection(0, filepath.Dir(pdfPath), pdfPath, "", 0)
	pumpUntil(time.Now().Add(3*time.Second), func() bool {
		entry := pane.selectedEntry()
		return entry != nil && entry.Path == pdfPath
	})
	ui.startFileViewer(0, time.Now())
	pumpUntil(time.Now().Add(6*time.Second), func() bool {
		return ui.fileViewer != nil && viewerPDFPreviewActive(ui.fileViewer) && ui.fileViewer.pdfDoc.infoLoaded
	})
	st := ui.fileViewer
	if st == nil || !viewerPDFPreviewActive(st) {
		t.Fatal("PDF viewer did not open")
	}

	// Exercise the actual shortcut filter before filling the query.
	frame()
	router.Queue(key.Event{Name: "f", Modifiers: key.ModShortcut, State: key.Press})
	frame()
	if !st.find.open {
		t.Fatal("Cmd/Ctrl+F did not open PDF find")
	}
	st.find.editor.SetText("needle")
	ui.refreshFileViewerFind(time.Now(), false)
	img := pumpUntil(time.Now().Add(8*time.Second), func() bool {
		return !st.find.searching && len(st.find.pdfMatches) == 12
	})
	if st.find.searching || len(st.find.pdfMatches) != 12 {
		t.Fatalf("find state: searching=%v matches=%d status=%q", st.find.searching, len(st.find.pdfMatches), st.find.status)
	}
	// Exercise the last-result jump too, exposing the bottom of the list and
	// the compact "12p" page notation in the verification frame.
	ui.applyFileViewerPDFFindMatch(time.Now(), len(st.find.pdfMatches)-1)
	img = pumpUntil(time.Now().Add(3*time.Second), func() bool {
		page, ok := st.pdfDoc.pages[11]
		return st.find.index == 11 && ok && page.img != nil
	})
	// One more frame paints the completed compact result list.
	img = frame()
	outPath := filepath.Join(outDir, "pdf-find-results.png")
	f, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", outPath)

	// Reproduce the real transition that previously left Find routed to the
	// empty PDF result slice even though Hex matches had completed.
	st.find.editor.SetText("%PDF")
	ui.setFileViewerMode("hex", time.Now())
	img = pumpUntil(time.Now().Add(8*time.Second), func() bool {
		return st.mode == "hex" && !st.loading && !st.find.searching && len(st.find.hexMatches) > 0
	})
	if viewerPDFPreviewActive(st) || st.mode != "hex" || len(st.find.hexMatches) == 0 {
		t.Fatalf("PDF to Hex find transition: pdfActive=%v mode=%q hexMatches=%d status=%q", viewerPDFPreviewActive(st), st.mode, len(st.find.hexMatches), st.find.status)
	}
	if got := fileViewerFindResultCount(st); got != len(st.find.hexMatches) {
		t.Fatalf("visible result count=%d want Hex count %d", got, len(st.find.hexMatches))
	}
	img = frame()
	outPath = filepath.Join(outDir, "pdf-to-hex-find-results.png")
	f, err = os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", outPath)
}
