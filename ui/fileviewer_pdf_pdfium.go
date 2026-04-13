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

func (r *viewerPDFiumRenderer) RenderPage(req viewerPDFRenderRequest) (viewerPDFRenderResult, error) {
	if req.LocalPath == "" && len(req.Data) == 0 {
		return viewerPDFRenderResult{}, fmt.Errorf("pdf preview data is empty")
	}
	if req.Width <= 0 {
		req.Width = viewerPDFPreviewTargetWidthPx
	}

	pool, err := r.poolInstance()
	if err != nil {
		return viewerPDFRenderResult{}, err
	}
	instance, err := pool.GetInstance(viewerPDFiumPoolTimeout)
	if err != nil {
		return viewerPDFRenderResult{}, err
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
		return viewerPDFRenderResult{}, err
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	if err != nil {
		return viewerPDFRenderResult{}, err
	}
	if pageCount.PageCount < 1 {
		return viewerPDFRenderResult{}, fmt.Errorf("pdf has no pages")
	}
	if req.Page < 0 || req.Page >= pageCount.PageCount {
		return viewerPDFRenderResult{}, fmt.Errorf("pdf page %d is out of range", req.Page+1)
	}

	rendered, err := instance.RenderPageInPixels(&requests.RenderPageInPixels{
		Width: req.Width,
		Page: requests.Page{
			ByIndex: &requests.PageByIndex{
				Document: doc.Document,
				Index:    req.Page,
			},
		},
	})
	if err != nil {
		return viewerPDFRenderResult{}, err
	}
	// The WebAssembly backend's image Pix slice points directly into WASM
	// linear memory. Cleanup() calls FPDFBitmap_Destroy which can free or
	// reuse that memory. Copy pixels into a Go-owned image before cleanup
	// so the returned image remains valid after this function returns.
	src := rendered.Result.Image
	img := image.NewRGBA(src.Bounds())
	copy(img.Pix, src.Pix)
	rendered.Cleanup()

	size := image.Pt(rendered.Result.Width, rendered.Result.Height)
	return viewerPDFRenderResult{
		Image:     img,
		Page:      req.Page,
		PageCount: pageCount.PageCount,
		Size:      size,
	}, nil
}

func (r *viewerPDFiumRenderer) poolInstance() (pdfium.Pool, error) {
	r.initOnce.Do(func() {
		r.pool, r.poolErr = webassembly.Init(webassembly.Config{
			MaxIdle:      1,
			MaxTotal:     1,
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
