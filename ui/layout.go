package ui

import (
	"hexone/ui/widget/table"
	"image"
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var (
	txtColor  = color.NRGBA{R: 210, G: 210, B: 210, A: 255}
	hintColor = color.NRGBA{R: 140, G: 140, B: 140, A: 255}
)

type DemoRow struct {
	Name string
	Size string
	Date string
	Kind int // 0 normal, 1 dir, 2 warn, 3 bad
}

type DemoModel struct {
	rows []DemoRow
}

const (
	repeatSlow       = 80 * time.Millisecond
	repeatFast       = 25 * time.Millisecond
	repeatAccelAfter = 120 * time.Millisecond
)

type KeyRepeat struct {
	active     bool
	name       string
	next       time.Time
	started    time.Time
	period     time.Duration
	slow       time.Duration
	fast       time.Duration
	accelAfter time.Duration
}

func (m *DemoModel) Len() int { return len(m.rows) }

func (m *DemoModel) Cell(r, c int) (string, table.CellStyle) {
	row := m.rows[r]

	st := table.CellStyle{Color: txtColor, Weight: font.Medium}
	switch row.Kind {
	case 1:
		st.Color = color.NRGBA{R: 170, G: 200, B: 255, A: 255}
	case 2:
		st.Color = color.NRGBA{R: 240, G: 200, B: 120, A: 255}
	case 3:
		st.Color = color.NRGBA{R: 255, G: 120, B: 120, A: 255}
		st.Weight = font.Bold
	}

	switch c {
	case 0:
		return row.Name, st
	case 1:
		return row.Size, st
	default:
		return row.Date, st
	}
}

type UI struct {
	Tabs widget.Enum // selected tab key: "tab0" / "tab1" / "tab2"

	LeftEd  widget.Editor
	RightEd widget.Editor

	LeftInfo string

	// naive "on change" detection
	leftPrev       string
	rightPrev      string
	wantFocusTable bool
	rep            KeyRepeat
	held           map[string]bool // key glyph -> isDown

	// Tab buttons
	tab0, tab1, tab2 widget.Clickable
	tbl              *table.Table
	model            *DemoModel
}

func NewUI() *UI {
	ui := &UI{}
	ui.Tabs.Value = "tab0"

	ui.LeftEd.SingleLine = false
	ui.LeftEd.Submit = false

	ui.RightEd.SingleLine = false
	ui.RightEd.Submit = false

	ui.LeftInfo = "0 bytes"
	ui.held = make(map[string]bool, 16)

	ui.model = &DemoModel{
		rows: []DemoRow{
			{"src/", "<DIR>", "Jan 31 2026", 1},
			{"assets/", "<DIR>", "Jan 12 2026", 1},
			{"main.go", "8.2 KB", "Jan 31 2026", 0},
			{"table.go", "6.5 KB", "Jan 31 2026", 2},
			{"broken_link", "0 B", "—", 3},
			{"src/", "<DIR>", "Jan 31 2026", 1},
			{"assets/", "<DIR>", "Jan 12 2026", 1},
			{"main.go", "8.2 KB", "Jan 31 2026", 0},
			{"table.go", "6.5 KB", "Jan 31 2026", 2},
			{"broken_link", "0 B", "—", 3},
			{"src/", "<DIR>", "Jan 31 2026", 1},
			{"assets/", "<DIR>", "Jan 12 2026", 1},
			{"main.go", "8.2 KB", "Jan 31 2026", 0},
			{"table.go", "6.5 KB", "Jan 31 2026", 2},
			{"broken_link", "0 B", "—", 3},
			{"src/", "<DIR>", "Jan 31 2026", 1},
			{"assets/", "<DIR>", "Jan 12 2026", 1},
			{"main.go", "8.2 KB", "Jan 31 2026", 0},
			{"table.go", "6.5 KB", "Jan 31 2026", 2},
			{"broken_link", "0 B", "—", 3},
			{"src/", "<DIR>", "Jan 31 2026", 1},
			{"assets/", "<DIR>", "Jan 12 2026", 1},
			{"main.go", "8.2 KB", "Jan 31 2026", 0},
			{"table.go", "6.5 KB", "Jan 31 2026", 2},
			{"broken_link", "0 B", "—", 3},
		},
	}

	cols := []table.Column{
		{Width: unit.Dp(220), Flex: true, Align: table.AlignStart, PadX: unit.Dp(8)},
		{Width: unit.Dp(110), Flex: false, Align: table.AlignEnd, PadX: unit.Dp(8)},
		{Width: unit.Dp(180), Flex: false, Align: table.AlignStart, PadX: unit.Dp(8)},
	}
	ui.tbl = table.New(cols)
	ui.tbl.SelectedFg = &color.NRGBA{R: 230, G: 230, B: 255, A: 255}
	ui.tbl.OnActivate = func(row int) {
		ui.LeftInfo = "Activated: " + ui.model.rows[row].Name
	}

	return ui
}

func (ui *UI) resetKeys() {
	ui.rep.active = false
	for k := range ui.held {
		ui.held[k] = false
	}
}

// Top tabs row: centered, closer together.
func (ui *UI) layoutTabs(th *material.Theme, gtx layout.Context) layout.Dimensions {
	tabBtn := func(gtx layout.Context, c *widget.Clickable, key, label string) layout.Dimensions {
		if c.Clicked(gtx) {
			ui.Tabs.Value = key
		}
		return material.Button(th, c, label).Layout(gtx)
	}

	in := layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(12), Right: unit.Dp(12)}
	return in.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gap := func(gtx layout.Context) layout.Dimensions {
				return layout.Spacer{Width: unit.Dp(8)}.Layout(gtx)
			}
			return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceSides}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return tabBtn(gtx, &ui.tab0, "tab0", "Tab 1") }),
				layout.Rigid(gap),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return tabBtn(gtx, &ui.tab1, "tab1", "Tab 2") }),
				layout.Rigid(gap),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return tabBtn(gtx, &ui.tab2, "tab2", "Tab 3") }),
			)
		})
	})
}

// Simple vertical rule (subtle). Use inside a fixed-width "gutter".
func vRule(gtx layout.Context, w unit.Dp) layout.Dimensions {
	width := gtx.Dp(w)
	h := gtx.Constraints.Max.Y
	if h < 1 {
		h = 1
	}
	r := image.Rect(0, 0, width, h)
	paint.FillShape(gtx.Ops, hintColor, clip.Rect(r).Op())
	return layout.Dimensions{Size: image.Pt(width, h)}
}

func (ui *UI) Layout(th *material.Theme, gtx layout.Context) layout.Dimensions {

	r := image.Rectangle{Max: gtx.Constraints.Max}
	paint.FillShape(gtx.Ops, color.NRGBA{R: 32, G: 32, B: 32, A: 255}, clip.Rect(r).Op())

	ui.handleEditorChanges()

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutTabs(th, gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			switch ui.Tabs.Value {
			case "tab0":
				ui.resetKeys()
				return ui.layoutTab0(th, gtx)
			case "tab1":
				ui.resetKeys()
				ui.wantFocusTable = true
				return ui.layoutTab1(th, gtx)
			case "tab2":
				ui.resetKeys()
				return ui.layoutTabPlaceholder(th, gtx, "Tab 3 content")
			default:
				ui.resetKeys()
				return ui.layoutTab0(th, gtx)
			}
		}),
	)
}

func (ui *UI) layoutTabPlaceholder(th *material.Theme, gtx layout.Context, name string) layout.Dimensions {
	in := layout.UniformInset(unit.Dp(16))
	return in.Layout(gtx, material.H6(th, name).Layout)
}
