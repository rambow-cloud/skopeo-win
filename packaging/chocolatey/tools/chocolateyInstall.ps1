$ErrorActionPreference = 'Stop'

# PROCESSOR_ARCHITEW6432 identifies the native OS when PowerShell is emulated.
$architecture = $env:PROCESSOR_ARCHITECTURE
if ($env:PROCESSOR_ARCHITEW6432) {
    $architecture = $env:PROCESSOR_ARCHITEW6432
}
if ($env:chocolateyForceX86 -eq 'true') {
    throw 'skopeo-win does not support forced 32-bit installation.'
}
switch ($architecture.ToUpperInvariant()) {
    'AMD64' { $assetName = 'x64' }
    'ARM64' { $assetName = 'arm64' }
    default { throw "skopeo-win requires x64 or ARM64 Windows; detected: $architecture" }
}

$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$release = Get-Content -LiteralPath (Join-Path $toolsDir 'release.json') -Raw | ConvertFrom-Json
$asset = $release.$assetName
# Download directly as skopeo.exe; Chocolatey creates and removes its normal shim.
Get-ChocolateyWebFile -PackageName 'skopeo-win' -FileFullPath (Join-Path $toolsDir 'skopeo.exe') -Url $asset.url -Checksum $asset.sha256 -ChecksumType 'sha256' | Out-Null
