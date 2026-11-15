#Requires -Version 7.0
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$repository = Split-Path -Parent $PSScriptRoot
Push-Location $repository
$oldCgo = $env:CGO_ENABLED
try {
    $env:CGO_ENABLED = '0'
    $sources = @(Get-Content -Raw -LiteralPath packaging/store/module-sources.json | ConvertFrom-Json)
    $configuration = go mod edit -json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0) { throw 'Cannot read module configuration' }
    if (@($configuration.Replace).Count -ne $sources.Count) { throw 'Every module replacement requires source review' }
    foreach ($source in $sources) {
        if ($source.repository -cne 'https://github.com/rambow-cloud/container-libs' -or
            $source.upstreamRepository -cne 'https://github.com/podman-container-tools/container-libs' -or
            $source.directory -cnotin @('image','common') -or
            $source.commit -cnotmatch '^[0-9a-f]{40}$' -or $source.upstreamCommit -cnotmatch '^[0-9a-f]{40}$' -or
            $source.replacement.version -cnotmatch '^v\d+\.\d+\.\d+-skopeo\.\d+$' -or
            $source.version -cnotmatch '^v\d+\.\d+\.\d+$') { throw 'Invalid reviewed dependency source' }
        $replacement = @($configuration.Replace | Where-Object { $_.Old.Path -ceq $source.module })
        $requirement = @($configuration.Require | Where-Object { $_.Path -ceq $source.module })
        if ($replacement.Count -ne 1 -or $requirement.Count -ne 1 -or
            $replacement[0].New.Path -cne $source.replacement.path -or $replacement[0].New.Version -cne $source.replacement.version -or
            $requirement[0].Version -cne $source.version) { throw 'Module configuration differs from source review' }
        $tag = [uri]::EscapeDataString($source.directory + '/' + $source.replacement.version)
        $commit = gh api "repos/rambow-cloud/container-libs/commits/$tag" --jq .sha
        if ($LASTEXITCODE -ne 0 -or $commit -cne $source.commit) { throw 'Reviewed fork tag no longer resolves to its pinned commit' }
        $upstreamTag = [uri]::EscapeDataString($source.directory + '/' + $source.version)
        $upstreamCommit = gh api "repos/podman-container-tools/container-libs/commits/$upstreamTag" --jq .sha
        if ($LASTEXITCODE -ne 0 -or $upstreamCommit -cne $source.upstreamCommit) { throw 'Upstream provenance does not match the reviewed release' }
        $digest = go run ./cmd/source-digest ("vendor/" + $source.module)
        if ($LASTEXITCODE -ne 0 -or $digest -cne $source.vendorSHA256) { throw 'Vendored dependency content differs from its reviewed snapshot' }
        Write-Host "Verified reviewed source $($source.module) -> $($source.replacement.path)@$($source.replacement.version)."
    }
    $graph = @(go mod graph)
    if ($LASTEXITCODE -ne 0 -or @($graph | Where-Object { $_ -match 'github.com/(docker|distribution)/distribution@' }).Count) {
        throw 'Legacy Distribution remains in the complete module graph'
    }
    $packages = @(go list -mod=vendor -tags 'containers_image_openpgp,exclude_graphdriver_btrfs' -deps ./cmd/skopeo)
    if ($LASTEXITCODE -ne 0 -or @($packages | Where-Object { $_ -match '^github.com/(docker|distribution)/distribution(/|$)' -and $_ -notmatch '^github.com/distribution/distribution/v3/' }).Count) {
        throw 'Legacy Distribution remains in the production import graph'
    }
    git diff --exit-code -- go.mod go.sum
    if ($LASTEXITCODE -ne 0) { throw 'Source verification changed the recorded module graph' }
    Write-Host 'Verified complete module and executable import graphs exclude legacy Distribution.'
} finally {
    $env:CGO_ENABLED = $oldCgo
    Pop-Location
}
