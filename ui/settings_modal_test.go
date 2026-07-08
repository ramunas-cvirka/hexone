// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	resources "hexone"
	"hexone/filesys"
	"image"
	"image/color"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"hexone/fm"
)

func TestSettingsTabIndexOrder(t *testing.T) {
	cases := []struct {
		key  string
		want int
	}{
		{key: "general", want: 0},
		{key: "fonts", want: 1},
		{key: "terminal", want: 2},
		{key: "viewer", want: 3},
		{key: "associations", want: 4},
		{key: "colors", want: 5},
		{key: "config", want: 6},
	}
	for _, tc := range cases {
		if got := settingsTabIndex(tc.key); got != tc.want {
			t.Fatalf("settingsTabIndex(%q)=%d want %d", tc.key, got, tc.want)
		}
	}
}

func TestSettingsModalFooterClaimsFullWidthForRightAlignedActions(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(500, 60),
		},
	}

	if got, want := ui.layoutSettingsModalFooter(th, gtx, &settingsModalState{}).Size.X, 500; got != want {
		t.Fatalf("settings footer width=%d want %d", got, want)
	}
}

func TestSettingsModalHeightTracksWindowWithoutFillingIt(t *testing.T) {
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	for _, tc := range []struct {
		available int
		want      int
	}{
		{available: 1000, want: 800},
		{available: 500, want: 460},
		{available: 400, want: 400},
	} {
		if got := responsiveModalHeight(gtx, tc.available); got != tc.want {
			t.Fatalf("responsive settings height for %dpx=%d want %d", tc.available, got, tc.want)
		}
	}
}

func TestSettingsModalSaveActionIsAtRightEdge(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	st := &settingsModalState{}
	router := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(500, 60),
		},
	}
	frame := func() bool {
		clicked := st.saveClick.Clicked(gtx)
		gtx.Ops.Reset()
		ui.layoutSettingsModalFooter(th, gtx, st)
		router.Frame(gtx.Ops)
		return clicked
	}

	frame()
	pos := f32.Pt(470, 12)
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pos})
	frame()
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pos})
	if !frame() {
		t.Fatal("Save action should occupy the right edge of the settings footer")
	}
}

func TestSettingsViewerIntroductionFitsWithoutScrolling(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal should open")
	}
	st.activeTab = "viewer"
	th := material.NewTheme()
	router := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(744, 444), // 760x460 card after its 8 dp inset.
		},
	}

	header := ui.layoutSettingsModalHeader(th, gtx, st)
	gtx.Ops.Reset()
	footer := ui.layoutSettingsModalFooter(th, gtx, st)
	bodyH := gtx.Constraints.Max.Y - header.Size.Y - footer.Size.Y - 14
	if bodyH < 1 {
		t.Fatalf("invalid settings body height %d", bodyH)
	}
	gtx.Ops.Reset()
	gtx.Constraints = layout.Exact(image.Pt(584, bodyH))
	ui.layoutSettingsViewerTab(th, gtx, st)
	if got := st.viewerTabList.Position.Count; got < 5 {
		t.Fatalf("viewer introduction and first priority section are cramped: visible sections=%d want at least 5", got)
	}
}

func TestSettingsShiftTabWraps(t *testing.T) {
	cases := []struct {
		key  string
		step int
		want string
	}{
		{key: "general", step: -1, want: "config"},
		{key: "config", step: 1, want: "general"},
		{key: "fonts", step: 1, want: "terminal"},
		{key: "terminal", step: 1, want: "viewer"},
		{key: "viewer", step: 1, want: "associations"},
		{key: "colors", step: -1, want: "associations"},
	}
	for _, tc := range cases {
		if got := settingsShiftTab(tc.key, tc.step); got != tc.want {
			t.Fatalf("settingsShiftTab(%q, %d)=%q want %q", tc.key, tc.step, got, tc.want)
		}
	}
}

func TestSettingsChoiceAnimAllowsEmptyValueSelection(t *testing.T) {
	now := time.Date(2026, time.March, 20, 12, 0, 0, 0, time.UTC)
	current := "any"
	anim := settingsChoiceAnim{}

	anim.setValue(&current, "", now)

	if current != "" {
		t.Fatalf("current=%q want empty", current)
	}
	if !anim.hasPrev || anim.prev != "any" {
		t.Fatalf("anim previous state=%q hasPrev=%v want any/true", anim.prev, anim.hasPrev)
	}
	fill, animating := anim.fill(now.Add(toolbarAnimDur), current, "")
	if fill != 1 || animating {
		t.Fatalf("fill=%v animating=%v want 1,false", fill, animating)
	}
}

func TestSegmentedAnimStateAllowsEmptyPulseKey(t *testing.T) {
	now := time.Date(2026, time.March, 20, 12, 0, 0, 0, time.UTC)
	anim := segmentedAnimState{}

	anim.setPulse("", now)

	fill, animating := anim.pulseFill(now, "")
	if fill != 1 || !animating {
		t.Fatalf("fill=%v animating=%v want 1,true", fill, animating)
	}
}

func TestSettingsStepActiveTabSetsPulse(t *testing.T) {
	now := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	st := &settingsModalState{activeTab: "general"}

	if !st.stepActiveTab(1, now) {
		t.Fatal("stepActiveTab should report a tab change")
	}
	if st.activeTab != "fonts" {
		t.Fatalf("activeTab=%q want %q", st.activeTab, "fonts")
	}
	if st.navPrevTab != "general" {
		t.Fatalf("navPrevTab=%q want %q", st.navPrevTab, "general")
	}
	if st.navPulseKey != "fonts" {
		t.Fatalf("navPulseKey=%q want %q", st.navPulseKey, "fonts")
	}
	if st.navPulseAt != now {
		t.Fatalf("navPulseAt=%v want %v", st.navPulseAt, now)
	}
}

func TestSettingsKeyboardFocusOrderTracksVisibleControls(t *testing.T) {
	st := &settingsModalState{
		activeTab:        "colors",
		colorScope:       "filenames",
		filenameRuleMode: "permissions",
	}

	got := st.focusOrder()
	want := []settingsKeyboardFocus{
		settingsKeyboardFocusNav,
		settingsKeyboardFocusColorsScope,
		settingsKeyboardFocusFilenameRuleMode,
		settingsKeyboardFocusFilenamePermMask,
		settingsKeyboardFocusFilenamePermPicker,
		settingsKeyboardFocusFilenamePermMatch,
		settingsKeyboardFocusFilenamePermTarget,
		settingsKeyboardFocusFilenamePermTextPicker,
		settingsKeyboardFocusFilenamePermText,
		settingsKeyboardFocusFilenamePermIconPicker,
		settingsKeyboardFocusFilenamePermApply,
		settingsKeyboardFocusFilenamePermRemove,
		settingsKeyboardFocusFooter,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("focusOrder()=%v want %v", got, want)
	}
}

func TestSettingsKeyboardFocusOrderIncludesEditorsAndCheckboxes(t *testing.T) {
	st := &settingsModalState{activeTab: "general"}

	got := st.focusOrder()
	want := []settingsKeyboardFocus{
		settingsKeyboardFocusNav,
		settingsKeyboardFocusGeneralDimInactive,
		settingsKeyboardFocusGeneralFavoritesNewTab,
		settingsKeyboardFocusGeneralCompletionSound,
		settingsKeyboardFocusFilePaneFileWeight,
		settingsKeyboardFocusFilePaneDirWeight,
		settingsKeyboardFocusFilePanePermissionsWeight,
		settingsKeyboardFocusFilePaneSizeWeight,
		settingsKeyboardFocusFilePaneDateWeight,
		settingsKeyboardFocusFooter,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("focusOrder()=%v want %v", got, want)
	}
}

func TestSettingsKeyboardFocusOrderIncludesTerminalControls(t *testing.T) {
	st := &settingsModalState{activeTab: "terminal"}

	got := st.focusOrder()
	want := []settingsKeyboardFocus{
		settingsKeyboardFocusNav,
		settingsKeyboardFocusTerminalShell,
		settingsKeyboardFocusTerminalAcceleratedKeys,
		settingsKeyboardFocusFooter,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("focusOrder()=%v want %v", got, want)
	}
}

func TestSettingsKeyboardFocusOrderIncludesFontControls(t *testing.T) {
	st := &settingsModalState{activeTab: "fonts"}

	got := st.focusOrder()
	want := []settingsKeyboardFocus{
		settingsKeyboardFocusNav,
		settingsKeyboardFocusFontsInterfaceFont,
		settingsKeyboardFocusFontsInterfaceFontSize,
		settingsKeyboardFocusGeneralPaneFont,
		settingsKeyboardFocusGeneralPaneFontSize,
		settingsKeyboardFocusFontsTabsFont,
		settingsKeyboardFocusFontsTabsFontSize,
		settingsKeyboardFocusGeneralViewFont,
		settingsKeyboardFocusGeneralViewFontSize,
		settingsKeyboardFocusFontsTerminalFont,
		settingsKeyboardFocusFontsTerminalFontSize,
		settingsKeyboardFocusFooter,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("focusOrder()=%v want %v", got, want)
	}
}

func TestSettingsModalLoadsTabsFontControls(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Tabs.Typeface = resources.BundledFontFamilyIosevkaNerdFontMono
	cfg.Tabs.FontSizeSp = 12
	ui := NewUI(cfg)
	ui.openSettingsModal()

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	if got, want := st.tabsFontFamily, cfg.Tabs.Typeface; got != want {
		t.Fatalf("tabs font family=%q want %q", got, want)
	}
	if got, want := st.tabsFontSizeSp, cfg.Tabs.FontSizeSp; got != want {
		t.Fatalf("tabs font size=%v want %v", got, want)
	}
}

func TestSettingsModalLoadsAndSavesInterfaceFontControls(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Interface.Typeface = resources.BundledFontFamilyIosevkaNerdFontMono
	cfg.Interface.FontSizeSp = 15
	ui := NewUI(cfg)
	ui.configPath = filepath.Join(t.TempDir(), "hexone.yaml")
	ui.openSettingsModal()

	st := ui.settingsModal
	if got, want := st.interfaceFontFamily, cfg.Interface.Typeface; got != want {
		t.Fatalf("interface font family=%q want %q", got, want)
	}
	if got, want := st.interfaceFontSizeSp, cfg.Interface.FontSizeSp; got != want {
		t.Fatalf("interface font size=%v want %v", got, want)
	}

	st.interfaceFontFamily = resources.BundledFontFamilyHackNerdFontMono
	st.interfaceFontSizeSp = 16
	if err := ui.saveSettingsModal(time.Now()); err != nil {
		t.Fatalf("save settings modal: %v", err)
	}
	saved := fm.LoadConfig(ui.configPath)
	if got, want := saved.Interface.Typeface, st.interfaceFontFamily; got != want {
		t.Fatalf("saved interface font family=%q want %q", got, want)
	}
	if got, want := saved.Interface.FontSizeSp, st.interfaceFontSizeSp; got != want {
		t.Fatalf("saved interface font size=%v want %v", got, want)
	}
}

func TestSettingsPanePreviewHeightTracksPaneFont(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	gtx := testLabelLayoutContext(image.Pt(640, 480))
	base := ui.settingsPanePreviewHostHeight(gtx)

	cfg.General.FontSizeSp = 22
	large := ui.settingsPanePreviewHostHeight(gtx)
	if large <= base {
		t.Fatalf("large pane preview height=%d want > %d", large, base)
	}
}

func TestSettingsColorPreviewPathHeightMatchesPanePathBar(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.General.FontSizeSp = 22
	ui := NewUI(cfg)
	th := material.NewTheme()
	ui.syncThemeRuntime(th)

	pane := newFilePaneState(".", cfg)
	pane.applyListing(testPathListing("/opt/gpstrack/log"), "", "", 0)
	pathGtx := testPathLayoutContext()
	pathDims := ui.layoutFilePanePathArea(th, pathGtx, 0, pane, true)

	previewPathH := settingsColorPreviewPathContainerHeight(pathGtx)
	if previewPathH != pathDims.Size.Y {
		t.Fatalf("preview path height=%d want real path bar height %d", previewPathH, pathDims.Size.Y)
	}

	previewGtx := testLabelLayoutContext(image.Pt(640, 480))
	if got, oldScaled := ui.settingsColorPreviewCurrentDirHeight(previewGtx), previewGtx.Dp(scaleFilePaneDp(cfg, 22)); got >= oldScaled {
		t.Fatalf("current-dir preview row height=%d should stay below old scaled path height %d", got, oldScaled)
	}
}

func TestSettingsColorsTabScrollsVertically(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	if got := ui.settingsModal.colorsTabList.Axis; got != layout.Vertical {
		t.Fatalf("colors tab list axis=%v want vertical", got)
	}
}

func TestSettingsModalSavesTabsFontControls(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	ui.configPath = filepath.Join(t.TempDir(), "hexone.yaml")
	ui.openSettingsModal()

	st := ui.settingsModal
	st.tabsFontFamily = resources.BundledFontFamilyIosevkaNerdFontMono
	st.tabsFontSizeSp = 12.5
	if err := ui.saveSettingsModal(time.Now()); err != nil {
		t.Fatalf("save settings modal: %v", err)
	}

	saved := fm.LoadConfig(ui.configPath)
	if got, want := saved.Tabs.Typeface, st.tabsFontFamily; got != want {
		t.Fatalf("saved tabs font family=%q want %q", got, want)
	}
	if got, want := saved.Tabs.FontSizeSp, st.tabsFontSizeSp; got != want {
		t.Fatalf("saved tabs font size=%v want %v", got, want)
	}
}

func TestSettingsModalLoadsAndSavesTerminalAcceleratedKeys(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Terminal.AcceleratedKeys = false
	ui := NewUI(cfg)
	ui.configPath = filepath.Join(t.TempDir(), "hexone.yaml")
	ui.openSettingsModal()

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	if st.terminalAcceleratedKeysBool.Value {
		t.Fatal("terminal accelerated keys checkbox should load false")
	}

	st.terminalAcceleratedKeysBool.Value = true
	if err := ui.saveSettingsModal(time.Now()); err != nil {
		t.Fatalf("save settings modal: %v", err)
	}
	saved := fm.LoadConfig(ui.configPath)
	if !saved.Terminal.AcceleratedKeys {
		t.Fatal("saved terminal accelerated keys should be true")
	}
}

func TestSettingsModalShellChangeRecreatesOpenTerminal(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	ui.configPath = filepath.Join(t.TempDir(), "hexone.yaml")
	old := newTerminalSession(nil)
	old.setActive(true)
	ui.terminal = old
	ui.ensureTerminalTabs()
	ui.openSettingsModal()
	ui.settingsModal.viewShellEdit.SetText("cmd")

	if err := ui.saveSettingsModal(time.Now()); err != nil {
		t.Fatalf("save settings modal: %v", err)
	}
	if got, want := ui.fmCfg.Viewer.Shell, "cmd"; got != want {
		t.Fatalf("saved shell=%q want %q", got, want)
	}
	if ui.terminal == old || !old.closing {
		t.Fatal("open terminal was not recreated after changing shells")
	}
	if !ui.terminal.active() {
		t.Fatal("terminal drawer should remain open after changing shells")
	}
}

func TestSettingsKeyboardFocusOrderIncludesViewerControls(t *testing.T) {
	st := &settingsModalState{activeTab: "viewer"}

	got := st.focusOrder()
	want := []settingsKeyboardFocus{
		settingsKeyboardFocusNav,
		settingsKeyboardFocusViewerRemoteSearch,
		settingsKeyboardFocusViewerSmoothScrolling,
		settingsKeyboardFocusViewerHideFunctionBar,
		settingsKeyboardFocusViewerTargetKey,
		settingsKeyboardFocusViewerTargetBrowse,
		settingsKeyboardFocusViewerTargetApply,
		settingsKeyboardFocusViewerTargetCommand,
		settingsKeyboardFocusViewerRulePattern,
		settingsKeyboardFocusViewerRuleBrowse,
		settingsKeyboardFocusViewerRuleApply,
		settingsKeyboardFocusViewerRuleCommand,
		settingsKeyboardFocusViewerCommand,
		settingsKeyboardFocusFooter,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("focusOrder()=%v want %v", got, want)
	}
}

func TestSettingsKeyboardFocusOrderIncludesColorTarget(t *testing.T) {
	st := &settingsModalState{activeTab: "colors", colorScope: "panes", colorCategory: "normal"}

	got := st.focusOrder()
	want := []settingsKeyboardFocus{
		settingsKeyboardFocusNav,
		settingsKeyboardFocusColorsScope,
		settingsKeyboardFocusColorsCategory,
		settingsKeyboardFocusColorsBgPicker,
		settingsKeyboardFocusColorsValue,
		settingsKeyboardFocusColorsTextPicker,
		settingsKeyboardFocusColorsTextValue,
		settingsKeyboardFocusFooter,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("focusOrder()=%v want %v", got, want)
	}
}

func TestSettingsKeyboardFocusOrderIncludesTransparentColorToggle(t *testing.T) {
	st := &settingsModalState{activeTab: "colors", colorScope: "panes", colorCategory: "hover"}

	got := st.focusOrder()
	want := []settingsKeyboardFocus{
		settingsKeyboardFocusNav,
		settingsKeyboardFocusColorsScope,
		settingsKeyboardFocusColorsCategory,
		settingsKeyboardFocusColorsBgPicker,
		settingsKeyboardFocusColorsValue,
		settingsKeyboardFocusColorsTextPicker,
		settingsKeyboardFocusColorsTextValue,
		settingsKeyboardFocusColorsTextTransparent,
		settingsKeyboardFocusFooter,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("focusOrder()=%v want %v", got, want)
	}
}

func TestSettingsKeyboardFocusOrderSkipsConfigPath(t *testing.T) {
	st := &settingsModalState{activeTab: "config"}

	got := st.focusOrder()
	want := []settingsKeyboardFocus{
		settingsKeyboardFocusNav,
		settingsKeyboardFocusConfigEditor,
		settingsKeyboardFocusFooter,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("focusOrder()=%v want %v", got, want)
	}
}

func TestSettingsModalKeyboardUsesUpDownOnlyForNavFocus(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "terminal"
	st.focus = settingsKeyboardFocusTerminalShell
	st.viewShellEdit.SetText("auto")
	st.footerFocus = settingsFooterActionSave
	st.keyFocus.wantFocus = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)

	router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	frame(now.Add(time.Millisecond))
	if st.activeTab != "terminal" {
		t.Fatalf("activeTab after DownArrow = %q, want terminal", st.activeTab)
	}

	router.Queue(key.Event{Name: key.NameRightArrow, State: key.Press})
	frame(now.Add(2 * time.Millisecond))
	if got := st.viewShellEdit.Text(); got != "auto" {
		t.Fatalf("terminal shell after RightArrow = %q, want auto", got)
	}
}

func TestSettingsModalKeyboardSpaceTogglesFocusedCheckbox(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "general"
	st.focus = settingsKeyboardFocusGeneralDimInactive
	st.keyFocus.wantFocus = true
	st.generalDimInactiveBool.Value = false

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameSpace, State: key.Press})
	frame(now.Add(time.Millisecond))

	if !st.generalDimInactiveBool.Value {
		t.Fatal("Space should toggle the focused general checkbox")
	}
}

func TestSettingsModalKeyboardSpaceTogglesFavoritesNewTab(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "general"
	st.focus = settingsKeyboardFocusGeneralFavoritesNewTab
	st.keyFocus.wantFocus = true
	st.generalFavoritesNewTabBool.Value = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameSpace, State: key.Press})
	frame(now.Add(time.Millisecond))

	if st.generalFavoritesNewTabBool.Value {
		t.Fatal("Space should toggle the focused favorites new-tab checkbox")
	}
}

func TestSettingsModalKeyboardSpaceTogglesTerminalAcceleratedKeys(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "terminal"
	st.focus = settingsKeyboardFocusTerminalAcceleratedKeys
	st.keyFocus.wantFocus = true
	st.terminalAcceleratedKeysBool.Value = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameSpace, State: key.Press})
	frame(now.Add(time.Millisecond))

	if st.terminalAcceleratedKeysBool.Value {
		t.Fatal("Space should toggle the focused terminal accelerated-keys checkbox")
	}
}

func TestSettingsModalLoadsAndSavesPaneWeights(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.General.FileWeight = "light"
	cfg.General.DirWeight = fm.FontWeightRegular
	cfg.General.PermissionsWeight = fm.FontWeightBold
	cfg.General.SizeWeight = "medium"
	cfg.General.DateWeight = fm.FontWeightRegular
	ui := NewUI(cfg)
	ui.configPath = filepath.Join(t.TempDir(), "hexone.yaml")
	ui.openSettingsModal()

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	if got, want := st.paneFileWeight, fm.FontWeightRegular; got != want {
		t.Fatalf("pane file weight=%q want normalized %q", got, want)
	}
	if got, want := st.paneDirWeight, fm.FontWeightRegular; got != want {
		t.Fatalf("pane dir weight=%q want %q", got, want)
	}
	if got, want := st.panePermissionsWeight, fm.FontWeightBold; got != want {
		t.Fatalf("pane permissions weight=%q want %q", got, want)
	}

	st.paneFileWeight = fm.FontWeightRegular
	st.paneDirWeight = fm.FontWeightBold
	st.panePermissionsWeight = fm.FontWeightRegular
	st.paneSizeWeight = fm.FontWeightBold
	st.paneDateWeight = fm.FontWeightRegular
	if err := ui.saveSettingsModal(time.Now()); err != nil {
		t.Fatalf("save settings modal: %v", err)
	}

	saved := fm.LoadConfig(ui.configPath)
	if got, want := saved.General.FileWeight, st.paneFileWeight; got != want {
		t.Fatalf("saved file weight=%q want %q", got, want)
	}
	if got, want := saved.General.DirWeight, st.paneDirWeight; got != want {
		t.Fatalf("saved dir weight=%q want %q", got, want)
	}
	if got, want := saved.General.PermissionsWeight, st.panePermissionsWeight; got != want {
		t.Fatalf("saved permissions weight=%q want %q", got, want)
	}
	if got, want := saved.General.SizeWeight, st.paneSizeWeight; got != want {
		t.Fatalf("saved size weight=%q want %q", got, want)
	}
	if got, want := saved.General.DateWeight, st.paneDateWeight; got != want {
		t.Fatalf("saved date weight=%q want %q", got, want)
	}
}

func TestSettingsModalKeyboardSpaceTogglesViewerSmoothScrolling(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "viewer"
	st.focus = settingsKeyboardFocusViewerSmoothScrolling
	st.keyFocus.wantFocus = true
	st.viewSmoothScrollingBool.Value = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameSpace, State: key.Press})
	frame(now.Add(time.Millisecond))

	if st.viewSmoothScrollingBool.Value {
		t.Fatal("Space should toggle the focused viewer smooth scrolling checkbox")
	}
}

func TestViewerRemoteSearchCommandNoticeTextUsesMultipleLines(t *testing.T) {
	got := viewerRemoteSearchCommandNoticeText()
	if !strings.Contains(got, "\n") {
		t.Fatal("remote search notice should be split across multiple lines")
	}
	if !strings.Contains(got, `{range_start}`) {
		t.Fatal("remote search notice should mention the relative offset placeholder")
	}
	if !strings.Contains(got, `{result_select}`) {
		t.Fatal("remote search notice should include the result selector placeholder")
	}
}

func TestSettingsModalKeyboardTabIncludesFontSizeStepper(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "fonts"
	st.focus = settingsKeyboardFocusNav
	st.keyFocus.wantFocus = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	for i := 0; i < 2; i++ {
		router.Queue(key.Event{Name: key.NameTab, State: key.Press})
		frame(now.Add(time.Duration(i+1) * time.Millisecond))
	}

	if st.focus != settingsKeyboardFocusFontsInterfaceFontSize {
		t.Fatalf("focus after tabbing = %v, want interface font size stepper", st.focus)
	}
	if st.focusPending != settingsKeyboardFocusNone {
		t.Fatalf("focusPending = %v, want none for keyboard-owned stepper focus", st.focusPending)
	}
}

func TestSettingsModalKeyboardUpDownStepsFontSize(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "fonts"
	st.focus = settingsKeyboardFocusGeneralPaneFontSize
	st.paneFontSizeSp = 14
	st.keyFocus.wantFocus = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameUpArrow, State: key.Press})
	frame(now.Add(time.Millisecond))
	if got, want := st.paneFontSizeSp, float32(15); got != want {
		t.Fatalf("pane font size after UpArrow = %v, want %v", got, want)
	}

	router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	frame(now.Add(2 * time.Millisecond))
	if got, want := st.paneFontSizeSp, float32(14); got != want {
		t.Fatalf("pane font size after DownArrow = %v, want %v", got, want)
	}

	st.paneFontSizeSp = settingsFontSizeMin
	router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	frame(now.Add(3 * time.Millisecond))
	if got, want := st.paneFontSizeSp, settingsFontSizeMin; got != want {
		t.Fatalf("pane font size should clamp at min = %v, want %v", got, want)
	}
}

func TestSettingsModalKeyboardEnterUsesDefaultSaveAction(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.configPath = filepath.Join(t.TempDir(), "hexone.yaml")
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "general"
	st.focus = settingsKeyboardFocusNav
	st.footerFocus = settingsFooterActionSave
	st.keyFocus.wantFocus = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(time.Millisecond))

	if ui.settingsModal != nil {
		t.Fatal("Enter from non-editor focus should trigger the default Save action")
	}
}

func TestSettingsModalKeyboardEnterActivatesFocusedViewerBrowse(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "viewer"
	st.focus = settingsKeyboardFocusViewerTargetBrowse
	st.keyFocus.wantFocus = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(time.Millisecond))

	if !st.viewTargetPickOpen {
		t.Fatal("Enter on the focused Browse button should open the target picker")
	}
}

func TestSettingsModalKeyboardTabWalksViewerTargetPopupRowsAndRemove(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "viewer"
	st.focus = settingsKeyboardFocusViewerTargetBrowse
	st.viewTargetEntries = []viewerCommandTargetEntry{{Key: "/tmp/demo.log", Command: "tail -f {path}"}}
	st.keyFocus.wantFocus = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(time.Millisecond))

	if !st.popupKeyboardMatches(settingsPopupKeyboardViewerTarget, 0, settingsPopupKeyboardActionRow) {
		t.Fatalf("popup focus after Enter = kind %v index %d action %v", st.popupFocusKind, st.popupFocusIndex, st.popupFocusAction)
	}

	router.Queue(key.Event{Name: key.NameTab, State: key.Press})
	frame(now.Add(2 * time.Millisecond))

	if !st.popupKeyboardMatches(settingsPopupKeyboardViewerTarget, 0, settingsPopupKeyboardActionRemove) {
		t.Fatalf("popup focus after first Tab = kind %v index %d action %v", st.popupFocusKind, st.popupFocusIndex, st.popupFocusAction)
	}
}

func TestSettingsModalKeyboardEnterActivatesViewerTargetPopupRemove(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "viewer"
	st.focus = settingsKeyboardFocusViewerTargetBrowse
	st.viewTargetEntries = []viewerCommandTargetEntry{{Key: "/tmp/demo.log", Command: "tail -f {path}"}}
	st.keyFocus.wantFocus = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(time.Millisecond))
	router.Queue(key.Event{Name: key.NameTab, State: key.Press})
	frame(now.Add(2 * time.Millisecond))
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(3 * time.Millisecond))

	if len(st.viewTargetEntries) != 0 {
		t.Fatalf("viewer target entry count after remove = %d, want 0", len(st.viewTargetEntries))
	}
	if st.targetInfoText != "Pending removal; Save to persist" {
		t.Fatalf("targetInfoText = %q, want pending removal", st.targetInfoText)
	}
}

func TestSettingsModalKeyboardTabExitsViewerTargetPopupToNextControl(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "viewer"
	st.focus = settingsKeyboardFocusViewerTargetBrowse
	st.viewTargetEntries = []viewerCommandTargetEntry{{Key: "/tmp/demo.log", Command: "tail -f {path}"}}
	st.keyFocus.wantFocus = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(time.Millisecond))
	router.Queue(key.Event{Name: key.NameTab, State: key.Press})
	frame(now.Add(2 * time.Millisecond))
	router.Queue(key.Event{Name: key.NameTab, State: key.Press})
	frame(now.Add(3 * time.Millisecond))

	if st.viewTargetPickOpen {
		t.Fatal("second Tab should close the target picker")
	}
	if st.focus != settingsKeyboardFocusViewerTargetApply {
		t.Fatalf("focus after exiting popup = %v, want viewer target apply", st.focus)
	}
}

func TestSettingsModalKeyboardReenterViewerTargetPopupKeepsLastRow(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "viewer"
	st.focus = settingsKeyboardFocusViewerTargetBrowse
	st.viewTargetEntries = []viewerCommandTargetEntry{
		{Key: "/tmp/a.log", Command: "a"},
		{Key: "/tmp/b.log", Command: "b"},
		{Key: "/tmp/c.log", Command: "c"},
	}
	st.keyFocus.wantFocus = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(time.Millisecond))
	router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	frame(now.Add(2 * time.Millisecond))
	router.Queue(key.Event{Name: key.NameTab, State: key.Press})
	frame(now.Add(3 * time.Millisecond))
	router.Queue(key.Event{Name: key.NameTab, State: key.Press})
	frame(now.Add(4 * time.Millisecond))

	if st.viewTargetPickOpen {
		t.Fatal("popup should be closed after tabbing out")
	}

	st.focus = settingsKeyboardFocusViewerTargetBrowse
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(5 * time.Millisecond))

	if !st.popupKeyboardMatches(settingsPopupKeyboardViewerTarget, 1, settingsPopupKeyboardActionRow) {
		t.Fatalf("popup focus after re-enter Enter = kind %v index %d action %v", st.popupFocusKind, st.popupFocusIndex, st.popupFocusAction)
	}
}

func TestSettingsModalKeyboardColorPickerTabMovesAndEnterAppliesColor(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "colors"
	st.colorScope = "panes"
	st.focus = settingsKeyboardFocusColorsBgPicker
	st.keyFocus.wantFocus = true
	originalHex := strings.TrimSpace(st.colorValueEdit.Text())

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}
	colorAt := func(groups []settingsColorSwatchGroup, index int) string {
		cursor := 0
		for _, group := range groups {
			for _, hex := range group.hexes {
				if cursor == index {
					return fm.NormalizeHexColor(hex, hex)
				}
				cursor++
			}
		}
		return ""
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(time.Millisecond))

	if !st.colorPickerOpen || st.colorPickerTarget != "background" {
		t.Fatal("Enter on the background picker should open the color picker")
	}
	groups := st.colorPickerSwatchGroups("background")
	_, startIndex, ok := st.popupKeyboardDefaultFocus(nil, nil, nil, nil, groups, nil)
	if !ok {
		t.Fatal("color picker should provide a default popup focus")
	}
	if !st.popupKeyboardMatches(settingsPopupKeyboardColor, startIndex, settingsPopupKeyboardActionRow) {
		t.Fatalf("popup focus after Enter = kind %v index %d action %v", st.popupFocusKind, st.popupFocusIndex, st.popupFocusAction)
	}

	nextIndex := settingsPopupGridStep(startIndex, 1, 0, settingsColorPopupRowLengths(groups))
	wantHex := colorAt(groups, nextIndex)
	if wantHex == "" {
		t.Fatal("expected a keyboard-reachable color swatch")
	}

	router.Queue(key.Event{Name: key.NameRightArrow, State: key.Press})
	frame(now.Add(2 * time.Millisecond))

	if st.popupFocusIndex != nextIndex {
		t.Fatalf("popup focus index after RightArrow = %d, want %d", st.popupFocusIndex, nextIndex)
	}

	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(3 * time.Millisecond))

	if !st.colorPickerOpen {
		t.Fatal("selecting a base color should keep the color picker open")
	}
	if st.colorPickerBase != wantHex {
		t.Fatalf("selected base=%q want %q", st.colorPickerBase, wantHex)
	}
	if got := strings.TrimSpace(st.colorValueEdit.Text()); got != originalHex {
		t.Fatalf("base selection changed background to %q, want unchanged %q", got, originalHex)
	}

	st.colorPickerShade.Value = 0.25
	wantShade := settingsColorShade(wantHex, st.colorPickerShade.Value)
	setIndex := settingsColorSwatchCount(groups)
	st.setPopupKeyboardFocus(settingsPopupKeyboardColor, setIndex, settingsPopupKeyboardActionRow)
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(4 * time.Millisecond))

	if st.colorPickerOpen {
		t.Fatal("activating Set should close the color picker")
	}
	if got := strings.TrimSpace(st.colorValueEdit.Text()); got != wantShade {
		t.Fatalf("background after shade selection=%q want %q", got, wantShade)
	}
}

func TestSettingsModalKeyboardColorPickerTabExitsToNextControl(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "colors"
	st.colorScope = "panes"
	st.focus = settingsKeyboardFocusColorsBgPicker
	st.keyFocus.wantFocus = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(time.Millisecond))
	router.Queue(key.Event{Name: key.NameTab, State: key.Press})
	frame(now.Add(2 * time.Millisecond))

	if st.colorPickerOpen {
		t.Fatal("first Tab should close the color picker")
	}
	if st.focus != settingsKeyboardFocusColorsValue {
		t.Fatalf("focus after exiting color picker = %v, want background color editor", st.focus)
	}
}

func TestSettingsModalKeyboardColorPickerTabExitGivesWidgetFocusToEditor(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "colors"
	st.colorScope = "panes"
	st.focus = settingsKeyboardFocusColorsBgPicker
	st.keyFocus.wantFocus = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(time.Millisecond))
	router.Queue(key.Event{Name: key.NameTab, State: key.Press})
	frame(now.Add(2 * time.Millisecond))
	frame(now.Add(3 * time.Millisecond))

	if !gtx.Focused(&st.colorValueEdit) {
		t.Fatal("background color editor should gain widget focus after exiting the color picker")
	}
}

func TestSettingsModalKeyboardFilenameIconPickerMovesAndAppliesSelection(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)

	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.activeTab = "colors"
	st.colorScope = "filenames"
	st.filenameRuleMode = "age"
	st.focus = settingsKeyboardFocusFilenameAgeIconPicker
	st.keyFocus.wantFocus = true

	frame := func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(time.Millisecond))

	if !st.filenameIconPickerOpen || st.filenameIconPickerTarget != "filename-age-icon" {
		t.Fatal("Enter on the age icon picker should open the icon picker")
	}
	_, startIndex, ok := st.popupKeyboardDefaultFocus(nil, nil, nil, nil, nil, filenameIconOptions)
	if !ok {
		t.Fatal("icon picker should provide a default popup focus")
	}
	if !st.popupKeyboardMatches(settingsPopupKeyboardFilenameIcon, startIndex, settingsPopupKeyboardActionRow) {
		t.Fatalf("popup focus after Enter = kind %v index %d action %v", st.popupFocusKind, st.popupFocusIndex, st.popupFocusAction)
	}

	nextIndex := settingsPopupGridStep(startIndex, 0, 1, settingsIconPopupRowLengths(filenameIconOptions))
	wantIcon := filenameIconOptions[nextIndex].key
	router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	frame(now.Add(2 * time.Millisecond))

	if st.popupFocusIndex != nextIndex {
		t.Fatalf("popup focus index after DownArrow = %d, want %d", st.popupFocusIndex, nextIndex)
	}

	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(3 * time.Millisecond))

	if st.filenameIconPickerOpen {
		t.Fatal("Enter on the focused icon swatch should close the icon picker")
	}
	if got := st.filenameAgeIcon; got != wantIcon {
		t.Fatalf("age filename icon after keyboard selection = %q, want %q", got, wantIcon)
	}
}

func TestSettingsPopupKeyboardScrollStartsAtLastFullyVisibleRow(t *testing.T) {
	st := &settingsModalState{
		viewTargetPickRemember: -1,
		viewTargetEntries: []viewerCommandTargetEntry{
			{Key: "/tmp/a.log", Command: "a"},
			{Key: "/tmp/b.log", Command: "b"},
			{Key: "/tmp/c.log", Command: "c"},
			{Key: "/tmp/d.log", Command: "d"},
			{Key: "/tmp/e.log", Command: "e"},
		},
		viewTargetPickOpen: true,
	}
	st.viewTargetPickList.Position.First = 0
	st.viewTargetPickList.Position.Count = 4

	st.setPopupKeyboardFocus(settingsPopupKeyboardViewerTarget, 3, settingsPopupKeyboardActionRow)

	if got := st.viewTargetPickList.Position.First; got != 1 {
		t.Fatalf("first visible row after focusing almost-hidden row = %d, want 1", got)
	}
}

func TestSettingsTabPositionSlidesToAssociations(t *testing.T) {
	now := time.Date(2026, time.March, 7, 10, 0, 0, 0, time.UTC)
	st := &settingsModalState{activeTab: "general"}

	st.setActiveTab("associations", now)
	if st.activeTab != "associations" {
		t.Fatalf("activeTab=%q want %q", st.activeTab, "associations")
	}
	if st.navPrevTab != "general" {
		t.Fatalf("navPrevTab=%q want %q", st.navPrevTab, "general")
	}

	start, anim := st.tabPosition(now)
	if !anim {
		t.Fatal("tabPosition should animate immediately after tab switch")
	}
	if start != 0 {
		t.Fatalf("start position=%v want 0", start)
	}

	mid, anim := st.tabPosition(now.Add(toolbarAnimDur / 2))
	if !anim {
		t.Fatal("tabPosition should still animate mid-transition")
	}
	if mid <= 0 || mid >= 4 {
		t.Fatalf("mid position=%v want between 0 and 4", mid)
	}

	end, anim := st.tabPosition(now.Add(toolbarAnimDur))
	if anim {
		t.Fatal("tabPosition should stop animating at the end of the transition")
	}
	if end != 4 {
		t.Fatalf("end position=%v want 4", end)
	}
	if st.navPrevTab != "" {
		t.Fatalf("navPrevTab should clear after transition, got %q", st.navPrevTab)
	}
}

func TestSettingsToggleViewerCommandRulePickerClosesTargetPicker(t *testing.T) {
	st := &settingsModalState{viewTargetPickOpen: true}

	st.toggleViewerCommandRulePicker()

	if st.viewTargetPickOpen {
		t.Fatal("target picker should close when opening rule picker")
	}
	if !st.viewRulePickOpen {
		t.Fatal("rule picker should open")
	}
}

func TestSettingsToggleViewerCommandTargetPickerClosesRulePicker(t *testing.T) {
	st := &settingsModalState{viewRulePickOpen: true}

	st.toggleViewerCommandTargetPicker()

	if st.viewRulePickOpen {
		t.Fatal("rule picker should close when opening target picker")
	}
	if !st.viewTargetPickOpen {
		t.Fatal("target picker should open")
	}
}

func TestViewerAssociationPickerProgramsCollapseByAppAndShowAllWhenNoMatch(t *testing.T) {
	st := &settingsModalState{
		viewAssocEntries: []fm.ViewerAssociation{
			{Extension: "pdf", AppPath: `C:\Apps\reader.exe`},
			{Extension: "txt", AppPath: `C:\Apps\reader.exe`},
			{Extension: "mp3", AppPath: `C:\Apps\music.exe`},
		},
	}
	st.viewAssocExtEdit.SetText("docx")

	programs, matches := st.viewerAssociationPickerPrograms()
	if matches != 0 {
		t.Fatalf("matches=%d want 0", matches)
	}
	if len(programs) != 2 {
		t.Fatalf("len(programs)=%d want 2", len(programs))
	}
	if programs[0].AppPath != `C:\Apps\music.exe` || programs[1].AppPath != `C:\Apps\reader.exe` {
		t.Fatalf("unexpected program order: got [%q, %q]", programs[0].AppPath, programs[1].AppPath)
	}
	if got := strings.Join(programs[1].Extensions, ","); got != "pdf,txt" {
		t.Fatalf("grouped extensions=%q want %q", got, "pdf,txt")
	}
}

func TestViewerAssociationPickerProgramsPrioritizeMatchingApps(t *testing.T) {
	st := &settingsModalState{
		viewAssocEntries: []fm.ViewerAssociation{
			{Extension: "mp3", AppPath: `C:\Apps\music.exe`},
			{Extension: "mp4", AppPath: `C:\Apps\video.exe`},
			{Extension: "m4v", AppPath: `C:\Apps\video.exe`},
			{Extension: "pdf", AppPath: `C:\Apps\pdf.exe`},
		},
	}
	st.viewAssocExtEdit.SetText("mp")

	programs, matches := st.viewerAssociationPickerPrograms()
	if matches != 2 {
		t.Fatalf("matches=%d want 2", matches)
	}
	if len(programs) != 3 {
		t.Fatalf("len(programs)=%d want 3", len(programs))
	}
	if programs[0].MatchRank == 0 || programs[1].MatchRank == 0 || programs[2].MatchRank != 0 {
		t.Fatalf("unexpected match ranks: got [%d %d %d]", programs[0].MatchRank, programs[1].MatchRank, programs[2].MatchRank)
	}
}

func TestSettingsAssociationPickerRowClickLoadsAppPathWhenExtensionBlank(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	st := &settingsModalState{
		viewAssocEntries: []fm.ViewerAssociation{
			{Extension: "pdf", AppPath: `C:\Apps\pdf.exe`},
			{Extension: "mp3", AppPath: `C:\Apps\music.exe`},
		},
	}
	st.openViewerAssociationPicker()
	st.viewAssocPickList.Axis = layout.Vertical

	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(420, 220),
		},
	}

	frame := func() layout.Dimensions {
		gtx.Ops.Reset()
		programs, matches := st.viewerAssociationPickerPrograms()
		dims := ui.layoutSettingsViewerAssocPicker(th, gtx, st, programs, matches)
		r.Frame(gtx.Ops)
		return dims
	}

	dims := frame()
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("picker has invalid size: %v", dims.Size)
	}

	st.viewerAssocRowClick(`C:\Apps\pdf.exe`).Click()
	frame()

	if got := st.viewAssocExtEdit.Text(); got != "" {
		t.Fatalf("blank extension should stay blank: got %q", got)
	}
	if got := st.viewAssocAppEdit.Text(); got != `C:\Apps\pdf.exe` {
		t.Fatalf("app path not loaded from picker row: got %q", got)
	}
	if st.viewAssocPickOpen {
		t.Fatal("picker should close after row selection")
	}
}

func TestSettingsAssociationPickerRowClickKeepsTypedExtensionAndCopiesApp(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	st := &settingsModalState{
		viewAssocEntries: []fm.ViewerAssociation{
			{Extension: "pdf", AppPath: `C:\Apps\pdf.exe`},
			{Extension: "mp3", AppPath: `C:\Apps\music.exe`},
		},
	}
	st.viewAssocExtEdit.SetText("mkv")
	st.viewAssocLookupExt = fm.NormalizeViewerAssociationExtension("mkv")
	st.openViewerAssociationPicker()
	st.viewAssocPickList.Axis = layout.Vertical

	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(420, 220),
		},
	}

	frame := func() layout.Dimensions {
		gtx.Ops.Reset()
		programs, matches := st.viewerAssociationPickerPrograms()
		dims := ui.layoutSettingsViewerAssocPicker(th, gtx, st, programs, matches)
		r.Frame(gtx.Ops)
		return dims
	}

	dims := frame()
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("picker has invalid size: %v", dims.Size)
	}

	st.viewerAssocRowClick(`C:\Apps\music.exe`).Click()
	frame()

	if got := st.viewAssocExtEdit.Text(); got != "mkv" {
		t.Fatalf("typed extension should be preserved: got %q want %q", got, "mkv")
	}
	if got := st.viewAssocAppEdit.Text(); got != `C:\Apps\music.exe` {
		t.Fatalf("typed extension should borrow selected app path: got %q", got)
	}
	if want := fm.NormalizeViewerAssociationExtension("mkv"); st.viewAssocLookupExt != want {
		t.Fatalf("lookup extension should remain typed target: got %q want %q", st.viewAssocLookupExt, want)
	}
	if _, ok := st.viewerAssociation(fm.NormalizeViewerAssociationExtension("mkv")); ok {
		t.Fatal("picker should not auto-associate typed extension before Add")
	}
	if got := st.viewerAssociationNoticeText(); got != "Click Add" {
		t.Fatalf("viewerAssociationNoticeText=%q want %q", got, "Click Add")
	}
	if st.viewAssocPickOpen {
		t.Fatal("picker should close after row selection")
	}
}

func TestSettingsAssociationPickerOpenClearsStaleRowClicks(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	st := &settingsModalState{
		viewAssocEntries: []fm.ViewerAssociation{
			{Extension: "pdf", AppPath: `C:\Apps\pdf.exe`},
			{Extension: "mp3", AppPath: `C:\Apps\music.exe`},
		},
	}
	st.viewAssocPickList.Axis = layout.Vertical

	// Simulate a stale queued click from a prior picker session.
	st.viewerAssocRowClick(`C:\Apps\music.exe`).Click()
	st.openViewerAssociationPicker()

	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(420, 220),
		},
	}

	frame := func() {
		gtx.Ops.Reset()
		programs, matches := st.viewerAssociationPickerPrograms()
		_ = ui.layoutSettingsViewerAssocPicker(th, gtx, st, programs, matches)
		r.Frame(gtx.Ops)
	}

	frame()
	st.viewerAssocRowClick(`C:\Apps\pdf.exe`).Click()
	frame()

	if got := st.viewAssocExtEdit.Text(); got != "" {
		t.Fatalf("blank extension should stay blank after grouped picker select: got %q", got)
	}
	if got := st.viewAssocAppEdit.Text(); got != `C:\Apps\pdf.exe` {
		t.Fatalf("stale row click leaked into new picker session: got app %q want %q", got, `C:\Apps\pdf.exe`)
	}
}

func TestRefreshViewerAssociationDraftInfoPromptsUpdateForExistingAssociation(t *testing.T) {
	st := &settingsModalState{
		viewAssocEntries: []fm.ViewerAssociation{
			{Extension: ".mp3", AppPath: `C:\Apps\music.exe`},
		},
		viewAssocSavedEntries: []fm.ViewerAssociation{
			{Extension: ".mp3", AppPath: `C:\Apps\music.exe`},
		},
	}
	st.viewAssocExtEdit.SetText("mp3")
	st.viewAssocLookupExt = fm.NormalizeViewerAssociationExtension("mp3")
	st.viewAssocAppEdit.SetText(`C:\Apps\new-music.exe`)

	st.refreshViewerAssociationDraftInfo(true)

	assoc, ok := st.viewerAssociation(".mp3")
	if !ok {
		t.Fatal("existing association should still exist")
	}
	if assoc.AppPath != `C:\Apps\music.exe` {
		t.Fatalf("existing association should not change before Update: got %q", assoc.AppPath)
	}
	if !strings.Contains(st.assocInfoText, "Click Update") {
		t.Fatalf("missing association update hint, got %q", st.assocInfoText)
	}
}

func TestViewerAssociationNoticeTextUsesCurrentEditorState(t *testing.T) {
	st := &settingsModalState{
		viewAssocEntries: []fm.ViewerAssociation{
			{Extension: ".mp3", AppPath: `C:\Apps\music.exe`},
		},
		viewAssocSavedEntries: []fm.ViewerAssociation{
			{Extension: ".mp3", AppPath: `C:\Apps\music.exe`},
		},
	}
	st.viewAssocExtEdit.SetText("mp3")
	st.viewAssocAppEdit.SetText(`C:\Apps\changed.exe`)

	got := st.viewerAssociationNoticeText()
	if !strings.Contains(got, "Click Update") {
		t.Fatalf("viewerAssociationNoticeText=%q, want change notice", got)
	}
}

func TestViewerAssociationNoticeTextPromptsAddForNewAssociation(t *testing.T) {
	st := &settingsModalState{}
	st.viewAssocExtEdit.SetText("mp3")
	st.viewAssocAppEdit.SetText(`C:\Apps\music.exe`)

	got := st.viewerAssociationNoticeText()
	if !strings.Contains(got, "Click Add") {
		t.Fatalf("viewerAssociationNoticeText=%q, want add prompt", got)
	}
}

func TestViewerCommandTargetPickerEntriesFiltersAndFallsBackToAll(t *testing.T) {
	localLog := normalizeViewerCommandTargetInput("local:/tmp/error.log")
	localCompose := normalizeViewerCommandTargetInput("local:/tmp/docker-compose.yml")
	st := &settingsModalState{
		viewTargetEntries: []viewerCommandTargetEntry{
			{Key: localLog, Command: `tail -f {path}`},
			{Key: localCompose, Command: `docker compose -f {path} config`},
			{Key: "ssh:root@example.com:22:/var/log/app.log", Command: `journalctl -f --file {path}`},
		},
	}

	st.viewTargetKeyEdit.SetText("docker")
	entries, matches := st.viewerCommandTargetPickerEntries()
	if matches != 1 {
		t.Fatalf("matches=%d want 1", matches)
	}
	if len(entries) != 1 || entries[0].Key != localCompose {
		t.Fatalf("unexpected filtered entries: %#v", entries)
	}

	st.viewTargetKeyEdit.SetText("nomatch")
	entries, matches = st.viewerCommandTargetPickerEntries()
	if matches != 0 {
		t.Fatalf("matches=%d want 0", matches)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries)=%d want 3", len(entries))
	}
}

func TestSettingsViewerCommandTargetPickerRowClickLoadsTargetAndCommand(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	localLog := normalizeViewerCommandTargetInput("local:/tmp/error.log")
	localCompose := normalizeViewerCommandTargetInput("local:/tmp/docker-compose.yml")
	st := &settingsModalState{
		viewTargetEntries: []viewerCommandTargetEntry{
			{Key: localLog, Command: `tail -f {path}`},
			{Key: localCompose, Command: `docker compose -f {path} config`},
		},
	}
	st.openViewerCommandTargetPicker()
	st.viewTargetPickList.Axis = layout.Vertical

	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(520, 240),
		},
	}

	frame := func() layout.Dimensions {
		gtx.Ops.Reset()
		entries, matches := st.viewerCommandTargetPickerEntries()
		dims := ui.layoutSettingsViewerCommandTargetPicker(th, gtx, st, entries, matches)
		r.Frame(gtx.Ops)
		return dims
	}

	dims := frame()
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("picker has invalid size: %v", dims.Size)
	}

	st.viewerCommandTargetRowClick(localLog).Click()
	frame()

	if got := st.viewTargetKeyEdit.Text(); got != localLog {
		t.Fatalf("target key not loaded from picker row: got %q", got)
	}
	if got := st.viewTargetCommandEdit.Text(); got != `tail -f {path}` {
		t.Fatalf("command not loaded from picker row: got %q", got)
	}
	if st.viewTargetPickOpen {
		t.Fatal("picker should close after row selection")
	}
}

func TestSettingsViewerCommandTargetPickerRemoveClickDeletesDraftEntry(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	localLog := normalizeViewerCommandTargetInput("local:/tmp/error.log")
	localCompose := normalizeViewerCommandTargetInput("local:/tmp/docker-compose.yml")
	st := &settingsModalState{
		viewTargetEntries: []viewerCommandTargetEntry{
			{Key: localLog, Command: `tail -f {path}`},
			{Key: localCompose, Command: `docker compose -f {path} config`},
		},
	}
	st.openViewerCommandTargetPicker()
	st.viewTargetPickList.Axis = layout.Vertical

	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(520, 240),
		},
	}

	frame := func() {
		gtx.Ops.Reset()
		entries, matches := st.viewerCommandTargetPickerEntries()
		_ = ui.layoutSettingsViewerCommandTargetPicker(th, gtx, st, entries, matches)
		r.Frame(gtx.Ops)
	}

	frame()
	st.viewerCommandTargetRowRemoveClick(localLog).Click()
	frame()

	if _, ok := st.viewerCommandTarget(localLog); ok {
		t.Fatal("exact override should be removed from draft list")
	}
	if !strings.Contains(st.targetInfoText, "Pending removal") {
		t.Fatalf("targetInfoText=%q want pending removal notice", st.targetInfoText)
	}
	if !st.viewTargetPickOpen {
		t.Fatal("picker should stay open after removing a row")
	}
}

func TestRefreshViewerCommandTargetDraftInfoPromptsUpdateForExistingOverride(t *testing.T) {
	localLog := normalizeViewerCommandTargetInput("local:/tmp/error.log")
	st := &settingsModalState{
		viewTargetEntries: []viewerCommandTargetEntry{
			{Key: localLog, Command: `tail -f {path}`},
		},
		viewTargetSavedEntries: []viewerCommandTargetEntry{
			{Key: localLog, Command: `tail -f {path}`},
		},
	}
	st.viewTargetKeyEdit.SetText(localLog)
	st.viewTargetCommandEdit.SetText(`tail -n 50 {path}`)

	st.refreshViewerCommandTargetDraftInfo(false)

	entry, ok := st.viewerCommandTarget(localLog)
	if !ok {
		t.Fatal("existing exact override should still exist")
	}
	if entry.Command != `tail -f {path}` {
		t.Fatalf("existing exact override should not change before Update: got %q", entry.Command)
	}
	if !strings.Contains(st.targetInfoText, "Click Update") {
		t.Fatalf("missing override update hint, got %q", st.targetInfoText)
	}
}

func TestViewerCommandTargetNoticeTextUsesCurrentEditorState(t *testing.T) {
	localLog := normalizeViewerCommandTargetInput("local:/tmp/error.log")
	st := &settingsModalState{
		viewTargetEntries: []viewerCommandTargetEntry{
			{Key: localLog, Command: `tail -n 50 {path}`},
		},
		viewTargetSavedEntries: []viewerCommandTargetEntry{
			{Key: localLog, Command: `tail -f {path}`},
		},
	}
	st.viewTargetKeyEdit.SetText(localLog)
	st.viewTargetCommandEdit.SetText(`tail -n 50 {path}`)

	got := st.viewerCommandTargetNoticeText()
	if !strings.Contains(got, "Pending change") {
		t.Fatalf("viewerCommandTargetNoticeText=%q, want pending change notice", got)
	}
}

func TestViewerCommandTargetNoticeTextPromptsAddForNewEntry(t *testing.T) {
	st := &settingsModalState{}
	st.viewTargetKeyEdit.SetText("/tmp/error.log")
	st.viewTargetCommandEdit.SetText(`tail -f {path}`)

	got := st.viewerCommandTargetNoticeText()
	if !strings.Contains(got, "Click Add") {
		t.Fatalf("viewerCommandTargetNoticeText=%q, want add prompt", got)
	}
}

func TestViewerCommandRulePickerRulesFiltersAndFallsBackToAll(t *testing.T) {
	st := &settingsModalState{
		viewRuleEntries: []fm.ViewerCommandRule{
			{Pattern: `(?i)\.log$`, Command: `tail -f {path}`},
			{Pattern: `^docker.*\.ya?ml$`, Command: `docker compose -f {path} config`},
			{Pattern: `^README`, Command: `bat {path}`},
		},
	}

	st.viewRulePatternEdit.SetText("docker")
	rules, matches := st.viewerCommandRulePickerRules()
	if matches != 1 {
		t.Fatalf("matches=%d want 1", matches)
	}
	if len(rules) != 1 || rules[0].Pattern != `^docker.*\.ya?ml$` {
		t.Fatalf("unexpected filtered rules: %#v", rules)
	}

	st.viewRulePatternEdit.SetText("nomatch")
	rules, matches = st.viewerCommandRulePickerRules()
	if matches != 0 {
		t.Fatalf("matches=%d want 0", matches)
	}
	if len(rules) != 3 {
		t.Fatalf("len(rules)=%d want 3", len(rules))
	}
}

func TestSettingsViewerCommandRulePickerRowClickLoadsPatternAndCommand(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	st := &settingsModalState{
		viewRuleEntries: []fm.ViewerCommandRule{
			{Pattern: `(?i)\.log$`, Command: `tail -f {path}`},
			{Pattern: `^docker.*\.ya?ml$`, Command: `docker compose -f {path} config`},
		},
	}
	st.openViewerCommandRulePicker()
	st.viewRulePickList.Axis = layout.Vertical

	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(520, 240),
		},
	}

	frame := func() layout.Dimensions {
		gtx.Ops.Reset()
		rules, matches := st.viewerCommandRulePickerRules()
		dims := ui.layoutSettingsViewerCommandRulePicker(th, gtx, st, rules, matches)
		r.Frame(gtx.Ops)
		return dims
	}

	dims := frame()
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("picker has invalid size: %v", dims.Size)
	}

	st.viewerCommandRuleRowClick(`(?i)\.log$`).Click()
	frame()

	if got := st.viewRulePatternEdit.Text(); got != `(?i)\.log$` {
		t.Fatalf("pattern not loaded from picker row: got %q", got)
	}
	if got := st.viewRuleCommandEdit.Text(); got != `tail -f {path}` {
		t.Fatalf("command not loaded from picker row: got %q", got)
	}
	if st.viewRulePickOpen {
		t.Fatal("picker should close after row selection")
	}
}

func TestSettingsViewerCommandRulePickerRemoveClickDeletesDraftRule(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	st := &settingsModalState{
		viewRuleEntries: []fm.ViewerCommandRule{
			{Pattern: `(?i)\.log$`, Command: `tail -f {path}`},
			{Pattern: `^docker.*\.ya?ml$`, Command: `docker compose -f {path} config`},
		},
	}
	st.openViewerCommandRulePicker()
	st.viewRulePickList.Axis = layout.Vertical

	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(520, 240),
		},
	}

	frame := func() {
		gtx.Ops.Reset()
		rules, matches := st.viewerCommandRulePickerRules()
		_ = ui.layoutSettingsViewerCommandRulePicker(th, gtx, st, rules, matches)
		r.Frame(gtx.Ops)
	}

	frame()
	st.viewerCommandRuleRowRemoveClick(`(?i)\.log$`).Click()
	frame()

	if _, ok := st.viewerCommandRule(`(?i)\.log$`); ok {
		t.Fatal("regex rule should be removed from draft list")
	}
	if !strings.Contains(st.ruleInfoText, "Pending removal") {
		t.Fatalf("ruleInfoText=%q want pending removal notice", st.ruleInfoText)
	}
	if !st.viewRulePickOpen {
		t.Fatal("picker should stay open after removing a row")
	}
}

func TestRefreshViewerCommandRuleDraftInfoPromptsUpdateForExistingRule(t *testing.T) {
	st := &settingsModalState{
		viewRuleEntries: []fm.ViewerCommandRule{
			{Pattern: `(?i)\.log$`, Command: `tail -f {path}`},
		},
		viewRuleSavedEntries: []fm.ViewerCommandRule{
			{Pattern: `(?i)\.log$`, Command: `tail -f {path}`},
		},
	}
	st.viewRulePatternEdit.SetText(`(?i)\.log$`)
	st.viewRuleCommandEdit.SetText(`tail -n 200 {path}`)

	st.refreshViewerCommandRuleDraftInfo(false)

	rule, ok := st.viewerCommandRule(`(?i)\.log$`)
	if !ok {
		t.Fatal("existing rule should still exist")
	}
	if rule.Command != `tail -f {path}` {
		t.Fatalf("existing rule should not change before Update: got %q", rule.Command)
	}
	if !strings.Contains(st.ruleInfoText, "Click Update") {
		t.Fatalf("missing rule update hint, got %q", st.ruleInfoText)
	}
}

func TestViewerCommandRuleNoticeTextUsesCurrentEditorState(t *testing.T) {
	st := &settingsModalState{
		viewRuleEntries: []fm.ViewerCommandRule{
			{Pattern: `(?i)\.log$`, Command: `tail -n 200 {path}`},
		},
		viewRuleSavedEntries: []fm.ViewerCommandRule{
			{Pattern: `(?i)\.log$`, Command: `tail -f {path}`},
		},
	}
	st.viewRulePatternEdit.SetText(`(?i)\.log$`)
	st.viewRuleCommandEdit.SetText(`tail -n 200 {path}`)

	got := st.viewerCommandRuleNoticeText()
	if !strings.Contains(got, "Pending change") {
		t.Fatalf("viewerCommandRuleNoticeText=%q, want pending change notice", got)
	}
}

func TestViewerCommandRuleNoticeTextPromptsAddForNewRule(t *testing.T) {
	st := &settingsModalState{}
	st.viewRulePatternEdit.SetText(`(?i)\.log$`)
	st.viewRuleCommandEdit.SetText(`tail -f {path}`)

	got := st.viewerCommandRuleNoticeText()
	if !strings.Contains(got, "Click Add") {
		t.Fatalf("viewerCommandRuleNoticeText=%q, want add prompt", got)
	}
}

func TestSettingsColorSwatchGroupsAlwaysContainOnlyHive(t *testing.T) {
	initial := settingsColorSwatchGroups("")
	if got := settingsColorSwatchCount(initial); got != 127 {
		t.Fatalf("initial swatch count=%d want 127", got)
	}
	groups := settingsColorSwatchGroups("#2D9AA5")
	if got := settingsColorSwatchCount(groups); got != 127 {
		t.Fatalf("selected-base swatch count=%d want 127", got)
	}
}

func TestSettingsColorHiveHasCompactHexagonalRows(t *testing.T) {
	groups := settingsColorHiveGroups()
	wantRows := []int{7, 8, 9, 10, 11, 12, 13, 12, 11, 10, 9, 8, 7}
	if len(groups) != len(wantRows) {
		t.Fatalf("hive rows=%d want %d", len(groups), len(wantRows))
	}
	seen := make(map[string]bool)
	for row, want := range wantRows {
		if got := len(groups[row].hexes); got != want {
			t.Fatalf("row %d swatches=%d want %d", row, got, want)
		}
		for _, hex := range groups[row].hexes {
			if seen[hex] {
				t.Fatalf("duplicate hive color %s", hex)
			}
			seen[hex] = true
		}
	}
}

func TestSettingsColorShadeUsesBaseAtCenterAndTintsBothWays(t *testing.T) {
	const base = "#4080C0"
	if got := settingsColorShade(base, 0.5); got != base {
		t.Fatalf("center shade=%q want %q", got, base)
	}
	if got := settingsColorShade(base, 0); got != "#000000" {
		t.Fatalf("dark endpoint=%q want black", got)
	}
	if got := settingsColorShade(base, 1); got != "#FFFFFF" {
		t.Fatalf("light endpoint=%q want white", got)
	}
}

func TestSettingsColorCategoriesSeparateFocusedStates(t *testing.T) {
	st := &settingsModalState{
		colorScope:               "panes",
		colorPopupHover:          "#485F96",
		colorPopupHoverText:      "#F6F9FF",
		colorSelection:           "#3456AA",
		colorSelectionText:       "#F2F7FF",
		colorFocusedSelected:     "#447F9C",
		colorFocusedSelectedText: "#F6FBFF",
	}
	if got := settingsColorLabel("panes", "selection"); got != "Focused" {
		t.Fatalf("selection label=%q want %q", got, "Focused")
	}
	if got := settingsColorLabel("panes", "focused_selected"); got != "Focused + Selected Files" {
		t.Fatalf("focused_selected label=%q want %q", got, "Focused + Selected Files")
	}
	if got := settingsColorLabel("panes", "popup_hover"); got != "Popup Hover" {
		t.Fatalf("popup_hover label=%q want %q", got, "Popup Hover")
	}
	if got := st.colorValue("popup_hover"); got != "#485F96" {
		t.Fatalf("popup_hover background=%q want %q", got, "#485F96")
	}
	if got := st.colorTextValue("popup_hover"); got != "#F6F9FF" {
		t.Fatalf("popup_hover text=%q want %q", got, "#F6F9FF")
	}
	if got := st.colorValue("selection"); got != "#3456AA" {
		t.Fatalf("selection background=%q want %q", got, "#3456AA")
	}
	if got := st.colorValue("focused_selected"); got != "#447F9C" {
		t.Fatalf("focused_selected background=%q want %q", got, "#447F9C")
	}
	if got := st.colorTextValue("selection"); got != "#F2F7FF" {
		t.Fatalf("selection text=%q want %q", got, "#F2F7FF")
	}
	if got := st.colorTextValue("focused_selected"); got != "#F6FBFF" {
		t.Fatalf("focused_selected text=%q want %q", got, "#F6FBFF")
	}
}

func TestDraftFilePanePaletteAppliesExplicitTextColors(t *testing.T) {
	st := &settingsModalState{
		colorPaneBackground:      "#101820",
		colorPaneText:            "#C8D0D8",
		colorHover:               "#20354F",
		colorHoverText:           "#EAF3FF",
		colorPopupHover:          "#485F96",
		colorPopupHoverText:      "#F6F9FF",
		colorSelection:           "#3456AA",
		colorSelectionText:       "#F2F7FF",
		colorSelectedFiles:       "#286F57",
		colorSelectedFilesText:   "#E6F7EE",
		colorFocusedSelected:     "#447F9C",
		colorFocusedSelectedText: "#F6FBFF",
	}

	palette, errText := st.draftFilePanePalette(fm.DefaultConfig())
	if errText != "" {
		t.Fatalf("unexpected draft palette error: %q", errText)
	}
	if got := fm.FormatHexColor(palette.PaneFg); got != "#C8D0D8" {
		t.Fatalf("PaneFg=%q want %q", got, "#C8D0D8")
	}
	if got := fm.FormatHexColor(palette.HoverBg); got != "#20354F" {
		t.Fatalf("HoverBg=%q want %q", got, "#20354F")
	}
	if got := fm.FormatHexColor(palette.HoverFg); got != "#EAF3FF" {
		t.Fatalf("HoverFg=%q want %q", got, "#EAF3FF")
	}
	if got := fm.FormatHexColor(palette.PopupHoverBg); got != "#485F96" {
		t.Fatalf("PopupHoverBg=%q want %q", got, "#485F96")
	}
	if got := fm.FormatHexColor(palette.PopupHoverFg); got != "#F6F9FF" {
		t.Fatalf("PopupHoverFg=%q want %q", got, "#F6F9FF")
	}
	if got := fm.FormatHexColor(palette.SelectedFg); got != "#F2F7FF" {
		t.Fatalf("SelectedFg=%q want %q", got, "#F2F7FF")
	}
	if got := fm.FormatHexColor(palette.MarkedFg); got != "#E6F7EE" {
		t.Fatalf("MarkedFg=%q want %q", got, "#E6F7EE")
	}
	if got := fm.FormatHexColor(palette.MarkedSelBg); got != "#447F9C" {
		t.Fatalf("MarkedSelBg=%q want %q", got, "#447F9C")
	}
	if got := fm.FormatHexColor(palette.MarkedSelFg); got != "#F6FBFF" {
		t.Fatalf("MarkedSelFg=%q want %q", got, "#F6FBFF")
	}
}

func TestDraftFilePanePaletteAllowsTransparentRowText(t *testing.T) {
	st := &settingsModalState{
		colorPaneBackground:      "#101820",
		colorPaneText:            "#C8D0D8",
		colorHover:               "#20354F",
		colorHoverText:           "transparent",
		colorSelection:           "#3456AA",
		colorSelectionText:       "transparent",
		colorSelectedFiles:       "#286F57",
		colorSelectedFilesText:   "transparent",
		colorFocusedSelected:     "#447F9C",
		colorFocusedSelectedText: "transparent",
	}

	palette, errText := st.draftFilePanePalette(fm.DefaultConfig())
	if errText != "" {
		t.Fatalf("unexpected draft palette error: %q", errText)
	}
	if palette.HoverFg != (color.NRGBA{}) {
		t.Fatalf("HoverFg=%v want transparent", palette.HoverFg)
	}
	if palette.SelectedFg != (color.NRGBA{}) {
		t.Fatalf("SelectedFg=%v want transparent", palette.SelectedFg)
	}
	if palette.MarkedFg != (color.NRGBA{}) {
		t.Fatalf("MarkedFg=%v want transparent", palette.MarkedFg)
	}
	if palette.MarkedSelFg != (color.NRGBA{}) {
		t.Fatalf("MarkedSelFg=%v want transparent", palette.MarkedSelFg)
	}

	colors := filePanePaletteToConfigColors(palette)
	if colors.HoverText != fm.TransparentColor || colors.SelectionText != fm.TransparentColor ||
		colors.SelectedFilesText != fm.TransparentColor || colors.FocusedSelectedText != fm.TransparentColor {
		t.Fatalf("transparent row text colors not preserved: %#v", colors)
	}
}

func TestSettingsColorTextTransparentToggleUpdatesEditor(t *testing.T) {
	st := &settingsModalState{
		colorScope:     "panes",
		colorCategory:  "hover",
		colorHoverText: "#ABCDEF",
	}
	st.colorTextValueEdit.SetText(st.colorHoverText)

	if !st.setColorTextTransparent(true) {
		t.Fatal("setColorTextTransparent(true) should report a change")
	}
	if st.colorHoverText != fm.TransparentColor {
		t.Fatalf("colorHoverText=%q want transparent", st.colorHoverText)
	}
	if got := st.colorTextValueEdit.Text(); got != fm.TransparentColor {
		t.Fatalf("editor text=%q want transparent", got)
	}
	if !st.colorTextTransparentBool.Value {
		t.Fatal("transparent checkbox should be checked")
	}

	if !st.setColorTextTransparent(false) {
		t.Fatal("setColorTextTransparent(false) should report a change")
	}
	if st.colorHoverText != fm.DefaultFilePaneHoverTextHex {
		t.Fatalf("colorHoverText=%q want default hover text", st.colorHoverText)
	}
	if got := st.colorTextValueEdit.Text(); got != fm.DefaultFilePaneHoverTextHex {
		t.Fatalf("editor text=%q want default hover text", got)
	}
	if st.colorTextTransparentBool.Value {
		t.Fatal("transparent checkbox should be unchecked")
	}
}

func TestSettingsLoadDefaultsTransparentRowText(t *testing.T) {
	st := &settingsModalState{
		colorScope:    "panes",
		colorCategory: "hover",
	}
	st.loadFromConfig(fm.DefaultConfig())

	if got := st.colorTextValue("hover"); got != fm.TransparentColor {
		t.Fatalf("hover text=%q want transparent", got)
	}
	if got := st.colorTextValueEdit.Text(); got != fm.TransparentColor {
		t.Fatalf("text editor=%q want transparent", got)
	}
	if !st.colorTextTransparentBool.Value {
		t.Fatal("transparent checkbox should be checked by default")
	}
}

func TestDraftViewerThemeAppliesExplicitOverrides(t *testing.T) {
	st := &settingsModalState{
		colorScope:            "viewer",
		colorViewerBackground: "#112233",
		colorViewerText:       "#F1E2D3",
		colorViewerSelection:  "#3355CC",
	}

	theme, errText := st.draftViewerTheme(fm.DefaultConfig())
	if errText != "" {
		t.Fatalf("unexpected viewer preview error: %q", errText)
	}
	if got := fm.FormatHexColor(theme.PanelBg); got != "#112233" {
		t.Fatalf("PanelBg=%q want %q", got, "#112233")
	}
	if got := fm.FormatHexColor(theme.Text); got != "#F1E2D3" {
		t.Fatalf("Text=%q want %q", got, "#F1E2D3")
	}
	if contrastScore(theme.PanelBg, theme.Selection) <= 1.45 {
		t.Fatalf("selection contrast=%0.2f want > 1.45", contrastScore(theme.PanelBg, theme.Selection))
	}
}

func TestDraftViewerThemeRejectsInvalidViewerColor(t *testing.T) {
	st := &settingsModalState{
		colorScope:            "viewer",
		colorViewerBackground: "oops",
		colorViewerText:       "#D2D2D2",
	}

	_, errText := st.draftViewerTheme(fm.DefaultConfig())
	if !strings.Contains(errText, "Viewer background") {
		t.Fatalf("errText=%q, want viewer background validation", errText)
	}
}

func TestSettingsViewerColorCategoryUsesSelectionValue(t *testing.T) {
	st := &settingsModalState{
		colorScope:            "viewer",
		colorViewerBackground: "#112233",
		colorViewerText:       "#F1E2D3",
		colorViewerSelection:  "#3355CC",
	}

	if got := settingsColorLabel("viewer", "selection"); got != "Selection" {
		t.Fatalf("viewer selection label=%q want %q", got, "Selection")
	}
	if got := st.colorValue("selection"); got != "#3355CC" {
		t.Fatalf("viewer selection value=%q want %q", got, "#3355CC")
	}
	if settingsViewerCategoryHasText("selection") {
		t.Fatal("viewer selection category should not expose a separate text field")
	}
}

func TestSettingsViewerHexSectionCategoriesUseIndependentColors(t *testing.T) {
	st := &settingsModalState{
		colorScope:               "viewer",
		colorViewerHexSelection:  "#224466",
		colorViewerHexOffsetText: "#112233",
		colorViewerHexBytesText:  "#445566",
		colorViewerHexASCIIText:  "#778899",
	}

	for key, want := range map[string]string{
		"hex_selection": "#224466",
		"hex_offset":    "#112233",
		"hex_bytes":     "#445566",
		"hex_ascii":     "#778899",
	} {
		if got := st.colorValue(key); got != want {
			t.Fatalf("%s color=%q want %q", key, got, want)
		}
		if settingsViewerCategoryHasText(key) {
			t.Fatalf("%s should expose one text-color field", key)
		}
	}

	theme, errText := st.draftViewerTheme(fm.DefaultConfig())
	if errText != "" {
		t.Fatalf("unexpected viewer preview error: %q", errText)
	}
	if got := fm.FormatHexColor(theme.OffsetText); got != "#112233" {
		t.Fatalf("OffsetText=%q", got)
	}
	if theme.HexSelection == theme.Selection {
		t.Fatal("hex selection preview should use its independent override")
	}
	if got := fm.FormatHexColor(theme.HexText); got != "#445566" {
		t.Fatalf("HexText=%q", got)
	}
	if got := fm.FormatHexColor(theme.ASCIIText); got != "#778899" {
		t.Fatalf("ASCIIText=%q", got)
	}
}

func TestSettingsViewerPreviewSelectionFillIsOpaque(t *testing.T) {
	fill := settingsViewerPreviewSelectionFill(fileViewerTheme{
		Selection:       colorNRGBA(0x11, 0x22, 0x33, 0x80),
		StrongSelection: colorNRGBA(0x44, 0x55, 0x66, 0x99),
	}, false, false)
	if fill.A != 0xFF {
		t.Fatalf("selection alpha=%d want 255", fill.A)
	}

	strong := settingsViewerPreviewSelectionFill(fileViewerTheme{
		Selection:       colorNRGBA(0x11, 0x22, 0x33, 0x80),
		StrongSelection: colorNRGBA(0x44, 0x55, 0x66, 0x99),
	}, true, false)
	if strong.A != 0xFF {
		t.Fatalf("strong selection alpha=%d want 255", strong.A)
	}
}

func TestSettingsViewerPreviewSelectionRectUsesFullRow(t *testing.T) {
	rect := settingsViewerPreviewSelectionRect(180, 24)
	if rect != image.Rect(0, 0, 180, 24) {
		t.Fatalf("selection rect=%v want %v", rect, image.Rect(0, 0, 180, 24))
	}

	if rect := settingsViewerPreviewSelectionRect(0, 24); !rect.Empty() {
		t.Fatalf("zero-width selection rect=%v want empty", rect)
	}
}

func TestSettingsViewerPreviewContentUsesContiguousRows(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	st := &settingsModalState{}

	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(420, 240),
		},
	}

	theme := ui.fileViewerTheme()
	for _, mode := range []string{"file", "hex"} {
		st.viewerPreviewMode = mode
		wantH := st.previewViewerContentHeight(ui, th, gtx)
		got := ui.layoutSettingsViewerPreviewContent(th, gtx, st, theme, ui)
		if got.Size.Y != wantH {
			t.Fatalf("%s preview content height=%d want %d", mode, got.Size.Y, wantH)
		}
	}
}

func TestSettingsViewerPreviewModeDefaultsAndNormalizes(t *testing.T) {
	st := &settingsModalState{}
	if got := st.normalizedViewerPreviewMode(); got != "file" {
		t.Fatalf("empty preview mode=%q want file", got)
	}
	st.viewerPreviewMode = "hex"
	if got := st.normalizedViewerPreviewMode(); got != "hex" {
		t.Fatalf("hex preview mode=%q want hex", got)
	}
	st.viewerPreviewMode = "command"
	if got := st.normalizedViewerPreviewMode(); got != "file" {
		t.Fatalf("unsupported preview mode=%q want file", got)
	}
}

func TestSettingsConfigEditorUsesFullWidth(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	st := &settingsModalState{}
	st.configEdit.SingleLine = false
	st.configEdit.Submit = false
	st.configEdit.SetText("general:\n  font_size_sp: 14\n")

	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(420, 240),
		},
	}

	dims, _, _ := ui.layoutSettingsConfigEditor(th, gtx, st, settingsScrollbarStyle(th, &st.configScrollbar))
	if dims.Size.X != gtx.Constraints.Max.X {
		t.Fatalf("config editor width=%d want %d", dims.Size.X, gtx.Constraints.Max.X)
	}
	if dims.Size.Y != gtx.Constraints.Max.Y {
		t.Fatalf("config editor height=%d want %d", dims.Size.Y, gtx.Constraints.Max.Y)
	}
}

func TestSettingsColorsPreviewHostHeightUsesSharedValue(t *testing.T) {
	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(420, 240),
		},
	}

	if got, want := settingsColorsPreviewHostHeight(gtx), 188; got != want {
		t.Fatalf("preview host height=%d want %d", got, want)
	}
}

func TestSettingsPreviewScrollbarGeometryCentersThumb(t *testing.T) {
	track, thumb := settingsPreviewScrollbarGeometry(8, 88, 22)

	if track != image.Rect(0, 0, 8, 88) {
		t.Fatalf("track=%v want %v", track, image.Rect(0, 0, 8, 88))
	}
	if thumb != image.Rect(1, 33, 7, 55) {
		t.Fatalf("thumb=%v want %v", thumb, image.Rect(1, 33, 7, 55))
	}
}

func TestSettingsPanePreviewScrollbarFillsAvailableHeight(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	palette := filePanePaletteFromConfig(ui.fmCfg)

	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(32, 118),
		},
	}

	dims := ui.layoutSettingsPanePreviewScrollbar(gtx, palette)
	if dims.Size.X != 8 {
		t.Fatalf("pane preview scrollbar width=%d want 8", dims.Size.X)
	}
	if dims.Size.Y != 118 {
		t.Fatalf("pane preview scrollbar height=%d want available height 118", dims.Size.Y)
	}
}

func TestMeasureTypefaceLineHeightDoesNotPaintSampleGlyphs(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()

	var ops op.Ops
	var r input.Router
	gtx := layout.Context{
		Ops:    &ops,
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(420, 240),
		},
	}

	before := reflect.ValueOf(&ops.Internal).Elem().FieldByName("data").Len()
	if got := measureTypefaceLineHeight(ui, th, gtx, ui.viewerTypeface()); got < 12 {
		t.Fatalf("line height=%d want >= 12", got)
	}
	after := reflect.ValueOf(&ops.Internal).Elem().FieldByName("data").Len()
	if after != before {
		t.Fatalf("measureTypefaceLineHeight wrote ops: before=%d after=%d", before, after)
	}
}

func TestSettingsViewerPreviewContentHeightUsesTextRows(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	st := &settingsModalState{}

	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(420, 240),
		},
	}

	wantH := st.previewViewerLineHeight(ui, th, gtx, false) * 4
	gotH := st.previewViewerContentHeight(ui, th, gtx)
	if gotH != wantH {
		t.Fatalf("preview content height=%d want %d", gotH, wantH)
	}
}

func TestDraftFilenameColorsNormalizesAllRuleTypes(t *testing.T) {
	st := &settingsModalState{
		filenameDefaultText:   "#AABBCC",
		filenameDefaultIcon:   fm.FilenameIconDocument,
		filenameDefaultTarget: "dirs",
		filenameAgeEntries: []fm.FilenameAgeRule{
			{MaxAge: "24h", Target: "files", Text: "#112233", Icon: fm.FilenameIconRecent},
			{MaxAge: "1w", Text: "", Icon: ""},
			{MaxAge: "3d", Text: "#334455", Icon: ""},
		},
		filenamePermEntries: []fm.FilenamePermissionRule{
			{Permissions: "111", Match: "any", Target: "dirs", Text: "#556677", Icon: fm.FilenameIconLocked},
		},
		filenameExtEntries: []fm.FilenameExtensionRule{
			{Extension: "GO", Text: "#223344", Icon: fm.FilenameIconCode},
		},
		filenameSizeEntries: []fm.FilenameSizeRule{
			{Size: "10mb", Match: "max", Text: "#778899", Icon: fm.FilenameIconArchive},
		},
	}

	got, errText := st.draftFilenameColors()
	if errText != "" {
		t.Fatalf("unexpected draft filename error: %q", errText)
	}
	if got.Text != "" {
		t.Fatalf("default text=%q want empty", got.Text)
	}
	if got.Icon != "" {
		t.Fatalf("default icon=%q want empty", got.Icon)
	}
	if got.Target != "" {
		t.Fatalf("default target=%q want empty", got.Target)
	}
	if len(got.AgeRules) != 2 {
		t.Fatalf("len(AgeRules)=%d want 2", len(got.AgeRules))
	}
	if got.AgeRules[0].MaxAge != "1d" || got.AgeRules[1].MaxAge != "3d" {
		t.Fatalf("age rules=%#v want normalized 1d and 3d", got.AgeRules)
	}
	if got.AgeRules[0].Target != fm.FilenameTargetFiles {
		t.Fatalf("age target=%q want %q", got.AgeRules[0].Target, fm.FilenameTargetFiles)
	}
	if len(got.PermissionRules) != 1 || got.PermissionRules[0].Permissions != "0111" {
		t.Fatalf("permission rules=%#v want normalized 0111", got.PermissionRules)
	}
	if got.PermissionRules[0].Match != fm.FilenamePermissionMatchAny {
		t.Fatalf("permission match=%q want %q", got.PermissionRules[0].Match, fm.FilenamePermissionMatchAny)
	}
	if got.PermissionRules[0].Target != fm.FilenameTargetDirs {
		t.Fatalf("permission target=%q want %q", got.PermissionRules[0].Target, fm.FilenameTargetDirs)
	}
	if len(got.ExtensionRules) != 1 || got.ExtensionRules[0].Extension != ".go" {
		t.Fatalf("extension rules=%#v want normalized .go", got.ExtensionRules)
	}
	if len(got.SizeRules) != 1 || got.SizeRules[0].Size != "10m" {
		t.Fatalf("size rules=%#v want normalized 10m", got.SizeRules)
	}
	if got.SizeRules[0].Match != fm.FilenameSizeMatchAtMost {
		t.Fatalf("size match=%q want %q", got.SizeRules[0].Match, fm.FilenameSizeMatchAtMost)
	}
}

func TestDraftFilenameColorsIgnoresDeprecatedDefaultFilenameFields(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Colors.FilePaneText = "#13579B"
	cfg.Colors.Filenames.Text = "#2468AC"
	cfg.Colors.Filenames.Icon = fm.FilenameIconCode
	cfg.Colors.Filenames.Target = fm.FilenameTargetDirs

	st := &settingsModalState{}
	st.loadFilenameColorsFromConfig(cfg)

	if st.filenameDefaultText != "" {
		t.Fatalf("filenameDefaultText=%q want empty deprecated override", st.filenameDefaultText)
	}
	if got := st.filenameDefaultTextEdit.Text(); got != "" {
		t.Fatalf("filenameDefaultTextEdit=%q want empty deprecated override", got)
	}
	if st.filenameDefaultIcon != "" {
		t.Fatalf("filenameDefaultIcon=%q want empty", st.filenameDefaultIcon)
	}
	if st.filenameDefaultTarget != "both" {
		t.Fatalf("filenameDefaultTarget=%q want both", st.filenameDefaultTarget)
	}

	got, errText := st.draftFilenameColors()
	if errText != "" {
		t.Fatalf("unexpected draft filename error: %q", errText)
	}
	if got.Text != "" {
		t.Fatalf("draft filename text=%q want empty inherited override", got.Text)
	}
	if got.Icon != "" {
		t.Fatalf("draft filename icon=%q want empty", got.Icon)
	}
	if got.Target != "" {
		t.Fatalf("draft filename target=%q want empty", got.Target)
	}
}

func TestFilenameIconPickerValueMapsTargets(t *testing.T) {
	st := &settingsModalState{}

	tests := []struct {
		target string
		icon   string
	}{
		{target: "filename-age-icon", icon: fm.FilenameIconAudio},
		{target: "filename-perm-icon", icon: fm.FilenameIconLink},
		{target: "filename-ext-icon", icon: fm.FilenameIconTable},
		{target: "filename-size-icon", icon: fm.FilenameIconApp},
	}

	for _, tc := range tests {
		st.setFilenameIconPickerValue(tc.target, tc.icon)
		if got := st.filenameIconPickerValue(tc.target); got != tc.icon {
			t.Fatalf("filenameIconPickerValue(%q)=%q want %q", tc.target, got, tc.icon)
		}
	}
}

func TestFilenameExtensionUIUsesBareSuffixDisplay(t *testing.T) {
	st := &settingsModalState{}
	st.loadFilenameExtensionFields(".tar.gz", "", fm.FilenameIconArchive, "both")

	if got := st.filenameExtEdit.Text(); got != "tar.gz" {
		t.Fatalf("filenameExtEdit=%q want bare suffix", got)
	}

	rule, err := parseFilenameExtensionRuleFields("go", "", fm.FilenameIconCode, "files")
	if err != nil {
		t.Fatalf("parseFilenameExtensionRuleFields error: %v", err)
	}
	if rule.Extension != ".go" {
		t.Fatalf("rule.Extension=%q want %q", rule.Extension, ".go")
	}
	if rule.Target != fm.FilenameTargetFiles {
		t.Fatalf("rule.Target=%q want %q", rule.Target, fm.FilenameTargetFiles)
	}
	if got := formatFilenameExtensionRuleLabel(rule); got != "go" {
		t.Fatalf("formatFilenameExtensionRuleLabel=%q want %q", got, "go")
	}
}

func TestPreviewFilenameThemeUsesCurrentPermissionDraftVisual(t *testing.T) {
	st := &settingsModalState{
		filenamePermEntries: []fm.FilenamePermissionRule{
			{Permissions: "0111", Match: fm.FilenamePermissionMatchAny, Text: "#112233"},
		},
		filenamePermMatch: fm.FilenamePermissionMatchAny,
		filenamePermIcon:  fm.FilenameIconLocked,
	}
	st.filenamePermEdit.SetText("0111")
	st.filenamePermTextEdit.SetText("#AABBCC")

	_, theme, colors, errText := st.previewFilenameTheme(fm.DefaultConfig())
	if errText != "" {
		t.Fatalf("unexpected preview filename error: %q", errText)
	}
	if len(colors.PermissionRules) != 1 {
		t.Fatalf("len(colors.PermissionRules)=%d want 1", len(colors.PermissionRules))
	}
	if colors.PermissionRules[0].Text != "#AABBCC" {
		t.Fatalf("preview permission text=%q want %q", colors.PermissionRules[0].Text, "#AABBCC")
	}
	if colors.PermissionRules[0].Icon != fm.FilenameIconLocked {
		t.Fatalf("preview permission icon=%q want %q", colors.PermissionRules[0].Icon, fm.FilenameIconLocked)
	}

	visual := theme.visualForEntry(filesys.Entry{
		Name:        "deploy.sh",
		DisplayName: "deploy.sh",
		Kind:        filesys.EntryFile,
		PermOctal:   "0755",
		ModTime:     time.Now().Add(-48 * time.Hour),
	}, time.Now())
	if visual.color != (color.NRGBA{R: 0xAA, G: 0xBB, B: 0xCC, A: 0xFF}) {
		t.Fatalf("preview permission color=%v want #AABBCC", visual.color)
	}
	if visual.iconKey != fm.FilenameIconLocked {
		t.Fatalf("preview permission icon=%q want %q", visual.iconKey, fm.FilenameIconLocked)
	}
}

func TestPreviewFilenameThemeUsesMatchingSampleForHasNonePermissions(t *testing.T) {
	rule := fm.FilenamePermissionRule{
		Permissions: "0520",
		Match:       fm.FilenamePermissionMatchNone,
		Text:        "#4C3FA8",
	}
	samplePerm := settingsFilenamePreviewSamplePermissions(rule)
	if samplePerm == "" {
		t.Fatal("samplePerm should not be empty")
	}
	if !fm.FilenamePermissionMatches(samplePerm, rule) {
		t.Fatalf("samplePerm=%q should satisfy rule %#v", samplePerm, rule)
	}

	theme := newFilePaneFilenameTheme(&fm.Config{
		Colors: fm.ColorsConfig{
			Filenames: fm.FilenameColorsConfig{
				PermissionRules: []fm.FilenamePermissionRule{rule},
			},
		},
	})
	visual := theme.visualForEntry(filesys.Entry{
		Name:        "mode-" + samplePerm + ".txt",
		DisplayName: "mode-" + samplePerm + ".txt",
		Kind:        filesys.EntryFile,
		PermOctal:   samplePerm,
		ModTime:     time.Now().Add(-48 * time.Hour),
	}, time.Now())
	if visual.color != (color.NRGBA{R: 0x4C, G: 0x3F, B: 0xA8, A: 0xFF}) {
		t.Fatalf("preview permission color=%v want #4C3FA8", visual.color)
	}
}

func TestDraftFilenameColorsRejectsInvalidAgeRule(t *testing.T) {
	st := &settingsModalState{
		filenameAgeEntries: []fm.FilenameAgeRule{
			{MaxAge: "later", Text: "#112233"},
		},
	}

	_, errText := st.draftFilenameColors()
	if !strings.Contains(errText, "Age rule 1") {
		t.Fatalf("errText=%q want age rule validation", errText)
	}
}

func TestUpsertCurrentFilenameAgeRuleNormalizesAndSortsEntries(t *testing.T) {
	st := &settingsModalState{
		filenameAgeEntries: []fm.FilenameAgeRule{
			{MaxAge: "1w", Text: "#334455"},
		},
		filenameAgeUnit: "h",
	}
	st.filenameAgeOffsetEdit.SetText("24")
	st.filenameAgeTextEdit.SetText("#112233")
	st.filenameAgeIcon = fm.FilenameIconRecent

	action, err := st.upsertCurrentFilenameAgeRule()
	if err != nil {
		t.Fatalf("upsertCurrentFilenameAgeRule error: %v", err)
	}
	if action != "Add" {
		t.Fatalf("action=%q want Add", action)
	}
	if len(st.filenameAgeEntries) != 2 {
		t.Fatalf("len(filenameAgeEntries)=%d want 2", len(st.filenameAgeEntries))
	}
	if st.filenameAgeEntries[0].MaxAge != "1d" || st.filenameAgeEntries[1].MaxAge != "1w" {
		t.Fatalf("filenameAgeEntries=%#v want sorted 1d then 1w", st.filenameAgeEntries)
	}
}

func TestFilenameRuleUpdatesCanChangeIdentityFields(t *testing.T) {
	t.Run("age", func(t *testing.T) {
		st := &settingsModalState{filenameAgeEntries: []fm.FilenameAgeRule{{MaxAge: "1d", Text: "#112233"}}}
		st.loadFilenameAgeFields("1", "d", "#112233", "", "both")
		st.filenameAgeOffsetEdit.SetText("2")
		st.syncFilenameAgeEditors()
		if got := st.filenameAgeTextEdit.Text(); got != "#112233" {
			t.Fatalf("color cleared while changing age: %q", got)
		}
		action, err := st.upsertCurrentFilenameAgeRule()
		if err != nil || action != "Update" {
			t.Fatalf("age update action=%q err=%v", action, err)
		}
		if len(st.filenameAgeEntries) != 1 || st.filenameAgeEntries[0].MaxAge != "2d" {
			t.Fatalf("age entries=%#v want one 2d rule", st.filenameAgeEntries)
		}
	})

	t.Run("permissions", func(t *testing.T) {
		st := &settingsModalState{filenamePermEntries: []fm.FilenamePermissionRule{{Permissions: "0644", Match: fm.FilenamePermissionMatchExact, Text: "#223344"}}}
		st.loadFilenamePermissionFields("0644", fm.FilenamePermissionMatchExact, "#223344", "", "both")
		st.filenamePermEdit.SetText("0755")
		st.syncFilenamePermissionEditors()
		action, err := st.upsertCurrentFilenamePermissionRule()
		if err != nil || action != "Update" {
			t.Fatalf("permission update action=%q err=%v", action, err)
		}
		if len(st.filenamePermEntries) != 1 || st.filenamePermEntries[0].Permissions != "0755" {
			t.Fatalf("permission entries=%#v want one 0755 rule", st.filenamePermEntries)
		}
	})

	t.Run("extension", func(t *testing.T) {
		st := &settingsModalState{filenameExtEntries: []fm.FilenameExtensionRule{{Extension: ".go", Text: "#334455"}}}
		st.loadFilenameExtensionFields(".go", "#334455", "", "both")
		st.filenameExtEdit.SetText("md")
		st.syncFilenameExtensionEditors()
		action, err := st.upsertCurrentFilenameExtensionRule()
		if err != nil || action != "Update" {
			t.Fatalf("extension update action=%q err=%v", action, err)
		}
		if len(st.filenameExtEntries) != 1 || st.filenameExtEntries[0].Extension != ".md" {
			t.Fatalf("extension entries=%#v want one .md rule", st.filenameExtEntries)
		}
	})

	t.Run("size", func(t *testing.T) {
		st := &settingsModalState{filenameSizeEntries: []fm.FilenameSizeRule{{Size: "1m", Match: fm.FilenameSizeMatchAtMost, Text: "#445566"}}}
		st.loadFilenameSizeFields("1m", fm.FilenameSizeMatchAtMost, "#445566", "", "both")
		st.filenameSizeEdit.SetText("2m")
		st.syncFilenameSizeEditors()
		action, err := st.upsertCurrentFilenameSizeRule()
		if err != nil || action != "Update" {
			t.Fatalf("size update action=%q err=%v", action, err)
		}
		if len(st.filenameSizeEntries) != 1 || st.filenameSizeEntries[0].Size != "2m" {
			t.Fatalf("size entries=%#v want one 2m rule", st.filenameSizeEntries)
		}
	})
}

func TestViewerSettingsUpdatesCanChangeIdentityFields(t *testing.T) {
	t.Run("target", func(t *testing.T) {
		oldKey := normalizeViewerCommandTargetInput("local:/tmp/old.log")
		newKey := normalizeViewerCommandTargetInput("local:/tmp/new.log")
		st := &settingsModalState{viewTargetEntries: []viewerCommandTargetEntry{{Key: oldKey, Command: "old-command"}}}
		st.loadViewerCommandTargetFields(oldKey, "old-command")
		st.viewTargetKeyEdit.SetText(newKey)
		st.syncViewerCommandTargetEditors()
		action, err := st.upsertCurrentViewerCommandTarget()
		if err != nil || action != "Update" {
			t.Fatalf("target update action=%q err=%v", action, err)
		}
		if len(st.viewTargetEntries) != 1 || st.viewTargetEntries[0].Key != newKey {
			t.Fatalf("target entries=%#v want renamed target", st.viewTargetEntries)
		}
	})

	t.Run("rule", func(t *testing.T) {
		st := &settingsModalState{viewRuleEntries: []fm.ViewerCommandRule{{Pattern: `\.log$`, Command: "old-command"}}}
		st.loadViewerCommandRuleFields(`\.log$`, "old-command")
		st.viewRulePatternEdit.SetText(`\.txt$`)
		st.syncViewerCommandRuleEditors()
		action, err := st.upsertCurrentViewerCommandRule()
		if err != nil || action != "Update" {
			t.Fatalf("rule update action=%q err=%v", action, err)
		}
		if len(st.viewRuleEntries) != 1 || st.viewRuleEntries[0].Pattern != `\.txt$` {
			t.Fatalf("rule entries=%#v want renamed pattern", st.viewRuleEntries)
		}
	})

	t.Run("association", func(t *testing.T) {
		st := &settingsModalState{viewAssocEntries: []fm.ViewerAssociation{{Extension: ".log", AppPath: "old-app"}}}
		st.loadViewerAssociationFields(".log", "old-app")
		st.viewAssocExtEdit.SetText("txt")
		st.syncViewerAssociationEditors()
		action, err := st.upsertCurrentViewerAssociation()
		if err != nil || action != "Update" {
			t.Fatalf("association update action=%q err=%v", action, err)
		}
		if len(st.viewAssocEntries) != 1 || st.viewAssocEntries[0].Extension != ".txt" {
			t.Fatalf("association entries=%#v want renamed extension", st.viewAssocEntries)
		}
	})
}

func TestKeyedSettingsUpdateRejectsExistingDestination(t *testing.T) {
	st := &settingsModalState{filenameAgeEntries: []fm.FilenameAgeRule{
		{MaxAge: "1d", Text: "#112233"},
		{MaxAge: "2d", Text: "#445566"},
	}}
	st.loadFilenameAgeFields("1", "d", "#112233", "", "both")
	st.filenameAgeOffsetEdit.SetText("2")

	if action, err := st.upsertCurrentFilenameAgeRule(); err == nil || action != "Update" {
		t.Fatalf("conflicting update action=%q err=%v, want Update error", action, err)
	}
	if len(st.filenameAgeEntries) != 2 || st.filenameAgeEntries[0].Text != "#112233" || st.filenameAgeEntries[1].Text != "#445566" {
		t.Fatalf("conflicting update mutated entries: %#v", st.filenameAgeEntries)
	}
}

func TestParseFilenameAgeRuleFieldsRequiresPositiveOffsetAndVisual(t *testing.T) {
	if _, err := parseFilenameAgeRuleFields("0", "d", "#112233", "", "both"); err == nil {
		t.Fatal("parseFilenameAgeRuleFields should reject zero offsets")
	}
	if _, err := parseFilenameAgeRuleFields("3", "d", "", "", "both"); err == nil {
		t.Fatal("parseFilenameAgeRuleFields should require a color or icon")
	}
}

func colorNRGBA(r, g, b, a uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: a}
}
