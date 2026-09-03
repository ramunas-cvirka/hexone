// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"image"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/platform"
	"hexone/ui/widget/table"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// measureByRunes stands in for real text measurement: one rune, one unit.
func measureByRunes(text string) int { return len([]rune(text)) }

func TestLayoutFilePaneStatusBarUsesFullPaneWidth(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() {
		localVolumeUsageFunc = oldLookup
	}()

	dir := t.TempDir()
	localVolumeUsageFunc = func(path string) (platform.VolumeUsage, error) {
		return platform.VolumeUsage{
			FreeBytes:  64 << 30,
			TotalBytes: 512 << 30,
		}, nil
	}

	ui := NewUI(nil)
	pane := newFilePaneState(dir, nil)
	ui.filePanes = []*filePaneState{pane}
	ui.activeFilePane = 0

	now := time.Unix(1700000000, 0)
	ui.archiveExtract = &archiveExtractState{
		pane:        0,
		archivePath: "bundle.zip",
		startedAt:   now.Add(-time.Second),
		progress: filesys.CopyProgress{
			BytesDone:   50 << 20,
			BytesTotal:  100 << 20,
			CurrentPath: filepath.Join("bundle.zip", "movie.mkv"),
		},
	}

	gtx := layout.Context{
		Ops:         new(op.Ops),
		Now:         now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(320, 80)),
	}
	dims := ui.layoutFilePaneStatusBar(material.NewTheme(), gtx, 0, pane, filePanePaletteFromConfig(ui.fmCfg))
	if dims.Size.X != 320 {
		t.Fatalf("status bar width = %d, want full pane width 320", dims.Size.X)
	}
	if dims.Size.Y <= 0 {
		t.Fatalf("status bar height = %d, want positive", dims.Size.Y)
	}
}

func TestLayoutFilePaneStatusBarShowsDirectPasteWithoutBlockingUI(t *testing.T) {
	ui := NewUI(nil)
	pane := newFilePaneState(t.TempDir(), nil)
	ui.filePanes = []*filePaneState{pane}
	ui.activeFilePane = 0

	now := time.Unix(1700000000, 0)
	ui.fileCopy = &fileCopyState{
		pane:        0,
		srcPane:     0,
		dstPane:     0,
		srcPath:     filepath.Join(t.TempDir(), "movie.mkv"),
		directPaste: true,
		running:     true,
		startedAt:   now.Add(-time.Second),
		progress: filesys.CopyProgress{
			BytesDone:   50 << 20,
			BytesTotal:  100 << 20,
			CurrentPath: "movie.mkv",
		},
	}

	gtx := layout.Context{
		Ops:         new(op.Ops),
		Now:         now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(320, 80)),
	}
	dims := ui.layoutFilePaneStatusBar(material.NewTheme(), gtx, 0, pane, filePanePaletteFromConfig(ui.fmCfg))
	if dims.Size.X != 320 || dims.Size.Y <= 0 {
		t.Fatalf("direct paste status bar size=%v, want full-width positive bar", dims.Size)
	}
	if ui.hasBlockingFileDialog() || ui.fileCopyBlocksUI() {
		t.Fatal("direct paste should not block normal pane interaction")
	}
}

func TestFilePaneStatusBarSeparatorModeFollowsPaneSide(t *testing.T) {
	ui := NewUI(nil)
	left := newFilePaneState(t.TempDir(), nil)
	right := newFilePaneState(t.TempDir(), nil)
	ui.filePanes = []*filePaneState{left, right}

	if got := ui.filePaneStatusBarSeparatorMode(0); got != filePaneStatusBarSeparatorTrailing {
		t.Fatalf("left pane separator mode = %v, want trailing", got)
	}
	if got := ui.filePaneStatusBarSeparatorMode(1); got != filePaneStatusBarSeparatorLeading {
		t.Fatalf("right pane separator mode = %v, want leading", got)
	}

	ui.filePanes = []*filePaneState{left}
	if got := ui.filePaneStatusBarSeparatorMode(0); got != filePaneStatusBarSeparatorNone {
		t.Fatalf("single pane separator mode = %v, want none", got)
	}
}

func TestStatusBarVisibility(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		hideInFull bool
		mode       table.Mode
		want       bool
	}{
		{"enabled, brief", true, false, table.ModeBrief, true},
		{"enabled, full", true, false, table.ModeFull, true},
		{"hide in full, brief", true, true, table.ModeBrief, true},
		{"hide in full, full", true, true, table.ModeFull, false},
		{"disabled, brief", false, false, table.ModeBrief, false},
		{"disabled, full", false, false, table.ModeFull, false},
		{"disabled overrides hide in full", false, true, table.ModeBrief, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fm.DefaultConfig()
			cfg.StatusBar.Enabled = tc.enabled
			cfg.StatusBar.HideInFull = tc.hideInFull
			pane := testStatusPane([]filesys.Entry{{Name: "a", Kind: filesys.EntryFile}}, 0)
			pane.table.SetMode(tc.mode)
			if got := filePaneStatusBarVisible(cfg, pane); got != tc.want {
				t.Fatalf("visible = %v, want %v", got, tc.want)
			}
		})
	}
}

// statusColumnEntries is the shared listing for the column-width tests: a
// parent row, a short name and a long one, and two owners of different widths.
func statusColumnEntries() []filesys.Entry {
	return []filesys.Entry{
		{Name: "..", Kind: filesys.EntryParent},
		{Name: "a.md", Kind: filesys.EntryFile, SizeBytes: 100,
			DateText: "2026-08-30 14:22", PermText: "-rw-r--r--", OwnerText: "root:wheel"},
		{Name: "gpstrack-dashboard-server.go", Kind: filesys.EntryFile, SizeBytes: 2516582,
			DateText: "2026-08-30 14:22", PermText: "-rw-r--r--", OwnerText: "ramunas:staff"},
	}
}

// TestStatusColumnWidthsIgnoreTheCursor pins Revision 2's no-jump foundation:
// the column widths are a property of the listing and the configuration, so
// two panes over the same listing with different selected rows must measure
// identically. The selection is not even an input — this test is what keeps it
// that way.
func TestStatusColumnWidthsIgnoreTheCursor(t *testing.T) {
	entries := statusColumnEntries()
	var got []filePaneStatusColumnWidths
	for _, selected := range []int{0, 1, 2} {
		pane := testStatusPane(entries, selected)
		got = append(got, computeFilePaneStatusColumnWidths(pane, measureByRunes))
	}
	for i := 1; i < len(got); i++ {
		if got[i] != got[0] {
			t.Fatalf("selection %d measured %+v, selection 0 measured %+v; column widths depend on the cursor", i, got[i], got[0])
		}
	}

	w := got[0]
	if want := measureByRunes("gpstrack-dashboard-server.go"); w.namePx != want {
		t.Errorf("namePx = %d, want the widest display name's %d", w.namePx, want)
	}
	if want := measureByRunes("ramunas:staff"); w.ownerPx != want {
		t.Errorf("ownerPx = %d, want the widest owner's %d", w.ownerPx, want)
	}
	if want := measureByRunes(filePaneStatusSizeSample); w.sizePx != want {
		t.Errorf("sizePx = %d, want the sample's %d", w.sizePx, want)
	}
	if want := measureByRunes(filePaneStatusPermsSymbolicSample); w.permsPx != want {
		t.Errorf("permsPx = %d, want the symbolic sample's %d", w.permsPx, want)
	}
	// The default DateFormats[0] on the wide sample stamp, not the entries'
	// baked DateText: the column must follow the user's configured format.
	if want := measureByRunes(filePaneStatusDateSampleTime.Format("Jan 02 2006 15:04")); w.datePx != want {
		t.Errorf("datePx = %d, want the DateFormats[0] sample's %d", w.datePx, want)
	}
	if want := measureByRunes(filePaneStatusCompactMarker(nil) + filePaneStatusNameFloorTail); w.floorPx != want {
		t.Errorf("floorPx = %d, want marker+tail = %d", w.floorPx, want)
	}
}

// TestStatusColumnWidthsCapTheOwnerColumn keeps a pathological owner string
// from pushing every other column off the pane.
func TestStatusColumnWidthsCapTheOwnerColumn(t *testing.T) {
	entries := []filesys.Entry{
		{Name: "f", Kind: filesys.EntryFile, OwnerText: strings.Repeat("x", 60)},
	}
	w := computeFilePaneStatusColumnWidths(testStatusPane(entries, 0), measureByRunes)
	if want := measureByRunes(filePaneStatusOwnerCapSample); w.ownerPx != want {
		t.Fatalf("ownerPx = %d, want capped at %d", w.ownerPx, want)
	}
}

// TestStatusFieldColumnPxCoversEveryField closes the checklist's column-width
// hole: a field whose filePaneStatusFieldColumnPx arm is missing renders as a
// zero-width, invisible column — while its settings checkbox still looks
// functional — and nothing else notices. The preview guard records a
// zero-width fixedWidth label's semantic text anyway, and the all-fields pixel
// capture runs in marked mode, where the per-entry columns never render.
//
// The widths come through the real computeFilePaneStatusColumnWidths, so a
// missing width source (the measured sample constant, or the per-listing scan)
// fails here too — provided the listing gives every scanned column a value:
// statusColumnEntries carries owner text for the owner scan, and a future
// scanned field must extend it the same way.
//
// Free space is the deliberate exception: it is the right-anchored region, not
// a left-cluster column, so it has no column width — buildFilePaneStatusBarPlan
// branches on it before ever consulting filePaneStatusFieldColumnPx. It is
// pinned at zero so an arm added for it by mistake fails loudly as well.
func TestStatusFieldColumnPxCoversEveryField(t *testing.T) {
	w := computeFilePaneStatusColumnWidths(testStatusPane(statusColumnEntries(), 1), measureByRunes)
	for _, field := range allFilePaneStatusFields {
		px := filePaneStatusFieldColumnPx(w, field)
		if field == filePaneStatusFieldFree {
			if px != 0 {
				t.Errorf("filePaneStatusFieldColumnPx(free) = %d, want 0: free space is the right region, not a left-cluster column", px)
			}
			continue
		}
		if px <= 0 {
			t.Errorf("filePaneStatusFieldColumnPx(field %d) = %d, want > 0: the field would render as an invisible zero-width column while its settings checkbox still looks functional", field, px)
		}
	}
}

// statusPlanFixture builds the widths the plan tests reason about, in
// rune-units. Every expected number below derives from these measurements.
func statusPlanFixture(t *testing.T) filePaneStatusColumnWidths {
	t.Helper()
	w := computeFilePaneStatusColumnWidths(testStatusPane(statusColumnEntries(), 1), measureByRunes)
	if w.namePx == 0 || w.sepPx == 0 || w.floorPx == 0 {
		t.Fatalf("fixture widths did not measure: %+v", w)
	}
	return w
}

func statusPlanFieldNames(plan filePaneStatusBarPlan) string {
	names := make([]string, 0, len(plan.fields)+1)
	for _, f := range plan.fields {
		switch f {
		case filePaneStatusFieldSize:
			names = append(names, "size")
		case filePaneStatusFieldDate:
			names = append(names, "date")
		case filePaneStatusFieldPerms:
			names = append(names, "perms")
		case filePaneStatusFieldOwner:
			names = append(names, "owner")
		}
	}
	if plan.showFree {
		names = append(names, "free")
	}
	return strings.Join(names, ",")
}

// TestStatusPlanNameAbsorbsShrinkageFirst pins degradation step one: the name
// column gives up width down to its floor before any field drops.
func TestStatusPlanNameAbsorbsShrinkageFirst(t *testing.T) {
	w := statusPlanFixture(t)
	enabled := []filePaneStatusField{filePaneStatusFieldSize, filePaneStatusFieldDate}
	fixed := 2*w.sepPx + w.sizePx + w.datePx

	full := buildFilePaneStatusBarPlan(w, enabled, 0, fixed+w.namePx)
	if full.nameColPx != w.namePx || len(full.fields) != 2 {
		t.Fatalf("plan at full width = %+v, want the whole name column and both fields", full)
	}

	squeezed := buildFilePaneStatusBarPlan(w, enabled, 0, fixed+w.floorPx+3)
	if squeezed.nameColPx != w.floorPx+3 || len(squeezed.fields) != 2 {
		t.Fatalf("squeezed plan = %+v, want a %d-wide name column and both fields intact", squeezed, w.floorPx+3)
	}

	atFloor := buildFilePaneStatusBarPlan(w, enabled, 0, fixed+w.floorPx)
	if atFloor.nameColPx != w.floorPx || len(atFloor.fields) != 2 {
		t.Fatalf("plan at the floor = %+v, want the floor-wide name column with both fields", atFloor)
	}

	belowFloor := buildFilePaneStatusBarPlan(w, enabled, 0, fixed+w.floorPx-1)
	if len(belowFloor.fields) != 1 {
		t.Fatalf("plan below the floor = %+v, want a field dropped before the name shrinks past its floor", belowFloor)
	}
}

// TestStatusPlanDropsFieldsInOrder walks the width down through every drop:
// owner → perms → free → date → size, the name surviving alone at the end — it
// is the anchor.
func TestStatusPlanDropsFieldsInOrder(t *testing.T) {
	w := statusPlanFixture(t)
	enabled := []filePaneStatusField{
		filePaneStatusFieldSize, filePaneStatusFieldDate, filePaneStatusFieldPerms,
		filePaneStatusFieldOwner, filePaneStatusFieldFree,
	}
	freePx := measureByRunes("41.00 GB free (41%)")

	fullFixed := 4*w.sepPx + w.sizePx + w.datePx + w.permsPx + w.ownerPx + w.regionSepPx + freePx
	want := []string{
		"size,date,perms,owner,free",
		"size,date,perms,free",
		"size,date,free",
		"size,date",
		"size",
		"",
	}
	var got []string
	for avail := fullFixed + w.namePx; avail >= 0; avail-- {
		plan := buildFilePaneStatusBarPlan(w, enabled, freePx, avail)
		names := statusPlanFieldNames(plan)
		if len(got) == 0 || got[len(got)-1] != names {
			got = append(got, names)
		}
		if len(plan.fields) > 0 || plan.showFree {
			if plan.nameColPx < min(w.namePx, w.floorPx) {
				t.Fatalf("at avail %d the plan %q kept fields with a %d-wide name column, below the floor %d", avail, names, plan.nameColPx, w.floorPx)
			}
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("degradation walked %v, want %v", got, want)
	}

	// The name renders last of all: with nothing else left it takes whatever
	// remains, even below its floor.
	tiny := buildFilePaneStatusBarPlan(w, enabled, freePx, 3)
	if tiny.nameColPx != 3 || len(tiny.fields) != 0 || tiny.showFree {
		t.Fatalf("plan at width 3 = %+v, want just a 3-wide name column", tiny)
	}
}

// TestStatusPlanFreeRegionWaitsForTheLookup pins the totalBytes == 0 contract
// one level down: freePx <= 0 means no free region and no budget reserved for
// one, so the name column gets the space until the reading lands.
func TestStatusPlanFreeRegionWaitsForTheLookup(t *testing.T) {
	w := statusPlanFixture(t)
	enabled := []filePaneStatusField{filePaneStatusFieldSize, filePaneStatusFieldFree}
	avail := w.sepPx + w.sizePx + w.regionSepPx + 19 + w.floorPx

	landed := buildFilePaneStatusBarPlan(w, enabled, 19, avail)
	if !landed.showFree {
		t.Fatalf("plan with a landed reading = %+v, want the free region shown", landed)
	}
	pending := buildFilePaneStatusBarPlan(w, enabled, 0, avail)
	if pending.showFree {
		t.Fatalf("plan before the lookup = %+v, want no free region", pending)
	}
	if pending.nameColPx <= landed.nameColPx {
		t.Fatalf("name column = %d with free pending vs %d landed; the pending region must not reserve space", pending.nameColPx, landed.nameColPx)
	}
}

// TestStatusPlanDoesNotMutateCallerFields guards the config's own slice: the
// layout hands buildFilePaneStatusBarPlan the fields filePaneStatusFields built
// from ui.fmCfg, and a narrow pane dropping fields must not delete them from
// the configuration.
func TestStatusPlanDoesNotMutateCallerFields(t *testing.T) {
	w := statusPlanFixture(t)
	enabled := []filePaneStatusField{
		filePaneStatusFieldSize, filePaneStatusFieldDate, filePaneStatusFieldPerms,
		filePaneStatusFieldOwner, filePaneStatusFieldFree,
	}
	before := slices.Clone(enabled)
	if plan := buildFilePaneStatusBarPlan(w, enabled, 19, 3); len(plan.fields) != 0 {
		t.Fatalf("plan at width 3 kept fields: %+v", plan)
	}
	if !slices.Equal(enabled, before) {
		t.Fatalf("caller's fields became %v after a narrow plan, want %v", enabled, before)
	}
}

// TestStatusLineDropNextLeavesItsInputIntact pins the full slice expression in
// filePaneStatusDropNext. Without the capacity cap, append would shift the tail
// down inside the input's own backing array, so the caller's slice would come
// back holding a duplicated last field. Repeated drops are exercised because
// each one takes a prefix of the previous result, and a single missing cap
// anywhere in that chain writes back into the original.
func TestStatusLineDropNextLeavesItsInputIntact(t *testing.T) {
	fields := []filePaneStatusField{
		filePaneStatusFieldSize, filePaneStatusFieldDate, filePaneStatusFieldPerms,
		filePaneStatusFieldOwner,
	}
	before := slices.Clone(fields)

	active := fields
	for {
		next, ok := filePaneStatusDropNext(active)
		if !slices.Equal(fields, before) {
			t.Fatalf("filePaneStatusDropNext wrote through to its input: %v, want %v", fields, before)
		}
		if !ok {
			if len(next) != 1 || next[0] != filePaneStatusFieldSize {
				t.Fatalf("degradation bottomed out at %v, want just size", next)
			}
			return
		}
		if len(next) != len(active)-1 {
			t.Fatalf("drop produced %d fields from %d", len(next), len(active))
		}
		active = next
	}
}

// TestFitFilePaneStatusName pins the px-to-capacity fit: a name over its column
// comes back as compactName's marker-plus-tail form at the largest capacity
// that measures within the column.
func TestFitFilePaneStatusName(t *testing.T) {
	const name = "gpstrack-dashboard-server.go"
	pane := testStatusPane([]filesys.Entry{
		{Name: name, Kind: filesys.EntryFile},
	}, 0)

	if got := fitFilePaneStatusName(pane, name, measureByRunes(name), measureByRunes); got != name {
		t.Fatalf("a name at exactly its column width = %q, want it untouched", got)
	}
	got := fitFilePaneStatusName(pane, name, 12, measureByRunes)
	if measureByRunes(got) > 12 {
		t.Fatalf("fitted name %q measures %d, over the 12 budget", got, measureByRunes(got))
	}
	if !strings.Contains(got, "..") || !strings.HasSuffix(got, ".go") {
		t.Fatalf("fitted name %q, want the marker-plus-extension-tail compaction", got)
	}
	if want := filePaneStatusNameValue(pane, 12); got != want {
		t.Fatalf("fitted name %q, want the full 12-rune capacity %q used", got, want)
	}
	if got := fitFilePaneStatusName(pane, name, 0, measureByRunes); got != "" {
		t.Fatalf("zero-width fit = %q, want empty", got)
	}
}

// statusBarSemanticLabel is one rendered text of the strip together with the
// rectangle Gio actually placed it in. The rectangle comes from the frame's
// semantic tree, which is the only public window onto where a widget landed:
// material.Label tags its text with semantic.LabelOp from inside its own clip
// area, so that area's transformed bounds carry every offset the layout
// applied.
type statusBarSemanticLabel struct {
	text   string
	bounds image.Rectangle
}

// statusBarSemanticLabels lays the strip out for one pane and returns every
// label it drew, in tree order.
func statusBarSemanticLabels(
	t *testing.T,
	ui *UI,
	th *material.Theme,
	idx int,
	pane *filePaneState,
	cs layout.Constraints,
	now time.Time,
) []statusBarSemanticLabel {
	t.Helper()

	var router input.Router
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Now:         now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: cs,
	}
	ui.layoutFilePaneStatusBar(th, gtx, idx, pane, filePanePaletteFromConfig(ui.fmCfg))
	router.Frame(gtx.Ops)
	var out []statusBarSemanticLabel
	for _, node := range router.AppendSemantics(nil) {
		if node.Desc.Label != "" {
			out = append(out, statusBarSemanticLabel{text: node.Desc.Label, bounds: node.Desc.Bounds})
		}
	}
	return out
}

func findStatusBarLabel(labels []statusBarSemanticLabel, match func(string) bool) (statusBarSemanticLabel, bool) {
	for _, l := range labels {
		if match(l.text) {
			return l, true
		}
	}
	return statusBarSemanticLabel{}, false
}

// statusInfoPane builds a pane the file info line has something to say about.
func statusInfoPane(t *testing.T) *filePaneState {
	t.Helper()
	pane := testStatusPane([]filesys.Entry{
		{Name: "f", Kind: filesys.EntryFile, SizeBytes: 2516582, DateText: "2026-08-30 14:22"},
	}, 0)
	pane.dir = t.TempDir()
	return pane
}

func statusBarTestContext(now time.Time) layout.Context {
	return layout.Context{
		Ops:         new(op.Ops),
		Now:         now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(320, 80)),
	}
}

// statusBarWideConstraints is the shape layoutFilePane really hands the strip —
// no minimum — at a width where the default fields plus free space all fit, so
// the anchoring tests are not muddied by degradation.
func statusBarWideConstraints() layout.Constraints {
	return layout.Constraints{Max: image.Pt(640, 80)}
}

// primeStatusPaneVolume plants a landed volume reading directly on the pane,
// so tests that only need the free-space text do not have to run the async
// lookup pipeline (settleVolumeLookup covers that path).
func primeStatusPaneVolume(pane *filePaneState, freeBytes, totalBytes uint64) {
	pane.volumeBadge.label = "primed"
	pane.volumeBadge.freeBytes = freeBytes
	pane.volumeBadge.totalBytes = totalBytes
	pane.volumeBadge.checkedAt = time.Unix(1700000000, 0)
}

// TestStatusBarBranchPriority drives the real branch decision that
// layoutFilePaneStatusBar consults, so gating a progress branch behind
// StatusBarConfig fails here rather than passing a re-derived copy of the rule.
func TestStatusBarBranchPriority(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		extracting bool
		pasting    bool
		want       filePaneStatusBarBranch
	}{
		{"info line when idle", true, false, false, filePaneStatusBarBranchFileInfo},
		{"extraction wins over info", true, true, false, filePaneStatusBarBranchArchiveExtract},
		{"extraction survives the bar being off", false, true, false, filePaneStatusBarBranchArchiveExtract},
		{"paste wins over info", true, false, true, filePaneStatusBarBranchDirectPaste},
		{"paste survives the bar being off", false, false, true, filePaneStatusBarBranchDirectPaste},
		{"extraction outranks paste", true, true, true, filePaneStatusBarBranchArchiveExtract},
		{"nothing when off and idle", false, false, false, filePaneStatusBarBranchNone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fm.DefaultConfig()
			cfg.StatusBar.Enabled = tc.enabled
			ui := NewUI(cfg)
			pane := statusInfoPane(t)
			pane.table.SetMode(table.ModeBrief)
			ui.filePanes = []*filePaneState{pane}
			ui.activeFilePane = 0

			if tc.extracting {
				ui.archiveExtract = &archiveExtractState{
					pane:        0,
					archivePath: filepath.Join(pane.dir, "bundle.zip"),
					startedAt:   time.Unix(1700000000, 0),
				}
			}
			if tc.pasting {
				ui.fileCopy = &fileCopyState{
					pane:        0,
					srcPane:     0,
					dstPane:     0,
					srcPath:     filepath.Join(pane.dir, "movie.mkv"),
					directPaste: true,
					running:     true,
					startedAt:   time.Unix(1700000000, 0),
				}
			}

			if got := ui.filePaneStatusBarBranch(0, pane); got != tc.want {
				t.Fatalf("filePaneStatusBarBranch() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestLayoutFilePaneStatusBarDrawsFileInfoLine is the end-to-end wiring proof:
// an idle pane with the bar enabled renders a full-width strip, and the same
// pane renders nothing once the bar is turned off.
func TestLayoutFilePaneStatusBarDrawsFileInfoLine(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() { localVolumeUsageFunc = oldLookup }()
	localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
		return platform.VolumeUsage{FreeBytes: 64 << 30, TotalBytes: 512 << 30}, nil
	}

	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	pane := statusInfoPane(t)
	ui.filePanes = []*filePaneState{pane}
	ui.activeFilePane = 0

	th := material.NewTheme()
	now := time.Unix(1700000000, 0)

	dims := ui.layoutFilePaneStatusBar(th, statusBarTestContext(now), 0, pane, filePanePaletteFromConfig(cfg))
	if dims.Size.X != 320 || dims.Size.Y <= 0 {
		t.Fatalf("idle status bar size = %v, want a full-width 320px strip", dims.Size)
	}

	cfg.StatusBar.Enabled = false
	if got := ui.layoutFilePaneStatusBar(th, statusBarTestContext(now), 0, pane, filePanePaletteFromConfig(cfg)); got != (layout.Dimensions{}) {
		t.Fatalf("disabled status bar size = %v, want nothing drawn", got.Size)
	}
}

// TestStatusBarAnchorsBothRegions pins Revision 2's geometry: the name column's
// text begins at the pane's left inset, and the free-space region — carrying
// its leading "│" because both regions render — ends at the right inset. The
// constraint shape has no minimum width, which is what layoutFilePane really
// hands the strip, so a right anchor that collapses without Min.X fails here.
func TestStatusBarAnchorsBothRegions(t *testing.T) {
	cfg := fm.DefaultConfig() // size, date, free
	ui := NewUI(cfg)
	pane := statusInfoPane(t)
	primeStatusPaneVolume(pane, 64<<30, 512<<30)
	ui.filePanes = []*filePaneState{pane}

	th := material.NewTheme()
	now := time.Unix(1700000000, 0)
	labels := statusBarSemanticLabels(t, ui, th, 0, pane, statusBarWideConstraints(), now)

	name, ok := findStatusBarLabel(labels, func(s string) bool { return s == "f" })
	if !ok {
		t.Fatalf("no name label among %v", labels)
	}
	inset := 8 // filePaneStatusBarInsetX at PxPerDp 1
	if name.bounds.Min.X != inset {
		t.Errorf("name label starts at x=%d, want the left inset at %d", name.bounds.Min.X, inset)
	}

	const wantFree = "64.00 GB free (13%)"
	free, ok := findStatusBarLabel(labels, func(s string) bool { return s == wantFree })
	if !ok {
		t.Fatalf("no free-region label %q among %v", wantFree, labels)
	}
	width := ui.measureFilePaneStatusBarTextWidth(th, statusBarTestContext(now), free.text)
	if got, want := free.bounds.Min.X+width, 640-inset; got != want {
		t.Errorf("free region spans x=[%d,%d), want its right edge on the inset at %d", free.bounds.Min.X, got, want)
	}

	// Both regions render, so the "│" rule sits between them, ending where the
	// free text begins.
	rule, ok := findStatusBarLabel(labels, func(s string) bool { return s == "│" })
	if !ok {
		t.Fatalf("no region separator among %v", labels)
	}
	if got := rule.bounds.Min.X + ui.measureFilePaneStatusBarTextWidth(th, statusBarTestContext(now), filePaneStatusRegionSeparator); got != free.bounds.Min.X {
		t.Errorf("region separator spans x=[%d,%d), want it to end where the free text begins at %d", rule.bounds.Min.X, got, free.bounds.Min.X)
	}
}

// TestStatusBarColumnsIgnoreTheCursorRow is the unit-level no-jump proof: two
// renders differing only in the selected row must place every label except the
// name's text identically. The two entries share size, date and permissions so
// even the VALUES outside the name column are byte-identical.
func TestStatusBarColumnsIgnoreTheCursorRow(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.StatusBar.Fields = []string{fm.StatusBarFieldSize, fm.StatusBarFieldDate}
	ui := NewUI(cfg)
	entries := []filesys.Entry{
		{Name: "a.md", Kind: filesys.EntryFile, SizeBytes: 2048, DateText: "2026-08-30 14:22"},
		{Name: "gpstrack-dashboard-server.go", Kind: filesys.EntryFile, SizeBytes: 2048, DateText: "2026-08-30 14:22"},
	}
	pane := testStatusPane(entries, 0)
	pane.dir = t.TempDir()
	ui.filePanes = []*filePaneState{pane}

	th := material.NewTheme()
	now := time.Unix(1700000000, 0)
	render := func(selected int) []statusBarSemanticLabel {
		pane.table.Selected = selected
		return statusBarSemanticLabels(t, ui, th, 0, pane, statusBarWideConstraints(), now)
	}

	short := render(0)
	long := render(1)
	if len(short) != len(long) {
		t.Fatalf("label count changed with the cursor: %v vs %v", short, long)
	}
	names := map[string]bool{"a.md": true, "gpstrack-dashboard-server.go": true}
	for i := range short {
		a, b := short[i], long[i]
		if names[a.text] && names[b.text] {
			// The name label's text differs by design; its column must not.
			if a.bounds.Min != b.bounds.Min {
				t.Errorf("name label moved from %v to %v with the cursor", a.bounds.Min, b.bounds.Min)
			}
			continue
		}
		if a.text != b.text || a.bounds != b.bounds {
			t.Errorf("label %d changed with the cursor: %q at %v vs %q at %v", i, a.text, a.bounds, b.text, b.bounds)
		}
	}
}

// TestStatusBarMarkedModeReplacesTheLeftCluster: marked rows swap the whole
// per-entry cluster for the two-part summary, and the free region — anchored
// right, decided by the same plan — must not move when the mode flips.
func TestStatusBarMarkedModeReplacesTheLeftCluster(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	entries := []filesys.Entry{
		{Name: "a.md", Kind: filesys.EntryFile, SizeBytes: 2516582, DateText: "2026-08-30 14:22"},
		{Name: "b.md", Kind: filesys.EntryFile, SizeBytes: 2516582, DateText: "2026-08-30 14:22"},
	}
	pane := testStatusPane(entries, 0)
	pane.dir = t.TempDir()
	primeStatusPaneVolume(pane, 64<<30, 512<<30)
	ui.filePanes = []*filePaneState{pane}

	th := material.NewTheme()
	now := time.Unix(1700000000, 0)
	isFree := func(s string) bool { return strings.HasSuffix(s, "free (13%)") }

	plain := statusBarSemanticLabels(t, ui, th, 0, pane, statusBarWideConstraints(), now)
	freeBefore, ok := findStatusBarLabel(plain, isFree)
	if !ok {
		t.Fatalf("no free label before marking: %v", plain)
	}
	if _, ok := findStatusBarLabel(plain, func(s string) bool { return s == "a.md" }); !ok {
		t.Fatalf("no per-entry name label before marking: %v", plain)
	}

	pane.markedRows = map[int]struct{}{0: {}, 1: {}}
	marked := statusBarSemanticLabels(t, ui, th, 0, pane, statusBarWideConstraints(), now)
	const wantSummary = "2 items selected" + filePaneStatusColumnSeparator + "4.80 MB"
	if _, ok := findStatusBarLabel(marked, func(s string) bool { return s == wantSummary }); !ok {
		t.Fatalf("no marked summary %q among %v", wantSummary, marked)
	}
	if _, ok := findStatusBarLabel(marked, func(s string) bool { return s == "a.md" }); ok {
		t.Fatalf("per-entry name label survived into marked mode: %v", marked)
	}
	freeDuring, ok := findStatusBarLabel(marked, isFree)
	if !ok {
		t.Fatalf("free label vanished in marked mode: %v", marked)
	}
	// Compared by origin, not by full rectangle: a semantic area's Max carries
	// the constraint space the label was offered, which legitimately differs
	// between the two modes; where the text starts is what must not move.
	if freeDuring.bounds.Min != freeBefore.bounds.Min || freeDuring.text != freeBefore.text {
		t.Fatalf("free region moved on entering marked mode: %v/%q vs %v/%q",
			freeDuring.bounds.Min, freeDuring.text, freeBefore.bounds.Min, freeBefore.text)
	}

	pane.markedRows = nil
	after := statusBarSemanticLabels(t, ui, th, 0, pane, statusBarWideConstraints(), now)
	freeAfter, ok := findStatusBarLabel(after, isFree)
	if !ok || freeAfter.bounds.Min != freeBefore.bounds.Min {
		t.Fatalf("free region did not return to %v after unmarking (ok=%v, got %v)", freeBefore.bounds.Min, ok, freeAfter.bounds.Min)
	}
}

// TestStatusBarRegionSeparatorNeedsBothRegions: the "│" belongs to neither
// region alone. With free space as the only configured field and an empty
// listing there is no left cluster at all, so the free text stands bare; a
// populated listing carries the separator. (Configured field COLUMNS count as
// a rendered left cluster even when their values are empty — their presence is
// a property of the configuration, which is what keeps the separator from
// flickering with the cursor.)
func TestStatusBarRegionSeparatorNeedsBothRegions(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.StatusBar.Fields = []string{fm.StatusBarFieldFree}
	ui := NewUI(cfg)
	th := material.NewTheme()
	now := time.Unix(1700000000, 0)

	empty := testStatusPane(nil, 0)
	empty.dir = t.TempDir()
	primeStatusPaneVolume(empty, 64<<30, 512<<30)
	ui.filePanes = []*filePaneState{empty}
	labels := statusBarSemanticLabels(t, ui, th, 0, empty, statusBarWideConstraints(), now)
	if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == "64.00 GB free (13%)" }); !ok {
		t.Fatalf("empty listing: want the bare free label, got %v", labels)
	}
	if _, ok := findStatusBarLabel(labels, func(s string) bool { return strings.Contains(s, "│") }); ok {
		t.Fatalf("empty listing: the region separator rendered with no left cluster: %v", labels)
	}

	populated := statusInfoPane(t)
	primeStatusPaneVolume(populated, 64<<30, 512<<30)
	ui.filePanes = []*filePaneState{populated}
	labels = statusBarSemanticLabels(t, ui, th, 0, populated, statusBarWideConstraints(), now)
	if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == "│" }); !ok {
		t.Fatalf("populated listing: want the region separator between the name and the free text, got %v", labels)
	}
	if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == "64.00 GB free (13%)" }); !ok {
		t.Fatalf("populated listing: want the free label, got %v", labels)
	}
}

// TestStatusBarMeasureCacheFollowsTheListingGeneration pins the caching
// contract: the O(entries) width scan runs once per (listing, metrics,
// samples) key, not per frame. The cache is poked with a sentinel value that
// only a recompute can overwrite.
func TestStatusBarMeasureCacheFollowsTheListingGeneration(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	pane := statusInfoPane(t)
	ui.filePanes = []*filePaneState{pane}
	th := material.NewTheme()
	gtx := statusBarTestContext(time.Unix(1700000000, 0))

	state := ui.filePaneStatusBarMeasure(th, gtx, pane)
	if !state.valid {
		t.Fatal("first measure did not validate the cache")
	}
	state.widths.namePx = 424242
	if got := ui.filePaneStatusBarMeasure(th, gtx, pane); got.widths.namePx != 424242 {
		t.Fatalf("an unchanged frame recomputed the widths (namePx = %d); the scan must be once per listing", got.widths.namePx)
	}

	pane.model.setEntries([]filesys.Entry{
		{Name: "renamed.txt", Kind: filesys.EntryFile, SizeBytes: 1},
	})
	if got := ui.filePaneStatusBarMeasure(th, gtx, pane); got.widths.namePx == 424242 || got.widths.namePx <= 0 {
		t.Fatalf("a new listing did not remeasure (namePx = %d)", got.widths.namePx)
	}

	state = ui.filePaneStatusBarMeasure(th, gtx, pane)
	state.widths.namePx = 424242
	scaled := gtx
	scaled.Metric = unit.Metric{PxPerDp: 2, PxPerSp: 2}
	if got := ui.filePaneStatusBarMeasure(th, scaled, pane); got.widths.namePx == 424242 {
		t.Fatal("a metrics change did not remeasure")
	}
}

// TestStatusColumnWidthsFollowTheStatusBarDateFormat pins the width half of
// Revision 2.1's date layout: the date column's fixed width derives from the
// CHOSEN layout's sample rendering — auto measures DateFormats[0] as before,
// and a fixed key measures its own layout.
func TestStatusColumnWidthsFollowTheStatusBarDateFormat(t *testing.T) {
	pane := testStatusPane(statusColumnEntries(), 1)

	pane.model.cfg.StatusBar.DateFormat = fm.StatusBarDateFormatShort
	w := computeFilePaneStatusColumnWidths(pane, measureByRunes)
	if want := measureByRunes(filePaneStatusDateSampleTime.Format("01-02 15:04")); w.datePx != want {
		t.Fatalf("short datePx = %d, want the short layout sample's %d", w.datePx, want)
	}

	pane.model.cfg.StatusBar.DateFormat = fm.StatusBarDateFormatUS
	w = computeFilePaneStatusColumnWidths(pane, measureByRunes)
	if want := measureByRunes(filePaneStatusDateSampleTime.Format("01/02/2006 3:04 PM")); w.datePx != want {
		t.Fatalf("us datePx = %d, want the us layout sample's %d", w.datePx, want)
	}

	// Auto keeps following the configured DateFormats[0].
	pane.model.cfg.StatusBar.DateFormat = fm.StatusBarDateFormatAuto
	w = computeFilePaneStatusColumnWidths(pane, measureByRunes)
	if want := measureByRunes(filePaneStatusDateSampleTime.Format("Jan 02 2006 15:04")); w.datePx != want {
		t.Fatalf("auto datePx = %d, want the DateFormats[0] sample's %d", w.datePx, want)
	}
}

// TestStatusBarMeasureCacheFollowsTheDateFormat extends the caching contract to
// the new config input: switching status_bar.date_format must invalidate the
// cached widths (the key already carries the date sample string, which the
// chosen layout now feeds) and the re-rendered strip must carry the date in the
// new layout.
func TestStatusBarMeasureCacheFollowsTheDateFormat(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	entry, ts := statusFieldDateSample()
	pane := testStatusPane([]filesys.Entry{entry}, 0)
	pane.dir = t.TempDir()
	ui.filePanes = []*filePaneState{pane}
	th := material.NewTheme()
	gtx := statusBarTestContext(time.Unix(1700000000, 0))

	state := ui.filePaneStatusBarMeasure(th, gtx, pane)
	autoPx := state.widths.datePx
	state.widths.namePx = 424242 // sentinel only a recompute can overwrite

	pane.model.cfg.StatusBar.DateFormat = fm.StatusBarDateFormatShort
	state = ui.filePaneStatusBarMeasure(th, gtx, pane)
	if state.widths.namePx == 424242 {
		t.Fatal("switching date_format did not remeasure the cached widths")
	}
	if want := ui.measureFilePaneStatusBarTextWidth(th, gtx, filePaneStatusDateSampleTime.Format("01-02 15:04")); state.widths.datePx != want {
		t.Fatalf("short datePx = %d, want the short layout sample's %d", state.widths.datePx, want)
	}
	if state.widths.datePx >= autoPx {
		t.Fatalf("short datePx = %d, not narrower than auto's %d; the column width is not following the chosen layout", state.widths.datePx, autoPx)
	}

	// And the strip renders the value in the new layout.
	labels := statusBarSemanticLabels(t, ui, th, 0, pane, statusBarWideConstraints(), time.Unix(1700000000, 0))
	want := ts.Format("01-02 15:04")
	if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == want }); !ok {
		t.Fatalf("labels = %v, want the short-layout date %q", labels, want)
	}
}

// TestStatusBarMeasureCacheFollowsTheCompactionHead extends the caching
// contract to NameCompact.KeepStartChars. The cached compacted cursor name
// (fitValue) is built by compactName, which reads the marker AND the head
// length, but the fit cache is keyed on (name, column width) alone — so the
// measure key must carry KeepStartChars alongside the marker, or a
// KeepStartChars change keeps serving the old compaction until the next cursor
// move or resize. No live path edits KeepStartChars today (it is yaml-only),
// which is exactly why nothing but this test would catch the stale cache.
func TestStatusBarMeasureCacheFollowsTheCompactionHead(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	pane := statusInfoPane(t)
	ui.filePanes = []*filePaneState{pane}
	th := material.NewTheme()
	gtx := statusBarTestContext(time.Unix(1700000000, 0))

	state := ui.filePaneStatusBarMeasure(th, gtx, pane)
	state.widths.namePx = 424242 // sentinel only a recompute can overwrite
	state.fitName, state.fitPx, state.fitValue = "stale-name", 77, "stale-fit"

	pane.model.cfg.NameCompact.KeepStartChars += 3
	state = ui.filePaneStatusBarMeasure(th, gtx, pane)
	if state.widths.namePx == 424242 {
		t.Fatal("changing NameCompact.KeepStartChars did not invalidate the measure cache")
	}
	if state.fitName == "stale-name" || state.fitValue == "stale-fit" {
		t.Fatalf("changing NameCompact.KeepStartChars left the compacted-name cache holding (%q, %q)", state.fitName, state.fitValue)
	}
}

// TestStatusBarFreeLabelRendersCachedVolumeBytes pins the raw-byte plumbing:
// the free region must format from the counts the volume poll cached — not the
// badge's already-formatted label — and reuse them across frames without new
// lookups.
func TestStatusBarFreeLabelRendersCachedVolumeBytes(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() { localVolumeUsageFunc = oldLookup }()
	var lookups atomic.Int64
	localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
		lookups.Add(1)
		return platform.VolumeUsage{FreeBytes: 64 << 30, TotalBytes: 512 << 30}, nil
	}

	cfg := fm.DefaultConfig()
	cfg.StatusBar.Fields = []string{fm.StatusBarFieldSize, fm.StatusBarFieldFree}
	ui := NewUI(cfg)
	pane := statusInfoPane(t)
	ui.filePanes = []*filePaneState{pane}
	th := material.NewTheme()

	// The lookup runs off-frame, so the counts only reach the strip once
	// pumpFilePaneVolumeLookups has landed a reading.
	landed := settleVolumeLookup(t, ui, pane, time.Unix(1700000000, 0))

	// 64/512 is 12.5%, which pins the round-half-up through the layout path too.
	const wantFree = "64.00 GB free (13%)"
	labels := statusBarSemanticLabels(t, ui, th, 0, pane, statusBarWideConstraints(), landed)
	if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == wantFree }); !ok {
		t.Fatalf("labels = %v, want the free region %q", labels, wantFree)
	}
	if got := lookups.Load(); got != 1 {
		t.Fatalf("volume lookups = %d, want 1", got)
	}

	// Further frames inside the poll window must reuse the cached counts without
	// starting another lookup.
	for frame := range 30 {
		now := landed.Add(time.Duration(frame) * 16 * time.Millisecond)
		gtx := statusBarTestContext(now)
		ui.pumpFilePaneVolumeLookups(gtx)
		labels := statusBarSemanticLabels(t, ui, th, 0, pane, statusBarWideConstraints(), now)
		if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == wantFree }); !ok {
			t.Fatalf("frame %d: labels = %v, want the cached free region %q", frame, labels, wantFree)
		}
	}
	if got := lookups.Load(); got != 1 {
		t.Fatalf("volume lookups after 30 cached frames = %d, want 1", got)
	}
}

// TestStatusBarOmitsFreeSpaceUntilTheLookupLands pins the totalBytes == 0
// contract through the layout: a pane whose volume lookup failed shows the rest
// of the line rather than an empty or half-formed free region.
func TestStatusBarOmitsFreeSpaceUntilTheLookupLands(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() { localVolumeUsageFunc = oldLookup }()
	localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
		return platform.VolumeUsage{}, errors.New("no such volume")
	}

	cfg := fm.DefaultConfig()
	cfg.StatusBar.Fields = []string{fm.StatusBarFieldSize, fm.StatusBarFieldFree}
	ui := NewUI(cfg)
	pane := statusInfoPane(t)
	ui.filePanes = []*filePaneState{pane}

	labels := statusBarSemanticLabels(t, ui, material.NewTheme(), 0, pane, statusBarWideConstraints(), time.Unix(1700000000, 0))
	if _, ok := findStatusBarLabel(labels, func(s string) bool { return strings.Contains(s, "free") || strings.Contains(s, "│") }); ok {
		t.Fatalf("labels = %v, want no free region before the lookup lands", labels)
	}
	if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == "2.40 MB" }); !ok {
		t.Fatalf("labels = %v, want the size column regardless", labels)
	}
}

// TestStatusBarSeamSeparatorHugsThePaneEdge pins where the left pane's trailing
// "|" lands — for the PROGRESS lines, the only strips that still carry a
// pane-seam separator (the info line's was retired by Revision 2's anchored
// layout). layoutFilePane's outer Stack has only an Expanded child and no
// Stacked ones, so Gio leaves maxSZ at zero and the strip is laid out with
// Constraints.Min.X == 0. layout.E has nothing to align against at that width
// and collapses to the label's natural width, floating the separator mid-pane
// instead of hugging the seam — which defeats the whole point of drawing it.
// layoutFilePaneStatusBar therefore widens Min.X to Max.X itself, and the two
// constraint shapes below must produce identical alignment.
func TestStatusBarSeamSeparatorHugsThePaneEdge(t *testing.T) {
	const paneWidth = 320

	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	left := statusInfoPane(t)
	right := statusInfoPane(t)
	ui.filePanes = []*filePaneState{left, right}

	th := material.NewTheme()
	now := time.Unix(1700000000, 0)
	metrics := statusBarTestContext(now)

	extractInto := func(idx int, pane *filePaneState) {
		ui.archiveExtract = &archiveExtractState{
			pane:        idx,
			archivePath: filepath.Join(pane.dir, "bundle.zip"),
			startedAt:   now.Add(-time.Second),
		}
	}

	tests := []struct {
		name string
		cs   layout.Constraints
	}{
		// What layoutFilePane really hands the strip.
		{"no minimum width", layout.Constraints{Max: image.Pt(paneWidth, 80)}},
		// The shape statusBarTestContext uses, which masks the defect because
		// Min already equals Max.
		{"exact width", layout.Exact(image.Pt(paneWidth, 80))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			extractInto(0, left)
			labels := statusBarSemanticLabels(t, ui, th, 0, left, tc.cs, now)
			label, ok := findStatusBarLabel(labels, func(s string) bool { return strings.HasSuffix(s, " |") })
			if !ok {
				t.Fatalf("left pane labels = %v, want one carrying the trailing seam separator", labels)
			}
			width := ui.measureFilePaneStatusBarTextWidth(th, metrics, label.text)
			if got := label.bounds.Min.X + width; got != paneWidth {
				t.Fatalf("label %q spans x=[%d,%d), want its right edge on the pane's at %d",
					label.text, label.bounds.Min.X, got, paneWidth)
			}
		})
	}

	// The other pane must stay put: its separator leads, so the label belongs on
	// the pane's left edge and nothing should be aligning it away from there.
	extractInto(1, right)
	labels := statusBarSemanticLabels(t, ui, th, 1, right, layout.Constraints{Max: image.Pt(paneWidth, 80)}, now)
	label, ok := findStatusBarLabel(labels, func(s string) bool { return strings.HasPrefix(s, "| ") })
	if !ok {
		t.Fatalf("right pane labels = %v, want one carrying the leading seam separator", labels)
	}
	if label.bounds.Min.X != 0 {
		t.Fatalf("right pane label starts at x=%d, want the pane's left edge at 0", label.bounds.Min.X)
	}
}

// TestStatusBarInfoLineKeepsItsHeightWhenEmpty pins the strip against a layout
// jump. The anchored row always holds at least one label — the name column, or
// an explicit empty one — and Gio gives a single-line label the font's full
// line box even with no text in it, so the strip's height must be identical
// across rows however empty their values come out. Only the info branch is
// exempted — the progress branches still draw nothing when they have nothing
// to report.
func TestStatusBarInfoLineKeepsItsHeightWhenEmpty(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.StatusBar.Fields = []string{fm.StatusBarFieldDate}
	ui := NewUI(cfg)
	th := material.NewTheme()
	now := time.Unix(1700000000, 0)

	entries := []filesys.Entry{
		{Name: "..", Kind: filesys.EntryParent},
		{Name: "f", Kind: filesys.EntryFile, SizeBytes: 2516582, DateText: "2026-08-30 14:22"},
	}
	// Min.Y is zero here on purpose: the strip is a Rigid child of a vertical
	// Flex, so that is the shape it is really handed. statusBarTestContext's
	// layout.Exact would pin the height from the outside and prove nothing.
	gtx := func() layout.Context {
		c := statusBarTestContext(now)
		c.Constraints = layout.Constraints{Max: image.Pt(320, 80)}
		return c
	}
	stripFor := func(row int) layout.Dimensions {
		pane := testStatusPane(entries, row)
		pane.dir = t.TempDir()
		ui.filePanes = []*filePaneState{pane}
		return ui.layoutFilePaneStatusBar(th, gtx(), 0, pane, filePanePaletteFromConfig(cfg))
	}

	dated := stripFor(1)
	if dated.Size.Y <= 0 {
		t.Fatalf("dated row strip height = %d, want positive", dated.Size.Y)
	}
	if parent := stripFor(0); parent.Size.Y != dated.Size.Y {
		t.Fatalf("parent-row strip height = %d, want the %d an occupied line takes", parent.Size.Y, dated.Size.Y)
	}

	// A pane with no listing at all still reserves the strip's height.
	if empty := stripFor(0); empty.Size.Y != dated.Size.Y {
		t.Fatalf("empty-value strip height = %d, want %d", empty.Size.Y, dated.Size.Y)
	}

	// The exemption is scoped to the info branch. On the very same ".." row an
	// extraction still outranks it and draws its own line, so nothing here turned
	// the progress branches into an always-on band.
	pane := testStatusPane(entries, 0)
	pane.dir = t.TempDir()
	ui.filePanes = []*filePaneState{pane}
	ui.archiveExtract = &archiveExtractState{
		pane:        0,
		archivePath: filepath.Join(pane.dir, "bundle.zip"),
		startedAt:   now.Add(-time.Second),
	}
	labels := statusBarSemanticLabels(t, ui, th, 0, pane, layout.Constraints{Max: image.Pt(320, 80)}, now)
	if _, ok := findStatusBarLabel(labels, func(s string) bool { return strings.HasPrefix(s, "[Extracting] ") }); !ok {
		t.Fatalf("extraction strip labels = %v, want the extraction line", labels)
	}
}

// TestStatusBarFileInfoDoesNotTickTheFrameClock guards battery life: the
// progress lines carry a live readout and repaint at 4 FPS, but the idle info
// line changes only on events that already invalidate, so it must not keep an
// otherwise idle window redrawing. Its only wakeup is the 15s volume poll.
//
// The wakeup comes from pumpFilePaneVolumeLookups rather than from the layout
// path, so a frame here is the pump plus the layout, which is the order
// ui.Layout runs them in. Measuring across the whole frame keeps the guard's
// teeth: a layout that ticked, or a pump that scheduled an immediate repaint
// while a reading was already fresh, both still fail the equality.
func TestStatusBarFileInfoDoesNotTickTheFrameClock(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() { localVolumeUsageFunc = oldLookup }()
	localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
		return platform.VolumeUsage{FreeBytes: 64 << 30, TotalBytes: 512 << 30}, nil
	}

	now := time.Unix(1700000000, 0)
	th := material.NewTheme()

	wakeupAfter := func(t *testing.T, ui *UI, pane *filePaneState, frameNow time.Time) (time.Duration, bool) {
		t.Helper()
		var router input.Router
		gtx := statusBarTestContext(frameNow)
		gtx.Source = router.Source()
		ui.pumpFilePaneVolumeLookups(gtx)
		ui.layoutFilePaneStatusBar(th, gtx, 0, pane, filePanePaletteFromConfig(ui.fmCfg))
		at, ok := router.WakeupTime()
		if !ok {
			return 0, false
		}
		return at.Sub(frameNow), true
	}

	// Asserted as an equality, not as a lower bound: "no wakeup at all" would
	// satisfy a lower bound, and that is exactly what deleting the volume poll's
	// InvalidateCmd produces. Free space would then go stale for as long as the
	// window sat idle, with nothing failing.
	idle := NewUI(fm.DefaultConfig())
	idlePane := statusInfoPane(t)
	idle.filePanes = []*filePaneState{idlePane}
	landed := settleVolumeLookup(t, idle, idlePane, now)
	if d, ok := wakeupAfter(t, idle, idlePane, landed); !ok || d != filePaneVolumeBadgeRefreshInterval {
		t.Fatalf("idle info line wakeup = %v (ok=%v), want exactly the %v volume poll", d, ok, filePaneVolumeBadgeRefreshInterval)
	}

	busy := NewUI(fm.DefaultConfig())
	busyPane := statusInfoPane(t)
	busy.filePanes = []*filePaneState{busyPane}
	busy.archiveExtract = &archiveExtractState{
		pane:        0,
		archivePath: filepath.Join(busyPane.dir, "bundle.zip"),
		startedAt:   now.Add(-time.Second),
		progress: filesys.CopyProgress{
			BytesDone:   50 << 20,
			BytesTotal:  100 << 20,
			CurrentPath: filepath.Join("bundle.zip", "movie.mkv"),
		},
	}
	d, ok := wakeupAfter(t, busy, busyPane, now)
	if !ok || d != archiveExtractStatusRefreshInterval {
		t.Fatalf("extraction wakeup = %v (ok=%v), want %v", d, ok, archiveExtractStatusRefreshInterval)
	}
}
