// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"testing"
	"time"
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
