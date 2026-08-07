// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"errors"
	resources "hexone"
	"hexone/fm"
	"hexone/ui/platform"
	"image"
	"image/color"
	"io"
	"math"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	uitheme "hexone/ui/theme"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/danielgatis/go-ansicode"
	headlessterm "github.com/danielgatis/go-headless-term"
	"golang.org/x/image/math/fixed"
)

const (
	terminalDefaultRows = 24
	terminalDefaultCols = 80
	terminalReadBufSize = 8192
	terminalScrollRange = 1 << 20
	terminalUTF8Locale  = "C.UTF-8"
)

const (
	terminalPreferredRows = 24
	terminalMaxPaneNum    = 3
	terminalMaxPaneDen    = 4
)

const (
	terminalSelectAutoScrollTick   = 50 * time.Millisecond
	terminalSelectAutoScrollNearPx = 20
	terminalSelectAutoScrollMidPx  = 64
	terminalSelectAutoScrollAccel1 = 650 * time.Millisecond
	terminalSelectAutoScrollAccel2 = 1400 * time.Millisecond
	terminalDoubleClickDur         = 420 * time.Millisecond
	terminalDoubleClickDist        = 6
)

const (
	terminalWheelAccel         = 2.25
	terminalSmoothTick         = 16 * time.Millisecond
	terminalSmoothTau          = 28 * time.Millisecond
	terminalSmoothSnapEpsilon  = 0.02
	terminalSmoothJumpMinLines = 6
	terminalCwdProbeTimeout    = 150 * time.Millisecond
	terminalPasteReadTimeout   = 2 * time.Second
)

var (
	terminalBG     = color.NRGBA{R: 4, G: 6, B: 10, A: 255}
	terminalBorder = color.NRGBA{R: 255, G: 255, B: 255, A: 28}
	terminalCursor = color.NRGBA{R: 230, G: 238, B: 248, A: 140}
	terminalError  = color.NRGBA{R: 255, G: 132, B: 132, A: 255}
	terminalSelect = color.NRGBA{R: 61, G: 132, B: 220, A: 110}
)

var (
	terminalLookPath          = exec.LookPath
	terminalGetenv            = os.Getenv
	readTerminalClipboardText = platform.ReadClipboardTextNow
	terminalProcessSSHTarget  = detectTerminalProcessSSHTarget
	terminalWordSelectRE      = regexp.MustCompile(`[A-Za-z0-9_./\\~:@%+=-]+`)
)

type TerminalCell struct {
	Rune rune
	FG   color.NRGBA
	BG   color.NRGBA
	Bold bool
	Dim  bool
}

type TerminalState struct {
	Mu   sync.RWMutex
	Grid [][]TerminalCell

	Cursor struct{ X, Y int }

	Active        bool
	CursorVisible bool
	Rows          int
	Cols          int
	ScrollOffset  int
	Scrollback    int
	ViewStart     int
	Alternate     bool
	Err           string
}

type terminalPoint struct {
	Row int
	Col int
}

type terminalProcess interface {
	io.ReadWriteCloser
	Resize(rows, cols int) error
	Wait() error
	Kill() error
	PID() int
}

type terminalSession struct {
	State TerminalState

	term       *headlessterm.Terminal
	parserMu   sync.Mutex
	outputNorm terminalOutputNormalizer
	modeMu     sync.RWMutex
	modes      terminalModes

	procMu          sync.Mutex
	writeMu         sync.Mutex
	pty             terminalProcess
	running         bool
	startAttempted  bool
	closing         bool
	startDir        string
	pendingStartDir string
	rows            int
	cols            int

	viewMu              sync.Mutex
	scrollOffset        int
	scrollCarry         float32
	lastScrollbackLen   int
	visualTop           float32
	visualReady         bool
	visualAt            time.Time
	scrollbarTrack      image.Rectangle
	scrollbarThumb      image.Rectangle
	scrollbarHover      bool
	scrollbarDragging   bool
	scrollbarDragID     pointer.ID
	scrollbarDragGrab   int
	resizeDragging      bool
	resizeDragID        pointer.ID
	resizeDragStartY    int
	resizeDragStartRows int
	resizeHover         bool
	paneHeight          int
	paneCellHeight      int
	selectionActive     bool
	selectionSelecting  bool
	selectionMoved      bool
	selectionStart      terminalPoint
	selectionEnd        terminalPoint
	selectionPointer    pointer.ID
	selectionLastPos    image.Point
	lastPrimaryPressed  bool
	lastPrimaryPressAt  time.Time
	lastPrimaryPressPos image.Point
	autoScrollDir       int
	autoScrollStep      int
	autoScrollAt        time.Time
	autoScrollStartedAt time.Time
	menuOpen            bool
	menuPos             image.Point
	menuRect            image.Rectangle
	menuItemRects       []terminalMenuItemRect
	menuOpenedAt        time.Time
	menuHoverID         string
	menuHoverAnim       segmentedAnimState
	pastePending        bool
	pastePendingAt      time.Time

	inputMu          sync.Mutex
	commandDraft     []rune
	commandCursor    int
	lastCommand      string
	commandReliable  bool
	keyRepeatActive  bool
	keyRepeatKey     key.Name
	keyRepeatStarted time.Time
	keyRepeatNext    time.Time
	keyRepeatPeriod  time.Duration
	find             terminalFindState

	invalidateMu sync.RWMutex
	invalidate   func()

	keyTag        uiEventTag
	pointerTag    uiEventTag
	resizeTag     uiEventTag
	menuTag       uiEventTag
	menuActionTag uiEventTag
	pasteTag      uiEventTag
	wantFocus     bool

	menuCopy      widget.Clickable
	menuPaste     widget.Clickable
	menuSelectAll widget.Clickable
	menuGoPane1   widget.Clickable
	menuGoPane2   widget.Clickable
	menuSetPane1  widget.Clickable
	menuSetPane2  widget.Clickable
}

func newTerminalSession(invalidate func(), preferredRows ...int) *terminalSession {
	rows := terminalDefaultRows
	if len(preferredRows) > 0 {
		rows = fm.NormalizeTerminalHeightRows(preferredRows[0])
	}
	st := &terminalSession{
		rows:            rows,
		cols:            terminalDefaultCols,
		commandReliable: true,
		find: terminalFindState{
			previewIndex: -1,
			previewStart: 0,
			previewEnd:   2,
		},
		modes: terminalModes{
			eraseTemplate: headlessterm.NewCellTemplate(),
		},
	}
	st.term = headlessterm.New(
		headlessterm.WithSize(rows, terminalDefaultCols),
		headlessterm.WithScrollback(headlessterm.NewMemoryScrollback(4000)),
		headlessterm.WithSixel(false),
		headlessterm.WithKitty(false),
		headlessterm.WithMiddleware(st.terminalMiddleware()),
	)
	st.setInvalidate(invalidate)
	st.snapshot()
	return st
}

func (s *terminalSession) terminalMiddleware() *headlessterm.Middleware {
	return &headlessterm.Middleware{
		Input: func(r rune, next func(rune)) {
			if s.inputTerminalNarrowRune(r, next) {
				return
			}
			next(r)
		},
		SetMode: func(mode ansicode.TerminalMode, next func(ansicode.TerminalMode)) {
			s.setTerminalMode(mode, true)
			next(mode)
		},
		UnsetMode: func(mode ansicode.TerminalMode, next func(ansicode.TerminalMode)) {
			s.setTerminalMode(mode, false)
			next(mode)
		},
		SetTerminalCharAttribute: func(attr ansicode.TerminalCharAttribute, next func(ansicode.TerminalCharAttribute)) {
			next(attr)
			s.updateTerminalEraseTemplate(attr)
		},
		ClearLine: func(mode ansicode.LineClearMode, next func(ansicode.LineClearMode)) {
			row, col, rows, cols := s.terminalCursorSnapshot()
			next(mode)
			s.applyTerminalEraseLine(mode, row, col, rows, cols)
		},
		ClearScreen: func(mode ansicode.ClearMode, next func(ansicode.ClearMode)) {
			row, col, rows, cols := s.terminalCursorSnapshot()
			next(mode)
			s.applyTerminalEraseScreen(mode, row, col, rows, cols)
		},
		MoveUpCr: func(n int, next func(int)) {
			next(terminalCorrectLineMoveCount(n))
		},
		MoveDownCr: func(n int, next func(int)) {
			next(terminalCorrectLineMoveCount(n))
		},
		SetScrollingRegion: func(top, bottom int, next func(int, int)) {
			_, _, rows, _ := s.terminalCursorSnapshot()
			next(terminalCorrectScrollRegion(top, bottom, rows))
		},
		EraseChars: func(n int, next func(int)) {
			row, col, rows, cols := s.terminalCursorSnapshot()
			next(n)
			if n > 0 && row >= 0 && row < rows {
				s.applyTerminalEraseRange(row, col, col+n, cols)
			}
		},
		InsertBlank: func(n int, next func(int)) {
			row, col, rows, cols := s.terminalCursorSnapshot()
			next(n)
			if n > 0 && row >= 0 && row < rows {
				s.applyTerminalEraseRange(row, col, col+n, cols)
			}
		},
		DeleteChars: func(n int, next func(int)) {
			row, _, rows, cols := s.terminalCursorSnapshot()
			next(n)
			if n > 0 && row >= 0 && row < rows {
				s.applyTerminalEraseRange(row, cols-n, cols, cols)
			}
		},
		ResetState: func(next func()) {
			s.resetTerminalModes()
			next()
		},
	}
}

func (s *terminalSession) inputTerminalNarrowRune(r rune, next func(rune)) bool {
	if !terminalNarrowStatusRune(r) || s == nil || s.term == nil {
		return false
	}
	// Homebrew emits these as one-cell text symbols; write a one-cell glyph
	// with current attributes, then restore the original rune in that cell.
	beforeRow, beforeCol := s.term.CursorPos()
	next(' ')
	row, col := s.term.CursorPos()
	writeRow, writeCol := row, col-1
	if writeCol < 0 {
		writeRow, writeCol = beforeRow, beforeCol
		if cols := s.term.Cols(); cols > 0 && writeCol >= cols {
			writeCol = cols - 1
		}
	}
	cell := s.term.Cell(writeRow, writeCol)
	if cell == nil {
		return true
	}
	cell.Char = r
	cell.ClearFlag(headlessterm.CellFlagWideChar | headlessterm.CellFlagWideCharSpacer)
	cell.MarkDirty()
	if nextCell := s.term.Cell(writeRow, writeCol+1); nextCell != nil && nextCell.HasFlag(headlessterm.CellFlagWideCharSpacer) {
		blank := cell.Copy()
		blank.Char = ' '
		blank.ClearFlag(headlessterm.CellFlagWideChar | headlessterm.CellFlagWideCharSpacer)
		*nextCell = blank
		nextCell.MarkDirty()
	}
	return true
}

func terminalNarrowStatusRune(r rune) bool {
	switch r {
	case '✔', '✘':
		return true
	default:
		return false
	}
}

func terminalCorrectLineMoveCount(n int) int {
	// go-ansicode v1.0.14 dispatches CSI E/F as MoveDownCr/MoveUpCr(n-1).
	if n < 0 {
		n = 0
	}
	return n + 1
}

func terminalCorrectScrollRegion(top, bottom, rows int) (int, int) {
	// go-headless-term v1.0.9 treats the 1-based inclusive bottom as though it
	// were already exclusive. Pass one extra row so regions like CSI 2;23 r
	// include row 23, which fullscreen tools such as mc rely on when scrolling.
	if rows < 1 {
		rows = terminalDefaultRows
	}
	if top < 1 {
		top = 1
	}
	if bottom <= top {
		bottom = rows
	}
	if bottom > rows {
		bottom = rows
	}
	return top, bottom + 1
}

type terminalModes struct {
	cursorKeys      bool
	mouseClicks     bool
	mouseCellMotion bool
	mouseAllMotion  bool
	mouseUTF8       bool
	mouseSGR        bool
	alternateScroll bool
	bracketedPaste  bool
	mousePressed    bool
	mousePointer    pointer.ID
	mouseButton     int
	eraseTemplate   headlessterm.CellTemplate
}

func (s *terminalSession) setTerminalMode(mode ansicode.TerminalMode, enabled bool) {
	if s == nil {
		return
	}
	s.modeMu.Lock()
	switch mode {
	case ansicode.TerminalModeCursorKeys:
		s.modes.cursorKeys = enabled
	case ansicode.TerminalModeReportMouseClicks:
		s.modes.mouseClicks = enabled
	case ansicode.TerminalModeReportCellMouseMotion:
		s.modes.mouseCellMotion = enabled
	case ansicode.TerminalModeReportAllMouseMotion:
		s.modes.mouseAllMotion = enabled
	case ansicode.TerminalModeUTF8Mouse:
		s.modes.mouseUTF8 = enabled
	case ansicode.TerminalModeSGRMouse:
		s.modes.mouseSGR = enabled
	case ansicode.TerminalModeAlternateScroll:
		s.modes.alternateScroll = enabled
	case ansicode.TerminalModeBracketedPaste:
		s.modes.bracketedPaste = enabled
	}
	s.modeMu.Unlock()
}

func (s *terminalSession) cursorKeysApplication() bool {
	if s == nil {
		return false
	}
	s.modeMu.RLock()
	defer s.modeMu.RUnlock()
	return s.modes.cursorKeys
}

func (s *terminalSession) terminalMouseModes() terminalMouseReportModes {
	if s == nil {
		return terminalMouseReportModes{}
	}
	s.modeMu.RLock()
	defer s.modeMu.RUnlock()
	return terminalMouseReportModes{
		clicks:     s.modes.mouseClicks,
		cellMotion: s.modes.mouseCellMotion,
		allMotion:  s.modes.mouseAllMotion,
		utf8:       s.modes.mouseUTF8,
		sgr:        s.modes.mouseSGR,
	}
}

func (s *terminalSession) terminalMouseReporting() bool {
	modes := s.terminalMouseModes()
	return modes.reporting()
}

func (s *terminalSession) bracketedPasteMode() bool {
	if s == nil {
		return false
	}
	s.modeMu.RLock()
	defer s.modeMu.RUnlock()
	return s.modes.bracketedPaste
}

func (s *terminalSession) beginTerminalMousePress(id pointer.ID, button int) {
	if s == nil {
		return
	}
	s.modeMu.Lock()
	s.modes.mousePressed = true
	s.modes.mousePointer = id
	s.modes.mouseButton = button
	s.modeMu.Unlock()
}

func (s *terminalSession) terminalMousePressButton(id pointer.ID) (button int, ok bool) {
	if s == nil {
		return 0, false
	}
	s.modeMu.RLock()
	defer s.modeMu.RUnlock()
	if !s.modes.mousePressed || s.modes.mousePointer != id {
		return 0, false
	}
	return s.modes.mouseButton, true
}

func (s *terminalSession) endTerminalMousePress(id pointer.ID) (button int, ok bool) {
	if s == nil {
		return 0, false
	}
	s.modeMu.Lock()
	defer s.modeMu.Unlock()
	if !s.modes.mousePressed || s.modes.mousePointer != id {
		return 0, false
	}
	button = s.modes.mouseButton
	s.modes.mousePressed = false
	s.modes.mousePointer = 0
	s.modes.mouseButton = 0
	return button, true
}

func (s *terminalSession) resetTerminalModes() {
	if s == nil {
		return
	}
	s.modeMu.Lock()
	s.modes = terminalModes{eraseTemplate: headlessterm.NewCellTemplate()}
	s.modeMu.Unlock()
}

func (s *terminalSession) updateTerminalEraseTemplate(attr ansicode.TerminalCharAttribute) {
	if s == nil {
		return
	}
	s.modeMu.Lock()
	defer s.modeMu.Unlock()
	switch attr.Attr {
	case ansicode.CharAttributeReset:
		s.modes.eraseTemplate = headlessterm.NewCellTemplate()
	case ansicode.CharAttributeBold:
		s.modes.eraseTemplate.SetFlag(headlessterm.CellFlagBold)
	case ansicode.CharAttributeDim:
		s.modes.eraseTemplate.SetFlag(headlessterm.CellFlagDim)
	case ansicode.CharAttributeItalic:
		s.modes.eraseTemplate.SetFlag(headlessterm.CellFlagItalic)
	case ansicode.CharAttributeUnderline:
		s.modes.eraseTemplate.SetFlag(headlessterm.CellFlagUnderline)
		s.modes.eraseTemplate.ClearFlag(headlessterm.CellFlagDoubleUnderline | headlessterm.CellFlagCurlyUnderline | headlessterm.CellFlagDottedUnderline | headlessterm.CellFlagDashedUnderline)
	case ansicode.CharAttributeDoubleUnderline:
		s.modes.eraseTemplate.SetFlag(headlessterm.CellFlagDoubleUnderline)
		s.modes.eraseTemplate.ClearFlag(headlessterm.CellFlagUnderline | headlessterm.CellFlagCurlyUnderline | headlessterm.CellFlagDottedUnderline | headlessterm.CellFlagDashedUnderline)
	case ansicode.CharAttributeCurlyUnderline:
		s.modes.eraseTemplate.SetFlag(headlessterm.CellFlagCurlyUnderline)
		s.modes.eraseTemplate.ClearFlag(headlessterm.CellFlagUnderline | headlessterm.CellFlagDoubleUnderline | headlessterm.CellFlagDottedUnderline | headlessterm.CellFlagDashedUnderline)
	case ansicode.CharAttributeDottedUnderline:
		s.modes.eraseTemplate.SetFlag(headlessterm.CellFlagDottedUnderline)
		s.modes.eraseTemplate.ClearFlag(headlessterm.CellFlagUnderline | headlessterm.CellFlagDoubleUnderline | headlessterm.CellFlagCurlyUnderline | headlessterm.CellFlagDashedUnderline)
	case ansicode.CharAttributeDashedUnderline:
		s.modes.eraseTemplate.SetFlag(headlessterm.CellFlagDashedUnderline)
		s.modes.eraseTemplate.ClearFlag(headlessterm.CellFlagUnderline | headlessterm.CellFlagDoubleUnderline | headlessterm.CellFlagCurlyUnderline | headlessterm.CellFlagDottedUnderline)
	case ansicode.CharAttributeBlinkSlow:
		s.modes.eraseTemplate.SetFlag(headlessterm.CellFlagBlinkSlow)
	case ansicode.CharAttributeBlinkFast:
		s.modes.eraseTemplate.SetFlag(headlessterm.CellFlagBlinkFast)
	case ansicode.CharAttributeReverse:
		s.modes.eraseTemplate.SetFlag(headlessterm.CellFlagReverse)
	case ansicode.CharAttributeHidden:
		s.modes.eraseTemplate.SetFlag(headlessterm.CellFlagHidden)
	case ansicode.CharAttributeStrike:
		s.modes.eraseTemplate.SetFlag(headlessterm.CellFlagStrike)
	case ansicode.CharAttributeCancelBold:
		s.modes.eraseTemplate.ClearFlag(headlessterm.CellFlagBold)
	case ansicode.CharAttributeCancelBoldDim:
		s.modes.eraseTemplate.ClearFlag(headlessterm.CellFlagBold | headlessterm.CellFlagDim)
	case ansicode.CharAttributeCancelItalic:
		s.modes.eraseTemplate.ClearFlag(headlessterm.CellFlagItalic)
	case ansicode.CharAttributeCancelUnderline:
		s.modes.eraseTemplate.ClearFlag(headlessterm.CellFlagUnderline | headlessterm.CellFlagDoubleUnderline | headlessterm.CellFlagCurlyUnderline | headlessterm.CellFlagDottedUnderline | headlessterm.CellFlagDashedUnderline)
	case ansicode.CharAttributeCancelBlink:
		s.modes.eraseTemplate.ClearFlag(headlessterm.CellFlagBlinkSlow | headlessterm.CellFlagBlinkFast)
	case ansicode.CharAttributeCancelReverse:
		s.modes.eraseTemplate.ClearFlag(headlessterm.CellFlagReverse)
	case ansicode.CharAttributeCancelHidden:
		s.modes.eraseTemplate.ClearFlag(headlessterm.CellFlagHidden)
	case ansicode.CharAttributeCancelStrike:
		s.modes.eraseTemplate.ClearFlag(headlessterm.CellFlagStrike)
	case ansicode.CharAttributeForeground:
		s.modes.eraseTemplate.Fg = terminalANSICodeColor(attr, true)
	case ansicode.CharAttributeBackground:
		s.modes.eraseTemplate.Bg = terminalANSICodeColor(attr, false)
	}
}

func terminalANSICodeColor(attr ansicode.TerminalCharAttribute, fg bool) color.Color {
	if attr.RGBColor != nil {
		return color.RGBA{R: attr.RGBColor.R, G: attr.RGBColor.G, B: attr.RGBColor.B, A: 255}
	}
	if attr.IndexedColor != nil {
		return &headlessterm.IndexedColor{Index: int(attr.IndexedColor.Index)}
	}
	if attr.NamedColor != nil {
		return &headlessterm.NamedColor{Name: int(*attr.NamedColor)}
	}
	if fg {
		return &headlessterm.NamedColor{Name: headlessterm.NamedColorForeground}
	}
	return &headlessterm.NamedColor{Name: headlessterm.NamedColorBackground}
}

func (s *terminalSession) terminalEraseCell() headlessterm.Cell {
	if s == nil {
		return headlessterm.NewCell()
	}
	s.modeMu.RLock()
	cell := s.modes.eraseTemplate.Cell.Copy()
	s.modeMu.RUnlock()
	cell.Char = ' '
	cell.ClearFlag(headlessterm.CellFlagWideChar | headlessterm.CellFlagWideCharSpacer | headlessterm.CellFlagDirty)
	cell.Hyperlink = nil
	cell.Image = nil
	return cell
}

func (s *terminalSession) terminalCursorSnapshot() (row, col, rows, cols int) {
	if s == nil || s.term == nil {
		return 0, 0, 0, 0
	}
	row, col = s.term.CursorPos()
	return row, col, s.term.Rows(), s.term.Cols()
}

func (s *terminalSession) applyTerminalEraseLine(mode ansicode.LineClearMode, row, col, rows, cols int) {
	if row < 0 || row >= rows || cols <= 0 {
		return
	}
	switch mode {
	case ansicode.LineClearModeRight:
		s.applyTerminalEraseRange(row, col, cols, cols)
	case ansicode.LineClearModeLeft:
		s.applyTerminalEraseRange(row, 0, col+1, cols)
	case ansicode.LineClearModeAll:
		s.applyTerminalEraseRange(row, 0, cols, cols)
	}
}

func (s *terminalSession) applyTerminalEraseScreen(mode ansicode.ClearMode, row, col, rows, cols int) {
	if rows <= 0 || cols <= 0 {
		return
	}
	switch mode {
	case ansicode.ClearModeBelow:
		if row >= 0 && row < rows {
			s.applyTerminalEraseRange(row, col, cols, cols)
		}
		for y := row + 1; y < rows; y++ {
			s.applyTerminalEraseRange(y, 0, cols, cols)
		}
	case ansicode.ClearModeAbove:
		for y := 0; y < row && y < rows; y++ {
			s.applyTerminalEraseRange(y, 0, cols, cols)
		}
		if row >= 0 && row < rows {
			s.applyTerminalEraseRange(row, 0, col+1, cols)
		}
	case ansicode.ClearModeAll, ansicode.ClearModeSaved:
		for y := 0; y < rows; y++ {
			s.applyTerminalEraseRange(y, 0, cols, cols)
		}
	}
}

func (s *terminalSession) applyTerminalEraseRange(row, fromCol, toCol, cols int) {
	if s == nil || s.term == nil || row < 0 || cols <= 0 {
		return
	}
	if fromCol < 0 {
		fromCol = 0
	}
	if toCol > cols {
		toCol = cols
	}
	if fromCol >= toCol {
		return
	}
	template := s.terminalEraseCell()
	for col := fromCol; col < toCol; col++ {
		cell := s.term.Cell(row, col)
		if cell == nil {
			continue
		}
		*cell = template.Copy()
		cell.MarkDirty()
	}
}

func (ui *UI) SetInvalidateFunc(fn func()) {
	if ui == nil {
		return
	}
	ui.invalidate = fn
	if ui.terminal != nil {
		ui.terminal.setInvalidate(fn)
	}
}

func (ui *UI) Close() {
	if ui == nil {
		return
	}
	ui.closeAllTerminalTabs()
}

func (ui *UI) ensureTerminalSession() *terminalSession {
	if ui == nil {
		return nil
	}
	ui.ensureTerminalTabs()
	return ui.terminal
}

func (ui *UI) toggleTerminal() bool {
	st := ui.ensureTerminalSession()
	if st == nil {
		return false
	}
	active := !st.active()
	st.setActive(active)
	if active {
		ui.closeFunctionBarPopups()
		ui.resetKeys()
		st.focusKeyboard()
	} else {
		ui.closeTerminalSnippetMenu()
		st.closeFind()
		st.wantFocus = false
	}
	return true
}

func (ui *UI) terminalMaximized() bool {
	return ui != nil &&
		ui.fmCfg != nil &&
		ui.fmCfg.Terminal.Maximized &&
		ui.terminal != nil &&
		ui.terminal.active()
}

func (ui *UI) toggleTerminalMaximized() bool {
	st := ui.ensureTerminalSession()
	if st == nil || ui.fmCfg == nil {
		return false
	}
	next := !ui.fmCfg.Terminal.Maximized
	ui.fmCfg.Terminal.Maximized = next
	ui.functionBarTerminalShown = false
	st.setActive(true)
	st.focusKeyboard()
	ui.closeFunctionBarPopups()
	ui.resetKeys()
	if err := ui.saveFMConfigWithOptions("terminal-layout", false); err != nil {
		st.setError("save terminal layout failed: " + err.Error())
	}
	if ui.invalidate != nil {
		ui.invalidate()
	}
	return true
}

func (ui *UI) handleTerminalFocusToggleKey(gtx layout.Context) bool {
	if ui == nil || ui.terminal == nil || !ui.terminal.active() {
		return false
	}
	terminalFocused := gtx.Focused(&ui.terminal.keyTag)
	if ui.helpModal != nil || ui.settingsModal != nil || ui.sshModal != nil || ui.hasBlockingFileDialog() {
		return false
	}
	if !terminalFocused && (ui.Tabs.Value != "tab0" || ui.fileViewer != nil || ui.pathEditActive()) {
		return false
	}
	anyMods := ^key.Modifiers(0)
	handled := false
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameTab, Required: key.ModShift, Optional: anyMods})
		if !ok {
			return handled
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		handled = true
		if ke.Modifiers != key.ModShift {
			continue
		}
		if ui.toggleTerminalKeyboardFocus(gtx, terminalFocused) {
			terminalFocused = !terminalFocused
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (ui *UI) toggleTerminalKeyboardFocus(gtx layout.Context, terminalFocused bool) bool {
	if ui == nil || ui.terminal == nil || !ui.terminal.active() {
		return false
	}
	ui.closeFunctionBarPopups()
	ui.resetKeys()
	if terminalFocused {
		if ui.Tabs.Value != "tab0" {
			ui.setActiveTab("tab0", gtx.Now)
		}
		ui.releaseTerminalKeyboardFocus(gtx)
		return true
	}
	ui.terminal.focusKeyboard()
	gtx.Execute(key.FocusCmd{Tag: &ui.terminal.keyTag})
	gtx.Execute(key.SoftKeyboardCmd{Show: true})
	return true
}

func (ui *UI) terminalFocused(gtx layout.Context) bool {
	return ui != nil && ui.terminal != nil && ui.terminal.active() && ui.terminal.keyboardFocused(gtx)
}

func (ui *UI) terminalVisuallyFocused(gtx layout.Context) bool {
	if ui == nil || ui.terminal == nil || !ui.terminal.active() {
		return false
	}
	return ui.terminal.wantFocus || ui.terminal.keyboardFocused(gtx)
}

func (s *terminalSession) keyboardFocused(gtx layout.Context) bool {
	if s == nil {
		return false
	}
	return gtx.Focused(&s.keyTag) || (s.find.open && gtx.Focused(&s.find.editor))
}

func (ui *UI) releaseTerminalKeyboardFocus(gtx layout.Context) bool {
	if ui == nil || ui.terminal == nil {
		return false
	}
	ui.terminal.wantFocus = false
	gtx.Execute(key.FocusCmd{})
	gtx.Execute(key.SoftKeyboardCmd{Show: false})
	return true
}

func (ui *UI) handleTerminalOutsidePointerFocus(gtx layout.Context) bool {
	if ui == nil {
		return false
	}
	terminalActive := ui.terminal != nil && ui.terminal.active()
	terminalFocused := terminalActive && gtx.Focused(&ui.terminal.keyTag)
	changed := false
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &ui.terminalFocusPointerTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			return changed
		}
		pe, ok := ev.(pointer.Event)
		if !ok || !terminalOutsideFocusPointerEvent(pe) {
			continue
		}
		if !terminalActive {
			continue
		}
		if !terminalFocused {
			continue
		}
		pos := pe.Position.Round()
		if ui.terminalPointerOnTerminalSurface(gtx, pos) {
			continue
		}
		if ui.releaseTerminalKeyboardFocus(gtx) {
			terminalFocused = false
			changed = true
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func terminalOutsideFocusPointerEvent(pe pointer.Event) bool {
	return pe.Kind == pointer.Press && terminalPointerPressButton(pe.Buttons)
}

func terminalSurfaceFocusPointerEvent(pe pointer.Event) bool {
	switch pe.Kind {
	case pointer.Press:
		return terminalPointerPressButton(pe.Buttons)
	case pointer.Scroll:
		return pe.Scroll.X != 0 || pe.Scroll.Y != 0
	default:
		return false
	}
}

func terminalPointerPressButton(buttons pointer.Buttons) bool {
	return buttons.Contain(pointer.ButtonPrimary) ||
		buttons.Contain(pointer.ButtonSecondary) ||
		buttons.Contain(pointer.ButtonTertiary)
}

func (ui *UI) terminalPointerOnTerminalSurface(gtx layout.Context, pos image.Point) bool {
	if ui == nil || ui.terminal == nil || !ui.terminal.active() {
		return false
	}
	if ui.terminal.contextMenuConsumesPointer(pos) {
		return true
	}
	height, _, ok := ui.terminal.paneMetrics()
	if !ok {
		height = ui.terminalPaneHeightWithTabs(gtx, 16, terminalConfiguredRows(ui.fmCfg))
	}
	if height <= 0 {
		return false
	}
	bounds := gtx.Constraints.Max
	if bounds.X <= 0 || bounds.Y <= 0 {
		return false
	}
	if height > bounds.Y {
		height = bounds.Y
	}
	return viewerPointInRect(pos, image.Rect(0, bounds.Y-height, bounds.X, bounds.Y))
}

func (ui *UI) registerTerminalOutsidePointerFocus(gtx layout.Context) {
	if ui == nil || ui.terminal == nil || !ui.terminal.active() {
		return
	}
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &ui.terminalFocusPointerTag)
	pass.Pop()
}

func (ui *UI) handleTerminalToggleKey(gtx layout.Context) bool {
	if ui == nil {
		return false
	}
	handled := false
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameF12})
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		handled = true
		if ke.Modifiers == 0 && ui.toggleTerminal() {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	return handled
}

func (ui *UI) handleTerminalFunctionBarToggleKey(gtx layout.Context) bool {
	if ui == nil || !ui.terminalMaximized() {
		return false
	}
	handled := false
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameF11})
		if !ok {
			return handled
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		handled = true
		if ke.Modifiers == 0 && ui.toggleFunctionBarVisibility(gtx.Now) {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (s *terminalSession) setInvalidate(fn func()) {
	if s == nil {
		return
	}
	s.invalidateMu.Lock()
	s.invalidate = fn
	s.invalidateMu.Unlock()
}

func (s *terminalSession) invalidateNow() {
	if s == nil {
		return
	}
	s.invalidateMu.RLock()
	invalidate := s.invalidate
	s.invalidateMu.RUnlock()
	if invalidate != nil {
		invalidate()
	}
}

func (s *terminalSession) active() bool {
	if s == nil {
		return false
	}
	s.State.Mu.RLock()
	defer s.State.Mu.RUnlock()
	return s.State.Active
}

func (s *terminalSession) setActive(active bool) {
	if s == nil {
		return
	}
	if !active {
		s.stopKeyRepeat("")
	}
	s.procMu.Lock()
	if active && !s.running {
		s.startAttempted = false
	}
	s.procMu.Unlock()

	s.State.Mu.Lock()
	s.State.Active = active
	if active {
		s.State.Err = ""
	}
	s.State.Mu.Unlock()
}

func (s *terminalSession) focusKeyboard() {
	if s != nil {
		s.wantFocus = true
	}
}

func (s *terminalSession) Close() {
	if s == nil {
		return
	}
	s.stopKeyRepeat("")
	s.procMu.Lock()
	s.closing = true
	proc := s.pty
	s.pty = nil
	s.running = false
	s.procMu.Unlock()

	if proc != nil {
		_ = proc.Kill()
		_ = proc.Close()
	}
}

func (s *terminalSession) start(cwd, shell string) {
	if s == nil {
		return
	}
	s.procMu.Lock()
	if s.running || s.startAttempted || s.closing {
		s.procMu.Unlock()
		return
	}
	s.startAttempted = true
	if pending := strings.TrimSpace(s.pendingStartDir); pending != "" {
		cwd = pending
		s.pendingStartDir = ""
	}
	rows, cols := s.rows, s.cols
	s.procMu.Unlock()

	if rows <= 0 {
		rows = terminalDefaultRows
	}
	if cols <= 0 {
		cols = terminalDefaultCols
	}

	name, args := terminalCommandForShell(shell, cwd)
	proc, err := startTerminalProcess(name, args, cwd, terminalEnv(os.Environ(), rows, cols), rows, cols)
	if err != nil {
		s.setError(terminalStartError(err))
		return
	}

	s.parserMu.Lock()
	s.term.SetPTYWriter(proc)
	s.parserMu.Unlock()

	s.procMu.Lock()
	if s.closing {
		s.procMu.Unlock()
		_ = proc.Close()
		_ = proc.Kill()
		return
	}
	s.pty = proc
	s.running = true
	s.startDir = cwd
	s.procMu.Unlock()

	s.setError("")
	go s.readLoop(proc)
}

func (s *terminalSession) readLoop(proc terminalProcess) {
	buf := make([]byte, terminalReadBufSize)
	for {
		n, err := proc.Read(buf)
		if n > 0 {
			s.writeOutput(buf[:n])
			s.invalidateNow()
		}
		if err != nil {
			if !errors.Is(err, os.ErrClosed) && !errors.Is(err, io.EOF) {
				s.setError(err.Error())
			}
			break
		}
	}
	if proc != nil {
		_ = proc.Wait()
	}

	s.procMu.Lock()
	closing := s.closing
	if s.pty == proc {
		s.pty = nil
		s.running = false
	}
	s.procMu.Unlock()
	if !closing {
		_ = proc.Close()
	}

	if !closing {
		s.setError("terminal exited; close and reopen the drawer to start a new shell")
	}
	s.invalidateNow()
}

func (s *terminalSession) writeOutput(data []byte) {
	if s == nil || s.term == nil || len(data) == 0 {
		return
	}
	s.parserMu.Lock()
	normalized := s.outputNorm.normalize(data)
	_, _ = s.term.Write(normalized)
	s.parserMu.Unlock()
	s.snapshot()
}

type terminalOutputNormalizer struct {
	pending []byte
}

func terminalNormalizeC1Controls(data []byte) []byte {
	var norm terminalOutputNormalizer
	return norm.normalize(data)
}

func (n *terminalOutputNormalizer) normalize(data []byte) []byte {
	var out []byte
	if len(n.pending) > 0 {
		merged := make([]byte, 0, len(n.pending)+len(data))
		merged = append(merged, n.pending...)
		merged = append(merged, data...)
		data = merged
		n.pending = n.pending[:0]
	}
	for i := 0; i < len(data); {
		b := data[i]
		if cont := terminalUTF8ContinuationCount(b); cont > 0 {
			size := cont + 1
			if i+size > len(data) {
				n.pending = append(n.pending[:0], data[i:]...)
				if out == nil {
					return data[:i]
				}
				return out
			}
			seq := data[i : i+size]
			if utf8.Valid(seq) {
				if size == 2 && b == 0xc2 {
					if repl, ok := terminalC1Replacement(seq[1]); ok {
						if out == nil {
							out = make([]byte, 0, len(data)+2)
							out = append(out, data[:i]...)
						}
						out = append(out, repl...)
						i += size
						continue
					}
				}
				if out != nil {
					out = append(out, seq...)
				}
				i += size
				continue
			}
		}
		repl, ok := terminalC1Replacement(b)
		if !ok {
			if out != nil {
				out = append(out, b)
			}
			i++
			continue
		}
		if out == nil {
			out = make([]byte, 0, len(data)+2)
			out = append(out, data[:i]...)
		}
		out = append(out, repl...)
		i++
	}
	if out == nil {
		return data
	}
	return out
}

func terminalUTF8ContinuationCount(b byte) int {
	switch {
	case b >= 0xc2 && b <= 0xdf:
		return 1
	case b >= 0xe0 && b <= 0xef:
		return 2
	case b >= 0xf0 && b <= 0xf4:
		return 3
	default:
		return 0
	}
}

func terminalC1Replacement(b byte) ([]byte, bool) {
	switch b {
	case 0x84:
		return []byte{0x1b, 'D'}, true // IND
	case 0x85:
		return []byte{0x1b, 'E'}, true // NEL
	case 0x88:
		return []byte{0x1b, 'H'}, true // HTS
	case 0x8d:
		return []byte{0x1b, 'M'}, true // RI
	case 0x8e:
		return []byte{0x1b, 'N'}, true // SS2
	case 0x8f:
		return []byte{0x1b, 'O'}, true // SS3
	case 0x90:
		return []byte{0x1b, 'P'}, true // DCS
	case 0x98:
		return []byte{0x1b, 'X'}, true // SOS
	case 0x9b:
		return []byte{0x1b, '['}, true // CSI
	case 0x9c:
		return []byte{0x1b, '\\'}, true // ST
	case 0x9d:
		return []byte{0x1b, ']'}, true // OSC
	case 0x9e:
		return []byte{0x1b, '^'}, true // PM
	case 0x9f:
		return []byte{0x1b, '_'}, true // APC
	default:
		return nil, false
	}
}

func (s *terminalSession) resize(rows, cols int) {
	if s == nil || rows <= 0 || cols <= 0 {
		return
	}
	s.procMu.Lock()
	if s.rows == rows && s.cols == cols {
		s.procMu.Unlock()
		return
	}
	s.rows = rows
	s.cols = cols
	proc := s.pty
	running := s.running
	s.procMu.Unlock()

	s.parserMu.Lock()
	s.term.Resize(rows, cols)
	s.parserMu.Unlock()

	if proc != nil && running {
		_ = proc.Resize(rows, cols)
	}
	s.snapshot()
}

func (s *terminalSession) write(data []byte) {
	if s == nil || len(data) == 0 {
		return
	}
	if s.scrollToBottom() {
		s.snapshot()
	}
	s.procMu.Lock()
	proc := s.pty
	running := s.running
	s.procMu.Unlock()
	if proc == nil || !running {
		return
	}
	s.trackCommandInput(data)
	s.writeMu.Lock()
	_, err := proc.Write(data)
	s.writeMu.Unlock()
	if err != nil {
		s.setError(err.Error())
	}
}

func (s *terminalSession) writeString(text string) {
	if text != "" {
		s.write([]byte(text))
	}
}

func (s *terminalSession) scrollToBottom() bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	changed := s.scrollOffset != 0 || s.scrollCarry != 0
	s.scrollOffset = 0
	s.scrollCarry = 0
	s.viewMu.Unlock()
	return changed
}

func (s *terminalSession) scrollByDelta(delta float32) bool {
	if s == nil || delta == 0 {
		return false
	}
	delta *= terminalWheelAccel
	if delta > 3 {
		delta = 3
	} else if delta < -3 {
		delta = -3
	}
	s.viewMu.Lock()
	if (delta > 0 && s.scrollCarry < 0) || (delta < 0 && s.scrollCarry > 0) {
		s.scrollCarry = 0
	}
	s.scrollCarry += delta
	steps := 0
	for s.scrollCarry >= 1 {
		steps++
		s.scrollCarry -= 1
	}
	for s.scrollCarry <= -1 {
		steps--
		s.scrollCarry += 1
	}
	s.viewMu.Unlock()
	if steps == 0 {
		return false
	}
	return s.scrollByLines(-steps)
}

func (s *terminalSession) scrollByLines(lines int) bool {
	if s == nil || lines == 0 {
		return false
	}
	s.viewMu.Lock()
	prev := s.scrollOffset
	s.scrollOffset += lines
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
	if s.scrollOffset > s.lastScrollbackLen {
		s.scrollOffset = s.lastScrollbackLen
	}
	changed := s.scrollOffset != prev
	s.viewMu.Unlock()
	if changed {
		s.snapshot()
	}
	return changed
}

func (s *terminalSession) setScrollOffset(offset int) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	prev := s.scrollOffset
	if offset < 0 {
		offset = 0
	}
	if offset > s.lastScrollbackLen {
		offset = s.lastScrollbackLen
	}
	s.scrollOffset = offset
	s.scrollCarry = 0
	changed := s.scrollOffset != prev
	s.viewMu.Unlock()
	if changed {
		s.snapshot()
	}
	return changed
}

func (s *terminalSession) prepareVisualScroll(now time.Time, smooth bool) bool {
	if s == nil {
		return false
	}
	s.State.Mu.RLock()
	target := float32(s.State.ViewStart)
	rows := s.State.Rows
	scrollback := s.State.Scrollback
	alternate := s.State.Alternate
	s.State.Mu.RUnlock()
	if rows <= 0 || alternate {
		s.viewMu.Lock()
		s.visualTop = target
		s.visualReady = true
		s.visualAt = now
		s.viewMu.Unlock()
		return false
	}

	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if !s.visualReady {
		s.visualTop = target
		s.visualReady = true
		s.visualAt = now
		return false
	}
	selectionAutoScroll := s.selectionSelecting && s.autoScrollDir != 0 && s.autoScrollStep > 0
	if !smooth || s.scrollbarDragging || selectionAutoScroll {
		s.visualTop = target
		s.visualAt = now
		return false
	}
	jumpLimit := float32(terminalSmoothJumpMinLines)
	if visible := float32(rows) * 0.75; visible > jumpLimit {
		jumpLimit = visible
	}
	if diff := float32Abs(target - s.visualTop); diff > jumpLimit {
		s.visualTop = target
		s.visualAt = now
		return false
	}

	if s.visualAt.IsZero() {
		s.visualAt = now
	}
	dt := now.Sub(s.visualAt)
	if dt < 0 {
		dt = 0
	}
	if dt > 120*time.Millisecond {
		s.visualTop = target
		s.visualAt = now
		return false
	}
	if dt == 0 && target != s.visualTop {
		dt = terminalSmoothTick
	}
	if dt > 0 {
		blend := float32(1 - math.Exp(-float64(dt)/float64(terminalSmoothTau)))
		s.visualTop += (target - s.visualTop) * clamp01(blend)
	}
	if s.visualTop < 0 {
		s.visualTop = 0
	}
	if maxTop := float32(scrollback); s.visualTop > maxTop {
		s.visualTop = maxTop
	}
	s.visualAt = now
	if float32Abs(target-s.visualTop) < terminalSmoothSnapEpsilon {
		s.visualTop = target
		return false
	}
	return true
}

func (s *terminalSession) displayRows(cellH int) (top, offsetY, count int) {
	if s == nil {
		return 0, 0, 0
	}
	s.State.Mu.RLock()
	rows := s.State.Rows
	scrollback := s.State.Scrollback
	target := s.State.ViewStart
	s.State.Mu.RUnlock()
	if rows <= 0 {
		return target, 0, 0
	}

	s.viewMu.Lock()
	visualTop := float32(target)
	if s.visualReady {
		visualTop = s.visualTop
	}
	s.viewMu.Unlock()

	maxTop := float32(scrollback)
	if visualTop < 0 {
		visualTop = 0
	}
	if visualTop > maxTop {
		visualTop = maxTop
	}
	top = int(math.Floor(float64(visualTop)))
	frac := visualTop - float32(top)
	if frac < 0 {
		frac = 0
	}
	if frac > 0 && top >= int(maxTop) {
		top = int(maxTop)
		frac = 0
	}
	if cellH > 0 && frac > 0 {
		offsetY = -int(math.Round(float64(frac * float32(cellH))))
		if offsetY <= -cellH {
			offsetY = 0
			top++
		}
	}
	if top < 0 {
		top = 0
	}
	if top > int(maxTop) {
		top = int(maxTop)
		offsetY = 0
	}
	count = rows
	total := scrollback + rows
	if offsetY < 0 && top+count < total {
		count++
	}
	if count < 1 {
		count = 1
	}
	return top, offsetY, count
}

func (s *terminalSession) terminalPointFromPosition(pos image.Point, content image.Rectangle, cellW, cellH int) (terminalPoint, bool) {
	if s == nil || cellW <= 0 || cellH <= 0 || content.Dx() <= 0 || content.Dy() <= 0 {
		return terminalPoint{}, false
	}
	s.State.Mu.RLock()
	rows := s.State.Rows
	cols := s.State.Cols
	viewStart := s.State.ViewStart
	s.State.Mu.RUnlock()
	if rows <= 0 || cols <= 0 {
		return terminalPoint{}, false
	}
	col := (pos.X - content.Min.X) / cellW
	if col < 0 {
		col = 0
	}
	if col >= cols {
		col = cols - 1
	}
	if pos.Y < content.Min.Y {
		return terminalPoint{Row: viewStart, Col: col}, true
	}
	if pos.Y >= content.Max.Y {
		return terminalPoint{Row: viewStart + rows - 1, Col: col}, true
	}

	displayTop, displayY, displayCount := s.displayRows(cellH)
	row := (pos.Y - content.Min.Y - displayY) / cellH
	if row < 0 {
		row = 0
	}
	if displayCount <= 0 {
		displayCount = rows
	}
	if row >= displayCount {
		row = displayCount - 1
	}
	return terminalPoint{Row: displayTop + row, Col: col}, true
}

func (s *terminalSession) beginSelection(pos image.Point, content image.Rectangle, cellW, cellH int, id pointer.ID) bool {
	pt, ok := s.terminalPointFromPosition(pos, content, cellW, cellH)
	if !ok {
		return false
	}
	s.viewMu.Lock()
	s.selectionSelecting = true
	s.selectionMoved = false
	s.selectionActive = false
	s.selectionStart = pt
	s.selectionEnd = pt
	s.selectionPointer = id
	s.selectionLastPos = pos
	s.autoScrollDir = 0
	s.autoScrollStep = 0
	s.autoScrollAt = time.Time{}
	s.autoScrollStartedAt = time.Time{}
	s.viewMu.Unlock()
	return true
}

func (s *terminalSession) registerPrimaryPress(now time.Time, pos image.Point) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if s.lastPrimaryPressed {
		dt := now.Sub(s.lastPrimaryPressAt)
		if dt < 0 {
			dt = -dt
		}
		dx := pos.X - s.lastPrimaryPressPos.X
		if dx < 0 {
			dx = -dx
		}
		dy := pos.Y - s.lastPrimaryPressPos.Y
		if dy < 0 {
			dy = -dy
		}
		if dt <= terminalDoubleClickDur && dx <= terminalDoubleClickDist && dy <= terminalDoubleClickDist {
			s.lastPrimaryPressed = false
			return true
		}
	}
	s.lastPrimaryPressed = true
	s.lastPrimaryPressAt = now
	s.lastPrimaryPressPos = pos
	return false
}

func (s *terminalSession) selectWordAtPosition(pos image.Point, content image.Rectangle, cellW, cellH int) bool {
	pt, ok := s.terminalPointFromPosition(pos, content, cellW, cellH)
	if !ok {
		return false
	}
	return s.selectWordAtPoint(pt)
}

func (s *terminalSession) selectWordAtPoint(pt terminalPoint) bool {
	if s == nil || s.term == nil {
		return false
	}
	s.parserMu.Lock()
	rows, cols := s.term.Rows(), s.term.Cols()
	scrollback := s.term.ScrollbackLen()
	alternate := s.term.IsAlternateScreen()
	maxRow := rows - 1
	if !alternate {
		maxRow = scrollback + rows - 1
	}
	if rows <= 0 || cols <= 0 || pt.Row < 0 || pt.Row > maxRow || pt.Col < 0 || pt.Col >= cols {
		s.parserMu.Unlock()
		return false
	}
	lineText, byteCols := s.virtualLineTextWithColumnMap(pt.Row, cols, scrollback, alternate)
	s.parserMu.Unlock()
	if lineText == "" || len(byteCols) == 0 {
		return false
	}

	targetStart, targetEnd := -1, -1
	for _, loc := range terminalWordSelectRE.FindAllStringIndex(lineText, -1) {
		if len(loc) != 2 || loc[0] < 0 || loc[1] <= loc[0] || loc[1] > len(byteCols) {
			continue
		}
		startCol := byteCols[loc[0]]
		endCol := byteCols[loc[1]-1]
		if terminalWordSpanContainsCol(startCol, endCol, pt.Col) ||
			(pt.Col > 0 && terminalWordSpanContainsCol(startCol, endCol, pt.Col-1)) {
			targetStart = startCol
			targetEnd = endCol
			break
		}
	}
	if targetStart < 0 || targetEnd < targetStart {
		return false
	}

	s.viewMu.Lock()
	s.selectionActive = true
	s.selectionSelecting = false
	s.selectionMoved = false
	s.selectionStart = terminalPoint{Row: pt.Row, Col: targetStart}
	s.selectionEnd = terminalPoint{Row: pt.Row, Col: targetEnd}
	s.selectionPointer = 0
	s.autoScrollDir = 0
	s.autoScrollStep = 0
	s.autoScrollAt = time.Time{}
	s.autoScrollStartedAt = time.Time{}
	s.viewMu.Unlock()
	return true
}

func (s *terminalSession) virtualLineTextWithColumnMap(row, cols, scrollback int, alternate bool) (string, []int) {
	if s == nil || cols <= 0 {
		return "", nil
	}
	var b strings.Builder
	byteCols := make([]int, 0, cols)
	lastNonSpaceByte := -1
	for col := 0; col < cols; col++ {
		r := ' '
		if cell, ok := s.virtualCell(row, col, scrollback, alternate); ok {
			tc := terminalCellFromHeadless(cell)
			if tc.Rune != 0 {
				r = tc.Rune
			}
		}
		start := b.Len()
		b.WriteRune(r)
		for i := start; i < b.Len(); i++ {
			byteCols = append(byteCols, col)
		}
		if !unicode.IsSpace(r) {
			lastNonSpaceByte = b.Len()
		}
	}
	if lastNonSpaceByte <= 0 {
		return "", nil
	}
	return b.String()[:lastNonSpaceByte], byteCols[:lastNonSpaceByte]
}

func terminalWordSpanContainsCol(start, end, col int) bool {
	return col >= start && col <= end
}

func (s *terminalSession) updateSelection(pos image.Point, content image.Rectangle, cellW, cellH int, now time.Time) bool {
	pt, ok := s.terminalPointFromPosition(pos, content, cellW, cellH)
	if !ok {
		return false
	}
	s.viewMu.Lock()
	changed := s.selectionEnd != pt || !s.selectionActive
	s.selectionEnd = pt
	s.selectionMoved = true
	s.selectionActive = true
	s.selectionLastPos = pos
	if s.updateSelectionAutoScrollLocked(pos, content, now) {
		changed = true
	}
	s.viewMu.Unlock()
	return changed
}

func (s *terminalSession) updateSelectionAutoScrollLocked(pos image.Point, content image.Rectangle, now time.Time) bool {
	dir, step := terminalSelectionAutoScrollParams(pos, content)
	prevDir := s.autoScrollDir
	prevStep := s.autoScrollStep
	prevAt := s.autoScrollAt
	s.autoScrollDir = dir
	s.autoScrollStep = step
	if dir == 0 || step <= 0 {
		s.autoScrollAt = time.Time{}
		s.autoScrollStartedAt = time.Time{}
		return prevDir != 0 || prevStep != 0 || !prevAt.IsZero()
	}
	if prevDir != dir || s.autoScrollStartedAt.IsZero() {
		s.autoScrollStartedAt = now
	}
	if prevDir != dir || prevStep != step {
		s.autoScrollAt = now
		return true
	}
	if s.autoScrollAt.IsZero() {
		s.autoScrollAt = now.Add(terminalSelectAutoScrollTick)
		return true
	}
	return false
}

func terminalSelectionAutoScrollParams(pos image.Point, content image.Rectangle) (dir, step int) {
	if content.Dx() <= 0 || content.Dy() <= 0 {
		return 0, 0
	}
	dist := 0
	switch {
	case pos.Y < content.Min.Y:
		dir = 1
		dist = content.Min.Y - pos.Y
	case pos.Y >= content.Max.Y:
		dir = -1
		dist = pos.Y - content.Max.Y + 1
	default:
		return 0, 0
	}
	return dir, terminalSelectionAutoScrollStep(dist)
}

func terminalSelectionAutoScrollStep(dist int) int {
	if dist > terminalSelectAutoScrollMidPx {
		return 6
	}
	if dist > terminalSelectAutoScrollNearPx {
		return 3
	}
	return 1
}

func terminalSelectionAutoScrollStepForElapsed(base int, elapsed time.Duration) int {
	if base <= 0 {
		return 0
	}
	step := base
	if elapsed >= terminalSelectAutoScrollAccel2 {
		step += 3
	} else if elapsed >= terminalSelectAutoScrollAccel1 {
		step++
	}
	if step > 9 {
		return 9
	}
	return step
}

func (s *terminalSession) endSelection(id pointer.ID) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if !s.selectionSelecting || s.selectionPointer != id {
		return false
	}
	changed := false
	if !s.selectionMoved {
		changed = s.selectionActive
		s.selectionActive = false
	}
	s.selectionSelecting = false
	s.selectionMoved = false
	s.autoScrollDir = 0
	s.autoScrollStep = 0
	s.autoScrollAt = time.Time{}
	s.autoScrollStartedAt = time.Time{}
	return changed
}

func (s *terminalSession) selectionAutoScrollNext() (time.Time, bool) {
	if s == nil {
		return time.Time{}, false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if !s.selectionSelecting || s.autoScrollDir == 0 || s.autoScrollStep <= 0 || s.autoScrollAt.IsZero() {
		return time.Time{}, false
	}
	return s.autoScrollAt, true
}

func (s *terminalSession) runSelectionAutoScroll(now time.Time, content image.Rectangle, cellW, cellH int) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	if !s.selectionSelecting || s.autoScrollDir == 0 || s.autoScrollStep <= 0 || s.autoScrollAt.IsZero() || now.Before(s.autoScrollAt) {
		s.viewMu.Unlock()
		return false
	}
	dir := s.autoScrollDir
	step := terminalSelectionAutoScrollStepForElapsed(s.autoScrollStep, now.Sub(s.autoScrollStartedAt))
	lastPos := s.selectionLastPos
	s.autoScrollAt = now.Add(terminalSelectAutoScrollTick)
	s.viewMu.Unlock()

	if !s.scrollByLines(dir * step) {
		s.viewMu.Lock()
		s.autoScrollDir = 0
		s.autoScrollStep = 0
		s.autoScrollAt = time.Time{}
		s.autoScrollStartedAt = time.Time{}
		s.viewMu.Unlock()
		return false
	}
	return s.updateSelection(lastPos, content, cellW, cellH, now)
}

func (s *terminalSession) selectAll() bool {
	if s == nil {
		return false
	}
	s.State.Mu.RLock()
	rows := s.State.Rows
	cols := s.State.Cols
	scrollback := s.State.Scrollback
	alternate := s.State.Alternate
	s.State.Mu.RUnlock()
	if rows <= 0 || cols <= 0 {
		return false
	}
	startRow := 0
	endRow := scrollback + rows - 1
	if alternate {
		endRow = rows - 1
	}
	s.viewMu.Lock()
	prevActive := s.selectionActive
	prevStart := s.selectionStart
	prevEnd := s.selectionEnd
	s.selectionActive = true
	s.selectionSelecting = false
	s.selectionMoved = false
	s.selectionStart = terminalPoint{Row: startRow, Col: 0}
	s.selectionEnd = terminalPoint{Row: endRow, Col: cols - 1}
	s.autoScrollDir = 0
	s.autoScrollStep = 0
	s.autoScrollAt = time.Time{}
	s.autoScrollStartedAt = time.Time{}
	changed := !prevActive || prevStart != s.selectionStart || prevEnd != s.selectionEnd
	s.viewMu.Unlock()
	return changed
}

func (s *terminalSession) clearSelection() bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	changed := s.selectionActive || s.selectionSelecting
	s.selectionActive = false
	s.selectionSelecting = false
	s.selectionMoved = false
	s.autoScrollDir = 0
	s.autoScrollStep = 0
	s.autoScrollAt = time.Time{}
	s.autoScrollStartedAt = time.Time{}
	s.viewMu.Unlock()
	return changed
}

func (s *terminalSession) clearBuffer() bool {
	if s == nil || s.term == nil {
		return false
	}
	s.parserMu.Lock()
	cursorRow, cursorCol := s.term.CursorPos()
	cols := s.term.Cols()
	currentLine := make([]headlessterm.Cell, 0, cols)
	if cursorRow >= 0 && cursorRow < s.term.Rows() {
		for col := 0; col < cols; col++ {
			if cell := s.term.Cell(cursorRow, col); cell != nil {
				currentLine = append(currentLine, cell.Copy())
			} else {
				currentLine = append(currentLine, headlessterm.NewCell())
			}
		}
	}
	_, _ = s.term.Write([]byte("\x1b[H\x1b[2J"))
	s.term.ClearScrollback()
	s.term.ClearSelection()
	s.term.ClearPromptMarks()
	for col, cell := range currentLine {
		if dst := s.term.Cell(0, col); dst != nil {
			*dst = cell.Copy()
			dst.MarkDirty()
		}
	}
	if cursorCol < 0 {
		cursorCol = 0
	}
	if cols > 0 && cursorCol >= cols {
		cursorCol = cols - 1
	}
	_, _ = s.term.Write([]byte("\x1b[1;" + strconvItoa(cursorCol+1) + "H"))
	s.term.SetWrapped(0, false)
	s.parserMu.Unlock()

	s.clearSelection()
	s.viewMu.Lock()
	s.scrollOffset = 0
	s.scrollCarry = 0
	s.lastScrollbackLen = 0
	s.visualTop = 0
	s.visualReady = false
	s.visualAt = time.Time{}
	s.viewMu.Unlock()
	s.snapshot()
	s.invalidateNow()
	return true
}

func (s *terminalSession) selectionSnapshot() (start, end terminalPoint, active bool) {
	if s == nil {
		return terminalPoint{}, terminalPoint{}, false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if !s.selectionActive {
		return terminalPoint{}, terminalPoint{}, false
	}
	start, end = normalizeTerminalSelection(s.selectionStart, s.selectionEnd)
	return start, end, true
}

func normalizeTerminalSelection(a, b terminalPoint) (terminalPoint, terminalPoint) {
	if compareTerminalPoints(a, b) <= 0 {
		return a, b
	}
	return b, a
}

func compareTerminalPoints(a, b terminalPoint) int {
	if a.Row < b.Row {
		return -1
	}
	if a.Row > b.Row {
		return 1
	}
	if a.Col < b.Col {
		return -1
	}
	if a.Col > b.Col {
		return 1
	}
	return 0
}

func terminalPointSelected(row, col int, start, end terminalPoint) bool {
	if row < start.Row || row > end.Row {
		return false
	}
	if start.Row == end.Row {
		return col >= start.Col && col <= end.Col
	}
	if row == start.Row {
		return col >= start.Col
	}
	if row == end.Row {
		return col <= end.Col
	}
	return true
}

func (ui *UI) copyTerminalText(gtx layout.Context, fallbackAll bool) bool {
	if ui == nil || ui.terminal == nil {
		return false
	}
	return ui.terminal.copyText(gtx, fallbackAll)
}

func (ui *UI) pasteTerminalText(gtx layout.Context) bool {
	if ui == nil || ui.terminal == nil {
		return false
	}
	return ui.terminal.pasteText(gtx)
}

func (s *terminalSession) copyText(gtx layout.Context, fallbackAll bool) bool {
	if s == nil {
		return false
	}
	text := s.selectedText(fallbackAll)
	text = strings.TrimRightFunc(text, unicode.IsSpace)
	if text == "" {
		return false
	}
	gtx.Execute(clipboard.WriteCmd{
		Type: "application/text",
		Data: io.NopCloser(strings.NewReader(text)),
	})
	return true
}

func (s *terminalSession) pasteText(gtx layout.Context) bool {
	if s == nil {
		return false
	}
	gtx.Execute(key.FocusCmd{Tag: &s.keyTag})
	if text, err := readTerminalClipboardText(); err == nil {
		return s.writePastedText(text)
	}
	s.beginPasteRead(gtx.Now)
	gtx.Execute(clipboard.ReadCmd{Tag: &s.pasteTag})
	return true
}

func (s *terminalSession) pastePrimarySelectionOrClipboard(gtx layout.Context) bool {
	if s == nil {
		return false
	}
	if s.hasActiveSelection() {
		data := terminalPasteBytes(s.selectedText(false), s.bracketedPasteMode())
		if len(data) > 0 {
			gtx.Execute(key.FocusCmd{Tag: &s.keyTag})
			// The terminal selection acts as a primary-selection buffer. Keep it
			// independent of the system clipboard and visible after it is pasted.
			s.write(data)
			return true
		}
	}
	return s.pasteText(gtx)
}

func (s *terminalSession) handleClipboardEvents(gtx layout.Context) bool {
	if s == nil || !s.pasteReadPending(gtx.Now) {
		return false
	}
	handled := false
	for {
		ev, ok := gtx.Event(transfer.TargetFilter{
			Target: &s.pasteTag,
			Type:   "application/text",
		})
		if !ok {
			break
		}
		s.setPastePending(false)
		de, ok := ev.(transfer.DataEvent)
		if !ok {
			continue
		}
		data := de.Open()
		if data == nil {
			continue
		}
		content, err := io.ReadAll(data)
		_ = data.Close()
		if err != nil {
			continue
		}
		if s.writePastedText(string(content)) {
			handled = true
		}
	}
	return handled
}

func (ui *UI) handleTerminalClipboardEvents(gtx layout.Context) {
	if ui == nil || ui.terminal == nil {
		return
	}
	if ui.terminal.handleClipboardEvents(gtx) {
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) registerTerminalClipboardTarget(gtx layout.Context) {
	if ui == nil || ui.terminal == nil {
		return
	}
	pending, deadline := ui.terminal.pasteReadState(gtx.Now)
	if !pending {
		return
	}
	registerPointerTransparentEventTarget(gtx, &ui.terminal.pasteTag)
	if !deadline.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: deadline})
	}
}

func (s *terminalSession) beginPasteRead(now time.Time) {
	if s == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	s.viewMu.Lock()
	s.pastePending = true
	s.pastePendingAt = now
	s.viewMu.Unlock()
}

func (s *terminalSession) setPastePending(pending bool) {
	if s == nil {
		return
	}
	s.viewMu.Lock()
	s.pastePending = pending
	if !pending {
		s.pastePendingAt = time.Time{}
	}
	s.viewMu.Unlock()
}

func (s *terminalSession) pasteReadPending(now time.Time) bool {
	pending, _ := s.pasteReadState(now)
	return pending
}

func (s *terminalSession) pasteReadState(now time.Time) (bool, time.Time) {
	if s == nil {
		return false, time.Time{}
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	deadline := time.Time{}
	if !s.pastePendingAt.IsZero() {
		deadline = s.pastePendingAt.Add(terminalPasteReadTimeout)
	}
	if s.pastePending && !deadline.IsZero() && !now.IsZero() && !now.Before(deadline) {
		s.pastePending = false
		s.pastePendingAt = time.Time{}
		deadline = time.Time{}
	}
	return s.pastePending, deadline
}

func (s *terminalSession) writePastedText(text string) bool {
	data := terminalPasteBytes(text, s.bracketedPasteMode())
	if len(data) == 0 {
		return false
	}
	s.clearSelection()
	s.write(data)
	return true
}

func terminalPasteBytes(text string, bracketed bool) []byte {
	text = strings.ReplaceAll(text, "\x00", "")
	if text == "" {
		return nil
	}
	if bracketed {
		text = normalizeTerminalPasteLineEndings(text, "\n")
		return []byte("\x1b[200~" + text + "\x1b[201~")
	}
	text = normalizeTerminalPasteLineEndings(text, "\r")
	return []byte(text)
}

func normalizeTerminalPasteLineEndings(text, newline string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if newline != "\n" {
		text = strings.ReplaceAll(text, "\n", newline)
	}
	return text
}

func (s *terminalSession) hasActiveSelection() bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	return s.selectionActive
}

func (s *terminalSession) selectedText(fallbackAll bool) string {
	if s == nil || s.term == nil {
		return ""
	}
	start, end, active := s.selectionSnapshot()
	s.parserMu.Lock()
	defer s.parserMu.Unlock()
	rows, cols := s.term.Rows(), s.term.Cols()
	scrollback := s.term.ScrollbackLen()
	alternate := s.term.IsAlternateScreen()
	if rows <= 0 || cols <= 0 {
		return ""
	}
	if !active {
		if !fallbackAll {
			return ""
		}
		start = terminalPoint{Row: 0, Col: 0}
		end = terminalPoint{Row: rows - 1, Col: cols - 1}
		if !alternate {
			end.Row = scrollback + rows - 1
		}
	}
	maxRow := rows - 1
	if !alternate {
		maxRow = scrollback + rows - 1
	}
	if start.Row < 0 {
		start.Row = 0
	}
	if end.Row > maxRow {
		end.Row = maxRow
	}
	if start.Row > end.Row {
		return ""
	}

	var b strings.Builder
	for row := start.Row; row <= end.Row; row++ {
		if row > start.Row {
			b.WriteByte('\n')
		}
		fromCol := 0
		toCol := cols - 1
		if row == start.Row {
			fromCol = start.Col
		}
		if row == end.Row {
			toCol = end.Col
		}
		if fromCol < 0 {
			fromCol = 0
		}
		if toCol >= cols {
			toCol = cols - 1
		}
		line := s.virtualLineText(row, fromCol, toCol, scrollback, alternate)
		b.WriteString(line)
	}
	return b.String()
}

func (s *terminalSession) virtualLineText(row, fromCol, toCol, scrollback int, alternate bool) string {
	if s == nil || s.term == nil || fromCol > toCol {
		return ""
	}
	var b strings.Builder
	for col := fromCol; col <= toCol; col++ {
		cell, ok := s.virtualCell(row, col, scrollback, alternate)
		if !ok {
			continue
		}
		tc := terminalCellFromHeadless(cell)
		if tc.Rune == 0 {
			continue
		}
		b.WriteRune(tc.Rune)
	}
	return strings.TrimRight(b.String(), " ")
}

func (s *terminalSession) virtualCell(row, col, scrollback int, alternate bool) (headlessterm.Cell, bool) {
	if s == nil || s.term == nil || col < 0 {
		return headlessterm.Cell{}, false
	}
	if !alternate && row < scrollback {
		line := s.term.ScrollbackLine(row)
		if col >= len(line) {
			return headlessterm.Cell{}, false
		}
		return line[col], true
	}
	termRow := row
	if !alternate {
		termRow = row - scrollback
	}
	cell := s.term.Cell(termRow, col)
	if cell == nil {
		return headlessterm.Cell{}, false
	}
	return *cell, true
}

func (s *terminalSession) setError(msg string) {
	if s == nil {
		return
	}
	s.State.Mu.Lock()
	s.State.Err = msg
	s.State.Mu.Unlock()
	s.invalidateNow()
}

func (s *terminalSession) snapshot() {
	if s == nil || s.term == nil {
		return
	}
	s.parserMu.Lock()
	rows, cols := s.term.Rows(), s.term.Cols()
	scrollbackLen := s.term.ScrollbackLen()
	alternate := s.term.IsAlternateScreen()

	s.viewMu.Lock()
	if alternate {
		s.scrollOffset = 0
	} else if scrollbackLen < s.lastScrollbackLen {
		if s.scrollOffset > scrollbackLen {
			s.scrollOffset = scrollbackLen
		}
	} else if s.scrollOffset > 0 && scrollbackLen > s.lastScrollbackLen {
		s.scrollOffset += scrollbackLen - s.lastScrollbackLen
		if s.scrollOffset > scrollbackLen {
			s.scrollOffset = scrollbackLen
		}
	}
	if s.scrollOffset > scrollbackLen {
		s.scrollOffset = scrollbackLen
	}
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
	s.lastScrollbackLen = scrollbackLen
	scrollOffset := s.scrollOffset
	s.viewMu.Unlock()

	viewStart := scrollbackLen - scrollOffset
	if alternate {
		viewStart = 0
	}
	if viewStart < 0 {
		viewStart = 0
	}
	grid := make([][]TerminalCell, rows)
	for row := 0; row < rows; row++ {
		grid[row] = make([]TerminalCell, cols)
		absoluteRow := viewStart + row
		for col := 0; col < cols; col++ {
			var cell *headlessterm.Cell
			if !alternate && absoluteRow < scrollbackLen {
				if line := s.term.ScrollbackLine(absoluteRow); col < len(line) {
					cell = &line[col]
				}
			} else {
				termRow := row
				if !alternate {
					termRow = absoluteRow - scrollbackLen
				}
				cell = s.term.Cell(termRow, col)
			}
			if cell != nil {
				grid[row][col] = terminalCellFromHeadless(*cell)
			}
		}
	}
	cursorRow, cursorCol := s.term.CursorPos()
	cursorVisible := s.term.CursorVisible()
	s.parserMu.Unlock()

	s.State.Mu.Lock()
	active := s.State.Active
	errText := s.State.Err
	s.State.Grid = grid
	s.State.Rows = rows
	s.State.Cols = cols
	s.State.Cursor.X = cursorCol
	s.State.Cursor.Y = cursorRow + scrollOffset
	s.State.CursorVisible = cursorVisible && scrollOffset == 0
	s.State.Active = active
	s.State.Err = errText
	s.State.ScrollOffset = scrollOffset
	s.State.Scrollback = scrollbackLen
	s.State.ViewStart = viewStart
	s.State.Alternate = alternate
	s.State.Mu.Unlock()
}

func terminalCellFromHeadless(cell headlessterm.Cell) TerminalCell {
	fg := terminalColor(cell.Fg, true)
	bg := terminalColor(cell.Bg, false)
	if cell.HasFlag(headlessterm.CellFlagReverse) {
		fg, bg = bg, fg
	}
	dim := cell.HasFlag(headlessterm.CellFlagDim)
	if dim {
		fg = mixNRGBA(fg, bg, 0.35)
	}
	if cell.HasFlag(headlessterm.CellFlagHidden) {
		fg = bg
	}
	r := cell.Char
	if r == 0 {
		r = ' '
	}
	if cell.HasFlag(headlessterm.CellFlagWideCharSpacer) {
		r = 0
	}
	return TerminalCell{
		Rune: r,
		FG:   fg,
		BG:   bg,
		Bold: cell.HasFlag(headlessterm.CellFlagBold),
		Dim:  dim,
	}
}

func terminalColor(c color.Color, fg bool) color.NRGBA {
	rgba := headlessterm.ResolveDefaultColor(c, fg)
	return color.NRGBA{R: rgba.R, G: rgba.G, B: rgba.B, A: rgba.A}
}

type terminalShellOption struct {
	Key   string
	Label string
}

func terminalDetectedShellOptions() []terminalShellOption {
	return terminalShellOptionsFor(runtime.GOOS, terminalLookPath, terminalDetectedWSLDistros())
}

func terminalShellOptionsFor(goos string, lookPath func(string) (string, error), wslDistros []string) []terminalShellOption {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	options := []terminalShellOption{{Key: "auto", Label: "Auto"}}
	add := func(key, label string) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		if normalized, ok := fm.NormalizeKnownViewerShell(key); ok {
			key = normalized
		}
		for _, opt := range options {
			if strings.EqualFold(opt.Key, key) {
				return
			}
		}
		options = append(options, terminalShellOption{Key: key, Label: label})
	}

	if goos == "windows" {
		if terminalFirstLookPath(lookPath, "pwsh.exe", "pwsh") != "" {
			add("pwsh", "PS 7")
		}
		if terminalFirstLookPath(lookPath, "powershell.exe", "powershell") != "" {
			add("powershell", "Win PS")
		}
		if terminalFirstLookPath(lookPath, "wsl.exe", "wsl") != "" {
			for _, distro := range wslDistros {
				distro = strings.TrimSpace(distro)
				if distro != "" {
					add("wsl:"+distro, terminalWSLDistroLabel(distro))
				}
			}
		}
		if strings.TrimSpace(terminalGetenv("COMSPEC")) != "" || terminalFirstLookPath(lookPath, "cmd.exe", "cmd") != "" {
			add("cmd", "Cmd")
		}
		if terminalFirstLookPath(lookPath, "sh.exe", "sh") != "" {
			add("sh", "sh")
		}
		return options
	}

	add("sh", "sh")
	if terminalFirstLookPath(lookPath, "pwsh") != "" {
		add("pwsh", "PS 7")
	}
	return options
}

func terminalWSLDistroLabel(distro string) string {
	distro = strings.TrimSpace(distro)
	if distro == "" {
		return "WSL"
	}
	return "WSL " + distro
}

func terminalDetectedWSLDistros() []string {
	if runtime.GOOS != "windows" || terminalFirstLookPath(terminalLookPath, "wsl.exe", "wsl") == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "wsl.exe", "--list", "--quiet")
	configureViewerCommandProcess(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseTerminalWSLDistros(out)
}

func parseTerminalWSLDistros(data []byte) []string {
	text := decodeTerminalWSLOutput(data)
	lines := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\r' || r == '\n'
	})
	out := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.Trim(line, "\ufeff\x00"))
		line = strings.ReplaceAll(line, "\x00", "")
		if line == "" {
			continue
		}
		key := strings.ToLower(line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, line)
	}
	return out
}

func decodeTerminalWSLOutput(data []byte) string {
	if len(data) < 2 || !terminalLooksUTF16(data) {
		return string(data)
	}
	littleEndian := true
	if data[0] == 0xfe && data[1] == 0xff {
		littleEndian = false
		data = data[2:]
	} else if data[0] == 0xff && data[1] == 0xfe {
		data = data[2:]
	}
	if len(data)%2 == 1 {
		data = data[:len(data)-1]
	}
	u16 := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		if littleEndian {
			u16 = append(u16, uint16(data[i])|uint16(data[i+1])<<8)
		} else {
			u16 = append(u16, uint16(data[i])<<8|uint16(data[i+1]))
		}
	}
	return string(utf16.Decode(u16))
}

func terminalLooksUTF16(data []byte) bool {
	if len(data) >= 2 {
		if (data[0] == 0xff && data[1] == 0xfe) || (data[0] == 0xfe && data[1] == 0xff) {
			return true
		}
	}
	nulCount := 0
	for _, b := range data {
		if b == 0 {
			nulCount++
		}
	}
	return nulCount > 0 && nulCount*3 >= len(data)
}

func terminalCommandForShell(raw, cwd string) (string, []string) {
	return terminalCommandForShellOnGOOS(runtime.GOOS, raw, cwd)
}

func terminalCommandForShellOnGOOS(goos, raw, cwd string) (string, []string) {
	shell := fm.NormalizeViewerShell(raw)
	if shell == "auto" {
		return defaultTerminalCommandForGOOS(goos, cwd)
	}
	switch {
	case shell == "pwsh":
		return terminalPowerShellCommand(goos, true, false)
	case shell == "powershell":
		return terminalPowerShellCommand(goos, false, false)
	case shell == "cmd":
		if goos == "windows" {
			return terminalCmdCommand()
		}
		return defaultTerminalCommandForGOOS(goos, cwd)
	case shell == "sh":
		return terminalShCommand(goos)
	case fm.ViewerShellIsWSL(shell):
		if goos == "windows" {
			return terminalWSLCommand(shell, cwd)
		}
		return defaultTerminalCommandForGOOS(goos, cwd)
	default:
		return defaultTerminalCommandForGOOS(goos, cwd)
	}
}

func defaultTerminalCommandForGOOS(goos, cwd string) (string, []string) {
	if goos == "windows" {
		if terminalFirstLookPath(terminalLookPath, "pwsh.exe", "pwsh") != "" {
			return terminalPowerShellCommand(goos, true, false)
		}
		if terminalFirstLookPath(terminalLookPath, "powershell.exe", "powershell") != "" {
			return terminalPowerShellCommand(goos, false, false)
		}
		if terminalFirstLookPath(terminalLookPath, "wsl.exe", "wsl") != "" {
			return terminalWSLCommand("wsl", cwd)
		}
		return terminalCmdCommand()
	}
	if shell := strings.TrimSpace(terminalGetenv("SHELL")); shell != "" {
		return shell, nil
	}
	return "/bin/sh", nil
}

func terminalPowerShellCommand(goos string, modern, nonInteractive bool) (string, []string) {
	var program string
	if modern {
		if goos == "windows" {
			program = terminalFirstLookPath(terminalLookPath, "pwsh.exe", "pwsh")
			if program == "" {
				program = "pwsh.exe"
			}
		} else {
			program = terminalFirstLookPath(terminalLookPath, "pwsh")
			if program == "" {
				program = "pwsh"
			}
		}
	} else if goos == "windows" {
		program = terminalFirstLookPath(terminalLookPath, "powershell.exe", "powershell")
		if program == "" {
			program = "powershell.exe"
		}
	} else {
		program = terminalFirstLookPath(terminalLookPath, "pwsh")
		if program == "" {
			program = "pwsh"
		}
	}
	args := []string{"-NoLogo"}
	if nonInteractive {
		args = append(args, "-NoProfile", "-NonInteractive", "-Command")
	} else {
		args = append(args, "-NoExit", "-Command", terminalPowerShellPromptHook)
	}
	return program, args
}

const terminalPowerShellPromptHook = `$global:__hexoneOriginalPrompt = $function:prompt; function global:prompt { try { $esc = [char]27; $uri = [Uri]::new((Get-Location).ProviderPath).AbsoluteUri; [Console]::Write("$esc]7;$uri$([char]7)") } catch {}; if ($global:__hexoneOriginalPrompt) { & $global:__hexoneOriginalPrompt } else { "PS $PWD> " } }`

func terminalCmdCommand() (string, []string) {
	if shell := strings.TrimSpace(terminalGetenv("COMSPEC")); shell != "" {
		return shell, terminalCmdPromptArgs()
	}
	if program := terminalFirstLookPath(terminalLookPath, "cmd.exe", "cmd"); program != "" {
		return program, terminalCmdPromptArgs()
	}
	return "cmd.exe", terminalCmdPromptArgs()
}

func terminalCmdPromptArgs() []string {
	return []string{"/K", `prompt $E]7;file://localhost/$P$E\$P$G`}
}

func terminalShCommand(goos string) (string, []string) {
	if goos == "windows" {
		if program := terminalFirstLookPath(terminalLookPath, "sh.exe", "sh"); program != "" {
			return program, nil
		}
		return "sh.exe", nil
	}
	if program := terminalFirstLookPath(terminalLookPath, "sh"); program != "" {
		return program, nil
	}
	return "/bin/sh", nil
}

func terminalWSLCommand(shell, cwd string) (string, []string) {
	program := terminalFirstLookPath(terminalLookPath, "wsl.exe", "wsl")
	if program == "" {
		program = "wsl.exe"
	}
	args := []string{}
	if distro := fm.ViewerShellWSLDistro(shell); distro != "" {
		args = append(args, "--distribution", distro)
	}
	if strings.TrimSpace(cwd) != "" {
		args = append(args, "--cd", cwd)
	}
	return program, args
}

func terminalFirstLookPath(lookPath func(string) (string, error), names ...string) string {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		if path, err := lookPath(name); err == nil && strings.TrimSpace(path) != "" {
			return path
		}
	}
	return ""
}

func terminalEnv(base []string, rows, cols int) []string {
	env := append([]string(nil), base...)
	env = setEnv(env, "TERM", "xterm-256color")
	env = setEnv(env, "COLORTERM", "truecolor")
	env = setEnv(env, "TERM_PROGRAM", "hexone")
	env = setEnv(env, "LINES", strconvItoa(rows))
	env = setEnv(env, "COLUMNS", strconvItoa(cols))
	env = ensureTerminalUTF8Locale(env)
	return env
}

func ensureTerminalUTF8Locale(env []string) []string {
	if value, ok := terminalEnvValue(env, "LC_ALL"); ok && strings.TrimSpace(value) != "" {
		if terminalLocaleIsUTF8(value) {
			return env
		}
		env = setEnv(env, "LC_ALL", terminalUTF8Locale)
		env = setEnv(env, "LANG", terminalUTF8Locale)
		env = setEnv(env, "LC_CTYPE", terminalUTF8Locale)
		return env
	}
	if value, ok := terminalEnvValue(env, "LC_CTYPE"); ok && strings.TrimSpace(value) != "" {
		if terminalLocaleIsUTF8(value) {
			return env
		}
		env = setEnv(env, "LC_CTYPE", terminalUTF8Locale)
		if lang, ok := terminalEnvValue(env, "LANG"); !ok || strings.TrimSpace(lang) == "" || !terminalLocaleIsUTF8(lang) {
			env = setEnv(env, "LANG", terminalUTF8Locale)
		}
		return env
	}
	if value, ok := terminalEnvValue(env, "LANG"); ok && terminalLocaleIsUTF8(value) {
		return env
	}
	env = setEnv(env, "LANG", terminalUTF8Locale)
	env = setEnv(env, "LC_CTYPE", terminalUTF8Locale)
	return env
}

func terminalEnvValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return entry[len(prefix):], true
		}
	}
	return "", false
}

func terminalLocaleIsUTF8(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(lower, "utf-8") || strings.Contains(lower, "utf8")
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func strconvItoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	n := v
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func terminalStartError(err error) string {
	if terminalProcessUnsupported(err) {
		return "terminal PTY is not supported by the selected backend on this platform yet"
	}
	return "terminal start failed: " + err.Error()
}

func (ui *UI) terminalShell() string {
	if ui != nil && ui.fmCfg != nil {
		return ui.fmCfg.Viewer.Shell
	}
	return "auto"
}

func (ui *UI) applyTerminalShellRuntime() bool {
	if ui == nil || ui.fmCfg == nil {
		return false
	}
	next := fm.NormalizeViewerShell(ui.fmCfg.Viewer.Shell)
	if ui.runtimeTerminalShell == "" {
		ui.runtimeTerminalShell = next
		return false
	}
	if next == ui.runtimeTerminalShell {
		return false
	}
	ui.runtimeTerminalShell = next
	return ui.restartTerminalSessionsForShellChange()
}

func (ui *UI) restartTerminalSessionsForShellChange() bool {
	if ui == nil {
		return false
	}
	sessions := ui.terminalTabs.sessions
	active := ui.terminalTabs.active
	if len(sessions) == 0 {
		if ui.terminal == nil {
			return false
		}
		sessions = []*terminalSession{ui.terminal}
		active = 0
	}
	active = clampTabIndex(active, len(sessions))
	drawerActive := ui.terminal != nil && ui.terminal.active()

	type replacementState struct {
		rows       int
		cols       int
		dir        string
		wasRunning bool
	}
	states := make([]replacementState, len(sessions))
	seen := make(map[*terminalSession]struct{}, len(sessions))
	for i, old := range sessions {
		state := replacementState{
			rows: terminalConfiguredRows(ui.fmCfg),
			cols: terminalDefaultCols,
		}
		if old != nil {
			state.dir = old.restartDirectory()
			old.procMu.Lock()
			if old.rows > 0 {
				state.rows = old.rows
			}
			if old.cols > 0 {
				state.cols = old.cols
			}
			state.wasRunning = old.running
			old.procMu.Unlock()
		}
		states[i] = state
	}
	for _, old := range sessions {
		if old == nil {
			continue
		}
		if _, ok := seen[old]; ok {
			continue
		}
		seen[old] = struct{}{}
		old.Close()
	}

	replacements := make([]*terminalSession, len(sessions))
	for i, state := range states {
		next := newTerminalSession(ui.invalidate, state.rows)
		next.resize(state.rows, state.cols)
		next.pendingStartDir = state.dir
		replacements[i] = next
	}

	ui.terminalTabs.sessions = replacements
	ui.terminalTabs.active = active
	ui.terminalTabs.scroll = clampTabScrollAnchor(ui.terminalTabs.scroll, len(replacements))
	ui.terminal = replacements[active]
	ui.terminal.setActive(drawerActive)
	fallbackDir := ui.terminalStartDir()
	for i, state := range states {
		if state.wasRunning {
			replacements[i].start(fallbackDir, ui.terminalShell())
		}
	}
	if drawerActive {
		ui.terminal.focusKeyboard()
	}
	if ui.invalidate != nil {
		ui.invalidate()
	}
	return true
}

func (s *terminalSession) restartDirectory() string {
	if s == nil {
		return ""
	}
	if loc, ok := s.osc7Location(); ok && terminalOSC7HostIsLocal(loc.Host) {
		if dir := terminalOSC7LocalDir(loc.Dir); terminalDirExists(dir) {
			return dir
		}
	}
	if dir, ok := s.currentDir(); ok && terminalDirExists(dir) {
		return dir
	}
	return ""
}

func (s *terminalSession) pendingDirectory() string {
	if s == nil {
		return ""
	}
	s.procMu.Lock()
	dir := strings.TrimSpace(s.pendingStartDir)
	s.procMu.Unlock()
	return dir
}

func (ui *UI) terminalStartDir() string {
	if ui != nil {
		if pane := ui.activePane(); pane != nil && !pane.remoteConnected() {
			if dir := strings.TrimSpace(pane.dir); dir != "" {
				if info, err := os.Stat(dir); err == nil && info.IsDir() {
					return dir
				}
			}
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func terminalPaneLocalDir(pane *filePaneState) string {
	if pane == nil || pane.remoteConnected() || pane.archiveBrowsing() {
		return ""
	}
	dir := strings.TrimSpace(pane.dir)
	if dir == "" {
		return ""
	}
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return ""
}

func (ui *UI) terminalGoToPaneDir(idx int, now time.Time) bool {
	if ui == nil || ui.terminal == nil || idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	dir := terminalPaneLocalDir(pane)
	if dir == "" {
		if pane != nil {
			pane.setNotice("pane dir is not a local folder", now)
		}
		return false
	}
	ui.terminal.writeString(terminalChangeDirCommandForShell(dir, ui.terminalShell(), runtime.GOOS))
	ui.terminal.focusKeyboard()
	return true
}

func (ui *UI) setPaneDirToTerminalDir(idx int, now time.Time) bool {
	if ui == nil || ui.terminal == nil || idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return false
	}
	if loc, ok := ui.terminal.osc7Location(); ok {
		if !terminalOSC7HostIsLocal(loc.Host) {
			return ui.setPaneDirToTerminalRemoteDir(idx, loc, now)
		}
		if _, sshOK := ui.terminal.activeSSHTarget(); sshOK {
			pane.setNotice("terminal SSH dir unavailable; remote shell must emit OSC 7", now)
			return false
		}
		if ui.setPaneDirToLocalTerminalDir(idx, terminalOSC7LocalDir(loc.Dir), now) {
			return true
		}
		pane = ui.filePanes[idx]
		if pane == nil {
			return false
		}
	}
	if _, sshOK := ui.terminal.activeSSHTarget(); sshOK {
		pane.setNotice("terminal SSH dir unavailable; remote shell must emit OSC 7", now)
		return false
	}
	dir, ok := ui.terminal.currentDir()
	if !ok {
		pane.setNotice("terminal current dir unavailable", now)
		return false
	}
	return ui.setPaneDirToLocalTerminalDir(idx, dir, now)
}

func (ui *UI) setPaneDirToLocalTerminalDir(idx int, dir string, now time.Time) bool {
	if ui == nil || idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return false
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		pane.setNotice("terminal current dir is not a folder", now)
		return false
	}
	if pane.remoteConnected() {
		ui.disconnectPaneSSH(idx, now)
		pane = ui.filePanes[idx]
		if pane == nil {
			return false
		}
	}
	if ui.loadPaneDir(idx, dir) {
		pane.setNotice("pane set to terminal dir", now)
		return true
	}
	pane.setNotice("failed to set pane dir", now)
	return false
}

func (ui *UI) setPaneDirToTerminalRemoteDir(idx int, loc terminalOSC7Location, now time.Time) bool {
	if ui == nil || idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return false
	}
	setup, found, ambiguous := findSSHSetupForTerminalOSC7(ui.fmCfg, loc)
	if (!found || ambiguous) && ui.terminal != nil {
		if target, ok := ui.terminal.activeSSHTarget(); ok {
			if targetSetup, targetFound, targetAmbiguous := findSSHSetupForTerminalSSHTarget(ui.fmCfg, target); targetFound {
				setup = targetSetup
				found = true
				ambiguous = false
			} else if !found && targetAmbiguous {
				ambiguous = true
			}
		}
	}
	if ambiguous {
		pane.setNotice("multiple SSH setups match terminal host: "+terminalOSC7DisplayHost(loc), now)
		return false
	}
	if !found {
		pane.setNotice("missing SSH setup for terminal host: "+terminalOSC7DisplayHost(loc), now)
		return false
	}
	if pane.remoteConnected() && pane.remote != nil && sameSSHRemoteTarget(pane.remote.setup, setup) {
		if ui.loadPaneDir(idx, loc.Dir) {
			pane.setNotice("pane set to terminal dir", now)
			return true
		}
		return false
	}
	if err := ui.connectPaneSSH(idx, setup, loc.Dir, now); err != nil {
		pane.setNotice("ssh connect failed: "+err.Error(), now)
		return false
	}
	if pane = ui.filePanes[idx]; pane != nil {
		pane.setNotice("pane set to terminal dir", now)
	}
	return true
}

func terminalChangeDirCommand(dir string) string {
	return terminalChangeDirCommandForShell(dir, "", runtime.GOOS)
}

func terminalChangeDirCommandForShell(dir, shell, goos string) string {
	if goos == "windows" && fm.ViewerShellIsWSL(shell) {
		return "cd " + terminalPosixQuotePath(windowsPathToWSLPath(dir)) + "\r"
	}
	if goos == "windows" {
		return "cd " + terminalWindowsQuotePath(dir) + "\r"
	}
	return "cd " + terminalPosixQuotePath(dir) + "\r"
}

func terminalPosixQuotePath(dir string) string {
	if dir == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(dir, "'", "'\\''") + "'"
}

func terminalWindowsQuotePath(dir string) string {
	return `"` + strings.ReplaceAll(dir, `"`, `""`) + `"`
}

type terminalOSC7Location struct {
	User    string
	Host    string
	Port    int
	HasPort bool
	Dir     string
}

type terminalSSHTarget struct {
	User    string
	Host    string
	Port    int
	HasPort bool
}

func (s *terminalSession) osc7Location() (terminalOSC7Location, bool) {
	if s == nil || s.term == nil {
		return terminalOSC7Location{}, false
	}
	s.parserMu.Lock()
	raw := s.term.WorkingDirectory()
	s.parserMu.Unlock()
	return parseTerminalOSC7Location(raw)
}

func (s *terminalSession) activeSSHTarget() (terminalSSHTarget, bool) {
	if s == nil {
		return terminalSSHTarget{}, false
	}
	s.procMu.Lock()
	pid := 0
	if s.pty != nil && s.running {
		pid = s.pty.PID()
	}
	s.procMu.Unlock()
	if pid <= 0 {
		return terminalSSHTarget{}, false
	}
	return terminalProcessSSHTarget(pid)
}

func parseTerminalOSC7Location(raw string) (terminalOSC7Location, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(raw), "file://") {
		raw = strings.ReplaceAll(raw, `\`, "/")
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil || !strings.EqualFold(u.Scheme, "file") {
		return terminalOSC7Location{}, false
	}
	dir := strings.TrimSpace(u.Path)
	if dir == "" {
		return terminalOSC7Location{}, false
	}
	host := strings.TrimSpace(u.Hostname())
	port := 0
	hasPort := false
	if rawPort := strings.TrimSpace(u.Port()); rawPort != "" {
		parsed, ok := parseTerminalOSC7Port(rawPort)
		if !ok {
			return terminalOSC7Location{}, false
		}
		port = parsed
		hasPort = true
	}
	user := ""
	if u.User != nil {
		user = strings.TrimSpace(u.User.Username())
	}
	if terminalOSC7HostIsLocal(host) {
		dir = terminalOSC7LocalDir(dir)
	} else {
		dir = normalizeRemoteFavoriteDir(dir)
	}
	return terminalOSC7Location{
		User:    user,
		Host:    host,
		Port:    port,
		HasPort: hasPort,
		Dir:     dir,
	}, true
}

func parseTerminalOSC7Port(raw string) (int, bool) {
	return terminalParsePositiveInt(raw, 65535)
}

func terminalParsePositiveInt(raw string, max int) (int, bool) {
	if raw == "" {
		return 0, false
	}
	n := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
		if max > 0 && n > max {
			return 0, false
		}
	}
	return n, n > 0
}

func terminalOSC7LocalDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if runtime.GOOS == "windows" && len(dir) >= 3 && dir[0] == '/' && dir[2] == ':' {
		dir = dir[1:]
	}
	return dir
}

func terminalOSC7HostIsLocal(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	if host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	local, err := os.Hostname()
	if err != nil {
		return false
	}
	local = strings.ToLower(strings.TrimSpace(local))
	if local == "" {
		return false
	}
	if host == local {
		return true
	}
	hostShort := strings.SplitN(host, ".", 2)[0]
	localShort := strings.SplitN(local, ".", 2)[0]
	return hostShort != "" && localShort != "" && hostShort == localShort
}

func findSSHSetupForTerminalOSC7(cfg *fm.Config, loc terminalOSC7Location) (fm.SSHSetup, bool, bool) {
	if cfg == nil || strings.TrimSpace(loc.Host) == "" {
		return fm.SSHSetup{}, false, false
	}
	matches := make([]fm.SSHSetup, 0, 1)
	for _, raw := range cfg.SSH.Setups {
		host := strings.TrimSpace(raw.Host)
		if !strings.EqualFold(host, loc.Host) {
			continue
		}
		user := strings.TrimSpace(raw.User)
		if loc.User != "" && user != loc.User {
			continue
		}
		port := raw.Port
		if port <= 0 {
			port = 22
		}
		if loc.HasPort && port != loc.Port {
			continue
		}
		setup := raw
		setup.Host = host
		setup.User = user
		setup.Port = port
		matches = append(matches, setup)
	}
	if len(matches) == 1 {
		return matches[0], true, false
	}
	return fm.SSHSetup{}, false, len(matches) > 1
}

func findSSHSetupForTerminalSSHTarget(cfg *fm.Config, target terminalSSHTarget) (fm.SSHSetup, bool, bool) {
	if cfg == nil || strings.TrimSpace(target.Host) == "" {
		return fm.SSHSetup{}, false, false
	}
	targetPort := target.Port
	if targetPort <= 0 {
		targetPort = 22
	}
	matches := make([]fm.SSHSetup, 0, 1)
	for _, raw := range cfg.SSH.Setups {
		host := strings.TrimSpace(raw.Host)
		if !strings.EqualFold(host, target.Host) {
			continue
		}
		user := strings.TrimSpace(raw.User)
		if target.User != "" && user != target.User {
			continue
		}
		port := raw.Port
		if port <= 0 {
			port = 22
		}
		if port != targetPort {
			continue
		}
		setup := raw
		setup.Host = host
		setup.User = user
		setup.Port = port
		matches = append(matches, setup)
	}
	if len(matches) == 1 {
		return matches[0], true, false
	}
	return fm.SSHSetup{}, false, len(matches) > 1
}

func terminalOSC7DisplayHost(loc terminalOSC7Location) string {
	host := strings.TrimSpace(loc.Host)
	if loc.User != "" {
		host = loc.User + "@" + host
	}
	if loc.HasPort {
		host += ":" + strconvItoa(loc.Port)
	}
	if host == "" {
		return "?"
	}
	return host
}

func detectTerminalProcessSSHTarget(pid int) (terminalSSHTarget, bool) {
	if pid <= 0 || runtime.GOOS == "windows" {
		return terminalSSHTarget{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), terminalCwdProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "axww", "-o", "pid=", "-o", "ppid=", "-o", "command=").Output()
	if err != nil {
		return terminalSSHTarget{}, false
	}
	return terminalSSHTargetFromPS(pid, string(out))
}

type terminalProcessInfo struct {
	pid     int
	ppid    int
	command string
}

func terminalSSHTargetFromPS(rootPID int, psOutput string) (terminalSSHTarget, bool) {
	if rootPID <= 0 {
		return terminalSSHTarget{}, false
	}
	children := make(map[int][]terminalProcessInfo)
	for _, line := range strings.Split(psOutput, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 {
			continue
		}
		pid, ok := terminalParsePositiveInt(fields[0], 0)
		if !ok {
			continue
		}
		ppid, ok := terminalParsePositiveInt(fields[1], 0)
		if !ok {
			continue
		}
		children[ppid] = append(children[ppid], terminalProcessInfo{
			pid:     pid,
			ppid:    ppid,
			command: strings.Join(fields[2:], " "),
		})
	}
	stack := append([]terminalProcessInfo(nil), children[rootPID]...)
	var found terminalSSHTarget
	ok := false
	for len(stack) > 0 {
		proc := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if target, targetOK := parseTerminalSSHCommand(proc.command); targetOK {
			found = target
			ok = true
		}
		stack = append(stack, children[proc.pid]...)
	}
	return found, ok
}

func parseTerminalSSHCommand(command string) (terminalSSHTarget, bool) {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) < 2 || terminalCommandBase(fields[0]) != "ssh" {
		return terminalSSHTarget{}, false
	}
	target := terminalSSHTarget{Port: 22}
	targetText := ""
	for i := 1; i < len(fields); i++ {
		arg := fields[i]
		if arg == "" {
			continue
		}
		if arg == "--" {
			if i+1 < len(fields) {
				targetText = fields[i+1]
			}
			break
		}
		if arg == "-p" {
			if i+1 >= len(fields) {
				return terminalSSHTarget{}, false
			}
			port, ok := parseTerminalOSC7Port(fields[i+1])
			if !ok {
				return terminalSSHTarget{}, false
			}
			target.Port = port
			target.HasPort = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "-p") && len(arg) > 2 {
			port, ok := parseTerminalOSC7Port(arg[2:])
			if !ok {
				return terminalSSHTarget{}, false
			}
			target.Port = port
			target.HasPort = true
			continue
		}
		if arg == "-l" {
			if i+1 >= len(fields) {
				return terminalSSHTarget{}, false
			}
			target.User = strings.TrimSpace(fields[i+1])
			i++
			continue
		}
		if strings.HasPrefix(arg, "-l") && len(arg) > 2 {
			target.User = strings.TrimSpace(arg[2:])
			continue
		}
		if terminalSSHOptionConsumesNext(arg) {
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		targetText = arg
		break
	}
	if targetText == "" {
		return terminalSSHTarget{}, false
	}
	if strings.HasPrefix(targetText, "ssh://") {
		if u, err := url.Parse(targetText); err == nil && u != nil {
			if u.User != nil && target.User == "" {
				target.User = strings.TrimSpace(u.User.Username())
			}
			target.Host = strings.TrimSpace(u.Hostname())
			if rawPort := strings.TrimSpace(u.Port()); rawPort != "" {
				port, ok := parseTerminalOSC7Port(rawPort)
				if !ok {
					return terminalSSHTarget{}, false
				}
				target.Port = port
				target.HasPort = true
			}
			return target, target.Host != ""
		}
	}
	if at := strings.LastIndex(targetText, "@"); at >= 0 {
		if target.User == "" {
			target.User = strings.TrimSpace(targetText[:at])
		}
		targetText = targetText[at+1:]
	}
	target.Host = strings.Trim(strings.TrimSpace(targetText), "[]")
	return target, target.Host != ""
}

func terminalCommandBase(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	if idx := strings.LastIndexAny(command, `/\`); idx >= 0 {
		command = command[idx+1:]
	}
	return command
}

func terminalSSHOptionConsumesNext(arg string) bool {
	switch arg {
	case "-B", "-b", "-c", "-D", "-E", "-e", "-F", "-I", "-i", "-J", "-L", "-l", "-m", "-O", "-o", "-P", "-Q", "-R", "-S", "-W", "-w":
		return true
	default:
		return false
	}
}

func (s *terminalSession) currentDir() (string, bool) {
	if s == nil {
		return "", false
	}
	s.procMu.Lock()
	pid := 0
	if s.pty != nil && s.running {
		pid = s.pty.PID()
	}
	startDir := strings.TrimSpace(s.startDir)
	s.procMu.Unlock()

	if pid > 0 {
		if dir, ok := terminalProcessWorkingDir(pid); ok {
			return dir, true
		}
	}
	if startDir != "" {
		if info, err := os.Stat(startDir); err == nil && info.IsDir() {
			return startDir, true
		}
	}
	return "", false
}

func terminalProcessWorkingDir(pid int) (string, bool) {
	if pid <= 0 {
		return "", false
	}
	switch runtime.GOOS {
	case "linux", "android":
		dir, err := os.Readlink("/proc/" + strconvItoa(pid) + "/cwd")
		if err == nil && terminalDirExists(dir) {
			return dir, true
		}
	case "darwin":
		ctx, cancel := context.WithTimeout(context.Background(), terminalCwdProbeTimeout)
		defer cancel()
		out, err := exec.CommandContext(ctx, "lsof", "-a", "-p", strconvItoa(pid), "-d", "cwd", "-Fn").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "n") {
					dir := strings.TrimSpace(strings.TrimPrefix(line, "n"))
					if terminalDirExists(dir) {
						return dir, true
					}
				}
			}
		}
	}
	return "", false
}

func terminalDirExists(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

func terminalConfiguredRows(cfg *fm.Config) int {
	if cfg == nil {
		return terminalDefaultRows
	}
	return fm.NormalizeTerminalHeightRows(cfg.Terminal.HeightRows)
}

func (ui *UI) terminalAcceleratedKeysEnabled() bool {
	return ui == nil || ui.fmCfg == nil || ui.fmCfg.Terminal.AcceleratedKeys
}

func terminalPaneVerticalOverhead(gtx layout.Context) int {
	return gtx.Dp(unit.Dp(4)) +
		gtx.Dp(unit.Dp(tabStripHeightDp)) +
		gtx.Dp(unit.Dp(3)) +
		gtx.Dp(unit.Dp(4))
}

func terminalPaneHeightForRows(gtx layout.Context, cellH, rows int) int {
	if cellH <= 0 {
		cellH = 16
	}
	if rows < 1 {
		rows = 1
	}
	return rows*cellH + terminalPaneVerticalOverhead(gtx)
}

func terminalMaxPaneHeight(gtx layout.Context) int {
	maxY := gtx.Constraints.Max.Y
	if maxY <= 0 {
		return 0
	}
	h := maxY * terminalMaxPaneNum / terminalMaxPaneDen
	if h < 1 {
		h = 1
	}
	return h
}

func terminalMaxPaneRows(gtx layout.Context, cellH int) int {
	maxH := terminalMaxPaneHeight(gtx)
	if maxH <= 0 {
		return 0
	}
	if cellH <= 0 {
		cellH = 16
	}
	rows := (maxH - terminalPaneVerticalOverhead(gtx)) / cellH
	if rows < 1 {
		rows = 1
	}
	return rows
}

func terminalClampPaneRows(gtx layout.Context, cellH, rows int) int {
	if rows < 4 {
		rows = 4
	}
	maxRows := terminalMaxPaneRows(gtx, cellH)
	if maxRows < 1 {
		return 1
	}
	if rows > maxRows {
		rows = maxRows
	}
	return rows
}

func terminalPaneHeight(gtx layout.Context, cellH int, preferredRows ...int) int {
	rows := terminalPreferredRows
	if len(preferredRows) > 0 {
		rows = preferredRows[0]
	}
	if rows <= 0 {
		rows = terminalPreferredRows
	}
	rows = terminalClampPaneRows(gtx, cellH, rows)
	return terminalPaneHeightForRows(gtx, cellH, rows)
}

func (ui *UI) terminalPaneHeightWithTabs(gtx layout.Context, cellH int, preferredRows ...int) int {
	height := terminalPaneHeight(gtx, cellH, preferredRows...)
	height += ui.tabStripHeight(gtx) - gtx.Dp(unit.Dp(tabStripHeightDp))
	if maxHeight := terminalMaxPaneHeight(gtx); maxHeight > 0 && height > maxHeight {
		height = maxHeight
	}
	return height
}

func terminalRowsForPaneHeight(gtx layout.Context, cellH, height int) int {
	if cellH <= 0 || height <= 0 {
		return 0
	}
	rows := (height - terminalPaneVerticalOverhead(gtx)) / cellH
	if rows < 1 {
		return 0
	}
	return terminalClampPaneRows(gtx, cellH, rows)
}

func (ui *UI) terminalRowsForPaneHeightWithTabs(gtx layout.Context, cellH, height int) int {
	height -= ui.tabStripHeight(gtx) - gtx.Dp(unit.Dp(tabStripHeightDp))
	return terminalRowsForPaneHeight(gtx, cellH, height)
}

func terminalPaneCols(width int, cellW int) int {
	if cellW <= 0 {
		return 2
	}
	cols := width / cellW
	if cols < 2 {
		return 2
	}
	if cols > 2 {
		cols--
	}
	return cols
}

func (ui *UI) layoutTerminalPane(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.ensureTerminalSession()
	if st == nil || !st.active() {
		return layout.Dimensions{}
	}
	cellW, cellH := ui.terminalCellSize(th, gtx)
	if cellW <= 0 {
		cellW = 8
	}
	if cellH <= 0 {
		cellH = 16
	}
	h := ui.terminalPaneHeightWithTabs(gtx, cellH, terminalConfiguredRows(ui.fmCfg))
	if ui.terminalMaximized() {
		h = gtx.Constraints.Max.Y
	}
	if h <= 0 {
		st.setPaneMetrics(0, cellH)
		return layout.Dimensions{}
	}
	st.setPaneMetrics(h, cellH)
	return fixedHeight(gtx, h, func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		if size.X <= 0 || size.Y <= 0 {
			return layout.Dimensions{Size: size}
		}
		terminalFocused := st.keyboardFocused(gtx)
		paint.FillShape(gtx.Ops, terminalBG, clip.Rect(image.Rectangle{Max: size}).Op())
		paint.FillShape(gtx.Ops, terminalBorder, clip.Rect(image.Rect(0, 0, size.X, 1)).Op())
		if st.resizeHandleActive() {
			paint.FillShape(gtx.Ops, color.NRGBA{R: 118, G: 156, B: 240, A: 92}, clip.Rect(image.Rect(0, 0, size.X, 2)).Op())
		}

		defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
		key.InputHintOp{Tag: &st.keyTag, Hint: key.HintText}.Add(gtx.Ops)
		if st.wantFocus {
			terminalFocused = true
			st.wantFocus = false
			gtx.Execute(key.FocusCmd{Tag: &st.keyTag})
			gtx.Execute(key.SoftKeyboardCmd{Show: true})
		}

		padX := gtx.Dp(unit.Dp(6))
		padTop := gtx.Dp(unit.Dp(4))
		padBottom := gtx.Dp(unit.Dp(4))
		tabH := ui.tabStripHeight(gtx)
		tabRect := image.Rect(padX, padTop, size.X-padX, padTop+tabH)
		contentTop := tabRect.Max.Y + gtx.Dp(unit.Dp(3))
		content := image.Rect(padX, contentTop, size.X-padX, size.Y-padBottom)
		if content.Dx() <= 0 || content.Dy() <= 0 {
			return layout.Dimensions{Size: size}
		}

		cols := terminalPaneCols(content.Dx(), cellW)
		content, rows := terminalGridContentRect(content, cellH)
		st.resize(rows, cols)
		st.start(ui.terminalStartDir(), ui.terminalShell())
		if ui.fmCfg != nil {
			st.setFindPreviewRange(ui.fmCfg.Terminal.PreviewStart, ui.fmCfg.Terminal.PreviewEnd)
		}
		st.layoutScrollbar(gtx, content)

		if st.handlePointer(gtx, content, cellW, cellH) {
			gtx.Execute(op.InvalidateCmd{})
		}
		if st.handleInputWithAcceleratedKeys(gtx, ui.terminalAcceleratedKeysEnabled()) {
			gtx.Execute(op.InvalidateCmd{})
		}
		if st.pumpFindResults() {
			gtx.Execute(op.InvalidateCmd{})
		}
		if st.runSelectionAutoScroll(gtx.Now, content, cellW, cellH) {
			gtx.Execute(op.InvalidateCmd{})
		}
		if next, ok := st.selectionAutoScrollNext(); ok {
			gtx.Execute(op.InvalidateCmd{At: next})
		}
		st.layoutScrollbar(gtx, content)
		if st.prepareVisualScroll(gtx.Now, true) {
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(terminalSmoothTick)})
		}

		contentGtx := gtx
		contentGtx.Constraints = layout.Constraints{Max: content.Size()}
		off := op.Offset(content.Min).Push(gtx.Ops)
		ui.drawTerminalGrid(th, contentGtx, st, cellW, cellH, terminalFocused)
		off.Pop()
		ui.drawTerminalScrollbar(gtx, st)
		event.Op(gtx.Ops, &st.keyTag)
		pointerStack := clip.Rect(content).Push(gtx.Ops)
		event.Op(gtx.Ops, &st.pointerTag)
		pointerStack.Pop()
		st.applyTerminalCursor(gtx, content)
		if tabRect.Dx() > 0 && tabRect.Dy() > 0 {
			tabGtx := gtx
			tabGtx.Constraints = layout.Exact(tabRect.Size())
			off := op.Offset(tabRect.Min).Push(gtx.Ops)
			ui.layoutTerminalTabStrip(th, tabGtx)
			off.Pop()
			ui.drawTerminalTabRail(gtx, tabRect)
		}
		ui.layoutTerminalFind(th, gtx, st, contentTop)
		if !terminalFocused {
			if shade := filePaneInactiveShadeColor(ui.fmCfg, terminalBG); shade.A != 0 {
				paint.FillShape(gtx.Ops, shade, clip.Rect(image.Rectangle{Max: size}).Op())
			}
		}
		ui.layoutTerminalContextMenu(th, gtx, st)

		return layout.Dimensions{Size: size}
	})
}

func (ui *UI) drawTerminalTabRail(gtx layout.Context, tabRect image.Rectangle) {
	if ui == nil || tabRect.Dx() <= 0 {
		return
	}
	stroke := gtx.Dp(unit.Dp(1))
	if stroke < 1 {
		stroke = 1
	}
	y := tabRect.Max.Y
	w := tabRect.Dx()
	geometry := ui.terminalTabs.geometry
	railColor := filePanePathFrameColor(filePanePaletteFromConfig(ui.fmCfg))
	if !geometry.activeVisible {
		paint.FillShape(gtx.Ops, railColor, clip.Rect(image.Rect(tabRect.Min.X, y, tabRect.Max.X, y+stroke)).Op())
		return
	}
	activeMin := max(0, min(w, geometry.activeMinX))
	activeMax := max(activeMin, min(w, geometry.activeMaxX))
	legH := gtx.Dp(unit.Dp(4))
	if legH < stroke {
		legH = stroke
	}
	if activeMin > 0 {
		paint.FillShape(gtx.Ops, railColor, clip.Rect(image.Rect(tabRect.Min.X, y, tabRect.Min.X+activeMin, y+stroke)).Op())
		leftX := tabRect.Min.X + activeMin - stroke
		paint.FillShape(gtx.Ops, railColor, clip.Rect(image.Rect(leftX, y-legH, leftX+stroke, y+stroke)).Op())
	}
	if activeMax < w {
		paint.FillShape(gtx.Ops, railColor, clip.Rect(image.Rect(tabRect.Min.X+activeMax, y, tabRect.Max.X, y+stroke)).Op())
		rightX := tabRect.Min.X + activeMax
		paint.FillShape(gtx.Ops, railColor, clip.Rect(image.Rect(rightX, y-legH, rightX+stroke, y+stroke)).Op())
	}
}

func terminalGridContentRect(content image.Rectangle, cellH int) (image.Rectangle, int) {
	if cellH <= 0 || content.Dy() < cellH {
		return content, 1
	}
	rows := content.Dy() / cellH
	content.Min.Y = content.Max.Y - rows*cellH
	return content, rows
}

func (ui *UI) handleTerminalResizeRows(rows int, final bool) {
	if ui == nil {
		return
	}
	rows = fm.NormalizeTerminalHeightRows(rows)
	if ui.fmCfg != nil && ui.fmCfg.Terminal.HeightRows != rows {
		ui.fmCfg.Terminal.HeightRows = rows
	}
	if !final {
		return
	}
	if err := ui.saveFMConfigWithOptions("terminal-height", false); err != nil && ui.terminal != nil {
		ui.terminal.setError("save terminal height failed: " + err.Error())
	}
}

func (ui *UI) layoutTerminalResizeHandle(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if ui == nil || ui.terminal == nil || !ui.terminal.active() || ui.terminalMaximized() {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	st := ui.terminal
	rect, cellH, rows, ok := ui.terminalResizeHandleGeometry(th, gtx)
	if !ok {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	if st.handleResizePointer(gtx, rect, cellH, rows, ui.handleTerminalResizeRows) {
		gtx.Execute(op.InvalidateCmd{})
	}

	if st.resizeDraggingActive() {
		stack := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
		pointer.CursorNorthSouthResize.Add(gtx.Ops)
		stack.Pop()
	}
	eventStack := clip.Rect(rect).Push(gtx.Ops)
	event.Op(gtx.Ops, &st.resizeTag)
	eventStack.Pop()
	cursorStack := clip.Rect(rect).Push(gtx.Ops)
	pointer.CursorNorthSouthResize.Add(gtx.Ops)
	cursorStack.Pop()
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *UI) terminalResizeHandleGeometry(th *material.Theme, gtx layout.Context) (image.Rectangle, int, int, bool) {
	if ui == nil || ui.terminal == nil || !ui.terminal.active() || ui.terminalMaximized() {
		return image.Rectangle{}, 0, 0, false
	}
	st := ui.terminal
	h, cellH, ok := st.paneMetrics()
	rows := ui.terminalRowsForPaneHeightWithTabs(gtx, cellH, h)
	if !ok || rows <= 0 {
		_, cellH = ui.terminalCellSize(th, gtx)
		if cellH <= 0 {
			cellH = 16
		}
		rows = terminalConfiguredRows(ui.fmCfg)
		h = ui.terminalPaneHeightWithTabs(gtx, cellH, rows)
	}
	if h <= 0 || gtx.Constraints.Max.X <= 0 || gtx.Constraints.Max.Y <= 0 {
		return image.Rectangle{}, 0, 0, false
	}
	rect := terminalResizeHandleRect(gtx, gtx.Constraints.Max.Y-h)
	if rect.Empty() {
		return image.Rectangle{}, 0, 0, false
	}
	return rect, cellH, rows, true
}

func (ui *UI) terminalCellSize(th *material.Theme, gtx layout.Context) (int, int) {
	face := ui.terminalBaseTypeface()
	size := ui.terminalTextSize()
	cellW := int(math.Ceil(float64(measureTypefaceCharAdvanceAt(th, gtx, face, size))))
	cellH := measureTypefaceLineHeightAt(th, gtx, face, size)
	if cellW < 4 {
		cellW = 4
	}
	if cellH < 8 {
		cellH = 8
	}
	return cellW, cellH
}

func (ui *UI) terminalBaseTypeface() font.Typeface {
	if ui != nil && ui.fmCfg != nil {
		face := strings.TrimSpace(ui.fmCfg.Terminal.Typeface)
		if resources.IsBundledMonospaceFontFamily(face) {
			return font.Typeface(face)
		}
	}
	face := ui.mainTypeface()
	if resources.IsBundledMonospaceFontFamily(string(face)) {
		return face
	}
	return font.Typeface(resources.BundledFontFamilyFiraCodeNerdFontMono)
}

func (ui *UI) terminalTextSize() unit.Sp {
	if ui == nil {
		return scaleConfigFontSize(nil, 13)
	}
	if ui.fmCfg != nil && ui.fmCfg.Terminal.FontSizeSp >= 6 {
		return normalizeUIFontSize(unit.Sp(ui.fmCfg.Terminal.FontSizeSp))
	}
	return scaleConfigFontSize(ui.fmCfg, 13)
}

func (ui *UI) terminalTypeface() font.Typeface {
	base := strings.TrimSpace(string(ui.terminalBaseTypeface()))
	fallbacks := strings.Join([]string{
		resources.BundledFontFamilyFiraCodeNerdFontMono,
		resources.BundledFontFamilyJetBrainsMonoNerdFontMono,
		resources.BundledFontFamilyHackNerdFontMono,
		resources.BundledFontFamilyIosevkaNerdFontMono,
		"Symbols Nerd Font Mono",
		"Symbols Nerd Font",
		"Font Awesome 6 Free",
		"Font Awesome 5 Free",
		"FontAwesome",
		"Font Awesome",
		"Apple Braille",
		"Apple Symbols",
		"Apple Color Emoji",
		"Segoe UI Symbol",
		"Segoe UI Emoji",
		"Noto Sans Symbols",
		"Noto Color Emoji",
		"monospace",
	}, ", ")
	if base == "" {
		return font.Typeface(fallbacks)
	}
	return font.Typeface(base + ", " + fallbacks)
}

func (s *terminalSession) layoutScrollbar(gtx layout.Context, content image.Rectangle) {
	if s == nil || content.Dx() <= 0 || content.Dy() <= 0 {
		return
	}
	s.State.Mu.RLock()
	rows := s.State.Rows
	scrollback := s.State.Scrollback
	viewStart := s.State.ViewStart
	alternate := s.State.Alternate
	s.State.Mu.RUnlock()

	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if alternate || scrollback <= 0 || rows <= 0 {
		s.scrollbarTrack = image.Rectangle{}
		s.scrollbarThumb = image.Rectangle{}
		s.scrollbarHover = false
		s.scrollbarDragging = false
		s.scrollbarDragGrab = 0
		return
	}
	trackW := viewerScrollbarThickness(gtx, content.Dx())
	if trackW <= 0 {
		s.scrollbarTrack = image.Rectangle{}
		s.scrollbarThumb = image.Rectangle{}
		s.scrollbarHover = false
		return
	}
	track := image.Rect(content.Max.X-trackW, content.Min.Y, content.Max.X, content.Max.Y)
	thumb := viewerScrollbarThumbForScroll(track, rows, scrollback+rows, float64(viewStart), true)
	s.scrollbarTrack = track
	s.scrollbarThumb = thumb
	if !s.scrollbarDragging && s.scrollbarHover && thumb.Empty() {
		s.scrollbarHover = false
	}
}

func (s *terminalSession) hitScrollbar(pos image.Point) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	return !s.scrollbarTrack.Empty() && viewerPointInRect(pos, s.scrollbarTrack)
}

func (s *terminalSession) beginScrollbarDrag(gtx layout.Context, pe pointer.Event) {
	if s == nil {
		return
	}
	pos := pe.Position.Round()
	s.viewMu.Lock()
	s.scrollbarDragging = true
	s.scrollbarDragID = pe.PointerID
	if viewerPointInRect(pos, s.scrollbarThumb) {
		s.scrollbarDragGrab = pos.Y - s.scrollbarThumb.Min.Y
	} else {
		s.scrollbarDragGrab = s.scrollbarThumb.Dy() / 2
	}
	s.scrollbarHover = true
	s.viewMu.Unlock()
	gtx.Execute(pointer.GrabCmd{Tag: &s.pointerTag, ID: pe.PointerID})
}

func (s *terminalSession) draggingScrollbar(id pointer.ID) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	return s.scrollbarDragging && s.scrollbarDragID == id
}

func (s *terminalSession) selectingPointer(id pointer.ID) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	return s.selectionSelecting && s.selectionPointer == id
}

func (s *terminalSession) endScrollbarDrag(id pointer.ID) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if !s.scrollbarDragging || s.scrollbarDragID != id {
		return false
	}
	s.scrollbarDragging = false
	s.scrollbarDragGrab = 0
	return true
}

func (s *terminalSession) setScrollFromScrollbarPos(pos image.Point) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	track := s.scrollbarTrack
	thumb := s.scrollbarThumb
	dragGrab := s.scrollbarDragGrab
	s.viewMu.Unlock()
	if track.Empty() || thumb.Empty() {
		return false
	}

	s.State.Mu.RLock()
	scrollback := s.State.Scrollback
	rows := s.State.Rows
	s.State.Mu.RUnlock()
	maxTop := scrollback
	if maxTop <= 0 || rows <= 0 {
		return false
	}
	trackLen := track.Dy()
	thumbLen := thumb.Dy()
	if trackLen <= thumbLen || thumbLen <= 0 {
		return false
	}
	drag := pos.Y - track.Min.Y - dragGrab
	if drag < 0 {
		drag = 0
	}
	maxDrag := trackLen - thumbLen
	if drag > maxDrag {
		drag = maxDrag
	}
	viewStart := int(float64(drag)/float64(maxDrag)*float64(maxTop) + 0.5)
	return s.setScrollOffset(scrollback - viewStart)
}

func (s *terminalSession) updateScrollbarHover(pos image.Point) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if s.scrollbarTrack.Empty() {
		changed := s.scrollbarHover
		s.scrollbarHover = false
		return changed
	}
	old := s.scrollbarHover
	s.scrollbarHover = viewerPointInRect(pos, s.scrollbarTrack)
	return old != s.scrollbarHover
}

func (s *terminalSession) clearScrollbarHover() bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if s.scrollbarDragging {
		return false
	}
	changed := s.scrollbarHover
	s.scrollbarHover = false
	return changed
}

func terminalResizeHandleHeight(gtx layout.Context) int {
	h := gtx.Dp(unit.Dp(10))
	if h < 6 {
		h = 6
	}
	return h
}

func terminalResizeHandleRect(gtx layout.Context, top int) image.Rectangle {
	h := terminalResizeHandleHeight(gtx)
	y0 := top - h/2
	y1 := y0 + h
	if y0 < 0 {
		y0 = 0
	}
	if y1 > gtx.Constraints.Max.Y {
		y1 = gtx.Constraints.Max.Y
	}
	if y1 <= y0 || gtx.Constraints.Max.X <= 0 {
		return image.Rectangle{}
	}
	return image.Rect(0, y0, gtx.Constraints.Max.X, y1)
}

func terminalRoundDiv(delta, divisor int) int {
	if divisor <= 0 {
		return 0
	}
	if delta >= 0 {
		return (delta + divisor/2) / divisor
	}
	return -((-delta + divisor/2) / divisor)
}

func (s *terminalSession) beginResizeDrag(gtx layout.Context, pe pointer.Event, rows int) {
	if s == nil {
		return
	}
	pos := pe.Position.Round()
	s.viewMu.Lock()
	s.resizeDragging = true
	s.resizeDragID = pe.PointerID
	s.resizeDragStartY = pos.Y
	s.resizeDragStartRows = rows
	s.resizeHover = true
	s.viewMu.Unlock()
	gtx.Execute(pointer.GrabCmd{Tag: &s.resizeTag, ID: pe.PointerID})
}

func (s *terminalSession) draggingResize(id pointer.ID) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	return s.resizeDragging && s.resizeDragID == id
}

func (s *terminalSession) resizeDraggingActive() bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	return s.resizeDragging
}

func (s *terminalSession) resizeRowsFromDrag(gtx layout.Context, pos image.Point, cellH int) int {
	if s == nil {
		return terminalClampPaneRows(gtx, cellH, terminalDefaultRows)
	}
	s.viewMu.Lock()
	startY := s.resizeDragStartY
	startRows := s.resizeDragStartRows
	s.viewMu.Unlock()
	return terminalClampPaneRows(gtx, cellH, startRows+terminalRoundDiv(startY-pos.Y, cellH))
}

func (s *terminalSession) endResizeDrag(id pointer.ID) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if !s.resizeDragging || s.resizeDragID != id {
		return false
	}
	s.resizeDragging = false
	s.resizeDragStartY = 0
	s.resizeDragStartRows = 0
	return true
}

func (s *terminalSession) setResizeHover(hover bool) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if s.resizeDragging {
		return false
	}
	old := s.resizeHover
	s.resizeHover = hover
	return old != s.resizeHover
}

func (s *terminalSession) setPaneMetrics(height, cellH int) {
	if s == nil {
		return
	}
	if height < 0 {
		height = 0
	}
	if cellH < 0 {
		cellH = 0
	}
	s.viewMu.Lock()
	s.paneHeight = height
	s.paneCellHeight = cellH
	s.viewMu.Unlock()
}

func (s *terminalSession) paneMetrics() (height, cellH int, ok bool) {
	if s == nil {
		return 0, 0, false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if s.paneHeight <= 0 || s.paneCellHeight <= 0 {
		return 0, 0, false
	}
	return s.paneHeight, s.paneCellHeight, true
}

func (s *terminalSession) clearResizeHover() bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if s.resizeDragging {
		return false
	}
	changed := s.resizeHover
	s.resizeHover = false
	return changed
}

func (s *terminalSession) resizeHandleActive() bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	return s.resizeDragging || s.resizeHover
}

func (s *terminalSession) handleResizePointer(gtx layout.Context, handle image.Rectangle, cellH, rows int, onResize terminalResizeRowsFunc) bool {
	if s == nil {
		return false
	}
	handled := false
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &s.resizeTag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Move | pointer.Enter | pointer.Leave,
		})
		if !ok {
			return handled
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		pos := pe.Position.Round()
		switch pe.Kind {
		case pointer.Press:
			if pe.Buttons.Contain(pointer.ButtonPrimary) && viewerPointInRect(pos, handle) {
				s.beginResizeDrag(gtx, pe, terminalClampPaneRows(gtx, cellH, rows))
				handled = true
			}
		case pointer.Drag:
			if s.draggingResize(pe.PointerID) {
				if onResize != nil {
					onResize(s.resizeRowsFromDrag(gtx, pos, cellH), false)
				}
				handled = true
			}
		case pointer.Release, pointer.Cancel:
			if s.draggingResize(pe.PointerID) {
				nextRows := s.resizeRowsFromDrag(gtx, pos, cellH)
				s.endResizeDrag(pe.PointerID)
				if onResize != nil {
					onResize(nextRows, true)
				}
				handled = true
			}
			if pe.Kind == pointer.Cancel {
				if s.clearResizeHover() {
					handled = true
				}
			} else if s.setResizeHover(viewerPointInRect(pos, handle)) {
				handled = true
			}
		case pointer.Move, pointer.Enter:
			if s.setResizeHover(true) {
				handled = true
			}
		case pointer.Leave:
			if s.clearResizeHover() {
				handled = true
			}
		}
	}
}

func (s *terminalSession) applyTerminalCursor(gtx layout.Context, content image.Rectangle) {
	if s == nil {
		return
	}
	s.viewMu.Lock()
	track := s.scrollbarTrack
	hover := s.scrollbarHover
	dragging := s.scrollbarDragging
	resizeDragging := s.resizeDragging
	s.viewMu.Unlock()
	if resizeDragging {
		return
	}
	if dragging {
		pointer.CursorGrabbing.Add(gtx.Ops)
		return
	}
	if hover && !track.Empty() {
		defer clip.Rect(track).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		return
	}
	if !content.Empty() {
		defer clip.Rect(content).Push(gtx.Ops).Pop()
		pointer.CursorText.Add(gtx.Ops)
	}
}

func (ui *UI) drawTerminalScrollbar(gtx layout.Context, st *terminalSession) {
	if st == nil {
		return
	}
	st.viewMu.Lock()
	track := st.scrollbarTrack
	thumb := st.scrollbarThumb
	hover := st.scrollbarHover
	dragging := st.scrollbarDragging
	st.viewMu.Unlock()
	if track.Empty() {
		return
	}
	paintViewerScrollbar(gtx, ui.fileViewerTheme(), track, thumb, hover, hover, dragging)
}

func (s *terminalSession) openContextMenu(pos image.Point, now time.Time) {
	if s == nil {
		return
	}
	s.viewMu.Lock()
	s.menuOpen = true
	s.menuPos = pos
	s.menuRect = image.Rectangle{}
	s.menuItemRects = nil
	s.menuOpenedAt = now
	s.menuHoverID = ""
	s.menuHoverAnim = segmentedAnimState{}
	s.viewMu.Unlock()
}

func (s *terminalSession) closeContextMenu() bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	changed := s.menuOpen
	s.menuOpen = false
	s.menuRect = image.Rectangle{}
	s.menuItemRects = nil
	s.menuOpenedAt = time.Time{}
	s.menuHoverID = ""
	s.menuHoverAnim = segmentedAnimState{}
	s.viewMu.Unlock()
	return changed
}

func (ui *UI) layoutTerminalContextMenu(th *material.Theme, gtx layout.Context, st *terminalSession) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	st.viewMu.Lock()
	open := st.menuOpen
	st.viewMu.Unlock()
	if !open {
		return layout.Dimensions{}
	}

	items := ui.terminalContextMenuItems(st)
	if ui.handleTerminalContextMenuPointer(gtx, st) {
		gtx.Execute(op.InvalidateCmd{})
	}

	st.viewMu.Lock()
	open = st.menuOpen
	openedAt := st.menuOpenedAt
	menuPos := st.menuPos
	st.viewMu.Unlock()
	if !open {
		return layout.Dimensions{}
	}

	alpha, slideY, animating := popupOpenProgress(gtx.Now, openedAt)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	menuDims := ui.terminalContextMenuSize(th, gtx, items)
	blockClip := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, &st.menuTag)
	blockClip.Pop()

	anchor := menuPos
	anchor.Y += slideY
	anchor = clampFilePaneMenuPoint(anchor, menuDims, gtx.Constraints.Max)
	st.setTerminalMenuGeometry(anchor, menuDims, ui.terminalContextMenuItemRects(gtx, anchor, menuDims.X, items))
	if st.setTerminalMenuHover(terminalContextMenuHoveredItemID(items), gtx.Now) {
		gtx.Execute(op.InvalidateCmd{})
	}

	bodyClip := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	offset := op.Offset(anchor).Push(gtx.Ops)
	cardGtx := gtx
	cardGtx.Constraints = layout.Exact(menuDims)
	ui.layoutTerminalContextMenuCard(th, cardGtx, st, items, menuDims.X, alpha)
	offset.Pop()
	bodyClip.Pop()

	menuClip := clip.Rect(image.Rectangle{Min: anchor, Max: anchor.Add(menuDims)}).Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &st.menuActionTag)
	pass.Pop()
	menuClip.Pop()

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *UI) handleTerminalContextMenuPointer(gtx layout.Context, st *terminalSession) bool {
	if ui == nil || st == nil {
		return false
	}
	changed := false
	consumedSurface := false
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &st.menuActionTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		consumedSurface = true
		if !pe.Buttons.Contain(pointer.ButtonPrimary) {
			continue
		}
		pos := pe.Position.Round()
		id, disabled, ok := st.terminalMenuItemAt(pos)
		if !ok || disabled {
			continue
		}
		ui.activateTerminalContextMenuItem(gtx, st, id)
		changed = true
	}
	if consumedSurface {
		return changed
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &st.menuTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		pos := pe.Position.Round()
		if pe.Buttons.Contain(pointer.ButtonPrimary) {
			if id, disabled, ok := st.terminalMenuItemAt(pos); ok {
				if !disabled {
					ui.activateTerminalContextMenuItem(gtx, st, id)
					changed = true
				}
				continue
			}
		}
		if st.terminalMenuContains(pos) {
			continue
		}
		if pe.Buttons.Contain(pointer.ButtonSecondary) {
			st.openContextMenu(pos, gtx.Now)
			changed = true
			continue
		}
		if pe.Buttons.Contain(pointer.ButtonPrimary) {
			if st.closeContextMenu() {
				changed = true
			}
		}
	}
	return changed
}

func (ui *UI) activateTerminalContextMenuItem(gtx layout.Context, st *terminalSession, id string) {
	if ui == nil || st == nil {
		return
	}
	switch id {
	case "copy":
		_ = ui.copyTerminalText(gtx, true)
	case "paste":
		_ = ui.pasteTerminalText(gtx)
	case "select-all":
		_ = st.selectAll()
	case "go-pane-1":
		ui.terminalGoToPaneDir(0, gtx.Now)
	case "go-pane-2":
		ui.terminalGoToPaneDir(1, gtx.Now)
	case "set-pane-1":
		ui.setPaneDirToTerminalDir(0, gtx.Now)
	case "set-pane-2":
		ui.setPaneDirToTerminalDir(1, gtx.Now)
	}
	st.closeContextMenu()
}

type terminalContextMenuItem struct {
	id        string
	label     string
	click     *widget.Clickable
	separator bool
	disabled  bool
}

type terminalMenuItemRect struct {
	id       string
	rect     image.Rectangle
	disabled bool
}

func (ui *UI) terminalContextMenuSize(th *material.Theme, gtx layout.Context, items []terminalContextMenuItem) image.Point {
	width := 0
	for _, item := range items {
		if item.separator {
			continue
		}
		lbl := material.Body2(th, item.label)
		lbl.Font.Typeface = ui.mainTypeface()
		lbl.TextSize = ui.functionBarTextSize()
		lbl.Font.Weight = font.Medium
		if measured := measureLabelUnconstrained(gtx, lbl).Size.X + gtx.Dp(unit.Dp(18)); measured > width {
			width = measured
		}
	}
	if minW := gtx.Dp(unit.Dp(122)); width < minW {
		width = minW
	}
	if width > gtx.Constraints.Max.X {
		width = gtx.Constraints.Max.X
	}
	if width < 1 {
		width = 1
	}
	height := gtx.Dp(unit.Dp(2))
	for _, item := range items {
		if item.separator {
			height += ui.fileContextMenuSeparatorHeight(gtx)
			continue
		}
		height += ui.fileContextMenuRowHeight(gtx, fileContextMenuItem{ID: item.id, Label: item.label, Disabled: item.disabled})
	}
	if height < 1 {
		height = 1
	}
	if height > gtx.Constraints.Max.Y {
		height = gtx.Constraints.Max.Y
	}
	return image.Pt(width, height)
}

func (ui *UI) terminalContextMenuItemRects(gtx layout.Context, anchor image.Point, width int, items []terminalContextMenuItem) []terminalMenuItemRect {
	y := anchor.Y + gtx.Dp(unit.Dp(1))
	rects := make([]terminalMenuItemRect, 0, len(items))
	for _, item := range items {
		if item.separator {
			y += ui.fileContextMenuSeparatorHeight(gtx)
			continue
		}
		rowH := ui.fileContextMenuRowHeight(gtx, fileContextMenuItem{ID: item.id, Label: item.label, Disabled: item.disabled})
		rects = append(rects, terminalMenuItemRect{
			id:       item.id,
			rect:     image.Rect(anchor.X, y, anchor.X+width, y+rowH),
			disabled: item.disabled,
		})
		y += rowH
	}
	return rects
}

func (s *terminalSession) setTerminalMenuGeometry(anchor, size image.Point, items []terminalMenuItemRect) {
	if s == nil {
		return
	}
	s.viewMu.Lock()
	s.menuRect = image.Rectangle{Min: anchor, Max: anchor.Add(size)}
	s.menuItemRects = append(s.menuItemRects[:0], items...)
	s.viewMu.Unlock()
}

func (s *terminalSession) terminalMenuContains(pos image.Point) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	rect := s.menuRect
	s.viewMu.Unlock()
	return viewerPointInRect(pos, rect)
}

func (s *terminalSession) terminalMenuItemAt(pos image.Point) (id string, disabled bool, ok bool) {
	if s == nil {
		return "", false, false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	for _, item := range s.menuItemRects {
		if viewerPointInRect(pos, item.rect) {
			return item.id, item.disabled, true
		}
	}
	return "", false, false
}

func (s *terminalSession) setTerminalMenuHover(id string, now time.Time) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if s.menuHoverID == id {
		return false
	}
	s.menuHoverID = id
	s.menuHoverAnim.setHover(id, now)
	return true
}

func (ui *UI) layoutTerminalContextMenuCard(th *material.Theme, gtx layout.Context, st *terminalSession, items []terminalContextMenuItem, width int, alpha float32) layout.Dimensions {
	theme := ui.filePanePopupTheme()
	cardGtx := gtx
	cardGtx.Constraints.Min.Y = 0
	return fixedWidth(cardGtx, width, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = 0
		return fillRoundedClipBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
			scaleColorAlpha(theme.Bg, alpha),
			scaleColorAlpha(theme.Border, alpha),
			func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = 0
				return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0, len(items))
					for _, item := range items {
						item := item
						if item.separator {
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								sepH := ui.fileContextMenuSeparatorHeight(gtx)
								return fixedHeight(gtx, sepH, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										h := gtx.Dp(unit.Dp(1))
										if h < 1 {
											h = 1
										}
										return fillBgExact(gtx, scaleColorAlpha(theme.Divider, alpha), func(gtx layout.Context) layout.Dimensions {
											return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
										})
									})
								})
							}))
							continue
						}
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							menuItem := fileContextMenuItem{ID: item.id, Label: item.label, Disabled: item.disabled}
							hoverFill, hoverAnim := st.terminalMenuHoverFill(gtx.Now, item.id)
							if hoverAnim {
								gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
							}
							dims, animating := ui.layoutTerminalContextMenuItem(th, gtx, theme, item.click, menuItem, hoverFill, alpha, width, ui.fileContextMenuRowHeight(gtx, menuItem))
							if animating {
								gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
							}
							return dims
						}))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			},
		)
	})
}

func (ui *UI) terminalContextMenuItems(st *terminalSession) []terminalContextMenuItem {
	if st == nil {
		return nil
	}
	paneDirAvailable := func(idx int) bool {
		return ui != nil && idx >= 0 && idx < len(ui.filePanes) && terminalPaneLocalDir(ui.filePanes[idx]) != ""
	}
	return []terminalContextMenuItem{
		{id: "copy", label: "Copy", click: &st.menuCopy},
		{id: "paste", label: "Paste", click: &st.menuPaste},
		{id: "select-all", label: "Select All", click: &st.menuSelectAll},
		{id: "sep-pane", separator: true},
		{id: "go-pane-1", label: "cd to Left Pane", click: &st.menuGoPane1, disabled: !paneDirAvailable(0)},
		{id: "go-pane-2", label: "cd to Right Pane", click: &st.menuGoPane2, disabled: !paneDirAvailable(1)},
		{id: "set-pane-1", label: "Set Left Pane to Terminal Dir", click: &st.menuSetPane1, disabled: ui == nil || len(ui.filePanes) <= 0 || ui.filePanes[0] == nil},
		{id: "set-pane-2", label: "Set Right Pane to Terminal Dir", click: &st.menuSetPane2, disabled: ui == nil || len(ui.filePanes) <= 1 || ui.filePanes[1] == nil},
	}
}

func (ui *UI) layoutTerminalContextMenuItem(th *material.Theme, gtx layout.Context, theme filePanePopupTheme, click *widget.Clickable, item fileContextMenuItem, hoverFill, alpha float32, width, rowH int) (layout.Dimensions, bool) {
	if click == nil {
		return layout.Dimensions{}, false
	}
	hoverT := smoothstep01(clamp01(hoverFill))
	dims := click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if !item.Disabled {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		bg := color.NRGBA{}
		fg := scaleColorAlpha(theme.Text, alpha)
		if item.Disabled {
			fg = scaleColorAlpha(theme.DisabledText, alpha)
		} else if hoverT > 0 {
			bg = scaleColorAlpha(theme.HoverBg, alpha*hoverT)
			fg = scaleColorAlpha(mixNRGBA(theme.Text, theme.HoverText, hoverT), alpha)
		}
		return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, rowH, func(gtx layout.Context) layout.Dimensions {
				return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, item.Label)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = ui.functionBarTextSize()
						lbl.Font.Weight = font.Medium
						lbl.Color = fg
						lbl.MaxLines = 1
						lbl.Truncator = "..."
						return layoutVCenteredLabel(gtx, lbl)
					})
				})
			})
		})
	})
	return dims, hoverT > 0 && hoverT < 1
}

func (s *terminalSession) terminalMenuHoverFill(now time.Time, id string) (float32, bool) {
	if s == nil {
		return 0, false
	}
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	fill, animating := s.menuHoverAnim.hoverFill(now, id)
	return fill, animating
}

func terminalContextMenuHoveredItemID(items []terminalContextMenuItem) string {
	for _, item := range items {
		if item.separator || item.disabled || item.click == nil {
			continue
		}
		if item.click.Hovered() {
			return item.id
		}
	}
	return ""
}

type terminalMouseReportModes struct {
	clicks     bool
	cellMotion bool
	allMotion  bool
	utf8       bool
	sgr        bool
}

func (m terminalMouseReportModes) reporting() bool {
	return m.clicks || m.cellMotion || m.allMotion
}

func (m terminalMouseReportModes) reportingMotion(buttonDown bool) bool {
	return m.allMotion || (m.cellMotion && buttonDown)
}

type terminalResizeRowsFunc func(rows int, final bool)

func (s *terminalSession) handlePointer(gtx layout.Context, content image.Rectangle, cellW, cellH int) bool {
	handled := false
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target:  &s.pointerTag,
			Kinds:   pointer.Scroll | pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Move | pointer.Enter | pointer.Leave,
			ScrollY: pointer.ScrollRange{Min: -terminalScrollRange, Max: terminalScrollRange},
		})
		if !ok {
			return handled
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		pos := pe.Position.Round()
		switch pe.Kind {
		case pointer.Scroll:
			if viewerPointInRect(pos, content) {
				gtx.Execute(key.FocusCmd{Tag: &s.keyTag})
			}
			if s.terminalMouseReporting() && viewerPointInRect(pos, content) {
				if s.reportTerminalMouseWheel(pe, content, cellW, cellH) {
					handled = true
					continue
				}
			}
			if pe.Scroll.Y != 0 && s.scrollByDelta(pe.Scroll.Y) {
				handled = true
			}
		case pointer.Press:
			if s.contextMenuConsumesPointer(pos) {
				handled = true
				continue
			}
			gtx.Execute(key.FocusCmd{Tag: &s.keyTag})
			// Middle-click is always the terminal's primary-selection paste
			// shortcut. Handle it before application mouse reporting so programs
			// that enable mouse mode cannot swallow the paste gesture.
			if pe.Buttons.Contain(pointer.ButtonTertiary) {
				s.closeContextMenu()
				if viewerPointInRect(pos, content) {
					_ = s.pastePrimarySelectionOrClipboard(gtx)
				}
				handled = true
				continue
			}
			if s.terminalMouseReporting() && viewerPointInRect(pos, content) {
				if s.reportTerminalMousePress(pe, content, cellW, cellH) {
					gtx.Execute(pointer.GrabCmd{Tag: &s.pointerTag, ID: pe.PointerID})
					handled = true
					continue
				}
			}
			if pe.Buttons.Contain(pointer.ButtonSecondary) {
				s.openContextMenu(pos, gtx.Now)
				handled = true
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) && s.hitScrollbar(pos) {
				s.beginScrollbarDrag(gtx, pe)
				handled = true
				if s.setScrollFromScrollbarPos(pos) {
					handled = true
				}
				continue
			}
			if pe.Buttons.Contain(pointer.ButtonPrimary) {
				s.closeContextMenu()
				if viewerPointInRect(pos, content) {
					doubleClick := s.registerPrimaryPress(gtx.Now, pos)
					if doubleClick && s.selectWordAtPosition(pos, content, cellW, cellH) {
						handled = true
						continue
					}
					if s.beginSelection(pos, content, cellW, cellH, pe.PointerID) {
						gtx.Execute(pointer.GrabCmd{Tag: &s.pointerTag, ID: pe.PointerID})
					}
				} else {
					s.clearSelection()
				}
				handled = true
			}
		case pointer.Drag:
			if s.reportTerminalMouseMotion(pe, content, cellW, cellH) {
				handled = true
				continue
			}
			if s.selectingPointer(pe.PointerID) {
				if s.updateSelection(pos, content, cellW, cellH, gtx.Now) {
					handled = true
				}
				continue
			}
			if s.draggingScrollbar(pe.PointerID) {
				if s.setScrollFromScrollbarPos(pos) {
					handled = true
				}
			}
			if s.updateScrollbarHover(pos) {
				handled = true
			}
		case pointer.Release, pointer.Cancel:
			if s.reportTerminalMouseRelease(pe, content, cellW, cellH) {
				handled = true
				continue
			}
			if s.endScrollbarDrag(pe.PointerID) {
				handled = true
			}
			if s.endSelection(pe.PointerID) {
				handled = true
			}
			if s.updateScrollbarHover(pos) {
				handled = true
			}
		case pointer.Move, pointer.Enter:
			if s.reportTerminalMouseMotion(pe, content, cellW, cellH) {
				handled = true
				continue
			}
			if s.updateScrollbarHover(pos) {
				handled = true
			}
		case pointer.Leave:
			if s.clearScrollbarHover() {
				handled = true
			}
		}
	}
}

func (s *terminalSession) reportTerminalMousePress(pe pointer.Event, content image.Rectangle, cellW, cellH int) bool {
	button, ok := terminalMouseButtonFromButtons(pe.Buttons)
	if !ok {
		return false
	}
	row, col, ok := s.terminalMouseCell(pe.Position.Round(), content, cellW, cellH)
	if !ok {
		return false
	}
	modes := s.terminalMouseModes()
	if !modes.reporting() {
		return false
	}
	data := terminalMouseReportBytes(button, row, col, false, false, pe.Modifiers, modes)
	if len(data) == 0 {
		return false
	}
	s.beginTerminalMousePress(pe.PointerID, button)
	s.write(data)
	return true
}

func (s *terminalSession) reportTerminalMouseRelease(pe pointer.Event, content image.Rectangle, cellW, cellH int) bool {
	button, ok := s.endTerminalMousePress(pe.PointerID)
	if !ok {
		return false
	}
	row, col, ok := s.terminalMouseCell(pe.Position.Round(), content, cellW, cellH)
	if !ok {
		return false
	}
	modes := s.terminalMouseModes()
	if !modes.reporting() {
		return false
	}
	data := terminalMouseReportBytes(button, row, col, true, false, pe.Modifiers, modes)
	if len(data) == 0 {
		return false
	}
	s.write(data)
	return true
}

func (s *terminalSession) reportTerminalMouseMotion(pe pointer.Event, content image.Rectangle, cellW, cellH int) bool {
	modes := s.terminalMouseModes()
	if !modes.reportingMotion(false) {
		if _, pressed := s.terminalMousePressButton(pe.PointerID); !modes.reportingMotion(pressed) {
			return false
		}
	}
	button, pressed := s.terminalMousePressButton(pe.PointerID)
	if !pressed {
		button = 0
	}
	if !modes.reportingMotion(pressed) {
		return false
	}
	row, col, ok := s.terminalMouseCell(pe.Position.Round(), content, cellW, cellH)
	if !ok {
		return false
	}
	data := terminalMouseReportBytes(button, row, col, false, true, pe.Modifiers, modes)
	if len(data) == 0 {
		return false
	}
	s.write(data)
	return true
}

func (s *terminalSession) reportTerminalMouseWheel(pe pointer.Event, content image.Rectangle, cellW, cellH int) bool {
	if pe.Scroll.Y == 0 {
		return false
	}
	row, col, ok := s.terminalMouseCell(pe.Position.Round(), content, cellW, cellH)
	if !ok {
		return false
	}
	modes := s.terminalMouseModes()
	if !modes.reporting() {
		return false
	}
	button := 65
	if pe.Scroll.Y < 0 {
		button = 64
	}
	data := terminalMouseReportBytes(button, row, col, false, false, pe.Modifiers, modes)
	if len(data) == 0 {
		return false
	}
	s.write(data)
	return true
}

func (s *terminalSession) terminalMouseCell(pos image.Point, content image.Rectangle, cellW, cellH int) (row, col int, ok bool) {
	if s == nil || cellW <= 0 || cellH <= 0 || !viewerPointInRect(pos, content) {
		return 0, 0, false
	}
	s.State.Mu.RLock()
	rows := s.State.Rows
	cols := s.State.Cols
	s.State.Mu.RUnlock()
	if rows <= 0 || cols <= 0 {
		return 0, 0, false
	}
	col = (pos.X - content.Min.X) / cellW
	row = (pos.Y - content.Min.Y) / cellH
	if col < 0 {
		col = 0
	}
	if row < 0 {
		row = 0
	}
	if col >= cols {
		col = cols - 1
	}
	if row >= rows {
		row = rows - 1
	}
	return row, col, true
}

func terminalMouseButtonFromButtons(buttons pointer.Buttons) (int, bool) {
	switch {
	case buttons.Contain(pointer.ButtonPrimary):
		return 0, true
	case buttons.Contain(pointer.ButtonTertiary):
		return 1, true
	case buttons.Contain(pointer.ButtonSecondary):
		return 2, true
	default:
		return 0, false
	}
}

func terminalMouseReportBytes(button, row, col int, release, motion bool, mods key.Modifiers, modes terminalMouseReportModes) []byte {
	if row < 0 || col < 0 {
		return nil
	}
	x := col + 1
	y := row + 1
	code := button | terminalMouseModifierBits(mods)
	if motion {
		code |= 32
	}
	if modes.sgr {
		return terminalMouseSGRReportBytes(code, x, y, release)
	}
	if release {
		code = 3 | terminalMouseModifierBits(mods)
	}
	return terminalMouseLegacyReportBytes(code, x, y, modes.utf8)
}

func terminalMouseModifierBits(mods key.Modifiers) int {
	bits := 0
	if mods.Contain(key.ModShift) {
		bits |= 4
	}
	if mods.Contain(key.ModAlt) {
		bits |= 8
	}
	if mods.Contain(key.ModCtrl) {
		bits |= 16
	}
	return bits
}

func terminalMouseSGRReportBytes(code, x, y int, release bool) []byte {
	out := []byte("\x1b[<")
	out = append(out, strconvItoa(code)...)
	out = append(out, ';')
	out = append(out, strconvItoa(x)...)
	out = append(out, ';')
	out = append(out, strconvItoa(y)...)
	if release {
		out = append(out, 'm')
	} else {
		out = append(out, 'M')
	}
	return out
}

func terminalMouseLegacyReportBytes(code, x, y int, utf8Mouse bool) []byte {
	if code < 0 || x <= 0 || y <= 0 {
		return nil
	}
	if !utf8Mouse && (code+32 > 255 || x+32 > 255 || y+32 > 255) {
		return nil
	}
	out := []byte{0x1b, '[', 'M'}
	out = appendTerminalMouseEncodedCoord(out, code+32, utf8Mouse)
	out = appendTerminalMouseEncodedCoord(out, x+32, utf8Mouse)
	out = appendTerminalMouseEncodedCoord(out, y+32, utf8Mouse)
	return out
}

func appendTerminalMouseEncodedCoord(out []byte, value int, utf8Mouse bool) []byte {
	if utf8Mouse && value > 255 {
		return append(out, string(rune(value))...)
	}
	return append(out, byte(value))
}

func (s *terminalSession) contextMenuConsumesPointer(pos image.Point) bool {
	if s == nil {
		return false
	}
	s.viewMu.Lock()
	open := s.menuOpen
	rect := s.menuRect
	s.viewMu.Unlock()
	return open && viewerPointInRect(pos, rect)
}

func (s *terminalSession) handleInput(gtx layout.Context) bool {
	return s.handleInputWithAcceleratedKeys(gtx, true)
}

func (s *terminalSession) handleInputWithAcceleratedKeys(gtx layout.Context, acceleratedKeys bool) bool {
	anyMods := ^key.Modifiers(0)
	handled := false
	if !acceleratedKeys {
		s.stopKeyRepeat("")
	}
	for {
		ev, ok := gtx.Event(
			key.FocusFilter{Target: &s.keyTag},
			key.Filter{Focus: &s.keyTag, Name: key.NameReturn, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameEnter, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameTab, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameEscape, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameDeleteBackward, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameDeleteForward, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameUpArrow, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameDownArrow, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameLeftArrow, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameRightArrow, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameHome, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameEnd, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NamePageUp, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NamePageDown, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameF1, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameF2, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameF3, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameF4, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameF5, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameF6, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameF7, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameF8, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameF9, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameF10, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameF11, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Name: key.NameF12, Optional: anyMods},
			key.Filter{Focus: &s.keyTag, Optional: anyMods},
		)
		if !ok {
			if acceleratedKeys && s.pumpKeyRepeat(gtx) {
				handled = true
			}
			return handled
		}
		switch ev := ev.(type) {
		case key.FocusEvent:
			if ev.Focus {
				gtx.Execute(key.SoftKeyboardCmd{Show: true})
			} else {
				s.stopKeyRepeat("")
			}
		case key.EditEvent:
			if ev.Text != "" {
				s.writeString(ev.Text)
				handled = true
			}
		case key.Event:
			if ev.State == key.Release {
				if terminalRepeatableKey(ev.Name) {
					s.stopKeyRepeat(ev.Name)
				}
				continue
			}
			if ev.State != key.Press {
				continue
			}
			if acceleratedKeys && terminalPlainRepeatableKey(ev) {
				if s.keyRepeating(ev.Name) {
					handled = true // Ignore operating-system repeat events.
					continue
				}
				s.stopKeyRepeat("")
				if data := terminalKeyBytesForMode(ev, s.cursorKeysApplication()); len(data) > 0 {
					s.write(data)
					s.startKeyRepeat(ev.Name, gtx.Now)
					if next, ok := s.keyRepeatDeadline(); ok {
						gtx.Execute(op.InvalidateCmd{At: next})
					}
					handled = true
				}
				continue
			}
			s.stopKeyRepeat("")
			if terminalFindKey(ev) {
				s.openFind(gtx)
				handled = true
				continue
			}
			if terminalClearBufferKey(ev) {
				if s.clearBuffer() {
					handled = true
				}
				continue
			}
			if terminalCopyKey(ev) && s.hasActiveSelection() {
				if s.copyText(gtx, false) {
					handled = true
				}
				continue
			}
			if terminalInterruptKey(ev) {
				s.write([]byte{0x03})
				handled = true
				continue
			}
			if terminalPasteKey(ev) {
				if s.pasteText(gtx) {
					handled = true
				}
				continue
			}
			if terminalSelectAllKey(ev) {
				if s.selectAll() {
					handled = true
				}
				continue
			}
			if data := terminalKeyBytesForMode(ev, s.cursorKeysApplication()); len(data) > 0 {
				s.write(data)
				handled = true
			}
		}
	}
}

func terminalRepeatableKey(name key.Name) bool {
	return name == key.NameLeftArrow ||
		name == key.NameRightArrow ||
		name == key.NameDeleteBackward ||
		name == key.NameDeleteForward
}

func terminalPlainRepeatableKey(ev key.Event) bool {
	return ev.Modifiers == 0 && terminalRepeatableKey(ev.Name)
}

func (s *terminalSession) startKeyRepeat(name key.Name, now time.Time) {
	if s == nil || !terminalRepeatableKey(name) {
		return
	}
	s.inputMu.Lock()
	s.keyRepeatActive = true
	s.keyRepeatKey = name
	s.keyRepeatStarted = now
	s.keyRepeatPeriod = repeatSlow
	s.keyRepeatNext = now.Add(repeatStartDelay)
	s.inputMu.Unlock()
}

func (s *terminalSession) stopKeyRepeat(name key.Name) bool {
	if s == nil {
		return false
	}
	s.inputMu.Lock()
	defer s.inputMu.Unlock()
	if !s.keyRepeatActive || (name != "" && s.keyRepeatKey != name) {
		return false
	}
	s.keyRepeatActive = false
	s.keyRepeatKey = ""
	s.keyRepeatStarted = time.Time{}
	s.keyRepeatNext = time.Time{}
	s.keyRepeatPeriod = 0
	return true
}

func (s *terminalSession) keyRepeating(name key.Name) bool {
	if s == nil {
		return false
	}
	s.inputMu.Lock()
	defer s.inputMu.Unlock()
	return s.keyRepeatActive && s.keyRepeatKey == name
}

func (s *terminalSession) keyRepeatDeadline() (time.Time, bool) {
	if s == nil {
		return time.Time{}, false
	}
	s.inputMu.Lock()
	defer s.inputMu.Unlock()
	return s.keyRepeatNext, s.keyRepeatActive
}

func (s *terminalSession) pumpKeyRepeat(gtx layout.Context) bool {
	if s == nil {
		return false
	}
	s.inputMu.Lock()
	if !s.keyRepeatActive {
		s.inputMu.Unlock()
		return false
	}
	if gtx.Now.Sub(s.keyRepeatStarted) >= repeatAccelAfter && s.keyRepeatPeriod != repeatFast {
		s.keyRepeatPeriod = repeatFast
		if s.keyRepeatNext.Before(gtx.Now) {
			s.keyRepeatNext = gtx.Now.Add(s.keyRepeatPeriod)
		}
	}
	due := !gtx.Now.Before(s.keyRepeatNext)
	name := s.keyRepeatKey
	if due {
		s.keyRepeatNext = gtx.Now.Add(s.keyRepeatPeriod)
	}
	next := s.keyRepeatNext
	s.inputMu.Unlock()

	if due {
		s.write(terminalKeyBytesForMode(key.Event{Name: name, State: key.Press}, s.cursorKeysApplication()))
	}
	gtx.Execute(op.InvalidateCmd{At: next})
	return due
}

func terminalClearBufferKey(ev key.Event) bool {
	return terminalClearBufferKeyForGOOS(ev, runtime.GOOS)
}

func terminalClearBufferKeyForGOOS(ev key.Event, goos string) bool {
	if ev.Name != "K" && ev.Name != "k" {
		return false
	}
	if goos == "darwin" {
		return ev.Modifiers == key.ModCommand
	}
	return ev.Modifiers == key.ModCtrl|key.ModShift
}

func terminalCopyKey(ev key.Event) bool {
	return terminalCopyKeyForGOOS(ev, runtime.GOOS)
}

func terminalCopyKeyForGOOS(ev key.Event, goos string) bool {
	if ev.Name != "C" && ev.Name != "c" {
		return false
	}
	if goos == "darwin" {
		return ev.Modifiers == key.ModCommand
	}
	return ev.Modifiers == key.ModCtrl|key.ModShift
}

func terminalInterruptKey(ev key.Event) bool {
	return terminalInterruptKeyForGOOS(ev, runtime.GOOS)
}

func terminalInterruptKeyForGOOS(ev key.Event, goos string) bool {
	if ev.Name != "C" && ev.Name != "c" {
		return false
	}
	if ev.Modifiers == key.ModCtrl {
		return true
	}
	return goos == "darwin" && ev.Modifiers == key.ModCommand
}

func terminalPasteKey(ev key.Event) bool {
	if ev.Name != "V" && ev.Name != "v" {
		return false
	}
	return ev.Modifiers.Contain(key.ModCtrl) || ev.Modifiers.Contain(key.ModShortcut)
}

func terminalSelectAllKey(ev key.Event) bool {
	if ev.Name != "A" && ev.Name != "a" {
		return false
	}
	return ev.Modifiers.Contain(key.ModCtrl) || ev.Modifiers.Contain(key.ModShortcut)
}

func terminalKeyBytes(ev key.Event) []byte {
	return terminalKeyBytesForMode(ev, false)
}

func terminalKeyBytesForMode(ev key.Event, cursorKeysApplication bool) []byte {
	if ev.Modifiers.Contain(key.ModCtrl) {
		if b, ok := terminalCtrlByte(ev.Name); ok {
			return []byte{b}
		}
	}
	switch ev.Name {
	case key.NameReturn, key.NameEnter:
		return []byte("\r")
	case key.NameTab:
		if ev.Modifiers.Contain(key.ModShift) {
			return []byte("\x1b[Z")
		}
		return []byte("\t")
	case key.NameEscape:
		return []byte("\x1b")
	case key.NameDeleteBackward:
		return []byte{0x7f}
	case key.NameDeleteForward:
		return []byte("\x1b[3~")
	case key.NameUpArrow:
		if cursorKeysApplication {
			return []byte("\x1bOA")
		}
		return []byte("\x1b[A")
	case key.NameDownArrow:
		if cursorKeysApplication {
			return []byte("\x1bOB")
		}
		return []byte("\x1b[B")
	case key.NameRightArrow:
		if cursorKeysApplication {
			return []byte("\x1bOC")
		}
		return []byte("\x1b[C")
	case key.NameLeftArrow:
		if cursorKeysApplication {
			return []byte("\x1bOD")
		}
		return []byte("\x1b[D")
	case key.NameHome:
		if cursorKeysApplication {
			return []byte("\x1bOH")
		}
		return []byte("\x1b[H")
	case key.NameEnd:
		if cursorKeysApplication {
			return []byte("\x1bOF")
		}
		return []byte("\x1b[F")
	case key.NamePageUp:
		return []byte("\x1b[5~")
	case key.NamePageDown:
		return []byte("\x1b[6~")
	case key.NameF1:
		return []byte("\x1bOP")
	case key.NameF2:
		return []byte("\x1bOQ")
	case key.NameF3:
		return []byte("\x1bOR")
	case key.NameF4:
		return []byte("\x1bOS")
	case key.NameF5:
		return []byte("\x1b[15~")
	case key.NameF6:
		return []byte("\x1b[17~")
	case key.NameF7:
		return []byte("\x1b[18~")
	case key.NameF8:
		return []byte("\x1b[19~")
	case key.NameF9:
		return []byte("\x1b[20~")
	case key.NameF10:
		return []byte("\x1b[21~")
	case key.NameF11:
		return []byte("\x1b[23~")
	case key.NameF12:
		return []byte("\x1b[24~")
	default:
		if ev.Modifiers.Contain(key.ModAlt) && !ev.Modifiers.Contain(key.ModCtrl) {
			if r, ok := singleKeyRune(ev.Name); ok {
				return []byte("\x1b" + string(r))
			}
		}
	}
	return nil
}

func terminalCtrlByte(name key.Name) (byte, bool) {
	if name == key.NameSpace {
		return 0, true
	}
	r, ok := singleKeyRune(name)
	if !ok {
		return 0, false
	}
	switch {
	case r >= 'A' && r <= 'Z':
		return byte(r - 'A' + 1), true
	case r >= 'a' && r <= 'z':
		return byte(r - 'a' + 1), true
	}
	switch r {
	case '[':
		return 0x1b, true
	case '\\':
		return 0x1c, true
	case ']':
		return 0x1d, true
	case '^':
		return 0x1e, true
	case '_':
		return 0x1f, true
	case '?':
		return 0x7f, true
	default:
		return 0, false
	}
}

func singleKeyRune(name key.Name) (rune, bool) {
	raw := string(name)
	r, size := utf8.DecodeRuneInString(raw)
	return r, r != utf8.RuneError && size == len(raw)
}

func (ui *UI) drawTerminalGrid(th *material.Theme, gtx layout.Context, st *terminalSession, cellW, cellH int, focused bool) {
	if st == nil || cellW <= 0 || cellH <= 0 {
		return
	}
	if gtx.Constraints.Max.X <= 0 || gtx.Constraints.Max.Y <= 0 {
		return
	}
	gridClip := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	defer gridClip.Pop()

	st.State.Mu.RLock()
	cursorX := st.State.Cursor.X
	cursorY := st.State.Cursor.Y
	cursorVisible := st.State.CursorVisible
	errText := st.State.Err
	viewStart := st.State.ViewStart
	cols := st.State.Cols
	scrollback := st.State.Scrollback
	alternate := st.State.Alternate
	st.State.Mu.RUnlock()
	selectionStart, selectionEnd, selectionActive := st.selectionSnapshot()

	defaultBG := terminalColor(nil, false)
	paint.FillShape(gtx.Ops, defaultBG, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())
	maxRows := gtx.Constraints.Max.Y / cellH
	if gtx.Constraints.Max.Y%cellH != 0 {
		maxRows++
	}
	maxCols := gtx.Constraints.Max.X / cellW
	if cols > 0 && maxCols > cols {
		maxCols = cols
	}
	displayTop, displayY, displayCount := st.displayRows(cellH)
	if maxRows > 0 && displayCount > maxRows+1 {
		displayCount = maxRows + 1
	}

	grid := make([][]TerminalCell, displayCount)
	st.parserMu.Lock()
	for y := 0; y < displayCount; y++ {
		row := make([]TerminalCell, maxCols)
		absoluteRow := displayTop + y
		for x := 0; x < maxCols; x++ {
			if cell, ok := st.virtualCell(absoluteRow, x, scrollback, alternate); ok {
				row[x] = terminalCellFromHeadless(cell)
			}
		}
		grid[y] = row
	}
	st.parserMu.Unlock()

	for y := 0; y < len(grid); y++ {
		row := grid[y]
		absoluteRow := displayTop + y
		rowY := displayY + y*cellH
		if rowY >= gtx.Constraints.Max.Y || rowY+cellH <= 0 {
			continue
		}
		for x := 0; x < len(row); x++ {
			cell := row[x]
			rect := image.Rect(x*cellW, rowY, (x+1)*cellW, rowY+cellH)
			if !sameNRGBA(cell.BG, defaultBG) {
				paint.FillShape(gtx.Ops, cell.BG, clip.Rect(rect).Op())
			}
			if selectionActive && terminalPointSelected(absoluteRow, x, selectionStart, selectionEnd) {
				paint.FillShape(gtx.Ops, terminalSelect, clip.Rect(rect).Op())
			}
			if cell.Rune == 0 || cell.Rune == ' ' {
				continue
			}
			glyphW := terminalGlyphCellWidth(row, x, maxCols) * cellW
			if drawTerminalBoxRune(gtx, rect.Min, glyphW, cellH, cell.Rune, cell.FG) {
				continue
			}
			ui.drawTerminalRune(th, gtx, rect.Min, glyphW, cellH, cell.Rune, cell.FG, cell.Bold)
		}
		rowEndX := len(row) * cellW
		if rowEndX < gtx.Constraints.Max.X && len(row) > 0 {
			bg := row[len(row)-1].BG
			if !sameNRGBA(bg, defaultBG) {
				paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rect(rowEndX, rowY, gtx.Constraints.Max.X, rowY+cellH)).Op())
			}
		}
	}

	cursorAbs := viewStart + cursorY
	cursorDrawY := displayY + (cursorAbs-displayTop)*cellH
	if focused && cursorVisible && cursorX >= 0 && cursorDrawY >= 0 && cursorX < maxCols && cursorDrawY < gtx.Constraints.Max.Y {
		rect := image.Rect(cursorX*cellW, cursorDrawY, (cursorX+1)*cellW, cursorDrawY+cellH)
		paint.FillShape(gtx.Ops, terminalCursor, clip.Rect(rect).Op())
	}
	if strings.TrimSpace(errText) != "" {
		ui.drawTerminalStatus(th, gtx, errText, cellW, cellH)
	}
}

func terminalGlyphCellWidth(row []TerminalCell, x, maxCols int) int {
	if x+1 < len(row) && x+1 < maxCols && row[x+1].Rune == 0 {
		return 2
	}
	return 1
}

func sameNRGBA(a, b color.NRGBA) bool {
	return a.R == b.R && a.G == b.G && a.B == b.B && a.A == b.A
}

func (ui *UI) drawTerminalStatus(th *material.Theme, gtx layout.Context, msg string, cellW, cellH int) {
	if msg == "" || cellW <= 0 || cellH <= 0 {
		return
	}
	maxRunes := gtx.Constraints.Max.X / cellW
	if maxRunes <= 0 {
		return
	}
	y := gtx.Constraints.Max.Y - cellH
	if y < 0 {
		y = 0
	}
	paint.FillShape(gtx.Ops, color.NRGBA{R: 34, G: 12, B: 16, A: 236}, clip.Rect(image.Rect(0, y, gtx.Constraints.Max.X, y+cellH)).Op())
	x := 0
	for _, r := range msg {
		if x >= maxRunes {
			break
		}
		ui.drawTerminalRune(th, gtx, image.Pt(x*cellW, y), cellW, cellH, r, terminalError, false)
		x++
	}
}

func (ui *UI) drawTerminalRune(th *material.Theme, gtx layout.Context, pos image.Point, cellW, cellH int, r rune, fg color.NRGBA, bold bool) {
	if th == nil || th.Shaper == nil || r == 0 {
		return
	}
	weight := font.Normal
	if bold {
		weight = font.Bold
	}
	size := ui.terminalTextSize()
	face := ui.terminalBaseTypeface()
	yOff := uitheme.OpticalTextYOffsetPx(gtx, face, size)
	params := text.Parameters{
		Font: font.Font{
			Typeface: ui.terminalTypeface(),
			Weight:   weight,
		},
		PxPerEm:          fixed.I(gtx.Sp(size)),
		MaxLines:         1,
		MaxWidth:         cellW,
		MinWidth:         cellW,
		Locale:           gtx.Locale,
		LineHeight:       fixed.I(cellH),
		LineHeightScale:  1,
		DisableSpaceTrim: true,
	}
	th.Shaper.LayoutString(params, string(r))

	var glyphArray [8]text.Glyph
	glyphs := glyphArray[:0]
	for g, ok := th.Shaper.NextGlyph(); ok; g, ok = th.Shaper.NextGlyph() {
		if g.Flags&text.FlagParagraphBreak != 0 {
			continue
		}
		glyphs = append(glyphs, g)
		if g.Flags&text.FlagLineBreak != 0 || len(glyphs) == cap(glyphs) {
			drawTerminalGlyphs(gtx, th.Shaper, glyphs, pos, yOff, fg)
			glyphs = glyphs[:0]
		}
	}
	if len(glyphs) > 0 {
		drawTerminalGlyphs(gtx, th.Shaper, glyphs, pos, yOff, fg)
	}
}

type terminalBoxLine uint8

const (
	terminalBoxNone terminalBoxLine = iota
	terminalBoxSingle
	terminalBoxDouble
)

type terminalBoxRuneSpec struct {
	left  terminalBoxLine
	right terminalBoxLine
	up    terminalBoxLine
	down  terminalBoxLine
}

func terminalBoxSpec(r rune) (terminalBoxRuneSpec, bool) {
	s := terminalBoxSingle
	d := terminalBoxDouble
	switch r {
	case '─':
		return terminalBoxRuneSpec{left: s, right: s}, true
	case '│':
		return terminalBoxRuneSpec{up: s, down: s}, true
	case '┌':
		return terminalBoxRuneSpec{right: s, down: s}, true
	case '┐':
		return terminalBoxRuneSpec{left: s, down: s}, true
	case '└':
		return terminalBoxRuneSpec{right: s, up: s}, true
	case '┘':
		return terminalBoxRuneSpec{left: s, up: s}, true
	case '├':
		return terminalBoxRuneSpec{right: s, up: s, down: s}, true
	case '┤':
		return terminalBoxRuneSpec{left: s, up: s, down: s}, true
	case '┬':
		return terminalBoxRuneSpec{left: s, right: s, down: s}, true
	case '┴':
		return terminalBoxRuneSpec{left: s, right: s, up: s}, true
	case '┼':
		return terminalBoxRuneSpec{left: s, right: s, up: s, down: s}, true
	case '═':
		return terminalBoxRuneSpec{left: d, right: d}, true
	case '║':
		return terminalBoxRuneSpec{up: d, down: d}, true
	case '╔':
		return terminalBoxRuneSpec{right: d, down: d}, true
	case '╗':
		return terminalBoxRuneSpec{left: d, down: d}, true
	case '╚':
		return terminalBoxRuneSpec{right: d, up: d}, true
	case '╝':
		return terminalBoxRuneSpec{left: d, up: d}, true
	case '╠':
		return terminalBoxRuneSpec{right: d, up: d, down: d}, true
	case '╣':
		return terminalBoxRuneSpec{left: d, up: d, down: d}, true
	case '╦':
		return terminalBoxRuneSpec{left: d, right: d, down: d}, true
	case '╩':
		return terminalBoxRuneSpec{left: d, right: d, up: d}, true
	case '╬':
		return terminalBoxRuneSpec{left: d, right: d, up: d, down: d}, true
	case '╟':
		return terminalBoxRuneSpec{right: s, up: d, down: d}, true
	case '╢':
		return terminalBoxRuneSpec{left: s, up: d, down: d}, true
	case '╞':
		return terminalBoxRuneSpec{right: d, up: s, down: s}, true
	case '╡':
		return terminalBoxRuneSpec{left: d, up: s, down: s}, true
	case '╤':
		return terminalBoxRuneSpec{left: d, right: d, down: s}, true
	case '╧':
		return terminalBoxRuneSpec{left: d, right: d, up: s}, true
	case '╪':
		return terminalBoxRuneSpec{left: d, right: d, up: s, down: s}, true
	case '╥':
		return terminalBoxRuneSpec{left: s, right: s, down: d}, true
	case '╨':
		return terminalBoxRuneSpec{left: s, right: s, up: d}, true
	case '╫':
		return terminalBoxRuneSpec{left: s, right: s, up: d, down: d}, true
	default:
		return terminalBoxRuneSpec{}, false
	}
}

func drawTerminalBoxRune(gtx layout.Context, pos image.Point, cellW, cellH int, r rune, fg color.NRGBA) bool {
	spec, ok := terminalBoxSpec(r)
	if !ok || cellW <= 0 || cellH <= 0 {
		return ok
	}
	cx := pos.X + cellW/2
	cy := pos.Y + cellH/2
	stroke := terminalBoxStrokePx(cellW, cellH)
	gap := terminalBoxDoubleGapPx(cellW, cellH, stroke)
	drawTerminalBoxHorizontal(gtx, pos.X, cx, cy, stroke, gap, spec.left, fg)
	drawTerminalBoxHorizontal(gtx, cx, pos.X+cellW, cy, stroke, gap, spec.right, fg)
	drawTerminalBoxVertical(gtx, pos.Y, cy, cx, stroke, gap, spec.up, fg)
	drawTerminalBoxVertical(gtx, cy, pos.Y+cellH, cx, stroke, gap, spec.down, fg)
	return true
}

func terminalBoxStrokePx(cellW, cellH int) int {
	stroke := terminalMinInt(cellW, cellH) / 10
	if stroke < 1 {
		stroke = 1
	}
	if stroke > 2 {
		stroke = 2
	}
	return stroke
}

func terminalBoxDoubleGapPx(cellW, cellH, stroke int) int {
	gap := terminalMinInt(cellW, cellH) / 4
	if gap < stroke+2 {
		gap = stroke + 2
	}
	if gap%2 == 0 {
		gap++
	}
	return gap
}

func terminalMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func drawTerminalBoxHorizontal(gtx layout.Context, x0, x1, cy, stroke, gap int, style terminalBoxLine, fg color.NRGBA) {
	if style == terminalBoxNone || x1 <= x0 {
		return
	}
	if style == terminalBoxDouble {
		drawTerminalBoxRect(gtx, image.Rect(x0, cy-gap/2-stroke/2, x1, cy-gap/2-stroke/2+stroke), fg)
		drawTerminalBoxRect(gtx, image.Rect(x0, cy+gap/2-stroke/2, x1, cy+gap/2-stroke/2+stroke), fg)
		return
	}
	drawTerminalBoxRect(gtx, image.Rect(x0, cy-stroke/2, x1, cy-stroke/2+stroke), fg)
}

func drawTerminalBoxVertical(gtx layout.Context, y0, y1, cx, stroke, gap int, style terminalBoxLine, fg color.NRGBA) {
	if style == terminalBoxNone || y1 <= y0 {
		return
	}
	if style == terminalBoxDouble {
		drawTerminalBoxRect(gtx, image.Rect(cx-gap/2-stroke/2, y0, cx-gap/2-stroke/2+stroke, y1), fg)
		drawTerminalBoxRect(gtx, image.Rect(cx+gap/2-stroke/2, y0, cx+gap/2-stroke/2+stroke, y1), fg)
		return
	}
	drawTerminalBoxRect(gtx, image.Rect(cx-stroke/2, y0, cx-stroke/2+stroke, y1), fg)
}

func drawTerminalBoxRect(gtx layout.Context, rect image.Rectangle, fg color.NRGBA) {
	if rect.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, fg, clip.Rect(rect).Op())
}

func drawTerminalGlyphs(gtx layout.Context, shaper *text.Shaper, glyphs []text.Glyph, pos image.Point, yOff int, fg color.NRGBA) {
	if len(glyphs) == 0 {
		return
	}
	lineOff := f32.Pt(float32(pos.X), float32(pos.Y+yOff)).Add(f32.Pt(terminalFixedToFloat(glyphs[0].X), float32(glyphs[0].Y)))
	stack := op.Affine(f32.AffineId().Offset(lineOff)).Push(gtx.Ops)
	path := shaper.Shape(glyphs)
	outline := clip.Outline{Path: path}.Op().Push(gtx.Ops)
	paint.ColorOp{Color: fg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	outline.Pop()
	if call := shaper.Bitmaps(glyphs); call != (op.CallOp{}) {
		call.Add(gtx.Ops)
	}
	stack.Pop()
}

func terminalFixedToFloat(v fixed.Int26_6) float32 {
	return float32(v) / 64
}
