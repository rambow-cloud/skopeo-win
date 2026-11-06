% skopeo-login(1)

## NAME
skopeo\-login - Login to a container registry.

## SYNOPSIS
**skopeo-win login** [*options*] _registry_

## DESCRIPTION
**skopeo-win login** logs into a specified registry server with the correct username
and password. **skopeo-win login** reads in the username and password from STDIN.
The username and password can also be set using the **username** and **password** flags.
The path of the credentials file can be specified by the user by setting the **authfile**
flag.

## OPTIONS

See also [skopeo(1)](skopeo.1.md) for options placed before the subcommand name.

**--password**, **-p**=*password*

Password for registry

**--password-stdin**

Take the password from stdin

**--username**, **-u**=*username*

Username for registry

**--authfile**=*path*

Path of the managed registry credentials file. On Windows, the default is
`$env:USERPROFILE\.config\containers\auth.json`. Override it with `--authfile`
or `$env:REGISTRY_AUTH_FILE`.

The default value of this option is read from the `REGISTRY\_AUTH\_FILE` environment variable.

**--compat-auth-file**=*path*

Instead of updating the default credentials file, update the one at *path*, and use a Docker-compatible format.

**--get-login**

Return the logged-in user for the registry. Return error if no login is found.

**--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry.
On Windows, select a certificate directory explicitly, for example `--cert-dir .\certs`.

**--help**, **-h**

Print usage statement

**--tls-verify**=_bool_

Require HTTPS and verify certificates when talking to the container registry or daemon. Default to registry.conf setting.

**--verbose**, **-v**

Write more detailed information to stdout

## EXAMPLES

Login with an interactive password prompt; no password is placed in the command:

```powershell
skopeo-win login registry.example.com
skopeo-win login --authfile .\auth.json registry.example.com
skopeo-win login --cert-dir .\certs registry.example.com
```

For automation, pipe a secret supplied by your credential provider to
`skopeo-win login --username USER --password-stdin registry.example.com`.

## SEE ALSO
skopeo(1), skopeo-logout(1), containers-auth.json(5), containers-registries.conf(5), containers-certs.d.5.md

## HISTORY
May 2020, Originally compiled by Qi Wang <qiwan@redhat.com>
