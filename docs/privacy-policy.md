# skopeo-win privacy policy

Effective date: November 2, 2026

skopeo-win is an independently maintained, open-source command-line application
provided by Rambow Cloud. Its source code is available at
<https://github.com/rambow-cloud/skopeo-win>.

## Application data

The application does not include Rambow Cloud analytics, advertising, or telemetry
services. Rambow Cloud does not operate an application backend that receives your
container images, registry credentials, or command history.

The application reads and writes files needed for the commands you run, including
container images, configuration, authentication files, signatures, and generated
keys. These files remain on your computer unless you request an operation that
transfers them to another location or service.

## Connections to other services

Commands can communicate with container registries, authentication services, image
layer download locations, proxies, and signing or verification services. Those
services may receive your IP address, requested image names, authentication
information, and the image or signature data needed for the requested operation.
Layer locations can be supplied by image manifests. Service operators process
that information under their own policies.

You select registries and services through command arguments, configuration, and
the images you work with. Only use services and images you trust. Microsoft
processes Store installation and distribution data under its own privacy terms.

## Credentials and local storage

Login commands may save credentials in local authentication files or configured
credential helpers. Authentication files are not necessarily encrypted. Protect
these files and generated private keys with appropriate filesystem permissions.
Use logout commands and remove files you no longer need; uninstalling the Store
package does not necessarily remove files you saved outside package-managed data.

Diagnostic output can contain image names, paths, or other information about your
commands. Review logs before sharing them. Do not post credentials or private
keys in public issues.

## Contact and changes

For privacy questions, open an issue at
<https://github.com/rambow-cloud/skopeo-win/issues>. To report a security concern
privately, follow
<https://github.com/rambow-cloud/skopeo-win/security/policy>.

Changes to this policy are reviewed in the public repository. The effective date
above identifies the current policy.
