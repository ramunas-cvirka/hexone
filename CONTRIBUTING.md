# Contributing

Coding guidelines for hexone. Read [ARCHITECTURE.md](ARCHITECTURE.md) first for
the source map; this document is about *how* to write code that fits.

The theme throughout: this is a large immediate-mode GUI codebase that already
has shared primitives, established patterns, and a real test suite. Most of the
value here comes from **finding and reusing what exists** rather than adding new
abstraction.

## Before you write a widget, look for the helper

The single most common mistake in this codebase is re-implementing a visual
primitive that already exists, because the helpers live in feature files rather
than an obvious home. Check these first:

| Helper | Lives in | Does |
| --- | --- | --- |
| `fillRoundedClipBox(gtx, radius, bg, border, w)` | `filepane_popup.go` | Rounded panel with background and border — every popup and menu card |
| `fixedWidth(gtx, w, wid)` / `fixedHeight(gtx, h, w)` | `protocol_analyzer.go` / `layout.go` | Pin one axis of a widget |
| `measureLabelUnconstrained(gtx, lbl)` | `protocol_analyzer.go` | Measure text without laying it out — for sizing to content |
| `scaleColorAlpha(c, alpha)` | `filepane_popup.go` | Fade a colour for animated popups |
| `mixNRGBA(a, b, t)` | see `grep` | Blend two colours |
| `clamp01(v)` | see `grep` | Clamp a float to 0..1 |

Yes, a generic helper like `fixedWidth` living in `protocol_analyzer.go` is
wrong. Until that is fixed, `grep -rn "^func fixedWidth" ui/` is faster than
writing a new one. **If you catch yourself writing a rounded box with a border,
stop and use `fillRoundedClipBox`.**

Compare `layoutFileViewerContextMenuCard` and `layoutTerminalContextMenuCard`:
two features, same visual, both thin compositions over the same primitives.
That is the pattern to follow — a per-feature function that *composes*, not one
that re-draws.

## Colours come from a theme accessor, never a literal

Each visual area has a theme struct and an accessor on `UI`:
`ui.fileViewerTheme()`, `ui.filePanePopupTheme()`, `ui.hexASCIITabTheme()`.
They resolve user configuration from `fm.Config`. Hard-coding a colour bypasses
theming and the user's settings, so don't.

Adding a colour means: field on the theme struct → default → resolution from
config → settings UI if it should be user-facing.

## Immediate mode: what goes where

Gio re-runs layout from the root every frame. So:

- **State that must survive a frame** goes on `UI` or on a feature `*State`
  struct. Widget values (`widget.Clickable`, `widget.Editor`) are state.
- **Everything derived** is recomputed in the layout function. Do not cache it
  on the struct "for performance" without measuring — a stale cache in
  immediate-mode code is a class of bug that is genuinely hard to find.
- **Never mutate shared state from a goroutine.** Send it over a channel and
  apply it during the frame (see below).

Prefer a new feature `*State` struct over new fields on `UI`. `UI` is already
111 fields and 1013 methods; every field added there makes the eventual split
into sub-packages harder.

## Background work: start, sequence, pump

Long operations never block the frame. Follow the existing shape:

```go
// 1. start: spawn, remember how to cancel, bump the sequence
st.seq++
seq := st.seq
go func() {
    res := doWork()
    ch <- result{seq: seq, doc: res}
}()

// 2. pump: drain at frame start, discard anything stale
case res := <-st.ch:
    if res.seq == st.seq {
        st.apply(res)
    }
```

The sequence number is what makes cancellation correct: a superseded result is
dropped instead of overwriting newer state. `fileViewerState` runs four of these
pipelines at once (content, syntax, find, save) and is the reference example.

If you find yourself calling a slow function directly in a layout path, that is
the bug — the frame will stutter.

## Interfaces: at real seams only

Go interfaces belong at the point of consumption, when there are genuinely
multiple implementations or you need a test seam. Adding them speculatively
("so it's swappable later") costs indirection now for a flexibility that usually
never gets used, and in layout code it makes call paths much harder to follow.

- **Good candidate:** the transfer/listing layer, where local, SFTP and archive
  sources really are three implementations of one idea.
- **Bad candidate:** layout functions. Keep them concrete.
- **Already solved differently:** platform differences use build tags and
  `_windows.go` / `_darwin.go` / `_linux.go` / `_other.go` siblings, not
  interfaces. That is deliberate — it is compile-time selection with no runtime
  dispatch, and it keeps per-OS code out of the shared path. Follow it.

The same applies to helper extraction: extract on the *second* real duplication,
not in anticipation of one.

## Platform-specific code

Add a file per platform with matching function signatures, not `runtime.GOOS`
branches inside shared code. Every platform needs an implementation, including
`_other.go` for the fallback.

Remember that a symbol used only on one platform looks dead everywhere else —
that is why `make unused` exists (below) and why deleting "unused" code without
checking every `GOOS` is dangerous.

## File organisation

Files are named `<feature>_<aspect>.go`. When one grows past roughly 2000 lines,
split it along an aspect boundary rather than letting it grow — `settings_modal`
is already split into six files (`_filenames`, `_filenames_rules`, `_keyboard`,
`settings_dirty`, `settings_filepanes`), which is the pattern to copy.

Keep state, event handling and layout for a feature in the same cluster so the
prefix stays a reliable index.

## Testing

Every change needs the suite green:

```bash
go test ./...
go test -tags pdfium ./ui/
go test -tags uiverify ./ui/
```

- **Logic** — plain unit tests. Most of `ui/` is testable without a window;
  construct the state struct directly and call the function.
- **Layout and paint** — a `uiverify` headless test that renders real frames.
  Reach for this when a change affects what the user actually sees; it catches
  what unit tests structurally cannot. Copy
  `ui/function_bar_headless_verify_test.go` for the basic shape, or
  `ui/tabcycle_headless_verify_test.go` to drive key and pointer events.

When fixing a bug, write the failing test first. It is the only way to know the
fix addresses the cause rather than the symptom.

## Dead code

```bash
make unused
```

Run it before opening a PR; CI runs it too. If it reports something you believe
is live, it is almost certainly reachable only from a platform or build tag the
local run could not analyze — verify that, then add it to
`tools/unusedcheck/allowlist.txt` **with the reason**, rather than deleting.

## Commits

Keep unrelated changes in separate commits — a dependency bump, a refactor and a
bug fix should not arrive together. `make headers` updates the licence headers;
run it before committing new files.
