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

> [!CAUTION]
> **Applies to: macOS**
>
> If the first launch gets blocked and macOS suggests moving Hexone to Trash, do not do that. First try opening Hexone once, then open `System Settings -> Privacy & Security`, scroll down to `Security`, and click `Open Anyway` for Hexone. On current macOS versions that button is typically available for about an hour after the blocked launch attempt. Avoiding that warning for outside-the-App-Store distribution generally means paid Apple code-signing/distribution, which is not a practical fit for this free project.

> [!CAUTION]
> **Applies to: Windows**
>
> SmartScreen is reputation-based and is aggressive toward new unsigned `.exe` files. If you see the blue `Windows protected your PC` dialog, use `More info -> Run anyway` if you trust the download. Removing that warning cleanly usually means either EV code signing or shipping through the Microsoft Store / MSIX path. Unsigned builds can build reputation over time, but each new release starts over with a new file hash.

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

Hexone stores `hexone.yaml`, `hexone.session.yaml`, `protocols.yaml`, and `protocols.sample.yaml` in these locations:

- Linux: `~/.config/hexone/`
- macOS: `~/Library/Application Support/hexone/`
- Windows: currently in the current working directory as `hexone.yaml`, `hexone.session.yaml`, `protocols.yaml`, and `protocols.sample.yaml`
- Other platforms: the same local-file fallback as Windows

The protocol analyzer checks `protocols.yaml` in that location first, falls back to the embedded default if it is missing, and writes `protocols.sample.yaml` on first run.

More usage details are in [HELP.md](HELP.md).
