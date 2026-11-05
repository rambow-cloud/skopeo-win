#Requires -Version 7.0
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'release-gate.psm1') -Force
foreach ($case in @(
    @{paths=@('.github/workflows/check_cirrus_cron.yml','docs/release-notes.md');want=$false},
    @{paths=@('.github/scripts/release-gate.psm1','README.md','WINDOWS.md');want=$false},
    @{paths=@();want=$false},
    @{paths=@('cmd/skopeo/copy.go','docs/release-notes.md');want=$true},
    @{paths=@('go.mod');want=$true},
    @{paths=@('vendor/example/file.go');want=$true},
    @{paths=@('build-store.ps1');want=$true},
    @{paths=@('packaging/store/Assets/Logo.png');want=$true},
    @{paths=@('LICENSE');want=$true},
    @{paths=@('.github/../cmd/skopeo/main.go');want=$true},
    @{paths=@('"docs/quoted-path"');want=$true}
)) {
    if ((Test-ReleaseNeeded $case.paths) -ne $case.want) { throw 'Release path classification failed' }
}
if (Test-AutomaticStoreSubmission 'workflow_run' @(@{name='release';conclusion='skipped'})) { throw 'Maintenance triggered Store submission' }
if (-not (Test-AutomaticStoreSubmission 'workflow_run' @(@{name='release';conclusion='success'}))) { throw 'Published source was skipped' }
if (-not (Test-AutomaticStoreSubmission 'workflow_dispatch' @())) { throw 'Explicit Store dispatch was skipped' }
foreach ($case in @(
    @{jobs=@()},
    @{jobs=@(@{name='release';conclusion='failure'})},
    @{jobs=@(@{name='release';conclusion='success'},@{name='release';conclusion='skipped'})}
)) {
    $rejected = $false
    try { $null = Test-AutomaticStoreSubmission 'workflow_run' $case.jobs } catch { $rejected = $true }
    if (-not $rejected) { throw 'Ambiguous source release status was accepted' }
}
$root = Join-Path (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent) ('.cache/release-gate-tests/' + [guid]::NewGuid().ToString('N'))
$null = New-Item -ItemType Directory -Force -Path $root
Push-Location -LiteralPath $root
try {
    git init --quiet
    if ($LASTEXITCODE -ne 0) { throw 'Cannot initialize fixture repository' }
    git config user.name 'Release gate fixture'
    git config user.email 'fixture@example.invalid'
    git config commit.gpgsign false
    $null = New-Item -ItemType Directory -Force cmd,.github/workflows,docs
    Set-Content cmd/app.go 'package main'
    git add .
    git commit --quiet -m baseline
    git tag v1.0.13
    Set-Content .github/workflows/ci.yml 'maintenance'
    git add .
    git commit --quiet -m maintenance
    if (Test-ReleaseNeededSince 'v1.0.13') { throw 'CI-only commit triggered publication' }
    Add-Content cmd/app.go '// production change'
    git add .
    git commit --quiet -m feature
    Add-Content .github/workflows/ci.yml 'later CI fix'
    git add .
    git commit --quiet -m follow-up
    if (-not (Test-ReleaseNeededSince 'v1.0.13')) { throw 'Unreleased source was hidden by a later CI-only commit' }
    git mv cmd/app.go docs/app.md
    git commit --quiet -m rename
    if (-not (Test-ReleaseNeededSince 'v1.0.13')) { throw 'A production deletion was hidden by rename detection' }
} finally { Pop-Location }
Write-Host 'Verified maintenance/source classification, published-source comparison, rename/deletion handling and Store submission gating.'
