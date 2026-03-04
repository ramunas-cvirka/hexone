package filesys

import (
	"os"
	"path/filepath"
)

// DeletePath removes a file or directory tree at path.
func DeletePath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	abs = filepath.Clean(abs)

	info, err := os.Lstat(abs)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return os.Remove(abs)
	}
	return os.RemoveAll(abs)
}
