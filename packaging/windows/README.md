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
