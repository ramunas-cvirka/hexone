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

func TestViewerPDFPageLabelUsesDraggedTargetPage(t *testing.T) {
	st := &fileViewerState{
		detectedImagePreview:  true,
		imagePreviewFormat:    "pdf",
		imagePreviewPage:      1,
		imagePreviewPageCount: 9,
	}
	st.imageView.pdfDragging = true
	st.imageView.pdfDragPage = 6

	if got := viewerPDFPageLabel(st); got != "Page 7/9" {
		t.Fatalf("page label=%q want %q", got, "Page 7/9")
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
