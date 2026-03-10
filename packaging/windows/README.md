# Windows Packaging

`cmd/hexone/hexone_windows.syso` is the compiled Windows resource object that Go
links into `hexone.exe`. It carries the executable icon and the metadata shown
in Explorer under file properties.

## Update metadata

Most fields live in `cmd/hexone/app_icon_windows.rc`. The version fields are not
hardcoded there; `make windows-resource` injects the numeric Windows version
from the latest `v*` Git tag:

- If `HEAD` is exactly on `v0.1.0`, Explorer gets `0.1.0.0`.
- If `HEAD` is ahead of `v0.1.0`, Explorer still gets `0.1.0.0`.
- The 4th component is reserved and currently fixed to `0`.

Regenerate the compiled resource with:

```powershell
make windows-resource
```

Useful fields in the resource:

- `FILEVERSION` and `PRODUCTVERSION`: numeric form derived from the latest Git
  tag, with the 4th Windows component fixed to `0`.
- `FileVersion` and `ProductVersion`: human-readable strings shown in Explorer.
- `FileDescription`: short description shown in file properties.
- `CompanyName`: publisher name.
- `OriginalFilename`: the canonical executable name.
- `FILETYPE VFT_APP`: marks the binary as an application.

After regenerating the `.syso`, rebuild the app:

```powershell
make build
```
