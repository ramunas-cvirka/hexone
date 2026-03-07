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
		ui.startViewerCommandEdit()
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.modeFileClick.Clicked(gtx) {
		ui.setFileViewerMode("file", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.modeHexClick.Clicked(gtx) {
		ui.setFileViewerMode("hex", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.modeCmdClick.Clicked(gtx) {
		ui.setFileViewerMode("command", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.autoRefreshClick.Clicked(gtx) {
		ui.toggleFileViewerAutoRefresh(gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	if st.historyClick.Clicked(gtx) {
		if st.commandEditOn {
			ui.cancelViewerCommandEdit()
		}
		st.historyOpen = !st.historyOpen
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
					lbl.Font.Typeface = ui.mainTypeface()
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
									wait.Font.Typeface = ui.mainTypeface()
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
				st.historyOpen = false
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
				st.historyOpen = false
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
								lbl.Font.Typeface = ui.mainTypeface()
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
							return ui.layoutFileViewerInfoStrip(th, gtx, st, stripH)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutFileViewerModeTabs(th, gtx, st, stripH)
						}),
					}
					if st.mode == "command" {
						children = append(children,
							layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutFileViewerInlineCommand(th, gtx, st, stripH)
							}),
						)
					}
					children = append(children,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if st.mode != "command" {
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

type viewerHeaderChip struct {
	label string
	bg    color.NRGBA
	fg    color.NRGBA
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

func (ui *UI) fileViewerHeaderChips(st *fileViewerState) []viewerHeaderChip {
	if st == nil {
		return nil
	}
	chips := make([]viewerHeaderChip, 0, 5)
	if !st.updatedAt.IsZero() {
		chips = append(chips, viewerHeaderChip{
			label: "pulled " + st.updatedAt.Format("15:04:05"),
			bg:    color.NRGBA{R: 28, G: 34, B: 48, A: 255},
			fg:    color.NRGBA{R: 194, G: 208, B: 232, A: 255},
		})
	}
	if st.mode == "command" && st.commandInfinite {
		chips = append(chips, viewerHeaderChip{
			label: "streaming",
			bg:    color.NRGBA{R: 36, G: 54, B: 86, A: 255},
			fg:    color.NRGBA{R: 220, G: 232, B: 255, A: 255},
		})
	}
	switch st.status {
	case "", "ready":
	case "loading...":
		chips = append(chips, viewerHeaderChip{
			label: "loading",
			bg:    color.NRGBA{R: 70, G: 56, B: 28, A: 255},
			fg:    color.NRGBA{R: 250, G: 226, B: 172, A: 255},
		})
	case "update pending":
		chips = append(chips, viewerHeaderChip{
			label: "pending",
			bg:    color.NRGBA{R: 66, G: 56, B: 22, A: 255},
			fg:    color.NRGBA{R: 246, G: 226, B: 152, A: 255},
		})
	case "nothing to copy":
		chips = append(chips, viewerHeaderChip{
			label: "nothing to copy",
			bg:    color.NRGBA{R: 56, G: 36, B: 36, A: 255},
			fg:    color.NRGBA{R: 244, G: 214, B: 214, A: 255},
		})
	case "streaming":
	case "truncated":
		chips = append(chips, viewerHeaderChip{
			label: "truncated",
			bg:    color.NRGBA{R: 80, G: 54, B: 28, A: 255},
			fg:    color.NRGBA{R: 252, G: 224, B: 186, A: 255},
		})
	case "streaming, truncated":
		chips = append(chips, viewerHeaderChip{
			label: "truncated",
			bg:    color.NRGBA{R: 80, G: 54, B: 28, A: 255},
			fg:    color.NRGBA{R: 252, G: 224, B: 186, A: 255},
		})
	default:
		chips = append(chips, viewerHeaderChip{
			label: st.status,
			bg:    color.NRGBA{R: 44, G: 50, B: 64, A: 255},
			fg:    color.NRGBA{R: 220, G: 228, B: 244, A: 255},
		})
	}
	return chips
}

func (ui *UI) layoutFileViewerInfoStrip(th *material.Theme, gtx layout.Context, st *fileViewerState, stripH int) layout.Dimensions {
	chips := ui.fileViewerHeaderChips(st)
	title := ui.fileViewerHeaderTitle(st)
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 18, G: 22, B: 30, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 14},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(1), Right: unit.Dp(1), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				titleLbl := material.Body2(th, title)
				titleLbl.Font.Typeface = ui.mainTypeface()
				titleLbl.Font.Weight = font.Medium
				titleLbl.TextSize = ui.viewerTextSize()
				titleW := measureLabelUnconstrained(gtx, titleLbl).Size.X + gtx.Dp(unit.Dp(18))
				if titleW < gtx.Dp(unit.Dp(78)) {
					titleW = gtx.Dp(unit.Dp(78))
				}
				titleMax := gtx.Constraints.Max.X * 2 / 3
				if titleMax < gtx.Dp(unit.Dp(140)) {
					titleMax = gtx.Dp(unit.Dp(140))
				}
				if titleW > titleMax {
					titleW = titleMax
				}

				children := make([]layout.FlexChild, 0, len(chips)*2+1)
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, titleW, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutFileViewerHeaderSegment(th, gtx, title, color.NRGBA{R: 30, G: 40, B: 58, A: 255}, color.NRGBA{R: 224, G: 234, B: 252, A: 255}, true, true, len(chips) == 0, true, stripH)
					})
				}))
				for i, chip := range chips {
					chip := chip
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return toolbarSeparator(gtx, stripH)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutFileViewerHeaderSegment(th, gtx, chip.label, chip.bg, chip.fg, false, false, i == len(chips)-1, false, stripH)
						}),
					)
				}
				return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
				})
			})
		},
	)
}

func (ui *UI) layoutFileViewerHeaderSegment(th *material.Theme, gtx layout.Context, label string, bg, fg color.NRGBA, bold, roundLeft, roundRight, truncate bool, stripH int) layout.Dimensions {
	return fillSegmentBg(gtx, bg, gtx.Dp(unit.Dp(filePaneControlCornerDp-1)), roundLeft, roundRight, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(9), Right: unit.Dp(9)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = ui.mainTypeface()
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
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 18, G: 22, B: 30, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 26},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(1), Right: unit.Dp(1), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutFileViewerModeTabSegment(th, gtx, &st.modeFileClick, "File", st.mode == "file", true, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return toolbarSeparator(gtx, stripH)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutFileViewerModeTabSegment(th, gtx, &st.modeHexClick, "Hex", st.mode == "hex", false, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return toolbarSeparator(gtx, stripH)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutFileViewerModeTabSegment(th, gtx, &st.modeCmdClick, "Cmd", st.mode == "command", false, false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return toolbarSeparator(gtx, stripH)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutFileViewerModeTabSegment(th, gtx, &st.historyClick, "..", st.historyOpen, false, true)
						}),
					)
				})
			})
		},
	)
}

func (ui *UI) layoutFileViewerModeTabSegment(th *material.Theme, gtx layout.Context, c *widget.Clickable, label string, active, roundLeft, roundRight bool) layout.Dimensions {
	if c == nil {
		return layout.Dimensions{}
	}
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{A: 0}
		fg := color.NRGBA{R: 202, G: 216, B: 242, A: 255}
		if active {
			bg = color.NRGBA{R: 68, G: 92, B: 180, A: 255}
			fg = color.NRGBA{R: 240, G: 246, B: 255, A: 255}
		} else if c.Hovered() {
			bg = color.NRGBA{R: 40, G: 54, B: 82, A: 255}
			fg = color.NRGBA{R: 232, G: 241, B: 255, A: 255}
		}
		return fillSegmentBg(gtx, bg, gtx.Dp(unit.Dp(filePaneControlCornerDp-1)), roundLeft, roundRight, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, label)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.Font.Weight = font.Medium
					lbl.TextSize = ui.viewerTextSize()
					lbl.Color = fg
					lbl.MaxLines = 1
					return lbl.Layout(gtx)
				})
			})
		})
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
	bg := color.NRGBA{R: 88, G: 70, B: 30, A: 255}
	if st.commandClick.Hovered() || st.commandEditOn {
		bg = color.NRGBA{R: 104, G: 82, B: 36, A: 255}
	}
	commandText := st.command
	if st.commandEditOn {
		commandText = st.commandEditor.Text()
	}
	if strings.TrimSpace(commandText) == "" {
		commandText = "cat {fullpath}"
	}
	measure := material.Body2(th, commandText)
	measure.Font.Typeface = ui.mainTypeface()
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
			return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
					if st.commandEditOn {
						ed := material.Editor(th, &st.commandEditor, "cat {fullpath}")
						ed.Font.Typeface = ui.mainTypeface()
						ed.TextSize = ui.viewerTextSize()
						ed.Color = fg
						ed.HintColor = color.NRGBA{R: 176, G: 160, B: 116, A: 255}
						return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return layout.W.Layout(gtx, ed.Layout)
							})
						})
					}
					label := commandText
					return st.commandClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									lbl := material.Body2(th, label)
									lbl.Font.Typeface = ui.mainTypeface()
									lbl.TextSize = ui.viewerTextSize()
									lbl.Font.Weight = font.Medium
									lbl.Color = fg
									lbl.MaxLines = 1
									lbl.Truncator = "..."
									return lbl.Layout(gtx)
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

func (ui *UI) layoutFileViewerHistoryList(th *material.Theme, gtx layout.Context, st *fileViewerState, history []string) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	const maxHistoryRows = 8
	if len(history) > maxHistoryRows {
		history = history[:maxHistoryRows]
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

	return fixedWidth(gtx, ui.fileViewerHistoryListWidth(th, gtx, history), func(gtx layout.Context) layout.Dimensions {
		return fillRoundedBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneControlCornerDp)),
			color.NRGBA{R: 18, G: 22, B: 30, A: 255},
			color.NRGBA{R: 255, G: 255, B: 255, A: 22},
			func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFileViewerHistoryListRows(th, gtx, st, history)
			},
		)
	})
}

func (ui *UI) layoutFileViewerHistoryListRows(th *material.Theme, gtx layout.Context, st *fileViewerState, history []string) layout.Dimensions {
	if len(history) == 0 {
		return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, "No past commands")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleThemeFontSize(th, 9)
			lbl.Color = color.NRGBA{R: 162, G: 172, B: 190, A: 255}
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		})
	}
	children := make([]layout.FlexChild, 0, len(history))
	for _, cmd := range history {
		cmd := cmd
		click := st.historyClickable("viewer-history:" + cmd)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if click == nil {
				return layout.Dimensions{}
			}
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				bg := color.NRGBA{A: 0}
				if click.Hovered() {
					bg = color.NRGBA{R: 255, G: 255, B: 255, A: 14}
				}
				return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, cmd)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleThemeFontSize(th, 9)
						lbl.Color = color.NRGBA{R: 236, G: 224, B: 184, A: 255}
						lbl.MaxLines = 1
						lbl.Truncator = "..."
						return lbl.Layout(gtx)
					})
				})
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (ui *UI) fileViewerHistoryListWidth(th *material.Theme, gtx layout.Context, history []string) int {
	minW := gtx.Dp(unit.Dp(160))
	maxW := minW
	samples := history
	if len(samples) == 0 {
		samples = []string{"No past commands"}
	}
	for _, cmd := range samples {
		lbl := material.Body2(th, cmd)
		lbl.Font.Typeface = ui.mainTypeface()
		lbl.TextSize = scaleThemeFontSize(th, 9)
		w := measureLabelUnconstrained(gtx, lbl).Size.X + gtx.Dp(unit.Dp(16))
		if w > maxW {
			maxW = w
		}
	}
	capW := gtx.Dp(unit.Dp(360))
	if avail := gtx.Constraints.Max.X; avail > 0 && capW > avail {
		capW = avail
	}
	if capW < minW {
		minW = capW
	}
	if maxW > capW {
		maxW = capW
	}
	if maxW < minW {
		maxW = minW
	}
	return maxW
}
