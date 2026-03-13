// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package buildinfo

import "strings"

var (
	Version    = "dev"
	SemVersion = "0.0.0"
	Commit     = "unknown"
)

func DisplayVersion() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		return "dev"
	}
	return v
}

func ReleaseVersion() string {
	v := strings.TrimSpace(SemVersion)
	if v == "" {
		return "0.0.0"
	}
	return strings.TrimPrefix(v, "v")
}

func HelpVersionText() string {
	v := DisplayVersion()
	c := strings.TrimSpace(Commit)
	if c == "" || c == "unknown" || strings.Contains(v, c) {
		return "Version " + v
	}
	return "Version " + v + " (" + c + ")"
}
