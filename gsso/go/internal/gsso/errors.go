package gsso

import "fmt"

// Exit codes.
const (
	ExOK   = 0
	ExFail = 1
	ExDep  = 2
	ExTTY  = 3
	ExAuth = 4
)

// Error is a failure with a known cause. The CLI prints it without a
// stack trace and exits with Code.
type Error struct {
	Message string
	Code    int
	Fix     string
}

func (e *Error) Error() string {
	return e.Message
}

// NewError builds a gsso.Error with the default EX_FAIL exit code.
func NewError(format string, args ...any) *Error {
	return &Error{Message: fmt.Sprintf(format, args...), Code: ExFail}
}

// NewErrorWithFix builds a gsso.Error carrying a suggested fix and exit code.
func NewErrorWithFix(code int, fix string, format string, args ...any) *Error {
	return &Error{Message: fmt.Sprintf(format, args...), Code: code, Fix: fix}
}
