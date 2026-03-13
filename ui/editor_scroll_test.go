// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"strings"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func TestEditorVerticalScrollMetricsUsesLocalAdapter(t *testing.T) {
	var ed widget.Editor
	ed.SingleLine = false
	ed.SetText(strings.Repeat("scroll line\n", 48))

	th := material.NewTheme()
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(260, 96),
		},
	}

	style := material.Editor(th, &ed, "")
	style.TextSize = unit.Sp(12)
	_ = style.Layout(gtx)

	metrics, ok := editorVerticalScrollMetrics(&ed)
	if !ok {
		t.Fatal("editorVerticalScrollMetrics should report a scrollable editor")
	}
	if metrics.Content <= metrics.Viewport {
		t.Fatalf("content=%d viewport=%d want content > viewport", metrics.Content, metrics.Viewport)
	}
	if metrics.MaxOffset <= 0 {
		t.Fatalf("maxOffset=%d want > 0", metrics.MaxOffset)
	}

	editorScrollToVerticalOffset(&ed, metrics.MaxOffset/2)

	after, ok := editorVerticalScrollMetrics(&ed)
	if !ok {
		t.Fatal("editorVerticalScrollMetrics should remain available after scroll")
	}
	if after.Offset <= 0 {
		t.Fatalf("offset=%d want > 0 after scrolling", after.Offset)
	}
}
