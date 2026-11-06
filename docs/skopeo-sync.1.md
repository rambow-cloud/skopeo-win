% skopeo-sync(1)

## NAME
skopeo\-sync - Synchronize images between registry repositories and local directories.


## SYNOPSIS
**skopeo sync** [*options*] --src _transport_ --dest _transport_ _source_ _destination_

## DESCRIPTION
Synchronize images between registry repositories and local directories. Synchronization is achieved by copying all the images found at _source_ to _destination_ - useful when synchronizing a local container registry mirror or for populating registries running inside of air-gapped environments.

Differently from other skopeo commands, skopeo sync requires both source and destination transports to be specified separately from _source_ and _destination_.
One of the problems of prefixing a destination with its transport is that, the registry `docker://hostname:port` would be wrongly interpreted as an image reference at a non-fully qualified registry, with `hostname` and `port` the image name and tag.

Available _source_ transports:
 - _docker_ (i.e. `--src docker`): _source_ is a repository hosted on a container registry (e.g.: `registry.example.com/busybox`).
 If no image tag is specified, skopeo sync copies all the tags found in that repository.
 - _dir_ (i.e. `--src dir`): _source_ is a local directory path (e.g.: `.\mirror`). Refer to skopeo(1) **dir:**_path_ for the local image format.
 - _yaml_ (i.e. `--src yaml`): _source_ is local YAML file path.
 The YAML file should specify the list of images copied from different container registries (local directories are not supported). Refer to EXAMPLES for the file format.

Available _destination_ transports:
 - _docker_ (i.e. `--dest docker`): _destination_ is a container registry (e.g.: `my-registry.local.lan`).
 - _dir_ (i.e. `--dest dir`): _destination_ is a local directory path (e.g.: `.\mirror`).
 One directory per source 'image:tag' is created for each copied image.

When the `--scoped` option is specified, images are prefixed with the source image path so that multiple images with the same
name can be stored at _destination_.

## OPTIONS

See also [skopeo(1)](skopeo.1.md) for options placed before the subcommand name.

**--all**, **-a**
If one of the images in __src__ refers to a list of images, instead of copying just the image which matches the current OS and
architecture (subject to the use of the global --override-os, --override-arch and --override-variant options), attempt to copy all of
the images in the list, and the list itself.

**--authfile** _path_

Path of the primary registry credentials file. On Windows, the default is
`$env:USERPROFILE\.config\containers\auth.json`. Override it with `--authfile`
or `$env:REGISTRY_AUTH_FILE`.

Use `skopeo login` to manage the credentials.

The default value of this option is read from the `REGISTRY\_AUTH\_FILE` environment variable.

**--src-authfile** _path_

Path of the primary registry credentials file for the source registry. Uses path given by `--authfile`, if not provided.

**--dest-authfile** _path_

Path of the primary registry credentials file for the destination registry. Uses path given by `--authfile`, if not provided.

**--dry-run**

Run the sync without actually copying data to the destination.
Destination directories and digest files are not created in this mode.

On Windows, registry-to-directory sync encodes reserved characters, uppercase
tag characters, percent signs, and reserved device names using a `~skopeo~`
prefix and percent escapes. This keeps tags such as `stable` and `Stable`
distinct on case-insensitive filesystems. Directory-to-registry sync decodes
these marked components; ordinary existing directory names remain unchanged.
Normal Windows path and component length limits still apply.

For a scoped export, restore from the directory representing the source
registry and use `--scoped` to retain its repository namespaces. For example,
an exported `team/image:stable` becomes `team/~skopeo~image%3astable` beneath
that registry directory. The registry directory may itself be encoded if its
address includes a port. Unscoped exports place image directories directly
under the chosen destination.

Invalid selection configuration and registry tag-discovery errors fail the
command, including YAML configurations with otherwise valid selections.
A valid filter matching no tags remains a successful empty selection.

**--src**, **-s** _transport_ Transport for the source repository.

**--dest**, **-d** _transport_ Destination transport.

**--format**, **-f** _manifest-type_ Manifest Type (oci, v2s1, or v2s2) to use when syncing image(s) to a destination (default is manifest type of source, with fallbacks).

**--help**, **-h**

Print usage statement.

**--scoped** Prefix images with the source image path, so that multiple images with the same name can be stored at _destination_.

**--append-suffix** _tag-suffix_ String to append to destination tags.

**--digestfile** _path_

After copying the images from source, write the digest of the resulting images along with Image Reference.

```
sha256:bf91f90823248017a4f920fb541727fa8368dc6cf377a7debbd271cf6a31c8a7 docker://myhost.com/alpine:edge
sha256:31603596830fc7e56753139f9c2c6bd3759e48a850659506ebfb885d1cf3aef5 docker://myhost.com/postgres:14.3

```

**--preserve-digests**

Preserve the digests during copying. Fail if the digest cannot be preserved.

This option does not change what will be copied; consider using `--all` at the same time.

**--remove-signatures** Do not copy signatures, if any, from _source-image_. This is necessary when copying a signed image to a destination which does not support signatures.

**--remove-list-signatures** Remove manifest-list signatures while retaining per-instance signatures when copying lists with **--all**. Mutually exclusive with **--remove-signatures**; not applicable to single-image copies.

**--download-foreign-layers** Download nondistributable (foreign) layers even if the destination supports external layer URLs. Disabled by default. Ensure that the layer licenses permit redistribution. Selectively ported from upstream main after v1.24.1.

The manifest may retain external URLs after download. Subsequent pulls can still require the external server; downloading alone does not guarantee an offline mirror.

**--sign-by** _key-id_

Add a “simple signing” signature using that key ID for an image name corresponding to _destination-image_

**--sign-by-sigstore** _param-file_

Add a sigstore signature based on the options in the specified containers sigstore signing parameter file, _param-file_.
See containers-sigstore-signing-params.yaml(5) for details about the file format.

**--sign-by-sigstore-private-key** _path_

Add a sigstore signature using a private key at _path_ for an image name corresponding to _destination-image_

**--sign-by-sq-fingerprint** _fingerprint_

Add a “simple signing” signature using a Sequoia-PGP key with the specified _fingerprint_.

**--sign-passphrase-file** _path_

The passphrase to use when signing with `--sign-by`, `--sign-by-sigstore-private-key` or `--sign-by-sq-fingerprint`.
Only the first line will be read. A passphrase stored in a file is of questionable security if other users can read this file. Do not use this option if at all avoidable.

**--src-creds** _username[:password]_ for accessing the source registry.

**--dest-creds** _username[:password]_ for accessing the destination registry.

**--src-cert-dir** _path_ Use certificates (*.crt, *.cert, *.key) at _path_ to connect to the source registry or daemon.

**--src-no-creds** Access the registry anonymously.

**--src-tls-verify**=_bool_ Require HTTPS and verify certificates when talking to a container source registry or daemon. Default to source registry entry in registry.conf setting.

**--dest-cert-dir** _path_ Use certificates (*.crt, *.cert, *.key) at _path_ to connect to the destination registry or daemon.

**--dest-no-creds** Access the registry anonymously.

**--dest-tls-verify**=_bool_ Require HTTPS and verify certificates when talking to a container destination registry or daemon. Default to destination registry entry in registry.conf setting.

**--src-registry-token** _Bearer token_ for accessing the source registry.

**--dest-registry-token** _Bearer token_ for accessing the destination registry.

**--retry-times**

The number of times to retry. By default, no retries are attempted.

**--retry-delay**

Fixed delay between retries. If not set (or set to 0s), retry wait time will be exponentially increased based on the number of failed attempts.

**--keep-going**
If any errors occur during copying of images, those errors are logged and the process continues syncing rest of the images and finally fails at the end.

**--src-username**

The username to access the source registry.

**--src-password**

The password to access the source registry.

**--dest-username**

The username to access the destination registry.

**--dest-password**

The password to access the destination registry.

## EXAMPLES

Preview or export a single tag in PowerShell:

```powershell
skopeo sync --dry-run --src docker --dest dir docker.io/library/nginx:latest .\mirror
skopeo sync --src docker --dest dir docker.io/library/nginx:latest .\mirror
```

Windows exports encode `nginx:latest` as `~skopeo~nginx%3alatest`. Keep names
unchanged when restoring the mirror:

```powershell
skopeo login registry.example.com
skopeo sync --src dir --dest docker .\mirror registry.example.com/team
```

To select several images, save this as `sync.yml`:

```yaml
docker.io:
  images:
    library/nginx: ["latest"]
    library/redis: ["latest"]
```

```powershell
skopeo sync --src yaml --dest dir .\sync.yml .\mirror-selected
```

Omitting a tag selects all repository tags. Use `--scoped` to retain source
registry/repository namespaces, as described above. The default policy accepts
unsigned sources; use a custom [trust policy](../WINDOWS.md#image-trust-policy)
to enforce your own source/signature requirements.

## SEE ALSO
skopeo(1), skopeo-login(1), docker-login(1), containers-auth.json(5), containers-policy.json(5), containers-transports(5)

## AUTHORS

Flavio Castelli <fcastelli@suse.com>, Marco Vedovati <mvedovati@suse.com>
