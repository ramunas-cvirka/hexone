// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build showcaseverify

package ui

import (
	resources "hexone"
	"hexone/filesys"
	"hexone/fm"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

const (
	showcaseWidth  = 1536
	showcaseHeight = 900
)

func TestHeadlessMacOSShowcase(t *testing.T) {
	outDir := os.Getenv("SHOWCASE_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := fm.DefaultConfig()
	th := showcaseTheme(t, cfg)

	hero := showcaseFileManagerUI(cfg)
	renderShowcase(t, th, hero, filepath.Join(outDir, "01-file-panes.png"))

	hexUI := showcaseHexEditorUI(cfg)
	renderShowcase(t, th, hexUI, filepath.Join(outDir, "02-hex-editor.png"))

	terminalUI := showcaseTerminalUI(t, cfg)
	renderShowcase(t, th, terminalUI, filepath.Join(outDir, "03-terminal-snippets.png"))

	analyzerUI := showcaseProtocolAnalyzerUI(cfg)
	renderShowcase(t, th, analyzerUI, filepath.Join(outDir, "04-protocol-analyzer.png"))

	clipboardUI := showcaseFileManagerUI(cfg)
	oldRead := readFileClipboardFilesFunc
	readFileClipboardFilesFunc = func() ([]string, error) {
		return []string{
			"/Users/ramunas/Desktop/release-notes.md",
			"/Users/ramunas/Desktop/hexone-demo.mov",
			"/Users/ramunas/Desktop/benchmarks.csv",
		}, nil
	}
	defer func() { readFileClipboardFilesFunc = oldRead }()
	pane := clipboardUI.filePanes[0]
	pane.replaceMarkedRows([]int{3, 4, 5})
	clipboardUI.openFilePaneContextMenu(0, 4, image.Pt(420, 300), time.Now().Add(-time.Second))
	renderShowcase(t, th, clipboardUI, filepath.Join(outDir, "05-file-clipboard.png"))
}

func showcaseTheme(t *testing.T, cfg *fm.Config) *material.Theme {
	t.Helper()
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(showcaseFontCollection(t)))
	th.Face = font.Typeface(cfg.General.Typeface)
	th.TextSize = unit.Sp(cfg.General.FontSizeSp)
	return th
}

func showcaseFontCollection(t *testing.T) []text.FontFace {
	t.Helper()
	collection := make([]text.FontFace, 0, len(resources.BundledFontFamilies())*2)
	for _, family := range resources.BundledFontFamilies() {
		for _, variant := range []struct {
			path   string
			weight font.Weight
		}{
			{path: family.RegularPath, weight: font.Normal},
			{path: family.BoldPath, weight: font.Bold},
		} {
			data, ok := resources.BundledFont(variant.path)
			if !ok {
				t.Fatalf("missing bundled font %s", variant.path)
			}
			face, err := opentype.Parse(data)
			if err != nil {
				t.Fatalf("parse bundled font %s: %v", variant.path, err)
			}
			collection = append(collection, text.FontFace{
				Font: font.Font{Typeface: font.Typeface(family.Name), Weight: variant.weight},
				Face: face,
			})
		}
	}
	return collection
}

func renderShowcase(t *testing.T, th *material.Theme, ui *UI, outPath string) {
	t.Helper()
	win, err := headless.NewWindow(showcaseWidth, showcaseHeight)
	if err != nil {
		t.Fatalf("headless window: %v", err)
	}
	defer win.Release()

	router := new(input.Router)
	base := time.Now()
	var img *image.RGBA
	for frame := 0; frame < 6; frame++ {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(showcaseWidth, showcaseHeight)),
			Now:         base.Add(time.Duration(frame) * 80 * time.Millisecond),
		}
		ui.Layout(th, gtx)
		router.Frame(&ops)
		if frame == 2 && ui.Tabs.Value == "tab2" {
			showcaseSelectAnalyzerSerial(ui)
		}
		if err := win.Frame(&ops); err != nil {
			t.Fatalf("render frame: %v", err)
		}
		img = image.NewRGBA(image.Rect(0, 0, showcaseWidth, showcaseHeight))
		if err := win.Screenshot(img); err != nil {
			t.Fatalf("capture frame: %v", err)
		}
	}

	file, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create %s: %v", outPath, err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatalf("encode %s: %v", outPath, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close %s: %v", outPath, err)
	}
	t.Logf("wrote %s", outPath)
}

func showcaseFileManagerUI(cfg *fm.Config) *UI {
	ui := NewUI(cfg)
	for _, pane := range ui.filePanes {
		if pane != nil {
			pane.cancelPendingLoad()
		}
	}

	leftDirs := []string{
		"/Users/ramunas/Projects",
		"/Users/ramunas/Projects/hexone",
		"/Users/ramunas/Downloads",
	}
	rightDirs := []string{
		"/Users/ramunas/Documents",
		"/Users/ramunas/Projects/hexone/releases",
		"/Users/ramunas/Projects/hexone/assets",
	}
	leftTabs := make([]*filePaneState, 0, len(leftDirs))
	for _, dir := range leftDirs {
		leftTabs = append(leftTabs, showcasePane(dir, cfg))
	}
	rightTabs := make([]*filePaneState, 0, len(rightDirs))
	for _, dir := range rightDirs {
		rightTabs = append(rightTabs, showcasePane(dir, cfg))
	}
	ui.filePaneTabs[0].tabs = leftTabs
	ui.filePaneTabs[0].active = 1
	ui.filePanes[0] = leftTabs[1]
	ui.filePaneTabs[1].tabs = rightTabs
	ui.filePaneTabs[1].active = 1
	ui.filePanes[1] = rightTabs[1]
	ui.setActiveFilePane(0)
	return ui
}

func showcasePane(dir string, cfg *fm.Config) *filePaneState {
	pane := newFilePaneState(dir, cfg)
	pane.cancelPendingLoad()
	listing := showcaseListing(dir)
	selected := ""
	if len(listing.Entries) > 5 {
		selected = listing.Entries[5].Path
	}
	pane.applyListing(listing, selected, "", 0)
	return pane
}

func showcaseListing(dir string) filesys.Listing {
	type item struct {
		name string
		kind filesys.EntryKind
		size string
		date string
	}
	items := []item{
		{name: "cmd", kind: filesys.EntryDir, date: "Jul 27 09:42"},
		{name: "ui", kind: filesys.EntryDir, date: "Jul 27 10:38"},
		{name: "assets", kind: filesys.EntryDir, date: "Jul 26 22:16"},
		{name: "filesys", kind: filesys.EntryDir, date: "Jul 25 18:04"},
		{name: "fm", kind: filesys.EntryDir, date: "Jul 25 17:51"},
		{name: "appicon", kind: filesys.EntryDir, date: "Jul 23 11:20"},
		{name: "buildinfo", kind: filesys.EntryDir, date: "Jul 22 18:42"},
		{name: "notify", kind: filesys.EntryDir, date: "Jul 22 17:08"},
		{name: "protocols", kind: filesys.EntryDir, date: "Jul 21 15:34"},
		{name: "secretstore", kind: filesys.EntryDir, date: "Jul 20 14:12"},
		{name: "tools", kind: filesys.EntryDir, date: "Jul 19 21:02"},
		{name: "windowstate", kind: filesys.EntryDir, date: "Jul 18 18:57"},
		{name: ".github", kind: filesys.EntryDir, date: "Jul 22 16:08"},
		{name: ".gitignore", size: "418 B", date: "Jul 22 16:08"},
		{name: "README.md", size: "7.8 KB", date: "Jul 27 10:35"},
		{name: "CHANGELOG.md", size: "12.4 KB", date: "Jul 27 10:47"},
		{name: "HELP.md", size: "22.7 KB", date: "Jul 27 10:45"},
		{name: "go.mod", size: "3.1 KB", date: "Jul 26 20:18"},
		{name: "go.sum", size: "18.9 KB", date: "Jul 26 20:18"},
		{name: "hexone.yaml", size: "6.4 KB", date: "Jul 25 12:02"},
		{name: "protocols.yaml", size: "14.6 KB", date: "Jul 24 15:36"},
		{name: "release-notes.md", size: "4.7 KB", date: "Jul 27 10:32"},
		{name: "hexone-macos-arm64.dmg", size: "18.3 MB", date: "Jul 27 09:58"},
		{name: "hexone-windows-amd64.zip", size: "16.9 MB", date: "Jul 27 09:57"},
		{name: "hexone-linux-amd64.AppImage", size: "21.5 MB", date: "Jul 27 09:56"},
		{name: "benchmark-results.csv", size: "38.2 KB", date: "Jul 26 23:14"},
		{name: "viewer-demo.bin", size: "2.4 MB", date: "Jul 26 21:03"},
		{name: "terminal-session.log", size: "842 KB", date: "Jul 26 20:49"},
		{name: "hexone", size: "24.8 MB", date: "Jul 27 10:26"},
		{name: "Makefile", size: "2.1 KB", date: "Jul 21 08:35"},
		{name: "release-checksums.txt", size: "384 B", date: "Jul 27 10:01"},
		{name: "screenshots.zip", size: "6.8 MB", date: "Jul 27 09:49"},
		{name: "coverage.out", size: "1.7 MB", date: "Jul 26 22:31"},
		{name: "LICENSE", size: "11.1 KB", date: "Jul 13 08:00"},
	}
	entries := []filesys.Entry{{
		Name:        "..",
		DisplayName: "..",
		Path:        filepath.Dir(dir),
		Kind:        filesys.EntryParent,
		CanEnter:    true,
		PermText:    "rwxr-xr-x",
		DateText:    "Jul 27 10:48",
	}}
	for _, value := range items {
		entry := filesys.Entry{
			Name:        value.name,
			DisplayName: value.name,
			Path:        filepath.Join(dir, value.name),
			Kind:        value.kind,
			CanEnter:    value.kind == filesys.EntryDir,
			PermText:    "rw-r--r--",
			SizeText:    value.size,
			DateText:    value.date,
		}
		if value.kind == filesys.EntryDir {
			entry.PermText = "rwxr-xr-x"
		}
		entries = append(entries, entry)
	}
	return filesys.Listing{Dir: dir, Entries: entries}
}

func showcaseHexEditorUI(cfg *fm.Config) *UI {
	cfgCopy := *cfg
	cfg = &cfgCopy
	cfg.Viewer.HideFunctionBarWhenOpen = false
	ui := NewUI(cfg)
	hex := newHexViewerState()
	hex.fileSize = 2048
	hex.buffer = make([]byte, hex.fileSize)
	sample := []byte{
		0x7F, 0x45, 0x4C, 0x46, 0x02, 0x01, 0x01, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x02, 0x00, 0xB7, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x78, 0x56, 0x34, 0x12, 0x10, 0x00, 0x00, 0x00,
	}
	for i := range hex.buffer {
		hex.buffer[i] = sample[i%len(sample)]
	}
	hex.offsetDigits = 8
	hex.setSelectionRange(18, 8)
	hex.editCaret = 18
	hex.editNibble = 1
	for off, value := range map[int64]byte{
		18: 0x48,
		19: 0x65,
		20: 0x78,
		21: 0x6F,
		22: 0x6E,
		23: 0x65,
	} {
		hex.setEditedByte(off, value)
	}
	ui.fileViewer = &fileViewerState{
		mode:      "hex",
		path:      "/Users/ramunas/Projects/hexone/releases/hexone-linux-amd64",
		name:      "hexone-linux-amd64",
		editMode:  true,
		editDirty: true,
		hex:       hex,
	}
	return ui
}

func showcaseTerminalUI(t *testing.T, cfg *fm.Config) *UI {
	t.Helper()
	cfgCopy := *cfg
	cfg = &cfgCopy
	cfg.Terminal.HeightRows = 16
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	ctx := localTerminalSnippetContext(dir)
	cfg.TerminalSnippets = []fm.TerminalSnippet{
		{Name: "Run all tests", Command: "go test ./...", Scope: fm.TerminalSnippetScopeRepository, Context: ctx.repository},
		{Name: "Build macOS app", Command: "go build ./cmd/hexone", Scope: fm.TerminalSnippetScopeRepository, Context: ctx.repository},
		{Name: "Show changed files", Command: "git status --short", Scope: fm.TerminalSnippetScopeDirectory, Context: ctx.directory},
		{Name: "Disk usage", Command: "du -sh .", Scope: fm.TerminalSnippetScopeGlobal},
	}
	ui := showcaseFileManagerUI(cfg)
	st := newTerminalSession(nil, cfg.Terminal.HeightRows)
	st.startDir = dir
	st.setActive(true)
	st.startAttempted = true
	st.wantFocus = true
	output := "\x1b[36m~/Projects/hexone\x1b[0m \x1b[33mcodex/file-clipboard\x1b[0m $ go test ./...\r\n" +
		"ok  \thexone/filesys\t0.184s\r\n" +
		"ok  \thexone/fm\t0.021s\r\n" +
		"ok  \thexone/ui\t8.504s\r\n" +
		"ok  \thexone/ui/platform\t0.043s\r\n\r\n" +
		"\x1b[32mAll packages passed.\x1b[0m\r\n" +
		"\x1b[36m~/Projects/hexone\x1b[0m \x1b[33mcodex/file-clipboard\x1b[0m $ "
	if _, err := st.term.Write([]byte(output)); err != nil {
		t.Fatal(err)
	}
	ui.terminal = st
	ui.terminalTabs.sessions = []*terminalSession{st}
	ui.terminalTabs.active = 0
	ui.terminalSnippetMenuOpen = true
	ui.terminalSnippetMenuOpenedAt = time.Now().Add(-time.Second)
	return ui
}

func showcaseProtocolAnalyzerUI(cfg *fm.Config) *UI {
	ui := NewUI(cfg)
	ui.Tabs.Value = "tab2"
	ui.tab2State.hexEd.SetText("78 78 1F 12 0B 08 1D 11 2E 10 CC 02 7A C7 EB 0C 46 58 49 00 14 8F 01 CC 00 28 7D 00 1F B8 00 03 80 81 0D 0A")
	return ui
}

func showcaseSelectAnalyzerSerial(ui *UI) {
	if ui == nil || ui.tab2State == nil {
		return
	}
	for _, row := range analyzerLeafRows(ui.tab2State) {
		if row.Span.Name != "serial" {
			continue
		}
		ui.tab2State.selectedSpanKey = rangeKey(row.Span.Start, row.Span.End)
		ui.tab2State.selectedRowID = row.Key
		ui.tab2State.selectedHint = row.Span
		break
	}
}
