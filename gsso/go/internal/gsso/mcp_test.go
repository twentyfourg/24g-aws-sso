package gsso

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestJSONAllowlistPreservesOtherSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	writeFile(t, path, `{
  "mcpServers": {
    "aws-mcp": {
      "command": "uvx",
      "args": ["mcp-proxy-for-aws@latest"],
      "env": {"OTHER": "kept"}
    },
    "other": {"command": "other"}
  }
}`)

	if err := writeJSONMCPProfiles(path, []string{"readonly-24g", "prod-24g"}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	servers := parsed["mcpServers"].(map[string]any)
	awsmcp := servers["aws-mcp"].(map[string]any)
	env := awsmcp["env"].(map[string]any)
	if env["OTHER"] != "kept" {
		t.Errorf("OTHER = %v, want kept", env["OTHER"])
	}
	if env[MCPEnvKey] != "readonly-24g prod-24g" {
		t.Errorf("%s = %v", MCPEnvKey, env[MCPEnvKey])
	}
	other := servers["other"].(map[string]any)
	if other["command"] != "other" {
		t.Errorf("other.command = %v", other["command"])
	}
}

func TestJSONAllowlistToleratesTrailingCommas(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	writeFile(t, path, "{\n"+
		`  "mcpServers": {`+"\n"+
		`    "aws-mcp": {`+"\n"+
		`      "command": "uvx",`+"\n"+
		`      "args": [`+"\n"+
		`        "mcp-proxy-for-aws@latest",`+"\n"+
		`        "INSTALL_SOURCE=aws-cli,]",`+"\n"+
		"      ],\n"+
		`      "env": {`+"\n"+
		`        "AWS_MCP_PROXY_PROFILES": "readonly-24g"`+"\n"+
		"      }\n"+
		"    }\n"+
		"  }\n"+
		"}\n")

	profiles, found, err := readJSONMCPProfiles(path)
	if err != nil || !found {
		t.Fatalf("readJSONMCPProfiles: profiles=%v found=%v err=%v", profiles, found, err)
	}
	if len(profiles) != 1 || profiles[0] != "readonly-24g" {
		t.Fatalf("got %v", profiles)
	}

	if err := writeJSONMCPProfiles(path, []string{"readonly-24g", "prod-24g"}); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, data)
	}
	args := parsed["mcpServers"].(map[string]any)["aws-mcp"].(map[string]any)["args"].([]any)
	if len(args) != 2 || args[1] != "INSTALL_SOURCE=aws-cli,]" {
		t.Errorf("args corrupted: %v", args)
	}

	profiles, found, err = readJSONMCPProfiles(path)
	if err != nil || !found {
		t.Fatalf("re-read failed: %v %v %v", profiles, found, err)
	}
	if len(profiles) != 2 || profiles[0] != "readonly-24g" || profiles[1] != "prod-24g" {
		t.Fatalf("got %v", profiles)
	}
}

func TestTOMLAllowlistAddsEnvironmentTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeFile(t, path, `[mcp_servers."aws-mcp"]
command = "uvx"
args = ["mcp-proxy-for-aws@latest"]

[unrelated]
value = "kept"
`)

	if err := writeTOMLMCPProfiles(path, []string{"readonly-24g", "prod-24g"}); err != nil {
		t.Fatal(err)
	}

	profiles, found, err := readTOMLMCPProfiles(path)
	if err != nil || !found {
		t.Fatalf("readTOMLMCPProfiles: %v %v %v", profiles, found, err)
	}
	if len(profiles) != 2 || profiles[0] != "readonly-24g" || profiles[1] != "prod-24g" {
		t.Fatalf("got %v", profiles)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "[unrelated]\nvalue = \"kept\"") {
		t.Errorf("unrelated section not preserved:\n%s", data)
	}
}

func TestTOMLAllowlistMergesInlineEnvironment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeFile(t, path, "[mcp_servers.aws-mcp]\n"+
		`command = "uvx"`+"\n"+
		`env = { OTHER = "kept" }`+"\n")

	if err := writeTOMLMCPProfiles(path, []string{"readonly-24g"}); err != nil {
		t.Fatal(err)
	}

	profiles, found, err := readTOMLMCPProfiles(path)
	if err != nil || !found {
		t.Fatalf("readTOMLMCPProfiles: %v %v %v", profiles, found, err)
	}
	if len(profiles) != 1 || profiles[0] != "readonly-24g" {
		t.Fatalf("got %v", profiles)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `OTHER = "kept"`) {
		t.Errorf("OTHER not preserved:\n%s", data)
	}
}

func TestWithDefaultFirstReordersWithoutResorting(t *testing.T) {
	got, err := WithDefaultFirst([]string{"alpha-24g", "readonly-24g", "prod-24g"}, "prod-24g")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"prod-24g", "alpha-24g", "readonly-24g"}
	if !sameProfiles(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCurrentMCPProfilesTruncatesToIntersection(t *testing.T) {
	configs := []McpConfig{
		{Agent: "Cursor", Profiles: []string{"prod-24g", "alpha-24g", "readonly-24g"}},
		{Agent: "Codex", Profiles: []string{"prod-24g", "alpha-24g"}},
	}
	got := CurrentMCPProfiles(configs, false)
	want := []string{"prod-24g", "alpha-24g"}
	if !sameProfiles(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCurrentMCPProfilesEmptyIntersectionIsEmpty(t *testing.T) {
	configs := []McpConfig{
		{Agent: "Cursor", Profiles: []string{"prod-24g"}},
		{Agent: "Codex", Profiles: []string{"readonly-24g"}},
	}
	got := CurrentMCPProfiles(configs, false)
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}
