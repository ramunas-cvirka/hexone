// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/filesys"
	"image"
	"image/color"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
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
		{key: "viewer", want: 2},
		{key: "associations", want: 3},
		{key: "colors", want: 4},
		{key: "config", want: 5},
	}
	for _, tc := range cases {
		if got := settingsTabIndex(tc.key); got != tc.want {
			t.Fatalf("settingsTabIndex(%q)=%d want %d", tc.key, got, tc.want)
		}
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
		{key: "fonts", step: 1, want: "viewer"},
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
		settingsKeyboardFocusFilenameDefaultTextPicker,
		settingsKeyboardFocusFilenameDefaultText,
		settingsKeyboardFocusFilenameDefaultIconPicker,
		settingsKeyboardFocusFilenameRuleMode,
		settingsKeyboardFocusFilenamePermMask,
		settingsKeyboardFocusFilenamePermPicker,
		settingsKeyboardFocusFilenamePermMatch,
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
		settingsKeyboardFocusGeneralPaneFont,
		settingsKeyboardFocusGeneralPaneFontSize,
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

func TestSettingsKeyboardFocusOrderIncludesViewerControls(t *testing.T) {
	st := &settingsModalState{activeTab: "viewer"}

	got := st.focusOrder()
	want := []settingsKeyboardFocus{
		settingsKeyboardFocusNav,
		settingsKeyboardFocusViewerShell,
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
	st := &settingsModalState{activeTab: "colors", colorScope: "panes"}

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
	st.activeTab = "viewer"
	st.focus = settingsKeyboardFocusViewerShell
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
	if st.activeTab != "viewer" {
		t.Fatalf("activeTab after DownArrow = %q, want viewer", st.activeTab)
	}

	router.Queue(key.Event{Name: key.NameRightArrow, State: key.Press})
	frame(now.Add(2 * time.Millisecond))
	if got := st.viewShellEdit.Text(); got != "auto" {
		t.Fatalf("viewer shell after RightArrow = %q, want auto", got)
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

	if st.focus != settingsKeyboardFocusGeneralPaneFontSize {
		t.Fatalf("focus after tabbing = %v, want pane font size stepper", st.focus)
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

	if st.colorPickerOpen {
		t.Fatal("Enter on the focused color swatch should close the color picker")
	}
	if got := strings.TrimSpace(st.colorValueEdit.Text()); got != wantHex {
		t.Fatalf("background color after keyboard selection = %q, want %q", got, wantHex)
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
	st.focus = settingsKeyboardFocusFilenameDefaultIconPicker
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

	if !st.filenameIconPickerOpen || st.filenameIconPickerTarget != "filename-default-icon" {
		t.Fatal("Enter on the default icon picker should open the icon picker")
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
	if got := st.filenameDefaultIcon; got != wantIcon {
		t.Fatalf("default filename icon after keyboard selection = %q, want %q", got, wantIcon)
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
	if mid <= 0 || mid >= 3 {
		t.Fatalf("mid position=%v want between 0 and 3", mid)
	}

	end, anim := st.tabPosition(now.Add(toolbarAnimDur))
	if anim {
		t.Fatal("tabPosition should stop animating at the end of the transition")
	}
	if end != 3 {
		t.Fatalf("end position=%v want 3", end)
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

func TestSettingsColorSwatchGroupsIncludeNearbyCurrentColor(t *testing.T) {
	groups := settingsColorSwatchGroups("#2D9AA5")
	if len(groups) == 0 {
		t.Fatal("expected swatch groups")
	}
	if groups[0].label != "Nearby" {
		t.Fatalf("first group label=%q want Nearby", groups[0].label)
	}
	want := fm.NormalizeHexColor("#2D9AA5", "")
	found := false
	for _, hex := range groups[0].hexes {
		if fm.NormalizeHexColor(hex, "") == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("nearby swatches do not include current color %q: %#v", want, groups[0].hexes)
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

func TestSettingsViewerPreviewSelectionFillIsOpaque(t *testing.T) {
	fill := settingsViewerPreviewSelectionFill(fileViewerTheme{
		Selection:       colorNRGBA(0x11, 0x22, 0x33, 0x80),
		StrongSelection: colorNRGBA(0x44, 0x55, 0x66, 0x99),
	}, false)
	if fill.A != 0xFF {
		t.Fatalf("selection alpha=%d want 255", fill.A)
	}

	strong := settingsViewerPreviewSelectionFill(fileViewerTheme{
		Selection:       colorNRGBA(0x11, 0x22, 0x33, 0x80),
		StrongSelection: colorNRGBA(0x44, 0x55, 0x66, 0x99),
	}, true)
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
	wantH := st.previewViewerContentHeight(ui, th, gtx)
	got := ui.layoutSettingsViewerPreviewContent(th, gtx, st, theme, ui)

	if got.Size.Y != wantH {
		t.Fatalf("preview content height=%d want %d", got.Size.Y, wantH)
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
		filenameDefaultText: "#AABBCC",
		filenameDefaultIcon: fm.FilenameIconDocument,
		filenameAgeEntries: []fm.FilenameAgeRule{
			{MaxAge: "24h", Text: "#112233", Icon: fm.FilenameIconRecent},
			{MaxAge: "1w", Text: "", Icon: ""},
			{MaxAge: "3d", Text: "#334455", Icon: ""},
		},
		filenamePermEntries: []fm.FilenamePermissionRule{
			{Permissions: "111", Match: "any", Text: "#556677", Icon: fm.FilenameIconLocked},
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
	if got.Text != "#AABBCC" {
		t.Fatalf("default text=%q want %q", got.Text, "#AABBCC")
	}
	if got.Icon != fm.FilenameIconDocument {
		t.Fatalf("default icon=%q want %q", got.Icon, fm.FilenameIconDocument)
	}
	if len(got.AgeRules) != 2 {
		t.Fatalf("len(AgeRules)=%d want 2", len(got.AgeRules))
	}
	if got.AgeRules[0].MaxAge != "1d" || got.AgeRules[1].MaxAge != "3d" {
		t.Fatalf("age rules=%#v want normalized 1d and 3d", got.AgeRules)
	}
	if len(got.PermissionRules) != 1 || got.PermissionRules[0].Permissions != "0111" {
		t.Fatalf("permission rules=%#v want normalized 0111", got.PermissionRules)
	}
	if got.PermissionRules[0].Match != fm.FilenamePermissionMatchAny {
		t.Fatalf("permission match=%q want %q", got.PermissionRules[0].Match, fm.FilenamePermissionMatchAny)
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

func TestLoadFilenameColorsFromConfigKeepsPaneTextInheritedUntilEdited(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Colors.FilePaneText = "#13579B"
	cfg.Colors.Filenames.Icon = fm.FilenameIconCode

	st := &settingsModalState{}
	st.loadFilenameColorsFromConfig(cfg)

	if st.filenameDefaultText != "" {
		t.Fatalf("filenameDefaultText=%q want empty inherited override", st.filenameDefaultText)
	}
	if got := st.filenameDefaultTextEdit.Text(); got != "#13579B" {
		t.Fatalf("filenameDefaultTextEdit=%q want pane text color", got)
	}
	if st.filenameDefaultIcon != fm.FilenameIconCode {
		t.Fatalf("filenameDefaultIcon=%q want %q", st.filenameDefaultIcon, fm.FilenameIconCode)
	}

	got, errText := st.draftFilenameColors()
	if errText != "" {
		t.Fatalf("unexpected draft filename error: %q", errText)
	}
	if got.Text != "" {
		t.Fatalf("draft filename text=%q want empty inherited override", got.Text)
	}
	if got.Icon != fm.FilenameIconCode {
		t.Fatalf("draft filename icon=%q want %q", got.Icon, fm.FilenameIconCode)
	}
}

func TestFilenameIconPickerValueMapsTargets(t *testing.T) {
	st := &settingsModalState{}

	tests := []struct {
		target string
		icon   string
	}{
		{target: "filename-default-icon", icon: fm.FilenameIconBook},
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
	st.loadFilenameExtensionFields(".tar.gz", "", fm.FilenameIconArchive)

	if got := st.filenameExtEdit.Text(); got != "tar.gz" {
		t.Fatalf("filenameExtEdit=%q want bare suffix", got)
	}

	rule, err := parseFilenameExtensionRuleFields("go", "", fm.FilenameIconCode)
	if err != nil {
		t.Fatalf("parseFilenameExtensionRuleFields error: %v", err)
	}
	if rule.Extension != ".go" {
		t.Fatalf("rule.Extension=%q want %q", rule.Extension, ".go")
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

func TestParseFilenameAgeRuleFieldsRequiresPositiveOffsetAndVisual(t *testing.T) {
	if _, err := parseFilenameAgeRuleFields("0", "d", "#112233", ""); err == nil {
		t.Fatal("parseFilenameAgeRuleFields should reject zero offsets")
	}
	if _, err := parseFilenameAgeRuleFields("3", "d", "", ""); err == nil {
		t.Fatal("parseFilenameAgeRuleFields should require a color or icon")
	}
}

func colorNRGBA(r, g, b, a uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: a}
}
