package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"image"
	"testing"
	"time"

	"gioui.org/widget"
)

func TestFilePaneContextMenuSpecForFileUsesNestedSupportedActions(t *testing.T) {
	prevManagerName := systemFileManagerNameFunc
	systemFileManagerNameFunc = func() string { return "Test Manager" }
	defer func() { systemFileManagerNameFunc = prevManagerName }()

	cfg := fm.DefaultConfig()
	cfg.Associations = []fm.AssociationProgram{
		{AppPath: "/Applications/VLC.app", Extensions: []string{".mp4"}},
	}

	ui := NewUI(cfg)
	pane := ui.filePanes[0]
	pane.model = &filePaneModel{
		entries: []filesys.Entry{{
			Path:        "/tmp/movie.mp4",
			DisplayName: "movie.mp4",
			Kind:        filesys.EntryFile,
		}},
		cfg: cfg,
	}
	pane.ctxMenuRow = 0

	spec := ui.filePaneContextMenuSpec(0, pane)
	assertMenuHasLabel(t, spec.Items, "Open")
	assertMenuHasLabel(t, spec.Items, "Reveal in Test Manager")
	openWith := assertMenuHasLabel(t, spec.Items, "Open With")
	if openWith.Submenu == nil {
		t.Fatal("Open With submenu should exist for local files")
	}
	assertMenuHasLabel(t, spec.Items, "Rename")
	ops := assertMenuHasLabel(t, spec.Items, "File Ops")
	if ops.Submenu == nil {
		t.Fatal("File Ops submenu should exist")
	}
	assertMenuHasLabel(t, spec.Items, "Copy Path")
	assertMenuHasLabel(t, spec.Items, "Permissions..")
	assertMenuMissingLabel(t, spec.Items, "Properties")

	assertMenuHasLabel(t, openWith.Submenu.Items, "System Default")
	assertMenuHasLabel(t, ops.Submenu.Items, "Copy..")
	assertMenuHasLabel(t, ops.Submenu.Items, "Move..")
	assertMenuHasLabel(t, ops.Submenu.Items, "Delete..")
}

func TestFilePaneContextMenuSpecForDirectoryIncludesSystemFileManager(t *testing.T) {
	prevManagerName := systemFileManagerNameFunc
	systemFileManagerNameFunc = func() string { return "Test Manager" }
	defer func() { systemFileManagerNameFunc = prevManagerName }()

	ui := NewUI(fm.DefaultConfig())
	pane := ui.filePanes[0]
	pane.model = &filePaneModel{
		entries: []filesys.Entry{{
			Path:        "/tmp/docs",
			DisplayName: "docs",
			Kind:        filesys.EntryDir,
		}},
		cfg: ui.fmCfg,
	}
	pane.ctxMenuRow = 0

	spec := ui.filePaneContextMenuSpec(0, pane)
	assertMenuHasLabel(t, spec.Items, "Open in Test Manager")
	assertMenuMissingLabel(t, spec.Items, "Reveal in Test Manager")
}

func TestFileOpenWithAppsForPathSortsBestMatchFirst(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.Associations = []fm.AssociationProgram{
		{AppPath: "/Applications/Gzip.app", Extensions: []string{".gz"}},
		{AppPath: "/Applications/Archive.app", Extensions: []string{".tar.gz"}},
		{AppPath: "/Applications/Text.app", Extensions: []string{".txt"}},
	}

	apps := fileOpenWithAppsForPath("/tmp/archive.tar.gz", cfg)
	if len(apps) != 3 {
		t.Fatalf("apps len=%d want 3", len(apps))
	}
	if got := apps[0].AppPath; got != "/Applications/Archive.app" {
		t.Fatalf("best match=%q want archive app", got)
	}
	if got := apps[1].AppPath; got != "/Applications/Gzip.app" {
		t.Fatalf("second match=%q want gzip app", got)
	}
}

func TestFilePaneContextCopyPathRemoteUsesSSHURL(t *testing.T) {
	cfg := fm.DefaultConfig()
	pane := newFilePaneState("/", cfg)
	pane.remote = &paneSSHSession{
		setup: fm.SSHSetup{
			User: "ramunas",
			Host: "example.test",
			Port: 2222,
		},
	}
	pane.dir = "/var/log"

	ui := &UI{
		fmCfg:     cfg,
		filePanes: []*filePaneState{pane},
	}

	got := ui.filePaneContextCopyPath(0, -1)
	want := "ssh://ramunas@example.test:2222/var/log"
	if got != want {
		t.Fatalf("copy path=%q want %q", got, want)
	}
}

func TestFileContextMenuParentRectRejectsStalePath(t *testing.T) {
	pane := &filePaneState{
		ctxMenuPath:      nil,
		ctxMenuItemRects: map[string]image.Rectangle{"open-with": image.Rect(10, 20, 40, 60)},
	}
	if rect, ok := fileContextMenuParentRect(pane, 1); ok {
		t.Fatalf("stale parent rect=%v should not resolve without a matching path", rect)
	}
}

func TestFileContextMenuParentRectReturnsMappedRect(t *testing.T) {
	want := image.Rect(10, 20, 40, 60)
	pane := &filePaneState{
		ctxMenuPath:      []string{"open-with"},
		ctxMenuItemRects: map[string]image.Rectangle{"open-with": want},
	}
	got, ok := fileContextMenuParentRect(pane, 1)
	if !ok {
		t.Fatal("parent rect should resolve for a valid submenu path")
	}
	if got != want {
		t.Fatalf("parent rect=%v want %v", got, want)
	}
}

func TestOpenContextMenuResetsItemClickables(t *testing.T) {
	pane := &filePaneState{
		ctxMenuClicks: map[string]*widget.Clickable{
			"system-file-manager": {},
		},
	}

	pane.openContextMenu(3, image.Pt(40, 60), time.Date(2026, time.March, 12, 10, 0, 0, 0, time.UTC))

	if pane.ctxMenuClicks != nil {
		t.Fatal("openContextMenu should reset item clickables")
	}
}

func TestCloseContextMenuResetsItemClickables(t *testing.T) {
	pane := &filePaneState{
		ctxMenuOpen: true,
		ctxMenuClicks: map[string]*widget.Clickable{
			"system-file-manager": {},
		},
	}

	pane.closeContextMenu()

	if pane.ctxMenuClicks != nil {
		t.Fatal("closeContextMenu should reset item clickables")
	}
}

func assertMenuHasLabel(t *testing.T, items []fileContextMenuItem, label string) fileContextMenuItem {
	t.Helper()
	for _, item := range items {
		if item.Separator {
			continue
		}
		if item.Label == label {
			return item
		}
	}
	t.Fatalf("missing menu item %q", label)
	return fileContextMenuItem{}
}

func assertMenuMissingLabel(t *testing.T, items []fileContextMenuItem, label string) {
	t.Helper()
	for _, item := range items {
		if item.Separator {
			continue
		}
		if item.Label == label {
			t.Fatalf("unexpected menu item %q", label)
		}
	}
}
