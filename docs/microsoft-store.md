# Microsoft Store distribution

skopeo-win is registered as `RambowCloud.skopeo-win`, published by `RambowCloud`.
The Store ID is `9MT5QPKZDHGD`; the public listing URL is
<https://apps.microsoft.com/detail/9MT5QPKZDHGD>. The listing may be unavailable
until the first submission passes certification.

The package manifest in `packaging/store/AppxManifest.xml.in` contains the exact
Partner Center identity. The publisher is
`CN=7E0D1E09-4C91-4B76-9600-126B6A63FA59`. The package family name is
`RambowCloud.skopeo-win_jw3fm4pvbvwt6`.

## Build in GitHub Actions

Open [Windows binaries](https://github.com/rambow-cloud/skopeo-win/actions/workflows/windows.yml),
select **Run workflow**, and enter an already published independent tag such as
`v1.0.5` in **store_release**. This mode downloads the four immutable release
assets, checks each against its GitHub SHA-256 digest, and builds the Store bundle
without rebuilding or publishing a new portable release. The input is passed
through an environment variable and validated before use.

Download the `microsoft-store-1.0.5` artifact after the run succeeds. Upload
`skopeo-win-1.0.5.0.msixbundle` from that artifact to Partner Center. The artifact
also includes the bundle checksum and an explicit verification report.

Normal successful main builds generate a Store bundle using the allocated release
version and upload it as a separate Actions artifact. Portable GitHub release
assets retain their existing immutable four-file contract. A Store packaging
failure blocks publication of that new main release. PR builds continue to test
the executable; they do not upload development versions to the Store.

## Package layout and verification

The unsigned bundle contains x64 and ARM64 MSIX packages. Each contains the
corresponding verified executable, a native usage-window launcher, project license,
third-party notices, Windows usage instructions, application
manifest, and rasterized project badge. There is no third-party C runtime or
container daemon bundled. The application targets Windows desktop 10.0.19041.0
and newer. It declares `runFullTrust` to run the native console application.

Two execution aliases are registered: `skopeo.exe` and `skopeo-win.exe`. The
application has a visible Start-menu entry that displays terminal usage instructions
and waits for Enter before closing. The aliases invoke `skopeo.exe` directly.
Windows Settings can disable aliases, and an existing executable earlier on PATH
can take precedence.

`AppListEntry="default"` is required by this distribution. Setting it to `none`
creates a headless app that Partner Center rejects without a HeadlessAppBypass
waiver. MakeAppx validation alone does not enforce that Store-specific permission.
The package verifier explicitly rejects hidden entries. The initial 1.0.6.0
bundle was rejected for this reason; upload the newer fixed bundle instead.

Local reproduction, with stable versioned binaries and their checksum files:

```powershell
./build-store.ps1 -Version 1.0.5 -BinaryDirectory dist -Scanner govulncheck
./hack/test-store-package.ps1 -Bundle dist-store/skopeo-win-1.0.5.0.msixbundle -Version 1.0.5
```

The script downloads the Microsoft-owned SDK BuildTools NuGet archive at a pinned
version and verifies its SHA-256 before extraction. No system SDK installation is
required. The example also requires govulncheck v1.9.0 on PATH; Actions installs
it automatically and scans both CLI inputs and both launcher binaries before uploading the bundle.
The CLI scan runs before SDK acquisition or packaging, including historical-release
repackaging. A regression script rejects a scan failure for either CLI architecture.
MakeAppx performs its normal schema and semantic validation; validation
is not disabled. A new staging directory prevents previous package contents from
leaking into a bundle. Existing output bundles are not overwritten.

The verification script checks both architectures, bundle/package identities,
publisher, four-component versions, visible application entry, command alias configuration, capabilities,
logo dimensions, license bytes, and exact executable SHA-256 values. It runs
`--version` and `--help` from the extracted native package and checks that its
launcher prints usage, waits for input, and closes after Enter. This is package-content
and executable validation, not a test of installed package activation. ARM64
execution is only checked on an ARM64 host.

### Third-party notices and historical releases

Each architecture contains `ThirdPartyNotices/README.txt`, `inventory.json`, and
the license, NOTICE, copyright, patent and author files found in its matched
dependency sources. Leading copyright/license comments from Go, assembly and C
header sources are retained in per-component `SOURCE-COPYRIGHTS.txt` files.
Nested module notices remain attributed to their own module. The Go runtime
notices include a conservative superset from the matching Go source tree,
excluding compiler tools and test data.

The inventory is derived from the packaged CLI and launcher's embedded build
information. It records each executable hash, architecture, Go version, linked
module versions, source paths, versioned module information links, notice hashes,
and the vendored module manifest hash. Module license and notice files are copied
byte-for-byte; the root Apache license does not replace third-party terms.

Packaging fails if a linked module's version differs from `vendor/modules.txt`,
a root dependency license is missing, replacement modules need review, or the
binary's Go version differs from the available Go source tree. Historical
repackaging therefore requires matching dependency and Go sources. Use the
corresponding source checkout/toolchain, or publish an updated build; the script
does not substitute current notices for unmatched historical dependencies or
download mutable latest license files.

Package verification regenerates the expected inventory from the extracted
executables and the matched sources, compares every notice byte, and rejects
missing, altered or unexpected files. It also checks that removing or altering
the extracted Go license is rejected in both architectures, restoring the test
copy afterwards. `verification.json` records `thirdPartyNoticesVerified` and
`thirdPartyNoticeRejectionTestsPassed`. The input bundle is not modified.

The package source checkout and matching vendored dependencies are available in
the repository's versioned release tags. This adds files inside the MSIX payload;
the portable release's four-file contract is unchanged.

Before submitting, install a suitably signed test package or register the
development layout on a test machine configured for development. Check the
installed aliases, working directory, stdin/stdout/exit codes, user-selected image
paths, private-registry login/logout, upgrade and uninstall behavior. Run Windows
App Certification Kit. Do not install the unsigned Store submission bundle as an
ordinary signed installer or import a development certificate on production
machines. The verification report explicitly records these outstanding checks.

## First submission and signing

In Partner Center, open the product and choose **Start submission**. Upload the
bundle in **Packages**. Complete pricing and availability, properties, age ratings,
Store listings with actual screenshots, and the `runFullTrust` explanation. Use
`packaging/store/listing.md` as a listing/certification draft and publish the
privacy policy URL after its changes reach main.

When replacing a rejected upload, remove that failed package from the draft and
upload the new incremented bundle. The runFullTrust approval warning is separate
from headless validation: keep that capability for the Win32 executables and
provide the explanation in the listing draft. Do not remove it to hide a warning.

Microsoft signs MSIX packages during publication after certification. A CA-trusted
certificate or SignPath approval is not required for this Store submission. This
does not sign portable executables on GitHub or provide a signed sideload bundle.
See [MSIX package requirements](https://learn.microsoft.com/en-us/windows/apps/publish/publish-your-app/msix/app-package-requirements).

## Later automated submissions

The separate `Microsoft Store update` workflow consumes the versioned Store
artifact from a successful main `Windows binaries` run after its portable release
has been published. It checks the source workflow, branch, repository, current
main SHA, immutable version tag, bundle hash, and verification report, including
successful third-party notice and rejection checks. PR runs
cannot trigger submission. It requires Partner Center API authentication;
reserving a name and obtaining package identity do not grant that permission.

Microsoft's current [Actions guide](https://learn.microsoft.com/en-us/windows/apps/publish/msstore-dev-cli/github-actions)
requires an already published free app, an associated Microsoft Entra tenant,
and an Entra application assigned the Partner Center Manager role. The underlying
[submission API prerequisites](https://learn.microsoft.com/en-us/windows/uwp/monetize/create-and-manage-submissions-using-windows-store-services)
also require creating the first submission in Partner Center and completing its
age-rating questionnaire before API-created submissions.

After that setup, store `AZURE_AD_TENANT_ID`, `AZURE_AD_APPLICATION_CLIENT_ID`,
and `AZURE_AD_APPLICATION_SECRET` as GitHub Actions secrets. `SELLER_ID` is also
needed if using Microsoft's Developer CLI instead of this direct API workflow. The MSA
app ID, package SID, and package publisher GUID are not substitutes for these
credentials. Never paste the client secret into issues, workflow source, or chat.
The workflow uses the documented submission API and never prints tokens, client
secrets, upload SAS URLs, or raw response bodies. It refuses an existing pending
submission rather than deleting or replacing that draft. A new submission copies
the last published metadata; only packages, English release notes, and the
post-certification publication mode are updated. Existing pricing, screenshots,
descriptions, capability explanations, and rollout settings are preserved.

Updates must increase the live package version. The new submission is committed
for certification with immediate publication after approval. API acceptance is
reported separately from a live Store release. The non-secret submission ID,
version, bundle hash and status are retained in the
`microsoft-store-submission-status` artifact. Do not edit an API-created submission
in Partner Center while the API is managing it; see
[Microsoft's submission guidance](https://learn.microsoft.com/en-us/windows/uwp/monetize/manage-app-submissions).

If credentials are missing, this separate workflow fails explicitly without
claiming an upload. After configuring credentials, run `Microsoft Store update`
manually on `main` and provide the successful main Windows run ID as `run_id`.
If a submission already exists after an interrupted attempt, inspect its status
in Partner Center before taking further action; reruns do not delete drafts.

## Artwork

The checked-in PNG icons are rasterized from the badge in `docs/skopeo.svg`.
To regenerate them with a build-only dependency:

```powershell
npm install --prefix .cache/store-assets --ignore-scripts --no-audit --no-fund @resvg/resvg-js@2.6.2
node hack/generate-store-assets.cjs
```
