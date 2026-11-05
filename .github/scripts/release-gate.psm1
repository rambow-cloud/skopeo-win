Set-StrictMode -Version Latest

function Test-ReleaseNeeded([AllowEmptyCollection()] [string[]] $ChangedPaths = @()) {
    $guides = @('README.md', 'WINDOWS.md', 'CONTRIBUTING.md', 'CODE_OF_CONDUCT.md', 'SECURITY.md')
    foreach ($path in $ChangedPaths) {
        # Unknown or quoted paths must never hide a production change.
        if ([string]::IsNullOrWhiteSpace($path) -or $path.Contains('\') -or
            $path.StartsWith('/') -or $path -cmatch '(^|/)\.\.(/|$)') { return $true }
        if ($path -cmatch '^(\.github/|docs/)' -or $path -cin $guides) { continue }
        return $true
    }
    return $false
}

function Test-ReleaseNeededSince([string] $BaseTag) {
    if ($BaseTag -cnotmatch '^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$') { throw 'Invalid published release tag' }
    # Include deletions and both sides of renames. Compare with the published
    # release, so a later CI fix cannot hide previously unreleased source changes.
    $paths = @(git -c core.quotepath=true diff --name-only --no-renames $BaseTag HEAD --)
    if ($LASTEXITCODE -ne 0) { throw 'Cannot compare the published release with the current source' }
    return Test-ReleaseNeeded $paths
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

Export-ModuleMember -Function Test-ReleaseNeeded, Test-ReleaseNeededSince, Test-AutomaticStoreSubmission
