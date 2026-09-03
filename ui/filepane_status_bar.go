// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"image/color"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"hexone/fm"
	"hexone/ui/widget/table"

	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type filePaneStatusBarSeparatorMode uint8

const (
	filePaneStatusBarSeparatorNone filePaneStatusBarSeparatorMode = iota
	filePaneStatusBarSeparatorLeading
	filePaneStatusBarSeparatorTrailing
)

// filePaneStatusBarSeparatorMode reports which pane-seam separator a PROGRESS
// line carries. The file info line no longer has one: Revision 2's layout
// anchors its free-space region to the pane's right edge, which is what the
// seam separator used to visually terminate, so only the extraction and paste
// lines still draw it.
func (ui *UI) filePaneStatusBarSeparatorMode(idx int) filePaneStatusBarSeparatorMode {
	if ui == nil || len(ui.filePanes) <= 1 {
		return filePaneStatusBarSeparatorNone
	}
	if idx <= 0 {
		return filePaneStatusBarSeparatorTrailing
	}
	return filePaneStatusBarSeparatorLeading
}

// filePaneStatusBarBranch names which of the strip's three contents a pane
// shows. One strip serves all three, so they are ranked rather than stacked.
type filePaneStatusBarBranch uint8

const (
	filePaneStatusBarBranchNone filePaneStatusBarBranch = iota
	filePaneStatusBarBranchArchiveExtract
	filePaneStatusBarBranchDirectPaste
	filePaneStatusBarBranchFileInfo
)

// filePaneStatusBarBranch picks the strip's content for a pane. Progress
// outranks the file info line, and the two progress branches are deliberately
// not gated on StatusBarConfig: turning the info bar off is a display
// preference, and it must never take extraction or paste feedback with it.
func (ui *UI) filePaneStatusBarBranch(idx int, pane *filePaneState) filePaneStatusBarBranch {
	if ui == nil || pane == nil {
		return filePaneStatusBarBranchNone
	}
	if ui.archiveExtractPane() == pane {
		return filePaneStatusBarBranchArchiveExtract
	}
	if ui.directFilePasteForPane(idx) != nil {
		return filePaneStatusBarBranchDirectPaste
	}
	if filePaneStatusBarVisible(ui.fmCfg, pane) {
		return filePaneStatusBarBranchFileInfo
	}
	return filePaneStatusBarBranchNone
}

// filePaneStatusBarInsetX is the strip's horizontal text inset. The info
// line's two regions anchor to it: the name column starts at the left inset
// and the free-space text ends at the right one.
const filePaneStatusBarInsetX = unit.Dp(8)

func (ui *UI) layoutFilePaneStatusBar(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState, palette filePanePalette) layout.Dimensions {
	branch := ui.filePaneStatusBarBranch(idx, pane)
	if branch == filePaneStatusBarBranchNone {
		return layout.Dimensions{}
	}
	if branch == filePaneStatusBarBranchFileInfo {
		return ui.layoutFilePaneStatusBarInfo(th, gtx, pane, palette)
	}

	// Progress branches (extraction, paste). Their single-string rendering and
	// pane-seam separators are Revision 1 behaviour kept deliberately intact.
	leftInset := filePaneStatusBarInsetX
	rightInset := filePaneStatusBarInsetX
	mode := ui.filePaneStatusBarSeparatorMode(idx)
	trailingSeparator := mode == filePaneStatusBarSeparatorTrailing
	switch mode {
	case filePaneStatusBarSeparatorLeading:
		leftInset = 0
	case filePaneStatusBarSeparatorTrailing:
		rightInset = 0
	}
	measure := func(text string) int {
		return ui.measureFilePaneStatusBarTextWidth(th, gtx, text)
	}
	textMax := gtx.Constraints.Max.X - gtx.Dp(leftInset) - gtx.Dp(rightInset)
	if textMax < 0 {
		textMax = 0
	}
	label := ""
	switch branch {
	case filePaneStatusBarBranchArchiveExtract:
		label = archiveExtractStatusLineForWidth(ui.archiveExtract, gtx.Now, textMax, measure)
		if mode != filePaneStatusBarSeparatorNone {
			label = archiveExtractStatusLineWithSeparatorForWidth(ui.archiveExtract, gtx.Now, textMax, measure, trailingSeparator)
		}
	case filePaneStatusBarBranchDirectPaste:
		directPaste := ui.directFilePasteForPane(idx)
		label = directFilePasteStatusLineForWidth(directPaste, gtx.Now, textMax, measure)
		if mode != filePaneStatusBarSeparatorNone {
			label = directFilePasteStatusLineWithSeparatorForWidth(directPaste, gtx.Now, textMax, measure, trailingSeparator)
		}
	}
	if strings.TrimSpace(label) == "" {
		// A progress branch with nothing to say has nothing to draw: the
		// extraction or paste simply is not running, so there is no strip.
		return layout.Dimensions{}
	}
	// Only the progress lines carry a live elapsed/percentage readout worth
	// repainting for. The file info line changes solely on cursor, listing
	// and marking events, which already invalidate the frame; its one timed
	// refresh, the volume poll, is scheduled by pumpFilePaneVolumeLookups.
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(archiveExtractStatusRefreshInterval)})

	bg, border, textColor := filePaneVolumeBadgeColors(palette)
	return layoutFilePaneStatusBarBox(gtx, bg, border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: leftInset, Right: rightInset, Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := ui.filePaneStatusBarLabel(th, label, textColor)
			lbl.Truncator = ""
			if trailingSeparator {
				// layout.E needs a Min width to align against; the pane's Stack
				// has no Stacked children, so Gio hands us Min.X == 0 and E
				// would collapse to natural width, leaving the seam separator
				// floating mid-pane.
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.E.Layout(gtx, lbl.Layout)
			}
			return lbl.Layout(gtx)
		})
	})
}

// layoutFilePaneStatusBarInfo renders the file info line as Revision 2's
// anchored two-region row: the name-anchored left cluster of fixed columns,
// a flexed spacer, and the right-anchored free-space region.
//
// Only reached through filePaneStatusBarBranch, which returns the file info
// branch only when filePaneStatusBarVisible accepted ui.fmCfg — and that
// rejects a nil config — so the ui.fmCfg dereference below is safe.
//
// No frame tick is scheduled here: the info line changes only on cursor,
// listing, marking and volume events, all of which already invalidate the
// frame (the volume poll's wakeup belongs to pumpFilePaneVolumeLookups).
func (ui *UI) layoutFilePaneStatusBarInfo(th *material.Theme, gtx layout.Context, pane *filePaneState, palette filePanePalette) layout.Dimensions {
	configured := filePaneStatusFields(ui.fmCfg.StatusBar.Fields)

	freeLabel := ""
	if slices.Contains(configured, filePaneStatusFieldFree) {
		// Calling this marks the pane as wanting a reading, which is what keeps
		// pumpFilePaneVolumeLookups polling it; the returned label is deliberately
		// ignored in favour of the raw bytes, which formatFilePaneStatusFree
		// renders in the bar's own "<free> free (<pct>%)" form — not the badge's.
		//
		// It performs no I/O and schedules no refresh — the pump owns the poll
		// cadence for both presentations of free space. Until the first lookup
		// lands, totalBytes stays 0, formatFilePaneStatusFree returns "" and the
		// free region simply is not there yet, which is the designed degradation.
		if _, ok := ui.filePaneVolumeBadgeLabel(gtx, pane); ok {
			freeLabel = formatFilePaneStatusFree(pane.volumeBadge.freeBytes, pane.volumeBadge.totalBytes)
		}
	}

	bg, border, textColor := filePaneVolumeBadgeColors(palette)
	return layoutFilePaneStatusBarBox(gtx, bg, border, func(gtx layout.Context) layout.Dimensions {
		inset := layout.Inset{
			Left:   filePaneStatusBarInsetX,
			Right:  filePaneStatusBarInsetX,
			Top:    unit.Dp(4),
			Bottom: unit.Dp(4),
		}
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutFilePaneStatusBarInfoRow(th, gtx, pane, configured, freeLabel, textColor)
		})
	})
}

// Separators of the info row. The column separator joins the left cluster's
// fixed columns and renders as text. The region separator — the "│" between
// the left cluster and the free region — is only MEASURED as text: its width
// sample carries two spaces of air on each side of the bar so the free region
// never butts against the last column even when the row is exactly full. It
// renders as a painted rule of exactly that width instead of the glyph,
// because U+2502 is a box-drawing character designed to span the whole line
// box (so bars connect across lines) and its ink overflows the strip's
// vertical insets; see layoutFilePaneStatusBarRegionRule.
const (
	filePaneStatusColumnSeparator = "  •  "
	filePaneStatusRegionSeparator = "  │  "
	filePaneStatusRegionRuleGap   = "  "
)

// filePaneStatusSizeSample is the size column's width sample (per the design
// table). Rare wider values ("1023.99 KB") ellipsise inside the column rather
// than widen it, keeping the column a property of the configuration alone.
const filePaneStatusSizeSample = "888.88 MB"

// Permission column samples, one per format (design table).
const (
	filePaneStatusPermsSymbolicSample = "-rw-r--r--"
	filePaneStatusPermsOctalSample    = "8888"
)

// filePaneStatusOwnerCapSample caps the owner column: a pathological owner
// string (an LDAP DN, say) must not push every other column off the pane.
// Twenty-four digit-width glyphs comfortably hold "username:groupname" forms.
var filePaneStatusOwnerCapSample = strings.Repeat("8", 24)

// filePaneStatusNameFloorTail is what must still fit beside the compaction
// marker for the name column to be worth keeping: one head rune plus the
// three tail runes compactName preserves (filePaneNameTailRunes) — enough for
// the marker-plus-extension form ("g….go") that still identifies a file.
// Below that the name stops saying anything, so columns drop instead.
const filePaneStatusNameFloorTail = "8888"

// filePaneStatusDateSampleTime is the wide sample timestamp the date column is
// measured on: two-digit day, a late-evening time, year 2088 for wide digit
// glyphs — and September, the longest English month name, so a month-name
// layout (a hand-configured "January 02 2006", say) is measured on its widest
// month. A narrower sample month would size the column under real September
// dates and ellipsise them.
var filePaneStatusDateSampleTime = time.Date(2088, time.September, 28, 22, 58, 48, 0, time.Local)

// filePaneStatusDateSample renders the date column's width sample in the
// CHOSEN layout — the same one filePaneStatusFieldValue's date path renders
// values in. A fixed status_bar.date_format key measures its own layout;
// "auto" measures the configured DateFormats[0] (the format formatDate
// resolves to at filePaneStatusDateWidthPx), with formatDate's own fallback
// for an empty config. All on the wide sample timestamp.
func filePaneStatusDateSample(m *filePaneModel) string {
	format := "Jan 02 2006"
	if m != nil && m.cfg != nil {
		if layout := fm.StatusBarDateLayout(m.cfg.StatusBar.DateFormat); layout != "" {
			format = layout
		} else if len(m.cfg.DateFormats) > 0 {
			format = m.cfg.DateFormats[0]
		}
	}
	return filePaneStatusDateSampleTime.Format(format)
}

// filePaneStatusPermsSample picks the permission column's width sample for the
// configured format, mirroring defaultPermissionText's octal-or-symbolic split.
func filePaneStatusPermsSample(m *filePaneModel) string {
	if m.permissionFormat() == "octal" {
		return filePaneStatusPermsOctalSample
	}
	return filePaneStatusPermsSymbolicSample
}

// filePaneStatusCompactMarker resolves the name-compaction marker the same way
// compactName does, so the floor is measured on the marker that will render.
func filePaneStatusCompactMarker(m *filePaneModel) string {
	if m != nil && m.cfg != nil && m.cfg.NameCompact.Marker != "" {
		return m.cfg.NameCompact.Marker
	}
	return ".."
}

// filePaneStatusCompactKeepStart resolves NameCompact.KeepStartChars the same
// way compactName does (its headMin default is 6), so the measure key tracks
// the head length the cached compacted cursor name was built with. The marker
// alone is not enough: compactName reads both, and the fit cache is keyed on
// (name, column width) only, so without this a KeepStartChars change would
// keep serving the old head length until the next cursor move or resize.
func filePaneStatusCompactKeepStart(m *filePaneModel) int {
	if m != nil && m.cfg != nil && m.cfg.NameCompact.KeepStartChars > 0 {
		return m.cfg.NameCompact.KeepStartChars
	}
	return 6
}

// filePaneStatusColumnWidths are the fixed column widths of the info row, in
// pixels. They are a property of the directory listing and the configuration —
// never of the selected entry — which is what keeps the cursor from shifting
// anything (the design's no-jump rule). All widths are exclusive of separators.
type filePaneStatusColumnWidths struct {
	// namePx is the widest display name in the listing, uncapped; the plan caps
	// it by whatever the other columns and the free region leave over.
	namePx int
	// Measured samples (see the constants above).
	sizePx  int
	datePx  int
	permsPx int
	// ownerPx is the widest owner text in the listing, capped.
	ownerPx int
	// Separator widths and the name column's floor. ruleGapPx is the air on
	// each side of the region rule, carved out of regionSepPx.
	sepPx       int
	regionSepPx int
	ruleGapPx   int
	floorPx     int
}

// computeFilePaneStatusColumnWidths measures the info row's column widths for
// the pane's current listing. This is the one O(entries) pass of the feature:
// callers cache the result and rerun it only when the listing, the metrics or
// the relevant configuration change (see filePaneStatusBarMeasure).
func computeFilePaneStatusColumnWidths(pane *filePaneState, measure func(string) int) filePaneStatusColumnWidths {
	var w filePaneStatusColumnWidths
	if pane == nil || pane.model == nil || measure == nil {
		return w
	}
	m := pane.model

	w.sepPx = measure(filePaneStatusColumnSeparator)
	w.regionSepPx = measure(filePaneStatusRegionSeparator)
	w.ruleGapPx = measure(filePaneStatusRegionRuleGap)
	w.floorPx = measure(filePaneStatusCompactMarker(m) + filePaneStatusNameFloorTail)
	w.sizePx = measure(filePaneStatusSizeSample)
	w.datePx = measure(filePaneStatusDateSample(m))
	w.permsPx = measure(filePaneStatusPermsSample(m))

	// Owners repeat heavily within a directory, so measuring is deduped by
	// string; names are mostly unique and measured as they come, exactly like
	// BriefColumnTextWidthPx's per-listing scan.
	ownerWidths := make(map[string]int)
	for i := range m.entries {
		entry := &m.entries[i]
		name := entry.DisplayName
		if name == "" {
			name = entry.Name
		}
		if name != "" {
			if px := measure(name); px > w.namePx {
				w.namePx = px
			}
		}
		if owner := strings.TrimSpace(entry.OwnerText); owner != "" {
			px, ok := ownerWidths[owner]
			if !ok {
				px = measure(owner)
				ownerWidths[owner] = px
			}
			if px > w.ownerPx {
				w.ownerPx = px
			}
		}
	}
	if ownerCap := measure(filePaneStatusOwnerCapSample); w.ownerPx > ownerCap {
		w.ownerPx = ownerCap
	}
	return w
}

// filePaneStatusFieldColumnPx maps a left-cluster field to its column width.
// Free space is not a left column — it is the right region — so it has no
// width here.
func filePaneStatusFieldColumnPx(w filePaneStatusColumnWidths, field filePaneStatusField) int {
	switch field {
	case filePaneStatusFieldSize:
		return w.sizePx
	case filePaneStatusFieldDate:
		return w.datePx
	case filePaneStatusFieldPerms:
		return w.permsPx
	case filePaneStatusFieldOwner:
		return w.ownerPx
	}
	return 0
}

// filePaneStatusBarPlan is one frame's layout decision for the info row: which
// field columns survive at this width, how wide the name column is, and
// whether the free-space region renders. Everything in it derives from the
// listing, the configuration and the pane width — never from the cursor.
type filePaneStatusBarPlan struct {
	// fields are the surviving left-cluster columns in display order, with
	// fieldPx their widths, index-aligned. Free space is never in here.
	fields  []filePaneStatusField
	fieldPx []int
	// nameColPx is the name column's width; zero means no name column (an
	// empty listing, or no room at all).
	nameColPx int
	// showFree reports whether the right region renders this frame.
	showFree bool
}

// buildFilePaneStatusBarPlan fits the configured fields into availPx.
//
// The name column absorbs shrinkage first: it is granted the widest display
// name's width when everything fits, and gives that up down to the floor
// (marker plus extension tail — see filePaneStatusNameFloorTail) before any
// column drops. Below the floor, whole fields drop in filePaneStatusDropOrder
// — free space as a whole, since Revision 2 retired its shorter forms — and
// the name outlives every one of them: it is the anchor.
//
// freePx <= 0 means no free region at all: the field is not configured, or the
// volume lookup has not landed yet. It contributes nothing to the budget then,
// so the columns do not reserve space for a value that is not being shown.
//
// enabled is never written through: the drops walk filePaneStatusDropNext,
// whose full slice expression keeps the caller's slice intact.
func buildFilePaneStatusBarPlan(w filePaneStatusColumnWidths, enabled []filePaneStatusField, freePx, availPx int) filePaneStatusBarPlan {
	active := enabled
	for {
		fixed := 0
		freeOn := false
		for _, field := range active {
			if field == filePaneStatusFieldFree {
				freeOn = freePx > 0
				continue
			}
			// The separator before the first field is budgeted even when the
			// name column comes out empty (an empty listing) and the render
			// skips it; over-budgeting by one separator in that corner errs in
			// the direction that cannot overflow.
			fixed += w.sepPx + filePaneStatusFieldColumnPx(w, field)
		}
		if freeOn {
			fixed += w.regionSepPx + freePx
		}
		nameAvail := availPx - fixed
		if nameAvail >= min(w.namePx, w.floorPx) || len(active) == 0 {
			plan := filePaneStatusBarPlan{
				nameColPx: max(min(w.namePx, nameAvail), 0),
				showFree:  freeOn,
			}
			for _, field := range active {
				if field == filePaneStatusFieldFree {
					continue
				}
				plan.fields = append(plan.fields, field)
				plan.fieldPx = append(plan.fieldPx, filePaneStatusFieldColumnPx(w, field))
			}
			return plan
		}
		next, ok := filePaneStatusDropNext(active)
		if !ok {
			// filePaneStatusDropNext keeps a last field; the anchored layout
			// does not — the name outlives everything.
			next = nil
		}
		active = next
	}
}

// filePaneStatusBarMeasureState caches the info row's measured widths on the
// pane, following the two established caching patterns: like
// filePaneModel.briefTextWidth it is invalidated by a listing change (the
// entriesGen counter setEntries bumps), and like the volume badge's
// measuredPxDp/measuredTypeface keys it is invalidated by a metrics, typeface
// or text-size change. The config inputs are keyed by the sample strings they
// produce (plus the compaction head length, which shapes fitValue rather than
// a width), so editing the date format, the permission format or the
// name-compaction settings re-measures without any explicit notification.
type filePaneStatusBarMeasureState struct {
	valid bool
	key   filePaneStatusBarMeasureKey

	widths filePaneStatusColumnWidths

	// The free label's measurement, keyed on the label string alone: it
	// changes on volume polls, which must not retrigger the O(entries) scan
	// above.
	freeLabel string
	freePx    int

	// The compacted cursor name, keyed on (name, column width): recomputed on
	// cursor moves and resizes only, never per frame.
	fitName  string
	fitPx    int
	fitValue string
}

type filePaneStatusBarMeasureKey struct {
	gen         uint64
	pxPerDp     float32
	pxPerSp     float32
	typeface    font.Typeface
	textSize    unit.Sp
	dateSample  string
	permsSample string
	marker      string
	keepStart   int
}

// filePaneStatusBarMeasure returns the pane's cached column widths, remeasuring
// only when the key — listing generation, metrics, typeface, text size, or the
// config-derived samples — has changed. The O(entries) widest-name scan
// therefore runs once per listing, not per frame.
func (ui *UI) filePaneStatusBarMeasure(th *material.Theme, gtx layout.Context, pane *filePaneState) *filePaneStatusBarMeasureState {
	state := &pane.statusBar
	key := filePaneStatusBarMeasureKey{
		gen:         pane.model.entriesGen,
		pxPerDp:     gtx.Metric.PxPerDp,
		pxPerSp:     gtx.Metric.PxPerSp,
		typeface:    ui.mainTypeface(),
		textSize:    scaleThemeFontSize(th, 11),
		dateSample:  filePaneStatusDateSample(pane.model),
		permsSample: filePaneStatusPermsSample(pane.model),
		marker:      filePaneStatusCompactMarker(pane.model),
		keepStart:   filePaneStatusCompactKeepStart(pane.model),
	}
	if state.valid && state.key == key {
		return state
	}
	state.widths = computeFilePaneStatusColumnWidths(pane, func(text string) int {
		return ui.measureFilePaneStatusBarTextWidth(th, gtx, text)
	})
	state.key = key
	state.valid = true
	state.freeLabel, state.freePx = "", 0
	state.fitName, state.fitPx, state.fitValue = "", 0, ""
	return state
}

// filePaneStatusBarFreePx measures the free label through the state's one-slot
// cache, so volume polls re-measure one short string rather than the listing.
func (ui *UI) filePaneStatusBarFreePx(th *material.Theme, gtx layout.Context, state *filePaneStatusBarMeasureState, label string) int {
	if label == "" {
		return 0
	}
	if state.freeLabel != label || state.freePx <= 0 {
		state.freeLabel = label
		state.freePx = ui.measureFilePaneStatusBarTextWidth(th, gtx, label)
	}
	return state.freePx
}

// filePaneStatusBarNameValue renders the cursor entry's name for the name
// column: the full name when it fits, otherwise filePaneStatusNameValue's
// compaction (compactName's marker-plus-extension-tail trim) at the largest
// rune capacity whose measured width fits the column. The result is cached on
// (name, column width) so an idle cursor costs nothing per frame.
func (ui *UI) filePaneStatusBarNameValue(th *material.Theme, gtx layout.Context, pane *filePaneState, state *filePaneStatusBarMeasureState, colPx int) string {
	entry := pane.selectedEntry()
	if entry == nil || colPx <= 0 {
		return ""
	}
	name := entry.DisplayName
	if name == "" {
		name = entry.Name
	}
	if state.fitName == name && state.fitPx == colPx {
		return state.fitValue
	}
	measure := func(text string) int {
		return ui.measureFilePaneStatusBarTextWidth(th, gtx, text)
	}
	fitted := fitFilePaneStatusName(pane, name, colPx, measure)
	state.fitName, state.fitPx, state.fitValue = name, colPx, fitted
	return fitted
}

// fitFilePaneStatusName compacts name into widthPx. The pane font is not
// monospace, so a rune capacity cannot be derived from the width arithmetically:
// this starts from a proportional guess and walks the capacity until the
// measured result fits, growing it back while there is room. compactName's
// output width is monotonic enough in its capacity for both walks to be short,
// and the caller caches the result per (name, width) anyway.
func fitFilePaneStatusName(pane *filePaneState, name string, widthPx int, measure func(string) int) string {
	if widthPx <= 0 || name == "" {
		return ""
	}
	fullPx := measure(name)
	if fullPx <= widthPx {
		return name
	}
	runes := utf8.RuneCountInString(name)
	capacity := runes * widthPx / fullPx
	if capacity >= runes {
		capacity = runes - 1
	}
	candidate := filePaneStatusNameValue(pane, capacity)
	for capacity > 0 && measure(candidate) > widthPx {
		capacity--
		candidate = filePaneStatusNameValue(pane, capacity)
	}
	for capacity+1 < runes {
		grown := filePaneStatusNameValue(pane, capacity+1)
		if measure(grown) > widthPx {
			break
		}
		capacity++
		candidate = grown
	}
	if measure(candidate) > widthPx {
		return ""
	}
	return candidate
}

// layoutFilePaneStatusBarInfoRow lays the info line out as a real flex row —
// the pane font is user-configurable and not guaranteed monospace, so padding
// spaces cannot align anything:
//
//	[name col][ • ][field col]…[flexed spacer][ │ ][free label]
//
// The left cluster's columns are fixed-width (fixedWidth) so the cursor cannot
// shift them; the flexed spacer absorbs the slack, anchoring the free region
// to the right inset. When the pane has marked rows the left cluster is
// replaced wholesale by the marked-mode summary; the free region — decided by
// the same plan in both modes — does not move.
//
// The strip must not collapse when there is nothing to say (the ".." row with
// every value empty, an empty listing): Gio gives a single-line label the
// font's full line box even with no text in it, and the row always contains at
// least one label, so the height is stable across rows.
func (ui *UI) layoutFilePaneStatusBarInfoRow(
	th *material.Theme,
	gtx layout.Context,
	pane *filePaneState,
	configured []filePaneStatusField,
	freeLabel string,
	textColor color.NRGBA,
) layout.Dimensions {
	// The pane's Stack hands the strip Min.X == 0 or the pane width depending
	// on the caller; neither should stretch the labels, and the row's width
	// comes from the flexed spacer keyed on Max.X.
	gtx.Constraints.Min = image.Point{}
	if pane == nil || pane.model == nil {
		return ui.filePaneStatusBarLabel(th, "", textColor).Layout(gtx)
	}

	state := ui.filePaneStatusBarMeasure(th, gtx, pane)
	freePx := ui.filePaneStatusBarFreePx(th, gtx, state, freeLabel)
	avail := gtx.Constraints.Max.X
	plan := buildFilePaneStatusBarPlan(state.widths, configured, freePx, avail)

	rigidLabel := func(text string) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.filePaneStatusBarLabel(th, text, textColor).Layout(gtx)
		})
	}
	columnLabel := func(px int, text string) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, px, func(gtx layout.Context) layout.Dimensions {
				return ui.filePaneStatusBarLabel(th, text, textColor).Layout(gtx)
			})
		})
	}

	children := make([]layout.FlexChild, 0, 2*len(plan.fields)+4)
	leftPresent := false
	if count, size, ok := filePaneStatusMarkedSummary(pane); ok {
		leftPresent = true
		summary := count + filePaneStatusColumnSeparator + size
		leftMax := avail
		if plan.showFree {
			leftMax -= state.widths.regionSepPx + freePx
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// The summary is transient and needs no directory-derived width;
			// it just may not push the free region off its anchor.
			gtx.Constraints.Max.X = max(leftMax, 0)
			return ui.filePaneStatusBarLabel(th, summary, textColor).Layout(gtx)
		}))
	} else {
		if plan.nameColPx > 0 {
			leftPresent = true
			name := ui.filePaneStatusBarNameValue(th, gtx, pane, state, plan.nameColPx)
			children = append(children, columnLabel(plan.nameColPx, name))
		}
		for i, field := range plan.fields {
			if leftPresent {
				children = append(children, rigidLabel(filePaneStatusColumnSeparator))
			}
			leftPresent = true
			children = append(children, columnLabel(plan.fieldPx[i], filePaneStatusFieldValue(pane, field, "")))
		}
	}
	if !leftPresent && !plan.showFree {
		// Nothing at all this frame; keep the strip's height (see the doc
		// comment above).
		children = append(children, rigidLabel(""))
	}
	children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
	}))
	if plan.showFree {
		if leftPresent {
			// The region separator renders only when both regions do.
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutFilePaneStatusBarRegionRule(th, gtx, state.widths, textColor)
			}))
		}
		children = append(children, rigidLabel(freeLabel))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

// layoutFilePaneStatusBarRegionRule draws the "│" between the info row's two
// regions as a painted hairline rather than the U+2502 glyph: box-drawing
// glyphs deliberately span the whole line box so they connect across lines,
// which makes the character's ink overflow the strip's 4dp vertical insets.
// The rule is exactly regionSepPx wide — the same measurement the plan
// budgets — so the free region still ends flush on the right inset, and it
// stands one text-size tall, centred by the row's Middle alignment.
//
// It carries a semantic "│" label the way widget.Label tags its text, so the
// separator stays visible to accessibility and to the semantic-tree tests.
func (ui *UI) layoutFilePaneStatusBarRegionRule(th *material.Theme, gtx layout.Context, w filePaneStatusColumnWidths, textColor color.NRGBA) layout.Dimensions {
	total := w.regionSepPx
	height := gtx.Sp(scaleThemeFontSize(th, 11))
	rulePx := max(gtx.Dp(unit.Dp(1)), 1)
	x := w.ruleGapPx + max((total-2*w.ruleGapPx-rulePx)/2, 0)
	if x+rulePx > total {
		x = max(total-rulePx, 0)
	}
	area := clip.Rect(image.Rect(0, 0, total, height)).Push(gtx.Ops)
	semantic.LabelOp("│").Add(gtx.Ops)
	paint.FillShape(gtx.Ops, textColor, clip.Rect(image.Rect(x, 0, x+rulePx, height)).Op())
	area.Pop()
	return layout.Dimensions{Size: image.Pt(total, height)}
}

// filePaneStatusBarLabel is the strip's one text style — shared by the info
// row's columns and the progress lines, and mirrored by
// measureFilePaneStatusBarTextWidth so measurements and rendering agree.
func (ui *UI) filePaneStatusBarLabel(th *material.Theme, text string, textColor color.NRGBA) material.LabelStyle {
	lbl := material.Body2(th, text)
	lbl.Font.Typeface = ui.mainTypeface()
	lbl.TextSize = scaleThemeFontSize(th, 11)
	lbl.Color = textColor
	lbl.MaxLines = 1
	lbl.Truncator = "…"
	return lbl
}

func (ui *UI) directFilePasteForPane(idx int) *fileCopyState {
	if ui == nil || ui.fileCopy == nil || !ui.fileCopy.directPaste || !ui.fileCopy.running || ui.fileCopy.dstPane != idx {
		return nil
	}
	return ui.fileCopy
}

func (ui *UI) measureFilePaneStatusBarTextWidth(th *material.Theme, gtx layout.Context, label string) int {
	lbl := material.Body2(th, label)
	lbl.Font.Typeface = ui.mainTypeface()
	lbl.TextSize = scaleThemeFontSize(th, 11)
	lbl.MaxLines = 1
	lbl.Truncator = ""
	return measureLabelUnconstrained(gtx, lbl).Size.X
}

func layoutFilePaneStatusBarBox(gtx layout.Context, bg, border color.NRGBA, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()

	width := gtx.Constraints.Max.X
	if width < dims.Size.X {
		width = dims.Size.X
	}
	if width <= 0 || dims.Size.Y <= 0 {
		call.Add(gtx.Ops)
		return dims
	}

	rect := image.Rect(0, 0, width, dims.Size.Y)
	paint.FillShape(gtx.Ops, bg, clip.Rect(rect).Op())
	if border.A != 0 {
		paint.FillShape(gtx.Ops, border, clip.Rect(image.Rect(0, 0, width, 1)).Op())
	}
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(width, dims.Size.Y), Baseline: dims.Baseline}
}

// filePaneStatusBarVisible reports whether the file info line should render for
// this pane. Evaluated per pane, because panes carry their view mode
// independently: a brief pane keeps its bar while a full pane beside it hides
// one.
func filePaneStatusBarVisible(cfg *fm.Config, pane *filePaneState) bool {
	if cfg == nil || pane == nil || pane.table == nil {
		return false
	}
	if !cfg.StatusBar.Enabled {
		return false
	}
	return !(cfg.StatusBar.HideInFull && pane.table.Mode == table.ModeFull)
}

// filePaneStatusBarShowsFreeSpace reports whether the ACTIVE pane's status bar
// is carrying free space right now.
//
// The floating badge and the bar are two presentations of one number — the
// active pane's volume. filePaneVolumeBadgeSourcePane draws the badge on the
// *inactive* panes but sources it from ui.filePanes[ui.activeFilePane], so the
// badge is redundant exactly while the active pane's own bar shows free space,
// and is the fallback the rest of the time. This predicate is that switch;
// filePaneVolumeBadgesHidden is what consults it, so exactly one presentation
// of free space is on at a time.
//
// The active pane is therefore consulted through filePaneStatusBarVisible, the
// same rule the strip itself renders by, rather than through StatusBar.Enabled
// alone. That is what makes every way of not showing the bar fall back to the
// badge alike: field unticked, bar switched off, or bar hidden in full view.
// Reading Enabled alone used to make hide_in_full the odd one out — the bar was
// gone and the badge stayed suppressed, so free space appeared nowhere at all.
//
// Nil-safe throughout: activePane returns nil when there are no panes, and
// filePaneStatusBarVisible rejects a nil config, a nil pane and a pane with no
// table.
func (ui *UI) filePaneStatusBarShowsFreeSpace() bool {
	if ui == nil || ui.fmCfg == nil {
		return false
	}
	if !slices.Contains(ui.fmCfg.StatusBar.Fields, fm.StatusBarFieldFree) {
		return false
	}
	return filePaneStatusBarVisible(ui.fmCfg, ui.activePane())
}

// filePaneStatusDropNext removes the highest-priority-to-drop field still
// present, returning false when only one field remains. buildFilePaneStatusBarPlan
// drives the info row's degradation through it, dropping even the last field —
// the name column is the anchor there, not a field.
//
// The full slice expression keeps the returned slice from writing over the
// input's backing array, so callers may keep using the slice they passed in.
func filePaneStatusDropNext(active []filePaneStatusField) ([]filePaneStatusField, bool) {
	if len(active) <= 1 {
		return active, false
	}
	for _, candidate := range filePaneStatusDropOrder {
		for i, field := range active {
			if field == candidate {
				return append(active[:i:i], active[i+1:]...), true
			}
		}
	}
	return active, false
}
