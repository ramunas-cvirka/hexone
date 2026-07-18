// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type fileViewerModeSwitchAction uint8

const fileViewerModeSwitchDialogWidthDp = 260

const (
	fileViewerModeSwitchDiscard fileViewerModeSwitchAction = iota
	fileViewerModeSwitchSave
)

type fileViewerModeSwitchState struct {
	open         bool
	targetMode   string
	awaitingSave bool
	actionFocus  fileViewerModeSwitchAction
	keyFocus     dialogKeyboardFocusState
	backdrop     widget.Clickable
	discardClick widget.Clickable
	saveClick    widget.Clickable
	actionsAnim  segmentedAnimState
}

func (st *fileViewerModeSwitchState) openFor(targetMode string, awaitingSave bool) {
	if st == nil {
		return
	}
	st.open = true
	st.targetMode = normalizeViewerMode(targetMode)
	st.awaitingSave = awaitingSave
	st.actionFocus = fileViewerModeSwitchSave
	st.keyFocus.focusKeyboard()
	st.actionsAnim = segmentedAnimState{}
}

func (st *fileViewerModeSwitchState) close() {
	if st == nil {
		return
	}
	st.open = false
	st.targetMode = ""
	st.awaitingSave = false
	st.actionFocus = fileViewerModeSwitchSave
	st.actionsAnim = segmentedAnimState{}
}

func (st *fileViewerModeSwitchState) stepAction(step int) bool {
	if st == nil || st.awaitingSave {
		return false
	}
	order := []fileViewerModeSwitchAction{fileViewerModeSwitchDiscard, fileViewerModeSwitchSave}
	current := 0
	for i, action := range order {
		if action == st.actionFocus {
			current = i
			break
		}
	}
	next := order[dialogWrappedIndex(current, len(order), step)]
	if next == st.actionFocus {
		return false
	}
	st.actionFocus = next
	return true
}

func (st *fileViewerModeSwitchState) actionVisualState(target fileViewerModeSwitchAction) dialogActionVisualState {
	if st == nil {
		return dialogActionVisualState{}
	}
	active := st.actionFocus == target
	return dialogActionVisualState{Focused: active, Default: active}
}

func viewerModeDialogName(mode string) string {
	switch normalizeViewerMode(mode) {
	case "hex":
		return "Hex"
	case "command":
		return "Cmd"
	default:
		return "File"
	}
}

func (ui *UI) openFileViewerModeSwitchPrompt(st *fileViewerState, targetMode string, awaitingSave bool) {
	if st == nil {
		return
	}
	st.closeContextMenu()
	st.closeEncodingMenu()
	ui.closeEditorContextMenu()
	st.modeSwitchPrompt.openFor(targetMode, awaitingSave)
}

func (ui *UI) closeFileViewerModeSwitchPrompt(st *fileViewerState) {
	if st == nil {
		return
	}
	st.modeSwitchPrompt.close()
}

func (ui *UI) confirmFileViewerModeSwitchDiscard(st *fileViewerState, now time.Time) {
	if st == nil || !st.modeSwitchPrompt.open || st.modeSwitchPrompt.awaitingSave {
		return
	}
	target := st.modeSwitchPrompt.targetMode
	ui.closeFileViewerModeSwitchPrompt(st)
	ui.discardFileViewerChanges(st)
	ui.setFileViewerMode(target, now)
}

func (ui *UI) confirmFileViewerModeSwitchSave(st *fileViewerState, now time.Time) {
	if st == nil || !st.modeSwitchPrompt.open || st.modeSwitchPrompt.awaitingSave {
		return
	}
	if ui.startFileViewerSave(now) {
		st.modeSwitchPrompt.awaitingSave = true
		return
	}
	if !st.editDirty && !st.saving {
		target := st.modeSwitchPrompt.targetMode
		ui.closeFileViewerModeSwitchPrompt(st)
		ui.setFileViewerMode(target, now)
	}
}

func (ui *UI) finishFileViewerModeSwitchSave(st *fileViewerState, succeeded bool, now time.Time) {
	if st == nil || !st.modeSwitchPrompt.open || !st.modeSwitchPrompt.awaitingSave {
		return
	}
	st.modeSwitchPrompt.awaitingSave = false
	if !succeeded || st.editDirty {
		st.modeSwitchPrompt.actionFocus = fileViewerModeSwitchSave
		st.modeSwitchPrompt.keyFocus.focusKeyboard()
		return
	}
	target := st.modeSwitchPrompt.targetMode
	ui.closeFileViewerModeSwitchPrompt(st)
	ui.setFileViewerMode(target, now)
}

func (ui *UI) handleFileViewerModeSwitchKeys(gtx layout.Context, st *fileViewerState) {
	if st == nil || !st.modeSwitchPrompt.open {
		return
	}
	prompt := &st.modeSwitchPrompt
	anyMods := ^key.Modifiers(0)
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &prompt.keyFocus.tag, Name: key.NameEscape, Optional: anyMods},
			key.Filter{Focus: &prompt.keyFocus.tag, Name: key.NameTab, Optional: anyMods},
			key.Filter{Focus: &prompt.keyFocus.tag, Name: key.NameLeftArrow, Optional: anyMods},
			key.Filter{Focus: &prompt.keyFocus.tag, Name: key.NameRightArrow, Optional: anyMods},
			key.Filter{Focus: &prompt.keyFocus.tag, Name: key.NameEnter, Optional: anyMods},
			key.Filter{Focus: &prompt.keyFocus.tag, Name: key.NameReturn, Optional: anyMods},
		)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case key.NameEscape:
			if !prompt.awaitingSave {
				ui.closeFileViewerModeSwitchPrompt(st)
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameTab:
			step, ok := dialogTabStep(ke.Modifiers)
			if ok && prompt.stepAction(step) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameLeftArrow:
			if ke.Modifiers == 0 && prompt.stepAction(-1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameRightArrow:
			if ke.Modifiers == 0 && prompt.stepAction(1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameEnter, key.NameReturn:
			if ke.Modifiers != 0 || prompt.awaitingSave {
				continue
			}
			switch prompt.actionFocus {
			case fileViewerModeSwitchDiscard:
				prompt.actionsAnim.setPulse("discard", gtx.Now)
				ui.confirmFileViewerModeSwitchDiscard(st, gtx.Now)
			case fileViewerModeSwitchSave:
				prompt.actionsAnim.setPulse("save", gtx.Now)
				ui.confirmFileViewerModeSwitchSave(st, gtx.Now)
			}
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (ui *UI) layoutFileViewerModeSwitchModal(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	if st == nil || !st.modeSwitchPrompt.open {
		return layout.Dimensions{}
	}
	prompt := &st.modeSwitchPrompt
	prompt.keyFocus.attach(gtx)
	ui.handleFileViewerModeSwitchKeys(gtx, st)
	if !prompt.open {
		return layout.Dimensions{}
	}

	if prompt.discardClick.Clicked(gtx) && !prompt.awaitingSave {
		prompt.actionsAnim.setPulse("discard", gtx.Now)
		ui.confirmFileViewerModeSwitchDiscard(st, gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	if prompt.saveClick.Clicked(gtx) && !prompt.awaitingSave {
		prompt.actionsAnim.setPulse("save", gtx.Now)
		ui.confirmFileViewerModeSwitchSave(st, gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	for prompt.backdrop.Clicked(gtx) {
	}

	return prompt.backdrop.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{A: 154}, clip.Rect(image.Rectangle{Max: size}).Op())

		width := gtx.Dp(ui.scaleInterfaceDp(unit.Dp(fileViewerModeSwitchDialogWidthDp)))
		maxWidth := size.X - gtx.Dp(unit.Dp(24))
		if width > maxWidth {
			width = maxWidth
		}
		if width < 1 {
			width = 1
		}

		record := op.Record(gtx.Ops)
		dialogGtx := gtx
		dialogGtx.Constraints.Min = image.Point{}
		dialog := fixedWidth(dialogGtx, width, func(gtx layout.Context) layout.Dimensions {
			return fillRoundedBox(
				gtx,
				gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
				color.NRGBA{R: 20, G: 20, B: 20, A: 252},
				color.NRGBA{R: 255, G: 255, B: 255, A: 24},
				func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutFileViewerModeSwitchBody(th, gtx, st)
					})
				},
			)
		})
		call := record.Stop()

		pos := image.Pt((size.X-dialog.Size.X)/2, (size.Y-dialog.Size.Y)/2)
		if pos.X < 0 {
			pos.X = 0
		}
		if pos.Y < 0 {
			pos.Y = 0
		}
		offset := op.Offset(pos).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: size, Baseline: dialog.Baseline}
	})
}

func (ui *UI) layoutFileViewerModeSwitchBody(th *material.Theme, gtx layout.Context, st *fileViewerState) layout.Dimensions {
	prompt := &st.modeSwitchPrompt
	hoverKey := ""
	if !prompt.awaitingSave && prompt.discardClick.Hovered() {
		hoverKey = "discard"
	}
	if !prompt.awaitingSave && prompt.saveClick.Hovered() {
		hoverKey = "save"
	}
	prompt.actionsAnim.setHover(hoverKey, gtx.Now)
	hoverDiscard, hoverDiscardAnim := prompt.actionsAnim.hoverFill(gtx.Now, "discard")
	hoverSave, hoverSaveAnim := prompt.actionsAnim.hoverFill(gtx.Now, "save")
	pulseDiscard, pulseDiscardAnim := prompt.actionsAnim.pulseFill(gtx.Now, "discard")
	pulseSave, pulseSaveAnim := prompt.actionsAnim.pulseFill(gtx.Now, "save")
	if hoverDiscardAnim || hoverSaveAnim || pulseDiscardAnim || pulseSaveAnim {
		gtx.Execute(op.InvalidateCmd{})
	}

	saveLabel := "Save"
	if prompt.awaitingSave {
		saveLabel = "Saving..."
	}
	transition := fmt.Sprintf("%s → %s", viewerModeDialogName(st.mode), viewerModeDialogName(prompt.targetMode))

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			title := material.Body1(th, "Unsaved changes")
			title.Font.Typeface = ui.interfaceTypeface()
			title.Font.Weight = font.Bold
			title.TextSize = ui.scaleDialogFontSize(12)
			title.Color = txtColor
			return title.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(layoutDialogHorizontalDivider),
		layout.Rigid(layout.Spacer{Height: unit.Dp(9)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, "Before switching modes, choose what to do with your unsaved changes.")
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleDialogFontSize(10)
			lbl.Color = txtColor
			lbl.MaxLines = 3
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, transition)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleDialogFontSize(9)
			lbl.Color = hintColor
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.err == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, st.err)
				lbl.Font.Typeface = ui.interfaceTypeface()
				lbl.TextSize = ui.scaleDialogFontSize(9)
				lbl.Color = color.NRGBA{R: 220, G: 140, B: 140, A: 255}
				lbl.MaxLines = 2
				return lbl.Layout(gtx)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(layoutDialogHorizontalDivider),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutDialogActionPair(
					th, gtx,
					&prompt.discardClick, "Discard", hoverDiscard, pulseDiscard, prompt.awaitingSave,
					&prompt.saveClick, saveLabel, hoverSave, pulseSave, prompt.awaitingSave,
					prompt.actionVisualState(fileViewerModeSwitchDiscard),
					prompt.actionVisualState(fileViewerModeSwitchSave),
				)
			})
		}),
	)
}
