# Changelog

All notable changes to this project will be documented in this file.

Release notes extraction in CI expects release headings that begin with `## v...`.

## Unreleased

- Added a preview-first Multi-Rename tool with find/replace, prefix/suffix, case conversion, extension protection, numbering, collision checks, and local/SFTP support.
- Added compact tabs for each file pane and the terminal drawer, with overflow controls and configurable colors/widths.
- Added keyboard shortcuts for creating, closing, and navigating file-pane and terminal tabs.
- Added an in-app help modal on `F1` with bundled help content.
- Embedded the default bundled fonts so portable builds no longer need the external `assets/` directory.
- Added Windows executable metadata resources with Git tag-derived versioning.

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
