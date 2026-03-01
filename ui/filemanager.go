package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/widget/table"
	"image/color"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
)

type filePaneModel struct {
	entries []filesys.Entry
	cfg     *fm.Config
}

type fileSortKey uint8

const (
	fileSortName fileSortKey = iota
	fileSortExt
	fileSortSize
	fileSortDate
)

func (m *filePaneModel) Len() int {
	if m == nil {
		return 0
	}
	return len(m.entries)
}

func (m *filePaneModel) Entry(row int) *filesys.Entry {
	if m == nil || row < 0 || row >= len(m.entries) {
		return nil
	}
	return &m.entries[row]
}

func (m *filePaneModel) Cell(r, c int) (string, table.CellStyle) {
	entry := m.entries[r]
	st := table.CellStyle{Color: txtColor, Weight: font.Medium}

	switch entry.Kind {
	case filesys.EntryDir, filesys.EntryParent:
		st.Color = color.NRGBA{R: 170, G: 200, B: 255, A: 255}
	case filesys.EntryBroken:
		st.Color = color.NRGBA{R: 255, G: 120, B: 120, A: 255}
		st.Weight = font.Bold
	}

	switch c {
	case 0:
		return entry.DisplayName, st
	case 1:
		return entry.SizeText, st
	default:
		return entry.DateText, st
	}
}

func (m *filePaneModel) CellWithWidth(r, c, widthPx int) (string, table.CellStyle) {
	txt, st := m.Cell(r, c)
	entry := m.entries[r]

	switch c {
	case 0:
		return m.nameOrEmpty(entry.DisplayName, widthPx), st
	case 1:
		return m.formatSize(entry.SizeText, widthPx), st
	case 2:
		return m.formatDate(entry, widthPx), st
	default:
		return txt, st
	}
}

type filePaneState struct {
	table          *table.Table
	model          *filePaneModel
	headerClick    widget.Clickable
	modeClick      widget.Clickable
	sortClick      widget.Clickable
	sortPointerTag struct{}
	sortOptionBtns [4]widget.Clickable
	sortMenuOpen   bool
	sortKey        fileSortKey
	sortDesc       bool
	dirsFirst      bool
	dir            string
	err            string
}

func newFilePaneState(dir string, cfg *fm.Config) *filePaneState {
	if cfg == nil {
		cfg = fm.DefaultConfig()
	}
	cols := []table.Column{
		{Width: unit.Dp(cfg.Columns.NameWidthDp), MinWidth: unit.Dp(cfg.Columns.NameMinWidthDp), Flex: true, Align: table.AlignStart, PadX: unit.Dp(2)},
		{Width: unit.Dp(cfg.Columns.SizeWidthDp), MinWidth: unit.Dp(cfg.Columns.SizeMinWidthDp), Flex: false, Align: table.AlignEnd, PadX: unit.Dp(2)},
		{Width: unit.Dp(cfg.Columns.DateWidthDp), MinWidth: unit.Dp(cfg.Columns.DateMinWidthDp), Flex: false, Align: table.AlignStart, PadX: unit.Dp(6)},
	}

	pane := &filePaneState{
		table:     table.New(cols),
		model:     &filePaneModel{cfg: cfg},
		sortKey:   parseFileSortKey(cfg.Sort.DefaultKey),
		sortDesc:  cfg.Sort.Descending,
		dirsFirst: cfg.Sort.DirectoriesFirst,
	}
	pane.table.TextSize = unit.Sp(13)
	pane.table.RowHeight = unit.Dp(18)
	pane.table.RowPadY = unit.Dp(0)
	pane.table.BriefColumnWidth = unit.Dp(cfg.Columns.BriefWidthDp)
	pane.table.BriefGap = unit.Dp(cfg.Columns.BriefGapDp)
	pane.table.SelectedFg = &color.NRGBA{R: 230, G: 230, B: 255, A: 255}
	_ = pane.load(dir)
	return pane
}

func (p *filePaneState) load(dir string) error {
	listing, err := filesys.ReadDir(dir)
	if err != nil {
		p.dir = dir
		p.err = err.Error()
		p.model.entries = nil
		return err
	}

	p.dir = listing.Dir
	p.err = ""
	p.model.entries = listing.Entries
	p.applySort("")
	p.table.Selected = 0
	p.table.List.Position = layout.Position{}
	return nil
}

func (p *filePaneState) selectedEntry() *filesys.Entry {
	if p == nil || p.table == nil || p.model == nil {
		return nil
	}
	return p.model.Entry(p.table.Selected)
}

func (p *filePaneState) findEntryIndex(name string) int {
	if p == nil || p.model == nil {
		return -1
	}
	for i := range p.model.entries {
		if p.model.entries[i].Name == name {
			return i
		}
	}
	return -1
}

func (p *filePaneState) findEntryPathIndex(path string) int {
	if p == nil || p.model == nil {
		return -1
	}
	for i := range p.model.entries {
		if p.model.entries[i].Path == path {
			return i
		}
	}
	return -1
}

func (m *filePaneModel) approxCharPx() int {
	if m == nil || m.cfg == nil || m.cfg.NameCompact.ApproxCharPx < 1 {
		return 7
	}
	return m.cfg.NameCompact.ApproxCharPx
}

func (m *filePaneModel) approxChars(widthPx, reservePx int) int {
	if widthPx <= reservePx {
		return 0
	}
	return (widthPx - reservePx) / m.approxCharPx()
}

func (m *filePaneModel) nameOrEmpty(text string, widthPx int) string {
	if m == nil {
		return text
	}
	capacity := m.approxChars(widthPx, m.approxCharPx()/2+2)
	if capacity >= utf8.RuneCountInString(text) {
		return text
	}
	return m.compactName(text, capacity)
}

func (m *filePaneModel) compactName(text string, capacity int) string {
	if m == nil {
		return text
	}
	runes := []rune(text)
	if len(runes) <= capacity {
		return text
	}
	if capacity < 3 {
		return ""
	}

	marker := ".."
	headMin := 6
	tailMin := 3
	if m.cfg != nil {
		if m.cfg.NameCompact.Marker != "" {
			marker = m.cfg.NameCompact.Marker
		}
		if m.cfg.NameCompact.MinHead > 0 {
			headMin = m.cfg.NameCompact.MinHead
		}
		if m.cfg.NameCompact.MinTail > 0 {
			tailMin = m.cfg.NameCompact.MinTail
		}
	}

	markerRunes := utf8.RuneCountInString(marker)
	available := capacity - markerRunes
	if available < 2 {
		return ""
	}

	tail := m.preferredNameTail(runes, tailMin)
	if tail > len(runes)-1 {
		tail = len(runes) - 1
	}
	if tail < 1 {
		tail = 1
	}

	head := available - tail
	if head < 1 {
		head = 1
		tail = available - head
	}
	if tail < 1 {
		return ""
	}

	if head > headMin && head+tail < len(runes) {
		// Keep a little more of the tail (often extension or trailing slash) when space allows.
		extra := head - headMin
		room := len(runes) - head - tail
		if room > 0 && extra > 0 {
			if extra > room {
				extra = room
			}
			head -= extra
			tail += extra
		}
	}

	if head+tail >= len(runes) {
		return text
	}
	return string(runes[:head]) + marker + string(runes[len(runes)-tail:])
}

func (m *filePaneModel) preferredNameTail(runes []rune, fallback int) int {
	if len(runes) == 0 {
		return fallback
	}
	if fallback < 1 {
		fallback = 1
	}

	if runes[len(runes)-1] == '/' {
		if fallback < 2 && len(runes) > 1 {
			return 2
		}
		return fallback
	}

	lastDot := -1
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] == '.' {
			lastDot = i
			break
		}
	}
	if lastDot > 0 && lastDot < len(runes)-1 {
		extLen := len(runes) - lastDot
		if extLen > fallback {
			return extLen
		}
	}
	return fallback
}

func (m *filePaneModel) fullOrEmpty(text string, widthPx int) string {
	if m == nil {
		return text
	}
	capacity := m.approxChars(widthPx, 4)
	if capacity < utf8.RuneCountInString(text) {
		return ""
	}
	return text
}

func (m *filePaneModel) formatSize(text string, widthPx int) string {
	if text == "" {
		return ""
	}

	candidates := sizeCandidates(text)

	for _, candidate := range candidates {
		if m.fullOrEmpty(candidate, widthPx) != "" {
			return candidate
		}
	}
	return ""
}

func sizeCandidates(text string) []string {
	out := make([]string, 0, 4)
	appendUnique := func(v string) {
		if v == "" {
			return
		}
		for _, existing := range out {
			if existing == v {
				return
			}
		}
		out = append(out, v)
	}

	appendUnique(text)

	compact := strings.ReplaceAll(text, " ", "")
	appendUnique(compact)

	noByte := trimSizeByteSuffix(compact)
	appendUnique(noByte)
	appendUnique(dropFraction(noByte))

	return out
}

func trimSizeByteSuffix(text string) string {
	if len(text) < 2 {
		return text
	}
	last := text[len(text)-1]
	if last != 'B' && last != 'b' {
		return text
	}
	prev := text[len(text)-2]
	if prev < 'A' || prev > 'Z' {
		return text
	}
	return text[:len(text)-1]
}

func dropFraction(text string) string {
	dot := strings.IndexByte(text, '.')
	if dot <= 0 {
		return text
	}
	end := dot + 1
	for end < len(text) && text[end] >= '0' && text[end] <= '9' {
		end++
	}
	if end == dot+1 {
		return text
	}
	return text[:dot] + text[end:]
}

func (m *filePaneModel) formatDate(entry filesys.Entry, widthPx int) string {
	if entry.ModTime.IsZero() {
		return entry.DateText
	}
	if m == nil || m.cfg == nil || len(m.cfg.DateFormats) == 0 {
		return entry.ModTime.Format("Jan 02 2006")
	}

	capacity := m.approxChars(widthPx, 4)
	if capacity < 1 {
		return ""
	}

	for _, format := range m.cfg.DateFormats {
		txt := entry.ModTime.Format(format)
		if utf8.RuneCountInString(txt) <= capacity {
			return txt
		}
	}
	return ""
}

func parseFileSortKey(raw string) fileSortKey {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ext", "extension", "type":
		return fileSortExt
	case "size":
		return fileSortSize
	case "date", "time", "datetime":
		return fileSortDate
	default:
		return fileSortName
	}
}

func (k fileSortKey) badgeLabel() string {
	switch k {
	case fileSortExt:
		return "E"
	case fileSortSize:
		return "S"
	case fileSortDate:
		return "D"
	default:
		return "N"
	}
}

func (p *filePaneState) sortBadgeText() string {
	if p == nil {
		return "N↑"
	}
	arrow := "↑"
	if p.sortDesc {
		arrow = "↓"
	}
	return p.sortKey.badgeLabel() + arrow
}

func (p *filePaneState) modeBadgeText() string {
	if p != nil && p.table != nil && p.table.Mode == table.ModeBrief {
		return "2C"
	}
	return "1C"
}

func (p *filePaneState) cycleSortKey() {
	if p == nil {
		return
	}
	switch p.sortKey {
	case fileSortName:
		p.sortKey = fileSortExt
	case fileSortExt:
		p.sortKey = fileSortSize
	case fileSortSize:
		p.sortKey = fileSortDate
	default:
		p.sortKey = fileSortName
	}
	p.sortDesc = false
}

func (p *filePaneState) setSortKey(next fileSortKey) {
	if p == nil {
		return
	}
	p.sortKey = next
}

func (p *filePaneState) applySort(preservePath string) {
	if p == nil || p.model == nil || len(p.model.entries) == 0 {
		return
	}

	start := 0
	if p.model.entries[0].Kind == filesys.EntryParent {
		start = 1
	}
	if start >= len(p.model.entries) {
		return
	}

	rows := p.model.entries[start:]
	sort.SliceStable(rows, func(i, j int) bool {
		a := rows[i]
		b := rows[j]

		if p.dirsFirst {
			aDir := a.Kind == filesys.EntryDir
			bDir := b.Kind == filesys.EntryDir
			if aDir != bDir {
				return aDir
			}
		}

		cmp := compareFileEntries(a, b, p.sortKey)
		if cmp == 0 {
			cmp = compareFileEntries(a, b, fileSortName)
		}
		if p.sortDesc {
			cmp = -cmp
		}
		return cmp < 0
	})

	if preservePath != "" && p.table != nil {
		if idx := p.findEntryPathIndex(preservePath); idx >= 0 {
			p.table.SetSelected(idx, p.model.Len(), true)
		}
	}
}

func compareFileEntries(a, b filesys.Entry, key fileSortKey) int {
	switch key {
	case fileSortExt:
		if cmp := compareStrings(fileSortExtKey(a), fileSortExtKey(b)); cmp != 0 {
			return cmp
		}
	case fileSortSize:
		if a.SizeBytes < b.SizeBytes {
			return -1
		}
		if a.SizeBytes > b.SizeBytes {
			return 1
		}
	case fileSortDate:
		if a.ModTime.Before(b.ModTime) {
			return -1
		}
		if a.ModTime.After(b.ModTime) {
			return 1
		}
	}
	return compareStrings(strings.ToLower(a.Name), strings.ToLower(b.Name))
}

func fileSortExtKey(e filesys.Entry) string {
	if e.Kind == filesys.EntryDir {
		return ""
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(e.Name), "."))
	if ext == "" {
		return "~"
	}
	return ext
}

func compareStrings(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func (ui *UI) activePane() *filePaneState {
	if ui == nil || len(ui.filePanes) == 0 {
		return nil
	}
	if ui.activeFilePane < 0 {
		ui.activeFilePane = 0
	}
	if ui.activeFilePane >= len(ui.filePanes) {
		ui.activeFilePane = len(ui.filePanes) - 1
	}
	return ui.filePanes[ui.activeFilePane]
}

func (ui *UI) setActiveFilePane(idx int) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	ui.activeFilePane = idx
	ui.rep.active = false
	ui.rep.pane = -1
	ui.closeSortMenusExcept(idx)
}

func (ui *UI) cycleActiveFilePane(step int) {
	if len(ui.filePanes) == 0 {
		return
	}
	if step == 0 {
		step = 1
	}
	next := ui.activeFilePane + step
	for next < 0 {
		next += len(ui.filePanes)
	}
	next %= len(ui.filePanes)
	ui.setActiveFilePane(next)
}

func (ui *UI) closeSortMenusExcept(active int) {
	for i, pane := range ui.filePanes {
		if pane == nil || i == active {
			continue
		}
		pane.sortMenuOpen = false
	}
}

func (ui *UI) cyclePaneSort(idx int) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return
	}
	ui.setActiveFilePane(idx)
	preserve := ""
	if sel := pane.selectedEntry(); sel != nil {
		preserve = sel.Path
	}
	pane.cycleSortKey()
	pane.applySort(preserve)
}

func (ui *UI) togglePaneSortDirection(idx int) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return
	}
	ui.setActiveFilePane(idx)
	preserve := ""
	if sel := pane.selectedEntry(); sel != nil {
		preserve = sel.Path
	}
	pane.sortDesc = !pane.sortDesc
	pane.applySort(preserve)
	pane.sortMenuOpen = false
}

func (ui *UI) choosePaneSort(idx int, key fileSortKey) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return
	}
	ui.setActiveFilePane(idx)
	preserve := ""
	if sel := pane.selectedEntry(); sel != nil {
		preserve = sel.Path
	}
	if pane.sortKey != key {
		pane.setSortKey(key)
		pane.applySort(preserve)
	}
	pane.sortMenuOpen = false
}

func (ui *UI) togglePaneMode(idx int) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.table == nil {
		return
	}
	ui.setActiveFilePane(idx)
	if pane.table.Mode == table.ModeFull {
		pane.table.SetMode(table.ModeBrief)
		return
	}
	pane.table.SetMode(table.ModeFull)
}

func (ui *UI) queueFilePaneOpen(idx, row int) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	ui.pendingFileOpen = &fileOpenRequest{
		pane: idx,
		row:  row,
	}
}

func (ui *UI) flushPendingFileOpen() bool {
	req := ui.pendingFileOpen
	if req == nil {
		return false
	}
	ui.pendingFileOpen = nil
	return ui.activateFilePaneRow(req.pane, req.row)
}

func (ui *UI) activateFilePaneRow(idx, row int) bool {
	if idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return false
	}
	ui.setActiveFilePane(idx)
	prevDir := pane.dir
	entry := pane.model.Entry(row)
	if entry == nil || !entry.CanEnter {
		return false
	}
	if err := pane.load(entry.Path); err != nil {
		return false
	}
	pane.sortMenuOpen = false
	if entry.Kind == filesys.EntryParent {
		childName := filepath.Base(filepath.Clean(prevDir))
		if childName != "." && childName != string(filepath.Separator) {
			if sel := pane.findEntryIndex(childName); sel >= 0 && pane.table != nil && pane.model != nil {
				pane.table.SetSelected(sel, pane.model.Len(), true)
			}
		}
	}
	return true
}
