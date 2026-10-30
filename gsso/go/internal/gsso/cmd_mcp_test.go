package gsso

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProfileNames creates minimal [profile NAME] sections so
// completionSwitchTargets() (the "known local profile" allowlist) includes
// them, without needing full SSO details.
func writeProfileNames(t *testing.T, paths Paths, names ...string) {
	t.Helper()
	var sb strings.Builder
	for _, n := range names {
		sb.WriteString("[profile " + n + "]\nregion = us-east-1\n\n")
	}
	writeFile(t, paths.ConfigPath, sb.String())
}

func countEntries(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestMCPAddRealignsDriftedAgentConfigs(t *testing.T) {
	home := t.TempDir()
	paths := NewPaths(home)
	writeProfileNames(t, paths, "prod-24g", "readonly-24g", "extra-24g", "alpha-24g")

	cursorPath := filepath.Join(home, ".cursor", "mcp.json")
	writeFile(t, cursorPath, `{"mcpServers":{"aws-mcp":{"command":"uvx","env":{"AWS_MCP_PROXY_PROFILES":"prod-24g readonly-24g extra-24g"}}}}`)
	codexPath := filepath.Join(home, ".codex", "config.toml")
	writeFile(t, codexPath, "[mcp_servers.aws-mcp]\n"+
		`command = "uvx"`+"\n"+
		"[mcp_servers.aws-mcp.env]\n"+
		`AWS_MCP_PROXY_PROFILES = "prod-24g readonly-24g"`+"\n")

	app := NewApp(paths)
	if err := app.runMCPChange("add", []string{"alpha-24g"}); err != nil {
		t.Fatal(err)
	}

	got, _, err := readJSONMCPProfiles(cursorPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"prod-24g", "readonly-24g", "alpha-24g"}
	if !sameProfiles(got, want) {
		t.Errorf("cursor: got %v, want %v", got, want)
	}
	got, _, err = readTOMLMCPProfiles(codexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !sameProfiles(got, want) {
		t.Errorf("codex: got %v, want %v", got, want)
	}
}

func TestMCPDefaultMovesProfileToFront(t *testing.T) {
	home := t.TempDir()
	paths := NewPaths(home)
	cursorPath := filepath.Join(home, ".cursor", "mcp.json")
	writeFile(t, cursorPath, `{"mcpServers":{"aws-mcp":{"command":"uvx","env":{"AWS_MCP_PROXY_PROFILES":"alpha-24g readonly-24g prod-24g"}}}}`)

	app := NewApp(paths)
	if err := app.runMCPDefault("readonly-24g"); err != nil {
		t.Fatal(err)
	}

	got, _, err := readJSONMCPProfiles(cursorPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"readonly-24g", "alpha-24g", "prod-24g"}
	if !sameProfiles(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMCPAddUpdatesAllDetectedConfigs(t *testing.T) {
	home := t.TempDir()
	paths := NewPaths(home)
	writeProfileNames(t, paths, "readonly-24g", "prod-24g")

	cursorPath := filepath.Join(home, ".cursor", "mcp.json")
	writeFile(t, cursorPath, `{"mcpServers":{"aws-mcp":{"command":"uvx","args":["proxy"]}}}`)
	codexPath := filepath.Join(home, ".codex", "config.toml")
	writeFile(t, codexPath, "[mcp_servers.aws-mcp]\ncommand = \"uvx\"\nargs = [\"proxy\"]\n")

	app := NewApp(paths)
	if err := app.runMCPChange("add", []string{"readonly-24g", "prod-24g"}); err != nil {
		t.Fatal(err)
	}

	want := []string{"readonly-24g", "prod-24g"}
	if got, _, _ := readJSONMCPProfiles(cursorPath); !sameProfiles(got, want) {
		t.Errorf("cursor: got %v", got)
	}
	if got, _, _ := readTOMLMCPProfiles(codexPath); !sameProfiles(got, want) {
		t.Errorf("codex: got %v", got)
	}
	if n := countEntries(t, paths.MCPBackupRoot); n != 1 {
		t.Errorf("expected exactly one backup directory, got %d", n)
	}
}

func TestMCPRemoveUpdatesAllowlist(t *testing.T) {
	home := t.TempDir()
	paths := NewPaths(home)
	writeProfileNames(t, paths, "readonly-24g", "prod-24g")

	cursorPath := filepath.Join(home, ".cursor", "mcp.json")
	writeFile(t, cursorPath, `{"mcpServers":{"aws-mcp":{"command":"uvx","env":{"AWS_MCP_PROXY_PROFILES":"readonly-24g prod-24g"}}}}`)

	app := NewApp(paths)
	if err := app.runMCPChange("remove", []string{"readonly-24g"}); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := readJSONMCPProfiles(cursorPath); !sameProfiles(got, []string{"prod-24g"}) {
		t.Errorf("got %v", got)
	}
}

func TestMCPRemoveRefusesToEmptyAllowlist(t *testing.T) {
	home := t.TempDir()
	paths := NewPaths(home)
	writeProfileNames(t, paths, "readonly-24g")
	cursorPath := filepath.Join(home, ".cursor", "mcp.json")
	writeFile(t, cursorPath, `{"mcpServers":{"aws-mcp":{"command":"uvx","env":{"AWS_MCP_PROXY_PROFILES":"readonly-24g"}}}}`)

	app := NewApp(paths)
	err := app.runMCPChange("remove", []string{"readonly-24g"})
	if err == nil {
		t.Fatal("expected an error")
	}
}
