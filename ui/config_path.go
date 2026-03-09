package ui

import (
	"hexone/appdata"
	"hexone/fm"
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

func (ui *UI) saveFMConfig() error {
	if ui == nil {
		return nil
	}
	if ui.fmCfg == nil {
		ui.fmCfg = fm.DefaultConfig()
	}
	return fm.SaveConfig(ui.configSavePath(), ui.fmCfg)
}
