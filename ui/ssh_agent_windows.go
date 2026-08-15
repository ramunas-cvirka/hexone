//go:build windows

// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"io"
	"os"
	"strings"
)

const defaultWindowsOpenSSHAgentPipe = `\\.\pipe\openssh-ssh-agent`

func dialSSHAgent(endpoint string) (io.ReadWriteCloser, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" || endpoint == "SSH_AUTH_SOCK" {
		if env := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK")); env != "" {
			endpoint = env
		} else {
			endpoint = defaultWindowsOpenSSHAgentPipe
		}
	}
	return os.OpenFile(endpoint, os.O_RDWR, 0)
}
