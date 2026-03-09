package resources

import (
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
	paths := make([]string, 0, 4)
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

	add(protocolsFileName)

	exePath, err := os.Executable()
	if err == nil && exePath != "" {
		exeDir := filepath.Dir(exePath)
		add(filepath.Join(exeDir, protocolsFileName))
		add(filepath.Join(exeDir, "..", "Resources", protocolsFileName))
	}

	return paths
}
