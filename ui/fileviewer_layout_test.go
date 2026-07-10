// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	resources "hexone"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"hexone/fm"
)

func TestEventTagsAreNonZeroSized(t *testing.T) {
	if unsafe.Sizeof(editorMenuEventTag{}) == 0 {
		t.Fatal("editorMenuEventTag must remain non-zero-sized")
	}
	if unsafe.Sizeof(uiEventTag{}) == 0 {
		t.Fatal("uiEventTag must remain non-zero-sized")
	}
	if unsafe.Sizeof(fileViewerEventTag{}) == 0 {
		t.Fatal("fileViewerEventTag must remain non-zero-sized")
	}
}

func TestFileViewerRootPressCancelsCommandEditWithoutPopup(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		commandEditOn: true,
		command:       "tail -f {path}",
	}
	st.commandEditor.SetText("edited command")
	ui.fileViewer = st

	gtx, router := testPointerContext()
	primePointerFilter(router, &st.rootPointerTag)
	registerPointerTag(router, gtx.Ops, &st.rootPointerTag)
	router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Buttons:  pointer.ButtonPrimary,
		Position: f32.Pt(50, 50),
	})

	ui.handleFileViewerRootPointerEvents(gtx, st)

	if st.commandEditOn {
		t.Fatal("outside press should cancel viewer command edit")
	}
	if ui.editorMenuOpenID != "" {
		t.Fatal("outside press should not open or keep popup")
	}
}

func TestFileViewerRootPressClosesPopupBeforeCancelingEdit(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		commandEditOn: true,
		command:       "tail -f {path}",
	}
	ui.fileViewer = st
	ui.editorMenuOpenID = "viewer-command"

	gtx, router := testPointerContext()
	primePointerFilter(router, &st.rootPointerTag)
	registerPointerTag(router, gtx.Ops, &st.rootPointerTag)
	router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Buttons:  pointer.ButtonPrimary,
		Position: f32.Pt(50, 50),
	})

	ui.handleFileViewerRootPointerEvents(gtx, st)

	if ui.editorMenuOpenID != "" {
		t.Fatal("outside press should close popup first")
	}
	if !st.commandEditOn {
		t.Fatal("outside press should keep command edit active when it only closes popup")
	}
}

func TestFileViewerRootSecondaryPressUpdatesMenuAnchor(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		menuOpen: true,
		menuPos:  image.Pt(12, 18),
	}
	ui.fileViewer = st

	gtx, router := testPointerContext()
	primePointerFilter(router, &st.rootPointerTag)
	registerPointerTag(router, gtx.Ops, &st.rootPointerTag)
	router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Buttons:  pointer.ButtonSecondary,
		Position: f32.Pt(50, 70),
	})

	ui.handleFileViewerRootPointerEvents(gtx, st)

	if got := st.menuPos; got != image.Pt(50, 70) {
		t.Fatalf("menuPos=%v want %v", got, image.Pt(50, 70))
	}
}

func TestOpenFileViewerFindSkipsImagePreview(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	now := time.Date(2026, time.March, 27, 14, 0, 0, 0, time.UTC)
	st := &fileViewerState{
		mode:                 "file",
		detectedImagePreview: true,
	}
	st.find.resultCh = make(chan fileViewerFindResult, 1)
	ui.fileViewer = st

	ui.openFileViewerFind(now)

	if st.find.open {
		t.Fatal("image preview should not open text find UI")
	}
}

func TestFileViewerContentPressClosesEncodingMenuOutsidePopup(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		mode:             "file",
		encodingMenuOpen: true,
		encodingMenuRect: image.Rect(20, 20, 80, 60),
	}
	ui.fileViewer = st

	gtx, router := testPointerContext()
	primePointerFilter(router, &st.contentPointerTag)
	registerPointerTag(router, gtx.Ops, &st.contentPointerTag)
	router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Buttons:  pointer.ButtonPrimary,
		Position: f32.Pt(100, 100),
	})

	ui.handleFileViewerPointerEvents(gtx, st, image.Pt(200, 200))

	if st.encodingMenuOpen {
		t.Fatal("outside press should close encoding menu")
	}
}

func TestFileViewerContentPressKeepsEncodingMenuInsidePopup(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		mode:             "file",
		encodingMenuOpen: true,
		encodingMenuRect: image.Rect(20, 20, 80, 60),
	}
	ui.fileViewer = st

	gtx, router := testPointerContext()
	primePointerFilter(router, &st.contentPointerTag)
	registerPointerTag(router, gtx.Ops, &st.contentPointerTag)
	router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Buttons:  pointer.ButtonPrimary,
		Position: f32.Pt(30, 30),
	})

	ui.handleFileViewerPointerEvents(gtx, st, image.Pt(200, 200))

	if !st.encodingMenuOpen {
		t.Fatal("inside popup press should keep encoding menu open")
	}
}

func TestFileViewerContentScrollReleasesTerminalFocus(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.terminal = newTerminalSession(nil)
	ui.terminal.setActive(true)
	st := &fileViewerState{mode: "file"}
	ui.fileViewer = st

	gtx, router := testPointerContext()
	primePointerScrollFilter(router, &st.contentPointerTag, pointer.ScrollRange{}, pointer.ScrollRange{Min: -100, Max: 100})
	registerPointerTag(router, gtx.Ops, &st.contentPointerTag)
	gtx.Execute(key.FocusCmd{Tag: &ui.terminal.keyTag})
	if !ui.terminalFocused(gtx) {
		t.Fatal("terminal should start focused")
	}
	router.Queue(pointer.Event{
		Kind:     pointer.Scroll,
		Scroll:   f32.Pt(0, -1),
		Position: f32.Pt(100, 100),
	})

	ui.handleFileViewerPointerEvents(gtx, st, image.Pt(200, 200))

	if ui.terminalFocused(gtx) {
		t.Fatal("terminal should lose focus after viewer scroll")
	}
}

func TestFileViewerTabAnimationFollowsHistoryToggle(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	st := &fileViewerState{mode: "hex"}

	if got := st.activeTabKey(); got != "hex" {
		t.Fatalf("activeTabKey=%q want %q", got, "hex")
	}
	st.setHistoryOpen(true, now)
	if !st.historyOpen {
		t.Fatal("history should open")
	}
	if got := st.tabPrev; got != "hex" {
		t.Fatalf("tabPrev=%q want %q", got, "hex")
	}

	pos, anim := st.tabPosition(now.Add(toolbarAnimDur / 2))
	if !anim {
		t.Fatal("tabPosition should animate after history opens")
	}
	if pos <= 1 || pos >= 3 {
		t.Fatalf("tabPosition=%v want between 1 and 3", pos)
	}

	fillHistory, anim := st.tabFill(now.Add(toolbarAnimDur/2), "history")
	if !anim {
		t.Fatal("tabFill should animate for history tab")
	}
	if fillHistory <= 0 || fillHistory >= 1 {
		t.Fatalf("history fill=%v want between 0 and 1", fillHistory)
	}
}

func TestFileViewerModeTabsUseRegularConfiguredTabWidths(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Tabs.WidthMode = "fixed"
	cfg.Tabs.FixedWidthDp = 84
	ui := NewUI(cfg)
	th := material.NewTheme()
	st := &fileViewerState{mode: "hex"}

	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(640, 24),
		},
	}
	items := []appTabItem{{title: "File"}, {title: "Hex"}, {title: "Cmd"}}
	widths := ui.tabStripWidths(th, gtx, cfg, items)

	dims := ui.layoutFileViewerModeTabs(th, gtx, st, 24)
	separatorW := tabStripSeparatorWidth(gtx)
	historyW := tabStripTitleTextWidth(th, gtx, ui.tabStripTypeface(), ui.tabStripTextSize(), "..") + gtx.Dp(unit.Dp(14))
	if minW := tabStripControlWidth(gtx); historyW < minW {
		historyW = minW
	}
	wantW := widths[0] + widths[1] + widths[2] + separatorW*3 + historyW
	if dims.Size.X != wantW {
		t.Fatalf("mode tab strip width=%d want regular tab width total %d", dims.Size.X, wantW)
	}
	wantActive := image.Rect(widths[0]+separatorW, 0, widths[0]+separatorW+widths[1], 24)
	if st.activeTabRect != wantActive {
		t.Fatalf("active Hex tab rect=%v want %v", st.activeTabRect, wantActive)
	}
}

func TestViewerModeTabTitleMovesFilenameToActiveMode(t *testing.T) {
	st := &fileViewerState{name: "tik_tok.jpg", mode: "file"}
	if got := viewerModeTabTitle(st, "file", "File"); got != "File - tik_tok.jpg" {
		t.Fatalf("file tab=%q", got)
	}
	if got := viewerModeTabTitle(st, "hex", "Hex"); got != "Hex" {
		t.Fatalf("inactive hex tab=%q", got)
	}
	st.mode = "hex"
	if got := viewerModeTabTitle(st, "hex", "Hex"); got != "Hex - tik_tok.jpg" {
		t.Fatalf("hex tab=%q", got)
	}
	st.mode = "command"
	if got := viewerModeTabTitle(st, "command", "Cmd"); got != "Cmd - tik_tok.jpg" {
		t.Fatalf("command tab=%q", got)
	}
	st.historyOpen = true
	if got := viewerModeTabTitle(st, "command", "Cmd"); got != "Cmd" {
		t.Fatalf("history-open command tab=%q", got)
	}
}

func TestFileViewerActiveModeTabExpandsForFullFilename(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Tabs.WidthMode = "fixed"
	cfg.Tabs.FixedWidthDp = 48
	ui := NewUI(cfg)
	th := material.NewTheme()
	st := &fileViewerState{name: "a-fully-visible-image-name.jpg", mode: "file"}
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(900, 24)},
	}
	ui.layoutFileViewerModeTabs(th, gtx, st, 24)
	wantMin := tabStripTitleTextWidth(th, gtx, ui.tabStripTypeface(), ui.tabStripTextSize(), "File - "+st.name)
	if got := st.activeTabRect.Dx(); got < wantMin {
		t.Fatalf("active tab width=%d want at least full title width %d", got, wantMin)
	}
}

func TestFileViewerHistoryUsesInterfaceFontAndFullWidth(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Interface.Typeface = resources.BundledFontFamilyIosevkaNerdFontMono
	cfg.Interface.FontSizeSp = 16
	cfg.Viewer.Typeface = resources.BundledFontFamilyHackNerdFontMono
	ui := NewUI(cfg)
	th := material.NewTheme()
	st := &fileViewerState{mode: "command", historyOpen: true}

	if got, want := ui.fileViewerHistoryTypeface(), font.Typeface(cfg.Interface.Typeface); got != want {
		t.Fatalf("history typeface=%q want interface typeface %q", got, want)
	}
	if got, want := ui.fileViewerHistoryTextSize(), ui.scaleInterfaceFontSize(9); got != want {
		t.Fatalf("history text size=%v want interface-scaled %v", got, want)
	}

	var r input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(720, 120),
		},
	}
	dims := ui.layoutFileViewerHistoryList(th, gtx, st, []string{"tail -f {path}", "tail -n200 {path}"})
	if dims.Size.X != gtx.Constraints.Max.X {
		t.Fatalf("history width=%d want full available width %d", dims.Size.X, gtx.Constraints.Max.X)
	}
}

func TestFileViewerHeaderDetailsDropPlainFileSizeStatus(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		status:    "file: 74748 bytes",
		updatedAt: time.Date(2026, time.March, 8, 11, 4, 22, 0, time.UTC),
	}

	parts := ui.fileViewerHeaderDetails(st)
	if len(parts) != 0 {
		t.Fatalf("detail parts=%d want 0", len(parts))
	}
}

func TestFileViewerHeaderDetailsKeepStreamingStatus(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		status:    "streaming",
		updatedAt: time.Date(2026, time.March, 8, 11, 4, 22, 0, time.UTC),
	}

	parts := ui.fileViewerHeaderDetails(st)
	if len(parts) != 1 {
		t.Fatalf("detail parts=%d want 1", len(parts))
	}
	if got := parts[0].Text; got != "streaming" {
		t.Fatalf("status part=%q want %q", got, "streaming")
	}
}

func TestViewerImageZoomLabelUsesCurrentZoom(t *testing.T) {
	st := &fileViewerState{detectedImagePreview: true}
	st.imageView.zoom = 1.25

	if got := viewerImageZoomLabel(st); got != "125%" {
		t.Fatalf("zoom label=%q want %q", got, "125%")
	}
}

func TestViewerPDFPageLabelUsesScrolledDocumentPage(t *testing.T) {
	st := &fileViewerState{
		detectedImagePreview:  true,
		imagePreviewFormat:    "pdf",
		imagePreviewPage:      1,
		imagePreviewPageCount: 9,
	}
	sizes := make([]viewerPDFPageSize, 9)
	for i := range sizes {
		sizes[i] = viewerPDFPageSize{W: 612, H: 792}
	}
	st.pdfDoc.viewportRect = image.Rect(0, 0, 200, 260)
	st.pdfDoc.configure(viewerPDFDocInfo{PageCount: 9, PageSizes: sizes})
	st.pdfDoc.scrollToPage(6)

	if got := viewerPDFPageLabel(st); got != "Page 7/9" {
		t.Fatalf("page label=%q want %q", got, "Page 7/9")
	}
}

func TestViewerTOCAccordionStartsAtRootsAndKeepsOneBranchOpen(t *testing.T) {
	st := &fileViewerState{}
	st.pdfDoc.toc = normalizeViewerPDFTOC([]viewerPDFTOCEntry{
		{Title: "Chapter A", Page: 0, Level: 0},
		{Title: "Section A.1", Page: 1, Level: 1},
		{Title: "Topic A.1.a", Page: 2, Level: 2},
		{Title: "Section A.2", Page: 3, Level: 1},
		{Title: "Chapter B", Page: 4, Level: 0},
		{Title: "Section B.1", Page: 5, Level: 1},
	})

	assertVisible := func(want ...int) {
		t.Helper()
		got := viewerTOCVisibleIndices(st)
		if len(got) != len(want) {
			t.Fatalf("visible=%v want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("visible=%v want %v", got, want)
			}
		}
	}

	assertVisible(0, 4)
	if !toggleViewerTOCEntry(st, st.pdfDoc.toc[0]) {
		t.Fatal("root entry should expand")
	}
	assertVisible(0, 1, 3, 4)
	if !toggleViewerTOCEntry(st, st.pdfDoc.toc[1]) {
		t.Fatal("nested entry should expand")
	}
	assertVisible(0, 1, 2, 3, 4)

	// Expanding another root closes the previous root and all of its nested
	// expansion state while keeping both root rows visible.
	if !toggleViewerTOCEntry(st, st.pdfDoc.toc[4]) {
		t.Fatal("second root entry should expand")
	}
	assertVisible(0, 4, 5)
	if viewerTOCEntryExpanded(st, st.pdfDoc.toc[0]) || viewerTOCEntryExpanded(st, st.pdfDoc.toc[1]) {
		t.Fatal("opening a sibling branch should close the previous branch")
	}
	if !viewerTOCEntryExpanded(st, st.pdfDoc.toc[4]) {
		t.Fatal("second root should be marked expanded")
	}

	if !toggleViewerTOCEntry(st, st.pdfDoc.toc[4]) {
		t.Fatal("expanded root should collapse")
	}
	assertVisible(0, 4)
}

func TestNormalizeViewerTOCBuildsStableHierarchy(t *testing.T) {
	toc := normalizeViewerPDFTOC([]viewerPDFTOCEntry{
		{Title: "Root", Level: 0},
		{Title: "Child", Level: 1},
		{Title: "Grandchild", Level: 4},
		{Title: "Other root", Level: 0},
	})
	if len(toc) != 4 {
		t.Fatalf("TOC length=%d want 4", len(toc))
	}
	if toc[0].ID == "" || !toc[0].HasChildren {
		t.Fatalf("root=%+v want stable ID and children", toc[0])
	}
	if toc[1].ParentID != toc[0].ID || !toc[1].HasChildren {
		t.Fatalf("child=%+v want parent %q and children", toc[1], toc[0].ID)
	}
	if toc[2].Level != 2 || toc[2].ParentID != toc[1].ID {
		t.Fatalf("grandchild=%+v want clamped level 2 and parent %q", toc[2], toc[1].ID)
	}
	if toc[3].Level != 0 || toc[3].ParentID != "" || toc[3].HasChildren {
		t.Fatalf("other root=%+v", toc[3])
	}
}

func TestViewerTOCDisclosureAndTitleHaveIndependentActions(t *testing.T) {
	st := &fileViewerState{}
	st.mode = "file"
	st.tocMenuOpen = true
	st.pdfDoc.viewportRect = image.Rect(0, 0, 300, 240)
	st.pdfDoc.configure(viewerPDFDocInfo{
		PageCount: 2,
		PageSizes: []viewerPDFPageSize{{W: 612, H: 792}, {W: 612, H: 792}},
	})
	st.pdfDoc.toc = normalizeViewerPDFTOC([]viewerPDFTOCEntry{
		{Title: "Chapter", Page: 0, Level: 0},
		{Title: "Leaf", Page: 1, Level: 1},
	})

	root := st.pdfDoc.toc[0]
	if !viewerTOCEntryNavigates(st, root) {
		t.Fatal("a branch title with a valid destination should navigate")
	}
	if !viewerTOCEntryNavigates(st, st.pdfDoc.toc[1]) {
		t.Fatal("a leaf with a valid destination should navigate")
	}
	if glyph := viewerTOCDisclosureGlyph(st, root); glyph != "→" {
		t.Fatalf("collapsed disclosure=%q want right arrow", glyph)
	}
	if !st.pdfDoc.scrollToPage(1) {
		t.Fatal("test document should scroll to page 2")
	}
	before := st.pdfDoc.scrollY

	ensureViewerTOCClicks(st)
	st.tocDisclosureClicks[0].Click()
	ui := NewUI(fm.DefaultConfig())
	gtx := layout.Context{Ops: new(op.Ops), Now: time.Now()}
	ui.handleFileViewerTOCClicks(gtx, st)
	if st.pdfDoc.scrollY != before {
		t.Fatalf("disclosure changed scrollY=%f want %f", st.pdfDoc.scrollY, before)
	}
	if !st.tocMenuOpen || !viewerTOCEntryExpanded(st, root) {
		t.Fatal("disclosure should expand the branch and keep the TOC open")
	}
	if glyph := viewerTOCDisclosureGlyph(st, root); glyph != "↓" {
		t.Fatalf("expanded disclosure=%q want down arrow", glyph)
	}

	st.tocClicks[0].Click()
	ui.handleFileViewerTOCClicks(gtx, st)
	if got := st.pdfDoc.currentPage(); got != 0 {
		t.Fatalf("root title navigated to page=%d want 0", got)
	}
	if st.tocMenuOpen {
		t.Fatal("title navigation should close the TOC")
	}
}

func TestFileViewerHeaderStatusTextShowsRefreshingForFiniteCommandReload(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	got, _ := ui.fileViewerHeaderStatusText(&fileViewerState{
		mode:            "command",
		status:          "streaming",
		commandInfinite: false,
		autoRefresh:     true,
		loading:         true,
		content:         "existing output",
	})
	if got != "refreshing" {
		t.Fatalf("status=%q want %q", got, "refreshing")
	}
}

func TestFileViewerHeaderStatusTextShowsNoRefreshForFiniteCommandWhenDisabled(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	got, _ := ui.fileViewerHeaderStatusText(&fileViewerState{
		mode:            "command",
		status:          "streaming",
		commandInfinite: false,
		autoRefresh:     false,
	})
	if got != "no-refresh" {
		t.Fatalf("status=%q want %q", got, "no-refresh")
	}
}

func TestFileViewerHeaderStatusTextStaysRefreshingForFiniteCommandWhenIdle(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	got, _ := ui.fileViewerHeaderStatusText(&fileViewerState{
		mode:            "command",
		status:          "streaming",
		commandInfinite: false,
		autoRefresh:     true,
	})
	if got != "refreshing" {
		t.Fatalf("status=%q want %q", got, "refreshing")
	}
}

func TestFileViewerHeaderStatusTextKeepsStreamingForInfiniteCommand(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	got, _ := ui.fileViewerHeaderStatusText(&fileViewerState{
		mode:            "command",
		commandInfinite: true,
		autoRefresh:     false,
	})
	if got != "streaming" {
		t.Fatalf("status=%q want %q", got, "streaming")
	}
}

func TestFileViewerFindFocusedEnterStepsNextMatch(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Date(2026, time.March, 21, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "viewer.txt")
	if err := os.WriteFile(path, []byte("alpha beta alpha"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	st := &fileViewerState{
		path:     path,
		name:     "viewer.txt",
		mode:     "file",
		content:  "alpha beta alpha",
		status:   "file: 16 bytes",
		resultCh: make(chan fileViewerResult, 1),
	}
	st.find.editor.SingleLine = true
	st.find.editor.Submit = false
	st.find.resultCh = make(chan fileViewerFindResult, 1)
	st.captureWatchState()
	ui.fileViewer = st
	st.find.editor.SetText("alpha")
	ui.openFileViewerFind(now)

	router := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(1024, 720),
		},
		Now: now,
	}
	frame := func(now time.Time) {
		gtx.Now = now
		gtx.Ops.Reset()
		ui.layoutFileViewer(th, gtx)
		router.Frame(gtx.Ops)
	}

	frame(now)
	frame(now.Add(time.Millisecond))
	frame(now.Add(2 * time.Millisecond))
	if !gtx.Focused(&st.find.editor) {
		t.Fatal("find editor did not gain focus")
	}
	if st.find.index != 0 {
		t.Fatalf("initial find index=%d want 0", st.find.index)
	}

	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(now.Add(3 * time.Millisecond))

	if got := st.find.editor.Text(); got != "alpha" {
		t.Fatalf("find text=%q want %q", got, "alpha")
	}
	if st.find.index != 1 {
		t.Fatalf("find index after Enter=%d want 1", st.find.index)
	}
	if st.find.status != "2/2" {
		t.Fatalf("find status after Enter=%q want %q", st.find.status, "2/2")
	}
}

func TestViewerShowsAutoRefreshButtonOnlyForNonStreamingCommand(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())

	if ui.viewerShowsAutoRefreshButton(&fileViewerState{mode: "file"}) {
		t.Fatal("file mode should not show refresh button")
	}
	if !ui.viewerShowsAutoRefreshButton(&fileViewerState{mode: "command"}) {
		t.Fatal("non-streaming command mode should show refresh button")
	}
	if ui.viewerShowsAutoRefreshButton(&fileViewerState{mode: "command", commandInfinite: true}) {
		t.Fatal("streaming command mode should not show refresh button")
	}
}

func TestFileViewerInlineCommandDisplayWidthLeavesFullTextPadding(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	st := &fileViewerState{
		mode:    "command",
		command: "cat {fullpath} --with-longer-token",
	}
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(960, 48),
		},
	}

	dims := ui.layoutFileViewerInlineCommand(th, gtx, st, 24)

	lbl := material.Body2(th, st.command)
	lbl.Font.Typeface = ui.tabStripTypeface()
	lbl.Font.Weight = font.Medium
	lbl.TextSize = ui.tabStripTextSize()
	labelW := measureLabelUnconstrained(gtx, lbl).Size.X
	wantPadding := gtx.Dp(unit.Dp(viewerInlineCommandMeasurePaddingDp))
	if got := dims.Size.X - labelW; got < wantPadding {
		t.Fatalf("inline command padding=%dpx want at least %dpx", got, wantPadding)
	}
}

func TestFileViewerInlineCommandKeepsPlateGeometryWhileEditing(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	command := "tail -f {path}"
	st := &fileViewerState{mode: "command", command: command}
	router := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(960, 48),
		},
	}

	display := ui.layoutFileViewerInlineCommand(th, gtx, st, 24)
	gtx.Ops.Reset()
	st.commandEditOn = true
	st.commandFocus = true
	st.commandEditor.SetText(command)
	editing := ui.layoutFileViewerInlineCommand(th, gtx, st, 24)

	if editing.Size != display.Size {
		t.Fatalf("inline command geometry changed while editing: display=%v editing=%v", display.Size, editing.Size)
	}
}

func TestFileViewerOverlayTextUsesWidthAsMaximum(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(400, 40),
		},
	}

	unconstrained := ui.layoutFileViewerOverlayText(th, gtx, "protocols.yaml", ui.fileViewerTheme().TooltipText, 0)
	constrained := ui.layoutFileViewerOverlayText(th, gtx, "protocols.yaml", ui.fileViewerTheme().TooltipText, 200)

	if constrained.Size.X != unconstrained.Size.X {
		t.Fatalf("overlay text width=%d want intrinsic width %d", constrained.Size.X, unconstrained.Size.X)
	}
}

func TestFileViewerOverlayBarKeepsPreviewControlsCompact(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(500, 60),
		},
	}
	st := &fileViewerState{
		name:                  "preview.pdf",
		mode:                  "file",
		detectedImagePreview:  true,
		imagePreviewFormat:    "pdf",
		imagePreviewPage:      2,
		imagePreviewPageCount: 12,
	}
	st.imageView.zoom = 1.25

	dims := ui.layoutFileViewerOverlayBar(th, gtx, st)

	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("overlay dimensions=%v want visible bar", dims.Size)
	}
	if maxH := gtx.Dp(unit.Dp(20)); dims.Size.Y > maxH {
		t.Fatalf("overlay height=%d want <= %d", dims.Size.Y, maxH)
	}
}

func TestFileViewerOverlayStatusDropsPlainFileSizeStatus(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	got, _ := ui.fileViewerOverlayStatusText(&fileViewerState{status: "file: 2545 bytes"})
	if got != "" {
		t.Fatalf("overlay status=%q want empty", got)
	}
}

func testPointerContext() (layout.Context, *input.Router) {
	router := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
	}
	return gtx, router
}

func registerPointerTag(router *input.Router, ops *op.Ops, tag event.Tag) {
	ops.Reset()
	defer clip.Rect(image.Rect(0, 0, 200, 200)).Push(ops).Pop()
	pass := pointer.PassOp{}.Push(ops)
	event.Op(ops, tag)
	pass.Pop()
	router.Frame(ops)
}

func primePointerFilter(router *input.Router, tag event.Tag) {
	for {
		_, ok := router.Event(pointer.Filter{Target: tag, Kinds: pointer.Press})
		if !ok {
			return
		}
	}
}

func primePointerScrollFilter(router *input.Router, tag event.Tag, scrollX, scrollY pointer.ScrollRange) {
	for {
		_, ok := router.Event(pointer.Filter{
			Target:  tag,
			Kinds:   pointer.Scroll,
			ScrollX: scrollX,
			ScrollY: scrollY,
		})
		if !ok {
			return
		}
	}
}
