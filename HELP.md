# Hexone Help

Hexone is a keyboard-first file manager with an internal viewer, SSH support, favorites, and a protocol analyzer.
Press `F1` to open or close this help window.

## Navigation

- `Tab` switches the active file pane.
- `Up` and `Down` move the selection.
- `PageUp` and `PageDown` jump by a larger visible block.
- `Home` goes to the first item. `End` goes to the last item.
- In `full` mode, `Left` behaves like `Home` and `Right` behaves like `End`.
- In `brief` mode, `Left` and `Right` move across the visible columns.
- `Enter` opens the selected file or directory.
- `Esc` closes the current popup, leaves an editor, or returns to the file manager.
- `Insert` toggles the current row selection and moves to the next row.
- `Ctrl+A` or `Cmd+A` toggles `Select All`.
- `Ctrl+E` or `Cmd+E` toggles selecting rows that match the current item.
- when a file is selected, `Ctrl+E` matches the same extension, including files with no extension
- when a directory is selected, `Ctrl+E` matches all directories

Each pane has a mode badge in its header:

- `full` `mode:full` shows a detailed row with columns
- `brief` `mode:brief` shows a denser multi-column layout

Click the mode badge to switch between `full` and `brief`.

## Current Dir Line

The current-dir line at the top of each pane is interactive.

- Click a path segment to jump to that level.
- Double-click the current-dir line to edit the path directly.
- Press `Enter` in the path editor to go there.
- Press `Esc` to cancel path editing.

On Windows:

- the drive part of the current-dir line can open the drive picker
- `Alt+1` opens the drive picker for the left pane
- `Alt+2` opens the drive picker for the right pane
- in the drive picker, use `Up`/`Down` to move, `Enter` to select, and `Esc` to close

## Sorting

Each pane also has a sort badge in the header.

- Left click the sort badge to open the sort menu and choose what to sort by.
- Right click the sort badge to flip ascending or descending order directly.
- Current sort choices are `Name`, `Date`, `Ext`, and `Size`.

Use this when a folder makes more sense grouped by modification time, extension, or size instead of name.

## Function Key Bar

- `F1` opens or closes Help.
- `F2` opens custom commands. The first menu item opens the custom command editor; saved commands appear below it.
- `F3` opens the Internal Viewer.
- `F4` opens the selected file with the system association.
- `F5` copies.
- `F6` moves or renames.
- `F7` creates a file or folder.
- `F8` deletes.
- `F9` opens the Tools menu (`Hex to ASCII`, `Protocol Analyzer`, `Settings`).
- `F10` exits the app.
- `F11` hides or shows the function key bar.

If the function key bar is auto-hidden in the viewer, `F11` can still bring it back.

## Custom Commands

`F2` is for saved shell snippets that are not tied to a single file. The editor stores up to 10 commands, each with a short name, optional shortcut, and multi-line command body.

- `Run` saves the current command and shows its output in a command-only viewer.
- `Ctrl+Enter` or `Cmd+Enter` runs from the editor.
- `Ctrl+S` or `Cmd+S` saves from the editor without closing it.
- `Esc` closes the editor, F2 menu, or command output viewer.
- `Ctrl+1` through `Ctrl+0` run the saved commands while the F2 menu is open. Plain number keys also work there.

Commands run locally for local panes and over SSH for connected remote panes. `{path}`, `{fullpath}`, and `{filename}` are expanded like viewer commands when used.

## Favorites And SSH

Use the `☆` button in a pane header to work with favorites.

- save the current local folder as a favorite
- save a remote location as a favorite
- jump back to commonly used places without browsing there again

To manage SSH sessions:

- press `Ctrl+F` or `Cmd+F` in the file manager to open `SSH Sessions`
- add a host, port, user, and authentication method
- save the session, then connect the active pane

Inside the internal viewer, the same shortcut opens Find instead of `SSH Sessions`.

Once connected, a remote pane supports normal browsing plus viewer-based inspection, command-driven log viewing, and remote-assisted hex searching.

## Internal Viewer

The internal viewer has three explicit modes, plus automatic image-style preview inside `file` mode for supported images and PDFs:

- `file` for normal text content plus image/PDF preview when supported
- `hex` for raw bytes
- `command` for shell output based on the selected file

New viewer opens default to `file`; exact target commands and filename command rules open in `command`, while files over the configured read limit open in `hex` when no target or rule command applies.

Useful viewer keys:

- `F3` refreshes the current file or reruns the current command
- `Tab` moves `file -> hex -> command`; `Shift+Tab` moves backward
- `Ctrl+F` or `Cmd+F` opens Find in `file`, `hex`, and `command`
- `Enter` jumps to the next find result; `Shift+Enter` jumps to the previous one
- `PageUp`, `PageDown`, `Home`, and `End` scroll
- `Ctrl+C` or `Cmd+C` copies the current selection
- `Ctrl+A` or `Cmd+A` selects all loaded data in text, `hex`, and command output
- `Esc` closes Find, closes the command editor, or closes the viewer
- smooth scrolling is enabled by default and can be turned off in `Settings -> Viewer`

### File And Hex

- `file` mode is best for quick reading and exposes an encoding picker for `auto`, `utf-8`, `utf-16le`, `utf-16be`, and `cp437`
- supported images open as an image preview inside `file` mode
- supported PDFs open as a rendered page preview inside `file` mode
- `hex` mode is better for binary files, mixed data, or damaged content
- on SSH panes, `hex` Find can use the configured remote search utility command for large files

### Command Mode

`command` mode runs a shell command against the selected file and captures its output.
It is not a terminal: there is no PTY, no interactive stdin, and no full-screen pager UI.
Recent commands are kept in viewer command history for quick reuse.
These placeholders are available:

- `{path}` or `{fullpath}` for the full selected path
- `{filename}` for just the file name

Useful snapshot commands:

```sh
cat {path}
sed -n '1,200p' {path}
strings -n 8 {path}
jq . {path}
```

These finish and leave static output behind. Use them when you want a one-time read.

### Streaming vs Non-Streaming

Some commands keep producing output and behave like a live stream. Common examples:

- `tail -f`
- `tail --follow`
- `tailf`
- `journalctl -f`

Useful live examples:

```sh
tail -f {path}
tail -n 200 -f {path}
tail -f {path} | grep --line-buffered ERROR
tail -f {path} | grep --line-buffered 'login|disconnect'
journalctl -f
```

Practical rule:

- use `cat`, `sed`, `jq`, or `strings` for a snapshot
- use `tail -f` or `journalctl -f` for a live stream
- if you pipe a live stream through `grep`, prefer `grep --line-buffered`

Avoid commands that expect a real terminal, such as:

- `less`
- `more`
- `watch`
- `top`
- `htop`
- editors like `vim`

Why `--line-buffered` matters:
without it, `grep` may buffer output and make a live command look stuck.

Remote panes support `command` mode too, so the same log-following patterns work over SSH.

### Image Preview

- arrow keys pan the image
- `PageUp` and `PageDown` move by a larger vertical chunk
- `Home` goes to the origin and `End` goes to the far edge
- `Ctrl++` / `Ctrl+-` or `Cmd++` / `Cmd+-` zoom in and out

### PDF Preview

- rendered PDF pages use the same pan and zoom controls as image preview
- `Up` / `Down` and `PageUp` / `PageDown` scroll inside the current page first, then move to the previous or next page at the edge
- the vertical scrollbar represents the whole PDF and can be dragged to jump between pages quickly
- mouse-wheel scrolling crosses page boundaries at the top and bottom, just like the keyboard controls
- `[` moves to the previous page and `]` moves to the next page

## Customization

Most everyday customization is available from Settings.
The full configuration also lives in `hexone.yaml`.

On Linux, the writable config files live under `~/.config/hexone/`.
On macOS, they live under `~/Library/Application Support/hexone/`.
On Windows, they currently live in the current working directory as `hexone.yaml` and related files.

Useful things to adjust:

- fallback command
- exact target overrides
- filename regex rules
- shell selection
- remote search utility command for SSH hex find
- viewer smooth scrolling
- file encoding defaults
- auto-refresh interval for non-streaming command mode
- viewer font size
- function bar auto-hide while the viewer is open
- system associations

Example:

```yaml
viewer:
  shell: auto
  command: cat {path}
  smooth_scrolling: true
  command_by_target:
    local:/Users/me/logs/app.log: tail -n 200 -f {path}
  command_rules:
    - pattern: (?i)\.log(?:\.\d+)?$
      command: tail -n 200 -f {path}
    - pattern: ^docker-compose.*\.ya?ml$
      command: docker compose -f {path} config
  remote_search_command: tail -c +{range_start_1based} {path} | head -c {range_len} | LC_ALL=C grep -aobF {match_limit} -- {pattern} | {result_select}
  command_auto_refresh: true
  command_refresh_ms: 1500
  hide_function_bar_when_open: true

custom_commands:
  - name: Process summary
    shortcut: Ctrl+1
    command: |
      pid=$(pidof gpstrack | awk '{print $1}')
      python - "$pid" <<'PY'
      print("PID", __import__("sys").argv[1])
      PY
```

Notes:

- `shell: auto` picks a sensible shell automatically
- Priority 1: `command_by_target` matches one exact full path and overrides any regex-selected command
- Priority 2: `command_rules` match the filename only; later matches override earlier ones
- Priority 3: `command` is the generic fallback
- `command_rules` switch the viewer into command mode automatically when a filename matches
- `command_by_target` overrides the chosen command and opens that target in command mode by default
- `remote_search_command` is used by SSH hex Find; set it to `off` to disable the remote utility path
- `command_auto_refresh` matters most for non-streaming command mode
- Settings -> Viewer exposes the same priority order directly in the UI, along with smooth scrolling and viewer auto-hide

## Protocol Analyzer

The Protocol Analyzer decodes pasted hex using `protocols.yaml`.

On Linux, Hexone first checks `~/.config/hexone/protocols.yaml`. If that file is missing, it uses the embedded default and writes a reference sample to `~/.config/hexone/protocols.sample.yaml`.
On macOS, it first checks `~/Library/Application Support/hexone/protocols.yaml` and writes the reference sample to `~/Library/Application Support/hexone/protocols.sample.yaml`.
On Windows, it currently uses `protocols.yaml` in the current working directory and writes `protocols.sample.yaml` there too.

Input tips:

- spaces are fine
- newlines are fine
- `0x` prefixes are ignored
- an odd number of hex digits is an error

### Basic Syntax

A small useful protocol usually needs:

- a protocol `name`
- an `endian`
- a `layout`
- one or more `field` entries

Common field types:

- `u8`, `i8`
- `u16`, `i16`
- `u24`
- `u32`, `i32`
- `u64`, `i64`
- `bytes`

For `bytes`, provide either `len` or `len_expr`.

Useful layout nodes:

- `field` to read and label a value
- `assert` to validate something
- `switch` to branch by a parsed value
- `choose` for first-match conditions
- `repeat` for repeated records
- `set` to store a computed value
- `peek` to inspect bytes without consuming them
- `hook` for built-in or custom decode logic

Useful built-in expressions:

- `remaining`
- `offset`
- `size`

### Minimal Example

```yaml
version: 1

protocols:
  - name: demo
    desc: "Simple frame with header, length, and payload"
    endian: be
    layout:
      - field:
          name: magic
          type: u8
          const: 0x7E
          value_fmt: hex

      - field:
          name: length
          type: u8

      - field:
          name: payload
          type: bytes
          len_expr: "length"

      - assert:
          expr: "remaining == 0"
          message: "Extra bytes at end of frame"
```

A good first protocol is:

- read a fixed header
- read a length
- read a payload with `len_expr`
- assert that no unexpected bytes remain
