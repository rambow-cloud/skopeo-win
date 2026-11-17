[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^v[1-9][0-9]*\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$')]
    [string]$Tag,
    [string]$OutputDirectory = '.cache/chocolatey'
)
$ErrorActionPreference = 'Stop'
$repository = 'rambow-cloud/skopeo-win'
$version = $Tag.Substring(1)
$root = Split-Path -Parent $PSScriptRoot

function Read-GitHubJson([string]$Path) {
    $response = & gh api $Path
    if ($LASTEXITCODE -ne 0) { throw "Cannot read GitHub resource: $Path" }
    return ($response -join "`n") | ConvertFrom-Json
}

$release = Read-GitHubJson "repos/$repository/releases/tags/$Tag"
if ($release.draft -or $release.prerelease -or $release.tag_name -cne $Tag -or -not $release.published_at) {
    throw 'Only a published stable release can be packaged.'
}
$commit = Read-GitHubJson "repos/$repository/commits/$Tag"
$comparison = Read-GitHubJson "repos/$repository/compare/$($commit.sha)...main"
if ($comparison.merge_base_commit.sha -cne $commit.sha) {
    throw 'The release tag must identify a commit in main history.'
}

$destination = Join-Path ([IO.Path]::GetFullPath($OutputDirectory)) $version
if (Test-Path -LiteralPath $destination) { throw "Output already exists: $destination" }
$packageDir = Join-Path $destination 'package'
$toolsDir = Join-Path $packageDir 'tools'
$verifiedDir = Join-Path $destination 'verified-downloads'
New-Item -ItemType Directory -Path $toolsDir, $verifiedDir | Out-Null
$metadata = [ordered]@{ version = $version; tag = $Tag; commit = $commit.sha }
foreach ($entry in @(
    @{ architecture = 'x64'; name = 'skopeo-windows-amd64.exe'; machine = 0x8664 },
    @{ architecture = 'arm64'; name = 'skopeo-windows-arm64.exe'; machine = 0xAA64 }
)) {
    $assets = @($release.assets | Where-Object { $_.name -ceq $entry.name })
    if ($assets.Count -ne 1) { throw "Expected one release asset: $($entry.name)" }
    $asset = $assets[0]
    $url = "https://github.com/$repository/releases/download/$Tag/$($entry.name)"
    if ($asset.browser_download_url -cne $url -or $asset.digest -cnotmatch '^sha256:[a-f0-9]{64}$') {
        throw "Unexpected release asset URL or digest: $($entry.name)"
    }
    & gh release download $Tag --repo $repository --pattern $entry.name --dir $verifiedDir
    if ($LASTEXITCODE -ne 0) { throw "Cannot download $($entry.name)" }
    $path = Join-Path $verifiedDir $entry.name
    $hash = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ("sha256:$hash" -cne $asset.digest) { throw "Release asset checksum mismatch: $($entry.name)" }
    $binary = [IO.File]::ReadAllBytes($path)
    if ($binary.Length -lt 64 -or [BitConverter]::ToUInt16($binary, 0) -ne 0x5A4D) { throw 'Invalid DOS header' }
    $peOffset = [BitConverter]::ToInt32($binary, 60)
    if ($peOffset -lt 64 -or $peOffset -gt ($binary.Length - 6) -or
        [BitConverter]::ToUInt32($binary, $peOffset) -ne 0x00004550 -or
        [BitConverter]::ToUInt16($binary, $peOffset + 4) -ne $entry.machine) {
        throw "Release asset architecture mismatch: $($entry.name)"
    }
    $metadata[$entry.architecture] = [ordered]@{ url = $url; sha256 = $hash }
}
$metadata | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $toolsDir 'release.json') -Encoding utf8
Copy-Item -LiteralPath (Join-Path $root 'packaging/chocolatey/tools/chocolateyInstall.ps1') -Destination $toolsDir
$template = Get-Content -LiteralPath (Join-Path $root 'packaging/chocolatey/skopeo-win.nuspec.in') -Raw
$template.Replace('@VERSION@', $version).Replace('@TAG@', $Tag) | Set-Content -LiteralPath (Join-Path $packageDir 'skopeo-win.nuspec') -Encoding utf8
& choco pack (Join-Path $packageDir 'skopeo-win.nuspec') --output-directory $destination --no-progress
if ($LASTEXITCODE -ne 0) { throw 'Chocolatey pack failed.' }
$package = Join-Path $destination "skopeo-win.$version.nupkg"
if (-not (Test-Path -LiteralPath $package -PathType Leaf)) { throw 'Chocolatey package was not created.' }
$metadata | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $destination 'verification.json') -Encoding utf8
if ($env:GITHUB_OUTPUT) {
    "version=$version" >> $env:GITHUB_OUTPUT
    "package=$package" >> $env:GITHUB_OUTPUT
    "directory=$destination" >> $env:GITHUB_OUTPUT
}
Write-Output "Created and verified: $package"
