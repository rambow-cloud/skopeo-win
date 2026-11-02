# Contributing to Skopeo

This repository maintains the Windows distribution at
[rambow-cloud/skopeo-win](https://github.com/rambow-cloud/skopeo-win).
Follow our [issue-driven development workflow](docs/issue-driven-development.md)
for changes here. The upstream contribution requirements below remain applicable
unless this document explicitly describes a distribution-specific process.

This project uses independent [release versioning](docs/release-policy.md).
Contributions focus on bug fixes, security updates, and selected feature ports.
Do not bulk-merge upstream Skopeo source releases. Dependency updates and focused
ports must preserve Windows compatibility, licensing, and the pure-Go build.

We'd love to have you join the community! Below summarizes the processes
that we follow.

## Topics

* [LLM ("AI") Policy](#llm-ai-policy)
* [Reporting Issues](#reporting-issues)
* [Submitting Pull Requests](#submitting-pull-requests)
* [Communications](#communications)
<!--
* [Becoming a Maintainer](#becoming-a-maintainer)
-->

## LLM ("AI") Policy

If your contribution is aided by LLMs or other AI tools, please read the [LLM Policy in the Podman project](https://github.com/containers/podman/blob/main/LLM_POLICY.md).
This project also follows this LLM policy, which includes comments, issues, PRs, and any other interactions with the team.

## Reporting Issues

Before reporting an issue, check our backlog of 
[open issues](https://github.com/rambow-cloud/skopeo-win/issues)
to see if someone else has already reported it. If so, feel free to add
your scenario, or additional information, to the discussion. Or simply 
"subscribe" to it to be notified when it is updated.

If you find a new issue with the project we'd love to hear about it! The most
important aspect of a bug report is that it includes enough information for
us to reproduce it. So, please include as much detail as possible and try
to remove the extra stuff that doesn't really relate to the issue itself.
The easier it is for us to reproduce it, the faster it'll be fixed!

Please don't include any private/sensitive information in your issue!

Use the [issue forms](https://github.com/rambow-cloud/skopeo-win/issues/new/choose)
for bug reports, feature requests, and implementation tasks. Maintainers refine
scope and acceptance criteria before implementation. Link related upstream
reports when the behavior also occurs outside this Windows distribution.
Use the Security fix form for already public security remediation or hardening.
For new or undisclosed vulnerabilities, follow [SECURITY.md](SECURITY.md) and
use the private reporting process instead of opening a public issue or PR.

## Submitting Pull Requests

No Pull Request (PR) is too small! Typos, additional comments in the code,
new testcases, bug fixes, new features, more documentation, ... it's all 
welcome!

For this Windows distribution, open or link an issue before implementation.
Undisclosed security fixes follow the private process in [SECURITY.md](SECURITY.md).
Agree on scope and acceptance criteria there, then create an
`issue/<number>-<short-description>` branch. Use the PR template to link the
issue and record validation evidence. Use `Closes #N` for completed work and
`Refs #N` when the PR only completes part of the issue.

PRs for new features should include some background on what use cases the
new code is trying to address. When possible and when it makes sense, try to break-up
larger PRs into smaller ones - it's easier to review smaller
code changes. But only if those smaller ones make sense as stand-alone PRs.

PRs should include well documented changes and relevant validation evidence.
Behavior changes should include focused regression tests when practical.
Documentation-only or template changes do not require unrelated runtime tests.
Update user-facing documentation when behavior changes, and keep repository
documentation and templates in English.

Squash your commits into logical pieces of work that might want to be reviewed
separate from the rest of the PRs. Ideally, each commit should implement a single
idea, and the PR branch should pass the tests at every commit.  GitHub makes it easy
to review the cumulative effect of many commits; so, when in doubt, use smaller commits.

PRs that fix issues should include a reference like `Closes #XXXX` in the
commit message so that github will automatically close the referenced issue
when the PR is merged.

<!--
All PRs require at least two LGTMs (Looks Good To Me) from maintainers.
-->

### Sign your PRs

The sign-off is a line at the end of the explanation for the patch. Your
signature certifies that you wrote the patch or otherwise have the right to pass
it on as an open-source patch. The rules are simple: if you can certify
the below (from [developercertificate.org](http://developercertificate.org/)):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
660 York Street, Suite 102,
San Francisco, CA 94110 USA

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

Then you just add a line to every git commit message:

    Signed-off-by: Joe Smith <joe.smith@email.com>

Use your real name (sorry, no pseudonyms or anonymous contributions.)

If you set your `user.name` and `user.email` git configs, you can sign your
commit automatically with `git commit -s`.

### Dependencies management

Dependencies are managed via [standard go modules](https://golang.org/ref/mod).

In order to add a new dependency to this project:

- use `go get -d path/to/dep@version` to add a new line to `go.mod`
- run `make vendor`

In order to update an existing dependency:

- use `go get -d -u path/to/dep@version` to update the relevant dependency line in `go.mod`
- run `make vendor`

When new PRs for [containers/container-libs](https://github.com/containers/container-libs) break `skopeo` (i.e. `containers/container-libs` tests fail in `make test-skopeo`):

- create out a new branch in your `skopeo` checkout and switch to it
- find out the version of `containers/container-libs` you want to use and note its commit ID. You might also want to use a fork of `containers/container-libs`, in that case note its repo
- use `go get -d github.com/$REPO/container-libs/image/v5@$COMMIT_ID` to download the right version. The command will fetch the dependency and then fail because of a conflict in `go.mod`, this is expected. Note the pseudo-version (eg. `v5.13.1-0.20210707123201-50afbf0a326`)
- use `go mod edit -replace=go.podman.io/image/v5=github.com/$REPO/container-libs/image/v5@$PSEUDO_VERSION` to add a replacement line to `go.mod` (e.g. `replace go.podman.io/image/v5 => github.com/moio/container-libs/image/v5 v5.13.1-0.20210707123201-50afbf0a3262`)
- run `make vendor`
- make any other necessary changes in the skopeo repo (e.g. add other dependencies now required by `containers/container-libs`, or update skopeo for changed `containers/container-libs` API)
- optionally add new integration tests to the skopeo repo
- submit the resulting branch as a skopeo PR, marked “DO NOT MERGE”
- iterate until tests pass and the PR is reviewed
- then the original `containers/container-libs` PR can be merged, disregarding its `make test-skopeo` failure
- as soon as possible after that, in the skopeo PR, use `go mod edit -dropreplace=go.podman.io/image/v5` to remove the `replace` line in `go.mod`
- run `make vendor`
- update the skopeo PR with the result, drop the “DO NOT MERGE” marking
- after tests complete successfully again, merge the skopeo PR

## Communications

For general questions, or discussions, please use the
[#podman](https://app.slack.com/client/T08PSQ7BQ/C08MXJLCFCN) channel on the [CNCF
Slack](https://cloud-native.slack.com).

For development related discussions, please use the
[#podman-dev](https://app.slack.com/client/T08PSQ7BQ/C08NTKCDC1W) channel on the CNCF
Slack.

For discussions around issues/bugs and features, you can use the github
[issues](https://github.com/rambow-cloud/skopeo-win/issues)
and
[PRs](https://github.com/rambow-cloud/skopeo-win/pulls)
tracking system.

<!--
## Becoming a Maintainer

To become a maintainer you must first be nominated by an existing maintainer.
If a majority (>50%) of maintainers agree then the proposal is adopted and
you will be added to the list.

Removing a maintainer requires at least 75% of the remaining maintainers
approval, or if the person requests to be removed then it is automatic.
Normally, a maintainer will only be removed if they are considered to be
inactive for a long period of time or are viewed as disruptive to the community.

The current list of maintainers can be found in the 
[MAINTAINERS](MAINTAINERS) file.
-->
