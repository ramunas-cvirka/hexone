// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package secretstore

import (
	"errors"
	"strings"

	"github.com/zalando/go-keyring"
)

var ErrNotFound = errors.New("secret not found")

type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
}

type Keyring struct {
	service string
}

func NewKeyring(service string) *Keyring {
	return &Keyring{service: strings.TrimSpace(service)}
}

func (s *Keyring) Get(key string) (string, error) {
	value, err := keyring.Get(s.service, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return value, err
}

func (s *Keyring) Set(key, value string) error {
	return keyring.Set(s.service, key, value)
}

func (s *Keyring) Delete(key string) error {
	err := keyring.Delete(s.service, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
