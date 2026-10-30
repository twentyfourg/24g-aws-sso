package gsso

import "testing"

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"Acme Corp":    "Acme-Corp",
		"  spaced  ":   "spaced",
		"a--b":         "a-b",
		"weird!@#name": "weirdname",
	}
	for input, want := range cases {
		if got := Sanitize(input); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestProposedName(t *testing.T) {
	shared := map[string]bool{"Shared": true}
	cases := []struct {
		account, role, want string
	}{
		{"Acme", "AdministratorAccess", "Acme-AdministratorAccess"}, // system role
		{"Acme", "2772-1-lxp-base", "2772-1-lxp-base"},              // unique, not system-cased
		{"Acme", "Shared", "Acme-Shared"},                           // shared across accounts
	}
	for _, c := range cases {
		if got := ProposedName(c.account, c.role, shared); got != c.want {
			t.Errorf("ProposedName(%q, %q) = %q, want %q", c.account, c.role, got, c.want)
		}
	}
}

func TestValidateProfileName(t *testing.T) {
	if err := ValidateProfileName("acme-read.1_2"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	for _, bad := range []string{"", "-leading-dash", "has space", "has/slash"} {
		if err := ValidateProfileName(bad); err == nil {
			t.Errorf("expected an error for %q", bad)
		}
	}
}

func TestSharedRoleNames(t *testing.T) {
	pairs := []Pair{
		{AccountID: "1", RoleName: "ReadOnly"},
		{AccountID: "2", RoleName: "ReadOnly"},
		{AccountID: "1", RoleName: "Unique"},
	}
	shared := SharedRoleNames(pairs)
	if !shared["ReadOnly"] || shared["Unique"] {
		t.Errorf("got %v", shared)
	}
}
