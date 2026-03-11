//go:build !darwin

package appdata

const (
	configFileName  = "hexone.yaml"
	sessionFileName = "hexone.session.yaml"
)

func ConfigPath() string {
	return configFileName
}

func SessionPath() string {
	return sessionFileName
}
