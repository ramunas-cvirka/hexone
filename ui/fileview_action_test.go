package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"path/filepath"
	"testing"
	"time"
)

func TestViewerAssociatedExtensionMatch(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Associations = []fm.AssociationProgram{
		{AppPath: `C:\Apps\pdf.exe`, Extensions: []string{".pdf"}},
		{AppPath: `C:\Apps\gzip.exe`, Extensions: []string{".gz"}},
		{AppPath: `C:\Apps\archive.exe`, Extensions: []string{".tar.gz"}},
	}

	if assoc, ok := viewerAssociationForPath(`C:\tmp\REPORT.PDF`, cfg); !ok || assoc.Extension != ".pdf" || assoc.AppPath != `C:\Apps\pdf.exe` {
		t.Fatalf("match for pdf = (%+v, %v), want (.pdf, pdf.exe, true)", assoc, ok)
	}
	if assoc, ok := viewerAssociationForPath(`C:\tmp\archive.TAR.GZ`, cfg); !ok || assoc.Extension != ".tar.gz" || assoc.AppPath != `C:\Apps\archive.exe` {
		t.Fatalf("match for tar.gz = (%+v, %v), want (.tar.gz, archive.exe, true)", assoc, ok)
	}
	if assoc, ok := viewerAssociationForPath(`C:\tmp\archive.gz`, cfg); !ok || assoc.Extension != ".gz" || assoc.AppPath != `C:\Apps\gzip.exe` {
		t.Fatalf("match for gz = (%+v, %v), want (.gz, gzip.exe, true)", assoc, ok)
	}
}

func TestStartFileViewActionUsesAssociatedApp(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.fmCfg.Associations = []fm.AssociationProgram{
		{AppPath: `C:\Apps\pdf.exe`, Extensions: []string{".pdf"}},
	}

	pane := ui.filePanes[0]
	pane.model = &filePaneModel{
		entries: []filesys.Entry{{
			Path:        `C:\tmp\report.pdf`,
			DisplayName: "report.pdf",
			Kind:        filesys.EntryFile,
		}},
		cfg: ui.fmCfg,
	}
	pane.table.Selected = 0

	var openedApp string
	var openedPath string
	prevConfigured := openFileWithConfiguredAppFunc
	prevSystem := openFileWithSystemAssociationFunc
	openFileWithConfiguredAppFunc = func(appPath, filePath string) error {
		openedApp = appPath
		openedPath = filePath
		return nil
	}
	openFileWithSystemAssociationFunc = func(filePath string) error {
		t.Fatalf("system association should not be used, got %q", filePath)
		return nil
	}
	defer func() {
		openFileWithConfiguredAppFunc = prevConfigured
		openFileWithSystemAssociationFunc = prevSystem
	}()

	ui.startFileExternalOpenAction(0, time.Now())

	if openedApp != `C:\Apps\pdf.exe` {
		t.Fatalf("opened app = %q, want pdf.exe path", openedApp)
	}
	if openedPath != `C:\tmp\report.pdf` {
		t.Fatalf("opened path = %q, want report.pdf path", openedPath)
	}
	if ui.fileViewer != nil {
		t.Fatal("associated app open should not start internal viewer")
	}
	if pane.noticeText != "" {
		t.Fatalf("associated app open should stay quiet, got notice %q", pane.noticeText)
	}
}

func TestStartFileExternalOpenFallsBackToSystemAssociation(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())

	pane := ui.filePanes[0]
	pane.model = &filePaneModel{
		entries: []filesys.Entry{{
			Path:        `C:\tmp\report.txt`,
			DisplayName: "report.txt",
			Kind:        filesys.EntryFile,
		}},
		cfg: ui.fmCfg,
	}
	pane.table.Selected = 0

	var openedPath string
	prevConfigured := openFileWithConfiguredAppFunc
	prevSystem := openFileWithSystemAssociationFunc
	openFileWithConfiguredAppFunc = func(appPath, filePath string) error {
		t.Fatalf("configured app should not be used, got %q %q", appPath, filePath)
		return nil
	}
	openFileWithSystemAssociationFunc = func(filePath string) error {
		openedPath = filePath
		return nil
	}
	defer func() {
		openFileWithConfiguredAppFunc = prevConfigured
		openFileWithSystemAssociationFunc = prevSystem
	}()

	ui.startFileExternalOpenAction(0, time.Now())

	if openedPath != `C:\tmp\report.txt` {
		t.Fatalf("opened path = %q, want report.txt path", openedPath)
	}
	if pane.noticeText != "" {
		t.Fatalf("system association open should stay quiet, got notice %q", pane.noticeText)
	}
}

func TestDoubleClickFileUsesSystemAssociationOnly(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.fmCfg.Associations = []fm.AssociationProgram{
		{AppPath: `C:\Apps\pdf.exe`, Extensions: []string{".pdf"}},
	}

	pane := ui.filePanes[0]
	pane.model = &filePaneModel{
		entries: []filesys.Entry{{
			Path:        `C:\tmp\report.pdf`,
			DisplayName: "report.pdf",
			Kind:        filesys.EntryFile,
		}},
		cfg: ui.fmCfg,
	}
	pane.table.Selected = 0

	var openedPath string
	prevConfigured := openFileWithConfiguredAppFunc
	prevSystem := openFileWithSystemAssociationFunc
	openFileWithConfiguredAppFunc = func(appPath, filePath string) error {
		t.Fatalf("configured app should not be used on double click, got %q %q", appPath, filePath)
		return nil
	}
	openFileWithSystemAssociationFunc = func(filePath string) error {
		openedPath = filePath
		return nil
	}
	defer func() {
		openFileWithConfiguredAppFunc = prevConfigured
		openFileWithSystemAssociationFunc = prevSystem
	}()

	if !ui.activateFilePaneDoubleClick(0, 0) {
		t.Fatal("activateFilePaneDoubleClick returned false")
	}

	if openedPath != `C:\tmp\report.pdf` {
		t.Fatalf("opened path = %q, want report.pdf path", openedPath)
	}
	if pane.noticeText != "" {
		t.Fatalf("system association double click should stay quiet, got notice %q", pane.noticeText)
	}
}

func TestDoubleClickDirectoryStillNavigates(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	root := t.TempDir()
	target := filepath.Join(root, "docs")

	pane.dir = root
	pane.model = &filePaneModel{
		entries: []filesys.Entry{{
			Path:        target,
			DisplayName: "docs",
			Kind:        filesys.EntryDir,
			CanEnter:    true,
		}},
		cfg: ui.fmCfg,
	}
	pane.table.Selected = 0

	prevConfigured := openFileWithConfiguredAppFunc
	prevSystem := openFileWithSystemAssociationFunc
	openFileWithConfiguredAppFunc = func(appPath, filePath string) error {
		t.Fatalf("configured app should not be used for directories, got %q %q", appPath, filePath)
		return nil
	}
	openFileWithSystemAssociationFunc = func(filePath string) error {
		t.Fatalf("system association should not be used for directories, got %q", filePath)
		return nil
	}
	defer func() {
		openFileWithConfiguredAppFunc = prevConfigured
		openFileWithSystemAssociationFunc = prevSystem
	}()

	if !ui.activateFilePaneDoubleClick(0, 0) {
		t.Fatal("activateFilePaneDoubleClick returned false")
	}
	if !pane.loading {
		t.Fatal("double click on directory should trigger navigation load")
	}
	if got, want := pane.loadingDir, filepath.Clean(target); got != want {
		t.Fatalf("loading dir = %q, want %q", got, want)
	}
}
