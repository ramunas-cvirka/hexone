# Copyright 2026 Ramunas Cvirka
# SPDX-License-Identifier: Apache-2.0

param(
    [Parameter(Mandatory = $true)][string]$Target,
    [Parameter(Mandatory = $true)][string]$Destination
)

$ErrorActionPreference = "Stop"

switch ($Target) {
    "windows-amd64" {
        $asset = "pdfium-win-x64.tgz"
        $importName = "pdfium.dll.lib"
        $runtimeName = "pdfium.dll"
        $libs = '-L${libdir} -l:pdfium.dll.lib'
    }
    default {
        throw "unsupported pdfium target: $Target"
    }
}

$tmp = "$Destination.tmp"
$archive = Join-Path $tmp $asset
$extract = Join-Path $tmp "extract"

if (Test-Path -LiteralPath $tmp) {
    Remove-Item -LiteralPath $tmp -Recurse -Force
}
if (Test-Path -LiteralPath $Destination) {
    Remove-Item -LiteralPath $Destination -Recurse -Force
}

New-Item -ItemType Directory -Force -Path $extract | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $Destination "include") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $Destination "lib") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $Destination "bin") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path (Join-Path $Destination "lib") "pkgconfig") | Out-Null

$url = "https://github.com/bblanchon/pdfium-binaries/releases/latest/download/$asset"
Invoke-WebRequest -Uri $url -OutFile $archive
tar -xzf $archive -C $extract

$header = Get-ChildItem -Path $extract -Recurse -File -Filter "fpdfview.h" | Select-Object -First 1
if (-not $header) {
    throw "failed to locate fpdfview.h in $asset"
}

$import = Get-ChildItem -Path $extract -Recurse -File -Filter $importName | Select-Object -First 1
if (-not $import) {
    throw "failed to locate $importName in $asset"
}

$runtime = Get-ChildItem -Path $extract -Recurse -File -Filter $runtimeName | Select-Object -First 1
if (-not $runtime) {
    throw "failed to locate $runtimeName in $asset"
}

Copy-Item -Path (Join-Path $header.Directory.FullName "*") -Destination (Join-Path $Destination "include") -Recurse -Force
Copy-Item -LiteralPath $import.FullName -Destination (Join-Path (Join-Path $Destination "lib") $import.Name) -Force
if ($runtimeName -ne $importName) {
    Copy-Item -LiteralPath $runtime.FullName -Destination (Join-Path (Join-Path $Destination "bin") $runtime.Name) -Force
}

$prefix = (Resolve-Path -LiteralPath $Destination).Path.Replace('\', '/')
$pcPath = Join-Path (Join-Path $Destination "lib") "pkgconfig\pdfium.pc"
$pc = @"
prefix=$prefix
libdir=`${prefix}/lib
includedir=`${prefix}/include

Name: PDFium
Description: PDFium
Version: latest
Libs: $libs
Cflags: -I`${includedir}
"@
[System.IO.File]::WriteAllText($pcPath, $pc)

Remove-Item -LiteralPath $tmp -Recurse -Force
