package ui

import (
	"image"
	"testing"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
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
