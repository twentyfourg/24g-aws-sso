# 24G AWS SSO and MCP Setup

Sets up a developer's machine for 24G AWS access through IAM Identity Center: installs the AWS CLI, configures `~/.aws/config`, and generates a named profile for every account and role you've been granted. Do it by hand or hand the prompt below to an AI coding tool.

Optionally, configure local AI coding agents to use an approved subset of those profiles through the official [AWS MCP Server](https://docs.aws.amazon.com/agent-toolkit/latest/userguide/getting-started-aws-mcp-server.html).

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

Set this up by hand or by handing the setup instructions to an AI coding tool.

<details>
<summary>Manual setup</summary>

1. Install [AWS CLI v2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html). (v1 does not support `sso-session`.)
2. Add the 24G SSO session and one profile per role to `~/.aws/config`:

   ```ini
   [sso-session 24g]
   sso_start_url = https://24glogin.awsapps.com/start
   sso_region = us-east-1
   sso_registration_scopes = sso:account:access

   [profile 1234-job-name]
   sso_session = 24g
   sso_account_id = 111122223333
   sso_role_name = 1234-1-job-name-sso-role-name
   region = us-east-1
   output = json
   ```

   The `[sso-session 24g]` block is written once. Add a `[profile ...]` block for every account and role you've been granted, each with `sso_session = 24g` and its own `sso_account_id` and `sso_role_name`. Find those account IDs and role names in the [IAM Identity Center access portal](https://24glogin.awsapps.com/start).

3. Sign in:

   ```bash
   aws sso login --sso-session 24g
   ```

4. Verify:

   ```bash
   aws sts get-caller-identity --profile <profile-name>
   ```

</details>

<details>
<summary>AI-driven setup</summary>

Paste this into your AI coding tool and it does the above for every account and role you have access to in one pass, with a rollback point if anything goes wrong:

```
Set up 24G AWS SSO by following instructions:
https://raw.githubusercontent.com/twentyfourg/24g-aws-sso/main/setup-instructions/24GAWSSOSetup.md
```

</details>

### Using a profile

Set `AWS_PROFILE` environment variable for the terminal session. The AWS CLI and SDKs then use that profile until you change it or close the terminal.

<details>
<summary>macOS and Linux</summary>

```bash
export AWS_PROFILE=acme-read
```

Or add a function to `~/.bashrc` or `~/.zshrc`:

```bash
switch () { export AWS_PROFILE="$1" ; echo "AWS_PROFILE is now $1"; }
```

Open a new terminal, then:

```bash
switch acme-read
```

</details>

<details>
<summary>Windows (PowerShell)</summary>

```powershell
$env:AWS_PROFILE = "acme-read"
```

Or add a function to your PowerShell profile (`notepad $PROFILE`):

```powershell
function switch-aws { param([string]$profile) $env:AWS_PROFILE = $profile; Write-Host "AWS_PROFILE is now $env:AWS_PROFILE" }
```

Open a new terminal, then:

```powershell
switch-aws acme-read
```

</details>

[`gsso`](#gsso) can switch the profile for you. See that section for setup and its other commands.

You can also pass the profile on one command, or select it in SDK code:

```bash
aws s3 ls --profile acme-read
```

```typescript
const s3 = new S3Client({
  region: "us-west-2",
  credentials: fromIni({ profile: "acme-read" }),
});
```

Sessions last about 12 hours. Renew with:

```bash
aws sso login --sso-session 24g
```

## AWS MCP With Profiles

The AWS MCP Server uses the [AWS MCP Proxy](https://github.com/aws/mcp-proxy-for-aws) to sign requests with your named profiles. For each agent, this adds an `aws-mcp` MCP server entry whose `AWS_MCP_PROXY_PROFILES` env var is an explicit allowlist: only the profiles listed can be used, the first one is the default, and the agent can pick another listed profile for an individual call.

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

This requires working 24G SSO profiles first; see [AWS SSO](#aws-sso) above. Set the MCP entry up by hand or by handing the setup instructions to an AI coding tool.

<details>
<summary>Manual setup</summary>

Edit each agent's `aws-mcp` entry directly, preserving every other key.

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

</details>

<details>
<summary>AI-driven setup</summary>

Paste this into your AI coding tool. It checks for working 24G SSO profiles first and runs the SSO workflow if they are missing, then asks which profiles each detected agent may use before writing anything:

```
Set up the AWS MCP Server for my agents by following instructions:
https://raw.githubusercontent.com/twentyfourg/24g-aws-sso/main/setup-instructions/24GAWSMCPSetup.md
```

</details>

[`gsso`](#gsso) can update the MCP allowlist across every detected agent at once. See that section for setup and its other commands.

#### Skills

AWS has prebuilt, high quality [skills](https://github.com/aws/agent-toolkit-for-aws/tree/main/skills) that can be added to your agent. Copy a skill from the repo into the skills directory your coding agent reads, or use the [AWS CLI to add it](https://docs.aws.amazon.com/agent-toolkit/latest/userguide/aws-cli.html#aws-cli-add-skill) for you.

```bash
aws agent-toolkit add-skill \
    --skill-name aws-serverless
```

## gsso

An optional helper for 24G AWS SSO profiles. `gsso` switches `AWS_PROFILE`, adds and removes named profiles in `~/.aws/config` without hand-editing it, and updates every detected agent's MCP allowlist at once instead of editing each agent's config file.

A single binary for macOS, Linux, and Windows. AWS CLI v2 must be on `PATH`. Build and command reference: [`gsso/README.md`](gsso/README.md).

### Install

Run the command for your platform. It downloads the latest release for your CPU (amd64 or arm64), puts it on `PATH`, and registers the shell hook that `gsso switch` needs to set `AWS_PROFILE`. Then open a new terminal.

<details>
<summary>macOS / Linux (bash or zsh)</summary>

```bash
curl -fsSL https://raw.githubusercontent.com/twentyfourg/24g-aws-sso/main/gsso/install.sh | bash
```

</details>

<details>
<summary>Windows (PowerShell)</summary>

```powershell
irm https://raw.githubusercontent.com/twentyfourg/24g-aws-sso/main/gsso/install.ps1 | iex
```

</details>

Install paths, building from source, and registering the hook by hand: [`gsso/README.md`](gsso/README.md#install).

### Update

Run the same install command again. It replaces `gsso` with the latest release and prints `Updated gsso <old> -> <new>`, or that it is already the latest. Open a new terminal afterwards so the shell loads the updated hook. Details: [`gsso/README.md`](gsso/README.md#update).

### Everyday use

```bash
gsso login
gsso list
gsso configure
gsso switch acme-read
gsso mcp add
```

- `gsso login` renews the 24G SSO session.
- `gsso list` / `gsso configure` list, and add or remove, managed profiles in one searchable checkbox menu.
- `gsso switch <profile>` sets `AWS_PROFILE` for the current shell.
- `gsso mcp add` / `gsso mcp remove` / `gsso mcp default` edit the `AWS_MCP_PROXY_PROFILES` allowlist on every detected agent (Cursor, Claude Code, Kiro, Codex) at once.

See [`gsso/README.md`](gsso/README.md) for the full command reference, naming rules, and rollback behavior.

## Requirements

- macOS, Linux, or Windows
- An identity assigned access in IAM Identity Center
- AWS CLI v2 and `jq` — the setup installs them if they are missing

AWS CLI v1 will not work; the `sso-session` configuration style needs v2.

> The macOS and Linux commands are tested. The Windows and PowerShell paths are written but have not been run on a real Windows machine yet. If you are the first to try it, expect rough edges and please [open an issue](#if-something-goes-wrong).
