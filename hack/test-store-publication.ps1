#Requires -Version 7.0
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'store-publication.psm1') -Force
$root = Join-Path (Split-Path -Parent $PSScriptRoot) ('.cache/store-publication-tests/' + [guid]::NewGuid().ToString('N'))
$null = New-Item -ItemType Directory -Force $root
$seedFile = Join-Path $root 'seeds.json'
$saved = @{}
foreach ($name in @('AZURE_AD_TENANT_ID','AZURE_AD_APPLICATION_CLIENT_ID','AZURE_AD_APPLICATION_SECRET','GITHUB_REPOSITORY','GITHUB_ACTIONS')) {
    $saved[$name] = [Environment]::GetEnvironmentVariable($name)
}
$env:AZURE_AD_TENANT_ID = '11111111-1111-1111-1111-111111111111'
$env:AZURE_AD_APPLICATION_CLIENT_ID = '22222222-2222-2222-2222-222222222222'
$env:AZURE_AD_APPLICATION_SECRET = 'synthetic-sensitive-secret'
$env:GITHUB_REPOSITORY = 'owner/repo'
$env:GITHUB_ACTIONS = 'false'
$global:publicationModel = @{issues=@{};mutations=[Collections.Generic.List[string]]::new();requests=[Collections.Generic.List[string]]::new();status='Certification';bad='';next=100}
function global:gh {
    $arguments = @($args)
    if ($arguments[0] -cne 'api') { throw 'Unexpected native command in publication model' }
    $path = $arguments[1]
    $method = $arguments[[array]::IndexOf($arguments, '--method') + 1]
    $data = $null
    if ($arguments -contains '--input') { $data = Get-Content -Raw -LiteralPath $arguments[[array]::IndexOf($arguments, '--input') + 1] | ConvertFrom-Json -AsHashtable }
    $global:LASTEXITCODE = 0
    if ($method -cne 'GET') { $publicationModel.mutations.Add("$method $path") }
    if ($path -ceq 'repos/owner/repo/issues?creator=github-actions%5Bbot%5D&state=all&per_page=100' -and $method -ceq 'GET') {
        return ConvertTo-Json -InputObject @(@(@($publicationModel.issues.Values))) -Depth 30 -Compress
    }
    if ($path -ceq 'repos/owner/repo/issues' -and $method -ceq 'POST') {
        $number = $publicationModel.next++
        $publicationModel.issues[$number] = @{number=$number;title=$data.title;body=$data.body;state='open';user=@{login='github-actions[bot]';type='Bot'}}
        return $publicationModel.issues[$number] | ConvertTo-Json -Depth 30 -Compress
    }
    if ($path -cmatch '^repos/owner/repo/issues/([1-9]\d*)$') {
        $issue = $publicationModel.issues[[int]$Matches[1]]
        if ($method -ceq 'PATCH') { foreach ($key in $data.Keys) { $issue[$key] = $data[$key] } }
        elseif ($method -cne 'GET') { throw 'Unexpected issue mutation' }
        return $issue | ConvertTo-Json -Depth 30 -Compress
    }
    throw 'Unexpected GitHub path in publication model'
}
function Invoke-RestMethod {
    [CmdletBinding()]
    param($Uri,$Method='Get',$Headers,$Body,$ContentType,$TimeoutSec,$MaximumRedirection)
    $publicationModel.requests.Add("$Method $Uri")
    if ($MaximumRedirection -ne 0) { throw 'Publication requests must not redirect' }
    if ($Uri -ceq "https://login.microsoftonline.com/$env:AZURE_AD_TENANT_ID/oauth2/token" -and $Method -ceq 'Post') {
        if ($publicationModel.bad -ceq 'auth failure') { throw 'synthetic-sensitive-secret' }
        return @{access_token='synthetic-sensitive-token'}
    }
    if ($Method -cne 'Get' -or $Headers.Authorization -cne 'Bearer synthetic-sensitive-token') { throw 'Polling must only read the Store' }
    $api = 'https://manage.devcenter.microsoft.com/v1.0/my/applications/9MT5QPKZDHGD'
    if ($Uri -ceq $api) { return @{id='9MT5QPKZDHGD';packageIdentityName='RambowCloud.skopeo-win'} }
    if ($Uri -cmatch "^$([regex]::Escape($api))/submissions/([1-9]\d*)(/status)?$") {
        $id = $Matches[1]
        if ($Uri.EndsWith('/status')) {
            if ($publicationModel.bad -ceq 'API failure') { throw 'synthetic-sensitive-secret https://secret.example/?sig=secret' }
            return @{status=$publicationModel.status}
        }
        $version = if ($id -ceq '123') { '1.0.12.0' } else { '1.0.14.0' }
        $packages = @(@{fileStatus='Uploaded';version=$version},@{fileStatus='Uploaded';version=$version;architecture='ARM64'})
        $status = 'Published'
        switch ($publicationModel.bad) {
            'wrong version' { $packages[1].version = '1.0.13.0' }
            'wrong submission' { $id = '999' }
            'status mismatch' { $status = 'Certification' }
            'empty packages' { $packages = @() }
            'incomplete package' { $packages[1].fileStatus = 'PendingUpload' }
        }
        return @{id=$id;status=$status;applicationPackages=$packages}
    }
    throw 'Unexpected Store request in publication model'
}
function Reset-PublicationModel {
    $publicationModel.issues.Clear(); $publicationModel.mutations.Clear(); $publicationModel.requests.Clear()
    $publicationModel.status = 'Certification'; $publicationModel.bad = ''
    $record = New-StorePublicationRecord '1.0.14.0' '456'
    $publicationModel.issues[7] = @{number=7;state='open';user=@{login='github-actions[bot]';type='Bot'};body=(Get-StorePublicationBody $record 'AwaitingVerification')}
    $publicationModel.issues[8] = @{number=8;state='open';user=@{login='someone';type='User'};body=$publicationModel.issues[7].body}
    $publicationModel.issues[9] = @{number=9;state='open';user=@{login='github-actions[bot]';type='Bot'};body=$publicationModel.issues[7].body;pull_request=@{}}
    '[]' | Set-Content -LiteralPath $seedFile
}
function Invoke-PublicationFixture([string] $Name, [bool] $ExpectFailure = $false) {
    $caught = $null
    try { & (Join-Path $PSScriptRoot 'watch-store-publication.ps1') -SeedFile $seedFile -OutputDirectory (Join-Path $root $Name) }
    catch { $caught = $_.Exception.Message }
    if (($null -ne $caught) -ne $ExpectFailure -or ($caught -and $caught -match 'synthetic|secret.example')) { throw "Unexpected or unsafe publication outcome: $Name / $caught" }
    if ($publicationModel.issues[8].state -cne 'open' -or $publicationModel.issues[9].state -cne 'open') { throw 'A human issue or PR was closed by issue text' }
}
try {
    foreach ($status in @('PendingCommit','CommitStarted','PreProcessing','Certification','CertificationPassed','ReadyForRelease','Release','PendingPublication','Publishing')) {
        Reset-PublicationModel
        $publicationModel.status = $status
        Invoke-PublicationFixture $status
        if ($publicationModel.issues[7].state -cne 'open') { throw 'Intermediate status closed an issue' }
        $count = $publicationModel.mutations.Count
        Invoke-PublicationFixture "$status-repeat"
        if ($publicationModel.mutations.Count -ne $count) { throw 'Unchanged polling repeatedly edited the issue' }
    }
    foreach ($status in @('CommitFailed','PreProcessingFailed','CertificationFailed','ReleaseFailed','PublishFailed','Canceled','UnknownStatus')) {
        Reset-PublicationModel; $publicationModel.status = $status
        Invoke-PublicationFixture $status $true
        if ($publicationModel.issues[7].state -cne 'open') { throw 'Failure closed an issue' }
    }
    foreach ($bad in @('wrong version','wrong submission','status mismatch','empty packages','incomplete package','API failure','auth failure')) {
        Reset-PublicationModel; $publicationModel.status = 'Published'; $publicationModel.bad = $bad
        Invoke-PublicationFixture ($bad -replace ' ','-') $true
        if ($publicationModel.issues[7].state -cne 'open') { throw 'Unverified publication closed an issue' }
    }
    Reset-PublicationModel; $publicationModel.status = 'Published'
    $publicationModel.issues[22] = @{number=22;state='open';user=@{login='maintainer';type='User'};body='Original historical delivery evidence.'}
    '[{"version":"1.0.12.0","submissionId":"123","legacyIssue":22}]' | Set-Content -LiteralPath $seedFile
    Invoke-PublicationFixture 'published-and-legacy'
    if ($publicationModel.issues[7].state -cne 'closed' -or $publicationModel.issues[22].state -cne 'closed' -or
        -not $publicationModel.issues[22].body.StartsWith('Original historical delivery evidence.')) { throw 'Verified publication or reviewed legacy closure failed' }
    $count = $publicationModel.mutations.Count; $requests = $publicationModel.requests.Count
    Invoke-PublicationFixture 'published-repeat'
    if ($publicationModel.mutations.Count -ne $count -or $publicationModel.requests.Count -ne $requests) { throw 'Closed publications were repeatedly polled or edited' }
    Reset-PublicationModel; $publicationModel.issues.Remove(7)
    $report = Join-Path $root 'submission.json'
    @{productId='9MT5QPKZDHGD';version='1.0.14.0';submissionId='456';bundleSHA256=('a'*64)} | ConvertTo-Json | Set-Content -LiteralPath $report
    & (Join-Path $PSScriptRoot 'track-store-submission.ps1') -Report $report -ExpectedVersion '1.0.14'
    $count = $publicationModel.mutations.Count
    & (Join-Path $PSScriptRoot 'track-store-submission.ps1') -Report $report -ExpectedVersion '1.0.14'
    if ($publicationModel.mutations.Count -ne $count) { throw 'Submission registration created a duplicate issue' }
    $rejected = $false
    try { & (Join-Path $PSScriptRoot 'track-store-submission.ps1') -Report $report -ExpectedVersion '1.0.15' } catch { $rejected = $true }
    if (-not $rejected -or $publicationModel.mutations.Count -ne $count) { throw 'Mismatched report created a tracking issue' }
    Reset-PublicationModel; $publicationModel.issues.Remove(7)
    '[{"version":"1.0.14.0","submissionId":"456"}]' | Set-Content -LiteralPath $seedFile
    Invoke-PublicationFixture 'seed-current-submission'
    if (@($publicationModel.mutations | Where-Object {$_ -ceq 'POST repos/owner/repo/issues'}).Count -ne 1) { throw 'Existing Store submission was not adopted' }
    $publicationModel.issues.Clear(); $publicationModel.mutations.Clear()
    & (Join-Path $PSScriptRoot 'watch-store-publication.ps1') -SeedFile $seedFile -OutputDirectory (Join-Path $root 'first-tracker')
    if ($publicationModel.issues.Count -ne 1 -or @($publicationModel.mutations | Where-Object {$_ -ceq 'POST repos/owner/repo/issues'}).Count -ne 1) { throw 'First-ever tracker registration failed on empty pagination' }
    @{productId='9MT5QPKZDHGD';version='1.0.14.0';submissionId=$null} | ConvertTo-Json | Set-Content -LiteralPath $report
    $count = $publicationModel.mutations.Count
    & (Join-Path $PSScriptRoot 'track-store-submission.ps1') -Report $report -ExpectedVersion '1.0.14'
    if ($publicationModel.mutations.Count -ne $count) { throw 'A missing submission created an issue' }
    Write-Host 'Verified Store polling lifecycle, exact publication/version gates, durable issue registration, legacy migration, ownership checks, idempotence and secret-safe failures.'
} finally {
    Remove-Item function:global:gh
    Remove-Variable publicationModel -Scope Global
    foreach ($name in $saved.Keys) { [Environment]::SetEnvironmentVariable($name, $saved[$name]) }
}
