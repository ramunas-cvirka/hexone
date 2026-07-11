# Copyright 2026 Ramunas Cvirka
# SPDX-License-Identifier: Apache-2.0

param(
    [Parameter(Mandatory = $true)][string]$InputPackage,
    [Parameter(Mandatory = $true)][string]$OutputPackage,
    [Parameter(Mandatory = $true)][string]$Publisher
)

$ErrorActionPreference = "Stop"
$friendlyName = "Hexone MSIX development"

if (-not (Get-PSDrive -Name Cert -ErrorAction SilentlyContinue)) {
    try {
        $certificateProvider = Get-PSProvider Certificate -ErrorAction Stop
        New-PSDrive `
            -Name Cert `
            -PSProvider $certificateProvider.Name `
            -Root "\" `
            -Scope Global | Out-Null
    } catch {
        throw "The Windows PowerShell certificate provider could not be initialized (PowerShell $($PSVersionTable.PSVersion), PSHOME $PSHOME): $($_.Exception.Message)"
    }
}

if (-not (Get-PSDrive -Name Cert -ErrorAction SilentlyContinue)) {
    throw "The Windows PowerShell certificate drive could not be initialized (PowerShell $($PSVersionTable.PSVersion), PSHOME $PSHOME)."
}

function Resolve-SignTool {
    $sdkRoot = Join-Path ${env:ProgramFiles(x86)} "Windows Kits\10"

    # The App Certification Kit ships the SignTool variant paired with its
    # MakeAppx tooling. Some SDK-bin SignTool builds do not recognize MSIX.
    $certificationKit = Join-Path $sdkRoot "App Certification Kit\signtool.exe"
    if (Test-Path -LiteralPath $certificationKit) {
        return $certificationKit
    }

    $versioned = Get-ChildItem (Join-Path $sdkRoot "bin\*\x64\signtool.exe") -ErrorAction SilentlyContinue |
        Sort-Object { [version]$_.Directory.Parent.Name } -Descending |
        Select-Object -First 1 -ExpandProperty FullName
    if ($versioned) {
        return $versioned
    }
    throw "SignTool.exe was not found. Install the Windows SDK."
}

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

$inputPath = (Resolve-Path -LiteralPath $InputPackage).Path
$outputPath = [System.IO.Path]::GetFullPath($OutputPackage)
$outputDir = Split-Path -Parent $outputPath
New-Item -ItemType Directory -Force -Path $outputDir | Out-Null
Copy-Item -LiteralPath $inputPath -Destination $outputPath -Force

$cert = Get-ChildItem Cert:\CurrentUser\My |
    Where-Object {
        $_.Subject -eq $Publisher -and
        $_.FriendlyName -eq $friendlyName -and
        $_.HasPrivateKey -and
        $_.NotAfter -gt (Get-Date)
    } |
    Sort-Object NotAfter -Descending |
    Select-Object -First 1

if (-not $cert) {
    $cert = New-SelfSignedCertificate `
        -Type Custom `
        -KeyUsage DigitalSignature `
        -Subject $Publisher `
        -CertStoreLocation "Cert:\CurrentUser\My" `
        -TextExtension @(
            "2.5.29.37={text}1.3.6.1.5.5.7.3.3",
            "2.5.29.19={text}"
        ) `
        -FriendlyName $friendlyName `
        -NotAfter (Get-Date).AddYears(2)
}

$cerPath = Join-Path $outputDir "hexone-msix-dev.cer"
Export-Certificate -Cert $cert -FilePath $cerPath -Force | Out-Null

$trusted = Get-ChildItem Cert:\LocalMachine\TrustedPeople |
    Where-Object Thumbprint -eq $cert.Thumbprint |
    Select-Object -First 1
if (-not $trusted) {
    if (Test-Administrator) {
        Import-Certificate -FilePath $cerPath -CertStoreLocation "Cert:\LocalMachine\TrustedPeople" | Out-Null
    } else {
        Write-Host "Trusting the Hexone development certificate requires one UAC confirmation."
        $certutil = Join-Path $env:SystemRoot "System32\certutil.exe"
        $process = Start-Process `
            -FilePath $certutil `
            -ArgumentList @("-addstore", "TrustedPeople", "`"$cerPath`"") `
            -Verb RunAs `
            -Wait `
            -PassThru
        if ($process.ExitCode -ne 0) {
            throw "The development certificate was not added to LocalMachine\TrustedPeople."
        }
    }
}

$signtool = Resolve-SignTool
$signOutput = & $signtool sign /fd SHA256 /sha1 $cert.Thumbprint /s My $outputPath 2>&1
$signExitCode = $LASTEXITCODE
if ($signExitCode -ne 0) {
    $signOutput | ForEach-Object { Write-Host $_ }
    throw "SignTool.exe failed to sign $outputPath"
}

$verifyOutput = & $signtool verify /pa $outputPath 2>&1
$verifyExitCode = $LASTEXITCODE
if ($verifyExitCode -ne 0) {
    $verifyOutput | ForEach-Object { Write-Host $_ }
    throw "SignTool.exe failed to verify $outputPath"
}

Write-Host "Created trusted local-testing MSIX: $outputPath"
