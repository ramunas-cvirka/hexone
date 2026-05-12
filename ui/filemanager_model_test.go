// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/widget/table"
	"testing"
	"time"

	"gioui.org/font"
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
