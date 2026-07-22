// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	resources "hexone"
	"hexone/fm"
	"image"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
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

func TestDisconnectCurrentRemoteTabClosesItWhenAnotherTabExists(t *testing.T) {
	cfg := fm.DefaultConfig()
	local := newFilePaneState(t.TempDir(), cfg)
	remote := newFilePaneState("/srv/app", cfg)
	remote.remote = &paneSSHSession{identity: "root@srv.test:22"}
	ui := &UI{
		fmCfg:        cfg,
		filePanes:    []*filePaneState{remote},
		filePaneTabs: []filePaneTabSet{{tabs: []*filePaneState{local, remote}, active: 1}},
	}

	if !ui.disconnectCurrentFilePaneTab(0, time.Now()) {
		t.Fatal("disconnect should close the current remote tab")
	}
	if got, want := len(ui.filePaneTabs[0].tabs), 1; got != want {
		t.Fatalf("tab count=%d want %d", got, want)
	}
	if ui.filePanes[0] != local {
		t.Fatal("remaining local tab should become active")
	}
}

func TestDisconnectLastRemoteTabKeepsItAndReturnsLocal(t *testing.T) {
	cfg := fm.DefaultConfig()
	localDir := t.TempDir()
	pane := newFilePaneState("/srv/app", cfg)
	pane.localDirBeforeRemote = localDir
	pane.remote = &paneSSHSession{identity: "root@srv.test:22"}
	ui := &UI{
		fmCfg:        cfg,
		filePanes:    []*filePaneState{pane},
		filePaneTabs: []filePaneTabSet{{tabs: []*filePaneState{pane}, active: 0}},
	}

	if !ui.disconnectCurrentFilePaneTab(0, time.Now()) {
		t.Fatal("disconnect should convert the last remote tab to local")
	}
	if got, want := len(ui.filePaneTabs[0].tabs), 1; got != want {
		t.Fatalf("tab count=%d want %d", got, want)
	}
	if pane.remoteConnected() {
		t.Fatal("last tab should no longer be remote")
	}
	if got, want := pane.loadingDir, filepath.Clean(localDir); got != want {
		t.Fatalf("local loading dir=%q want %q", got, want)
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

func TestFilePaneTabStripReportsConnectedFrameGeometry(t *testing.T) {
	cfg := fm.DefaultConfig()
	tabs := []*filePaneState{
		newFilePaneState(filepath.Join(t.TempDir(), "src"), cfg),
		newFilePaneState(filepath.Join(t.TempDir(), "gpstrack-go"), cfg),
		newFilePaneState(filepath.Join(t.TempDir(), "git"), cfg),
	}
	for _, pane := range tabs {
		pane.cancelPendingLoad()
	}
	ui := &UI{
		fmCfg:        cfg,
		filePanes:    []*filePaneState{tabs[1]},
		filePaneTabs: []filePaneTabSet{{tabs: tabs, active: 1}},
	}
	th := material.NewTheme()
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(800, tabStripHeightDp)),
	}
	ui.layoutFilePaneTabStrip(th, gtx, 0)

	items := make([]appTabItem, len(tabs))
	for i, pane := range tabs {
		items[i] = filePaneTabItem(pane)
	}
	widths := ui.tabStripWidths(th, gtx, cfg, items)
	separatorW := tabStripSeparatorWidth(gtx)
	wantMin := widths[0] + separatorW
	wantMax := wantMin + widths[1]
	got := ui.filePaneTabs[0].geometry
	if !got.activeVisible || got.activeMinX != wantMin || got.activeMaxX != wantMax {
		t.Fatalf("active geometry=%+v want visible span [%d,%d)", got, wantMin, wantMax)
	}

	connectorGtx := gtx
	connectorGtx.Ops = new(op.Ops)
	connectorGtx.Constraints = layout.Constraints{Max: image.Pt(800, 100)}
	if dims := ui.layoutFilePaneTabConnector(connectorGtx, 0, tabs[1]); dims.Size != image.Pt(800, filePaneTabConnectorHeightDp) {
		t.Fatalf("connector size=%v want %v", dims.Size, image.Pt(800, filePaneTabConnectorHeightDp))
	}
}

func TestSnapshotSessionIncludesFilePaneTabs(t *testing.T) {
	cfg := fm.DefaultConfig()
	leftA := newFilePaneState(filepath.Join(t.TempDir(), "alpha"), cfg)
	leftB := newFilePaneState(filepath.Join(t.TempDir(), "beta"), cfg)
	right := newFilePaneState(filepath.Join(t.TempDir(), "gamma"), cfg)
	leftA.filterText = "*.go"
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
	if got, want := s.FilePaneTabs[0].Tabs[0].Filter, "*.go"; got != want {
		t.Fatalf("saved hidden tab filter=%q want %q", got, want)
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
					{Dir: leftB, SortKey: "size", SortDescending: true, Mode: "full", Filter: "*.go"},
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
	if got, want := active.displayFilter(), "*.go"; got != want {
		t.Fatalf("active tab filter=%q want %q", got, want)
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

func TestTabStripWidthsUseMeasuredTitleText(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Tabs.Typeface = resources.BundledFontFamilyIosevkaNerdFontMono
	cfg.Tabs.MinWidthDp = 44
	cfg.Tabs.MaxWidthDp = 220
	ui := NewUI(cfg)
	th := material.NewTheme()
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(320, tabStripHeightDp),
		},
	}
	items := []appTabItem{{title: "retranslator", active: true}, {title: "Downloads"}}

	measured := ui.tabStripWidths(th, gtx, cfg, items)
	estimated := tabStripWidths(gtx, cfg, items)

	if len(measured) != len(estimated) || len(measured) == 0 {
		t.Fatalf("width count measured=%d estimated=%d", len(measured), len(estimated))
	}
	if measured[0] >= estimated[0] {
		t.Fatalf("measured tab width=%d should be tighter than rune estimate %d", measured[0], estimated[0])
	}
	if measured[0] < gtx.Dp(unit.Dp(cfg.Tabs.MinWidthDp)) {
		t.Fatalf("measured tab width=%d below configured min", measured[0])
	}
}

func TestTabStripUsesConfiguredFont(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.General.Typeface = resources.BundledFontFamilyHackNerdFontMono
	cfg.Tabs.Typeface = resources.BundledFontFamilyIosevkaNerdFontMono
	cfg.Tabs.FontSizeSp = 12.5
	ui := NewUI(cfg)

	if got, want := ui.tabStripTypeface(), font.Typeface(cfg.Tabs.Typeface); got != want {
		t.Fatalf("tab typeface=%q want %q", got, want)
	}
	if got, want := ui.tabStripTextSize(), unit.Sp(cfg.Tabs.FontSizeSp); got != want {
		t.Fatalf("tab text size=%v want %v", got, want)
	}
}

func TestFlatCloseButtonUsesCompactTabStyleGeometry(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	var click widget.Clickable
	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(100, 100),
		},
	}

	dims := ui.layoutFlatCloseButton(gtx, &click, false)
	if want := image.Pt(20, 18); dims.Size != want {
		t.Fatalf("flat close size=%v want %v", dims.Size, want)
	}
}

func TestTerminalTabTitleUsesOSC7WorkingDirectory(t *testing.T) {
	st := newTerminalSession(nil)
	st.startDir = `C:\Users\me`
	st.writeOutput([]byte("\x1b]7;file://localhost/C:/Users/me/Downloads\x07"))

	if got, want := terminalTabTitle(st), "Downloads"; got != want {
		t.Fatalf("terminal tab title=%q want %q", got, want)
	}
}

func TestTerminalTabTitleUsesRemoteOSC7WorkingDirectory(t *testing.T) {
	st := newTerminalSession(nil)
	st.writeOutput([]byte("\x1b]7;file://srv.test/var/log/app\x07"))

	if got, want := terminalTabTitle(st), "app"; got != want {
		t.Fatalf("terminal tab title=%q want %q", got, want)
	}
}

func TestTerminalTabItemUsesConfiguredSSHNameAndHostIdentity(t *testing.T) {
	st := newTerminalSession(nil)
	st.writeOutput([]byte("\x1b]7;file://root@srv.test/var/log/app\x07"))
	cfg := fm.DefaultConfig()
	cfg.SSH.Setups = []fm.SSHSetup{{Name: "production", User: "root", Host: "srv.test", Port: 22}}

	item := terminalTabItem(st, cfg)
	if got, want := item.title, "app"; got != want {
		t.Fatalf("terminal tab title=%q want %q", got, want)
	}
	if got, want := item.remoteKey, "root@srv.test:22"; got != want {
		t.Fatalf("terminal remote key=%q want %q", got, want)
	}
	if got, want := item.remoteTip, "production · root@srv.test:22"; got != want {
		t.Fatalf("terminal remote tooltip=%q want %q", got, want)
	}
}

func TestFilePaneRemoteTabPutsDirectoryBeforeHost(t *testing.T) {
	pane := newFilePaneState("/var/log/app", fm.DefaultConfig())
	pane.remote = &paneSSHSession{setup: fm.SSHSetup{Name: "production", User: "root", Host: "srv.test", Port: 22}}

	item := filePaneTabItem(pane)
	if got, want := item.title, "app"; got != want {
		t.Fatalf("file pane tab title=%q want %q", got, want)
	}
	if got, want := item.remoteKey, "root@srv.test:22"; got != want {
		t.Fatalf("file pane remote key=%q want %q", got, want)
	}
	pane.dir = "/"
	if got, want := filePaneTabTitle(pane), "/"; got != want {
		t.Fatalf("remote root tab title=%q want %q", got, want)
	}
}

func TestRemoteHostAccentIsStablePerHost(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	first := ui.remoteHostAccent("root@one.test:22")
	if got := ui.remoteHostAccent("ROOT@ONE.TEST:22"); got != first {
		t.Fatalf("case-normalized host accent=%v want %v", got, first)
	}
	if second := ui.remoteHostAccent("root@two.test:22"); second == first {
		t.Fatalf("different test hosts unexpectedly share accent %v", first)
	}
}

func TestRemoteFavoriteCarriesSameHostIdentityIndicator(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.FavoriteLocations = []string{"ssh://root@srv.test:22/var/log/app"}
	ui := &UI{fmCfg: cfg}

	items := ui.paneFavoriteItems(nil)
	if len(items) < 1 {
		t.Fatal("expected remote favorite item")
	}
	if got, want := items[0].remoteKey, "root@srv.test:22"; got != want {
		t.Fatalf("favorite remote key=%q want %q", got, want)
	}
}

func TestTerminalTabTitleUsesShellReportedDirectoryTitle(t *testing.T) {
	st := newTerminalSession(nil)
	st.startDir = `C:\Users\me`
	st.writeOutput([]byte("\x1b]0;me@host: /var/log/app\x07"))

	if got, want := terminalTabTitle(st), "app"; got != want {
		t.Fatalf("terminal tab title=%q want %q", got, want)
	}
}

func TestTerminalTabTitleUsesPendingRestartDirectory(t *testing.T) {
	st := newTerminalSession(nil)
	st.pendingStartDir = filepath.Join("somewhere", "preserved")

	if got, want := terminalTabTitle(st), "preserved"; got != want {
		t.Fatalf("terminal tab title=%q want %q", got, want)
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

func TestTerminalPaneTabStripMaximizeButtonTogglesConfig(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Terminal.HeightRows = 7
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	if err := fm.SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	ui := NewUI(cfg)
	ui.configPath = path
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
	x := gtx.Constraints.Max.X - 6 - tabStripControlWidth(gtx)/2
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

	if !ui.fmCfg.Terminal.Maximized {
		t.Fatal("terminal maximize button should set maximized config")
	}
	if got, want := ui.fmCfg.Terminal.HeightRows, 7; got != want {
		t.Fatalf("terminal maximize should preserve saved row count=%d want %d", got, want)
	}
	saved := fm.LoadConfig(path)
	if !saved.Terminal.Maximized {
		t.Fatal("terminal maximized state should be persisted")
	}
	if got, want := saved.Terminal.HeightRows, 7; got != want {
		t.Fatalf("saved terminal rows=%d want %d", got, want)
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

func TestTerminalVisualFocusSurvivesTabHandoff(t *testing.T) {
	first := newTerminalSession(nil)
	second := newTerminalSession(nil)
	first.setActive(true)
	ui := &UI{
		terminal: first,
		terminalTabs: terminalTabSet{
			sessions: []*terminalSession{first, second},
			active:   0,
		},
	}

	if !ui.activateTerminalTab(1) {
		t.Fatal("activateTerminalTab should switch tabs")
	}
	if !ui.terminalVisuallyFocused(layout.Context{}) {
		t.Fatal("pending terminal focus should keep pane muting stable during tab handoff")
	}
}
