// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package platform

import "errors"

func ReadClipboardTextNow() (string, error) {
	return "", errors.New("synchronous clipboard read unsupported")
}

func WriteClipboardTextNow(string) error {
	return errors.New("synchronous clipboard write unsupported")
}
