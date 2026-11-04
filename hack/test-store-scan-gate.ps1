[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$testRoot = Join-Path $repositoryRoot (".cache/store-scan-gate/" + [guid]::NewGuid().ToString("N"))
$null = New-Item -ItemType Directory -Force -Path $testRoot
$fakeScanner = Join-Path $testRoot "scanner.cmd"
$savedFailure = $env:STORE_SCAN_TEST_FAIL
$savedLog = $env:STORE_SCAN_TEST_LOG
$env:STORE_SCAN_TEST_LOG = Join-Path $testRoot "calls.txt"
try {
    @'
@echo off
echo %*>>"%STORE_SCAN_TEST_LOG%"
echo %*|findstr /c:"%STORE_SCAN_TEST_FAIL%" >nul
if not errorlevel 1 exit /b 3
exit /b 0
'@ | Set-Content -LiteralPath $fakeScanner -Encoding ascii
    foreach ($arch in @("amd64", "arm64")) {
        $env:STORE_SCAN_TEST_FAIL = "skopeo-windows-$arch.exe"
        [IO.File]::WriteAllText($env:STORE_SCAN_TEST_LOG, "")
        $outputDirectory = Join-Path $testRoot $arch
        $caught = $null
        try {
            # Missing executables intentionally prove rejection happens before
            # checksum verification, SDK acquisition, or MakeAppx execution.
            & (Join-Path $repositoryRoot "build-store.ps1") -Version 1.0.8 -BinaryDirectory $testRoot -OutputDirectory $outputDirectory -Scanner $fakeScanner -ScanReportDirectory (Join-Path $testRoot "reports")
        } catch { $caught = $_.Exception.Message }
        if ($caught -ne "Store CLI vulnerability scan failed for $arch") {
            throw "Expected CLI scan rejection for $arch; got: $caught"
        }
        $calls = @(Get-Content -LiteralPath $env:STORE_SCAN_TEST_LOG)
        $expected = if ($arch -eq "amd64") { 1 } else { 2 }
        if ($calls.Count -ne $expected -or $calls[-1] -notlike "*-mode binary -scan package*skopeo-windows-$arch.exe*") {
            throw "Wrong scanner order or package scan arguments for $arch"
        }
        if (@(Get-ChildItem -LiteralPath $outputDirectory -File).Count -ne 0) {
            throw "Scan rejection left a packaged artifact"
        }
        Write-Host "Verified: $arch CLI scan failure blocks Store packaging."
    }
} finally {
    $env:STORE_SCAN_TEST_FAIL = $savedFailure
    $env:STORE_SCAN_TEST_LOG = $savedLog
}
$global:LASTEXITCODE = 0
