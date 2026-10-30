package gsso

import (
	"os/exec"
	"strings"
)

// RequireAWS checks that AWS CLI v2 is on PATH. Every command that talks to
// AWS calls this first so the error names the real cause instead of a raw
// "executable not found". It is a variable, not a plain function, so tests
// can stub it out on machines without the AWS CLI installed.
var RequireAWS = requireAWSImpl

func requireAWSImpl() error {
	path, err := exec.LookPath("aws")
	if err != nil || path == "" {
		return NewErrorWithFix(ExDep, "install AWS CLI v2, then re-run", "the AWS CLI is not on PATH")
	}
	out, _ := exec.Command("aws", "--version").Output()
	version := strings.TrimSpace(string(out))
	if !strings.HasPrefix(version, "aws-cli/2") {
		label := version
		if label == "" {
			label = "unknown"
		}
		return NewErrorWithFix(ExDep, "sso-session configuration needs v2; upgrade and re-run",
			"AWS CLI v2 is required, found: %s", label)
	}
	return nil
}

// RunAWS runs `aws <args> --no-cli-pager` and returns stdout. secret, if
// non-empty, is redacted from any error text and from the command shown in
// the error, because the SSO access token is passed to some commands as an
// argument and would otherwise leak through a failure message.
func RunAWS(args []string, secret string) (string, error) {
	fullArgs := append(append([]string{}, args...), "--no-cli-pager")
	cmd := exec.Command("aws", fullArgs...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if runErr != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if secret != "" {
			detail = strings.ReplaceAll(detail, secret, "<redacted>")
		}
		return "", NewError("aws %s failed: %s", strings.Join(redactArgs(args, secret), " "), detail)
	}
	return stdout.String(), nil
}

func redactArgs(args []string, secret string) []string {
	if secret == "" {
		return args
	}
	out := make([]string, len(args))
	for i, a := range args {
		if a == secret {
			out[i] = "<redacted>"
		} else {
			out[i] = a
		}
	}
	return out
}

// AWSConfigureSet runs `aws configure set key value`.
func AWSConfigureSet(key, value string) error {
	return exec.Command("aws", "configure", "set", key, value).Run()
}
