// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"hexone/fm"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	headlessterm "github.com/danielgatis/go-headless-term"
)

type terminalSnippetContext struct {
	directory         string
	directoryDisplay  string
	repository        string
	repositoryDisplay string
}

type terminalSnippetMenuItem struct {
	configIndex int
	snippet     fm.TerminalSnippet
}

type terminalSnippetEditorState struct {
	nameEdit    widget.Editor
	commandEdit widget.Editor
	scope       string
	context     terminalSnippetContext
	wantFocus   bool
	lastErr     string

	scopeClicks   [3]widget.Clickable
	cancelClick   widget.Clickable
	saveClick     widget.Clickable
	closeClick    widget.Clickable
	backdropClick widget.Clickable
	scopeAnim     segmentedAnimState
}

func (ui *UI) toggleTerminalSnippetMenu(now time.Time) {
	if ui == nil {
		return
	}
	if ui.terminalSnippetEditor != nil {
		return
	}
	ui.terminalSnippetMenuOpen = !ui.terminalSnippetMenuOpen
	if ui.terminalSnippetMenuOpen {
		ui.terminalSnippetMenuOpenedAt = now
		ui.terminalSnippetMenuSelected = 0
		ui.closeCustomCommandMenu()
		ui.closeFunctionBarToolsMenu()
		return
	}
	ui.closeTerminalSnippetMenu()
}

func (ui *UI) closeTerminalSnippetMenu() {
	if ui == nil {
		return
	}
	ui.terminalSnippetMenuOpen = false
	ui.terminalSnippetMenuOpenedAt = time.Time{}
	ui.terminalSnippetMenuSelected = -1
	ui.terminalSnippetMenuHoverID = ""
	ui.terminalSnippetMenuHoverAnim = segmentedAnimState{}
}

func (ui *UI) terminalSnippetContext() terminalSnippetContext {
	if ui == nil || ui.terminal == nil {
		return terminalSnippetContext{}
	}
	st := ui.terminal
	if loc, ok := st.osc7Location(); ok {
		if terminalOSC7HostIsLocal(loc.Host) {
			dir := filepath.Clean(terminalOSC7LocalDir(loc.Dir))
			return localTerminalSnippetContext(dir)
		}
		dir := strings.TrimSpace(loc.Dir)
		if dir == "" {
			return terminalSnippetContext{}
		}
		host := strings.ToLower(strings.TrimSpace(loc.Host))
		port := loc.Port
		if port <= 0 {
			port = 22
		}
		user := strings.TrimSpace(loc.User)
		identity := host
		if user != "" {
			identity = user + "@" + identity
		}
		key := fmt.Sprintf("ssh://%s:%d%s", identity, port, dir)
		return terminalSnippetContext{
			directory:        key,
			directoryDisplay: identity + ":" + dir,
		}
	}
	if dir, ok := st.currentDir(); ok {
		return localTerminalSnippetContext(dir)
	}
	return terminalSnippetContext{}
}

func localTerminalSnippetContext(dir string) terminalSnippetContext {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" || dir == "." {
		return terminalSnippetContext{}
	}
	ctx := terminalSnippetContext{
		directory:        terminalSnippetLocalKey(dir),
		directoryDisplay: dir,
	}
	if root, ok := findTerminalSnippetRepository(dir); ok {
		ctx.repository = terminalSnippetLocalKey(root)
		ctx.repositoryDisplay = root
	}
	return ctx
}

func terminalSnippetLocalKey(path string) string {
	clean := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		clean = strings.ToLower(clean)
	}
	return clean
}

func findTerminalSnippetRepository(dir string) (string, bool) {
	dir = filepath.Clean(strings.TrimSpace(dir))
	for dir != "" {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

func (ui *UI) applicableTerminalSnippets() []terminalSnippetMenuItem {
	if ui == nil || ui.fmCfg == nil {
		return nil
	}
	ctx := ui.terminalSnippetContext()
	items := make([]terminalSnippetMenuItem, 0, len(ui.fmCfg.TerminalSnippets))
	for i, raw := range ui.fmCfg.TerminalSnippets {
		snippet, ok := fm.NormalizeTerminalSnippet(raw)
		if !ok {
			continue
		}
		matches := snippet.Scope == fm.TerminalSnippetScopeGlobal ||
			(snippet.Scope == fm.TerminalSnippetScopeDirectory && ctx.directory != "" && snippet.Context == ctx.directory) ||
			(snippet.Scope == fm.TerminalSnippetScopeRepository && ctx.repository != "" && snippet.Context == ctx.repository)
		if matches {
			items = append(items, terminalSnippetMenuItem{configIndex: i, snippet: snippet})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := terminalSnippetScopeRank(items[i].snippet.Scope)
		right := terminalSnippetScopeRank(items[j].snippet.Scope)
		if left != right {
			return left < right
		}
		return strings.ToLower(items[i].snippet.Name) < strings.ToLower(items[j].snippet.Name)
	})
	return items
}

func terminalSnippetScopeRank(scope string) int {
	switch scope {
	case fm.TerminalSnippetScopeRepository:
		return 0
	case fm.TerminalSnippetScopeDirectory:
		return 1
	default:
		return 2
	}
}

func ensureTerminalSnippetClickables(clicks *[]widget.Clickable, n int) {
	if n <= cap(*clicks) {
		*clicks = (*clicks)[:n]
		return
	}
	old := *clicks
	*clicks = make([]widget.Clickable, n)
	copy(*clicks, old)
}

func (ui *UI) insertTerminalSnippet(snippet fm.TerminalSnippet) bool {
	if ui == nil || ui.terminal == nil {
		return false
	}
	normalized, ok := fm.NormalizeTerminalSnippet(snippet)
	if !ok {
		return false
	}
	ui.terminal.writeString(normalized.Command)
	ui.terminal.focusKeyboard()
	ui.closeTerminalSnippetMenu()
	return true
}

func (ui *UI) removeTerminalSnippet(index int) bool {
	if ui == nil || ui.fmCfg == nil || index < 0 || index >= len(ui.fmCfg.TerminalSnippets) {
		return false
	}
	previous := append([]fm.TerminalSnippet(nil), ui.fmCfg.TerminalSnippets...)
	ui.fmCfg.TerminalSnippets = append(ui.fmCfg.TerminalSnippets[:index], ui.fmCfg.TerminalSnippets[index+1:]...)
	if err := ui.saveFMConfigWithOptions("terminal-snippets", false); err != nil {
		ui.fmCfg.TerminalSnippets = previous
		if ui.terminal != nil {
			ui.terminal.setError("save snippets failed: " + err.Error())
		}
		return false
	}
	return true
}

func (ui *UI) openTerminalSnippetEditor() {
	if ui == nil {
		return
	}
	ctx := ui.terminalSnippetContext()
	scope := fm.TerminalSnippetScopeGlobal
	if ctx.repository != "" {
		scope = fm.TerminalSnippetScopeRepository
	} else if ctx.directory != "" {
		scope = fm.TerminalSnippetScopeDirectory
	}
	st := &terminalSnippetEditorState{
		scope:     scope,
		context:   ctx,
		wantFocus: true,
	}
	st.nameEdit.SingleLine = true
	st.nameEdit.Submit = true
	st.commandEdit.SingleLine = true
	st.commandEdit.Submit = true
	st.commandEdit.SetText(ui.terminal.currentCommandDraft())
	ui.terminalSnippetEditor = st
	ui.closeTerminalSnippetMenu()
}

func (ui *UI) closeTerminalSnippetEditor() {
	if ui == nil {
		return
	}
	ui.terminalSnippetEditor = nil
	if ui.terminal != nil {
		ui.terminal.focusKeyboard()
	}
}

func (st *terminalSnippetEditorState) selectedContext() (string, string, bool) {
	if st == nil {
		return "", "", false
	}
	switch st.scope {
	case fm.TerminalSnippetScopeGlobal:
		return "", "Available in every terminal", true
	case fm.TerminalSnippetScopeDirectory:
		return st.context.directory, st.context.directoryDisplay, st.context.directory != ""
	case fm.TerminalSnippetScopeRepository:
		return st.context.repository, st.context.repositoryDisplay, st.context.repository != ""
	default:
		return "", "", false
	}
}

func (ui *UI) saveTerminalSnippetEditor() bool {
	if ui == nil || ui.fmCfg == nil || ui.terminalSnippetEditor == nil {
		return false
	}
	st := ui.terminalSnippetEditor
	context, _, available := st.selectedContext()
	if !available {
		st.lastErr = "The selected scope is unavailable for this terminal location."
		return false
	}
	snippet, ok := fm.NormalizeTerminalSnippet(fm.TerminalSnippet{
		Name:    st.nameEdit.Text(),
		Command: st.commandEdit.Text(),
		Scope:   st.scope,
		Context: context,
	})
	if !ok {
		st.lastErr = "Enter a command to save."
		return false
	}
	previous := append([]fm.TerminalSnippet(nil), ui.fmCfg.TerminalSnippets...)
	ui.fmCfg.TerminalSnippets = fm.NormalizeTerminalSnippets(append(ui.fmCfg.TerminalSnippets, snippet))
	if err := ui.saveFMConfigWithOptions("terminal-snippets", false); err != nil {
		ui.fmCfg.TerminalSnippets = previous
		st.lastErr = "Save failed: " + err.Error()
		return false
	}
	ui.closeTerminalSnippetEditor()
	return true
}

func (s *terminalSession) currentCommandDraft() string {
	if s == nil || s.term == nil {
		return ""
	}
	if selected := strings.TrimSpace(s.selectedText(false)); selected != "" && !strings.Contains(selected, "\n") {
		return selected
	}
	s.parserMu.Lock()
	defer s.parserMu.Unlock()
	if s.term.IsAlternateScreen() {
		return ""
	}
	row, col := s.term.CursorPos()
	rows, cols := s.term.Rows(), s.term.Cols()
	if row < 0 || row >= rows || cols <= 0 {
		return ""
	}
	if col < 0 {
		col = 0
	}
	if col > cols {
		col = cols
	}
	parts := []string{terminalScreenLineText(s.term, row, col)}
	for previous := row - 1; previous >= 0 && len(parts) < 8; previous-- {
		line := terminalScreenLineText(s.term, previous, cols)
		if terminalScreenLineLastColumnBlank(s.term, previous, cols) {
			break
		}
		parts = append([]string{line}, parts...)
	}
	return stripTerminalPrompt(strings.Join(parts, ""))
}

func terminalScreenLineText(term *headlessterm.Terminal, row, end int) string {
	if term == nil || row < 0 || end <= 0 {
		return ""
	}
	var b strings.Builder
	for col := 0; col < end; col++ {
		cell := term.Cell(row, col)
		if cell == nil {
			continue
		}
		r := terminalCellFromHeadless(*cell).Rune
		if r != 0 {
			b.WriteRune(r)
		}
	}
	return strings.TrimRight(b.String(), " ")
}

func terminalScreenLineLastColumnBlank(term *headlessterm.Terminal, row, cols int) bool {
	if term == nil || row < 0 || cols <= 0 {
		return true
	}
	cell := term.Cell(row, cols-1)
	if cell == nil {
		return true
	}
	r := terminalCellFromHeadless(*cell).Rune
	return r == 0 || r == ' '
}

func stripTerminalPrompt(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	best := -1
	for _, marker := range []string{"❯ ", "➜ ", "λ ", "$ ", "# ", "% ", "> "} {
		if index := terminalPromptMarkerIndex(line, marker); index >= 0 {
			end := index + len(marker)
			if best < 0 || end < best {
				best = end
			}
		}
	}
	if best >= 0 && best <= len(line) {
		line = line[best:]
	}
	return strings.TrimSpace(line)
}

func terminalPromptMarkerIndex(line, marker string) int {
	index := strings.Index(line, marker)
	if index < 0 || index > 96 {
		return -1
	}
	if marker == "❯ " || marker == "➜ " || marker == "λ " || index == 0 {
		return index
	}
	prefix := line[:index]
	if strings.ContainsAny(prefix, "@:/\\~[]()") {
		return index
	}
	if marker == "> " && strings.HasPrefix(strings.ToLower(strings.TrimSpace(prefix)), "ps ") {
		return index
	}
	return -1
}

func (ui *UI) handleTerminalSnippetMenuKeys(gtx layout.Context) {
	if ui == nil || !ui.terminalSnippetMenuOpen {
		return
	}
	items := ui.applicableTerminalSnippets()
	count := len(items) + 1
	if count < 1 {
		count = 1
	}
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape},
			key.Filter{Name: key.NameUpArrow},
			key.Filter{Name: key.NameDownArrow},
			key.Filter{Name: key.NameEnter},
			key.Filter{Name: key.NameReturn},
		)
		if !ok {
			return
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press || ke.Modifiers != 0 {
			continue
		}
		switch ke.Name {
		case key.NameEscape:
			ui.closeTerminalSnippetMenu()
		case key.NameUpArrow:
			ui.terminalSnippetMenuSelected--
			if ui.terminalSnippetMenuSelected < 0 {
				ui.terminalSnippetMenuSelected = count - 1
			}
		case key.NameDownArrow:
			ui.terminalSnippetMenuSelected++
			if ui.terminalSnippetMenuSelected >= count {
				ui.terminalSnippetMenuSelected = 0
			}
		case key.NameEnter, key.NameReturn:
			if ui.terminalSnippetMenuSelected <= 0 {
				ui.openTerminalSnippetEditor()
			} else if index := ui.terminalSnippetMenuSelected - 1; index >= 0 && index < len(items) {
				ui.insertTerminalSnippet(items[index].snippet)
			}
		}
		gtx.Execute(op.InvalidateCmd{})
		return
	}
}

func (ui *UI) handleTerminalSnippetShortcut(gtx layout.Context) {
	if ui == nil || ui.terminal == nil || ui.terminalSnippetEditor != nil {
		return
	}
	anyMods := ^key.Modifiers(0)
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &ui.terminal.keyTag, Name: "P", Required: key.ModShortcut | key.ModShift, Optional: anyMods},
			key.Filter{Focus: &ui.terminal.keyTag, Name: "p", Required: key.ModShortcut | key.ModShift, Optional: anyMods},
		)
		if !ok {
			return
		}
		ke, ok := ev.(key.Event)
		if ok && ke.State == key.Press {
			ui.toggleTerminalSnippetMenu(gtx.Now)
			gtx.Execute(op.InvalidateCmd{})
			return
		}
	}
}

func (ui *UI) handleTerminalSnippetEditorPreLayoutInput(gtx layout.Context) {
	st := ui.terminalSnippetEditor
	if ui == nil || st == nil {
		return
	}
	for _, editor := range []*widget.Editor{&st.nameEdit, &st.commandEdit} {
		for {
			ev, ok := editor.Update(gtx)
			if !ok {
				break
			}
			if _, ok := ev.(widget.ChangeEvent); ok {
				st.lastErr = ""
			}
		}
	}
	anyMods := ^key.Modifiers(0)
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape, Optional: anyMods},
			key.Filter{Name: "S", Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "s", Required: key.ModShortcut, Optional: anyMods},
		)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		if ke.Name == key.NameEscape {
			ui.closeTerminalSnippetEditor()
		} else {
			ui.saveTerminalSnippetEditor()
		}
		gtx.Execute(op.InvalidateCmd{})
		return
	}
}

func (ui *UI) layoutTerminalSnippetMenuPopup(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if ui == nil || !ui.terminalSnippetMenuOpen {
		return layout.Dimensions{}
	}
	ui.handleTerminalSnippetMenuKeys(gtx)
	if !ui.terminalSnippetMenuOpen {
		return layout.Dimensions{}
	}
	items := ui.applicableTerminalSnippets()
	ensureTerminalSnippetClickables(&ui.terminalSnippetMenuClicks, len(items)+1)
	ensureTerminalSnippetClickables(&ui.terminalSnippetRemoveClicks, len(items))
	if ui.terminalSnippetMenuClicks[0].Clicked(gtx) {
		ui.openTerminalSnippetEditor()
		gtx.Execute(op.InvalidateCmd{})
		return layout.Dimensions{}
	}
	for i := range items {
		if ui.terminalSnippetRemoveClicks[i].Clicked(gtx) {
			ui.removeTerminalSnippet(items[i].configIndex)
			gtx.Execute(op.InvalidateCmd{})
			return layout.Dimensions{}
		}
		if ui.terminalSnippetMenuClicks[i+1].Clicked(gtx) {
			ui.insertTerminalSnippet(items[i].snippet)
			gtx.Execute(op.InvalidateCmd{})
			return layout.Dimensions{}
		}
	}
	if ui.terminalSnippetMenuSelected < 0 || ui.terminalSnippetMenuSelected > len(items) {
		ui.terminalSnippetMenuSelected = 0
	}

	alpha, slideY, animating := popupOpenProgress(gtx.Now, ui.terminalSnippetMenuOpenedAt)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	block := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, &ui.terminalSnippetMenuTag)
	block.Pop()
	macro := op.Record(gtx.Ops)
	card := ui.layoutTerminalSnippetMenuCard(th, gtx, items, alpha)
	call := macro.Stop()
	margin := gtx.Dp(unit.Dp(10))
	anchorY := gtx.Constraints.Max.Y - card.Size.Y - gtx.Dp(unit.Dp(30)) - slideY
	if ui.terminal != nil {
		if paneHeight, _, ok := ui.terminal.paneMetrics(); ok {
			anchorY = gtx.Constraints.Max.Y - paneHeight + gtx.Dp(unit.Dp(4+tabStripHeightDp+3)) + slideY
		}
	}
	anchor := image.Pt(
		gtx.Constraints.Max.X-card.Size.X-margin,
		anchorY,
	)
	anchor = clampFilePaneMenuPoint(anchor, card.Size, gtx.Constraints.Max)
	offset := op.Offset(anchor).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	ui.handleTerminalSnippetMenuOutsideClick(gtx)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (ui *UI) handleTerminalSnippetMenuOutsideClick(gtx layout.Context) {
	if ui == nil || !ui.terminalSnippetMenuOpen {
		return
	}
	pressedBody := popupPressed(gtx, &ui.terminalSnippetMenuBodyTag)
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &ui.terminalSnippetMenuTag, Kinds: pointer.Press})
		if !ok {
			return
		}
		pe, ok := ev.(pointer.Event)
		if ok && pe.Kind == pointer.Press && pe.Buttons.Contain(pointer.ButtonPrimary) && !pressedBody {
			ui.closeTerminalSnippetMenu()
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (ui *UI) layoutTerminalSnippetMenuCard(th *material.Theme, gtx layout.Context, items []terminalSnippetMenuItem, alpha float32) layout.Dimensions {
	width := ui.filePaneFavoriteMenuWidth(gtx)
	hoverID := ""
	if len(ui.terminalSnippetMenuClicks) > 0 && ui.terminalSnippetMenuClicks[0].Hovered() {
		hoverID = "snippet-save-current"
	} else {
		for i := range items {
			if i+1 < len(ui.terminalSnippetMenuClicks) && ui.terminalSnippetMenuClicks[i+1].Hovered() {
				hoverID = fmt.Sprintf("snippet:%d", items[i].configIndex)
				break
			}
		}
	}
	if hoverID != ui.terminalSnippetMenuHoverID {
		ui.terminalSnippetMenuHoverID = hoverID
		ui.terminalSnippetMenuHoverAnim.setHover(hoverID, gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
	theme := ui.filePanePopupTheme()
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		dims := fillRoundedClipBox(gtx, gtx.Dp(unit.Dp(filePaneOverlayCornerDp)), scaleColorAlpha(theme.Bg, alpha), scaleColorAlpha(theme.Border, alpha), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, ui.filePaneFavoriteMenuTitleHeight(gtx), func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(3), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Caption(th, "Snippets")
							lbl.Font.Typeface = ui.interfaceTypeface()
							lbl.Font.Weight = font.Medium
							lbl.TextSize = ui.scaleInterfaceFontSize(9)
							lbl.Color = scaleColorAlpha(theme.Title, alpha)
							lbl.MaxLines = 1
							lbl.Truncator = "…"
							return layoutVCenteredLabel(gtx, lbl)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					hoverFill, animating := ui.terminalSnippetMenuHoverAnim.hoverFill(gtx.Now, "snippet-save-current")
					if hoverID == "" && ui.terminalSnippetMenuSelected == 0 && hoverFill < 1 {
						hoverFill = 1
					}
					if animating {
						gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
					}
					item := fileFavoriteItem{label: "Save current command…", addCurrent: true}
					return ui.layoutFilePaneFavoriteMenuItem(th, gtx, theme, &ui.terminalSnippetMenuClicks[0], nil, item, hoverFill, alpha, 0)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutTerminalSnippetMenuSeparator(gtx, theme, alpha)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if len(items) == 0 {
						item := fileFavoriteItem{label: "No snippets here", disabled: true}
						return ui.layoutFilePaneFavoriteMenuItem(th, gtx, theme, nil, nil, item, 0, alpha, 0)
					}
					rowH := ui.filePaneFavoriteMenuRowHeight(gtx)
					maxRows := len(items)
					if maxRows > 12 {
						maxRows = 12
					}
					ui.terminalSnippetList.Axis = layout.Vertical
					return fixedHeight(gtx, rowH*maxRows, func(gtx layout.Context) layout.Dimensions {
						return ui.terminalSnippetList.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
							item := items[index]
							id := fmt.Sprintf("snippet:%d", item.configIndex)
							hoverFill, animating := ui.terminalSnippetMenuHoverAnim.hoverFill(gtx.Now, id)
							if hoverID == "" && ui.terminalSnippetMenuSelected == index+1 && hoverFill < 1 {
								hoverFill = 1
							}
							if animating {
								gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
							}
							favorite := fileFavoriteItem{label: item.snippet.Name, removable: true}
							return ui.layoutFilePaneFavoriteMenuItem(th, gtx, theme, &ui.terminalSnippetMenuClicks[index+1], &ui.terminalSnippetRemoveClicks[index], favorite, hoverFill, alpha, 0)
						})
					})
				}),
			)
		})
		registerPopupArea(gtx, &ui.terminalSnippetMenuBodyTag, dims.Size)
		return dims
	})
}

func (ui *UI) layoutTerminalSnippetMenuSeparator(gtx layout.Context, theme filePanePopupTheme, alpha float32) layout.Dimensions {
	sepH := ui.filePaneFavoriteMenuSeparatorHeight(gtx)
	return fixedHeight(gtx, sepH, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			h := gtx.Dp(unit.Dp(1))
			if h < 1 {
				h = 1
			}
			return fillBgExact(gtx, scaleColorAlpha(theme.Divider, alpha), func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
			})
		})
	})
}

func (ui *UI) layoutTerminalSnippetEditor(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.terminalSnippetEditor
	if ui == nil || st == nil {
		return layout.Dimensions{}
	}
	for i, scope := range []string{fm.TerminalSnippetScopeGlobal, fm.TerminalSnippetScopeDirectory, fm.TerminalSnippetScopeRepository} {
		if st.scopeClicks[i].Clicked(gtx) {
			_, _, available := (&terminalSnippetEditorState{scope: scope, context: st.context}).selectedContext()
			if available {
				st.scope = scope
				st.lastErr = ""
			}
		}
	}
	if st.cancelClick.Clicked(gtx) || st.closeClick.Clicked(gtx) {
		ui.closeTerminalSnippetEditor()
		return layout.Dimensions{}
	}
	if st.saveClick.Clicked(gtx) {
		ui.saveTerminalSnippetEditor()
		if ui.terminalSnippetEditor == nil {
			return layout.Dimensions{}
		}
	}
	for st.backdropClick.Clicked(gtx) {
	}
	return st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{A: 130}, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())
		width := gtx.Dp(unit.Dp(500))
		if width > gtx.Constraints.Max.X-gtx.Dp(unit.Dp(20)) {
			width = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(20))
		}
		macro := op.Record(gtx.Ops)
		dialog := fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
			return fillRoundedBox(gtx, gtx.Dp(unit.Dp(3)), color.NRGBA{R: 20, G: 20, B: 20, A: 252}, color.NRGBA{R: 255, G: 255, B: 255, A: 18}, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(7)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutTerminalSnippetEditorBody(th, gtx, st)
				})
			})
		})
		call := macro.Stop()
		position := image.Pt((gtx.Constraints.Max.X-dialog.Size.X)/2, (gtx.Constraints.Max.Y-dialog.Size.Y)/2)
		offset := op.Offset(position).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
}

func (ui *UI) layoutTerminalSnippetEditorBody(th *material.Theme, gtx layout.Context, st *terminalSnippetEditorState) layout.Dimensions {
	_, contextDisplay, scopeAvailable := st.selectedContext()
	editor := func(id, hint string, ed *widget.Editor, height unit.Dp) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			style := material.Editor(th, ed, hint)
			style.Font.Typeface = ui.viewerTypeface()
			style.TextSize = ui.scaleDialogFontSize(10)
			style.Color = txtColor
			style.HintColor = hintColor
			if st.wantFocus && ed == &st.nameEdit {
				st.wantFocus = false
				gtx.Execute(key.FocusCmd{Tag: ed})
			}
			return ui.layoutEditorWithContextMenu(th, gtx, id, ed, true, func(gtx layout.Context) layout.Dimensions {
				return layoutNeutralEditorBox(gtx, gtx.Focused(ed), true, func(gtx layout.Context) layout.Dimensions {
					if height > 0 {
						return fixedHeight(gtx, gtx.Dp(height), style.Layout)
					}
					return style.Layout(gtx)
				})
			})
		}
	}
	label := func(text string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, text)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleDialogFontSize(9)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}
	}
	hoverCancel := float32(0)
	if st.cancelClick.Hovered() {
		hoverCancel = 1
	}
	hoverSave := float32(0)
	if st.saveClick.Hovered() {
		hoverSave = 1
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					title := material.Body1(th, "Save Terminal Snippet")
					title.Font.Typeface = ui.interfaceTypeface()
					title.Font.Weight = font.Bold
					title.TextSize = ui.scaleDialogFontSize(10.5)
					title.Color = txtColor
					return title.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutFlatCloseButton(gtx, &st.closeClick, false)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Rigid(layoutDialogHorizontalDivider),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(label("Name")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout),
		layout.Rigid(editor("terminal-snippet-name", "Deploy staging", &st.nameEdit, 0)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(label("Command")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout),
		layout.Rigid(editor("terminal-snippet-command", "go test ./...", &st.commandEdit, 0)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(label("Scope")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			active := 0
			if st.scope == fm.TerminalSnippetScopeDirectory {
				active = 1
			} else if st.scope == fm.TerminalSnippetScopeRepository {
				active = 2
			}
			return ui.layoutMultiRenameTabs(th, gtx, []string{"Global", "Directory", "Git repository"}, st.scopeClicks[:], active, &st.scopeAnim, "terminal-snippet-scope", false, false, st.context.directory == "", st.context.repository == "")
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if contextDisplay == "" {
				contextDisplay = "Unavailable for the current terminal"
			}
			lbl := material.Caption(th, contextDisplay)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleDialogFontSize(8)
			lbl.Color = hintColor
			lbl.MaxLines = 1
			lbl.Truncator = "…"
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if st.lastErr == "" {
				return layout.Dimensions{}
			}
			lbl := material.Caption(th, st.lastErr)
			lbl.Font.Typeface = ui.interfaceTypeface()
			lbl.TextSize = ui.scaleDialogFontSize(9)
			lbl.Color = color.NRGBA{R: 220, G: 140, B: 140, A: 255}
			return layout.Inset{Top: unit.Dp(3)}.Layout(gtx, lbl.Layout)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(layoutDialogHorizontalDivider),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutDialogActionPair(
					th, gtx,
					&st.cancelClick, "Cancel", hoverCancel, 0, false,
					&st.saveClick, "Save", hoverSave, 0, !scopeAvailable || strings.TrimSpace(st.commandEdit.Text()) == "",
					dialogActionVisualState{},
					dialogActionVisualState{Default: true},
				)
			})
		}),
	)
}
