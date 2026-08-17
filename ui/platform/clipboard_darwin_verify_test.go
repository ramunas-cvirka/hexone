// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build darwin && clipboardverify

package platform

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// TestDarwinClipboardRoundTrip is an opt-in system pasteboard check. It leaves
// the matching lines on the clipboard so a manual paste can verify the result.
func TestDarwinClipboardRoundTrip(t *testing.T) {
	path := os.Getenv("CLIPBOARD_VERIFY_FILE")
	if path == "" {
		t.Skip("CLIPBOARD_VERIFY_FILE is not set")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	lines := make([]string, 0, 3)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() && len(lines) < cap(lines) {
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), "failed") {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(lines) != cap(lines) {
		t.Fatalf("found %d failed lines, want %d", len(lines), cap(lines))
	}
	want := strings.Join(lines, "\n")
	if err := WriteClipboardTextNow(want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadClipboardTextNow()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("clipboard round trip=%q want %q", got, want)
	}
}
