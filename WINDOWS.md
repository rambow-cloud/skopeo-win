# Windows guide

Use PowerShell 5.1 or 7. Examples use the Store alias `skopeo-win`; for a portable
download, substitute `.\skopeo-windows-amd64.exe` or the ARM64 filename.
See [installation](install.md) and the [quick start](README.md#quick-usage).

## Images and paths

Images default to **Linux** and the native machine architecture, including ARM64
hosts running an emulated x64 binary. Override either when needed:

```powershell
skopeo-win --override-os windows inspect docker://mcr.microsoft.com/windows/nanoserver:ltsc2022
skopeo-win --override-arch arm64 inspect docker://docker.io/library/nginx:latest
```

Quote file transports, especially paths containing spaces. Use an absolute
Windows path for named OCI layouts; `$PWD` supplies the current directory:

```powershell
skopeo-win copy docker://docker.io/library/nginx:latest 'docker-archive:.\nginx.tar:nginx:latest'
skopeo-win copy docker://docker.io/library/nginx:latest "dir:$PWD\nginx"
skopeo-win copy docker://docker.io/library/nginx:latest "oci:$PWD\nginx-oci:latest"
```

`docker-archive:` creates a tar file; `dir:` stores separate manifests/layers;
`oci:` creates an OCI layout. `copy --all` copies all image platforms. No Docker
daemon is needed; Docker can optionally import the tar with `docker load -i .\nginx.tar`.

## Login and sync

```powershell
skopeo-win login registry.example.com
skopeo-win sync --dry-run --src docker --dest dir docker.io/library/nginx:latest .\mirror
skopeo-win sync --src docker --dest dir docker.io/library/nginx:latest .\mirror
skopeo-win logout registry.example.com
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

The default accepts unsigned images only from `docker.io/library`,
`mcr.microsoft.com`, `registry.k8s.io`, `quay.io/prometheus` and `ghcr.io/fluxcd`.
This is namespace-based acceptance, not publisher-signature verification;
HTTPS verification remains enabled.

Private registries, other namespaces and local image sources need a policy
that trusts the selected source. Select it with `--policy .\policy.json` before
the command, or set `$env:CONTAINERS_POLICY_JSON`. For example, a policy for one
private registry can contain:

```json
{
  "default": [{"type": "reject"}],
  "transports": {
    "docker": {"registry.example.com": [{"type": "insecureAcceptAnything"}]}
  }
}
```

For local sources, configure the relevant transport and absolute Windows path.
See the [policy format](https://github.com/containers/image/blob/main/docs/containers-policy.json.5.md)
for signed-image policies. `--insecure-policy` explicitly accepts all sources.

## PowerShell completion

```powershell
skopeo-win completion
```

Run once, then restart PowerShell. Setup creates or updates the current user's
current console-host profile, preserving its content and encoding. Changed
nonempty profiles receive `.skopeo-win-*.bak` backups. Ordinary commands never
configure profiles; signed, linked or ambiguous profiles are rejected.

For a custom host such as VS Code, use `skopeo-win completion --profile "$PROFILE"`.
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
