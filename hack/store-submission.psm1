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

function Assert-StoreApplication([System.Collections.IDictionary] $Application, [switch] $InspectPending) {
    if ($Application['id'] -cne '9MT5QPKZDHGD' -or $Application['packageIdentityName'] -cne 'RambowCloud.skopeo-win') {
        throw 'Partner Center returned a different application identity'
    }
    if (-not $InspectPending -and $Application['pendingApplicationSubmission'] -and $Application['pendingApplicationSubmission']['id']) {
        throw 'An existing Store submission is pending. Preserve it and resolve it in Partner Center before retrying.'
    }
    if (-not $Application['lastPublishedApplicationSubmission'] -or -not $Application['lastPublishedApplicationSubmission']['id']) {
        throw 'The initial app submission must already be published'
    }
}

function ConvertTo-StoreCanonicalValue($Value) {
    if ($Value -is [System.Collections.IDictionary]) {
        $result = [ordered]@{}
        foreach ($key in @($Value.Keys | Sort-Object -CaseSensitive)) {
            $result[$key] = ConvertTo-StoreCanonicalValue $Value[$key]
        }
        return $result
    }
    if ($Value -is [array]) {
        $items = @($Value | ForEach-Object { ConvertTo-StoreCanonicalValue $_ })
        return ,$items
    }
    return $Value
}

function Assert-StoreUnchangedDraft(
    [System.Collections.IDictionary] $Draft,
    [System.Collections.IDictionary] $Published,
    [string] $ExpectedId
) {
    if ($ExpectedId -notmatch '^\d+$' -or $Draft['id'] -cne $ExpectedId -or
        $Draft['status'] -cne 'PendingCommit' -or $Published['status'] -cne 'Published') {
        throw 'Existing Store submission is not an unchanged editable draft of a published submission'
    }
    if (-not $Draft['applicationPackages'] -or -not $Draft['listings']) { throw 'Draft metadata is incomplete' }
    # Only service-generated submission fields may differ. Compare every other
    # field, including package IDs/status/versions, listing assets and rollout settings.
    $ignored = @('id', 'status', 'statusDetails', 'fileUploadUrl', 'friendlyName')
    $keys = @(@($Draft.Keys) + @($Published.Keys) | Sort-Object -Unique -CaseSensitive)
    foreach ($key in $keys) {
        if ($key -cin $ignored) { continue }
        $left = ConvertTo-StoreCanonicalValue $Draft[$key] | ConvertTo-Json -Depth 100 -Compress
        $right = ConvertTo-StoreCanonicalValue $Published[$key] | ConvertTo-Json -Depth 100 -Compress
        if (-not $Draft.Contains($key) -or -not $Published.Contains($key) -or $left -cne $right) {
            # Report field names only; metadata can contain private certification notes.
            throw "Existing Store draft contains changes in field '$key'; it was preserved without modification"
        }
    }
}

function Get-StoreReleaseNotes([string] $Version, [string] $Body) {
    if ($Version -notmatch '^[1-9]\d*\.(0|[1-9]\d*)\.(0|[1-9]\d*)$' -or
        [string]::IsNullOrWhiteSpace($Body)) { throw 'Version-specific published release notes are required' }
    $footer = "`nFull release notes: https://github.com/rambow-cloud/skopeo-win/releases/tag/v$Version"
    # Store text is limited to 1500 characters. Use actual feature names from
    # the verified release body and link to its complete descriptions.
    $notes = "Version $Version"
    foreach ($match in [regex]::Matches($Body, '(?m)^- \*\*([^\r\n*]+):\*\*')) {
        $line = "`n- $($match.Groups[1].Value.Trim())"
        if ($notes.Length + $line.Length + $footer.Length -gt 1500) { break }
        $notes += $line
    }
    if ($notes -ceq "Version $Version") {
        $text = ($Body -replace '[\p{Cc}\p{Cf}]', ' ').Trim()
        $limit = 1500 - $notes.Length - $footer.Length - 1
        $notes += "`n" + $text.Substring(0, [Math]::Min($limit, $text.Length))
    }
    return $notes + $footer
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

Export-ModuleMember -Function Assert-StoreApplication, Assert-StoreUnchangedDraft, Get-StoreReleaseNotes, New-StorePackageUpdate, Get-StoreErrorDiagnostic
