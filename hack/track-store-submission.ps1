#Requires -Version 7.0
param([Parameter(Mandatory)] [string] $Report, [Parameter(Mandatory)] [string] $ExpectedVersion)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'store-publication.psm1') -Force
if (-not (Test-Path -LiteralPath $Report)) { Write-Host 'No Store submission report to register.'; return }
$data = Get-Content -Raw -LiteralPath $Report | ConvertFrom-Json -AsHashtable
if (-not $data['submissionId']) { Write-Host 'No Store submission was created.'; return }
if ($data['productId'] -cne '9MT5QPKZDHGD' -or $data['version'] -cne "$ExpectedVersion.0" -or
    $data['bundleSHA256'] -cnotmatch '^[a-f0-9]{64}$') { throw 'Store report differs from the verified package' }
$record = New-StorePublicationRecord $data['version'] $data['submissionId']
$tracker = Register-StorePublication $record @(Get-StorePublicationIssues)
Write-Host "Store publication tracked in issue #$($tracker.issue.number)."
