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
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"hexone/fm"
)

func TestHeadlessMarkdownPreviewLifecycle(t *testing.T) {
	outDir := os.Getenv("MARKDOWN_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	const width, height = 1100, 820
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("create headless window: %v", err)
	}
	defer win.Release()

	const source = `# HexOne Markdown Preview

This is a **native Gio renderer** powered by *Goldmark*. It wraps long prose without using an embedded browser and keeps [safe links](https://github.com/yuin/goldmark) visually distinct.

> Markdown preview is read-only. Press F4 to edit the actual source, then F3 to return here.

## Supported content

- [x] Headings, emphasis, links, and task lists
- [x] Nested and ordered lists
- [x] Quotes, rules, tables, and fenced code

| Mode | Key | Result |
| --- | --- | --- |
| Preview | F3 | Rendered Markdown |
| Edit | F4 | Raw editable text |

` + "```go\nfunc preview(path string) error {\n\treturn renderMarkdown(path)\n}\n```\n"

	cfg := fm.DefaultConfig()
	cfg.Viewer.HideFunctionBarWhenOpen = false
	ui := NewUI(cfg)
	ui.Tabs = widget.Enum{Value: "tab0"}
	st := &fileViewerState{
		mode:             "file",
		path:             "/tmp/README.md",
		name:             "README.md",
		content:          source,
		editableContent:  source,
		editBaselineText: source,
		status:           "ready",
	}
	st.stream.SetContent(source)
	st.contentEditor.SetText(source)
	st.markdown.setSource(st.path, source)
	ui.fileViewer = st

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	router := new(input.Router)
	render := func(name string) {
		t.Helper()
		var img *image.RGBA
		base := time.Now()
		for frame := 0; frame < 4; frame++ {
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
				t.Fatalf("render %s: %v", name, err)
			}
			img = image.NewRGBA(image.Rect(0, 0, width, height))
			if err := win.Screenshot(img); err != nil {
				t.Fatalf("capture %s: %v", name, err)
			}
		}
		path := filepath.Join(outDir, name)
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
		if err := png.Encode(file, img); err != nil {
			file.Close()
			t.Fatalf("encode %s: %v", path, err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close %s: %v", path, err)
		}
		t.Logf("wrote %s", path)
	}

	render("markdown-preview.png")
	selectionStart := f32.Pt(90, 291)
	selectionEnd := f32.Pt(316, 292)
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: selectionStart})
	render("markdown-selection-press.png")
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: selectionEnd})
	render("markdown-selection-drag.png")
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: selectionEnd})
	render("markdown-selection.png")
	selected := st.markdown.selectedText()
	if want := "- [x] Headings, emphasis, links, and task lists\n"; selected != want {
		t.Fatalf("pointer drag selected %q want individual source line %q", selected, want)
	}
	oldWriteNow := writeFileViewerClipboardNow
	copiedNow := ""
	writeFileViewerClipboardNow = func(text string) error {
		copiedNow = text
		return nil
	}
	t.Cleanup(func() { writeFileViewerClipboardNow = oldWriteNow })
	var copyOps op.Ops
	copyGTX := layout.Context{
		Ops:         &copyOps,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(width, height)),
		Now:         time.Now(),
		Source:      router.Source(),
	}
	if !ui.copyFileViewerText(copyGTX, false) {
		t.Fatal("copy selected Markdown text")
	}
	router.Frame(&copyOps)
	_, copied, ok := router.WriteClipboard()
	if !ok || copiedNow != selected || string(copied) != selected {
		t.Fatalf("clipboard sync=%q Gio=(%v,%q) selection=%q", copiedNow, ok, copied, selected)
	}

	multiStart := f32.Pt(72, 130)
	multiEnd := f32.Pt(320, 430)
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: multiStart})
	render("markdown-multiblock-move.png")
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: multiStart})
	render("markdown-multiblock-press.png")
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: multiEnd})
	render("markdown-multiblock-drag.png")
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: multiEnd})
	render("markdown-multiblock-selection.png")
	selected = st.markdown.selectedText()
	if !st.markdown.blockSelection {
		t.Fatal("cross-block pointer drag did not switch to Markdown document selection")
	}
	for _, syntax := range []string{"**native Gio renderer**", "> Markdown preview", "## Supported content", "- [x] Headings", "| Mode | Key | Result |"} {
		if !strings.Contains(selected, syntax) {
			t.Fatalf("cross-block selection %q does not preserve %q", selected, syntax)
		}
	}
	if !ui.copyFileViewerText(copyGTX, false) {
		t.Fatal("copy cross-block Markdown source")
	}
	router.Frame(&copyOps)
	_, copied, ok = router.WriteClipboard()
	if !ok || copiedNow != selected || string(copied) != selected {
		t.Fatalf("cross-block clipboard sync=%q Gio=(%v,%q) selection=%q", copiedNow, ok, copied, selected)
	}

	contextPos := f32.Pt(520, 405)
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: contextPos})
	render("markdown-context-move.png")
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonSecondary, Position: contextPos})
	render("markdown-context-menu.png")
	if !st.menuOpen || st.menuRect.Empty() {
		t.Fatalf("Markdown secondary click did not open context menu: open=%v rect=%v", st.menuOpen, st.menuRect)
	}
	rows := fileViewerContextMenuRows(st)
	if len(rows) != 1 || rows[0].item.Label != "Copy" {
		t.Fatalf("Markdown context menu rows=%#v want Copy only", rows)
	}
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: contextPos})
	render("markdown-context-release.png")
	copyPoint := f32.Pt(float32(st.menuRect.Min.X+st.menuRect.Dx()/2), float32(st.menuRect.Min.Y+st.menuRect.Dy()/2))
	// menuRect is local to the file-viewer surface; queued pointer positions
	// include the 24px function bar above it in this headless configuration.
	copyPoint.Y += 24
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: copyPoint})
	render("markdown-context-copy-hover.png")
	copiedNow = ""
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: copyPoint})
	render("markdown-context-copy-press.png")
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: copyPoint})
	render("markdown-context-copy.png")
	if st.menuOpen || copiedNow != selected {
		t.Fatalf("Markdown context Copy result: menuOpen=%v copied=%q want %q", st.menuOpen, copiedNow, selected)
	}

	if !ui.startFileViewerEdit(time.Now()) {
		t.Fatalf("enter raw edit mode: %s", st.status)
	}
	render("markdown-edit.png")
	st.contentEditor.SetText("# Edited preview\n\nThe unsaved raw buffer is reparsed when F3 returns to preview.\n")
	ui.syncFileViewerTextEdit(st)
	if !ui.performFunctionBarAction(functionBarActionView, time.Now()) {
		t.Fatal("return to preview")
	}
	render("markdown-edited-preview.png")

	var longSource strings.Builder
	longSource.WriteString("# Auto-scroll selection\n\n")
	for index := 1; index <= 80; index++ {
		fmt.Fprintf(&longSource, "Paragraph %02d with **original Markdown** selection text.\n\n", index)
	}
	longText := longSource.String()
	st.content = longText
	st.editableContent = longText
	st.markdown.setSource(st.path, longText)
	render("markdown-autoscroll-start.png")
	autoStart := f32.Pt(90, 90)
	autoEnd := f32.Pt(320, height+120)
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: autoStart})
	render("markdown-autoscroll-move.png")
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: autoStart})
	render("markdown-autoscroll-press.png")
	beforeScroll := st.markdown.list.Position
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: autoEnd})
	render("markdown-autoscroll-drag.png")
	afterScroll := st.markdown.list.Position
	if !st.markdown.blockSelection || afterScroll == beforeScroll || st.markdown.selectionHead <= st.markdown.selectionAnchor {
		t.Fatalf("edge drag did not continuously extend and scroll: before=%+v after=%+v selection=%d..%d block=%v",
			beforeScroll, afterScroll, st.markdown.selectionAnchor, st.markdown.selectionHead, st.markdown.blockSelection)
	}
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: autoEnd})
	render("markdown-autoscroll-selection.png")

	const quoteSource = "> Laikai iš nupirktų bilietų pažymėti **paryškintai**.  \n" +
		"> Atstumai tarp viešbučių ir stočių yra apytiksliai.  \n" +
		"> Traukinių peronus ir galimus tvarkaraščio pakeitimus patikrinti kelionės išvakarėse ir dar kartą kelionės dieną.\n"
	st.content = quoteSource
	st.editableContent = quoteSource
	st.markdown.setSource(st.path, quoteSource)
	st.markdown.list.Position = layout.Position{}
	render("markdown-quote-source.png")
	if len(st.markdown.visibleBlockRects) != 1 || len(st.markdown.blockLineWeights) != 1 || len(st.markdown.blockLineWeights[0]) != 3 {
		t.Fatalf("quote selection geometry: rects=%v weights=%v", st.markdown.visibleBlockRects, st.markdown.blockLineWeights)
	}
	quoteRect := st.markdown.visibleBlockRects[0].contentRect
	quoteWeights := st.markdown.blockLineWeights[0]
	_, quoteTotalWeight := markdownSelectionWeightOffset(quoteWeights, len(quoteWeights))
	startLocalY := quoteRect.Min.Y + quoteRect.Dy()*(quoteWeights[0]/2)/quoteTotalWeight
	secondCenterWeight := quoteWeights[0] + quoteWeights[1]/2
	endLocalY := quoteRect.Min.Y + quoteRect.Dy()*secondCenterWeight/quoteTotalWeight
	// Pointer input is in window coordinates; this configuration has the
	// function bar and viewer header 58px above the Markdown viewport.
	quoteStart := f32.Pt(100, float32(startLocalY+58))
	quoteEnd := f32.Pt(320, float32(endLocalY+58))
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: quoteStart})
	render("markdown-quote-move.png")
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: quoteStart})
	render("markdown-quote-press.png")
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: quoteEnd})
	render("markdown-quote-two-lines-drag.png")
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: quoteEnd})
	render("markdown-quote-two-lines.png")
	wantQuote := "> Laikai iš nupirktų bilietų pažymėti **paryškintai**.  \n" +
		"> Atstumai tarp viešbučių ir stočių yra apytiksliai.  \n"
	if got := st.markdown.selectedText(); got != wantQuote {
		t.Fatalf("two-line quote selection=%q want %q; weights=%v", got, wantQuote, quoteWeights)
	}
}
