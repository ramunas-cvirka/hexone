package main

import (
	"hexone/appicon"
	"hexone/fm"
	"hexone/ui"
	"hexone/windowstate"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

const sessionPath = "fm.session.yaml"

func main() {
	if exportPath := os.Getenv("HEXONE_WRITE_DEFAULT_ICON"); exportPath != "" {
		if err := appicon.WriteICO(exportPath); err != nil {
			log.Fatal(err)
		}
		return
	}
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
	cfg, err := fm.LoadConfigEnsuringFile("fm.yaml")
	if err != nil {
		log.Printf("save default config: %v", err)
	}
	session := fm.LoadSession(sessionPath)
	window.Option(app.Title(appicon.AppTitle))
	windowstate.ApplyWindowOptions(window, session)
	th := material.NewTheme()

	regular := mustFont(cfg.Font.RegularPath)
	medium := mustFont(cfg.Font.MediumPath)
	bold := mustFont(cfg.Font.BoldPath)

	th.Shaper = text.NewShaper(text.WithCollection([]text.FontFace{
		{Font: font.Font{Typeface: font.Typeface(cfg.Font.Typeface), Weight: font.Normal}, Face: regular},
		{Font: font.Font{Typeface: font.Typeface(cfg.Font.Typeface), Weight: font.Medium}, Face: medium},
		{Font: font.Font{Typeface: font.Typeface(cfg.Font.Typeface), Weight: font.Bold}, Face: bold},
	}))
	th.Face = font.Typeface(cfg.Font.Typeface)
	th.TextSize = unit.Sp(cfg.Font.SizeSp)

	var ops op.Ops
	mainUI := ui.NewUI(cfg)
	windowTracker := windowstate.NewTracker(session)
	iconSetter := appicon.NewSetter()
	sessionApplied := false

	for {
		switch typ := window.Event().(type) {
		case app.DestroyEvent:
			snapshot := mainUI.SnapshotSession()
			windowTracker.ApplyToSession(snapshot)
			if err := fm.SaveSession(sessionPath, snapshot); err != nil {
				log.Printf("save session: %v", err)
			}
			os.Exit(0)
		case app.ViewEvent:
			windowTracker.ObserveView(typ)
			iconSetter.HandleViewEvent(typ)
		case app.ConfigEvent:
			windowTracker.ObserveConfig(typ.Config)
		case app.FrameEvent:
			gtx := app.NewContext(&ops, typ)
			mainUI.Layout(th, gtx)
			typ.Frame(gtx.Ops)
			if mainUI.ConsumeWindowCloseRequest() {
				window.Perform(system.ActionClose)
			}
			windowTracker.ObserveFrame(gtx.Metric)
			if !sessionApplied {
				sessionApplied = true
				mainUI.ApplySession(session)
				window.Invalidate()
			}
		}
	}
}
