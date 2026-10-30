package gsso

import (
	"os"

	"github.com/spf13/cobra"
)

const bashZshWrapper = `# gsso switch cannot export AWS_PROFILE into the shell that started it.
# The function below evals the export when the command name is gsso.
# A path such as ./gsso skips that function, so the binary writes
# ~/.aws/gsso/sessions/<shell-pid> and this hook exports it here.
__gsso_apply_profile() {
  local __gsso_file="$HOME/.aws/gsso/sessions/$$"
  [ -f "$__gsso_file" ] || return 0
  local __gsso_profile
  IFS= read -r __gsso_profile < "$__gsso_file" || true
  rm -f "$__gsso_file"
  if [ -n "$__gsso_profile" ]; then
    export AWS_PROFILE="$__gsso_profile"
  fi
  return 0
}

if [ -n "${ZSH_VERSION:-}" ]; then
  if [ "${GSSO_SHELL_HOOK:-}" != "1" ]; then
    autoload -Uz add-zsh-hook
    add-zsh-hook precmd __gsso_apply_profile
  fi
else
  case ";${PROMPT_COMMAND:-};" in
    *";__gsso_apply_profile;"*) ;;
    *) PROMPT_COMMAND="__gsso_apply_profile${PROMPT_COMMAND:+;$PROMPT_COMMAND}" ;;
  esac
fi
export GSSO_SHELL_HOOK=1

gsso() {
  case "$1" in
    switch)
      shift
      __gsso_out="$(command gsso switch --shell posix "$@")" || return $?
      if [ -n "$__gsso_out" ]; then
        eval "$__gsso_out"
      fi
      unset __gsso_out
      ;;
    *)
      command gsso "$@"
      ;;
  esac
}
`

// The zsh completion script ends with compdef, which only exists after
// compinit. A default macOS zsh never runs compinit.
const zshCompinitGuard = `(( $+functions[compdef] )) || { autoload -Uz compinit && compinit; }
`

const powershellWrapper = `function global:__gsso_apply_profile {
    $file = Join-Path $HOME ".aws/gsso/sessions/$PID"
    if (-not (Test-Path -LiteralPath $file)) { return }
    $name = (Get-Content -LiteralPath $file -Raw).Trim()
    Remove-Item -LiteralPath $file -Force -ErrorAction SilentlyContinue
    if ($name) { $env:AWS_PROFILE = $name }
}

if (-not $env:GSSO_SHELL_HOOK) {
    $global:__gsso_previous_prompt = $function:prompt
    function global:prompt {
        __gsso_apply_profile
        if ($global:__gsso_previous_prompt) { & $global:__gsso_previous_prompt }
    }
}
$env:GSSO_SHELL_HOOK = '1'

function gsso {
    param([Parameter(ValueFromRemainingArguments=$true)] [string[]]$Args)
    if ($Args.Count -gt 0 -and $Args[0] -eq 'switch') {
        $rest = @()
        if ($Args.Count -gt 1) { $rest = $Args[1..($Args.Count - 1)] }
        $out = & gsso.exe switch --shell powershell @rest
        if ($LASTEXITCODE -ne 0) { return $LASTEXITCODE }
        if ($out) { $out | ForEach-Object { Invoke-Expression $_ } }
    } else {
        & gsso.exe @Args
    }
}
`

// shellInitCmd prints a shell function wrapper plus tab completion for eval,
// e.g. `eval "$(gsso shell-init bash)"`. The wrapper exports AWS_PROFILE in
// the current shell. `gsso switch` evals the assignment immediately. A path
// such as `./gsso switch` skips the function, so the hook applies the
// profile file the binary writes before the next prompt.
func (a *App) shellInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "shell-init {bash|zsh|powershell}",
		Short:     "print the shell wrapper and tab completion for eval",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				os.Stdout.WriteString(bashZshWrapper)
				return root.GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				os.Stdout.WriteString(bashZshWrapper)
				os.Stdout.WriteString(zshCompinitGuard)
				return root.GenZshCompletion(os.Stdout)
			case "powershell":
				os.Stdout.WriteString(powershellWrapper)
				return root.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return NewError("unsupported shell: %s", args[0])
			}
		},
	}
	return cmd
}
