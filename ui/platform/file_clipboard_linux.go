// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package platform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	gnomeCopiedFilesMIME = "x-special/gnome-copied-files"
	uriListMIME          = "text/uri-list"
)

func ReadClipboardFiles() ([]string, error) {
	var lastErr error
	for _, mime := range []string{gnomeCopiedFilesMIME, uriListMIME} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		var cmd *exec.Cmd
		switch {
		case commandAvailable("wl-paste"):
			cmd = exec.CommandContext(ctx, "wl-paste", "--no-newline", "--type", mime)
		case commandAvailable("xclip"):
			cmd = exec.CommandContext(ctx, "xclip", "-o", "-selection", "clipboard", "-t", mime)
		default:
			cancel()
			return nil, errors.New("file clipboard requires wl-paste or xclip")
		}
		out, err := cmd.Output()
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		if paths := parseFileClipboardURIs(string(out)); len(paths) > 0 {
			return paths, nil
		}
	}
	return nil, lastErr
}

func WriteClipboardFiles(paths []string) error {
	paths = normalizeClipboardFilePaths(paths)
	if len(paths) == 0 {
		return errors.New("no local files to copy")
	}
	uris := make([]string, 0, len(paths))
	for _, path := range paths {
		uris = append(uris, fileClipboardURI(path))
	}
	mime := uriListMIME
	payload := strings.Join(uris, "\r\n") + "\r\n"
	if desktop := linuxDesktopSignature(); strings.Contains(desktop, "gnome") ||
		strings.Contains(desktop, "ubuntu") ||
		strings.Contains(desktop, "unity") ||
		strings.Contains(desktop, "cinnamon") ||
		strings.Contains(desktop, "pantheon") {
		mime = gnomeCopiedFilesMIME
		payload = "copy\n" + strings.Join(uris, "\n") + "\n"
	}

	var cmd *exec.Cmd
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	switch {
	case commandAvailable("wl-copy"):
		cmd = exec.CommandContext(ctx, "wl-copy", "--type", mime)
	case commandAvailable("xclip"):
		cmd = exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-t", mime)
	default:
		return errors.New("file clipboard requires wl-copy or xclip")
	}
	cmd.Stdin = bytes.NewBufferString(payload)
	if out, err := cmd.CombinedOutput(); err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("write file clipboard: %s: %w", detail, err)
		}
		return fmt.Errorf("write file clipboard: %w", err)
	}
	return nil
}

func commandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
