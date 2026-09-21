# 24G AWS SSO and MCP Setup

Sets up a developer's machine for 24G AWS access through IAM Identity Center. Hand the prompt below to your AI coding tool and it installs the AWS CLI and `jq`, configures `~/.aws/config`, signs you in through the browser, and generates a named profile for every account and role you have been granted.

Optionally, configure local AI coding agents to use an approved subset of those profiles through the official [AWS MCP Server](https://docs.aws.amazon.com/agent-toolkit/latest/userguide/getting-started-aws-mcp-server.html).

An optional 24G helper SSO CLI adds commands for switching between and adding or removing roles.

## Usage

### 24G AWS SSO

Paste this into your AI coding tool:

```
Set up 24G AWS SSO by following instructions:
https://raw.githubusercontent.com/twentyfourg/24g-aws-sso/main/setup-instructions/24GAWSSOSetup.md
```

### AWS MCP setup for your agents

The MCP workflow checks for working 24G SSO profiles first and runs the SSO workflow if they are missing.

```
Set up the AWS MCP Server for my agents by following instructions:
https://raw.githubusercontent.com/twentyfourg/24g-aws-sso/main/setup-instructions/24GAWSMCPSetup.md
```

## AWS SSO

The setup writes a `[sso-session 24g]` block in `~/.aws/config` plus one [named profile](https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-files.html#cli-configure-files-using-profiles) per Identity Center role you choose:

```ini
[sso-session 24g]
sso_start_url = https://24glogin.awsapps.com/start
sso_region = us-east-1
sso_registration_scopes = sso:account:access

[profile example-prod-AdministratorAccess]
# managed-by: 24g-sso-setup
sso_session = 24g
sso_account_id = 111122223333
sso_role_name = AdministratorAccess
region = us-east-1
output = json
```

Profiles are named `<account>-<role>` for roles that exist in several accounts, such as `AdministratorAccess` and `ViewOnlyAccess`, and just `<role>` for project-specific permission sets like `app-deploy`. The agent shows the full list and proposed names before writing anything, and you can rename any of them or take a subset.

Use a profile with the AWS CLI or SDK:

```bash
aws s3 ls --profile acme-read
```

```typescript
const s3 = new S3Client({
  region: "us-west-2",
  credentials: fromIni({ profile: "acme-read" }),
});
```

You can also set `AWS_PROFILE` for the terminal session, or use the `gsso` helper:

```bash
export AWS_PROFILE=acme-read
gsso list
gsso configure
gsso switch acme-read
```

`gsso configure` opens one searchable checkbox menu containing every role
reported by IAM Identity Center. Existing profiles are checked. Check roles to
add them, uncheck roles to remove them, and press Enter to apply all changes
under one rollback point. A configured role that is no longer reported remains
visible and checked with a `not currently available` label, so it is never
removed silently. Use `--hard-refresh` to bypass the local portal cache.

To override the generated profile name when adding one role directly:

```bash
gsso add --alias foo-bar acme-account/api-service
```

`--alias` requires exactly one `ACCOUNT/ROLE`. During `gsso configure`, each
newly checked role gets a profile-name prompt; press Enter to accept the
generated name. Existing checked profiles keep their current names.

Sessions last about 12 hours. Renew with:

```bash
aws sso login --sso-session 24g
# Or
gsso login
```

## AWS MCP With Profiles

The AWS MCP Server uses the [AWS MCP Proxy](https://github.com/aws/mcp-proxy-for-aws) to sign requests with these named profiles. The proxy is an allowlist: only profiles you list can be used, and the agent can pick another listed profile for an individual call.

Edit each agent's `aws-mcp` entry by hand, or update every detected agent at once:

```bash
gsso mcp add
gsso mcp default readonly-24g
gsso mcp remove production-24g
```

The first name in `AWS_MCP_PROXY_PROFILES` is the default. `gsso mcp default` moves that profile to the front without changing the rest of the list. Restart the agent after you change it.

If the detected agents already disagree (for example Codex was edited by hand), `gsso` does not guess a union. It truncates to the profiles present in every agent, keeps that shortest list's order, then writes the resulting shared list to all of them. If they share no profiles, the current list is treated as empty and `gsso mcp add PROFILE` becomes the new shared allowlist.

Without profile arguments, `gsso mcp add` edits the whole allowlist: it lists every local profile with the allowed ones already checked, so unchecking one removes it. Up/Down moves, Space toggles, `/` starts a profile-name search, `d` marks the default, and Enter saves exactly what is checked. While searching, type to filter, press Enter to finish the search, or Esc to clear it. `gsso mcp remove` shows the same menu limited to the profiles you currently allow. Limited terminals fall back to a numbered multi-select (`d3` marks item 3 as default).

<details>
<summary>Cursor</summary>

Edit `~/.cursor/mcp.json`. Add or update `env.AWS_MCP_PROXY_PROFILES` on the `aws-mcp` server. Leave `command` and `args` as they are.

```json
"aws-mcp": {
  "command": "uvx",
  "args": [
    "mcp-proxy-for-aws@latest",
    "https://aws-mcp.us-east-1.api.aws/mcp",
    "--metadata",
    "INSTALL_SOURCE=aws-cli"
  ],
  "env": {
    "AWS_MCP_PROXY_PROFILES": "readonly-24g app-24g production-24g"
  }
}
```

</details>

<details>
<summary>Claude Code</summary>

Edit the top-level `mcpServers` object in `~/.claude.json` (or `$CLAUDE_CONFIG_DIR/.claude.json` if that variable is set). Add or update `env.AWS_MCP_PROXY_PROFILES` on `aws-mcp`.

```json
"aws-mcp": {
  "command": "uvx",
  "args": [
    "mcp-proxy-for-aws@latest",
    "https://aws-mcp.us-east-1.api.aws/mcp",
    "--metadata",
    "INSTALL_SOURCE=aws-cli"
  ],
  "env": {
    "AWS_MCP_PROXY_PROFILES": "readonly-24g app-24g production-24g"
  }
}
```

</details>

<details>
<summary>Kiro</summary>

Edit `~/.kiro/settings/mcp.json`. Add or update `env.AWS_MCP_PROXY_PROFILES` on `aws-mcp`. Keep any existing `timeout` or `transport` keys.

```json
"aws-mcp": {
  "command": "uvx",
  "args": [
    "mcp-proxy-for-aws@latest",
    "https://aws-mcp.us-east-1.api.aws/mcp",
    "--metadata",
    "INSTALL_SOURCE=aws-cli"
  ],
  "env": {
    "AWS_MCP_PROXY_PROFILES": "readonly-24g app-24g production-24g"
  }
}
```

</details>

<details>
<summary>Codex</summary>

Edit `~/.codex/config.toml` (or `$CODEX_HOME/config.toml`). Codex uses `mcp_servers`, not `mcpServers`. Add or update the environment table; leave the generated `command` and `args` alone.

```toml
[mcp_servers.aws-mcp]
command = "uvx"
args = [
  "mcp-proxy-for-aws@latest",
  "https://aws-mcp.us-east-1.api.aws/mcp",
  "--metadata",
  "INSTALL_SOURCE=aws-cli",
]

[mcp_servers.aws-mcp.env]
AWS_MCP_PROXY_PROFILES = "readonly-24g app-24g production-24g"
```

If `env` is already an inline table on `[mcp_servers.aws-mcp]`, set `AWS_MCP_PROXY_PROFILES` there instead of adding a second table.

</details>

#### Skills

AWS has rebuilt, high quality [skills](https://github.com/aws/agent-toolkit-for-aws/tree/main/skills) that can be add to your agent. You can use manually copy them from the repo or use the [AWS CLI to add them](https://docs.aws.amazon.com/agent-toolkit/latest/userguide/aws-cli.html#aws-cli-add-skill)

```bash
aws agent-toolkit add-skill \
    --skill-name aws-serverless
```

## Requirements

- macOS, Linux, or Windows
- An identity assigned access in IAM Identity Center
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

| File                                      | What it is                                                                     |
| ----------------------------------------- | ------------------------------------------------------------------------------ |
| `setup-instructions/24GAWSSOSetup.md`     | Configures AWS CLI access through 24G IAM Identity Center                      |
| `setup-instructions/24GAWSMCPSetup.md`    | Configures approved AWS profiles for local AI agents through the AWS MCP proxy |
| `.github/ISSUE_TEMPLATE/setup-failure.md` | Issue template for a setup error with no documented resolution                 |
| `gsso/python/gsso`                        | Optional `gsso` helper for login, profile switching, and MCP allowlist edits   |
