package gsso

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Run builds the command tree, executes it against argv, and returns the
// process exit code. Errors are printed as "gsso: message" (plus an
// optional fix line) rather than a stack trace, since a frame could hold an
// access token.
func Run(argv []string) int {
	app := NewApp(DefaultPaths())
	root := app.RootCommand()
	root.SetArgs(argv)
	// Cobra runs this only after the command, flags, and arguments parse, so
	// an error without it is a usage mistake and gets the command's usage.
	parsed := false
	root.PersistentPreRun = func(*cobra.Command, []string) { parsed = true }
	cmd, err := root.ExecuteC()
	if err == nil {
		return ExOK
	}
	var gerr *Error
	if e, ok := err.(*Error); ok {
		gerr = e
	} else {
		gerr = &Error{Message: err.Error(), Code: ExFail}
	}
	if gerr.Message != "" {
		fmt.Fprintf(os.Stderr, "gsso: %s\n", strings.TrimRight(gerr.Message, "\n"))
	}
	if gerr.Fix != "" {
		fmt.Fprintf(os.Stderr, "  fix: %s\n", gerr.Fix)
	}
	if !parsed {
		cmd.InitDefaultHelpFlag()
		cmd.InitDefaultVersionFlag()
		fmt.Fprintf(os.Stderr, "\n%s", cmd.UsageString())
	}
	return gerr.Code
}
