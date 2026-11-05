# Skopeo for Windows (Pure Go Build)

This independently maintained project originated from Skopeo v1.23.0 and produces native Windows executables without GCC, MSYS2, GPGME, or other third-party C libraries. Its release versions are independent of Skopeo. The build enforces `CGO_ENABLED=0` and selects the pure-Go OpenPGP implementation with the `containers_image_openpgp` build tag.

## Build

PowerShell 7 (or Windows PowerShell 5.1) and Go 1.27.3 or newer are required. Run this command from the repository root:

```powershell
.\build-windows.ps1
```

By default, the script creates `dist\skopeo-windows-amd64.exe` and its SHA-256 checksum file. Build both amd64 and arm64 binaries with:

```powershell
.\build-windows.ps1 -Architecture amd64,arm64
```

Local builds identify themselves as `0.0.0-dev`. To reproduce a tagged release version, use `-Version 1.0.0` from that tag's source. Release CI uses the exact maintainer-created tag version; other CI builds use development versions.

Debug symbols are retained by default so binary vulnerability scans can identify packages accurately. Stripped PE executables make govulncheck fall back to conservative module-level matching. Release builds must retain symbols; `-KeepDebugSymbols:$false` is available only for custom size-optimized builds that will not pass the same scan guarantees.

Before delivering each executable, the script verifies that:

- the Go build metadata contains `CGO_ENABLED=0`, `GOOS=windows`, and the requested architecture;
- the `containers_image_openpgp` build tag is enabled;
- the PE import table does not contain GPGME, OpenSSL, MSVC/UCRT, or MinGW C/C++ runtime libraries.

The executable can still use standard Windows system APIs. In this repository, "no C library dependency" means that users do not need to install or distribute a third-party C library or C runtime with Skopeo.

## Usage

Rename the executable to `skopeo.exe` and add it to `PATH`, or run it directly:

```powershell
.\dist\skopeo-windows-amd64.exe --version
.\dist\skopeo-windows-amd64.exe inspect docker://docker.io/library/alpine:latest
.\dist\skopeo-windows-amd64.exe copy docker://docker.io/library/nginx:latest docker-archive:./nginx.tar:nginx:latest
```

The copy example creates `nginx.tar` in the current directory. Docker can import
it with `docker load -i .\nginx.tar`; exporting the archive does not require Docker.
Images default to `linux` and the native Windows machine architecture (`amd64`
or `arm64`), including detection of ARM64 hosts running an emulated x64 binary.
Explicit flags still take precedence:

```powershell
skopeo-win --override-os windows inspect docker://mcr.microsoft.com/windows/nanoserver:ltsc2022
skopeo-win --override-arch arm64 inspect docker://docker.io/library/nginx:latest
```

`copy --all` and `sync --all` continue to copy every platform in a manifest list.
For a named OCI layout on Windows, use an absolute path, for example
`oci:C:\images\nginx:latest`.

For private registries, use `skopeo login registry.example.com` or pass `--creds USER:PASSWORD` to an individual command. TLS certificate, authentication file, and policy file options work like their upstream Skopeo equivalents.

Running `skopeo-win` without a command displays the same help as `skopeo-win --help`
and exits successfully with status 0. Valid global flags without a command also
display help. Unknown commands, invalid flags and missing required subcommand
arguments remain errors.

## Image trust policy

On a clean Windows profile, the first `copy` or non-dry-run `sync` that needs a
policy creates `%APPDATA%\containers\policy.json` from an embedded template.
If the template cannot be safely saved, the same policy is used in memory and
the operation continues. `--version`, help, `inspect`, and `sync --dry-run` do not
create this file. Concurrent first runs publish complete bytes without replacing
an existing file; creation cannot follow a junction outside the configured root.

The template rejects sources by default and permits only these public namespaces:

- `docker.io/library` (Docker Official Images)
- `mcr.microsoft.com`
- `registry.k8s.io`
- `quay.io/prometheus`
- `ghcr.io/fluxcd`

This is namespace-based acceptance of unsigned images, not cryptographic
publisher-signature verification. HTTPS certificate verification remains enabled.
Copying from other Docker Hub/GHCR namespaces, private registries or local image
sources needs an explicit trust policy. Edit the generated file or select a policy using
`--policy PATH` or `CONTAINERS_POLICY_JSON`. For signature enforcement, configure
trusted signing identities in that policy and use `--require-signed` as needed.

Existing user/system policies and explicit selections take precedence. Invalid
existing policies and missing explicit policy paths remain errors; they are never
silently replaced with the embedded template. The explicit `--insecure-policy`
option remains available for operations where you intentionally accept every
source. See the [policy format](https://github.com/containers/image/blob/main/docs/containers-policy.json.5.md).

## Windows feature scope

Common transports that do not depend on Linux container storage drivers are supported, including remote registries, `dir:`, `oci:`, `oci-archive:`, and `docker-archive:`. The upstream experimental hidden `proxy` command explicitly does not support Windows. Workflows that require Linux namespaces, mounts, or Linux graph drivers are outside the scope of this executable.

## Independent maintenance

Bug fixes, security updates, and selected feature ports are implemented through scoped issues and reviewed PRs. Do not bulk-merge upstream Skopeo source releases or copy their version constants. Dependency upgrades remain supported and must pass Windows compatibility and vulnerability checks. Preserve applicable upstream copyright and license notices when porting a feature.

## Tagged releases

Merges and pushes to `main`, PRs and normal manual builds run checks without publishing or updating the Store. Accumulate reviewed features on `main`, then create and push an unused stable tag such as `v1.0.15` to release. The exact tag version is embedded in both executables, so `--version` matches the release tag. The tagged source must belong to main history. A successful tagged release triggers the existing Store automation. Other builds use development versions.

Publication first creates a draft, verifies all four uploaded asset digests, and then makes it public. A release-job retry resumes missing draft uploads or verifies an already published release without modifying it. Published files and tags are never overwritten or moved. Historical `v1.23.0` is retained but excluded from the independent counter. See [release-policy.md](docs/release-policy.md) for details.

## Vulnerability checks

The release build uses the Go version declared in `go.mod`. CI runs command unit tests and installs `govulncheck` v1.9.0 to scan production Windows source and both binaries before uploading release assets. Source analysis checks reachable vulnerabilities; binary scans also reject findings in packages present in the executable. Module-only advisories for packages absent from the build are reported separately and do not fail the gate. A separate release job receives write permission only after the build and scan job succeeds.

To repeat these checks locally after building both architectures:

```powershell
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
.\hack\check-vulnerabilities.ps1
```

The scanner needs network access to the Go vulnerability database. Reports are written under `.cache/vulnerability-scan` and retained as CI artifacts. A clean result covers known Go database findings for the scanned configuration and database snapshot, not all possible vulnerabilities.
