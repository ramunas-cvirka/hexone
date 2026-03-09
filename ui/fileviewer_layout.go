package ui

import (
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
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type viewerHeaderDetailPart struct {
	Text  string
	Color color.NRGBA
}

func (ui *UI) layoutFileViewer(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.fileViewer
	if st == nil {
		return layout.Dimensions{}
	}
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
	if st.modeCmdClick.Clicked(gtx) {
		st.tabAnim.setPulse("command", gtx.Now)
		ui.setFileViewerMode("command", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if ui.viewerShowsAutoRefreshButton(st) && st.autoRefreshClick.Clicked(gtx) {
		ui.toggleFileViewerAutoRefresh(gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.historyClick.Clicked(gtx) {
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
		gtx.Execute(op.InvalidateCmd{})
	}
	for st.backdropClick.Clicked(gtx) {
	}
	if st.closeClick.Clicked(gtx) {
		ui.closeFileViewer()
		return layout.Dimensions{}
	}
	if st.loading {
		// Keep frames ticking while background load is running, otherwise
		// results can remain pending until an external event (e.g. resize).
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(33 * time.Millisecond)})
	}

	ui.scheduleFileViewerWatch(gtx)

	return st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{R: 14, G: 18, B: 24, A: 252}, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

		dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFileViewerHeader(th, gtx, st)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if st.err == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, st.err)
					lbl.Font.Typeface = ui.viewerTypeface()
					lbl.TextSize = scaleThemeFontSize(th, 10)
					lbl.Color = color.NRGBA{R: 255, G: 170, B: 170, A: 255}
					lbl.MaxLines = 2
					lbl.Truncator = "..."
					return lbl.Layout(gtx)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(2), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
					return fillRoundedBox(
						gtx,
						gtx.Dp(unit.Dp(filePaneControlCornerDp)),
						color.NRGBA{R: 18, G: 24, B: 34, A: 255},
						color.NRGBA{R: 255, G: 255, B: 255, A: 8},
						func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
							return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
								if st.loading && st.content == "" {
									if st.mode == "hex" && st.hex != nil && len(st.hex.buffer) > 0 {
										return ui.layoutHexOutputView(th, gtx, st)
									}
									wait := material.Body2(th, "Loading...")
									wait.Font.Typeface = ui.viewerTypeface()
									wait.TextSize = ui.viewerTextSize()
									wait.Color = hintColor
									return wait.Layout(gtx)
								}
								gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
								if st.mode == "hex" {
									return ui.layoutHexOutputView(th, gtx, st)
								}
								return ui.layoutStreamOutputView(th, gtx, st)
							})
						},
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
		if st.menuOpen {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
		return dims
	})
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
			if st.commandEditOn {
				ui.cancelViewerCommandEdit()
				gtx.Execute(op.InvalidateCmd{})
			}
			if pe.Buttons.Contain(pointer.ButtonSecondary) {
				st.setHistoryOpen(false, gtx.Now)
				st.menuOpen = true
				st.menuPos = pos
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) && st.scrollbarVisible && viewerPointInRect(pos, st.scrollbarTrack) {
				st.scrollbarDragging = true
				st.scrollbarDragID = pe.PointerID
				gtx.Execute(pointer.GrabCmd{Tag: &st.contentPointerTag, ID: pe.PointerID})
				st.markUserBrowsing(gtx.Now)
				viewerScrollFromScrollbarPos(st, pos.Y)
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) || pe.Buttons.Contain(pointer.ButtonSecondary) || pe.Buttons.Contain(pointer.ButtonTertiary) {
				st.markUserBrowsing(gtx.Now)
				st.setHistoryOpen(false, gtx.Now)
				if st.menuOpen {
					st.menuOpen = false
				}
				st.updateScrollbarHover(pos)
			}
		case pointer.Drag:
			if st.scrollbarDragging && pe.PointerID == st.scrollbarDragID {
				st.markUserBrowsing(gtx.Now)
				viewerScrollFromScrollbarPos(st, pos.Y)
			}
			st.updateScrollbarHover(pos)
		case pointer.Release, pointer.Cancel:
			if st.scrollbarDragging && pe.PointerID == st.scrollbarDragID {
				st.scrollbarDragging = false
			}
			st.updateScrollbarHover(pos)
		case pointer.Move, pointer.Enter:
			st.updateScrollbarHover(pos)
		case pointer.Leave:
			st.scrollbarHover = false
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

	thumbH := int(float32(size.Y) * float32(visibleLines) / float32(totalLines))
	if thumbH < 18 {
		thumbH = 18
	}
	if thumbH > size.Y {
		thumbH = size.Y
	}
	thumbY := 0
	if maxTop > 0 && size.Y > thumbH {
		thumbY = int(float32(top) / float32(maxTop) * float32(size.Y-thumbH))
	}

	const trackW = 6
	trackX := size.X - trackW - 1
	if trackX < 0 {
		trackX = 0
	}
	track := image.Rect(trackX, 0, trackX+trackW, size.Y)
	thumb := image.Rect(trackX+1, thumbY, trackX+trackW-1, thumbY+thumbH)
	st.scrollbarVisible = true
	st.scrollbarTrack = track
	st.scrollbarThumb = thumb
	st.scrollbarLines = totalLines
	st.scrollbarVisibleN = visibleLines

	trackColor := color.NRGBA{R: 255, G: 255, B: 255, A: 30}
	thumbColor := color.NRGBA{R: 173, G: 197, B: 238, A: 168}
	if st.scrollbarHover {
		trackColor = color.NRGBA{R: 255, G: 255, B: 255, A: 42}
		thumbColor = color.NRGBA{R: 194, G: 214, B: 248, A: 214}
	}
	if st.scrollbarDragging {
		thumbColor = color.NRGBA{R: 204, G: 224, B: 255, A: 245}
	}
	paint.FillShape(gtx.Ops, trackColor, clip.Rect(track).Op())
	paint.FillShape(gtx.Ops, thumbColor, clip.Rect(thumb).Op())
}

func (ui *UI) applyFileViewerScrollCursor(gtx layout.Context, st *fileViewerState) {
	if st == nil || !st.scrollbarVisible {
		return
	}
	if st.scrollbarDragging {
		defer clip.Rect(st.scrollbarTrack).Push(gtx.Ops).Pop()
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
		st.menuOpen = false
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
			st.menuOpen = false
		}
	}
	if !st.menuOpen {
		return layout.Dimensions{}
	}

	m := op.Record(gtx.Ops)
	menuDims := ui.layoutFileViewerContextMenuCard(th, gtx, st)
	call := m.Stop()

	anchor := clampFilePaneMenuPoint(st.menuPos, menuDims.Size, gtx.Constraints.Max)
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

func (ui *UI) layoutFileViewerContextMenuCard(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	width := gtx.Dp(unit.Dp(132))
	if width > gtx.Constraints.Max.X {
		width = gtx.Constraints.Max.X
	}
	if width < 1 {
		width = 1
	}
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			color.NRGBA{R: 20, G: 24, B: 34, A: 250},
			color.NRGBA{R: 255, G: 255, B: 255, A: 22},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return st.copyToggle.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						bg := color.NRGBA{R: 96, G: 130, B: 186, A: 24}
						if st.copyToggle.Hovered() {
							bg = color.NRGBA{R: 96, G: 130, B: 186, A: 54}
						}
						return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, "Copy")
								lbl.Font.Typeface = ui.viewerTypeface()
								lbl.Font.Weight = font.Medium
								lbl.TextSize = scaleThemeFontSize(th, 10)
								lbl.Color = color.NRGBA{R: 224, G: 234, B: 252, A: 255}
								return lbl.Layout(gtx)
							})
						})
					})
				})
			},
		)
	})
}

func (ui *UI) layoutFileViewerHeader(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	history := ui.viewerHistoryCommands(st.command)
	stripH := ui.viewerHeaderStripHeight(gtx)

	return fillRoundedBox(
		gtx,
		0,
		color.NRGBA{R: 20, G: 26, B: 38, A: 255},
		color.NRGBA{},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				row := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					children := []layout.FlexChild{
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutFileViewerModeTabs(th, gtx, st, stripH)
						}),
					}
					if st.mode == "command" {
						children = append(children,
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutFileViewerInlineCommand(th, gtx, st, stripH)
							}),
						)
					}
					children = append(children,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutFileViewerInfoStrip(th, gtx, st, stripH)
							})
						}),
					)
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
				})

				if !st.historyOpen {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, row)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					row,
					layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.layoutFileViewerHistoryList(th, gtx, st, history)
					}),
				)
			})
		},
	)
}

func (ui *UI) viewerHeaderStripHeight(gtx layout.Context) int {
	h := gtx.Sp(ui.viewerTextSize()) + gtx.Dp(unit.Dp(8))
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

func (ui *UI) fileViewerHeaderStatusText(st *fileViewerState) (string, color.NRGBA) {
	if st == nil {
		return "", hintColor
	}
	statusText := ""
	statusColor := hintColor
	switch st.status {
	case "", "ready":
	case "loading...":
		statusText = "loading"
		statusColor = color.NRGBA{R: 250, G: 226, B: 172, A: 255}
	case "update pending":
		statusText = "pending"
		statusColor = color.NRGBA{R: 246, G: 226, B: 152, A: 255}
	case "nothing to copy":
		statusText = "nothing to copy"
		statusColor = color.NRGBA{R: 244, G: 214, B: 214, A: 255}
	case "truncated":
		statusText = "truncated"
		statusColor = color.NRGBA{R: 252, G: 224, B: 186, A: 255}
	case "streaming":
		statusText = "streaming"
		statusColor = color.NRGBA{R: 220, G: 232, B: 255, A: 255}
	case "streaming, truncated":
		statusText = "streaming, truncated"
		statusColor = color.NRGBA{R: 252, G: 224, B: 186, A: 255}
	default:
		if !isViewerHeaderSizeStatus(st.status) {
			statusText = st.status
			statusColor = color.NRGBA{R: 220, G: 228, B: 244, A: 255}
		}
	}
	if st.mode == "command" && st.commandInfinite {
		if statusText == "" {
			statusText = "streaming"
			statusColor = color.NRGBA{R: 220, G: 232, B: 255, A: 255}
		}
	}
	return statusText, statusColor
}

func (ui *UI) fileViewerHeaderDetails(st *fileViewerState) []viewerHeaderDetailPart {
	if st == nil {
		return nil
	}
	statusText, statusColor := ui.fileViewerHeaderStatusText(st)
	parts := make([]viewerHeaderDetailPart, 0, 2)
	if statusText != "" {
		parts = append(parts, viewerHeaderDetailPart{
			Text:  statusText,
			Color: statusColor,
		})
	}
	if !st.updatedAt.IsZero() {
		parts = append(parts, viewerHeaderDetailPart{
			Text:  "updated at " + st.updatedAt.Format("15:04:05"),
			Color: color.NRGBA{R: 194, G: 208, B: 232, A: 255},
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

func (ui *UI) layoutFileViewerInfoStrip(th *material.Theme, gtx layout.Context, st *fileViewerState, stripH int) layout.Dimensions {
	title := ui.fileViewerHeaderTitle(st)
	details := ui.fileViewerHeaderDetails(st)
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
								lbl.Color = color.NRGBA{R: 224, G: 234, B: 252, A: 255}
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
			paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 28}, clip.Rect(image.Rect(0, 0, w, h)).Op())
			return layout.Dimensions{Size: image.Pt(w, h)}
		})
	})
}

func (ui *UI) viewerShowsAutoRefreshButton(st *fileViewerState) bool {
	return st != nil && st.mode == "command" && !st.commandInfinite
}

func (ui *UI) layoutFileViewerInfoButtons(th *material.Theme, gtx layout.Context, st *fileViewerState, stripH int) layout.Dimensions {
	return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if !ui.viewerShowsAutoRefreshButton(st) {
				return layoutTinyIconModeButton(th, gtx, &st.closeClick, uitheme.CloseIcon(), false)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutTinyIconModeButton(th, gtx, &st.autoRefreshClick, uitheme.RefreshIcon(), st.autoRefresh)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutTinyIconModeButton(th, gtx, &st.closeClick, uitheme.CloseIcon(), false)
				}),
			)
		})
	})
}

func (ui *UI) layoutFileViewerHeaderSegment(th *material.Theme, gtx layout.Context, label string, bg, fg color.NRGBA, bold, roundLeft, roundRight, truncate bool, stripH int) layout.Dimensions {
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

func (ui *UI) layoutFileViewerModeTabs(th *material.Theme, gtx layout.Context, st *fileViewerState, stripH int) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	hoverKey := ""
	if st.modeFileClick.Hovered() {
		hoverKey = "file"
	}
	if st.modeHexClick.Hovered() {
		hoverKey = "hex"
	}
	if st.modeCmdClick.Hovered() {
		hoverKey = "command"
	}
	if st.historyClick.Hovered() {
		hoverKey = "history"
	}
	st.tabAnim.setHover(hoverKey, gtx.Now)
	fillFile, animFile := st.tabFill(gtx.Now, "file")
	fillHex, animHex := st.tabFill(gtx.Now, "hex")
	fillCommand, animCommand := st.tabFill(gtx.Now, "command")
	fillHistory, animHistory := st.tabFill(gtx.Now, "history")
	hoverFile, hoverAnimFile := st.tabAnim.hoverFill(gtx.Now, "file")
	hoverHex, hoverAnimHex := st.tabAnim.hoverFill(gtx.Now, "hex")
	hoverCommand, hoverAnimCommand := st.tabAnim.hoverFill(gtx.Now, "command")
	hoverHistory, hoverAnimHistory := st.tabAnim.hoverFill(gtx.Now, "history")
	pulseFile, pulseAnimFile := st.tabAnim.pulseFill(gtx.Now, "file")
	pulseHex, pulseAnimHex := st.tabAnim.pulseFill(gtx.Now, "hex")
	pulseCommand, pulseAnimCommand := st.tabAnim.pulseFill(gtx.Now, "command")
	pulseHistory, pulseAnimHistory := st.tabAnim.pulseFill(gtx.Now, "history")
	pos, animPos := st.tabPosition(gtx.Now)
	if animFile || animHex || animCommand || animHistory ||
		hoverAnimFile || hoverAnimHex || hoverAnimCommand || hoverAnimHistory ||
		pulseAnimFile || pulseAnimHex || pulseAnimCommand || pulseAnimHistory ||
		animPos {
		gtx.Execute(op.InvalidateCmd{})
	}
	return ui.layoutSlidingTabStrip(th, gtx, stripH, pos, ui.viewerTextSize(), []slidingTabSpec{
		{
			Label:      "File",
			Click:      &st.modeFileClick,
			ActiveFill: fillFile,
			HoverFill:  hoverFile,
			PulseFill:  pulseFile,
		},
		{
			Label:      "Hex",
			Click:      &st.modeHexClick,
			ActiveFill: fillHex,
			HoverFill:  hoverHex,
			PulseFill:  pulseHex,
		},
		{
			Label:      "Cmd",
			Click:      &st.modeCmdClick,
			ActiveFill: fillCommand,
			HoverFill:  hoverCommand,
			PulseFill:  pulseCommand,
		},
		{
			Label:      "..",
			Click:      &st.historyClick,
			ActiveFill: fillHistory,
			HoverFill:  hoverHistory,
			PulseFill:  pulseHistory,
		},
	})
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
	fg := color.NRGBA{R: 245, G: 231, B: 180, A: 255}
	bg := color.NRGBA{R: 72, G: 58, B: 28, A: 255}
	border := color.NRGBA{R: 189, G: 158, B: 84, A: 132}
	if st.commandClick.Hovered() {
		bg = color.NRGBA{R: 86, G: 69, B: 33, A: 255}
		border = color.NRGBA{R: 210, G: 180, B: 100, A: 168}
	}
	commandText := st.command
	if st.commandEditOn {
		commandText = st.commandEditor.Text()
	}
	if strings.TrimSpace(commandText) == "" {
		commandText = "cat {fullpath}"
	}
	measure := material.Body2(th, commandText)
	measure.Font.Typeface = ui.viewerTypeface()
	measure.Font.Weight = font.Medium
	measure.TextSize = ui.viewerTextSize()
	desiredW := measureLabelUnconstrained(gtx, measure).Size.X + gtx.Dp(unit.Dp(18))
	if desiredW < gtx.Dp(unit.Dp(96)) {
		desiredW = gtx.Dp(unit.Dp(96))
	}
	if maxW := gtx.Constraints.Max.X; maxW > 0 && desiredW > maxW {
		desiredW = maxW
	}
	host := func(gtx layout.Context) layout.Dimensions {
		return fixedWidth(gtx, desiredW, func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
				if st.commandEditOn {
					ed := material.Editor(th, &st.commandEditor, "cat {fullpath}")
					ed.Font.Typeface = ui.viewerTypeface()
					ed.TextSize = ui.viewerTextSize()
					ed.Color = fg
					ed.HintColor = color.NRGBA{R: 176, G: 160, B: 116, A: 255}
					focused := st.commandFocus || gtx.Focused(&st.commandEditor)
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layoutNeutralEditorBox(gtx, focused, true, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(2), Right: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return ed.Layout(gtx)
							})
						})
					})
				}
				label := commandText
				return st.commandClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return fillRoundedBox(
						gtx,
						gtx.Dp(unit.Dp(filePaneControlCornerDp)),
						bg,
						border,
						func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								lbl := material.Body2(th, label)
								lbl.Font.Typeface = ui.viewerTypeface()
								lbl.TextSize = ui.viewerTextSize()
								lbl.Font.Weight = font.Medium
								lbl.Color = fg
								lbl.MaxLines = 1
								lbl.Truncator = "..."
								return layoutVCenteredLabel(gtx, lbl)
							})
						},
					)
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
	rows, listW := ui.fileViewerHistoryRows(th, gtx, history)

	return fixedWidth(gtx, listW, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneControlCornerDp)),
			color.NRGBA{R: 18, G: 22, B: 30, A: 255},
			color.NRGBA{R: 255, G: 255, B: 255, A: 22},
			func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFileViewerHistoryListRows(th, gtx, st, rows)
			},
		)
	})
}

func (ui *UI) layoutFileViewerHistoryListRows(th *material.Theme, gtx layout.Context, st *fileViewerState, rows [][]string) layout.Dimensions {
	if len(rows) == 0 {
		return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, "No past commands")
			lbl.Font.Typeface = ui.viewerTypeface()
			lbl.TextSize = scaleThemeFontSize(th, 8)
			lbl.Color = color.NRGBA{R: 162, G: 172, B: 190, A: 255}
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
		lbl.Font.Typeface = ui.viewerTypeface()
		lbl.TextSize = scaleThemeFontSize(th, 8)
		return nil, measureLabelUnconstrained(gtx, lbl).Size.X + gtx.Dp(unit.Dp(18))
	}
	maxW := gtx.Dp(unit.Dp(640))
	if avail := gtx.Constraints.Max.X; avail > 0 && maxW > avail {
		maxW = avail
	}
	minW := gtx.Dp(unit.Dp(180))
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
	lbl.Font.Typeface = ui.viewerTypeface()
	lbl.TextSize = scaleThemeFontSize(th, 8)
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
	return fixedWidth(gtx, w, func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			bg := color.NRGBA{R: 74, G: 58, B: 24, A: 255}
			bd := color.NRGBA{R: 188, G: 160, B: 92, A: 46}
			if click.Hovered() {
				bg = color.NRGBA{R: 92, G: 72, B: 28, A: 255}
				bd = color.NRGBA{R: 220, G: 190, B: 110, A: 76}
			}
			return fillRoundedBox(gtx, gtx.Dp(unit.Dp(5)), bg, bd, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, label)
					lbl.Font.Typeface = ui.viewerTypeface()
					lbl.TextSize = scaleThemeFontSize(th, 8)
					lbl.Color = color.NRGBA{R: 245, G: 231, B: 180, A: 255}
					lbl.MaxLines = 1
					lbl.Truncator = "..."
					return lbl.Layout(gtx)
				})
			})
		})
	})
}
