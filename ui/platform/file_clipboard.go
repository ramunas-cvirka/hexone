// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"net/url"
	"path/filepath"
	"strings"
)

func normalizeClipboardFilePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, raw := range paths {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		abs, err := filepath.Abs(raw)
		if err != nil {
			continue
		}
		clean := filepath.Clean(abs)
		key := clean
		if filepath.Separator == '\\' {
			key = strings.ToLower(key)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, clean)
	}
	return out
}

func fileClipboardURI(path string) string {
	slashPath := filepath.ToSlash(path)
	if filepath.Separator == '\\' && !strings.HasPrefix(slashPath, "/") {
		slashPath = "/" + slashPath
	}
	return (&url.URL{Scheme: "file", Path: slashPath}).String()
}

func parseFileClipboardURIs(raw string) []string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	paths := make([]string, 0, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i == 0 && (line == "copy" || line == "cut") {
			continue
		}
		u, err := url.Parse(line)
		if err != nil || !strings.EqualFold(u.Scheme, "file") {
			continue
		}
		if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
			continue
		}
		decoded, err := url.PathUnescape(u.EscapedPath())
		if err != nil {
			continue
		}
		if filepath.Separator == '\\' && len(decoded) >= 3 && decoded[0] == '/' && decoded[2] == ':' {
			decoded = decoded[1:]
		}
		paths = append(paths, filepath.FromSlash(decoded))
	}
	return normalizeClipboardFilePaths(paths)
}
