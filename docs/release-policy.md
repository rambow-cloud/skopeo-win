# Independent skopeo-win releases

skopeo-win is maintained independently. Its historical origin is upstream Skopeo v1.23.0; this does not determine future release numbers. Maintenance consists of bug fixes, security and dependency updates, and selected feature ports through issues and reviewed PRs. Bulk upstream source merges are not part of this process.

## Version sequence

The independent release sequence starts at `v1.0.0`. Each new release increments the patch number: `v1.0.1`, `v1.0.2`, and so on. Allocation considers existing stable tags and releases, including visible drafts and orphan tags, to avoid reuse. A deliberately introduced higher major or minor release becomes the new counter base. The legacy tag `v1.23.0` is excluded and reserved permanently.

Successful pushes to `main` and manual workflow dispatches on `main` produce releases. A new manual run receives a new version even for the same source commit. Rerunning a workflow that already has a release associated with its run ID reuses that version and verifies the existing release. Failed runs without reserved tags do not consume a version. Interrupted drafts may reserve version numbers; numbering is monotonic but gaps are permitted.

PR builds use `0.0.0-dev.<commit>` and local builds default to `0.0.0-dev`. `version.Version` is a development fallback, not a release counter. CI resolves the independent version before compiling and injects it into both executables with the Go linker. It checks the native executable's `--version` output before publication.

## Publication and retries

Main workflows are serialized by the workflow concurrency group. A publisher rejects tags pointing to another commit and releases belonging to another workflow run. If a separately created tag collides with an allocation, publication fails instead of moving the tag or replacing files.

After the build, tests, and vulnerability checks pass, the publication job:

1. Requires exactly two executables and their two SHA-256 files and verifies the checksums.
2. Creates a draft release targeting the exact build commit.
3. Uploads only missing assets, without `--clobber`.
4. Checks the server-side digest of every asset before making the release public and marking it Latest.

If an upload fails, the draft stays unpublished. Rerunning the failed publication job resumes missing files, but refuses any existing asset with different content. If publication already completed, a retry only verifies the tag and all four asset digests and returns without changing the release. Existing public releases with missing or mismatched files are not repaired in place; investigate and publish a new version instead.

Do not delete/recreate published tags, overwrite assets, or edit a released binary. This is enforced by the project's publisher; it does not claim that GitHub repository administrators are technically unable to change a release through other tools.

## Historical release

The old `v1.23.0` release was rebuilt in place under the previous policy. Its original tag does not identify the later replacement assets. Its release notes record the corresponding source commit and build. Keep that historical record unchanged; use independent releases for all new distributions.
