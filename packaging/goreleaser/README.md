# GoReleaser Packaging

This is the experimental GoReleaser OSS release path. The existing Makefile
package targets and `.github/workflows/release.yml` remain the backup release
path.

## What GoReleaser Owns

- Linux amd64 archive for direct download and Homebrew/Linuxbrew.
- Windows amd64 portable ZIP.
- Linux Flatpak bundle.
- GitHub release creation, checksums, changelog, and optional Homebrew tap
  publishing.

## What Stays Outside GoReleaser OSS

- macOS `.app` and `.dmg` creation stay on `make package-macos`; GoReleaser
  uploads the DMG as an extra release file.
- Windows Store MSIX creation uses nFPM directly via
  `packaging/windows/build_msix.sh`; GoReleaser uploads the MSIX as an extra
  release file.
- GoReleaser Pro-only features such as DMG/app bundles and split/merge are not
  used.

## GitHub Workflow

`.github/workflows/goreleaser.yml` is manual-only for now so it cannot race the
current tag-based release workflow. Run it with `publish=false` for a snapshot
dry run.

To publish for real:

1. Run the workflow from a `v*` tag.
2. Set `publish=true`.
3. Set repository variables `HEXONE_MSIX_PUBLISHER` and
   `HEXONE_MSIX_PUBLISHER_DISPLAY_NAME` to the values from Partner Center.
4. Add `HOMEBREW_TAP_TOKEN` if publishing to `ramunas-cvirka/homebrew-hexone`.
5. Disable or retarget the old `release.yml` tag trigger once this workflow is
   the primary release path.
