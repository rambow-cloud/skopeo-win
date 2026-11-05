# Independent skopeo-win releases

skopeo-win is maintained independently. Its historical origin is upstream Skopeo v1.23.0; this does not determine future release numbers. Maintenance consists of bug fixes, security and dependency updates, and selected feature ports through issues and reviewed PRs. Bulk upstream source merges are not part of this process.

## Maintainer-selected versions

Main pushes and merges, PRs, and normal manual workflow dispatches run builds, integration tests and vulnerability scans without publishing. They use `0.0.0-dev.<commit>` versions and do not produce Store bundles or automatically submit Store updates. Local builds default to `0.0.0-dev`.

Accumulate reviewed features on main, then create and push a stable `vX.Y.Z` tag to publish the chosen version. Normally increment the patch number, for example `v1.0.14` to `v1.0.15`; minor and major releases are also explicit choices. The exact tag version is embedded in both executables and checked through `--version`. There is no automatic version allocation.

The selected version must be newer than other reserved stable tags and releases, including drafts and orphan tags. The historical `v1.23.0` is excluded from comparisons and permanently reserved. Major versions must be positive and each component must fit the Store package limit of 65535. Prerelease tags and versions with leading zeroes are rejected.

Only a tag **push** requests publication. Lightweight and annotated tags are supported, and the source commit must belong to main history. Manual dispatch against a tag remains a development build. A successful tagged release continues to trigger the existing Store automation. Manual Store recovery verifies the public release, source workflow, main ancestry and artifact; main can advance without invalidating a reviewed release tag.

Store submissions have separate, durable publication tracking issues. Actions checks them hourly and after submission workflows, updating only changed status. Only Microsoft's `Published` status plus matching submission/package versions closes the associated tracker. Intermediate states and failures keep it open. Ordinary issue text cannot opt a human-created issue or PR into automatic closure; reviewed migration links cover pre-existing delivery issues. Registration is a separate task so an issue API failure can be retried without resubmitting a package. Store publication status checks do not create releases or change submitted packages.

## Create a release tag

After the desired changes have been merged and checked, choose an unused higher version and tag the intended main commit. For example, when `v1.0.14` is the latest reserved version:

```powershell
git switch main
git pull --ff-only origin main
git tag -a v1.0.15 -m "skopeo-win v1.0.15"
git push origin v1.0.15
```

Push one release tag at a time. The tagged source must contain the tag-driven workflow; older workflow revisions retain their original behavior. Do not create an empty GitHub release first: the workflow owns draft creation and immutable asset publication. Do not move or recreate a tag to retry publication; rerun the original workflow's failed job instead.

## Publication and retries

Every PR updates [the reviewed release summary](release-notes.md) with user-facing feature descriptions, defaults, compatibility limits and accurate verification scope. The summary accumulates reviewed content for the next tagged publication; it is not a claim that every capability is newly introduced in that patch. The publisher combines the summary from the tagged source with the selected version, exact commit, workflow link and binary limitations. Empty or missing summaries fail publication. Later edits affect future publications only.

Publication requires the Windows executable integration suite, command/release tests and vulnerability gates. See [Windows integration tests](windows-integration-tests.md) for coverage and local execution. Tag workflows share a serialized release concurrency group. The publisher requires the maintainer-created tag to exist, resolves annotated tags to their source commit and rejects mismatched source or ownership. It never creates, moves or deletes release tags.

After the build, tests and scans pass, the publication job:

1. Requires exactly two executables and their two SHA-256 files and verifies their checksums.
2. Creates a draft release for the existing tag and exact build commit.
3. Uploads only missing assets, without `--clobber`.
4. Rechecks the draft ID, workflow ownership, tag commit and server-side asset digests before publishing by ID and marking Latest.

If an upload fails, the draft stays unpublished. Rerunning the failed publication job resumes missing files but refuses existing assets with different content. A retry of the same already-public release verifies its tag and all four digests without editing it. A newer reserved version can block an older retry; investigate rather than changing existing tags or assets.

Existing public releases with missing or mismatched files are not repaired in place. Investigate and publish a new version instead. Maintainers may explicitly correct release-description text without changing tags or binary assets. These safeguards constrain this publisher; repository administrators can still independently change GitHub release state through other tools.

## Historical release

The old `v1.23.0` release was rebuilt in place under the previous policy. Its original tag does not identify the later replacement assets. Its release notes record the corresponding source commit and build. Keep that historical record unchanged; use independent releases for all new distributions.
