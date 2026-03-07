package ui

import (
	"errors"
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"hexone/fm"
)

func TestEditorContextMenuPasteTargetsRightClickedEditor(t *testing.T) {
	oldRead := readEditorContextClipboardText
	readEditorContextClipboardText = func() (string, error) {
		return "-paste-", nil
	}
	defer func() {
		readEditorContextClipboardText = oldRead
	}()

	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	var (
		r   input.Router
		ed1 widget.Editor
		ed2 widget.Editor
		ed3 widget.Editor
	)
	for _, ed := range []*widget.Editor{&ed1, &ed2, &ed3} {
		ed.SingleLine = true
		ed.Submit = false
	}
	ed1.SetText("one")
	ed2.SetText("two")
	ed3.SetText("three")
	ed1.SetCaret(ed1.Len(), ed1.Len())
	ed2.SetCaret(ed2.Len(), ed2.Len())
	ed3.SetCaret(ed3.Len(), ed3.Len())

	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(420, 220),
		},
	}

	layoutFrame := func() map[string]image.Rectangle {
		rects := make(map[string]image.Rectangle, 3)
		ui.handleEditorContextMenuGlobalPresses(gtx)
		ui.handleEditorContextMenuClipboardEvents(gtx)

		y := 0
		layoutEditor := func(id string, ed *widget.Editor) {
			off := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			gtx2 := gtx
			gtx2.Constraints = layout.Constraints{
				Max: image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y-y),
			}
			dims := ui.layoutEditorWithContextMenu(th, gtx2, id, ed, true, func(gtx layout.Context) layout.Dimensions {
				me := material.Editor(th, ed, "")
				me.Font.Typeface = ui.mainTypeface()
				me.TextSize = scaleConfigFontSize(ui.fmCfg, 10)
				me.Color = txtColor
				me.HintColor = hintColor
				return layoutNeutralEditorBox(gtx, gtx.Focused(ed), true, me.Layout)
			})
			off.Pop()
			rects[id] = image.Rectangle{Min: image.Pt(0, y), Max: image.Pt(dims.Size.X, y+dims.Size.Y)}
			y += dims.Size.Y + 10
		}

		layoutEditor("ed1", &ed1)
		layoutEditor("ed2", &ed2)
		layoutEditor("ed3", &ed3)

		ui.layoutEditorContextMenuOverlay(th, gtx)
		ui.registerEditorContextMenuGlobalPointer(gtx)
		ui.registerEditorContextMenuClipboardTarget(gtx)
		return rects
	}

	frame := func() map[string]image.Rectangle {
		gtx.Ops.Reset()
		rects := layoutFrame()
		r.Frame(gtx.Ops)
		return rects
	}

	rects := frame()
	gtx.Execute(key.FocusCmd{Tag: &ed1})
	frame()
	if !gtx.Focused(&ed1) {
		t.Fatal("first editor did not gain initial focus")
	}

	third := rects["ed3"]
	if first := rects["ed1"]; third.Min.Y < first.Max.Y {
		t.Fatalf("test layout invalid: ed1=%v ed3=%v", first, third)
	}
	r.Queue(pointer.Event{
		Kind:     pointer.Press,
		Buttons:  pointer.ButtonSecondary,
		Position: f32.Pt(float32(third.Min.X+12), float32(third.Min.Y+12)),
	})
	frame()

	if ui.editorMenuTarget != &ed3 {
		got := "unknown"
		switch ui.editorMenuTarget {
		case &ed1:
			got = "ed1"
		case &ed2:
			got = "ed2"
		case &ed3:
			got = "ed3"
		}
		t.Fatalf("context menu bound to wrong editor: got %s (%p) want ed3 (%p), openID=%q, rects=%v", got, ui.editorMenuTarget, &ed3, ui.editorMenuOpenID, rects)
	}
	if ui.editorMenuOpenID != "ed3" {
		t.Fatalf("context menu opened for wrong id: got %q want %q", ui.editorMenuOpenID, "ed3")
	}

	pastePoint := image.Pt(
		ui.editorMenuRect.Min.X+8,
		ui.editorMenuRect.Min.Y+ui.editorContextMenuRowHeight(gtx)+2+ui.editorContextMenuRowHeight(gtx)/2,
	)
	r.Queue(pointer.Event{
		Kind:     pointer.Press,
		Buttons:  pointer.ButtonPrimary,
		Position: f32.Pt(float32(pastePoint.X), float32(pastePoint.Y)),
	})
	frame()

	if got := ed1.Text(); got != "one" {
		t.Fatalf("first editor changed unexpectedly: got %q", got)
	}
	if got := ed2.Text(); got != "two" {
		t.Fatalf("second editor changed unexpectedly: got %q", got)
	}
	if got := ed3.Text(); got != "three-paste-" {
		t.Fatalf("third editor did not receive paste: got %q", got)
	}
}

func TestEditorContextMenuClipboardFallbackTargetsStoredEditor(t *testing.T) {
	oldRead := readEditorContextClipboardText
	readEditorContextClipboardText = func() (string, error) {
		return "", errors.New("force async fallback")
	}
	defer func() {
		readEditorContextClipboardText = oldRead
	}()

	ui := NewUI(fm.DefaultConfig())
	var ed1, ed2 widget.Editor
	ed1.SetText("left")
	ed2.SetText("right")
	ui.editorMenuClipboardTarget = &ed2

	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
	}

	if ok := ui.pasteEditorText(gtx, &ed2, true); !ok {
		t.Fatal("pasteEditorText returned false")
	}
	if ui.editorMenuClipboardTarget != &ed2 {
		t.Fatal("clipboard fallback did not keep explicit target")
	}
	if ed1.Text() != "left" {
		t.Fatalf("first editor changed unexpectedly: got %q", ed1.Text())
	}
}

func TestEditorContextMenuPasteAppendsWhenCaretWasNotExplicit(t *testing.T) {
	oldRead := readEditorContextClipboardText
	readEditorContextClipboardText = func() (string, error) {
		return "-paste-", nil
	}
	defer func() {
		readEditorContextClipboardText = oldRead
	}()

	ui := NewUI(fm.DefaultConfig())
	var ed widget.Editor
	ed.SetText("abc")
	ed.SetCaret(0, 0)
	ui.editorMenuUseExplicitCaret = false

	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
	}

	if ok := ui.pasteEditorText(gtx, &ed, true); !ok {
		t.Fatal("pasteEditorText returned false")
	}
	if got := ed.Text(); got != "abc-paste-" {
		t.Fatalf("paste should append by default, got %q", got)
	}
}

func TestEditorContextMenuPasteUsesCaretWhenExplicit(t *testing.T) {
	oldRead := readEditorContextClipboardText
	readEditorContextClipboardText = func() (string, error) {
		return "-paste-", nil
	}
	defer func() {
		readEditorContextClipboardText = oldRead
	}()

	ui := NewUI(fm.DefaultConfig())
	var ed widget.Editor
	ed.SetText("abc")
	ed.SetCaret(1, 1)
	ui.editorMenuUseExplicitCaret = true

	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
	}

	if ok := ui.pasteEditorText(gtx, &ed, true); !ok {
		t.Fatal("pasteEditorText returned false")
	}
	if got := ed.Text(); got != "a-paste-bc" {
		t.Fatalf("paste should respect explicit caret, got %q", got)
	}
}

func TestEditorContextMenuOpenTargetsRightClickedBoxWithoutEditorWidget(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	var (
		r   input.Router
		ed1 widget.Editor
		ed2 widget.Editor
		ed3 widget.Editor
	)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(420, 220),
		},
	}

	layoutFrame := func() map[string]image.Rectangle {
		rects := make(map[string]image.Rectangle, 3)
		ui.handleEditorContextMenuGlobalPresses(gtx)

		y := 0
		layoutBox := func(id string, ed *widget.Editor) {
			off := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			gtx2 := gtx
			gtx2.Constraints = layout.Constraints{
				Max: image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y-y),
			}
			dims := ui.layoutEditorWithContextMenu(th, gtx2, id, ed, true, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(80, 20)}
			})
			off.Pop()
			rects[id] = image.Rectangle{Min: image.Pt(0, y), Max: image.Pt(dims.Size.X, y+dims.Size.Y)}
			y += dims.Size.Y + 10
		}

		layoutBox("ed1", &ed1)
		layoutBox("ed2", &ed2)
		layoutBox("ed3", &ed3)
		ui.layoutEditorContextMenuOverlay(th, gtx)
		ui.registerEditorContextMenuGlobalPointer(gtx)
		return rects
	}

	frame := func() map[string]image.Rectangle {
		gtx.Ops.Reset()
		rects := layoutFrame()
		r.Frame(gtx.Ops)
		return rects
	}

	rects := frame()
	third := rects["ed3"]
	r.Queue(pointer.Event{
		Kind:     pointer.Press,
		Buttons:  pointer.ButtonSecondary,
		Position: f32.Pt(float32(third.Min.X+12), float32(third.Min.Y+12)),
	})
	frame()

	if ui.editorMenuOpenID != "ed3" || ui.editorMenuTarget != &ed3 {
		t.Fatalf("plain box target mismatch: openID=%q target=%p rects=%v", ui.editorMenuOpenID, ui.editorMenuTarget, rects)
	}
}

func TestEditorContextMenuLowLevelPointerAreas(t *testing.T) {
	var (
		r    input.Router
		ops  op.Ops
		tag1 = new(int)
		tag2 = new(int)
		tag3 = new(int)
	)
	filter := func(tag event.Tag) pointer.Filter {
		return pointer.Filter{Target: tag, Kinds: pointer.Press}
	}
	drain := func(filters ...event.Filter) []event.Event {
		var out []event.Event
		for {
			ev, ok := r.Event(filters...)
			if !ok {
				return out
			}
			out = append(out, ev)
		}
	}

	drain(filter(tag1))
	drain(filter(tag2))
	drain(filter(tag3))

	addBox := func(tag event.Tag, y int) {
		off := op.Offset(image.Pt(0, y)).Push(&ops)
		defer off.Pop()
		defer clip.Rect(image.Rect(0, 0, 80, 20)).Push(&ops).Pop()
		pass := pointer.PassOp{}.Push(&ops)
		event.Op(&ops, tag)
		pass.Pop()
	}

	ops.Reset()
	addBox(tag1, 0)
	addBox(tag2, 30)
	addBox(tag3, 60)
	r.Frame(&ops)

	r.Queue(pointer.Event{
		Kind:     pointer.Press,
		Buttons:  pointer.ButtonSecondary,
		Position: f32.Pt(12, 72),
	})

	if got := len(drain(filter(tag1))); got != 0 {
		t.Fatalf("tag1 unexpectedly received %d events", got)
	}
	if got := len(drain(filter(tag2))); got != 0 {
		t.Fatalf("tag2 unexpectedly received %d events", got)
	}
	if got := len(drain(filter(tag3))); got != 1 {
		t.Fatalf("tag3 received %d events, want 1", got)
	}
}

func TestEditorContextMenuLowLevelPointerAreasWithGlobalPass(t *testing.T) {
	var (
		r         input.Router
		ops       op.Ops
		globalTag = new(int)
		tag1      = new(int)
		tag2      = new(int)
		tag3      = new(int)
	)
	filter := func(tag event.Tag) pointer.Filter {
		return pointer.Filter{Target: tag, Kinds: pointer.Press}
	}
	drain := func(filters ...event.Filter) []event.Event {
		var out []event.Event
		for {
			ev, ok := r.Event(filters...)
			if !ok {
				return out
			}
			out = append(out, ev)
		}
	}

	drain(filter(globalTag))
	drain(filter(tag1))
	drain(filter(tag2))
	drain(filter(tag3))

	addBox := func(tag event.Tag, y int) {
		off := op.Offset(image.Pt(0, y)).Push(&ops)
		defer off.Pop()
		defer clip.Rect(image.Rect(0, 0, 80, 20)).Push(&ops).Pop()
		pass := pointer.PassOp{}.Push(&ops)
		event.Op(&ops, tag)
		pass.Pop()
	}

	ops.Reset()
	addBox(tag1, 0)
	addBox(tag2, 30)
	addBox(tag3, 60)
	defer clip.Rect(image.Rect(0, 0, 420, 220)).Push(&ops).Pop()
	pass := pointer.PassOp{}.Push(&ops)
	event.Op(&ops, globalTag)
	pass.Pop()
	r.Frame(&ops)

	r.Queue(pointer.Event{
		Kind:     pointer.Press,
		Buttons:  pointer.ButtonSecondary,
		Position: f32.Pt(12, 72),
	})

	if got := len(drain(filter(globalTag))); got != 1 {
		t.Fatalf("global tag received %d events, want 1", got)
	}
	if got := len(drain(filter(tag1))); got != 0 {
		t.Fatalf("tag1 unexpectedly received %d events with global pass", got)
	}
	if got := len(drain(filter(tag2))); got != 0 {
		t.Fatalf("tag2 unexpectedly received %d events with global pass", got)
	}
	if got := len(drain(filter(tag3))); got != 1 {
		t.Fatalf("tag3 received %d events with global pass, want 1", got)
	}
}
