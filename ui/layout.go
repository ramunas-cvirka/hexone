// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	resources "hexone"
	"hexone/fm"
	"hexone/protocols"
	"image"
	"image/color"
	"os"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
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
	defaultUIFontSp  = unit.Sp(14)
	modalFontScale   = 15.0 / 14.0
	toolbarAnimDur   = 260 * time.Millisecond
	toolbarHoverDur  = 120 * time.Millisecond
	toolbarClickDur  = 140 * time.Millisecond
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
	pane           int
	row            int
	systemOpenOnly bool
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
	typeface    font.Typeface

	spec *protocols.Spec
	reg  *protocols.DefaultHookRegistry

	lastHexText string
	lastProto   string
	lastBytes   []byte
	lastRes     protocols.Result
	lastErr     string

	// selection/hover are by *span*, not by row2 piece.
	selectedSpanKey string // "start:end"
	selectedHint    *protocols.Span
	hoverSpanKey    string // "start:end"
	hoverSpan       *protocols.Span
	hoverFromBytes  bool

	protoDropOpen     bool
	protoDropOpenedAt time.Time

	hoverRowID    string
	selectedRowID string

	// scroll-to-selected: tracks which rowID we last scrolled to.
	lastScrolledRowID string

	list   layout.List
	clicks map[string]*widget.Clickable

	selectPressHeld map[string]bool

	hintCopyPulseAt time.Time
}

// Gio event tags must not be zero-sized. Distinct zero-sized fields can share
// the same address, which breaks event routing across handlers that are meant
// to be independent.
type uiEventTag struct {
	_ byte
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
	tab0, tab1, tab2            widget.Clickable
	settingsClick               widget.Clickable
	toolbarPrevTab              string
	toolbarAnimAt               time.Time
	toolbarHoverKey             string
	toolbarHoverPrev            string
	toolbarHoverAt              time.Time
	toolbarPulseKey             string
	toolbarPulseAt              time.Time
	functionBarClicks           [10]widget.Clickable
	functionBarHidden           bool
	functionBarViewerShown      bool
	customCommandMenuOpen       bool
	customCommandMenuButtonRect image.Rectangle
	customCommandMenuRect       image.Rectangle
	customCommandMenuOpenedAt   time.Time
	customCommandMenuHoverID    string
	customCommandMenuHoverAnim  segmentedAnimState
	customCommandMenuSelected   int
	customCommandMenuClicks     []widget.Clickable
	customCommandMenuGlobalTag  uiEventTag
	customCommandMenuBodyTag    uiEventTag
	functionBarToolsOpen        bool
	functionBarToolsButtonRect  image.Rectangle
	functionBarToolsRect        image.Rectangle
	functionBarToolsOpenedAt    time.Time
	functionBarToolsHoverID     string
	functionBarToolsHoverAnim   segmentedAnimState
	functionBarToolsSelected    int
	functionBarToolClicks       []widget.Clickable
	functionBarPopupGlobalTag   uiEventTag
	functionBarPopupBodyTag     uiEventTag
	functionBarSliderPrevIndex  int
	functionBarSliderIndex      int
	functionBarSliderPrevShown  bool
	functionBarSliderShown      bool
	functionBarSliderAnimAt     time.Time
	functionBarHeldMods         key.Modifiers
	requestedWindowClose        bool
	filePanes                   []*filePaneState
	fmCfg                       *fm.Config
	configPath                  string
	typeface                    font.Typeface
	textSize                    unit.Sp
	invalidate                  func()
	fileKeys                    fileKeyMap
	activeFilePane              int
	filePaneTabs                []filePaneTabSet
	tabShortcut                 tabShortcutState
	sortDirPrunedAt             time.Time
	terminal                    *terminalSession
	terminalTabs                terminalTabSet
	runtimeTerminalShell        string
	terminalFocusPointerTag     uiEventTag
	pendingFileOpen             *fileOpenRequest
	fileCopy                    *fileCopyState
	archiveExtract              *archiveExtractState
	fileDelete                  *fileDeleteState
	fileMove                    *fileMoveState
	fileCreate                  *fileCreateState
	filePerm                    *filePermState
	multiRename                 *multiRenameState
	fileViewer                  *fileViewerState
	customCommandEditor         *customCommandEditorState
	helpModal                   *helpModalState
	settingsModal               *settingsModalState
	sshModal                    *sshModalState
	editorMenuOpenID            string
	editorMenuTarget            *widget.Editor
	editorMenuPos               image.Point
	editorMenuPressPos          image.Point
	editorMenuRect              image.Rectangle
	editorMenuOpenedAt          time.Time
	editorMenuHoverAction       string
	editorMenuHoverAnim         segmentedAnimState
	editorMenuTags              map[string]*editorMenuEventTag
	editorMenuCanPaste          bool
	editorMenuUseExplicitCaret  bool
	editorMenuClipboardTarget   *widget.Editor
	editorMenuClipboardUseCaret bool
	editorMenuClipboardTag      uiEventTag
	editorMenuGlobalPointerTag  uiEventTag

	protoDropGlobalPointerTag uiEventTag
}

func NewUI(cfg *fm.Config) *UI {
	if cfg == nil {
		cfg = fm.DefaultConfig()
	}
	ui := &UI{
		fmCfg:                      cfg,
		configPath:                 resolveUIConfigPath(),
		typeface:                   font.Typeface(cfg.General.Typeface),
		textSize:                   fontSizeFromConfig(cfg),
		runtimeTerminalShell:       fm.NormalizeViewerShell(cfg.Viewer.Shell),
		customCommandMenuSelected:  -1,
		functionBarToolsSelected:   -1,
		functionBarSliderPrevIndex: -1,
		functionBarSliderIndex:     -1,
	}
	ui.Tabs.Value = "tab0"

	ui.LeftEd.SingleLine = false
	ui.LeftEd.Submit = false

	ui.RightEd.SingleLine = false
	ui.RightEd.Submit = false

	ui.LeftInfo = "0 bytes"
	ui.held = make(map[string]bool, 16)

	ui.ensureTab2Loaded(resources.ProtocolsYAML())
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
	ui.filePaneTabs = make([]filePaneTabSet, len(ui.filePanes))
	for i, pane := range ui.filePanes {
		ui.installFilePaneHandlers(i, pane)
		ui.filePaneTabs[i].tabs = []*filePaneState{pane}
		ui.requestPaneLoadWithSelection(i, cwd, "", "", 0)
	}
	return ui
}

func (ui *UI) resetKeys() {
	ui.rep.active = false
	ui.rep.pane = -1
	for k := range ui.held {
		ui.held[k] = false
	}
	ui.functionBarHeldMods = 0
}

func (ui *UI) mainTypeface() font.Typeface {
	if ui == nil || ui.typeface == "" {
		return font.Typeface(resources.BundledFontFamilyFiraCodeNerdFontMono)
	}
	return ui.typeface
}

func (ui *UI) viewerTypeface() font.Typeface {
	if ui == nil || ui.fmCfg == nil || ui.fmCfg.Viewer.Typeface == "" {
		return ui.mainTypeface()
	}
	return font.Typeface(ui.fmCfg.Viewer.Typeface)
}

func (ui *UI) viewerMonospaceTypeface() font.Typeface {
	if resources.IsBundledMonospaceFontFamily(string(ui.viewerTypeface())) {
		return ui.viewerTypeface()
	}
	return font.Typeface(resources.BundledFontFamilyFiraCodeNerdFontMono)
}

func (ui *UI) mainTextSize() unit.Sp {
	if ui == nil {
		return defaultUIFontSp
	}
	return normalizeUIFontSize(ui.textSize)
}

func normalizeUIFontSize(size unit.Sp) unit.Sp {
	if size <= 0 {
		return defaultUIFontSp
	}
	return size
}

func fontSizeFromConfig(cfg *fm.Config) unit.Sp {
	if cfg == nil {
		return defaultUIFontSp
	}
	return normalizeUIFontSize(unit.Sp(cfg.General.FontSizeSp))
}

func themeFontSize(th *material.Theme) unit.Sp {
	if th == nil {
		return defaultUIFontSp
	}
	return normalizeUIFontSize(th.TextSize)
}

func scaleFontSize(base, size unit.Sp) unit.Sp {
	base = normalizeUIFontSize(base)
	size = normalizeUIFontSize(size)
	scaled := unit.Sp(float32(size) * float32(base) / float32(defaultUIFontSp))
	if scaled < 1 {
		return 1
	}
	return scaled
}

func scaleThemeFontSize(th *material.Theme, size unit.Sp) unit.Sp {
	return scaleFontSize(themeFontSize(th), size)
}

func scaleModalThemeFontSize(th *material.Theme, size unit.Sp) unit.Sp {
	out := unit.Sp(float32(scaleThemeFontSize(th, size)) * modalFontScale)
	if out < 1 {
		return 1
	}
	return out
}

func scaleDialogThemeFontSize(th *material.Theme, size unit.Sp) unit.Sp {
	return scaleThemeFontSize(th, size)
}

func scaleConfigFontSize(cfg *fm.Config, size unit.Sp) unit.Sp {
	return scaleFontSize(fontSizeFromConfig(cfg), size)
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func smoothstep01(t float32) float32 {
	t = clamp01(t)
	return t * t * (3 - 2*t)
}

func mixNRGBA(a, b color.NRGBA, t float32) color.NRGBA {
	t = clamp01(t)
	return color.NRGBA{
		R: uint8(float32(a.R) + (float32(b.R)-float32(a.R))*t),
		G: uint8(float32(a.G) + (float32(b.G)-float32(a.G))*t),
		B: uint8(float32(a.B) + (float32(b.B)-float32(a.B))*t),
		A: uint8(float32(a.A) + (float32(b.A)-float32(a.A))*t),
	}
}

func (ui *UI) setActiveTab(key string, now time.Time) {
	if ui == nil || key == "" || ui.Tabs.Value == key {
		return
	}
	ui.closeFunctionBarPopups()
	ui.toolbarPrevTab = ui.Tabs.Value
	ui.toolbarAnimAt = now
	ui.Tabs.Value = key
}

func (ui *UI) toolbarTabHighlight(now time.Time, key string) (float32, bool) {
	if ui == nil || key == "" {
		return 0, false
	}
	if ui.toolbarPrevTab == "" || ui.toolbarAnimAt.IsZero() || ui.toolbarPrevTab == ui.Tabs.Value {
		if key == ui.Tabs.Value {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(ui.toolbarAnimAt)
	if elapsed >= toolbarAnimDur {
		ui.toolbarPrevTab = ""
		ui.toolbarAnimAt = time.Time{}
		if key == ui.Tabs.Value {
			return 1, false
		}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarAnimDur))
	t = smoothstep01(t)
	if key == ui.Tabs.Value {
		return t, true
	}
	if key == ui.toolbarPrevTab {
		return 1 - t, true
	}
	return 0, true
}

func fixedHeight(gtx layout.Context, h int, w layout.Widget) layout.Dimensions {
	if h < 1 {
		h = 1
	}
	gtx2 := gtx
	gtx2.Constraints.Min.Y = h
	gtx2.Constraints.Max.Y = h
	return w(gtx2)
}

func (ui *UI) toolbarLabelSize(th *material.Theme) unit.Sp {
	if ui == nil {
		return scaleThemeFontSize(th, 13)
	}
	return scaleConfigFontSize(ui.fmCfg, 13)
}

func fillSegmentBg(gtx layout.Context, bg color.NRGBA, radius int, roundLeft, roundRight bool, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		call.Add(gtx.Ops)
		return dims
	}
	if bg.A != 0 {
		rr := clip.RRect{Rect: image.Rect(0, 0, dims.Size.X, dims.Size.Y)}
		if roundLeft {
			rr.NW = radius
			rr.SW = radius
		}
		if roundRight {
			rr.NE = radius
			rr.SE = radius
		}
		paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
	}
	call.Add(gtx.Ops)
	return dims
}

func (ui *UI) setToolbarHover(key string, now time.Time) {
	if ui == nil {
		return
	}
	if key == ui.toolbarHoverKey {
		return
	}
	ui.toolbarHoverPrev = ui.toolbarHoverKey
	ui.toolbarHoverKey = key
	ui.toolbarHoverAt = now
}

func (ui *UI) toolbarHoverLevel(now time.Time, key string) (float32, bool) {
	if ui == nil || key == "" {
		return 0, false
	}
	if ui.toolbarHoverAt.IsZero() || ui.toolbarHoverPrev == ui.toolbarHoverKey {
		if ui.toolbarHoverKey == key {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(ui.toolbarHoverAt)
	if elapsed >= toolbarHoverDur {
		ui.toolbarHoverPrev = ""
		ui.toolbarHoverAt = time.Time{}
		if ui.toolbarHoverKey == key {
			return 1, false
		}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarHoverDur))
	if key == ui.toolbarHoverKey {
		return t, true
	}
	if key == ui.toolbarHoverPrev {
		return 1 - t, true
	}
	return 0, true
}

func (ui *UI) setToolbarPulse(key string, now time.Time) {
	if ui == nil || key == "" {
		return
	}
	ui.toolbarPulseKey = key
	ui.toolbarPulseAt = now
}

func (ui *UI) toolbarPulseLevel(now time.Time, key string) (float32, bool) {
	if ui == nil || key == "" || ui.toolbarPulseKey != key || ui.toolbarPulseAt.IsZero() {
		return 0, false
	}
	elapsed := now.Sub(ui.toolbarPulseAt)
	if elapsed >= toolbarClickDur {
		ui.toolbarPulseKey = ""
		ui.toolbarPulseAt = time.Time{}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarClickDur))
	return 1 - t, true
}

func (ui *UI) layoutToolbarSegment(th *material.Theme, gtx layout.Context, c *widget.Clickable, label string, activeFill, hoverFill, pulseFill float32, stripH int, roundLeft, roundRight bool) layout.Dimensions {
	if c == nil {
		return layout.Dimensions{}
	}
	dims := fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			activeFill = clamp01(activeFill)
			hoverFill = clamp01(hoverFill)
			pulseFill = clamp01(pulseFill)
			if c.Pressed() && pulseFill < 0.5 {
				pulseFill = 0.5
			}

			baseBlue := color.NRGBA{R: 68, G: 92, B: 180, A: 255}
			hoverDark := color.NRGBA{R: 34, G: 44, B: 66, A: 255}
			hoverLight := color.NRGBA{R: 86, G: 112, B: 204, A: 255}
			pulseCol := color.NRGBA{R: 126, G: 154, B: 255, A: 255}

			bg := mixNRGBA(color.NRGBA{}, baseBlue, activeFill)
			// Inactive tabs darken on hover; active tabs only brighten a bit.
			darkMix := hoverFill * (1 - activeFill)
			lightMix := hoverFill * activeFill * 0.25
			bg = mixNRGBA(bg, hoverDark, darkMix)
			bg = mixNRGBA(bg, hoverLight, lightMix)
			bg = mixNRGBA(bg, pulseCol, pulseFill*0.35)

			fg := mixNRGBA(txtColor, color.NRGBA{R: 240, G: 246, B: 255, A: 255}, activeFill)
			fg = mixNRGBA(fg, color.NRGBA{R: 230, G: 236, B: 255, A: 255}, hoverFill*0.75)
			fg = mixNRGBA(fg, color.NRGBA{R: 245, G: 250, B: 255, A: 255}, pulseFill*0.25)

			radius := gtx.Dp(unit.Dp(filePaneControlCornerDp - 1))
			return fillSegmentBg(gtx, bg, radius, roundLeft, roundRight, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, label)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.Font.Weight = font.Medium
						lbl.TextSize = ui.toolbarLabelSize(th)
						lbl.Color = fg
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					})
				})
			})
		})
	})
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}

	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func toolbarSeparator(gtx layout.Context, stripH int) layout.Dimensions {
	w := gtx.Dp(unit.Dp(1))
	if w < 1 {
		w = 1
	}
	h := stripH
	if h < 1 {
		h = 1
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 22}, clip.Rect(image.Rect(0, 0, w, h)).Op())
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
}

// Top toolbar row: compact connected segments.
func (ui *UI) layoutTabs(th *material.Theme, gtx layout.Context) layout.Dimensions {
	for ui.tab1.Clicked(gtx) {
		ui.setActiveTab("tab1", gtx.Now)
	}
	for ui.tab2.Clicked(gtx) {
		ui.setActiveTab("tab2", gtx.Now)
	}
	for ui.settingsClick.Clicked(gtx) {
		ui.setToolbarPulse("settings", gtx.Now)
		ui.openSettingsModal()
		gtx.Execute(op.InvalidateCmd{})
	}

	fillHex, animHex := ui.toolbarTabHighlight(gtx.Now, "tab1")
	fillProto, animProto := ui.toolbarTabHighlight(gtx.Now, "tab2")
	hoverKey := ""
	if ui.tab1.Hovered() {
		hoverKey = "tab1"
	}
	if ui.tab2.Hovered() {
		hoverKey = "tab2"
	}
	if ui.settingsClick.Hovered() {
		hoverKey = "settings"
	}
	ui.setToolbarHover(hoverKey, gtx.Now)
	hoverHex, hoverAnimHex := ui.toolbarHoverLevel(gtx.Now, "tab1")
	hoverProto, hoverAnimProto := ui.toolbarHoverLevel(gtx.Now, "tab2")
	hoverSettings, hoverAnimSettings := ui.toolbarHoverLevel(gtx.Now, "settings")
	pulseHex, pulseAnimHex := ui.toolbarPulseLevel(gtx.Now, "tab1")
	pulseProto, pulseAnimProto := ui.toolbarPulseLevel(gtx.Now, "tab2")
	pulseSettings, pulseAnimSettings := ui.toolbarPulseLevel(gtx.Now, "settings")
	if animHex || animProto || hoverAnimHex || hoverAnimProto || hoverAnimSettings || pulseAnimHex || pulseAnimProto || pulseAnimSettings {
		gtx.Execute(op.InvalidateCmd{})
	}
	fillSettings := float32(0)
	if ui.settingsModal != nil {
		fillSettings = 1
	}
	stripH := gtx.Dp(unit.Dp(24))
	if stripH < 1 {
		stripH = 1
	}

	in := layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(2), Left: unit.Dp(8), Right: unit.Dp(8)}
	return in.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneControlCornerDp)),
			color.NRGBA{R: 18, G: 22, B: 30, A: 255},
			color.NRGBA{R: 255, G: 255, B: 255, A: 22},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(1), Right: unit.Dp(1), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutToolbarSegment(th, gtx, &ui.tab1, "hex-to-ascii", fillHex, hoverHex, pulseHex, stripH, true, false)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return toolbarSeparator(gtx, stripH)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutToolbarSegment(th, gtx, &ui.tab2, "protocol analyzer", fillProto, hoverProto, pulseProto, stripH, false, false)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return toolbarSeparator(gtx, stripH)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutToolbarSegment(th, gtx, &ui.settingsClick, "settings", fillSettings, hoverSettings, pulseSettings, stripH, false, true)
							}),
						)
					})
				})
			},
		)
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

func (ui *UI) syncThemeRuntime(th *material.Theme) {
	if th == nil {
		return
	}
	th.Face = ui.mainTypeface()
	th.TextSize = ui.mainTextSize()
}

func (ui *UI) Layout(th *material.Theme, gtx layout.Context) layout.Dimensions {
	ui.syncThemeRuntime(th)
	ui.handleFunctionBarModifierKeys(gtx)
	ui.handleGlobalFunctionKeys(gtx)
	ui.handleCustomCommandMenuKeys(gtx)
	ui.handleFunctionBarPopupKeys(gtx)
	ui.handleGlobalEscapeToFileManager(gtx)
	ui.handleEditorContextMenuGlobalPresses(gtx)
	ui.handleEditorContextMenuClipboardEvents(gtx)
	ui.handleTerminalClipboardEvents(gtx)
	ui.handleTerminalOutsidePointerFocus(gtx)
	ui.handleCustomCommandEditorPreLayoutInput(gtx)
	ui.handleMultiRenamePreLayoutInput(gtx)

	r := image.Rectangle{Max: gtx.Constraints.Max}
	paint.FillShape(gtx.Ops, color.NRGBA{R: 32, G: 32, B: 32, A: 255}, clip.Rect(r).Op())

	ui.handleEditorChanges()

	dims := layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if ui == nil || !ui.functionBarVisible() {
						return layout.Dimensions{}
					}
					return ui.layoutFunctionBar(th, gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					switch ui.Tabs.Value {
					case "tab0":
						ui.wantFocusTable = true
						return ui.layoutTab1(th, gtx)
					case "tab1":
						ui.closeFileViewer()
						ui.resetKeys()
						return ui.layoutTab0(th, gtx)
					case "tab2":
						ui.closeFileViewer()
						ui.resetKeys()
						return ui.layoutTab2(th, gtx)
					default:
						ui.wantFocusTable = true
						return ui.layoutTab1(th, gtx)
					}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if ui == nil || ui.terminal == nil || !ui.terminal.active() {
						return layout.Dimensions{}
					}
					return ui.layoutTerminalPane(th, gtx)
				}),
			)
			ui.applyFunctionBarCursor(gtx)
			return dims
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutTerminalResizeHandle(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutCustomCommandMenuPopup(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFunctionBarPopup(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutCustomCommandEditor(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutMultiRename(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutHelpModal(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsModal(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSSHModal(th, gtx)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutEditorContextMenuOverlay(th, gtx)
		}),
	)
	ui.handleProtocolDropdownOutsideClick(gtx)
	ui.registerEditorContextMenuGlobalPointer(gtx)
	ui.registerEditorContextMenuClipboardTarget(gtx)
	ui.registerTerminalClipboardTarget(gtx)
	ui.registerTerminalOutsidePointerFocus(gtx)
	if ui != nil && ui.Tabs.Value == "tab2" && ui.tab2State != nil && ui.tab2State.protoDropOpen {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		pass := pointer.PassOp{}.Push(gtx.Ops)
		event.Op(gtx.Ops, &ui.protoDropGlobalPointerTag)
		pass.Pop()
	}
	ui.consumeUnusedFunctionKeys(gtx)
	return dims
}

func (ui *UI) registerEditorContextMenuGlobalPointer(gtx layout.Context) {
	if ui == nil {
		return
	}
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	if ui.editorMenuOpenID == "" {
		pass := pointer.PassOp{}.Push(gtx.Ops)
		event.Op(gtx.Ops, &ui.editorMenuGlobalPointerTag)
		pass.Pop()
		return
	}
	event.Op(gtx.Ops, &ui.editorMenuGlobalPointerTag)
}

func (ui *UI) handleEditorContextMenuGlobalPresses(gtx layout.Context) {
	if ui == nil {
		return
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &ui.editorMenuGlobalPointerTag,
			Kinds:  pointer.Press | pointer.Move,
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
		case pointer.Move:
			if ui.editorMenuOpenID == "" {
				continue
			}
			hover := ui.editorContextMenuActionAt(gtx, pos)
			if hover != ui.editorMenuHoverAction {
				ui.editorMenuHoverAction = hover
				ui.editorMenuHoverAnim.setHover(hover, gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
			}
		case pointer.Press:
			ui.editorMenuPressPos = pos
			if ui.editorMenuOpenID == "" {
				continue
			}
			action := ui.editorContextMenuActionAt(gtx, pos)
			if action == "" {
				ui.closeEditorContextMenu()
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			if !pe.Buttons.Contain(pointer.ButtonPrimary) {
				continue
			}
			ed := ui.editorMenuTarget
			if ed == nil {
				ui.closeEditorContextMenu()
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			switch action {
			case "copy":
				ui.copyEditorText(gtx, ed)
			case "paste":
				if ui.editorMenuCanPaste {
					ui.pasteEditorText(gtx, ed, true)
				}
			}
			ui.closeEditorContextMenu()
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (ui *UI) handleProtocolDropdownOutsideClick(gtx layout.Context) {
	if ui == nil || ui.Tabs.Value != "tab2" || ui.tab2State == nil || !ui.tab2State.protoDropOpen {
		return
	}
	st := ui.tab2State
	closed := false
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &ui.protoDropGlobalPointerTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok || pe.Kind != pointer.Press {
			continue
		}
		if !pe.Buttons.Contain(pointer.ButtonPrimary) {
			continue
		}

		// Keep dropdown open while user clicks dropdown controls;
		// local handlers in tab2 own those interactions.
		if st.click("proto:btn").Hovered() {
			continue
		}
		overOption := false
		for _, opt := range protocolOptions(st) {
			if st.click("proto:" + opt.Name).Hovered() {
				overOption = true
				break
			}
		}
		if overOption {
			continue
		}

		st.protoDropOpen = false
		st.protoDropOpenedAt = time.Time{}
		closed = true
	}
	if closed {
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) handleGlobalEscapeToFileManager(gtx layout.Context) {
	if ui == nil || ui.helpModal != nil || ui.settingsModal != nil || ui.sshModal != nil {
		return
	}
	if ui.terminalFocused(gtx) {
		return
	}
	if ui.Tabs.Value == "tab0" && !ui.customCommandMenuOpen && !ui.functionBarToolsOpen {
		return
	}
	switched := false
	closedProtoDropdown := false
	closedCustom := false
	closedTools := false
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press || ke.Name != key.NameEscape {
			continue
		}
		if ui.customCommandMenuOpen {
			ui.closeCustomCommandMenu()
			closedCustom = true
			continue
		}
		if ui.functionBarToolsOpen {
			ui.closeFunctionBarToolsMenu()
			closedTools = true
			continue
		}
		if ui.Tabs.Value == "tab0" {
			continue
		}
		if ui.Tabs.Value == "tab2" && ui.tab2State != nil && ui.tab2State.protoDropOpen {
			ui.tab2State.protoDropOpen = false
			ui.tab2State.protoDropOpenedAt = time.Time{}
			closedProtoDropdown = true
			continue
		}
		ui.setActiveTab("tab0", gtx.Now)
		ui.closeFileViewer()
		ui.resetKeys()
		switched = true
	}
	if switched || closedProtoDropdown || closedCustom || closedTools {
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) handleGlobalFunctionKeys(gtx layout.Context) {
	if ui != nil && (ui.customCommandEditor != nil || ui.multiRename != nil) {
		return
	}
	if ui != nil {
		ui.handleTabShortcuts(gtx)
		ui.handleTerminalFocusToggleKey(gtx)
	}
	if ui != nil && ui.terminalFocused(gtx) {
		ui.handleTerminalToggleKey(gtx)
		ui.handleGlobalSettingsShortcut(gtx)
		return
	}
	anyMods := ^key.Modifiers(0)
	filters := []event.Filter{
		key.Filter{Name: key.NameF1},
		key.Filter{Name: key.NameF2},
		key.Filter{Name: key.NameF3, Optional: anyMods},
		key.Filter{Name: key.NameF4, Optional: anyMods},
		key.Filter{Name: key.NameF9, Optional: anyMods},
		key.Filter{Name: key.NameF10, Optional: anyMods},
		key.Filter{Name: key.NameF11, Optional: anyMods},
		key.Filter{Name: key.NameF12, Optional: anyMods},
		key.Filter{Name: "1", Required: key.ModAlt, Optional: anyMods},
		key.Filter{Name: "2", Required: key.ModAlt, Optional: anyMods},
		key.Filter{Name: "F", Required: key.ModCtrl, Optional: anyMods},
		key.Filter{Name: "f", Required: key.ModCtrl, Optional: anyMods},
		key.Filter{Name: "F", Required: key.ModShortcut, Optional: anyMods},
		key.Filter{Name: "f", Required: key.ModShortcut, Optional: anyMods},
		key.Filter{Name: "S", Required: key.ModCtrl, Optional: anyMods},
		key.Filter{Name: "s", Required: key.ModCtrl, Optional: anyMods},
		key.Filter{Name: "S", Required: key.ModShortcut, Optional: anyMods},
		key.Filter{Name: "s", Required: key.ModShortcut, Optional: anyMods},
		key.Filter{Name: "M", Required: key.ModCtrl, Optional: anyMods},
		key.Filter{Name: "m", Required: key.ModCtrl, Optional: anyMods},
		key.Filter{Name: "M", Required: key.ModShortcut, Optional: anyMods},
		key.Filter{Name: "m", Required: key.ModShortcut, Optional: anyMods},
	}
	filters = append(filters, customCommandShortcutKeyFilters(anyMods)...)
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		ke, ok := ev.(key.Event)
		if !ok {
			continue
		}
		switch ke.Name {
		case key.NameF1:
			if ke.State != key.Press || ke.Modifiers != 0 {
				continue
			}
			if ui.helpModal != nil {
				ui.closeHelpModal()
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			if ui.performFunctionBarAction(functionBarActionHelp, gtx.Now) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameF2:
			if ke.State != key.Press || ke.Modifiers != 0 {
				continue
			}
			if ui.performFunctionBarAction(functionBarActionCustom, gtx.Now) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameF3:
			if ke.State == key.Release {
				ui.clearFileViewHotkeyHold()
				continue
			}
			if ke.State != key.Press {
				continue
			}
			// Swallow modified/unknown-flag F3 to prevent system beep, but don't trigger actions.
			if ke.Modifiers != 0 {
				continue
			}
			if ui.performFunctionBarAction(functionBarActionView, gtx.Now) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameF4:
			if ke.State != key.Press {
				continue
			}
			if ke.Modifiers != 0 {
				continue
			}
			if ui.performFunctionBarAction(functionBarActionOpen, gtx.Now) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameF9:
			if ke.State != key.Press || ke.Modifiers != 0 {
				continue
			}
			if ui.performFunctionBarAction(functionBarActionTools, gtx.Now) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameF10:
			if ke.State != key.Press || ke.Modifiers != 0 {
				continue
			}
			if ui.performFunctionBarAction(functionBarActionExit, gtx.Now) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameF11:
			if ke.State != key.Press || ke.Modifiers != 0 {
				continue
			}
			if ui.toggleFunctionBarVisibility(gtx.Now) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case key.NameF12:
			if ke.State != key.Press || ke.Modifiers != 0 {
				continue
			}
			if ui.toggleTerminal() {
				gtx.Execute(op.InvalidateCmd{})
			}
		case "1":
			if ke.State != key.Press {
				continue
			}
			if customCommandShortcutModifier(ke.Modifiers) {
				if ui.activateCustomCommandGlobalShortcut(ke, gtx.Now) {
					gtx.Execute(op.InvalidateCmd{})
				}
				continue
			}
			// Swallow Alt+1 globally to avoid platform dinging even when the
			// drive picker can't be opened in the current context.
			if ke.Modifiers != key.ModAlt {
				continue
			}
			if ui == nil || ui.Tabs.Value != "tab0" || ui.helpModal != nil || ui.settingsModal != nil || ui.sshModal != nil || ui.hasBlockingFileDialog() || ui.pathEditActive() || ui.fileViewer != nil {
				continue
			}
			if ui.openPaneDriveMenu(0) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case "2":
			if ke.State != key.Press {
				continue
			}
			if customCommandShortcutModifier(ke.Modifiers) {
				if ui.activateCustomCommandGlobalShortcut(ke, gtx.Now) {
					gtx.Execute(op.InvalidateCmd{})
				}
				continue
			}
			// Swallow Alt+2 globally to avoid platform dinging even when the
			// drive picker can't be opened in the current context.
			if ke.Modifiers != key.ModAlt {
				continue
			}
			if ui == nil || ui.Tabs.Value != "tab0" || ui.helpModal != nil || ui.settingsModal != nil || ui.sshModal != nil || ui.hasBlockingFileDialog() || ui.pathEditActive() || ui.fileViewer != nil {
				continue
			}
			if ui.openPaneDriveMenu(1) {
				gtx.Execute(op.InvalidateCmd{})
			}
		case "F", "f":
			if ke.State != key.Press {
				continue
			}
			// Swallow ctrl/cmd+f with extra modifiers to prevent system beep,
			// but only trigger action for plain Ctrl+F or Cmd+F.
			if ke.Modifiers != key.ModCtrl && ke.Modifiers != key.ModShortcut {
				continue
			}
			if ui == nil || ui.Tabs.Value != "tab0" {
				continue
			}
			if ui.fileViewer != nil {
				ui.openFileViewerFind(gtx.Now)
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			if ui.settingsModal != nil || ui.sshModal != nil {
				continue
			}
			if ui.hasBlockingFileDialog() {
				continue
			}
			if ui.pathEditActive() {
				continue
			}
			ui.openSSHModal()
			gtx.Execute(op.InvalidateCmd{})
		case "S", "s":
			if ke.State != key.Press {
				continue
			}
			if ke.Modifiers != key.ModCtrl && ke.Modifiers != key.ModShortcut {
				continue
			}
			if ui == nil || ui.helpModal != nil || ui.settingsModal != nil || ui.sshModal != nil || ui.hasBlockingFileDialog() {
				continue
			}
			if ui.pathEditActive() {
				continue
			}
			ui.activateFunctionBarTool("settings", gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		case "M", "m":
			if ke.State != key.Press || (ke.Modifiers != key.ModCtrl && ke.Modifiers != key.ModShortcut) {
				continue
			}
			if ui == nil || ui.Tabs.Value != "tab0" || ui.helpModal != nil || ui.settingsModal != nil || ui.sshModal != nil || ui.hasBlockingFileDialog() || ui.pathEditActive() || ui.fileViewer != nil {
				continue
			}
			ui.activateFunctionBarTool("multi-rename", gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
		default:
			if ui.activateCustomCommandGlobalShortcut(ke, gtx.Now) {
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
}

func (ui *UI) handleGlobalSettingsShortcut(gtx layout.Context) bool {
	if ui == nil {
		return false
	}
	anyMods := ^key.Modifiers(0)
	handled := false
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: "S", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "s", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "S", Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "s", Required: key.ModShortcut, Optional: anyMods},
		)
		if !ok {
			return handled
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		handled = true
		if ke.Modifiers != key.ModCtrl && ke.Modifiers != key.ModShortcut {
			continue
		}
		if ui.helpModal != nil || ui.settingsModal != nil || ui.sshModal != nil || ui.hasBlockingFileDialog() {
			continue
		}
		if ui.pathEditActive() {
			continue
		}
		ui.activateFunctionBarTool("settings", gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) layoutTabPlaceholder(th *material.Theme, gtx layout.Context, name string) layout.Dimensions {
	in := layout.UniformInset(unit.Dp(16))
	return in.Layout(gtx, material.H6(th, name).Layout)
}

func (ui *UI) consumeUnusedFunctionKeys(gtx layout.Context) {
	anyMods := ^key.Modifiers(0)
	for {
		_, ok := gtx.Event(
			key.Filter{Name: key.NameF1, Optional: anyMods},
			key.Filter{Name: key.NameF2, Optional: anyMods},
			key.Filter{Name: key.NameF3, Optional: anyMods},
			key.Filter{Name: key.NameF4, Optional: anyMods},
			key.Filter{Name: key.NameF5, Optional: anyMods},
			key.Filter{Name: key.NameF6, Optional: anyMods},
			key.Filter{Name: key.NameF7, Optional: anyMods},
			key.Filter{Name: key.NameF8, Optional: anyMods},
			key.Filter{Name: key.NameF9, Optional: anyMods},
			key.Filter{Name: key.NameF10, Optional: anyMods},
			key.Filter{Name: key.NameF11, Optional: anyMods},
			key.Filter{Name: key.NameF12, Optional: anyMods},
			key.Filter{Name: "1", Required: key.ModAlt, Optional: anyMods},
			key.Filter{Name: "2", Required: key.ModAlt, Optional: anyMods},
			key.Filter{Name: "F", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "f", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "F", Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "f", Required: key.ModShortcut, Optional: anyMods},
			// Also drain any unmatched Ctrl/Cmd shortcuts to prevent macOS beep.
			key.Filter{Required: key.ModAlt, Optional: anyMods},
			key.Filter{Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Required: key.ModShortcut, Optional: anyMods},
		)
		if !ok {
			break
		}
	}

	// Fallback: drain any remaining unhandled key events to avoid system beep
	// on macOS when a key press is not claimed by other handlers.
	for {
		_, ok := gtx.Event(key.Filter{Optional: anyMods})
		if !ok {
			return
		}
	}
}
