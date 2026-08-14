// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"hexone/fm"
)

func TestViewerPathLooksMarkdown(t *testing.T) {
	tests := map[string]bool{
		"README.md":         true,
		"guide.MARKDOWN":    true,
		"notes.mdown":       true,
		"legacy.mkdn":       true,
		"readme.mdtext":     true,
		"markdown.txt":      false,
		"README":            false,
		"archive.md.backup": false,
	}
	for path, want := range tests {
		if got := viewerPathLooksMarkdown(path); got != want {
			t.Errorf("viewerPathLooksMarkdown(%q)=%v want %v", path, got, want)
		}
	}
}

func TestParseMarkdownDocumentSupportsGFMBlocksAndInlineStyles(t *testing.T) {
	source := []byte("# Project\n\nParagraph with *italic*, **bold**, ~~gone~~, `code`, and [docs](https://example.com).\n\n> quoted **text**\n\n- [x] shipped\n- [ ] pending\n\n| Name | Value |\n| --- | ---: |\n| alpha | 1 |\n\n```go\nfmt.Println(\"ok\")\n```\n")
	doc := parseMarkdownDocument(source)

	var heading, quote, list, table, code bool
	var allInline []markdownInline
	var walk func([]markdownBlock)
	walk = func(blocks []markdownBlock) {
		for _, block := range blocks {
			switch block.kind {
			case markdownBlockHeading:
				heading = block.level == 1 && markdownInlineText(block.inlines) == "Project"
			case markdownBlockQuote:
				quote = true
			case markdownBlockList:
				list = len(block.children) == 2
			case markdownBlockTable:
				table = len(block.rows) == 2 && block.rows[0].header && len(block.rows[1].cells) == 2
			case markdownBlockCode:
				code = block.language == "go" && strings.Contains(block.text, `fmt.Println("ok")`)
			}
			allInline = append(allInline, block.inlines...)
			for _, row := range block.rows {
				for _, cell := range row.cells {
					allInline = append(allInline, cell.inlines...)
				}
			}
			walk(block.children)
		}
	}
	walk(doc.blocks)
	if !heading || !quote || !list || !table || !code {
		t.Fatalf("parsed blocks heading=%v quote=%v list=%v table=%v code=%v", heading, quote, list, table, code)
	}

	var italic, bold, strike, inlineCode, link, checked, unchecked bool
	for _, inline := range allInline {
		italic = italic || inline.italic
		bold = bold || inline.bold
		strike = strike || inline.strike
		inlineCode = inlineCode || inline.code
		link = link || inline.link == "https://example.com"
		checked = checked || inline.text == "[x]"
		unchecked = unchecked || inline.text == "[ ]"
	}
	if !italic || !bold || !strike || !inlineCode || !link || !checked || !unchecked {
		t.Fatalf("parsed inline italic=%v bold=%v strike=%v code=%v link=%v checked=%v unchecked=%v", italic, bold, strike, inlineCode, link, checked, unchecked)
	}
}

func TestParseMarkdownDocumentDisplaysRawHTMLAsSource(t *testing.T) {
	doc := parseMarkdownDocument([]byte("<script>alert('no')</script>\n"))
	if len(doc.blocks) != 1 || doc.blocks[0].kind != markdownBlockCode {
		t.Fatalf("raw HTML blocks=%#v want one code block", doc.blocks)
	}
	if doc.blocks[0].language != "html" || !strings.Contains(doc.blocks[0].text, "<script>") {
		t.Fatalf("raw HTML preview=%#v", doc.blocks[0])
	}
}

func TestMarkdownInitialModeOverridesViewerCommandRule(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Viewer.CommandRules = []fm.ViewerCommandRule{{Pattern: `.*`, Command: `cat {path}`}}
	ui := NewUI(cfg)
	mode, _ := ui.viewerInitialModeAndCommand("/tmp/README.md", nil, cfg.Viewer.Command)
	if mode != "file" {
		t.Fatalf("Markdown initial mode=%q want native file preview", mode)
	}
}

func TestMarkdownF4EditF3PreviewF3CloseLifecycle(t *testing.T) {
	const original = "# Original\n"
	st := &fileViewerState{
		mode:             "file",
		path:             "/tmp/README.md",
		name:             "README.md",
		content:          original,
		editableContent:  original,
		editBaselineText: original,
	}
	st.stream.SetContent(original)
	st.contentEditor.SetText(original)
	st.markdown.setSource(st.path, original)
	ui := NewUI(fm.DefaultConfig())
	ui.Tabs = widget.Enum{Value: "tab0"}
	ui.fileViewer = st

	if !viewerMarkdownPreviewActive(st) {
		t.Fatal("Markdown should begin in preview mode")
	}
	if specs := ui.viewerFunctionBarButtonSpecs(); specs[2].label != "Close" || specs[3].label != "Edit" {
		t.Fatalf("preview labels F3=%q F4=%q", specs[2].label, specs[3].label)
	}
	if !ui.performFunctionBarAction(functionBarActionOpen, time.Now()) || !st.editMode {
		t.Fatalf("F4 should enter raw Markdown edit mode: %q", st.status)
	}
	if specs := ui.viewerFunctionBarButtonSpecs(); specs[2].label != "Preview" {
		t.Fatalf("edit label F3=%q want Preview", specs[2].label)
	}

	st.contentEditor.SetText("# Changed\n\nNew paragraph.\n")
	ui.syncFileViewerTextEdit(st)
	if !st.editDirty {
		t.Fatal("raw Markdown edit should be dirty")
	}
	if !ui.performFunctionBarAction(functionBarActionView, time.Now()) {
		t.Fatal("first F3 should return to Markdown preview")
	}
	if ui.fileViewer != st || st.editMode || !st.editDirty {
		t.Fatalf("preview return viewer=%p edit=%v dirty=%v", ui.fileViewer, st.editMode, st.editDirty)
	}
	if got := st.markdown.source; !strings.Contains(got, "# Changed") {
		t.Fatalf("preview source=%q want unsaved edited Markdown", got)
	}
	if len(st.markdown.doc.blocks) == 0 || markdownInlineText(st.markdown.doc.blocks[0].inlines) != "Changed" {
		t.Fatalf("preview was not reparsed from edited buffer: %#v", st.markdown.doc.blocks)
	}

	if !ui.performFunctionBarAction(functionBarActionView, time.Now()) {
		t.Fatal("second F3 should close Markdown preview")
	}
	if ui.fileViewer != nil {
		t.Fatal("second F3 should close the viewer")
	}
}

func TestLayoutMarkdownPreviewSmoke(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{mode: "file", path: "README.md", name: "README.md"}
	st.markdown.setSource(st.path, "# Preview\n\nA **native** Markdown preview.\n\n| A | B |\n|---|---|\n| 1 | 2 |\n")
	ui.fileViewer = st
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(900, 600)),
		Now:         time.Now(),
	}
	dims := ui.layoutMarkdownPreview(th, gtx, st)
	if dims.Size.X != 900 || dims.Size.Y != 600 {
		t.Fatalf("preview dimensions=%v want (900,600)", dims.Size)
	}
}

func TestMarkdownPreviewSelectionCopiesOriginalSource(t *testing.T) {
	oldWriteNow := writeFileViewerClipboardNow
	writeFileViewerClipboardNow = func(string) error { return nil }
	t.Cleanup(func() { writeFileViewerClipboardNow = oldWriteNow })

	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{mode: "file", path: "README.md", name: "README.md"}
	st.markdown.setSource(st.path, "# Heading\n\nCopy **selected** words.\n")
	ui.fileViewer = st
	st.markdown.blockSelection = true
	st.markdown.selectionAnchor = 1
	st.markdown.selectionHead = 1

	router := new(input.Router)
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(640, 480)),
		Now:         time.Now(),
	}
	if !ui.copyFileViewerText(gtx, false) {
		t.Fatalf("copy selected Markdown failed: %s", st.status)
	}
	router.Frame(&ops)
	_, copied, ok := router.WriteClipboard()
	if want := "Copy **selected** words.\n"; !ok || string(copied) != want {
		t.Fatalf("clipboard=(%v,%q) want %q", ok, copied, want)
	}
}

func TestMarkdownPreviewSelectAllAndContextMenuUseOriginalMarkdown(t *testing.T) {
	st := &fileViewerState{mode: "file", path: "README.md"}
	source := "# Heading\n\n- first\n- second\n"
	st.markdown.setSource(st.path, source)
	st.markdown.selectAllText()
	if got := st.markdown.selectedText(); got != source {
		t.Fatalf("selected Markdown source=%q", got)
	}
	rows := fileViewerContextMenuRows(st)
	if len(rows) != 1 || rows[0].item.Label != "Copy" {
		t.Fatalf("Markdown context rows=%#v want Copy only", rows)
	}
}

func TestMarkdownBlockSelectionPreservesOriginalSourceSyntax(t *testing.T) {
	source := "# Heading\n\nParagraph with **bold**.\n\n- first\n- second\n\n```go\nfunc main() {}\n```\n"
	st := markdownPreviewState{source: source, doc: parseMarkdownDocument([]byte(source))}
	if len(st.doc.blocks) != 4 {
		t.Fatalf("top-level blocks=%d want 4", len(st.doc.blocks))
	}
	st.blockSelection = true
	st.selectionAnchor = 2
	st.selectionHead = 3
	want := "- first\n- second\n\n```go\nfunc main() {}\n```\n"
	if got := st.selectedMarkdownSource(); got != want {
		t.Fatalf("selected Markdown source=%q want %q", got, want)
	}
	st.selectionAnchor, st.selectionHead = st.selectionHead, st.selectionAnchor
	if got := st.selectedMarkdownSource(); got != want {
		t.Fatalf("reverse selected Markdown source=%q want %q", got, want)
	}
}

func TestMarkdownLineSelectionCopiesIndividualListAndTableRows(t *testing.T) {
	source := "- first\n- second\n- third\n\n| Name | Value |\n| --- | --- |\n| alpha | 1 |\n| beta | 2 |\n"
	st := markdownPreviewState{source: source, doc: parseMarkdownDocument([]byte(source))}
	if len(st.doc.blocks) != 2 {
		t.Fatalf("top-level blocks=%d want 2", len(st.doc.blocks))
	}
	st.blockSelection = true
	st.lineSelection = true
	st.selectionAnchor, st.selectionHead = 0, 0
	st.selectionAnchorLine, st.selectionHeadLine = 1, 1
	if got, want := st.selectedMarkdownSource(), "- second\n"; got != want {
		t.Fatalf("selected list line=%q want %q", got, want)
	}

	st.selectionAnchor, st.selectionHead = 1, 1
	st.selectionAnchorLine, st.selectionHeadLine = 0, 0
	if got, want := st.selectedMarkdownSource(), "| Name | Value |\n| --- | --- |\n"; got != want {
		t.Fatalf("selected table header=%q want %q", got, want)
	}
	st.selectionAnchorLine, st.selectionHeadLine = 2, 1
	if got, want := st.selectedMarkdownSource(), "| alpha | 1 |\n| beta | 2 |\n"; got != want {
		t.Fatalf("reverse selected table rows=%q want %q", got, want)
	}
}

func TestMarkdownQuoteSelectionWeightsWrappedSourceLines(t *testing.T) {
	source := "> Laikai iš nupirktų bilietų pažymėti **paryškintai**.  \n" +
		"> Atstumai tarp viešbučių ir stočių yra apytiksliai.  \n" +
		"> Traukinių peronus ir galimus tvarkaraščio pakeitimus patikrinti kelionės išvakarėse ir dar kartą kelionės dieną.\n"
	ui := NewUI(fm.DefaultConfig())
	st := markdownPreviewState{source: source, doc: parseMarkdownDocument([]byte(source))}
	if len(st.doc.blocks) != 1 || st.doc.blocks[0].kind != markdownBlockQuote {
		t.Fatalf("quote blocks=%#v", st.doc.blocks)
	}
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(480, 600)),
		Now:         time.Now(),
	}
	weights := ui.measureMarkdownSelectionLineWeights(th, gtx, &st, 0)
	if len(weights) != 3 || weights[2] <= weights[1] || weights[0] == weights[1] && weights[1] == weights[2] {
		t.Fatalf("quote line weights=%v want wrapped lines to receive additional height", weights)
	}
	st.blockLineWeights = [][]int{weights}
	total := 0
	for _, weight := range weights {
		total += weight
	}
	st.visibleBlockRects = []markdownVisibleBlock{{
		index:       0,
		rect:        image.Rect(0, 0, 480, total),
		contentRect: image.Rect(0, 0, 480, total),
	}}
	secondLineY := weights[0] + weights[1]/2
	block, line, ok := st.selectionPointAt(image.Pt(100, secondLineY), false)
	if !ok || block != 0 || line != 1 {
		t.Fatalf("second quote line hit=(%d,%d,%v), weights=%v", block, line, ok, weights)
	}
}

func TestMarkdownBlockGapUsesConsistentVerticalRhythm(t *testing.T) {
	paragraph := markdownBlock{kind: markdownBlockParagraph}
	heading := markdownBlock{kind: markdownBlockHeading}
	rule := markdownBlock{kind: markdownBlockRule}
	if got := markdownBlockGap(paragraph, paragraph, 0); got != markdownSpaceMD {
		t.Fatalf("top-level paragraph gap=%v want %v", got, markdownSpaceMD)
	}
	if got := markdownBlockGap(paragraph, heading, 0); got != markdownSpaceLG {
		t.Fatalf("gap before heading=%v want %v", got, markdownSpaceLG)
	}
	if got := markdownBlockGap(paragraph, rule, 0); got != markdownSpaceLG {
		t.Fatalf("gap before rule=%v want %v", got, markdownSpaceLG)
	}
	if got := markdownBlockGap(paragraph, paragraph, 1); got != unit.Dp(12) {
		t.Fatalf("nested paragraph gap=%v want 12dp", got)
	}
	if got := markdownBlockGap(paragraph, markdownBlock{kind: markdownBlockList}, 1); got != markdownSpaceSM {
		t.Fatalf("nested structural gap=%v want %v", got, markdownSpaceSM)
	}
}

func markdownInlineText(inlines []markdownInline) string {
	var b strings.Builder
	for _, inline := range inlines {
		b.WriteString(inline.text)
	}
	return b.String()
}
