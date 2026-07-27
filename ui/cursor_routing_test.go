// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"hexone/fm"
)

func TestTerminalClipboardTargetDoesNotMaskCursorAndSchedulesTimeout(t *testing.T) {
	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	st := newTerminalSession(nil)
	st.beginPasteRead(now)
	ui := &UI{terminal: st}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Now:         now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(320, 200)),
	}

	ui.handleTerminalClipboardEvents(gtx)
	cursorArea := clip.Rect(image.Rect(40, 40, 280, 160)).Push(&ops)
	pointer.CursorText.Add(&ops)
	cursorArea.Pop()
	ui.registerTerminalClipboardTarget(gtx)
	router.Frame(&ops)

	deadline := now.Add(terminalPasteReadTimeout)
	if got, wake := router.WakeupTime(); !wake || !got.Equal(deadline) {
		t.Fatalf("clipboard timeout wake=(%v, %v), want (%v, true)", got, wake, deadline)
	}

	router.Queue(pointer.Event{
		Kind:     pointer.Move,
		Source:   pointer.Mouse,
		Position: f32.Pt(100, 100),
	})
	if got, want := router.Cursor(), pointer.CursorText; got != want {
		t.Fatalf("pending terminal clipboard cursor=%v want underlying %v", got, want)
	}

	ops.Reset()
	gtx.Now = deadline
	ui.handleTerminalClipboardEvents(gtx)
	cursorArea = clip.Rect(image.Rect(40, 40, 280, 160)).Push(&ops)
	pointer.CursorText.Add(&ops)
	cursorArea.Pop()
	ui.registerTerminalClipboardTarget(gtx)
	router.Frame(&ops)
	if st.pasteReadPending(gtx.Now) {
		t.Fatal("terminal clipboard pending state should clear at its scheduled deadline")
	}
}

func TestEditorClipboardTargetDoesNotMaskCursorAndExpires(t *testing.T) {
	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	var ed widget.Editor
	ui := &UI{
		editorMenuClipboardTarget:    &ed,
		editorMenuClipboardPendingAt: now,
	}

	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Now:         now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(320, 200)),
	}

	ui.handleEditorContextMenuClipboardEvents(gtx)
	cursorArea := clip.Rect(image.Rect(40, 40, 280, 160)).Push(&ops)
	pointer.CursorText.Add(&ops)
	cursorArea.Pop()
	ui.registerEditorContextMenuClipboardTarget(gtx)
	router.Frame(&ops)

	deadline := now.Add(editorClipboardReadTimeout)
	if got, wake := router.WakeupTime(); !wake || !got.Equal(deadline) {
		t.Fatalf("editor clipboard timeout wake=(%v, %v), want (%v, true)", got, wake, deadline)
	}

	router.Queue(pointer.Event{
		Kind:     pointer.Move,
		Source:   pointer.Mouse,
		Position: f32.Pt(100, 100),
	})
	if got, want := router.Cursor(), pointer.CursorText; got != want {
		t.Fatalf("pending editor clipboard cursor=%v want underlying %v", got, want)
	}

	ops.Reset()
	gtx.Now = deadline
	ui.handleEditorContextMenuClipboardEvents(gtx)
	ui.registerEditorContextMenuClipboardTarget(gtx)
	router.Frame(&ops)
	if ui.editorMenuClipboardTarget != nil {
		t.Fatal("editor clipboard target should clear at its scheduled deadline")
	}
	if !ui.editorMenuClipboardPendingAt.IsZero() {
		t.Fatal("editor clipboard deadline should clear with its target")
	}
}

func TestFunctionBarCursorDoesNotLeakAcrossWindow(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	defer ui.Close()

	th := material.NewTheme()
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Now:         time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(800, 600)),
	}
	frame := func() {
		gtx.Now = gtx.Now.Add(time.Millisecond)
		ops.Reset()
		ui.layoutFunctionBar(th, gtx)
		router.Frame(&ops)
	}

	frame()
	router.Queue(pointer.Event{
		Kind:     pointer.Move,
		Source:   pointer.Mouse,
		Position: f32.Pt(20, 10),
	})
	frame()
	if got, want := router.Cursor(), pointer.CursorPointer; got != want {
		t.Fatalf("function bar cursor=%v want %v", got, want)
	}

	router.Queue(pointer.Event{
		Kind:     pointer.Move,
		Source:   pointer.Mouse,
		Position: f32.Pt(400, 300),
	})
	frame()
	if got, want := router.Cursor(), pointer.CursorDefault; got != want {
		t.Fatalf("cursor outside function bar=%v want %v", got, want)
	}
}

func TestLayoutClippedToDimensionsScopesCursor(t *testing.T) {
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(800, 600)),
	}
	got := layoutClippedToDimensions(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: image.Pt(800, 24)}
	})
	if got.Size != image.Pt(800, 24) {
		t.Fatalf("dimensions=%v want (800,24)", got.Size)
	}
	router.Frame(&ops)
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(400, 300)})
	if got, want := router.Cursor(), pointer.CursorDefault; got != want {
		t.Fatalf("cursor outside clipped dimensions=%v want %v", got, want)
	}
}
