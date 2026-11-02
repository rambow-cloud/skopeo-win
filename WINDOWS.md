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

Local builds identify themselves as `0.0.0-dev`. To reproduce a tagged release version, use `-Version 1.0.0` from that tag's source. CI allocates its version automatically before building.

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
.\dist\skopeo-windows-amd64.exe copy docker://docker.io/library/alpine:latest oci:alpine:latest
```

For private registries, use `skopeo login registry.example.com` or pass `--creds USER:PASSWORD` to an individual command. TLS certificate, authentication file, and policy file options work like their upstream Skopeo equivalents.

## Windows feature scope

Common transports that do not depend on Linux container storage drivers are supported, including remote registries, `dir:`, `oci:`, `oci-archive:`, and `docker-archive:`. The upstream experimental hidden `proxy` command explicitly does not support Windows. Workflows that require Linux namespaces, mounts, or Linux graph drivers are outside the scope of this executable.

## Independent maintenance

Bug fixes, security updates, and selected feature ports are implemented through scoped issues and reviewed PRs. Do not bulk-merge upstream Skopeo source releases or copy their version constants. Dependency upgrades remain supported and must pass Windows compatibility and vulnerability checks. Preserve applicable upstream copyright and license notices when porting a feature.

## Automatic releases

Successful `main` push builds and manual `main` runs publish independent releases starting at `v1.0.0`. Each new release increments the patch number. The version is allocated before compilation and embedded in the executables, so `--version` matches the release tag. PRs and local builds use development versions.

Publication first creates a draft, verifies all four uploaded asset digests, and then makes it public. A release-job retry resumes missing draft uploads or verifies an already published release without modifying it. Published files and tags are never overwritten or moved. Historical `v1.23.0` is retained but excluded from the independent counter. See [release-policy.md](docs/release-policy.md) for details.

## Vulnerability checks

The release build uses the Go version declared in `go.mod`. CI runs command unit tests and installs `govulncheck` v1.9.0 to scan production Windows source and both binaries before uploading release assets. Source analysis checks reachable vulnerabilities; binary scans also reject findings in packages present in the executable. Module-only advisories for packages absent from the build are reported separately and do not fail the gate. A separate release job receives write permission only after the build and scan job succeeds.

To repeat these checks locally after building both architectures:

```powershell
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
.\hack\check-vulnerabilities.ps1
```

The scanner needs network access to the Go vulnerability database. Reports are written under `.cache/vulnerability-scan` and retained as CI artifacts. A clean result covers known Go database findings for the scanned configuration and database snapshot, not all possible vulnerabilities.
