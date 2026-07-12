# Windows Packaging

`cmd/hexone/hexone_windows.syso` is the compiled Windows resource object that Go
links into `hexone.exe`. It carries the executable icon and the metadata shown
in Explorer under file properties, plus the embedded application manifest.

## Update metadata

Most fields live in `cmd/hexone/app_icon_windows.rc`, while the Windows
manifest template lives in `cmd/hexone/app_windows.manifest`. `make
windows-resource` renders both files, injects the numeric Windows version from
the latest reachable `v*` Git tag, and also injects the current copyright year:

- If `HEAD` is exactly on `v0.1.0`, Explorer gets `0.1.0.0`.
- If `HEAD` is ahead of `v0.1.0`, Explorer still gets `0.1.0.0`.
- The 4th component is reserved and currently fixed to `0`.

The normal `make build`, `make build-windows`, packaging, and GoReleaser paths
regenerate the compiled resource automatically. To regenerate it on its own,
use:

```powershell
make windows-resource
```

Useful fields in the resource:

- `FILEVERSION` and `PRODUCTVERSION`: numeric form derived from the latest Git
  tag, with the 4th Windows component fixed to `0`.
- `FileVersion` and `ProductVersion`: human-readable strings shown in Explorer.
- `FileDescription`: short description shown in file properties.
- `CompanyName`: publisher name.
- `LegalCopyright`: rendered with the current year by default. Override with
  `HEXONE_COPYRIGHT_YEAR` if needed.
- `OriginalFilename`: the canonical executable name.
- `FILETYPE VFT_APP`: marks the binary as an application.

The Windows manifest is embedded through `cmd/hexone/app_icon_windows.rc`. It
currently enables Common Controls v6, `asInvoker`, and Per-Monitor V2 DPI
awareness.

After regenerating the `.syso`, rebuild the app:

```powershell
make build
```

## Build the Store MSIX

From the repository root, build the Windows PDFium binary, generate the MSIX
assets, and package them with the Store identity in one command:

```powershell
make package-windows-msix
```

The build stages the package directly, generates `resources.pri` with the
Windows SDK's `MakePri.exe`, and then packs and validates the archive with
`MakeAppx.exe`. The PRI step is required for Windows to resolve the scale,
target-size, and light/dark unplated icon variants. It writes two packages
locally:

- `dist/hexone_windows_amd64.msix` is unsigned and intended for Partner Center.
- `dist/hexone_windows_amd64_dev.msix` is signed with a reusable local
  development certificate and intended only for local installation.

The first local build requests one UAC confirmation to trust the public
development certificate in `LocalMachine\TrustedPeople`. The private key stays
in the current user's Windows certificate store and is never written to the
repository or package output. Later builds reuse the same certificate and sign
the development package without another prompt. GitHub Actions builds with
`MSIX_DEV_SIGN=false`, producing only the unsigned Store artifact.

`MakePri.exe` is installed by the Windows SDK's **UWP Managed Apps** component.
The packaging command stops with an actionable error if that component is not
installed; silently packaging qualified filenames without `resources.pri`
would leave Windows unable to select the intended shell icons.

Install the local package by double-clicking `hexone_windows_amd64_dev.msix` or
with `Add-AppxPackage`. Do not use the `_dev.msix` package for Store submission.
