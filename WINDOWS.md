# Windows guide

Use PowerShell 5.1 or 7. Examples use `skopeo`. For a portable download, rename
the executable to `skopeo.exe` and use `.\skopeo.exe` until its folder is on `PATH`.
See [installation](install.md) and the [quick start](README.md#quick-usage).

## Images and paths

Images default to **Linux** and the native machine architecture, including ARM64
hosts running an emulated x64 binary. Override either when needed:

```powershell
skopeo --override-os windows inspect docker://mcr.microsoft.com/windows/nanoserver:ltsc2022
skopeo --override-arch arm64 inspect docker://docker.io/library/nginx:latest
```

Quote file transports, especially paths containing spaces. Use an absolute
Windows path for named OCI layouts; `$PWD` supplies the current directory:

```powershell
skopeo copy docker://docker.io/library/nginx:latest 'docker-archive:.\nginx.tar:nginx:latest'
skopeo copy docker://docker.io/library/nginx:latest "docker-archive:$PWD\nginx.tar:nginx:latest"
skopeo copy docker://docker.io/library/nginx:latest "dir:$PWD\nginx"
skopeo copy docker://docker.io/library/nginx:latest "oci:$PWD\nginx-oci:latest"
```

Docker archives accept absolute drive-letter or relative paths, including spaces;
tag/index selectors follow the archive path. Both slash styles work on Windows.
`docker-archive:` creates a tar file; `dir:` stores separate manifests/layers;
`oci:` creates an OCI layout. `copy --all` copies all image platforms. No Docker
daemon is needed; Docker can optionally import the tar with `docker load -i .\nginx.tar`.

## Login and sync

```powershell
skopeo login registry.example.com
skopeo sync --dry-run --src docker --dest dir docker.io/library/nginx:latest .\mirror
skopeo sync --src docker --dest dir docker.io/library/nginx:latest .\mirror
skopeo logout registry.example.com
```

Login prompts for credentials. The default auth file is
`$env:USERPROFILE\.config\containers\auth.json`; select another with `--authfile`
or `$env:REGISTRY_AUTH_FILE`. For private CAs, pass `--cert-dir .\certs` to the
relevant command, or `--src-cert-dir` / `--dest-cert-dir` for copy/sync.

Windows sync encodes reserved filename characters using `~skopeo~` and percent
escapes. Keep exported names intact for restoration; see [sync](docs/skopeo-sync.1.md).

## Image trust policy

When no policy exists, the first copy or non-dry-run sync creates
`$env:APPDATA\containers\policy.json`. If it cannot be safely written, the same
embedded policy is used in memory. Existing or explicitly selected policies take
precedence; invalid existing policies are reported rather than replaced.

The default matches [upstream Skopeo](https://github.com/podman-container-tools/skopeo/blob/main/default-policy.json):
accept unsigned images from every registry and local source, including ECR Public.
TLS verification and registry authentication remain enabled. The generated policy is:

```json
{
  "default": [{"type": "insecureAcceptAnything"}],
  "transports": {
    "docker-daemon": {"": [{"type": "insecureAcceptAnything"}]}
  }
}
```

To adopt this default instead of an older generated user policy, back up that
file, then run copy/sync again:

```powershell
Rename-Item "$env:APPDATA\containers\policy.json" policy.json.bak
```

Existing policies are never automatically replaced. To restrict sources or
require signatures, select your own policy with `--policy .\policy.json` or
`$env:CONTAINERS_POLICY_JSON`; see the [policy format](https://github.com/containers/image/blob/main/docs/containers-policy.json.5.md).
`--require-signed` still rejects unsigned images.

## PowerShell completion

```powershell
skopeo completion
```

Run once, then restart PowerShell. Setup creates or updates the current user's
current console-host profile, preserving its content and encoding. Changed
nonempty profiles receive `.skopeo-win-*.bak` backups. Ordinary commands never
configure profiles; signed, linked or ambiguous profiles are rejected.

For a custom host such as VS Code, use `skopeo completion --profile "$PROFILE"`.
Use the same profile selection with `--status` or `--uninstall`. `--path` prints
the installed script path; `--no-descriptions` omits completion descriptions.
The script is stored under Documents\PowerShell\SkopeoWin. Setup does not change
execution policy; the host must permit profile scripts. There are no shell subcommands.

## Build

Use PowerShell 7 and the Go version declared in `go.mod`, from the repository root:

```powershell
.\build-windows.ps1 -Architecture amd64,arm64
.\hack\test-windows-integration.ps1 -ExpectedVersion 0.0.0-dev
```

Outputs are under `dist\` and default to `0.0.0-dev`. The build enforces
`CGO_ENABLED=0`, pure-Go OpenPGP and PE dependency checks. Debug symbols remain
enabled for vulnerability scanning; release builds must retain them.

See [integration coverage](docs/windows-integration-tests.md), the
[security scan](docs/security-scan-2026-11-02.md) and [release policy](docs/release-policy.md).
Linux storage drivers, mounts/namespaces and the experimental `proxy` command
are outside this executable's scope.
