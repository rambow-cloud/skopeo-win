[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$PackageDirectory,
    [switch]$Install
)
$ErrorActionPreference = 'Stop'
$toolsDir = Join-Path $PackageDirectory 'package/tools'
$installer = Join-Path $toolsDir 'chocolateyInstall.ps1'
$release = Get-Content -LiteralPath (Join-Path $toolsDir 'release.json') -Raw | ConvertFrom-Json
$downloads = [System.Collections.Generic.List[object]]::new()
function Get-ChocolateyWebFile {
    param($PackageName, $FileFullPath, $Url, $Checksum, $ChecksumType)
    if ($PackageName -cne 'skopeo-win' -or $ChecksumType -cne 'sha256' -or
        (Split-Path -Leaf $FileFullPath) -cne 'skopeo.exe') { throw 'Invalid helper invocation.' }
    $downloads.Add(@{ url = $Url; sha256 = $Checksum })
}
$environmentNames = @('PROCESSOR_ARCHITECTURE', 'PROCESSOR_ARCHITEW6432', 'chocolateyForceX86')
$savedEnvironment = @{}
foreach ($name in $environmentNames) { $savedEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process') }
try {
    foreach ($case in @(
        @{ process = 'AMD64'; native = ''; expected = 'x64' },
        @{ process = 'ARM64'; native = ''; expected = 'arm64' },
        @{ process = 'x86'; native = 'AMD64'; expected = 'x64' },
        @{ process = 'AMD64'; native = 'ARM64'; expected = 'arm64' }
    )) {
        $env:PROCESSOR_ARCHITECTURE = $case.process
        $env:PROCESSOR_ARCHITEW6432 = $case.native
        $env:chocolateyForceX86 = ''
        $downloads.Clear()
        & $installer
        $expected = $release.($case.expected)
        if ($downloads.Count -ne 1 -or $downloads[0].url -cne $expected.url -or
            $downloads[0].sha256 -cne $expected.sha256) { throw 'Incorrect native architecture download.' }
    }
    foreach ($case in @(
        @{ process = 'x86'; force = ''; message = 'requires x64 or ARM64 Windows' },
        @{ process = 'AMD64'; force = 'true'; message = 'does not support forced 32-bit installation' }
    )) {
        $env:PROCESSOR_ARCHITECTURE = $case.process
        $env:PROCESSOR_ARCHITEW6432 = ''
        $env:chocolateyForceX86 = $case.force
        $downloads.Clear()
        $rejected = $false
        try { & $installer } catch {
            if ($_.Exception.Message.Contains($case.message)) { $rejected = $true } else { throw }
        }
        if (-not $rejected -or $downloads.Count -ne 0) { throw 'Unsupported architecture was not rejected before download.' }
    }
} finally {
    foreach ($name in $environmentNames) { [Environment]::SetEnvironmentVariable($name, $savedEnvironment[$name], 'Process') }
}
Write-Output 'PASS: four native/emulated architecture selections and two unsupported-architecture rejections.'
if (-not $Install) { return }
if ($env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted') {
    throw 'Real package installation tests require a disposable GitHub-hosted runner.'
}
$shim = Join-Path $env:ChocolateyInstall 'bin/skopeo.exe'
if (Test-Path -LiteralPath $shim) { throw 'An existing skopeo shim would be overwritten.' }
& choco install skopeo-win --version $release.version --source $PackageDirectory --yes --no-progress --limit-output
if ($LASTEXITCODE -ne 0) { throw 'Chocolatey installation failed.' }
try {
    if (-not (Test-Path -LiteralPath $shim)) { throw 'The skopeo shim was not created.' }
    $actualVersion = & $shim --version
    if ($LASTEXITCODE -ne 0 -or ($actualVersion -join "`n").Trim() -cne "skopeo version $($release.version)") {
        throw 'Installed shim version check failed.'
    }
    $stderrFile = Join-Path $PackageDirectory 'no-args.stderr.txt'
    $help = & $shim 2> $stderrFile
    if ($LASTEXITCODE -ne 0 -or ($help -join "`n") -notmatch 'Usage:' -or
        (Get-Item -LiteralPath $stderrFile).Length -ne 0) { throw 'Zero-argument installed-shim check failed.' }
    Write-Output 'PASS: real Chocolatey installation, skopeo shim, version and zero-argument help.'
} finally {
    & choco uninstall skopeo-win --yes --no-progress --limit-output
    if ($LASTEXITCODE -ne 0) { throw 'Chocolatey uninstall failed.' }
}
if (Test-Path -LiteralPath $shim) { throw 'Chocolatey did not remove the skopeo shim.' }
if (Test-Path -LiteralPath (Join-Path $env:ChocolateyInstall 'lib/skopeo-win')) { throw 'Package files remain after uninstall.' }
Write-Output 'PASS: real Chocolatey uninstall removed package files and shim.'
