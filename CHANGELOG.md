# Changelog

All notable changes to this project will be documented in this file.

Release notes extraction in CI expects release headings that begin with `## v...`.

## Unreleased

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