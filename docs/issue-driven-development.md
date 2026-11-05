# Issue-driven development

The issue is the record of a change's purpose, scope, acceptance criteria, and completion evidence. Pull requests implement that agreement. This process applies to `rambow-cloud/skopeo-win`; upstream contributions also follow upstream's own process.

## 1. Open the right issue

Use the [issue chooser](https://github.com/rambow-cloud/skopeo-win/issues/new/choose):

- **Bug report:** a reproducible failure, environment, expected behavior, and actual behavior.
- **Feature request:** a user need, proposed outcome, and observable acceptance criteria.
- **Implementation task:** a bounded work item covering build, CI, signing, selected feature ports, dependency updates, documentation, or release maintenance.
- **Security fix:** remediation of an already disclosed vulnerability or non-sensitive hardening, with affected versions, public references, verification, and release impact.

For new or undisclosed vulnerabilities, follow [SECURITY.md](../SECURITY.md) and use a private vulnerability report instead of a public issue. Plan, implement, and review the fix privately until coordinated disclosure; the public issue-first requirement does not apply to this work.

Search existing reports first. Keep credentials and private registry data out of issues. A bug reporter does not need to design a fix; the maintainer can add scope and acceptance criteria after reproducing the report.

## 2. Agree on scope before implementation

A maintainer confirms the objective, scope, non-goals, dependencies, and acceptance criteria in the issue. Split unrelated outcomes into separate issues. A contributor claims the work in a comment so others can avoid duplicate effort.

Use this planning comment when the original report needs more detail:

```markdown
## Implementation brief

Objective:

In scope:

Out of scope:

Acceptance criteria:
- [ ] ...

Plan:
1. ...

Validation:
- Criterion -> command, CI check, or manual verification

Dependencies / decisions needed:

Release impact / rollback:
```

## 3. Implement on an issue branch

Create a branch from the current `main`, named `issue/<number>-<short-description>`, for example `issue/42-verify-release-checksums`. Keep the change focused on the linked issue and use DCO-signed-off commits as described in [CONTRIBUTING.md](../CONTRIBUTING.md).

Record meaningful discoveries, blockers, or scope changes in the issue. Confirm scope changes before adding unrelated work. For large changes, use child issues or multiple PRs with `Refs #N` until the entire issue can be closed.

## 4. Verify against acceptance criteria

Record evidence for each criterion. Use checks appropriate to the change:

- Documentation: run `./hack/verify-no-chinese-docs.ps1` and check affected links and instructions.
- Build or dependency changes: run `./build-windows.ps1 -Architecture amd64,arm64` and preserve the CGO-disabled, architecture, OpenPGP, and PE checks.
- Command behavior: run focused tests with `CGO_ENABLED=0` and the `containers_image_openpgp exclude_graphdriver_btrfs` build tags, plus the relevant runtime scenario.
- CI or releases: link the affected workflow run and inspect its relevant steps and artifacts. A PR build does not verify the main-only release publication step.

Run each relevant verification once unless a failure or meaningful change justifies another run. State skipped or environment-blocked checks explicitly. Do not claim ARM64 runtime testing solely from cross-compilation.

## 5. Open and review the PR

Use the [PR template](../.github/pull_request_template.md). Include `Closes #N` only when the PR completes the issue, and `Refs #N` for partial work. Map acceptance criteria to evidence and describe compatibility and release impact. Review both the implementation and its verification before merging.

This is a contributor workflow supported by templates, not a branch protection rule or an automated issue-link check.

## 6. Close with delivery evidence

Merge after review and relevant checks pass. If the issue requires a published artifact or external action, use `Refs #N` and keep it open until that outcome is verified. Record the final PR, CI run, and release link when relevant before closing the issue.

Successful `main` push builds publish a new independent patch version under the [release policy](release-policy.md) when production inputs have changed since the latest published release. Automation and documentation maintenance runs checks without publishing. Existing published assets are not replaced. Follow-up work should have its own issue. Port selected features through scoped issues and PRs rather than bulk-merging upstream source releases.
