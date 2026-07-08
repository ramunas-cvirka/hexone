// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build pdfium

package ui

import (
	"fmt"
	"image"
	"io"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

const viewerPDFiumPoolTimeout = 30 * time.Second

func init() {
	viewerPDFPreviewBackend = &viewerPDFiumRenderer{}
	viewerPDFPreviewUsesLocalPath = false
}

type viewerPDFiumRenderer struct {
	initOnce sync.Once
	pool     pdfium.Pool
	poolErr  error
}

func (r *viewerPDFiumRenderer) Available() bool {
	_, err := r.poolInstance()
	return err == nil
}

// withDocument opens the requested document on a pooled instance and calls
// fn with the open handles. The document and instance are released on return.
func (r *viewerPDFiumRenderer) withDocument(req viewerPDFRenderRequest, fn func(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, pageCount int) error) error {
	if req.LocalPath == "" && len(req.Data) == 0 {
		return fmt.Errorf("pdf preview data is empty")
	}

	pool, err := r.poolInstance()
	if err != nil {
		return err
	}
	instance, err := pool.GetInstance(viewerPDFiumPoolTimeout)
	if err != nil {
		return err
	}
	defer instance.Close()

	openReq := &requests.OpenDocument{}
	if req.LocalPath != "" {
		openReq.FilePath = &req.LocalPath
	} else {
		openReq.File = &req.Data
	}
	doc, err := instance.OpenDocument(openReq)
	if err != nil {
		return err
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	if err != nil {
		return err
	}
	if pageCount.PageCount < 1 {
		return fmt.Errorf("pdf has no pages")
	}
	return fn(instance, doc.Document, pageCount.PageCount)
}

func (r *viewerPDFiumRenderer) RenderPage(req viewerPDFRenderRequest) (viewerPDFRenderResult, error) {
	if req.Width <= 0 {
		req.Width = viewerPDFPreviewTargetWidthPx
	}
	var result viewerPDFRenderResult
	err := r.withDocument(req, func(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, pageCount int) error {
		if req.Page < 0 || req.Page >= pageCount {
			return fmt.Errorf("pdf page %d is out of range", req.Page+1)
		}
		rendered, err := instance.RenderPageInPixels(&requests.RenderPageInPixels{
			Width: req.Width,
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: doc,
					Index:    req.Page,
				},
			},
		})
		if err != nil {
			return err
		}
		// The WebAssembly backend's image Pix slice points directly into WASM
		// linear memory. Cleanup() calls FPDFBitmap_Destroy which can free or
		// reuse that memory. Copy pixels into a Go-owned image before cleanup
		// so the returned image remains valid after this function returns.
		src := rendered.Result.Image
		img := image.NewRGBA(src.Bounds())
		copy(img.Pix, src.Pix)
		rendered.Cleanup()

		result = viewerPDFRenderResult{
			Image:     img,
			Page:      req.Page,
			PageCount: pageCount,
			Size:      image.Pt(rendered.Result.Width, rendered.Result.Height),
		}
		return nil
	})
	if err != nil {
		return viewerPDFRenderResult{}, err
	}
	return result, nil
}

func (r *viewerPDFiumRenderer) DocInfo(req viewerPDFRenderRequest) (viewerPDFDocInfo, error) {
	var info viewerPDFDocInfo
	err := r.withDocument(req, func(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, pageCount int) error {
		info.PageCount = pageCount
		info.PageSizes = make([]viewerPDFPageSize, 0, pageCount)
		for i := 0; i < pageCount; i++ {
			size, err := instance.FPDF_GetPageSizeByIndex(&requests.FPDF_GetPageSizeByIndex{
				Document: doc,
				Index:    i,
			})
			if err != nil {
				return err
			}
			info.PageSizes = append(info.PageSizes, viewerPDFPageSize{
				W: size.Width,
				H: size.Height,
			})
		}
		return nil
	})
	if err != nil {
		return viewerPDFDocInfo{}, err
	}
	return info, nil
}

func (r *viewerPDFiumRenderer) PageText(req viewerPDFRenderRequest) (viewerPDFPageText, error) {
	var text viewerPDFPageText
	err := r.withDocument(req, func(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, pageCount int) error {
		if req.Page < 0 || req.Page >= pageCount {
			return fmt.Errorf("pdf page %d is out of range", req.Page+1)
		}
		size, err := instance.FPDF_GetPageSizeByIndex(&requests.FPDF_GetPageSizeByIndex{
			Document: doc,
			Index:    req.Page,
		})
		if err != nil {
			return err
		}
		structured, err := instance.GetPageTextStructured(&requests.GetPageTextStructured{
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: doc,
					Index:    req.Page,
				},
			},
			Mode: requests.GetPageTextStructuredModeChars,
		})
		if err != nil {
			return err
		}
		text.Page = req.Page
		text.Chars = make([]viewerPDFTextChar, 0, len(structured.Chars))
		for _, ch := range structured.Chars {
			if ch == nil || ch.Text == "" {
				continue
			}
			runes := []rune(ch.Text)
			// pdfium char boxes use a bottom-left origin; flip to top-left.
			text.Chars = append(text.Chars, viewerPDFTextChar{
				Rune:   runes[0],
				Left:   ch.PointPosition.Left,
				Top:    size.Height - ch.PointPosition.Top,
				Right:  ch.PointPosition.Right,
				Bottom: size.Height - ch.PointPosition.Bottom,
			})
		}
		return nil
	})
	if err != nil {
		return viewerPDFPageText{}, err
	}
	return text, nil
}

func (r *viewerPDFiumRenderer) poolInstance() (pdfium.Pool, error) {
	r.initOnce.Do(func() {
		r.pool, r.poolErr = webassembly.Init(webassembly.Config{
			MaxIdle:      2,
			MaxTotal:     3,
			ReuseWorkers: true,
			Stdout:       io.Discard,
			Stderr:       io.Discard,
		})
	})
	if r.poolErr != nil {
		return nil, r.poolErr
	}
	if r.pool == nil {
		return nil, fmt.Errorf("pdf preview backend is unavailable")
	}
	return r.pool, nil
}
