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
corresponding verified executable, license, Windows usage instructions, application
manifest, and rasterized project badge. There is no third-party C runtime or
container daemon bundled. The application targets Windows desktop 10.0.19041.0
and newer. It declares `runFullTrust` to run the native console application.

Two execution aliases are registered: `skopeo.exe` and `skopeo-win.exe`. The
application has no Start-menu entry; use a terminal. Windows Settings can disable
aliases, and an existing executable earlier on PATH can take precedence.

Local reproduction, with stable versioned binaries and their checksum files:

```powershell
./build-store.ps1 -Version 1.0.5 -BinaryDirectory dist
./hack/test-store-package.ps1 -Bundle dist-store/skopeo-win-1.0.5.0.msixbundle -Version 1.0.5
```

The script downloads the Microsoft-owned SDK BuildTools NuGet archive at a pinned
version and verifies its SHA-256 before extraction. No system SDK installation is
required. MakeAppx performs its normal schema and semantic validation; validation
is not disabled. A new staging directory prevents previous package contents from
leaking into a bundle. Existing output bundles are not overwritten.

The verification script checks both architectures, bundle/package identities,
publisher, four-component versions, command alias configuration, capabilities,
logo dimensions, license bytes, and exact executable SHA-256 values. It runs
`--version` and `--help` from the extracted native package. This is package-content
and executable validation, not a test of installed package activation. ARM64
execution is only checked on an ARM64 host.

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

Microsoft signs MSIX packages during publication after certification. A CA-trusted
certificate or SignPath approval is not required for this Store submission. This
does not sign portable executables on GitHub or provide a signed sideload bundle.
See [MSIX package requirements](https://learn.microsoft.com/en-us/windows/apps/publish/publish-your-app/msix/app-package-requirements).

## Later automated submissions

Actions artifact upload is implemented; Partner Center authentication and direct
submission are not configured. Reserving a name and obtaining package identity
does not grant the workflow permission to submit the app.

Microsoft's current [Actions guide](https://learn.microsoft.com/en-us/windows/apps/publish/msstore-dev-cli/github-actions)
requires an already published free app, an associated Microsoft Entra tenant,
and an Entra application assigned the Partner Center Manager role. The underlying
[submission API prerequisites](https://learn.microsoft.com/en-us/windows/uwp/monetize/create-and-manage-submissions-using-windows-store-services)
also require creating the first submission in Partner Center and completing its
age-rating questionnaire before API-created submissions.

After that setup, store `AZURE_AD_TENANT_ID`, `AZURE_AD_APPLICATION_CLIENT_ID`,
`AZURE_AD_APPLICATION_SECRET`, and `SELLER_ID` as GitHub Actions secrets. The MSA
app ID, package SID, and package publisher GUID are not substitutes for these
credentials. Never paste the client secret into issues, workflow source, or chat.
Configure the Microsoft Store Developer CLI only after this authorization exists,
and decide whether updates should remain drafts or be committed for certification.

## Artwork

The checked-in PNG icons are rasterized from the badge in `docs/skopeo.svg`.
To regenerate them with a build-only dependency:

```powershell
npm install --prefix .cache/store-assets --ignore-scripts --no-audit --no-fund @resvg/resvg-js@2.6.2
node hack/generate-store-assets.cjs
```
