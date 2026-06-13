// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"errors"
	"hexone/fm"
	"image"
	"image/color"
	"io"
	"math"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget/material"
	headlessterm "github.com/danielgatis/go-headless-term"
)

func TestTerminalKeyBytesSpecialKeys(t *testing.T) {
	tests := []struct {
		name string
		ev   key.Event
		want string
	}{
		{
			name: "enter",
			ev:   key.Event{Name: key.NameEnter, State: key.Press},
			want: "\r",
		},
		{
			name: "shift tab",
			ev:   key.Event{Name: key.NameTab, State: key.Press, Modifiers: key.ModShift},
			want: "\x1b[Z",
		},
		{
			name: "up",
			ev:   key.Event{Name: key.NameUpArrow, State: key.Press},
			want: "\x1b[A",
		},
		{
			name: "ctrl c",
			ev:   key.Event{Name: "C", State: key.Press, Modifiers: key.ModCtrl},
			want: "\x03",
		},
		{
			name: "alt x",
			ev:   key.Event{Name: "x", State: key.Press, Modifiers: key.ModAlt},
			want: "\x1bx",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(terminalKeyBytes(tc.ev)); got != tc.want {
				t.Fatalf("terminalKeyBytes()=%q want %q", got, tc.want)
			}
		})
	}
}

func TestTerminalApplicationCursorKeys(t *testing.T) {
	if got, want := string(terminalKeyBytesForMode(key.Event{Name: key.NameUpArrow, State: key.Press}, true)), "\x1bOA"; got != want {
		t.Fatalf("application up=%q want %q", got, want)
	}
	if got, want := string(terminalKeyBytesForMode(key.Event{Name: key.NameDownArrow, State: key.Press}, true)), "\x1bOB"; got != want {
		t.Fatalf("application down=%q want %q", got, want)
	}
	if got, want := string(terminalKeyBytesForMode(key.Event{Name: key.NameRightArrow, State: key.Press}, true)), "\x1bOC"; got != want {
		t.Fatalf("application right=%q want %q", got, want)
	}
	if got, want := string(terminalKeyBytesForMode(key.Event{Name: key.NameLeftArrow, State: key.Press}, true)), "\x1bOD"; got != want {
		t.Fatalf("application left=%q want %q", got, want)
	}
	if got, want := string(terminalKeyBytesForMode(key.Event{Name: key.NameHome, State: key.Press}, true)), "\x1bOH"; got != want {
		t.Fatalf("application home=%q want %q", got, want)
	}
	if got, want := string(terminalKeyBytesForMode(key.Event{Name: key.NameEnd, State: key.Press}, true)), "\x1bOF"; got != want {
		t.Fatalf("application end=%q want %q", got, want)
	}
}

func TestTerminalTracksApplicationCursorMode(t *testing.T) {
	st := newTerminalSession(nil)

	st.writeOutput([]byte("\x1b[?1h"))
	if !st.cursorKeysApplication() {
		t.Fatal("DECCKM set should enable application cursor-key mode")
	}

	st.writeOutput([]byte("\x1b[?1l"))
	if st.cursorKeysApplication() {
		t.Fatal("DECCKM reset should disable application cursor-key mode")
	}
}

func TestTerminalTracksMouseModes(t *testing.T) {
	st := newTerminalSession(nil)

	st.writeOutput([]byte("\x1b[?1000h\x1b[?1006h"))
	modes := st.terminalMouseModes()
	if !modes.clicks || !modes.sgr || !st.terminalMouseReporting() {
		t.Fatalf("mouse modes after enable=%+v reporting=%v", modes, st.terminalMouseReporting())
	}

	st.writeOutput([]byte("\x1b[?1000l\x1b[?1006l"))
	modes = st.terminalMouseModes()
	if modes.clicks || modes.sgr || st.terminalMouseReporting() {
		t.Fatalf("mouse modes after disable=%+v reporting=%v", modes, st.terminalMouseReporting())
	}
}

func TestTerminalTracksBracketedPasteMode(t *testing.T) {
	st := newTerminalSession(nil)

	st.writeOutput([]byte("\x1b[?2004h"))
	if !st.bracketedPasteMode() {
		t.Fatal("bracketed paste should be enabled after CSI ? 2004 h")
	}

	st.writeOutput([]byte("\x1b[?2004l"))
	if st.bracketedPasteMode() {
		t.Fatal("bracketed paste should be disabled after CSI ? 2004 l")
	}
}

func TestTerminalMouseReportBytes(t *testing.T) {
	sgr := terminalMouseReportModes{clicks: true, sgr: true}
	if got, want := string(terminalMouseReportBytes(0, 4, 9, false, false, 0, sgr)), "\x1b[<0;10;5M"; got != want {
		t.Fatalf("sgr press=%q want %q", got, want)
	}
	if got, want := string(terminalMouseReportBytes(0, 4, 9, true, false, 0, sgr)), "\x1b[<0;10;5m"; got != want {
		t.Fatalf("sgr release=%q want %q", got, want)
	}
	if got, want := string(terminalMouseReportBytes(0, 4, 9, false, true, key.ModShift|key.ModCtrl, sgr)), "\x1b[<52;10;5M"; got != want {
		t.Fatalf("sgr motion=%q want %q", got, want)
	}
	if got, want := string(terminalMouseReportBytes(64, 0, 0, false, false, 0, sgr)), "\x1b[<64;1;1M"; got != want {
		t.Fatalf("sgr wheel=%q want %q", got, want)
	}

	legacy := terminalMouseReportModes{clicks: true}
	if got, want := string(terminalMouseReportBytes(0, 0, 0, false, false, 0, legacy)), "\x1b[M !!"; got != want {
		t.Fatalf("legacy press=%q want %q", got, want)
	}
	if got, want := string(terminalMouseReportBytes(0, 0, 0, true, false, 0, legacy)), "\x1b[M#!!"; got != want {
		t.Fatalf("legacy release=%q want %q", got, want)
	}
}

func TestTerminalSelectAllKey(t *testing.T) {
	if !terminalSelectAllKey(key.Event{Name: "A", State: key.Press, Modifiers: key.ModCtrl}) {
		t.Fatal("Ctrl+A should select all")
	}
	if !terminalSelectAllKey(key.Event{Name: "a", State: key.Press, Modifiers: key.ModShortcut}) {
		t.Fatal("Shortcut+A should select all")
	}
	if terminalSelectAllKey(key.Event{Name: "A", State: key.Press}) {
		t.Fatal("plain A should not select all")
	}
}

func TestTerminalCopyKey(t *testing.T) {
	if !terminalCopyKey(key.Event{Name: "C", State: key.Press, Modifiers: key.ModCtrl}) {
		t.Fatal("Ctrl+C should copy terminal selection when one exists")
	}
	if !terminalCopyKey(key.Event{Name: "c", State: key.Press, Modifiers: key.ModShortcut}) {
		t.Fatal("Shortcut+C should copy terminal selection when one exists")
	}
	if terminalCopyKey(key.Event{Name: "C", State: key.Press}) {
		t.Fatal("plain C should not copy")
	}
}

func TestTerminalPasteKey(t *testing.T) {
	if !terminalPasteKey(key.Event{Name: "V", State: key.Press, Modifiers: key.ModCtrl}) {
		t.Fatal("Ctrl+V should paste")
	}
	if !terminalPasteKey(key.Event{Name: "v", State: key.Press, Modifiers: key.ModShortcut}) {
		t.Fatal("Shortcut+V should paste")
	}
	if terminalPasteKey(key.Event{Name: "V", State: key.Press}) {
		t.Fatal("plain V should not paste")
	}
}

func TestTerminalPasteBytesNormalizeLineEndings(t *testing.T) {
	got := string(terminalPasteBytes("one\r\ntwo\nthree\rfour", false))
	want := "one\rtwo\rthree\rfour"
	if got != want {
		t.Fatalf("plain paste bytes=%q want %q", got, want)
	}

	got = string(terminalPasteBytes("one\r\ntwo\nthree\rfour", true))
	want = "\x1b[200~one\ntwo\nthree\nfour\x1b[201~"
	if got != want {
		t.Fatalf("bracketed paste bytes=%q want %q", got, want)
	}

	if got := terminalPasteBytes("\x00", false); got != nil {
		t.Fatalf("empty sanitized paste=%q want nil", string(got))
	}
}

func TestTerminalNormalizeC1Controls(t *testing.T) {
	got := terminalNormalizeC1Controls([]byte{'a', 0x9b, '2', 'K', 0x9d, '0', 0x9c, 'z'})
	want := []byte{'a', 0x1b, '[', '2', 'K', 0x1b, ']', '0', 0x1b, '\\', 'z'}
	if string(got) != string(want) {
		t.Fatalf("normalized=%q want %q", got, want)
	}
	plain := []byte("plain")
	if normalized := terminalNormalizeC1Controls(plain); string(normalized) != "plain" {
		t.Fatalf("plain normalized=%q", normalized)
	}
	unicodeGlyphs := []byte("⠋ ✔ Bottle")
	if normalized := terminalNormalizeC1Controls(unicodeGlyphs); string(normalized) != string(unicodeGlyphs) {
		t.Fatalf("unicode normalized=%q want %q", normalized, unicodeGlyphs)
	}
	mcBoxGlyphs := []byte("╔══╗ ║ │")
	if normalized := terminalNormalizeC1Controls(mcBoxGlyphs); string(normalized) != string(mcBoxGlyphs) {
		t.Fatalf("mc box glyphs normalized=%q want %q", normalized, mcBoxGlyphs)
	}
	boxAfterControl := terminalNormalizeC1Controls(append([]byte{0x9b, 'H'}, []byte("╔══╗")...))
	if got, want := string(boxAfterControl), "\x1b[H╔══╗"; got != want {
		t.Fatalf("mc box glyphs after C1 normalized=%q want %q", got, want)
	}
	var norm terminalOutputNormalizer
	split := append([]byte{}, norm.normalize([]byte{0xe2})...)
	split = append(split, norm.normalize([]byte{0x9c, 0x94, 0x9b, 'B'})...)
	if got, want := string(split), "✔\x1b[B"; got != want {
		t.Fatalf("split UTF-8/C1 normalized=%q want %q", got, want)
	}
	var boxNorm terminalOutputNormalizer
	splitBox := append([]byte{}, boxNorm.normalize([]byte{0xe2})...)
	splitBox = append(splitBox, boxNorm.normalize([]byte{0x95})...)
	splitBox = append(splitBox, boxNorm.normalize([]byte{0x90, 0x9b, 'A'})...)
	if got, want := string(splitBox), "═\x1b[A"; got != want {
		t.Fatalf("split box UTF-8/C1 normalized=%q want %q", got, want)
	}
}

func TestTerminalC1CursorDownDoesNotPrintFinalByte(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(3, 24)
	st.parserMu.Unlock()

	st.writeOutput([]byte("top\r\nmid\x9bBdown"))

	if got := st.term.LineContent(1); strings.Contains(got, "B") {
		t.Fatalf("C1 CSI final byte leaked into line: %q", got)
	}
	if got := st.term.LineContent(2); !strings.Contains(got, "down") {
		t.Fatalf("C1 CSI B did not move cursor down, row2=%q", got)
	}
}

func TestTerminalC1ProgressRedrawStaysInPlace(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(4, 48)
	st.parserMu.Unlock()

	st.writeOutput([]byte("==> Fetching\r\n" +
		"spinner Bottle pyenv (2.7.2)\r\n" +
		"\x9bA\x9b2K" +
		"done Bottle pyenv (2.7.2)"))

	if got := st.term.LineContent(1); strings.Contains(got, "spinner") || !strings.Contains(got, "done Bottle pyenv") {
		t.Fatalf("progress line was not redrawn in place: %q", got)
	}
	if scrollback := st.term.ScrollbackLen(); scrollback != 0 {
		t.Fatalf("progress redraw should not create scrollback, got %d lines", scrollback)
	}
}

func TestTerminalCSIPreviousLineUsesFullCount(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(4, 24)
	st.parserMu.Unlock()

	st.writeOutput([]byte("one\r\ntwo\r\nthree\x1b[2Ftop"))

	if got := st.term.LineContent(0); !strings.HasPrefix(got, "top") {
		t.Fatalf("CSI 2 F should move up two rows, row0=%q", got)
	}
	if got := st.term.LineContent(1); strings.HasPrefix(got, "top") {
		t.Fatalf("CSI 2 F only moved up one row, row1=%q", got)
	}
}

func TestTerminalCSINextLineUsesFullCount(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(4, 24)
	st.parserMu.Unlock()

	st.writeOutput([]byte("top\x1b[2Ebottom"))

	if got := st.term.LineContent(2); !strings.HasPrefix(got, "bottom") {
		t.Fatalf("CSI 2 E should move down two rows, row2=%q", got)
	}
	if got := st.term.LineContent(1); strings.HasPrefix(got, "bottom") {
		t.Fatalf("CSI 2 E only moved down one row, row1=%q", got)
	}
}

func TestTerminalScrollRegionIncludesBottomRow(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(24, 24)
	st.parserMu.Unlock()

	var b strings.Builder
	b.WriteString("title")
	for i := 1; i <= 22; i++ {
		b.WriteString("\r\nline")
		b.WriteString(strconvItoa(i))
	}
	b.WriteString("\r\nstatus")
	b.WriteString("\x1b[2;23r\x1b[2;1H\x1b[3M")
	st.writeOutput([]byte(b.String()))

	if got := st.term.LineContent(1); !strings.HasPrefix(got, "line4") {
		t.Fatalf("scroll region did not shift from the top, row2=%q", got)
	}
	if got := st.term.LineContent(22); got != "" {
		t.Fatalf("scroll region bottom row should be cleared by delete-lines, row23=%q", got)
	}
	if got := st.term.LineContent(23); !strings.HasPrefix(got, "status") {
		t.Fatalf("delete-lines should not affect row24 outside scroll region, row24=%q", got)
	}
}

func TestTerminalHomebrewDownloadRowsRedrawInPlace(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(8, 100)
	st.parserMu.Unlock()

	st.writeOutput([]byte("==> Fetching downloads\r\n" +
		"⠋ Bottle pandoc (3.10) Downloading 18.7MB/55.2MB\r\n" +
		"⠋ Cask postman (12.14.5) Downloading 20.0MB/137.4MB\r\n" +
		"⠋ Cask visual-studio-code (1.124.2) Verifying 249.5MB/249.5MB" +
		"\x1b[2F" +
		"⠙ Bottle pandoc (3.10) Downloading 21.4MB/55.2MB\x1b[K\r\n" +
		"✔ Cask postman (12.14.5) Verified 137.4MB/137.4MB\x1b[K\r\n" +
		"✔ Cask visual-studio-code (1.124.2) Verified 249.5MB/249.5MB\x1b[K"))

	for row, want := range map[int]string{
		1: "⠙ Bottle pandoc",
		2: "✔ Cask postman",
		3: "✔ Cask visual-studio-code",
	} {
		if got := st.term.LineContent(row); !strings.HasPrefix(got, want) {
			t.Fatalf("row %d=%q want prefix %q", row, got, want)
		}
	}
	if got := st.term.LineContent(4); strings.Contains(got, "visual-studio-code") {
		t.Fatalf("homebrew redraw leaked into an extra row: %q", got)
	}
}

func TestTerminalPreservesHomebrewStatusGlyphs(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(4, 48)
	st.parserMu.Unlock()

	st.writeOutput([]byte("⠋ Bottle nss\r\n"))
	st.writeOutput([]byte("🍺 /opt/homebrew/Cellar/nss/3.125"))

	if got := st.term.LineContent(0); !strings.HasPrefix(got, "⠋ Bottle") || strings.HasPrefix(got, "B Bottle") {
		t.Fatalf("spinner glyph was not preserved: %q", got)
	}
	if got := st.term.LineContent(1); !strings.HasPrefix(got, "🍺 /opt") {
		t.Fatalf("emoji glyph was not preserved in terminal cells: %q", got)
	}
}

func TestTerminalACSLineDrawingForMCLikeBorders(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(2, 16)
	st.parserMu.Unlock()

	st.writeOutput([]byte("\x1b)0\x0elqqqk\x0f"))

	if got, want := st.term.LineContent(0), "┌───┐"; got != want {
		t.Fatalf("G1 ACS border=%q want %q", got, want)
	}

	st.writeOutput([]byte("\r\n\x1b(0xqqqx\x1b(B"))
	if got, want := st.term.LineContent(1), "│───│"; got != want {
		t.Fatalf("G0 ACS border=%q want %q", got, want)
	}
}

func TestTerminalPreservesMCUTF8BoxDrawing(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(3, 16)
	st.parserMu.Unlock()

	st.writeOutput([]byte("╔══╗\r\n║mc║\r\n╚══╝"))

	for row, want := range []string{"╔══╗", "║mc║", "╚══╝"} {
		if got := st.term.LineContent(row); got != want {
			t.Fatalf("row %d=%q want %q", row, got, want)
		}
		if got := st.term.LineContent(row); strings.ContainsAny(got, "PQ") {
			t.Fatalf("row %d leaked C1 replacement letters: %q", row, got)
		}
	}
}

func TestTerminalEraseLineUsesCurrentBackground(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(1, 8)
	st.parserMu.Unlock()

	st.writeOutput([]byte("\x1b[48;5;250m\x1b[2K"))

	for col := 0; col < 8; col++ {
		cell := st.term.Cell(0, col)
		if cell == nil {
			t.Fatalf("missing cell %d", col)
		}
		assertTerminalCellBGIndex(t, *cell, 250)
	}
}

func TestTerminalEraseCharsAndInsertBlanksUseCurrentBackground(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(1, 8)
	st.parserMu.Unlock()

	st.writeOutput([]byte("abcdefgh\r\x1b[48;5;240m\x1b[2X"))
	for col := 0; col < 2; col++ {
		cell := st.term.Cell(0, col)
		if cell == nil {
			t.Fatalf("missing erased cell %d", col)
		}
		assertTerminalCellBGIndex(t, *cell, 240)
	}

	st.writeOutput([]byte("\r\x1b[48;5;242m\x1b[2@"))
	for col := 0; col < 2; col++ {
		cell := st.term.Cell(0, col)
		if cell == nil {
			t.Fatalf("missing inserted cell %d", col)
		}
		assertTerminalCellBGIndex(t, *cell, 242)
	}
}

func TestTerminalDeleteCharsUsesCurrentBackgroundAtLineEnd(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(1, 8)
	st.parserMu.Unlock()

	st.writeOutput([]byte("abcdefgh\r\x1b[48;5;244m\x1b[2P"))

	for col := 6; col < 8; col++ {
		cell := st.term.Cell(0, col)
		if cell == nil {
			t.Fatalf("missing deleted tail cell %d", col)
		}
		assertTerminalCellBGIndex(t, *cell, 244)
	}
}

func TestTerminalMCStartupUTF8BorderSegment(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(6, 80)
	st.parserMu.Unlock()

	st.writeOutput([]byte("\x1b[?1049h\x1b[1;24r\x1b[4l\x1b[?1h\x1b=\x1b(B\x1b[m\x1b[39m\x1b[49m\x1b[H\x1b[2J" +
		"\x1b[2;79H\x1b[1K \x1b[38;5;250m\x1b[48;5;234m╗\x1b[2;80H" +
		"\x1b]0;mc [ramunas@host]:~/go/src/hexone\x1b\\\x1b>" +
		"\x1b]7;file://host/Users/ramunas/go/src/hexone\x1b\\\r" +
		"╔«═\x1b[38;5;234m\x1b[48;5;250m ~/go/src/hexone \x1b[38;5;250m\x1b[48;5;234m══════════════•[^]»╗"))

	got := ""
	for row := 0; row < st.term.Rows(); row++ {
		line := st.term.LineContent(row)
		if strings.Contains(line, "go/src/hexone") || strings.ContainsAny(line, "PQ") {
			got = line
			break
		}
	}
	if strings.ContainsAny(got, "PQ") {
		t.Fatalf("mc UTF-8 border leaked literal control fallback letters: %q", got)
	}
	if !strings.Contains(got, "╔«═") || !strings.Contains(got, "════════") {
		t.Fatalf("mc UTF-8 border not preserved: %q", got)
	}
}

func assertTerminalCellBGIndex(t *testing.T, cell headlessterm.Cell, index int) {
	t.Helper()
	got := terminalColor(cell.Bg, false)
	palette := headlessterm.DefaultPalette[index]
	want := color.NRGBA{R: palette.R, G: palette.G, B: palette.B, A: palette.A}
	if got != want {
		t.Fatalf("cell bg=%+v want palette[%d]=%+v", got, index, want)
	}
	if cell.Char != ' ' {
		t.Fatalf("cell char=%q want space", cell.Char)
	}
}

func TestTerminalBoxDrawingSpecCoversMCGlyphs(t *testing.T) {
	for _, r := range "─│┌┐└┘├┤┬┴┼═║╔╗╚╝╠╣╦╩╬╟╢" {
		if _, ok := terminalBoxSpec(r); !ok {
			t.Fatalf("terminalBoxSpec(%q) not covered", r)
		}
	}
	if _, ok := terminalBoxSpec('P'); ok {
		t.Fatal("plain letters should not be treated as box drawing")
	}
	spec, ok := terminalBoxSpec('╟')
	if !ok {
		t.Fatal("mixed connector missing")
	}
	if spec.right != terminalBoxSingle || spec.up != terminalBoxDouble || spec.down != terminalBoxDouble || spec.left != terminalBoxNone {
		t.Fatalf("╟ spec=%+v", spec)
	}
	spec, ok = terminalBoxSpec('╢')
	if !ok {
		t.Fatal("mixed connector missing")
	}
	if spec.left != terminalBoxSingle || spec.up != terminalBoxDouble || spec.down != terminalBoxDouble || spec.right != terminalBoxNone {
		t.Fatalf("╢ spec=%+v", spec)
	}
}

func TestTerminalContextMenuPressDoesNotCloseFromTerminalPointer(t *testing.T) {
	st := newTerminalSession(nil)
	gtx, router := testPointerContext()
	registerPointerTag(router, gtx.Ops, &st.pointerTag)
	primePointerFilter(router, &st.pointerTag)

	st.openContextMenu(image.Pt(20, 20), time.Now())
	st.setTerminalMenuGeometry(
		image.Pt(20, 20),
		image.Pt(120, 90),
		[]terminalMenuItemRect{{id: "copy", rect: image.Rect(20, 20, 140, 42)}},
	)
	router.Queue(pointer.Event{
		Kind:     pointer.Press,
		Buttons:  pointer.ButtonPrimary,
		Position: f32.Pt(34, 34),
	})

	if !st.handlePointer(gtx, image.Rect(0, 0, 240, 160), 8, 16) {
		t.Fatal("press inside open menu should be handled")
	}
	st.viewMu.Lock()
	open := st.menuOpen
	st.viewMu.Unlock()
	if !open {
		t.Fatal("terminal pointer handler should not close context menu for menu item clicks")
	}
}

func TestTerminalContextMenuIncludesPaste(t *testing.T) {
	ui := &UI{}
	st := newTerminalSession(nil)

	items := ui.terminalContextMenuItems(st)
	for _, item := range items {
		if item.id == "paste" && item.label == "Paste" && item.click == &st.menuPaste {
			return
		}
	}
	t.Fatalf("terminal context menu missing Paste item: %+v", items)
}

func TestTerminalCellFromHeadlessMapsStyle(t *testing.T) {
	src := headlessterm.Cell{
		Char: 'X',
		Fg:   color.RGBA{R: 10, G: 20, B: 30, A: 255},
		Bg:   color.RGBA{R: 40, G: 50, B: 60, A: 255},
	}
	src.SetFlag(headlessterm.CellFlagBold)
	src.SetFlag(headlessterm.CellFlagReverse)

	got := terminalCellFromHeadless(src)
	if got.Rune != 'X' {
		t.Fatalf("Rune=%q want X", got.Rune)
	}
	if !got.Bold {
		t.Fatal("Bold=false want true")
	}
	if got.FG != (color.NRGBA{R: 40, G: 50, B: 60, A: 255}) {
		t.Fatalf("FG=%v want reversed background", got.FG)
	}
	if got.BG != (color.NRGBA{R: 10, G: 20, B: 30, A: 255}) {
		t.Fatalf("BG=%v want reversed foreground", got.BG)
	}
}

func TestTerminalParserPreservesPromptGlyphs(t *testing.T) {
	st := newTerminalSession(nil)
	prompt := "~ \u276f \u279c \ue0b0"

	st.parserMu.Lock()
	if _, err := st.term.Write([]byte(prompt)); err != nil {
		st.parserMu.Unlock()
		t.Fatalf("Write prompt glyphs: %v", err)
	}
	st.parserMu.Unlock()
	st.snapshot()

	st.State.Mu.RLock()
	defer st.State.Mu.RUnlock()
	gotRunes := make([]rune, 0, len([]rune(prompt)))
	for _, cell := range st.State.Grid[0] {
		if cell.Rune == 0 {
			continue
		}
		gotRunes = append(gotRunes, cell.Rune)
		if len(gotRunes) == len([]rune(prompt)) {
			break
		}
	}
	if len(gotRunes) != len([]rune(prompt)) {
		t.Fatalf("visible rune count=%d want %d (%q)", len(gotRunes), len([]rune(prompt)), string(gotRunes))
	}
	for i, r := range []rune(prompt) {
		got := gotRunes[i]
		if got != r {
			t.Fatalf("visible rune %d=%U want %U", i, got, r)
		}
	}
}

func TestTerminalGlyphCellWidthUsesWideSpacer(t *testing.T) {
	row := []TerminalCell{{Rune: '\u276f'}, {Rune: 0}, {Rune: 'x'}}
	if got := terminalGlyphCellWidth(row, 0, len(row)); got != 2 {
		t.Fatalf("wide glyph cell width=%d want 2", got)
	}
	if got := terminalGlyphCellWidth(row, 2, len(row)); got != 1 {
		t.Fatalf("normal glyph cell width=%d want 1", got)
	}
	if got := terminalGlyphCellWidth(row, 0, 1); got != 1 {
		t.Fatalf("clipped glyph cell width=%d want 1", got)
	}
}

func TestTerminalScrollOffsetClamps(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(3, 12)
	if _, err := st.term.Write([]byte("line0\r\nline1\r\nline2\r\nline3\r\nline4\r\nline5")); err != nil {
		st.parserMu.Unlock()
		t.Fatalf("Write lines: %v", err)
	}
	st.parserMu.Unlock()
	st.snapshot()

	st.State.Mu.RLock()
	scrollback := st.State.Scrollback
	st.State.Mu.RUnlock()
	if scrollback < 2 {
		t.Fatalf("scrollback=%d want at least 2", scrollback)
	}

	if !st.setScrollOffset(2) {
		t.Fatal("setScrollOffset should report change")
	}
	st.viewMu.Lock()
	if got := st.scrollOffset; got != 2 {
		st.viewMu.Unlock()
		t.Fatalf("scrollOffset=%d want 2", got)
	}
	st.viewMu.Unlock()

	st.setScrollOffset(99)
	st.viewMu.Lock()
	if got := st.scrollOffset; got != scrollback {
		st.viewMu.Unlock()
		t.Fatalf("scrollOffset after high clamp=%d want %d", got, scrollback)
	}
	st.viewMu.Unlock()

	st.setScrollOffset(-10)
	st.viewMu.Lock()
	if got := st.scrollOffset; got != 0 {
		st.viewMu.Unlock()
		t.Fatalf("scrollOffset after low clamp=%d want 0", got)
	}
	st.viewMu.Unlock()
}

func TestTerminalWheelDeltaScrollsIntoHistory(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(3, 12)
	if _, err := st.term.Write([]byte("line0\r\nline1\r\nline2\r\nline3\r\nline4\r\nline5")); err != nil {
		st.parserMu.Unlock()
		t.Fatalf("Write lines: %v", err)
	}
	st.parserMu.Unlock()
	st.snapshot()

	if !st.scrollByDelta(-1) {
		t.Fatal("negative wheel delta should move into scrollback history")
	}
	st.viewMu.Lock()
	if got := st.scrollOffset; got != 2 {
		st.viewMu.Unlock()
		t.Fatalf("scrollOffset=%d want 2", got)
	}
	st.viewMu.Unlock()
	if !st.scrollByDelta(1) {
		t.Fatal("positive wheel delta should move back toward live bottom")
	}
	st.viewMu.Lock()
	if got := st.scrollOffset; got != 0 {
		st.viewMu.Unlock()
		t.Fatalf("scrollOffset after reverse=%d want 0", got)
	}
	st.viewMu.Unlock()
}

func TestTerminalVisualScrollAnimatesShortSteps(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(3, 12)
	if _, err := st.term.Write([]byte("line0\r\nline1\r\nline2\r\nline3\r\nline4\r\nline5")); err != nil {
		st.parserMu.Unlock()
		t.Fatalf("Write lines: %v", err)
	}
	st.parserMu.Unlock()
	st.snapshot()

	now := time.Now()
	if st.prepareVisualScroll(now, true) {
		t.Fatal("initial visual sync should not animate")
	}
	if !st.scrollByDelta(-1) {
		t.Fatal("negative wheel delta should move into scrollback history")
	}
	if !st.prepareVisualScroll(now.Add(terminalSmoothTick), true) {
		t.Fatal("short terminal scroll should animate")
	}
	st.viewMu.Lock()
	visual := st.visualTop
	st.viewMu.Unlock()
	st.State.Mu.RLock()
	target := float32(st.State.ViewStart)
	st.State.Mu.RUnlock()
	if visual == target {
		t.Fatal("visual top should ease toward target instead of snapping")
	}
}

func TestTerminalChangeDirCommandQuotesPath(t *testing.T) {
	got := terminalPosixQuotePath("/tmp/it's here")
	if got != "'/tmp/it'\\''s here'" {
		t.Fatalf("terminalPosixQuotePath=%q", got)
	}
}

func TestTerminalSnapshotExposesScrollbackView(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(3, 12)
	if _, err := st.term.Write([]byte("line0\r\nline1\r\nline2\r\nline3\r\nline4")); err != nil {
		st.parserMu.Unlock()
		t.Fatalf("Write lines: %v", err)
	}
	st.parserMu.Unlock()
	st.snapshot()

	st.State.Mu.RLock()
	scrollback := st.State.Scrollback
	st.State.Mu.RUnlock()
	if scrollback <= 0 {
		t.Fatal("expected scrollback after writing past terminal height")
	}
	st.setScrollOffset(scrollback)

	st.State.Mu.RLock()
	if got := st.State.ViewStart; got != 0 {
		st.State.Mu.RUnlock()
		t.Fatalf("ViewStart=%d want 0 when scrolled to oldest history", got)
	}
	if got := st.State.ScrollOffset; got != scrollback {
		st.State.Mu.RUnlock()
		t.Fatalf("ScrollOffset=%d want %d", got, scrollback)
	}
	st.State.Mu.RUnlock()
}

func TestTerminalSelectedTextRange(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(3, 12)
	if _, err := st.term.Write([]byte("abcdef")); err != nil {
		st.parserMu.Unlock()
		t.Fatalf("Write text: %v", err)
	}
	st.parserMu.Unlock()
	st.snapshot()

	st.viewMu.Lock()
	st.selectionActive = true
	st.selectionStart = terminalPoint{Row: 0, Col: 1}
	st.selectionEnd = terminalPoint{Row: 0, Col: 3}
	st.viewMu.Unlock()

	if got := st.selectedText(false); got != "bcd" {
		t.Fatalf("selectedText=%q want bcd", got)
	}
}

func TestTerminalSelectAllTextIncludesScrollback(t *testing.T) {
	st := newTerminalSession(nil)
	st.parserMu.Lock()
	st.term.Resize(3, 12)
	if _, err := st.term.Write([]byte("line0\r\nline1\r\nline2\r\nline3\r\nline4")); err != nil {
		st.parserMu.Unlock()
		t.Fatalf("Write lines: %v", err)
	}
	st.parserMu.Unlock()
	st.snapshot()
	if !st.selectAll() {
		t.Fatal("selectAll should change selection")
	}
	text := st.selectedText(false)
	if !strings.Contains(text, "line1") || !strings.Contains(text, "line4") {
		t.Fatalf("selectAll text=%q, want scrollback and live rows", text)
	}
}

func TestTerminalPasteWritesClipboardToProcess(t *testing.T) {
	oldRead := readTerminalClipboardText
	readTerminalClipboardText = func() (string, error) {
		return "echo one\r\necho two", nil
	}
	defer func() {
		readTerminalClipboardText = oldRead
	}()

	st := newTerminalSession(nil)
	proc := &terminalWriteProcess{}
	st.procMu.Lock()
	st.pty = proc
	st.running = true
	st.procMu.Unlock()
	ui := &UI{terminal: st}
	gtx := testTerminalPaneHeightContext(image.Pt(640, 120))

	if !ui.pasteTerminalText(gtx) {
		t.Fatal("pasteTerminalText returned false")
	}
	if got, want := proc.String(), "echo one\recho two"; got != want {
		t.Fatalf("pasted bytes=%q want %q", got, want)
	}
	if st.pasteReadPending(gtx.Now) {
		t.Fatal("synchronous paste should not leave async paste pending")
	}
}

func TestTerminalPasteAsyncReadPendingExpires(t *testing.T) {
	oldRead := readTerminalClipboardText
	readTerminalClipboardText = func() (string, error) {
		return "", errors.New("force async paste")
	}
	defer func() {
		readTerminalClipboardText = oldRead
	}()

	st := newTerminalSession(nil)
	gtx := testTerminalPaneHeightContext(image.Pt(640, 120))
	gtx.Now = time.Now()

	if !st.pasteText(gtx) {
		t.Fatal("pasteText returned false")
	}
	if !st.pasteReadPending(gtx.Now) {
		t.Fatal("async paste should be pending after clipboard read request")
	}
	if st.pasteReadPending(gtx.Now.Add(terminalPasteReadTimeout + time.Millisecond)) {
		t.Fatal("async paste pending state should expire")
	}
}

func TestTerminalTypefaceUsesPaneFontWithSymbolFallback(t *testing.T) {
	for _, tc := range []struct {
		name string
		ui   *UI
		want string
	}{
		{name: "default", ui: &UI{}, want: "Fira Code"},
		{name: "pane font", ui: &UI{typeface: "Consolas"}, want: "Consolas"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := string(tc.ui.terminalTypeface())
			if !strings.HasPrefix(got, tc.want+", ") {
				t.Fatalf("terminalTypeface=%q want base pane face %q", got, tc.want)
			}
			if !strings.Contains(got, "Apple Symbols") || !strings.Contains(got, "Segoe UI Symbol") {
				t.Fatalf("terminalTypeface=%q missing symbol fallbacks", got)
			}
			if !strings.Contains(got, "Apple Braille") || !strings.Contains(got, "Apple Color Emoji") {
				t.Fatalf("terminalTypeface=%q missing braille/emoji fallbacks", got)
			}
		})
	}
}

func TestTerminalTextSizeTracksPaneTableSize(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.General.FontSizeSp = 18
	ui := &UI{fmCfg: cfg}

	if got, want := ui.terminalTextSize(), scaleConfigFontSize(cfg, 13); got != want {
		t.Fatalf("terminalTextSize=%v want pane table size %v", got, want)
	}
}

func TestTerminalCellWidthUsesTypefaceAdvance(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.General.Typeface = "Consolas"
	ui := &UI{fmCfg: cfg, typeface: "Consolas"}
	th := material.NewTheme()
	gtx := testTerminalPaneHeightContext(image.Pt(640, 120))

	got, _ := ui.terminalCellSize(th, gtx)
	want := int(math.Ceil(float64(measureTypefaceCharAdvanceAt(th, gtx, ui.terminalBaseTypeface(), ui.terminalTextSize()))))
	if got != want {
		t.Fatalf("terminal cell width=%d want measured advance %d", got, want)
	}
}

func TestTerminalPaneHeightPrefersFullscreenRows(t *testing.T) {
	gtx := testTerminalPaneHeightContext(image.Pt(1200, 800))
	cellH := 20
	h := terminalPaneHeight(gtx, cellH)
	rows := (h - gtx.Dp(unit.Dp(9))) / cellH
	if rows < terminalPreferredRows {
		t.Fatalf("terminal rows=%d want at least %d, height=%d", rows, terminalPreferredRows, h)
	}
	if h > gtx.Constraints.Max.Y-gtx.Dp(unit.Dp(150)) {
		t.Fatalf("terminal height=%d should leave file pane room in %dpx", h, gtx.Constraints.Max.Y)
	}
}

func TestTerminalPaneHeightUsesConfiguredRows(t *testing.T) {
	gtx := testTerminalPaneHeightContext(image.Pt(1200, 800))
	cellH := 20
	h := terminalPaneHeight(gtx, cellH, 18)
	rows := (h - terminalPaneVerticalOverhead(gtx)) / cellH
	if got, want := rows, 18; got != want {
		t.Fatalf("terminal rows=%d want %d, height=%d", got, want, h)
	}
}

func TestTerminalPaneHeightFallsBackInSmallWindow(t *testing.T) {
	gtx := testTerminalPaneHeightContext(image.Pt(900, 500))
	h := terminalPaneHeight(gtx, 22)
	if h <= 0 || h >= gtx.Constraints.Max.Y {
		t.Fatalf("terminal height=%d should fit small window height=%d", h, gtx.Constraints.Max.Y)
	}
	if h > gtx.Constraints.Max.Y-gtx.Dp(unit.Dp(150)) {
		t.Fatalf("terminal height=%d should leave minimum file pane room in %dpx", h, gtx.Constraints.Max.Y)
	}
}

func TestTerminalResizeDragSnapsByRows(t *testing.T) {
	gtx := testTerminalPaneHeightContext(image.Pt(1200, 800))
	st := newTerminalSession(nil)
	st.beginResizeDrag(gtx, pointer.Event{
		PointerID: 7,
		Position:  f32.Pt(10, 6),
	}, 10)

	if got, want := st.resizeRowsFromDrag(gtx, image.Pt(10, -34), 20), 12; got != want {
		t.Fatalf("resize rows after upward drag=%d want %d", got, want)
	}
	if got, want := st.resizeRowsFromDrag(gtx, image.Pt(10, 25), 20), 9; got != want {
		t.Fatalf("resize rows after downward drag=%d want %d", got, want)
	}
}

func TestTerminalResizeHandleGeometryUsesLaidOutPaneHeight(t *testing.T) {
	rootGtx := testTerminalPaneHeightContext(image.Pt(1200, 800))
	paneGtx := testTerminalPaneHeightContext(image.Pt(1200, 680))
	cellH := 22
	rows := terminalConfiguredRows(fm.DefaultConfig())
	paneH := terminalPaneHeight(paneGtx, cellH, rows)
	fullWindowH := terminalPaneHeight(rootGtx, cellH, rows)
	if paneH == fullWindowH {
		t.Fatalf("test setup should clamp pane/full-window heights differently, both=%d", paneH)
	}

	st := newTerminalSession(nil)
	st.setActive(true)
	st.setPaneMetrics(paneH, cellH)
	ui := &UI{
		fmCfg:    fm.DefaultConfig(),
		terminal: st,
	}

	rect, gotCellH, gotRows, ok := ui.terminalResizeHandleGeometry(material.NewTheme(), rootGtx)
	if !ok {
		t.Fatal("resize handle geometry missing")
	}
	wantRect := terminalResizeHandleRect(rootGtx, rootGtx.Constraints.Max.Y-paneH)
	staleRect := terminalResizeHandleRect(rootGtx, rootGtx.Constraints.Max.Y-fullWindowH)
	if rect != wantRect {
		t.Fatalf("resize handle rect=%v want laid-out pane rect %v", rect, wantRect)
	}
	if rect == staleRect {
		t.Fatalf("resize handle used full-window fallback rect %v instead of pane rect %v", staleRect, wantRect)
	}
	if gotCellH != cellH {
		t.Fatalf("resize handle cellH=%d want %d", gotCellH, cellH)
	}
	if wantRows := terminalRowsForPaneHeight(rootGtx, cellH, paneH); gotRows != wantRows {
		t.Fatalf("resize handle rows=%d want %d", gotRows, wantRows)
	}
}

func TestTerminalResizeHandleUsesResizeCursor(t *testing.T) {
	gtx, router, st, rect := testTerminalResizeCursorFrame(t, false)
	router.Frame(gtx.Ops)
	router.Queue(pointer.Event{
		Kind:     pointer.Move,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonPrimary,
		Position: f32.Pt(float32(rect.Min.X+10), float32(rect.Min.Y+rect.Dy()/2)),
	})
	drainTerminalResizePointerEvents(gtx, st)

	if got, want := router.Cursor(), pointer.CursorNorthSouthResize; got != want {
		t.Fatalf("resize handle cursor=%v want %v", got, want)
	}
}

func TestTerminalResizeDragKeepsResizeCursorAcrossWindow(t *testing.T) {
	gtx, router, st, rect := testTerminalResizeCursorFrame(t, true)
	router.Frame(gtx.Ops)
	router.Queue(pointer.Event{
		Kind:     pointer.Move,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonPrimary,
		Position: f32.Pt(float32(gtx.Constraints.Max.X/2), float32(rect.Min.Y+rect.Dy()+80)),
	})
	drainTerminalResizePointerEvents(gtx, st)

	if got, want := router.Cursor(), pointer.CursorNorthSouthResize; got != want {
		t.Fatalf("drag cursor=%v want %v", got, want)
	}
}

func testTerminalPaneHeightContext(size image.Point) layout.Context {
	return layout.Context{
		Ops:    new(op.Ops),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: size,
		},
	}
}

func testTerminalResizeCursorFrame(t *testing.T, dragging bool) (layout.Context, *input.Router, *terminalSession, image.Rectangle) {
	t.Helper()
	cfg := fm.DefaultConfig()
	cfg.Terminal.HeightRows = 12
	st := newTerminalSession(nil, cfg.Terminal.HeightRows)
	st.setActive(true)
	ui := &UI{
		fmCfg:    cfg,
		terminal: st,
	}
	router := new(input.Router)
	gtx := testTerminalPaneHeightContext(image.Pt(1200, 800))
	gtx.Source = router.Source()
	th := material.NewTheme()
	_, cellH := ui.terminalCellSize(th, gtx)
	rows := terminalConfiguredRows(ui.fmCfg)
	top := gtx.Constraints.Max.Y - terminalPaneHeight(gtx, cellH, rows)
	rect := terminalResizeHandleRect(gtx, top)
	if rect.Empty() {
		t.Fatal("resize handle rect is empty")
	}
	underlay := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	pointer.CursorText.Add(gtx.Ops)
	underlay.Pop()
	if dragging {
		st.beginResizeDrag(gtx, pointer.Event{
			PointerID: 1,
			Position:  f32.Pt(float32(rect.Min.X+10), float32(rect.Min.Y+rect.Dy()/2)),
		}, rows)
	}
	ui.layoutTerminalResizeHandle(th, gtx)
	return gtx, router, st, rect
}

func drainTerminalResizePointerEvents(gtx layout.Context, st *terminalSession) {
	for {
		_, ok := gtx.Event(pointer.Filter{
			Target: &st.resizeTag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Move | pointer.Enter | pointer.Leave,
		})
		if !ok {
			return
		}
	}
}

func TestTerminalEnvOverridesSizeAndTerm(t *testing.T) {
	env := terminalEnv([]string{"TERM=dumb", "LINES=1", "COLUMNS=2", "KEEP=yes"}, 33, 101)
	want := map[string]string{
		"TERM":         "xterm-256color",
		"COLORTERM":    "truecolor",
		"TERM_PROGRAM": "hexone",
		"LINES":        "33",
		"COLUMNS":      "101",
		"KEEP":         "yes",
		"LANG":         terminalUTF8Locale,
		"LC_CTYPE":     terminalUTF8Locale,
	}
	for key, value := range want {
		if got := lookupEnvValue(env, key); got != value {
			t.Fatalf("%s=%q want %q in %v", key, got, value, env)
		}
	}
}

func TestTerminalEnvPreservesUTF8Locale(t *testing.T) {
	env := terminalEnv([]string{"LANG=lt_LT.UTF-8", "LC_CTYPE=lt_LT.UTF-8"}, 24, 80)
	if got := lookupEnvValue(env, "LANG"); got != "lt_LT.UTF-8" {
		t.Fatalf("LANG=%q want preserved UTF-8 locale in %v", got, env)
	}
	if got := lookupEnvValue(env, "LC_CTYPE"); got != "lt_LT.UTF-8" {
		t.Fatalf("LC_CTYPE=%q want preserved UTF-8 locale in %v", got, env)
	}
}

func TestTerminalEnvOverridesNonUTF8Locale(t *testing.T) {
	env := terminalEnv([]string{"LANG=C", "LC_CTYPE=C", "LC_ALL=C"}, 24, 80)
	for _, key := range []string{"LANG", "LC_CTYPE", "LC_ALL"} {
		if got := lookupEnvValue(env, key); got != terminalUTF8Locale {
			t.Fatalf("%s=%q want %q in %v", key, got, terminalUTF8Locale, env)
		}
	}
}

func TestTerminalEnvFixesLCTypeOverridingUTF8Lang(t *testing.T) {
	env := terminalEnv([]string{"LANG=en_US.UTF-8", "LC_CTYPE=C"}, 24, 80)
	if got := lookupEnvValue(env, "LANG"); got != "en_US.UTF-8" {
		t.Fatalf("LANG=%q want preserved UTF-8 locale in %v", got, env)
	}
	if got := lookupEnvValue(env, "LC_CTYPE"); got != terminalUTF8Locale {
		t.Fatalf("LC_CTYPE=%q want %q in %v", got, terminalUTF8Locale, env)
	}
}

func TestTerminalCommandForWindowsShellSelection(t *testing.T) {
	oldLookPath := terminalLookPath
	oldGetenv := terminalGetenv
	terminalLookPath = func(name string) (string, error) {
		switch name {
		case "pwsh.exe", "powershell.exe", "wsl.exe":
			return name, nil
		default:
			return "", errors.New("not found")
		}
	}
	terminalGetenv = func(string) string { return "" }
	defer func() {
		terminalLookPath = oldLookPath
		terminalGetenv = oldGetenv
	}()

	name, args := terminalCommandForShellOnGOOS("windows", "auto", `C:\Users\me`)
	if name != "pwsh.exe" || strings.Join(args, " ") != "-NoLogo" {
		t.Fatalf("auto command=%q %v, want pwsh.exe -NoLogo", name, args)
	}

	name, args = terminalCommandForShellOnGOOS("windows", "powershell", `C:\Users\me`)
	if name != "powershell.exe" || strings.Join(args, " ") != "-NoLogo" {
		t.Fatalf("powershell command=%q %v, want powershell.exe -NoLogo", name, args)
	}

	name, args = terminalCommandForShellOnGOOS("windows", "wsl:Ubuntu-24.04", `C:\Users\me`)
	wantArgs := []string{"--distribution", "Ubuntu-24.04", "--cd", `C:\Users\me`}
	if name != "wsl.exe" || strings.Join(args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("wsl command=%q %v, want wsl.exe %v", name, args, wantArgs)
	}
}

func TestTerminalShellOptionsIncludeDetectedWindowsShells(t *testing.T) {
	oldGetenv := terminalGetenv
	terminalGetenv = func(string) string { return "" }
	defer func() { terminalGetenv = oldGetenv }()

	lookPath := func(name string) (string, error) {
		switch name {
		case "pwsh.exe", "powershell.exe", "wsl.exe":
			return name, nil
		default:
			return "", errors.New("not found")
		}
	}
	options := terminalShellOptionsFor("windows", lookPath, []string{"Ubuntu", "Debian"})
	got := make([]string, 0, len(options))
	for _, opt := range options {
		got = append(got, opt.Key)
	}
	want := []string{"auto", "pwsh", "powershell", "wsl:Ubuntu", "wsl:Debian"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("shell options=%v want %v", got, want)
	}

	options = terminalShellOptionsFor("windows", lookPath, nil)
	got = got[:0]
	for _, opt := range options {
		got = append(got, opt.Key)
	}
	want = []string{"auto", "pwsh", "powershell"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("shell options without distros=%v want %v", got, want)
	}
}

func TestParseTerminalWSLDistrosUTF16(t *testing.T) {
	data := []byte{
		0xff, 0xfe,
		'U', 0, 'b', 0, 'u', 0, 'n', 0, 't', 0, 'u', 0, '\r', 0, '\n', 0,
		'D', 0, 'e', 0, 'b', 0, 'i', 0, 'a', 0, 'n', 0, '\r', 0, '\n', 0,
	}
	got := parseTerminalWSLDistros(data)
	want := []string{"Ubuntu", "Debian"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("distros=%v want %v", got, want)
	}
}

func TestWindowsPathToWSLPath(t *testing.T) {
	got := windowsPathToWSLPath(`C:\Users\me\My File.txt`)
	want := "/mnt/c/Users/me/My File.txt"
	if got != want {
		t.Fatalf("windowsPathToWSLPath=%q want %q", got, want)
	}
}

func lookupEnvValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if len(entry) >= len(prefix) && entry[:len(prefix)] == prefix {
			return entry[len(prefix):]
		}
	}
	return ""
}

type terminalWriteProcess struct {
	buf bytes.Buffer
}

func (p *terminalWriteProcess) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (p *terminalWriteProcess) Write(data []byte) (int, error) {
	return p.buf.Write(data)
}

func (p *terminalWriteProcess) Close() error {
	return nil
}

func (p *terminalWriteProcess) Resize(int, int) error {
	return nil
}

func (p *terminalWriteProcess) Wait() error {
	return nil
}

func (p *terminalWriteProcess) Kill() error {
	return nil
}

func (p *terminalWriteProcess) PID() int {
	return 1
}

func (p *terminalWriteProcess) String() string {
	return p.buf.String()
}
