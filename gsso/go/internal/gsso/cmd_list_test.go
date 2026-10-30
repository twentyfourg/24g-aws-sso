package gsso

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestListLocalRendersATable(t *testing.T) {
	paths := NewPaths(t.TempDir())
	writeFile(t, paths.ConfigPath, ""+
		"[profile auth-24g]\n"+
		"# managed-by: 24g-sso-setup\n"+
		"# account-name: auth-24g\n"+
		"sso_session = 24g\n"+
		"sso_account_id = 259542831251\n"+
		"sso_role_name = AdministratorAccess\n"+
		"region = us-east-1\n"+
		"output = json\n"+
		"\n"+
		"[profile route53-read]\n"+
		"# managed-by: 24g-sso-setup\n"+
		"# account-name: auth-24g\n"+
		"sso_session = 24g\n"+
		"sso_account_id = 259542831251\n"+
		"sso_role_name = route53-read\n"+
		"region = us-east-1\n"+
		"output = json\n"+
		"\n"+
		"[profile elam-24g]\n"+
		"region = us-east-1\n")
	t.Setenv("AWS_PROFILE", "route53-read")

	var buf bytes.Buffer
	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan struct{})
	go func() {
		_, _ = buf.ReadFrom(r)
		close(done)
	}()

	err = NewApp(paths).listLocal()
	w.Close()
	os.Stdout = stdout
	<-done
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	want := "" +
		"ACCOUNT   ACCOUNT ID    PROFILE       ROLE                 ACTIVE\n" +
		"auth-24g  259542831251  auth-24g      AdministratorAccess  \n" +
		"auth-24g  259542831251  route53-read  route53-read         *\n" +
		"\n" +
		"Unmanaged profiles (" + paths.ConfigPath + ")\n" +
		"PROFILE   ACTIVE\n" +
		"elam-24g  \n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "auth-24g (259542831251)") {
		t.Fatal("grouped heading leaked into table output")
	}
	if strings.HasPrefix(got, " ") {
		t.Fatal("table is indented")
	}
}

func TestListLocalMarksSessionAndDefault(t *testing.T) {
	paths := NewPaths(t.TempDir())
	writeFile(t, paths.ConfigPath, ""+
		"[default]\n"+
		"sso_session = 24g\n"+
		"sso_account_id = 259542831251\n"+
		"sso_role_name = route53-read\n"+
		"region = us-east-1\n"+
		"\n"+
		"[profile auth-24g]\n"+
		"# managed-by: 24g-sso-setup\n"+
		"# account-name: auth-24g\n"+
		"sso_account_id = 259542831251\n"+
		"sso_role_name = AdministratorAccess\n"+
		"\n"+
		"[profile route53-read]\n"+
		"# managed-by: 24g-sso-setup\n"+
		"# account-name: auth-24g\n"+
		"sso_account_id = 259542831251\n"+
		"sso_role_name = route53-read\n")
	t.Setenv("AWS_PROFILE", "auth-24g")

	got := captureStdout(t, func() error { return NewApp(paths).listLocal() })
	want := "" +
		"ACCOUNT   ACCOUNT ID    PROFILE       ROLE                 ACTIVE\n" +
		"auth-24g  259542831251  auth-24g      AdministratorAccess  *\n" +
		"auth-24g  259542831251  route53-read  route53-read         default\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestListLocalReadsDefaultFromCredentials(t *testing.T) {
	paths := NewPaths(t.TempDir())
	writeFile(t, paths.ConfigPath, ""+
		"[default]\n"+
		"region = us-east-1\n"+
		"\n"+
		"[profile auth-24g]\n"+
		"# managed-by: 24g-sso-setup\n"+
		"# account-name: auth-24g\n"+
		"sso_account_id = 259542831251\n"+
		"sso_role_name = AdministratorAccess\n")
	writeFile(t, paths.CredentialsPath, ""+
		"[default]\n"+
		"sso_session = 24g\n"+
		"sso_account_id = 259542831251\n"+
		"sso_role_name = AdministratorAccess\n"+
		"region = us-east-1\n")
	t.Setenv("AWS_PROFILE", "")

	got := captureStdout(t, func() error { return NewApp(paths).listLocal() })
	if !strings.Contains(got, "AdministratorAccess  * default") {
		t.Fatalf("credentials [default] was not marked:\n%s", got)
	}
}

func TestActiveLabel(t *testing.T) {
	label, on := activeLabel("auth-24g", "259542831251", "route53-read", "auth-24g", "259542831251", "AdministratorAccess")
	if label != "*" || !on {
		t.Fatalf("session = %q %v", label, on)
	}
	label, on = activeLabel("auth-24g", "259542831251", "route53-read", "route53-read", "259542831251", "route53-read")
	if label != "default" || on {
		t.Fatalf("default-only = %q %v", label, on)
	}
	label, on = activeLabel("", "259542831251", "route53-read", "route53-read", "259542831251", "route53-read")
	if label != "* default" || !on {
		t.Fatalf("unset session falls through to default = %q %v", label, on)
	}
	label, on = activeLabel("auth-24g", "259542831251", "AdministratorAccess", "auth-24g", "259542831251", "AdministratorAccess")
	if label != "* default" || !on {
		t.Fatalf("session and default = %q %v", label, on)
	}
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	var buf bytes.Buffer
	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan struct{})
	go func() {
		_, _ = buf.ReadFrom(r)
		close(done)
	}()
	err = fn()
	w.Close()
	os.Stdout = stdout
	<-done
	if err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
