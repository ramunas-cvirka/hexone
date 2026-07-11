// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"hexone/fm"
)

const viewerStructuredLogSample = "2026-04-09 12:03:37  INFO: [e96bfd45: 5048] -- id: 111111111111762, time: 2026-04-09 09:03:35, lat: 54.90000, lon: 23.90000, speed: 32.39742, course: 90.0\n" +
	"2026-04-09 12:03:38  INFO: [e96bfd45: 5048 < 127.0.0.1:55977] [tcp] HEX: 68682a0102011111111111176210041c1e776966695f737369643a5465737441502c776966695f727373693a2d35300d0a\n" +
	"2026-04-09 12:03:38  INFO: [e96bfd45: 5048] -- id: 111111111111762, time: 2026-04-09 09:03:35, lat: 54.90000, lon: 23.90000, speed: 32.39742, course: 90.0, result: wifi_ssid:TestAP,wifi_rssi:-50"

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

func TestViewerBuildSyntaxDocumentNeverGuessesLanguageForTxt(t *testing.T) {
	content := "package main\n\nfunc main() { println(\"looks like Go\") }\n"
	doc := viewerBuildSyntaxDocument(context.Background(), "/tmp/pasted-snippet.txt", content)
	if doc.ready() {
		t.Fatal("an explicit .txt file should remain plain text even when its content resembles source code")
	}
}

func TestViewerBuildSyntaxDocumentForStructuredLogs(t *testing.T) {
	doc := viewerBuildSyntaxDocument(context.Background(), "/tmp/app.log", viewerStructuredLogSample)
	if !doc.ready() {
		t.Fatal("expected syntax document for structured log content")
	}
	if !syntaxLineHasRoleText(viewerStructuredLogSample, doc, 0, viewerSyntaxKeyword, "INFO") {
		t.Fatal("expected INFO log level highlight on line 0")
	}
	if !syntaxLineHasRoleText(viewerStructuredLogSample, doc, 0, viewerSyntaxAttribute, "lat") {
		t.Fatal("expected key highlight for lat on line 0")
	}
	if !syntaxLineHasRoleText(viewerStructuredLogSample, doc, 0, viewerSyntaxNumber, "54.90000") {
		t.Fatal("expected numeric highlight for lat value on line 0")
	}
	if !syntaxLineHasRoleText(viewerStructuredLogSample, doc, 1, viewerSyntaxAttribute, "HEX") {
		t.Fatal("expected HEX field highlight on line 1")
	}
	if !syntaxLineHasRoleText(viewerStructuredLogSample, doc, 1, viewerSyntaxString, "68682a0102011111111111176210041c1e776966695f737369643a5465737441502c776966695f727373693a2d35300d0a") {
		t.Fatal("expected hex payload highlight on line 1")
	}
}

func TestViewerShouldBuildSyntaxAllowsCommandMode(t *testing.T) {
	if !viewerShouldBuildSyntax("command", viewerReadInfo{}, viewerStructuredLogSample) {
		t.Fatal("expected command mode to allow syntax highlighting")
	}
}

func TestPumpFileViewerStateAppliesSyntaxInCommandMode(t *testing.T) {
	doc := viewerBuildSyntaxDocument(context.Background(), "/tmp/app.log", viewerStructuredLogSample)
	if !doc.ready() {
		t.Fatal("expected syntax document for command mode test")
	}

	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		mode:     "command",
		path:     "/tmp/app.log",
		resultCh: make(chan fileViewerResult, 1),
		seq:      1,
	}
	ui.fileViewer = st
	st.resultCh <- fileViewerResult{
		seq:         1,
		syntax:      doc,
		syntaxReady: true,
	}

	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
	}
	ui.pumpFileViewerState(gtx)

	if !st.stream.syntax.ready() {
		t.Fatal("expected command mode to accept syntax results")
	}
	if !syntaxLineHasRoleText(viewerStructuredLogSample, st.stream.syntax, 1, viewerSyntaxAttribute, "HEX") {
		t.Fatal("expected applied syntax document to preserve HEX highlight")
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
