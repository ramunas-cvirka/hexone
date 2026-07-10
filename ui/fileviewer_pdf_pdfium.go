// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build pdfium

package ui

import (
	"fmt"
	"image"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/klippa-app/go-pdfium/structs"
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
		page := requests.Page{
			ByIndex: &requests.PageByIndex{
				Document: doc,
				Index:    req.Page,
			},
		}
		bounds, rotation, err := pdfiumPageDisplayGeometry(instance, page)
		if err != nil {
			return err
		}
		structured, err := instance.GetPageTextStructured(&requests.GetPageTextStructured{
			Page: page,
			Mode: requests.GetPageTextStructuredModeChars,
		})
		if err != nil {
			return err
		}
		pageW, pageH := pdfiumDisplaySize(bounds, rotation)
		text.Page = req.Page
		text.Chars = make([]viewerPDFTextChar, 0, len(structured.Chars))
		for _, ch := range structured.Chars {
			if ch == nil || ch.Text == "" {
				continue
			}
			runes := []rune(ch.Text)
			left, top, right, bottom := pdfiumCharBoxToDisplay(
				ch.PointPosition.Left, ch.PointPosition.Top,
				ch.PointPosition.Right, ch.PointPosition.Bottom,
				bounds, rotation,
			)
			// Text cropped away by the page bounding box is not part of the
			// rendered page; keep it out of selection and copy too.
			if right < 0 || left > pageW || bottom < 0 || top > pageH {
				continue
			}
			text.Chars = append(text.Chars, viewerPDFTextChar{
				Rune:   runes[0],
				Left:   left,
				Top:    top,
				Right:  right,
				Bottom: bottom,
			})
		}
		return nil
	})
	if err != nil {
		return viewerPDFPageText{}, err
	}
	return text, nil
}

// pdfiumPageDisplayGeometry fetches the two properties that define the
// display space a page is rendered in: the page bounding box (media box
// intersected with the crop box) and the /Rotate value. Char boxes and
// annotation rects come back in raw, unrotated page user space and must be
// mapped through these to line up with the rendered bitmap.
func pdfiumPageDisplayGeometry(instance pdfium.Pdfium, page requests.Page) (structs.FPDF_FS_RECTF, enums.FPDF_PAGE_ROTATION, error) {
	bounds, err := instance.FPDF_GetPageBoundingBox(&requests.FPDF_GetPageBoundingBox{Page: page})
	if err != nil {
		return structs.FPDF_FS_RECTF{}, 0, err
	}
	rotation, err := instance.FPDFPage_GetRotation(&requests.FPDFPage_GetRotation{Page: page})
	if err != nil {
		return structs.FPDF_FS_RECTF{}, 0, err
	}
	return bounds.Rect, rotation.PageRotation, nil
}

// pdfiumDisplaySize returns the displayed page size in points, i.e. the
// bounding box dimensions with /Rotate applied.
func pdfiumDisplaySize(bounds structs.FPDF_FS_RECTF, rotation enums.FPDF_PAGE_ROTATION) (float64, float64) {
	w := float64(bounds.Right - bounds.Left)
	h := float64(bounds.Top - bounds.Bottom)
	if rotation == enums.FPDF_PAGE_ROTATION_90_CW || rotation == enums.FPDF_PAGE_ROTATION_270_CW {
		w, h = h, w
	}
	return w, h
}

// pdfiumCharBoxToDisplay maps a char box from unrotated page user space
// (bottom-left origin, box corners at (l,b) and (r,t)) into top-left-origin
// display coordinates matching the rendered bitmap: the page bounding box
// corner becomes (0,0) and /Rotate is applied.
func pdfiumCharBoxToDisplay(l, t, r, b float64, bounds structs.FPDF_FS_RECTF, rotation enums.FPDF_PAGE_ROTATION) (left, top, right, bottom float64) {
	bl := float64(bounds.Left)
	bt := float64(bounds.Top)
	br := float64(bounds.Right)
	bb := float64(bounds.Bottom)
	transform := func(x, y float64) (float64, float64) {
		switch rotation {
		case enums.FPDF_PAGE_ROTATION_90_CW:
			return y - bb, x - bl
		case enums.FPDF_PAGE_ROTATION_180_CW:
			return br - x, y - bb
		case enums.FPDF_PAGE_ROTATION_270_CW:
			return bt - y, br - x
		default:
			return x - bl, bt - y
		}
	}
	x0, y0 := transform(l, b)
	x1, y1 := transform(r, t)
	return math.Min(x0, x1), math.Min(y0, y1), math.Max(x0, x1), math.Max(y0, y1)
}

func (r *viewerPDFiumRenderer) PageLinks(req viewerPDFRenderRequest) (viewerPDFPageLinks, error) {
	var links viewerPDFPageLinks
	err := r.withDocument(req, func(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, pageCount int) error {
		if req.Page < 0 || req.Page >= pageCount {
			return fmt.Errorf("pdf page %d is out of range", req.Page+1)
		}
		page := requests.Page{
			ByIndex: &requests.PageByIndex{
				Document: doc,
				Index:    req.Page,
			},
		}
		bounds, rotation, err := pdfiumPageDisplayGeometry(instance, page)
		if err != nil {
			return err
		}
		pageW, pageH := pdfiumDisplaySize(bounds, rotation)
		links.Page = req.Page
		pos := 0
		for {
			entry, err := instance.FPDFLink_Enumerate(&requests.FPDFLink_Enumerate{Page: page, StartPos: pos})
			if err != nil || entry.Link == nil || entry.NextStartPos == nil {
				break
			}
			pos = *entry.NextStartPos
			destPage, ok := pdfiumLinkDestPage(instance, doc, *entry.Link)
			if !ok || destPage >= pageCount {
				continue
			}
			rect, err := instance.FPDFLink_GetAnnotRect(&requests.FPDFLink_GetAnnotRect{Link: *entry.Link})
			if err != nil || rect.Rect == nil {
				continue
			}
			// Annotation rects live in the same raw user space as char boxes.
			left, top, right, bottom := pdfiumCharBoxToDisplay(
				float64(rect.Rect.Left), float64(rect.Rect.Top),
				float64(rect.Rect.Right), float64(rect.Rect.Bottom),
				bounds, rotation,
			)
			if right < 0 || left > pageW || bottom < 0 || top > pageH {
				continue
			}
			links.Links = append(links.Links, viewerPDFPageLink{
				Left:     left,
				Top:      top,
				Right:    right,
				Bottom:   bottom,
				DestPage: destPage,
			})
		}
		return nil
	})
	if err != nil {
		return viewerPDFPageLinks{}, err
	}
	return links, nil
}

// pdfiumLinkDestPage resolves the destination page of a link annotation,
// either from its direct /Dest or from a GoTo action. External links (URI,
// remote files) report no destination.
func pdfiumLinkDestPage(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, link references.FPDF_LINK) (int, bool) {
	dest, err := instance.FPDFLink_GetDest(&requests.FPDFLink_GetDest{Document: doc, Link: link})
	if err == nil && dest.Dest != nil {
		idx, err := instance.FPDFDest_GetDestPageIndex(&requests.FPDFDest_GetDestPageIndex{Document: doc, Dest: *dest.Dest})
		if err != nil || idx.Index < 0 {
			return 0, false
		}
		return idx.Index, true
	}
	action, err := instance.FPDFLink_GetAction(&requests.FPDFLink_GetAction{Link: link})
	if err != nil || action.Action == nil {
		return 0, false
	}
	adest, err := instance.FPDFAction_GetDest(&requests.FPDFAction_GetDest{Document: doc, Action: *action.Action})
	if err != nil || adest.Dest == nil {
		return 0, false
	}
	idx, err := instance.FPDFDest_GetDestPageIndex(&requests.FPDFDest_GetDestPageIndex{Document: doc, Dest: *adest.Dest})
	if err != nil || idx.Index < 0 {
		return 0, false
	}
	return idx.Index, true
}

func (r *viewerPDFiumRenderer) TOC(req viewerPDFRenderRequest) ([]viewerPDFTOCEntry, error) {
	var toc []viewerPDFTOCEntry
	err := r.withDocument(req, func(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, _ int) error {
		bookmarks, err := instance.GetBookmarks(&requests.GetBookmarks{Document: doc})
		if err != nil {
			return err
		}
		const maxTOCEntries = 2048
		var appendBookmarks func([]responses.GetBookmarksBookmark, int)
		appendBookmarks = func(items []responses.GetBookmarksBookmark, level int) {
			for _, item := range items {
				if len(toc) >= maxTOCEntries {
					return
				}
				page := -1
				if item.DestInfo != nil {
					page = item.DestInfo.PageIndex
				} else if item.ActionInfo != nil && item.ActionInfo.DestInfo != nil {
					page = item.ActionInfo.DestInfo.PageIndex
				}
				title := strings.TrimSpace(item.Title)
				if title != "" {
					toc = append(toc, viewerPDFTOCEntry{Title: title, Page: page, Level: level})
					appendBookmarks(item.Children, level+1)
				} else {
					// Promote descendants of untitled outline nodes so every
					// visible child still has a visible parent.
					appendBookmarks(item.Children, level)
				}
			}
		}
		appendBookmarks(bookmarks.Bookmarks, 0)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return normalizeViewerPDFTOC(toc), nil
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
