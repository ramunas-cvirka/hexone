// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"reflect"
	"unsafe"

	"gioui.org/layout"
	"gioui.org/widget"
)

type editorScrollMetrics struct {
	Offset    int
	Viewport  int
	Content   int
	MaxOffset int
}

type editorTextViewAccess interface {
	Dimensions() layout.Dimensions
	FullDimensions() layout.Dimensions
	ScrollBounds() image.Rectangle
	ScrollOff() image.Point
	ScrollRel(dx, dy int)
	MoveCoord(image.Point)
	Selection() (start, end int)
	SetCaret(start, end int)
}

func editorVerticalScrollMetrics(ed *widget.Editor) (editorScrollMetrics, bool) {
	textView, ok := editorTextView(ed)
	if !ok {
		return editorScrollMetrics{}, false
	}
	visibleDims := textView.Dimensions()
	fullDims := textView.FullDimensions()
	viewport := visibleDims.Size.Y
	content := fullDims.Size.Y
	if viewport <= 0 || content <= 0 {
		return editorScrollMetrics{}, false
	}
	maxOffset := textView.ScrollBounds().Max.Y
	if maxOffset < 0 {
		maxOffset = 0
	}
	return editorScrollMetrics{
		Offset:    textView.ScrollOff().Y,
		Viewport:  viewport,
		Content:   content,
		MaxOffset: maxOffset,
	}, content > viewport && maxOffset > 0
}

func editorScrollToVerticalOffset(ed *widget.Editor, offset int) {
	textView, ok := editorTextView(ed)
	if !ok {
		return
	}
	current := textView.ScrollOff().Y
	textView.ScrollRel(0, offset-current)
}

func editorVisibleRuneRange(ed *widget.Editor) (start, end int, ok bool) {
	textView, ok := editorTextView(ed)
	if !ok {
		return 0, 0, false
	}
	size := textView.Dimensions().Size
	if size.X <= 0 || size.Y <= 0 {
		return 0, 0, false
	}
	selStart, selEnd := textView.Selection()
	caret, savedCaret, savedExactly := editorCaretSnapshot(ed)
	defer func() {
		if savedExactly {
			caret.Set(savedCaret)
		} else {
			textView.SetCaret(selStart, selEnd)
		}
	}()
	textView.MoveCoord(image.Pt(0, 0))
	start, _ = textView.Selection()
	textView.MoveCoord(image.Pt(size.X-1, size.Y-1))
	end, _ = textView.Selection()
	if end < start {
		start, end = end, start
	}
	return start, end, true
}

func editorCaretSnapshot(ed *widget.Editor) (caret, saved reflect.Value, ok bool) {
	if ed == nil {
		return reflect.Value{}, reflect.Value{}, false
	}
	editorValue := reflect.ValueOf(ed)
	if editorValue.Kind() != reflect.Pointer || editorValue.IsNil() {
		return reflect.Value{}, reflect.Value{}, false
	}
	textField := editorValue.Elem().FieldByName("text")
	if !textField.IsValid() || !textField.CanAddr() {
		return reflect.Value{}, reflect.Value{}, false
	}
	caretField := textField.FieldByName("caret")
	if !caretField.IsValid() || !caretField.CanAddr() {
		return reflect.Value{}, reflect.Value{}, false
	}
	caret = reflect.NewAt(caretField.Type(), unsafe.Pointer(caretField.UnsafeAddr())).Elem()
	saved = reflect.New(caret.Type()).Elem()
	saved.Set(caret)
	return caret, saved, true
}

func editorTextView(ed *widget.Editor) (editorTextViewAccess, bool) {
	if ed == nil || ed.SingleLine {
		return nil, false
	}
	editorValue := reflect.ValueOf(ed)
	if editorValue.Kind() != reflect.Pointer || editorValue.IsNil() {
		return nil, false
	}
	textField := editorValue.Elem().FieldByName("text")
	if !textField.IsValid() || !textField.CanAddr() {
		return nil, false
	}
	textPtr := reflect.NewAt(textField.Type(), unsafe.Pointer(textField.UnsafeAddr()))
	textView, ok := textPtr.Interface().(editorTextViewAccess)
	return textView, ok
}
