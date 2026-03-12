# Hexone

Hexone is a keyboard-first desktop file manager with a built-in text/hex/command viewer.

<p align="center">
  <img src="assets/main.png" alt="Hexone main view" />
</p>

<p align="center">
  <img src="assets/viewer.png" alt="Hexone viewer" />
</p>

## Features

- One viewer for text, hex, and command output against the selected file
- SSH browsing is supported
- The viewer is meant to be more practical for very large logs, where opening multi-GB files naively is not useful
- Keyboard-first workflow without leaving the app for common inspection tasks
- Favorites are supported

## Also Included

- A few smaller tools I use in daily work are included too, such as hex-to-ASCII and a protocol analyzer based on `protocols.yaml`

## Downloads

Use the packages from the Releases page.

macOS note: if the first launch gets blocked and macOS suggests moving Hexone to Trash, do not do that. Open `System Settings -> Privacy & Security`, scroll down, and click `Open Anyway` for Hexone. A quicker one-time workaround is usually to Control-click the app in Finder and choose `Open`.

If you want to build it yourself instead:

Requirements:

- Go 1.26

Commands:

```sh
make build
make run
```

Optional packaging:

```sh
make package-linux
make package-macos
make package-windows
```

- `package-linux` needs a Linux host and `patchelf`
- `package-macos` needs macOS

More usage details are in [HELP.md](HELP.md).
