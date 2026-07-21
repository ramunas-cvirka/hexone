// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package table

import (
	uitheme "hexone/ui/theme"
	"image"
	"image/color"
	"sort"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
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
	// GapBefore reserves explicit space before this column in full mode.
	// It is included in Width calculations but not in the cell's content box.
	GapBefore unit.Dp
	// DropPriority controls full-mode column drop order when width is too small.
	// Smaller values are dropped earlier; ties drop rightmost columns first.
	// Column 0 is always preserved.
	DropPriority int
}

type CellStyle struct {
	Color               color.NRGBA
	Weight              font.Weight
	PreserveColor       bool
	Suffix              string
	SuffixColor         color.NRGBA
	SuffixWeight        font.Weight
	SuffixWeightSet     bool
	SuffixPreserveColor bool
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

// BriefColumnTextWidthModel reports the measured width of the widest
// untruncated first-column label. Brief mode adds its own icon and inset
// reserves, then caps the resulting column at BriefColumnWidth.
type BriefColumnTextWidthModel interface {
	BriefColumnTextWidthPx() int
}

type IconKind uint8

const (
	IconNone IconKind = iota
	IconFile
	IconFolder
	IconParent
	IconBroken
	IconLink
)

type LeadingIcon struct {
	Kind   IconKind
	Color  color.NRGBA
	Widget *widget.Icon
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
	link   *widget.Icon
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
	IsMarked      func(row int) bool

	TextSize    unit.Sp
	Typeface    font.Typeface
	RowHeight   unit.Dp
	RowPadY     unit.Dp
	Bg          color.NRGBA
	HoverBg     color.NRGBA
	HoverFg     *color.NRGBA
	MarkedBg    color.NRGBA
	MarkedFg    *color.NRGBA
	SelectedBg  color.NRGBA
	MarkedSelBg color.NRGBA
	MarkedSelFg *color.NRGBA
	SelectedFg  *color.NRGBA

	BriefColumnWidth unit.Dp // maximum width after measuring brief-mode content
	BriefGap         unit.Dp

	ScrollbarWidth      unit.Dp
	ScrollbarMinThumb   unit.Dp
	ScrollbarTrack      color.NRGBA
	ScrollbarTrackHover color.NRGBA
	ScrollbarThumb      color.NRGBA
	ScrollbarThumbHover color.NRGBA
	ScrollbarThumbDrag  color.NRGBA

	viewRows          int
	partialRowVisible bool
	briefRowsPerCol   int
	briefVisibleCols  int

	rowClicks []widget.Clickable

	// internal: request ensureVisible next frame (after list updated Count)
	pendingEnsure       bool
	lastClickRow        int
	lastClickAt         time.Time
	scrollCarry         float32
	viewportScrollCarry float32
	hitOffset           image.Point
	hitSize             image.Point
	fullModeWidths      []int
	rowHeightPx         int
	briefColPx          int
	briefGapPx          int
	briefLastColExtraPx int
	scrollbarTag        struct{}
	scrollbarVisible    bool
	scrollbarAxis       layout.Axis
	scrollbarTrack      image.Rectangle
	scrollbarThumb      image.Rectangle
	scrollbarHover      bool
	scrollbarDragging   bool
	scrollbarDragID     pointer.ID
	scrollbarDragGrab   int
}

const doubleClickWindow = 400 * time.Millisecond

func New(cols []Column) *Table {
	t := &Table{
		Columns:           cols,
		Selected:          0,
		Mode:              ModeFull,
		TextSize:          unit.Sp(15),
		Typeface:          "",
		RowHeight:         unit.Dp(24),
		RowPadY:           unit.Dp(2),
		Bg:                color.NRGBA{R: 32, G: 32, B: 32, A: 255},
		HoverBg:           color.NRGBA{R: 45, G: 45, B: 45, A: 255},
		MarkedBg:          color.NRGBA{R: 52, G: 64, B: 92, A: 255},
		SelectedBg:        color.NRGBA{R: 60, G: 60, B: 80, A: 255},
		MarkedSelBg:       color.NRGBA{R: 76, G: 92, B: 136, A: 255},
		BriefColumnWidth:  unit.Dp(220),
		BriefGap:          unit.Dp(12),
		ScrollbarWidth:    unit.Dp(10),
		ScrollbarMinThumb: unit.Dp(22),
		ScrollbarTrack: color.NRGBA{
			R: 255, G: 255, B: 255, A: 42,
		},
		ScrollbarTrackHover: color.NRGBA{
			R: 255, G: 255, B: 255, A: 76,
		},
		ScrollbarThumb: color.NRGBA{
			R: 190, G: 202, B: 224, A: 214,
		},
		ScrollbarThumbHover: color.NRGBA{
			R: 214, G: 226, B: 246, A: 234,
		},
		ScrollbarThumbDrag: color.NRGBA{
			R: 232, G: 240, B: 255, A: 248,
		},
	}
	t.List.Axis = layout.Vertical
	return t
}

func (t *Table) textTypeface(th *material.Theme) font.Typeface {
	if t != nil && t.Typeface != "" {
		return t.Typeface
	}
	if th != nil && th.Face != "" {
		return th.Face
	}
	return font.Typeface("sans-serif")
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

func (t *Table) ResetPointerState() {
	if t == nil {
		return
	}
	for i := range t.rowClicks {
		t.rowClicks[i] = widget.Clickable{}
	}
	t.lastClickRow = -1
	t.lastClickAt = time.Time{}
	t.scrollbarHover = false
	t.scrollbarDragging = false
	t.scrollbarDragID = 0
	t.scrollbarDragGrab = 0
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
	if visible := t.fullModeVisibleRows(); visible > 0 {
		return visible
	}
	if t.List.Position.Count > 0 {
		return t.List.Position.Count
	}
	return 10
}

func (t *Table) fullModeVisibleRows() int {
	if t == nil {
		return 0
	}
	visible := t.viewRows
	if t.partialRowVisible {
		visible++
	}
	if visible < 1 {
		visible = 1
	}
	return visible
}

func intersectingRows(height, rowHeight int) int {
	if height <= 0 || rowHeight <= 0 {
		return 1
	}
	rows := height / rowHeight
	if height%rowHeight != 0 {
		rows++
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

func partialRowViewportHeight(height, rowHeight, minimumPartial int) int {
	if height <= 0 || rowHeight <= 0 {
		return height
	}
	completeHeight := height / rowHeight * rowHeight
	remainder := height - completeHeight
	if completeHeight == 0 || remainder == 0 {
		return height
	}
	if minimumPartial < 1 {
		minimumPartial = 1
	}
	if minimumPartial > rowHeight {
		minimumPartial = rowHeight
	}
	if remainder < minimumPartial {
		return completeHeight
	}
	return height
}

func (t *Table) minimumPartialRowHeight(gtx layout.Context, rowHeight int, m Model) int {
	if rowHeight <= 0 {
		return 1
	}
	pad := gtx.Dp(t.RowPadY)
	cellHeight := rowHeight - 2*pad
	if cellHeight < 1 {
		cellHeight = 1
	}
	// Add a small line-metric allowance to the configured text size, then
	// find the bottom edge of that content when vertically centered.
	textHeight := gtx.Sp(t.TextSize) + 2
	if textHeight > cellHeight {
		textHeight = cellHeight
	}
	minimum := pad + (cellHeight-textHeight)/2 + textHeight
	if _, hasIcons := m.(LeadingIconModel); hasIcons {
		// Parent/link glyphs are the tallest leading icons and leave roughly
		// two pixels below them in a normal row.
		iconBottom := rowHeight - 2
		if iconBottom > minimum {
			minimum = iconBottom
		}
	}
	if minimum < 1 {
		minimum = 1
	}
	if minimum > rowHeight {
		minimum = rowHeight
	}
	return minimum
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

func pointInRect(pos image.Point, rect image.Rectangle) bool {
	return pos.X >= rect.Min.X && pos.X < rect.Max.X && pos.Y >= rect.Min.Y && pos.Y < rect.Max.Y
}

func scrollbarRadius(rect image.Rectangle) int {
	r := rect.Dx()
	if rect.Dy() < r {
		r = rect.Dy()
	}
	r /= 2
	if r < 1 {
		r = 1
	}
	return r
}

func fillRoundedRect(gtx layout.Context, rect image.Rectangle, radius int, c color.NRGBA) {
	if c.A == 0 || rect.Dx() <= 0 || rect.Dy() <= 0 {
		return
	}
	if radius <= 0 {
		paint.FillShape(gtx.Ops, c, clip.Rect(rect).Op())
		return
	}
	paint.FillShape(gtx.Ops, c, clip.UniformRRect(rect, radius).Op(gtx.Ops))
}

func layoutCellLabel(gtx layout.Context, th *material.Theme, face font.Typeface, size unit.Sp, txt string, st CellStyle, align text.Alignment, hideIfTruncated bool) layout.Dimensions {
	if st.Suffix != "" && align == text.Start {
		return layoutCellLabelWithSuffix(gtx, th, face, size, txt, st, hideIfTruncated)
	}
	return layoutCellLabelPlain(gtx, th, face, size, txt, st, align, hideIfTruncated)
}

func layoutCellLabelPlain(gtx layout.Context, th *material.Theme, face font.Typeface, size unit.Sp, txt string, st CellStyle, align text.Alignment, hideIfTruncated bool) layout.Dimensions {
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
	labelGtx := gtx
	labelGtx.Constraints.Min.Y = 0

	m := op.Record(gtx.Ops)
	dims, info := lbl.LayoutDetailed(labelGtx, th.Shaper, fontSpec, size, txt, textColor)
	call := m.Stop()
	if !hideIfTruncated || info.Truncated == 0 {
		offsetY := 0
		spareY := gtx.Constraints.Max.Y - dims.Size.Y
		if spareY > 0 {
			offsetY = spareY / 2
			if nudge := uitheme.OpticalTextYOffsetPx(gtx, face, size); nudge > 0 {
				maxExtra := spareY - offsetY
				if maxExtra < 0 {
					maxExtra = 0
				}
				if nudge > maxExtra {
					nudge = maxExtra
				}
				offsetY += nudge
			}
		}
		if offsetY > 0 {
			tr := op.Offset(image.Pt(0, offsetY)).Push(gtx.Ops)
			call.Add(gtx.Ops)
			tr.Pop()
		} else {
			call.Add(gtx.Ops)
		}
	}

	out := dims
	if gtx.Constraints.Max.Y > out.Size.Y {
		out.Size.Y = gtx.Constraints.Max.Y
	}
	out.Size = gtx.Constraints.Constrain(out.Size)
	if out.Baseline > 0 && out.Size.Y > dims.Size.Y {
		out.Baseline += (out.Size.Y - dims.Size.Y) / 2
	}
	return out
}

func layoutCellLabelWithSuffix(gtx layout.Context, th *material.Theme, face font.Typeface, size unit.Sp, txt string, st CellStyle, hideIfTruncated bool) layout.Dimensions {
	base := st
	base.Suffix = ""
	base.SuffixColor = color.NRGBA{}
	base.SuffixWeight = 0
	base.SuffixWeightSet = false
	base.SuffixPreserveColor = false
	prefixGtx := gtx
	prefixGtx.Constraints.Min.X = 0
	prefixDims := layoutCellLabelPlain(prefixGtx, th, face, size, txt, base, text.Start, hideIfTruncated)
	if prefixDims.Size.X >= gtx.Constraints.Max.X {
		return prefixDims
	}

	suffix := CellStyle{
		Color:  st.SuffixColor,
		Weight: st.SuffixWeight,
	}
	if suffix.Color.A == 0 {
		suffix.Color = st.Color
	}
	if !st.SuffixWeightSet {
		suffix.Weight = st.Weight
	}
	suffixGtx := gtx
	remaining := gtx.Constraints.Max.X - prefixDims.Size.X
	if remaining < 0 {
		remaining = 0
	}
	suffixGtx.Constraints = layout.Exact(image.Pt(remaining, gtx.Constraints.Max.Y))
	tr := op.Offset(image.Pt(prefixDims.Size.X, 0)).Push(gtx.Ops)
	suffixDims := layoutCellLabelPlain(suffixGtx, th, face, size, st.Suffix, suffix, text.Start, false)
	tr.Pop()

	out := prefixDims
	if suffixDims.Size.Y > out.Size.Y {
		out.Size.Y = suffixDims.Size.Y
	}
	out.Size.X += suffixDims.Size.X
	if out.Size.X > gtx.Constraints.Max.X {
		out.Size.X = gtx.Constraints.Max.X
	}
	out.Size = gtx.Constraints.Constrain(out.Size)
	return out
}

func leadingIconMetrics(kind IconKind, cellH int) (size, gap int) {
	if cellH <= 0 {
		return 0, 0
	}
	size = cellH - 6
	minSize := 7
	gap = maxInt(2, cellH/6)
	if kind == IconParent {
		size = cellH - 4
		minSize = 10
		gap = maxInt(2, cellH/7)
	}
	if size < minSize {
		size = minSize
	}
	maxSize := cellH - 2
	if maxSize < minSize {
		maxSize = minSize
	}
	if size > maxSize {
		size = maxSize
	}
	return size, gap
}

func canShowLeadingIcon(kind IconKind, contentW, cellH int) bool {
	iconW, gapW := leadingIconMetrics(kind, cellH)
	const minTextPx = 8
	return contentW >= iconW+gapW+minTextPx
}

func adaptiveCellPadX(gtx layout.Context, requested unit.Dp, cellW int) unit.Dp {
	if requested <= 0 || cellW <= 0 {
		return 0
	}
	pad := requested
	const minContentPx = 8
	for pad > 0 && cellW-2*gtx.Dp(pad) < minContentPx {
		pad--
	}
	if pad < 0 {
		return 0
	}
	return pad
}

func adaptiveBriefCellInsets(gtx layout.Context, requested unit.Dp, cellW int) (left, right unit.Dp) {
	if requested <= 0 || cellW <= 0 {
		return 0, 0
	}

	left = adaptiveCellPadX(gtx, requested, cellW)
	right = requested / 2
	if right < 1 {
		right = 1
	}

	const minContentPx = 8
	for right > 0 && cellW-gtx.Dp(left)-gtx.Dp(right) < minContentPx {
		right--
	}
	for left > 0 && cellW-gtx.Dp(left)-gtx.Dp(right) < minContentPx {
		left--
	}
	return left, right
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
	leadingIconSet.parent = mustIcon(widget.NewIcon(mdicons.NavigationSubdirectoryArrowLeft))
	leadingIconSet.broken = mustIcon(widget.NewIcon(mdicons.AlertErrorOutline))
	leadingIconSet.link = mustIcon(widget.NewIcon(mdicons.ContentLink))
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
	case IconLink:
		return leadingIconSet.link
	default:
		return nil
	}
}

// LayoutLeadingIcon renders the same icon, size, and trailing gap used by
// table name cells. It is useful for previews that must match pane rows.
func LayoutLeadingIcon(gtx layout.Context, icon LeadingIcon) layout.Dimensions {
	if icon.Kind == IconNone {
		return layout.Dimensions{Size: image.Pt(0, gtx.Constraints.Max.Y)}
	}
	iconPx, gapPx := leadingIconMetrics(icon.Kind, gtx.Constraints.Max.Y)
	if iconPx > gtx.Constraints.Max.X {
		iconPx = gtx.Constraints.Max.X
	}
	reserve := iconPx
	if reserve > 0 {
		reserve += gapPx
	}
	if reserve > gtx.Constraints.Max.X {
		reserve = gtx.Constraints.Max.X
	}
	ic := icon.Widget
	if ic == nil {
		ic = leadingWidgetIcon(icon.Kind)
	}
	if ic != nil && iconPx > 0 {
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
	return layout.Dimensions{Size: image.Pt(reserve, gtx.Constraints.Max.Y)}
}

func layoutCellLabelWithIcon(gtx layout.Context, th *material.Theme, face font.Typeface, size unit.Sp, txt string, st CellStyle, align text.Alignment, hideIfTruncated bool, icon LeadingIcon) layout.Dimensions {
	if icon.Kind == IconNone || align != text.Start {
		return layoutCellLabel(gtx, th, face, size, txt, st, align, hideIfTruncated)
	}

	iconPx, gapPx := leadingIconMetrics(icon.Kind, gtx.Constraints.Max.Y)
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
		ic := icon.Widget
		if ic == nil {
			ic = leadingWidgetIcon(icon.Kind)
		}
		if ic != nil {
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
	gap := gtx.Dp(c.GapBefore)
	if gap > 0 {
		base += gap
		min += gap
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

	for i := len(t.Columns) - 1; i >= 1 && deficit > 0; i-- {
		_, min := t.columnWidthPx(gtx, t.Columns[i])
		shrinkable := widths[i] - min
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

	for _, i := range t.columnDropOrder() {
		if deficit <= 0 {
			break
		}
		if widths[i] <= 0 {
			continue
		}
		deficit -= widths[i]
		widths[i] = 0
	}
	if deficit > 0 && len(widths) > 0 {
		_, min := t.columnWidthPx(gtx, t.Columns[0])
		shrinkable := widths[0] - min
		if shrinkable > 0 {
			cut := shrinkable
			if cut > deficit {
				cut = deficit
			}
			widths[0] -= cut
			deficit -= cut
		}
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

func (t *Table) columnDropOrder() []int {
	n := len(t.Columns)
	if n <= 1 {
		return nil
	}

	type dropSpec struct {
		index    int
		priority int
	}

	specs := make([]dropSpec, 0, n-1)
	hasCustom := false
	for i := 1; i < n; i++ {
		prio := t.Columns[i].DropPriority
		if prio != 0 {
			hasCustom = true
		}
		specs = append(specs, dropSpec{index: i, priority: prio})
	}

	if !hasCustom {
		order := make([]int, 0, n-1)
		for i := n - 1; i >= 1; i-- {
			order = append(order, i)
		}
		return order
	}

	sort.Slice(specs, func(i, j int) bool {
		if specs[i].priority == specs[j].priority {
			return specs[i].index > specs[j].index
		}
		return specs[i].priority < specs[j].priority
	})

	order := make([]int, 0, len(specs))
	for _, spec := range specs {
		order = append(order, spec.index)
	}
	return order
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

// HandleScrollViewport scrolls rows in full mode or columns in brief mode
// without changing the active row.
func (t *Table) HandleScrollViewport(deltaY float32, n int) bool {
	if t == nil || n <= 0 || deltaY == 0 {
		if t != nil && n <= 0 {
			t.viewportScrollCarry = 0
		}
		return false
	}

	if deltaY > 1 {
		deltaY = 1
	} else if deltaY < -1 {
		deltaY = -1
	}
	if (deltaY > 0 && t.viewportScrollCarry < 0) || (deltaY < 0 && t.viewportScrollCarry > 0) {
		t.viewportScrollCarry = 0
	}
	t.viewportScrollCarry += deltaY

	steps := 0
	for t.viewportScrollCarry >= 1 {
		steps++
		t.viewportScrollCarry--
	}
	for t.viewportScrollCarry <= -1 {
		steps--
		t.viewportScrollCarry++
	}
	if steps == 0 {
		return false
	}

	_, _, maxFirst := t.scrollbarMetrics(n)
	prev := t.List.Position.First
	t.List.Position.First += steps
	if t.List.Position.First < 0 {
		t.List.Position.First = 0
	}
	if t.List.Position.First > maxFirst {
		t.List.Position.First = maxFirst
	}
	t.List.Position.Offset = 0
	return prev != t.List.Position.First
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
		contentY := pos.Y + t.List.Position.Offset
		if contentY < 0 {
			return -1
		}
		row := t.List.Position.First + contentY/t.rowHeightPx
		if row < 0 || row >= n {
			return -1
		}
		return row
	}

	if t.briefColPx <= 0 || t.briefRowsPerCol <= 0 {
		return -1
	}

	col := -1
	x := pos.X
	for i := 0; i < t.briefVisibleCols; i++ {
		colW := t.briefColPx
		if i == t.briefVisibleCols-1 {
			colW += t.briefLastColExtraPx
		}
		if x < colW {
			col = i
			break
		}
		x -= colW
		if i == t.briefVisibleCols-1 {
			break
		}
		if x < t.briefGapPx {
			return -1
		}
		x -= t.briefGapPx
	}
	if col < 0 {
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

func (t *Table) HitColumn(pos image.Point) int {
	pos = pos.Sub(t.hitOffset)
	if pos.X < 0 || pos.Y < 0 || pos.X >= t.hitSize.X || pos.Y >= t.hitSize.Y {
		return -1
	}
	if t.Mode != ModeFull {
		if len(t.Columns) == 0 {
			return -1
		}
		return 0
	}
	if len(t.fullModeWidths) == 0 {
		return -1
	}

	x := 0
	for col, w := range t.fullModeWidths {
		if w <= 0 {
			continue
		}
		if pos.X < x+w {
			return col
		}
		x += w
	}
	return -1
}

func (t *Table) RowRect(row, n int) (image.Rectangle, bool) {
	if t == nil || n <= 0 || row < 0 || row >= n || t.rowHeightPx <= 0 {
		return image.Rectangle{}, false
	}
	switch t.Mode {
	case ModeBrief:
		if t.briefRowsPerCol <= 0 || t.briefVisibleCols <= 0 || t.briefColPx <= 0 {
			return image.Rectangle{}, false
		}
		itemCol := row / t.briefRowsPerCol
		visibleCol := itemCol - t.List.Position.First
		if visibleCol < 0 || visibleCol >= t.briefVisibleCols {
			return image.Rectangle{}, false
		}
		rowInCol := row % t.briefRowsPerCol
		x := t.hitOffset.X
		for i := 0; i < visibleCol; i++ {
			x += t.briefColPx
			if i == t.briefVisibleCols-1 {
				x += t.briefLastColExtraPx
			}
			x += t.briefGapPx
		}
		w := t.briefColPx
		if visibleCol == t.briefVisibleCols-1 {
			w += t.briefLastColExtraPx
		}
		y := t.hitOffset.Y + rowInCol*t.rowHeightPx
		rect := image.Rect(x, y, x+w, y+t.rowHeightPx)
		viewport := image.Rect(t.hitOffset.X, t.hitOffset.Y, t.hitOffset.X+t.hitSize.X, t.hitOffset.Y+t.hitSize.Y)
		if !rect.Overlaps(viewport) {
			return image.Rectangle{}, false
		}
		return rect.Intersect(viewport), true
	default:
		visibleRow := row - t.List.Position.First
		if visibleRow < 0 {
			return image.Rectangle{}, false
		}
		y := t.hitOffset.Y + visibleRow*t.rowHeightPx - t.List.Position.Offset
		rect := image.Rect(t.hitOffset.X, y, t.hitOffset.X+t.hitSize.X, y+t.rowHeightPx)
		viewport := image.Rect(t.hitOffset.X, t.hitOffset.Y, t.hitOffset.X+t.hitSize.X, t.hitOffset.Y+t.hitSize.Y)
		if !rect.Overlaps(viewport) {
			return image.Rectangle{}, false
		}
		return rect.Intersect(viewport), true
	}
}

func (t *Table) CellRect(row, col, n int) (image.Rectangle, bool) {
	rowRect, ok := t.RowRect(row, n)
	if !ok {
		return image.Rectangle{}, false
	}
	if t.Mode != ModeFull {
		if col != 0 {
			return image.Rectangle{}, false
		}
		return rowRect, true
	}
	if col < 0 || col >= len(t.fullModeWidths) {
		return image.Rectangle{}, false
	}
	x := rowRect.Min.X
	for i := 0; i < col; i++ {
		x += t.fullModeWidths[i]
	}
	w := t.fullModeWidths[col]
	if w <= 0 {
		return image.Rectangle{}, false
	}
	return image.Rect(x, rowRect.Min.Y, x+w, rowRect.Max.Y), true
}

func (t *Table) HitScrollbar(pos image.Point) bool {
	return t != nil && t.scrollbarVisible && pointInRect(pos, t.scrollbarTrack)
}

func (t *Table) handleScrollbarEvents(gtx layout.Context, n int) bool {
	if t == nil {
		return false
	}
	if !t.scrollbarVisible && !t.scrollbarDragging {
		t.scrollbarHover = false
		return false
	}

	changed := false
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &t.scrollbarTag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Move | pointer.Enter | pointer.Leave,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		pos := pe.Position.Round()
		switch pe.Kind {
		case pointer.Press:
			if !pe.Buttons.Contain(pointer.ButtonPrimary) || !t.HitScrollbar(pos) {
				continue
			}
			t.scrollbarDragging = true
			t.scrollbarDragID = pe.PointerID
			t.scrollbarDragGrab = t.scrollbarThumbGrab(pos)
			gtx.Execute(pointer.GrabCmd{Tag: &t.scrollbarTag, ID: pe.PointerID})
			if t.setScrollFromScrollbarPos(pos, n) {
				changed = true
			}
			if t.updateScrollbarHover(pos) {
				changed = true
			}
		case pointer.Drag:
			if t.scrollbarDragging && pe.PointerID == t.scrollbarDragID {
				if t.setScrollFromScrollbarPos(pos, n) {
					changed = true
				}
			}
			if t.updateScrollbarHover(pos) {
				changed = true
			}
		case pointer.Release, pointer.Cancel:
			if t.scrollbarDragging && pe.PointerID == t.scrollbarDragID {
				t.scrollbarDragging = false
				t.scrollbarDragGrab = 0
			}
			if t.updateScrollbarHover(pos) {
				changed = true
			}
		case pointer.Move, pointer.Enter:
			if t.updateScrollbarHover(pos) {
				changed = true
			}
		case pointer.Leave:
			if !t.scrollbarDragging {
				if t.scrollbarHover {
					changed = true
				}
				t.scrollbarHover = false
			}
		}
	}
	return changed
}

func (t *Table) updateScrollbarHover(pos image.Point) bool {
	if t == nil || !t.scrollbarVisible {
		if t != nil && !t.scrollbarDragging {
			changed := t.scrollbarHover
			t.scrollbarHover = false
			return changed
		}
		return false
	}
	old := t.scrollbarHover
	t.scrollbarHover = pointInRect(pos, t.scrollbarTrack)
	return old != t.scrollbarHover
}

func (t *Table) scrollbarThumbGrab(pos image.Point) int {
	if t == nil || t.scrollbarThumb.Empty() {
		return 0
	}
	if pointInRect(pos, t.scrollbarThumb) {
		if t.scrollbarAxis == layout.Horizontal {
			return pos.X - t.scrollbarThumb.Min.X
		}
		return pos.Y - t.scrollbarThumb.Min.Y
	}
	if t.scrollbarAxis == layout.Horizontal {
		return t.scrollbarThumb.Dx() / 2
	}
	return t.scrollbarThumb.Dy() / 2
}

func (t *Table) setScrollFromScrollbarPos(pos image.Point, n int) bool {
	items, visible, maxFirst := t.scrollbarMetrics(n)
	if items <= 0 || visible <= 0 || maxFirst <= 0 || t.scrollbarTrack.Empty() || t.scrollbarThumb.Empty() {
		return false
	}

	trackLen := t.scrollbarTrack.Dy()
	thumbLen := t.scrollbarThumb.Dy()
	coord := pos.Y
	trackStart := t.scrollbarTrack.Min.Y
	if t.scrollbarAxis == layout.Horizontal {
		trackLen = t.scrollbarTrack.Dx()
		thumbLen = t.scrollbarThumb.Dx()
		coord = pos.X
		trackStart = t.scrollbarTrack.Min.X
	}
	travel := trackLen - thumbLen
	if travel <= 0 {
		return false
	}

	drag := coord - trackStart - t.scrollbarDragGrab
	if drag < 0 {
		drag = 0
	}
	if drag > travel {
		drag = travel
	}
	first := int(float32(drag)/float32(travel)*float32(maxFirst) + 0.5)
	if first < 0 {
		first = 0
	}
	if first > maxFirst {
		first = maxFirst
	}

	if t.List.Position.First == first && t.List.Position.Offset == 0 {
		return false
	}
	t.List.Position.First = first
	t.List.Position.Offset = 0
	return true
}

func (t *Table) scrollbarMetrics(n int) (items, visible, maxFirst int) {
	if t == nil || n <= 0 {
		return 0, 0, 0
	}
	if t.Mode == ModeBrief {
		items = t.listItemCount(n)
		visible = t.briefVisibleCols
	} else {
		items = n
		visible = t.fullModeVisibleRows()
	}
	if visible < 1 {
		visible = 1
	}
	if visible > items {
		visible = items
	}
	maxFirst = items - visible
	if maxFirst < 0 {
		maxFirst = 0
	}
	return items, visible, maxFirst
}

func (t *Table) scrollbarThicknessPx(gtx layout.Context, minor int) int {
	if t == nil || minor < 6 {
		return 0
	}
	w := gtx.Dp(t.ScrollbarWidth)
	if w < 6 {
		w = 6
	}
	if w >= minor {
		w = minor - 1
	}
	if w < 1 {
		return 0
	}
	return w
}

func (t *Table) resetScrollbarGeometry() {
	if t == nil {
		return
	}
	t.scrollbarVisible = false
	t.scrollbarTrack = image.Rectangle{}
	t.scrollbarThumb = image.Rectangle{}
}

func (t *Table) setScrollbarGeometry(gtx layout.Context, axis layout.Axis, track image.Rectangle, n int) {
	if t == nil {
		return
	}
	t.resetScrollbarGeometry()
	if track.Empty() {
		return
	}
	t.scrollbarAxis = axis
	items, visible, maxFirst := t.scrollbarMetrics(n)
	if items <= 0 || visible <= 0 || maxFirst <= 0 {
		return
	}
	if t.List.Position.First > maxFirst {
		t.List.Position.First = maxFirst
		t.List.Position.Offset = 0
	}
	if t.List.Position.First < 0 {
		t.List.Position.First = 0
		t.List.Position.Offset = 0
	}

	trackLen := track.Dy()
	minorLen := track.Dx()
	if axis == layout.Horizontal {
		trackLen = track.Dx()
		minorLen = track.Dy()
	}
	if trackLen <= 0 || minorLen <= 0 {
		return
	}

	minThumb := gtx.Dp(t.ScrollbarMinThumb)
	if minThumb < 14 {
		minThumb = 14
	}
	if minThumb > trackLen {
		minThumb = trackLen
	}
	thumbLen := int(float32(trackLen) * float32(visible) / float32(items))
	if thumbLen < minThumb {
		thumbLen = minThumb
	}
	if thumbLen > trackLen {
		thumbLen = trackLen
	}

	travel := trackLen - thumbLen
	thumbPos := 0
	if travel > 0 && maxFirst > 0 {
		thumbPos = int(float32(t.List.Position.First)/float32(maxFirst)*float32(travel) + 0.5)
	}
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos > travel {
		thumbPos = travel
	}

	pad := minorLen / 4
	if pad < 1 {
		pad = 1
	}
	if pad*2 >= minorLen {
		pad = 0
	}
	thumb := track
	if axis == layout.Horizontal {
		thumb.Min.X = track.Min.X + thumbPos
		thumb.Max.X = thumb.Min.X + thumbLen
		thumb.Min.Y += pad
		thumb.Max.Y -= pad
	} else {
		thumb.Min.X += pad
		thumb.Max.X -= pad
		thumb.Min.Y = track.Min.Y + thumbPos
		thumb.Max.Y = thumb.Min.Y + thumbLen
	}
	if thumb.Empty() {
		return
	}

	t.scrollbarVisible = true
	t.scrollbarTrack = track
	t.scrollbarThumb = thumb
}

func (t *Table) paintScrollbar(gtx layout.Context) {
	if t == nil || !t.scrollbarVisible {
		return
	}
	trackColor := t.ScrollbarTrack
	thumbColor := t.ScrollbarThumb
	if t.scrollbarHover {
		trackColor = t.ScrollbarTrackHover
		thumbColor = t.ScrollbarThumbHover
	}
	if t.scrollbarDragging {
		thumbColor = t.ScrollbarThumbDrag
	}

	trackRadius := scrollbarRadius(t.scrollbarTrack)
	thumbRadius := scrollbarRadius(t.scrollbarThumb)
	fillRoundedRect(gtx, t.scrollbarTrack, trackRadius, trackColor)
	fillRoundedRect(gtx, t.scrollbarThumb, thumbRadius, thumbColor)
}

func (t *Table) applyScrollbarCursor(gtx layout.Context) {
	if t == nil || (!t.scrollbarVisible && !t.scrollbarDragging) {
		return
	}
	if t.scrollbarDragging {
		pointer.CursorGrabbing.Add(gtx.Ops)
		return
	}
	if !t.scrollbarVisible || t.scrollbarTrack.Empty() {
		return
	}
	defer clip.Rect(t.scrollbarTrack).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
}

func (t *Table) ApplyScrollbarCursor(gtx layout.Context) {
	t.applyScrollbarCursor(gtx)
}

func (t *Table) registerScrollbarInput(gtx layout.Context, size image.Point) {
	if t == nil || (!t.scrollbarVisible && !t.scrollbarDragging) || size.X <= 0 || size.Y <= 0 {
		return
	}
	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	if t.scrollbarVisible {
		defer clip.Rect(t.scrollbarTrack).Push(gtx.Ops).Pop()
	}
	event.Op(gtx.Ops, &t.scrollbarTag)
}

func (t *Table) Layout(th *material.Theme, gtx layout.Context, m Model) layout.Dimensions {
	n := 0
	if m != nil {
		n = m.Len()
	}

	if t.handleScrollbarEvents(gtx, n) {
		gtx.Execute(op.InvalidateCmd{})
	}

	insetPx := gtx.Dp(unit.Dp(2))
	t.hitOffset = image.Pt(insetPx, insetPx)
	t.hitSize = image.Point{}
	t.fullModeWidths = nil
	t.rowHeightPx = 0
	t.partialRowVisible = false
	t.briefColPx = 0
	t.briefGapPx = 0
	t.briefLastColExtraPx = 0
	t.resetScrollbarGeometry()

	t.ensureClicks(n)
	t.clampSelection(n)

	// Background
	fillRect(gtx, t.Bg, gtx.Constraints.Max)

	outer := layout.UniformInset(unit.Dp(2))
	dims := outer.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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

		innerW := gtx.Constraints.Max.X
		innerH := gtx.Constraints.Max.Y
		if innerW < 1 {
			innerW = 1
		}
		if innerH < 1 {
			innerH = 1
		}
		minimumPartialH := t.minimumPartialRowHeight(gtx, rowHpx, m)

		if t.Mode == ModeBrief {
			briefViewportW := innerW
			contentH := innerH
			scrollbarH := t.scrollbarThicknessPx(gtx, innerH)

			listH := partialRowViewportHeight(contentH, rowHpx, minimumPartialH)
			t.viewRows = intersectingRows(listH, rowHpx)
			t.briefRowsPerCol = t.viewRows
			t.briefVisibleCols = 1
			t.computeBriefLayout(th, gtx, m, n, briefViewportW, rowHpx)
			itemCount := t.listItemCount(n)
			needScrollbar := itemCount > t.briefVisibleCols && scrollbarH > 0 && contentH > scrollbarH
			if needScrollbar {
				contentH = innerH - scrollbarH
				if contentH < 1 {
					contentH = 1
				}
				listH = partialRowViewportHeight(contentH, rowHpx, minimumPartialH)
				t.viewRows = intersectingRows(listH, rowHpx)
				t.briefRowsPerCol = t.viewRows
				t.computeBriefLayout(th, gtx, m, n, briefViewportW, rowHpx)
				itemCount = t.listItemCount(n)
			}
			t.hitSize = image.Pt(briefViewportW, listH)
			t.clampListPos(itemCount)

			// If selection changed by click in previous frame, ensure visible now (after Count exists).
			if t.pendingEnsure {
				t.pendingEnsure = false
				t.ensureVisible(n)
			}
			if needScrollbar && itemCount > t.briefVisibleCols {
				track := image.Rect(t.hitOffset.X, t.hitOffset.Y+contentH, t.hitOffset.X+briefViewportW, t.hitOffset.Y+contentH+scrollbarH)
				t.setScrollbarGeometry(gtx, layout.Horizontal, track, n)
			}

			listGtx := gtx
			listGtx.Constraints.Min.X = briefViewportW
			listGtx.Constraints.Max.X = briefViewportW
			listGtx.Constraints.Min.Y = listH
			listGtx.Constraints.Max.Y = listH
			_ = t.layoutBrief(th, listGtx, m, n, rowHpx, itemCount)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}

		contentW := innerW
		scrollbarW := t.scrollbarThicknessPx(gtx, innerW)
		listH := partialRowViewportHeight(innerH, rowHpx, minimumPartialH)
		completeRows := listH / rowHpx
		t.viewRows = completeRows
		if t.viewRows < 1 {
			t.viewRows = 1
		}
		t.partialRowVisible = completeRows > 0 && listH%rowHpx != 0
		needScrollbar := int64(n)*int64(rowHpx) > int64(listH) && scrollbarW > 0 && contentW > scrollbarW
		if needScrollbar {
			contentW = innerW - scrollbarW
			if contentW < 1 {
				contentW = 1
			}
		}
		t.briefRowsPerCol = t.viewRows
		t.briefVisibleCols = 1

		itemCount := t.listItemCount(n)
		t.clampListPos(itemCount)

		// If selection changed by click in previous frame, ensure visible now (after Count exists).
		if t.pendingEnsure {
			t.pendingEnsure = false
			t.ensureVisible(n)
		}

		// Avoid drawing an accidental-looking sliver. Once at least half a row
		// fits, use all remaining height rather than artificially clipping it.
		t.hitSize = image.Pt(contentW, listH)
		if needScrollbar {
			track := image.Rect(t.hitOffset.X+contentW, t.hitOffset.Y, t.hitOffset.X+contentW+scrollbarW, t.hitOffset.Y+innerH)
			t.setScrollbarGeometry(gtx, layout.Vertical, track, n)
		}

		listGtx := gtx
		listGtx.Constraints.Min.X = contentW
		listGtx.Constraints.Max.X = contentW
		listGtx.Constraints.Min.Y = listH
		listGtx.Constraints.Max.Y = listH
		_ = t.layoutFull(th, listGtx, m, n, rowHpx)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	if !t.scrollbarVisible && !t.scrollbarDragging {
		t.scrollbarHover = false
	}
	t.paintScrollbar(gtx)
	t.applyScrollbarCursor(gtx)
	t.registerScrollbarInput(gtx, dims.Size)
	return dims
}

func (t *Table) rowColors(row int, hovered, marked bool) (color.NRGBA, *color.NRGBA) {
	if row == t.Selected {
		if marked && t.MarkedSelBg.A != 0 {
			return t.MarkedSelBg, t.MarkedSelFg
		}
		return t.SelectedBg, t.SelectedFg
	}
	if marked {
		return t.MarkedBg, t.MarkedFg
	}
	if hovered {
		return t.HoverBg, t.HoverFg
	}
	return color.NRGBA{}, nil
}

func applyRowForeground(st CellStyle, fg *color.NRGBA) CellStyle {
	if fg != nil && !st.PreserveColor {
		st.Color = *fg
	}
	if fg != nil && st.Suffix != "" && !st.SuffixPreserveColor {
		st.SuffixColor = *fg
	}
	return st
}

func (t *Table) layoutFull(th *material.Theme, gtx layout.Context, m Model, n, rowHpx int) layout.Dimensions {
	face := t.textTypeface(th)
	cachedWidth := -1
	var cachedColumnWidths []int
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

		marked := t.IsMarked != nil && t.IsMarked(row)
		bg, fg := t.rowColors(row, click.Hovered(), marked)

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

				if maxW != cachedWidth {
					cachedWidth = maxW
					cachedColumnWidths = t.computeColumnWidths(gtx, maxW)
					t.fullModeWidths = append(t.fullModeWidths[:0], cachedColumnWidths...)
				}
				widths := cachedColumnWidths
				aware, awareOK := m.(WidthAwareModel)
				iconModel, iconOK := m.(LeadingIconModel)
				x := 0
				for col := 0; col < len(t.Columns); col++ {
					c := t.Columns[col]
					w := widths[col]
					if w < 0 {
						w = 0
					}

					gapBefore := gtx.Dp(c.GapBefore)
					if gapBefore > w {
						gapBefore = w
					}
					cellW := w - gapBefore
					padX := adaptiveCellPadX(gtx, c.PadX, cellW)
					contentW := cellW - 2*gtx.Dp(padX)
					if contentW < 0 {
						contentW = 0
					}
					icon := LeadingIcon{}
					hasIcon := false
					if col == 0 && iconOK {
						icon, hasIcon = iconModel.LeadingIcon(row, col)
						if hasIcon && icon.Kind != IconNone {
							if !canShowLeadingIcon(icon.Kind, contentW, cellH) {
								hasIcon = false
							}
						}
						if hasIcon && icon.Kind != IconNone {
							iconW, gapW := leadingIconMetrics(icon.Kind, cellH)
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
					st = applyRowForeground(st, fg)

					align := text.Start
					switch c.Align {
					case AlignEnd:
						align = text.End
					case AlignCenter:
						align = text.Middle
					}

					cellGtx := gtx
					cellGtx.Constraints = layout.Exact(image.Pt(cellW, cellH))

					tr := op.Offset(image.Pt(x+gapBefore, 0)).Push(gtx.Ops)
					hideIfTruncated := false
					_ = layout.Inset{Left: padX, Right: padX}.Layout(cellGtx, func(gtx layout.Context) layout.Dimensions {
						if hasIcon && icon.Kind != IconNone {
							return layoutCellLabelWithIcon(gtx, th, face, t.TextSize, txt, st, align, hideIfTruncated, icon)
						}
						return layoutCellLabel(gtx, th, face, t.TextSize, txt, st, align, hideIfTruncated)
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
			innerW := t.briefColumnWidthPx(col)
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

func (t *Table) computeBriefLayout(th *material.Theme, gtx layout.Context, m Model, n, viewportW, rowHpx int) {
	gapW := gtx.Dp(t.BriefGap)
	if gapW < 0 {
		gapW = 0
	}
	if viewportW < 1 {
		viewportW = 1
	}

	rowsPerCol := t.briefRowsPerCol
	if rowsPerCol < 1 {
		rowsPerCol = 1
	}

	totalCols := 0
	if n > 0 {
		totalCols = (n + rowsPerCol - 1) / rowsPerCol
	}

	visibleCols := 1
	if totalCols > 0 {
		targetColW := t.briefTargetColumnWidthPx(th, gtx, m, n, rowHpx)
		visibleCols = (viewportW + gapW) / maxInt(1, targetColW+gapW)
		if visibleCols < 1 {
			visibleCols = 1
		}
		if visibleCols > totalCols {
			visibleCols = totalCols
		}
	}

	gapCount := visibleCols - 1
	if gapCount < 0 {
		gapCount = 0
	}
	totalGap := gapCount * gapW
	available := viewportW - totalGap
	if available < visibleCols {
		available = visibleCols
	}
	dynamicColW := available / maxInt(1, visibleCols)
	if dynamicColW < 1 {
		dynamicColW = 1
	}
	extra := available - dynamicColW*visibleCols
	if extra < 0 {
		extra = 0
	}

	t.briefVisibleCols = visibleCols
	t.briefColPx = dynamicColW
	t.briefGapPx = gapW
	t.briefLastColExtraPx = extra
}

func (t *Table) briefTargetColumnWidthPx(th *material.Theme, gtx layout.Context, m Model, n, rowHpx int) int {
	configuredMax := gtx.Dp(t.BriefColumnWidth)
	if configuredMax < 1 {
		configuredMax = 1
	}

	minWidth := 1
	padLeft := unit.Dp(0)
	padRight := unit.Dp(0)
	if len(t.Columns) > 0 {
		if configured := gtx.Dp(t.Columns[0].MinWidth); configured > minWidth {
			minWidth = configured
		}
		padLeft, padRight = adaptiveBriefCellInsets(gtx, t.Columns[0].PadX, configuredMax)
	}
	if minWidth > configuredMax {
		minWidth = configuredMax
	}

	textWidth := t.briefModelTextWidthPx(th, gtx, m, n, rowHpx)
	target := textWidth + gtx.Dp(padLeft) + gtx.Dp(padRight)
	if _, hasIcons := m.(LeadingIconModel); hasIcons && n > 0 {
		iconWidth, iconGap := leadingIconMetrics(IconParent, rowHpx-2*gtx.Dp(t.RowPadY))
		target += iconWidth + iconGap
	}
	if target < minWidth {
		target = minWidth
	}
	if target > configuredMax {
		target = configuredMax
	}
	return target
}

func (t *Table) briefModelTextWidthPx(th *material.Theme, gtx layout.Context, m Model, n, rowHpx int) int {
	if m == nil || n <= 0 {
		return 0
	}
	if measured, ok := m.(BriefColumnTextWidthModel); ok {
		if width := measured.BriefColumnTextWidthPx(); width > 0 {
			return width
		}
	}
	if th == nil || th.Shaper == nil {
		return 0
	}

	face := t.textTypeface(th)
	maxWidth := 0
	for row := 0; row < n; row++ {
		txt, st := m.Cell(row, 0)
		var measureOps op.Ops
		measureGtx := gtx
		measureGtx.Ops = &measureOps
		measureGtx.Constraints = layout.Constraints{
			Max: image.Pt(1<<20, maxInt(1, rowHpx)),
		}
		width := layoutCellLabel(measureGtx, th, face, t.TextSize, txt, st, text.Start, false).Size.X
		if width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}

func (t *Table) briefColumnWidthPx(col int) int {
	w := t.briefColPx
	if w < 1 {
		w = 1
	}
	visible := col - t.List.Position.First
	if visible == t.briefVisibleCols-1 {
		w += t.briefLastColExtraPx
	}
	return w
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (t *Table) layoutBriefRow(th *material.Theme, gtx layout.Context, m Model, row, n, rowHpx int) layout.Dimensions {
	face := t.textTypeface(th)
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

	marked := t.IsMarked != nil && t.IsMarked(row)
	bg, fg := t.rowColors(row, click.Hovered(), marked)

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

			leftPad := unit.Dp(0)
			rightPad := unit.Dp(0)
			if len(t.Columns) > 0 {
				leftPad, rightPad = adaptiveBriefCellInsets(gtx, t.Columns[0].PadX, maxW)
			}
			contentW := maxW - gtx.Dp(leftPad) - gtx.Dp(rightPad)
			if contentW < 0 {
				contentW = 0
			}
			icon := LeadingIcon{}
			hasIcon := false
			if withIcon, ok := m.(LeadingIconModel); ok {
				icon, hasIcon = withIcon.LeadingIcon(row, 0)
				if hasIcon && icon.Kind != IconNone {
					if !canShowLeadingIcon(icon.Kind, contentW, cellH) {
						hasIcon = false
					}
				}
				if hasIcon && icon.Kind != IconNone {
					iconW, gapW := leadingIconMetrics(icon.Kind, cellH)
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
			st = applyRowForeground(st, fg)

			cellGtx := gtx
			cellGtx.Constraints = layout.Exact(image.Pt(maxW, cellH))
			_ = layout.Inset{Left: leftPad, Right: rightPad}.Layout(cellGtx, func(gtx layout.Context) layout.Dimensions {
				if hasIcon && icon.Kind != IconNone {
					return layoutCellLabelWithIcon(gtx, th, face, t.TextSize, txt, st, text.Start, false, icon)
				}
				return layoutCellLabel(gtx, th, face, t.TextSize, txt, st, text.Start, false)
			})

			return layout.Dimensions{Size: image.Pt(maxW, rowHpx)}
		})
	})
}
