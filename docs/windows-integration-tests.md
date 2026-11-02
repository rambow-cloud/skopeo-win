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

## Coverage

| Scenario | Assertion |
| --- | --- |
| Executable identity | The child process reports the exact CI-allocated version. |
| Registry round trip | Push both Windows architectures, inspect the index and arm64 config, list tags, pull both instances, and compare layer bytes and instance manifests. A missing image must return a failure. |
| Platform subsets | Default selection keeps a sparse index; stripping pushes a one-platform index to the registry; preserve-digests rejects an incompatible change. |
| Signature controls | Directory copies preserve actual signature packets by default, selectively remove list signatures, remove all signatures, and reject signed-list changes or conflicting flags. These packets are opaque under the test's explicit insecure policy; this scenario does not establish cryptographic trust. |
| YAML sync | Parse a maintained-YAML configuration, sync both architectures between registry repositories, and inspect the resulting index. |
| Foreign-layer copy/sync | Defaults retain external references with zero downloads. Opt-in copy and sync store exact layer bytes, verified through the registry blob endpoint even when the external server fails. Retained external URLs can still make subsequent CLI pulls depend on that server; those failures and failed opt-in downloads must return a nonzero exit. |

The original command tests remain useful for isolated option/error handling and pure-Go OpenPGP trust verification. The original upstream `integration/` directory is a separate, primarily Linux-oriented suite and is not claimed to run by this Windows harness. Local registry tests do not certify every hosted registry, authentication service, TLS deployment, container runtime, or network condition. ARM64 is cross-compiled and PE-checked; it is not runtime-tested here.

## CI and release gate

The Windows workflow runs this suite against the same executable it will upload, after command/release tests and before security scans and publication. Test reports are uploaded even on failure. A failing integration step prevents the build job from succeeding, so the dependent release job cannot publish. See the release's linked workflow run for the exact result rather than inferring test coverage from build success alone.
