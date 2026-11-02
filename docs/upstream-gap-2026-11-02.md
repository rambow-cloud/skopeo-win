# Upstream gap assessment: 2026-11-02

## Baselines and method

The independent Windows release before this work is **skopeo-win v1.0.0**, main commit `3cab771c74bf7264265188d040fe85bd79c9064f`. Its original source baseline is upstream **v1.23.0**, commit `9645b282ca2d8792235db5a3e142ea9ae2f0c63b`.

The latest official upstream GitHub Release at assessment time is [v1.24.1](https://github.com/podman-container-tools/skopeo/releases/tag/v1.24.1), published September 16, 2026, commit `77f3d92f861271c7cb9175afcf876017bcae9202`. The separate development snapshot is upstream main `48f771712127522e3073bac8dd6e51d1059ca41e`, whose source version is `1.26.0-dev`. Development-branch changes are not represented as released stable features.

The upstream history has 75 commits from v1.23.0 to v1.24.1 and 134 more from v1.24.1 to the pinned main snapshot (counts include merges). These are upstream ancestry counts, not the number of missing features in this fork. Our independent dependency, security, Windows, and release changes already cover part of the difference. Our own v1.0.x numbering does not imply an upstream downgrade.

The audit compared command sources and tests, manuals, module manifests, vendored APIs, platform-specific code, build configuration, and automation. Reproduce the source comparison with:

```powershell
git diff v1.23.0 v1.24.1 -- cmd/skopeo docs go.mod
git log --oneline v1.24.1..48f771712127522e3073bac8dd6e51d1059ca41e -- cmd/skopeo docs
git diff v1.23.0 48f771712127522e3073bac8dd6e51d1059ca41e -- . ':!vendor' ':!go.sum'
```

## Functional gaps and selected ports

| Area | Upstream source | Before this work | Resolution / tracking |
| --- | --- | --- | --- |
| Selective signature removal | Stable, [51566c51](https://github.com/podman-container-tools/skopeo/commit/51566c51) | Only removal of all signatures exposed | Add `--remove-list-signatures` to copy/sync; reject conflicting removal flags. [Issue #5](https://github.com/rambow-cloud/skopeo-win/issues/5) |
| Dense platform subsets | Same stable change | Platform selection exists, but leaves a sparse list | Add copy `--strip-removed-platforms`; preserve the default. Signed-list and digest protections remain enforced. [Issue #5](https://github.com/rambow-cloud/skopeo-win/issues/5) |
| Foreign-layer downloads | Development main, [f1131544](https://github.com/podman-container-tools/skopeo/commit/f11315440f5a219475d84dd9795193adbe232e50) and documentation `9084ebaa` | Library supports downloads, CLI does not expose the option | Add opt-in `--download-foreign-layers` to copy/sync. Especially relevant to Windows container images. [Issue #6](https://github.com/rambow-cloud/skopeo-win/issues/6) |
| Maintained YAML parser | Stable, `02dddcbd`, `b76c6226` | Sync still imports `gopkg.in/yaml.v3` 3.0.1 | Use `go.yaml.in/yaml/v3` 3.0.5, removing the unused old module and its vendor copy. [Issue #7](https://github.com/rambow-cloud/skopeo-win/issues/7) |
| Remaining common dependency | Stable, `c415e84a` | common 0.68.0 | Update to 0.69.2 without downgrading newer libraries. [Issue #7](https://github.com/rambow-cloud/skopeo-win/issues/7) |
| Invalid-argument diagnostics | Development main, `0f936053` | Several commands return usage fragments instead of command help | Display full help on stderr plus actionable argument/required-option errors. [Issue #8](https://github.com/rambow-cloud/skopeo-win/issues/8) |
| Public-key write error | Development main, `9ad2b4e7` | Incorrectly identifies the public key as a private key | Correct the error text. [Issue #8](https://github.com/rambow-cloud/skopeo-win/issues/8) |
| Windows cross-platform help | Development main `a154e54a`; stable duration/manual fixes | Missing OS/architecture override example; incomplete duration/archive help | Add inspect override example, duration units, archive syntax fixes, signature option documentation, and consistent manual headings. [Issue #8](https://github.com/rambow-cloud/skopeo-win/issues/8) |

These ports use the existing image v5.42.0 implementation; no upstream branch merge is needed. Removing list signatures does not remove instance signatures. Stripping platforms changes the list digest and fails if signature or preserve-digest requirements prohibit modification. Foreign-layer download defaults to false, and downloading does not grant redistribution rights.

## Dependency comparison

| Component | Fork before | Upstream v1.24.1 | Pinned upstream main | Fork after |
| --- | --- | --- | --- | --- |
| Go directive | 1.27.3 | 1.25.7 | 1.26.0 | 1.27.3 |
| image/v5 | 5.42.0 | 5.41.2 | 5.42.0 | 5.42.0 |
| storage | 1.65.0 | 1.64.1 | 1.65.0 | 1.65.0 |
| common | 0.68.0 | 0.69.2 | 0.70.0 | 0.69.2 |
| ocicrypt | 1.3.2 | 1.3.2 | 1.3.2 | 1.3.2 |
| YAML used by sync | gopkg.in/yaml.v3 3.0.1 | go.yaml.in/yaml/v3 3.0.5 | go.yaml.in/yaml/v3 3.0.5 | go.yaml.in/yaml/v3 3.0.5 |
| x/term | 0.47.0 | 0.45.0 | 0.47.0 | 0.47.0 |
| logrus | 1.10.2 | 1.9.4 | 1.10.2 | 1.10.2 |

Upstream v1.24.1's release notes associate common 0.69.2, image 5.41.2, and storage 1.64.1 with CVE-2025-11395 remediation. Image and storage were already newer here; this work aligns common as well. That release note alone does not establish exploitability of the pure-Go Windows executable by a Podman load vulnerability. Source and package-level binary vulnerability gates remain authoritative for our shipped configuration, with no new suppression.

The main snapshot also has common 0.70.0, moby/sys/capability 0.5.0 and a newer image-spec pseudo-version. They are dependency differences, not additional demonstrated missing CLI features. This change uses the stable common baseline and retains the existing image-spec/capability versions; future upgrades should have their own compatibility and security evidence.

The common update also brings its stdin-password read-error handling into the vendored auth helper. Only packages used by this project are vendored, so the source delta in common is small despite the module version change.

## Covered or deliberately outside Windows scope

| Upstream change group | Decision |
| --- | --- |
| Newer Go, image/storage security dependencies, ocicrypt, x/crypto and x/net updates | Already covered by the previous security work or newer versions; retain those versions. |
| Checkout/setup-go/upload/download Node runtime upgrades | Already covered by pinned current Actions in the Windows pipeline. |
| Cirrus migration, Linux/macOS VM jobs, storage/rootless tests, Packit, pause-image integration fixtures | Upstream infrastructure and operating-system integration; not a Windows executable feature gap. Keep the fork's Windows CI and local synthetic fixtures. |
| logrus-to-slog migration and final error formatting/exit refactoring | Internal logging changes, deferred as a separate behavior review; no missing copy/inspect/sync capability. |
| `new(value)`, `errors.AsType`, linter suppression cleanups | Internal Go/style changes; defer unrelated refactors. |
| Upstream governance, maintainers, contribution policy, support, repository names | Organization-specific; preserve this fork's issue-driven development and security-reporting routes. |
| Upstream source version bumps and upstream release automation | Deliberately retain independent automatically incremented immutable v1.0.x releases. |

## Verification and limits

- CGO-disabled command and release tests exercise the complete existing unit suite plus offline Windows amd64/arm64 platform selection, sparse versus stripped lists, opaque instance-signature preservation, full signature removal, signed-list rejection, and preserve-digest rejection.
- A loopback HTTP server serves a synthetic foreign layer. Default copies preserve the external reference without a download; opt-in copies fetch and store the exact compressed layer. No external registry, Docker daemon, Alpine image, or container execution is required.
- Shared copy/sync flag checks and YAML aliases, tags, regex, credentials, and TLS parsing guard compatibility.
- Windows amd64 and arm64 build verification checks CGO, OpenPGP configuration, PE machine type, and imported DLLs. ARM64 cross-compilation is not an ARM64 runtime test.
- The existing source and both binary vulnerability gates and English-documentation check apply before publication. Published delivery evidence belongs in Issues #5-#8 and the implementing PR.

Local verification passed the complete command/release suite, the expanded copy regression scenarios, dual-architecture PE/build checks, and English-documentation/whitespace checks. Source scanning found zero reachable vulnerabilities; both package-level binary scans found zero applicable vulnerabilities. All three scans retain the existing one module-only advisory, described in [the security assessment](security-scan-2026-11-02.md), without suppressing it.

This is a source and capability assessment at pinned revisions, not a claim of universal parity across Linux-only transports, every external registry, or later upstream commits. Future bug fixes and feature ports continue through scoped issues; upstream source releases are not bulk-merged.
