// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"context"
	"fmt"
	"hexone/filesys"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/sftp"
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

type concurrentSFTPWriteProbe struct {
	base sftp.FileWriter

	mu          sync.Mutex
	active      int
	maxActive   int
	overlapped  chan struct{}
	overlapOnce sync.Once
}

func newConcurrentSFTPWriteProbe(base sftp.FileWriter) *concurrentSFTPWriteProbe {
	return &concurrentSFTPWriteProbe{
		base:       base,
		overlapped: make(chan struct{}),
	}
}

func (p *concurrentSFTPWriteProbe) Filewrite(request *sftp.Request) (io.WriterAt, error) {
	writer, err := p.base.Filewrite(request)
	if err != nil {
		return nil, err
	}
	return &concurrentSFTPWriter{WriterAt: writer, probe: p}, nil
}

func (p *concurrentSFTPWriteProbe) maximumActive() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.maxActive
}

type concurrentSFTPWriter struct {
	io.WriterAt
	probe *concurrentSFTPWriteProbe
}

func (w *concurrentSFTPWriter) WriteAt(data []byte, offset int64) (int, error) {
	w.probe.mu.Lock()
	w.probe.active++
	if w.probe.active > w.probe.maxActive {
		w.probe.maxActive = w.probe.active
	}
	if w.probe.active >= 2 {
		w.probe.overlapOnce.Do(func() { close(w.probe.overlapped) })
	}
	w.probe.mu.Unlock()

	select {
	case <-w.probe.overlapped:
	case <-time.After(2 * time.Second):
		w.probe.mu.Lock()
		w.probe.active--
		w.probe.mu.Unlock()
		return 0, fmt.Errorf("SFTP writes did not overlap")
	}

	n, err := w.WriterAt.WriteAt(data, offset)
	w.probe.mu.Lock()
	w.probe.active--
	w.probe.mu.Unlock()
	return n, err
}

type concurrentSFTPReadProbe struct {
	base sftp.FileReader

	mu          sync.Mutex
	active      int
	maxActive   int
	overlapped  chan struct{}
	overlapOnce sync.Once
}

func newConcurrentSFTPReadProbe(base sftp.FileReader) *concurrentSFTPReadProbe {
	return &concurrentSFTPReadProbe{
		base:       base,
		overlapped: make(chan struct{}),
	}
}

func (p *concurrentSFTPReadProbe) Fileread(request *sftp.Request) (io.ReaderAt, error) {
	reader, err := p.base.Fileread(request)
	if err != nil {
		return nil, err
	}
	return &concurrentSFTPReader{ReaderAt: reader, probe: p}, nil
}

func (p *concurrentSFTPReadProbe) maximumActive() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.maxActive
}

type concurrentSFTPReader struct {
	io.ReaderAt
	probe *concurrentSFTPReadProbe
}

func (r *concurrentSFTPReader) ReadAt(data []byte, offset int64) (int, error) {
	r.probe.mu.Lock()
	r.probe.active++
	if r.probe.active > r.probe.maxActive {
		r.probe.maxActive = r.probe.active
	}
	if r.probe.active >= 2 {
		r.probe.overlapOnce.Do(func() { close(r.probe.overlapped) })
	}
	r.probe.mu.Unlock()

	select {
	case <-r.probe.overlapped:
	case <-time.After(2 * time.Second):
		r.probe.mu.Lock()
		r.probe.active--
		r.probe.mu.Unlock()
		return 0, fmt.Errorf("SFTP reads did not overlap")
	}

	n, err := r.ReaderAt.ReadAt(data, offset)
	r.probe.mu.Lock()
	r.probe.active--
	r.probe.mu.Unlock()
	return n, err
}

func newTestSFTPClient(t *testing.T, handlers sftp.Handlers) *sftp.Client {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	server := sftp.NewRequestServer(serverConn, handlers)
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve() }()

	client, err := sftp.NewClientPipe(clientConn, clientConn)
	if err != nil {
		_ = server.Close()
		t.Fatalf("NewClientPipe: %v", err)
	}
	t.Cleanup(func() {
		_ = server.Close()
		_ = client.Close()
		select {
		case <-serverDone:
		case <-time.After(2 * time.Second):
			t.Error("SFTP request server did not stop")
		}
	})
	return client
}

func TestCopyToSFTPContextPipelinesWritesAndReportsProgress(t *testing.T) {
	handlers := sftp.InMemHandler()
	probe := newConcurrentSFTPWriteProbe(handlers.FilePut)
	handlers.FilePut = probe
	client := newTestSFTPClient(t, handlers)

	remoteFile, err := client.OpenFile("/upload.bin", os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	data := bytes.Repeat([]byte("hexone-sftp-pipeline\n"), 32<<10)
	progress := filesys.CopyProgress{BytesTotal: int64(len(data))}
	var reports []int64
	if err := copyToSFTPContext(context.Background(), remoteFile, bytes.NewReader(data), &progress, func(p filesys.CopyProgress) {
		reports = append(reports, p.BytesDone)
	}); err != nil {
		_ = remoteFile.Close()
		t.Fatalf("copyToSFTPContext: %v", err)
	}
	if err := remoteFile.Close(); err != nil {
		t.Fatalf("Close upload: %v", err)
	}

	if got := probe.maximumActive(); got < 2 {
		t.Fatalf("maximum concurrent SFTP writes = %d, want at least 2", got)
	}
	if got, want := progress.BytesDone, int64(len(data)); got != want {
		t.Fatalf("progress bytes = %d, want %d", got, want)
	}
	if len(reports) == 0 || reports[len(reports)-1] != int64(len(data)) {
		t.Fatalf("progress reports = %v, want final value %d", reports, len(data))
	}
	for i := 1; i < len(reports); i++ {
		if reports[i] < reports[i-1] {
			t.Fatalf("progress reports are not monotonic: %v", reports)
		}
	}

	download, err := client.Open("/upload.bin")
	if err != nil {
		t.Fatalf("Open download: %v", err)
	}
	got, readErr := io.ReadAll(download)
	closeErr := download.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read uploaded file: read=%v close=%v", readErr, closeErr)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("uploaded data differs: got %d bytes, want %d", len(got), len(data))
	}
}

func TestCopyRegularToRemoteEndpointUsesPipelinedUpload(t *testing.T) {
	handlers := sftp.InMemHandler()
	probe := newConcurrentSFTPWriteProbe(handlers.FilePut)
	handlers.FilePut = probe
	client := newTestSFTPClient(t, handlers)
	remote := &paneSSHSession{conn: newSharedSSHConn(sshClientBundle{sftp: client})}

	src := filepath.Join(t.TempDir(), "source.bin")
	data := bytes.Repeat([]byte("remote endpoint pipeline\n"), 24<<10)
	if err := os.WriteFile(src, data, 0o640); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	info, err := os.Stat(src)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	entry := transferEntry{
		srcPath: src,
		rel:     ".",
		mode:    info.Mode(),
		modTime: info.ModTime(),
		size:    info.Size(),
	}
	progress := filesys.CopyProgress{BytesTotal: info.Size()}
	if err := copyRegularToEndpoint(
		context.Background(),
		copyEndpoint{},
		copyEndpoint{remote: remote},
		entry,
		"/endpoint-copy.bin",
		&progress,
		nil,
	); err != nil {
		t.Fatalf("copyRegularToEndpoint: %v", err)
	}
	if got := probe.maximumActive(); got < 2 {
		t.Fatalf("maximum concurrent SFTP writes = %d, want at least 2", got)
	}
	if got, want := progress.BytesDone, info.Size(); got != want {
		t.Fatalf("progress bytes = %d, want %d", got, want)
	}
}

func TestCopyFromSFTPContextPipelinesReadsAndReportsProgress(t *testing.T) {
	handlers := sftp.InMemHandler()
	probe := newConcurrentSFTPReadProbe(handlers.FileGet)
	handlers.FileGet = probe
	client := newTestSFTPClient(t, handlers)

	data := bytes.Repeat([]byte("hexone-sftp-download-pipeline\n"), 32<<10)
	upload, err := client.OpenFile("/download.bin", os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		t.Fatalf("OpenFile upload: %v", err)
	}
	if _, err := upload.Write(data); err != nil {
		_ = upload.Close()
		t.Fatalf("seed remote file: %v", err)
	}
	if err := upload.Close(); err != nil {
		t.Fatalf("Close upload: %v", err)
	}

	remoteFile, err := client.Open("/download.bin")
	if err != nil {
		t.Fatalf("Open download: %v", err)
	}
	var dst bytes.Buffer
	progress := filesys.CopyProgress{BytesTotal: int64(len(data))}
	var reports []int64
	if err := copyFromSFTPContext(context.Background(), &dst, remoteFile, &progress, func(p filesys.CopyProgress) {
		reports = append(reports, p.BytesDone)
	}); err != nil {
		_ = remoteFile.Close()
		t.Fatalf("copyFromSFTPContext: %v", err)
	}
	if err := remoteFile.Close(); err != nil {
		t.Fatalf("Close download: %v", err)
	}

	if got := probe.maximumActive(); got < 2 {
		t.Fatalf("maximum concurrent SFTP reads = %d, want at least 2", got)
	}
	if got, want := progress.BytesDone, int64(len(data)); got != want {
		t.Fatalf("progress bytes = %d, want %d", got, want)
	}
	if len(reports) == 0 || reports[len(reports)-1] != int64(len(data)) {
		t.Fatalf("progress reports = %v, want final value %d", reports, len(data))
	}
	for i := 1; i < len(reports); i++ {
		if reports[i] < reports[i-1] {
			t.Fatalf("progress reports are not monotonic: %v", reports)
		}
	}
	if !bytes.Equal(dst.Bytes(), data) {
		t.Fatalf("downloaded data differs: got %d bytes, want %d", dst.Len(), len(data))
	}
}

func TestCopyRegularFromRemoteEndpointUsesPipelinedDownload(t *testing.T) {
	handlers := sftp.InMemHandler()
	probe := newConcurrentSFTPReadProbe(handlers.FileGet)
	handlers.FileGet = probe
	client := newTestSFTPClient(t, handlers)
	remote := &paneSSHSession{conn: newSharedSSHConn(sshClientBundle{sftp: client})}

	data := bytes.Repeat([]byte("remote download endpoint pipeline\n"), 24<<10)
	upload, err := client.OpenFile("/endpoint-download.bin", os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		t.Fatalf("OpenFile upload: %v", err)
	}
	if _, err := upload.Write(data); err != nil {
		_ = upload.Close()
		t.Fatalf("seed remote file: %v", err)
	}
	if err := upload.Close(); err != nil {
		t.Fatalf("Close upload: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "destination.bin")
	entry := transferEntry{
		srcPath: "/endpoint-download.bin",
		rel:     ".",
		mode:    0o640,
		modTime: time.Unix(1700000000, 0),
		size:    int64(len(data)),
	}
	progress := filesys.CopyProgress{BytesTotal: int64(len(data))}
	if err := copyRegularToEndpoint(
		context.Background(),
		copyEndpoint{remote: remote},
		copyEndpoint{},
		entry,
		dst,
		&progress,
		nil,
	); err != nil {
		t.Fatalf("copyRegularToEndpoint: %v", err)
	}
	if got := probe.maximumActive(); got < 2 {
		t.Fatalf("maximum concurrent SFTP reads = %d, want at least 2", got)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("downloaded data differs: got %d bytes, want %d", len(got), len(data))
	}
}

func TestCopyProgressReaderStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	progress := filesys.CopyProgress{BytesDone: 7}
	var reports []int64
	reader := &copyProgressReader{
		ctx:      ctx,
		reader:   strings.NewReader("payload"),
		progress: &progress,
		report: func(p filesys.CopyProgress) {
			reports = append(reports, p.BytesDone)
		},
	}

	buf := make([]byte, 4)
	if n, err := reader.Read(buf); n != 4 || err != nil {
		t.Fatalf("first read = %d, %v; want 4, nil", n, err)
	}
	cancel()
	if n, err := reader.Read(buf); n != 0 || err != context.Canceled {
		t.Fatalf("canceled read = %d, %v; want 0, context.Canceled", n, err)
	}
	if got, want := progress.BytesDone, int64(11); got != want {
		t.Fatalf("progress bytes = %d, want %d", got, want)
	}
	if len(reports) != 1 || reports[0] != 11 {
		t.Fatalf("progress reports = %v, want [11]", reports)
	}
}

func TestCopyProgressWriterStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	progress := filesys.CopyProgress{BytesDone: 7}
	var reports []int64
	var dst bytes.Buffer
	writer := &copyProgressWriter{
		ctx:      ctx,
		writer:   &dst,
		progress: &progress,
		report: func(p filesys.CopyProgress) {
			reports = append(reports, p.BytesDone)
		},
	}

	if n, err := writer.Write([]byte("data")); n != 4 || err != nil {
		t.Fatalf("first write = %d, %v; want 4, nil", n, err)
	}
	cancel()
	if n, err := writer.Write([]byte("more")); n != 0 || err != context.Canceled {
		t.Fatalf("canceled write = %d, %v; want 0, context.Canceled", n, err)
	}
	if got, want := progress.BytesDone, int64(11); got != want {
		t.Fatalf("progress bytes = %d, want %d", got, want)
	}
	if len(reports) != 1 || reports[0] != 11 {
		t.Fatalf("progress reports = %v, want [11]", reports)
	}
	if got := dst.String(); got != "data" {
		t.Fatalf("destination = %q, want data", got)
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
