// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import "hexone/secretstore"

type httpCredentialState struct {
	store       secretstore.Store
	initialized bool
}

// InitializeHTTPCredentialStore enables protected HTTP collection persistence.
// Secret values are stored in the operating system credential vault; YAML files
// contain opaque references only.
func (ui *UI) InitializeHTTPCredentialStore() {
	if ui == nil {
		return
	}
	ui.httpCredentials.store = secretstore.NewKeyring(sshCredentialService)
	ui.httpCredentials.initialized = true
}
