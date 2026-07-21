// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"image"
	"os"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"hexone/fm"
)

func BenchmarkFileViewerTextEditFrame(b *testing.B) {
	content := strings.Repeat("2026-07-19 12:34:56 INFO request completed component=viewer elapsed=17ms status=ok\n", 5400)
	if path := os.Getenv("HEXONE_EDITOR_BENCH_FILE"); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			content = string(data)
		}
	}

	for _, tc := range []struct {
		name   string
		wrap   bool
		resize bool
	}{
		{name: "steady_unwrapped", wrap: false},
		{name: "resize_unwrapped", wrap: false, resize: true},
		{name: "steady_wrapped", wrap: true},
		{name: "resize_wrapped", wrap: true, resize: true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			st := &fileViewerState{
				mode:             "file",
				path:             "viewer-performance.log",
				editBaselineText: content,
				content:          content,
				wrapEnabled:      tc.wrap,
			}
			st.stream.SetContent(content)
			ui := NewUI(fm.DefaultConfig())
			ui.fileViewer = st
			if !ui.startFileViewerEdit(time.Now()) {
				b.Fatalf("startFileViewerEdit failed: %s", st.status)
			}
			th := material.NewTheme()
			router := new(input.Router)
			ops := new(op.Ops)
			now := time.Now()

			// Warm the editor's shaping and scrolling caches before measuring.
			for i := 0; i < 3; i++ {
				ops.Reset()
				gtx := layout.Context{
					Ops:         ops,
					Source:      router.Source(),
					Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
					Constraints: layout.Exact(image.Pt(960, 600)),
					Now:         now,
				}
				ui.layoutFileViewerTextEditor(th, gtx, st)
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(content)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				width := 960
				if tc.resize {
					width = 760 + i%201
				}
				ops.Reset()
				gtx := layout.Context{
					Ops:         ops,
					Source:      router.Source(),
					Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
					Constraints: layout.Exact(image.Pt(width, 600)),
					Now:         now.Add(time.Duration(i) * time.Millisecond),
				}
				ui.layoutFileViewerTextEditor(th, gtx, st)
			}
		})
	}
}

func BenchmarkFileViewerVirtualTextEditTyping(b *testing.B) {
	for _, lines := range []int{100, 5400, 50000} {
		b.Run(fmt.Sprintf("lines_%d", lines), func(b *testing.B) {
			content := strings.Repeat("2026-07-19 INFO component=viewer elapsed=17ms status=ok\n", lines)
			st := &fileViewerState{}
			st.initializeVirtualEditText(content)
			st.stream.wrapEnabled = true
			st.stream.prepareWrapRows(72)
			line := lines / 2
			caret := st.stream.lineByteStart(line) + len("2026-07-19")

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				st.applyVirtualEditReplacement(caret, caret, "x")
				st.applyVirtualEditReplacement(caret, caret+1, "")
			}
		})
	}
}
