package ui

import (
	"encoding/hex"
	"hexone/protocols"
	"image"
	"image/color"
	"sort"
	"strings"
	"unicode"

	"gioui.org/font"
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
		typeface = font.Typeface("Fira Code")
	}
	st := &tab2State{
		list:     layout.List{Axis: layout.Vertical},
		clicks:   map[string]*widget.Clickable{},
		typeface: typeface,
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

		b, err := parseHexLine(hexText)
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

	inset := layout.UniformInset(unit.Dp(10))
	gap := unit.Dp(10)

	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		hoverSeen := false

		// Reset hover each frame — rebuilt during layout from current pointer state.
		st.hoverSpanKey = ""
		st.hoverSpan = nil
		st.hoverRowID = ""

		dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,

			// Row 1: editor + floating combobox
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row1InputAndProtocol(th, gtx, st, &hoverSeen)
			}),

			layout.Rigid(layout.Spacer{Height: gap}.Layout),

			// Error
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				msg := st.lastErr
				if msg == "" && len(st.lastRes.Errors) > 0 {
					msg = st.lastRes.Errors[0]
				}
				if msg == "" {
					return layout.Dimensions{}
				}
				lbl := material.Body2(th, msg)
				lbl.Font.Typeface = st.typeface
				lbl.Color = color.NRGBA{R: 240, G: 90, B: 90, A: 255}
				lbl.MaxLines = 2
				return lbl.Layout(gtx)
			}),

			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),

			// Row 2: hex grid. Paint card bg before cells to avoid recording-layer ordering issues.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				hexAreaClick := st.click("hexarea:bg")
				for hexAreaClick.Clicked(gtx) {
					st.selectedSpanKey = ""
					st.selectedRowID = ""
				}
				return hexAreaClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					const row2Sp = 18
					// Estimate card height: measure one text line height.
					sampleLbl := material.Body1(th, "00")
					sampleLbl.TextSize = scaleThemeFontSize(th, row2Sp)
					sampleLbl.Font.Typeface = st.typeface
					sampleLbl.MaxLines = 1
					lineH := measureLabelUnconstrained(gtx, sampleLbl).Size.Y + gtx.Dp(unit.Dp(8)) // +padding
					padV := gtx.Dp(unit.Dp(10))
					padH := gtx.Dp(unit.Dp(16))
					fullW := gtx.Constraints.Max.X
					numLines := 1
					if b := st.lastBytes; len(b) > 0 {
						// measure tokenW same way as buildRow2HexLines
						tok := material.Body1(th, "00 ")
						tok.TextSize = scaleThemeFontSize(th, row2Sp)
						tok.Font.Typeface = st.typeface
						tok.MaxLines = 1
						tokenW := measureLabelUnconstrained(gtx, tok).Size.X
						if tokenW < 1 {
							tokenW = 14
						}
						innerW := fullW - padH*2
						if innerW < 1 {
							innerW = 1
						}
						bpl := innerW / tokenW
						if bpl < 1 {
							bpl = 1
						}
						numLines = (len(b) + bpl - 1) / bpl
					}
					cardH := padV*2 + numLines*lineH

					// Paint card bg + border FIRST (directly into ops, no recording).
					rect := image.Rect(0, 0, fullW, cardH)
					rr := clip.UniformRRect(rect, gtx.Dp(unit.Dp(14)))
					paint.FillShape(gtx.Ops, color.NRGBA{R: 18, G: 22, B: 30, A: 255}, rr.Op(gtx.Ops))
					paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 18},
						clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())

					// Render hex cells on top (events + drawing).
					inset := layout.Inset{
						Left: unit.Dp(16), Right: unit.Dp(16),
						Top: unit.Dp(10), Bottom: unit.Dp(10),
					}
					lines := buildRow2HexLines(th, gtx, st, &hoverSeen)
					if len(lines) == 0 {
						inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body1(th, "(no data)")
							lbl.Font.Typeface = st.typeface
							lbl.Color = txtColor
							lbl.MaxLines = 1
							return lbl.Layout(gtx)
						})
					} else {
						inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								func() []layout.FlexChild {
									fc := make([]layout.FlexChild, len(lines))
									for i, ln := range lines {
										line := ln
										fc[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, line...)
										})
									}
									return fc
								}()...)
						})
					}

					return layout.Dimensions{Size: image.Pt(fullW, cardH)}
				})
			}),

			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),

			// Hint
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				sp := st.hoverSpan
				if sp == nil && st.selectedSpanKey != "" {
					sp = findSpanByKey(st.lastRes.Spans, st.selectedSpanKey)
				}
				return hintCardFixed(th, gtx, st.typeface, sp)
			}),

			layout.Rigid(layout.Spacer{Height: gap}.Layout),

			// Row 3 list
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return card(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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

						// Filter out container spans that have decoded children —
						// they are structural only (like "payload"), not real fields.
						filtered := flat[:0:0]
						for _, f := range flat {
							if len(f.Span.Children) > 0 {
								continue // skip containers that were decoded into sub-fields
							}
							filtered = append(filtered, f)
						}
						flat = filtered

						if len(flat) == 0 {
							lbl := material.Body1(th, "No decoded fields.")
							lbl.Font.Typeface = st.typeface
							lbl.Color = txtColor
							lbl.MaxLines = 1
							return lbl.Layout(gtx)
						}

						// Scroll to the selected row once, and keep hovered rows visible.
						if st.selectedRowID != "" && st.selectedRowID != st.lastScrolledRowID {
							if idx := findFlatRowIndex(flat, st.selectedRowID); idx >= 0 {
								st.list.Position.First = idx
								st.list.Position.Offset = 0
							}
							st.lastScrolledRowID = st.selectedRowID
						}
						if st.hoverRowID != "" {
							if idx := findFlatRowIndex(flat, st.hoverRowID); idx >= 0 && !rowIndexVisible(st.list.Position, idx, len(flat)) {
								st.list.Position.First = idx
								st.list.Position.Offset = 0
							}
						}

						listDims := st.list.Layout(gtx, len(flat), func(gtx layout.Context, i int) layout.Dimensions {
							it := flat[i]
							rowID := it.Key
							spanKey := rangeKey(it.Span.Start, it.Span.End)

							click := st.click("row3:" + rowID)

							return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								for click.Clicked(gtx) {
									st.selectedSpanKey = spanKey
									st.selectedRowID = rowID
								}
								if click.Hovered() {
									// Last item that reports Hovered() wins (bottom-most under cursor).
									st.hoverRowID = rowID
									st.hoverSpanKey = spanKey
									st.hoverSpan = it.Span
									hoverSeen = true
								}

								isSel := st.selectedRowID != "" && rowID == st.selectedRowID
								isHover := st.hoverRowID != "" && rowID == st.hoverRowID

								bg := color.NRGBA{A: 0}
								if isSel {
									bg = color.NRGBA{R: 80, G: 120, B: 220, A: 45}
								} else if isHover {
									bg = color.NRGBA{R: 255, G: 255, B: 255, A: 12}
								}

								prefix := strings.Repeat("  ", it.Depth)
								sp := it.Span
								line := prefix + formatListOffset(sp.Start)
								if sp.Value != "" {
									line += "  " + sp.Value
								}
								line += "  " + sp.Name

								return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										lbl := material.Body2(th, line)
										lbl.Font.Typeface = st.typeface
										lbl.TextSize = scaleThemeFontSize(th, 12)
										lbl.Color = colorForSpanText(it.Span, st.lastRes.Spans)
										lbl.MaxLines = 1
										lbl.Font.Weight = font.Medium
										return lbl.Layout(gtx)
									})
								})
							})
						})
						return listDims
					})
				})
			}),
		)

		return dims
	})
}

// ---------- Row 1: editor + floating combobox (NO layout shift) ----------

func row1InputAndProtocol(th *material.Theme, gtx layout.Context, st *tab2State, hoverSeen *bool) layout.Dimensions {
	// We need btnDims for popup anchoring, so lay out the base row first.
	// The popup is deferred so it paints on top of everything.

	var (
		btnLeft int
		btnTop  int
		btnH    int
		btnW    int
	)

	// Process backdrop click before laying out (so click is consumed this frame).
	if st.protoDropOpen {
		back := st.click("proto:backdrop")
		for back.Clicked(gtx) {
			st.protoDropOpen = false
		}
		// Full-screen hit area for backdrop — drawn via defer below.
	}

	// Base row.
	dims := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			ed := material.Editor(th, &st.hexEd, "Paste hex (spaces/commas ok)")
			ed.TextSize = scaleThemeFontSize(th, 14)
			ed.Font.Typeface = st.typeface
			ed.Font.Weight = font.Medium
			ed.Color = txtColor
			ed.HintColor = hintColor
			return ed.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Record offset of the button so we can anchor popup.
			// In Gio there is no direct "get current offset" API, but since
			// we are last in the flex, we can compute it: left = total_width - btnW.
			d := protocolDropdownButton(th, gtx, st, hoverSeen)
			btnH = d.Size.Y
			btnW = d.Size.X
			_ = btnTop // will be 0 relative to row
			return d
		}),
	)

	btnLeft = dims.Size.X - btnW

	if st.protoDropOpen {
		// Defer renders after all children — proper overlay.
		m := op.Record(gtx.Ops)

		// Backdrop hit area.
		back := st.click("proto:backdrop")
		_ = back.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})

		// Popup positioned below the button.
		popupX := btnLeft
		popupY := btnTop + btnH + gtx.Dp(unit.Dp(4))
		offset := op.Offset(image.Pt(popupX, popupY))
		offset.Add(gtx.Ops)
		protocolDropdownPopup(th, gtx, st, hoverSeen, btnW)

		popupCall := m.Stop()
		op.Defer(gtx.Ops, popupCall)
	}

	return dims
}

func protocolDropdownButton(th *material.Theme, gtx layout.Context, st *tab2State, hoverSeen *bool) layout.Dimensions {
	btn := st.click("proto:btn")
	for btn.Clicked(gtx) {
		st.protoDropOpen = !st.protoDropOpen
	}

	label := protocolLabel(st.protoChoice.Value)
	txt := label + "  ▾"

	w := gtx.Dp(unit.Dp(260))
	return fixedWidth(gtx, w, func(gtx layout.Context) layout.Dimensions {
		return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if btn.Hovered() {
				*hoverSeen = true
			}
			bg := color.NRGBA{R: 18, G: 22, B: 30, A: 255}
			bd := color.NRGBA{R: 255, G: 255, B: 255, A: 22}
			return fillRoundedBox(gtx, gtx.Dp(unit.Dp(10)), bg, bd, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, txt)
					lbl.Font.Typeface = st.typeface
					lbl.TextSize = scaleThemeFontSize(th, 12)
					lbl.Color = txtColor
					lbl.MaxLines = 1
					return lbl.Layout(gtx)
				})
			})
		})
	})
}

// Popup ONLY draws the option box. Backdrop is handled in stacked overlay.
func protocolDropdownPopup(th *material.Theme, gtx layout.Context, st *tab2State, hoverSeen *bool, width int) layout.Dimensions {
	opts := protocolOptions(st)
	for _, opt := range opts {
		click := st.click("proto:" + opt.Name)
		for click.Clicked(gtx) {
			st.protoChoice.Value = opt.Name
			st.protoDropOpen = false
		}
	}

	// Hard clamp popup height (prevents “stretches to bottom”).
	gtx2 := gtx
	maxH := gtx.Dp(unit.Dp(380))
	if gtx2.Constraints.Max.Y > maxH {
		gtx2.Constraints.Max.Y = maxH
	}

	return fixedWidth(gtx2, width, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{R: 18, G: 22, B: 30, A: 255}
		bd := color.NRGBA{R: 255, G: 255, B: 255, A: 18}

		return fillRoundedBox(gtx, gtx.Dp(unit.Dp(10)), bg, bd, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(opts))
			for _, opt := range opts {
				opt := opt
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					click := st.click("proto:" + opt.Name)
					return dropdownItem(th, gtx, st.typeface, click, opt.Label, st.protoChoice.Value == opt.Name, hoverSeen)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
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
	case "teltonika_tcp":
		return "Teltonika AVL TCP"
	case "teltonika_imei_tcp":
		return "Teltonika IMEI TCP"
	case "teltonika_imei_ack":
		return "Teltonika IMEI ACK"
	case "teltonika_tcp_ack":
		return "Teltonika AVL ACK"
	case "teltonika_udp":
		return "Teltonika AVL UDP"
	case "teltonika_udp_ack":
		return "Teltonika UDP ACK"
	case "teltonika_codec12":
		return "Teltonika Codec 12"
	case "teltonika_codec13":
		return "Teltonika Codec 13"
	case "teltonika_codec14":
		return "Teltonika Codec 14"
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

func dropdownItem(th *material.Theme, gtx layout.Context, typeface font.Typeface, c *widget.Clickable, label string, selected bool, hoverSeen *bool) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if c.Hovered() {
			*hoverSeen = true
		}
		bg := color.NRGBA{A: 0}
		if selected {
			bg = color.NRGBA{R: 80, G: 120, B: 220, A: 45}
		} else if c.Hovered() {
			bg = color.NRGBA{R: 255, G: 255, B: 255, A: 10}
		}
		return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = typeface
				lbl.TextSize = scaleThemeFontSize(th, 12)
				lbl.Color = txtColor
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

	const row2Sp = 18 // bigger font

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
					for click.Clicked(gtx) {
						if seg.owner.key != "" {
							st.selectedSpanKey = seg.owner.key
							st.selectedRowID = ""
							for _, f := range flattenAllWithKeys(st.lastRes.Spans) {
								if rangeKey(f.Span.Start, f.Span.End) == seg.owner.key {
									st.selectedRowID = f.Key
									break
								}
							}
						}
					}
					if click.Hovered() && seg.owner.sp != nil {
						*hoverSeen = true
						st.hoverSpanKey = seg.owner.key
						st.hoverSpan = seg.owner.sp
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

					// Center the text inside the colored block so each segment has
					// some visual breathing room on both sides.
					txtDims := measureLabelUnconstrained(gtx, lbl)
					txtW := txtDims.Size.X
					vPad := gtx.Dp(unit.Dp(4))
					cellH := txtDims.Size.Y + vPad*2
					txtX := (cellW - txtW) / 2
					if txtX < 0 {
						txtX = 0
					}

					// Paint bg FIRST — self-contained shape, no clip stack interaction.
					if bg.A != 0 {
						paint.FillShape(gtx.Ops, bg,
							clip.Rect{Max: image.Pt(cellW, cellH)}.Op())
					}

					// Paint text on top — into real ops.
					textOff := op.Offset(image.Pt(txtX, vPad)).Push(gtx.Ops)
					gtx2 := gtx
					gtx2.Constraints.Min = image.Point{}
					gtx2.Constraints.Max = image.Pt(cellW-txtX, txtDims.Size.Y)
					lbl.Layout(gtx2)
					textOff.Pop()

					// Border on top.
					sz := image.Pt(cellW, cellH)
					if isSel {
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

// Selection = dotted border; Hover = thin solid border.
func drawSelectionDecor(gtx layout.Context, selected, hovered bool, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := w(gtx)
	call := m.Stop()

	call.Add(gtx.Ops)

	if hovered {
		drawRectBorder(gtx, d.Size, color.NRGBA{R: 255, G: 255, B: 255, A: 70}, 1)
	}
	if selected {
		drawDottedBorder(gtx, d.Size, color.NRGBA{R: 255, G: 255, B: 255, A: 170})
	}
	return d
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

func hintCardFixed(th *material.Theme, gtx layout.Context, typeface font.Typeface, sp *protocols.Span) layout.Dimensions {
	return card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			title := "Hover a field"
			line2 := ""
			line3 := ""
			if sp != nil {
				title = sp.Name
				line2 = formatRange(sp.Start, sp.End)
				if sp.Value != "" {
					line2 += "   " + sp.Value
				}
				line3 = sp.Desc
			}

			l1 := material.Body2(th, title)
			l1.Font.Typeface = typeface
			l1.TextSize = scaleThemeFontSize(th, 12)
			l1.MaxLines = 1
			l1.Color = txtColor
			if sp == nil {
				l1.Color = hintColor
			}

			l2 := material.Caption(th, line2)
			l2.Font.Typeface = typeface
			l2.TextSize = scaleThemeFontSize(th, 11)
			l2.MaxLines = 1
			l2.Color = hintColor

			l3 := material.Caption(th, line3)
			l3.Font.Typeface = typeface
			l3.TextSize = scaleThemeFontSize(th, 11)
			l3.MaxLines = 2
			l3.Color = txtColor

			return minHeight(gtx, gtx.Dp(unit.Dp(60)), func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(l1.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if line2 == "" {
							return layout.Dimensions{}
						}
						return l2.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if line3 == "" {
							return layout.Dimensions{}
						}
						return l3.Layout(gtx)
					}),
				)
			})
		})
	})
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

func findFlatRowIndex(rows []flatSpan, rowID string) int {
	for i, row := range rows {
		if row.Key == rowID {
			return i
		}
	}
	return -1
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

func filterTopLevelSpans(spans []*protocols.Span, total int) []*protocols.Span {
	out := make([]*protocols.Span, 0, len(spans))
	for _, sp := range spans {
		if sp == nil {
			continue
		}
		if sp.Start < 0 || sp.End > total || sp.End <= sp.Start {
			continue
		}
		out = append(out, sp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Start == out[j].Start {
			return out[i].End < out[j].End
		}
		return out[i].Start < out[j].Start
	})
	return out
}

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

func parseHexLine(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
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

func card(gtx layout.Context, w layout.Widget) layout.Dimensions {
	bg := color.NRGBA{R: 18, G: 22, B: 30, A: 255}
	border := color.NRGBA{R: 255, G: 255, B: 255, A: 18}

	// Record child to measure.
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()

	fullW := gtx.Constraints.Max.X
	if fullW < dims.Size.X {
		fullW = dims.Size.X
	}
	rect := image.Rect(0, 0, fullW, dims.Size.Y)
	rr := clip.UniformRRect(rect, gtx.Dp(unit.Dp(14)))
	// Draw bg BEFORE content.
	paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())

	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(fullW, dims.Size.Y)}
}

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
