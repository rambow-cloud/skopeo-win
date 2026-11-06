% skopeo-inspect(1)

## NAME
skopeo\-inspect - Return low-level information about _image-name_ in a registry.

## SYNOPSIS
**skopeo-win inspect** [*options*] _image-name_

## DESCRIPTION

Return low-level information about _image-name_ in a registry.
See [skopeo(1)](skopeo.1.md) for the format of _image-name_.

The default output includes data from various sources: user input (**Name**), the remote repository, if any (**RepoTags**), the top-level manifest (**Digest**),
and a per-architecture/OS image matching the current run-time environment (most other values).
To see values for a different architecture/OS, use the **--override-os** / **--override-arch** options documented in [skopeo(1)](skopeo.1.md).

## OPTIONS

See also [skopeo(1)](skopeo.1.md) for options placed before the subcommand name.

**--authfile** _path_

Path of the primary registry credentials file. On Windows, the default is
`$env:USERPROFILE\.config\containers\auth.json`. Override it with `--authfile`
or `$env:REGISTRY_AUTH_FILE`.

Use `skopeo-win login` to manage the credentials.

The default value of this option is read from the `REGISTRY\_AUTH\_FILE` environment variable.

**--cert-dir** _path_

Use certificates at _path_ (\*.crt, \*.cert, \*.key) to connect to the registry.

**--config**

Output configuration in OCI format, default is to format in JSON format.

**--creds** _username[:password]_

Username and password for accessing the registry.

**--daemon-host** _host_

Use docker daemon host at _host_ (`docker-daemon:` transport only)

**--format**, **-f**=*format*

Format the output using the given Go template.
The keys of the returned JSON can be used as the values for the --format flag (see examples below).
Supports the Go templating functions available at https://pkg.go.dev/github.com/containers/common/pkg/report#hdr-Template_Functions

**--help**, **-h**

Print usage statement

**--no-creds**

Access the registry anonymously.

**--raw**

Output raw manifest or config data depending on --config option.
The --format option is not supported with --raw option.

**--registry-token** _Bearer token_

Registry token for accessing the registry.

**--retry-times**

The number of times to retry. By default, no retries are attempted.

**--retry-delay**

Fixed delay between retries. If not set (or set to 0s), retry wait time will be exponentially increased based on the number of failed attempts.

**--shared-blob-dir** _directory_

Directory to use to share blobs across OCI repositories.

**--tls-verify**=_bool_

Require HTTPS and verify certificates when talking to the container registry or daemon. Default to registry.conf setting.

**--username**

The username to access the registry.

**--password**

The password to access the registry.

**--no-tags**, **-n**

Do not list the available tags from the repository in the output. When `true`, the `RepoTags` array will be empty.  Defaults to `false`, which includes all available tags.

**--manifest-digest**=_algorithm_ **EXPERIMENTAL**

Algorithm to use for computing manifest digest (sha256, sha512); defaults to algorithm used in config digest.

**Note:** This flag is experimental and its behavior may change in future releases.

## EXAMPLES

```powershell
skopeo-win inspect docker://docker.io/library/nginx:latest
skopeo-win inspect --no-tags --format '{{.Digest}}' docker://docker.io/library/nginx:latest
skopeo-win inspect --config docker://docker.io/library/nginx:latest | ConvertFrom-Json
skopeo-win inspect --manifest-digest sha512 --format '{{.Digest}}' docker://docker.io/library/nginx:latest
```

## SEE ALSO
skopeo(1), skopeo-login(1), docker-login(1), containers-auth.json(5)

## AUTHORS

Antonio Murdaca <runcom@redhat.com>, Miloslav Trmac <mitr@redhat.com>, Jhon Honce <jhonce@redhat.com>
