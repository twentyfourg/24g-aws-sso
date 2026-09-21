# 24g-aws-sso

Sets up a developer's machine for 24G AWS access through IAM Identity Center. Hand the prompt below to your AI coding tool and it installs the AWS CLI and `jq`, configures `~/.aws/config`, signs you in through the browser, and generates a named profile for every account and role you have been granted.

You sign in once in the browser. Everything else the agent does for you.

## Usage

### Just 24G AWS SSO

Paste this into your AI coding tool:

```
Set up 24G AWS SSO by following instructions:
https://raw.githubusercontent.com/twentyfourg/24g-aws-sso/main/24GAWSSOSetup.md
```

The agent inspects your machine first, tells you what is already in place and what is missing, and asks how you want to proceed before it changes anything.

It is safe to run more than once. On a machine that is already configured it reports that there is nothing to do, and after you are granted access to a new account or role, re-running picks up the new profiles without disturbing the existing ones.

## What you end up with

A `[sso-session 24g]` block in `~/.aws/config` that holds the login details once, and one profile per account and role that points at it:

```ini
[sso-session 24g]
sso_start_url = https://24glogin.awsapps.com/start
sso_region = us-east-1
sso_registration_scopes = sso:account:access

[profile cadillac-24g-AdministratorAccess]
# managed-by: 24g-sso-setup
sso_session = 24g
sso_account_id = 219323925250
sso_role_name = AdministratorAccess
region = us-east-1
output = json
```

Then `aws s3 ls --profile cadillac-24g-AdministratorAccess` works, as does any AWS SDK that reads the shared config file.

Profiles are named `<account>-<role>` for roles that exist in several accounts, such as `AdministratorAccess` and `ViewOnlyAccess`, and just `<role>` for project-specific permission sets like `2772-1-lxp-base-microsite`. The agent shows you the full list and its proposed names before writing anything, and you can rename any of them or take a subset.

Sessions last about 12 hours. Renew with:

```bash
aws sso login --sso-session 24g
```

## Requirements

- macOS, Linux, or Windows
- A 24G Google account with access assigned in IAM Identity Center
- AWS CLI v2 and `jq` — the setup installs them if they are missing

AWS CLI v1 will not work; the `sso-session` configuration style needs v2.

> The macOS and Linux commands are tested. The Windows and PowerShell paths are written but have not been run on a real Windows machine yet. If you are the first to try it, expect rough edges and please [open an issue](#if-something-goes-wrong).

## Safety

The setup never asks for access keys, never writes `~/.aws/credentials`, and preserves any profiles or comments you already have in `~/.aws/config`.

Before it changes anything it takes a rollback point — a timestamped copy of the files it is about to touch, under `~/.aws/.24g-setup-backups/`. If a run goes wrong, or you change your mind partway through, the agent can restore your files exactly as they were. The backups are deleted only after you confirm the result is good.

## If something goes wrong

The setup file has a troubleshooting table for every step. If you hit an error it does not cover, the agent will collect a diagnostic summary and offer to open an issue using [the setup failure template](.github/ISSUE_TEMPLATE/setup-failure.md).

Do not paste an SSO access token into an issue. The diagnostics the agent collects report versions and counts only, with no tokens or credentials in them.

## Repository contents

| File                                          | What it is                                                                        |
| --------------------------------------------- | --------------------------------------------------------------------------------- |
| `24GAWSSOSetup.md`                            | The setup workflow. This is the file you hand to your AI tool                      |
| `.github/ISSUE_TEMPLATE/setup-failure.md`     | Issue template for a setup error with no documented resolution                     |
| `SPEC.md`                                     | Design notes for a possible `aws-24g` CLI that would replace most of the workflow. Not built |
