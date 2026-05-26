// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"os"
	"path/filepath"
	"testing"

	"hexone/fm"
)

func TestViewerInitialModeAndCommandUsesMatchingRule(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Viewer.Command = "cat {path}"
	cfg.Viewer.CommandRules = []fm.ViewerCommandRule{
		{Pattern: `\.log$`, Command: `tail -f {path}`},
		{Pattern: `^error\.log$`, Command: `grep ERROR {path}`},
	}

	ui := &UI{fmCfg: cfg}
	mode, cmd := ui.viewerInitialModeAndCommand("/tmp/error.log", nil, cfg.Viewer.Command)

	if mode != "command" {
		t.Fatalf("mode=%q want command", mode)
	}
	if cmd != `grep ERROR {path}` {
		t.Fatalf("cmd=%q want %q", cmd, `grep ERROR {path}`)
	}
}

func TestViewerDefaultCommandKeepsTargetOverrideOverRule(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Viewer.Command = "cat {path}"
	cfg.Viewer.CommandRules = []fm.ViewerCommandRule{
		{Pattern: `\.log$`, Command: `tail -f {path}`},
	}
	cfg.Viewer.CommandByTarget = map[string]string{
		viewerCommandTargetKey("/tmp/error.log", nil): `tail -n 50 {path}`,
	}

	ui := &UI{fmCfg: cfg}
	cmd, matchedRule, matchedTarget := ui.viewerDefaultCommand("/tmp/error.log", nil, cfg.Viewer.Command)

	if !matchedRule {
		t.Fatal("viewerDefaultCommand should report a matching regex rule")
	}
	if !matchedTarget {
		t.Fatal("viewerDefaultCommand should report a matching exact target")
	}
	if cmd != `tail -n 50 {path}` {
		t.Fatalf("cmd=%q want %q", cmd, `tail -n 50 {path}`)
	}
}

func TestViewerInitialModeAndCommandUsesTargetOverrideMode(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Viewer.Command = "cat {path}"
	cfg.Viewer.CommandByTarget = map[string]string{
		viewerCommandTargetKey("/tmp/readme.txt", nil): `type {path}`,
	}

	ui := &UI{fmCfg: cfg}
	mode, cmd := ui.viewerInitialModeAndCommand("/tmp/readme.txt", nil, cfg.Viewer.Command)

	if mode != "command" {
		t.Fatalf("mode=%q want command", mode)
	}
	if cmd != `type {path}` {
		t.Fatalf("cmd=%q want %q", cmd, `type {path}`)
	}
}

func TestViewerInitialModeAndCommandFallsBackToFile(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Viewer.Command = "cat {path}"

	ui := &UI{fmCfg: cfg}
	mode, cmd := ui.viewerInitialModeAndCommand("/tmp/readme.txt", nil, cfg.Viewer.Command)

	if mode != "file" {
		t.Fatalf("mode=%q want file", mode)
	}
	if cmd != `cat {path}` {
		t.Fatalf("cmd=%q want %q", cmd, `cat {path}`)
	}
}

func TestViewerInitialModeAndCommandUsesHexForOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	cfg := fm.DefaultConfig()
	cfg.Viewer.MaxReadMB = 0.000001

	ui := &UI{fmCfg: cfg}
	mode, _ := ui.viewerInitialModeAndCommand(path, nil, cfg.Viewer.Command)

	if mode != "hex" {
		t.Fatalf("mode=%q want hex", mode)
	}
}

func TestViewerInitialModeAndCommandTargetOverrideBeatsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.log")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	cfg := fm.DefaultConfig()
	cfg.Viewer.MaxReadMB = 0.000001
	cfg.Viewer.CommandByTarget = map[string]string{
		viewerCommandTargetKey(path, nil): `tail -f {path}`,
	}

	ui := &UI{fmCfg: cfg}
	mode, cmd := ui.viewerInitialModeAndCommand(path, nil, cfg.Viewer.Command)

	if mode != "command" {
		t.Fatalf("mode=%q want command", mode)
	}
	if cmd != `tail -f {path}` {
		t.Fatalf("cmd=%q want %q", cmd, `tail -f {path}`)
	}
}
