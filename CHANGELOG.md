# Changelog

All notable changes to this project will be documented in this file.

Release notes extraction in CI expects release headings that begin with `## v...`.

## v0.1.0 - 2026-03-10

- Initial public release.
- Added tag-based GitHub Releases workflow with portable ZIP artifacts for Linux, macOS, and Windows.
- Portable ZIP layout includes the app binary and `protocols.yaml`.

## v0.2.0 - 2026-03-27
- Search bar inside the File/Hex mode (ctrl+f)
- Remote search feature allows searching inside huge files without downloading them in Hex mode
- File name coloring by extension and various other filters
- Viewing images (PNG,GIF,JPG) inside the viewer (F3)
- Various UI & navigation tweaks

## v0.3.0 - 2026-04-06
- Tabbing support everywhere - to walk through controls
- Smooth scrolling inside the viewer (F3)
- Function keys bar context-aware (hold ctrl or alt)
- Added shortcuts: to select all and all of the same type

## v0.4.0 - 2026-04-14
- PDF viewing — open and page through PDFs directly in the viewer with a scrollbar and keyboard navigation
- Syntax highlighting — code and log files are now highlighted in File mode
- File list auto-refreshes when the current directory changes on disk
- Free and total disk space shown at the bottom of each pane
- Various fixes & tweaks

## v0.5.0 - 2026-05-26
- Added rounded scrollbars throughout the app
- Improved streaming mode by discarding buffered data before it reaches the size limit
- Added multi-file RAR extraction with progress status
- Fixed associated app launching in cases where paths or arguments were handled incorrectly
- Added cancellable copy operations with transfer speed indication
- Added a small diff view when overwriting files during copy
- Improved Unix link handling and warmed up the viewer for faster initial loads
- Removed the default viewer mode setting
- Replaced the `F2` WIP behavior with custom commands, including a fixed-size command builder with 10 command slots

## v0.6.0 - 2026-06-15
- New integrated terminal drawer on `F12`, with `Shift+Tab` to jump between the terminal and file panes.
- Added terminal folder shortcuts for the left and right panes, including supported SSH sessions.

## v0.7.0 - 2026-06-28

- Added tabs to both file panes and the terminal drawer, with overflow controls.
- Added `Ctrl+N`, `Ctrl+X`, `Ctrl+Tab`, and `Ctrl+Shift+Tab` shortcuts for tab management.
- Added preview-first Multi-Rename with common transformations, collision checks, and local/SFTP support.
- Improved terminal selection, clipboard actions, buffer clearing, and pane-directory integration.
- Embedded FiraCode, Hack, JetBrainsMono, and Iosevka Nerd Font Mono, replacing Fira Code and Consolas.

## v0.8.0 - 2026-06-30

- Added optional completion sounds for long copy and archive extraction operations.
- Added a maximized Terminal mode with saved restore state.
- Added an Interface font setting for modals, menus, and tools.
- Improved font scaling support across interface surfaces.
- Improved Favorites and SSH setup layouts with a flatter look.
- Improved terminal key repeat, drag selection, and smooth scrolling behavior.
- Terminal now uses all available bottom space.
- Fixed SSH favorites opened in new tabs using the previous local pane path.

## v1.0.0 - 2026-07-13

- Rebuilt F5 Copy so directory contents are discovered and transferred concurrently instead of scanning the complete directory tree before copying begins. The new progress view reports discovered and copied files, the current item, transferred bytes, and transfer speed for local and SFTP operations.
- Rebuilt the PDF viewer as one continuously scrolling document with text selection and copying, clickable links, drag panning, zoom controls, page navigation, and an expandable table of contents for PDF outlines.
- Added whole-document PDF search and redesigned Find in File, Hex, and Cmd views to show matching snippets in a result list and jump directly to any selected match.
- Added fit-to-width startup, native-size centering, drag panning, keyboard navigation, and selectable zoom levels to image previews.
- Added dedicated settings for Full, Brief, and Other file-pane layouts, including live previews, per-column font weights, optional permission columns, responsive date/time formats, and filename color rules that can target files, directories, or both.
- Added separate text colors for the offset, byte, and ASCII sections of the hex viewer, a separate hex-selection color, and visual dividers between the three sections.
- Reworked Settings, Help, SSH Sessions, Custom Commands, Multi-Rename, and file-operation dialogs around the same flat, sectioned layout, and replaced the Settings color editor with an RGB honeycomb and tonal slider.
- Added a saved Word Wrap option for File and Cmd text views.
- Saved SSH passwords and private-key passphrases in Windows Credential Manager, macOS Keychain, or the Linux Secret Service instead of `hexone.yaml`.
- Changed Enter to open regular files with their system-associated application while continuing to open directories and archives inside Hexone.
- Added a Microsoft Store MSIX distribution. Store installations keep configuration and session data in the package's `LocalState` folder, while Windows portable builds continue to keep configuration beside `hexone.exe`.

## v1.1.0 - Unreleased

- Added terminal snippets, available from the `☆` button or `Ctrl+Shift+P` / `Cmd+Shift+P`. Snippets can be global, limited to one directory, or shared across a local Git repository, and are inserted at the prompt for review without running automatically.
- Improved Brief mode so filename columns follow the longest displayed name in the current directory up to the configured maximum. Shorter names now produce narrower columns, allowing more columns to fit without prematurely truncating filenames.
- Restyled the function bar so `F1`–`F10` and held `Ctrl` / `Alt` shortcuts use a bold, high-contrast color while their action labels remain normal-weight.
- Improved the File panes settings previews with representative long filenames, a responsive Brief-width preview, and `?` help tooltips that explain when the Full and Brief filename-width settings take effect.
- Added a GitHub Actions workflow that runs the complete Go unit-test suite on every branch push and pull request.
- Added editable File and Hex viewer modes. `F4` enters editing, `F3` discards changes and returns to view mode, `Esc` discards changes and closes the viewer, and `F2` / `Ctrl+S` / `Cmd+S` saves. Switching viewer modes with unsaved changes now offers Save or Discard.
- File editing preserves syntax highlighting, compact line spacing, scrollbars, word wrapping, text encoding, BOM, and line endings, and adds working Copy and Paste context-menu actions. Find remains available while editing and searches the last saved/view buffer.
- Hex editing supports separate hexadecimal-nibble and ASCII input lanes, keyboard navigation, drag selection, multi-byte replacement, highlighted active nibbles, and distinct unsaved-byte coloring. Hex saves are sparse in-place patches that write only changed byte ranges, regardless of total file size, and Hex copy now supports both hexadecimal and escaped-text formats.
- Redesigned the viewer header with equal-width retro File, Hex, and Cmd tabs, a centered and middle-truncated filename rail, dirty-file marking, and eye/pen actions for view/edit state and current-command/history selection.
- Custom Command and SSH Sessions editors now show `Save (*)` whenever their drafts differ from the saved configuration and clear the marker after saving or reverting the changes.
- Added `Backspace` navigation in file panes to open the parent directory, matching the traditional file-manager `cd ..` behavior.
- Changed file-pane mouse-wheel scrolling to move the viewport without changing the active item: rows scroll in Full mode and columns scroll in Brief mode. The previous selection-moving behavior remains available through the **Mouse wheel moves the active item** setting and `wheel_moves_selection` configuration option.
- Added a viewer-specific function bar: `F2` saves edits, `F3` returns to view-only mode or closes an already read-only viewer, `F4` enters editing, `F7` finds, `F8` switches between text and hex, `F9` toggles wrapping, and `F10` exits Hexone while discarding unsaved viewer changes. `F5` and `F6` are intentionally unassigned in the viewer.
- Added optional local Trash / Recycle Bin deletion, disabled by default, with a recoverable-action confirmation message and native platform integration. A separate disabled-by-default option skips deletion confirmation; SSH deletions always remain permanent. These options are also available as `use_trash` and `delete_without_confirmation` in the configuration file.
- Settings now marks the draft as `Save (*)` when any of the new file-pane mouse-wheel or deletion options changes, and clears the marker when the change is reverted.
- Fixed mouse cursors becoming stuck as an arrow or hand after crossing between the viewer, file panes, terminal, and app boundaries. Clipboard-read targets now remain pointer-transparent and expire cleanly so pending or failed paste operations cannot leave stale cursor routing behind.
