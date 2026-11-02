## Features

- **Store acceptance fix:** Store packages now declare a visible default application entry instead of the headless entry rejected in 1.0.6.0. The Start-menu entry opens a native usage window that waits for Enter; command aliases still invoke the CLI directly. Package verification rejects headless manifests and tests launcher input/exit behavior. `runFullTrust` remains subject to Microsoft's separate approval.

- **Microsoft Store packaging:** stable main builds produce a separate x64/ARM64 MSIX bundle with `skopeo` and `skopeo-win` command aliases and the registered RambowCloud identity. Actions can also package an existing immutable release without rebuilding or replacing its executables. Store signing and publication require Microsoft certification; the workflow uploads submission artifacts, not a live Store listing.

- **Selective manifest-list signatures:** `copy` and `sync` support `--remove-list-signatures`, removing list signatures while preserving per-platform signatures. It cannot be combined with `--remove-signatures` and is intended for manifest-list copies.
- **Platform subset copies:** `copy --multi-arch=windows/amd64 --strip-removed-platforms --remove-list-signatures` creates a manifest list containing only the chosen platform. The default still creates a sparse list. Signed lists and `--preserve-digests` continue to prevent prohibited manifest changes.
- **Foreign-layer downloads:** `copy` and `sync` expose opt-in `--download-foreign-layers` for nondistributable layers, including Windows base-image layers. External references remain the default where the destination supports them. Layer licenses still govern redistribution. This capability was selectively ported from upstream development main after v1.24.1.

Downloading a foreign layer does not guarantee a self-contained offline mirror: the current image library may retain external URLs in the manifest, and subsequent CLI pulls prefer those URLs even if the registry stores the layer. An unavailable external server can therefore still cause pulls to fail.

## Fixes and maintenance

- **Key-output safety:** key generation now creates output files exclusively at the write boundary and refuses links or files introduced after its initial validation. Existing unrelated files remain unchanged. Use a trusted output directory; exploitation of the former race required local directory/link permissions and the invoker's target-file rights.
- **Publication state checks:** the publisher binds a draft to its numeric release ID and rechecks its ownership, tag and asset digests after upload. If another actor has already published the same verified release, retries do not edit it again.
- **Security test redesign:** replaced shallow key-output checks and positional release mocks with real-filesystem concurrency tests, a strict release-service model, version-allocation fuzzing, and executable trust-policy/integrity regressions. Existing upstream command tests and pure-Go OpenPGP checks are retained.

- **Windows directory sync:** fixed destination image names for `sync --src=dir --dest=docker`. Native filesystem paths are now converted to registry-relative names without embedding a Windows drive or backslashes. Absolute/relative inputs, trailing separators, and scoped nested directories have executable regression coverage.

- Sync uses maintained `go.yaml.in/yaml/v3` v3.0.5, and common is v0.69.2; newer image/storage security dependencies are retained.
- Invalid command arguments display actionable help; public-key write errors identify the correct file. Help includes duration units and OS/architecture override examples.
- Automated releases now include this reviewed feature and verification summary alongside their source and CI links. Independent patch versions continue to increase without replacing published binaries or moving tags.

## Verification

- Release publication is gated on command/release tests, English-only documentation, source and both binary vulnerability scans, and executable integration tests on Windows amd64.
- Integration tests start the built executable as separate processes against a local OCI Registry implementation and local layer server. They cover registry push/pull, inspect/list-tags, platform selection, signatures, digest conflicts, sync, and foreign-layer defaults/opt-in downloads.
- Every selected feature port now has a named executable scenario: signed-list and instance-signature behavior in both copy and sync; single-image and incompatible-flag rejection; preserved sparse-index digests; authenticated YAML aliases/regex/TLS precedence and invalid inputs; changed argument diagnostics, duration enforcement, help examples, key generation/overwrite protection, and deterministic public-key write errors.
- CI uploads parseable JSON test events separately from Go toolchain diagnostics, including first-time dependency-download messages. Executable stdout/stderr and exit results remain in the JSON events.
- These local integration tests do not certify hosted registries, Windows container execution, the entire upstream Linux integration suite, or ARM64 runtime behavior. Vulnerability scan reports remain available in the linked CI run, including any module-only advisory that is not applicable to imported or compiled packages.
