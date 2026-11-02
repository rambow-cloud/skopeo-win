# Microsoft Store listing draft

Product ID: `9MT5QPKZDHGD`  
Display name: `skopeo-win`  
Publisher display name: `RambowCloud`  
Suggested category: Developer tools  
Price: Free

## Short description

Inspect, copy and synchronize container images from the Windows terminal.

## Description

skopeo-win brings a native container-image command-line tool to Windows. Inspect
remote images, copy images between registries and local image formats, and
synchronize image repositories without running a container daemon.

Work with OCI and Docker image formats, authenticate to private registries, and
use explicit signature policies for image trust. Native x64 and ARM64 packages
are provided without a third-party C runtime dependency.

This is a terminal application. After installation, open PowerShell, Command
Prompt or Windows Terminal and run `skopeo --help`. The additional command name
`skopeo-win` is available if another installation already uses `skopeo`.

Independently maintained by Rambow Cloud, this project originated from Skopeo
and uses its own release versions. See the repository for Windows feature scope,
source code, licensing and support. This application manages container images;
it does not run containers or provide a graphical interface.

## Features

- Inspect remote container-image metadata and enumerate repository tags.
- Copy images between container registries and supported local image formats.
- Synchronize image repositories for migration and offline distribution workflows.
- Select platforms when copying multi-platform images.
- Authenticate to private registries and apply image signature policies.
- Run natively on Windows x64 and ARM64.

## Website, support and privacy

- Website: https://github.com/rambow-cloud/skopeo-win
- Support: https://github.com/rambow-cloud/skopeo-win/issues
- Privacy: https://github.com/rambow-cloud/skopeo-win/blob/main/docs/privacy-policy.md

## Certification notes

The Start-menu entry opens a console usage window that waits for Enter before
closing. The actual CLI is available through execution aliases. Open PowerShell
or Windows Terminal after installation and run:

```powershell
skopeo-win --version
skopeo-win --help
skopeo-win inspect --help
skopeo-win sync --help
```

These checks do not require an account, network access, administrator privileges,
or a container runtime. Registry operations require network access; private
registries require credentials supplied by the user. No test account is needed
for the offline checks above. App execution aliases must be enabled in Windows
Settings. An existing executable earlier on PATH can take precedence over an
alias.

The `runFullTrust` capability is required to run the existing Win32 console
executable and its usage-window launcher outside AppContainer. CLI commands make
registry connections and read/write user-selected image, configuration, credential
and signature files. The launcher displays local usage instructions and performs
no network operations. Neither executable requests automatic administrator
elevation. This is the justification for the restricted capability approval;
the application uses a visible default entry and does not require a headless waiver.

## Screenshots and declarations

Capture actual Windows Terminal screenshots of the installed Store package.
Include `skopeo-win --help`, an image inspection, and a successful local image
copy. Remove usernames, credentials and private registry information. At least
one real screenshot is required before submission.

Complete age ratings, availability, account/contact details, and the restricted
capability declaration in Partner Center. Do not treat this draft as evidence of
certification, Store publication, or installation testing.
