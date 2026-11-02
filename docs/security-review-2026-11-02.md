# Private security review: recent independent-maintenance changes

This report and its patch remain private pending maintainer disclosure coordination under [SECURITY.md](../SECURITY.md). Private draft: [GHSA-94rg-p8r7-w6f3](https://github.com/rambow-cloud/skopeo-win/security/advisories/GHSA-94rg-p8r7-w6f3). No CVE or public disclosure is claimed.

## Scope

Reviewed the custom changes ending at main `9dd040cb2f4fdbcdc4054f2af13339ac76a93292`, including independent publication, selected copy/sync feature ports, Windows path handling, workflow permissions, key diagnostics and the recently introduced tests. This is a review of the project's changes with production vulnerability scans, not a complete manual audit of every vendored dependency.

## Findings, ordered by impact

### P2: key-output validation does not enforce non-overwrite during creation

The inherited `generate-sigstore-key` path checked output existence, generated keys, then used `os.WriteFile`. An actor able to change the output directory between validation and writing could introduce a hard link and cause an unrelated file writable by the invoker to be overwritten. A dangling symlink could also pass the former `Stat` preflight. Exploitation depends on local directory/link permissions and the invoker's target-file permissions; remote exploitation, unconditional privilege escalation and exploitation in the wild have not been established.

The recent feature ports did not introduce this pattern. The review found it in a command recently modified for diagnostics and exercised by new tests. An isolated Windows executable regression confirmed both `.private` and `.pub` cases: before the fix, the command exited successfully and changed the sentinel file outside its output directory.

The patch uses `Lstat` for early diagnostics and exclusive `OpenFile(O_WRONLY|O_CREATE|O_EXCL)` for the actual write, checking write and close errors. The atomic creation is the enforcement boundary. Both executable attack simulations now reject the output and retain the sentinel. Real-filesystem unit tests check existing objects and 32 competing creators with exactly one winner. The [Go file API](https://pkg.go.dev/os#OpenFile) documents creation flags and error behavior.

This protects the final output entry; it is not a sandbox for attacker-controlled parent directories. Use a trusted output directory. Windows ACL inheritance is separate from Unix mode bits, and this change does not claim to enforce a private Windows DACL or to encrypt keys beyond the command's existing passphrase mechanism.

### Publication state race: defense in depth and consistency hardening

The old publisher validated workflow ownership before upload, then only checked tag/digests in its final snapshot before editing a release by tag. A changed/replaced draft or a concurrently published release was not fully rechecked. Another actor already needs repository release-write authority to cause those transitions; this is not evidence of an unprivileged repository takeover.

The patch captures the numeric draft ID from the create response, checks ID/ownership/tag/assets again after upload, and publishes by that ID. It rejects replacement or ownership changes, and performs no edit when the same verified release is already public. This narrows the race but does not claim a global transaction over other administrators' writes after the last snapshot.

The rewritten service model computes asset digests from actual fixture bytes, rejects unsupported commands and replacements, and injects post-upload state transitions. Regression cases cover changed owner, replaced ID, moved tag, modified asset, already-public state, malformed draft response, interrupted upload and read-only published retry.

### Testing blind spots corrected

- Earlier copy tests used an insecure fixture policy, so passing them did not demonstrate trust enforcement. New executable tests explicitly reject copy/sync under configured trust policy despite signature-removal/download options, and verify that rejected images make no external layer request.
- Added executable cryptographic checks for a valid trusted image signature and rejection of changed manifest data, changed signature packets, wrong identity/fingerprint, and unsigned input. Mutation targets real signature packets; corrupting only a compressed wrapper can leave authenticated packets unchanged.
- Replaced the key-generation smoke test with filesystem security assertions; removed the redundant in-process feature suite already covered by executable tests. Retained established upstream command tests and the existing pure-Go OpenPGP verification tests.
- Replaced the recent positional release fake with an independent, stricter state model. Added a fuzz property that neither stable refs nor draft reservations can be reused by version allocation.

## Areas reviewed without a newly demonstrated bypass

- Copy/sync flags are explicitly opt-in and still pass through the original source policy context. No newly introduced remote command execution or signature-policy bypass was established.
- The directory-sync fix converts native relative filesystem paths to registry names and rejects lexical escape from the source base. It does not introduce filesystem deletion or a new arbitrary destination path from registry metadata.
- GitHub Actions use pinned action revisions, read-only build permissions, a separate write-enabled publication job, no `pull_request_target` execution, and no interpolation of PR titles into shell code. GitHub's [secure-use guidance](https://docs.github.com/en/actions/reference/security/secure-use) describes the token and injection boundaries used in this assessment.
- Publisher commands use structured argument arrays, strict version/environment validation and hash comparison. Reviewed notes come from trusted repository content. No new credential exposure was confirmed; visible integration credentials are synthetic fixtures.
- Foreign-layer URLs are external network input processed by the image library. Opt-in downloads are not an offline guarantee or a network-egress sandbox. The existing retained-URL limitation remains documented; it was not reclassified as a new remote vulnerability without a separate threat model.

## Local verification

- Both pre-patch executable key-race scenarios reproduced the overwrite in isolated temporary files.
- After the patch, the complete command/release suite passed. One optional symlink unit scenario can skip when Windows denies symlink creation; hard-link, concurrent-create and executable race checks do not require that privilege.
- Complete executable integration: **19 top-level tests, 70 entries including subtests, 0 failures, 0 skipped tests, 112 executable invocations**.
- Windows amd64 and arm64 builds passed CGO/OpenPGP/PE-import checks. ARM64 was not executed.
- Source and both binary vulnerability gates passed with zero applicable findings. The existing one module-only advisory remains visible without suppression; these scans cannot detect an unpublished logic flaw such as the confirmed key race.
- Version-allocation fuzzing passed 683,894 evaluations in 15 seconds with two workers. No live public release API mutation was used as a test.

The patch is available in [private PR #1](https://github.com/rambow-cloud/skopeo-win-ghsa-94rg-p8r7-w6f3/pull/1). GitHub does not run status checks or expose CI integrations in temporary security-advisory forks, as its [private-fork documentation](https://docs.github.com/en/code-security/tutorials/fix-reported-vulnerabilities/collaborate-in-a-fork) explicitly states. Consequently these are local verification results, not a claimed private CI pass. The public release workflow must validate the eventual disclosed merge before publishing a fixed distribution.

## Delivery boundary

The fix, report and reproduction belong in the temporary private advisory fork. Public main and its immutable releases remain unchanged until the maintainers coordinate disclosure, including the inherited-code question with upstream. No upstream messages or CVE request are part of this review.
