#Requires -Version 7.0
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'release-gate.psm1') -Force
$jobs = @()
if ($env:STORE_EVENT -ceq 'workflow_run') {
    if ($env:SOURCE_RUN_ID -notmatch '^[1-9]\d*$') { throw 'Invalid source run ID' }
    $response = gh api "repos/$env:GITHUB_REPOSITORY/actions/runs/$env:SOURCE_RUN_ID/jobs?per_page=100" | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0) { throw 'Cannot read source release job status' }
    $jobs = @($response.jobs)
}
$submit = Test-AutomaticStoreSubmission $env:STORE_EVENT $jobs
$value = $submit.ToString().ToLowerInvariant()
"submit=$value" >> $env:GITHUB_OUTPUT
if (-not $submit) {
    Write-Host 'Source run did not publish a release; skipping Store submission without accessing Store credentials.'
}
