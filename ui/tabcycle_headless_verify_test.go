// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build uiverify

package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
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

// TestHeadlessTabIndentedViewEditCycle drives the real UI through
// view -> edit -> view on a tab-indented JSON file, which is the exact
// reproduction reported for ~/.docker/config.json.
func TestHeadlessTabIndentedViewEditCycle(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	const width, height = 900, 420
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("create headless window: %v", err)
	}
	defer win.Release()

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	cfg := fm.DefaultConfig()
	cfg.Viewer.HideFunctionBarWhenOpen = false
	ui := NewUI(cfg)
	router := new(input.Router)
	base := time.Now()
	frameNo := 0
	render := func() *image.RGBA {
		var img *image.RGBA
		for frame := 0; frame < 6; frame++ {
			frameNo++
			var ops op.Ops
			gtx := layout.Context{
				Ops:         &ops,
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Constraints: layout.Exact(image.Pt(width, height)),
				Now:         base.Add(time.Duration(frameNo) * 60 * time.Millisecond),
				Source:      router.Source(),
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

	viewerDir := t.TempDir()
	viewerPath := filepath.Join(viewerDir, "config.json")
	viewerContent := "{\n\t\"auths\": {},\n\t\"credsStore\": \"desktop\",\n\t\"currentContext\": \"desktop-linux\",\n\t\"cliPluginsExtraDirs\": [\n\t\t\"/opt/homebrew/lib/docker/cli-plugins\"\n\t]\n}"
	if err := os.WriteFile(viewerPath, []byte(viewerContent), 0o644); err != nil {
		t.Fatalf("write viewer fixture: %v", err)
	}
	if !ui.requestPaneLoadWithSelection(0, viewerDir, viewerPath, "", 0) {
		t.Fatal("request viewer fixture directory")
	}
	waitFor := func(label string, ready func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !ready() && time.Now().Before(deadline) {
			render()
			time.Sleep(5 * time.Millisecond)
		}
		if !ready() {
			t.Fatalf("timed out waiting for %s", label)
		}
	}
	waitFor("fixture selection", func() bool {
		pane := ui.filePanes[0]
		entry := pane.selectedEntry()
		return !pane.loading && entry != nil && entry.Path == viewerPath
	})
	ui.startFileViewer(0, time.Now())
	waitFor("viewer load", func() bool {
		return ui.fileViewer != nil && !ui.fileViewer.loading
	})
	waitFor("viewer syntax", func() bool {
		return ui.fileViewer.stream.syntax.ready()
	})
	dumpState := func(label string) {
		st := ui.fileViewer
		t.Logf("--- %s: editMode=%v indentStyle=%q tabSize=%d maxCols=%d syntaxReady=%v",
			label, st.editMode, st.editIndentStyle, st.editTabSize, st.stream.maxCols, st.stream.syntax.ready())
		for i, line := range st.stream.lines {
			spans, _ := st.stream.syntaxLine(i)
			fits := viewerSyntaxSpansFitLine(line, spans)
			t.Logf("    line[%d]=%q len=%d spans=%d fits=%v", i, line, len(line), len(spans), fits)
			for _, s := range spans {
				t.Logf("        span bytes=[%d:%d] cols=[%d:%d] role=%v text=%q",
					s.byteStart, s.byteEnd, s.colStart, s.colEnd, s.role, safeSlice(line, s.byteStart, s.byteEnd))
			}
		}
	}
	dumpState("view")
	writePNG("tabcycle-1-view.png", render())

	router.Queue(key.Event{Name: key.NameF4, State: key.Press})
	editImg := render()
	if !ui.fileViewer.editMode {
		t.Fatalf("F4 did not start viewer edit: %s", ui.fileViewer.status)
	}
	// let the edit-mode syntax rebuild land
	waitFor("edit syntax", func() bool {
		return ui.fileViewer.editSyntax.ready()
	})
	editImg = render()
	dumpState("edit")
	writePNG("tabcycle-2-edit.png", editImg)

	// Click inside the tab-indented run of line 6, then shift-click further along
	// it, to exercise column -> byte hit testing and the selection rectangle.
	st := ui.fileViewer
	v := &st.stream
	lineY := v.textRect.Min.Y + 5*v.lineH + v.lineH/2
	clickAt := func(x int, mods key.Modifiers) {
		pos := f32.Pt(float32(v.textRect.Min.X+v.textPad+x), float32(lineY))
		router.Queue(
			pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pos, Modifiers: mods},
			pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pos, Modifiers: mods},
		)
		render()
	}
	clickAt(v.colOffsetPx(6), 0)
	caretLine, caretLocal, _ := v.lineForOffset(v.selHead)
	t.Logf("click at column 6 -> line=%d local=%d col=%d", caretLine, caretLocal, v.colAtByte(v.lines[caretLine], caretLocal))
	var from, to int
	var ok bool

	// Select the key on the caret's line so the selection rectangle is drawn
	// over columns that sit behind a tab.
	selLine := caretLine
	selStart := v.lineByteStart(selLine) + 1
	v.beginSelection(selStart)
	v.selHead = selStart + 12
	v.updateSelectionRange()
	from, to, ok = v.selectionColsForLine(selLine)
	t.Logf("selection on line %d: cols=[%d:%d] ok=%v", selLine, from, to, ok)
	writePNG("tabcycle-4-edit-selection.png", render())

	router.Queue(key.Event{Name: key.NameF3, State: key.Press})
	backImg := render()
	if ui.fileViewer.editMode {
		t.Fatalf("F3 did not leave viewer edit: %s", ui.fileViewer.status)
	}
	dumpState("view-after-edit")
	writePNG("tabcycle-3-view-after-edit.png", backImg)

	// Find highlighting maps byte offsets onto display columns too.
	ui.openFileViewerFind(time.Now())
	ui.fileViewer.find.editor.SetText("desktop")
	ui.refreshFileViewerFind(time.Now(), true)
	render()
	writePNG("tabcycle-6-view-find.png", render())
	t.Logf("find matches=%d valid=%v", len(ui.fileViewer.find.matches), ui.fileViewer.find.currentValid)
}

func safeSlice(s string, from, to int) string {
	if from < 0 || to > len(s) || to <= from {
		return "<OUT OF RANGE>"
	}
	return s[from:to]
}

// TestHeadlessTabIndentedNarrowHScroll renders a tab-indented file in a window
// too narrow to fit it, so the horizontal offset has to cut into the expanded
// tabs of the deepest line.
func TestHeadlessTabIndentedNarrowHScroll(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	const width, height = 420, 260
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("create headless window: %v", err)
	}
	defer win.Release()

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	cfg := fm.DefaultConfig()
	cfg.Viewer.HideFunctionBarWhenOpen = false
	ui := NewUI(cfg)
	router := new(input.Router)
	base := time.Now()
	frameNo := 0
	render := func() *image.RGBA {
		var img *image.RGBA
		for frame := 0; frame < 6; frame++ {
			frameNo++
			var ops op.Ops
			gtx := layout.Context{
				Ops:         &ops,
				Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Constraints: layout.Exact(image.Pt(width, height)),
				Now:         base.Add(time.Duration(frameNo) * 60 * time.Millisecond),
				Source:      router.Source(),
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

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := "{\n\t\"auths\": {},\n\t\"cliPluginsExtraDirs\": [\n\t\t\"/opt/homebrew/lib/docker/cli-plugins\"\n\t]\n}"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if !ui.requestPaneLoadWithSelection(0, dir, path, "", 0) {
		t.Fatal("request fixture directory")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pane := ui.filePanes[0]
		if entry := pane.selectedEntry(); !pane.loading && entry != nil && entry.Path == path {
			break
		}
		render()
		time.Sleep(5 * time.Millisecond)
	}
	ui.startFileViewer(0, time.Now())
	for time.Now().Before(deadline) && (ui.fileViewer == nil || ui.fileViewer.loading) {
		render()
		time.Sleep(5 * time.Millisecond)
	}
	st := ui.fileViewer
	v := &st.stream

	setHScroll := func(col int) int {
		v.hCol = col
		render()
		return v.hCol
	}
	setHScroll(0)
	writePNG("narrow-1-view-hcol0.png", render())
	t.Logf("view hCol=%d maxCols=%d visibleCols=%d", setHScroll(6), v.maxCols, v.visibleCols(v.textRect.Dx()))
	writePNG("narrow-2-view-hcol6.png", render())

	setHScroll(0)
	router.Queue(key.Event{Name: key.NameF4, State: key.Press})
	render()
	if !st.editMode {
		t.Fatalf("F4 did not start edit: %s", st.status)
	}
	writePNG("narrow-3-edit-hcol0.png", render())
	t.Logf("edit hCol=%d maxCols=%d", setHScroll(6), v.maxCols)
	writePNG("narrow-4-edit-hcol6.png", render())
}
