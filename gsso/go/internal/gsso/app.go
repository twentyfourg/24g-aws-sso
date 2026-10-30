package gsso

import (
	"github.com/spf13/cobra"
)

// App holds the resolved filesystem paths every command operates on. A
// struct (rather than package-level state) lets tests point gsso at a
// temporary HOME.
type App struct {
	Paths Paths
}

// NewApp builds an App.
func NewApp(paths Paths) *App {
	return &App{Paths: paths}
}

// RootCommand builds the `gsso` command tree.
func (a *App) RootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "gsso",
		Short:         "24G AWS SSO profile helper",
		Version:       Version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(
		a.loginCmd(),
		a.listCmd(),
		a.addCmd(),
		a.configureCmd(),
		a.removeCmd(),
		a.switchCmd(),
		a.whoamiCmd(),
		a.mcpCmd(),
		a.shellInitCmd(),
	)
	return root
}
