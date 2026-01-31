package table

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type Align uint8

const (
	AlignStart Align = iota
	AlignEnd
	AlignCenter
)

type Column struct {
	Width unit.Dp // base width (minimum if Flex)
	Flex  bool    // takes share of remaining width
	Align Align
	PadX  unit.Dp
}

type CellStyle struct {
	Color  color.NRGBA
	Weight font.Weight
}

type Model interface {
	Len() int
	Cell(row, col int) (string, CellStyle)
}

type Table struct {
	List widget.List

	Columns  []Column
	Selected int

	OnActivate func(row int) // Enter
	OnSelect   func(row int) // selection change

	TextSize   unit.Sp
	RowHeight  unit.Dp
	RowPadY    unit.Dp
	Bg         color.NRGBA
	HoverBg    color.NRGBA
	SelectedBg color.NRGBA
	SelectedFg *color.NRGBA

	rowClicks []widget.Clickable

	// internal: request ensureVisible next frame (after list updated Count)
	pendingEnsure bool
}

func New(cols []Column) *Table {
	t := &Table{
		Columns:    cols,
		Selected:   0,
		TextSize:   unit.Sp(15),
		RowHeight:  unit.Dp(24),
		RowPadY:    unit.Dp(2),
		Bg:         color.NRGBA{R: 32, G: 32, B: 32, A: 255},
		HoverBg:    color.NRGBA{R: 45, G: 45, B: 45, A: 255},
		SelectedBg: color.NRGBA{R: 60, G: 60, B: 80, A: 255},
	}
	t.List.Axis = layout.Vertical
	return t
}

func (t *Table) ensureClicks(n int) {
	if n <= cap(t.rowClicks) {
		t.rowClicks = t.rowClicks[:n]
		return
	}
	old := t.rowClicks
	t.rowClicks = make([]widget.Clickable, n)
	copy(t.rowClicks, old)
}

func (t *Table) clampSelection(n int) {
	if n <= 0 {
		t.Selected = -1
		return
	}
	if t.Selected < 0 {
		t.Selected = 0
	}
	if t.Selected >= n {
		t.Selected = n - 1
	}
}

func (t *Table) clampListPos(n int) {
	if n <= 0 {
		t.List.Position.First = 0
		t.List.Position.Offset = 0
		return
	}
	if t.List.Position.First < 0 {
		t.List.Position.First = 0
		t.List.Position.Offset = 0
		return
	}
	if t.List.Position.First > n-1 {
		t.List.Position.First = n - 1
		t.List.Position.Offset = 0
	}
}

func (t *Table) notifySelect(prev int) {
	if t.OnSelect != nil && prev != t.Selected {
		t.OnSelect(t.Selected)
	}
}

// ensureVisible uses the authoritative List.Position.Count (computed by List.Layout).
func (t *Table) ensureVisible(n int) {
	if n <= 0 || t.Selected < 0 {
		return
	}

	visible := t.List.Position.Count
	if visible < 1 {
		visible = 1
	}

	first := t.List.Position.First
	last := first + visible - 1

	if t.Selected < first {
		t.List.Position.First = t.Selected
		t.List.Position.Offset = 0
	} else if t.Selected > last {
		t.List.Position.First = t.Selected - (visible - 1)
		if t.List.Position.First < 0 {
			t.List.Position.First = 0
		}
		t.List.Position.Offset = 0
	}

	t.clampListPos(n)
}

func fillRect(gtx layout.Context, c color.NRGBA, size image.Point) {
	paint.FillShape(gtx.Ops, c, clip.Rect(image.Rectangle{Max: size}).Op())
}

// HandleKey is called from Tab code after tbl.Layout has run at least once this frame.
// It uses List.Position.Count to scroll correctly.
func (t *Table) HandleKey(name string, n int) bool {
	if n <= 0 {
		return false
	}

	prev := t.Selected

	switch name {
	case "↑":
		t.Selected--
	case "↓":
		t.Selected++
	case "⇞": // PageUp
		step := t.List.Position.Count
		if step < 1 {
			step = 10
		}
		t.Selected -= step
	case "⇟": // PageDown
		step := t.List.Position.Count
		if step < 1 {
			step = 10
		}
		t.Selected += step
	case "⇱": // Home
		t.Selected = 0
	case "⇲": // End
		t.Selected = n - 1
	case "⏎":
		if t.OnActivate != nil && t.Selected >= 0 && t.Selected < n {
			t.OnActivate(t.Selected)
		}
		return true
	default:
		return false
	}

	t.clampSelection(n)
	t.ensureVisible(n)
	t.notifySelect(prev)
	return true
}

func (t *Table) Layout(th *material.Theme, gtx layout.Context, m Model) layout.Dimensions {
	n := 0
	if m != nil {
		n = m.Len()
	}

	t.ensureClicks(n)
	t.clampSelection(n)
	t.clampListPos(n)

	// Background
	fillRect(gtx, t.Bg, gtx.Constraints.Max)

	outer := layout.UniformInset(unit.Dp(10))
	return outer.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// IMPORTANT: clip so table doesn't steal clicks outside (tabs etc.)
		clipArea := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
		defer clipArea.Pop()

		rowHpx := gtx.Dp(t.RowHeight)
		if rowHpx < 1 {
			rowHpx = 1
		}

		// If selection changed by click in previous frame, ensure visible now (after Count exists).
		if t.pendingEnsure {
			t.pendingEnsure = false
			t.ensureVisible(n)
		}

		return t.List.Layout(gtx, n, func(gtx layout.Context, row int) layout.Dimensions {
			if row < 0 || row >= len(t.rowClicks) {
				return layout.Dimensions{}
			}

			// Fixed height rows.
			gtx.Constraints.Min.Y = rowHpx
			gtx.Constraints.Max.Y = rowHpx

			click := &t.rowClicks[row]
			for click.Clicked(gtx) {
				prev := t.Selected
				t.Selected = row
				t.clampSelection(n)
				// ensure visible next frame, after List updates Position.Count
				t.pendingEnsure = true
				t.notifySelect(prev)
			}

			bg := color.NRGBA{}
			if row == t.Selected {
				bg = t.SelectedBg
			} else if click.Hovered() {
				bg = t.HoverBg
			}

			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if bg.A != 0 {
					fillRect(gtx, bg, image.Pt(gtx.Constraints.Max.X, rowHpx))
				}

				return layout.Inset{Top: t.RowPadY, Bottom: t.RowPadY}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					maxW := gtx.Constraints.Max.X
					cellH := rowHpx - 2*gtx.Dp(t.RowPadY)
					if cellH < 1 {
						cellH = 1
					}

					// column widths
					fixedW := 0
					minFlexW := 0
					flexCount := 0
					for _, c := range t.Columns {
						w := gtx.Dp(c.Width)
						if c.Flex {
							flexCount++
							minFlexW += w
						} else {
							fixedW += w
						}
					}
					rem := maxW - fixedW
					if rem < 0 {
						rem = 0
					}
					extra := 0
					if flexCount > 0 && rem > minFlexW {
						extra = rem - minFlexW
					}
					share := 0
					if flexCount > 0 {
						share = extra / flexCount
					}

					x := 0
					for col := 0; col < len(t.Columns); col++ {
						c := t.Columns[col]
						w := gtx.Dp(c.Width)
						if c.Flex {
							w += share
						}
						if w < 0 {
							w = 0
						}

						txt, st := m.Cell(row, col)
						if row == t.Selected && t.SelectedFg != nil {
							st.Color = *t.SelectedFg
						}

						lbl := material.Label(th, t.TextSize, txt)
						lbl.MaxLines = 1
						lbl.Truncator = "…"
						lbl.Color = st.Color
						lbl.Font.Weight = st.Weight
						switch c.Align {
						case AlignEnd:
							lbl.Alignment = text.End
						case AlignCenter:
							lbl.Alignment = text.Middle
						default:
							lbl.Alignment = text.Start
						}

						cellGtx := gtx
						cellGtx.Constraints = layout.Exact(image.Pt(w, cellH))

						tr := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
						_ = layout.Inset{Left: c.PadX, Right: c.PadX}.Layout(cellGtx, lbl.Layout)
						tr.Pop()

						x += w
						if x >= maxW {
							break
						}
					}

					return layout.Dimensions{Size: image.Pt(maxW, rowHpx)}
				})
			})
		})
	})
}
