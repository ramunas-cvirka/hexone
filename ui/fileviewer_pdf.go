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

type viewerPDFRenderer interface {
	Available() bool
	RenderPage(req viewerPDFRenderRequest) (viewerPDFRenderResult, error)
}

type viewerNoopPDFRenderer struct{}

func (viewerNoopPDFRenderer) Available() bool {
	return false
}

func (viewerNoopPDFRenderer) RenderPage(_ viewerPDFRenderRequest) (viewerPDFRenderResult, error) {
	return viewerPDFRenderResult{}, errors.New("pdf preview is unavailable")
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
	page := st.imagePreviewPage + 1
	if page < 1 {
		page = 1
	}
	if page > pageCount {
		page = pageCount
	}
	return "Page " + strconv.Itoa(page) + "/" + strconv.Itoa(pageCount)
}
