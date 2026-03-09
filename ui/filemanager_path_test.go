package ui

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"hexone/filesys"
	"hexone/fm"
)

func TestSplitFilePathSegmentsCompactUnixLabels(t *testing.T) {
	segments := splitFilePathSegments("/opt/gpstrack/log")
	if len(segments) != 4 {
		t.Fatalf("segment count = %d, want 4", len(segments))
	}

	wantLabels := []string{"/", "opt", "/gpstrack", "/log"}
	wantPaths := []string{"/", "/opt", "/opt/gpstrack", "/opt/gpstrack/log"}
	for i := range wantLabels {
		if segments[i].label != wantLabels[i] {
			t.Fatalf("segment %d label = %q, want %q", i, segments[i].label, wantLabels[i])
		}
		if segments[i].path != wantPaths[i] {
			t.Fatalf("segment %d path = %q, want %q", i, segments[i].path, wantPaths[i])
		}
	}
}

func TestRemotePathDisplaySegmentsMergesHostIntoRoot(t *testing.T) {
	segments := remotePathDisplaySegments("root@157.180.68.247", "/opt/gpstrack/log")
	if len(segments) != 4 {
		t.Fatalf("segment count = %d, want 4", len(segments))
	}

	wantLabels := []string{"root@157.180.68.247/", "opt", "/gpstrack", "/log"}
	wantPaths := []string{"/", "/opt", "/opt/gpstrack", "/opt/gpstrack/log"}
	for i := range wantLabels {
		if segments[i].label != wantLabels[i] {
			t.Fatalf("segment %d label = %q, want %q", i, segments[i].label, wantLabels[i])
		}
		if segments[i].path != wantPaths[i] {
			t.Fatalf("segment %d path = %q, want %q", i, segments[i].path, wantPaths[i])
		}
	}
}

func TestRemotePathDisplaySegmentsDefaultsAddress(t *testing.T) {
	segments := remotePathDisplaySegments("", "/")
	if len(segments) != 1 {
		t.Fatalf("segment count = %d, want 1", len(segments))
	}
	if segments[0].label != "ssh/" {
		t.Fatalf("root label = %q, want %q", segments[0].label, "ssh/")
	}
	if segments[0].path != "/" {
		t.Fatalf("root path = %q, want /", segments[0].path)
	}
}

func TestLayoutFilePanePathEditorUsesWrappedPathAreaWhenInactive(t *testing.T) {
	ui := &UI{fmCfg: fm.DefaultConfig()}
	pane := newFilePaneState(".", ui.fmCfg)
	pane.applyListing(testPathListing("/opt/gpstrack/log"), "", "", 0)
	pane.pathEditing = false

	th := material.NewTheme()
	areaDims := ui.layoutFilePanePathArea(th, testPathLayoutContext(), 0, pane, true)
	editorDims := ui.layoutFilePanePathEditor(th, testPathLayoutContext(), 0, pane, true)

	if editorDims.Size != areaDims.Size {
		t.Fatalf("inactive editor dims = %v, want path area dims %v", editorDims.Size, areaDims.Size)
	}
}

func TestActivateFilePanePathSegmentStartsLoadImmediately(t *testing.T) {
	cfg := fm.DefaultConfig()
	root := t.TempDir()
	target := filepath.Join(root, "logs")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	ui := &UI{fmCfg: cfg}
	pane := newFilePaneState(root, cfg)
	pane.pendingPathNav = "/stale"
	pane.pendingPathAt = time.Now().Add(time.Second)
	pane.pathClickKey = "row:" + root
	pane.pathClickAt = time.Now()
	ui.filePanes = []*filePaneState{pane}

	if !ui.activateFilePanePathSegment(0, pane, target) {
		t.Fatal("activateFilePanePathSegment returned false")
	}
	if pane.pendingPathNav != "" || !pane.pendingPathAt.IsZero() {
		t.Fatalf("pending path navigate not cleared: %q at %v", pane.pendingPathNav, pane.pendingPathAt)
	}
	if pane.pathClickKey != "" || !pane.pathClickAt.IsZero() {
		t.Fatalf("path click state not cleared: %q at %v", pane.pathClickKey, pane.pathClickAt)
	}
	if !pane.loading {
		t.Fatal("pane not marked loading")
	}
	if got, want := pane.loadingDir, filepath.Clean(target); got != want {
		t.Fatalf("loading dir = %q, want %q", got, want)
	}
}

func testPathListing(dir string) filesys.Listing {
	return filesys.Listing{Dir: dir}
}

func testPathLayoutContext() layout.Context {
	var router input.Router
	return layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(520, 48),
		},
	}
}
