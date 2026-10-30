package gsso

import (
	"strings"
	"testing"
)

func runCaptured(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var code int
	stdout, stderr, _ := captureStd(t, func() error {
		code = Run(argv)
		return nil
	})
	return code, stdout, stderr
}

func TestRunShowsUsageAfterParseErrors(t *testing.T) {
	for _, tc := range []struct {
		argv   []string
		prefix string
		usage  string
	}{
		{[]string{"zzzzzzzz"}, "gsso: unknown command \"zzzzzzzz\" for \"gsso\"\n\nUsage:", "Usage:\n  gsso [command]"},
		{[]string{"asdf"}, "gsso: unknown command \"asdf\" for \"gsso\"\n\nDid you mean this?\n\tadd\n\nUsage:", "-v, --version"},
		{[]string{"list", "--nope"}, "gsso: unknown flag: --nope\n\nUsage:", "Usage:\n  gsso list"},
		{[]string{"switch", "a", "b"}, "gsso: accepts at most 1 arg(s), received 2\n\nUsage:", "Usage:\n  gsso switch [PROFILE]"},
	} {
		code, stdout, stderr := runCaptured(t, tc.argv...)
		if code != ExFail {
			t.Fatalf("%v: code %d", tc.argv, code)
		}
		if stdout != "" {
			t.Fatalf("%v: stdout = %q", tc.argv, stdout)
		}
		if !strings.HasPrefix(stderr, tc.prefix) || !strings.Contains(stderr, tc.usage) {
			t.Fatalf("%v: stderr = %q", tc.argv, stderr)
		}
	}
}

func TestRunOmitsUsageForCommandErrors(t *testing.T) {
	original := RequireAWS
	RequireAWS = func() error { return NewError("aws not found") }
	t.Cleanup(func() { RequireAWS = original })

	code, _, stderr := runCaptured(t, "switch", "k6")
	if code != ExFail {
		t.Fatalf("code %d", code)
	}
	if stderr != "gsso: aws not found\n" {
		t.Fatalf("stderr = %q", stderr)
	}
}
