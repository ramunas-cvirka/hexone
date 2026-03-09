//go:build darwin

package appdata

import (
	"hexone/appicon"
	"os"
	"path/filepath"

	"gioui.org/app"
)

const (
	configFileName  = "fm.yaml"
	sessionFileName = "fm.session.yaml"
)

func ConfigPath() string {
	return dataFilePath(configFileName)
}

func SessionPath() string {
	return dataFilePath(sessionFileName)
}

func dataFilePath(name string) string {
	base, err := app.DataDir()
	if err != nil || base == "" {
		return name
	}
	base = filepath.Join(base, appicon.AppID)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return name
	}
	return filepath.Join(base, name)
}
