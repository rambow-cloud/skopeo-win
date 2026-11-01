# Windows vulnerability remediation (2026-11-02)

Tracking issue: [#1](https://github.com/rambow-cloud/skopeo-win/issues/1).

## Scope and baseline

The scan uses govulncheck v1.9.0 and the live Go vulnerability database. Production source is analyzed for Windows with CGO disabled and the `containers_image_openpgp exclude_graphdriver_btrfs` build tags. Release executables are also scanned directly.

The original v1.23.0 amd64 executable, built with Go 1.25.6, returned 76 symbol-level advisory matches and one additional Fulcio module-only finding. Because this executable was stripped, govulncheck conservatively matched module contents rather than actual binary symbols. These counts are advisory matches, not 76 proven exploitable vulnerabilities or necessarily 76 CVEs. Baseline source analysis with Go 1.27.1 could not load the old grpc/x/net API combination.

## Remediation

| Component | Before | After | Representative advisory |
| --- | --- | --- | --- |
| Go compiler and standard library | 1.25.6 in CI | 1.27.3 | [GO-2026-6809 / CVE-2026-111943](https://pkg.go.dev/vuln/GO-2026-6809) and other standard-library findings |
| OpenPGP | Unmaintained x/crypto/openpgp | ProtonMail/go-crypto 1.5.2 via upstream image 5.42.0 and ocicrypt 1.3.2 | [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932) |
| x/crypto | 0.51.0 | 0.58.0 | SSH findings fixed upstream |
| x/net | 0.54.0 | 0.62.0 | HTTP/2, IDNA, and HTML findings fixed upstream |
| x/text | 0.37.0 | 0.43.0 | [GO-2026-6629 / CVE-2026-56851](https://pkg.go.dev/vuln/GO-2026-6629) |
| grpc | 1.80.0 | 1.83.2 | [GO-2026-6443 / CVE-2026-84445](https://pkg.go.dev/vuln/GO-2026-6443), [GO-2026-6348 / CVE-2026-84304](https://pkg.go.dev/vuln/GO-2026-6348) |
| klauspost/compress | 1.18.6 | 1.20.3 | [GO-2026-5841 / GHSA-259r-337f-4rfw](https://pkg.go.dev/vuln/GO-2026-5841) |
| OpenTelemetry | 1.43.0 | 1.46.0 | [GO-2026-5158 / CVE-2026-41178](https://pkg.go.dev/vuln/GO-2026-5158) |
| Fulcio | 1.8.5 | 1.8.8 | [GO-2026-5853 / CVE-2026-49478](https://pkg.go.dev/vuln/GO-2026-5853), baseline module-only finding |

Dependency versions and vendored sources are regenerated together. OpenPGP changes come from maintained upstream releases rather than a local cryptographic implementation.

## Results and residual finding

- Updated production source: zero reachable vulnerabilities and zero imported-package findings.
- Updated Windows amd64 and arm64 executables: zero package-level findings in both binaries.
- Both binaries retain symbols, are built with Go 1.27.3 and CGO disabled, and import only `kernel32.dll` in their PE loader import tables.
- Signature regression tests accept a valid signature and reject altered, unsigned, untrusted, and expired inputs.
- Upstream ocicrypt PGP key-wrapping tests pass with the upgraded module versions.
- Command unit tests pass locally with the previously identified Windows symlink-privilege test `TestGenerateSigstoreKey` excluded. CI runs the complete command suite.

GO-2026-5932 remains a module-level advisory because other maintained packages from `golang.org/x/crypto` are still needed. The unsafe `golang.org/x/crypto/openpgp` package and all its subpackages are absent from the production dependency graph and both symbol-preserving binaries. The advisory has no fixed x/crypto version. No advisory suppression is used: reports retain the module-only finding, while source and binary package checks verify the vulnerable package is absent.

The original PE verifier used `debug/pe.ImportedLibraries`, which is a stub returning no entries. It now uses actual `ImportedSymbols` loader entries, so the historical empty import-table output must not be used as evidence of no DLL imports.

## Release gate and tooling

CI builds both architectures, runs unit tests, scans production source and binary packages, and retains scan reports. Publication is a separate job with write permission and depends on the read-only build job succeeding. Release versions remain aligned with `version/version.go`; rebuilding the same version replaces its release assets under the repository's existing policy.

Official Actions are pinned to verified release commits: checkout 7.0.1, setup-go 7.0.0, upload-artifact 7.0.2, download-artifact 8.0.2, and stale 11.0.0. Inherited upstream-only Cirrus monitoring and issue-lock jobs are guarded so they do not run against this Windows distribution.

Scans cover the known Go database findings for the specified database snapshot and build configuration. They are not a guarantee against undisclosed vulnerabilities, nor a full audit of excluded Linux code or external credential helpers.
