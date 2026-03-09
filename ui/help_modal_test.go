package ui

import (
	"testing"
	"time"
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
