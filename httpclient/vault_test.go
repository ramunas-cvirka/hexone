// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type memorySecretStore struct {
	values  map[string]string
	setErr  error
	getErr  error
	deletes []string
}

func newMemorySecretStore() *memorySecretStore {
	return &memorySecretStore{values: make(map[string]string)}
}

func (s *memorySecretStore) Get(key string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	value, ok := s.values[key]
	if !ok {
		return "", errors.New("not found")
	}
	return value, nil
}

func (s *memorySecretStore) Set(key, value string) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.values[key] = value
	return nil
}

func (s *memorySecretStore) Delete(key string) error {
	delete(s.values, key)
	s.deletes = append(s.deletes, key)
	return nil
}

func protectedHTTPFile() *File {
	return &File{
		Version: CurrentVersion,
		Environments: []Environment{{
			Name: "local",
			Variables: map[string]string{
				"base_url": "https://private.example.test",
				"token":    "environment-secret",
			},
			Auth: Auth{Type: AuthBasic, Username: "ada", Password: "basic-secret"},
		}},
		Collections: []Collection{{
			Name: "API",
			Requests: []Request{{
				Name: "Bearer request", Method: "GET", URL: "{{base_url}}/v1",
				Auth: Auth{Type: AuthBearer, Token: "bearer-secret"},
			}},
			Folders: []Folder{{Name: "Admin", Requests: []Request{{
				Name: "Key request", Method: "POST", URL: "{{base_url}}/admin",
				Auth: Auth{Type: AuthAPIKey, Key: "X-API-Key", Value: "api-key-secret", In: AuthInHeader},
			}}}},
		}},
	}
}

func TestVaultSaveKeepsSecretsOutOfYAMLAndHydratesThem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	store := newMemorySecretStore()
	file := protectedHTTPFile()
	vault := NewVault(store)
	if err := vault.Save(path, file); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, secret := range []string{"private.example.test", "environment-secret", "basic-secret", "bearer-secret", "api-key-secret"} {
		if strings.Contains(text, secret) {
			t.Fatalf("YAML contains protected value %q:\n%s", secret, text)
		}
	}
	if !strings.Contains(text, "credential_id:") || !strings.Contains(text, "variables_credential_id:") {
		t.Fatalf("YAML is missing credential references:\n%s", text)
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("collection mode=%#o want 0600", got)
	}
	file.Collections[0].Name = "Renamed API"
	if err := vault.Save(path, file); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + backupSuffix)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"environment-secret", "basic-secret", "bearer-secret", "api-key-secret"} {
		if strings.Contains(string(backup), secret) {
			t.Fatalf("backup contains protected value %q:\n%s", secret, backup)
		}
	}
	if info, err := os.Stat(path + backupSuffix); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("backup mode=%#o want 0600", got)
	}

	loaded, err := NewVault(store).LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Environments[0].Variables["token"]; got != "environment-secret" {
		t.Fatalf("environment token=%q", got)
	}
	if got := loaded.Environments[0].Auth.Password; got != "basic-secret" {
		t.Fatalf("basic password=%q", got)
	}
	if got := loaded.Collections[0].Requests[0].Auth.Token; got != "bearer-secret" {
		t.Fatalf("bearer token=%q", got)
	}
	if got := loaded.Collections[0].Folders[0].Requests[0].Auth.Value; got != "api-key-secret" {
		t.Fatalf("API key=%q", got)
	}
}

func TestVaultMigratesPlaintextWithoutLeavingPlaintextBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	legacy := protectedHTTPFile()
	if err := Save(path, legacy); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+backupSuffix, []byte("bearer-secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	store := newMemorySecretStore()
	loaded, err := NewVault(store).LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Collections[0].Requests[0].Auth.Token != "bearer-secret" {
		t.Fatal("migrated credential was not hydrated")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "bearer-secret") {
		t.Fatalf("migrated YAML still contains plaintext:\n%s", data)
	}
	if _, err := os.Stat(path + backupSuffix); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("plaintext backup still exists: %v", err)
	}
}

func TestVaultStoreFailureDoesNotWriteCollectionFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	original := []byte("version: 3\ncollections: []\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store := newMemorySecretStore()
	store.setErr = errors.New("vault locked")
	if err := NewVault(store).Save(path, protectedHTTPFile()); err == nil {
		t.Fatal("Save succeeded with a locked vault")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatalf("file changed after vault failure:\n%s", data)
	}
}

func TestVaultRotatesAndDeletesOldCredentialReferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	store := newMemorySecretStore()
	vault := NewVault(store)
	file := protectedHTTPFile()
	if err := vault.Save(path, file); err != nil {
		t.Fatal(err)
	}
	oldCount := len(store.values)
	if oldCount == 0 {
		t.Fatal("no credentials stored")
	}
	file.Collections[0].Requests[0].Auth = Auth{Type: AuthNone}
	if err := vault.Save(path, file); err != nil {
		t.Fatal(err)
	}
	if len(store.deletes) < oldCount {
		t.Fatalf("deleted %d old entries, want at least %d", len(store.deletes), oldCount)
	}
	if len(store.values) != oldCount-1 {
		t.Fatalf("stored entries=%d want %d after removing bearer auth", len(store.values), oldCount-1)
	}
}
