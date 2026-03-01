package ui

import (
	"hexone/fm"
	"hexone/protocols"
	"image"
	"image/color"
	"os"
	"time"

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

const (
	repeatStartDelay = 180 * time.Millisecond
	repeatSlow       = 80 * time.Millisecond
	repeatFast       = 25 * time.Millisecond
	repeatAccelAfter = 120 * time.Millisecond
)

type KeyRepeat struct {
	active     bool
	pane       int
	name       string
	next       time.Time
	started    time.Time
	period     time.Duration
	slow       time.Duration
	fast       time.Duration
	accelAfter time.Duration
}

type fileOpenRequest struct {
	pane int
	row  int
}

type FieldSpan struct {
	Name      string
	StartByte int
	EndByte   int
	Value     string
	Meaning   string
	Color     color.NRGBA

	click widget.Clickable
}

type tab2State struct {
	hexEd       widget.Editor
	protoChoice widget.Enum // "gt06" | "teltonika_tcp"

	spec *protocols.Spec
	reg  *protocols.DefaultHookRegistry

	lastHexText string
	lastProto   string
	lastBytes   []byte
	lastRes     protocols.Result
	lastErr     string

	// selection/hover are by *span*, not by row2 piece.
	selectedSpanKey string // "start:end"
	hoverSpanKey    string // "start:end"
	hoverSpan       *protocols.Span

	protoDropOpen bool

	hoverRowID    string
	selectedRowID string

	// scroll-to-selected: tracks which rowID we last scrolled to.
	lastScrolledRowID string

	list   layout.List
	clicks map[string]*widget.Clickable
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

	tab2State *tab2State

	// Tab buttons
	tab0, tab1, tab2 widget.Clickable
	filePanes        []*filePaneState
	fmCfg            *fm.Config
	fileKeys         fileKeyMap
	activeFilePane   int
	pendingFileOpen  *fileOpenRequest
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

	data, _ := os.ReadFile("protocols.yaml")

	ui.ensureTab2Loaded(data)
	ui.fmCfg = fm.LoadConfig("fm.yaml")
	ui.fileKeys = newFileKeyMap(ui.fmCfg)

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	ui.filePanes = []*filePaneState{
		newFilePaneState(cwd, ui.fmCfg),
		newFilePaneState(cwd, ui.fmCfg),
	}
	ui.activeFilePane = 0
	for i, pane := range ui.filePanes {
		idx := i
		pane.table.OnClick = func(row int) {
			_ = row
			ui.setActiveFilePane(idx)
		}
		pane.table.OnDoubleClick = func(row int) {
			ui.queueFilePaneOpen(idx, row)
		}
		pane.table.OnActivate = func(row int) {
			ui.queueFilePaneOpen(idx, row)
		}
	}
	return ui
}

func (ui *UI) resetKeys() {
	ui.rep.active = false
	ui.rep.pane = -1
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
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return tabBtn(gtx, &ui.tab0, "tab0", "hex-to-ascii") }),
				layout.Rigid(gap),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return tabBtn(gtx, &ui.tab1, "tab1", "file manager") }),
				layout.Rigid(gap),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return tabBtn(gtx, &ui.tab2, "tab2", "protocol analyzer") }),
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
				ui.wantFocusTable = true
				return ui.layoutTab1(th, gtx)
			case "tab2":
				ui.resetKeys()
				return ui.layoutTab2(th, gtx)
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
