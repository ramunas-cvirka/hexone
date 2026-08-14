// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"html"
	"image"
	"image/color"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	gmtext "github.com/yuin/goldmark/text"
)

type markdownBlockKind uint8

const (
	markdownBlockParagraph markdownBlockKind = iota
	markdownBlockHeading
	markdownBlockCode
	markdownBlockQuote
	markdownBlockList
	markdownBlockListItem
	markdownBlockRule
	markdownBlockTable
)

type markdownInline struct {
	id      int
	text    string
	link    string
	image   string
	bold    bool
	italic  bool
	strike  bool
	code    bool
	hardEnd bool
}

type markdownTableCell struct {
	inlines []markdownInline
}

type markdownTableRow struct {
	header bool
	cells  []markdownTableCell
}

type markdownBlock struct {
	id       int
	kind     markdownBlockKind
	level    int
	ordered  bool
	start    int
	language string
	text     string
	inlines  []markdownInline
	children []markdownBlock
	rows     []markdownTableRow

	sourceStart int
	sourceEnd   int
}

type markdownDocument struct {
	blocks []markdownBlock
}

type markdownLinkState struct {
	click       widget.Clickable
	destination string
}

type markdownPreviewState struct {
	detected  bool
	path      string
	source    string
	doc       markdownDocument
	list      widget.List
	selectAll bool

	links      map[string]*markdownLinkState
	tableLists map[int]*widget.List
	codeLists  map[int]*widget.List

	selectionTag        struct{}
	selectionViewport   image.Rectangle
	blockHeights        []int
	blockContentTop     []int
	blockContentSize    []int
	blockLineWeights    [][]int
	visibleBlockRects   []markdownVisibleBlock
	selectingBlocks     bool
	blockSelection      bool
	lineSelection       bool
	selectionID         pointer.ID
	selectionAnchor     int
	selectionAnchorLine int
	selectionHead       int
	selectionHeadLine   int
	selectionDragPos    image.Point
	selectionAutoDir    int
	selectionAutoStep   float32
	selectionAutoAt     time.Time
}

type markdownVisibleBlock struct {
	index       int
	rect        image.Rectangle
	contentRect image.Rectangle
}

type markdownSourceLine struct {
	start int
	end   int
}

const markdownSelectionAutoScrollTick = 50 * time.Millisecond

func viewerPathLooksMarkdown(path string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	switch ext {
	case ".md", ".markdown", ".mdown", ".mkd", ".mkdn", ".mdtext":
		return true
	default:
		return false
	}
}

func (st *markdownPreviewState) initialize(path string) {
	if st == nil {
		return
	}
	st.detected = viewerPathLooksMarkdown(path)
	st.path = path
	st.list.Axis = layout.Vertical
	st.ensureMaps()
}

func (st *markdownPreviewState) ensureMaps() {
	if st.links == nil {
		st.links = make(map[string]*markdownLinkState)
	}
	if st.tableLists == nil {
		st.tableLists = make(map[int]*widget.List)
	}
	if st.codeLists == nil {
		st.codeLists = make(map[int]*widget.List)
	}
}

func (st *markdownPreviewState) setSource(path, source string) {
	if st == nil {
		return
	}
	st.list.Axis = layout.Vertical
	detected := viewerPathLooksMarkdown(path)
	pathChanged := st.path != path
	if st.detected == detected && !pathChanged && st.source == source {
		return
	}
	st.detected = detected
	st.path = path
	st.source = source
	if pathChanged {
		st.list.Position = layout.Position{}
	}
	st.links = make(map[string]*markdownLinkState)
	st.selectAll = false
	st.tableLists = make(map[int]*widget.List)
	st.codeLists = make(map[int]*widget.List)
	st.blockHeights = nil
	st.blockContentTop = nil
	st.blockContentSize = nil
	st.blockLineWeights = nil
	st.visibleBlockRects = nil
	st.stopBlockSelectionDrag()
	st.blockSelection = false
	st.lineSelection = false
	st.selectionAnchor = -1
	st.selectionAnchorLine = -1
	st.selectionHead = -1
	st.selectionHeadLine = -1
	if !detected {
		st.doc = markdownDocument{}
		return
	}
	st.doc = parseMarkdownDocument([]byte(source))
}

func (st *markdownPreviewState) selectedText() string {
	if st == nil {
		return ""
	}
	if st.blockSelection {
		return st.selectedMarkdownSource()
	}
	if st.selectAll {
		return st.source
	}
	return ""
}

func (st *markdownPreviewState) selectAllText() {
	if st == nil {
		return
	}
	st.selectAll = true
	st.blockSelection = false
}

func (st *markdownPreviewState) selectedMarkdownSource() string {
	if st == nil || !st.blockSelection || len(st.doc.blocks) == 0 {
		return ""
	}
	startBlock, startLine := st.selectionAnchor, st.selectionAnchorLine
	endBlock, endLine := st.selectionHead, st.selectionHeadLine
	if startBlock > endBlock || startBlock == endBlock && startLine > endLine {
		startBlock, endBlock = endBlock, startBlock
		startLine, endLine = endLine, startLine
	}
	if startBlock < 0 || endBlock >= len(st.doc.blocks) {
		return ""
	}
	start := st.doc.blocks[startBlock].sourceStart
	end := st.doc.blocks[endBlock].sourceEnd
	if st.lineSelection {
		startLines := st.blockSourceLines(startBlock)
		endLines := st.blockSourceLines(endBlock)
		if startLine < 0 || startLine >= len(startLines) || endLine < 0 || endLine >= len(endLines) {
			return ""
		}
		start = startLines[startLine].start
		end = endLines[endLine].end
	}
	if start < 0 || end < start || end > len(st.source) {
		return ""
	}
	return st.source[start:end]
}

func (st *markdownPreviewState) blockSourceLines(index int) []markdownSourceLine {
	if st == nil || index < 0 || index >= len(st.doc.blocks) {
		return nil
	}
	block := st.doc.blocks[index]
	start := max(0, block.sourceStart)
	end := min(len(st.source), block.sourceEnd)
	if start >= end {
		return nil
	}
	lines := make([]markdownSourceLine, 0, 4)
	for cursor := start; cursor < end; {
		lineEnd := end
		if newline := strings.IndexByte(st.source[cursor:end], '\n'); newline >= 0 {
			lineEnd = cursor + newline + 1
		}
		lines = append(lines, markdownSourceLine{start: cursor, end: lineEnd})
		cursor = lineEnd
	}
	for len(lines) > 0 {
		line := strings.TrimSuffix(st.source[lines[len(lines)-1].start:lines[len(lines)-1].end], "\n")
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) != "" {
			break
		}
		lines = lines[:len(lines)-1]
	}
	if block.kind == markdownBlockTable && len(lines) >= 2 {
		merged := make([]markdownSourceLine, 0, len(lines)-1)
		merged = append(merged, markdownSourceLine{start: lines[0].start, end: lines[1].end})
		merged = append(merged, lines[2:]...)
		return merged
	}
	if block.kind == markdownBlockCode && len(lines) >= 2 &&
		markdownSourceLineLooksFence([]byte(st.source), lines[0].start) &&
		markdownSourceLineLooksFence([]byte(st.source), lines[len(lines)-1].start) {
		visible := append([]markdownSourceLine(nil), lines[:len(lines)-1]...)
		visible[len(visible)-1].end = lines[len(lines)-1].end
		return visible
	}
	return lines
}

func (st *markdownPreviewState) clearSelectableSelections() {
	if st == nil {
		return
	}
	st.selectAll = false
}

func (st *markdownPreviewState) blockSelected(index int) bool {
	if st == nil {
		return false
	}
	if st.selectAll {
		return true
	}
	if !st.blockSelection {
		return false
	}
	start := min(st.selectionAnchor, st.selectionHead)
	end := max(st.selectionAnchor, st.selectionHead)
	return index >= start && index <= end
}

func (st *markdownPreviewState) blockSelectionLines(index int) (start, end, count int, selected bool) {
	lines := st.blockSourceLines(index)
	count = max(1, len(lines))
	if st == nil {
		return 0, 0, count, false
	}
	if st.selectAll || st.blockSelection && !st.lineSelection && st.blockSelected(index) {
		return 0, count - 1, count, true
	}
	if !st.blockSelection || !st.lineSelection {
		return 0, 0, count, false
	}
	startBlock, startLine := st.selectionAnchor, st.selectionAnchorLine
	endBlock, endLine := st.selectionHead, st.selectionHeadLine
	if startBlock > endBlock || startBlock == endBlock && startLine > endLine {
		startBlock, endBlock = endBlock, startBlock
		startLine, endLine = endLine, startLine
	}
	if index < startBlock || index > endBlock {
		return 0, 0, count, false
	}
	start, end = 0, count-1
	if index == startBlock {
		start = min(max(startLine, 0), count-1)
	}
	if index == endBlock {
		end = min(max(endLine, 0), count-1)
	}
	return start, end, count, start <= end
}

func (st *markdownPreviewState) selectionLineWeights(index, count int) []int {
	if st != nil && index >= 0 && index < len(st.blockLineWeights) && len(st.blockLineWeights[index]) == count {
		return st.blockLineWeights[index]
	}
	weights := make([]int, count)
	for i := range weights {
		weights[i] = 1
	}
	return weights
}

func markdownSelectionWeightOffset(weights []int, line int) (offset, total int) {
	for index, weight := range weights {
		weight = max(1, weight)
		total += weight
		if index < line {
			offset += weight
		}
	}
	return offset, max(1, total)
}

func (st *markdownPreviewState) stopBlockSelectionDrag() {
	if st == nil {
		return
	}
	st.selectingBlocks = false
	st.selectionID = 0
	st.selectionAutoDir = 0
	st.selectionAutoStep = 0
	st.selectionAutoAt = time.Time{}
}

func viewerMarkdownPreviewActive(st *fileViewerState) bool {
	return st != nil && st.markdown.detected && st.mode == "file" && !st.editMode &&
		!st.detectedImagePreview && !st.detectedBinaryPreview
}

func (st *markdownPreviewState) scrollByKey(name key.Name) bool {
	if st == nil || len(st.doc.blocks) == 0 {
		return false
	}
	before := st.list.Position
	switch name {
	case key.NameUpArrow:
		st.list.ScrollBy(-1)
	case key.NameDownArrow:
		st.list.ScrollBy(1)
	case key.NamePageUp:
		st.list.ScrollBy(-float32(max(1, st.list.Position.Count-1)))
	case key.NamePageDown:
		st.list.ScrollBy(float32(max(1, st.list.Position.Count-1)))
	case key.NameHome:
		st.list.ScrollTo(0)
	case key.NameEnd:
		st.list.ScrollToEnd = true
		st.list.Position.BeforeEnd = false
	default:
		return false
	}
	return st.list.Position != before || name == key.NameEnd
}

type markdownBuilder struct {
	source []byte
	nextID int
}

func (b *markdownBuilder) id() int {
	b.nextID++
	return b.nextID
}

func parseMarkdownDocument(source []byte) markdownDocument {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	root := md.Parser().Parse(gmtext.NewReader(source))
	builder := markdownBuilder{source: source}
	type blockGroup struct {
		start  int
		blocks []markdownBlock
	}
	groups := make([]blockGroup, 0, root.ChildCount())
	for child := root.FirstChild(); child != nil; child = child.NextSibling() {
		blocks := builder.block(child)
		if len(blocks) == 0 {
			continue
		}
		groups = append(groups, blockGroup{start: markdownNodeSourceStart(child, source), blocks: blocks})
	}
	var blocks []markdownBlock
	for index, group := range groups {
		end := len(source)
		if index+1 < len(groups) {
			end = groups[index+1].start
		}
		if end < group.start {
			end = group.start
		}
		for _, block := range group.blocks {
			block.sourceStart = group.start
			block.sourceEnd = end
			blocks = append(blocks, block)
		}
	}
	return markdownDocument{blocks: blocks}
}

func markdownNodeSourceStart(node gast.Node, source []byte) int {
	start := len(source)
	var visit func(gast.Node)
	visit = func(current gast.Node) {
		if pos := current.Pos(); pos >= 0 && pos < start {
			start = pos
		}
		if current.Type() == gast.TypeBlock {
			lines := current.Lines()
			for index := range lines.Len() {
				if segment := lines.At(index); segment.Start >= 0 && segment.Start < start {
					start = segment.Start
				}
			}
		}
		for child := current.FirstChild(); child != nil; child = child.NextSibling() {
			visit(child)
		}
	}
	visit(node)
	if fenced, ok := node.(*gast.FencedCodeBlock); ok && fenced.Info != nil && fenced.Info.Segment.Start < start {
		start = fenced.Info.Segment.Start
	}
	if start < 0 || start > len(source) {
		start = 0
	}
	start = markdownSourceLineStart(source, start)
	if _, ok := node.(*gast.FencedCodeBlock); ok && !markdownSourceLineLooksFence(source, start) && start > 0 {
		previous := markdownSourceLineStart(source, start-1)
		if markdownSourceLineLooksFence(source, previous) {
			start = previous
		}
	}
	return start
}

func markdownSourceLineStart(source []byte, offset int) int {
	offset = min(max(offset, 0), len(source))
	for offset > 0 && source[offset-1] != '\n' {
		offset--
	}
	return offset
}

func markdownSourceLineLooksFence(source []byte, start int) bool {
	if start < 0 || start >= len(source) {
		return false
	}
	end := start
	for end < len(source) && source[end] != '\n' {
		end++
	}
	line := strings.TrimSpace(string(source[start:end]))
	return strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~")
}

func (b *markdownBuilder) blocks(parent gast.Node) []markdownBlock {
	var out []markdownBlock
	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		out = append(out, b.block(child)...)
	}
	return out
}

func (b *markdownBuilder) block(node gast.Node) []markdownBlock {
	switch n := node.(type) {
	case *gast.Paragraph:
		return []markdownBlock{{id: b.id(), kind: markdownBlockParagraph, inlines: b.inlines(n)}}
	case *gast.TextBlock:
		return []markdownBlock{{id: b.id(), kind: markdownBlockParagraph, inlines: b.inlines(n)}}
	case *gast.Heading:
		return []markdownBlock{{id: b.id(), kind: markdownBlockHeading, level: n.Level, inlines: b.inlines(n)}}
	case *gast.ThematicBreak:
		return []markdownBlock{{id: b.id(), kind: markdownBlockRule}}
	case *gast.CodeBlock:
		return []markdownBlock{{id: b.id(), kind: markdownBlockCode, text: string(n.Lines().Value(b.source))}}
	case *gast.FencedCodeBlock:
		return []markdownBlock{{
			id:       b.id(),
			kind:     markdownBlockCode,
			language: strings.TrimSpace(string(n.Language(b.source))),
			text:     string(n.Lines().Value(b.source)),
		}}
	case *gast.HTMLBlock:
		// Raw HTML is intentionally displayed as source. Native preview never
		// executes document-controlled markup.
		return []markdownBlock{{id: b.id(), kind: markdownBlockCode, language: "html", text: string(n.Lines().Value(b.source))}}
	case *gast.Blockquote:
		return []markdownBlock{{id: b.id(), kind: markdownBlockQuote, children: b.blocks(n)}}
	case *gast.List:
		start := n.Start
		if start < 1 {
			start = 1
		}
		return []markdownBlock{{id: b.id(), kind: markdownBlockList, ordered: n.IsOrdered(), start: start, children: b.blocks(n)}}
	case *gast.ListItem:
		return []markdownBlock{{id: b.id(), kind: markdownBlockListItem, children: b.blocks(n)}}
	case *extast.Table:
		return []markdownBlock{b.table(n)}
	default:
		return b.blocks(node)
	}
}

func (b *markdownBuilder) table(table *extast.Table) markdownBlock {
	block := markdownBlock{id: b.id(), kind: markdownBlockTable}
	for rowNode := table.FirstChild(); rowNode != nil; rowNode = rowNode.NextSibling() {
		row := markdownTableRow{}
		switch rowNode.(type) {
		case *extast.TableHeader:
			row.header = true
		case *extast.TableRow:
		default:
			continue
		}
		for cellNode := rowNode.FirstChild(); cellNode != nil; cellNode = cellNode.NextSibling() {
			cell, ok := cellNode.(*extast.TableCell)
			if !ok {
				continue
			}
			row.cells = append(row.cells, markdownTableCell{inlines: b.inlines(cell)})
		}
		block.rows = append(block.rows, row)
	}
	return block
}

type markdownInlineStyle struct {
	bold   bool
	italic bool
	strike bool
	code   bool
	link   string
}

func (b *markdownBuilder) inlines(parent gast.Node) []markdownInline {
	var out []markdownInline
	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		b.collectInline(child, markdownInlineStyle{}, &out)
	}
	return out
}

func (b *markdownBuilder) collectInline(node gast.Node, style markdownInlineStyle, out *[]markdownInline) {
	switch n := node.(type) {
	case *gast.Text:
		value := string(n.Value(b.source))
		if !n.IsRaw() {
			value = html.UnescapeString(value)
		}
		b.appendInline(out, value, style, "", n.HardLineBreak())
		if n.SoftLineBreak() {
			b.appendInline(out, " ", style, "", false)
		}
		return
	case *gast.String:
		b.appendInline(out, html.UnescapeString(string(n.Value)), style, "", false)
		return
	case *gast.Emphasis:
		if n.Level >= 2 {
			style.bold = true
		} else {
			style.italic = true
		}
	case *gast.CodeSpan:
		style.code = true
	case *gast.Link:
		style.link = strings.TrimSpace(string(n.Destination))
	case *gast.Image:
		alt := strings.TrimSpace(b.plainInlineText(n))
		if alt == "" {
			alt = "image"
		}
		*out = append(*out, markdownInline{id: b.id(), text: alt, image: strings.TrimSpace(string(n.Destination))})
		return
	case *gast.AutoLink:
		destination := string(n.URL(b.source))
		if n.AutoLinkType == gast.AutoLinkEmail && !strings.HasPrefix(strings.ToLower(destination), "mailto:") {
			destination = "mailto:" + destination
		}
		style.link = destination
		b.appendInline(out, string(n.Label(b.source)), style, "", false)
		return
	case *gast.RawHTML:
		b.appendInline(out, string(n.Segments.Value(b.source)), markdownInlineStyle{code: true}, "", false)
		return
	case *extast.Strikethrough:
		style.strike = true
	case *extast.TaskCheckBox:
		mark := "[ ]"
		if n.IsChecked {
			mark = "[x]"
		}
		b.appendInline(out, mark, markdownInlineStyle{bold: true}, "", false)
		return
	}
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		b.collectInline(child, style, out)
	}
}

func (b *markdownBuilder) appendInline(out *[]markdownInline, value string, style markdownInlineStyle, image string, hardEnd bool) {
	if value == "" && !hardEnd {
		return
	}
	*out = append(*out, markdownInline{
		id: b.id(), text: value, image: image, hardEnd: hardEnd,
		link: style.link, bold: style.bold, italic: style.italic, strike: style.strike, code: style.code,
	})
}

func (b *markdownBuilder) plainInlineText(parent gast.Node) string {
	var parts []string
	_ = gast.Walk(parent, func(node gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering || node == parent {
			return gast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *gast.Text:
			parts = append(parts, string(n.Value(b.source)))
		case *gast.String:
			parts = append(parts, string(n.Value))
		}
		return gast.WalkContinue, nil
	})
	return html.UnescapeString(strings.Join(parts, ""))
}

type markdownRenderToken struct {
	inline      markdownInline
	text        string
	spaceBefore bool
	breakAfter  bool
	key         string
}

func markdownTokens(blockID int, inlines []markdownInline) []markdownRenderToken {
	var out []markdownRenderToken
	pendingSpace := false
	for _, in := range inlines {
		if in.image != "" || in.code {
			text := strings.TrimSpace(in.text)
			if in.image != "" {
				text = "▧ " + text
			}
			if text != "" {
				out = append(out, markdownRenderToken{
					inline: in, text: text, spaceBefore: pendingSpace && len(out) > 0,
					breakAfter: in.hardEnd, key: fmt.Sprintf("%d:%d:0", blockID, in.id),
				})
				pendingSpace = false
			}
			continue
		}
		var word []rune
		wordIndex := 0
		flush := func() {
			if len(word) == 0 {
				return
			}
			out = append(out, markdownRenderToken{
				inline: in, text: string(word), spaceBefore: pendingSpace && len(out) > 0,
				key: fmt.Sprintf("%d:%d:%d", blockID, in.id, wordIndex),
			})
			wordIndex++
			word = word[:0]
			pendingSpace = false
		}
		for _, r := range in.text {
			if unicode.IsSpace(r) {
				flush()
				pendingSpace = true
				continue
			}
			word = append(word, r)
		}
		flush()
		if in.hardEnd && len(out) > 0 {
			out[len(out)-1].breakAfter = true
			pendingSpace = false
		}
	}
	return out
}

type markdownColors struct {
	text, muted, heading, link               color.NRGBA
	codeBg, codeBorder, quoteBg, quoteBorder color.NRGBA
	tableHeader, tableCell, tableBorder      color.NRGBA
}

const (
	markdownSpaceXS unit.Dp = 4
	markdownSpaceSM unit.Dp = 8
	markdownSpaceMD unit.Dp = 16
	markdownSpaceLG unit.Dp = 24
)

func markdownBlockGap(current, next markdownBlock, depth int) unit.Dp {
	if depth <= 0 {
		if next.kind == markdownBlockHeading || current.kind == markdownBlockRule || next.kind == markdownBlockRule {
			return markdownSpaceLG
		}
		return markdownSpaceMD
	}
	if next.kind == markdownBlockHeading || current.kind == markdownBlockRule || next.kind == markdownBlockRule {
		return markdownSpaceMD
	}
	if current.kind == markdownBlockParagraph && next.kind == markdownBlockParagraph {
		return unit.Dp(12)
	}
	return markdownSpaceSM
}

func markdownPreviewColors(theme fileViewerTheme) markdownColors {
	link := mixNRGBA(theme.Text, color.NRGBA{R: 72, G: 190, B: 255, A: 255}, 0.72)
	link.A = 255
	return markdownColors{
		text: theme.Text, muted: theme.Muted, heading: theme.HeaderText, link: link,
		codeBg:      mixNRGBA(theme.PanelBg, theme.Text, 0.055),
		codeBorder:  mixNRGBA(theme.PanelBg, theme.Text, 0.22),
		quoteBg:     mixNRGBA(theme.PanelBg, theme.StatusAccent, 0.07),
		quoteBorder: mixNRGBA(theme.PanelBg, theme.StatusAccent, 0.52),
		tableHeader: mixNRGBA(theme.PanelBg, theme.Text, 0.10),
		tableCell:   mixNRGBA(theme.PanelBg, theme.Text, 0.035),
		tableBorder: mixNRGBA(theme.PanelBg, theme.Text, 0.20),
	}
}

func (st *markdownPreviewState) rebuildVisibleBlockRects() {
	if st == nil {
		return
	}
	st.visibleBlockRects = st.visibleBlockRects[:0]
	if len(st.blockHeights) == 0 || st.list.Position.Count <= 0 {
		return
	}
	top := st.list.Position.Offset
	first := max(0, st.list.Position.First)
	end := min(len(st.blockHeights), first+st.list.Position.Count)
	for index := first; index < end; index++ {
		height := st.blockHeights[index]
		if height < 1 {
			continue
		}
		rect := image.Rect(st.selectionViewport.Min.X, top, st.selectionViewport.Max.X, top+height)
		contentTop := top
		contentHeight := height
		if index < len(st.blockContentTop) {
			contentTop += st.blockContentTop[index]
		}
		if index < len(st.blockContentSize) && st.blockContentSize[index] > 0 {
			contentHeight = st.blockContentSize[index]
		}
		contentRect := image.Rect(rect.Min.X, contentTop, rect.Max.X, min(rect.Max.Y, contentTop+contentHeight))
		st.visibleBlockRects = append(st.visibleBlockRects, markdownVisibleBlock{index: index, rect: rect, contentRect: contentRect})
		top += height
	}
}

func (st *markdownPreviewState) blockAtPoint(pos image.Point, clampToVisible bool) (int, bool) {
	if st == nil || len(st.visibleBlockRects) == 0 {
		return 0, false
	}
	for _, visible := range st.visibleBlockRects {
		if pos.Y >= visible.rect.Min.Y && pos.Y < visible.rect.Max.Y {
			return visible.index, true
		}
	}
	if !clampToVisible {
		return 0, false
	}
	if pos.Y < st.visibleBlockRects[0].rect.Min.Y {
		return st.visibleBlockRects[0].index, true
	}
	return st.visibleBlockRects[len(st.visibleBlockRects)-1].index, true
}

func (st *markdownPreviewState) selectionPointAt(pos image.Point, clampToVisible bool) (block, line int, ok bool) {
	block, ok = st.blockAtPoint(pos, clampToVisible)
	if !ok {
		return 0, 0, false
	}
	lines := st.blockSourceLines(block)
	if len(lines) == 0 {
		return block, 0, true
	}
	var content image.Rectangle
	for _, visible := range st.visibleBlockRects {
		if visible.index == block {
			content = visible.contentRect
			break
		}
	}
	if content.Dy() <= 0 {
		return block, 0, true
	}
	switch {
	case pos.Y <= content.Min.Y:
		line = 0
	case pos.Y >= content.Max.Y:
		line = len(lines) - 1
	default:
		weights := st.selectionLineWeights(block, len(lines))
		_, totalWeight := markdownSelectionWeightOffset(weights, len(weights))
		pointWeight := (pos.Y - content.Min.Y) * totalWeight / content.Dy()
		cumulative := 0
		for index, weight := range weights {
			cumulative += max(1, weight)
			line = index
			if pointWeight < cumulative {
				break
			}
		}
	}
	return block, line, true
}

func (st *markdownPreviewState) updateBlockSelectionAutoScroll(pos image.Point, now time.Time) {
	if st == nil || !st.selectingBlocks {
		return
	}
	dir := 0
	distance := 0
	switch {
	case pos.Y < st.selectionViewport.Min.Y:
		dir = -1
		distance = st.selectionViewport.Min.Y - pos.Y
	case pos.Y >= st.selectionViewport.Max.Y:
		dir = 1
		distance = pos.Y - st.selectionViewport.Max.Y + 1
	default:
		st.selectionAutoDir = 0
		st.selectionAutoStep = 0
		st.selectionAutoAt = time.Time{}
		return
	}
	step := float32(0.35)
	if distance > streamAutoScrollMidPx {
		step = 1.5
	} else if distance > streamAutoScrollNearPx {
		step = 0.8
	}
	changed := st.selectionAutoDir != dir || st.selectionAutoStep != step
	st.selectionAutoDir = dir
	st.selectionAutoStep = step
	if changed || st.selectionAutoAt.IsZero() {
		st.selectionAutoAt = now
	}
}

func (st *markdownPreviewState) runBlockSelectionAutoScroll(now time.Time) bool {
	if st == nil || !st.selectingBlocks || st.selectionAutoDir == 0 || st.selectionAutoStep <= 0 ||
		st.selectionAutoAt.IsZero() || now.Before(st.selectionAutoAt) {
		return false
	}
	st.list.ScrollBy(float32(st.selectionAutoDir) * st.selectionAutoStep)
	st.selectionAutoAt = now.Add(markdownSelectionAutoScrollTick)
	return true
}

func (ui *UI) handleMarkdownPreviewSelectionEvents(gtx layout.Context, viewer *fileViewerState) {
	if viewer == nil {
		return
	}
	st := &viewer.markdown
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &st.selectionTag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		pos := pe.Position.Round()
		switch pe.Kind {
		case pointer.Press:
			if pe.Buttons.Contain(pointer.ButtonSecondary) {
				st.stopBlockSelectionDrag()
				viewer.openContextMenu(pos, gtx.Now)
				viewer.markUserBrowsing(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			if !pe.Buttons.Contain(pointer.ButtonPrimary) || !viewerPointInRect(pos, st.selectionViewport) ||
				pos.X >= st.selectionViewport.Max.X-gtx.Dp(unit.Dp(12)) {
				continue
			}
			index, line, ok := st.selectionPointAt(pos, false)
			if !ok {
				continue
			}
			st.blockSelection = false
			st.lineSelection = true
			st.selectingBlocks = true
			st.selectionID = pe.PointerID
			st.selectionAnchor = index
			st.selectionAnchorLine = line
			st.selectionHead = index
			st.selectionHeadLine = line
			st.selectionDragPos = pos
			st.selectionAutoDir = 0
			st.selectionAutoStep = 0
			st.selectionAutoAt = time.Time{}
		case pointer.Drag:
			if !st.selectingBlocks || pe.PointerID != st.selectionID {
				continue
			}
			if !pe.Buttons.Contain(pointer.ButtonPrimary) {
				st.stopBlockSelectionDrag()
				continue
			}
			st.selectionDragPos = pos
			if index, line, ok := st.selectionPointAt(pos, true); ok {
				if !st.blockSelection {
					st.clearSelectableSelections()
					st.blockSelection = true
					gtx.Execute(pointer.GrabCmd{Tag: &st.selectionTag, ID: pe.PointerID})
				}
				st.selectionHead = index
				st.selectionHeadLine = line
			}
			st.updateBlockSelectionAutoScroll(pos, gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
			viewer.markUserBrowsing(gtx.Now)
		case pointer.Release, pointer.Cancel:
			if st.selectingBlocks && pe.PointerID == st.selectionID {
				st.stopBlockSelectionDrag()
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
}

func (ui *UI) measureMarkdownSelectionLineWeights(th *material.Theme, gtx layout.Context, state *markdownPreviewState, index int) []int {
	lines := state.blockSourceLines(index)
	weights := make([]int, len(lines))
	if len(lines) == 0 {
		return weights
	}
	block := state.doc.blocks[index]
	if block.kind == markdownBlockTable || block.kind == markdownBlockCode || block.kind == markdownBlockRule {
		for line := range weights {
			weights[line] = 1
		}
		return weights
	}
	for lineIndex, sourceLine := range lines {
		lineText := state.source[sourceLine.start:sourceLine.end]
		lineText = strings.TrimSuffix(lineText, "\n")
		lineText = strings.TrimSuffix(lineText, "\r")
		measureGtx := gtx
		measureGtx.Constraints.Min = image.Point{}
		if block.kind == markdownBlockQuote {
			trimmed := strings.TrimLeft(lineText, " \t")
			if strings.HasPrefix(trimmed, ">") {
				lineText = strings.TrimPrefix(trimmed, ">")
				lineText = strings.TrimPrefix(lineText, " ")
			}
			measureGtx.Constraints.Max.X = max(1, measureGtx.Constraints.Max.X-gtx.Dp(unit.Dp(34)))
		}
		lineText = markdownSelectionMeasureText(lineText)
		if lineText == "" {
			lineText = " "
		}
		label := material.Body1(th, lineText)
		label.Font.Typeface = ui.viewerTypeface()
		label.TextSize = ui.viewerTextSize()
		record := op.Record(gtx.Ops)
		dims := label.Layout(measureGtx)
		_ = record.Stop()
		weights[lineIndex] = max(1, dims.Size.Y)
	}
	return weights
}

func markdownSelectionMeasureText(sourceLine string) string {
	text := strings.TrimRight(sourceLine, " \t")
	for {
		labelEnd := strings.Index(text, "](")
		if labelEnd < 0 {
			break
		}
		open := strings.LastIndexByte(text[:labelEnd], '[')
		if open < 0 {
			break
		}
		destinationEnd := strings.IndexByte(text[labelEnd+2:], ')')
		if destinationEnd < 0 {
			break
		}
		destinationEnd += labelEnd + 2
		labelStart := open + 1
		if open > 0 && text[open-1] == '!' {
			open--
		}
		text = text[:open] + text[labelStart:labelEnd] + text[destinationEnd+1:]
	}
	return strings.NewReplacer("**", "", "__", "", "~~", "", "`", "", "*", "", "_", "").Replace(text)
}

func (ui *UI) layoutMarkdownBlockRangeSelection(gtx layout.Context, startLine, endLine int, lineWeights []int, selected bool, block layout.Widget) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dims := block(gtx)
	call := record.Stop()
	call.Add(gtx.Ops)
	if selected && dims.Size.X > 0 && dims.Size.Y > 0 {
		lineCount := max(1, len(lineWeights))
		startLine = min(max(startLine, 0), lineCount-1)
		endLine = min(max(endLine, startLine), lineCount-1)
		if len(lineWeights) != lineCount {
			lineWeights = []int{1}
		}
		topWeight, totalWeight := markdownSelectionWeightOffset(lineWeights, startLine)
		bottomWeight, _ := markdownSelectionWeightOffset(lineWeights, endLine+1)
		top := dims.Size.Y * topWeight / totalWeight
		bottom := dims.Size.Y * bottomWeight / totalWeight
		bottom = max(top+1, bottom)
		theme := ui.fileViewerTheme()
		selection := scaleColorAlpha(theme.Selection, 0.34)
		rect := image.Rect(0, top, dims.Size.X, bottom)
		paint.FillShape(gtx.Ops, selection, clip.Rect(rect).Op())

		edge := scaleColorAlpha(theme.Selection, 0.92)
		line := max(1, gtx.Dp(unit.Dp(1)))
		accent := max(line, gtx.Dp(unit.Dp(3)))
		paint.FillShape(gtx.Ops, edge, clip.Rect(image.Rect(0, top, dims.Size.X, min(bottom, top+line))).Op())
		paint.FillShape(gtx.Ops, edge, clip.Rect(image.Rect(0, max(top, bottom-line), dims.Size.X, bottom)).Op())
		paint.FillShape(gtx.Ops, edge, clip.Rect(image.Rect(0, top, accent, bottom)).Op())
		paint.FillShape(gtx.Ops, edge, clip.Rect(image.Rect(dims.Size.X-line, top, dims.Size.X, bottom)).Op())
	}
	return dims
}

func (ui *UI) layoutMarkdownPreview(th *material.Theme, gtx layout.Context, viewer *fileViewerState) layout.Dimensions {
	if viewer == nil {
		return layout.Dimensions{}
	}
	state := &viewer.markdown
	state.ensureMaps()
	colors := markdownPreviewColors(ui.fileViewerTheme())
	style := material.List(th, &state.list)
	style.AnchorStrategy = material.Occupy
	style.ScrollbarStyle.Track.Color = ui.fileViewerTheme().ScrollTrack
	style.ScrollbarStyle.Indicator.Color = ui.fileViewerTheme().ScrollThumb
	style.ScrollbarStyle.Indicator.HoverColor = ui.fileViewerTheme().ScrollThumbHover
	style.ScrollbarStyle.Indicator.MinorWidth = unit.Dp(7)
	style.ScrollbarStyle.Indicator.CornerRadius = unit.Dp(4)
	state.selectionViewport = image.Rectangle{Max: gtx.Constraints.Max}
	ui.handleMarkdownPreviewSelectionEvents(gtx, viewer)
	if state.runBlockSelectionAutoScroll(gtx.Now) {
		viewer.markUserBrowsing(gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}

	if len(state.doc.blocks) == 0 {
		label := material.Body1(th, "Empty Markdown document")
		label.Font.Typeface = ui.viewerTypeface()
		label.TextSize = ui.viewerTextSize()
		label.Color = colors.muted
		return layout.Center.Layout(gtx, label.Layout)
	}
	if len(state.blockHeights) != len(state.doc.blocks) {
		state.blockHeights = make([]int, len(state.doc.blocks))
		state.blockContentTop = make([]int, len(state.doc.blocks))
		state.blockContentSize = make([]int, len(state.doc.blocks))
		state.blockLineWeights = make([][]int, len(state.doc.blocks))
	}
	if len(state.blockLineWeights) != len(state.doc.blocks) {
		state.blockLineWeights = make([][]int, len(state.doc.blocks))
	}
	selectionArea := clip.Rect(state.selectionViewport).Push(gtx.Ops)
	event.Op(gtx.Ops, &state.selectionTag)
	selectionArea.Pop()

	pass := pointer.PassOp{}.Push(gtx.Ops)
	dims := style.Layout(gtx, len(state.doc.blocks), func(gtx layout.Context, index int) layout.Dimensions {
		bottom := markdownSpaceMD
		if index < len(state.doc.blocks)-1 {
			bottom = markdownBlockGap(state.doc.blocks[index], state.doc.blocks[index+1], 0)
		}
		top := unit.Dp(0)
		if index == 0 {
			top = markdownSpaceMD
		}
		state.blockContentTop[index] = gtx.Dp(top)
		itemDims := layout.Inset{Left: markdownSpaceMD, Right: markdownSpaceMD, Top: top, Bottom: bottom}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			maxWidth := gtx.Dp(unit.Dp(980))
			width := gtx.Constraints.Max.X
			if width > maxWidth {
				width = maxWidth
			}
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
					startLine, endLine, lineCount, selected := state.blockSelectionLines(index)
					state.blockLineWeights[index] = ui.measureMarkdownSelectionLineWeights(th, gtx, state, index)
					lineWeights := state.selectionLineWeights(index, lineCount)
					dims := ui.layoutMarkdownBlockRangeSelection(gtx, startLine, endLine, lineWeights, selected, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutMarkdownBlock(th, gtx, viewer, state.doc.blocks[index], colors, 0)
					})
					state.blockContentSize[index] = dims.Size.Y
					return dims
				})
			})
		})
		state.blockHeights[index] = itemDims.Size.Y
		return itemDims
	})
	pass.Pop()
	state.rebuildVisibleBlockRects()
	if state.selectingBlocks && state.blockSelection {
		if index, line, ok := state.selectionPointAt(state.selectionDragPos, true); ok &&
			(index != state.selectionHead || line != state.selectionHeadLine) {
			state.selectionHead = index
			state.selectionHeadLine = line
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	if state.selectingBlocks && state.selectionAutoDir != 0 && !state.selectionAutoAt.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: state.selectionAutoAt})
	}
	for _, link := range state.links {
		if link.click.Hovered() {
			pointer.CursorPointer.Add(gtx.Ops)
			break
		}
	}
	return dims
}

func (ui *UI) handleMarkdownLinkClick(viewer *fileViewerState, destination string) {
	destination = strings.TrimSpace(destination)
	if viewer == nil || destination == "" {
		return
	}
	parsed, err := url.Parse(destination)
	if err != nil {
		viewer.status = "invalid Markdown link"
		return
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "http", "https", "mailto":
		if err := openFileWithSystemAssociationFunc(destination); err != nil {
			viewer.status = "link open failed: " + err.Error()
		}
		return
	case "":
		if parsed.Path == "" {
			viewer.status = "document anchor: " + destination
			return
		}
		if viewer.remote != nil {
			viewer.status = "relative links from SFTP previews are not available yet"
			return
		}
		target := parsed.Path
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(viewer.path), filepath.FromSlash(target))
		}
		if err := openFileWithSystemAssociationFunc(filepath.Clean(target)); err != nil {
			viewer.status = "link open failed: " + err.Error()
		}
		return
	default:
		viewer.status = "blocked Markdown link scheme: " + scheme
	}
}

func (ui *UI) layoutMarkdownBlock(th *material.Theme, gtx layout.Context, viewer *fileViewerState, block markdownBlock, colors markdownColors, depth int) layout.Dimensions {
	base := ui.viewerTextSize()
	switch block.kind {
	case markdownBlockHeading:
		scale := float32(1.08)
		switch block.level {
		case 1:
			scale = 1.70
		case 2:
			scale = 1.45
		case 3:
			scale = 1.28
		case 4:
			scale = 1.16
		}
		return ui.layoutMarkdownInline(th, gtx, viewer, block.id, block.inlines, unit.Sp(float32(base)*scale), font.Bold, colors)
	case markdownBlockParagraph:
		return ui.layoutMarkdownInline(th, gtx, viewer, block.id, block.inlines, base, font.Normal, colors)
	case markdownBlockRule:
		h := max(1, gtx.Dp(unit.Dp(1)))
		paint.FillShape(gtx.Ops, colors.tableBorder, clip.Rect(image.Rect(0, 0, gtx.Constraints.Max.X, h)).Op())
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
	case markdownBlockCode:
		return ui.layoutMarkdownCode(th, gtx, viewer, block, colors)
	case markdownBlockQuote:
		return ui.layoutMarkdownQuote(th, gtx, viewer, block, colors, depth)
	case markdownBlockList:
		return ui.layoutMarkdownList(th, gtx, viewer, block, colors, depth)
	case markdownBlockListItem:
		return ui.layoutMarkdownChildren(th, gtx, viewer, block.children, colors, depth)
	case markdownBlockTable:
		return ui.layoutMarkdownTable(th, gtx, viewer, block, colors)
	default:
		return layout.Dimensions{}
	}
}

func (ui *UI) layoutMarkdownChildren(th *material.Theme, gtx layout.Context, viewer *fileViewerState, blocks []markdownBlock, colors markdownColors, depth int) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(blocks)*2)
	for i := range blocks {
		block := blocks[i]
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutMarkdownBlock(th, gtx, viewer, block, colors, depth)
		}))
		if i < len(blocks)-1 {
			gap := markdownBlockGap(block, blocks[i+1], depth)
			children = append(children, layout.Rigid(layout.Spacer{Height: gap}.Layout))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (ui *UI) layoutMarkdownQuote(th *material.Theme, gtx layout.Context, viewer *fileViewerState, block markdownBlock, colors markdownColors, depth int) layout.Dimensions {
	return fillRoundedBox(gtx, gtx.Dp(unit.Dp(4)), colors.quoteBg, color.NRGBA{}, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 18, Right: markdownSpaceMD, Top: 12, Bottom: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			dims := ui.layoutMarkdownChildren(th, gtx, viewer, block.children, colors, depth+1)
			barW := max(2, gtx.Dp(unit.Dp(3)))
			barX := -gtx.Dp(unit.Dp(12))
			paint.FillShape(gtx.Ops, colors.quoteBorder, clip.Rect(image.Rect(barX, 0, barX+barW, dims.Size.Y)).Op())
			return dims
		})
	})
}

func (ui *UI) layoutMarkdownList(th *material.Theme, gtx layout.Context, viewer *fileViewerState, block markdownBlock, colors markdownColors, depth int) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(block.children)*2)
	for index := range block.children {
		item := block.children[index]
		marker := "•"
		if block.ordered {
			marker = fmt.Sprintf("%d.", block.start+index)
		} else if depth%3 == 1 {
			marker = "◦"
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Body1(th, marker)
					label.Font.Typeface = ui.viewerTypeface()
					label.Font.Weight = font.Bold
					label.TextSize = ui.viewerTextSize()
					label.Color = colors.muted
					return fixedWidth(gtx, gtx.Dp(unit.Dp(30)), label.Layout)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutMarkdownChildren(th, gtx, viewer, item.children, colors, depth+1)
				}),
			)
		}))
		if index < len(block.children)-1 {
			children = append(children, layout.Rigid(layout.Spacer{Height: markdownSpaceXS}.Layout))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (ui *UI) markdownHorizontalList(state *markdownPreviewState, id int, table bool) *widget.List {
	state.ensureMaps()
	lists := state.codeLists
	if table {
		lists = state.tableLists
	}
	if list := lists[id]; list != nil {
		return list
	}
	list := &widget.List{List: layout.List{Axis: layout.Horizontal}}
	lists[id] = list
	return list
}

func (ui *UI) layoutMarkdownCode(th *material.Theme, gtx layout.Context, viewer *fileViewerState, block markdownBlock, colors markdownColors) layout.Dimensions {
	return fillRoundedBox(gtx, gtx.Dp(unit.Dp(5)), colors.codeBg, colors.codeBorder, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(markdownSpaceMD).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 3)
			if block.language != "" {
				children = append(children,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Caption(th, strings.ToUpper(block.language))
						label.Font.Typeface = ui.viewerMonospaceTypeface()
						label.Font.Weight = font.Medium
						label.TextSize = unit.Sp(float32(ui.viewerTextSize()) * 0.82)
						label.Color = colors.muted
						return label.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: markdownSpaceSM}.Layout),
				)
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				listState := ui.markdownHorizontalList(&viewer.markdown, block.id, false)
				style := material.List(th, listState)
				style.AnchorStrategy = material.Overlay
				return style.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
					code := strings.ReplaceAll(strings.TrimSuffix(block.text, "\n"), "\t", "    ")
					label := material.Body2(th, code)
					label.Font.Typeface = ui.viewerMonospaceTypeface()
					label.TextSize = unit.Sp(float32(ui.viewerTextSize()) * 0.92)
					label.Color = colors.text
					return label.Layout(gtx)
				})
			}))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (ui *UI) layoutMarkdownTable(th *material.Theme, gtx layout.Context, viewer *fileViewerState, block markdownBlock, colors markdownColors) layout.Dimensions {
	columns := 0
	for _, row := range block.rows {
		columns = max(columns, len(row.cells))
	}
	if columns == 0 {
		return layout.Dimensions{}
	}
	viewportW := gtx.Constraints.Max.X
	minCellW := gtx.Dp(unit.Dp(120))
	tableW := max(viewportW, minCellW*columns)
	cellW := tableW / columns
	listState := ui.markdownHorizontalList(&viewer.markdown, block.id, true)
	style := material.List(th, listState)
	style.AnchorStrategy = material.Occupy
	return style.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return fixedWidth(gtx, tableW, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(block.rows))
			for rowIndex := range block.rows {
				row := block.rows[rowIndex]
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutMarkdownTableRow(th, gtx, viewer, block.id, rowIndex, row, columns, cellW, colors)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (ui *UI) layoutMarkdownTableRow(th *material.Theme, gtx layout.Context, viewer *fileViewerState, blockID, rowIndex int, row markdownTableRow, columns, cellW int, colors markdownColors) layout.Dimensions {
	type recordedCell struct {
		call op.CallOp
		dims layout.Dimensions
	}
	cells := make([]recordedCell, columns)
	rowH := 0
	for column := 0; column < columns; column++ {
		cellGtx := gtx
		cellGtx.Constraints.Min = image.Point{X: cellW}
		cellGtx.Constraints.Max.X = cellW
		record := op.Record(gtx.Ops)
		var dims layout.Dimensions
		if column < len(row.cells) {
			dims = layout.Inset{Left: 13, Right: 13, Top: 6, Bottom: 6}.Layout(cellGtx, func(gtx layout.Context) layout.Dimensions {
				weight := font.Normal
				if row.header {
					weight = font.Bold
				}
				return ui.layoutMarkdownInline(th, gtx, viewer, blockID*10000+rowIndex*100+column, row.cells[column].inlines, unit.Sp(float32(ui.viewerTextSize())*0.92), weight, colors)
			})
		} else {
			dims = layout.Dimensions{Size: image.Pt(cellW, gtx.Dp(unit.Dp(24)))}
		}
		cells[column] = recordedCell{call: record.Stop(), dims: dims}
		rowH = max(rowH, dims.Size.Y)
	}
	bg := colors.tableCell
	if row.header {
		bg = colors.tableHeader
	}
	for column, cell := range cells {
		x := column * cellW
		rect := image.Rect(x, 0, x+cellW, rowH)
		paint.FillShape(gtx.Ops, bg, clip.Rect(rect).Op())
		border := max(1, gtx.Dp(unit.Dp(1)))
		paint.FillShape(gtx.Ops, colors.tableBorder, clip.Rect(image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+border)).Op())
		paint.FillShape(gtx.Ops, colors.tableBorder, clip.Rect(image.Rect(rect.Min.X, rect.Max.Y-border, rect.Max.X, rect.Max.Y)).Op())
		paint.FillShape(gtx.Ops, colors.tableBorder, clip.Rect(image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+border, rect.Max.Y)).Op())
		paint.FillShape(gtx.Ops, colors.tableBorder, clip.Rect(image.Rect(rect.Max.X-border, rect.Min.Y, rect.Max.X, rect.Max.Y)).Op())
		offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		cell.call.Add(gtx.Ops)
		offset.Pop()
	}
	return layout.Dimensions{Size: image.Pt(cellW*columns, rowH)}
}

func (ui *UI) layoutMarkdownInline(th *material.Theme, gtx layout.Context, viewer *fileViewerState, blockID int, inlines []markdownInline, size unit.Sp, baseWeight font.Weight, colors markdownColors) layout.Dimensions {
	tokens := markdownTokens(blockID, inlines)
	if len(tokens) == 0 {
		return layout.Dimensions{}
	}
	spaceW := ui.measureMarkdownSpace(th, gtx, size)
	maxW := max(1, gtx.Constraints.Max.X)
	rows := make([][]markdownRenderToken, 1)
	rowW := 0
	for _, token := range tokens {
		width := ui.measureMarkdownToken(th, gtx, token, size, baseWeight)
		gap := 0
		if token.spaceBefore && len(rows[len(rows)-1]) > 0 {
			gap = spaceW
		}
		if len(rows[len(rows)-1]) > 0 && rowW+gap+width > maxW {
			rows = append(rows, nil)
			rowW = 0
			gap = 0
			token.spaceBefore = false
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], token)
		rowW += gap + width
		if token.breakAfter {
			rows = append(rows, nil)
			rowW = 0
		}
	}
	if len(rows[len(rows)-1]) == 0 {
		rows = rows[:len(rows)-1]
	}
	children := make([]layout.FlexChild, 0, len(rows)*2)
	for rowIndex := range rows {
		row := rows[rowIndex]
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			rowChildren := make([]layout.FlexChild, 0, len(row)*2)
			for tokenIndex := range row {
				token := row[tokenIndex]
				if token.spaceBefore && tokenIndex > 0 {
					rowChildren = append(rowChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Pt(spaceW, 0)}
					}))
				}
				rowChildren = append(rowChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutMarkdownToken(th, gtx, viewer, token, size, baseWeight, colors)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx, rowChildren...)
		}))
		if rowIndex < len(rows)-1 {
			children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (ui *UI) measureMarkdownSpace(th *material.Theme, gtx layout.Context, size unit.Sp) int {
	label := material.Body1(th, " ")
	label.Font.Typeface = ui.viewerTypeface()
	label.TextSize = size
	return max(gtx.Dp(unit.Dp(3)), measureLabelUnconstrained(gtx, label).Size.X)
}

func (ui *UI) measureMarkdownToken(th *material.Theme, gtx layout.Context, token markdownRenderToken, size unit.Sp, baseWeight font.Weight) int {
	label := material.Body1(th, token.text)
	label.Font.Typeface = ui.viewerTypeface()
	label.Font.Weight = baseWeight
	label.TextSize = size
	if token.inline.bold {
		label.Font.Weight = font.Bold
	}
	if token.inline.italic {
		label.Font.Style = font.Italic
	}
	if token.inline.code || token.inline.image != "" {
		label.Font.Typeface = ui.viewerMonospaceTypeface()
	}
	width := measureLabelUnconstrained(gtx, label).Size.X
	if token.inline.code || token.inline.image != "" {
		width += gtx.Dp(unit.Dp(8))
	}
	return width
}

func (ui *UI) markdownLink(state *markdownPreviewState, key, destination string) *markdownLinkState {
	state.ensureMaps()
	if link := state.links[key]; link != nil {
		return link
	}
	link := &markdownLinkState{destination: destination}
	state.links[key] = link
	return link
}

func (ui *UI) layoutMarkdownToken(th *material.Theme, gtx layout.Context, viewer *fileViewerState, token markdownRenderToken, size unit.Sp, baseWeight font.Weight, colors markdownColors) layout.Dimensions {
	labelWidget := func(gtx layout.Context) layout.Dimensions {
		label := material.Body1(th, token.text)
		label.Font.Typeface = ui.viewerTypeface()
		label.Font.Weight = baseWeight
		label.TextSize = size
		label.Color = colors.text
		if token.inline.bold {
			label.Font.Weight = font.Bold
		}
		if token.inline.italic {
			label.Font.Style = font.Italic
		}
		if token.inline.link != "" {
			label.Color = colors.link
		}
		if token.inline.code || token.inline.image != "" {
			label.Font.Typeface = ui.viewerMonospaceTypeface()
			label.TextSize = unit.Sp(float32(size) * 0.92)
			if token.inline.image != "" {
				label.Color = colors.muted
			}
			return fillRoundedBox(gtx, gtx.Dp(unit.Dp(3)), colors.codeBg, colors.codeBorder, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: markdownSpaceXS, Right: markdownSpaceXS, Top: 2, Bottom: 2}.Layout(gtx, label.Layout)
			})
		}
		return label.Layout(gtx)
	}

	var link *markdownLinkState
	content := layout.Widget(labelWidget)
	if token.inline.link != "" {
		link = ui.markdownLink(&viewer.markdown, token.key, token.inline.link)
		content = func(gtx layout.Context) layout.Dimensions {
			return link.click.Layout(gtx, labelWidget)
		}
	}
	record := op.Record(gtx.Ops)
	dims := content(gtx)
	call := record.Stop()
	call.Add(gtx.Ops)
	if link != nil && link.click.Clicked(gtx) {
		ui.handleMarkdownLinkClick(viewer, link.destination)
	}
	lineH := max(1, gtx.Dp(unit.Dp(1)))
	if token.inline.link != "" {
		y := max(0, dims.Size.Y-lineH)
		paint.FillShape(gtx.Ops, colors.link, clip.Rect(image.Rect(0, y, dims.Size.X, y+lineH)).Op())
	}
	if token.inline.strike {
		y := dims.Size.Y / 2
		paint.FillShape(gtx.Ops, colors.muted, clip.Rect(image.Rect(0, y, dims.Size.X, y+lineH)).Op())
	}
	return dims
}
