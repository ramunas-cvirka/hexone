// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build uiverify

package ui

import (
	"context"
	"fmt"
	"hexone/filesys"
	"hexone/fm"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestHeadlessFileOperationDialogs(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	tests := []struct {
		name  string
		setup func(*UI) func()
	}{
		{
			name: "filecopy-small-files",
			setup: func(ui *UI) func() {
				_, cancel := context.WithCancel(context.Background())
				ui.fileCopy = &fileCopyState{
					op:      fileCopyOpCopy,
					sources: []fileCopySource{{Path: "/srv/data/logs", Name: "logs"}, {Path: "/srv/data/archive", Name: "archive"}, {Path: "/srv/data/readme", Name: "readme"}},
					srcPath: "/srv/data/logs",
					dstPath: "/srv/backup",
					dstRaw:  "/srv/backup",
					running: true,
					progress: filesys.CopyProgress{
						Streaming:         true,
						FilesDiscovered:   8492,
						FilesCopied:       1248,
						BytesDone:         84 << 20,
						CurrentBytesDone:  8 << 10,
						CurrentBytesTotal: 24 << 10,
						CurrentPath:       "/srv/data/archive/2026/07/13/app.log",
						CurrentRootPath:   "/srv/data/archive",
					},
					progressCh:  make(chan filesys.CopyProgress),
					doneCh:      make(chan error),
					cancelFunc:  cancel,
					startedAt:   time.Now(),
					speedBytes:  10 << 20,
					speedAt:     time.Now(),
					speedDone:   84 << 20,
					speedSeenAt: time.Now(),
					srcEndpoint: copyEndpoint{dir: "/srv/data"},
					dstEndpoint: copyEndpoint{dir: "/srv/backup"},
				}
				ui.fileCopy.dstEdit.SetText("/srv/backup")
				return cancel
			},
		},
		{
			name: "filecopy-large-file",
			setup: func(ui *UI) func() {
				_, cancel := context.WithCancel(context.Background())
				ui.fileCopy = &fileCopyState{
					op:      fileCopyOpCopy,
					sources: []fileCopySource{{Path: "/srv/data/big-dataset.tar.gz", Name: "big-dataset.tar.gz"}},
					srcPath: "/srv/data/big-dataset.tar.gz",
					dstPath: "/srv/backup",
					running: true,
					progress: filesys.CopyProgress{
						Streaming:         true,
						FilesDiscovered:   1,
						CurrentBytesDone:  14 << 30,
						CurrentBytesTotal: 45 << 30,
						CurrentPath:       "/srv/data/big-dataset.tar.gz",
						CurrentRootPath:   "/srv/data/big-dataset.tar.gz",
					},
					progressCh:  make(chan filesys.CopyProgress),
					doneCh:      make(chan error),
					cancelFunc:  cancel,
					startedAt:   time.Now(),
					speedBytes:  84 << 20,
					speedAt:     time.Now(),
					speedDone:   14 << 30,
					speedSeenAt: time.Now(),
					srcEndpoint: copyEndpoint{dir: "/srv/data"},
					dstEndpoint: copyEndpoint{dir: "/srv/backup"},
				}
				return cancel
			},
		},
		{
			name: "filecopy-overwrite",
			setup: func(ui *UI) func() {
				now := time.Date(2026, 7, 13, 10, 20, 0, 0, time.Local)
				ui.fileCopy = &fileCopyState{
					op:          fileCopyOpCopy,
					sources:     []fileCopySource{{Path: "/srv/data/vendor", Name: "vendor"}},
					srcPath:     "/srv/data/vendor",
					dstPath:     "/srv/backup/vendor",
					dstRaw:      "/srv/backup/vendor",
					srcInfo:     fileCopyPathInfo{Path: "/srv/data/vendor", Exists: true, IsDir: true, ModTime: now},
					dstInfo:     fileCopyPathInfo{Path: "/srv/backup/vendor", Exists: true, IsDir: true, ModTime: now.Add(-time.Hour)},
					srcEndpoint: copyEndpoint{dir: "/srv/data"},
					dstEndpoint: copyEndpoint{dir: "/srv/backup"},
					focus:       fileCopyDialogFocusDestination,
				}
				ui.fileCopy.dstEdit.SetText("/srv/backup/vendor")
				return func() {}
			},
		},
		{
			name: "filecopy-multi-overwrite",
			setup: func(ui *UI) func() {
				srcDir, dstDir, sources, cleanup := headlessOverwriteFixture([]string{
					"quarterly-reconciliation-archive-2026-final.txt", "a2.txt", "a3.txt", "a4.txt", "a5.txt", "a6.txt",
				})
				headlessSetAllSourcesNewer(sources, dstDir)
				ui.fileCopy = &fileCopyState{
					op:          fileCopyOpCopy,
					sources:     sources,
					srcPath:     sources[0].Path,
					dstPath:     dstDir,
					dstRaw:      dstDir,
					srcEndpoint: copyEndpoint{dir: srcDir},
					dstEndpoint: copyEndpoint{dir: dstDir},
					focus:       fileCopyDialogFocusDestination,
				}
				ui.fileCopy.dstEdit.SetText(dstDir)
				ui.fileCopy.refreshPreview()
				return cleanup
			},
		},
		{
			name: "filemove",
			setup: func(ui *UI) func() {
				ui.fileMove = &fileMoveState{
					pane:     0,
					sources:  []fileMoveSource{{Path: "/srv/data/logs", Name: "logs"}, {Path: "/srv/data/archive", Name: "archive"}, {Path: "/srv/data/readme", Name: "readme"}},
					srcPath:  "/srv/data/logs",
					dstPath:  "/srv/backup",
					endpoint: copyEndpoint{dir: "/srv/data"},
					focus:    fileMoveDialogFocusDestination,
				}
				ui.fileMove.dstEdit.SetText("/srv/backup")
				return func() {}
			},
		},
		{
			name: "filemove-multi-overwrite",
			setup: func(ui *UI) func() {
				srcDir, dstDir, copySources, cleanup := headlessOverwriteFixture([]string{
					"alpha.log", "beta.log", "gamma.log", "delta.log", "epsilon.log", "zeta.log", "eta.log",
				})
				sources := make([]fileMoveSource, 0, len(copySources))
				for _, source := range copySources {
					sources = append(sources, fileMoveSource{Path: source.Path, Name: source.Name})
				}
				ui.fileMove = &fileMoveState{
					pane:     0,
					sources:  sources,
					srcPath:  sources[0].Path,
					dstPath:  dstDir,
					endpoint: copyEndpoint{dir: srcDir},
					focus:    fileMoveDialogFocusDestination,
				}
				ui.fileMove.dstEdit.SetText(dstDir)
				ui.fileMove.refreshPreview()
				return cleanup
			},
		},
		{
			name: "filecreate",
			setup: func(ui *UI) func() {
				ui.fileCreate = &fileCreateState{
					pane:       0,
					baseDir:    "/srv/data/releases",
					kind:       fileCreateKindFolder,
					kindPrev:   fileCreateKindFolder,
					targetPath: "/srv/data/releases/2026-07-13",
					focus:      fileCreateDialogFocusName,
					kindFocus:  fileCreateKindFolder,
				}
				ui.fileCreate.nameEdit.SetText("2026-07-13")
				return func() {}
			},
		},
		{
			name: "filedelete",
			setup: func(ui *UI) func() {
				ui.fileDelete = &fileDeleteState{
					pane:       0,
					targets:    []fileDeleteTarget{{Path: "/srv/data/logs", Name: "logs"}, {Path: "/srv/data/archive", Name: "archive"}, {Path: "/srv/data/readme", Name: "readme"}},
					targetPath: "/srv/data/logs",
					targetName: "logs",
					targetInfo: fileCopyPathInfo{Path: "/srv/data/logs", Exists: true, IsDir: true},
				}
				return func() {}
			},
		},
		{
			name: "filedelete-trash",
			setup: func(ui *UI) func() {
				ui.fileDelete = &fileDeleteState{
					pane:        0,
					targets:     []fileDeleteTarget{{Path: "/srv/data/logs", Name: "logs"}, {Path: "/srv/data/archive", Name: "archive"}, {Path: "/srv/data/readme", Name: "readme"}},
					targetPath:  "/srv/data/logs",
					targetName:  "logs",
					targetInfo:  fileCopyPathInfo{Path: "/srv/data/logs", Exists: true, IsDir: true},
					useTrash:    true,
					actionFocus: fileDeleteDialogActionConfirm,
				}
				return func() {}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui := NewUI(fm.DefaultConfig())
			waitForPaneLoads(t, ui, ui.filePanes...)
			cleanup := tt.setup(ui)
			defer cleanup()
			renderHeadlessFileOperation(t, th, ui, filepath.Join(outDir, tt.name+".png"))
		})
	}
}

func renderHeadlessFileOperation(t *testing.T, th *material.Theme, ui *UI, outPath string) {
	t.Helper()
	width, height := 900, 620
	if raw := os.Getenv("UI_VERIFY_WIDTH"); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &width); err != nil || width < 320 {
			t.Fatalf("invalid UI_VERIFY_WIDTH %q", raw)
		}
	}
	if raw := os.Getenv("UI_VERIFY_HEIGHT"); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &height); err != nil || height < 240 {
			t.Fatalf("invalid UI_VERIFY_HEIGHT %q", raw)
		}
	}
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("headless window: %v", err)
	}
	defer win.Release()
	router := new(input.Router)
	base := time.Now()
	var img *image.RGBA
	completeFrames := 0
	for i := 0; i < 60; i++ {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)),
			Now:         base.Add(time.Duration(i) * 100 * time.Millisecond),
			Source:      router.Source(),
		}
		ui.Layout(th, gtx)
		router.Frame(&ops)
		if err := win.Frame(&ops); err != nil {
			t.Fatalf("render frame: %v", err)
		}
		img = image.NewRGBA(image.Rect(0, 0, width, height))
		if err := win.Screenshot(img); err != nil {
			t.Fatalf("capture frame: %v", err)
		}
		if headlessFrameCoversWindow(img) {
			completeFrames++
			if completeFrames >= 3 {
				break
			}
		} else {
			completeFrames = 0
		}
		time.Sleep(5 * time.Millisecond)
	}
	if completeFrames < 3 {
		t.Fatal("headless UI never produced a complete frame")
	}

	f, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create screenshot: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encode screenshot: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close screenshot: %v", err)
	}
	t.Logf("wrote %s", outPath)
}

func headlessOverwriteFixture(names []string) (srcDir, dstDir string, sources []fileCopySource, cleanup func()) {
	root, err := os.MkdirTemp("", "hexone-overwrite-preview-")
	if err != nil {
		panic(err)
	}
	cleanup = func() { _ = os.RemoveAll(root) }
	srcDir = filepath.Join(root, "source")
	dstDir = filepath.Join(root, "destination")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		cleanup()
		panic(err)
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		cleanup()
		panic(err)
	}
	baseTime := time.Date(2026, 8, 3, 9, 15, 0, 0, time.Local)
	for i, name := range names {
		srcPath := filepath.Join(srcDir, name)
		dstPath := filepath.Join(dstDir, name)
		if err := os.WriteFile(srcPath, []byte("new contents for "+name), 0o644); err != nil {
			cleanup()
			panic(err)
		}
		if err := os.WriteFile(dstPath, []byte("old"), 0o644); err != nil {
			cleanup()
			panic(err)
		}
		if err := os.Truncate(srcPath, 2_400_000+int64(i)*64_000); err != nil {
			cleanup()
			panic(err)
		}
		if err := os.Truncate(dstPath, 2_300_000+int64(i)*32_000); err != nil {
			cleanup()
			panic(err)
		}
		srcTime := baseTime.Add(time.Duration(i) * 6 * time.Second)
		dstTime := baseTime.Add(-53 * time.Second)
		if i%3 == 1 {
			srcTime = baseTime.Add(-2 * time.Minute)
			dstTime = baseTime.Add(time.Duration(i) * time.Second)
		} else if i%3 == 2 {
			dstTime = srcTime
		}
		if err := os.Chtimes(srcPath, srcTime, srcTime); err != nil {
			cleanup()
			panic(err)
		}
		if err := os.Chtimes(dstPath, dstTime, dstTime); err != nil {
			cleanup()
			panic(err)
		}
		sources = append(sources, fileCopySource{Path: srcPath, Name: name})
	}
	return srcDir, dstDir, sources, cleanup
}

func headlessSetAllSourcesNewer(sources []fileCopySource, dstDir string) {
	baseTime := time.Date(2026, 8, 3, 9, 14, 0, 0, time.Local)
	for i, source := range sources {
		dstTime := baseTime.Add(7 * time.Second)
		srcTime := baseTime.Add(time.Minute + time.Duration(i)*6*time.Second)
		if err := os.Chtimes(source.Path, srcTime, srcTime); err != nil {
			panic(err)
		}
		dstPath := filepath.Join(dstDir, source.Name)
		if err := os.Chtimes(dstPath, dstTime, dstTime); err != nil {
			panic(err)
		}
	}
}

func headlessFrameCoversWindow(img *image.RGBA) bool {
	if img == nil {
		return false
	}
	bounds := img.Bounds()
	opaque := 0
	painted := 0
	sampled := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 8 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 8 {
			sampled++
			pixel := img.RGBAAt(x, y)
			if pixel.A >= 250 {
				opaque++
			}
			if int(pixel.R)+int(pixel.G)+int(pixel.B) > 2 {
				painted++
			}
		}
	}
	return sampled > 0 && opaque*1000/sampled >= 995 && painted*100/sampled >= 99
}
