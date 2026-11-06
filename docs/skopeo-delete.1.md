% skopeo-delete(1)

## NAME
skopeo\-delete - Mark the _image-name_ for later deletion by the registry's garbage collector.

## SYNOPSIS
**skopeo-win delete** [*options*] _image-name_

## DESCRIPTION

Mark _image-name_ for deletion.
The effect of this is registry-specific; many registries don’t support this operation, or don’t allow it in some circumstances / configurations.

**WARNING**: If _image-name_ contains a digest, this affects the referenced manifest, and may delete all tags (within the current repository?) pointing to that manifest.

**WARNING**: If _image-name_ contains a tag (but not a digest), in the current version of Skopeo this resolves the tag into a digest, and then deletes the manifest by digest, as described above (possibly deleting all tags pointing to that manifest, not just the provided tag). This behavior may change in the future.


Reclaiming storage requires registry-side garbage collection, managed by the
registry administrator. It is not a local Windows cleanup command.

## OPTIONS

See also [skopeo(1)](skopeo.1.md) for options placed before the subcommand name.

**--authfile** _path_

Path of the primary registry credentials file. On Windows, the default is
`$env:USERPROFILE\.config\containers\auth.json`. Override it with `--authfile`
or `$env:REGISTRY_AUTH_FILE`.

Use `skopeo-win login` to manage the credentials.

The default value of this option is read from the `REGISTRY\_AUTH\_FILE` environment variable.

**--creds** _username[:password]_

Credentials for accessing the registry.

**--cert-dir** _path_

Use certificates at _path_ (*.crt, *.cert, *.key) to connect to the registry.

**--daemon-host** _host_

Use docker daemon host at _host_ (`docker-daemon:` transport only)

**--help**, **-h**

Print usage statement

**--no-creds**

Access the registry anonymously.

Additionally, the registry must allow deletions by setting `REGISTRY_STORAGE_DELETE_ENABLED=true` for the registry daemon.

**--registry-token** _token_

Bearer token for accessing the registry.

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

## EXAMPLES

Delete a manifest from a registry where you have deletion permission:

```powershell
skopeo-win delete docker://registry.example.com/team/nginx:old
```

This can affect other tags pointing to the same manifest, as described above.

## SEE ALSO
skopeo(1), skopeo-login(1), docker-login(1), containers-auth.json(5)

## AUTHORS

Antonio Murdaca <runcom@redhat.com>, Miloslav Trmac <mitr@redhat.com>, Jhon Honce <jhonce@redhat.com>
