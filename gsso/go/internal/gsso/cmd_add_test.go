package gsso

import "testing"

// withoutAWSCLI stubs RequireAWS for a test, restoring it afterward, so
// these tests don't depend on the AWS CLI being installed on the machine
// running `go test`.
func withoutAWSCLI(t *testing.T) {
	t.Helper()
	original := RequireAWS
	RequireAWS = func() error { return nil }
	t.Cleanup(func() { RequireAWS = original })
}

func TestAddAliasNamesASingleProfile(t *testing.T) {
	withoutAWSCLI(t)
	paths := NewPaths(t.TempDir())
	if err := WritePortalCache(paths, []Pair{
		{AccountID: "111122223333", AccountName: "Acme", RoleName: "deploy"},
	}); err != nil {
		t.Fatal(err)
	}

	app := NewApp(paths)
	if err := app.runAdd([]string{"Acme/deploy"}, "acme-production", false); err != nil {
		t.Fatal(err)
	}

	cf, err := LoadConfigFile(paths.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	profiles := ReadConfigProfiles(cf)
	if len(profiles) != 1 {
		t.Fatalf("got %d profiles, want 1: %+v", len(profiles), profiles)
	}
	p := profiles[0]
	if p.Name != "acme-production" || p.AccountID != "111122223333" || p.RoleName != "deploy" {
		t.Errorf("got %+v", p)
	}
}

func TestAddAliasRequiresExactlyOneTarget(t *testing.T) {
	withoutAWSCLI(t)
	paths := NewPaths(t.TempDir())
	app := NewApp(paths)
	err := app.runAdd([]string{"Acme/deploy", "Acme/read"}, "acme", false)
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestAddSkipsRolesAlreadyConfigured(t *testing.T) {
	withoutAWSCLI(t)
	paths := NewPaths(t.TempDir())
	if err := WritePortalCache(paths, []Pair{
		{AccountID: "111122223333", AccountName: "Acme", RoleName: "deploy"},
	}); err != nil {
		t.Fatal(err)
	}
	app := NewApp(paths)
	if err := app.runAdd(nil, "", false); err == nil {
		// no targets and not a TTY: ChooseMultiple should refuse, since there
		// is exactly one candidate but no terminal to pick it interactively.
		t.Fatal("expected a TTY error when no targets are given outside a terminal")
	}
}
