#!/usr/bin/env bash
# Installs or updates gsso to the latest release in ~/.local/bin and registers the shell hook.
#   curl -fsSL https://raw.githubusercontent.com/twentyfourg/24g-aws-sso/main/gsso/install.sh | bash
set -eu

tmp=""
trap 'rm -f "$tmp"' EXIT

main() {
  local os arch url sh rc old="" new found
  local bin="$HOME/.local/bin/gsso"
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) echo "Unsupported CPU: $(uname -m)" >&2; exit 1 ;;
  esac

  url=$(curl -fsSL "https://api.github.com/repos/twentyfourg/24g-aws-sso/releases?per_page=100" |
    grep -o "https://[^\"]*/gsso-$os-$arch\"" | sed -n '1s/"$//p')
  [ -n "$url" ] || { echo "No gsso release found for $os-$arch" >&2; exit 1; }

  if [ -x "$bin" ]; then
    old=$("$bin" --version 2>/dev/null || true)
    old=${old##* }
  fi

  # Download beside the binary and rename over it, so a failed download leaves
  # the old binary intact and a running gsso does not block the update.
  mkdir -p "$HOME/.local/bin"
  tmp=$(mktemp "$HOME/.local/bin/.gsso.XXXXXX")
  curl -fsSL "$url" -o "$tmp"
  chmod 755 "$tmp"
  mv -f "$tmp" "$bin"
  new=$("$bin" --version)
  new=${new##* }

  found=$(command -v gsso || true)
  if [ -n "$found" ] && [ "$found" != "$bin" ]; then
    echo "Warning: $found comes before $bin on PATH, so this shell still runs that one." >&2
  fi

  case "$(basename "$SHELL")" in
    zsh) sh=zsh rc="$HOME/.zshrc" ;;
    bash) sh=bash rc="$HOME/.bashrc" ;;
    *) echo "Installed. The shell hook supports bash and zsh, not $SHELL." >&2; exit 0 ;;
  esac
  touch "$rc"
  # Skip comment lines so a commented mention of the path or hook is not
  # treated as configuration. Any amount of leading whitespace still counts.
  rc_without_comments() { grep -v '^[[:space:]]*#' "$rc"; }
  case ":$PATH:" in
    *":$HOME/.local/bin:"*) ;;
    *) rc_without_comments | grep -qF '.local/bin' || echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$rc" ;;
  esac
  rc_without_comments | grep -qF 'gsso shell-init' || echo "eval \"\$(gsso shell-init $sh)\"" >> "$rc"

  if [ -z "$old" ]; then
    echo "Installed gsso $new to ~/.local/bin. Open a new terminal."
  elif [ "$old" = "$new" ]; then
    echo "gsso $new is already the latest release."
  else
    echo "Updated gsso $old -> $new. Open a new terminal to load the updated hook."
  fi
}

main
