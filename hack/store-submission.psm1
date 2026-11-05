Set-StrictMode -Version Latest

function Get-StoreErrorDiagnostic([string] $Body, [string[]] $SensitiveValues = @()) {
    # Read only structured error fields, never arbitrary response bodies or headers.
    if ([string]::IsNullOrWhiteSpace($Body) -or $Body.Length -gt 65536) { return '' }
    try { $data = ConvertFrom-Json -InputObject $Body -AsHashtable -ErrorAction Stop }
    catch { return '' }
    if ($data -isnot [System.Collections.IDictionary]) { return '' }
    $errorData = if ($data['error'] -is [System.Collections.IDictionary]) { $data['error'] } else { $data }
    $fields = [Collections.Generic.List[string]]::new()
    foreach ($key in @('code', 'message', 'error_description')) {
        if ($errorData[$key] -is [string]) { $fields.Add($errorData[$key]) }
    }
    if ($data['error'] -is [string]) { $fields.Add($data['error']) }
    foreach ($detail in @($errorData['details']) | Select-Object -First 4) {
        if ($detail -is [System.Collections.IDictionary]) {
            foreach ($key in @('code', 'message')) {
                if ($detail[$key] -is [string]) { $fields.Add($detail[$key]) }
            }
        }
    }
    $text = $fields -join '; '
    # Remove known credentials before truncating, including URL-encoded forms.
    foreach ($value in $SensitiveValues) {
        if (-not [string]::IsNullOrEmpty($value)) {
            foreach ($form in @($value, [uri]::EscapeDataString($value), [Net.WebUtility]::UrlEncode($value))) {
                $text = $text.Replace($form, '[redacted]', [StringComparison]::OrdinalIgnoreCase)
            }
        }
    }
    $text = $text -replace 'https?://[^\s"<>]+', '[URL omitted]'
    $text = $text -replace '(?i)Bearer\s+[^\s,;"<>]+', 'Bearer [redacted]'
    $text = $text -replace '[\p{Cc}\p{Cf}]', ' '
    $text = $text.Replace('::', ':')
    if ($text.Length -gt 1000) { $text = $text.Substring(0, 1000) + '...' }
    return $text
}

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

Export-ModuleMember -Function Assert-StoreApplication, New-StorePackageUpdate, Get-StoreErrorDiagnostic
