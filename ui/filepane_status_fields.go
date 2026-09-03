// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"math/bits"
	"strings"

	"hexone/filesys"
	"hexone/fm"
)

// Adding a sixth field means touching all of these. Each site has its own
// local comment; this is the only list of the whole set. Verify by grep before
// trusting it — it is hand-maintained like everything it points at.
//
//	fm/config.go                       StatusBarField* constant, statusBarFieldOrder
//	fm/config_test.go                  the roster in TestStatusBarFieldOrderCoversEveryFieldConstant
//	ui/filepane_status_fields.go       the enum below, filePaneStatusFieldFromConfigKey,
//	                                   filePaneStatusDropOrder, the filePaneStatusFieldValue switch
//	ui/filepane_status_fields_test.go  allFilePaneStatusFields
//	ui/filepane_status_bar.go          the filePaneStatusFieldColumnPx switch; a
//	                                   width source in computeFilePaneStatusColumnWidths
//	                                   (a measured sample constant like the size's,
//	                                   or a per-listing scan like the owner's —
//	                                   without one the column is zero-width and
//	                                   invisible while its checkbox looks live);
//	                                   and a filePaneStatusBarMeasureKey entry
//	                                   when the sample is config-derived, so
//	                                   editing that config re-measures
//	ui/settings_modal.go               statusBarFieldCount, buildStatusBarFieldConfigKeys
//	ui/settings_filepanes.go           settingsStatusBarFieldRows, and the
//	                                   settingsStatusBarPreviewEntries cursor entry
//	                                   if the new field reads an Entry attribute the
//	                                   sample does not carry — otherwise the field
//	                                   renders empty and its checkbox looks dead
//	HELP.md                            the field table (added by the docs task)
//
// ui/settings_modal_keyboard.go deliberately needs nothing: the tab's keyboard
// focus targets are a range derived from statusBarFieldCount and ordered by
// settingsStatusBarFieldRows, so do not write out per-field constants there.
//
// Two of the bar's values are deliberately NOT in this enum (Revision 2):
//
//   - The name has no config key and no checkbox — it is always rendered, as
//     the left anchor, and survives every drop. filePaneStatusNameValue below
//     is its whole surface; putting it in the enum would force exclusions into
//     every list above and every guard test that says "each enum value has a
//     config key and a checkbox".
//   - The marked-mode summary (filePaneStatusMarkedSummary below) is not a
//     field either: whenever the pane has marked rows it replaces the whole
//     per-entry left cluster, automatically. Revision 1's "selection" field and
//     config key are retired; fm.NormalizeStatusBarFields drops the key from
//     old configs as unknown.
//
// One field is retired outright (Revision 2.1): the item count. It was never a
// per-file attribute, and the marked-mode summary already reports counts, so
// its "items" config key migrates exactly like "selection" — dropped from old
// configs as unknown.
//
// The two test rosters — allFilePaneStatusFields and the one in
// TestStatusBarFieldOrderCoversEveryFieldConstant — come first: Go cannot
// enumerate an enum, so every guard test below is blind until they list the new
// constant. Update them and the guards turn most of the rest loud
// (statusBarFieldOrder, filePaneStatusFieldFromConfigKey,
// filePaneStatusDropOrder, statusBarFieldCount, buildStatusBarFieldConfigKeys,
// settingsStatusBarFieldRows, and — via TestStatusFieldColumnPxCoversEveryField,
// provided its listing gives the new field a value — the
// filePaneStatusFieldColumnPx switch and its width source). Leave them stale
// and every failure below is silent: a missing statusBarFieldOrder entry drops
// the field from every user's config, a missing statusBarFieldCount bump
// panics on the first settings open, a missing filePaneStatusFieldValue case
// renders empty and vanishes from the line, and a missing
// filePaneStatusFieldColumnPx arm renders the field as a zero-width, invisible
// column whose checkbox still looks functional. Nothing guards the
// filePaneStatusFieldValue switch, the config-derived measure-key entry, or
// HELP.md.

// filePaneStatusField identifies one field of the pane status bar. Declaration
// order is canonical display order; the drop order below is deliberately
// different.
type filePaneStatusField uint8

const (
	filePaneStatusFieldSize filePaneStatusField = iota
	filePaneStatusFieldDate
	filePaneStatusFieldPerms
	filePaneStatusFieldOwner
	filePaneStatusFieldFree
)

// filePaneStatusDateWidthPx is the width the Date field hands to
// filePaneModel.formatDate. It is deliberately far larger than any pane.
//
// formatDate was written for the responsive Date *column*: it walks
// cfg.DateFormats and returns the richest one that fits the width it is given.
// The status bar has its own degradation mechanism — it drops whole fields, in
// filePaneStatusDropOrder — so a realistic width here would give the bar a
// second, invisible one layered underneath, silently shortening the date
// instead of dropping the field. A width nothing can overflow makes formatDate
// return cfg.DateFormats[0], the user's preferred format, and leaves shrinking
// to the line builder. That is how every other field in this bar behaves: each
// renders at its natural width and the *line* degrades.
//
// The constant has to clear both of formatDate's fitting tests, and in this
// path only the approximate one runs. The exact one goes through
// measuredTextWidth, which reports "no measurement" whenever the model has no
// text measurer installed — and the status bar never sees one: layoutFilePaneTable
// installs the measurer for the duration of the table only and its own deferred
// setTextMeasurerForStyle(key, nil) tears it down before the strip below lays
// out. (Note that this nils the measurer while leaving the cache populated, and
// measuredTextWidth checks the measurer first, so even a warm cache reports no
// measurement here.) So formatDate falls back to a budget of
// m.approxChars(widthPx, 4) characters, which at 1<<20 px is tens of thousands
// even at the largest font scale. Both branches therefore pick the first
// format, which is what keeps this field correct whether or not some future
// caller lays the bar out with a measurer installed.
const filePaneStatusDateWidthPx = 1 << 20

// filePaneStatusDropOrder lists fields from first-dropped to last-dropped when
// the line does not fit. Size and date survive longest because reading them in
// brief mode is the reason this bar exists. The name is not listed because it
// is not a field: it is the left anchor, absorbs shrinkage via compactName
// first, and outlives everything here (the degradation order, items retired by
// Revision 2.1, is owner → perms → free → date → size, then the name stands
// alone).
var filePaneStatusDropOrder = []filePaneStatusField{
	filePaneStatusFieldOwner,
	filePaneStatusFieldPerms,
	filePaneStatusFieldFree,
	filePaneStatusFieldDate,
	filePaneStatusFieldSize,
}

func filePaneStatusFieldFromConfigKey(key string) (filePaneStatusField, bool) {
	switch key {
	case fm.StatusBarFieldSize:
		return filePaneStatusFieldSize, true
	case fm.StatusBarFieldDate:
		return filePaneStatusFieldDate, true
	case fm.StatusBarFieldPerms:
		return filePaneStatusFieldPerms, true
	case fm.StatusBarFieldOwner:
		return filePaneStatusFieldOwner, true
	case fm.StatusBarFieldFree:
		return filePaneStatusFieldFree, true
	}
	return 0, false
}

// filePaneStatusFields converts normalised config keys into fields, preserving
// the config's order (which normalisation has already made canonical).
func filePaneStatusFields(keys []string) []filePaneStatusField {
	out := make([]filePaneStatusField, 0, len(keys))
	for _, key := range keys {
		if field, ok := filePaneStatusFieldFromConfigKey(key); ok {
			out = append(out, field)
		}
	}
	return out
}

// formatFilePaneStatusFree renders the free-space field in Revision 2's single
// form: "<free> free (<pct>%)", the percentage being free/total rounded to the
// nearest integer — it qualifies the word it follows. The Revision 1 usage bar
// and three-form ladder are retired; this form either fits or the whole field
// is dropped.
//
// Deliberately NOT the volume badge's label: the badge still shows
// "X free / Y" (formatFilePaneVolumeBadgeLabel), and the two presentations
// differ by design now — do not reunify them.
func formatFilePaneStatusFree(freeBytes, totalBytes uint64) string {
	if totalBytes == 0 {
		return ""
	}
	// Clamped because SFTP and network volumes do report free above total, and
	// the percentage below would then read over 100 — a bar claiming more free
	// space than the volume has. Clamping first also makes the division safe to
	// reason about: freeBytes/totalBytes is at most 1 from here on.
	if freeBytes > totalBytes {
		freeBytes = totalBytes
	}

	// round(free*100/total), half away from zero, in 128-bit integer space:
	// free*100 overflows uint64 for volumes past ~184 PB (which SFTP servers
	// can claim), and a float64 product cannot be trusted to land exactly on
	// .5 boundaries.
	hi, lo := bits.Mul64(freeBytes, 100)
	lo, carry := bits.Add64(lo, totalBytes/2, 0)
	// Div64 needs hi < totalBytes; the clamp guarantees the quotient is at most
	// 100, so this holds for every totalBytes >= 1.
	pct, _ := bits.Div64(hi+carry, lo, totalBytes)

	return fmt.Sprintf("%s free (%d%%)", formatFilePaneVolumeBytes(freeBytes), pct)
}

// filePaneStatusFieldValue renders one field for the pane's cursor entry. A
// field with nothing to show returns an empty string and renders as an empty
// column — the anchored layout keeps the column's fixed width either way, so
// neighbours never shift.
//
// freeLabel is passed in rather than computed here because the volume lookup is
// cached on the pane and refreshed on a timer by the layout path.
func filePaneStatusFieldValue(pane *filePaneState, field filePaneStatusField, freeLabel string) string {
	if pane == nil || pane.model == nil {
		return ""
	}
	entry := pane.selectedEntry()

	switch field {
	case filePaneStatusFieldSize:
		if entry == nil {
			return ""
		}
		switch entry.Kind {
		case filesys.EntryParent:
			return "<UP>"
		case filesys.EntryDir:
			return "<DIR>"
		}
		return formatFilePaneVolumeBytes(uint64(max(entry.SizeBytes, 0)))

	case filePaneStatusFieldDate:
		if entry == nil {
			return ""
		}
		// The bar has its own date layout (status_bar.date_format, Revision
		// 2.1): a fixed key formats the entry's ModTime directly, independent
		// of the Full-mode column date builder. A zero ModTime (the ".." row,
		// a broken entry) renders empty in every layout — never the zero time
		// pushed through one.
		if pane.model.cfg != nil {
			if layout := fm.StatusBarDateLayout(pane.model.cfg.StatusBar.DateFormat); layout != "" {
				if entry.ModTime.IsZero() {
					return ""
				}
				return entry.ModTime.Format(layout)
			}
		}
		// "auto": the pre-existing behaviour. Deliberately not entry.DateText.
		// The listing layer bakes that string once, hardcoded as "Jan 02 2006"
		// (filesys.formatDate), so reading it here left this field deaf to
		// every date format the user can configure under Settings -> File
		// panes — no time component, and no ISO, day-first or slash form.
		// formatDate is the one place that reads cfg.DateFormats, and it
		// already carries the two behaviours this field needs: a zero ModTime
		// returns DateText verbatim, so ".." still renders empty, and a config
		// with no formats at all falls back to "Jan 02 2006".
		return strings.TrimSpace(pane.model.formatDate(*entry, filePaneStatusDateWidthPx))

	case filePaneStatusFieldPerms:
		if entry == nil {
			return ""
		}
		return strings.TrimSpace(pane.model.defaultPermissionText(*entry))

	case filePaneStatusFieldOwner:
		if entry == nil {
			return ""
		}
		// SFTP owners render numerically (Task 2), which includes "0:0". That
		// is genuinely ambiguous: pkg/sftp drops the SSH_FILEXFER_ATTR_UIDGID
		// flag, so a server that never reported ownership is indistinguishable
		// from a root-owned file. Render it anyway — suppressing "0:0" would
		// hide real root ownership, which matters far more often than the
		// ambiguity bites, and the raw numeric form already signals that these
		// are unresolved ids rather than a confirmed "root".
		return strings.TrimSpace(entry.OwnerText)

	case filePaneStatusFieldFree:
		return freeLabel
	}
	return ""
}

// filePaneStatusNameValue renders the bar's name value — the left anchor of
// Revision 2's columnar layout. It is not a filePaneStatusField: it has no
// config key, no checkbox and no drop-order slot, because it is always shown
// and survives every field drop (see the checklist comment above).
//
// The name is the entry's DisplayName — the same string the pane grid renders
// in filePaneEntryNameCell — falling back to Name for entries that never got
// one. capacityRunes is the name column's width in runes: a longer name is
// compacted by filePaneModel.compactName, the grid's own marker-plus-tail trim
// (gpstrack-dashb….go), which honours the name_compact config and preserves
// the extension. A capacity of zero or less means no room at all and renders
// empty, matching compactName; the anchored layout skips the name column
// entirely before offering that (fitFilePaneStatusName is the px-budget
// caller).
func filePaneStatusNameValue(pane *filePaneState, capacityRunes int) string {
	if pane == nil || pane.model == nil {
		return ""
	}
	entry := pane.selectedEntry()
	if entry == nil {
		return ""
	}
	name := entry.DisplayName
	if name == "" {
		name = entry.Name
	}
	return pane.model.compactName(name, capacityRunes)
}

// filePaneStatusMarkedSummary builds the marked-mode summary that replaces the
// whole per-entry left cluster while the pane has marked rows — automatically,
// not via a config field (Revision 1's "selection" field is retired).
//
// The two parts are returned separately on purpose: the anchored layout
// renders the left cluster as fixed columns joined by its own "  •  "
// separator, so the count ("3 items selected", "1 item selected") and the
// combined size ("14.20 MB") are two column values, not one pre-joined string.
// ok reports whether the mode is active at all; when it is false both strings
// are empty and the caller renders the per-entry cluster instead.
func filePaneStatusMarkedSummary(pane *filePaneState) (count, size string, ok bool) {
	if pane == nil || pane.model == nil || !pane.hasMarkedRows() {
		return "", "", false
	}
	total := int64(0)
	for row := range pane.markedRows {
		// Directories and other sizeless entries report non-positive sizes;
		// they count as items but contribute nothing to the sum.
		if marked := pane.model.Entry(row); marked != nil && marked.SizeBytes > 0 {
			total += marked.SizeBytes
		}
	}
	count = fmt.Sprintf("%d items selected", len(pane.markedRows))
	if len(pane.markedRows) == 1 {
		count = "1 item selected"
	}
	return count, formatFilePaneVolumeBytes(uint64(total)), true
}
