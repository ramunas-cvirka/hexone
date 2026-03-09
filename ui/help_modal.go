package ui

import (
	"fmt"
	resources "hexone"
	"image"
	"image/color"
	"strings"

	uitheme "hexone/ui/theme"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type helpBlockKind uint8

const (
	helpBlockParagraph helpBlockKind = iota
	helpBlockBullets
	helpBlockCode
	helpBlockHeading
)

type helpBlock struct {
	Kind  helpBlockKind
	Text  string
	Items []string
	Level int
}

type helpSection struct {
	Title  string
	Blocks []helpBlock
}

type helpDocument struct {
	Title      string
	SourcePath string
	LoadErr    string
	Sections   []helpSection
}

type helpModalState struct {
	backdropClick widget.Clickable
	closeClick    widget.Clickable
	sectionClicks []widget.Clickable
	navList       layout.List
	bodyList      layout.List
	doc           helpDocument
	activeSection int
}

type helpInlineToken struct {
	Text string
	Code bool
}

func (ui *UI) openHelpModal() {
	if ui == nil {
		return
	}
	ui.closeFunctionBarToolsMenu()
	st := ui.helpModal
	if st == nil {
		st = &helpModalState{}
		st.navList.Axis = layout.Vertical
		st.bodyList.Axis = layout.Vertical
	}
	st.doc = loadHelpDocument()
	st.ensureSectionClicks(len(st.doc.Sections))
	if st.activeSection < 0 || st.activeSection >= len(st.doc.Sections) {
		st.activeSection = 0
	}
	st.bodyList = layout.List{Axis: layout.Vertical}
	ui.helpModal = st
}

func (ui *UI) closeHelpModal() {
	if ui == nil {
		return
	}
	ui.helpModal = nil
}

func (st *helpModalState) ensureSectionClicks(n int) {
	if st == nil || n <= 0 {
		return
	}
	if len(st.sectionClicks) >= n {
		return
	}
	st.sectionClicks = append(st.sectionClicks, make([]widget.Clickable, n-len(st.sectionClicks))...)
}

func loadHelpDocument() helpDocument {
	doc := parseHelpDocument(resources.EmbeddedHelpSource, resources.HelpMarkdown())
	if strings.TrimSpace(doc.Title) == "" {
		doc = parseHelpDocument(resources.EmbeddedHelpSource, fallbackHelpMarkdown())
		doc.LoadErr = fmt.Sprintf("embedded help content is empty")
	}
	return doc
}

func parseHelpDocument(path, raw string) helpDocument {
	doc := helpDocument{
		Title:      "Help",
		SourcePath: path,
	}

	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	current := helpSection{Title: "Overview"}
	var paragraph []string
	var bullets []string
	var code []string
	inCode := false

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		current.Blocks = append(current.Blocks, helpBlock{
			Kind: helpBlockParagraph,
			Text: strings.Join(paragraph, " "),
		})
		paragraph = nil
	}
	flushBullets := func() {
		if len(bullets) == 0 {
			return
		}
		items := make([]string, len(bullets))
		copy(items, bullets)
		current.Blocks = append(current.Blocks, helpBlock{
			Kind:  helpBlockBullets,
			Items: items,
		})
		bullets = nil
	}
	flushCode := func() {
		if len(code) == 0 {
			return
		}
		current.Blocks = append(current.Blocks, helpBlock{
			Kind: helpBlockCode,
			Text: strings.Join(code, "\n"),
		})
		code = nil
	}
	flushAll := func() {
		flushParagraph()
		flushBullets()
		flushCode()
	}
	pushSection := func() {
		flushAll()
		if strings.TrimSpace(current.Title) == "" {
			current.Title = "Overview"
		}
		if len(current.Blocks) == 0 {
			return
		}
		doc.Sections = append(doc.Sections, current)
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inCode {
				flushCode()
				inCode = false
			} else {
				flushParagraph()
				flushBullets()
				inCode = true
			}
			continue
		}
		if inCode {
			code = append(code, line)
			continue
		}
		if trimmed == "" {
			flushParagraph()
			flushBullets()
			continue
		}
		if level, title, ok := parseHelpHeading(trimmed); ok {
			flushAll()
			switch level {
			case 1:
				doc.Title = title
			case 2:
				pushSection()
				current = helpSection{Title: title}
			default:
				current.Blocks = append(current.Blocks, helpBlock{
					Kind:  helpBlockHeading,
					Text:  title,
					Level: level,
				})
			}
			continue
		}
		if item, ok := parseHelpBullet(trimmed); ok {
			flushParagraph()
			bullets = append(bullets, item)
			continue
		}
		paragraph = append(paragraph, trimmed)
	}

	pushSection()
	if len(doc.Sections) == 0 {
		doc.Sections = append(doc.Sections, helpSection{
			Title: "Overview",
			Blocks: []helpBlock{{
				Kind: helpBlockParagraph,
				Text: "No help content is available yet.",
			}},
		})
	}
	if strings.TrimSpace(doc.Title) == "" {
		doc.Title = "Help"
	}
	if doc.Sections[0].Title == "" {
		doc.Sections[0].Title = "Overview"
	}
	return doc
}

func parseHelpHeading(line string) (int, string, bool) {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}
	title := strings.TrimSpace(line[level:])
	if title == "" {
		return 0, "", false
	}
	return level, title, true
}

func parseHelpBullet(line string) (string, bool) {
	if len(line) < 3 {
		return "", false
	}
	switch line[0] {
	case '-', '*', '+':
		if line[1] == ' ' {
			item := strings.TrimSpace(line[2:])
			if item != "" {
				return item, true
			}
		}
	}
	return "", false
}

func fallbackHelpMarkdown() string {
	return "# Hexone Help\n\n" +
		"## Overview\n\n" +
		"Hexone is a keyboard-focused file manager with extra protocol and text tools.\n" +
		"This fallback help is bundled into the application binary.\n\n" +
		"## Basics\n\n" +
		"- Press F1 to open or close this help window.\n" +
		"- Press Esc to close dialogs or return to the file manager.\n" +
		"- Press F3 to open the viewer for the current selection.\n" +
		"- Press F4 to open the current file with the system association.\n\n" +
		"## Packaging\n\n" +
		"Portable builds only need the app binary plus protocols.yaml.\n\n" +
		"```text\n" +
		"protocols.yaml\n" +
		"```\n"
}

func (ui *UI) layoutHelpModal(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.helpModal
	if st == nil {
		return layout.Dimensions{}
	}

	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape},
			key.Filter{Name: key.NameF1},
		)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press || ke.Modifiers != 0 {
			continue
		}
		if ke.Name == key.NameEscape || ke.Name == key.NameF1 {
			ui.closeHelpModal()
			return layout.Dimensions{}
		}
	}

	for st.backdropClick.Clicked(gtx) {
	}
	if st.closeClick.Clicked(gtx) {
		ui.closeHelpModal()
		return layout.Dimensions{}
	}
	for i := range st.doc.Sections {
		for st.sectionClicks[i].Clicked(gtx) {
			st.activeSection = i
			st.bodyList = layout.List{Axis: layout.Vertical}
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	dims := st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{A: 150}, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

		width := gtx.Dp(unit.Dp(900))
		height := gtx.Dp(unit.Dp(560))
		maxW := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(18))
		maxH := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(18))
		if width > maxW {
			width = maxW
		}
		if height > maxH {
			height = maxH
		}
		if width < 560 {
			width = 560
		}
		if height < 360 {
			height = 360
		}

		m := op.Record(gtx.Ops)
		card := fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
			return minHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
				return fillRoundedBox(
					gtx,
					gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
					color.NRGBA{R: 17, G: 20, B: 27, A: 252},
					color.NRGBA{R: 255, G: 255, B: 255, A: 22},
					func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutHelpModalHeader(th, gtx, st)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return ui.layoutHelpModalBody(th, gtx, st)
								}),
							)
						})
					},
				)
			})
		})
		call := m.Stop()
		x := (gtx.Constraints.Max.X - card.Size.X) / 2
		y := (gtx.Constraints.Max.Y - card.Size.Y) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	return dims
}

func (ui *UI) layoutHelpModalHeader(th *material.Theme, gtx layout.Context, st *helpModalState) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body1(th, st.doc.Title)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.Font.Weight = font.Bold
					lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 12)
					lbl.Color = txtColor
					return lbl.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					text := "F1 or Esc closes this window"
					if st.doc.SourcePath != "" {
						text = st.doc.SourcePath + "   " + text
					}
					lbl := material.Caption(th, text)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 8)
					lbl.Color = hintColor
					return lbl.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutTinyIconModeButton(th, gtx, &st.closeClick, uitheme.CloseIcon(), false)
		}),
	)
}

func (ui *UI) layoutHelpModalBody(th *material.Theme, gtx layout.Context, st *helpModalState) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(220)), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutHelpSectionList(th, gtx, st)
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutHelpSectionContent(th, gtx, st)
		}),
	)
}

func (ui *UI) layoutHelpSectionList(th *material.Theme, gtx layout.Context, st *helpModalState) layout.Dimensions {
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
		color.NRGBA{R: 24, G: 28, B: 36, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "Sections")
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.Font.Weight = font.Medium
						lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 8)
						lbl.Color = hintColor
						return lbl.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return st.navList.Layout(gtx, len(st.doc.Sections), func(gtx layout.Context, index int) layout.Dimensions {
							return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutHelpSectionButton(th, gtx, st, index)
							})
						})
					}),
				)
			})
		},
	)
}

func (ui *UI) layoutHelpSectionButton(th *material.Theme, gtx layout.Context, st *helpModalState, index int) layout.Dimensions {
	active := index == st.activeSection
	bg := color.NRGBA{R: 30, G: 35, B: 46, A: 255}
	border := color.NRGBA{R: 255, G: 255, B: 255, A: 16}
	fg := txtColor
	if active {
		bg = color.NRGBA{R: 76, G: 88, B: 118, A: 255}
		border = color.NRGBA{R: 198, G: 212, B: 255, A: 118}
		fg = color.NRGBA{R: 246, G: 249, B: 255, A: 255}
	}
	return st.sectionClicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), bg, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, st.doc.Sections[index].Title)
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.Font.Weight = font.Medium
				lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
				lbl.Color = fg
				return lbl.Layout(gtx)
			})
		})
	})
}

func (ui *UI) layoutHelpSectionContent(th *material.Theme, gtx layout.Context, st *helpModalState) layout.Dimensions {
	if st.activeSection < 0 || st.activeSection >= len(st.doc.Sections) {
		return layout.Dimensions{}
	}
	section := st.doc.Sections[st.activeSection]
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
		color.NRGBA{R: 21, G: 24, B: 31, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body1(th, section.Title)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.Font.Weight = font.Bold
						lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 11)
						lbl.Color = txtColor
						return lbl.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if st.doc.LoadErr == "" {
							return layout.Dimensions{}
						}
						lbl := material.Caption(th, st.doc.LoadErr)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 8)
						lbl.Color = color.NRGBA{R: 226, G: 177, B: 109, A: 255}
						return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, lbl.Layout)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return st.bodyList.Layout(gtx, len(section.Blocks), func(gtx layout.Context, index int) layout.Dimensions {
							return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutHelpBlock(th, gtx, section.Blocks[index])
							})
						})
					}),
				)
			})
		},
	)
}

func (ui *UI) layoutHelpBlock(th *material.Theme, gtx layout.Context, block helpBlock) layout.Dimensions {
	switch block.Kind {
	case helpBlockHeading:
		return ui.layoutHelpInlineText(
			th,
			gtx,
			block.Text,
			scaleModalThemeFontSize(th, ui.fmCfg, 10),
			color.NRGBA{R: 228, G: 233, B: 244, A: 255},
			font.Bold,
		)
	case helpBlockBullets:
		children := make([]layout.FlexChild, 0, len(block.Items))
		for _, item := range block.Items {
			item := item
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							bullet := material.Body2(th, "•")
							bullet.Font.Typeface = ui.mainTypeface()
							bullet.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
							bullet.Color = color.NRGBA{R: 155, G: 193, B: 255, A: 255}
							return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, bullet.Layout)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return ui.layoutHelpInlineText(
								th,
								gtx,
								item,
								scaleModalThemeFontSize(th, ui.fmCfg, 9),
								txtColor,
								font.Normal,
							)
						}),
					)
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	case helpBlockCode:
		return fillRoundedBox(gtx, gtx.Dp(unit.Dp(filePaneControlCornerDp)), color.NRGBA{R: 14, G: 17, B: 22, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 16}, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(7), Bottom: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, block.Text)
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 8)
				lbl.Color = color.NRGBA{R: 197, G: 226, B: 255, A: 255}
				return lbl.Layout(gtx)
			})
		})
	default:
		return ui.layoutHelpInlineText(
			th,
			gtx,
			block.Text,
			scaleModalThemeFontSize(th, ui.fmCfg, 9),
			txtColor,
			font.Normal,
		)
	}
}

func parseHelpInlineTokens(text string) []helpInlineToken {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var out []helpInlineToken
	appendPlain := func(chunk string) {
		for _, field := range strings.Fields(chunk) {
			if field == "" {
				continue
			}
			out = append(out, helpInlineToken{Text: field})
		}
	}
	for len(text) > 0 {
		start := strings.IndexByte(text, '`')
		if start < 0 {
			appendPlain(text)
			break
		}
		appendPlain(text[:start])
		text = text[start+1:]
		end := strings.IndexByte(text, '`')
		if end < 0 {
			appendPlain(text)
			break
		}
		code := strings.TrimSpace(text[:end])
		if code != "" {
			out = append(out, helpInlineToken{Text: code, Code: true})
		}
		text = text[end+1:]
	}
	return out
}

func (ui *UI) layoutHelpInlineText(th *material.Theme, gtx layout.Context, text string, size unit.Sp, fg color.NRGBA, weight font.Weight) layout.Dimensions {
	tokens := parseHelpInlineTokens(text)
	if len(tokens) == 0 {
		return layout.Dimensions{}
	}
	spaceW := gtx.Dp(unit.Dp(5))
	if spaceW < 2 {
		spaceW = 2
	}
	maxW := gtx.Constraints.Max.X
	if maxW <= 0 {
		maxW = 1 << 20
	}

	rows := make([][]helpInlineToken, 1)
	rowW := 0
	for _, tok := range tokens {
		tokW := ui.measureHelpInlineToken(gtx, th, tok, size, weight).X
		nextW := tokW
		if len(rows[len(rows)-1]) > 0 {
			nextW += spaceW
		}
		if len(rows[len(rows)-1]) > 0 && rowW+nextW > maxW {
			rows = append(rows, nil)
			rowW = 0
			nextW = tokW
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], tok)
		rowW += nextW
	}

	children := make([]layout.FlexChild, 0, len(rows)*2)
	for i, row := range rows {
		row := row
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			rowChildren := make([]layout.FlexChild, 0, len(row)*2)
			for j, tok := range row {
				tok := tok
				if j > 0 {
					rowChildren = append(rowChildren, layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout))
				}
				rowChildren = append(rowChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if tok.Code {
						return ui.layoutHelpInlineCodeChip(th, gtx, tok.Text, size)
					}
					lbl := material.Body2(th, tok.Text)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.Font.Weight = weight
					lbl.TextSize = size
					lbl.Color = fg
					return lbl.Layout(gtx)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, rowChildren...)
		}))
		if i < len(rows)-1 {
			children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (ui *UI) measureHelpInlineToken(gtx layout.Context, th *material.Theme, tok helpInlineToken, size unit.Sp, weight font.Weight) image.Point {
	if tok.Code {
		return measureWidgetUnconstrained(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutHelpInlineCodeChip(th, gtx, tok.Text, size)
		}).Size
	}
	lbl := material.Body2(th, tok.Text)
	lbl.Font.Typeface = ui.mainTypeface()
	lbl.Font.Weight = weight
	lbl.TextSize = size
	return measureLabelUnconstrained(gtx, lbl).Size
}

func (ui *UI) layoutHelpInlineCodeChip(th *material.Theme, gtx layout.Context, text string, size unit.Sp) layout.Dimensions {
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(4)),
		color.NRGBA{R: 44, G: 53, B: 70, A: 255},
		color.NRGBA{R: 162, G: 186, B: 226, A: 78},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, text)
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.Font.Weight = font.Bold
				lbl.TextSize = size
				lbl.Color = color.NRGBA{R: 242, G: 247, B: 255, A: 255}
				return lbl.Layout(gtx)
			})
		},
	)
}
