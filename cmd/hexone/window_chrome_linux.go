//go:build linux

package main

import (
	"os"
	"strings"
)

func useClientWindowChrome() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("HEXONE_CLIENT_DECORATIONS"))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	if strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("XDG_SESSION_TYPE")), "wayland")
}
