#Requires -Version 7.0
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'release-gate.psm1') -Force
$tag = Get-ReleaseTag $env:GITHUB_EVENT_NAME $env:GITHUB_REF
$publish = -not [string]::IsNullOrEmpty($tag)
$reason = if ($publish) { "Maintainer-created release tag $tag" } else { 'Build only; publishing requires a stable tag push' }
$value = $publish.ToString().ToLowerInvariant()
"publish=$value" >> $env:GITHUB_OUTPUT
"tag=$tag" >> $env:GITHUB_OUTPUT
Write-Host "Release eligible: $value. $reason."
"### Release decision`n- Publish: $value`n- Reason: $reason" >> $env:GITHUB_STEP_SUMMARY
