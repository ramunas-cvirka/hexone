// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build pdfium

package ui

import (
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/multi_threaded"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

const (
	viewerPDFiumWorkerEnv          = "HEXONE_PDFIUM_WORKER"
	viewerPDFiumWorkerBaseName     = "hexone-pdfium-worker"
	viewerPDFiumWorkerStartTimeout = 15 * time.Second
	viewerPDFiumWorkerPoolTimeout  = 30 * time.Second
)

func init() {
	viewerPDFPreviewBackend = &viewerPDFiumRenderer{}
	viewerPDFPreviewUsesLocalPath = runtime.GOOS != "windows"
}

type viewerPDFiumRenderer struct {
	resolveOnce sync.Once
	workerPath  string
	resolveErr  error

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
	instance, err := pool.GetInstance(viewerPDFiumWorkerPoolTimeout)
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
	defer rendered.Cleanup()

	size := image.Pt(rendered.Result.Width, rendered.Result.Height)
	return viewerPDFRenderResult{
		Image:     rendered.Result.Image,
		Page:      req.Page,
		PageCount: pageCount.PageCount,
		Size:      size,
	}, nil
}

func (r *viewerPDFiumRenderer) poolInstance() (pdfium.Pool, error) {
	r.initOnce.Do(func() {
		if runtime.GOOS == "windows" {
			r.pool, r.poolErr = webassembly.Init(webassembly.Config{
				MaxIdle:      1,
				MaxTotal:     1,
				ReuseWorkers: true,
				Stdout:       io.Discard,
				Stderr:       io.Discard,
			})
			return
		}

		workerPath, err := r.worker()
		if err != nil {
			r.poolErr = err
			return
		}
		r.pool = multi_threaded.Init(multi_threaded.Config{
			MaxIdle:  1,
			MaxTotal: 1,
			Command: multi_threaded.Command{
				BinPath:      workerPath,
				StartTimeout: viewerPDFiumWorkerStartTimeout,
			},
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

func (r *viewerPDFiumRenderer) worker() (string, error) {
	r.resolveOnce.Do(func() {
		r.workerPath, r.resolveErr = viewerResolvePDFiumWorker()
	})
	return r.workerPath, r.resolveErr
}

func viewerResolvePDFiumWorker() (string, error) {
	if override := strings.TrimSpace(os.Getenv(viewerPDFiumWorkerEnv)); override != "" {
		return viewerExistingPDFiumWorkerPath(override)
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}

	workerName := viewerPDFiumWorkerBaseName
	if runtime.GOOS == "windows" {
		workerName += ".exe"
	}

	return viewerExistingPDFiumWorkerPath(filepath.Join(filepath.Dir(executablePath), workerName))
}

func viewerExistingPDFiumWorkerPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve pdf preview worker %q: %w", path, err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("pdf preview worker is missing: %s", absolute)
	}
	if info.IsDir() {
		return "", fmt.Errorf("pdf preview worker path is a directory: %s", absolute)
	}
	return absolute, nil
}
