// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build uiverify

package ui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"hexone/fm"
	"hexone/ui/platform"
	"hexone/ui/widget/table"
)

// The window the four captures are rendered at. Two panes at 1200px means 600px
// each, which is wide enough for the default three fields and narrow enough that
// the all-fields configuration has to degrade — both of which are worth looking
// at.
const (
	statusBarVerifyWidth  = 1200
	statusBarVerifyHeight = 620
)

// statusBarVerifyCursorName is the file the cursor is parked on in both panes.
// Deliberately not "..": the parent row renders "<UP>" for size and nothing at
// all for owner, so half the configured fields would be invisible in the
// captures.
const statusBarVerifyCursorName = "report.pdf"

// The volume readings the stubbed lookup returns, one per pane. They differ so
// that "each pane shows its own free space" is visible in the frame and
// assertable afterwards; a real statfs would hand both panes the same number and
// prove nothing.
const (
	statusBarVerifyLeftFree   = uint64(128) << 30
	statusBarVerifyLeftTotal  = uint64(512) << 30
	statusBarVerifyRightFree  = uint64(3) << 30
	statusBarVerifyRightTotal = uint64(250) << 30
)

// TestHeadlessFilePaneStatusBar renders the per-pane status bar through the real
// widget tree and reads back real pixels, which is the only way to catch the
// failures unit tests structurally cannot see: a strip that measures correctly
// but never paints, one that paints in the wrong place, or one that takes its
// height out of the file grid instead of out of the pane.
//
// Four configurations are captured, and each is asserted on rather than merely
// photographed:
//
//	brief-with-bar     brief mode, size/date/free
//	full-with-bar      full mode, size/date/perms/free
//	full-hidden        full mode, hide_in_full — no strip, floating badge instead
//	brief-all-fields   brief mode, all six fields, so the line has to degrade
//
// The volume lookup is stubbed (localVolumeUsageFunc). The pipeline underneath
// it — startVolumeLookup's goroutine, the sequence guard, sendFilePaneVolumeResult,
// drainVolumeResults, applyVolumeResult — all still runs; only the statfs syscall
// wrapper is replaced. What that buys is worth far more than the one line of
// coverage it gives up: the free-space reading is baked into the captured PNG as
// text, so a real statfs would make these frames differ between machines and
// between runs, and "did the reading land?" would depend on the health of the
// test machine's mounts. See the two constants above for the second reason.
func TestHeadlessFilePaneStatusBar(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	root := t.TempDir()
	dirs := [2]string{filepath.Join(root, "left"), filepath.Join(root, "right")}
	for _, dir := range dirs {
		writeStatusBarVerifyDir(t, dir)
	}
	wantFree := [2]uint64{statusBarVerifyLeftFree, statusBarVerifyRightFree}

	oldLookup := localVolumeUsageFunc
	t.Cleanup(func() { localVolumeUsageFunc = oldLookup })
	localVolumeUsageFunc = func(path string) (platform.VolumeUsage, error) {
		if strings.HasPrefix(path, dirs[1]) {
			return platform.VolumeUsage{FreeBytes: statusBarVerifyRightFree, TotalBytes: statusBarVerifyRightTotal}, nil
		}
		return platform.VolumeUsage{FreeBytes: statusBarVerifyLeftFree, TotalBytes: statusBarVerifyLeftTotal}, nil
	}

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	cases := []struct {
		name       string
		mode       table.Mode
		hideInFull bool
		fields     []string
		markRows   bool
		wantBar    bool
	}{
		{
			name:    "brief-with-bar",
			mode:    table.ModeBrief,
			fields:  []string{fm.StatusBarFieldSize, fm.StatusBarFieldDate, fm.StatusBarFieldFree},
			wantBar: true,
		},
		{
			name:    "full-with-bar",
			mode:    table.ModeFull,
			fields:  []string{fm.StatusBarFieldSize, fm.StatusBarFieldDate, fm.StatusBarFieldPerms, fm.StatusBarFieldFree},
			wantBar: true,
		},
		{
			// hide_in_full plus full mode is the one configuration with no strip
			// at all, so it is where the fallback has to show: free space is
			// configured, and with no bar to carry it the floating badge comes
			// back over the inactive pane — the same behaviour as switching the
			// bar off entirely. (Until live user review this configuration showed
			// free space nowhere; see the dated note in the design doc.)
			name:       "full-hidden",
			mode:       table.ModeFull,
			hideInFull: true,
			fields:     []string{fm.StatusBarFieldSize, fm.StatusBarFieldDate, fm.StatusBarFieldFree},
			wantBar:    false,
		},
		{
			// markRows makes this the marked-mode capture: the anchored layout
			// replaces the whole per-entry left cluster with the two-part
			// summary (filePaneStatusMarkedSummary) while rows are marked, and
			// this frame is where that is visible and asserted.
			name: "brief-all-fields",
			mode: table.ModeBrief,
			fields: []string{
				fm.StatusBarFieldSize, fm.StatusBarFieldDate, fm.StatusBarFieldPerms,
				fm.StatusBarFieldOwner, fm.StatusBarFieldFree,
			},
			markRows: true,
			wantBar:  true,
		},
	}

	// Row counts per case, compared across the full-mode pair once every subtest
	// has run: full-with-bar and full-hidden are the same listing in the same mode
	// with and without the strip, so a strip that steals height from the grid
	// instead of from the pane shows up as a difference here.
	fullyVisibleRows := map[string][2]int{}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fm.DefaultConfig()
			cfg.StatusBar.Enabled = true
			cfg.StatusBar.HideInFull = tc.hideInFull
			cfg.StatusBar.Fields = fm.NormalizeStatusBarFields(tc.fields)

			ui := NewUI(cfg)
			if len(ui.filePanes) != len(dirs) {
				t.Fatalf("pane count = %d, want %d", len(ui.filePanes), len(dirs))
			}
			for i, pane := range ui.filePanes {
				if pane == nil {
					t.Fatalf("pane %d is nil", i)
				}
				pane.table.SetMode(tc.mode)
				ui.requestPaneLoadWithSelection(i, dirs[i], filepath.Join(dirs[i], statusBarVerifyCursorName), "", 0)
			}

			win, err := headless.NewWindow(statusBarVerifyWidth, statusBarVerifyHeight)
			if err != nil {
				t.Fatalf("create headless window: %v", err)
			}
			defer win.Release()

			router := new(input.Router)
			frame := func() *image.RGBA {
				var ops op.Ops
				gtx := layout.Context{
					Ops:         &ops,
					Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
					Constraints: layout.Exact(image.Pt(statusBarVerifyWidth, statusBarVerifyHeight)),
					Now:         time.Now(),
					Source:      router.Source(),
				}
				ui.Layout(th, gtx)
				router.Frame(&ops)
				if err := win.Frame(&ops); err != nil {
					t.Fatalf("render frame: %v", err)
				}
				img := image.NewRGBA(image.Rect(0, 0, statusBarVerifyWidth, statusBarVerifyHeight))
				if err := win.Screenshot(img); err != nil {
					t.Fatalf("capture frame: %v", err)
				}
				return img
			}

			var img *image.RGBA
			// Frames are what drives every pump in ui.Layout, so "wait" here means
			// "keep rendering", not "sleep". The sleep only keeps the loop from
			// spinning the GPU while a goroutine is out.
			pump := func(what string, budget time.Duration, ready func() bool) {
				t.Helper()
				for deadline := time.Now().Add(budget); ; {
					img = frame()
					if ready() {
						return
					}
					if time.Now().After(deadline) {
						t.Fatalf("timed out waiting for %s", what)
					}
					time.Sleep(8 * time.Millisecond)
				}
			}

			pump("both panes to list their directory", 20*time.Second, func() bool {
				for i, pane := range ui.filePanes {
					if pane.dir != dirs[i] || pane.model == nil || pane.model.Len() != statusBarVerifyEntryCount {
						return false
					}
					entry := pane.selectedEntry()
					if entry == nil || entry.Name != statusBarVerifyCursorName {
						return false
					}
				}
				return true
			})

			if tc.markRows {
				// Marking after the listing lands, because markRow resolves rows
				// through the model.
				for _, pane := range ui.filePanes {
					for _, name := range []string{"notes.md", "archive.tar.gz"} {
						row := pane.findEntryIndex(name)
						if row < 0 {
							t.Fatalf("mark target %q missing from listing", name)
						}
						pane.markRow(row)
					}
				}
				// The marked-mode line as specced: the exact two parts the left
				// cluster renders (joined by the column separator in the frame).
				for i, pane := range ui.filePanes {
					count, size, ok := filePaneStatusMarkedSummary(pane)
					if !ok || count != "2 items selected" || size != "3.00 MB" {
						t.Errorf("pane %d marked summary = %q/%q (ok=%v), want \"2 items selected\"/\"3.00 MB\"", i, count, size, ok)
					}
				}
			}

			// Exactly one presentation of free space is on at a time: the pane's
			// own bar while it is visible, the floating badge over the inactive
			// panes otherwise. filePaneStatusBarShowsFreeSpace is that switch, and
			// both panes are in the same view mode in every case here, so it has
			// to agree with wantBar.
			showsFree := ui.filePaneStatusBarShowsFreeSpace()
			if showsFree != tc.wantBar {
				t.Fatalf("filePaneStatusBarShowsFreeSpace() = %v, want %v; free space would be shown twice, or nowhere", showsFree, tc.wantBar)
			}

			// The free-space reading renders empty until a lookup lands, and the
			// lookup is a real goroutine: a fixed number of frames is not enough,
			// so wait on the documented sentinel instead. Which panes are asked
			// depends on which presentation is on — a pane nothing asks about never
			// marks itself as wanting a reading, and the pump correctly never
			// starts a lookup for it.
			activeIdx := ui.activeFilePane
			if showsFree {
				// Split from the wait below so that a bar which never lays out at
				// all fails here, in seconds, saying so — rather than spending the
				// full landing budget waiting for a lookup nothing ever asked for.
				pump("the status bar to ask for a volume reading", 5*time.Second, func() bool {
					for _, pane := range ui.filePanes {
						if pane.volumeBadge.wantedAt.IsZero() {
							return false
						}
					}
					return true
				})
				pump("both panes' volume readings to land", 15*time.Second, func() bool {
					for _, pane := range ui.filePanes {
						if pane.volumeBadge.totalBytes == 0 {
							return false
						}
					}
					return true
				})
			} else {
				// The fallback badge is drawn on the inactive panes but sourced
				// from the active one (filePaneVolumeBadgeSourcePane), so exactly
				// one pane's reading is asked for and waited on here.
				active := ui.filePanes[activeIdx]
				pump("the fallback volume badge to ask for a reading", 5*time.Second, func() bool {
					return !active.volumeBadge.wantedAt.IsZero()
				})
				pump("the active pane's volume reading to land", 15*time.Second, func() bool {
					return active.volumeBadge.totalBytes != 0
				})
			}
			for i := 0; i < 4; i++ {
				img = frame()
			}

			if ui.terminal.active() {
				t.Fatal("terminal pane is active; the file panes no longer reach the bottom of the window and every pixel assertion below is measuring the wrong band")
			}
			if showsFree {
				for i, pane := range ui.filePanes {
					if got := pane.volumeBadge.freeBytes; got != wantFree[i] {
						t.Errorf("pane %d free bytes = %d, want %d (each pane must show its own volume's reading)", i, got, wantFree[i])
					}
				}
			} else if got := ui.filePanes[activeIdx].volumeBadge.freeBytes; got != wantFree[activeIdx] {
				t.Errorf("active pane %d free bytes = %d, want %d (the badge mirrors the active pane's volume)", activeIdx, got, wantFree[activeIdx])
			}

			stripH := measureStatusBarVerifyStripHeight(t, ui, th)
			bounds := statusBarVerifyPaneBounds()
			rows := [2]int{}
			for i, pane := range ui.filePanes {
				rows[i] = statusBarVerifyFullyVisibleRows(t, pane)
				switch {
				case tc.wantBar:
					assertStatusBarVerifyStrip(t, img, i, bounds[i], stripH, true)
				case i == activeIdx:
					// No strip, and the active pane never hosts the badge either —
					// it is the badge's source, not its host — so its bottom band
					// is bare pane background.
					assertStatusBarVerifyStrip(t, img, i, bounds[i], stripH, false)
				default:
					assertStatusBarVerifyBadge(t, img, i, activeIdx, bounds[i], stripH)
				}
			}
			fullyVisibleRows[tc.name] = rows

			writeStatusBarVerifyPNG(t, outDir, tc.name, img)
		})
	}

	withBar, okWith := fullyVisibleRows["full-with-bar"]
	hidden, okHidden := fullyVisibleRows["full-hidden"]
	if okWith && okHidden && withBar != hidden {
		t.Errorf("fully visible grid rows with the bar = %v, without it = %v; the strip is taking height out of the file grid", withBar, hidden)
	}
}

// statusBarVerifyEntryCount is how many rows the listing below produces,
// including the ".." parent row. Kept small on purpose: the grid must not reach
// the bottom of the pane, or "is the bottom band the strip or the last file
// row?" stops being answerable from pixels alone.
const statusBarVerifyEntryCount = 15

// The no-jump pair: two names of very different length with byte-identical
// size, mtime and permissions, so moving the cursor between them changes ONLY
// the name column's text — every other pixel of the strip must stay put.
const (
	statusBarVerifyShortName = "z.txt"
	statusBarVerifyLongName  = "z-really-long-filename-for-the-no-jump-check.txt"
)

func writeStatusBarVerifyDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create %s: %v", dir, err)
	}
	for _, name := range []string{"archive", "src"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatalf("create directory %s: %v", name, err)
		}
	}
	files := []struct {
		name string
		size int
	}{
		{"archive.tar.gz", 3 << 20},
		{"config.yaml", 812},
		{"hexone.log", 64 << 10},
		{"main.go", 4211},
		{"notes.md", 1536},
		{"report.pdf", 1 << 20},
		{"screenshot.png", 244 << 10},
		{"todo.txt", 96},
		{"video.mkv", 12 << 20},
		{"weights.bin", 7 << 20},
		{statusBarVerifyShortName, 2048},
		{statusBarVerifyLongName, 2048},
	}
	// A fixed mtime keeps the date field — and therefore the captured PNGs —
	// stable between runs on the same machine.
	stamp := time.Date(2026, time.March, 14, 9, 26, 53, 0, time.Local)
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		if err := os.WriteFile(path, make([]byte, f.size), 0o600); err != nil {
			t.Fatalf("create file %s: %v", f.name, err)
		}
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatalf("stamp file %s: %v", f.name, err)
		}
	}
}

// statusBarVerifyPaneBounds returns each pane's horizontal pixel range, using the
// same split layoutFilePanes hands the flex.
func statusBarVerifyPaneBounds() [2][2]int {
	widths := paneColumnWidths(statusBarVerifyWidth, 2)
	out := [2][2]int{}
	x := 0
	for i, w := range widths {
		out[i] = [2]int{x, x + w}
		x += w
	}
	return out
}

// measureStatusBarVerifyStripHeight measures the strip the way the pane's flex
// does. hide_in_full is lifted for the measurement so the hidden configuration
// still gets the height its strip *would* have, which is the band its bottom rows
// are then asserted to be free of.
func measureStatusBarVerifyStripHeight(t *testing.T, ui *UI, th *material.Theme) int {
	t.Helper()
	hidden := ui.fmCfg.StatusBar.HideInFull
	ui.fmCfg.StatusBar.HideInFull = false
	defer func() { ui.fmCfg.StatusBar.HideInFull = hidden }()

	bounds := statusBarVerifyPaneBounds()
	paneWidth := bounds[0][1] - bounds[0][0]
	var ops op.Ops
	gtx := layout.Context{
		Ops:    &ops,
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		// Not layout.Exact: the strip is a Rigid in the pane's vertical flex, so it
		// is offered a free main axis. Pinning Min.Y would make the label stretch
		// and measure the whole pane instead of one line.
		Constraints: layout.Constraints{Max: image.Pt(paneWidth, statusBarVerifyHeight)},
		Now:         time.Now(),
	}
	// Recorded and dropped: this is a measurement, and its ops belong to no frame.
	m := op.Record(gtx.Ops)
	dims := ui.layoutFilePaneStatusBar(th, gtx, 0, ui.filePanes[0], filePanePaletteFromConfig(ui.fmCfg))
	m.Stop()
	if dims.Size.Y <= 0 {
		t.Fatalf("status bar measured %v; expected a positive height", dims.Size)
	}
	// The strip is exactly one line of its own text style plus 4dp of inset above
	// and below. Pinning that is what keeps "the strip costs the file grid one
	// row's worth of height, once" true: a strip that quietly grows is taking
	// height the body needs, however correctly it paints.
	probe := material.Body2(th, "0")
	probe.Font.Typeface = ui.mainTypeface()
	probe.TextSize = scaleThemeFontSize(th, 11)
	probe.MaxLines = 1
	probe.Truncator = ""
	wantHeight := measureLabelUnconstrained(gtx, probe).Size.Y + 2*gtx.Dp(unit.Dp(4))
	if dims.Size.Y != wantHeight {
		t.Fatalf("status bar measured %dpx tall, want %dpx (one line of Body2 at 11sp plus 4dp of inset above and below)", dims.Size.Y, wantHeight)
	}
	return dims.Size.Y
}

// statusBarVerifyFullyVisibleRows counts the listing rows the table renders at
// their full height. A row the strip has pushed under the pane's bottom edge
// comes back either clipped (a short rectangle) or not at all.
func statusBarVerifyFullyVisibleRows(t *testing.T, pane *filePaneState) int {
	t.Helper()
	n := pane.model.Len()
	tallest := 0
	heights := make([]int, n)
	for row := 0; row < n; row++ {
		rect, ok := pane.table.RowRect(row, n)
		if !ok {
			continue
		}
		heights[row] = rect.Dy()
		if rect.Dy() > tallest {
			tallest = rect.Dy()
		}
	}
	if tallest <= 0 {
		t.Fatal("table reported no visible rows at all")
	}
	full := 0
	for _, h := range heights {
		if h == tallest {
			full++
		}
	}
	if full != n {
		t.Errorf("%d of %d listing rows are fully visible (row heights %v, full row = %dpx); the pane body is clipped", full, n, heights, tallest)
	}
	return full
}

// assertStatusBarVerifyStrip checks the bottom band of one pane against what the
// configuration says should be there.
//
// The strip's fill is palette.PaneBg, the same colour the pane is painted with,
// so "the strip is present" cannot be a fill-colour comparison. What it can be is
// the strip's 1px top border — a solid full-width line in a colour the pane
// background never uses — plus the ink of the line itself.
func assertStatusBarVerifyStrip(t *testing.T, img *image.RGBA, idx int, bounds [2]int, stripH int, wantBar bool) {
	t.Helper()

	// Inset past the pane's own edges: the seam between panes is a 1px vertical
	// line drawn over the whole pane height, and it is not part of the strip.
	x0, x1 := bounds[0]+4, bounds[1]-4
	top := statusBarVerifyHeight - stripH

	bg, uniform := statusBarVerifyRowColor(img, top-2, x0, x1)
	if !uniform {
		t.Fatalf("pane %d: the row 2px above the strip is not a uniform background; the listing reaches the bottom band and these assertions cannot tell strip from file row", idx)
	}

	if !wantBar {
		for y := top - 1; y < statusBarVerifyHeight; y++ {
			got, ok := statusBarVerifyRowColor(img, y, x0, x1)
			if !ok || got != bg {
				t.Errorf("pane %d: row %d of the bottom band is %v, want the uniform pane background %v; a strip is painted in a configuration that hides it", idx, y, got, bg)
				return
			}
		}
		return
	}

	border, solid := statusBarVerifyRowColor(img, top, x0, x1)
	if !solid || border == bg {
		t.Errorf("pane %d: row %d is %v (solid=%v), want a solid border line differing from the pane background %v; the strip's top edge is not where the flex puts it", idx, top, border, solid, bg)
		return
	}

	if ink := statusBarVerifyDiffCount(img, top+1, statusBarVerifyHeight, x0, x1, bg); ink < 50 {
		t.Errorf("pane %d: only %d non-background pixels inside the strip; the bar is drawn but its text is not", idx, ink)
	}

	// The last rows are the strip's bottom inset. Text spilling into them means
	// the strip is shorter than the line it is holding.
	for y := statusBarVerifyHeight - 2; y < statusBarVerifyHeight; y++ {
		if got, ok := statusBarVerifyRowColor(img, y, x0, x1); !ok || got != bg {
			t.Errorf("pane %d: row %d of the strip's bottom inset is %v, want %v; the line overflows the strip", idx, y, got, bg)
			return
		}
	}

	// Revision 2's two anchors, read straight off the pixels. The info line
	// carries no pane-seam separators any more; what pins its geometry instead
	// is that the left cluster's name text begins at the left inset and the
	// free-space text ends at the right inset (filePaneStatusBarInsetX, 8dp =
	// 8px at these metrics).
	//
	// Left anchor: the inset itself stays clean (its first pixels skip the
	// pane-seam line, which belongs to neither strip) and the first glyph's ink
	// starts right after it.
	textTop := top + 2
	if ink := statusBarVerifyDiffCount(img, textTop, statusBarVerifyHeight-2, x0-2, bounds[0]+8, bg); ink != 0 {
		t.Errorf("pane %d: %d ink pixels inside the left inset x=[%d,%d); the left cluster is not anchored at the inset", idx, ink, x0-2, bounds[0]+8)
	}
	if ink := statusBarVerifyDiffCount(img, textTop, statusBarVerifyHeight-2, bounds[0]+8, bounds[0]+28, bg); ink == 0 {
		t.Errorf("pane %d: no ink in x=[%d,%d); the name column does not begin at the left inset", idx, bounds[0]+8, bounds[0]+28)
	}

	// Right anchor: every with-bar configuration here carries a landed
	// free-space reading, so its text must end flush against the right inset —
	// ink just inside it, none within it.
	if ink := statusBarVerifyDiffCount(img, textTop, statusBarVerifyHeight-2, bounds[1]-8, x1+2, bg); ink != 0 {
		t.Errorf("pane %d: %d ink pixels inside the right inset x=[%d,%d); the free region overruns its anchor", idx, ink, bounds[1]-8, x1+2)
	}
	if ink := statusBarVerifyDiffCount(img, textTop, statusBarVerifyHeight-2, bounds[1]-24, bounds[1]-8, bg); ink == 0 {
		t.Errorf("pane %d: no ink in x=[%d,%d); the free-space text does not end at the right inset", idx, bounds[1]-24, bounds[1]-8)
	}
}

// assertStatusBarVerifyBadge checks the bottom band of an inactive pane against
// the floating volume badge — the fallback presentation of free space whenever
// the pane's own strip is not carrying it, which since the hide_in_full fix
// includes a full-mode pane with the bar hidden.
//
// The badge lands in the very band the strip would have occupied: same text
// style, same 4dp insets, therefore the same height. Two things tell them apart
// from pixels alone. Width: the strip's 1px top border spans the whole pane,
// while the badge is only as wide as its label. And position: the badge pins to
// the pane's INNER corner, the one facing the active pane it is reporting for
// (filePaneVolumeBadgeOffset), so which edge its run touches is a property of
// the pane's index relative to the active one.
func assertStatusBarVerifyBadge(t *testing.T, img *image.RGBA, idx, activeIdx int, bounds [2]int, stripH int) {
	t.Helper()

	// Inset past the pane's own edges, exactly as assertStatusBarVerifyStrip
	// does: the pane seam is a full-height 1px line and belongs to neither.
	x0, x1 := bounds[0]+4, bounds[1]-4
	top := statusBarVerifyHeight - stripH

	bg, uniform := statusBarVerifyRowColor(img, top-2, x0, x1)
	if !uniform {
		t.Fatalf("pane %d: the row 2px above the badge is not a uniform background; the listing reaches the bottom band and these assertions cannot tell badge from file row", idx)
	}

	// The badge's top border is the one contiguous run of non-background pixels
	// on the band's first row.
	start := x0
	for start < x1 && img.RGBAAt(start, top) == bg {
		start++
	}
	if start >= x1 {
		t.Errorf("pane %d: the whole of row %d is the pane background %v; the fallback volume badge is not painted in the band the hidden strip left free", idx, top, bg)
		return
	}
	end := start
	for end < x1 && img.RGBAAt(end, top) != bg {
		end++
	}
	for x := end; x < x1; x++ {
		if got := img.RGBAAt(x, top); got != bg {
			t.Errorf("pane %d: row %d at x=%d is %v, want the pane background %v; the band holds more than the one badge", idx, top, x, got, bg)
			return
		}
	}
	if start == x0 && end == x1 {
		t.Errorf("pane %d: row %d is a solid line across the whole pane; that is a status strip, not the floating badge", idx, top)
		return
	}
	// Pinned to the corner facing the active pane: flush right for a pane left
	// of it, flush left for one right of it.
	if idx < activeIdx {
		if end != x1 {
			t.Errorf("pane %d: the badge's border spans x=[%d,%d), which does not reach the pane's inner (right) edge at x=%d", idx, start, end, x1)
		}
	} else if start != x0 {
		t.Errorf("pane %d: the badge's border spans x=[%d,%d), which does not reach the pane's inner (left) edge at x=%d", idx, start, end, x0)
	}
	if ink := statusBarVerifyDiffCount(img, top+1, statusBarVerifyHeight, start, end, bg); ink < 50 {
		t.Errorf("pane %d: only %d non-background pixels inside the badge's x=[%d,%d); it is drawn but its free-space label is not", idx, ink, start, end)
	}
}

// statusBarVerifyRowColor returns the colour of a row across [x0,x1) and whether
// every pixel in that span shares it.
func statusBarVerifyRowColor(img *image.RGBA, y, x0, x1 int) (color.RGBA, bool) {
	first := img.RGBAAt(x0, y)
	for x := x0 + 1; x < x1; x++ {
		if img.RGBAAt(x, y) != first {
			return first, false
		}
	}
	return first, true
}

func statusBarVerifyDiffCount(img *image.RGBA, y0, y1, x0, x1 int, bg color.RGBA) int {
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if img.RGBAAt(x, y) != bg {
				n++
			}
		}
	}
	return n
}

// TestHeadlessFilePaneStatusBarNoJump is the no-jump property read off real
// pixels: with the cursor on a short name and then on a long one — two files
// with byte-identical size, mtime and permissions — every pixel of the strip
// outside the name column's span must be identical. Fixed columns that
// re-measure from the selected entry, a separator that shifts, or a free
// region that floats all fail this byte-compare.
func TestHeadlessFilePaneStatusBarNoJump(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	root := t.TempDir()
	dirs := [2]string{filepath.Join(root, "left"), filepath.Join(root, "right")}
	for _, dir := range dirs {
		writeStatusBarVerifyDir(t, dir)
	}

	oldLookup := localVolumeUsageFunc
	t.Cleanup(func() { localVolumeUsageFunc = oldLookup })
	localVolumeUsageFunc = func(path string) (platform.VolumeUsage, error) {
		if strings.HasPrefix(path, dirs[1]) {
			return platform.VolumeUsage{FreeBytes: statusBarVerifyRightFree, TotalBytes: statusBarVerifyRightTotal}, nil
		}
		return platform.VolumeUsage{FreeBytes: statusBarVerifyLeftFree, TotalBytes: statusBarVerifyLeftTotal}, nil
	}

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	cfg := fm.DefaultConfig()
	cfg.StatusBar.Enabled = true
	cfg.StatusBar.Fields = fm.NormalizeStatusBarFields([]string{fm.StatusBarFieldSize, fm.StatusBarFieldDate, fm.StatusBarFieldFree})

	ui := NewUI(cfg)
	for i, pane := range ui.filePanes {
		pane.table.SetMode(table.ModeBrief)
		ui.requestPaneLoadWithSelection(i, dirs[i], filepath.Join(dirs[i], statusBarVerifyShortName), "", 0)
	}

	win, err := headless.NewWindow(statusBarVerifyWidth, statusBarVerifyHeight)
	if err != nil {
		t.Fatalf("create headless window: %v", err)
	}
	defer win.Release()

	router := new(input.Router)
	frame := func() *image.RGBA {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(statusBarVerifyWidth, statusBarVerifyHeight)),
			Now:         time.Now(),
			Source:      router.Source(),
		}
		ui.Layout(th, gtx)
		router.Frame(&ops)
		if err := win.Frame(&ops); err != nil {
			t.Fatalf("render frame: %v", err)
		}
		img := image.NewRGBA(image.Rect(0, 0, statusBarVerifyWidth, statusBarVerifyHeight))
		if err := win.Screenshot(img); err != nil {
			t.Fatalf("capture frame: %v", err)
		}
		return img
	}
	pump := func(what string, budget time.Duration, ready func() bool) {
		t.Helper()
		for deadline := time.Now().Add(budget); ; {
			frame()
			if ready() {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(8 * time.Millisecond)
		}
	}

	pump("both panes to list their directory", 20*time.Second, func() bool {
		for i, pane := range ui.filePanes {
			if pane.dir != dirs[i] || pane.model == nil || pane.model.Len() != statusBarVerifyEntryCount {
				return false
			}
			entry := pane.selectedEntry()
			if entry == nil || entry.Name != statusBarVerifyShortName {
				return false
			}
		}
		return true
	})
	pump("both panes' volume readings to land", 15*time.Second, func() bool {
		for _, pane := range ui.filePanes {
			if pane.volumeBadge.totalBytes == 0 {
				return false
			}
		}
		return true
	})

	pane := ui.filePanes[0]
	shortRow := pane.findEntryIndex(statusBarVerifyShortName)
	longRow := pane.findEntryIndex(statusBarVerifyLongName)
	if shortRow < 0 || longRow < 0 {
		t.Fatalf("no-jump pair missing from listing: short=%d long=%d", shortRow, longRow)
	}

	capture := func(row int) *image.RGBA {
		pane.table.Selected = row
		var img *image.RGBA
		for i := 0; i < 3; i++ {
			img = frame()
		}
		return img
	}
	imgShort := capture(shortRow)
	imgLong := capture(longRow)
	writeStatusBarVerifyPNG(t, outDir, "nojump-short", imgShort)
	writeStatusBarVerifyPNG(t, outDir, "nojump-long", imgLong)

	// The name column's pixel span, derived the way the shipping row derives
	// it: the cached listing-and-config widths plus the plan at this pane's
	// width. Nothing here reads the selected entry — that is the point.
	bounds := statusBarVerifyPaneBounds()
	paneWidth := bounds[0][1] - bounds[0][0]
	avail := paneWidth - 16 // 8dp inset each side at PxPerDp 1
	state := &pane.statusBar
	if !state.valid || state.freePx <= 0 {
		t.Fatalf("the strip's measurement cache did not populate (valid=%v freePx=%d)", state.valid, state.freePx)
	}
	plan := buildFilePaneStatusBarPlan(state.widths, filePaneStatusFields(cfg.StatusBar.Fields), state.freePx, avail)
	if plan.nameColPx <= 0 {
		t.Fatalf("plan carries no name column: %+v", plan)
	}
	stripH := measureStatusBarVerifyStripHeight(t, ui, th)
	top := statusBarVerifyHeight - stripH
	// One pixel of slack past the column for the last glyph's antialiasing.
	nameX0 := bounds[0][0] + 8
	nameX1 := nameX0 + plan.nameColPx + 1

	// Outside the name column a real jump moves glyph ink by whole pixels —
	// channel deltas in the tens to hundreds. The GPU rasterizer is however
	// allowed a ±2/255 antialiasing wobble on IDENTICAL glyphs: the long name
	// adds new glyphs to the shaper's atlas, and repacking can shift a fringe
	// pixel of an unchanged glyph by one level. Tolerating that keeps the
	// assertion's teeth while making it deterministic.
	const aaTolerance = 2
	samePixel := func(a, b color.RGBA) bool {
		diff := func(x, y uint8) int {
			d := int(x) - int(y)
			if d < 0 {
				d = -d
			}
			return d
		}
		return diff(a.R, b.R) <= aaTolerance && diff(a.G, b.G) <= aaTolerance && diff(a.B, b.B) <= aaTolerance
	}
	changedInside, changedOutside := 0, 0
	firstOutside := image.Point{X: -1}
	for y := top; y < statusBarVerifyHeight; y++ {
		for x := bounds[0][0]; x < bounds[0][1]; x++ {
			if x >= nameX0 && x < nameX1 {
				if imgShort.RGBAAt(x, y) != imgLong.RGBAAt(x, y) {
					changedInside++
				}
				continue
			}
			if !samePixel(imgShort.RGBAAt(x, y), imgLong.RGBAAt(x, y)) {
				changedOutside++
				if firstOutside.X < 0 {
					firstOutside = image.Pt(x, y)
				}
			}
		}
	}
	if changedInside == 0 {
		t.Error("moving the cursor between the short and long name changed nothing in the name column; the capture is not exercising the property")
	}
	if changedOutside != 0 {
		t.Errorf("%d strip pixels outside the name column x=[%d,%d) changed with the cursor (first at %v); the columns are jumping", changedOutside, nameX0, nameX1, firstOutside)
	}
}

func writeStatusBarVerifyPNG(t *testing.T, outDir, name string, img *image.RGBA) {
	t.Helper()
	path := filepath.Join(outDir, "status-bar-"+name+".png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create screenshot: %v", err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatalf("encode screenshot: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close screenshot: %v", err)
	}
	t.Logf("wrote %s", path)
}
