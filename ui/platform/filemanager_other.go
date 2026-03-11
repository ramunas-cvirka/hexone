//go:build !windows

package platform

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func SystemFileManagerName() string {
	if runtime.GOOS == "darwin" {
		return "Finder"
	}
	desktop := linuxDesktopSignature()
	switch {
	case strings.Contains(desktop, "kde") && linuxCommandAvailable("dolphin"):
		return "Dolphin"
	case (strings.Contains(desktop, "gnome") || strings.Contains(desktop, "ubuntu") || strings.Contains(desktop, "unity") || strings.Contains(desktop, "pantheon")) && linuxCommandAvailable("nautilus"):
		return "Files"
	case strings.Contains(desktop, "cinnamon") && linuxCommandAvailable("nemo"):
		return "Nemo"
	case strings.Contains(desktop, "xfce") && linuxCommandAvailable("thunar"):
		return "Thunar"
	default:
		return "File Manager"
	}
}

func OpenDirectoryInSystemFileManager(dirPath string) error {
	dirPath = strings.TrimSpace(dirPath)
	if dirPath == "" {
		return errors.New("directory path is empty")
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", dirPath)
	} else {
		cmd = exec.Command("xdg-open", dirPath)
	}
	return cmd.Start()
}

func RevealPathInSystemFileManager(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("path is empty")
	}
	if runtime.GOOS == "darwin" {
		return exec.Command("open", "-R", path).Start()
	}
	if cmd, ok := linuxRevealInSystemFileManagerCommand(path); ok {
		return cmd.Start()
	}
	dirPath := filepath.Dir(path)
	if strings.TrimSpace(dirPath) == "" || dirPath == "." {
		dirPath = path
	}
	return OpenDirectoryInSystemFileManager(dirPath)
}

func linuxRevealInSystemFileManagerCommand(path string) (*exec.Cmd, bool) {
	desktop := linuxDesktopSignature()
	switch {
	case strings.Contains(desktop, "kde"):
		if _, err := exec.LookPath("dolphin"); err == nil {
			return exec.Command("dolphin", "--select", path), true
		}
	case strings.Contains(desktop, "gnome"), strings.Contains(desktop, "ubuntu"), strings.Contains(desktop, "unity"), strings.Contains(desktop, "pantheon"):
		if _, err := exec.LookPath("nautilus"); err == nil {
			return exec.Command("nautilus", "--select", path), true
		}
	}
	if _, err := exec.LookPath("dolphin"); err == nil {
		return exec.Command("dolphin", "--select", path), true
	}
	if _, err := exec.LookPath("nautilus"); err == nil {
		return exec.Command("nautilus", "--select", path), true
	}
	return nil, false
}

func linuxCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func linuxDesktopSignature() string {
	parts := []string{
		os.Getenv("XDG_CURRENT_DESKTOP"),
		os.Getenv("DESKTOP_SESSION"),
	}
	if strings.TrimSpace(os.Getenv("KDE_FULL_SESSION")) != "" {
		parts = append(parts, "kde")
	}
	return strings.ToLower(strings.Join(parts, " "))
}
