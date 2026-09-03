# Architecture

A map of the hexone source tree: what lives where, how a frame is produced, and
where to start when you want to change something.

For how to write code that fits, see [CONTRIBUTING.md](CONTRIBUTING.md). For
user-facing behaviour see [HELP.md](HELP.md). For release history see
[CHANGELOG.md](CHANGELOG.md).

## Orientation

hexone is a desktop file manager built on [Gio](https://gioui.org) — an
**immediate-mode** GUI toolkit. There is no retained widget tree: every frame
re-runs the layout code from the root, and widgets keep their state in structs
you own. That single fact explains most of the shape of this codebase, and it is
worth internalising before changing anything in `ui/`.

The app is one binary, `cmd/hexone`, on top of one very large package, `ui/`,
plus a set of small focused packages for everything that is not drawing.

```
cmd/hexone      main(), window creation, font loading, native input hooks
  └── ui/       ~90k lines: every screen, every widget, every interaction
        ├── fm/          config + session model (YAML on disk)
        ├── filesys/     listing, copy, delete, trash, archives, SFTP listing
        ├── httpclient/  HTTP request model, execution, collection persistence
        ├── protocols/   GPS/telematics protocol decoding (Protocol Analyzer)
        ├── notify/      completion sounds
        ├── appdata/     per-OS config/session paths
        ├── windowstate/ window geometry save/restore
        ├── appicon/     icon rendering and per-OS installation
        ├── secretstore/ OS keyring access
        └── buildinfo/   version strings
```

## Build and test

```bash
go build ./cmd/hexone
```

The test matrix has three configurations. All three must pass; CI runs the
first, and the build tags exist so the heavy ones stay opt-in.

| Command | What it covers |
| --- | --- |
| `go test ./...` | Everything. The PDF backend is a no-op stub here. |
| `go test -tags pdfium ./ui/` | Real pdfium (WASM via `klippa-app/go-pdfium`). |
| `go test -tags uiverify ./ui/` | Renders the real UI headlessly and writes PNG frames. |

`uiverify` is the one worth knowing about: it drives the actual widget tree
through `gioui.org/gpu/headless` and reads back real pixels, so it catches
layout and paint regressions that unit tests cannot. Set `UI_VERIFY_OUT` to a
directory to keep the frames:

```bash
UI_VERIFY_OUT=/tmp/frames go test -tags uiverify ./ui/ -run TestHeadless
```

`ui/function_bar_headless_verify_test.go` is the smallest example to copy;
`ui/tabcycle_headless_verify_test.go` shows driving real key and pointer events
through the router. On macOS this is the only way to screenshot the UI — the
sandbox has no Screen Recording permission, so `screencapture` of a live window
fails.

Cross-compiling to Linux **from macOS fails** and always has: Gio's Linux
backend needs cgo (X11/Wayland), and cross-compilation disables it. CI builds
Linux on Linux runners. Windows and darwin cross-builds work fine.

`cmd/hexone-pdfium-worker` needs a native `pdfium` library discoverable via
pkg-config; without it that one target does not build. Nothing else depends on
it.

## The frame loop

`cmd/hexone/main.go` creates the window and calls `ui.Layout(th, gtx)` once per
frame. That function ([ui/layout.go](ui/layout.go)) is the root of everything:

1. **Pump async state.** Results from background goroutines (directory listings,
   file loads, syntax highlighting, saves, SSH probes) arrive on channels and
   are drained at the top of the frame — never applied directly by the worker.
2. **Handle keys.** A series of `ui.handle*Keys(gtx)` calls, ordered so that
   modal and popup handlers get first refusal on an event.
3. **Lay out.** A `layout.Stack` with the function bar and the active tab at the
   bottom, then popups, editors, and modals stacked above.

Anything that must survive between frames lives on `UI` or on a `*State` struct
hanging off it. Anything computed from those is recomputed every frame.

### The async pattern

Long operations never block the frame. The shape is consistent across the
codebase, and new work should follow it:

- a `start*` function spawns a goroutine and stores a cancel func plus a result
  channel on the relevant state struct
- the goroutine sends a result value carrying a **sequence number**
- a `pump*` function drains the channel at frame start and **discards results
  whose sequence no longer matches**, which is how stale work is ignored

`fileViewerState` is the fullest example: separate sequence-guarded pipelines
for content loading, syntax highlighting, find, and saving.

`filePaneVolumeBadgeState` (`startVolumeLookup` / `pumpFilePaneVolumeLookups`) is
the smallest one worth reading, and it shows the wrinkle the pattern hides: a
goroutine finishing cannot wake Gio by itself, so a pump with work outstanding
has to schedule its own invalidation and come back to look. It also stops polling
a pane no frame has asked about, and caps the goroutines it will leave behind on
a wedged mount — an abandoned lookup is not a cancelled one.

## Package map

### `ui/` — everything visual

81 files. The prefix of a filename is its feature cluster; that is the fastest
way to navigate. Paths in this table are relative to `ui/`.

| Cluster | Files | What it owns |
| --- | --- | --- |
| **Root layout** | `layout.go`, `function_bar.go`, `segmented_tabs.go`, `session.go` | `UI` struct, frame root, tab switching, F1–F12 bar |
| **File manager** | `filemanager*.go`, `filepane_*.go`, `pane_tabs.go`, `filekeys.go` | Panes, listing, selection, sorting, tabs, context menus |
| **File operations** | `filecopy*.go`, `filemove.go`, `filedelete.go`, `filecreate.go`, `fileperm.go`, `fileop_*.go`, `multi_rename.go`, `archive_extract.go` | Copy/move/delete dialogs, progress, conflict resolution |
| **Viewer** | `fileviewer*.go` (20 files) | F3/F4 viewer: text, hex, markdown, images, PDF, editing |
| **Terminal** | `terminal*.go` | PTY drawer, tabs, snippets, find |
| **Settings** | `settings_*.go` | The settings modal (`settings_modal.go` is the largest file in the repo) |
| **SSH** | `ssh_*.go` | Connection modal, OpenSSH config parsing, remote sessions |
| **HTTP client** | `http_client.go`, `http_credentials.go` | Collections, requests, environments |
| **Protocol analyzer** | `protocol_analyzer.go` | Decoding UI over the `protocols` package |
| **Shared widgets** | `dialog_*.go`, `checkbox_style.go`, `label_layout.go`, `find_surface.go` | Reusable styling and dialog chrome |

Sub-packages:

- **`ui/platform/`** — the per-OS shims (clipboard, drive enumeration, file
  clipboard, file associations, volume usage). Every file here has a
  `_windows` / `_darwin` / `_linux` / `_other` sibling. **Check all of them
  before deleting anything that looks unused.**
- **`ui/theme/`** — icons and text metrics.
- **`ui/widget/table/`** — the table widget the file panes are built on.

### Support packages

| Package | Responsibility |
| --- | --- |
| `fm` | Config and session model. `hexone.yaml` parsing, colour handling, filename-colouring rules, defaults, normalisation. |
| `filesys` | Filesystem work with no UI: listings (local, SFTP, archive), copy, delete, trash. |
| `httpclient` | Request model, execution, variable expansion, YAML collections, secret vault. |
| `protocols` | Telematics protocol specs and packet decoding. Spec-driven from `protocols.yaml`. |
| `appdata` | Per-OS config/session paths. Handles the MSIX `LocalState` case on Windows. |
| `windowstate` | Window geometry save/restore and startup sizing. |
| `notify` | Operation-complete sounds. |
| `appicon` | Icon rendering plus per-OS installation (ICNS, ICO, X11). |
| `secretstore` | OS keyring wrapper. |
| `buildinfo` | Version strings baked in at build time. |

## Where do I change…?

| Goal | Start here |
| --- | --- |
| A function-key action | `ui/function_bar.go` — `activateFunctionBarTool`; global key routing is `handleGlobalFunctionKeys` in `ui/layout.go` |
| File pane behaviour | `ui/filemanager.go` (state) and `ui/filemanager_layout.go` (drawing) |
| Viewer text rendering | `ui/fileviewer_stream.go` — `streamOutputView` |
| Viewer editing | `ui/fileviewer_virtual_edit.go` |
| Syntax highlighting | `ui/fileviewer_syntax.go` (chroma) |
| A config option | `fm/config.go` — add the field, a default, and normalisation; then wire the settings UI in `ui/settings_modal.go` |
| Copy/move mechanics | `filesys/copy.go`; the dialog lives in `ui/filecopy.go` |
| Terminal behaviour | `ui/terminal.go` |
| A new protocol | `protocols.yaml` first; `protocols/core.go` only if the spec format needs extending |

## Conventions and gotchas

**Tab keys are offset from their layout functions.** This trips people up:

| Key | Feature | Rendered by |
| --- | --- | --- |
| `tab0` | Files | `layoutTab1()` in `filemanager_layout.go` |
| `tab1` | Hex/ASCII | `layoutTab0()` in `hex_ascii.go` |
| `tab2` | Protocol Analyzer | `layoutTab2()` in `protocol_analyzer.go` |
| `tab3` | HTTP client | `layoutTab3()` in `http_client.go` |

**The viewer holds text in two representations.** The read-only view shows
*sanitized* text (tabs expanded to tab stops, control runes replaced); the
editor holds the file's *raw bytes* so saving cannot corrupt them. They are not
interchangeable: a syntax document built for one addresses the wrong offsets in
the other, and the painter draws only span-covered ranges, so stale spans
silently drop the tail of every line. `viewerTabColumns` is the shared tab stop
width that keeps both representations on the same columns. If you touch
`sanitizeViewerContent` or `streamOutputView`, keep them in step.

**Columns are not rune indices when tabs are present.** `streamOutputView` is a
fixed-cell grid. Use `colAtByte` / `byteAtCol` / `displayText` rather than
`runeIndexAtByte` / `byteIndexAtRune` for anything positional in the editor.

**Platform files hide real callers.** A symbol that looks unused on your OS may
be the only caller on another. Always check across `GOOS` before deleting.

**Build tags hide real callers too.** `pdfium` swaps out the PDF backend and
`uiverify` adds test drivers; a symbol can be live in one configuration and dead
in the others.

## Tooling

Dead code accumulates quickly in a codebase this size:

```bash
make unused
```

`tools/unusedcheck` runs `staticcheck -checks U1000` under every OS and build
tag combination and reports only what all of them agree is unreachable. Do not
just run staticcheck once — a symbol reported dead on your machine is regularly
the only caller on another platform, and deleting it breaks a build you never
ran. The tool skips configurations it cannot type-check (Gio's Linux backend
needs cgo, so `GOOS=linux` is unanalyzable from macOS and `GOOS=darwin` from
Linux) rather than counting them as empty.

Symbols kept alive only by a configuration the local machine cannot analyze go
in `tools/unusedcheck/allowlist.txt` with the reason. CI runs the check on
Linux, so it covers what a macOS developer cannot.
