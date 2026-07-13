// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"hexone/fm"
)

type blockingPDFFindRenderer struct {
	started chan struct{}
	release chan struct{}
	pages   []viewerPDFPageText
}

func (r *blockingPDFFindRenderer) Available() bool { return true }

func (r *blockingPDFFindRenderer) RenderPage(req viewerPDFRenderRequest) (viewerPDFRenderResult, error) {
	return viewerPDFRenderResult{
		Image:     image.NewNRGBA(image.Rect(0, 0, req.Width, req.Width*4/3)),
		Page:      req.Page,
		PageCount: len(r.pages),
	}, nil
}

func (r *blockingPDFFindRenderer) DocInfo(viewerPDFRenderRequest) (viewerPDFDocInfo, error) {
	sizes := make([]viewerPDFPageSize, len(r.pages))
	for i := range sizes {
		sizes[i] = viewerPDFPageSize{W: 100, H: 160}
	}
	return viewerPDFDocInfo{PageCount: len(sizes), PageSizes: sizes}, nil
}

func (r *blockingPDFFindRenderer) PageText(req viewerPDFRenderRequest) (viewerPDFPageText, error) {
	if req.Page == 0 && r.started != nil {
		select {
		case r.started <- struct{}{}:
		default:
		}
		<-r.release
	}
	return r.pages[req.Page], nil
}

func (r *blockingPDFFindRenderer) PageLinks(req viewerPDFRenderRequest) (viewerPDFPageLinks, error) {
	return viewerPDFPageLinks{Page: req.Page}, nil
}

func (r *blockingPDFFindRenderer) TOC(viewerPDFRenderRequest) ([]viewerPDFTOCEntry, error) {
	return nil, nil
}

func testPDFFindPage(page int, value string) viewerPDFPageText {
	chars := make([]viewerPDFTextChar, 0, len([]rune(value)))
	for i, r := range value {
		chars = append(chars, viewerPDFTextChar{
			Rune: r, Left: 5 + float64(i)*5, Top: 20, Right: 10 + float64(i)*5, Bottom: 30,
		})
	}
	return viewerPDFPageText{Page: page, Chars: chars}
}

func TestViewerFindTextMatchesAllowsOverlaps(t *testing.T) {
	got := viewerFindTextMatches("banana", "ana")

	if len(got) != 2 {
		t.Fatalf("len(matches)=%d want 2", len(got))
	}
	if got[0].Start != 1 || got[0].End != 4 {
		t.Fatalf("first match=%+v want {Start:1 End:4}", got[0])
	}
	if got[1].Start != 3 || got[1].End != 6 {
		t.Fatalf("second match=%+v want {Start:3 End:6}", got[1])
	}
}

func TestViewerFindTextMatchesIncludesLineAndCompactSnippet(t *testing.T) {
	got := viewerFindTextMatches("header\nsecond needle with context\nfooter", "needle")
	if len(got) != 1 {
		t.Fatalf("matches=%d want 1", len(got))
	}
	if got[0].Line != 2 {
		t.Fatalf("line=%d want 2", got[0].Line)
	}
	if got[0].Snippet != "second needle with context" {
		t.Fatalf("snippet=%q", got[0].Snippet)
	}
}

func TestViewerFindBytesAllReturnsOverlapsAndHonorsLimit(t *testing.T) {
	data := []byte("aaaaa")
	src := viewerFindChunkSource{
		size: int64(len(data)),
		read: func(start, length int64) ([]byte, error) {
			end := start + length
			if end > int64(len(data)) {
				end = int64(len(data))
			}
			return append([]byte(nil), data[start:end]...), nil
		},
	}
	offsets, limited, err := viewerFindBytesAll(context.Background(), src, []byte("aa"), 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(offsets) != 3 || offsets[0] != 0 || offsets[1] != 1 || offsets[2] != 2 {
		t.Fatalf("offsets=%v want [0 1 2]", offsets)
	}
	if !limited {
		t.Fatal("expected capped result set to be marked limited")
	}
}

func TestViewerRemoteSearchOffsetsParsesMultipleSortedOffsets(t *testing.T) {
	offsets, valid := viewerRemoteSearchOffsets("18:needle\n2:needle\n18:needle\n", 64, 10)
	if !valid {
		t.Fatal("expected valid multi-result output")
	}
	if len(offsets) != 2 || offsets[0] != 2 || offsets[1] != 18 {
		t.Fatalf("offsets=%v want sorted unique offsets", offsets)
	}
}

func TestViewerRunRemoteSearchAllRequestsMultipleResults(t *testing.T) {
	prev := runViewerRemoteSearchCommandFunc
	defer func() { runViewerRemoteSearchCommandFunc = prev }()
	var gotCmd string
	runViewerRemoteSearchCommandFunc = func(_ context.Context, _ *paneSSHSession, cmdline string, _ viewerShellSpec) (string, error) {
		gotCmd = cmdline
		return "2:needle\n18:needle\n", nil
	}
	spec := viewerRemoteSearchSpec{
		template: fm.DefaultViewerRemoteSearchCommand,
		mode:     fm.ViewerRemoteSearchModeRemote,
		shell:    resolveViewerShell("sh", true),
	}
	offsets, used := viewerRunRemoteSearchAll(context.Background(), "/tmp/sample", &paneSSHSession{}, []byte("needle"), spec, 64, 59, 20)
	if !used || len(offsets) != 2 || offsets[0] != 2 || offsets[1] != 18 {
		t.Fatalf("offsets=%v used=%v", offsets, used)
	}
	if strings.Contains(gotCmd, "-m 1") {
		t.Fatalf("multi-result command unexpectedly limited to one match: %q", gotCmd)
	}
	if !strings.Contains(gotCmd, "head -n 20") {
		t.Fatalf("multi-result command missing result cap: %q", gotCmd)
	}
}

func TestViewerHexFindPreviewSwitchesBetweenTextAndHex(t *testing.T) {
	raw := []byte{0x00, 'h', 'e', 'l', 'l', 'o', 0x01, 0x02}
	st := &fileViewerState{hex: &hexViewerState{
		bufferStart: 0,
		buffer:      raw,
	}}
	match := viewerHexFindMatch{Start: 1, Length: 5, PreviewBytes: append([]byte(nil), raw...)}
	st.find.hexMatches = []viewerHexFindMatch{match}

	textPreview := viewerHexFindSnippet(st, match)
	if !strings.Contains(textPreview, "hello") || strings.Contains(textPreview, "68 65") {
		t.Fatalf("text preview=%q", textPreview)
	}

	st.find.hexPreview = true
	redecodeViewerHexFindPreviews(st)
	hexPreview := viewerHexFindSnippet(st, st.find.hexMatches[0])
	if !strings.Contains(hexPreview, "68 65 6C 6C 6F") || strings.Contains(hexPreview, "hello") {
		t.Fatalf("hex preview=%q", hexPreview)
	}
}

func TestViewerBufferedHexFindMatchesAreImmediatelyPreviewable(t *testing.T) {
	buffer := []byte("before needle between needle after")
	matches := viewerBufferedHexFindMatches(buffer, 4096, []byte("needle"), 20)
	if len(matches) != 2 {
		t.Fatalf("matches=%d want 2", len(matches))
	}
	if matches[0].Start != 4096+7 || len(matches[0].PreviewBytes) == 0 {
		t.Fatalf("first match=%+v", matches[0])
	}
	if preview := viewerHexFindSnippet(&fileViewerState{}, matches[0]); !strings.Contains(preview, "needle") {
		t.Fatalf("preview=%q", preview)
	}
}

func TestPrepareFileViewerFindModeClearsSharedResults(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{mode: "hex"}
	st.find.modeKey = "file"
	st.find.matches = []viewerFindMatch{{Start: 1, End: 2}}
	st.find.textClicks = make([]widget.Clickable, 1)
	st.find.currentValid = true

	ui.prepareFileViewerFindMode(st)
	if st.find.modeKey != "hex" || len(st.find.matches) != 0 || len(st.find.textClicks) != 0 || st.find.currentValid {
		t.Fatalf("mode transition left shared state: key=%q matches=%d clicks=%d valid=%v", st.find.modeKey, len(st.find.matches), len(st.find.textClicks), st.find.currentValid)
	}
}

func TestPDFPreviewIsInactiveAfterSwitchingToHex(t *testing.T) {
	st := &fileViewerState{
		mode:                  "file",
		detectedImagePreview:  true,
		imagePreviewFormat:    "pdf",
		imagePreviewPageCount: 3,
	}
	if !viewerPDFPreviewActive(st) {
		t.Fatal("PDF preview should be active in File mode")
	}
	st.mode = "hex"
	if viewerPDFPreviewActive(st) {
		t.Fatal("PDF preview must not remain active in Hex mode")
	}
	st.find.pdfMatches = []viewerPDFFindMatch{{Page: 0}, {Page: 1}}
	st.find.hexMatches = []viewerHexFindMatch{{Start: 12, Length: 4}}
	if got := fileViewerFindResultCount(st); got != 1 {
		t.Fatalf("Hex result count=%d want 1", got)
	}
}

func TestViewerPDFFindPageMatchesIsCaseInsensitiveAndOverlapping(t *testing.T) {
	text := testPDFFindPage(2, "BANANa")
	matches := viewerPDFFindPageMatches(text, "ana")

	if len(matches) != 2 {
		t.Fatalf("matches=%d want 2", len(matches))
	}
	if matches[0].Page != 2 || matches[0].Start != 1 || matches[0].End != 4 {
		t.Fatalf("first match=%+v", matches[0])
	}
	if matches[1].Start != 3 || matches[1].End != 6 {
		t.Fatalf("second match=%+v", matches[1])
	}
}

func TestPDFViewerFindRunsAsynchronouslyAndStreamsMultipleResults(t *testing.T) {
	prev := viewerPDFPreviewBackend
	renderer := &blockingPDFFindRenderer{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
		pages: []viewerPDFPageText{
			testPDFFindPage(0, "needle first needle"),
			testPDFFindPage(1, "nothing here"),
			testPDFFindPage(2, "last NEEDLE"),
		},
	}
	viewerPDFPreviewBackend = renderer
	t.Cleanup(func() { viewerPDFPreviewBackend = prev })

	ui := NewUI(fm.DefaultConfig())
	st := &fileViewerState{
		detectedImagePreview:  true,
		imagePreviewFormat:    "pdf",
		imagePreviewPageCount: len(renderer.pages),
		pdfDocCh:              make(chan pdfDocResult, 4),
	}
	st.pdfDoc.configure(viewerPDFDocInfo{PageCount: 3, PageSizes: []viewerPDFPageSize{{W: 100, H: 160}, {W: 100, H: 160}, {W: 100, H: 160}}})
	st.pdfDoc.viewportRect = image.Rect(0, 0, 100, 120)
	st.pdfDoc.relayout()
	st.find.open = true
	st.find.editor.SetText("needle")
	st.find.resultCh = make(chan fileViewerFindResult, 1)
	st.find.pdfResultCh = make(chan viewerPDFFindResult, 16)
	st.find.pdfList.Axis = layout.Vertical
	st.find.index = -1
	ui.fileViewer = st

	start := time.Now()
	ui.refreshFileViewerFind(start, false)
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("refresh blocked for %v", elapsed)
	}
	select {
	case <-renderer.started:
	case <-time.After(time.Second):
		t.Fatal("background PDF search did not start")
	}
	if !st.find.searching {
		t.Fatal("search should remain active while page extraction is blocked")
	}
	close(renderer.release)

	gtx := layout.Context{Ops: new(op.Ops), Now: time.Now()}
	deadline := time.Now().Add(2 * time.Second)
	for st.find.searching && time.Now().Before(deadline) {
		gtx.Now = time.Now()
		ui.pumpFileViewerFindState(gtx, st)
		time.Sleep(time.Millisecond)
	}
	if st.find.searching {
		t.Fatal("PDF search did not finish")
	}
	if got := len(st.find.pdfMatches); got != 3 {
		t.Fatalf("matches=%d want 3", got)
	}
	if st.find.pdfSearched != 3 {
		t.Fatalf("searched pages=%d want 3", st.find.pdfSearched)
	}
	if st.find.index != 0 || !st.find.currentValid {
		t.Fatalf("current result=(%d,%v) want first valid result", st.find.index, st.find.currentValid)
	}
	if got := st.find.status; got != "1/3" {
		t.Fatalf("status=%q want 1/3", got)
	}
	if st.pdfDoc.text[2].Page != 2 {
		t.Fatal("search results should populate the PDF text cache")
	}
	if !ui.stepFileViewerFind(time.Now(), 1) || st.find.index != 1 {
		t.Fatalf("next result index=%d want 1", st.find.index)
	}
	if got := st.find.pdfMatches[st.find.index].Page; got != 0 {
		t.Fatalf("second result page=%d want page 0", got)
	}
	if !ui.stepFileViewerFind(time.Now(), 1) || st.find.index != 2 {
		t.Fatalf("third result index=%d want 2", st.find.index)
	}
	if got := st.find.pdfMatches[st.find.index].Page; got != 2 {
		t.Fatalf("third result page=%d want page 2", got)
	}
	if st.pdfDoc.scrollY <= 0 {
		t.Fatal("jumping to a later-page result should scroll the document")
	}
}

func TestParseViewerFindHexStringNormalizesSeparators(t *testing.T) {
	got, errText := parseViewerFindHexString("0xDE AD-be:ef")

	if errText != "" {
		t.Fatalf("parseViewerFindHexString err=%q", errText)
	}
	want := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if len(got) != len(want) {
		t.Fatalf("len(bytes)=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte[%d]=0x%X want 0x%X", i, got[i], want[i])
		}
	}
}

func TestParseViewerFindHexStringRejectsOddDigits(t *testing.T) {
	_, errText := parseViewerFindHexString("ABC")

	if errText == "" {
		t.Fatal("expected odd-length hex query to be rejected")
	}
}

func TestSearchViewerHexNextWrapsAcrossFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.bin")
	if err := os.WriteFile(path, []byte("abc---abc"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	res := searchViewerHexNext(context.Background(), path, nil, []byte("abc"), 4, viewerRemoteSearchSpec{})

	if !res.found {
		t.Fatal("expected wrapped next search to find a match")
	}
	if res.start != 6 {
		t.Fatalf("res.start=%d want 6", res.start)
	}
	if res.wrapped {
		t.Fatal("expected later in-file match before wrap")
	}

	res = searchViewerHexNext(context.Background(), path, nil, []byte("abc"), 9, viewerRemoteSearchSpec{})

	if !res.found || res.start != 0 || !res.wrapped {
		t.Fatalf("wrapped next search = %+v want start=0 wrapped=true", res)
	}
}

func TestSearchViewerHexPrevWrapsAndFallsBackToCurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.bin")
	if err := os.WriteFile(path, []byte("abc---abc"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	res := searchViewerHexPrev(context.Background(), path, nil, []byte("abc"), 6, 6, viewerRemoteSearchSpec{})

	if !res.found || res.start != 0 || res.wrapped {
		t.Fatalf("previous search = %+v want start=0 wrapped=false", res)
	}

	onePath := filepath.Join(t.TempDir(), "one.bin")
	if err := os.WriteFile(onePath, []byte("zzabczz"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	res = searchViewerHexPrev(context.Background(), onePath, nil, []byte("abc"), 2, 2, viewerRemoteSearchSpec{})

	if !res.found || res.start != 2 || !res.wrapped {
		t.Fatalf("single-match previous search = %+v want start=2 wrapped=true", res)
	}
}

func TestFileViewerFindStatusTextDelaysSearchingIndicator(t *testing.T) {
	ui := NewUI(nil)
	now := time.Now()
	st := &fileViewerState{
		find: fileViewerFindState{
			searching:       true,
			searchStartedAt: now,
		},
	}

	if got := ui.fileViewerFindStatusText(st, now.Add(viewerFindSearchingDelay/2)); got != "" {
		t.Fatalf("early searching status=%q want empty", got)
	}
	if got := ui.fileViewerFindStatusText(st, now.Add(viewerFindSearchingDelay)); got != "Searching..." {
		t.Fatalf("delayed searching status=%q want %q", got, "Searching...")
	}

	st.find.currentValid = true
	st.find.status = "2/5"
	if got := ui.fileViewerFindStatusText(st, now.Add(viewerFindSearchingDelay)); got != "2/5" {
		t.Fatalf("searching with current match status=%q want %q", got, "2/5")
	}
}

func TestFileViewerFindBarWidthsStayStableAcrossStatusChanges(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Now()
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(640, 120),
		},
	}

	empty := &fileViewerState{}
	counted := &fileViewerState{
		find: fileViewerFindState{
			status: "1234/3441",
		},
	}
	searching := &fileViewerState{
		find: fileViewerFindState{
			searching:       true,
			searchStartedAt: now.Add(-viewerFindSearchingDelay),
		},
	}

	emptyBarW, _ := ui.fileViewerFindBarWidths(th, gtx, empty, now)
	countedBarW, _ := ui.fileViewerFindBarWidths(th, gtx, counted, now)
	searchingBarW, _ := ui.fileViewerFindBarWidths(th, gtx, searching, now)

	if emptyBarW != countedBarW {
		t.Fatalf("find bar width empty=%d counted=%d want equal", emptyBarW, countedBarW)
	}
	if emptyBarW != searchingBarW {
		t.Fatalf("find bar width empty=%d searching=%d want equal", emptyBarW, searchingBarW)
	}
}

func TestFileViewerFindBarHeightStaysStableAcrossStatusChanges(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	th := material.NewTheme()
	now := time.Now()
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: new(input.Router).Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(640, 120),
		},
		Now: now,
	}

	searching := &fileViewerState{
		find: fileViewerFindState{
			open:            true,
			searching:       true,
			searchStartedAt: now.Add(-viewerFindSearchingDelay),
		},
	}
	counted := &fileViewerState{
		find: fileViewerFindState{
			open:   true,
			status: "1234/3441",
		},
	}

	searchingDims := ui.layoutFileViewerFindBar(th, gtx, searching)
	countedDims := ui.layoutFileViewerFindBar(th, gtx, counted)

	if searchingDims.Size.Y != countedDims.Size.Y {
		t.Fatalf("find bar height searching=%d counted=%d want equal", searchingDims.Size.Y, countedDims.Size.Y)
	}
}

func TestViewerFindModeGlyphKeepsStableCanvas(t *testing.T) {
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(14, 10)),
	}
	fg := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	textDims := layoutFileViewerFindModeGlyph(gtx, false, fg)
	gtx.Ops = new(op.Ops)
	hexDims := layoutFileViewerFindModeGlyph(gtx, true, fg)
	if textDims.Size != hexDims.Size || textDims.Size != image.Pt(14, 10) {
		t.Fatalf("mode glyph sizes text=%v hex=%v", textDims.Size, hexDims.Size)
	}
}

func TestViewerFindHintCentersUnderItsGlyph(t *testing.T) {
	anchor := image.Rect(100, 8, 122, 30)
	pos := viewerFindHintPoint(anchor, image.Pt(80, 18), image.Pt(300, 100), 2)
	if want := image.Pt(71, 32); pos != want {
		t.Fatalf("hint position=%v want %v", pos, want)
	}
	if gotCenter := pos.X + 40; gotCenter != anchor.Min.X+anchor.Dx()/2 {
		t.Fatalf("hint center=%d glyph center=%d", gotCenter, anchor.Min.X+anchor.Dx()/2)
	}
}

func TestViewerScrollStreamFindMatchKeepsMatchVisibleBeforeScrolling(t *testing.T) {
	now := time.Now()
	st := &fileViewerState{}
	st.stream.SetContent(strings.Join([]string{
		"match",
		"match",
		"match",
		"match",
		"match",
		"match",
	}, "\n"))
	st.stream.visibleLines = 5

	lastVisibleStart := st.stream.lineByteStart(4)
	viewerScrollStreamFindMatch(st, viewerFindMatch{Start: lastVisibleStart, End: lastVisibleStart + len("match")}, now)
	if got := st.stream.topLine; got != 0 {
		t.Fatalf("topLine at last visible match=%d want 0", got)
	}

	nextStart := st.stream.lineByteStart(5)
	viewerScrollStreamFindMatch(st, viewerFindMatch{Start: nextStart, End: nextStart + len("match")}, now)
	if got := st.stream.topLine; got != 1 {
		t.Fatalf("topLine after next offscreen match=%d want 1", got)
	}
}

func TestViewerScrollHexFindMatchKeepsMatchVisibleBeforeScrolling(t *testing.T) {
	now := time.Now()
	st := &fileViewerState{
		hex: &hexViewerState{
			bytesPerLine: 16,
			fileSize:     16 * 32,
			visibleLines: 5,
			topLine:      10,
		},
	}

	lastVisibleStart := int64((10 + 4) * 16)
	viewerScrollHexFindMatch(st, lastVisibleStart, 1, now)
	if got := st.hex.topLine; got != 10 {
		t.Fatalf("hex topLine at last visible match=%d want 10", got)
	}

	nextStart := int64((10 + 5) * 16)
	viewerScrollHexFindMatch(st, nextStart, 1, now)
	if got := st.hex.topLine; got != 11 {
		t.Fatalf("hex topLine after next offscreen match=%d want 11", got)
	}
}

func TestViewerFindPatternModeIsExplicit(t *testing.T) {
	textPattern, errText := viewerFindPatternBytes("DEADBEEF", false)
	if errText != "" || string(textPattern) != "DEADBEEF" {
		t.Fatalf("text pattern=%q err=%q", textPattern, errText)
	}

	hexPattern, errText := viewerFindPatternBytes("DE AD BE EF", true)
	if errText != "" {
		t.Fatalf("hex pattern err=%q", errText)
	}
	wantHex := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if !bytes.Equal(hexPattern, wantHex) {
		t.Fatalf("hex pattern=% X want % X", hexPattern, wantHex)
	}

	if _, errText := viewerFindPatternBytes("ABC", true); errText == "" {
		t.Fatal("explicit hex mode should reject an incomplete byte")
	}
}

func TestHexFindWorkerWakesWindowOnCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "typing.bin")
	if err := os.WriteFile(path, []byte("before needle after needle"), 0o600); err != nil {
		t.Fatal(err)
	}

	ui := NewUI(fm.DefaultConfig())
	wake := make(chan struct{}, 1)
	ui.SetInvalidateFunc(func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	})
	st := &fileViewerState{mode: "hex", path: path, hex: newHexViewerState()}
	st.hex.fileSize = 26
	st.hex.bytesPerLine = 16
	st.find.open = true
	st.find.editor.SetText("needle")
	st.find.resultCh = make(chan fileViewerFindResult, 1)
	st.find.index = -1
	ui.fileViewer = st

	ui.refreshFileViewerFind(time.Now(), false)
	select {
	case <-wake:
	case <-time.After(2 * time.Second):
		t.Fatal("completed asynchronous Hex find did not wake the window")
	}

	gtx := layout.Context{Ops: new(op.Ops), Now: time.Now()}
	ui.pumpFileViewerFindState(gtx, st)
	if got := len(st.find.hexMatches); got != 2 {
		t.Fatalf("matches=%d want 2", got)
	}
}

func TestSendFileViewerFindResultKeepsNewestRequest(t *testing.T) {
	ch := make(chan fileViewerFindResult, 1)
	sendFileViewerFindResult(ch, fileViewerFindResult{requestSeq: 12})
	sendFileViewerFindResult(ch, fileViewerFindResult{requestSeq: 11})
	if got := (<-ch).requestSeq; got != 12 {
		t.Fatalf("request sequence=%d want newest 12", got)
	}

	sendFileViewerFindResult(ch, fileViewerFindResult{requestSeq: 12})
	sendFileViewerFindResult(ch, fileViewerFindResult{requestSeq: 13})
	if got := (<-ch).requestSeq; got != 13 {
		t.Fatalf("request sequence=%d want newest 13", got)
	}
}

func TestViewerRemoteSearchOffsetParsesFirstOffsetLine(t *testing.T) {
	got, ok := viewerRemoteSearchOffset("grep: warning\n123:needle\n")

	if !ok {
		t.Fatal("viewerRemoteSearchOffset should parse a numeric match line")
	}
	if got != 123 {
		t.Fatalf("offset=%d want 123", got)
	}
}

func TestViewerFindRemoteUtilityForwardUsesRelativeOffset(t *testing.T) {
	prev := runViewerRemoteSearchCommandFunc
	defer func() { runViewerRemoteSearchCommandFunc = prev }()

	var gotCmd string
	runViewerRemoteSearchCommandFunc = func(_ context.Context, _ *paneSSHSession, cmdline string, _ viewerShellSpec) (string, error) {
		gotCmd = cmdline
		return "7:needle\n", nil
	}

	spec := viewerRemoteSearchSpec{
		template: fm.DefaultViewerRemoteSearchCommand,
		shell:    resolveViewerShell("sh", true),
		hexInput: false,
	}

	res, used := viewerFindRemoteUtilityForward(context.Background(), 1024, "/var/log/app.log", &paneSSHSession{}, []byte("needle"), 100, 1000, spec)

	if !used {
		t.Fatal("viewer remote search utility should be used when remote search is enabled")
	}
	if !res.found || res.start != 107 || res.length != int64(len("needle")) {
		t.Fatalf("result=%+v want found start=107 length=6", res)
	}
	if !strings.Contains(gotCmd, "tail -c +101") {
		t.Fatalf("command=%q want 1-based range start", gotCmd)
	}
	if !strings.Contains(gotCmd, "head -c 900") {
		t.Fatalf("command=%q want range length", gotCmd)
	}
	if !strings.Contains(gotCmd, "grep -aobF -m 1 -- 'needle'") {
		t.Fatalf("command=%q want fixed-string grep", gotCmd)
	}
}

func TestViewerRemoteSearchUtilityDefaultSkipsHexPatterns(t *testing.T) {
	spec := viewerRemoteSearchSpec{
		template: fm.DefaultViewerRemoteSearchCommand,
		shell:    resolveViewerShell("sh", true),
		hexInput: true,
	}

	if viewerRemoteSearchUsable(spec, 1024) {
		t.Fatal("default remote search command should not claim hex-pattern support")
	}
}

func TestViewerFindRemoteUtilityForwardFallsBackOnGarbageOutput(t *testing.T) {
	prev := runViewerRemoteSearchCommandFunc
	defer func() { runViewerRemoteSearchCommandFunc = prev }()

	runViewerRemoteSearchCommandFunc = func(_ context.Context, _ *paneSSHSession, _ string, _ viewerShellSpec) (string, error) {
		return "grep: command not found\n", nil
	}

	spec := viewerRemoteSearchSpec{
		template: fm.DefaultViewerRemoteSearchCommand,
		shell:    resolveViewerShell("sh", true),
		hexInput: false,
	}

	if _, used := viewerFindRemoteUtilityForward(context.Background(), 1024, "/var/log/app.log", &paneSSHSession{}, []byte("needle"), 0, 900, spec); used {
		t.Fatal("garbage command output should fall back to the built-in remote scan")
	}
}

func TestViewerRemoteSearchRemoteModeUsesUtility(t *testing.T) {
	prev := runViewerRemoteSearchCommandFunc
	defer func() { runViewerRemoteSearchCommandFunc = prev }()

	runViewerRemoteSearchCommandFunc = func(_ context.Context, _ *paneSSHSession, _ string, _ viewerShellSpec) (string, error) {
		return "3:needle\n", nil
	}

	spec := viewerRemoteSearchSpec{
		template: fm.DefaultViewerRemoteSearchCommand,
		mode:     fm.ViewerRemoteSearchModeRemote,
		shell:    resolveViewerShell("sh", true),
		hexInput: false,
	}

	res, used := viewerFindRemoteUtilityForward(context.Background(), 1024, "/var/log/app.log", &paneSSHSession{}, []byte("needle"), 10, 128, spec)
	if !used {
		t.Fatal("remote mode should use the remote utility")
	}
	if !res.found || res.start != 13 {
		t.Fatalf("result=%+v want found start=13", res)
	}
}

func TestViewerRemoteSearchLocalModeSkipsUtility(t *testing.T) {
	spec := viewerRemoteSearchSpec{
		template: fm.DefaultViewerRemoteSearchCommand,
		mode:     fm.ViewerRemoteSearchModeLocal,
		shell:    resolveViewerShell("sh", true),
		hexInput: false,
	}

	if viewerRemoteSearchUsable(spec, 1024) {
		t.Fatal("local mode should skip the remote utility")
	}
}
