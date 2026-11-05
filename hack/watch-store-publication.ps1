#Requires -Version 7.0
param([string] $SeedFile = (Join-Path $PSScriptRoot '../.github/store-publications.json'), [string] $OutputDirectory = '.cache/store-publication')
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'store-publication.psm1') -Force
$trackers = @(Get-StorePublicationIssues)
# These migration links come only from reviewed main-branch configuration,
# never from issue text or caller-provided issue numbers.
foreach ($seed in @(Get-Content -Raw -LiteralPath $SeedFile | ConvertFrom-Json -AsHashtable)) {
    $record = New-StorePublicationRecord $seed['version'] $seed['submissionId']
    if ($seed.Contains('legacyIssue')) {
        if ([string]$seed['legacyIssue'] -notmatch '^[1-9]\d*$') { throw 'Invalid reviewed legacy issue' }
        $issue = Invoke-StoreIssueAPI "issues/$($seed['legacyIssue'])"
        if ($issue.Contains('pull_request')) { throw 'Legacy Store tracking cannot target a PR' }
        $trackers += @{ issue = $issue; record = $record }
    } else {
        $registered = Register-StorePublication $record $trackers
        if (@($trackers | Where-Object { $_.issue.number -eq $registered.issue.number }).Count -eq 0) { $trackers += $registered }
    }
}
$pending = @($trackers | Where-Object { $_.issue.state -ceq 'open' })
if ($pending.Count -gt 50) { throw 'Too many open Store publication trackers' }
if ($pending.Count -eq 0) { Write-Host 'No pending Store publications.'; return }
foreach ($name in @('AZURE_AD_TENANT_ID','AZURE_AD_APPLICATION_CLIENT_ID')) {
    $parsed = [guid]::Empty
    if (-not [guid]::TryParse([Environment]::GetEnvironmentVariable($name), [ref]$parsed) -or $parsed -eq [guid]::Empty) { throw "Invalid $name" }
}
if ([string]::IsNullOrWhiteSpace($env:AZURE_AD_APPLICATION_SECRET)) { throw 'Missing Store application credential' }
function Invoke-StatusRequest([string] $Operation, [hashtable] $Parameters) {
    try { Invoke-RestMethod @Parameters -MaximumRedirection 0 -ErrorAction Stop }
    catch { throw "$Operation failed. Credentials and raw response are omitted." }
}
$auth = Invoke-StatusRequest 'Store authentication' @{
    Uri = "https://login.microsoftonline.com/$env:AZURE_AD_TENANT_ID/oauth2/token"; Method = 'Post'
    ContentType = 'application/x-www-form-urlencoded'; TimeoutSec = 60
    Body = @{grant_type='client_credentials';client_id=$env:AZURE_AD_APPLICATION_CLIENT_ID;client_secret=$env:AZURE_AD_APPLICATION_SECRET;resource='https://manage.devcenter.microsoft.com'}
}
if (-not $auth.access_token) { throw 'Store authentication returned no access token' }
if ($env:GITHUB_ACTIONS -eq 'true') { Write-Output "::add-mask::$($auth.access_token)" }
$headers = @{Authorization="Bearer $($auth.access_token)";Accept='application/json'}
$api = 'https://manage.devcenter.microsoft.com/v1.0/my/applications/9MT5QPKZDHGD'
$application = Invoke-StatusRequest 'Read Store identity' @{Uri=$api;Headers=$headers;TimeoutSec=60}
if ($application.id -cne '9MT5QPKZDHGD' -or $application.packageIdentityName -cne 'RambowCloud.skopeo-win') { throw 'Store identity mismatch' }
$null = New-Item -ItemType Directory -Force -Path $OutputDirectory
$results = [Collections.Generic.List[object]]::new()
$failed = $false
foreach ($tracker in $pending) {
    $record = $tracker.record
    $status = 'CheckFailed'
    $published = $false
    try {
        $response = Invoke-StatusRequest 'Read Store submission status' @{Uri="$api/submissions/$($record.submissionId)/status";Headers=$headers;TimeoutSec=60}
        $status = [string]$response.status
        $submission = $null
        if ($status -ceq 'Published') {
            $submission = Invoke-StatusRequest 'Verify published Store package' @{Uri="$api/submissions/$($record.submissionId)";Headers=$headers;TimeoutSec=60}
            $submission = $submission | ConvertTo-Json -Depth 100 | ConvertFrom-Json -AsHashtable
        }
        $published = Test-StorePublicationComplete $record $status $submission
        $null = Get-StorePublicationBody $record $status
    } catch { $status = 'CheckFailed'; $published = $false; $failed = $true }
    Update-StorePublicationIssue $tracker $status $published
    $results.Add(@{version=$record.version;submissionId=$record.submissionId;issue=$tracker.issue.number;status=$status;published=$published})
    Write-Host "Store $($record.version): $status; issue #$($tracker.issue.number)."
    if ($status -in @('Canceled','CommitFailed','PublishFailed','PreProcessingFailed','CertificationFailed','ReleaseFailed')) { $failed = $true }
}
$results | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $OutputDirectory 'status.json') -Encoding utf8
if ($failed) { throw 'Some Store publications require attention; tracking issues remain open.' }
