// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/widget/table"

	"gioui.org/io/input"
	"gioui.org/widget/material"
)

// allFilePaneStatusFields is the hand-maintained roster of the
// filePaneStatusField enum, kept in one place so the guard tests below cannot
// drift apart from each other. Go cannot enumerate the values of an enum, so
// adding a seventh field means adding it here too — that is the one manual step
// these tests still depend on, exactly as in fm/config_test.go.
var allFilePaneStatusFields = []filePaneStatusField{
	filePaneStatusFieldSize,
	filePaneStatusFieldDate,
	filePaneStatusFieldPerms,
	filePaneStatusFieldOwner,
	filePaneStatusFieldFree,
}

// testStatusPane builds a pane with no columns. The status bar reads only
// table.Selected and table.Mode, never the columns, so table.New(nil) is enough
// and avoids dragging the real column definitions into these tests.
func testStatusPane(entries []filesys.Entry, selected int) *filePaneState {
	cfg := fm.DefaultConfig()
	pane := &filePaneState{}
	pane.model = &filePaneModel{cfg: cfg, entries: entries}
	pane.table = table.New(nil)
	pane.table.Selected = selected
	return pane
}

func TestStatusFieldSize(t *testing.T) {
	entries := []filesys.Entry{
		{Name: "..", Kind: filesys.EntryParent},
		{Name: "dir", Kind: filesys.EntryDir},
		{Name: "file.txt", Kind: filesys.EntryFile, SizeBytes: 2516582},
	}
	tests := []struct {
		row  int
		want string
	}{
		{0, "<UP>"},
		{1, "<DIR>"},
		{2, "2.40 MB"},
	}
	for _, tc := range tests {
		pane := testStatusPane(entries, tc.row)
		got := filePaneStatusFieldValue(pane, filePaneStatusFieldSize, "")
		if got != tc.want {
			t.Fatalf("row %d size = %q, want %q", tc.row, got, tc.want)
		}
	}
}

func TestStatusFieldDateAndPermsEmptyOnParent(t *testing.T) {
	entries := []filesys.Entry{{Name: "..", Kind: filesys.EntryParent}}
	pane := testStatusPane(entries, 0)
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldDate, ""); got != "" {
		t.Fatalf("parent date = %q, want empty", got)
	}
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldPerms, ""); got != "" {
		t.Fatalf("parent perms = %q, want empty", got)
	}

	// The ".." row has no modification time, which is the branch that sends
	// filePaneModel.formatDate back to entry.DateText — empty for a parent row.
	// A configured format must not talk it into formatting the zero time, which
	// would put "0001-01-01 00:00:00" in the bar on every directory.
	pane.model.cfg.DateFormats = []string{"2006-01-02 15:04:05", "01-02"}
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldDate, ""); got != "" {
		t.Fatalf("parent date with a configured format = %q, want empty", got)
	}
}

// statusFieldDateSample is the entry the date tests below share: a real
// modification time plus the exact string filesys.formatDate would have baked
// into DateText for it. Carrying both is the point — the field has to read the
// time through the configured format and ignore the baked string.
func statusFieldDateSample() (filesys.Entry, time.Time) {
	ts := time.Date(2026, time.August, 30, 14, 22, 9, 0, time.Local)
	return filesys.Entry{
		Name: "report.pdf", Kind: filesys.EntryFile,
		ModTime: ts, DateText: ts.Format("Jan 02 2006"),
	}, ts
}

// TestStatusFieldDateFollowsConfiguredFormat is the regression test for the bar
// ignoring Settings -> File panes -> the date/time builder. The field used to
// return entry.DateText, which filesys.formatDate bakes exactly once as a
// hardcoded "Jan 02 2006": no time component, and unmoved by any ISO,
// day-first, slash or with-time preset the user can pick.
func TestStatusFieldDateFollowsConfiguredFormat(t *testing.T) {
	entry, ts := statusFieldDateSample()
	pane := testStatusPane([]filesys.Entry{entry}, 0)

	for _, format := range []string{"2006-01-02 15:04:05", "02 Jan 2006 15:04", "01/02/2006"} {
		// The remaining formats are the column's narrower fallbacks. The bar
		// must never reach for them: it degrades by dropping whole fields, not
		// by shortening one.
		pane.model.cfg.DateFormats = []string{format, "Jan 02", "01-02"}
		want := ts.Format(format)
		got := filePaneStatusFieldValue(pane, filePaneStatusFieldDate, "")
		if got != want {
			t.Fatalf("date with DateFormats[0]=%q = %q, want %q", format, got, want)
		}
		if got == entry.DateText {
			t.Fatalf("date = %q, the string filesys.formatDate baked into DateText; the field is still reading DateText instead of the configured format", got)
		}
		if got == ts.Format("Jan 02 2006") {
			t.Fatalf("date = %q, the hardcoded Jan 02 2006 form, with %q configured", got, format)
		}
	}
}

// TestStatusFieldDateIgnoresWidthNegotiation covers the seam this fix reaches
// through. filePaneModel.formatDate is the responsive Date *column's* formatter:
// it walks cfg.DateFormats and returns the richest one that fits the width it is
// handed, using exact glyph measurements when the model has a text measurer and
// an approximate character budget when it does not. The status bar wants neither
// negotiation — it drops whole fields instead — so it passes a width nothing can
// overflow and must come out with DateFormats[0] down both branches.
//
// The measurer branch is the one worth pinning. The status bar never has a
// measurer in the running app: layoutFilePaneTable installs it for the table and
// its own defer clears it again before the strip below lays out, and
// measuredTextWidth checks the measurer before the cache, so even a cache left
// warm by the table reports nothing. This test therefore runs the field with the
// bar's real state (no measurer) and then again with one installed anyway, so
// that a future caller laying the bar out mid-table cannot quietly start
// shortening the date.
func TestStatusFieldDateIgnoresWidthNegotiation(t *testing.T) {
	entry, ts := statusFieldDateSample()
	pane := testStatusPane([]filesys.Entry{entry}, 0)
	pane.model.cfg.DateFormats = []string{"2006-01-02 15:04:05", "Jan 02", "01-02"}
	want := ts.Format("2006-01-02 15:04:05")

	// State the status bar actually runs in.
	if _, ok := pane.model.measuredTextWidth("probe"); ok {
		t.Fatal("a pane model with no measurer installed reported a measurement; this test no longer covers the status bar's own path")
	}
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldDate, ""); got != want {
		t.Fatalf("date without a text measurer = %q, want the richest configured format %q", got, want)
	}

	// And with a measurer charging a realistic per-glyph width.
	pane.model.setTextMeasurer(func(text string) int { return 8 * utf8.RuneCountInString(text) })
	if _, ok := pane.model.measuredTextWidth("probe"); !ok {
		t.Fatal("setTextMeasurer did not install a measurer")
	}
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldDate, ""); got != want {
		t.Fatalf("date with a text measurer = %q, want the richest configured format %q; the width the bar passes formatDate is not generous enough", got, want)
	}
}

// TestStatusFieldDateFollowsStatusBarDateFormat covers Revision 2.1's own date
// layout for the bar. "auto" keeps the pre-existing behaviour — DateFormats[0]
// through formatDate, which is what every test above exercises — while the
// three fixed keys format the entry's ModTime directly, unmoved by whatever the
// column date builder is set to.
func TestStatusFieldDateFollowsStatusBarDateFormat(t *testing.T) {
	entry, ts := statusFieldDateSample()
	pane := testStatusPane([]filesys.Entry{entry}, 0)
	// A deliberately distinctive column format: the fixed keys must ignore it,
	// and auto must follow it.
	pane.model.cfg.DateFormats = []string{"02 Jan 2006 15:04", "Jan 02", "01-02"}

	tests := []struct {
		key  string
		want string
	}{
		{fm.StatusBarDateFormatAuto, ts.Format("02 Jan 2006 15:04")},
		{fm.StatusBarDateFormatISO, ts.Format("2006-01-02 15:04")},
		{fm.StatusBarDateFormatUS, ts.Format("01/02/2006 3:04 PM")},
		{fm.StatusBarDateFormatShort, ts.Format("01-02 15:04")},
	}
	for _, tc := range tests {
		pane.model.cfg.StatusBar.DateFormat = tc.key
		if got := filePaneStatusFieldValue(pane, filePaneStatusFieldDate, ""); got != tc.want {
			t.Fatalf("date under %q = %q, want %q", tc.key, got, tc.want)
		}
	}

	// Auto really is DateFormats[0], not a coincidence of the fixture: change
	// the column format and the auto value must move with it.
	pane.model.cfg.StatusBar.DateFormat = fm.StatusBarDateFormatAuto
	pane.model.cfg.DateFormats = []string{"2006/01/02", "01-02"}
	if got, want := filePaneStatusFieldValue(pane, filePaneStatusFieldDate, ""), ts.Format("2006/01/02"); got != want {
		t.Fatalf("auto date after a DateFormats change = %q, want %q", got, want)
	}
}

// TestStatusFieldDateZeroModTimeEmptyInEveryLayout pins the ".." row contract
// across the new layouts: a zero ModTime renders empty whatever date_format
// says, never the zero time pushed through a layout.
func TestStatusFieldDateZeroModTimeEmptyInEveryLayout(t *testing.T) {
	entries := []filesys.Entry{{Name: "..", Kind: filesys.EntryParent}}
	pane := testStatusPane(entries, 0)
	pane.model.cfg.DateFormats = []string{"2006-01-02 15:04:05"}
	for _, key := range []string{
		fm.StatusBarDateFormatAuto, fm.StatusBarDateFormatISO,
		fm.StatusBarDateFormatUS, fm.StatusBarDateFormatShort,
	} {
		pane.model.cfg.StatusBar.DateFormat = key
		if got := filePaneStatusFieldValue(pane, filePaneStatusFieldDate, ""); got != "" {
			t.Fatalf("zero-ModTime date under %q = %q, want empty", key, got)
		}
	}
}

// TestStatusFieldDateFallsBackWithoutConfiguredFormats pins formatDate's own
// fallback as seen from the bar: a config carrying no formats at all still
// prints a date rather than an empty field.
func TestStatusFieldDateFallsBackWithoutConfiguredFormats(t *testing.T) {
	entry, ts := statusFieldDateSample()
	pane := testStatusPane([]filesys.Entry{entry}, 0)
	pane.model.cfg.DateFormats = nil
	if got, want := filePaneStatusFieldValue(pane, filePaneStatusFieldDate, ""), ts.Format("Jan 02 2006"); got != want {
		t.Fatalf("date with no configured formats = %q, want the %q fallback", got, want)
	}
}

// TestSettingsStatusBarPreviewFollowsTheDraftDateFormat is the preview half of
// the same defect, and it lives here rather than beside the other preview tests
// because it is about this file's Date field. The sample entry used to carry a
// literal DateText of "2026-08-30 14:22" — ISO with a time component, a form the
// shipping bar could not produce for any setting — so the one preview whose
// stated job is that it cannot drift from what ships had drifted on this field.
//
// It now samples a real ModTime through the same formatter, and — while the
// bar's own date layout stays on "auto" — follows the draft column date format
// the way the Full mode preview beside it already does, so switching the date
// or time preset moves both previews in the same frame.
func TestSettingsStatusBarPreviewFollowsTheDraftDateFormat(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSettingsModal()
	st := ui.settingsModal
	if st == nil {
		t.Fatal("settings modal did not open")
	}
	th := material.NewTheme()
	gtx := statusBarPreviewContext(new(input.Router))

	for i := range st.statusBarFieldBools {
		st.statusBarFieldBools[i].Value = false
	}
	st.statusBarFieldBools[filePaneStatusFieldDate].Value = true

	presets := []struct{ date, time string }{
		{"iso", "seconds"},
		{"day_first", "minutes"},
		{"slash", "none"},
	}
	seen := make(map[string]string, len(presets))
	for _, preset := range presets {
		st.paneDatePreset, st.paneTimePreset = preset.date, preset.time
		want := settingsPanePreviewTime.Format(settingsGeneratedPaneDateFormats(preset.date, preset.time)[0])
		labels := settingsStatusBarPreviewLabels(t, ui, th, st, gtx)
		if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == want }); !ok {
			t.Fatalf("date preview for %s/%s renders %v, want the date column %q; the preview is not following the draft date format",
				preset.date, preset.time, settingsStatusBarPreviewLabelTexts(labels), want)
		}
		if _, ok := findStatusBarLabel(labels, func(s string) bool { return s == "2026-08-30 14:22" }); ok {
			t.Fatalf("date preview renders %q, the old literal DateText sample; the preview is not going through the formatter the bar uses", "2026-08-30 14:22")
		}
		if other, dup := seen[want]; dup {
			t.Fatalf("the %s/%s preset renders %q, the same as %s; the preview cannot tell the date formats apart", preset.date, preset.time, want, other)
		}
		seen[want] = preset.date + "/" + preset.time
	}

	// Rendering the preview must not write the draft through to the config the
	// rest of the app is still running on.
	if got, want := ui.fmCfg.DateFormats, fm.DefaultConfig().DateFormats; !slices.Equal(got, want) {
		t.Fatalf("rendering the preview rewrote the live config's date formats to %v, want %v", got, want)
	}
}

// TestSettingsStatusBarPreviewSampleGoesThroughTheDateFormatter guards the
// sample entry itself, separately from the preview line, so removing the ModTime
// or reintroducing a literal DateText fails with a message that says why.
func TestSettingsStatusBarPreviewSampleGoesThroughTheDateFormatter(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.DateFormats = []string{"2006-01-02 15:04:05", "01-02"}
	pane := settingsStatusBarPreviewPane(cfg)

	entry := pane.selectedEntry()
	if entry == nil {
		t.Fatal("the preview sample pane has no cursor entry")
	}
	if entry.ModTime.IsZero() {
		t.Fatal("the preview sample entry has no ModTime, so formatDate falls back to DateText and the preview cannot show the configured date format")
	}
	if entry.DateText != "" {
		t.Fatalf("the preview sample entry carries DateText %q; it is unreachable while ModTime is set and misleads the next reader about where the preview's date comes from", entry.DateText)
	}
	want := settingsPanePreviewTime.Format(cfg.DateFormats[0])
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldDate, ""); got != want {
		t.Fatalf("preview sample date = %q, want %q", got, want)
	}
}

func TestStatusFieldPermsFollowsFormatSetting(t *testing.T) {
	entries := []filesys.Entry{
		{Name: "f", Kind: filesys.EntryFile, PermText: "-rw-r--r--", PermOctal: "0644"},
	}
	pane := testStatusPane(entries, 0)

	pane.model.cfg.Columns.PermissionFormat = "symbolic"
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldPerms, ""); got != "-rw-r--r--" {
		t.Fatalf("symbolic perms = %q", got)
	}

	pane.model.cfg.Columns.PermissionFormat = "octal"
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldPerms, ""); got != "0644" {
		t.Fatalf("octal perms = %q", got)
	}
}

func TestStatusFieldOwner(t *testing.T) {
	entries := []filesys.Entry{
		{Name: "f", Kind: filesys.EntryFile, OwnerText: "ramunas:staff"},
		{Name: "g", Kind: filesys.EntryFile, OwnerText: ""},
	}
	pane := testStatusPane(entries, 0)
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldOwner, ""); got != "ramunas:staff" {
		t.Fatalf("owner = %q", got)
	}
	pane.table.Selected = 1
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldOwner, ""); got != "" {
		t.Fatalf("missing owner should render empty, got %q", got)
	}
}

// TestStatusMarkedSummaryInactiveWithoutMarks pins the mode switch: the summary
// is only offered while rows are marked, and a pane with none reports inactive
// so R2's layout renders the per-entry left cluster instead.
func TestStatusMarkedSummaryInactiveWithoutMarks(t *testing.T) {
	entries := []filesys.Entry{{Name: "a", Kind: filesys.EntryFile, SizeBytes: 100}}
	pane := testStatusPane(entries, 0)
	if count, size, ok := filePaneStatusMarkedSummary(pane); ok || count != "" || size != "" {
		t.Fatalf("unmarked summary = (%q, %q, %t), want inactive", count, size, ok)
	}
	if count, size, ok := filePaneStatusMarkedSummary(nil); ok || count != "" || size != "" {
		t.Fatalf("nil pane summary = (%q, %q, %t), want inactive", count, size, ok)
	}
}

func TestStatusMarkedSummarySumsMarkedSizes(t *testing.T) {
	entries := []filesys.Entry{
		{Name: "a", Kind: filesys.EntryFile, Path: "/a", SizeBytes: 1048576},
		{Name: "b", Kind: filesys.EntryFile, Path: "/b", SizeBytes: 2097152},
		{Name: "c", Kind: filesys.EntryFile, Path: "/c", SizeBytes: 4194304},
	}
	pane := testStatusPane(entries, 0)
	pane.markedRows = map[int]struct{}{0: {}, 1: {}}
	count, size, ok := filePaneStatusMarkedSummary(pane)
	if !ok {
		t.Fatal("summary inactive with two marked rows")
	}
	if count != "2 items selected" {
		t.Fatalf("count = %q, want %q", count, "2 items selected")
	}
	if size != "3.00 MB" {
		t.Fatalf("size = %q, want %q", size, "3.00 MB")
	}
}

// One marked row already switches modes, and it is worded in the singular —
// both are Revision 2 decisions taken without a user ruling, flagged there.
func TestStatusMarkedSummarySingular(t *testing.T) {
	entries := []filesys.Entry{{Name: "a", Kind: filesys.EntryFile, SizeBytes: 1048576}}
	pane := testStatusPane(entries, 0)
	pane.markedRows = map[int]struct{}{0: {}}
	count, size, ok := filePaneStatusMarkedSummary(pane)
	if !ok || count != "1 item selected" || size != "1.00 MB" {
		t.Fatalf("summary = (%q, %q, %t), want (%q, %q, true)", count, size, ok, "1 item selected", "1.00 MB")
	}
}

// A marked directory reports no size, so it must count without contributing —
// the old selection field skipped non-positive sizes and the summary keeps that.
func TestStatusMarkedSummarySkipsSizelessEntries(t *testing.T) {
	entries := []filesys.Entry{
		{Name: "docs", Kind: filesys.EntryDir, SizeBytes: -1},
		{Name: "a", Kind: filesys.EntryFile, SizeBytes: 2097152},
	}
	pane := testStatusPane(entries, 0)
	pane.markedRows = map[int]struct{}{0: {}, 1: {}}
	count, size, ok := filePaneStatusMarkedSummary(pane)
	if !ok || count != "2 items selected" || size != "2.00 MB" {
		t.Fatalf("summary = (%q, %q, %t), want (%q, %q, true)", count, size, ok, "2 items selected", "2.00 MB")
	}
}

// TestStatusNameValueUsesDisplayName pins where the name comes from: the same
// DisplayName the pane grid renders (filePaneEntryNameCell), with a fallback to
// Name for entries that never got one.
func TestStatusNameValueUsesDisplayName(t *testing.T) {
	entries := []filesys.Entry{
		{Name: "raw-name.txt", DisplayName: "shown.txt", Kind: filesys.EntryFile},
		{Name: "bare.txt", Kind: filesys.EntryFile},
	}
	pane := testStatusPane(entries, 0)
	if got := filePaneStatusNameValue(pane, 80); got != "shown.txt" {
		t.Fatalf("name = %q, want the display name %q", got, "shown.txt")
	}
	pane.table.Selected = 1
	if got := filePaneStatusNameValue(pane, 80); got != "bare.txt" {
		t.Fatalf("name = %q, want the Name fallback %q", got, "bare.txt")
	}
	if got := filePaneStatusNameValue(nil, 80); got != "" {
		t.Fatalf("nil pane name = %q, want empty", got)
	}
}

// TestStatusNameValueCompactsLongNames is the gpstrack-dashb….go trim from the
// Revision 2 spec: a name over capacity goes through the pane grid's own
// compactName, so it honours the name_compact config (marker, keep-start-chars)
// and keeps the extension tail.
func TestStatusNameValueCompactsLongNames(t *testing.T) {
	const name = "gpstrack-dashboard-frontend.go"
	entries := []filesys.Entry{{Name: name, DisplayName: name, Kind: filesys.EntryFile}}
	pane := testStatusPane(entries, 0)
	pane.model.cfg.NameCompact.Marker = "…"
	pane.model.cfg.NameCompact.KeepStartChars = 6

	const capacity = 18
	got := filePaneStatusNameValue(pane, capacity)
	if got == name {
		t.Fatalf("a %d-rune name survived a %d-rune capacity uncompacted", utf8.RuneCountInString(name), capacity)
	}
	if utf8.RuneCountInString(got) > capacity {
		t.Fatalf("compacted name %q is %d runes, over the %d capacity", got, utf8.RuneCountInString(got), capacity)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("compacted name %q does not carry the configured marker", got)
	}
	if !strings.HasSuffix(got, ".go") {
		t.Fatalf("compacted name %q lost its extension tail", got)
	}
	// KeepStartChars is 6, so at least that much of the head survives.
	if !strings.HasPrefix(got, "gpstra") {
		t.Fatalf("compacted name %q lost its head", got)
	}
}

func TestStatusNameValuePassesShortNamesThrough(t *testing.T) {
	entries := []filesys.Entry{{Name: "main.go", DisplayName: "main.go", Kind: filesys.EntryFile}}
	pane := testStatusPane(entries, 0)
	// Exactly at capacity: no compaction.
	if got := filePaneStatusNameValue(pane, 7); got != "main.go" {
		t.Fatalf("name at exact capacity = %q, want it untouched", got)
	}
	// Capacity 0 means no room at all; compactName renders nothing, and that is
	// the sane answer — R2's layout drops the whole line before this happens.
	if got := filePaneStatusNameValue(pane, 0); got != "" {
		t.Fatalf("name at zero capacity = %q, want empty", got)
	}
}

// TestStatusFieldDropOrderCoversEveryFieldConstant mirrors
// TestStatusBarFieldOrderCoversEveryFieldConstant in fm/config_test.go: both
// guard a hand-maintained list against the enum it is meant to cover.
//
// As with the fm guard, this cannot catch a field the test was never told
// about — whoever adds an eighth constant has to extend allFilePaneStatusFields
// as well, because Go offers no way to enumerate an enum. What it does catch is
// the common half-update: a constant added to the enum and to
// allFilePaneStatusFields but not to filePaneStatusDropOrder. That mistake
// currently produces no build error and no test failure anywhere.
//
// The consequence it guards is specific. The degradation loop walks
// filePaneStatusDropOrder looking for the first listed field that is still
// active, so an unlisted field is never a drop candidate:
//   - active = [size, X] with X unlisted finds no listed candidate among the
//     active fields until the walk reaches size, so it drops *size* — the one
//     field this bar exists to protect — and leaves X standing.
//   - active = [X, Y] with both unlisted matches nothing at all, so the drop
//     step reports failure and the caller renders the unshrunk, overflowing
//     line. Degradation silently switches off.
func TestStatusFieldDropOrderCoversEveryFieldConstant(t *testing.T) {
	// Two constants sharing a value would satisfy every check below while still
	// making one of them unreachable.
	known := make(map[filePaneStatusField]bool, len(allFilePaneStatusFields))
	for _, field := range allFilePaneStatusFields {
		if known[field] {
			t.Fatalf("two filePaneStatusField constants share the value %d", field)
		}
		known[field] = true
	}

	counts := make(map[filePaneStatusField]int, len(filePaneStatusDropOrder))
	for _, field := range filePaneStatusDropOrder {
		counts[field]++
	}

	// Direction 1: every enum value appears in the drop order.
	for _, field := range allFilePaneStatusFields {
		if counts[field] == 0 {
			t.Fatalf("filePaneStatusField %d is missing from filePaneStatusDropOrder; it would never be a drop candidate, so it would outrank size and could switch degradation off entirely", field)
		}
	}

	// Direction 2: the drop order holds nothing else, and nothing twice. A
	// duplicate would silently consume a drop step that changes nothing.
	for _, field := range filePaneStatusDropOrder {
		if !known[field] {
			t.Fatalf("filePaneStatusDropOrder contains %d, which is not a known filePaneStatusField constant", field)
		}
		if counts[field] > 1 {
			t.Fatalf("filePaneStatusDropOrder lists field %d %d times", field, counts[field])
		}
	}

	if len(filePaneStatusDropOrder) != len(allFilePaneStatusFields) {
		t.Fatalf("filePaneStatusDropOrder has %d entries but there are %d field constants: %v vs %v", len(filePaneStatusDropOrder), len(allFilePaneStatusFields), filePaneStatusDropOrder, allFilePaneStatusFields)
	}
}

// TestStatusFieldFromConfigKey covers the third parallel list in the chain
// (fm config strings -> ui enum). A missing case here is fully silent:
// fm.NormalizeStatusBarFields keeps the key, filePaneStatusFields drops it, and
// the user's configured field simply never appears in the bar.
func TestStatusFieldFromConfigKey(t *testing.T) {
	tests := []struct {
		key  string
		want filePaneStatusField
	}{
		{fm.StatusBarFieldSize, filePaneStatusFieldSize},
		{fm.StatusBarFieldDate, filePaneStatusFieldDate},
		{fm.StatusBarFieldPerms, filePaneStatusFieldPerms},
		{fm.StatusBarFieldOwner, filePaneStatusFieldOwner},
		{fm.StatusBarFieldFree, filePaneStatusFieldFree},
	}

	for _, tc := range tests {
		got, ok := filePaneStatusFieldFromConfigKey(tc.key)
		if !ok {
			t.Fatalf("filePaneStatusFieldFromConfigKey(%q) reported the key unknown; fm accepts it, so a user configuring that field would get nothing in the bar", tc.key)
		}
		if got != tc.want {
			t.Fatalf("filePaneStatusFieldFromConfigKey(%q) = %d, want %d", tc.key, got, tc.want)
		}
	}

	// Every enum value must be reachable from some config key, or the field can
	// never be switched on at all.
	reachable := make(map[filePaneStatusField]bool, len(tests))
	for _, tc := range tests {
		reachable[tc.want] = true
	}
	for _, field := range allFilePaneStatusFields {
		if !reachable[field] {
			t.Fatalf("filePaneStatusField %d has no config key mapping to it, so no configuration can turn it on", field)
		}
	}

	// "selection" (Revision 1's marked-selection field) and "items" (the item
	// count, retired by Revision 2.1) are the keys the migrations retire: fm
	// drops them on load, and the ui must treat a stray copy as unknown too.
	for _, key := range []string{"", "bogus", "Size", " size", "sizes", "selection", "items"} {
		if _, ok := filePaneStatusFieldFromConfigKey(key); ok {
			t.Fatalf("filePaneStatusFieldFromConfigKey(%q) accepted an unknown key", key)
		}
	}
}

func TestStatusFieldsFromDefaultConfigKeys(t *testing.T) {
	cfg := fm.DefaultConfig()
	want := []filePaneStatusField{
		filePaneStatusFieldSize,
		filePaneStatusFieldDate,
		filePaneStatusFieldFree,
	}
	if got := filePaneStatusFields(cfg.StatusBar.Fields); !slices.Equal(got, want) {
		t.Fatalf("filePaneStatusFields(%v) = %v, want %v", cfg.StatusBar.Fields, got, want)
	}
}

// TestStatusFieldsSurvivesEveryNormalisedKey drives the whole chain: every fm
// constant through fm.NormalizeStatusBarFields and out the other side as a
// field. This is the end-to-end version of the silent-drop failure — if the two
// packages ever disagree about a key, the count comes up short here.
func TestStatusFieldsSurvivesEveryNormalisedKey(t *testing.T) {
	keys := fm.NormalizeStatusBarFields([]string{
		fm.StatusBarFieldSize,
		fm.StatusBarFieldDate,
		fm.StatusBarFieldPerms,
		fm.StatusBarFieldOwner,
		fm.StatusBarFieldFree,
	})
	got := filePaneStatusFields(keys)
	if len(got) != len(keys) {
		t.Fatalf("filePaneStatusFields(%v) returned %d fields for %d normalised keys; ui dropped a key fm accepted", keys, len(got), len(keys))
	}
	if len(got) != len(allFilePaneStatusFields) {
		t.Fatalf("got %d fields, want all %d", len(got), len(allFilePaneStatusFields))
	}
}

func TestStatusFieldsDropsUnrecognisedKeys(t *testing.T) {
	keys := []string{fm.StatusBarFieldSize, "bogus", fm.StatusBarFieldDate, "", fm.StatusBarFieldFree}
	want := []filePaneStatusField{
		filePaneStatusFieldSize,
		filePaneStatusFieldDate,
		filePaneStatusFieldFree,
	}
	if got := filePaneStatusFields(keys); !slices.Equal(got, want) {
		t.Fatalf("filePaneStatusFields(%v) = %v, want %v", keys, got, want)
	}
}

// TestStatusFreeFormat pins Revision 2's single free-space form:
// "<free> free (<pct>%)", where the percentage is free/total rounded to the
// nearest integer. The three-form ladder and the ASCII usage bar are retired.
func TestStatusFreeFormat(t *testing.T) {
	// gib is a variable on purpose: 41.2*float64(uint64(1)<<30) would be a
	// constant expression, and converting a constant with a fractional part to
	// uint64 does not compile.
	gib := float64(uint64(1) << 30)
	tests := []struct {
		name        string
		free, total uint64
		want        string
	}{
		{"typical", uint64(41.2 * gib), 100 << 30, "41.20 GB free (41%)"},
		{"zero free", 0, 100 << 30, "0 B free (0%)"},
		{"all free", 100 << 30, 100 << 30, "100.00 GB free (100%)"},
		// 12.5% must round to 13: nearest integer, halves away from zero.
		{"rounds .5 up", 1 << 30, 8 << 30, "1.00 GB free (13%)"},
		// 31/250 = 12.4% must round down.
		{"rounds below .5 down", 31, 250, "31 B free (12%)"},
	}
	for _, tc := range tests {
		if got := formatFilePaneStatusFree(tc.free, tc.total); got != tc.want {
			t.Fatalf("%s: formatFilePaneStatusFree(%d, %d) = %q, want %q", tc.name, tc.free, tc.total, got, tc.want)
		}
	}
}

// TestStatusFreeEmptyWhenTotalUnknown pins the not-yet-loaded case. The volume
// lookup reports a zero total until it has landed, and the layout path relies
// on this rendering empty so the field is skipped rather than showing
// "0 B free (0%)" — or dividing by zero for the percentage.
func TestStatusFreeEmptyWhenTotalUnknown(t *testing.T) {
	if got := formatFilePaneStatusFree(41<<30, 0); got != "" {
		t.Fatalf("zero total = %q, want empty", got)
	}
	if got := formatFilePaneStatusFree(0, 0); got != "" {
		t.Fatalf("zero free and total = %q, want empty", got)
	}
}

// TestStatusFreeClampsFreeAboveTotal covers a free value larger than the total,
// which SFTP and network volumes do report. Without the clamp the label would
// claim more free space than the volume has, and the percentage would read over
// 100.
func TestStatusFreeClampsFreeAboveTotal(t *testing.T) {
	const total = uint64(100) << 30
	const free = uint64(150) << 30
	if got, want := formatFilePaneStatusFree(free, total), "100.00 GB free (100%)"; got != want {
		t.Fatalf("free above total = %q, want the clamped %q", got, want)
	}
}
