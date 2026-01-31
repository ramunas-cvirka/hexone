package main

import (
	"hexone/ui"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget/material"
)

func main() {
	go func() {
		window := new(app.Window)
		err := run(window)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func mustFont(path string) font.Face {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	face, err := opentype.Parse(b)
	if err != nil {
		panic(err)
	}
	return face
}

func run(window *app.Window) error {
	th := material.NewTheme()

	regular := mustFont("assets/FiraCode-Regular.ttf")
	medium := mustFont("assets/FiraCode-Medium.ttf")
	bold := mustFont("assets/FiraCode-Bold.ttf")

	th.Shaper = text.NewShaper(text.WithCollection([]text.FontFace{
		{Font: font.Font{Typeface: "Fira Code", Weight: font.Normal}, Face: regular},
		{Font: font.Font{Typeface: "Fira Code", Weight: font.Medium}, Face: medium},
		{Font: font.Font{Typeface: "Fira Code", Weight: font.Bold}, Face: bold},
	}))

	var ops op.Ops
	mainUI := ui.NewUI()

	for {
		switch typ := window.Event().(type) {
		case app.DestroyEvent:
			os.Exit(0)
		case app.FrameEvent:
			gtx := app.NewContext(&ops, typ)
			mainUI.Layout(th, gtx)
			typ.Frame(gtx.Ops)
		}
	}
}

// func decodeHex(hexStr string) string {
// 	hexStr = strings.TrimSpace(hexStr)
// 	if hexStr == "" {
// 		return ""
// 	}

// 	data, err := hex.DecodeString(hexStr)
// 	if err != nil {
// 		// if not valid hex, just return original input
// 		return hexStr
// 	}

// 	var b strings.Builder
// 	b.Grow(len(data))

// 	for _, v := range data {
// 		if v >= 0x20 && v <= 0x7E {
// 			b.WriteByte(v)
// 		} else {
// 			b.WriteRune('�')
// 		}
// 	}

// 	return b.String()
// }

// ---- State ----

// var (
// 	txtColor  = color.NRGBA{R: 210, G: 210, B: 210, A: 255}
// 	hintColor = color.NRGBA{R: 140, G: 140, B: 140, A: 255}
// )

// type UI struct {
// 	Tabs widget.Enum // selected tab key: "tab0" / "tab1" / "tab2"

// 	LeftEd  widget.Editor
// 	RightEd widget.Editor

// 	LeftInfo string

// 	// naive "on change" detection
// 	leftPrev  string
// 	rightPrev string

// 	// Tab buttons
// 	tab0, tab1, tab2 widget.Clickable
// }

// func NewUI() *UI {
// 	ui := &UI{}
// 	ui.Tabs.Value = "tab0"

// 	ui.LeftEd.SingleLine = false
// 	ui.LeftEd.Submit = false

// 	ui.RightEd.SingleLine = false
// 	ui.RightEd.Submit = false

// 	ui.LeftInfo = "0 bytes"

// 	return ui
// }

// // Call this each frame; if text changed, handle it.
// func (ui *UI) handleEditorChanges() {
// 	lt := ui.LeftEd.Text()
// 	if lt != ui.leftPrev {
// 		ui.leftPrev = lt
// 		ui.onLeftTextChanged(lt)
// 	}

// 	rt := ui.RightEd.Text()
// 	if rt != ui.rightPrev {
// 		ui.rightPrev = rt
// 		ui.onRightTextChanged(rt)
// 	}
// }

// func (ui *UI) onLeftTextChanged(text string) {

// 	if len(text)%2 == 1 {
// 		return
// 	}

// 	// Avoid pointless resets + cursor jumps.
// 	if text == ui.RightEd.Text() {
// 		return
// 	}

// 	// SetText updates the editor content.
// 	rText := decodeHex(text)

// 	ui.RightEd.SetText(rText)
// 	ui.LeftInfo = strconv.Itoa(len(text)/2) + " bytes"

// 	// Keep your change-detector in sync so it doesn't immediately fire
// 	// onRightTextChanged for this programmatic update (optional).
// 	ui.rightPrev = rText
// }
// func (ui *UI) onRightTextChanged(text string) {}

// ---- Drawing helpers ----

// // Simple vertical rule (subtle). Use inside a fixed-width "gutter".
// func vRule(gtx layout.Context, w unit.Dp) layout.Dimensions {
// 	width := gtx.Dp(w)
// 	h := gtx.Constraints.Max.Y
// 	if h < 1 {
// 		h = 1
// 	}
// 	r := image.Rect(0, 0, width, h)
// 	paint.FillShape(gtx.Ops, hintColor, clip.Rect(r).Op())
// 	return layout.Dimensions{Size: image.Pt(width, h)}
// }

// ---- Layout helpers ----

// Top tabs row: centered, closer together.
// func (ui *UI) layoutTabs(th *material.Theme, gtx layout.Context) layout.Dimensions {
// 	tabBtn := func(gtx layout.Context, c *widget.Clickable, key, label string) layout.Dimensions {
// 		if c.Clicked(gtx) {
// 			ui.Tabs.Value = key
// 		}
// 		return material.Button(th, c, label).Layout(gtx)
// 	}

// 	in := layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(12), Right: unit.Dp(12)}
// 	return in.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
// 		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
// 			gap := func(gtx layout.Context) layout.Dimensions {
// 				return layout.Spacer{Width: unit.Dp(8)}.Layout(gtx)
// 			}
// 			return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceSides}.Layout(gtx,
// 				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return tabBtn(gtx, &ui.tab0, "tab0", "Tab 1") }),
// 				layout.Rigid(gap),
// 				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return tabBtn(gtx, &ui.tab1, "tab1", "Tab 2") }),
// 				layout.Rigid(gap),
// 				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return tabBtn(gtx, &ui.tab2, "tab2", "Tab 3") }),
// 			)
// 		})
// 	})
// }

// func (ui *UI) layoutTab0(th *material.Theme, gtx layout.Context) layout.Dimensions {
// 	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
// 		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
// 			outer := layout.UniformInset(unit.Dp(12))
// 			return outer.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
// 				// two editors, with a gutter between them that includes a vertical rule
// 				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
// 					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
// 						// right padding so text isn't glued to the divider
// 						pad := layout.Inset{Right: unit.Dp(6)}
// 						return pad.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
// 							ed := material.Editor(th, &ui.LeftEd, "Left text...")
// 							ed.Color = txtColor
// 							ed.HintColor = hintColor
// 							ed.TextSize = unit.Sp(15)
// 							gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
// 							return ed.Layout(gtx)
// 						})
// 					}),

// 					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
// 						// gutter width between columns
// 						return layout.Stack{}.Layout(gtx,
// 							layout.Expanded(func(gtx layout.Context) layout.Dimensions {
// 								return layout.Spacer{Width: unit.Dp(14)}.Layout(gtx)
// 							}),
// 							layout.Stacked(func(gtx layout.Context) layout.Dimensions {
// 								// vertical line centered within gutter
// 								return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
// 									return vRule(gtx, unit.Dp(1))
// 								})
// 							}),
// 						)
// 					}),

// 					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
// 						// left padding so text isn't glued to the divider
// 						pad := layout.Inset{Left: unit.Dp(6)}
// 						return pad.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
// 							ed := material.Editor(th, &ui.RightEd, "Right text...")
// 							ed.Color = txtColor
// 							ed.HintColor = hintColor
// 							ed.TextSize = unit.Sp(15)
// 							gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
// 							return ed.Layout(gtx)
// 						})
// 					}),
// 				)
// 			})
// 		}),

// 		// Bottom has fixed/min height, padding, and a Body2 label
// 		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
// 			// Force a minimum height for this bottom bar
// 			gtx.Constraints.Min.Y = gtx.Dp(32)

// 			pad := layout.Inset{
// 				Left:   unit.Dp(16),
// 				Right:  unit.Dp(16),
// 				Top:    unit.Dp(10),
// 				Bottom: unit.Dp(10),
// 			}
// 			return pad.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
// 				lbl := material.Body2(th, ui.LeftInfo)
// 				lbl.Color = hintColor
// 				// lbl.Color = ... if you set colors
// 				return layout.W.Layout(gtx, lbl.Layout)
// 			})
// 		}),
// 	)
// }

// func (ui *UI) layoutTabPlaceholder(th *material.Theme, gtx layout.Context, name string) layout.Dimensions {
// 	in := layout.UniformInset(unit.Dp(16))
// 	return in.Layout(gtx, material.H6(th, name).Layout)
// }

// ---- Main frame layout ----

// func (ui *UI) Layout(th *material.Theme, gtx layout.Context) layout.Dimensions {

// 	r := image.Rectangle{Max: gtx.Constraints.Max}
// 	paint.FillShape(gtx.Ops, color.NRGBA{R: 32, G: 32, B: 32, A: 255}, clip.Rect(r).Op())

// 	ui.handleEditorChanges()

// 	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
// 		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
// 			return ui.layoutTabs(th, gtx)
// 		}),
// 		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
// 			switch ui.Tabs.Value {
// 			case "tab0":
// 				return ui.layoutTab0(th, gtx)
// 			case "tab1":
// 				return ui.layoutTabPlaceholder(th, gtx, "Tab 2 content")
// 			case "tab2":
// 				return ui.layoutTabPlaceholder(th, gtx, "Tab 3 content")
// 			default:
// 				return ui.layoutTab0(th, gtx)
// 			}
// 		}),
// 	)
// }
