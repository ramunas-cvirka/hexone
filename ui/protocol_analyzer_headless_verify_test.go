// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build uiverify

package ui

import (
	"image"
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
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"hexone/fm"
)

func TestHeadlessProtocolAnalyzer(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	const width, height = 1200, 640
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("create headless window: %v", err)
	}
	defer win.Release()

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs.Value = "tab2"
	router := new(input.Router)
	focusInput := false
	clearInputFocus := false

	render := func() *image.RGBA {
		var img *image.RGBA
		base := time.Now()
		for frame := 0; frame < 4; frame++ {
			var ops op.Ops
			gtx := layout.Context{
				Ops:         &ops,
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Constraints: layout.Exact(image.Pt(width, height)),
				Now:         base.Add(time.Duration(frame) * 50 * time.Millisecond),
				Source:      router.Source(),
			}
			if focusInput {
				gtx.Execute(key.FocusCmd{Tag: &ui.tab2State.hexEd})
			} else if clearInputFocus {
				gtx.Execute(key.FocusCmd{})
			}
			ui.Layout(th, gtx)
			router.Frame(&ops)
			if err := win.Frame(&ops); err != nil {
				t.Fatalf("render frame: %v", err)
			}
			img = image.NewRGBA(image.Rect(0, 0, width, height))
			if err := win.Screenshot(img); err != nil {
				t.Fatalf("capture frame: %v", err)
			}
		}
		return img
	}

	writePNG := func(name string, img *image.RGBA) {
		path := filepath.Join(outDir, name)
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create screenshot: %v", err)
		}
		if err := png.Encode(file, img); err != nil {
			file.Close()
			t.Fatalf("encode screenshot: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close screenshot: %v", err)
		}
		t.Logf("wrote %s", path)
	}

	ui.tab2State.hexEd.SetText("78 78 0D 01 03 58 24 00 01 00 05 9C 1A 0D 0A")
	writePNG("protocol-analyzer-error.png", render())
	focusInput = true
	writePNG("protocol-analyzer-input-focused.png", render())
	focusInput = false
	clearInputFocus = true
	render()
	clearInputFocus = false

	ui.tab2State.hexEd.SetText("78 78 1F 12 0B 08 1D 11 2E 10 CC 02 7A C7 EB 0C 46 58 49 00 14 8F 01 CC 00 28 7D 00 1F B8 00 03 80 81 0D 0A")
	render()
	for _, row := range analyzerLeafRows(ui.tab2State) {
		if row.Span.Name != "serial" {
			continue
		}
		ui.tab2State.selectedSpanKey = rangeKey(row.Span.Start, row.Span.End)
		ui.tab2State.selectedRowID = row.Key
		ui.tab2State.selectedHint = row.Span
		break
	}
	writePNG("protocol-analyzer-decoded.png", render())

	ui.tab2State.protoDropOpen = true
	ui.tab2State.protoDropOpenedAt = time.Now().Add(-time.Second)
	writePNG("protocol-analyzer-dropdown.png", render())
	ui.tab2State.protoDropOpen = false

	ui.tab2State.selectedSpanKey = ""
	ui.tab2State.selectedRowID = ""
	ui.tab2State.selectedHint = nil
	ui.tab2State.scrollList.Position = layout.Position{}
	ui.tab2State.hexEd.SetText(strings.Repeat("00 ", 512))
	writePNG("protocol-analyzer-large-byte-map.png", render())
	if ui.tab2State.scrollList.Position.Length <= height {
		t.Fatalf("large byte map content height=%d want greater than viewport %d", ui.tab2State.scrollList.Position.Length, height)
	}
	beforeScroll := ui.tab2State.scrollList.Position
	var scrolledImage *image.RGBA
	for range 4 {
		router.Queue(pointer.Event{
			Kind:     pointer.Scroll,
			Source:   pointer.Mouse,
			Position: f32.Pt(width/2, height/2),
			Scroll:   f32.Pt(0, 80),
		})
		scrolledImage = render()
	}
	afterScroll := ui.tab2State.scrollList.Position
	if beforeScroll.First == afterScroll.First && beforeScroll.Offset == afterScroll.Offset {
		t.Fatal("large byte map wheel scroll did not move analyzer content")
	}
	writePNG("protocol-analyzer-large-byte-map-scrolled.png", scrolledImage)

	const reducedHeight = 360
	reducedWin, err := headless.NewWindow(width, reducedHeight)
	if err != nil {
		t.Fatalf("create reduced-height headless window: %v", err)
	}
	defer reducedWin.Release()
	ui.tab2State.scrollList.Position = layout.Position{}
	ui.tab2State.hexEd.SetText("78 78 1F 12 0B 08 1D 11 2E 10 CC 02 7A C7 EB 0C 46 58 49 00 14 8F 01 CC 00 28 7D 00 1F B8 00 03 80 81 0D 0A")
	renderReduced := func() *image.RGBA {
		var img *image.RGBA
		base := time.Now()
		for frame := 0; frame < 4; frame++ {
			var ops op.Ops
			gtx := layout.Context{
				Ops:         &ops,
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Constraints: layout.Exact(image.Pt(width, reducedHeight)),
				Now:         base.Add(time.Duration(frame) * 50 * time.Millisecond),
				Source:      router.Source(),
			}
			ui.Layout(th, gtx)
			router.Frame(&ops)
			if err := reducedWin.Frame(&ops); err != nil {
				t.Fatalf("render reduced-height frame: %v", err)
			}
			img = image.NewRGBA(image.Rect(0, 0, width, reducedHeight))
			if err := reducedWin.Screenshot(img); err != nil {
				t.Fatalf("capture reduced-height frame: %v", err)
			}
		}
		return img
	}
	writePNG("protocol-analyzer-reduced-height.png", renderReduced())
	if ui.tab2State.scrollList.Position.Length <= reducedHeight {
		t.Fatalf("reduced-height content height=%d want greater than viewport %d", ui.tab2State.scrollList.Position.Length, reducedHeight)
	}
}
