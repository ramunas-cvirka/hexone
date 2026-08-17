// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const httpCredentialKeyPrefix = "http:"

type SecretStore interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
}

type Vault struct {
	store SecretStore
	ids   map[string]struct{}
}

type storedAuthSecrets struct {
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
	Value    string `json:"value,omitempty"`
}

type storedVariableSecrets struct {
	Values map[string]string `json:"values"`
}

func NewVault(store SecretStore) *Vault {
	return &Vault{store: store, ids: make(map[string]struct{})}
}

func (v *Vault) LoadOrCreate(path string) (*File, error) {
	file, err := Load(path)
	if errors.Is(err, os.ErrNotExist) {
		file = DefaultFile()
		if err := v.save(path, file, false); err != nil {
			return nil, err
		}
		return file, nil
	}
	if err != nil {
		return nil, err
	}
	plaintext := hasPersistedHTTPSecrets(file)
	if err := v.hydrate(file); err != nil {
		return nil, err
	}
	if plaintext {
		if err := os.Remove(path + backupSuffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("remove plaintext HTTP backup: %w", err)
		}
		if err := v.save(path, file, false); err != nil {
			return nil, fmt.Errorf("migrate HTTP credentials: %w", err)
		}
	}
	return file, nil
}

func (v *Vault) Save(path string, file *File) error {
	return v.save(path, file, true)
}

func (v *Vault) save(path string, file *File, backup bool) error {
	if v == nil || v.store == nil {
		return errors.New("secure credential storage is unavailable")
	}
	prepared := cloneHTTPFile(file)
	newIDs := make(map[string]struct{})
	cleanup := true
	defer func() {
		if cleanup {
			for id := range newIDs {
				_ = v.store.Delete(httpCredentialKeyPrefix + id)
			}
		}
	}()
	if err := v.protect(prepared, file, newIDs); err != nil {
		return err
	}
	if err := save(path, prepared, backup); err != nil {
		return err
	}
	cleanup = false
	oldIDs := v.ids
	v.ids = newIDs
	applyHTTPCredentialIDs(file, prepared)
	for id := range oldIDs {
		if _, keep := newIDs[id]; !keep {
			_ = v.store.Delete(httpCredentialKeyPrefix + id)
		}
	}
	return nil
}

func (v *Vault) protect(dst, src *File, ids map[string]struct{}) error {
	for i := range src.Environments {
		if len(src.Environments[i].Variables) > 0 {
			id, err := v.storeJSON(storedVariableSecrets{Values: src.Environments[i].Variables}, ids)
			if err != nil {
				return fmt.Errorf("store variables for environment %s: %w", src.Environments[i].Name, err)
			}
			dst.Environments[i].VariablesCredentialID = id
			for key := range dst.Environments[i].Variables {
				dst.Environments[i].Variables[key] = ""
			}
		} else {
			dst.Environments[i].VariablesCredentialID = ""
		}
		if err := v.protectAuth(&dst.Environments[i].Auth, src.Environments[i].Auth, ids); err != nil {
			return fmt.Errorf("store auth for environment %s: %w", src.Environments[i].Name, err)
		}
	}
	for ci := range src.Collections {
		for ri := range src.Collections[ci].Requests {
			if err := v.protectAuth(&dst.Collections[ci].Requests[ri].Auth, src.Collections[ci].Requests[ri].Auth, ids); err != nil {
				return fmt.Errorf("store auth for request %s: %w", src.Collections[ci].Requests[ri].Name, err)
			}
		}
		for fi := range src.Collections[ci].Folders {
			for ri := range src.Collections[ci].Folders[fi].Requests {
				if err := v.protectAuth(&dst.Collections[ci].Folders[fi].Requests[ri].Auth, src.Collections[ci].Folders[fi].Requests[ri].Auth, ids); err != nil {
					return fmt.Errorf("store auth for request %s: %w", src.Collections[ci].Folders[fi].Requests[ri].Name, err)
				}
			}
		}
	}
	return nil
}

func (v *Vault) protectAuth(dst *Auth, src Auth, ids map[string]struct{}) error {
	secrets := storedAuthSecrets{Password: src.Password, Token: src.Token, Value: src.Value}
	dst.Password, dst.Token, dst.Value = "", "", ""
	if secrets == (storedAuthSecrets{}) {
		dst.CredentialID = ""
		return nil
	}
	id, err := v.storeJSON(secrets, ids)
	if err != nil {
		return err
	}
	dst.CredentialID = id
	return nil
}

func (v *Vault) storeJSON(value any, ids map[string]struct{}) (string, error) {
	id, err := newHTTPCredentialID()
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	ids[id] = struct{}{}
	if err := v.store.Set(httpCredentialKeyPrefix+id, string(data)); err != nil {
		return "", err
	}
	return id, nil
}

func (v *Vault) hydrate(file *File) error {
	if v == nil {
		return errors.New("HTTP credential vault is nil")
	}
	for i := range file.Environments {
		env := &file.Environments[i]
		if env.VariablesCredentialID != "" {
			var secrets storedVariableSecrets
			if err := v.loadJSON(env.VariablesCredentialID, &secrets); err != nil {
				return fmt.Errorf("load variables for environment %s: %w", env.Name, err)
			}
			env.Variables = secrets.Values
		}
		if err := v.hydrateAuth(&env.Auth); err != nil {
			return fmt.Errorf("load auth for environment %s: %w", env.Name, err)
		}
	}
	for ci := range file.Collections {
		for ri := range file.Collections[ci].Requests {
			if err := v.hydrateAuth(&file.Collections[ci].Requests[ri].Auth); err != nil {
				return fmt.Errorf("load auth for request %s: %w", file.Collections[ci].Requests[ri].Name, err)
			}
		}
		for fi := range file.Collections[ci].Folders {
			for ri := range file.Collections[ci].Folders[fi].Requests {
				request := &file.Collections[ci].Folders[fi].Requests[ri]
				if err := v.hydrateAuth(&request.Auth); err != nil {
					return fmt.Errorf("load auth for request %s: %w", request.Name, err)
				}
			}
		}
	}
	return nil
}

func (v *Vault) hydrateAuth(auth *Auth) error {
	if auth.CredentialID == "" {
		return nil
	}
	var secrets storedAuthSecrets
	if err := v.loadJSON(auth.CredentialID, &secrets); err != nil {
		return err
	}
	auth.Password, auth.Token, auth.Value = secrets.Password, secrets.Token, secrets.Value
	return nil
}

func (v *Vault) loadJSON(id string, dst any) error {
	if v.store == nil {
		return errors.New("secure credential storage is unavailable")
	}
	raw, err := v.store.Get(httpCredentialKeyPrefix + id)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		return fmt.Errorf("decode credential: %w", err)
	}
	v.ids[id] = struct{}{}
	return nil
}

func hasPersistedHTTPSecrets(file *File) bool {
	if file == nil {
		return false
	}
	checkAuth := func(auth Auth) bool { return auth.Password != "" || auth.Token != "" || auth.Value != "" }
	for _, env := range file.Environments {
		for _, value := range env.Variables {
			if value != "" {
				return true
			}
		}
		if checkAuth(env.Auth) {
			return true
		}
	}
	for _, collection := range file.Collections {
		for _, request := range collection.Requests {
			if checkAuth(request.Auth) {
				return true
			}
		}
		for _, folder := range collection.Folders {
			for _, request := range folder.Requests {
				if checkAuth(request.Auth) {
					return true
				}
			}
		}
	}
	return false
}

func newHTTPCredentialID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func cloneHTTPFile(src *File) *File {
	if src == nil {
		return nil
	}
	dst := *src
	dst.Environments = append([]Environment(nil), src.Environments...)
	for i := range dst.Environments {
		dst.Environments[i].Variables = make(map[string]string, len(src.Environments[i].Variables))
		for k, value := range src.Environments[i].Variables {
			dst.Environments[i].Variables[k] = value
		}
	}
	dst.Collections = append([]Collection(nil), src.Collections...)
	for ci := range dst.Collections {
		dst.Collections[ci].Requests = append([]Request(nil), src.Collections[ci].Requests...)
		dst.Collections[ci].Folders = append([]Folder(nil), src.Collections[ci].Folders...)
		for fi := range dst.Collections[ci].Folders {
			dst.Collections[ci].Folders[fi].Requests = append([]Request(nil), src.Collections[ci].Folders[fi].Requests...)
		}
	}
	return &dst
}

func applyHTTPCredentialIDs(dst, src *File) {
	for i := range dst.Environments {
		dst.Environments[i].VariablesCredentialID = src.Environments[i].VariablesCredentialID
		dst.Environments[i].Auth.CredentialID = src.Environments[i].Auth.CredentialID
	}
	for ci := range dst.Collections {
		for ri := range dst.Collections[ci].Requests {
			dst.Collections[ci].Requests[ri].Auth.CredentialID = src.Collections[ci].Requests[ri].Auth.CredentialID
		}
		for fi := range dst.Collections[ci].Folders {
			for ri := range dst.Collections[ci].Folders[fi].Requests {
				dst.Collections[ci].Folders[fi].Requests[ri].Auth.CredentialID = src.Collections[ci].Folders[fi].Requests[ri].Auth.CredentialID
			}
		}
	}
}
