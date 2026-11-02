[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repositoryRoot = Split-Path -Parent $PSScriptRoot

# Microsoft-owned NuGet package. Pin the archive, not a mutable latest URL.
$sdkVersion = "10.0.26100.9457"
$archiveHash = "4c2e0d85f3490e7f7c065c35b4c47fe913b1bc9627e9711ae349b2cdc7edc791"
$cache = Join-Path $repositoryRoot ".cache/store-sdk/$sdkVersion"
$null = New-Item -ItemType Directory -Force -Path $cache
$archive = Join-Path $cache "buildtools.zip"
if (-not (Test-Path -LiteralPath $archive)) {
    $url = "https://api.nuget.org/v3-flatcontainer/microsoft.windows.sdk.buildtools/$sdkVersion/microsoft.windows.sdk.buildtools.$sdkVersion.nupkg"
    Write-Host "Downloading Microsoft Windows SDK BuildTools $sdkVersion"
    Invoke-WebRequest -Uri $url -OutFile $archive
}
if ((Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash.ToLowerInvariant() -ne $archiveHash) {
    throw "Windows SDK archive checksum mismatch: $archive"
}
Expand-Archive -LiteralPath $archive -DestinationPath (Join-Path $cache "tools") -Force
$hostArchitecture = [Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
if ($hostArchitecture -notin @("x64", "arm64", "x86")) {
    throw "Unsupported SDK host architecture: $hostArchitecture"
}
$makeAppx = Join-Path $cache "tools/bin/10.0.26100.0/$hostArchitecture/makeappx.exe"
if (-not (Test-Path -LiteralPath $makeAppx -PathType Leaf)) {
    throw "MakeAppx is missing from the pinned Windows SDK"
}
$makeAppx
