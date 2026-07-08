// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf16"

	"gioui.org/layout"
	"gioui.org/op"
	"hexone/filesys"
	"hexone/fm"
)

func TestStartFileViewerBrokenSymlinkShowsPaneNotice(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := &UI{
		fmCfg: fm.DefaultConfig(),
		filePanes: []*filePaneState{
			newFilePaneState("/", cfg),
		},
	}
	pane := ui.filePanes[0]
	pane.applyListing(filesys.Listing{
		Dir: "/",
		Entries: []filesys.Entry{{
			Name:        ".VolumeIcon.icns",
			DisplayName: ".VolumeIcon.icns",
			Path:        "/.VolumeIcon.icns",
			Kind:        filesys.EntryBroken,
			IsSymlink:   true,
			LinkTarget:  "System/Volumes/Data/.VolumeIcon.icns",
		}},
	}, "", "", 0)

	ui.startFileViewer(0, time.Now())

	if ui.fileViewer != nil {
		t.Fatal("broken symlink should not open the viewer")
	}
	if got, want := pane.noticeText, "link target does not exist: System/Volumes/Data/.VolumeIcon.icns"; got != want {
		t.Fatalf("notice = %q, want %q", got, want)
	}
}

func TestStartFileViewerPermissionDeniedShowsPaneNotice(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bit behavior is platform-specific on Windows")
	}

	root := t.TempDir()
	target := filepath.Join(root, "locked.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write locked file: %v", err)
	}
	if err := os.Chmod(target, 0); err != nil {
		t.Fatalf("chmod locked file: %v", err)
	}
	defer os.Chmod(target, 0o600)
	if file, err := os.Open(target); err == nil {
		_ = file.Close()
		t.Skip("current user can still open mode-000 files")
	}

	cfg := fm.DefaultConfig()
	ui := &UI{
		fmCfg: fm.DefaultConfig(),
		filePanes: []*filePaneState{
			newFilePaneState(root, cfg),
		},
	}
	pane := ui.filePanes[0]
	pane.applyListing(filesys.Listing{
		Dir: root,
		Entries: []filesys.Entry{{
			Name:        "locked.txt",
			DisplayName: "locked.txt",
			Path:        target,
			Kind:        filesys.EntryFile,
		}},
	}, "", "", 0)

	ui.startFileViewer(0, time.Now())

	if ui.fileViewer != nil {
		t.Fatal("permission denied file should not open the viewer")
	}
	if got := pane.noticeText; !strings.Contains(got, "permission denied") || !strings.Contains(got, target) {
		t.Fatalf("notice = %q, want permission denied with path %q", got, target)
	}
}

func TestStartFileViewerNamedPipeShowsPaneNotice(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mkfifo is unsupported on Windows")
	}

	root := t.TempDir()
	target := filepath.Join(root, "pipe")
	if err := mkfifoForTest(target, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}

	cfg := fm.DefaultConfig()
	ui := &UI{
		fmCfg: fm.DefaultConfig(),
		filePanes: []*filePaneState{
			newFilePaneState(root, cfg),
		},
	}
	pane := ui.filePanes[0]
	pane.applyListing(filesys.Listing{
		Dir: root,
		Entries: []filesys.Entry{{
			Name:        "pipe",
			DisplayName: "pipe",
			Path:        target,
			Kind:        filesys.EntryFile,
		}},
	}, "", "", 0)

	ui.startFileViewer(0, time.Now())

	if ui.fileViewer != nil {
		t.Fatal("named pipe should not open the viewer")
	}
	if got := pane.noticeText; !strings.Contains(got, "viewer supports regular files only") || !strings.Contains(got, "named pipe") || !strings.Contains(got, target) {
		t.Fatalf("notice = %q, want regular-file notice for named pipe %q", got, target)
	}
}

type fakeViewerPDFRenderer struct {
	available bool
	requests  []viewerPDFRenderRequest
	result    viewerPDFRenderResult
	docInfo   viewerPDFDocInfo
	pageText  viewerPDFPageText
	err       error
}

func (f *fakeViewerPDFRenderer) Available() bool {
	return f != nil && f.available
}

func (f *fakeViewerPDFRenderer) RenderPage(req viewerPDFRenderRequest) (viewerPDFRenderResult, error) {
	f.requests = append(f.requests, req)
	return f.result, f.err
}

func (f *fakeViewerPDFRenderer) DocInfo(_ viewerPDFRenderRequest) (viewerPDFDocInfo, error) {
	if f.err != nil {
		return viewerPDFDocInfo{}, f.err
	}
	info := f.docInfo
	if info.PageCount == 0 {
		info.PageCount = f.result.PageCount
	}
	if len(info.PageSizes) == 0 && info.PageCount > 0 {
		info.PageSizes = make([]viewerPDFPageSize, info.PageCount)
		for i := range info.PageSizes {
			info.PageSizes[i] = viewerPDFPageSize{W: 612, H: 792}
		}
	}
	return info, nil
}

func (f *fakeViewerPDFRenderer) PageText(req viewerPDFRenderRequest) (viewerPDFPageText, error) {
	if f.err != nil {
		return viewerPDFPageText{}, f.err
	}
	text := f.pageText
	text.Page = req.Page
	return text, nil
}

func testViewerPreviewImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.NRGBA{R: 0xCC, G: 0x33, B: 0x22, A: 0xFF})
	img.Set(1, 0, color.NRGBA{R: 0x22, G: 0x99, B: 0x44, A: 0xFF})
	img.Set(2, 0, color.NRGBA{R: 0x11, G: 0x55, B: 0xCC, A: 0xFF})
	img.Set(0, 1, color.NRGBA{R: 0xF0, G: 0xE0, B: 0x60, A: 0xFF})
	img.Set(1, 1, color.NRGBA{R: 0x44, G: 0x44, B: 0x44, A: 0xFF})
	img.Set(2, 1, color.NRGBA{R: 0xEE, G: 0xEE, B: 0xEE, A: 0xFF})
	return img
}

func encodeViewerPreviewPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, testViewerPreviewImage()); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func encodeViewerPreviewJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, testViewerPreviewImage(), &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	return buf.Bytes()
}

func encodeViewerPreviewGIF(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gif.Encode(&buf, testViewerPreviewImage(), nil); err != nil {
		t.Fatalf("gif.Encode: %v", err)
	}
	return buf.Bytes()
}

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

func TestDecodeViewerTextAutoUsesBinaryPreviewForBinaryData(t *testing.T) {
	data := []byte{'A', 0x00, 'B', 0x01, 'C', 0x7F, 'D'}

	got, info := decodeViewerText("sample.bin", data, fm.ViewerFileEncodingAuto)

	if got != "A.B.C.D" {
		t.Fatalf("decodeViewerText=%q want %q", got, "A.B.C.D")
	}
	if strings.Contains(got, `\x`) {
		t.Fatalf("decodeViewerText=%q should not contain escaped bytes", got)
	}
	if !info.binaryPreview {
		t.Fatal("expected binary preview for NUL-heavy data")
	}
}

func TestFormatViewerBinaryPreviewWrapsFixedRows(t *testing.T) {
	data := bytes.Repeat([]byte{'A'}, viewerBinaryPreviewBytes+1)

	got := formatViewerBinaryPreview(data)
	want := strings.Repeat("A", viewerBinaryPreviewBytes) + "\nA"

	if got != want {
		t.Fatalf("formatViewerBinaryPreview=%q want %q", got, want)
	}
}

func TestFormatViewerBinaryPreviewWithColsUsesViewportWidth(t *testing.T) {
	data := bytes.Repeat([]byte{'A'}, 11)

	got := formatViewerBinaryPreviewWithCols(data, 5)
	want := "AAAAA\nAAAAA\nA"

	if got != want {
		t.Fatalf("formatViewerBinaryPreviewWithCols=%q want %q", got, want)
	}
}

func TestViewerBinaryPreviewWrapColsCapsWideViewports(t *testing.T) {
	if got := viewerBinaryPreviewWrapCols(400); got != viewerBinaryPreviewBytes {
		t.Fatalf("viewerBinaryPreviewWrapCols=%d want %d", got, viewerBinaryPreviewBytes)
	}
}

func TestDecodeViewerTextAutoUsesBinaryPreviewForPDFData(t *testing.T) {
	data := []byte("%PDF-1.7\r\n1 0 obj\r\n<< /Type /Catalog >>\r\nendobj\r\n")

	got, info := decodeViewerText("sample.pdf", data, fm.ViewerFileEncodingAuto)

	if !info.binaryPreview {
		t.Fatal("expected PDF data to use binary preview")
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("decodeViewerText=%q should not preserve source newlines in binary preview", got)
	}
}

func TestDecodeViewerTextManualUTF8StillUsesBinaryPreviewForPDFData(t *testing.T) {
	data := []byte("%PDF-1.7\r\n1 0 obj\r\n<< /Type /Catalog >>\r\nendobj\r\n")

	_, info := decodeViewerText("sample.pdf", data, fm.ViewerFileEncodingUTF8)

	if !info.binaryPreview {
		t.Fatal("expected manual UTF-8 PDF view to still use binary preview")
	}
}

func TestReadViewerFileAutoUsesPDFPreviewWhenRendererAvailable(t *testing.T) {
	prev := viewerPDFPreviewBackend
	fake := &fakeViewerPDFRenderer{
		available: true,
		result: viewerPDFRenderResult{
			Image:     image.NewNRGBA(image.Rect(0, 0, 120, 180)),
			Page:      0,
			PageCount: 4,
			Size:      image.Pt(120, 180),
		},
	}
	viewerPDFPreviewBackend = fake
	t.Cleanup(func() {
		viewerPDFPreviewBackend = prev
	})

	path := filepath.Join(t.TempDir(), "sample.pdf")
	data := []byte("%PDF-1.7\r\n1 0 obj\r\n<< /Type /Catalog >>\r\nendobj\r\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	content, _, errText, info := readViewerFile(path, fm.ViewerFileEncodingAuto, 1<<20, time.Time{}, nil)

	if errText != "" {
		t.Fatalf("readViewerFile err=%q", errText)
	}
	if content != "" {
		t.Fatalf("content=%q want empty pdf preview payload", content)
	}
	if !info.imagePreview {
		t.Fatal("expected PDF preview info")
	}
	if info.imageFormat != "pdf" {
		t.Fatalf("imageFormat=%q want %q", info.imageFormat, "pdf")
	}
	if info.imageSize != image.Pt(120, 180) {
		t.Fatalf("imageSize=%v want %v", info.imageSize, image.Pt(120, 180))
	}
	if info.imagePage != 0 || info.imagePageCount != 4 {
		t.Fatalf("page metadata=(%d,%d) want (0,4)", info.imagePage, info.imagePageCount)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("render requests=%d want 1", len(fake.requests))
	}
	if fake.requests[0].Page != 0 {
		t.Fatalf("rendered page=%d want 0", fake.requests[0].Page)
	}
	if fake.requests[0].Width != viewerPDFPreviewTargetWidthPx {
		t.Fatalf("render width=%d want %d", fake.requests[0].Width, viewerPDFPreviewTargetWidthPx)
	}
}

func TestReadViewerFileLocalPDFBypassesSizeLimitUsingPath(t *testing.T) {
	prev := viewerPDFPreviewBackend
	fake := &fakeViewerPDFRenderer{
		available: true,
		result: viewerPDFRenderResult{
			Image:     image.NewNRGBA(image.Rect(0, 0, 120, 160)),
			Page:      0,
			PageCount: 2,
			Size:      image.Pt(120, 160),
		},
	}
	viewerPDFPreviewBackend = fake
	t.Cleanup(func() {
		viewerPDFPreviewBackend = prev
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "large.pdf")
	if err := os.WriteFile(path, bytes.Repeat([]byte{0}, 2<<20), 0o600); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	content, status, errText, info := readViewerFile(path, fm.ViewerFileEncodingAuto, 1<<20, time.Time{}, nil)
	if errText != "" {
		t.Fatalf("readViewerFile err=%q", errText)
	}
	if content != "" {
		t.Fatalf("content=%q want empty pdf preview payload", content)
	}
	if status == "" {
		t.Fatal("expected file size status")
	}
	if !info.imagePreview {
		t.Fatal("expected PDF preview info")
	}
	if len(fake.requests) != 1 {
		t.Fatalf("render requests=%d want 1", len(fake.requests))
	}
	if fake.requests[0].LocalPath != "" {
		t.Fatalf("expected empty LocalPath, got %q", fake.requests[0].LocalPath)
	}
	if len(fake.requests[0].Data) <= 1<<20 {
		t.Fatalf("expected full pdf bytes beyond size limit, got %d", len(fake.requests[0].Data))
	}
}

func TestSanitizeViewerContentReplacesControlRunesWithDots(t *testing.T) {
	raw := "A\tB" + string([]rune{0x07, unicode.ReplacementChar}) + "C"

	got := sanitizeViewerContent(raw)

	if got != "A    B..C" {
		t.Fatalf("sanitizeViewerContent=%q want %q", got, "A    B..C")
	}
	if strings.Contains(got, `\x`) || strings.Contains(got, `\u`) {
		t.Fatalf("sanitizeViewerContent=%q should not contain escaped controls", got)
	}
}

func TestReadViewerFileAutoUsesBinaryPreview(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.bin")
	data := append(bytes.Repeat([]byte{'A'}, viewerBinaryPreviewBytes), 0x00)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	content, _, errText, info := readViewerFile(path, fm.ViewerFileEncodingAuto, 1<<20, time.Time{}, nil)

	if errText != "" {
		t.Fatalf("readViewerFile err=%q", errText)
	}
	want := strings.Repeat("A", viewerBinaryPreviewBytes) + "\n."
	if content != want {
		t.Fatalf("content=%q want %q", content, want)
	}
	if !info.binaryPreview {
		t.Fatal("expected binary preview info")
	}
	if info.lineEnding != "" {
		t.Fatalf("lineEnding=%q want empty for binary preview", info.lineEnding)
	}
}

func TestReadViewerFileAutoUsesPNGImagePreview(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.png")
	data := encodeViewerPreviewPNG(t)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	content, _, errText, info := readViewerFile(path, fm.ViewerFileEncodingAuto, 1<<20, time.Time{}, nil)

	if errText != "" {
		t.Fatalf("readViewerFile err=%q", errText)
	}
	if content != "" {
		t.Fatalf("content=%q want empty image payload", content)
	}
	if !info.imagePreview {
		t.Fatal("expected PNG image preview info")
	}
	if info.image == nil {
		t.Fatal("expected decoded PNG image")
	}
	if info.imageFormat != "png" {
		t.Fatalf("imageFormat=%q want %q", info.imageFormat, "png")
	}
	if info.imageSize != image.Pt(3, 2) {
		t.Fatalf("imageSize=%v want %v", info.imageSize, image.Pt(3, 2))
	}
	if info.binaryPreview {
		t.Fatal("image preview should not be marked as binary preview")
	}
}

func TestDecodeViewerImagePreviewUsesJPEG(t *testing.T) {
	info, ok := decodeViewerImagePreview("sample.jpg", encodeViewerPreviewJPEG(t))

	if !ok {
		t.Fatal("expected JPEG preview decode")
	}
	if !info.imagePreview {
		t.Fatal("expected JPEG image preview info")
	}
	if info.image == nil {
		t.Fatal("expected decoded JPEG image")
	}
	if info.imageFormat != "jpeg" {
		t.Fatalf("imageFormat=%q want %q", info.imageFormat, "jpeg")
	}
	if info.imageSize != image.Pt(3, 2) {
		t.Fatalf("imageSize=%v want %v", info.imageSize, image.Pt(3, 2))
	}
}

func TestDecodeViewerImagePreviewUsesGIF(t *testing.T) {
	info, ok := decodeViewerImagePreview("sample.gif", encodeViewerPreviewGIF(t))

	if !ok {
		t.Fatal("expected GIF preview decode")
	}
	if !info.imagePreview {
		t.Fatal("expected GIF image preview info")
	}
	if info.image == nil {
		t.Fatal("expected decoded GIF image")
	}
	if info.imageFormat != "gif" {
		t.Fatalf("imageFormat=%q want %q", info.imageFormat, "gif")
	}
	if info.imageSize != image.Pt(3, 2) {
		t.Fatalf("imageSize=%v want %v", info.imageSize, image.Pt(3, 2))
	}
}

func TestReflowFileViewerBinaryPreviewUsesViewportCols(t *testing.T) {
	data := bytes.Repeat([]byte{'A'}, 11)
	st := &fileViewerState{
		content:               formatViewerBinaryPreview(data),
		detectedBinaryPreview: true,
		binaryPreviewData:     append([]byte(nil), data...),
		binaryPreviewCols:     viewerBinaryPreviewBytes,
	}

	if !reflowFileViewerBinaryPreview(st, 5) {
		t.Fatal("expected binary preview reflow")
	}
	if got := st.content; got != "AAAAA\nAAAAA\nA" {
		t.Fatalf("reflowed content=%q want %q", got, "AAAAA\nAAAAA\nA")
	}
	if got := st.binaryPreviewCols; got != 5 {
		t.Fatalf("binaryPreviewCols=%d want %d", got, 5)
	}
}

func TestViewerUpdateActionTreatsSameBinaryPreviewBytesAsSame(t *testing.T) {
	data := bytes.Repeat([]byte{'A'}, 11)
	st := &fileViewerState{
		content:               formatViewerBinaryPreviewWithCols(data, 5),
		detectedBinaryPreview: true,
		binaryPreviewData:     append([]byte(nil), data...),
		binaryPreviewCols:     5,
	}

	got := viewerUpdateAction(st, formatViewerBinaryPreview(data), false, nil, true, data)

	if got != viewerUpdateSame {
		t.Fatalf("viewerUpdateAction=%d want %d", got, viewerUpdateSame)
	}
}

func TestViewerUpdateActionTreatsSameImageBytesAsSame(t *testing.T) {
	data := encodeViewerPreviewPNG(t)
	st := &fileViewerState{
		detectedImagePreview: true,
		imagePreviewData:     append([]byte(nil), data...),
	}

	got := viewerUpdateAction(st, "", true, data, false, nil)

	if got != viewerUpdateSame {
		t.Fatalf("viewerUpdateAction=%d want %d", got, viewerUpdateSame)
	}
}

func TestEnsurePDFDocAssetsRendersVisiblePages(t *testing.T) {
	prev := viewerPDFPreviewBackend
	fake := &fakeViewerPDFRenderer{
		available: true,
		result: viewerPDFRenderResult{
			Image:     image.NewNRGBA(image.Rect(0, 0, 90, 140)),
			Page:      0,
			PageCount: 3,
			Size:      image.Pt(90, 140),
		},
	}
	viewerPDFPreviewBackend = fake
	t.Cleanup(func() {
		viewerPDFPreviewBackend = prev
	})

	ui := NewUI(fm.DefaultConfig())
	now := time.Date(2026, time.April, 11, 8, 0, 0, 0, time.UTC)
	st := &fileViewerState{
		detectedImagePreview:  true,
		imagePreviewFormat:    "pdf",
		imagePreviewData:      []byte("%PDF-1.7"),
		imagePreviewPage:      0,
		imagePreviewPageCount: 3,
		pdfDocCh:              make(chan pdfDocResult, 16),
		seq:                   7,
	}
	sizes := make([]viewerPDFPageSize, 3)
	for i := range sizes {
		sizes[i] = viewerPDFPageSize{W: 612, H: 792}
	}
	st.pdfDoc.viewportRect = image.Rect(0, 0, 160, 120)
	st.pdfDoc.configure(viewerPDFDocInfo{PageCount: 3, PageSizes: sizes})
	ui.fileViewer = st

	gtx := layout.Context{Ops: new(op.Ops), Now: now}
	ui.ensurePDFDocAssets(gtx, st)

	if len(st.pdfDoc.renderPending) == 0 {
		t.Fatal("expected page renders to be requested for visible pages")
	}
	deadline := time.After(2 * time.Second)
	for {
		if _, ok := st.pdfDoc.pages[0]; ok {
			break
		}
		select {
		case res := <-st.pdfDocCh:
			if res.seq != st.seq {
				t.Fatalf("seq=%d want %d", res.seq, st.seq)
			}
			if res.render != nil {
				st.pdfDoc.storeRender(res.page, *res.render)
				delete(st.pdfDoc.renderPending, res.page)
			}
		case <-deadline:
			t.Fatal("timed out waiting for visible page 0 render result")
		}
	}
	found := false
	for _, req := range fake.requests {
		if req.Page == 0 {
			found = true
			if req.Width != st.pdfDoc.renderWidthFor(0) {
				t.Fatalf("render width=%d want %d", req.Width, st.pdfDoc.renderWidthFor(0))
			}
		}
	}
	if !found {
		t.Fatal("expected a render request for page 0")
	}
}

func TestPDFDocConfigurePreservesReadingPosition(t *testing.T) {
	var v pdfDocView
	v.viewportRect = image.Rect(0, 0, 200, 300)
	fallback := make([]viewerPDFPageSize, 4)
	for i := range fallback {
		fallback[i] = viewerPDFPageSize{W: 612, H: 792}
	}
	v.configure(viewerPDFDocInfo{PageCount: 4, PageSizes: fallback})
	// Move to page 2, halfway down.
	v.scrollY = v.layoutTops[2] + v.layoutHeights[2]/2
	v.clampScroll()

	real := make([]viewerPDFPageSize, 4)
	for i := range real {
		real[i] = viewerPDFPageSize{W: 300, H: 1200}
	}
	v.infoLoaded = false
	v.configure(viewerPDFDocInfo{PageCount: 4, PageSizes: real})

	page, frac := v.readingPosition()
	if page != 2 {
		t.Fatalf("page=%d want 2 after configure", page)
	}
	if frac < 0.4 || frac > 0.6 {
		t.Fatalf("frac=%f want ~0.5 after configure", frac)
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

func TestFileViewerRestoresSeparateStreamSelectionsPerMode(t *testing.T) {
	st := &fileViewerState{mode: "file"}
	st.stream.SetContent("alpha\nbeta")
	st.stream.beginSelection(1)
	st.stream.updateSelection(4)
	fileStart, fileLen := st.stream.selStart, st.stream.selLen

	st.prepareStreamSelectionForMode("command")
	if st.stream.selActive {
		t.Fatal("command mode should not inherit file selection")
	}

	st.mode = "command"
	st.stream.SetContent("cmd output")
	st.stream.beginSelection(2)
	st.stream.updateSelection(6)
	cmdStart, cmdLen := st.stream.selStart, st.stream.selLen

	st.prepareStreamSelectionForMode("file")
	st.mode = "file"
	st.stream.SetContent("alpha\nbeta")
	st.restorePendingStreamSelection()
	if !st.stream.selActive || st.stream.selStart != fileStart || st.stream.selLen != fileLen {
		t.Fatalf("file selection restored as active=%v start=%d len=%d, want active start=%d len=%d",
			st.stream.selActive, st.stream.selStart, st.stream.selLen, fileStart, fileLen)
	}

	st.prepareStreamSelectionForMode("command")
	st.mode = "command"
	st.stream.SetContent("cmd output")
	st.restorePendingStreamSelection()
	if !st.stream.selActive || st.stream.selStart != cmdStart || st.stream.selLen != cmdLen {
		t.Fatalf("command selection restored as active=%v start=%d len=%d, want active start=%d len=%d",
			st.stream.selActive, st.stream.selStart, st.stream.selLen, cmdStart, cmdLen)
	}
}

func TestFileViewerRestoringHexModeClearsSharedStreamSelection(t *testing.T) {
	st := &fileViewerState{mode: "file"}
	st.stream.SetContent("alpha\nbeta")
	st.stream.beginSelection(0)
	st.stream.updateSelection(5)
	st.rememberStreamSelection("file")

	st.prepareStreamSelectionForMode("hex")

	if st.stream.selActive || st.stream.selectingText || st.stream.autoScrollActive {
		t.Fatalf("hex mode should clear shared stream selection state, got active=%v selecting=%v auto=%v",
			st.stream.selActive, st.stream.selectingText, st.stream.autoScrollActive)
	}

	st.restoreStreamSelection("file")
	if !st.stream.selActive || st.stream.selStart != 0 || st.stream.selLen != 5 {
		t.Fatalf("file selection restored as active=%v start=%d len=%d, want active start=0 len=5",
			st.stream.selActive, st.stream.selStart, st.stream.selLen)
	}
}

func TestSetFileViewerModeClearsStaleErrorsAndPendingState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "small.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	ui := NewUI(fm.DefaultConfig())
	ui.configPath = filepath.Join(t.TempDir(), "hexone-test.yaml")
	st := &fileViewerState{
		mode:               "file",
		path:               path,
		command:            "cat {path}",
		err:                "file too large: 1.8 MB > 1.0 MB limit",
		status:             "file: 1887437 bytes",
		pendingUpdate:      true,
		pendingContent:     "stale",
		pendingStatus:      "file: 1887437 bytes",
		pendingErr:         "file too large: 1.8 MB > 1.0 MB limit",
		pendingSyntaxReady: true,
		resultCh:           make(chan fileViewerResult, 4),
	}
	ui.fileViewer = st

	ui.setFileViewerMode("command", time.Now())

	if st.err != "" {
		t.Fatalf("stale err should clear on mode switch, got %q", st.err)
	}
	if st.status == "file: 1887437 bytes" {
		t.Fatalf("stale status should clear on mode switch, got %q", st.status)
	}
	if st.pendingUpdate {
		t.Fatal("pending update should clear on mode switch")
	}
	if st.pendingContent != "" || st.pendingErr != "" || st.pendingStatus != "" {
		t.Fatalf("pending state should clear on mode switch, got content=%q err=%q status=%q",
			st.pendingContent, st.pendingErr, st.pendingStatus)
	}
	if st.pendingSyntaxReady {
		t.Fatal("pending syntax should clear on mode switch")
	}
	if st.mode != "command" {
		t.Fatalf("mode=%q want command", st.mode)
	}
}

func TestSetFileViewerModeLeavesCommandOnlyViewerInCommandMode(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.fileViewer = &fileViewerState{
		mode:        "command",
		command:     "uptime",
		commandOnly: true,
	}

	ui.setFileViewerMode("file", time.Now())

	if ui.fileViewer.mode != "command" {
		t.Fatalf("mode=%q want command", ui.fileViewer.mode)
	}
	if ui.fileViewer.command != "uptime" {
		t.Fatalf("command=%q want uptime", ui.fileViewer.command)
	}
}

func TestViewerLocalCommandWorkingDirUsesDirectoryTargets(t *testing.T) {
	dir := t.TempDir()
	if got := viewerLocalCommandWorkingDir(dir); got != filepath.Clean(dir) {
		t.Fatalf("working dir=%q want %q", got, filepath.Clean(dir))
	}

	path := filepath.Join(dir, "log.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
	if got := viewerLocalCommandWorkingDir(path); got != filepath.Clean(dir) {
		t.Fatalf("file working dir=%q want %q", got, filepath.Clean(dir))
	}
}

func TestViewerEncodingStatusLabelUsesBinaryPreviewLabel(t *testing.T) {
	st := &fileViewerState{
		fileEncoding:          fm.ViewerFileEncodingAuto,
		detectedBinaryPreview: true,
	}

	if got := viewerEncodingStatusLabel(st); got != "Binary" {
		t.Fatalf("viewerEncodingStatusLabel=%q want %q", got, "Binary")
	}
}

func TestViewerEncodingStatusLabelUsesImageFormatLabel(t *testing.T) {
	st := &fileViewerState{
		detectedImagePreview: true,
		imagePreviewFormat:   "png",
	}

	if got := viewerEncodingStatusLabel(st); got != "PNG" {
		t.Fatalf("viewerEncodingStatusLabel=%q want %q", got, "PNG")
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

func TestViewerCommandBufferRollsStreamingOutputByLines(t *testing.T) {
	canceled := false
	buf := newViewerCommandBuffer(20, func() { canceled = true }, true)

	if _, err := buf.Write([]byte("line01\nline02\nline03\nline04\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out := string(buf.Bytes())

	if canceled {
		t.Fatal("rolling streaming buffer should not cancel the command at the size limit")
	}
	if len(out) > 20 {
		t.Fatalf("rolling output length=%d want <= 20", len(out))
	}
	if strings.Contains(out, "line01") || strings.Contains(out, "line02") {
		t.Fatalf("rolling output kept trimmed head: %q", out)
	}
	if !strings.Contains(out, "line03") || !strings.Contains(out, "line04") {
		t.Fatalf("rolling output should keep newest complete lines, got %q", out)
	}
	if !buf.Truncated() {
		t.Fatal("rolling buffer should report truncated history after dropping old lines")
	}
}

func TestViewerCommandBufferFiniteOutputCancelsAtLimit(t *testing.T) {
	canceled := false
	buf := newViewerCommandBuffer(8, func() { canceled = true }, false)

	if _, err := buf.Write([]byte("0123456789")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if !canceled {
		t.Fatal("finite command buffer should cancel when it reaches the size limit")
	}
	if got := string(buf.Bytes()); got != "01234567" {
		t.Fatalf("finite output=%q want capped prefix", got)
	}
	if !buf.Truncated() {
		t.Fatal("finite buffer should report truncation")
	}
}

func TestEmitViewerCommandProgressTracksRollingChangesAtSameLength(t *testing.T) {
	buf := newViewerCommandBuffer(12, nil, true)
	var sent []string
	progress := func(content, status string) {
		sent = append(sent, content+"|"+status)
	}

	if _, err := buf.Write([]byte("aa\nbb\ncc\n")); err != nil {
		t.Fatalf("Write first: %v", err)
	}
	version := emitViewerCommandProgress(progress, buf, "command", viewerShellSpec{}, time.Now(), true, true, viewerCommandUnsentVersion)
	if _, err := buf.Write([]byte("dd\nee\n")); err != nil {
		t.Fatalf("Write second: %v", err)
	}
	version = emitViewerCommandProgress(progress, buf, "command", viewerShellSpec{}, time.Now(), true, true, version)

	if len(sent) != 2 {
		t.Fatalf("progress sends=%d want 2 (%#v)", len(sent), sent)
	}
	if sent[0] == sent[1] {
		t.Fatalf("rolling progress did not change content: %#v", sent)
	}
	if strings.Contains(sent[1], "[truncated]") {
		t.Fatalf("rolling progress should use status, not an inline marker: %q", sent[1])
	}
	_ = version
}

func TestApplyFileViewerContentResultKeepsBottomAfterRollingTrim(t *testing.T) {
	st := &fileViewerState{
		mode:            "command",
		commandInfinite: true,
		content:         "a\nb\nc\n",
	}
	st.stream.SetContent(st.content)
	st.stream.visibleLines = 2
	st.stream.scrollToBottom()

	applyFileViewerContentResult(st, "b\nc\nd\n")

	if got, want := st.content, "b\nc\nd\n"; got != want {
		t.Fatalf("content=%q want %q", got, want)
	}
	if got, want := st.stream.topLine, st.stream.maxTopLine(); got != want {
		t.Fatalf("topLine=%d want bottom %d", got, want)
	}
}

func TestApplyFileViewerContentResultPreservesViewportAfterRollingTrim(t *testing.T) {
	st := &fileViewerState{
		mode:            "command",
		commandInfinite: true,
		content:         "a\nb\nc\nd\n",
	}
	st.stream.SetContent(st.content)
	st.stream.visibleLines = 2
	st.stream.topLine = 1
	st.stream.syncVisualTop()

	applyFileViewerContentResult(st, "b\nc\nd\ne\n")

	if got, want := st.stream.topLine, 0; got != want {
		t.Fatalf("topLine=%d want shifted %d", got, want)
	}
	if got, want := st.stream.lines[st.stream.topLine], "b"; got != want {
		t.Fatalf("top visible line=%q want %q", got, want)
	}
	if got, want := st.stream.visualTop, float32(0); got != want {
		t.Fatalf("visualTop=%v want %v", got, want)
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
