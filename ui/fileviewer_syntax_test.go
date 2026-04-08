// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"testing"
)

func TestViewerBuildSyntaxDocumentForGoSource(t *testing.T) {
	content := "package main\n\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n"
	doc := viewerBuildSyntaxDocument(context.Background(), "/tmp/main.go", content)
	if !doc.ready() {
		t.Fatal("expected syntax document for Go source")
	}
	if got, want := len(doc.lines), viewerTotalLines(content); got != want {
		t.Fatalf("syntax line count = %d, want %d", got, want)
	}
	if !syntaxLineHasNonTextSpan(doc, 0) {
		t.Fatal("expected highlighted spans on the package line")
	}
	if !syntaxLineHasRoleText(content, doc, 2, viewerSyntaxFunction, "main") {
		t.Fatal("expected function-name highlight on line 2")
	}
	if !syntaxLineHasRoleText(content, doc, 3, viewerSyntaxString, "\"hi\"") {
		t.Fatal("expected string highlight on line 3")
	}
}

func TestViewerBuildSyntaxDocumentSkipsPlaintextFallback(t *testing.T) {
	doc := viewerBuildSyntaxDocument(context.Background(), "/tmp/notes.txt", "just some plain text\nnothing fancy")
	if doc.ready() {
		t.Fatal("plain text fallback should not enable syntax highlighting")
	}
}

func TestStreamOutputViewSetContentClearsSyntax(t *testing.T) {
	v := streamOutputView{}
	v.setSyntax(viewerSyntaxDocument{
		lines: []viewerSyntaxLine{{spans: []viewerSyntaxSpan{{
			role:      viewerSyntaxKeyword,
			byteStart: 0,
			byteEnd:   4,
			colStart:  0,
			colEnd:    4,
		}}}},
	})
	v.SetContent("alpha")
	if v.syntax.ready() {
		t.Fatal("SetContent should clear stale syntax spans")
	}
}

func TestFileViewerClearSyntaxStateClearsCurrentAndPending(t *testing.T) {
	doc := viewerSyntaxDocument{
		lines: []viewerSyntaxLine{{spans: []viewerSyntaxSpan{{
			role:      viewerSyntaxKeyword,
			byteStart: 0,
			byteEnd:   4,
			colStart:  0,
			colEnd:    4,
		}}}},
	}
	st := fileViewerState{
		pendingSyntax:      doc,
		pendingSyntaxReady: true,
	}
	st.stream.setSyntax(doc)
	st.clearSyntaxState()
	if st.stream.syntax.ready() {
		t.Fatal("clearSyntaxState should clear visible syntax spans")
	}
	if st.pendingSyntax.ready() || st.pendingSyntaxReady {
		t.Fatal("clearSyntaxState should clear pending syntax state")
	}
}

func syntaxLineHasRoleText(content string, doc viewerSyntaxDocument, line int, role viewerSyntaxRole, want string) bool {
	lines := splitStreamLines(content)
	if line < 0 || line >= len(lines) || line >= len(doc.lines) {
		return false
	}
	lineText := lines[line]
	for _, span := range doc.lines[line].spans {
		if span.role != role {
			continue
		}
		if span.byteStart < 0 || span.byteEnd > len(lineText) || span.byteEnd <= span.byteStart {
			continue
		}
		if lineText[span.byteStart:span.byteEnd] == want {
			return true
		}
	}
	return false
}

func syntaxLineHasNonTextSpan(doc viewerSyntaxDocument, line int) bool {
	if line < 0 || line >= len(doc.lines) {
		return false
	}
	for _, span := range doc.lines[line].spans {
		if span.role != viewerSyntaxText {
			return true
		}
	}
	return false
}
