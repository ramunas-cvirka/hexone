// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import "testing"

func TestEncodeHexTextUsesCompactUppercaseBytes(t *testing.T) {
	got, n := encodeHexText("Hello")

	if got != "48656C6C6F" {
		t.Fatalf("encodeHexText=%q want %q", got, "48656C6C6F")
	}
	if n != 5 {
		t.Fatalf("byteCount=%d want 5", n)
	}
}

func TestOnRightTextChangedUpdatesLeftEditor(t *testing.T) {
	ui := NewUI(nil)

	ui.onRightTextChanged("Hi")

	if got := ui.LeftEd.Text(); got != "4869" {
		t.Fatalf("LeftEd=%q want %q", got, "4869")
	}
	if got := ui.LeftInfo; got != "2 bytes" {
		t.Fatalf("LeftInfo=%q want %q", got, "2 bytes")
	}
	if got := ui.leftPrev; got != "4869" {
		t.Fatalf("leftPrev=%q want %q", got, "4869")
	}
}

func TestOnRightTextChangedUpdatesByteCountWithoutReset(t *testing.T) {
	ui := NewUI(nil)
	ui.LeftEd.SetText("4869")

	ui.onRightTextChanged("Hi")

	if got := ui.LeftEd.Text(); got != "4869" {
		t.Fatalf("LeftEd=%q want unchanged %q", got, "4869")
	}
	if got := ui.LeftInfo; got != "2 bytes" {
		t.Fatalf("LeftInfo=%q want %q", got, "2 bytes")
	}
}

func TestDecodeHexIgnoresMultilineSeparators(t *testing.T) {
	got, n, err := decodeHex("48 65 6c 6c 6f\n57 6f 72 6c 64")
	if err != nil {
		t.Fatalf("decodeHex returned error: %v", err)
	}
	if got != "HelloWorld" {
		t.Fatalf("decodeHex=%q want %q", got, "HelloWorld")
	}
	if n != 10 {
		t.Fatalf("byteCount=%d want 10", n)
	}
}

func TestDecodeHexIgnoresEscapedNewlineMarkers(t *testing.T) {
	got, n, err := decodeHex(`48 65 6c 6c 6f \r\n 57 6f 72 6c 64`)
	if err != nil {
		t.Fatalf("decodeHex returned error: %v", err)
	}
	if got != "HelloWorld" {
		t.Fatalf("decodeHex=%q want %q", got, "HelloWorld")
	}
	if n != 10 {
		t.Fatalf("byteCount=%d want 10", n)
	}
}

func TestParseHexTextIgnoresNamedLineMarkers(t *testing.T) {
	got, err := parseHexText("48 65 LF 6C 6C CRLF 6F")
	if err != nil {
		t.Fatalf("parseHexText returned error: %v", err)
	}
	if string(got) != "Hello" {
		t.Fatalf("parsed=%q want %q", string(got), "Hello")
	}
}

func TestParseHexTextKeepsRealHexBytes(t *testing.T) {
	got, err := parseHexText("CF")
	if err != nil {
		t.Fatalf("parseHexText returned error: %v", err)
	}
	if len(got) != 1 || got[0] != 0xCF {
		t.Fatalf("parsed=%v want [207]", got)
	}
}
