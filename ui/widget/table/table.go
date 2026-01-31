package table

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
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
	Width unit.Dp
	Flex  bool
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

	OnActivate func(row int)
	OnSelect   func(row int)

	TextSize   unit.Sp
	RowHeight  unit.Dp
	RowPadY    unit.Dp
	Bg         color.NRGBA
	HoverBg    color.NRGBA
	SelectedBg color.NRGBA
	SelectedFg *color.NRGBA

	tag           struct{}
	rowClicks     []widget.Clickable
	pendingScroll bool
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

func (t *Table) requestFocus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: &t.tag})
}

func fillRect(gtx layout.Context, c color.NRGBA, size image.Point) {
	paint.FillShape(gtx.Ops, c, clip.Rect(image.Rectangle{Max: size}).Op())
}

func (t *Table) scrollSelectionIntoView(viewportHpx int, rowHpx int, n int) {
	if n <= 0 || t.Selected < 0 {
		return
	}
	if rowHpx < 1 {
		rowHpx = 1
	}
	visible := viewportHpx / rowHpx
	if visible < 1 {
		visible = 1
	}

	first := t.List.Position.First
	last := first + visible - 1

	if t.Selected < first {
		t.List.Position.First = t.Selected
		t.List.Position.Offset = 0
	} else if t.Selected > last {
		newFirst := t.Selected - (visible - 1)
		if newFirst < 0 {
			newFirst = 0
		}
		t.List.Position.First = newFirst
		t.List.Position.Offset = 0
	}
	t.clampListPos(n)
}

func (t *Table) notifySelect(prev int) {
	if t.OnSelect != nil && t.Selected != prev {
		t.OnSelect(t.Selected)
	}
}

func (t *Table) handleKeys(gtx layout.Context, n int, viewportHpx int, rowHpx int) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameUpArrow},
			key.Filter{Name: key.NameDownArrow},
			key.Filter{Name: key.NamePageUp},
			key.Filter{Name: key.NamePageDown},
			key.Filter{Name: key.NameHome},
			key.Filter{Name: key.NameEnd},
			key.Filter{Name: key.NameReturn},
			key.Filter{Name: key.NameEnter},
		)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}

		prev := t.Selected

		switch ke.Name {
		case key.NameUpArrow:
			t.Selected--
		case key.NameDownArrow:
			t.Selected++
		case key.NameHome:
			t.Selected = 0
		case key.NameEnd:
			t.Selected = n - 1
		case key.NamePageUp:
			step := viewportHpx / rowHpx
			if step < 1 {
				step = 10
			}
			t.Selected -= step
		case key.NamePageDown:
			step := viewportHpx / rowHpx
			if step < 1 {
				step = 10
			}
			t.Selected += step
		case key.NameReturn, key.NameEnter:
			if t.OnActivate != nil && t.Selected >= 0 && t.Selected < n {
				t.OnActivate(t.Selected)
			}
		}

		t.clampSelection(n)
		t.scrollSelectionIntoView(viewportHpx, rowHpx, n)
		t.notifySelect(prev)
	}
}

func (t *Table) Focus(gtx layout.Context) { gtx.Execute(key.FocusCmd{Tag: &t.tag}) }

func (t *Table) Layout(th *material.Theme, gtx layout.Context, m Model) layout.Dimensions {

	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, &t.tag)
	area.Pop()

	event.Op(gtx.Ops, &t.tag)

	n := 0
	if m != nil {
		n = m.Len()
	}

	t.ensureClicks(n)
	t.clampSelection(n)
	t.clampListPos(n)

	fillRect(gtx, t.Bg, gtx.Constraints.Max)

	outer := layout.UniformInset(unit.Dp(10))
	return outer.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		viewportHpx := gtx.Constraints.Max.Y

		rowHpx := gtx.Dp(t.RowHeight)
		if rowHpx < 1 {
			rowHpx = 1
		}

		if t.pendingScroll {
			t.pendingScroll = false
			t.scrollSelectionIntoView(viewportHpx, rowHpx, n)
		}

		t.handleKeys(gtx, n, viewportHpx, rowHpx)

		return t.List.Layout(gtx, n, func(gtx layout.Context, row int) layout.Dimensions {
			if row < 0 || row >= len(t.rowClicks) {
				return layout.Dimensions{}
			}

			gtx.Constraints.Min.Y = rowHpx
			gtx.Constraints.Max.Y = rowHpx

			click := &t.rowClicks[row]

			for click.Clicked(gtx) {
				prev := t.Selected
				t.Selected = row
				t.clampSelection(n)
				t.pendingScroll = true
				t.requestFocus(gtx)
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
