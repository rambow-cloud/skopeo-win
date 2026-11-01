# Security policy

## Reporting a vulnerability

For new or undisclosed vulnerabilities affecting this Windows distribution,
submit a [private vulnerability report](https://github.com/rambow-cloud/skopeo-win/security/advisories/new).
Do not open a public issue or pull request containing an undisclosed vulnerability.

Include the affected version or commit, download source, environment, impact,
and minimal reproduction instructions. Remove live credentials and private
registry data; use synthetic examples where possible. Reports are reviewed by
the repository's security maintainers through GitHub's private advisory workflow.

The public issue-first contribution process does not apply to undisclosed
vulnerabilities. Keep investigation, patches, and review private until the
maintainers coordinate disclosure. Publicly link the fix and release after
disclosure where appropriate.

## Public security fixes and hardening

Use the [Security fix issue form](https://github.com/rambow-cloud/skopeo-win/issues/new?template=04-security-fix.yml)
for already disclosed vulnerabilities or non-sensitive hardening work. Link the
public advisory when available, establish applicability to this distribution,
and record the affected versions, remediation, verification, and release impact.
Do not treat a dependency scan finding as confirmed exploitability without
checking whether the affected code is used in the Windows build.

## Upstream coordination

The skopeo Project follows the [Security and Disclosure Information Policy](https://github.com/containers/common/blob/main/SECURITY.md) for the Containers Projects.

For vulnerabilities in shared upstream Skopeo code, follow that upstream policy
and coordinate with the Windows distribution maintainers privately when this
distribution is also affected. This repository does not speak for upstream's
security team.
