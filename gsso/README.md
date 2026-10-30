# gsso

Optional helper for 24G AWS SSO profiles. It logs in through IAM Identity Center,
creates and removes named profiles in `~/.aws/config`, switches `AWS_PROFILE`, and
updates AWS MCP allowlists.

`gsso` never writes access keys and never writes `~/.aws/credentials`. It only
edits `~/.aws/config` (SSO) and detected agent MCP configs.

It is a single binary in [`go/`](go). AWS CLI v2 must be on `PATH`. Every AWS
call shells out to the CLI.

## Install

Paste the command for your platform. It downloads the latest release for your
CPU (amd64 or arm64), puts it on `PATH`, and registers the shell hook. Run it
again to upgrade; it does not add duplicate lines.

<details>
<summary>macOS / Linux (bash or zsh)</summary>

```bash
(
  set -eu
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) echo "Unsupported CPU: $(uname -m)" >&2; exit 1 ;;
  esac
  url=$(curl -fsSL "https://api.github.com/repos/twentyfourg/24g-aws-sso/releases?per_page=100" |
    grep -o "https://[^\"]*/gsso-$os-$arch\"" | sed -n '1s/"$//p')
  [ -n "$url" ] || { echo "No gsso release found for $os-$arch" >&2; exit 1; }
  mkdir -p "$HOME/.local/bin"
  curl -fsSL "$url" -o "$HOME/.local/bin/gsso"
  chmod 755 "$HOME/.local/bin/gsso"
  case "$(basename "$SHELL")" in
    zsh) sh=zsh rc="$HOME/.zshrc" ;;
    bash) sh=bash rc="$HOME/.bashrc" ;;
    *) echo "Installed. The shell hook supports bash and zsh, not $SHELL." >&2; exit 0 ;;
  esac
  touch "$rc"
  case ":$PATH:" in
    *":$HOME/.local/bin:"*) ;;
    *) grep -qF '$HOME/.local/bin' "$rc" || echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$rc" ;;
  esac
  grep -qF 'gsso shell-init' "$rc" || echo "eval \"\$(gsso shell-init $sh)\"" >> "$rc"
  echo "Installed $("$HOME/.local/bin/gsso" --version) to ~/.local/bin. Open a new terminal."
)
```

Installs to `~/.local/bin/gsso`. Adds `~/.local/bin` to `PATH` (if it is not
already) and the hook line to `~/.zshrc` or `~/.bashrc`.

</details>

<details>
<summary>Windows (PowerShell)</summary>

```powershell
& {
  $ErrorActionPreference = 'Stop'
  $ProgressPreference = 'SilentlyContinue'
  $arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
  $asset = (Invoke-RestMethod 'https://api.github.com/repos/twentyfourg/24g-aws-sso/releases?per_page=100').assets |
    Where-Object name -eq "gsso-windows-$arch.exe" | Select-Object -First 1
  if (-not $asset) { throw "No gsso release found for windows-$arch" }
  $dir = Join-Path $env:LOCALAPPDATA 'Programs\gsso'
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
  Invoke-WebRequest -UseBasicParsing $asset.browser_download_url -OutFile (Join-Path $dir 'gsso.exe')
  $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  if (($userPath -split ';') -notcontains $dir) {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$dir".TrimStart(';'), 'User')
  }
  if (-not (Test-Path $PROFILE)) { New-Item -ItemType File -Force -Path $PROFILE | Out-Null }
  $line = 'gsso shell-init powershell | Out-String | Invoke-Expression'
  if (-not (Select-String -LiteralPath $PROFILE -SimpleMatch $line -Quiet)) {
    Add-Content -LiteralPath $PROFILE -Value $line
  }
  Write-Host "Installed $(& (Join-Path $dir 'gsso.exe') --version) to $dir. Open a new terminal."
}
```

Installs to `%LOCALAPPDATA%\Programs\gsso\gsso.exe`. Adds that directory to
the user `PATH` and the hook line to `$PROFILE`. If the new terminal reports
that running scripts is disabled, PowerShell's execution policy is blocking
`$PROFILE`; `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned` allows it.

</details>

The shell hook is required. `gsso switch` cannot export `AWS_PROFILE` into the
shell that started it, so every new shell must load the hook. To register it by
hand instead:

```bash
eval "$(gsso shell-init bash)"                         # ~/.bashrc
eval "$(gsso shell-init zsh)"                          # ~/.zshrc
gsso shell-init powershell | Out-String | Invoke-Expression   # $PROFILE
```

Open a new terminal after installing and `gsso switch PROFILE` sets
`AWS_PROFILE`. A path such as `./gsso switch` skips the function; the same hook
still applies the profile before the next prompt.

Without the hook, `gsso switch` in a terminal exits and leaves `AWS_PROFILE`
unchanged.

Menus need an interactive terminal. Passing the names as arguments does not.

### Build from source

```bash
cd gsso/go
go build -o gsso ./cmd/gsso                              # macOS/Linux
GOOS=windows GOARCH=amd64 go build -o gsso.exe ./cmd/gsso
install -m 755 gsso "$HOME/.local/bin/gsso"
```

Then register the shell hook as above.

## Everyday use

```bash
gsso login
gsso list
gsso configure
gsso switch acme-read
gsso whoami
```

SSO sessions last about 12 hours. `gsso login` is `aws sso login --sso-session 24g`.
Extra arguments are forwarded, for example `gsso login --use-device-code`.

## Commands

### `gsso login`

Renew the 24G SSO session. Does not create or change profiles.

### `gsso list`

Print managed profiles as a table: account, account id, profile, role, and
which profile this shell will use. `*` is `AWS_PROFILE`, or the profile that
matches `[default]` when that variable is unset. `default` marks a profile
whose account and role match `[default]`. Unmanaged config profiles are listed
separately and are not modified.

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

Set `AWS_PROFILE` in the current shell. With the shell hook loaded, `gsso
switch` evals the export immediately. A path invocation queues the profile
and the hook exports it before the next prompt. Without the hook, a terminal
run tells you to load `shell-init` and leaves the variable unchanged. Stdout
is still only the export line when something is capturing it
(`eval "$(gsso switch PROFILE)"`).

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

Interactive menus use a checkbox or single-select. Type to filter. Space
toggles, Enter confirms, Esc cancels.

`gsso mcp add` with no arguments is two steps: choose the profiles, then (if
more than one is selected) choose which of those is the default.

Without a terminal, pass the names as arguments.

## Cache and rollback

- Portal cache: `~/.aws/gsso/portal-cache.json`. `list --remote`, `add`, and
  `configure` reuse it until `--hard-refresh`.
- SSO rollback points: `~/.aws/.24g-setup-backups/<timestamp>/`
- MCP rollback points: `~/.aws/.24g-mcp-setup-backups/<timestamp>/`

A rollback point is a copy of the files about to change. Restore by copying
those files back. `gsso` does not delete backups.

## Tests

```bash
cd gsso/go
go test ./...
```
