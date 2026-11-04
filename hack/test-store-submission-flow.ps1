#Requires -Version 7.0
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = Join-Path (Split-Path -Parent $PSScriptRoot) ('.cache/store-submission-test/' + [guid]::NewGuid().ToString('N'))
$null = New-Item -ItemType Directory -Force -Path $root
$bundle = Join-Path $root 'skopeo-win-1.0.8.0.msixbundle'
[IO.File]::WriteAllText($bundle, 'Synthetic package bytes; package validation belongs to the build workflow.')
$hash = (Get-FileHash -LiteralPath $bundle -Algorithm SHA256).Hash.ToLowerInvariant()
[IO.File]::WriteAllText("$bundle.sha256", "$hash  skopeo-win-1.0.8.0.msixbundle`n")
$saved = @{}
foreach ($name in @('AZURE_AD_TENANT_ID','AZURE_AD_APPLICATION_CLIENT_ID','AZURE_AD_APPLICATION_SECRET','GITHUB_ACTIONS')) {
    $saved[$name] = [Environment]::GetEnvironmentVariable($name)
}
$env:AZURE_AD_TENANT_ID = '11111111-1111-1111-1111-111111111111'
$env:AZURE_AD_APPLICATION_CLIENT_ID = '22222222-2222-2222-2222-222222222222'
$env:AZURE_AD_APPLICATION_SECRET = 'synthetic-client-secret'
$env:GITHUB_ACTIONS = 'false'
$storeModel = @{calls = [Collections.Generic.List[string]]::new(); scenario = ''; uploaded = $false}
$api = 'https://manage.devcenter.microsoft.com/v1.0/my/applications/9MT5QPKZDHGD'

# These command replacements exist only in this test script's scope. Every
# request is handled by the model; unknown URLs and methods fail closed.
function Invoke-RestMethod {
    [CmdletBinding()]
    param($Uri, $Method = 'Get', $Headers, $Body, $ContentType, $TimeoutSec, $MaximumRedirection)
    $storeModel.calls.Add("$Method $Uri")
    if ($MaximumRedirection -ne 0) { throw 'Redirects must be disabled' }
    if ($Uri -eq "https://login.microsoftonline.com/$env:AZURE_AD_TENANT_ID/oauth2/token" -and $Method -eq 'Post') {
        if ($Body.client_secret -cne 'synthetic-client-secret') { throw 'Authentication body changed' }
        return @{ access_token = 'synthetic-access-token' }
    }
    if ($Headers.Authorization -cne 'Bearer synthetic-access-token') { throw 'Missing bearer authorization' }
    switch ("$Method $Uri") {
        "Get $api" {
            if ($storeModel.scenario -eq 'api failure') { throw 'synthetic-sensitive-response-body' }
            $app = @{id='9MT5QPKZDHGD';packageIdentityName='RambowCloud.skopeo-win';lastPublishedApplicationSubmission=@{id='123'}}
            if ($storeModel.scenario -eq 'pending') { $app.pendingApplicationSubmission = @{id='999'} }
            return $app
        }
        "Get $api/submissions/123" { return @{applicationPackages=@(@{version='1.0.7.0'})} }
        "Post $api/submissions" {
            return @{
                id='456';status='PendingCommit';fileUploadUrl='https://synthetic.blob.core.windows.net/upload?sig=synthetic-sas'
                applicationPackages=@(@{version='1.0.7.0';fileName='old.msix';fileStatus='Uploaded'})
                listings=@{'en-us'=@{baseListing=@{title='Keep title';description='Keep description';releaseNotes='Old notes'}}}
                pricing=@{priceId='Free'};notesForCertification='Keep certification explanation'
            }
        }
        "Put $api/submissions/456" {
            $update = $Body | ConvertFrom-Json -AsHashtable
            if ($update.listings['en-us'].baseListing.title -cne 'Keep title' -or
                $update.notesForCertification -cne 'Keep certification explanation' -or
                $update.applicationPackages[1].fileName -cne 'skopeo-win-1.0.8.0.msixbundle') { throw 'Metadata preservation failed' }
            return @{}
        }
        "Post $api/submissions/456/commit" {
            if (-not $storeModel.uploaded) { throw 'Commit preceded successful upload' }
            return @{}
        }
        "Get $api/submissions/456/status" { return @{status='PreProcessing'} }
        default { throw "Unexpected request in offline model: $Method $Uri" }
    }
}

function Invoke-WebRequest {
    [CmdletBinding()]
    param($Uri, $Method, $InFile, $Headers, $ContentType, $MaximumRedirection, $TimeoutSec)
    $storeModel.calls.Add('Upload')
    if ($storeModel.scenario -eq 'upload failure') { throw 'synthetic-sensitive-upload-url' }
    if ($Uri.AbsoluteUri -cne 'https://synthetic.blob.core.windows.net/upload?sig=synthetic-sas' -or
        $Method -ne 'Put' -or $Headers['x-ms-blob-type'] -ne 'BlockBlob' -or $MaximumRedirection -ne 0) { throw 'Unsafe upload parameters' }
    $zip = [IO.Compression.ZipFile]::OpenRead($InFile)
    try {
        if ($zip.Entries.Count -ne 1 -or $zip.Entries[0].FullName -cne 'skopeo-win-1.0.8.0.msixbundle') { throw 'Wrong upload ZIP contents' }
    } finally { $zip.Dispose() }
    $storeModel.uploaded = $true
}

try {
    foreach ($case in @('success','pending','upload failure','api failure')) {
        $storeModel.scenario = $case
        $storeModel.calls.Clear()
        $storeModel.uploaded = $false
        $out = Join-Path $root ($case -replace ' ', '-')
        $caught = $null
        try { & (Join-Path $PSScriptRoot 'submit-store.ps1') -Bundle $bundle -Version '1.0.8' -OutputDirectory $out }
        catch { $caught = $_.Exception.Message }
        $commits = @($storeModel.calls | Where-Object { $_ -eq "Post $api/submissions/456/commit" }).Count
        if ($case -eq 'success') {
            $report = Get-Content -Raw (Join-Path $out 'submission.json') | ConvertFrom-Json
            if ($caught -or $commits -ne 1 -or $report.status -cne 'PreProcessing' -or $report.submissionId -cne '456') { throw "Submission model failed: $caught" }
        } else {
            if (-not $caught -or $commits -ne 0 -or $caught -match 'synthetic-sensitive') { throw "Unsafe failure handling for $case" }
            if ($case -eq 'pending' -and @($storeModel.calls | Where-Object {$_ -eq "Post $api/submissions"}).Count -ne 0) { throw 'Existing draft was replaced' }
        }
        Write-Host "Verified offline Store transaction: $case."
    }
} finally {
    foreach ($name in $saved.Keys) { [Environment]::SetEnvironmentVariable($name, $saved[$name]) }
}
