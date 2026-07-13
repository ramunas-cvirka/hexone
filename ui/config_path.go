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
	if configHasUnstoredSSHSecrets(ui.fmCfg) {
		if ui.sshCredentials.store == nil {
			return errors.New("secure credential storage is unavailable")
		}
		if err := ui.loadSSHSecrets(true); err != nil {
			ui.sshCredentials.plaintextMigrationBlocked = true
			return err
		}
		ui.sshCredentials.plaintextMigrationBlocked = false
	}
	if ui.sshCredentials.plaintextMigrationBlocked {
		return errors.New("refusing to save while plaintext SSH credentials could not be moved to secure storage")
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
	log.Printf("save config: reason=%s path=%s favorites=%d ssh=%d custom_commands=%d command_history=%d command_targets=%d", reason, path, len(ui.fmCfg.FavoriteLocations), len(ui.fmCfg.SSH.Setups), len(ui.fmCfg.CustomCommands), len(ui.fmCfg.Viewer.CommandHistory), len(ui.fmCfg.Viewer.CommandByTarget))
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
	if configHasUnstoredSSHSecrets(cfg) {
		if ui.sshCredentials.store == nil {
			return cfg, errors.New("secure credential storage is unavailable for SSH credentials added to the config")
		}
		active := ui.fmCfg
		ui.fmCfg = cfg
		err = ui.loadSSHSecrets(true)
		ui.fmCfg = active
		if err != nil {
			ui.sshCredentials.plaintextMigrationBlocked = true
			return cfg, err
		}
		ui.sshCredentials.plaintextMigrationBlocked = false
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
	if len(cfg.FavoriteLocations) > 0 || len(cfg.SSH.Setups) > 0 || len(cfg.CustomCommands) > 0 || len(cfg.Viewer.CommandByTarget) > 0 || len(cfg.Viewer.CommandHistory) > 0 || len(cfg.Viewer.CommandRules) > 0 || len(cfg.Associations) > 0 {
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
	mergeRuntimeSSHSecrets(rebased, next)
	switch reason {
	case "viewer-auto-refresh":
		rebased.Viewer.CommandAutoRefresh = next.Viewer.CommandAutoRefresh
	case "viewer-word-wrap":
		rebased.Viewer.WordWrap = next.Viewer.WordWrap
	case "viewer-encoding":
		rebased.Viewer.FileEncoding = next.Viewer.FileEncoding
	case "terminal-height":
		rebased.Terminal.HeightRows = next.Terminal.HeightRows
	case "terminal-layout":
		rebased.Terminal.HeightRows = next.Terminal.HeightRows
		rebased.Terminal.Maximized = next.Terminal.Maximized
	case "sort-dir":
		rebased.Sort.PerDir = cloneStringMap(next.Sort.PerDir)
	case "viewer-command":
		rebased.Viewer.Command = next.Viewer.Command
		rebased.Viewer.CommandByTarget = cloneStringMap(next.Viewer.CommandByTarget)
		rebased.Viewer.CommandHistory = cloneStringSlice(next.Viewer.CommandHistory)
	case "custom-commands":
		rebased.CustomCommands = cloneCustomCommands(next.CustomCommands)
	case "favorites-add", "favorites-remove":
		rebased.FavoriteLocations = cloneStringSlice(next.FavoriteLocations)
	case "ssh-modal":
		rebased.SSH.Setups = cloneSSHSetups(next.SSH.Setups)
	default:
		return nil, false
	}
	return rebased, true
}

func mergeRuntimeSSHSecrets(dst, src *fm.Config) {
	if dst == nil || src == nil || len(dst.SSH.Setups) == 0 || len(src.SSH.Setups) == 0 {
		return
	}
	byID := make(map[string]fm.SSHSetup, len(src.SSH.Setups))
	byIdentity := make(map[string]fm.SSHSetup, len(src.SSH.Setups))
	for _, setup := range src.SSH.Setups {
		if id := strings.TrimSpace(setup.CredentialID); id != "" {
			byID[id] = setup
		}
		byIdentity[runtimeSSHSetupIdentity(setup)] = setup
	}
	for i := range dst.SSH.Setups {
		setup := &dst.SSH.Setups[i]
		match, ok := byID[strings.TrimSpace(setup.CredentialID)]
		if !ok || strings.TrimSpace(setup.CredentialID) == "" {
			match, ok = byIdentity[runtimeSSHSetupIdentity(*setup)]
		}
		if !ok {
			continue
		}
		setup.Password = match.Password
		setup.KeyPassphrase = match.KeyPassphrase
	}
}

func runtimeSSHSetupIdentity(setup fm.SSHSetup) string {
	port := setup.Port
	if port <= 0 {
		port = 22
	}
	return strings.ToLower(strings.TrimSpace(setup.User)) + "\x00" +
		strings.ToLower(strings.TrimSpace(setup.Host)) + "\x00" + fmt.Sprint(port)
}

func cloneFMConfigForRuntimeSave(cfg *fm.Config) *fm.Config {
	if cfg == nil {
		return nil
	}
	out := *cfg
	out.DateFormats = cloneStringSlice(cfg.DateFormats)
	out.FavoriteLocations = cloneStringSlice(cfg.FavoriteLocations)
	out.Columns.FullDropPriority = cloneStringSlice(cfg.Columns.FullDropPriority)
	out.Sort.PerDir = cloneStringMap(cfg.Sort.PerDir)
	out.Associations = cloneAssociationPrograms(cfg.Associations)
	out.CustomCommands = cloneCustomCommands(cfg.CustomCommands)
	out.Viewer.Associations = cloneViewerAssociations(cfg.Viewer.Associations)
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

func cloneCustomCommands(src []fm.CustomCommand) []fm.CustomCommand {
	if src == nil {
		return nil
	}
	return append([]fm.CustomCommand(nil), src...)
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
