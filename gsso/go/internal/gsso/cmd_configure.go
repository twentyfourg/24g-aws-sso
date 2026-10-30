package gsso

import (
	"fmt"
	"os"
	"sort"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

func (a *App) configureCmd() *cobra.Command {
	var hardRefresh bool
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "add and remove managed roles in one menu",
		Long: `Choose the complete set of managed SSO roles.

Every available role is listed. Roles that already have a profile start
checked. Leave a role checked to keep it, check a new one to add it, and
uncheck a managed role to remove it.

Use add when you only want to create profiles and leave everything else alone.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireAWS(); err != nil {
				return err
			}
			return a.runConfigure(hardRefresh)
		},
	}
	cmd.Flags().BoolVar(&hardRefresh, "hard-refresh", false, "re-query Identity Center before configuring, instead of using the portal cache")
	cmd.Flags().BoolVar(&hardRefresh, "hard", false, "alias of --hard-refresh")
	_ = cmd.Flags().MarkHidden("hard")
	return cmd
}

func (a *App) runConfigure(hardRefresh bool) error {
	remotePairs, err := LoadRemotePairs(a.Paths, hardRefresh)
	if err != nil {
		return err
	}
	cf, err := LoadConfigFile(a.Paths.ConfigPath)
	if err != nil {
		return err
	}
	profiles := ReadConfigProfiles(cf)

	configured := map[PairKey]bool{}
	for _, p := range profiles {
		if p.AccountID != "" && p.RoleName != "" {
			configured[PairKey{p.AccountID, p.RoleName}] = true
		}
	}
	managedByPair := map[PairKey]Profile{}
	for _, p := range profiles {
		if p.Managed && p.AccountID != "" && p.RoleName != "" {
			managedByPair[PairKey{p.AccountID, p.RoleName}] = p
		}
	}
	remoteNameByPair := map[PairKey]string{}
	for _, p := range remotePairs {
		remoteNameByPair[PairKey{p.AccountID, p.RoleName}] = p.AccountName
	}

	keySet := map[PairKey]bool{}
	for k := range remoteNameByPair {
		keySet[k] = true
	}
	for k := range managedByPair {
		keySet[k] = true
	}
	var keys []PairKey
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].AccountID != keys[j].AccountID {
			return keys[i].AccountID < keys[j].AccountID
		}
		return keys[i].RoleName < keys[j].RoleName
	})

	choices := map[string]PairKey{}
	var preselected []string
	for _, key := range keys {
		var label string
		if accountName, ok := remoteNameByPair[key]; ok {
			label = fmt.Sprintf("%s (%s)/%s", accountName, key.AccountID, key.RoleName)
		} else {
			profile := managedByPair[key]
			accountName := profile.AccountName
			if accountName == "" {
				accountName = key.AccountID
			}
			label = fmt.Sprintf("%s (%s)/%s (not currently available)", accountName, key.AccountID, key.RoleName)
		}
		choices[label] = key
		if configured[key] {
			preselected = append(preselected, label)
		}
	}

	if len(choices) == 0 {
		Err("No available or configured SSO roles found.")
		return nil
	}

	var labels []string
	for label := range choices {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	picked, err := ChooseMultiple("Configure SSO roles (checked = configured now)", labels, preselected)
	if err != nil {
		return err
	}
	selected := map[PairKey]bool{}
	for _, label := range picked {
		selected[choices[label]] = true
	}

	var additions []Pair
	for _, p := range remotePairs {
		key := PairKey{p.AccountID, p.RoleName}
		if selected[key] && !configured[key] {
			additions = append(additions, p)
		}
	}
	var doomed []Profile
	for key, profile := range managedByPair {
		if !selected[key] {
			doomed = append(doomed, profile)
		}
	}
	sort.Slice(doomed, func(i, j int) bool { return doomed[i].Name < doomed[j].Name })

	if len(additions) == 0 && len(doomed) == 0 {
		Err("SSO role configuration is already up to date.")
		return nil
	}

	shared := SharedRoleNames(remotePairs)
	removedNames := map[string]bool{}
	for _, p := range doomed {
		removedNames[p.Name] = true
	}
	occupied := map[string]bool{}
	for _, p := range profiles {
		if !removedNames[p.Name] {
			occupied[p.Name] = true
		}
	}

	var plannedAdditions []ProfileEntry
	for _, p := range additions {
		def := ProposedName(p.AccountName, p.RoleName, shared)
		name, err := a.promptProfileName(fmt.Sprintf("%s/%s", p.AccountName, p.RoleName), def, occupied)
		if err != nil {
			return err
		}
		occupied[name] = true
		plannedAdditions = append(plannedAdditions, ProfileEntry{Name: name, AccountID: p.AccountID, AccountName: p.AccountName, RoleName: p.RoleName})
	}

	backup, err := CreateRollbackPoint(a.Paths)
	if err != nil {
		return err
	}
	Err("Rollback point: %s", backup)

	if len(doomed) > 0 {
		cf2, err := LoadConfigFile(a.Paths.ConfigPath)
		if err != nil {
			return err
		}
		defaultBlock := cf2.Section("default")
		var defaultAccount string
		if defaultBlock != nil {
			defaultAccount, _ = defaultBlock.Get("sso_account_id")
		}
		current := os.Getenv("AWS_PROFILE")
		for _, profile := range doomed {
			cf2.RemoveSection("profile " + profile.Name)
			Err("removed %s", profile.Name)
			if profile.Name == current {
				Err("  note: AWS_PROFILE still points at %s in this shell", profile.Name)
			}
			if defaultAccount != "" && profile.AccountID == defaultAccount {
				Err("  note: [default] mirrors this profile and now references a removed role")
			}
		}
		if err := cf2.Save(); err != nil {
			return err
		}
	}

	if err := WriteProfiles(a.Paths, plannedAdditions, ResolveDefaultRegion(a.Paths)); err != nil {
		return err
	}
	for _, e := range plannedAdditions {
		Err("added %s", e.Name)
	}
	return nil
}

// promptProfileName asks for a name for a newly checked role, pre-filled
// with the generated default. Enter accepts it; any other value must still
// be a valid, unused profile name.
func (a *App) promptProfileName(label, def string, occupied map[string]bool) (string, error) {
	if err := RequireTTY("naming a new profile"); err != nil {
		return "", err
	}
	name := def
	field := huh.NewInput().
		Title(fmt.Sprintf("Profile name for %s", label)).
		Description(fmt.Sprintf("default: %s", def)).
		Value(&name).
		Validate(func(s string) error {
			if s == "" {
				return nil // empty submits the default, handled below
			}
			if err := ValidateProfileName(s); err != nil {
				return err
			}
			if occupied[s] {
				return fmt.Errorf("profile already exists: %s", s)
			}
			return nil
		})
	if err := field.WithTheme(theme()).Run(); err != nil {
		return "", mapHuhErr(err)
	}
	if name == "" {
		name = def
	}
	return name, nil
}
