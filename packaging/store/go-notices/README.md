# Pinned Go root legal files

Hosted Go installations can omit the root `LICENSE` and `PATENTS` files while
retaining runtime source. These snapshots supply missing root files only when
the actual binary, Go source `VERSION`, and snapshot `goVersion` all match.
Source copyright headers and nested notices still come from the matching Go
source tree. A missing runtime source tree is rejected.

`go1.27.3/LICENSE` and `go1.27.3/PATENTS` were retrieved without editing from the
official Go release commit
[`2aae1000ea1d4b28923b96db28acf4e3b99d6aff`](https://github.com/golang/go/tree/2aae1000ea1d4b28923b96db28acf4e3b99d6aff),
identified by the `go1.27.3` release tag. `source.json` records that immutable
commit and the SHA-256 of each file. Git preserves these files byte-for-byte.

The generator checks the version, repository, commit format and file hash before
using a snapshot, and records its source path and immutable upstream URL in the
package inventory. It performs no network download during generation. New Go
versions need an explicitly reviewed snapshot if their installation also omits
root legal files; unrelated versions and unverified snapshots are rejected.
