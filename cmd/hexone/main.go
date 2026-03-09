package main

import (
	"fmt"
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

func buildFontCollection(cfg *fm.Config) ([]text.FontFace, error) {
	collection := make([]text.FontFace, 0, 12)
	for _, family := range resources.BundledFontFamilies() {
		collection = appendTypefaceFaces(collection, family.Name, family.RegularPath, family.MediumPath, family.BoldPath)
	}
	if cfg == nil || resources.IsBundledFontFamily(cfg.Font.Typeface) {
		return collection, nil
	}
	if cfg.Font.Typeface == "" {
		return nil, fmt.Errorf("font.typeface is empty")
	}
	if cfg.Font.RegularPath == "" || cfg.Font.MediumPath == "" || cfg.Font.BoldPath == "" {
		return nil, fmt.Errorf("custom pane font %q requires regular_path, medium_path, and bold_path", cfg.Font.Typeface)
	}
	collection = appendTypefaceFaces(collection, cfg.Font.Typeface, cfg.Font.RegularPath, cfg.Font.MediumPath, cfg.Font.BoldPath)
	return collection, nil
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
	collection, err := buildFontCollection(cfg)
	if err != nil {
		return err
	}

	th.Shaper = text.NewShaper(text.WithCollection(collection))
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
