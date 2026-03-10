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
