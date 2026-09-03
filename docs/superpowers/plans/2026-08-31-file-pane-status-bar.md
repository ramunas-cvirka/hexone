# File Pane Status Bar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a compact per-pane status bar at the bottom of each file pane showing the cursor entry's size, date and configurable extra fields, so brief mode no longer hides that information; move free space into it from the floating opposite-pane badge.

**Architecture:** The pane already has a bottom status strip (`layoutFilePaneStatusBar`) used by archive extraction and paste progress. This adds a third, lowest-priority branch to that strip: a line assembled from configurable fields, degraded to fit the pane width by dropping fields in a fixed order. Owner/group is resolved once during the async directory listing, never in the layout path. A new `status_bar:` config block drives visibility and field selection, with a new settings tab.

**Tech Stack:** Go 1.26, Gio (immediate-mode GUI), `gopkg.in/yaml.v3`, `github.com/pkg/sftp`.

**Spec:** `docs/superpowers/specs/2026-08-31-file-pane-status-bar-design.md`

---

## Orientation for the implementer

Read `ARCHITECTURE.md` and `CONTRIBUTING.md` before starting. The three facts that matter most here:

1. **Gio is immediate-mode.** Every frame re-runs layout from the root. State that must survive a frame lives on a struct; everything else is recomputed. Never cache derived values "for performance" without measuring.
2. **Never call slow functions from a layout path.** This is why owner/group is resolved during listing, not when drawing the bar.
3. **Platform differences use build tags and `_windows.go` / `_other.go` sibling files**, never `runtime.GOOS` branches in shared code.

Every new `.go` file needs this header (or run `make headers`):

```go
// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0
```

The full test matrix, all of which must pass before the work is done:

```bash
go test ./...
go test -tags pdfium ./ui/
go test -tags uiverify ./ui/
make unused
```

---

## File Structure

| File | Status | Responsibility |
| --- | --- | --- |
| `fm/config.go` | Modify | `StatusBarConfig` type, defaults, normalisation |
| `fm/config_test.go` | Modify | Config round-trip and normalisation tests |
| `filesys/listing.go` | Modify | `Entry.OwnerText` field; populate it |
| `filesys/listing_sftp.go` | Modify | Populate `OwnerText` numerically for remote entries |
| `filesys/owner.go` | Create | uid/gid → name resolution with cache; formatting |
| `filesys/owner_other.go` | Create | `statOwnerIDs` for non-Windows (`*syscall.Stat_t`, `*sftp.FileStat`) |
| `filesys/owner_windows.go` | Create | `statOwnerIDs` for Windows (`*sftp.FileStat` only) |
| `filesys/owner_test.go` | Create | Owner extraction, cache, numeric fallback |
| `ui/filepane_status_fields.go` | Create | Field enum, per-field value builders, line assembly, degradation |
| `ui/filepane_status_fields_test.go` | Create | Per-field rendering and line assembly tests |
| `ui/filepane_status_bar.go` | Create | Status strip chrome (moved) + the file info branch |
| `ui/filepane_status_bar_test.go` | Create | Branch priority, visibility predicate, degradation |
| `ui/filepane_volume_badge.go` | Modify | Remove moved chrome; leave badge and volume lookup |
| `ui/filepane_volume_badge_test.go` | Modify | Badge suppression when the free field is on |
| `ui/filemanager_layout.go` | Modify | `filePaneVolumeBadgesHidden` gains the free-field condition |
| `ui/settings_filepanes.go` | Modify | New "Status bar" tab and its layout |
| `ui/settings_modal.go` | Modify | Widget state, load from config, apply to config |
| `ui/settings_modal_keyboard.go` | Modify | Focus constants, traversal order, checkbox toggling |
| `ui/settings_dirty.go` | Modify | Draft signature includes the new settings |
| `ui/filepane_status_bar_headless_verify_test.go` | Create | `uiverify` pixel test |
| `HELP.md`, `CHANGELOG.md`, `ARCHITECTURE.md` | Modify | Documentation |

**Task order rationale:** config first (everything depends on it), then owner (independent, in a different package), then the field/line logic (pure functions, easily TDD'd), then layout wiring, then settings UI, then docs. Each task ends green and committed.

---

## Task 1: Status bar configuration

**Files:**
- Modify: `fm/config.go`
- Test: `fm/config_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `fm/config_test.go`:

```go
func TestStatusBarConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.StatusBar.Enabled {
		t.Fatalf("status bar should default to enabled")
	}
	if cfg.StatusBar.HideInFull {
		t.Fatalf("status bar should default to visible in full mode")
	}
	want := []string{"size", "date", "free"}
	if !slices.Equal(cfg.StatusBar.Fields, want) {
		t.Fatalf("default fields = %v, want %v", cfg.StatusBar.Fields, want)
	}
}

func TestStatusBarConfigAbsentBlockKeepsDefaults(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("columns:\n  brief_chars: 20\n"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg.normalize()
	if !cfg.StatusBar.Enabled {
		t.Fatalf("absent status_bar block should leave the bar enabled")
	}
	want := []string{"size", "date", "free"}
	if !slices.Equal(cfg.StatusBar.Fields, want) {
		t.Fatalf("fields = %v, want %v", cfg.StatusBar.Fields, want)
	}
}

func TestStatusBarConfigAbsentEnabledKeyMeansTrue(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("status_bar:\n  hide_in_full: true\n"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg.normalize()
	if !cfg.StatusBar.Enabled {
		t.Fatalf("absent enabled key should mean true, not false")
	}
	if !cfg.StatusBar.HideInFull {
		t.Fatalf("hide_in_full should have been read")
	}
}

func TestStatusBarConfigExplicitDisable(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("status_bar:\n  enabled: false\n"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg.normalize()
	if cfg.StatusBar.Enabled {
		t.Fatalf("enabled: false should disable the bar")
	}
}

func TestNormalizeStatusBarFields(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"empty falls back to defaults", nil, []string{"size", "date", "free"}},
		{"unknown keys dropped", []string{"size", "bogus", "date"}, []string{"size", "date"}},
		{"duplicates removed", []string{"size", "size", "date"}, []string{"size", "date"}},
		{"sorted into canonical order", []string{"free", "perms", "size"}, []string{"size", "perms", "free"}},
		{"all unknown falls back to defaults", []string{"nope"}, []string{"size", "date", "free"}},
		{"case and space tolerant", []string{" Size ", "DATE"}, []string{"size", "date"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeStatusBarFields(tc.in)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("NormalizeStatusBarFields(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestStatusBarConfigRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StatusBar.Enabled = true
	cfg.StatusBar.HideInFull = true
	cfg.StatusBar.Fields = []string{"size", "date", "perms", "owner", "items", "selection", "free"}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Config
	if err := yaml.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	back.normalize()
	if !back.StatusBar.HideInFull {
		t.Fatalf("hide_in_full did not survive the round trip")
	}
	if !slices.Equal(back.StatusBar.Fields, cfg.StatusBar.Fields) {
		t.Fatalf("fields = %v, want %v", back.StatusBar.Fields, cfg.StatusBar.Fields)
	}
}
```

Check the existing imports at the top of `fm/config_test.go`. Add `"slices"` if it is not already imported. **The project uses `go.yaml.in/yaml/v4`, not `gopkg.in/yaml.v3`** — it is already imported in both files, so add no new yaml dependency.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./fm/ -run 'StatusBar' -v`

Expected: compile failure — `cfg.StatusBar` undefined, `NormalizeStatusBarFields` undefined.

- [ ] **Step 3: Add the config type and normalisation**

In `fm/config.go`, add near the other config constants (around line 30, beside `defaultBriefChars`):

```go
// Status bar field keys, in canonical display order. The order of this slice
// is the order fields render in; NormalizeStatusBarFields sorts into it.
var statusBarFieldOrder = []string{
	StatusBarFieldSize,
	StatusBarFieldDate,
	StatusBarFieldPerms,
	StatusBarFieldOwner,
	StatusBarFieldItems,
	StatusBarFieldSelection,
	StatusBarFieldFree,
}

const (
	StatusBarFieldSize      = "size"
	StatusBarFieldDate      = "date"
	StatusBarFieldPerms     = "perms"
	StatusBarFieldOwner     = "owner"
	StatusBarFieldItems     = "items"
	StatusBarFieldSelection = "selection"
	StatusBarFieldFree      = "free"
)

func defaultStatusBarFields() []string {
	return []string{StatusBarFieldSize, StatusBarFieldDate, StatusBarFieldFree}
}
```

Add the type beside the other config structs (after `ColumnWidths`, around line 160):

```go
type StatusBarConfig struct {
	Enabled    bool     `yaml:"enabled"`
	HideInFull bool     `yaml:"hide_in_full"`
	Fields     []string `yaml:"fields"`
}

func defaultStatusBarConfig() StatusBarConfig {
	return StatusBarConfig{
		Enabled:    true,
		HideInFull: false,
		Fields:     defaultStatusBarFields(),
	}
}

func (s *StatusBarConfig) UnmarshalYAML(node *yaml.Node) error {
	var raw struct {
		Enabled    *bool    `yaml:"enabled"`
		HideInFull bool     `yaml:"hide_in_full"`
		Fields     []string `yaml:"fields"`
	}
	if err := node.Decode(&raw); err != nil {
		return err
	}

	out := defaultStatusBarConfig()
	if raw.Enabled != nil {
		out.Enabled = *raw.Enabled
	}
	out.HideInFull = raw.HideInFull
	if len(raw.Fields) > 0 {
		out.Fields = NormalizeStatusBarFields(raw.Fields)
	}

	*s = out
	return nil
}

// NormalizeStatusBarFields drops unknown keys, removes duplicates and sorts the
// result into canonical display order, so a hand-edited config cannot produce an
// unexpected field arrangement. An empty result falls back to the defaults.
func NormalizeStatusBarFields(raw []string) []string {
	seen := make(map[string]bool, len(raw))
	for _, field := range raw {
		key := strings.ToLower(strings.TrimSpace(field))
		if slices.Contains(statusBarFieldOrder, key) {
			seen[key] = true
		}
	}
	if len(seen) == 0 {
		return defaultStatusBarFields()
	}
	out := make([]string, 0, len(seen))
	for _, field := range statusBarFieldOrder {
		if seen[field] {
			out = append(out, field)
		}
	}
	return out
}
```

Add `"slices"` to the `fm/config.go` import block if it is not already there.

Add the field to `Config` (line 675 block), after `Columns`:

```go
	StatusBar         StatusBarConfig      `yaml:"status_bar"`
```

Add the same field to the anonymous raw struct inside `Config.UnmarshalYAML` (line 697 block), after `Columns`:

```go
		StatusBar         *StatusBarConfig     `yaml:"status_bar"`
```

It is a pointer for the same reason `Columns` is: a nil pointer means the block was absent, so the defaults apply rather than a zero value. Then in the body of `Config.UnmarshalYAML`, where the other raw fields are copied across, add:

```go
	c.StatusBar = defaultStatusBarConfig()
	if raw.StatusBar != nil {
		c.StatusBar = *raw.StatusBar
	}
```

Find how `raw.Columns` is handled in that function and place this beside it, following the same style.

In `DefaultConfig()` (line 769), add after `Columns: defaultColumnWidths(),`:

```go
		StatusBar: defaultStatusBarConfig(),
```

In `(c *Config) normalize()` (line 1110), add after the `c.Columns.PermissionFormat` switch:

```go
	c.StatusBar.Fields = NormalizeStatusBarFields(c.StatusBar.Fields)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./fm/ -run 'StatusBar' -v`

Expected: PASS for all six tests.

- [ ] **Step 5: Run the whole fm package**

Run: `go test ./fm/`

Expected: `ok hexone/fm`. If an existing test compares a marshalled config against a golden string, it will need the new `status_bar` block added — update the golden, do not remove the field.

- [ ] **Step 6: Commit**

```bash
git add fm/config.go fm/config_test.go
git commit -m "feat(config): add status_bar configuration block"
```

---

## Task 2: Owner and group in listings

**Files:**
- Create: `filesys/owner.go`, `filesys/owner_other.go`, `filesys/owner_windows.go`
- Modify: `filesys/listing.go`, `filesys/listing_sftp.go`
- Test: `filesys/owner_test.go`

Background: `populateListingEntry(row *Entry, name string, info os.FileInfo)` at `filesys/listing.go:192` is called by **both** the local reader and the SFTP reader. `info.Sys()` returns `*syscall.Stat_t` for local unix files and `*sftp.FileStat` for remote ones. Local entries resolve uid/gid to names; remote entries stay numeric, because a remote uid means nothing in the local passwd database.

- [ ] **Step 1: Write the failing tests**

Create `filesys/owner_test.go`:

```go
// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFormatOwnerNumeric(t *testing.T) {
	if got := formatOwnerNumeric(501, 20); got != "501:20" {
		t.Fatalf("formatOwnerNumeric(501, 20) = %q, want %q", got, "501:20")
	}
}

func TestFormatOwnerNames(t *testing.T) {
	if got := formatOwnerNames("ramunas", "staff"); got != "ramunas:staff" {
		t.Fatalf("got %q, want %q", got, "ramunas:staff")
	}
}

func TestLookupOwnerNameFallsBackToNumeric(t *testing.T) {
	// A uid this high is not in any passwd database, so the lookup fails and
	// the numeric form is used instead.
	const missing = 4294967000
	got := lookupUserName(missing)
	if got != "4294967000" {
		t.Fatalf("lookupUserName(%d) = %q, want the numeric fallback", missing, got)
	}
}

func TestLookupOwnerNameIsCached(t *testing.T) {
	const missing = 4294966999
	first := lookupUserName(missing)
	second := lookupUserName(missing)
	if first != second {
		t.Fatalf("cached lookup returned %q then %q", first, second)
	}
	ownerCacheMu.RLock()
	_, cached := userNameCache[missing]
	ownerCacheMu.RUnlock()
	if !cached {
		t.Fatalf("lookupUserName did not populate the cache")
	}
}

func TestLocalListingPopulatesOwnerText(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows local files carry no uid/gid")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	listing, err := ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range listing.Entries {
		if entry.Name != "file.txt" {
			continue
		}
		if entry.OwnerText == "" {
			t.Fatalf("OwnerText was not populated for a local file")
		}
		return
	}
	t.Fatalf("file.txt not found in listing")
}

func TestParentEntryHasNoOwner(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "child")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	listing, err := ReadDir(sub)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range listing.Entries {
		if entry.Kind == EntryParent && entry.OwnerText != "" {
			t.Fatalf("parent entry OwnerText = %q, want empty", entry.OwnerText)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./filesys/ -run 'Owner' -v`

Expected: compile failure — `formatOwnerNumeric`, `lookupUserName`, `ownerCacheMu`, `userNameCache`, `Entry.OwnerText` all undefined.

- [ ] **Step 3: Create the shared owner code**

Create `filesys/owner.go`:

```go
// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"os"
	"os/user"
	"strconv"
	"sync"
)

// os/user lookups can hit NSS, which is slow enough to matter when a directory
// holds thousands of entries owned by the same few users. Listings run on a
// goroutine, so the cache needs a mutex. Failed lookups cache their numeric
// fallback so they are not retried per entry.
var (
	ownerCacheMu   sync.RWMutex
	userNameCache  = map[uint32]string{}
	groupNameCache = map[uint32]string{}
)

func lookupUserName(uid uint32) string {
	ownerCacheMu.RLock()
	name, ok := userNameCache[uid]
	ownerCacheMu.RUnlock()
	if ok {
		return name
	}

	name = strconv.FormatUint(uint64(uid), 10)
	if resolved, err := user.LookupId(name); err == nil && resolved.Username != "" {
		name = resolved.Username
	}

	ownerCacheMu.Lock()
	userNameCache[uid] = name
	ownerCacheMu.Unlock()
	return name
}

func lookupGroupName(gid uint32) string {
	ownerCacheMu.RLock()
	name, ok := groupNameCache[gid]
	ownerCacheMu.RUnlock()
	if ok {
		return name
	}

	name = strconv.FormatUint(uint64(gid), 10)
	if resolved, err := user.LookupGroupId(name); err == nil && resolved.Name != "" {
		name = resolved.Name
	}

	ownerCacheMu.Lock()
	groupNameCache[gid] = name
	ownerCacheMu.Unlock()
	return name
}

func formatOwnerNames(userName, groupName string) string {
	if userName == "" && groupName == "" {
		return ""
	}
	return userName + ":" + groupName
}

func formatOwnerNumeric(uid, gid uint32) string {
	return strconv.FormatUint(uint64(uid), 10) + ":" + strconv.FormatUint(uint64(gid), 10)
}

// localOwnerText resolves uid/gid to names. Only call it for local files: a
// remote uid has no meaning in the local passwd database.
func localOwnerText(info os.FileInfo) string {
	uid, gid, ok := statOwnerIDs(info)
	if !ok {
		return ""
	}
	return formatOwnerNames(lookupUserName(uid), lookupGroupName(gid))
}

// remoteOwnerText keeps uid/gid numeric, which is the only honest rendering for
// an SFTP entry.
func remoteOwnerText(info os.FileInfo) string {
	uid, gid, ok := statOwnerIDs(info)
	if !ok {
		return ""
	}
	return formatOwnerNumeric(uid, gid)
}
```

- [ ] **Step 4: Create the platform files**

Create `filesys/owner_other.go`:

```go
// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package filesys

import (
	"os"
	"syscall"

	"github.com/pkg/sftp"
)

// statOwnerIDs extracts uid/gid from an os.FileInfo. Local files carry a
// *syscall.Stat_t; SFTP entries carry a *sftp.FileStat. SFTP listings happen on
// every platform, so both arms are needed here and the sftp arm is repeated in
// the Windows file.
func statOwnerIDs(info os.FileInfo) (uid, gid uint32, ok bool) {
	if info == nil {
		return 0, 0, false
	}
	switch sys := info.Sys().(type) {
	case *syscall.Stat_t:
		return uint32(sys.Uid), uint32(sys.Gid), true
	case *sftp.FileStat:
		return sys.UID, sys.GID, true
	}
	return 0, 0, false
}
```

Create `filesys/owner_windows.go`:

```go
// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package filesys

import (
	"os"

	"github.com/pkg/sftp"
)

// statOwnerIDs extracts uid/gid from an os.FileInfo. Windows local files carry
// a *syscall.Win32FileAttributeData, which has no uid/gid, so only remote SFTP
// entries resolve here. Local files report ok == false and the status bar omits
// the field.
func statOwnerIDs(info os.FileInfo) (uid, gid uint32, ok bool) {
	if info == nil {
		return 0, 0, false
	}
	if sys, isRemote := info.Sys().(*sftp.FileStat); isRemote {
		return sys.UID, sys.GID, true
	}
	return 0, 0, false
}
```

- [ ] **Step 5: Add the Entry field and populate it**

In `filesys/listing.go`, add to the `Entry` struct (line 24) after `DateText`:

```go
	OwnerText   string
```

`populateListingEntry` is shared by local, archive and SFTP readers, and each
needs different handling, so the caller decides rather than the shared function.

In `filesys/listing.go`, `readLocalDir` has a symlink/regular branch at lines
99–103. Add the owner immediately after it closes:

```go
			populateSymlinkListingEntry(&row, name, info, targetInfo, target)
		} else {
			populateListingEntry(&row, name, info)
		}
		row.OwnerText = localOwnerText(info)
```

**Do not touch the call at line 157** — that is `readArchiveDir`. Archive
entries have no owner, and leaving `OwnerText` empty there is correct: the field
renders empty and the status bar skips it.

In `filesys/listing_sftp.go`, in the loop around line 66, after the `populateListingEntry` / `populateSymlinkListingEntry` branch closes, add:

```go
		row.OwnerText = remoteOwnerText(item)
```

Leave archive entries alone — they never get an owner, which is correct.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./filesys/ -run 'Owner|Parent' -v`

Expected: PASS. `TestLocalListingPopulatesOwnerText` skips on Windows.

- [ ] **Step 7: Run the whole filesys package**

Run: `go test ./filesys/`

Expected: `ok hexone/filesys`.

- [ ] **Step 8: Verify the Windows build compiles**

Run: `GOOS=windows go build ./filesys/`

Expected: no output. This catches a `syscall.Stat_t` reference leaking into shared code.

- [ ] **Step 9: Commit**

```bash
git add filesys/owner.go filesys/owner_other.go filesys/owner_windows.go filesys/owner_test.go filesys/listing.go filesys/listing_sftp.go
git commit -m "feat(filesys): resolve owner and group during directory listing"
```

---

## Task 3: Status bar field values

**Files:**
- Create: `ui/filepane_status_fields.go`
- Test: `ui/filepane_status_fields_test.go`

This task builds the pure functions that turn a pane into a list of rendered field strings. No layout, no Gio. Keeping it separate is what makes it testable without a window.

- [ ] **Step 1: Write the failing tests**

Create `ui/filepane_status_fields_test.go`:

```go
// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"testing"

	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/widget/table"
)

// testStatusPane builds a pane with no columns. The status bar reads only
// table.Selected and table.Mode, never the columns, so table.New(nil) is enough
// and avoids dragging the real column definitions into these tests.
func testStatusPane(entries []filesys.Entry, selected int) *filePaneState {
	cfg := fm.DefaultConfig()
	pane := &filePaneState{}
	pane.model = &filePaneModel{cfg: cfg, entries: entries}
	pane.table = table.New(nil)
	pane.table.Selected = selected
	return pane
}

func TestStatusFieldSize(t *testing.T) {
	entries := []filesys.Entry{
		{Name: "..", Kind: filesys.EntryParent},
		{Name: "dir", Kind: filesys.EntryDir},
		{Name: "file.txt", Kind: filesys.EntryFile, SizeBytes: 2516582},
	}
	tests := []struct {
		row  int
		want string
	}{
		{0, "<UP>"},
		{1, "<DIR>"},
		{2, "2.40 MB"},
	}
	for _, tc := range tests {
		pane := testStatusPane(entries, tc.row)
		got := filePaneStatusFieldValue(pane, filePaneStatusFieldSize, "")
		if got != tc.want {
			t.Fatalf("row %d size = %q, want %q", tc.row, got, tc.want)
		}
	}
}

func TestStatusFieldDateAndPermsEmptyOnParent(t *testing.T) {
	entries := []filesys.Entry{{Name: "..", Kind: filesys.EntryParent}}
	pane := testStatusPane(entries, 0)
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldDate, ""); got != "" {
		t.Fatalf("parent date = %q, want empty", got)
	}
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldPerms, ""); got != "" {
		t.Fatalf("parent perms = %q, want empty", got)
	}
}

func TestStatusFieldPermsFollowsFormatSetting(t *testing.T) {
	entries := []filesys.Entry{
		{Name: "f", Kind: filesys.EntryFile, PermText: "-rw-r--r--", PermOctal: "0644"},
	}
	pane := testStatusPane(entries, 0)

	pane.model.cfg.Columns.PermissionFormat = "symbolic"
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldPerms, ""); got != "-rw-r--r--" {
		t.Fatalf("symbolic perms = %q", got)
	}

	pane.model.cfg.Columns.PermissionFormat = "octal"
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldPerms, ""); got != "0644" {
		t.Fatalf("octal perms = %q", got)
	}
}

func TestStatusFieldOwner(t *testing.T) {
	entries := []filesys.Entry{
		{Name: "f", Kind: filesys.EntryFile, OwnerText: "ramunas:staff"},
		{Name: "g", Kind: filesys.EntryFile, OwnerText: ""},
	}
	pane := testStatusPane(entries, 0)
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldOwner, ""); got != "ramunas:staff" {
		t.Fatalf("owner = %q", got)
	}
	pane.table.Selected = 1
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldOwner, ""); got != "" {
		t.Fatalf("missing owner should render empty, got %q", got)
	}
}

func TestStatusFieldItemsExcludesParent(t *testing.T) {
	entries := []filesys.Entry{
		{Name: "..", Kind: filesys.EntryParent},
		{Name: "a", Kind: filesys.EntryFile},
		{Name: "b", Kind: filesys.EntryFile},
	}
	pane := testStatusPane(entries, 1)
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldItems, ""); got != "2 items" {
		t.Fatalf("items = %q, want %q", got, "2 items")
	}
}

func TestStatusFieldItemsSingular(t *testing.T) {
	entries := []filesys.Entry{{Name: "a", Kind: filesys.EntryFile}}
	pane := testStatusPane(entries, 0)
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldItems, ""); got != "1 item" {
		t.Fatalf("items = %q, want %q", got, "1 item")
	}
}

func TestStatusFieldSelectionEmptyWhenNothingMarked(t *testing.T) {
	entries := []filesys.Entry{{Name: "a", Kind: filesys.EntryFile, SizeBytes: 100}}
	pane := testStatusPane(entries, 0)
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldSelection, ""); got != "" {
		t.Fatalf("unmarked selection = %q, want empty", got)
	}
}

func TestStatusFieldSelectionSumsMarkedSizes(t *testing.T) {
	entries := []filesys.Entry{
		{Name: "a", Kind: filesys.EntryFile, Path: "/a", SizeBytes: 1048576},
		{Name: "b", Kind: filesys.EntryFile, Path: "/b", SizeBytes: 2097152},
		{Name: "c", Kind: filesys.EntryFile, Path: "/c", SizeBytes: 4194304},
	}
	pane := testStatusPane(entries, 0)
	pane.markedRows = map[int]struct{}{0: {}, 1: {}}
	want := "2 marked · 3.00 MB"
	if got := filePaneStatusFieldValue(pane, filePaneStatusFieldSelection, ""); got != want {
		t.Fatalf("selection = %q, want %q", got, want)
	}
}

func TestStatusFieldFreeForms(t *testing.T) {
	tests := []struct {
		form filePaneStatusFreeForm
		want string
	}{
		{filePaneStatusFreeFull, "██████░░░░ 41.20 GB free / 100.00 GB"},
		{filePaneStatusFreeNoTotal, "██████░░░░ 41.20 GB free"},
		{filePaneStatusFreeNoBar, "41.20 GB free"},
	}
	const total = uint64(100) << 30
	// gib is a variable on purpose: 41.2*float64(uint64(1)<<30) would be a
	// constant expression, and converting a constant with a fractional part to
	// uint64 does not compile.
	gib := float64(uint64(1) << 30)
	free := uint64(41.2 * gib)
	for _, tc := range tests {
		got := formatFilePaneStatusFree(free, total, tc.form)
		if got != tc.want {
			t.Fatalf("form %v = %q, want %q", tc.form, got, tc.want)
		}
	}
}

func TestStatusFreeUsageBarFillsUsedFraction(t *testing.T) {
	// 90% used should show nine filled cells, not one.
	const total = uint64(100) << 30
	const free = uint64(10) << 30
	got := formatFilePaneStatusFree(free, total, filePaneStatusFreeNoTotal)

	// The bar cells are block-drawing runes of three bytes each, so the leading
	// cells have to be taken as runes; slicing the string by bytes would cut a
	// rune in half and never match.
	runes := []rune(got)
	if len(runes) < filePaneStatusFreeBarCells {
		t.Fatalf("free field = %q, too short to hold a %d-cell bar", got, filePaneStatusFreeBarCells)
	}
	bar := string(runes[:filePaneStatusFreeBarCells])
	if bar != "█████████░" {
		t.Fatalf("usage bar = %q, want the bar to fill with the used fraction (full field %q)", bar, got)
	}
}

func TestBuildStatusLineJoinsWithPipes(t *testing.T) {
	parts := []string{"2.40 MB", "2026-08-30 14:22", "-rw-r--r--"}
	want := "2.40 MB | 2026-08-30 14:22 | -rw-r--r--"
	if got := buildFilePaneStatusLine(parts); got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
}

func TestBuildStatusLineSkipsEmptyParts(t *testing.T) {
	parts := []string{"2.40 MB", "", "-rw-r--r--", ""}
	want := "2.40 MB | -rw-r--r--"
	if got := buildFilePaneStatusLine(parts); got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
}
```

`table.New(nil)` is the real constructor (`ui/widget/table/table.go:187`); the
pane itself calls `table.New(cols)` at `ui/filemanager.go:544`. Passing nil
columns is safe here because these tests never lay the table out.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./ui/ -run 'StatusField|StatusLine|StatusFree' -v`

Expected: compile failure — the field constants and functions do not exist.

- [ ] **Step 3: Write the field implementation**

Create `ui/filepane_status_fields.go`:

```go
// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"strings"

	"hexone/filesys"
	"hexone/fm"
)

// filePaneStatusField identifies one field of the pane status bar. Declaration
// order is canonical display order; the drop order below is deliberately
// different.
type filePaneStatusField uint8

const (
	filePaneStatusFieldSize filePaneStatusField = iota
	filePaneStatusFieldDate
	filePaneStatusFieldPerms
	filePaneStatusFieldOwner
	filePaneStatusFieldItems
	filePaneStatusFieldSelection
	filePaneStatusFieldFree
)

const filePaneStatusSeparator = " | "

// filePaneStatusDropOrder lists fields from first-dropped to last-dropped when
// the line does not fit. Size and date survive longest because reading them in
// brief mode is the reason this bar exists. Free space is dropped before
// selection because it is ambient information, whereas a marked-file summary is
// only present while the user is actively marking.
var filePaneStatusDropOrder = []filePaneStatusField{
	filePaneStatusFieldOwner,
	filePaneStatusFieldItems,
	filePaneStatusFieldPerms,
	filePaneStatusFieldFree,
	filePaneStatusFieldSelection,
	filePaneStatusFieldDate,
	filePaneStatusFieldSize,
}

func filePaneStatusFieldFromConfigKey(key string) (filePaneStatusField, bool) {
	switch key {
	case fm.StatusBarFieldSize:
		return filePaneStatusFieldSize, true
	case fm.StatusBarFieldDate:
		return filePaneStatusFieldDate, true
	case fm.StatusBarFieldPerms:
		return filePaneStatusFieldPerms, true
	case fm.StatusBarFieldOwner:
		return filePaneStatusFieldOwner, true
	case fm.StatusBarFieldItems:
		return filePaneStatusFieldItems, true
	case fm.StatusBarFieldSelection:
		return filePaneStatusFieldSelection, true
	case fm.StatusBarFieldFree:
		return filePaneStatusFieldFree, true
	}
	return 0, false
}

// filePaneStatusFields converts normalised config keys into fields, preserving
// the config's order (which normalisation has already made canonical).
func filePaneStatusFields(keys []string) []filePaneStatusField {
	out := make([]filePaneStatusField, 0, len(keys))
	for _, key := range keys {
		if field, ok := filePaneStatusFieldFromConfigKey(key); ok {
			out = append(out, field)
		}
	}
	return out
}

// filePaneStatusFreeForm selects how much of the free-space field is rendered.
// The degradation loop tries these in order before dropping the field entirely.
type filePaneStatusFreeForm uint8

const (
	filePaneStatusFreeFull filePaneStatusFreeForm = iota
	filePaneStatusFreeNoTotal
	filePaneStatusFreeNoBar
)

const filePaneStatusFreeBarCells = 10

// formatFilePaneStatusFree renders the free-space field. The usage bar fills
// with the *used* fraction, so a nearly-full disk reads as a nearly-full bar.
func formatFilePaneStatusFree(freeBytes, totalBytes uint64, form filePaneStatusFreeForm) string {
	if totalBytes == 0 {
		return ""
	}
	if freeBytes > totalBytes {
		freeBytes = totalBytes
	}

	label := formatFilePaneVolumeBytes(freeBytes) + " free"
	if form == filePaneStatusFreeFull {
		label += " / " + formatFilePaneVolumeBytes(totalBytes)
	}
	if form == filePaneStatusFreeNoBar {
		return label
	}

	used := float32(totalBytes-freeBytes) / float32(totalBytes)
	return textCellProgressBar(used, filePaneStatusFreeBarCells) + " " + label
}

// filePaneStatusFieldValue renders one field for the pane's cursor entry. A
// field with nothing to show returns an empty string and is skipped by
// buildFilePaneStatusLine, taking its separator with it.
//
// freeLabel is passed in rather than computed here because the volume lookup is
// cached on the pane and refreshed on a timer by the layout path.
func filePaneStatusFieldValue(pane *filePaneState, field filePaneStatusField, freeLabel string) string {
	if pane == nil || pane.model == nil {
		return ""
	}
	entry := pane.selectedEntry()

	switch field {
	case filePaneStatusFieldSize:
		if entry == nil {
			return ""
		}
		switch entry.Kind {
		case filesys.EntryParent:
			return "<UP>"
		case filesys.EntryDir:
			return "<DIR>"
		}
		return formatFilePaneVolumeBytes(uint64(max(entry.SizeBytes, 0)))

	case filePaneStatusFieldDate:
		if entry == nil {
			return ""
		}
		return strings.TrimSpace(entry.DateText)

	case filePaneStatusFieldPerms:
		if entry == nil {
			return ""
		}
		return strings.TrimSpace(pane.model.defaultPermissionText(*entry))

	case filePaneStatusFieldOwner:
		if entry == nil {
			return ""
		}
		// SFTP owners render numerically (Task 2), which includes "0:0". That
		// is genuinely ambiguous: pkg/sftp drops the SSH_FILEXFER_ATTR_UIDGID
		// flag, so a server that never reported ownership is indistinguishable
		// from a root-owned file. Render it anyway — suppressing "0:0" would
		// hide real root ownership, which matters far more often than the
		// ambiguity bites, and the raw numeric form already signals that these
		// are unresolved ids rather than a confirmed "root".
		return strings.TrimSpace(entry.OwnerText)

	case filePaneStatusFieldItems:
		count := 0
		for i := range pane.model.entries {
			if pane.model.entries[i].Kind != filesys.EntryParent {
				count++
			}
		}
		if count == 1 {
			return "1 item"
		}
		return fmt.Sprintf("%d items", count)

	case filePaneStatusFieldSelection:
		if !pane.hasMarkedRows() {
			return ""
		}
		total := int64(0)
		for row := range pane.markedRows {
			if marked := pane.model.Entry(row); marked != nil && marked.SizeBytes > 0 {
				total += marked.SizeBytes
			}
		}
		return fmt.Sprintf("%d marked · %s", len(pane.markedRows), formatFilePaneVolumeBytes(uint64(total)))

	case filePaneStatusFieldFree:
		return freeLabel
	}
	return ""
}

func buildFilePaneStatusLine(parts []string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, filePaneStatusSeparator)
}
```

Note the reuse: `formatFilePaneVolumeBytes` (`ui/filepane_volume_badge.go:273`) and `textCellProgressBar` (`ui/fileop_dialog_style.go:81`) already exist. Do not write new formatters.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./ui/ -run 'StatusField|StatusLine|StatusFree' -v`

Expected: PASS.

If `TestStatusFieldSize` fails on the exact string, check what `formatFilePaneVolumeBytes` produces for 2516582 bytes — it uses `%.2f`, so `2.40 MB`. Adjust the test expectation to the real output rather than changing the formatter, since the whole point is agreeing with the free-space field.

- [ ] **Step 5: Commit**

```bash
git add ui/filepane_status_fields.go ui/filepane_status_fields_test.go
git commit -m "feat(ui): add status bar field value builders"
```

---

## Task 4: Move the status strip chrome

**Files:**
- Create: `ui/filepane_status_bar.go`
- Modify: `ui/filepane_volume_badge.go`

Pure move, no behaviour change. Doing it as its own commit keeps the next task's diff readable.

- [ ] **Step 1: Move the code**

Cut these from `ui/filepane_volume_badge.go` and paste them into a new `ui/filepane_status_bar.go`, unchanged:

- `type filePaneStatusBarSeparatorMode uint8` and its three constants (line 392)
- `func (ui *UI) filePaneStatusBarSeparatorMode(idx int)` (line 400)
- `func (ui *UI) layoutFilePaneStatusBar(...)` (line 410)
- `func (ui *UI) directFilePasteForPane(idx int)` (line 471)
- `func (ui *UI) measureFilePaneStatusBarTextWidth(...)` (line 527)
- `func layoutFilePaneStatusBarBox(...)` (line 552)

Give the new file the licence header and a `package ui` line. Fix the import blocks in both files: the new file needs whatever those functions reference (`image`, `image/color`, `strings`, `gioui.org/layout`, `gioui.org/op`, `gioui.org/op/clip`, `gioui.org/op/paint`, `gioui.org/unit`, `gioui.org/widget/material`), and `ui/filepane_volume_badge.go` will have imports that are now unused.

- [ ] **Step 2: Verify it builds and nothing changed**

Run: `go build ./... && go test ./ui/ -run 'StatusBar|VolumeBadge' -v`

Expected: builds clean, existing tests still pass. `goimports -w ui/filepane_status_bar.go ui/filepane_volume_badge.go` will sort the imports out if the build complains.

- [ ] **Step 3: Commit**

```bash
git add ui/filepane_status_bar.go ui/filepane_volume_badge.go
git commit -m "refactor(ui): move pane status strip chrome out of the volume badge file"
```

---

## Task 5: Render the file info line

**Files:**
- Modify: `ui/filepane_status_bar.go`
- Test: `ui/filepane_status_bar_test.go`

- [ ] **Step 0: Relocate the existing strip tests**

Task 4 moved the strip chrome into `ui/filepane_status_bar.go` but left its tests
behind in `ui/filepane_volume_badge_test.go` (around lines 315–375):
`TestLayoutFilePaneStatusBarUsesFullPaneWidth`,
`TestLayoutFilePaneStatusBarShowsDirectPasteWithoutBlockingUI`,
`TestFilePaneStatusBarSeparatorModeFollowsPaneSide`, and any other
`*StatusBar*` test in that file.

Since this task creates `ui/filepane_status_bar_test.go` anyway, move them there
first, **verbatim** — no renames, no changes — so tests live beside the code they
exercise. Locate them by name rather than by line number. Run
`go test ./ui/ -run 'StatusBar|VolumeBadge' -v` before and after and confirm the
same set of tests runs and passes; the count must not change.

Do this as a distinct step before writing anything new, so the relocation stays
separable from the new work.

- [ ] **Step 1: Write the failing tests**

Add to `ui/filepane_status_bar_test.go`:

```go
// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"strings"
	"testing"

	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/widget/table"
)

// measureByRunes stands in for real text measurement: one rune, one unit.
func measureByRunes(text string) int { return len([]rune(text)) }

func TestStatusBarVisibility(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		hideInFull bool
		mode       table.Mode
		want       bool
	}{
		{"enabled, brief", true, false, table.ModeBrief, true},
		{"enabled, full", true, false, table.ModeFull, true},
		{"hide in full, brief", true, true, table.ModeBrief, true},
		{"hide in full, full", true, true, table.ModeFull, false},
		{"disabled, brief", false, false, table.ModeBrief, false},
		{"disabled, full", false, false, table.ModeFull, false},
		{"disabled overrides hide in full", false, true, table.ModeBrief, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fm.DefaultConfig()
			cfg.StatusBar.Enabled = tc.enabled
			cfg.StatusBar.HideInFull = tc.hideInFull
			pane := testStatusPane([]filesys.Entry{{Name: "a", Kind: filesys.EntryFile}}, 0)
			pane.table.SetMode(tc.mode)
			if got := filePaneStatusBarVisible(cfg, pane); got != tc.want {
				t.Fatalf("visible = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStatusLineForWidthKeepsEverythingWhenItFits(t *testing.T) {
	pane := testStatusPane([]filesys.Entry{
		{Name: "f", Kind: filesys.EntryFile, SizeBytes: 2516582, DateText: "2026-08-30 14:22", PermText: "-rw-r--r--"},
	}, 0)
	fields := []filePaneStatusField{
		filePaneStatusFieldSize, filePaneStatusFieldDate, filePaneStatusFieldPerms,
	}
	got := filePaneStatusLineForWidth(pane, fields, 0, 0, 1000, measureByRunes)
	want := "2.40 MB | 2026-08-30 14:22 | -rw-r--r--"
	if got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
}

func TestStatusLineShortensFreeBeforeDroppingFields(t *testing.T) {
	pane := testStatusPane([]filesys.Entry{
		{Name: "f", Kind: filesys.EntryFile, SizeBytes: 2516582, DateText: "2026-08-30 14:22"},
	}, 0)
	fields := []filePaneStatusField{
		filePaneStatusFieldSize, filePaneStatusFieldDate, filePaneStatusFieldFree,
	}
	const freeBytes = uint64(41) << 30
	const totalBytes = uint64(100) << 30

	full := filePaneStatusLineForWidth(pane, fields, freeBytes, totalBytes, 1000, measureByRunes)
	if !strings.Contains(full, "/ 100.00 GB") {
		t.Fatalf("wide line should carry the total: %q", full)
	}

	// Just too narrow for the total, but wide enough for everything else.
	narrow := filePaneStatusLineForWidth(pane, fields, freeBytes, totalBytes, measureByRunes(full)-1, measureByRunes)
	if strings.Contains(narrow, "/ 100.00 GB") {
		t.Fatalf("narrower line should have dropped the total: %q", narrow)
	}
	if !strings.Contains(narrow, "free") {
		t.Fatalf("narrower line should still show free space: %q", narrow)
	}
	if !strings.Contains(narrow, "2026-08-30 14:22") {
		t.Fatalf("shortening free must happen before dropping date: %q", narrow)
	}
}

func TestStatusLineDropsFieldsInOrder(t *testing.T) {
	pane := testStatusPane([]filesys.Entry{
		{Name: "f", Kind: filesys.EntryFile, SizeBytes: 2516582, DateText: "2026-08-30 14:22",
			PermText: "-rw-r--r--", OwnerText: "ramunas:staff"},
	}, 0)
	fields := []filePaneStatusField{
		filePaneStatusFieldSize, filePaneStatusFieldDate, filePaneStatusFieldPerms,
		filePaneStatusFieldOwner, filePaneStatusFieldItems,
	}

	// Squeeze progressively and assert what survives. Owner goes first, then
	// items, then perms, then date, leaving size last.
	widest := filePaneStatusLineForWidth(pane, fields, 0, 0, 1000, measureByRunes)
	if !strings.Contains(widest, "ramunas:staff") {
		t.Fatalf("widest line should include owner: %q", widest)
	}

	line := filePaneStatusLineForWidth(pane, fields, 0, 0, 45, measureByRunes)
	if strings.Contains(line, "ramunas:staff") {
		t.Fatalf("owner should drop first: %q", line)
	}
	if !strings.Contains(line, "-rw-r--r--") {
		t.Fatalf("perms should outlive owner: %q", line)
	}

	line = filePaneStatusLineForWidth(pane, fields, 0, 0, 20, measureByRunes)
	if !strings.Contains(line, "2.40 MB") {
		t.Fatalf("size must survive longest: %q", line)
	}

	line = filePaneStatusLineForWidth(pane, fields, 0, 0, 7, measureByRunes)
	if line != "2.40 MB" {
		t.Fatalf("at minimum width only size should remain, got %q", line)
	}
}

func TestStatusLineEmptyWhenNoFieldsHaveValues(t *testing.T) {
	pane := testStatusPane([]filesys.Entry{{Name: "..", Kind: filesys.EntryParent}}, 0)
	fields := []filePaneStatusField{filePaneStatusFieldDate, filePaneStatusFieldPerms}
	if got := filePaneStatusLineForWidth(pane, fields, 0, 0, 1000, measureByRunes); got != "" {
		t.Fatalf("line = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./ui/ -run 'StatusBarVisibility|StatusLine' -v`

Expected: compile failure — `filePaneStatusBarVisible` and `filePaneStatusLineForWidth` undefined.

- [ ] **Step 3: Add the visibility predicate and line builder**

Append to `ui/filepane_status_bar.go`:

```go
// filePaneStatusBarVisible reports whether the file info line should render for
// this pane. Evaluated per pane, because panes carry their view mode
// independently: a brief pane keeps its bar while a full pane beside it hides
// one.
func filePaneStatusBarVisible(cfg *fm.Config, pane *filePaneState) bool {
	if cfg == nil || pane == nil || pane.table == nil {
		return false
	}
	if !cfg.StatusBar.Enabled {
		return false
	}
	return !(cfg.StatusBar.HideInFull && pane.table.Mode == table.ModeFull)
}

// filePaneStatusLineForWidth assembles the file info line and shrinks it to fit.
//
// Two phases: first try the free-space field's shorter forms, since losing a
// total is cheaper than losing a whole field; only then start dropping fields in
// filePaneStatusDropOrder.
//
// The free-space bytes are passed raw rather than pre-formatted so each shorter
// form is produced by formatFilePaneStatusFree from the same numbers, instead of
// by re-parsing an already-formatted label. totalBytes == 0 means the volume
// lookup has not landed yet; formatFilePaneStatusFree then returns "" and the
// field is skipped along with its separator.
func filePaneStatusLineForWidth(
	pane *filePaneState,
	fields []filePaneStatusField,
	freeBytes, totalBytes uint64,
	maxWidth int,
	measure func(string) int,
) string {
	if pane == nil || len(fields) == 0 {
		return ""
	}

	// Render the non-free fields once. Only free space varies between shrink
	// attempts, and filePaneStatusFieldValue is not free — the items field scans
	// the whole listing — so re-rendering everything on each iteration would do
	// O(fields x forms) work per frame. This mirrors archiveExtractStatusPartsFor
	// (ui/archive_extract.go:509), which likewise computes its parts once and
	// lets the degradation loop re-join them.
	rendered := make(map[filePaneStatusField]string, len(fields))
	for _, field := range fields {
		if field != filePaneStatusFieldFree {
			rendered[field] = filePaneStatusFieldValue(pane, field, "")
		}
	}

	forms := []filePaneStatusFreeForm{filePaneStatusFreeFull}
	if slices.Contains(fields, filePaneStatusFieldFree) {
		forms = append(forms, filePaneStatusFreeNoTotal, filePaneStatusFreeNoBar)
	}

	render := func(active []filePaneStatusField, form filePaneStatusFreeForm) string {
		parts := make([]string, 0, len(active))
		for _, field := range active {
			if field == filePaneStatusFieldFree {
				parts = append(parts, formatFilePaneStatusFree(freeBytes, totalBytes, form))
				continue
			}
			parts = append(parts, rendered[field])
		}
		return buildFilePaneStatusLine(parts)
	}

	fits := func(line string) bool {
		return measure == nil || maxWidth <= 0 || measure(line) <= maxWidth
	}

	active := append([]filePaneStatusField(nil), fields...)
	for {
		for _, form := range forms {
			if line := render(active, form); fits(line) {
				return line
			}
		}
		next, ok := filePaneStatusDropNext(active)
		if !ok {
			// Nothing left to drop; return the shortest form we have.
			return render(active, forms[len(forms)-1])
		}
		active = next
	}
}

// filePaneStatusDropNext removes the highest-priority-to-drop field still
// present, returning false when only one field remains.
func filePaneStatusDropNext(active []filePaneStatusField) ([]filePaneStatusField, bool) {
	if len(active) <= 1 {
		return active, false
	}
	for _, candidate := range filePaneStatusDropOrder {
		for i, field := range active {
			if field == candidate {
				return append(active[:i:i], active[i+1:]...), true
			}
		}
	}
	return active, false
}
```

Add `"hexone/fm"`, `"hexone/ui/widget/table"`, `"slices"` and `"strings"` to the file's imports.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./ui/ -run 'StatusBarVisibility|StatusLine' -v`

Expected: PASS. The exact widths in `TestStatusLineDropsFieldsInOrder` depend on the rendered strings; if a case fails, print the line with `t.Logf` and pick a width that sits between the two states you are asserting, rather than loosening the assertion.

- [ ] **Step 5: Commit**

```bash
git add ui/filepane_status_bar.go ui/filepane_status_bar_test.go
git commit -m "feat(ui): assemble and degrade the pane status line"
```

---

## Task 6: Wire the line into the pane

**Files:**
- Modify: `ui/filepane_status_bar.go`
- Modify: `ui/filemanager_layout.go`
- Test: `ui/filepane_volume_badge_test.go`

- [ ] **Step 1: Write the failing test**

Append to `ui/filepane_volume_badge_test.go`:

```go
func TestVolumeBadgeSuppressedWhenFreeFieldEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		fields  []string
		want    bool
	}{
		{"free field on", true, []string{"size", "date", "free"}, true},
		{"free field off", true, []string{"size", "date"}, false},
		{"bar disabled", false, []string{"size", "date", "free"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fm.DefaultConfig()
			cfg.StatusBar.Enabled = tc.enabled
			cfg.StatusBar.Fields = tc.fields
			ui := NewUI(cfg)
			if got := ui.filePaneStatusBarShowsFreeSpace(); got != tc.want {
				t.Fatalf("filePaneStatusBarShowsFreeSpace() = %v, want %v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./ui/ -run 'VolumeBadgeSuppressed' -v`

Expected: compile failure — `filePaneStatusBarShowsFreeSpace` undefined.

- [ ] **Step 3: Add the free-space predicate and badge suppression**

Append to `ui/filepane_status_bar.go`:

```go
// filePaneStatusBarShowsFreeSpace reports whether free space has moved into the
// status bar. When it has, the floating volume badge is never drawn: a pane
// whose bar is hidden simply shows no free space, rather than falling back to a
// second presentation of the same number.
func (ui *UI) filePaneStatusBarShowsFreeSpace() bool {
	if ui == nil || ui.fmCfg == nil || !ui.fmCfg.StatusBar.Enabled {
		return false
	}
	return slices.Contains(ui.fmCfg.StatusBar.Fields, fm.StatusBarFieldFree)
}
```

Add `"slices"` to the imports.

> **Superseded 2026-09-03, after live user review.** The predicate above shipped
> as written and was reported as a bug: reading `StatusBar.Enabled` alone meant
> that with `hide_in_full: true` in a full-mode pane the bar was gone *and* the
> badge stayed suppressed, so free space appeared nowhere — while switching the
> bar off entirely handed it back to the badge. The badge reports the **active**
> pane's volume, so the body is now gated on the strip's own visibility rule,
> `filePaneStatusBarVisible(ui.fmCfg, ui.activePane())`, instead of on `Enabled`;
> the field check comes first and stays. See the dated note in
> `docs/superpowers/specs/2026-08-31-file-pane-status-bar-design.md` §3 for the
> reasoning and the current body. Everything else in this step is unchanged.

In `ui/filemanager_layout.go`, change `filePaneVolumeBadgesHidden` (line 631) from:

```go
func (ui *UI) filePaneVolumeBadgesHidden(gtx layout.Context) bool {
	return ui.terminalVisuallyFocused(gtx)
}
```

to:

```go
func (ui *UI) filePaneVolumeBadgesHidden(gtx layout.Context) bool {
	return ui.terminalVisuallyFocused(gtx) || ui.filePaneStatusBarShowsFreeSpace()
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./ui/ -run 'VolumeBadgeSuppressed' -v`

Expected: PASS.

- [ ] **Step 5: Add the file info branch to the strip**

In `ui/filepane_status_bar.go`, `layoutFilePaneStatusBar` currently returns early when neither extraction nor paste is running:

```go
	showArchiveExtract := ui.archiveExtractPane() == pane
	directPaste := ui.directFilePasteForPane(idx)
	if !showArchiveExtract && directPaste == nil {
		return layout.Dimensions{}
	}
```

Replace that early return so the file info line becomes the fallback. The progress branches must stay ungated by `StatusBarConfig` — turning the info bar off must not remove extraction feedback:

**Also scope the 250ms repaint tick to the progress branches.**
`layoutFilePaneStatusBar` calls
`gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(archiveExtractStatusRefreshInterval)})`
unconditionally, which was correct while the strip only ever showed progress.
Left unconditional once the info branch exists, an **idle** file manager would
force a repaint every 250ms forever — 4 FPS of wasted CPU and battery with
nothing animating. The info line needs no periodic tick: its only timed wakeup is
the 15s volume poll that `filePaneStatusInfoLine` schedules, and a cursor move
invalidates the frame anyway. Pin this with a test that drives a real
`input.Router` and asserts `WakeupTime()` is not within 250ms for an idle pane.

```go
	showArchiveExtract := ui.archiveExtractPane() == pane
	directPaste := ui.directFilePasteForPane(idx)
	showFileInfo := !showArchiveExtract && directPaste == nil && filePaneStatusBarVisible(ui.fmCfg, pane)
	if !showArchiveExtract && directPaste == nil && !showFileInfo {
		return layout.Dimensions{}
	}
```

Then in the body, where `label` is currently chosen between the extraction and paste lines, add the third branch. The existing shape is:

```go
	label := ""
	if showArchiveExtract {
		label = archiveExtractStatusLineForWidth(...)
		if mode != filePaneStatusBarSeparatorNone {
			label = archiveExtractStatusLineWithSeparatorForWidth(...)
		}
	} else {
		label = directFilePasteStatusLineForWidth(...)
		if mode != filePaneStatusBarSeparatorNone {
			label = directFilePasteStatusLineWithSeparatorForWidth(...)
		}
	}
```

Change the `else` to `else if directPaste != nil`, and add:

```go
	} else {
		label = ui.filePaneStatusInfoLine(gtx, pane, textMax, measure, mode, trailingSeparator)
	}
```

The free-space field needs raw byte counts, not the badge's formatted string.
`filePaneVolumeBadgeLabel` returns only the formatted label, so first extend the
cached state to keep the counts. In `ui/filepane_volume_badge.go`, add to
`filePaneVolumeBadgeState`:

```go
	freeBytes  uint64
	totalBytes uint64
```

and populate them where `state.label` is assigned inside
`filePaneVolumeBadgeLabel`, which already has the `platform.VolumeUsage` in hand:

```go
		state.freeBytes = usage.FreeBytes
		state.totalBytes = usage.TotalBytes
		state.label = formatFilePaneVolumeBadgeLabel(usage.FreeBytes, usage.TotalBytes)
```

Also clear them on the failure path in that function, beside `state.label = ""`:

```go
			state.freeBytes = 0
			state.totalBytes = 0
```

This keeps a single source of truth for the numbers, and avoids parsing a
formatted string back into integers.

Now add the helper to `ui/filepane_status_bar.go`. It resolves the free-space
label and applies the same seam separator treatment the other two lines get:

```go
// filePaneStatusInfoLine builds the file info line for a pane, including the
// pane-seam separator so adjacent strips read as one continuous band.
func (ui *UI) filePaneStatusInfoLine(
	gtx layout.Context,
	pane *filePaneState,
	maxWidth int,
	measure func(string) int,
	mode filePaneStatusBarSeparatorMode,
	trailing bool,
) string {
	fields := filePaneStatusFields(ui.fmCfg.StatusBar.Fields)
	if len(fields) == 0 {
		return ""
	}

	var freeBytes, totalBytes uint64
	if slices.Contains(fields, filePaneStatusFieldFree) {
		// Calling this drives the 15s volume poll and refreshes the cached
		// counts; the returned label is deliberately ignored in favour of the
		// raw bytes, which the line builder formats itself so that each shorter
		// form comes from the same numbers.
		if _, _, ok := ui.filePaneVolumeBadgeLabel(pane, gtx.Now); ok {
			freeBytes = pane.volumeBadge.freeBytes
			totalBytes = pane.volumeBadge.totalBytes
		}
		if nextRefreshAt := pane.volumeBadge.nextRefreshAt; nextRefreshAt.After(gtx.Now) {
			gtx.Execute(op.InvalidateCmd{At: nextRefreshAt})
		}
	}

	// Careful: filePaneStatusLineForWidth treats maxWidth <= 0 as "unlimited"
	// (matching archiveExtractStatusLineForWidth). layoutFilePaneStatusBar
	// clamps textMax to 0 when the pane is narrower than its insets, so the two
	// collide: a pane that narrow emits the FULL line and relies on Gio's
	// MaxLines: 1 to clip it. That is pre-existing behaviour on the
	// archive-extract path, not a regression — but do not "fix" the clamp here
	// without checking that path too.
	lineMax := maxWidth
	separator := ""
	switch mode {
	case filePaneStatusBarSeparatorLeading:
		separator = "| "
	case filePaneStatusBarSeparatorTrailing:
		separator = " |"
	}
	if separator != "" && measure != nil && maxWidth > 0 {
		lineMax = max(maxWidth-measure(separator), 0)
	}

	line := filePaneStatusLineForWidth(pane, fields, freeBytes, totalBytes, lineMax, measure)
	if strings.TrimSpace(line) == "" {
		return ""
	}
	if separator == "" {
		return line
	}
	if trailing {
		return line + separator
	}
	return separator + line
}
```

Add `"gioui.org/op"` to the file's imports if the move in Task 4 did not already
bring it in.

- [ ] **Step 5b: Make the seam separator actually right-align**

The strip's trailing-separator branch calls `layout.E.Layout(gtx, lbl.Layout)`
to push the left pane's line against the seam. **It has never worked.**
`layoutFilePane`'s outer `layout.Stack` has only an `Expanded` child and no
`Stacked` ones, so Gio leaves `maxSZ` at zero, `Constraints.Min.X` arrives as 0
all the way down, and `layout.E` collapses to natural width. The left pane's
trailing `|` floats mid-pane instead of hugging the seam.

This was invisible before because the strip only ever showed transient progress
lines. The always-on info line makes it permanent, and the seam separator exists
precisely so adjacent strips read as one continuous band — so the feature's
visual design depends on it.

Fix it locally in `layoutFilePaneStatusBar`, immediately before the `layout.E`
call:

```go
			if trailingSeparator {
				// layout.E needs a Min width to align against; the pane's Stack
				// has no Stacked children, so Gio hands us Min.X == 0 and E
				// would collapse to natural width, leaving the seam separator
				// floating mid-pane.
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.E.Layout(gtx, lbl.Layout)
			}
```

Do not change `layoutFilePane`'s Stack — that would affect every pane child.
This also fixes the extraction and paste lines, which carry the same latent bug.

Note the existing unit test passes today only because it builds its context with
`layout.Exact` (Min == Max), which masks the defect. Add a case that uses a
context with `Min.X == 0` and `Max.X` set, and assert the label's right edge
lands at the pane's right edge. Prove by mutation that removing the new line
fails it.

- [ ] **Step 6: Write the branch-priority test**

The spec requires that extraction and paste progress win over the info line, and
that the progress lines keep working when the info bar is switched off. Add to
`ui/filepane_status_bar_test.go`:

```go
func TestStatusBarBranchPriority(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		extracting  bool
		wantExtract bool
		wantInfo    bool
	}{
		{"info line when idle", true, false, false, true},
		{"extraction wins over info", true, true, true, false},
		{"extraction survives the bar being off", false, true, true, false},
		{"nothing when off and idle", false, false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fm.DefaultConfig()
			cfg.StatusBar.Enabled = tc.enabled
			ui := NewUI(cfg)
			pane := ui.filePanes[0]
			pane.table.SetMode(table.ModeBrief)

			if tc.extracting {
				ui.archiveExtract = &archiveExtractState{
					pane:        0,
					archivePath: "/tmp/bundle.zip",
					startedAt:   time.Now(),
				}
			}

			gotExtract := ui.archiveExtractPane() == pane
			gotInfo := !gotExtract &&
				ui.directFilePasteForPane(0) == nil &&
				filePaneStatusBarVisible(ui.fmCfg, pane)

			if gotExtract != tc.wantExtract {
				t.Fatalf("extraction branch = %v, want %v", gotExtract, tc.wantExtract)
			}
			if gotInfo != tc.wantInfo {
				t.Fatalf("info branch = %v, want %v", gotInfo, tc.wantInfo)
			}
		})
	}
}
```

Add `"time"` to that file's imports. Check the real field names on
`archiveExtractState` with `grep -n "type archiveExtractState struct" -A 15
ui/archive_extract.go` and adjust the literal — the test only needs
`archiveExtractPane()` to return pane 0.

- [ ] **Step 7: Run the test to verify it passes**

Run: `go test ./ui/ -run 'StatusBarBranchPriority' -v`

Expected: PASS for all four cases. The third case is the important one: it is
what stops a future change from gating extraction feedback behind the status bar
setting.

- [ ] **Step 8: Build and run the ui suite**

Run: `go build ./... && go test ./ui/`

Expected: `ok hexone/ui`.

- [ ] **Step 9: Commit**

```bash
git add ui/filepane_status_bar.go ui/filepane_status_bar_test.go ui/filepane_volume_badge.go ui/filepane_volume_badge_test.go ui/filemanager_layout.go
git commit -m "feat(ui): show the file info line in the pane status bar"
```

---

## Task 7: Settings state, load and apply

**Files:**
- Modify: `ui/settings_modal.go`, `ui/settings_dirty.go`

Widget state and config plumbing first; the visible tab comes next. Splitting them keeps each diff small.

- [ ] **Step 1: Add the widget state**

In `ui/settings_modal.go`, in the `settingsModalState` struct beside `generalDimInactiveBool` (line 302):

```go
	statusBarEnabledBool         widget.Bool
	statusBarHideInFullBool      widget.Bool
	statusBarFieldBools          [7]widget.Bool
	paneSettingsStatusBarClick   widget.Clickable
```

The array is indexed by `filePaneStatusField`, so `statusBarFieldBools[filePaneStatusFieldSize]` is the Size checkbox. That coupling is why the order of the enum matters.

- [ ] **Step 2: Load from config**

In `ui/settings_modal.go`, beside `st.generalDimInactiveBool.Value = cfg.General.DimInactivePanes` (line 734):

```go
	st.statusBarEnabledBool.Value = cfg.StatusBar.Enabled
	st.statusBarHideInFullBool.Value = cfg.StatusBar.HideInFull
	for i := range st.statusBarFieldBools {
		st.statusBarFieldBools[i].Value = false
	}
	for _, field := range filePaneStatusFields(cfg.StatusBar.Fields) {
		st.statusBarFieldBools[field].Value = true
	}
```

- [ ] **Step 3: Apply back to config**

In `ui/settings_modal.go`, beside `ui.fmCfg.General.DimInactivePanes = st.generalDimInactiveBool.Value` (line 3322):

```go
	statusBarFields := st.statusBarSelectedFields()
	// Unchecking every field is how a user says "I don't want this bar". Honour
	// that literally: NormalizeStatusBarFields restores the defaults for an
	// empty list, so without this the checkboxes would visibly snap back to
	// size/date/free on the next open, which reads as a bug.
	ui.fmCfg.StatusBar.Enabled = st.statusBarEnabledBool.Value && len(statusBarFields) > 0
	ui.fmCfg.StatusBar.HideInFull = st.statusBarHideInFullBool.Value
	ui.fmCfg.StatusBar.Fields = fm.NormalizeStatusBarFields(statusBarFields)
```

Add the helper near the other `settingsModalState` methods in `ui/settings_modal.go`:

```go
// statusBarSelectedFields converts the checkbox array back into config keys, in
// canonical order.
func (st *settingsModalState) statusBarSelectedFields() []string {
	if st == nil {
		return nil
	}
	keys := []string{
		fm.StatusBarFieldSize, fm.StatusBarFieldDate, fm.StatusBarFieldPerms,
		fm.StatusBarFieldOwner, fm.StatusBarFieldItems, fm.StatusBarFieldSelection,
		fm.StatusBarFieldFree,
	}
	out := make([]string, 0, len(keys))
	for i, key := range keys {
		if st.statusBarFieldBools[i].Value {
			out = append(out, key)
		}
	}
	return out
}
```

**Why the `len(statusBarFields) > 0` guard matters.** `NormalizeStatusBarFields`
deliberately treats an empty list as "use the defaults" (Task 1), because a
hand-edited config with no valid keys should still produce a usable bar. But a
user who unticks all seven checkboxes in the UI is expressing the opposite
intent. Without the guard, they would save, reopen settings, and find
size/date/free ticked again — an apparent bug. Mapping "no fields" to "bar off"
is honest and discoverable: the **Show pane status bar** checkbox visibly
unticks, and re-ticking it brings back the default field set.

Add a test for exactly this, in `ui/settings_modal_test.go`:

```go
func TestStatusBarUncheckingEveryFieldDisablesTheBar(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := NewUI(cfg)
	// saveSettingsModal writes through saveFMConfigWithOptions to
	// ui.configSavePath(), which defaults to the user's REAL hexone.yaml.
	// Redirect it before opening the modal.
	ui.configPath = filepath.Join(t.TempDir(), "hexone.yaml")
	ui.openSettingsModal()
	st := ui.settingsModal

	st.statusBarEnabledBool.Value = true
	for i := range st.statusBarFieldBools {
		st.statusBarFieldBools[i].Value = false
	}

	if got := st.statusBarSelectedFields(); len(got) != 0 {
		t.Fatalf("selected fields = %v, want none", got)
	}

	if err := ui.saveSettingsModal(time.Now()); err != nil {
		t.Fatalf("saveSettingsModal: %v", err)
	}

	if ui.fmCfg.StatusBar.Enabled {
		t.Fatalf("unticking every field should disable the bar")
	}
	// The field list still normalises to the defaults, so re-enabling the bar
	// gives a usable configuration rather than an empty strip.
	if len(ui.fmCfg.StatusBar.Fields) == 0 {
		t.Fatalf("fields should fall back to the defaults, got none")
	}
}
```

The real names, already verified: the state lives on `ui.settingsModal`
(`ui/layout.go:219`) and the apply path is
`(ui *UI) saveSettingsModal(now time.Time) error` (`ui/settings_modal.go:3076`),
which is the function containing the apply block you edited.

**Verified names and the disk-write hazard.** `openSettingsModal()` takes no
arguments. `saveSettingsModal` really does write to disk — it calls
`saveFMConfigWithOptions("settings-modal", false)`, which writes to
`ui.configSavePath()`, defaulting to the user's real `hexone.yaml` via
`appdata.ConfigPath()`. **Every test touching the save path must override
`ui.configPath` to a `t.TempDir()` file first**, which is what the existing
settings tests do. Read the config back with `fm.LoadConfig(ui.configPath)` to
prove the temp file is the one written.

- [ ] **Step 4: Include the settings in the dirty signature**

In `ui/settings_dirty.go`, the `PaneBehavior` line (line 53) currently reads:

```go
		PaneBehavior: fmt.Sprintf("%t|%t|%t|%t|%t|%q",
			st.generalDimInactiveBool.Value,
			st.generalFavoritesNewTabBool.Value,
			st.generalWheelMovesSelection.Value,
			st.generalUseTrash.Value,
			st.generalDeleteWithoutConfirm.Value,
			st.generalCompletionSound),
```

The snapshot is an anonymous struct declared at the top of `draftSignature()`
(`ui/settings_dirty.go:11`). Add a field to it:

```go
	snapshot := struct {
		PaneColors, ViewerColors, FilenameDefaults string
		FilenameRules                              string
		ViewerFields, ViewerEntries                string
		Fonts, PaneAppearance, PaneBehavior        string
		PaneColumns, PaneDates, Terminal           string
		StatusBar                                  string
		ConfigYAML                                 string
	}{
```

and a matching entry in the literal, beside `PaneBehavior`:

```go
		StatusBar: fmt.Sprintf("%t|%t|%q",
			st.statusBarEnabledBool.Value,
			st.statusBarHideInFullBool.Value,
			st.statusBarSelectedFields()),
```

Without this, Cancel and the unsaved-changes prompt will not notice status bar edits.

- [ ] **Step 5: Build and test**

Run: `go build ./... && go test ./ui/`

Expected: `ok hexone/ui`.

- [ ] **Step 6: Commit**

```bash
git add ui/settings_modal.go ui/settings_dirty.go
git commit -m "feat(ui): plumb status bar settings through the settings modal"
```

---

## Task 8: Settings tab and keyboard navigation

**Files:**
- Modify: `ui/settings_filepanes.go`, `ui/settings_modal_keyboard.go`

- [ ] **Step 1: Register the tab**

In `ui/settings_filepanes.go`, `normalizeSettingsPaneMode` (line 41) must accept the new key:

```go
func normalizeSettingsPaneMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "brief":
		return "brief"
	case "statusbar":
		return "statusbar"
	case "other":
		return "other"
	default:
		return "full"
	}
}
```

In `layoutSettingsFilePaneEditor` (line 373), add to the `modeClicks` slice between brief and other:

```go
		{key: "statusbar", label: "Status bar", click: &st.paneSettingsStatusBarClick},
```

**Easy to miss:** the sliding tab indicator has its own hard-coded key list a few
lines below (line 404). It must match the tab order or the highlight lands on the
wrong tab:

```go
	pos, posAnim := st.paneSettingsModeAnim.position(gtx.Now, mode, []string{"full", "brief", "statusbar", "other"})
```

Then extend the dispatch switch at the bottom of the same function (line 430):

```go
			switch mode {
			case "brief":
				return ui.layoutSettingsPaneBriefTab(th, gtx, st)
			case "statusbar":
				return ui.layoutSettingsPaneStatusBarTab(th, gtx, st)
			case "other":
				return ui.layoutSettingsPaneOtherTab(th, gtx, st)
			default:
				return ui.layoutSettingsPaneFullTab(th, gtx, st)
			}
```

- [ ] **Step 2: Write the tab body**

Append to `ui/settings_filepanes.go`:

```go
type settingsStatusBarFieldRow struct {
	field filePaneStatusField
	label string
}

func settingsStatusBarFieldRows() []settingsStatusBarFieldRow {
	return []settingsStatusBarFieldRow{
		{filePaneStatusFieldSize, "Size"},
		{filePaneStatusFieldDate, "Date"},
		{filePaneStatusFieldPerms, "Permissions"},
		{filePaneStatusFieldOwner, "Owner / group"},
		{filePaneStatusFieldItems, "Item count"},
		{filePaneStatusFieldSelection, "Marked selection"},
		{filePaneStatusFieldFree, "Free space"},
	}
}

func (ui *UI) layoutSettingsPaneStatusBarTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	sectionLabel := func(txt string) layout.Widget { return settingsViewerRowLabel(ui, th, txt, true) }

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			before := st.statusBarEnabledBool.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.statusBarEnabledBool, "Show pane status bar", ui.scaleModalFontSize(10))
			if before != st.statusBarEnabledBool.Value {
				st.focus = settingsKeyboardFocusStatusBarEnabled
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusStatusBarEnabled, &st.statusBarEnabledBool)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !st.statusBarEnabledBool.Value {
				gtx = gtx.Disabled()
			}
			before := st.statusBarHideInFullBool.Value
			dims := ui.layoutThemeCheckbox(th, gtx, &st.statusBarHideInFullBool, "Hide it in full mode", ui.scaleModalFontSize(10))
			if before != st.statusBarHideInFullBool.Value {
				st.focus = settingsKeyboardFocusStatusBarHideInFull
			}
			st.applyPendingWidgetFocus(gtx, settingsKeyboardFocusStatusBarHideInFull, &st.statusBarHideInFullBool)
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(sectionLabel("Fields")),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
	}

	for _, row := range settingsStatusBarFieldRows() {
		field := row.field
		label := row.label
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !st.statusBarEnabledBool.Value {
					gtx = gtx.Disabled()
				}
				return ui.layoutThemeCheckbox(th, gtx, &st.statusBarFieldBools[field], label, ui.scaleModalFontSize(10))
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		)
	}

	children = append(children,
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsStatusBarPreview(th, gtx, st)
		}),
	)

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// layoutSettingsStatusBarPreview renders a sample line through the same builder
// the live pane uses, so the preview cannot drift from what ships.
func (ui *UI) layoutSettingsStatusBarPreview(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	return ui.layoutSettingsPanePreviewFrame(th, gtx, "STATUS BAR PREVIEW", st, func(gtx layout.Context) layout.Dimensions {
		palette := ui.settingsPaneDraftPalette(st)
		bg, border, textColor := filePaneVolumeBadgeColors(palette)
		return layout.S.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layoutFilePaneStatusBarBox(gtx, bg, border, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, ui.settingsStatusBarPreviewLine(th, gtx, st))
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleThemeFontSize(th, 11)
					lbl.Color = textColor
					lbl.MaxLines = 1
					lbl.Truncator = ""
					return lbl.Layout(gtx)
				})
			})
		})
	})
}

func (ui *UI) settingsStatusBarPreviewLine(th *material.Theme, gtx layout.Context, st *settingsModalState) string {
	pane := settingsStatusBarPreviewPane(ui.fmCfg)
	fields := filePaneStatusFields(fm.NormalizeStatusBarFields(st.statusBarSelectedFields()))
	const previewFree = uint64(41) << 30
	const previewTotal = uint64(100) << 30
	measure := func(text string) int {
		return ui.measureFilePaneStatusBarTextWidth(th, gtx, text)
	}
	textMax := max(gtx.Constraints.Max.X-gtx.Dp(unit.Dp(16)), 0)
	return filePaneStatusLineForWidth(pane, fields, previewFree, previewTotal, textMax, measure)
}

// settingsStatusBarPreviewPane builds a fixed sample pane for the preview.
func settingsStatusBarPreviewPane(cfg *fm.Config) *filePaneState {
	pane := &filePaneState{}
	pane.model = &filePaneModel{
		cfg: cfg,
		entries: []filesys.Entry{
			{Name: "..", Kind: filesys.EntryParent},
			{
				Name: "report.pdf", Kind: filesys.EntryFile,
				SizeBytes: 2516582, DateText: "2026-08-30 14:22",
				PermText: "-rw-r--r--", PermOctal: "0644", OwnerText: "demo:staff",
			},
		},
	}
	pane.table = table.New(nil)
	pane.table.Selected = 1
	// Mark a row so the "Marked selection" field has something to render.
	// Without this, hasMarkedRows() is false, the field renders empty and gets
	// dropped along with its separator — so ticking that checkbox would leave
	// the preview visibly unchanged, which defeats the point of having one.
	pane.markedRows = map[int]struct{}{1: {}}
	return pane
}
```

**Verify this actually works**: tick every field in the settings tab and confirm
the preview line changes for each one, including Selection. A field that renders
empty is silently dropped, so a preview that ignores a checkbox looks like a
broken checkbox.

Check the imports of `ui/settings_filepanes.go`; it will need `hexone/filesys` and `hexone/ui/widget/table` if they are not already there.

- [ ] **Step 3: Add keyboard focus constants**

In `ui/settings_modal_keyboard.go`, add to the focus const block (line 18), after `settingsKeyboardFocusGeneralCompletionSound`:

```go
	settingsKeyboardFocusStatusBarEnabled
	settingsKeyboardFocusStatusBarHideInFull
```

Register them as widget focus targets in `isWidgetFocusTarget` (line 179), adding to the case list:

```go
		settingsKeyboardFocusStatusBarEnabled,
		settingsKeyboardFocusStatusBarHideInFull,
```

Add them to `syncFocusedWidget` (line 227):

```go
	case gtx.Focused(&st.statusBarEnabledBool):
		st.focus = settingsKeyboardFocusStatusBarEnabled
	case gtx.Focused(&st.statusBarHideInFullBool):
		st.focus = settingsKeyboardFocusStatusBarHideInFull
```

Add them to `toggleFocusedCheckbox` (line 563):

```go
	case settingsKeyboardFocusStatusBarEnabled:
		st.statusBarEnabledBool.Value = !st.statusBarEnabledBool.Value
		return true
	case settingsKeyboardFocusStatusBarHideInFull:
		st.statusBarHideInFullBool.Value = !st.statusBarHideInFullBool.Value
		return true
```

Add the tab's focus order in `focusOrder` (line 319), inside the `case "general":` switch on pane mode:

```go
		case "statusbar":
			order = append(order,
				settingsKeyboardFocusStatusBarEnabled,
				settingsKeyboardFocusStatusBarHideInFull,
			)
```

- [ ] **Step 3b: Write down the "adding an eighth field" checklist**

By the end of this task the status bar field set is spread across roughly eight
hand-maintained lists in six files. Every one has a local "keep this in sync"
comment, but nothing enumerates the whole set, and several failures are silent —
a missing entry drops the field from every user's config, or panics at runtime.

Task 8 is the first point where the full list is actually knowable. Write it
down as a comment directly above the `filePaneStatusField` enum in
`ui/filepane_status_fields.go`, since that enum is what someone adding a field
will edit first. Enumerate the real sites — verify each by grep rather than
copying this list, which may have drifted:

- `fm/config.go` — the `StatusBarField*` constant and `statusBarFieldOrder`
- `fm/config_test.go` — the roster in `TestStatusBarFieldOrderCoversEveryFieldConstant`
- `ui/filepane_status_fields.go` — the enum, `filePaneStatusFieldFromConfigKey`,
  `filePaneStatusDropOrder`, and the `filePaneStatusFieldValue` switch
- `ui/filepane_status_fields_test.go` — `allFilePaneStatusFields`
- `ui/settings_modal.go` — `statusBarFieldCount`
- `ui/settings_filepanes.go` — `settingsStatusBarFieldRows`
- `HELP.md` — the field table

Note which failures are loud (a guard test fires) and which are silent, so the
next person knows where the danger is. Keep it to a short list; this is a
signpost, not documentation.

- [ ] **Step 4: Build and test**

Run: `go build ./... && go test ./ui/`

Expected: `ok hexone/ui`.

- [ ] **Step 5: Verify the tab renders**

Use the project's `verify` skill, or run the existing settings headless test to confirm nothing regressed:

```bash
go test -tags uiverify ./ui/ -run TestHeadlessSettings -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add ui/settings_filepanes.go ui/settings_modal_keyboard.go
git commit -m "feat(ui): add the status bar settings tab"
```

---

## Task 8b: Make the volume lookup async

**Files:**
- Modify: `ui/filepane_volume_badge.go`
- Test: `ui/filepane_volume_badge_test.go`

**Must land before Task 9**, whose PNG goldens would otherwise bake in the
current behaviour.

### Why this task exists

This was not in the original plan. It was found during Task 6 review, and it is
a direct consequence of this feature's design rather than a pre-existing bug we
merely inherited.

`filePaneStatusInfoLine` calls `ui.filePaneVolumeBadgeLabel(pane, gtx.Now)` from
a layout path. The badge did that too, so the *shape* is pre-existing — but the
blast radius changed:

| | Before this feature | After Task 6 |
| --- | --- | --- |
| Panes polled per frame | at most 1, always the **active** pane | **every visible pane**, its own volume |
| Single-pane layout | zero polls (`layoutFilePaneVolumeBadge` needs ≥2 panes) | one poll |
| Terminal focused | suppressed by `filePaneVolumeBadgesHidden` | still polls |
| Two SFTP panes | one blocking round trip | **two independent** blocking round trips |

What is actually on that path:

- `filePaneVolumeBadgeLabel` computes `pane.filePaneVolumeLookupPath()` **before**
  the 15s cache check, and for a local pane that runs `nearestExistingLocalPath`
  → `os.Stat`. So it is **one uncached `os.Stat` per pane per frame** — 60/s per
  pane during a scroll or resize drag, and a blocking one on a stale SMB/NFS
  mount.
- On the cache-miss branch for SFTP: blocking `StatVFS`, then `df` with a 4s
  timeout, then `reconnectSFTPClient` → a synchronous SSH dial with a **12 second**
  budget (`sshConnectBudget`, `ui/ssh_remote.go:72`). **One frame can block for
  ~12 seconds.**
- The failure path sets `nextRefreshAt = now + 4s` *and* schedules an
  `InvalidateCmd`, so a broken remote self-drives a frame every 4s that blocks
  again — a permanent stall loop, not a one-off.

CONTRIBUTING.md is unambiguous: "If you find yourself calling a slow function
directly in a layout path, that is the bug."

### Step 1: The cheap fix first, on its own

Hoist the lookup-path computation behind the cache check in
`filePaneVolumeBadgeLabel`. Today the `os.Stat` runs every frame even on a cache
hit; it only needs to run when the cache is about to be refreshed, or when the
pane's directory has changed. This removes the per-frame syscall — the cost this
feature actually introduced — independently of the async work, and it is a small,
reviewable change.

Pin it with a test that counts `localVolumeUsageFunc` calls **and** `os.Stat`
calls across many frames with an unchanged directory, asserting both stay at one.
`localVolumeUsageFunc` is already a package var, so it is stubbable; you may need
a seam for the stat.

### Step 2: Convert to the project's async pattern

Follow the documented `start` / sequence / `pump` shape from
[ARCHITECTURE.md](../../../ARCHITECTURE.md) — `fileViewerState` is the reference
example with four such pipelines:

- a `startVolumeLookup` spawns a goroutine, stores a cancel func and a result
  channel on `filePaneVolumeBadgeState`, and bumps a sequence number
- the goroutine sends a result carrying that sequence number
- a `pumpVolumeLookup` drains the channel at frame start (add it to the pump
  block at the top of `ui.Layout`) and **discards results whose sequence no
  longer matches**

The layout path then only ever reads the last landed result and never blocks.
`totalBytes == 0` already means "not landed yet" and renders the field empty
(Task 3), so the first frame after a directory change degrades gracefully with no
extra work.

Keep the 15s refresh interval and the 4s retry-after-failure, but drive them from
the pump rather than from a layout-path `InvalidateCmd`.

### Step 3: Verify

- `go test ./ui/` and `go test -tags uiverify ./ui/`
- `go test -race ./ui/` — this is new concurrency; the race detector matters here
- Confirm a pane whose volume lookup fails does not schedule a 4s repaint loop
- `GOTOOLCHAIN=go1.26.6 go run ./tools/unusedcheck`

### If this task is descoped

It is defensible to ship Step 1 alone and defer Step 2, since Step 1 removes the
new per-frame cost and the blocking remote call is pre-existing. If that is the
call, say so explicitly in `CHANGELOG.md` and leave a `// TODO` referencing this
section — do not let it disappear silently.

---

## Task 9: Headless pixel verification

**Files:**
- Create: `ui/filepane_status_bar_headless_verify_test.go`

This is the test that catches what unit tests structurally cannot: whether the bar actually paints, at the right place, without clipping the pane body.

- [ ] **Step 1: Write the test**

Create `ui/filepane_status_bar_headless_verify_test.go`, modelled on `ui/brief_layout_headless_verify_test.go`:

```go
// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build uiverify

package ui

import (
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

	"hexone/fm"
	"hexone/ui/widget/table"
)

func TestHeadlessFilePaneStatusBar(t *testing.T) {
	outDir := os.Getenv("UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}

	dir := t.TempDir()
	for _, name := range []string{"alpha.txt", "beta.log", "gamma.md", "notes", "src"} {
		path := filepath.Join(dir, name)
		if filepath.Ext(name) == "" {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatalf("create directory %s: %v", name, err)
			}
			continue
		}
		if err := os.WriteFile(path, []byte("sample contents"), 0o600); err != nil {
			t.Fatalf("create file %s: %v", name, err)
		}
	}

	cases := []struct {
		name       string
		mode       table.Mode
		hideInFull bool
		fields     []string
	}{
		{"brief-with-bar", table.ModeBrief, false, []string{"size", "date", "free"}},
		{"full-with-bar", table.ModeFull, false, []string{"size", "date", "perms", "free"}},
		{"full-hidden", table.ModeFull, true, []string{"size", "date", "free"}},
		{"brief-all-fields", table.ModeBrief, false, []string{"size", "date", "perms", "owner", "items", "selection", "free"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fm.DefaultConfig()
			cfg.StatusBar.Enabled = true
			cfg.StatusBar.HideInFull = tc.hideInFull
			cfg.StatusBar.Fields = fm.NormalizeStatusBarFields(tc.fields)

			const width, height = 1200, 620
			win, err := headless.NewWindow(width, height)
			if err != nil {
				t.Fatalf("create headless window: %v", err)
			}
			defer win.Release()

			th := material.NewTheme()
			th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
			ui := NewUI(cfg)
			for index, pane := range ui.filePanes {
				if pane == nil {
					continue
				}
				pane.table.SetMode(tc.mode)
				ui.requestPaneLoadWithSelection(index, dir, "", "", 0)
			}

			router := new(input.Router)
			frame := func() *image.RGBA {
				var ops op.Ops
				gtx := layout.Context{
					Ops:         &ops,
					Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
					Constraints: layout.Exact(image.Pt(width, height)),
					Now:         time.Now(),
					Source:      router.Source(),
				}
				ui.Layout(th, gtx)
				router.Frame(&ops)
				if err := win.Frame(&ops); err != nil {
					t.Fatalf("render frame: %v", err)
				}
				img := image.NewRGBA(image.Rect(0, 0, width, height))
				if err := win.Screenshot(img); err != nil {
					t.Fatalf("capture frame: %v", err)
				}
				return img
			}

			var img *image.RGBA
			loaded := false
			for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
				img = frame()
				loaded = true
				for _, pane := range ui.filePanes {
					if pane == nil || pane.dir != dir || pane.findEntryIndex("alpha.txt") < 0 {
						loaded = false
						break
					}
				}
				if loaded {
					break
				}
				time.Sleep(12 * time.Millisecond)
			}
			if !loaded {
				t.Fatal("synthetic directory did not load")
			}
			for i := 0; i < 4; i++ {
				img = frame()
			}

			path := filepath.Join(outDir, "status-bar-"+tc.name+".png")
			file, err := os.Create(path)
			if err != nil {
				t.Fatalf("create screenshot: %v", err)
			}
			if err := png.Encode(file, img); err != nil {
				file.Close()
				t.Fatalf("encode screenshot: %v", err)
			}
			if err := file.Close(); err != nil {
				t.Fatalf("close screenshot: %v", err)
			}
			t.Logf("wrote %s", path)
		})
	}
}
```

- [ ] **Step 2: Run it and keep the frames**

```bash
UI_VERIFY_OUT=/tmp/hexone-status-bar go test -tags uiverify ./ui/ -run TestHeadlessFilePaneStatusBar -v
```

Expected: PASS, four PNGs written.

- [ ] **Step 3: Look at the frames**

Open the four PNGs and confirm, by eye:

- `brief-with-bar` — a strip at the bottom of both panes, each showing its own free space, no floating badge anywhere
- `full-with-bar` — the strip present, the file grid above it not clipped
- `full-hidden` — no strip, and no floating badge either (free space is gone in this configuration, by design)
  - **Superseded 2026-09-03, after live user review:** this frame now shows no strip but *does* show the floating badge, pinned to the inactive pane's inner corner and carrying the active pane's reading. "Free space is gone" was the reported bug — see the dated note in Task 6, Step 3.
- `brief-all-fields` — fields dropped from the right end as configured, nothing overflowing the pane edge

If the pane body is clipped, the `layout.Rigid` wrapping the strip is taking height from the wrong place — check the flex in `layoutFilePane` (`ui/filemanager_layout.go:706`).

- [ ] **Step 4: Commit**

```bash
git add ui/filepane_status_bar_headless_verify_test.go
git commit -m "test(ui): headless pixel verification for the pane status bar"
```

---

## Task 10: Documentation and final verification

**Files:**
- Modify: `HELP.md`, `CHANGELOG.md`, `ARCHITECTURE.md`

- [ ] **Step 1: Document the feature in HELP.md**

Find the file pane section of `HELP.md` and add:

```markdown
### Pane status bar

Each file pane has a compact status bar along its bottom edge describing the
entry your cursor is on. It is most useful in brief mode, where the grid shows
filenames only.

| Field | Shows |
| --- | --- |
| Size | The entry's size; `<DIR>` for directories |
| Date | Modification time, in your configured date format |
| Permissions | `-rw-r--r--` or `0644`, following the permissions column setting |
| Owner / group | The owning user and group. Blank on Windows local files; numeric for SFTP |
| Item count | How many entries the directory holds |
| Marked selection | How many files are marked and their combined size |
| Free space | Free and total space on the pane's volume, with a usage bar |

Configure it under **Settings → File panes → Status bar**:

- **Show pane status bar** turns it off entirely.
- **Hide it in full mode** keeps it in brief mode only, where size and date are
  not already columns. Note that free space disappears along with the bar.
  - **Superseded 2026-09-03, after live user review:** free space does not
    disappear — hiding the bar hands it back to the floating badge, exactly as
    turning the bar off does. The shipped `HELP.md` wording says so; see the
    dated note in Task 6, Step 3.

When a pane is extracting an archive or pasting files, the status bar shows that
operation's progress instead, and returns to file information when it finishes.

When the free space field is enabled, free space is shown in each pane's own
status bar rather than as a floating badge over the opposite pane.
```

- [ ] **Step 2: Add a changelog entry**

In `CHANGELOG.md`, under the pre-1.3.0 heading:

```markdown
- Added a per-pane status bar showing the selected entry's size, modification
  date and other configurable fields, so brief mode no longer hides them.
  Configure it under Settings → File panes → Status bar.
- Free space now appears in each pane's own status bar instead of as a floating
  badge over the opposite pane. Turn off the Free space field to remove it.
```

The second bullet matters: it is a visible change for existing installations.

- [ ] **Step 3: Update the architecture map**

In `ARCHITECTURE.md`, in the `ui/` cluster table, add `filepane_status_*.go` to the **File manager** row's file list.

- [ ] **Step 4: Run the full test matrix**

```bash
go test ./...
```

Expected: all packages `ok`.

```bash
go test -tags pdfium ./ui/
```

Expected: `ok hexone/ui`.

```bash
go test -tags uiverify ./ui/
```

Expected: `ok hexone/ui`.

- [ ] **Step 5: Check for dead code**

**Use the pinned toolchain, not bare `make unused`:**

```bash
GOTOOLCHAIN=go1.26.6 go run ./tools/unusedcheck
```

Expected (verified working 2026-08-31):

```
analyzed 5 configuration(s): darwin, windows, uiverify, pdfium, pdfium+uiverify
skipped 1: linux — run this there to cover them
no declaration is unused in every configuration that can see it
```

**Why not plain `make unused`.** It fails on this machine with
`no configuration could be analyzed`, skipping all six configurations. The cause
is a toolchain mismatch, not this feature: the local Go toolchain is 1.27.0
while the pinned staticcheck (v0.7.0) cannot decode Go 1.27 export data
(`export data version 4 is greater than maximum supported version 2`). It fails
identically on a pristine `HEAD` checkout. Pinning `GOTOOLCHAIN` to the version
in `go.mod` reproduces what CI actually runs, and it works.

Do not "fix" this by downgrading the system Go or editing the allowlist — that
is the user's call. Just use the `GOTOOLCHAIN` form.

The windows configuration is analyzed, so a `statOwnerIDs` that looks dead on
macOS is correctly recognised as live. If a genuinely dead symbol is reported,
delete it rather than allowlisting it; `tools/unusedcheck/allowlist.txt` is only
for symbols kept alive by a configuration the local machine cannot analyze
(currently just linux), and each entry needs its reason.

- [ ] **Step 6: Cross-compile check**

```bash
GOOS=windows go build ./...
```

Expected: no output. Linux cross-compilation from macOS fails for unrelated reasons (Gio needs cgo) — do not treat that as a regression.

- [ ] **Step 7: Commit**

```bash
git add HELP.md CHANGELOG.md ARCHITECTURE.md
git commit -m "docs: document the file pane status bar"
```

---

## Verification checklist

Before calling this done, confirm each of these by running it, not by reasoning about it:

- [ ] `go test ./...` passes
- [ ] `go test -tags pdfium ./ui/` passes
- [ ] `go test -tags uiverify ./ui/` passes
- [ ] `GOTOOLCHAIN=go1.26.6 go run ./tools/unusedcheck` reports nothing unused (plain `make unused` cannot run here — see Task 10)
- [ ] `GOOS=windows go build ./...` succeeds
- [ ] The four headless PNGs from Task 9 look right
- [ ] Settings → File panes → Status bar toggles the bar live, and Cancel discards changes


---

# Revision 2 — anchored columnar layout

The user rejected Revision 1's joined-string rendering after seeing it live. The
new design is specified in the design doc's "Revision 2" section: a left-anchored
cluster of fixed-width columns (Name always first, then the enabled fields,
joined by `  •  `), a right-anchored free-space region (`519.71 GB free (56%)`)
preceded by `│`, marked rows replacing the left cluster with
`N items selected  •  <size>`, name compaction via the existing
`filePaneModel.compactName`, and column widths that never depend on the selected
entry.

Execution is three tasks, same rules as Tasks 1–10: TDD, nothing committed or
staged, full verification per task.

### Task R1: Field layer for the new design

**Files:** `fm/config.go`, `fm/config_test.go`, `ui/filepane_status_fields.go`,
`ui/filepane_status_fields_test.go`, plus every guard-test roster the checklist
comment names.

- Retire the `selection` config key: remove `StatusBarFieldSelection` from the
  constants and `statusBarFieldOrder`; normalization then drops it from existing
  configs like any unknown key. Update the fm guard-test roster.
- Remove `filePaneStatusFieldSelection` from the ui enum, the drop order, the
  config-key mapping, `allFilePaneStatusFields`, and `statusBarFieldCount`
  (7 → 6). The checklist comment above the enum enumerates every site.
- Add `filePaneStatusFieldName` as a new enum value that is NOT a config key —
  it is always rendered, has no checkbox, and drops last. Decide its
  representation cleanly (it may live outside the config-driven field list).
- New free-space format: `%s free (%d%%)` where the percentage is
  free/total rounded. Retire the `filePaneStatusFreeForm` ladder and the
  `textCellProgressBar` usage.
- New marked-mode builder: `N items selected  •  <combined size>`
  (`1 item selected` singular), replacing the old selection field text.
- Name value: the entry display name, compacted by
  `filePaneModel.compactName` when over its column width.

### Task R2: Anchored two-region layout

**Files:** `ui/filepane_status_bar.go`, `ui/filepane_status_bar_test.go`,
`ui/filepane_status_bar_headless_verify_test.go`.

- Replace the info branch's single-label rendering with a real flex row:
  fixed-width column labels joined by `  •  `, a flexed spacer, `│`, the
  free-space label. Retire `filePaneStatusLineForWidth`'s role for the info line
  and the seam `|`/`| ` separators on the info line only — progress lines keep
  string rendering and seam behaviour untouched.
- Column widths per the design doc's table: name from the widest listing entry
  (capped by remaining space), size/date/perms from measured samples, owner from
  the widest listing owner (capped), items natural. Widths are cached per
  directory + config, not recomputed per frame from scratch, and never depend on
  the cursor row.
- Degradation: shrink the name column first via `compactName`; below a usable
  name width drop columns owner → items → perms → free → date → size.
- Marked mode replaces the left cluster wholesale.
- Headless pixel tests updated: assert the name column starts at the pane's
  left inset, the free-space text ends at the right inset, moving the cursor
  between a short-named and a long-named entry moves **nothing** (byte-compare
  the strip's pixel rows outside the name column), and the marked-mode line.

### Task R3: Settings, docs, and the matrix

**Files:** `ui/settings_filepanes.go`, `ui/settings_modal.go`,
`ui/settings_statusbar_tab_test.go`, `ui/settings_statusbar_headless_verify_test.go`,
`HELP.md`, `CHANGELOG.md`.

- Remove the Marked selection checkbox; six field checkboxes remain. The
  preview renders the anchored layout, both modes (a second preview line or a
  toggle for marked mode — implementer's call, but marked mode must be visible
  somewhere in the preview).
- Update HELP.md's section and field table to the new rendering; the CHANGELOG
  v1.3.0 entry still describes the feature accurately (adjust wording only if it
  named the retired bar/ladder).
- Full matrix: `go test ./...`, `-tags pdfium`, `-tags uiverify`, `-race`,
  `go build`, `GOOS=windows go build`, `gofmt`, `go vet`,
  `GOTOOLCHAIN=go1.26.6 go run ./tools/unusedcheck`.

### Revision 2.1 amendments (2026-09-01, user review of R2)

The design doc's "Revision 2.1 amendments" section landed after the tasks above
were written, and execution followed it where the two disagree:

- The item count field is retired with `selection`, so R1's field set is FIVE
  checkboxes, not six, and `items` leaves the width table and the drop order
  (owner → perms → free → date → size).
- The bar gains its own `status_bar.date_format` key (`auto | iso | us |
  short`) with a `layoutSettingsShellPicker`-style picker in the Status bar
  tab, labelled with rendered samples derived from `fm.StatusBarDateLayout`.
- Task R3 executed as two tasks: R3a (field retirement plus the date-format
  config, state and normalization plumbing) and R3b (the picker's layout and
  keyboard wiring, the anchored preview reusing the live layout path — laid out
  wide enough that no field ever drops from it and scaled into the frame — and
  the docs).
- R3b's preview was reworked after review: its two captioned strips (cursor and
  marked selection) read as a diagram rather than a picture of the app. It is
  now the pane mock the Full and Brief previews are — a brief-mode grid of
  sample rows with one under the cursor — carrying ONE status bar along its
  bottom edge, describing that highlighted row. The marked-mode summary is not
  previewed at all: it is automatic, so no checkbox on the tab can change it.
