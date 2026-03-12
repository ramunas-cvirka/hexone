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

Each pane has a mode badge in its header:

- `full` shows a detailed row with columns
- `brief` shows a denser multi-column layout

Click the mode badge to switch between `full` and `brief`.

## Current Dir Line

The current-dir line at the top of each pane is interactive.

- Click a path segment to jump to that level.
- Double-click the current-dir line to edit the path directly.
- Press `Enter` in the path editor to go there.
- Press `Esc` to cancel path editing.

On Windows:

- the drive part of the current-dir line can open the drive picker
- `Alt+F1` opens the drive picker for the left pane
- `Alt+F2` opens the drive picker for the right pane

## Sorting

Each pane also has a sort badge in the header.

- Left click the sort badge to flip ascending or descending order.
- Right click the sort badge to choose what to sort by.
- Current sort choices are `Name`, `Date`, `Ext`, and `Size`.

Use this when a folder makes more sense grouped by modification time, extension, or size instead of name.

## Function Key Bar

- `F1` opens Help.
- `F3` opens the Internal Viewer.
- `F4` opens the selected file with the system association.
- `F5` copies.
- `F6` moves or renames.
- `F7` creates a file or folder.
- `F8` deletes.
- `F9` opens the Tools menu.
- `F10` exits the app.
- `F11` hides or shows the function key bar.

If the function key bar is auto-hidden in the viewer, `F11` can still bring it back.

## Favorites And SSH

Use the `*` button in a pane header to work with favorites.

- save the current local folder as a favorite
- save a remote location as a favorite
- jump back to commonly used places without browsing there again

To manage SSH sessions:

- press `Ctrl+F` or `Cmd+F` to open `SSH Sessions`
- add a host, port, user, and authentication method
- save the session, then connect the active pane

Once connected, a remote pane supports normal browsing plus viewer-based inspection and command-driven log viewing.

## Internal Viewer

The internal viewer has three modes:

- `file` for normal text content
- `hex` for raw bytes
- `command` for shell output based on the selected file

Useful viewer keys:

- `F3` refreshes or reopens the current item
- `Esc` closes the command editor or the viewer
- `Ctrl+C` or `Cmd+C` copies the current selection from text or command output
- `Ctrl+A` or `Cmd+A` selects all visible data in `hex` mode

### File And Hex

- `file` mode is best for quick reading
- `hex` mode is better for binary files, mixed data, or damaged content

### Command Mode

`command` mode runs a shell command against the selected file.
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
- `watch`

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

Why `--line-buffered` matters:
without it, `grep` may buffer output and make a live command look stuck.

Remote panes support `command` mode too, so the same log-following patterns work over SSH.

## Customization

Most everyday customization is available from Settings.
The full configuration also lives in `hexone.yaml`.

Useful things to adjust:

- default viewer mode
- command template
- shell selection
- auto-refresh interval for non-streaming command mode
- viewer font size
- word wrap
- function bar auto-hide while the viewer is open

Example:

```yaml
viewer:
  mode: command
  shell: auto
  command: tail -n 200 -f {path} | grep --line-buffered ERROR
  command_rules:
    - pattern: (?i)\.log(?:\.\d+)?$
      command: tail -n 200 -f {path}
    - pattern: ^docker-compose.*\.ya?ml$
      command: docker compose -f {path} config
  command_auto_refresh: true
  command_refresh_ms: 1500
  word_wrap: false
```

Notes:

- `shell: auto` picks a sensible shell automatically
- `command_by_target` has the highest priority, then `command_rules`, then the generic `command`
- `command_rules` match the filename and open the viewer in command mode automatically
- later `command_rules` override earlier ones when more than one regex matches
- `command_auto_refresh` matters most for non-streaming command mode
- `word_wrap` affects how text and command output are wrapped on screen

## Protocol Analyzer

The Protocol Analyzer decodes pasted hex using `protocols.yaml`.

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
