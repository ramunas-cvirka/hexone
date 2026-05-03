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
	original := "viewer:\n  mode: [broken\n"
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
	original.Viewer.Mode = "command"
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

func TestViewerModeRuntimeSaveRebasesOntoExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	original := fm.DefaultConfig()
	original.FavoriteLocations = []string{"/tmp/demo"}
	original.SSH.Setups = []fm.SSHSetup{{Name: "demo", Host: "example.com", Port: 22, User: "root"}}
	original.Viewer.CommandHistory = []string{"tail -f {path}"}
	if err := fm.SaveConfig(path, original); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	ui := NewUI(fm.DefaultConfig())
	ui.configPath = path
	ui.fmCfg.Viewer.Mode = "hex"
	if err := ui.saveFMConfigWithOptions("viewer-mode", false); err != nil {
		t.Fatalf("saveFMConfigWithOptions(viewer-mode): %v", err)
	}

	saved := fm.LoadConfig(path)
	if got, want := saved.Viewer.Mode, "hex"; got != want {
		t.Fatalf("viewer mode=%q want %q", got, want)
	}
	if got, want := len(saved.FavoriteLocations), 1; got != want {
		t.Fatalf("favorites count=%d want %d", got, want)
	}
	if got, want := len(saved.SSH.Setups), 1; got != want {
		t.Fatalf("ssh setups count=%d want %d", got, want)
	}
	if got, want := len(saved.Viewer.CommandHistory), 1; got != want {
		t.Fatalf("command history count=%d want %d", got, want)
	}
}

func TestViewerModeRuntimeSaveAllowsSwitchingCommandBackToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	original := fm.DefaultConfig()
	original.Viewer.Mode = "command"
	if err := fm.SaveConfig(path, original); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	ui := NewUI(fm.DefaultConfig())
	ui.configPath = path
	ui.fmCfg.Viewer.Mode = "file"
	if err := ui.saveFMConfigWithOptions("viewer-mode", false); err != nil {
		t.Fatalf("saveFMConfigWithOptions(viewer-mode): %v", err)
	}

	saved := fm.LoadConfig(path)
	if got, want := saved.Viewer.Mode, "file"; got != want {
		t.Fatalf("viewer mode=%q want %q", got, want)
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
	ui.fmCfg.Viewer.Mode = "command"
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
