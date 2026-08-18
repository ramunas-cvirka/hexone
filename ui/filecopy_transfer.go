// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hexone/filesys"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
)

var errRemoteCommandUnavailable = errors.New("remote command session is unavailable")

type copyEndpoint struct {
	pane    int
	remote  *paneSSHSession
	dir     string
	archive bool
}

func copyEndpointFromPane(idx int, pane *filePaneState) copyEndpoint {
	if pane == nil {
		return copyEndpoint{pane: idx}
	}
	return copyEndpoint{
		pane:    idx,
		remote:  pane.remote,
		dir:     strings.TrimSpace(pane.dir),
		archive: pane.archiveBrowsing(),
	}
}

func (ep copyEndpoint) isRemote() bool {
	return ep.remote != nil
}

func (ep copyEndpoint) isArchive() bool {
	return !ep.isRemote() && ep.archive
}

func (ep copyEndpoint) normalizePath(raw string) (string, error) {
	txt := strings.TrimSpace(raw)
	if ep.isRemote() {
		if txt == "" {
			return "", errors.New("path is empty")
		}
		base := ep.dir
		if base == "" {
			base = "/"
		}
		if !path.IsAbs(txt) {
			txt = path.Join(base, txt)
		}
		txt = path.Clean(txt)
		if txt == "." || txt == "" {
			txt = "/"
		}
		if !strings.HasPrefix(txt, "/") {
			txt = "/" + txt
		}
		return txt, nil
	}
	if ep.isArchive() {
		return "", errors.New("destination cannot be inside an archive")
	}
	if txt == "" {
		return "", errors.New("path is empty")
	}
	abs, err := filepath.Abs(txt)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func (ep copyEndpoint) normalizeSourcePath(raw string) (string, error) {
	txt := strings.TrimSpace(raw)
	if ep.isRemote() {
		if txt == "" {
			return "", errors.New("source path is empty")
		}
		if !path.IsAbs(txt) {
			base := ep.dir
			if base == "" {
				base = "/"
			}
			txt = path.Join(base, txt)
		}
		txt = path.Clean(txt)
		if txt == "." || txt == "" {
			txt = "/"
		}
		if !strings.HasPrefix(txt, "/") {
			txt = "/" + txt
		}
		return txt, nil
	}
	if ep.isArchive() {
		if txt == "" {
			return "", errors.New("source path is empty")
		}
		if !filepath.IsAbs(txt) {
			base := ep.dir
			if strings.TrimSpace(base) == "" {
				return "", errors.New("archive source path is empty")
			}
			txt = filepath.Join(base, txt)
		}
		return filepath.Clean(txt), nil
	}
	abs, err := filepath.Abs(txt)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func (ep copyEndpoint) baseName(p string) string {
	if ep.isRemote() {
		return path.Base(path.Clean(p))
	}
	return filepath.Base(filepath.Clean(p))
}

func (ep copyEndpoint) dirName(p string) string {
	if ep.isRemote() {
		return path.Dir(path.Clean(p))
	}
	return filepath.Dir(filepath.Clean(p))
}

func (ep copyEndpoint) join(base, rel string) string {
	if ep.isRemote() {
		if rel == "." || rel == "" {
			return path.Clean(base)
		}
		return path.Join(base, rel)
	}
	if rel == "." || rel == "" {
		return filepath.Clean(base)
	}
	return filepath.Join(base, filepath.FromSlash(rel))
}

func (ep copyEndpoint) samePath(a, b string) bool {
	if ep.isRemote() {
		return path.Clean(a) == path.Clean(b)
	}
	return samePath(a, b)
}

func endpointSamePath(a copyEndpoint, aPath string, b copyEndpoint, bPath string) bool {
	if a.isRemote() != b.isRemote() {
		return false
	}
	if a.isRemote() {
		if a.remote == nil || b.remote == nil {
			return false
		}
		if !sameSSHRemoteTarget(a.remote.setup, b.remote.setup) {
			return false
		}
		return path.Clean(aPath) == path.Clean(bPath)
	}
	return samePath(aPath, bPath)
}

func endpointPathWithin(ep copyEndpoint, pathVal, root string) bool {
	if ep.isRemote() {
		p := path.Clean(pathVal)
		r := path.Clean(root)
		if p == r {
			return true
		}
		prefix := r
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		return strings.HasPrefix(p, prefix)
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(pathVal))
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	prefix := ".." + string(filepath.Separator)
	return rel != ".." && !strings.HasPrefix(rel, prefix)
}

func endpointLstat(ep copyEndpoint, p string) (os.FileInfo, error) {
	if ep.isRemote() {
		client := ep.remote.sftpClient()
		if client == nil {
			return nil, errors.New("sftp session is not connected")
		}
		return client.Lstat(p)
	}
	if ep.isArchive() {
		return filesys.LstatLocalPath(p)
	}
	return os.Lstat(p)
}

func endpointStat(ep copyEndpoint, p string) (os.FileInfo, error) {
	if ep.isRemote() {
		client := ep.remote.sftpClient()
		if client == nil {
			return nil, errors.New("sftp session is not connected")
		}
		return client.Stat(p)
	}
	if ep.isArchive() {
		return filesys.StatLocalPath(p)
	}
	return os.Stat(p)
}

func inspectCopyPaths(srcEp copyEndpoint, srcPath string, dstEp copyEndpoint, dstRaw string) (string, fileCopyPathInfo, fileCopyPathInfo, error) {
	dstNorm, _, srcInfo, dstInfo, err := resolveCopyPaths(srcEp, srcPath, dstEp, dstRaw)
	return dstNorm, srcInfo, dstInfo, err
}

func resolveCopyPaths(srcEp copyEndpoint, srcPath string, dstEp copyEndpoint, dstRaw string) (string, os.FileInfo, fileCopyPathInfo, fileCopyPathInfo, error) {
	srcNorm, err := srcEp.normalizeSourcePath(srcPath)
	if err != nil {
		return "", nil, fileCopyPathInfo{}, fileCopyPathInfo{}, err
	}
	srcStat, err := endpointLstat(srcEp, srcNorm)
	if err != nil {
		return "", nil, fileCopyPathInfo{}, fileCopyPathInfo{}, err
	}

	dstNorm, err := dstEp.normalizePath(dstRaw)
	if err != nil {
		return "", nil, fileCopyPathInfo{}, fileCopyPathInfo{}, err
	}
	if dstDirInfo, err := endpointStat(dstEp, dstNorm); err == nil && dstDirInfo.IsDir() {
		dstNorm = dstEp.join(dstNorm, srcEp.baseName(srcNorm))
	}

	if endpointSamePath(srcEp, srcNorm, dstEp, dstNorm) {
		return "", nil, fileCopyPathInfo{}, fileCopyPathInfo{}, errors.New("source and destination are the same")
	}
	if srcStat.IsDir() && endpointsShareFilesystem(srcEp, dstEp) &&
		endpointPathWithin(dstEp, dstNorm, srcNorm) {
		return "", nil, fileCopyPathInfo{}, fileCopyPathInfo{}, errors.New("destination cannot be inside source directory")
	}

	srcInfo := fileCopyPathInfo{
		Path:    srcNorm,
		Exists:  true,
		IsDir:   srcStat.IsDir(),
		ModTime: srcStat.ModTime(),
	}
	if srcStat.Mode().IsRegular() {
		srcInfo.Size = srcStat.Size()
	}

	dstInfo := fileCopyPathInfo{Path: dstNorm}
	if dstStat, err := endpointLstat(dstEp, dstNorm); err == nil {
		dstInfo.Exists = true
		dstInfo.IsDir = dstStat.IsDir()
		dstInfo.ModTime = dstStat.ModTime()
		if dstStat.Mode().IsRegular() {
			dstInfo.Size = dstStat.Size()
		}
	}
	return dstNorm, srcStat, srcInfo, dstInfo, nil
}

func endpointsShareFilesystem(a, b copyEndpoint) bool {
	if a.isRemote() || b.isRemote() {
		return a.isRemote() && b.isRemote() && a.remote != nil && b.remote != nil &&
			sameSSHRemoteTarget(a.remote.setup, b.remote.setup)
	}
	return !a.isArchive() && !b.isArchive()
}

func endpointsUseSameRemote(a, b copyEndpoint) bool {
	return a.isRemote() && b.isRemote() && a.remote != nil && b.remote != nil &&
		sameSSHRemoteTarget(a.remote.setup, b.remote.setup)
}

func runSameRemoteCopyContext(ctx context.Context, srcEp copyEndpoint, srcPath string, dstEp copyEndpoint, dstPath string) (string, error) {
	if !endpointsUseSameRemote(srcEp, dstEp) {
		return "", errRemoteCommandUnavailable
	}
	effectiveDst, srcStat, srcInfo, _, err := resolveCopyPaths(srcEp, srcPath, dstEp, dstPath)
	if err != nil {
		return "", err
	}
	command := sameRemoteCopyCommand(srcInfo.Path, effectiveDst, srcStat.IsDir())
	if err := runRemoteShellCommandContext(ctx, srcEp.remote, command); err != nil {
		return "", err
	}
	return effectiveDst, nil
}

func sameRemoteCopyCommand(srcPath, dstPath string, isDir bool) string {
	parentCmd := "mkdir -p -- " + shellQuote(path.Dir(path.Clean(dstPath)))
	copyCmd := "cp -a -- " + shellQuote(path.Clean(srcPath)) + " " + shellQuote(path.Clean(dstPath))
	if isDir {
		sourceContents := strings.TrimSuffix(path.Clean(srcPath), "/") + "/."
		copyCmd = "mkdir -p -- " + shellQuote(path.Clean(dstPath)) +
			" && cp -a -- " + shellQuote(sourceContents) + " " + shellQuote(path.Clean(dstPath)+"/")
	}
	return parentCmd + " && " + copyCmd
}

func runRemoteShellCommandContext(ctx context.Context, remote *paneSSHSession, command string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if remote == nil || remote.commandClient() == nil {
		return errRemoteCommandUnavailable
	}
	session, err := remote.commandClient().NewSession()
	if err != nil {
		return fmt.Errorf("%w: %v", errRemoteCommandUnavailable, err)
	}
	defer session.Close()
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if err := session.Start(command); err != nil {
		return fmt.Errorf("%w: %v", errRemoteCommandUnavailable, err)
	}
	done := make(chan error, 1)
	go func() { done <- session.Wait() }()
	select {
	case <-ctx.Done():
		_ = session.Close()
		<-done
		return ctx.Err()
	case err := <-done:
		if err == nil {
			return nil
		}
		message := strings.TrimSpace(strings.TrimSpace(stderr.String()) + "\n" + strings.TrimSpace(stdout.String()))
		if remoteShellCommandUnsupported(message) {
			return fmt.Errorf("%w: %s", errRemoteCommandUnavailable, message)
		}
		if message == "" {
			return fmt.Errorf("remote command failed: %w", err)
		}
		return fmt.Errorf("remote command failed: %s", message)
	}
}

func remoteShellCommandUnsupported(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "command not found") ||
		strings.Contains(lower, "not recognized as an internal or external command") ||
		strings.Contains(lower, "not recognized as the name of a cmdlet") ||
		strings.Contains(lower, "illegal option --") ||
		strings.Contains(lower, "unknown option --")
}

type transferEntry struct {
	srcPath     string
	rel         string
	mode        os.FileMode
	modTime     time.Time
	size        int64
	isDir       bool
	isSymlink   bool
	symlinkDest string
}

type streamedTransferEntry struct {
	entry      transferEntry
	dstRoot    string
	sourceRoot string
}

const streamingCopyQueueSize = 32

// Match OpenSSH's default number of outstanding SFTP transfer requests. Each
// request carries up to pkg/sftp's default 32 KiB packet, keeping enough data
// in flight to avoid making upload throughput proportional to one packet per
// network round trip.
const sftpUploadConcurrency = 64

type streamingCopyProgress struct {
	mu       sync.Mutex
	progress filesys.CopyProgress
	report   func(filesys.CopyProgress)
}

func newStreamingCopyProgress(report func(filesys.CopyProgress)) *streamingCopyProgress {
	return &streamingCopyProgress{
		progress: filesys.CopyProgress{Streaming: true},
		report:   report,
	}
}

func (p *streamingCopyProgress) update(fn func(*filesys.CopyProgress)) {
	if p == nil {
		return
	}
	p.mu.Lock()
	fn(&p.progress)
	snapshot := p.progress
	reportCopyProgress(p.report, snapshot)
	p.mu.Unlock()
}

func (p *streamingCopyProgress) discovered(entry transferEntry) {
	if entry.isDir {
		return
	}
	p.update(func(progress *filesys.CopyProgress) {
		progress.FilesDiscovered++
		progress.EntriesTotal = progress.FilesDiscovered
	})
}

func (p *streamingCopyProgress) scanFinished() {
	p.update(func(progress *filesys.CopyProgress) { progress.ScanDone = true })
}

func (p *streamingCopyProgress) startFile(entry transferEntry, sourceRoot string) {
	p.update(func(progress *filesys.CopyProgress) {
		progress.CurrentPath = entry.srcPath
		progress.CurrentRootPath = sourceRoot
		progress.CurrentBytesDone = 0
		progress.CurrentBytesTotal = 0
		if entry.mode.IsRegular() {
			progress.CurrentBytesTotal = entry.size
		}
	})
}

func (p *streamingCopyProgress) fileBytes(done int64) {
	p.update(func(progress *filesys.CopyProgress) {
		delta := done - progress.CurrentBytesDone
		if delta > 0 {
			progress.BytesDone += delta
		}
		progress.CurrentBytesDone = done
	})
}

func (p *streamingCopyProgress) fileFinished(entry transferEntry) {
	p.update(func(progress *filesys.CopyProgress) {
		progress.FilesCopied++
		progress.EntriesDone = progress.FilesCopied
		if entry.mode.IsRegular() {
			progress.CurrentBytesDone = entry.size
		}
	})
}

func runStreamingCopyContext(ctx context.Context, srcEp copyEndpoint, sources []fileCopySource, srcPath string, dstEp copyEndpoint, dstRaw string, multi bool, report func(filesys.CopyProgress)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	progress := newStreamingCopyProgress(report)
	reportCopyProgress(report, progress.progress)
	items := make(chan streamedTransferEntry, streamingCopyQueueSize)
	producerDone := make(chan error, 1)

	go func() {
		defer close(items)
		destinationDir := dstRaw
		if multi {
			var err error
			destinationDir, _, err = inspectCopyDestinationDir(dstEp, dstRaw)
			if err != nil {
				producerDone <- err
				return
			}
		}
		streamSources := sources
		if !multi {
			streamSources = []fileCopySource{{Path: srcPath, Name: srcEp.baseName(srcPath)}}
		}
		for _, source := range streamSources {
			target := destinationDir
			if multi {
				target = dstEp.join(destinationDir, srcEp.baseName(source.Path))
			}
			effectiveDst, srcStat, srcInfo, _, err := resolveCopyPaths(srcEp, source.Path, dstEp, target)
			if err != nil {
				producerDone <- err
				return
			}
			err = streamTransferEntriesContext(ctx, srcEp, srcInfo.Path, srcStat, func(entry transferEntry) error {
				progress.discovered(entry)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case items <- streamedTransferEntry{entry: entry, dstRoot: effectiveDst, sourceRoot: srcInfo.Path}:
					return nil
				}
			})
			if err != nil {
				producerDone <- err
				return
			}
		}
		progress.scanFinished()
		producerDone <- nil
	}()

	for item := range items {
		entry := item.entry
		dstPath := dstEp.join(item.dstRoot, entry.rel)
		perFile := filesys.CopyProgress{}
		if !entry.isDir {
			progress.startFile(entry, item.sourceRoot)
		}
		err := copyTransferEntryContext(ctx, srcEp, dstEp, entry, dstPath, &perFile, func(fileProgress filesys.CopyProgress) {
			progress.fileBytes(fileProgress.BytesDone)
		})
		if err != nil {
			cancel()
			return err
		}
		if !entry.isDir {
			progress.fileFinished(entry)
		}
	}
	return <-producerDone
}

func streamTransferEntriesContext(ctx context.Context, srcEp copyEndpoint, srcRoot string, srcInfo os.FileInfo, emit func(transferEntry) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if srcEp.isRemote() {
		return streamRemoteTransferEntriesContext(ctx, srcEp.remote.sftpClient(), srcRoot, srcInfo, emit)
	}
	if srcEp.isArchive() {
		return streamArchiveTransferEntriesContext(ctx, srcRoot, srcInfo, emit)
	}
	return streamLocalTransferEntriesContext(ctx, srcRoot, srcInfo, emit)
}

func streamLocalTransferEntriesContext(ctx context.Context, srcRoot string, srcInfo os.FileInfo, emit func(transferEntry) error) error {
	return filepath.WalkDir(srcRoot, func(curr string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(curr)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, curr)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		entry := transferEntry{
			srcPath: curr, rel: rel, mode: info.Mode(), modTime: info.ModTime(), size: info.Size(),
			isDir: info.IsDir(), isSymlink: info.Mode()&os.ModeSymlink != 0,
		}
		if entry.isSymlink {
			entry.symlinkDest, err = os.Readlink(curr)
			if err != nil {
				return err
			}
			entry.size = 0
		}
		return emit(entry)
	})
}

func streamArchiveTransferEntriesContext(ctx context.Context, srcRoot string, srcInfo os.FileInfo, emit func(transferEntry) error) error {
	loc, ok := filesys.ParseArchivePath(srcRoot)
	if !ok {
		return errors.New("source path is not inside an archive")
	}
	fsys, _, err := filesys.OpenArchiveFS(srcRoot)
	if err != nil {
		return err
	}
	rootInner := loc.InnerPath
	if rootInner == "" {
		rootInner = "."
	}
	return fs.WalkDir(fsys, rootInner, func(curr string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel := "."
		if curr != rootInner {
			if rootInner == "." {
				rel = curr
			} else {
				rel = strings.TrimPrefix(curr, rootInner+"/")
			}
		}
		entry := transferEntry{
			srcPath: archiveDisplayPath(loc.ArchivePath, curr), rel: path.Clean(rel),
			mode: normalizeArchiveEntryMode(info.Mode(), info.IsDir()), modTime: info.ModTime(),
			size: info.Size(), isDir: info.IsDir(),
		}
		return emit(entry)
	})
}

func streamRemoteTransferEntriesContext(ctx context.Context, client sftpWalkerLike, srcRoot string, srcInfo os.FileInfo, emit func(transferEntry) error) error {
	if client == nil {
		return errors.New("sftp session is not connected")
	}
	var walk func(string, string, os.FileInfo) error
	walk = func(curr, rel string, info os.FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry := transferEntry{
			srcPath: curr, rel: rel, mode: info.Mode(), modTime: info.ModTime(), size: info.Size(),
			isDir: info.IsDir(), isSymlink: info.Mode()&os.ModeSymlink != 0,
		}
		if entry.isSymlink {
			var err error
			entry.symlinkDest, err = client.ReadLink(curr)
			if err != nil {
				return err
			}
			entry.size = 0
		}
		if err := emit(entry); err != nil {
			return err
		}
		if !entry.isDir || entry.isSymlink {
			return nil
		}
		children, err := client.ReadDir(curr)
		if err != nil {
			return err
		}
		for _, child := range children {
			childRel := child.Name()
			if rel != "." {
				childRel = rel + "/" + child.Name()
			}
			if err := walk(path.Join(curr, child.Name()), childRel, child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(srcRoot, ".", srcInfo)
}

func runCopyBetweenEndpoints(ctx context.Context, srcEp copyEndpoint, srcPath string, dstEp copyEndpoint, dstPath string, report func(filesys.CopyProgress)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !srcEp.isRemote() && !dstEp.isRemote() && !srcEp.isArchive() && !dstEp.isArchive() {
		return filesys.CopyPathContext(ctx, srcPath, dstPath, report)
	}

	srcNorm, err := srcEp.normalizeSourcePath(srcPath)
	if err != nil {
		return err
	}
	dstNorm, err := dstEp.normalizePath(dstPath)
	if err != nil {
		return err
	}

	srcInfo, err := endpointLstat(srcEp, srcNorm)
	if err != nil {
		return err
	}
	if dstDirInfo, err := endpointStat(dstEp, dstNorm); err == nil && dstDirInfo.IsDir() {
		dstNorm = dstEp.join(dstNorm, srcEp.baseName(srcNorm))
	}

	if endpointSamePath(srcEp, srcNorm, dstEp, dstNorm) {
		return errors.New("source and destination are the same")
	}
	if srcInfo.IsDir() && srcEp.isRemote() && dstEp.isRemote() &&
		sameSSHRemoteTarget(srcEp.remote.setup, dstEp.remote.setup) &&
		endpointPathWithin(dstEp, dstNorm, srcNorm) {
		return errors.New("destination cannot be inside source directory")
	}

	entries, bytesTotal, err := collectTransferEntriesContext(ctx, srcEp, srcNorm, srcInfo)
	if err != nil {
		return err
	}

	progress := filesys.CopyProgress{
		EntriesTotal: len(entries),
		BytesTotal:   bytesTotal,
	}
	reportCopyProgress(report, progress)

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		progress.CurrentPath = entry.srcPath
		reportCopyProgress(report, progress)

		dstEntryPath := dstEp.join(dstNorm, entry.rel)
		if err := copyTransferEntryContext(ctx, srcEp, dstEp, entry, dstEntryPath, &progress, report); err != nil {
			return err
		}
		progress.EntriesDone++
		reportCopyProgress(report, progress)
	}

	return nil
}

func collectTransferEntries(srcEp copyEndpoint, srcRoot string, srcInfo os.FileInfo) ([]transferEntry, int64, error) {
	return collectTransferEntriesContext(context.Background(), srcEp, srcRoot, srcInfo)
}

func collectTransferEntriesContext(ctx context.Context, srcEp copyEndpoint, srcRoot string, srcInfo os.FileInfo) ([]transferEntry, int64, error) {
	return collectTransferEntriesContextWithProgress(ctx, srcEp, srcRoot, srcInfo, nil)
}

type transferCollectProgress func(entries int, bytes int64, current string)

func collectTransferEntriesContextWithProgress(ctx context.Context, srcEp copyEndpoint, srcRoot string, srcInfo os.FileInfo, report transferCollectProgress) ([]transferEntry, int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if srcEp.isRemote() {
		return collectRemoteTransferEntriesContextWithProgress(ctx, srcEp.remote.sftpClient(), srcRoot, srcInfo, report)
	}
	if srcEp.isArchive() {
		return collectArchiveTransferEntriesContextWithProgress(ctx, srcRoot, srcInfo, report)
	}
	return collectLocalTransferEntriesContextWithProgress(ctx, srcRoot, srcInfo, report)
}

func collectArchiveTransferEntriesContextWithProgress(ctx context.Context, srcRoot string, srcInfo os.FileInfo, report transferCollectProgress) ([]transferEntry, int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	loc, ok := filesys.ParseArchivePath(srcRoot)
	if !ok {
		return nil, 0, errors.New("source path is not inside an archive")
	}
	fsys, _, err := filesys.OpenArchiveFS(srcRoot)
	if err != nil {
		return nil, 0, err
	}
	rootInner := loc.InnerPath
	if rootInner == "" {
		rootInner = "."
	}

	if !srcInfo.IsDir() {
		mode := normalizeArchiveEntryMode(srcInfo.Mode(), false)
		entry := transferEntry{
			srcPath: srcRoot,
			rel:     ".",
			mode:    mode,
			modTime: srcInfo.ModTime(),
			size:    srcInfo.Size(),
			isDir:   false,
		}
		bytesTotal := int64(0)
		if srcInfo.Mode().IsRegular() {
			bytesTotal = srcInfo.Size()
		}
		if report != nil {
			report(1, bytesTotal, srcRoot)
		}
		return []transferEntry{entry}, bytesTotal, nil
	}

	entries := make([]transferEntry, 0, 64)
	var bytesTotal int64
	err = fs.WalkDir(fsys, rootInner, func(curr string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := normalizeArchiveEntryMode(info.Mode(), info.IsDir())
		rel := "."
		if curr != rootInner {
			if rootInner == "." {
				rel = curr
			} else {
				rel = strings.TrimPrefix(curr, rootInner+"/")
			}
		}
		rel = path.Clean(rel)
		if rel == "" {
			rel = "."
		}
		entry := transferEntry{
			srcPath: archiveDisplayPath(loc.ArchivePath, curr),
			rel:     rel,
			mode:    mode,
			modTime: info.ModTime(),
			size:    info.Size(),
			isDir:   info.IsDir(),
		}
		if info.Mode().IsRegular() {
			bytesTotal += info.Size()
		}
		entries = append(entries, entry)
		if report != nil {
			report(len(entries), bytesTotal, entry.srcPath)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return entries, bytesTotal, nil
}

func normalizeArchiveEntryMode(mode os.FileMode, isDir bool) os.FileMode {
	if mode.Perm() != 0 {
		return mode
	}
	if isDir {
		return (mode &^ os.ModePerm) | 0o755
	}
	return (mode &^ os.ModePerm) | 0o644
}

func archiveDisplayPath(archivePath, innerPath string) string {
	if innerPath == "" || innerPath == "." {
		return filepath.Clean(archivePath)
	}
	return filepath.Join(filepath.Clean(archivePath), filepath.FromSlash(innerPath))
}

func collectLocalTransferEntriesContextWithProgress(ctx context.Context, srcRoot string, srcInfo os.FileInfo, report transferCollectProgress) ([]transferEntry, int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if !srcInfo.IsDir() {
		entry := transferEntry{
			srcPath:   srcRoot,
			rel:       ".",
			mode:      srcInfo.Mode(),
			modTime:   srcInfo.ModTime(),
			size:      srcInfo.Size(),
			isDir:     false,
			isSymlink: srcInfo.Mode()&os.ModeSymlink != 0,
		}
		if entry.isSymlink {
			target, err := os.Readlink(srcRoot)
			if err != nil {
				return nil, 0, err
			}
			entry.symlinkDest = target
			entry.size = 0
		}
		bytesTotal := int64(0)
		if srcInfo.Mode().IsRegular() {
			bytesTotal = srcInfo.Size()
		}
		if report != nil {
			report(1, bytesTotal, srcRoot)
		}
		return []transferEntry{entry}, bytesTotal, nil
	}

	entries := make([]transferEntry, 0, 64)
	var bytesTotal int64
	err := filepath.WalkDir(srcRoot, func(curr string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(curr)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, curr)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "" {
			rel = "."
		}
		entry := transferEntry{
			srcPath:   curr,
			rel:       rel,
			mode:      info.Mode(),
			modTime:   info.ModTime(),
			size:      info.Size(),
			isDir:     info.IsDir(),
			isSymlink: info.Mode()&os.ModeSymlink != 0,
		}
		if entry.isSymlink {
			target, err := os.Readlink(curr)
			if err != nil {
				return err
			}
			entry.symlinkDest = target
			entry.size = 0
		}
		if info.Mode().IsRegular() {
			bytesTotal += info.Size()
		}
		entries = append(entries, entry)
		if report != nil {
			report(len(entries), bytesTotal, entry.srcPath)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return entries, bytesTotal, nil
}

type sftpWalkerLike interface {
	ReadDir(string) ([]os.FileInfo, error)
	ReadLink(string) (string, error)
}

func collectRemoteTransferEntriesContext(ctx context.Context, client sftpWalkerLike, srcRoot string, srcInfo os.FileInfo) ([]transferEntry, int64, error) {
	return collectRemoteTransferEntriesContextWithProgress(ctx, client, srcRoot, srcInfo, nil)
}

func collectRemoteTransferEntriesContextWithProgress(ctx context.Context, client sftpWalkerLike, srcRoot string, srcInfo os.FileInfo, report transferCollectProgress) ([]transferEntry, int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if client == nil {
		return nil, 0, errors.New("sftp session is not connected")
	}
	if !srcInfo.IsDir() {
		entry := transferEntry{
			srcPath:   srcRoot,
			rel:       ".",
			mode:      srcInfo.Mode(),
			modTime:   srcInfo.ModTime(),
			size:      srcInfo.Size(),
			isDir:     false,
			isSymlink: srcInfo.Mode()&os.ModeSymlink != 0,
		}
		if entry.isSymlink {
			target, err := client.ReadLink(srcRoot)
			if err != nil {
				return nil, 0, err
			}
			entry.symlinkDest = target
			entry.size = 0
		}
		bytesTotal := int64(0)
		if srcInfo.Mode().IsRegular() {
			bytesTotal = srcInfo.Size()
		}
		if report != nil {
			report(1, bytesTotal, srcRoot)
		}
		return []transferEntry{entry}, bytesTotal, nil
	}

	entries := make([]transferEntry, 0, 64)
	var bytesTotal int64
	var walk func(curr, rel string, info os.FileInfo) error
	walk = func(curr, rel string, info os.FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry := transferEntry{
			srcPath:   curr,
			rel:       rel,
			mode:      info.Mode(),
			modTime:   info.ModTime(),
			size:      info.Size(),
			isDir:     info.IsDir(),
			isSymlink: info.Mode()&os.ModeSymlink != 0,
		}
		if entry.isSymlink {
			target, err := client.ReadLink(curr)
			if err != nil {
				return err
			}
			entry.symlinkDest = target
			entry.size = 0
		}
		if info.Mode().IsRegular() {
			bytesTotal += info.Size()
		}
		entries = append(entries, entry)
		if report != nil {
			report(len(entries), bytesTotal, entry.srcPath)
		}
		if !info.IsDir() || entry.isSymlink {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		items, err := client.ReadDir(curr)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := ctx.Err(); err != nil {
				return err
			}
			name := item.Name()
			childPath := path.Join(curr, name)
			childRel := name
			if rel != "." {
				childRel = rel + "/" + name
			}
			// ReadDir already returned the child's SFTP attributes. Reusing them
			// avoids one full network round trip per directory entry.
			if err := walk(childPath, childRel, item); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(srcRoot, ".", srcInfo); err != nil {
		return nil, 0, err
	}
	return entries, bytesTotal, nil
}

func copyTransferEntry(srcEp, dstEp copyEndpoint, entry transferEntry, dstPath string, progress *filesys.CopyProgress, report func(filesys.CopyProgress)) error {
	return copyTransferEntryContext(context.Background(), srcEp, dstEp, entry, dstPath, progress, report)
}

func copyTransferEntryContext(ctx context.Context, srcEp, dstEp copyEndpoint, entry transferEntry, dstPath string, progress *filesys.CopyProgress, report func(filesys.CopyProgress)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	switch {
	case entry.isDir:
		return createDirAtEndpoint(dstEp, dstPath, entry.mode, entry.modTime)
	case entry.isSymlink:
		return copySymlinkToEndpoint(dstEp, dstPath, entry.symlinkDest)
	default:
		return copyRegularToEndpoint(ctx, srcEp, dstEp, entry, dstPath, progress, report)
	}
}

func createDirAtEndpoint(ep copyEndpoint, dstPath string, mode os.FileMode, modTime time.Time) error {
	if ep.isRemote() {
		client := ep.remote.sftpClient()
		if client == nil {
			return errors.New("sftp session is not connected")
		}
		if err := client.MkdirAll(dstPath); err != nil {
			return err
		}
		if err := client.Chmod(dstPath, mode.Perm()); err != nil {
			return err
		}
		return client.Chtimes(dstPath, modTime, modTime)
	}
	if err := os.MkdirAll(dstPath, mode.Perm()); err != nil {
		return err
	}
	if err := os.Chmod(dstPath, mode.Perm()); err != nil {
		return err
	}
	return os.Chtimes(dstPath, modTime, modTime)
}

func copySymlinkToEndpoint(ep copyEndpoint, dstPath, target string) error {
	if err := removeEndpointPathIfExists(ep, dstPath); err != nil {
		return err
	}
	parent := ep.dirName(dstPath)
	if err := ensureEndpointDir(ep, parent); err != nil {
		return err
	}
	if ep.isRemote() {
		client := ep.remote.sftpClient()
		if client == nil {
			return errors.New("sftp session is not connected")
		}
		return client.Symlink(target, dstPath)
	}
	return os.Symlink(target, dstPath)
}

func copyRegularToEndpoint(ctx context.Context, srcEp, dstEp copyEndpoint, entry transferEntry, dstPath string, progress *filesys.CopyProgress, report func(filesys.CopyProgress)) error {
	parent := dstEp.dirName(dstPath)
	if err := ensureEndpointDir(dstEp, parent); err != nil {
		return err
	}

	in, err := openEndpointReader(srcEp, entry.srcPath)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := openEndpointWriter(dstEp, dstPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if dstEp.isRemote() && !srcEp.isRemote() {
		remoteOut, ok := out.(*sftp.File)
		if !ok {
			return errors.New("sftp destination is not a remote file")
		}
		if err := copyToSFTPContext(ctx, remoteOut, in, progress, report); err != nil {
			return err
		}
	} else if srcEp.isRemote() && !dstEp.isRemote() {
		remoteIn, ok := in.(*sftp.File)
		if !ok {
			return errors.New("sftp source is not a remote file")
		}
		if err := copyFromSFTPContext(ctx, out, remoteIn, progress, report); err != nil {
			return err
		}
	} else if err := copyRegularStreamContext(ctx, out, in, progress, report); err != nil {
		return err
	}

	if err := applyEndpointFileAttrs(dstEp, dstPath, entry.mode, entry.modTime); err != nil {
		return err
	}
	return nil
}

type copyProgressReader struct {
	ctx      context.Context
	reader   io.Reader
	progress *filesys.CopyProgress
	report   func(filesys.CopyProgress)
}

func (r *copyProgressReader) Read(buf []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(buf)
	if n > 0 {
		r.progress.BytesDone += int64(n)
		reportCopyProgress(r.report, *r.progress)
	}
	return n, err
}

func copyToSFTPContext(ctx context.Context, dst *sftp.File, src io.Reader, progress *filesys.CopyProgress, report func(filesys.CopyProgress)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if dst == nil {
		return errors.New("sftp destination file is not open")
	}
	reader := &copyProgressReader{
		ctx:      ctx,
		reader:   src,
		progress: progress,
		report:   report,
	}
	if _, err := dst.ReadFromWithConcurrency(reader, sftpUploadConcurrency); err != nil {
		// Concurrent writes can complete out of order. pkg/sftp leaves the file
		// offset at the earliest failed request, so truncate there to remove any
		// later writes and preserve the same contiguous-partial-file behavior as
		// the former sequential upload path.
		if offset, seekErr := dst.Seek(0, io.SeekCurrent); seekErr == nil {
			_ = dst.Truncate(offset)
		}
		return err
	}
	return nil
}

type copyProgressWriter struct {
	ctx      context.Context
	writer   io.Writer
	progress *filesys.CopyProgress
	report   func(filesys.CopyProgress)
}

func (w *copyProgressWriter) Write(buf []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := w.writer.Write(buf)
	if n > 0 {
		w.progress.BytesDone += int64(n)
		reportCopyProgress(w.report, *w.progress)
	}
	return n, err
}

func copyFromSFTPContext(ctx context.Context, dst io.Writer, src *sftp.File, progress *filesys.CopyProgress, report func(filesys.CopyProgress)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if src == nil {
		return errors.New("sftp source file is not open")
	}
	writer := &copyProgressWriter{
		ctx:      ctx,
		writer:   dst,
		progress: progress,
		report:   report,
	}
	_, err := src.WriteTo(writer)
	return err
}

func copyRegularStreamContext(ctx context.Context, out io.Writer, in io.Reader, progress *filesys.CopyProgress, report func(filesys.CopyProgress)) error {
	buf := make([]byte, 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		nr, readErr := in.Read(buf)
		if nr > 0 {
			chunk := buf[:nr]
			for len(chunk) > 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
				nw, writeErr := out.Write(chunk)
				if nw > 0 {
					chunk = chunk[nw:]
					progress.BytesDone += int64(nw)
					reportCopyProgress(report, *progress)
				}
				if writeErr != nil {
					return writeErr
				}
				if nw == 0 {
					return io.ErrShortWrite
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

func openEndpointReader(ep copyEndpoint, p string) (io.ReadCloser, error) {
	if ep.isRemote() {
		client := ep.remote.sftpClient()
		if client == nil {
			return nil, errors.New("sftp session is not connected")
		}
		return client.Open(p)
	}
	if ep.isArchive() {
		reader, _, err := filesys.OpenLocalPath(p)
		return reader, err
	}
	return os.Open(p)
}

func openEndpointWriter(ep copyEndpoint, p string) (io.WriteCloser, error) {
	if ep.isRemote() {
		client := ep.remote.sftpClient()
		if client == nil {
			return nil, errors.New("sftp session is not connected")
		}
		return client.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	}
	if ep.isArchive() {
		return nil, errors.New("cannot write into an archive")
	}
	return os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
}

func applyEndpointFileAttrs(ep copyEndpoint, p string, mode os.FileMode, modTime time.Time) error {
	if ep.isRemote() {
		client := ep.remote.sftpClient()
		if client == nil {
			return errors.New("sftp session is not connected")
		}
		if err := client.Chmod(p, mode.Perm()); err != nil {
			return err
		}
		return client.Chtimes(p, modTime, modTime)
	}
	if ep.isArchive() {
		return errors.New("cannot modify archive contents")
	}
	if err := os.Chmod(p, mode.Perm()); err != nil {
		return err
	}
	return os.Chtimes(p, modTime, modTime)
}

func removeEndpointPathIfExists(ep copyEndpoint, p string) error {
	if ep.isRemote() {
		client := ep.remote.sftpClient()
		if client == nil {
			return errors.New("sftp session is not connected")
		}
		if _, err := client.Lstat(p); err != nil {
			return nil
		}
		return client.RemoveAll(p)
	}
	if ep.isArchive() {
		return errors.New("cannot modify archive contents")
	}
	if _, err := os.Lstat(p); err != nil {
		return nil
	}
	return os.RemoveAll(p)
}

func ensureEndpointDir(ep copyEndpoint, p string) error {
	if ep.isRemote() {
		client := ep.remote.sftpClient()
		if client == nil {
			return errors.New("sftp session is not connected")
		}
		return client.MkdirAll(p)
	}
	if ep.isArchive() {
		return errors.New("cannot create directories inside an archive")
	}
	return os.MkdirAll(p, 0o755)
}

func reportCopyProgress(report func(filesys.CopyProgress), progress filesys.CopyProgress) {
	if report != nil {
		report(progress)
	}
}
