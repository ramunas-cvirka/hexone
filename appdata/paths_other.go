//go:build !darwin

package appdata

const (
	configFileName  = "fm.yaml"
	sessionFileName = "fm.session.yaml"
)

func ConfigPath() string {
	return configFileName
}

func SessionPath() string {
	return sessionFileName
}
