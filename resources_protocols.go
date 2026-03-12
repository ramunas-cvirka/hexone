package resources

import (
	"errors"
	"hexone/appdata"
	"os"
	"path/filepath"
)

const protocolsFileName = "protocols.yaml"

func ProtocolsYAML() []byte {
	for _, path := range protocolCandidatePaths() {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			return data
		}
	}
	return append([]byte(nil), embeddedProtocolsYAML...)
}

func protocolCandidatePaths() []string {
	paths := make([]string, 0, 5)
	seen := map[string]struct{}{}
	add := func(path string) {
		if path == "" {
			return
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		paths = append(paths, clean)
	}

	add(appdata.ProtocolPath())

	exePath, err := os.Executable()
	if err == nil && exePath != "" {
		exeDir := filepath.Dir(exePath)
		add(filepath.Join(exeDir, protocolsFileName))
		add(filepath.Join(exeDir, "..", "Resources", protocolsFileName))
	}

	add(protocolsFileName)

	return paths
}

func EnsureProtocolSample() error {
	samplePath := appdata.ProtocolSamplePath()
	if samplePath == "" {
		return nil
	}
	if _, err := os.Stat(samplePath); err == nil {
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(samplePath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(samplePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	defer f.Close()
	_, err = f.Write(embeddedProtocolsYAML)
	return err
}
