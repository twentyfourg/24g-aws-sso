package gsso

import (
	"fmt"
	"os"
)

// Run builds the command tree, executes it against argv, and returns the
// process exit code. Errors are printed as "gsso: message" (plus an
// optional fix line) rather than a stack trace, since a frame could hold an
// access token.
func Run(argv []string) int {
	app := NewApp(DefaultPaths())
	root := app.RootCommand()
	root.SetArgs(argv)
	err := root.Execute()
	if err == nil {
		return ExOK
	}
	var gerr *Error
	if e, ok := err.(*Error); ok {
		gerr = e
	} else {
		gerr = &Error{Message: fmt.Sprintf("unexpected %T: %v", err, err), Code: ExFail}
	}
	if gerr.Message != "" {
		fmt.Fprintf(os.Stderr, "gsso: %s\n", gerr.Message)
	}
	if gerr.Fix != "" {
		fmt.Fprintf(os.Stderr, "  fix: %s\n", gerr.Fix)
	}
	return gerr.Code
}
