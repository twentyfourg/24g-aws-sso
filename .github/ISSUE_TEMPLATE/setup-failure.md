---
name: Setup failure
about: An uncaught error while running 24GAWSSOSetup.md
title: "[setup] Step <N> failed: <one-line summary>"
labels: ["setup-failure"]
---

<!--
Before you post: nothing below may contain an SSO access token, refresh token,
clientSecret, or the full contents of ~/.aws/config. See the redaction check at
the bottom. If you are unsure whether a value is sensitive, remove it.
-->

## Workflow version

<!-- The `Version:` line from the top of 24GAWSSOSetup.md. -->

## Step that failed

<!-- e.g. "Step 7: Enumerate accounts and roles" -->

## What happened

<!--
Two or three sentences. What was the agent trying to do, what did it expect, and
what happened instead.
-->

## Command and error output

<!--
The exact command, and the complete output including the error. Do not trim it
to the last line; the surrounding output is usually what identifies the cause.
-->

```
paste here
```

## Trace

<!--
The steps that ran before the failure, in order, and what each one did. A short
list is fine. Note any step that was skipped because Step 0 found it already
satisfied, and any question the agent asked along with the answer given — a
wrong branch is often chosen several steps before the error surfaces.
-->

1.
2.
3.

## Diagnostics

<!--
Output of the diagnostics command from the "Report an uncaught error" section of
24GAWSSOSetup.md. It reports counts and versions only, no account IDs, profile
names, or tokens.
-->

```
paste here
```

## Starting state

<!--
The state report Step 0 produced at the beginning of the run, before anything
was changed. This is what makes the failure reproducible.
-->

```
paste here
```

## Rollback status

<!-- Tick one. -->

- [ ] No rollback point existed yet (the run failed before Step 2)
- [ ] A rollback point exists and was left in place
- [ ] A rollback was performed and succeeded
- [ ] A rollback was attempted and failed — describe below

## Anything else

<!--
Whatever does not fit above. Useful things: whether this machine had a working
setup before, whether a colleague hit the same error, whether the error is
reproducible on a re-run or happened once.
-->

## Redaction check

- [ ] No SSO access token, refresh token, or `clientSecret` appears anywhere above
- [ ] No full dump of `~/.aws/config` or `~/.aws/credentials`
- [ ] No static AWS access key ID or secret access key
