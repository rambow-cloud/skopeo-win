<p align="center">
   <img src="docs/skopeo.svg" width="150" alt="Skopeo">
</p>

<h1 align="center">skopeo-win</h1>

<p align="center">
   Inspect, copy and synchronize container images on Windows.<br>
   Native x64 and ARM64 binaries. No container daemon required.
</p>

<p align="center">
   <a href="https://apps.microsoft.com/detail/9MT5QPKZDHGD">
      <img src="https://get.microsoft.com/images/en-us%20dark.svg" width="280" alt="Download skopeo-win from Microsoft Store">
   </a>
</p>

<p align="center">
   <strong><a href="https://apps.microsoft.com/detail/9MT5QPKZDHGD">Get skopeo-win on Microsoft Store</a></strong><br>
   <a href="https://github.com/rambow-cloud/skopeo-win/releases/latest">Portable downloads</a> &middot;
   <a href="WINDOWS.md">Windows guide</a> &middot;
   <a href="docs/release-notes.md">Release notes</a>
</p>

<p align="center">
   <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="Apache License 2.0"></a>
   <a href="https://github.com/rambow-cloud/skopeo-win/releases/latest"><img src="https://img.shields.io/github/v/release/rambow-cloud/skopeo-win?sort=date" alt="Latest skopeo-win release"></a>
</p>

## Get started on Windows

Install from **[Microsoft Store](https://apps.microsoft.com/detail/9MT5QPKZDHGD)**,
then open PowerShell or Windows Terminal:

```powershell
skopeo-win --version
skopeo-win --help
```

For portable use, download the x64 or ARM64 executable from
**[GitHub Releases](https://github.com/rambow-cloud/skopeo-win/releases/latest)**
and run it with `.\`, for example `.\skopeo-windows-amd64.exe --help`.

## Quick usage

Inspect an image or list its tags without downloading its layers:

```powershell
skopeo-win inspect docker://docker.io/library/nginx:latest
skopeo-win list-tags docker://docker.io/library/nginx
```

Save an image as a Docker archive or as individual local files:

```powershell
skopeo-win copy docker://docker.io/library/nginx:latest 'docker-archive:.\nginx.tar:nginx:latest'
skopeo-win copy docker://docker.io/library/nginx:latest "dir:$PWD\nginx"
```

The archive can be imported with `docker load -i .\nginx.tar` if Docker is
installed. Skopeo itself does not need Docker or a container daemon.

Log in to a private registry with an interactive password prompt:

```powershell
skopeo-win login registry.example.com
```

Enable PowerShell Tab completion once, then restart PowerShell:

```powershell
skopeo-win completion
```

Images default to **Linux** and your computer's native **architecture**. Copy/sync
creates a default policy for selected public namespaces when no policy exists;
private registries and local image sources need an explicit trust policy. See the
[Windows guide](WINDOWS.md) for paths, OCI layouts, sync, policies and completion.
Packaged features follow the [release notes](docs/release-notes.md).

## Command reference

| Task | Command |
| --- | --- |
| Inspect metadata | [`inspect`](docs/skopeo-inspect.1.md) |
| List repository tags | [`list-tags`](docs/skopeo-list-tags.1.md) |
| Copy between registries, archives and directories | [`copy`](docs/skopeo-copy.1.md) |
| Mirror repositories | [`sync`](docs/skopeo-sync.1.md) |
| Manage registry credentials | [`login`](docs/skopeo-login.1.md), [`logout`](docs/skopeo-logout.1.md) |

Use `skopeo-win COMMAND --help` or the [full reference](docs/skopeo.1.md) for more
commands and options. [Installation](install.md) and [building](WINDOWS.md#build)
use native Windows binaries with no third-party C runtime.

## About this project

Rambow Cloud independently maintains this Windows distribution, originally based
on Skopeo v1.23.0. It uses its own release numbers and ports selected features and
fixes through reviewed issues and PRs. Linux container storage drivers and the
experimental `proxy` command are outside its Windows scope.

Report problems using the [issue templates](https://github.com/rambow-cloud/skopeo-win/issues/new/choose).
See [contributing](CONTRIBUTING.md), the [release policy](docs/release-policy.md)
and the [upstream assessment](docs/upstream-gap-2026-11-02.md) for maintenance details.

Licensed under [Apache 2.0](LICENSE). Applicable upstream notices are preserved.
