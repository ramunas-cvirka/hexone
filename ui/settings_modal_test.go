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
		{key: "viewer", want: 0},
		{key: "associations", want: 1},
		{key: "general", want: 2},
		{key: "config", want: 3},
	}
	for _, tc := range cases {
		if got := settingsTabIndex(tc.key); got != tc.want {
			t.Fatalf("settingsTabIndex(%q)=%d want %d", tc.key, got, tc.want)
		}
	}
}

func TestSettingsTabPositionSlidesToAssociations(t *testing.T) {
	now := time.Date(2026, time.March, 7, 10, 0, 0, 0, time.UTC)
	st := &settingsModalState{activeTab: "viewer"}

	st.setActiveTab("associations", now)
	if st.activeTab != "associations" {
		t.Fatalf("activeTab=%q want %q", st.activeTab, "associations")
	}
	if st.navPrevTab != "viewer" {
		t.Fatalf("navPrevTab=%q want %q", st.navPrevTab, "viewer")
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
	if mid <= 0 || mid >= 1 {
		t.Fatalf("mid position=%v want between 0 and 1", mid)
	}

	end, anim := st.tabPosition(now.Add(toolbarAnimDur))
	if anim {
		t.Fatal("tabPosition should stop animating at the end of the transition")
	}
	if end != 1 {
		t.Fatalf("end position=%v want 1", end)
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
