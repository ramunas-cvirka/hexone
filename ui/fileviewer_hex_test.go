package ui

import "testing"

func TestNormalizeViewerModeSupportsHex(t *testing.T) {
	if got := normalizeViewerMode("hex"); got != "hex" {
		t.Fatalf("normalizeViewerMode(hex) = %q, want hex", got)
	}
	if got := normalizeViewerMode(" HEX "); got != "hex" {
		t.Fatalf("normalizeViewerMode(trimmed hex) = %q, want hex", got)
	}
}

func TestComputeHexBytesPerLineGrowsWithWidth(t *testing.T) {
	narrow := computeHexBytesPerLine(240, 8, 8, 0)
	wide := computeHexBytesPerLine(640, 8, 8, 0)
	if narrow < hexViewerMinBytesPerLine {
		t.Fatalf("narrow bytes/line = %d, want at least %d", narrow, hexViewerMinBytesPerLine)
	}
	if wide <= narrow {
		t.Fatalf("wide bytes/line = %d, want > narrow %d", wide, narrow)
	}
	if wide > hexViewerMaxBytesPerLine {
		t.Fatalf("wide bytes/line = %d, want <= %d", wide, hexViewerMaxBytesPerLine)
	}
}

func TestFormatHexLineAndTextLine(t *testing.T) {
	data := []byte{0x41, 0x00, 0x7A}
	if got, want := formatHexLine(data, 4, 0), "41 00 7A   "; got != want {
		t.Fatalf("formatHexLine = %q, want %q", got, want)
	}
	if got, want := formatHexTextLine(data, 4), "A.z "; got != want {
		t.Fatalf("formatHexTextLine = %q, want %q", got, want)
	}
}

func TestFormatHexLineWithGrouping(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04}
	if got, want := formatHexLine(data, 4, 2), "01 02  03 04"; got != want {
		t.Fatalf("formatHexLine = %q, want %q", got, want)
	}
}
