// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/ui/platform"
	"image"
	"image/color"
	"io"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var readEditorContextClipboardText = platform.ReadClipboardTextNow

// Use a non-zero-sized tag type here. Zero-sized pointer allocations like
// &struct{}{} are not guaranteed to have unique addresses, which can collapse
// multiple edit boxes onto the same Gio event tag and route context-menu input
// to the wrong editor.
type editorMenuEventTag struct {
	_ byte
}

func layoutNeutralEditorBox(gtx layout.Context, focused, enabled bool, inner layout.Widget) layout.Dimensions {
	bg := color.NRGBA{R: 34, G: 34, B: 34, A: 255}
	border := color.NRGBA{R: 255, G: 255, B: 255, A: 18}
	underline := color.NRGBA{R: 122, G: 114, B: 98, A: 185}
	underlineH := 1
	if focused && enabled {
		bg = color.NRGBA{R: 48, G: 48, B: 48, A: 255}
		border = color.NRGBA{R: 255, G: 255, B: 255, A: 46}
		underline = color.NRGBA{R: 160, G: 148, B: 122, A: 230}
		underlineH = 2
	}
	if !enabled {
		bg = color.NRGBA{R: 24, G: 24, B: 24, A: 255}
		border = color.NRGBA{R: 255, G: 255, B: 255, A: 10}
		underline = color.NRGBA{R: 104, G: 98, B: 88, A: 132}
		underlineH = 1
	}

	m := op.Record(gtx.Ops)
	dims := layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, inner)
	call := m.Stop()

	rect := image.Rect(0, 0, dims.Size.X, dims.Size.Y)
	rr := clip.UniformRRect(rect, 0)
	paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())
	if dims.Size.Y >= underlineH {
		paint.FillShape(gtx.Ops, underline, clip.Rect(image.Rect(0, dims.Size.Y-underlineH, dims.Size.X, dims.Size.Y)).Op())
	}

	call.Add(gtx.Ops)
	return dims
}

func layoutCompactNeutralEditorBox(gtx layout.Context, focused, enabled bool, inner layout.Widget) layout.Dimensions {
	bg := color.NRGBA{R: 34, G: 34, B: 34, A: 255}
	border := color.NRGBA{R: 255, G: 255, B: 255, A: 18}
	underline := color.NRGBA{R: 122, G: 114, B: 98, A: 185}
	underlineH := 1
	if focused && enabled {
		bg = color.NRGBA{R: 48, G: 48, B: 48, A: 255}
		border = color.NRGBA{R: 255, G: 255, B: 255, A: 46}
		underline = color.NRGBA{R: 160, G: 148, B: 122, A: 230}
		underlineH = 2
	}
	if !enabled {
		bg = color.NRGBA{R: 24, G: 24, B: 24, A: 255}
		border = color.NRGBA{R: 255, G: 255, B: 255, A: 10}
		underline = color.NRGBA{R: 104, G: 98, B: 88, A: 132}
		underlineH = 1
	}

	m := op.Record(gtx.Ops)
	dims := layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, inner)
	call := m.Stop()

	rect := image.Rect(0, 0, dims.Size.X, dims.Size.Y)
	rr := clip.UniformRRect(rect, 0)
	paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())
	if dims.Size.Y >= underlineH {
		paint.FillShape(gtx.Ops, underline, clip.Rect(image.Rect(0, dims.Size.Y-underlineH, dims.Size.X, dims.Size.Y)).Op())
	}

	call.Add(gtx.Ops)
	return dims
}

func (ui *UI) editorMenuTag(id string) event.Tag {
	if ui == nil || id == "" {
		return nil
	}
	if ui.editorMenuTags == nil {
		ui.editorMenuTags = make(map[string]*editorMenuEventTag)
	}
	if tag, ok := ui.editorMenuTags[id]; ok {
		return tag
	}
	tag := &editorMenuEventTag{}
	ui.editorMenuTags[id] = tag
	return tag
}

func (ui *UI) closeEditorContextMenu() {
	if ui == nil {
		return
	}
	ui.editorMenuOpenID = ""
	ui.editorMenuTarget = nil
	ui.editorMenuPos = image.Point{}
	ui.editorMenuPressPos = image.Point{}
	ui.editorMenuRect = image.Rectangle{}
	ui.editorMenuOpenedAt = time.Time{}
	ui.editorMenuHoverAction = ""
	ui.editorMenuHoverAnim = segmentedAnimState{}
	ui.editorMenuCanPaste = false
	ui.editorMenuUseExplicitCaret = false
}

func (ui *UI) copyEditorText(gtx layout.Context, ed *widget.Editor) bool {
	if ed == nil {
		return false
	}
	text := ed.SelectedText()
	if text == "" {
		text = ed.Text()
	}
	if text == "" {
		return false
	}
	gtx.Execute(clipboard.WriteCmd{
		Type: "application/text",
		Data: io.NopCloser(strings.NewReader(text)),
	})
	return true
}

func (ui *UI) pasteEditorText(gtx layout.Context, ed *widget.Editor, enabled bool) bool {
	if ed == nil || !enabled || ed.ReadOnly {
		return false
	}
	ui.editorMenuClipboardTarget = nil
	ui.editorMenuClipboardUseCaret = false
	if text, err := readEditorContextClipboardText(); err == nil {
		ui.prepareEditorPasteTarget(ed, ui.editorMenuUseExplicitCaret)
		gtx.Execute(key.FocusCmd{Tag: ed})
		if ed.Insert(text) != 0 {
			gtx.Execute(op.InvalidateCmd{})
		}
		return true
	}
	ui.editorMenuClipboardTarget = ed
	ui.editorMenuClipboardUseCaret = ui.editorMenuUseExplicitCaret
	gtx.Execute(key.FocusCmd{Tag: ed})
	gtx.Execute(clipboard.ReadCmd{Tag: &ui.editorMenuClipboardTag})
	return true
}

func (ui *UI) prepareEditorPasteTarget(ed *widget.Editor, useExplicitCaret bool) {
	if ed == nil || useExplicitCaret {
		return
	}
	// Context-menu paste defaults to appending unless the editor already had
	// an explicit caret/selection (focused at menu-open time).
	ed.ClearSelection()
	ed.SetCaret(ed.Len(), ed.Len())
}

func (ui *UI) handleEditorContextMenuClipboardEvents(gtx layout.Context) {
	if ui == nil || ui.editorMenuClipboardTarget == nil {
		return
	}
	for {
		ev, ok := gtx.Event(transfer.TargetFilter{
			Target: &ui.editorMenuClipboardTag,
			Type:   "application/text",
		})
		if !ok {
			break
		}
		de, ok := ev.(transfer.DataEvent)
		if !ok {
			ui.editorMenuClipboardTarget = nil
			continue
		}
		data := de.Open()
		if data == nil {
			continue
		}
		content, err := io.ReadAll(data)
		_ = data.Close()
		target := ui.editorMenuClipboardTarget
		ui.editorMenuClipboardTarget = nil
		useExplicitCaret := ui.editorMenuClipboardUseCaret
		ui.editorMenuClipboardUseCaret = false
		if err != nil || target == nil || target.ReadOnly {
			continue
		}
		ui.prepareEditorPasteTarget(target, useExplicitCaret)
		if target.Insert(string(content)) != 0 {
			gtx.Execute(key.FocusCmd{Tag: target})
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (ui *UI) registerEditorContextMenuClipboardTarget(gtx layout.Context) {
	if ui == nil || ui.editorMenuClipboardTarget == nil {
		return
	}
	event.Op(gtx.Ops, &ui.editorMenuClipboardTag)
}

func (ui *UI) layoutEditorWithContextMenu(_ *material.Theme, gtx layout.Context, id string, ed *widget.Editor, enabled bool, host layout.Widget) layout.Dimensions {
	if ui == nil || id == "" || ed == nil || host == nil {
		if host == nil {
			return layout.Dimensions{}
		}
		return host(gtx)
	}

	tag := ui.editorMenuTag(id)
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Press})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok || pe.Kind != pointer.Press {
			continue
		}
		if pe.Buttons.Contain(pointer.ButtonSecondary) {
			ui.editorMenuOpenID = id
			ui.editorMenuTarget = ed
			ui.editorMenuRect = image.Rectangle{}
			ui.editorMenuOpenedAt = gtx.Now
			ui.editorMenuHoverAction = ""
			ui.editorMenuHoverAnim = segmentedAnimState{}
			ui.editorMenuCanPaste = enabled && !ed.ReadOnly
			ui.editorMenuUseExplicitCaret = gtx.Focused(ed)
			if ui.editorMenuPressPos != (image.Point{}) {
				ui.editorMenuPos = ui.editorMenuPressPos
			} else {
				ui.editorMenuPos = pe.Position.Round()
			}
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	m := op.Record(gtx.Ops)
	dims := host(gtx)
	hostCall := m.Stop()
	if dims.Size.X > 0 && dims.Size.Y > 0 {
		defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
		hostCall.Add(gtx.Ops)
	}
	if dims.Size.X > 0 && dims.Size.Y > 0 && tag != nil {
		pass := pointer.PassOp{}.Push(gtx.Ops)
		event.Op(gtx.Ops, tag)
		pass.Pop()
	} else if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		hostCall.Add(gtx.Ops)
	}

	return dims
}

func (ui *UI) layoutEditorContextMenuOverlay(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if ui == nil || ui.editorMenuOpenID == "" {
		return layout.Dimensions{}
	}
	id := ui.editorMenuOpenID
	ed := ui.editorMenuTarget
	if ed == nil {
		ui.closeEditorContextMenu()
		return layout.Dimensions{}
	}
	menuGtx := gtx
	menuGtx.Constraints.Min = image.Point{}
	m := op.Record(gtx.Ops)
	alpha, slideY, animating := popupOpenProgress(gtx.Now, ui.editorMenuOpenedAt)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	menuDims := ui.layoutEditorContextMenu(th, menuGtx, id, ui.editorMenuCanPaste, alpha)
	call := m.Stop()

	anchor := ui.editorMenuPos
	anchor.Y += slideY
	bounds := gtx.Constraints.Max
	if anchor.X+menuDims.Size.X > bounds.X {
		anchor.X = bounds.X - menuDims.Size.X
	}
	if anchor.Y+menuDims.Size.Y > bounds.Y {
		anchor.Y = bounds.Y - menuDims.Size.Y
	}
	if anchor.X < 0 {
		anchor.X = 0
	}
	if anchor.Y < 0 {
		anchor.Y = 0
	}
	ui.editorMenuRect = image.Rectangle{Min: anchor, Max: anchor.Add(menuDims.Size)}

	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	offset := op.Offset(anchor).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *UI) layoutEditorContextMenu(th *material.Theme, gtx layout.Context, id string, pasteEnabled bool, alpha float32) layout.Dimensions {
	if ui == nil || id == "" {
		return layout.Dimensions{}
	}
	theme := ui.filePanePopupTheme()
	copyHover, copyAnim := ui.editorMenuHoverAnim.hoverFill(gtx.Now, "copy")
	pasteHover, pasteAnim := ui.editorMenuHoverAnim.hoverFill(gtx.Now, "paste")
	wrapHover, wrapAnim := ui.editorMenuHoverAnim.hoverFill(gtx.Now, "word-wrap")
	if copyAnim || pasteAnim || wrapAnim {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	menuWidth := ui.editorContextMenuWidth(gtx)
	viewerFileEdit := id == "viewer-file-edit" && ui.fileViewer != nil
	wrapLabel := ""
	if viewerFileEdit {
		wrapLabel = viewerWordWrapMenuLabel(ui.fileViewer.wrapEnabled)
		lbl := material.Body2(th, wrapLabel)
		lbl.Font.Typeface = ui.mainTypeface()
		lbl.Font.Weight = font.Medium
		lbl.TextSize = ui.functionBarTextSize()
		if measured := measureLabelUnconstrained(gtx, lbl).Size.X + gtx.Dp(unit.Dp(20)); measured > menuWidth {
			menuWidth = measured
		}
	}
	return fixedWidth(gtx, menuWidth, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedClipBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			scaleColorAlpha(theme.Bg, alpha),
			scaleColorAlpha(theme.Border, alpha),
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					children := []layout.FlexChild{
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, ui.editorContextMenuRowHeight(gtx), func(gtx layout.Context) layout.Dimensions {
								return ui.layoutEditorContextMenuSegment(th, gtx, menuWidth, "Copy", copyHover, true, alpha)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, ui.fileContextMenuSeparatorHeight(gtx), func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									h := gtx.Dp(unit.Dp(1))
									if h < 1 {
										h = 1
									}
									return fillBgExact(gtx, scaleColorAlpha(theme.Divider, alpha), func(gtx layout.Context) layout.Dimensions {
										return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
									})
								})
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, ui.editorContextMenuRowHeight(gtx), func(gtx layout.Context) layout.Dimensions {
								return ui.layoutEditorContextMenuSegment(th, gtx, menuWidth, "Paste", pasteHover, pasteEnabled, alpha)
							})
						}),
					}
					if viewerFileEdit {
						children = append(children,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, ui.fileContextMenuSeparatorHeight(gtx), func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										h := max(1, gtx.Dp(unit.Dp(1)))
										return fillBgExact(gtx, scaleColorAlpha(theme.Divider, alpha), func(gtx layout.Context) layout.Dimensions {
											return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
										})
									})
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, ui.editorContextMenuRowHeight(gtx), func(gtx layout.Context) layout.Dimensions {
									return ui.layoutEditorContextMenuSegment(th, gtx, menuWidth, wrapLabel, wrapHover, true, alpha)
								})
							}),
						)
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			},
		)
	})
}

func (ui *UI) layoutEditorContextMenuSegment(th *material.Theme, gtx layout.Context, menuWidth int, label string, hoverFill float32, enabled bool, alpha float32) layout.Dimensions {
	theme := ui.filePanePopupTheme()
	return fixedWidth(gtx, menuWidth, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{}
		fg := scaleColorAlpha(theme.Text, alpha)
		hoverT := smoothstep01(clamp01(hoverFill))
		if !enabled {
			fg = scaleColorAlpha(theme.DisabledText, alpha)
		} else if hoverT > 0 {
			bg = scaleColorAlpha(theme.HoverBg, alpha*hoverT)
			fg = scaleColorAlpha(mixNRGBA(theme.Text, theme.HoverText, hoverT), alpha)
		}
		return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(6), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, label)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.Font.Weight = font.Medium
						lbl.TextSize = ui.functionBarTextSize()
						lbl.Color = fg
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					})
				})
			})
		})
	})
}

func (ui *UI) editorContextMenuRowHeight(gtx layout.Context) int {
	return ui.fileContextMenuRowHeight(gtx, fileContextMenuItem{})
}

func (ui *UI) editorContextMenuWidth(gtx layout.Context) int {
	w := gtx.Dp(unit.Dp(88))
	if w < 1 {
		w = 1
	}
	return w
}

func (ui *UI) editorContextMenuActionAt(gtx layout.Context, pos image.Point) string {
	if ui == nil || ui.editorMenuRect.Dx() <= 0 || ui.editorMenuRect.Dy() <= 0 {
		return ""
	}
	if pos.X < ui.editorMenuRect.Min.X || pos.X >= ui.editorMenuRect.Max.X ||
		pos.Y < ui.editorMenuRect.Min.Y || pos.Y >= ui.editorMenuRect.Max.Y {
		return ""
	}
	rowH := ui.editorContextMenuRowHeight(gtx)
	divH := ui.fileContextMenuSeparatorHeight(gtx)
	localY := pos.Y - ui.editorMenuRect.Min.Y
	switch {
	case localY < rowH:
		return "copy"
	case localY >= rowH+divH && localY < rowH+divH+rowH:
		return "paste"
	case ui.editorMenuOpenID == "viewer-file-edit" &&
		localY >= rowH+divH+rowH+divH &&
		localY < rowH+divH+rowH+divH+rowH:
		return "word-wrap"
	default:
		return ""
	}
}

func (ui *UI) editorForMenuID(id string) (*widget.Editor, bool) {
	if ui == nil || id == "" {
		return nil, false
	}
	switch id {
	case "tab0-left":
		return &ui.LeftEd, true
	case "tab0-right":
		return &ui.RightEd, true
	case "tab2-hex":
		if ui.tab2State == nil {
			return nil, false
		}
		return &ui.tab2State.hexEd, true
	case "viewer-command":
		if ui.fileViewer == nil {
			return nil, false
		}
		return &ui.fileViewer.commandEditor, true
	case "viewer-find":
		if ui.fileViewer == nil {
			return nil, false
		}
		return &ui.fileViewer.find.editor, true
	case "settings-view-command":
		if ui.settingsModal == nil {
			return nil, false
		}
		return &ui.settingsModal.viewCommandEdit, true
	case "settings-view-shell":
		if ui.settingsModal == nil {
			return nil, false
		}
		return &ui.settingsModal.viewShellEdit, true
	case "settings-view-assoc-ext":
		if ui.settingsModal == nil {
			return nil, false
		}
		return &ui.settingsModal.viewAssocExtEdit, true
	case "settings-view-assoc-app":
		if ui.settingsModal == nil {
			return nil, false
		}
		return &ui.settingsModal.viewAssocAppEdit, true
	case "settings-config":
		if ui.settingsModal == nil {
			return nil, false
		}
		return &ui.settingsModal.configEdit, true
	case "filecopy-dst":
		if ui.fileCopy == nil {
			return nil, false
		}
		return &ui.fileCopy.dstEdit, true
	case "filemove-dst":
		if ui.fileMove == nil {
			return nil, false
		}
		return &ui.fileMove.dstEdit, true
	case "filecreate-name":
		if ui.fileCreate == nil {
			return nil, false
		}
		return &ui.fileCreate.nameEdit, true
	case "fileperm-digits":
		if ui.filePerm == nil {
			return nil, false
		}
		return &ui.filePerm.permEdit, true
	}
	if strings.HasPrefix(id, "pane-path-") {
		idx, err := strconv.Atoi(strings.TrimPrefix(id, "pane-path-"))
		if err != nil || idx < 0 || idx >= len(ui.filePanes) || ui.filePanes[idx] == nil {
			return nil, false
		}
		return &ui.filePanes[idx].pathEdit, true
	}
	if strings.HasPrefix(id, "ssh-") {
		if ui.sshModal == nil {
			return nil, false
		}
		switch id {
		case "ssh-name":
			return &ui.sshModal.nameEdit, true
		case "ssh-host":
			return &ui.sshModal.hostEdit, true
		case "ssh-port":
			return &ui.sshModal.portEdit, true
		case "ssh-user":
			return &ui.sshModal.userEdit, true
		case "ssh-password":
			return &ui.sshModal.passEdit, true
		case "ssh-key-path":
			return &ui.sshModal.keyPathEdit, true
		case "ssh-passphrase":
			return &ui.sshModal.keyPassEdit, true
		}
	}
	return nil, false
}
