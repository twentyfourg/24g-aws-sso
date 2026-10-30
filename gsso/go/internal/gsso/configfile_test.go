package gsso

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigFileRoundTripsCommentsAndSections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	original := "[profile hand-written]\nregion = us-west-2\n"
	writeFile(t, path, original)

	cf, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cf.Render(); got != original {
		t.Fatalf("Render() changed an untouched file:\n%q\nwant:\n%q", got, original)
	}
	if cf.Section("profile missing") != nil {
		t.Error("expected no section")
	}
	block := cf.Section("profile hand-written")
	if block == nil {
		t.Fatal("expected section")
	}
	if v, ok := block.Get("region"); !ok || v != "us-west-2" {
		t.Errorf("Get(region) = %q, %v", v, ok)
	}
}

func TestWriteProfilesAppendsEverySectionInOneSave(t *testing.T) {
	paths := NewPaths(t.TempDir())
	writeFile(t, paths.ConfigPath, "[profile hand-written]\nregion = us-west-2\n")

	entries := []ProfileEntry{
		{Name: "acme-admin", AccountID: "111122223333", AccountName: "Acme", RoleName: "AdministratorAccess"},
		{Name: "beta-admin", AccountID: "444455556666", AccountName: "Beta", RoleName: "AdministratorAccess"},
	}
	if err := WriteProfiles(paths, entries, "us-east-1"); err != nil {
		t.Fatal(err)
	}

	cf, err := LoadConfigFile(paths.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	managed := map[string]bool{}
	for _, p := range ReadConfigProfiles(cf) {
		if p.Managed {
			managed[p.Name] = true
		}
	}
	if !managed["acme-admin"] || !managed["beta-admin"] || len(managed) != 2 {
		t.Fatalf("got managed = %v", managed)
	}

	want := "[profile hand-written]\n" +
		"region = us-west-2\n" +
		"\n" +
		"[profile acme-admin]\n" +
		"# managed-by: 24g-sso-setup\n" +
		"# account-name: Acme\n" +
		"sso_session = 24g\n" +
		"sso_account_id = 111122223333\n" +
		"sso_role_name = AdministratorAccess\n" +
		"region = us-east-1\n" +
		"output = json\n" +
		"\n" +
		"[profile beta-admin]\n" +
		"# managed-by: 24g-sso-setup\n" +
		"# account-name: Beta\n" +
		"sso_session = 24g\n" +
		"sso_account_id = 444455556666\n" +
		"sso_role_name = AdministratorAccess\n" +
		"region = us-east-1\n" +
		"output = json\n"
	got, err := os.ReadFile(paths.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("config mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteProfilesRefusesToShadowAnExistingProfile(t *testing.T) {
	paths := NewPaths(t.TempDir())
	original := "[profile acme-admin]\nregion = us-west-2\n"
	writeFile(t, paths.ConfigPath, original)

	err := WriteProfiles(paths, []ProfileEntry{
		{Name: "acme-admin", AccountID: "111122223333", AccountName: "Acme", RoleName: "Admin"},
	}, "us-east-1")
	if err == nil {
		t.Fatal("expected an error")
	}
	got, _ := os.ReadFile(paths.ConfigPath)
	if string(got) != original {
		t.Errorf("config was modified despite the error:\n%s", got)
	}
}
