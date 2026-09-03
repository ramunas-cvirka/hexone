// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"slices"
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
	"gioui.org/widget"
	"gioui.org/widget/material"

	"hexone/fm"
)

// TestSettingsStatusBarFieldRowsCoverEveryField extends the guards in
// filepane_status_fields_test.go and settings_modal_test.go to the last
// hand-maintained list keyed by the enum. A field missing a row has no checkbox
// anywhere in the UI, so nothing but hand-editing the YAML can ever turn it on —
// and since focusOrder builds the tab's keyboard order from these rows, the
// field would be unreachable by keyboard too.
func TestSettingsStatusBarFieldRowsCoverEveryField(t *testing.T) {
	rows := settingsStatusBarFieldRows()

	seen := make(map[filePaneStatusField]bool, len(rows))
	labels := make(map[string]bool, len(rows))
	for _, row := range rows {
		if seen[row.field] {
			t.Fatalf("settingsStatusBarFieldRows lists field %d twice", row.field)
		}
		seen[row.field] = true
		if strings.TrimSpace(row.label) == "" {
			t.Fatalf("field %d has an empty checkbox label", row.field)
		}
		if labels[row.label] {
			t.Fatalf("two field checkboxes share the label %q", row.label)
		}
		labels[row.label] = true
		if int(row.field) >= statusBarFieldCount {
			t.Fatalf("field %d is out of range for the %d-slot checkbox array", row.field, statusBarFieldCount)
		}
	}

	for _, field := range allFilePaneStatusFields {
		if !seen[field] {
			t.Fatalf("filePaneStatusField %d has no row in settingsStatusBarFieldRows, so no checkbox can ever enable it", field)
		}
	}
	if len(rows) != len(allFilePaneStatusFields) {
		t.Fatalf("settingsStatusBarFieldRows has %d rows but there are %d field constants", len(rows), len(allFilePaneStatusFields))
	}

	// The rows are the tab's display order and the bar renders in enum order;
	// a mismatch would make the checkbox list disagree with the preview under
	// it for no reason a reader could see.
	for i, row := range rows {
		if int(row.field) != i {
			t.Fatalf("row %d is field %d; the rows must follow enum order, which is the bar's render order", i, row.field)
		}
	}
}

func TestNormalizeSettingsPaneModeAcceptsStatusBar(t *testing.T) {
	for _, raw := range []string{"statusbar", "StatusBar", "  STATUSBAR "} {
		if got := normalizeSettingsPaneMode(raw); got != "statusbar" {
			t.Fatalf("normalizeSettingsPaneMode(%q) = %q, want %q", raw, got, "statusbar")
		}
	}
	if got := normalizeSettingsPaneMode("status bar"); got != "full" {
		t.Fatalf("normalizeSettingsPaneMode(%q) = %q, want the full fallback", "status bar", got)
	}
}

// TestSettingsPaneModeKeysMatchTheTabStrip pins the list the sliding indicator
// and the arrow stepper share against the tab buttons actually rendered. They
// were separate literals before the status bar tab; a strip whose order differs
// from the indicator's puts the highlight on the wrong tab.
func TestSettingsPaneModeKeysMatchTheTabStrip(t *testing.T) {
	want := []string{"full", "brief", "statusbar", "other"}
	if !slices.Equal(settingsPaneModeKeys, want) {
		t.Fatalf("settingsPaneModeKeys = %v, want %v", settingsPaneModeKeys, want)
	}
	for _, key := range settingsPaneModeKeys {
		if got := normalizeSettingsPaneMode(key); got != key {
			t.Fatalf("tab key %q normalises to %q, so the tab could never stay selected", key, got)
		}
	}
}

// TestStepPaneSettingsModeReachesStatusBar covers Left/Right on the tab strip.
// The stepper had its own copy of the tab list, so the new tab was one more
// place to forget.
func TestStepPaneSettingsModeReachesStatusBar(t *testing.T) {
	st := &settingsModalState{paneSettingsMode: "brief"}
	if !st.stepPaneSettingsMode(1, time.Now()) {
		t.Fatal("right arrow from brief should step")
	}
	if got := normalizeSettingsPaneMode(st.paneSettingsMode); got != "statusbar" {
		t.Fatalf("right arrow from brief landed on %q, want statusbar", got)
	}
	if !st.stepPaneSettingsMode(1, time.Now()) {
		t.Fatal("right arrow from statusbar should step")
	}
	if got := normalizeSettingsPaneMode(st.paneSettingsMode); got != "other" {
		t.Fatalf("right arrow from statusbar landed on %q, want other", got)
	}
	if !st.stepPaneSettingsMode(-1, time.Now()) {
		t.Fatal("left arrow from other should step")
	}
	if got := normalizeSettingsPaneMode(st.paneSettingsMode); got != "statusbar" {
		t.Fatalf("left arrow from other landed on %q, want statusbar", got)
	}
}

func TestSettingsStatusBarTabFocusOrder(t *testing.T) {
	st := &settingsModalState{activeTab: "general", paneSettingsMode: "statusbar"}
	st.statusBarEnabledBool.Value = true

	got := st.focusOrder()
	want := []settingsKeyboardFocus{
		settingsKeyboardFocusNav,
		settingsKeyboardFocusFilePaneMode,
		settingsKeyboardFocusStatusBarEnabled,
		settingsKeyboardFocusStatusBarHideInFull,
	}
	for _, row := range settingsStatusBarFieldRows() {
		want = append(want, settingsKeyboardFocusStatusBarField(row.field))
	}
	want = append(want, settingsKeyboardFocusStatusBarDateFormat, settingsKeyboardFocusFooter)
	if !slices.Equal(got, want) {
		t.Fatalf("focusOrder()=%v want %v", got, want)
	}

	// Every field checkbox must be Tab-reachable, or the tab's whole point is
	// mouse-only — and so must the date-layout picker.
	for _, field := range allFilePaneStatusFields {
		if !slices.Contains(got, settingsKeyboardFocusStatusBarField(field)) {
			t.Fatalf("field %d has no focus target in %v", field, got)
		}
	}
	if !slices.Contains(got, settingsKeyboardFocusStatusBarDateFormat) {
		t.Fatalf("the date-layout picker has no focus target in %v", got)
	}
}

// TestSettingsStatusBarTabFocusOrderSkipsDisabledControls pins the greying and
// the keyboard order to the same rule. The controls below the master switch are
// laid out with gtx.Disabled() when the bar is off, and toggleFocusedCheckbox
// writes the widget.Bool directly — it never sees a disabled context — so a
// disabled control left in the focus order would ignore the mouse but still
// answer Space.
func TestSettingsStatusBarTabFocusOrderSkipsDisabledControls(t *testing.T) {
	st := &settingsModalState{activeTab: "general", paneSettingsMode: "statusbar"}
	st.statusBarEnabledBool.Value = false

	got := st.focusOrder()
	want := []settingsKeyboardFocus{
		settingsKeyboardFocusNav,
		settingsKeyboardFocusFilePaneMode,
		settingsKeyboardFocusStatusBarEnabled,
		settingsKeyboardFocusFooter,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("focusOrder() with the bar off = %v, want %v", got, want)
	}
}

// TestSettingsStatusBarFieldFocusRangeIsDistinct guards the derived focus
// constants. They live past the end of the hand-written iota block, so an
// overlap with a real constant would silently alias two controls.
func TestSettingsStatusBarFieldFocusRangeIsDistinct(t *testing.T) {
	for _, field := range allFilePaneStatusFields {
		focus := settingsKeyboardFocusStatusBarField(field)
		if focus <= settingsKeyboardFocusFooter {
			t.Fatalf("field %d maps to %d, which collides with the named focus constants (last is %d)", field, focus, settingsKeyboardFocusFooter)
		}
		got, ok := settingsStatusBarFieldForFocus(focus)
		if !ok || got != field {
			t.Fatalf("settingsStatusBarFieldForFocus(%d) = %d, %t; want %d, true", focus, got, ok, field)
		}
		if !(&settingsModalState{}).isWidgetFocusTarget(focus) {
			t.Fatalf("field %d focus target is not a widget focus target, so Tab would never hand it real focus", field)
		}
	}

	for _, focus := range []settingsKeyboardFocus{
		settingsKeyboardFocusNone,
		settingsKeyboardFocusNav,
		settingsKeyboardFocusStatusBarEnabled,
		settingsKeyboardFocusStatusBarHideInFull,
		settingsKeyboardFocusFooter,
		settingsKeyboardFocusStatusBarField0 + statusBarFieldCount,
	} {
		if field, ok := settingsStatusBarFieldForFocus(focus); ok {
			t.Fatalf("focus %d was decoded as status bar field %d", focus, field)
		}
	}
}

func TestSettingsStatusBarToggleFocusedCheckbox(t *testing.T) {
	st := &settingsModalState{activeTab: "general", paneSettingsMode: "statusbar"}

	st.focus = settingsKeyboardFocusStatusBarEnabled
	if !st.toggleFocusedCheckbox() || !st.statusBarEnabledBool.Value {
		t.Fatal("Space on the enabled checkbox should switch the bar on")
	}
	st.focus = settingsKeyboardFocusStatusBarHideInFull
	if !st.toggleFocusedCheckbox() || !st.statusBarHideInFullBool.Value {
		t.Fatal("Space on hide-in-full should tick it")
	}
	for _, field := range allFilePaneStatusFields {
		st.focus = settingsKeyboardFocusStatusBarField(field)
		before := st.statusBarFieldBools[field].Value
		if !st.toggleFocusedCheckbox() {
			t.Fatalf("Space on field %d reported no toggle", field)
		}
		if st.statusBarFieldBools[field].Value == before {
			t.Fatalf("Space on field %d did not change its checkbox", field)
		}
	}
}

// statusBarPreviewUnscaledWidth is the frame width the preview assertions
// render at. It is wider than any width settingsStatusBarPreviewPaneWidth can
// ask for — the widest field set with the widest date layout measures about
// 832px at PxPerDp 1 — so layoutSettingsStatusBarPreviewPane takes its unscaled
// path and every semantic label sits exactly where the live plan put it.
const statusBarPreviewUnscaledWidth = 1000

// statusBarPreviewRealFrameWidth is the width the preview frame actually gets
// inside the settings modal, whose content column is fixed: the mock is scaled
// into this, and TestSettingsStatusBarPreviewKeepsEveryFieldAtTheRealFrameWidth
// is the test that says the scaling is what keeps the checkboxes alive there.
const statusBarPreviewRealFrameWidth = 558

// statusBarPreviewContext builds the context the preview assertions run in.
// PxPerDp 1 makes dp and px the same number here.
func statusBarPreviewContext(router *input.Router) layout.Context {
	return layout.Context{
		Ops:         new(op.Ops),
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(statusBarPreviewUnscaledWidth, 720)},
		Now:         time.Now(),
	}
}

// settingsStatusBarPreviewNodes lays the whole preview out — frame, sample grid
// and strip — and returns the semantic tree it drew. The preview renders
// through the live bar's layout path, so a joined string no longer exists to
// assert on; this is the same technique the live bar's own tests use.
func settingsStatusBarPreviewNodes(t *testing.T, ui *UI, th *material.Theme, st *settingsModalState, gtx layout.Context) []input.SemanticNode {
	t.Helper()
	router := new(input.Router)
	gtx.Ops = new(op.Ops)
	gtx.Source = router.Source()
	ui.layoutSettingsStatusBarPreview(th, gtx, st)
	router.Frame(gtx.Ops)
	return router.AppendSemantics(nil)
}

func settingsStatusBarPreviewLabels(t *testing.T, ui *UI, th *material.Theme, st *settingsModalState, gtx layout.Context) []statusBarSemanticLabel {
	t.Helper()
	var out []statusBarSemanticLabel
	for _, node := range settingsStatusBarPreviewNodes(t, ui, th, st, gtx) {
		if node.Desc.Label != "" {
			out = append(out, statusBarSemanticLabel{text: node.Desc.Label, bounds: node.Desc.Bounds})
		}
	}
	return out
}

func settingsStatusBarPreviewLabelTexts(labels []statusBarSemanticLabel) []string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		out = append(out, l.text)
	}
	return out
}

// settingsStatusBarPreviewStripTop is the y the mock's strip starts at inside
// the preview frame: the frame's fixed height less the live strip's own height
// (one line of its text style plus the 4dp insets above and below). Everything
// below it belongs to the strip; everything above it is the sample grid.
func settingsStatusBarPreviewStripTop(ui *UI, th *material.Theme, gtx layout.Context) int {
	probe := material.Body2(th, "0")
	probe.Font.Typeface = ui.mainTypeface()
	probe.TextSize = scaleThemeFontSize(th, 11)
	probe.MaxLines = 1
	probe.Truncator = ""
	stripH := measureLabelUnconstrained(gtx, probe).Size.Y + 2*gtx.Dp(unit.Dp(4))
	return gtx.Dp(settingsPanePreviewFrameHeightDp) - stripH
}

func settingsStatusBarPreviewStripLabels(ui *UI, th *material.Theme, gtx layout.Context, labels []statusBarSemanticLabel) []statusBarSemanticLabel {
	top := settingsStatusBarPreviewStripTop(ui, th, gtx)
	out := make([]statusBarSemanticLabel, 0, len(labels))
	for _, l := range labels {
		if l.bounds.Min.Y >= top {
			out = append(out, l)
		}
	}
	return out
}

// settingsStatusBarPreviewFieldValues maps every field checkbox to the exact
// column text the preview's fixed sample renders for it. Distinct values by
// construction, so a preview that cannot tell two checkboxes apart fails the
// map lookup rather than a fuzzy comparison.
func settingsStatusBarPreviewFieldValues() map[filePaneStatusField]string {
	return map[filePaneStatusField]string{
		filePaneStatusFieldSize:  "2.40 MB",
		filePaneStatusFieldDate:  settingsPanePreviewTime.Format("Jan 02 2006 15:04"),
		filePaneStatusFieldPerms: "-rw-r--r--",
		filePaneStatusFieldOwner: "demo:staff",
		filePaneStatusFieldFree:  formatFilePaneStatusFree(settingsStatusBarPreviewFreeBytes, settingsStatusBarPreviewTotalBytes),
	}
}

// TestSettingsStatusBarPreviewGridFitsTheSample pins the two hand-maintained
// numbers against each other: the grid draws exactly
// settingsStatusBarPreviewGridColumns * settingsStatusBarPreviewGridRows cells
// by index, so a sample listing shorter than that panics on the first settings
// open, and a longer one silently hides entries the strip's column widths were
// measured from.
func TestSettingsStatusBarPreviewGridFitsTheSample(t *testing.T) {
	cells := settingsStatusBarPreviewGridColumns * settingsStatusBarPreviewGridRows
	if got := len(settingsStatusBarPreviewEntries); got != cells {
		t.Fatalf("the sample listing has %d entries but the grid draws %d cells", got, cells)
	}
	if settingsStatusBarPreviewCursor < 0 || settingsStatusBarPreviewCursor >= cells {
		t.Fatalf("the cursor index %d is outside the grid", settingsStatusBarPreviewCursor)
	}
	pane := settingsStatusBarPreviewPane(fm.DefaultConfig())
	if pane.table.Selected != settingsStatusBarPreviewCursor {
		t.Fatalf("the sample pane's cursor is row %d, the grid highlights row %d", pane.table.Selected, settingsStatusBarPreviewCursor)
	}
	if pane.selectedEntry() == nil {
		t.Fatal("the preview sample pane has no cursor entry, so every per-entry field renders empty")
	}
	if pane.hasMarkedRows() {
		t.Fatal("the preview sample pane has marked rows, so the strip would render the marked-mode summary instead of the cursor line the preview is meant to show")
	}
}

// TestSettingsStatusBarPreviewStripDescribesTheHighlightedRow is the coherence
// the rework exists for: the mock highlights one row and the strip below
// describes THAT row. The old preview's strip talked about report.pdf over a
// frame that showed no files at all.
//
// The highlighted cell tags itself semantic.SelectedOp(true), so the test finds
// it without reimplementing the grid's arithmetic; the strip's name is the one
// label anchored on the left inset inside the strip's own band.
func TestSettingsStatusBarPreviewStripDescribesTheHighlightedRow(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	th := material.NewTheme()
	gtx := statusBarPreviewContext(new(input.Router))
	nodes := settingsStatusBarPreviewNodes(t, ui, th, st, gtx)

	var selected image.Rectangle
	found := false
	for _, node := range nodes {
		if node.Desc.Selected {
			if found {
				t.Fatalf("two cells are marked selected (%v and %v); the mock must have one cursor row", selected, node.Desc.Bounds)
			}
			selected, found = node.Desc.Bounds, true
		}
	}
	if !found {
		t.Fatal("no cell in the sample grid is marked selected; the mock draws no cursor row for the strip to describe")
	}

	labels := make([]statusBarSemanticLabel, 0, len(nodes))
	for _, node := range nodes {
		if node.Desc.Label != "" {
			labels = append(labels, statusBarSemanticLabel{text: node.Desc.Label, bounds: node.Desc.Bounds})
		}
	}

	// The strip's name column: the only label starting exactly on the left inset
	// inside the strip's band (the grid's own cells inset their text by
	// fm.ColumnPadDp, which is narrower).
	strip := settingsStatusBarPreviewStripLabels(ui, th, gtx, labels)
	inset := gtx.Dp(filePaneStatusBarInsetX)
	stripName := ""
	for _, l := range strip {
		if l.bounds.Min.X == inset {
			if stripName != "" {
				t.Fatalf("two strip labels start on the left inset (%q and %q)", stripName, l.text)
			}
			stripName = l.text
		}
	}
	if stripName == "" {
		t.Fatalf("the strip has no label on its left inset; labels: %v", settingsStatusBarPreviewLabelTexts(strip))
	}

	highlighted := make([]string, 0, 2)
	for _, l := range labels {
		if l.bounds.In(selected) {
			highlighted = append(highlighted, l.text)
		}
	}
	if !slices.Contains(highlighted, stripName) {
		t.Fatalf("the strip describes %q but the highlighted cell renders %v; the mock and its status bar disagree about which row the cursor is on",
			stripName, highlighted)
	}
	if want := settingsStatusBarPreviewEntries[settingsStatusBarPreviewCursor].Name; stripName != want {
		t.Fatalf("the strip's name column reads %q, want the cursor entry %q", stripName, want)
	}
}

// TestSettingsStatusBarPreviewReflectsEveryFieldCheckbox is the point of having
// a preview at all: it renders through buildFilePaneStatusBarPlan and
// layoutFilePaneStatusBarInfoRow — the layout path the live pane uses — and a
// field whose column the plan drops or whose value renders empty is a checkbox
// that looks broken. The sample pane therefore has to carry a value for every
// field, and the mock has to be laid out at a width where every column survives.
func TestSettingsStatusBarPreviewReflectsEveryFieldCheckbox(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	// Pinned so the permissions value below is deterministic across platforms;
	// "auto" resolves per-OS.
	st.panePermissionFormat = "symbolic"
	th := material.NewTheme()
	gtx := statusBarPreviewContext(new(input.Router))
	values := settingsStatusBarPreviewFieldValues()
	clear := func() {
		for i := range st.statusBarFieldBools {
			st.statusBarFieldBools[i].Value = false
		}
	}

	// One field at a time: its column value must appear, and no other field's.
	for _, row := range settingsStatusBarFieldRows() {
		clear()
		st.statusBarFieldBools[row.field].Value = true
		labels := settingsStatusBarPreviewLabels(t, ui, th, st, gtx)
		if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == values[row.field] }); !ok {
			t.Fatalf("%q ticked on its own never renders its value %q; the checkbox looks broken (labels: %v)",
				row.label, values[row.field], settingsStatusBarPreviewLabelTexts(labels))
		}
		for other, value := range values {
			if other == row.field {
				continue
			}
			if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == value }); ok {
				t.Fatalf("%q ticked alone also rendered field %d's value %q", row.label, other, value)
			}
		}
	}

	// All ticked: every value present at once — nothing dropped for room,
	// because the mock is laid out at settingsStatusBarPreviewPaneWidth.
	for i := range st.statusBarFieldBools {
		st.statusBarFieldBools[i].Value = true
	}
	labels := settingsStatusBarPreviewLabels(t, ui, th, st, gtx)
	for field, value := range values {
		if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == value }); !ok {
			t.Fatalf("the all-fields preview is missing field %d's value %q (labels: %v)",
				field, value, settingsStatusBarPreviewLabelTexts(labels))
		}
	}
}

// TestSettingsStatusBarPreviewAnchorsBothRegions pins the preview to Revision
// 2's geometry the way TestStatusBarAnchorsBothRegions pins the live strip: the
// name column's text begins at the left inset, the free-space text ends at the
// right inset, and the "│" region separator ends where the free text begins.
// If the preview forked its own layout instead of reusing the live row, this is
// the test that notices the geometry drifting.
func TestSettingsStatusBarPreviewAnchorsBothRegions(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	th := material.NewTheme()
	gtx := statusBarPreviewContext(new(input.Router))
	all := settingsStatusBarPreviewLabels(t, ui, th, st, gtx) // defaults: size, date, free
	labels := settingsStatusBarPreviewStripLabels(ui, th, gtx, all)

	cursorName := settingsStatusBarPreviewEntries[settingsStatusBarPreviewCursor].Name
	name, ok := findStatusBarLabel(labels, func(s string) bool { return s == cursorName })
	if !ok {
		t.Fatalf("no strip name label among %v", settingsStatusBarPreviewLabelTexts(labels))
	}
	inset := 8 // filePaneStatusBarInsetX at PxPerDp 1
	if name.bounds.Min.X != inset {
		t.Errorf("name label starts at x=%d, want the left inset at %d", name.bounds.Min.X, inset)
	}

	free, ok := findStatusBarLabel(labels, func(s string) bool {
		return s == formatFilePaneStatusFree(settingsStatusBarPreviewFreeBytes, settingsStatusBarPreviewTotalBytes)
	})
	if !ok {
		t.Fatalf("no free-region label among %v", settingsStatusBarPreviewLabelTexts(labels))
	}
	width := ui.measureFilePaneStatusBarTextWidth(th, gtx, free.text)
	if got, want := free.bounds.Min.X+width, statusBarPreviewUnscaledWidth-inset; got != want {
		t.Errorf("free region spans x=[%d,%d), want its right edge on the inset at %d", free.bounds.Min.X, got, want)
	}

	rule, ok := findStatusBarLabel(labels, func(s string) bool { return s == "│" })
	if !ok {
		t.Fatalf("no region separator among %v", settingsStatusBarPreviewLabelTexts(labels))
	}
	if got := rule.bounds.Min.X + ui.measureFilePaneStatusBarTextWidth(th, gtx, filePaneStatusRegionSeparator); got != free.bounds.Min.X {
		t.Errorf("region separator spans x=[%d,%d), want it to end where the free text begins at %d", rule.bounds.Min.X, got, free.bounds.Min.X)
	}
}

// TestSettingsStatusBarPreviewShowsOneCursorStrip: the marked-mode summary is
// automatic and unconfigurable, so previewing it would show the user nothing
// they can change. One strip, and it is the per-entry cursor line.
func TestSettingsStatusBarPreviewShowsOneCursorStrip(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	th := material.NewTheme()
	gtx := statusBarPreviewContext(new(input.Router))
	labels := settingsStatusBarPreviewLabels(t, ui, th, st, gtx)
	texts := settingsStatusBarPreviewLabelTexts(labels)

	freeText := formatFilePaneStatusFree(settingsStatusBarPreviewFreeBytes, settingsStatusBarPreviewTotalBytes)
	count := 0
	for _, l := range labels {
		if l.text == freeText {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("the free-space region renders %d times, want once — the mock has a single status bar (labels: %v)", count, texts)
	}
	for _, gone := range []string{"CURSOR", "MARKED SELECTION"} {
		if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == gone }); ok {
			t.Fatalf("the caption %q is still rendered; the mock is a picture of a pane, not a captioned diagram (labels: %v)", gone, texts)
		}
	}
	for _, l := range labels {
		if strings.Contains(l.text, "item selected") || strings.Contains(l.text, "items selected") {
			t.Fatalf("the preview renders the marked-mode summary %q, which no checkbox on this tab can change", l.text)
		}
	}
}

// TestSettingsStatusBarPreviewFallsBackToDefaultsWhenNothingTicked documents a
// deliberately odd-looking case. With no field ticked, the preview runs the
// draft through fm.NormalizeStatusBarFields, which restores the default field
// set for an empty list — so the strip shows size/date/free rather than going
// blank. That matches what saving actually writes: saveSettingsModal stores the
// same normalised defaults and switches Enabled off instead, which is why
// TestStatusBarUncheckingEveryFieldDisablesTheBar passes.
func TestSettingsStatusBarPreviewFallsBackToDefaultsWhenNothingTicked(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	th := material.NewTheme()
	gtx := statusBarPreviewContext(new(input.Router))

	for i := range st.statusBarFieldBools {
		st.statusBarFieldBools[i].Value = false
	}
	got := settingsStatusBarPreviewLabelTexts(settingsStatusBarPreviewLabels(t, ui, th, st, gtx))

	defaults := filePaneStatusFields(fm.NormalizeStatusBarFields(nil))
	for i := range st.statusBarFieldBools {
		st.statusBarFieldBools[i].Value = slices.Contains(defaults, filePaneStatusField(i))
	}
	want := settingsStatusBarPreviewLabelTexts(settingsStatusBarPreviewLabels(t, ui, th, st, gtx))
	if !slices.Equal(got, want) {
		t.Fatalf("no fields ticked renders %v, want the default set's %v", got, want)
	}
}

// TestSettingsStatusBarPreviewUsesTheDraftNotTheSavedConfig makes sure the
// preview reads the modal's checkboxes rather than ui.fmCfg, which still holds
// the last saved state while the user is editing.
func TestSettingsStatusBarPreviewUsesTheDraftNotTheSavedConfig(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.StatusBar.Fields = []string{fm.StatusBarFieldSize}
	ui := NewUI(cfg)
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	th := material.NewTheme()
	gtx := statusBarPreviewContext(new(input.Router))

	for i := range st.statusBarFieldBools {
		st.statusBarFieldBools[i].Value = false
	}
	st.statusBarFieldBools[filePaneStatusFieldOwner].Value = true
	labels := settingsStatusBarPreviewLabels(t, ui, th, st, gtx)

	if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == "demo:staff" }); !ok {
		t.Fatalf("owner-only draft renders %v, want the sample owner", settingsStatusBarPreviewLabelTexts(labels))
	}
	if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == "2.40 MB" }); ok {
		t.Fatalf("the preview still renders the saved config's size column: %v", settingsStatusBarPreviewLabelTexts(labels))
	}
}

// TestSettingsStatusBarPreviewKeepsEveryFieldAtTheRealFrameWidth pins the
// decision behind settingsStatusBarPreviewPaneWidth. The mock is drawn into the
// settings modal's fixed content column — 558px, which leaves the strip 542 —
// and the all-fields row measures about 816 there, so laying the strip out at
// the frame's own width would make buildFilePaneStatusBarPlan drop Permissions,
// Owner and Free space: three checkboxes that then change nothing visible. The
// mock is therefore laid out at the width the configuration needs and scaled
// into the frame, and every field's label survives that.
//
// Without this, "fixing" the mock back to gtx.Constraints.Max.X would leave the
// rest of the file green, because statusBarPreviewContext is already wide
// enough for everything.
func TestSettingsStatusBarPreviewKeepsEveryFieldAtTheRealFrameWidth(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.panePermissionFormat = "symbolic"
	th := material.NewTheme()
	for i := range st.statusBarFieldBools {
		st.statusBarFieldBools[i].Value = true
	}

	narrow := statusBarPreviewContext(new(input.Router))
	narrow.Constraints.Max.X = statusBarPreviewRealFrameWidth
	labels := settingsStatusBarPreviewLabels(t, ui, th, st, narrow)
	for field, value := range settingsStatusBarPreviewFieldValues() {
		if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == value }); !ok {
			t.Fatalf("field %d's value %q vanished in the modal's own %dpx frame; the mock is laying the strip out at the frame width instead of settingsStatusBarPreviewPaneWidth, so the real settings dialog grows dead checkboxes (labels: %v)",
				field, value, statusBarPreviewRealFrameWidth, settingsStatusBarPreviewLabelTexts(labels))
		}
	}

	router := new(input.Router)
	gtx := statusBarPreviewContext(router)
	gtx.Constraints.Max.X = statusBarPreviewRealFrameWidth
	dims := ui.layoutSettingsStatusBarPreview(th, gtx, st)
	if dims.Size.X != statusBarPreviewRealFrameWidth {
		t.Fatalf("the preview reports %dpx wide in a %dpx frame; the scaled mock must fill exactly the frame", dims.Size.X, statusBarPreviewRealFrameWidth)
	}
	if want := gtx.Dp(settingsPanePreviewFrameHeightDp); dims.Size.Y != want {
		t.Fatalf("the preview reports %dpx tall, want the shared frame's %dpx", dims.Size.Y, want)
	}
}

// TestSettingsStatusBarPreviewFollowsTheDraftPermissionFormat pins the preview
// to the edits in progress rather than to the config last saved, the way the
// Full mode preview beside it already is. Switching the Full tab's permission
// picker to octal has to change this preview in the same frame; reading
// ui.fmCfg left it showing -rw-r--r-- until the user pressed Save.
func TestSettingsStatusBarPreviewFollowsTheDraftPermissionFormat(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Columns.PermissionFormat = "symbolic"
	ui := NewUI(cfg)
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	th := material.NewTheme()
	gtx := statusBarPreviewContext(new(input.Router))

	for i := range st.statusBarFieldBools {
		st.statusBarFieldBools[i].Value = false
	}
	st.statusBarFieldBools[filePaneStatusFieldPerms].Value = true

	labels := settingsStatusBarPreviewLabels(t, ui, th, st, gtx)
	if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == "-rw-r--r--" }); !ok {
		t.Fatalf("permissions preview renders %v, want the sample's symbolic form", settingsStatusBarPreviewLabelTexts(labels))
	}
	st.panePermissionFormat = "octal"
	labels = settingsStatusBarPreviewLabels(t, ui, th, st, gtx)
	if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == "0644" }); !ok {
		t.Fatalf("permissions preview renders %v after switching the draft to octal, want %q; the preview is reading the saved config rather than the draft", settingsStatusBarPreviewLabelTexts(labels), "0644")
	}
	// The draft is a copy: building the preview must not write through to the
	// config the rest of the app is still running on.
	if ui.fmCfg.Columns.PermissionFormat != "symbolic" {
		t.Fatalf("rendering the preview rewrote the live config's permission format to %q", ui.fmCfg.Columns.PermissionFormat)
	}
}

// TestSettingsStatusBarPreviewFollowsTheDateFormatPicker is the picker half of
// the draft rule: switching status_bar.date_format must change the date
// column's text (and only its text) in the same frame, through the same
// fm.StatusBarDateLayout the live bar renders with.
func TestSettingsStatusBarPreviewFollowsTheDateFormatPicker(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	th := material.NewTheme()
	gtx := statusBarPreviewContext(new(input.Router))
	for i := range st.statusBarFieldBools {
		st.statusBarFieldBools[i].Value = false
	}
	st.statusBarFieldBools[filePaneStatusFieldDate].Value = true

	tests := []struct {
		key  string
		want string
	}{
		// auto follows the Full-mode date builder's first format, which the
		// freshly opened modal loaded from the default config.
		{fm.StatusBarDateFormatAuto, settingsPanePreviewTime.Format("Jan 02 2006 15:04")},
		{fm.StatusBarDateFormatISO, settingsPanePreviewTime.Format("2006-01-02 15:04")},
		{fm.StatusBarDateFormatUS, settingsPanePreviewTime.Format("01/02/2006 3:04 PM")},
		{fm.StatusBarDateFormatShort, settingsPanePreviewTime.Format("01-02 15:04")},
	}
	for _, tc := range tests {
		st.statusBarDateFormat = tc.key
		labels := settingsStatusBarPreviewLabels(t, ui, th, st, gtx)
		if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == tc.want }); !ok {
			t.Fatalf("date format %q renders %v, want the date column %q", tc.key, settingsStatusBarPreviewLabelTexts(labels), tc.want)
		}
	}
}

// TestSettingsStatusBarPreviewOffRendersNoStrip: the master switch takes the
// strip and nothing else. filePaneStatusBarVisible hides the bar, not the pane,
// so the mock keeps its sample rows — but no part of the strip may survive into
// the frame, or the preview would be advertising something that cannot ship.
func TestSettingsStatusBarPreviewOffRendersNoStrip(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	st.panePermissionFormat = "symbolic"
	for i := range st.statusBarFieldBools {
		st.statusBarFieldBools[i].Value = true
	}
	th := material.NewTheme()
	gtx := statusBarPreviewContext(new(input.Router))

	st.statusBarEnabledBool.Value = true
	on := settingsStatusBarPreviewLabels(t, ui, th, st, gtx)
	if len(settingsStatusBarPreviewStripLabels(ui, th, gtx, on)) == 0 {
		t.Fatal("the bar is on and nothing renders in the strip's band; the assertions below would prove nothing")
	}

	st.statusBarEnabledBool.Value = false
	off := settingsStatusBarPreviewLabels(t, ui, th, st, gtx)
	if strip := settingsStatusBarPreviewStripLabels(ui, th, gtx, off); len(strip) != 0 {
		t.Fatalf("%v rendered in the strip's band with the bar switched off", settingsStatusBarPreviewLabelTexts(strip))
	}
	// Belt and braces: no field value and no region separator anywhere in the
	// frame, wherever the mock happened to put them.
	texts := settingsStatusBarPreviewLabelTexts(off)
	for field, value := range settingsStatusBarPreviewFieldValues() {
		if _, ok := findStatusBarLabel(off, func(s string) bool { return s == value }); ok {
			t.Fatalf("field %d's value %q rendered with the bar switched off (labels: %v)", field, value, texts)
		}
	}
	if _, ok := findStatusBarLabel(off, func(s string) bool { return s == "\u2502" }); ok {
		t.Fatalf("the strip's region separator rendered with the bar switched off (labels: %v)", texts)
	}
	// The pane itself stays: the switch is about the bar, and an empty frame
	// would read as a picture that failed to draw.
	cursorName := settingsStatusBarPreviewEntries[settingsStatusBarPreviewCursor].Name
	if _, ok := findStatusBarLabel(off, func(s string) bool { return s == cursorName }); !ok {
		t.Fatalf("the sample row %q vanished with the bar switched off; the switch must drop the strip, not the pane (labels: %v)", cursorName, texts)
	}
}

// TestStepStatusBarDateFormatWalksTheOptions covers the picker's Left/Right
// stepping, mirroring stepPanePermissionFormat's behaviour: the four options in
// order, wrapping at both ends, and unknown drafts normalising to auto first.
func TestStepStatusBarDateFormatWalksTheOptions(t *testing.T) {
	st := &settingsModalState{}
	want := []string{"iso", "us", "short", "auto", "iso"}
	for i, next := range want {
		if !st.stepStatusBarDateFormat(1, time.Now()) {
			t.Fatalf("step %d from %q reported no change", i, st.statusBarDateFormat)
		}
		if st.statusBarDateFormat != next {
			t.Fatalf("step %d landed on %q, want %q", i, st.statusBarDateFormat, next)
		}
	}
	if !st.stepStatusBarDateFormat(-1, time.Now()) || st.statusBarDateFormat != "auto" {
		t.Fatalf("left from iso landed on %q, want auto", st.statusBarDateFormat)
	}
	st.statusBarDateFormat = "not-a-format"
	if !st.stepStatusBarDateFormat(1, time.Now()) || st.statusBarDateFormat != "iso" {
		t.Fatalf("right from an unknown draft landed on %q, want iso (normalise to auto, then step)", st.statusBarDateFormat)
	}
	if st.stepStatusBarDateFormat(0, time.Now()) {
		t.Fatal("a zero step must not report a change")
	}
}

// TestSettingsStatusBarDateFormatOptionsDeriveFromTheLayouts pins the picker's
// sample labels to fm.StatusBarDateLayout, so changing a layout in fm re-labels
// the picker without anyone remembering this file.
func TestSettingsStatusBarDateFormatOptionsDeriveFromTheLayouts(t *testing.T) {
	options := settingsStatusBarDateFormatOptions()
	if len(options) != 4 {
		t.Fatalf("picker has %d options, want 4", len(options))
	}
	if options[0].Key != fm.StatusBarDateFormatAuto || options[0].Label != "Match columns" {
		t.Fatalf("option 0 = %+v, want the auto option labelled Match columns", options[0])
	}
	for _, option := range options[1:] {
		layout := fm.StatusBarDateLayout(option.Key)
		if layout == "" {
			t.Fatalf("option %q has no layout in fm.StatusBarDateLayout", option.Key)
		}
		if want := settingsStatusBarDateSampleTime.Format(layout); option.Label != want {
			t.Fatalf("option %q is labelled %q, want the rendered sample %q", option.Key, option.Label, want)
		}
	}
	var st settingsModalState
	if len(st.statusBarDateFormatClicks) < len(options) {
		t.Fatalf("statusBarDateFormatClicks holds %d clickables for %d options", len(st.statusBarDateFormatClicks), len(options))
	}
}

func settingsStatusBarFrameLoop(t *testing.T, ui *UI, th *material.Theme, router *input.Router, gtx *layout.Context) func(time.Time) {
	t.Helper()
	return func(at time.Time) {
		gtx.Now = at
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, *gtx)
		router.Frame(gtx.Ops)
	}
}

// TestSettingsStatusBarTabRendersAndClickSelectsIt drives the real layout path:
// it exercises the tab button (the click state Task 7 left unconsumed) and the
// tab body, which nothing else in the suite reaches.
func TestSettingsStatusBarTabRendersAndClickSelectsIt(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)
	frame := settingsStatusBarFrameLoop(t, ui, th, router, &gtx)

	st.activeTab = "general"
	st.paneSettingsMode = "full"
	frame(now)

	st.paneSettingsStatusBarClick.Click()
	frame(now.Add(time.Millisecond))

	if got := normalizeSettingsPaneMode(st.paneSettingsMode); got != "statusbar" {
		t.Fatalf("clicking the Status bar tab selected %q", got)
	}
	// A second frame lays out the tab body itself.
	frame(now.Add(2 * time.Millisecond))
}

func TestSettingsStatusBarTabKeyboardSpaceTogglesFieldCheckbox(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)
	frame := settingsStatusBarFrameLoop(t, ui, th, router, &gtx)

	st.activeTab = "general"
	st.paneSettingsMode = "statusbar"
	st.statusBarEnabledBool.Value = true
	st.statusBarFieldBools[filePaneStatusFieldOwner].Value = false
	st.focus = settingsKeyboardFocusStatusBarField(filePaneStatusFieldOwner)
	st.keyFocus.wantFocus = true

	frame(now)
	router.Queue(key.Event{Name: key.NameSpace, State: key.Press})
	frame(now.Add(time.Millisecond))

	if !st.statusBarFieldBools[filePaneStatusFieldOwner].Value {
		t.Fatal("Space should tick the focused status bar field checkbox")
	}
	if !slices.Contains(st.statusBarSelectedFields(), fm.StatusBarFieldOwner) {
		t.Fatalf("selected fields = %v, want owner", st.statusBarSelectedFields())
	}
}

// TestSettingsStatusBarTabKeyboardArrowStepsDatePicker drives the real modal
// keyboard path: with the picker focused, Right and Left step the draft date
// format the way the permission-format picker steps, via
// stepFocusedHorizontalGroup.
func TestSettingsStatusBarTabKeyboardArrowStepsDatePicker(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)
	frame := settingsStatusBarFrameLoop(t, ui, th, router, &gtx)

	st.activeTab = "general"
	st.paneSettingsMode = "statusbar"
	st.statusBarEnabledBool.Value = true
	st.focus = settingsKeyboardFocusStatusBarDateFormat
	st.keyFocus.wantFocus = true

	frame(now)
	router.Queue(key.Event{Name: key.NameRightArrow, State: key.Press})
	frame(now.Add(time.Millisecond))
	if st.statusBarDateFormat != fm.StatusBarDateFormatISO {
		t.Fatalf("Right on the focused picker landed on %q, want iso", st.statusBarDateFormat)
	}

	router.Queue(key.Event{Name: key.NameLeftArrow, State: key.Press})
	frame(now.Add(2 * time.Millisecond))
	if st.statusBarDateFormat != fm.StatusBarDateFormatAuto {
		t.Fatalf("Left on the focused picker landed on %q, want auto", st.statusBarDateFormat)
	}
}

// TestSettingsStatusBarTabKeyboardSpaceIgnoresGreyedFields is the behavioural
// half of the focus-order gating: with the bar off, the greyed field checkboxes
// must ignore Space exactly as they ignore the mouse.
func TestSettingsStatusBarTabKeyboardSpaceIgnoresGreyedFields(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	th := material.NewTheme()
	now := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, now)
	frame := settingsStatusBarFrameLoop(t, ui, th, router, &gtx)

	st.activeTab = "general"
	st.paneSettingsMode = "statusbar"
	st.statusBarEnabledBool.Value = false
	st.statusBarFieldBools[filePaneStatusFieldOwner].Value = false
	st.statusBarHideInFullBool.Value = false
	// Aimed at a greyed control; normalizeKeyboardFocus should refuse it.
	st.focus = settingsKeyboardFocusStatusBarField(filePaneStatusFieldOwner)
	st.keyFocus.wantFocus = true

	frame(now)
	router.Queue(key.Event{Name: key.NameSpace, State: key.Press})
	frame(now.Add(time.Millisecond))

	if st.statusBarFieldBools[filePaneStatusFieldOwner].Value {
		t.Fatal("Space ticked a greyed field checkbox that ignores the mouse")
	}

	st.focus = settingsKeyboardFocusStatusBarHideInFull
	st.keyFocus.wantFocus = true
	frame(now.Add(2 * time.Millisecond))
	router.Queue(key.Event{Name: key.NameSpace, State: key.Press})
	frame(now.Add(3 * time.Millisecond))

	if st.statusBarHideInFullBool.Value {
		t.Fatal("Space ticked the greyed hide-in-full checkbox")
	}
}

// settingsStatusBarMouseColumnX is the x the greying scans click at: inside the
// status bar tab's checkbox rows, and well right of the modal's left-hand nav.
const settingsStatusBarMouseColumnX = 300

// settingsStatusBarClickColumn clicks straight down the status bar tab's
// checkbox column, one pixel at a time, and reports every y at which changed
// saw its control flip. It scans rather than taking a y, so the tests below
// locate the row themselves and keep working when the tab's spacing moves.
//
// Every control is reset to the baseline before each click, so a stray hit on a
// neighbour — the master switch above the fields, say — cannot poison the rest
// of the sweep. One frame per y is enough: the frame that processes a click also
// re-registers the pointer areas for the next one, and nothing here moves them.
func settingsStatusBarClickColumn(t *testing.T, barEnabled bool, changed func(*settingsModalState) bool) []int {
	t.Helper()
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	th := material.NewTheme()
	base := time.Now()
	router := new(input.Router)
	gtx := testDialogLayoutContext(router, base)

	frames := 0
	frame := func() {
		frames++
		gtx.Now = base.Add(time.Duration(frames) * time.Millisecond)
		gtx.Ops.Reset()
		ui.layoutSettingsModal(th, gtx)
		router.Frame(gtx.Ops)
	}
	reset := func() {
		st.activeTab = "general"
		st.paneSettingsMode = "statusbar"
		st.statusBarEnabledBool.Value = barEnabled
		st.statusBarHideInFullBool.Value = false
		for i := range st.statusBarFieldBools {
			st.statusBarFieldBools[i].Value = false
		}
		// "short" rather than the default "auto" because the scan clicks at one
		// x, which lands on the picker's first option — a click that selects the
		// already-active option would report no change and hide the row from the
		// with-bar-on scan.
		st.statusBarDateFormat = fm.StatusBarDateFormatShort
	}

	reset()
	// The first frame opens the tab; the second lays out its body, which is
	// what registers the checkboxes' pointer areas.
	frame()
	frame()

	hits := make([]int, 0, 4)
	for y := 0; y < gtx.Constraints.Max.Y; y++ {
		reset()
		at := f32.Pt(settingsStatusBarMouseColumnX, float32(y))
		router.Queue(
			pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
			pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		)
		frame()
		if changed(st) {
			hits = append(hits, y)
		}
	}
	return hits
}

// TestSettingsStatusBarTabFieldCheckboxesIgnoreTheMouseWhenTheBarIsOff is the
// mouse half of the tab's stated safety property: you cannot change which
// fields the bar shows while the bar is switched off. It rests on one
// gtx.Disabled() inside the field loop, and nothing else in the suite goes
// anywhere near it — TestSettingsStatusBarTabFocusOrderSkipsDisabledControls
// covers the keyboard, and TestGtxDisabledBlocksCheckboxPointerInput covers a
// bare widget.Bool, so both stay green if the call is deleted.
func TestSettingsStatusBarTabFieldCheckboxesIgnoreTheMouseWhenTheBarIsOff(t *testing.T) {
	owner := func(st *settingsModalState) bool {
		return st.statusBarFieldBools[filePaneStatusFieldOwner].Value
	}

	on := settingsStatusBarClickColumn(t, true, owner)
	if len(on) == 0 {
		t.Fatal("with the bar on, no click anywhere down the column toggled Owner / group; the scan never found the row, so the assertion below would prove nothing")
	}
	if off := settingsStatusBarClickColumn(t, false, owner); len(off) != 0 {
		t.Fatalf("with the bar off, clicking at y=%v toggled the greyed Owner / group checkbox (with the bar on it toggles at y=%v); the field loop is missing its gtx.Disabled()", off, on)
	}
}

// TestSettingsStatusBarTabHideInFullIgnoresTheMouseWhenTheBarIsOff is the same
// guard for the hide-in-full row, which greys itself with a separate
// gtx.Disabled() of its own and so can regress on its own.
func TestSettingsStatusBarTabHideInFullIgnoresTheMouseWhenTheBarIsOff(t *testing.T) {
	hideInFull := func(st *settingsModalState) bool { return st.statusBarHideInFullBool.Value }

	on := settingsStatusBarClickColumn(t, true, hideInFull)
	if len(on) == 0 {
		t.Fatal("with the bar on, no click anywhere down the column toggled Hide it in full mode; the scan never found the row, so the assertion below would prove nothing")
	}
	if off := settingsStatusBarClickColumn(t, false, hideInFull); len(off) != 0 {
		t.Fatalf("with the bar off, clicking at y=%v toggled the greyed Hide it in full mode checkbox (with the bar on it toggles at y=%v); that row is missing its gtx.Disabled()", off, on)
	}
}

// TestSettingsStatusBarTabDatePickerIgnoresTheMouseWhenTheBarIsOff is the same
// guard again for the date-layout picker, which sits below the field checkboxes
// and greys itself with its own gtx.Disabled(). The scan's x lands on one of
// the picker's option tabs; the reset parks the draft on "short" so a landed
// click always changes the value.
func TestSettingsStatusBarTabDatePickerIgnoresTheMouseWhenTheBarIsOff(t *testing.T) {
	dateFormat := func(st *settingsModalState) bool {
		return st.statusBarDateFormat != fm.StatusBarDateFormatShort
	}

	on := settingsStatusBarClickColumn(t, true, dateFormat)
	if len(on) == 0 {
		t.Fatal("with the bar on, no click anywhere down the column changed the date-layout picker; the scan never found the row, so the assertion below would prove nothing")
	}
	if off := settingsStatusBarClickColumn(t, false, dateFormat); len(off) != 0 {
		t.Fatalf("with the bar off, clicking at y=%v changed the greyed date-layout picker (with the bar on it changes at y=%v); the picker is missing its gtx.Disabled()", off, on)
	}
}

// TestSettingsPaneModeKeysEachRenderADistinctTabBody guards the dispatch switch
// in layoutSettingsFilePaneEditor, which is an implicit list: it names brief,
// statusbar and other, and lets everything else fall through to the full mode
// body. A fifth key added to settingsPaneModeKeys, to modeClicks and to
// normalizeSettingsPaneMode but not to that switch would draw the Full tab under
// its own tab button, and nothing else in the suite would notice.
//
// Body height is the proxy, because a fall-through gives itself away by
// rendering the Full body exactly. Two tabs that legitimately grow to the same
// height would trip this as well; the fix then is a sharper signal, not a
// deleted test.
func TestSettingsPaneModeKeysEachRenderADistinctTabBody(t *testing.T) {
	th := material.NewTheme()
	seen := make(map[int]string, len(settingsPaneModeKeys))
	for _, mode := range settingsPaneModeKeys {
		ui := NewUI(fm.DefaultConfig())
		ui.openSettingsModal()
		st := ui.settingsModal
		if st == nil {
			t.Fatal("settings modal did not open")
		}
		st.activeTab = "general"
		st.paneSettingsMode = mode
		gtx := testDialogLayoutContext(new(input.Router), time.Now())
		height := ui.layoutSettingsFilePaneEditor(th, gtx, st).Size.Y
		if other, dup := seen[height]; dup {
			t.Fatalf("the %q and %q tabs both render a %dpx body; %q is probably missing from the dispatch switch in layoutSettingsFilePaneEditor and is falling through to the full mode body", other, mode, height, mode)
		}
		seen[height] = mode
	}
}

// TestGtxDisabledBlocksCheckboxPointerInput answers the question the greyed
// field checkboxes depend on: gtx.Disabled() must actually stop the widget from
// seeing input, not merely change its colours. Both halves matter — a click
// swallowed while disabled must not resurface when the context is enabled
// again, or unticking the master switch would arm a delayed toggle.
//
// Scope, so nobody mistakes this for coverage of the status bar tab: it drives a
// bare widget.Bool and never touches layoutSettingsPaneStatusBarTab, so it stays
// green if either gtx.Disabled() call in that tab is deleted. It is a Gio
// upgrade tripwire — if a future Gio delivers pointer input to disabled widgets,
// this fails first and explains why the two scans above then fail too. The tab's
// own greying is covered by those scans and by
// TestSettingsStatusBarTabFocusOrderSkipsDisabledControls.
func TestGtxDisabledBlocksCheckboxPointerInput(t *testing.T) {
	var box widget.Bool
	router := new(input.Router)
	now := time.Now()

	layoutAt := func(at time.Time, disabled bool) {
		gtx := testDialogLayoutContext(router, at)
		gtx.Constraints.Min = image.Pt(40, 20)
		if disabled {
			gtx = gtx.Disabled()
		}
		box.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(40, 20)}
		})
		router.Frame(gtx.Ops)
	}
	clickAt := func(at time.Time) {
		router.Queue(
			pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(10, 10)},
			pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(10, 10)},
		)
	}

	layoutAt(now, true)
	clickAt(now)
	layoutAt(now.Add(time.Millisecond), true)
	if box.Value {
		t.Fatal("gtx.Disabled() did not block the click; the greyed field checkboxes would still be togglable by mouse")
	}

	// Re-enabled, and without a fresh click, the swallowed press must not land.
	layoutAt(now.Add(2*time.Millisecond), false)
	layoutAt(now.Add(3*time.Millisecond), false)
	if box.Value {
		t.Fatal("a click swallowed while disabled resurfaced once the context was enabled")
	}

	// Sanity check the harness: an enabled click has to work, or the assertions
	// above prove nothing.
	clickAt(now.Add(4 * time.Millisecond))
	layoutAt(now.Add(5*time.Millisecond), false)
	if !box.Value {
		t.Fatal("an enabled click did not toggle the checkbox; the disabled assertions above are vacuous")
	}
}
