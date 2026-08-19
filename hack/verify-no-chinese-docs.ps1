[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$excludedDirectories = @(".git", ".cache", "dist")
$documentationPattern = '\.(md|markdown|rst|adoc|txt)(\.in)?$'
$hanPattern = '[\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff]'
$violations = [System.Collections.Generic.List[string]]::new()

Get-ChildItem -LiteralPath $repositoryRoot -Recurse -File | Where-Object {
    $relativePath = [System.IO.Path]::GetRelativePath($repositoryRoot, $_.FullName)
    $segments = $relativePath -split '[\\/]'
    -not ($segments | Where-Object { $_ -in $excludedDirectories }) -and
        $_.Name -match $documentationPattern
} | ForEach-Object {
    $relativePath = [System.IO.Path]::GetRelativePath($repositoryRoot, $_.FullName)
    $lineNumber = 0
    foreach ($line in [System.IO.File]::ReadLines($_.FullName)) {
        $lineNumber++
        if ($line -match $hanPattern) {
            $violations.Add("${relativePath}:${lineNumber}")
        }
    }
}

if ($violations.Count -ne 0) {
    $violations | ForEach-Object { Write-Error "Chinese text found in documentation: $_" }
    throw "Documentation must be English-only."
}

Write-Host "Verified: documentation contains no Chinese text."
