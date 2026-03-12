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

macOS note: if the first launch gets blocked and macOS suggests moving Hexone to Trash, do not do that. First try opening Hexone once, then open `System Settings -> Privacy & Security`, scroll down to `Security`, and click `Open Anyway` for Hexone. On current macOS versions that button is typically available for about an hour after the blocked launch attempt. The reason this workaround is needed is that avoiding that warning for outside-the-App-Store distribution generally means paying for Apple Developer Program membership, which Apple currently lists at `US$99/year`, and that is not a practical fit for this free project.

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
make package-linux-zip
make package-macos
make package-windows
```

- `package-linux` builds an AppImage and needs a Linux host, `patchelf`, and `appimagetool`
- `package-linux-zip` keeps the older portable ZIP layout
- `package-macos` needs macOS

On Linux, Hexone stores `hexone.yaml` and `hexone.session.yaml` under `~/.config/hexone/`. The protocol analyzer looks for `~/.config/hexone/protocols.yaml` first, falls back to the embedded default, and writes a sample to `~/.config/hexone/protocols.sample.yaml` on first run.

More usage details are in [HELP.md](HELP.md).
