package gsso

import (
	"os"
	"sort"

	"github.com/spf13/cobra"
)

func (a *App) removeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove [PROFILE ...]",
		Short: "delete managed profiles",
		Args:  cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			already := profileSet(args)
			var out []string
			for _, p := range a.completionManagedProfiles() {
				if !already[p] {
					out = append(out, p)
				}
			}
			return out, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runRemove(args)
		},
	}
	return cmd
}

func (a *App) runRemove(targets []string) error {
	if err := RequireAWS(); err != nil {
		return err
	}
	cf, err := LoadConfigFile(a.Paths.ConfigPath)
	if err != nil {
		return err
	}
	profiles := map[string]Profile{}
	for _, p := range ReadConfigProfiles(cf) {
		profiles[p.Name] = p
	}
	var managed []string
	for name, p := range profiles {
		if p.Managed {
			managed = append(managed, name)
		}
	}
	sort.Strings(managed)

	if len(targets) == 0 {
		if len(managed) == 0 {
			Err("No managed profiles to remove.")
			return nil
		}
		picked, err := ChooseMultiple("Remove which profiles", managed, nil)
		if err != nil {
			return err
		}
		targets = picked
	}

	var doomed []string
	for _, name := range targets {
		profile, ok := profiles[name]
		if !ok {
			Err("skipped %s: no such profile in %s", name, a.Paths.ConfigPath)
		} else if !profile.Managed {
			Err("skipped %s: not managed by this tool, so it will not be touched", name)
		} else {
			doomed = append(doomed, name)
		}
	}
	if len(doomed) == 0 {
		// Every target was already reported via a "skipped ..." line above.
		return &Error{Code: ExFail}
	}

	backup, err := CreateRollbackPoint(a.Paths)
	if err != nil {
		return err
	}
	Err("Rollback point: %s", backup)

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
	for _, name := range doomed {
		cf2.RemoveSection("profile " + name)
		Err("removed %s", name)
		if name == current {
			Err("  note: AWS_PROFILE still points at %s in this shell", name)
		}
		if defaultAccount != "" && profiles[name].AccountID == defaultAccount {
			Err("  note: [default] mirrors this profile and now references a removed role")
		}
	}
	return cf2.Save()
}

func (a *App) completionManagedProfiles() []string {
	cf, err := LoadConfigFile(a.Paths.ConfigPath)
	if err != nil {
		return nil
	}
	var out []string
	for _, p := range ReadConfigProfiles(cf) {
		if p.Managed {
			out = append(out, p.Name)
		}
	}
	sort.Strings(out)
	return out
}
