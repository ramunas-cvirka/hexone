package ui

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"
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
		{key: "viewer", want: 1},
		{key: "associations", want: 2},
		{key: "colors", want: 3},
		{key: "config", want: 4},
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
		{key: "viewer", step: 1, want: "associations"},
		{key: "colors", step: -1, want: "associations"},
	}
	for _, tc := range cases {
		if got := settingsShiftTab(tc.key, tc.step); got != tc.want {
			t.Fatalf("settingsShiftTab(%q, %d)=%q want %q", tc.key, tc.step, got, tc.want)
		}
	}
}

func TestSettingsStepActiveTabSetsPulse(t *testing.T) {
	now := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	st := &settingsModalState{activeTab: "general"}

	if !st.stepActiveTab(1, now) {
		t.Fatal("stepActiveTab should report a tab change")
	}
	if st.activeTab != "viewer" {
		t.Fatalf("activeTab=%q want %q", st.activeTab, "viewer")
	}
	if st.navPrevTab != "general" {
		t.Fatalf("navPrevTab=%q want %q", st.navPrevTab, "general")
	}
	if st.navPulseKey != "viewer" {
		t.Fatalf("navPulseKey=%q want %q", st.navPulseKey, "viewer")
	}
	if st.navPulseAt != now {
		t.Fatalf("navPulseAt=%v want %v", st.navPulseAt, now)
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
	if mid <= 0 || mid >= 2 {
		t.Fatalf("mid position=%v want between 0 and 2", mid)
	}

	end, anim := st.tabPosition(now.Add(toolbarAnimDur))
	if anim {
		t.Fatal("tabPosition should stop animating at the end of the transition")
	}
	if end != 2 {
		t.Fatalf("end position=%v want 2", end)
	}
	if st.navPrevTab != "" {
		t.Fatalf("navPrevTab should clear after transition, got %q", st.navPrevTab)
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
	if assoc, ok := st.viewerAssociation(fm.NormalizeViewerAssociationExtension("mkv")); !ok || assoc.AppPath != `C:\Apps\music.exe` {
		t.Fatalf("picked app should auto-associate current extension, got %#v ok=%v", assoc, ok)
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

func TestRefreshViewerAssociationDraftInfoAutoUpdatesExistingAssociation(t *testing.T) {
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
	if assoc.AppPath != `C:\Apps\new-music.exe` {
		t.Fatalf("existing association not updated: got %q", assoc.AppPath)
	}
	if !strings.Contains(st.assocInfoText, "mp3 association changed") {
		t.Fatalf("missing green association change hint, got %q", st.assocInfoText)
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
	if !strings.Contains(got, "mp3 association changed") {
		t.Fatalf("viewerAssociationNoticeText=%q, want change notice", got)
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
		colorSelection:           "#3456AA",
		colorSelectionText:       "#F2F7FF",
		colorFocusedSelected:     "#447F9C",
		colorFocusedSelectedText: "#F6FBFF",
	}
	if got := settingsColorLabel("selection"); got != "Focused" {
		t.Fatalf("selection label=%q want %q", got, "Focused")
	}
	if got := settingsColorLabel("focused_selected"); got != "Focused + Selected Files" {
		t.Fatalf("focused_selected label=%q want %q", got, "Focused + Selected Files")
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
