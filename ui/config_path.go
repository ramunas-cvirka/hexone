package ui

import (
	"hexone/fm"
	"path/filepath"
)

const uiDefaultConfigPath = "fm.yaml"

func resolveUIConfigPath() string {
	abs, err := filepath.Abs(uiDefaultConfigPath)
	if err != nil || abs == "" {
		return uiDefaultConfigPath
	}
	return abs
}

func (ui *UI) configSavePath() string {
	if ui == nil || ui.configPath == "" {
		return uiDefaultConfigPath
	}
	return ui.configPath
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
