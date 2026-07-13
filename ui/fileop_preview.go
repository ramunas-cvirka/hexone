// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"strings"
)

func fileOpPreviewLabel(name, path string) string {
	label := strings.TrimSpace(name)
	if label != "" {
		return label
	}
	return strings.TrimSpace(path)
}
