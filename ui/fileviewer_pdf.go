// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"image"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
)

const viewerPDFPreviewTargetWidthPx = 1600

type viewerPDFRenderRequest struct {
	Data      []byte
	LocalPath string
	Page      int
	Width     int
}

type viewerPDFRenderResult struct {
	Image     image.Image
	Page      int
	PageCount int
	Size      image.Point
}

// viewerPDFPageSize is a native page size in PDF points.
type viewerPDFPageSize struct {
	W float64
	H float64
}

type viewerPDFDocInfo struct {
	PageCount int
	PageSizes []viewerPDFPageSize
}

// viewerPDFTextChar is one character of page text. The rect is in page
// points with the origin at the top-left corner of the page (already
// flipped from pdfium's bottom-left origin).
type viewerPDFTextChar struct {
	Rune   rune
	Left   float64
	Top    float64
	Right  float64
	Bottom float64
}

type viewerPDFPageText struct {
	Page  int
	Chars []viewerPDFTextChar
}

type viewerPDFRenderer interface {
	Available() bool
	RenderPage(req viewerPDFRenderRequest) (viewerPDFRenderResult, error)
	DocInfo(req viewerPDFRenderRequest) (viewerPDFDocInfo, error)
	PageText(req viewerPDFRenderRequest) (viewerPDFPageText, error)
}

type viewerNoopPDFRenderer struct{}

func (viewerNoopPDFRenderer) Available() bool {
	return false
}

func (viewerNoopPDFRenderer) RenderPage(_ viewerPDFRenderRequest) (viewerPDFRenderResult, error) {
	return viewerPDFRenderResult{}, errors.New("pdf preview is unavailable")
}

func (viewerNoopPDFRenderer) DocInfo(_ viewerPDFRenderRequest) (viewerPDFDocInfo, error) {
	return viewerPDFDocInfo{}, errors.New("pdf preview is unavailable")
}

func (viewerNoopPDFRenderer) PageText(_ viewerPDFRenderRequest) (viewerPDFPageText, error) {
	return viewerPDFPageText{}, errors.New("pdf preview is unavailable")
}

var viewerPDFPreviewBackend viewerPDFRenderer = viewerNoopPDFRenderer{}
var viewerPDFPreviewUsesLocalPath bool

func viewerPathLooksPDF(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		ext = strings.ToLower(pathpkg.Ext(path))
	}
	return ext == ".pdf"
}

func viewerCanPreviewPDFPath(path string) bool {
	return viewerPathLooksPDF(path) && viewerPDFPreviewBackend.Available()
}

func viewerLooksPreviewablePDF(path string, data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if !viewerPDFPreviewBackend.Available() {
		return false
	}
	if mediaType := viewerBinaryMediaType(path, data); mediaType != "" {
		return viewerTrimMediaType(mediaType) == "application/pdf"
	}
	return viewerPathLooksPDF(path)
}

func viewerPDFPreviewActive(st *fileViewerState) bool {
	return st != nil &&
		st.detectedImagePreview &&
		normalizeViewerImageFormat(st.imagePreviewFormat) == "pdf" &&
		st.imagePreviewPageCount > 0
}

func viewerPDFPageLabel(st *fileViewerState) string {
	if !viewerPDFPreviewActive(st) {
		return ""
	}
	pageCount := st.imagePreviewPageCount
	page := viewerPDFDisplayedPage(st) + 1
	if page < 1 {
		page = 1
	}
	if page > pageCount {
		page = pageCount
	}
	return "Page " + strconv.Itoa(page) + "/" + strconv.Itoa(pageCount)
}

func viewerPDFDisplayedPage(st *fileViewerState) int {
	if st == nil {
		return 0
	}
	page := st.imagePreviewPage
	if st.pdfDoc.pageCount() > 0 {
		page = st.pdfDoc.currentPage()
	}
	if page < 0 {
		return 0
	}
	if page >= st.imagePreviewPageCount {
		return st.imagePreviewPageCount - 1
	}
	return page
}
