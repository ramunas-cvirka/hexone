---
name: verify
description: How to verify hexone UI changes end-to-end by rendering the real UI headlessly and capturing PNG frames
---

# Verifying hexone (Gio desktop app)

hexone is a Gio (gioui.org) desktop app. On macOS the sandboxed shell has no
Screen Recording permission, so `screencapture` of the live window fails
("could not create image from display"). Use the headless GPU backend
instead: it renders the real UI (real layout, router events, async loads,
pdfium) into `gioui.org/gpu/headless` and reads back actual frames.

## Recipe

1. Write a driver as an in-package test in `ui/` behind a dedicated build
   tag so it never runs in CI, e.g. `//go:build pdfium && pdfverify`.
   A working example: `ui/pdfdoc_headless_drive_test.go`
   (drives the PDF viewer: open, wheel scroll, drag-pan, zoom, text select).
2. Core loop:
   - `win, _ := headless.NewWindow(w, h)`
   - `th := material.NewTheme(); th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))`
   - `ui := NewUI(fm.DefaultConfig())`, `router := new(input.Router)`
   - per frame: build `layout.Context{Ops, Metric: {1,1}, Constraints: layout.Exact(...), Now, Source: router.Source()}`,
     call `ui.Layout(th, gtx)`, then `router.Frame(&ops)`, `win.Frame(&ops)`,
     `win.Screenshot(img)` and save PNGs.
3. Navigate panes through the real async path:
   `ui.requestPaneLoadWithSelection(0, dir, path, "", 0)`, pump frames until
   `pane.selectedEntry()` matches, then `ui.startFileViewer(0, time.Now())`.
4. Input gotchas:
   - Queue `pointer.Move` (not `pointer.Drag`) while a button is pressed —
     the router synthesizes Drag; queuing raw Drag panics.
   - Queued pointer positions are WINDOW coordinates; widget rects
     (e.g. `st.pdfDoc.viewportRect`) are widget-local. Aim at window-center
     landmarks or account for the header offset.
   - Key handling is easiest via the internal handlers
     (`ui.performFileViewerKeyScroll`) since key focus routing is finicky.
5. Run:
   `PDF_DRIVE_OUT=<dir> PDF_DRIVE_FILE=<pdf> go test -tags "pdfium pdfverify" ./ui/ -run TestHeadlessPDFDocDrive -v`

## Build/test matrix

- default: `go test ./...` (PDF backend is a noop; tests use `fakeViewerPDFRenderer`)
- pdfium: `go test -tags pdfium ./ui/` (real WASM pdfium via klippa-app/go-pdfium)
- app binary: `go build ./cmd/hexone` or `go build -tags pdfium ./cmd/hexone`
- known pre-existing: `go build -tags pdfium ./cmd/hexone-pdfium-worker` fails
  on a missing go.sum entry for go-hclog — unrelated to UI work.
- generate throwaway multi-page text PDFs by hand (see `testPDFWithText` in
  `ui/fileviewer_pdf_pdfium_test.go`) — no external tools needed.
