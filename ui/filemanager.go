// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/platform"
	"hexone/ui/widget/table"
	"image"
	"image/color"
	"math"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

type filePaneModel struct {
	entries         []filesys.Entry
	cfg             *fm.Config
	baseTextColor   color.NRGBA
	filenameTheme   filePaneFilenameTheme
	filenameVisuals []filePaneFilenameVisual
	measureTextPx   func(string) int
	measureCache    map[string]int
}

type fileSortKey uint8

const (
	fileSortName fileSortKey = iota
	fileSortExt
	fileSortSize
	fileSortDate
	filePaneApproxCharPx         = 8
	filePaneNameTailRunes        = 3
	filePaneDirWatchPoll         = 900 * time.Millisecond
	filePaneSortDirPruneInterval = 30 * time.Minute
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
	st := table.CellStyle{Color: m.paneTextColor(), Weight: font.Medium}

	switch entry.Kind {
	case filesys.EntryDir, filesys.EntryParent:
		st.Color = color.NRGBA{R: 170, G: 200, B: 255, A: 255}
	case filesys.EntryBroken:
		st.Color = color.NRGBA{R: 255, G: 120, B: 120, A: 255}
	}
	if c == 0 {
		switch entry.Kind {
		case filesys.EntryDir, filesys.EntryParent, filesys.EntryBroken:
			st.Weight = font.Bold
		}
		if visual := m.filenameVisual(r); visual.hasColor {
			st.Color = visual.color
			st.PreserveColor = true
		}
	}

	showPerms := m.showPermissionColumn()
	switch c {
	case 0:
		return m.filePaneEntryNameCell(entry, st, 0)
	case 1:
		if showPerms {
			return m.defaultPermissionText(entry), st
		}
		return entry.SizeText, st
	case 2:
		if showPerms {
			return entry.SizeText, st
		}
		return entry.DateText, st
	default:
		return entry.DateText, st
	}
}

func (m *filePaneModel) CellWithWidth(r, c, widthPx int) (string, table.CellStyle) {
	txt, st := m.Cell(r, c)
	entry := m.entries[r]
	showPerms := m.showPermissionColumn()

	switch c {
	case 0:
		return m.filePaneEntryNameCell(entry, st, widthPx)
	case 1:
		if showPerms {
			return m.formatPermissions(entry, widthPx), st
		}
		return m.formatSize(entry.SizeText, widthPx), st
	case 2:
		if showPerms {
			return m.formatSize(entry.SizeText, widthPx), st
		}
		return m.formatDate(entry, widthPx), st
	case 3:
		return m.formatDate(entry, widthPx), st
	default:
		return txt, st
	}
}

func (m *filePaneModel) showPermissionColumn() bool {
	return m != nil && m.cfg != nil && m.cfg.Columns.ShowPermissions
}

func (m *filePaneModel) paneTextColor() color.NRGBA {
	if m != nil && m.baseTextColor.A != 0 {
		return m.baseTextColor
	}
	return txtColor
}

func (m *filePaneModel) permissionFormat() string {
	if m == nil || m.cfg == nil {
		return "auto"
	}
	switch m.cfg.Columns.PermissionFormat {
	case "symbolic", "octal":
		return m.cfg.Columns.PermissionFormat
	default:
		return "auto"
	}
}

func (m *filePaneModel) defaultPermissionText(entry filesys.Entry) string {
	if m.permissionFormat() == "octal" {
		return entry.PermOctal
	}
	return entry.PermText
}

func (m *filePaneModel) formatPermissions(entry filesys.Entry, widthPx int) string {
	preferred := m.permissionFormat()
	candidates := []string{entry.PermText, entry.PermOctal}
	if preferred == "octal" {
		candidates[0], candidates[1] = candidates[1], candidates[0]
	}

	for _, candidate := range candidates {
		if m.fullOrEmpty(candidate, widthPx) != "" {
			return candidate
		}
	}
	return ""
}

func (m *filePaneModel) LeadingIcon(r, c int) (table.LeadingIcon, bool) {
	if m == nil || c != 0 || r < 0 || r >= len(m.entries) {
		return table.LeadingIcon{}, false
	}
	entry := m.entries[r]
	if entry.IsSymlink {
		linkColor := color.NRGBA{R: 120, G: 202, B: 214, A: 255}
		if entry.Kind == filesys.EntryBroken {
			linkColor = color.NRGBA{R: 220, G: 85, B: 85, A: 255}
		}
		return table.LeadingIcon{
			Kind:  table.IconLink,
			Color: linkColor,
		}, true
	}
	switch entry.Kind {
	case filesys.EntryParent:
		return table.LeadingIcon{
			Kind:  table.IconParent,
			Color: color.NRGBA{R: 214, G: 186, B: 96, A: 255},
		}, true
	case filesys.EntryDir:
		return table.LeadingIcon{
			Kind:  table.IconFolder,
			Color: color.NRGBA{R: 205, G: 176, B: 88, A: 255},
		}, true
	case filesys.EntryBroken:
		return table.LeadingIcon{
			Kind:  table.IconBroken,
			Color: color.NRGBA{R: 220, G: 85, B: 85, A: 255},
		}, true
	default:
		baseColor := m.fileIconColor(entry.Name)
		if visual := m.filenameVisual(r); visual.iconKey != "" {
			if visual.hasColor {
				baseColor = visual.color
			}
			return table.LeadingIcon{
				Kind:   table.IconFile,
				Color:  baseColor,
				Widget: filenameRuleIcon(visual.iconKey),
			}, true
		}
		return table.LeadingIcon{
			Kind:   table.IconFile,
			Color:  baseColor,
			Widget: m.defaultFileIconWidget(entry.Name),
		}, true
	}
}

func (m *filePaneModel) filePaneEntryNameCell(entry filesys.Entry, st table.CellStyle, widthPx int) (string, table.CellStyle) {
	name := entry.DisplayName
	if entry.IsSymlink && strings.TrimSpace(entry.LinkTarget) != "" {
		st.Suffix = " -> " + entry.LinkTarget
		st.SuffixColor = color.NRGBA{R: 132, G: 146, B: 156, A: 255}
		st.SuffixWeight = font.Normal
		st.SuffixWeightSet = true
		st.SuffixPreserveColor = true
		if entry.Kind == filesys.EntryBroken {
			st.SuffixColor = color.NRGBA{R: 224, G: 88, B: 88, A: 255}
		}
		if widthPx > 0 {
			suffixW := m.approxCharPx() * utf8.RuneCountInString(st.Suffix)
			if measured, ok := m.measuredTextWidth(st.Suffix); ok {
				suffixW = measured
			}
			if suffixW > widthPx/2 {
				suffixW = widthPx / 2
			}
			if nameW := widthPx - suffixW; nameW > m.approxCharPx()*3 {
				name = m.nameOrEmpty(name, nameW)
			} else {
				name = m.nameOrEmpty(name, widthPx)
			}
		}
		return name, st
	}
	if widthPx > 0 {
		name = m.nameOrEmpty(name, widthPx)
	}
	return name, st
}

type filePaneState struct {
	table                 *table.Table
	model                 *filePaneModel
	pathSegClicks         []widget.Clickable
	pathEdit              widget.Editor
	pathEditing           bool
	pathEditFocus         bool
	inlineNameEdit        widget.Editor
	inlineNameEditing     bool
	inlineNameFocus       bool
	inlineNameRow         int
	inlineNamePath        string
	inlineNameOriginal    string
	inlineNameRect        image.Rectangle
	inlineNamePendingRow  int
	inlineNamePendingAt   time.Time
	pathClickKey          string
	pathClickAt           time.Time
	pendingPathNav        string
	pendingPathAt         time.Time
	tableClickRow         int
	tableClickCol         int
	tableClickAt          time.Time
	pathRowClick          widget.Clickable
	modeClick             widget.Clickable
	sortClick             widget.Clickable
	favoriteClick         widget.Clickable
	disconnectClick       widget.Clickable
	tablePointerTag       uiEventTag
	sortOptionBtns        [4]widget.Clickable
	sortMenuOpen          bool
	sortMenuOpenedAt      time.Time
	sortMenuHoverID       string
	sortMenuHoverAnim     segmentedAnimState
	sortMenuRect          image.Rectangle
	sortPointerTag        uiEventTag
	sortMenuClick         widget.Clickable
	favoritePointerTag    uiEventTag
	favoriteMenuClick     widget.Clickable
	favoriteOptionClicks  []widget.Clickable
	favoriteRemoveClicks  []widget.Clickable
	favoriteMenuOpen      bool
	favoriteMenuRect      image.Rectangle
	favoriteMenuOpenedAt  time.Time
	favoriteMenuHoverID   string
	favoriteMenuHoverAnim segmentedAnimState
	favoriteHoverKey      string
	favoriteHoverAt       time.Time
	favoriteRevealKey     string
	favoriteRevealHideAt  time.Time
	favoritePointerPos    image.Point
	favoritePointerPosSet bool
	headerHeight          int
	ctxPointerTag         uiEventTag
	ctxMenuClicks         map[string]*widget.Clickable
	ctxMenuOpen           bool
	ctxMenuRow            int
	ctxMenuPos            image.Point
	ctxMenuRects          []image.Rectangle
	ctxMenuItemRects      map[string]image.Rectangle
	ctxMenuPath           []string
	ctxMenuOpenedAt       time.Time
	ctxMenuHoverID        string
	ctxMenuHoverAnim      segmentedAnimState
	drivePointerTag       uiEventTag
	driveMenuPointerTag   uiEventTag
	driveMenuClicks       []widget.Clickable
	driveMenuOpen         bool
	driveMenuPos          image.Point
	driveMenuRect         image.Rectangle
	driveSegmentRect      image.Rectangle
	driveMenuOpenedAt     time.Time
	driveMenuSelected     int
	driveMenuHoverID      string
	driveMenuHoverAnim    segmentedAnimState
	sortKey               fileSortKey
	sortDesc              bool
	dirsFirst             bool
	remote                *paneSSHSession
	localDirBeforeRemote  string
	dir                   string
	loading               bool
	loadQuiet             bool
	loadingDir            string
	loadingStartedAt      time.Time
	loadSeq               int
	loadResultCh          chan filePaneLoadResult
	dirWatch              filePaneDirWatchState
	navScrollByDir        map[string]layout.Position
	err                   string
	noticeText            string
	noticeShownAt         time.Time
	noticeUntil           time.Time
	volumeBadge           filePaneVolumeBadgeState
	markedRows            map[int]struct{}
}

type filePaneDirWatchState struct {
	nextCheck time.Time
	dir       string
	modTime   time.Time
	size      int64
	errText   string
	ready     bool
}

type filePaneLoadResult struct {
	seq           int
	listing       filesys.Listing
	err           error
	primaryPath   string
	secondaryPath string
	fallbackRow   int
	restorePos    layout.Position
	restoreScroll bool
	restoreAnchor string
	noticeText    string
	noticeDur     time.Duration
	background    bool
}

type filePaneApplyOptions struct {
	primaryPath         string
	secondaryPath       string
	fallbackRow         int
	restorePos          layout.Position
	restoreScroll       bool
	restoreAnchor       string
	preserveMarks       bool
	preserveNotice      bool
	preserveInteraction bool
}

func newFilePaneState(dir string, cfg *fm.Config) *filePaneState {
	if cfg == nil {
		cfg = fm.DefaultConfig()
	}
	scaleDp := func(v int) unit.Dp {
		return scaleFilePaneDp(cfg, v)
	}
	fullPad := unit.Dp(fm.ColumnPadDp())
	dropPriority := filePaneFullDropPriority(cfg)
	cols := []table.Column{
		{
			Width:        scaleDp(fm.NameWidthDp(cfg)),
			MinWidth:     scaleDp(fm.NameMinWidthDp(cfg)),
			Flex:         true,
			Align:        table.AlignStart,
			PadX:         fullPad,
			DropPriority: dropPriority["name"],
		},
	}
	if cfg.Columns.ShowPermissions {
		cols = append(cols, table.Column{
			Width:        scaleDp(fm.PermWidthDp(cfg)),
			MinWidth:     scaleDp(fm.PermMinWidthDp(cfg)),
			Flex:         false,
			Align:        table.AlignStart,
			PadX:         fullPad,
			DropPriority: dropPriority["permissions"],
		})
	}
	cols = append(cols,
		table.Column{
			Width:        scaleDp(fm.SizeWidthDp(cfg)),
			MinWidth:     scaleDp(fm.SizeMinWidthDp(cfg)),
			Flex:         false,
			Align:        table.AlignEnd,
			PadX:         fullPad,
			DropPriority: dropPriority["size"],
		},
		table.Column{
			Width:        scaleDp(fm.DateWidthDp(cfg)),
			MinWidth:     scaleDp(fm.DateMinWidthDp(cfg)),
			Flex:         false,
			Align:        table.AlignStart,
			PadX:         fullPad,
			DropPriority: dropPriority["date"],
		},
	)

	pane := &filePaneState{
		table:        table.New(cols),
		model:        &filePaneModel{cfg: cfg, filenameTheme: newFilePaneFilenameTheme(cfg)},
		sortKey:      parseFileSortKey(cfg.Sort.DefaultKey),
		sortDesc:     cfg.Sort.Descending,
		dirsFirst:    cfg.Sort.DirectoriesFirst,
		dir:          filepath.Clean(dir),
		loadResultCh: make(chan filePaneLoadResult, 8),
	}
	pane.pathEdit.SingleLine = true
	pane.pathEdit.Submit = true
	pane.inlineNameEdit.SingleLine = true
	pane.inlineNameEdit.Submit = true
	pane.inlineNameRow = -1
	pane.inlineNamePendingRow = -1
	pane.ctxMenuRow = -1
	pane.tableClickRow = -1
	pane.tableClickCol = -1
	pane.driveMenuSelected = -1
	pane.table.TextSize = scaleConfigFontSize(cfg, 13)
	pane.table.Typeface = font.Typeface(cfg.General.Typeface)
	pane.table.RowHeight = scaleDp(18)
	pane.table.RowPadY = unit.Dp(0)
	pane.table.BriefColumnWidth = scaleDp(fm.BriefWidthDp(cfg))
	pane.table.BriefGap = scaleDp(fm.BriefGapDp())
	palette := filePanePaletteFromConfig(cfg)
	pane.table.Bg = palette.PaneBg
	pane.table.HoverBg = palette.HoverBg
	pane.table.HoverFg = &palette.HoverFg
	pane.table.MarkedBg = palette.MarkedBg
	pane.table.MarkedFg = &palette.MarkedFg
	pane.table.SelectedBg = palette.SelectedBg
	pane.table.MarkedSelBg = palette.MarkedSelBg
	pane.table.MarkedSelFg = &palette.MarkedSelFg
	pane.table.SelectedFg = &palette.SelectedFg
	pane.table.ScrollbarWidth = scaleDp(10)
	pane.table.ScrollbarMinThumb = scaleDp(22)
	pane.table.ScrollbarTrack = palette.ScrollTrack
	pane.table.ScrollbarTrackHover = palette.ScrollTrackH
	pane.table.ScrollbarThumb = palette.ScrollThumb
	pane.table.ScrollbarThumbHover = palette.ScrollThumbH
	pane.table.ScrollbarThumbDrag = palette.ScrollThumbD
	pane.model.baseTextColor = palette.PaneFg
	pane.table.IsMarked = func(row int) bool {
		return pane.isMarkedRow(row)
	}
	if pane.dir != "" {
		pane.localDirBeforeRemote = pane.dir
	}
	return pane
}

func filePaneFullDropPriority(cfg *fm.Config) map[string]int {
	out := map[string]int{
		"name":        4,
		"permissions": 3,
		"size":        2,
		"date":        1,
	}
	if cfg == nil {
		return out
	}
	for i, key := range cfg.Columns.FullDropPriority {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "date", "time", "datetime":
			out["date"] = i + 1
		case "size":
			out["size"] = i + 1
		case "permissions", "permission", "perms", "perm":
			out["permissions"] = i + 1
		case "name", "filename", "file":
			out["name"] = i + 1
		}
	}
	return out
}

func filePaneFontScale(cfg *fm.Config) float32 {
	size := float32(fontSizeFromConfig(cfg))
	base := float32(defaultUIFontSp)
	if size <= 0 || base <= 0 {
		return 1
	}
	return size / base
}

func scaleFilePaneDp(cfg *fm.Config, v int) unit.Dp {
	if v <= 0 {
		return unit.Dp(v)
	}
	scaled := int(math.Round(float64(float32(v) * filePaneFontScale(cfg))))
	if scaled < 1 {
		scaled = 1
	}
	return unit.Dp(scaled)
}

func scaleFilePanePx(cfg *fm.Config, v int) int {
	if v <= 0 {
		return v
	}
	scaled := int(math.Round(float64(float32(v) * filePaneFontScale(cfg))))
	if scaled < 1 {
		scaled = 1
	}
	return scaled
}

func (p *filePaneState) load(dir string) error {
	var (
		listing filesys.Listing
		err     error
	)
	if p.remote != nil {
		listing, err = p.remote.readDir(dir)
	} else {
		listing, err = filesys.ReadDir(dir)
	}
	if err != nil {
		return err
	}

	p.applyListing(listing, "", "", 0)
	return nil
}

func (p *filePaneState) applyListing(listing filesys.Listing, primaryPath, secondaryPath string, fallbackRow int) {
	p.applyListingWithOptions(listing, filePaneApplyOptions{
		primaryPath:   primaryPath,
		secondaryPath: secondaryPath,
		fallbackRow:   fallbackRow,
	})
}

func (p *filePaneState) applyListingWithRestore(listing filesys.Listing, primaryPath, secondaryPath string, fallbackRow int, restorePos layout.Position, restoreScroll bool, restoreAnchor string) {
	p.applyListingWithOptions(listing, filePaneApplyOptions{
		primaryPath:   primaryPath,
		secondaryPath: secondaryPath,
		fallbackRow:   fallbackRow,
		restorePos:    restorePos,
		restoreScroll: restoreScroll,
		restoreAnchor: restoreAnchor,
	})
}

func (p *filePaneState) applyListingRefresh(listing filesys.Listing, primaryPath, secondaryPath string, fallbackRow int, restorePos layout.Position, restoreAnchor string) {
	p.applyListingWithOptions(listing, filePaneApplyOptions{
		primaryPath:         primaryPath,
		secondaryPath:       secondaryPath,
		fallbackRow:         fallbackRow,
		restorePos:          restorePos,
		restoreScroll:       true,
		restoreAnchor:       restoreAnchor,
		preserveMarks:       true,
		preserveNotice:      true,
		preserveInteraction: true,
	})
}

func (p *filePaneState) applyListingWithOptions(listing filesys.Listing, opts filePaneApplyOptions) {
	if p == nil {
		return
	}
	markedPaths := []string(nil)
	if opts.preserveMarks {
		markedPaths = append(markedPaths, p.markedPaths()...)
	}
	noticeText := ""
	noticeShownAt := time.Time{}
	noticeUntil := time.Time{}
	if opts.preserveNotice {
		noticeText = p.noticeText
		noticeShownAt = p.noticeShownAt
		noticeUntil = p.noticeUntil
	}
	p.dir = listing.Dir
	if p.remote == nil && p.dir != "" {
		p.localDirBeforeRemote = p.dir
	}
	p.loading = false
	p.loadQuiet = false
	p.loadingDir = ""
	p.loadingStartedAt = time.Time{}
	p.err = ""
	p.noticeText = ""
	p.noticeShownAt = time.Time{}
	p.noticeUntil = time.Time{}
	p.invalidateVolumeBadge()
	if !opts.preserveInteraction {
		p.stopInlineNameEdit()
		if p.table != nil {
			p.table.ResetPointerState()
		}
		p.clearTableClickState()
		p.clearPathClickState()
		p.clearPendingPathNavigate()
	}
	if !opts.preserveMarks {
		p.clearMarkedRows()
	}
	p.model.entries = listing.Entries
	p.applyConfiguredSortForCurrentDir()
	p.applySort("")
	p.table.Selected = 0
	if opts.restoreScroll {
		p.table.List.Position = restorePaneListPosition(p.model.entries, opts.restorePos, opts.restoreAnchor)
	} else {
		p.table.List.Position = layout.Position{}
	}
	p.applySelection(opts.primaryPath, opts.secondaryPath, opts.fallbackRow, !opts.restoreScroll)
	if opts.preserveMarks {
		p.restoreMarkedPaths(markedPaths)
	}
	if opts.preserveNotice && noticeText != "" {
		p.noticeText = noticeText
		p.noticeShownAt = noticeShownAt
		p.noticeUntil = noticeUntil
	}
	p.resetDirWatch()
}

func sanitizePaneListPosition(pos layout.Position) layout.Position {
	if pos.First < 0 {
		pos.First = 0
	}
	pos.Count = 0
	pos.Length = 0
	return pos
}

func restorePaneListPosition(entries []filesys.Entry, pos layout.Position, anchorPath string) layout.Position {
	pos = sanitizePaneListPosition(pos)
	if strings.TrimSpace(anchorPath) != "" {
		for i := range entries {
			if entries[i].Path == anchorPath {
				pos.First = i
				break
			}
		}
	}
	if len(entries) == 0 {
		pos.First = 0
		pos.Offset = 0
		return pos
	}
	if pos.First >= len(entries) {
		pos.First = len(entries) - 1
		pos.Offset = 0
	}
	return pos
}

func (p *filePaneState) applySelection(primaryPath, secondaryPath string, fallbackRow int, ensureVisible bool) {
	if p == nil || p.table == nil || p.model == nil || p.model.Len() == 0 {
		return
	}
	if primaryPath != "" {
		if idx := p.findEntryPathIndex(primaryPath); idx >= 0 {
			p.table.SetSelected(idx, p.model.Len(), ensureVisible)
			return
		}
	}
	if secondaryPath != "" {
		if idx := p.findEntryPathIndex(secondaryPath); idx >= 0 {
			p.table.SetSelected(idx, p.model.Len(), ensureVisible)
			return
		}
	}
	row := fallbackRow
	if row < 0 {
		row = 0
	}
	if row >= p.model.Len() {
		row = p.model.Len() - 1
	}
	p.table.SetSelected(row, p.model.Len(), ensureVisible)
}

func (p *filePaneState) setNotice(msg string, now time.Time) {
	p.setNoticeFor(msg, now, filePaneNoticeVisibleDur)
}

func (p *filePaneState) setNoticeFor(msg string, now time.Time, dur time.Duration) {
	if p == nil || msg == "" {
		return
	}
	if dur <= 0 {
		dur = filePaneNoticeVisibleDur
	}
	p.err = ""
	p.noticeText = msg
	p.noticeShownAt = now
	p.noticeUntil = now.Add(dur)
}

func (p *filePaneState) beginPathEdit() {
	if p == nil {
		return
	}
	p.cancelInlineNameEdit()
	p.clearPathClickState()
	p.clearPendingPathNavigate()
	p.pathEditing = true
	p.pathEditFocus = true
	text := p.dir
	if p.remoteConnected() {
		if strings.TrimSpace(text) == "" {
			text = "/"
		}
	}
	p.pathEdit.SetText(text)
	p.pathEdit.SetCaret(0, p.pathEdit.Len())
}

func (p *filePaneState) stopPathEdit() {
	if p == nil {
		return
	}
	p.pathEditing = false
	p.pathEditFocus = false
}

func (p *filePaneState) inlineNameEntry() *filesys.Entry {
	if p == nil || !p.inlineNameEditing || p.inlineNameRow < 0 || p.model == nil {
		return nil
	}
	return p.model.Entry(p.inlineNameRow)
}

func (p *filePaneState) beginInlineNameEdit(row int) bool {
	if p == nil || p.model == nil {
		return false
	}
	entry := p.model.Entry(row)
	if entry == nil || entry.Path == "" || entry.Kind == filesys.EntryParent {
		return false
	}
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		name = p.pathBaseName(entry.Path)
	}
	if strings.TrimSpace(name) == "" {
		return false
	}
	p.stopPathEdit()
	p.clearPathClickState()
	p.clearPendingPathNavigate()
	p.clearPendingInlineNameEdit()
	p.inlineNameEditing = true
	p.inlineNameFocus = true
	p.inlineNameRow = row
	p.inlineNamePath = entry.Path
	p.inlineNameOriginal = name
	p.inlineNameRect = image.Rectangle{}
	p.inlineNameEdit.SetText(name)
	p.inlineNameEdit.SetCaret(0, p.inlineNameEdit.Len())
	return true
}

func (p *filePaneState) stopInlineNameEdit() {
	if p == nil {
		return
	}
	p.inlineNameEditing = false
	p.inlineNameFocus = false
	p.inlineNameRow = -1
	p.inlineNamePath = ""
	p.inlineNameOriginal = ""
	p.inlineNameRect = image.Rectangle{}
	p.clearPendingInlineNameEdit()
}

func (p *filePaneState) cancelInlineNameEdit() {
	if p == nil {
		return
	}
	p.stopInlineNameEdit()
	p.inlineNameEdit.SetText("")
}

func (p *filePaneState) clearPendingPathNavigate() {
	if p == nil {
		return
	}
	p.pendingPathNav = ""
	p.pendingPathAt = time.Time{}
}

func (p *filePaneState) clearPendingInlineNameEdit() {
	if p == nil {
		return
	}
	p.inlineNamePendingRow = -1
	p.inlineNamePendingAt = time.Time{}
}

func (p *filePaneState) queueInlineNameEdit(row int, at time.Time) {
	if p == nil {
		return
	}
	p.inlineNamePendingRow = row
	p.inlineNamePendingAt = at
}

func (p *filePaneState) queuePathNavigate(path string, at time.Time) {
	if p == nil {
		return
	}
	p.pendingPathNav = path
	p.pendingPathAt = at
}

func (p *filePaneState) clearPathClickState() {
	if p == nil {
		return
	}
	p.pathClickKey = ""
	p.pathClickAt = time.Time{}
}

func (p *filePaneState) clearTableClickState() {
	if p == nil {
		return
	}
	p.tableClickRow = -1
	p.tableClickCol = -1
	p.tableClickAt = time.Time{}
}

func (p *filePaneState) registerTablePrimaryClick(row, col int, now time.Time, window time.Duration) bool {
	if p == nil || row < 0 || col < 0 {
		return false
	}
	if p.tableClickRow == row && p.tableClickCol == col && !p.tableClickAt.IsZero() && now.Sub(p.tableClickAt) <= window {
		p.tableClickRow = -1
		p.tableClickCol = -1
		p.tableClickAt = time.Time{}
		return true
	}
	p.tableClickRow = row
	p.tableClickCol = col
	p.tableClickAt = now
	return false
}

func (p *filePaneState) permissionColumnIndex() int {
	if p == nil || p.table == nil || p.table.Mode != table.ModeFull || p.model == nil || !p.model.showPermissionColumn() {
		return -1
	}
	return 1
}

func (p *filePaneState) registerPathClick(key string, now time.Time, window time.Duration) bool {
	if p == nil {
		return false
	}
	if key != "" && p.pathClickKey == key && !p.pathClickAt.IsZero() && now.Sub(p.pathClickAt) <= window {
		p.clearPathClickState()
		return true
	}
	p.pathClickKey = key
	p.pathClickAt = now
	return false
}

func (p *filePaneState) closeContextMenu() {
	if p == nil {
		return
	}
	p.ctxMenuOpen = false
	p.ctxMenuRow = -1
	p.ctxMenuRects = nil
	p.ctxMenuPath = nil
	p.ctxMenuOpenedAt = time.Time{}
	p.ctxMenuHoverID = ""
	p.ctxMenuHoverAnim = segmentedAnimState{}
	if p.table != nil {
		p.table.ResetPointerState()
	}
	if p.ctxMenuItemRects != nil {
		clear(p.ctxMenuItemRects)
	}
	p.ctxMenuClicks = nil
}

func (p *filePaneState) closeFavoriteMenu() {
	if p == nil {
		return
	}
	p.favoriteMenuOpen = false
	p.favoriteMenuRect = image.Rectangle{}
	p.favoriteMenuOpenedAt = time.Time{}
	p.favoriteMenuHoverID = ""
	p.favoriteMenuHoverAnim = segmentedAnimState{}
	p.favoriteHoverKey = ""
	p.favoriteHoverAt = time.Time{}
	p.favoriteRevealKey = ""
	p.favoriteRevealHideAt = time.Time{}
	p.favoritePointerPos = image.Point{}
	p.favoritePointerPosSet = false
}

func (p *filePaneState) openFavoriteMenu(now time.Time) {
	if p == nil {
		return
	}
	p.favoriteMenuOpen = true
	p.favoriteMenuRect = image.Rectangle{}
	p.favoriteMenuOpenedAt = now
	p.favoriteMenuHoverID = ""
	p.favoriteMenuHoverAnim = segmentedAnimState{}
	p.favoriteHoverKey = ""
	p.favoriteHoverAt = time.Time{}
	p.favoriteRevealKey = ""
	p.favoriteRevealHideAt = time.Time{}
	p.favoritePointerPos = image.Point{}
	p.favoritePointerPosSet = false
}

func (p *filePaneState) closeSortMenu() {
	if p == nil {
		return
	}
	p.sortMenuOpen = false
	p.sortMenuOpenedAt = time.Time{}
	p.sortMenuHoverID = ""
	p.sortMenuHoverAnim = segmentedAnimState{}
	p.sortMenuRect = image.Rectangle{}
}

func (p *filePaneState) openSortMenu(now time.Time) {
	if p == nil {
		return
	}
	p.sortMenuOpen = true
	p.sortMenuOpenedAt = now
	p.sortMenuHoverID = ""
	p.sortMenuHoverAnim = segmentedAnimState{}
	p.sortMenuRect = image.Rectangle{}
}

func (p *filePaneState) closeDriveMenu() {
	if p == nil {
		return
	}
	p.driveMenuOpen = false
	p.driveMenuRect = image.Rectangle{}
	p.driveMenuOpenedAt = time.Time{}
	p.driveMenuSelected = -1
	p.driveMenuHoverID = ""
}

func (p *filePaneState) openContextMenu(row int, pos image.Point, now time.Time) {
	if p == nil {
		return
	}
	p.ctxMenuOpen = true
	p.ctxMenuRow = row
	p.ctxMenuPos = pos
	p.ctxMenuRects = nil
	p.ctxMenuPath = nil
	p.ctxMenuOpenedAt = now
	p.ctxMenuHoverID = ""
	p.ctxMenuHoverAnim = segmentedAnimState{}
	if p.table != nil {
		p.table.ResetPointerState()
	}
	if p.ctxMenuItemRects != nil {
		clear(p.ctxMenuItemRects)
	}
	p.ctxMenuClicks = nil
}

func (p *filePaneState) openDriveMenu(pos image.Point, now time.Time) {
	if p == nil {
		return
	}
	p.driveMenuOpen = true
	p.driveMenuPos = pos
	p.driveMenuRect = image.Rectangle{}
	p.driveMenuOpenedAt = now
	p.driveMenuSelected = -1
	p.driveMenuHoverID = ""
	p.driveMenuHoverAnim = segmentedAnimState{}
}

func driveMenuDefaultSelection(p *filePaneState, drives []string) int {
	if p == nil || len(drives) == 0 {
		return -1
	}
	currentDrive := localDriveRoot(p.displayDir())
	for i, drive := range drives {
		if strings.EqualFold(localDriveRoot(drive), currentDrive) || strings.EqualFold(drive, currentDrive) {
			return i
		}
	}
	return 0
}

func clampDriveMenuSelection(index int, count int) int {
	if count <= 0 {
		return -1
	}
	if index < 0 {
		return -1
	}
	if index >= count {
		return count - 1
	}
	return index
}

func (p *filePaneState) currentDriveMenuSelection(drives []string) int {
	if p == nil {
		return -1
	}
	if p.driveMenuSelected >= 0 && p.driveMenuSelected < len(drives) {
		return p.driveMenuSelected
	}
	return driveMenuDefaultSelection(p, drives)
}

func (p *filePaneState) setDriveMenuSelection(index int, drives []string) bool {
	if p == nil {
		return false
	}
	next := clampDriveMenuSelection(index, len(drives))
	if next < 0 {
		next = driveMenuDefaultSelection(p, drives)
	}
	if p.driveMenuSelected == next {
		return false
	}
	p.driveMenuSelected = next
	return true
}

func (p *filePaneState) moveDriveMenuSelection(delta int, drives []string) bool {
	if p == nil || delta == 0 || len(drives) == 0 {
		return false
	}
	index := p.currentDriveMenuSelection(drives)
	if index < 0 {
		return false
	}
	index += delta
	if index < 0 {
		index = len(drives) - 1
	} else if index >= len(drives) {
		index = 0
	}
	return p.setDriveMenuSelection(index, drives)
}

func (p *filePaneState) contextMenuClick(id string) *widget.Clickable {
	if p == nil || strings.TrimSpace(id) == "" {
		return nil
	}
	if p.ctxMenuClicks == nil {
		p.ctxMenuClicks = make(map[string]*widget.Clickable)
	}
	if click, ok := p.ctxMenuClicks[id]; ok {
		return click
	}
	click := &widget.Clickable{}
	p.ctxMenuClicks[id] = click
	return click
}

func (p *filePaneState) ensureFavoriteOptionClicks(n int) {
	if n <= cap(p.favoriteOptionClicks) {
		p.favoriteOptionClicks = p.favoriteOptionClicks[:n]
		return
	}
	old := p.favoriteOptionClicks
	p.favoriteOptionClicks = make([]widget.Clickable, n)
	copy(p.favoriteOptionClicks, old)
}

func (p *filePaneState) ensureFavoriteRemoveClicks(n int) {
	if n <= cap(p.favoriteRemoveClicks) {
		p.favoriteRemoveClicks = p.favoriteRemoveClicks[:n]
		return
	}
	old := p.favoriteRemoveClicks
	p.favoriteRemoveClicks = make([]widget.Clickable, n)
	copy(p.favoriteRemoveClicks, old)
}

func (p *filePaneState) ensureDriveMenuClicks(n int) {
	if n <= cap(p.driveMenuClicks) {
		p.driveMenuClicks = p.driveMenuClicks[:n]
		return
	}
	old := p.driveMenuClicks
	p.driveMenuClicks = make([]widget.Clickable, n)
	copy(p.driveMenuClicks, old)
}

func (p *filePaneState) ensurePathClicks(n int) {
	if n <= cap(p.pathSegClicks) {
		p.pathSegClicks = p.pathSegClicks[:n]
		return
	}
	old := p.pathSegClicks
	p.pathSegClicks = make([]widget.Clickable, n)
	copy(p.pathSegClicks, old)
}

type filePathSegment struct {
	label string
	path  string
}

func splitFilePathSegments(dir string) []filePathSegment {
	if dir == "" {
		dir = "."
	}
	cleaned := filepath.Clean(dir)
	sep := string(filepath.Separator)
	vol := filepath.VolumeName(cleaned)
	rest := cleaned[len(vol):]
	hasRoot := strings.HasPrefix(rest, sep)
	if hasRoot {
		rest = strings.TrimLeft(rest, sep)
	}

	parts := make([]string, 0, 8)
	if rest != "" {
		parts = strings.FieldsFunc(rest, func(r rune) bool {
			return r == rune(filepath.Separator)
		})
	}

	out := make([]filePathSegment, 0, len(parts)+1)
	current := ""
	if hasRoot {
		root := vol + sep
		if root == "" {
			root = sep
		}
		current = root
		out = append(out, filePathSegment{label: root, path: current})
	} else if vol != "" {
		current = vol
		out = append(out, filePathSegment{label: vol, path: current})
	}

	for i, part := range parts {
		label := part
		if len(out) > 0 && !(hasRoot && i == 0) {
			label = sep + part
		}
		if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}
		out = append(out, filePathSegment{label: label, path: current})
	}

	if len(out) == 0 {
		out = append(out, filePathSegment{label: cleaned, path: cleaned})
	}
	return out
}

func localDriveRoot(dir string) string {
	cleaned := filepath.Clean(strings.TrimSpace(dir))
	vol := filepath.VolumeName(cleaned)
	if vol == "" {
		return ""
	}
	return vol + string(filepath.Separator)
}

func splitRemotePathSegments(dir string) []filePathSegment {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = "/"
	}
	cleaned := path.Clean(dir)
	if cleaned == "" || cleaned == "." {
		cleaned = "/"
	}
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	if cleaned == "/" {
		return []filePathSegment{{label: "/", path: "/"}}
	}

	parts := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	out := make([]filePathSegment, 0, len(parts)+1)
	out = append(out, filePathSegment{label: "/", path: "/"})

	current := "/"
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if current == "/" {
			current = "/" + part
		} else {
			current = current + "/" + part
		}
		label := "/" + part
		if i == 0 {
			label = part
		}
		out = append(out, filePathSegment{label: label, path: current})
	}
	return out
}

func remotePathDisplaySegments(address, dir string) []filePathSegment {
	segments := splitRemotePathSegments(dir)
	if len(segments) == 0 {
		segments = []filePathSegment{{label: "/", path: "/"}}
	}
	address = strings.TrimSpace(address)
	if address == "" {
		address = "ssh"
	}
	segments[0].label = address + segments[0].label
	return segments
}

func (p *filePaneState) selectedEntry() *filesys.Entry {
	if p == nil || p.table == nil || p.model == nil {
		return nil
	}
	return p.model.Entry(p.table.Selected)
}

func (p *filePaneState) isMarkedRow(row int) bool {
	if p == nil || len(p.markedRows) == 0 {
		return false
	}
	_, ok := p.markedRows[row]
	return ok
}

func (p *filePaneState) hasMarkedRows() bool {
	return p != nil && len(p.markedRows) > 0
}

func (p *filePaneState) clearMarkedRows() bool {
	if p == nil || len(p.markedRows) == 0 {
		return false
	}
	p.markedRows = nil
	return true
}

func (p *filePaneState) toggleMarkedRow(row int) bool {
	if p == nil || p.model == nil {
		return false
	}
	entry := p.model.Entry(row)
	if entry == nil || entry.Path == "" || entry.Kind == filesys.EntryParent {
		return false
	}
	if p.isMarkedRow(row) {
		delete(p.markedRows, row)
		if len(p.markedRows) == 0 {
			p.markedRows = nil
		}
		return true
	}
	return p.markRow(row)
}

func (p *filePaneState) replaceMarkedRows(rows []int) bool {
	if p == nil {
		return false
	}
	changed := p.clearMarkedRows()
	for _, row := range rows {
		if p.markRow(row) {
			changed = true
		}
	}
	return changed
}

func (p *filePaneState) markRow(row int) bool {
	if p == nil || p.model == nil {
		return false
	}
	entry := p.model.Entry(row)
	if entry == nil || entry.Path == "" || entry.Kind == filesys.EntryParent {
		return false
	}
	if p.markedRows == nil {
		p.markedRows = make(map[int]struct{}, 4)
	}
	if _, exists := p.markedRows[row]; exists {
		return false
	}
	p.markedRows[row] = struct{}{}
	return true
}

func (p *filePaneState) markedRowsExactly(rows []int) bool {
	if p == nil {
		return len(rows) == 0
	}
	if len(rows) != len(p.markedRows) {
		return false
	}
	for _, row := range rows {
		if !p.isMarkedRow(row) {
			return false
		}
	}
	return true
}

func (p *filePaneState) replaceMarkedRange(start, end int) bool {
	if p == nil || p.model == nil || p.model.Len() == 0 {
		return false
	}
	if start > end {
		start, end = end, start
	}
	changed := p.clearMarkedRows()
	for row := start; row <= end; row++ {
		if p.markRow(row) {
			changed = true
		}
	}
	return changed
}

func (p *filePaneState) selectableRowIndexes() []int {
	if p == nil || p.model == nil {
		return nil
	}
	rows := make([]int, 0, p.model.Len())
	for row := 0; row < p.model.Len(); row++ {
		entry := p.model.Entry(row)
		if entry == nil || entry.Path == "" || entry.Kind == filesys.EntryParent {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func (p *filePaneState) matchingRowIndexesForCurrentSelection() []int {
	if p == nil || p.model == nil {
		return nil
	}
	selected := p.selectedEntry()
	if selected == nil || selected.Path == "" || selected.Kind == filesys.EntryParent {
		return nil
	}

	rows := make([]int, 0, p.model.Len())
	switch selected.Kind {
	case filesys.EntryDir:
		for row := 0; row < p.model.Len(); row++ {
			entry := p.model.Entry(row)
			if entry != nil && entry.Kind == filesys.EntryDir && entry.Path != "" {
				rows = append(rows, row)
			}
		}
	default:
		wantExt := fileExtension(selected.Name)
		for row := 0; row < p.model.Len(); row++ {
			entry := p.model.Entry(row)
			if entry == nil || entry.Path == "" || entry.Kind == filesys.EntryParent || entry.Kind == filesys.EntryDir {
				continue
			}
			if fileExtension(entry.Name) == wantExt {
				rows = append(rows, row)
			}
		}
	}
	return rows
}

func (p *filePaneState) toggleMarkedRows(rows []int) bool {
	if len(rows) == 0 {
		return false
	}
	if p.markedRowsExactly(rows) {
		return p.clearMarkedRows()
	}
	return p.replaceMarkedRows(rows)
}

func (p *filePaneState) toggleMarkAllSelectable() bool {
	return p.toggleMarkedRows(p.selectableRowIndexes())
}

func (p *filePaneState) toggleMarkRowsMatchingCurrentSelection() bool {
	return p.toggleMarkedRows(p.matchingRowIndexesForCurrentSelection())
}

func (p *filePaneState) markCurrentAndAdvance() bool {
	if p == nil || p.table == nil || p.model == nil {
		return false
	}
	total := p.model.Len()
	if total <= 0 {
		return false
	}
	changed := p.toggleMarkedRow(p.table.Selected)
	if p.table.Selected < total-1 {
		p.table.SetSelected(p.table.Selected+1, total, true)
		changed = true
	}
	return changed
}

func (p *filePaneState) markedRowIndexes() []int {
	if p == nil || len(p.markedRows) == 0 {
		return nil
	}
	rows := make([]int, 0, len(p.markedRows))
	for row := range p.markedRows {
		if p.model == nil || p.model.Entry(row) == nil {
			continue
		}
		rows = append(rows, row)
	}
	sort.Ints(rows)
	return rows
}

func (p *filePaneState) markedPaths() []string {
	rows := p.markedRowIndexes()
	if len(rows) == 0 {
		return nil
	}
	paths := make([]string, 0, len(rows))
	for _, row := range rows {
		if entry := p.model.Entry(row); entry != nil && entry.Path != "" {
			paths = append(paths, entry.Path)
		}
	}
	return paths
}

func (p *filePaneState) restoreMarkedPaths(paths []string) {
	if p == nil {
		return
	}
	p.markedRows = nil
	for _, itemPath := range paths {
		if idx := p.findEntryPathIndex(itemPath); idx >= 0 {
			p.markRow(idx)
		}
	}
}

func (p *filePaneState) selectedEntriesForAction() []filesys.Entry {
	if p == nil || p.model == nil {
		return nil
	}
	rows := p.markedRowIndexes()
	if len(rows) > 0 {
		out := make([]filesys.Entry, 0, len(rows))
		for _, row := range rows {
			if entry := p.model.Entry(row); entry != nil {
				out = append(out, *entry)
			}
		}
		return out
	}
	if entry := p.selectedEntry(); entry != nil && entry.Path != "" && entry.Kind != filesys.EntryParent {
		return []filesys.Entry{*entry}
	}
	return nil
}

func (p *filePaneState) remoteConnected() bool {
	return p != nil && p.remote != nil
}

func (p *filePaneState) archiveBrowsing() bool {
	return p != nil && !p.remoteConnected() && filesys.ArchivePathActive(p.dir)
}

func (p *filePaneState) writableLocalView() bool {
	return p != nil && !p.remoteConnected() && !p.archiveBrowsing()
}

func (p *filePaneState) archiveParentDir() string {
	if p == nil {
		return ""
	}
	if loc, ok := filesys.ParseArchivePath(p.dir); ok {
		return filepath.Dir(loc.ArchivePath)
	}
	return ""
}

func (p *filePaneState) rememberDirScroll(dir string) {
	if p == nil || p.table == nil || strings.TrimSpace(dir) == "" {
		return
	}
	if p.navScrollByDir == nil {
		p.navScrollByDir = make(map[string]layout.Position)
	}
	p.navScrollByDir[dir] = sanitizePaneListPosition(p.table.List.Position)
}

func (p *filePaneState) restoreDirScroll(dir string) (layout.Position, bool) {
	if p == nil || strings.TrimSpace(dir) == "" || len(p.navScrollByDir) == 0 {
		return layout.Position{}, false
	}
	pos, ok := p.navScrollByDir[dir]
	if !ok {
		return layout.Position{}, false
	}
	return sanitizePaneListPosition(pos), true
}

func (p *filePaneState) visibleAnchorPath() string {
	if p == nil || p.table == nil || p.model == nil || p.model.Len() == 0 {
		return ""
	}
	row := p.table.List.Position.First
	if row < 0 {
		row = 0
	}
	if row >= p.model.Len() {
		row = p.model.Len() - 1
	}
	entry := p.model.Entry(row)
	if entry == nil {
		return ""
	}
	return entry.Path
}

func (p *filePaneState) displayDir() string {
	if p == nil {
		return ""
	}
	if p.remote == nil && p.loading && strings.TrimSpace(p.loadingDir) != "" {
		return p.loadingDir
	}
	if p.remote == nil {
		return p.dir
	}
	base := p.remote.displayPrefix()
	if base == "" {
		base = "ssh"
	}
	dir := p.dir
	if strings.TrimSpace(dir) == "" {
		dir = "/"
	}
	if !strings.HasPrefix(dir, "/") {
		dir = "/" + dir
	}
	return base + dir
}

func (p *filePaneState) loadingHintVisible(now time.Time) bool {
	if p == nil || !p.loading || p.loadQuiet || p.remoteConnected() || p.loadingStartedAt.IsZero() {
		return false
	}
	return !now.Before(p.loadingStartedAt.Add(filePaneLoadingHintDelay))
}

func (p *filePaneState) resetDirWatch() {
	if p == nil {
		return
	}
	p.dirWatch = filePaneDirWatchState{}
}

func (p *filePaneState) dirWatchEligible() bool {
	if p == nil || p.remoteConnected() || p.archiveBrowsing() || p.loading || p.pathEditing || p.inlineNameEditing || p.ctxMenuOpen {
		return false
	}
	return strings.TrimSpace(p.dir) != ""
}

func (p *filePaneState) dirWatchChanged() bool {
	if p == nil {
		return false
	}
	target := filepath.Clean(p.dir)
	info, err := os.Stat(target)
	errText := ""
	modTime := time.Time{}
	size := int64(0)
	if err != nil {
		errText = err.Error()
	} else {
		modTime = info.ModTime()
		size = info.Size()
	}
	changed := p.dirWatch.ready &&
		(p.dirWatch.dir != target ||
			!p.dirWatch.modTime.Equal(modTime) ||
			p.dirWatch.size != size ||
			p.dirWatch.errText != errText)
	p.dirWatch.dir = target
	p.dirWatch.modTime = modTime
	p.dirWatch.size = size
	p.dirWatch.errText = errText
	p.dirWatch.ready = true
	return changed
}

func (p *filePaneState) pathBaseName(raw string) string {
	if p == nil {
		return filepath.Base(filepath.Clean(raw))
	}
	if p.remoteConnected() {
		return path.Base(path.Clean(raw))
	}
	return filepath.Base(filepath.Clean(raw))
}

func (p *filePaneState) contextMenuEntry() *filesys.Entry {
	if p == nil || p.model == nil || p.ctxMenuRow < 0 {
		return nil
	}
	return p.model.Entry(p.ctxMenuRow)
}

type fileFavoriteItem struct {
	label      string
	targetDir  string
	addCurrent bool
	active     bool
	disabled   bool
	removable  bool
}

func normalizeFavoriteLocation(raw string) string {
	loc := strings.TrimSpace(raw)
	if loc == "" {
		return ""
	}
	if remote, ok := parseRemoteFavoriteLocation(loc); ok {
		return formatRemoteFavoriteLocation(remote)
	}
	if filepath.IsAbs(loc) {
		return filepath.Clean(loc)
	}
	return loc
}

type remoteFavoriteLocation struct {
	User string
	Host string
	Port int
	Dir  string
}

func normalizeRemoteFavoriteDir(raw string) string {
	dir := strings.TrimSpace(raw)
	if dir == "" {
		return "/"
	}
	if !strings.HasPrefix(dir, "/") {
		dir = "/" + dir
	}
	dir = path.Clean(dir)
	if dir == "" || dir == "." {
		dir = "/"
	}
	if !strings.HasPrefix(dir, "/") {
		dir = "/" + dir
	}
	return dir
}

func parseRemoteFavoriteLocation(raw string) (remoteFavoriteLocation, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || !strings.EqualFold(u.Scheme, "ssh") {
		return remoteFavoriteLocation{}, false
	}

	user := ""
	if u.User != nil {
		user = strings.TrimSpace(u.User.Username())
	}
	host := strings.TrimSpace(u.Hostname())
	if user == "" || host == "" {
		return remoteFavoriteLocation{}, false
	}

	port := 22
	if pText := strings.TrimSpace(u.Port()); pText != "" {
		p, err := strconv.Atoi(pText)
		if err != nil || p < 1 || p > 65535 {
			return remoteFavoriteLocation{}, false
		}
		port = p
	}

	return remoteFavoriteLocation{
		User: user,
		Host: host,
		Port: port,
		Dir:  normalizeRemoteFavoriteDir(u.Path),
	}, true
}

func formatRemoteFavoriteLocation(loc remoteFavoriteLocation) string {
	port := loc.Port
	if port <= 0 || port > 65535 {
		port = 22
	}
	hostPort := net.JoinHostPort(strings.TrimSpace(loc.Host), strconv.Itoa(port))
	u := &url.URL{
		Scheme: "ssh",
		User:   url.User(strings.TrimSpace(loc.User)),
		Host:   hostPort,
		Path:   normalizeRemoteFavoriteDir(loc.Dir),
	}
	return u.String()
}

func displayRemoteFavoriteLocation(loc remoteFavoriteLocation) string {
	base := strings.TrimSpace(loc.User) + "@" + strings.TrimSpace(loc.Host)
	if loc.Port > 0 && loc.Port != 22 {
		base += ":" + strconv.Itoa(loc.Port)
	}
	return base + normalizeRemoteFavoriteDir(loc.Dir)
}

func remoteFavoriteFromPane(pane *filePaneState) (string, bool) {
	if pane == nil || !pane.remoteConnected() || pane.remote == nil {
		return "", false
	}
	setup := pane.remote.setup
	user := strings.TrimSpace(setup.User)
	host := strings.TrimSpace(setup.Host)
	if user == "" || host == "" {
		return "", false
	}
	port := setup.Port
	if port <= 0 {
		port = 22
	}
	return formatRemoteFavoriteLocation(remoteFavoriteLocation{
		User: user,
		Host: host,
		Port: port,
		Dir:  pane.dir,
	}), true
}

func paneMatchesRemoteFavorite(pane *filePaneState, loc remoteFavoriteLocation) bool {
	return paneMatchesRemoteFavoriteTarget(pane, loc) &&
		normalizeRemoteFavoriteDir(pane.dir) == loc.Dir
}

func paneMatchesRemoteFavoriteTarget(pane *filePaneState, loc remoteFavoriteLocation) bool {
	if pane == nil || !pane.remoteConnected() || pane.remote == nil {
		return false
	}
	setup := pane.remote.setup
	port := setup.Port
	if port <= 0 {
		port = 22
	}
	return strings.TrimSpace(setup.User) == loc.User &&
		strings.EqualFold(strings.TrimSpace(setup.Host), loc.Host) &&
		port == loc.Port
}

func findSSHSetupForRemoteFavorite(cfg *fm.Config, loc remoteFavoriteLocation) (fm.SSHSetup, bool) {
	if cfg == nil {
		return fm.SSHSetup{}, false
	}
	for _, raw := range cfg.SSH.Setups {
		user := strings.TrimSpace(raw.User)
		host := strings.TrimSpace(raw.Host)
		port := raw.Port
		if port <= 0 {
			port = 22
		}
		if user == loc.User && strings.EqualFold(host, loc.Host) && port == loc.Port {
			setup := raw
			setup.User = user
			setup.Host = host
			setup.Port = port
			return setup, true
		}
	}
	return fm.SSHSetup{}, false
}

func favoriteLocationEqual(a, b string) bool {
	a = normalizeFavoriteLocation(a)
	b = normalizeFavoriteLocation(b)
	if a == "" || b == "" {
		return false
	}
	ar, aRemote := parseRemoteFavoriteLocation(a)
	br, bRemote := parseRemoteFavoriteLocation(b)
	if aRemote || bRemote {
		if !aRemote || !bRemote {
			return false
		}
		return ar.User == br.User &&
			strings.EqualFold(ar.Host, br.Host) &&
			ar.Port == br.Port &&
			ar.Dir == br.Dir
	}
	if filepath.IsAbs(a) && filepath.IsAbs(b) {
		return samePath(a, b)
	}
	return a == b
}

func (ui *UI) paneFavoriteItems(pane *filePaneState) []fileFavoriteItem {
	if ui == nil || ui.fmCfg == nil {
		return []fileFavoriteItem{{label: "Add current dir", addCurrent: true}}
	}
	items := make([]fileFavoriteItem, 0, 2+len(ui.fmCfg.FavoriteLocations))

	hasFavorites := false
	current := ""
	if pane != nil {
		current = normalizeFavoriteLocation(pane.dir)
	}
	for _, raw := range ui.fmCfg.FavoriteLocations {
		loc := normalizeFavoriteLocation(raw)
		if loc == "" {
			continue
		}
		hasFavorites = true
		if remoteLoc, ok := parseRemoteFavoriteLocation(loc); ok {
			items = append(items, fileFavoriteItem{
				label:     displayRemoteFavoriteLocation(remoteLoc),
				targetDir: loc,
				active:    paneMatchesRemoteFavorite(pane, remoteLoc),
				removable: true,
			})
			continue
		}
		items = append(items, fileFavoriteItem{
			label:     loc,
			targetDir: loc,
			active:    !pane.remoteConnected() && current != "" && favoriteLocationEqual(loc, current),
			removable: true,
		})
	}
	if !hasFavorites {
		items = append(items, fileFavoriteItem{
			label:    "No favorites saved",
			disabled: true,
		})
	}
	items = append(items, fileFavoriteItem{
		label:      "Add current dir",
		addCurrent: true,
	})
	return items
}

func (ui *UI) addFavoriteLocation(raw string) (string, bool, error) {
	if ui == nil {
		return "", false, nil
	}
	if err := ui.ensureFMConfigLoaded(); err != nil {
		return "", false, err
	}
	loc := normalizeFavoriteLocation(raw)
	if loc == "" {
		return "", false, nil
	}
	for _, existing := range ui.fmCfg.FavoriteLocations {
		if favoriteLocationEqual(existing, loc) {
			return loc, false, nil
		}
	}
	ui.fmCfg.FavoriteLocations = append(ui.fmCfg.FavoriteLocations, loc)
	if err := ui.saveFMConfigWithOptions("favorites-add", false); err != nil {
		ui.fmCfg.FavoriteLocations = ui.fmCfg.FavoriteLocations[:len(ui.fmCfg.FavoriteLocations)-1]
		return loc, false, err
	}
	return loc, true, nil
}

func (ui *UI) addPaneCurrentDirFavorite(idx int, now time.Time) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return
	}
	target := pane.dir
	if pane.remoteConnected() {
		remoteLoc, ok := remoteFavoriteFromPane(pane)
		if !ok {
			pane.setNotice("failed to capture current ssh location", now)
			return
		}
		target = remoteLoc
	}
	_, added, err := ui.addFavoriteLocation(target)
	if err != nil {
		pane.setNotice("failed to save favorites: "+err.Error(), now)
		return
	}
	if !added {
		return
	}
}

func (ui *UI) removeFavoriteLocation(raw string) (bool, error) {
	if ui == nil || ui.fmCfg == nil {
		return false, nil
	}
	loc := normalizeFavoriteLocation(raw)
	if loc == "" || len(ui.fmCfg.FavoriteLocations) == 0 {
		return false, nil
	}

	next := make([]string, 0, len(ui.fmCfg.FavoriteLocations))
	removed := false
	for _, existing := range ui.fmCfg.FavoriteLocations {
		if !removed && favoriteLocationEqual(existing, loc) {
			removed = true
			continue
		}
		next = append(next, existing)
	}
	if !removed {
		return false, nil
	}

	prev := ui.fmCfg.FavoriteLocations
	ui.fmCfg.FavoriteLocations = next
	if err := ui.saveFMConfigWithOptions("favorites-remove", false); err != nil {
		ui.fmCfg.FavoriteLocations = prev
		return false, err
	}
	return true, nil
}

func (ui *UI) navigatePaneFavorite(idx int, target string) bool {
	if idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return false
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	target = normalizeFavoriteLocation(target)
	if target == "" {
		return false
	}

	if remoteLoc, ok := parseRemoteFavoriteLocation(target); ok {
		pane.closeFavoriteMenu()
		if paneMatchesRemoteFavorite(pane, remoteLoc) {
			if normalizeRemoteFavoriteDir(pane.dir) == remoteLoc.Dir {
				return true
			}
			return ui.loadPaneDir(idx, remoteLoc.Dir)
		}
		if paneMatchesRemoteFavoriteTarget(pane, remoteLoc) {
			return ui.loadPaneDir(idx, remoteLoc.Dir)
		}
		if shared := ui.findReusableRemoteSessionForFavorite(idx, remoteLoc); shared != nil {
			next := shared.clone()
			if next != nil {
				if err := ui.attachPaneSSHSession(idx, next, remoteLoc.Dir, time.Now(), false); err != nil {
					pane.setNotice("ssh connect failed: "+err.Error(), time.Now())
					return false
				}
				return true
			}
		}

		setup, found := findSSHSetupForRemoteFavorite(ui.fmCfg, remoteLoc)
		if !found {
			pane.setNotice("missing SSH setup for favorite: "+displayRemoteFavoriteLocation(remoteLoc), time.Now())
			return false
		}
		if err := ui.connectPaneSSH(idx, setup, remoteLoc.Dir, time.Now()); err != nil {
			pane.setNotice("ssh connect failed: "+err.Error(), time.Now())
			return false
		}
		return true
	}

	if pane.remoteConnected() {
		ui.disconnectPaneSSH(idx, time.Now())
		pane = ui.filePanes[idx]
		if pane == nil {
			return false
		}
	}
	if favoriteLocationEqual(target, pane.dir) {
		pane.closeFavoriteMenu()
		return true
	}
	pane.closeFavoriteMenu()
	return ui.loadPaneDir(idx, target)
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
	if m == nil {
		return filePaneApproxCharPx
	}
	return scaleFilePanePx(m.cfg, filePaneApproxCharPx)
}

func (m *filePaneModel) setTextMeasurer(measure func(string) int) {
	if m == nil {
		return
	}
	m.measureTextPx = measure
	if measure == nil {
		m.measureCache = nil
		return
	}
	if m.measureCache == nil {
		m.measureCache = make(map[string]int)
		return
	}
	for key := range m.measureCache {
		delete(m.measureCache, key)
	}
}

func (m *filePaneModel) measuredTextWidth(text string) (int, bool) {
	if m == nil || m.measureTextPx == nil {
		return 0, false
	}
	if width, ok := m.measureCache[text]; ok {
		return width, true
	}
	width := m.measureTextPx(text)
	m.measureCache[text] = width
	return width, true
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
	if capacity <= 0 {
		return ""
	}
	if capacity < 3 {
		return string(runes[:capacity])
	}

	marker := ".."
	headMin := 6
	tailMin := filePaneNameTailRunes
	if m.cfg != nil {
		if m.cfg.NameCompact.Marker != "" {
			marker = m.cfg.NameCompact.Marker
		}
		if m.cfg.NameCompact.KeepStartChars > 0 {
			headMin = m.cfg.NameCompact.KeepStartChars
		}
	}

	markerRunes := utf8.RuneCountInString(marker)
	available := capacity - markerRunes
	if available < 2 {
		if capacity > len(runes) {
			capacity = len(runes)
		}
		return string(runes[:capacity])
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
	if measured, ok := m.measuredTextWidth(text); ok {
		if measured <= widthPx {
			return text
		}
		return ""
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
		if measured, ok := m.measuredTextWidth(txt); ok {
			if measured <= widthPx {
				return txt
			}
			continue
		}
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

func (k fileSortKey) sessionValue() string {
	switch k {
	case fileSortExt:
		return "ext"
	case fileSortSize:
		return "size"
	case fileSortDate:
		return "date"
	default:
		return "name"
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

func (p *filePaneState) sessionSortKey() string {
	if p == nil {
		return "name"
	}
	return p.sortKey.sessionValue()
}

func (p *filePaneState) sessionMode() string {
	if p != nil && p.table != nil && p.table.Mode == table.ModeBrief {
		return "brief"
	}
	return "full"
}

func (p *filePaneState) sortDirConfigKey() string {
	if p == nil || p.remoteConnected() || filesys.ArchivePathActive(p.dir) {
		return ""
	}
	dir := strings.TrimSpace(p.dir)
	if dir == "" {
		return ""
	}
	return filepath.Clean(dir)
}

func (p *filePaneState) applyConfiguredSortForCurrentDir() {
	if p == nil || p.model == nil || p.model.cfg == nil {
		return
	}
	cfg := p.model.cfg
	key := fm.NormalizeSortKey(cfg.Sort.DefaultKey)
	desc := cfg.Sort.Descending
	if dir := p.sortDirConfigKey(); dir != "" {
		if raw, ok := cfg.Sort.PerDir[dir]; ok {
			if savedKey, savedDesc, savedOK := fm.ParseSortOrderCode(raw); savedOK {
				key = savedKey
				desc = savedDesc
			}
		}
	}
	p.sortKey = parseFileSortKey(key)
	p.sortDesc = desc
	p.dirsFirst = cfg.Sort.DirectoriesFirst
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
	markedPaths := p.markedPaths()
	p.model.rebuildFilenameVisuals(time.Now())

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
	if len(markedPaths) > 0 {
		p.restoreMarkedPaths(markedPaths)
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

func (m *filePaneModel) fileIconColor(name string) color.NRGBA {
	ext := fileExtension(name)
	switch ext {
	case "go", "js", "ts", "tsx", "jsx", "py", "rb", "rs", "c", "cc", "cpp", "h", "hpp", "java", "cs", "swift", "kt", "php", "lua", "sh", "zsh", "bash":
		return color.NRGBA{R: 92, G: 168, B: 255, A: 255}
	case "md", "txt", "rtf", "doc", "docx", "pdf":
		return color.NRGBA{R: 130, G: 210, B: 170, A: 255}
	case "png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "tif", "tiff", "ico":
		return color.NRGBA{R: 92, G: 210, B: 190, A: 255}
	case "mp3", "wav", "flac", "aac", "ogg", "m4a", "mp4", "mkv", "mov", "avi", "webm":
		return color.NRGBA{R: 190, G: 138, B: 255, A: 255}
	case "zip", "tar", "gz", "tgz", "bz2", "xz", "7z", "rar":
		return color.NRGBA{R: 220, G: 176, B: 96, A: 255}
	case "exe", "dll", "app", "msi", "bat", "cmd", "com":
		return color.NRGBA{R: 235, G: 150, B: 92, A: 255}
	default:
		return color.NRGBA{R: 170, G: 176, B: 190, A: 255}
	}
}

func (m *filePaneModel) defaultFileIconWidget(name string) *widget.Icon {
	switch {
	case isImageFileExtension(name):
		return filenameRuleIcon(fm.FilenameIconImage)
	case isVideoFileExtension(name):
		return filenameRuleIcon(fm.FilenameIconVideo)
	case isArchiveFileExtension(name):
		return filenameRuleIcon(fm.FilenameIconArchive)
	default:
		return nil
	}
}

func isArchiveFileExtension(name string) bool {
	ext := fileExtension(name)
	switch ext {
	case "zip", "tar", "gz", "tgz", "bz2", "xz", "7z", "rar":
		return true
	default:
		return false
	}
}

func isImageFileExtension(name string) bool {
	ext := fileExtension(name)
	switch ext {
	case "png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "tif", "tiff", "ico":
		return true
	default:
		return false
	}
}

func isVideoFileExtension(name string) bool {
	ext := fileExtension(name)
	switch ext {
	case "mp4", "mkv", "mov", "avi", "webm":
		return true
	default:
		return false
	}
}

func fileExtension(name string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
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
	ui.stopPathEditExcept(idx)
	ui.closeSortMenusExcept(idx)
	ui.closeDriveMenusExcept(idx)
	ui.closeFavoriteMenusExcept(idx)
	ui.closeContextMenusExcept(idx)
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
		pane.closeSortMenu()
	}
}

func (ui *UI) closeFavoriteMenusExcept(active int) {
	for i, pane := range ui.filePanes {
		if pane == nil || i == active {
			continue
		}
		pane.closeFavoriteMenu()
	}
}

func (ui *UI) closeDriveMenusExcept(active int) {
	for i, pane := range ui.filePanes {
		if pane == nil || i == active {
			continue
		}
		pane.closeDriveMenu()
	}
}

func (ui *UI) closeContextMenusExcept(active int) {
	for i, pane := range ui.filePanes {
		if pane == nil || i == active {
			continue
		}
		pane.closeContextMenu()
	}
}

func (ui *UI) stopPathEditExcept(active int) {
	for i, pane := range ui.filePanes {
		if pane == nil || i == active {
			continue
		}
		pane.stopPathEdit()
		pane.stopInlineNameEdit()
	}
}

func (ui *UI) pathEditActive() bool {
	for _, pane := range ui.filePanes {
		if pane != nil && (pane.pathEditing || pane.inlineNameEditing) {
			return true
		}
	}
	return false
}

func (ui *UI) openPaneDriveMenu(idx int) bool {
	if idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.remoteConnected() || localDriveRoot(pane.displayDir()) == "" {
		return false
	}
	if len(platform.AvailableLocalDrives()) == 0 {
		return false
	}
	ui.setActiveFilePane(idx)
	ui.closeSortMenusExcept(idx)
	ui.closeDriveMenusExcept(idx)
	ui.closeFavoriteMenusExcept(idx)
	ui.closeContextMenusExcept(idx)
	pane.closeSortMenu()
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	pane.stopPathEdit()
	pane.openDriveMenu(image.Point{
		X: 0,
		Y: pane.headerHeight + 4,
	}, time.Now())
	pane.setDriveMenuSelection(-1, platform.AvailableLocalDrives())
	return true
}

func (ui *UI) openDriveMenuPane() (int, *filePaneState) {
	if ui == nil {
		return -1, nil
	}
	for idx, pane := range ui.filePanes {
		if pane != nil && pane.driveMenuOpen {
			return idx, pane
		}
	}
	return -1, nil
}

func (ui *UI) activatePaneDriveMenuSelection(idx int, pane *filePaneState, drives []string) bool {
	if ui == nil || pane == nil || len(drives) == 0 {
		return false
	}
	selected := pane.currentDriveMenuSelection(drives)
	if selected < 0 || selected >= len(drives) {
		return false
	}
	pane.closeDriveMenu()
	return ui.requestPaneLoadWithSelection(idx, drives[selected], "", "", 0)
}

func sendFilePaneLoadResult(ch chan filePaneLoadResult, res filePaneLoadResult) {
	if ch == nil {
		return
	}
	select {
	case ch <- res:
	default:
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- res:
		default:
		}
	}
}

func startLocalPaneLoad(pane *filePaneState, dir, primaryPath, secondaryPath string, fallbackRow int) bool {
	return startLocalPaneLoadWithRestore(pane, dir, primaryPath, secondaryPath, fallbackRow, layout.Position{}, false, "", "", 0)
}

func startLocalPaneLoadWithRestore(pane *filePaneState, dir, primaryPath, secondaryPath string, fallbackRow int, restorePos layout.Position, restoreScroll bool, restoreAnchor, noticeText string, noticeDur time.Duration) bool {
	return startLocalPaneLoadRequest(pane, dir, filePaneLoadResult{
		primaryPath:   primaryPath,
		secondaryPath: secondaryPath,
		fallbackRow:   fallbackRow,
		restorePos:    restorePos,
		restoreScroll: restoreScroll,
		restoreAnchor: restoreAnchor,
		noticeText:    noticeText,
		noticeDur:     noticeDur,
	}, false)
}

func startLocalPaneBackgroundRefresh(pane *filePaneState) bool {
	if pane == nil {
		return false
	}
	if !pane.dirWatchEligible() {
		return false
	}
	selectedPath := ""
	if sel := pane.selectedEntry(); sel != nil {
		selectedPath = sel.Path
	}
	fallbackRow := 0
	restorePos := layout.Position{}
	if pane.table != nil {
		fallbackRow = pane.table.Selected
		restorePos = sanitizePaneListPosition(pane.table.List.Position)
	}
	return startLocalPaneLoadRequest(pane, pane.dir, filePaneLoadResult{
		primaryPath:   selectedPath,
		fallbackRow:   fallbackRow,
		restorePos:    restorePos,
		restoreScroll: true,
		restoreAnchor: pane.visibleAnchorPath(),
		background:    true,
	}, true)
}

func startLocalPaneLoadRequest(pane *filePaneState, dir string, req filePaneLoadResult, quiet bool) bool {
	if pane == nil {
		return false
	}
	target := filepath.Clean(dir)
	pane.loadSeq++
	seq := pane.loadSeq
	pane.loading = true
	pane.loadQuiet = quiet
	pane.loadingDir = target
	pane.loadingStartedAt = time.Now()
	pane.err = ""
	pane.clearPendingPathNavigate()

	ch := pane.loadResultCh
	go func(targetDir string, currentSeq int, out filePaneLoadResult) {
		listing, err := filesys.ReadDir(targetDir)
		out.seq = currentSeq
		out.listing = listing
		out.err = err
		sendFilePaneLoadResult(ch, out)
	}(target, seq, req)
	return true
}

func (ui *UI) requestPaneLoadWithSelection(idx int, dir, primaryPath, secondaryPath string, fallbackRow int) bool {
	return ui.requestPaneLoadWithSelectionAndScroll(idx, dir, primaryPath, secondaryPath, fallbackRow, layout.Position{}, false, "", "", 0)
}

func (ui *UI) requestPaneLoadWithSelectionAndScroll(idx int, dir, primaryPath, secondaryPath string, fallbackRow int, restorePos layout.Position, restoreScroll bool, restoreAnchor, noticeText string, noticeDur time.Duration) bool {
	if idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return false
	}
	ui.setActiveFilePane(idx)

	pane.closeSortMenu()
	pane.closeFavoriteMenu()
	pane.closeDriveMenu()
	pane.closeContextMenu()
	pane.stopPathEdit()
	if pane.table != nil {
		pane.table.ResetPointerState()
	}
	pane.clearTableClickState()

	if pane.remoteConnected() {
		if err := pane.load(dir); err != nil {
			pane.setNotice(err.Error(), time.Now())
			return false
		}
		if restoreScroll {
			pane.table.List.Position = restorePaneListPosition(pane.model.entries, restorePos, restoreAnchor)
		}
		pane.applySelection(primaryPath, secondaryPath, fallbackRow, !restoreScroll)
		if noticeText != "" {
			pane.setNoticeFor(noticeText, time.Now(), noticeDur)
		}
		return true
	}
	return startLocalPaneLoadWithRestore(pane, dir, primaryPath, secondaryPath, fallbackRow, restorePos, restoreScroll, restoreAnchor, noticeText, noticeDur)
}

func (ui *UI) loadPaneDir(idx int, dir string) bool {
	return ui.requestPaneLoadWithSelection(idx, dir, "", "", 0)
}

func (ui *UI) activateFilePanePathSegment(idx int, pane *filePaneState, target string) bool {
	if ui == nil || pane == nil || strings.TrimSpace(target) == "" {
		return false
	}
	pane.clearPendingPathNavigate()
	pane.clearPathClickState()
	return ui.loadPaneDir(idx, target)
}

func (ui *UI) pumpFilePaneLoads(gtx layout.Context) {
	anyLoading := false
	for _, pane := range ui.filePanes {
		if pane == nil || pane.loadResultCh == nil {
			continue
		}
		for {
			select {
			case res := <-pane.loadResultCh:
				if res.seq != pane.loadSeq {
					continue
				}
				if res.err != nil {
					pane.loading = false
					pane.loadQuiet = false
					pane.loadingDir = ""
					pane.loadingStartedAt = time.Time{}
					pane.resetDirWatch()
					pane.setNotice(res.err.Error(), gtx.Now)
					gtx.Execute(op.InvalidateCmd{})
					continue
				}
				if res.background {
					pane.applyListingRefresh(res.listing, res.primaryPath, res.secondaryPath, res.fallbackRow, res.restorePos, res.restoreAnchor)
				} else {
					pane.applyListingWithRestore(res.listing, res.primaryPath, res.secondaryPath, res.fallbackRow, res.restorePos, res.restoreScroll, res.restoreAnchor)
				}
				if res.noticeText != "" {
					pane.setNoticeFor(res.noticeText, gtx.Now, res.noticeDur)
				}
				gtx.Execute(op.InvalidateCmd{})
			default:
				goto nextPane
			}
		}
	nextPane:
		if pane.loading {
			anyLoading = true
		}
	}
	if anyLoading {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(50 * time.Millisecond)})
	}
}

func (ui *UI) pumpFilePaneLocalRefresh(gtx layout.Context) {
	if ui == nil {
		return
	}
	nextCheck := time.Time{}
	for _, pane := range ui.filePanes {
		if pane == nil {
			continue
		}
		if !pane.dirWatchEligible() {
			pane.resetDirWatch()
			continue
		}
		if pane.dirWatch.nextCheck.IsZero() {
			pane.dirWatch.nextCheck = gtx.Now
		}
		if !gtx.Now.Before(pane.dirWatch.nextCheck) {
			pane.dirWatch.nextCheck = gtx.Now.Add(filePaneDirWatchPoll)
			if pane.dirWatchChanged() && startLocalPaneBackgroundRefresh(pane) {
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
		}
		if nextCheck.IsZero() || pane.dirWatch.nextCheck.Before(nextCheck) {
			nextCheck = pane.dirWatch.nextCheck
		}
	}
	if !nextCheck.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: nextCheck})
	}
}

func (ui *UI) submitPanePathEdit(idx int, raw string) bool {
	if idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return false
	}

	target := strings.TrimSpace(raw)
	if target == "" {
		pane.setNotice("path is empty", time.Now())
		return false
	}
	if pane.remoteConnected() {
		if !strings.HasPrefix(target, "/") {
			base := pane.dir
			if strings.TrimSpace(base) == "" {
				base = "/"
			}
			target = path.Join(base, target)
		}
		target = path.Clean(target)
		if target == path.Clean(pane.dir) {
			pane.stopPathEdit()
			return true
		}
		return ui.loadPaneDir(idx, target)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(pane.dir, target)
	}
	target = filepath.Clean(target)
	if target == filepath.Clean(pane.dir) {
		pane.stopPathEdit()
		return true
	}
	return ui.loadPaneDir(idx, target)
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
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	ui.rememberPaneSortForDirectory(idx)
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
	pane.closeSortMenu()
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	ui.rememberPaneSortForDirectory(idx)
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
	pane.closeSortMenu()
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	ui.rememberPaneSortForDirectory(idx)
}

func (ui *UI) rememberPaneSortForDirectory(idx int) {
	if ui == nil || idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return
	}
	dir := pane.sortDirConfigKey()
	if dir == "" {
		return
	}
	if err := ui.ensureFMConfigLoaded(); err != nil {
		pane.setNotice("failed to load config for sort: "+err.Error(), time.Now())
		return
	}
	if ui.fmCfg == nil {
		return
	}

	changed := ui.pruneSortPerDirOverrides(time.Now(), false)
	key := pane.sessionSortKey()
	desc := pane.sortDesc
	if fm.SortOrderIsDefault(ui.fmCfg.Sort, key, desc) {
		if _, ok := ui.fmCfg.Sort.PerDir[dir]; ok {
			delete(ui.fmCfg.Sort.PerDir, dir)
			changed = true
		}
	} else {
		code := fm.SortOrderCode(key, desc)
		if ui.fmCfg.Sort.PerDir == nil {
			ui.fmCfg.Sort.PerDir = make(map[string]string, 8)
		}
		if ui.fmCfg.Sort.PerDir[dir] != code {
			ui.fmCfg.Sort.PerDir[dir] = code
			changed = true
		}
	}
	if len(ui.fmCfg.Sort.PerDir) == 0 {
		ui.fmCfg.Sort.PerDir = nil
	}
	if !changed {
		return
	}
	if err := ui.saveFMConfigWithOptions("sort-dir", false); err != nil {
		pane.setNotice("failed to save sort: "+err.Error(), time.Now())
		return
	}
	ui.refreshFilePaneConfigRefs()
}

func (ui *UI) refreshFilePaneConfigRefs() {
	if ui == nil || ui.fmCfg == nil {
		return
	}
	for _, pane := range ui.filePanes {
		if pane != nil && pane.model != nil {
			pane.model.cfg = ui.fmCfg
		}
	}
}

func (ui *UI) pruneSortPerDirOverrides(now time.Time, force bool) bool {
	if ui == nil || ui.fmCfg == nil || len(ui.fmCfg.Sort.PerDir) == 0 {
		return false
	}
	if !force && !ui.sortDirPrunedAt.IsZero() && now.Sub(ui.sortDirPrunedAt) < filePaneSortDirPruneInterval {
		return false
	}
	ui.sortDirPrunedAt = now
	changed := false
	for dir, raw := range ui.fmCfg.Sort.PerDir {
		key, desc, ok := fm.ParseSortOrderCode(raw)
		if !ok || fm.SortOrderIsDefault(ui.fmCfg.Sort, key, desc) || !localSortDirExists(dir) {
			delete(ui.fmCfg.Sort.PerDir, dir)
			changed = true
		}
	}
	if len(ui.fmCfg.Sort.PerDir) == 0 {
		ui.fmCfg.Sort.PerDir = nil
	}
	return changed
}

func localSortDirExists(dir string) bool {
	dir = strings.TrimSpace(dir)
	if dir == "" || filesys.ArchivePathActive(dir) {
		return false
	}
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
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
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
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

func (ui *UI) queueFilePaneSystemOpen(idx, row int) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	ui.pendingFileOpen = &fileOpenRequest{
		pane:           idx,
		row:            row,
		systemOpenOnly: true,
	}
}

func (ui *UI) flushPendingFileOpen() bool {
	req := ui.pendingFileOpen
	if req == nil {
		return false
	}
	ui.pendingFileOpen = nil
	if req.systemOpenOnly {
		return ui.activateFilePaneDoubleClick(req.pane, req.row)
	}
	_ = ui.activateFilePaneRow(req.pane, req.row)
	return true
}

func (ui *UI) activateFilePaneDoubleClick(idx, row int) bool {
	if idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.model == nil {
		return false
	}
	entry := pane.model.Entry(row)
	if entry == nil {
		return false
	}
	switch entry.Kind {
	case filesys.EntryDir, filesys.EntryParent:
		return ui.activateFilePaneRow(idx, row)
	default:
		if entry.CanEnter {
			return ui.activateFilePaneRow(idx, row)
		}
		if pane.archiveBrowsing() || filesys.ArchiveMemberPath(entry.Path) {
			ui.startFileViewer(idx, time.Now())
			return true
		}
		ui.startFileSystemOpenAction(idx, row, time.Now())
		return true
	}
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
	pane.rememberDirScroll(prevDir)
	primaryPath := ""
	restorePos := layout.Position{}
	restoreScroll := false
	if entry.Kind == filesys.EntryParent {
		primaryPath = prevDir
		restorePos, restoreScroll = pane.restoreDirScroll(entry.Path)
	}
	if !ui.requestPaneLoadWithSelectionAndScroll(idx, entry.Path, primaryPath, "", 0, restorePos, restoreScroll, "", "", 0) {
		return false
	}
	return true
}

func (ui *UI) openFilePaneContextMenu(idx, row int, pos image.Point, now time.Time) {
	if idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return
	}
	ui.setActiveFilePane(idx)
	ui.closeSortMenusExcept(idx)
	ui.closeDriveMenusExcept(idx)
	ui.closeFavoriteMenusExcept(idx)
	ui.closeContextMenusExcept(idx)
	pane.closeSortMenu()
	pane.closeDriveMenu()
	pane.closeFavoriteMenu()
	pane.openContextMenu(row, pos, now)
}

func (ui *UI) prepareFilePaneContextMenuTarget(pane *filePaneState, row int) bool {
	if pane == nil || pane.table == nil || row < 0 {
		return false
	}
	total := 0
	if pane.model != nil {
		total = pane.model.Len()
	}
	selectionChanged := false
	if row != pane.table.Selected {
		prev := pane.table.Selected
		pane.table.SetSelected(row, total, false)
		if pane.table.OnSelect != nil && prev != pane.table.Selected {
			pane.table.OnSelect(pane.table.Selected)
		}
		selectionChanged = prev != pane.table.Selected
	}
	if !pane.isMarkedRow(row) && pane.clearMarkedRows() {
		selectionChanged = true
	}
	return selectionChanged
}

func (ui *UI) openFilePaneContextMenuAtPointer(idx int, pane *filePaneState, pos image.Point, now time.Time) bool {
	if pane == nil || pane.table == nil {
		return false
	}
	total := 0
	if pane.model != nil {
		total = pane.model.Len()
	}
	row := pane.table.HitRow(pos, total)
	selectionChanged := ui.prepareFilePaneContextMenuTarget(pane, row)
	ui.openFilePaneContextMenu(idx, row, pos, now)
	return selectionChanged
}
