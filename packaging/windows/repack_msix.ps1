# Copyright 2026 Ramunas Cvirka
# SPDX-License-Identifier: Apache-2.0

param(
    [Parameter(Mandatory = $true)][string]$InputPackage,
    [Parameter(Mandatory = $true)][string]$OutputPackage
)

$ErrorActionPreference = "Stop"

function Resolve-MakeAppx {
    $sdkRoot = Join-Path ${env:ProgramFiles(x86)} "Windows Kits\10"
    $versioned = Get-ChildItem (Join-Path $sdkRoot "bin\*\x64\makeappx.exe") -ErrorAction SilentlyContinue |
        Sort-Object { [version]$_.Directory.Parent.Name } -Descending |
        Select-Object -First 1 -ExpandProperty FullName
    if ($versioned) {
        return $versioned
    }

    $certificationKit = Join-Path $sdkRoot "App Certification Kit\makeappx.exe"
    if (Test-Path -LiteralPath $certificationKit) {
        return $certificationKit
    }
    throw "MakeAppx.exe was not found. Install the Windows SDK or Windows App Certification Kit."
}

function Remove-WorkDirectory([string]$Path, [string]$AllowedRoot) {
    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }
    $resolvedPath = (Resolve-Path -LiteralPath $Path).Path
    $resolvedRoot = (Resolve-Path -LiteralPath $AllowedRoot).Path.TrimEnd('\') + '\'
    if (-not $resolvedPath.StartsWith($resolvedRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to remove work directory outside $resolvedRoot`: $resolvedPath"
    }
    Remove-Item -LiteralPath $resolvedPath -Recurse -Force
}

$inputPath = (Resolve-Path -LiteralPath $InputPackage).Path
$outputPath = [System.IO.Path]::GetFullPath($OutputPackage)
$outputDir = Split-Path -Parent $outputPath
New-Item -ItemType Directory -Force -Path $outputDir | Out-Null

$stageDir = Join-Path $outputDir "msix-makeappx-stage"
$validationDir = Join-Path $outputDir "msix-makeappx-validation"
$makeappx = Resolve-MakeAppx

try {
    Remove-WorkDirectory $stageDir $outputDir
    Remove-WorkDirectory $validationDir $outputDir
    New-Item -ItemType Directory -Path $stageDir | Out-Null

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($inputPath, $stageDir)

    @(
        "AppxBlockMap.xml",
        "[Content_Types].xml",
        "AppxSignature.p7x",
        "AppxMetadata"
    ) | ForEach-Object {
        $metadataPath = Join-Path $stageDir $_
        if (Test-Path -LiteralPath $metadataPath) {
            Remove-Item -LiteralPath $metadataPath -Recurse -Force
        }
    }

    $packOutput = & $makeappx pack /d $stageDir /p $outputPath /o 2>&1
    $packExitCode = $LASTEXITCODE
    if ($packExitCode -ne 0) {
        $packOutput | ForEach-Object { Write-Host $_ }
        throw "MakeAppx.exe failed to create $outputPath"
    }

    $validationOutput = & $makeappx unpack /p $outputPath /d $validationDir /o 2>&1
    $validationExitCode = $LASTEXITCODE
    if ($validationExitCode -ne 0) {
        $validationOutput | ForEach-Object { Write-Host $_ }
        throw "MakeAppx.exe validation failed for $outputPath"
    }

    Write-Host "Created and validated MSIX: $outputPath"
    Remove-Item -LiteralPath $inputPath -Force
} finally {
    Remove-WorkDirectory $stageDir $outputDir
    Remove-WorkDirectory $validationDir $outputDir
}
