#Requires -Version 7.0
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)] [string] $Bundle,
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[1-9]\d*\.(0|[1-9]\d*)\.(0|[1-9]\d*)$')] [string] $Version,
    [string] $OutputDirectory = '.cache/store-submission'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'store-submission.psm1') -Force
$bundlePath = (Resolve-Path -LiteralPath $Bundle).Path
$fileName = [IO.Path]::GetFileName($bundlePath)
if ($fileName -cne "skopeo-win-$Version.0.msixbundle") { throw 'Bundle name differs from the requested version' }
$checksum = (Get-Content -Raw -LiteralPath "$bundlePath.sha256").Trim() -split '\s+'
$hash = (Get-FileHash -LiteralPath $bundlePath -Algorithm SHA256).Hash.ToLowerInvariant()
if ($checksum.Count -ne 2 -or $checksum[0] -cne $hash -or $checksum[1] -cne $fileName) {
    throw 'Store bundle checksum mismatch'
}
foreach ($name in @('AZURE_AD_TENANT_ID', 'AZURE_AD_APPLICATION_CLIENT_ID')) {
    $value = [Environment]::GetEnvironmentVariable($name)
    $parsed = [guid]::Empty
    if (-not [guid]::TryParse($value, [ref]$parsed) -or $parsed -eq [guid]::Empty) { throw "Missing or invalid $name" }
}
if ([string]::IsNullOrWhiteSpace($env:AZURE_AD_APPLICATION_SECRET)) { throw 'Missing AZURE_AD_APPLICATION_SECRET' }
$null = New-Item -ItemType Directory -Force -Path $OutputDirectory
$reportPath = Join-Path $OutputDirectory 'submission.json'
$report = [ordered]@{ productId = '9MT5QPKZDHGD'; version = "$Version.0"; bundleSHA256 = $hash; submissionId = $null; status = 'NotStarted' }
function Save-Report { $report | ConvertTo-Json | Set-Content -LiteralPath $reportPath -Encoding utf8 }
$script:storeAccessToken = ''
function Invoke-SafeRequest([string] $Operation, [hashtable] $Parameters) {
    try { Invoke-RestMethod @Parameters -MaximumRedirection 0 -ErrorAction Stop }
    catch {
        # Never print raw response bodies, request parameters, tokens, or upload SAS URLs.
        $responseProperty = $_.Exception.PSObject.Properties['Response']
        $code = if ($responseProperty -and $responseProperty.Value) { [int]$responseProperty.Value.StatusCode } else { 'transport error' }
        $errorBody = if ($_.ErrorDetails) { $_.ErrorDetails.Message } else { '' }
        $diagnostic = Get-StoreErrorDiagnostic -Body $errorBody -SensitiveValues @(
            $env:AZURE_AD_TENANT_ID, $env:AZURE_AD_APPLICATION_CLIENT_ID,
            $env:AZURE_AD_APPLICATION_SECRET, $script:storeAccessToken
        )
        $report['failedOperation'] = $Operation
        $report['httpStatus'] = $code
        $report['diagnostic'] = $diagnostic
        Save-Report
        if ($diagnostic) { throw "$Operation failed ($code). API diagnostic: $diagnostic" }
        throw "$Operation failed ($code). No safe structured API diagnostic was available."
    }
}
Save-Report
$auth = Invoke-SafeRequest 'Entra authentication' @{
    Uri = "https://login.microsoftonline.com/$env:AZURE_AD_TENANT_ID/oauth2/token"
    Method = 'Post'; ContentType = 'application/x-www-form-urlencoded'; TimeoutSec = 60
    Body = @{ grant_type = 'client_credentials'; client_id = $env:AZURE_AD_APPLICATION_CLIENT_ID; client_secret = $env:AZURE_AD_APPLICATION_SECRET; resource = 'https://manage.devcenter.microsoft.com' }
}
if (-not $auth.access_token) { throw 'Entra returned no access token' }
$script:storeAccessToken = [string]$auth.access_token
if ($env:GITHUB_ACTIONS -eq 'true') { Write-Output "::add-mask::$($auth.access_token)" }
$headers = @{ Authorization = "Bearer $($auth.access_token)"; Accept = 'application/json' }
$baseUri = 'https://manage.devcenter.microsoft.com/v1.0/my/applications/9MT5QPKZDHGD'
$application = Invoke-SafeRequest 'Get Store application' @{ Uri = $baseUri; Headers = $headers; TimeoutSec = 60 }
$applicationData = $application | ConvertTo-Json -Depth 100 | ConvertFrom-Json -AsHashtable
Assert-StoreApplication $applicationData
$publishedId = $applicationData['lastPublishedApplicationSubmission']['id']
if ($publishedId -notmatch '^\d+$') { throw 'Invalid published submission ID' }
$published = Invoke-SafeRequest 'Get published submission' @{ Uri = "$baseUri/submissions/$publishedId"; Headers = $headers; TimeoutSec = 60 }
$publishedData = $published | ConvertTo-Json -Depth 100 | ConvertFrom-Json -AsHashtable
# Reject stale versions before creating any draft.
foreach ($package in $publishedData['applicationPackages']) {
    if ([version]$package['version'] -ge [version]"$Version.0") { throw 'This version is already published or has been superseded' }
}
$submission = Invoke-SafeRequest 'Create Store submission' @{ Uri = "$baseUri/submissions"; Method = 'Post'; Headers = $headers; ContentType = 'application/json'; TimeoutSec = 60 }
$data = $submission | ConvertTo-Json -Depth 100 | ConvertFrom-Json -AsHashtable
$submissionId = $data['id']
if ($submissionId -notmatch '^\d+$') { throw 'Store returned an invalid submission ID' }
$report.submissionId = $submissionId
$report.status = 'PendingCommit'
Save-Report
Write-Host "Created Store submission $submissionId for $Version.0. Existing drafts were not deleted."
$uploadUrl = [string]$data['fileUploadUrl']
if ($env:GITHUB_ACTIONS -eq 'true' -and $uploadUrl) { Write-Output "::add-mask::$uploadUrl" }
try { $uploadUri = [uri]$uploadUrl } catch { throw 'Store returned an invalid upload destination' }
if ($uploadUri.Scheme -cne 'https' -or -not $uploadUri.DnsSafeHost.EndsWith('.blob.core.windows.net') -or
    $uploadUri.UserInfo -or $uploadUri.Port -ne 443) { throw 'Store returned an unexpected upload destination' }
$notes = "Version $Version adds scoped public-image trust defaults and Linux/native-architecture image selection on Windows. A missing policy is generated automatically or supplied by the embedded template; configured policies, signature requirements and TLS checks remain in effect."
$updated = New-StorePackageUpdate $data "$Version.0" $fileName $notes
$null = Invoke-SafeRequest 'Update Store package metadata' @{
    Uri = "$baseUri/submissions/$submissionId"; Method = 'Put'; Headers = $headers
    ContentType = 'application/json; charset=utf-8'; Body = ($updated | ConvertTo-Json -Depth 100); TimeoutSec = 60
}
$archive = Join-Path $OutputDirectory "submission-$submissionId.zip"
if (Test-Path -LiteralPath $archive) { throw 'Submission archive already exists; do not overwrite an earlier attempt' }
Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [IO.Compression.ZipFile]::Open($archive, [IO.Compression.ZipArchiveMode]::Create)
try { $null = [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $bundlePath, $fileName) }
finally { $zip.Dispose() }
try {
    $null = Invoke-WebRequest -Uri $uploadUri -Method Put -InFile $archive -Headers @{'x-ms-blob-type' = 'BlockBlob'} -ContentType 'application/zip' -MaximumRedirection 0 -TimeoutSec 600 -ErrorAction Stop
} catch { throw 'Store package upload failed. The existing draft is retained; upload credentials are omitted.' }
$null = Invoke-SafeRequest 'Commit Store submission' @{ Uri = "$baseUri/submissions/$submissionId/commit"; Method = 'Post'; Headers = $headers; ContentType = 'application/json'; TimeoutSec = 60 }
$report.status = 'CommitStarted'
Save-Report
$accepted = $false
for ($attempt = 0; $attempt -lt 30; $attempt++) {
    $status = Invoke-SafeRequest 'Check Store submission status' @{ Uri = "$baseUri/submissions/$submissionId/status"; Headers = $headers; TimeoutSec = 60 }
    $report.status = $status.status
    Save-Report
    Write-Host "Store submission $submissionId status: $($report.status)"
    if ($report.status -in @('PreProcessing', 'Certification', 'CertificationPassed', 'Release', 'Published', 'ReadyForRelease')) {
        Write-Host 'Microsoft accepted the submission for processing. Publication remains subject to certification.'
        $accepted = $true
        break
    }
    if ($report.status -ne 'CommitStarted') { throw "Store submission stopped at $($report.status). Review Partner Center." }
    Start-Sleep -Seconds 20
}
if (-not $accepted) { throw 'Store commit is still processing. Inspect the retained submission ID before retrying; no draft was deleted.' }
