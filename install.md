# Install skopeo on Windows

## Microsoft Store

Install **[skopeo-win](https://apps.microsoft.com/detail/9MT5QPKZDHGD)**, then open
PowerShell or Windows Terminal:

```powershell
skopeo --version
skopeo --help
```

## Portable executable

Download `skopeo-windows-amd64.exe` for x64 Windows or
`skopeo-windows-arm64.exe` for ARM64 Windows from
[GitHub Releases](https://github.com/rambow-cloud/skopeo-win/releases/latest).
In the download directory, run:

```powershell
Rename-Item .\skopeo-windows-amd64.exe skopeo.exe
.\skopeo.exe --version
.\skopeo.exe inspect docker://docker.io/library/nginx:latest
```

Use the ARM64 download filename for that build. Add the executable's directory
to `PATH` to use `skopeo` from any directory.
Docker, WSL, MSYS2 and third-party C runtimes are unnecessary.

See the [quick usage](README.md#quick-usage), [Windows guide](WINDOWS.md) and
[source build instructions](WINDOWS.md#build).
