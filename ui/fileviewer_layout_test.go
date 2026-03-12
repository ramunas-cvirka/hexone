package ui

import (
	"image"
	"testing"
	"time"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/input"
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

func TestFileViewerHeaderDetailsDropPlainFileSizeStatus(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		status:    "file: 74748 bytes",
		updatedAt: time.Date(2026, time.March, 8, 11, 4, 22, 0, time.UTC),
	}

	parts := ui.fileViewerHeaderDetails(st)
	if len(parts) != 1 {
		t.Fatalf("detail parts=%d want 1", len(parts))
	}
	if got := parts[0].Text; got != "updated at 11:04:22" {
		t.Fatalf("detail text=%q want %q", got, "updated at 11:04:22")
	}
}

func TestFileViewerHeaderDetailsKeepStreamingStatus(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		status:    "streaming",
		updatedAt: time.Date(2026, time.March, 8, 11, 4, 22, 0, time.UTC),
	}

	parts := ui.fileViewerHeaderDetails(st)
	if len(parts) != 2 {
		t.Fatalf("detail parts=%d want 2", len(parts))
	}
	if got := parts[0].Text; got != "streaming" {
		t.Fatalf("status part=%q want %q", got, "streaming")
	}
	if got := parts[1].Text; got != "updated at 11:04:22" {
		t.Fatalf("updated part=%q want %q", got, "updated at 11:04:22")
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
	lbl.Font.Typeface = ui.viewerTypeface()
	lbl.Font.Weight = font.Medium
	lbl.TextSize = ui.viewerTextSize()
	labelW := measureLabelUnconstrained(gtx, lbl).Size.X
	wantPadding := gtx.Dp(unit.Dp(viewerInlineCommandDisplayInsetDp * 2))
	if got := dims.Size.X - labelW; got < wantPadding {
		t.Fatalf("inline command padding=%dpx want at least %dpx", got, wantPadding)
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
