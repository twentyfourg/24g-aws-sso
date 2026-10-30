package gsso

import (
	"os"
	"path/filepath"
)

// semantic-release-version-start
const Version = "1.0.0"

// semantic-release-version-end

const (
	SessionName   = "24g"
	SSOStartURL   = "https://24glogin.awsapps.com/start"
	SSORegion     = "us-east-1"
	DefaultRegion = "us-east-1"

	ManagedMarker  = "# managed-by: 24g-sso-setup"
	AccountNameKey = "# account-name:"

	// Identity Center read APIs share a 20 TPS account-level throttle. Each
	// list-account-roles CLI call takes ~0.8s, so 10 in flight is about 12 TPS.
	RoleFetchConcurrency = 10
	RoleFetchRetries     = 4

	MCPEnvKey = "AWS_MCP_PROXY_PROFILES"
)

// Paths bundles every filesystem location gsso reads or writes. A struct
// (rather than package-level vars) keeps tests able to point at a temp HOME.
type Paths struct {
	Home            string
	AWSDir          string
	ConfigPath      string
	CredentialsPath string
	SSOCacheDir     string
	BackupRoot      string
	MCPBackupRoot   string
	PortalCachePath string
}

// NewPaths derives every path from $HOME (or the given override).
func NewPaths(home string) Paths {
	awsDir := filepath.Join(home, ".aws")
	return Paths{
		Home:            home,
		AWSDir:          awsDir,
		ConfigPath:      filepath.Join(awsDir, "config"),
		CredentialsPath: filepath.Join(awsDir, "credentials"),
		SSOCacheDir:     filepath.Join(awsDir, "sso", "cache"),
		BackupRoot:      filepath.Join(awsDir, ".24g-setup-backups"),
		MCPBackupRoot:   filepath.Join(awsDir, ".24g-mcp-setup-backups"),
		PortalCachePath: filepath.Join(awsDir, "gsso", "portal-cache.json"),
	}
}

// DefaultPaths resolves paths from the real user home directory.
func DefaultPaths() Paths {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return NewPaths(home)
}
