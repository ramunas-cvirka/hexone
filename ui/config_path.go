// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"fmt"
	"hexone/appdata"
	"hexone/fm"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const uiConfigBackupSuffix = ".bak"

func resolveUIConfigPath() string {
	return appdata.ConfigPath()
}

func (ui *UI) configSavePath() string {
	if ui == nil || ui.configPath == "" {
		return appdata.ConfigPath()
	}
	return ui.configPath
}

func (ui *UI) configDisplayPath() string {
	path := ui.configSavePath()
	if path == "" {
		return ""
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absPath
}

func (ui *UI) ensureFMConfigLoaded() error {
	if ui == nil || ui.fmCfg != nil {
		return nil
	}
	path := ui.configSavePath()
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("config path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config unexpectedly missing at %s", path)
		}
		return err
	}
	cfg, err := fm.LoadConfigEnsuringFile(path)
	if err != nil {
		return err
	}
	ui.fmCfg = cfg
	return nil
}

func (ui *UI) saveFMConfig() error {
	return ui.saveFMConfigWithOptions("runtime", false)
}

func (ui *UI) saveFMConfigAllowDefaultReset(reason string) error {
	return ui.saveFMConfigWithOptions(reason, true)
}

func (ui *UI) saveFMConfigWithOptions(reason string, allowDefaultReset bool) error {
	if ui == nil {
		return nil
	}
	if err := ui.ensureFMConfigLoaded(); err != nil {
		return err
	}
	path := ui.configSavePath()
	existing, err := ui.loadExistingFMConfig(path)
	if err != nil {
		return err
	}
	if !allowDefaultReset && configLooksLikeCriticalStateReset(existing, ui.fmCfg) {
		return fmt.Errorf("refusing to overwrite existing customized config with runtime defaults; restore from %s and use the Config tab for an explicit full reset", path+uiConfigBackupSuffix)
	}
	if strings.TrimSpace(reason) == "" {
		reason = "runtime"
	}
	log.Printf("save config: reason=%s path=%s favorites=%d ssh=%d viewer_mode=%s command_history=%d command_targets=%d", reason, path, len(ui.fmCfg.FavoriteLocations), len(ui.fmCfg.SSH.Setups), strings.TrimSpace(ui.fmCfg.Viewer.Mode), len(ui.fmCfg.Viewer.CommandHistory), len(ui.fmCfg.Viewer.CommandByTarget))
	return fm.SaveConfig(path, ui.fmCfg)
}

func (ui *UI) loadExistingFMConfig(path string) (*fm.Config, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("config path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	cfg, err := fm.LoadConfigEnsuringFile(path)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

func configLooksLikeCriticalStateReset(existing, next *fm.Config) bool {
	if existing == nil || next == nil || !configHasCriticalUserState(existing) {
		return false
	}
	return !configHasCriticalUserState(next)
}

func configHasCriticalUserState(cfg *fm.Config) bool {
	if cfg == nil {
		return false
	}
	defaultCfg := fm.DefaultConfig()
	if len(cfg.FavoriteLocations) > 0 || len(cfg.SSH.Setups) > 0 || len(cfg.Viewer.CommandByTarget) > 0 || len(cfg.Viewer.CommandHistory) > 0 || len(cfg.Viewer.CommandRules) > 0 || len(cfg.Associations) > 0 {
		return true
	}
	if strings.TrimSpace(cfg.Viewer.Mode) == "command" {
		return true
	}
	return strings.TrimSpace(cfg.Viewer.Command) != strings.TrimSpace(defaultCfg.Viewer.Command)
}
