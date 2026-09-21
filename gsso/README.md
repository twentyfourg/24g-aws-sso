# gsso

Optional helper for 24G AWS SSO profiles. It logs in through IAM Identity Center,
creates and removes named profiles in `~/.aws/config`, switches `AWS_PROFILE`, and
updates AWS MCP allowlists.

`gsso` never writes access keys and never writes `~/.aws/credentials`. It only
edits `~/.aws/config` (SSO) and detected agent MCP configs.

The script is [`python/gsso`](python/gsso). Standard library only. AWS CLI v2
must already be on `PATH`.

## Install

Put `gsso` on `PATH`. One option:

```bash
install -m 755 python/gsso "$HOME/.local/bin/gsso"
```

Then add a shell wrapper so `gsso switch` actually exports `AWS_PROFILE` in the
current shell, and so TAB completion works:

```bash
# bash
echo 'eval "$(gsso shell-init bash)"' >> ~/.bashrc

# zsh
echo 'eval "$(gsso shell-init zsh)"' >> ~/.zshrc
```

Without the wrapper, `gsso switch` prints `export AWS_PROFILE=...` and you have
to eval it yourself.

Requires macOS or Linux with an interactive TTY for menus. Python 3.9+ is
enough.

## Everyday use

```bash
gsso login
gsso list
gsso configure
eval "$(gsso switch --shell posix acme-read)"   # or just: gsso switch acme-read
gsso whoami
```

SSO sessions last about 12 hours. `gsso login` is `aws sso login --sso-session 24g`.
Extra arguments are forwarded, for example `gsso login --use-device-code`.

## Commands

### `gsso login`

Renew the 24G SSO session. Does not create or change profiles.

### `gsso list`

List managed profiles grouped by account. Marks the current `AWS_PROFILE`. Also
shows unmanaged config profiles and static credential profiles, without touching
them.

```bash
gsso list              # local ~/.aws/config
gsso list --remote     # Identity Center roles (uses portal cache)
gsso list --hard-refresh
```

`--hard-refresh` (alias `--hard`) re-queries Identity Center and rewrites
`~/.aws/gsso/portal-cache.json`. It implies `--remote`.

### `gsso configure`

Edit the complete set of managed SSO roles in one searchable checkbox menu.

- Lists every role Identity Center reports.
- Configured roles start checked.
- Check to add, uncheck to remove, Enter to apply.
- All adds and removes share one rollback point.
- A configured role that is no longer in the portal stays visible and checked,
  labeled `(not currently available)`, so it is never removed silently.

Each newly checked role then prompts for a profile name. Press Enter to accept
the generated name, or type a custom one. Existing profiles keep their names.

```bash
gsso configure
gsso configure --hard-refresh
```

### `gsso add [ACCOUNT/ROLE ...]`

Create managed profiles for roles that are not configured yet.

Without arguments, a multi-select lists unconfigured `ACCOUNT/ROLE` pairs.

With arguments, targets must be `account-name/role-name`, not a profile name
and not a bare role. Role names such as `AdministratorAccess` exist in many
accounts, so the account is required:

```bash
gsso add vxp-24g/2772-1-lxp-base
gsso add --alias lxp-base vxp-24g/2772-1-lxp-base
```

`--alias` requires exactly one target. `--hard-refresh` re-queries Identity
Center before adding.

### `gsso remove [PROFILE ...]`

Delete managed profiles. Targets are **profile names** in `~/.aws/config`, not
`ACCOUNT/ROLE` pairs, because those names are already unique.

Without arguments, a multi-select lists managed profiles. Unmanaged sections
and credential profiles are never removed.

```bash
gsso remove lxp-base
```

### `gsso switch [PROFILE]`

Print an export for `AWS_PROFILE` (or a PowerShell assignment with
`--shell powershell`). With the shell wrapper, the current shell is updated.

`--set-default` also copies the chosen profile's SSO keys into `[default]`.
That is refused if `~/.aws/credentials` already has a `[default]` unless you
confirm.

### `gsso whoami`

Call `sts get-caller-identity` for the current profile and print account, role,
ARN, and user id. This confirms the session still works, not only that
`AWS_PROFILE` is set.

### `gsso mcp add|remove|default`

Edit `AWS_MCP_PROXY_PROFILES` on every detected `aws-mcp` server (Cursor, Claude
Code, Kiro, Codex). The first name in that list is the default.

```bash
gsso mcp add                 # checkbox of every local profile
gsso mcp add readonly-24g
gsso mcp default readonly-24g
gsso mcp remove production-24g
```

Without arguments, `mcp add` is an exact-state editor: checked profiles stay,
unchecked ones are dropped. Named arguments are additive. `mcp remove` will not
empty the allowlist.

If agents disagree, `gsso` truncates to the intersection and writes that shared
list to all of them. Restart the agents after a change. MCP backups go under
`~/.aws/.24g-mcp-setup-backups/`.

## Names

Two different strings:

| What | Example | Used by |
| --- | --- | --- |
| Identity Center role | `vxp-24g/AdministratorAccess` | `add`, `list --remote`, `configure` menu |
| Local profile | `vxp-24g` or `vxp-24g-AdministratorAccess` | `remove`, `switch`, `mcp`, `list` |

Generated profile names:

- `<account>-<role>` when the role is a CamelCase system role (`AdministratorAccess`)
  or exists in more than one account.
- `<role>` when the permission set is unique, such as `2772-1-lxp-base`.

`--alias` and the configure prompt override that.

## Menus

On a POSIX TTY:

- Up / Down move
- Space toggles
- `/` starts a name search (Enter finishes, Esc clears)
- `d` marks the MCP default (`mcp add` only)
- Enter confirms
- Esc or `q` cancels

Limited terminals fall back to numbered multi-select (`1,3-4`, and `d3` for a
default). Non-interactive shells require the names as arguments.

## Cache and rollback

- Portal cache: `~/.aws/gsso/portal-cache.json`. `list --remote`, `add`, and
  `configure` reuse it until `--hard-refresh`.
- SSO rollback points: `~/.aws/.24g-setup-backups/<timestamp>/`
- MCP rollback points: `~/.aws/.24g-mcp-setup-backups/<timestamp>/`

A rollback point is a copy of the files about to change. Restore by copying
those files back. `gsso` does not delete backups.

## Tests

From the repository root:

```bash
python3 -m unittest gsso/python/test_gsso.py
```
