# Copyright 2026 Ramunas Cvirka
# SPDX-License-Identifier: Apache-2.0

param(
    [Parameter(Mandatory = $true)][string]$AppDirectory,
    [Parameter(Mandatory = $true)][string]$AssetsDirectory,
    [Parameter(Mandatory = $true)][string]$OutputPackage,
    [Parameter(Mandatory = $true)][string]$IdentityName,
    [Parameter(Mandatory = $true)][string]$Publisher,
    [Parameter(Mandatory = $true)][string]$Version
)

$ErrorActionPreference = "Stop"

function Resolve-WindowsSdkTool([string]$Name) {
    $command = Get-Command $Name -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty Source
    if ($command) {
        return $command
    }

    $sdkRoot = Join-Path ${env:ProgramFiles(x86)} "Windows Kits\10"
    $versioned = Get-ChildItem (Join-Path $sdkRoot "bin\*\x64\$Name") -ErrorAction SilentlyContinue |
        Sort-Object { [version]$_.Directory.Parent.Name } -Descending |
        Select-Object -First 1 -ExpandProperty FullName
    if ($versioned) {
        return $versioned
    }

    if ($Name -ieq "makeappx.exe") {
        $certificationKit = Join-Path $sdkRoot "App Certification Kit\makeappx.exe"
        if (Test-Path -LiteralPath $certificationKit) {
            return $certificationKit
        }
    }

    if ($Name -ieq "makepri.exe") {
        throw "MakePri.exe was not found. Install the Windows SDK 'UWP Managed Apps' component; qualified MSIX icon assets require a resources.pri file."
    }
    throw "$Name was not found. Install the Windows SDK or Windows App Certification Kit."
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

function Invoke-Checked([string]$Tool, [string[]]$Arguments, [string]$FailureMessage) {
    $output = & $Tool @Arguments 2>&1
    if ($LASTEXITCODE -ne 0) {
        $output | ForEach-Object { Write-Host $_ }
        throw $FailureMessage
    }
}

$appPath = (Resolve-Path -LiteralPath $AppDirectory).Path
$assetsPath = (Resolve-Path -LiteralPath $AssetsDirectory).Path
$outputPath = [System.IO.Path]::GetFullPath($OutputPackage)
$outputDir = Split-Path -Parent $outputPath
New-Item -ItemType Directory -Force -Path $outputDir | Out-Null

$stageDir = Join-Path $outputDir "msix-stage"
$validationDir = Join-Path $outputDir "msix-validation"
$manifestTemplate = Join-Path $PSScriptRoot "AppxManifest.xml.in"
$makepri = Resolve-WindowsSdkTool "makepri.exe"
$makeappx = Resolve-WindowsSdkTool "makeappx.exe"

try {
    Remove-WorkDirectory $stageDir $outputDir
    Remove-WorkDirectory $validationDir $outputDir
    New-Item -ItemType Directory -Path $stageDir | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $stageDir "app") | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $stageDir "Assets") | Out-Null

    Copy-Item -LiteralPath (Join-Path $appPath "hexone.exe") -Destination (Join-Path $stageDir "app\hexone.exe")
    Copy-Item -LiteralPath (Join-Path $appPath "protocols.yaml") -Destination (Join-Path $stageDir "app\protocols.yaml")
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot "..\..\LICENSE") -Destination $stageDir
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot "..\..\NOTICE") -Destination $stageDir
    Copy-Item -Path (Join-Path $assetsPath "*.png") -Destination (Join-Path $stageDir "Assets")

    $utf8NoBom = [System.Text.UTF8Encoding]::new($false, $true)
    $manifest = [System.IO.File]::ReadAllText($manifestTemplate, $utf8NoBom)
    $manifest = $manifest.Replace("@HEXONE_MSIX_IDENTITY_NAME@", [System.Security.SecurityElement]::Escape($IdentityName))
    $manifest = $manifest.Replace("@HEXONE_MSIX_PUBLISHER@", [System.Security.SecurityElement]::Escape($Publisher))
    $manifest = $manifest.Replace("@HEXONE_MSIX_VERSION@", [System.Security.SecurityElement]::Escape($Version))
    $manifestPath = Join-Path $stageDir "AppxManifest.xml"
    [System.IO.File]::WriteAllText($manifestPath, $manifest, $utf8NoBom)

    $priConfig = Join-Path $stageDir "priconfig.xml"
    $priPath = Join-Path $stageDir "resources.pri"
    Invoke-Checked $makepri @("createconfig", "/cf", $priConfig, "/dq", "en-US", "/o") "MakePri.exe failed to create priconfig.xml"
    Invoke-Checked $makepri @("new", "/pr", $stageDir, "/cf", $priConfig, "/mn", $manifestPath, "/of", $priPath, "/o") "MakePri.exe failed to create resources.pri"
    Remove-Item -LiteralPath $priConfig -Force

    Invoke-Checked $makeappx @("pack", "/d", $stageDir, "/p", $outputPath, "/o") "MakeAppx.exe failed to create $outputPath"
    Invoke-Checked $makeappx @("unpack", "/p", $outputPath, "/d", $validationDir, "/o") "MakeAppx.exe validation failed for $outputPath"
    if (-not (Test-Path -LiteralPath (Join-Path $validationDir "resources.pri"))) {
        throw "The validated package does not contain resources.pri"
    }
    Write-Host "Created and validated MSIX with package resource index: $outputPath"
} finally {
    Remove-WorkDirectory $stageDir $outputDir
    Remove-WorkDirectory $validationDir $outputDir
}
