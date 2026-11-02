[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)] [string] $Bundle,
    [Parameter(Mandatory = $true)] [string] $Version,
    [string] $BinaryDirectory = "dist"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.IO.Compression.FileSystem
Add-Type -AssemblyName System.Drawing
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$bundlePath = (Resolve-Path -LiteralPath $Bundle).Path
$binaryRoot = if ([IO.Path]::IsPathRooted($BinaryDirectory)) { $BinaryDirectory } else { Join-Path $repositoryRoot $BinaryDirectory }
$verificationRoot = Join-Path $repositoryRoot (".cache/store-verification/" + [guid]::NewGuid().ToString("N"))
[IO.Compression.ZipFile]::ExtractToDirectory($bundlePath, $verificationRoot)

function Assert-Equal($Actual, $Expected, [string] $Message) {
    if ($Actual -cne $Expected) { throw "$Message (actual: $Actual; expected: $Expected)" }
}

$expectedName = "RambowCloud.skopeo-win"
$expectedPublisher = "CN=7E0D1E09-4C91-4B76-9600-126B6A63FA59"
$packageVersion = "$Version.0"
[xml] $bundleManifest = Get-Content -Raw -LiteralPath (Join-Path $verificationRoot "AppxMetadata/AppxBundleManifest.xml")
Assert-Equal $bundleManifest.Bundle.Identity.Name $expectedName "Wrong bundle identity"
Assert-Equal $bundleManifest.Bundle.Identity.Publisher $expectedPublisher "Wrong bundle publisher"
Assert-Equal $bundleManifest.Bundle.Identity.Version $packageVersion "Wrong bundle version"
$descriptors = @($bundleManifest.Bundle.Packages.Package)
Assert-Equal $descriptors.Count 2 "Bundle must contain exactly two application packages"
$hashes = [ordered]@{}
$layouts = @{}
foreach ($arch in @("x64", "arm64")) {
    $fileName = "skopeo-win-$packageVersion-$arch.msix"
    $descriptor = @($descriptors | Where-Object { $_.Architecture -ceq $arch })
    Assert-Equal $descriptor.Count 1 "Missing or duplicate bundle architecture"
    Assert-Equal $descriptor[0].FileName $fileName "Wrong package filename"
    Assert-Equal $descriptor[0].Type "application" "Unexpected resource-only package"
    $layout = Join-Path $verificationRoot $arch
    [IO.Compression.ZipFile]::ExtractToDirectory((Join-Path $verificationRoot $fileName), $layout)
    $layouts[$arch] = $layout
    [xml] $manifest = Get-Content -Raw -LiteralPath (Join-Path $layout "AppxManifest.xml")
    Assert-Equal $manifest.Package.Identity.Name $expectedName "Wrong package identity"
    Assert-Equal $manifest.Package.Identity.Publisher $expectedPublisher "Wrong package publisher"
    Assert-Equal $manifest.Package.Identity.Version $packageVersion "Wrong package version"
    Assert-Equal $manifest.Package.Identity.ProcessorArchitecture $arch "Wrong package architecture"
    Assert-Equal $manifest.Package.Properties.PublisherDisplayName "RambowCloud" "Wrong publisher display name"
    $application = $manifest.Package.Applications.Application
    Assert-Equal $application.Executable "skopeo.exe" "Wrong application executable"
    Assert-Equal $application.EntryPoint "Windows.FullTrustApplication" "Wrong desktop entry point"
    $desktopNamespace = "http://schemas.microsoft.com/appx/manifest/desktop/windows10/4"
    Assert-Equal $application.GetAttribute("Subsystem", $desktopNamespace) "console" "Missing console subsystem"
    Assert-Equal $application.GetAttribute("SupportsMultipleInstances", $desktopNamespace) "true" "Concurrent CLI invocations must be supported"
    $aliases = @($application.Extensions.Extension.AppExecutionAlias.ExecutionAlias | ForEach-Object { $_.Alias } | Sort-Object)
    Assert-Equal ($aliases -join ',') "skopeo-win.exe,skopeo.exe" "Wrong command aliases"
    Assert-Equal $application.Extensions.Extension.Executable "skopeo.exe" "Alias targets the wrong executable"
    $capabilities = @($manifest.Package.Capabilities.Capability | ForEach-Object { $_.Name })
    Assert-Equal ($capabilities -join ',') "runFullTrust" "Unexpected application capabilities"
    $sourceArch = if ($arch -eq "x64") { "amd64" } else { "arm64" }
    $sourceBinary = Join-Path $binaryRoot "skopeo-windows-$sourceArch.exe"
    $packagedBinary = Join-Path $layout "skopeo.exe"
    $sourceHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $sourceBinary).Hash
    $packagedHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $packagedBinary).Hash
    Assert-Equal $packagedHash $sourceHash "Packaged executable differs from the verified release"
    Assert-Equal (Get-Content -Raw -LiteralPath (Join-Path $layout "LICENSE")) (Get-Content -Raw -LiteralPath (Join-Path $repositoryRoot "LICENSE")) "License changed during packaging"
    foreach ($asset in @(@("StoreLogo", 50), @("Square44x44Logo", 44), @("Square150x150Logo", 150))) {
        $image = [Drawing.Image]::FromFile((Join-Path $layout "Assets/$($asset[0]).png"))
        try {
            Assert-Equal $image.Width $asset[1] "Wrong logo width"
            Assert-Equal $image.Height $asset[1] "Wrong logo height"
        } finally { $image.Dispose() }
    }
    $hashes[$arch] = $packagedHash.ToLowerInvariant()
}

$hostArchitecture = [Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
if (-not $layouts.ContainsKey($hostArchitecture)) { throw "No native package for test host: $hostArchitecture" }
$nativeBinary = Join-Path $layouts[$hostArchitecture] "skopeo.exe"
$actualVersion = & $nativeBinary --version
if ($LASTEXITCODE -ne 0) { throw "Packaged executable failed --version" }
Assert-Equal ($actualVersion -join "`n") "skopeo version $Version" "Executable version differs from the Store package"
$help = & $nativeBinary --help
if ($LASTEXITCODE -ne 0 -or ($help -join "`n") -notmatch 'Usage:') { throw "Packaged executable failed --help" }

$bundleHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $bundlePath).Hash.ToLowerInvariant()
$checksum = (Get-Content -Raw -LiteralPath "$bundlePath.sha256").Trim() -split '\s+'
Assert-Equal $checksum.Count 2 "Malformed bundle checksum"
Assert-Equal $checksum[0] $bundleHash "Wrong bundle checksum"
Assert-Equal $checksum[1] ([IO.Path]::GetFileName($bundlePath)) "Wrong checksum filename"
$report = [ordered]@{
    packageVersion = $packageVersion
    bundleSHA256 = $bundleHash
    packagedExecutableSHA256 = $hashes
    manifestAndContentVerified = $true
    nativeExecutableSmokeTest = $hostArchitecture
    installedPackageTested = $false
    activatedExecutionAliasesTested = $false
    windowsAppCertificationKitRun = $false
}
$reportPath = Join-Path (Split-Path -Parent $bundlePath) "verification.json"
$report | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $reportPath -Encoding utf8
Write-Host "Verified both package identities, architectures, aliases, logos, licenses and executable hashes."
Write-Host "Extracted $hostArchitecture executable passed --version and --help. Installation, activated aliases and WACK still require validation."
