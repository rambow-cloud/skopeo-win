## Features

- **Help on empty invocation:** running the CLI without a command displays root help on stdout and exits with status 0, matching `--help`. Valid global flags without a command also display help; unknown commands, invalid flags, conflicting global options and missing required subcommand arguments still fail. Executable integration tests cover literal zero-argument invocation and confirm that help creates no default policy.

- **Windows first-use policy:** a clean Windows profile automatically receives an embedded, default-reject `policy.json` permitting Docker Official Images, Microsoft Container Registry, Kubernetes, Prometheus on Quay and Flux on GHCR. If saving is unavailable, the identical policy runs in memory. Existing policies and explicit selections remain authoritative; invalid configured policies are not bypassed. Namespace acceptance does not verify publisher signatures, and TLS checks stay enabled.
- **Linux image defaults:** image selection now defaults to Linux and the native machine architecture, including ARM64 detection under Windows x64 emulation. Explicit OS/architecture flags and all-platform copies remain supported. Examples include exporting a Docker-loadable tar archive without Docker.

- **Store acceptance fix:** Store packages now declare a visible default application entry instead of the headless entry rejected in 1.0.6.0. The Start-menu entry opens a native usage window that waits for Enter; command aliases still invoke the CLI directly. Package verification rejects headless manifests and tests launcher input/exit behavior. `runFullTrust` remains subject to Microsoft's separate approval.

- **Microsoft Store packaging:** stable main builds produce a separate x64/ARM64 MSIX bundle with `skopeo` and `skopeo-win` command aliases and the registered RambowCloud identity. Actions can also package an existing immutable release without rebuilding or replacing its executables. Builds retain submission artifacts; a separately configured Store workflow can submit the update. Store signing and publication remain subject to Microsoft certification.

- **Selective manifest-list signatures:** `copy` and `sync` support `--remove-list-signatures`, removing list signatures while preserving per-platform signatures. It cannot be combined with `--remove-signatures` and is intended for manifest-list copies.
- **Platform subset copies:** `copy --multi-arch=windows/amd64 --strip-removed-platforms --remove-list-signatures` creates a manifest list containing only the chosen platform. The default still creates a sparse list. Signed lists and `--preserve-digests` continue to prevent prohibited manifest changes.
- **Foreign-layer downloads:** `copy` and `sync` expose opt-in `--download-foreign-layers` for nondistributable layers, including Windows base-image layers. External references remain the default where the destination supports them. Layer licenses still govern redistribution. This capability was selectively ported from upstream development main after v1.24.1.

Downloading a foreign layer does not guarantee a self-contained offline mirror: the current image library may retain external URLs in the manifest, and subsequent CLI pulls prefer those URLs even if the registry stores the layer. An unavailable external server can therefore still cause pulls to fail.

## Fixes and maintenance

- **Maintenance release gating:** automation and documentation changes run build, integration and vulnerability checks without allocating a stable version, publishing a release or submitting a Store update. Production input changes are compared with the latest published source so unreleased code cannot be hidden by a later CI fix. Explicit manual releases remain available.

- **Independent repository automation:** removed inherited upstream Cirrus-monitor and issue-lock workflow callers whose old Podman references fail GitHub's workflow validation. Windows build/release, Store submission and the standalone stale-issue workflow are retained. Historical failed runs remain available as evidence.

- **Store API diagnostics:** Store requests explicitly negotiate JSON, including empty draft-creation and commit requests. Failed requests retain the operation, HTTP status and structured API error fields after redacting credentials, bearer tokens and URLs; arbitrary response bodies remain hidden. Offline transaction checks cover these headers, diagnostic redaction and draft-creation rejection before upload.

- **Third-party Store notices:** both MSIX architectures now include notices inventoried from their actual CLI/launcher dependencies and matching Go source tree, with versions, source attribution and file hashes. Package checks reject missing or changed notices and incorrect inventories. Historical binary/source mismatches stop packaging instead of attaching unrelated license files. Portable release assets remain unchanged.

- **Reliable sync planning:** invalid YAML selections and registry tag-discovery failures now produce a failing exit code instead of a misleading zero-image success. All Docker and YAML tag-discovery paths honor `--command-timeout`.
- **Windows directory mirror round trips:** tagged and digest-selected registry images use reversible Windows-safe directory names. Encoded tag case stays distinct on case-insensitive filesystems; existing ordinary directory layouts remain readable. Scoped restores should use the exported source-registry directory as their source root to preserve repository namespaces.
- **Read-only dry runs:** sync plans no longer create destination directory trees or digest files. Existing destination directories are still rejected.
- **Store delivery gates:** both input CLI architectures are scanned again before Store packaging, including historical-release repackaging. CI verifies that either CLI scanner failure stops packaging. A separate main-only Actions workflow can submit validated packages through the Microsoft Store API once Entra credentials are configured; it refuses existing pending submissions and preserves listing metadata.

- **Key-output safety:** key generation now creates output files exclusively at the write boundary and refuses links or files introduced after its initial validation. Existing unrelated files remain unchanged. Use a trusted output directory; exploitation of the former race required local directory/link permissions and the invoker's target-file rights.
- **Publication state checks:** the publisher binds a draft to its numeric release ID and rechecks its ownership, tag and asset digests after upload. If another actor has already published the same verified release, retries do not edit it again.
- **Security test redesign:** replaced shallow key-output checks and positional release mocks with real-filesystem concurrency tests, a strict release-service model, version-allocation fuzzing, and executable trust-policy/integrity regressions. Existing upstream command tests and pure-Go OpenPGP checks are retained.

- **Windows directory sync:** fixed destination image names for `sync --src=dir --dest=docker`. Native filesystem paths are now converted to registry-relative names without embedding a Windows drive or backslashes. Absolute/relative inputs, trailing separators, and scoped nested directories have executable regression coverage.

- Sync uses maintained `go.yaml.in/yaml/v3` v3.0.5, and common is v0.69.2; newer image/storage security dependencies are retained.
- Invalid command arguments display actionable help; public-key write errors identify the correct file. Help includes duration units and OS/architecture override examples.
- Automated releases now include this reviewed feature and verification summary alongside their source and CI links. Independent patch versions continue to increase without replacing published binaries or moving tags.

## Verification

- New executable integration scenarios exercise clean-profile archive exports, default and overridden platform selection, all-platform OCI copies, namespace boundaries, signature/TLS rejection, existing and explicit policy precedence, embedded fallback, read-only commands, concurrent bootstrap and Windows directory-junction containment. Tests use isolated local registries rather than live public services; ARM64 PE verification and cross-builds do not imply ARM64 runtime testing.

- Release publication is gated on command/release tests, English-only documentation, source and both binary vulnerability scans, and executable integration tests on Windows amd64.
- Integration tests start the built executable as separate processes against a local OCI Registry implementation and local layer server. They cover registry push/pull, inspect/list-tags, platform selection, signatures, digest conflicts, sync, and foreign-layer defaults/opt-in downloads.
- Every selected feature port now has a named executable scenario: signed-list and instance-signature behavior in both copy and sync; single-image and incompatible-flag rejection; preserved sparse-index digests; authenticated YAML aliases/regex/TLS precedence and invalid inputs; changed argument diagnostics, duration enforcement, help examples, key generation/overwrite protection, and deterministic public-key write errors.
- CI uploads parseable JSON test events separately from Go toolchain diagnostics, including first-time dependency-download messages. Executable stdout/stderr and exit results remain in the JSON events.
- These local integration tests do not certify hosted registries, Windows container execution, the entire upstream Linux integration suite, or ARM64 runtime behavior. Vulnerability scan reports remain available in the linked CI run, including any module-only advisory that is not applicable to imported or compiled packages.
