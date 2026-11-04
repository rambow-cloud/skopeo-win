#Requires -Version 7.0
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'store-submission.psm1') -Force
$app = @{ id = '9MT5QPKZDHGD'; packageIdentityName = 'RambowCloud.skopeo-win'; lastPublishedApplicationSubmission = @{ id = '123' } }
Assert-StoreApplication $app
foreach ($change in @(@{id='OTHER'}, @{packageIdentityName='OTHER'}, @{pendingApplicationSubmission=@{id='456'}}, @{lastPublishedApplicationSubmission=$null})) {
    $candidate = $app.Clone()
    foreach ($key in $change.Keys) { $candidate[$key] = $change[$key] }
    $rejected = $false
    try { Assert-StoreApplication $candidate } catch { $rejected = $true }
    if (-not $rejected) { throw 'Unsafe application state was accepted' }
}
$draft = @{
    id = '456'; status = 'PendingCommit'; fileUploadUrl = 'https://example.invalid/synthetic'
    applicationPackages = @(@{fileName='old.msix'; fileStatus='Uploaded'; version='1.0.7.0'})
    listings = @{'en-us' = @{baseListing=@{title='Existing title'; description='Existing description'; releaseNotes='Old notes'; images=@(@{id='image1';fileStatus='Uploaded'})}}}
    pricing = @{priceId='Free'; marketSpecificPricings=@{US='keep'}}
    targetPublishMode='Manual'; notesForCertification='Keep existing capability explanation'
}
$before = $draft | ConvertTo-Json -Depth 100 -Compress
$updated = New-StorePackageUpdate $draft '1.0.8.0' 'skopeo-win-1.0.8.0.msixbundle' 'New fixes'
if (($draft | ConvertTo-Json -Depth 100 -Compress) -cne $before) { throw 'Input metadata was mutated' }
if ($updated.Contains('fileUploadUrl') -or $updated.Contains('id') -or $updated.Contains('status')) { throw 'Read-only or secret metadata leaked into update' }
if ($updated.applicationPackages.Count -ne 2 -or $updated.applicationPackages[0].fileStatus -ne 'PendingDelete' -or
    $updated.applicationPackages[1].fileStatus -ne 'PendingUpload') { throw 'Package replacement state is incorrect' }
foreach ($key in @('pricing','notesForCertification')) {
    if (($updated[$key] | ConvertTo-Json -Depth 100 -Compress) -cne ($draft[$key] | ConvertTo-Json -Depth 100 -Compress)) { throw "Metadata changed: $key" }
}
$listing = $updated.listings['en-us'].baseListing
if ($listing.title -cne 'Existing title' -or $listing.description -cne 'Existing description' -or
    $listing.images[0].id -cne 'image1' -or $listing.releaseNotes -cne 'New fixes') { throw 'Listing preservation failed' }
foreach ($version in @('1.0.7.0', '1.0.6.0')) {
    $rejected = $false
    try { $null = New-StorePackageUpdate $draft $version "skopeo-win-$version.msixbundle" 'notes' } catch { $rejected = $true }
    if (-not $rejected) { throw 'Non-increasing package version was accepted' }
}
Write-Host 'Verified Store identity, pending-draft protection, monotonic versions, package transitions and metadata preservation.'
