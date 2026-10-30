package gsso

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var mcpProfileRE = regexp.MustCompile(`^[^\s"\\]+$`)

// McpConfig is one AI agent's aws-mcp server entry, as found on disk.
type McpConfig struct {
	Agent    string
	Path     string
	Kind     string // "json" or "toml"
	Profiles []string
}

type mcpLocation struct {
	agent string
	path  string
	kind  string
}

func mcpConfigLocations(home string) []mcpLocation {
	claudeRoot := envOr("CLAUDE_CONFIG_DIR", home)
	codexRoot := envOr("CODEX_HOME", home+string(os.PathSeparator)+".codex")
	return []mcpLocation{
		{"Cursor", home + "/.cursor/mcp.json", "json"},
		{"Claude Code", claudeRoot + "/.claude.json", "json"},
		{"Gemini CLI", home + "/.gemini/settings.json", "json"},
		{"Kiro", home + "/.kiro/settings/mcp.json", "json"},
		{"Codex", codexRoot + "/config.toml", "toml"},
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ValidateMCPProfileName rejects profile names the MCP env-var encoding
// (space-separated, double-quoted TOML string) cannot represent.
func ValidateMCPProfileName(profile string) error {
	if !mcpProfileRE.MatchString(profile) {
		return NewErrorWithFix(ExFail, "use a profile name without whitespace, quotes, or backslashes",
			"unsupported MCP profile name: %q", profile)
	}
	return nil
}

func splitMCPProfiles(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	profiles := strings.Fields(raw)
	for _, p := range profiles {
		if err := ValidateMCPProfileName(p); err != nil {
			return nil, err
		}
	}
	return profiles, nil
}

// stripTrailingCommas drops a `,` that sits directly before a `}` or `]`,
// ignoring commas inside strings. Editors such as Cursor write JSON with a
// trailing comma that this script's own hand-edited files may carry too;
// that is not valid strict JSON but it is what the agent itself loads fine.
func stripTrailingCommas(text string) string {
	runes := []rune(text)
	var out []rune
	inString := false
	escaped := false
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if inString {
			out = append(out, ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			out = append(out, ch)
			continue
		}
		if ch == ',' {
			j := i + 1
			for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\r' || runes[j] == '\n') {
				j++
			}
			if j < len(runes) && (runes[j] == '}' || runes[j] == ']') {
				continue // drop this comma
			}
		}
		out = append(out, ch)
	}
	return string(out)
}

// loadMCPJSON reads and strict-ish-parses an agent's MCP config, tolerating
// the trailing comma some editors write.
func loadMCPJSON(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", NewError("cannot read MCP JSON %s: %v", path, err)
	}
	text := string(data)
	if gjson.Valid(text) {
		return text, nil
	}
	stripped := stripTrailingCommas(text)
	if !gjson.Valid(stripped) {
		return "", NewErrorWithFix(ExFail, "comments are not supported; remove them or fix the syntax by hand",
			"cannot parse MCP JSON %s", path)
	}
	return stripped, nil
}

// readJSONMCPProfiles returns (profiles, true) if mcpServers.aws-mcp exists,
// or (nil, false) if this agent has no aws-mcp entry at all.
func readJSONMCPProfiles(path string) ([]string, bool, error) {
	text, err := loadMCPJSON(path)
	if err != nil {
		return nil, false, err
	}
	root := gjson.Parse(text)
	if !root.IsObject() {
		return nil, false, NewError("MCP JSON root is not an object: %s", path)
	}
	server := root.Get("mcpServers.aws-mcp")
	if !server.Exists() {
		return nil, false, nil
	}
	if !server.IsObject() {
		return nil, false, NewError("mcpServers.aws-mcp is not an object in %s", path)
	}
	env := server.Get("env")
	if !env.Exists() {
		return nil, true, nil
	}
	if !env.IsObject() {
		return nil, false, NewError("mcpServers.aws-mcp.env is not an object in %s", path)
	}
	value := env.Get(MCPEnvKey)
	if !value.Exists() {
		return nil, true, nil
	}
	if value.Type != gjson.String {
		return nil, false, NewError("%s is not a string in %s", MCPEnvKey, path)
	}
	profiles, err := splitMCPProfiles(value.String())
	if err != nil {
		return nil, false, err
	}
	return profiles, true, nil
}

// writeJSONMCPProfiles sets mcpServers.aws-mcp.env.AWS_MCP_PROXY_PROFILES,
// creating the env object if needed, while preserving every other key and
// its original order and formatting.
func writeJSONMCPProfiles(path string, profiles []string) error {
	text, err := loadMCPJSON(path)
	if err != nil {
		return err
	}
	updated, err := sjson.Set(text, "mcpServers.aws-mcp.env."+MCPEnvKey, strings.Join(profiles, " "))
	if err != nil {
		return err
	}
	return AtomicWrite(path, updated)
}

// --- Codex TOML -------------------------------------------------------
//
// Codex's config.toml is edited with the same line-oriented, comment- and
// formatting-preserving approach as ~/.aws/config, since Go's TOML libraries
// reformat the whole file on write.

var (
	codexMCPRE    = regexp.MustCompile(`^\s*\[mcp_servers\.(?:"aws-mcp"|aws-mcp)\]\s*(?:#.*)?$`)
	codexMCPEnvRE = regexp.MustCompile(`^\s*\[mcp_servers\.(?:"aws-mcp"|aws-mcp)\.env\]\s*(?:#.*)?$`)
	tomlSectionRE = regexp.MustCompile(`^\s*\[.+\]\s*(?:#.*)?$`)
	tomlEnvLineRE = regexp.MustCompile(`^\s*env\s*=`)
)

func tomlEnvValueRE() *regexp.Regexp {
	return regexp.MustCompile(`^\s*` + MCPEnvKey + `\s*=\s*"([^"]*)"\s*(?:#.*)?$`)
}

func tomlInlineValueRE() *regexp.Regexp {
	return regexp.MustCompile(MCPEnvKey + `\s*=\s*"([^"]*)"`)
}

type tomlBounds struct{ start, end int }

// tomlSectionBounds finds the first line matching headerRE, then the next
// line matching any section header, giving an exclusive [start, end) range.
func tomlSectionBounds(lines []string, headerRE *regexp.Regexp) (tomlBounds, bool) {
	start := -1
	for i, line := range lines {
		if headerRE.MatchString(line) {
			start = i
			break
		}
	}
	if start == -1 {
		return tomlBounds{}, false
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if tomlSectionRE.MatchString(lines[i]) {
			end = i
			break
		}
	}
	return tomlBounds{start, end}, true
}

func tomlEnvValue(line string) (string, bool) {
	m := tomlEnvValueRE().FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// readTOMLMCPProfiles mirrors readJSONMCPProfiles: (nil, false, nil) means
// this agent has no aws-mcp entry.
func readTOMLMCPProfiles(path string) ([]string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, NewError("cannot read Codex MCP config %s: %v", path, err)
	}
	lines := strings.Split(string(data), "\n")
	parent, ok := tomlSectionBounds(lines, codexMCPRE)
	if !ok {
		return nil, false, nil
	}
	if envSection, ok := tomlSectionBounds(lines, codexMCPEnvRE); ok {
		for i := envSection.start + 1; i < envSection.end; i++ {
			if value, ok := tomlEnvValue(strings.TrimRight(lines[i], "\r")); ok {
				profiles, err := splitMCPProfiles(value)
				return profiles, true, err
			}
		}
		return nil, true, nil
	}
	for i := parent.start + 1; i < parent.end; i++ {
		if !tomlEnvLineRE.MatchString(lines[i]) {
			continue
		}
		m := tomlInlineValueRE().FindStringSubmatch(lines[i])
		if m == nil {
			return nil, true, nil
		}
		profiles, err := splitMCPProfiles(m[1])
		return profiles, true, err
	}
	return nil, true, nil
}

// writeTOMLMCPProfiles updates an existing [mcp_servers.aws-mcp.env] table,
// an inline env = {...} entry, or appends a new table when neither is present.
func writeTOMLMCPProfiles(path string, profiles []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(data)
	newline := "\n"
	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
	}
	lines := splitKeepEnds(text)
	value := strings.Join(profiles, " ")

	quoted := `"` + value + `"` // profile names are validated to exclude '"' and '\'

	if envSection, ok := tomlSectionBounds(lines, codexMCPEnvRE); ok {
		for i := envSection.start + 1; i < envSection.end; i++ {
			trimmed := strings.TrimRight(lines[i], "\r\n")
			if _, found := tomlEnvValue(trimmed); found {
				lineNewline := newline
				if strings.HasSuffix(lines[i], "\r\n") {
					lineNewline = "\r\n"
				} else if strings.HasSuffix(lines[i], "\n") {
					lineNewline = "\n"
				}
				lines[i] = MCPEnvKey + " = " + quoted + lineNewline
				return AtomicWrite(path, strings.Join(lines, ""))
			}
		}
		inserted := MCPEnvKey + " = " + quoted + "\n"
		lines = append(lines[:envSection.start+1], append([]string{inserted}, lines[envSection.start+1:]...)...)
		return AtomicWrite(path, strings.Join(lines, ""))
	}

	parent, ok := tomlSectionBounds(lines, codexMCPRE)
	if !ok {
		return NewError("no aws-mcp entry in %s", path)
	}
	for i := parent.start + 1; i < parent.end; i++ {
		if !tomlEnvLineRE.MatchString(lines[i]) {
			continue
		}
		if !strings.Contains(lines[i], "{") || !strings.Contains(lines[i], "}") {
			return NewErrorWithFix(ExFail, "convert it to [mcp_servers.aws-mcp.env] and re-run",
				"unsupported multi-line inline env in %s", path)
		}
		if loc := tomlInlineValueRE().FindStringIndex(lines[i]); loc != nil {
			lines[i] = lines[i][:loc[0]] + MCPEnvKey + " = " + quoted + lines[i][loc[1]:]
		} else {
			close := strings.LastIndex(lines[i], "}")
			before := strings.TrimRight(lines[i][:close], " \t")
			separator := ","
			if strings.HasSuffix(before, "{") {
				separator = ""
			}
			lines[i] = before + separator + " " + MCPEnvKey + " = " + quoted + lines[i][close:]
		}
		return AtomicWrite(path, strings.Join(lines, ""))
	}

	separator := ""
	if text != "" && !strings.HasSuffix(text, "\n") {
		separator = newline
	}
	addition := separator + newline + "[mcp_servers.aws-mcp.env]" + newline + MCPEnvKey + " = " + quoted + newline
	return AtomicWrite(path, text+addition)
}

// --- Discovery and the cross-agent allowlist ---------------------------

func discoverMCPConfigs(home string) ([]McpConfig, error) {
	var configs []McpConfig
	for _, loc := range mcpConfigLocations(home) {
		info, err := os.Stat(loc.path)
		if err != nil || info.IsDir() {
			continue
		}
		var profiles []string
		var found bool
		if loc.kind == "json" {
			profiles, found, err = readJSONMCPProfiles(loc.path)
		} else {
			profiles, found, err = readTOMLMCPProfiles(loc.path)
		}
		if err != nil {
			return nil, err
		}
		if found {
			configs = append(configs, McpConfig{Agent: loc.agent, Path: loc.path, Kind: loc.kind, Profiles: profiles})
		}
	}
	return configs, nil
}

func writeMCPProfiles(config McpConfig, profiles []string) error {
	if config.Kind == "json" {
		return writeJSONMCPProfiles(config.Path, profiles)
	}
	return writeTOMLMCPProfiles(config.Path, profiles)
}

// WithDefaultFirst reorders profiles so default is first, since the proxy
// uses the first AWS_MCP_PROXY_PROFILES name as the default.
func WithDefaultFirst(profiles []string, def string) ([]string, error) {
	if def == "" {
		return append([]string(nil), profiles...), nil
	}
	found := false
	for _, p := range profiles {
		if p == def {
			found = true
			break
		}
	}
	if !found {
		return nil, NewErrorWithFix(ExFail, "add it with `gsso mcp add` first",
			"%s is not in the AWS MCP allowlist", def)
	}
	out := []string{def}
	for _, p := range profiles {
		if p != def {
			out = append(out, p)
		}
	}
	return out, nil
}

func profileSet(names []string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

func sameProfiles(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// CurrentMCPProfiles returns the shared allowlist across agents. If the
// lists already match, that list is used. If they drifted (for example
// Codex was edited by hand), it truncates to the intersection so it never
// invents a profile missing from some agent. Order and the default come
// from the shortest list.
func CurrentMCPProfiles(configs []McpConfig, announce bool) []string {
	if len(configs) == 0 {
		return nil
	}
	allSame := true
	for _, c := range configs[1:] {
		if !sameProfiles(c.Profiles, configs[0].Profiles) {
			allSame = false
			break
		}
	}
	if allSame {
		return append([]string(nil), configs[0].Profiles...)
	}

	shortest := configs[0]
	for _, c := range configs[1:] {
		if len(c.Profiles) < len(shortest.Profiles) ||
			(len(c.Profiles) == len(shortest.Profiles) && c.Agent < shortest.Agent) {
			shortest = c
		}
	}
	sharedNames := profileSet(shortest.Profiles)
	for _, c := range configs {
		cur := profileSet(c.Profiles)
		for name := range sharedNames {
			if !cur[name] {
				delete(sharedNames, name)
			}
		}
	}
	var shared []string
	for _, name := range shortest.Profiles {
		if sharedNames[name] {
			shared = append(shared, name)
		}
	}

	if announce {
		var detailParts []string
		everySeen := map[string]bool{}
		for _, c := range configs {
			label := strings.Join(c.Profiles, ",")
			if label == "" {
				label = "(empty)"
			}
			detailParts = append(detailParts, fmt.Sprintf("%s=%s", c.Agent, label))
			for _, p := range c.Profiles {
				everySeen[p] = true
			}
		}
		sharedSet := profileSet(shared)
		var dropped []string
		for p := range everySeen {
			if !sharedSet[p] {
				dropped = append(dropped, p)
			}
		}
		sort.Strings(dropped)
		extra := ""
		if len(dropped) > 0 {
			extra = fmt.Sprintf("; dropping %s", strings.Join(dropped, ", "))
		}
		if len(shared) > 0 {
			plural := "s"
			if len(shared) == 1 {
				plural = ""
			}
			Err("AWS MCP allowlists differ across agents; truncating to the %d shared profile%s%s", len(shared), plural, extra)
		} else {
			Err("AWS MCP allowlists differ across agents and share no profiles; treating the current allowlist as empty")
		}
		Err("  %s", strings.Join(detailParts, "; "))
	}
	return shared
}
