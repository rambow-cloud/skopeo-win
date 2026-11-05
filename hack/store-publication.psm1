Set-StrictMode -Version Latest

function New-StorePublicationRecord([string] $Version, [string] $SubmissionId) {
    if ($Version -cnotmatch '^[1-9]\d*\.(0|[1-9]\d*)\.(0|[1-9]\d*)\.0$' -or
        $Version -ceq '1.23.0.0' -or $SubmissionId -notmatch '^[1-9]\d{0,19}$') {
        throw 'Invalid Store publication identity'
    }
    $parsed = [version]$Version
    if ($parsed.Major -gt 65535 -or $parsed.Minor -gt 65535 -or $parsed.Build -gt 65535) { throw 'Invalid Store version range' }
    return [ordered]@{ schema = 1; productId = '9MT5QPKZDHGD'; version = $Version; submissionId = $SubmissionId }
}

function Get-StorePublicationRecord([System.Collections.IDictionary] $Issue) {
    # Only Actions-created tracker issues may opt in through issue text.
    if ($Issue.Contains('pull_request') -or $Issue['user']['login'] -cne 'github-actions[bot]' -or $Issue['user']['type'] -cne 'Bot') { return $null }
    $matches = [regex]::Matches([string]$Issue['body'], '<!-- skopeo-win-store-publication:(\{[^\r\n]{1,1000}\}) -->')
    if ($matches.Count -eq 0) { return $null }
    if ($matches.Count -ne 1) { throw 'Ambiguous Store tracking metadata' }
    $data = $matches[0].Groups[1].Value | ConvertFrom-Json -AsHashtable
    if ($data['schema'] -ne 1 -or $data['productId'] -cne '9MT5QPKZDHGD' -or $data.Count -ne 4) { throw 'Unexpected Store tracking metadata' }
    return New-StorePublicationRecord $data['version'] $data['submissionId']
}

function Test-StorePublicationComplete([System.Collections.IDictionary] $Record, [string] $Status, [System.Collections.IDictionary] $Submission) {
    if ($Status -cne 'Published') { return $false }
    if (-not $Submission -or $Submission['id'] -cne $Record['submissionId'] -or $Submission['status'] -cne 'Published') {
        throw 'Published status does not match the expected submission'
    }
    $packages = @($Submission['applicationPackages'] | Where-Object { $_['fileStatus'] -ceq 'Uploaded' })
    if ($packages.Count -eq 0) { throw 'Published submission has no uploaded package' }
    foreach ($package in $packages) {
        if ($package['version'] -cne $Record['version']) { throw 'Published package version differs from the tracked version' }
    }
    if (@($Submission['applicationPackages'] | Where-Object { $_['fileStatus'] -cne 'Uploaded' -and $_['fileStatus'] -cne 'PendingDelete' }).Count -gt 0) {
        throw 'Published submission contains an incomplete package'
    }
    return $true
}

function Get-StorePublicationBody([System.Collections.IDictionary] $Record, [string] $Status, [string] $ExistingBody = '') {
    $known = @('AwaitingVerification','CheckFailed','None','Canceled','PendingCommit','CommitStarted','CommitFailed',
        'PendingPublication','Publishing','Published','PublishFailed','PreProcessing','PreProcessingFailed',
        'Certification','CertificationFailed','CertificationPassed','ReadyForRelease','Release','ReleaseFailed')
    if ($Status -cnotin $known) { throw 'Unrecognized Store publication status' }
    $version = ([version]$Record['version']).ToString(3)
    $data = $Record | ConvertTo-Json -Compress
    $block = @"
<!-- skopeo-win-store-status:start -->
## Microsoft Store publication

- Package: $($Record['version'])
- Submission: $($Record['submissionId'])
- Status: **$Status**
- [Source release](https://github.com/$env:GITHUB_REPOSITORY/releases/tag/v$version)
- [Microsoft Store](https://apps.microsoft.com/detail/9MT5QPKZDHGD)

Actions checks this submission hourly. This issue closes only after Microsoft reports Published and the package version matches. Processing, certification, release-ready states and errors keep it open.
<!-- skopeo-win-store-publication:$data -->
<!-- skopeo-win-store-status:end -->
"@
    if ($ExistingBody.Contains('<!-- skopeo-win-store-status:start -->')) {
        return [regex]::Replace($ExistingBody, '(?s)<!-- skopeo-win-store-status:start -->.*?<!-- skopeo-win-store-status:end -->', [Text.RegularExpressions.MatchEvaluator]{ param($m) $block }).TrimEnd()
    }
    return ($ExistingBody.TrimEnd() + "`n`n" + $block).Trim()
}

function Invoke-StoreIssueAPI([string] $Path, [string] $Method = 'GET', [hashtable] $Data, [switch] $Paginate) {
    if ($env:GITHUB_REPOSITORY -cnotmatch '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$') { throw 'Invalid repository identity' }
    $arguments = @('api', "repos/$env:GITHUB_REPOSITORY/$Path", '--method', $Method)
    if ($Paginate) { $arguments += @('--paginate','--slurp') }
    $file = $null
    try {
        if ($null -ne $Data) {
            $file = Join-Path ([IO.Path]::GetTempPath()) ('skopeo-store-issue-' + [guid]::NewGuid().ToString('N') + '.json')
            $Data | ConvertTo-Json -Depth 30 | Set-Content -LiteralPath $file -Encoding utf8
            $arguments += @('--input', $file)
        }
        $raw = gh @arguments 2>$null
        if ($LASTEXITCODE -ne 0) { throw 'GitHub Store tracking request failed' }
        $result = ($raw -join "`n") | ConvertFrom-Json -AsHashtable
        return ,$result
    } finally { if ($file) { Remove-Item -LiteralPath $file -ErrorAction SilentlyContinue } }
}

function Get-StorePublicationIssues {
    $pages = Invoke-StoreIssueAPI 'issues?creator=github-actions%5Bbot%5D&state=all&per_page=100' -Paginate
    foreach ($page in $pages) {
        foreach ($issue in $page) {
            $record = Get-StorePublicationRecord $issue
            if ($record) { @{ issue = $issue; record = $record } }
        }
    }
}

function Register-StorePublication([System.Collections.IDictionary] $Record, [object[]] $Trackers) {
    $matching = @($Trackers | Where-Object { $_.record.version -ceq $Record['version'] -and $_.record.submissionId -ceq $Record['submissionId'] })
    if ($matching.Count -gt 1) { throw 'Duplicate Store tracking issues require review' }
    if ($matching.Count -eq 1) { return $matching[0] }
    $issue = Invoke-StoreIssueAPI 'issues' 'POST' @{
        title = "Microsoft Store $($Record['version']) publication"
        body = Get-StorePublicationBody $Record 'AwaitingVerification'
    }
    if (-not (Get-StorePublicationRecord $issue)) { throw 'Store tracking issue was not created by Actions' }
    return @{ issue = $issue; record = $Record }
}

function Update-StorePublicationIssue([System.Collections.IDictionary] $Tracker, [string] $Status, [bool] $Published) {
    $issue = $Tracker['issue']
    if ($issue['number'] -notmatch '^[1-9]\d*$' -or $issue.Contains('pull_request')) { throw 'Invalid Store tracking issue' }
    if ($Published -and $Status -cne 'Published') { throw 'Only verified publication may close a tracking issue' }
    $body = Get-StorePublicationBody $Tracker['record'] $Status ([string]$issue['body'])
    $data = @{ body = $body }
    if ($Published -and $issue['state'] -ceq 'open') { $data['state'] = 'closed'; $data['state_reason'] = 'completed' }
    if ($body -cne $issue['body'] -or $data.ContainsKey('state')) {
        $null = Invoke-StoreIssueAPI "issues/$($issue['number'])" 'PATCH' $data
    }
}

Export-ModuleMember -Function New-StorePublicationRecord, Get-StorePublicationRecord, Test-StorePublicationComplete, Get-StorePublicationBody, Invoke-StoreIssueAPI, Get-StorePublicationIssues, Register-StorePublication, Update-StorePublicationIssue
