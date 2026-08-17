// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"image/color"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	chroma "github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

const (
	viewerSyntaxAnalyseBytes = 64 << 10
	viewerSyntaxMaxLines     = 20000
	viewerSyntaxMaxSpans     = 120000
)

type viewerSyntaxRole uint8

const (
	viewerSyntaxText viewerSyntaxRole = iota
	viewerSyntaxComment
	viewerSyntaxKeyword
	viewerSyntaxType
	viewerSyntaxFunction
	viewerSyntaxBuiltin
	viewerSyntaxVariable
	viewerSyntaxString
	viewerSyntaxNumber
	viewerSyntaxOperator
	viewerSyntaxPunctuation
	viewerSyntaxTag
	viewerSyntaxAttribute
	viewerSyntaxEscape
	viewerSyntaxPreproc
	viewerSyntaxError
)

type viewerSyntaxSpan struct {
	role      viewerSyntaxRole
	byteStart int
	byteEnd   int
	colStart  int
	colEnd    int
}

type viewerSyntaxLine struct {
	spans []viewerSyntaxSpan
}

type viewerSyntaxDocument struct {
	lines []viewerSyntaxLine
}

func (doc viewerSyntaxDocument) ready() bool {
	return len(doc.lines) > 0
}

func (v *streamOutputView) setSyntax(doc viewerSyntaxDocument) {
	v.syntax = doc
}

func (v *streamOutputView) clearSyntax() {
	v.syntax = viewerSyntaxDocument{}
}

func (v *streamOutputView) syntaxLine(line int) ([]viewerSyntaxSpan, bool) {
	if v == nil || line < 0 || line >= len(v.syntax.lines) {
		return nil, false
	}
	return v.syntax.lines[line].spans, true
}

// viewerPreserveSyntaxAfterEdit keeps unaffected highlighting visible while a
// debounced full-document reparse is pending. The edited line is adjusted for
// a single-line replacement; multiline edits temporarily render only their
// changed lines as plain text.
func viewerPreserveSyntaxAfterEdit(doc viewerSyntaxDocument, oldChangedLines []string, startLine, endLine, startLocal, endLocal int, replacement string, newLines []string) viewerSyntaxDocument {
	if !doc.ready() || startLine < 0 || endLine < startLine || startLine > len(doc.lines) {
		return viewerSyntaxDocument{}
	}
	affected := strings.Count(replacement, "\n") + 1
	if affected < 1 || startLine+affected > len(newLines) {
		return viewerSyntaxDocument{}
	}
	if len(oldChangedLines) == 1 && affected == 1 && len(doc.lines) == len(newLines) && startLine < len(doc.lines) {
		doc.lines[startLine] = viewerAdjustSyntaxLineForEdit(doc.lines[startLine], oldChangedLines[0], startLocal, endLocal, replacement)
		return doc
	}
	next := viewerSyntaxDocument{lines: make([]viewerSyntaxLine, len(newLines))}
	copy(next.lines[:startLine], doc.lines[:min(startLine, len(doc.lines))])

	oldSuffix := endLine + 1
	newSuffix := startLine + affected
	if oldSuffix < len(doc.lines) && newSuffix < len(next.lines) {
		copy(next.lines[newSuffix:], doc.lines[oldSuffix:])
	}
	for i := startLine; i < newSuffix; i++ {
		next.lines[i] = viewerPlainSyntaxLine(newLines[i])
	}
	return next
}

func viewerAdjustSyntaxLineForEdit(line viewerSyntaxLine, oldText string, startByte, endByte int, replacement string) viewerSyntaxLine {
	if startByte < 0 || endByte < startByte || endByte > len(oldText) || strings.Contains(replacement, "\n") {
		return viewerPlainSyntaxLine(oldText[:max(0, min(len(oldText), startByte))] + replacement + oldText[max(0, min(len(oldText), endByte)):])
	}
	oldRunes := []rune(oldText)
	roles := make([]viewerSyntaxRole, len(oldRunes))
	for _, span := range line.spans {
		from := max(0, min(len(roles), span.colStart))
		to := max(from, min(len(roles), span.colEnd))
		for i := from; i < to; i++ {
			roles[i] = span.role
		}
	}
	startCol := utf8.RuneCountInString(oldText[:startByte])
	endCol := utf8.RuneCountInString(oldText[:endByte])
	replacementRunes := []rune(replacement)
	replacementRole := viewerSyntaxText
	if startCol > 0 && startCol-1 < len(roles) {
		replacementRole = roles[startCol-1]
	}
	if replacementRole == viewerSyntaxText && endCol < len(roles) {
		replacementRole = roles[endCol]
	}
	if strings.TrimSpace(replacement) == "" {
		replacementRole = viewerSyntaxText
	}
	newRoles := make([]viewerSyntaxRole, 0, len(roles)-(endCol-startCol)+len(replacementRunes))
	newRoles = append(newRoles, roles[:startCol]...)
	for range replacementRunes {
		newRoles = append(newRoles, replacementRole)
	}
	newRoles = append(newRoles, roles[endCol:]...)
	newText := oldText[:startByte] + replacement + oldText[endByte:]
	return viewerSyntaxLineFromRoles(newText, newRoles)
}

func viewerPlainSyntaxLine(text string) viewerSyntaxLine {
	return viewerSyntaxLineFromRoles(text, make([]viewerSyntaxRole, utf8.RuneCountInString(text)))
}

func viewerSyntaxLineFromRoles(text string, roles []viewerSyntaxRole) viewerSyntaxLine {
	if text == "" {
		return viewerSyntaxLine{}
	}
	runes := []rune(text)
	if len(roles) != len(runes) {
		roles = make([]viewerSyntaxRole, len(runes))
	}
	var line viewerSyntaxLine
	byteStart := 0
	for colStart := 0; colStart < len(runes); {
		role := roles[colStart]
		colEnd := colStart + 1
		for colEnd < len(runes) && roles[colEnd] == role {
			colEnd++
		}
		byteEnd := byteStart + len(string(runes[colStart:colEnd]))
		viewerSyntaxAppendSpan(&line, viewerSyntaxSpan{role: role, byteStart: byteStart, byteEnd: byteEnd, colStart: colStart, colEnd: colEnd})
		byteStart = byteEnd
		colStart = colEnd
	}
	return line
}

func viewerBuildSyntaxDocument(ctx context.Context, path, content string) viewerSyntaxDocument {
	if strings.TrimSpace(content) == "" || viewerTotalLines(content) > viewerSyntaxMaxLines {
		return viewerSyntaxDocument{}
	}
	if doc := viewerBuildStructuredLogSyntaxDocument(path, content); doc.ready() {
		return doc
	}
	lexer := viewerSyntaxLexer(path, content)
	if lexer == nil {
		return viewerSyntaxDocument{}
	}
	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, content)
	if err != nil {
		return viewerSyntaxDocument{}
	}

	wantLines := viewerTotalLines(content)
	if wantLines < 1 {
		wantLines = 1
	}
	doc := viewerSyntaxDocument{
		lines: make([]viewerSyntaxLine, 1, wantLines),
	}

	line := 0
	bytePos := 0
	colPos := 0
	totalSpans := 0
	checkCountdown := 128

	for {
		if ctx != nil {
			checkCountdown--
			if checkCountdown <= 0 {
				checkCountdown = 128
				if err := ctx.Err(); err != nil {
					return viewerSyntaxDocument{}
				}
			}
		}
		tok := iterator()
		if tok == chroma.EOF {
			break
		}
		value := tok.Value
		if value == "" {
			continue
		}
		role := viewerSyntaxRoleForToken(tok.Type)
		for len(value) > 0 {
			split := strings.IndexByte(value, '\n')
			part := value
			hasNewline := false
			if split >= 0 {
				part = value[:split]
				hasNewline = true
			}
			if part != "" {
				endByte := bytePos + len(part)
				endCol := colPos + utf8.RuneCountInString(part)
				if viewerSyntaxAppendSpan(&doc.lines[line], viewerSyntaxSpan{
					role:      role,
					byteStart: bytePos,
					byteEnd:   endByte,
					colStart:  colPos,
					colEnd:    endCol,
				}) {
					totalSpans++
					if totalSpans > viewerSyntaxMaxSpans {
						return viewerSyntaxDocument{}
					}
				}
				bytePos = endByte
				colPos = endCol
			}
			if !hasNewline {
				break
			}
			line++
			if line >= len(doc.lines) {
				doc.lines = append(doc.lines, viewerSyntaxLine{})
			}
			bytePos = 0
			colPos = 0
			value = value[split+1:]
		}
	}

	switch {
	case len(doc.lines) > wantLines:
		doc.lines = doc.lines[:wantLines]
	case len(doc.lines) < wantLines:
		doc.lines = append(doc.lines, make([]viewerSyntaxLine, wantLines-len(doc.lines))...)
	}
	if !doc.ready() {
		return viewerSyntaxDocument{}
	}
	return doc
}

func viewerBuildLiveSyntaxDocument(path, content string) viewerSyntaxDocument {
	if strings.TrimSpace(content) == "" || viewerTotalLines(content) > viewerSyntaxMaxLines {
		return viewerSyntaxDocument{}
	}
	return viewerBuildStructuredLogSyntaxDocument(path, content)
}

func viewerApplyLiveSyntax(st *fileViewerState) {
	if st == nil || !viewerModeSupportsSyntax(st.mode) || st.detectedImagePreview || st.detectedBinaryPreview {
		return
	}
	if syntax := viewerBuildLiveSyntaxDocument(st.path, st.content); syntax.ready() {
		st.stream.setSyntax(syntax)
	}
}

func viewerShouldBuildSyntax(mode string, info viewerReadInfo, content string) bool {
	return viewerModeSupportsSyntax(mode) &&
		!info.imagePreview &&
		!info.binaryPreview &&
		strings.TrimSpace(content) != ""
}

func viewerModeSupportsSyntax(mode string) bool {
	switch normalizeViewerMode(mode) {
	case "file", "command":
		return true
	default:
		return false
	}
}

func viewerSyntaxAppendSpan(line *viewerSyntaxLine, span viewerSyntaxSpan) bool {
	if line == nil || span.byteEnd <= span.byteStart || span.colEnd <= span.colStart {
		return false
	}
	if n := len(line.spans); n > 0 {
		last := &line.spans[n-1]
		if last.role == span.role && last.byteEnd == span.byteStart && last.colEnd == span.colStart {
			last.byteEnd = span.byteEnd
			last.colEnd = span.colEnd
			return false
		}
	}
	line.spans = append(line.spans, span)
	return true
}

func viewerSyntaxLexer(path, content string) chroma.Lexer {
	matchName := strings.TrimSpace(path)
	if matchName == "" {
		matchName = "view"
	}
	// An explicit plain-text extension is a stronger signal than Chroma's
	// content analyser. Without this guard, code-like prose or logs saved as
	// .txt can be guessed as an unrelated language and colored unpredictably.
	if viewerPathIsExplicitPlainText(matchName) {
		return nil
	}
	lexer := lexers.Match(matchName)
	if viewerSyntaxIsFallbackLexer(lexer) {
		lexer = nil
	}
	if lexer == nil {
		base := filepath.Base(matchName)
		if base != matchName {
			lexer = lexers.Match(base)
			if viewerSyntaxIsFallbackLexer(lexer) {
				lexer = nil
			}
		}
	}
	if lexer == nil && len(content) <= viewerSyntaxAnalyseBytes {
		lexer = lexers.Analyse(content)
		if viewerSyntaxIsFallbackLexer(lexer) {
			lexer = nil
		}
	}
	return lexer
}

func viewerPathIsExplicitPlainText(path string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case ".txt", ".text":
		return true
	default:
		return false
	}
}

func viewerSyntaxIsFallbackLexer(lexer chroma.Lexer) bool {
	if lexer == nil {
		return true
	}
	cfg := lexer.Config()
	if cfg == nil {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(cfg.Name))
	return name == "" || name == "fallback" || name == "plaintext"
}

func viewerSyntaxRoleForToken(tt chroma.TokenType) viewerSyntaxRole {
	switch {
	case tt == chroma.Error:
		return viewerSyntaxError
	case tt == chroma.CommentPreproc || tt.InSubCategory(chroma.CommentPreproc):
		return viewerSyntaxPreproc
	case tt.InCategory(chroma.Comment):
		return viewerSyntaxComment
	case tt == chroma.NameTag:
		return viewerSyntaxTag
	case tt == chroma.NameAttribute || tt == chroma.NameDecorator:
		return viewerSyntaxAttribute
	case tt == chroma.LiteralStringEscape || tt == chroma.LiteralStringAffix || tt == chroma.LiteralStringInterpol:
		return viewerSyntaxEscape
	case tt == chroma.KeywordType || tt == chroma.KeywordDeclaration || tt.InSubCategory(chroma.KeywordType):
		return viewerSyntaxType
	case tt.InCategory(chroma.Keyword):
		return viewerSyntaxKeyword
	case tt == chroma.NameFunction || tt == chroma.NameFunctionMagic || tt.InSubCategory(chroma.NameFunction):
		return viewerSyntaxFunction
	case tt == chroma.NameBuiltin || tt == chroma.NameBuiltinPseudo || tt.InSubCategory(chroma.NameBuiltin):
		return viewerSyntaxBuiltin
	case tt == chroma.NameVariable || tt == chroma.NameVariableClass || tt == chroma.NameVariableGlobal || tt == chroma.NameVariableInstance || tt == chroma.NameVariableMagic || tt.InSubCategory(chroma.NameVariable):
		return viewerSyntaxVariable
	case tt == chroma.NameClass || tt == chroma.NameNamespace || tt == chroma.NameException || tt == chroma.NameConstant || tt == chroma.NameEntity || tt == chroma.NameLabel:
		return viewerSyntaxType
	case tt.InSubCategory(chroma.LiteralString):
		return viewerSyntaxString
	case tt.InSubCategory(chroma.LiteralNumber):
		return viewerSyntaxNumber
	case tt.InCategory(chroma.Operator):
		return viewerSyntaxOperator
	case tt == chroma.Punctuation:
		return viewerSyntaxPunctuation
	default:
		return viewerSyntaxText
	}
}

func viewerBuildStructuredLogSyntaxDocument(path, content string) viewerSyntaxDocument {
	if !viewerLooksLikeStructuredLog(path, content) {
		return viewerSyntaxDocument{}
	}
	lines := splitStreamLines(content)
	doc := viewerSyntaxDocument{lines: make([]viewerSyntaxLine, len(lines))}
	totalSpans := 0
	matchedLines := 0
	for i, line := range lines {
		spans, ok := viewerBuildStructuredLogSyntaxLine(line)
		if !ok {
			continue
		}
		doc.lines[i].spans = spans
		totalSpans += len(spans)
		if totalSpans > viewerSyntaxMaxSpans {
			return viewerSyntaxDocument{}
		}
		matchedLines++
	}
	if matchedLines == 0 {
		return viewerSyntaxDocument{}
	}
	return doc
}

func viewerLooksLikeStructuredLog(path, content string) bool {
	strongPath := viewerPathLooksLikeLog(path)
	strongLines := 0
	structuredLines := 0
	sampledLines := 0
	for _, raw := range splitStreamLines(content) {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		sampledLines++
		sig := viewerStructuredLogLineSignature(line)
		if sig.strong() {
			strongLines++
		}
		if sig.structured() {
			structuredLines++
		}
		if sampledLines >= 32 {
			break
		}
	}
	switch {
	case strongLines >= 2:
		return true
	case strongPath && strongLines >= 1:
		return true
	case strongPath && structuredLines >= 2:
		return true
	default:
		return false
	}
}

type viewerStructuredLogSignature struct {
	timestamp bool
	level     bool
	bracket   bool
	fields    bool
	hex       bool
}

func (sig viewerStructuredLogSignature) structured() bool {
	return sig.timestamp && (sig.level || sig.fields || sig.bracket || sig.hex)
}

func (sig viewerStructuredLogSignature) strong() bool {
	return sig.timestamp && (sig.level || sig.fields || sig.hex)
}

func viewerStructuredLogLineSignature(line string) viewerStructuredLogSignature {
	sig := viewerStructuredLogSignature{}
	if viewerMatchLogTimestamp(line) == 0 {
		return sig
	}
	sig.timestamp = true
	rest := line[viewerMatchLogTimestamp(line):]
	rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
	if levelLen, _, _ := viewerMatchLogLevel(rest); levelLen > 0 {
		sig.level = true
	}
	sig.bracket = strings.Contains(line, "[") && strings.Contains(line, "]")
	sig.hex = strings.Contains(line, "HEX:")
	sig.fields = viewerLogHasKeyValue(line)
	return sig
}

func viewerPathLooksLikeLog(path string) bool {
	name := strings.ToLower(strings.TrimSpace(filepath.Base(path)))
	switch {
	case strings.HasSuffix(name, ".log"),
		strings.Contains(name, ".log."),
		strings.HasSuffix(name, ".out"),
		strings.Contains(name, ".out."),
		strings.HasSuffix(name, ".trace"),
		strings.Contains(name, "journal"):
		return true
	default:
		return false
	}
}

func viewerLogHasKeyValue(line string) bool {
	for i := 0; i < len(line); i++ {
		if !viewerLogKeyStart(line[i]) {
			continue
		}
		j := i + 1
		for j < len(line) && viewerLogKeyChar(line[j]) {
			j++
		}
		if j > i && j < len(line) && (line[j] == ':' || line[j] == '=') {
			return true
		}
		i = j
	}
	return false
}

func viewerBuildStructuredLogSyntaxLine(line string) ([]viewerSyntaxSpan, bool) {
	prefixLen := viewerMatchLogTimestamp(line)
	if prefixLen == 0 {
		return nil, false
	}

	builder := viewerStructuredLogLineBuilder{
		line:  line,
		spans: make([]viewerSyntaxSpan, 0, 16),
	}
	builder.emit(prefixLen, viewerSyntaxComment)
	for builder.pos < len(line) {
		if n := viewerConsumeSpaces(line[builder.pos:]); n > 0 {
			builder.emit(n, viewerSyntaxText)
			if levelLen, punctLen, role := viewerMatchLogLevel(line[builder.pos:]); levelLen > 0 {
				builder.emit(levelLen, role)
				if punctLen > 0 {
					builder.emit(punctLen, viewerSyntaxPunctuation)
				}
			}
			continue
		}
		if n := viewerMatchLogTimestamp(line[builder.pos:]); n > 0 {
			builder.emit(n, viewerSyntaxComment)
			continue
		}
		if n, role := viewerMatchLogBracket(line[builder.pos:]); n > 0 {
			if n >= 2 {
				builder.emit(1, viewerSyntaxPunctuation)
				builder.emit(n-2, role)
				builder.emit(1, viewerSyntaxPunctuation)
			} else {
				builder.emit(n, role)
			}
			continue
		}
		if n := viewerMatchQuotedString(line[builder.pos:]); n > 0 {
			builder.emit(n, viewerSyntaxString)
			continue
		}
		if strings.HasPrefix(line[builder.pos:], "--") {
			builder.emit(2, viewerSyntaxOperator)
			continue
		}
		if keyLen, sepLen := viewerMatchLogKey(line[builder.pos:]); keyLen > 0 {
			builder.emit(keyLen, viewerSyntaxAttribute)
			if sepLen > 0 {
				role := viewerSyntaxPunctuation
				if line[builder.pos] == '=' {
					role = viewerSyntaxOperator
				}
				builder.emit(sepLen, role)
			}
			continue
		}
		if n, role := viewerMatchLogValueToken(line[builder.pos:]); n > 0 {
			builder.emit(n, role)
			continue
		}
		if viewerLogWordStart(line[builder.pos]) {
			builder.emit(viewerConsumeLogWord(line[builder.pos:]), viewerSyntaxText)
			continue
		}
		if n := viewerConsumePunctuation(line[builder.pos:]); n > 0 {
			builder.emit(n, viewerSyntaxText)
			continue
		}
		builder.emit(1, viewerSyntaxText)
	}
	return builder.spans, true
}

type viewerStructuredLogLineBuilder struct {
	line  string
	pos   int
	col   int
	spans []viewerSyntaxSpan
}

func (b *viewerStructuredLogLineBuilder) emit(n int, role viewerSyntaxRole) {
	if b == nil || n <= 0 || b.pos >= len(b.line) {
		return
	}
	end := b.pos + n
	if end > len(b.line) {
		end = len(b.line)
	}
	if end <= b.pos {
		return
	}
	part := b.line[b.pos:end]
	nextCol := b.col + utf8.RuneCountInString(part)
	b.spans = viewerAppendStructuredLogSpan(b.spans, viewerSyntaxSpan{
		role:      role,
		byteStart: b.pos,
		byteEnd:   end,
		colStart:  b.col,
		colEnd:    nextCol,
	})
	b.pos = end
	b.col = nextCol
}

func viewerAppendStructuredLogSpan(spans []viewerSyntaxSpan, span viewerSyntaxSpan) []viewerSyntaxSpan {
	if span.byteEnd <= span.byteStart || span.colEnd <= span.colStart {
		return spans
	}
	if n := len(spans); n > 0 {
		last := &spans[n-1]
		if last.role == span.role && last.byteEnd == span.byteStart && last.colEnd == span.colStart {
			last.byteEnd = span.byteEnd
			last.colEnd = span.colEnd
			return spans
		}
	}
	return append(spans, span)
}

func viewerConsumeSpaces(s string) int {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

func viewerMatchLogLevel(s string) (int, int, viewerSyntaxRole) {
	i := 0
	for i < len(s) && s[i] >= 'A' && s[i] <= 'Z' {
		i++
	}
	if i == 0 {
		return 0, 0, viewerSyntaxText
	}
	level := s[:i]
	role, ok := viewerSyntaxRoleForLogLevel(level)
	if !ok {
		return 0, 0, viewerSyntaxText
	}
	punctLen := 0
	if i < len(s) && s[i] == ':' {
		punctLen = 1
	}
	return i, punctLen, role
}

func viewerSyntaxRoleForLogLevel(level string) (viewerSyntaxRole, bool) {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "TRACE":
		return viewerSyntaxComment, true
	case "DEBUG":
		return viewerSyntaxBuiltin, true
	case "INFO", "NOTICE":
		return viewerSyntaxKeyword, true
	case "WARN", "WARNING":
		return viewerSyntaxPreproc, true
	case "ERROR", "ERR", "FATAL", "PANIC", "CRITICAL":
		return viewerSyntaxError, true
	default:
		return viewerSyntaxText, false
	}
}

func viewerMatchLogTimestamp(s string) int {
	if len(s) < len("2006-01-02 15:04:05") {
		return 0
	}
	if !viewerIsDigits(s, 0, 4) || len(s) < 5 || s[4] != '-' {
		return 0
	}
	if !viewerIsDigits(s, 5, 2) || len(s) < 8 || s[7] != '-' {
		return 0
	}
	if !viewerIsDigits(s, 8, 2) || len(s) < 11 {
		return 0
	}
	sep := s[10]
	if sep != ' ' && sep != 'T' {
		return 0
	}
	if !viewerIsDigits(s, 11, 2) || len(s) < 14 || s[13] != ':' {
		return 0
	}
	if !viewerIsDigits(s, 14, 2) || len(s) < 17 || s[16] != ':' {
		return 0
	}
	if !viewerIsDigits(s, 17, 2) {
		return 0
	}
	i := 19
	if i < len(s) && s[i] == '.' {
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == i+1 {
			return 0
		}
		i = j
	}
	if i < len(s) && (s[i] == 'Z' || s[i] == 'z') {
		i++
	}
	if i+6 <= len(s) && (s[i] == '+' || s[i] == '-') && viewerIsDigits(s, i+1, 2) && s[i+3] == ':' && viewerIsDigits(s, i+4, 2) {
		i += 6
	}
	return i
}

func viewerIsDigits(s string, start, count int) bool {
	if start < 0 || count <= 0 || start+count > len(s) {
		return false
	}
	for i := start; i < start+count; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func viewerMatchLogBracket(s string) (int, viewerSyntaxRole) {
	if s == "" || s[0] != '[' {
		return 0, viewerSyntaxText
	}
	end := strings.IndexByte(s, ']')
	if end <= 0 {
		return 0, viewerSyntaxText
	}
	content := strings.TrimSpace(s[1:end])
	role := viewerSyntaxVariable
	if content != "" && !strings.ContainsAny(content, " :<>") {
		role = viewerSyntaxTag
	}
	return end + 1, role
}

func viewerMatchQuotedString(s string) int {
	if s == "" {
		return 0
	}
	quote := s[0]
	if quote != '"' && quote != '\'' {
		return 0
	}
	escaped := false
	for i := 1; i < len(s); i++ {
		if escaped {
			escaped = false
			continue
		}
		if s[i] == '\\' {
			escaped = true
			continue
		}
		if s[i] == quote {
			return i + 1
		}
	}
	return len(s)
}

func viewerMatchLogKey(s string) (int, int) {
	if s == "" || !viewerLogKeyStart(s[0]) {
		return 0, 0
	}
	i := 1
	for i < len(s) && viewerLogKeyChar(s[i]) {
		i++
	}
	if i >= len(s) {
		return 0, 0
	}
	switch s[i] {
	case ':', '=':
		return i, 1
	default:
		return 0, 0
	}
}

func viewerLogKeyStart(b byte) bool {
	return b == '_' || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func viewerLogKeyChar(b byte) bool {
	return viewerLogKeyStart(b) || (b >= '0' && b <= '9') || b == '.' || b == '-'
}

func viewerMatchLogValueToken(s string) (int, viewerSyntaxRole) {
	if n := viewerMatchHexBytes(s); n > 0 {
		return n, viewerSyntaxString
	}
	if n := viewerMatchIPPort(s); n > 0 {
		return n, viewerSyntaxNumber
	}
	if n := viewerMatchSignedNumber(s); n > 0 {
		return n, viewerSyntaxNumber
	}
	return 0, viewerSyntaxText
}

func viewerMatchHexBytes(s string) int {
	if len(s) < 8 {
		return 0
	}
	i := 0
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		i = 2
	}
	start := i
	for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || (s[i] >= 'a' && s[i] <= 'f') || (s[i] >= 'A' && s[i] <= 'F')) {
		i++
	}
	n := i - start
	if n < 8 || n%2 != 0 {
		return 0
	}
	return i
}

func viewerMatchIPPort(s string) int {
	i := 0
	for part := 0; part < 4; part++ {
		digits := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
		if digits == 0 {
			return 0
		}
		if part < 3 {
			if i >= len(s) || s[i] != '.' {
				return 0
			}
			i++
		}
	}
	if i < len(s) && s[i] == ':' {
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j > i+1 {
			i = j
		}
	}
	return i
}

func viewerMatchSignedNumber(s string) int {
	if s == "" {
		return 0
	}
	i := 0
	if s[i] == '+' || s[i] == '-' {
		i++
	}
	startDigits := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	if i == startDigits {
		return 0
	}
	return i
}

func viewerLogWordStart(b byte) bool {
	return viewerLogKeyStart(b) || (b >= '0' && b <= '9')
}

func viewerConsumeLogWord(s string) int {
	i := 0
	for i < len(s) && viewerLogKeyChar(s[i]) {
		i++
	}
	if i == 0 {
		for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '[' && s[i] != ']' && s[i] != '"' && s[i] != '\'' {
			i++
		}
	}
	if i == 0 {
		return 1
	}
	return i
}

func viewerConsumePunctuation(s string) int {
	if s == "" {
		return 0
	}
	switch s[0] {
	case ',', '.', ';', '(', ')', '{', '}', '/', '\\', '<', '>', '|':
		return 1
	default:
		return 0
	}
}

func viewerSyntaxColor(theme fileViewerTheme, role viewerSyntaxRole) color.NRGBA {
	switch role {
	case viewerSyntaxComment:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Muted, color.NRGBA{R: 143, G: 177, B: 126, A: 255}, 0.58))
	case viewerSyntaxKeyword:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Text, color.NRGBA{R: 117, G: 167, B: 246, A: 255}, 0.74))
	case viewerSyntaxType:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Text, color.NRGBA{R: 100, G: 202, B: 212, A: 255}, 0.66))
	case viewerSyntaxFunction:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Text, color.NRGBA{R: 234, G: 196, B: 111, A: 255}, 0.68))
	case viewerSyntaxBuiltin:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Text, color.NRGBA{R: 191, G: 149, B: 244, A: 255}, 0.66))
	case viewerSyntaxVariable:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Text, color.NRGBA{R: 232, G: 165, B: 118, A: 255}, 0.52))
	case viewerSyntaxString:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Text, color.NRGBA{R: 135, G: 207, B: 138, A: 255}, 0.74))
	case viewerSyntaxNumber:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Text, color.NRGBA{R: 242, G: 159, B: 109, A: 255}, 0.7))
	case viewerSyntaxOperator:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Text, color.NRGBA{R: 218, G: 223, B: 231, A: 255}, 0.22))
	case viewerSyntaxPunctuation:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Text, theme.Muted, 0.28))
	case viewerSyntaxTag:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Text, color.NRGBA{R: 111, G: 183, B: 246, A: 255}, 0.7))
	case viewerSyntaxAttribute:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Text, color.NRGBA{R: 221, G: 184, B: 111, A: 255}, 0.64))
	case viewerSyntaxEscape:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Text, color.NRGBA{R: 250, G: 206, B: 129, A: 255}, 0.78))
	case viewerSyntaxPreproc:
		return viewerSyntaxTone(theme, mixNRGBA(theme.Text, color.NRGBA{R: 205, G: 142, B: 239, A: 255}, 0.68))
	case viewerSyntaxError:
		return theme.Error
	default:
		return theme.Text
	}
}

func viewerSyntaxTone(theme fileViewerTheme, c color.NRGBA) color.NRGBA {
	c.A = 0xFF
	if contrastScore(theme.PanelBg, c) < contrastScore(theme.PanelBg, theme.Text)*0.62 {
		c = mixNRGBA(theme.Text, c, 0.58)
		c.A = 0xFF
	}
	return c
}
