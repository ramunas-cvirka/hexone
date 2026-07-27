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

- File and Hex viewer modes now support editing. Changes can be saved or discarded. Hex mode accepts HEX and ASCII input. A selection can overwrite several bytes. Changed bytes are highlighted and saved in place.
- Redesigned the internal viewer. Separate File, Hex, and Cmd tabs share a persistent filename rail. The viewer now has its own function bar with viewer-specific actions.
- Added native desktop file clipboard integration and optional Trash / Recycle Bin deletion.
- Redesigned the file-pane tabs and current-directory line. Active tabs connect to the current-directory line through a notched frame. The line includes breadcrumbs, editable filters, and pane controls. Brief mode now sizes columns to their contents.
- Added terminal snippets with global, directory, and Git-repository scopes.
- Redesigned the Protocol Analyzer for clearer input, navigation, and decoding.
