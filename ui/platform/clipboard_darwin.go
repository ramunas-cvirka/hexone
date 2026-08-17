// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package platform

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

const clipboardCommandTimeout = 2 * time.Second

// ReadClipboardTextNow reads text directly from the macOS pasteboard. Keeping
// this synchronous path alongside Gio's clipboard events makes context-menu
// paste reliable even when focus or popup state changes during the frame.
func ReadClipboardTextNow() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), clipboardCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "pbpaste").Output()
	if err != nil {
		return "", fmt.Errorf("read clipboard text: %w", err)
	}
	return string(out), nil
}

// WriteClipboardTextNow writes text directly to the macOS pasteboard. Gio's
// portable clipboard command remains in use as well, but this path prevents a
// copy from being lost if the deferred command is dropped with a closing popup.
func WriteClipboardTextNow(text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), clipboardCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pbcopy")
	cmd.Stdin = bytes.NewBufferString(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("write clipboard text: %w", err)
	}
	return nil
}
