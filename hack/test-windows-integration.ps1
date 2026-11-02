[CmdletBinding()]
param(
    [string] $Binary = "dist/skopeo-windows-amd64.exe",
    [string] $ExpectedVersion = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$binaryInput = if ([IO.Path]::IsPathRooted($Binary)) { $Binary } else { Join-Path $repositoryRoot $Binary }
$binaryPath = (Resolve-Path -LiteralPath $binaryInput).Path
$reportDirectory = Join-Path $repositoryRoot ".cache/integration-tests"
$null = New-Item -ItemType Directory -Force -Path $reportDirectory
$savedBinary = $env:SKOPEO_BINARY
$savedVersion = $env:SKOPEO_EXPECTED_VERSION
$savedCgo = $env:CGO_ENABLED
Push-Location -LiteralPath (Join-Path $repositoryRoot "tests/windows")
try {
    $env:SKOPEO_BINARY = $binaryPath
    $env:SKOPEO_EXPECTED_VERSION = $ExpectedVersion
    $env:CGO_ENABLED = "0"
    go test -mod=readonly -count=1 -timeout=5m -json ./... 2>&1 |
        Tee-Object -FilePath (Join-Path $reportDirectory "windows-amd64.jsonl")
    if ($LASTEXITCODE -ne 0) {
        throw "Windows executable integration tests failed. See .cache/integration-tests/windows-amd64.jsonl"
    }
}
finally {
    Pop-Location
    $env:SKOPEO_BINARY = $savedBinary
    $env:SKOPEO_EXPECTED_VERSION = $savedVersion
    $env:CGO_ENABLED = $savedCgo
}
