# Windows executable integration tests

The `tests/windows` module starts the built Windows amd64 executable as child processes. It connects those processes to a loopback OCI Registry from [`go-containerregistry/pkg/registry` v0.22.1](https://github.com/google/go-containerregistry/tree/v0.22.1/pkg/registry), an implementation of the registry protocol, and to a local foreign-layer HTTP server. It does not replace registry operations with canned responses. Synthetic image fixtures contain real gzip-compressed tar layers; no Docker daemon, public registry account, Alpine image, or Windows container execution is needed.

## Run locally

From the repository root in PowerShell:

```powershell
./build-windows.ps1 -Architecture amd64,arm64 -Version 0.0.0-dev
./hack/test-windows-integration.ps1 -ExpectedVersion 0.0.0-dev
```

The script requires the built executable and fails if it is absent. It sets an absolute executable path, isolates credential/configuration directories, disables CGO for the test harness, and runs with the committed integration module checksums in readonly mode. Child commands have a 30-second timeout; the suite has a five-minute timeout. Every invocation, exit result, stdout, stderr, and Go test event is retained in `.cache/integration-tests/windows-amd64.jsonl`. No test is skipped because the binary is missing.

Go toolchain diagnostics, including dependency-download progress, are stored separately in `.cache/integration-tests/go-stderr.log` so the JSONL event stream remains parseable. Both files are uploaded by CI. Executable stdout/stderr remain part of their corresponding JSON test events.

The integration module is separate from the production module. Its registry implementation is test infrastructure and is not linked into the released executable or added to the production vendor tree. Keep its Go version aligned with the production module and update its dependencies/checksums through reviewed changes.

The Windows key-diagnostic tests additionally use go-winio to coordinate a named-pipe passphrase read. This creates a deterministic public-output write failure after the executable's initial path checks, without sleeps, timing races, privileged symlinks, or production test hooks. go-winio remains a test dependency of this module.

## Coverage

| Scenario | Assertion |
| --- | --- |
| Executable identity | The child process reports the exact CI-allocated version. |
| Default root help | `TestDefaultRootHelp` invokes the executable with literally zero arguments and compares stdout with explicit `--help`, requiring exit 0 and empty stderr. Valid global-only invocations also show help; invalid commands/flags and missing subcommand arguments still fail without creating a default policy. |
| Registry round trip | Push both Windows architectures, inspect the index and arm64 config, list tags, pull both instances, and compare layer bytes and instance manifests. A missing image must return a failure. |
| Platform subsets | Default selection keeps a sparse index; stripping pushes a one-platform index to the registry; preserve-digests rejects an incompatible change. |
| Signature controls | Directory copies preserve actual signature packets by default, selectively remove list signatures, remove all signatures, and reject signed-list changes or conflicting flags. These packets are opaque under the test's explicit insecure policy; this scenario does not establish cryptographic trust. |
| YAML sync | Parse a maintained-YAML configuration, sync both architectures between registry repositories, and inspect the resulting index. |
| Foreign-layer copy/sync | Defaults retain external references with zero downloads. Opt-in copy and sync store exact layer bytes, verified through the registry blob endpoint even when the external server fails. Retained external URLs can still make subsequent CLI pulls depend on that server; those failures and failed opt-in downloads must return a nonzero exit. |

## Feature-to-test matrix

Each row identifies user-visible behavior from the selected feature ports, rather than inferring coverage from a parameter appearing in help. All scenarios below execute the built program as a child process.

| Feature / supported command | Executable integration scenario | Observable acceptance criteria |
| --- | --- | --- |
| copy platform stripping and defaults | `TestPlatformSubsetThroughRegistry`, `TestCopyPlatformAndSignatureBoundaries` | Registry subset has one selected instance; default remains sparse without copying the omitted instance; both requested platforms are retained; unsigned lists can be stripped; preserve-digests retains the original sparse index or rejects incompatible stripping; `--all` conflicts with explicit selection. |
| copy signature controls | `TestExecutableSignatureControls`, `TestCopyPlatformAndSignatureBoundaries` | Preserve packets by default, selectively remove the list signature, remove all signatures, reject modifications of signed lists, reject conflicting flags, and reject list-only removal on a single image. |
| sync signature controls | `TestSyncSignatureControls` | Sync actual signed directory fixtures to a registry with an isolated signature lookaside store, then pull them back to verify list and both instance packets. Default preserves all; list removal retains both instances; full removal removes all; conflicting flags and single-image list removal fail. |
| copy/sync foreign-layer download | `TestForeignLayerCopyAndSync` | Defaults perform zero layer downloads; each opt-in stores exact bytes; missing external service produces nonzero exits in both copy and sync; retained URL behavior remains visible. |
| maintained YAML compatibility | `TestYAMLSyncThroughRegistry`, `TestYAMLCompatibilityAndAuthentication` | Real transfers parse aliases and regex filters, authenticate with fixture credentials, apply YAML TLS settings over a conflicting CLI setting, and copy only selected tags. Bad credentials, malformed YAML, and invalid TLS booleans fail. |
| changed argument/required-option diagnostics | `TestNewCLIArgumentDiagnostics` | Each changed command exits with code 1, emits help plus its actionable error on stderr, and leaves stdout empty. Signing output and key-prefix requirements are included. |
| duration syntax and inspect override examples | `TestDurationAndCrossPlatformHelp`, `TestCommandTimeoutIsEnforced`, `TestRegistryRoundTrip` | Root help shows duration examples, inspect help shows OS/architecture override examples, valid units parse, invalid units fail, a blocked local request obeys the timeout, and an actual arm64 image configuration is selected. |
| public-key write error and key output | `TestSigstoreKeyGenerationAndOverwriteProtection`, `TestPublicKeyWriteFailureIdentifiesPublicKey` | Generate a key pair through a path with spaces, refuse overwrite without changing bytes, and force a real public-output write failure that identifies the public key after the private key was written. |
| Windows directory sync regression found by these tests | `TestDirectorySyncWindowsPaths` | Absolute, relative, trailing-separator, slash-normalized, scoped nested and unscoped nested sources produce valid registry names and preserve image-instance digests. |
| Key-output race boundary | `TestKeyOutputRaceDoesNotClobberLinkedFiles` | A deterministic named-pipe barrier introduces a hard link after initial validation for each output. Both commands must fail and preserve an unrelated sentinel file. This is a local-filesystem test, not a remote exploit claim. |
| Actual image-policy enforcement | `TestSignatureRemovalDoesNotBypassImageTrustPolicy`, `TestRejectedImageCannotTriggerForeignLayerDownload` | Positive fixture controls succeed, then explicit reject policies deny copy/sync operations using removal/download flags. Stripping still obeys policy; a rejected image must trigger zero external layer requests. |
| Cryptographic verification through the executable | `TestStandaloneVerificationEnforcesTrustAndIntegrity` | A valid trusted signature verifies. Changed manifest bytes, changed signature packets, wrong identity, wrong fingerprint and unsigned payloads fail without success output. The fixture unwraps compression so mutation changes authenticated signature bytes rather than only compression framing. |

Release-number allocation, reviewed-note rendering, checksum/tag collision protection, and interrupted-publication retries are covered by the separate `internal/release` suite and actual main publication job. They are release infrastructure, not commands inside the Windows image executable. Existing pure-Go OpenPGP tests verify trust and tampering; the packet-copy scenarios above deliberately test preservation under an insecure policy and do not establish signature trust.

The original command tests remain useful for isolated option/error handling and pure-Go OpenPGP trust verification. The original upstream `integration/` directory is a separate, primarily Linux-oriented suite and is not claimed to run by this Windows harness. Local registry tests do not certify every hosted registry, authentication service, TLS deployment, container runtime, or network condition. ARM64 is cross-compiled and PE-checked; it is not runtime-tested here.

## CI and release gate

The Windows workflow runs this suite against the same executable it will upload, after command/release tests and before security scans and publication. Test reports are uploaded even on failure. A failing integration step prevents the build job from succeeding, so the dependent release job cannot publish. See the release's linked workflow run for the exact result rather than inferring test coverage from build success alone.
