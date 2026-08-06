// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"image"
	"image/color"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const (
	terminalFindWidthDp      = 360
	terminalFindRowHeightDp  = 24
	terminalFindPreviewRowDp = 17
	terminalFindMaxRows      = 7
	terminalFindMaxResults   = 500
	terminalFindPreviewLines = 3
)

type terminalFindMatch struct {
	Row          int
	StartCol     int
	EndCol       int
	Line         string
	Preview      []string
	PreviewFocus int
}

type terminalFindResult struct {
	generation uint64
	matches    []terminalFindMatch
}

type terminalFindState struct {
	open         bool
	focus        bool
	editor       widget.Editor
	closeClick   widget.Clickable
	list         widget.List
	clicks       []widget.Clickable
	matches      []terminalFindMatch
	index        int
	previewIndex int
	previewAt    time.Time
	previewStart int
	previewEnd   int
	searching    bool
	generation   uint64
	results      chan terminalFindResult
}

func terminalFindKey(ev key.Event) bool {
	if ev.State != key.Press || (ev.Name != "F" && ev.Name != "f") {
		return false
	}
	return ev.Modifiers == key.ModCtrl || ev.Modifiers == key.ModShortcut
}

func (s *terminalSession) openFind(gtx layout.Context) {
	if s == nil {
		return
	}
	find := &s.find
	if find.results == nil {
		find.results = make(chan terminalFindResult, 16)
		find.editor.SingleLine = true
		find.editor.Submit = true
		find.list.Axis = layout.Vertical
		find.previewIndex = -1
	}
	find.open = true
	find.focus = true
	find.editor.SetCaret(0, find.editor.Len())
	gtx.Execute(key.FocusCmd{Tag: &find.editor})
	gtx.Execute(key.SoftKeyboardCmd{Show: true})
	if strings.TrimSpace(find.editor.Text()) != "" && len(find.matches) == 0 {
		s.startFindSearch(find.editor.Text())
	}
}

func (s *terminalSession) closeFind() bool {
	if s == nil || !s.find.open {
		return false
	}
	s.find.open = false
	s.find.focus = false
	s.find.searching = false
	s.find.generation++
	s.wantFocus = true
	return true
}

func (s *terminalSession) startFindSearch(query string) {
	if s == nil {
		return
	}
	find := &s.find
	find.generation++
	generation := find.generation
	query = strings.TrimSpace(query)
	find.index = 0
	find.previewIndex = -1
	find.previewAt = time.Time{}
	find.matches = nil
	find.clicks = nil
	if query == "" {
		find.searching = false
		return
	}
	find.searching = true
	previewStart, previewEnd := find.previewStart, find.previewEnd
	results := find.results
	go func() {
		lines := s.terminalFindLines()
		matches := terminalFindMatchesWithPreview(lines, query, terminalFindMaxResults, previewStart, previewEnd)
		results <- terminalFindResult{generation: generation, matches: matches}
		s.invalidateNow()
	}()
}

func (s *terminalSession) terminalFindLines() []string {
	if s == nil || s.term == nil {
		return nil
	}
	s.parserMu.Lock()
	defer s.parserMu.Unlock()
	rows := s.term.Rows()
	cols := s.term.Cols()
	scrollback := s.term.ScrollbackLen()
	alternate := s.term.IsAlternateScreen()
	count := rows
	if !alternate {
		count += scrollback
	}
	lines := make([]string, count)
	for row := range lines {
		lines[row] = s.virtualLineText(row, 0, cols-1, scrollback, alternate)
	}
	return lines
}

func terminalFindMatches(lines []string, query string, limit int) []terminalFindMatch {
	return terminalFindMatchesWithPreview(lines, query, limit, 0, 2)
}

func terminalFindMatchesWithPreview(lines []string, query string, limit, previewStart, previewEnd int) []terminalFindMatch {
	query = strings.TrimSpace(query)
	if query == "" || limit <= 0 {
		return nil
	}
	needle := []rune(strings.ToLower(query))
	matches := make([]terminalFindMatch, 0, min(len(lines), 32))
	for row, line := range lines {
		lower := []rune(strings.ToLower(line))
		from := 0
		for from <= len(lower) {
			at := terminalFindRuneIndex(lower[from:], needle)
			if at < 0 {
				break
			}
			at += from
			startCol := at
			endCol := startCol + len(needle) - 1
			preview, previewFocus := terminalFindPreviewWindowRange(lines, row, previewStart, previewEnd)
			matches = append(matches, terminalFindMatch{
				Row:          row,
				StartCol:     startCol,
				EndCol:       max(startCol, endCol),
				Line:         line,
				Preview:      preview,
				PreviewFocus: previewFocus,
			})
			if len(matches) >= limit {
				return matches
			}
			from = at + len(needle)
		}
	}
	return matches
}

func terminalFindRuneIndex(haystack, needle []rune) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		matched := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}

func terminalFindPreview(lines []string, row int) []string {
	preview, _ := terminalFindPreviewWindow(lines, row)
	return preview
}

func terminalFindPreviewWindow(lines []string, row int) ([]string, int) {
	return terminalFindPreviewWindowRange(lines, row, 0, 2)
}

func terminalFindPreviewWindowRange(lines []string, row, previewStart, previewEnd int) ([]string, int) {
	if row < 0 || row >= len(lines) {
		return nil, -1
	}
	previewStart, previewEnd = fm.NormalizeTerminalPreviewRange(previewStart, previewEnd)
	start := row + previewStart
	if start < 0 {
		start = 0
	}
	end := row + previewEnd + 1
	if end > len(lines) {
		end = len(lines)
	}
	return append([]string(nil), lines[start:end]...), row - start
}

func (s *terminalSession) setFindPreviewRange(start, end int) bool {
	if s == nil {
		return false
	}
	start, end = fm.NormalizeTerminalPreviewRange(start, end)
	if s.find.previewStart == start && s.find.previewEnd == end {
		return false
	}
	s.find.previewStart = start
	s.find.previewEnd = end
	if s.find.open && strings.TrimSpace(s.find.editor.Text()) != "" {
		s.startFindSearch(s.find.editor.Text())
	}
	return true
}

func (s *terminalSession) pumpFindResults() bool {
	if s == nil || s.find.results == nil {
		return false
	}
	changed := false
	for {
		select {
		case result := <-s.find.results:
			if result.generation != s.find.generation {
				continue
			}
			s.find.searching = false
			s.find.matches = result.matches
			s.find.clicks = make([]widget.Clickable, len(result.matches))
			s.find.previewIndex = -1
			s.find.previewAt = time.Time{}
			if len(result.matches) == 0 {
				s.find.index = 0
			} else if s.find.index >= len(result.matches) {
				s.find.index = len(result.matches) - 1
			}
			changed = true
		default:
			return changed
		}
	}
}

func (s *terminalSession) applyFindMatch(index int) bool {
	if s == nil || index < 0 || index >= len(s.find.matches) {
		return false
	}
	s.find.index = index
	match := s.find.matches[index]
	s.State.Mu.RLock()
	rows := s.State.Rows
	scrollback := s.State.Scrollback
	alternate := s.State.Alternate
	s.State.Mu.RUnlock()
	if !alternate {
		viewStart := match.Row - rows/2
		if viewStart < 0 {
			viewStart = 0
		}
		if viewStart > scrollback {
			viewStart = scrollback
		}
		s.setScrollOffset(scrollback - viewStart)
	}
	s.viewMu.Lock()
	s.selectionActive = true
	s.selectionSelecting = false
	s.selectionStart = terminalPoint{Row: match.Row, Col: match.StartCol}
	s.selectionEnd = terminalPoint{Row: match.Row, Col: match.EndCol}
	s.viewMu.Unlock()
	return true
}

func (s *terminalSession) stepFind(step int) bool {
	if s == nil || len(s.find.matches) == 0 || step == 0 {
		return false
	}
	next := (s.find.index + step) % len(s.find.matches)
	if next < 0 {
		next += len(s.find.matches)
	}
	return s.applyFindMatch(next)
}

func (ui *UI) layoutTerminalFind(th *material.Theme, gtx layout.Context, st *terminalSession, top int) layout.Dimensions {
	if ui == nil || th == nil || st == nil || !st.find.open {
		return layout.Dimensions{}
	}
	find := &st.find
	anyMods := ^key.Modifiers(0)
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &find.editor, Name: key.NameEscape, Optional: anyMods},
			key.Filter{Focus: &find.editor, Name: key.NameUpArrow, Optional: anyMods},
			key.Filter{Focus: &find.editor, Name: key.NameDownArrow, Optional: anyMods},
			key.Filter{Focus: &find.editor, Name: key.NameEnter, Optional: anyMods},
			key.Filter{Focus: &find.editor, Name: key.NameReturn, Optional: anyMods},
		)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case key.NameEscape:
			if st.closeFind() {
				gtx.Execute(op.InvalidateCmd{})
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		case key.NameUpArrow:
			if st.stepFind(-1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameDownArrow, key.NameEnter, key.NameReturn:
			if st.stepFind(1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
	for {
		ev, ok := find.editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.startFindSearch(find.editor.Text())
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	if find.closeClick.Clicked(gtx) {
		st.closeFind()
		gtx.Execute(op.InvalidateCmd{})
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	for i := range find.clicks {
		if find.clicks[i].Clicked(gtx) && st.applyFindMatch(i) {
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	theme := ui.fileViewerTheme()
	width := gtx.Dp(unit.Dp(terminalFindWidthDp))
	if maxWidth := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(12)); width > maxWidth {
		width = maxWidth
	}
	if width < 1 {
		return layout.Dimensions{}
	}
	recorded := op.Record(gtx.Ops)
	panelGTX := gtx
	panelGTX.Constraints.Min.Y = 0
	panelDims := fixedWidth(panelGTX, width, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutTerminalFindBar(th, gtx, st, theme)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutTerminalFindResults(th, gtx, st, theme)
			}),
		)
	})
	call := recorded.Stop()
	pos := image.Pt(gtx.Constraints.Max.X-panelDims.Size.X-gtx.Dp(unit.Dp(6)), top)
	if pos.X < 0 {
		pos.X = 0
	}
	offset := op.Offset(pos).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *UI) layoutTerminalFindBar(th *material.Theme, gtx layout.Context, st *terminalSession, theme fileViewerTheme) layout.Dimensions {
	bg := mixNRGBA(theme.PanelBg, theme.HeaderBg, 0.24)
	bg.A = 248
	border := mixNRGBA(theme.PanelBorder, theme.Divider, 0.42)
	border.A = 180
	return fillFlatBox(gtx, bg, border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(5), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(th, "Find")
					label.Font.Typeface = ui.terminalTypeface()
					label.Font.Weight = font.Medium
					label.TextSize = scaleThemeFontSize(th, 10)
					label.Color = theme.HeaderText
					return label.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(th, &st.find.editor, "Search terminal output")
					ed.Font.Typeface = ui.terminalTypeface()
					ed.TextSize = scaleThemeFontSize(th, 10)
					ed.Color = theme.CommandText
					ed.HintColor = theme.CommandHint
					focused := st.find.focus || gtx.Focused(&st.find.editor)
					return layoutNeutralEditorBox(gtx, focused, true, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(3), Right: unit.Dp(3)}.Layout(gtx, ed.Layout)
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					status := terminalFindStatus(&st.find)
					label := material.Caption(th, status)
					label.Font.Typeface = ui.terminalTypeface()
					label.TextSize = scaleThemeFontSize(th, 9)
					label.Color = theme.Hint
					label.MaxLines = 1
					return fixedWidth(gtx, gtx.Dp(unit.Dp(58)), func(gtx layout.Context) layout.Dimensions {
						return layout.E.Layout(gtx, label.Layout)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutFlatCloseButton(gtx, &st.find.closeClick, false)
				}),
			)
		})
	})
}

func terminalFindStatus(find *terminalFindState) string {
	if find == nil {
		return ""
	}
	if find.searching {
		return "Searching…"
	}
	if strings.TrimSpace(find.editor.Text()) == "" {
		return ""
	}
	if len(find.matches) == 0 {
		return "No matches"
	}
	return strconv.Itoa(find.index+1) + "/" + strconv.Itoa(len(find.matches))
}

func (ui *UI) layoutTerminalFindResults(th *material.Theme, gtx layout.Context, st *terminalSession, theme fileViewerTheme) layout.Dimensions {
	count := len(st.find.matches)
	if count == 0 {
		return layout.Dimensions{}
	}
	rows := min(count, terminalFindMaxRows)
	if hovered := terminalFindHoveredIndex(&st.find); hovered >= 0 {
		st.find.previewIndex = hovered
		st.find.previewAt = gtx.Now
	} else if st.find.previewIndex >= 0 && !st.find.previewAt.IsZero() {
		const hoverBridge = 140 * time.Millisecond
		expires := st.find.previewAt.Add(hoverBridge)
		if !gtx.Now.Before(expires) {
			st.find.previewIndex = -1
			st.find.previewAt = time.Time{}
		} else {
			gtx.Execute(op.InvalidateCmd{At: expires})
		}
	}
	previewMatch, hasPreview := terminalFindPreviewMatchForIndex(&st.find, st.find.previewIndex)
	var preview []string
	if hasPreview {
		preview = previewMatch.Preview
	}
	resultsHeight := rows * gtx.Dp(unit.Dp(terminalFindRowHeightDp))
	previewHeight := terminalFindDockedPreviewHeight(gtx, preview)
	height := resultsHeight + previewHeight
	if height > gtx.Constraints.Max.Y {
		height = gtx.Constraints.Max.Y
	}
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	bg := mixNRGBA(theme.PanelBg, theme.HeaderBg, 0.18)
	bg.A = 248
	border := mixNRGBA(theme.Divider, theme.HeaderText, 0.18)
	border.A = 84
	return fillFlatBox(gtx, bg, border, func(gtx layout.Context) layout.Dimensions {
		availablePreviewHeight := previewHeight
		if resultsHeight+availablePreviewHeight > gtx.Constraints.Max.Y {
			availablePreviewHeight = max(0, gtx.Constraints.Max.Y-resultsHeight)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, min(resultsHeight, gtx.Constraints.Max.Y), func(gtx layout.Context) layout.Dimensions {
					list := material.List(th, &st.find.list)
					list.AnchorStrategy = material.Occupy
					list.ScrollbarStyle.Track.Color = color.NRGBA{}
					list.ScrollbarStyle.Indicator.MinorWidth = unit.Dp(3)
					list.ScrollbarStyle.Indicator.CornerRadius = 0
					list.ScrollbarStyle.Indicator.Color = theme.ScrollThumb
					list.ScrollbarStyle.Indicator.HoverColor = theme.ScrollThumbHover
					return list.Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
						return ui.layoutTerminalFindResult(th, gtx, st, theme, index)
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if availablePreviewHeight <= 0 {
					return layout.Dimensions{}
				}
				return fixedHeight(gtx, availablePreviewHeight, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutTerminalFindDockedPreview(th, gtx, theme, previewMatch, hasPreview)
				})
			}),
		)
	})
}

func (ui *UI) layoutTerminalFindResult(th *material.Theme, gtx layout.Context, st *terminalSession, theme fileViewerTheme, index int) layout.Dimensions {
	if index < 0 || index >= len(st.find.matches) || index >= len(st.find.clicks) {
		return layout.Dimensions{}
	}
	match := st.find.matches[index]
	click := &st.find.clicks[index]
	rowBg := color.NRGBA{}
	if index == st.find.index {
		rowBg = mixNRGBA(theme.PanelBg, theme.StrongSelection, 0.34)
		rowBg.A = 224
	} else if click.Hovered() {
		rowBg = mixNRGBA(theme.PanelBg, theme.HeaderText, 0.08)
		rowBg.A = 238
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return fixedHeight(gtx, gtx.Dp(unit.Dp(terminalFindRowHeightDp)), func(gtx layout.Context) layout.Dimensions {
			return fillBgExact(gtx, rowBg, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(6), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							cursor := " "
							cursorIndex := st.find.index
							if st.find.previewIndex >= 0 {
								cursorIndex = st.find.previewIndex
							}
							if index == cursorIndex {
								cursor = ">"
							}
							marker := material.Body2(th, cursor)
							marker.Font.Typeface = ui.terminalTypeface()
							marker.Font.Weight = font.Bold
							marker.TextSize = scaleThemeFontSize(th, 10)
							marker.Color = theme.StatusAccent
							return fixedWidth(gtx, gtx.Dp(unit.Dp(12)), func(gtx layout.Context) layout.Dimensions {
								return layoutVCenteredLabel(gtx, marker)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lineNumber := material.Body2(th, strconv.Itoa(match.Row+1))
							lineNumber.Font.Typeface = ui.terminalTypeface()
							lineNumber.Font.Weight = font.Bold
							lineNumber.TextSize = scaleThemeFontSize(th, 10)
							lineNumber.Color = theme.StatusAccent
							return fixedWidth(gtx, gtx.Dp(unit.Dp(29)), func(gtx layout.Context) layout.Dimensions {
								return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layoutVCenteredLabel(gtx, lineNumber)
								})
							})
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(9)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							line := material.Body2(th, terminalFindPreviewText(match.Line))
							line.Font.Typeface = ui.terminalTypeface()
							line.TextSize = scaleThemeFontSize(th, 10)
							line.Color = theme.HeaderText
							line.MaxLines = 1
							line.Truncator = "…"
							return layoutVCenteredLabel(gtx, line)
						}),
					)
				})
			})
		})
	})
}

func terminalFindHoveredIndex(find *terminalFindState) int {
	if find == nil {
		return -1
	}
	for i := range find.clicks {
		if find.clicks[i].Hovered() {
			return i
		}
	}
	return -1
}

func terminalFindPreviewForIndex(find *terminalFindState, index int) []string {
	match, ok := terminalFindPreviewMatchForIndex(find, index)
	if !ok {
		return nil
	}
	return match.Preview
}

func terminalFindPreviewMatchForIndex(find *terminalFindState, index int) (terminalFindMatch, bool) {
	if find == nil || index < 0 || index >= len(find.matches) {
		return terminalFindMatch{}, false
	}
	return find.matches[index], true
}

func terminalFindDockedPreviewHeight(gtx layout.Context, preview []string) int {
	if len(preview) == 0 {
		return 0
	}
	return len(preview)*gtx.Dp(unit.Dp(terminalFindPreviewRowDp)) + gtx.Dp(unit.Dp(3))
}

func terminalFindPreviewText(text string) string {
	return strings.ReplaceAll(text, "\t", "    ")
}

func (ui *UI) layoutTerminalFindDockedPreview(th *material.Theme, gtx layout.Context, theme fileViewerTheme, match terminalFindMatch, visible bool) layout.Dimensions {
	if !visible || len(match.Preview) == 0 {
		return layout.Dimensions{}
	}
	preview := match.Preview
	height := terminalFindDockedPreviewHeight(gtx, preview)
	return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
		previewBG := mixNRGBA(theme.PanelBg, theme.Backdrop, 0.08)
		return fillBgExact(gtx, previewBG, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutTerminalFindPreviewDivider(gtx, theme)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0, len(preview))
					for index, text := range preview {
						index := index
						text := terminalFindPreviewText(text)
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, gtx.Dp(unit.Dp(terminalFindPreviewRowDp)), func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Right: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											indicator := color.NRGBA{}
											if index == match.PreviewFocus {
												indicator = theme.StatusAccent
											}
											return fixedWidth(gtx, gtx.Dp(unit.Dp(2)), func(gtx layout.Context) layout.Dimensions {
												return fillBgExact(gtx, indicator, func(gtx layout.Context) layout.Dimensions {
													return layout.Dimensions{Size: gtx.Constraints.Max}
												})
											})
										}),
										layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											label := material.Caption(th, text)
											label.Font.Typeface = ui.terminalTypeface()
											label.TextSize = scaleThemeFontSize(th, 9)
											label.Color = theme.HeaderText
											label.MaxLines = 1
											label.Truncator = "…"
											return layoutVCenteredLabel(gtx, label)
										}),
									)
								})
							})
						}))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout),
			)
		})
	})
}

func layoutTerminalFindPreviewDivider(gtx layout.Context, theme fileViewerTheme) layout.Dimensions {
	height := gtx.Dp(unit.Dp(1))
	if height < 1 {
		height = 1
	}
	width := gtx.Constraints.Max.X
	line := mixNRGBA(theme.Divider, theme.StatusAccent, 0.28)
	line.A = 188
	dash := gtx.Dp(unit.Dp(5))
	gap := gtx.Dp(unit.Dp(3))
	if dash < 2 {
		dash = 2
	}
	if gap < 1 {
		gap = 1
	}
	for x := 0; x < width; x += dash + gap {
		end := min(width, x+dash)
		paint.FillShape(gtx.Ops, line, clip.Rect(image.Rect(x, 0, end, height)).Op())
	}
	return layout.Dimensions{Size: image.Pt(width, height)}
}
