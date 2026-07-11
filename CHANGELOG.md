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

## v0.9.0

- Made Enter launch ordinary files with their system association while continuing to traverse directories and archives.
- Sped up multipart RAR indexing and reused archive indexes while browsing nested folders and opening members.
- Kept explicit plain-text files such as `.txt` free from content-guessed syntax highlighting.
- Added SSH Setup to the F9 Tools menu and moved Colors directly after Fonts in Settings.
- Added separate Full mode, Brief mode, and Other file-pane settings with large live previews, accurate Full-mode column sizing, distinct Brief-mode filenames, and a date/time builder that automatically derives responsive fallback formats.
- Settings now marks a dirty draft explicitly as `Save (*)`, returning to `Save` after changes are saved or reverted.
- Reworked the Settings color picker with an RGB honeycomb, tonal slider, live color indicator, and explicit Set action.
- Replaced rounded Settings remove icons with flat close controls and red hover feedback matching Favorites.
- Flattened the Settings Browse, Add, Update, and Remove actions for a lighter visual hierarchy.
- Fixed keyed Settings entries so Update can change ages, permission modes, extensions, sizes, viewer targets, command patterns, and association extensions without creating a duplicate.
- Added theme-aware separators between the offset, byte, and ASCII sections of the hex viewer without changing selection hit-boxes.
- Added independent text color settings for all three hex viewer sections.
- Added an independent hex-selection color setting.
- Restyled the sticky File, Hex, and Cmd viewer tabs to use the configured tab font and color styling.
- Added a File/Hex toggle to the viewer color preview in Settings.
- Improved past-command visibility with a flat, full-width history list using the Interface font.
- Centered labels across viewer, file-pane, and terminal tabs.
- Replaced rounded dialog close buttons with flat tab-style controls and red hover feedback.
- Fixed newly created folders remaining outside the visible file-pane rows when the terminal drawer reduces the viewport.
- Disabled visual smoothing during active selection auto-scroll in File, Hex, Cmd, and Terminal views to keep text and selection row-aligned.
- Improved SSH tabs and favorites with clean directory-only tab titles, hoverable host-detail indicators, and stable theme-aware colors per host.
- Aligned SSH Setup and Settings actions with the file-operation dialogs, including right-aligned footers, improved spacing, and a flatter, roomier SSH setup layout.
- Extended the flatter modal treatment across SSH Sessions, Multi-Rename, file-operation dialogs, Custom Commands, Settings, and F1 Help, including divider-based sections, flat action buttons, and a flattened current-directory control strip.
- Changed the SSH Disconnect action to close the current remote tab when alternatives exist, while preserving the last tab and returning it to its previous local directory.
- Restyled the active Cmd command as a compact fully bordered recessed field using the Tabs font and consistent display and edit-mode geometry.
- Reworked Settings into File panes and Terminal sections, including regular/bold file-pane weight controls per file, directory, permissions, size, and date columns, plus relocated terminal shell and accelerated-key options.
- Added file/directory/both targeting to filename color rules, applying filename customizations to both by default while preserving an explicit directory icon color.
- Made Settings height responsive at roughly 80% of the window so dense tabs remain comfortable without filling the entire screen, retained right-aligned actions, and tightened Custom Commands so its editor aligns with the slot list and wastes less footer space.
- Prevented Custom Commands editor navigation and shortcut keys from leaking through to the underlying file pane.
- Applied the same responsive 80%-of-window height policy to the F1 Help modal as Settings, with an integrated scrollbar and flattened content area.
- Fixed File-mode text shifting horizontally when asynchronous syntax highlighting appears.
- Simplified Multi-Rename counters by replacing the manual digit-width field with automatic zero-padding.
- Improved image previews with responsive fit-width startup, native-size centering, drag panning, and clickable zoom presets; viewer mode tabs now carry the full filename, and outlined PDFs expose dense accordion TOC navigation with separate disclosure and bookmark links.
- Kept supported oversized images and PDFs in File preview mode instead of incorrectly opening them in Hex mode because of the text-read size limit.
- Made `Esc` close open viewer zoom and TOC popups before closing the viewer itself.
- Kept Windows portable configuration beside `hexone.exe` while moving MSIX configuration and session data to the package's writable `LocalState` folder.
- Added `make package-windows-msix` to build the Windows PDFium binary, generate Store assets, and create the MSIX package in one command.
- Repacked and validated Windows MSIX output with the Windows SDK to prevent invalid nFPM block maps from reaching App Installer or the Store.
- Enlarged the visible MSIX icon artwork and added scale-qualified Store, app-list, and tile assets for sharper Windows rendering.
- Made local MSIX builds create a separately signed development package with a reusable trusted certificate while keeping the Partner Center artifact unsigned.
