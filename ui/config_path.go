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
	rebasedRuntimeSave := false
	if !allowDefaultReset {
		if rebased, ok := rebaseRuntimeConfigSave(reason, existing, ui.fmCfg); ok {
			ui.fmCfg = rebased
			rebasedRuntimeSave = true
		}
	}
	if !allowDefaultReset && !rebasedRuntimeSave && configLooksLikeCriticalStateReset(existing, ui.fmCfg) {
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

func rebaseRuntimeConfigSave(reason string, existing, next *fm.Config) (*fm.Config, bool) {
	if existing == nil || next == nil {
		return nil, false
	}
	reason = strings.ToLower(strings.TrimSpace(reason))
	if reason == "" {
		reason = "runtime"
	}
	rebased := cloneFMConfigForRuntimeSave(existing)
	switch reason {
	case "viewer-mode":
		rebased.Viewer.Mode = next.Viewer.Mode
		if normalizeViewerMode(next.Viewer.Mode) == "command" {
			rebased.Viewer.Command = next.Viewer.Command
		}
	case "viewer-auto-refresh":
		rebased.Viewer.CommandAutoRefresh = next.Viewer.CommandAutoRefresh
	case "viewer-word-wrap":
		rebased.Viewer.WordWrap = next.Viewer.WordWrap
	case "viewer-encoding":
		rebased.Viewer.FileEncoding = next.Viewer.FileEncoding
	case "viewer-command":
		rebased.Viewer.Mode = next.Viewer.Mode
		rebased.Viewer.Command = next.Viewer.Command
		rebased.Viewer.CommandByTarget = cloneStringMap(next.Viewer.CommandByTarget)
		rebased.Viewer.CommandHistory = cloneStringSlice(next.Viewer.CommandHistory)
	case "favorites-add", "favorites-remove":
		rebased.FavoriteLocations = cloneStringSlice(next.FavoriteLocations)
	case "ssh-modal":
		rebased.SSH.Setups = cloneSSHSetups(next.SSH.Setups)
	default:
		return nil, false
	}
	return rebased, true
}

func cloneFMConfigForRuntimeSave(cfg *fm.Config) *fm.Config {
	if cfg == nil {
		return nil
	}
	out := *cfg
	out.DateFormats = cloneStringSlice(cfg.DateFormats)
	out.FavoriteLocations = cloneStringSlice(cfg.FavoriteLocations)
	out.Columns.FullDropPriority = cloneStringSlice(cfg.Columns.FullDropPriority)
	out.Associations = cloneAssociationPrograms(cfg.Associations)
	out.Viewer.Associations = cloneViewerAssociations(cfg.Viewer.Associations)
	out.Viewer.AssociatedExtensions = cloneStringSlice(cfg.Viewer.AssociatedExtensions)
	out.Viewer.CommandRules = cloneViewerCommandRules(cfg.Viewer.CommandRules)
	out.Viewer.CommandByTarget = cloneStringMap(cfg.Viewer.CommandByTarget)
	out.Viewer.CommandHistory = cloneStringSlice(cfg.Viewer.CommandHistory)
	out.SSH.Setups = cloneSSHSetups(cfg.SSH.Setups)
	return &out
}

func cloneStringSlice(src []string) []string {
	if src == nil {
		return nil
	}
	return append([]string(nil), src...)
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneViewerCommandRules(src []fm.ViewerCommandRule) []fm.ViewerCommandRule {
	if src == nil {
		return nil
	}
	return append([]fm.ViewerCommandRule(nil), src...)
}

func cloneViewerAssociations(src []fm.ViewerAssociation) []fm.ViewerAssociation {
	if src == nil {
		return nil
	}
	return append([]fm.ViewerAssociation(nil), src...)
}

func cloneAssociationPrograms(src []fm.AssociationProgram) []fm.AssociationProgram {
	if src == nil {
		return nil
	}
	dst := append([]fm.AssociationProgram(nil), src...)
	for i := range dst {
		dst[i].Extensions = cloneStringSlice(dst[i].Extensions)
	}
	return dst
}
