[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$')]
    [string] $Version,

    [string] $BinaryDirectory = "dist",
    [string] $OutputDirectory = "dist-store",
    [string] $Scanner = "",
    [string] $ScanReportDirectory = ".cache/vulnerability-scan"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repositoryRoot = $PSScriptRoot
$binaryRoot = if ([IO.Path]::IsPathRooted($BinaryDirectory)) { $BinaryDirectory } else { Join-Path $repositoryRoot $BinaryDirectory }
$outputRoot = if ([IO.Path]::IsPathRooted($OutputDirectory)) { $OutputDirectory } else { Join-Path $repositoryRoot $OutputDirectory }
$parts = $Version.Split('.')
if ([uint64]$parts[0] -lt 1 -or @($parts | Where-Object { [uint64]$_ -gt 65535 }).Count -ne 0) {
    throw "Microsoft Store versions require major >= 1 and each component <= 65535"
}
# The fourth component is reserved by Microsoft Store.
$packageVersion = "$Version.0"
$bundleName = "skopeo-win-$packageVersion.msixbundle"
$null = New-Item -ItemType Directory -Force -Path $outputRoot
if (Test-Path -LiteralPath (Join-Path $outputRoot $bundleName)) {
    throw "Output already exists; use a new output directory or version: $bundleName"
}

# Check every CLI input before downloading the SDK or creating any package.
# This gate also applies when Actions repackages a historical release.
if ($Scanner -ne "") {
    $scannerPath = (Get-Command $Scanner -ErrorAction Stop).Source
    $reportRoot = if ([IO.Path]::IsPathRooted($ScanReportDirectory)) { $ScanReportDirectory } else { Join-Path $repositoryRoot $ScanReportDirectory }
    $null = New-Item -ItemType Directory -Force -Path $reportRoot
    foreach ($arch in @("amd64", "arm64")) {
        $binary = Join-Path $binaryRoot "skopeo-windows-$arch.exe"
        & $scannerPath -mode binary -scan package $binary 2>&1 |
            Tee-Object -FilePath (Join-Path $reportRoot "store-cli-$arch.txt")
        if ($LASTEXITCODE -ne 0) { throw "Store CLI vulnerability scan failed for $arch" }
    }
}

$stagingRoot = Join-Path $repositoryRoot (".cache/store-packaging/" + [guid]::NewGuid().ToString("N"))
$packages = Join-Path $stagingRoot "packages"
$null = New-Item -ItemType Directory -Force -Path $packages
$template = Get-Content -Raw -LiteralPath (Join-Path $repositoryRoot "packaging/store/AppxManifest.xml.in")
$makeAppx = & (Join-Path $repositoryRoot "hack/get-store-sdk.ps1")
$go = Get-Command go -ErrorAction Stop
$utf8 = New-Object System.Text.UTF8Encoding($false)

foreach ($arch in @("amd64", "arm64")) {
    $name = "skopeo-windows-$arch.exe"
    $binary = Join-Path $binaryRoot $name
    $checksum = (Get-Content -Raw -LiteralPath "$binary.sha256").Trim() -split '\s+'
    $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $binary).Hash.ToLowerInvariant()
    if ($checksum.Count -ne 2 -or $checksum[0] -cne $actualHash -or $checksum[1] -cne $name) {
        throw "Invalid release checksum for $name"
    }
    & $go.Source run (Join-Path $repositoryRoot "hack/verify-windows-binary.go") $binary $arch
    if ($LASTEXITCODE -ne 0) { throw "Invalid Windows release executable: $name" }

    $packageArch = if ($arch -eq "amd64") { "x64" } else { "arm64" }
    $layout = Join-Path $stagingRoot $packageArch
    $null = New-Item -ItemType Directory -Force -Path $layout
    Copy-Item -LiteralPath $binary -Destination (Join-Path $layout "skopeo.exe")
    $launcher = Join-Path $layout "store-launcher.exe"
    $savedCgo = $env:CGO_ENABLED
    $savedGoos = $env:GOOS
    $savedGoarch = $env:GOARCH
    Push-Location -LiteralPath $repositoryRoot
    try {
        $env:CGO_ENABLED = "0"
        $env:GOOS = "windows"
        $env:GOARCH = $arch
        & $go.Source build -buildvcs=false -trimpath `
            -tags "containers_image_openpgp exclude_graphdriver_btrfs" `
            -ldflags "-buildid= -X go.podman.io/skopeo/version.Version=$Version" `
            -o $launcher ./cmd/store-launcher
        if ($LASTEXITCODE -ne 0) { throw "Store launcher build failed for $arch" }
    } finally {
        Pop-Location
        $env:CGO_ENABLED = $savedCgo
        $env:GOOS = $savedGoos
        $env:GOARCH = $savedGoarch
    }
    & $go.Source run (Join-Path $repositoryRoot "hack/verify-windows-binary.go") $launcher $arch
    if ($LASTEXITCODE -ne 0) { throw "Invalid Windows Store launcher: $arch" }
    if ($Scanner -ne "") {
        $scannerPath = (Get-Command $Scanner -ErrorAction Stop).Source
        $reportRoot = if ([IO.Path]::IsPathRooted($ScanReportDirectory)) { $ScanReportDirectory } else { Join-Path $repositoryRoot $ScanReportDirectory }
        $null = New-Item -ItemType Directory -Force -Path $reportRoot
        & $scannerPath -mode binary -scan package $launcher 2>&1 |
            Tee-Object -FilePath (Join-Path $reportRoot "store-launcher-$arch.txt")
        if ($LASTEXITCODE -ne 0) { throw "Store launcher vulnerability scan failed for $arch" }
    }
    foreach ($document in @("LICENSE", "WINDOWS.md")) {
        Copy-Item -LiteralPath (Join-Path $repositoryRoot $document) -Destination $layout
    }
    Copy-Item -LiteralPath (Join-Path $repositoryRoot "packaging/store/Assets") -Destination $layout -Recurse
    $manifest = $template.Replace("@VERSION@", $packageVersion).Replace("@ARCHITECTURE@", $packageArch)
    [IO.File]::WriteAllText((Join-Path $layout "AppxManifest.xml"), $manifest, $utf8)
    $packagePath = Join-Path $packages "skopeo-win-$packageVersion-$packageArch.msix"
    # Keep schema and semantic validation enabled. Never pass /nv.
    & $makeAppx pack /d $layout /p $packagePath /no
    if ($LASTEXITCODE -ne 0) { throw "MSIX packaging failed for $packageArch" }
}

$bundlePath = Join-Path $outputRoot $bundleName
& $makeAppx bundle /d $packages /p $bundlePath /bv $packageVersion /no
if ($LASTEXITCODE -ne 0) { throw "MSIX bundling failed" }
$hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $bundlePath).Hash.ToLowerInvariant()
[IO.File]::WriteAllText("$bundlePath.sha256", "$hash  $bundleName`n", $utf8)
Write-Host "Unsigned Microsoft Store submission bundle: $bundlePath"
Write-Host "Microsoft Store signs the bundle after certification; this is not a signed sideload installer."
