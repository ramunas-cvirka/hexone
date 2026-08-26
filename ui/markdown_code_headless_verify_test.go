// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build markdownverify

package ui

import (
	"fmt"
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
	"gioui.org/widget"
	"gioui.org/widget/material"

	"hexone/fm"
)

// Mirrors the supported-trackers README section where overflowing fenced code
// lines showed a black bar over the last code line (the unthemed Overlay
// scrollbar of the horizontal code list).
const markdownCodeOverflowSource = "Fetchers can also be run manually and write source snapshots into `data/imports/`:\n" +
	"\n" +
	"```sh\n" +
	"python3 supported-trackers/scripts/fetch_flespi.py --out supported-trackers/data/imports/flespi.json\n" +
	"python3 supported-trackers/scripts/fetch_wialon.py --out supported-trackers/data/imports/wialon.json\n" +
	"python3 supported-trackers/scripts/fetch_gpswox.py --out supported-trackers/data/imports/gpswox-public.json\n" +
	"```\n" +
	"\n" +
	"For dynamic pages or pages you opened in a browser, save the HTML and pass it explicitly:\n" +
	"\n" +
	"```sh\n" +
	"python3 supported-trackers/scripts/fetch_flespi.py --html /tmp/flespi-devices.html --out supported-trackers/data/imports/flespi.json\n" +
	"```\n" +
	"\n" +
	"The main script replaces the old multi-step flow below:\n" +
	"\n" +
	"```sh\n" +
	"python3 supported-trackers/scripts/seed_known_from_parity.py --out supported-trackers/data/imports/gpstrack-parity.json\n" +
	"python3 supported-trackers/scripts/merge_catalog.py\n" +
	"python3 supported-trackers/scripts/report_gaps.py --catalog supported-trackers/data/generated/catalog.json\n" +
	"```\n"

func TestHeadlessMarkdownCodeBlocks(t *testing.T) {
	outDir := os.Getenv("MARKDOWN_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	const width, height = 950, 800
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("create headless window: %v", err)
	}
	defer win.Release()

	source := markdownCodeOverflowSource
	path := "/tmp/README.md"
	if file := os.Getenv("MARKDOWN_VERIFY_FILE"); file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read MARKDOWN_VERIFY_FILE: %v", err)
		}
		source = string(data)
		path = file
	}

	cfg := fm.DefaultConfig()
	cfg.Viewer.HideFunctionBarWhenOpen = false
	ui := NewUI(cfg)
	ui.Tabs = widget.Enum{Value: "tab0"}
	st := &fileViewerState{
		mode:             "file",
		path:             path,
		name:             filepath.Base(path),
		content:          source,
		editableContent:  source,
		editBaselineText: source,
		status:           "ready",
	}
	st.stream.SetContent(source)
	st.contentEditor.SetText(source)
	st.markdown.setSource(st.path, source)
	ui.fileViewer = st

	pageDowns := 0
	if raw := os.Getenv("MARKDOWN_VERIFY_PAGEDOWN"); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &pageDowns); err != nil {
			t.Fatalf("parse MARKDOWN_VERIFY_PAGEDOWN: %v", err)
		}
	}
	if os.Getenv("MARKDOWN_VERIFY_TERMINAL") != "" && !ui.toggleTerminal() {
		t.Fatal("open terminal")
	}

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	router := new(input.Router)
	var img *image.RGBA
	base := time.Now()
	frames := 4 + pageDowns
	for frame := 0; frame < frames; frame++ {
		if frame >= 4 {
			st.markdown.scrollByKey(key.NamePageDown)
		}
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)),
			Now:         base.Add(time.Duration(frame) * 100 * time.Millisecond),
			Source:      router.Source(),
		}
		ui.Layout(th, gtx)
		router.Frame(&ops)
		if err := win.Frame(&ops); err != nil {
			t.Fatalf("render frame %d: %v", frame, err)
		}
		img = image.NewRGBA(image.Rect(0, 0, width, height))
		if err := win.Screenshot(img); err != nil {
			t.Fatalf("capture frame %d: %v", frame, err)
		}
	}

	outPath := filepath.Join(outDir, "markdown-code-blocks.png")
	file, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create %s: %v", outPath, err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatalf("encode %s: %v", outPath, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close %s: %v", outPath, err)
	}
	t.Logf("wrote %s", outPath)

	for _, visible := range st.markdown.visibleBlockRects {
		block := st.markdown.doc.blocks[visible.index]
		t.Logf("block %d kind=%d lang=%q rect=%v content=%v", visible.index, block.kind, block.language, visible.rect, visible.contentRect)
	}
}

// markdownCodeHarness renders the Markdown preview headlessly and exposes the
// helpers the scrollbar/selection drive tests need.
type markdownCodeHarness struct {
	t       *testing.T
	ui      *UI
	viewer  *fileViewerState
	th      *material.Theme
	router  *input.Router
	win     *headless.Window
	width   int
	height  int
	outDir  string
	base    time.Time
	elapsed time.Duration
	img     *image.RGBA
}

func newMarkdownCodeHarness(t *testing.T, source string, width, height int) *markdownCodeHarness {
	t.Helper()
	outDir := os.Getenv("MARKDOWN_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("create headless window: %v", err)
	}
	t.Cleanup(win.Release)

	cfg := fm.DefaultConfig()
	cfg.Viewer.HideFunctionBarWhenOpen = false
	ui := NewUI(cfg)
	ui.Tabs = widget.Enum{Value: "tab0"}
	viewer := &fileViewerState{
		mode:             "file",
		path:             "/tmp/README.md",
		name:             "README.md",
		content:          source,
		editableContent:  source,
		editBaselineText: source,
		status:           "ready",
	}
	viewer.stream.SetContent(source)
	viewer.contentEditor.SetText(source)
	viewer.markdown.setSource(viewer.path, source)
	ui.fileViewer = viewer

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	return &markdownCodeHarness{
		t: t, ui: ui, viewer: viewer, th: th, router: new(input.Router),
		win: win, width: width, height: height, outDir: outDir, base: time.Now(),
	}
}

func (h *markdownCodeHarness) frame() {
	h.t.Helper()
	var ops op.Ops
	h.elapsed += 100 * time.Millisecond
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(h.width, h.height)),
		Now:         h.base.Add(h.elapsed),
		Source:      h.router.Source(),
	}
	h.ui.Layout(h.th, gtx)
	h.router.Frame(&ops)
	if err := h.win.Frame(&ops); err != nil {
		h.t.Fatalf("render frame: %v", err)
	}
	h.img = image.NewRGBA(image.Rect(0, 0, h.width, h.height))
	if err := h.win.Screenshot(h.img); err != nil {
		h.t.Fatalf("capture frame: %v", err)
	}
}

func (h *markdownCodeHarness) render(name string) {
	h.t.Helper()
	for range 4 {
		h.frame()
	}
	if name == "" {
		return
	}
	path := filepath.Join(h.outDir, name)
	file, err := os.Create(path)
	if err != nil {
		h.t.Fatalf("create %s: %v", path, err)
	}
	if err := png.Encode(file, h.img); err != nil {
		file.Close()
		h.t.Fatalf("encode %s: %v", path, err)
	}
	if err := file.Close(); err != nil {
		h.t.Fatalf("close %s: %v", path, err)
	}
	h.t.Logf("wrote %s", path)
}

// viewportOffset reports the window-coordinate origin of the Markdown preview
// viewport, so pointer events can be queued in window coordinates while block
// geometry stays viewport-local. Gio delivers pointer positions in the
// handler's local space, so the offset has to be probed rather than assumed —
// it changes with the function bar, the viewer header, and the terminal.
func (h *markdownCodeHarness) viewportOffset() image.Point {
	h.t.Helper()
	state := &h.viewer.markdown
	if len(state.visibleBlockRects) == 0 {
		h.t.Fatal("no visible Markdown blocks; render first")
	}
	if state.list.Position.First != 0 || state.list.Position.Offset != 0 {
		h.t.Fatalf("viewport probe needs the preview scrolled to the top: %+v", state.list.Position)
	}
	// A press only registers at or below the viewport origin, and the first
	// block starts at local y 0 while scrolled to the top, so the smallest
	// window y that registers a press is the origin itself.
	high := -1
	for _, candidate := range []int{h.height / 2, h.height / 3, h.height / 4, 120, 90, 70} {
		if h.pressRegisters(candidate) {
			high = candidate
			break
		}
	}
	if high < 0 {
		h.t.Fatal("no probe point reached the Markdown viewport")
	}
	low := 0
	for low < high {
		mid := (low + high) / 2
		if h.pressRegisters(mid) {
			high = mid
		} else {
			low = mid + 1
		}
	}
	h.clearSelection()
	return image.Pt(0, low)
}

func (h *markdownCodeHarness) pressRegisters(y int) bool {
	h.t.Helper()
	h.clearSelection()
	position := f32.Pt(60, float32(y))
	h.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse,
		Buttons: pointer.ButtonPrimary, Position: position})
	h.frame()
	registered := h.viewer.markdown.selectingBlocks
	h.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: position})
	h.frame()
	return registered
}

func (h *markdownCodeHarness) clearSelection() {
	state := &h.viewer.markdown
	state.stopBlockSelectionDrag()
	state.blockSelection = false
	state.lineSelection = false
	state.selectAll = false
	state.selectionAnchor, state.selectionAnchorLine = -1, -1
	state.selectionHead, state.selectionHeadLine = -1, -1
}

func (h *markdownCodeHarness) blockRect(index int) markdownVisibleBlock {
	h.t.Helper()
	for _, visible := range h.viewer.markdown.visibleBlockRects {
		if visible.index == index {
			return visible
		}
	}
	h.t.Fatalf("block %d is not visible: %+v", index, h.viewer.markdown.visibleBlockRects)
	return markdownVisibleBlock{}
}

// codeScrollbarPoint returns the window-coordinate centre of a code block's
// horizontal scrollbar strip, which Occupy reserves at the bottom of the
// block's padded content box.
func (h *markdownCodeHarness) codeScrollbarPoint(index int, offset image.Point) f32.Point {
	h.t.Helper()
	visible := h.blockRect(index)
	style := h.ui.markdownListStyle(h.th, &widget.List{})
	gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	barWidth := gtx.Dp(style.Width())
	stripBottom := visible.contentRect.Max.Y - gtx.Dp(markdownSpaceMD)
	y := stripBottom - barWidth/2
	x := visible.contentRect.Min.X + visible.contentRect.Dx()/2
	return f32.Pt(float32(x+offset.X), float32(y+offset.Y))
}

func TestHeadlessMarkdownCodeScrollbarDragScrollsWithoutSelecting(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		name := "terminal-closed"
		if terminal {
			name = "terminal-open"
		}
		t.Run(name, func(t *testing.T) {
			h := newMarkdownCodeHarness(t, markdownCodeOverflowSource, 950, 800)
			if terminal {
				if !h.ui.toggleTerminal() {
					t.Fatal("open terminal")
				}
			}
			h.render("")
			offset := h.viewportOffset()
			t.Logf("%s: markdown viewport origin=%v", name, offset)

			// Block 3 is the single-line overflowing fenced block.
			state := &h.viewer.markdown
			if got := state.doc.blocks[3].kind; got != markdownBlockCode {
				t.Fatalf("block 3 kind=%d want code", got)
			}
			codeList := h.ui.markdownHorizontalList(state, state.doc.blocks[3].id, false)
			before := codeList.Position
			point := h.codeScrollbarPoint(3, offset)
			t.Logf("%s: scrollbar point=%v content=%v", name, point, h.blockRect(3).contentRect)

			bar := func(stage string) {
				t.Logf("%s/%s: dragging=%v track=%v indicator=%v distance=%v position=%+v",
					name, stage, codeList.Scrollbar.Dragging(), codeList.Scrollbar.TrackHovered(),
					codeList.Scrollbar.IndicatorHovered(), codeList.Scrollbar.ScrollDistance(), codeList.Position)
			}
			h.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: point})
			h.frame()
			bar("after-move")
			h.render("markdown-code-scrollbar-hover-" + name + ".png")
			h.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse,
				Buttons: pointer.ButtonPrimary, Position: point})
			h.frame()
			bar("after-press")
			// A real mouse drag arrives as a stream of moves; gio's scrollbar
			// only accumulates travel from the second one onward.
			dragged := point
			for step := 0; step < 4; step++ {
				dragged.X += 65
				h.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse,
					Buttons: pointer.ButtonPrimary, Position: dragged})
				h.frame()
				bar(fmt.Sprintf("drag-step-%d", step))
			}
			h.render("markdown-code-scrollbar-drag-" + name + ".png")
			after := codeList.Position
			selection := state.selectedText()
			h.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: dragged})
			h.render("")

			t.Logf("%s: code list before=%+v after=%+v", name, before, after)
			t.Logf("%s: blockSelection=%v lineSelection=%v selection=%q",
				name, state.blockSelection, state.lineSelection, selection)
			if after.Offset <= before.Offset {
				t.Errorf("%s: dragging the code scrollbar did not scroll the code: before=%+v after=%+v",
					name, before, after)
			}
			if selection != "" {
				t.Errorf("%s: dragging the code scrollbar selected text %q", name, selection)
			}
		})
	}
}

// codeLinePoint returns the window-coordinate centre of a rendered code line.
func (h *markdownCodeHarness) codeLinePoint(index, line int, offset image.Point) f32.Point {
	h.t.Helper()
	visible := h.blockRect(index)
	block := h.viewer.markdown.doc.blocks[index]
	metrics, ok := h.viewer.markdown.codeMetrics[block.id]
	if !ok || metrics.lineHeight <= 0 {
		h.t.Fatalf("block %d has no recorded code metrics: %+v", index, metrics)
	}
	y := visible.contentRect.Min.Y + metrics.padTop + line*metrics.lineHeight + metrics.lineHeight/2
	x := visible.contentRect.Min.X + visible.contentRect.Dx()/2
	return f32.Pt(float32(x+offset.X), float32(y+offset.Y))
}

func TestHeadlessMarkdownCodeClickSelectsTheLineUnderThePointer(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		name := "terminal-closed"
		if terminal {
			name = "terminal-open"
		}
		t.Run(name, func(t *testing.T) {
			h := newMarkdownCodeHarness(t, markdownCodeOverflowSource, 950, 800)
			if terminal {
				if !h.ui.toggleTerminal() {
					t.Fatal("open terminal")
				}
			}
			h.render("")
			offset := h.viewportOffset()
			state := &h.viewer.markdown

			// Block 1 is the three-line fenced block; every rendered row must
			// resolve to its own source line.
			want := []string{
				"```sh\npython3 supported-trackers/scripts/fetch_flespi.py --out supported-trackers/data/imports/flespi.json\n",
				"python3 supported-trackers/scripts/fetch_wialon.py --out supported-trackers/data/imports/wialon.json\n",
				"python3 supported-trackers/scripts/fetch_gpswox.py --out supported-trackers/data/imports/gpswox-public.json\n```\n",
			}
			if got := len(state.blockSourceLines(1)); got != len(want) {
				t.Fatalf("%s: block 1 selectable lines=%d want %d", name, got, len(want))
			}
			for line := range want {
				h.clearSelection()
				point := h.codeLinePoint(1, line, offset)
				h.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: point})
				h.frame()
				h.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse,
					Buttons: pointer.ButtonPrimary, Position: point})
				h.frame()
				nudged := point
				nudged.X += 4
				h.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse,
					Buttons: pointer.ButtonPrimary, Position: nudged})
				h.render(fmt.Sprintf("markdown-code-line-%d-%s.png", line, name))
				got := state.selectedText()
				h.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: nudged})
				h.frame()
				t.Logf("%s: click on rendered line %d at %v selected %q", name, line, point, got)
				if got != want[line] {
					t.Errorf("%s: click on rendered code line %d selected %q want %q", name, line, got, want[line])
				}
			}
		})
	}
}
