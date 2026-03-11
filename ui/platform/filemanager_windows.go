//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	fileManagerShell32ProcILCreateFromPath      = windows.NewLazySystemDLL("shell32.dll").NewProc("ILCreateFromPathW")
	fileManagerShell32ProcILFree                = windows.NewLazySystemDLL("shell32.dll").NewProc("ILFree")
	fileManagerShell32ProcSHOpenFolderAndSelect = windows.NewLazySystemDLL("shell32.dll").NewProc("SHOpenFolderAndSelectItems")
	fileManagerCoInitializeAlreadyDoneErr       = windows.Errno(windows.S_FALSE)
	fileManagerCoInitializeChangedModeErr       = windows.Errno(windows.RPC_E_CHANGED_MODE)
)

func SystemFileManagerName() string {
	return "Explorer"
}

func OpenDirectoryInSystemFileManager(dirPath string) error {
	dirPath = strings.TrimSpace(dirPath)
	if dirPath == "" {
		return errors.New("directory path is empty")
	}
	return openDirectoryInSystemFileManagerCommand(dirPath).Start()
}

func RevealPathInSystemFileManager(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("path is empty")
	}
	target := normalizeWindowsFileManagerPath(path)
	if err := revealPathInSystemFileManagerShell(target); err == nil {
		return nil
	}
	return revealPathInSystemFileManagerCommand(target).Start()
}

func openDirectoryInSystemFileManagerCommand(dirPath string) *exec.Cmd {
	target := normalizeWindowsFileManagerPath(dirPath)
	cmd := exec.Command("cmd", "/c", "start", "", target)
	configureViewerCommandProcess(cmd)
	return cmd
}

func revealPathInSystemFileManagerCommand(path string) *exec.Cmd {
	target := normalizeWindowsFileManagerPath(path)
	cmd := exec.Command("explorer.exe", `/select,"`+target+`"`)
	configureViewerCommandProcess(cmd)
	return cmd
}

func normalizeWindowsFileManagerPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil && strings.TrimSpace(abs) != "" {
		path = abs
	}
	return filepath.Clean(path)
}

func revealPathInSystemFileManagerShell(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	coUninit := initFileManagerCOM()
	if coUninit != nil {
		defer coUninit()
	}
	pidl, err := fileManagerShellItemIDList(path)
	if err != nil {
		return err
	}
	defer fileManagerShell32ProcILFree.Call(pidl)

	hr, _, _ := fileManagerShell32ProcSHOpenFolderAndSelect.Call(pidl, 0, 0, 0)
	if int32(hr) != 0 {
		return fmt.Errorf("SHOpenFolderAndSelectItems failed: 0x%x", uint32(hr))
	}
	return nil
}

func initFileManagerCOM() func() {
	err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	switch {
	case err == nil:
		return windows.CoUninitialize
	case errors.Is(err, fileManagerCoInitializeAlreadyDoneErr):
		return nil
	case errors.Is(err, fileManagerCoInitializeChangedModeErr):
		return nil
	default:
		return nil
	}
}

func fileManagerShellItemIDList(path string) (uintptr, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	pidl, _, callErr := fileManagerShell32ProcILCreateFromPath.Call(uintptr(unsafe.Pointer(ptr)))
	if pidl == 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return 0, fmt.Errorf("ILCreateFromPathW failed: %w", callErr)
		}
		return 0, errors.New("ILCreateFromPathW returned nil")
	}
	return pidl, nil
}
