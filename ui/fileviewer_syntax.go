// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"image/color"
	"path/filepath"
	"strings"
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

func viewerBuildSyntaxDocument(ctx context.Context, path, content string) viewerSyntaxDocument {
	if strings.TrimSpace(content) == "" || viewerTotalLines(content) > viewerSyntaxMaxLines {
		return viewerSyntaxDocument{}
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

func viewerShouldBuildSyntax(mode string, info viewerReadInfo, content string) bool {
	return normalizeViewerMode(mode) == "file" &&
		!info.imagePreview &&
		!info.binaryPreview &&
		strings.TrimSpace(content) != ""
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
