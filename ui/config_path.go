// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/appdata"
	"hexone/fm"
	"path/filepath"
)

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

func (ui *UI) saveFMConfig() error {
	if ui == nil {
		return nil
	}
	if ui.fmCfg == nil {
		ui.fmCfg = fm.DefaultConfig()
	}
	return fm.SaveConfig(ui.configSavePath(), ui.fmCfg)
}
