package gsso

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	xterm "github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

// shellHookEnv is set by `gsso shell-init`. When it is "1", this process's
// parent shell will export a profile written to ~/.aws/gsso/sessions/<pid>.
const shellHookEnv = "GSSO_SHELL_HOOK"

// stdoutIsTerminal reports whether switch's export line would be displayed
// rather than captured. The shell wrapper and `eval "$(gsso switch ...)"`
// capture stdout through a pipe, which is not a terminal. A direct
// `./gsso switch` writes the export to the terminal, where the shell never
// executes it, so AWS_PROFILE in the parent shell stays unchanged.
var stdoutIsTerminal = func() bool {
	return xterm.IsTerminal(os.Stdout.Fd())
}

func (a *App) switchCmd() *cobra.Command {
	var setDefault bool
	var shell string
	cmd := &cobra.Command{
		Use:   "switch [PROFILE]",
		Short: "set AWS_PROFILE for this shell",
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return a.completionSwitchTargets(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var profile string
			if len(args) == 1 {
				profile = args[0]
			}
			return a.runSwitch(profile, setDefault, shell)
		},
	}
	cmd.Flags().BoolVar(&setDefault, "set-default", false, "also mirror into [default]")
	cmd.Flags().StringVar(&shell, "shell", "posix", "posix or powershell")
	return cmd
}

func (a *App) runSwitch(profile string, setDefault bool, shell string) error {
	if err := RequireAWS(); err != nil {
		return err
	}
	if shell != "posix" && shell != "powershell" {
		return NewError("invalid shell %q: must be posix or powershell", shell)
	}
	known := a.completionSwitchTargets()
	if len(known) == 0 {
		return NewErrorWithFix(ExFail, "run `gsso login`", "no profiles configured")
	}
	target := profile
	if target != "" {
		found := false
		for _, n := range known {
			if n == target {
				found = true
				break
			}
		}
		if !found {
			return NewErrorWithFix(ExFail, "run `gsso list`", "no such profile: %s", target)
		}
	} else {
		chosen, err := ChooseOne("Switch to which profile", known)
		if err != nil {
			return err
		}
		target = chosen
	}

	if setDefault {
		if err := a.setDefaultProfile(target); err != nil {
			return err
		}
	}

	// Captured stdout is the `gsso switch` function: it evals this export in
	// the current shell. A terminal means the function did not wrap us
	// (./gsso, or no wrapper). The shell hook applies a session file instead.
	if !stdoutIsTerminal() {
		if shell == "powershell" {
			fmt.Printf("$env:AWS_PROFILE = '%s'\n", target)
		} else {
			fmt.Printf("export AWS_PROFILE=%s\n", target)
		}
		Err("AWS_PROFILE=%s", target)
		return nil
	}
	if os.Getenv(shellHookEnv) == "1" {
		if err := a.queueSwitchForParent(target); err != nil {
			return err
		}
		Err("AWS_PROFILE=%s", target)
		return nil
	}
	return switchHookMissing(shell, setDefault)
}

// queueSwitchForParent writes the profile for the parent shell's hook.
// The file name is the parent pid. Command substitution (the gsso function)
// is not a terminal, so it never writes here: its parent pid is a subshell
// that exits before the hook could read the file.
func (a *App) queueSwitchForParent(profile string) error {
	if strings.ContainsAny(profile, "\r\n") {
		return NewError("invalid profile name")
	}
	dir := filepath.Join(a.Paths.AWSDir, "gsso", "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := io.WriteString(tmp, profile+"\n"); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, filepath.Join(dir, strconv.Itoa(os.Getppid()))); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// switchHookMissing reports that this shell has not loaded `gsso shell-init`,
// so nothing here can export AWS_PROFILE. When setDefault is set, [default]
// was already updated.
func switchHookMissing(shell string, setDefault bool) error {
	state := "unset"
	if current := os.Getenv("AWS_PROFILE"); current != "" {
		state = current
	}
	fix := shellInitFix(shell)
	if setDefault {
		Err("AWS_PROFILE is still %s in this shell.", state)
		Err("  fix: %s", fix)
		return nil
	}
	return NewErrorWithFix(ExFail, fix,
		"this shell cannot apply AWS_PROFILE until the gsso hook is loaded (AWS_PROFILE is still %s)",
		state)
}

func shellInitFix(shellFlag string) string {
	bin := os.Args[0]
	if bin == "" {
		bin = "gsso"
	}
	name := shellInitTarget(shellFlag)
	if name == "powershell" {
		return shellWord(bin) + " shell-init powershell | Out-String | Invoke-Expression"
	}
	return fmt.Sprintf("eval \"$(%s shell-init %s)\"", shellWord(bin), name)
}

func shellInitTarget(shellFlag string) string {
	if shellFlag == "powershell" {
		return "powershell"
	}
	if filepath.Base(os.Getenv("SHELL")) == "zsh" {
		return "zsh"
	}
	return "bash"
}

func shellWord(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\n\"'\\$`;|&<>(){}[]*?~#") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

func (a *App) setDefaultProfile(name string) error {
	for _, n := range ReadCredentialsProfileNames(a.Paths) {
		if n == "default" {
			Err("")
			Err("%s has its own [default] section with static keys.", a.Paths.CredentialsPath)
			Err("Writing an SSO default into ~/.aws/config changes how plain `aws` behaves:")
			Err("it will attempt SSO and fail when no token exists, instead of falling back")
			Err("to those static keys. Anything relying on them will break once the session")
			Err("expires.")
			ok, err := Confirm("Write the SSO default anyway?")
			if err != nil {
				return err
			}
			if !ok {
				return NewError("left [default] alone")
			}
			break
		}
	}
	cf, err := LoadConfigFile(a.Paths.ConfigPath)
	if err != nil {
		return err
	}
	source := cf.Section("profile " + name)
	if source == nil {
		return NewError("%s is not an SSO profile in %s", name, a.Paths.ConfigPath)
	}
	backup, err := CreateRollbackPoint(a.Paths)
	if err != nil {
		return err
	}
	Err("Rollback point: %s", backup)
	for _, key := range []string{"sso_session", "sso_account_id", "sso_role_name", "region"} {
		if value, ok := source.Get(key); ok && value != "" {
			if err := AWSConfigureSet("default."+key, value); err != nil {
				return err
			}
		}
	}
	Err("[default] now mirrors %s", name)
	return nil
}

func (a *App) completionSwitchTargets() []string {
	cf, err := LoadConfigFile(a.Paths.ConfigPath)
	var names []string
	if err == nil {
		for _, p := range ReadConfigProfiles(cf) {
			names = append(names, p.Name)
		}
	}
	names = append(names, ReadCredentialsProfileNames(a.Paths)...)
	seen := map[string]bool{}
	var out []string
	for _, n := range names {
		if n == "default" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
