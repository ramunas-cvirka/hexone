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

type viewerPDFTOCEntry struct {
	Title       string
	Page        int
	Level       int
	ID          string
	ParentID    string
	HasChildren bool
}

// normalizeViewerPDFTOC turns a flat, pre-order outline into a stable tree
// model. Renderers only need to provide titles, pages, and levels; identity,
// parent links, and disclosure state are derived here for the TOC accordion.
func normalizeViewerPDFTOC(entries []viewerPDFTOCEntry) []viewerPDFTOCEntry {
	if len(entries) == 0 {
		return nil
	}
	normalized := append([]viewerPDFTOCEntry(nil), entries...)
	stack := make([]string, 0, 8)
	for i := range normalized {
		entry := &normalized[i]
		if entry.Level < 0 {
			entry.Level = 0
		}
		// A level cannot skip past its nearest available ancestor.
		if entry.Level > len(stack) {
			entry.Level = len(stack)
		}
		if entry.ID == "" {
			entry.ID = "toc-" + strconv.Itoa(i)
		}
		entry.ParentID = ""
		if entry.Level > 0 {
			entry.ParentID = stack[entry.Level-1]
		}
		if entry.Level < len(stack) {
			stack[entry.Level] = entry.ID
			stack = stack[:entry.Level+1]
		} else {
			stack = append(stack, entry.ID)
		}
		entry.HasChildren = false
	}
	for i := 0; i+1 < len(normalized); i++ {
		normalized[i].HasChildren = normalized[i+1].Level > normalized[i].Level
	}
	return normalized
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
	TOC(req viewerPDFRenderRequest) ([]viewerPDFTOCEntry, error)
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

func (viewerNoopPDFRenderer) TOC(_ viewerPDFRenderRequest) ([]viewerPDFTOCEntry, error) {
	return nil, errors.New("pdf preview is unavailable")
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

func viewerPathSupportsUnboundedPreview(path string) bool {
	return viewerCanPreviewPDFPath(path) || viewerPathLooksImage(path)
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
