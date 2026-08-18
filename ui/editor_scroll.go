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
