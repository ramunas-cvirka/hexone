// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"io"
	"net"
	"os"
	"strings"
)

func dialSSHAgent(endpoint string) (io.ReadWriteCloser, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" || endpoint == "SSH_AUTH_SOCK" {
		endpoint = strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
	}
	if endpoint == "" {
		return nil, errors.New("SSH_AUTH_SOCK is not set")
	}
	return net.Dial("unix", endpoint)
}
