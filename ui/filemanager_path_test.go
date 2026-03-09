package ui

import (
	"image"
	"testing"

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
