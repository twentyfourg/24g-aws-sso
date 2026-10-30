package gsso

import (
	"regexp"
	"strings"
)

var (
	systemRoleRE   = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	profileNameRE  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	sanitizeDropRE = regexp.MustCompile(`[^A-Za-z0-9._-]`)
	sanitizeWSRE   = regexp.MustCompile(`\s+`)
	sanitizeDashRE = regexp.MustCompile(`-{2,}`)
)

// PairKey identifies one (account, role) grant, independent of the local
// profile name chosen for it.
type PairKey struct {
	AccountID string
	RoleName  string
}

// Profile mirrors one `[profile NAME]` section of ~/.aws/config.
type Profile struct {
	Name        string
	AccountID   string
	AccountName string
	RoleName    string
	Managed     bool
}

// ReadConfigProfiles returns every `[profile ...]` section.
func ReadConfigProfiles(cf *ConfigFile) []Profile {
	var out []Profile
	for _, block := range cf.Sections() {
		header := *block.Header
		if !strings.HasPrefix(header, "profile ") {
			continue
		}
		accountID, _ := block.Get("sso_account_id")
		accountName, _ := block.CommentValue(AccountNameKey)
		roleName, _ := block.Get("sso_role_name")
		out = append(out, Profile{
			Name:        strings.TrimSpace(strings.TrimPrefix(header, "profile ")),
			AccountID:   accountID,
			AccountName: accountName,
			RoleName:    roleName,
			Managed:     block.HasComment(ManagedMarker),
		})
	}
	return out
}

// ReadCredentialsProfileNames returns section headers only; values in
// ~/.aws/credentials are never read.
func ReadCredentialsProfileNames(paths Paths) []string {
	cf, err := LoadConfigFile(paths.CredentialsPath)
	if err != nil {
		return nil
	}
	var names []string
	for _, block := range cf.Sections() {
		names = append(names, *block.Header)
	}
	return names
}

// Sanitize turns free text into a profile-name-safe fragment: whitespace
// becomes '-', disallowed characters are dropped, and repeated '-' collapse.
func Sanitize(part string) string {
	part = strings.TrimSpace(part)
	part = sanitizeWSRE.ReplaceAllString(part, "-")
	part = sanitizeDropRE.ReplaceAllString(part, "")
	part = sanitizeDashRE.ReplaceAllString(part, "-")
	return strings.Trim(part, "-")
}

// ProposedName generates a default local profile name. System roles
// (CamelCase, e.g. AdministratorAccess) and roles that exist in more than
// one account get an account prefix so names stay unambiguous.
func ProposedName(accountName, roleName string, sharedRoles map[string]bool) string {
	account := Sanitize(accountName)
	role := Sanitize(roleName)
	if systemRoleRE.MatchString(roleName) || sharedRoles[roleName] {
		return account + "-" + role
	}
	return role
}

// ValidateProfileName rejects names that AWS config section syntax, or
// gsso's own parsing, cannot round-trip.
func ValidateProfileName(name string) error {
	if !profileNameRE.MatchString(name) {
		return NewErrorWithFix(ExFail, "use letters, numbers, periods, underscores, and hyphens",
			"unsupported AWS profile name: %q", name)
	}
	return nil
}

// SharedRoleNames returns role names that appear in more than one account,
// since those need an account prefix to stay unambiguous.
func SharedRoleNames(pairs []Pair) map[string]bool {
	counts := map[string]map[string]bool{}
	for _, p := range pairs {
		if counts[p.RoleName] == nil {
			counts[p.RoleName] = map[string]bool{}
		}
		counts[p.RoleName][p.AccountID] = true
	}
	shared := map[string]bool{}
	for role, accounts := range counts {
		if len(accounts) > 1 {
			shared[role] = true
		}
	}
	return shared
}

// ProfileEntry is one profile to create, as produced by `add` and
// `configure`.
type ProfileEntry struct {
	Name        string
	AccountID   string
	AccountName string
	RoleName    string
}

func profileSectionLines(name, accountID, accountName, roleName, region string) []string {
	return []string{
		"[profile " + name + "]\n",
		ManagedMarker + "\n",
		AccountNameKey + " " + accountName + "\n",
		"sso_session = " + SessionName + "\n",
		"sso_account_id = " + accountID + "\n",
		"sso_role_name = " + roleName + "\n",
		"region = " + region + "\n",
		"output = json\n",
	}
}

// WriteProfiles appends every entry in one parse and one atomic write of
// ~/.aws/config.
func WriteProfiles(paths Paths, entries []ProfileEntry, region string) error {
	if len(entries) == 0 {
		return nil
	}
	cf, err := LoadConfigFile(paths.ConfigPath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if cf.Section("profile "+e.Name) != nil {
			return NewError("profile already exists: %s", e.Name)
		}
		if _, err := cf.AppendSection(profileSectionLines(e.Name, e.AccountID, e.AccountName, e.RoleName, region)); err != nil {
			return err
		}
	}
	return cf.Save()
}

// ResolveDefaultRegion is the region new profiles get: [default]'s region if
// set, else the DefaultRegion constant.
func ResolveDefaultRegion(paths Paths) string {
	cf, err := LoadConfigFile(paths.ConfigPath)
	if err != nil {
		return DefaultRegion
	}
	block := cf.Section("default")
	if block == nil {
		return DefaultRegion
	}
	if value, ok := block.Get("region"); ok && value != "" {
		return value
	}
	return DefaultRegion
}

// FindAccountName is a best-effort account name lookup for an account id
// that has no `# account-name:` comment of its own.
func FindAccountName(paths Paths, accountID string) string {
	if accountID == "" {
		return ""
	}
	cf, err := LoadConfigFile(paths.ConfigPath)
	if err != nil {
		return ""
	}
	for _, p := range ReadConfigProfiles(cf) {
		if p.AccountID == accountID && p.AccountName != "" {
			return p.AccountName
		}
	}
	return ""
}
