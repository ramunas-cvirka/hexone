// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"path/filepath"
	"testing"
	"time"

	"hexone/filesys"
	"hexone/ui/platform"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestFormatFilePaneVolumeBadgeLabel(t *testing.T) {
	free := (uint64(6323) * (1 << 30)) / 100
	total := uint64(512 * (1 << 30))
	if got := formatFilePaneVolumeBadgeLabel(free, total); got != "63.23 GB free / 512.00 GB" {
		t.Fatalf("formatFilePaneVolumeBadgeLabel() = %q", got)
	}
}

func TestFilePaneVolumeBadgeLabelCachesUntilRefresh(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() {
		localVolumeUsageFunc = oldLookup
	}()

	tempDir := t.TempDir()
	lookupCount := 0
	localVolumeUsageFunc = func(path string) (platform.VolumeUsage, error) {
		lookupCount++
		if path != filepath.Clean(tempDir) {
			t.Fatalf("lookup path = %q, want %q", path, filepath.Clean(tempDir))
		}
		return platform.VolumeUsage{
			FreeBytes:  64 << 30,
			TotalBytes: 512 << 30,
		}, nil
	}

	ui := NewUI(nil)
	pane := newFilePaneState(tempDir, nil)
	now := time.Unix(1700000000, 0)

	label, nextRefreshAt, ok := ui.filePaneVolumeBadgeLabel(pane, now)
	if !ok {
		t.Fatal("filePaneVolumeBadgeLabel() = not ok, want ok")
	}
	if label != "64.00 GB free / 512.00 GB" {
		t.Fatalf("label = %q", label)
	}
	if lookupCount != 1 {
		t.Fatalf("lookupCount = %d, want 1", lookupCount)
	}
	if got, _, ok := ui.filePaneVolumeBadgeLabel(pane, now.Add(time.Second)); !ok || got != label {
		t.Fatalf("cached label = %q, ok=%v", got, ok)
	}
	if lookupCount != 1 {
		t.Fatalf("lookupCount after cached read = %d, want 1", lookupCount)
	}

	if _, _, ok := ui.filePaneVolumeBadgeLabel(pane, nextRefreshAt); !ok {
		t.Fatal("filePaneVolumeBadgeLabel() at refresh time = not ok, want ok")
	}
	if lookupCount != 2 {
		t.Fatalf("lookupCount after refresh = %d, want 2", lookupCount)
	}
}

func TestFilePaneVolumeBadgeLabelUsesRemotePaneUsage(t *testing.T) {
	oldRemoteLookup := remoteVolumeUsageFunc
	defer func() {
		remoteVolumeUsageFunc = oldRemoteLookup
	}()

	lookupCount := 0
	remote := &paneSSHSession{}
	remoteVolumeUsageFunc = func(got *paneSSHSession, path string) (platform.VolumeUsage, error) {
		lookupCount++
		if got != remote {
			t.Fatalf("remote = %p, want %p", got, remote)
		}
		if path != "/srv/projects" {
			t.Fatalf("lookup path = %q, want %q", path, "/srv/projects")
		}
		return platform.VolumeUsage{
			FreeBytes:  128 << 30,
			TotalBytes: 512 << 30,
		}, nil
	}

	ui := NewUI(nil)
	pane := newFilePaneState("/srv/projects", nil)
	pane.remote = remote
	now := time.Unix(1700000000, 0)

	label, nextRefreshAt, ok := ui.filePaneVolumeBadgeLabel(pane, now)
	if !ok {
		t.Fatal("filePaneVolumeBadgeLabel() = not ok, want ok")
	}
	if label != "128.00 GB free / 512.00 GB" {
		t.Fatalf("label = %q", label)
	}
	if lookupCount != 1 {
		t.Fatalf("lookupCount = %d, want 1", lookupCount)
	}
	if got, _, ok := ui.filePaneVolumeBadgeLabel(pane, now.Add(time.Second)); !ok || got != label {
		t.Fatalf("cached remote label = %q, ok=%v", got, ok)
	}
	if lookupCount != 1 {
		t.Fatalf("lookupCount after cached remote read = %d, want 1", lookupCount)
	}
	if _, _, ok := ui.filePaneVolumeBadgeLabel(pane, nextRefreshAt); !ok {
		t.Fatal("filePaneVolumeBadgeLabel() at remote refresh time = not ok, want ok")
	}
	if lookupCount != 2 {
		t.Fatalf("lookupCount after remote refresh = %d, want 2", lookupCount)
	}
}

func TestApplyListingWithRestoreInvalidatesVolumeBadge(t *testing.T) {
	dir := t.TempDir()
	pane := newFilePaneState(dir, nil)
	pane.volumeBadge.label = "cached"
	pane.volumeBadge.nextRefreshAt = time.Now().Add(time.Minute)

	pane.applyListingWithRestore(filesys.Listing{Dir: dir}, "", "", 0, layout.Position{}, false, "")

	if pane.volumeBadge.nextRefreshAt != (time.Time{}) {
		t.Fatalf("nextRefreshAt = %v, want zero", pane.volumeBadge.nextRefreshAt)
	}
}

func TestFilePaneVolumeBadgeOffsetPinsToBottomInnerCorner(t *testing.T) {
	paneSize := image.Pt(320, 200)
	badgeSize := image.Pt(120, 20)

	leftInactive := filePaneVolumeBadgeOffset(0, 1, paneSize, badgeSize)
	if want := image.Pt(201, 180); leftInactive != want {
		t.Fatalf("left inactive offset = %v, want %v", leftInactive, want)
	}

	rightInactive := filePaneVolumeBadgeOffset(1, 0, paneSize, badgeSize)
	if want := image.Pt(0, 180); rightInactive != want {
		t.Fatalf("right inactive offset = %v, want %v", rightInactive, want)
	}
}

func TestFilePaneVolumeBadgeSourcePaneUsesActivePane(t *testing.T) {
	ui := NewUI(nil)
	left := newFilePaneState(t.TempDir(), nil)
	right := newFilePaneState(t.TempDir(), nil)
	ui.filePanes = []*filePaneState{left, right}

	ui.activeFilePane = 0
	if got := ui.filePaneVolumeBadgeSourcePane(1, right, false); got != left {
		t.Fatalf("source pane = %p, want active left pane %p", got, left)
	}

	ui.activeFilePane = 1
	if got := ui.filePaneVolumeBadgeSourcePane(0, left, false); got != right {
		t.Fatalf("source pane = %p, want active right pane %p", got, right)
	}

	if got := ui.filePaneVolumeBadgeSourcePane(1, right, true); got != nil {
		t.Fatalf("active pane badge source = %p, want nil", got)
	}
}

func TestFilePaneVolumeBadgeSourcePaneKeepsMirroringExtractingPane(t *testing.T) {
	ui := NewUI(nil)
	left := newFilePaneState(t.TempDir(), nil)
	right := newFilePaneState(t.TempDir(), nil)
	ui.filePanes = []*filePaneState{left, right}
	ui.activeFilePane = 0

	now := time.Unix(1700000000, 0)
	ui.archiveExtract = &archiveExtractState{
		pane:        0,
		archivePath: "bundle.zip",
		startedAt:   now.Add(-time.Second),
		progress: filesys.CopyProgress{
			BytesDone:   50 << 20,
			BytesTotal:  100 << 20,
			CurrentPath: filepath.Join("bundle.zip", "movie.mkv"),
		},
	}

	if got := ui.filePaneVolumeBadgeSourcePane(1, right, false); got != left {
		t.Fatalf("badge source mirrored from extracting active pane = %p, want left pane %p", got, left)
	}

	if got := ui.filePaneVolumeBadgeSourcePane(0, left, true); got != nil {
		t.Fatalf("active extracting pane badge source = %p, want nil", got)
	}
}

func TestLayoutFilePaneStatusBarUsesFullPaneWidth(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() {
		localVolumeUsageFunc = oldLookup
	}()

	dir := t.TempDir()
	localVolumeUsageFunc = func(path string) (platform.VolumeUsage, error) {
		return platform.VolumeUsage{
			FreeBytes:  64 << 30,
			TotalBytes: 512 << 30,
		}, nil
	}

	ui := NewUI(nil)
	pane := newFilePaneState(dir, nil)
	ui.filePanes = []*filePaneState{pane}
	ui.activeFilePane = 0

	now := time.Unix(1700000000, 0)
	ui.archiveExtract = &archiveExtractState{
		pane:        0,
		archivePath: "bundle.zip",
		startedAt:   now.Add(-time.Second),
		progress: filesys.CopyProgress{
			BytesDone:   50 << 20,
			BytesTotal:  100 << 20,
			CurrentPath: filepath.Join("bundle.zip", "movie.mkv"),
		},
	}

	gtx := layout.Context{
		Ops:         new(op.Ops),
		Now:         now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(320, 80)),
	}
	dims := ui.layoutFilePaneStatusBar(material.NewTheme(), gtx, 0, pane, filePanePaletteFromConfig(ui.fmCfg))
	if dims.Size.X != 320 {
		t.Fatalf("status bar width = %d, want full pane width 320", dims.Size.X)
	}
	if dims.Size.Y <= 0 {
		t.Fatalf("status bar height = %d, want positive", dims.Size.Y)
	}
}

func TestFilePaneStatusBarSeparatorModeFollowsPaneSide(t *testing.T) {
	ui := NewUI(nil)
	left := newFilePaneState(t.TempDir(), nil)
	right := newFilePaneState(t.TempDir(), nil)
	ui.filePanes = []*filePaneState{left, right}

	if got := ui.filePaneStatusBarSeparatorMode(0); got != filePaneStatusBarSeparatorTrailing {
		t.Fatalf("left pane separator mode = %v, want trailing", got)
	}
	if got := ui.filePaneStatusBarSeparatorMode(1); got != filePaneStatusBarSeparatorLeading {
		t.Fatalf("right pane separator mode = %v, want leading", got)
	}

	ui.filePanes = []*filePaneState{left}
	if got := ui.filePaneStatusBarSeparatorMode(0); got != filePaneStatusBarSeparatorNone {
		t.Fatalf("single pane separator mode = %v, want none", got)
	}
}
