package ui

import (
	"errors"
	"hexone/filesys"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
)

type copyEndpoint struct {
	pane   int
	remote *paneSSHSession
	dir    string
}

func copyEndpointFromPane(idx int, pane *filePaneState) copyEndpoint {
	if pane == nil {
		return copyEndpoint{pane: idx}
	}
	return copyEndpoint{
		pane:   idx,
		remote: pane.remote,
		dir:    strings.TrimSpace(pane.dir),
	}
}

func (ep copyEndpoint) isRemote() bool {
	return ep.remote != nil
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
	return os.Stat(p)
}

func inspectCopyPaths(srcEp copyEndpoint, srcPath string, dstEp copyEndpoint, dstRaw string) (string, fileCopyPathInfo, fileCopyPathInfo, error) {
	srcNorm, err := srcEp.normalizeSourcePath(srcPath)
	if err != nil {
		return "", fileCopyPathInfo{}, fileCopyPathInfo{}, err
	}
	srcStat, err := endpointLstat(srcEp, srcNorm)
	if err != nil {
		return "", fileCopyPathInfo{}, fileCopyPathInfo{}, err
	}

	dstNorm, err := dstEp.normalizePath(dstRaw)
	if err != nil {
		return "", fileCopyPathInfo{}, fileCopyPathInfo{}, err
	}
	if dstDirInfo, err := endpointStat(dstEp, dstNorm); err == nil && dstDirInfo.IsDir() {
		dstNorm = dstEp.join(dstNorm, srcEp.baseName(srcNorm))
	}

	if endpointSamePath(srcEp, srcNorm, dstEp, dstNorm) {
		return "", fileCopyPathInfo{}, fileCopyPathInfo{}, errors.New("source and destination are the same")
	}
	if srcStat.IsDir() && srcEp.isRemote() && dstEp.isRemote() &&
		sameSSHRemoteTarget(srcEp.remote.setup, dstEp.remote.setup) &&
		endpointPathWithin(dstEp, dstNorm, srcNorm) {
		return "", fileCopyPathInfo{}, fileCopyPathInfo{}, errors.New("destination cannot be inside source directory")
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
	return dstNorm, srcInfo, dstInfo, nil
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

func runCopyBetweenEndpoints(srcEp copyEndpoint, srcPath string, dstEp copyEndpoint, dstPath string, report func(filesys.CopyProgress)) error {
	if !srcEp.isRemote() && !dstEp.isRemote() {
		return filesys.CopyPath(srcPath, dstPath, report)
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

	entries, bytesTotal, err := collectTransferEntries(srcEp, srcNorm, srcInfo)
	if err != nil {
		return err
	}

	progress := filesys.CopyProgress{
		EntriesTotal: len(entries),
		BytesTotal:   bytesTotal,
	}
	reportCopyProgress(report, progress)

	for _, entry := range entries {
		progress.CurrentPath = entry.srcPath
		reportCopyProgress(report, progress)

		dstEntryPath := dstEp.join(dstNorm, entry.rel)
		if err := copyTransferEntry(srcEp, dstEp, entry, dstEntryPath, &progress, report); err != nil {
			return err
		}
		progress.EntriesDone++
		reportCopyProgress(report, progress)
	}

	return nil
}

func collectTransferEntries(srcEp copyEndpoint, srcRoot string, srcInfo os.FileInfo) ([]transferEntry, int64, error) {
	if srcEp.isRemote() {
		return collectRemoteTransferEntries(srcEp.remote.sftpClient(), srcRoot, srcInfo)
	}
	return collectLocalTransferEntries(srcRoot, srcInfo)
}

func collectLocalTransferEntries(srcRoot string, srcInfo os.FileInfo) ([]transferEntry, int64, error) {
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
		return []transferEntry{entry}, bytesTotal, nil
	}

	entries := make([]transferEntry, 0, 64)
	var bytesTotal int64
	err := filepath.WalkDir(srcRoot, func(curr string, d fs.DirEntry, walkErr error) error {
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
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return entries, bytesTotal, nil
}

func collectRemoteTransferEntries(client sftpClientLike, srcRoot string, srcInfo os.FileInfo) ([]transferEntry, int64, error) {
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
		return []transferEntry{entry}, bytesTotal, nil
	}

	entries := make([]transferEntry, 0, 64)
	var bytesTotal int64
	var walk func(curr, rel string, info os.FileInfo) error
	walk = func(curr, rel string, info os.FileInfo) error {
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
		if !info.IsDir() || entry.isSymlink {
			return nil
		}
		items, err := client.ReadDir(curr)
		if err != nil {
			return err
		}
		for _, item := range items {
			name := item.Name()
			childPath := path.Join(curr, name)
			childInfo, err := client.Lstat(childPath)
			if err != nil {
				return err
			}
			childRel := name
			if rel != "." {
				childRel = rel + "/" + name
			}
			if err := walk(childPath, childRel, childInfo); err != nil {
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
	switch {
	case entry.isDir:
		return createDirAtEndpoint(dstEp, dstPath, entry.mode, entry.modTime)
	case entry.isSymlink:
		return copySymlinkToEndpoint(dstEp, dstPath, entry.symlinkDest)
	default:
		return copyRegularToEndpoint(srcEp, dstEp, entry, dstPath, progress, report)
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

func copyRegularToEndpoint(srcEp, dstEp copyEndpoint, entry transferEntry, dstPath string, progress *filesys.CopyProgress, report func(filesys.CopyProgress)) error {
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

	buf := make([]byte, 1<<20)
	for {
		nr, readErr := in.Read(buf)
		if nr > 0 {
			chunk := buf[:nr]
			for len(chunk) > 0 {
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

	if err := applyEndpointFileAttrs(dstEp, dstPath, entry.mode, entry.modTime); err != nil {
		return err
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
	return os.MkdirAll(p, 0o755)
}

type sftpClientLike interface {
	Lstat(string) (os.FileInfo, error)
	Stat(string) (os.FileInfo, error)
	ReadDir(string) ([]os.FileInfo, error)
	Open(string) (*sftp.File, error)
	OpenFile(string, int) (*sftp.File, error)
	MkdirAll(string) error
	Chmod(string, os.FileMode) error
	Chtimes(string, time.Time, time.Time) error
	ReadLink(string) (string, error)
	Symlink(string, string) error
	RemoveAll(string) error
}

func reportCopyProgress(report func(filesys.CopyProgress), progress filesys.CopyProgress) {
	if report != nil {
		report(progress)
	}
}
