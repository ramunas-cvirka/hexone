# File pane status bar — design

Date: 2026-08-31

## Problem

Brief mode shows filenames only. To read a file's size or modification date you
must switch the pane to full mode, which costs the column density brief mode
exists to provide.

Separately, free space is shown by `layoutFilePaneVolumeBadge` as a floating
badge drawn over the *inactive* pane, reporting the *active* pane's volume. It
occupies pane space, and the indirection ("the number over there describes the
directory over here") is hard to read at a glance.

## Solution

A compact per-pane status bar pinned to the bottom of each file pane, styled
like the existing archive-extraction status line, showing the pane's own cursor
entry. Free space becomes one of its fields, replacing the badge.

## Scope

In scope: the status bar, its configuration, the settings UI for it, owner/group
support in listings, removal of the volume badge when free space moves into the
bar.

Out of scope: user-defined field order, full path and symlink target fields
(considered and rejected), per-pane configuration (the settings are global).

---

## 1. Bar content

### Style

Fields are joined with `" | "`, matching the extraction line
(`[Extracting] file.zip | ██████░░░░ 60% | 4.2 MB/s | 0:12 left`). Sub-parts
within a single field are joined with `" · "`, giving two levels of separation.

Free space carries an ASCII usage bar built with the existing
`textCellProgressBar` (`█` / `░`, `ui/fileop_dialog_style.go:81`), which both
matches the extraction bar's visual language and makes disk fullness legible
without reading the numbers.

```
│ 2.40 MB | 2026-08-30 14:22 | -rw-r--r-- | ramunas:staff | 142 items | ██████░░░░ 41.20 GB free / 465.00 GB │
```

No filename is shown. Brief mode already displays it in the grid, and omitting
it buys width for the fields that are otherwise unavailable.

### Fields

Canonical display order, which is also the declaration order of the enum:

| Field | Key | Renders | Source |
| --- | --- | --- | --- |
| Size | `size` | `2.40 MB`; `<DIR>` for directories; `<UP>` for `..` | `Entry.SizeBytes`, `Entry.Kind` |
| Date | `date` | the item's modification time in the user's configured format | `Entry.ModTime` via `filePaneModel.formatDate` — **not** `Entry.DateText`, which `filesys.formatDate` bakes with a hardcoded `Jan 02 2006` and which ignores `date_formats` entirely |
| Permissions | `perms` | `-rw-r--r--` or `0644` | `Entry.PermText` / `Entry.PermOctal`, selected by the existing `columns.permission_format` |
| Owner / group | `owner` | `ramunas:staff`; `501:20` for SFTP | `Entry.OwnerText` (new, §4) |
| Item count | `items` | `142 items` | `len(model.entries)`, excluding the `..` parent row |
| Marked selection | `selection` | `3 marked · 12.10 MB` | `filePaneState.markedRows` |
| Free space | `free` | `██████░░░░ 41.20 GB free / 465.00 GB`, with two shorter forms (see below) | existing `filePaneVolumeBadgeLabel` |

A field that has no value for the current entry renders as an empty string and
is skipped entirely, taking its separator with it. This covers: `selection` when
nothing is marked, `owner` on Windows local files and inside archives, and
`perms` and `date` on the `..` row, which carries neither.

Size formatting reuses `formatFilePaneVolumeBytes`
(`ui/filepane_volume_badge.go:273`) so the bar agrees with the free-space field
about what a megabyte looks like.

The free-space field composes three parts, in this order:

```
██████░░░░ 41.20 GB free / 465.00 GB
└ usage bar   └ free / total, via formatFilePaneVolumeBytes
```

The usage bar is `textCellProgressBar(used/total, 10)` — note it fills with
*used* fraction, not free. The label is the existing
`formatFilePaneVolumeBadgeLabel`, reused verbatim so the badge and the bar can
never disagree.

The field has two shorter forms, tried in order before the field is dropped
entirely by the degradation loop:

| Form | Rendering |
| --- | --- |
| full | `██████░░░░ 41.20 GB free / 465.00 GB` |
| no total | `██████░░░░ 41.20 GB free` |
| no bar | `41.20 GB free` |

This is the only field with internal shortening; every other field is already
minimal.

### Width degradation

The bar reuses the measure-and-shrink loop from
`archiveExtractStatusLineForWidth` (`ui/archive_extract.go:462`): build the
line, measure it, and if it overflows the available width, drop the
lowest-priority field and remeasure.

Display order and drop order are deliberately **different**. Drop order, first
dropped to last:

```
owner → items → perms → free → selection → date → size
```

Size and date survive longest because seeing them in brief mode is the reason
this feature exists. Owner and item count go first as the least essential.
Free space is dropped before selection because it is periodic ambient
information, whereas a marked-file summary is only present when the user is
actively marking.

The loop runs in two phases. First it tries the free-space field's shorter
forms, since losing a total is cheaper than losing a whole field. Only when the
line still overflows does it start dropping fields in the order above.

There is no middle-truncation step. Without a filename in the line every field
is short and bounded, so shortening free space and then dropping whole fields is
sufficient.

---

## 2. Configuration

A new top-level block in `hexone.yaml`, sibling to `columns`, `sort`, `tabs`
and `viewer`:

```yaml
status_bar:
  enabled: true
  hide_in_full: false
  fields: [size, date, free]
```

New type in `fm/config.go`:

```go
type StatusBarConfig struct {
    Enabled    bool     `yaml:"enabled"`
    HideInFull bool     `yaml:"hide_in_full"`
    Fields     []string `yaml:"fields"`
}
```

Hung off `Config` as `StatusBar StatusBarConfig \`yaml:"status_bar"\``, and added
to the raw struct inside `Config.UnmarshalYAML`.

`StatusBarConfig.UnmarshalYAML` follows the compat pattern already used by
`GeneralConfig` and `ColumnWidths`: decode into a raw struct whose `Enabled` is
a `*bool` so that an absent key means `true` rather than `false`, then overlay
onto the defaults.

Normalisation, applied on load and after the settings modal writes:

- unknown field keys are dropped
- duplicates are removed
- the result is sorted into canonical display order

So a hand-edited config cannot produce an unexpected field arrangement, and the
rendering order is a property of the code rather than of the file.

`len(Fields) == 0` resolves to the defaults. An explicitly empty list is not
treated as "a bar with no fields", because that is indistinguishable in effect
from `enabled: false`.

### Defaults

| Setting | Default | Reason |
| --- | --- | --- |
| `enabled` | `true` | The feature is the point; shipping it off by default hides it. |
| `hide_in_full` | `false` | With free space living in the bar, hiding it in full mode would lose that information. Opt in. (Since the 2026-09-03 revision in §3 it no longer loses it — the badge takes over — but the default stands: opting in is still a choice to lose the other fields in full mode.) |
| `fields` | `[size, date, free]` | Size and date are the request. Free space moves out of the badge, per §3. |

This changes first-run appearance for existing configs: the floating volume
badge disappears and its information relocates into the bar. That is the
intended behaviour, confirmed during design.

---

## 3. Layout integration

### Where the bar draws

`layoutFilePaneStatusBar` already exists and is already wired as a `layout.Rigid`
at the bottom of every pane, inside the pane chrome
(`ui/filemanager_layout.go:710`). It currently returns empty unless an archive
extraction or a direct paste is running in that pane.

It gains a fallback branch:

1. archive extraction running in this pane → extraction line (unchanged)
2. direct paste targeting this pane → paste line (unchanged)
3. otherwise → the file info line

**Progress lines are not gated by the new configuration.** Setting
`status_bar.enabled: false` must not remove extraction or paste feedback; those
are transient operation status, not the file info bar. Only branch 3 consults
`StatusBarConfig`.

### Visibility

Branch 3 renders when:

```go
cfg.StatusBar.Enabled && !(cfg.StatusBar.HideInFull && pane.table.Mode == table.ModeFull)
```

Evaluated per pane. Panes carry their view mode independently, so a brief pane
keeps its bar while a full pane beside it hides one. That is correct, not a
glitch: the condition is about what the pane's grid already shows.

### Which entry is described

Each pane's bar describes that pane's own cursor entry, whether the pane is
active or not. There is no cross-pane indirection — that was the flaw in the
badge this replaces.

### Pane seam separators

`filePaneStatusBarSeparatorMode` (`ui/filepane_volume_badge.go:400`) already
appends `"| "` or `" |"` at pane edges so adjacent strips read as one continuous
band across the seam. Reused unchanged; the file info line passes through the
same `...WithSeparatorForWidth` treatment as the extraction line.

### Volume badge removal

`filePaneVolumeBadgesHidden` (`ui/filemanager_layout.go:631`) gains a second
condition:

```go
func (ui *UI) filePaneStatusBarShowsFreeSpace() bool {
    return ui.fmCfg.StatusBar.Enabled && slices.Contains(ui.fmCfg.StatusBar.Fields, "free")
}
```

When true, the badge is never drawn. This is global and does not consult
`hide_in_full` or per-pane mode: a pane whose bar is hidden simply shows no free
space, rather than falling back to a second presentation of the same number.

> **Superseded 2026-09-03, after live user review.** The decision above shipped
> as written and was reported as a bug: with `hide_in_full: true` and the pane in
> full mode, free space appeared *nowhere*, while turning the bar off entirely
> gave the badge back. Two ways of having no status bar, two different outcomes.
>
> The original reasoning — one number, one presentation — still holds; what was
> wrong was reading `StatusBar.Enabled` as a stand-in for "the bar is showing".
> The badge reports the **active** pane's volume (`filePaneVolumeBadgeSourcePane`
> mirrors it onto the inactive panes), so it is redundant exactly while the
> *active pane's own bar* carries free space, and is the fallback otherwise.
> `filePaneStatusBarShowsFreeSpace` now gates on `filePaneStatusBarVisible(cfg,
> ui.activePane())` — the same rule the strip renders by, `Enabled` plus the
> `HideInFull && ModeFull` case — instead of on `Enabled` alone:
>
> ```go
> func (ui *UI) filePaneStatusBarShowsFreeSpace() bool {
>     if ui == nil || ui.fmCfg == nil {
>         return false
>     }
>     if !slices.Contains(ui.fmCfg.StatusBar.Fields, fm.StatusBarFieldFree) {
>         return false
>     }
>     return filePaneStatusBarVisible(ui.fmCfg, ui.activePane())
> }
> ```
>
> Exactly one presentation is still on at a time; every way of not showing the
> bar — field unticked, bar off, bar hidden in full mode — now falls back to the
> badge alike. The `full-hidden` headless capture, which used to assert "no strip
> and no badge", asserts the fallback badge instead.

### File organisation

The generic status-strip chrome currently lives in `filepane_volume_badge.go`,
which at 597 lines already covers three unrelated concerns (volume polling
including SFTP `statvfs`/`df` fallback, the floating badge, and the shared status
strip). These move to the new `ui/filepane_status_bar.go`:

- `layoutFilePaneStatusBar`
- `layoutFilePaneStatusBarBox`
- `filePaneStatusBarSeparatorMode` and its constants
- `measureFilePaneStatusBarTextWidth`
- `directFilePasteForPane`

`filepane_volume_badge.go` is left describing the badge and volume lookups only.
This is a move, not a rewrite: no behaviour changes, and it keeps the new code
from landing in a file that is already doing too much.

New files:

| File | Contents |
| --- | --- |
| `ui/filepane_status_bar.go` | Strip chrome (moved) plus the file info branch and layout |
| `ui/filepane_status_fields.go` | Field enum, per-field value builders, line assembly, drop-order degradation |

---

## 4. Owner and group

### Where it is resolved

During listing, not during layout. Calling `os/user` from a layout function
would put a potentially NSS-backed lookup on the frame path, which
CONTRIBUTING.md explicitly calls out as a bug.

`populateListingEntry(row *Entry, name string, info os.FileInfo)`
(`filesys/listing.go:192`) is shared by the local reader, the SFTP reader **and
the archive reader**, and `info.Sys()` carries what is needed in the first two:

- local unix: `*syscall.Stat_t` → `Uid`, `Gid`
- SFTP: `*sftp.FileStat` → `UID`, `GID`
- local Windows: `*syscall.Win32FileAttributeData` → no uid/gid

Because SFTP listings run on every platform, the Windows implementation must
still handle `*sftp.FileStat`; only the `syscall.Stat_t` arm is unix-only.

### Files

Following the established `_other.go` / `_windows.go` pairing used by
`ui/platform/volume_usage_*.go`:

| File | Build tag | Contents |
| --- | --- | --- |
| `filesys/owner_other.go` | `!windows` | `statOwnerIDs(os.FileInfo) (uid, gid uint32, ok bool)` — handles `*syscall.Stat_t` and `*sftp.FileStat` |
| `filesys/owner_windows.go` | `windows` | same signature — handles `*sftp.FileStat` only, returns `ok == false` otherwise |
| `filesys/owner.go` | — | name resolution, cache, formatting |

### Name resolution

Local entries resolve uid/gid to names via `os/user`. SFTP entries stay
numeric — a remote uid has no meaning in the local passwd database, and
resolving it remotely is out of scope.

**The owner is set by the two listing call sites, not inside
`populateListingEntry`**, precisely because that function has three callers that
each want something different: local resolves names, SFTP stays numeric, and
archives get nothing. The archive case is why this cannot be pushed down into
the shared function — doing so would silently give archive members owners.
`statOwnerIDs` returns raw ids and the callers format them differently.

**Known ambiguity, accepted.** `pkg/sftp` populates `FileStat.UID`/`GID` only
when the server sends `SSH_FILEXFER_ATTR_UIDGID`, and discards the flag itself,
so a server reporting no ownership is indistinguishable from a root-owned file:
both surface as `0:0`. The bar renders it regardless. Suppressing `0:0` would
hide genuine root ownership — common and consequential on a server — whereas the
ambiguous case is rare, since mainstream SFTP servers do send the attribute. The
numeric rendering already signals unresolved ids rather than asserting a
confirmed `root`.

`os/user.LookupId` can be slow, so `filesys/owner.go` holds a mutex-guarded
`map[uint32]string` cache for uids and another for gids. Listings run on a
goroutine, hence the mutex. A directory of 10,000 files owned by one user costs
two lookups rather than twenty thousand. Failed lookups cache the numeric
fallback so they are not retried per entry.

Archive entries have no owner and get an empty string.

### Entry field

`filesys.Entry` gains `OwnerText string`, populated beside `PermText`,
`SizeText` and `DateText`. Those are all presentational strings computed at
listing time; this is the same kind of thing in the same place.

It is populated unconditionally, not only when the field is enabled. The cost
after caching is a map read per entry, and making listing depend on UI
configuration would be a worse trade.

---

## 5. Settings UI

### Placement

A fourth tab in the file pane settings section, beside Full mode / Brief mode /
Other, declared in the `modeClicks` slice in `layoutSettingsFilePaneEditor`
(`ui/settings_filepanes.go:379`):

```go
{key: "statusbar", label: "Status bar", click: &st.paneSettingsStatusBarClick},
```

The bar spans both view modes, so it belongs in neither the Full nor the Brief
tab. The Other tab already carries eleven controls; adding nine more would make
it unusable, and a dedicated tab has room for the live preview that the Full and
Brief tabs already provide via `layoutSettingsPanePreviewFrame`.

### Layout

```
 [Full mode] [Brief mode] [Status bar] [Other]

 [x] Show pane status bar
 [ ] Hide it in full mode          ← greyed out while the above is off

 Fields
 [x] Size          [ ] Owner / group
 [x] Date          [ ] Item count
 [ ] Permissions   [ ] Marked selection
 [x] Free space

 Preview
 ┌──────────────────────────────────────────────────────┐
 │ 2.40 MB | 2026-08-30 14:22 | ██████░░░░ 41.20 GB free  │
 └──────────────────────────────────────────────────────┘
```

Checkboxes use `layoutThemeCheckbox` with the `before != after` focus pattern
already used throughout `layoutSettingsPaneOtherTab`.

The preview renders a real status bar through the same line builder as the live
pane, against fixed sample data, so it cannot drift from what ships.

### Wiring

Each control needs the full settings-modal circuit, not just a widget:

- `widget.Bool` fields on `settingsModalState` — two toggles plus seven field
  checkboxes
- `settingsKeyboardFocus*` constants and their entry in the focus traversal
  order (`ui/settings_modal_keyboard.go`)
- dirty comparison in `ui/settings_dirty.go`, so Cancel and the unsaved-changes
  prompt work
- load from `fm.Config` when the modal opens and apply back on save, in
  `ui/settings_modal.go`
- `normalizeSettingsPaneMode` accepting the new `"statusbar"` key

---

## 6. Testing

| Test | Build tag | Covers |
| --- | --- | --- |
| `fm/config_test.go` | — | YAML round-trip; absent `status_bar` yields the documented defaults; absent `enabled` yields `true`; unknown, duplicate and out-of-order field keys normalise correctly |
| `filesys/owner_test.go` | — | uid/gid extraction from both `Sys()` shapes; cache returns a stable value and does not re-lookup; numeric fallback on lookup failure |
| `ui/filepane_status_fields_test.go` | — | Per-field rendering for file, directory, `..`, symlink, marked and unmarked panes; empty fields skipped along with their separator |
| `ui/filepane_status_bar_test.go` | — | Line assembly; free-space shortening then drop order at progressively narrower widths; extraction and paste lines still win over the info line; visibility predicate across the four enabled/hide-in-full combinations and both view modes |
| `ui/filepane_volume_badge_test.go` | — | Badge suppressed while the active pane's bar carries free space; drawn back for every way of not showing that bar — field off, bar off, `hide_in_full` in full mode (per the 2026-09-03 revision in §3); follows the active pane's view mode, not a neighbour's; nil-safe with no panes |
| `ui/filepane_status_bar_headless_verify_test.go` | `uiverify` | Real pixels: brief pane with bar, full pane with bar, full pane with `hide_in_full` (no strip, fallback badge over the inactive pane), narrow pane degradation |

Full matrix before completion:

```bash
go test ./...
go test -tags pdfium ./ui/
go test -tags uiverify ./ui/
make unused
```

`make unused` matters here because the platform shim adds symbols that look dead
on whichever OS the check runs on.

---

## 7. Documentation

- `HELP.md` — a section describing the bar, its fields, and the two settings
- `CHANGELOG.md` — entry under the pre-1.3.0 heading, noting that free space
  moves from the floating badge into the bar by default
- `ARCHITECTURE.md` — add `filepane_status_*.go` to the File manager cluster row

---

## Risks

**The default changes existing installations.** Users upgrading see the volume
badge vanish and a new strip appear. This is intended and confirmed, and the
information is not lost, only relocated. The changelog must call it out.

**Owner adds a platform surface.** Three new files with build tags, and a
symbol set that looks dead on any single OS. Mitigated by `make unused` and by
keeping the platform-specific part down to one function that only extracts two
integers.

**Vertical space.** The bar consumes roughly 20dp per pane. Users who object
can turn it off, which is why the disable checkbox is part of the feature rather
than an afterthought.


---

# Revision 2 — anchored columnar layout (2026-09-01)

The joined-string design above shipped and was rejected on review by the user.
Three complaints, all about how the line behaves rather than what it contains:
fields floated as one left-justified string, so free space sat at a different x
in every directory; every cursor move re-flowed the whole line, so values jumped
sideways; and the filename was omitted entirely. This revision replaces §1's
line-assembly model. Configuration (§2), owner resolution (§4), the async volume
pipeline, and the progress-line branches are unchanged.

## Target rendering

Single entry under the cursor:

```
Makefile  •  1.39 KB  •  2026-08-18 16:40                    │  519.71 GB free (56%)
```

One or more marked entries:

```
3 items selected  •  14.20 MB                                │  519.71 GB free (56%)
```

## Layout model

The info line is no longer one string. It is a two-region row:

- **Left cluster, anchored to the pane's left edge**: fixed-width columns joined
  by `  •  `.
- **Right cluster, anchored to the pane's right edge**: the free-space text,
  preceded by a `│` separator. The `│` renders only when both regions do.

Rendered with real layout (a flex row with a flexed spacer), not with padding
spaces — the pane font is user-configurable and not guaranteed monospace.

## Fixed columns — no jumping

Column widths are a property of the *directory and configuration*, never of the
selected entry, so moving the cursor cannot shift anything:

| Column | Width source |
| --- | --- |
| Name | widest display name in the listing, capped by the space remaining after every other column and the free-space region |
| Size | a measured sample (`888.88 MB`) |
| Date | the configured `DateFormats[0]` rendered on a wide sample timestamp |
| Permissions | a measured sample per format (`-rw-r--r--` / `8888`) |
| Owner / group | widest owner text in the listing, capped |
| Item count | natural width (constant within a directory) |

A name longer than its column is compacted by the existing
`filePaneModel.compactName` — the same marker + extension-preserving-tail trim
the pane grid uses (`gpstrack-dashb….go`), honouring `name_compact` config.

Values render left-aligned within their columns.

## Marked-selection mode

When the pane has marked rows, the left cluster is replaced wholesale by
`N items selected  •  <combined size>` (`1 item selected` for one). This
supersedes Revision 1's optional `selection` field: it is no longer a checkbox,
it is what the bar does. The free-space region is unaffected.

## Free space

`519.71 GB free (56%)` — the percentage is **free/total**, rounded to the
nearest integer, qualifying the word it follows. The ASCII usage bar and the
three-form ladder are retired; the field renders in this single form or is
dropped.

## Degradation on narrow panes

1. The name column absorbs shrinkage first, via `compactName`.
2. Below a usable name width, columns drop:
   `owner → items → perms → free → date → size`, and the name survives last —
   it is the anchor.

## Retired from Revision 1

- The `" | "` joined-string assembly and the pane-seam `|`/`| ` separators for
  the info line. Progress lines (extraction, paste) keep their existing format
  and seam behaviour untouched.
- The `selection` config key. `NormalizeStatusBarFields` no longer recognises
  it; existing configs carrying it lose it silently on load, which is the
  normalizer's documented behaviour for unknown keys.
- The `filePaneStatusFreeForm` ladder and `textCellProgressBar` usage in the
  bar.

## Settings changes

- Field checkboxes: Size, Date, Permissions, Owner / group, Item count,
  Free space — six now. **Name is always shown** and has no checkbox; it is the
  left anchor and the feature is pointless without it.
- The **Marked selection** checkbox is removed; the mode is automatic.
- The preview renders the anchored two-region layout at the same representative
  width as before, including the marked-mode line.

## Decisions taken without a user ruling (flag if wrong)

- `(56%)` reads as *percent free*, since it qualifies "free".
- Sizes keep `%.2f` everywhere (`14.2 MB` in the user's example is normalised
  to `14.20 MB`).
- One marked row already switches modes, worded `1 item selected`.


## Revision 2.1 amendments (2026-09-01, user review of R2)

**Item count is retired.** Not a per-file attribute worth a checkbox; the marked
mode already reports counts. The `items` config key joins `selection` in the
retired set — `NormalizeStatusBarFields` drops it from existing configs as an
unknown key. Field set becomes five: size, date, perms, owner, free. The items
column leaves the width computation and the drop order.

**The bar's date gets its own layout choice**, independent of the Full-mode
column date builder. New config key:

```yaml
status_bar:
  date_format: auto   # auto | iso | us | short
```

| Key | Layout | Renders |
| --- | --- | --- |
| `auto` (default) | follow `DateFormats[0]` via `filePaneModel.formatDate` — the pre-existing behaviour, so nobody's bar changes on upgrade | whatever the column date builder is set to |
| `iso` | `2006-01-02 15:04` | `2026-08-18 16:40` — the LT/current form from the user's example |
| `us` | `01/02/2006 3:04 PM` | `08/18/2026 4:40 PM` |
| `short` | `01-02 15:04` | `08-18 16:40` |

Unknown values normalise to `auto`, matching `permission_format`'s treatment.
The settings tab gains a picker (the `layoutSettingsShellPicker` style already
used for the permission format and the date builder), labelled with rendered
samples rather than key names. The date column's fixed width derives from the
chosen layout's sample rendering.

Zero `ModTime` still renders empty regardless of layout.
