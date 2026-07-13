// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hexone/fm"
	"hexone/secretstore"
)

type memorySecretStore struct {
	values map[string]string
}

func newMemorySecretStore() *memorySecretStore {
	return &memorySecretStore{values: make(map[string]string)}
}

func (s *memorySecretStore) Get(key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", secretstore.ErrNotFound
	}
	return value, nil
}

func (s *memorySecretStore) Set(key, value string) error {
	s.values[key] = value
	return nil
}

func (s *memorySecretStore) Delete(key string) error {
	delete(s.values, key)
	return nil
}

func TestPlaintextSSHCredentialsMigrateToSecretStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hexone.yaml")
	legacy := []byte("ssh:\n  setups:\n    - host: example.test\n      port: 22\n      user: alice\n      password: plain-password\n      key_path: /home/alice/.ssh/id_ed25519\n      key_passphrase: plain-passphrase\n")
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(path+".bak", legacy, 0o644); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	cfg, err := fm.LoadConfigEnsuringFile(path)
	if err != nil {
		t.Fatalf("LoadConfigEnsuringFile: %v", err)
	}
	store := newMemorySecretStore()
	ui := NewUI(cfg)
	ui.configPath = path
	ui.sshCredentials.store = store

	if err := ui.loadSSHSecrets(true); err != nil {
		t.Fatalf("loadSSHSecrets migration: %v", err)
	}
	setup := ui.fmCfg.SSH.Setups[0]
	if setup.CredentialID == "" {
		t.Fatal("migration did not assign a credential ID")
	}
	stored, err := loadSSHSecrets(store, setup.CredentialID)
	if err != nil {
		t.Fatalf("load migrated secret: %v", err)
	}
	if stored.Password != "plain-password" || stored.KeyPassphrase != "plain-passphrase" {
		t.Fatalf("migrated secrets = %+v", stored)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "plain-password") || strings.Contains(text, "plain-passphrase") {
		t.Fatalf("migrated config retained plaintext secrets:\n%s", text)
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("plaintext config backup still exists: %v", err)
	}

	reloaded, err := fm.LoadConfigEnsuringFile(path)
	if err != nil {
		t.Fatalf("reload migrated config: %v", err)
	}
	reloadedUI := NewUI(reloaded)
	reloadedUI.sshCredentials.store = store
	if err := reloadedUI.loadSSHSecrets(false); err != nil {
		t.Fatalf("hydrate migrated config: %v", err)
	}
	got := reloadedUI.fmCfg.SSH.Setups[0]
	if got.Password != "plain-password" || got.KeyPassphrase != "plain-passphrase" {
		t.Fatalf("hydrated setup secrets = password %q passphrase %q", got.Password, got.KeyPassphrase)
	}
}

func TestPrepareSSHSecretsForSaveUsesCredentialReference(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	store := newMemorySecretStore()
	ui.sshCredentials.store = store
	setups, removed, err := ui.prepareSSHSecretsForSave([]fm.SSHSetup{{
		Host:          "example.test",
		Port:          22,
		User:          "alice",
		Password:      "plain-password",
		KeyPassphrase: "plain-passphrase",
	}})
	if err != nil {
		t.Fatalf("prepareSSHSecretsForSave: %v", err)
	}
	if len(removed) != 0 || len(setups) != 1 || setups[0].CredentialID == "" {
		t.Fatalf("prepared setups=%+v removed=%v", setups, removed)
	}
	stored, err := loadSSHSecrets(store, setups[0].CredentialID)
	if err != nil {
		t.Fatalf("load stored secret: %v", err)
	}
	if stored.Password != "plain-password" || stored.KeyPassphrase != "plain-passphrase" {
		t.Fatalf("stored secrets = %+v", stored)
	}
}

func TestRuntimeConfigSaveKeepsHydratedSSHSecretsInMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	cfg := fm.DefaultConfig()
	cfg.SSH.Setups = []fm.SSHSetup{{
		Host:         "example.test",
		Port:         22,
		User:         "alice",
		CredentialID: "credential-id",
	}}
	if err := fm.SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	runtimeCfg := fm.LoadConfig(path)
	runtimeCfg.SSH.Setups[0].Password = "plain-password"
	ui := NewUI(runtimeCfg)
	ui.configPath = path
	ui.fmCfg.Viewer.WordWrap = true
	if err := ui.saveFMConfigWithOptions("viewer-word-wrap", false); err != nil {
		t.Fatalf("saveFMConfigWithOptions: %v", err)
	}
	if got := ui.fmCfg.SSH.Setups[0].Password; got != "plain-password" {
		t.Fatalf("in-memory password=%q want preserved secret", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(data), "plain-password") {
		t.Fatalf("runtime save wrote plaintext secret:\n%s", data)
	}
}
