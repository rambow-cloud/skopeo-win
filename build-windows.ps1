[CmdletBinding()]
param(
    [ValidateSet("amd64", "arm64")]
    [string[]] $Architecture = @("amd64"),

    [string] $OutputDirectory = "dist",

    [ValidatePattern('^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$')]
    [string] $Version = "0.0.0-dev",

    [switch] $KeepDebugSymbols = $true
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repositoryRoot = $PSScriptRoot
$outputRoot = if ([System.IO.Path]::IsPathRooted($OutputDirectory)) {
    $OutputDirectory
} else {
    Join-Path $repositoryRoot $OutputDirectory
}

$go = Get-Command go -ErrorAction Stop
$null = New-Item -ItemType Directory -Force -Path $outputRoot
$hostGoos = (& $go.Source env GOOS).Trim()
$hostGoarch = (& $go.Source env GOARCH).Trim()

$oldCgoEnabled = $env:CGO_ENABLED
$oldGoos = $env:GOOS
$oldGoarch = $env:GOARCH

Push-Location -LiteralPath $repositoryRoot
try {
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"

    foreach ($arch in $Architecture) {
        $env:GOOS = "windows"
        $env:GOARCH = $arch
        $outputPath = Join-Path $outputRoot "skopeo-windows-$arch.exe"

        $ldflags = "-buildid= -X go.podman.io/skopeo/version.Version=$Version -X go.podman.io/image/v5/signature/internal/sequoia.sequoiaLibraryDir="
        if (-not $KeepDebugSymbols) {
            $ldflags = "-s -w $ldflags"
        }

        Write-Host "Building $outputPath"
        & $go.Source build `
            -buildvcs=false `
            -trimpath `
            -tags "containers_image_openpgp exclude_graphdriver_btrfs" `
            -ldflags $ldflags `
            -o $outputPath `
            ./cmd/skopeo
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for windows/$arch"
        }

        # The verifier inspects any PE architecture, but must itself run on the
        # host architecture (important while cross-building arm64).
        $env:GOOS = $hostGoos
        $env:GOARCH = $hostGoarch
        & $go.Source run ./hack/verify-windows-binary.go $outputPath $arch
        if ($LASTEXITCODE -ne 0) {
            throw "binary verification failed for windows/$arch"
        }

        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $outputPath).Hash.ToLowerInvariant()
        Set-Content -Encoding ascii -NoNewline -LiteralPath "$outputPath.sha256" -Value "$hash  $([System.IO.Path]::GetFileName($outputPath))`n"
        Write-Host "SHA256 $hash"
    }
} finally {
    $env:CGO_ENABLED = $oldCgoEnabled
    $env:GOOS = $oldGoos
    $env:GOARCH = $oldGoarch
    Pop-Location
}
