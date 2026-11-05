#Requires -Version 7.0
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'release-gate.psm1') -Force
foreach ($case in @(
    @{event='push';ref='refs/heads/main';tag=''},
    @{event='pull_request';ref='refs/pull/40/merge';tag=''},
    @{event='workflow_dispatch';ref='refs/heads/main';tag=''},
    @{event='workflow_dispatch';ref='refs/tags/v1.0.15';tag=''},
    @{event='push';ref='refs/tags/v1.0.15';tag='v1.0.15'},
    @{event='push';ref='refs/tags/v2.0.0';tag='v2.0.0'}
)) {
    if ((Get-ReleaseTag $case.event $case.ref) -cne $case.tag) { throw 'Tag-only release gating failed' }
}
foreach ($tag in @('v01.0.15','v0.1.0','v1.0.15-rc.1','v1.23.0','v65536.0.0','v1.0.65536','v1.0.15/extra')) {
    $rejected = $false
    try { $null = Get-ReleaseTag 'push' "refs/tags/$tag" } catch { $rejected = $true }
    if (-not $rejected) { throw "Invalid release tag accepted: $tag" }
}
if (Test-AutomaticStoreSubmission 'workflow_run' @(@{name='release';conclusion='skipped'})) { throw 'Build-only run triggered Store submission' }
if (-not (Test-AutomaticStoreSubmission 'workflow_run' @(@{name='release';conclusion='success'}))) { throw 'Published tag was skipped' }
if (-not (Test-AutomaticStoreSubmission 'workflow_dispatch' @())) { throw 'Explicit Store recovery was skipped' }
foreach ($case in @(
    @{jobs=@()},
    @{jobs=@(@{name='release';conclusion='failure'})},
    @{jobs=@(@{name='release';conclusion='success'},@{name='release';conclusion='skipped'})}
)) {
    $rejected = $false
    try { $null = Test-AutomaticStoreSubmission 'workflow_run' $case.jobs } catch { $rejected = $true }
    if (-not $rejected) { throw 'Ambiguous source release status was accepted' }
}
$sourceSHA = '1' * 40
foreach ($case in @('valid tag','main advanced','legacy main recovery','foreign repository','PR run','failed run','wrong workflow','draft','prerelease','wrong tag','wrong owner','moved tag','outside main')) {
    $run = @{id=123;path='.github/workflows/windows.yml';head_repository=@{full_name='owner/repo'};status='completed';conclusion='success';event='push';head_sha=$sourceSHA;head_branch='v1.0.15'}
    $release = @{tag_name='v1.0.15';draft=$false;prerelease=$false;body='<!-- skopeo-win-release-run:123 -->'}
    $commit = $sourceSHA
    $mergeBase = $sourceSHA
    switch ($case) {
        'main advanced' { $run.head_branch = 'v1.0.15' } # Only the merge base must match, not main's current tip.
        'legacy main recovery' { $run.head_branch = 'main' }
        'foreign repository' { $run.head_repository.full_name = 'other/repo' }
        'PR run' { $run.event = 'pull_request' }
        'failed run' { $run.conclusion = 'failure' }
        'wrong workflow' { $run.path = '.github/workflows/other.yml' }
        'draft' { $release.draft = $true }
        'prerelease' { $release.prerelease = $true }
        'wrong tag' { $release.tag_name = 'v1.0.16' }
        'wrong owner' { $release.body = '<!-- skopeo-win-release-run:999 -->' }
        'moved tag' { $commit = '2' * 40 }
        'outside main' { $mergeBase = '2' * 40 }
    }
    $caught = $false
    try {
        Assert-StoreSourceRun $run 'owner/repo'
        Assert-StoreSourceRelease $run '1.0.15' $release $commit $mergeBase
    } catch { $caught = $true }
    if ($caught -eq ($case -in @('valid tag','main advanced','legacy main recovery'))) { throw "Unexpected Store source result: $case" }
}
Write-Host 'Verified tag-only publication, build-only main/manual runs, invalid tags, Store eligibility and trusted release provenance.'
