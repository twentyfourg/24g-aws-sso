#!/usr/bin/env bash
# Installs the latest gsso release to ~/.local/bin and registers the shell hook.
#   curl -fsSL https://raw.githubusercontent.com/twentyfourg/24g-aws-sso/main/gsso/install.sh | bash
set -eu

main() {
  local os arch url sh rc
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
}

main
