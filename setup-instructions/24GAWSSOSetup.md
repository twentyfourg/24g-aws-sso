# Set Up Local AWS Configuration and Credentials for 24G

<!-- semantic-release-version-start -->
Version: 1.0.0
Updated: 2026-09-21
<!-- semantic-release-version-end -->
Repository: <https://github.com/twentyfourg/24g-aws-sso>

You MUST report the version above to the user at the start of a run, and you MUST include it in any issue you file.

## Overview

This is a set up file for an AI agent to use to set up a developer's local system to work with the 24G AWS IAM Identity Center instance. Once complete, developers can sign in to AWS through the Identity Center browser flow to obtain credentials, and switch between [named profiles](https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-files.html#cli-configure-files-using-profiles) that map to the Identity Center roles they have been granted.

The workflow includes:

- Detecting the user's operating system
- Detecting the current configuration state of the user's system and reporting what is missing
- Installing the AWS CLI v2 and `jq` if they are missing
- Writing a `[default]` section and a `[sso-session 24g]` section into `~/.aws/config`
- Signing the user in through the browser with `aws sso login --sso-session 24g`
- Enumerating every account and role the user can access, and generating a named profile for each one the user selects
- Verifying the result

This file is **re-runnable**. It may be handed to an agent on a completely fresh machine, on a machine that is already fully configured, or anywhere in between. Step 0 establishes the current state and decides which steps still need to run.

## Secret handling

This is the single authority on secrets in this workflow. It overrides anything elsewhere in this file that appears to permit otherwise, and it applies to every step, not only the ones that read credentials.

### What counts as a secret

- The **SSO access token** and **refresh token** from `~/.aws/sso/cache/*.json`
- The **`clientId`** and **`clientSecret`** from those same files
- Any **AWS access key ID, secret access key, or session token**, including ones already in `~/.aws/credentials`
- The **contents of any file under `~/.aws/sso/cache/`**, which is why you read specific fields with `jq` rather than printing the file

Account IDs, account names, role names, profile names, and ARNs are **not** secrets. They are internal but shareable, and they are usually what makes a problem diagnosable. Do not redact them reflexively.

### The rule

You MUST NOT cause a secret to appear in any of the following:

- A message to the user, including a summary, a status update, a code block, or a quoted command
- The chat transcript in any form, which includes the output of a command you chose to run
- A GitHub issue, issue comment, pull request, or commit message
- Any file written to disk, including notes, logs, temp files, and the filled-in issue body

Concretely, that means:

- Read the token into a shell variable and reference it only as `"$AWS_SSO_TOKEN"` / `$AwsSsoToken`. Never substitute the literal value into a command, and never `echo`, `cat`, or `printf` it.
- Never print a whole cache file. When you need to inspect the cache, select non-secret fields only: `jq -r '.startUrl // "-", .expiresAt' ~/.aws/sso/cache/*.json`.
- Before you send a message or file an issue, re-read what you are about to send and check it for a token. If one is present, replace it with `<redacted>` and keep going.

### The trap to watch for

Step 7 passes the access token as a command-line argument, because `aws sso list-accounts` offers no other way to supply it. A failure in that step can therefore echo the entire failing command — token included — into the error output. That output is exactly what you would otherwise paste into a summary or an issue.

So when a command that carries the token fails, redact before you report. Say that you redacted something and why; do not quietly drop the output, because the rest of it is what diagnoses the failure.

The same argument is briefly visible to other processes on the machine through the process table. That is inherent to the AWS CLI and cannot be avoided here. Mention it only if the user asks.

### If a secret is exposed anyway

Do not paper over it. Tell the user plainly what leaked and where, then have them invalidate the token:

```bash
aws sso logout
```

That clears the local cache and calls the Identity Center Logout API to invalidate the server-side sign-in session, so the leaked token can no longer list accounts or mint new credentials. Be accurate about the limit: temporary role credentials that were already issued from a permission set keep working until their configured duration runs out, so logging out is not retroactive.

Then have the user sign in again with Step 5. If the secret reached a GitHub issue, editing the issue is not sufficient on its own — the value must be treated as compromised and rotated regardless, because edit history and notification emails retain the original text.

## 24G Identity Center constants

These values are fixed for 24G. Do not ask the user for them and do not substitute other values.

| Name                      | Value                                |
| ------------------------- | ------------------------------------ |
| `sso_session` name        | `24g`                                |
| `sso_start_url`           | `https://24glogin.awsapps.com/start` |
| `sso_region`              | `us-east-1`                          |
| `sso_registration_scopes` | `sso:account:access`                 |
| Default client region     | `us-east-1`                          |
| Managed profile marker    | `# managed-by: 24g-sso-setup`        |

## Before you start

Gather the following inputs and confirm prerequisites **before** running any step. Ask for everything you need in a single message, then proceed autonomously.

### Required inputs

- **operating_system** (optional): macOS, Linux, or Windows. You MUST detect this automatically in Step 1 before asking the user.
- **default_region** (optional): The Region to write into `[default]` and into each generated profile. Defaults to `us-east-1`. Only ask if the user brings it up or if `[default]` already has a different Region (see Step 4).
- **profile_selection** (required, gathered in Step 8): which account/role pairs become profiles, and whether the user wants custom profile names.

### Input constraints

- You MUST NOT ask the user for AWS credentials, access keys, or secret keys. Authentication happens entirely through the `aws sso login` browser flow.
- You MUST NOT write anything to `~/.aws/credentials`. IAM Identity Center does not use that file, and it may contain unrelated static credentials the user depends on.
- You MUST follow [Secret handling](#secret-handling). No secret reaches the chat transcript, a GitHub issue, or a file on disk.
- You MUST inform the user that the resulting credentials are valid for a limited session (typically 12 hours) and that re-running `aws sso login --sso-session 24g` renews them without reconfiguring anything.
- You MUST create the rollback point described in [Rollback and cleanup](#rollback-and-cleanup) before the first write of a run, and you MUST preserve every section of `~/.aws/config` that you did not generate.
- You MUST NOT delete the rollback point until the user has confirmed the result is good.

### Prerequisites

The following must be present on the system. Steps 2 and 3 install them if they are missing.

| Tool                                              | Why                                                       |
| ------------------------------------------------- | --------------------------------------------------------- |
| AWS CLI v2                                        | `aws sso login`, `aws sso list-accounts`, `aws configure` |
| `jq`                                              | Parsing the SSO token cache and the account/role listings |
| `curl` (macOS/Linux) or PowerShell 5.1+ (Windows) | Downloading installers                                    |

You MUST verify internet connectivity to `https://awscli.amazonaws.com` and `https://24glogin.awsapps.com` before installing anything. You MUST tell the user about any missing tool with a clear message, ask whether to install it, and respect a decision to abort at any point.

## How to run this file

- Run Step 0 first, every time, without exception. Its output determines which of the remaining steps are needed.
- After Step 0, report the state you found and ask the user how to proceed **before** changing anything on their system. Do not silently repair state.
- Explain what each step is doing, why, and which command you are about to run.
- One step requires the human user to act: **Step 5 (`aws sso login`)** opens a browser for sign-in. Pause and let them complete it.
- Every step lists a success criterion. If a step fails, find the matching table in [Troubleshooting](#troubleshooting), apply the resolution, and resume at that step. For errors not covered there, report the full error output and do not proceed.
- Take the rollback point before Step 2, which is the first step that modifies a file. See [Rollback and cleanup](#rollback-and-cleanup).
- If an error has no matching troubleshooting row, follow [Report an uncaught error](#report-an-uncaught-error) after you have reported it and offered a rollback.

## Rollback and cleanup

Any run that starts from something other than a completely fresh machine is editing files the user already depends on. Before the first write, copy those files somewhere known, so the run can be undone if it goes wrong or the user changes their mind partway through.

### Create the rollback point

Do this once per run, before Step 2. The backup directory is named for the moment the run started, so repeated runs do not overwrite each other's copies.

**macOS / Linux:**

```bash
RUN_ID="$(date +%Y%m%d-%H%M%S)"
BACKUP_DIR="$HOME/.aws/.24g-setup-backups/$RUN_ID"
mkdir -p "$BACKUP_DIR"
: > "$BACKUP_DIR/manifest.tsv"

save() {
  if [ -f "$1" ]; then
    cp -p "$1" "$BACKUP_DIR/$2"
    printf '%s\t%s\n' "$2" "$1" >> "$BACKUP_DIR/manifest.tsv"
  else
    printf 'ABSENT\t%s\n' "$1" >> "$BACKUP_DIR/manifest.tsv"
  fi
}

SHELL_RC="$HOME/.bashrc"
[ "$(basename "$SHELL")" = "zsh" ] && SHELL_RC="$HOME/.zshrc"

save "$HOME/.aws/config" config
save "$SHELL_RC" shell-rc

echo "Rollback point: $BACKUP_DIR"
cat "$BACKUP_DIR/manifest.tsv"
```

**Windows (PowerShell):**

```powershell
$RunId = Get-Date -Format 'yyyyMMdd-HHmmss'
$BackupDir = "$env:USERPROFILE\.aws\.24g-setup-backups\$RunId"
New-Item -ItemType Directory -Force -Path $BackupDir | Out-Null
$Manifest = "$BackupDir\manifest.tsv"
New-Item -ItemType File -Force -Path $Manifest | Out-Null

$Src = "$env:USERPROFILE\.aws\config"
if (Test-Path $Src) {
  Copy-Item $Src "$BackupDir\config"
  "config`t$Src" | Add-Content $Manifest
} else {
  "ABSENT`t$Src" | Add-Content $Manifest
}

"Rollback point: $BackupDir"
Get-Content $Manifest
```

The manifest records a line per file. A file that existed is saved under a short name; a file that did **not** exist is recorded as `ABSENT`, which is how a rollback knows to remove a file this run created rather than leave an empty one behind.

Tell the user the rollback path as soon as it exists, and do not delete it mid-run.

### What the rollback point does and does not cover

Covered: `~/.aws/config`, and on macOS and Linux the shell rc file that Step 2 appends a `PATH` line to.

Not covered, and you MUST say so when you offer a rollback:

- **Installing the AWS CLI and `jq`** (Steps 2 and 3). Rolling back leaves them installed. That is almost always what the user wants; if not, they can remove them separately.
- **The SSO session** written to `~/.aws/sso/cache/` by Step 5. Harmless to leave. `aws sso logout` clears it.
- **Anything the user did in the browser.** Authorizing the device is an account-side action.
- `~/.aws/credentials` is never written by this file, so there is nothing to restore.
- **The `~/.aws` directory itself**, on a machine that did not have one. The rollback empties it but cannot remove it, because the rollback point lives inside it. An empty `~/.aws` is harmless — the AWS CLI creates it on demand — but do not describe the rollback as leaving no trace.

### Roll back

Offer a rollback when the user aborts partway through, when a step fails and the troubleshooting table has no resolution, or when the user asks. Never roll back on your own initiative: show the manifest, say plainly what will be restored and what will not, and wait for a yes.

**macOS / Linux:**

```bash
BACKUP_DIR="$(find "$HOME/.aws/.24g-setup-backups" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort | tail -1)"
if [ -z "$BACKUP_DIR" ]; then
  echo "No rollback point found."
else
  echo "Restoring from $BACKUP_DIR"
  while IFS="$(printf '\t')" read -r saved original; do
    if [ "$saved" = "ABSENT" ]; then
      [ -f "$original" ] && mv "$original" "$BACKUP_DIR/removed-$(basename "$original")"
      echo "removed (a copy is kept in the rollback directory): $original"
    else
      cp -p "$BACKUP_DIR/$saved" "$original"
      echo "restored: $original"
    fi
  done < "$BACKUP_DIR/manifest.tsv"
fi
```

**Windows (PowerShell):**

```powershell
$BackupDir = Get-ChildItem "$env:USERPROFILE\.aws\.24g-setup-backups" -Directory -ErrorAction SilentlyContinue |
  Sort-Object Name | Select-Object -Last 1
if (-not $BackupDir) { "No rollback point found." } else {
  "Restoring from $($BackupDir.FullName)"
  Get-Content "$($BackupDir.FullName)\manifest.tsv" | ForEach-Object {
    $saved, $original = $_ -split "`t", 2
    if ($saved -eq 'ABSENT') {
      if (Test-Path $original) {
        Move-Item $original "$($BackupDir.FullName)\removed-$(Split-Path $original -Leaf)" -Force
      }
      "removed (a copy is kept in the rollback directory): $original"
    } else {
      Copy-Item "$($BackupDir.FullName)\$saved" $original -Force
      "restored: $original"
    }
  }
}
```

A rollback never destroys anything. Files this run created are moved into the rollback directory as `removed-<name>` rather than deleted, so a rollback can itself be undone.

The commands above pick the newest rollback directory. If the user has run this file several times, list the directories and confirm which run they want to undo before restoring.

### Clean up

Backups are deleted in Step 11, only after the user confirms the result is good. Never as part of an earlier step.

---

## Step 0: Determine the current state of the system

Run the checks below and build a state report. Every check is read-only. A failed check is information, not an error, so keep going after a non-zero exit and record the result.

**macOS / Linux:**

```bash
uname -s
aws --version
jq --version
cat ~/.aws/config 2>/dev/null
ls -la ~/.aws/sso/cache/ 2>/dev/null
```

**Windows (PowerShell):**

```powershell
$env:OS
aws --version
jq --version
Get-Content "$env:USERPROFILE\.aws\config" -ErrorAction SilentlyContinue
Get-ChildItem "$env:USERPROFILE\.aws\sso\cache\" -ErrorAction SilentlyContinue
```

From that output, resolve each of the following facts:

| Fact                           | How to determine it                                                                                                                                                        | If missing, go to |
| ------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------- |
| AWS CLI installed              | `aws --version` exits 0 and reports `aws-cli/2.x`                                                                                                                          | Step 2            |
| `jq` installed                 | `jq --version` exits 0                                                                                                                                                     | Step 3            |
| `~/.aws/config` exists         | The file is readable                                                                                                                                                       | Step 4            |
| `[default]` configured         | A `[default]` section exists with a `region`                                                                                                                               | Step 4            |
| `[sso-session 24g]` configured | A `[sso-session 24g]` section exists with `sso_start_url = https://24glogin.awsapps.com/start`, `sso_region = us-east-1`, `sso_registration_scopes = sso:account:access`   | Step 5            |
| Active SSO session             | A cache file in `~/.aws/sso/cache/` has `startUrl` equal to the 24G start URL, a non-null `accessToken`, and an `expiresAt` in the future (see Step 6 for the exact check) | Step 5            |
| Managed profiles present       | Count the `[profile ...]` sections in `~/.aws/config` whose first body line is `# managed-by: 24g-sso-setup`                                                               | Step 7            |

Record two extra things, because they change how later steps behave:

- **AWS CLI v1 instead of v2.** If `aws --version` reports `aws-cli/1.x`, `sso-session` is not supported. Treat this as "AWS CLI missing" and go to Step 2 to install v2 alongside or in place of v1.
- **Conflicting existing configuration.** Note any pre-existing `[sso-session]` section with a different name, any `[profile ...]` section using the legacy inline `sso_start_url` style, and any profile whose name collides with one this file would generate. These are never overwritten silently; they are raised with the user in the step that touches them.

**Success:** You can state, for each row of the table above, whether it is satisfied.

### Report and ask before changing anything

Present the findings to the user as a short checklist, for example:

```
Current state of your system:
  [x] AWS CLI v2 (2.32.4)
  [ ] jq            - not installed
  [x] ~/.aws/config exists
  [x] [default] section (region = us-east-1)
  [ ] [sso-session 24g] section  - missing
  [ ] Active 24G SSO session     - none found
  [x] 3 existing profiles, none managed by this setup

To finish setup I need to: install jq, add the [sso-session 24g] section,
sign you in through the browser, and generate profiles for your accounts.
I will take a rollback point first and leave your 3 existing profiles alone.
```

Then ask how to proceed, offering at minimum: run everything that is missing, run only specific items, or stop. Wait for an answer. If the state report shows nothing missing, say so and offer to re-run Step 7 anyway to pick up newly granted accounts or roles.

If this step fails, see [Troubleshooting: Step 0](#troubleshooting-step-0-determine-the-current-state).

---

## Step 1: Determine the operating system

Check the session context first. If the OS is not already known, detect it:

- Unix-like shell: `uname -s` (`Darwin` = macOS, `Linux` = Linux)
- PowerShell: `$env:OS` (`Windows_NT` = Windows)

**Success:** OS identified as macOS, Linux, or Windows. Use the matching variant of every command that follows.

If this step fails, see [Troubleshooting: Step 1](#troubleshooting-step-1-determine-the-operating-system).

---

## Step 2: Install or verify the AWS CLI v2

Skip if Step 0 found AWS CLI v2. The `sso-session` configuration style used here requires v2; v1 will not work.

This step appends a `PATH` line to the user's shell rc file, so it is the first step that modifies an existing file. Create the [rollback point](#create-the-rollback-point) before running it, even if you are skipping the install itself and only fixing `PATH`.

**macOS / Linux:**

```bash
curl -fsSL 'https://awscli.amazonaws.com/v2/install.sh' | bash
```

Make `aws` available in the current session and in future sessions:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

```bash
SHELL_RC="$HOME/.bashrc"
if [ "$(basename "$SHELL")" = "zsh" ]; then
  SHELL_RC="$HOME/.zshrc"
fi
grep -qF 'export PATH="$HOME/.local/bin:$PATH"' "$SHELL_RC" 2>/dev/null \
  || echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$SHELL_RC"
```

**Windows (PowerShell):**

```powershell
irm 'https://awscli.amazonaws.com/v2/install.ps1' | iex
```

If that installer is unavailable, fall back to the MSI:

```powershell
msiexec.exe /i https://awscli.amazonaws.com/AWSCLIV2.msi /qn
```

> **Note on the piped installer:** this runs AWS's official install script directly. A security-conscious user can instead download the script, inspect it, and run it, or follow the manual steps on the [AWS CLI install page](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html). Offer this alternative if the user raises it.

**Success:** `aws --version` exits 0 and reports version 2.x.

If this step fails, see [Troubleshooting: Step 2](#troubleshooting-step-2-install-the-aws-cli).

---

## Step 3: Install or verify jq

Skip if Step 0 found `jq`.

**macOS:**

```bash
brew install jq
```

If Homebrew is not installed, do not install Homebrew for this. Download the static binary instead:

```bash
mkdir -p "$HOME/.local/bin"
curl -fsSL -o "$HOME/.local/bin/jq" \
  "https://github.com/jqlang/jq/releases/latest/download/jq-macos-$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')"
chmod +x "$HOME/.local/bin/jq"
export PATH="$HOME/.local/bin:$PATH"
```

**Linux:** use the distribution package manager.

```bash
sudo apt-get update && sudo apt-get install -y jq    # Debian / Ubuntu
sudo dnf install -y jq                                # Fedora / RHEL / Amazon Linux
```

If the user has no `sudo`, use the static binary approach above with `jq-linux-amd64` or `jq-linux-arm64`.

**Windows (PowerShell):**

```powershell
winget install --exact --id jqlang.jq --accept-source-agreements --accept-package-agreements
```

**Success:** `jq --version` exits 0.

If this step fails, see [Troubleshooting: Step 3](#troubleshooting-step-3-install-jq).

---

## Step 4: Configure the `[default]` section

Confirm the [rollback point](#create-the-rollback-point) exists before writing. It should already, from before Step 2; if the run skipped straight to this step because the CLI and `jq` were already installed, create it now. Creating the directory is safe either way:

```bash
mkdir -p ~/.aws
```

The target `[default]` section is:

```ini
[default]
region = us-east-1
```

Use `aws configure set`, not a hand-edit, so the CLI owns the formatting and existing keys are preserved:

```bash
aws configure set region us-east-1
```

Rules:

- If `[default]` already exists with a **different** `region`, do not change it. Tell the user what it is set to and ask whether to keep it or switch to `us-east-1`. Whatever they choose becomes `default_region` for the rest of this run.
- Leave every other key in `[default]` alone.

**Success:** `aws configure get region` returns the expected Region.

If this step fails, see [Troubleshooting: Step 4](#troubleshooting-step-4-configure-the-default-section).

---

## Step 5: Configure the `[sso-session 24g]` section and sign in

The target section is:

```ini
[sso-session 24g]
sso_start_url = https://24glogin.awsapps.com/start
sso_region = us-east-1
sso_registration_scopes = sso:account:access
```

There is no non-interactive `aws configure` command that writes this section. `aws configure sso-session` is an interactive wizard, and `aws configure set sso-session.24g.sso_start_url ...` does **not** work — the CLI does not recognize `sso-session` as a section type in a dotted path and silently writes a bogus `sso-session` key into `[default]` instead. Append the section to the file directly, and only if `[sso-session 24g]` is absent:

**macOS / Linux:**

```bash
if ! grep -q '^\[sso-session 24g\]' ~/.aws/config 2>/dev/null; then
  cat >> ~/.aws/config <<'EOF'

[sso-session 24g]
sso_start_url = https://24glogin.awsapps.com/start
sso_region = us-east-1
sso_registration_scopes = sso:account:access
EOF
fi
```

**Windows (PowerShell):**

```powershell
$cfg = "$env:USERPROFILE\.aws\config"
if (-not (Select-String -Path $cfg -Pattern '^\[sso-session 24g\]' -Quiet -ErrorAction SilentlyContinue)) {
  @'

[sso-session 24g]
sso_start_url = https://24glogin.awsapps.com/start
sso_region = us-east-1
sso_registration_scopes = sso:account:access
'@ | Add-Content -Path $cfg
}
```

If a `[sso-session 24g]` section exists with different values, show the user the difference and ask before correcting it.

Then sign in:

```bash
aws sso login --sso-session 24g
```

A browser window opens. **The human user completes this.** Wait for the command to exit; do not poll, retry, or interrupt it. If the user wants to cancel, let them.

On success the CLI writes a token cache file under `~/.aws/sso/cache/` (`%USERPROFILE%\.aws\sso\cache\` on Windows).

**Success:** `aws sso login --sso-session 24g` exits 0 and prints a confirmation that the SSO session is valid.

If this step fails, see [Troubleshooting: Step 5](#troubleshooting-step-5-configure-the-sso-session-and-sign-in).

---

## Step 6: Read the access token from the SSO cache

The account and role listing APIs take the SSO access token directly. Pull it out of the cache.

The cache directory holds two kinds of file: client registration files (which have `clientId` and `clientSecret` but no `accessToken`) and token files (which have `accessToken`, `startUrl`, and `expiresAt`). Select the token file that belongs to the 24G start URL and has not expired. When more than one qualifies, take the one expiring furthest in the future, which is the most recently issued.

**macOS / Linux:**

```bash
AWS_SSO_TOKEN="$(jq -rs --arg url 'https://24glogin.awsapps.com/start' '
  [ .[]
    | select(.accessToken != null and .startUrl == $url)
    | select((.expiresAt | sub("\\.[0-9]+"; "") | sub("UTC$"; "Z") | fromdateiso8601) > now)
  ]
  | sort_by(.expiresAt) | last | .accessToken // empty
' ~/.aws/sso/cache/*.json)"
[ -n "$AWS_SSO_TOKEN" ] && echo "Found a valid 24G SSO token." || echo "No valid token found."
```

The two `sub` calls are load-bearing. `expiresAt` appears in three shapes across cache files written by different CLI versions — `2026-11-29T12:02:06Z`, `2025-08-28T14:51:21.922Z`, and the legacy `2021-01-01T00:00:00UTC` — and jq's `fromdateiso8601` accepts only the first. Stripping fractional seconds and normalizing the `UTC` suffix covers all three.

**Windows (PowerShell):**

```powershell
$AwsSsoToken = Get-ChildItem "$env:USERPROFILE\.aws\sso\cache\*.json" |
  ForEach-Object { Get-Content $_.FullName -Raw | ConvertFrom-Json } |
  Where-Object { $_.accessToken -and $_.startUrl -eq 'https://24glogin.awsapps.com/start' -and
                 [datetimeoffset]($_.expiresAt -replace 'UTC$','Z') -gt [datetimeoffset]::UtcNow } |
  Sort-Object expiresAt | Select-Object -Last 1 -ExpandProperty accessToken
if ($AwsSsoToken) { "Found a valid 24G SSO token." } else { "No valid token found." }
```

Use `[datetimeoffset]`, not `[datetime]`. Casting an ISO string ending in `Z` to `[datetime]` silently converts it to local time, which makes the comparison wrong by the machine's UTC offset.

What you just read into that variable is a secret. [Secret handling](#secret-handling) governs it in full; the two rules that matter most here are that you reference it only as `"$AWS_SSO_TOKEN"` / `$AwsSsoToken` and never substitute the literal value anywhere, and that you never print a cache file.

One more rule specific to this step:

- Run Step 6 and Step 7 in a **single shell invocation**. Many agent shell tools start a fresh process per command, so a non-exported variable set in one call is empty in the next. An empty `--access-token` fails with `UnauthorizedException: Session token not found or invalid`, which looks like an expired session but is not. Either put both steps in one command block, or `export AWS_SSO_TOKEN` and confirm the tool actually persists the environment.

**Success:** The variable is non-empty.

If this step fails, see [Troubleshooting: Step 6](#troubleshooting-step-6-read-the-access-token).

---

## Step 7: Enumerate accounts and roles

List every account the user can reach:

```bash
aws sso list-accounts --access-token "$AWS_SSO_TOKEN" --region us-east-1 --no-cli-pager
```

Then list the roles in each account. Use the account IDs from the previous response:

```bash
aws sso list-account-roles --access-token "$AWS_SSO_TOKEN" --region us-east-1 --account-id <accountId> --no-cli-pager
```

Build the full matrix in one pass and keep it in a shell variable or a temp file. Note that this makes one API call per account, so a user with twenty accounts makes twenty-one calls; that is expected, not a loop bug.

```bash
ACCOUNTS_JSON="$(aws sso list-accounts --access-token "$AWS_SSO_TOKEN" --region us-east-1 --no-cli-pager)"
MATRIX_JSON="$(echo "$ACCOUNTS_JSON" | jq -c '.accountList[]' | while read -r acct; do
  id="$(echo "$acct" | jq -r '.accountId')"
  name="$(echo "$acct" | jq -r '.accountName')"
  aws sso list-account-roles --access-token "$AWS_SSO_TOKEN" --region us-east-1 \
    --account-id "$id" --no-cli-pager \
    | jq -c --arg name "$name" '[.roleList[] | {accountId, accountName: $name, roleName}]'
done | jq -sc 'add')"
```

Both APIs paginate; the AWS CLI v2 follows the pages automatically, so no `--next-token` handling is needed.

**Success:** You have a list of `{accountId, accountName, roleName}` entries covering every account and role.

If this step fails, see [Troubleshooting: Step 7](#troubleshooting-step-7-enumerate-accounts-and-roles).

---

## Step 8: Propose profile names and confirm the selection with the user

### Naming rules

A profile name is derived from the account name and the role name.

**System roles** are the AWS-managed-style permission sets that appear in many accounts: `AdministratorAccess`, `ViewOnlyAccess`, `ReadOnlyAccess`, `PowerUserAccess`, and anything else matching PascalCase with a leading capital and no separators — regex `^[A-Z][A-Za-z0-9]*$`. Because the same system role exists in several accounts, the role name alone would collide. Prefix these with the **account name**:

```
<accountName>-<roleName>      e.g. example-prod-AdministratorAccess
```

**Project roles** are account-specific permission sets, typically containing hyphens, underscores, or digits and not PascalCase, such as `app-deploy`. These are already unique, so the role name is used on its own:

```
<roleName>                    e.g. app-deploy
```

Some project roles are short or ambiguous on their own (`k6`, `route53-read`). These are still unique, so the rule stands, but offer the account-prefixed form as an alternative when you present the list.

**Collisions.** After applying both rules, check the generated names for duplicates, including against every name in `aws configure list-profiles`. That command covers both `~/.aws/config` and `~/.aws/credentials`, and the two files share one profile namespace, so a name already used in `credentials` is a collision even though this file never writes there. If a project role name turns out to appear in more than one account, prefix every instance of it with the account name as well. Use the **account name**, never the account ID, in profile names. If a name still collides with a pre-existing, hand-written profile, do not overwrite it: raise it with the user and ask for a different name.

### Present and confirm

Show the user the full matrix grouped by account, with the proposed profile name for each role. For example:

```
example-prod (111122223333)
  AdministratorAccess        -> example-prod-AdministratorAccess
  ViewOnlyAccess             -> example-prod-ViewOnlyAccess

example-dev (444455556666)
  AdministratorAccess        -> example-dev-AdministratorAccess
  ViewOnlyAccess             -> example-dev-ViewOnlyAccess
  app-deploy                 -> app-deploy
  app-readonly               -> app-readonly
```

Then ask, in one message:

1. **Which profiles to create:** all of them, or a subset. Accept a subset by account, by role, or by individual entry.
2. **Whether to rename any of them:** the proposed names are the default; the user may supply a custom alias for any entry. Re-check for collisions after applying custom names.

Wait for the answer before writing anything.

**Success:** A confirmed list of `{profileName, accountId, roleName}` entries.

If this step fails, see [Troubleshooting: Step 8](#troubleshooting-step-8-propose-profile-names).

---

## Step 9: Write the profiles into `~/.aws/config`

Each selected entry becomes a section of this shape. The first body line is the managed marker, which is how Step 0 recognizes profiles this file generated on a later run:

```ini
[profile example-prod-AdministratorAccess]
# managed-by: 24g-sso-setup
# account-name: example-prod
sso_session = 24g
sso_account_id = 111122223333
sso_role_name = AdministratorAccess
region = us-east-1
output = json
```

`region` is `default_region` from Step 4. `sso_session` is always `24g` — the account ID, Region, and start URL of the portal itself live in the `[sso-session 24g]` section, not in the profile.

`# account-name:` records the account name from Step 7. It is needed because the account name is otherwise unrecoverable from the profile: `sso_account_id` is stored but the name is not, and a profile named for a project role such as `app-deploy` carries no hint of which account it belongs to. Write it for every generated profile, including ones where the name already appears in the profile name.

Write the sections with `aws configure set`, which creates or updates a profile in place without disturbing the rest of the file:

```bash
aws configure set profile.example-prod-AdministratorAccess.sso_session 24g
aws configure set profile.example-prod-AdministratorAccess.sso_account_id 111122223333
aws configure set profile.example-prod-AdministratorAccess.sso_role_name AdministratorAccess
aws configure set profile.example-prod-AdministratorAccess.region us-east-1
aws configure set profile.example-prod-AdministratorAccess.output json
```

`aws configure set` creates the section if it is absent and updates keys in place if it is present, leaving comments and every other section of the file untouched. It cannot write comments, so after setting the keys, insert the `# managed-by: 24g-sso-setup` and `# account-name: <accountName>` lines directly beneath each generated `[profile ...]` header, skipping either line that is already present. It also appends new sections without a blank separator line; adding one is cosmetic but makes the file easier for the user to read.

Rules for this step:

- **Re-runs replace, they do not duplicate.** If a section with the same profile name already exists and carries the managed marker, update its keys in place. If it exists without the marker, it is hand-written: leave it alone and report it to the user.
- **Never touch sections you did not generate.** Hand-written profiles, comments, and unrelated sections must survive byte for byte.
- **Do not remove profiles** for accounts or roles the user has lost access to in this version. Report them instead so the user can decide.

**Success:** `aws configure list-profiles` lists every selected profile name.

If this step fails, see [Troubleshooting: Step 9](#troubleshooting-step-9-write-the-profiles).

---

## Step 10: Verify

Confirm one profile end to end. Prefer a read-only role for the check:

```bash
aws sts get-caller-identity --profile example-prod-ViewOnlyAccess
```

**Success:** Returns `Account`, `Arn`, and `UserId`, where `Account` matches the profile's `sso_account_id` and `Arn` contains the expected role name.

Then report to the user:

- The number of profiles created, and their names
- The rollback point path, and that it is still in place
- Any pre-existing profiles or conflicts that were left untouched
- That the session expires (typically after 12 hours) and is renewed with `aws sso login --sso-session 24g`
- That re-running this file after being granted a new account or role will pick it up

If this step fails, see [Troubleshooting: Step 10](#troubleshooting-step-10-verify).

---

## Step 11: Confirm the result and clean up

Ask the user to confirm the setup is good before removing anything. Give them something concrete to check rather than a bare yes-or-no question: the profile list from Step 9, the identity returned in Step 10, and a suggestion to try a profile they actually use, for example `aws s3 ls --profile <one of theirs>`.

Then offer three outcomes and wait:

1. **It works.** Delete the rollback point.
2. **Something is wrong.** [Roll back](#roll-back), then report what happened.
3. **Not sure yet.** Leave the rollback point in place and tell the user where it is, how to restore from it later, and that re-running this file will create a new one rather than overwrite it.

Only on answer 1, delete the run's backup directory:

**macOS / Linux:**

```bash
BACKUP_DIR="$(find "$HOME/.aws/.24g-setup-backups" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort | tail -1)"
[ -n "$BACKUP_DIR" ] && rm -rf "$BACKUP_DIR"
rmdir "$HOME/.aws/.24g-setup-backups" 2>/dev/null
echo "Removed the rollback point."
```

**Windows (PowerShell):**

```powershell
$BackupDir = Get-ChildItem "$env:USERPROFILE\.aws\.24g-setup-backups" -Directory -ErrorAction SilentlyContinue |
  Sort-Object Name | Select-Object -Last 1
if ($BackupDir) { Remove-Item $BackupDir.FullName -Recurse -Force }
"Removed the rollback point."
```

The `rmdir` removes the parent only when it is empty, so backups from earlier runs are left alone. If earlier runs did leave directories behind, list them and ask whether to remove those too; do not assume.

**Success:** The user has confirmed, and either the backup directory is gone or they have chosen to keep it.

---

## Step 12: Optional `gsso` helper commands

Specified but not yet built. See [`gsso/24GCLISpec.md`](../gsso/24GCLISpec.md).

When it exists, this step will offer to install an optional `gsso` script providing `login`, `list`, `add`, `remove`, and `switch`. It is genuinely optional: every profile written by Step 9 already works with plain `aws --profile <name>`.

For now, skip this step. Tell the user the setup is complete and that profile switching is done with `--profile` or by exporting `AWS_PROFILE` themselves:

```bash
export AWS_PROFILE=example-prod-ViewOnlyAccess
```

---

## Troubleshooting

If a step fails with an error not covered in its table, report the full error output to the user and do not proceed to the next step. If the failure came after the run had already written something, also offer a [rollback](#roll-back). Then offer to [file an issue](#report-an-uncaught-error).

### Report an uncaught error

An error with no matching troubleshooting row is a gap in this file, so it is worth recording. Do this **after** the normal flow — report the error, stop, offer the rollback — not instead of it. Fixing the user's machine comes first; filing the issue is cleanup.

Ask before filing. This posts the user's system information to a shared repository, and they may prefer to report it themselves or not at all. If they decline, print the filled-in body so they have it, and stop.

**1. Collect diagnostics.** This reports versions and counts only. It contains no account IDs, profile names, or tokens, which is what makes it safe to paste into an issue.

**macOS / Linux:**

```bash
CFG="$HOME/.aws/config"
CACHE_N=$(ls -1 "$HOME"/.aws/sso/cache/*.json 2>/dev/null | wc -l | tr -d ' ')
if [ "$CACHE_N" -gt 0 ]; then
  TOKEN_OK=$(jq -rs --arg url 'https://24glogin.awsapps.com/start' \
    '[ .[] | select(.accessToken != null and .startUrl == $url)
           | select((.expiresAt | sub("\\.[0-9]+"; "") | sub("UTC$"; "Z") | fromdateiso8601) > now) ]
     | if length > 0 then "yes" else "no (all expired or none for 24G)" end' "$HOME"/.aws/sso/cache/*.json 2>/dev/null)
else
  TOKEN_OK="no cache files"
fi
cat <<EOF
os:                 $(uname -srm)
shell:              $SHELL
aws_cli:            $(aws --version 2>&1)
jq:                 $(jq --version 2>&1)
config_exists:      $([ -f "$CFG" ] && echo yes || echo no)
sso_session_24g:    $(grep -c '^\[sso-session 24g\]' "$CFG" 2>/dev/null || echo 0)
total_profiles:     $(grep -c '^\[profile ' "$CFG" 2>/dev/null || echo 0)
managed_profiles:   $(grep -c '^# managed-by: 24g-sso-setup' "$CFG" 2>/dev/null || echo 0)
sso_cache_files:    $CACHE_N
valid_24g_token:    ${TOKEN_OK:-unknown}
rollback_points:    $(find "$HOME/.aws/.24g-setup-backups" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l | tr -d ' ')
AWS_CONFIG_FILE:    ${AWS_CONFIG_FILE:-unset}
AWS_PROFILE:        ${AWS_PROFILE:-unset}
AWS_DEFAULT_REGION: ${AWS_DEFAULT_REGION:-unset}
EOF
```

**Windows (PowerShell):**

```powershell
$Cfg = "$env:USERPROFILE\.aws\config"
$CacheFiles = @(Get-ChildItem "$env:USERPROFILE\.aws\sso\cache\*.json" -ErrorAction SilentlyContinue)
$TokenOk = if ($CacheFiles.Count -eq 0) { 'no cache files' } else {
  $valid = $CacheFiles | ForEach-Object { Get-Content $_.FullName -Raw | ConvertFrom-Json } |
    Where-Object { $_.accessToken -and $_.startUrl -eq 'https://24glogin.awsapps.com/start' -and
                   [datetimeoffset]($_.expiresAt -replace 'UTC$','Z') -gt [datetimeoffset]::UtcNow }
  if ($valid) { 'yes' } else { 'no (all expired or none for 24G)' }
}
@"
os:                 $([System.Environment]::OSVersion.VersionString) $env:PROCESSOR_ARCHITECTURE
shell:              PowerShell $($PSVersionTable.PSVersion)
aws_cli:            $(aws --version 2>&1)
jq:                 $(jq --version 2>&1)
config_exists:      $(if (Test-Path $Cfg) { 'yes' } else { 'no' })
sso_session_24g:    $(@(Select-String -Path $Cfg -Pattern '^\[sso-session 24g\]' -ErrorAction SilentlyContinue).Count)
total_profiles:     $(@(Select-String -Path $Cfg -Pattern '^\[profile ' -ErrorAction SilentlyContinue).Count)
managed_profiles:   $(@(Select-String -Path $Cfg -Pattern '^# managed-by: 24g-sso-setup' -ErrorAction SilentlyContinue).Count)
sso_cache_files:    $($CacheFiles.Count)
valid_24g_token:    $TokenOk
rollback_points:    $(@(Get-ChildItem "$env:USERPROFILE\.aws\.24g-setup-backups" -Directory -ErrorAction SilentlyContinue).Count)
AWS_CONFIG_FILE:    $(if ($env:AWS_CONFIG_FILE) { $env:AWS_CONFIG_FILE } else { 'unset' })
AWS_PROFILE:        $(if ($env:AWS_PROFILE) { $env:AWS_PROFILE } else { 'unset' })
AWS_DEFAULT_REGION: $(if ($env:AWS_DEFAULT_REGION) { $env:AWS_DEFAULT_REGION } else { 'unset' })
"@
```

**2. Check for a duplicate.** Someone may have hit this already.

```bash
gh issue list --repo twentyfourg/24g-aws-sso --state all --limit 20 --search "<a distinctive phrase from the error>"
```

If a match exists, add a comment with your diagnostics instead of opening a new issue, and tell the user which issue you commented on.

**3. Fill in the template.** Use [`.github/ISSUE_TEMPLATE/setup-failure.md`](.github/ISSUE_TEMPLATE/setup-failure.md). If this file was fetched by URL and the repository is not checked out locally, read the template from:

```
https://raw.githubusercontent.com/twentyfourg/24g-aws-sso/main/.github/ISSUE_TEMPLATE/setup-failure.md
```

Fill every section and strip the HTML comments. The template's own comments explain what each section is for. Two sections carry most of the diagnostic value, so do not shortcut them:

- **Command and error output** — the complete output, not just the final line. The lines before an error usually identify the cause; three of the traps documented in this file produce a misleading last line.
- **Trace** — the steps that ran before the failure, which ones Step 0 let you skip, and any question you asked with the answer you got. A wrong branch is usually chosen several steps before the error appears.

**4. Redact before posting.** [Secret handling](#secret-handling) applies to the issue body in full, and an issue is the highest-risk place in this workflow for a leak: it is durable, it notifies people by email, and editing it later does not undo the exposure.

Re-read the assembled body line by line before you file it. Check specifically for a token in pasted error output, and for a full dump of `~/.aws/config` or `~/.aws/credentials`. If you redact something, say so in the issue.

Account IDs, account names, and profile names are internal but not secret, and they are often what makes a naming bug diagnosable. Include them when they are relevant to the failure, and tell the user they are in the body before you file.

**5. File it.**

```bash
gh issue create --repo twentyfourg/24g-aws-sso \
  --title "[setup] Step <N> failed: <one-line summary>" \
  --body-file <path to the filled-in body> \
  --label setup-failure
```

`gh issue create` fails if the label does not exist in the repository. If that happens, re-run without `--label` rather than creating the label.

If `gh` is not installed or not authenticated (`gh auth status`), do not install it as a side effect of this workflow. Print the complete body and give the user this link to paste it into:

```
https://github.com/twentyfourg/24g-aws-sso/issues/new?template=setup-failure.md
```

**Success:** The issue URL, reported to the user along with the reminder that their machine may still need a rollback if they have not decided yet.

### Troubleshooting: Step 0 (Determine the current state)

| Symptom                                                      | Cause                                                               | Resolution                                                                                                                                           |
| ------------------------------------------------------------ | ------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `aws: command not found`                                     | AWS CLI not installed, or not on PATH                               | Run `ls -la ~/.local/bin/aws` and `which -a aws`. If a binary exists, the problem is PATH; fix PATH rather than reinstalling. Otherwise go to Step 2 |
| `aws --version` reports `aws-cli/1.x`                        | AWS CLI v1 is first on PATH                                         | `sso-session` requires v2. Run `which -a aws` to find all installs, show the user the locations, and ask whether to remove v1 or reorder PATH        |
| `~/.aws` does not exist                                      | Fresh machine, or the user keeps config elsewhere                   | Check whether `AWS_CONFIG_FILE` is set to a different path. If not, Step 4 creates the directory                                                     |
| `AWS_CONFIG_FILE` or `AWS_PROFILE` is set in the environment | The user has a non-standard setup                                   | Ask the user where their config lives and whether these should stay set. Every path in this file assumes the default location                        |
| `~/.aws/config` exists but is not valid INI                  | Manual edit introduced a syntax error                               | Show the user the offending lines. Do not attempt to repair it automatically; ask how to proceed                                                     |
| A `[sso-session]` section exists under a different name      | The user is already configured for another Identity Center instance | Leave it alone. Adding `[sso-session 24g]` alongside it is safe; explain this to the user                                                            |

### Troubleshooting: Step 1 (Determine the operating system)

| Symptom                                                        | Cause                                  | Resolution                                                                                                |
| -------------------------------------------------------------- | -------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| Cannot determine OS                                            | No shell access or unknown environment | Ask the user what operating system they are using                                                         |
| WSL detected (`uname -s` reports `Linux` on a Windows machine) | Running inside WSL                     | Treat it as Linux, and tell the user the config written inside WSL is separate from any config in Windows |

### Troubleshooting: Step 2 (Install the AWS CLI)

| Symptom                                                      | Cause                                                     | Resolution                                                                                                                                                  |
| ------------------------------------------------------------ | --------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `command not found: curl`                                    | Download tool missing                                     | Install `curl` with the system package manager, then re-run                                                                                                 |
| `curl` exits non-zero (e.g. 22)                              | HTTP error or no connectivity                             | Verify network access to `https://awscli.amazonaws.com`                                                                                                     |
| `missing required dependencies: ...`                         | `unzip` (Linux) or `pkgutil` (macOS) missing              | Install the listed dependencies, then re-run                                                                                                                |
| `unsupported OS` or `unsupported architecture`               | Installer supports Linux (x86_64, aarch64) and macOS only | Cannot proceed on this system                                                                                                                               |
| `post-install check failed`                                  | `aws --version` did not succeed after install             | Confirm `$HOME/.local/bin` is on PATH, then re-run                                                                                                          |
| `aws --version` reports an older version than just installed | An older install elsewhere takes PATH precedence          | Run `which -a aws` (`Get-Command aws -All` on Windows), show the user the locations, and offer to remove the old install or reorder PATH. Ask before acting |
| `irm`/`iex` not recognized                                   | Running in `cmd.exe`                                      | Re-run from PowerShell                                                                                                                                      |
| `msiexec failed with exit code ...`                          | MSI install failed                                        | Check the Windows Event Log; make sure no other AWS CLI installer is running                                                                                |
| Permission denied writing the shell rc file                  | File permissions                                          | Check with `ls -la "$SHELL_RC"` and fix with `chmod u+w "$SHELL_RC"`                                                                                        |

### Troubleshooting: Step 3 (Install jq)

| Symptom                            | Cause                                   | Resolution                                                                                                             |
| ---------------------------------- | --------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `brew: command not found`          | Homebrew not installed                  | Use the static binary download instead. Do not install Homebrew as a side effect of this setup                         |
| `sudo` prompts for a password      | Package manager install needs elevation | The agent cannot type a password. Give the user the exact command to run in their terminal, then wait for confirmation |
| `winget: command not found`        | Older Windows or App Installer missing  | Download `jq.exe` from the [jq releases page](https://github.com/jqlang/jq/releases/latest) into a directory on PATH   |
| `jq` installed but still not found | PATH not refreshed                      | Run `export PATH="$HOME/.local/bin:$PATH"`, or open a new shell on Windows                                             |

### Troubleshooting: Step 4 (Configure the `[default]` section)

| Symptom                                                         | Cause                                    | Resolution                                                                                                                                                           |
| --------------------------------------------------------------- | ---------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `[default]` already has a different `region`                    | The user has an existing preference      | Do not overwrite. Ask which Region they want, and use the answer for every profile in Step 9                                                                         |
| `~/.aws/credentials` has a `[default]` section with static keys | Legacy static credentials                | Leave the file untouched. Warn the user that those keys take precedence over `[default]` in `config` for the default profile, and that named profiles are unaffected |
| Permission denied writing `~/.aws/config`                       | File owned by another user, or read-only | Show `ls -la ~/.aws/`, and ask the user to fix ownership or permissions                                                                                              |

### Troubleshooting: Step 5 (Configure the SSO session and sign in)

| Symptom                                                     | Cause                                                               | Resolution                                                                                                                              |
| ----------------------------------------------------------- | ------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `Unknown component: sso-session` or the section is ignored  | AWS CLI v1                                                          | Go back to Step 2 and install v2                                                                                                        |
| Browser does not open                                       | Headless environment, no default browser, or a remote/SSH session   | Re-run with `aws sso login --sso-session 24g --no-browser` and give the user the verification URL and code to open on their own machine |
| `aws sso login` exits non-zero                              | The user closed the browser or the request timed out                | Re-run the command and let the user finish the browser flow                                                                             |
| Browser shows an authorization or access error              | The user's identity is not assigned to the Identity Center instance | This is not fixable locally. Tell the user to contact whoever administers AWS access                                                    |
| Sign-in succeeds but no file appears in `~/.aws/sso/cache/` | `AWS_CONFIG_FILE` or a non-standard `HOME` is redirecting the cache | Check those environment variables and locate the real cache directory before Step 6                                                     |

### Troubleshooting: Step 6 (Read the access token)

| Symptom                                                               | Cause                                                                                | Resolution                                                                                                                                                                                        |
| --------------------------------------------------------------------- | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Token variable is empty                                               | No cache file matched the 24G start URL                                              | Confirm Step 5 actually completed, then re-run `aws sso login --sso-session 24g`                                                                                                                  |
| Token variable is empty but files exist in the cache                  | Every matching token has expired, or the files belong to a different start URL       | Inspect with `jq -r '.startUrl, .expiresAt' ~/.aws/sso/cache/*.json` (this prints no secrets) and re-run Step 5                                                                                   |
| `jq: error ... Cannot index string` or a parse error                  | A cache file is a client registration file rather than a token file, or is malformed | The `select(.accessToken != null)` filter already skips registration files. For a malformed file, process the files one at a time to identify it                                                  |
| `jq: error: date "..." does not match format "%Y-%m-%dT%H:%M:%SZ"`    | `expiresAt` has fractional seconds or the legacy `...UTC` suffix                     | The two `sub` calls in the Step 6 command handle both. Confirm they were not dropped. To inspect formats without printing secrets: `jq -r '.startUrl // "-", .expiresAt' ~/.aws/sso/cache/*.json` |
| `UnauthorizedException: Session token not found or invalid` in Step 7 | The variable was empty because Steps 6 and 7 ran in separate shell processes         | Re-run both in one command block before concluding the session expired                                                                                                                            |

### Troubleshooting: Step 7 (Enumerate accounts and roles)

| Symptom                                           | Cause                                                                                                                                                              | Resolution                                                                                                     |
| ------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------- |
| `UnauthorizedException`                           | The access token expired between Step 6 and now                                                                                                                    | Re-run Step 5, then Step 6                                                                                     |
| `TooManyRequestsException`                        | Throttled by calling `list-account-roles` once per account                                                                                                         | Retry with a short backoff. Do not run the per-account calls in parallel                                       |
| `Could not connect to the endpoint URL`           | Wrong Region on the `sso` client                                                                                                                                   | These calls must use `--region us-east-1`, the Identity Center Region, regardless of the user's default Region |
| `The specified sso-session does not exist: "24g"` | `AWS_CONFIG_FILE` points at a file without the `[sso-session 24g]` section. These calls still resolve the config file even though the token is supplied explicitly | Run `echo $AWS_CONFIG_FILE`. Unset it, or point it at the file Step 5 wrote                                    |
| Account list is empty                             | The user has no account assignments                                                                                                                                | Not a local problem. Tell the user to contact whoever administers AWS access                                   |
| An account returns no roles                       | No permission sets assigned in that account                                                                                                                        | Skip it and note it in the summary                                                                             |

### Troubleshooting: Step 8 (Propose profile names)

| Symptom                                                                | Cause                                                                                                                | Resolution                                                                                                              |
| ---------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| Two generated names are identical                                      | The same project role name exists in multiple accounts                                                               | Prefix both with the account name, per the collision rule                                                               |
| A generated name matches an existing, unmanaged profile                | The user already hand-wrote a profile with that name, in either `config` or `credentials`                            | Do not overwrite, and do not edit `credentials`. Show the existing section and ask for a different name                 |
| `aws configure list-profiles` shows a name like `profile example-prod` | A section in `~/.aws/credentials` was written with the `[profile name]` header form, which is only valid in `config` | Pre-existing and out of scope here. Report it to the user; do not fix it as part of this setup                          |
| A role name does not clearly fit either category                       | Naming convention drift in Identity Center                                                                           | Show the role to the user with both candidate names and let them choose                                                 |
| An account name contains spaces or unusual characters                  | Account naming in Identity Center                                                                                    | Replace whitespace with `-` and strip characters outside `[A-Za-z0-9._-]`. Show the result to the user for confirmation |

### Troubleshooting: Step 9 (Write the profiles)

| Symptom                                                            | Cause                                                            | Resolution                                                                                                     |
| ------------------------------------------------------------------ | ---------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| Profile written but `aws configure list-profiles` does not show it | Wrote to the wrong file, or a syntax error in the section header | Confirm the header is `[profile <name>]` in `config` — the bare `[<name>]` form is only valid in `credentials` |
| Re-run produced duplicate sections                                 | The existing section was appended to instead of updated          | Remove the duplicate, keep the one with the managed marker, and re-apply with `aws configure set`              |
| Comments or hand-written sections disappeared                      | The file was rewritten wholesale instead of edited in place      | [Roll back](#roll-back), then redo the step using `aws configure set`                                          |

### Troubleshooting: Step 10 (Verify)

| Symptom                                                         | Cause                                                                                                         | Resolution                                                                                                       |
| --------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| `The SSO session associated with this profile has expired`      | Session expired                                                                                               | Run `aws sso login --sso-session 24g`                                                                            |
| `ForbiddenException` or `AccessDenied` on `get-caller-identity` | The role exists but the session cannot assume it                                                              | Confirm `sso_account_id` and `sso_role_name` in the profile match the values from Step 7 exactly, including case |
| `ProfileNotFound`                                               | Name mismatch between what was written and what was used                                                      | Compare against `aws configure list-profiles`                                                                    |
| `get-caller-identity` returns an unexpected account             | `AWS_PROFILE` is set in the environment and overrides expectations, or the wrong `sso_account_id` was written | Check `echo $AWS_PROFILE` and the profile section                                                                |

### Troubleshooting: Step 11 (Confirm the result and clean up)

| Symptom                                       | Cause                                                            | Resolution                                                                                                          |
| --------------------------------------------- | ---------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| No rollback directory found                   | The run made no changes, or the rollback point was never created | If the run wrote anything, that is a process failure worth reporting. Either way there is nothing to clean up       |
| Several rollback directories exist            | Earlier runs were never cleaned up                               | List them with their timestamps and ask which to remove. Only this run's directory is safe to delete without asking |
| Permission denied removing the directory      | Ownership or permissions on `~/.aws`                             | Show `ls -la ~/.aws/.24g-setup-backups/` and let the user remove it themselves. Do not escalate with `sudo`         |
| The user is unsure whether the setup is right | Reasonable, and nothing is broken                                | Leave the rollback point in place. Tell them where it is and how to restore from it. Keeping it costs nothing       |
