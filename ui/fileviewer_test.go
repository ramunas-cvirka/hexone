// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"hexone/fm"
)

func encodeViewerUTF16Test(text string, order binary.ByteOrder) []byte {
	units := utf16.Encode([]rune(text))
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		order.PutUint16(data[i*2:], unit)
	}
	return data
}

func TestDecodeViewerTextAutoDetectsUTF16BEBOM(t *testing.T) {
	data := []byte{0xFE, 0xFF, 0x00, 'A', 0x00, 'B'}

	got, info := decodeViewerText("sample.txt", data, fm.ViewerFileEncodingAuto)

	if got != "AB" {
		t.Fatalf("decodeViewerText=%q want %q", got, "AB")
	}
	if info.encoding != fm.ViewerFileEncodingUTF16BE {
		t.Fatalf("encoding=%q want %q", info.encoding, fm.ViewerFileEncodingUTF16BE)
	}
	if !info.encodingBOM {
		t.Fatal("expected UTF-16BE BOM to be reported")
	}
}

func TestChooseViewerEncodingBOMOverridesManualCP437(t *testing.T) {
	data := []byte{0xFF, 0xFE, 'A', 0x00, 'B', 0x00}

	decision := chooseViewerEncoding("sample.bin", data, fm.ViewerFileEncodingCP437)

	if decision.encoding != fm.ViewerFileEncodingUTF16LE {
		t.Fatalf("encoding=%q want %q", decision.encoding, fm.ViewerFileEncodingUTF16LE)
	}
	if !decision.withBOM {
		t.Fatal("expected BOM to be reported")
	}
}

func TestReadViewerFileAutoDetectsUTF16LENormalizesCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "utf16le.txt")
	data := []byte{
		0xFF, 0xFE,
		'a', 0x00,
		'\r', 0x00,
		'\n', 0x00,
		'b', 0x00,
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	content, _, errText, info := readViewerFile(path, fm.ViewerFileEncodingAuto, 1<<20, time.Time{}, nil)

	if errText != "" {
		t.Fatalf("readViewerFile err=%q", errText)
	}
	if content != "a\nb" {
		t.Fatalf("content=%q want %q", content, "a\nb")
	}
	if info.encoding != fm.ViewerFileEncodingUTF16LE {
		t.Fatalf("encoding=%q want %q", info.encoding, fm.ViewerFileEncodingUTF16LE)
	}
	if !info.encodingBOM {
		t.Fatal("expected UTF-16LE BOM to be reported")
	}
	if info.lineEnding != viewerLineEndingCRLF {
		t.Fatalf("lineEnding=%q want %q", info.lineEnding, viewerLineEndingCRLF)
	}
}

func TestDetectViewerUTF16EncodingHeuristic(t *testing.T) {
	data := []byte{
		'a', 0x00, 'b', 0x00, 'c', 0x00, 'd', 0x00,
		'e', 0x00, 'f', 0x00, 'g', 0x00, '\n', 0x00,
	}

	got := detectViewerUTF16Encoding(data)

	if got != fm.ViewerFileEncodingUTF16LE {
		t.Fatalf("detectViewerUTF16Encoding=%q want %q", got, fm.ViewerFileEncodingUTF16LE)
	}
}

func TestDetectViewerUTF16EncodingShortPrefix(t *testing.T) {
	data := []byte{'A', 0x00, 'B', 0x00}

	got := detectViewerUTF16Encoding(data)

	if got != fm.ViewerFileEncodingUTF16LE {
		t.Fatalf("detectViewerUTF16Encoding=%q want %q", got, fm.ViewerFileEncodingUTF16LE)
	}
}

func TestDetectViewerUTF16EncodingFallsBackWhenUnsure(t *testing.T) {
	data := []byte{0x00, 'a', 0x00, 0x01, 'b', 0x00, 0x02, 'c', 0x00, 0x03, 'd', 0x00, 0x04, 'e', 0x00, 0x05}

	got := detectViewerUTF16Encoding(data)

	if got != "" {
		t.Fatalf("detectViewerUTF16Encoding=%q want empty", got)
	}
}

func TestChooseViewerEncodingDetectsUTF16LEWithoutBOM(t *testing.T) {
	data := []byte{
		'[', 0x00, 'S', 0x00, 'e', 0x00, 'c', 0x00, 't', 0x00, 'i', 0x00, 'o', 0x00, 'n', 0x00,
		']', 0x00, '\r', 0x00, '\n', 0x00, 'K', 0x00, 'e', 0x00, 'y', 0x00, '=', 0x00, 'V', 0x00,
		'a', 0x00, 'l', 0x00, 'u', 0x00, 'e', 0x00,
	}

	decision := chooseViewerEncoding("config.bin", data, fm.ViewerFileEncodingAuto)

	if decision.encoding != fm.ViewerFileEncodingUTF16LE {
		t.Fatalf("encoding=%q want %q", decision.encoding, fm.ViewerFileEncodingUTF16LE)
	}
}

func TestChooseViewerEncodingDetectsUTF16BENonASCIIWithoutBOM(t *testing.T) {
	data := encodeViewerUTF16Test("Žąžuolas\r\n", binary.BigEndian)

	decision := chooseViewerEncoding("sample.bin", data, fm.ViewerFileEncodingAuto)

	if decision.encoding != fm.ViewerFileEncodingUTF16BE {
		t.Fatalf("encoding=%q want %q", decision.encoding, fm.ViewerFileEncodingUTF16BE)
	}
}

func TestChooseViewerEncodingDetectsUTF16LEWithSparseZeroPattern(t *testing.T) {
	data := encodeViewerUTF16Test("Žąaėbįc\r\n", binary.LittleEndian)

	decision := chooseViewerEncoding("sample.bin", data, fm.ViewerFileEncodingAuto)

	if decision.encoding != fm.ViewerFileEncodingUTF16LE {
		t.Fatalf("encoding=%q want %q", decision.encoding, fm.ViewerFileEncodingUTF16LE)
	}
}

func TestDecodeViewerTextAutoDetectsCP437Art(t *testing.T) {
	data := []byte{0xDA, 0xC4, 0xBF, '\r', '\n', 0xC0, 0xC4, 0xD9}

	got, info := decodeViewerText("sample.bin", data, fm.ViewerFileEncodingAuto)

	if got != "┌─┐\r\n└─┘" {
		t.Fatalf("decodeViewerText=%q want %q", got, "┌─┐\r\n└─┘")
	}
	if info.encoding != fm.ViewerFileEncodingCP437 {
		t.Fatalf("encoding=%q want %q", info.encoding, fm.ViewerFileEncodingCP437)
	}
	if info.encodingBOM {
		t.Fatal("cp437 detection should not mark BOM")
	}
}

func TestDecodeViewerTextAutoDetectsCP437ByContent(t *testing.T) {
	data := []byte{
		0xDA, 0xC4, 0xBF, 0x20, 0xDA, 0xC4, 0xBF, '\r', '\n',
		0xB3, 0x20, 0xB3, 0x20, 0xB3, 0x20, 0xB3, '\r', '\n',
		0xC0, 0xC4, 0xD9, 0x20, 0xC0, 0xC4, 0xD9,
	}

	got, info := decodeViewerText("scene.txt", data, fm.ViewerFileEncodingAuto)

	if !strings.Contains(got, "┌") || !strings.Contains(got, "└") {
		t.Fatalf("decodeViewerText=%q want decoded box drawing", got)
	}
	if info.encoding != fm.ViewerFileEncodingCP437 {
		t.Fatalf("encoding=%q want %q", info.encoding, fm.ViewerFileEncodingCP437)
	}
}

func TestDetectViewerLineEndingMixed(t *testing.T) {
	got := detectViewerLineEnding("alpha\r\nbeta\ngamma")

	if got != viewerLineEndingMixed {
		t.Fatalf("detectViewerLineEnding=%q want %q", got, viewerLineEndingMixed)
	}
}

func TestViewerClipboardContentPreservesCRLFForFileMode(t *testing.T) {
	st := &fileViewerState{
		mode:               "file",
		detectedLineEnding: viewerLineEndingCRLF,
	}

	got := viewerClipboardContent(st, "alpha\nbeta")

	if got != "alpha\r\nbeta" {
		t.Fatalf("viewerClipboardContent=%q want %q", got, "alpha\r\nbeta")
	}
}

func TestViewerCommandUsesNoMatchExitRecognizesSearchTools(t *testing.T) {
	tests := []string{
		`grep needle {path}`,
		`rg needle {path}`,
		`findstr needle {path}`,
		`git grep needle`,
	}

	for _, cmdline := range tests {
		if !viewerCommandUsesNoMatchExit(cmdline) {
			t.Fatalf("viewerCommandUsesNoMatchExit(%q)=false want true", cmdline)
		}
	}
}

func TestViewerCommandUsesNoMatchExitRejectsNonSearchTools(t *testing.T) {
	tests := []string{
		`cat {path}`,
		`pgrep hexone`,
		`git status`,
	}

	for _, cmdline := range tests {
		if viewerCommandUsesNoMatchExit(cmdline) {
			t.Fatalf("viewerCommandUsesNoMatchExit(%q)=true want false", cmdline)
		}
	}
}

func TestViewerCommandTokenNameNormalizesExeAndPaths(t *testing.T) {
	tests := map[string]string{
		`C:\Tools\rg.exe`: `rg`,
		`/usr/bin/grep`:   `grep`,
		`"findstr.exe"`:   `findstr`,
	}

	for token, want := range tests {
		if got := viewerCommandTokenName(token); got != want {
			t.Fatalf("viewerCommandTokenName(%q)=%q want %q", token, got, want)
		}
	}
}

func TestFileViewerEmptyPanelMessageUsesNoOutputForSettledEmptyCommand(t *testing.T) {
	st := &fileViewerState{
		mode:      "command",
		content:   "",
		err:       "",
		updatedAt: time.Now(),
	}

	if got := fileViewerEmptyPanelMessage(st); got != "No output" {
		t.Fatalf("fileViewerEmptyPanelMessage=%q want %q", got, "No output")
	}
}

func TestFileViewerEmptyPanelMessageKeepsNoOutputDuringRefresh(t *testing.T) {
	st := &fileViewerState{
		mode:      "command",
		content:   "",
		err:       "",
		loading:   true,
		updatedAt: time.Now(),
	}

	if got := fileViewerEmptyPanelMessage(st); got != "No output" {
		t.Fatalf("fileViewerEmptyPanelMessage=%q want %q", got, "No output")
	}
}

func TestFileViewerEmptyPanelMessageKeepsLoadingForInitialEmptyLoad(t *testing.T) {
	st := &fileViewerState{
		mode:    "command",
		content: "",
		err:     "",
		loading: true,
	}

	if got := fileViewerEmptyPanelMessage(st); got != "Loading..." {
		t.Fatalf("fileViewerEmptyPanelMessage=%q want %q", got, "Loading...")
	}
}
