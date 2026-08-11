// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOrCreateWritesSeparateCollectionFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	file, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if file.Version != CurrentVersion || len(file.Collections) == 0 {
		t.Fatalf("unexpected default file: %#v", file)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("created collection file is empty")
	}
	if strings.Contains(string(data), "auth: {}") || strings.Contains(string(data), "auth: null") {
		t.Fatalf("empty auth was persisted:\n%s", data)
	}
}

func TestSaveCreatesBackupAndPreservesOrderedHeaders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	file := DefaultFile()
	if err := Save(path, file); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	request := &file.Collections[0].Folders[0].Requests[0]
	request.Headers = []KeyValue{
		{Name: "X-First", Value: "one"},
		{Name: "X-First", Value: "two"},
		{Name: "X-Disabled", Value: "three", Disabled: true},
	}
	request.Auth = Auth{Type: AuthInherit}
	file.Environments[0].Auth = Auth{Type: AuthBearer, Token: "{{token}}"}
	if err := Save(path, file); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	if _, err := os.Stat(path + backupSuffix); err != nil {
		t.Fatalf("backup missing: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := loaded.Collections[0].Folders[0].Requests[0].Headers
	if len(got) != 3 || got[0].Value != "one" || got[1].Value != "two" || !got[2].Disabled {
		t.Fatalf("headers lost order or state: %#v", got)
	}
	if auth := loaded.Collections[0].Folders[0].Requests[0].Auth; auth.Type != AuthInherit {
		t.Fatalf("request auth=%#v want inherited auth", auth)
	}
	if auth := loaded.Environments[0].Auth; auth.Type != AuthBearer || auth.Token != "{{token}}" {
		t.Fatalf("environment auth=%#v want persisted bearer template", auth)
	}
}

func TestLoadMigratesLegacyScalarAuth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	legacy := []byte(`version: 1
environments:
  - name: local
collections:
  - name: API
    requests:
      - name: health
        method: GET
        url: https://example.test
        auth: Bearer {{token}}
`)
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != CurrentVersion {
		t.Fatalf("version=%d want %d", loaded.Version, CurrentVersion)
	}
	auth := loaded.Collections[0].Requests[0].Auth
	if auth.Type != AuthBearer || auth.Token != "{{token}}" {
		t.Fatalf("migrated auth=%#v", auth)
	}
}

func TestLoadRejectsNewerCollectionVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	if err := os.WriteFile(path, []byte("version: 99\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load accepted a newer collection version")
	}
}
