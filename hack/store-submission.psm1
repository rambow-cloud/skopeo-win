Set-StrictMode -Version Latest

function Assert-StoreApplication([System.Collections.IDictionary] $Application) {
    if ($Application['id'] -cne '9MT5QPKZDHGD' -or $Application['packageIdentityName'] -cne 'RambowCloud.skopeo-win') {
        throw 'Partner Center returned a different application identity'
    }
    if ($Application['pendingApplicationSubmission'] -and $Application['pendingApplicationSubmission']['id']) {
        throw 'An existing Store submission is pending. Preserve it and resolve it in Partner Center before retrying.'
    }
    if (-not $Application['lastPublishedApplicationSubmission'] -or -not $Application['lastPublishedApplicationSubmission']['id']) {
        throw 'The initial app submission must already be published'
    }
}

function New-StorePackageUpdate(
    [System.Collections.IDictionary] $Submission,
    [string] $Version,
    [string] $FileName,
    [string] $ReleaseNotes
) {
    if ($Version -notmatch '^[1-9]\d*\.(0|[1-9]\d*)\.(0|[1-9]\d*)\.0$' -or
        $FileName -cne "skopeo-win-$Version.msixbundle") {
        throw 'Invalid Store package version or filename'
    }
    if ($Submission['status'] -cne 'PendingCommit') { throw 'Store submission is not an editable draft' }
    if (-not $Submission['listings'] -or -not $Submission['applicationPackages']) {
        throw 'Published listing metadata and packages are required'
    }
    # Clone before editing so the retrieved metadata remains available for comparison.
    $updated = $Submission | ConvertTo-Json -Depth 100 | ConvertFrom-Json -AsHashtable
    foreach ($key in @('id', 'status', 'statusDetails', 'fileUploadUrl')) { $null = $updated.Remove($key) }
    $packages = @()
    foreach ($package in $updated['applicationPackages']) {
        if ($package['fileStatus'] -cne 'Uploaded' -or -not $package['version']) {
            throw 'Existing Store package is not a published, versioned package'
        }
        if ([version]$package['version'] -ge [version]$Version) {
            throw 'Store updates must increase the published package version'
        }
        $package['fileStatus'] = 'PendingDelete'
        $packages += $package
    }
    $packages += @{
        fileName = $FileName
        fileStatus = 'PendingUpload'
        minimumDirectXVersion = 'None'
        minimumSystemRam = 'None'
    }
    $updated['applicationPackages'] = $packages
    $updated['targetPublishMode'] = 'Immediate'
    $null = $updated.Remove('targetPublishDate')
    if ($updated['listings'].Contains('en-us')) {
        $updated['listings']['en-us']['baseListing']['releaseNotes'] = $ReleaseNotes
    }
    return $updated
}

Export-ModuleMember -Function Assert-StoreApplication, New-StorePackageUpdate
