// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"hexone/fm"
	uitheme "hexone/ui/theme"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type viewerHeaderDetailPart struct {
	Text  string
	Color color.NRGBA
}

const (
	viewerInlineCommandMinWidthDp       = 96
	viewerInlineCommandDisplayInsetDp   = 10
	viewerInlineCommandVerticalInsetDp  = 2
	viewerInlineCommandMeasurePaddingDp = viewerInlineCommandDisplayInsetDp * 2
	viewerModeTabFilenameMaxRunes       = 64
	viewerModeTabActionWidthDp          = 20
	viewerModeTabOuterInsetDp           = 5
	viewerModeTabMarkerGapDp            = 5
	viewerModeTabLabelGapDp             = 3
	viewerModeTabInterGapDp             = 2
	fileViewerOverlayEdgeInsetXDp       = 4
	fileViewerOverlayEdgeInsetYDp       = 2
	fileViewerTooltipEdgeInsetDp        = 4
)

type viewerZoomPreset struct {
	label string
	zoom  float32
	fit   bool
}

var viewerZoomPresets = [...]viewerZoomPreset{
	{label: "Fit width", fit: true},
	{label: "25%", zoom: 0.25},
	{label: "50%", zoom: 0.5},
	{label: "75%", zoom: 0.75},
	{label: "100%", zoom: 1},
	{label: "125%", zoom: 1.25},
	{label: "150%", zoom: 1.5},
	{label: "200%", zoom: 2},
	{label: "400%", zoom: 4},
}

func viewerZoomPresetActive(st *fileViewerState, preset viewerZoomPreset) bool {
	if st == nil || !st.detectedImagePreview {
		return false
	}
	current := st.imageView.effectiveZoom()
	target := preset.zoom
	if viewerPDFPreviewActive(st) {
		current = st.pdfDoc.effectiveZoom()
		if preset.fit {
			target = 1
		}
	} else if preset.fit {
		target = st.imageView.fitWidthZoom(st.imagePreview)
	}
	return float32Abs(current-target) < 0.0001
}

func applyViewerZoomPreset(st *fileViewerState, preset viewerZoomPreset) bool {
	if st == nil || !st.detectedImagePreview {
		return false
	}
	if viewerPDFPreviewActive(st) {
		if preset.fit {
			return st.pdfDoc.resetZoom()
		}
		return st.pdfDoc.setZoom(preset.zoom)
	}
	if preset.fit {
		return st.imageView.fitWidth(st.imagePreview)
	}
	return st.imageView.setZoom(st.imagePreview, preset.zoom)
}

func ensureViewerTOCClicks(st *fileViewerState) {
	if st == nil || (len(st.tocClicks) == len(st.pdfDoc.toc) && len(st.tocDisclosureClicks) == len(st.pdfDoc.toc)) {
		return
	}
	st.tocClicks = make([]widget.Clickable, len(st.pdfDoc.toc))
	st.tocDisclosureClicks = make([]widget.Clickable, len(st.pdfDoc.toc))
}

func viewerTOCEntryExpanded(st *fileViewerState, entry viewerPDFTOCEntry) bool {
	return st != nil && entry.HasChildren && st.tocExpanded[entry.Level] == entry.ID
}

func viewerTOCVisibleIndices(st *fileViewerState) []int {
	if st == nil || len(st.pdfDoc.toc) == 0 {
		return nil
	}
	visible := make([]int, 0, len(st.pdfDoc.toc))
	visibleByID := make(map[string]bool, len(st.pdfDoc.toc))
	for i, entry := range st.pdfDoc.toc {
		entryVisible := entry.Level == 0
		if entry.Level > 0 {
			entryVisible = visibleByID[entry.ParentID] && st.tocExpanded[entry.Level-1] == entry.ParentID
		}
		visibleByID[entry.ID] = entryVisible
		if entryVisible {
			visible = append(visible, i)
		}
	}
	return visible
}

func toggleViewerTOCEntry(st *fileViewerState, entry viewerPDFTOCEntry) bool {
	if st == nil || !entry.HasChildren {
		return false
	}
	if st.tocExpanded == nil {
		st.tocExpanded = make(map[int]string)
	}
	if st.tocExpanded[entry.Level] == entry.ID {
		for level := range st.tocExpanded {
			if level >= entry.Level {
				delete(st.tocExpanded, level)
			}
		}
		return true
	}
	st.tocExpanded[entry.Level] = entry.ID
	for level := range st.tocExpanded {
		if level > entry.Level {
			delete(st.tocExpanded, level)
		}
	}
	return true
}

func viewerTOCEntryNavigates(st *fileViewerState, entry viewerPDFTOCEntry) bool {
	return st != nil && entry.Page >= 0 && entry.Page < st.pdfDoc.pageCount()
}

func viewerTOCDisclosureGlyph(st *fileViewerState, entry viewerPDFTOCEntry) string {
	if !entry.HasChildren {
		return ""
	}
	if viewerTOCEntryExpanded(st, entry) {
		return "↓"
	}
	return "→"
}

func (ui *UI) handleFileViewerTOCClicks(gtx layout.Context, st *fileViewerState) {
	if st == nil {
		return
	}
	ensureViewerTOCClicks(st)
	for i := range st.pdfDoc.toc {
		entry := st.pdfDoc.toc[i]
		if st.tocDisclosureClicks[i].Clicked(gtx) && toggleViewerTOCEntry(st, entry) {
			gtx.Execute(op.InvalidateCmd{})
		}
		if !st.tocClicks[i].Clicked(gtx) || !viewerTOCEntryNavigates(st, entry) {
			continue
		}
		if st.pdfDoc.scrollToPage(entry.Page) {
			st.pdfDoc.syncVisualScroll()
			st.markUserBrowsing(gtx.Now)
		}
		st.closeEncodingMenu()
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) layoutFileViewer(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.fileViewer
	if st == nil {
		return layout.Dimensions{}
	}
	theme := ui.fileViewerTheme()
	if st.commandAreaPress != nil {
		clear(st.commandAreaPress)
	}
	if st.commandEditOn {
		for {
			ev, ok := st.commandEditor.Update(gtx)
			if !ok {
				break
			}
			if submit, ok := ev.(widget.SubmitEvent); ok {
				st.commandEditor.SetText(submit.Text)
				ui.applyViewerCommandEdit(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
		}
	}
	if st.commandClick.Clicked(gtx) {
		ui.startViewerCommandEdit(gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	ui.handleFileViewerFindInput(gtx, st)
	historyActionClicked := st.historyClick.Clicked(gtx)
	if st.modeFileClick.Clicked(gtx) {
		st.tabAnim.setPulse("file", gtx.Now)
		ui.setFileViewerMode("file", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.modeHexClick.Clicked(gtx) {
		st.tabAnim.setPulse("hex", gtx.Now)
		ui.setFileViewerMode("hex", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.modeCmdClick.Clicked(gtx) && !historyActionClicked {
		st.tabAnim.setPulse("command", gtx.Now)
		ui.setFileViewerMode("command", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.editToggleClick.Clicked(gtx) {
		ui.toggleFileViewerEdit(gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.mode == "file" && st.detectedImagePreview {
		if st.zoomMenuClick.Clicked(gtx) {
			wasOpen := st.zoomMenuOpen
			st.closeEncodingMenu()
			if !wasOpen {
				st.zoomMenuOpen = true
				st.zoomMenuAt = gtx.Now
			}
			gtx.Execute(op.InvalidateCmd{})
		}
		for i := range viewerZoomPresets {
			if st.zoomPresetClicks[i].Clicked(gtx) {
				if applyViewerZoomPreset(st, viewerZoomPresets[i]) {
					st.markUserBrowsing(gtx.Now)
				}
				st.closeEncodingMenu()
				gtx.Execute(op.InvalidateCmd{})
			}
		}
		ensureViewerTOCClicks(st)
		if st.tocMenuClick.Clicked(gtx) && len(st.pdfDoc.toc) > 0 {
			wasOpen := st.tocMenuOpen
			st.closeEncodingMenu()
			if !wasOpen {
				st.tocMenuOpen = true
				st.tocMenuAt = gtx.Now
				st.tocList.Axis = layout.Vertical
			}
			gtx.Execute(op.InvalidateCmd{})
		}
		ui.handleFileViewerTOCClicks(gtx, st)
	}
	if st.mode == "file" && !st.detectedImagePreview && !st.editMode {
		if st.encodingMenuClick.Clicked(gtx) {
			if st.encodingMenuOpen {
				st.closeEncodingMenu()
			} else {
				st.encodingMenuOpen = true
				st.encodingMenuAt = gtx.Now
			}
			gtx.Execute(op.InvalidateCmd{})
		}
		if st.encodingAutoClick.Clicked(gtx) {
			ui.setFileViewerEncoding(fm.ViewerFileEncodingAuto, gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		}
		if st.encodingUTF8Click.Clicked(gtx) {
			ui.setFileViewerEncoding(fm.ViewerFileEncodingUTF8, gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		}
		if st.encodingUTF16LEClick.Clicked(gtx) {
			ui.setFileViewerEncoding(fm.ViewerFileEncodingUTF16LE, gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		}
		if st.encodingUTF16BEClick.Clicked(gtx) {
			ui.setFileViewerEncoding(fm.ViewerFileEncodingUTF16BE, gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		}
		if st.encodingCP437Click.Clicked(gtx) {
			ui.setFileViewerEncoding(fm.ViewerFileEncodingCP437, gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	if ui.viewerShowsAutoRefreshButton(st) && st.autoRefreshClick.Clicked(gtx) {
		ui.toggleFileViewerAutoRefresh(gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if historyActionClicked {
		if st.editMode {
			st.status = "finish editing before opening history"
		} else {
			prevTab := st.activeTabKey()
			if st.commandEditOn {
				ui.cancelViewerCommandEdit()
			}
			st.historyOpen = !st.historyOpen
			if nextTab := st.activeTabKey(); nextTab != prevTab {
				st.tabPrev = prevTab
				st.tabAnimAt = gtx.Now
			}
			st.tabAnim.setPulse("history", gtx.Now)
		}
		gtx.Execute(op.InvalidateCmd{})
	}
	for st.backdropClick.Clicked(gtx) {
	}
	if st.closeClick.Clicked(gtx) {
		ui.closeFileViewer()
		gtx.Execute(op.InvalidateCmd{})
		return ui.layoutFilePanes(th, gtx)
	}
	if st.loading {
		// Keep frames ticking while background load is running, otherwise
		// results can remain pending until an external event (e.g. resize).
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(33 * time.Millisecond)})
	}
	if st.find.open && st.find.searching {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(33 * time.Millisecond)})
	}

	ui.scheduleFileViewerWatch(gtx)

	return st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, theme.Backdrop, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

		dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if st.err == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, st.err)
					lbl.Font.Typeface = ui.viewerTypeface()
					lbl.TextSize = scaleThemeFontSize(th, 10)
					lbl.Color = theme.Error
					lbl.MaxLines = 2
					lbl.Truncator = "..."
					return lbl.Layout(gtx)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
				return fillBgExact(gtx, theme.PanelBg, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutFileViewerHeader(th, gtx, st)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
							return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(10), Top: unit.Dp(4), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
								return layout.Stack{}.Layout(gtx,
									layout.Expanded(func(gtx layout.Context) layout.Dimensions {
										if st.loading && st.content == "" {
											if st.mode == "hex" && st.hex != nil && len(st.hex.buffer) > 0 {
												return ui.layoutHexOutputView(th, gtx, st)
											}
										}
										if message := fileViewerEmptyPanelMessage(st); message != "" {
											wait := material.Body2(th, message)
											wait.Font.Typeface = ui.viewerTypeface()
											wait.TextSize = ui.viewerTextSize()
											wait.Color = theme.Hint
											return wait.Layout(gtx)
										}
										gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
										if st.mode == "hex" {
											return ui.layoutHexOutputView(th, gtx, st)
										}
										if st.detectedImagePreview {
											if viewerPDFPreviewActive(st) {
												return ui.layoutPDFDocOutputView(th, gtx, st)
											}
											return ui.layoutImageOutputView(th, gtx, st)
										}
										if st.editMode && st.mode == "file" {
											return ui.layoutFileViewerTextEditor(th, gtx, st)
										}
										if viewerMarkdownPreviewActive(st) {
											return ui.layoutMarkdownPreview(th, gtx, st)
										}
										return ui.layoutStreamOutputView(th, gtx, st)
									}),
									layout.Stacked(func(gtx layout.Context) layout.Dimensions {
										return ui.layoutFileViewerOverlay(th, gtx, st)
									}),
									layout.Stacked(func(gtx layout.Context) layout.Dimensions {
										return ui.layoutFileViewerFindBar(th, gtx, st)
									}),
								)
							})
						}),
					)
				})
			}),
		)
		ui.handleFileViewerRootPointerEvents(gtx, st)
		ui.layoutFileViewerContextMenu(th, gtx, st)
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		pass := pointer.PassOp{}.Push(gtx.Ops)
		event.Op(gtx.Ops, &st.rootPointerTag)
		pass.Pop()
		if st.modeSwitchPrompt.open {
			ui.layoutFileViewerModeSwitchModal(th, gtx, st)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
		if st.menuOpen {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
		return dims
	})
}

func fileViewerEmptyPanelMessage(st *fileViewerState) string {
	if st == nil {
		return ""
	}
	if st.loading && st.content == "" && st.updatedAt.IsZero() {
		return "Loading..."
	}
	if st.mode == "command" && st.content == "" && st.err == "" {
		return "No output"
	}
	return ""
}

func (ui *UI) handleFileViewerRootPointerEvents(gtx layout.Context, st *fileViewerState) {
	if st == nil {
		return
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &st.rootPointerTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok || pe.Kind != pointer.Press {
			continue
		}
		if pe.Buttons.Contain(pointer.ButtonSecondary) {
			st.menuPos = pe.Position.Round()
		}
		if st.commandAreaPress != nil {
			if _, ok := st.commandAreaPress[pe.PointerID]; ok {
				delete(st.commandAreaPress, pe.PointerID)
				continue
			}
		}
		if ui.editorMenuOpenID != "" {
			if pe.Buttons.Contain(pointer.ButtonSecondary) && ui.editorMenuOpenID == "viewer-file-edit" {
				continue
			}
			ui.closeEditorContextMenu()
			gtx.Execute(op.InvalidateCmd{})
			continue
		}
		if st.commandEditOn {
			ui.cancelViewerCommandEdit()
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (ui *UI) handleFileViewerPointerEvents(gtx layout.Context, st *fileViewerState, size image.Point) {
	if st == nil || size.X <= 0 || size.Y <= 0 {
		return
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &st.contentPointerTag,
			Kinds:  pointer.Scroll | pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Move | pointer.Enter | pointer.Leave,
			ScrollY: pointer.ScrollRange{
				Min: -100,
				Max: 100,
			},
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		if ui.terminalFocused(gtx) && terminalSurfaceFocusPointerEvent(pe) {
			ui.releaseTerminalKeyboardFocus(gtx)
		}
		pos := pe.Position.Round()
		switch pe.Kind {
		case pointer.Scroll:
			if pe.Scroll.Y != 0 {
				st.markUserBrowsing(gtx.Now)
				viewerScrollByDelta(st, pe.Scroll.Y)
			}
		case pointer.Press:
			if ui.editorMenuOpenID != "" {
				ui.closeEditorContextMenu()
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			popupOpen := st.encodingMenuOpen || st.zoomMenuOpen || st.tocMenuOpen
			popupRect := st.encodingMenuRect
			if st.zoomMenuOpen {
				popupRect = st.zoomMenuRect
			} else if st.tocMenuOpen {
				popupRect = st.tocMenuRect
			}
			if popupOpen &&
				!viewerPointInRect(pos, popupRect) &&
				!viewerPointInRect(pos, st.encodingBarRect) {
				st.closeEncodingMenu()
				gtx.Execute(op.InvalidateCmd{})
			}
			if st.find.sourceMenuOpen &&
				!viewerPointInRect(pos, st.find.sourceMenuRect) &&
				!viewerPointInRect(pos, st.find.sourceButtonRect) {
				st.find.closeSourceMenu()
				gtx.Execute(op.InvalidateCmd{})
			}
			if st.commandEditOn {
				ui.cancelViewerCommandEdit()
				gtx.Execute(op.InvalidateCmd{})
			}
			if pe.Buttons.Contain(pointer.ButtonSecondary) {
				st.setHistoryOpen(false, gtx.Now)
				if st.editMode && st.mode == "file" {
					continue
				}
				st.openContextMenu(pos, gtx.Now)
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) && st.scrollbarVisible && viewerPointInRect(pos, st.scrollbarTrack) {
				st.scrollbarDragging = true
				st.scrollbarDragID = pe.PointerID
				gtx.Execute(pointer.GrabCmd{Tag: &st.contentPointerTag, ID: pe.PointerID})
				st.markUserBrowsing(gtx.Now)
				viewerScrollFromScrollbarPos(st, pos.Y)
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) || pe.Buttons.Contain(pointer.ButtonSecondary) || pe.Buttons.Contain(pointer.ButtonTertiary) {
				st.markUserBrowsing(gtx.Now)
				st.setHistoryOpen(false, gtx.Now)
				if st.menuOpen {
					st.closeContextMenu()
				}
				if st.updateScrollbarHover(pos) {
					gtx.Execute(op.InvalidateCmd{})
				}
			}
		case pointer.Drag:
			if st.scrollbarDragging && pe.PointerID == st.scrollbarDragID {
				st.markUserBrowsing(gtx.Now)
				viewerScrollFromScrollbarPos(st, pos.Y)
				gtx.Execute(op.InvalidateCmd{})
			}
			if st.updateScrollbarHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Release, pointer.Cancel:
			if st.scrollbarDragging && pe.PointerID == st.scrollbarDragID {
				st.scrollbarDragging = false
			}
			if st.updateScrollbarHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Move, pointer.Enter:
			if st.updateScrollbarHover(pos) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Leave:
			if st.scrollbarHover {
				st.scrollbarHover = false
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}

	ui.applyFileViewerScrollCursor(gtx, st)

	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &st.contentPointerTag)
	pass.Pop()
}

func (ui *UI) handleFileViewerCommandAreaPresses(gtx layout.Context, st *fileViewerState) {
	if st == nil || !st.commandEditOn {
		return
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &st.commandAreaTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok || pe.Kind != pointer.Press {
			continue
		}
		if st.commandAreaPress == nil {
			st.commandAreaPress = make(map[pointer.ID]struct{}, 1)
		}
		st.commandAreaPress[pe.PointerID] = struct{}{}
	}
}

func (ui *UI) paintFileViewerScrollHint(gtx layout.Context, st *fileViewerState, size image.Point) {
	if st == nil || size.X < 8 || size.Y < 24 {
		if st != nil {
			st.scrollbarVisible = false
			st.scrollbarTrack = image.Rectangle{}
			st.scrollbarThumb = image.Rectangle{}
			st.scrollbarHover = false
		}
		return
	}
	totalLines := viewerTotalLines(st.content)
	if totalLines <= 1 {
		st.scrollbarVisible = false
		st.scrollbarTrack = image.Rectangle{}
		st.scrollbarThumb = image.Rectangle{}
		st.scrollbarHover = false
		return
	}

	line, _ := st.contentEditor.CaretPos()
	lineHeight := gtx.Sp(ui.viewerTextSize())
	if lineHeight < 10 {
		lineHeight = 10
	}
	visibleLines := size.Y / lineHeight
	if visibleLines < 1 {
		visibleLines = 1
	}
	if visibleLines > totalLines {
		visibleLines = totalLines
	}
	maxTop := totalLines - visibleLines
	top := line - visibleLines/2
	if top < 0 {
		top = 0
	}
	if top > maxTop {
		top = maxTop
	}

	trackW := viewerScrollbarThickness(gtx, size.X)
	if trackW <= 0 {
		st.scrollbarVisible = false
		st.scrollbarTrack = image.Rectangle{}
		st.scrollbarThumb = image.Rectangle{}
		st.scrollbarHover = false
		return
	}
	trackX := size.X - trackW
	if trackX < 0 {
		trackX = 0
	}
	track := image.Rect(trackX, 0, trackX+trackW, size.Y)
	thumb := viewerScrollbarThumbForScroll(track, visibleLines, totalLines, float64(top), true)
	st.scrollbarVisible = true
	st.scrollbarTrack = track
	st.scrollbarThumb = thumb
	st.scrollbarLines = totalLines
	st.scrollbarVisibleN = visibleLines

	paintViewerScrollbar(gtx, ui.fileViewerTheme(), track, thumb, st.scrollbarHover, st.scrollbarHover, st.scrollbarDragging)
}

func (ui *UI) applyFileViewerScrollCursor(gtx layout.Context, st *fileViewerState) {
	if st == nil || !st.scrollbarVisible {
		return
	}
	if st.scrollbarDragging {
		pointer.CursorGrabbing.Add(gtx.Ops)
		return
	}
	if st.scrollbarHover {
		defer clip.Rect(st.scrollbarTrack).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
	}
}

func (ui *UI) layoutFileViewerContextMenu(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	if st == nil || !st.menuOpen {
		return layout.Dimensions{}
	}
	if st.copyToggle.Clicked(gtx) {
		_ = ui.copyFileViewerText(gtx, true)
		st.closeContextMenu()
	}
	if st.copyTextToggle.Clicked(gtx) {
		_ = ui.copyFileViewerHex(gtx, true, true)
		st.closeContextMenu()
	}
	if st.wrapToggle.Clicked(gtx) {
		if st.mode != "hex" {
			ui.toggleViewerWordWrap()
		}
		st.closeContextMenu()
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &st.menuPointerTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok || pe.Kind != pointer.Press {
			continue
		}
		if !pe.Buttons.Contain(pointer.ButtonPrimary) {
			continue
		}
		pos := pe.Position.Round()
		if st.menuRect.Dx() <= 0 || st.menuRect.Dy() <= 0 ||
			pos.X < st.menuRect.Min.X || pos.X >= st.menuRect.Max.X ||
			pos.Y < st.menuRect.Min.Y || pos.Y >= st.menuRect.Max.Y {
			st.closeContextMenu()
		}
	}
	if !st.menuOpen {
		return layout.Dimensions{}
	}

	alpha, slideY, animating := popupOpenProgress(gtx.Now, st.menuOpenedAt)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	m := op.Record(gtx.Ops)
	menuDims := ui.layoutFileViewerContextMenuCard(th, gtx, st, alpha)
	call := m.Stop()

	anchor := st.menuPos
	anchor.Y += slideY
	anchor = clampFilePaneMenuPoint(anchor, menuDims.Size, gtx.Constraints.Max)
	st.menuRect = image.Rectangle{Min: anchor, Max: anchor.Add(menuDims.Size)}

	bodyClip := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	offset := op.Offset(anchor).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	bodyClip.Pop()

	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &st.menuPointerTag)
	pass.Pop()
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

type fileViewerContextMenuRow struct {
	click  *widget.Clickable
	item   fileContextMenuItem
	active bool
}

func fileViewerContextMenuRows(st *fileViewerState) []fileViewerContextMenuRow {
	if st == nil {
		return nil
	}
	items := []fileViewerContextMenuRow{
		{
			click: &st.copyToggle,
			item: fileContextMenuItem{
				ID:    "viewer-copy",
				Label: "Copy",
			},
		},
	}
	if viewerMarkdownPreviewActive(st) {
		return items
	}
	if st.mode == "hex" {
		items[0].item.Label = "Copy as Hex"
		items = append(items, fileViewerContextMenuRow{
			click: &st.copyTextToggle,
			item: fileContextMenuItem{
				ID:    "viewer-copy-text",
				Label: "Copy as Text",
			},
		})
	} else {
		items = append(items, fileViewerContextMenuRow{
			click: &st.wrapToggle,
			item: fileContextMenuItem{
				ID:    "viewer-word-wrap",
				Label: viewerWordWrapMenuLabel(st.wrapEnabled),
			},
			active: st.wrapEnabled,
		})
	}
	return items
}

func (ui *UI) layoutFileViewerContextMenuCard(th *material.Theme, gtx layout.Context, st *fileViewerState, alpha float32) layout.Dimensions {
	theme := ui.filePanePopupTheme()
	items := fileViewerContextMenuRows(st)
	width := gtx.Dp(unit.Dp(96))
	for _, row := range items {
		lbl := material.Body2(th, row.item.Label)
		lbl.Font.Typeface = ui.mainTypeface()
		lbl.TextSize = ui.functionBarTextSize()
		lbl.Font.Weight = font.Medium
		if measured := measureLabelUnconstrained(gtx, lbl).Size.X + gtx.Dp(unit.Dp(28)); measured > width {
			width = measured
		}
	}
	if width > gtx.Constraints.Max.X {
		width = gtx.Constraints.Max.X
	}
	if width < 1 {
		width = 1
	}
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedClipBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			scaleColorAlpha(theme.Bg, alpha),
			scaleColorAlpha(theme.Border, alpha),
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					hoverID := ""
					for _, row := range items {
						if row.click.Hovered() {
							hoverID = row.item.ID
							break
						}
					}
					if hoverID != st.menuHoverID {
						st.menuHoverID = hoverID
						st.menuHoverAnim.setHover(hoverID, gtx.Now)
						gtx.Execute(op.InvalidateCmd{})
					}
					children := make([]layout.FlexChild, 0, len(items))
					for _, row := range items {
						row := row
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							hoverFill, hoverAnim := st.menuHoverAnim.hoverFill(gtx.Now, row.item.ID)
							if hoverAnim {
								gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
							}
							dims, _, _ := ui.layoutFilePaneContextMenuItem(th, gtx, theme, row.click, row.item, row.active, hoverFill, alpha, ui.fileContextMenuRowHeight(gtx, row.item))
							return dims
						}))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			},
		)
	})
}

func (ui *UI) layoutFileViewerHeader(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	return layoutClippedToDimensions(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.layoutFileViewerHeaderContent(th, gtx, st)
	})
}

func (ui *UI) layoutFileViewerHeaderContent(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	history := ui.viewerHistoryCommands(st.command)
	stripH := ui.viewerHeaderStripHeight(gtx)
	row := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return ui.layoutFileViewerHeaderRow(th, gtx, st, stripH)
	})
	if !st.historyOpen {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, row)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		row,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFileViewerHistoryList(th, gtx, st, history)
			})
		}),
	)
}

func (ui *UI) layoutFileViewerOverlay(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	bar := op.Record(gtx.Ops)
	barDims := ui.layoutFileViewerOverlayBar(th, gtx, st)
	barCall := bar.Stop()
	if barDims.Size.X <= 0 || barDims.Size.Y <= 0 {
		st.encodingBarRect = image.Rectangle{}
		st.encodingMenuRect = image.Rectangle{}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	marginX := gtx.Dp(unit.Dp(fileViewerOverlayEdgeInsetXDp))
	marginY := gtx.Dp(unit.Dp(fileViewerOverlayEdgeInsetYDp))
	barPos := image.Pt(gtx.Constraints.Max.X-barDims.Size.X-marginX, gtx.Constraints.Max.Y-barDims.Size.Y-marginY)
	if barPos.X < 0 {
		barPos.X = 0
	}
	if barPos.Y < 0 {
		barPos.Y = 0
	}
	st.encodingBarRect = image.Rectangle{Min: barPos, Max: barPos.Add(barDims.Size)}
	offset := op.Offset(barPos).Push(gtx.Ops)
	barCall.Add(gtx.Ops)
	offset.Pop()

	st.encodingMenuRect = image.Rectangle{}
	st.zoomMenuRect = image.Rectangle{}
	st.tocMenuRect = image.Rectangle{}
	var menuWidget func(layout.Context, float32) layout.Dimensions
	var menuAt time.Time
	var menuRect *image.Rectangle
	switch {
	case st.zoomMenuOpen && st.mode == "file" && st.detectedImagePreview:
		menuWidget = func(gtx layout.Context, alpha float32) layout.Dimensions {
			return ui.layoutFileViewerZoomMenu(th, gtx, st, alpha)
		}
		menuAt = st.zoomMenuAt
		menuRect = &st.zoomMenuRect
	case st.tocMenuOpen && st.mode == "file" && viewerPDFPreviewActive(st) && len(st.pdfDoc.toc) > 0:
		menuWidget = func(gtx layout.Context, alpha float32) layout.Dimensions {
			return ui.layoutFileViewerTOCMenu(th, gtx, st, alpha)
		}
		menuAt = st.tocMenuAt
		menuRect = &st.tocMenuRect
	case st.encodingMenuOpen && st.mode == "file" && !st.detectedImagePreview:
		menuWidget = func(gtx layout.Context, alpha float32) layout.Dimensions {
			return ui.layoutFileViewerEncodingMenu(th, gtx, st, alpha)
		}
		menuAt = st.encodingMenuAt
		menuRect = &st.encodingMenuRect
	}
	if menuWidget != nil {
		alpha, slideY, animating := popupOpenProgress(gtx.Now, menuAt)
		if animating {
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
		}
		menu := op.Record(gtx.Ops)
		menuDims := menuWidget(gtx, alpha)
		menuCall := menu.Stop()
		menuPos := image.Pt(barPos.X+barDims.Size.X-menuDims.Size.X, barPos.Y-gtx.Dp(unit.Dp(6))-menuDims.Size.Y+slideY)
		menuPos = clampFilePaneMenuPoint(menuPos, menuDims.Size, gtx.Constraints.Max)
		*menuRect = image.Rectangle{Min: menuPos, Max: menuPos.Add(menuDims.Size)}
		offset = op.Offset(menuPos).Push(gtx.Ops)
		menuCall.Add(gtx.Ops)
		offset.Pop()
	}
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *UI) layoutFileViewerOverlayBar(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	theme := ui.fileViewerTheme()
	statusText, statusColor := ui.fileViewerOverlayStatusText(st)
	detailLabel := ""
	pageLabel := ""
	tocLabel := ""
	encodingLabel := ""
	if st.mode == "file" {
		if st.detectedImagePreview {
			detailLabel = viewerImageZoomLabel(st)
			pageLabel = viewerPDFPageLabel(st)
			if viewerPDFPreviewActive(st) && len(st.pdfDoc.toc) > 0 {
				tocLabel = "TOC"
			}
		} else if !st.detectedBinaryPreview {
			detailLabel = viewerLineEndingLabel(st.detectedLineEnding)
			encodingLabel = viewerEncodingStatusLabel(st)
		}
	}
	if statusText == "" && detailLabel == "" && pageLabel == "" && tocLabel == "" && encodingLabel == "" {
		return layout.Dimensions{}
	}
	return fillFlatBox(
		gtx,
		scaleColorAlpha(theme.TooltipBg, 0.88),
		scaleColorAlpha(theme.TooltipBorder, 0.78),
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, 10)
				addGap := func(width unit.Dp) {
					if len(children) > 0 {
						children = append(children, layout.Rigid(layout.Spacer{Width: width}.Layout))
					}
				}
				addSeparator := func() {
					if len(children) > 0 {
						children = append(children,
							layout.Rigid(layout.Spacer{Width: unit.Dp(3)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutFileViewerOverlayDivider(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(3)}.Layout),
						)
					}
				}
				if statusText != "" {
					addSeparator()
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.layoutFileViewerOverlayText(th, gtx, statusText, statusColor, 0)
					}))
				}
				if detailLabel != "" {
					addGap(unit.Dp(3))
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.layoutFileViewerOverlayChip(th, gtx, detailLabel, theme.CommandText, st.zoomMenuOpen, &st.zoomMenuClick)
					}))
				}
				if pageLabel != "" {
					addGap(unit.Dp(3))
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.layoutFileViewerOverlayChip(th, gtx, pageLabel, theme.CommandStaticText, false, nil)
					}))
				}
				if tocLabel != "" {
					addGap(unit.Dp(3))
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.layoutFileViewerOverlayChip(th, gtx, tocLabel, theme.CommandText, st.tocMenuOpen, &st.tocMenuClick)
					}))
				}
				if encodingLabel != "" {
					addGap(unit.Dp(3))
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						click := &st.encodingMenuClick
						if st.mode != "file" || st.detectedImagePreview {
							click = nil
						}
						return ui.layoutFileViewerOverlayChip(th, gtx, encodingLabel, theme.CommandText, st.encodingMenuOpen, click)
					}))
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
			})
		},
	)
}

func (ui *UI) layoutFileViewerOverlayText(th *material.Theme, gtx layout.Context, text string, fg color.NRGBA, width int) layout.Dimensions {
	if strings.TrimSpace(text) == "" {
		return layout.Dimensions{}
	}
	host := func(gtx layout.Context) layout.Dimensions {
		lbl := material.Body2(th, text)
		lbl.Font.Typeface = ui.viewerTypeface()
		lbl.TextSize = scaleThemeFontSize(th, 10)
		lbl.Color = fg
		lbl.MaxLines = 1
		lbl.Truncator = "..."
		return layoutVCenteredLabel(gtx, lbl)
	}
	if width > 0 {
		return maxWidth(gtx, width, host)
	}
	return host(gtx)
}

func (ui *UI) layoutFileViewerOverlayChip(th *material.Theme, gtx layout.Context, label string, fg color.NRGBA, active bool, click *widget.Clickable) layout.Dimensions {
	if strings.TrimSpace(label) == "" {
		return layout.Dimensions{}
	}
	theme := ui.fileViewerTheme()
	bg := mixNRGBA(theme.CommandBg, theme.TooltipBg, 0.32)
	border := scaleColorAlpha(theme.CommandBorder, 0.72)
	if active {
		bg = mixNRGBA(theme.CommandBgHover, theme.TooltipBg, 0.22)
		border = scaleColorAlpha(theme.CommandBorderHover, 0.86)
	} else if click != nil && click.Hovered() {
		bg = mixNRGBA(theme.CommandBgHover, theme.TooltipBg, 0.28)
		border = scaleColorAlpha(theme.CommandBorderHover, 0.78)
	}
	layoutChip := func(gtx layout.Context) layout.Dimensions {
		return fillFlatBox(
			gtx,
			scaleColorAlpha(bg, 0.95),
			border,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, label)
					lbl.Font.Typeface = ui.viewerTypeface()
					lbl.Font.Weight = font.Medium
					lbl.TextSize = scaleThemeFontSize(th, 10)
					lbl.Color = fg
					lbl.MaxLines = 1
					return layoutVCenteredLabel(gtx, lbl)
				})
			},
		)
	}
	if click == nil {
		return layoutChip(gtx)
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layoutChip(gtx)
	})
}

func (ui *UI) layoutFileViewerOverlayDivider(gtx layout.Context) layout.Dimensions {
	theme := ui.fileViewerTheme()
	fill := mixNRGBA(theme.TooltipText, theme.TooltipBorder, 0.45)
	fill.A = 112
	h := gtx.Dp(unit.Dp(12))
	if h < gtx.Dp(unit.Dp(8)) {
		h = gtx.Dp(unit.Dp(8))
	}
	w := gtx.Dp(unit.Dp(1))
	if w < 1 {
		w = 1
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, fill, clip.Rect(image.Rect(0, 0, w, h)).Op())
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}

func (ui *UI) layoutFileViewerZoomMenu(th *material.Theme, gtx layout.Context, st *fileViewerState, alpha float32) layout.Dimensions {
	theme := ui.filePanePopupTheme()
	width := gtx.Dp(unit.Dp(132))
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedClipBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			scaleColorAlpha(theme.Bg, alpha),
			scaleColorAlpha(theme.Border, alpha),
			func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(viewerZoomPresets)*2)
				for i, preset := range viewerZoomPresets {
					if i > 0 {
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
						}))
					}
					i, preset := i, preset
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						click := &st.zoomPresetClicks[i]
						hoverFill := float32(0)
						if click.Hovered() {
							hoverFill = 1
						}
						item := fileContextMenuItem{ID: fmt.Sprintf("viewer-zoom-%d", i), Label: preset.label}
						dims, _, _ := ui.layoutFilePaneContextMenuItem(th, gtx, theme, click, item, viewerZoomPresetActive(st, preset), hoverFill, alpha, ui.fileContextMenuRowHeight(gtx, item))
						return dims
					}))
				}
				return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			},
		)
	})
}

func (ui *UI) layoutFileViewerTOCRow(th *material.Theme, gtx layout.Context, st *fileViewerState, index, rowH int, theme filePanePopupTheme, alpha float32) layout.Dimensions {
	entry := st.pdfDoc.toc[index]
	navigable := viewerTOCEntryNavigates(st, entry)
	active := navigable && entry.Page == st.pdfDoc.currentPage()
	titleClick := &st.tocClicks[index]
	disclosureClick := &st.tocDisclosureClicks[index]
	titleHovered := navigable && titleClick.Hovered()
	disclosureHovered := entry.HasChildren && disclosureClick.Hovered()

	rowBG := color.NRGBA{}
	if active {
		rowBG = scaleColorAlpha(theme.ActiveBg, alpha)
	}
	titleBG := color.NRGBA{}
	if titleHovered && !active {
		titleBG = scaleColorAlpha(theme.HoverBg, alpha)
	}
	titleFG := scaleColorAlpha(theme.Text, alpha)
	if !navigable {
		titleFG = scaleColorAlpha(theme.DisabledText, alpha)
	} else if active {
		titleFG = scaleColorAlpha(theme.ActiveText, alpha)
	} else if titleHovered {
		titleFG = scaleColorAlpha(theme.HoverText, alpha)
	}

	level := min(entry.Level, 6)
	indentW := gtx.Dp(unit.Dp(level * 12))
	disclosureW := gtx.Dp(unit.Dp(22))
	return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
		return fillBgExact(gtx, rowBG, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(indentW, rowH)}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !entry.HasChildren {
						return layout.Dimensions{Size: image.Pt(disclosureW, rowH)}
					}
					return fixedWidth(gtx, disclosureW, func(gtx layout.Context) layout.Dimensions {
						return disclosureClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							pointer.CursorPointer.Add(gtx.Ops)
							bg := color.NRGBA{}
							fg := scaleColorAlpha(theme.Muted, alpha)
							if disclosureHovered {
								hoverBG := theme.HoverBg
								if active {
									hoverBG = mixNRGBA(theme.ActiveBg, theme.HoverBg, 0.64)
								}
								bg = scaleColorAlpha(hoverBG, alpha)
								fg = scaleColorAlpha(bestContrastColor(hoverBG, theme.HoverText, theme.ActiveText, theme.Text), alpha)
							}
							return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
								return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
									lbl := material.Body2(th, viewerTOCDisclosureGlyph(st, entry))
									lbl.Font.Typeface = ui.mainTypeface()
									lbl.Font.Weight = font.Bold
									lbl.TextSize = ui.functionBarTextSize()
									lbl.Color = fg
									return layout.Center.Layout(gtx, lbl.Layout)
								})
							})
						})
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					titleGtx := gtx
					if !navigable {
						titleGtx = titleGtx.Disabled()
					}
					return titleClick.Layout(titleGtx, func(gtx layout.Context) layout.Dimensions {
						if navigable {
							pointer.CursorPointer.Add(gtx.Ops)
						}
						return fixedWidth(gtx, gtx.Constraints.Max.X, func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
								return fillBgExact(gtx, titleBG, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(3), Right: unit.Dp(4), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										lbl := material.Body2(th, entry.Title)
										lbl.Font.Typeface = ui.mainTypeface()
										lbl.Font.Weight = font.Medium
										lbl.TextSize = ui.functionBarTextSize()
										lbl.Color = titleFG
										lbl.MaxLines = 1
										lbl.Truncator = "…"
										return layoutVCenteredLabel(gtx, lbl)
									})
								})
							})
						})
					})
				}),
			)
		})
	})
}

func (ui *UI) layoutFileViewerTOCMenu(th *material.Theme, gtx layout.Context, st *fileViewerState, alpha float32) layout.Dimensions {
	ensureViewerTOCClicks(st)
	visible := viewerTOCVisibleIndices(st)
	if len(visible) == 0 {
		return layout.Dimensions{}
	}
	theme := ui.filePanePopupTheme()
	width := gtx.Dp(unit.Dp(292))
	if width > gtx.Constraints.Max.X {
		width = gtx.Constraints.Max.X
	}
	rowH := ui.fileContextMenuRowHeight(gtx, fileContextMenuItem{Label: "Entry"})
	height := rowH * len(visible)
	if maxH := gtx.Dp(unit.Dp(320)); height > maxH {
		height = maxH
	}
	if height > gtx.Constraints.Max.Y {
		height = gtx.Constraints.Max.Y
	}
	if height < 1 {
		height = 1
	}
	st.tocList.Axis = layout.Vertical
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
			return fillRoundedClipBox(
				gtx,
				gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
				scaleColorAlpha(theme.Bg, alpha),
				scaleColorAlpha(theme.Border, alpha),
				func(gtx layout.Context) layout.Dimensions {
					return material.List(th, &st.tocList).Layout(gtx, len(visible), func(gtx layout.Context, row int) layout.Dimensions {
						i := visible[row]
						return ui.layoutFileViewerTOCRow(th, gtx, st, i, rowH, theme, alpha)
					})
				},
			)
		})
	})
}

func (ui *UI) layoutFileViewerEncodingMenu(th *material.Theme, gtx layout.Context, st *fileViewerState, alpha float32) layout.Dimensions {
	theme := ui.filePanePopupTheme()
	type menuRow struct {
		click  *widget.Clickable
		item   fileContextMenuItem
		active bool
	}
	rows := []menuRow{
		{
			click: &st.encodingAutoClick,
			item: fileContextMenuItem{
				ID:     "viewer-encoding-auto",
				Label:  "Auto Detect",
				Detail: viewerEncodingAutoDetail(st),
			},
			active: st.fileEncoding == fm.ViewerFileEncodingAuto,
		},
		{
			click: &st.encodingUTF8Click,
			item: fileContextMenuItem{
				ID:    "viewer-encoding-utf8",
				Label: "UTF-8",
			},
			active: st.fileEncoding == fm.ViewerFileEncodingUTF8,
		},
		{
			click: &st.encodingUTF16LEClick,
			item: fileContextMenuItem{
				ID:    "viewer-encoding-utf16le",
				Label: "UTF-16 LE",
			},
			active: st.fileEncoding == fm.ViewerFileEncodingUTF16LE,
		},
		{
			click: &st.encodingUTF16BEClick,
			item: fileContextMenuItem{
				ID:    "viewer-encoding-utf16be",
				Label: "UTF-16 BE",
			},
			active: st.fileEncoding == fm.ViewerFileEncodingUTF16BE,
		},
		{
			click: &st.encodingCP437Click,
			item: fileContextMenuItem{
				ID:     "viewer-encoding-cp437",
				Label:  "CP437",
				Detail: "DOS / scene NFO",
			},
			active: st.fileEncoding == fm.ViewerFileEncodingCP437,
		},
	}
	width := gtx.Dp(unit.Dp(188))
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedClipBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			scaleColorAlpha(theme.Bg, alpha),
			scaleColorAlpha(theme.Border, alpha),
			func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(rows)*2)
				for i := range rows {
					if i > 0 {
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fillBgExact(gtx, scaleColorAlpha(theme.Divider, alpha), func(gtx layout.Context) layout.Dimensions {
								return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
							})
						}))
					}
					row := rows[i]
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						hoverFill := float32(0)
						if row.click.Hovered() {
							hoverFill = 1
						}
						dims, _, _ := ui.layoutFilePaneContextMenuItem(th, gtx, theme, row.click, row.item, row.active, hoverFill, alpha, ui.fileContextMenuRowHeight(gtx, row.item))
						return dims
					}))
				}
				return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			},
		)
	})
}

func viewerEncodingStatusLabel(st *fileViewerState) string {
	if st == nil {
		return ""
	}
	if st.detectedImagePreview {
		if label := viewerImageFormatDisplayName(st.imagePreviewFormat); label != "" {
			return label
		}
		return "Image"
	}
	if st.detectedBinaryPreview {
		return "Binary"
	}
	enc := st.detectedEncoding
	if enc == "" {
		enc = fm.NormalizeViewerFileEncoding(st.fileEncoding)
		if enc == fm.ViewerFileEncodingAuto {
			return "Auto"
		}
	}
	label := viewerEncodingDisplayName(enc)
	if label == "" {
		return ""
	}
	if st.detectedEncodingBOM {
		label += " BOM"
	}
	if st.fileEncoding == fm.ViewerFileEncodingAuto {
		label += " Auto"
	}
	return label
}

func viewerEncodingAutoDetail(st *fileViewerState) string {
	if st != nil && st.detectedImagePreview {
		if label := viewerImageFormatDisplayName(st.imagePreviewFormat); label != "" {
			return "Detected " + label + " image"
		}
		return "Detected image"
	}
	if st != nil && st.detectedBinaryPreview {
		return "Detected binary data"
	}
	if st == nil || st.detectedEncoding == "" {
		return "Detect UTF-8 / UTF-16 / CP437"
	}
	label := viewerEncodingDisplayName(st.detectedEncoding)
	if label == "" {
		return "Detect UTF-8 / UTF-16 / CP437"
	}
	if st.detectedEncodingBOM {
		label += " BOM"
	}
	return "Detected " + label
}

func viewerEncodingDisplayName(encoding string) string {
	switch fm.NormalizeViewerFileEncoding(encoding) {
	case fm.ViewerFileEncodingUTF16LE:
		return "UTF-16LE"
	case fm.ViewerFileEncodingUTF16BE:
		return "UTF-16BE"
	case fm.ViewerFileEncodingCP437:
		return "CP437"
	case fm.ViewerFileEncodingAuto:
		return "Auto"
	default:
		return "UTF-8"
	}
}

func viewerImageFormatDisplayName(format string) string {
	switch normalizeViewerImageFormat(format) {
	case "png":
		return "PNG"
	case "jpeg":
		return "JPEG"
	case "gif":
		return "GIF"
	case "pdf":
		return "PDF"
	default:
		return ""
	}
}

func viewerImageZoomLabel(st *fileViewerState) string {
	if st == nil || !st.detectedImagePreview {
		return ""
	}
	if viewerPDFPreviewActive(st) {
		return fmt.Sprintf("%.0f%%", float64(st.pdfDoc.effectiveZoom()*100))
	}
	return fmt.Sprintf("%.0f%%", float64(st.imageView.effectiveZoom()*100))
}

func viewerLineEndingLabel(kind string) string {
	switch kind {
	case viewerLineEndingCRLF:
		return "CRLF"
	case viewerLineEndingLF:
		return "LF"
	case viewerLineEndingMixed:
		return "Mixed EOL"
	case viewerLineEndingNone:
		return "No EOL"
	default:
		return ""
	}
}

func (ui *UI) viewerHeaderStripHeight(gtx layout.Context) int {
	h := gtx.Sp(ui.viewerTextSize()) + gtx.Dp(unit.Dp(8))
	if tabsH := gtx.Sp(ui.tabStripTextSize()) + gtx.Dp(unit.Dp(8)); tabsH > h {
		h = tabsH
	}
	if h < gtx.Dp(unit.Dp(24)) {
		h = gtx.Dp(unit.Dp(24))
	}
	return h
}

func (ui *UI) fileViewerHeaderTitle(st *fileViewerState) string {
	if st == nil {
		return "viewer"
	}
	if st.name != "" {
		if st.editDirty {
			return st.name + " *"
		}
		return st.name
	}
	if st.path != "" {
		return st.path
	}
	return "viewer"
}

func isViewerHeaderSizeStatus(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" || !strings.HasSuffix(status, " bytes") {
		return false
	}
	return strings.HasPrefix(status, "file: ") || strings.HasPrefix(status, "remote file: ")
}

func (ui *UI) fileViewerBaseStatusText(st *fileViewerState) (string, color.NRGBA) {
	if st == nil {
		return "", ui.fileViewerTheme().Hint
	}
	theme := ui.fileViewerTheme()
	statusText := ""
	statusColor := theme.Hint
	switch st.status {
	case "", "ready":
	case "editing":
	case "modified":
	case "loading...":
		statusText = "loading"
		statusColor = theme.StatusWarn
	case "update pending":
		statusText = "pending"
		statusColor = theme.StatusWarn
	case "nothing to copy":
		statusText = "nothing to copy"
		statusColor = theme.StatusError
	case "truncated":
		statusText = "truncated"
		statusColor = theme.StatusWarn
	case "streaming":
		statusText = "streaming"
		statusColor = theme.StatusAccent
	case "streaming, truncated":
		statusText = "streaming, truncated"
		statusColor = theme.StatusWarn
	default:
		if !isViewerHeaderSizeStatus(st.status) {
			statusText = st.status
			statusColor = theme.StatusAccent
		}
	}
	return statusText, statusColor
}

func isViewerStreamingStatus(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "streaming" || status == "streaming, truncated"
}

func (ui *UI) fileViewerHeaderStatusText(st *fileViewerState) (string, color.NRGBA) {
	if st == nil {
		return "", ui.fileViewerTheme().Hint
	}
	theme := ui.fileViewerTheme()
	statusText, statusColor := ui.fileViewerBaseStatusText(st)
	if st.commandOnly {
		if st.commandInfinite {
			return "streaming", theme.StatusAccent
		}
		return statusText, statusColor
	}
	if st.mode == "command" && st.commandInfinite {
		return "streaming", theme.StatusAccent
	}
	if st.mode == "command" {
		if !st.autoRefresh {
			return "no-refresh", theme.Muted
		}
		return "refreshing", theme.StatusAccent
	}
	return statusText, statusColor
}

func (ui *UI) fileViewerOverlayStatusText(st *fileViewerState) (string, color.NRGBA) {
	if st == nil {
		return "", ui.fileViewerTheme().Hint
	}
	return ui.fileViewerHeaderStatusText(st)
}

func (ui *UI) fileViewerHeaderDetails(st *fileViewerState) []viewerHeaderDetailPart {
	if st == nil {
		return nil
	}
	statusText, statusColor := ui.fileViewerHeaderStatusText(st)
	parts := make([]viewerHeaderDetailPart, 0, 1)
	if statusText != "" {
		parts = append(parts, viewerHeaderDetailPart{
			Text:  statusText,
			Color: statusColor,
		})
	}
	return parts
}

func measureWidgetUnconstrained(gtx layout.Context, w layout.Widget) layout.Dimensions {
	gtx2 := gtx
	var measureOps op.Ops
	gtx2.Ops = &measureOps
	gtx2.Constraints = layout.Constraints{Min: image.Point{}, Max: image.Point{X: 1 << 30, Y: 1 << 30}}
	return w(gtx2)
}

func maxWidth(gtx layout.Context, w int, wid layout.Widget) layout.Dimensions {
	if w <= 0 {
		return wid(gtx)
	}
	gtx2 := gtx
	if gtx2.Constraints.Max.X > w {
		gtx2.Constraints.Max.X = w
	}
	if gtx2.Constraints.Min.X > gtx2.Constraints.Max.X {
		gtx2.Constraints.Min.X = gtx2.Constraints.Max.X
	}
	return wid(gtx2)
}

func (ui *UI) layoutFileViewerInfoStrip(th *material.Theme, gtx layout.Context, st *fileViewerState, stripH int) layout.Dimensions {
	title := ui.fileViewerHeaderTitle(st)
	details := ui.fileViewerHeaderDetails(st)
	theme := ui.fileViewerTheme()
	titleLbl := material.Body2(th, title)
	titleLbl.Font.Typeface = ui.viewerTypeface()
	titleLbl.Font.Weight = font.Medium
	titleLbl.TextSize = ui.viewerTextSize()
	titleW := measureLabelUnconstrained(gtx, titleLbl).Size.X
	titleW += gtx.Dp(unit.Dp(2))
	if maxTitleW := gtx.Dp(unit.Dp(220)); titleW > maxTitleW {
		titleW = maxTitleW
	}
	buttonW := measureWidgetUnconstrained(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.layoutFileViewerInfoButtons(th, gtx, st, stripH)
	}).Size.X
	spaceW := gtx.Dp(unit.Dp(7))
	dividerW := gtx.Dp(unit.Dp(1))
	if dividerW < 1 {
		dividerW = 1
	}
	detailAvail := gtx.Constraints.Max.X - titleW - buttonW - spaceW
	if len(details) > 0 {
		detailAvail -= spaceW + dividerW + spaceW
	}
	if detailAvail < 0 {
		detailAvail = 0
	}
	statusW := 0
	pulledW := 0
	if len(details) == 1 {
		lbl := material.Body2(th, details[0].Text)
		lbl.Font.Typeface = ui.viewerTypeface()
		lbl.TextSize = ui.viewerTextSize()
		statusW = measureLabelUnconstrained(gtx, lbl).Size.X + gtx.Dp(unit.Dp(2))
		if statusW > detailAvail {
			statusW = detailAvail
		}
	} else if len(details) >= 2 {
		statusLbl := material.Body2(th, details[0].Text)
		statusLbl.Font.Typeface = ui.viewerTypeface()
		statusLbl.TextSize = ui.viewerTextSize()
		statusPreferred := measureLabelUnconstrained(gtx, statusLbl).Size.X + gtx.Dp(unit.Dp(2))

		pulledLbl := material.Body2(th, details[1].Text)
		pulledLbl.Font.Typeface = ui.viewerTypeface()
		pulledLbl.TextSize = ui.viewerTextSize()
		pulledPreferred := measureLabelUnconstrained(gtx, pulledLbl).Size.X + gtx.Dp(unit.Dp(2))

		innerAvail := detailAvail - (spaceW + dividerW + spaceW)
		if innerAvail < 0 {
			innerAvail = 0
		}
		pulledW = pulledPreferred
		if pulledW > innerAvail {
			pulledW = innerAvail
		}
		statusW = innerAvail - pulledW
		if statusW > statusPreferred {
			statusW = statusPreferred
		}
		if statusW < 0 {
			statusW = 0
		}
		remaining := innerAvail - statusW
		if remaining < 0 {
			remaining = 0
		}
		if pulledW > remaining {
			pulledW = remaining
		}
		if pulledW < 0 {
			pulledW = 0
		}
	}
	return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, titleW, func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
						return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, title)
								lbl.Font.Typeface = ui.viewerTypeface()
								lbl.Font.Weight = font.Medium
								lbl.TextSize = ui.viewerTextSize()
								lbl.Color = theme.HeaderText
								lbl.MaxLines = 1
								lbl.Truncator = "..."
								return lbl.Layout(gtx)
							})
						})
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if len(details) == 0 {
					return layout.Dimensions{}
				}
				return layout.Inset{Left: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutFileViewerInfoDivider(gtx, stripH)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(7)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							children := make([]layout.FlexChild, 0, 3)
							if len(details) > 0 {
								children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									w := statusW
									if len(details) == 1 {
										w = statusW
									}
									return fixedWidth(gtx, w, func(gtx layout.Context) layout.Dimensions {
										return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
											return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
													lbl := material.Body2(th, details[0].Text)
													lbl.Font.Typeface = ui.viewerTypeface()
													lbl.TextSize = ui.viewerTextSize()
													lbl.Color = details[0].Color
													lbl.MaxLines = 1
													lbl.Truncator = "..."
													return lbl.Layout(gtx)
												})
											})
										})
									})
								}))
							}
							if len(details) >= 2 {
								children = append(children,
									layout.Rigid(layout.Spacer{Width: unit.Dp(7)}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return ui.layoutFileViewerInfoDivider(gtx, stripH)
									}),
									layout.Rigid(layout.Spacer{Width: unit.Dp(7)}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return fixedWidth(gtx, pulledW, func(gtx layout.Context) layout.Dimensions {
											return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
												return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
													return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
														lbl := material.Body2(th, details[1].Text)
														lbl.Font.Typeface = ui.viewerTypeface()
														lbl.TextSize = ui.viewerTextSize()
														lbl.Color = details[1].Color
														lbl.MaxLines = 1
														lbl.Truncator = "..."
														return lbl.Layout(gtx)
													})
												})
											})
										})
									}),
								)
							}
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
						}),
					)
				})
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(7)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFileViewerInfoButtons(th, gtx, st, stripH)
			}),
		)
	})
}

func (ui *UI) layoutFileViewerInfoDivider(gtx layout.Context, stripH int) layout.Dimensions {
	theme := ui.fileViewerTheme()
	return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			h := stripH - gtx.Dp(unit.Dp(12))
			if h < gtx.Dp(unit.Dp(8)) {
				h = gtx.Dp(unit.Dp(8))
			}
			w := gtx.Dp(unit.Dp(1))
			if w < 1 {
				w = 1
			}
			paint.FillShape(gtx.Ops, theme.Divider, clip.Rect(image.Rect(0, 0, w, h)).Op())
			return layout.Dimensions{Size: image.Pt(w, h)}
		})
	})
}

func (ui *UI) viewerShowsAutoRefreshButton(st *fileViewerState) bool {
	return st != nil && st.mode == "command" && !st.commandOnly && !st.commandInfinite
}

func (ui *UI) layoutFileViewerInfoButtons(th *material.Theme, gtx layout.Context, st *fileViewerState, stripH int) layout.Dimensions {
	return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if !ui.viewerShowsAutoRefreshButton(st) {
				return ui.layoutFlatCloseButton(gtx, &st.closeClick, false)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutTinyIconModeButton(th, gtx, &st.autoRefreshClick, uitheme.RefreshIcon(), st.autoRefresh)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutFlatCloseButton(gtx, &st.closeClick, false)
				}),
			)
		})
	})
}

func (ui *UI) layoutFileViewerHeaderSegment(th *material.Theme, gtx layout.Context, label string, bg, fg color.NRGBA, bold, roundLeft, roundRight, truncate bool, _ int) layout.Dimensions {
	return fillSegmentBg(gtx, bg, gtx.Dp(unit.Dp(filePaneControlCornerDp-1)), roundLeft, roundRight, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(9), Right: unit.Dp(9)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = ui.viewerTypeface()
				if bold {
					lbl.Font.Weight = font.Medium
				}
				lbl.TextSize = ui.viewerTextSize()
				lbl.Color = fg
				lbl.MaxLines = 1
				if truncate {
					lbl.Truncator = "..."
				}
				return lbl.Layout(gtx)
			})
		})
	})
}

func viewerFilenameRailTitle(st *fileViewerState) string {
	if st == nil {
		return ""
	}
	name := strings.TrimSpace(st.name)
	if name == "" {
		name = strings.TrimSpace(st.path)
	}
	if name == "" {
		return "viewer"
	}
	name = middleTruncateRunes(name, viewerModeTabFilenameMaxRunes)
	if st.editDirty {
		return name + " *"
	}
	return name
}

func middleTruncateRunes(value string, maxRunes int) string {
	runes := []rune(value)
	if maxRunes <= 0 {
		return ""
	}
	if len(runes) <= maxRunes {
		return value
	}
	if maxRunes == 1 {
		return "…"
	}
	left := (maxRunes - 1 + 1) / 2
	right := maxRunes - 1 - left
	return string(runes[:left]) + "…" + string(runes[len(runes)-right:])
}

type viewerModeTabSpec struct {
	label        string
	active       bool
	click        *widget.Clickable
	actionIcon   *widget.Icon
	actionClick  *widget.Clickable
	reserveSides bool
	index        int
}

func (ui *UI) viewerModeTabWidth(th *material.Theme, gtx layout.Context, item viewerModeTabSpec) int {
	face := ui.tabStripTypeface()
	size := ui.tabStripTextSize()
	markerW := tabStripTitleTextWidth(th, gtx, face, size, "█")
	labelW := 0
	for _, label := range [...]string{"File", "Hex", "Cmd"} {
		labelW = max(labelW, tabStripTitleTextWidth(th, gtx, face, size, label))
	}
	width := gtx.Dp(unit.Dp(viewerModeTabOuterInsetDp*2+viewerModeTabMarkerGapDp)) + markerW + labelW
	if item.reserveSides {
		width += gtx.Dp(unit.Dp((viewerModeTabActionWidthDp + viewerModeTabLabelGapDp) * 2))
	}
	minW := gtx.Dp(unit.Dp(38))
	if width < minW {
		width = minW
	}
	return width
}

func (ui *UI) layoutFileViewerModeAction(gtx layout.Context, click *widget.Clickable, icon *widget.Icon, fg color.NRGBA) layout.Dimensions {
	if click == nil || icon == nil {
		return layout.Dimensions{}
	}
	return fixedWidth(gtx, gtx.Dp(unit.Dp(viewerModeTabActionWidthDp)), func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			highlighted := click.Hovered() || click.Pressed()
			ui.drawFileViewerModeActionSpot(gtx, highlighted)
			iconColor := fg
			iconColor.A = 255
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				iconSize := gtx.Dp(unit.Dp(14))
				if iconSize < 10 {
					iconSize = 10
				}
				iconGtx := gtx
				iconGtx.Constraints = layout.Exact(image.Pt(iconSize, iconSize))
				icon.Layout(iconGtx, iconColor)
				return layout.Dimensions{Size: image.Pt(iconSize, iconSize)}
			})
		})
	})
}

func (ui *UI) drawFileViewerModeActionSpot(gtx layout.Context, highlighted bool) {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	// Reserve the bottom rows for the tab's ASCII underline.
	bottom := size.Y - gtx.Dp(unit.Dp(3))
	if bottom < 1 {
		bottom = 1
	}
	palette := filePanePaletteFromConfig(ui.fmCfg)
	bg := mixNRGBA(palette.PaneBg, color.NRGBA{R: 82, G: 88, B: 104, A: 255}, 0.52)
	bg.A = 54
	if highlighted {
		bg = mixNRGBA(palette.PaneBg, color.NRGBA{R: 43, G: 129, B: 157, A: 255}, 0.76)
		bg.A = 220
	}
	paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rect(0, 0, size.X, bottom)).Op())
	separator := ui.tabStripSeparatorColor()
	separator.A = 96
	separatorTop := bottom / 4
	separatorBottom := bottom - bottom/4
	if separatorBottom > separatorTop {
		paint.FillShape(gtx.Ops, separator, clip.Rect(image.Rect(0, separatorTop, 1, separatorBottom)).Op())
	}
}

func (ui *UI) layoutFileViewerModeTab(th *material.Theme, gtx layout.Context, st *fileViewerState, item viewerModeTabSpec) layout.Dimensions {
	if item.click == nil {
		return layout.Dimensions{}
	}
	return item.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		bg, fg := ui.tabStripColors(item.active, item.click.Hovered(), item.index)
		palette := filePanePaletteFromConfig(ui.fmCfg)
		if item.active {
			bg = mixNRGBA(palette.PaneBg, palette.HoverBg, 0.38)
			bg.A = 138
			if item.click.Hovered() {
				bg = mixNRGBA(palette.PaneBg, palette.HoverBg, 0.58)
				bg.A = 184
			}
		} else if item.click.Hovered() {
			bg = mixNRGBA(palette.PaneBg, palette.HoverBg, 0.82)
			bg.A = 194
		}
		if bg.A != 0 {
			paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())
		}
		contentDims := layout.Inset{
			Left:   unit.Dp(viewerModeTabOuterInsetDp),
			Right:  unit.Dp(viewerModeTabOuterInsetDp),
			Top:    unit.Dp(1),
			Bottom: unit.Dp(1),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					marker := "░"
					markerColor := fg
					markerColor.A = 132
					if item.active {
						marker = "█"
						markerColor = fg
						markerColor.A = 255
					}
					lbl := material.Body2(th, marker)
					lbl.Font.Typeface = ui.tabStripTypeface()
					lbl.Font.Weight = font.Bold
					lbl.TextSize = ui.tabStripTextSize()
					lbl.Color = markerColor
					lbl.MaxLines = 1
					return layoutVCenteredLabel(gtx, lbl)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(viewerModeTabMarkerGapDp)}.Layout),
			}
			if item.reserveSides {
				children = append(children,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if item.actionIcon == nil || item.actionClick == nil {
							return layout.Spacer{Width: unit.Dp(viewerModeTabActionWidthDp)}.Layout(gtx)
						}
						return ui.layoutFileViewerModeAction(gtx, item.actionClick, item.actionIcon, fg)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(viewerModeTabLabelGapDp)}.Layout),
				)
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, item.label)
				lbl.Font.Typeface = ui.tabStripTypeface()
				lbl.Font.Weight = font.Medium
				lbl.TextSize = ui.tabStripTextSize()
				lbl.Color = fg
				lbl.MaxLines = 1
				return layoutVCenteredLabel(gtx, lbl)
			}))
			if item.reserveSides {
				children = append(children,
					layout.Rigid(layout.Spacer{Width: unit.Dp(viewerModeTabLabelGapDp)}.Layout),
					layout.Rigid(layout.Spacer{Width: unit.Dp(viewerModeTabActionWidthDp)}.Layout),
				)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		})
		if item.active && item.index < 3 {
			ui.layoutFileViewerModeASCIIUnderline(th, gtx, item, fg)
		}
		return contentDims
	})
}

func (ui *UI) layoutFileViewerModeASCIIUnderline(th *material.Theme, gtx layout.Context, item viewerModeTabSpec, fg color.NRGBA) {
	markerW := tabStripTitleTextWidth(th, gtx, ui.tabStripTypeface(), ui.tabStripTextSize(), "█")
	x := gtx.Dp(unit.Dp(viewerModeTabOuterInsetDp+viewerModeTabMarkerGapDp)) + markerW
	availableW := gtx.Constraints.Max.X - x
	if availableW <= 0 {
		return
	}
	probe := material.Body2(th, "_")
	probe.Font.Typeface = ui.tabStripTypeface()
	probe.Font.Weight = font.Medium
	probe.TextSize = ui.tabStripTextSize()
	probe.Color = fg
	probe.Color.A = 224
	probe.MaxLines = 1
	charW := measureLabelUnconstrained(gtx, probe).Size.X
	if charW < 1 {
		charW = 1
	}
	count := availableW/charW + 1
	if count < 1 {
		return
	}
	underline := material.Body2(th, strings.Repeat("_", count))
	underline.Font = probe.Font
	underline.TextSize = probe.TextSize
	underline.Color = probe.Color
	underline.MaxLines = 1
	underlineDims := measureLabelUnconstrained(gtx, underline)
	y := gtx.Constraints.Max.Y - underlineDims.Size.Y + gtx.Dp(unit.Dp(1))
	if y < 0 {
		y = 0
	}
	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	underlineClip := clip.Rect(image.Rect(0, 0, availableW, max(0, gtx.Constraints.Max.Y-y))).Push(gtx.Ops)
	underlineGtx := gtx
	underlineGtx.Constraints = layout.Exact(underlineDims.Size)
	underline.Layout(underlineGtx)
	underlineClip.Pop()
	offset.Pop()
}

func (ui *UI) layoutFileViewerModeTabs(th *material.Theme, gtx layout.Context, st *fileViewerState, stripH int) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	historyActive := st.historyOpen
	items := []viewerModeTabSpec{
		{label: "File", active: !historyActive && st.mode == "file", click: &st.modeFileClick, reserveSides: true, index: 0},
		{label: "Hex", active: !historyActive && st.mode == "hex", click: &st.modeHexClick, reserveSides: true, index: 1},
		{
			label:        "Cmd",
			active:       historyActive || st.mode == "command",
			click:        &st.modeCmdClick,
			reserveSides: true,
			index:        2,
		},
	}
	for i := range items {
		if !items[i].active {
			continue
		}
		if i == 2 {
			items[i].actionClick = &st.historyClick
			if historyActive {
				items[i].actionIcon = uitheme.EditModeIcon()
			} else {
				items[i].actionIcon = uitheme.ViewModeIcon()
			}
		} else if st.editMode {
			items[i].actionClick = &st.editToggleClick
			items[i].actionIcon = uitheme.EditModeIcon()
		} else {
			items[i].actionClick = &st.editToggleClick
			items[i].actionIcon = uitheme.ViewModeIcon()
		}
	}

	starts := make([]int, len(items))
	widths := make([]int, len(items))
	gapW := gtx.Dp(unit.Dp(viewerModeTabInterGapDp))
	terminatorMarkerW := tabStripTitleTextWidth(th, gtx, ui.tabStripTypeface(), ui.tabStripTextSize(), "░")
	terminatorW := gtx.Dp(unit.Dp(viewerModeTabOuterInsetDp)) + terminatorMarkerW
	totalW := 0
	for i, item := range items {
		starts[i] = totalW
		widths[i] = ui.viewerModeTabWidth(th, gtx, item)
		totalW += widths[i]
		totalW += gapW
	}
	totalW += terminatorW

	activeIdx := 0
	switch {
	case historyActive:
		activeIdx = 2
	case st.mode == "hex":
		activeIdx = 1
	case st.mode == "command":
		activeIdx = 2
	}
	st.activeTabRect = image.Rect(starts[activeIdx], 0, starts[activeIdx]+widths[activeIdx], stripH)

	return fixedWidth(gtx, totalW, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(items)*2+1)
			for i := range items {
				idx := i
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, widths[idx], func(gtx layout.Context) layout.Dimensions {
						return ui.layoutFileViewerModeTab(th, gtx, st, items[idx])
					})
				}))
				children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(viewerModeTabInterGapDp)}.Layout))
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, terminatorW, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(viewerModeTabOuterInsetDp)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						markerColor := ui.fileViewerTheme().Muted
						markerColor.A = 180
						lbl := material.Body2(th, "░")
						lbl.Font.Typeface = ui.tabStripTypeface()
						lbl.Font.Weight = font.Bold
						lbl.TextSize = ui.tabStripTextSize()
						lbl.Color = markerColor
						lbl.MaxLines = 1
						return layoutVCenteredLabel(gtx, lbl)
					})
				})
			}))
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

func (ui *UI) layoutFileViewerASCIIRail(th *material.Theme, gtx layout.Context, railColor color.NRGBA, align layout.Direction) layout.Dimensions {
	return layoutASCIIRail(th, gtx, railColor, align, ui.tabStripTypeface(), ui.tabStripTextSize())
}

func layoutASCIIRail(th *material.Theme, gtx layout.Context, railColor color.NRGBA, align layout.Direction, typeface font.Typeface, textSize unit.Sp) layout.Dimensions {
	w := gtx.Constraints.Max.X
	if w <= 0 {
		return layout.Dimensions{}
	}
	probe := material.Body2(th, "-")
	probe.Font.Typeface = typeface
	probe.Font.Weight = font.Normal
	probe.TextSize = textSize
	probe.Color = railColor
	probe.MaxLines = 1
	charW := measureLabelUnconstrained(gtx, probe).Size.X
	if charW < 1 {
		charW = 1
	}
	count := w / charW
	if count < 1 {
		return layout.Dimensions{Size: image.Pt(w, 0)}
	}
	rail := probe
	rail.Text = strings.Repeat("-", count)
	rail.Truncator = ""
	textW := measureLabelUnconstrained(gtx, rail).Size.X
	for count > 0 && textW > w {
		count--
		rail.Text = strings.Repeat("-", count)
		textW = measureLabelUnconstrained(gtx, rail).Size.X
	}
	x := 0
	if align == layout.E {
		x = w - textW
	}
	defer clip.Rect(image.Rect(0, 0, w, max(1, gtx.Constraints.Max.Y))).Push(gtx.Ops).Pop()
	offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
	childGtx := gtx
	childGtx.Constraints.Min.X = 0
	childGtx.Constraints.Max.X = textW
	dims := rail.Layout(childGtx)
	offset.Pop()
	dims.Size.X = w
	return dims
}

func fitViewerFilenameLabel(gtx layout.Context, label material.LabelStyle, maxWidth int) (material.LabelStyle, int) {
	if maxWidth <= 0 {
		label.Text = ""
		label.Truncator = ""
		return label, 0
	}
	label.Truncator = ""
	if width := measureLabelUnconstrained(gtx, label).Size.X; width <= maxWidth {
		return label, width
	}
	original := label.Text
	for runeCount := len([]rune(original)) - 1; runeCount > 0; runeCount-- {
		label.Text = middleTruncateRunes(original, runeCount)
		width := measureLabelUnconstrained(gtx, label).Size.X
		if width <= maxWidth {
			return label, width
		}
	}
	label.Text = ""
	return label, 0
}

func viewerFilenameRailSideWidths(railWidth, titleCenterX, groupWidth, minRailWidth int) (left, right int) {
	minRailWidth = min(max(0, minRailWidth), railWidth/2)
	maxGroupWidth := max(0, railWidth-2*minRailWidth)
	groupWidth = min(max(0, groupWidth), maxGroupWidth)
	left = titleCenterX - groupWidth/2
	left = min(max(minRailWidth, left), railWidth-minRailWidth-groupWidth)
	right = max(0, railWidth-left-groupWidth)
	return left, right
}

func (ui *UI) layoutFileViewerFilenameRail(th *material.Theme, gtx layout.Context, st *fileViewerState, stripH, titleCenterX int) layout.Dimensions {
	title := viewerFilenameRailTitle(st)
	theme := ui.fileViewerTheme()
	return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			capColor := theme.Muted
			capColor.A = 182
			titleColor := theme.HeaderText
			if st != nil && st.editDirty {
				titleColor = theme.ModifiedText
			}
			leftCap := material.Body2(th, "[")
			leftCap.Font.Typeface = ui.tabStripTypeface()
			leftCap.Font.Weight = font.Normal
			leftCap.TextSize = ui.tabStripTextSize()
			leftCap.Color = capColor
			leftCap.MaxLines = 1
			rightCap := material.Body2(th, "]")
			rightCap.Font = leftCap.Font
			rightCap.TextSize = leftCap.TextSize
			rightCap.Color = capColor
			rightCap.MaxLines = 1
			titleLabel := material.Body2(th, title)
			titleLabel.Font.Typeface = ui.tabStripTypeface()
			titleLabel.Font.Weight = font.Medium
			titleLabel.TextSize = ui.tabStripTextSize()
			titleLabel.Color = titleColor
			titleLabel.MaxLines = 1
			titleLabel.Alignment = text.Middle

			leftCapW := measureLabelUnconstrained(gtx, leftCap).Size.X
			rightCapW := measureLabelUnconstrained(gtx, rightCap).Size.X
			railW := gtx.Constraints.Max.X
			titleCenterX = min(max(0, titleCenterX), railW)
			hyphenProbe := leftCap
			hyphenProbe.Text = "-"
			hyphenW := max(1, measureLabelUnconstrained(gtx, hyphenProbe).Size.X)
			maxTitleW := max(0, railW-leftCapW-rightCapW-2*hyphenW)
			titleLabel, titleW := fitViewerFilenameLabel(gtx, titleLabel, maxTitleW)
			groupW := leftCapW + titleW + rightCapW
			leftRailW, rightRailW := viewerFilenameRailSideWidths(railW, titleCenterX, groupW, hyphenW)
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, leftRailW, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutFileViewerASCIIRail(th, gtx, capColor, layout.E)
					})
				}),
				layout.Rigid(leftCap.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if titleW == 0 {
						return layout.Dimensions{}
					}
					return fixedWidth(gtx, titleW, titleLabel.Layout)
				}),
				layout.Rigid(rightCap.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, rightRailW, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutFileViewerASCIIRail(th, gtx, capColor, layout.W)
					})
				}),
			)
		})
	})
}

func viewerHeaderFilenameRailBounds(width, leftWidth, rightWidth, gap int) (start, railWidth, titleCenterX int) {
	if width <= 0 {
		return 0, 0, 0
	}
	start = min(width, leftWidth+max(0, gap))
	end := max(start, width-rightWidth)
	railWidth = end - start
	titleCenterX = min(max(0, width/2-start), railWidth)
	return start, railWidth, titleCenterX
}

func (ui *UI) layoutFileViewerHeaderRow(th *material.Theme, gtx layout.Context, st *fileViewerState, stripH int) layout.Dimensions {
	if st != nil && st.commandOnly {
		m := op.Record(gtx.Ops)
		dims := layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return ui.layoutFileViewerInfoStrip(th, gtx, st, stripH)
		})
		call := m.Stop()
		ui.paintFileViewerHeaderDivider(gtx, dims.Size, st)
		call.Add(gtx.Ops)
		return dims
	}
	m := op.Record(gtx.Ops)
	dims := layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		rowW := gtx.Constraints.Max.X
		if rowW < 0 {
			rowW = 0
		}
		childGtx := gtx
		childGtx.Constraints.Min.X = 0
		childGtx.Constraints.Min.Y = 0

		leftRecord := op.Record(gtx.Ops)
		leftChildren := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFileViewerModeTabs(th, gtx, st, stripH)
			}),
		}
		if st.mode == "command" {
			leftChildren = append(leftChildren,
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutFileViewerInlineCommand(th, gtx, st, stripH)
				}),
			)
		}
		leftDims := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(childGtx, leftChildren...)
		leftCall := leftRecord.Stop()

		rightRecord := op.Record(gtx.Ops)
		rightDims := ui.layoutFileViewerInfoButtons(th, childGtx, st, stripH)
		rightCall := rightRecord.Stop()

		railStart, railW, titleCenterX := viewerHeaderFilenameRailBounds(rowW, leftDims.Size.X, rightDims.Size.X, 0)
		railRecord := op.Record(gtx.Ops)
		railGtx := childGtx
		railGtx.Constraints = layout.Exact(image.Pt(railW, stripH))
		ui.layoutFileViewerFilenameRail(th, railGtx, st, stripH, titleCenterX)
		railCall := railRecord.Stop()

		railOffset := op.Offset(image.Pt(railStart, 0)).Push(gtx.Ops)
		railCall.Add(gtx.Ops)
		railOffset.Pop()
		leftCall.Add(gtx.Ops)
		rightOffset := op.Offset(image.Pt(max(0, rowW-rightDims.Size.X), 0)).Push(gtx.Ops)
		rightCall.Add(gtx.Ops)
		rightOffset.Pop()
		return layout.Dimensions{Size: image.Pt(rowW, stripH)}
	})
	call := m.Stop()
	ui.paintFileViewerHeaderDivider(gtx, dims.Size, st)
	call.Add(gtx.Ops)
	return dims
}

func (ui *UI) paintFileViewerHeaderDivider(gtx layout.Context, size image.Point, st *fileViewerState) {
	if size.X < 1 || size.Y < 1 {
		return
	}
	theme := ui.fileViewerTheme()
	paint.FillShape(gtx.Ops, theme.HeaderBg, clip.Rect(image.Rect(0, 0, size.X, size.Y)).Op())
	inset := gtx.Dp(unit.Dp(8))
	if inset*2 >= size.X {
		inset = 0
	}
	h := gtx.Dp(unit.Dp(1))
	if h < 1 {
		h = 1
	}
	y0 := size.Y - h
	if y0 < 0 {
		y0 = 0
	}
	baseRect := image.Rect(inset, y0, size.X-inset, size.Y)
	if baseRect.Dx() <= 0 || baseRect.Dy() <= 0 {
		return
	}
	gapMin := baseRect.Max.X
	gapMax := baseRect.Max.X
	if st != nil && st.activeTabRect.Dx() > 0 {
		gapMin = inset + st.activeTabRect.Min.X - 1
		gapMax = inset + st.activeTabRect.Max.X + 1
		if gapMin < baseRect.Min.X {
			gapMin = baseRect.Min.X
		}
		if gapMax > baseRect.Max.X {
			gapMax = baseRect.Max.X
		}
		if gapMax < gapMin {
			gapMax = gapMin
		}
	}
	if gapMin > baseRect.Min.X {
		paint.FillShape(gtx.Ops, theme.Divider, clip.Rect(image.Rect(baseRect.Min.X, baseRect.Min.Y, gapMin, baseRect.Max.Y)).Op())
	}
	if gapMax < baseRect.Max.X {
		paint.FillShape(gtx.Ops, theme.Divider, clip.Rect(image.Rect(gapMax, baseRect.Min.Y, baseRect.Max.X, baseRect.Max.Y)).Op())
	}
}

func (ui *UI) layoutFileViewerInlineCommand(th *material.Theme, gtx layout.Context, st *fileViewerState, stripH int) layout.Dimensions {
	if st == nil || st.mode != "command" {
		return layout.Dimensions{}
	}
	ui.handleFileViewerCommandAreaPresses(gtx, st)
	if st.commandEditOn {
		if st.commandFocus {
			st.commandFocus = false
			gtx.Execute(key.FocusCmd{Tag: &st.commandEditor})
		} else if !gtx.Focused(&st.commandEditor) {
			ui.cancelViewerCommandEdit()
		}
	}
	theme := ui.fileViewerTheme()
	fg := theme.CommandStaticText
	commandText := st.command
	if st.commandEditOn {
		commandText = st.commandEditor.Text()
	}
	desiredW := ui.fileViewerInlineCommandWidth(th, gtx, commandText)
	host := func(gtx layout.Context) layout.Dimensions {
		return fixedWidth(gtx, desiredW, func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
				plateH := stripH - gtx.Dp(unit.Dp(viewerInlineCommandVerticalInsetDp*2))
				if plateH < gtx.Dp(unit.Dp(16)) {
					plateH = gtx.Dp(unit.Dp(16))
				}
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, plateH, func(gtx layout.Context) layout.Dimensions {
						if st.commandEditOn {
							ed := material.Editor(th, &st.commandEditor, "cat {fullpath}")
							ed.Font.Typeface = ui.tabStripTypeface()
							ed.TextSize = ui.tabStripTextSize()
							ed.Color = theme.CommandText
							ed.HintColor = theme.CommandHint
							focused := st.commandFocus || gtx.Focused(&st.commandEditor)
							return ui.layoutFileViewerCommandPlate(gtx, theme, true, false, focused, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(viewerInlineCommandDisplayInsetDp), Right: unit.Dp(viewerInlineCommandDisplayInsetDp), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									gtx.Constraints.Min.X = gtx.Constraints.Max.X
									return layout.Center.Layout(gtx, ed.Layout)
								})
							})
						}
						label := commandText
						return st.commandClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							pointer.CursorText.Add(gtx.Ops)
							return ui.layoutFileViewerCommandPlate(gtx, theme, false, st.commandClick.Hovered(), false, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(viewerInlineCommandDisplayInsetDp), Right: unit.Dp(viewerInlineCommandDisplayInsetDp)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									gtx.Constraints.Min.X = gtx.Constraints.Max.X
									lbl := material.Body2(th, label)
									lbl.Font.Typeface = ui.tabStripTypeface()
									lbl.TextSize = ui.tabStripTextSize()
									lbl.Font.Weight = font.Medium
									lbl.Color = fg
									lbl.MaxLines = 1
									lbl.Truncator = "..."
									return layoutVCenteredLabel(gtx, lbl)
								})
							})
						})
					})
				})
			})
		})
	}
	if st.commandEditOn {
		hostWithGuard := func(gtx layout.Context) layout.Dimensions {
			dims := host(gtx)
			if dims.Size.X <= 0 || dims.Size.Y <= 0 {
				return dims
			}
			defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
			pass := pointer.PassOp{}.Push(gtx.Ops)
			event.Op(gtx.Ops, &st.commandAreaTag)
			pass.Pop()
			return dims
		}
		return ui.layoutEditorWithContextMenu(th, gtx, "viewer-command", &st.commandEditor, true, hostWithGuard)
	}
	return host(gtx)
}

func (ui *UI) layoutFileViewerCommandPlate(gtx layout.Context, theme fileViewerTheme, editable, hovered, focused bool, content layout.Widget) layout.Dimensions {
	if content == nil {
		return layout.Dimensions{}
	}
	bg := mixNRGBA(theme.CommandBg, theme.PanelBg, 0.28)
	edge := theme.CommandBorder
	if hovered {
		bg = mixNRGBA(theme.CommandBgHover, theme.PanelBg, 0.18)
		edge = theme.CommandBorderHover
	}
	if editable {
		bg = mixNRGBA(theme.CommandBgHover, theme.PanelBg, 0.12)
		edge = theme.CommandBorderHover
	}
	if focused {
		bg = mixNRGBA(bg, theme.CommandText, 0.07)
	}

	m := op.Record(gtx.Ops)
	gtx.Constraints.Min = gtx.Constraints.Max
	dims := content(gtx)
	call := m.Stop()
	if dims.Size.X < 1 || dims.Size.Y < 1 {
		call.Add(gtx.Ops)
		return dims
	}

	rect := image.Rectangle{Max: dims.Size}
	paint.FillShape(gtx.Ops, bg, clip.Rect(rect).Op())
	line := gtx.Dp(unit.Dp(1))
	if line < 1 {
		line = 1
	}
	edge.A = 92
	if hovered || editable {
		edge.A = 138
	}
	topEdge := edge
	topEdge.A = 118
	if hovered || editable {
		topEdge.A = 158
	}
	paint.FillShape(gtx.Ops, topEdge, clip.Rect(image.Rect(0, 0, dims.Size.X, line)).Op())
	paint.FillShape(gtx.Ops, edge, clip.Rect(image.Rect(0, dims.Size.Y-line, dims.Size.X, dims.Size.Y)).Op())
	paint.FillShape(gtx.Ops, edge, clip.Rect(image.Rect(0, line, line, dims.Size.Y-line)).Op())
	paint.FillShape(gtx.Ops, edge, clip.Rect(image.Rect(dims.Size.X-line, line, dims.Size.X, dims.Size.Y-line)).Op())
	if editable && focused {
		focusH := gtx.Dp(unit.Dp(2))
		if focusH < 1 {
			focusH = 1
		}
		focus := theme.CommandText
		focus.A = 214
		paint.FillShape(gtx.Ops, focus, clip.Rect(image.Rect(0, dims.Size.Y-focusH, dims.Size.X, dims.Size.Y)).Op())
	}
	call.Add(gtx.Ops)
	return dims
}

func (ui *UI) fileViewerInlineCommandWidth(th *material.Theme, gtx layout.Context, commandText string) int {
	if strings.TrimSpace(commandText) == "" {
		commandText = "cat {fullpath}"
	}
	measure := material.Body2(th, commandText)
	measure.Font.Typeface = ui.tabStripTypeface()
	measure.Font.Weight = font.Medium
	measure.TextSize = ui.tabStripTextSize()
	desiredW := measureLabelUnconstrained(gtx, measure).Size.X + gtx.Dp(unit.Dp(viewerInlineCommandMeasurePaddingDp))
	if desiredW < gtx.Dp(unit.Dp(viewerInlineCommandMinWidthDp)) {
		desiredW = gtx.Dp(unit.Dp(viewerInlineCommandMinWidthDp))
	}
	if maxW := gtx.Constraints.Max.X; maxW > 0 && desiredW > maxW {
		desiredW = maxW
	}
	return desiredW
}

func (ui *UI) layoutFileViewerHistoryList(th *material.Theme, gtx layout.Context, st *fileViewerState, history []string) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	const maxHistoryItems = 12
	if len(history) > maxHistoryItems {
		history = history[:maxHistoryItems]
	}
	for _, cmd := range history {
		click := st.historyClickable("viewer-history:" + cmd)
		if click == nil {
			continue
		}
		for click.Clicked(gtx) {
			ui.applyViewerHistoryCommand(cmd, gtx.Now)
		}
	}
	rows, _ := ui.fileViewerHistoryRows(th, gtx, history)
	listW := gtx.Constraints.Max.X
	if listW < 1 {
		listW = 1
	}

	theme := ui.fileViewerTheme()
	return fixedWidth(gtx, listW, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedBox(
			gtx,
			0,
			theme.HistoryBg,
			theme.HistoryBorder,
			func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFileViewerHistoryListRows(th, gtx, st, rows)
			},
		)
	})
}

func (ui *UI) layoutFileViewerHistoryListRows(th *material.Theme, gtx layout.Context, st *fileViewerState, rows [][]string) layout.Dimensions {
	if len(rows) == 0 {
		theme := ui.fileViewerTheme()
		return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, "No past commands")
			lbl.Font.Typeface = ui.fileViewerHistoryTypeface()
			lbl.TextSize = ui.fileViewerHistoryTextSize()
			lbl.Color = theme.HistoryMuted
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		})
	}
	children := make([]layout.FlexChild, 0, len(rows)*2)
	for rowIdx, row := range rows {
		row := row
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			rowChildren := make([]layout.FlexChild, 0, len(row)*2)
			for i, cmd := range row {
				cmd := cmd
				click := st.historyClickable("viewer-history:" + cmd)
				rowChildren = append(rowChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutFileViewerHistoryChip(th, gtx, click, cmd)
				}))
				if i < len(row)-1 {
					rowChildren = append(rowChildren, layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout))
				}
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, rowChildren...)
		}))
		if rowIdx < len(rows)-1 {
			children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout))
		}
	}
	return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (ui *UI) fileViewerHistoryRows(th *material.Theme, gtx layout.Context, history []string) ([][]string, int) {
	if len(history) == 0 {
		lbl := material.Body2(th, "No past commands")
		lbl.Font.Typeface = ui.fileViewerHistoryTypeface()
		lbl.TextSize = ui.fileViewerHistoryTextSize()
		return nil, measureLabelUnconstrained(gtx, lbl).Size.X + gtx.Dp(unit.Dp(18))
	}
	maxW := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(10))
	minW := gtx.Dp(unit.Dp(180))
	if maxW < 1 {
		maxW = 1
	}
	if maxW < minW {
		maxW = minW
	}
	gap := gtx.Dp(unit.Dp(4))
	rows := make([][]string, 1, 2)
	rows[0] = []string{}
	rowWidths := []int{0}
	usedW := 0
	const maxRows = 2
	for _, cmd := range history {
		chipW := ui.fileViewerHistoryChipWidth(th, gtx, cmd)
		rowIdx := len(rows) - 1
		nextW := rowWidths[rowIdx]
		if len(rows[rowIdx]) > 0 {
			nextW += gap
		}
		nextW += chipW
		if nextW > maxW && len(rows[rowIdx]) > 0 {
			if len(rows) >= maxRows {
				break
			}
			rows = append(rows, []string{})
			rowWidths = append(rowWidths, 0)
			rowIdx++
			nextW = chipW
		}
		rows[rowIdx] = append(rows[rowIdx], cmd)
		rowWidths[rowIdx] = nextW
		if nextW > usedW {
			usedW = nextW
		}
	}
	if usedW < minW {
		usedW = minW
	}
	return rows, usedW + gtx.Dp(unit.Dp(10))
}

func (ui *UI) fileViewerHistoryChipWidth(th *material.Theme, gtx layout.Context, label string) int {
	lbl := material.Body2(th, label)
	lbl.Font.Typeface = ui.fileViewerHistoryTypeface()
	lbl.TextSize = ui.fileViewerHistoryTextSize()
	lbl.MaxLines = 1
	w := measureLabelUnconstrained(gtx, lbl).Size.X + gtx.Dp(unit.Dp(10))
	minW := gtx.Dp(unit.Dp(58))
	if w < minW {
		w = minW
	}
	maxW := gtx.Dp(unit.Dp(260))
	if w > maxW {
		w = maxW
	}
	return w
}

func (ui *UI) layoutFileViewerHistoryChip(th *material.Theme, gtx layout.Context, click *widget.Clickable, label string) layout.Dimensions {
	if click == nil {
		return layout.Dimensions{}
	}
	w := ui.fileViewerHistoryChipWidth(th, gtx, label)
	theme := ui.fileViewerTheme()
	return fixedWidth(gtx, w, func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			bg := theme.HistoryChipBg
			bd := theme.HistoryChipBorder
			if click.Hovered() {
				bg = theme.HistoryChipBgHover
				bd = theme.HistoryChipBorderH
			}
			return fillRoundedBox(gtx, 0, bg, bd, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, label)
					lbl.Font.Typeface = ui.fileViewerHistoryTypeface()
					lbl.TextSize = ui.fileViewerHistoryTextSize()
					lbl.Color = theme.HistoryChipText
					lbl.MaxLines = 1
					lbl.Truncator = "..."
					return lbl.Layout(gtx)
				})
			})
		})
	})
}

func (ui *UI) fileViewerHistoryTypeface() font.Typeface {
	return ui.interfaceTypeface()
}

func (ui *UI) fileViewerHistoryTextSize() unit.Sp {
	return ui.scaleInterfaceFontSize(9)
}
