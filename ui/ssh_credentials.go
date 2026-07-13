// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hexone/fm"
	"hexone/secretstore"
	"strings"
)

const sshCredentialService = "hexone"

type storedSSHSecrets struct {
	Password      string `json:"password,omitempty"`
	KeyPassphrase string `json:"key_passphrase,omitempty"`
}

type sshCredentialState struct {
	store                     secretstore.Store
	plaintextMigrationBlocked bool
}

// InitializeSSHCredentialStore loads SSH authentication secrets from the
// operating system vault and migrates any values written by older versions.
func (ui *UI) InitializeSSHCredentialStore() error {
	if ui == nil {
		return nil
	}
	ui.sshCredentials.store = secretstore.NewKeyring(sshCredentialService)
	hadPlaintext := configHasUnstoredSSHSecrets(ui.fmCfg)
	err := ui.loadSSHSecrets(true)
	ui.sshCredentials.plaintextMigrationBlocked = hadPlaintext && err != nil
	return err
}

func configHasUnstoredSSHSecrets(cfg *fm.Config) bool {
	if cfg == nil {
		return false
	}
	for _, setup := range cfg.SSH.Setups {
		if (setup.Password != "" || setup.KeyPassphrase != "") && strings.TrimSpace(setup.CredentialID) == "" {
			return true
		}
	}
	return false
}

func (ui *UI) loadSSHSecrets(allowPlaintextMigration bool) error {
	if ui == nil || ui.fmCfg == nil || ui.sshCredentials.store == nil {
		return nil
	}
	migrated := false
	for i := range ui.fmCfg.SSH.Setups {
		setup := &ui.fmCfg.SSH.Setups[i]
		setup.CredentialID = strings.TrimSpace(setup.CredentialID)
		legacy := setup.Password != "" || setup.KeyPassphrase != ""
		if legacy && allowPlaintextMigration {
			if setup.CredentialID == "" {
				id, err := newSSHCredentialID()
				if err != nil {
					return err
				}
				setup.CredentialID = id
			}
			if err := storeSSHSecrets(ui.sshCredentials.store, setup.CredentialID, storedSSHSecrets{
				Password:      setup.Password,
				KeyPassphrase: setup.KeyPassphrase,
			}); err != nil {
				return fmt.Errorf("migrate credentials for %s: %w", sshSetupIdentity(*setup), err)
			}
			migrated = true
			continue
		}
		if setup.CredentialID == "" {
			continue
		}
		secrets, err := loadSSHSecrets(ui.sshCredentials.store, setup.CredentialID)
		if errors.Is(err, secretstore.ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("load credentials for %s: %w", sshSetupIdentity(*setup), err)
		}
		setup.Password = secrets.Password
		setup.KeyPassphrase = secrets.KeyPassphrase
	}
	if !migrated {
		return nil
	}
	if err := fm.RewriteConfigWithoutBackup(ui.configSavePath(), ui.fmCfg); err != nil {
		return fmt.Errorf("remove migrated SSH credentials from config: %w", err)
	}
	return nil
}

func (ui *UI) prepareSSHSecretsForSave(setups []fm.SSHSetup) ([]fm.SSHSetup, []string, error) {
	prepared := cloneSSHSetups(setups)
	removed := make(map[string]struct{})
	for _, setup := range ui.fmCfg.SSH.Setups {
		if id := strings.TrimSpace(setup.CredentialID); id != "" {
			removed[id] = struct{}{}
		}
	}

	for i := range prepared {
		setup := &prepared[i]
		setup.CredentialID = strings.TrimSpace(setup.CredentialID)
		if setup.Password == "" && setup.KeyPassphrase == "" {
			setup.CredentialID = ""
			continue
		}
		if ui.sshCredentials.store == nil {
			return nil, nil, errors.New("secure credential storage is unavailable")
		}
		if setup.CredentialID == "" {
			id, err := newSSHCredentialID()
			if err != nil {
				return nil, nil, err
			}
			setup.CredentialID = id
		}
		if err := storeSSHSecrets(ui.sshCredentials.store, setup.CredentialID, storedSSHSecrets{
			Password:      setup.Password,
			KeyPassphrase: setup.KeyPassphrase,
		}); err != nil {
			return nil, nil, fmt.Errorf("store credentials for %s: %w", sshSetupIdentity(*setup), err)
		}
		delete(removed, setup.CredentialID)
	}

	removedIDs := make([]string, 0, len(removed))
	for id := range removed {
		removedIDs = append(removedIDs, id)
	}
	return prepared, removedIDs, nil
}

func (ui *UI) deleteSSHSecrets(ids []string) {
	if ui == nil || ui.sshCredentials.store == nil {
		return
	}
	for _, id := range ids {
		_ = ui.sshCredentials.store.Delete(sshCredentialKey(id))
	}
}

func newSSHCredentialID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate credential identifier: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func sshCredentialKey(id string) string {
	return "ssh:" + strings.TrimSpace(id)
}

func storeSSHSecrets(store secretstore.Store, id string, secrets storedSSHSecrets) error {
	data, err := json.Marshal(secrets)
	if err != nil {
		return err
	}
	return store.Set(sshCredentialKey(id), string(data))
}

func loadSSHSecrets(store secretstore.Store, id string) (storedSSHSecrets, error) {
	raw, err := store.Get(sshCredentialKey(id))
	if err != nil {
		return storedSSHSecrets{}, err
	}
	var secrets storedSSHSecrets
	if err := json.Unmarshal([]byte(raw), &secrets); err != nil {
		return storedSSHSecrets{}, fmt.Errorf("decode stored credentials: %w", err)
	}
	return secrets, nil
}
