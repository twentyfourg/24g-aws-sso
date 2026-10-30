package gsso

import (
	"sort"

	"github.com/spf13/cobra"
)

func (a *App) addCmd() *cobra.Command {
	var alias string
	var hardRefresh bool
	cmd := &cobra.Command{
		Use:   "add [ACCOUNT/ROLE ...]",
		Short: "add profiles for roles that are not configured yet",
		Long: `Create managed profiles for Identity Center roles that do not have one yet.

Add never removes a profile. Pass one or more ACCOUNT/ROLE targets, or omit
them to pick from a menu of unconfigured roles.

Use configure when you want to add and remove managed roles in one pass.`,
		Args: cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			already := profileSet(args)
			var out []string
			for _, c := range a.completionAddTargets() {
				if !already[c] {
					out = append(out, c)
				}
			}
			return out, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runAdd(args, alias, hardRefresh)
		},
	}
	cmd.Flags().StringVar(&alias, "alias", "", "use this profile name (requires exactly one ACCOUNT/ROLE)")
	cmd.Flags().BoolVar(&hardRefresh, "hard-refresh", false, "re-query Identity Center before adding, instead of using the portal cache")
	cmd.Flags().BoolVar(&hardRefresh, "hard", false, "alias of --hard-refresh")
	_ = cmd.Flags().MarkHidden("hard")
	return cmd
}

func (a *App) runAdd(targets []string, alias string, hardRefresh bool) error {
	if err := RequireAWS(); err != nil {
		return err
	}
	if alias != "" && len(targets) != 1 {
		return NewErrorWithFix(ExFail, "run `gsso add --alias PROFILE ACCOUNT/ROLE`",
			"--alias requires exactly one ACCOUNT/ROLE target")
	}
	if alias != "" {
		if err := ValidateProfileName(alias); err != nil {
			return err
		}
	}
	pairs, err := LoadRemotePairs(a.Paths, hardRefresh)
	if err != nil {
		return err
	}
	shared := SharedRoleNames(pairs)
	cf, err := LoadConfigFile(a.Paths.ConfigPath)
	if err != nil {
		return err
	}
	existingProfiles := ReadConfigProfiles(cf)
	existing := map[string]bool{}
	configured := map[PairKey]bool{}
	for _, p := range existingProfiles {
		existing[p.Name] = true
		configured[PairKey{p.AccountID, p.RoleName}] = true
	}

	candidates := map[string]Pair{}
	for _, p := range pairs {
		if !configured[PairKey{p.AccountID, p.RoleName}] {
			candidates[p.AccountName+"/"+p.RoleName] = p
		}
	}
	if len(candidates) == 0 {
		Err("Every available role already has a profile.")
		return nil
	}

	var selected []Pair
	if len(targets) > 0 {
		for _, target := range targets {
			p, ok := candidates[target]
			if !ok {
				return NewErrorWithFix(ExFail, "run `gsso list --remote` to see valid values",
					"not an available, unconfigured role: %s", target)
			}
			selected = append(selected, p)
		}
	} else {
		var names []string
		for name := range candidates {
			names = append(names, name)
		}
		sort.Strings(names)
		picked, err := ChooseMultiple("Add which roles", names, nil)
		if err != nil {
			return err
		}
		for _, name := range picked {
			selected = append(selected, candidates[name])
		}
	}

	if alias != "" && existing[alias] {
		return NewError("profile already exists: %s", alias)
	}

	var planned []ProfileEntry
	for _, p := range selected {
		name := alias
		if name == "" {
			name = ProposedName(p.AccountName, p.RoleName, shared)
		}
		if existing[name] {
			Err("skipped %s: a profile with that name already exists", name)
			continue
		}
		planned = append(planned, ProfileEntry{Name: name, AccountID: p.AccountID, AccountName: p.AccountName, RoleName: p.RoleName})
		existing[name] = true
	}
	if len(planned) == 0 {
		return nil
	}

	backup, err := CreateRollbackPoint(a.Paths)
	if err != nil {
		return err
	}
	Err("Rollback point: %s", backup)
	if err := WriteProfiles(a.Paths, planned, ResolveDefaultRegion(a.Paths)); err != nil {
		return err
	}
	for _, e := range planned {
		Err("added %s", e.Name)
	}
	return nil
}

// completionAddTargets lists unconfigured account/role pairs from the portal
// cache only. It never queries live, since shell completion must stay fast
// and must never touch the network.
func (a *App) completionAddTargets() []string {
	_, pairs, ok := ReadPortalCache(a.Paths)
	if !ok {
		return nil
	}
	cf, err := LoadConfigFile(a.Paths.ConfigPath)
	if err != nil {
		return nil
	}
	configured := map[PairKey]bool{}
	for _, p := range ReadConfigProfiles(cf) {
		configured[PairKey{p.AccountID, p.RoleName}] = true
	}
	var out []string
	for _, p := range pairs {
		if !configured[PairKey{p.AccountID, p.RoleName}] {
			out = append(out, p.AccountName+"/"+p.RoleName)
		}
	}
	sort.Strings(out)
	return out
}
