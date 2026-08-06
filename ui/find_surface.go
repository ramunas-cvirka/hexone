// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"image"
	"image/color"
	"strings"
	"time"
	"unicode"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func compactFindPreviewWindow(lines []string, row, previewStart, previewEnd int) ([]string, int) {
	if row < 0 || row >= len(lines) {
		return nil, -1
	}
	previewStart, previewEnd = fm.NormalizeViewerPreviewRange(previewStart, previewEnd)
	start := max(0, row+previewStart)
	end := min(len(lines), row+previewEnd+1)
	preview := append([]string(nil), lines[start:end]...)
	for i := range preview {
		preview[i] = strings.TrimSuffix(preview[i], "\r")
	}
	return preview, row - start
}

const compactFindCursorAnimDuration = 90 * time.Millisecond

const compactFindPreviewRowDp = 17

type compactFindPreview struct {
	Lines      []string
	Focus      int
	Highlights []compactFindHighlight
}

type compactFindHighlight struct {
	Start int
	End   int
}

func compactFindPreviewHeight(gtx layout.Context, preview compactFindPreview) int {
	if len(preview.Lines) == 0 {
		return 0
	}
	return len(preview.Lines)*gtx.Dp(unit.Dp(compactFindPreviewRowDp)) + gtx.Dp(unit.Dp(3))
}

func layoutCompactFindDockedPreview(th *material.Theme, gtx layout.Context, theme fileViewerTheme, typeface font.Typeface, textSize unit.Sp, preview compactFindPreview) layout.Dimensions {
	if len(preview.Lines) == 0 {
		return layout.Dimensions{}
	}
	height := compactFindPreviewHeight(gtx, preview)
	return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
		previewBG := mixNRGBA(theme.PanelBg, theme.Backdrop, 0.08)
		return fillBgExact(gtx, previewBG, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutCompactFindPreviewDivider(gtx, theme)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0, len(preview.Lines))
					for index, line := range preview.Lines {
						index, line := index, line
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, gtx.Dp(unit.Dp(compactFindPreviewRowDp)), func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Right: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											indicator := color.NRGBA{}
											if index == preview.Focus {
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
											if index < len(preview.Highlights) {
												return layoutCompactFindHighlightedText(th, gtx, theme, typeface, textSize, line, preview.Highlights[index], false)
											}
											label := material.Caption(th, line)
											label.Font.Typeface = typeface
											label.TextSize = textSize
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

func layoutCompactFindHighlightedText(th *material.Theme, gtx layout.Context, theme fileViewerTheme, typeface font.Typeface, textSize unit.Sp, value string, highlight compactFindHighlight, body bool) layout.Dimensions {
	if highlight.Start < 0 || highlight.End <= highlight.Start || highlight.Start >= len(value) {
		label := material.Caption(th, value)
		if body {
			label = material.Body2(th, value)
		}
		label.Font.Typeface = typeface
		label.TextSize = textSize
		label.Color = theme.HeaderText
		label.MaxLines = 1
		label.Truncator = "…"
		return layoutVCenteredLabel(gtx, label)
	}
	highlight.End = min(highlight.End, len(value))
	parts := []string{value[:highlight.Start], value[highlight.Start:highlight.End], value[highlight.End:]}
	contextColor := mixNRGBA(theme.HeaderText, theme.PanelBg, 0.28)
	contextColor.A = theme.HeaderText.A
	matchColor := mixNRGBA(theme.HeaderText, contrastTextColor(theme.PanelBg), 0.72)
	matchColor.A = 0xFF
	makeLabel := func(text string, hit bool) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			label := material.Caption(th, text)
			if body {
				label = material.Body2(th, text)
			}
			label.Font.Typeface = typeface
			label.TextSize = textSize
			label.Color = contextColor
			label.MaxLines = 1
			label.Truncator = "…"
			if !hit {
				return layoutVCenteredLabel(gtx, label)
			}
			label.Font.Weight = font.Bold
			label.Color = matchColor
			return layoutVCenteredLabel(gtx, label)
		}
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(makeLabel(parts[0], false)),
		layout.Rigid(makeLabel(parts[1], true)),
		layout.Flexed(1, makeLabel(parts[2], false)),
	)
}

func compactFindTabbedTextHighlight(value string, startByte, endByte int) (string, compactFindHighlight) {
	startByte = min(max(startByte, 0), len(value))
	endByte = min(max(endByte, startByte), len(value))
	highlight := compactFindHighlight{Start: -1, End: -1}
	var out strings.Builder
	for sourceAt, r := range value {
		sourceEnd := sourceAt + len(string(r))
		token := string(r)
		if r == '\t' {
			token = "    "
		}
		outputStart := out.Len()
		out.WriteString(token)
		if sourceAt < endByte && sourceEnd > startByte {
			if highlight.Start < 0 {
				highlight.Start = outputStart
			}
			highlight.End = out.Len()
		}
	}
	return out.String(), highlight
}

func compactFindCollapsedTextHighlight(value string, startByte, endByte int) (string, compactFindHighlight) {
	startByte = min(max(startByte, 0), len(value))
	endByte = min(max(endByte, startByte), len(value))
	highlight := compactFindHighlight{Start: -1, End: -1}
	var out strings.Builder
	pendingSpace := false
	pendingHit := false
	for sourceAt, r := range value {
		sourceEnd := sourceAt + len(string(r))
		overlaps := sourceAt < endByte && sourceEnd > startByte
		if unicode.IsSpace(r) {
			if out.Len() > 0 {
				pendingSpace = true
				pendingHit = pendingHit || overlaps
			}
			continue
		}
		if pendingSpace {
			outputStart := out.Len()
			out.WriteByte(' ')
			if pendingHit {
				if highlight.Start < 0 {
					highlight.Start = outputStart
				}
				highlight.End = out.Len()
			}
			pendingSpace = false
			pendingHit = false
		}
		outputStart := out.Len()
		out.WriteRune(r)
		if overlaps {
			if highlight.Start < 0 {
				highlight.Start = outputStart
			}
			highlight.End = out.Len()
		}
	}
	return out.String(), highlight
}

func compactFindRuneByteOffset(value string, runeIndex int) int {
	if runeIndex <= 0 {
		return 0
	}
	count := 0
	for byteAt := range value {
		if count == runeIndex {
			return byteAt
		}
		count++
	}
	return len(value)
}

// compactFindCursorAnim keeps the active result cursor responsive while giving
// short pointer moves enough continuity to read as movement rather than flicker.
type compactFindCursorAnim struct {
	initialized bool
	from        int
	target      int
	startedAt   time.Time
}

func (a *compactFindCursorAnim) setTarget(now time.Time, target int) {
	if a == nil || target < 0 {
		return
	}
	if !a.initialized {
		a.initialized = true
		a.from = target
		a.target = target
		a.startedAt = time.Time{}
		return
	}
	if a.target == target {
		return
	}
	a.from = a.target
	a.target = target
	a.startedAt = now
}

func (a *compactFindCursorAnim) reset() {
	if a != nil {
		*a = compactFindCursorAnim{}
	}
}

func (a *compactFindCursorAnim) frame(now time.Time, index int) (offsetY int, alpha uint8, progress float32, animating bool) {
	if a == nil || !a.initialized || index != a.target {
		return 0, 0, 0, false
	}
	progress = 1
	if !a.startedAt.IsZero() {
		progress = clamp01(float32(now.Sub(a.startedAt)) / float32(compactFindCursorAnimDuration))
		animating = progress < 1
	}
	eased := 1 - (1-progress)*(1-progress)*(1-progress)
	direction := 0
	if a.target > a.from {
		direction = 1
	} else if a.target < a.from {
		direction = -1
	}
	offsetY = int(float32(-direction) * (1 - eased) * 5)
	alpha = uint8(150 + eased*105)
	return offsetY, alpha, eased, animating
}

func layoutCompactFindCursor(th *material.Theme, gtx layout.Context, typeface font.Typeface, textSize unit.Sp, base color.NRGBA, anim *compactFindCursorAnim, index int) layout.Dimensions {
	offsetY, alpha, _, animating := anim.frame(gtx.Now, index)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	return fixedWidth(gtx, gtx.Dp(unit.Dp(12)), func(gtx layout.Context) layout.Dimensions {
		if alpha == 0 {
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}
		label := material.Body2(th, ">")
		label.Font.Typeface = typeface
		label.Font.Weight = font.Bold
		label.TextSize = textSize
		label.Color = base
		label.Color.A = uint8(uint16(label.Color.A) * uint16(alpha) / 255)
		offset := op.Offset(image.Pt(0, offsetY)).Push(gtx.Ops)
		dims := layoutVCenteredLabel(gtx, label)
		offset.Pop()
		return dims
	})
}

func compactFindHoverProgress(anim *compactFindCursorAnim, now time.Time, index int) float32 {
	_, _, progress, _ := anim.frame(now, index)
	return progress
}

// layoutCompactFindPreviewDivider draws an ASCII-like dashed rule in the same
// single pixel previously occupied by a solid divider.
func layoutCompactFindPreviewDivider(gtx layout.Context, theme fileViewerTheme) layout.Dimensions {
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
