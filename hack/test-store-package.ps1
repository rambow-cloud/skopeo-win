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

function Test-NoticeRejections([string] $Layout) {
    # Modify only the extracted verification copy, never the input bundle.
    $license = Join-Path $Layout "ThirdPartyNotices/go/LICENSE"
    $original = [IO.File]::ReadAllBytes($license)
    try {
        foreach ($scenario in @("missing", "changed")) {
            if ($scenario -eq "missing") {
                Remove-Item -LiteralPath $license
            } else {
                [IO.File]::WriteAllText($license, "Synthetic altered notice for package verification.")
            }
            $diagnostics = & go -C $repositoryRoot run ./cmd/store-notices -repository $repositoryRoot -layout $Layout -verify 2>&1
            if ($LASTEXITCODE -eq 0 -or ($diagnostics -join "`n") -notmatch '(missing notice file|notice content differs from matching sources)') {
                throw "Package verification did not reject the $scenario third-party license: $diagnostics"
            }
            [IO.File]::WriteAllBytes($license, $original)
        }
    } finally {
        [IO.File]::WriteAllBytes($license, $original)
    }
    $global:LASTEXITCODE = 0
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
    # MakeAppx accepts headless manifests, but Partner Center requires a waiver.
    Assert-Equal $application.VisualElements.GetAttribute("AppListEntry") "default" "Store AppListEntry must be default; headless applications require a waiver"
    Assert-Equal $application.Executable "store-launcher.exe" "Wrong visible application executable"
    Assert-Equal $application.EntryPoint "Windows.FullTrustApplication" "Wrong desktop entry point"
    $desktopNamespace = "http://schemas.microsoft.com/appx/manifest/desktop/windows10/4"
    Assert-Equal $application.GetAttribute("Subsystem", $desktopNamespace) "console" "Missing console subsystem"
    Assert-Equal $application.GetAttribute("SupportsMultipleInstances", $desktopNamespace) "true" "Concurrent CLI invocations must be supported"
    $aliases = @($application.Extensions.Extension.AppExecutionAlias.ExecutionAlias | ForEach-Object { $_.Alias } | Sort-Object)
    Assert-Equal ($aliases -join ',') "skopeo-win.exe,skopeo.exe" "Wrong command aliases"
    Assert-Equal $application.Extensions.Extension.Executable "skopeo.exe" "Alias targets the wrong executable"
    Assert-Equal $application.Extensions.Extension.EntryPoint "Windows.FullTrustApplication" "Wrong alias entry point"
    $capabilities = @($manifest.Package.Capabilities.Capability | ForEach-Object { $_.Name })
    Assert-Equal ($capabilities -join ',') "runFullTrust" "Unexpected application capabilities"
    $sourceArch = if ($arch -eq "x64") { "amd64" } else { "arm64" }
    $sourceBinary = Join-Path $binaryRoot "skopeo-windows-$sourceArch.exe"
    $packagedBinary = Join-Path $layout "skopeo.exe"
    $sourceHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $sourceBinary).Hash
    $packagedHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $packagedBinary).Hash
    Assert-Equal $packagedHash $sourceHash "Packaged executable differs from the verified release"
    $launcher = Join-Path $layout "store-launcher.exe"
    & go run (Join-Path $repositoryRoot "hack/verify-windows-binary.go") $launcher $sourceArch
    if ($LASTEXITCODE -ne 0) { throw "Wrong Store launcher architecture or runtime dependencies" }
    Assert-Equal (Get-Content -Raw -LiteralPath (Join-Path $layout "LICENSE")) (Get-Content -Raw -LiteralPath (Join-Path $repositoryRoot "LICENSE")) "License changed during packaging"
    & go -C $repositoryRoot run ./cmd/store-notices -repository $repositoryRoot -layout $layout -verify
    if ($LASTEXITCODE -ne 0) { throw "Third-party notice inventory or package content differs from matching sources: $arch" }
    Test-NoticeRejections $layout
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

$launcherInfo = New-Object Diagnostics.ProcessStartInfo
$launcherInfo.FileName = Join-Path $layouts[$hostArchitecture] "store-launcher.exe"
$launcherInfo.UseShellExecute = $false
$launcherInfo.CreateNoWindow = $true
$launcherInfo.RedirectStandardInput = $true
$launcherInfo.RedirectStandardOutput = $true
$launcherInfo.RedirectStandardError = $true
$launcherProcess = [Diagnostics.Process]::Start($launcherInfo)
try {
    $launcherOutput = $launcherProcess.StandardOutput.ReadToEndAsync()
    $launcherError = $launcherProcess.StandardError.ReadToEndAsync()
    if ($launcherProcess.WaitForExit(250)) { throw "Store launcher exited before receiving input" }
    $launcherProcess.StandardInput.WriteLine()
    $launcherProcess.StandardInput.Close()
    if (-not $launcherProcess.WaitForExit(10000)) { throw "Store launcher did not exit after Enter" }
    Assert-Equal $launcherProcess.ExitCode 0 "Store launcher failed"
    Assert-Equal $launcherError.GetAwaiter().GetResult() "" "Unexpected Store launcher error"
    $text = $launcherOutput.GetAwaiter().GetResult()
    if (-not $text.Contains("skopeo-win $Version") -or -not $text.Contains("skopeo-win --help") -or
        -not $text.Contains("Press Enter to close this window")) {
        throw "Store launcher did not display its version, terminal usage and close instructions"
    }
} finally {
    if (-not $launcherProcess.HasExited) { $launcherProcess.Kill() }
    $launcherProcess.Dispose()
}

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
    visibleApplicationEntryVerified = $true
    thirdPartyNoticesVerified = $true
    thirdPartyNoticeRejectionTestsPassed = $true
    nativeExecutableSmokeTest = $hostArchitecture
    nativeLauncherInputTest = $hostArchitecture
    installedPackageTested = $false
    activatedExecutionAliasesTested = $false
    windowsAppCertificationKitRun = $false
}
$reportPath = Join-Path (Split-Path -Parent $bundlePath) "verification.json"
$report | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $reportPath -Encoding utf8
Write-Host "Verified both package identities, visible entries, architectures, aliases, logos, licenses and executable hashes."
Write-Host "Extracted $hostArchitecture CLI passed --version/--help; its launcher waited for input and closed on Enter. Installed activation and WACK still require validation."
