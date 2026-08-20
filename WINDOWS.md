# Skopeo for Windows (Pure Go Build)

This repository is based on upstream Skopeo v1.23.0 and produces native Windows executables without GCC, MSYS2, GPGME, or other third-party C libraries. The build enforces `CGO_ENABLED=0` and selects the pure-Go OpenPGP implementation with the `containers_image_openpgp` build tag.

## Build

PowerShell 7 (or Windows PowerShell 5.1) and Go 1.25.6 or newer are required. Run this command from the repository root:

```powershell
.\build-windows.ps1
```

By default, the script creates `dist\skopeo-windows-amd64.exe` and its SHA-256 checksum file. Build both amd64 and arm64 binaries with:

```powershell
.\build-windows.ps1 -Architecture amd64,arm64
```

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

## Updating from upstream

The core command and image-processing code remains unchanged from upstream. To update this fork, merge a newer stable Skopeo release and then run `build-windows.ps1 -Architecture amd64,arm64`. The build metadata and PE checks in the script act as upgrade gates, and CI runs the same process before uploading artifacts.

## Automatic releases

Every push to `main` builds and verifies both Windows architectures. The workflow reads the version from `version/version.go`, creates a matching `vX.Y.Z` GitHub Release, and uploads both executables and SHA-256 files. If a release for that upstream version already exists, its binary assets are replaced with the newly verified build instead of creating a second version.
