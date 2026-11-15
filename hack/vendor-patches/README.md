# Reviewed compatibility sources

The Distribution API migration uses the versioned image/common source forks in
`packaging/store/module-sources.json`. They preserve the original upstream release
baselines, migrate registry API types together and include the Windows archive fix.
`go mod vendor` therefore restores the reviewed Windows fix without an additional
patch application. Do not apply `image-windows-archive-paths.patch` twice.

Source updates require review of both immutable commits, the upstream release
provenance and the complete vendored-content digests. Run
`hack/verify-module-sources.ps1`, the Windows executable integration suite and Store
package/notice verification. The canonical upstream advisory audit must also pass.
Preserve English-only documentation when regenerating vendor.

The original archive patch remains available as a record of the compatibility
change and for a future migration back to unmodified upstream modules.
