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
- `Insert` toggles the current row selection and moves to the next row; hold it to select successive rows.
- `Ctrl+A` or `Cmd+A` toggles `Select All`.
- `Ctrl+E` or `Cmd+E` toggles selecting rows that match the current item.
- when a file is selected, `Ctrl+E` matches the same extension, including files with no extension
- when a directory is selected, `Ctrl+E` matches all directories

Each pane has a mode badge in its header:

- `full` `mode:full` shows a detailed row with columns
- `brief` `mode:brief` shows a denser multi-column layout

Click the mode badge to switch between `full` and `brief`.

## Pane Tabs

Each file pane has its own independent compact tab strip above the current-dir line. Switching or creating a tab affects only the active pane.

- `+` opens a new tab in the pane's current directory.
- `x` closes a tab; the last tab in a pane stays open.
- `<` and `>` appear when the tab row overflows.
- `Ctrl+N` opens a new tab in the active pane.
- `Ctrl+X` closes the active tab; the last tab stays open.
- `Ctrl+Tab` selects the next tab; `Ctrl+Shift+Tab` selects the previous tab.
- You can also hold `Ctrl+Tab` and press `Left` or `Right` before releasing `Tab` to choose the direction explicitly.

Tab titles use the current directory and trim to fit. The active tab opens through the notched frame into its current-dir line; inactive tabs remain separated above the connecting rail. Their font family and size can be set independently with `tabs.typeface` and `tabs.font_size_sp`, either in the Fonts settings or in `hexone.yaml`. Widths and colors are controlled by `tabs.width_mode`, `tabs.max_width_dp`, `tabs.color`, `tabs.alt_color`, and `tabs.active_color`.

## Current Dir Line

The current-dir line at the top of each pane is interactive.

Its font family and size are independent from the rest of the interface. Change the `Current dir` row in Fonts settings, or set `current_dir.typeface` and `current_dir.font_size_sp` in `hexone.yaml`. The default is Iosevka Nerd Font Mono at 11sp. The ASCII-shaped frame and sort-direction arrow are painted independently, so the frame remains continuous and the arrow scales cleanly with every selected font and size.

- Click an ancestor path segment to jump to that level. Click the current directory to reset its filter to `*.*`.
- Double-click the current-dir line to edit the path directly.
- Press `Enter` in the path editor to go there.
- Press `Esc` to cancel path editing.
- Click the `*.*` mask after `>` to edit the combined OS-style path and mask for that pane tab (for example `/work/src/*.go` or `C:\work\src\*.go`).
- Press `Enter` to apply both values. Press `Esc` or click outside the editor to cancel the draft.
- Wildcard masks accept `*` and `?`; separate alternatives with `;` or `,` (for example `*.go;*.md`).
- Prefix a filter with `re:` to use a regular expression (for example `re:^test_.*\.go$`).
- Directories and the `..` row remain visible while a filter is active. Submit an empty mask to restore `*.*`.

On Windows:

- right-click the drive segment of the current-dir line to open the drive picker
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
- `F8` or `Delete` deletes selected items. In **Settings → File panes → Other**, deletion can use the local system Trash / Recycle Bin and can optionally skip confirmation. SSH deletions are always permanent.
- `F9` opens the Tools menu (`Multi-Rename`, `Hex to ASCII`, `Protocol Analyzer`, `HTTP Client`, `Settings`).
- `Ctrl+M` / `Cmd+M` opens Multi-Rename for the current file-pane selection.
- `F10` exits the app.
- `F11` hides or shows the function key bar.
- `F12` opens or closes the terminal drawer.

If the function key bar is auto-hidden in the viewer, `F11` can still bring it back.

Right-click a local file-pane item to copy the selected file or marked files to
the system file clipboard. Finder, File Explorer, and applications that accept
file clipboard data can paste them. When the system clipboard contains files,
the local pane context menu also offers **Paste File(s)** and starts copying
immediately into the current directory. Paste progress appears in the pane's
bottom status bar without opening a modal. File clipboard actions are
unavailable inside archives and SSH panes.

When the terminal drawer is open, `Shift+Tab` toggles keyboard focus between the terminal and the file panes. Plain `Tab` stays available to the terminal for shell completion while the terminal is focused.

## Multi-Rename

Multi-Rename applies a set of filename transformations to the selected items in the active pane. Open it from `F9 -> Multi-Rename` or press `Ctrl+M` / `Cmd+M`.

- The live preview shows every old and proposed filename before anything is changed.
- Find/replace supports optional case-sensitive matching.
- Prefix and suffix text can be applied independently.
- Case conversion can keep, uppercase, or lowercase the selected part.
- `Name`, `Extension`, and `Both` control which part of each filename is transformed; directory names are handled safely.
- An optional counter supports start, step, placement at the beginning or end, and automatic zero-padding that expands to fit the largest generated number.
- Invalid names and destination collisions are detected before the operation starts. `Rename` is enabled only when at least one valid filename will change.
- Local and SFTP panes are supported. Files inside an archive cannot be renamed with this tool.

## Terminal Drawer

The terminal drawer is a real PTY-backed terminal. Open or close it with `F12`, and use `Shift+Tab` to move keyboard focus between it and the file panes.

The terminal drawer also has tabs. Its active tab opens through the straight separator rail into the terminal surface, matching the file-pane tab treatment without the current-dir frame. Use `+` for a new terminal tab, `x` to close one, and `<` or `>` when the tab row overflows.

While the terminal has keyboard focus, `Ctrl+N`, `Ctrl+X`, `Ctrl+Tab`, `Ctrl+Shift+Tab`, and the directional tab chord act on terminal tabs instead of file-pane tabs. Terminal tabs and both file-pane tab groups remain independent.

Useful terminal input:

- `Ctrl+V` or `Cmd+V` pastes clipboard text.
- Middle-click pastes the active terminal selection, or clipboard text when there is no selection. The selection does not replace the clipboard. Applications that enable terminal mouse reporting receive the middle click instead.
- Drag with the primary mouse button to select text; double-click selects a word.
- Holding `Left`, `Right`, or `Backspace` accelerates cursor movement or deletion.
- `Ctrl+C` or `Cmd+C` copies an active selection. Without a selection, plain `Ctrl+C` remains available to interrupt the running shell command.
- `Ctrl+A` or `Cmd+A` selects the terminal buffer, including scrollback.
- `Cmd+K` on macOS or `Ctrl+Shift+K` on Windows and Linux clears the active terminal tab's visible viewport and scrollback. The current prompt and partially typed command remain at the top.
- Plain `Ctrl+K` is passed through to the shell for line editing.

Right-click opens the terminal context menu. It can:

- copy, paste, or select the full terminal buffer
- send `cd` commands to move the terminal to the left or right pane's local directory
- set the left or right pane to the terminal's current directory, including SSH shells through OSC 7 tracking or an on-demand directory query

For local shells, Hexone can usually read the terminal process current directory directly.

### Terminal Snippets

Use the `☆` button at the right side of the terminal tab row, or `Ctrl+Shift+P` / `Cmd+Shift+P`, to save and insert terminal snippets. The keyboard shortcut opens the menu; use the arrow keys and Enter to choose an item.

- `Save current command…` opens an editable draft taken from the current prompt line.
- Terminal snippets are single-line prompt insertions; use the F2 custom commands feature for multi-line commands.
- The snippet editor prefills the command currently being typed, or the last command submitted in that terminal when the prompt is empty.
- Every snippet has exactly one scope: `Global`, `Directory`, or `Git repository`.
- Directory snippets match only the exact terminal folder.
- Git repository snippets match anywhere below the saved repository root.
- Selecting a snippet inserts it at the prompt for review; it is not executed automatically.
- Use the `x` beside a snippet to remove it.

When the current terminal is inside a local Git repository, new snippets default to repository scope. Outside a repository they default to directory scope when the terminal location is available, and otherwise to global scope. Repository scope is unavailable for remote terminals because Hexone cannot safely infer the remote repository root without running a command.

For SSH shells that already emit OSC 7 working-directory updates, Hexone continuously tracks the reported remote directory. No remote-shell configuration is required for on-demand pane sync: if an active SSH shell has not reported OSC 7, `Set Left Pane to Terminal Dir` and `Set Right Pane to Terminal Dir` inject a one-shot `printf` command at the empty shell prompt to query `$PWD`. Leave full-screen programs and clear any partially typed command before using the action.

Remote terminal sync preserves existing pane tabs. If the chosen pane already has a tab connected to the terminal's SSH server, Hexone activates that tab and changes only its directory. Otherwise it opens the server in a new pane tab instead of replacing the currently selected local or remote tab.

If your remote shell does not already emit OSC 7, add something like this to the remote `~/.bashrc`:

```sh
__osc7() {
  printf '\033]7;file://%s%s\007' "$(hostname)" "$PWD"
}
PROMPT_COMMAND="__osc7${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
```

After reconnecting, Hexone can use those OSC 7 updates without injecting a command. If the remote OSC 7 hostname differs from the saved SSH setup host, Hexone also checks the active `ssh` process target to map the terminal session back to a saved SSH setup.

If there is no matching Hexone SSH setup, Hexone can use the destination from the active terminal command. It asks the installed OpenSSH client for the effective `~/.ssh/config` values, then connects with a configured identity file or a key already loaded in `ssh-agent`. An encrypted key that is not available through the agent opens a one-time passphrase prompt. The remote host must already be present in the OpenSSH `known_hosts` file. `ProxyJump` and `ProxyCommand` configurations are reported as unsupported instead of being ignored.

## Custom Commands

`F2` is for saved commands that run in the command-only viewer and are not tied to a single file. The editor stores up to 10 fixed slots, each with a short name and multi-line command body.

- `Run` saves the current command and shows its output in a command-only viewer.
- Saving an empty command clears the selected slot.
- `Ctrl+Enter` or `Cmd+Enter` runs from the editor.
- `Ctrl+S` or `Cmd+S` saves from the editor without closing it.
- `Esc` closes the editor, F2 menu, or command output viewer.
- `Ctrl+1` through `Ctrl+0` run slots 1 through 10. Plain number keys also work while the F2 menu is open.

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

Saved passwords and private-key passphrases are stored in Windows Credential Manager, macOS Keychain, or the Linux Secret Service. They are not written to `hexone.yaml`; existing plaintext values are moved automatically the next time Hexone starts. If no secure credential service is available, the credentials can still be entered for a one-time connection but cannot be saved.

Remote favorites do not require a separate Hexone SSH setup when their host can be resolved by OpenSSH configuration and authenticated through `ssh-agent` or an `IdentityFile`. Hexone SSH setups remain the first choice when a matching setup exists.

Inside the internal viewer, the same shortcut opens Find instead of `SSH Sessions`.

Once connected, a remote pane supports normal browsing plus viewer-based inspection, command-driven log viewing, and remote-assisted hex searching.

## Internal Viewer

The internal viewer has three explicit modes, plus automatic image-style preview inside `file` mode for supported images and PDFs:

- `file` for normal text content plus image/PDF preview when supported
- `hex` for raw bytes
- `command` for shell output based on the selected file

The centered filename rail stays visible while switching between `File`, `Hex`, and `Cmd`. Unsaved edits are marked after the filename, for example `[notes.txt *]`.

New viewer opens default to `file`; exact target commands and filename command rules open in `command`, while files over the configured read limit open in `hex` when no target or rule command applies.

Useful viewer keys:

- `F4` turns editing on in text and hex views
- `F3` discards unsaved changes and turns editing off; outside edit mode it refreshes the current file or reruns the current command
- `Esc` discards unsaved changes and closes the viewer
- `F5` toggles line numbers in File and Cmd text views
- `F2`, `Ctrl+S`, or `Cmd+S` saves changes
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
- in Hex edit mode, click the HEX columns to edit individual nibbles
- click the ASCII column to edit whole bytes
- two hex digits complete one byte and move to the next byte
- drag in either lane to select multiple bytes
- the cyan-tinted background shows the active input lane
- dragging across the HEX and ASCII lanes switches the input mode
- a multi-byte selection is an overwrite range
- ASCII input writes the typed character to every selected byte
- the first HEX digit overwrites every selected high nibble
- the second HEX digit overwrites every selected low nibble
- modified bytes use pink/red text in both columns until saved
- restoring a byte to its original value removes its modified marker
- the active HEX nibble is cyan
- active ASCII characters are also cyan
- arrow keys, paging keys, Home, and End move the active byte
- saving clears all modified-byte markers
- hold `Shift` with those navigation keys to extend the byte selection from the active caret
- moving away after entering one hex digit keeps the changed high nibble and preserves the byte's original low nibble
- the Hex context menu offers `Copy as Hex` and `Copy as Text`
- text copy recognizes printable UTF-8 text and writes invalid or non-text bytes as `\xNN` escapes
- text saves preserve the detected UTF-8, UTF-16, or CP437 encoding, BOM, and CRLF line endings
- read-only File mode uses the same compact line spacing and visual font weight as File edit mode
- File edit mode follows the Word Wrap setting
- when wrapping is off, long lines stay intact and use a horizontal scrollbar
- File edit mode provides `Copy` and `Paste` in its context menu
- local and SFTP files can be edited; files inside archives and image/PDF/command previews remain read-only
- on SSH panes, `hex` Find can use the configured remote search utility command for large files

### Command Mode

`command` mode runs a shell command against the selected file and captures its output.
It is not a terminal: there is no PTY, no interactive stdin, and no full-screen pager UI.
Recent commands are kept in viewer command history for quick reuse. The eye action in the active `Cmd` tab shows the current command; click it to open command history, where the action changes to a pen. Click the pen to return to the current command.
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

- oversized images initially fit the viewport width and stay aligned to the top; smaller images remain at native size and are centered
- drag the image directly to pan it, or use the scrollbars
- arrow keys pan the image
- `PageUp` and `PageDown` move by a larger vertical chunk
- `Home` goes to the origin and `End` goes to the far edge
- `Ctrl++` / `Ctrl+-` or `Cmd++` / `Cmd+-` zoom in and out
- click the zoom percentage for fit-width and common zoom presets

### PDF Preview

- PDFs open as one continuous document: pages are stacked vertically and fitted to the window width
- scrolling is smooth and seamless across page boundaries with the mouse wheel, `Up` / `Down`, and `PageUp` / `PageDown`
- the vertical scrollbar represents the combined height of all pages, not the page count
- dragging over text selects it (the cursor becomes a text beam); dragging anywhere else pans the page
- holding a selection drag past the top or bottom edge auto-scrolls the document; double-click selects the word under the cursor
- `Shift`+drag forces text selection; `Ctrl+C` / `Cmd+C` or the right-click `Copy` menu copies the selection
- `Ctrl++` / `Ctrl+-` or `Cmd++` / `Cmd+-` zoom in and out around the viewport center; `100%` means fit-to-width
- click the zoom percentage for fit-width and common zoom presets
- PDFs with an outline show a compact `TOC` control; use a row's arrow to expand one branch at a time without moving the current page, or click any bookmark title—including a parent—to navigate
- `Esc` closes an open zoom or TOC popup before closing the viewer
- `[` moves to the previous page and `]` moves to the next page

## Customization

Most everyday customization is available from Settings.
The full configuration also lives in `hexone.yaml`.

Hexone embeds four Nerd Font Mono families, so portable builds do not need external font files:

- FiraCode Nerd Font Mono
- Hack Nerd Font Mono
- JetBrainsMono Nerd Font Mono
- Iosevka Nerd Font Mono

Use `Settings -> Fonts` to choose the family and size independently for interface controls, file panes, tabs, the internal viewer, and the terminal. The interface font covers menus, tools, and dialogs, so pane text can be enlarged without distorting them.

On Linux, the writable config files live under `~/.config/hexone/`.
On macOS, they live under `~/Library/Application Support/hexone/`.
On Windows portable builds, they live beside `hexone.exe`.
On Windows MSIX builds, they live in the package's `LocalState` folder under `%LOCALAPPDATA%\Packages\`.

Useful things to adjust:

- fallback command
- exact target overrides
- filename regex rules
- shell selection for viewer commands and the terminal drawer
- remote search utility command for SSH hex find
- viewer smooth scrolling
- viewer line numbers (shown by default)
- viewer word wrapping for File and Cmd text
- file encoding defaults
- auto-refresh interval for non-streaming command mode
- pane, tab, viewer, and terminal font family and size
- function bar auto-hide while the viewer is open
- system associations

Example:

```yaml
viewer:
  shell: auto
  command: cat {path}
  smooth_scrolling: true
  show_line_numbers: true
  word_wrap: false
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
    slot: 1
    command: |
      pid=$(pidof gpstrack | awk '{print $1}')
      python - "$pid" <<'PY'
      print("PID", __import__("sys").argv[1])
      PY
```

Notes:

- `shell: auto` picks a sensible shell automatically; Windows can also use `pwsh`, `powershell`, `cmd`, `wsl`, or `wsl:<distro>`
- Priority 1: `command_by_target` matches one exact full path and overrides any regex-selected command
- Priority 2: `command_rules` match the filename only; later matches override earlier ones
- Priority 3: `command` is the generic fallback
- `command_rules` switch the viewer into command mode automatically when a filename matches
- `command_by_target` overrides the chosen command and opens that target in command mode by default
- `remote_search_command` is used by SSH hex Find; set it to `off` to disable the remote utility path
- `show_line_numbers` controls the File and Cmd text gutter and can also be toggled with `F5`
- `word_wrap` controls File and Cmd text and can also be toggled from their right-click menu
- `command_auto_refresh` matters most for non-streaming command mode
- Settings -> Viewer exposes the same priority order directly in the UI, along with smooth scrolling, line numbers, and viewer auto-hide

## HTTP Client

Open **F9 → HTTP Client** for a compact request workbench.

- The left pane lists collections, folders, and saved requests from `hexone-http.yaml`. Use the connected `[ + ]` menu to add a request, folder, or collection to the selected part of the tree; click collection or folder rows to collapse or expand them.
- Press `Tab` or `Shift+Tab` to move between the environment controls, collection tree, request-tab group, method and URL controls, Send/Save group, request-detail and Auth groups, response tabs, and environment-editor fields/actions. Within a horizontal group, use `Left`/`Right` to move and activate its item; a thin accent underline marks keyboard focus. Use `Enter` or `Space` to activate focused buttons. A request context menu also exposes its Rename, Run-with-environment, and Delete actions to the same keyboard navigation.
- In the collection tree, `Up`/`Down` visit every visible row in order, `Left` enters or expands the selected collection/folder, and `Right` returns to its parent. The selected row scrolls into view automatically. When the terminal owns keyboard focus, HTTP Client shortcuts yield all key presses to it.
- Drag the vertical collection separator or the horizontal request/response separator to resize the panes. The collection width stays fixed when the application window width changes; the request column absorbs the change. Collections, request content, and responses scroll independently and show compact scrollbars when their content overflows.
- Selecting a request opens it in the connected request-tab strip. Use `x` to close a view without deleting the saved request, or `+` to create a new request in the `Scratch requests` collection.
- Click the compact method or environment selector to move to the next available value.
- Edit query parameters as `name=value` lines and headers as `Name: value` lines. Prefix a line with `#` to keep it saved but disabled.
- The Auth view supports Basic credentials, Bearer tokens, and API keys sent in either a header or the query string. Choose **Inherit env** to use the authentication configured on the selected environment; double-click the environment selector to edit its variables and authentication.
- HTTP passwords, bearer tokens, API-key values, and environment-variable values are stored in Windows Credential Manager, macOS Keychain, or the Linux Secret Service. The YAML file and its backup contain only opaque credential references; existing plaintext collections are migrated when the HTTP Client opens. Saving fails safely when secure credential storage is unavailable.
- Secret authentication fields are masked by default; use **Show** or **Hide** at the end of the field to control their visibility while editing.
- Request bodies are stored as plain multi-line text.
- `Enter` in the URL field sends the current request. `Ctrl+Enter` or `Cmd+Enter` sends it from elsewhere in the workbench.
- `Ctrl+S` or `Cmd+S` saves all collection changes atomically. Replacing an existing file also creates `hexone-http.yaml.bak`.
- Response views include pretty JSON, the raw body, and ordered response headers.
- Environment values can be referenced as `{{variable_name}}` in URLs, query parameters, headers, authentication fields, and bodies.

Hexone creates `hexone-http.yaml` beside `hexone.yaml` the first time the HTTP Client opens and restricts it to the current OS user. Collection headers and query parameters use YAML lists so duplicate names and display order are preserved.

## Protocol Analyzer

The Protocol Analyzer decodes pasted hex using `protocols.yaml`.

On Linux, Hexone first checks `~/.config/hexone/protocols.yaml`. If that file is missing, it uses the embedded default and writes a reference sample to `~/.config/hexone/protocols.sample.yaml`.
On macOS, it first checks `~/Library/Application Support/hexone/protocols.yaml` and writes the reference sample to `~/Library/Application Support/hexone/protocols.sample.yaml`.
On Windows portable builds, it uses `protocols.yaml` beside `hexone.exe`.
On Windows MSIX builds, it first checks the package's writable `LocalState` folder, then uses the packaged default and writes `protocols.sample.yaml` to `LocalState`.

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
