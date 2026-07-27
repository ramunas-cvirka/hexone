// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build uiverify

package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"hexone/fm"
)

func TestHeadlessViewerEditModes(t *testing.T) {
	outDir := os.Getenv("VIEWER_EDIT_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	const width, height = 920, 420
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("headless window: %v", err)
	}
	defer win.Release()

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	router := new(input.Router)
	frame := func(draw func(layout.Context)) *image.RGBA {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)),
			Now:         time.Now(),
			Source:      router.Source(),
		}
		draw(gtx)
		router.Frame(&ops)
		if err := win.Frame(&ops); err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		if err := win.Screenshot(img); err != nil {
			t.Fatal(err)
		}
		return img
	}
	shoot := func(name string, img *image.RGBA) {
		path := filepath.Join(outDir, name)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}

	tabUI := NewUI(fm.DefaultConfig())
	tabContent := "editable tabs"
	tabState := &fileViewerState{
		mode:             "file",
		path:             "viewer-tabs.txt",
		name:             "swagger-2.yaml",
		content:          tabContent,
		editBaselineText: tabContent,
	}
	tabState.contentEditor.SetText(tabContent)
	tabState.stream.SetContent(tabContent)
	tabUI.fileViewer = tabState
	drawTabs := func(gtx layout.Context) {
		paint.Fill(gtx.Ops, color.NRGBA{R: 25, G: 27, B: 31, A: 255})
		gtx.Constraints.Max.Y = 30
		tabUI.layoutFileViewerHeaderRow(th, gtx, tabState, 24)
	}
	shoot("viewer-header-view.png", frame(drawTabs))
	shoot("viewer-header-narrow.png", frame(func(gtx layout.Context) {
		paint.Fill(gtx.Ops, color.NRGBA{R: 25, G: 27, B: 31, A: 255})
		gtx.Constraints = layout.Exact(image.Pt(600, 30))
		tabUI.layoutFileViewerHeaderRow(th, gtx, tabState, 24)
	}))
	tabState.name = "swagger-generated-client-with-a-very-long-filename.openapi.yaml"
	shoot("viewer-header-tight.png", frame(func(gtx layout.Context) {
		paint.Fill(gtx.Ops, color.NRGBA{R: 25, G: 27, B: 31, A: 255})
		gtx.Constraints = layout.Exact(image.Pt(430, 30))
		tabUI.layoutFileViewerHeaderRow(th, gtx, tabState, 24)
	}))
	tabState.name = "swagger-2.yaml"
	hexHover := f32.Pt(float32(8+tabState.activeTabRect.Max.X+2+40), 18)
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: hexHover})
	shoot("viewer-header-hover.png", frame(drawTabs))
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(600, 120)})
	frame(drawTabs)
	if !tabUI.toggleFileViewerEdit(time.Now()) {
		t.Fatalf("toggle tab edit: %s", tabState.status)
	}
	shoot("viewer-header-editing.png", frame(drawTabs))
	tabState.editDirty = true
	shoot("viewer-header-dirty.png", frame(drawTabs))
	tabState.editDirty = false
	tabUI.setFileViewerMode("hex", time.Now())
	shoot("viewer-header-hex.png", frame(drawTabs))
	tabState.command = "cat {path}"
	tabUI.setFileViewerMode("command", time.Now())
	shoot("viewer-header-command.png", frame(drawTabs))
	cmdActionHover := f32.Pt(float32(8+tabState.activeTabRect.Min.X+27), 18)
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: cmdActionHover})
	shoot("viewer-header-command-action-hover.png", frame(drawTabs))
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(600, 120)})
	frame(drawTabs)
	tabState.setHistoryOpen(true, time.Now())
	shoot("viewer-header-command-history.png", frame(drawTabs))
	tabState.setHistoryOpen(false, time.Now())

	modalUI := NewUI(fm.DefaultConfig())
	modalState := &fileViewerState{
		mode:             "file",
		path:             "unsaved.txt",
		name:             "unsaved.txt",
		editMode:         true,
		editDirty:        true,
		editBaselineText: "before",
	}
	modalState.contentEditor.SetText("after")
	modalState.modeSwitchPrompt.openFor("hex", false)
	modalUI.fileViewer = modalState
	shoot("viewer-mode-switch.png", frame(func(gtx layout.Context) {
		paint.Fill(gtx.Ops, color.NRGBA{R: 25, G: 27, B: 31, A: 255})
		modalUI.layoutFileViewerModeSwitchModal(th, gtx, modalState)
	}))

	var source strings.Builder
	source.WriteString("package main\n\nimport \"fmt\"\n\n")
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&source, "func line%02d() { fmt.Println(\"cyan\", %d) } // editable syntax\n", i, i)
	}
	textUI := NewUI(fm.DefaultConfig())
	textState := &fileViewerState{
		mode:             "file",
		path:             "viewer_edit.go",
		name:             "viewer_edit.go",
		content:          source.String(),
		editBaselineText: source.String(),
	}
	textState.stream.SetContent(source.String())
	textState.stream.setSyntax(viewerBuildSyntaxDocument(context.Background(), textState.path, source.String()))
	textUI.fileViewer = textState
	textReadImage := frame(func(gtx layout.Context) {
		textUI.layoutStreamOutputView(th, gtx, textState)
	})
	readLineHeight := textState.stream.lineH
	shoot("viewer-text-read.png", textReadImage)
	if !textUI.startFileViewerEdit(time.Now()) {
		t.Fatalf("start text edit: %s", textState.status)
	}
	textImage := frame(func(gtx layout.Context) {
		textUI.layoutFileViewerTextEditor(th, gtx, textState)
	})
	if textState.stream.totalRows() <= textState.stream.visibleLines || textState.stream.trackRect.Empty() {
		t.Fatal("text edit visual fixture did not produce a scrollbar")
	}
	editorLineHeight := textState.stream.lineH
	t.Logf("read line height=%d; virtual editor line height=%d", readLineHeight, editorLineHeight)
	delta := editorLineHeight - readLineHeight
	if delta < 0 {
		delta = -delta
	}
	if delta > 1 {
		t.Fatalf("read/editor line-height delta=%d want <= 1", delta)
	}
	shoot("viewer-text-edit.png", textImage)
	textState.stream.beginSelection(18)
	textState.stream.updateSelection(170)
	textSelectionImage := frame(func(gtx layout.Context) {
		textUI.layoutFileViewerTextEditor(th, gtx, textState)
	})
	shoot("viewer-text-edit-selection.png", textSelectionImage)
	textState.stream.clearSelection()
	textState.stream.scrollToBottom()
	textBottomImage := frame(func(gtx layout.Context) {
		textUI.layoutFileViewerTextEditor(th, gtx, textState)
	})
	shoot("viewer-text-edit-bottom.png", textBottomImage)

	wrapContent := strings.Repeat("word-wrap-setting must keep this logical line horizontal when disabled ", 18)
	wrapUI := NewUI(fm.DefaultConfig())
	wrapState := &fileViewerState{
		mode:             "file",
		path:             "word-wrap.txt",
		name:             "word-wrap.txt",
		content:          wrapContent,
		editBaselineText: wrapContent,
		wrapEnabled:      false,
	}
	wrapState.stream.SetContent(wrapContent)
	wrapUI.fileViewer = wrapState
	if !wrapUI.startFileViewerEdit(time.Now()) {
		t.Fatalf("start wrap fixture edit: %s", wrapState.status)
	}
	noWrapImage := frame(func(gtx layout.Context) {
		wrapUI.layoutFileViewerTextEditor(th, gtx, wrapState)
	})
	shoot("viewer-text-edit-nowrap.png", noWrapImage)
	wrapState.wrapEnabled = true
	wrapImage := frame(func(gtx layout.Context) {
		wrapUI.layoutFileViewerTextEditor(th, gtx, wrapState)
	})
	shoot("viewer-text-edit-wrap.png", wrapImage)

	hexUI := NewUI(fm.DefaultConfig())
	hexState := newHexViewerState()
	hexState.fileSize = 256
	hexState.buffer = make([]byte, hexState.fileSize)
	for i := range hexState.buffer {
		hexState.buffer[i] = byte(32 + i%95)
	}
	hexState.offsetDigits = 8
	setHexViewerEditCaret(hexState, 3, false)
	hexState.setEditedByte(3, 0xE1)
	hexState.setEditedByte(4, 0x7F)
	hexState.editNibble = 1
	viewState := &fileViewerState{
		mode:     "hex",
		name:     "viewer-edit.bin",
		editMode: true,
		hex:      hexState,
	}
	hexUI.fileViewer = viewState
	hexImage := frame(func(gtx layout.Context) {
		hexUI.layoutHexOutputView(th, gtx, viewState)
	})
	shoot("viewer-hex-edit.png", hexImage)

	for range 4 {
		if !hexUI.handleFileViewerHexEditKey(viewState, key.Event{
			Name:      key.NameRightArrow,
			Modifiers: key.ModShift,
			State:     key.Press,
		}) {
			t.Fatal("keyboard HEX selection was not handled")
		}
	}
	if hexState.selectionStart != 3 || hexState.selectionLen != 5 || hexState.editCaret != 7 {
		t.Fatalf(
			"keyboard HEX selection=(%d,%d) caret=%d want (3,5), caret 7",
			hexState.selectionStart,
			hexState.selectionLen,
			hexState.editCaret,
		)
	}
	hexSelectionImage := frame(func(gtx layout.Context) {
		hexUI.layoutHexOutputView(th, gtx, viewState)
	})
	shoot("viewer-hex-edit-selection.png", hexSelectionImage)
	hexState.editNibble = 1
	hexSelectionLowImage := frame(func(gtx layout.Context) {
		hexUI.layoutHexOutputView(th, gtx, viewState)
	})
	shoot("viewer-hex-edit-selection-low.png", hexSelectionLowImage)

	hexDragStart := f32.Pt(
		float32(hexState.hexRect.Min.X+hexState.hexByteLeft(3)+hexState.charW),
		float32(hexState.lineH/2),
	)
	asciiDragEnd := f32.Pt(
		float32(hexState.textRect.Min.X+7*hexState.charW+hexState.charW/2),
		float32(hexState.lineH/2),
	)
	router.Queue(pointer.Event{
		Kind:      pointer.Press,
		Source:    pointer.Mouse,
		PointerID: 11,
		Buttons:   pointer.ButtonPrimary,
		Position:  hexDragStart,
	})
	frame(func(gtx layout.Context) {
		hexUI.layoutHexOutputView(th, gtx, viewState)
	})
	router.Queue(pointer.Event{
		Kind:      pointer.Move,
		Source:    pointer.Mouse,
		PointerID: 11,
		Buttons:   pointer.ButtonPrimary,
		Position:  asciiDragEnd,
	})
	asciiSelectionImage := frame(func(gtx layout.Context) {
		hexUI.layoutHexOutputView(th, gtx, viewState)
	})
	if !hexState.editASCII || hexState.selectionStart != 3 || hexState.selectionLen != 5 || hexState.editCaret != 7 {
		t.Fatalf(
			"cross-lane drag ascii=%v selection=(%d,%d) caret=%d want true, (3,5), caret 7",
			hexState.editASCII,
			hexState.selectionStart,
			hexState.selectionLen,
			hexState.editCaret,
		)
	}
	shoot("viewer-ascii-edit-selection.png", asciiSelectionImage)
	router.Queue(pointer.Event{
		Kind:      pointer.Release,
		Source:    pointer.Mouse,
		PointerID: 11,
		Position:  asciiDragEnd,
	})
	frame(func(gtx layout.Context) {
		hexUI.layoutHexOutputView(th, gtx, viewState)
	})

	setHexViewerEditCaret(hexState, 3, true)
	asciiImage := frame(func(gtx layout.Context) {
		hexUI.layoutHexOutputView(th, gtx, viewState)
	})
	shoot("viewer-ascii-edit.png", asciiImage)

	wheelUI := NewUI(fm.DefaultConfig())
	wheelHex := newHexViewerState()
	wheelHex.fileSize = 4096
	wheelHex.buffer = make([]byte, wheelHex.fileSize)
	for i := range wheelHex.buffer {
		wheelHex.buffer[i] = byte(i)
	}
	wheelState := &fileViewerState{
		mode:     "hex",
		name:     "fine-wheel.bin",
		editMode: true,
		hex:      wheelHex,
	}
	wheelUI.fileViewer = wheelState
	frame(func(gtx layout.Context) {
		wheelUI.layoutHexOutputView(th, gtx, wheelState)
	})
	wheelPos := f32.Pt(float32(wheelHex.hexRect.Min.X+8), float32(wheelHex.hexRect.Min.Y+8))
	router.Queue(pointer.Event{
		Kind:     pointer.Scroll,
		Source:   pointer.Mouse,
		Position: wheelPos,
		Scroll:   f32.Pt(0, 0.5),
	})
	wheelImage := frame(func(gtx layout.Context) {
		wheelUI.layoutHexOutputView(th, gtx, wheelState)
	})
	if wheelHex.topLine != 1 {
		t.Fatalf("fine wheel event top line=%d want 1", wheelHex.topLine)
	}
	shoot("viewer-hex-edit-fine-wheel.png", wheelImage)
}
