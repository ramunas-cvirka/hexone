// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"fmt"
	"hexone/filesys"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDirectFilePasteStatusLineShowsBottomBarProgress(t *testing.T) {
	now := time.Unix(1700000000, 0)
	st := &fileCopyState{
		directPaste: true,
		running:     true,
		srcPath:     "/tmp/movie.mkv",
		startedAt:   now.Add(-2 * time.Second),
		progress: filesys.CopyProgress{
			BytesDone:   50 << 20,
			BytesTotal:  100 << 20,
			CurrentPath: "/tmp/movie.mkv",
		},
	}

	got := directFilePasteStatusLineForWidth(st, now, 2000, func(text string) int { return len(text) })
	for _, want := range []string{"[Pasting] movie.mkv", "50%", "MB/s", "left"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status line %q missing %q", got, want)
		}
	}
}

type transferTestInfo struct {
	name string
	mode os.FileMode
	size int64
}

func (i transferTestInfo) Name() string       { return i.name }
func (i transferTestInfo) Size() int64        { return i.size }
func (i transferTestInfo) Mode() os.FileMode  { return i.mode }
func (i transferTestInfo) ModTime() time.Time { return time.Unix(1, 0) }
func (i transferTestInfo) IsDir() bool        { return i.mode.IsDir() }
func (i transferTestInfo) Sys() any           { return nil }

type transferTestWalker struct {
	tree      map[string][]os.FileInfo
	readCalls []string
}

func (w *transferTestWalker) ReadDir(dir string) ([]os.FileInfo, error) {
	w.readCalls = append(w.readCalls, path.Clean(dir))
	return w.tree[path.Clean(dir)], nil
}

func (*transferTestWalker) ReadLink(string) (string, error) { return "", nil }

func TestCollectRemoteTransferEntriesReusesReadDirMetadata(t *testing.T) {
	walker := &transferTestWalker{tree: map[string][]os.FileInfo{
		"/root": {
			transferTestInfo{name: "sub", mode: os.ModeDir | 0o755},
			transferTestInfo{name: "a.txt", mode: 0o644, size: 3},
		},
		"/root/sub": {
			transferTestInfo{name: "b.txt", mode: 0o644, size: 5},
		},
	}}
	root := transferTestInfo{name: "root", mode: os.ModeDir | 0o755}
	entries, bytesTotal, err := collectRemoteTransferEntriesContext(context.Background(), walker, "/root", root)
	if err != nil {
		t.Fatalf("collectRemoteTransferEntriesContext: %v", err)
	}
	if got, want := len(entries), 4; got != want {
		t.Fatalf("entries = %d, want %d", got, want)
	}
	if got, want := bytesTotal, int64(8); got != want {
		t.Fatalf("bytes = %d, want %d", got, want)
	}
	if got, want := len(walker.readCalls), 2; got != want {
		t.Fatalf("ReadDir calls = %d, want one per directory (%d): %v", got, want, walker.readCalls)
	}
}

func TestSameRemoteCopyCommandQuotesPathsAndMergesDirectory(t *testing.T) {
	got := sameRemoteCopyCommand("/src/it's here", "/dst/new dir", true)
	want := "mkdir -p -- '/dst' && mkdir -p -- '/dst/new dir' && cp -a -- '/src/it'\"'\"'s here/.' '/dst/new dir/'"
	if got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func TestRemoteShellCommandUnsupportedRecognizesPortableFallbacks(t *testing.T) {
	for _, message := range []string{
		"sh: cp: command not found",
		"'rm' is not recognized as an internal or external command",
		"cp: illegal option -- -",
	} {
		if !remoteShellCommandUnsupported(message) {
			t.Fatalf("message should trigger SFTP fallback: %q", message)
		}
	}
	if remoteShellCommandUnsupported("cp: cannot create '/root/x': Permission denied") {
		t.Fatal("permission failure must be reported instead of retried as an unsupported command")
	}
}

func TestCopyProgressTextReportsEnumerationActivity(t *testing.T) {
	if got, want := copyProgressText(filesys.CopyProgress{EntriesDone: 37}, 0), "Preparing... 37 entries found"; got != want {
		t.Fatalf("copyProgressText = %q, want %q", got, want)
	}
}

func TestStreamingCopyStartsBeforeScanFinishes(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "source")
	dst := filepath.Join(root, "destination")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	const fileCount = streamingCopyQueueSize + 48
	for i := 0; i < fileCount; i++ {
		name := filepath.Join(src, fmt.Sprintf("file-%03d.txt", i))
		if err := os.WriteFile(name, []byte("payload"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	var mu sync.Mutex
	copiedBeforeScanFinished := false
	var last filesys.CopyProgress
	err := runStreamingCopyContext(context.Background(), copyEndpoint{}, nil, src, copyEndpoint{}, dst, false, func(progress filesys.CopyProgress) {
		mu.Lock()
		defer mu.Unlock()
		last = progress
		if progress.FilesCopied > 0 && !progress.ScanDone {
			copiedBeforeScanFinished = true
		}
	})
	if err != nil {
		t.Fatalf("runStreamingCopyContext: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dst, fmt.Sprintf("file-%03d.txt", fileCount-1)))
	if err != nil || string(data) != "payload" {
		t.Fatalf("copied payload = %q, err=%v", data, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !copiedBeforeScanFinished {
		t.Fatal("copy did not begin while directory enumeration was still active")
	}
	if !last.ScanDone || last.FilesDiscovered != fileCount || last.FilesCopied != fileCount {
		t.Fatalf("final streaming progress = %+v", last)
	}
}

func TestStreamingCopyProgressUsesCurrentFileForBar(t *testing.T) {
	progress := filesys.CopyProgress{
		Streaming:         true,
		FilesDiscovered:   12,
		FilesCopied:       4,
		CurrentBytesDone:  2 << 20,
		CurrentBytesTotal: 8 << 20,
	}
	if got, want := copyProgressFraction(progress), float32(0.25); got != want {
		t.Fatalf("copyProgressFraction = %v, want %v", got, want)
	}
	if got, want := copyProgressCountText(progress), "4 copied  •  12 discovered"; got != want {
		t.Fatalf("copyProgressCountText = %q, want %q", got, want)
	}
	if got, want := copyProgressTransferText(progress, 1<<20), "2.0 MB / 8.0 MB  •  1.0 MB/s"; got != want {
		t.Fatalf("copyProgressTransferText = %q, want %q", got, want)
	}
	if copyProgressIndeterminate(progress) {
		t.Fatal("8 MiB file should use determinate per-file progress")
	}
	display := buildFileCopyProgressDisplay(progress, 1<<20)
	if got, want := display.PrimaryValue, "2.0 MB / 8.0 MB (25%) @ 1.0 MB/s"; got != want {
		t.Fatalf("large-file processed status = %q, want %q", got, want)
	}
	if got, want := display.SecondaryLabel+": "+display.SecondaryValue, "Remaining: ~00:00:06"; got != want {
		t.Fatalf("large-file secondary status = %q, want %q", got, want)
	}
	progress.CurrentBytesTotal = 24 << 10
	if !copyProgressIndeterminate(progress) {
		t.Fatal("small file should use an indeterminate activity bar")
	}
	progress.BytesDone = 84 << 20
	display = buildFileCopyProgressDisplay(progress, 10<<20)
	if got, want := display.PrimaryValue, "4 files (84.0 MB) @ 10.0 MB/s"; got != want {
		t.Fatalf("small-file processed status = %q, want %q", got, want)
	}
	if got, want := display.SecondaryValue, "12 files..."; got != want {
		t.Fatalf("small-file discovered status = %q, want %q", got, want)
	}
}

func TestFileOperationCountsUseThousandsSeparators(t *testing.T) {
	if got, want := fileOpCountText(8492, "file", "files"), "8,492 files"; got != want {
		t.Fatalf("fileOpCountText = %q, want %q", got, want)
	}
}

func TestTextCellProgressBarMatchesArchiveStyle(t *testing.T) {
	if got, want := textCellProgressBar(0.25, 8), "██░░░░░░"; got != want {
		t.Fatalf("textCellProgressBar = %q, want %q", got, want)
	}
	activity := []rune(textCellActivityBar(time.Unix(0, 0), 12))
	if got, want := len(activity), 12; got != want {
		t.Fatalf("activity bar width = %d, want %d", got, want)
	}
}
