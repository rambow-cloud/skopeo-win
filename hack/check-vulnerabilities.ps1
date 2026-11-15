[CmdletBinding()]
param(
    [string] $Scanner = "govulncheck",
    [string] $BinaryDirectory = "dist"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$scannerPath = (Get-Command $Scanner -ErrorAction Stop).Source
$oldCgo = $env:CGO_ENABLED
$oldGoos = $env:GOOS
$oldGoarch = $env:GOARCH
$oldGoFlags = $env:GOFLAGS
Push-Location -LiteralPath (Split-Path -Parent $PSScriptRoot)
try {
    $reportDirectory = Join-Path (Get-Location) ".cache/vulnerability-scan"
    $null = New-Item -ItemType Directory -Force -Path $reportDirectory
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"

    & $scannerPath -show verbose -tags "containers_image_openpgp,exclude_graphdriver_btrfs" ./cmd/skopeo ./cmd/store-launcher 2>&1 |
        Tee-Object -FilePath (Join-Path $reportDirectory "source.txt")
    if ($LASTEXITCODE -ne 0) {
        throw "Source vulnerability scan failed. See .cache/vulnerability-scan/source.txt"
    }

    foreach ($arch in @("amd64", "arm64")) {
        $binary = Join-Path $BinaryDirectory "skopeo-windows-$arch.exe"
        if (-not (Test-Path -LiteralPath $binary -PathType Leaf)) {
            throw "Build the Windows binaries before scanning: $binary is missing"
        }
        # Reject findings in packages present in the binary, including functions
        # the source call graph may not reach. Do not fail for unimported packages
        # merely sharing a module (for example, the removed x/crypto/openpgp).
        & $scannerPath -show verbose -mode binary -scan package $binary 2>&1 |
            Tee-Object -FilePath (Join-Path $reportDirectory "binary-$arch.txt")
        if ($LASTEXITCODE -ne 0) {
            throw "Binary vulnerability scan failed for $arch"
        }
    }
    # Fork module paths are not indexed under upstream advisory identities.
    # Scan canonical upstream sources at the exact reviewed base versions too;
    # this prevents a replacement from hiding a known upstream vulnerability.
    $sources = @(Get-Content -Raw -LiteralPath 'packaging/store/module-sources.json' | ConvertFrom-Json)
    $auditMod = '.cache/vulnerability-scan/upstream.go.mod'
    Copy-Item -LiteralPath go.mod -Destination $auditMod
    Copy-Item -LiteralPath go.sum -Destination '.cache/vulnerability-scan/upstream.go.sum'
    foreach ($source in $sources) {
        go mod edit "-modfile=$auditMod" "-dropreplace=$($source.module)"
        if ($LASTEXITCODE -ne 0) { throw 'Cannot prepare canonical upstream vulnerability audit' }
    }
    $env:GOFLAGS = "-mod=mod -modfile=$auditMod"
    $env:GOARCH = 'amd64'
    & $scannerPath -show verbose -scan package -tags "containers_image_openpgp,exclude_graphdriver_btrfs" ./cmd/skopeo ./cmd/store-launcher 2>&1 |
        Tee-Object -FilePath (Join-Path $reportDirectory 'upstream-source-audit.txt')
    if ($LASTEXITCODE -ne 0) { throw 'Canonical upstream vulnerability audit failed; review the fork baseline' }
} finally {
    $env:CGO_ENABLED = $oldCgo
    $env:GOOS = $oldGoos
    $env:GOARCH = $oldGoarch
    $env:GOFLAGS = $oldGoFlags
    Pop-Location
}
