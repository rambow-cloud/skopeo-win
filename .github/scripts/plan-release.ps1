#Requires -Version 7.0
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'release-gate.psm1') -Force
$publish = $false
$reason = 'Preview build'
if ($env:GITHUB_REF -ceq 'refs/heads/main') {
    if ($env:GITHUB_EVENT_NAME -ceq 'workflow_dispatch') {
        $publish = $true
        $reason = 'Explicit manual main release'
    } elseif ($env:GITHUB_EVENT_NAME -ceq 'push') {
        $raw = gh api "repos/$env:GITHUB_REPOSITORY/releases/latest" 2>$null
        $apiExit = $LASTEXITCODE
        try { $latest = ($raw -join "`n") | ConvertFrom-Json -AsHashtable }
        catch { throw 'Cannot determine the latest published release' }
        if ($apiExit -ne 0 -and $latest['status'] -ne '404') { throw 'Cannot read the latest published release' }
        $tag = if ($apiExit -eq 0) { [string]$latest['tag_name'] } else { '' }
        if ($tag -eq '' -or $tag -ceq 'v1.23.0') {
            $publish = $true
            $reason = 'Initial independent release'
        } else {
            if ($tag -cnotmatch '^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$') { throw 'Unexpected published release tag' }
            git fetch --no-tags --depth=1 origin "refs/tags/${tag}:refs/tags/${tag}"
            if ($LASTEXITCODE -ne 0) { throw 'Cannot fetch the published release source' }
            $publish = Test-ReleaseNeededSince $tag
            $reason = if ($publish) { "Production inputs changed since $tag" } else { "Only automation/documentation changed since $tag" }
        }
    }
}
$value = $publish.ToString().ToLowerInvariant()
"publish=$value" >> $env:GITHUB_OUTPUT
Write-Host "Release eligible: $value. $reason."
"### Release decision`n- Publish: $value`n- Reason: $reason" >> $env:GITHUB_STEP_SUMMARY
