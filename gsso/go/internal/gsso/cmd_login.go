package gsso

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// loginCmd renews the SSO session. It deliberately does not touch profiles:
// discovering newly granted roles is `list --remote`, creating profiles for
// them is `add`.
//
// Flags are forwarded verbatim to `aws sso login`, so flag parsing is
// disabled here rather than declared through Cobra, which would reject
// anything it does not already know (e.g. --use-device-code).
func (a *App) loginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "login [-- aws-sso-login-flags]",
		Short:              "renew the 24G SSO session",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "-h" || arg == "--help" {
					return cmd.Help()
				}
			}
			if err := RequireAWS(); err != nil {
				return err
			}
			full := append([]string{"sso", "login", "--sso-session", SessionName}, args...)
			sub := exec.Command("aws", full...)
			sub.Stdin = os.Stdin
			sub.Stdout = os.Stdout
			sub.Stderr = os.Stderr
			err := sub.Run()
			// Mirror `return subprocess.run(command).returncode`: the exit
			// code is `aws`'s, not wrapped in a "gsso:" error line, since
			// `aws` already explained itself on stderr.
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			return err
		},
	}
	return cmd
}
