package gsso

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSwitchPrintsExportWhenStdoutIsCaptured(t *testing.T) {
	withoutAWSCLI(t)
	stubStdoutTerminal(t, false)
	t.Setenv(shellHookEnv, "1")
	paths := switchTestPaths(t)

	stdout, stderr, err := captureStd(t, func() error {
		return NewApp(paths).runSwitch("k6", false, "posix")
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "export AWS_PROFILE=k6\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "AWS_PROFILE=k6\n" {
		t.Fatalf("stderr = %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(paths.AWSDir, "gsso", "sessions")); !os.IsNotExist(err) {
		t.Fatalf("captured switch wrote a session file: %v", err)
	}
}

func TestSwitchWithHookQueuesProfileForParentShell(t *testing.T) {
	withoutAWSCLI(t)
	stubStdoutTerminal(t, true)
	t.Setenv(shellHookEnv, "1")
	paths := switchTestPaths(t)

	stdout, stderr, err := captureStd(t, func() error {
		return NewApp(paths).runSwitch("k6", false, "posix")
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "AWS_PROFILE=k6\n" {
		t.Fatalf("stderr = %q", stderr)
	}
	got, err := os.ReadFile(filepath.Join(paths.AWSDir, "gsso", "sessions", strconv.Itoa(os.Getppid())))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "k6\n" {
		t.Fatalf("session file %q", got)
	}
}

func TestSwitchWithoutHookRefusesToPretend(t *testing.T) {
	withoutAWSCLI(t)
	stubStdoutTerminal(t, true)
	t.Setenv(shellHookEnv, "")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("SHELL", "/bin/bash")
	paths := switchTestPaths(t)

	stdout, stderr, err := captureStd(t, func() error {
		return NewApp(paths).runSwitch("k6", false, "posix")
	})
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout %q stderr %q", stdout, stderr)
	}
	gerr, ok := err.(*Error)
	if !ok {
		t.Fatalf("got %T %v", err, err)
	}
	if gerr.Code != ExFail {
		t.Fatalf("code %d", gerr.Code)
	}
	if !strings.Contains(gerr.Message, "AWS_PROFILE is still unset") {
		t.Fatalf("message %q", gerr.Message)
	}
	if !strings.Contains(gerr.Fix, "shell-init bash") {
		t.Fatalf("fix %q", gerr.Fix)
	}
	if _, statErr := os.Stat(filepath.Join(paths.AWSDir, "gsso", "sessions")); !os.IsNotExist(statErr) {
		t.Fatalf("hook-less switch wrote a session file: %v", statErr)
	}
}

func TestShellInitFixUsesShell(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	if got := shellInitFix("posix"); !strings.Contains(got, "shell-init zsh") {
		t.Fatalf("got %q", got)
	}
	if got := shellInitFix("powershell"); !strings.Contains(got, "shell-init powershell") {
		t.Fatalf("got %q", got)
	}
}

func TestShellInitWrapperExportsHook(t *testing.T) {
	for _, src := range []string{bashZshWrapper, powershellWrapper} {
		if !strings.Contains(src, "GSSO_SHELL_HOOK") || !strings.Contains(src, "__gsso_apply_profile") {
			t.Fatalf("wrapper missing hook:\n%s", src)
		}
	}
	if !strings.Contains(bashZshWrapper, "PROMPT_COMMAND") {
		t.Fatal("bash wrapper missing PROMPT_COMMAND")
	}
}

func switchTestPaths(t *testing.T) Paths {
	t.Helper()
	paths := NewPaths(t.TempDir())
	writeFile(t, paths.ConfigPath, ""+
		"[profile k6]\n"+
		"sso_session = 24g\n"+
		"sso_account_id = 033632814797\n"+
		"sso_role_name = k6\n"+
		"region = us-east-1\n")
	return paths
}

func stubStdoutTerminal(t *testing.T, terminal bool) {
	t.Helper()
	original := stdoutIsTerminal
	stdoutIsTerminal = func() bool { return terminal }
	t.Cleanup(func() { stdoutIsTerminal = original })
}

func captureStd(t *testing.T, fn func() error) (string, string, error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	t.Cleanup(func() {
		os.Stdout, os.Stderr = oldOut, oldErr
	})

	outDone := make(chan struct{})
	go func() {
		_, _ = outBuf.ReadFrom(outR)
		close(outDone)
	}()
	errDone := make(chan struct{})
	go func() {
		_, _ = errBuf.ReadFrom(errR)
		close(errDone)
	}()

	runErr := fn()
	outW.Close()
	errW.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	<-outDone
	<-errDone
	return outBuf.String(), errBuf.String(), runErr
}
