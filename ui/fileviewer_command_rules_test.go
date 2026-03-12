package ui

import (
	"testing"

	"hexone/fm"
)

func TestViewerConfiguredModeAndCommandUsesMatchingRule(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Viewer.Mode = "file"
	cfg.Viewer.Command = "cat {path}"
	cfg.Viewer.CommandRules = []fm.ViewerCommandRule{
		{Pattern: `\.log$`, Command: `tail -f {path}`},
		{Pattern: `^error\.log$`, Command: `grep ERROR {path}`},
	}

	ui := &UI{fmCfg: cfg}
	mode, cmd := ui.viewerConfiguredModeAndCommand("/tmp/error.log", nil, cfg.Viewer.Mode, cfg.Viewer.Command)

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
		viewerCommandTargetKey("/tmp/error.log", nil): `less +F {path}`,
	}

	ui := &UI{fmCfg: cfg}
	cmd, matchedRule := ui.viewerDefaultCommand("/tmp/error.log", nil, cfg.Viewer.Command)

	if !matchedRule {
		t.Fatal("viewerDefaultCommand should report a matching regex rule")
	}
	if cmd != `less +F {path}` {
		t.Fatalf("cmd=%q want %q", cmd, `less +F {path}`)
	}
}

func TestViewerConfiguredModeAndCommandFallsBackToGenericCommand(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Viewer.Mode = "command"
	cfg.Viewer.Command = "cat {path}"
	cfg.Viewer.CommandRules = []fm.ViewerCommandRule{
		{Pattern: `\.log$`, Command: `tail -f {path}`},
	}

	ui := &UI{fmCfg: cfg}
	mode, cmd := ui.viewerConfiguredModeAndCommand("/tmp/readme.txt", nil, cfg.Viewer.Mode, cfg.Viewer.Command)

	if mode != "command" {
		t.Fatalf("mode=%q want command", mode)
	}
	if cmd != `cat {path}` {
		t.Fatalf("cmd=%q want %q", cmd, `cat {path}`)
	}
}
