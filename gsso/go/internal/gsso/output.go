package gsso

import (
	"fmt"
	"os"
)

// Err writes a line to stderr. stdout is reserved for `switch`'s export line
// so it can be eval'd directly.
func Err(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
