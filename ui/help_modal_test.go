// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"strings"
	"testing"
	"time"

	"gioui.org/layout"
	"hexone/ui/widget/table"
)

func TestParseHelpDocumentBuildsSectionsFromMarkdown(t *testing.T) {
	raw := "# Hexone Help\n\n" +
		"Intro paragraph before sections.\n\n" +
		"## Files\n\n" +
		"Open files fast.\n\n" +
		"- F3 views\n" +
		"- F4 opens\n\n" +
		"## Viewer\n\n" +
		"### Tips\n\n" +
		"Use Esc to close overlays.\n\n" +
		"```text\n" +
		"tail -f app.log\n" +
		"```\n"
	doc := parseHelpDocument("HELP.md", raw)
	if doc.Title != "Hexone Help" {
		t.Fatalf("Title=%q want %q", doc.Title, "Hexone Help")
	}
	if len(doc.Sections) != 3 {
		t.Fatalf("len(Sections)=%d want 3", len(doc.Sections))
	}
	if doc.Sections[0].Title != "Overview" {
		t.Fatalf("Sections[0].Title=%q want %q", doc.Sections[0].Title, "Overview")
	}
	if doc.Sections[1].Title != "Files" {
		t.Fatalf("Sections[1].Title=%q want %q", doc.Sections[1].Title, "Files")
	}
	if got := doc.Sections[2].Blocks[0].Kind; got != helpBlockHeading {
		t.Fatalf("first Viewer block kind=%v want %v", got, helpBlockHeading)
	}
}

func TestFunctionBarHelpActionOpensHelpModal(t *testing.T) {
	ui := &UI{}
	if !ui.performFunctionBarAction(functionBarActionHelp, time.Now()) {
		t.Fatal("help action should be handled")
	}
	if ui.helpModal == nil {
		t.Fatal("help action should open the help modal")
	}
	if ui.helpModal.doc.Title == "" {
		t.Fatal("help modal should load a help document")
	}
}

func TestParseHelpInlineTokensDetectsCodeSegments(t *testing.T) {
	tokens := parseHelpInlineTokens("Press `F1` or `Ctrl+F` to open help and SSH.")
	if len(tokens) == 0 {
		t.Fatal("expected parsed tokens")
	}
	var codes []string
	for _, tok := range tokens {
		if tok.Code {
			codes = append(codes, tok.Text)
		}
	}
	if len(codes) != 2 {
		t.Fatalf("code token count=%d want 2", len(codes))
	}
	if codes[0] != "F1" || codes[1] != "Ctrl+F" {
		t.Fatalf("code tokens=%v want [F1 Ctrl+F]", codes)
	}
}

func TestHelpModalSetActiveSectionTracksAnimation(t *testing.T) {
	st := &helpModalState{
		doc: helpDocument{
			Sections: []helpSection{
				{Title: "Navigation"},
				{Title: "Viewer"},
				{Title: "Analyzer"},
			},
		},
		bodyList:      layout.List{Axis: layout.Vertical},
		activeSection: 0,
	}
	now := time.Now()
	if !st.setActiveSection(2, now) {
		t.Fatal("setActiveSection should report a change")
	}
	if st.activeSection != 2 {
		t.Fatalf("activeSection=%d want 2", st.activeSection)
	}
	if st.sectionPrev != 0 {
		t.Fatalf("sectionPrev=%d want 0", st.sectionPrev)
	}
	if st.sectionAnimAt != now {
		t.Fatalf("sectionAnimAt=%v want %v", st.sectionAnimAt, now)
	}
	if !st.wantKeyFocus {
		t.Fatal("setActiveSection should request keyboard focus")
	}
}

func TestHelpModalCodeSelectableReusesStatePerBlock(t *testing.T) {
	st := &helpModalState{}
	first := st.codeSelectable(1, 2)
	if first == nil {
		t.Fatal("codeSelectable should return state")
	}
	second := st.codeSelectable(1, 2)
	if second != first {
		t.Fatal("codeSelectable should reuse the same state for the same block")
	}
	other := st.codeSelectable(1, 3)
	if other == first {
		t.Fatal("different blocks should not share selectable state")
	}
}

func TestFallbackHelpMarkdownOmitsBundledHelpCopy(t *testing.T) {
	if strings.Contains(strings.ToLower(fallbackHelpMarkdown()), "bundled help") {
		t.Fatal("fallback help should not mention bundled help")
	}
}

func TestHelpInlineCodeChipModeMapsSpecialTokens(t *testing.T) {
	if mode, ok := helpInlineCodeChipMode("mode:full"); !ok || mode != table.ModeFull {
		t.Fatalf("mode:full = (%v, %v), want (%v, true)", mode, ok, table.ModeFull)
	}
	if mode, ok := helpInlineCodeChipMode("mode:brief"); !ok || mode != table.ModeBrief {
		t.Fatalf("mode:brief = (%v, %v), want (%v, true)", mode, ok, table.ModeBrief)
	}
	if _, ok := helpInlineCodeChipMode("mode:other"); ok {
		t.Fatal("mode:other should not map to a mode glyph")
	}
}

func TestAppendHelpPlainTextSectionAddsCodeSection(t *testing.T) {
	doc := helpDocument{
		Title: "Help",
		Sections: []helpSection{
			{Title: "Overview"},
		},
	}
	appendHelpPlainTextSection(&doc, "License", "line 1\nline 2\n")
	if len(doc.Sections) != 2 {
		t.Fatalf("len(Sections)=%d want 2", len(doc.Sections))
	}
	got := doc.Sections[1]
	if got.Title != "License" {
		t.Fatalf("section title=%q want License", got.Title)
	}
	if len(got.Blocks) != 1 {
		t.Fatalf("len(Blocks)=%d want 1", len(got.Blocks))
	}
	if got.Blocks[0].Kind != helpBlockCode || got.Blocks[0].Text != "line 1\nline 2" {
		t.Fatalf("code block=%+v want trimmed code block", got.Blocks[0])
	}
}

func TestAppendHelpPlainTextSectionSkipsEmptyContent(t *testing.T) {
	doc := helpDocument{Title: "Help"}
	appendHelpPlainTextSection(&doc, "Notice", " \n\t")
	if len(doc.Sections) != 0 {
		t.Fatalf("len(Sections)=%d want 0", len(doc.Sections))
	}
}
