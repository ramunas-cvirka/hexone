package table

import (
	"image"
	"image/color"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	mdicons "golang.org/x/exp/shiny/materialdesign/icons"
)

type Align uint8

const (
	AlignStart Align = iota
	AlignEnd
	AlignCenter
)

type Column struct {
	Width    unit.Dp // preferred width
	MinWidth unit.Dp // minimum width when shrinking
	Flex     bool    // takes share of remaining width
	Align    Align
	PadX     unit.Dp
}

type CellStyle struct {
	Color  color.NRGBA
	Weight font.Weight
}

type Mode uint8

const (
	ModeFull Mode = iota
	ModeBrief
)

type Model interface {
	Len() int
	Cell(row, col int) (string, CellStyle)
}

type WidthAwareModel interface {
	CellWithWidth(row, col, widthPx int) (string, CellStyle)
}

type IconKind uint8

const (
	IconNone IconKind = iota
	IconFile
	IconFolder
	IconParent
	IconBroken
)

type LeadingIcon struct {
	Kind  IconKind
	Color color.NRGBA
}

type LeadingIconModel interface {
	LeadingIcon(row, col int) (LeadingIcon, bool)
}

var leadingIconSet struct {
	once   sync.Once
	file   *widget.Icon
	folder *widget.Icon
	parent *widget.Icon
	broken *widget.Icon
}

type Table struct {
	List widget.List

	Columns  []Column
	Selected int
	Mode     Mode

	OnActivate    func(row int) // keyboard activation (Enter)
	OnClick       func(row int) // mouse click
	OnDoubleClick func(row int) // mouse double-click
	OnSelect      func(row int) // selection change

	TextSize   unit.Sp
	RowHeight  unit.Dp
	RowPadY    unit.Dp
	Bg         color.NRGBA
	HoverBg    color.NRGBA
	SelectedBg color.NRGBA
	SelectedFg *color.NRGBA

	BriefColumnWidth unit.Dp
	BriefGap         unit.Dp

	viewRows         int
	briefRowsPerCol  int
	briefVisibleCols int

	rowClicks []widget.Clickable

	// internal: request ensureVisible next frame (after list updated Count)
	pendingEnsure bool
	lastClickRow  int
	lastClickAt   time.Time
	scrollCarry   float32
	hitOffset     image.Point
	hitSize       image.Point
	rowHeightPx   int
	briefColPx    int
	briefGapPx    int
}

const doubleClickWindow = 400 * time.Millisecond

func New(cols []Column) *Table {
	t := &Table{
		Columns:          cols,
		Selected:         0,
		Mode:             ModeFull,
		TextSize:         unit.Sp(15),
		RowHeight:        unit.Dp(24),
		RowPadY:          unit.Dp(2),
		Bg:               color.NRGBA{R: 32, G: 32, B: 32, A: 255},
		HoverBg:          color.NRGBA{R: 45, G: 45, B: 45, A: 255},
		SelectedBg:       color.NRGBA{R: 60, G: 60, B: 80, A: 255},
		BriefColumnWidth: unit.Dp(220),
		BriefGap:         unit.Dp(12),
	}
	t.List.Axis = layout.Vertical
	return t
}

func (t *Table) SetMode(mode Mode) {
	if t.Mode == mode {
		return
	}
	t.Mode = mode
	t.List.Position = layout.Position{}
	t.pendingEnsure = true
}

func (t *Table) SetSelected(row, total int, ensureVisible bool) {
	t.Selected = row
	t.clampSelection(total)
	if ensureVisible {
		t.pendingEnsure = true
	}
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

func (t *Table) pageStep() int {
	if t.Mode == ModeBrief {
		rowsPerCol := t.briefRowsPerCol
		if rowsPerCol < 1 {
			rowsPerCol = 1
		}
		cols := t.briefVisibleCols
		if cols < 1 {
			cols = t.List.Position.Count
		}
		if cols < 1 {
			cols = 1
		}
		return rowsPerCol * cols
	}
	if t.List.Position.Count > 0 {
		return t.List.Position.Count
	}
	return 10
}

func (t *Table) columnStep() int {
	if t.briefRowsPerCol > 0 {
		return t.briefRowsPerCol
	}
	return t.pageStep()
}

func (t *Table) listItemCount(n int) int {
	if n <= 0 {
		return 0
	}
	if t.Mode != ModeBrief {
		return n
	}
	step := t.columnStep()
	return (n + step - 1) / step
}

func (t *Table) ensureVisible(n int) {
	if n <= 0 || t.Selected < 0 {
		return
	}

	if t.Mode == ModeBrief {
		visible := t.briefVisibleCols
		if visible < 1 {
			visible = t.List.Position.Count
		}
		if visible < 1 {
			visible = 1
		}
		step := t.columnStep()
		selectedCol := t.Selected / step
		first := t.List.Position.First
		last := first + visible - 1

		if selectedCol < first {
			t.List.Position.First = selectedCol
			t.List.Position.Offset = 0
		} else if selectedCol > last {
			t.List.Position.First = selectedCol - (visible - 1)
			if t.List.Position.First < 0 {
				t.List.Position.First = 0
			}
			t.List.Position.Offset = 0
		}
		t.clampListPos(t.listItemCount(n))
		return
	}

	visible := t.pageStep()
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

func layoutCellLabel(gtx layout.Context, th *material.Theme, face font.Typeface, size unit.Sp, txt string, st CellStyle, align text.Alignment, hideIfTruncated bool) layout.Dimensions {
	colorMacro := op.Record(gtx.Ops)
	paint.ColorOp{Color: st.Color}.Add(gtx.Ops)
	textColor := colorMacro.Stop()

	lbl := widget.Label{
		Alignment: align,
		MaxLines:  1,
		Truncator: "…",
	}
	fontSpec := font.Font{
		Typeface: face,
		Weight:   st.Weight,
	}

	if !hideIfTruncated {
		return lbl.Layout(gtx, th.Shaper, fontSpec, size, txt, textColor)
	}

	m := op.Record(gtx.Ops)
	dims, info := lbl.LayoutDetailed(gtx, th.Shaper, fontSpec, size, txt, textColor)
	call := m.Stop()
	if info.Truncated == 0 {
		call.Add(gtx.Ops)
	}
	return dims
}

func leadingIconMetrics(cellH int) (size, gap int) {
	size = cellH - 6
	if size < 7 {
		size = 7
	}
	if size > 10 {
		size = 10
	}
	gap = 4
	return size, gap
}

func canShowLeadingIcon(contentW, cellH int) bool {
	iconW, gapW := leadingIconMetrics(cellH)
	const minTextPx = 8
	return contentW >= iconW+gapW+minTextPx
}

func mustIcon(ic *widget.Icon, err error) *widget.Icon {
	if err != nil {
		panic(err)
	}
	return ic
}

func loadLeadingIcons() {
	leadingIconSet.file = mustIcon(widget.NewIcon(mdicons.EditorInsertDriveFile))
	leadingIconSet.folder = mustIcon(widget.NewIcon(mdicons.FileFolder))
	leadingIconSet.parent = mustIcon(widget.NewIcon(mdicons.NavigationArrowBack))
	leadingIconSet.broken = mustIcon(widget.NewIcon(mdicons.AlertErrorOutline))
}

func leadingWidgetIcon(kind IconKind) *widget.Icon {
	leadingIconSet.once.Do(loadLeadingIcons)
	switch kind {
	case IconFile:
		return leadingIconSet.file
	case IconFolder:
		return leadingIconSet.folder
	case IconParent:
		return leadingIconSet.parent
	case IconBroken:
		return leadingIconSet.broken
	default:
		return nil
	}
}

func layoutCellLabelWithIcon(gtx layout.Context, th *material.Theme, face font.Typeface, size unit.Sp, txt string, st CellStyle, align text.Alignment, hideIfTruncated bool, icon LeadingIcon) layout.Dimensions {
	if icon.Kind == IconNone || align != text.Start {
		return layoutCellLabel(gtx, th, face, size, txt, st, align, hideIfTruncated)
	}

	iconPx, gapPx := leadingIconMetrics(gtx.Constraints.Max.Y)
	if iconPx > gtx.Constraints.Max.X {
		iconPx = gtx.Constraints.Max.X
	}
	reserve := iconPx
	if reserve > 0 {
		reserve += gapPx
	}
	if reserve < 0 {
		reserve = 0
	}

	if reserve > 0 {
		if ic := leadingWidgetIcon(icon.Kind); ic != nil {
			iconGtx := gtx
			iconGtx.Constraints = layout.Exact(image.Pt(iconPx, iconPx))
			y := (gtx.Constraints.Max.Y - iconPx) / 2
			if y < 0 {
				y = 0
			}
			tr := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			ic.Layout(iconGtx, icon.Color)
			tr.Pop()
		}
	}

	labelW := gtx.Constraints.Max.X - reserve
	if labelW < 0 {
		labelW = 0
	}

	labelGtx := gtx
	labelGtx.Constraints = layout.Exact(image.Pt(labelW, gtx.Constraints.Max.Y))
	tr := op.Offset(image.Pt(reserve, 0)).Push(gtx.Ops)
	dims := layoutCellLabel(labelGtx, th, face, size, txt, st, align, hideIfTruncated)
	tr.Pop()
	if dims.Size.X+reserve > gtx.Constraints.Max.X {
		dims.Size.X = gtx.Constraints.Max.X
	} else {
		dims.Size.X += reserve
	}
	if dims.Size.Y < gtx.Constraints.Max.Y {
		dims.Size.Y = gtx.Constraints.Max.Y
	}
	return dims
}

func (t *Table) columnWidthPx(gtx layout.Context, c Column) (base, min int) {
	base = gtx.Dp(c.Width)
	min = gtx.Dp(c.MinWidth)
	if min < 0 {
		min = 0
	}
	if base < min {
		base = min
	}
	return base, min
}

func (t *Table) computeColumnWidths(gtx layout.Context, maxW int) []int {
	widths := make([]int, len(t.Columns))
	if len(t.Columns) == 0 {
		return widths
	}

	flexIdx := make([]int, 0, len(t.Columns))
	baseSum := 0
	for i, c := range t.Columns {
		base, _ := t.columnWidthPx(gtx, c)
		widths[i] = base
		baseSum += base
		if c.Flex {
			flexIdx = append(flexIdx, i)
		}
	}

	if maxW >= baseSum && len(flexIdx) > 0 {
		extra := maxW - baseSum
		share := extra / len(flexIdx)
		rem := extra % len(flexIdx)
		for _, idx := range flexIdx {
			widths[idx] += share
			if rem > 0 {
				widths[idx]++
				rem--
			}
		}
		return widths
	}

	deficit := baseSum - maxW
	if deficit <= 0 {
		return widths
	}

	for i := len(t.Columns) - 1; i >= 0 && deficit > 0; i-- {
		base, min := t.columnWidthPx(gtx, t.Columns[i])
		shrinkable := base - min
		if shrinkable <= 0 {
			continue
		}
		cut := shrinkable
		if cut > deficit {
			cut = deficit
		}
		widths[i] -= cut
		deficit -= cut
	}

	for i := len(t.Columns) - 1; i >= 1 && deficit > 0; i-- {
		if widths[i] <= 0 {
			continue
		}
		deficit -= widths[i]
		widths[i] = 0
	}

	total := 0
	for _, w := range widths {
		total += w
	}
	if total < maxW && len(flexIdx) > 0 {
		extra := maxW - total
		share := extra / len(flexIdx)
		rem := extra % len(flexIdx)
		for _, idx := range flexIdx {
			widths[idx] += share
			if rem > 0 {
				widths[idx]++
				rem--
			}
		}
		total = maxW
	}
	if total > maxW && len(widths) > 0 {
		widths[0] -= total - maxW
		if widths[0] < 0 {
			widths[0] = 0
		}
	}

	return widths
}

func (t *Table) isDoubleClick(row int, ev widget.Click, now time.Time) bool {
	if ev.NumClicks >= 2 {
		t.lastClickRow = row
		t.lastClickAt = now
		return true
	}
	if t.lastClickRow == row && !t.lastClickAt.IsZero() && now.Sub(t.lastClickAt) <= doubleClickWindow {
		t.lastClickRow = row
		t.lastClickAt = now
		return true
	}
	t.lastClickRow = row
	t.lastClickAt = now
	return false
}

// HandleKey is called from Tab code after tbl.Layout has run at least once this frame.
// It uses the most recent viewport measurements recorded during Layout.
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
	case "←":
		if t.Mode != ModeBrief {
			t.Selected = 0
			break
		}
		t.Selected -= t.columnStep()
	case "→":
		if t.Mode != ModeBrief {
			t.Selected = n - 1
			break
		}
		t.Selected += t.columnStep()
	case "⇞": // PageUp
		step := t.pageStep()
		t.Selected -= step
	case "⇟": // PageDown
		step := t.pageStep()
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

func (t *Table) HandleScrollSelection(deltaY float32, n int) bool {
	if n <= 0 || deltaY == 0 {
		if n <= 0 {
			t.scrollCarry = 0
		}
		return false
	}

	// Gio delivers wheel delta magnitudes that vary a lot by platform/device
	// (for example 120 on Windows, ~10 on X11, fractional on touchpads).
	// Normalize each event to at most a single logical step and only accumulate
	// sub-unit trackpad deltas.
	if deltaY > 1 {
		deltaY = 1
	} else if deltaY < -1 {
		deltaY = -1
	}
	if (deltaY > 0 && t.scrollCarry < 0) || (deltaY < 0 && t.scrollCarry > 0) {
		t.scrollCarry = 0
	}

	t.scrollCarry += deltaY

	steps := 0
	for t.scrollCarry >= 1 {
		steps++
		t.scrollCarry -= 1
	}
	for t.scrollCarry <= -1 {
		steps--
		t.scrollCarry += 1
	}
	if steps == 0 {
		return false
	}

	prev := t.Selected
	t.Selected += steps
	t.clampSelection(n)
	t.ensureVisible(n)
	t.notifySelect(prev)
	return prev != t.Selected
}

func (t *Table) HitRow(pos image.Point, n int) int {
	if n <= 0 || t.rowHeightPx <= 0 {
		return -1
	}

	pos = pos.Sub(t.hitOffset)
	if pos.X < 0 || pos.Y < 0 || pos.X >= t.hitSize.X || pos.Y >= t.hitSize.Y {
		return -1
	}

	if t.Mode != ModeBrief {
		row := t.List.Position.First + pos.Y/t.rowHeightPx
		if row < 0 || row >= n {
			return -1
		}
		return row
	}

	colStride := t.briefColPx + t.briefGapPx
	if colStride <= 0 || t.briefRowsPerCol <= 0 {
		return -1
	}
	col := pos.X / colStride
	if col < 0 || col >= t.briefVisibleCols {
		return -1
	}
	if pos.X%colStride >= t.briefColPx {
		return -1
	}

	rowInCol := pos.Y / t.rowHeightPx
	if rowInCol < 0 || rowInCol >= t.briefRowsPerCol {
		return -1
	}

	itemCol := t.List.Position.First + col
	row := itemCol*t.briefRowsPerCol + rowInCol
	if row < 0 || row >= n {
		return -1
	}
	return row
}

func (t *Table) Layout(th *material.Theme, gtx layout.Context, m Model) layout.Dimensions {
	n := 0
	if m != nil {
		n = m.Len()
	}

	insetPx := gtx.Dp(unit.Dp(2))
	t.hitOffset = image.Pt(insetPx, insetPx)
	t.hitSize = image.Point{}
	t.rowHeightPx = 0
	t.briefColPx = 0
	t.briefGapPx = 0

	t.ensureClicks(n)
	t.clampSelection(n)

	// Background
	fillRect(gtx, t.Bg, gtx.Constraints.Max)

	outer := layout.UniformInset(unit.Dp(2))
	return outer.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// IMPORTANT: clip so table doesn't steal clicks outside (tabs etc.)
		clipArea := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
		defer clipArea.Pop()

		rowHpx := gtx.Dp(t.RowHeight)
		if rowHpx < 1 {
			rowHpx = 1
		}
		t.rowHeightPx = rowHpx

		if t.Mode == ModeBrief {
			t.List.Axis = layout.Horizontal
		} else {
			t.List.Axis = layout.Vertical
		}

		t.viewRows = gtx.Constraints.Max.Y / rowHpx
		if t.viewRows < 1 {
			t.viewRows = 1
		}
		t.briefRowsPerCol = t.viewRows
		t.briefVisibleCols = 1
		briefViewportW := gtx.Constraints.Max.X
		if t.Mode == ModeBrief {
			colW := gtx.Dp(t.BriefColumnWidth)
			if colW < 1 {
				colW = 1
			}
			gapW := gtx.Dp(t.BriefGap)
			if gapW < 0 {
				gapW = 0
			}
			t.briefColPx = colW
			t.briefGapPx = gapW
			itemW := colW + gapW
			if itemW < 1 {
				itemW = 1
			}
			t.briefVisibleCols = (gtx.Constraints.Max.X + gapW) / itemW
			if t.briefVisibleCols < 1 {
				t.briefVisibleCols = 1
			}
			briefViewportW = t.briefVisibleCols * colW
			if t.briefVisibleCols > 1 {
				briefViewportW += (t.briefVisibleCols - 1) * gapW
			}
			if briefViewportW > gtx.Constraints.Max.X {
				briefViewportW = gtx.Constraints.Max.X
			}
			if briefViewportW < 1 {
				briefViewportW = gtx.Constraints.Max.X
			}
			t.hitSize = image.Pt(briefViewportW, gtx.Constraints.Max.Y)
		}

		itemCount := t.listItemCount(n)
		t.clampListPos(itemCount)

		// If selection changed by click in previous frame, ensure visible now (after Count exists).
		if t.pendingEnsure {
			t.pendingEnsure = false
			t.ensureVisible(n)
		}

		if t.Mode == ModeBrief {
			listGtx := gtx
			listGtx.Constraints.Min.X = briefViewportW
			listGtx.Constraints.Max.X = briefViewportW
			return t.layoutBrief(th, listGtx, m, n, rowHpx, itemCount)
		}

		listH := t.viewRows * rowHpx
		if listH > gtx.Constraints.Max.Y {
			listH = gtx.Constraints.Max.Y
		}
		if listH < 1 {
			listH = gtx.Constraints.Max.Y
		}
		t.hitSize = image.Pt(gtx.Constraints.Max.X, listH)
		spareH := gtx.Constraints.Max.Y - listH

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				listGtx := gtx
				listGtx.Constraints.Min.Y = listH
				listGtx.Constraints.Max.Y = listH
				return t.layoutFull(th, listGtx, m, n, rowHpx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if spareH <= 0 {
					return layout.Dimensions{}
				}
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, spareH)}
			}),
		)
	})
}

func (t *Table) layoutFull(th *material.Theme, gtx layout.Context, m Model, n, rowHpx int) layout.Dimensions {
	return t.List.Layout(gtx, n, func(gtx layout.Context, row int) layout.Dimensions {
		if row < 0 || row >= len(t.rowClicks) {
			return layout.Dimensions{}
		}

		// Fixed height rows.
		gtx.Constraints.Min.Y = rowHpx
		gtx.Constraints.Max.Y = rowHpx

		click := &t.rowClicks[row]
		for {
			ev, ok := click.Update(gtx)
			if !ok {
				break
			}
			if t.OnClick != nil {
				t.OnClick(row)
			}
			prev := t.Selected
			t.Selected = row
			t.clampSelection(n)
			t.pendingEnsure = true
			t.notifySelect(prev)
			if t.OnDoubleClick != nil && t.isDoubleClick(row, ev, gtx.Now) {
				t.OnDoubleClick(row)
			}
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

				widths := t.computeColumnWidths(gtx, maxW)
				aware, awareOK := m.(WidthAwareModel)
				iconModel, iconOK := m.(LeadingIconModel)
				x := 0
				for col := 0; col < len(t.Columns); col++ {
					c := t.Columns[col]
					w := widths[col]
					if w < 0 {
						w = 0
					}

					contentW := w - 2*gtx.Dp(c.PadX)
					if contentW < 0 {
						contentW = 0
					}
					icon := LeadingIcon{}
					hasIcon := false
					if col == 0 && iconOK {
						icon, hasIcon = iconModel.LeadingIcon(row, col)
						if hasIcon && icon.Kind != IconNone {
							if !canShowLeadingIcon(contentW, cellH) {
								hasIcon = false
							}
						}
						if hasIcon && icon.Kind != IconNone {
							iconW, gapW := leadingIconMetrics(cellH)
							reserve := iconW + gapW
							if reserve > contentW {
								contentW = 0
							} else {
								contentW -= reserve
							}
						}
					}
					txt, st := m.Cell(row, col)
					if awareOK {
						txt, st = aware.CellWithWidth(row, col, contentW)
					}
					if row == t.Selected && t.SelectedFg != nil {
						st.Color = *t.SelectedFg
					}

					align := text.Start
					switch c.Align {
					case AlignEnd:
						align = text.End
					case AlignCenter:
						align = text.Middle
					}

					cellGtx := gtx
					cellGtx.Constraints = layout.Exact(image.Pt(w, cellH))

					tr := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
					hideIfTruncated := false
					_ = layout.Inset{Left: c.PadX, Right: c.PadX}.Layout(cellGtx, func(gtx layout.Context) layout.Dimensions {
						if hasIcon && icon.Kind != IconNone {
							return layoutCellLabelWithIcon(gtx, th, th.Face, t.TextSize, txt, st, align, hideIfTruncated, icon)
						}
						return layoutCellLabel(gtx, th, th.Face, t.TextSize, txt, st, align, hideIfTruncated)
					})
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
}

func (t *Table) layoutBrief(th *material.Theme, gtx layout.Context, m Model, n, rowHpx, itemCount int) layout.Dimensions {
	rowsPerCol := t.columnStep()
	colW := gtx.Dp(t.BriefColumnWidth)
	if colW < 1 {
		colW = 1
	}

	return t.List.Layout(gtx, itemCount, func(gtx layout.Context, col int) layout.Dimensions {
		start := col * rowsPerCol
		if start >= n {
			return layout.Dimensions{}
		}
		end := start + rowsPerCol
		if end > n {
			end = n
		}

		return layout.Inset{Right: t.BriefGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			innerW := colW
			if gtx.Constraints.Max.X < innerW {
				innerW = gtx.Constraints.Max.X
			}
			if innerW < 1 {
				innerW = 1
			}
			gtx.Constraints.Min.X = innerW
			gtx.Constraints.Max.X = innerW

			children := make([]layout.FlexChild, 0, end-start)
			for row := start; row < end; row++ {
				row := row
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return t.layoutBriefRow(th, gtx, m, row, n, rowHpx)
				}))
			}

			dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			minH := rowsPerCol * rowHpx
			if dims.Size.Y < minH {
				dims.Size.Y = minH
			}
			return dims
		})
	})
}

func (t *Table) layoutBriefRow(th *material.Theme, gtx layout.Context, m Model, row, n, rowHpx int) layout.Dimensions {
	if row < 0 || row >= len(t.rowClicks) {
		return layout.Dimensions{}
	}

	gtx.Constraints.Min.Y = rowHpx
	gtx.Constraints.Max.Y = rowHpx

	click := &t.rowClicks[row]
	for {
		ev, ok := click.Update(gtx)
		if !ok {
			break
		}
		if t.OnClick != nil {
			t.OnClick(row)
		}
		prev := t.Selected
		t.Selected = row
		t.clampSelection(n)
		t.pendingEnsure = true
		t.notifySelect(prev)
		if t.OnDoubleClick != nil && t.isDoubleClick(row, ev, gtx.Now) {
			t.OnDoubleClick(row)
		}
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

			padX := unit.Dp(0)
			if len(t.Columns) > 0 {
				padX = t.Columns[0].PadX
			}
			contentW := maxW - 2*gtx.Dp(padX)
			if contentW < 0 {
				contentW = 0
			}
			icon := LeadingIcon{}
			hasIcon := false
			if withIcon, ok := m.(LeadingIconModel); ok {
				icon, hasIcon = withIcon.LeadingIcon(row, 0)
				if hasIcon && icon.Kind != IconNone {
					if !canShowLeadingIcon(contentW, cellH) {
						hasIcon = false
					}
				}
				if hasIcon && icon.Kind != IconNone {
					iconW, gapW := leadingIconMetrics(cellH)
					reserve := iconW + gapW
					if reserve > contentW {
						contentW = 0
					} else {
						contentW -= reserve
					}
				}
			}
			txt, st := m.Cell(row, 0)
			if aware, ok := m.(WidthAwareModel); ok {
				txt, st = aware.CellWithWidth(row, 0, contentW)
			}
			if row == t.Selected && t.SelectedFg != nil {
				st.Color = *t.SelectedFg
			}

			lbl := material.Label(th, t.TextSize, txt)
			lbl.MaxLines = 1
			lbl.Truncator = "…"
			lbl.Color = st.Color
			lbl.Font.Weight = st.Weight
			lbl.Alignment = text.Start

			cellGtx := gtx
			cellGtx.Constraints = layout.Exact(image.Pt(maxW, cellH))
			_ = layout.Inset{Left: padX, Right: padX}.Layout(cellGtx, func(gtx layout.Context) layout.Dimensions {
				if hasIcon && icon.Kind != IconNone {
					return layoutCellLabelWithIcon(gtx, th, th.Face, t.TextSize, txt, st, text.Start, false, icon)
				}
				return lbl.Layout(gtx)
			})

			return layout.Dimensions{Size: image.Pt(maxW, rowHpx)}
		})
	})
}
