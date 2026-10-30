package gsso

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

func (a *App) listCmd() *cobra.Command {
	var remote, hardRefresh bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list configured profiles",
		Long: `List profiles from ~/.aws/config.

The default view is local: managed SSO profiles, then any static credential
profiles and unmanaged config profiles. It does not contact Identity Center.

* in the ACTIVE column is the profile this shell will use: AWS_PROFILE when
set, otherwise the profile mirrored by [default]. default means that profile's
account and role match [default], even when this shell is using another
profile. In a terminal the * row is green.

--remote shows the roles Identity Center last reported, from the portal cache,
and whether each one already has a local profile.

--hard-refresh ignores that cache, queries Identity Center, rewrites the cache,
and prints the remote view. --hard is a hidden alias of --hard-refresh.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireAWS(); err != nil {
				return err
			}
			if remote || hardRefresh {
				return a.listRemote(hardRefresh)
			}
			return a.listLocal()
		},
	}
	cmd.Flags().BoolVar(&remote, "remote", false, "show Identity Center roles from the portal cache instead of local profiles")
	cmd.Flags().BoolVar(&hardRefresh, "hard-refresh", false, "re-query Identity Center, rewrite the portal cache, and show that remote view")
	cmd.Flags().BoolVar(&hardRefresh, "hard", false, "alias of --hard-refresh")
	_ = cmd.Flags().MarkHidden("hard")
	return cmd
}

type accountKey struct {
	Name string
	ID   string
}

func (a *App) listLocal() error {
	cf, err := LoadConfigFile(a.Paths.ConfigPath)
	if err != nil {
		return err
	}
	configProfiles := ReadConfigProfiles(cf)
	credentialNames := map[string]bool{}
	for _, n := range ReadCredentialsProfileNames(a.Paths) {
		if n != "default" {
			credentialNames[n] = true
		}
	}
	session := os.Getenv("AWS_PROFILE")
	defaultAccount, defaultRole := defaultSSO(a.Paths, cf)

	var managed []Profile
	var unmanaged []string
	for _, p := range configProfiles {
		if p.Managed {
			managed = append(managed, p)
		} else if !credentialNames[p.Name] {
			unmanaged = append(unmanaged, p.Name)
		}
	}

	groups := map[accountKey][]Profile{}
	for _, p := range managed {
		name := p.AccountName
		if name == "" {
			name = "(unknown account)"
		}
		id := p.AccountID
		if id == "" {
			id = "?"
		}
		key := accountKey{name, id}
		groups[key] = append(groups[key], p)
	}

	if len(groups) == 0 && len(credentialNames) == 0 && len(unmanaged) == 0 {
		Err("No profiles configured. Run `gsso login`.")
		return nil
	}

	var keys []accountKey
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Name != keys[j].Name {
			return keys[i].Name < keys[j].Name
		}
		return keys[i].ID < keys[j].ID
	})
	byName := map[string]Profile{}
	for _, p := range configProfiles {
		byName[p.Name] = p
	}

	var rows [][]string
	var highlight []bool
	for _, key := range keys {
		profiles := groups[key]
		sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
		for _, p := range profiles {
			label, on := activeLabel(session, defaultAccount, defaultRole, p.Name, p.AccountID, p.RoleName)
			rows = append(rows, []string{key.Name, key.ID, p.Name, p.RoleName, label})
			highlight = append(highlight, on)
		}
	}
	if len(rows) > 0 {
		fmt.Print(renderTable([]string{"ACCOUNT", "ACCOUNT ID", "PROFILE", "ROLE", "ACTIVE"}, rows, highlight))
	}

	if len(credentialNames) > 0 {
		fmt.Printf("\nStatic credential profiles (%s)\n", a.Paths.CredentialsPath)
		var names []string
		for n := range credentialNames {
			names = append(names, n)
		}
		sort.Strings(names)
		credRows := make([][]string, len(names))
		credHighlight := make([]bool, len(names))
		for i, n := range names {
			label, on := activeLabel(session, defaultAccount, defaultRole, n, "", "")
			credRows[i] = []string{n, label}
			credHighlight[i] = on
		}
		fmt.Print(renderTable([]string{"PROFILE", "ACTIVE"}, credRows, credHighlight))
	}

	if len(unmanaged) > 0 {
		fmt.Printf("\nUnmanaged profiles (%s)\n", a.Paths.ConfigPath)
		sort.Strings(unmanaged)
		rows := make([][]string, len(unmanaged))
		rowHighlight := make([]bool, len(unmanaged))
		for i, n := range unmanaged {
			p := byName[n]
			label, on := activeLabel(session, defaultAccount, defaultRole, n, p.AccountID, p.RoleName)
			rows[i] = []string{n, label}
			rowHighlight[i] = on
		}
		fmt.Print(renderTable([]string{"PROFILE", "ACTIVE"}, rows, rowHighlight))
	}
	return nil
}

// defaultSSO is the account and role the AWS CLI uses when AWS_PROFILE is
// unset. Those keys are often in ~/.aws/credentials [default]; ~/.aws/config
// [default] is checked first and wins when both are set.
func defaultSSO(paths Paths, cf *ConfigFile) (accountID, role string) {
	accountID, role = ssoAccountRole(cf.Section("default"))
	if accountID != "" && role != "" {
		return accountID, role
	}
	cred, err := LoadConfigFile(paths.CredentialsPath)
	if err != nil {
		return accountID, role
	}
	credAccount, credRole := ssoAccountRole(cred.Section("default"))
	if accountID == "" {
		accountID = credAccount
	}
	if role == "" {
		role = credRole
	}
	return accountID, role
}

func ssoAccountRole(block *Block) (accountID, role string) {
	if block == nil {
		return "", ""
	}
	accountID, _ = block.Get("sso_account_id")
	role, _ = block.Get("sso_role_name")
	return accountID, role
}

// activeLabel reports how a profile is selected.
//
// "*" is the profile this shell will use: AWS_PROFILE when it is set, or the
// profile mirrored by [default] when it is not. "default" is a profile whose
// account and role match [default], including when this shell is pointed
// somewhere else. The bool is true for the shell's profile, which is the row
// list colors.
func activeLabel(session, defaultAccount, defaultRole, name, accountID, role string) (string, bool) {
	matchesDefault := defaultAccount != "" && defaultRole != "" && accountID == defaultAccount && role == defaultRole
	inSession := false
	switch {
	case session == "":
		inSession = matchesDefault
	case session == "default":
		inSession = matchesDefault
	default:
		inSession = name == session
	}
	switch {
	case inSession && matchesDefault:
		return "* default", true
	case inSession:
		return "*", true
	case matchesDefault:
		return "default", false
	default:
		return "", false
	}
}

func (a *App) listRemote(hardRefresh bool) error {
	pairs, err := LoadRemotePairs(a.Paths, hardRefresh)
	if err != nil {
		return err
	}
	cf, err := LoadConfigFile(a.Paths.ConfigPath)
	if err != nil {
		return err
	}
	configured := map[PairKey]bool{}
	for _, p := range ReadConfigProfiles(cf) {
		configured[PairKey{p.AccountID, p.RoleName}] = true
	}
	byAccount := map[accountKey][]string{}
	for _, p := range pairs {
		key := accountKey{p.AccountName, p.AccountID}
		byAccount[key] = append(byAccount[key], p.RoleName)
	}
	var keys []accountKey
	for k := range byAccount {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Name != keys[j].Name {
			return keys[i].Name < keys[j].Name
		}
		return keys[i].ID < keys[j].ID
	})
	var rows [][]string
	for _, key := range keys {
		roles := append([]string(nil), byAccount[key]...)
		sort.Strings(roles)
		for _, role := range roles {
			state := "not configured"
			if configured[PairKey{key.ID, role}] {
				state = "configured"
			}
			rows = append(rows, []string{key.Name, key.ID, role, state})
		}
	}
	if len(rows) == 0 {
		fmt.Println("No Identity Center roles found.")
		return nil
	}
	fmt.Print(renderTable([]string{"ACCOUNT", "ACCOUNT ID", "ROLE", "STATUS"}, rows, nil))
	return nil
}
