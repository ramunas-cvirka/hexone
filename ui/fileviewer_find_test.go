// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hexone/fm"
)

func TestViewerFindTextMatchesAllowsOverlaps(t *testing.T) {
	got := viewerFindTextMatches("banana", "ana")

	if len(got) != 2 {
		t.Fatalf("len(matches)=%d want 2", len(got))
	}
	if got[0].Start != 1 || got[0].End != 4 {
		t.Fatalf("first match=%+v want {Start:1 End:4}", got[0])
	}
	if got[1].Start != 3 || got[1].End != 6 {
		t.Fatalf("second match=%+v want {Start:3 End:6}", got[1])
	}
}

func TestParseViewerFindHexStringNormalizesSeparators(t *testing.T) {
	got, errText := parseViewerFindHexString("0xDE AD-be:ef")

	if errText != "" {
		t.Fatalf("parseViewerFindHexString err=%q", errText)
	}
	want := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if len(got) != len(want) {
		t.Fatalf("len(bytes)=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte[%d]=0x%X want 0x%X", i, got[i], want[i])
		}
	}
}

func TestParseViewerFindHexStringRejectsOddDigits(t *testing.T) {
	_, errText := parseViewerFindHexString("ABC")

	if errText == "" {
		t.Fatal("expected odd-length hex query to be rejected")
	}
}

func TestSearchViewerHexNextWrapsAcrossFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.bin")
	if err := os.WriteFile(path, []byte("abc---abc"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	res := searchViewerHexNext(context.Background(), path, nil, []byte("abc"), 4, viewerRemoteSearchSpec{})

	if !res.found {
		t.Fatal("expected wrapped next search to find a match")
	}
	if res.start != 6 {
		t.Fatalf("res.start=%d want 6", res.start)
	}
	if res.wrapped {
		t.Fatal("expected later in-file match before wrap")
	}

	res = searchViewerHexNext(context.Background(), path, nil, []byte("abc"), 9, viewerRemoteSearchSpec{})

	if !res.found || res.start != 0 || !res.wrapped {
		t.Fatalf("wrapped next search = %+v want start=0 wrapped=true", res)
	}
}

func TestSearchViewerHexPrevWrapsAndFallsBackToCurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.bin")
	if err := os.WriteFile(path, []byte("abc---abc"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	res := searchViewerHexPrev(context.Background(), path, nil, []byte("abc"), 6, 6, viewerRemoteSearchSpec{})

	if !res.found || res.start != 0 || res.wrapped {
		t.Fatalf("previous search = %+v want start=0 wrapped=false", res)
	}

	onePath := filepath.Join(t.TempDir(), "one.bin")
	if err := os.WriteFile(onePath, []byte("zzabczz"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	res = searchViewerHexPrev(context.Background(), onePath, nil, []byte("abc"), 2, 2, viewerRemoteSearchSpec{})

	if !res.found || res.start != 2 || !res.wrapped {
		t.Fatalf("single-match previous search = %+v want start=2 wrapped=true", res)
	}
}

func TestFileViewerFindStatusTextDelaysSearchingIndicator(t *testing.T) {
	ui := NewUI(nil)
	now := time.Now()
	st := &fileViewerState{
		find: fileViewerFindState{
			searching:       true,
			searchStartedAt: now,
		},
	}

	if got := ui.fileViewerFindStatusText(st, now.Add(viewerFindSearchingDelay/2)); got != "" {
		t.Fatalf("early searching status=%q want empty", got)
	}
	if got := ui.fileViewerFindStatusText(st, now.Add(viewerFindSearchingDelay)); got != "Searching..." {
		t.Fatalf("delayed searching status=%q want %q", got, "Searching...")
	}

	st.find.currentValid = true
	st.find.status = "2/5"
	if got := ui.fileViewerFindStatusText(st, now.Add(viewerFindSearchingDelay)); got != "2/5" {
		t.Fatalf("searching with current match status=%q want %q", got, "2/5")
	}
}

func TestViewerFindModeTabsToggleBetweenTextAndHex(t *testing.T) {
	ui := &UI{
		fileViewer: &fileViewerState{
			mode: "hex",
		},
	}
	now := time.Now()

	if !ui.stepFileViewerFindMode(now, 1) {
		t.Fatal("stepFileViewerFindMode should toggle from text to hex")
	}
	if !ui.fileViewer.find.hexInput {
		t.Fatal("hexInput should be enabled after stepping forward")
	}
	if !ui.stepFileViewerFindMode(now, -1) {
		t.Fatal("stepFileViewerFindMode should toggle from hex to text")
	}
	if ui.fileViewer.find.hexInput {
		t.Fatal("hexInput should be disabled after stepping backward")
	}
}

func TestViewerRemoteSearchOffsetParsesFirstOffsetLine(t *testing.T) {
	got, ok := viewerRemoteSearchOffset("grep: warning\n123:needle\n")

	if !ok {
		t.Fatal("viewerRemoteSearchOffset should parse a numeric match line")
	}
	if got != 123 {
		t.Fatalf("offset=%d want 123", got)
	}
}

func TestViewerFindRemoteUtilityForwardUsesRelativeOffset(t *testing.T) {
	prev := runViewerRemoteSearchCommandFunc
	defer func() { runViewerRemoteSearchCommandFunc = prev }()

	var gotCmd string
	runViewerRemoteSearchCommandFunc = func(_ context.Context, _ *paneSSHSession, cmdline string, _ viewerShellSpec) (string, error) {
		gotCmd = cmdline
		return "7:needle\n", nil
	}

	spec := viewerRemoteSearchSpec{
		template: fm.DefaultViewerRemoteSearchCommand,
		shell:    resolveViewerShell("sh", true),
		hexInput: false,
	}

	res, used := viewerFindRemoteUtilityForward(context.Background(), 1024, "/var/log/app.log", &paneSSHSession{}, []byte("needle"), 100, 1000, spec)

	if !used {
		t.Fatal("viewer remote search utility should be used when remote search is enabled")
	}
	if !res.found || res.start != 107 || res.length != int64(len("needle")) {
		t.Fatalf("result=%+v want found start=107 length=6", res)
	}
	if !strings.Contains(gotCmd, "tail -c +101") {
		t.Fatalf("command=%q want 1-based range start", gotCmd)
	}
	if !strings.Contains(gotCmd, "head -c 900") {
		t.Fatalf("command=%q want range length", gotCmd)
	}
	if !strings.Contains(gotCmd, "grep -aobF -m 1 -- 'needle'") {
		t.Fatalf("command=%q want fixed-string grep", gotCmd)
	}
}

func TestViewerRemoteSearchUtilityDefaultSkipsHexPatterns(t *testing.T) {
	spec := viewerRemoteSearchSpec{
		template: fm.DefaultViewerRemoteSearchCommand,
		shell:    resolveViewerShell("sh", true),
		hexInput: true,
	}

	if viewerRemoteSearchUsable(spec, 1024) {
		t.Fatal("default remote search command should not claim hex-pattern support")
	}
}

func TestViewerFindRemoteUtilityForwardFallsBackOnGarbageOutput(t *testing.T) {
	prev := runViewerRemoteSearchCommandFunc
	defer func() { runViewerRemoteSearchCommandFunc = prev }()

	runViewerRemoteSearchCommandFunc = func(_ context.Context, _ *paneSSHSession, _ string, _ viewerShellSpec) (string, error) {
		return "grep: command not found\n", nil
	}

	spec := viewerRemoteSearchSpec{
		template: fm.DefaultViewerRemoteSearchCommand,
		shell:    resolveViewerShell("sh", true),
		hexInput: false,
	}

	if _, used := viewerFindRemoteUtilityForward(context.Background(), 1024, "/var/log/app.log", &paneSSHSession{}, []byte("needle"), 0, 900, spec); used {
		t.Fatal("garbage command output should fall back to the built-in remote scan")
	}
}

func TestViewerRemoteSearchRemoteModeUsesUtility(t *testing.T) {
	prev := runViewerRemoteSearchCommandFunc
	defer func() { runViewerRemoteSearchCommandFunc = prev }()

	runViewerRemoteSearchCommandFunc = func(_ context.Context, _ *paneSSHSession, _ string, _ viewerShellSpec) (string, error) {
		return "3:needle\n", nil
	}

	spec := viewerRemoteSearchSpec{
		template: fm.DefaultViewerRemoteSearchCommand,
		mode:     fm.ViewerRemoteSearchModeRemote,
		shell:    resolveViewerShell("sh", true),
		hexInput: false,
	}

	res, used := viewerFindRemoteUtilityForward(context.Background(), 1024, "/var/log/app.log", &paneSSHSession{}, []byte("needle"), 10, 128, spec)
	if !used {
		t.Fatal("remote mode should use the remote utility")
	}
	if !res.found || res.start != 13 {
		t.Fatalf("result=%+v want found start=13", res)
	}
}

func TestViewerRemoteSearchLocalModeSkipsUtility(t *testing.T) {
	spec := viewerRemoteSearchSpec{
		template: fm.DefaultViewerRemoteSearchCommand,
		mode:     fm.ViewerRemoteSearchModeLocal,
		shell:    resolveViewerShell("sh", true),
		hexInput: false,
	}

	if viewerRemoteSearchUsable(spec, 1024) {
		t.Fatal("local mode should skip the remote utility")
	}
}
