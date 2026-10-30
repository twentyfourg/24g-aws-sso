package gsso

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func (a *App) mcpCmd() *cobra.Command {
	mcp := &cobra.Command{
		Use:   "mcp",
		Short: "manage profiles allowed through AWS MCP",
	}
	mcp.AddCommand(a.mcpAddCmd(), a.mcpRemoveCmd(), a.mcpDefaultCmd())
	return mcp
}

func (a *App) mcpAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [PROFILE ...]",
		Short: "allow local AWS profiles through AWS MCP",
		Args:  cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return a.completionMCPAddTargets(args), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runMCPChange("add", args)
		},
	}
}

func (a *App) mcpRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [PROFILE ...]",
		Short: "remove AWS profiles from the MCP allowlist",
		Args:  cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return a.completionMCPCurrentProfiles(args), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runMCPChange("remove", args)
		},
	}
}

func (a *App) mcpDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "default [PROFILE]",
		Short: "set the default AWS MCP profile",
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return a.completionMCPCurrentProfiles(nil), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var profile string
			if len(args) == 1 {
				profile = args[0]
			}
			return a.runMCPDefault(profile)
		},
	}
}

func (a *App) runMCPChange(action string, profiles []string) error {
	configs, err := discoverMCPConfigs(a.Paths.Home)
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		return NewErrorWithFix(ExFail, "run the AWS MCP setup workflow first", "no configured aws-mcp server found")
	}
	current := CurrentMCPProfiles(configs, true)
	known := a.completionSwitchTargets()
	currentSet := profileSet(current)
	knownSet := profileSet(known)

	var updated []string
	if action == "add" {
		if len(profiles) > 0 {
			seen := map[string]bool{}
			var targets []string
			for _, p := range profiles {
				if err := ValidateMCPProfileName(p); err != nil {
					return err
				}
				if !knownSet[p] {
					return NewErrorWithFix(ExFail, "run `gsso list` to see configured profiles",
						"no such local AWS profile: %s", p)
				}
				if currentSet[p] {
					Err("already allowed: %s", p)
				} else if !seen[p] {
					seen[p] = true
					targets = append(targets, p)
				}
			}
			updated, err = WithDefaultFirst(append(append([]string(nil), current...), targets...), "")
			if err != nil {
				return err
			}
		} else {
			optionSet := map[string]bool{}
			for p := range knownSet {
				optionSet[p] = true
			}
			for p := range currentSet {
				optionSet[p] = true
			}
			if len(optionSet) == 0 {
				return NewErrorWithFix(ExFail, "run `gsso add` to create profiles first", "no local AWS profiles found")
			}
			var options []string
			for p := range optionSet {
				options = append(options, p)
			}
			sort.Strings(options)
			targets, chosenDefault, err := ChooseMultipleWithDefault(
				"Allow which profiles in AWS MCP (checked = allowed now)", options, current, "")
			if err != nil {
				return err
			}
			targetSet := profileSet(targets)
			var kept, added []string
			for _, p := range current {
				if targetSet[p] {
					kept = append(kept, p)
				}
			}
			for _, p := range targets {
				if !currentSet[p] {
					added = append(added, p)
				}
			}
			updated, err = WithDefaultFirst(append(kept, added...), chosenDefault)
			if err != nil {
				return err
			}
		}
	} else {
		if len(current) == 0 {
			return NewErrorWithFix(ExFail, "use `gsso mcp add` first", "the AWS MCP profile allowlist is empty")
		}
		var targets []string
		if len(profiles) > 0 {
			seen := map[string]bool{}
			for _, p := range profiles {
				if err := ValidateMCPProfileName(p); err != nil {
					return err
				}
				if !currentSet[p] {
					Err("not currently allowed: %s", p)
				} else if !seen[p] {
					seen[p] = true
					targets = append(targets, p)
				}
			}
		} else {
			picked, err := ChooseMultiple("Remove which profiles from AWS MCP", current, nil)
			if err != nil {
				return err
			}
			targets = picked
		}
		targetSet := profileSet(targets)
		for _, p := range current {
			if !targetSet[p] {
				updated = append(updated, p)
			}
		}
		if len(updated) == 0 {
			return NewErrorWithFix(ExFail, "keep one default profile, or disable aws-mcp in each agent",
				"refusing to remove every AWS MCP profile")
		}
	}
	return a.saveMCPProfiles(configs, updated)
}

func (a *App) runMCPDefault(profile string) error {
	configs, err := discoverMCPConfigs(a.Paths.Home)
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		return NewErrorWithFix(ExFail, "run the AWS MCP setup workflow first", "no configured aws-mcp server found")
	}
	current := CurrentMCPProfiles(configs, true)
	if len(current) == 0 {
		return NewErrorWithFix(ExFail, "use `gsso mcp add` first", "the AWS MCP profile allowlist is empty")
	}
	target := profile
	if target == "" {
		chosen, err := ChooseOne("Default AWS MCP profile", current)
		if err != nil {
			return err
		}
		target = chosen
	} else if err := ValidateMCPProfileName(target); err != nil {
		return err
	}
	updated, err := WithDefaultFirst(current, target)
	if err != nil {
		return err
	}
	return a.saveMCPProfiles(configs, updated)
}

func (a *App) saveMCPProfiles(configs []McpConfig, updated []string) error {
	allSame := true
	for _, c := range configs {
		if !sameProfiles(c.Profiles, updated) {
			allSame = false
			break
		}
	}
	if allSame {
		if len(updated) > 0 {
			Err("AWS MCP default profile: %s", updated[0])
			Err("AWS MCP allowed profiles: %s", strings.Join(updated, " "))
		}
		return nil
	}
	var paths []string
	for _, c := range configs {
		paths = append(paths, c.Path)
	}
	backup, err := CreateMCPRollbackPoint(a.Paths, paths)
	if err != nil {
		return err
	}
	Err("Rollback point: %s", backup)
	for _, c := range configs {
		if err := writeMCPProfiles(c, updated); err != nil {
			return err
		}
		Err("updated %s: %s", c.Agent, c.Path)
	}
	if len(updated) > 0 {
		Err("AWS MCP default profile: %s", updated[0])
		Err("AWS MCP allowed profiles: %s", strings.Join(updated, " "))
	}
	Err("Restart the configured agents to reload AWS MCP.")
	return nil
}

func (a *App) completionMCPAddTargets(already []string) []string {
	configs, err := discoverMCPConfigs(a.Paths.Home)
	if err != nil {
		return nil
	}
	allowed := profileSet(CurrentMCPProfiles(configs, false))
	seen := profileSet(already)
	var out []string
	for _, p := range a.completionSwitchTargets() {
		if !allowed[p] && !seen[p] {
			out = append(out, p)
		}
	}
	return out
}

func (a *App) completionMCPCurrentProfiles(already []string) []string {
	configs, err := discoverMCPConfigs(a.Paths.Home)
	if err != nil {
		return nil
	}
	seen := profileSet(already)
	var out []string
	for _, p := range CurrentMCPProfiles(configs, false) {
		if !seen[p] {
			out = append(out, p)
		}
	}
	return out
}
