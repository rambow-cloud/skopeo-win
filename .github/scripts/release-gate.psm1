Set-StrictMode -Version Latest

function Get-ReleaseTag([string] $EventName, [string] $Ref) {
    if ($EventName -cne 'push' -or -not $Ref.StartsWith('refs/tags/', [StringComparison]::Ordinal)) { return '' }
    if ($Ref -cnotmatch '^refs/tags/v([1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$' -or $Ref -ceq 'refs/tags/v1.23.0') {
        throw 'Release tags must use vX.Y.Z; the historical v1.23.0 is reserved'
    }
    foreach ($part in $Matches[1..3]) {
        if ([decimal]$part -gt 65535) { throw 'Release version components must fit Microsoft Store package limits' }
    }
    return $Ref.Substring('refs/tags/'.Length)
}

function Test-AutomaticStoreSubmission([string] $EventName, [object[]] $Jobs) {
    if ($EventName -ceq 'workflow_dispatch') { return $true }
    if ($EventName -cne 'workflow_run') { throw 'Unexpected Store workflow event' }
    $releaseJobs = @($Jobs | Where-Object { $_.name -ceq 'release' })
    if ($releaseJobs.Count -ne 1) { throw 'Expected exactly one source release job' }
    if ($releaseJobs[0].conclusion -ceq 'skipped') { return $false }
    if ($releaseJobs[0].conclusion -cne 'success') { throw 'The source release job did not succeed' }
    return $true
}

function Assert-StoreSourceRun([object] $Run, [string] $Repository) {
    if ($Run.path -cne '.github/workflows/windows.yml' -or $Run.head_repository.full_name -cne $Repository -or
        $Run.conclusion -cne 'success' -or $Run.status -cne 'completed' -or $Run.event -notin @('push', 'workflow_dispatch') -or
        [string]$Run.id -notmatch '^[1-9]\d*$' -or $Run.head_sha -cnotmatch '^[a-f0-9]{40}$') {
        throw 'Only a successful trusted Windows binaries run can update the Store'
    }
}

function Assert-StoreSourceRelease([object] $Run, [string] $Version, [object] $Release, [string] $Commit, [string] $MergeBase) {
    if ($Version -cnotmatch '^[1-9]\d*\.(0|[1-9]\d*)\.(0|[1-9]\d*)$' -or $Version -ceq '1.23.0' -or
        $Release.draft -or $Release.prerelease -or $Release.tag_name -cne "v$Version" -or
        -not $Release.body.Contains("<!-- skopeo-win-release-run:$($Run.id) -->") -or
        $Commit -cne $Run.head_sha -or $MergeBase -cne $Run.head_sha) {
        throw 'Store source must match its published release tag, workflow provenance and main history'
    }
}

Export-ModuleMember -Function Get-ReleaseTag, Test-AutomaticStoreSubmission, Assert-StoreSourceRun, Assert-StoreSourceRelease
