// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/widget/table"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/font"
	"gioui.org/unit"
)

func TestFilePaneFormatDateUsesMeasuredWidth(t *testing.T) {
	cfg := fm.DefaultConfig()
	ts := time.Date(2026, time.March, 12, 15, 4, 0, 0, time.UTC)
	entry := filesys.Entry{
		ModTime:  ts,
		DateText: ts.Format(cfg.DateFormats[0]),
	}
	longText := ts.Format(cfg.DateFormats[0])
	shortText := ts.Format(cfg.DateFormats[1])

	model := &filePaneModel{cfg: cfg}
	model.setTextMeasurer(func(text string) int {
		switch text {
		case longText:
			return 140
		case shortText:
			return 92
		default:
			return 0
		}
	})

	if got := model.formatDate(entry, 100); got != shortText {
		t.Fatalf("formatDate should choose the shorter exact-fit format, got %q want %q", got, shortText)
	}
}

func TestFilePanePreferredDateFormatFitsConfiguredColumn(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.DateFormats = []string{"2006-01-02 15:04:05", "01-02"}
	ts := time.Date(2026, time.July, 11, 16, 47, 9, 0, time.UTC)
	entry := filesys.Entry{ModTime: ts}
	model := &filePaneModel{cfg: cfg}
	pane := newFilePaneState(".", cfg)
	dateColumn := pane.table.Columns[len(pane.table.Columns)-1]
	contentWidth := int(dateColumn.Width) - 2*fm.ColumnPadDp()
	if got, want := model.formatDate(entry, contentWidth), ts.Format(cfg.DateFormats[0]); got != want {
		t.Fatalf("preferred date format=%q want %q at configured width %d", got, want, contentWidth)
	}
}

func TestFilePaneFullModeAnchorsMetadataAtRightWithExplicitGaps(t *testing.T) {
	cfg := fm.DefaultConfig()
	pane := newFilePaneState(".", cfg)
	if len(pane.table.Columns) < 3 {
		t.Fatalf("full-mode columns=%d want at least 3", len(pane.table.Columns))
	}
	if !pane.table.Columns[0].Flex {
		t.Fatal("filename column must absorb spare width so metadata stays anchored at the right")
	}
	last := pane.table.Columns[len(pane.table.Columns)-1]
	if last.Flex {
		t.Fatal("trailing date column must remain fixed at the right edge")
	}
	for i, col := range pane.table.Columns {
		if got, want := col.PadX, unit.Dp(fm.ColumnPadDp()); got != want {
			t.Fatalf("column %d horizontal padding=%v want %v", i, got, want)
		}
		wantGap := unit.Dp(0)
		if i > 0 {
			wantGap = scaleFilePaneDp(cfg, fm.FullColumnGapDp())
		}
		if col.GapBefore != wantGap {
			t.Fatalf("column %d leading gap=%v want %v", i, col.GapBefore, wantGap)
		}
	}
}

func TestFilePaneFullOrEmptyUsesMeasuredWidth(t *testing.T) {
	model := &filePaneModel{cfg: fm.DefaultConfig()}
	model.setTextMeasurer(func(text string) int {
		if text == "Mar 12 2026 15:04" {
			return 120
		}
		return 0
	})

	if got := model.fullOrEmpty("Mar 12 2026 15:04", 110); got != "" {
		t.Fatalf("fullOrEmpty should reject text that measured wider than the cell, got %q", got)
	}
}

func TestFilePaneModelDisplaysSymlinkTarget(t *testing.T) {
	model := &filePaneModel{
		entries: []filesys.Entry{{
			Name:        "etc",
			DisplayName: "etc",
			IsSymlink:   true,
			LinkTarget:  "private/etc",
			Kind:        filesys.EntryDir,
		}},
		cfg: fm.DefaultConfig(),
	}

	text, _ := model.Cell(0, 0)
	if text != "etc" {
		t.Fatalf("cell text = %q, want base symlink name", text)
	}
	_, st := model.Cell(0, 0)
	if st.Suffix != " -> private/etc" {
		t.Fatalf("cell suffix = %q, want symlink target suffix", st.Suffix)
	}
	if st.SuffixColor.A == 0 || !st.SuffixPreserveColor {
		t.Fatalf("suffix style = %#v, want preserved dim color", st)
	}
	if !st.SuffixWeightSet || st.SuffixWeight != font.Normal {
		t.Fatalf("suffix weight = %v set=%v, want regular", st.SuffixWeight, st.SuffixWeightSet)
	}
	icon, ok := model.LeadingIcon(0, 0)
	if !ok || icon.Kind != table.IconLink {
		t.Fatalf("leading icon = %#v ok=%v, want link icon", icon, ok)
	}
}

func TestFilePaneModelUsesConfiguredColumnWeights(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Columns.ShowPermissions = true
	cfg.General.FileWeight = fm.FontWeightRegular
	cfg.General.DirWeight = fm.FontWeightRegular
	cfg.General.PermissionsWeight = fm.FontWeightBold
	cfg.General.SizeWeight = fm.FontWeightRegular
	cfg.General.DateWeight = fm.FontWeightRegular
	model := &filePaneModel{
		entries: []filesys.Entry{
			{
				Name:        "file.txt",
				DisplayName: "file.txt",
				Kind:        filesys.EntryFile,
				PermText:    "-rw-r--r--",
				SizeText:    "12 B",
				DateText:    "Jan 02",
			},
			{
				Name:        "docs",
				DisplayName: "docs",
				Kind:        filesys.EntryDir,
				PermText:    "drwxr-xr-x",
				SizeText:    "-",
				DateText:    "Jan 02",
			},
		},
		cfg: cfg,
	}

	if _, st := model.Cell(0, 0); st.Weight != font.Normal {
		t.Fatalf("file name weight=%v want regular", st.Weight)
	}
	if _, st := model.Cell(1, 0); st.Weight != font.Normal {
		t.Fatalf("dir name weight=%v want regular", st.Weight)
	}
	if _, st := model.Cell(0, 1); st.Weight != font.Bold {
		t.Fatalf("permissions weight=%v want bold", st.Weight)
	}
	if _, st := model.Cell(0, 2); st.Weight != font.Normal {
		t.Fatalf("size weight=%v want regular", st.Weight)
	}
	if _, st := model.Cell(0, 3); st.Weight != font.Normal {
		t.Fatalf("date weight=%v want regular", st.Weight)
	}
}

func TestFilePaneModelBrokenSymlinkSuffixIsRed(t *testing.T) {
	model := &filePaneModel{
		entries: []filesys.Entry{{
			Name:        "missing-link",
			DisplayName: "missing-link",
			IsSymlink:   true,
			LinkTarget:  "missing",
			Kind:        filesys.EntryBroken,
		}},
		cfg: fm.DefaultConfig(),
	}

	_, st := model.Cell(0, 0)
	if st.Suffix == "" {
		t.Fatal("broken symlink should expose a suffix")
	}
	if st.SuffixColor.R <= st.SuffixColor.G || st.SuffixColor.R <= st.SuffixColor.B {
		t.Fatalf("broken symlink suffix color = %#v, want red-tinted color", st.SuffixColor)
	}
}

func TestFilePaneAppliesConfiguredSortPerDirectory(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	other := filepath.Clean(t.TempDir())
	cfg := fm.DefaultConfig()
	cfg.Sort.PerDir = map[string]string{root: "s-"}
	pane := newFilePaneState(root, cfg)

	pane.applyListing(filesys.Listing{
		Dir: root,
		Entries: []filesys.Entry{
			{Name: "small.txt", Path: filepath.Join(root, "small.txt"), SizeBytes: 1},
			{Name: "large.txt", Path: filepath.Join(root, "large.txt"), SizeBytes: 10},
		},
	}, "", "", 0)

	if pane.sortKey != fileSortSize || !pane.sortDesc {
		t.Fatalf("sort for configured dir = %v desc=%v, want size desc", pane.sortKey, pane.sortDesc)
	}

	pane.applyListing(filesys.Listing{
		Dir: other,
		Entries: []filesys.Entry{
			{Name: "b.txt", Path: filepath.Join(other, "b.txt")},
			{Name: "a.txt", Path: filepath.Join(other, "a.txt")},
		},
	}, "", "", 0)

	if pane.sortKey != fileSortName || pane.sortDesc {
		t.Fatalf("sort for unconfigured dir = %v desc=%v, want name asc", pane.sortKey, pane.sortDesc)
	}
}

func TestRememberPaneSortForDirectorySavesOnlyOverrides(t *testing.T) {
	dir := filepath.Clean(t.TempDir())
	configPath := filepath.Join(t.TempDir(), "hexone.yaml")
	original := fm.DefaultConfig()
	original.FavoriteLocations = []string{dir}
	if err := fm.SaveConfig(configPath, original); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	cfg := fm.DefaultConfig()
	pane := newFilePaneState(dir, cfg)
	pane.dir = dir
	ui := &UI{
		fmCfg:      cfg,
		configPath: configPath,
		filePanes:  []*filePaneState{pane},
	}

	pane.sortKey = fileSortDate
	pane.sortDesc = true
	ui.rememberPaneSortForDirectory(0)

	saved := fm.LoadConfig(configPath)
	if got, want := saved.Sort.PerDir[dir], "d-"; got != want {
		t.Fatalf("saved per-dir sort=%q want %q", got, want)
	}
	if got, want := len(saved.FavoriteLocations), 1; got != want {
		t.Fatalf("favorites count=%d want %d", got, want)
	}

	ui.fmCfg = saved
	pane.sortKey = fileSortName
	pane.sortDesc = false
	ui.rememberPaneSortForDirectory(0)

	saved = fm.LoadConfig(configPath)
	if len(saved.Sort.PerDir) != 0 {
		t.Fatalf("default sort should remove per-dir override, got %#v", saved.Sort.PerDir)
	}
}
