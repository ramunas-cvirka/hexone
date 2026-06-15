// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"os"
	"path/filepath"
	"testing"

	"hexone/fm"
)

func TestSaveFMConfigRefusesToOverwriteConfigWithLoadIssue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	original := "viewer:\n  command_by_target: [broken\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	cfg, err := fm.LoadConfigEnsuringFile(path)
	if err == nil {
		t.Fatal("expected invalid config load to fail")
	}

	ui := NewUI(cfg)
	ui.configPath = path
	if err := ui.saveFMConfig(); err == nil {
		t.Fatal("expected saveFMConfig to refuse overwriting a config with load issues")
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("os.ReadFile: %v", readErr)
	}
	if string(data) != original {
		t.Fatalf("config file should remain unchanged, got:\n%s", string(data))
	}
}

func TestEnsureFMConfigLoadedReloadsFromDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	cfg := fm.DefaultConfig()
	cfg.FavoriteLocations = []string{"/tmp/demo"}
	cfg.SSH.Setups = []fm.SSHSetup{{Name: "demo", Host: "example.com", Port: 22, User: "root"}}
	if err := fm.SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	ui := NewUI(fm.DefaultConfig())
	ui.fmCfg = nil
	ui.configPath = path
	if err := ui.ensureFMConfigLoaded(); err != nil {
		t.Fatalf("ensureFMConfigLoaded: %v", err)
	}
	if got, want := len(ui.fmCfg.FavoriteLocations), 1; got != want {
		t.Fatalf("favorites count=%d want %d", got, want)
	}
	if got, want := len(ui.fmCfg.SSH.Setups), 1; got != want {
		t.Fatalf("ssh setups count=%d want %d", got, want)
	}
}

func TestSaveFMConfigRefusesSuspiciousDefaultOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	original := fm.DefaultConfig()
	original.FavoriteLocations = []string{"/tmp/demo"}
	original.SSH.Setups = []fm.SSHSetup{{Name: "demo", Host: "example.com", Port: 22, User: "root"}}
	original.Viewer.Command = "tail -f {path}"
	original.Viewer.CommandHistory = []string{"tail -f {path}"}
	if err := fm.SaveConfig(path, original); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	ui := NewUI(fm.DefaultConfig())
	ui.configPath = path
	if err := ui.saveFMConfig(); err == nil {
		t.Fatal("expected saveFMConfig to refuse overwriting customized config with defaults")
	}

	saved := fm.LoadConfig(path)
	if got, want := len(saved.FavoriteLocations), 1; got != want {
		t.Fatalf("favorites count after refused save=%d want %d", got, want)
	}
	if got, want := len(saved.SSH.Setups), 1; got != want {
		t.Fatalf("ssh setups count after refused save=%d want %d", got, want)
	}
}

func TestSaveFMConfigAllowDefaultResetOverwritesCustomizedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	original := fm.DefaultConfig()
	original.FavoriteLocations = []string{"/tmp/demo"}
	original.SSH.Setups = []fm.SSHSetup{{Name: "demo", Host: "example.com", Port: 22, User: "root"}}
	if err := fm.SaveConfig(path, original); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	ui := NewUI(fm.DefaultConfig())
	ui.configPath = path
	if err := ui.saveFMConfigAllowDefaultReset("test"); err != nil {
		t.Fatalf("saveFMConfigAllowDefaultReset: %v", err)
	}

	saved := fm.LoadConfig(path)
	if got := len(saved.FavoriteLocations); got != 0 {
		t.Fatalf("favorites count=%d want 0", got)
	}
	if got := len(saved.SSH.Setups); got != 0 {
		t.Fatalf("ssh setups count=%d want 0", got)
	}
}

func TestViewerCommandRuntimeSavePreservesUnrelatedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	original := fm.DefaultConfig()
	original.FavoriteLocations = []string{"/tmp/demo"}
	original.SSH.Setups = []fm.SSHSetup{{Name: "demo", Host: "example.com", Port: 22, User: "root"}}
	if err := fm.SaveConfig(path, original); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	ui := NewUI(fm.DefaultConfig())
	ui.configPath = path
	ui.fmCfg.Viewer.Command = "type {path}"
	ui.fmCfg.Viewer.CommandHistory = []string{"type {path}"}
	ui.fmCfg.Viewer.CommandByTarget = map[string]string{"local:c:/tmp/demo.txt": "type {path}"}
	if err := ui.saveFMConfigWithOptions("viewer-command", false); err != nil {
		t.Fatalf("saveFMConfigWithOptions(viewer-command): %v", err)
	}

	saved := fm.LoadConfig(path)
	if got, want := len(saved.FavoriteLocations), 1; got != want {
		t.Fatalf("favorites count=%d want %d", got, want)
	}
	if got, want := len(saved.SSH.Setups), 1; got != want {
		t.Fatalf("ssh setups count=%d want %d", got, want)
	}
	if got, want := saved.Viewer.Command, "type {path}"; got != want {
		t.Fatalf("viewer command=%q want %q", got, want)
	}
	if got, want := len(saved.Viewer.CommandHistory), 1; got != want {
		t.Fatalf("command history count=%d want %d", got, want)
	}
	if got, want := saved.Viewer.CommandByTarget["local:c:/tmp/demo.txt"], "type {path}"; got != want {
		t.Fatalf("target command=%q want %q", got, want)
	}
}

func TestCustomCommandRuntimeSavePreservesUnrelatedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	original := fm.DefaultConfig()
	original.FavoriteLocations = []string{"/tmp/demo"}
	original.SSH.Setups = []fm.SSHSetup{{Name: "demo", Host: "example.com", Port: 22, User: "root"}}
	if err := fm.SaveConfig(path, original); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	ui := NewUI(fm.DefaultConfig())
	ui.configPath = path
	ui.fmCfg.CustomCommands = []fm.CustomCommand{{Name: "Health", Command: "uptime"}}
	if err := ui.saveFMConfigWithOptions("custom-commands", false); err != nil {
		t.Fatalf("saveFMConfigWithOptions(custom-commands): %v", err)
	}

	saved := fm.LoadConfig(path)
	if got, want := len(saved.FavoriteLocations), 1; got != want {
		t.Fatalf("favorites count=%d want %d", got, want)
	}
	if got, want := len(saved.SSH.Setups), 1; got != want {
		t.Fatalf("ssh setups count=%d want %d", got, want)
	}
	if got, want := len(saved.CustomCommands), 1; got != want {
		t.Fatalf("custom command count=%d want %d", got, want)
	}
	if got, want := saved.CustomCommands[0].Name, "Health"; got != want {
		t.Fatalf("custom command name=%q want %q", got, want)
	}
}

func TestTerminalHeightRuntimeSavePreservesUnrelatedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	original := fm.DefaultConfig()
	original.FavoriteLocations = []string{"/tmp/demo"}
	if err := fm.SaveConfig(path, original); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	ui := NewUI(fm.DefaultConfig())
	ui.configPath = path
	ui.fmCfg.Terminal.HeightRows = 32
	if err := ui.saveFMConfigWithOptions("terminal-height", false); err != nil {
		t.Fatalf("saveFMConfigWithOptions(terminal-height): %v", err)
	}

	saved := fm.LoadConfig(path)
	if got, want := len(saved.FavoriteLocations), 1; got != want {
		t.Fatalf("favorites count=%d want %d", got, want)
	}
	if got, want := saved.Terminal.HeightRows, 32; got != want {
		t.Fatalf("terminal height rows=%d want %d", got, want)
	}
}
