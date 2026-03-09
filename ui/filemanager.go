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
	entries       []filesys.Entry
	cfg           *fm.Config
	baseTextColor color.NRGBA
}

type fileSortKey uint8

const (
	fileSortName fileSortKey = iota
	fileSortExt
	fileSortSize
	fileSortDate
	filePaneApproxCharPx  = 8
	filePaneNameTailRunes = 3
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
	}

	showPerms := m.showPermissionColumn()
	switch c {
	case 0:
		return entry.DisplayName, st
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
		return m.nameOrEmpty(entry.DisplayName, widthPx), st
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
	switch entry.Kind {
	case filesys.EntryParent:
		return table.LeadingIcon{
			Kind:  table.IconParent,
			Color: color.NRGBA{R: 170, G: 200, B: 255, A: 255},
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
		return table.LeadingIcon{
			Kind:  table.IconFile,
			Color: m.fileIconColor(entry.Name),
		}, true
	}
}

type filePaneState struct {
	table                *table.Table
	model                *filePaneModel
	pathSegClicks        []widget.Clickable
	pathEdit             widget.Editor
	pathEditing          bool
	pathEditFocus        bool
	inlineNameEdit       widget.Editor
	inlineNameEditing    bool
	inlineNameFocus      bool
	inlineNameRow        int
	inlineNamePath       string
	inlineNameOriginal   string
	inlineNameRect       image.Rectangle
	inlineNamePendingRow int
	inlineNamePendingAt  time.Time
	pathClickKey         string
	pathClickAt          time.Time
	pendingPathNav       string
	pendingPathAt        time.Time
	tableClickRow        int
	tableClickCol        int
	tableClickAt         time.Time
	pathRowClick         widget.Clickable
	modeClick            widget.Clickable
	sortClick            widget.Clickable
	favoriteClick        widget.Clickable
	disconnectClick      widget.Clickable
	tablePointerTag      uiEventTag
	sortOptionBtns       [4]widget.Clickable
	sortMenuOpen         bool
	favoritePointerTag   uiEventTag
	favoriteMenuClick    widget.Clickable
	favoriteOptionClicks []widget.Clickable
	favoriteRemoveClicks []widget.Clickable
	favoriteMenuOpen     bool
	favoriteMenuRect     image.Rectangle
	favoriteHoverKey     string
	favoriteHoverLabel   string
	favoriteHoverAt      time.Time
	headerHeight         int
	ctxPointerTag        uiEventTag
	ctxMenuClicks        []widget.Clickable
	ctxMenuOpen          bool
	ctxMenuRow           int
	ctxMenuPos           image.Point
	ctxMenuRect          image.Rectangle
	drivePointerTag      uiEventTag
	driveMenuPointerTag  uiEventTag
	driveMenuClicks      []widget.Clickable
	driveMenuOpen        bool
	driveMenuPos         image.Point
	driveMenuRect        image.Rectangle
	driveSegmentRect     image.Rectangle
	sortKey              fileSortKey
	sortDesc             bool
	dirsFirst            bool
	remote               *paneSSHSession
	localDirBeforeRemote string
	dir                  string
	loading              bool
	loadingDir           string
	loadingStartedAt     time.Time
	loadSeq              int
	loadResultCh         chan filePaneLoadResult
	err                  string
	noticeText           string
	noticeUntil          time.Time
	markedRows           map[int]struct{}
}

type filePaneLoadResult struct {
	seq           int
	listing       filesys.Listing
	err           error
	primaryPath   string
	secondaryPath string
	fallbackRow   int
}

func newFilePaneState(dir string, cfg *fm.Config) *filePaneState {
	if cfg == nil {
		cfg = fm.DefaultConfig()
	}
	scaleDp := func(v int) unit.Dp {
		return scaleFilePaneDp(cfg, v)
	}
	fullPad := unit.Dp(cfg.Columns.FullPadDp)
	if fullPad < 0 {
		fullPad = 0
	}
	dropPriority := filePaneFullDropPriority(cfg)
	cols := []table.Column{
		{
			Width:        scaleDp(cfg.Columns.NameWidthDp),
			MinWidth:     scaleDp(cfg.Columns.NameMinWidthDp),
			Flex:         true,
			Align:        table.AlignStart,
			PadX:         fullPad,
			DropPriority: dropPriority["name"],
		},
	}
	if cfg.Columns.ShowPermissions {
		cols = append(cols, table.Column{
			Width:        scaleDp(cfg.Columns.PermWidthDp),
			MinWidth:     scaleDp(fm.PermMinWidthDp(cfg)),
			Flex:         false,
			Align:        table.AlignStart,
			PadX:         fullPad,
			DropPriority: dropPriority["permissions"],
		})
	}
	cols = append(cols,
		table.Column{
			Width:        scaleDp(cfg.Columns.SizeWidthDp),
			MinWidth:     scaleDp(fm.SizeMinWidthDp(cfg)),
			Flex:         false,
			Align:        table.AlignEnd,
			PadX:         fullPad,
			DropPriority: dropPriority["size"],
		},
		table.Column{
			Width:        scaleDp(cfg.Columns.DateWidthDp),
			MinWidth:     scaleDp(fm.DateMinWidthDp(cfg)),
			Flex:         false,
			Align:        table.AlignStart,
			PadX:         fullPad,
			DropPriority: dropPriority["date"],
		},
	)

	pane := &filePaneState{
		table:        table.New(cols),
		model:        &filePaneModel{cfg: cfg},
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
	pane.table.TextSize = scaleConfigFontSize(cfg, 13)
	pane.table.RowHeight = scaleDp(18)
	pane.table.RowPadY = unit.Dp(0)
	pane.table.BriefColumnWidth = scaleDp(cfg.Columns.BriefWidthDp)
	pane.table.BriefGap = scaleDp(cfg.Columns.BriefGapDp)
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
	if p == nil {
		return
	}
	p.dir = listing.Dir
	if p.remote == nil && p.dir != "" {
		p.localDirBeforeRemote = p.dir
	}
	p.loading = false
	p.loadingDir = ""
	p.loadingStartedAt = time.Time{}
	p.err = ""
	p.noticeText = ""
	p.noticeUntil = time.Time{}
	p.stopInlineNameEdit()
	p.clearMarkedRows()
	p.clearPathClickState()
	p.clearPendingPathNavigate()
	p.model.entries = listing.Entries
	p.applySort("")
	p.table.Selected = 0
	p.table.List.Position = layout.Position{}
	p.applySelection(primaryPath, secondaryPath, fallbackRow)
}

func (p *filePaneState) applySelection(primaryPath, secondaryPath string, fallbackRow int) {
	if p == nil || p.table == nil || p.model == nil || p.model.Len() == 0 {
		return
	}
	if primaryPath != "" {
		if idx := p.findEntryPathIndex(primaryPath); idx >= 0 {
			p.table.SetSelected(idx, p.model.Len(), false)
			return
		}
	}
	if secondaryPath != "" {
		if idx := p.findEntryPathIndex(secondaryPath); idx >= 0 {
			p.table.SetSelected(idx, p.model.Len(), false)
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
	p.table.SetSelected(row, p.model.Len(), false)
}

func (p *filePaneState) setNotice(msg string, now time.Time) {
	if p == nil || msg == "" {
		return
	}
	p.err = ""
	p.noticeText = msg
	p.noticeUntil = now.Add(3 * time.Second)
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
	p.ctxMenuRect = image.Rectangle{}
}

func (p *filePaneState) closeFavoriteMenu() {
	if p == nil {
		return
	}
	p.favoriteMenuOpen = false
	p.favoriteMenuRect = image.Rectangle{}
	p.favoriteHoverKey = ""
	p.favoriteHoverLabel = ""
	p.favoriteHoverAt = time.Time{}
}

func (p *filePaneState) closeDriveMenu() {
	if p == nil {
		return
	}
	p.driveMenuOpen = false
	p.driveMenuRect = image.Rectangle{}
}

func (p *filePaneState) openContextMenu(row int, pos image.Point) {
	if p == nil {
		return
	}
	p.ctxMenuOpen = true
	p.ctxMenuRow = row
	p.ctxMenuPos = pos
	p.ctxMenuRect = image.Rectangle{}
}

func (p *filePaneState) openDriveMenu(pos image.Point) {
	if p == nil {
		return
	}
	p.driveMenuOpen = true
	p.driveMenuPos = pos
	p.driveMenuRect = image.Rectangle{}
}

func (p *filePaneState) ensureContextMenuClicks(n int) {
	if n <= cap(p.ctxMenuClicks) {
		p.ctxMenuClicks = p.ctxMenuClicks[:n]
		return
	}
	old := p.ctxMenuClicks
	p.ctxMenuClicks = make([]widget.Clickable, n)
	copy(p.ctxMenuClicks, old)
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

func (p *filePaneState) markCurrentAndAdvance() bool {
	if p == nil || p.table == nil || p.model == nil {
		return false
	}
	total := p.model.Len()
	if total <= 0 {
		return false
	}
	changed := p.markRow(p.table.Selected)
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
	if p == nil || !p.loading || p.remoteConnected() || p.loadingStartedAt.IsZero() {
		return false
	}
	return !now.Before(p.loadingStartedAt.Add(filePaneLoadingHintDelay))
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

type fileContextMenuSpec struct {
	title string
	items []string
}

type fileFavoriteItem struct {
	label      string
	targetDir  string
	addCurrent bool
	active     bool
	disabled   bool
	removable  bool
}

func (p *filePaneState) contextMenuSpec() fileContextMenuSpec {
	entry := p.contextMenuEntry()
	if entry == nil {
		return fileContextMenuSpec{
			title: "This Folder",
			items: []string{
				"New Folder",
				"New File",
				"Paste",
				"Refresh",
				"Properties",
			},
		}
	}

	title := entry.DisplayName
	if title == "" {
		title = entry.Name
	}
	if title == "" {
		title = entry.Path
	}

	switch entry.Kind {
	case filesys.EntryParent:
		return fileContextMenuSpec{
			title: title,
			items: []string{
				"Open",
				"Open in Other Pane",
				"Copy Path",
				"Properties",
			},
		}
	case filesys.EntryDir:
		return fileContextMenuSpec{
			title: title,
			items: []string{
				"Open",
				"Open in Other Pane",
				"Rename",
				"Copy",
				"Cut",
				"Delete",
				"Copy Path",
				"Properties",
			},
		}
	default:
		return fileContextMenuSpec{
			title: title,
			items: []string{
				"Open",
				"Open With...",
				"Rename",
				"Copy",
				"Cut",
				"Delete",
				"Copy Path",
				"Properties",
			},
		}
	}
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
		port == loc.Port &&
		normalizeRemoteFavoriteDir(pane.dir) == loc.Dir
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
	items = append(items, fileFavoriteItem{
		label:      "Add current dir",
		addCurrent: true,
	})

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
	return items
}

func (ui *UI) addFavoriteLocation(raw string) (string, bool, error) {
	if ui == nil {
		return "", false, nil
	}
	if ui.fmCfg == nil {
		ui.fmCfg = fm.DefaultConfig()
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
	if err := ui.saveFMConfig(); err != nil {
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
	if err := ui.saveFMConfig(); err != nil {
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
	markedPaths := p.markedPaths()

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
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
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
		pane.sortMenuOpen = false
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
	pane.sortMenuOpen = false
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	pane.stopPathEdit()
	pane.openDriveMenu(image.Point{
		X: 0,
		Y: pane.headerHeight + 4,
	})
	return true
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
	if pane == nil {
		return false
	}
	target := filepath.Clean(dir)
	pane.loadSeq++
	seq := pane.loadSeq
	pane.loading = true
	pane.loadingDir = target
	pane.loadingStartedAt = time.Now()
	pane.err = ""
	pane.clearPendingPathNavigate()

	ch := pane.loadResultCh
	go func(targetDir, wantPrimary, wantSecondary string, wantRow, currentSeq int) {
		listing, err := filesys.ReadDir(targetDir)
		sendFilePaneLoadResult(ch, filePaneLoadResult{
			seq:           currentSeq,
			listing:       listing,
			err:           err,
			primaryPath:   wantPrimary,
			secondaryPath: wantSecondary,
			fallbackRow:   wantRow,
		})
	}(target, primaryPath, secondaryPath, fallbackRow, seq)
	return true
}

func (ui *UI) requestPaneLoadWithSelection(idx int, dir, primaryPath, secondaryPath string, fallbackRow int) bool {
	if idx < 0 || idx >= len(ui.filePanes) {
		return false
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return false
	}
	ui.setActiveFilePane(idx)

	pane.sortMenuOpen = false
	pane.closeFavoriteMenu()
	pane.closeDriveMenu()
	pane.closeContextMenu()
	pane.stopPathEdit()

	if pane.remoteConnected() {
		if err := pane.load(dir); err != nil {
			pane.setNotice(err.Error(), time.Now())
			return false
		}
		pane.applySelection(primaryPath, secondaryPath, fallbackRow)
		return true
	}
	return startLocalPaneLoad(pane, dir, primaryPath, secondaryPath, fallbackRow)
}

func (ui *UI) loadPaneDir(idx int, dir string) bool {
	return ui.requestPaneLoadWithSelection(idx, dir, "", "", 0)
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
					pane.loadingDir = ""
					pane.loadingStartedAt = time.Time{}
					pane.setNotice(res.err.Error(), gtx.Now)
					gtx.Execute(op.InvalidateCmd{})
					continue
				}
				pane.applyListing(res.listing, res.primaryPath, res.secondaryPath, res.fallbackRow)
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
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
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
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
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

func (ui *UI) flushPendingFileOpen() bool {
	req := ui.pendingFileOpen
	if req == nil {
		return false
	}
	ui.pendingFileOpen = nil
	_ = ui.activateFilePaneRow(req.pane, req.row)
	return true
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
	primaryPath := ""
	if entry.Kind == filesys.EntryParent {
		primaryPath = prevDir
	}
	if !ui.requestPaneLoadWithSelection(idx, entry.Path, primaryPath, "", 0) {
		return false
	}
	return true
}

func (ui *UI) openFilePaneContextMenu(idx, row int, pos image.Point) {
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
	pane.sortMenuOpen = false
	pane.closeDriveMenu()
	pane.closeFavoriteMenu()
	pane.openContextMenu(row, pos)
}
