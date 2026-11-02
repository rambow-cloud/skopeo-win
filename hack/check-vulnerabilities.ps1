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
Push-Location -LiteralPath (Split-Path -Parent $PSScriptRoot)
try {
    $reportDirectory = Join-Path (Get-Location) ".cache/vulnerability-scan"
    $null = New-Item -ItemType Directory -Force -Path $reportDirectory
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"

    & $scannerPath -tags "containers_image_openpgp,exclude_graphdriver_btrfs" ./cmd/skopeo ./cmd/store-launcher 2>&1 |
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
        & $scannerPath -mode binary -scan package $binary 2>&1 |
            Tee-Object -FilePath (Join-Path $reportDirectory "binary-$arch.txt")
        if ($LASTEXITCODE -ne 0) {
            throw "Binary vulnerability scan failed for $arch"
        }
    }
} finally {
    $env:CGO_ENABLED = $oldCgo
    $env:GOOS = $oldGoos
    $env:GOARCH = $oldGoarch
    Pop-Location
}
