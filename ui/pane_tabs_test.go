// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/fm"
	"image"
	"path/filepath"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func TestFilePaneTabsAddActivateAndClose(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	first := ui.filePanes[0]
	first.dir = t.TempDir()

	if !ui.addFilePaneTab(0) {
		t.Fatal("addFilePaneTab should add a tab")
	}
	if got, want := len(ui.filePaneTabs[0].tabs), 2; got != want {
		t.Fatalf("tab count=%d want %d", got, want)
	}
	second := ui.filePaneTabs[0].tabs[1]
	if ui.filePanes[0] != second {
		t.Fatal("new tab should become active")
	}

	if !ui.activateFilePaneTab(0, 0) {
		t.Fatal("activateFilePaneTab should switch tabs")
	}
	if ui.filePanes[0] != first {
		t.Fatal("first tab should become active")
	}

	if !ui.closeFilePaneTab(0, 1) {
		t.Fatal("closeFilePaneTab should close inactive tab")
	}
	if got, want := len(ui.filePaneTabs[0].tabs), 1; got != want {
		t.Fatalf("tab count after close=%d want %d", got, want)
	}
	if ui.filePanes[0] != first {
		t.Fatal("closing an inactive tab should keep the current tab active")
	}
	if ui.closeFilePaneTab(0, 0) {
		t.Fatal("closing the last tab should be ignored")
	}
}

func TestActivateFilePaneTabPreservesScrollAnchor(t *testing.T) {
	cfg := fm.DefaultConfig()
	tabs := []*filePaneState{
		newFilePaneState(filepath.Join(t.TempDir(), "alpha"), cfg),
		newFilePaneState(filepath.Join(t.TempDir(), "beta"), cfg),
		newFilePaneState(filepath.Join(t.TempDir(), "gamma"), cfg),
		newFilePaneState(filepath.Join(t.TempDir(), "delta"), cfg),
	}
	ui := &UI{
		fmCfg:        cfg,
		filePanes:    []*filePaneState{tabs[3]},
		filePaneTabs: []filePaneTabSet{{tabs: tabs, active: 3, scroll: 3}},
	}

	if !ui.activateFilePaneTab(0, 2) {
		t.Fatal("activateFilePaneTab should switch tabs")
	}
	if got, want := ui.filePaneTabs[0].scroll, 3; got != want {
		t.Fatalf("scroll anchor=%d want preserved %d", got, want)
	}
}

func TestFilePaneTabCloseButtonDoesNotSelectInactiveTab(t *testing.T) {
	cfg := fm.DefaultConfig()
	first := newFilePaneState(filepath.Join(t.TempDir(), "alpha"), cfg)
	second := newFilePaneState(filepath.Join(t.TempDir(), "beta"), cfg)
	third := newFilePaneState(filepath.Join(t.TempDir(), "gamma"), cfg)
	ui := &UI{
		fmCfg:        cfg,
		filePanes:    []*filePaneState{third},
		filePaneTabs: []filePaneTabSet{{tabs: []*filePaneState{first, second, third}, active: 2}},
	}

	th := material.NewTheme()
	router := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(360, tabStripHeightDp),
		},
	}
	frame := func() {
		gtx.Ops.Reset()
		ui.layoutFilePaneTabStrip(th, gtx, 0)
		router.Frame(gtx.Ops)
	}

	frame()
	widths := tabStripWidths(gtx, cfg, []appTabItem{
		{title: filePaneTabTitle(first)},
		{title: filePaneTabTitle(second)},
		{title: filePaneTabTitle(third), active: true},
	})
	x := widths[0] + tabStripSeparatorWidth(gtx) + widths[1] - gtx.Dp(unit.Dp(10))
	y := tabStripHeightDp / 2
	pos := f32.Pt(float32(x), float32(y))
	router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonPrimary,
		Position: pos,
	})
	frame()
	router.Queue(pointer.Event{
		Kind:     pointer.Release,
		Source:   pointer.Mouse,
		Position: pos,
	})
	frame()

	if got, want := len(ui.filePaneTabs[0].tabs), 2; got != want {
		t.Fatalf("tab count after close=%d want %d", got, want)
	}
	if ui.filePanes[0] != third {
		t.Fatal("close button on an inactive tab should keep the existing active tab")
	}
	if got, want := ui.filePaneTabs[0].active, 1; got != want {
		t.Fatalf("active tab=%d want %d", got, want)
	}
}

func TestSnapshotSessionIncludesFilePaneTabs(t *testing.T) {
	cfg := fm.DefaultConfig()
	leftA := newFilePaneState(filepath.Join(t.TempDir(), "alpha"), cfg)
	leftB := newFilePaneState(filepath.Join(t.TempDir(), "beta"), cfg)
	right := newFilePaneState(filepath.Join(t.TempDir(), "gamma"), cfg)
	ui := &UI{
		Tabs:           widget.Enum{Value: "tab0"},
		fmCfg:          cfg,
		filePanes:      []*filePaneState{leftB, right},
		filePaneTabs:   []filePaneTabSet{{tabs: []*filePaneState{leftA, leftB}, active: 1}, {tabs: []*filePaneState{right}}},
		activeFilePane: 0,
	}

	s := ui.SnapshotSession()

	if got, want := len(s.FilePaneTabs), 2; got != want {
		t.Fatalf("len(FilePaneTabs)=%d want %d", got, want)
	}
	if got, want := s.FilePaneTabs[0].Active, 1; got != want {
		t.Fatalf("left active tab=%d want %d", got, want)
	}
	if got, want := s.Panes[0].Dir, leftB.dir; got != want {
		t.Fatalf("active pane dir=%q want %q", got, want)
	}
	if got, want := s.FilePaneTabs[0].Tabs[0].Dir, leftA.dir; got != want {
		t.Fatalf("saved hidden tab dir=%q want %q", got, want)
	}
}

func TestApplySessionRestoresFilePaneTabs(t *testing.T) {
	cfg := fm.DefaultConfig()
	leftA := t.TempDir()
	leftB := t.TempDir()
	ui := NewUI(cfg)

	ui.ApplySession(&fm.SessionState{
		ActiveTab:  "tab0",
		ActivePane: 0,
		FilePaneTabs: []fm.SessionPaneTabs{
			{
				Active: 1,
				Tabs: []fm.SessionPane{
					{Dir: leftA, SortKey: "date", Mode: "brief"},
					{Dir: leftB, SortKey: "size", SortDescending: true, Mode: "full"},
				},
			},
		},
	})

	if got, want := len(ui.filePaneTabs[0].tabs), 2; got != want {
		t.Fatalf("left tab count=%d want %d", got, want)
	}
	active := ui.filePaneTabs[0].tabs[1]
	if ui.filePanes[0] != active {
		t.Fatal("active restored tab should be visible")
	}
	if got, want := active.dir, filepath.Clean(leftB); got != want {
		t.Fatalf("active tab dir=%q want %q", got, want)
	}
	if got, want := active.sessionSortKey(), "size"; got != want {
		t.Fatalf("active tab sort=%q want %q", got, want)
	}
	if !active.sortDesc {
		t.Fatal("active tab sort direction should be restored")
	}
}

func TestTabStripPlanUsesScrollControlsWhenOverflowing(t *testing.T) {
	plan := tabStripPlan([]int{80, 80, 80, 80}, 220, 22, 1)

	if !plan.overflow {
		t.Fatal("plan should overflow")
	}
	if got, want := plan.start, 1; got != want {
		t.Fatalf("plan start=%d want %d", got, want)
	}
	if plan.end <= plan.start || plan.end > 4 {
		t.Fatalf("invalid visible range %d:%d", plan.start, plan.end)
	}
}

func TestTabStripPlanBackfillsWhenAnchoredAtEnd(t *testing.T) {
	plan := tabStripPlan([]int{80, 80, 80, 80}, 320, 22, 3)

	if !plan.overflow {
		t.Fatal("plan should overflow")
	}
	if got, want := plan.start, 1; got != want {
		t.Fatalf("plan start=%d want %d to show a filled suffix", got, want)
	}
	if got, want := plan.end, 4; got != want {
		t.Fatalf("plan end=%d want %d", got, want)
	}
}

func TestTabStripPlanNextAnchorAdvancesAfterBackfilledStart(t *testing.T) {
	widths := []int{80, 80, 80, 80, 80}
	first := tabStripPlan(widths, 320, 22, 0)
	if got, want := first.start, 0; got != want {
		t.Fatalf("first start=%d want %d", got, want)
	}
	if got, want := first.end, 3; got != want {
		t.Fatalf("first end=%d want %d", got, want)
	}

	nextAnchor := tabStripNextScrollAnchor(first, len(widths))
	if got, want := nextAnchor, 1; got != want {
		t.Fatalf("next anchor=%d want %d", got, want)
	}
	next := tabStripPlan(widths, 320, 22, nextAnchor)
	if got, want := next.start, 1; got != want {
		t.Fatalf("next start=%d want %d", got, want)
	}
	if got, want := next.end, 4; got != want {
		t.Fatalf("next end=%d want %d", got, want)
	}

	secondAnchor := tabStripNextScrollAnchor(next, len(widths))
	if got, want := secondAnchor, 2; got != want {
		t.Fatalf("second next anchor=%d want %d", got, want)
	}
	second := tabStripPlan(widths, 320, 22, secondAnchor)
	if got, want := second.start, 2; got != want {
		t.Fatalf("second start=%d want %d", got, want)
	}
	if got, want := second.end, 5; got != want {
		t.Fatalf("second end=%d want %d", got, want)
	}
}

func TestTabStripPlanShrinksTabsBeforeOverflowing(t *testing.T) {
	plan := tabStripPlanWithMin([]int{100, 100, 100}, []int{44, 44, 44}, 280, 22, 2)

	if plan.overflow {
		t.Fatal("tabs should moderately shrink to fit before scroll controls appear")
	}
	if got, want := plan.start, 0; got != want {
		t.Fatalf("plan start=%d want %d", got, want)
	}
	if got, want := plan.end, 3; got != want {
		t.Fatalf("plan end=%d want %d", got, want)
	}
	if len(plan.widths) != 3 {
		t.Fatalf("fit width count=%d want 3", len(plan.widths))
	}
	for i, w := range plan.widths {
		if w < 80 || w > 100 {
			t.Fatalf("fit width[%d]=%d want between comfortable min and preferred", i, w)
		}
	}
}

func TestTabStripPlanOverflowsBeforeOverShrinkingTabs(t *testing.T) {
	plan := tabStripPlanWithMin([]int{100, 100, 100}, []int{44, 44, 44}, 240, 22, 2)

	if !plan.overflow {
		t.Fatal("tabs should overflow instead of shrinking all the way to the hard minimum")
	}
}

func TestTabStripPlanOverflowWidthsFillAvailableStrip(t *testing.T) {
	available := 320
	controlW := 22
	plan := tabStripPlan([]int{80, 80, 80, 80}, available, controlW, 3)

	used := 3 * controlW
	used += (len(plan.widths) + 2) * tabStripSeparatorWidth(layout.Context{})
	for _, w := range plan.widths {
		used += w
	}
	if got, want := used, available; got != want {
		t.Fatalf("overflow strip used width=%d want %d", got, want)
	}
}

func TestNavigateFavoriteOpensNewFilePaneTabByDefault(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	base := ui.filePanes[0]
	base.dir = filepath.Clean(t.TempDir())
	target := filepath.Clean(t.TempDir())

	if !ui.navigatePaneFavorite(0, target) {
		t.Fatal("navigatePaneFavorite should open the favorite")
	}
	if got, want := len(ui.filePaneTabs[0].tabs), 2; got != want {
		t.Fatalf("tab count=%d want %d", got, want)
	}
	active := ui.filePanes[0]
	if active == base {
		t.Fatal("favorite should open in a new tab by default")
	}
	if got := active.loadingDir; got != target {
		t.Fatalf("active loadingDir=%q want %q", got, target)
	}
}

func TestNavigateFavoriteCanReuseCurrentFilePaneTab(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.General.OpenFavoritesInNewTab = false
	ui := NewUI(cfg)
	ui.ensureFilePaneTabs()
	base := ui.filePanes[0]
	base.dir = filepath.Clean(t.TempDir())
	target := filepath.Clean(t.TempDir())

	if !ui.navigatePaneFavorite(0, target) {
		t.Fatal("navigatePaneFavorite should open the favorite")
	}
	if got, want := len(ui.filePaneTabs[0].tabs), 1; got != want {
		t.Fatalf("tab count=%d want %d", got, want)
	}
	if ui.filePanes[0] != base {
		t.Fatal("favorite should reuse the current tab when the setting is disabled")
	}
	if got := base.loadingDir; got != target {
		t.Fatalf("loadingDir=%q want %q", got, target)
	}
}

func TestTerminalPaneTabStripAddButtonHandlesPointerClick(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Terminal.HeightRows = 4
	ui := NewUI(cfg)
	ui.terminal = newTerminalSession(nil)
	ui.terminal.setActive(true)
	ui.terminal.startAttempted = true
	ui.ensureTerminalTabs()

	th := material.NewTheme()
	router := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(320, 160),
		},
	}
	frame := func() {
		gtx.Ops.Reset()
		ui.layoutTerminalPane(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame()
	tabGtx := gtx
	tabGtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X-12, tabStripHeightDp))
	widths := tabStripWidths(tabGtx, cfg, []appTabItem{{title: "terminal", active: true}})
	x := 6 + widths[0] + tabStripSeparatorWidth(tabGtx) + tabStripControlWidth(tabGtx)/2
	y := 4 + tabStripHeightDp/2
	pos := f32.Pt(float32(x), float32(y))
	router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonPrimary,
		Position: pos,
	})
	frame()
	router.Queue(pointer.Event{
		Kind:     pointer.Release,
		Source:   pointer.Mouse,
		Position: pos,
	})
	frame()

	if got, want := len(ui.terminalTabs.sessions), 2; got != want {
		t.Fatalf("terminal tab count=%d want %d", got, want)
	}
	if got, want := ui.terminalTabs.active, 1; got != want {
		t.Fatalf("active terminal tab=%d want %d", got, want)
	}
}

func TestActivateTerminalTabPreservesScrollAnchor(t *testing.T) {
	cfg := fm.DefaultConfig()
	sessions := []*terminalSession{
		newTerminalSession(nil),
		newTerminalSession(nil),
		newTerminalSession(nil),
		newTerminalSession(nil),
	}
	ui := &UI{
		fmCfg: cfg,
		terminalTabs: terminalTabSet{
			sessions: sessions,
			active:   3,
			scroll:   3,
		},
		terminal: sessions[3],
	}

	if !ui.activateTerminalTab(2) {
		t.Fatal("activateTerminalTab should switch tabs")
	}
	if got, want := ui.terminalTabs.scroll, 3; got != want {
		t.Fatalf("terminal scroll anchor=%d want preserved %d", got, want)
	}
}
