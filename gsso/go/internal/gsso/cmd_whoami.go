package gsso

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func (a *App) whoamiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "show which profile and identity are currently active",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runWhoami()
		},
	}
	return cmd
}

type callerIdentity struct {
	Account string `json:"Account"`
	Arn     string `json:"Arn"`
	UserID  string `json:"UserId"`
}

func (a *App) runWhoami() error {
	if err := RequireAWS(); err != nil {
		return err
	}
	profileEnv := os.Getenv("AWS_PROFILE")
	awsArgs := []string{"sts", "get-caller-identity"}
	if profileEnv != "" {
		awsArgs = append(awsArgs, "--profile", profileEnv)
	}
	out, err := RunAWS(awsArgs, "")
	if err != nil {
		lowered := strings.ToLower(err.Error())
		if strings.Contains(lowered, "sso token") || strings.Contains(lowered, "sso session") {
			return NewErrorWithFix(ExAuth, "run `gsso login`", "%s", err.Error())
		}
		if strings.Contains(lowered, "could not be found") && strings.Contains(lowered, "profile") {
			return NewErrorWithFix(ExFail, "run `gsso list` to see valid profile names", "%s", err.Error())
		}
		return err
	}
	var identity callerIdentity
	if err := json.Unmarshal([]byte(out), &identity); err != nil {
		return NewError("could not parse sts get-caller-identity output: %v", err)
	}

	cf, err := LoadConfigFile(a.Paths.ConfigPath)
	if err != nil {
		return err
	}
	var label string
	var block *Block
	if profileEnv != "" {
		label = fmt.Sprintf("%s  (from AWS_PROFILE)", profileEnv)
		block = cf.Section("profile " + profileEnv)
	} else {
		label = "[default]  (AWS_PROFILE is not set in this shell)"
		block = cf.Section("default")
	}
	var roleName, accountName string
	if block != nil {
		roleName, _ = block.Get("sso_role_name")
		accountName, _ = block.CommentValue(AccountNameKey)
	}
	if roleName == "" && profileEnv == "" {
		_, roleName = defaultSSO(a.Paths, cf)
	}
	if accountName == "" {
		accountName = FindAccountName(a.Paths, identity.Account)
	}

	fmt.Printf("Profile:  %s\n", label)
	accountLabel := ""
	if accountName != "" {
		accountLabel = accountName + " "
	}
	fmt.Printf("Account:  %s(%s)\n", accountLabel, identity.Account)
	if roleName != "" {
		fmt.Printf("Role:     %s\n", roleName)
	} else if accountName == "" {
		fmt.Println("Note:     not a profile gsso manages; showing raw AWS identity only")
	}
	fmt.Printf("ARN:      %s\n", identity.Arn)
	fmt.Printf("User ID:  %s\n", identity.UserID)
	return nil
}
