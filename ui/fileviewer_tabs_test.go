// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"hexone/fm"
)

const tabbedJSONFixture = "{\n\t\"auths\": {},\n\t\"credsStore\": \"desktop\",\n\t\"currentContext\": \"desktop-linux\",\n\t\"cliPluginsExtraDirs\": [\n\t\t\"/opt/homebrew/lib/docker/cli-plugins\"\n\t]\n}"

// assertSyntaxSpansTileLines checks the invariant the syntax painter relies on:
// spans are drawn instead of the raw line, so they must reproduce the line they
// belong to exactly. Spans that were built against a different representation of
// the text paint the wrong slices and silently drop the tail of every line.
func assertSyntaxSpansTileLines(t *testing.T, label string, doc viewerSyntaxDocument, lines []string) {
	t.Helper()
	if !doc.ready() {
		return
	}
	for i, line := range lines {
		spans, ok := spansForSyntaxLine(doc, i)
		if !ok || len(spans) == 0 {
			continue
		}
		var rebuilt strings.Builder
		next := 0
		for _, span := range spans {
			if span.byteStart != next || span.byteEnd > len(line) || span.byteEnd <= span.byteStart {
				t.Fatalf("%s: line %d span [%d:%d] does not tile %q (next=%d len=%d)",
					label, i, span.byteStart, span.byteEnd, line, next, len(line))
			}
			rebuilt.WriteString(line[span.byteStart:span.byteEnd])
			next = span.byteEnd
		}
		if rebuilt.String() != line {
			t.Fatalf("%s: line %d spans paint %q but the line is %q", label, i, rebuilt.String(), line)
		}
	}
}

func newTabbedJSONViewer(t *testing.T) (*UI, *fileViewerState) {
	t.Helper()
	display := sanitizeViewerContent(tabbedJSONFixture)
	doc := viewerBuildSyntaxDocument(context.Background(), "config.json", display)
	if !doc.ready() {
		t.Fatal("JSON fixture should produce syntax spans")
	}
	st := &fileViewerState{
		mode:             "file",
		path:             "config.json",
		editBaselineText: tabbedJSONFixture,
		editableContent:  tabbedJSONFixture,
		content:          display,
	}
	st.contentEditor.SetText(tabbedJSONFixture)
	st.stream.SetContent(display)
	st.stream.setSyntax(doc)
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	return ui, st
}

func TestFileViewerViewSyntaxTilesLines(t *testing.T) {
	_, st := newTabbedJSONViewer(t)
	assertSyntaxSpansTileLines(t, "view", st.stream.syntax, st.stream.lines)
}

func TestFileViewerLeavingEditKeepsViewSyntaxAligned(t *testing.T) {
	ui, st := newTabbedJSONViewer(t)
	now := time.Now()
	if !ui.startFileViewerEdit(now) {
		t.Fatalf("startFileViewerEdit failed: %s", st.status)
	}
	// Edit mode holds the raw bytes, so its syntax must be built from them.
	st.editSyntax = viewerBuildSyntaxDocument(context.Background(), st.path, st.virtualEditText())
	st.stream.setSyntax(st.editSyntax)
	assertSyntaxSpansTileLines(t, "edit", st.stream.syntax, st.stream.lines)

	ui.discardFileViewerChanges(st)
	if !ui.stopFileViewerEdit() {
		t.Fatal("stopFileViewerEdit failed")
	}
	if st.editMode {
		t.Fatal("viewer should have left edit mode")
	}
	assertSyntaxSpansTileLines(t, "view-after-edit", st.stream.syntax, st.stream.lines)
}

func TestFileViewerEditorLaysTabsOnViewerColumns(t *testing.T) {
	ui, st := newTabbedJSONViewer(t)
	viewCols := make([]int, len(st.stream.lines))
	for i := range st.stream.lines {
		viewCols[i] = st.stream.lineCols(i)
	}
	viewMax := st.stream.maxCols

	if !ui.startFileViewerEdit(time.Now()) {
		t.Fatalf("startFileViewerEdit failed: %s", st.status)
	}
	if len(st.stream.lines) != len(viewCols) {
		t.Fatalf("edit line count=%d want %d", len(st.stream.lines), len(viewCols))
	}
	for i := range st.stream.lines {
		if got := st.stream.lineCols(i); got != viewCols[i] {
			t.Fatalf("edit line %d occupies %d columns, viewer shows %d (line %q)",
				i, got, viewCols[i], st.stream.lines[i])
		}
	}
	if st.stream.maxCols != viewMax {
		t.Fatalf("edit maxCols=%d want %d", st.stream.maxCols, viewMax)
	}
}

func spansForSyntaxLine(doc viewerSyntaxDocument, line int) ([]viewerSyntaxSpan, bool) {
	if line < 0 || line >= len(doc.lines) {
		return nil, false
	}
	return doc.lines[line].spans, true
}

func TestViewerTabColumnConversionsRoundTrip(t *testing.T) {
	const line = "\t\"a\": [\t1]"
	cases := []struct {
		byteIdx int
		col     int
	}{
		{0, 0},   // before the leading tab
		{1, 4},   // the tab filled columns 0..3
		{4, 7},   // "a" plus its quotes
		{7, 10},  // just before the inner tab
		{8, 12},  // inner tab starts at column 10, next stop is 12
		{10, 14}, // end of line
	}
	for _, tc := range cases {
		if got := viewerDisplayColAtByte(line, tc.byteIdx, viewerTabColumns); got != tc.col {
			t.Fatalf("displayColAtByte(%d)=%d want %d", tc.byteIdx, got, tc.col)
		}
		if got := viewerByteAtDisplayCol(line, tc.col, viewerTabColumns); got != tc.byteIdx {
			t.Fatalf("byteAtDisplayCol(%d)=%d want %d", tc.col, got, tc.byteIdx)
		}
	}
	if got := viewerLineDisplayCols(line, viewerTabColumns); got != 14 {
		t.Fatalf("lineDisplayCols=%d want 14", got)
	}
	// Columns inside a tab resolve to the tab's own byte, never past it.
	for col := 1; col < 4; col++ {
		if got := viewerByteAtDisplayCol(line, col, viewerTabColumns); got != 0 {
			t.Fatalf("byteAtDisplayCol(%d)=%d want 0 (inside the leading tab)", col, got)
		}
	}
	if got := viewerExpandLineTabs(line, 0, viewerTabColumns); got != "    \"a\": [  1]" {
		t.Fatalf("expandLineTabs=%q", got)
	}
}

func TestViewerTabExpansionMatchesSanitizedText(t *testing.T) {
	for _, raw := range []string{
		"\tone\n\t\ttwo\n",
		"a\tb\tc",
		"\t\ta\tb",
		"no tabs at all",
	} {
		for i, line := range strings.Split(raw, "\n") {
			want := sanitizeViewerContent(line)
			if got := viewerExpandLineTabs(line, 0, viewerTabColumns); got != want {
				t.Fatalf("line %d of %q expands to %q, the viewer sanitizes it to %q", i, raw, got, want)
			}
		}
	}
}

func TestFileViewerEditorCaretAndSelectionUseDisplayColumns(t *testing.T) {
	ui, st := newTabbedJSONViewer(t)
	if !ui.startFileViewerEdit(time.Now()) {
		t.Fatalf("startFileViewerEdit failed: %s", st.status)
	}
	v := &st.stream
	// Line 1 is "\t\"auths\": {},": the quote after the tab sits on column 4.
	quoteOffset := v.lineByteStart(1) + 1
	from, to, ok := v.rangeColsForLine(1, quoteOffset, quoteOffset+7)
	if !ok || from != 4 || to != 11 {
		t.Fatalf(`selection of "auths" spans cols [%d:%d] ok=%v, want [4:11]`, from, to, ok)
	}
	// Line 5 is indented with two tabs, so its text starts on column 8.
	if got := v.colAtByte(v.lines[5], 2); got != 8 {
		t.Fatalf("second tab ends on column %d want 8", got)
	}
	if got := v.byteAtCol(v.lines[5], 8); got != 2 {
		t.Fatalf("column 8 resolves to byte %d want 2", got)
	}
}

func TestFileViewerEditorTypingKeepsTabColumns(t *testing.T) {
	ui, st := newTabbedJSONViewer(t)
	now := time.Now()
	if !ui.startFileViewerEdit(now) {
		t.Fatalf("startFileViewerEdit failed: %s", st.status)
	}
	v := &st.stream
	caret := v.lineByteStart(1)
	if !ui.replaceFileViewerVirtualText(st, caret, caret, "\t", now) {
		t.Fatal("inserting a tab failed")
	}
	if want := "\t\t\"auths\": {},"; v.lines[1] != want {
		t.Fatalf("line=%q want %q", v.lines[1], want)
	}
	if got := v.lineCols(1); got != 20 {
		t.Fatalf("line occupies %d columns want 20", got)
	}
	if v.maxCols < 20 {
		t.Fatalf("maxCols=%d did not account for the widened line", v.maxCols)
	}
	if got := sanitizeViewerContent(v.lines[1]); viewerLineDisplayCols(v.lines[1], viewerTabColumns) != len(got) {
		t.Fatalf("edited line lays out on %d columns but sanitizes to %d", v.lineCols(1), len(got))
	}
}

func TestFileViewerEditorWrapRowsUseDisplayColumns(t *testing.T) {
	ui, st := newTabbedJSONViewer(t)
	if !ui.startFileViewerEdit(time.Now()) {
		t.Fatalf("startFileViewerEdit failed: %s", st.status)
	}
	v := &st.stream
	v.wrapEnabled = true
	rows := v.wrapRowsForLine(5, v.lines[5], 20)
	if len(rows) < 2 {
		t.Fatalf("expected the long path line to wrap, got %d row(s)", len(rows))
	}
	if rows[0].from != 0 || rows[0].to > 20 {
		t.Fatalf("first row spans cols [%d:%d], want a span within the 20 column width", rows[0].from, rows[0].to)
	}
	last := rows[len(rows)-1]
	if last.to != v.lineCols(5) {
		t.Fatalf("last wrap row ends on column %d, line occupies %d", last.to, v.lineCols(5))
	}
}

func TestStreamLinePaintSpecScrolledIntoTab(t *testing.T) {
	v := &streamOutputView{tabCols: viewerTabColumns, textPad: 3}
	v.SetContent("\t\t\"deep\"\n")
	line := v.lines[0]
	if got := v.lineCols(0); got != 14 {
		t.Fatalf("line occupies %d columns want 14", got)
	}
	// Not scrolled: the tabs paint as the eight columns the viewer shows.
	text, x := v.linePaintSpec(line)
	if text != "        \"deep\"" || x != 3 {
		t.Fatalf("unscrolled paint spec=%q x=%d", text, x)
	}
	// Scrolled halfway into the first tab: the columns scrolled off the left are
	// dropped, so the quote still lands on display column 8.
	v.hCol = 2
	text, _ = v.linePaintSpec(line)
	if text != "      \"deep\"" {
		t.Fatalf("paint spec at hCol=2 is %q, want the first two columns dropped", text)
	}
	if quoteCol := 2 + strings.IndexByte(text, '"'); quoteCol != 8 {
		t.Fatalf("quote paints on column %d want 8", quoteCol)
	}
	// Scrolled past both tabs entirely.
	v.hCol = 8
	text, _ = v.linePaintSpec(line)
	if text != "\"deep\"" {
		t.Fatalf("paint spec at hCol=8 is %q", text)
	}
}

func TestStreamViewWithoutTabColsIsUnchanged(t *testing.T) {
	// The read-only viewer feeds pre-expanded text and must keep the plain
	// rune-index behaviour, tab helpers included.
	v := &streamOutputView{textPad: 3}
	v.SetContent("\tstill raw\n")
	if got := v.lineCols(0); got != 10 {
		t.Fatalf("lineCols=%d want the plain rune count 10", got)
	}
	if got := v.byteAtCol(v.lines[0], 3); got != 3 {
		t.Fatalf("byteAtCol=%d want the plain rune index 3", got)
	}
	text, _ := v.linePaintSpec(v.lines[0])
	if text != "\tstill raw" {
		t.Fatalf("paint spec=%q want the line unchanged", text)
	}
}

func TestFileViewerEditingTabIndentedFileWritesTabsBack(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(tabbedJSONFixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	content, _, errText, info := readViewerFile(path, fm.ViewerFileEncodingUTF8, 1<<20, time.Time{}, nil)
	if errText != "" {
		t.Fatalf("readViewerFile: %s", errText)
	}
	st := &fileViewerState{
		mode:             "file",
		path:             path,
		editBaselineText: info.editableText,
		editableContent:  info.editableText,
		content:          content,
	}
	st.contentEditor.SetText(info.editableText)
	st.stream.SetContent(content)
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = st
	now := time.Now()
	if !ui.startFileViewerEdit(now) {
		t.Fatalf("startFileViewerEdit failed: %s", st.status)
	}
	// Type at the start of an indented line; the tabs in front of it must survive.
	caret := st.stream.lineByteStart(1) + 1
	if !ui.replaceFileViewerVirtualText(st, caret, caret, "X", now) {
		t.Fatal("insert failed")
	}
	if !ui.startFileViewerSave(now) {
		t.Fatalf("startFileViewerSave failed: %s %s", st.status, st.err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for st.saving && time.Now().Before(deadline) {
		ui.pumpFileViewerSaveState(layout.Context{Ops: new(op.Ops), Now: time.Now()}, st)
		time.Sleep(2 * time.Millisecond)
	}
	if st.saving {
		t.Fatal("save did not complete")
	}
	if st.err != "" {
		t.Fatalf("save error: %s", st.err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := strings.Replace(tabbedJSONFixture, "\t\"auths\"", "\tX\"auths\"", 1)
	if string(saved) != want {
		t.Fatalf("saved file=%q want %q", saved, want)
	}
	if !strings.Contains(string(saved), "\t\t\"/opt/") {
		t.Fatal("nested tab indentation was not preserved")
	}
}

func TestFileViewerLeavingEditAfterTypingRebuildsSyntax(t *testing.T) {
	ui, st := newTabbedJSONViewer(t)
	now := time.Now()
	if !ui.startFileViewerEdit(now) {
		t.Fatalf("startFileViewerEdit failed: %s", st.status)
	}
	st.editSyntax = viewerBuildSyntaxDocument(context.Background(), st.path, st.virtualEditText())
	st.stream.setSyntax(st.editSyntax)
	st.editSyntaxDue = time.Time{}

	// Type a string value in; the pending rebuild has not run yet when F3 lands.
	caret := st.stream.lineByteStart(1) + 12
	if !ui.replaceFileViewerVirtualText(st, caret, caret, " \"x\"", now) {
		t.Fatal("insert failed")
	}
	if st.editSyntaxDue.IsZero() {
		t.Fatal("typing should have scheduled a syntax rebuild")
	}
	if !ui.stopFileViewerEdit() {
		t.Fatal("stopFileViewerEdit failed")
	}
	assertSyntaxSpansTileLines(t, "view-after-typing", st.stream.syntax, st.stream.lines)
	fresh := viewerBuildSyntaxDocument(context.Background(), st.path, st.content)
	if !fresh.ready() || len(fresh.lines) != len(st.stream.syntax.lines) {
		t.Fatalf("view syntax has %d lines, a fresh build has %d", len(st.stream.syntax.lines), len(fresh.lines))
	}
	for i := range fresh.lines {
		got, _ := st.stream.syntaxLine(i)
		if len(got) != len(fresh.lines[i].spans) {
			t.Fatalf("line %d has %d spans, a fresh build has %d", i, len(got), len(fresh.lines[i].spans))
		}
	}
}
