# Copyright 2026 Ramunas Cvirka
# SPDX-License-Identifier: Apache-2.0

param(
    [Parameter(Mandatory = $true)][string]$PkgConfigPath,
    [Parameter(Mandatory = $true)][string]$Arch,
    [Parameter(Mandatory = $true)][string]$Ldflags,
    [Parameter(Mandatory = $true)][string]$Output,
    [Parameter(Mandatory = $true)][string]$Target
)

$ErrorActionPreference = "Stop"

$env:PKG_CONFIG_PATH = (Resolve-Path -LiteralPath $PkgConfigPath).Path
$env:GOOS = "windows"
$env:GOARCH = $Arch
$env:CGO_ENABLED = "1"

go build -tags pdfium -ldflags $Ldflags -o $Output $Target
