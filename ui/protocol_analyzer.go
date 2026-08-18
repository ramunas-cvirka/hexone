// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"encoding/hex"
	resources "hexone"
	"hexone/protocols"
	"image"
	"image/color"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func newTab2State(typeface font.Typeface) *tab2State {
	if typeface == "" {
		typeface = font.Typeface(resources.BundledFontFamilyFiraCodeNerdFontMono)
	}
	st := &tab2State{
		scrollList:      widget.List{List: layout.List{Axis: layout.Vertical}},
		clicks:          map[string]*widget.Clickable{},
		typeface:        typeface,
		selectPressHeld: map[string]bool{},
	}
	st.hexEd.SingleLine = true
	st.hexEd.Submit = false
	st.hexEd.Filter = "0123456789abcdefABCDEFxX ,:\t\r\n-_"
	st.protoChoice.Value = "gt06"
	return st
}

func (ui *UI) ensureTab2Loaded(specYAML []byte) {
	if ui.tab2State != nil && ui.tab2State.spec != nil {
		return
	}
	if ui.tab2State == nil {
		ui.tab2State = newTab2State(ui.mainTypeface())
	} else if ui.tab2State.typeface == "" {
		ui.tab2State.typeface = ui.mainTypeface()
	}
	sp, err := protocols.LoadSpecYAML(specYAML)
	if err != nil {
		ui.tab2State.lastErr = err.Error()
		return
	}
	ui.tab2State.spec = sp
	ui.tab2State.reg = protocols.NewDefaultHookRegistry()
	ui.tab2State.lastErr = ""
}

func (ui *UI) layoutTab2(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if ui.tab2State == nil {
		ui.tab2State = newTab2State(ui.mainTypeface())
	}
	st := ui.tab2State

	// Live decode
	if st.spec != nil && !protocolExists(st.spec, st.protoChoice.Value) && len(st.spec.Protocols) > 0 {
		st.protoChoice.Value = st.spec.Protocols[0].Name
	}

	proto := st.protoChoice.Value
	hexText := st.hexEd.Text()
	if proto != st.lastProto || hexText != st.lastHexText {
		st.lastProto = proto
		st.lastHexText = hexText
		st.lastErr = ""
		st.lastRes = protocols.Result{}
		st.lastBytes = nil
		st.selectedHint = nil
		st.scrollList.Position = layout.Position{}
		for k := range st.selectPressHeld {
			delete(st.selectPressHeld, k)
		}

		b, err := parseHexText(hexText)
		if err != nil {
			st.lastErr = err.Error()
		} else if st.spec == nil {
			st.lastErr = "spec not loaded"
		} else {
			st.lastBytes = b
			res, derr := st.spec.Decode(proto, b, st.reg)
			if derr != nil {
				st.lastErr = derr.Error()
			}
			st.lastRes = res
		}
	}

	inset := layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(5), Bottom: unit.Dp(5)}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hoverSeen := false

		// Reset hover each frame — rebuilt during layout from current pointer state.
		st.hoverSpanKey = ""
		st.hoverSpan = nil
		st.hoverRowID = ""
		st.hoverFromBytes = false

		treeRows := analyzerTreeRows(st)
		widgets := []layout.Widget{
			func(gtx layout.Context) layout.Dimensions {
				return ui.row1InputAndProtocol(th, gtx, st, &hoverSeen)
			},
			func(gtx layout.Context) layout.Dimensions {
				return layoutAnalyzerStatus(th, gtx, st)
			},
			func(gtx layout.Context) layout.Dimensions {
				return layoutAnalyzerSectionHeader(th, gtx, st.typeface, "BYTE MAP", analyzerByteCount(st), nil)
			},
			func(gtx layout.Context) layout.Dimensions {
				return layoutAnalyzerByteMap(th, gtx, st, &hoverSeen)
			},
			func(gtx layout.Context) layout.Dimensions {
				return hintCardFixed(th, gtx, st, st.typeface, currentHintSpan(st))
			},
			func(gtx layout.Context) layout.Dimensions {
				return layoutAnalyzerSectionHeader(th, gtx, st.typeface, "PARSED DECODE TREE", analyzerFieldCount(st), nil)
			},
		}
		if len(treeRows) == 0 {
			widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
				return layoutAnalyzerEmptyDecodeTree(th, gtx, st)
			})
		} else {
			for _, item := range treeRows {
				row := item
				widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
					return layoutAnalyzerDecodeTreeRow(th, gtx, st, &hoverSeen, row)
				})
			}
		}

		return fillAnalyzerSurface(gtx, func(gtx layout.Context) layout.Dimensions {
			style := protocolAnalyzerListStyle(th, &st.scrollList)
			return style.LayoutWidgets(gtx, widgets...)
		})
	})
}

var (
	analyzerSurfaceBg = color.NRGBA{R: 13, G: 17, B: 23, A: 255}
	analyzerHeaderBg  = analyzerSurfaceBg
	analyzerBodyBg    = analyzerSurfaceBg
	analyzerRule      = color.NRGBA{R: 116, G: 157, B: 175, A: 132}
	analyzerAccent    = color.NRGBA{R: 92, G: 206, B: 226, A: 255}
	analyzerOK        = color.NRGBA{R: 126, G: 220, B: 160, A: 255}
	analyzerError     = color.NRGBA{R: 255, G: 112, B: 112, A: 255}
)

func protocolAnalyzerListStyle(th *material.Theme, list *widget.List) material.ListStyle {
	style := material.List(th, list)
	style.AnchorStrategy = material.Overlay
	style.Track.MajorPadding = unit.Dp(1)
	style.Track.MinorPadding = unit.Dp(1)
	style.Track.Color = color.NRGBA{R: 20, G: 38, B: 48, A: 170}
	style.Indicator.MajorMinLen = unit.Dp(24)
	style.Indicator.MinorWidth = unit.Dp(6)
	style.Indicator.CornerRadius = 0
	style.Indicator.Color = color.NRGBA{R: 92, G: 206, B: 226, A: 175}
	style.Indicator.HoverColor = analyzerAccent
	return style
}

func fillAnalyzerSurface(gtx layout.Context, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	fullW := gtx.Constraints.Max.X
	if fullW < dims.Size.X {
		fullW = dims.Size.X
	}
	if fullW < 1 {
		fullW = 1
	}
	fullH := dims.Size.Y
	if fullH < 1 {
		fullH = 1
	}
	rect := image.Rect(0, 0, fullW, fullH)
	paint.FillShape(gtx.Ops, analyzerSurfaceBg, clip.Rect(rect).Op())
	drawRectBorder(gtx, image.Pt(fullW, fullH), analyzerRule, 1)
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(fullW, fullH)}
}

func fillAnalyzerBody(gtx layout.Context, bg color.NRGBA, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	fullW := gtx.Constraints.Max.X
	if fullW < dims.Size.X {
		fullW = dims.Size.X
	}
	if fullW < 1 {
		fullW = 1
	}
	if dims.Size.Y < 1 {
		dims.Size.Y = 1
	}
	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: image.Pt(fullW, dims.Size.Y)}.Op())
	paint.FillShape(gtx.Ops, analyzerAccent, clip.Rect{Max: image.Pt(1, dims.Size.Y)}.Op())
	paint.FillShape(gtx.Ops, analyzerRule, clip.Rect{Min: image.Pt(fullW-1, 0), Max: image.Pt(fullW, dims.Size.Y)}.Op())
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(fullW, dims.Size.Y)}
}

func layoutAnalyzerSectionHeader(th *material.Theme, gtx layout.Context, typeface font.Typeface, title, rightText string, right layout.Widget) layout.Dimensions {
	titleLabel := material.Body2(th, "[ "+title+" ]")
	titleLabel.Font.Typeface = typeface
	titleLabel.Font.Weight = font.SemiBold
	titleLabel.TextSize = scaleThemeFontSize(th, 11)
	titleLabel.Color = analyzerAccent
	titleLabel.MaxLines = 1

	var rightWidget layout.Widget
	if right != nil {
		rightWidget = right
	} else if rightText != "" {
		rightWidget = func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, "[ "+rightText+" ]")
			lbl.Font.Typeface = typeface
			lbl.Font.Weight = font.Medium
			lbl.TextSize = scaleThemeFontSize(th, 10)
			lbl.Color = hintColor
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		}
	}

	mask := func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return fillBgExact(gtx, analyzerHeaderBg, w)
		}
	}

	return fillAnalyzerHeader(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(mask(titleLabel.Layout)),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					w := gtx.Constraints.Max.X
					if w < 1 {
						w = 1
					}
					return layout.Dimensions{Size: image.Pt(w, 1)}
				}),
			}
			if rightWidget != nil {
				children = append(children, layout.Rigid(mask(rightWidget)))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

func fillAnalyzerHeader(gtx layout.Context, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	fullW := gtx.Constraints.Max.X
	if fullW < dims.Size.X {
		fullW = dims.Size.X
	}
	if fullW < 1 {
		fullW = 1
	}
	minH := gtx.Dp(unit.Dp(23))
	if dims.Size.Y < minH {
		dims.Size.Y = minH
	}
	paint.FillShape(gtx.Ops, analyzerHeaderBg, clip.Rect{Max: image.Pt(fullW, dims.Size.Y)}.Op())
	lineY := dims.Size.Y / 2
	paint.FillShape(gtx.Ops, analyzerRule, clip.Rect{Min: image.Pt(0, lineY), Max: image.Pt(fullW, lineY+1)}.Op())
	paint.FillShape(gtx.Ops, analyzerRule, clip.Rect{Min: image.Pt(fullW-1, 0), Max: image.Pt(fullW, dims.Size.Y)}.Op())
	paint.FillShape(gtx.Ops, analyzerAccent, clip.Rect{Max: image.Pt(1, dims.Size.Y)}.Op())
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(fullW, dims.Size.Y)}
}

func layoutAnalyzerStatus(th *material.Theme, gtx layout.Context, st *tab2State) layout.Dimensions {
	msg, status, statusColor := analyzerStatus(st)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutAnalyzerSectionHeader(th, gtx, st.typeface, "ERROR / STATUS", "", func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, "[ "+status+" ]")
				lbl.Font.Typeface = st.typeface
				lbl.Font.Weight = font.SemiBold
				lbl.TextSize = scaleThemeFontSize(th, 10)
				lbl.Color = statusColor
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fillAnalyzerBody(gtx, analyzerBodyBg, func(gtx layout.Context) layout.Dimensions {
				return minHeight(gtx, gtx.Dp(unit.Dp(30)), func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(11), Right: unit.Dp(11), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, msg)
						lbl.Font.Typeface = st.typeface
						lbl.TextSize = scaleThemeFontSize(th, 11)
						lbl.Color = statusColor
						lbl.MaxLines = 2
						return lbl.Layout(gtx)
					})
				})
			})
		}),
	)
}

func analyzerStatus(st *tab2State) (msg, status string, statusColor color.NRGBA) {
	if st == nil {
		return "Analyzer state unavailable.", "× OFFLINE", analyzerError
	}
	msg = st.lastErr
	if msg == "" && len(st.lastRes.Errors) > 0 {
		msg = st.lastRes.Errors[0]
	}
	if msg != "" {
		return "×  " + msg, "× PARSE FAIL", analyzerError
	}
	if len(st.lastBytes) == 0 {
		return "·  Waiting for a hex stream.", "○ IDLE", hintColor
	}
	return "✓  Frame decoded. Hover or select a byte range to inspect its field.", "✓ DECODE OK", analyzerOK
}

func analyzerByteCount(st *tab2State) string {
	if st == nil || len(st.lastBytes) == 0 {
		return "NO BYTES"
	}
	if len(st.lastBytes) == 1 {
		return "1 BYTE"
	}
	return itoa(len(st.lastBytes)) + " BYTES"
}

func analyzerLeafRows(st *tab2State) []flatSpan {
	if st == nil {
		return nil
	}
	flat := flattenAllWithKeys(st.lastRes.Spans)
	sort.Slice(flat, func(i, j int) bool {
		if flat[i].Span.Start == flat[j].Span.Start {
			if flat[i].Span.End == flat[j].Span.End {
				return flat[i].Key < flat[j].Key
			}
			return flat[i].Span.End < flat[j].Span.End
		}
		return flat[i].Span.Start < flat[j].Span.Start
	})
	filtered := flat[:0:0]
	for _, f := range flat {
		if f.Span != nil && len(f.Span.Children) == 0 {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

type analyzerTreeRow struct {
	Span         *protocols.Span
	Key          string
	Last         bool
	AncestorLast []bool
	Container    bool
}

func analyzerTreeRows(st *tab2State) []analyzerTreeRow {
	if st == nil {
		return nil
	}
	var rows []analyzerTreeRow
	var walk func(spans []*protocols.Span, path string, ancestorLast []bool)
	walk = func(spans []*protocols.Span, path string, ancestorLast []bool) {
		for i, sp := range spans {
			if sp == nil {
				continue
			}
			last := true
			for j := i + 1; j < len(spans); j++ {
				if spans[j] != nil {
					last = false
					break
				}
			}
			key := path + itoa(i) + "/" + sp.Name + ":" + rangeKey(sp.Start, sp.End)
			rows = append(rows, analyzerTreeRow{
				Span:         sp,
				Key:          key,
				Last:         last,
				AncestorLast: append([]bool(nil), ancestorLast...),
				Container:    len(sp.Children) > 0,
			})
			if len(sp.Children) > 0 {
				nextAncestors := append(append([]bool(nil), ancestorLast...), last)
				walk(sp.Children, key+"/", nextAncestors)
			}
		}
	}
	walk(st.lastRes.Spans, "", nil)
	return rows
}

func analyzerFieldCount(st *tab2State) string {
	n := len(analyzerLeafRows(st))
	if n == 1 {
		return "1 FIELD"
	}
	return itoa(n) + " FIELDS"
}

func layoutAnalyzerByteMap(th *material.Theme, gtx layout.Context, st *tab2State, hoverSeen *bool) layout.Dimensions {
	return fillAnalyzerBody(gtx, analyzerBodyBg, func(gtx layout.Context) layout.Dimensions {
		return minHeight(gtx, gtx.Dp(unit.Dp(38)), func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(11), Right: unit.Dp(11), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lines := buildRow2HexLines(th, gtx, st, hoverSeen)
				if len(lines) == 0 {
					lbl := material.Body1(th, "│  ··· waiting for bytes ···")
					lbl.Font.Typeface = st.typeface
					lbl.TextSize = scaleThemeFontSize(th, 12)
					lbl.Color = hintColor
					lbl.MaxLines = 1
					return lbl.Layout(gtx)
				}
				children := make([]layout.FlexChild, len(lines))
				for i, ln := range lines {
					line := ln
					children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, line...)
					})
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
	})
}

func layoutAnalyzerEmptyDecodeTree(th *material.Theme, gtx layout.Context, st *tab2State) layout.Dimensions {
	return fillAnalyzerBody(gtx, analyzerBodyBg, func(gtx layout.Context) layout.Dimensions {
		return minHeight(gtx, gtx.Dp(unit.Dp(34)), func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body1(th, "│\n└─── no decoded fields yet")
				lbl.Font.Typeface = st.typeface
				lbl.TextSize = scaleThemeFontSize(th, 11)
				lbl.Color = hintColor
				lbl.MaxLines = 2
				return lbl.Layout(gtx)
			})
		})
	})
}

func layoutAnalyzerDecodeTreeRow(th *material.Theme, gtx layout.Context, st *tab2State, hoverSeen *bool, it analyzerTreeRow) layout.Dimensions {
	rowID := it.Key
	spanKey := rangeKey(it.Span.Start, it.Span.End)

	return fillAnalyzerBody(gtx, analyzerBodyBg, func(gtx layout.Context) layout.Dimensions {
		render := func(gtx layout.Context) layout.Dimensions {
			click := st.click("row3:" + rowID)
			if !it.Container {
				st.handleSelectTogglePress("row3:"+rowID, click, spanKey, rowID, it.Span)
				if click.Hovered() {
					st.hoverRowID = rowID
					st.hoverSpanKey = spanKey
					st.hoverSpan = it.Span
					st.hoverFromBytes = false
					*hoverSeen = true
				}
			}

			isSel := st.selectedRowID != "" && rowID == st.selectedRowID
			isHover := st.hoverRowID != "" && rowID == st.hoverRowID
			bg := color.NRGBA{A: 0}
			if isSel {
				bg = color.NRGBA{R: 48, G: 105, B: 136, A: 110}
			} else if isHover {
				bg = color.NRGBA{R: 255, G: 255, B: 255, A: 12}
			}

			var connector strings.Builder
			for _, ancestorLast := range it.AncestorLast {
				if ancestorLast {
					connector.WriteString("   ")
				} else {
					connector.WriteString("│  ")
				}
			}
			if it.Last {
				connector.WriteString("└")
			} else {
				connector.WriteString("├")
			}
			switch {
			case it.Container:
				connector.WriteString("──┬─")
			case it.Span.IsError:
				connector.WriteString("──×─")
			default:
				connector.WriteString("────")
			}

			line := connector.String() + formatListOffset(it.Span.Start)
			if it.Span.Value != "" {
				line += "  " + it.Span.Value
			}
			line += "  " + it.Span.Name

			return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(13), Right: unit.Dp(10), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, line)
					lbl.Font.Typeface = st.typeface
					lbl.TextSize = scaleThemeFontSize(th, 11)
					lbl.Color = colorForSpanText(it.Span, st.lastRes.Spans)
					if it.Container && !it.Span.IsError {
						lbl.Color = analyzerAccent
					}
					lbl.MaxLines = 1
					lbl.Font.Weight = font.Medium
					if it.Container {
						lbl.Font.Weight = font.SemiBold
					}
					return lbl.Layout(gtx)
				})
			})
		}
		if it.Container {
			return render(gtx)
		}
		return st.click("row3:"+rowID).Layout(gtx, render)
	})
}

// ---------- Input + floating protocol combobox (NO layout shift) ----------

func (ui *UI) row1InputAndProtocol(th *material.Theme, gtx layout.Context, st *tab2State, hoverSeen *bool) layout.Dimensions {
	var (
		btnW    int
		headerH int
	)

	// Consume option clicks before backdrop clicks, so item selection wins.
	if st.protoDropOpen {
		opts := protocolOptions(st)
		for _, opt := range opts {
			click := st.click("proto:" + opt.Name)
			for click.Clicked(gtx) {
				st.protoChoice.Value = opt.Name
				st.protoDropOpen = false
				st.protoDropOpenedAt = time.Time{}
			}
		}
		if st.protoDropOpen {
			back := st.click("proto:backdrop")
			for back.Clicked(gtx) {
				st.protoDropOpen = false
				st.protoDropOpenedAt = time.Time{}
			}
		}
	}

	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d := layoutAnalyzerSectionHeader(th, gtx, st.typeface, "INPUT · HEX STREAM", "", func(gtx layout.Context) layout.Dimensions {
				btnDims := ui.protocolDropdownButton(th, gtx, st, hoverSeen)
				btnW = btnDims.Size.X
				return btnDims
			})
			headerH = d.Size.Y
			return d
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fillAnalyzerBody(gtx, analyzerBodyBg, func(gtx layout.Context) layout.Dimensions {
				return minHeight(gtx, gtx.Dp(unit.Dp(36)), func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layoutAnalyzerHexInputField(th, gtx, ui, st)
					})
				})
			})
		}),
	)

	if st.protoDropOpen {
		m := op.Record(gtx.Ops)

		back := st.click("proto:backdrop")
		_ = back.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})

		popupX := dims.Size.X - gtx.Dp(unit.Dp(7)) - btnW
		if popupX < 0 {
			popupX = 0
		}
		popupY := headerH
		offset := op.Offset(image.Pt(popupX, popupY))
		offset.Add(gtx.Ops)
		ui.protocolDropdownPopup(th, gtx, st, hoverSeen, btnW)

		popupCall := m.Stop()
		op.Defer(gtx.Ops, popupCall)
	}

	return dims
}

func layoutAnalyzerHexInputField(th *material.Theme, gtx layout.Context, ui *UI, st *tab2State) layout.Dimensions {
	focused := gtx.Focused(&st.hexEd)
	prompt := material.Body2(th, "HEX >")
	prompt.Font.Typeface = st.typeface
	prompt.Font.Weight = font.SemiBold
	prompt.TextSize = scaleThemeFontSize(th, 11)
	prompt.Color = analyzerAccent
	prompt.MaxLines = 1

	return layoutAnalyzerInputStrip(gtx, focused, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(prompt.Layout),
			layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				ed := material.Editor(th, &st.hexEd, "78 78 0D 01 …  ·  paste hex here")
				ed.TextSize = scaleThemeFontSize(th, 13)
				ed.Font.Typeface = st.typeface
				ed.Font.Weight = font.Medium
				ed.Color = txtColor
				ed.HintColor = hintColor
				return ui.layoutEditorWithContextMenu(th, gtx, "tab2-hex", &st.hexEd, true, ed.Layout)
			}),
		)
	})
}

func layoutAnalyzerInputStrip(gtx layout.Context, focused bool, inner layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, inner)
	call := m.Stop()

	fullW := gtx.Constraints.Max.X
	if fullW < dims.Size.X {
		fullW = dims.Size.X
	}
	if fullW < 1 {
		fullW = 1
	}
	if dims.Size.Y < 1 {
		dims.Size.Y = 1
	}
	bg := color.NRGBA{R: 17, G: 25, B: 33, A: 255}
	border := color.NRGBA{R: 116, G: 157, B: 175, A: 105}
	underline := analyzerRule
	underlineH := 1
	if focused {
		bg = color.NRGBA{R: 19, G: 30, B: 39, A: 255}
		border = color.NRGBA{R: 92, G: 206, B: 226, A: 145}
		underline = analyzerAccent
		underlineH = 2
	}

	rect := image.Rect(0, 0, fullW, dims.Size.Y)
	paint.FillShape(gtx.Ops, bg, clip.Rect(rect).Op())
	drawRectBorder(gtx, image.Pt(fullW, dims.Size.Y), border, 1)
	paint.FillShape(gtx.Ops, underline, clip.Rect{Min: image.Pt(0, dims.Size.Y-underlineH), Max: image.Pt(fullW, dims.Size.Y)}.Op())
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(fullW, dims.Size.Y)}
}

func (ui *UI) protocolDropdownButton(th *material.Theme, gtx layout.Context, st *tab2State, hoverSeen *bool) layout.Dimensions {
	btn := st.click("proto:btn")
	for btn.Clicked(gtx) {
		st.protoDropOpen = !st.protoDropOpen
		if st.protoDropOpen {
			st.protoDropOpenedAt = gtx.Now
		} else {
			st.protoDropOpenedAt = time.Time{}
		}
	}

	label := protocolLabel(st.protoChoice.Value)
	txt := strings.ToUpper(label)

	w := ui.protocolDropdownWidth(th, gtx, st)
	popupTheme := ui.filePanePopupTheme()
	return fixedWidth(gtx, w, func(gtx layout.Context) layout.Dimensions {
		bracket := func(text string) layout.Widget {
			return func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, text)
				lbl.Font.Typeface = st.typeface
				lbl.TextSize = scaleThemeFontSize(th, 11)
				lbl.Color = txtColor
				lbl.MaxLines = 1
				lbl.Font.Weight = font.Medium
				return lbl.Layout(gtx)
			}
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(bracket("[ ")),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					if btn.Hovered() {
						*hoverSeen = true
					}
					bg := color.NRGBA{A: 0}
					fg := txtColor
					if st.protoDropOpen {
						bg = popupTheme.ActiveBg
						fg = popupTheme.ActiveText
					} else if btn.Hovered() {
						bg = popupTheme.HoverBg
						fg = popupTheme.HoverText
					}
					return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, txt)
								lbl.Font.Typeface = st.typeface
								lbl.TextSize = scaleThemeFontSize(th, 11)
								lbl.Color = fg
								lbl.MaxLines = 1
								lbl.Font.Weight = font.Medium
								return lbl.Layout(gtx)
							})
						})
					})
				})
			}),
			layout.Rigid(bracket(" ]")),
		)
	})
}

// Popup ONLY draws the option box. Backdrop is handled in stacked overlay.
func (ui *UI) protocolDropdownPopup(th *material.Theme, gtx layout.Context, st *tab2State, hoverSeen *bool, width int) layout.Dimensions {
	opts := protocolOptions(st)

	// Hard clamp popup height (prevents “stretches to bottom”).
	gtx2 := gtx
	maxH := gtx.Dp(unit.Dp(380))
	if gtx2.Constraints.Max.Y > maxH {
		gtx2.Constraints.Max.Y = maxH
	}

	popupTheme := ui.filePanePopupTheme()
	alpha, offsetY, animating := popupOpenProgress(gtx.Now, st.protoDropOpenedAt)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	return fixedWidth(gtx2, width, func(gtx layout.Context) layout.Dimensions {
		defer op.Offset(image.Pt(0, offsetY)).Push(gtx.Ops).Pop()
		return fillRoundedClipBox(gtx, 0, scaleColorAlpha(popupTheme.Bg, alpha), scaleColorAlpha(popupTheme.Border, alpha), func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(opts))
			for _, opt := range opts {
				opt := opt
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					click := st.click("proto:" + opt.Name)
					return ui.dropdownItem(th, gtx, st.typeface, click, opt.Label, st.protoChoice.Value == opt.Name, hoverSeen, alpha)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (ui *UI) protocolDropdownWidth(th *material.Theme, gtx layout.Context, st *tab2State) int {
	opts := protocolOptions(st)
	maxTextW := 0
	for _, opt := range opts {
		lbl := material.Body2(th, strings.ToUpper(opt.Label))
		lbl.Font.Typeface = st.typeface
		lbl.TextSize = scaleThemeFontSize(th, 11)
		lbl.MaxLines = 1
		w := measureLabelUnconstrained(gtx, lbl).Size.X
		if w > maxTextW {
			maxTextW = w
		}
	}
	if maxTextW == 0 {
		lbl := material.Body2(th, strings.ToUpper(protocolLabel(st.protoChoice.Value)))
		lbl.Font.Typeface = st.typeface
		lbl.TextSize = scaleThemeFontSize(th, 11)
		lbl.MaxLines = 1
		maxTextW = measureLabelUnconstrained(gtx, lbl).Size.X
	}
	brackets := material.Body2(th, "[  ]")
	brackets.Font.Typeface = st.typeface
	brackets.TextSize = scaleThemeFontSize(th, 11)
	brackets.MaxLines = 1
	w := maxTextW + measureLabelUnconstrained(gtx, brackets).Size.X + gtx.Dp(unit.Dp(12))
	minW := gtx.Dp(unit.Dp(84))
	if w < minW {
		w = minW
	}
	if max := gtx.Constraints.Max.X; max > 0 && w > max {
		w = max
	}
	if w < 1 {
		w = 1
	}
	return w
}

type protoOption struct {
	Name  string
	Label string
}

func protocolOptions(st *tab2State) []protoOption {
	if st != nil && st.spec != nil && len(st.spec.Protocols) > 0 {
		opts := make([]protoOption, 0, len(st.spec.Protocols))
		teltonikaSeen := false
		for _, p := range st.spec.Protocols {
			if isTeltonikaProtocolName(p.Name) {
				if !teltonikaSeen {
					opts = append(opts, protoOption{Name: "teltonika", Label: protocolLabel("teltonika")})
					teltonikaSeen = true
				}
				continue
			}
			opts = append(opts, protoOption{Name: p.Name, Label: protocolLabel(p.Name)})
		}
		return opts
	}
	return []protoOption{
		{Name: "gt06", Label: protocolLabel("gt06")},
		{Name: "teltonika", Label: protocolLabel("teltonika")},
	}
}

func protocolExists(sp *protocols.Spec, name string) bool {
	if sp == nil {
		return false
	}
	if name == "teltonika" {
		for _, p := range sp.Protocols {
			if isTeltonikaProtocolName(p.Name) {
				return true
			}
		}
		return false
	}
	_, ok := sp.ProtocolByName(name)
	return ok
}

func isTeltonikaProtocolName(name string) bool {
	return strings.HasPrefix(name, "teltonika")
}

func protocolLabel(name string) string {
	switch name {
	case "gt06":
		return "GT06"
	case "teltonika":
		return "Teltonika"
	}

	parts := strings.Split(name, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func (ui *UI) dropdownItem(th *material.Theme, gtx layout.Context, typeface font.Typeface, c *widget.Clickable, label string, selected bool, hoverSeen *bool, alpha float32) layout.Dimensions {
	popupTheme := ui.filePanePopupTheme()
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if c.Hovered() {
			*hoverSeen = true
		}
		bg := color.NRGBA{A: 0}
		fg := scaleColorAlpha(popupTheme.Text, alpha)
		if selected {
			bg = scaleColorAlpha(popupTheme.ActiveBg, alpha)
			fg = scaleColorAlpha(popupTheme.ActiveText, alpha)
		} else if c.Hovered() {
			bg = scaleColorAlpha(popupTheme.HoverBg, alpha)
			fg = scaleColorAlpha(popupTheme.HoverText, alpha)
		}
		return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				prefix := "  "
				if selected {
					prefix = "> "
				}
				lbl := material.Body2(th, prefix+strings.ToUpper(label))
				lbl.Font.Typeface = typeface
				lbl.TextSize = scaleThemeFontSize(th, 11)
				lbl.Color = fg
				lbl.MaxLines = 1
				lbl.Font.Weight = font.Medium
				return lbl.Layout(gtx)
			})
		})
	})
}

// ---------- Row 2: centered grid, dotted selection border ----------

type spanOwner struct {
	key string
	sp  *protocols.Span
}

type row2Seg struct {
	start, end int
	owner      spanOwner
	txt        string
	widthPx    int
}

func buildRow2HexLines(th *material.Theme, gtx layout.Context, st *tab2State, hoverSeen *bool) [][]layout.FlexChild {
	b := st.lastBytes
	if len(b) == 0 {
		return nil
	}

	const row2Sp = 16

	// measure "00 " cell width
	token := material.Body1(th, "00 ")
	token.TextSize = scaleThemeFontSize(th, row2Sp)
	token.Font.Typeface = st.typeface
	token.Font.Weight = font.Medium
	token.Color = txtColor
	token.MaxLines = 1
	tokenW := measureLabelUnconstrained(gtx, token).Size.X
	if tokenW <= 0 {
		tokenW = 14
	}

	maxW := gtx.Constraints.Max.X
	if maxW <= 0 {
		maxW = 1
	}

	bytesPerLine := maxW / tokenW
	if bytesPerLine < 1 {
		bytesPerLine = 1
	}

	// Build per-byte owners from leaf spans (no decoded children).
	// Leaf spans correspond 1:1 with the fields list rows.
	allFlat := flattenAllWithKeys(st.lastRes.Spans)

	// Collect leaves sorted by start.
	var leafSpans []*protocols.Span
	for _, f := range allFlat {
		if f.Span == nil || len(f.Span.Children) > 0 {
			continue
		}
		if f.Span.Start < 0 || f.Span.End > len(b) || f.Span.End <= f.Span.Start {
			continue
		}
		leafSpans = append(leafSpans, f.Span)
	}
	sort.Slice(leafSpans, func(i, j int) bool {
		if leafSpans[i].Start == leafSpans[j].Start {
			return leafSpans[i].End < leafSpans[j].End
		}
		return leafSpans[i].Start < leafSpans[j].Start
	})

	// Deduplicate / remove contained-within spans (keep outermost if same start).
	{
		deduped := leafSpans[:0:0]
		prev := 0
		for _, sp := range leafSpans {
			if sp.Start < prev {
				continue // contained in previous span, skip
			}
			deduped = append(deduped, sp)
			prev = sp.End
		}
		leafSpans = deduped
	}

	// Fill gaps between leaf spans so every byte has an owner.
	var top []*protocols.Span
	if len(leafSpans) == 0 {
		top = []*protocols.Span{{Name: "data", Start: 0, End: len(b), ColorKey: "meta"}}
	} else {
		top = fillGapsWithMeta(leafSpans, len(b))
	}

	owners := make([]spanOwner, len(b))
	for i := range owners {
		owners[i] = spanOwner{}
	}
	for _, sp := range top {
		k := rangeKey(sp.Start, sp.End)
		for i := sp.Start; i < sp.End && i < len(b); i++ {
			owners[i] = spanOwner{key: k, sp: sp}
		}
	}

	// Build line segments
	var lines [][]row2Seg
	for lineStart := 0; lineStart < len(b); lineStart += bytesPerLine {
		lineEnd := lineStart + bytesPerLine
		if lineEnd > len(b) {
			lineEnd = len(b)
		}

		var segs []row2Seg
		cur := owners[lineStart]
		segStart := lineStart

		flush := func(end int) {
			if end <= segStart {
				return
			}
			n := end - segStart
			segs = append(segs, row2Seg{
				start:   segStart,
				end:     end,
				owner:   cur,
				txt:     bytesHexSpaced(b[segStart:end]),
				widthPx: n * tokenW,
			})
		}

		for i := lineStart + 1; i < lineEnd; i++ {
			if owners[i].key != cur.key {
				flush(i)
				cur = owners[i]
				segStart = i
			}
		}
		flush(lineEnd)
		lines = append(lines, segs)
	}

	// Render lines
	out := make([][]layout.FlexChild, 0, len(lines))
	for _, segs := range lines {
		line := make([]layout.FlexChild, 0, len(segs))

		for _, s := range segs {
			seg := s
			click := st.click("row2:" + rangeKey(seg.start, seg.end))

			line = append(line, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				cellW := seg.widthPx

				return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					// Process events inside click.Layout.
					rowID := ""
					if seg.owner.key != "" {
						for _, f := range flattenAllWithKeys(st.lastRes.Spans) {
							if rangeKey(f.Span.Start, f.Span.End) == seg.owner.key {
								rowID = f.Key
								break
							}
						}
					}
					st.handleSelectTogglePress("row2:"+rangeKey(seg.start, seg.end), click, seg.owner.key, rowID, seg.owner.sp)
					if click.Hovered() && seg.owner.sp != nil {
						*hoverSeen = true
						st.hoverSpanKey = seg.owner.key
						st.hoverSpan = seg.owner.sp
						st.hoverFromBytes = true
						// Sync list hover.
						if st.hoverRowID == "" {
							for _, f := range flattenAllWithKeys(st.lastRes.Spans) {
								if rangeKey(f.Span.Start, f.Span.End) == seg.owner.key {
									st.hoverRowID = f.Key
									break
								}
							}
						}
					}

					isSel := seg.owner.key != "" && st.selectedSpanKey == seg.owner.key
					isHover := seg.owner.key != "" && st.hoverSpanKey == seg.owner.key
					bg := colorForSpan(seg.owner.sp, isSel, isHover, st.lastRes.Spans)

					// Build label style (no layout yet).
					lbl := material.Body1(th, seg.txt)
					lbl.TextSize = scaleThemeFontSize(th, row2Sp)
					lbl.Font.Typeface = st.typeface
					lbl.MaxLines = 1
					lbl.Color = txtColor
					lbl.Font.Weight = font.Medium
					if seg.owner.sp != nil && seg.owner.sp.IsError {
						lbl.Color = color.NRGBA{R: 255, G: 120, B: 120, A: 255}
					}

					// Every segment shares the same left-aligned token grid.
					// Centering individual spans shifts bytes at color boundaries
					// because the final token intentionally reserves trailing space.
					txtDims := measureLabelUnconstrained(gtx, lbl)
					vPad := gtx.Dp(unit.Dp(2))
					cellH := txtDims.Size.Y + vPad*2

					// Paint bg FIRST — self-contained shape, no clip stack interaction.
					if bg.A != 0 {
						paint.FillShape(gtx.Ops, bg,
							clip.Rect{Max: image.Pt(cellW, cellH)}.Op())
					}

					// Paint text on top — into real ops.
					textOff := op.Offset(image.Pt(0, vPad)).Push(gtx.Ops)
					gtx2 := gtx
					gtx2.Constraints.Min = image.Point{}
					gtx2.Constraints.Max = image.Pt(cellW, txtDims.Size.Y)
					lbl.Layout(gtx2)
					textOff.Pop()

					// Border on top.
					sz := image.Pt(cellW, cellH)
					if isSel {
						ulH := gtx.Dp(unit.Dp(2))
						if ulH < 1 {
							ulH = 1
						}
						ulY := cellH - ulH
						if ulY < 0 {
							ulY = 0
						}
						paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 180},
							clip.Rect{Min: image.Pt(0, ulY), Max: image.Pt(cellW, ulY+ulH)}.Op())
						drawDottedBorder(gtx, sz, color.NRGBA{R: 255, G: 255, B: 255, A: 170})
					} else if isHover {
						drawRectBorder(gtx, sz, color.NRGBA{R: 255, G: 255, B: 255, A: 70}, 1)
					}
					return layout.Dimensions{Size: sz}
				})
			}))
		}

		out = append(out, line)
	}

	return out
}

func drawRectBorder(gtx layout.Context, sz image.Point, c color.NRGBA, wPx int) {
	if sz.X <= 0 || sz.Y <= 0 {
		return
	}
	r := image.Rect(0, 0, sz.X, sz.Y)
	paint.FillShape(gtx.Ops, c, clip.Rect{Min: r.Min, Max: image.Pt(r.Max.X, r.Min.Y+wPx)}.Op())
	paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(r.Min.X, r.Max.Y-wPx), Max: r.Max}.Op())
	paint.FillShape(gtx.Ops, c, clip.Rect{Min: r.Min, Max: image.Pt(r.Min.X+wPx, r.Max.Y)}.Op())
	paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(r.Max.X-wPx, r.Min.Y), Max: r.Max}.Op())
}

func drawDottedBorder(gtx layout.Context, sz image.Point, c color.NRGBA) {
	if sz.X <= 0 || sz.Y <= 0 {
		return
	}
	dot := 2
	gap := 4
	w := 1

	for x := 0; x < sz.X; x += dot + gap {
		x2 := x + dot
		if x2 > sz.X {
			x2 = sz.X
		}
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(x, 0), Max: image.Pt(x2, w)}.Op())
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(x, sz.Y-w), Max: image.Pt(x2, sz.Y)}.Op())
	}
	for y := 0; y < sz.Y; y += dot + gap {
		y2 := y + dot
		if y2 > sz.Y {
			y2 = sz.Y
		}
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(0, y), Max: image.Pt(w, y2)}.Op())
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(sz.X-w, y), Max: image.Pt(sz.X, y2)}.Op())
	}
}

// ---------- Hint ----------

func hintCardFixed(th *material.Theme, gtx layout.Context, st *tab2State, typeface font.Typeface, sp *protocols.Span) layout.Dimensions {
	copyBtn := st.click("hint:copy")
	for copyBtn.Clicked(gtx) {
		text := hintClipboardText(sp)
		if text != "" {
			gtx.Execute(clipboard.WriteCmd{
				Type: "application/text",
				Data: io.NopCloser(strings.NewReader(text)),
			})
			st.hintCopyPulseAt = gtx.Now
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	copyWidget := func(gtx layout.Context) layout.Dimensions {
		const pulseDur = 220 * time.Millisecond
		pulseAge := gtx.Now.Sub(st.hintCopyPulseAt)
		pulseActive := pulseAge >= 0 && pulseAge < pulseDur
		if pulseActive || copyBtn.Hovered() || copyBtn.Pressed() {
			gtx.Execute(op.InvalidateCmd{})
		}
		return copyBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			bg := color.NRGBA{A: 0}
			fg := hintColor
			if sp != nil {
				fg = txtColor
			}
			if copyBtn.Hovered() {
				bg = color.NRGBA{R: 46, G: 83, B: 104, A: 150}
				fg = analyzerAccent
			}
			if copyBtn.Pressed() || pulseActive {
				bg = color.NRGBA{R: 54, G: 111, B: 139, A: 220}
				fg = color.NRGBA{R: 235, G: 252, B: 255, A: 255}
			}
			return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, "[ COPY ]")
					lbl.Font.Typeface = typeface
					lbl.Font.Weight = font.SemiBold
					lbl.TextSize = scaleThemeFontSize(th, 10)
					lbl.Color = fg
					lbl.MaxLines = 1
					return lbl.Layout(gtx)
				})
			})
		})
	}

	title := "NO FIELD SELECTED"
	meta := "│  hover the byte map or decode tree to inspect a field"
	desc := "└─ click a field to lock the inspector"
	titleColor := hintColor
	if sp != nil {
		title = strings.ToUpper(sp.Name)
		length := sp.End - sp.Start
		meta = "│  OFFSET 0x" + strings.ToUpper(strconv.FormatInt(int64(sp.Start), 16)) +
			"  ·  LENGTH " + itoa(length) + " " + byteWord(length)
		if sp.Value != "" {
			meta += "  ·  VALUE " + sp.Value
		}
		desc = "└─ " + sp.Desc
		if sp.Desc == "" {
			desc = "└─ decoded field"
		}
		titleColor = colorForSpanText(sp, st.lastRes.Spans)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutAnalyzerSectionHeader(th, gtx, typeface, "FIELD INSPECTOR · "+title, "", copyWidget)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fillAnalyzerBody(gtx, analyzerBodyBg, func(gtx layout.Context) layout.Dimensions {
				return minHeight(gtx, gtx.Dp(unit.Dp(42)), func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(11), Right: unit.Dp(11), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						metaLabel := material.Body2(th, meta)
						metaLabel.Font.Typeface = typeface
						metaLabel.TextSize = scaleThemeFontSize(th, 11)
						metaLabel.Color = titleColor
						metaLabel.MaxLines = 1

						descLabel := material.Caption(th, desc)
						descLabel.Font.Typeface = typeface
						descLabel.TextSize = scaleThemeFontSize(th, 10)
						descLabel.Color = hintColor
						descLabel.MaxLines = 2
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(metaLabel.Layout),
							layout.Rigid(descLabel.Layout),
						)
					})
				})
			})
		}),
	)
}

func byteWord(n int) string {
	if n == 1 {
		return "BYTE"
	}
	return "BYTES"
}

func currentHintSpan(st *tab2State) *protocols.Span {
	if st == nil {
		return nil
	}
	if st.hoverFromBytes && st.hoverSpan != nil {
		return st.hoverSpan
	}
	if st.selectedSpanKey != "" {
		if sp := findSpanByKey(st.lastRes.Spans, st.selectedSpanKey); sp != nil {
			return sp
		}
	}
	if st.selectedHint != nil {
		return st.selectedHint
	}
	if st.hoverSpan != nil {
		return st.hoverSpan
	}
	return nil
}

func hintClipboardText(sp *protocols.Span) string {
	if sp == nil {
		return ""
	}
	line2 := formatRange(sp.Start, sp.End)
	if sp.Value != "" {
		line2 += "   " + sp.Value
	}
	if sp.Desc == "" {
		return sp.Name + "\n" + line2
	}
	return sp.Name + "\n" + line2 + "\n" + sp.Desc
}

// ---------- Flatten spans with unique keys ----------

type flatSpan struct {
	Span  *protocols.Span
	Depth int
	Key   string
}

func flattenAllWithKeys(spans []*protocols.Span) []flatSpan {
	var out []flatSpan
	var walk func(list []*protocols.Span, depth int, path string)
	walk = func(list []*protocols.Span, depth int, path string) {
		for i, sp := range list {
			if sp == nil {
				continue
			}
			k := path + itoa(i) + "/" + sp.Name + ":" + rangeKey(sp.Start, sp.End)
			out = append(out, flatSpan{Span: sp, Depth: depth, Key: k})
			if len(sp.Children) > 0 {
				walk(sp.Children, depth+1, k+"/")
			}
		}
	}
	walk(spans, 0, "")
	return out
}

func rowIndexVisible(pos layout.Position, idx, total int) bool {
	if idx < 0 || idx >= total || total == 0 {
		return false
	}

	first := pos.First
	if first < 0 {
		first = 0
	}
	if first >= total {
		first = total - 1
	}

	count := pos.Count
	if count <= 0 {
		return idx == first
	}

	last := first + count - 1
	if last >= total {
		last = total - 1
	}
	return idx >= first && idx <= last
}

// ---------- Misc helpers ----------

func fillGapsWithMeta(spans []*protocols.Span, total int) []*protocols.Span {
	var out []*protocols.Span
	prev := 0
	for _, sp := range spans {
		if sp.Start > prev {
			out = append(out, &protocols.Span{
				Name:     "gap",
				Start:    prev,
				End:      sp.Start,
				ColorKey: "meta",
				Desc:     "unparsed bytes",
			})
		}
		out = append(out, sp)
		if sp.End > prev {
			prev = sp.End
		}
	}
	if prev < total {
		out = append(out, &protocols.Span{
			Name:     "gap",
			Start:    prev,
			End:      total,
			ColorKey: "meta",
			Desc:     "unparsed bytes",
		})
	}
	return out
}

func findSpanByKey(spans []*protocols.Span, key string) *protocols.Span {
	var walk func(list []*protocols.Span) *protocols.Span
	walk = func(list []*protocols.Span) *protocols.Span {
		for _, sp := range list {
			if sp == nil {
				continue
			}
			if rangeKey(sp.Start, sp.End) == key {
				return sp
			}
			if len(sp.Children) > 0 {
				if got := walk(sp.Children); got != nil {
					return got
				}
			}
		}
		return nil
	}
	return walk(spans)
}

func (st *tab2State) handleSelectTogglePress(pressID string, c *widget.Clickable, spanKey, rowID string, sp *protocols.Span) {
	if st == nil || c == nil {
		return
	}
	if st.selectPressHeld == nil {
		st.selectPressHeld = map[string]bool{}
	}
	if c.Pressed() {
		if !st.selectPressHeld[pressID] {
			st.selectPressHeld[pressID] = true
			st.toggleSelection(spanKey, rowID, sp)
		}
		return
	}
	delete(st.selectPressHeld, pressID)
}

func (st *tab2State) toggleSelection(spanKey, rowID string, sp *protocols.Span) {
	if st == nil || spanKey == "" || sp == nil {
		return
	}
	if st.selectedSpanKey == spanKey {
		st.selectedSpanKey = ""
		st.selectedRowID = ""
		st.selectedHint = nil
		return
	}
	st.selectedSpanKey = spanKey
	st.selectedRowID = rowID
	st.selectedHint = sp
}

func (st *tab2State) click(key string) *widget.Clickable {
	if st.clicks == nil {
		st.clicks = map[string]*widget.Clickable{}
	}
	if c, ok := st.clicks[key]; ok {
		return c
	}
	c := new(widget.Clickable)
	st.clicks[key] = c
	return c
}

func parseHexText(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if n := hexTextIgnoredPrefixLen(s, i); n > 0 {
			i += n - 1
			continue
		}
		ch := s[i]
		if ch == '0' && i+1 < len(s) && (s[i+1] == 'x' || s[i+1] == 'X') {
			i++
			continue
		}
		if isHexChar(ch) {
			b.WriteByte(byte(unicode.ToLower(rune(ch))))
		}
	}
	clean := b.String()
	if len(clean)%2 != 0 {
		return nil, errText("odd number of hex digits")
	}
	if clean == "" {
		return nil, nil
	}
	out := make([]byte, len(clean)/2)
	_, err := hex.Decode(out, []byte(clean))
	return out, err
}

func hexTextIgnoredPrefixLen(s string, i int) int {
	if i < 0 || i >= len(s) {
		return 0
	}
	rest := s[i:]
	switch {
	case strings.HasPrefix(rest, `\r\n`):
		return 4
	case strings.HasPrefix(rest, `\n`), strings.HasPrefix(rest, `\r`):
		return 2
	}
	for _, marker := range [...]string{"CRLF", "CR", "LF"} {
		if len(rest) < len(marker) || !strings.EqualFold(rest[:len(marker)], marker) {
			continue
		}
		beforeOK := i == 0 || !isHexChar(s[i-1])
		afterIdx := i + len(marker)
		afterOK := afterIdx >= len(s) || !isHexChar(s[afterIdx])
		if beforeOK && afterOK {
			return len(marker)
		}
	}
	return 0
}

func bytesHexSpaced(bb []byte) string {
	if len(bb) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(bb)*3 - 1)
	const hexd = "0123456789ABCDEF"
	for i, v := range bb {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteByte(hexd[v>>4])
		sb.WriteByte(hexd[v&0x0F])
	}
	return sb.String()
}

func bytesHexCompact(bb []byte) string {
	if len(bb) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(bb) * 2)
	const hexd = "0123456789ABCDEF"
	for _, v := range bb {
		sb.WriteByte(hexd[v>>4])
		sb.WriteByte(hexd[v&0x0F])
	}
	return sb.String()
}

func isHexChar(c byte) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'a' && c <= 'f') ||
		(c >= 'A' && c <= 'F')
}

type errText string

func (e errText) Error() string { return string(e) }

func measureLabelUnconstrained(gtx layout.Context, lbl material.LabelStyle) layout.Dimensions {
	gtx2 := gtx
	var measureOps op.Ops
	gtx2.Ops = &measureOps
	gtx2.Constraints = layout.Constraints{Min: image.Point{}, Max: image.Point{X: 1 << 30, Y: 1 << 30}}
	return lbl.Layout(gtx2)
}

func rangeKey(s, e int) string    { return itoa(s) + ":" + itoa(e) }
func formatRange(s, e int) string { return "[" + itoa(s) + ".." + itoa(e) + "]" }
func formatListOffset(v int) string {
	return "[" + itoa(v) + "]"
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [32]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + (v % 10))
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func clampU8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// ---------- Background helpers ----------

func fillRoundedBox(gtx layout.Context, radius int, bg, border color.NRGBA, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()

	rect := image.Rect(0, 0, dims.Size.X, dims.Size.Y)
	rr := clip.UniformRRect(rect, radius)
	// Draw bg BEFORE content.
	paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())

	call.Add(gtx.Ops)
	return dims
}

func fillBgExact(gtx layout.Context, c color.NRGBA, w layout.Widget) layout.Dimensions {
	// Record the child first to measure it, then draw bg BEFORE child so bg doesn't overpaint.
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	if c.A != 0 {
		// Draw bg shape directly (before call.Add so it's underneath).
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: dims.Size}.Op())
	}
	call.Add(gtx.Ops)
	return dims
}

func fixedWidth(gtx layout.Context, w int, wid layout.Widget) layout.Dimensions {
	gtx2 := gtx
	gtx2.Constraints.Min.X = w
	gtx2.Constraints.Max.X = w

	m := op.Record(gtx.Ops)
	d := wid(gtx2)
	call := m.Stop()
	if d.Size.X < w {
		d.Size.X = w
	}
	call.Add(gtx.Ops)
	return d
}

func minHeight(gtx layout.Context, h int, wid layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := wid(gtx)
	call := m.Stop()
	if d.Size.Y < h {
		d.Size.Y = h
	}
	call.Add(gtx.Ops)
	return d
}

// ---------- Colors ----------

// spanPalette: a set of distinct, readable colors on dark bg.
// Each entry is {bg fill (low alpha), text color}.
var spanPaletteBg = []color.NRGBA{
	{R: 74, G: 114, B: 191, A: 84},  // 0 cobalt    (header)
	{R: 186, G: 150, B: 68, A: 90},  // 1 amber     (len)
	{R: 118, G: 148, B: 48, A: 92},  // 2 lime      (id)
	{R: 170, G: 82, B: 136, A: 92},  // 3 rose      (crc)
	{R: 110, G: 88, B: 170, A: 92},  // 4 violet    (tail)
	{R: 166, G: 108, B: 58, A: 94},  // 5 copper    (serial)
	{R: 72, G: 136, B: 156, A: 82},  // 6 teal
	{R: 124, G: 148, B: 72, A: 82},  // 7 olive
	{R: 226, G: 92, B: 92, A: 92},   // 8 red       (error)
	{R: 86, G: 158, B: 176, A: 82},  // 9 aqua
	{R: 194, G: 128, B: 88, A: 82},  // 10 clay
	{R: 108, G: 140, B: 194, A: 82}, // 11 steel
	{R: 170, G: 136, B: 74, A: 82},  // 12 brass
	{R: 76, G: 150, B: 126, A: 82},  // 13 sea
	{R: 156, G: 102, B: 138, A: 82}, // 14 mauve
	{R: 132, G: 138, B: 150, A: 52}, // 15 slate    (meta/gap)
}

var spanPaletteTxt = []color.NRGBA{
	{R: 144, G: 188, B: 255, A: 255}, // 0 cobalt
	{R: 244, G: 214, B: 132, A: 255}, // 1 amber
	{R: 214, G: 238, B: 136, A: 255}, // 2 lime
	{R: 246, G: 164, B: 214, A: 255}, // 3 rose
	{R: 194, G: 176, B: 255, A: 255}, // 4 violet
	{R: 247, G: 196, B: 138, A: 255}, // 5 copper
	{R: 154, G: 220, B: 236, A: 255}, // 6 teal
	{R: 208, G: 226, B: 144, A: 255}, // 7 olive
	{R: 255, G: 132, B: 132, A: 255}, // 8 red
	{R: 164, G: 230, B: 244, A: 255}, // 9 aqua
	{R: 244, G: 194, B: 156, A: 255}, // 10 clay
	{R: 176, G: 206, B: 255, A: 255}, // 11 steel
	{R: 232, G: 202, B: 132, A: 255}, // 12 brass
	{R: 150, G: 222, B: 192, A: 255}, // 13 sea
	{R: 228, G: 178, B: 216, A: 255}, // 14 mauve
	{R: 168, G: 214, B: 236, A: 255}, // 15 slate
}

// payloadShade*: two alternating accents for generic payload/meta fields.
var payloadShadeBg = []color.NRGBA{
	{R: 52, G: 82, B: 126, A: 78},
	{R: 66, G: 112, B: 94, A: 78},
}

var payloadShadeTxt = []color.NRGBA{
	{R: 164, G: 208, B: 255, A: 255},
	{R: 176, G: 236, B: 204, A: 255},
}

// paletteIndexForSpan returns a stable palette index for a span.
func paletteIndexForSpan(sp *protocols.Span) int {
	if sp == nil {
		return 15
	}
	switch sp.ColorKey {
	case "header":
		return 0
	case "len":
		return 1
	case "id":
		return 2
	case "crc":
		return 3
	case "tail":
		return 4
	case "serial":
		return 5
	case "error":
		return 8
	case "meta":
		if isNeutralMetaSpan(sp) {
			return 15
		}
	}
	// Hash fields into slots 6..14 (avoid 0-5 reserved, 15=meta).
	// Use start+end to distinguish adjacent single-byte fields.
	h := (sp.Start*11 + sp.End*7) % 9
	return 6 + h
}

func isNeutralMetaSpan(sp *protocols.Span) bool {
	if sp == nil {
		return true
	}
	switch sp.Name {
	case "gap", "data", "align":
		return true
	}
	return sp.Desc == "unparsed bytes"
}

func payloadShadeIndex(sp *protocols.Span, roots []*protocols.Span) int {
	if sp == nil || len(payloadShadeBg) == 0 {
		return 0
	}
	if len(payloadShadeBg) == 1 {
		return 0
	}

	idx := 0
	for _, item := range flattenAllWithKeys(roots) {
		cur := item.Span
		if cur == nil || len(cur.Children) > 0 {
			continue
		}
		if cur.ColorKey != "meta" || isNeutralMetaSpan(cur) {
			continue
		}
		if cur.Start == sp.Start && cur.End == sp.End && cur.Name == sp.Name {
			return idx % len(payloadShadeBg)
		}
		idx++
	}

	return 0
}

func colorForSpan(sp *protocols.Span, selected, hovered bool, roots []*protocols.Span) color.NRGBA {
	if sp != nil && sp.IsError {
		c := color.NRGBA{R: 255, G: 90, B: 90, A: 90}
		if hovered {
			c.A = clampU8(int(c.A) + 25)
		}
		if selected {
			c.A = clampU8(int(c.A) + 45)
		}
		return c
	}
	var base color.NRGBA
	if sp != nil && sp.ColorKey == "meta" && !isNeutralMetaSpan(sp) {
		base = payloadShadeBg[payloadShadeIndex(sp, roots)]
	} else {
		idx := paletteIndexForSpan(sp)
		base = spanPaletteBg[idx]
	}
	if hovered {
		base.A = clampU8(int(base.A) + 25)
	}
	if selected {
		base.A = clampU8(int(base.A) + 45)
	}
	return base
}

// colorForSpanText returns the text color for a span in the fields list.
func colorForSpanText(sp *protocols.Span, roots []*protocols.Span) color.NRGBA {
	if sp == nil {
		return txtColor
	}
	if sp.IsError {
		return color.NRGBA{R: 255, G: 120, B: 120, A: 255}
	}
	if sp.ColorKey == "meta" && !isNeutralMetaSpan(sp) {
		return payloadShadeTxt[payloadShadeIndex(sp, roots)]
	}
	idx := paletteIndexForSpan(sp)
	return spanPaletteTxt[idx]
}
