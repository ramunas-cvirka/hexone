package main

import (
	resources "hexone"
	"hexone/appdata"
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

const defaultExportPNGIconSize = 1024

func main() {
	if exportPath := os.Getenv("HEXONE_WRITE_DEFAULT_ICON_ICNS"); exportPath != "" {
		if err := appicon.WriteICNS(exportPath); err != nil {
			log.Fatal(err)
		}
		return
	}
	if exportPath := os.Getenv("HEXONE_WRITE_DEFAULT_ICON_PNG"); exportPath != "" {
		if err := appicon.WritePNG(exportPath, defaultExportPNGIconSize); err != nil {
			log.Fatal(err)
		}
		return
	}
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
	b, ok := resources.BundledFont(path)
	if !ok {
		var err error
		b, err = os.ReadFile(path)
		if err != nil {
			panic(err)
		}
	}
	face, err := opentype.Parse(b)
	if err != nil {
		panic(err)
	}
	return face
}

func appendTypefaceFaces(dst []text.FontFace, typeface string, regularPath, mediumPath, boldPath string) []text.FontFace {
	regular := mustFont(regularPath)
	medium := mustFont(mediumPath)
	bold := mustFont(boldPath)
	return append(dst,
		text.FontFace{Font: font.Font{Typeface: font.Typeface(typeface), Weight: font.Normal}, Face: regular},
		text.FontFace{Font: font.Font{Typeface: font.Typeface(typeface), Weight: font.Medium}, Face: medium},
		text.FontFace{Font: font.Font{Typeface: font.Typeface(typeface), Weight: font.Bold}, Face: bold},
	)
}

func buildFontCollection() []text.FontFace {
	collection := make([]text.FontFace, 0, 12)
	for _, family := range resources.BundledFontFamilies() {
		collection = appendTypefaceFaces(collection, family.Name, family.RegularPath, family.MediumPath, family.BoldPath)
	}
	return collection
}

func run(window *app.Window) error {
	cfgPath := appdata.ConfigPath()
	sessionPath := appdata.SessionPath()

	cfg, err := fm.LoadConfigEnsuringFile(cfgPath)
	if err != nil {
		log.Printf("save default config: %v", err)
	}
	session := fm.LoadSession(sessionPath)
	window.Option(app.Title(appicon.AppTitle))
	windowstate.ApplyWindowOptions(window, session)
	th := material.NewTheme()
	collection := buildFontCollection()

	th.Shaper = text.NewShaper(text.WithCollection(collection))
	th.Face = font.Typeface(cfg.General.Typeface)
	th.TextSize = unit.Sp(cfg.General.FontSizeSp)

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
